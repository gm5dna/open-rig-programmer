// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import (
	"io"
	"net"
	"sync"
	"time"
)

// Radio is a simulated Icom IC-705 speaking CI-V: an in-memory duplex pipe
// presenting the host end via Port(), serviced from the Radio's own goroutine
// through this package's independent parser (parser.go).
//
// A Radio is safe for concurrent use. SlotState, FramesSeen, SetsSeen,
// AnswerNextReadWithAddress and Close may all be called from goroutines other
// than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// latency and emitters are populated only while New's options run, before
	// any goroutine starts, and are never mutated afterwards — so serve() and
	// the emitters may read them without r.mu.
	latency  time.Duration
	emitters []emitter

	mu           sync.Mutex
	slots        Image
	framesSeen   int
	setsSeen     int
	wrongAddress *Slot // armed by AnswerNextReadWithAddress; nil when unarmed

	// shutdown is closed exactly once, by closePipes, when the radio goes away.
	// Every wait in this package selects against it rather than sleeping
	// blind, so that Close never has to wait out a scripted latency or a
	// broadcast interval before its wg.Wait can return.
	shutdown chan struct{}

	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// emitter is one configured stream of unsolicited frames: what it addresses
// them to, and how often it sends one. A zero interval means CONTINUOUSLY —
// paced only by whatever is reading the port.
type emitter struct {
	to    byte
	every time.Duration
}

// New constructs a simulated IC-705 and starts its servicing goroutine.
//
// A radio built with no options is EMPTY — no memory slot holds anything, and
// every read draws NG. That is deliberate, and it is this package's one
// structural departure from internal/fakedx101, whose New seeds a default
// image: the design's probe treats an all-NG search as a real and expected case
// (an unprogrammed radio), and a default image would put records in slots no
// test asked for, competing silently with whatever that test seeded. A caller
// who wants a populated radio says so — WithFactoryImage(DefaultImage()).
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn: hostConn,
		fakeConn: fakeConn,
		slots:    EmptyImage(),
		shutdown: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}

	r.wg.Add(1)
	go r.serve()

	for _, e := range r.emitters {
		r.wg.Add(1)
		go r.emit(e)
	}
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.hostConn }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine and every emitter to exit. Safe to call
// more than once.
//
// It deliberately closes only fakeConn, not hostConn, and the reason is a
// property of net.Pipe rather than of this radio: Close reports
// io.ErrClosedPipe to a Read or Write made against the end YOU closed, while a
// pending or subsequent Read on the other, still-open end sees io.EOF — which
// is exactly the signal a host should get from "the radio went away". Closing
// hostConn here as well would turn that into io.ErrClosedPipe for the caller, a
// worse and less consistent signal. A caller wanting to release its own Port()
// handle may still call r.Port().Close() itself.
func (r *Radio) Close() error {
	err := r.closePipes()
	r.wg.Wait()
	return err
}

// closePipes is the idempotent, race-safe close of the radio's own pipe end.
// Factored out of Close so that anything running INSIDE a serviced goroutine
// can shut the pipe without deadlocking: Close waits on r.wg, which only
// reaches zero once those goroutines have returned.
func (r *Radio) closePipes() error {
	r.closeOnce.Do(func() {
		// shutdown FIRST: anything parked in a latency or interval wait wakes
		// immediately, before, or regardless of, noticing the pipe close.
		close(r.shutdown)
		r.closeErr = r.fakeConn.Close()
	})
	return r.closeErr
}

// sleepInterruptible waits d, returning early (false) if the radio shuts down
// first. Returns true when the full d genuinely elapsed; d <= 0 returns true at
// once without yielding.
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

// serve is the Radio's own goroutine: it reads from fakeConn, reassembles
// terminated runs, and drives handling and replies.
func (r *Radio) serve() {
	defer r.wg.Done()

	acc := newReassembler(maxAccumulatorBytes)
	buf := make([]byte, 4096)
	for {
		n, err := r.fakeConn.Read(buf)
		if n > 0 {
			for _, ev := range acc.push(buf[:n]) {
				r.handleEvent(ev)
			}
		}
		if err != nil {
			return
		}
	}
}

// handleEvent processes one reassembler event — a terminated run, or the
// accumulator filling without one — and sends whatever reply it produces.
//
// An overflow draws exactly one NG. That is this package's own bounded-input
// policy (register entry 6), not a radio claim.
func (r *Radio) handleEvent(ev accEvent) {
	var reply []byte
	if ev.overflow {
		reply = buildNAK()
	} else {
		reply = r.handleFrame(ev.frame)
	}
	if reply == nil {
		return
	}
	r.rawWrite(reply)
}

// emit runs one configured stream of unsolicited frames until the radio closes.
//
// A zero interval sends continuously: net.Pipe is unbuffered, so each Write
// blocks until the far end reads it, and the flood is paced by whatever is
// draining the port rather than by a timer. A closed pipe fails the Write and
// ends the goroutine, which is how Close stops a flood that nobody is reading.
//
// Writes from here and from serve() cannot interleave mid-frame: net.Pipe
// serialises whole Writes against one another, so a reader sees complete
// frames in some order rather than two frames shuffled together.
func (r *Radio) emit(e emitter) {
	defer r.wg.Done()

	frame := buildUnsolicited(e.to)
	for {
		select {
		case <-r.shutdown:
			return
		default:
		}
		if _, err := r.fakeConn.Write(frame); err != nil {
			return
		}
		if !r.sleepInterruptible(e.every) {
			return
		}
	}
}

// rawWrite sends bytes to the port, honouring the configured per-reply latency.
//
// Errors are not reported: a write failing because the peer has gone away —
// closed the port, or stopped reading — is an expected outcome of a test
// ending, not a bug in the fake. The latency wait is interruptible, since a
// Close mid-wait means the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	if r.latency > 0 && !r.sleepInterruptible(r.latency) {
		return
	}
	_, _ = r.fakeConn.Write(data)
}
