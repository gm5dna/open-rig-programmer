// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9100 "github.com/gm5dna/open-rig-programmer/core/civ/ic9100"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

const (
	retryReads = 1

	// probeSlots is how many band-0 (HF/50 MHz) MEM channels the open-time
	// fingerprint probe reads before giving up on finding an occupied one,
	// argued the same way as core/driver/ic7100's identical bound: bounded
	// per spec D3.2, small, and confined to one band rather than the
	// full 297-channel walk caps.go declares.
	probeSlots = 8
)

// Option configures the sessions produced by New.
type Option func(*ic9100Driver)

// SiblingLengths maps a foreign record-only length to a model name. Empty
// in Stage 2: this matrix declares no registered sibling and tier
// integration owns any later cross-model attribution.
type SiblingLengths map[int]string

// WithConsentedUnverifiedWrites records user consent for this session only.
// The static capability set remains Unverified and FieldErase remains zero.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic9100Driver) { d.Consented = true }
}

// WithSiblingRecordLengths supplies a foreign-length attribution table for
// the probe's WrongRadioError diagnostics.
func WithSiblingRecordLengths(s SiblingLengths) Option {
	return func(d *ic9100Driver) { d.siblingLengths = s }
}

// New constructs the IC-9100 driver. It intentionally returns only the
// neutral driver seam and does not register the model.
func New(profile driver.Profile, opts ...Option) driver.Driver {
	d := &ic9100Driver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type ic9100Driver struct {
	driver.Base
	siblingLengths SiblingLengths
}

func (d *ic9100Driver) Model() string { return "IC-9100" }

func (d *ic9100Driver) Capabilities() spec.Capabilities {
	if d.Profile == Simulated {
		return CapabilitiesSimulated()
	}
	return CapabilitiesUnverified()
}

// Open takes ownership of port, sends no Init mutation, requires an
// address-matched 19 00 reply, and performs a bounded occupied-slot search.
func (d *ic9100Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	p := civic9100.Profile()
	framing, err := civ.NewFraming(p)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic9100: Open: %w", err)
	}
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic9100: Open: CI-V framing does not report accumulator statistics")
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic9100: Open: %w", err)
	}
	s, err := d.open(ctx, eng, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return s, nil
}

func (d *ic9100Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := civic9100.Profile()
	s := &Session{eng: eng, stats: stats, profile: p, caps: d.SessionCaps(d.Capabilities()), siblingLengths: d.siblingLengths}
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic9100: Open: %w", err)
		}
		s.diag.InitDrainCapExceeded = true
	}

	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic9100: Open: %w", err)
	}
	answer, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), retryReads))
	if err != nil {
		return nil, fmt.Errorf("ic9100: Open: no address-matched reply to 19 00: %w", err)
	}
	token, err := p.ParseTransceiverID(answer)
	if err != nil {
		return nil, fmt.Errorf("ic9100: Open: parse 19 00: %w", err)
	}
	s.diag.IDToken = token
	// HEADLINE FINDING (matrix §3.4): the static address is 7C, not 88.
	id.CATID = "7C"
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
		want := civ.ChannelAddress{Group: 0, Channel: channel}
		cmd, err := s.profile.BuildMemoryRead(want)
		if err != nil {
			return fmt.Errorf("ic9100: Open: probe HF-%03d: %w", channel, err)
		}
		answer, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), retryReads))
		s.diag.ProbeSlotsRead = channel
		if errors.Is(err, transport.ErrRejected) {
			continue
		}
		if err != nil {
			return fmt.Errorf("ic9100: Open: probe HF-%03d: %w", channel, err)
		}
		got, raw, err := s.profile.MemoryAnswerRecord(answer)
		if err != nil {
			var lengthErr *civ.RecordLengthError
			if errors.As(err, &lengthErr) {
				return s.wrongRecordLength(lengthErr)
			}
			return fmt.Errorf("ic9100: Open: probe HF-%03d: %w", channel, err)
		}
		if got != want {
			s.noteMismatch()
			continue
		}
		// ASSUMED ic9100-all-ff-record is empty and therefore cannot
		// fingerprint; any other undecodable record is an unclassified
		// response and fails closed rather than being invented into a
		// channel.
		if allFF(raw) {
			continue
		}
		if _, err := s.profile.ParseMemoryAnswer(answer); err != nil {
			return fmt.Errorf("ic9100: Open: probe HF-%03d: record has the expected length but does not parse: %w", channel, err)
		}
		s.diag.Fingerprinted = true
		s.diag.Status = fmt.Sprintf("FINGERPRINTED %d B", civic9100.RecordLength)
		return nil
	}
	s.diag.Status = "UNFINGERPRINTED"
	return nil
}

// wrongRecordLength is probe identity classification, not ordinary read
// parsing.
func (s *Session) wrongRecordLength(lengthErr *civ.RecordLengthError) error {
	want := fmt.Sprintf("record %d", civic9100.RecordLength)
	got := fmt.Sprintf("record %d", lengthErr.Got)
	wrong := &driver.WrongRadioError{Want: want, Got: got}
	if model, ok := s.siblingLengths[lengthErr.Got]; ok {
		wrong.WantModel = civic9100.Profile().Model()
		wrong.GotModel = model
		return fmt.Errorf("ic9100: Open: record-length fingerprint: %w — attribution is PROVISIONAL because the compared record lengths are ASSUMED derivations", wrong)
	}
	return fmt.Errorf("ic9100: Open: record-length fingerprint: %w", wrong)
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

// Session is one concurrent-safe connection to an IC-9100.
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
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }
func (s *Session) Close() error                    { return s.eng.Close() }

func (s *Session) CIVDiagnostics() CIVDiagnostics {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.diag
	out.Accumulator = s.stats.AccumulatorStats()
	return out
}

func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.stats.AccumulatorStats().Unexpected)}
}

func (s *Session) noteMismatch() {
	s.mu.Lock()
	s.diag.AnswerMismatches++
	s.mu.Unlock()
}

var (
	_ driver.Driver              = (*ic9100Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)
