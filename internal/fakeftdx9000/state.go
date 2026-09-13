// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

// MemState is this fake's OWN in-memory representation of one slot's
// content: a memory channel (001-099) or a PMS band-edge (100-117), which on
// this radio are the only two classes there are. It is deliberately NOT
// core/codeplug.Channel and NOT any core/cat/core/driver wire type — this
// package must not import either (doc.go, THE HARD RULE) — so every field is
// stored in the closest thing to raw wire form: single ASCII digit/sign
// bytes and fixed-width digit strings, matching the MR/MW field block layout
// in parser.go byte for byte. Building a reply is then plain concatenation.
//
// No Tag field, unlike internal/fakeft991a's MemState: this radio's record
// has no name/tag byte anywhere (NoTag, matrix §0/§1.6).
type MemState struct {
	// Freq is P2: the 8-digit ASCII frequency field in Hz, zero-padded (e.g.
	// "07000000" = 7.000000 MHz). ONE DIGIT NARROWER than every registered
	// Yaesu sibling's 9-digit field (matrix §2).
	Freq string
	// ClarSign is P3's sign byte, '+' or '-'.
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude, "0000"-"9999".
	ClarMag string
	// RXClar is P4, the RX clarifier flag. STORED (register entry 4).
	RXClar bool
	// TXClar is P5, the TX clarifier flag. STORED (register entry 4).
	TXClar bool
	// Mode is P6, the mode nibble as one ASCII byte: '1'-'9' or 'A'-'C'
	// (matrix §1.5).
	Mode byte
	// Kind is P7 — THE BYTE THIS FAKE ANSWERS WITH (register entry 3),
	// kindMemory for every populated slot, memory or PMS alike. A Set's P7 is
	// a fixed placeholder carrying no channel information, so there is
	// nothing in it worth storing; this field is always the answer's byte.
	Kind byte
	// CTCSS is P8: '0' OFF, '1' ENC/DEC, '2' ENC. Three values — this radio
	// has no DCS state.
	CTCSS byte
	// Tone is P9: the 2-digit ASCII tone-chart index, "00"-"49", LIVE (matrix
	// §1.10) — not a fixed placeholder as some sibling radios' P9 is.
	Tone string
	// Shift is P10: '0' Simplex, '1' Plus Shift, '2' Minus Shift.
	Shift byte
}

// SlotState returns this fake's current stored state for slot and whether any
// state has ever been recorded for it. A test-inspection API: production code
// talks to the fake only through Port(), never this method.
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// CurrentChannel returns this fake's "selected channel" state, as last set by
// an MC-set, or the answer-only none form ("000") if no MC-set has happened
// yet. An MW Set never changes it (register entry 6).
func (r *Radio) CurrentChannel() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
