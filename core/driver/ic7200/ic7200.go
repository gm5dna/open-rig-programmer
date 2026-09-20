// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7200 "github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// probeSlotCount is how many memory channels Open's occupied-slot search
// reads before giving up and opening UNFINGERPRINTED — the same bound and
// the same reasoning core/driver/ic7610/ic7610.go gives.
const probeSlotCount = 10

// New returns the IC-7200 driver built with profile.
//
// NO MODEL ENUM: this family has one member (matrix §4), so which radio a
// driver is for is fixed by the package rather than by a value a caller
// could get wrong.
func New(profile driver.Profile, opts ...Option) driver.Driver {
	d := &ic7200Driver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures a driver at construction.
type Option func(*ic7200Driver)

// WithConsentedUnverifiedWrites records the user's consent to writes this
// project has never verified against an IC-7200.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7200Driver) { d.Consented = true }
}

// ic7200Driver implements driver.Driver for the Icom IC-7200.
type ic7200Driver struct {
	driver.Base
}

// Model implements driver.Driver.
func (d *ic7200Driver) Model() string { return "IC-7200" }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile, before any radio has been probed.
func (d *ic7200Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select; a real-hardware session gets
		// the all-Unverified fail-safe.
		return CapabilitiesUnverified()
	default:
		return CapabilitiesUnverified()
	}
}

// OpenReport is what Open observed while probing.
type OpenReport struct {
	// IDToken is what the radio answered to 19 00, recorded and NEVER
	// matched (matrix §3.12(i), register entry, unresolved reply value).
	IDToken []byte
	// SlotsTried is how many channels the occupied-slot search read
	// before it stopped.
	SlotsTried int
	// Fingerprinted is false when every probed slot was rejected — an
	// empty radio, opened on address evidence alone.
	Fingerprinted bool
	// RecordLength is the record-only length the fingerprint confirmed,
	// or 0 when Fingerprinted is false.
	RecordLength int
}

// RecordLengthMismatchError reports that a memory answer carried a record
// at a length this profile does not declare. Fields are
// icom.RecordLengthMismatchError's shared shape
// (core/driver/internal/icom/errors.go); Unwrap is promoted from there
// unchanged, and only Error() is this package's own.
type RecordLengthMismatchError struct {
	icom.RecordLengthMismatchError
}

func (e *RecordLengthMismatchError) Error() string {
	return fmt.Sprintf(
		"ic7200: %s answered a %d-byte memory record, want %d — the expected length is itself an ASSUMED derivation from one document (matrix §3.11/§3.12(iii)), and this refusal names no other model because cross-model record-length distinctness is a tier-level check",
		e.Slot, e.Got, e.Want)
}

// AnswerMismatchError reports that a memory answer's decoded channel
// address was not the one that was asked for (tier ruling T2).
type AnswerMismatchError = driver.AnswerMismatchError[civ.ChannelAddress]

// Open implements driver.Driver.
func (d *ic7200Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	framing, err := civ.NewFraming(civic7200.Profile())
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7200: framing: %w", err)
	}
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic7200: the CI-V framing does not report accumulator stats — this driver's diagnostics require civ.AccumulatorStatsReporter")
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7200: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

func (d *ic7200Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := civic7200.Profile()
	var report OpenReport

	// Init is a drain alone on CI-V; the initial failure is nonfatal
	// (R9-SPLIT), matching every sibling Icom package in this tier.
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic7200: Open: %w", err)
		}
	}

	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic7200: Open: building the 19 00 read: %w", err)
	}
	frame, err := eng.Do(ctx, idCmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), 1))
	if err != nil {
		return nil, fmt.Errorf("ic7200: Open: 19 00 identity probe: %w", err)
	}
	token, err := p.ParseTransceiverID(frame)
	if err != nil {
		return nil, fmt.Errorf("ic7200: Open: 19 00 identity probe: %w", err)
	}
	if raw, derr := hex.DecodeString(token); derr == nil {
		report.IDToken = raw
	}
	id.CATID = fmt.Sprintf("%02x%s", p.RadioAddress(), token)

	for ch := 1; ch <= probeSlotCount; ch++ {
		report.SlotsTried = ch
		raw, empty, err := probeSlot(ctx, eng, p, civ.ChannelAddress{Channel: ch})
		if err != nil {
			return nil, err
		}
		if empty {
			continue
		}
		report.Fingerprinted = true
		report.RecordLength = len(raw)
		break
	}
	// An empty radio opens anyway, on address evidence alone.

	return &Session{
		eng:    eng,
		stats:  stats,
		id:     id,
		caps:   d.SessionCaps(d.Capabilities()),
		report: report,
	}, nil
}

func probeSlot(ctx context.Context, eng *transport.Engine, p civ.Profile, a civ.ChannelAddress) ([]byte, bool, error) {
	cmd, err := p.BuildMemoryRead(a)
	if err != nil {
		return nil, false, fmt.Errorf("ic7200: Open: building the 1A 00 read for %s: %w", a, err)
	}
	frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ic7200: Open: probing %s: %w", a, err)
	}
	got, raw, err := p.MemoryAnswerRecord(frame)
	if err != nil {
		var lengthErr *civ.RecordLengthError
		if errors.As(err, &lengthErr) {
			return nil, false, &RecordLengthMismatchError{icom.RecordLengthMismatchError{Err: lengthErr, Got: lengthErr.Got, Want: civic7200.RecordOnlyLength, Slot: a}}
		}
		return nil, false, fmt.Errorf("ic7200: Open: probing %s: %w", a, err)
	}
	if got != a {
		return nil, false, &AnswerMismatchError{Model: "ic7200", Requested: a, Answered: got}
	}
	return raw, false, nil
}

// Session is one open, probed connection to an IC-7200.
type Session struct {
	eng              *transport.Engine
	stats            civ.AccumulatorStatsReporter
	id               driver.Identity
	caps             spec.Capabilities
	report           OpenReport
	answerMismatches atomic.Uint64
	// writeMu serialises WriteChannel's read-then-set pair into one
	// logical operation; the engine alone only serialises individual
	// exchanges.
	writeMu sync.Mutex
}

func (s *Session) Identity() driver.Identity { return s.id }

func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

func (s *Session) Close() error { return s.eng.Close() }

// Fingerprint reports the record-only length Open confirmed, and whether
// it confirmed one at all.
func (s *Session) Fingerprint() (recordLength int, confirmed bool) {
	return s.report.RecordLength, s.report.Fingerprinted
}

// OpenDiagnostics returns what Open observed while probing.
func (s *Session) OpenDiagnostics() OpenReport { return s.report }

// WireStats exposes the adapter's full counter set.
func (s *Session) WireStats() civ.AccumulatorStats { return s.stats.AccumulatorStats() }

func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{
		UnexpectedFrames: uint64(s.eng.UnexpectedFrames()) + uint64(s.stats.AccumulatorStats().Unexpected),
	}
}

// AnswerMismatches reports how many memory answers this session has seen
// whose decoded channel address was not the one requested (tier ruling
// T2).
func (s *Session) AnswerMismatches() uint64 { return s.answerMismatches.Load() }

var (
	_ driver.Driver                = (*ic7200Driver)(nil)
	_ driver.SerialFramingReporter = (*ic7200Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// StopBits reports one stop bit for the CI-V link (8-N-1).
//
// ASSUMED, ON NO EVIDENCE FROM THIS RADIO'S DOCUMENT (matrix §3.1): the
// words "stop bit", "data bit", "parity" and "8 bit" appear nowhere in
// the manual, about any port. Register home: shared D5 entry 8.
func (d *ic7200Driver) StopBits() int { return 1 }
