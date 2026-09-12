// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7200 "github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func TestStopBits(t *testing.T) {
	for _, d := range []driver.Driver{New(RealHardware), New(Simulated)} {
		r, ok := d.(driver.SerialFramingReporter)
		if !ok || r.StopBits() != 1 {
			t.Fatalf("%s does not report 8-N-1", d.Model())
		}
	}
}

func TestProfileProbeShape(t *testing.T) {
	p := civic7200.Profile()
	if p.RadioAddress() != 0x76 || p.BuildRecordLength() != civic7200.RecordOnlyLength || len(p.RecordLengths()) != 1 || !p.AcceptsRecordLength(civic7200.RecordOnlyLength) {
		t.Fatalf("probe shape: address=%x length=%d records=%v", p.RadioAddress(), p.BuildRecordLength(), p.RecordLengths())
	}
	if p.ControllerAddress() != civ.ControllerAddressDefault {
		t.Fatalf("controller address = %#02x", p.ControllerAddress())
	}
}
