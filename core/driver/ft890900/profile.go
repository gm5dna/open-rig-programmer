// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import "github.com/gm5dna/open-rig-programmer/core/bincat"

// The two radios' true channel counts (capability matrices §1.9): what
// caps.go advertises and read.go/write.go enforce for every slot a caller
// can actually name. Distinct from the *EngineProfile.SlotCount values
// below, which is bincat's own outbound-gate domain (see doc.go's
// "Identity" section for why FT-890's differs from its true count by
// one).
const (
	ft890TrueSlotCount = 32
	ft900TrueSlotCount = 100
	slotBase           = 1
)

// probeChannel1 and probeChannel2 are the two fixed channels Open's
// identity probe reads (spec.md §Identity probe, Codex #2). Channel 33 is
// one past FT-890's documented top channel (32) and comfortably inside
// FT-900's (100).
const (
	probeChannel1 = 1
	probeChannel2 = 33
)

// sharedModes is the 5-value mode legend FT-890 and FT-900 share
// byte-for-byte (capability matrices §1.5): the record's own legend, not
// the broader/OCR-degraded live-VFO MODE opcode table.
var sharedModes = map[byte]string{
	bincat.ModeLSB: "LSB",
	bincat.ModeUSB: "USB",
	bincat.ModeCW:  "CW",
	bincat.ModeAM:  "AM",
	bincat.ModeFM:  "FM",
}

// Record offsets, relative to the whole 19-byte record (1 leading Memory
// Status Flags byte, then the 9-byte front sub-record whose own offsets
// the capability matrices give relative to ITS start — these are that
// plus one). Shared identically by FT-890 and FT-900 (matrices §2).
const (
	freqOffset  = 2 // sub-record offset 1, 3 bytes
	clarOffset  = 5 // sub-record offset 4, 2 bytes
	modeOffset  = 7 // sub-record offset 6, 1 byte
	toneOffset  = 8 // sub-record offset 7, 1 byte
	flagsOffset = 9 // sub-record offset 8, 1 byte (carries Shift)
)

// engineProfile builds the bincat.Profile used to gate and frame this
// model's Engine. engineSlotCount is the OUTBOUND-GATE domain (see the
// package doc comment); it is the model's true slot count for FT-900 and
// one wider for FT-890.
func engineProfile(model, catID string, engineSlotCount, fullDumpLen int) bincat.Profile {
	return bincat.Profile{
		Model:       model,
		CATID:       catID,
		SlotBase:    slotBase,
		SlotCount:   engineSlotCount,
		RecordLen:   19,
		FullDumpLen: fullDumpLen,
		Modes:       sharedModes,
		HasTone:     true,
		FreqOffset:  freqOffset,
		ClarOffset:  clarOffset,
		ModeOffset:  modeOffset,
		ToneOffset:  toneOffset,
		FlagsOffset: flagsOffset,
	}
}

// ft890CATID and ft900CATID are fixed, invented, DISPLAY-ONLY synthetic
// identifiers (capability matrices §1.7): this family has no wire CAT-ID
// byte at all, so nothing here is wire-derived — identity is proved
// entirely by the Open probe (ft890900.go), never by these strings.
const (
	ft890CATID = "0890"
	ft900CATID = "0900"
)

var (
	// ft890EngineProfile's SlotCount is 33, ONE MORE than FT-890's true
	// 32-channel range (capability matrix §1.9) — see the package doc
	// comment's "Identity" section: this is what lets Open's boundary
	// probe (channel 33) reach the wire at all through bincat's own
	// AllowedCommand gate. caps.go and read.go/write.go's slotToChannel
	// use ft890TrueSlotCount, not this value, for everything else.
	ft890EngineProfile = engineProfile("FT-890", ft890CATID, ft890TrueSlotCount+1, 649)
	// ft900EngineProfile's SlotCount is already the true count (100),
	// which comfortably covers the boundary probe (channel 33) with no
	// widening needed.
	ft900EngineProfile = engineProfile("FT-900", ft900CATID, ft900TrueSlotCount, 1941)
)
