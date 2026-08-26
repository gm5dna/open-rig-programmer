// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes.
type Option func(*Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose
// SESSIONS carry the consent transform: at session-capability assembly
// every write-side spec.Unverified label becomes spec.ConsentedUnverified,
// which spec.FieldSupport.CanWrite opens.
//
// Consent is a statement about a SESSION, never about the radio: this
// driver's STATIC Capabilities is untouched by the option, and only the
// set Open assembles carries the state. It is deliberately not sufficient
// on its own — an unrecognised Profile stays on the untransformed
// fail-safe even WITH the option, and spec.FieldErase is exempt inside the
// transform itself, so no consent can mint an erase.
func WithConsentedUnverifiedWrites() Option {
	return func(d *Driver) { d.consentUnverifiedWrites = true }
}

// WithFullInventoryWalk makes Open read EVERY address in this radio's
// sparse memory space — all 100 groups of 100 channels — instead of the
// bounded default of the first ten groups.
//
// IT COSTS MINUTES, and the cost is the reason it is opt-in: 10 000
// exchanges against 1 000. The bounded default is a CHOICE, argued (see
// inventory.go): the radio fills groups from the bottom, its budget is 500
// channels against 10 000 addresses, and a user whose memories sit above
// group 10 has this flag. Nothing is silently truncated for the user who
// does not pass it — a write to a slot the bounded walk never visited is
// REFUSED if the radio turns out to hold a record there (ruling T3), never
// overwritten.
func WithFullInventoryWalk() Option {
	return func(d *Driver) { d.fullInventoryWalk = true }
}

// withEngineOptions threads transport options into the engine this driver
// builds at Open.
//
// UNEXPORTED, AND A TEST SEAM: it exists so this package's own tests can
// give the engine a clock whose post-exchange pacing sleep is a no-op,
// because an inventory walk is a thousand exchanges and twenty seconds of
// pure pacing per test is a fact about a serial link rather than about
// this driver. Nothing outside this package can reach it, so it widens no
// public surface; production callers get the engine's real clock, which is
// the only clock New's exported options can produce.
func withEngineOptions(opts ...transport.Option) Option {
	return func(d *Driver) { d.engineOptions = append(d.engineOptions, opts...) }
}

// New builds the IC-705 driver for profile. RealHardware — the ZERO VALUE
// — selects the all-Unverified capability set while writeTrialsComplete is
// false (nothing writable), and ANY unrecognised Profile value
// deliberately selects the same fail-safe: the failure direction for a
// forged or corrupted Profile is always "nothing writable".
func New(profile Profile, opts ...Option) driver.Driver {
	d := &Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Driver implements driver.Driver for the Icom IC-705.
type Driver struct {
	profile                 Profile
	transportLogger         transport.Logger
	consentUnverifiedWrites bool
	fullInventoryWalk       bool
	engineOptions           []transport.Option
}

// Compile-time proof of the seams this driver satisfies.
var (
	_ driver.Driver              = (*Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

// Model implements driver.Driver.
func (d *Driver) Model() string { return capabilitiesUnverified().Model }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile — the model, before any radio has been probed. It
// carries no materialised memory inventory (that is per radio and per
// session; see Session.Capabilities and inventory.go) and it is NEVER the
// consent transform's output, because internal/wiring reads exactly this
// value to decide whether consent is needed at all.
func (d *Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		// No IC-705 has ever been written to by this project:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe. The constant is not READ here,
		// deliberately — see its doc comment for what a flip must change,
		// and why a constant that was load-bearing on its own would let a
		// one-character edit unlock a write.
		return capabilitiesUnverified()
	default:
		// Any unrecognised Profile fails safe through its OWN arm rather
		// than by sharing RealHardware's: the two return the same value
		// today, and a reader must be able to see that the fail-safe is a
		// decision rather than a coincidence of the guard's state.
		return capabilitiesUnverified()
	}
}

// probeSlots is how many memory addresses the OPEN-TIME FINGERPRINT probe
// reads before giving up on finding an occupied one: display slots
// G01-001…G01-016, wire group 0 channels 0-15.
//
// SIXTEEN IS A CHOICE, ARGUED: bounded, inside one group, and well under a
// second at any plausible rate. It is deliberately NOT the inventory walk
// (inventory.go), which is far larger and runs only after this probe has
// decided the radio on the other end is an IC-705 at all — walking a
// thousand addresses before refusing a foreign radio would be a minute
// spent proving something the first answered frame already showed.
const probeSlots = 16

// Open implements driver.Driver: it builds a CI-V engine over port,
// establishes the session (Init sends NOTHING — E1's InitSequence is empty
// — and drains to quiet), probes the radio's address with `19 00`,
// fingerprints the first occupied memory it finds against this profile's
// declared record length, and then discovers this radio's occupied slots
// so the session's sparse MEM bank reports what it actually holds.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
func (d *Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	opts := make([]transport.Option, 0, len(d.engineOptions)+1)
	if d.transportLogger != nil {
		opts = append(opts, transport.WithLogger(d.transportLogger))
	}
	opts = append(opts, d.engineOptions...)

	eng, stats, err := newEngine(port, opts...)
	if err != nil {
		// newEngine has not taken the port on this path (civ.NewFraming
		// refuses before the engine is constructed, and NewEngineWith
		// refuses a nil framing before touching the port), so closing it
		// here is Open's own ownership obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ic705: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, stats, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path closes the engine in
// exactly one place.
func (d *Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	var info SessionInfo

	// THE INIT-UNDER-FLOOD RULE, consumed rather than invented (enabler
	// E1's central rule, spec D2). Init SENDS NOTHING — the CI-V framing's
	// InitSequence is empty, so a session opens without writing one byte
	// to the radio — and then drains the line to quiet. That drain's
	// INITIAL ErrDrainCapExceeded is NONFATAL here: transceive is
	// factory-ON on this radio (Basic Manual rev 9, PDF p.69, folio 8-16,
	// "CI-V Transceive (Default: ON)") and this tier ships no off-switch,
	// so a line that never goes quiet is a NORMAL operating state at open
	// rather than a fault. It is diagnosed on the model surface and the
	// session opens. Every LATER quarantine drain failure stays
	// fail-closed exactly as the engine has it, and nothing here changes
	// that: this branch is reached once, at Init, and never again.
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic705: Open: %w", err)
		}
		info.InitDrainCapExceeded = true
	}

	token, err := probeTransceiverID(ctx, eng)
	if err != nil {
		return nil, fmt.Errorf("ic705: Open: transceiver-ID probe: %w", err)
	}
	info.IDToken = token

	// Identity.CATID carries the ADDRESS, plus the observed token when
	// there was one (spec D3.2; plan O-8). The token is NEVER matched: the
	// `19 00` reply value is undocumented on all six models in this tier
	// (D5 entry 7, lift L-IDTOKEN), so what the probe requires is that an
	// ADDRESS-MATCHED reply arrived at all. A Wave-4 note goes with this:
	// internal/wiring's per-model end-to-end test compares
	// Identity().CATID against Capabilities().CATID for the Yaesu models,
	// and the IC-705 row must compare the ADDRESS HALF only.
	id.CATID = capabilitiesUnverified().CATID
	if token != "" {
		id.CATID += ":" + token
	}

	fingerprinted, err := fingerprintProbe(ctx, eng)
	if err != nil {
		return nil, err
	}
	// An EMPTY RADIO opens on address evidence alone, with the
	// unfingerprinted state recorded (spec D5 entry 2(a), lift
	// L-EMPTY-FA). Nothing rests on the probe alone in any case: the
	// fingerprint is CONTINUOUS, because civ re-validates the record
	// length on every later read, so a wrong-model session cannot be
	// opened once and then trusted.
	info.Fingerprinted = fingerprinted

	s := &Session{
		eng:   eng,
		stats: stats,
		id:    id,
		caps:  d.sessionCapabilities(),
		info:  info,
	}
	// The inventory walk is the LAST thing Open does, and deliberately:
	// it is far the most expensive step (a thousand exchanges by default,
	// ten thousand with WithFullInventoryWalk), so it runs only after the
	// probe has established that the radio on this port answers to this
	// address and holds records of this length. Walking first would spend
	// minutes proving something the first answered frame already showed.
	if err := s.materialiseInventory(ctx, d.fullInventoryWalk); err != nil {
		return nil, err
	}
	return s, nil
}

// probeTransceiverID performs the `19 00` exchange and returns the
// observed ID token, which may legitimately be empty.
//
// AN ADDRESS-MATCHED REPLY IS REQUIRED; ITS VALUE IS NOT. The matcher
// (civ's own, per deviation (a)) checks the envelope — to this controller,
// from this radio, carrying 19 00 — and the token is recorded as a
// diagnostic and compared against nothing. A radio that answers the
// envelope with no data at all has still answered: it has proved the
// address, which is the whole of what this step asks.
func probeTransceiverID(ctx context.Context, eng *transport.Engine) (string, error) {
	p := civic705.Profile()
	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return "", err
	}
	frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), 1))
	if err != nil {
		return "", err
	}
	// The only way this can fail on a frame the matcher accepted is an
	// EMPTY data area, which is a legal envelope and an unusable token.
	token, err := p.ParseTransceiverID(frame)
	if err != nil {
		return "", nil
	}
	return token, nil
}

// fingerprintProbe reads up to probeSlots memory addresses and reports
// whether one of them answered with a record — the LENGTH FINGERPRINT of
// spec D3.2.
//
// It reads at the CODEC level rather than through the session's own
// readRaw, and the reason is the error MAPPING rather than convenience: a
// record whose length this profile does not declare means, HERE, that the
// radio on this port is not an IC-705, and the honest report is a typed
// *driver.WrongRadioError. The same mismatch met later, during the
// inventory walk or an ordinary read, means something quite different — a
// session that was right about the radio has met a record it cannot parse
// — and surfaces as civ's own ErrRecordLength. One helper cannot answer
// both ways, and collapsing them would either refuse a mid-session read as
// "wrong radio" or open a session against a radio whose very first record
// said otherwise.
//
// GotModel is left EMPTY on purpose. Naming the model a foreign length
// belongs to would need the other five models' length sets, and
// cross-model record-length distinctness is a TIER-level Wave-4 check this
// driver must not claim. driver.WrongRadioError renders its ID-only text
// when GotModel is empty, which is exactly the honest sentence here.
func fingerprintProbe(ctx context.Context, eng *transport.Engine) (bool, error) {
	p := civic705.Profile()
	readSpec := civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1)
	for c := 1; c <= probeSlots; c++ {
		slot := spec.SparseSlot(1, c)
		addr, _, err := slotToAddress(slot)
		if err != nil {
			return false, fmt.Errorf("ic705: Open: probe slot %q: %w", slot, err)
		}
		cmd, err := p.BuildMemoryRead(addr)
		if err != nil {
			return false, fmt.Errorf("ic705: Open: probe slot %q: %w", slot, err)
		}
		frame, err := eng.Do(ctx, cmd, readSpec)
		if errors.Is(err, transport.ErrRejected) {
			// An unwritten channel (D5 entry 2(a), ASSUMED, lift
			// L-EMPTY-FA). The FA is an ERROR and never a frame — ruling
			// T4 — so this branch keys on the sentinel and no code in
			// this driver tests for "an FA frame" after Do.
			continue
		}
		if err != nil {
			return false, fmt.Errorf("ic705: Open: probe slot %q: %w", slot, err)
		}
		got, record, err := p.MemoryAnswerRecord(frame)
		if err != nil {
			var lenErr *civ.RecordLengthError
			if errors.As(err, &lenErr) {
				return false, &driver.WrongRadioError{
					Want:      fmt.Sprintf("%s/%d", capabilitiesUnverified().CATID, p.BuildRecordLength()),
					Got:       fmt.Sprintf("%s/%d", capabilitiesUnverified().CATID, lenErr.Got),
					WantModel: capabilitiesUnverified().Model,
				}
			}
			return false, fmt.Errorf("ic705: Open: probe slot %q: %w", slot, err)
		}
		if got != addr {
			return false, &AnswerMismatchError{Requested: addr, Answered: got}
		}
		if allFF(record) {
			// The other unverified empty-channel shape (D5 entry 2(b),
			// lift L-EMPTY-FF): a record of 0xFF bytes. It is recognised
			// here, on the RAW bytes, because 0xFF fails the enum decode —
			// testing for it after parsing would be too late.
			continue
		}
		return true, nil
	}
	return false, nil
}

// allFF reports whether every byte of record is 0xFF — one of the two
// unverified shapes an empty channel may answer with (D5 entry 2(b), lift
// L-EMPTY-FF). An empty slice is NOT all-FF: it is a length this profile
// does not declare, and civ has already refused it.
func allFF(record []byte) bool {
	if len(record) == 0 {
		return false
	}
	for _, b := range record {
		if b != 0xFF {
			return false
		}
	}
	return true
}

// sessionCapabilities is the ONE place a session's effective capability
// set is assembled: this driver's static baseline, then — only when it was
// built with WithConsentedUnverifiedWrites AND its profile is one of the
// declared constants — the consent transform. An unrecognised profile
// stays untransformed even with the option, so the fail-safe direction
// survives consent.
//
// Applying it HERE, before the Session exists, keeps the set WriteChannel
// enforces (s.caps) and the set Capabilities() hands out the same value.
func (d *Driver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// profileRecognised reports whether this driver's profile is one of the
// declared Profile constants — the same set Capabilities' switch names
// explicitly, restated here so the consent gate cannot drift open for a
// profile that switch would fail safe on.
func (d *Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// SessionInfo is the MODEL surface for what a probe and an inventory walk
// learned — everything driver.SessionDiagnostics' single counter cannot
// carry.
//
// It is a model type on a model session, deliberately: the neutral seam
// holds one number because "which diagnostics exist" is a per-driver
// matter (core/driver/optional.go), and an ID token, a fingerprint verdict
// and an inventory-walk mode are three facts about CI-V memory radios that
// no Yaesu driver has an analogue for.
type SessionInfo struct {
	// IDToken is the `19 00` reply's data, as a hex token, or "" when the
	// radio answered the envelope with no data. RECORDED, NEVER MATCHED
	// (D5 entry 7, lift L-IDTOKEN).
	IDToken string
	// Fingerprinted reports whether the open-time probe found an occupied
	// memory whose record length this profile declares. False means the
	// session opened on ADDRESS EVIDENCE ALONE — an empty radio — which
	// is legitimate and recorded rather than refused.
	Fingerprinted bool
	// InitDrainCapExceeded reports that the line never went quiet during
	// Init: the drain hit its absolute cap and the open continued anyway
	// (E1's central rule). It is the flood observation a user deserves to
	// see, and the reason a session on a busy bus is not silently normal.
	InitDrainCapExceeded bool
	// InventoryWalk is which walk Open performed: "bounded" (the first
	// ten groups) or "full" (WithFullInventoryWalk).
	InventoryWalk string
	// InventorySlots is how many occupied memories that walk materialised.
	InventorySlots int
	// AnswerMismatches counts memory answers whose channel address was
	// not the one requested (ruling T2). driver.SessionDiagnostics has
	// one counter and it means something else — frames that reached the
	// engine and matched no spec — so this fact has nowhere else to live,
	// and it is worth surfacing: a non-zero value means the radio, the
	// bus or the cable answered about a channel nobody asked about.
	AnswerMismatches uint64
}

// Session is an IC-705's driver.Session: one open, address-probed,
// inventory-discovered connection.
//
// Safe for concurrent use: transport.Engine serialises every individual
// exchange, the capability set is immutable after Open, and the counters
// below are atomic. There is no operation mutex because no operation this
// session performs is more than ONE wire exchange — except the write,
// which is a read followed by a set, and which takes s.opMu (write.go).
type Session struct {
	eng *transport.Engine
	// stats is the CI-V framing adapter's own counter surface, retained
	// from newEngine because the engine keeps its framing unexported and
	// this is the only value that can answer "how much of this line was
	// broadcast?" — the engine's own counter cannot: the accumulator
	// swallows every transceive frame before the engine sees one.
	stats civ.AccumulatorStatsReporter
	id    driver.Identity
	caps  spec.Capabilities // effective; never mutated after Open
	info  SessionInfo
	// inventory is the set of display slots the open-time walk
	// MATERIALISED — what this radio was observed to hold. It is retained
	// rather than merged and forgotten because ruling T3's
	// occupied-surprise refusal asks a question no capability set can
	// answer: "did the walk actually visit this slot?"
	inventory map[string]bool
	// opMu serialises WriteChannel, which is the one operation here made
	// of TWO exchanges — the E6 preservation read and the memory set. A
	// concurrent write landing between them would decide against one
	// radio state and write against another.
	opMu sync.Mutex
	// mismatches counts T2's refusals. Atomic because a Session is safe
	// for concurrent use and ReadChannel is where it is incremented.
	mismatches atomic.Uint64
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE set — the static
// baseline, plus this radio's own materialised memory inventory, plus
// consent if the user gave it — as a deep copy per call.
func (s *Session) Capabilities() spec.Capabilities { return cloneCapabilities(s.caps) }

// SessionInfo reports what the probe and the inventory walk learned, plus
// the counters this session accrues as it runs. See SessionInfo for why
// this is a model method rather than seam data.
func (s *Session) SessionInfo() SessionInfo {
	info := s.info
	info.AnswerMismatches = s.mismatches.Load()
	return info
}

// Diagnostics reports this session's transport-level health counters,
// satisfying the optional driver.DiagnosticsReporter capability.
//
// IT SUMS TWO COUNTERS, AND THE SECOND IS THE ONE THAT MATTERS HERE. The
// engine counts frames that reached it and matched no spec in force; on a
// CI-V bus that is systematically the wrong question, because the
// accumulator has already dropped every transceive broadcast and every
// other station's traffic BEFORE the engine could see one. A session that
// reported the engine's counter alone would show a healthy zero on a line
// saturated with transceive. Both conversions are guarded: the engine's
// counter is an int64 and the accumulator's an int, and neither may wrap
// into an absurd uint64 even though both are monotonic counters.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	var engine uint64
	if n := s.eng.UnexpectedFrames(); n > 0 {
		engine = uint64(n)
	}
	var dropped uint64
	if n := s.stats.AccumulatorStats().Unexpected; n > 0 {
		dropped = uint64(n)
	}
	return driver.SessionDiagnostics{UnexpectedFrames: engine + dropped}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

// ReadChannel is implemented in read.go; WriteChannel in write.go.

// ErrAnswerMismatch is the sentinel a caller should compare against (via
// errors.Is) when a memory answer names a DIFFERENT channel than the one
// requested.
//
// IT IS THIS DRIVER'S CHECK OR NOBODY'S (ruling T2). The landed
// MemoryAnswerMatcher is deliberately ENVELOPE-ONLY — it matches any
// `1A 00` answer addressed to this controller from this radio — so an
// answer for the wrong channel matches the read in flight. The transport's
// quarantine discipline makes a stale same-shape reply unlikely, and
// "unlikely" is not the standard for silently relabelling one channel's
// contents with another channel's name.
var ErrAnswerMismatch = errors.New("ic705: memory answer names a different channel than was requested")

// AnswerMismatchError reports the requested and the answered address. It
// is this PACKAGE's own typed error, in this package's own namespace: the
// Yaesu drivers have same-shaped ones and none imports another, because a
// caller distinguishing which radio's read went wrong needs distinct
// types.
type AnswerMismatchError struct {
	Requested civ.ChannelAddress
	Answered  civ.ChannelAddress
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ic705: requested channel %v but the answer names %v — refusing to map a reply onto the wrong channel", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }
