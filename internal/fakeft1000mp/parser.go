// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft1000mp

// This file is fakeft1000mp's own, independent byte-level CAT parser and
// reply builder — derived from
// docs/superpowers/ft1000mp-capability-matrix.md and the manuals it cites,
// NOT from core/bincat or core/driver/ft1000mp (doc.go, THE HARD RULE).

// frameLen is every frame's fixed wire width: four argument bytes
// followed by the opcode byte, no preamble, no terminator (matrix §1.1) —
// core/bincat.FrameLen's own value, reimplemented independently here.
const frameLen = 5

// Opcodes this fake wires — see doc.go's "Scope this milestone actually
// ships" for the full list of what is deliberately absent.
const (
	opStore        byte = 0x03 // VFO/MEM: Store/Enter (K=00H only), Mask/Un-Mask (unwired)
	opSetFreq      byte = 0x0A // Set Main VFO-A Operating Freq (packed BCD)
	opSetMode      byte = 0x0C // MODE
	opShift        byte = 0x84 // Repeater shift direction
	opStatusUpdate byte = 0x10 // Status Update — only U=00H (full dump) is wired
	opFAH          byte = 0xFA // Read Internal Status Flags (identity)
)

// storeEnter is the 03H opcode's K=00H "Enter" value — matrix §1.8: the
// ONLY K value this milestone ships. K's own byte POSITION is an ASSUMED
// row (1st parameter, matrix §1.8) chosen among three equally unsupported
// candidates; Enter's own frame is byte-identical whichever is right, so
// nothing here depends on that choice being correct.
const storeEnter byte = 0x00

// ufullDump is Status Update's U (1st parameter) value that returns the
// whole 1,863-byte RAM table — the only U value this fake answers.
const ufullDump byte = 0x00

// reassembler groups an arbitrary stream of Write() chunks into complete
// frameLen-byte frames — trivial framing, since this family's frames carry
// no terminator and are always exactly frameLen bytes (matrix §1.1): every
// frameLen bytes accumulated is one frame, no boundary detection needed
// beyond counting.
type reassembler struct {
	buf []byte
}

func (a *reassembler) push(chunk []byte) [][frameLen]byte {
	a.buf = append(a.buf, chunk...)
	var frames [][frameLen]byte
	for len(a.buf) >= frameLen {
		var f [frameLen]byte
		copy(f[:], a.buf[:frameLen])
		frames = append(frames, f)
		a.buf = a.buf[frameLen:]
	}
	return frames
}

// decodeBCDField decodes a's four bytes as packed BCD, least-significant
// decimal pair in a[0] (the write-side convention matrix §1.3 and doc.go
// both cite — core/bincat.DecodeBCD's own algorithm, reimplemented
// independently). ok is false if any nibble exceeds 9.
func decodeBCDField(a [4]byte) (v uint64, ok bool) {
	for i := 3; i >= 0; i-- {
		hi, lo := a[i]>>4, a[i]&0x0F
		if hi > 9 || lo > 9 {
			return 0, false
		}
		v = v*100 + uint64(hi)*10 + uint64(lo)
	}
	return v, true
}

// handleFrame parses one complete frame and returns the reply to write:
// nil for fire-and-forget (every write opcode this fake handles, and
// every unhandled opcode — matrix §Context, no NAK), or the built reply
// for FAH. Status Update's own reply is written directly by
// handleStatusUpdate (possibly chunked, options.go), not returned here.
func (r *Radio) handleFrame(frame [frameLen]byte) []byte {
	args := [4]byte{frame[0], frame[1], frame[2], frame[3]}
	opcode := frame[4]

	switch opcode {
	case opFAH:
		return r.handleFAH(args)
	case opStatusUpdate:
		r.handleStatusUpdate(args)
	case opStore:
		r.handleStore(args)
	case opSetFreq:
		r.handleSetFreq(args)
	case opSetMode:
		r.handleSetMode(args)
	case opShift:
		r.handleShift(args)
	}
	return nil
}

// handleFAH answers Read Internal Status Flags. args[3] is F, the 4th
// parameter byte, per the Opcode Command Chart (4) row (matrix §1.7):
// F=01H asks for the 6-byte all-flags form (doc.go ASSUMED #1: always
// zero, nothing this milestone claims reads a flag bit); anything else —
// F=00H included — gets the 5-byte form, whose last two bytes are the
// fixed MARK-V transceiver ID this milestone's identity probe requires,
// 03H/93H (matrix §1.7).
func (r *Radio) handleFAH(args [4]byte) []byte {
	if args[3] == 0x01 {
		return make([]byte, 6)
	}
	return []byte{0x00, 0x00, 0x00, 0x03, 0x93}
}

// handleStatusUpdate answers Status Update. Only U=00H (the full,
// 1,863-byte dump) is wired (doc.go) — every other U value gets silence,
// same as any unhandled opcode.
func (r *Radio) handleStatusUpdate(args [4]byte) {
	if args[0] != ufullDump {
		return
	}
	dump := r.buildFullDump()

	r.mu.Lock()
	chunk, gap := r.fullDumpChunk, r.fullDumpGap
	r.mu.Unlock()

	if chunk <= 0 {
		r.rawWrite(dump)
		return
	}
	for len(dump) > 0 {
		n := chunk
		if n > len(dump) {
			n = len(dump)
		}
		if !r.pipe.WriteNow(dump[:n]) {
			return // peer gone, or shutdown — nothing left to do
		}
		dump = dump[n:]
		if len(dump) > 0 && !r.pipe.Sleep(gap) {
			return
		}
	}
}

// handleStore implements 03H VFO/MEM: Store/Enter (matrix §1.8's override
// — this milestone ships ONLY K=00H). X (args[3], the 4th parameter byte)
// is the target channel, taken 1-based directly per matrix §1.4's
// override (channel 1 -> X=01H): out-of-range X (outside 1..numMemories)
// is silently ignored, this family's own no-NAK convention.
func (r *Radio) handleStore(args [4]byte) {
	if args[0] != storeEnter {
		return // Mask/Un-Mask — unwired, matrix §1.8
	}
	ch := int(args[3])
	if ch < 1 || ch > numMemories {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memories[ch-1] = r.vfoRecordBytesLocked()
}

// handleSetFreq implements 0AH: Set Main VFO-A Operating Freq. A
// malformed BCD field (a nibble above 9) is ignored, not stored.
func (r *Radio) handleSetFreq(args [4]byte) {
	v, ok := decodeBCDField(args)
	if !ok {
		return
	}
	r.mu.Lock()
	r.vfoFreqTensOfHz = v
	r.mu.Unlock()
}

// handleSetMode implements 0CH: MODE. An out-of-legend byte is ignored.
func (r *Radio) handleSetMode(args [4]byte) {
	if _, ok := modeNames[args[0]]; !ok {
		return
	}
	r.mu.Lock()
	r.vfoMode = args[0]
	r.mu.Unlock()
}

// Shift bits inside the VFO/Memory Operating Flags byte (record offset
// 9) — 0x08 Minus, 0x10 Plus, matching core/bincat.ParseRecord's own
// documented bit meaning for this shared family convention (spec.md
// §Write model step 5: "confirmed identical across all four family
// members"), reimplemented independently here rather than imported.
const (
	shiftFlagMinus byte = 0x08
	shiftFlagPlus  byte = 0x10
)

// handleShift implements 84H: repeater shift direction. R (args[0]) is
// 00H Simplex, 01H Minus or 02H Plus; anything else is ignored.
func (r *Radio) handleShift(args [4]byte) {
	var flags byte
	switch args[0] {
	case 0x00:
		flags = 0x00
	case 0x01:
		flags = shiftFlagMinus
	case 0x02:
		flags = shiftFlagPlus
	default:
		return
	}
	r.mu.Lock()
	r.vfoFlags = flags
	r.mu.Unlock()
}
