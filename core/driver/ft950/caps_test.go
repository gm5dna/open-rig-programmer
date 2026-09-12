// SPDX-License-Identifier: GPL-3.0-or-later

package ft950

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// deliberatelyUnexpressedFields is EMPTY: this driver's bank map (caps.go's
// bankFields) names every spec.Field explicitly, including the twenty-one
// that carry the zero FieldSupport — so there is no field whose absence
// from allFields needs a second reason here.
var deliberatelyUnexpressedFields = map[spec.Field]string{}

// TestFieldAuditCoversEverySpecField is the brief's TestDeliberatelyZeroAudit
// obligation: every spec.Field is either mapped (allFields, caps.go) or
// given a one-line reason (there are none to give, here).
func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// TestWriteTrialsComplete_PinnedFalse pins both halves of the write guard:
// the constant is false, and the RealHardware baseline is genuinely
// nothing-writable.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true: no FT-950 has ever been asked anything by this project")
	}
	caps := CapabilitiesUnverified()
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if fs.CanWrite() {
				t.Errorf("bank %s field %s is CanWrite() on the RealHardware baseline while writeTrialsComplete is false", b.ID, f)
			}
		}
	}
}

// TestCapabilities_NoTagPairing pins the NoTag/TagLen pairing validate.go
// requires (matrix §0/§3).
func TestCapabilities_NoTagPairing(t *testing.T) {
	caps := CapabilitiesUnverified()
	if !caps.NoTag {
		t.Error("NoTag = false, want true (matrix §0/§3)")
	}
	if caps.TagLen != 0 {
		t.Errorf("TagLen = %d, want 0 under NoTag", caps.TagLen)
	}
	if err := caps.Validate(); err != nil {
		t.Errorf("CapabilitiesUnverified().Validate() = %v, want nil", err)
	}
	if err := CapabilitiesSimulated().Validate(); err != nil {
		t.Errorf("CapabilitiesSimulated().Validate() = %v, want nil", err)
	}
}

// TestCapabilities_CTCSSToneMapped pins that FieldCTCSSTone is mapped, not
// zero (doc.go entry 1).
func TestCapabilities_CTCSSToneMapped(t *testing.T) {
	caps := CapabilitiesSimulated()
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	fs := mem.Fields[spec.FieldCTCSSTone]
	if fs.Read != spec.Supported || fs.Write != spec.Supported {
		t.Errorf("FieldCTCSSTone on Simulated MEM = %+v, want {Supported Supported}", fs)
	}
}

// TestToneIndexRoundTrip pins toneForIndex/indexForTone as exact inverses
// over the whole 50-entry chart.
func TestToneIndexRoundTrip(t *testing.T) {
	for i := 0; i < 50; i++ {
		tone, ok := toneForIndex(uint8(i))
		if !ok {
			t.Fatalf("toneForIndex(%d) refused, want ok", i)
		}
		idx, ok := indexForTone(tone)
		if !ok || idx != uint8(i) {
			t.Errorf("indexForTone(toneForIndex(%d)) = %d, %v, want %d, true", i, idx, ok, i)
		}
	}
	if _, ok := toneForIndex(50); ok {
		t.Error("toneForIndex(50) succeeded, want refused (chart is 0-49)")
	}
}

// TestBanks_StartAtZero pins this radio's own delta: the MEM bank's first
// slot is "000", not "001" (matrix §1.5, doc.go entry 4), and holds 100
// slots (000-099).
func TestBanks_StartAtZero(t *testing.T) {
	caps := CapabilitiesUnverified()
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	if len(mem.Slots) != 100 {
		t.Fatalf("len(MEM.Slots) = %d, want 100 (000-099)", len(mem.Slots))
	}
	if mem.Slots[0] != "000" {
		t.Errorf("MEM.Slots[0] = %q, want %q", mem.Slots[0], "000")
	}
	if mem.Slots[len(mem.Slots)-1] != "099" {
		t.Errorf("MEM.Slots[last] = %q, want %q", mem.Slots[len(mem.Slots)-1], "099")
	}

	pms, ok := caps.Bank(spec.BankPMS)
	if !ok {
		t.Fatal("no PMS bank")
	}
	if len(pms.Slots) != 18 {
		t.Fatalf("len(PMS.Slots) = %d, want 18 (9 pairs)", len(pms.Slots))
	}
	if pms.Slots[0] != "100" || pms.Slots[len(pms.Slots)-1] != "117" {
		t.Errorf("PMS.Slots = %v, want to run 100..117", pms.Slots)
	}
}

// TestCapabilities_RequiredSlotsEmpty pins doc.go entry 6: no slot is
// claimed required.
func TestCapabilities_RequiredSlotsEmpty(t *testing.T) {
	if got := CapabilitiesUnverified().RequiredSlots; len(got) != 0 {
		t.Errorf("RequiredSlots = %v, want empty (matrix §4 leaves this unresolved)", got)
	}
}
