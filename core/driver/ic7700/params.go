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
	Name: "ic7700",
	// Profile's Open identity probe: what identifies the radio is that an
	// ADDRESS-MATCHED 19 00 reply arrived at all. The reply VALUE is
	// undocumented on every model in this tier — PDF p.204 (folio 14-4)
	// prints the 19 00 row with no reply value and no cross-reference — so
	// it is recorded as a diagnostic and compared against nothing.
	Profile:      civic7700.Profile,
	RecordLength: civic7700.RecordOnlyLength,
	// ProbeSlots are the memory channels Open's occupied-slot search
	// reads, in order, before giving up and opening UNFINGERPRINTED.
	//
	// BOUNDED ON PURPOSE. The fingerprint's job is to confirm a record length,
	// which one occupied channel settles; walking all 99 to find a radio's
	// only populated memory would put ninety-nine exchanges into every open
	// for no further evidence. Ten is enough to clear a radio whose first few
	// memories happen to be empty and short enough that an entirely empty
	// radio opens promptly — on address evidence alone, which spec D3.2
	// explicitly allows.
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
