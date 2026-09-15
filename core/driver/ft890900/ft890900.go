// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures a driver New*-built value.
type Option func(*ft890900Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft890900Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields (see driver.Base.SessionCaps).
// writeTrialsComplete is false for both radios (caps.go), so every
// RealHardware session stays on CapabilitiesUnverified until one is.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft890900Driver) { d.Consented = true }
}

// NewFT890 builds the FT-890 driver for profile.
func NewFT890(profile Profile, opts ...Option) driver.Driver {
	return newDriver(ft890Info, profile, opts...)
}

// NewFT900 builds the FT-900 driver for profile.
func NewFT900(profile Profile, opts ...Option) driver.Driver {
	return newDriver(ft900Info, profile, opts...)
}

func newDriver(info modelInfo, profile Profile, opts ...Option) driver.Driver {
	d := &ft890900Driver{Base: driver.Base{Profile: profile}, info: info}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft890900Driver implements driver.Driver for both the FT-890 and the
// FT-900 — info is the only thing that varies (see modelInfo).
type ft890900Driver struct {
	driver.Base
	info            modelInfo
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ft890900Driver) Model() string { return d.info.Name }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile. Open discovers nothing beyond identity, so
// Session.Capabilities is a copy of this same set, plus the consent
// transform when given.
func (d *ft890900Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return d.info.CapabilitiesSimulated()
	case RealHardware:
		return d.info.CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe: nothing writable.
		return d.info.CapabilitiesUnverified()
	}
}

func (d *ft890900Driver) sessionCapabilities() spec.Capabilities {
	return d.SessionCaps(d.Capabilities())
}

// probeIdentity runs Open's two fixed Status-Update probes (spec.md
// §Identity probe, Codex #2) and returns nil once info's identity is
// confirmed, or a typed *driver.WrongRadioError / plain error otherwise.
// Neither probe, nor any step of the write choreography, may reach the
// wire before both probes resolve — this is the ONLY thing Open's identity
// step does.
// canceller is bincat.PendingWantCanceller: an optional interface, so
// probeIdentity degrades gracefully (never crashes) if a future framing
// doesn't implement it — see its own doc comment for why this driver
// needs it at all.
func probeIdentity(ctx context.Context, eng *transport.Engine, info modelInfo, canceller bincat.PendingWantCanceller) error {
	probe := func(ch byte) error {
		cmd := bincat.NewCommand(bincat.OpStatusUpdate, [4]byte{bincat.UMemoryRecord, 0, 0, ch})
		_, err := eng.Do(ctx, cmd, bincat.ReadSpec(0, 0))
		return err
	}

	// Probe 1 (CH=01h): must answer, in range for both radios. A failure
	// here means no radio, the wrong protocol, or a transport fault — not
	// a wrong-radio verdict, since nothing has yet distinguished the two
	// models.
	if err := probe(probeChannel1); err != nil {
		return fmt.Errorf("ft890900: %s: Open: identity probe 1 (CH=%02Xh) got no answer: %w", info.Name, probeChannel1, err)
	}

	// Probe 2 (CH=21h/33): the boundary channel. Silence and an answer
	// are both meaningful signals here (the family's documented
	// no-ack/no-NAK model, spec.md §Context) — bounded by probe 1 already
	// having proved a radio speaking this protocol is present.
	err := probe(probeChannel2)
	switch {
	case err == nil && info.AnswersBoundaryProbe:
		return nil // confirmed: an answer is exactly what this model predicts
	case err == nil:
		other := otherInfo(info)
		return &driver.WrongRadioError{Want: info.CATID, Got: other.CATID, WantModel: info.Name, GotModel: other.Name}
	case errors.Is(err, transport.ErrTimeout) && !info.AnswersBoundaryProbe:
		// CONFIRMED, via the silence this model's own true range predicts
		// — but that silence leaves the accumulator's own bookkeeping
		// expecting a reply that will NEVER arrive (bincat's
		// PendingWantCanceller doc comment). Retract it before this
		// Engine is trusted with anything else, or the next genuine
		// exchange's real reply gets misattributed to this one.
		if canceller != nil {
			canceller.CancelPendingWant()
		}
		return nil
	case errors.Is(err, transport.ErrTimeout):
		other := otherInfo(info)
		return &driver.WrongRadioError{Want: info.CATID, Got: other.CATID, WantModel: info.Name, GotModel: other.Name}
	default:
		return fmt.Errorf("ft890900: %s: Open: identity probe 2 (CH=%02Xh): %w", info.Name, probeChannel2, err)
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// drains it to quiet, probes identity, and returns a Session.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ft890900Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	framing, err := bincat.NewFraming(d.info.EngineProfile)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ft890900: %s: Open: %w", d.info.Name, err)
	}
	eng, err := transport.NewEngineWith(port, framing, transport.WithLogger(d.transportLogger))
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ft890900: %s: Open: %w", d.info.Name, err)
	}
	canceller, _ := framing.(bincat.PendingWantCanceller)
	sess, err := d.open(ctx, eng, id, canceller)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

func (d *ft890900Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity, canceller bincat.PendingWantCanceller) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ft890900: %s: Open: %w", d.info.Name, err)
		}
	}
	if err := probeIdentity(ctx, eng, d.info, canceller); err != nil {
		return nil, err
	}
	id.CATID = d.info.CATID

	return &Session{
		eng:  eng,
		info: d.info,
		id:   id,
		caps: d.sessionCapabilities(),
	}, nil
}

// Session is the FT-890/FT-900 driver.Session: one open, identity-verified
// connection. Safe for concurrent use.
type Session struct {
	eng  *transport.Engine
	info modelInfo
	opMu sync.Mutex
	id   driver.Identity
	caps spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a deep copy per call.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters, per
// the optional driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

var (
	_ driver.Driver              = (*ft890900Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

// Profile selects which capability profile New* builds the driver with.
// The zero value is RealHardware on purpose.
type Profile = driver.Profile

// RealHardware and Simulated are this package's own names for the shared
// profile constants.
const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)
