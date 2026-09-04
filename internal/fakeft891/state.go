// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

// MemState is this fake's OWN in-memory representation of one memory slot's
// content: a memory channel, a PMS pair member, a 5 MHz channel, or EMG. It
// is deliberately NOT core/codeplug.Channel and NOT any core/cat wire type —
// fakeft891 must not import either (see doc.go, THE HARD RULE) — so every
// field is stored in the closest thing to raw wire form: single ASCII
// digit/sign bytes and fixed-width digit strings, matching the combined MT
// and MR frame layouts in parser.go byte for byte. Building a reply is then
// plain concatenation, with no numeric/text conversion anywhere, and no
// opportunity for this fake to "fix up" a value a real radio would echo back
// as it stands.
//
// The fields are the 41-position combined record's, which contains the
// 28-position MR block as its first 27 positions, so one struct serves both
// answers (ft891_layout.txt:996-1027 and 968-975; the two counts in
// core/cat/ft891/testdata/provenance.md §MT and §MR).
//
// TWO OF internal/fakedx10's FIELDS ARE ABSENT AND ONE IS NEW, and each is
// this radio's manual rather than a preference:
//
//   - No TXClar. Position 21 is P5, and this radio's legend prints it
//     "0: (Fixed)" on all five blocks that carry the field (MR 971, MT 1006,
//     MW 1042, IF 783, OI 1129) where its registered siblings print
//     `0: TX CLAR "OFF" 1: TX CLAR "ON"`. There is no TX clarifier state on
//     the wire in either direction, so there is none to store. What IS here
//     is P5 below, the schema byte itself.
//   - No Populated. internal/fakeradio needs it because the FT-710's short MT
//     frame writes a tag ALONE, so a slot can exist with a tag and no channel
//     data. The combined form cannot express that: every MT Set carries the
//     whole record. Presence in the slot map IS populated here, and that is
//     the whole of the empty-slot rule (doc.go's register entry EMPTY-SLOT
//     ANSWERS).
//   - TagDisplay is NEW, and it is this radio's one genuinely new axis. MT's
//     P11 legend prints `0: TAG "OFF" 1: TAG "ON"` (1016) where every
//     registered combined-form sibling prints "0: (Fixed)", so byte 28 is a
//     LIVE per-channel flag this fake stores and answers back.
type MemState struct {
	// Freq is P2: the 9-digit ASCII frequency field in Hz, zero-padded
	// (e.g. "014250000" = 14.250000 MHz). Combined-MT positions 6-14; MR
	// answer positions 6-14.
	Freq string
	// ClarSign is P3's sign byte, '+' or '-'. Position 15.
	//
	// STORED, not zeroed — doc.go's register entry THE CLARIFIER IS STORED,
	// the deliberate non-borrowing of the FT-710's clarifier finding.
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude. Positions 16-19. STORED, for
	// the same reason. The accepted range is this manual's printed
	// "Clarifier Offset: 0000 - 9999 (Hz)" and NOT the dialect's narrower
	// deduction — see validClarMagDigits.
	ClarMag string
	// RXClar is P4, the RX clarifier flag. Position 20. STORED.
	RXClar bool
	// P5 is position 21, documented "0: (Fixed)" in both directions.
	//
	// THE ZERO VALUE MEANS THAT FIXED '0'. P5 is schema rather than channel
	// data — every honest slot carries the same byte — so requiring every
	// image literal and every WithSlot call to spell a constant would add
	// noise to each of them and information to none. The field exists so that
	// a test can craft the answer this radio is ASSUMED never to give: the
	// driver register's "P5 IS ANSWERED '0'" entry records that a single
	// non-'0' byte would convert core/cat's parse posture from strict to
	// tolerant, and a fake that could not play that radio would leave the
	// refutation unreachable. TestMRAnswer_CarriesP5AsTheFixedZero pins both
	// halves. The defaulting lives in the answer builder rather than in a
	// constructor a caller could bypass.
	P5 byte
	// Mode is P6, the mode nibble as one ASCII byte. Position 22. This
	// radio's legend — printed identically beside MR (972-974), MT
	// (1007-1010) and MW (1043-1046) — runs 1..9 then B, C, D, with a
	// PRINTED HOLE at 'A' ("A: -") and no 'E' or 'F' entry at all, plus '0'
	// for the "-" placeholder the dialect includes as ASSUMED (its register
	// entry "THE cat.ModeUnset MEMBER OF THE MODE TABLE", cited).
	Mode byte
	// Kind is P7 — THE BYTE THIS FAKE ANSWERS WITH, which is not the byte a
	// Set carries.
	//
	// MT's own legend prints "P7 0: (Fixed)" (1011) and NO read vocabulary at
	// all; MR's prints "P7 0: VFO 1: Memory" (976). So the Set direction's P7
	// is a fixed placeholder carrying no channel information — there would be
	// nothing to learn from storing it, and echoing it back would answer
	// "VFO" for a memory channel — and the ANSWER direction's is read across
	// from MR's legend, which is doc.go's register entry P7 IN AN MT ANSWER.
	// Every Set arm therefore stores the ANSWER kind, and this fake's images
	// populate it the same way.
	//
	// It is a field rather than a constant so that a test can craft a slot
	// answering something else (a '0', or a byte outside the tolerated pair)
	// and drive a real driver's parse-error path through a real fake. Nothing
	// in this package ever writes anything but kindMemory into it.
	Kind byte
	// CTCSS is P8: '0' OFF, '1' ENC/DEC, '2' ENC — three values and no
	// fourth, printed identically on MR (977), MT (1012), MW (1048), IF (790)
	// and OI (1136). Position 24. (P9, positions 25-26, is the documented
	// fixed "00" and is carried implicitly by the answer builders rather than
	// stored.)
	CTCSS byte
	// Shift is P10: '0' Simplex, '1' Plus Shift, '2' Minus Shift (MR 979, MT
	// 1015, MW 1050, IF 792, OI 1138). Position 27.
	Shift byte
	// TagDisplay is P11, position 28: this radio's LIVE TAG display flag,
	// `0: TAG "OFF" 1: TAG "ON"` (1016). A plain bool rather than a
	// sentinel-carrying byte, because unlike P5 it is channel data with two
	// legal values and no schema constant to default to — a live flag is
	// never defaulted, which is the same ruling that makes core/cat's
	// display-LESS combined pair refuse under this dialect
	// (core/cat/ft891/dialect.go's MTPolicy.P11 comment).
	TagDisplay bool
	// Tag is P12's text, positions 29-40: 0-12 ASCII bytes stored with
	// trailing fill TRIMMED, and re-padded to the full 12-byte field on every
	// answer (doc.go's register entry THE TAG IS STORED TRIMMED AND ANSWERED
	// PADDED; the fill byte is a space because the dialect says so — its
	// register entry "MTPolicy.TagFill", cited). A tag longer than 12 bytes
	// cannot arrive from the wire, since the field is 12 bytes wide; one
	// supplied through WithSlot is truncated by the answer builder rather
	// than overflowing the frame.
	Tag string
}

// SlotState returns this fake's current stored state for slot and whether any
// state has ever been recorded for it. It is a test-inspection API:
// production code (the transport engine, the driver) talks to the fake only
// through Port(), never this method.
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// EXState returns this fake's current stored raw P4 for a FOUR-digit EX (MENU)
// wire address and whether it holds any entry for it. A false second return is
// exactly what makes an EX read of addr answer "?;" (ex.go's handleEX, and
// doc.go's register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;"). Like
// SlotState it is a test-inspection API: production code reaches the menu state
// only through Port().
func (r *Radio) EXState(addr string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.exSettings[addr]
	return v, ok
}

// CurrentChannel returns this fake's "selected channel" state, as last set by
// an MC-set, or the answer-only none form ("000") if no MC-set has happened
// yet. A Set never changes it — doc.go's register entry A SET DOES NOT MOVE
// THE SELECTED CHANNEL, pinned by TestSetsDoNotMoveTheSelection.
func (r *Radio) CurrentChannel() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
