// SPDX-License-Identifier: GPL-3.0-or-later

package driver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// stubDriver is a minimal Driver implementation for exercising Registry:
// it carries a canned Capabilities and never opens anything.
type stubDriver struct {
	caps spec.Capabilities
}

func (d *stubDriver) Model() string                   { return d.caps.Model }
func (d *stubDriver) Capabilities() spec.Capabilities { return d.caps }
func (d *stubDriver) Open(context.Context, transport.Port, driver.Identity) (driver.Session, error) {
	return nil, errors.New("stubDriver: Open not implemented")
}

// validStubCaps returns a minimal Capabilities that passes
// spec.Capabilities.Validate, for the given model name.
func validStubCaps(model string) spec.Capabilities {
	return spec.Capabilities{
		Model:        model,
		CATID:        "9999",
		Bauds:        []int{38400},
		DefaultBaud:  38400,
		TagLen:       12,
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := driver.NewRegistry()
	d := &stubDriver{caps: validStubCaps("Stub-1")}

	if err := r.Register(d); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}

	got, ok := r.Get("Stub-1")
	if !ok {
		t.Fatal("Get(\"Stub-1\") = _, false; want the registered driver")
	}
	if got != driver.Driver(d) {
		t.Errorf("Get(\"Stub-1\") returned a different Driver value than was registered")
	}

	if _, ok := r.Get("Nonexistent"); ok {
		t.Error("Get(\"Nonexistent\") = _, true; want false")
	}
}

func TestRegistry_Register_Rejections(t *testing.T) {
	invalidCaps := validStubCaps("Bad")
	invalidCaps.DefaultBaud = 1 // not in Bauds -> Validate fails

	tests := []struct {
		name    string
		prepare func(r *driver.Registry) error // returns the FINAL Register call's error
	}{
		{
			name: "nil driver",
			prepare: func(r *driver.Registry) error {
				return r.Register(nil)
			},
		},
		{
			name: "duplicate model",
			prepare: func(r *driver.Registry) error {
				if err := r.Register(&stubDriver{caps: validStubCaps("Dup")}); err != nil {
					t.Fatalf("first Register: unexpected error: %v", err)
				}
				return r.Register(&stubDriver{caps: validStubCaps("Dup")})
			},
		},
		{
			name: "invalid capabilities",
			prepare: func(r *driver.Registry) error {
				return r.Register(&stubDriver{caps: invalidCaps})
			},
		},
		{
			name: "Model()/Capabilities().Model mismatch",
			prepare: func(r *driver.Registry) error {
				return r.Register(&mismatchedDriver{stubDriver{caps: validStubCaps("CapsName")}})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := driver.NewRegistry()
			if err := tt.prepare(r); err == nil {
				t.Error("Register: got nil error, want a rejection")
			}
		})
	}
}

// mismatchedDriver reports a Model() that disagrees with its
// Capabilities().Model, to exercise Register's consistency check.
type mismatchedDriver struct{ stubDriver }

func (d *mismatchedDriver) Model() string { return "SomethingElse" }

func TestRegistry_Models_Sorted(t *testing.T) {
	r := driver.NewRegistry()
	for _, m := range []string{"Zulu", "Alpha", "Mike"} {
		if err := r.Register(&stubDriver{caps: validStubCaps(m)}); err != nil {
			t.Fatalf("Register(%q): unexpected error: %v", m, err)
		}
	}

	got := r.Models()
	want := []string{"Alpha", "Mike", "Zulu"}
	if len(got) != len(want) {
		t.Fatalf("Models() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Models() = %v, want %v (sorted)", got, want)
		}
	}
}

func TestRegistry_Models_Empty(t *testing.T) {
	r := driver.NewRegistry()
	if got := r.Models(); len(got) != 0 {
		t.Errorf("Models() on an empty registry = %v, want empty", got)
	}
}
