// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import "fmt"

// Record is one decoded 19-byte VFO/Memory Data Record — the shape FT-890
// and FT-900 share exactly (spec.md §Frame grammar table: "identical
// shape"). Only the FRONT 9-byte sub-record — the operating-relevant half
// — is decoded.
//
// THE SECOND, TRAILING 9-BYTE SUB-RECORD IS NOT MODELLED AT ALL, and that
// is a withdrawal, not an oversight (plan.md Phase 1 v2 change, Codex #6).
// No manual states what it holds, and there is no coherent "preserve the
// current unknown bytes" operation available at the seam this codec feeds:
// Store/Enter overwrites the WHOLE record from live VFO/mode/clarifier/
// shift/tone state, so nothing this codec writes controls that
// sub-record's post-write content. A caller that genuinely needs the raw
// bytes still has the slice it passed to ParseRecord; this package does
// not duplicate them onto Record only to imply a preservation guarantee it
// cannot back.
type Record struct {
	// FreqTensOfHz is the base frequency, in units of 10 Hz, WITHOUT
	// clarifier/repeater offset applied — a plain 24-bit BINARY value
	// (NOT packed BCD; this is the read-side encoding, distinct from
	// SetFreq's write-side packed BCD — spec.md §Frame grammar table).
	FreqTensOfHz uint32
	// ClarifierHz is the clarifier offset, 2's-complement signed, in Hz.
	ClarifierHz int16
	// ModeByte is the record's raw mode byte; Mode is p.Modes[ModeByte],
	// empty if the byte is not in the table.
	ModeByte byte
	Mode     string
	// Tone is the raw CTCSS tone code byte. Meaningless unless the
	// Profile this record was parsed with says HasTone.
	Tone byte
	// Shift is "SIMPLEX", "MINUS" or "PLUS", decoded from the record's
	// VFO/Memory Operating Flags byte.
	Shift string
	// Blanked and Split are the Memory Status Flags byte's own bits 7
	// and 6 (offset 0 of the whole 19-byte record, not the sub-record).
	Blanked bool
	Split   bool
}

// ParseRecord decodes raw as one VFO/Memory Data Record, per p's own
// offsets. raw must be exactly p.RecordLen bytes — a length mismatch is
// refused rather than guessed at, since record length is exactly the
// evidence a wrong-radio identity probe reads (spec.md's Identity probe).
func ParseRecord(raw []byte, p Profile) (Record, error) {
	if !p.Configured() {
		return Record{}, fmt.Errorf("%w: unconfigured profile", ErrRecord)
	}
	if len(raw) != p.RecordLen {
		return Record{}, fmt.Errorf("%w: record is %d bytes, want %d", ErrRecord, len(raw), p.RecordLen)
	}
	if p.FreqOffset < 0 || p.FreqOffset+3 > len(raw) ||
		p.ClarOffset < 0 || p.ClarOffset+2 > len(raw) ||
		p.ModeOffset < 0 || p.ModeOffset >= len(raw) ||
		p.FlagsOffset < 0 || p.FlagsOffset >= len(raw) {
		return Record{}, fmt.Errorf("%w: profile offsets do not fit a %d-byte record", ErrRecord, len(raw))
	}
	if p.HasTone && (p.ToneOffset < 0 || p.ToneOffset >= len(raw)) {
		return Record{}, fmt.Errorf("%w: profile HasTone but ToneOffset does not fit a %d-byte record", ErrRecord, len(raw))
	}

	rec := Record{
		Blanked: raw[0]&0x80 != 0,
		Split:   raw[0]&0x40 != 0,
	}

	f := raw[p.FreqOffset : p.FreqOffset+3]
	rec.FreqTensOfHz = uint32(f[0])<<16 | uint32(f[1])<<8 | uint32(f[2])

	rec.ClarifierHz = int16(uint16(raw[p.ClarOffset])<<8 | uint16(raw[p.ClarOffset+1]))

	rec.ModeByte = raw[p.ModeOffset]
	rec.Mode = p.Modes[rec.ModeByte]

	if p.HasTone {
		rec.Tone = raw[p.ToneOffset]
	}

	flags := raw[p.FlagsOffset]
	minus, plus := flags&0x08 != 0, flags&0x10 != 0
	switch {
	case minus && plus:
		return Record{}, fmt.Errorf("%w: operating flags byte %#02x sets both Minus and Plus shift bits", ErrRecord, flags)
	case minus:
		rec.Shift = "MINUS"
	case plus:
		rec.Shift = "PLUS"
	default:
		rec.Shift = "SIMPLEX"
	}

	return rec, nil
}
