// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	civic7700 "github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
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
	// ProbeSlots: see icom.Params.ProbeSlots for why ten and not all 99.
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
}
