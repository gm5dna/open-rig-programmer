// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

// MemState is this fake's OWN in-memory representation of one memory slot's
// content: a memory channel, a PMS pair member, a 5 MHz channel, or EMG. It is
// deliberately NOT core/codeplug.Channel and NOT any core/cat wire type —
// fakedx101 must not import either (see doc.go, THE HARD RULE) — so every field
// is stored in the closest thing to raw wire form: single ASCII digit/sign
// bytes and fixed-width digit strings, matching the combined MT and MR frame
// layouts in parser.go byte for byte. Building a reply is then plain
// concatenation, with no numeric/text conversion anywhere, and no opportunity
// for this fake to "fix up" a value a real radio would echo back as it stands.
//
// The fields are the 41-byte combined record's, whose first 27 positions carry
// the same 25-position field block (chart positions 3-27) that the 28-byte
// MR/MW frame carries — that frame's own ';' sits at 28, where the combined
// record continues with P11 — so one struct serves both answers.
//
// ONE STRUCT FOR BOTH MODELS. Nothing in it is model-conditional, because
// nothing in the manual's memory-channel surface is: the MT, MR and MW blocks
// and every one of their legends are printed once for both radios and carry no
// model qualifier (matrix §2.5's sweep). The D/MP distinction lives in
// Radio.catID and nowhere else in this package.
//
// THREE OF fakeradio's FIELDS ARE ABSENT, and each absence is a decision:
//
//   - No TagDisplay. The combined record has no display flag at any position —
//     P11 is the manual's "0: (Fixed)" (layout 1329), which is matrix §3.7's
//     manual-evidenced absence and what makes core/driver/ftdx101 report
//     codeplug.Unavailable for it. There is nothing to store.
//   - No Populated. fakeradio needs it because the FT-710's short MT frame
//     writes a tag ALONE, so a slot can exist with a tag and no channel data.
//     The combined form cannot express that: every MT Set carries the whole
//     record. Presence in the slot map IS populated here, and that is the whole
//     of the empty-slot rule (doc.go register entry 1).
//   - No separate stored kind on the SET side. See Kind below.
type MemState struct {
	// Freq is P2: the 9-digit ASCII frequency field in Hz, zero-padded (e.g.
	// "014250000" = 14.250000 MHz). Combined-MT positions 6-14; MR answer
	// positions 6-14.
	Freq string
	// ClarSign is P3's sign byte, '+' or '-'. Position 15.
	//
	// STORED, not zeroed — doc.go register entry 5, the deliberate
	// non-borrowing of the FT-710's clarifier finding. The '-' spelling itself
	// is the DIALECT's ASSUMED minus-direction byte (this manual prints that
	// direction as a two-hyphen glyph), cited at validClarSign.
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude, 0000-9990 Hz on the dialect's
	// ASSUMED 10 Hz step ("ClarifierPolicy.StepHz = 10", cited). Positions
	// 16-19. STORED — register entry 5.
	ClarMag string
	// RXClar is P4, the RX clarifier flag. Position 20. STORED — entry 5.
	RXClar bool
	// TXClar is P5, the TX clarifier flag. Position 21. STORED — entry 5.
	TXClar bool
	// Mode is P6, the mode nibble as one ASCII byte: '1'-'9' or 'A'-'F' per
	// this radio's own legend, printed identically beside five commands and
	// running 1 to F with no other member, plus '0' for the "-" placeholder the
	// dialect includes as ASSUMED (its ModeUnset entry, cited). Position 22.
	Mode byte
	// Kind is P7 — THE BYTE THIS FAKE ANSWERS WITH, which is not the byte a Set
	// carries.
	//
	// The chart's legend reads "Set: 0: (Fixed) / Read: 0: VFO 1: Memory"
	// (layout 1324): the Set direction's P7 is a fixed placeholder carrying no
	// channel information at all, so there would be nothing to learn from
	// storing it, and echoing it back would answer "VFO" for a memory channel.
	// Every Set arm therefore stores the ANSWER kind, and this fake's images
	// populate it the same way: '1' (Memory), for memory channels AND for PMS,
	// 5xx and EMG slots alike — doc.go register entries 2 and 3.
	//
	// It is a field rather than a constant so that a test can craft a slot
	// answering something else (a '0', or a byte outside the documented pair)
	// and drive a real driver's parse-error path through a real fake. Nothing
	// in this package ever writes anything but kindMemory into it.
	Kind byte
	// CTCSS is P8: '0' off, '1' ENC/DEC, '2' ENC (layout 1325). Position 24.
	CTCSS byte
	// Shift is P10: '0' simplex, '1' plus, '2' minus (layout 1327). Position
	// 27. (P9, positions 25-26, is the documented fixed "00" and is carried
	// implicitly by the answer builders rather than stored.)
	Shift byte
	// P11 is the combined record's position 28, documented "0: (Fixed)".
	//
	// THE ZERO VALUE MEANS THAT FIXED '0'. P11 is schema rather than channel
	// data — every honest slot carries the same byte — so requiring every image
	// literal and every WithSlot call to spell a constant would add noise to
	// each of them and information to none. The field exists only so that a
	// test can craft a malformed answer deliberately, which is also why the
	// defaulting lives in the answer builder rather than in a constructor a
	// caller could bypass.
	P11 byte
	// Tag is P12's text, positions 29-40: 0-12 ASCII bytes stored with trailing
	// fill TRIMMED, and re-padded to the full 12-byte field on every answer
	// (doc.go register entry 13; the fill byte is a space because the dialect
	// says so — its "MTPolicy.TagFill = ' '" entry, cited). A tag longer than
	// 12 bytes cannot arrive from the wire, since the field is 12 bytes wide;
	// one supplied through WithSlot is truncated by the answer builder rather
	// than overflowing the frame.
	Tag string
}

// SlotState returns this fake's current stored state for slot and whether any
// state has ever been recorded for it. It is a test-inspection API: production
// code (the transport engine, the driver) talks to the fake only through
// Port(), never this method.
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// CurrentChannel returns this fake's "selected channel" state, as last set by
// an MC-set, or the answer-only none form ("000") if no MC-set has happened
// yet. A Set never changes it — doc.go register entry 10.
func (r *Radio) CurrentChannel() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
