// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func TestStopBits(t *testing.T) {
	for _, d := range []driver.Driver{New7851(), New7850()} {
		r, ok := d.(driver.SerialFramingReporter)
		if !ok || r.StopBits() != 1 {
			t.Fatalf("%s does not report 8-N-1", d.Model())
		}
	}
}

func TestProfileProbeShape(t *testing.T) {
	p := civicProfile()
	if p.RadioAddress() != 0x8e || p.BuildRecordLength() != 25 || len(p.RecordLengths()) != 1 || !p.AcceptsRecordLength(25) {
		t.Fatalf("probe shape: address=%x length=%d records=%v", p.RadioAddress(), p.BuildRecordLength(), p.RecordLengths())
	}
}

func civicProfile() civ.Profile { return civic7851.Profile() }
