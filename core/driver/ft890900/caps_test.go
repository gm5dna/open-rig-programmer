// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestCapabilities_ValidateEveryProfile(t *testing.T) {
	for _, tc := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"FT890/Unverified", ft890Info.CapabilitiesUnverified()},
		{"FT890/Simulated", ft890Info.CapabilitiesSimulated()},
		{"FT900/Unverified", ft900Info.CapabilitiesUnverified()},
		{"FT900/Simulated", ft900Info.CapabilitiesSimulated()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.caps.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestCapabilities_ModelAndCATIDAndSlotCounts(t *testing.T) {
	fc := ft890Info.CapabilitiesUnverified()
	if fc.Model != "FT-890" || fc.CATID != "0890" {
		t.Errorf("FT-890: Model=%q CATID=%q", fc.Model, fc.CATID)
	}
	if got := len(fc.Banks[0].Slots); got != ft890TrueSlotCount {
		t.Errorf("FT-890 slot count = %d, want %d", got, ft890TrueSlotCount)
	}
	if fc.Banks[0].Slots[len(fc.Banks[0].Slots)-1] != "032" {
		t.Errorf("FT-890 last slot = %q, want \"032\"", fc.Banks[0].Slots[len(fc.Banks[0].Slots)-1])
	}

	nc := ft900Info.CapabilitiesUnverified()
	if nc.Model != "FT-900" || nc.CATID != "0900" {
		t.Errorf("FT-900: Model=%q CATID=%q", nc.Model, nc.CATID)
	}
	if got := len(nc.Banks[0].Slots); got != ft900TrueSlotCount {
		t.Errorf("FT-900 slot count = %d, want %d", got, ft900TrueSlotCount)
	}
	if nc.Banks[0].Slots[len(nc.Banks[0].Slots)-1] != "100" {
		t.Errorf("FT-900 last slot = %q, want \"100\"", nc.Banks[0].Slots[len(nc.Banks[0].Slots)-1])
	}
}
