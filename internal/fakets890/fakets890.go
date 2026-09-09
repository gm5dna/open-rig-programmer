// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated TS-890S: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this package's
// independent parser (parser.go). A Radio is safe for concurrent use: its
// inspection methods and Close may all be called from goroutines other than
// whatever is reading or writing Port() (run tests with -race).
//
// THERE IS NO ROW ARGUMENT, unlike internal/fakets590's New. That package
// serves two registry rows out of one book and cannot know which without being
// told; this one serves the single row its book describes, and its sibling
// TS-990S is a separate package because the two books print different grids.
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing
	// it: internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and
	// NO protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	// The fields below are populated only while New's options run and never
	// mutated afterwards, so serve() and the parser may read them without
	// r.mu.
	memoryReadUnsupported  bool
	transientNAKSuppressed bool
	streamErrors           map[int]StreamError

	// exchanges counts the frames serve() has handled, 1-based, and is
	// touched by serve() alone — the only goroutine that ever reads or
	// writes the pipe. It indexes the scripted stream errors.
	exchanges int

	mu      sync.Mutex
	records map[int]MemState
	// exSettings is this radio's menu state: five-character wire address ->
	// raw P5. It starts as EXDefaults() (exinventory.go's projection of this
	// package's own copy of transcription B) and moves only by
	// WithEXSetting/WithEXUnavailable at construction; nothing here models an
	// EX Set, and nothing models a front panel.
	exSettings map[string]string
	// ai is '0', '2' or '4' — the three values this book's AI legend prints
	// with a meaning (890:175-181). It starts OFF, which is this package's
	// own choice rather than a printed one: unlike the TS-590's book, this
	// one prints no power-on AI state anywhere — doc.go's register entry THE
	// AI STATE AT CONSTRUCTION.
	ai byte
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the record map defaults to DefaultImage().
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:         fakepipe.New(),
		streamErrors: map[int]StreamError{},
		records:      DefaultImage(),
		// EXDefaults returns a fresh map, so two radios never share menu
		// state.
		exSettings: EXDefaults(),
		ai:         aiOff,
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
// models a sentence of this book's own error table (890:98-123) rather than a
// misbehaving radio:
//
//   - a scripted STREAM ERROR replaces this exchange's reply with "E;" or
//     "O;" (WithStreamError);
//   - WithTransientNAKSuppressed drops a "?;" that would otherwise be sent,
//     which is the Note printed under the "?;" row itself: "Occasionally,
//     this message may not appear due to microprocessor transients in the
//     transceiver." (890:114-116).
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

// rawWrite sends data to the port, honouring the configured per-reply latency.
// Errors are not reported: a write failing because the peer has gone away
// (closed the port, or stopped reading) is an expected outcome, not a bug in
// the fake. The latency wait is interruptible — a Close mid-wait abandons the
// write, since the pipe is gone and the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
