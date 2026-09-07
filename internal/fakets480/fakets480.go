// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated TS-480: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	// The fields below are populated only while New's options run and never
	// mutated afterwards, so serve() and the parser may read them without
	// r.mu.
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
	r := &Radio{
		pipe:         fakepipe.New(),
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
	r.pipe.Write(data)
}
