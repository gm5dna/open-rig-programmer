// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx1200

// MemState is this fake's OWN in-memory representation of one slot's
// content — a memory channel (001-099) or a PMS band-edge (100-117), the
// only two classes this radio has. It is deliberately NOT core/codeplug's
// channel type and NOT any core/cat wire type (doc.go, THE HARD RULE) —
// every field is stored in the closest thing to raw wire form, matching the
// 24-byte field block MW's Set and MR's Answer both carry (parser.go), so
// building a reply is plain concatenation with no numeric conversion
// anywhere.
//
// NO Tag FIELD (this radio's NoTag, matrix §0) and NO Tone FIELD (P9 is
// printed-fixed "00" on both read and write, matrix §1.3 — doc.go's
// register entry P9 (TONE) IS PRINTED-FIXED — so there is no live value to hold).
type MemState struct {
	// Freq is P2: the 8-digit ASCII frequency field in Hz, zero-padded (e.g.
	// "07000000" = 7.000000 MHz), matrix §1.1.
	Freq string
	// ClarSign is P3's sign byte, '+' or '-'.
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude, 0000-9999 Hz.
	ClarMag string
	// RXClar is P4, '0'/'1'.
	RXClar bool
	// TXClar is P5, '0'/'1'.
	TXClar bool
	// Mode is P6, one ASCII byte from the eleven-member legend with a hole
	// at 'A' (matrix §1.2): 1 LSB, 2 USB, 3 CW, 4 FM, 5 AM, 6 RTTY-LSB, 7
	// CW-R, 8 DATA-LSB, 9 RTTY-USB, B FM-N, C DATA-USB. 'A' is never a legal
	// value here — see validModeByte.
	Mode byte
	// Kind is P7 as MR answers it — "0: VFO 1: Memory" (matrix §1.4).
	// doc.go's register entry AN ANSWER'S KIND BYTE IS ALWAYS '1': every populated slot this fake stores,
	// memory or PMS, is answered as Memory ('1'). MW's own Set direction is
	// write-fixed to '0' and carries no channel information (mwSetKindFixed).
	Kind byte
	// CTCSS is P8: '0' CTCSS OFF, '1' CTCSS ENC/DEC, '2' CTCSS ENC — the
	// legacy three-value domain, no DCS member (matrix §2.9).
	CTCSS byte
	// Shift is P10: '0' Simplex, '1' Plus Shift, '2' Minus Shift.
	Shift byte
}

// SlotState returns this fake's current stored state for slot and whether
// any state has ever been recorded for it. A test-inspection API: production
// code (the transport engine, the driver) talks to the fake only through
// Port(), never this method.
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// Model returns the constant row name this Radio answers ID under
// ("FTdx1200") — there is only one row (doc.go).
func (r *Radio) Model() string {
	return "FTdx1200"
}

// CATID returns the 4-digit ID answer this Radio currently gives ("0582" or
// "0583", matrix §2.2) — New's default, or WithFFT1NotFitted's setting.
func (r *Radio) CATID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.catID
}
