// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestConstructors(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func() interface {
			Model() string
			Capabilities() spec.Capabilities
		}
	}{
		{"7851", func() interface {
			Model() string
			Capabilities() spec.Capabilities
		} {
			return New7851()
		}},
		{"7850", func() interface {
			Model() string
			Capabilities() spec.Capabilities
		} {
			return New7850()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.make()
			c := d.Capabilities()
			want := "IC-" + tc.name
			if d.Model() != want {
				t.Fatalf("Model = %q", d.Model())
			}
			if c.CATID != "8e" || c.Transmit != spec.HasTransmitter || c.TagLen != 10 {
				t.Fatalf("capabilities = %+v", c)
			}
			if err := c.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestCapabilitiesShape(t *testing.T) {
	c := New7851().Capabilities()
	mem, ok := c.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
		t.Fatalf("MEM = %+v", mem)
	}
	scan, ok := c.Bank(spec.BankScan)
	if !ok || len(scan.Slots) != 2 || scan.Slots[0] != "P1" || scan.Slots[1] != "P2" {
		t.Fatalf("SCAN = %+v", scan)
	}
	if _, ok := c.Bank(spec.BankCall); ok {
		t.Fatal("CALL bank unexpectedly present")
	}
	for _, b := range c.Banks {
		if b.Fields[spec.FieldScanSkip].CanWrite() || b.Fields[spec.FieldDataMode].CanWrite() {
			t.Fatal("lossy E6 field writable")
		}
	}
}

func TestWriteTrialsCompletePinnedFalse(t *testing.T) {
	if writeTrialsComplete7851 || writeTrialsComplete7850 {
		t.Fatal("write trial guard unlocked")
	}
}

func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, caps := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
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
