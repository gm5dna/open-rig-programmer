// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300mk2

import (
	"io"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// Radio is a simulated Icom IC-7300MK2: an in-memory duplex pipe presenting
// the host end via Port(), serviced from the Radio's own goroutine by this
// package's independent CI-V reassembler (civ.go) and record codec
// (record.go).
//
// A Radio is safe for concurrent use. Channel, Channels, Received, Sent and
// Close may all be called from goroutines other than whatever is reading or
// writing Port() (run the tests with -race).
type Radio struct {
	// pipe is the in-memory duplex connection and the goroutines servicing
	// it: internal/fakepipe, the one package the fakes share. It carries the
	// net.Pipe pair, the interruptible latency wait and the raw write, and NO
	// protocol at all — see doc.go's "A SIBLING, not a refactor".
	pipe *fakepipe.Pipe

	// The fields below are populated only while New's options run, before any
	// goroutine starts, and are never mutated afterwards. They may therefore
	// be read without mu.
	addr      byte          // this radio's CI-V address; see WithRadioAddress
	idToken   []byte        // the 19 00 answer's data area; nil means "the address"
	echo      bool          // echo every complete frame seen, bus-fashion
	broadcast time.Duration // WithTransceiveBroadcasts period; 0 disables
	flood     time.Duration // WithAddressedFlood period; 0 disables

	mu       sync.Mutex
	channels map[string][]byte
	received [][]byte
	sent     [][]byte

	// wmu serialises writes to the radio's pipe end. serve() is not the only writer: the
	// broadcast and flood goroutines write too, and two interleaved frames
	// would be an unparseable stream rather than a hard radio to talk to.
	wmu sync.Mutex
}

// New constructs a simulated IC-7300MK2 and starts its goroutines. The radio
// begins with NO channels stored: a read of any address answers the six-byte
// FAIL frame until something has been written to it, whether by a 1A 00 set on
// the wire or by WithChannel / WithRawChannel at construction.
func New(opts ...Option) *Radio {
	r := &Radio{
		pipe:     fakepipe.New(),
		addr:     defaultRadioAddr,
		channels: make(map[string][]byte),
	}
	for _, opt := range opts {
		opt(r)
	}

	r.serve()

	if r.broadcast > 0 {
		r.pipe.Go(func() { r.unsolicited(0x00, r.broadcast) })
	}
	if r.flood > 0 {
		r.pipe.Go(func() { r.unsolicited(controllerAddr, r.flood) })
	}
	return r
}

// Port returns the HOST end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.pipe.Host() }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutines to exit. Safe to call more than once.
//
// It is prompt despite a scripted latency or a broadcast period, and it
// deliberately leaves the HOST end open, so a read there sees io.EOF — exactly
// the signal a host should get from "the radio went away". See
// internal/fakepipe's Shutdown for why that direction matters.
func (r *Radio) Close() error { return r.pipe.Close() }

// ---------------------------------------------------------------------------
// Transcript and stored state
// ---------------------------------------------------------------------------

// Received returns every COMPLETE frame the radio saw, in order, as defensive
// copies. It includes frames the radio ignored because they were addressed to
// somebody else: a frame that reached the wire is recorded whether or not it
// was answered, which is what makes "this frame never reached the radio" an
// assertion a test can make.
//
// Each frame is NORMALISED to the grammar's two preamble bytes — leading noise
// is dropped and a run of three or more FE bytes is recorded as two — because
// what a caller wants to assert about is the frame, not the padding.
func (r *Radio) Received() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return copyFrames(r.received)
}

// Sent returns every frame the radio put on the wire, in order, as defensive
// copies: its answers, and — when the options that produce them are in force —
// the echo of each frame it saw (WithEcho), the unsolicited broadcasts
// (WithTransceiveBroadcasts) and the addressed flood (WithAddressedFlood).
//
// Sent() IS THE WIRE, in other words, not just the answers. With the default
// options, where none of those three is on, the two readings coincide. A frame
// whose write was abandoned — the peer stopped reading, or the radio was
// closed mid-write — is not recorded, because it never arrived.
func (r *Radio) Sent() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return copyFrames(r.sent)
}

// Channel returns the record stored for a slot, as a defensive copy, and
// whether anything is stored there at all. addr is the canonical slot string:
// "001" … "099", "P1" or "P2" (case and surrounding space are forgiven).
//
// The length of the returned record is whatever was stored: recordLen (45) for
// anything written over the wire or seeded by WithChannel, and ANY length for
// a slot seeded by WithRawChannel.
func (r *Radio) Channel(addr string) ([]byte, bool) {
	slot, ok := canonicalSlot(addr)
	if !ok {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.channels[slot]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), rec...), true
}

// Channels returns a copy of the whole stored map, keyed by canonical slot
// string, with every record copied too. Mutating the result cannot reach the
// radio.
func (r *Radio) Channels() map[string][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string][]byte, len(r.channels))
	for slot, rec := range r.channels {
		out[slot] = append([]byte(nil), rec...)
	}
	return out
}

func copyFrames(in [][]byte) [][]byte {
	out := make([][]byte, len(in))
	for i, f := range in {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

func (r *Radio) recordReceived(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = append(r.received, append([]byte(nil), frame...))
}

func (r *Radio) recordSent(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, append([]byte(nil), frame...))
}

func (r *Radio) storeChannel(slot string, rec []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[slot] = append([]byte(nil), rec...)
}

func (r *Radio) loadChannel(slot string) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.channels[slot]
	return rec, ok
}

// ---------------------------------------------------------------------------
// The serving goroutines
// ---------------------------------------------------------------------------

// serve starts the Radio's reading goroutine: it reads the pipe, reassembles
// frames and handles each one.
func (r *Radio) serve() {
	acc := &reassembler{max: maxFrameBytes}
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, frame := range acc.push(b) {
				r.handleFrame(frame)
			}
		})
	})
}

// handleFrame records one complete frame and answers it if it is addressed to
// this radio.
func (r *Radio) handleFrame(frame []byte) {
	r.recordReceived(frame)

	// WithEcho models the CI-V bus itself, which is one wire: a controller
	// hears its own transmission back. The echo therefore covers EVERY frame
	// the radio saw, including ones addressed elsewhere that it will not
	// answer, and it precedes the answer.
	if r.echo {
		r.writeWire(frame, 0)
	}

	body := bodyOf(frame)
	if len(body) < 2 || body[0] != r.addr {
		// Addressed to somebody else, or too short to carry an address at
		// all: counted (it is in Received()) and ignored.
		return
	}

	answer := r.answer(body)
	if answer == nil {
		return
	}
	if !r.pipe.Sleep(r.pipe.Latency) {
		return
	}
	r.writeWire(answer, 0)
}

// answer builds the reply to a frame already known to be addressed to this
// radio. It never returns nil: this radio acknowledges everything it is asked,
// with FB or with FA.
func (r *Radio) answer(body []byte) []byte {
	if len(body) < 3 {
		// FE FE <addr> <from> FD — addressed here, but carrying no command
		// byte at all. There is nothing to do and nothing was done.
		return r.fail()
	}
	cn, rest := body[2], body[3:]

	switch cn {
	case cmdIdentity:
		// 19 00, "sent with no data area". 19 with any other sub-command, or
		// 19 00 with a data area, is not the identity read.
		if len(rest) == 1 && rest[0] == subIdentity {
			return r.answerFrame([]byte{cmdIdentity, subIdentity}, r.token())
		}
		return r.fail()

	case cmdMemory:
		if len(rest) >= 1 && rest[0] == subMemory {
			return r.memory(rest[1:])
		}
		// 1A 05 and every other 1A sub-command: this radio does not have them.
		return r.fail()
	}

	// 0B, 18 00, 18 01, and everything else.
	return r.fail()
}

// memory handles the data area of a 1A 00 frame: the two channel-address bytes
// and whatever follows them.
func (r *Radio) memory(data []byte) []byte {
	if len(data) < 2 {
		return r.fail()
	}
	slot, ok := slotFromWire(data[0], data[1])
	if !ok {
		// Not one of the three address forms the ①, ② legend prints.
		return r.fail()
	}
	payload := data[2:]

	switch {
	case len(payload) == 0:
		// A read: "carries the two channel-address bytes and nothing more".
		rec, stored := r.loadChannel(slot)
		if !stored {
			return r.fail()
		}
		out := make([]byte, 0, 2+len(rec))
		out = append(out, data[0], data[1])
		out = append(out, rec...)
		return r.answerFrame([]byte{cmdMemory, subMemory}, out)

	case len(payload) == 1 && payload[0] == clearMarker:
		// The documented clear form, refused DELIBERATELY — including, and
		// especially, when slot is P1 or P2. See doc.go, "Why the clear forms
		// are refused".
		return r.fail()

	case len(payload) == recordLen:
		if !validRecord(payload) {
			return r.fail()
		}
		r.storeChannel(slot, payload)
		return r.pass()
	}

	// Any other length, the record-length rejection rule.
	return r.fail()
}

// token is the data area of the identity answer. WithIDToken fixes it; without
// that option it is one byte, this radio's own configured CI-V address.
func (r *Radio) token() []byte {
	if r.idToken != nil {
		return r.idToken
	}
	return []byte{r.addr}
}

// unsolicited emits a frequency report addressed to `to` every period, for as
// long as the radio lives. It is the engine behind WithTransceiveBroadcasts
// (to = 00) and WithAddressedFlood (to = E0).
func (r *Radio) unsolicited(to byte, period time.Duration) {
	for {
		if !r.pipe.Sleep(period) {
			return
		}
		frame := r.frameTo(to, []byte{cmdTransceive}, broadcastPayload)
		// A short write deadline, so that a caller which has stopped reading
		// cannot wedge the radio by parking the flood goroutine on wmu with
		// an answer queued behind it. A dropped unsolicited frame is what a
		// real bus does with a frame nobody was listening for.
		if !r.writeWire(frame, floodWriteDeadline(period)) {
			select {
			case <-r.pipe.Done():
				return
			default:
			}
		}
	}
}

// floodWriteDeadline is how long an unsolicited frame may block the write lock
// before it is abandoned: its own period, capped, so that a fast flood never
// holds the wire for long and a slow one still gets a fair chance to land.
func floodWriteDeadline(period time.Duration) time.Duration {
	const longest = 50 * time.Millisecond
	if period > longest {
		return longest
	}
	if period < time.Millisecond {
		return time.Millisecond
	}
	return period
}

// writeWire puts one whole frame on the wire under wmu and records it in
// Sent() if it fully arrived. deadline <= 0 means "block until the peer reads
// it or the radio closes" — the treatment answers get, since an answer that
// silently evaporated would be a much more confusing fake than one that waits.
//
// Errors are not reported: a write failing because the peer has gone away, or
// stopped reading, is an expected outcome and not a bug in the fake.
func (r *Radio) writeWire(frame []byte, deadline time.Duration) bool {
	r.wmu.Lock()
	defer r.wmu.Unlock()

	if deadline > 0 {
		_ = r.pipe.Conn().SetWriteDeadline(time.Now().Add(deadline))
		defer func() { _ = r.pipe.Conn().SetWriteDeadline(time.Time{}) }()
	}
	if !r.pipe.WriteNow(frame) {
		return false
	}
	r.recordSent(frame)
	return true
}
