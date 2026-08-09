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
//
// A SLOT CARRIES ITS CONSTRUCTING DIALECT'S CLASSIFICATION. Every
// constructor is a Dialect method, so the kind is known at construction:
// ParseSlot stores what d.classifySlot said about the wire form it
// accepted, and MemorySlot, PMSSlot, SixtyMSlot and EMGSlot each store the
// one kind they build by definition. Slot's own predicates — IsMemory,
// IsPMS, Is60m, IsEMG, IsNone and Writable — read that stored kind, so
// they answer FOR THE DIALECT THAT BUILT THE SLOT, by construction, on
// every dialect. There is no package-level classification left anywhere.
//
// THIS DISCHARGES THE M9b DEFERRAL (item 2 of the M9b plan's "Deferred,
// and ledgered as such" list). Until M9d those predicates classified
// through a package-level classifySlotWire helper that forwarded to FT710,
// because a Slot then carried no dialect tag of its own — harmless while
// only the FT-710 existed, wrong for the FTdx10 and FTdx101 slots that now
// do. The helper is deleted.
//
// THE FOUR STATIC KINDS ARE NOT A SECOND OPINION. Each is exactly what the
// constructing dialect's own classifySlot would return for the wire form
// that constructor built, because DialectConfig validation rejects every
// configuration in which they could disagree: V6 forbids a memory range
// overlapping the 60m range, and V7 forbids a none or emergency wire that
// shadows either numeric range or a PMS form the dialect can build (see
// validateSixtyRange and validateShadowing in dialectvalidate.go).
//
// A Dialect method asking about a Slot IT MAY NOT HAVE BUILT must still
// classify the wire form under itself — d.classifySlot — rather than read
// the stored kind: see readableSlot and writableSlot below.
//
// The zero Slot's kind is slotKindInvalid, so every predicate is false on
// it, consistent with "the zero value is not a valid Slot" above.
type Slot struct {
	wire string   // canonical 3-byte wire form, e.g. "001", "P1L", "EMG"
	kind slotKind // classification under the dialect that constructed it
}

// slotKind classifies a slot wire form under some one dialect's slot
// space. Dialect.classifySlot computes it; a Slot stores the value its own
// constructing dialect gave it (see Slot above).
type slotKind int

const (
	slotKindInvalid slotKind = iota
	slotKindMemory
	slotKindPMS
	slotKind60m
	slotKindEMG
	slotKindNone
)

// ParseSlot parses a 3-byte wire slot code under this dialect's slot
// space, accepting exactly the forms produced by MemorySlot, PMSSlot,
// SixtyMSlot and EMGSlot, plus the dialect's "000"-equivalent (which only
// ever appears in MR answers; see Slot.IsNone). Anything else —
// including out-of-range numbers, malformed PMS suffixes, and lower case
// — is rejected with a *ParseError. Reference: "Slot codes (3 bytes on
// the wire)".
func (d Dialect) ParseSlot(wire string) (Slot, error) {
	kind := d.classifySlot(wire)
	if kind == slotKindInvalid {
		return Slot{}, newParseError([]byte(wire), "not a valid slot wire form")
	}
	return Slot{wire: wire, kind: kind}, nil
}

// MemorySlot builds the Slot for memory channel n under this dialect's
// configured memory range (memoryLo..memoryHi). For FT710 that range is
// 1..99, i.e. M-01…M-99; reference: "001-099 | Memory channels
// M-01…M-99".
func (d Dialect) MemorySlot(n int) (Slot, error) {
	if n < d.slots.memoryLo || n > d.slots.memoryHi || d.slots.memoryHi == 0 {
		return Slot{}, newParseError([]byte(fmt.Sprintf("MemorySlot(%d)", n)), "memory channel out of range 1-99")
	}
	return Slot{wire: fmt.Sprintf("%03d", n), kind: slotKindMemory}, nil
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
	return Slot{wire: fmt.Sprintf("P%d%c", pair, suffix), kind: slotKindPMS}, nil
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
	return Slot{wire: fmt.Sprintf("%03d", d.slots.sixtyLo+n-1), kind: slotKind60m}, nil
}

// EMGSlot returns the Slot for this dialect's Alaska-emergency-equivalent
// channel, or the zero Slot if this dialect has none. Reference: "EMG |
// Alaska emergency channel".
func (d Dialect) EMGSlot() Slot {
	if d.slots.emgWire == "" {
		return Slot{}
	}
	return Slot{wire: d.slots.emgWire, kind: slotKindEMG}
}

// Wire returns the canonical 3-byte wire form of s.
func (s Slot) Wire() string {
	return s.wire
}

// IsMemory reports whether s is a memory channel slot (001-099 on the
// FT-710) under the dialect that constructed it.
func (s Slot) IsMemory() bool {
	return s.kind == slotKindMemory
}

// IsPMS reports whether s is a PMS pair slot (P1L-P9U on the FT-710) under
// the dialect that constructed it.
func (s Slot) IsPMS() bool {
	return s.kind == slotKindPMS
}

// Is60m reports whether s is a 60m channel slot (5xx on the FT-710,
// ASSUMED numbering) under the dialect that constructed it.
func (s Slot) Is60m() bool {
	return s.kind == slotKind60m
}

// IsEMG reports whether s is the Alaska emergency channel slot (EMG on the
// FT-710) under the dialect that constructed it.
func (s Slot) IsEMG() bool {
	return s.kind == slotKindEMG
}

// IsNone reports whether s is the special "000" slot seen in MR answers
// ("VFO or MT or QMB") — more precisely, whether it is the none form of
// the dialect that constructed it, since WHICH wire form means none is
// dialect data (SlotSpace.NoneWire). Its semantics beyond that are
// UNKNOWN/ASSUMED per the reference, and it must never be emitted in a
// builder.
func (s Slot) IsNone() bool {
	return s.kind == slotKindNone
}

// Writable reports whether s is valid as the target of an MW (memory
// write) command UNDER THE DIALECT THAT CONSTRUCTED IT: memory and PMS
// slots only. Reference: "MW — ... P1 restricted to 001-099, P1L-P9U (no
// 5xx, no EMG; 000 listed but semantics unknown — reject in builder)".
//
// A Dialect method deciding whether IT will accept a write to s must use
// Dialect.writableSlot instead: see that method for why the two are not
// interchangeable on a Slot built elsewhere.
func (s Slot) Writable() bool {
	return s.kind == slotKindMemory || s.kind == slotKindPMS
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
// It classifies s's wire form through d.classifySlot rather than reading
// the kind s carries. Since M9d a Slot does carry one (see Slot's own doc
// comment), but it is the classification of the dialect that BUILT the
// slot, and the question here is whether THIS dialect will accept it —
// which for a Slot built elsewhere is a different question with a
// different answer. seconddialect_test.go is where that difference is
// measured: an FT-710 slot outside a narrower dialect's space must be
// refused by that dialect's read builders. The older form of this note
// warned against a package-level classifySlotWire helper, which the same
// change deleted.
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
// This is the receiver-aware counterpart of Slot.Writable, and exists
// because validateMWFields — reached from BuildMWSet AND from
// AllowedCommand's MW grammar check — must decide writability against the
// dialect it was called on, for a MemoryData whose Slot the caller may
// have built under a different one (or forged wholesale).
//
// SINCE M9d THE TWO AGREE ON A SLOT THIS DIALECT BUILT, and must still not
// be confused. Slot.Writable now reads the kind its own constructing
// dialect stored — the M9b deferral discharged, see Slot's doc comment —
// rather than answering for the FT-710 as it did while a Slot carried no
// dialect tag. On a FOREIGN slot the two diverge by design, and it is this
// one, classifying the wire form under the receiver, that the outbound
// write gate needs.
func (d Dialect) writableSlot(s Slot) bool {
	kind := d.classifySlot(s.wire)
	return kind == slotKindMemory || kind == slotKindPMS
}
