// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes. See WithTransportLogger.
type Option func(*ft991aDriver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a
// caller of this driver to receive them. A nil l is ignored (the engine's
// default is kept).
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft991aDriver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose SESSIONS
// carry the consent transform: at session-capability assembly every
// write-side spec.Unverified label becomes spec.ConsentedUnverified, which
// FieldSupport.CanWrite opens (see sessionCapabilities, and
// spec.ConsentUnverifiedWrites for the one definition of what consent
// means).
//
// Consent is a statement about a SESSION, never about the radio: this
// driver's STATIC Capabilities — what internal/wiring's registry publishes,
// what the app describes the model with, what offline synthesis classifies
// against — is untouched by the option, and only the set Open assembles
// carries the state.
//
// It is deliberately not sufficient on its own. An unrecognised Profile
// stays on the untransformed fail-safe even WITH the option
// (profileRecognised); spec.FieldErase is exempt inside the transform
// itself, so no consent can mint an erase — which on this radio is doubly
// idle, since its Control Command List contains no erase command at all —
// and nothing here consults writeTrialsComplete, because consent is a user
// accepting an unverified write, not evidence that the write has been
// proven.
//
// ON THIS RADIO CONSENT REACHES A VOCABULARY NO SIBLING HAS. The five-state
// P8 (caps.go's ctcssStates) is graded writable, so a consented session may
// put '3' or '4' — a DCS state — on the wire. That is the matrix's own
// [STUART] cell, ruled writable because the legend prints both values
// against the MT and MW SET charts as well as the read-side blocks; what the
// radio does with the DCS CODE it already holds for such a channel is
// doc.go's register entry A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft991aDriver) {
		d.consentUnverifiedWrites = true
	}
}

// New builds the FT-991A driver for profile. RealHardware — the ZERO VALUE —
// selects the all-Unverified capability set while writeTrialsComplete is
// false (nothing writable; see that constant's doc comment), and ANY
// unrecognised Profile value deliberately selects the same fail-safe: the
// failure direction for a forged or corrupted Profile is always "nothing
// writable", never a writable set. Options: WithTransportLogger,
// WithConsentedUnverifiedWrites.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ft991aDriver{profile: profile, dialect: catDialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft991aDriver implements driver.Driver for the Yaesu FT-991A.
type ft991aDriver struct {
	profile Profile
	// dialect is the CAT dialect every codec call this driver makes — and
	// every Session it Opens makes — goes through. Set from catDialect in
	// New; no Option touches it.
	//
	// It lives HERE rather than only on Session because Open builds the
	// transport.Engine — handing it this very value, from which the engine
	// takes both its outbound gate and its AI init frame — BEFORE any
	// Session exists: a dialect reachable only from Session would be
	// reachable too late to bind the engine at all.
	dialect cat.Dialect
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default.
	consentUnverifiedWrites bool
}

// Model implements driver.Driver.
func (d *ft991aDriver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
//
// ON THIS RADIO IT IS ALSO THE WHOLE STORY, unlike every sibling that has a
// discovered bank: Open probes nothing (matrix §3.4), so a Session's
// EFFECTIVE capabilities are this same set, plus the consent transform when
// the user has given it. Session.Capabilities is therefore a copy of this
// rather than of something larger.
func (d *ft991aDriver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No FT-991A has ever been written to over CAT by this project —
		// no FT-991A has ever been ASKED anything at all:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe — nothing writable, every write
		// refused before a frame is built UNLESS the user has consented,
		// which is the one thing that reaches past these labels and does so
		// downstream of this method (sessionCapabilities transforms what
		// this arm returns; the set returned HERE is never transformed, and
		// that is what internal/wiring.NeedsUnverifiedConsent reads). See
		// writeTrialsComplete's doc comment for what its flip must change.
		return CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence of the flip's state. Pinned by
		// TestDriver_ProfileSelection's unrecognised rows.
		return CapabilitiesUnverified()
	}
}

// idSpec is the transport spec for the ID; probe: a fixed 7-byte answer
// ("ID0670;"). The length is core/cat's idAnswerLen, and core/cat/ft991a's
// reused-command verification checked this radio's own ID frame table
// against it (layout 771-779: no Set, Read "ID;" three bytes at 776, Answer
// seven at 779) before the shared codec was accepted — see that package's
// doc.go. One retry: an identity read is idempotent and Open should survive
// a single swallowed reply.
//
// ONE RETRY HERE AND NONE ON THE MT READ, and the asymmetry is deliberate
// (read.go's mtSpec says the other half): a retried ID probe cannot be
// confused with anything, because Open has no second frame whose meaning
// depends on how many times the first was asked.
func idSpec() transport.CommandSpec {
	return transport.CATReadSpec("ID", 7, 1)
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID; and
// verifies this really is an FT-991A (a typed *driver.WrongRadioError
// otherwise), and returns the Session.
//
// AI0; THEN ID; AND NOTHING ELSE. There is no discovery phase at all
// (matrix §3.4, plan P10 and task 10): the transcript is TWO FRAMES on a
// match and two on a mismatch, port closed. Nothing is probed because there
// is nothing to probe — "5xx", "5 MHz", "5MHz" and "EMG" appear in no slot
// legend of this manual, checked mechanically over the whole extraction, and
// both banks this radio does have are dense and complete at construction.
//
// THE PIN FOR THAT IS NEGATIVE, AND IT HAS TO BE. The FT-891 spends up to
// eleven MR exchanges here and can assert that they happened; an absence
// cannot be caught by watching the right thing happen, and a regression that
// added a walk would simply work, slowly, against any peer that answered. So
// TestOpen_SendsExactlyTwoFrames asserts the WHOLE transcript and
// TestOpen_NeverBuildsAnMROfANonExistentBank asserts it over a whole session.
//
// The AI0 preamble is transport.Engine.Init's and is shared by every
// registered Yaesu driver; a WRONG RADIO therefore receives exactly two
// frames — the preamble and the probe — and nothing more. Moving ID ahead of
// AI0 is a fleet seam and a roadmap item, not this driver's to take
// unilaterally.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
//
// The engine is bound to THIS driver's own dialect, passed WHOLE (d.dialect
// — see transport.NewEngine): both the outbound allowlist a session enforces
// and the AI init frame it opens with belong to the radio the session is
// for, never a package-level default that would gate every radio by whatever
// one of them permits. On this radio the gate carries a refusal no sibling's
// does — a record naming a DCS state is buildable here and refused by every
// ToneStatesCTCSS dialect — so a session gated by a sibling's dialect would
// refuse frames this one must send. transport.NewEngine refuses an
// unconfigured dialect outright, so there is no ungated path through here
// even if the field were somehow left zero.
func (d *ft991aDriver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	var engOpts []transport.Option
	if d.transportLogger != nil {
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngine(port, d.dialect, engOpts...)
	if err != nil {
		// NewEngine has not taken the port on this path (it refuses before
		// touching it), so closing it here is Open's own ownership
		// obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ft991a: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in exactly
// one place.
func (d *ft991aDriver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ft991a: Open: %w", err)
	}

	// Identity probe: the ID; answer is authoritative, and anything other
	// than the FT-991A's "0670" (layout 772) means the wrong radio — or
	// something else that speaks CAT — is on this port. That matters more
	// here than on most: this radio shares a connector, a baud menu and a
	// CAT grammar with every registered Yaesu sibling, and differs from
	// them in axes a wrong-radio session would silently mis-encode — a
	// five-state P8, a numeric PMS form, a fixed P11 and a live P5.
	frame, err := eng.Do(ctx, d.dialect.BuildIDRead(), idSpec())
	if err != nil {
		return nil, fmt.Errorf("ft991a: Open: ID probe: %w", err)
	}
	got, err := d.dialect.ParseIDAnswer(frame)
	if err != nil {
		return nil, fmt.Errorf("ft991a: Open: ID probe: %w", err)
	}
	if got != catID {
		// WantModel populated, GotModel deliberately EMPTY (plan P10,
		// matrix §3.10). driver.WrongRadioError.Error() renders its NAMED
		// form only when BOTH are present, while cmd/rigprog's probe
		// formatter keys on GotModel alone — so a driver filling one alone
		// would render the same refusal two different ways.
		//
		// THERE IS NO SIBLING ID TABLE, and in particular NO attempt to
		// name "FT-991" on the GOT side. That radio is a DIFFERENT REAL
		// RADIO rather than a typo of this one, and this project has never
		// seen its ID answer: putting a guessed name in a refusal about
		// identity would be the one place a guess is least excusable. "With
		// names" is satisfied on the WANT side only, and the rendered text
		// is pinned verbatim by TestOpen_WrongRadio because rendered
		// refusals are recorded in baselines.
		return nil, &driver.WrongRadioError{Want: catID, Got: got, WantModel: modelName}
	}
	id.CATID = got

	return &Session{
		eng:     eng,
		dialect: d.dialect,
		id:      id,
		caps:    d.sessionCapabilities(),
	}, nil
}

// sessionCapabilities is the ONE place a session's effective capability set
// is assembled: this driver's static baseline, then — only when it was built
// with WithConsentedUnverifiedWrites AND its profile is one of the declared
// constants — the consent transform. An unrecognised profile stays
// untransformed even with the option: the fail-safe direction ("no value a
// caller can pass produces a writable session") survives consent. Applying
// the transform here, before the Session exists, keeps the set WriteChannel
// enforces (s.caps) and the set Capabilities() hands out the same value.
//
// IT TAKES NO DISCOVERED INVENTORY, where every sibling's namesake does,
// because Open discovers nothing (matrix §3.4). There is deliberately no
// effectiveCapabilities function to pass one to: a seam that existed only to
// append banks this radio cannot have would be a place for a later reader to
// add one.
func (d *ft991aDriver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// profileRecognised reports whether this driver's profile is one of the
// package's declared Profile constants — the same set the capability switch
// names explicitly, restated here so the consent gate cannot drift open for
// a profile the switch would fail safe on.
func (d *ft991aDriver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// Session is the FT-991A's driver.Session: one open, identity-verified
// connection. Safe for concurrent use.
//
// IT CARRIES AN OPERATION MUTEX, and the reason is not the FT-891's.
// transport.Engine serialises each individual EXCHANGE, and this radio's
// ReadChannel is ONE exchange — the combined MT read, with no cross-check to
// straddle (read.go, matrix §3.5) — so a read needs no lock against another
// read's second frame, because there is no second frame.
//
// What opMu guards is a whole DRIVER OPERATION (spec erratum S-E4, matrix
// M-E2), and this session has more than one kind of those: a read, a write
// (write.go) and a settings read (task 12) must not interleave their frames
// even though the engine would happily serialise them one exchange at a
// time. The concurrency pin plan P12 asks for is that two racing
// ReadChannels cannot interleave two MT frames.
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to core/clone,
// as the driver interface assigns it, and holding a driver lock across it
// would serialise two operations the seam deliberately keeps separate.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// dialect is the CAT dialect this session's every codec call goes
	// through — builders, parsers, the answer geometry, and the mode
	// rendering ReadChannel puts in front of the user. Copied from the
	// ft991aDriver that Opened it (which is also where the engine's gate
	// came from), so a session can never encode with one radio's dialect
	// while its transport gates with another's.
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set — on
// this radio the profile baseline itself, plus consent when the user gave it,
// because nothing is ever discovered — as a deep copy per call (see
// cloneCapabilities for why the copy is load-bearing).
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable (the *transport.Engine is
// unexported inside this Session). Safe for concurrent use, like the
// accessor it wraps. It satisfies the optional driver.DiagnosticsReporter
// capability; like its siblings', it is a method on the concrete *Session
// rather than part of driver.Session, because which diagnostics exist is a
// per-driver matter.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.eng.UnexpectedFrames()
	if n < 0 {
		// Unreachable (the engine only ever increments), but never let a
		// negative int64 wrap into an absurd uint64.
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

// ErrAnswerMismatch is the sentinel a caller should compare against (via
// errors.Is) when a slot-addressed answer names a DIFFERENT slot than the
// one just requested. The transport's quarantine discipline makes this
// unlikely (a stale same-shape reply should have been drained), but the
// driver still refuses to map an answer onto the wrong slot. The error
// actually returned is an *AnswerMismatchError.
var ErrAnswerMismatch = errors.New("ft991a: answer names a different slot than was requested")

// AnswerMismatchError reports the requested and the answered slot. It is
// this driver's OWN typed error, in this driver's own namespace: four
// sibling drivers have same-shaped ones and none imports another — a caller
// distinguishing which radio's read went wrong needs distinct types, and a
// shared one would put a radio-specific failure on a seam that is meant to
// be neutral.
type AnswerMismatchError struct {
	// Requested is the slot the read asked for.
	Requested string
	// Answered is the slot the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ft991a: requested slot %q but the answer names slot %q — refusing to map a reply onto the wrong slot", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }
