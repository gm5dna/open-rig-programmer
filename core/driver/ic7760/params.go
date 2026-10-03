// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7760 "github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
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
	// ProbeSlots is the bounded occupied-slot fingerprint search: ten early
	// MEM slots followed by both SCAN selectors P1/P2 (channels 100 and
	// 101). Walking all 99 memories would add no evidence once one occupied
	// record has fixed the length, but omitting P1/P2 would miss a radio
	// whose early memories are empty and a scan edge is occupied.
	// TestProbeScheduleIncludesP1AndP2AfterBoundedMEMSearch pins the exact
	// frames and stop point.
	ProbeSlots: append(icom.ChannelRange(1, 10), civ.ChannelAddress{Channel: 100}, civ.ChannelAddress{Channel: 101}),
	Domain: func(d codeplug.ChannelData, _ spec.Capabilities) error {
		if d.FreqHz > MaxEncodableFreqHz {
			return &OutOfDomainError{driver.OutOfDomainError{
				Field: spec.FieldFrequency, Value: d.FreqHz, Max: MaxEncodableFreqHz,
			}}
		}
		return nil
	},
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },
}
