// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts870s is the driver half of the TS-870S registration: the
// static Capabilities baseline (caps.go) and a driver.Driver value.
//
// # Why Open refuses rather than opening a live session
//
// core/kw's Book870S has no transcribed "E;"/"O;" stream-error citation
// (core/kw/errors.go's newStreamError panics on it, deliberately — see
// core/kw/ts870s/doc.go's own note on the lift-K gap). This project's
// standing rule is that a live NewFraming session for such a book "must
// not be wired up ... until a citation lands" (core/kw/framing.go), so
// Open here never attempts one — and today it structurally could not:
// core/kw.NewFramingFor takes a kw.Layout, not the kw.Layout870 this row's
// codec is built on, so there is no live-framing constructor to call in
// the first place.
//
// This is Phase 3's (brief option 2): the driver exists, its
// Capabilities are complete and audited, and Open fails closed with a
// named error rather than ever reaching a real port. Wiring a working
// session is future work, gated on a real stream-error citation landing
// in core/kw/errors.go first.
package ts870s

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ErrNoLiveSession is what Open always returns. See the package doc
// comment.
var ErrNoLiveSession = errors.New("ts870s: no live session this phase — Book870S has no transcribed stream-error citation (core/kw/errors.go), and core/kw.NewFramingFor does not accept a kw.Layout870 — wiring a real session is gated on both landing first")

// New returns the TS-870S driver built with profile.
//
// NO MODEL ENUM AND NO New<Variant>: this is a single-row package (matrix
// "ONE ROW, ONE COLUMN"), so which radio a driver is for is fixed by the
// package rather than by a value a caller could get wrong — the ic7200
// shape.
func New(profile Profile) driver.Driver {
	return &ts870sDriver{Base: driver.Base{Profile: profile}}
}

type ts870sDriver struct {
	driver.Base
}

// Model implements driver.Driver.
func (d *ts870sDriver) Model() string { return "TS-870S" }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile, before any radio has been probed.
func (d *ts870sDriver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	default:
		// writeTrialsComplete is false, so RealHardware and any
		// unrecognised profile both get the all-Unverified fail-safe.
		return CapabilitiesUnverified()
	}
}

// Open implements driver.Driver by always refusing — see the package doc
// comment for why. port is closed before returning, matching every other
// driver's failure path in this tier.
func (d *ts870sDriver) Open(_ context.Context, port transport.Port, _ driver.Identity) (driver.Session, error) {
	_ = port.Close()
	return nil, fmt.Errorf("ts870s: Open: %w", ErrNoLiveSession)
}

var _ driver.Driver = (*ts870sDriver)(nil)
