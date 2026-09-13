// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft950

// MemState is this fake's OWN in-memory representation of one slot's
// content — a memory channel (000-099) or a PMS band-edge (100-117), the
// only two classes this radio has. It is deliberately NOT core/codeplug's
// channel type and NOT any core/cat wire type (doc.go, THE HARD RULE) — every
// field is stored in the closest thing to raw wire form, matching the 24-byte
// field block MW's Set and MR's Answer both carry (parser.go), so building a
// reply is plain concatenation with no numeric conversion anywhere.
//
// NO Tag FIELD. This radio's NoTag: no command in the manual carries a
// channel name (doc.go, "THIS RADIO IS NoTag").
type MemState struct {
	// Freq is P2: the 8-digit ASCII frequency field in Hz, zero-padded (e.g.
	// "07000000" = 7.000000 MHz) — one digit narrower than every other
	// registered Yaesu dialect's 9 (matrix §1.2).
	Freq string
	// ClarSign is P3's sign byte, '+' or '-'. "Clarifier Direction +: Plus
	// Shift, --: Minus Shift" (layout:876-877, printed on MW's own block).
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude, "Clarifier Offset: 0000 - 9999
	// (Hz)" (layout:877).
	ClarMag string
	// RXClar is P4, "0: RX CLAR OFF 1: RX CLAR ON" (layout:877).
	RXClar bool
	// TXClar is P5, "0: TX CLAR OFF 1: TX CLAR ON" (layout:877).
	TXClar bool
	// Mode is P6, one ASCII byte from the twelve-member legend
	// (layout:877/898): 1 LSB, 2 USB, 3 CW, 4 FM, 5 AM, 6 FSK(RTTY-LSB),
	// 7 CW-R, 8 PKT-L, 9 FSK-R(RTTY-USB), A PKT-FM, B FM-N, C PKT-U. No hole,
	// no '0' placeholder — the legend is closed (matrix §1.3, doc.go).
	Mode byte
	// Kind is P7 — the ANSWER byte, "0: VFO 1: Memory" (layout:857, MR's own
	// Read/Answer legend). doc.go's register entry AN ANSWER'S KIND BYTE IS
	// ALWAYS '1': every populated slot this fake stores, memory or PMS, is
	// answered as Memory. MW's own Set direction prints "P7 0: (Fixed)"
	// (layout:899) — the Set carries no channel information at all, so there
	// is nothing to store from it; this field is what a read answers with,
	// not what a Set delivered.
	Kind byte
	// CTCSS is P8: '0' CTCSS OFF, '1' CTCSS ENC/DEC, '2' CTCSS ENC — THREE
	// values (layout:857/898), the ordinary legacy domain, no DCS member
	// (matrix §4).
	CTCSS byte
	// Tone is P9: the 2-digit ASCII tone-table index, "Tone Number (See Table
	// 1)" / "(See Table on page 5)" (layout:857/898). A LIVE VALUE where
	// every other registered Yaesu dialect prints and enforces a fixed "00"
	// (matrix §1.4) — stored at its printed WIDTH and not range-checked
	// against CN's separate 00-49 ceiling (doc.go's register entry TONE
	// INDEX (P9) IS STORED AT ITS PRINTED WIDTH).
	Tone string
	// Shift is P10: '0' Simplex, '1' Plus Shift, '2' Minus Shift
	// (layout:857/898).
	Shift byte
}

// SlotState returns this fake's current stored state for slot and whether any
// state has ever been recorded for it. A test-inspection API: production code
// (the transport engine, the driver) talks to the fake only through Port(),
// never this method.
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// CurrentChannel returns this fake's "selected channel" state, as last set by
// an MC-set, or the answer-only none form ("118", doc.go's register entry 2)
// if no MC-set has happened yet. A Set never changes it (doc.go's register
// entry A SET DOES NOT MOVE THE SELECTED CHANNEL).
func (r *Radio) CurrentChannel() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
