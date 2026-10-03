// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// New returns the IC-7760 driver built with profile.
//
// NO MODEL ENUM: this family has one member (matrix §4), so which radio a
// driver is for is fixed by the package rather than by a value a caller
// could get wrong.
func New(profile driver.Profile, opts ...Option) driver.Driver {
	d := &ic7760Driver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures a driver at construction.
type Option func(*ic7760Driver)

// WithConsentedUnverifiedWrites records the user's consent to writes this
// project has never verified against an IC-7760.
//
// It changes the SESSION's effective capabilities and never the driver's
// static ones: internal/wiring reads the static set to decide whether to
// ASK for consent, and a driver whose static set already claimed consent
// would never be asked about. FieldErase is structurally out of reach of
// the transform (spec D4), and an unrecognised Profile is not transformed
// at all, so no value a caller can pass produces a writable session.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7760Driver) { d.Consented = true }
}

// ic7760Driver implements driver.Driver for the Icom IC-7760.
type ic7760Driver struct {
	driver.Base
}

// Model implements driver.Driver. It must equal Capabilities().Model and
// the Wave-4 registry key.
func (d *ic7760Driver) Model() string { return "IC-7760" }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile, before any radio has been probed.
func (d *ic7760Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe. The constant is not READ here,
		// deliberately: see its doc comment for what a flip must change,
		// and why a constant that was load-bearing on its own would let a
		// one-character edit unlock a write.
		return capabilitiesUnverified()
	default:
		// Any unrecognised Profile fails safe through its OWN explicit
		// arm rather than by sharing RealHardware's. The two return the
		// same value today, and a reader must be able to see that the
		// fail-safe is a decision rather than a coincidence of the flip's
		// state.
		return capabilitiesUnverified()
	}
}

// OpenReport is what Open observed while probing, kept on the ic7760
// Session rather than on the neutral seam: driver.SessionDiagnostics
// carries ONE aggregate counter (core/driver/optional.go) and cannot carry
// any of this, and widening the neutral seam is a tier-shared change five
// worktrees would want.
// The shared form (icom.OpenReport) has no per-package behaviour, so
// this package needs no type of its own.
type OpenReport = icom.OpenReport

// RecordLengthMismatchError reports that a memory answer carried a record
// at a length this profile does not declare — which is the probe's
// CONTINUOUS length fingerprint failing (spec D3.2).
//
// IT NAMES NO FOUND MODEL, and driver.WrongRadioError is deliberately not
// used for that reason: that type's whole shape is a pair of CAT IDs, and
// filling it with lengths would put a made-up identity in a field callers
// render as one. Cross-model record-length distinctness is a TIER-LEVEL
// Wave-4 check and this package holds no table of other radios' lengths,
// so the honest refusal says what was measured, what was expected, and
// that the expectation is itself ASSUMED.
// Fields are icom.RecordLengthMismatchError's shared shape
// (core/driver/internal/icom/errors.go); Unwrap is promoted from there
// unchanged, and only Error() is this package's own.
type RecordLengthMismatchError struct {
	icom.RecordLengthMismatchError
}

func (e *RecordLengthMismatchError) Error() string {
	return fmt.Sprintf(
		"ic7760: %s answered a %d-byte memory record, want %d — the expected length is itself an ASSUMED derivation from one document (D5 entry 6, register entry ic7760-record-length), and this refusal names no other model because cross-model record-length distinctness is a Wave-4 tier check",
		e.Slot, e.Got, e.Want)
}

// ErrAnswerMismatch is the sentinel for tier ruling T2: a memory answer
// whose decoded channel address is not the one that was asked for.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports T2's failure, naming both channels; the
// shared form (driver.AnswerMismatchError) carries the model name so this
// package needs no typed error of its own.
//
// IT EXISTS BECAUSE THE MATCHER CANNOT CATCH THIS. The landed
// civ.Profile.MemoryAnswerMatcher is deliberately ENVELOPE-ONLY — it
// checks `to`, `from`, `cn` and `sc` and nothing else — so an answer for
// channel 7 satisfies the spec for a read of channel 3 perfectly well.
// The address inside the answer is therefore the DRIVER's to check, and it
// is checked BEFORE ANY USE of the answer: before empty recognition,
// before any caching, before record mapping, before the E6 template check
// and before a write merge. A record silently mis-attributed to the wrong
// channel is the corruption this whole project refuses.
type AnswerMismatchError = driver.AnswerMismatchError[civ.ChannelAddress]

// Open implements driver.Driver: the shared Open choreography in
// core/driver/internal/icom, wrapped in this package's Session. It takes
// ownership of port on BOTH outcomes.
func (d *ic7760Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	o, err := icom.Open(ctx, &params, port, id)
	if err != nil {
		return nil, err
	}
	return &Session{
		eng:    o.Eng,
		stats:  o.Stats,
		id:     o.ID,
		caps:   d.SessionCaps(d.Capabilities()),
		report: o.Report,
	}, nil
}

// Session is one open, probed connection to an IC-7760.
type Session struct {
	eng *transport.Engine
	// stats is the framing value Open passed to transport.NewEngineWith,
	// RETAINED for the life of the session — the DIAGNOSTICS CARRIER
	// ruling. Reaching for Engine.UnexpectedFrames instead is forbidden:
	// it would report a healthy zero on a line saturated with transceive.
	stats  civ.AccumulatorStatsReporter
	id     driver.Identity
	caps   spec.Capabilities
	report OpenReport
	// answerMismatches counts memory answers whose decoded channel address
	// was not the one requested (tier ruling T2).
	//
	// ATOMIC because it is the one piece of MUTABLE state on a Session,
	// and driver.Session's contract says implementations must be safe for
	// concurrent use. Every other field here is written once by Open and
	// only read afterwards; the transport engine serialises the exchanges
	// themselves, but nothing serialises a caller reading the diagnostic
	// while another goroutine drives a read.
	answerMismatches atomic.Uint64
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: this session's EFFECTIVE set, as
// a deep copy per call.
//
// THE COPY IS LOAD-BEARING. WriteChannel re-checks against s.caps, the
// session's own value; a caller that could reach into what Capabilities
// handed out and flip a FieldSupport would otherwise be editing the write
// gate from outside it.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Close implements driver.Session. Idempotent, because Engine.Close is.
func (s *Session) Close() error { return s.eng.Close() }

// Fingerprint reports the record-only length Open confirmed, and whether
// it confirmed one at all.
//
// AN IC7610-PACKAGE ACCESSOR, NOT A NEUTRAL-SEAM ADDITION.
// driver.SessionDiagnostics carries only UnexpectedFrames, and widening
// the neutral seam to carry a per-tier fingerprint is a tier-shared change
// five worktrees would want a say in. A caller that needs this reaches for
// the concrete session, exactly as it would for any model-specific fact.
func (s *Session) Fingerprint() (recordLength int, confirmed bool) {
	return s.report.RecordLength, s.report.Fingerprinted
}

// OpenDiagnostics returns what Open observed while probing. See
// Fingerprint for why this is a package accessor.
func (s *Session) OpenDiagnostics() OpenReport { return s.report }

// WireStats exposes the adapter's full counter set, which the neutral
// SessionDiagnostics has no room for: echoes, noise bytes, truncations.
func (s *Session) WireStats() civ.AccumulatorStats { return s.stats.AccumulatorStats() }

// Diagnostics implements driver.DiagnosticsReporter by SUMMING the two
// sides of the filter: the engine's own unmatched-frame counter and the
// adapter's Unexpected count (the broadcasts and other stations' traffic
// the accumulator dropped before the engine could see them).
//
// NEITHER NUMBER ALONE IS THE TRUTH ABOUT THIS WIRE. The engine's counter
// answers "how many frames did the engine see that did not match the spec
// in force?", and on a CI-V bus the accumulator has already swallowed
// every transceive broadcast before the engine could count one.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{
		UnexpectedFrames: uint64(s.eng.UnexpectedFrames()) + uint64(s.stats.AccumulatorStats().Unexpected),
	}
}

// ReadChannel implements driver.Session; its body is in read.go, beside
// the slot map and the one read primitive it is made of.

// WriteChannel implements driver.Session; its body is in write.go,
// alongside the T5-ordered refusal ladder it is made of.

// Compile-time proof that this package really does implement the neutral
// seams it claims.
var (
	_ driver.Driver                = (*ic7760Driver)(nil)
	_ driver.SerialFramingReporter = (*ic7760Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// StopBits reports one stop bit for the CI-V link (8-N-1), per spec D3.1.
//
// ASSUMED, ON NO EVIDENCE FROM THIS RADIO'S DOCUMENT. The IC-7760 CI-V
// Reference Guide says nothing about serial framing anywhere: the words
// "stop bit", "data bit", "parity" and "8 bit" appear in none of its 28
// PDF pages, about any port (matrix §3.1, whose absence sweep and mandatory
// DATA/RTTY hazard sentence doc.go reproduces).
//
// WHERE THE 1 COMES FROM: the tier convention, additions design D5 entry 8
// ("serial framing 8-N-1"), which grades this model A — assumed. The
// IC-7760's own home for that assumption, and the only place a capture can
// discharge it, is the matrix register entry ic7760-serial-framing.
//
// The exact assumption and lift are recorded in package doc.go under
// ic7760-serial-framing. With an IC-7760 at its factory CI-V settings,
// open its USB (B) CI-V endpoint at 8-N-1 and then at 8-N-2, send
// FE FE B2 E0 19 00 FD at each, and record which framing
// returns a well-formed address-matched frame and which returns nothing or
// garbage. SCOPE: that capture settles which framing THAT radio's USB CI-V
// endpoint accepts, and nothing wider — not the [REMOTE] jack, not the
// [LAN] port, and not any other model.
//
// IT IS ON THE DRIVER, NOT THE SESSION, and enabler E2 records why that is
// forced: internal/wiring holds the driver value BEFORE the port is
// opened, and the stop bits are chosen at open. A session-side reporter
// could only be consulted after the framing had already been guessed.
//
// PROFILE-INDEPENDENT, and deliberately so: which capability set a caller
// asked for says nothing about how the radio frames a byte on the wire. An
// unrecognised Profile reports 1 like the rest.
//
// MATERIALITY: transport.DefaultStopBits is 2, so a driver that did NOT
// implement this interface would have its port opened at 8-N-2 — the
// silent divergence from the tier's assumed 8-N-1 that spec D3.1 exists to
// prevent. internal/wiring consults this and REFUSES any value but 1 or 2
// rather than substituting a default, so a zero could never quietly become
// 8-N-2 either.
func (d *ic7760Driver) StopBits() int { return 1 }
