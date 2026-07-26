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

// classifySlotWire reports what kind of slot, if any, wire represents,
// under the FT-710 dialect's slot space.
//
// FT-710-SCOPED (deferred, ledgered in the M9b plan's "Deferred, and
// ledgered as such" list, item 2): Slot's own predicates below — Wire,
// IsMemory, IsPMS, Is60m, IsEMG, IsNone and Writable — classify through
// this helper rather than through a Dialect receiver, because Slot itself
// carries no dialect tag yet. That is harmless while only the FT-710
// dialect exists. Giving Slot a dialect tag, so these predicates read the
// Slot's own dialect instead, is M9c's, once a second slot space exists to
// force it.
//
// NOTHING REACHED FROM A DIALECT METHOD MAY CALL THIS (Task 54). The
// former package-level readableSlot did, and is now Dialect.readableSlot;
// the one Dialect-method use of Slot.Writable is now Dialect.writableSlot.
// Both classify through d.classifySlot. A Dialect method routed through
// this helper would silently answer for the FT-710 whatever dialect it was
// called on — the exact failure mode M9b exists to prevent.
func classifySlotWire(wire string) slotKind {
	return FT710.classifySlot(wire)
}

// ParseSlot parses a 3-byte wire slot code under this dialect's slot
// space, accepting exactly the forms produced by MemorySlot, PMSSlot,
// SixtyMSlot and EMGSlot, plus the dialect's "000"-equivalent (which only
// ever appears in MR answers; see Slot.IsNone). Anything else —
// including out-of-range numbers, malformed PMS suffixes, and lower case
// — is rejected with a *ParseError. Reference: "Slot codes (3 bytes on
// the wire)".
func (d Dialect) ParseSlot(wire string) (Slot, error) {
	if d.classifySlot(wire) == slotKindInvalid {
		return Slot{}, newParseError([]byte(wire), "not a valid slot wire form")
	}
	return Slot{wire: wire}, nil
}

// MemorySlot builds the Slot for memory channel n under this dialect's
// configured memory range (memoryLo..memoryHi). For FT710 that range is
// 1..99, i.e. M-01…M-99; reference: "001-099 | Memory channels
// M-01…M-99".
func (d Dialect) MemorySlot(n int) (Slot, error) {
	if n < d.slots.memoryLo || n > d.slots.memoryHi || d.slots.memoryHi == 0 {
		return Slot{}, newParseError([]byte(fmt.Sprintf("MemorySlot(%d)", n)), "memory channel out of range 1-99")
	}
	return Slot{wire: fmt.Sprintf("%03d", n)}, nil
}

// PMSSlot builds the Slot for PMS pair (1-9), lower or upper, under this
// dialect's PMS pair count. Reference: "P1L-P9U | PMS pairs (9
// lower/upper pairs)".
func (d Dialect) PMSSlot(pair int, upper bool) (Slot, error) {
	if pair < 1 || pair > d.pmsCap() {
		return Slot{}, newParseError([]byte(fmt.Sprintf("PMSSlot(%d)", pair)), "PMS pair out of range 1-9")
	}
	suffix := byte('L')
	if upper {
		suffix = 'U'
	}
	return Slot{wire: fmt.Sprintf("P%d%c", pair, suffix)}, nil
}

// SixtyMSlot builds the Slot for 60m channel n (an ordinal starting at 1)
// under this dialect's 60m range: the wire form is the dialect's own
// sixtyLo+n-1, capped at sixtyHi, so the result is always inside this
// SAME dialect's own slot space — codex review Important-1 caught an
// earlier version that read the receiver only for the bounds check and
// hardcoded a '5' prefix for the wire form itself, which a differently
// numbered 60m dialect's own ParseSlot then rejected. For FT710 (sixtyLo
// 501, sixtyHi 599) that is n=1 -> "501" through n=99 -> "599".
//
// ASSUMED: the reference documents the wire form only as "5xx" with
// "ASSUMED 501… numbering" — neither the numbering start nor the channel
// count is confirmed by the manual for FT710. Both bounds must be
// verified at the M5a/M5b hardware sessions.
func (d Dialect) SixtyMSlot(n int) (Slot, error) {
	count := d.slots.sixtyHi - d.slots.sixtyLo + 1
	if d.slots.sixtyHi == 0 || n < 1 || n > count {
		return Slot{}, newParseError([]byte(fmt.Sprintf("SixtyMSlot(%d)", n)), "60m channel out of ASSUMED range 1-99")
	}
	return Slot{wire: fmt.Sprintf("%03d", d.slots.sixtyLo+n-1)}, nil
}

// EMGSlot returns the Slot for this dialect's Alaska-emergency-equivalent
// channel, or the zero Slot if this dialect has none. Reference: "EMG |
// Alaska emergency channel".
func (d Dialect) EMGSlot() Slot {
	if d.slots.emgWire == "" {
		return Slot{}
	}
	return Slot{wire: d.slots.emgWire}
}

// MemorySlot builds the Slot for memory channel n (M-01…M-99), n in
// [1, 99]. Reference: "001-099 | Memory channels M-01…M-99".
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func MemorySlot(n int) (Slot, error) {
	return FT710.MemorySlot(n)
}

// PMSSlot builds the Slot for PMS pair (1-9), lower or upper.
// Reference: "P1L-P9U | PMS pairs (9 lower/upper pairs)".
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func PMSSlot(pair int, upper bool) (Slot, error) {
	return FT710.PMSSlot(pair, upper)
}

// SixtyMSlot builds the Slot for 60m channel n.
//
// ASSUMED: the reference documents the wire form only as "5xx" with
// "ASSUMED 501… numbering" — neither the numbering start nor the channel
// count is confirmed by the manual. This constructor assumes numbering
// starts at n=1 -> "501" and, because the wire form is fixed at 3 bytes
// ('5' + 2 digits), caps n at 99 -> "599". Both bounds must be verified at
// the M5a/M5b hardware sessions.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func SixtyMSlot(n int) (Slot, error) {
	return FT710.SixtyMSlot(n)
}

// EMGSlot returns the Slot for the Alaska emergency channel.
// Reference: "EMG | Alaska emergency channel".
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func EMGSlot() Slot {
	return FT710.EMGSlot()
}

// ParseSlot parses a 3-byte wire slot code, accepting exactly the forms
// produced by MemorySlot, PMSSlot, SixtyMSlot and EMGSlot, plus "000"
// (which only ever appears in MR answers; see Slot.IsNone). Anything else
// — including out-of-range numbers, malformed PMS suffixes, and lower case
// — is rejected with a *ParseError.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func ParseSlot(wire string) (Slot, error) {
	return FT710.ParseSlot(wire)
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
// READ command (MR read, MT read) UNDER THIS DIALECT'S slot space: any
// slot this dialect's ParseSlot accepts EXCEPT the special "000"
// placeholder (this dialect's noneWire), whose semantics the reference
// marks UNKNOWN/ASSUMED with an explicit "do not emit". Reads carry none
// of the write-direction hardware-verification concern that restricts
// MW/MT SET to memory and PMS slots only, so 5xx and EMG are both legal
// read targets here.
//
// Shared by BuildMRRead, BuildMTRead, and AllowedCommand's MR/MT-read
// grammar checks (allowlist.go), so this rule is expressed in exactly one
// place.
//
// It classifies through d.classifySlot, NOT through the package-level
// classifySlotWire: this helper is reached from inside Dialect methods,
// and a Dialect method that consults a package global has the shape of a
// seam and none of the substance (see Dialect's doc comment).
func (d Dialect) readableSlot(s Slot) bool {
	switch d.classifySlot(s.wire) {
	case slotKindInvalid, slotKindNone:
		return false
	default:
		return true
	}
}

// writableSlot reports whether s is valid as the target of an MW (memory
// write) command UNDER THIS DIALECT'S slot space: memory and PMS slots
// only. Reference: "MW — ... P1 restricted to 001-099, P1L-P9U (no 5xx, no
// EMG; 000 listed but semantics unknown — reject in builder)".
//
// This is the dialect-aware counterpart of Slot.Writable, and exists
// because validateMWFields — reached from BuildMWSet AND from
// AllowedCommand's MW grammar check — must decide writability against the
// dialect it was called on. Slot.Writable classifies through
// classifySlotWire (i.e. through FT710) because a Slot carries no dialect
// tag of its own; giving Slot that tag is deferred to M9c, ledgered in the
// M9b plan. Until then the two must not be confused: Slot's own predicates
// are the FT-710-scoped convenience form, this is the seam-correct one.
func (d Dialect) writableSlot(s Slot) bool {
	kind := d.classifySlot(s.wire)
	return kind == slotKindMemory || kind == slotKindPMS
}
