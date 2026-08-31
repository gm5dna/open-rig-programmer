// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7100 "github.com/gm5dna/open-rig-programmer/core/civ/ic7100"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

const (
	retryReads = 1

	// probeSlots is how many bank-A MEM channels (A-001..A-probeSlots)
	// the open-time fingerprint probe reads before giving up on finding
	// an occupied one, argued the same way as the sibling drivers' probe
	// bounds (core/driver/ic7300/ic7300.go:31-44,
	// core/driver/ic705/ic705.go:167-176): bounded per spec D3.2, small,
	// and confined to one bank rather than the 495-channel walk (caps.go:107
	// declares the full A-E geometry, Matrix §1 row 3 and §1b) that only
	// runs once the radio is already fingerprinted.
	//
	// EIGHT IS A CHOICE, ARGUED, not a document-derived value: the manual
	// states the memory geometry this bound is small against, never a
	// recommended probe depth.
	// TestProbeSlotsIsEight pins the literal staying eight;
	// TestOpenEmptyRadioIsExplicitlyUnfingerprinted pins only that the probe
	// reads exactly probeSlots slots before giving up, which holds for any
	// value of the constant.
	probeSlots = 8
)

// Option configures the sessions produced by New.
type Option func(*ic7100Driver)

// SiblingLengths maps a foreign record-only length to a model name. It is
// empty in Stage 2: the IC-7100 matrix declares no registered sibling, and
// tier integration owns any later cross-model attribution.
type SiblingLengths map[int]string

// WithTransportLogger supplies transport diagnostics to opened sessions.
func WithTransportLogger(logger transport.Logger) Option {
	return func(d *ic7100Driver) {
		if logger != nil {
			d.transportLogger = logger
		}
	}
}

// WithConsentedUnverifiedWrites records user consent for this session only.
// The static capability set remains Unverified and FieldErase remains zero.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7100Driver) { d.consentUnverifiedWrites = true }
}

// WithSiblingRecordLengths supplies the tier-integration attribution table.
// It changes only diagnostic attribution; it never changes accepted lengths.
func WithSiblingRecordLengths(lengths SiblingLengths) Option {
	return func(d *ic7100Driver) {
		d.siblingLengths = make(SiblingLengths, len(lengths))
		for length, model := range lengths {
			d.siblingLengths[length] = model
		}
	}
}

// New constructs the IC-7100 driver. It intentionally returns only the
// neutral driver seam and does not register the model.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ic7100Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type ic7100Driver struct {
	profile                 Profile
	transportLogger         transport.Logger
	consentUnverifiedWrites bool
	siblingLengths          SiblingLengths
}

func (d *ic7100Driver) Model() string { return "IC-7100" }

func (d *ic7100Driver) Capabilities() spec.Capabilities {
	if d.profile == Simulated {
		return CapabilitiesSimulated()
	}
	return CapabilitiesUnverified()
}

func (d *ic7100Driver) recognised() bool {
	return d.profile == RealHardware || d.profile == Simulated
}

func (d *ic7100Driver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if d.consentUnverifiedWrites && d.recognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// Open takes ownership of port, sends no Init mutation, requires an
// address-matched 19 00 reply, and performs a bounded occupied-slot search.
func (d *ic7100Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	p := civic7100.Profile()
	framing, err := civ.NewFraming(p)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7100: Open: %w", err)
	}
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic7100: Open: CI-V framing does not report accumulator statistics")
	}
	var options []transport.Option
	if d.transportLogger != nil {
		options = append(options, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngineWith(port, framing, options...)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7100: Open: %w", err)
	}
	s, err := d.open(ctx, eng, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return s, nil
}

func (d *ic7100Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := civic7100.Profile()
	s := &Session{eng: eng, stats: stats, profile: p, caps: d.sessionCapabilities(), siblingLengths: d.siblingLengths}
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic7100: Open: %w", err)
		}
		s.diag.InitDrainCapExceeded = true
	}

	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic7100: Open: %w", err)
	}
	answer, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), retryReads))
	if err != nil {
		return nil, fmt.Errorf("ic7100: Open: no address-matched reply to 19 00: %w", err)
	}
	token, err := p.ParseTransceiverID(answer)
	if err != nil {
		return nil, fmt.Errorf("ic7100: Open: parse 19 00: %w", err)
	}
	s.diag.IDToken = token
	id.CATID = "88"
	if token != "" {
		id.CATID += ":" + token
		s.caps.CATID = id.CATID
	}
	s.id = id

	if err := s.probeFingerprint(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) probeFingerprint(ctx context.Context) error {
	for channel := 1; channel <= probeSlots; channel++ {
		want := civ.ChannelAddress{Group: 1, Channel: channel}
		cmd, err := s.profile.BuildMemoryRead(want)
		if err != nil {
			return fmt.Errorf("ic7100: Open: probe A-%03d: %w", channel, err)
		}
		answer, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), retryReads))
		s.diag.ProbeSlotsRead = channel
		if errors.Is(err, transport.ErrRejected) {
			continue
		}
		if err != nil {
			return fmt.Errorf("ic7100: Open: probe A-%03d: %w", channel, err)
		}
		got, raw, err := s.profile.MemoryAnswerRecord(answer)
		if err != nil {
			var lengthErr *civ.RecordLengthError
			if errors.As(err, &lengthErr) {
				return s.wrongRecordLength(lengthErr)
			}
			return fmt.Errorf("ic7100: Open: probe A-%03d: %w", channel, err)
		}
		if got != want {
			s.noteMismatch()
			continue
		}
		// The fingerprint is an OCCUPIED, DECODABLE 111-byte record, not
		// length alone. ASSUMED ic7100-all-ff-record is empty and therefore
		// cannot fingerprint; any other undecodable record is an unclassified
		// response and fails closed rather than being invented into a channel.
		if allFF(raw) {
			continue
		}
		if _, err := s.profile.ParseMemoryAnswer(answer); err != nil {
			return fmt.Errorf("ic7100: Open: probe A-%03d: record has the expected length but does not parse: %w", channel, err)
		}
		s.diag.Fingerprinted = true
		s.diag.Status = fmt.Sprintf("FINGERPRINTED %d B", civic7100.RecordLength)
		return nil
	}
	s.diag.Status = "UNFINGERPRINTED"
	return nil
}

// wrongRecordLength is probe identity classification, not ordinary read
// parsing. TestProbeRejectsWrongRecordLengthContinuously pins an
// unattributed WrongRadioError when Stage 2 has no sibling table; the
// synthetic attribution test pins the tier's explicitly provisional seam.
func (s *Session) wrongRecordLength(lengthErr *civ.RecordLengthError) error {
	want := fmt.Sprintf("record %d", civic7100.RecordLength)
	got := fmt.Sprintf("record %d", lengthErr.Got)
	wrong := &driver.WrongRadioError{Want: want, Got: got}
	if model, ok := s.siblingLengths[lengthErr.Got]; ok {
		wrong.WantModel = civic7100.Profile().Model()
		wrong.GotModel = model
		return fmt.Errorf("ic7100: Open: record-length fingerprint: %w — attribution is PROVISIONAL because the compared record lengths are ASSUMED derivations", wrong)
	}
	return fmt.Errorf("ic7100: Open: record-length fingerprint: %w", wrong)
}

// CIVDiagnostics records address-only versus length-fingerprinted opening
// and the undocumented 19 00 token without treating that token as identity.
type CIVDiagnostics struct {
	IDToken              string
	Fingerprinted        bool
	Status               string
	ProbeSlotsRead       int
	InitDrainCapExceeded bool
	AnswerMismatches     uint64
	Accumulator          civ.AccumulatorStats
}

// Session is one concurrent-safe connection to an IC-7100.
type Session struct {
	eng     *transport.Engine
	stats   civ.AccumulatorStatsReporter
	profile civ.Profile
	caps    spec.Capabilities
	id      driver.Identity
	// siblingLengths is an optional diagnostic table and never widens the
	// profile's accepted record-length set.
	siblingLengths SiblingLengths

	mu      sync.Mutex
	diag    CIVDiagnostics
	writeMu sync.Mutex
}

func (s *Session) Identity() driver.Identity       { return s.id }
func (s *Session) Capabilities() spec.Capabilities { return cloneCapabilities(s.caps) }
func (s *Session) Close() error                    { return s.eng.Close() }

func (s *Session) CIVDiagnostics() CIVDiagnostics {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.diag
	out.Accumulator = s.stats.AccumulatorStats()
	return out
}

func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.stats.AccumulatorStats().Unexpected
	if n < 0 {
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

func (s *Session) noteMismatch() {
	s.mu.Lock()
	s.diag.AnswerMismatches++
	s.mu.Unlock()
}

var (
	_ driver.Driver              = (*ic7100Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)
