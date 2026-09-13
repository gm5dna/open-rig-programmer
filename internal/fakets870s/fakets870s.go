// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated TS-870S: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package the fakes share — see doc.go's
	// sibling section, THE HARD RULE.
	pipe *fakepipe.Pipe

	// Populated only while New's options run and never mutated afterwards,
	// so serve() and the parser may read them without r.mu.
	memoryReadUnsupported  bool
	transientNAKSuppressed bool
	streamErrors           map[int]StreamError

	// exchanges counts the frames serve() has handled, 1-based, and is
	// touched by serve() alone — the only goroutine that ever reads or
	// writes the pipe — so it indexes streamErrors without r.mu.
	exchanges int

	mu      sync.Mutex
	records map[recordKey]MemState
	// ai is '0', '1' or '2' — Format 32's whole AI legend. It is '0' at
	// construction: this book prints no power-on value for it either, and
	// '0' ("AI OFF") is the legend's own first value.
	ai byte
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the record map defaults to DefaultImage().
//
// IT TAKES NO ROW ARGUMENT. This book describes one PC-control radio and
// prints one identity for it (doc.go); there is no sibling row to select.
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:         fakepipe.New(),
		streamErrors: map[int]StreamError{},
		records:      DefaultImage(),
		ai:           aiOff,
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
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY goroutine
// that ever reads or writes the pipe, so no synchronisation is needed around
// the connection itself — only around the shared state behind Radio.mu,
// which the inspection methods also touch from test goroutines.
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
// A nil reply is silence, and silence is SUCCESS for every accepted Set this
// radio takes — doc.go's register entry AN ACCEPTED MW PRODUCES NO REPLY:
// the accepted-Set path and the nothing-to-say path are the same path,
// deliberately.
//
// A SCRIPTED STREAM ERROR REPLACES THIS EXCHANGE'S REPLY WITH "E;" OR "O;"
// (WithStreamError, STREAM ERRORS ARE SCRIPTABLE) — doc.go's register entry
// of that name, revised once a live session existed to interrupt.
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
// away is an expected outcome, not a bug in the fake.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
