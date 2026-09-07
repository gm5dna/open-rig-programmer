// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"fmt"
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
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
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	// row is fixed at construction and never mutated, so every reader may
	// take it without r.mu.
	row Row

	// The fields below are populated only while New's options run and never
	// mutated afterwards, so serve() and the parser may read them without
	// r.mu.
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
	// exSettings is this radio's menu state: three-digit wire address ->
	// raw P5. It starts as this ROW's own EXDefaults (ex.go — the two
	// siblings' charts are two disjoint tables) and moves only by
	// WithEXSetting/WithEXUnavailable at construction; nothing here models
	// an EX Set, and nothing models a front panel.
	exSettings map[string]string
	// currentChannel is what an "MC;" reports. It moves only by an MC Set;
	// nothing here models a front panel.
	currentChannel int
	// ai is '0', '2' or '4' — the whole of this book's AI legend
	// (590:159-162). OFF at construction, a MANUAL FACT: "Turn this
	// function on using the AI command (the initial state is OFF)"
	// (590:81-82).
	ai byte
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
	r := &Radio{
		pipe: fakepipe.New(),
		row:  row,
		// The book's ONE worked example of an FV answer, "for firmware
		// version 1.00, it reads 'FV1.00;'" (590:1035). It is a default
		// rather than a claim about any radio: WithFirmwareVersion is how a
		// test reaches the versions A13 and A14 turn on.
		firmware:     defaultFirmware,
		streamErrors: map[int]StreamError{},
		records:      DefaultImage(),
		// THIS ROW's menu table, never the other's (ex.go). EXDefaults
		// returns a fresh map, so two radios never share menu state.
		exSettings: EXDefaults(row),
		// A radio that has had no MC Set is sitting on SOME channel, and
		// this book prints no power-on value anywhere, so the fake takes the
		// lowest number its slot space has and says so — doc.go's register
		// entry THE SELECTED CHANNEL AT CONSTRUCTION.
		currentChannel: lowestChannel,
		ai:             aiOff,
	}
	for _, opt := range opts {
		opt(r)
	}

	r.serve()
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.pipe.Host() }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than once.
//
// It is prompt despite a pending WithLatency wait — the promptness
// internal/wiring's OpenFakeSessionFor relies on for every fake rig — and it
// deliberately leaves the HOST end open, so a pending or subsequent read there
// sees io.EOF, exactly the signal a host should get from "the radio went
// away". See internal/fakepipe's Shutdown for why that direction matters.
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY goroutine
// that ever reads or writes the pipe (see rawWrite), so no synchronisation is
// needed around the connection itself — only around the shared state behind
// Radio.mu, which the inspection methods also touch from test goroutines.
func (r *Radio) serve() {
	acc := newReassembler()
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, ev := range acc.push(b) {
				r.handleEvent(ev)
			}
		})
	})
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
	r.pipe.Write(data)
}
