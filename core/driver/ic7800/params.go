// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800

import (
	civic7800 "github.com/gm5dna/open-rig-programmer/core/civ/ic7800"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// params is this radio's own values for the shared Open and Read engine in
// core/driver/internal/icom.
//
// WHY AN ALL-0xFF RECORD READS AS EMPTY (the engine's RecordIsAbsent): the
// evidence is two separate, unverified entries and one capture cannot
// establish both. D5 entry 2(a) / matrix lift R2a is the FA reading; D5
// entry 2(b) / matrix lift R2b is the all-0xFF reading, and it is this
// radio's own decision on its own model's evidence, as
// civ.Profile.MemoryAnswerRecord's doc comment says. R2a's scope EXCLUDES
// the scan edges (matrix erratum 2), so P1/P2 emptiness rides matrix lift
// R18 instead, and R18's capture names P1 only, so P2 is uncovered even by
// that.
var params = icom.Params{
	Name:         "ic7800",
	Profile:      civic7800.Profile,
	RecordLength: civic7800.RecordOnlyLength,
	// ProbeSlots: see icom.Params.ProbeSlots for the bound. Ten, not all 99.
	ProbeSlots: icom.ChannelRange(1, 10),
	// Domain is write.go's rung-4 helper: a write refuses a value it
	// cannot encode, and a read refuses a frequency the write would refuse.
	Domain:       domainRefusal,
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },

	// The rest is the shared write ladder's (icom.WriteChannel).
	//
	// WHY THESE REGIONS: record byte 0 ENTIRELY, printed 'e', whose HIGH
	// nibble is implicitly fixed 0 and whose LOW nibble is the four-valued
	// SELECT-group marker, and record byte 8's LOW nibble, the four-valued
	// data mode. UNLIKE THE IC-7610 FAMILY, byte 8's HIGH nibble is
	// tone_mode here (matrix §1b: "byte (11), high nibble" for tone_mode,
	// "low nibble" for data_mode) and IS mapped. The order decides which
	// error fires.
	//
	// WRITE-SIDE EVIDENCE NOTES. FieldErase: the memory-clear forms are
	// printed but have never been used on the IC-7800, and no builder for
	// either shape exists in this tier (doc.go §7). The write sends the 25
	// record bytes in full (register entry ic7800-full-record-mandatory,
	// matrix lift R15: the document prints one full-record form and one
	// three-index clear form and never says whether a short record is
	// accepted). That a 1A 00 SET is answered by FB or FA is ASSUMED
	// (register entry ic7800-1a00-set-ack, matrix lift R14: the CODES are
	// MANUAL-EVIDENCED, but the nearest printed statement is about command
	// 29, which this driver does not read across). The numeric domain
	// (Domain, doc.go §6) is defence in depth and not the gate.
	Unmapped: []icom.Region{
		{Offset: civic7800.SelectNibbleOffset, Nibble: "high"},
		{Offset: civic7800.SelectNibbleOffset, Nibble: "low"},
		{Offset: civic7800.DataModeNibbleOffset, Nibble: "low"},
	},
	Template:    civic7800.FixedTemplate(),
	NewUnmapped: func(e driver.UnmappedRegionError) error { return &UnmappedRegionError{e} },
	Text: icom.WriteText{
		Erase:          "this radio's memory-clear forms are printed in its document but no IC-7800 has ever been asked to use one, so FieldErase carries the zero FieldSupport and consent structurally never reaches it (spec D4 \"Erase\"; matrix §3.13)",
		Unwritable:     "either no IC-7800 has ever been written to by this project and no consent was recorded, or the field is one ruling E6 leaves UNMAPPED (scan_skip, data_mode) and therefore one this driver refuses rather than collapses 4->2",
		TagCharsetTail: "printed memory-name charset (PDF p.12's two character tables)",
		NoDefaultTone:  "this slot has no prior record to preserve a tone from, and THIS RADIO'S DOCUMENT PRINTS NO DEFAULT TONE VALUE — all 17 pages were swept for the matrix and no \"Default:\" line appears against the tone frequency; the only tone material is the 1B 00 / 1B 01 digit strip. Tier ruling T1(5)'s REFUSE arm rather than its default arm; register entry ic7800-default-tone-undocumented, whose lift is a capture of 1B 00 and 1B 01 on an IC-7800 taken to All Reset",
		LayoutNote:     "single 25-byte layout",
		ModeWhere:      "the record's ⑨ is a mode enum, and PDF p.11's \"①Receiving mode\" column prints these ten codes and no more",
		FilterWhere:    "the record's ⑩ is a filter enum, and PDF p.11's \"Filter setting\" column prints these three values and no default",
		ToneModeWhere:  "the record's ⑪ low nibble is a tone-mode enum, and PDF p.12's ⑪ sub-diagram prints these three values",
	},
}
