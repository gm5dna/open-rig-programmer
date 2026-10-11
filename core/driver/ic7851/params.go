// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// params is this radio's own values for the shared Open and Read engine in
// core/driver/internal/icom.
//
// WHY AN ALL-0xFF RECORD READS AS EMPTY (the engine's RecordIsAbsent): the
// evidence is two separate, unverified entries and one capture cannot
// establish both. Register entry ic7851-empty-reply-fa is the FA reading,
// whose lift clears M-CH50 and records the answer verbatim;
// ic7851-all-ff-record is the all-FF reading, whose lift asks whether that
// same answer was instead a full-length record of all FF. Both lifts name a
// MEMORY channel, so neither covers P1 or P2. Register entry
// ic7851-all-ff-record's reading of an unwritten channel is the driver's
// decision on its own model's evidence, as civ.Profile.MemoryAnswerRecord's
// own doc comment says.
var params = icom.Params{
	Name:         "ic7851",
	Profile:      civic7851.Profile,
	RecordLength: civic7851.RecordOnlyLength,
	// ProbeSlots: see icom.Params.ProbeSlots for the bound. Ten, not all 99.
	ProbeSlots: icom.ChannelRange(1, 10),
	// Domain is write.go's rung-4 helper: a write refuses a value it cannot
	// encode, and a read refuses a frequency the write would refuse. It
	// bounds the frequency by the capability set and has no TX frequency to
	// judge, so TXDuplicate stays unset.
	Domain: domainRefusal,
	// ReadCheck is the read side's alone: a write never meets a record with
	// a digit in a printed-fixed byte, because the read that precedes it
	// refuses one first.
	ReadCheck:    fixedDigitsDiffer,
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },

	// The rest is the shared write ladder's (icom.WriteChannel).
	//
	// WHY THESE REGIONS: record byte 0's two nibbles (the SELECT-group
	// marker) and byte 8's HIGH nibble (the four-valued data mode); byte 8's
	// low nibble is tone_mode and IS mapped. The order decides which error
	// fires. The template is the profile's own Fixed record, all zero.
	//
	// WRITE-SIDE EVIDENCE NOTES. FieldErase: the memory-clear forms are
	// printed in two shapes (doc.go §9) and no builder for either exists in
	// this tier. The write sends the 25 record bytes in full, because the
	// document prints one full-record form and one three-byte clear form and
	// never says whether a short record is accepted (matrix §3.10 grades the
	// "mandatory" half CANNOT ESTABLISH).
	Unmapped: []icom.Region{
		{Offset: civic7851.SelectNibbleOffset, Nibble: "high"},
		{Offset: civic7851.SelectNibbleOffset, Nibble: "low"},
		{Offset: civic7851.DataModeNibbleOffset, Nibble: "high"},
	},
	Template:    make([]byte, civic7851.RecordOnlyLength),
	NewUnmapped: func(e driver.UnmappedRegionError) error { return &UnmappedRegionError{e} },
	Text: icom.WriteText{
		Erase:          "this radio's memory-clear forms are printed in its document but no IC-7851 has ever been asked to use one, so FieldErase carries the zero FieldSupport and consent structurally never reaches it (spec D4 \"Erase\"; matrix §3.13)",
		Unwritable:     "either no IC-7851 has ever been written to by this project and no consent was recorded, or the field is one ruling E6 leaves UNMAPPED (scan_skip, data_mode) and therefore one this driver refuses rather than collapses 4->2",
		TagCharsetTail: "printed memory-name charset (PDF p.261, folio 18-12, the Letters and Symbols code tables)",
		NoDefaultTone:  "this slot has no prior record to preserve a tone from, and THIS RADIO'S DOCUMENT PRINTS NO DEFAULT TONE VALUE — all 283 pages were swept for the matrix and no \"Default:\" line appears against the tone frequency; the only tone material is PDF p.262 (folio 18-13)'s 1B 00 / 1B 01 digit strip. Tier ruling T1(5)'s REFUSE arm rather than its default arm",
		LayoutNote:     "single 25-byte layout",
		ModeWhere:      "the record's ⑨ is a mode enum, and PDF p.260's \"① Operating mode\" column prints these ten codes and no more",
		FilterWhere:    "the record's ⑩ is a filter enum, and PDF p.260's \"② Filter setting\" column prints these three values and no default",
		ToneModeWhere:  "the record's ⑪ low nibble is a tone-mode enum, and PDF p.263's ⑪ sub-diagram prints these three values",
	},
}
