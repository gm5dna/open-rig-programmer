// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft900

import (
	"io"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

const frameLen = 5

// This family's opcodes (matrix §1's CAT Commands table). Only the ones
// this milestone's write choreography and read model actually use are
// named — everything else (SPLIT, Recall Memory, LOCK, M->VFO, UP/DOWN,
// HAM/GEN, Pacing, PTT, TUNER, START, A=B, Memory Scan Skip, Step Op
// Freq, Read Meter, Display Brightness, Read Flags) is unimplemented and
// falls through to silence, exactly as an unrecognised or out-of-range
// command does on the real radio (matrix §1.8, manual p.34).
const (
	opStore        byte = 0x03 // VFO->M: args [CH, P2, -, -]
	opABSelect     byte = 0x05 // args [V, -, -, -]; V=0 VFO-A, V=1 VFO-B
	opClarifier    byte = 0x09 // args [C1, C2, C3, C4]
	opSetFreq      byte = 0x0A // args: 4-byte packed BCD, LSB pair first
	opSetMode      byte = 0x0C // args [M, -, -, -]
	opStatusUpdate byte = 0x10 // args [U, -, -, CH]
	opShift        byte = 0x84 // args [R, -, -, -]; R=0/1/2 simplex/-/+
	opTone         byte = 0x90 // args [CC, -, -, -]
	opOffset       byte = 0xF9 // args [0x00, S2, S3, S4]
)

// U values for opStatusUpdate (matrix §1.8's Status Update Data
// Selection table).
const (
	uFullDump      byte = 0
	uMemoryNumber  byte = 1
	uOperatingData byte = 2
	uBothVFOs      byte = 3
	uMemoryRecord  byte = 4
)

// Radio is a simulated FT-900: an in-memory duplex pipe presenting the
// host end via Port(), serviced from the Radio's own goroutine. Safe for
// concurrent use (run tests with -race): mu guards every field the
// servicing goroutine and any inspection method both touch.
type Radio struct {
	pipe *fakepipe.Pipe

	mu        sync.Mutex
	inbuf     []byte
	activeVFO int // 0 or 1
	vfo       [2]vfoState
	channels  [slotCount]channel

	chunkFullDumpN int // 0 or 1 disables chunking
	chunkDelay     time.Duration
}

// New constructs a *Radio, seeds its 100 channels with a fixed non-zero
// rear sub-record each (state.go's seedRear — see doc.go, "not a
// preservation claim"), and starts its servicing goroutine.
func New(opts ...Option) *Radio {
	r := &Radio{pipe: fakepipe.New()}
	for i := range r.channels {
		r.channels[i].rear = seedRear(i)
	}
	for _, opt := range opts {
		opt(r)
	}
	r.serve()
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.pipe.Host() }

// Close shuts the fake radio down. Safe to call more than once.
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: the only one that ever reads or
// writes the pipe, so state (guarded by mu) is the only thing needing
// synchronisation against inspection methods called from test goroutines.
func (r *Radio) serve() {
	r.pipe.Go(func() {
		r.pipe.ReadLoop(r.handleBytes)
	})
}

// handleBytes groups the incoming stream into fixed 5-byte frames
// (doc.go: no preamble, no terminator — this is the whole of reassembly
// for this family) and dispatches each complete one.
func (r *Radio) handleBytes(b []byte) {
	r.mu.Lock()
	r.inbuf = append(r.inbuf, b...)
	var frames [][frameLen]byte
	for len(r.inbuf) >= frameLen {
		var f [frameLen]byte
		copy(f[:], r.inbuf[:frameLen])
		frames = append(frames, f)
		rest := make([]byte, len(r.inbuf)-frameLen)
		copy(rest, r.inbuf[frameLen:])
		r.inbuf = rest
	}
	r.mu.Unlock()

	for _, f := range frames {
		r.handleFrame(f)
	}
}

// handleFrame processes one 5-byte command (args[0..3], opcode last —
// core/bincat's frame.go documents the identical shape, re-derived here
// independently per THE HARD RULE) and sends whatever reply it produces,
// if any. Every write opcode is fire-and-forget: silence is success.
func (r *Radio) handleFrame(f [frameLen]byte) {
	args := [4]byte{f[0], f[1], f[2], f[3]}
	opcode := f[4]

	switch opcode {
	case opStatusUpdate:
		r.handleStatusUpdate(args)
	case opABSelect:
		r.mu.Lock()
		if args[0] == 0 || args[0] == 1 {
			r.activeVFO = int(args[0])
		}
		r.mu.Unlock()
	case opSetFreq:
		if v, ok := decodeBCD4LE(args); ok {
			r.mu.Lock()
			r.vfo[r.activeVFO].freqTensOfHz = uint32(v)
			r.mu.Unlock()
		}
	case opSetMode:
		if args[0] <= 4 {
			r.mu.Lock()
			r.vfo[r.activeVFO].mode = args[0]
			r.mu.Unlock()
		}
	case opClarifier:
		r.handleClarifier(args)
	case opShift:
		if args[0] <= 2 {
			r.mu.Lock()
			v := &r.vfo[r.activeVFO]
			v.opFlags &^= 0x18 // clear both minus (bit3) and plus (bit4)
			switch args[0] {
			case 1:
				v.opFlags |= 0x08
			case 2:
				v.opFlags |= 0x10
			}
			r.mu.Unlock()
		}
	case opTone:
		if args[0] <= 0x20 {
			r.mu.Lock()
			r.vfo[r.activeVFO].tone = args[0]
			r.mu.Unlock()
		}
	case opOffset:
		if hz, ok := decodeBCD2BE(args[2], args[3]); ok && args[0] == 0x00 && args[1] <= 2 {
			r.mu.Lock()
			r.vfo[r.activeVFO].offsetHz = uint32(args[1])*100000 + uint32(hz)*100
			r.mu.Unlock()
		}
	case opStore:
		r.handleStore(args)
	}
	// Any other opcode, or a rejected argument above: silence, matching
	// the manual's documented "should do nothing" behaviour (p.34).
}

// handleClarifier applies opClarifier's args. The manual's own printed
// byte layout for this opcode is OCR-degraded past confident recovery
// (matrix's OpClarifier note) — this is this fake's OWN reasonable
// reading of the legible fragment ("Clarifier on/off (C1=1/0) or clear
// offset (C1=FFh). Tune clarifier up/down (C2=0/1) by C3 (kHz) + C4
// (Hz)"), not a manual-evidenced byte-for-byte reconstruction.
func (r *Radio) handleClarifier(args [4]byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := &r.vfo[r.activeVFO]
	switch args[0] {
	case 0xFF:
		v.clarifierHz = 0
		v.clarOn = false
	case 0, 1:
		v.clarOn = args[0] == 1
		khz, ok1 := decodeBCDByte(args[2])
		hz, ok2 := decodeBCDByte(args[3])
		if ok1 && ok2 {
			delta := int16(uint16(khz)*1000 + uint16(hz))
			if args[1] == 1 {
				delta = -delta
			}
			v.clarifierHz = delta
		}
	}
}

// handleStore applies opStore ("VFO->M"): copies the active VFO's front
// sub-record into channel CH's front sub-record. Out-of-range CH does
// nothing (matrix §1.8's no-ack/no-NAK model) — and, per doc.go, the
// channel's REAR sub-record is left completely untouched either way: this
// fake has no operation that computes it.
func (r *Radio) handleStore(args [4]byte) {
	ch := int(args[0])
	if !validChannel(ch) {
		return
	}
	r.mu.Lock()
	r.channels[ch-slotBase].front = r.vfo[r.activeVFO]
	r.mu.Unlock()
}

// handleStatusUpdate answers opcode 0x10 per U (args[0]); an unrecognised
// U, or an out-of-range CH under U=uMemoryRecord, is SILENCE — no reply at
// all — which is how this family signals "wrong radio" or "bad parameter"
// alike (matrix §1.8).
func (r *Radio) handleStatusUpdate(args [4]byte) {
	switch args[0] {
	case uFullDump:
		r.sendFullDump(r.buildFullDump())
	case uMemoryNumber:
		r.rawWrite([]byte{0})
	case uOperatingData:
		r.rawWrite(r.buildOperatingRecord())
	case uBothVFOs:
		r.mu.Lock()
		a := encodeFront(r.vfo[0])
		b := encodeFront(r.vfo[1])
		r.mu.Unlock()
		reply := make([]byte, 0, frontLen*2)
		reply = append(reply, a[:]...)
		reply = append(reply, b[:]...)
		r.rawWrite(reply)
	case uMemoryRecord:
		ch := int(args[3])
		if !validChannel(ch) {
			return // SILENCE: out-of-range channel (matrix §1.8)
		}
		r.mu.Lock()
		c := r.channels[ch-slotBase]
		r.mu.Unlock()
		rec := encodeRecord(c.flags, encodeFront(c.front), c.rear)
		r.rawWrite(rec[:])
	}
}

// buildOperatingRecord renders the 19-byte "current operating data"
// record: when operating on a VFO (this fake's only mode), its two
// sub-records ARE VFO-A and VFO-B themselves (manual p.33: "when
// operating on a VFO, the values in these records are identical to the
// two 9-byte records included in the 19-byte Data Record for current
// operation").
func (r *Radio) buildOperatingRecord() []byte {
	r.mu.Lock()
	a := encodeFront(r.vfo[0])
	b := encodeFront(r.vfo[1])
	r.mu.Unlock()
	rec := encodeRecord(0, a, b)
	return rec[:]
}

// buildFullDump renders the full 1941-byte RAM table (matrix §1.10): 3
// zero status-flag bytes (unmapped, matrix §"(A) Flag Bytes" — no field
// this milestone maps), 1 zero memory-number byte, the 19-byte current
// operating record, VFO-A (9) and VFO-B (9), then 100 19-byte channel
// records in order.
func (r *Radio) buildFullDump() []byte {
	out := make([]byte, 0, fullDumpLen)
	out = append(out, 0, 0, 0, 0) // 3 flag bytes + memory number
	out = append(out, r.buildOperatingRecord()...)

	r.mu.Lock()
	a := encodeFront(r.vfo[0])
	b := encodeFront(r.vfo[1])
	chans := r.channels
	r.mu.Unlock()

	out = append(out, a[:]...)
	out = append(out, b[:]...)
	for _, c := range chans {
		rec := encodeRecord(c.flags, encodeFront(c.front), c.rear)
		out = append(out, rec[:]...)
	}
	return out
}

// rawWrite sends a reply immediately (honouring only the pipe's own
// per-write Latency, if configured via WithLatency).
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}

// sendFullDump sends a U=uFullDump reply, split into WithChunkedFullDump's
// configured number of pieces with a sleep before each — the ONLY reply
// this fake ever chunks, since it is the only one long enough for a
// caller's read timeout to plausibly bite mid-reply (doc.go). Without
// that option (the default), it is one ordinary write.
func (r *Radio) sendFullDump(data []byte) {
	r.mu.Lock()
	n, delay := r.chunkFullDumpN, r.chunkDelay
	r.mu.Unlock()

	if n <= 1 {
		r.rawWrite(data)
		return
	}
	size := (len(data) + n - 1) / n
	for i := 0; i < len(data); i += size {
		end := i + size
		if end > len(data) {
			end = len(data)
		}
		if !r.pipe.Sleep(delay) {
			return
		}
		if !r.pipe.WriteNow(data[i:end]) {
			return
		}
	}
}
