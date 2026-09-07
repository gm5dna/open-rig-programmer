// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import (
	"io"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated Icom IC-705 speaking CI-V: an in-memory duplex pipe
// presenting the host end via Port(), serviced from the Radio's own goroutine
// through this package's independent parser (parser.go).
//
// A Radio is safe for concurrent use. SlotState, FramesSeen, SetsSeen,
// AnswerNextReadWithAddress and Close may all be called from goroutines other
// than whatever is reading or writing Port() (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing it:
	// internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	// emitters is populated only while New's options run, before any goroutine
	// starts, and never mutated afterwards — so the emitters may read it
	// without r.mu. (The reply latency lives on pipe, set the same way.)
	emitters []emitter

	mu           sync.Mutex
	slots        Image
	framesSeen   int
	setsSeen     int
	wrongAddress *Slot // armed by AnswerNextReadWithAddress; nil when unarmed

}

// emitter is one configured stream of unsolicited frames: what it addresses
// them to, and how often it sends one. A zero interval means CONTINUOUSLY —
// paced only by whatever is reading the port.
type emitter struct {
	to    byte
	every time.Duration
}

// New constructs a simulated IC-705 and starts its servicing goroutine.
//
// A radio built with no options is EMPTY — no memory slot holds anything, and
// every read draws NG. That is deliberate, and it is this package's one
// structural departure from internal/fakedx101, whose New seeds a default
// image: the design's probe treats an all-NG search as a real and expected case
// (an unprogrammed radio), and a default image would put records in slots no
// test asked for, competing silently with whatever that test seeded. A caller
// who wants a populated radio says so — WithFactoryImage(DefaultImage()).
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:  fakepipe.New(),
		slots: EmptyImage(),
	}
	for _, opt := range opts {
		opt(r)
	}

	r.serve()

	for _, e := range r.emitters {
		r.pipe.Go(func() { r.emit(e) })
	}
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
// terminated runs, and drives handling and replies.
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

// handleEvent processes one reassembler event — a terminated run, or the
// accumulator filling without one — and sends whatever reply it produces.
//
// An overflow draws exactly one NG. That is this package's own bounded-input
// policy (register entry 6), not a radio claim.
func (r *Radio) handleEvent(ev accEvent) {
	var reply []byte
	if ev.overflow {
		reply = buildNAK()
	} else {
		reply = r.handleFrame(ev.frame)
	}
	if reply == nil {
		return
	}
	r.rawWrite(reply)
}

// emit runs one configured stream of unsolicited frames until the radio closes.
//
// A zero interval sends continuously: net.Pipe is unbuffered, so each Write
// blocks until the far end reads it, and the flood is paced by whatever is
// draining the port rather than by a timer. A closed pipe fails the Write and
// ends the goroutine, which is how Close stops a flood that nobody is reading.
//
// Writes from here and from serve() cannot interleave mid-frame: net.Pipe
// serialises whole Writes against one another, so a reader sees complete
// frames in some order rather than two frames shuffled together.
func (r *Radio) emit(e emitter) {
	frame := buildUnsolicited(e.to)
	for {
		select {
		case <-r.pipe.Done():
			return
		default:
		}
		if !r.pipe.WriteNow(frame) {
			return
		}
		if !r.pipe.Sleep(e.every) {
			return
		}
	}
}

// rawWrite sends bytes to the port, honouring the configured per-reply latency.
//
// Errors are not reported: a write failing because the peer has gone away —
// closed the port, or stopped reading — is an expected outcome of a test
// ending, not a bug in the fake. The latency wait is interruptible, since a
// Close mid-wait means the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}
