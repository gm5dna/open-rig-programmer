// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated FT-991A: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this package's
// independent parser (parser.go). A Radio is safe for concurrent use: its
// inspection methods and Close may all be called from goroutines other than
// whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	mu             sync.Mutex
	slots          map[string]MemState
	exSettings     map[string]string // EX (MENU) three-digit address -> raw P4; see ex.go
	currentChannel string
	ai             byte // '0' or '1'; OFF at construction, a MANUAL FACT (ft991a_layout.txt:242)

}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the slot map defaults to DefaultImage().
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:  fakepipe.New(),
		slots: DefaultImage(),
		// The menu state is SEPARATE from the slot map, and deliberately so:
		// WithFactoryImage replaces the slots and says nothing about the menu
		// (TestWithFactoryImage_LeavesTheMenuAlone). EXDefaults() returns a
		// fresh copy per call, so this Radio's map is its own — a later
		// WithEXSetting cannot reach the generated table or another Radio.
		exSettings: EXDefaults(),
		// The answer-only none form: what "MC;" reports before any MC-set has
		// happened. The wire spelling is the DIALECT's ASSUMED NoneWire
		// (core/cat/ft991a/doc.go's register entry `SlotSpace.NoneWire =
		// "000"`), cited not re-derived — it appears in no FT-991A slot
		// legend.
		//
		// This opening answer ("MC000;") is a frame core/cat's ParseMCAnswer
		// deliberately REJECTS (its mcParseValid ASSUMED rejection of "000"),
		// the same as internal/fakeradio's and internal/fakeft891's own
		// opening answers. That is fleet-wide posture, not a bug in this
		// fake: the FT-710's driver already turns that parse failure into
		// ErrMCSnapshotUnavailable (core/driver/ft710/mc.go) rather than
		// guessing, and this milestone's plan has any FT-991A analogue do
		// the same — skip the restore, never invent a recall target.
		currentChannel: slotNoneWire,
		// OFF at construction: New models a freshly-powered radio, and this
		// radio's own manual says what that state is — "This parameter is set
		// to '0' (OFF) automatically when the transceiver is turned 'OFF'"
		// (ft991a_layout.txt:242, in AI's own block at 237-246). A MANUAL
		// FACT, not an assumption; see doc.go's "What is NOT in this register,
		// and why". Pinned by TestAI_SetIsSilentAndReadReportsIt.
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
//
// ITS ONLY WRITE IS A REPLY, which is the whole of doc.go's register entry
// AUTOMATIC-INFORMATION SUPPRESSION: nothing in this loop can originate a
// frame, so an AI-on session and an AI-off session are byte-identical on the
// wire.
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
// takes: the accepted-Set path and the nothing-to-say path are the same path,
// deliberately, so a handler cannot acknowledge a Set by accident. That
// silence is the DIALECT's assumption, cited by name — "THE ACKNOWLEDGEMENT
// CONVENTIONS" (core/cat/ft991a/doc.go's register) — and core/driver/ft991a's
// write path depends on it, so a radio that acknowledged would break the
// driver and not merely this fake.
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

// rawWrite sends data to the port, honouring the configured per-reply latency.
// Errors are not reported: a write failing because the peer has gone away
// (closed the port, or stopped reading) is an expected outcome, not a bug in
// the fake. The latency wait is interruptible — a Close mid-wait abandons the
// write, since the pipe is gone and the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
