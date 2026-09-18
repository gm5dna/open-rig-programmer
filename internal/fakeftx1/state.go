// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

// MemState is this fake's OWN in-memory representation of one slot's
// MR/MW memory-block content — a memory channel, a PMS band-edge, a 5 MHz
// channel or the emergency channel, the four classes FTX-1's slot space
// has (spec.md §4). It is deliberately NOT core/codeplug's channel type
// and NOT any core/cat wire type (doc.go, THE HARD RULE) — every field is
// stored in the closest thing to raw wire form, so building a reply is
// plain concatenation with no numeric conversion anywhere.
//
// NO Tag FIELD HERE. A slot's tag is MT's own, entirely separate, state
// (Radio.tags) — doc.go's register entry MW AND MT MUTATE INDEPENDENT
// FIELDS.
type MemState struct {
	// Freq is P2: the 9-digit ASCII frequency field in Hz, zero-padded
	// (spec.md §3.1 offsets 5-13).
	Freq string
	// ClarSign is P3's sign byte, '+' or '-' (spec.md §3.1 offset 14).
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude, "0000-9990 (Hz)" (spec.md
	// §3.1 offsets 15-18, §7).
	ClarMag string
	// RXClar is P4, '0' OFF / '1' ON (spec.md §3.1 offset 19).
	RXClar bool
	// TXClar is P5, '0' OFF / '1' ON (spec.md §3.1 offset 20).
	TXClar bool
	// Mode is P6, one ASCII byte from the 18-nibble legend (spec.md §5):
	// the FT-710's own 16 members ('0'-'9','A'-'F') plus 'H' (C4FM-DN)
	// and 'I' (C4FM-VW). 'G'/'J' are ASSUMED reserved and refused
	// (spec.md §5, §13 item 3) — not modelled here (parser.go's
	// validModeByte).
	Mode byte
	// Kind is P7, one of the six legend bytes '0'-'5' (spec.md §3.1
	// offset 22, table row P7). Stored verbatim — doc.go's register
	// entry KIND (P7) AND TONE (P8) ARE STORED VERBATIM.
	Kind byte
	// Tone is P8, one of the six legend bytes '0'-'5' (spec.md §6):
	// OFF/CTCSS ENC-DEC/CTCSS ENC/DCS/PR FREQ/REV TONE — wider than every
	// other registered dialect's tone domain. Stored verbatim.
	Tone byte
	// Shift is P10: '0' Simplex, '1' Plus, '2' Minus (spec.md §3.1
	// offset 26).
	Shift byte
}

// SlotState returns this fake's current stored MR/MW state for addr and
// whether any has ever been recorded for it. A test-inspection API:
// production code (a future core/driver/ftx1) talks to the fake only
// through Port(), never this method.
func (r *Radio) SlotState(addr string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[addr]
	return s, ok
}

// TagState returns this fake's current stored MT tag for addr (already
// the fixed 12-byte wire form) and whether one has ever been recorded.
func (r *Radio) TagState(addr string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tags[addr]
	return t, ok
}

// CATID returns the 4-digit ID answer this Radio gives, always "0840"
// (doc.go — one row, no option to change it, body never surfaced).
func (r *Radio) CATID() string { return catID }
