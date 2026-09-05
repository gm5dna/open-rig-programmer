// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Row is the registry row a *Radio plays. The TS-590S and the TS-590SG share
// one PC-command document and one 50-byte memory grid, so a single fake
// serves both — but WHICH one it is has to be said, and the zero value says
// nothing.
//
// THE ROW IS REQUIRED AND HAS NO DEFAULT (the plan's P1, which makes the
// model argument required on the driver and on the fake alike). The two rows
// differ on the ID answer — "021: TS-590S" and "023: TS-590SG"
// (590:1114-1116) — and on byte 28's liveness (590:1478), and a zero value
// that quietly meant one of them would make every test of the other a fixture
// accident.
type Row int

// The two rows, plus the refusing default.
const (
	RowUnset Row = iota
	// RowS is the TS-590S: ID "021" (590:1114).
	RowS
	// RowSG is the TS-590SG: ID "023" (590:1116).
	RowSG
)

// String renders r as the registry key it stands for, and the unset value as
// its own constant name so that a refusal reads as a programming error rather
// than as a radio.
func (r Row) String() string {
	switch r {
	case RowS:
		return "TS-590S"
	case RowSG:
		return "TS-590SG"
	default:
		return "RowUnset"
	}
}

// idAnswer returns the five-byte ID answer this row gives: "I D P1 P1 P1 ;"
// (590:1119) carrying the row's own printed number (590:1114-1116).
func (r Row) idAnswer() string {
	switch r {
	case RowS:
		return "ID021;"
	case RowSG:
		return "ID023;"
	default:
		return ""
	}
}

// Radio is a simulated TS-590S or TS-590SG: an in-memory duplex pipe
// presenting the host end via Port(), serviced from the Radio's own goroutine
// using this package's independent parser (parser.go). A Radio is safe for
// concurrent use: its inspection methods and Close may all be called from
// goroutines other than whatever is reading or writing Port() (run tests with
// -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// row is fixed at construction and never mutated, so every reader may
	// take it without r.mu.
	row Row

	// The fields below are populated only while New's options run and never
	// mutated afterwards, so serve() and the parser may read them without
	// r.mu.
	latency                time.Duration
	firmware               string
	memoryReadUnsupported  bool
	transientNAKSuppressed bool
	streamErrors           map[int]StreamError

	// exchanges counts the frames serve() has handled, 1-based, and is
	// touched by serve() alone — the only goroutine that ever reads or
	// writes fakeConn. It indexes the scripted stream errors.
	exchanges int

	mu      sync.Mutex
	records map[recordKey]MemState
	// currentChannel is what an "MC;" reports. It moves only by an MC Set;
	// nothing here models a front panel.
	currentChannel int
	// ai is '0', '2' or '4' — the whole of this book's AI legend
	// (590:159-162). OFF at construction, a MANUAL FACT: "Turn this
	// function on using the AI command (the initial state is OFF)"
	// (590:81-82).
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

// New constructs a *Radio for row and starts its servicing goroutine.
// Without a WithFactoryImage option the record map defaults to
// DefaultImage().
//
// It PANICS on a row outside {RowS, RowSG} — MustNewLayout's reasoning one
// layer down. Every call site passes a compile-time-known constant, so an
// unset row is a programming error in a fixture and must stop the programme
// rather than be threaded through an error return no caller could act on.
// TestNew_RefusesAnUnsetRow pins it.
func New(row Row, opts ...Option) *Radio {
	if row != RowS && row != RowSG {
		panic(fmt.Sprintf("fakets590: New called with row %v — the row is REQUIRED and has no default (the two siblings differ at ID, 590:1114-1116, and at byte 28, 590:1478)", row))
	}
	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn: hostConn,
		fakeConn: fakeConn,
		row:      row,
		// The book's ONE worked example of an FV answer, "for firmware
		// version 1.00, it reads 'FV1.00;'" (590:1035). It is a default
		// rather than a claim about any radio: WithFirmwareVersion is how a
		// test reaches the versions A13 and A14 turn on.
		firmware:     defaultFirmware,
		streamErrors: map[int]StreamError{},
		records:      DefaultImage(),
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
// models a sentence of the book's own error table (590:93-113) rather than a
// misbehaving radio:
//
//   - a scripted STREAM ERROR replaces this exchange's reply with "E;" or
//     "O;" (WithStreamError);
//   - WithTransientNAKSuppressed drops a "?;" that would otherwise be sent,
//     which is the Note printed under the "?;" row itself: "Occasionally,
//     this message may not appear due to microprocessor transients in the
//     transceiver." (590:106-108).
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
