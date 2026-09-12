// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7410 "github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// probeSlotCount bounds Open's occupied-slot search: the fingerprint's job
// is to confirm a record length, which one occupied channel settles.
const probeSlotCount = 10

// New builds the IC-7410 driver.
func New(opts ...Option) driver.Driver {
	d := &ic7410Driver{Base: driver.Base{Profile: RealHardware}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures a driver at construction.
type Option func(*ic7410Driver)

// WithConsentedUnverifiedWrites records the user's consent to writes this
// project has never verified against an IC-7410.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7410Driver) { d.Consented = true }
}

// WithSimulatedProfile selects the in-process simulated capability arm.
func WithSimulatedProfile() Option {
	return func(d *ic7410Driver) { d.Profile = Simulated }
}

// ic7410Driver implements driver.Driver for the Icom IC-7410.
type ic7410Driver struct {
	driver.Base
}

// Model implements driver.Driver.
func (d *ic7410Driver) Model() string { return "IC-7410" }

// Capabilities implements driver.Driver: the STATIC baseline, before any
// radio has been probed.
func (d *ic7410Driver) Capabilities() spec.Capabilities {
	var caps spec.Capabilities
	switch d.Profile {
	case Simulated:
		caps = capabilitiesSimulated()
	case RealHardware:
		caps = capabilitiesUnverified()
	default:
		caps = capabilitiesUnverified()
	}
	caps.Model = "IC-7410"
	return caps
}

// OpenReport is what Open observed while probing.
type OpenReport struct {
	// IDToken is what the radio answered to 19 00, recorded and NEVER
	// matched — the reply value is undocumented on this model (matrix
	// §3.12).
	IDToken []byte
	// SlotsTried is how many channels the occupied-slot search read.
	SlotsTried int
	// Fingerprinted is false when every probed slot was rejected.
	Fingerprinted bool
	// RecordLength is the record-only length the fingerprint confirmed.
	RecordLength int
	// InitDrainCapExceeded records the nonfatal half of R9-SPLIT.
	InitDrainCapExceeded bool
	WireAtOpen           civ.AccumulatorStats
}

// RecordLengthMismatchError reports that a memory answer carried a record
// at a length this profile does not declare (spec D3.2's continuous
// length fingerprint).
type RecordLengthMismatchError struct {
	Err  *civ.RecordLengthError
	Got  int
	Want int
	Slot civ.ChannelAddress
}

func (e *RecordLengthMismatchError) Error() string {
	return fmt.Sprintf(
		"ic7410: %s answered a %d-byte memory record, want %d — matrix §3.11 derives this expected length from the record diagram, and this refusal names no other model because cross-model record-length distinctness is a Wave-4 tier check",
		e.Slot, e.Got, e.Want)
}

func (e *RecordLengthMismatchError) Unwrap() []error {
	return []error{driver.ErrWrongRadio, e.Err}
}

// AnswerMismatchError reports tier ruling T2: a memory answer whose
// decoded channel address is not the one that was asked for.
type AnswerMismatchError = driver.AnswerMismatchError[civ.ChannelAddress]

// Open implements driver.Driver.
func (d *ic7410Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	framing, err := civ.NewFraming(civic7410.Profile())
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7410: framing: %w", err)
	}
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic7410: the CI-V framing does not report accumulator stats — this driver's diagnostics require civ.AccumulatorStatsReporter")
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7410: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

func (d *ic7410Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := civic7410.Profile()
	var report OpenReport

	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic7410: Open: %w", err)
		}
		report.InitDrainCapExceeded = true
	}

	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic7410: Open: building the 19 00 read: %w", err)
	}
	frame, err := eng.Do(ctx, idCmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), 1))
	if err != nil {
		return nil, fmt.Errorf("ic7410: Open: 19 00 identity probe: %w", err)
	}
	token, err := p.ParseTransceiverID(frame)
	if err != nil {
		return nil, fmt.Errorf("ic7410: Open: 19 00 identity probe: %w", err)
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

	report.WireAtOpen = stats.AccumulatorStats()

	return &Session{
		eng: eng, stats: stats, id: id,
		caps: d.SessionCaps(d.Capabilities()), report: report,
	}, nil
}

// probeSlot performs ONE 1A 00 read for the occupied-slot search.
func probeSlot(ctx context.Context, eng *transport.Engine, p civ.Profile, a civ.ChannelAddress) ([]byte, bool, error) {
	cmd, err := p.BuildMemoryRead(a)
	if err != nil {
		return nil, false, fmt.Errorf("ic7410: Open: building the 1A 00 read for %s: %w", a, err)
	}
	frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ic7410: Open: probing %s: %w", a, err)
	}
	got, raw, err := p.MemoryAnswerRecord(frame)
	if err != nil {
		var lengthErr *civ.RecordLengthError
		if errors.As(err, &lengthErr) {
			return nil, false, &RecordLengthMismatchError{Err: lengthErr, Got: lengthErr.Got, Want: civic7410.RecordOnlyLength, Slot: a}
		}
		return nil, false, fmt.Errorf("ic7410: Open: probing %s: %w", a, err)
	}
	if got != a {
		return nil, false, &AnswerMismatchError{Model: "ic7410", Requested: a, Answered: got}
	}
	return raw, false, nil
}

// Session is one open, probed connection to an IC-7410.
type Session struct {
	eng              *transport.Engine
	stats            civ.AccumulatorStatsReporter
	id               driver.Identity
	caps             spec.Capabilities
	report           OpenReport
	answerMismatches atomic.Uint64
}

func (s *Session) Identity() driver.Identity       { return s.id }
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }
func (s *Session) Close() error                    { return s.eng.Close() }

// Fingerprint reports the record-only length Open confirmed.
func (s *Session) Fingerprint() (recordLength int, confirmed bool) {
	return s.report.RecordLength, s.report.Fingerprinted
}

// OpenDiagnostics returns what Open observed while probing.
func (s *Session) OpenDiagnostics() OpenReport { return s.report }

// WireStats exposes the adapter's full counter set.
func (s *Session) WireStats() civ.AccumulatorStats { return s.stats.AccumulatorStats() }

// Diagnostics implements driver.DiagnosticsReporter.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{
		UnexpectedFrames: uint64(s.eng.UnexpectedFrames()) + uint64(s.stats.AccumulatorStats().Unexpected),
	}
}

// AnswerMismatches reports how many memory answers this session has seen
// whose decoded channel address was not the one requested (tier ruling
// T2).
func (s *Session) AnswerMismatches() uint64 { return s.answerMismatches.Load() }

// ReadChannel implements driver.Session; its body is in read.go.
// WriteChannel implements driver.Session; its body is in write.go.

var (
	_ driver.Driver                = (*ic7410Driver)(nil)
	_ driver.SerialFramingReporter = (*ic7410Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// StopBits reports one stop bit for the CI-V link (8-N-1), per spec D3.1.
//
// ASSUMED, ON NO EVIDENCE FROM THIS RADIO'S DOCUMENT — "stop bit", "start
// bit", "parity", "8 bit" and "handshake" have zero hits across all 124
// pages, about any port (matrix §3.1). Register entry:
// ic7410-framing-8n1.
func (d *ic7410Driver) StopBits() int { return 1 }
