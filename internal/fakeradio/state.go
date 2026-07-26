// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

// MemState is the fake's OWN in-memory representation of one memory slot's
// content (a channel, a PMS pair member, a 60m channel, or EMG). It is
// deliberately NOT core/codeplug.Channel and NOT any core/cat wire type —
// fakeradio must not import either (see doc.go) — so every field here is
// stored in the closest thing to raw wire form: single ASCII digit/sign
// bytes and fixed-width digit strings, matching the MR/MW frame layout in
// parser.go byte-for-byte. This keeps building a reply a matter of plain
// concatenation, with no numeric<->text conversion — and no possibility of
// the fake "fixing up" a value a real radio would just echo back as-is.
type MemState struct {
	// Freq is P2: the 9-digit ASCII frequency field, Hz, zero-padded
	// (e.g. "007000000" = 7.000000 MHz). Reference: MR answer pos 6-14.
	Freq string
	// ClarSign is P3's sign byte: '+' or '-'. Reference: MR answer pos 15.
	ClarSign byte
	// ClarMag is P3's 4-digit ASCII magnitude, 0000-9990 Hz in 10 Hz
	// steps. Reference: MR answer pos 16-19.
	ClarMag string
	// RXClar is P4: RX clarifier on/off. Reference: MR answer pos 20.
	RXClar bool
	// TXClar is P5: TX clarifier on/off. Reference: MR answer pos 21.
	TXClar bool
	// Mode is P6, the mode nibble as a single upper-case ASCII byte
	// ('1'-'9', 'A'-'F', or '0' for the documented "-"/unset
	// placeholder). Reference: MR answer pos 22, mode nibble table.
	Mode byte
	// Kind is P7, the channel kind digit ('0'-'5': VFO, Memory, Memory
	// Tune, QMB, "-" placeholder, PMS). Reference: MR answer pos 23.
	Kind byte
	// CTCSS is P8 ('0' off, '1' ENC/DEC, '2' ENC). Reference: MR answer
	// pos 24.
	CTCSS byte
	// Shift is P10 ('0' simplex, '1' plus, '2' minus). Reference: MR
	// answer pos 27.
	Shift byte

	// Tag is the MT tag text (P2 of MT), 0-12 ASCII bytes, stored
	// exactly as last written by MT-set — never trimmed or padded. This
	// is a DELIBERATE simplification (see doc.go register item 8): the
	// real radio IS now HW-CONFIRMED to pad a CAT-MT-set tag to 12 bytes
	// internally and echo it padded on read, but this fake's own default
	// behaviour stays exactly as it is — core/cat.Dialect.ParseMTAnswer's
	// trim fix (Fix: tag normalisation) is what makes the model-level tag
	// correct regardless of which side pads the wire bytes.
	Tag string
	// TagDisplay is the MT display flag (P1 of MT: '0' off, '1' on).
	TagDisplay bool

	// Populated marks whether MW (or the factory image) has stored full
	// channel data for this slot. A slot that exists in the map only
	// because of an MT tag write, with Populated == false, still answers
	// "?;" to MR and MC-set (ASSUMED, see doc.go register) — tag state
	// is modelled as independent of channel-data state.
	Populated bool
}

// SlotState returns the fake's current stored state for slot and whether
// any state (from the factory image, MW, or MT) has ever been recorded
// for it. It is a test-inspection API: production code (the future
// transport/driver) talks to the fake only via Port(), never this method.
func (r *Radio) SlotState(slot string) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[slot]
	return s, ok
}

// EXState returns the current stored raw P4 for a 6-digit EX (MENU) wire
// address — the EX equivalent of SlotState. Test-inspection API:
// production code talks to the fake only via Port(), never this method.
func (r *Radio) EXState(addr string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.exSettings[addr]
	return v, ok
}

// CurrentChannel returns the fake's current "selected channel" state, as
// last set by an MC-set (recall), or "000" if no MC-set has happened yet
// (VFO mode — ASSUMED semantics for "000", see doc.go register).
func (r *Radio) CurrentChannel() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
