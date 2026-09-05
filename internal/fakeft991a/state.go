// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

// MemState is this fake's OWN in-memory representation of one slot's content:
// a memory channel (001-099) or a PMS band-edge (100-117), which on this radio
// are the only two classes there are. It is deliberately NOT
// core/codeplug.Channel and NOT any core/cat wire type — fakeft991a must not
// import either (see doc.go, THE HARD RULE) — so every field is stored in the
// closest thing to raw wire form: single ASCII digit/sign bytes and
// fixed-width digit strings, matching the combined MT and MR frame layouts in
// parser.go byte for byte. Building a reply is then plain concatenation, with
// no numeric/text conversion anywhere, and no opportunity for this fake to
// "fix up" a value a real radio would echo back as it stands.
//
// The fields are the 41-position combined record's, which contains the
// 28-position MR block as its first 27 positions, so one struct serves both
// answers (ft991a_layout.txt:998-1033 and 965-981; the two counts in
// core/cat/ft991a/testdata/mt-vectors.golden and mr-vectors.golden).
//
// TWO FIELDS SPLIT THIS RADIO FROM internal/fakeft891, AND THEY SPLIT IT IN
// OPPOSITE DIRECTIONS. Each is this manual's legend rather than a preference:
//
//   - TXClar EXISTS HERE. Position 21 is P5, printed
//     `0: TX CLAR "OFF" 1: TX CLAR "ON"` on every block that carries the field
//     (MR 971, MT 1004, MW 1042, IF 787, OI 1122), where the FT-891 prints
//     "0: (Fixed)" on all five of its own. So there IS a TX clarifier state on
//     the wire in both directions, and this fake stores it.
//   - NO TagDisplay. Position 28 is P11, printed "0: (Fixed)" (1015), where
//     the FT-891 prints `0: TAG "OFF" 1: TAG "ON"` and the byte is a live
//     per-channel flag. There is no display flag in this record to store; what
//     IS here is P11 below, the schema byte itself.
//
// No Populated field, for internal/fakedx10's reason: the combined form cannot
// express a slot that exists with a tag and no channel data, because every MT
// Set carries the whole record. Presence in the slot map IS populated here,
// and that is the whole of the empty-slot rule (doc.go's register entry
// EMPTY-SLOT ANSWERS).
type MemState struct {
	// Freq is P2: the 9-digit ASCII frequency field in Hz, zero-padded
	// (e.g. "014250000" = 14.250000 MHz). Positions 6-14 in both frames.
	Freq string
	// ClarSign is P3's sign byte, '+' or '-'. Position 15.
	//
	// STORED, not zeroed — doc.go's register entry THE CLARIFIER IS STORED,
	// the deliberate non-borrowing of the FT-710's clarifier finding.
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude. Positions 16-19. STORED, for
	// the same reason. The accepted range is this manual's printed "Clarifier
	// Offset: 0000 - 9999 (Hz)" and NOT the dialect's narrower deduction —
	// see validClarMagDigits.
	ClarMag string
	// RXClar is P4, the RX clarifier flag. Position 20. STORED.
	RXClar bool
	// TXClar is P5, the TX clarifier flag. Position 21. STORED.
	//
	// THIS FIELD IS THE HALF internal/fakeft891 DOES NOT HAVE: this radio's
	// five blocks print P5 as a live TX clarifier state (971, 1004, 1042, 787,
	// 1122) where that radio's print "0: (Fixed)". A bool rather than a
	// sentinel-carrying byte, because it is channel data with two legal values
	// and no schema constant to default to — the same reasoning P11 below
	// inverts.
	TXClar bool
	// Mode is P6, the mode nibble as one ASCII byte. Position 22. This
	// radio's legend — printed identically beside five commands, MR
	// (973-975), MT (1006-1008), MW (1044-1046), IF (789-791) and OI
	// (1124-1126) — runs 1..9 then A..E with NO hole and no 'F', plus '0' for
	// the "-" placeholder the dialect includes as ASSUMED (its register entry
	// "THE cat.ModeUnset MEMBER OF THE MODE TABLE", cited).
	Mode byte
	// Kind is P7 — THE BYTE THIS FAKE ANSWERS WITH, which is not the byte a
	// Set carries.
	//
	// THIS MANUAL PRINTS BOTH DIRECTIONS IN ONE LEGEND: "P7 Set: 0: (Fixed) /
	// Read: 0: VFO 1: Memory" (ft991a_layout.txt:1009), where the FT-891
	// prints no read vocabulary beside MT at all and its fake has to read the
	// answer's domain across from MR. So the Set direction's P7 is a fixed
	// placeholder carrying no channel information — there would be nothing to
	// learn from storing it — and the ANSWER direction's is printed. Every Set
	// arm therefore stores the ANSWER kind, and this fake's images populate it
	// the same way.
	//
	// It is a field rather than a constant so that a test can craft a slot
	// answering something else (a '0', or a byte outside the printed pair) and
	// drive a real driver's parse-error path through a real fake. Nothing in
	// this package ever writes anything but kindMemory into it.
	Kind byte
	// CTCSS is P8: '0' CTCSS OFF, '1' CTCSS ENC/DEC, '2' CTCSS ENC, '3' DCS
	// ENC/DEC, '4' DCS ENC — FIVE values, printed identically on IF (795-796),
	// MR (977-978), MT (1010-1011), MW (1048-1049) and OI (1128-1129).
	// Position 24. Every registered sibling prints three, which is what makes
	// this the axis this radio exists to exercise. (P9, positions 25-26, is the
	// documented fixed "00" and is carried implicitly by the answer builders
	// rather than stored.)
	CTCSS byte
	// Shift is P10: '0' Simplex, '1' Plus Shift, '2' Minus Shift (MR 981, MT
	// 1014, MW 1051, IF 799, OI 1132). Position 27.
	Shift byte
	// P11 is position 28, documented "0: (Fixed)" in both directions
	// (ft991a_layout.txt:1015).
	//
	// THE ZERO VALUE MEANS THAT FIXED '0'. P11 is schema rather than channel
	// data — every honest slot carries the same byte — so requiring every
	// image literal and every WithSlot call to spell a constant would add
	// noise to each of them and information to none. The field exists so that
	// a test can craft the answer this radio is ASSUMED never to give and
	// drive a real driver's parse-error path through a real fake;
	// TestMTAnswer_CarriesP11AsTheFixedZero pins both halves. The defaulting
	// lives in the answer builder rather than in a constructor a caller could
	// bypass.
	P11 byte
	// Tag is P12's text, positions 29-40: 0-12 ASCII bytes stored with
	// trailing fill TRIMMED, and re-padded to the full 12-byte field on every
	// answer (doc.go's register entry THE TAG IS STORED TRIMMED AND ANSWERED
	// PADDED; the fill byte is a space because the dialect says so — its
	// register entry "MTPolicy.TagFill = ' '", cited). A tag longer than 12
	// bytes cannot arrive from the wire, since the field is 12 bytes wide; one
	// supplied through WithSlot is truncated by the answer builder rather than
	// overflowing the frame.
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

// EXState returns this fake's current stored raw P4 for a THREE-digit EX (MENU)
// wire address and whether it holds any entry for it. A false second return is
// exactly what makes an EX read of addr answer "?;" (ex.go's handleEX, and
// doc.go's register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;") — which
// is the answer 087 draws, since the chart prints no parameter for it and
// neither transcription's generator admits it (plan P18). Like SlotState it is
// a test-inspection API: production code reaches the menu state only through
// Port().
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
