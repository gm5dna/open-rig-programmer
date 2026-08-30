// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver and every session it opens.
type Option func(*icr8600Driver)

// WithTransportLogger exposes transport contamination and quarantine
// diagnostics without changing protocol behaviour.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *icr8600Driver) {
		if l != nil {
			d.transportOptions = append(d.transportOptions, transport.WithLogger(l))
		}
	}
}

// WithConsentedUnverifiedWrites records the user's session-local consent.
// It changes no static capability and never consents FieldErase.
func WithConsentedUnverifiedWrites() Option {
	return func(d *icr8600Driver) { d.consented = true }
}

// New returns the one-radio IC-R8600 driver.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &icr8600Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type icr8600Driver struct {
	profile          Profile
	consented        bool
	transportOptions []transport.Option
	// Non-zero only in focused tests. Production deliberately takes the
	// transport defaults until Stage R measures this radio.
	readTimeout time.Duration
	settle      time.Duration
}

func (d *icr8600Driver) Model() string { return civicr8600.Model }

func (d *icr8600Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		return CapabilitiesUnverified()
	default:
		return CapabilitiesUnverified()
	}
}

func (d *icr8600Driver) profileRecognised() bool {
	return d.profile == RealHardware || d.profile == Simulated
}

func (d *icr8600Driver) sessionCapabilities(discovered []string, catID string) spec.Capabilities {
	caps := d.Capabilities()
	for i := range caps.Banks {
		if caps.Banks[i].ID == spec.BankMemory {
			caps.Banks[i].Slots = append([]string(nil), discovered...)
		}
	}
	caps.CATID = catID
	if d.consented && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// Open sends only the address-matched 19 00 read and bounded 1A 00 memory
// reads. civ's initialisation sequence is empty, so it never mutates the
// receiver merely by opening it.
func (d *icr8600Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	profile := civicr8600.Profile()
	framing, err := civ.NewFraming(profile)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("icr8600: Open: framing: %w", err)
	}
	stats, _ := framing.(civ.AccumulatorStatsReporter)
	eng, err := transport.NewEngineWith(port, framing, d.transportOptions...)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("icr8600: Open: %w", err)
	}
	s, err := d.open(ctx, eng, profile, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return s, nil
}

func (d *icr8600Driver) open(ctx context.Context, eng *transport.Engine, profile civ.Profile, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	s := &Session{eng: eng, profile: profile, stats: stats, readTimeout: d.readTimeout, settle: d.settle}
	if err := eng.Init(ctx); err != nil && !errors.Is(err, transport.ErrDrainCapExceeded) {
		return nil, fmt.Errorf("icr8600: Open: %w", err)
	}

	cmd, err := profile.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("icr8600: Open: building 19 00: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, s.idSpec())
	if err != nil {
		// A receiver moved away from 96h or from the assumed default baud
		// times out here. Neither case is attributed to a different model.
		return nil, fmt.Errorf("icr8600: Open: 19 00 identity probe: %w", err)
	}
	token, err := profile.ParseTransceiverID(frame)
	if err != nil {
		return nil, fmt.Errorf("icr8600: Open: 19 00 identity probe: %w", err)
	}

	discovered, err := s.discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("icr8600: Open: bounded occupied-slot search: %w", err)
	}
	catID := fmt.Sprintf("%02X:%s", civicr8600.RadioAddress, strings.ToUpper(token))
	id.CATID = catID
	s.id = id
	s.caps = d.sessionCapabilities(discovered, catID)
	if s.report.Fingerprinted {
		s.report.AddressDiagnostic = fmt.Sprintf("address %02X confirmed; record fingerprint %d", civicr8600.RadioAddress, s.report.RecordLength)
	} else {
		s.report.AddressDiagnostic = fmt.Sprintf("address %02X confirmed; UNFINGERPRINTED: bounded occupied-slot search found no record", civicr8600.RadioAddress)
	}
	return s, nil
}

// OpenReport records the bounded probe result without making it part of the
// neutral driver seam.
type OpenReport struct {
	SlotsTried        int
	Fingerprinted     bool
	RecordLength      int
	FirstOccupied     string
	AddressDiagnostic string
}

// Session is one address-matched IC-R8600 connection.
type Session struct {
	eng     *transport.Engine
	profile civ.Profile
	stats   civ.AccumulatorStatsReporter
	id      driver.Identity
	caps    spec.Capabilities
	report  OpenReport

	readTimeout time.Duration
	settle      time.Duration

	answerMismatches atomic.Uint64
	writeMu          sync.Mutex
}

func (s *Session) idSpec() transport.CommandSpec {
	spec := civ.CIVReadSpec(s.profile.TransceiverIDAnswerMatcher(), 1)
	spec.Timeout, spec.Settle = s.readTimeout, s.settle
	return spec
}

func (s *Session) memoryReadSpec() transport.CommandSpec {
	spec := civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), 1)
	spec.Timeout, spec.Settle = s.readTimeout, s.settle
	return spec
}

func (s *Session) Identity() driver.Identity { return s.id }

func (s *Session) Capabilities() spec.Capabilities { return cloneCapabilities(s.caps) }

func (s *Session) Close() error { return s.eng.Close() }

func (s *Session) OpenDiagnostics() OpenReport { return s.report }

func (s *Session) WireStats() civ.AccumulatorStats {
	if s.stats == nil {
		return civ.AccumulatorStats{}
	}
	return s.stats.AccumulatorStats()
}

func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames()) + uint64(s.WireStats().Unexpected)}
}

var (
	_ driver.Driver              = (*icr8600Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)
