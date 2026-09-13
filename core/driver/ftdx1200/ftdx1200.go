// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"context"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// modelName is this package's registry-key spelling (matrix §2.1).
const modelName = "FTdx1200"

// acceptedCATIDs is BOTH IDs this ONE radio identifies with: "0582" (FFT-1
// fitted) or "0583" (not fitted) — matrix §1.6/§2.2, an option-split on a
// single product, not two models. This is a shape variation on
// yaesu.Handshake (which compares a single string): ftdx101's dual-catID
// D/MP pair is the nearest precedent for "two accepted IDs" (its own
// Want.CATID/Sibling machinery), but that precedent is two constructors
// over two dialects; this package registers ONE row, so identify() below
// accepts either ID directly rather than reaching for two Wants.
var acceptedCATIDs = []string{"0582", "0583"}

func catIDAccepted(got string) bool {
	for _, id := range acceptedCATIDs {
		if id == got {
			return true
		}
	}
	return false
}

// params is this radio's yaesu.Params value, used only for the generic
// engine-setup helper this package reuses (yaesu.NewEngine) — identify()
// below replaces yaesu.Handshake's single-ID compare with the
// accept-either-ID logic this radio's option split needs.
var params = yaesu.Params{Name: "ftdx1200", Model: modelName, Dialect: dialect}

// Profile selects which capability description a driver value publishes.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// Option configures the driver New builds.
type Option func(*ftdx1200Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftdx1200Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields — see core/driver/ftdx101's own
// doc comment for the full reasoning.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ftdx1200Driver) { d.Consented = true }
}

// New builds the FTdx1200 driver for profile. ONE row, bare New: matrix
// §5's own two-package verdict against ftdx3000 gives this package
// nothing to disambiguate, and the "0582"/"0583" option split is handled
// inside identify(), not by a second constructor (there is only one
// product here, unlike ftdx101's genuinely separate D/MP). RealHardware —
// the zero value — selects the all-Unverified capability set while
// writeTrialsComplete is false: NO FTdx1200 HAS EVER BEEN ASKED ANYTHING
// BY THIS PROJECT. Any unrecognised Profile value selects the same
// fail-safe.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ftdx1200Driver{Base: driver.Base{Profile: profile}, dialect: dialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftdx1200Driver implements driver.Driver for the Yaesu FTdx1200.
type ftdx1200Driver struct {
	driver.Base
	dialect         cat.Dialect
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ftdx1200Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
func (d *ftdx1200Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		return capabilitiesUnverified()
	default:
		return capabilitiesUnverified()
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (AI0 init + drain-to-quiet), probes ID, and
// verifies the answer is ONE of this radio's two accepted IDs (a typed
// *driver.WrongRadioError otherwise). No discovery phase: this radio has
// no 60m or EMG bank at all.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ftdx1200Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	eng, err := yaesu.NewEngine(port, d.dialect, d.transportLogger, &params)
	if err != nil {
		return nil, err
	}
	sess, err := d.open(ctx, eng, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// identify runs the ID probe and confirms the answer is one of this
// radio's TWO accepted CAT IDs (acceptedCATIDs) — the option-split shape
// matrix §1.6/§2.2 pins, distinct from yaesu.Handshake's single-ID
// compare (which a genuinely two-model package like ftdx101 uses once per
// row). Duplicated in miniature rather than stretching Handshake's Want
// to carry a slice: one call site, one radio, and the refusal text needs
// to name BOTH accepted IDs, which Want's single Want.CATID field cannot.
func identify(ctx context.Context, eng *transport.Engine, dialect cat.Dialect) (string, error) {
	if err := eng.Init(ctx); err != nil {
		return "", fmt.Errorf("%s: Open: %w", params.Name, err)
	}
	frame, err := eng.Do(ctx, dialect.BuildIDRead(), yaesu.IDSpec())
	if err != nil {
		return "", fmt.Errorf("%s: Open: ID probe: %w", params.Name, err)
	}
	got, err := dialect.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("%s: Open: ID probe: %w", params.Name, err)
	}
	if !catIDAccepted(got) {
		return "", &driver.WrongRadioError{Want: "0582 or 0583", Got: got, WantModel: modelName}
	}
	return got, nil
}

func (d *ftdx1200Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	got, err := identify(ctx, eng, d.dialect)
	if err != nil {
		return nil, err
	}
	id.CATID = got

	return &Session{
		eng:     eng,
		dialect: d.dialect,
		id:      id,
		caps:    d.SessionCaps(d.Capabilities()),
	}, nil
}

// Session is the FTdx1200's driver.Session: one open, identity-verified
// connection.
type Session struct {
	eng     *transport.Engine
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open

	opMu sync.Mutex
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a deep copy per call.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

// ErrAnswerMismatch is the sentinel a caller compares against (via
// errors.Is) to ask "did a radio answer about the wrong channel?".
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports that an MR answer's decoded slot was not
// the one asked for.
type AnswerMismatchError = driver.AnswerMismatchError[string]
