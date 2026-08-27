// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
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

// SiblingLengths maps a record length to the model that accepts it: the
// seam through which a Wave-4 tier check can teach this driver to
// ATTRIBUTE a foreign record length to the radio that produced it.
//
// EMPTY IN WAVE 3, and TestProbe_TheSiblingTableIsEmptyInWaveThree pins
// it so. This worktree does not know any other model's accepted set —
// cross-model record-length distinctness is a tier-level check, and this
// driver claims none — so with no table EVERY unrecognised length takes
// the unattributed branch, which is the honest one for a driver that
// cannot name what it found.
type SiblingLengths map[int]string

// WithSiblingRecordLengths supplies the table above. Wave 4 populates it
// from the registry in the same commit that registers the tier's models
// and runs the distinctness check; until then the branch exists, is
// reachable, and is proven by test with a synthetic table.
func WithSiblingRecordLengths(l SiblingLengths) Option {
	return func(d *ic905Driver) {
		d.siblingLengths = make(SiblingLengths, len(l))
		for n, model := range l {
			d.siblingLengths[n] = model
		}
	}
}

// WithFullInventoryWalk makes Open discover the WHOLE 100 × 100 memory
// space instead of the bounded default walk.
//
// IT IS OPT-IN, AND THE DEFAULT IS BOUNDED FOR AN OPERATIONAL REASON
// (ruling R12). A complete walk is 10,000 reads — minutes of Open at CI-V
// rates on a sparse or empty radio — and a multi-minute default Open
// trains users to interrupt it, an interrupted discovery being exactly
// the "codeplug full of deletions" hazard discovery exists to guard
// against. The bounded walk reads group 0 in full, then one channel per
// group, descending only where that channel answered.
//
// USE IT WHEN CHANNELS ARE SCATTERED — and note that it is also the
// remedy the write ladder's occupied-surprise refusal NAMES: a slot the
// bounded walk missed cannot be written until the inventory knows about
// it (ruling T3).
func WithFullInventoryWalk() Option {
	return func(d *ic905Driver) { d.fullInventoryWalk = true }
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose
// SESSIONS carry the consent transform: at session-capability assembly
// every write-side spec.Unverified label becomes
// spec.ConsentedUnverified, which FieldSupport.CanWrite opens.
//
// CONSENT IS A STATEMENT ABOUT A SESSION, NEVER ABOUT THE RADIO. This
// driver's STATIC Capabilities — what internal/wiring's registry
// publishes, what the app describes the model with, what offline
// synthesis classifies against — is untouched by the option, and only the
// set Open assembles carries the state. The profile keeps saying the true
// thing either way: it describes the EVIDENCE (none — no IC-905 has ever
// been asked anything), and consent is a decision about risk, not
// evidence.
//
// It is deliberately not sufficient on its own. An unrecognised Profile
// stays on the untransformed fail-safe even WITH the option
// (profileRecognised); spec.ConsentUnverifiedWrites never touches
// spec.FieldErase, so no consent can mint an erase; and nothing here
// consults writeTrialsComplete, because consent is a user accepting an
// unverified write, not evidence that the write has been proven.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic905Driver) { d.consentUnverifiedWrites = true }
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
	// siblingLengths is the Wave-4 attribution table — see
	// SiblingLengths. Nil is the Wave-3 default.
	siblingLengths SiblingLengths
	// fullInventoryWalk opts out of the bounded default discovery walk —
	// see WithFullInventoryWalk. FALSE is the zero value and the default.
	fullInventoryWalk bool
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
	// THE CONSENT TRANSFORM, applied HERE and nowhere else: before the
	// Session exists, so the set WriteChannel enforces (s.caps) and the
	// set Capabilities() hands out are one value. An unrecognised profile
	// stays untransformed even with the option — the fail-safe direction
	// ("no value a caller can pass produces a writable session") survives
	// consent.
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// StopBits reports the CI-V link's stop-bit count, 8-N-1, per spec D3.1.
// internal/wiring consults it when opening the port; WITHOUT IT THE PORT
// WOULD OPEN AT transport.DefaultStopBits, WHICH IS 2 — the Yaesu value,
// on a radio this tier assumes speaks 8-N-1.
//
// It is on the concrete DRIVER, not the Session, and that is forced: the
// stop bits are a property of the port, chosen when the port is opened,
// and the session is what Open returns once the port already exists.
//
// ASSUMED, AND THE ASSUMPTION IS THE WHOLE ENTRY. THIS DOCUMENT EVIDENCES
// NOTHING ABOUT SERIAL FRAMING, FOR ANY PORT: no data-bit count, no
// stop-bit count, no parity statement and no rate figure anywhere (matrix
// §3.1's sweep, re-run at run-report §5.1). The tier-wide hazard — an
// Icom manual's "8 bit / 1 stop" line about the DATA/RTTY application
// port being mistaken for CI-V framing — does not even arise here,
// because no such line exists in this document to be misread. The value
// is the TIER's, not this manual's.
//
// Register: D5 entry 8 (serial framing 8-N-1, per model), carried in this
// package's own register — see doc.go — because StopBits() lives here.
// Lift: Stage R capture ic905-R-10 — open the IC-905's Serial Port A
// (CI-V) at 8-N-1, send FE FE AC E0 19 00 FD, and record whether an
// address-matched reply is framed cleanly. Scope: this one radio's CI-V
// port at that one rate; it says nothing about any other Icom.
func (d *ic905Driver) StopBits() int { return 1 }

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
	s := &Session{eng: eng, profile: profile, stats: stats, inventory: map[string]bool{}}

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

	if err := s.fingerprint(ctx, d.siblingLengths); err != nil {
		return nil, err
	}

	// Sparse-bank discovery, and it is not optional: core/clone's ReadAll
	// walks Capabilities().Banks[].Slots, so a sparse bank whose Slots
	// stayed empty would report a radio with no memories at all (ruling
	// R12). See discoverInventory for the bound and for why
	// InventoryComplete is the load-bearing half of it.
	budget := memBudget(d.Capabilities())
	if budget <= 0 {
		return nil, fmt.Errorf("ic905: Open: %w", errNoMemoryBank)
	}
	discovered, complete, err := s.discoverInventory(ctx, d.fullInventoryWalk, budget)
	if err != nil {
		return nil, fmt.Errorf("ic905: Open: memory inventory discovery: %w", err)
	}
	s.inventoryComplete = complete

	catID := identityCATID(token)
	id.CATID = catID
	s.id = id
	s.caps = d.sessionCapabilities(discovered, catID)
	return s, nil
}

// The probe's bounded record-length search, and its bound.
//
// BOUNDED SO AN EMPTY RADIO'S OPEN CANNOT TAKE TEN THOUSAND READS. The
// addressable space is 100 × 100, and a search that walked it would spend
// minutes on a radio with nothing in it — before this driver had even
// decided the radio was an IC-905. Sixteen channels of group 0 is enough
// to find the first channel of any radio somebody has actually used, and
// the twelve CALL slots are added because they are the twelve addresses
// this document says a radio always has.
//
// The FULL inventory walk is a DIFFERENT question and is discovery's
// (read.go): this search stops at the first record it sees, because one
// record is all a fingerprint needs.
const (
	probeSlotsInGroupZero = 16
	callSlotsProbed       = civic905.CallChannels
	callWireGroup         = civic905.CallGroup
)

// fingerprint is the probe's second half (spec D3.2): read memory
// channels until one answers with a record, and decide what its LENGTH
// says about which radio this is.
//
// THE ACCEPTED SET IS THE FINGERPRINT, and this model's set has two
// members — 64 and 65 — because its frequency field is documented at two
// widths. Either confirms; the observed one is recorded for diagnostics.
//
// The three other outcomes:
//
//   - A length in the SIBLING TABLE is a wrong radio WITH provisional
//     attribution. The word "provisional" is this driver's own, added by
//     the wrapper below, because driver.WrongRadioError's two rendered
//     formats are fixed in core/driver and the ID-only one is
//     baseline-pinned.
//   - Any OTHER length is a wrong radio WITHOUT attribution: both model
//     fields empty, which is the honest value for a driver that cannot
//     name what it found.
//   - ALL FA is an EMPTY RADIO, not a wrong one. The session opens on
//     ADDRESS EVIDENCE ALONE with Fingerprinted false, and that flag is
//     what stops an unfingerprinted open being mistaken for a confirmed
//     one. That FA means "empty" is ASSUMED: D5 entry 2(a), lift
//     ic905-R-14.
func (s *Session) fingerprint(ctx context.Context, siblings SiblingLengths) error {
	for _, addr := range probeCandidates() {
		record, present, err := s.recordAt(ctx, addr)
		if err != nil {
			var rle *civ.RecordLengthError
			if errors.As(err, &rle) {
				return s.wrongRecordLength(rle, siblings)
			}
			return fmt.Errorf("ic905: Open: record-length fingerprint: %w", err)
		}
		if !present {
			continue
		}
		s.fingerprinted, s.observedLen = true, len(record)
		return nil
	}
	// Every probed slot answered FA. Nothing is wrong; there is simply
	// nothing stored.
	return nil
}

// probeCandidates is the bounded search's address list, in probe order.
func probeCandidates() []civ.ChannelAddress {
	addrs := make([]civ.ChannelAddress, 0, probeSlotsInGroupZero+callSlotsProbed)
	for ch := 0; ch < probeSlotsInGroupZero; ch++ {
		addrs = append(addrs, civ.ChannelAddress{Group: 0, Channel: ch})
	}
	for ch := 0; ch < callSlotsProbed; ch++ {
		addrs = append(addrs, civ.ChannelAddress{Group: callWireGroup, Channel: ch})
	}
	return addrs
}

// wrongRecordLength turns an observed, undeclared record length into the
// refusal spec D3.2 calls for.
//
// THE ATTRIBUTION IS PROVISIONAL AND SAYS SO, IN THIS DRIVER'S OWN WORDS.
// driver.WrongRadioError.Error() has two fixed formats and the ID-only
// one is baseline-pinned, so neither may be edited to carry the
// qualification; the wrapper is where it goes, and the test asserts both
// the wrapped chain and the word.
//
// Want and Got carry the LENGTHS rather than CAT IDs, because on this
// tier the accepted record-length set IS the identity evidence — there is
// no four-character ID to compare (spec D3.2). They are spelled "record
// N" so the rendered "CAT ID" wording cannot be read as a number this
// radio ever printed.
func (s *Session) wrongRecordLength(rle *civ.RecordLengthError, siblings SiblingLengths) error {
	want := make([]string, 0, len(rle.Want))
	for _, n := range rle.Want {
		want = append(want, strconv.Itoa(n))
	}
	wre := &driver.WrongRadioError{
		Want: "record " + strings.Join(want, "/"),
		Got:  "record " + strconv.Itoa(rle.Got),
	}
	model, known := siblings[rle.Got]
	if !known {
		// Branch (b): no attribution. BOTH model fields stay empty, so
		// Error() renders its ID-only text and cmd/rigprog's probe
		// formatter — which keys on GotModel alone — agrees with it.
		return fmt.Errorf("ic905: Open: record-length fingerprint: %w", wre)
	}
	// Branch (a): the length belongs to a model the caller taught this
	// driver about. BOTH fields are populated, because Error() renders
	// its named form only when both are.
	wre.WantModel = civic905.Model
	wre.GotModel = model
	return fmt.Errorf("ic905: Open: record-length fingerprint: %w — attribution is PROVISIONAL: the record lengths this tier compares are themselves ASSUMED derivations, never captured from a radio", wre)
}

// memoryReadSpec is the transport spec for a 1A 00 memory read, assembled
// from E1's helper over the CODEC's own memory-answer matcher. One retry,
// for the reason idSpec gives: a read is idempotent, and a single
// swallowed reply should not fail an operation.
//
// THE MATCHER IS ENVELOPE-ONLY BY DESIGN, and recordAt is what closes the
// gap — see ruling T2 there.
func (s *Session) memoryReadSpec() transport.CommandSpec {
	return civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), 1)
}

// ErrAnswerMismatch is the sentinel a caller should compare against (via
// errors.Is) when a memory answer's decoded channel address is not the
// address that was requested.
//
// IT EXISTS BECAUSE THE CODEC'S MATCHER DELIBERATELY DOES NOT CHECK IT
// (ruling T2; civ.MemoryAnswerMatcher's own doc comment): the matcher
// checks to/from/cn/sc and NOT the requested channel, so an answer for
// group 5 channel 7 satisfies the matcher for a read of group 5 channel
// 6. civ decodes the address and hands it back; comparing it is the
// driver's job, and storing a channel under the wrong slot would corrupt
// a codeplug silently.
var ErrAnswerMismatch = errors.New("ic905: the memory answer names a different channel than was requested")

// AnswerMismatchError reports the requested and the answered address. It
// is this PACKAGE's own typed error in this package's own namespace: the
// other drivers have same-shaped ones and none imports another, because a
// caller distinguishing which radio's read went wrong needs distinct
// types.
type AnswerMismatchError struct {
	// Requested is the address the read asked for.
	Requested civ.ChannelAddress
	// Answered is the address the reply actually decoded to.
	Answered civ.ChannelAddress
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ic905: requested channel %s but the answer names %s — refusing to map a reply onto the wrong channel", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }

// recordAt performs ONE 1A 00 read of addr and returns its RAW record
// bytes, undecoded.
//
// IT IS THE ONE PLACE A MEMORY RECORD ENTERS THIS DRIVER, and that is
// what makes three separate guarantees hold everywhere rather than
// per-caller:
//
//   - THE LENGTH FINGERPRINT IS CONTINUOUS. civ.MemoryAnswerRecord checks
//     the record's length against the profile's accepted set on EVERY
//     call, so a wrong-model session cannot be confirmed once and then
//     trusted; a record at an undeclared length comes back as
//     *civ.RecordLengthError however late it arrives.
//   - THE ANSWER'S ADDRESS IS CHECKED BEFORE ANY USE (ruling T2), because
//     the codec's matcher is envelope-only. The check is here, ahead of
//     empty recognition, field decoding, the E6 template comparison and
//     any write merge.
//   - AN FA IS AN EMPTY CHANNEL, NOT AN ERROR, and the branch keys on
//     errors.Is(err, transport.ErrRejected) — NEVER on "receiving an FA
//     frame" (ruling T4). Engine.Do CONSUMES the FA and returns
//     ErrRejected with NO frame, so a driver that expected a frame back
//     and then asked civ.IsRejection about it could never fire at all.
//     civ.IsRejection stays the framing's internal concern.
//
// present is false for the FA case. The returned slice aliases nothing.
func (s *Session) recordAt(ctx context.Context, addr civ.ChannelAddress) (record []byte, present bool, err error) {
	cmd, err := s.profile.BuildMemoryRead(addr)
	if err != nil {
		return nil, false, fmt.Errorf("ic905: read %s: %w", addr, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.memoryReadSpec())
	if errors.Is(err, transport.ErrRejected) {
		// The ASSUMED empty-channel answer: D5 entry 2(a), lift
		// ic905-R-14.
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ic905: read %s: %w", addr, err)
	}
	got, rec, err := s.profile.MemoryAnswerRecord(frame)
	if err != nil {
		return nil, false, err
	}
	if got != addr {
		s.answerMismatches.Add(1)
		return nil, false, &AnswerMismatchError{Requested: addr, Answered: got}
	}
	return rec, true, nil
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
// individual exchange, the capability set is immutable after Open, the
// one mutable counter is atomic, and the one mutable MAP is behind invMu
// (see markOccupied).
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
	// invMu guards inventory. Discovery fills it before the session is
	// handed out, but a CONFIRMED write adds to it afterwards, so the map
	// is live for the session's whole life and every access goes through
	// here — including discovery's, which needs no lock and takes it
	// anyway rather than leaving one access to be reasoned about
	// separately.
	invMu sync.Mutex
	// inventory is the set of slots this session KNOWS hold a channel:
	// what discovery materialised, plus every slot a confirmed write has
	// since put one in. Rung 11 of the write ladder consults it (ruling
	// T3).
	inventory map[string]bool
	// initDrainCapExceeded records R9-SPLIT branch (b).
	initDrainCapExceeded bool
	// answerMismatches counts memory answers whose decoded address was
	// not the address requested (ruling T2). Atomic because reads may run
	// concurrently.
	answerMismatches atomic.Int64
}

// markOccupied records that slot now holds a channel.
//
// IT IS CALLED ONLY AFTER A CONFIRMED WRITE — the radio's own OK message,
// the one outcome that is a positive acknowledgement rather than an
// absence of complaint — and on no other path. A rejection or an
// unacknowledged set leaves the inventory alone, deliberately: neither
// says the slot holds anything, and an inventory that recorded a
// PRESUMED channel would disarm rung 11 for a slot nothing has read.
//
// The direction is one-way. Nothing removes a slot, because this tier
// ships no erase path at all, and a future one would have to remove here
// rather than leave a stale entry claiming the channel is still there.
func (s *Session) markOccupied(slot string) {
	s.invMu.Lock()
	defer s.invMu.Unlock()
	s.inventory[slot] = true
}

// knownOccupied reports whether this session knows slot holds a channel:
// discovery materialised it, or a confirmed write of this session put one
// there.
func (s *Session) knownOccupied(slot string) bool {
	s.invMu.Lock()
	defer s.invMu.Unlock()
	return s.inventory[slot]
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

// ReadChannel lives in read.go, alongside the slot namespaces and the
// neutral mapping it is made of.

// WriteChannel lives in write.go, alongside the refusal ladder and the
// one preservation read it is made of.
