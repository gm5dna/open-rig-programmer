// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import (
	"io"
	"net"
	"sync"
	"time"
)

// Radio is a simulated FTdx10: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: SlotState, CurrentChannel and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// latency is populated only while New's options run and never mutated
	// afterwards, so serve() may read it without r.mu.
	latency time.Duration

	mu             sync.Mutex
	slots          map[string]MemState
	currentChannel string
	ai             byte              // '0' or '1'; ASSUMED OFF at construction — doc.go register entry 14
	exSettings     map[string]string // EX (MENU) six-digit address -> raw P4; see ex.go

	// shutdown is closed (exactly once, by closePipes) when the radio goes
	// away. WithLatency's wait selects against it (sleepInterruptible)
	// instead of calling bare time.Sleep, so Close never has to wait out a
	// pending scripted delay before its wg.Wait on serve() can return — the
	// promptness internal/wiring's OpenFakeSessionFor relies on for the
	// FT-710's fake and now for this one.
	shutdown chan struct{}

	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the slot map defaults to DefaultImage().
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn: hostConn,
		fakeConn: fakeConn,
		slots:    DefaultImage(),
		// The answer-only none form: what "MC;" reports before any MC-set
		// has happened. The wire spelling is the DIALECT's ASSUMED
		// NoneWire (core/cat/ftdx10's register entry 3), cited not
		// re-derived — it appears in no FTdx10 slot legend.
		currentChannel: slotNoneWire,
		// ASSUMED OFF — doc.go register entry 14: New models a
		// freshly-powered radio.
		ai: '0',
		// The generated projection of transcription B, expanded to
		// address -> raw P4. The VALUES are invented (doc.go register
		// entry 4); EXDefaults returns a fresh map per call, so this
		// radio's menu state is its own. There is no runtime/manual
		// split as fakeradio has — no FTdx10 has ever been asked
		// anything, so there is nothing to overlay (see EXDefaults).
		exSettings: EXDefaults(),
		shutdown:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}

	r.wg.Add(1)
	go r.serve()
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.hostConn }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than once.
//
// Deliberately closes only fakeConn, not hostConn — fakeradio's reasoning,
// verbatim, because it is a property of net.Pipe rather than of either radio:
// Close() only reports io.ErrClosedPipe to a Read or Write made against the
// END YOU YOURSELF closed, while a pending or subsequent Read on the other
// (still-open) end sees io.EOF, which is exactly the signal a host should get
// from "the radio went away". Closing hostConn here too would flip that to
// io.ErrClosedPipe for the caller, a worse and less consistent signal. A
// caller that wants to release its own Port() handle explicitly may still
// call r.Port().Close() itself.
func (r *Radio) Close() error {
	err := r.closePipes()
	r.wg.Wait()
	return err
}

// closePipes is the idempotent, race-safe close of the radio's own pipe end.
// Factored out of the public Close() so that anything running INSIDE serve()
// can shut the pipe without deadlocking: Close() waits on r.wg, which only
// reaches zero once serve() has returned.
func (r *Radio) closePipes() error {
	r.closeOnce.Do(func() {
		// Close shutdown FIRST: a serve goroutine parked in a latency wait
		// wakes immediately, before (or regardless of) noticing the pipe
		// itself closing.
		close(r.shutdown)
		r.closeErr = r.fakeConn.Close()
	})
	return r.closeErr
}

// sleepInterruptible waits d, returning early (false) if the radio's shutdown
// channel closes first. Returns true when the full d genuinely elapsed; d <= 0
// returns true at once.
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
// frames, and drives command handling and replies. It is the ONLY goroutine
// that ever reads or writes fakeConn (see rawWrite), so no synchronisation is
// needed around the connection itself — only around the shared state behind
// Radio.mu, which SlotState and CurrentChannel also touch from test
// goroutines.
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

// handleEvent processes one reassembler event — a complete frame, or an
// accumulator overflow — and sends whatever reply it produces.
//
// A nil reply is silence, and silence is SUCCESS for every Set this radio
// takes (doc.go register entry 11): the accepted-Set path and the
// nothing-to-say path are the same path, deliberately, so a handler cannot
// acknowledge a Set by accident.
func (r *Radio) handleEvent(ev accEvent) {
	var reply []byte
	if ev.overflow {
		reply = rejection
	} else {
		reply = r.handleFrame(ev.frame)
	}
	if reply == nil {
		return
	}
	r.rawWrite(reply)
}

// rawWrite sends data to the port, honouring the configured per-reply
// latency. Errors are not reported: a write failing because the peer has gone
// away (closed the port, or stopped reading) is an expected outcome, not a bug
// in the fake. The latency wait is interruptible — a Close mid-wait abandons
// the write, since the pipe is gone and the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	if r.latency > 0 && !r.sleepInterruptible(r.latency) {
		return
	}
	_, _ = r.fakeConn.Write(data)
}
