// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds. See WithTransportLogger,
// WithConsentedUnverifiedWrites.
type Option func(*ftdx9000Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine — see the sibling drivers'
// identical reasoning. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftdx9000Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records the user's consent to writing this
// radio's Unverified fields — see the sibling drivers' identical reasoning.
// No FTdx9000 write trial has ever been run, so there is no
// CapabilitiesRealHardware profile for consent to reach past; every
// RealHardware session stays on CapabilitiesUnverified until one is.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ftdx9000Driver) {
		d.Consented = true
	}
}

// New builds the FTdx9000 driver for profile. RealHardware — the zero
// value — selects the all-Unverified capability set (writeTrialsComplete
// is false); any unrecognised Profile value selects the same fail-safe.
// ONE row, bare New: matrix §1.1/§1.2, "FTdx9000" registers as a single row
// even though its ID probe may answer one of three CAT IDs (acceptedCATIDs,
// caps.go).
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ftdx9000Driver{Base: driver.Base{Profile: profile}, dialect: catDialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftdx9000Driver implements driver.Driver for the Yaesu FTdx9000.
type ftdx9000Driver struct {
	driver.Base
	dialect         cat.Dialect
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ftdx9000Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile. There is no discovered bank on this radio at all
// (matrix §1.4: no 60 m, no EMG) — Session.Capabilities is therefore a
// copy of this same set, plus the consent transform when given.
func (d *ftdx9000Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		return CapabilitiesUnverified()
	default:
		return CapabilitiesUnverified()
	}
}

// params is this radio's configuration of the Yaesu NEWCAT engine
// constructor (core/driver/internal/yaesu). Only Name/Model/Dialect are
// meaningful here: this package never calls yaesu.WriteChannel,
// yaesu.ReadChannel-shaped helpers or yaesu.DiscoverInventory (Probe stays
// its zero value, NoProbe — matrix §1.4, no 60m/EMG inventory to
// discover), because this radio's write/read paths go through plain
// MR/MW, not the combined MT form those helpers assume — see write.go's
// and read.go's own doc comments, and doc.go.
var params = yaesu.Params{
	Name:    "ftdx9000",
	Model:   modelName,
	Dialect: catDialect,
}

// Open implements driver.Driver: builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID against
// acceptedCATIDs, and returns the Session. No discovery phase: this radio
// has no 60m/EMG bank to probe for (matrix §1.4), so — like the FT-991A —
// the transcript is exactly two frames on a match.
func (d *ftdx9000Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
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

func (d *ftdx9000Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	got, err := handshake(ctx, eng, d.dialect)
	if err != nil {
		return nil, err
	}
	id.CATID = got

	return &Session{
		eng:     eng,
		dialect: d.dialect,
		id:      id,
		caps:    d.sessionCapabilities(),
	}, nil
}

// handshake is this package's own ID probe, NOT yaesu.Handshake: that
// shared helper accepts exactly one CAT ID, and this ONE registered row
// must accept any of THREE (matrix §1.2) — the FTDX9000D/Contest/MP
// sub-variants this project registers as a single radio. Otherwise
// identical to yaesu.Handshake's own body (AI0 already sent by Init before
// this runs; here only the ID probe itself).
func handshake(ctx context.Context, eng *transport.Engine, dialect cat.Dialect) (string, error) {
	if err := eng.Init(ctx); err != nil {
		return "", fmt.Errorf("ftdx9000: Open: %w", err)
	}
	frame, err := eng.Do(ctx, dialect.BuildIDRead(), yaesu.IDSpec())
	if err != nil {
		return "", fmt.Errorf("ftdx9000: Open: ID probe: %w", err)
	}
	got, err := dialect.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ftdx9000: Open: ID probe: %w", err)
	}
	for _, want := range acceptedCATIDs {
		if got == want {
			return got, nil
		}
	}
	return "", &driver.WrongRadioError{Want: strings.Join(acceptedCATIDs, "/"), Got: got, WantModel: modelName}
}

// sessionCapabilities is the one place a session's effective capability
// set is assembled: this driver's static baseline, then the consent
// transform when given (driver.Base.SessionCaps).
func (d *ftdx9000Driver) sessionCapabilities() spec.Capabilities {
	return d.SessionCaps(d.Capabilities())
}

// Session is the FTdx9000's driver.Session: one open, identity-verified
// connection. Safe for concurrent use.
//
// IT CARRIES AN OPERATION MUTEX, like every registered Yaesu driver: this
// radio's WriteChannel is one exchange (write.go) and ReadChannel is one
// exchange (read.go), but the shared rule ("one driver operation at a
// time") is easier to keep true uniformly than as a per-radio exception.
type Session struct {
	eng     *transport.Engine
	opMu    sync.Mutex
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a deep copy per call — see
// spec.Capabilities.Clone for why the copy is load-bearing.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters, per
// the optional driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

// ErrAnswerMismatch is the sentinel a caller compares against (via
// errors.Is) to ask "did a radio answer about the wrong channel?" The
// error actually returned is an *AnswerMismatchError naming both
// addresses.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports that a memory answer's decoded slot was not
// the one asked for.
type AnswerMismatchError = driver.AnswerMismatchError[string]
