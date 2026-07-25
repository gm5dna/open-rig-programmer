// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"io"
	"net"
	"sync"
	"time"
)

// Radio is a simulated FT-710: an in-memory duplex pipe presenting the
// host end via Port(), serviced from the Radio's own goroutine using
// fakeradio's independent parser (parser.go). A Radio is safe for
// concurrent use: SlotState, CurrentChannel, and Close may all be called
// from goroutines other than whatever is reading/writing Port() (run
// tests with -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// faults is populated only while New's options run, then never
	// mutated again — see faultConfig's doc comment for why serve() may
	// read it without r.mu.
	faults  faultConfig
	latency time.Duration

	mu             sync.Mutex
	slots          map[string]MemState
	exSettings     map[string]string // EX (MENU) address -> raw P4; see ex.go
	currentChannel string
	ai             byte // '0' or '1'; reference: "AI resets to OFF at radio power-off"
	exchangeN      int

	// shutdown is closed (exactly once, by closePipes) when the radio
	// goes away — a Close call, or a FaultDisconnect firing. Every
	// scripted delay (WithLatency, FaultDelayedReply,
	// FaultDelayedRejection) selects against it (sleepInterruptible)
	// instead of calling bare time.Sleep, so Close never has to wait out
	// a pending multi-second scripted delay before its wg.Wait on
	// serve() can return (M3 Codex-review fix wave, Fix 8).
	shutdown chan struct{}

	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option, the slot map defaults to ImageUK.
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn:       hostConn,
		fakeConn:       fakeConn,
		slots:          ImageUK(),
		exSettings:     EXRuntimeDefaults(),
		currentChannel: "000",
		ai:             '0',
		shutdown:       make(chan struct{}),
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

// Close shuts the fake radio down: closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than
// once (and safe to call after a FaultDisconnect has already closed the
// pipe from inside serve()).
//
// Deliberately closes only fakeConn, not hostConn: net.Pipe's Close()
// only reports io.ErrClosedPipe to a Read/Write call made against the END
// YOU YOURSELF closed — a pending or subsequent Read on the OTHER
// (still-open) end instead sees io.EOF, exactly the signal a host should
// get from "the radio went away". Closing hostConn here too would flip
// that to io.ErrClosedPipe for the caller, which is a worse, less
// consistent signal (and would no longer match FaultDisconnect's
// identical, EOF-producing internal close). A caller that wants to
// release its own Port() handle explicitly may still call
// r.Port().Close() itself.
func (r *Radio) Close() error {
	err := r.closePipes()
	r.wg.Wait()
	return err
}

// closePipes is the idempotent, race-safe close of the radio's own pipe
// end. It is called both by the public Close() and, internally, by
// serve() when FaultDisconnect fires — serve() must NEVER call the
// public Close() itself, since Close() waits on r.wg, which only reaches
// zero once serve() returns; calling it from inside serve() would
// deadlock.
func (r *Radio) closePipes() error {
	r.closeOnce.Do(func() {
		// Close shutdown FIRST: a serve goroutine currently parked in a
		// sleepInterruptible delay wakes immediately, before (or
		// regardless of) noticing the pipe itself closing.
		close(r.shutdown)
		r.closeErr = r.fakeConn.Close()
	})
	return r.closeErr
}

// sleepInterruptible waits d, returning early (false) if the radio's
// shutdown channel closes first — see Radio.shutdown (Fix 8). Returns
// true when the full d genuinely elapsed. d <= 0 returns true at once.
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
// frames, and drives command handling and replies. It is the ONLY
// goroutine that ever reads or writes fakeConn (see rawWrite), so no
// synchronisation is needed around the connection itself — only around
// the shared state in Radio.mu, which SlotState/CurrentChannel also
// touch from test goroutines.
func (r *Radio) serve() {
	defer r.wg.Done()

	acc := newReassembler(maxAccumulatorBytes)
	buf := make([]byte, 4096)
	for {
		n, err := r.fakeConn.Read(buf)
		if n > 0 {
			for _, ev := range acc.push(buf[:n]) {
				stop := r.handleEvent(ev)
				if stop {
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// handleEvent processes one reassembler event (a complete frame, or an
// accumulator overflow) as one fault-counted "exchange" (see faultConfig
// doc comment), and reports whether serve's read loop should stop (a
// FaultDisconnect fired).
func (r *Radio) handleEvent(ev accEvent) (stop bool) {
	var reply []byte
	if ev.overflow {
		reply = rejection
	} else {
		reply = r.handleFrame(ev.frame)
	}

	r.mu.Lock()
	r.exchangeN++
	n := r.exchangeN
	r.mu.Unlock()

	for _, sf := range r.faults.spurious {
		if sf.beforeN == n {
			r.rawWrite(sf.frame)
		}
	}

	// FaultDelayedReply: delay whatever reply is about to be sent below
	// (a normal answer, a "?;" from delayedRejectionN, or a natural
	// validation rejection) without changing its content. Spurious
	// frames above are unaffected — they are unconditional pushes, not
	// "the reply". Interruptible (Fix 8): a Close mid-delay skips the
	// reply — the pipe is gone, so it could never arrive anyway.
	if r.faults.delayedReplyN == n && r.faults.delayedReplyD > 0 {
		if !r.sleepInterruptible(r.faults.delayedReplyD) {
			return true
		}
	}

	switch {
	case r.faults.delayedRejectionN == n:
		if !r.sleepInterruptible(r.faults.delayedRejectionD) {
			return true
		}
		r.rawWrite(rejection)

	case reply != nil:
		out := reply
		if r.faults.garbleReplyN == n {
			out = garbleReply(out)
		}
		drop := r.faults.dropRepliesAfterN > 0 && n >= r.faults.dropRepliesAfterN
		if !drop {
			r.rawWrite(out)
		}
	}

	if r.faults.disconnectAfterN > 0 && n == r.faults.disconnectAfterN {
		r.closePipes()
		return true
	}
	return false
}

// garbleReply deterministically corrupts b (never mutating the caller's
// slice): it flips every bit of the first byte, leaving the frame's
// length and terminator intact, so a garbled reply is distinguishable
// from a truncated one.
func garbleReply(b []byte) []byte {
	out := append([]byte(nil), b...)
	if len(out) > 0 {
		out[0] ^= 0xFF
	}
	return out
}

// rawWrite sends data to the port, honouring the configured per-reply
// latency and chunking. Errors are not reported: a write failing because
// the peer has gone away (closed, or a FaultDisconnect already fired) is
// an expected outcome, not a bug in the fake. The latency wait is
// interruptible (Fix 8): a Close mid-wait abandons the write — the pipe
// is gone, so the bytes could never arrive anyway.
func (r *Radio) rawWrite(data []byte) {
	if r.latency > 0 && !r.sleepInterruptible(r.latency) {
		return
	}
	size := r.faults.chunkedSize
	if size <= 0 {
		_, _ = r.fakeConn.Write(data)
		return
	}
	for i := 0; i < len(data); i += size {
		end := i + size
		if end > len(data) {
			end = len(data)
		}
		if _, err := r.fakeConn.Write(data[i:end]); err != nil {
			return
		}
	}
}
