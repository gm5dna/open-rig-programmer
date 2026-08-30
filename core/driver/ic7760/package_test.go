// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7760 "github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func TestPackageBoundaryAndAdmittedGeometry(t *testing.T) {
	d := New(Simulated)
	if d.Model() != "IC-7760" {
		t.Fatalf("Model() = %q, want IC-7760", d.Model())
	}
	if got := civic7760.Profile().RadioAddress(); got != 0xB2 {
		t.Fatalf("radio address = %#x, want 0xB2", got)
	}
	if got := civic7760.Profile().ControllerAddress(); got != 0xE0 {
		t.Fatalf("controller address = %#x, want 0xE0", got)
	}
	for _, addr := range []civ.ChannelAddress{{Channel: 1}, {Channel: 99}, {Channel: 100}, {Channel: 101}} {
		if _, err := civic7760.Profile().BuildMemoryRead(addr); err != nil {
			t.Fatalf("BuildMemoryRead(%s): %v", addr, err)
		}
	}
	if _, ok := d.(driver.Driver); !ok {
		t.Fatal("IC-7760 constructor does not implement driver.Driver")
	}
}
