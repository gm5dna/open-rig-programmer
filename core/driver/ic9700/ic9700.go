// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	// ALIASED deliberately: the DIALECT package's own name is also
	// "ic9700", and so is this one. civic9700 reads as "the core/civ side
	// of the IC-9700", which is exactly what it is, and it keeps the bare
	// spelling meaning this package throughout.
	civic9700 "github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// retryReads is how many ADDITIONAL attempts a CI-V read gets after a
// timeout.
//
// ONE, and it is a read-only decision. Retrying a read is safe (transport
// safety obligation 2: a `1A 00` read of a memory channel is idempotent
// and changes nothing), and one extra attempt covers the single dropped
// frame a marginal cable produces without turning a genuinely silent radio
// into a long wait. It is emphatically NOT extended to writes: the
// acknowledged memory set is built by civ.CIVWriteWithAckSpec, whose
// RetryReads is zero and which Engine.Do refuses to run with any other
// value.
const retryReads = 1

// probeSearchChannels is how many channels of the first band the probe
// reads looking for an occupied one.
//
// EIGHT IS A CHOICE, recorded as one. Spec D3.2 requires a bounded search
// and leaves the bound to the driver. Eight is enough to find a record on
// any radio somebody has actually used (channel 1 is where a front-panel
// store lands) and small enough that a completely empty radio costs eight
// rejections rather than ninety-nine. A radio where all eight are empty
// opens UNFINGERPRINTED — see CIVDiagnostics.Fingerprinted — rather than
// failing, because an empty radio is an ordinary radio.
const probeSearchChannels = 8

// probeSearchBand is the band the search walks: 1, the 144 MHz band, under
// E4's wire-index Group. One band rather than three, for the same reason
// the count is eight: the fingerprint is a LENGTH check, and the record
// length does not vary by band.
const probeSearchBand = 1

// CIVDiagnostics is the IC-9700's own diagnostics surface.
//
// IT SITS ALONGSIDE driver.DiagnosticsReporter RATHER THAN REPLACING IT,
// because the neutral one cannot hold any of this. Its landed return type
// carries a single field — SessionDiagnostics{UnexpectedFrames} — and on a
// CI-V line that counter is the WRONG question besides: the accumulator
// swallows every transceive broadcast before the engine can count one, so
// Engine.UnexpectedFrames reports a healthy zero on a saturated bus. The
// numbers worth having come from THIS side of the filter, which is why the
// session keeps the transport.Framing value it handed to NewEngineWith and
// asks it (civ.AccumulatorStatsReporter).
type CIVDiagnostics struct {
	// IDToken is the data of the `19 00` answer, recorded and NEVER
	// matched (spec D5 entry 7, lift R6). Empty when no answer was
	// parsed.
	IDToken []byte
	// Fingerprinted reports whether the bounded occupied-slot search
	// found a record whose length confirmed this profile. False means the
	// session opened on ADDRESS evidence alone, which is what an empty
	// radio gives (D5 entry 2(a)).
	Fingerprinted bool
	// InitDrainCapExceeded records that Engine.Init hit its absolute
	// drain cap and the driver continued anyway — the R9 nonfatal path.
	// Only the INITIAL drain is forgiven; every later one fails closed.
	InitDrainCapExceeded bool
	// AnswerMismatches counts memory answers whose decoded address was
	// not the address requested (T2).
	AnswerMismatches int
	// Accumulator is the adapter's own tally: Unexpected counts the
	// broadcasts and other stations' traffic the filter dropped, Echoes
	// the suppressed echoes, NoiseBytes the discarded noise.
	Accumulator civ.AccumulatorStats
}

// Option configures a driver built by New.
type Option func(*ic9700Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it the engine's
// diagnostics — unexpected frames, quarantine drains, contamination —
// fall into the engine's own drop-everything default with no way for a
// caller to receive them. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ic9700Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose
// SESSIONS carry the consent transform: at session-capability assembly
// every write-side spec.Unverified label becomes spec.ConsentedUnverified,
// which FieldSupport.CanWrite opens.
//
// CONSENT IS NOT EVIDENCE, and three properties keep that honest. The
// driver's STATIC Capabilities are untouched, so what internal/wiring
// publishes and what the app describes the model with never carry the
// state. An unrecognised Profile stays on the untransformed fail-safe even
// WITH the option. And spec.FieldErase is exempt inside the transform
// itself, so no consent can mint an erase — see caps.go's erase note and
// doc.go's erase record.
//
// Nothing here consults writeTrialsComplete, deliberately: consent is a
// user accepting an unverified write, not a claim that the write has been
// proven.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic9700Driver) { d.consentUnverifiedWrites = true }
}

// New builds the IC-9700 driver for profile.
//
// RealHardware is the ZERO VALUE and, while writeTrialsComplete is false,
// selects the all-Unverified capability set — nothing writable. ANY
// unrecognised Profile value deliberately selects the same fail-safe: the
// failure direction for a forged or corrupted Profile is always "nothing
// writable", never a writable set.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ic9700Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ic9700Driver implements driver.Driver for the Icom IC-9700.
type ic9700Driver struct {
	profile         Profile
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent — set only by
	// WithConsentedUnverifiedWrites, read only by sessionCapabilities.
	consentUnverifiedWrites bool
}

// Model implements driver.Driver.
func (d *ic9700Driver) Model() string { return "IC-9700" }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile, with no session state and no consent.
func (d *ic9700Driver) Capabilities() spec.Capabilities {
	if d.profile == Simulated {
		return CapabilitiesSimulated()
	}
	// RealHardware, and every unrecognised value, land here: while
	// writeTrialsComplete is false there is no hardware-verified profile
	// for a real-radio session to select.
	return CapabilitiesUnverified()
}

// profileRecognised reports whether this driver's Profile is one this
// package declares. The consent transform is applied only for a
// recognised one, so a forged value cannot be consented into writability.
func (d *ic9700Driver) profileRecognised() bool {
	return d.profile == RealHardware || d.profile == Simulated
}

// sessionCapabilities is the EFFECTIVE set a session carries: the static
// baseline, plus the consent transform when — and only when — the option
// was passed AND the Profile is recognised.
//
// spec.ConsentUnverifiedWrites is the project's ONE definition of what
// consent means and is never reimplemented here.
func (d *ic9700Driver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// StopBits reports the CI-V link's stop-bit count, satisfying the optional
// driver.SerialFramingReporter (spec D3.1: every Icom driver returns 1).
//
// ASSUMED. The IC-9700 CI-V Reference Guide states NO bit count, parity or
// stop-bit count for any port — a full 28-page sweep found none, and the
// "8 bit / 1 stop" lines Icom prints elsewhere are about the DATA/RTTY
// application port, which is not CI-V. Register: FAMILY D5 entry 8, this
// model's own row. LIFTED BY R1 — one `19 00` transaction at 8-N-1, then
// at 8-N-2; which framing draws an address-matched reply.
//
// It is on the DRIVER, not the Session, and that is forced:
// internal/wiring holds the driver value BEFORE the port is opened, and
// the session does not exist until afterwards.
func (d *ic9700Driver) StopBits() int { return 1 }

var _ driver.SerialFramingReporter = (*ic9700Driver)(nil)

// Open implements driver.Driver: it builds a transport.Engine over port
// with the shared CI-V framing, drains the bus to quiet WITHOUT WRITING
// ANYTHING, probes `19 00` for an address-matched reply, and then searches
// a bounded run of channels for a record whose LENGTH confirms this
// profile.
//
// NO RADIO MUTATION AT INIT, EVER. Framing.InitSequence() is empty for
// CI-V — there is no transceive-off write, no clear and no `1A 05` — and
// broadcasts are excluded structurally by the accumulator's address
// filter rather than by asking somebody's radio to stop talking.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
func (d *ic9700Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	p := civic9700.Profile()
	framing, err := civ.NewFraming(p)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic9700: Open: %w", err)
	}
	// The two-result assertion is the house's optional-capability pattern
	// (core/driver/optional.go). It cannot fail for the value NewFraming
	// returns, and it is checked anyway: the alternative to a loud
	// refusal here is a session whose diagnostics silently report zero on
	// a saturated bus.
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic9700: the CI-V framing does not report accumulator stats")
	}

	var engOpts []transport.Option
	if d.transportLogger != nil {
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngineWith(port, framing, engOpts...)
	if err != nil {
		// NewEngineWith has not taken the port on this path, so closing
		// it is Open's own ownership obligation rather than a double
		// close.
		_ = port.Close()
		return nil, fmt.Errorf("ic9700: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path closes eng in exactly
// one place.
func (d *ic9700Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := civic9700.Profile()
	s := &Session{
		eng:     eng,
		stats:   stats,
		profile: p,
		caps:    d.sessionCapabilities(),
		raw:     map[string][]byte{},
	}

	// THE INIT-UNDER-FLOOD RULE (R9-SPLIT), and this is its ONE
	// implementation site.
	//
	// Init sends nothing and drains to quiet under an absolute cap. Two
	// floods behave differently, because the accumulator's address filter
	// sits between the line and the engine. A BROADCAST flood (to=00)
	// never reaches the drain cap at all — every frame is dropped before
	// the engine sees it, the idle timer is never re-armed, and Init
	// returns nil with the traffic visible only in
	// AccumulatorStats().Unexpected. A CONTROLLER-ADDRESSED flood is what
	// produces ErrDrainCapExceeded, because those frames DO reach the
	// engine.
	//
	// The INITIAL ErrDrainCapExceeded is NONFATAL: the spec's bounded
	// drain cannot fail the open, so it is RECORDED and the probe
	// continues. Every LATER quarantine drain failure — inside a read,
	// inside a write — stays fail-closed and is not touched here.
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic9700: Open: %w", err)
		}
		s.diag.InitDrainCapExceeded = true
	}

	token, err := probeIdentity(ctx, p, eng)
	if err != nil {
		return nil, err
	}
	s.diag.IDToken = token

	// Identity.CATID carries the ADDRESS — spec D3.2's CI-V identity, and
	// what an address-matched reply actually proved — with the observed
	// token beside it. The token is a diagnostic and is compared against
	// nothing; putting it in the string rather than in place of the
	// address is what keeps that visible to a human reading a journal.
	id.CATID = "A2"
	if len(token) > 0 {
		id.CATID = fmt.Sprintf("A2/%X", token)
	}
	s.id = id

	fingerprinted, mismatches, err := probeFingerprint(ctx, p, eng)
	if err != nil {
		return nil, err
	}
	s.diag.Fingerprinted = fingerprinted
	s.diag.AnswerMismatches += mismatches

	return s, nil
}

// probeIdentity sends `19 00` and returns the answer's data bytes.
//
// AN ADDRESS-MATCHED REPLY IS REQUIRED, and the VALUE is not. The reply's
// data is undocumented on all six models in this tier (spec D5 entry 7,
// lift R6), so nothing here compares it against an expected value: what
// identifies the radio at this step is that a reply addressed to this
// controller, from this radio, arrived at all. A matcher that checked the
// value would refuse every real radio whose byte differed from a guess.
//
// NO driver.WrongRadioError IS MINTED. That type reports an ID answer that
// named a DIFFERENT model, and this driver cannot know that: it has no
// token table and no cross-model claim to make. A radio at another address
// simply never answers, and the failure is the honest one — nothing that
// identified itself as this radio replied.
func probeIdentity(ctx context.Context, p civ.Profile, eng *transport.Engine) ([]byte, error) {
	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic9700: Open: %w", err)
	}
	answer, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), retryReads))
	if err != nil {
		return nil, fmt.Errorf("ic9700: Open: no address-matched reply to `19 00`: %w", err)
	}
	token, err := p.ParseTransceiverID(answer)
	if err != nil {
		return nil, fmt.Errorf("ic9700: Open: %w", err)
	}
	return decodeHexToken(token), nil
}

// decodeHexToken turns ParseTransceiverID's compact hex token back into
// the answer's own data bytes.
//
// The parser returns a token because that is the whole of its intended use
// — a diagnostics line and half of Identity.CATID — and this driver's
// diagnostics record the BYTES, so that a future capture comparing against
// R6's lift compares like with like. The conversion is mechanical and
// total: the token is always an even run of lower-case hex digits.
func decodeHexToken(token string) []byte {
	if len(token)%2 != 0 {
		return nil
	}
	val := func(c byte) (byte, bool) {
		switch {
		case c >= '0' && c <= '9':
			return c - '0', true
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10, true
		default:
			return 0, false
		}
	}
	out := make([]byte, 0, len(token)/2)
	for i := 0; i < len(token); i += 2 {
		hi, okHi := val(token[i])
		lo, okLo := val(token[i+1])
		if !okHi || !okLo {
			return nil
		}
		out = append(out, hi<<4|lo)
	}
	return out
}

// probeFingerprint walks a bounded run of channels looking for a record
// whose length confirms this profile.
//
// THE FINGERPRINT IS THE RECORD-ONLY LENGTH. Profile.MemoryAnswerRecord
// strips the three address bytes before AcceptsRecordLength is asked, so a
// wire data area of 114 bytes fingerprints as a RECORD of 111 and a
// *civ.RecordLengthError always reports record-only bytes. This model's
// characteristic bug is confusing the two.
//
// THE REFUSAL NAMES NO MODEL. Cross-model record-length distinctness is a
// TIER-level Wave-4 check needing a registry-wide table of accepted
// lengths; claiming here that {111} identifies this radio would be a claim
// this worktree has no way to support.
//
// T4: an empty slot arrives as transport.ErrRejected from Engine.Do, which
// CONSUMES the `FA` and returns no frame — so no branch here inspects "an
// FA frame".
//
// T2: the length gate precedes THIS FUNCTION'S address comparison — and
// it is the second gate inside MemoryAnswerRecord, not the first.
// p.MemoryAnswerRecord decodes the three address bytes and validates them
// against this profile's band and channel ranges (core/civ/parse.go:105 →
// core/civ/profile.go:316) BEFORE it checks AcceptsRecordLength
// (core/civ/parse.go:110), so an answer whose address bytes this radio
// could never have sent comes back as a *civ.ParseError and never reaches
// the length check at all. What T2 pins is the ordering that matters
// here: both of those gates return as `err` above, before this function
// reaches `got != want` below, so a wrong-channel answer that is ALSO the
// wrong length reports as a length error rather than as a mismatch
// counted toward `mismatches` — and one that is also un-decodable as an
// address reports as a parse error. `got != want` sees only answers whose
// address decoded cleanly AND whose record length this profile accepts.
//
// THE ADDRESS GATE IS WHY THE IC-705 IS NOT DISTINGUISHED HERE BY LENGTH.
// That radio shares this one's {111} record-only set and differs in
// address width, and core/civ/tier_test.go sweeps every address either
// radio has, in both directions: the refusal is a *civ.ParseError every
// time and a *civ.RecordLengthError never. The open still fails, which is
// the safety property; it simply fails unattributed.
func probeFingerprint(ctx context.Context, p civ.Profile, eng *transport.Engine) (bool, int, error) {
	mismatches := 0
	for ch := 1; ch <= probeSearchChannels; ch++ {
		want := civ.ChannelAddress{Group: probeSearchBand, Channel: ch}
		cmd, err := p.BuildMemoryRead(want)
		if err != nil {
			return false, mismatches, fmt.Errorf("ic9700: Open: %w", err)
		}
		answer, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), retryReads))
		if errors.Is(err, transport.ErrRejected) {
			continue // an unwritten channel: D5 entry 2(a)
		}
		if err != nil {
			return false, mismatches, fmt.Errorf("ic9700: Open: %w", err)
		}
		got, _, err := p.MemoryAnswerRecord(answer)
		if err != nil {
			return false, mismatches, fmt.Errorf("ic9700: Open: %w", err)
		}
		if got != want {
			mismatches++
			continue
		}
		return true, mismatches, nil
	}
	// Every channel in the bounded run answered FA (or answered for
	// somebody else). An empty radio is an ordinary radio: the session
	// opens on ADDRESS evidence alone, and says so.
	return false, mismatches, nil
}

// Session is one open, probed connection to an IC-9700.
//
// It implements driver.Session and driver.DiagnosticsReporter, and adds
// this model's own CIVDiagnostics.
//
// SAFE FOR CONCURRENT USE. transport.Engine serialises every exchange, and
// the mutex here covers only this type's own mutable state: the
// diagnostics counters and the raw-record cache the write path's template
// guard reads.
type Session struct {
	eng     *transport.Engine
	stats   civ.AccumulatorStatsReporter
	profile civ.Profile
	caps    spec.Capabilities
	id      driver.Identity

	mu   sync.Mutex
	diag CIVDiagnostics
	// raw holds the LAST raw record read for a slot, keyed by slot
	// string.
	//
	// ITS ONE PERMITTED USE IS THE E6 TEMPLATE GUARD, and the ruling that
	// admits it also forbids everything else: a value here is never a
	// source for a field a write would send, because that would be
	// preservation-by-cache. The write path re-reads the slot for itself
	// and uses THAT read's bytes; this map is what the read path fills so
	// the two cannot disagree about which read they are talking about.
	raw map[string][]byte
}

var (
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a defensive copy, so mutating
// what a caller was handed can never alter what this session's own write
// gate enforces.
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// Diagnostics implements driver.DiagnosticsReporter — the NEUTRAL
// counter, kept answering the neutral question.
//
// It is not the CI-V one, and the difference is the whole of the
// DIAGNOSTICS CARRIER ruling: this counts frames the ENGINE saw that did
// not match the spec in force, which on a CI-V bus is systematically zero
// because the accumulator dropped every broadcast first. CIVDiagnostics is
// where the numbers that mean something live.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.eng.UnexpectedFrames()
	if n < 0 {
		// Unreachable (the engine only ever increments), but never let a
		// negative int64 wrap into an absurd uint64.
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// CIVDiagnostics returns this session's CI-V diagnostics: what the probe
// learnt, what the driver refused, and the framing adapter's own tallies.
func (s *Session) CIVDiagnostics() CIVDiagnostics {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.diag
	out.IDToken = append([]byte(nil), s.diag.IDToken...)
	out.Accumulator = s.stats.AccumulatorStats()
	return out
}

// noteAnswerMismatch records a T2 refusal.
func (s *Session) noteAnswerMismatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diag.AnswerMismatches++
}

// rememberRaw stores a slot's raw record bytes for the write path's
// template guard. A nil record forgets the slot, which is what an empty
// answer means.
func (s *Session) rememberRaw(slot string, record []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record == nil {
		delete(s.raw, slot)
		return
	}
	s.raw[slot] = append([]byte(nil), record...)
}

// Close implements driver.Session. Idempotent, because
// transport.Engine.Close is.
func (s *Session) Close() error { return s.eng.Close() }
