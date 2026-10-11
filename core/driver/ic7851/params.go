// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
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
	// Domain is write.go's rung-4 helper: a read refuses a frequency the
	// write would refuse.
	Domain:       domainRefusal,
	ReadCheck:    fixedDigitsDiffer,
	NewRecordLen: func(e icom.RecordLengthMismatchError) error { return &RecordLengthMismatchError{e} },
}
