// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	ic7300mk2civ "github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Compile-time proof of the optional seams this package fills.
//
// THE FRAMING REPORTER IS ON THE DRIVER, UNCONDITIONALLY. internal/wiring
// holds the driver value BEFORE the port exists, which is the one moment
// "how many stop bits?" can be asked at; a session-side reporter could only
// ever be consulted after the framing had been guessed.
var (
	_ driver.Driver                = (*ic7300mk2Driver)(nil)
	_ driver.SerialFramingReporter = (*ic7300mk2Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// probeSlots is how many MEM channels the open probe reads before giving up
// on finding an occupied one.
//
// BOUNDED, per spec D3.2, and SMALL: the probe exists to fingerprint the
// record length, not to inventory the radio, and a radio whose first eight
// memories are empty is opened UNFINGERPRINTED rather than searched to 99.
//
// CONFINED TO MEM, and that is the load-bearing half. A P1/P2 record's shape
// is itself ASSUMED (`ic7300mk2-scan-edge-record-layout`, lift MK2-R10,
// capture `ic7300mk2-scan-edge-p1-read`): this document NEVER STATES whether
// a scan-edge record carries the same 45 bytes (§3.16 A5), so a short answer
// — or an FA — from 1A 00 01 00 would be a fact about the scan-edge bank
// rather than a fault, and the probe must not learn the length fingerprint
// from a record whose layout is not established.
const probeSlots = 8

// foreignRecordLengths attributes a record length this model does not
// declare to the sibling that does.
//
// ONE ENTRY, and it is a HINT rather than a distinctness claim (plan
// decision D10). BOTH lengths — this model's 45 and the sibling's 39 — are
// ASSUMED derivations from printed field
// widths — neither document prints a record total — which is why the error
// text carries the word *provisional* and names both numbers. Cross-model
// record-length distinctness is a TIER-level check belonging to
// registration, and it is what may add or correct entries here. DO NOT ADD A
// SECOND ENTRY from this package.
var foreignRecordLengths = map[int]string{
	39: "IC-7300 (provisional)",
}

// WithTransportLogger hands the engine a logger for wire tracing.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ic7300mk2Driver) { d.transportLogger = l }
}

// StopBits is how many stop bits this radio's CI-V port expects.
//
// ONE, ASSUMED at spec D5 entry 8, lift MK2-R5. THE HAZARD, stated wherever
// the claim is: this document prints NO character format anywhere (matrix
// §3.1) — it is a CI-V reference guide, and it says nothing about the serial
// line at all — and an Icom manual's "8 bit / 1 stop" line about the
// DATA/RTTY port is NOT evidence about CI-V; they are different ports with
// different jobs. internal/wiring refuses any value other than 1 or 2, so
// this is a statement rather than a hint.
func (d *ic7300mk2Driver) StopBits() int { return 1 }

// Open builds the engine and hands it to open, closing it on any failure.
//
// TWO FUNCTIONS, on core/driver/ftdx101's shape: Open owns the resources and
// the cleanup, open is the body and may return an error from anywhere
// without leaking a port. Open takes ownership of port on BOTH outcomes.
func (d *ic7300mk2Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	p := ic7300mk2civ.Profile()

	// E1's constructor. It REFUSES an unconfigured profile, which a plain
	// interface nil-check could not see: the Framing built from a zero
	// Profile is a perfectly non-nil value carrying a perfectly non-nil
	// Allow method, and the engine would come up gating for no radio.
	fr, err := civ.NewFraming(p)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7300mk2: Open: %w", err)
	}

	// THE TWO-RESULT ASSERTION, and it must stay two-result (D22). The
	// framing is an INTERFACE value; a one-result assertion would panic on
	// a future adapter that dropped the optional stats interface, which is
	// a diagnostics feature killing a session that was otherwise working.
	statser, ok := fr.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic7300mk2: Open: the CI-V framing adapter does not report accumulator statistics — this driver's diagnostics come from the ADAPTER's counters, never from the engine's, because the accumulator has already swallowed every broadcast before the engine could count one")
	}

	var engOpts []transport.Option
	if d.transportLogger != nil {
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	// transport.NewEngineWith is GUARDED — internal/guards'
	// TestNewEngineReachableOnlyFromDriver covers both constructors by
	// name — so this call appears HERE and nowhere else in the package.
	// Nothing local is built: no adapter, no matcher, no DrainPolicy
	// constant copied from the CAT side.
	eng, err := transport.NewEngineWith(port, fr, engOpts...)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7300mk2: Open: %w", err)
	}

	sess, err := d.open(ctx, eng, fr, statser, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// probeReport is what the open probe learned, kept beside the live
// accumulator counters rather than mixed into them: these three facts are
// fixed at open and never move again.
type probeReport struct {
	// Fingerprinted records that a record of a length THIS profile declares
	// was actually read. False means the radio answered FA to every probe
	// slot, so the session was opened on ADDRESS EVIDENCE ALONE.
	Fingerprinted bool
	// ProbeSlotsRead is how many MEM channels the search read.
	ProbeSlotsRead int
	// InitDrainCapExceeded records that the line never went quiet at open.
	// NONFATAL — spec D2's bounded drain "cannot fail the open", and
	// transceive is factory-ON with no off-switch shipped in this tier — but
	// not unrecorded.
	InitDrainCapExceeded bool
}

// open runs the whole probe against an engine that already exists.
func (d *ic7300mk2Driver) open(ctx context.Context, eng *transport.Engine, fr transport.Framing, statser civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := ic7300mk2civ.Profile()
	var probe probeReport

	// THE CI-V INIT SEQUENCE IS EMPTY, so this writes NOTHING: it is a
	// bounded wait for the line to go quiet and nothing else. NO
	// transceive-off, no clear, no 1A 05 — broadcasts are excluded
	// STRUCTURALLY by the accumulator's address filter, never by changing
	// somebody's radio settings.
	//
	// AN INITIAL ErrDrainCapExceeded IS NONFATAL AND DIAGNOSED. Every LATER
	// drain stays fail-closed exactly as the engine has it; the split is by
	// WHEN, and it lives here in the driver because the engine cannot know
	// which drain is the first one.
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic7300mk2: Open: %w", err)
		}
		probe.InitDrainCapExceeded = true
	}

	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic7300mk2: Open: %w", err)
	}
	// retryReads is ONE — a behavioural parameter stated here rather than
	// left to the signature. Retrying a read is safe: it is idempotent, and
	// a CI-V read changes nothing.
	frame, err := eng.Do(ctx, idCmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), 1))
	if err != nil {
		return nil, fmt.Errorf("ic7300mk2: Open: 19 00 identity read: %w", err)
	}
	// THE TOKEN IS RECORDED AND NEVER MATCHED (D5 entry 7,
	// `ic7300mk2-identity-token`, lift MK2-R4). The reply value is undocumented on every model in
	// this tier, so what identifies the radio at this step is that an
	// ADDRESS-MATCHED reply arrived at all — a property of the frame's `to`
	// and `from` bytes, which the matcher and the parser both check.
	token, err := p.ParseTransceiverID(frame)
	if err != nil {
		return nil, fmt.Errorf("ic7300mk2: Open: 19 00 identity read: %w", err)
	}
	id.CATID = fmt.Sprintf("%02x:%s", p.RadioAddress(), token)

	if err := d.probeForFingerprint(ctx, eng, p, id, &probe); err != nil {
		return nil, err
	}

	return &Session{
		eng:     eng,
		p:       p,
		fr:      fr,
		statser: statser,
		id:      id,
		caps:    d.sessionCapabilities(),
		probe:   probe,
	}, nil
}

// probeForFingerprint reads MEM channels 1..probeSlots in order, stopping at
// the first record.
//
// THE FINGERPRINT IS THE LENGTH, and civ.MemoryAnswerRecord is what applies
// it: a record at a length this profile does not declare comes back as a
// *civ.RecordLengthError, whatever the record contains. It is also why
// nothing here caches "fingerprinted" as a permission: every LATER record
// read re-validates the length through the same call, so the property is
// CONTINUOUS rather than one-shot, and this flag is a diagnostic only.
func (d *ic7300mk2Driver) probeForFingerprint(ctx context.Context, eng *transport.Engine, p civ.Profile, id driver.Identity, probe *probeReport) error {
	for n := 1; n <= probeSlots; n++ {
		want := civ.ChannelAddress{Channel: n}
		cmd, err := p.BuildMemoryRead(want)
		if err != nil {
			return fmt.Errorf("ic7300mk2: Open: probe of channel %d: %w", n, err)
		}
		frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1))
		probe.ProbeSlotsRead = n

		// AN FA IS NOT AN ERROR: it is an empty channel (D5 entry 2(a),
		// ASSUMED, lift MK2-R2). Engine.Do CONSUMES the FA and
		// returns ErrRejected with no frame, so this branch keys on the
		// ERROR and never on "an FA frame arrived" (ruling T4).
		if errors.Is(err, transport.ErrRejected) {
			continue
		}
		if err != nil {
			return fmt.Errorf("ic7300mk2: Open: probe of channel %d: %w", n, err)
		}

		got, _, rerr := p.MemoryAnswerRecord(frame)
		if rerr != nil {
			var lenErr *civ.RecordLengthError
			if errors.As(rerr, &lenErr) {
				return wrongRecordLength(p, lenErr, id.CATID)
			}
			return fmt.Errorf("ic7300mk2: Open: probe of channel %d: %w", n, rerr)
		}
		// THE ADDRESS CHECK PRECEDES EVERY USE OF THE ANSWER (ruling T2,
		// plan decision D20). civ's MemoryAnswerMatcher is ENVELOPE-ONLY by
		// design — it matches to/from/cn/sc and deliberately does not look
		// at the channel — so the address is the DRIVER's to check, here and
		// in every other read path.
		if got != want {
			return &AnswerMismatchError{Requested: want.String(), Answered: got.String()}
		}
		probe.Fingerprinted = true
		return nil
	}
	// EVERY SLOT ANSWERED FA. The session opens UNFINGERPRINTED, on address
	// evidence alone, with the fact recorded: a radio whose first eight
	// memories are empty is an ordinary radio, and refusing to open one
	// would be refusing the commonest state a new radio is in.
	return nil
}

// wrongRecordLength renders a length the profile does not accept as either
// an attributed wrong-radio refusal or an unattributed one.
//
// THE ATTRIBUTION IS PROVISIONAL AND SAYS SO. Both numbers in the message —
// the observed length and this model's — are ASSUMED derivations from
// printed field widths, so the text names both and uses the word
// *provisional*; a reader must be able to tell a fingerprint from a
// certainty. A length in no table claims NO model at all, because guessing
// one from a number nobody has seen would be worse than saying nothing.
func wrongRecordLength(p civ.Profile, e *civ.RecordLengthError, observedID string) error {
	if model, ok := foreignRecordLengths[e.Got]; ok {
		wrong := &driver.WrongRadioError{
			// The ADDRESS HEX, which is what CATID means in this tier.
			Want: fmt.Sprintf("%02x", p.RadioAddress()),
			// The identity string this probe actually observed. An empty
			// string here would render as (CAT ID "") in the seam's own
			// error text.
			Got:       observedID,
			WantModel: p.Model(),
			GotModel:  model,
		}
		return fmt.Errorf("ic7300mk2: Open: the radio answered a %d-byte memory record where the %s's is %d — both lengths are ASSUMED derivations from printed field widths (neither document prints a record total), so the attribution is provisional: %w",
			e.Got, p.Model(), p.BuildRecordLength(), wrong)
	}
	return fmt.Errorf("ic7300mk2: Open: the radio answered a %d-byte memory record, which the %s does not declare and which matches no registered sibling's length — NO model is claimed for it: %w",
		e.Got, p.Model(), e)
}

// sessionCapabilities is the driver's static baseline plus the user's
// consent, and consent is applied ONLY to a profile this driver recognises:
// a forged Profile value must not pick up a consented capability set on the
// way past.
func (d *ic7300mk2Driver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// Session is one open, probed connection to an IC-7300MK2.
type Session struct {
	eng *transport.Engine
	p   civ.Profile

	// fr is the framing value this session's engine was built from, RETAINED
	// because it is the only handle to the adapter's counters (D22). The
	// engine does not surface them, and it must not: the accumulator has
	// already dropped every transceive broadcast before the engine could
	// count one, so Engine.UnexpectedFrames would report a healthy zero on a
	// line saturated with transceive.
	fr transport.Framing
	// statser is fr, asserted once at Open so no read path has to.
	statser civ.AccumulatorStatsReporter

	id    driver.Identity
	caps  spec.Capabilities // effective; never mutated after Open
	probe probeReport

	// writeMu serialises the write path's READ-AND-SET pair (plan decision
	// D15). transport.Engine.Do locks ONE exchange, not a read-modify-write
	// SEQUENCE, and driver.Session's contract does not promise
	// single-threaded use — so without this the record the E6 check
	// inspected need not be the record the set is built against.
	writeMu sync.Mutex

	// mismatches counts answers whose channel address was not the one
	// requested. A DIAGNOSTIC beside the refusal, never instead of it.
	mismatchMu sync.Mutex
	mismatches uint64
}

// Identity returns who this session is talking to: the CI-V address hex,
// a colon, and the 19 00 token this probe observed and matched nothing
// against.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities returns this session's EFFECTIVE capabilities as a defensive
// copy — including the tone RANGE, which is a pointer and would otherwise be
// shared with every caller.
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// CIVDiagnostics is this driver's MODEL-SPECIFIC diagnostics surface.
//
// It exists because the neutral driver.SessionDiagnostics carries exactly
// ONE field, UnexpectedFrames, which cannot hold the probe's fingerprint
// note, the broadcast counts, the echo count or the drain outcome. The
// neutral shape is not widened for one tier's counters; this type carries
// them, and a caller that wants them names this package deliberately.
type CIVDiagnostics struct {
	// The adapter's own counters, from THIS side of the address filter.
	Frames           int
	Echoes           int
	Unexpected       int
	Rejections       int
	Acknowledgements int
	NoiseBytes       int
	Truncated        int

	// What the open probe learned.
	Fingerprinted        bool
	ProbeSlotsRead       int
	InitDrainCapExceeded bool

	// AnswerMismatches counts memory answers whose decoded channel address
	// was not the one requested (D20).
	AnswerMismatches uint64
}

// CIVDiagnostics returns a point-in-time snapshot: the adapter's live
// counters merged with the facts the probe fixed at open.
//
// THE ENGINE'S OWN COUNTER IS DELIBERATELY ABSENT. Engine.UnexpectedFrames
// answers a different question — how many frames did the ENGINE see that did
// not match the spec in force — and on a CI-V bus it is systematically the
// wrong one, because the accumulator filters every broadcast and every other
// station's traffic before an engine event exists. Reading it here would
// report zero on a saturated line.
func (s *Session) CIVDiagnostics() CIVDiagnostics {
	st := s.statser.AccumulatorStats()
	s.mismatchMu.Lock()
	mismatches := s.mismatches
	s.mismatchMu.Unlock()
	return CIVDiagnostics{
		Frames:               st.Frames,
		Echoes:               st.Echoes,
		Unexpected:           st.Unexpected,
		Rejections:           st.Rejections,
		Acknowledgements:     st.Acknowledgements,
		NoiseBytes:           st.NoiseBytes,
		Truncated:            st.Truncated,
		Fingerprinted:        s.probe.Fingerprinted,
		ProbeSlotsRead:       s.probe.ProbeSlotsRead,
		InitDrainCapExceeded: s.probe.InitDrainCapExceeded,
		AnswerMismatches:     mismatches,
	}
}

// Diagnostics implements the neutral driver.DiagnosticsReporter with the ONE
// field it carries, taken from the ADAPTER's Unexpected count. Everything
// else this driver knows is on CIVDiagnostics.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.statser.AccumulatorStats().Unexpected
	if n < 0 {
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// noteAnswerMismatch records one D20 refusal for the diagnostics surface.
func (s *Session) noteAnswerMismatch() {
	s.mismatchMu.Lock()
	s.mismatches++
	s.mismatchMu.Unlock()
}

// Close releases the session and its port. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

// ErrAnswerMismatch is the sentinel for a memory answer that names a
// different channel than the one requested.
//
// IT IS THIS DRIVER'S OWN, minted here rather than imported from
// core/driver/ftdx101, whose ErrAnswerMismatch is the precedent for the
// shape. No driver package imports another: a shared sentinel would make one
// radio's diagnostics another's.
var ErrAnswerMismatch = errors.New("ic7300mk2: answer names a different channel than was requested")

// AnswerMismatchError reports that a 1A 00 answer's decoded channel address
// was not the one asked for.
//
// THE CHECK IS THE DRIVER'S BECAUSE NOTHING BELOW IT MAKES ONE.
// civ.Profile.MemoryAnswerMatcher is envelope-only by design — it matches
// to/from/cn/sc and not the channel — so if the driver did not compare, an
// answer for the wrong slot would be mapped onto the requested one and a
// codeplug would be corrupted silently. It is checked BEFORE the empty
// recognition, the template check, the record mapping and the write merge
// alike.
type AnswerMismatchError struct {
	// Requested is the channel address the read asked for.
	Requested string
	// Answered is the channel address the answer carried.
	Answered string
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ic7300mk2: requested channel %s but the answer names %s — refusing to map a reply onto the wrong slot", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }
