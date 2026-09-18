// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"context"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// params is this radio's yaesu.Params value, used only for the generic
// engine-setup/handshake/MT-geometry helpers this package reuses
// (yaesu.NewEngine, yaesu.Handshake, yaesu.MTSpec). Probe stays the zero
// value (yaesu.NoProbe): the 5 MHz/EMGCH banks are STATIC here, not
// discovered (caps.go's sixtyMSlots doc comment), so Open runs no
// yaesu.DiscoverInventory sweep at all and this Params' Probe/MRAnswerLen
// fields go unused.
var params = yaesu.Params{
	Name: "ftx1",
	// Model deliberately spells the wire's own name, not a body-suffixed
	// one — see modelName's own doc comment.
	Model:   modelName,
	Dialect: dialect,
	// MTRetries 1: an MT read is idempotent, and this driver's own
	// per-channel read is exactly the case that value's own doc comment
	// names.
	MTRetries: 1,
}

// Profile selects which capability description a driver value publishes.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// Option configures the driver New builds.
type Option func(*ftx1Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftx1Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields: at session-capability assembly
// every write-side spec.Unverified label becomes spec.ConsentedUnverified
// (spec.ConsentUnverifiedWrites), which FieldSupport.CanWrite opens. No
// FTX-1 field has ever been proven against real hardware — every write
// this milestone ships stays behind this gate, never enabled by default
// (spec.md §11).
func WithConsentedUnverifiedWrites() Option {
	return func(d *ftx1Driver) { d.Consented = true }
}

// New builds the FTX-1 driver for profile. RealHardware — the zero value
// — selects the all-Unverified capability set (writeTrialsComplete is
// false and always has been): NO FTX-1 HAS EVER BEEN ASKED ANYTHING BY
// THIS PROJECT. Any unrecognised Profile value selects the same
// fail-safe. Options: WithTransportLogger, WithConsentedUnverifiedWrites.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ftx1Driver{Base: driver.Base{Profile: profile}, dialect: dialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftx1Driver implements driver.Driver for the Yaesu FTX-1.
type ftx1Driver struct {
	driver.Base
	// dialect is the CAT dialect every codec call this driver makes — and
	// every Session it Opens makes — goes through. Set from the
	// package-level dialect in New; no Option touches it.
	dialect         cat.Dialect
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ftx1Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
func (d *ftx1Driver) Capabilities() spec.Capabilities {
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

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID; and
// verifies the answer is "0840" (a typed *driver.WrongRadioError
// otherwise — see dialect.go's CATID comment: this ID is shared,
// unmodified, by both bodies, so a match here says nothing about which
// body answered). No discovery phase: caps.go's four banks (including 5
// MHz and EMGCH) are all static.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ftx1Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
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

func (d *ftx1Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
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

// Session is the FTX-1's driver.Session: one open, identity-verified
// connection. Safe for concurrent use — transport.Engine serialises every
// individual exchange, and opMu additionally serialises whole DRIVER
// OPERATIONS (ReadChannel's MR+MT pair, WriteChannel's MW+MT pair), the
// same rule every registered Yaesu dialect in this fleet keeps.
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

// AnswerMismatchError reports that an MR/MT answer's decoded slot was not
// the one asked for, naming both.
type AnswerMismatchError = driver.AnswerMismatchError[string]
