// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"context"
	"fmt"

	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ic905Driver)

// New builds the IC-905 driver for profile. RealHardware — the ZERO
// VALUE — selects the all-Unverified capability set while
// writeTrialsComplete is false (nothing writable; see that constant's doc
// comment), and ANY unrecognised Profile value deliberately selects the
// same fail-safe: the failure direction for a forged or corrupted Profile
// is always "nothing writable", never a writable set.
//
// ONE constructor, because there is one radio. core/driver/ftdx101 offers
// NewD and NewMP because it drives two; this package's model is fixed by
// the package.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ic905Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ic905Driver implements driver.Driver for the Icom IC-905.
type ic905Driver struct {
	profile Profile
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default.
	consentUnverifiedWrites bool
}

// Model implements driver.Driver: the registry key and display name,
// taken from the DIALECT rather than restated, so the two cannot drift.
func (d *ic905Driver) Model() string { return civic905.Model }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile.
//
// MEM's Slots is EMPTY here and that is the point: it is a sparse bank
// whose materialised set is discovered per session at Open (see
// Session.Capabilities for the effective, per-radio set). The static
// baseline is what internal/wiring's registry publishes, what the app
// describes the model with, and what offline synthesis classifies
// against — none of which has a radio to ask.
func (d *ic905Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		// No IC-905 has ever been written to over CI-V by this project:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe — nothing writable, every write
		// refused before a frame is built UNLESS the user has consented,
		// which is the one thing that reaches past these labels and does
		// so downstream of this method (sessionCapabilities transforms
		// what this arm returns; the set returned HERE is never
		// transformed, and that is what
		// internal/wiring.NeedsUnverifiedConsent reads).
		//
		// writeTrialsComplete is deliberately NOT read here: see its doc
		// comment for what a flip must change, and why a constant that
		// was load-bearing on its own would let a one-character edit
		// unlock a write.
		return capabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence.
		return capabilitiesUnverified()
	}
}

// profileRecognised reports whether this driver's profile is one of the
// package's declared Profile constants — the same set Capabilities'
// switch names explicitly, restated here so the consent gate cannot drift
// open for a profile that switch would fail safe on.
func (d *ic905Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// Open implements driver.Driver.
//
// NOT IMPLEMENTED AT THIS TASK, and honestly so: the probe, the framing
// adapter and the Session are Task 11's, discovery is Task 13's, and a
// driver whose Open says "not implemented yet" is better than one that
// opens a session it cannot yet defend. Capabilities() above is testable
// without any of it.
//
// It still takes ownership of port on this path, exactly as
// core/driver's contract requires on BOTH outcomes.
func (d *ic905Driver) Open(_ context.Context, port transport.Port, _ driver.Identity) (driver.Session, error) {
	if port != nil {
		_ = port.Close()
	}
	return nil, fmt.Errorf("ic905: Open is not implemented yet")
}
