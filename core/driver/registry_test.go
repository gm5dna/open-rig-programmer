// SPDX-License-Identifier: GPL-3.0-or-later

package driver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
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

// bankedStubCaps returns validStubCaps(model) carrying a single bank with
// a single field whose support is fs, so a test can put one support state
// in front of Register and nothing else.
func bankedStubCaps(model string, fs spec.FieldSupport) spec.Capabilities {
	caps := validStubCaps(model)
	caps.Banks = []spec.Bank{{
		ID:    spec.BankMemory,
		Label: "Memory",
		Slots: []string{"001"},
		Fields: map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency: fs,
		},
	}}
	return caps
}

// TestRegistry_Register_RejectsConsentedUnverifiedBaseline pins the
// architectural boundary this registry exists to enforce:
// ConsentedUnverified is a SESSION-ONLY state, minted by
// spec.ConsentUnverifiedWrites from a user's recorded acceptance of an
// unproven write. A driver's static Capabilities() is a BASELINE — a
// description of the radio — so a baseline carrying ConsentedUnverified
// would be claiming one particular user's consent as a permanent property
// of the hardware, and every session derived from it would start already
// consenting to writes nobody had agreed to.
//
// spec.Capabilities.Validate cannot catch this on the write side: a
// session's capability set legitimately carries write-side
// ConsentedUnverified and must still validate. Register is the one place
// where only static, baseline capabilities pass, so it is where the
// boundary is enforced.
func TestRegistry_Register_RejectsConsentedUnverifiedBaseline(t *testing.T) {
	t.Run("write side", func(t *testing.T) {
		r := driver.NewRegistry()
		caps := bankedStubCaps("Consenting", spec.FieldSupport{
			Read:  spec.Supported,
			Write: spec.ConsentedUnverified,
		})

		err := r.Register(&stubDriver{caps: caps})
		if err == nil {
			t.Fatal("Register: got nil error, want a rejection of a baseline carrying write-side ConsentedUnverified")
		}
		for _, want := range []string{"ConsentedUnverified", "session-only"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Register error = %q, want it to mention %q", err, want)
			}
		}
		if _, ok := r.Get("Consenting"); ok {
			t.Error("Get(\"Consenting\") = _, true; a rejected driver must not be registered")
		}
	})

	// The read side is already refused by spec.Capabilities.Validate
	// (consent is a write-side state; reads need no consent), which
	// Register calls first — so Register's own read-side arm is defence
	// in depth, not the only guard. Assert the rejection at THIS level
	// anyway: the guarantee callers of Register rely on is "no
	// registered driver's capabilities carry ConsentedUnverified in
	// either direction", and that must hold regardless of which check
	// happens to fire.
	t.Run("read side", func(t *testing.T) {
		r := driver.NewRegistry()
		caps := bankedStubCaps("Consenting", spec.FieldSupport{
			Read:  spec.ConsentedUnverified,
			Write: spec.Supported,
		})

		err := r.Register(&stubDriver{caps: caps})
		if err == nil {
			t.Fatal("Register: got nil error, want a rejection of a baseline carrying read-side ConsentedUnverified")
		}
		if !strings.Contains(err.Error(), "ConsentedUnverified") {
			t.Errorf("Register error = %q, want it to mention %q", err, "ConsentedUnverified")
		}
		if _, ok := r.Get("Consenting"); ok {
			t.Error("Get(\"Consenting\") = _, true; a rejected driver must not be registered")
		}
	})
}

// TestRegistry_Register_RealCompositionRootStillRegisters is the other
// half of the guard: the check above must reject consent-carrying
// baselines WITHOUT rejecting any real one. internal/wiring is the
// project's composition root, so registering its real driver through it
// exercises the production path end to end. (Every other real driver is
// covered by its own package's tests and by the full suite; this is the
// explicit canary that the new check has not broken registration for a
// driver built the way production builds them.)
func TestRegistry_Register_RealCompositionRootStillRegisters(t *testing.T) {
	r, err := wiring.NewRegistry(wiring.NewRealDriver())
	if err != nil {
		t.Fatalf("wiring.NewRegistry(wiring.NewRealDriver()): unexpected error: %v", err)
	}
	if got := r.Models(); len(got) != 1 {
		t.Fatalf("Models() = %v, want exactly one registered model", got)
	}
}

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
