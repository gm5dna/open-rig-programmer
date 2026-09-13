// SPDX-License-Identifier: GPL-3.0-or-later

package fakets570

import (
	"net"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Half is the P1 byte: which half of a channel a frame names. '0' is the
// receive frequency (and, for channels 90-99, the scan-edge start
// frequency); '1' is the transmit frequency (scan-edge end). Matrix §1.2.
type Half byte

const (
	HalfRXOrStart Half = '0'
	HalfTXOrEnd   Half = '1'
)

func (h Half) valid() bool { return h == HalfRXOrStart || h == HalfTXOrEnd }

// recordKey names one stored half of one channel.
type recordKey struct {
	channel int
	half    Half
}

// MemState is one 28-byte record's fields, in wire form. Every field is a raw
// wire byte or byte run: this fake performs no unit conversion and applies no
// validation on the way OUT, so a caller building one directly (WithChannel)
// may script a deliberately malformed record.
type MemState struct {
	Freq     string // P4, 11 ASCII digits
	P2       byte   // P2, unused filler (register entry 3)
	Mode     byte   // P5, one ASCII digit '0'-'9'
	Lockout  byte   // P6, '0' or '1'
	ToneMode byte   // P7, '0' OFF or '1' ON
	ToneNo   string // P8, 2 ASCII digits
	P9       string // P9, 5 bytes, unused filler (register entry 3)
}

// zeroRecord is the answer this fake gives for a channel with no stored
// half — register entry 2. Every field takes the zero/OFF value its own
// legend prints; P2 and P9 take '0' fill per register entry 3.
func zeroRecord() MemState {
	return MemState{
		Freq:     "00000000000",
		P2:       '0',
		Mode:     '0',
		Lockout:  '0',
		ToneMode: '0',
		ToneNo:   "00",
		P9:       "00000",
	}
}

// Radio is a simulated TS-570D, TS-570S or TS-570DG: an in-memory duplex pipe
// presenting the host end via Port(), serviced from the Radio's own goroutine
// using this package's independent parser (parser.go). A Radio is safe for
// concurrent use (run tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing
	// it: internal/fakepipe, the one package the fakes share. See doc.go.
	pipe *fakepipe.Pipe

	// row and catID are fixed at construction and never mutated, so every
	// reader may take them without r.mu.
	row   string
	catID string

	// streamErrors is populated only while New's options run and never
	// mutated afterwards, so serve() may read it without r.mu — the same
	// shape the sibling Kenwood fakes use.
	streamErrors map[int]StreamError
	// exchanges counts frames/overflows serve() has handled, 1-based, and is
	// touched by serve()'s own goroutine alone — the only goroutine that
	// ever reads or writes the pipe. It indexes streamErrors.
	exchanges int

	mu      sync.Mutex
	records map[recordKey]MemState
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithModelName option the row is TS-570D — doc.go's register entry 1.
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:         fakepipe.New(),
		row:          "TS-570D",
		catID:        "017",
		streamErrors: map[int]StreamError{},
		records:      map[recordKey]MemState{},
	}
	for _, opt := range opts {
		opt(r)
	}
	r.serve()
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() net.Conn { return r.pipe.Host() }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than once.
func (r *Radio) Close() error { return r.pipe.Close() }

// SetChannel overlays one half of one channel, applied to whatever the
// records map already holds. No validation is applied — see MemState.
func (r *Radio) SetChannel(channel int, half Half, s MemState) {
	r.mu.Lock()
	r.records[recordKey{channel: channel, half: half}] = s
	r.mu.Unlock()
}

// ChannelState reports one half's stored record, and whether it has been
// written at all (as opposed to answering the zero record by default).
func (r *Radio) ChannelState(channel int, half Half) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	return s, ok
}

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY goroutine
// that ever reads or writes the pipe, so no synchronisation is needed around
// the connection itself — only around the shared state behind Radio.mu, which
// the inspection methods also touch from test goroutines.
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
// accumulator overflow — and sends whatever reply it produces. A nil reply is
// silence, and silence is SUCCESS for every accepted Set this radio takes:
// neither MW's chart nor the general command grammar prints an
// acknowledgement, matching the sibling Kenwood fakes' own reading.
//
// A SCRIPTED STREAM ERROR REPLACES THIS EXCHANGE'S REPLY ENTIRELY, including a
// fire-and-forget silent success — WithStreamError, doc.go register entry 7.
// This is now a live wire path per the lift-K follow-up (e7515d0): the
// manual's own "E;"/"O;" tokens are cited (printed folio 70), so this fake
// must be able to script them for the cross-check to drive core/driver/ts570's
// framing against a real fault.
func (r *Radio) handleEvent(ev accEvent) {
	r.exchanges++
	if kind, ok := r.streamErrors[r.exchanges]; ok {
		r.pipe.Write([]byte(kind.token()))
		return
	}

	var reply []byte
	if ev.overflow {
		reply = rejection
	} else {
		reply = r.handleFrame(ev.frame)
	}
	if reply != nil {
		r.pipe.Write(reply)
	}
}
