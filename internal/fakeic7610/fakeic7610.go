// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7610

import (
	"net"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated IC-7610: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutines using this package's
// independent frame parser (parser.go).
//
// A Radio is safe for concurrent use — see doc.go, "Concurrency and the pipe".
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutines servicing
	// it: internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	// Fixed at construction, before any goroutine starts, and never mutated
	// afterwards — so serve(), the writer and the flood goroutines may all
	// read them without r.mu.
	idToken   []byte
	usbEcho   bool
	recordLen int

	mu           sync.Mutex
	slots        map[int]MemState
	commandLog   [][2]byte
	bytesWritten []byte

	// The two floods' stop channels, each nil when that flood is not running.
	// They are SEPARATE FIELDS, not one field with a kind, because the two
	// floods are separate line conditions a consumer switches on — doc.go,
	// "Two floods".
	broadcastStop chan struct{}
	addressedStop chan struct{}

	// out is the radio's output queue, drained by one writer goroutine. Every
	// byte the radio sends goes through it: echoes, answers and flood frames
	// alike. One writer means frames can never interleave mid-frame; a queue
	// (rather than a direct write) means a flood or an unread answer can never
	// wedge the goroutine that is reading the host's commands.
	out *outQueue
}

// New constructs a simulated IC-7610 and starts its servicing goroutines.
//
// No channel is set. This fake seeds NO factory image and invents NO record
// contents: a read of any channel answers NG until something sets it, over the
// wire or through SetSlot. That is a deliberate refusal to invent — the guide
// prints no shipped record for any channel, so there is nothing to model, and a
// plausible-looking default would be a fabrication a consumer could mistake for
// evidence. The one byte sequence this package does invent is the ID token, and
// options.go says so at the place it is defined.
func New(opts ...Option) *Radio {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	r := &Radio{
		pipe:      fakepipe.New(),
		idToken:   cfg.idToken,
		usbEcho:   cfg.usbEcho,
		recordLen: cfg.recordLen,
		slots:     make(map[int]MemState),
		out:       newOutQueue(maxQueuedFrames),
	}
	r.pipe.Latency = cfg.latency

	r.serve()
	r.pipe.Go(r.writer)

	// The two construction-time floods, started after the goroutines that
	// carry them and independently of each other. A non-positive interval
	// starts nothing.
	r.StartBroadcastFlood(cfg.transceiveEvery)
	r.StartAddressedFlood(cfg.addressedEvery)

	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
//
// It is a net.Conn rather than an io.ReadWriteCloser so that a consumer can set
// read and write deadlines on it. A test driving a fake radio needs a deadline
// far more than a real serial port does: without one, a test that expects an
// answer the radio has decided not to give hangs instead of failing.
func (r *Radio) Port() net.Conn { return r.pipe.Host() }

// Close shuts the fake radio down: it stops any floods, closes the RADIO's own
// end of the pipe, and waits for every goroutine to exit. Safe to call more
// than once.
//
// It is prompt despite a scripted latency or a flood interval, and it
// deliberately leaves the HOST end open, so a read there sees io.EOF — exactly
// the signal a host should get from "the radio went away". See
// internal/fakepipe's Shutdown for why that direction matters.
func (r *Radio) Close() error {
	r.StopFloods()
	return r.pipe.Close()
}

// closed reports whether the radio has been shut down.
func (r *Radio) closed() bool {
	select {
	case <-r.pipe.Done():
		return true
	default:
		return false
	}
}

// serve starts the Radio's reading goroutine: it takes bytes off the pipe, records
// them, reassembles frames and dispatches each one.
func (r *Radio) serve() {
	acc := newReassembler(maxAccumulatorBytes)
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			r.recordBytes(b)
			for _, f := range acc.push(b) {
				r.dispatch(f)
			}
		})
	})
}

// dispatch handles one complete frame: the echo, then the answer.
//
// THE ECHO COMES FIRST AND IT COMES BEFORE THE ADDRESS FILTER. An echo is a
// property of the line — a USB codec or a bus reflecting what was put on it —
// not a decision the radio's command handler makes, so a frame addressed to
// some other radio is echoed and then ignored. handleFrame applies the address
// filter and returns nil for anything not addressed to 0x98. See doc.go,
// "Echo".
//
// The echo is not delayed by WithLatency; the answer is. A reflection is not a
// reply.
func (r *Radio) dispatch(f frame) {
	if r.usbEcho {
		r.emit(f.raw, nil)
	}
	reply := r.handleFrame(f)
	if reply == nil {
		return
	}
	if !r.pipe.Sleep(r.pipe.Latency) {
		return
	}
	r.emit(reply, nil)
}

// emit queues one frame for the writer goroutine. It never blocks on the pipe:
// see doc.go, "Concurrency and the pipe". abort, when non-nil, is a flood's
// stop channel — a stopped flood drops the frame it was about to send rather
// than queueing it behind a reader that may never come.
func (r *Radio) emit(b []byte, abort <-chan struct{}) {
	select {
	case <-r.pipe.Done():
		return
	case <-abort:
		return
	default:
	}
	r.out.push(b)
}

// writer is the one goroutine that ever writes the radio's pipe end.
//
// Write errors are not reported: a write failing because the peer has gone away
// — closed the port, or stopped reading and then shut down — is an expected
// outcome of a test ending, not a bug in the fake.
func (r *Radio) writer() {
	for {
		for {
			b, ok := r.out.pop()
			if !ok {
				break
			}
			if !r.pipe.WriteNow(b) {
				return
			}
			if r.closed() {
				return
			}
		}
		select {
		case <-r.out.signal:
		case <-r.pipe.Done():
			return
		}
	}
}

// StartBroadcastFlood starts (or restarts) the BROADCAST flood: a frame every
// `every`, addressed to 0x00.
//
// Independent of the addressed flood, and startable AFTER construction so that
// a consumer can Open a quiet radio and then make the line noisy — which is the
// sequence that matters, because an Open that succeeds on a quiet line and a
// session that then has to cope with a flood is the realistic failure a
// consumer must survive.
//
// Calling it while this flood is already running replaces it. A non-positive
// interval starts nothing, so a zero-valued option is simply "no flood".
func (r *Radio) StartBroadcastFlood(every time.Duration) {
	r.startFlood(AddrBroadcast, every)
}

// StartAddressedFlood starts (or restarts) the CONTROLLER-ADDRESSED flood: a
// frame every `every`, addressed to 0xE0. A synthetic line condition; see
// WithAddressedFlood and doc.go, "Two floods". Same independence and same
// restart behaviour as StartBroadcastFlood.
func (r *Radio) StartAddressedFlood(every time.Duration) {
	r.startFlood(AddrController, every)
}

// StopFloods stops both floods. Stopping a flood that is not running is a
// no-op, and stopping is idempotent.
//
// It returns as soon as both are signalled. A frame already on the output queue
// when a flood stops will still be written — the queue is what the writer
// drains, and this method does not reach into it — so a consumer proving the
// line went quiet should drain briefly before asserting silence, exactly as it
// would against a radio whose last transceive frame was already in the UART.
func (r *Radio) StopFloods() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.broadcastStop != nil {
		close(r.broadcastStop)
		r.broadcastStop = nil
	}
	if r.addressedStop != nil {
		close(r.addressedStop)
		r.addressedStop = nil
	}
}

// startFlood is the shared body of the two Start methods. The two floods differ
// in one byte on the wire and in which stop channel they own, and in nothing
// else; that is the claim the two exported methods above make, and this is
// where it is true.
func (r *Radio) startFlood(to byte, every time.Duration) {
	if every <= 0 {
		return
	}

	r.mu.Lock()
	if r.closed() {
		r.mu.Unlock()
		return
	}
	slot := &r.broadcastStop
	if to == AddrController {
		slot = &r.addressedStop
	}
	if *slot != nil {
		close(*slot)
	}
	stop := make(chan struct{})
	*slot = stop
	// Started while mu is held, so a concurrent Close (StopFloods takes mu)
	// cannot reach the pipe's Wait between here and the goroutine registering.
	r.pipe.Go(func() { r.flood(to, every, stop) })
	r.mu.Unlock()
}

// flood emits one frame every `every` until it is stopped or the radio closes.
func (r *Radio) flood(to byte, every time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	f := r.floodFrame(to)

	for {
		select {
		case <-stop:
			return
		case <-r.pipe.Done():
			return
		case <-ticker.C:
			r.emit(f, stop)
		}
	}
}

// recordBytes appends to the BytesWritten record. Every byte the host wrote,
// before framing and before any filtering.
func (r *Radio) recordBytes(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bytesWritten = append(r.bytesWritten, b...)
}

// logCommand appends to CommandLog. Called only for a frame addressed to this
// radio that carries a command — see CommandLog's own doc for what "seen"
// means and why a command with no sub-command logs a zero one.
func (r *Radio) logCommand(cn, sc byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commandLog = append(r.commandLog, [2]byte{cn, sc})
}

// readSlot returns the record stored for ch and whether ch is set at all.
func (r *Radio) readSlot(ch int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.slots[ch]
	if !ok {
		return MemState{}, false
	}
	return m.clone(), true
}

// writeSlot stores a record for ch, copying it: the caller's slice is a window
// on the reassembler's buffer and must not become the radio's state.
func (r *Radio) writeSlot(ch int, m MemState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[ch] = m.clone()
}

// outQueue is a bounded FIFO of whole frames with a one-slot wake-up signal.
//
// It exists so that nothing which PRODUCES output — the serving goroutine, a
// flood — can be blocked by the pipe, which is unbuffered and blocks a writer
// until the host reads. Without it, a consumer that starts a flood and then
// writes a command would deadlock: the writer parked on an unread flood frame,
// the serving goroutine parked behind it, and the host parked in a write to a
// radio that had stopped reading.
type outQueue struct {
	mu     sync.Mutex
	items  [][]byte
	max    int
	signal chan struct{}
}

func newOutQueue(max int) *outQueue {
	return &outQueue{max: max, signal: make(chan struct{}, 1)}
}

// push enqueues one frame, DROPPING THE OLDEST if the queue is full.
//
// Dropping the oldest rather than the newest, and dropping rather than
// blocking, is the behaviour of a UART whose peer has stopped listening: the
// line keeps moving and the unread bytes are gone. A consumer only reaches this
// by running a flood and never reading; doc.go says so.
func (q *outQueue) push(b []byte) {
	q.mu.Lock()
	if len(q.items) >= q.max {
		copy(q.items, q.items[1:])
		q.items = q.items[:len(q.items)-1]
	}
	q.items = append(q.items, b)
	q.mu.Unlock()

	select {
	case q.signal <- struct{}{}:
	default:
	}
}

// pop returns the oldest queued frame, or false if the queue is empty.
func (q *outQueue) pop() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil, false
	}
	b := q.items[0]
	copy(q.items, q.items[1:])
	q.items = q.items[:len(q.items)-1]
	return b, true
}
