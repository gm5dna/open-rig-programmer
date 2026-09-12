// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7800

import (
	"net"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated IC-7800: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutines using this
// package's independent frame parser (parser.go). See doc.go.
//
// A Radio is safe for concurrent use — doc.go, "Concurrency and the pipe".
type Radio struct {
	pipe *fakepipe.Pipe

	// Fixed at construction, before any goroutine starts, and never mutated
	// afterwards — so serve(), the writer and the flood goroutines may all
	// read them without r.mu.
	idToken     []byte
	usbEcho     bool
	recordLen   int
	allFFEmpty  bool
	shortSetPad bool

	mu           sync.Mutex
	slots        map[int]MemState
	commandLog   [][2]byte
	bytesWritten []byte

	// The two floods' stop channels, each nil when that flood is not
	// running — separate fields, not one field with a kind, because a
	// consumer switches on WHICH is running (doc.go, "Two floods").
	broadcastStop chan struct{}
	addressedStop chan struct{}

	out *outQueue
}

// New constructs a simulated IC-7800 and starts its servicing goroutines.
//
// No channel is set. This fake seeds NO factory image and invents NO record
// contents: matrix §0 records that no IC-7800 has ever been asked anything,
// so there is no shipped record to model, and a plausible-looking default
// would be a fabrication a consumer could mistake for evidence.
func New(opts ...Option) *Radio {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	r := &Radio{
		pipe:        fakepipe.New(),
		idToken:     cfg.idToken,
		usbEcho:     cfg.usbEcho,
		recordLen:   cfg.recordLen,
		allFFEmpty:  cfg.allFFEmpty,
		shortSetPad: cfg.shortSetPad,
		slots:       make(map[int]MemState),
		out:         newOutQueue(maxQueuedFrames),
	}
	r.pipe.Latency = cfg.latency

	r.serve()
	r.pipe.Go(r.writer)

	r.StartBroadcastFlood(cfg.transceiveEvery)
	r.StartAddressedFlood(cfg.addressedEvery)

	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() net.Conn { return r.pipe.Host() }

// Close shuts the fake radio down: stops any floods, closes the radio's own
// end of the pipe, and waits for every goroutine to exit. Safe to call more
// than once. It deliberately leaves the HOST end open, so a read there sees
// io.EOF.
func (r *Radio) Close() error {
	r.StopFloods()
	return r.pipe.Close()
}

func (r *Radio) closed() bool {
	select {
	case <-r.pipe.Done():
		return true
	default:
		return false
	}
}

// serve starts the Radio's reading goroutine: it takes bytes off the pipe,
// records them, reassembles frames and dispatches each one.
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
// THE ECHO COMES FIRST AND BEFORE THE ADDRESS FILTER — doc.go, "Echo".
// handleFrame applies the address filter and returns nil for anything not
// addressed to AddrRadio.
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

// emit queues one frame for the writer goroutine. It never blocks on the
// pipe. abort, when non-nil, is a flood's stop channel.
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

// StartBroadcastFlood starts (or restarts) the BROADCAST flood: a frame
// every `every`, addressed to 0x00. A non-positive interval starts nothing.
func (r *Radio) StartBroadcastFlood(every time.Duration) {
	r.startFlood(AddrBroadcast, every)
}

// StartAddressedFlood starts (or restarts) the CONTROLLER-ADDRESSED flood: a
// frame every `every`, addressed to 0xE0. A synthetic line condition — see
// doc.go, "Two floods".
func (r *Radio) StartAddressedFlood(every time.Duration) {
	r.startFlood(AddrController, every)
}

// StopFloods stops both floods. Idempotent; stopping a flood that is not
// running is a no-op.
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
	r.pipe.Go(func() { r.flood(to, every, stop) })
	r.mu.Unlock()
}

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

// recordBytes appends to the BytesWritten record: every byte the host wrote,
// before framing and before any filtering.
func (r *Radio) recordBytes(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bytesWritten = append(r.bytesWritten, b...)
}

// logCommand appends to CommandLog. Called only for a frame addressed to
// this radio that carries a command.
func (r *Radio) logCommand(cn, sc byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commandLog = append(r.commandLog, [2]byte{cn, sc})
}

func (r *Radio) readSlot(ch int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.slots[ch]
	if !ok {
		return MemState{}, false
	}
	return m.clone(), true
}

func (r *Radio) writeSlot(ch int, m MemState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[ch] = m.clone()
}

// outQueue is a bounded FIFO of whole frames with a one-slot wake-up signal.
// See doc.go, "Concurrency and the pipe".
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
