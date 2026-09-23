// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated FT-710: an in-memory duplex pipe presenting the
// host end via Port(), serviced from the Radio's own goroutine using
// fakeradio's independent parser (parser.go). A Radio is safe for
// concurrent use: SlotState, CurrentChannel, and Close may all be called
// from goroutines other than whatever is reading/writing Port() (run
// tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutine servicing
	// it: internal/fakepipe, the one package the fakes share. It carries
	// the net.Pipe pair, the interruptible latency wait and the raw write,
	// and NO protocol at all — see doc.go's sibling section.
	pipe *fakepipe.Pipe

	// faults is populated only while New's options run, then never
	// mutated again — see faultConfig's doc comment for why serve() may
	// read it without r.mu.
	faults faultConfig

	mu             sync.Mutex
	slots          map[string]MemState
	exSettings     map[string]string // EX (MENU) address -> raw P4; see ex.go
	exSetWidths    map[string]int    // EX (MENU) address -> characterised Set P4 width; see ex.go
	exSetStuck     map[string]bool   // EX (MENU) address -> Set accepted but never stored; test-only, see WithEXSetStuck
	currentChannel string
	ai             byte // '0' or '1'; reference: "AI resets to OFF at radio power-off"
	exchangeN      int
}

// New constructs a *Radio and starts its servicing goroutine. Without a
// WithFactoryImage option, the slot map defaults to ImageUK.
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:           fakepipe.New(),
		slots:          ImageUK(),
		exSettings:     EXRuntimeDefaults(),
		exSetWidths:    cloneEXSetWidths(),
		currentChannel: "000",
		ai:             '0',
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

// Close shuts the fake radio down: closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than
// once (and safe to call after a FaultDisconnect has already closed the
// pipe from inside serve()).
//
// It is prompt despite a pending scripted delay (WithLatency,
// FaultDelayedReply, FaultDelayedRejection), and it deliberately leaves the
// HOST end open, so a pending or subsequent read there sees io.EOF — exactly
// the signal a host should get from "the radio went away", and the same
// signal FaultDisconnect's internal close produces. See internal/fakepipe's
// Shutdown for why that direction matters.
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: it reads the pipe, reassembles
// frames, and drives command handling and replies. It is the ONLY
// goroutine that ever reads or writes the pipe (see rawWrite), so no
// synchronisation is needed around the connection itself — only around
// the shared state in Radio.mu, which SlotState/CurrentChannel also
// touch from test goroutines.
func (r *Radio) serve() {
	acc := newReassembler(maxAccumulatorBytes)
	stopped := false
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, ev := range acc.push(b) {
				if stopped {
					return
				}
				stopped = r.handleEvent(ev)
			}
		})
	})
}

// handleEvent processes one reassembler event (a complete frame, or an
// accumulator overflow) as one fault-counted "exchange" (see faultConfig
// doc comment), and reports whether serve's read loop should stop (a
// FaultDisconnect fired).
func (r *Radio) handleEvent(ev accEvent) (stop bool) {
	var reply []byte
	if ev.overflow {
		reply = rejection
	} else {
		reply = r.handleFrame(ev.frame)
	}

	r.mu.Lock()
	r.exchangeN++
	n := r.exchangeN
	r.mu.Unlock()

	for _, sf := range r.faults.spurious {
		if sf.beforeN == n {
			r.rawWrite(sf.frame)
		}
	}

	// FaultDelayedReply: delay whatever reply is about to be sent below
	// (a normal answer, a "?;" from delayedRejectionN, or a natural
	// validation rejection) without changing its content. Spurious
	// frames above are unaffected — they are unconditional pushes, not
	// "the reply". Interruptible (Fix 8): a Close mid-delay skips the
	// reply — the pipe is gone, so it could never arrive anyway.
	if r.faults.delayedReplyN == n && r.faults.delayedReplyD > 0 {
		if !r.pipe.Sleep(r.faults.delayedReplyD) {
			return true
		}
	}

	switch {
	case r.faults.delayedRejectionN == n:
		if !r.pipe.Sleep(r.faults.delayedRejectionD) {
			return true
		}
		r.rawWrite(rejection)

	case reply != nil:
		out := reply
		if r.faults.garbleReplyN == n {
			out = garbleReply(out)
		}
		if r.faults.garbleReplyPayload == n {
			out = garbleReplyPayload(out)
		}
		drop := r.faults.dropRepliesAfterN > 0 && n >= r.faults.dropRepliesAfterN
		if !drop {
			r.rawWrite(out)
		}
	}

	if r.faults.disconnectAfterN > 0 && n == r.faults.disconnectAfterN {
		r.pipe.Shutdown()
		return true
	}
	return false
}

// garbleReply deterministically corrupts b (never mutating the caller's
// slice): it flips every bit of the first byte, leaving the frame's
// length and terminator intact, so a garbled reply is distinguishable
// from a truncated one.
func garbleReply(b []byte) []byte {
	out := append([]byte(nil), b...)
	if len(out) > 0 {
		out[0] ^= 0xFF
	}
	return out
}

// garbleReplyPayload deterministically corrupts b (never mutating the
// caller's slice) inside the FIELD BLOCK rather than the command prefix:
// it flips every bit of the second-to-last byte — offset 26 (P10, the
// shift field) in a 28-byte MR/MW frame, per core/cat/memdata.go's
// memShiftOffset — leaving byte[0]/[1] (the prefix cat.PrefixLenMatcher
// checks) and the final ';' terminator intact. The frame is still
// accepted on the wire and still the correct length; it just decodes to
// an invalid shift character, which core/cat's parseMemoryFields refuses
// with a parse error the driver wraps as driver.ErrRecordDecode. See
// FaultGarbleReplyPayload's doc comment for why this differs from
// garbleReply above.
func garbleReplyPayload(b []byte) []byte {
	out := append([]byte(nil), b...)
	if len(out) >= 2 {
		out[len(out)-2] ^= 0xFF
	}
	return out
}

// rawWrite sends data to the port, honouring the configured per-reply
// latency and chunking. Errors are not reported: a write failing because
// the peer has gone away (closed, or a FaultDisconnect already fired) is
// an expected outcome, not a bug in the fake. The latency wait is
// interruptible (Fix 8): a Close mid-wait abandons the write — the pipe
// is gone, so the bytes could never arrive anyway.
func (r *Radio) rawWrite(data []byte) {
	size := r.faults.chunkedSize
	if size <= 0 {
		r.pipe.Write(data)
		return
	}
	if !r.pipe.Sleep(r.pipe.Latency) {
		return
	}
	for i := 0; i < len(data); i += size {
		end := min(i+size, len(data))
		if !r.pipe.WriteNow(data[i:end]) {
			return
		}
	}
}
