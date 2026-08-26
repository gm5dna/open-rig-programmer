// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300

import (
	"io"
	"net"
	"sync"
	"time"
)

// The CI-V frame grammar this fake speaks: FE FE <to> <from> <cn> [<sc>]
// <data> FD. FE and FD are reserved and appear nowhere else in a well-formed
// frame, which is what lets the framer resynchronise on them.
const (
	preamble     byte = 0xFE
	endOfMessage byte = 0xFD

	// defaultRadioAddress is this radio's default CI-V address; WithRadioAddress
	// moves it, and every byte of every answer that shows the radio's address
	// moves with it.
	defaultRadioAddress byte = 0x94
	// controllerAddress is the controller's address — the `to` byte of every
	// answer the radio sends, and the address WithAddressedFlood's frames
	// carry.
	controllerAddress byte = 0xE0
	// broadcastAddress is the `to` byte of an unsolicited transceive frame,
	// which is addressed to nobody in particular.
	broadcastAddress byte = 0x00

	codeOK byte = 0xFB // FE FE E0 94 FB FD
	codeNG byte = 0xFA // FE FE E0 94 FA FD

	cmdID            byte = 0x19 // 19 00 — identity read
	subID            byte = 0x00
	cmdMemory        byte = 0x1A // 1A 00 — memory content
	subMemoryContent byte = 0x00
	cmdTransceive    byte = 0x00 // the unsolicited frames' command byte
)

// maxFrameBytes caps how many body bytes the framer will accumulate before it
// gives up on a frame and hunts for a fresh FE FE. It is this package's own
// bounded-input policy and no manual figure (doc.go, ASSUMED entry 11): the
// longest frame this radio has any business seeing is a `1A 00` set, at
// 2 preamble + 2 address + 2 command + 2 channel + 39 record + 1 terminator =
// 48 bytes.
const maxFrameBytes = 256

// unsolicitedPayload is the data area of the frames WithTransceiveBroadcasts
// and WithAddressedFlood emit. Its content carries no meaning to this fake and
// is asserted on by nothing: what those options exist to produce is TRAFFIC —
// bytes arriving at a host that did not ask for them — not a particular
// reading. See doc.go, ASSUMED entry 12.
var unsolicitedPayload = []byte{cmdTransceive, 0x00, 0x40, 0x07, 0x14, 0x00}

// Radio is a simulated Icom IC-7300 speaking CI-V over an in-memory duplex
// pipe, whose host end is Port(). It is serviced by its own goroutine, and is
// safe for concurrent use: Channel, Channels, Received, Sent and Close may all
// be called from goroutines other than whatever is reading or writing Port()
// (run the tests with -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// The fields below are populated only while New's options run, before any
	// goroutine starts, and are never mutated afterwards, so they are read
	// without r.mu.

	// addr is the address this radio is CURRENTLY CONFIGURED with. It is the
	// only address this radio answers to, and it is the `from` byte of every
	// answer it sends — the identity answer, the record answer, the OK frame
	// and the NG frame alike.
	addr            byte
	idToken         []byte
	echo            bool
	latency         time.Duration
	broadcastPeriod time.Duration
	floodPeriod     time.Duration

	mu       sync.Mutex
	channels map[string][]byte
	received [][]byte
	sent     [][]byte
	// ignored counts the complete frames the radio saw and did not answer:
	// frames too short to carry an address, and frames whose `to` byte is not
	// this radio's. It is unexported because no downstream test was said to
	// need it; the package's own tests read it directly.
	ignored int

	// writeMu serialises the pipe's write side. serve() is not the only
	// writer once WithTransceiveBroadcasts or WithAddressedFlood is in play,
	// and two writers interleaving mid-frame would put bytes on the wire that
	// no framer could recover.
	writeMu sync.Mutex

	// shutdown is closed (exactly once, by closePipes) when the radio goes
	// away. Every wait in this package selects against it instead of calling
	// bare time.Sleep, so a pending latency or a flood's inter-frame gap can
	// never make Close wait it out.
	shutdown chan struct{}

	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// New constructs a simulated IC-7300 and starts its servicing goroutine. With
// no options it answers at CI-V address 94 with an empty memory — every `1A 00`
// read answers NG until something has been written or seeded.
//
// The caller must Close the radio when finished with it.
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn: hostConn,
		fakeConn: fakeConn,
		addr:     defaultRadioAddress,
		channels: make(map[string][]byte),
		shutdown: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	if len(r.idToken) == 0 {
		// The identity answer's data byte is undocumented on this radio
		// (doc.go, ASSUMED entry 8). Absent a WithIDToken, answer with the
		// address the radio is configured with, resolved HERE rather than in
		// the struct literal so that the default follows a WithRadioAddress
		// given in any position in the option list.
		r.idToken = []byte{r.addr}
	}

	r.wg.Add(1)
	go r.serve()
	if r.broadcastPeriod > 0 {
		r.wg.Add(1)
		go r.emit(r.broadcastPeriod, broadcastAddress)
	}
	if r.floodPeriod > 0 {
		r.wg.Add(1)
		go r.emit(r.floodPeriod, controllerAddress)
	}
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection. It is an io.ReadWriteCloser and
// never a repository interface: this package may not import one (doc.go, THE
// HARD RULE).
func (r *Radio) Port() io.ReadWriteCloser { return r.hostConn }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine — and any broadcast or flood goroutine
// — to exit. Safe to call more than once.
//
// It deliberately closes only fakeConn, not hostConn. net.Pipe reports
// io.ErrClosedPipe only to a Read or Write made against the end you yourself
// closed; a Read on the other, still-open end sees io.EOF, which is what a host
// should get from "the radio went away". Closing hostConn here too would flip
// that to io.ErrClosedPipe for the caller. A caller wanting to release its own
// Port() handle may still call r.Port().Close() itself.
func (r *Radio) Close() error {
	err := r.closePipes()
	r.wg.Wait()
	return err
}

// closePipes is the idempotent, race-safe close of the radio's own pipe end.
// It is factored out of Close so that anything running inside a serving
// goroutine can shut the pipe without deadlocking on r.wg.
func (r *Radio) closePipes() error {
	r.closeOnce.Do(func() {
		// Close shutdown FIRST, so a goroutine parked in a latency wait or a
		// flood's inter-frame gap wakes at once, before or regardless of it
		// noticing the pipe itself close.
		close(r.shutdown)
		r.closeErr = r.fakeConn.Close()
	})
	return r.closeErr
}

// sleepInterruptible waits d, returning early (false) if the radio's shutdown
// channel closes first. It returns true when the full d genuinely elapsed;
// d <= 0 returns true at once.
func (r *Radio) sleepInterruptible(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.shutdown:
		return false
	}
}

// --- Test-inspection API ---
//
// Everything below is read by tests, never by anything talking to the radio.
// Production code reaches this fake only through Port().

// Received returns every COMPLETE frame the radio saw, in arrival order, as
// defensive copies — including the frames it ignored because they were not
// addressed to it. Each is normalised to exactly two preamble bytes: leading
// noise is dropped and a run of three or more FE bytes is recorded as two
// (doc.go, ASSUMED entry 10).
func (r *Radio) Received() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return copyFrames(r.received)
}

// Sent returns every frame the radio put on the wire, in order, as defensive
// copies: its answers, and also any echo, transceive broadcast or addressed
// flood frame it emitted. A frame is recorded when the radio commits it to the
// wire, which may precede its delivery and does not require the write to
// succeed (doc.go, ASSUMED entry 10).
func (r *Radio) Sent() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return copyFrames(r.sent)
}

// Channel returns the record stored for a canonical slot string ("001".."099",
// "P1", "P2") and whether any record has ever been stored for it. The record is
// a defensive copy.
func (r *Radio) Channel(addr string) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.channels[addr]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), rec...), true
}

// Channels returns a copy of the whole stored map, records included.
func (r *Radio) Channels() map[string][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string][]byte, len(r.channels))
	for k, v := range r.channels {
		out[k] = append([]byte(nil), v...)
	}
	return out
}

func copyFrames(in [][]byte) [][]byte {
	out := make([][]byte, len(in))
	for i, f := range in {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// --- Serving ---

// serve is the Radio's own goroutine: it reads from fakeConn, reassembles
// frames and drives command handling and replies.
func (r *Radio) serve() {
	defer r.wg.Done()

	f := newFramer(maxFrameBytes)
	buf := make([]byte, 4096)
	for {
		n, err := r.fakeConn.Read(buf)
		if n > 0 {
			for _, frame := range f.push(buf[:n]) {
				r.handleComplete(frame)
			}
		}
		if err != nil {
			return
		}
	}
}

// emit is a broadcast or flood goroutine: one frame addressed to `to` every
// period, forever, until the radio shuts down. Its writes are not latency-
// delayed — latency is a property of a REPLY (WithLatency) — and a write that
// fails ends the goroutine, since the pipe it was writing to has gone.
func (r *Radio) emit(period time.Duration, to byte) {
	defer r.wg.Done()
	frame := buildFrame(to, r.addr, unsolicitedPayload)
	for {
		if !r.sleepInterruptible(period) {
			return
		}
		if !r.send(frame) {
			return
		}
	}
}

// handleComplete records one complete frame, echoes it if echoing is on, and
// sends whatever reply it produces.
func (r *Radio) handleComplete(frame []byte) {
	r.mu.Lock()
	r.received = append(r.received, frame)
	r.mu.Unlock()

	if r.echo {
		// A CI-V bus is one wire, so a controller hears its own transmission
		// come back. The echo is the frame VERBATIM — it is not addressed to
		// the controller and is not an answer — and every complete frame is
		// echoed, including one addressed to some other radio, because a bus
		// echo is a property of the wire and not of who was being spoken to.
		if !r.send(append([]byte(nil), frame...)) {
			return
		}
	}

	reply := r.handleFrame(frame)
	if reply == nil {
		return
	}
	r.rawWrite(reply)
}

// handleFrame decides what one complete frame deserves. A nil reply is silence,
// and silence here means "not for me": this radio answers only frames whose
// `to` byte is the address it is currently configured with, and counts and
// ignores everything else.
func (r *Radio) handleFrame(frame []byte) []byte {
	body := frameBody(frame)
	// Below three bytes there is no <to>, <from> and <cn> to read, so there is
	// nothing to be addressed by and nothing to answer.
	if len(body) < 3 || body[0] != r.addr {
		r.mu.Lock()
		r.ignored++
		r.mu.Unlock()
		return nil
	}

	cn, rest := body[2], body[3:]
	switch cn {
	case cmdID:
		if len(rest) == 1 && rest[0] == subID {
			return r.buildFrame(append([]byte{cmdID, subID}, r.idToken...))
		}
		return r.ng()
	case cmdMemory:
		if len(rest) >= 1 && rest[0] == subMemoryContent {
			return r.handleMemory(rest[1:])
		}
		// Every other subcommand of 1A — 1A 05 among them — is refused.
		return r.ng()
	default:
		// 0B, 18 00, 18 01 and the rest of the command set: this fake models
		// the memory surface and the identity read, and answers NG to
		// everything else rather than inventing behaviour for it.
		return r.ng()
	}
}

// handleMemory serves the data area of a `1A 00` frame: the two channel-address
// bytes alone are a read, and anything after them is the record of a set.
func (r *Radio) handleMemory(data []byte) []byte {
	if len(data) < 2 {
		return r.ng()
	}
	slot, ok := canonicalSlot(data[0], data[1])
	if !ok {
		return r.ng()
	}

	if len(data) == 2 {
		rec, ok := r.Channel(slot)
		if !ok {
			// A channel that has never been written has nothing to answer
			// with. The page prints no "empty channel" answer, so NG it is.
			return r.ng()
		}
		return r.buildFrame(append([]byte{cmdMemory, subMemoryContent, data[0], data[1]}, rec...))
	}

	// A set. THE DOCUMENTED CLEAR FORMS ARE REFUSED HERE, by the length check
	// and deliberately: `1A 00 <channel> FF` carries a one-byte data area, not
	// a record, and this fake refuses it because the software under test ships
	// no erase path (doc.go, "Why the clear forms are refused").
	record := data[2:]
	if err := validateRecord(record); err != nil {
		return r.ng()
	}
	stored := append([]byte(nil), record...)
	r.mu.Lock()
	r.channels[slot] = stored
	r.mu.Unlock()
	return r.ok()
}

// --- Frame building ---

// buildFrame wraps a command body in this radio's own answer envelope:
// FE FE <controller> <this radio's configured address> <body> FD.
func (r *Radio) buildFrame(body []byte) []byte {
	return buildFrame(controllerAddress, r.addr, body)
}

// ok is the six-byte OK frame, FE FE E0 94 FB FD at the default address.
func (r *Radio) ok() []byte { return r.buildFrame([]byte{codeOK}) }

// ng is the six-byte NG frame, FE FE E0 94 FA FD at the default address.
func (r *Radio) ng() []byte { return r.buildFrame([]byte{codeNG}) }

// buildFrame assembles FE FE <to> <from> <body> FD.
func buildFrame(to, from byte, body []byte) []byte {
	frame := make([]byte, 0, 5+len(body))
	frame = append(frame, preamble, preamble, to, from)
	frame = append(frame, body...)
	return append(frame, endOfMessage)
}

// frameBody strips a frame's two preamble bytes and its terminator, leaving
// <to> <from> <cn> [<sc>] <data>.
func frameBody(frame []byte) []byte { return frame[2 : len(frame)-1] }

// rawWrite sends one reply, honouring the configured per-reply latency. The
// wait is interruptible: a Close during it abandons the reply, since the pipe
// is gone and the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	if r.latency > 0 && !r.sleepInterruptible(r.latency) {
		return
	}
	r.send(data)
}

// send records a frame in the transcript and writes it, serialised against
// every other writer. It reports whether the write succeeded; a failure means
// the peer has gone away, which is an expected outcome and not a bug in the
// fake.
//
// The transcript entry is made BEFORE the write, not after. net.Pipe hands the
// bytes to the reader and only then returns from Write, so recording afterwards
// would leave a window in which a host had read a frame that Sent() did not yet
// list — a race a test could not avoid.
func (r *Radio) send(frame []byte) bool {
	r.mu.Lock()
	r.sent = append(r.sent, frame)
	r.mu.Unlock()

	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.fakeConn.Write(frame)
	return err == nil
}

// --- Framing ---

// framer turns an arbitrary stream of Write() chunks into complete CI-V frames.
// Framing only: it says nothing about what any frame means.
//
// It tolerates LEADING NOISE — any byte before a preamble run is discarded —
// and ANY NUMBER OF EXTRA FE PREAMBLE BYTES: a frame opens on the first
// non-FE byte after a run of two or more FEs. Because FE and FD are reserved
// and appear nowhere in a well-formed frame, an FE inside a body can only mean
// the body was never a frame, so the partial is abandoned and the FE counted as
// the first of a new run. The zero value is not usable; construct with
// newFramer.
type framer struct {
	max    int
	feRun  int
	inBody bool
	body   []byte
}

func newFramer(max int) *framer {
	if max <= 0 {
		max = maxFrameBytes
	}
	return &framer{max: max}
}

// push appends chunk and returns, in arrival order, every complete frame it
// produced. Each returned frame is normalised to exactly two preamble bytes.
func (f *framer) push(chunk []byte) [][]byte {
	var frames [][]byte
	for _, b := range chunk {
		if !f.inBody {
			switch {
			case b == preamble:
				if f.feRun < 2 {
					f.feRun++
				}
			case f.feRun >= 2 && b == endOfMessage:
				// FE FE FD: a complete but bodyless frame. It is recorded,
				// because it IS a complete frame, and then ignored for want of
				// an address.
				frames = append(frames, buildBare(nil))
				f.reset()
			case f.feRun >= 2:
				f.inBody = true
				f.body = append(f.body[:0], b)
			default:
				f.feRun = 0 // leading noise
			}
			continue
		}

		switch {
		case b == endOfMessage:
			frames = append(frames, buildBare(f.body))
			f.reset()
		case b == preamble:
			f.reset()
			f.feRun = 1
		default:
			f.body = append(f.body, b)
			if len(f.body) > f.max {
				// Over the cap with no terminator in sight: drop the partial
				// and hunt for a fresh FE FE.
				f.reset()
			}
		}
	}
	return frames
}

func (f *framer) reset() {
	f.feRun = 0
	f.inBody = false
	f.body = f.body[:0]
}

// buildBare wraps a framed body back up as FE FE <body> FD, copying it — the
// framer reuses its accumulator, so the transcript must not alias it.
func buildBare(body []byte) []byte {
	frame := make([]byte, 0, 3+len(body))
	frame = append(frame, preamble, preamble)
	frame = append(frame, body...)
	return append(frame, endOfMessage)
}
