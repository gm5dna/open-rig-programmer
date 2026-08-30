// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestNewProfilesFailSafeAndDoNotExposeSerialFraming(t *testing.T) {
	for name, profile := range map[string]Profile{
		"real":      RealHardware,
		"simulated": Simulated,
		"unknown":   Profile(99),
	} {
		d := New(profile)
		if d.Model() != "IC-7100" || d.Capabilities().Model != d.Model() {
			t.Errorf("%s model mismatch: %q/%q", name, d.Model(), d.Capabilities().Model)
		}
		if _, ok := d.(driver.SerialFramingReporter); ok {
			t.Errorf("%s implements SerialFramingReporter; PDF p.174 is DV low-speed data, not CI-V evidence (ic7100-serial-framing)", name)
		}
		if name != "simulated" && d.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
			t.Errorf("%s permits frequency writes without a consented session", name)
		}
	}
}

func TestControlLinePolicyMakesNoDriverAssertion(t *testing.T) {
	// Matrix §3.2 and register entry ic7100-control-lines establish no
	// CI-V RTS/DTR level. The driver therefore has no control-line option or
	// reporter and leaves the existing serial-open defaults (both low) intact.
	d := New(RealHardware)
	if _, ok := d.(interface{ RTS() bool }); ok {
		t.Fatal("driver unexpectedly asserts RTS")
	}
	if _, ok := d.(interface{ DTR() bool }); ok {
		t.Fatal("driver unexpectedly asserts DTR")
	}
}
