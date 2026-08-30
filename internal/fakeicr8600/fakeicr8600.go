// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

import (
	"io"
	"net"
	"sync"
	"time"
)

// eventQueueDepth is how many reassembled frames may wait for the serve loop.
//
// It is buffered so that reading the port and answering it are genuinely
// separate: a serve loop held up writing a broadcast into a pipe nobody is
// draining must not also stop the fake reading, or a consumer's own write would
// block against a fake that had stopped listening, and both ends would wait for
// each other forever.
const eventQueueDepth = 64

// readChunkBytes is the read buffer size. Frames may split across reads and
// several may arrive in one; the reassembler handles both, so this is only a
// syscall-sizing choice.
const readChunkBytes = 4096

// Radio is a simulated Icom IC-R8600 speaking binary CI-V over an in-memory
// duplex connection (Port()), serviced from its own goroutines.
//
// A Radio is safe for concurrent use: Record, Frames and Close may all be
// called from goroutines other than whatever is reading or writing Port(). Run
// tests with -race.
type Radio struct {
	hostConn net.Conn // returned by Port(); the consumer's end
	fakeConn net.Conn // the radio's own end, read and written only by this package

	// The fields below are populated while New runs its options, before any
	// goroutine starts, and never mutated afterwards, so the serve loop reads
	// them without r.mu.
	radioAddr       byte
	identityToken   []byte
	latency         time.Duration
	broadcastPeriod time.Duration
	echo            bool
	emptyReplyAllFF bool
	shortSets       bool
	shortSetFill    byte

	events chan accEvent

	mu      sync.Mutex
	records map[chanAddr]MemState
	frames  [][]byte
	replyQ  [][]byte

	// replyReady carries one token per reply whose scheduled latency has
	// elapsed. The replies themselves queue in replyQ, so they are written in
	// the order they were produced whatever the timers do.
	replyReady chan struct{}

	shutdown  chan struct{}
	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// New constructs a simulated IC-R8600 and starts its servicing goroutines.
//
// Without options it answers at 96h, holds the default image (image.go — one
// channel per declared mode class) and answers 19 00 with the constructor's own
// arbitrary identity token. The image's extent and the token's value are both
// inventions, recorded in doc.go's ASSUMED register rather than presented as
// facts about any receiver.
//
// Close it when done: a Radio owns goroutines and a pipe.
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()

	r := &Radio{
		hostConn:      hostConn,
		fakeConn:      fakeConn,
		radioAddr:     defaultRadioAddr,
		identityToken: append([]byte(nil), defaultIdentityToken...),
		events:        make(chan accEvent, eventQueueDepth),
		records:       defaultImage(),
		replyReady:    make(chan struct{}, eventQueueDepth),
		shutdown:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}

	r.wg.Add(2)
	go r.readLoop()
	go r.serve()
	return r
}

// Port returns the consumer's end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.hostConn }

// Close shuts the fake radio down and waits for its goroutines to exit. Safe to
// call more than once.
//
// It closes only the RADIO's end of the pipe. Close on a net.Pipe reports
// io.ErrClosedPipe to reads and writes made against the end you closed
// yourself, whilst the other, still-open end sees io.EOF — which is exactly the
// signal a host should get from "the radio went away". Closing the consumer's
// end here too would turn that into io.ErrClosedPipe, a worse signal. A
// consumer that wants to release its own handle may call r.Port().Close().
//
// It does NOT wait out a pending WithLatency delay: the delay is scheduled
// rather than slept through, so a test may script seconds of latency and still
// tear down at once.
func (r *Radio) Close() error {
	r.closeOnce.Do(func() {
		// shutdown closes FIRST, so anything parked on a channel send wakes
		// before, or regardless of, the pipe closing under it.
		close(r.shutdown)
		r.closeErr = r.fakeConn.Close()
	})
	r.wg.Wait()
	return r.closeErr
}

// Record returns the raw record bytes the fake currently holds for a channel,
// so a consumer can compare a write byte for byte, and whether it holds any.
//
// The bytes are a copy: a consumer that mutates them is not mutating the fake.
// group and channel are the two printed two-byte address fields as numbers,
// 0-based — see WithRecord and bcd2.
func (r *Radio) Record(group, channel int) ([]byte, bool) {
	addr := addrOf(group, channel)

	r.mu.Lock()
	defer r.mu.Unlock()

	st, ok := r.records[addr]
	if !ok {
		return nil, false
	}
	return st.clone().Record, true
}

// Frames returns every frame the fake has RECEIVED, in order, so a consumer can
// assert that a refused write put nothing on the wire, or that opening a
// session sent no mutation at all.
//
// RECEIVED, not sent: a broadcast running beside the traffic never appears
// here. Every frame the reassembler completed appears, including ones the fake
// said nothing to — a frame addressed to some other radio is silently ignored
// on the wire but is recorded here, because "the fake never saw it" and "the
// fake saw it and held its tongue" are different facts and a consumer may need
// to tell them apart. An over-length run is refused before it is ever a frame,
// so it has none to record.
//
// The frames, and the slice holding them, are copies.
func (r *Radio) Frames() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.frames) == 0 {
		return nil
	}
	out := make([][]byte, len(r.frames))
	for i, f := range r.frames {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// recordFrame appends one received frame to the log.
func (r *Radio) recordFrame(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, append([]byte(nil), frame...))
}

// readLoop is the only goroutine that ever reads fakeConn. It reassembles
// frames and hands each event to the serve loop.
func (r *Radio) readLoop() {
	defer r.wg.Done()
	defer close(r.events)

	acc := newReassembler(maxBodyBytes)
	buf := make([]byte, readChunkBytes)
	for {
		n, err := r.fakeConn.Read(buf)
		if n > 0 {
			for _, ev := range acc.push(buf[:n]) {
				select {
				case r.events <- ev:
				case <-r.shutdown:
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// serve is the only goroutine that ever writes fakeConn. It answers requests
// and drives the broadcast flood.
//
// THE FLOOD IS A TICKER CASE IN THE SAME SELECT AS REQUEST HANDLING, so it
// interleaves with request handling rather than waiting for it. Nothing in this
// loop blocks on a request: reading happens in readLoop, and a scripted latency
// is a scheduled timer rather than a sleep, so the ticker keeps firing
// throughout both.
func (r *Radio) serve() {
	defer r.wg.Done()

	broadcastC, stopBroadcast := tickerFor(r.broadcastPeriod)
	defer stopBroadcast()

	for {
		select {
		case <-r.shutdown:
			return

		case ev, ok := <-r.events:
			if !ok {
				return
			}
			for _, reply := range r.handleEvent(ev) {
				r.scheduleReply(reply)
			}

		case <-r.replyReady:
			if b := r.popReply(); b != nil {
				r.rawWrite(b)
			}

		case <-broadcastC:
			r.rawWrite(r.broadcastFrame())
		}
	}
}

// tickerFor returns a tick channel and its stop function. A non-positive
// period yields a nil channel, which blocks forever in a select — the
// idiomatic way to leave a case switched off.
func tickerFor(period time.Duration) (<-chan time.Time, func()) {
	if period <= 0 {
		return nil, func() {}
	}
	t := time.NewTicker(period)
	return t.C, t.Stop
}

// scheduleReply sends a reply, after the configured latency if there is one.
//
// With no latency the reply goes straight out. With one, the bytes join a FIFO
// queue and a timer signals the serve loop when their wait is over — never a
// sleep inside the loop, which would stop the flood and stall the next request.
// Because the queue preserves order, replies are written in the order they were
// produced regardless of how the timers happen to fire.
func (r *Radio) scheduleReply(reply []byte) {
	if r.latency <= 0 {
		r.rawWrite(reply)
		return
	}

	r.mu.Lock()
	r.replyQ = append(r.replyQ, reply)
	r.mu.Unlock()

	time.AfterFunc(r.latency, func() {
		select {
		case r.replyReady <- struct{}{}:
		case <-r.shutdown:
		}
	})
}

// popReply takes the oldest queued reply, or nil if there is none.
func (r *Radio) popReply() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.replyQ) == 0 {
		return nil
	}
	next := r.replyQ[0]
	r.replyQ = r.replyQ[1:]
	return next
}

// rawWrite puts bytes on the wire.
//
// Errors are not reported: a write failing because the consumer has gone away —
// closed the port, or stopped reading — is an expected outcome of a test double
// being torn down, not a bug in the fake.
func (r *Radio) rawWrite(data []byte) {
	if len(data) == 0 {
		return
	}
	_, _ = r.fakeConn.Write(data)
}

// ---------------------------------------------------------------------------
// Dispatch
// ---------------------------------------------------------------------------

// handleEvent turns one reassembler event into whatever the fake should send —
// an echo, an answer, both, or nothing at all.
//
// SILENCE AND REJECTION ARE DIFFERENT ANSWERS HERE, and keeping them different
// is the point. A receiver at another address never hears the frame and the
// controller times out; a receiver that heard a frame it cannot honour says NG.
// A fake that answered both alike would make a driver's timeout branch
// untestable.
func (r *Radio) handleEvent(ev accEvent) [][]byte {
	if ev.overflow {
		// An over-length run is refused as its own event. The fake cannot know
		// who it was addressed to — the address is long gone behind the cap —
		// and answering NG is the honest reading of "something arrived that
		// could not be a frame". doc.go register entry 12.
		return [][]byte{r.ngReply()}
	}

	r.recordFrame(ev.frame)

	var out [][]byte
	if r.echo {
		// The echo is the frame VERBATIM, and it is emitted before any answer.
		// Every complete frame is echoed, including one addressed to some other
		// radio, because a bus echo is a property of the wire and not of who
		// was being spoken to.
		out = append(out, append([]byte(nil), ev.frame...))
	}

	to, ok := frameAddressee(ev.frame)
	if !ok || to != r.radioAddr {
		return out
	}

	pf, ok := parseFrame(ev.frame)
	if !ok {
		return append(out, r.ngReply())
	}
	if reply := r.handlePayload(pf.payload); reply != nil {
		out = append(out, reply)
	}
	return out
}

// handlePayload dispatches one payload — cn, then any sc, then any data — that
// arrived addressed to this receiver. Everything it does not recognise is NG.
//
// ONLY TWO GRAMMARS ARE ADMITTED: 19 00 (read the receiver ID) and 1A 00
// (send/read memory channel contents). Everything else is refused, INCLUDING
// commands a real IC-R8600 certainly honours — 1A 05 heads two full pages of
// set-mode items, 1A 0B 00 is the programmable scan-start record, 18 01 is the
// power-on the guide's own worked example illustrates. Refusing them is this
// FAKE's tier policy, not a fact about the receiver. doc.go register entry 16.
func (r *Radio) handlePayload(payload []byte) []byte {
	if len(payload) == 0 {
		return r.ngReply()
	}

	switch payload[0] {
	case cmdReceiverID:
		return r.handleReadID(payload[1:])
	case cmdMemory:
		if len(payload) >= 2 && payload[1] == subMemoryContents {
			return r.handleMemoryContents(payload[2:])
		}
		return r.ngReply()
	default:
		return r.ngReply()
	}
}

// handleReadID answers "Read the receiver ID", cn 19 sc 00.
//
// The command table's Data cell for this row is BLANK, so the request carries
// no data bytes and one that does is not the printed request. The ANSWER
// carries the fake's configured identity token, whose value this package
// asserts nothing about: see WithIDToken and doc.go register entry 1.
func (r *Radio) handleReadID(rest []byte) []byte {
	if len(rest) != 1 || rest[0] != subReadID {
		return r.ngReply()
	}
	payload := make([]byte, 0, 2+len(r.identityToken))
	payload = append(payload, cmdReceiverID, subReadID)
	payload = append(payload, r.identityToken...)
	return r.buildReply(payload...)
}

// handleMemoryContents answers "Send/read memory channel contents", cn 1A
// sc 00, in both directions. data is everything after the sub-command.
//
// The four printed address bytes come first — indices (1),(2) the memory group
// number and (3),(4) the memory channel number, both packed BCD and both
// 0-based. What follows them is the record.
func (r *Radio) handleMemoryContents(data []byte) []byte {
	if len(data) < addressBytes {
		return r.ngReply()
	}

	addr := chanAddr{
		group:   [2]byte{data[0], data[1]},
		channel: [2]byte{data[2], data[3]},
	}
	rest := data[addressBytes:]

	// The clear form, printed at the foot of PDF page 15's left column:
	// "Command 1A 00 clears a memory channel by sending the command in the
	// following format. (1),(2): 0000 ~ 0101 group … (3),(4): Memory channel
	// number; (5): 'FF'; (6) ~ : None." — four address bytes, then a single FF,
	// then nothing.
	//
	// This tier ships NO erase path at all, so this fake refuses it. The check
	// comes BEFORE the record rules so that the refusal is the clear form's
	// own, not an accident of a one-byte record failing a length test — the
	// two are indistinguishable on the wire, which is why isClearForm is
	// pinned directly by TestIsClearForm rather than only through a reply.
	// doc.go register entry 7.
	if isClearForm(rest) {
		return r.ngReply()
	}

	if len(rest) == 0 {
		return r.readChannel(addr)
	}
	return r.setChannel(addr, rest)
}

// isClearForm reports whether what follows the four address bytes is the
// printed clear form: exactly one byte, and that byte FF.
//
// It is its own function because the refusal it drives is INVISIBLE on the
// wire: a clear frame and a one-byte nonsense record both draw NG, so nothing a
// consumer can observe distinguishes "refused as an erase" from "refused as a
// record too short to carry a mode byte". Naming the form makes the branch
// assertable, and TestIsClearForm asserts it.
func isClearForm(rest []byte) bool {
	return len(rest) == clearFormLen-addressBytes && rest[0] == clearFormByte
}

// readChannel answers a read of one channel.
//
// The READ REQUEST FORM ITSELF IS ASSUMED — doc.go register entry 4. The guide
// declares 1A 00 a send/read pair (the command table's asterisk, expanded on
// PDF page 9 as "*(Asterisk) Send/read data") and never draws the read
// direction's data area anywhere. "The four address bytes and then stop" is
// this package's reading of it.
//
// An UNOCCUPIED channel takes the empty-channel path: NG by default, or an
// all-FF record under WithEmptyReplyAllFF. Both are ASSUMED, separately —
// doc.go entries 5 and 6.
func (r *Radio) readChannel(addr chanAddr) []byte {
	r.mu.Lock()
	st, ok := r.records[addr]
	r.mu.Unlock()

	var record []byte
	switch {
	case ok:
		record = st.clone().Record
	case r.emptyReplyAllFF:
		record = make([]byte, emptyReplyLen)
		for i := range record {
			record[i] = 0xFF
		}
	default:
		return r.ngReply()
	}

	payload := make([]byte, 0, 2+addressBytes+len(record))
	payload = append(payload, cmdMemory, subMemoryContents)
	payload = append(payload, addr.group[0], addr.group[1], addr.channel[0], addr.channel[1])
	payload = append(payload, record...)
	return r.buildReply(payload...)
}

// setChannel writes one channel, and is where every refusal rule about a record
// lives.
//
// THE MODE BYTE DECIDES, NOT THE LENGTH. On this receiver two layouts — FM and
// DCR — are both 44 record-only bytes with different contents, so a length
// names no layout on its own. The rules, in order:
//
//  1. A record too short to carry a mode byte, or whose mode byte is none of
//     the eighteen the mode table prints, is refused: no layout is selected and
//     this fake will not guess one.
//  2. A record whose length is not the selected layout's is refused. That
//     covers both the wrong-sibling case (37 ± n, 64, 65 — lengths no
//     IC-R8600 layout has) and the subtler mode/length disagreement (an FM
//     mode byte on a 45-byte dPMR record, which IS an accepted length and is
//     still wrong).
//  3. Unless WithShortSetsAccepted says otherwise, a head-only 37-byte record
//     for a mode class whose layout has a tail is refused. The document says
//     such a set IS accepted, with unstated defaults applied; refusing invents
//     nothing where filling in would invent up to eight bytes.
//
// A refused set stores NOTHING. A record that passes is stored RAW, exactly as
// it arrived: no byte is normalised, reordered or repaired, and no field is
// validated.
//
// Note what is deliberately NOT a rule: the length a channel already holds. A
// channel holding a 37-byte AM record accepts a 44-byte FM record over it,
// because on this receiver a channel's length is a property of its mode and a
// consumer is entitled to change the mode.
func (r *Radio) setChannel(addr chanAddr, record []byte) []byte {
	l, ok := layoutForRecord(record)
	if !ok {
		return r.ngReply()
	}

	stored := make([]byte, len(record))
	copy(stored, record)

	switch {
	case len(record) == l.RecordLen:
		// The full layout for the mode, which is all this program ever sends.
	case r.shortSets && len(record) == headRecordLen && l.RecordLen > headRecordLen:
		pad := make([]byte, l.RecordLen-headRecordLen)
		for i := range pad {
			pad[i] = r.shortSetFill
		}
		stored = append(stored, pad...)
	default:
		return r.ngReply()
	}

	r.mu.Lock()
	r.records[addr] = MemState{Record: stored}
	r.mu.Unlock()
	return r.ackReply()
}
