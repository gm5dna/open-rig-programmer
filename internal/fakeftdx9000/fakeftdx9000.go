// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated FTdx9000: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package every fake shares. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all.
	pipe *fakepipe.Pipe

	mu             sync.Mutex
	slots          map[string]MemState
	currentChannel string
	ai             byte // '0' or '1'; OFF at construction, a MANUAL FACT (register entry 8)
	catID          string
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the slot map defaults to DefaultImage().
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:  fakepipe.New(),
		slots: DefaultImage(),
		// The answer-only "no selection" wire form: what "MC;" reports before
		// any MC-set has happened. "000" is never a valid recall or write
		// target (parseSlotForm refuses it as slotNone), matching the
		// out-of-range placeholder every sibling fake's own SlotSpace.NoneWire
		// uses; this package cannot cite that fact by name since it lives in
		// the forbidden core/driver/ftdx9000, so it is this fake's own choice.
		currentChannel: slotNoneWire,
		// Register entry 8: OFF at construction, this radio's own manual fact.
		ai: '0',
		// Register entry 10: the default of three legal ID answers.
		catID: catIDDefault,
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
// the connection itself — only around the shared state behind Radio.mu, which
// the inspection methods also touch from test goroutines.
//
// ITS ONLY WRITE IS A REPLY (register entry 8): nothing in this loop can
// originate a frame, so an AI-on session and an AI-off session are
// byte-identical on the wire.
func (r *Radio) serve() {
	acc := newReassembler(maxAccumulatorBytes)
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
// takes (register entry 1): the accepted-Set path and the nothing-to-say path
// are the same path, deliberately, so a handler cannot acknowledge a Set by
// accident.
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
// away is an expected outcome, not a bug in the fake.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
