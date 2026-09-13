// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// deliberatelyUnexpressedFields is EMPTY: this driver's bank maps (caps.go's
// memFields/pmsFields) name every spec.Field explicitly, including the
// twenty-one that carry the zero FieldSupport — so there is no field whose
// absence from allFields needs a second reason here.
var deliberatelyUnexpressedFields = map[spec.Field]string{}

// TestFieldAuditCoversEverySpecField is the brief's TestDeliberatelyZeroAudit
// obligation: every spec.Field is either mapped (allFields, caps.go) or
// given a one-line reason (there are none to give, here).
func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// TestWriteTrialsComplete_PinnedFalse pins both halves of the write guard:
// the constant is false, and the RealHardware baseline is genuinely
// nothing-writable, on EITHER bank.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true: no FT-450D has ever been asked anything by this project")
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
// requires (matrix §0/§2.7).
func TestCapabilities_NoTagPairing(t *testing.T) {
	caps := CapabilitiesUnverified()
	if !caps.NoTag {
		t.Error("NoTag = false, want true (matrix §0/§2.7)")
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

// TestCapabilities_CTCSSToneMapped pins that FieldCTCSSTone is mapped on
// MEM, not zero.
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

// TestBanks_MemRangeAndPMSRange pins matrix §2.5: MEM is 001-500 (500
// slots), PMS is 501-504 (2 pairs, 4 slots), NoBlank true on PMS only.
func TestBanks_MemRangeAndPMSRange(t *testing.T) {
	caps := CapabilitiesUnverified()

	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	if len(mem.Slots) != 500 {
		t.Fatalf("len(MEM.Slots) = %d, want 500 (001-500)", len(mem.Slots))
	}
	if mem.Slots[0] != "001" || mem.Slots[len(mem.Slots)-1] != "500" {
		t.Errorf("MEM.Slots run %q..%q, want 001..500", mem.Slots[0], mem.Slots[len(mem.Slots)-1])
	}
	if mem.NoBlank {
		t.Error("MEM.NoBlank = true, want false")
	}

	pms, ok := caps.Bank(spec.BankPMS)
	if !ok {
		t.Fatal("no PMS bank")
	}
	if len(pms.Slots) != 4 {
		t.Fatalf("len(PMS.Slots) = %d, want 4 (2 pairs)", len(pms.Slots))
	}
	if pms.Slots[0] != "501" || pms.Slots[len(pms.Slots)-1] != "504" {
		t.Errorf("PMS.Slots = %v, want to run 501..504", pms.Slots)
	}
	if !pms.NoBlank {
		t.Error("PMS.NoBlank = false, want true (matrix §2.5: PMS pairs are never blank)")
	}
}

// TestCapabilities_PMSWriteUnsupported pins the new shape this package must
// carry that ft991a/ft2000 do not (matrix §3, spec.md §1): PMS's six
// shared fields are Write: Unsupported on BOTH profiles, while MEM's
// identical six follow the ordinary profile-dependent rw. Read stays
// aligned between the two banks.
func TestCapabilities_PMSWriteUnsupported(t *testing.T) {
	writableFields := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
		spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
	}

	for _, tc := range []struct {
		name string
		caps spec.Capabilities
		read spec.Support
	}{
		{"Unverified", CapabilitiesUnverified(), spec.Unverified},
		{"Simulated", CapabilitiesSimulated(), spec.Supported},
	} {
		mem, ok := tc.caps.Bank(spec.BankMemory)
		if !ok {
			t.Fatalf("%s: no MEM bank", tc.name)
		}
		pms, ok := tc.caps.Bank(spec.BankPMS)
		if !ok {
			t.Fatalf("%s: no PMS bank", tc.name)
		}
		for _, f := range writableFields {
			memFS := mem.Fields[f]
			pmsFS := pms.Fields[f]
			if pmsFS.Write != spec.Unsupported {
				t.Errorf("%s: PMS field %s Write = %v, want Unsupported", tc.name, f, pmsFS.Write)
			}
			if pmsFS.Read != tc.read {
				t.Errorf("%s: PMS field %s Read = %v, want %v (same as MEM's read column)", tc.name, f, pmsFS.Read, tc.read)
			}
			if memFS.Write == spec.Unsupported {
				t.Errorf("%s: MEM field %s Write = Unsupported, want the ordinary profile-dependent value", tc.name, f)
			}
		}
	}
}

// TestCapabilities_RequiredSlotsEmpty pins doc.go entry 5: no slot is
// claimed required.
func TestCapabilities_RequiredSlotsEmpty(t *testing.T) {
	if got := CapabilitiesUnverified().RequiredSlots; len(got) != 0 {
		t.Errorf("RequiredSlots = %v, want empty (matrix §2.12)", got)
	}
}
