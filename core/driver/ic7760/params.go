// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7760 "github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// params is this radio's own values for the shared Open and Read engine in
// core/driver/internal/icom.
//
// WHY AN ALL-0xFF RECORD READS AS EMPTY (the engine's RecordIsAbsent): the
// evidence is two separate, unverified entries and one capture cannot
// establish both. D5 entry 2(a) / register entry ic7760-empty-reply-fa is
// the FA reading; D5 entry 2(b) / register entry ic7760-empty-reply-ff is
// the all-0xFF reading. The -fa lift clears MEMORY CHANNEL 99, so its scope
// excludes the scan edges; P1/P2 emptiness rides
// ic7760-scan-edge-record-shape instead, and that lift reads 01 00 only, so
// P2 is uncovered even by that.
//
// There is no frequency floor on read: MinFreqHz is deliberately zero (see
// caps.go), so no decoded frequency can fall below it.
var params = icom.Params{
	Name:         "ic7760",
	Profile:      civic7760.Profile,
	RecordLength: civic7760.RecordOnlyLength,
	// ProbeSlots: see icom.Params.ProbeSlots for the bound. Ten MEM slots, not
	// all 99, then both SCAN selectors P1/P2 (channels 100 and 101):
	// omitting them would miss a radio whose early memories are empty and a
	// scan edge is occupied.
	// TestProbeScheduleIncludesP1AndP2AfterBoundedMEMSearch pins the exact
	// frames and stop point.
	ProbeSlots: append(icom.ChannelRange(1, 10), civ.ChannelAddress{Channel: 100}, civ.ChannelAddress{Channel: 101}),
	// Domain is write.go's rung-4 helper: a read refuses a frequency the
	// write would refuse.
	Domain:       domainRefusal,
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },

	// The rest is the shared write ladder's (icom.WriteChannel).
	//
	// WHY THESE REGIONS: record byte 0 ENTIRELY, printed ③, whose HIGH nibble
	// is the page's Fixed 0 and whose LOW nibble is the four-valued
	// SELECT-group marker, and record byte 8's HIGH nibble, printed ⑪'s left
	// nibble, the four-valued data mode. Byte 8's LOW nibble is tone_mode and
	// IS mapped. The order decides which error fires. The template is the
	// profile's zero-valued Fixed record (Stage 1 layout, E6).
	//
	// WRITE-SIDE EVIDENCE NOTES. FieldErase: the memory-clear forms are
	// printed in two shapes (matrix §3.13, assumption register entry
	// ic7760-clear-scope) and no builder for either exists in this tier.
	Unmapped: []icom.Region{
		{Offset: 0, Nibble: "high"},
		{Offset: 0, Nibble: "low"},
		{Offset: 8, Nibble: "high"},
	},
	Template:    make([]byte, civic7760.RecordOnlyLength),
	NewUnmapped: func(e driver.UnmappedRegionError) error { return &UnmappedRegionError{e} },
	Text: icom.WriteText{
		Erase:          "this radio's memory-clear forms are printed in its document but no IC-7760 has ever been asked to use one, so FieldErase carries the zero FieldSupport and consent structurally never reaches it (spec D4 \"Erase\"; matrix §3.13)",
		Unwritable:     "either no IC-7760 has ever been written to by this project and no consent was recorded, or the field is one ruling E6 leaves UNMAPPED (scan_skip, data_mode) and therefore one this driver refuses rather than collapses 4->2",
		TagCharsetTail: "printed memory-name charset (PDF p.20, folio 19, the two \"Codes for character entries\" tables)",
		NoDefaultTone:  "this slot has no prior record to preserve a tone from, and THIS RADIO'S DOCUMENT PRINTS NO DEFAULT TONE VALUE — all 28 pages of the revision 2 guide were swept for the matrix and no \"Default:\" line appears against the tone frequency; the only tone material is the 1B 00 / 1B 01 digit strip on PDF p.24 (folio 23). Tier ruling T1(5)'s REFUSE arm rather than its default arm; register entry ic7760-tone-domain, whose lift reads the lowest and highest tone the front panel offers on an IC-7760",
		LayoutNote:     "single 25-byte layout",
		ModeWhere:      "the record's ⑨ is a mode enum, and PDF p.18 (folio 17)'s \"Operating mode\" table, column ①, prints these ten codes and no more",
		FilterWhere:    "the record's ⑩ is a filter enum, and PDF p.18 (folio 17)'s \"Operating mode\" table, column ② \"Filter setting\", prints these three values and no default",
		ToneModeWhere:  "the record's ⑪ low nibble is a tone-mode enum, and PDF p.20 (folio 19)'s ⑪ sub-diagram prints these three values",
	},
}
