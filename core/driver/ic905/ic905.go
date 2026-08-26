// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ic905Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a
// caller of this driver to receive them. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ic905Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// New builds the IC-905 driver for profile. RealHardware — the ZERO
// VALUE — selects the all-Unverified capability set while
// writeTrialsComplete is false (nothing writable; see that constant's doc
// comment), and ANY unrecognised Profile value deliberately selects the
// same fail-safe: the failure direction for a forged or corrupted Profile
// is always "nothing writable", never a writable set.
//
// ONE constructor, because there is one radio. core/driver/ftdx101 offers
// NewD and NewMP because it drives two; this package's model is fixed by
// the package.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ic905Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ic905Driver implements driver.Driver for the Icom IC-905.
type ic905Driver struct {
	profile Profile
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default.
	consentUnverifiedWrites bool
}

// Model implements driver.Driver: the registry key and display name,
// taken from the DIALECT rather than restated, so the two cannot drift.
func (d *ic905Driver) Model() string { return civic905.Model }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile.
//
// MEM's Slots is EMPTY here and that is the point: it is a sparse bank
// whose materialised set is discovered per session at Open (see
// Session.Capabilities for the effective, per-radio set). The static
// baseline is what internal/wiring's registry publishes, what the app
// describes the model with, and what offline synthesis classifies
// against — none of which has a radio to ask.
func (d *ic905Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		// No IC-905 has ever been written to over CI-V by this project:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe — nothing writable, every write
		// refused before a frame is built UNLESS the user has consented,
		// which is the one thing that reaches past these labels and does
		// so downstream of this method (sessionCapabilities transforms
		// what this arm returns; the set returned HERE is never
		// transformed, and that is what
		// internal/wiring.NeedsUnverifiedConsent reads).
		//
		// writeTrialsComplete is deliberately NOT read here: see its doc
		// comment for what a flip must change, and why a constant that
		// was load-bearing on its own would let a one-character edit
		// unlock a write.
		return capabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence.
		return capabilitiesUnverified()
	}
}

// profileRecognised reports whether this driver's profile is one of the
// package's declared Profile constants — the same set Capabilities'
// switch names explicitly, restated here so the consent gate cannot drift
// open for a profile that switch would fail safe on.
func (d *ic905Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// sessionCapabilities is the ONE place a session's effective capability
// set is assembled: effectiveCapabilities' product — the static baseline
// with the sparse MEM bank's discovered inventory materialised — with the
// OBSERVED identity token joined onto the CATID.
//
// THE TOKEN GOES ON THE SESSION'S CAPABILITIES, NOT ONLY ON Identity, and
// that is Codex 12's finding: core/clone's ReadAll records the SESSION
// CAPABILITIES' CATID into the codeplug (core/clone/read.go), so a driver
// that put the pair on Identity alone would never get the observed token
// into a saved file. One format, used at both sites: "AC:" + the token.
//
// The STATIC Capabilities() keeps the bare "AC" — there is no observed
// token before a session exists, and that static set is what
// internal/wiring's registry publishes.
func (d *ic905Driver) sessionCapabilities(discovered []string, catID string) spec.Capabilities {
	caps := effectiveCapabilities(d.Capabilities(), discovered)
	caps.CATID = catID
	return caps
}

// Open implements driver.Driver: it builds a transport.Engine over port
// through core/civ's framing adapter, establishes the session (Init:
// NOTHING transmitted, then a bounded drain to quiet), and probes the
// radio's identity with 19 00.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes what it holds before
// returning an error, exactly as core/driver's contract requires.
//
// NO RADIO MUTATION AT INIT, EVER. civ's InitSequence is EMPTY (spec D2,
// adjudication 3): no transceive-off write, no clear, no 1A 05. A
// transceive flood is excluded STRUCTURALLY, by the accumulator's address
// filter, rather than by writing a setting — so opening a session touches
// nothing outside the consent regime.
//
// NO ECHO PROBING (D3.4). Echo is handled structurally by NoteSent plus
// byte equality, so a USB-echo-ON radio and a USB-echo-OFF radio behave
// identically to the engine. The USB Echo Back setting's factory value is
// ASSUMED and is RECORDED, not switched on: ic905.usb_echo_default, lift
// ic905-R-13.
//
// ONE ENGINE PER civ.NewFraming VALUE. The adapter holds ONE accumulator,
// built at NewFraming, and a second NewAccumulator on the same value is
// refused closed. Open therefore calls NewFraming per session and never
// caches one on the driver.
func (d *ic905Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	profile := civic905.Profile()

	framing, err := civ.NewFraming(profile)
	if err != nil {
		// NewFraming has not touched the port — it has never seen it —
		// so closing it here is Open's own ownership obligation, not a
		// double close.
		_ = port.Close()
		return nil, fmt.Errorf("ic905: Open: %w", err)
	}
	// THE TWO-RESULT TYPE ASSERTION IS HOW A DRIVER REACHES THE
	// ACCUMULATOR'S COUNTERS AT ALL (the house's optional-capability
	// pattern, core/driver/optional.go): NewFraming's declared result is
	// the neutral transport.Framing, and must stay so. A nil stats is
	// legal and Diagnostics905 handles it.
	stats, _ := framing.(civ.AccumulatorStatsReporter)

	var engOpts []transport.Option
	if d.transportLogger != nil {
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngineWith(port, framing, engOpts...)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic905: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, profile, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in
// exactly one place.
func (d *ic905Driver) open(ctx context.Context, eng *transport.Engine, profile civ.Profile, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	s := &Session{eng: eng, profile: profile, stats: stats}

	if err := eng.Init(ctx); err != nil {
		// THE INIT DRAIN'S TWO HALVES (ruling R9-SPLIT), and the
		// difference is WHICH ADDRESS the flood carries.
		//
		// (a) A BROADCAST flood (to = 00) never gets here: the
		// accumulator's address filter drops every frame before any
		// engine event, so the idle timer never re-arms and Init
		// SUCCEEDS. Those frames appear only in
		// AccumulatorStats().Unexpected.
		//
		// (b) A CONTROLLER-ADDRESSED flood reaches DrainCap, and Init
		// returns ErrDrainCapExceeded. It is NONFATAL-WITH-DIAGNOSTIC,
		// because the spec's bounded initial drain "cannot fail the
		// open": the line is noisy, not wrong, and refusing to open
		// would leave a user with a radio they cannot read. It is
		// RECORDED so an operator can see it.
		//
		// EVERY LATER QUARANTINE DRAIN FAILURE REMAINS FAIL-CLOSED —
		// this leniency is the INITIAL drain's alone. A later one means
		// an exchange's own outcome is unknowable, and Engine.Do returns
		// the failure unchanged.
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic905: Open: %w", err)
		}
		s.initDrainCapExceeded = true
	}

	// The identity probe: 19 00, address-matched, VALUE RECORDED AND NOT
	// COMPARED.
	//
	// The reply value is undocumented on all six of this tier's documents
	// (spec D5 entry 7; matrix §3.12: the command table gives the row a
	// description and an EMPTY Data cell and nothing more), so the probe
	// cannot compare a token. What it requires is that SOMETHING ANSWERED
	// AT AC, TO E0 — and a matcher that checked the value would refuse
	// every real radio this tier has never seen, which is all of them.
	// Register: D5 entry 7. Lift: ic905-R-02.
	//
	// A WRONG RADIO AT A DIFFERENT DEFAULT ADDRESS SIMPLY TIMES OUT, and
	// this driver claims no more than that: the record-length fingerprint
	// (Task 12) protects against SAME-ADDRESS confusion only (spec D3.2).
	cmd, err := profile.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic905: Open: 19 00 identity probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, s.idSpec())
	if err != nil {
		return nil, fmt.Errorf("ic905: Open: 19 00 identity probe: %w", err)
	}
	token, err := profile.ParseTransceiverID(frame)
	if err != nil {
		return nil, fmt.Errorf("ic905: Open: 19 00 identity probe: %w", err)
	}

	catID := identityCATID(token)
	id.CATID = catID
	s.id = id
	s.caps = d.sessionCapabilities(nil, catID)
	return s, nil
}

// identityCATID joins this radio's address with the observed 19 00 token
// into the ONE format spec D3.2 fixes for a CI-V Identity: the address in
// hex, a colon, then the answer's data bytes as UPPERCASE hex with no
// separators.
//
// civ.ParseTransceiverID already renders the token, in lower case, and
// this is the only place the case is decided — a driver deciding it twice
// would be how "AC:94" and "AC:94" stop being the same string. Upper
// case, because the address half is upper case and a CATID a user reads
// off a diagnostics line should not change case halfway through.
func identityCATID(token string) string {
	return fmt.Sprintf("%02X:%s", civic905.RadioAddress, strings.ToUpper(token))
}

// Session is an IC-905's driver.Session: one open, identity-probed
// connection. Safe for concurrent use — transport.Engine serialises every
// individual exchange, the capability set is immutable after Open, and
// the one mutable counter is atomic.
//
// There is NO operation mutex, and that is a consequence of the choreography
// rather than an omission: a read is ONE exchange, and a write is a read
// followed by a set, which the engine already serialises individually.
// (The write's pair is not atomic against a concurrent write to the same
// slot; nothing above this seam issues two concurrent writes to one slot,
// and core/clone executes a plan serially.)
type Session struct {
	eng *transport.Engine
	// profile is the CI-V dialect every codec call goes through —
	// builders, parsers, matchers, the gate the engine was bound with.
	// Copied from the driver that Opened it, so a session can never
	// encode with one profile while its transport gates with another.
	profile civ.Profile
	// stats is the framing adapter's own counter surface, or nil if the
	// adapter did not offer it. See Diagnostics905.
	stats civ.AccumulatorStatsReporter
	id    driver.Identity
	caps  spec.Capabilities // effective; never mutated after Open

	// fingerprinted and observedLen record what the probe's bounded
	// record-length search found (Task 12).
	fingerprinted bool
	observedLen   int
	// inventoryComplete records whether the discovery walk covered the
	// whole space (Task 13).
	inventoryComplete bool
	// inventory is the set of slots discovery MATERIALISED, which rung 11
	// of the write ladder consults (ruling T3).
	inventory map[string]bool
	// initDrainCapExceeded records R9-SPLIT branch (b).
	initDrainCapExceeded bool
	// answerMismatches counts memory answers whose decoded address was
	// not the address requested (ruling T2). Atomic because reads may run
	// concurrently.
	answerMismatches atomic.Int64
}

// idSpec is the transport spec for the 19 00 identity read, assembled
// from E1's helper over the CODEC's own matcher — the transceiver-ID
// answer matcher civ.Profile exports, which checks to == controller,
// from == this profile's radio address, cn == 0x19 and sc == 0x00.
// Deviation (a): matchers come from the codec; specs are assembled at the
// transport-consuming layer, and a driver-built closure over raw frame
// accessors would reimplement the codec's own knowledge one layer up.
//
// THE SECOND ARGUMENT IS THE RETRY POLICY AND IT IS EXPLICIT. One retry
// for the identity read: an unanswered 19 00 on a quiet line is worth
// asking twice before declaring the radio absent, and the read is
// idempotent (transport safety obligation 2 permits retrying reads for
// exactly that reason).
func (s *Session) idSpec() transport.CommandSpec {
	return civ.CIVReadSpec(s.profile.TransceiverIDAnswerMatcher(), 1)
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set
// (profile baseline, the discovered sparse inventory, the observed
// identity token), as a deep copy per call — see cloneCapabilities for
// why the copy is load-bearing.
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// Diagnostics is this driver's own point-in-time health snapshot. The
// neutral driver.SessionDiagnostics carries UnexpectedFrames and is
// deliberately not grown per model (core/driver/optional.go's own note),
// so the IC-905's model-specific facts live here and the neutral snapshot
// is embedded.
//
// THE BROADCAST AND ECHO COUNTS COME FROM THE ADAPTER, NOT THE ENGINE
// (ruling R1, and civ's own AccumulatorStatsReporter doc says why): the
// accumulator has already swallowed every transceive broadcast before the
// engine could count one, so Engine.UnexpectedFrames reports a healthy
// zero on a line saturated with transceive.
type Diagnostics struct {
	driver.SessionDiagnostics

	// Accumulator is the adapter's own snapshot: Frames, Echoes,
	// Unexpected (broadcasts and other stations' traffic), Rejections,
	// Acknowledgements, NoiseBytes, Truncated.
	Accumulator civ.AccumulatorStats

	// Fingerprinted is false when the radio answered FA to every slot in
	// the bounded probe search: the session opened on ADDRESS EVIDENCE
	// ALONE (spec D3.2) and no record length was ever observed.
	Fingerprinted bool

	// ObservedRecordLength is the length the probe confirmed, 0 when
	// Fingerprinted is false. Either 64 or 65.
	ObservedRecordLength int

	// InventoryComplete is false when discovery stopped at the budget, on
	// ctx cancellation, or because the bounded default walk does not
	// cover the whole space. AN EARLY STOP MUST NOT BE READ AS AN EMPTY
	// RADIO.
	InventoryComplete bool

	// InitDrainCapExceeded records the NONFATAL initial-drain case of
	// R9-SPLIT branch (b): a CONTROLLER-ADDRESSED flood reached DrainCap,
	// Init returned ErrDrainCapExceeded, and the open succeeded anyway. A
	// broadcast flood never sets it — those frames die at the address
	// filter (branch (a)).
	InitDrainCapExceeded bool

	// AnswerMismatches counts memory answers whose decoded ChannelAddress
	// was not the address requested (ruling T2). Every such read failed
	// with ErrAnswerMismatch rather than being stored under the wrong
	// slot.
	AnswerMismatches int
}

// Diagnostics implements the optional driver.DiagnosticsReporter with the
// NEUTRAL snapshot, so the optional capability keeps its declared shape.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.eng.UnexpectedFrames()
	if n < 0 {
		// Unreachable (the engine only ever increments), but never let a
		// negative int64 wrap into an absurd uint64.
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// Diagnostics905 returns the full per-model snapshot, SUMMED LIVE from
// both sides of the address filter: the engine's counters plus the
// adapter's AccumulatorStats. It is NOT a stored snapshot — a cached
// value nothing ever refreshed would leave the broadcast counts at zero,
// which is the one number the carrier exists to report.
func (s *Session) Diagnostics905() Diagnostics {
	d := Diagnostics{
		SessionDiagnostics:   s.Diagnostics(),
		Fingerprinted:        s.fingerprinted,
		ObservedRecordLength: s.observedLen,
		InventoryComplete:    s.inventoryComplete,
		InitDrainCapExceeded: s.initDrainCapExceeded,
		AnswerMismatches:     int(s.answerMismatches.Load()),
	}
	if s.stats != nil {
		d.Accumulator = s.stats.AccumulatorStats()
	}
	return d
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

// ReadChannel is Task 13's and lives in read.go; WriteChannel is Task
// 14's and lives in write.go. Until those land, both are placeholders
// that refuse honestly — a Session must satisfy driver.Session for Open
// to return one at all, and a placeholder that refuses is better than one
// that pretends.
func (s *Session) ReadChannel(_ context.Context, slot string) (codeplug.Channel, error) {
	return codeplug.Channel{}, fmt.Errorf("ic905: ReadChannel %s: not implemented yet", slot)
}

// WriteChannel — see ReadChannel.
func (s *Session) WriteChannel(_ context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ic905: WriteChannel %s: not implemented yet", ch.Slot)
}
