// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"context"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// params is this radio's yaesu.Params value, used ONLY for the two
// generic helpers this package reuses (yaesu.NewEngine, yaesu.Handshake),
// both of which consult nothing but Name. This driver does NOT go through
// yaesu.WriteChannel/BuildWriteCommand/MTSpec/MTSetSpec or yaesu.
// DiscoverInventory — see doc.go for why (no MT command, no 60m/EMG bank).
var params = yaesu.Params{Name: "ftdx5000", Model: modelName, Dialect: dialect}

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ftdx5000Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftdx5000Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields — see core/driver/ftdx10/
// ftdx10.go's own doc comment for the full reasoning, unchanged here:
// consent is a statement about a SESSION, never about the radio, applied
// once at session-capability assembly (sessionCapabilities), and it never
// reaches spec.FieldErase or an unrecognised Profile.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ftdx5000Driver) {
		d.Consented = true
	}
}

// New builds the FTdx5000 driver for profile. RealHardware — the ZERO
// VALUE — selects the all-Unverified capability set while
// writeTrialsComplete is false, and ANY unrecognised Profile value
// deliberately selects the same fail-safe.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ftdx5000Driver{Base: driver.Base{Profile: profile}, dialect: dialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftdx5000Driver implements driver.Driver for the Yaesu FTdx5000.
type ftdx5000Driver struct {
	driver.Base
	// dialect is the CAT dialect every codec call this driver — and every
	// Session it Opens — makes goes through. Set from the package-level
	// dialect in New; no Option touches it. It lives HERE rather than only
	// on Session because Open builds the transport.Engine, handing it this
	// value, BEFORE any Session exists.
	dialect cat.Dialect
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time.
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ftdx5000Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
func (d *ftdx5000Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// writeTrialsComplete is false: nothing writable anywhere unless
		// the user has consented (sessionCapabilities), which reaches
		// past this label downstream and never touches what this method
		// returns.
		return CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm.
		return CapabilitiesUnverified()
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID, and
// verifies this really is an FTdx5000 (a typed *driver.WrongRadioError
// otherwise), then returns a Session.
//
// NO DISCOVERY PHASE: this radio has no 60m or EMG bank at all (dialect.go,
// doc.go) — "5xx", "5MHz" and "EMG" appear nowhere in this manual's slot
// legends — so there is nothing for a discovery sweep to ask about, and
// Session.Capabilities is this driver's static baseline (plus consent),
// exactly as core/driver/ft991a's own NoProbe radio.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ftdx5000Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
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

// open is Open's body, factored so the error path can close eng in exactly
// one place.
func (d *ftdx5000Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
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

// sessionCapabilities is the ONE place a session's effective capability set
// is assembled: this driver's static baseline, then — only when built with
// WithConsentedUnverifiedWrites AND a recognised profile — the consent
// transform. There is no discovered inventory to fold in (Open discovers
// nothing).
func (d *ftdx5000Driver) sessionCapabilities() spec.Capabilities {
	return d.SessionCaps(d.Capabilities())
}

// profileRecognised reports whether this driver's profile is one of the
// declared constants.
func (d *ftdx5000Driver) profileRecognised() bool { return d.Recognised() }

// Session is the FTdx5000's driver.Session: one open, identity-verified
// connection. Safe for concurrent use.
//
// opMu serialises whole driver operations, taken by WriteChannel only:
// this radio's every operation (ReadChannel's single MR read,
// WriteChannel's single MW set) is one wire exchange, which
// transport.Engine already serialises on its own — the lock exists so a
// WRITE cannot interleave its refusal ladder and its frame with a
// concurrent write on the same session, mirroring core/driver/ftdx10's own
// choice for the same single-exchange shape. There is no settings surface
// to interleave with (doc.go: no EX inventory is built).
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
func (s *Session) Capabilities() spec.Capabilities {
	return s.caps.Clone()
}

// Diagnostics reports this session's transport-level health counters.
// Satisfies the optional driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

// ErrAnswerMismatch is the sentinel a caller compares against (via
// errors.Is) to ask "did a radio answer about the wrong channel?"
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports that a memory answer's decoded slot was not
// the one asked for, naming both.
type AnswerMismatchError = driver.AnswerMismatchError[string]
