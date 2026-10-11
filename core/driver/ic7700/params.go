// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	civic7700 "github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// params is this radio's own values for the shared Open and Read engine in
// core/driver/internal/icom.
//
// WHY AN ALL-0xFF RECORD READS AS EMPTY (the engine's RecordIsAbsent): the
// evidence is two separate, unverified entries and one capture cannot
// establish both. Register entry ic7700-empty-channel-reply (matrix
// §3.8(a)) is the FA reading; ic7700-all-ff-empty (matrix §3.8(b)) is the
// all-FF reading, whose lift asks whether that same answer was instead a
// full-length record of all FF. Both lifts name a MEMORY channel, so
// neither covers P1 or P2. Register entry ic7700-all-ff-empty's reading of
// an unwritten channel is the driver's decision on its own model's
// evidence, as civ.Profile.MemoryAnswerRecord's own doc comment says.
var params = icom.Params{
	Name:         "ic7700",
	Profile:      civic7700.Profile,
	RecordLength: civic7700.RecordOnlyLength,
	// ProbeSlots: see icom.Params.ProbeSlots for the bound. Ten, not all 99.
	ProbeSlots: icom.ChannelRange(1, 10),
	// Domain is write.go's rung-4 helper: a read refuses a frequency the
	// write would refuse.
	Domain: domainRefusal,
	// TXDuplicate: idx15-19 is the TX-duplicate block's OWN frequency span,
	// ALWAYS present on the wire (matrix §1 row 4's "still necessary" even
	// with Split OFF), so a read ALWAYS decodes it Known, never
	// Unavailable. SimplexTxEqualsRx (caps.go) states what a Split-OFF write
	// PUTS there; a read simply reports whatever the radio held.
	TXDuplicate:  true,
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },

	// The rest is the shared write ladder's (icom.WriteChannel).
	//
	// THE UNMAPPED SET IS THE MATRIX'S OWN §3.15 TABLE: idx0 ENTIRELY —
	// split flag (high nibble) and the four-valued SELECT-group marker (low
	// nibble) — idx8's HIGH nibble (the four-valued data mode; idx8's LOW
	// nibble is tone_mode and IS mapped), and idx20-28 (9 bytes), the
	// TX-duplicate block's own mode/filter/tone bytes, which have no neutral
	// field at all ("second TX-side copy, no neutral field", coordinator
	// ruling 12/09/2026). The order decides which error fires.
	Unmapped: append([]icom.Region{
		{Offset: civic7700.SplitSelectByteOffset, Nibble: "high"},
		{Offset: civic7700.SplitSelectByteOffset, Nibble: "low"},
		{Offset: civic7700.DataModeNibbleOffset, Nibble: "high"},
	}, txDupRegions()...),
	// WRITE-SIDE EVIDENCE NOTE. The acknowledgement CODES are
	// MANUAL-EVIDENCED (matrix §3.10) — PDF p.202 (folio 14-2), "D Data
	// format", prints the set frame FE FE 74 E0 Cn Sc <data> FD and the
	// acknowledgement FE FE E0 74 FB FD (OK) / FE FE E0 74 FA FD (NG) — but
	// that a 1A 00 SET specifically is answered by one of them is ASSUMED.
	// The record is always sent in full: the document prints one full-record
	// form and one three-byte clear form and never says whether a short
	// record is accepted (matrix §3.10 grades the "mandatory" half CANNOT
	// ESTABLISH).
	//
	// The IC-7700 profile's Fixed template is the all-zero record (Stage-1
	// geometry test). Kept local: the profile's public API intentionally
	// exposes layout, not a writable template accessor.
	Template:    make([]byte, civic7700.RecordOnlyLength),
	NewUnmapped: func(e driver.UnmappedRegionError) error { return &UnmappedRegionError{e} },
	Text: icom.WriteText{
		Erase:          "this radio's memory-clear forms are printed in its document but no IC-7700 has ever been asked to use one, so FieldErase carries the zero FieldSupport and consent structurally never reaches it (spec D4 \"Erase\"; matrix §3.13)",
		Unwritable:     "either no IC-7700 has ever been written to by this project and no consent was recorded, or the field is one ruling E6 leaves UNMAPPED (scan_skip, data_mode) and therefore one this driver refuses rather than collapses 4->2",
		TagCharsetTail: "derived memory-name charset (matrix §1 row 30, PDF p.211 folio 14-11's character-availability table)",
		NoDefaultTone:  "this slot has no prior record to preserve a tone from, and THIS RADIO'S DOCUMENT PRINTS NO DEFAULT TONE VALUE — the matrix's own sweep found no \"Default:\" line against the tone frequency; the only tone material is PDF p.211 (folio 14-11)'s 1B 00 / 1B 01 digit strip. Tier ruling T1(5)'s REFUSE arm rather than its default arm",
		LayoutNote:     "single 39-byte layout",
		ModeWhere:      "the record's ⑨ is a mode enum, and PDF p.210's operating-mode column prints these ten codes and no more",
		FilterWhere:    "the record's ⑩ is a filter enum, and PDF p.210's filter column prints these three values and no default",
		ToneModeWhere:  "the record's ⑪ low nibble is a tone-mode enum, and PDF p.213's !1 sub-diagram prints these three values",
	},
}

// txDupRegions is idx20-28, each byte compared whole.
func txDupRegions() []icom.Region {
	var rs []icom.Region
	for o := civic7700.TXDupUnmappedOffset; o < civic7700.TXDupUnmappedOffset+civic7700.TXDupUnmappedLength; o++ {
		rs = append(rs, icom.Region{Offset: o, Nibble: "whole"})
	}
	return rs
}
