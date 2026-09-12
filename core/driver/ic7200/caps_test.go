// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"reflect"
	"testing"

	civic7200 "github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

var allCapabilityFields = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
	spec.FieldTag, spec.FieldTagDisplay, spec.FieldScanSkip, spec.FieldErase,
	spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
	spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
	spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter,
	spec.FieldDataMode, spec.FieldTuningStepEnabled, spec.FieldTuningStep,
	spec.FieldProgramTuningStep, spec.FieldAttenuator, spec.FieldPreamp,
	spec.FieldAntenna, spec.FieldIPPlus,
}

var deliberatelyUnexpressedFields = map[spec.Field]string{}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allCapabilityFields", allCapabilityFields, deliberatelyUnexpressedFields)
}

func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		v := reflect.ValueOf(caps)
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			zero := v.Field(i).IsZero() || (v.Field(i).Kind() == reflect.Slice && v.Field(i).Len() == 0)
			_, listed := deliberatelyZero[name]
			if zero != listed {
				t.Errorf("%s zero=%v listed=%v", name, zero, listed)
			}
		}
	}
}

func TestCapabilitiesShape(t *testing.T) {
	c := CapabilitiesUnverified()
	if c.Model != "IC-7200" || c.CATID != "76" || c.Transmit != spec.HasTransmitter {
		t.Fatalf("capabilities = %+v", c)
	}
	if c.TagLen != 0 || !c.NoTag {
		t.Fatalf("NoTag capabilities = TagLen %d, NoTag %v, want 0, true", c.TagLen, c.NoTag)
	}
	if c.SimplexTx != spec.SimplexTxEqualsRx {
		t.Errorf("SimplexTx = %v, want SimplexTxEqualsRx", c.SimplexTx)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	mem, ok := c.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 199 || mem.Slots[0] != "001" || mem.Slots[198] != "199" {
		t.Fatalf("MEM = %+v", mem)
	}
	scan, ok := c.Bank(spec.BankScan)
	if !ok || len(scan.Slots) != 2 || scan.Slots[0] != "P1" || scan.Slots[1] != "P2" {
		t.Fatalf("SCAN = %+v", scan)
	}
	if _, ok := c.Bank(spec.BankCall); ok {
		t.Fatal("CALL bank unexpectedly present")
	}
}

// TestModesFiltersMatchTheCodec pins the capability's UI vocabularies
// against the codec's own enum tables, so the two can never disagree
// about what this radio can express.
func TestModesFiltersMatchTheCodec(t *testing.T) {
	checkVocabulary(t, "mode", CapabilitiesUnverified().Modes)
	checkVocabulary(t, "filter", CapabilitiesUnverified().Filters)
}

func checkVocabulary(t *testing.T, fieldName string, declared []string) {
	t.Helper()
	seen := map[string]bool{}
	var codec map[byte]string
	for _, sp := range civic7200.Profile().Layouts()[0].Fields {
		if string(sp.Field) == fieldName {
			codec = sp.Enum
		}
	}
	if codec == nil {
		t.Fatalf("the codec maps no %s span", fieldName)
	}
	for _, v := range declared {
		if seen[v] {
			t.Errorf("the capability declares %s %q twice", fieldName, v)
		}
		seen[v] = true
	}
	for code, name := range codec {
		if !seen[name] {
			t.Errorf("the codec decodes %#02x as %s %q, which the capability does not declare", code, fieldName, name)
		}
	}
	if len(declared) != len(codec) {
		t.Fatalf("the capability declares %d %s values and the codec maps %d", len(declared), fieldName, len(codec))
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("write trial guard unlocked")
	}
	for _, b := range CapabilitiesUnverified().Banks {
		for f, sup := range b.Fields {
			if sup.CanWrite() {
				t.Errorf("%s is writable on bank %s with the write-trial guard false", f, b.ID)
			}
		}
	}
}

func TestBaseline_Validate(t *testing.T) {
	for name, caps := range map[string]spec.Capabilities{
		"unverified":         CapabilitiesUnverified(),
		"simulated":          CapabilitiesSimulated(),
		"unverified+consent": spec.ConsentUnverifiedWrites(CapabilitiesUnverified()),
		"simulated+consent":  spec.ConsentUnverifiedWrites(CapabilitiesSimulated()),
		"New constructor":    New(RealHardware).Capabilities(),
		"New simulated arm":  New(Simulated).Capabilities(),
	} {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
