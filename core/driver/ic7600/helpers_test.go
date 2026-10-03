// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// probeSlotCount is the size of Open's occupied-slot search, written out
// here as the oracle for params.ProbeSlots.
const probeSlotCount = 10

func addressToSlot(a civ.ChannelAddress) (string, error) {
	return icom.AddressToSlot(params.Name, a, "%03d")
}

func recordIsAbsent(raw []byte) bool { return icom.RecordIsAbsent(raw) }
