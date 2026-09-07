// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated FT-891: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it
	// — internal/fakepipe, the one package the fakes share, and protocol-free
	// by construction (see doc.go, "A SIBLING ... not a refactor").
	pipe *fakepipe.Pipe

	// mtReadUnsupported is populated only while New's options run and never
	// mutated afterwards, so the parser may read it without r.mu.
	mtReadUnsupported bool

	mu             sync.Mutex
	slots          map[string]MemState
	exSettings     map[string]string // EX (MENU) four-digit address -> raw P4; see ex.go
	currentChannel string
	ai             byte // '0' or '1'; OFF at construction, a MANUAL FACT (ft891_layout.txt:231)

}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the slot map defaults to DefaultImage().
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:  fakepipe.New(),
		slots: DefaultImage(),
		// The menu is state of its own, seeded from the generated
		// inventory and NOT from the Image: an Image is the slot map and
		// nothing else, so WithFactoryImage leaves the menu at its
		// defaults (TestWithFactoryImage_LeavesTheMenuAlone). EXDefaults
		// returns a fresh map per call, so two fakes in one test binary
		// never share a menu.
		exSettings: EXDefaults(),
		// The answer-only none form: what "MC;" reports before any MC-set
		// has happened. The wire spelling is the DIALECT's ASSUMED
		// NoneWire (core/cat/ft891/doc.go's register entry
		// "SlotSpace.NoneWire = \"000\""), cited not re-derived — it
		// appears in no FT-891 slot legend.
		currentChannel: slotNoneWire,
		// OFF at construction: New models a freshly-powered radio, and this
		// radio's own manual says what that state is — "This parameter is set
		// to '0' (OFF) automatically when the transceiver is turned 'OFF'"
		// (ft891_layout.txt:231, in AI's own block at 226-235). A MANUAL
		// FACT, not an assumption; see doc.go's "What is NOT in this
		// register, and why". Pinned by TestAI_SetIsSilentAndReadReportsIt.
		ai: '0',
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
// Prompt despite a pending WithLatency wait, which internal/wiring's
// OpenFakeSessionFor relies on for every fake rig
// (TestClose_IsPromptDespiteAPendingLatency); the host end is deliberately
// left open so a pending read there sees io.EOF, the signal a host should get
// from "the radio went away" (TestClose_HostSeesEOF pins the direction).
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY goroutine
// that ever reads or writes the radio's end (see rawWrite), so no
// synchronisation is needed around the connection itself — only around the
// shared state behind Radio.mu, which the inspection methods also touch from
// test goroutines.
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
// takes (doc.go's register entry AN ACCEPTED SET PRODUCES NO REPLY): the
// accepted-Set path and the nothing-to-say path are the same path,
// deliberately, so a handler cannot acknowledge a Set by accident.
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
func (r *Radio) rawWrite(data []byte) { r.pipe.Write(data) }
