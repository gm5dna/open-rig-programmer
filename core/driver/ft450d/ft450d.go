// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"context"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds. See WithTransportLogger,
// WithConsentedUnverifiedWrites.
type Option func(*ft450dDriver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft450dDriver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields — see driver.Base.SessionCaps and
// spec.ConsentUnverifiedWrites for what consent means. Consent is a
// statement about a SESSION, never about the radio: this driver's static
// Capabilities is untouched by the option, and only the set Open assembles
// carries the state. No FT-450D write trial has ever been run
// (writeTrialsComplete is false, caps.go), so every RealHardware session
// stays on CapabilitiesUnverified until one is. Consent never reaches the
// PMS bank's write posture: that column is Unsupported, not Unverified
// (caps.go's pmsFields), and consent only opens an Unverified label — the
// SAFE SHAPE ruling's PMS caution survives consent unconditionally.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft450dDriver) { d.Consented = true }
}

// New builds the FT-450D driver for profile. BARE New: this is a
// single-row, own-document package (matrix header: "own document — no
// sibling row, bare New"). RealHardware — the zero value — selects the
// all-Unverified capability set (writeTrialsComplete is false, caps.go);
// any unrecognised Profile value selects the same fail-safe.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ft450dDriver{Base: driver.Base{Profile: profile}, dialect: catDialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft450dDriver implements driver.Driver for the Yaesu FT-450D.
type ft450dDriver struct {
	driver.Base
	dialect         cat.Dialect
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ft450dDriver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile. There is no discovered bank on this radio (Open probes
// nothing beyond identity — 505-510 is deliberately not in the dialect at
// all, dialect.go) — Session.Capabilities is therefore a copy of this same
// set, plus the consent transform when given.
func (d *ft450dDriver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		return CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe: nothing writable.
		return CapabilitiesUnverified()
	}
}

// params is this radio's configuration of the Yaesu NEWCAT engine
// constructor (core/driver/internal/yaesu). Only Name/Model/Dialect are
// meaningful here: this package never calls yaesu.WriteChannel,
// yaesu.ReadChannel-shaped helpers or yaesu.DiscoverInventory (Probe stays
// its zero value, NoProbe — no 60m/EMG inventory to discover, dialect.go),
// because this radio's write/read paths go through plain MR/MW, not the
// combined MT form those helpers assume — see write.go's and read.go's own
// doc comments.
var params = yaesu.Params{
	Name:    "ft450d",
	Model:   modelName,
	Dialect: catDialect,
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID, and
// verifies this really is an FT-450D — a typed *driver.WrongRadioError
// otherwise. It runs no 505-510 discovery sweep: that span is not in the
// dialect at all (dialect.go), so Session.Capabilities is exactly the
// static baseline, with no discovered banks appended.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ft450dDriver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
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

func (d *ft450dDriver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	got, err := yaesu.Handshake(ctx, eng, d.dialect, &params, yaesu.Want{CATID: catID, Model: modelName})
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

// sessionCapabilities is the one place a session's effective capability
// set is assembled: this driver's static baseline, then the consent
// transform when given (driver.Base.SessionCaps).
func (d *ft450dDriver) sessionCapabilities() spec.Capabilities {
	return d.SessionCaps(d.Capabilities())
}

// Session is the FT-450D's driver.Session: one open, identity-verified
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
// error actually returned is an *AnswerMismatchError naming both addresses.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports that an MR answer's decoded slot was not the
// one asked for.
type AnswerMismatchError = driver.AnswerMismatchError[string]

// Profile selects which capability profile New builds the driver with. The
// zero value is RealHardware on purpose: a forgotten or zero-valued
// Profile must fail towards the real-hardware capability set, never the
// simulator's.
type Profile = driver.Profile

// RealHardware and Simulated are this package's own names for the shared
// profile constants — an alias and untyped re-declarations, never a fresh
// named type.
const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)
