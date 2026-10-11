// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800

import (
	civic7800 "github.com/gm5dna/open-rig-programmer/core/civ/ic7800"
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
	// ProbeSlots: see icom.Params.ProbeSlots for why ten and not all 99.
	ProbeSlots: icom.ChannelRange(1, 10),
	// Domain is write.go's rung-4 helper: a read refuses a frequency the
	// write would refuse.
	Domain:       domainRefusal,
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },
}
