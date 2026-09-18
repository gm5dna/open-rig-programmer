// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated FTX-1: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go). A Radio is safe for concurrent
// use: its inspection methods and Close may all be called from goroutines
// other than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing
	// it: internal/fakepipe, the one package the fakes share. It carries
	// the net.Pipe pair, the interruptible latency wait and the raw write,
	// and NO protocol at all (doc.go).
	pipe *fakepipe.Pipe

	mu sync.Mutex
	// slots holds MR/MW's memory-block state, keyed by the 5-byte wire
	// address. tags holds MT's tag state, keyed the same way but in its
	// own map — doc.go's register entry MW AND MT MUTATE INDEPENDENT
	// FIELDS.
	slots map[string]MemState
	tags  map[string]string
	// ai is P1 of the AI command, '0' or '1'. No manual citation fixes a
	// power-on default for FTX-1 specifically; '0' (OFF) matches every
	// sibling fake's own choice.
	ai byte
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option the slot and tag maps default to DefaultImage().
// There is no body/model option: this package always answers ID as "0840"
// (doc.go, BODY IDENTITY IS NEVER SURFACED).
func New(opts ...Option) *Radio {
	slots, tags := DefaultImage()
	r := &Radio{
		pipe:  fakepipe.New(),
		slots: slots,
		tags:  tags,
		ai:    '0',
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
// subsequent read there sees io.EOF.
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY
// goroutine that ever reads or writes the pipe, so no synchronisation is
// needed around the connection itself — only around the shared state
// behind Radio.mu, which the inspection methods also touch from test
// goroutines.
//
// ITS ONLY WRITE IS A REPLY: nothing in this loop can originate a frame,
// so this fake never pushes anything unsolicited (doc.go's register entry
// AUTOMATIC-INFORMATION SUPPRESSION).
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
// is silence, the fire-and-forget convention every accepted MW Set takes.
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
