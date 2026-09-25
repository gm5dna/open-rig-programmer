// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// deliberatelyUnexpressedFields carries only the three TS-2000-only
// Satellite Memory bank flags (v1.10.0): every OTHER spec.Field is named
// explicitly in this driver's bank map (caps.go's bankFields), including
// the twenty-one that carry the zero FieldSupport — so none of THOSE
// needs a second reason here (ft891's own identical precedent).
var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldSatBandSwap: "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTrace:    "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTraceRev: "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
}

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
		t.Fatal("writeTrialsComplete = true: no FTdx9000 write trial has ever been run by this project")
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
// requires (matrix §0/§1.6).
func TestCapabilities_NoTagPairing(t *testing.T) {
	caps := CapabilitiesUnverified()
	if !caps.NoTag {
		t.Error("NoTag = false, want true (matrix §1.6)")
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

// TestCapabilities_CTCSSToneMapped pins the CHOICE doc.go records: unlike
// every sibling driver, this radio's FieldCTCSSTone is mapped, not zero.
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

// TestToneIndexRoundTrip: moved to core/driver/internal/yaesu (write_test.go)
// alongside the ToneForIndex/IndexForTone helpers it pins, now shared with
// ft450d and ft950 rather than duplicated per driver.
