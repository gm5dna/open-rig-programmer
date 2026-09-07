// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Profile selects which capability description a driver value publishes.
//
// Two profiles: the fail-safe one a real radio gets, and the one the fake
// gets. THE ZERO VALUE IS THE SAFE ONE — RealHardware — so a caller that
// forgets to choose gets the profile that writes nothing, not the profile
// that writes everything. Every driver package defined this same pair with
// this same order; this is the one declaration they share.
//
// CONSUMERS ALIAS IT, THEY DO NOT IMPORT IT BARE. internal/guards'
// TestSimulatedProfileTokensConfinement requires each driver package to
// keep its OWN <pkg>.Simulated selector — that is what confines the
// fake-only profile to one non-test file per driver — so a driver package
// adopting this type declares:
//
//	type Profile = driver.Profile
//	const (
//		RealHardware = driver.RealHardware
//		Simulated    = driver.Simulated
//	)
//
// An ALIAS and untyped constant re-declarations, never a fresh named type:
// the alias keeps ft710.Profile and driver.Profile the same type, so Base
// can be embedded and the package's own switch statements still compile,
// while ft710.Simulated remains the selector the guard walks for.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While a model's write trials are incomplete it selects that model's
	// all-Unverified capability set: nothing writable without recorded
	// consent.
	RealHardware Profile = iota
	// Simulated is the profile for fake-radio-backed sessions ONLY (the
	// CLI's --fake mode, the GUI's demo mode), where the write
	// choreography can be exercised end to end with no hardware at risk.
	Simulated
)

// String renders the profile for diagnostics.
func (p Profile) String() string {
	switch p {
	case RealHardware:
		return "RealHardware"
	case Simulated:
		return "Simulated"
	default:
		return fmt.Sprintf("Profile(%d)", int(p))
	}
}

// Base is the state every driver value carries: which profile it
// publishes, and whether the caller passed WithConsentedUnverifiedWrites.
// A driver embeds it in place of its own profile/consented pair.
type Base struct {
	// Profile selects which capability description this driver publishes.
	Profile Profile
	// Consented records that the caller explicitly accepted the risk of
	// writing fields no hardware trial has verified.
	Consented bool
}

// Recognised reports whether this driver's profile is one of the declared
// Profile constants — the same set every driver's capability switch names
// explicitly, restated here so the consent gate cannot drift open for a
// profile the switch would fail safe on.
//
// A profile value from outside the constant set (a caller casting an
// integer) is UNRECOGNISED: the capability switch's default arm hands it
// the fail-safe set, and this predicate keeps consent from transforming
// that set into a writable one.
func (b Base) Recognised() bool {
	return b.Profile == RealHardware || b.Profile == Simulated
}

// SessionCaps applies the consent transform to caps — this driver's static
// capability set — when, and only when, the driver was built with consent
// AND its profile is recognised. It is the shared form of the per-driver
// sessionCapabilities, and the fail-safe direction survives consent: an
// unrecognised profile is returned untransformed.
//
// Callers pass their OWN Capabilities() result: what a session enforces
// and what Capabilities() hands out must be the same value, and the
// static set differs per driver (some fold in discovered banks).
func (b Base) SessionCaps(caps spec.Capabilities) spec.Capabilities {
	if b.Consented && b.Recognised() {
		return spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}
