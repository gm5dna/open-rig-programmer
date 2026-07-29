// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
)

// mtReadLen is the fixed length of an MT read request: "MT" + 3-byte slot
// + ";". Golden vector G10: "MT001;".
const mtReadLen = 6

// mtAnswerMinLen is the SHORT form's structural floor: an MT Set/Answer
// frame with a zero-length tag — "MT"(2) + slot(3) + display(1) + tag(0) +
// ";"(1). It is a property of the frame GRAMMAR, shared by every short-form
// dialect, so it stays a constant.
//
// The MAXIMUM is not. It was a sibling constant here, sized to the FT-710's
// own 12-byte tag, with a comment recording that a dialect with a wider tag
// would need it revisited into a receiver method. M9c-3 is that
// revision: see mtShortAnswerMax below, which adds THIS dialect's
// TagMaxBytes to this floor. The prediction held exactly — the constant was
// never the thing doing the rejecting for either dialect that existed, so
// deleting it moves no FT-710 byte and no FT-710 error text (12 + 7 = 19,
// the number the old constant already carried).
const mtAnswerMinLen = 2 + 3 + 1 + 0 + 1

// mtShortAnswerMax is the longest SHORT-form MT Set/Answer frame THIS
// dialect can produce or accept: the zero-tag floor plus its own tag width.
// FT-710: 7 + 12 = 19, byte-identical to the constant it replaces.
//
// A receiver method rather than a package constant because it reaches the
// OUTBOUND WRITE GATE (allowlist.go's validMTCommand) as well as
// ParseMTAnswer, and a gate consulting the FT-710's width on every dialect
// is the exact shape this seam exists to eliminate: a 6-byte-tag family
// would have had frames up to 19 bytes admitted as "within the window",
// leaving only the tag charset check between a forged frame and the radio.
//
// It is meaningful only under MTFormShort; both callers refuse any other
// form before consulting it.
func (d Dialect) mtShortAnswerMax() int {
	return mtAnswerMinLen + d.mt.TagMaxBytes
}

// MTAnswerBounds returns the validated answer-length bounds, in bytes, for
// this dialect's MT form: [min, max] inclusive. The short form's window is
// its structural floor plus its own tag width (FT-710: 7-19); the combined
// form's length is EXACT, so its bounds are equal (29 + TagMaxBytes, 41 for
// the evidenced 12-byte tag).
//
// An unconfigured dialect gets an ERROR, not zeros. This is the
// receiver-derived geometry M9c-5's transport spec factories consume
// instead of hardcoding 41 or re-deriving 29+TagMaxBytes for themselves, and
// a driver reading a plausible (0, 0) would build a CommandSpec that admits
// no answer at all and says nothing about why.
//
// It lives here rather than in mtcombined.go because it spans both forms:
// the whole point is that a caller asks the dialect for its geometry instead
// of branching on the form itself.
func (d Dialect) MTAnswerBounds() (min, max int, err error) {
	switch d.mt.Form {
	case MTFormShort:
		return mtAnswerMinLen, d.mtShortAnswerMax(), nil
	case MTFormCombined:
		n := d.mtCombinedLen()
		return n, n, nil
	default:
		return 0, 0, newParseError(nil, fmt.Sprintf("MT: no answer bounds for a %v dialect — a form must be configured before its frame geometry can be derived", d.mt.Form))
	}
}

// MTForm reports which MT frame shape this dialect declares. The zero
// Dialect reports MTFormUnspecified — a form it can never be given, since
// NewDialect refuses a config omitting one.
//
// Exported for consumers OUTSIDE this package that must branch on the form:
// core/cat/dialecttest's conformance suite (which cannot see the unexported
// field, and is what M9c-4 runs over the real FTdx10 dialect), and M9c-4/5's
// per-model packages.
func (d Dialect) MTForm() MTForm { return d.mt.Form }

// MWWriteKind is the single P7 "kind" byte this family accepts on every
// memory write. FT-710: KindMemory.
//
// Exported for core/driver/ft710's write path, which hardcodes cat.KindMemory
// into the MemoryData it builds (M9c-3 task 9) and so writes the FT-710's
// byte whatever dialect it was handed, and for core/cat/dialecttest, which
// needs it to build a valid MW frame for an arbitrary dialect.
func (d Dialect) MWWriteKind() byte { return d.mwWriteKind }

// Clarifier returns this family's clarifier step and range.
//
// Exported for core/driver/ft710's write path, which pre-checks the
// clarifier against a hardcoded +/-9990 before any wire traffic (M9c-3 task
// 9), and for core/cat/dialecttest.
func (d Dialect) Clarifier() ClarifierPolicy { return d.clar }

// mtSlotValid reports whether s is a legal MT SET target under project
// policy AND THIS DIALECT'S slot space: memory or PMS slots only.
//
// The manual's slot table marks 5xx and EMG as ✓ for MT — but reference
// §MT states explicitly: "our policy: reject sets to 5xx/EMG until
// hardware-verified" (a project decision, not a manual requirement,
// repeated verbatim in the Task 3 brief). "000" is ✗ for MT in the manual
// itself (semantics unknown, do not emit) and is rejected unconditionally
// by d.classifySlot returning slotKindNone/slotKindInvalid here.
//
// It classifies through d.classifySlot, not the package-level
// classifySlotWire: this helper is shared by BuildMTSet and
// AllowedCommand's MT Set grammar check, both Dialect methods, and either
// consulting a global would decide against the FT-710 whatever dialect it
// was called on.
func (d Dialect) mtSlotValid(s Slot) bool {
	switch d.classifySlot(s.Wire()) {
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
// and contains only validMTTagByte bytes, against THIS DIALECT'S tag-length
// policy (d.mt.TagMaxBytes). Any byte >= 0x80 (which includes every byte of
// a multi-byte UTF-8 rune such as 'é') is rejected by the charset check
// alone, before the length check is even relevant.
//
// A Dialect method rather than a package function taking a fixed constant,
// because TagMaxBytes varies by radio family (Dialect.mt, task 62) — and
// this is reached by the OUTBOUND WRITE GATE, allowlist.go's
// AllowedCommand -> validMTCommand, so a hardwired bound here would let the
// gate authorise, on every dialect, whatever tag length the FT-710 happens
// to accept (or refuse a wider dialect's genuinely valid tag).
func (d Dialect) validMTTag(tag string) bool {
	if len(tag) > d.mt.TagMaxBytes {
		return false
	}
	for i := 0; i < len(tag); i++ {
		if !validMTTagByte(tag[i]) {
			return false
		}
	}
	return true
}

// clearTagForm is the wire form BuildMTSet emits for an EMPTY tag under
// THIS dialect: TagMaxBytes repetitions of ClearTagByte.
//
// HW-PROBED 13/07/2026, FT-710 ONLY (docs/fixtures-private/
// m5b-trials.private-capture, stages tagclear/tagclear2;
// docs/hardware-notes.md §Empty-slot create, tag-clear): the 0-byte-tag
// Set form ("MT0960;") is REJECTED ("?;", ~4 ms) and the existing tag
// SURVIVES, while the all-spaces 12-byte form is accepted and reads back
// as spaces (which ParseMTAnswer's trim models as "" — the two halves of
// the tag normalisation fix meet exactly here). No equivalent hardware
// evidence exists for any other dialect: this generalises the MECHANISM
// (fill the field with the dialect's own clear byte) onto the receiver,
// not the specific 12-space FT-710 form, so FT710.clearTagForm() is still
// exactly the same 12-space string as before while a dialect with its own
// TagMaxBytes/ClearTagByte gets its own.
func (d Dialect) clearTagForm() string {
	return strings.Repeat(string(d.mt.ClearTagByte), d.mt.TagMaxBytes)
}

// BuildMTSet builds an MT (memory channel tag) Set frame. NON-EMPTY tags
// are variable length, no padding — reference: "MT — MEMORY CHANNEL TAG
// WRITE" Set frame table; golden vectors G8 ("MT0011CALLING FREQ;",
// 12-byte tag, display on), G9 ("MT005040M;", 3-byte tag, display off);
// live-accepted M5b write-trial vectors (hw_derived_m5b_test.go). An
// EMPTY tag is the ONE special case: it is encoded as this dialect's own
// clear form (d.clearTagForm()), because the 0-byte form the frame
// grammar would otherwise produce is HW-CONFIRMED REJECTED by the
// FT-710 — both forms are hardware-proven for the FT-710, see
// clearTagForm's doc comment.
func (d Dialect) BuildMTSet(s Slot, display bool, tag string) (Command, error) {
	// FORM FIRST. This builder emits ONE shape, and a dialect declaring the
	// other must be refused here rather than handed a frame whose layout was
	// never its own: the combined form's record begins "MT" and ends ';'
	// too, so the radio would be sent a plausible-looking frame carrying its
	// tag where its slot field belongs.
	//
	// The error carries no input frame because the offending value is the
	// RECEIVER, not any argument.
	if d.mt.Form != MTFormShort {
		return Command{}, newParseError(nil, fmt.Sprintf("MT: short-form Set called on a %v dialect — use the combined-form API", d.mt.Form))
	}
	if !d.mtSlotValid(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MT: slot must be memory (001-099) or PMS (P1L-P9U); 5xx/EMG rejected by project policy pending M5a, \"000\"/invalid rejected per reference")
	}
	if !d.validMTTag(tag) {
		return Command{}, newParseError([]byte(tag), fmt.Sprintf("MT: tag must be 0-%d bytes of printable ASCII 0x20-0x7E, excluding ';', with no control bytes", d.mt.TagMaxBytes))
	}
	// A non-empty tag identical to the clear form round-trips to "". That
	// is the PROTOCOL'S SEMANTICS, not data loss, and it must not be
	// rejected here.
	//
	// The third milestone-review round read it as a defect and this code
	// briefly refused such a tag — which broke
	// TestBuildMTSet_HWDerived_M5b_TagSetAndClear, because the M5b trial
	// CLEARED a tag on the real FT-710 by sending exactly twelve spaces
	// and the radio accepted it. Sending a full field of the clear byte IS
	// how a tag is cleared; getting "" back is the correct reading of it.
	// The finding was verified to describe real behaviour and then
	// rejected on hardware evidence.
	if tag == "" {
		tag = d.clearTagForm()
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
// Dialect.readableSlot (slot.go), shared with BuildMRRead and
// AllowedCommand's MR/MT grammar checks.
func (d Dialect) BuildMTRead(s Slot) (Command, error) {
	if !d.readableSlot(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MT: slot must not be \"000\"/invalid (reference MT column: ✗)")
	}
	frame := make([]byte, 0, mtReadLen)
	frame = append(frame, 'M', 'T')
	frame = append(frame, s.Wire()...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// ParseMTAnswer strictly parses the SHAPE of a SHORT-FORM MT Set/Answer
// frame (prefix, slot, display digit, terminator, and the length window
// this dialect's own tag width derives — see mtShortAnswerMax) but — unlike
// BuildMTSet — does NOT re-validate the tag body's charset. It refuses any
// dialect that is not MTFormShort.
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
// concern only — this function normalises it via d.decodeMTTag, which
// recognises a full field of this dialect's own ClearTagByte as an empty
// tag, and trims trailing PadByte only when the dialect DECLARES one.
// A dialect with PadByte 0 declares no padding and gets its field back
// verbatim: nothing is trimmed from it at all. Leading and mid-tag bytes
// are always preserved, still passed through with no charset
// re-validation. For the FT-710 this is ASCII 0x20 (space) exactly as before —
// FT710.ParseMTAnswer's behaviour is byte-identical to the pre-M9c-0
// hardcoded strings.TrimRight(tag, " "). An all-clear-byte tag (the
// FT-710's own tag-CLEAR form, hw_derived_m5b_test.go — TagMaxBytes
// repetitions of ClearTagByte, see clearTagForm) trims to "", matching
// the model's "no tag" representation exactly, and this is the direction
// that MUST move together with BuildMTSet's own clear-form emission: a
// receiver-aware BUILD paired with a hardcoded-to-spaces DECODE would
// parse a non-FT-710 dialect's own cleared tag back as a string of its
// clear bytes rather than empty (mtpolicy_test.go's
// TestMTPolicy_ClearFormDecodeIsNotJustSpaces). Every caller downstream
// (codeplug JSON, Diff, clone's write-verify compare, the GUI, CSV)
// therefore compares and stores trimmed tags uniformly; see codeplug.Load
// for the mirrored normalisation on the JSON-file side (a hand-edited or
// pre-fix file may still carry an old padded tag).
func (d Dialect) ParseMTAnswer(frame []byte) (Slot, bool, string, error) {
	// FORM FIRST, symmetric with BuildMTSet: this parser reads ONE layout,
	// and a combined-form frame would yield data from the wrong offsets
	// rather than an error, since it shares this one's prefix and
	// terminator.
	if d.mt.Form != MTFormShort {
		return Slot{}, false, "", newParseError(frame, fmt.Sprintf("MT: short-form answer parsed on a %v dialect — use the combined-form API", d.mt.Form))
	}
	if len(frame) < mtAnswerMinLen || len(frame) > d.mtShortAnswerMax() {
		return Slot{}, false, "", newParseError(frame, fmt.Sprintf("MT answer must be %d-%d bytes", mtAnswerMinLen, d.mtShortAnswerMax()))
	}
	if frame[0] != 'M' || frame[1] != 'T' {
		return Slot{}, false, "", newParseError(frame, "MT answer missing \"MT\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return Slot{}, false, "", newParseError(frame, "MT answer missing ';' terminator")
	}
	slot, err := d.ParseSlot(string(frame[2:5]))
	if err != nil {
		return Slot{}, false, "", newParseError(frame, "MT answer: invalid slot field")
	}
	// d.classifySlot, not slot.IsNone(): the latter classifies through
	// classifySlotWire (i.e. the FT-710) whatever dialect this method was
	// called on, and "which wire form means none" is dialect data
	// (slotSpace.noneWire).
	if d.classifySlot(slot.Wire()) == slotKindNone {
		return Slot{}, false, "", newParseError(frame, "MT answer: slot must not be \"000\" (reference MT column: ✗)")
	}
	display, err := parseBoolDigit(frame[5])
	if err != nil {
		return Slot{}, false, "", newParseError(frame, "MT answer: display field must be '0' or '1'")
	}
	tag := d.decodeMTTag(string(frame[6 : len(frame)-1]))
	return slot, display, tag, nil
}

// decodeMTTag turns an MT answer's raw tag field into a tag value.
//
// TWO DISTINCT THINGS, which this code conflated until the M9c-0 milestone
// review (finding 4):
//
//  1. THE CLEAR FORM. An empty tag is written as exactly TagMaxBytes
//     repetitions of ClearTagByte, so exactly that field means "no tag".
//     This is dialect data, and it is what BuildMTSet emits.
//
//  2. PADDING. The radio may pad a short tag to fill the field. WHICH
//     byte it pads with is the dialect's own declaration, MTPolicy.PadByte
//     — not a space by assumption, and not derived from ClearTagByte.
//     PadByte 0 means the family declares no padding at all.
//
// The previous version trimmed every trailing ClearTagByte, which is right
// for the FT-710 only because its clear byte IS a space, so the two rules
// coincide. For a dialect clearing with any other byte it silently
// destroyed data: on a peer clearing with '-', the tag "CALL-" built,
// passed the gate, and came back as "CALL".
//
// FT-710 behaviour is unchanged in both directions. A full field of spaces
// still decodes to "" by rule 1, and "CALL" followed by spaces still
// decodes to "CALL" by rule 2 — the same string TrimRight produced.
func (d Dialect) decodeMTTag(raw string) string {
	// 1. The clear form: exactly a full field of this dialect's own clear
	//    byte. Dialect data, and what BuildMTSet emits for an empty tag.
	if w := d.mt.TagMaxBytes; w > 0 && len(raw) == w && raw == strings.Repeat(string(d.mt.ClearTagByte), w) {
		return ""
	}
	// 2. Padding, from the dialect's OWN declaration.
	//
	// PadByte 0 means the family declares no padding and its answers come
	// back verbatim. Deriving this from ClearTagByte instead — "trim
	// spaces if the clear byte is a space" — is what the second repair
	// did, and it merely relocated the assumption: a constructed dialect
	// declaring a space clear byte inherited the FT-710's padding without
	// ever declaring it.
	if d.mt.PadByte != 0 {
		return strings.TrimRight(raw, string(d.mt.PadByte))
	}
	return raw
}
