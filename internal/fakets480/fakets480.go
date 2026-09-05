// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"io"
	"net"
	"sync"
	"time"
)

// Radio is a simulated TS-480: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// The fields below are populated only while New's options run and never
	// mutated afterwards, so serve() and the parser may read them without
	// r.mu.
	latency                time.Duration
	tyReserved             string
	tyVariant              byte
	memoryReadUnsupported  bool
	transientNAKSuppressed bool
	streamErrors           map[int]StreamError

	// exchanges counts the frames serve() has handled, 1-based, and is
	// touched by serve() alone — the only goroutine that ever reads or
	// writes fakeConn. It indexes the scripted stream errors.
	exchanges int

	mu      sync.Mutex
	records map[recordKey]MemState
	// exSettings is this radio's menu state: three-digit wire address ->
	// raw P5. It starts as EXDefaults (ex.go) and moves only by
	// WithEXSetting/WithEXUnavailable at construction; nothing here models
	// an EX Set, and nothing models a front panel.
	exSettings map[string]string
	// currentChannel is what an "MC;" reports. It moves only by an MC Set;
	// nothing here models a front panel.
	currentChannel int
	// ai is '0', '1', '2' or '3' — the whole of this book's AI legend
	// (480:185-190). It is '0' at construction; see doc.go's register entry
	// THE INITIAL AI VALUE IS THE POWER-OFF ONE.
	ai byte

	// shutdown is closed (exactly once, by closePipes) when the radio goes
	// away. WithLatency's wait selects against it (sleepInterruptible)
	// instead of calling bare time.Sleep, so Close never has to wait out a
	// pending scripted delay before its wg.Wait on serve() can return — the
	// promptness internal/wiring's OpenFakeSessionFor relies on for every
	// fake rig, pinned by TestClose_IsPromptDespiteAPendingLatency.
	shutdown chan struct{}

	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the record map defaults to DefaultImage().
//
// IT TAKES NO ROW ARGUMENT, where internal/fakets590's New requires one. The
// 2003 document describes ONE PC-control radio and prints ONE identity for
// it, "020: TS-480" (480:678); the HX/SAT split it does print is a HARDWARE
// VARIANT reported by TY (480:1626-1629), not a second registry row, and
// decision 4 is explicit that the variant digit never selects a row. The 590
// pair need a required row because their two ID numbers and their byte 28
// differ; nothing here differs.
func New(opts ...Option) *Radio {
	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn:     hostConn,
		fakeConn:     fakeConn,
		tyReserved:   defaultTYReserved,
		tyVariant:    defaultTYVariant,
		streamErrors: map[int]StreamError{},
		records:      DefaultImage(),
		// EXDefaults returns a fresh map, so two radios never share menu
		// state (ex.go).
		exSettings: EXDefaults(),
		// A radio that has had no MC Set is sitting on SOME channel, and
		// this book prints no power-on value anywhere, so the fake takes the
		// lowest number its slot space has and says so — doc.go's register
		// entry THE SELECTED CHANNEL AT CONSTRUCTION.
		currentChannel: lowestChannel,
		ai:             aiOff,
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

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than once.
//
// Deliberately closes only fakeConn, not hostConn — internal/fakeradio's
// reasoning, verbatim, because it is a property of net.Pipe rather than of
// any radio: Close() only reports io.ErrClosedPipe to a Read or Write made
// against the END YOU YOURSELF closed, while a pending or subsequent Read on
// the other (still-open) end sees io.EOF, which is exactly the signal a host
// should get from "the radio went away". TestClose_HostSeesEOF pins the
// direction.
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
// Radio.mu, which the inspection methods also touch from test goroutines.
func (r *Radio) serve() {
	defer r.wg.Done()

	acc := newReassembler()
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
// takes (doc.go's register entry AN ACCEPTED SET PRODUCES NO REPLY): the
// accepted-Set path and the nothing-to-say path are the same path,
// deliberately, so a handler cannot acknowledge a Set by accident.
//
// TWO SCRIPTED DIVERSIONS SIT BETWEEN THE HANDLER AND THE WIRE, and each
// models a sentence of the book's own error table (480:126-144) rather than a
// misbehaving radio:
//
//   - a scripted STREAM ERROR replaces this exchange's reply with "E;" or
//     "O;" (WithStreamError);
//   - WithTransientNAKSuppressed drops a "?;" that would otherwise be sent,
//     which is the Note printed under the "?;" row itself: "Occasionally this
//     message may not appear due to microprocessor transients in the
//     transceiver." (480:136-138).
func (r *Radio) handleEvent(ev accEvent) {
	r.exchanges++

	if kind, ok := r.streamErrors[r.exchanges]; ok {
		r.rawWrite([]byte(kind.token()))
		return
	}

	var reply []byte
	if ev.overflow {
		reply = rejection
	} else {
		reply = r.handleFrame(ev.frame)
	}
	if reply == nil {
		return
	}
	if r.transientNAKSuppressed && string(reply) == string(rejection) {
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
