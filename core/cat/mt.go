// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
)

// mtTagMaxBytes is the maximum tag length in BYTES (not runes) accepted by
// BuildMTSet/ParseMTAnswer. Reference P2: "tag, up to 12 ASCII characters".
const mtTagMaxBytes = 12

// mtReadLen is the fixed length of an MT read request: "MT" + 3-byte slot
// + ";". Golden vector G10: "MT001;".
const mtReadLen = 6

// mtAnswerMinLen/mtAnswerMaxLen bound an MT Set/Answer frame's total
// length: "MT"(2) + slot(3) + display(1) + tag(0-12) + ";"(1).
const (
	mtAnswerMinLen = 2 + 3 + 1 + 0 + 1
	mtAnswerMaxLen = 2 + 3 + 1 + mtTagMaxBytes + 1
)

// mtSlotValid reports whether s is a legal MT SET target under project
// policy: memory or PMS slots only.
//
// The manual's slot table marks 5xx and EMG as ✓ for MT — but reference
// §MT states explicitly: "our policy: reject sets to 5xx/EMG until
// hardware-verified" (a project decision, not a manual requirement,
// repeated verbatim in the Task 3 brief). "000" is ✗ for MT in the manual
// itself (semantics unknown, do not emit) and is rejected unconditionally
// by classifySlotWire returning slotKindNone/slotKindInvalid here.
func mtSlotValid(s Slot) bool {
	switch classifySlotWire(s.Wire()) {
	case slotKindMemory, slotKindPMS:
		return true
	default:
		return false
	}
}

// validMTTagByte reports whether b is a legal MT tag byte: printable ASCII
// 0x20-0x7E, EXCLUDING ';' (0x3B).
//
// SAFETY CRITICAL: this is command-injection defence. A tag byte of ';'
// would let a caller-supplied string terminate the MT frame early and
// smuggle an attacker-chosen second command onto the wire to a physical
// radio. Reference: "Tag charset (SAFETY-CRITICAL) ... printable ASCII
// 0x20-0x7E excluding ';' (0x3B); reject all control bytes."
func validMTTagByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E && b != ';'
}

// validMTTag reports whether tag is short enough (measured in BYTES, so a
// multi-byte UTF-8 rune counts for every byte it occupies, never just one)
// and contains only validMTTagByte bytes. Any byte >= 0x80 (which includes
// every byte of a multi-byte UTF-8 rune such as 'é') is rejected by the
// charset check alone, before the length check is even relevant.
func validMTTag(tag string) bool {
	if len(tag) > mtTagMaxBytes {
		return false
	}
	for i := 0; i < len(tag); i++ {
		if !validMTTagByte(tag[i]) {
			return false
		}
	}
	return true
}

// mtClearTag is the wire form BuildMTSet emits for an EMPTY tag: the
// all-spaces 12-byte tag, the FT-710's one hardware-proven tag-CLEAR
// mechanism. HW-PROBED 13/07/2026 (docs/fixtures-private/
// m5b-trials.private-capture, stages tagclear/tagclear2;
// docs/hardware-notes.md §Empty-slot create, tag-clear): the 0-byte-tag
// Set form ("MT0960;") is REJECTED ("?;", ~4 ms) and the existing tag
// SURVIVES, while this spaces form is accepted and reads back as spaces
// (which ParseMTAnswer's trim models as "" — the two halves of the tag
// normalisation fix meet exactly here).
const mtClearTag = "            " // 12 spaces (mtTagMaxBytes)

// BuildMTSet builds an MT (memory channel tag) Set frame. NON-EMPTY tags
// are variable length, no padding — reference: "MT — MEMORY CHANNEL TAG
// WRITE" Set frame table; golden vectors G8 ("MT0011CALLING FREQ;",
// 12-byte tag, display on), G9 ("MT005040M;", 3-byte tag, display off);
// live-accepted M5b write-trial vectors (hw_derived_m5b_test.go). An
// EMPTY tag is the ONE special case: it is encoded as the all-spaces
// 12-byte clear form (mtClearTag), because the 0-byte form the frame
// grammar would otherwise produce is HW-CONFIRMED REJECTED by the radio
// — both forms are hardware-proven, see mtClearTag's doc comment.
func BuildMTSet(s Slot, display bool, tag string) (Command, error) {
	if !mtSlotValid(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MT: slot must be memory (001-099) or PMS (P1L-P9U); 5xx/EMG rejected by project policy pending M5a, \"000\"/invalid rejected per reference")
	}
	if !validMTTag(tag) {
		return Command{}, newParseError([]byte(tag), "MT: tag must be 0-12 bytes of printable ASCII 0x20-0x7E, excluding ';', with no control bytes")
	}
	if tag == "" {
		tag = mtClearTag
	}

	frame := make([]byte, 0, mtAnswerMinLen+len(tag))
	frame = append(frame, 'M', 'T')
	frame = append(frame, s.Wire()...)
	frame = append(frame, boolDigit(display))
	frame = append(frame, tag...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// BuildMTRead builds an MT read request for slot s. Reference: "Read frame
// (6 bytes): MT P0 P0 P0 ;", golden vector G10: "MT001;".
//
// Unlike BuildMTSet, reads are not restricted to memory/PMS slots: reading
// a tag has no side effect and carries none of the write-direction
// hardware-verification concern the project policy above is about, so 5xx
// and EMG (✓ in the manual's MT column) are allowed here. "000" remains
// rejected (✗ in the manual's MT column, semantics unknown). See
// readableSlot (slot.go), shared with BuildMRRead and AllowedCommand's
// MR/MT grammar checks.
func BuildMTRead(s Slot) (Command, error) {
	if !readableSlot(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MT: slot must not be \"000\"/invalid (reference MT column: ✗)")
	}
	frame := make([]byte, 0, mtReadLen)
	frame = append(frame, 'M', 'T')
	frame = append(frame, s.Wire()...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// ParseMTAnswer strictly parses the SHAPE of an MT Set/Answer frame
// (prefix, slot, display digit, terminator, and the 0-12 byte tag-length
// bound) but — unlike BuildMTSet — does NOT re-validate the tag body's
// charset.
//
// This mirrors ParseIDAnswer's precedent (id.go): the reference documents
// the tag charset only as "ASCII code" and explicitly flags it "TBD at
// M5a" (reference §MT), and both the reference's "Rejection cases" list
// and the Task 3 brief's test list scope charset enforcement to
// BuildMTSet only. Rejecting a genuine radio reply here because it used a
// byte we didn't anticipate would be worse than accepting it: the
// injection concern is a BUILD-time (write-to-radio) property, enforced
// by validMTTag, not a parse-time one.
//
// HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §MT short-form answer):
// the radio DOES pad a short tag with trailing spaces to the full 12
// bytes on read (live probe "MT006;" -> a 4-char tag padded with 8
// trailing spaces) — this was ASSUMED/unknown per the reference (§MT).
// CAT-MT-set-then-read padding is ALSO now HW-CONFIRMED, the hard way:
// the first real-radio production write (13/07/2026, docs/fixtures-private/,
// reproduced by core/clone's TestExecute_LiveBugRepro_UnpaddedTagWriteReadBackPadded)
// wrote an unpadded tag via CAT MT-set and read it back padded to 12
// bytes, which aborted clone's post-write verify with a false mismatch
// (candidate "PROD TEST" vs read-back "PROD TEST   ").
//
// ADJUDICATED FIX (Fix: tag normalisation): padding is a WIRE-ENCODING
// concern only — this function TRIMS trailing spaces (ASCII 0x20 only;
// mid-tag/leading spaces and every other byte are preserved verbatim,
// still passed through with no charset re-validation) before returning,
// so the model's canonical tag form is never space-padded regardless of
// how the radio chose to pad the wire reply. An all-spaces tag (the
// radio's own tag-CLEAR form, hw_derived_m5b_test.go) trims to "",
// matching the model's "no tag" representation exactly. Every caller
// downstream (codeplug JSON, Diff, clone's write-verify compare, the
// GUI, CSV) therefore compares and stores trimmed tags uniformly; see
// codeplug.Load for the mirrored normalisation on the JSON-file side (a
// hand-edited or pre-fix file may still carry an old padded tag).
func ParseMTAnswer(frame []byte) (Slot, bool, string, error) {
	if len(frame) < mtAnswerMinLen || len(frame) > mtAnswerMaxLen {
		return Slot{}, false, "", newParseError(frame, fmt.Sprintf("MT answer must be %d-%d bytes", mtAnswerMinLen, mtAnswerMaxLen))
	}
	if frame[0] != 'M' || frame[1] != 'T' {
		return Slot{}, false, "", newParseError(frame, "MT answer missing \"MT\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return Slot{}, false, "", newParseError(frame, "MT answer missing ';' terminator")
	}
	slot, err := ParseSlot(string(frame[2:5]))
	if err != nil {
		return Slot{}, false, "", newParseError(frame, "MT answer: invalid slot field")
	}
	if slot.IsNone() {
		return Slot{}, false, "", newParseError(frame, "MT answer: slot must not be \"000\" (reference MT column: ✗)")
	}
	display, err := parseBoolDigit(frame[5])
	if err != nil {
		return Slot{}, false, "", newParseError(frame, "MT answer: display field must be '0' or '1'")
	}
	tag := strings.TrimRight(string(frame[6:len(frame)-1]), " ")
	return slot, display, tag, nil
}
