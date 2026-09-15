// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft890

// This family's constants (matrix §1.9, §1.10, §2): 32 channels numbered
// 1..32, a 19-byte per-channel record, and a 649-byte full-dump.
const (
	slotBase    = 1
	slotCount   = 32
	recordLen   = 19
	frontLen    = 9
	fullDumpLen = 649
)

// vfoState is one VFO's operating data: the fields the 9-byte VFO/Memory
// Data Record's front half carries (matrix §2). offsetHz is write-only —
// no documented read-side field carries the repeater-offset MAGNITUDE
// (only its direction, in opFlags) — so it is kept purely to prove opcode
// 0xF9 was accepted and mutated VFO state, per the task brief, even though
// nothing this fake reads back exposes it.
type vfoState struct {
	freqTensOfHz uint32
	clarifierHz  int16
	clarOn       bool
	mode         byte
	tone         byte
	opFlags      byte // bit2 skip, bit3 minus-shift, bit4 plus-shift, bit5 clarifier-enabled
	offsetHz     uint32
}

// channel is one memory's full 19-byte record: a flags byte, a FRONT
// sub-record this fake computes from whatever was last Stored into it,
// and a REAR sub-record this fake never computes at all (doc.go).
type channel struct {
	flags byte
	front vfoState
	rear  [frontLen]byte
}

// seedRear fills a channel's rear sub-record with a fixed, per-channel,
// visibly non-zero pattern — so a test asserting it comes back unmodified
// is actually exercising the echo, not passing vacuously against a
// zero-value slice.
func seedRear(ch int) [frontLen]byte {
	var b [frontLen]byte
	for i := range b {
		b[i] = byte(0xA0 + ch + i)
	}
	return b
}

// encodeFront renders v as this family's 9-byte VFO/Memory Data Record
// (matrix §2: offset 0 BPF (unmodelled, left zero), 1-3 frequency 24-bit
// binary MSB-first, 4-5 clarifier 2's-complement, 6 mode, 7 tone, 8
// operating flags).
func encodeFront(v vfoState) [frontLen]byte {
	var b [frontLen]byte
	b[0] = 0
	b[1] = byte(v.freqTensOfHz >> 16)
	b[2] = byte(v.freqTensOfHz >> 8)
	b[3] = byte(v.freqTensOfHz)
	clar := uint16(v.clarifierHz)
	b[4] = byte(clar >> 8)
	b[5] = byte(clar)
	b[6] = v.mode
	b[7] = v.tone
	flags := v.opFlags
	if v.clarOn {
		flags |= 0x20
	} else {
		flags &^= 0x20
	}
	b[8] = flags
	return b
}

// encodeRecord renders a full 19-byte record: flags byte, front, rear.
func encodeRecord(flags byte, front [frontLen]byte, rear [frontLen]byte) [recordLen]byte {
	var b [recordLen]byte
	b[0] = flags
	copy(b[1:10], front[:])
	copy(b[10:19], rear[:])
	return b
}

// decodeBCDByte decodes one packed-BCD byte (two decimal digits, high
// nibble first). This fake writes its own decoder rather than importing
// core/civ or core/bincat (THE HARD RULE, doc.go) — the family's own BCD
// shape is simple enough not to need a shared helper.
func decodeBCDByte(b byte) (v byte, ok bool) {
	hi, lo := b>>4, b&0x0F
	if hi > 9 || lo > 9 {
		return 0, false
	}
	return hi*10 + lo, true
}

// decodeBCD4LE decodes SetOpFreq's 4-byte packed-BCD argument, least
// significant decimal pair FIRST (matrix §1.1's worked example: 00 50 42
// 01 for 1,425,000 tens-of-Hz).
func decodeBCD4LE(b [4]byte) (uint64, bool) {
	var out uint64
	for i := 3; i >= 0; i-- {
		pair, ok := decodeBCDByte(b[i])
		if !ok {
			return 0, false
		}
		out = out*100 + uint64(pair)
	}
	return out, true
}

// decodeBCD2BE decodes a 2-byte packed-BCD field, MOST significant
// decimal pair first — Rptr Offset's S3/S4 shape (matrix §2's Shift
// representation: "S3 is 1's & 10's of kHz, S4 is 10's & 100's of Hz").
func decodeBCD2BE(hi, lo byte) (uint64, bool) {
	p1, ok := decodeBCDByte(hi)
	if !ok {
		return 0, false
	}
	p2, ok := decodeBCDByte(lo)
	if !ok {
		return 0, false
	}
	return uint64(p1)*100 + uint64(p2), true
}

// validChannel reports whether ch (a 1-based memory number, matrix §1.9)
// is inside this radio's slotBase..slotBase+slotCount-1 range.
func validChannel(ch int) bool {
	return ch >= slotBase && ch < slotBase+slotCount
}
