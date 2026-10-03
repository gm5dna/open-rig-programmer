// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// params is this radio's own values for the shared Open and Read engine in
// core/driver/internal/icom.
var params = icom.Params{
	Name:         "ic7610",
	Profile:      civic7610.Profile,
	RecordLength: civic7610.RecordOnlyLength,
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
