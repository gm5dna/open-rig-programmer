// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

import (
	"context"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// modelName is this package's registry-key spelling of the manual's own
// "FTDX3000" (matrix §2.1, ID legend layout:759) — a CHOICE over a
// MANUAL-EVIDENCED fact.
const modelName = "FTDX3000"

// catID is this dialect's CAT ID, sourced from the dialect rather than
// restated: one place this string exists, and the value the ID probe
// compares against is the same value the capability data advertises.
var catID = dialect.CATID()

// params is this radio's yaesu.Params value, used only for the generic
// helpers this package reuses (yaesu.NewEngine, yaesu.Handshake) — see
// core/driver/ftdx5000/ftdx5000.go's own params comment for why this
// package does not go through yaesu.WriteChannel/DiscoverInventory: no MT
// command, no 60m/EMG bank (matrix §0/§2.4).
var params = yaesu.Params{Name: "ftdx3000", Model: modelName, Dialect: dialect}

// Profile selects which capability description a driver value publishes.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes.
type Option func(*ftdx3000Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftdx3000Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields — see core/driver/ftdx101's own
// doc comment for the full reasoning, unchanged here: consent is a
// statement about a SESSION, never about the radio, applied once at
// session-capability assembly, and it never reaches spec.FieldErase or an
// unrecognised Profile.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ftdx3000Driver) { d.Consented = true }
}

// New builds the FTDX3000 driver for profile. ONE row, bare New (matrix
// §5's own two-package verdict against ftdx1200 leaves this package with
// nothing to disambiguate). RealHardware — the zero value — selects the
// all-Unverified capability set while writeTrialsComplete is false:
// NO FTDX3000 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Any
// unrecognised Profile value deliberately selects the same fail-safe.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ftdx3000Driver{Base: driver.Base{Profile: profile}, dialect: dialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftdx3000Driver implements driver.Driver for the Yaesu FTDX3000.
type ftdx3000Driver struct {
	driver.Base
	dialect         cat.Dialect
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ftdx3000Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
func (d *ftdx3000Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		return capabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe: nothing writable.
		return capabilitiesUnverified()
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (AI0 init + drain-to-quiet), probes ID, and
// verifies this really is an FTDX3000 (a typed *driver.WrongRadioError
// otherwise). No discovery phase: this radio has no 60m or EMG bank at
// all (dialect.go), so Session.Capabilities is exactly the static
// baseline.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ftdx3000Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
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

func (d *ftdx3000Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	got, err := yaesu.Handshake(ctx, eng, d.dialect, &params, yaesu.Want{CATID: catID, Model: modelName})
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

// Session is the FTDX3000's driver.Session: one open, identity-verified
// connection. Safe for concurrent use — transport.Engine serialises every
// individual exchange, and everything else here is immutable after Open.
//
// ONE OPERATION MUTEX, taken by ReadChannel and WriteChannel: this
// radio's own operations are one wire exchange each (a bare MR, a bare
// MW), so the lock costs nothing, but it is the fleet's one rule rather
// than a per-radio exception.
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
// Satisfies the optional driver.DiagnosticsReporter capability.
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
