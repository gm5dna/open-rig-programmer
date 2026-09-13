// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx5000

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// catID is this radio's fixed CAT ID answer — "0362" (ID's own P1 legend,
// layout:770, matrix §4). Fixed, not an Option: this package builds one row
// with a bare New (matrix §0), so there is nothing for a model option to
// select between.
const catID = "0362"

// maxAccumulatorBytes bounds the un-terminated byte count the frame
// accumulator will hold before dropping and resyncing. See doc.go's ASSUMED
// register entry 3. 256 is generous headroom over the longest frame this
// package ever sends or receives (27 bytes, the MW Set), while still
// bounding a runaway peer's memory cost.
const maxAccumulatorBytes = 256

// MemState is one memory channel's stored record — every field MR/MW carry
// except the slot number itself (the map key) and the terminator. It is
// this package's own independent encoding of matrix §2's byte table, not a
// copy of any project type.
type MemState struct {
	FreqHz     uint32 // P2, 8 ASCII digits, 0-99999999
	ClarSign   byte   // P3 direction: '+' or '-'
	ClarHz     uint16 // P3 magnitude, 0-9999
	RxClar     bool   // P4
	TxClar     bool   // P5
	Mode       byte   // P6, one of "123456789ABC" (matrix §1.3)
	CTCSSState byte   // P8: '0', '1' or '2'
	ToneIndex  uint8  // P9, 0-49 (matrix §1.4)
	Shift      byte   // P10: '0', '1' or '2'
}

// zeroState is the ASSUMED answer for a channel that has never been
// written — doc.go register entry 1, EMPTY-SLOT ANSWERS.
var zeroState = MemState{ClarSign: '+', Mode: '1', CTCSSState: '0', Shift: '0'}

// Radio is a simulated FTdx5000: an in-memory duplex pipe presenting the
// host end via Port(), serviced from the Radio's own goroutine using this
// package's independent parser (parser.go).
type Radio struct {
	pipe *fakepipe.Pipe
	acc  *reassembler

	mu    sync.Mutex
	slots map[string]MemState
}

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// New constructs a simulated FTdx5000 and starts its servicing goroutine.
// Bare New, no model argument: this package builds one row (matrix §0).
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:  fakepipe.New(),
		acc:   newReassembler(maxAccumulatorBytes),
		slots: map[string]MemState{},
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

// Close shuts the fake radio down: it closes the radio's own end of the
// pipe and waits for the servicing goroutine to exit. Safe to call more
// than once.
func (r *Radio) Close() error { return r.pipe.Close() }

// SlotState returns the stored record for slot and whether one has ever
// been written. An unwritten slot is not reported as zeroState here — that
// substitution happens only when building an MR answer (doc.go register
// entry 1) — so a test can tell "never written" apart from "written as all
// zeros".
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames on the ';' terminator (layout:110), and drives command handling
// and replies. It is the only goroutine that ever reads or writes the
// pipe, so no synchronisation is needed around the connection itself —
// only around slots, which SlotState and WithSlot also touch.
func (r *Radio) serve() {
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, frame := range r.acc.push(b) {
				if reply := r.handleFrame(frame); reply != nil {
					r.pipe.Write(reply)
				}
			}
		})
	})
}
