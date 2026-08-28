// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

import (
	"io"
	"net"
	"sync"
	"time"
)

// eventQueueDepth is how many reassembled frames may wait for the serve loop.
//
// It is buffered so that reading the port and answering it are genuinely
// separate: a serve loop held up writing a flood frame into a pipe nobody is
// draining must not also stop the fake reading, or a consumer's own write would
// block against a fake that had stopped listening, and both ends would wait for
// each other forever.
const eventQueueDepth = 64

// readChunkBytes is the read buffer size. Frames may split across reads and
// several may arrive in one; the reassembler handles both, so this is only a
// syscall-sizing choice.
const readChunkBytes = 4096

// Radio is a simulated Icom IC-905 speaking binary CI-V over an in-memory
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
	identityToken     []byte
	latency           time.Duration
	broadcastInterval time.Duration
	broadcastFrame    []byte
	addressedInterval time.Duration
	addressedFrame    []byte

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

// New constructs a simulated IC-905 and starts its servicing goroutines.
//
// Without options it holds the default image (image.go) and answers 19 00 with
// the constructor's own arbitrary identity token. Both of those are inventions,
// recorded in doc.go's ASSUMED register rather than presented as facts about
// any radio.
//
// Close it when done: a Radio owns goroutines and a pipe.
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()

	r := &Radio{
		hostConn:      hostConn,
		fakeConn:      fakeConn,
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
// group and channel are the two printed two-byte address fields as numbers —
// see WithRecord and bcd2.
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
// assert that a refused write put nothing on the wire.
//
// RECEIVED, not sent: a flood running beside the traffic never appears here.
// Every frame the reassembler completed appears, including ones the fake said
// nothing to — a frame addressed to some other radio is silently ignored on the
// wire but is recorded here, because "the fake never saw it" and "the fake saw
// it and held its tongue" are different facts and a consumer may need to tell
// them apart. An over-length run is refused before it is ever a frame, so it
// has none to record.
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
// and drives both floods.
//
// THE FLOODS ARE TICKER CASES IN THE SAME SELECT AS REQUEST HANDLING, which is
// the whole of the brief's "a flood must interleave with, not wait for, request
// handling". Nothing in this loop blocks on a request: reading happens in
// readLoop, and a scripted latency is a scheduled timer rather than a sleep, so
// the tickers keep firing throughout both.
func (r *Radio) serve() {
	defer r.wg.Done()

	broadcastC, stopBroadcast := tickerFor(r.broadcastInterval)
	defer stopBroadcast()
	addressedC, stopAddressed := tickerFor(r.addressedInterval)
	defer stopAddressed()

	for {
		select {
		case <-r.shutdown:
			return

		case ev, ok := <-r.events:
			if !ok {
				return
			}
			if reply := r.handleEvent(ev); reply != nil {
				r.scheduleReply(reply)
			}

		case <-r.replyReady:
			if b := r.popReply(); b != nil {
				r.rawWrite(b)
			}

		case <-broadcastC:
			r.rawWrite(r.broadcastFrame)

		case <-addressedC:
			r.rawWrite(r.addressedFrame)
		}
	}
}

// tickerFor returns a tick channel and its stop function. A non-positive
// interval yields a nil channel, which blocks forever in a select — the
// idiomatic way to leave a case switched off.
func tickerFor(interval time.Duration) (<-chan time.Time, func()) {
	if interval <= 0 {
		return nil, func() {}
	}
	t := time.NewTicker(interval)
	return t.C, t.Stop
}

// scheduleReply sends a reply, after the configured latency if there is one.
//
// With no latency the reply goes straight out. With one, the bytes join a FIFO
// queue and a timer signals the serve loop when their wait is over — never a
// sleep inside the loop, which would stop the floods and stall the next
// request. Because the queue preserves order, replies are written in the order
// they were produced regardless of how the timers happen to fire.
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
