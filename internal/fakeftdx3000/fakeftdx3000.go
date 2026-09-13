// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx3000

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated FTDX3000: an in-memory duplex pipe presenting the
// host end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing
	// it: internal/fakepipe, the one package the fakes share. It carries
	// the net.Pipe pair, the interruptible latency wait and the raw write,
	// and NO protocol at all — it is PROTOCOL-FREE, so a bug in it cannot
	// make a wrong codec look right (doc.go).
	pipe *fakepipe.Pipe

	mu             sync.Mutex
	slots          map[string]MemState
	currentChannel string
	// ai is P1 of the AI command, '0' or '1'. "This parameter is set to
	// '0' (OFF) automatically when the transceiver is turned 'OFF'"
	// (layout:213) — a MANUAL FACT, not an assumption, and the reason New
	// starts it at '0' rather than at a chosen default.
	ai byte
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the slot map defaults to DefaultImage(). There is
// no model option: this package's single row always answers ID as "0462"
// (doc.go).
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:  fakepipe.New(),
		slots: DefaultImage(),
		// The answer-only none form — doc.go's register entry THE "NO
		// SELECTION YET" CHANNEL IS "000".
		currentChannel: slotNoneWire,
		// OFF at construction — a MANUAL FACT (layout:213), not a chosen
		// default. See the Radio.ai field doc.
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

// Close shuts the fake radio down: it closes the RADIO's own end of the
// pipe and waits for the servicing goroutine to exit. Safe to call more
// than once. It deliberately leaves the HOST end open, so a pending or
// subsequent read there sees io.EOF — the signal a host should get from
// "the radio went away".
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY
// goroutine that ever reads or writes the pipe, so no synchronisation is
// needed around the connection itself — only around the shared state
// behind Radio.mu, which the inspection methods also touch from test
// goroutines.
//
// ITS ONLY WRITE IS A REPLY: nothing in this loop can originate a frame,
// so this fake never pushes anything unsolicited.
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
// accumulator overflow — and sends whatever reply it produces. A nil reply
// is silence, and silence is SUCCESS for every accepted Set this radio
// takes (doc.go: "What is NOT in this register" — the accepted-Set/
// nothing-to-say convention).
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
// latency. Errors are not reported: a write failing because the peer has
// gone away is an expected outcome, not a bug in the fake.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
