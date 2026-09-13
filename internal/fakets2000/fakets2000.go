// SPDX-License-Identifier: GPL-3.0-or-later

package fakets2000

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated TS-2000, TS-2000X or TS-B2000: an in-memory duplex
// pipe presenting the host end via Port(), serviced from the Radio's own
// goroutine using this package's independent parser (parser.go). A Radio is
// safe for concurrent use: its inspection methods and Close may all be
// called from goroutines other than whatever is reading or writing Port()
// (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package the fakes share. See doc.go's
	// sibling section for why that one import is safe.
	pipe *fakepipe.Pipe

	// model is fixed at construction (WithModelName) and never mutated, so
	// every reader may take it without r.mu. It changes ONLY what Model()
	// reports — doc.go's register entry 16: the wire carries no per-row
	// difference at all.
	model string

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
	records map[recordKey]MemState
	// currentChannel is what an "MC;" reports. It moves only by an MC Set;
	// nothing here models a front panel — doc.go's register entry 11.
	currentChannel int
	// ai is '0', '1', '2' or '3' — the whole of this book's AI legend
	// (ts2000:9662-9668). OFF at construction — doc.go's register entry 12.
	ai byte
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the record map defaults to DefaultImage(); without
// a WithModelName option the row is "TS-2000" — doc.go's register entry 16.
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:           fakepipe.New(),
		model:          "TS-2000",
		streamErrors:   map[int]StreamError{},
		records:        DefaultImage(),
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
func (r *Radio) Close() error { return r.pipe.Close() }

// Model reports the row this Radio was constructed as ("TS-2000" unless
// WithModelName said otherwise). It carries no wire meaning — see doc.go's
// register entry 16 — and exists purely for a test's own bookkeeping.
func (r *Radio) Model() string { return r.model }

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
// A nil reply is silence, and silence is SUCCESS for every Set this radio
// takes (doc.go's register entry 1): the accepted-Set path and the
// nothing-to-say path are the same path, deliberately.
//
// TWO SCRIPTED DIVERSIONS sit between the handler and the wire, each playing
// a sentence of the book's own error table (ts2000:9600-9618) rather than a
// misbehaving radio: a scripted STREAM ERROR (WithStreamError) replaces this
// exchange's reply with "E;" or "O;", and WithTransientNAKSuppressed drops a
// "?;" that would otherwise be sent, per the Note printed under the "?;" row
// itself (ts2000:9608-9610).
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
// latency. Errors are not reported: a write failing because the peer has
// gone away is an expected outcome, not a bug in the fake.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
