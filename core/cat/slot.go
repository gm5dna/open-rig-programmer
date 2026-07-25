// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// Slot is a CAT slot code (the P1/P0 field used by MR/MW/MT/MC): a memory
// channel, a PMS pair, a 60m channel, the Alaska emergency channel, or the
// special "000" (VFO/MT/QMB) value seen in MR answers. It always holds a
// canonical 3-byte wire form.
//
// The zero value is not a valid Slot; construct one via MemorySlot,
// PMSSlot, SixtyMSlot, EMGSlot, or ParseSlot.
type Slot struct {
	wire string // canonical 3-byte wire form, e.g. "001", "P1L", "EMG"
}

// slotKind classifies a slot wire form. It exists purely to share
// classification logic between ParseSlot and the Slot.IsXxx predicates.
type slotKind int

const (
	slotKindInvalid slotKind = iota
	slotKindMemory
	slotKindPMS
	slotKind60m
	slotKindEMG
	slotKindNone
)

// classifySlotWire reports what kind of slot, if any, wire represents.
// Reference: "Slot codes (3 bytes on the wire)" table.
func classifySlotWire(wire string) slotKind {
	if len(wire) != 3 {
		return slotKindInvalid
	}

	switch wire {
	case "000":
		// Reference: "000 | In MR answers: 'VFO or MT or QMB'."
		return slotKindNone
	case "EMG":
		return slotKindEMG
	}

	allDigits := true
	for i := 0; i < len(wire); i++ {
		if wire[i] < '0' || wire[i] > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		n := int(wire[0]-'0')*100 + int(wire[1]-'0')*10 + int(wire[2]-'0')
		switch {
		case n >= 1 && n <= 99:
			return slotKindMemory
		case n >= 501 && n <= 599:
			// ASSUMED: reference marks 5xx numbering as unverified.
			return slotKind60m
		default:
			return slotKindInvalid
		}
	}

	if wire[0] == 'P' && wire[1] >= '1' && wire[1] <= '9' && (wire[2] == 'L' || wire[2] == 'U') {
		return slotKindPMS
	}

	return slotKindInvalid
}

// MemorySlot builds the Slot for memory channel n (M-01…M-99), n in
// [1, 99]. Reference: "001-099 | Memory channels M-01…M-99".
func MemorySlot(n int) (Slot, error) {
	if n < 1 || n > 99 {
		return Slot{}, newParseError([]byte(fmt.Sprintf("MemorySlot(%d)", n)), "memory channel out of range 1-99")
	}
	return Slot{wire: fmt.Sprintf("%03d", n)}, nil
}

// PMSSlot builds the Slot for PMS pair (1-9), lower or upper.
// Reference: "P1L-P9U | PMS pairs (9 lower/upper pairs)".
func PMSSlot(pair int, upper bool) (Slot, error) {
	if pair < 1 || pair > 9 {
		return Slot{}, newParseError([]byte(fmt.Sprintf("PMSSlot(%d)", pair)), "PMS pair out of range 1-9")
	}
	suffix := byte('L')
	if upper {
		suffix = 'U'
	}
	return Slot{wire: fmt.Sprintf("P%d%c", pair, suffix)}, nil
}

// SixtyMSlot builds the Slot for 60m channel n.
//
// ASSUMED: the reference documents the wire form only as "5xx" with
// "ASSUMED 501… numbering" — neither the numbering start nor the channel
// count is confirmed by the manual. This constructor assumes numbering
// starts at n=1 -> "501" and, because the wire form is fixed at 3 bytes
// ('5' + 2 digits), caps n at 99 -> "599". Both bounds must be verified at
// the M5a/M5b hardware sessions.
func SixtyMSlot(n int) (Slot, error) {
	if n < 1 || n > 99 {
		return Slot{}, newParseError([]byte(fmt.Sprintf("SixtyMSlot(%d)", n)), "60m channel out of ASSUMED range 1-99")
	}
	return Slot{wire: fmt.Sprintf("5%02d", n)}, nil
}

// EMGSlot returns the Slot for the Alaska emergency channel.
// Reference: "EMG | Alaska emergency channel".
func EMGSlot() Slot {
	return Slot{wire: "EMG"}
}

// ParseSlot parses a 3-byte wire slot code, accepting exactly the forms
// produced by MemorySlot, PMSSlot, SixtyMSlot and EMGSlot, plus "000"
// (which only ever appears in MR answers; see Slot.IsNone). Anything else
// — including out-of-range numbers, malformed PMS suffixes, and lower case
// — is rejected with a *ParseError.
func ParseSlot(wire string) (Slot, error) {
	if classifySlotWire(wire) == slotKindInvalid {
		return Slot{}, newParseError([]byte(wire), "not a valid slot wire form")
	}
	return Slot{wire: wire}, nil
}

// Wire returns the canonical 3-byte wire form of s.
func (s Slot) Wire() string {
	return s.wire
}

// IsMemory reports whether s is a memory channel slot (001-099).
func (s Slot) IsMemory() bool {
	return classifySlotWire(s.wire) == slotKindMemory
}

// IsPMS reports whether s is a PMS pair slot (P1L-P9U).
func (s Slot) IsPMS() bool {
	return classifySlotWire(s.wire) == slotKindPMS
}

// Is60m reports whether s is a 60m channel slot (5xx, ASSUMED numbering).
func (s Slot) Is60m() bool {
	return classifySlotWire(s.wire) == slotKind60m
}

// IsEMG reports whether s is the Alaska emergency channel slot (EMG).
func (s Slot) IsEMG() bool {
	return classifySlotWire(s.wire) == slotKindEMG
}

// IsNone reports whether s is the special "000" slot seen in MR answers
// ("VFO or MT or QMB"). Its semantics beyond that are UNKNOWN/ASSUMED per
// the reference, and it must never be emitted in a builder.
func (s Slot) IsNone() bool {
	return classifySlotWire(s.wire) == slotKindNone
}

// Writable reports whether s is valid as the target of an MW (memory
// write) command: memory and PMS slots only. Reference: "MW — ... P1
// restricted to 001-099, P1L-P9U (no 5xx, no EMG; 000 listed but semantics
// unknown — reject in builder)".
func (s Slot) Writable() bool {
	kind := classifySlotWire(s.wire)
	return kind == slotKindMemory || kind == slotKindPMS
}

// readableSlot reports whether s is a legal target for a bare slot-only
// READ command (MR read, MT read): any slot ParseSlot accepts EXCEPT the
// special "000" placeholder (Slot.IsNone), whose semantics the reference
// marks UNKNOWN/ASSUMED with an explicit "do not emit". Reads carry none
// of the write-direction hardware-verification concern that restricts
// MW/MT SET to memory and PMS slots only, so 5xx and EMG are both legal
// read targets here.
//
// Shared by BuildMRRead, BuildMTRead, and AllowedCommand's MR/MT-read
// grammar checks (allowlist.go), so this rule is expressed in exactly one
// place.
func readableSlot(s Slot) bool {
	switch classifySlotWire(s.wire) {
	case slotKindInvalid, slotKindNone:
		return false
	default:
		return true
	}
}
