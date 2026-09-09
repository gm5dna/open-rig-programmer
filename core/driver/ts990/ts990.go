// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// siblingModelName names the model a Kenwood ID token belongs to, for a
// wrong-radio refusal, and returns "" where no document this project reads
// prints a name.
//
// THIS MILESTONE COMPLETES THE FAMILY, which is why the table is four rows
// long rather than falling through to an unknown-model sentence: 020 is the
// TS-480 (built and deliberately unregistered), 021 and 023 are pair 1's two
// 590 rows, and 024 is this pair's other member. All four are printed with
// their model names beside them in a book this project has read, so all four
// can be NAMED — and an owner who has plugged in the wrong radio is told
// which one it is.
//
// 022 IS ABSENT BECAUSE IT IS THIS ROW'S OWN and never reaches here. It is
// also the one identity in the family whose book prints it BARE (990:2612),
// which is what makes this row's own model name a project choice rather than
// a transcription — see modelName.
//
// A token outside the table leaves GotModel empty, and
// driver.WrongRadioError renders its ID-only sentence: supplying a name on
// the strength of nothing would put a name in a manufacturer's mouth.
// TestOpen_AnUnprintedIdentityIsRefusedWithoutInventingAName pins that text.
func siblingModelName(id string) string {
	switch id {
	case "020":
		// Built by pair 1 and deliberately NOT registered.
		return "TS-480"
	case "021":
		return "TS-590S"
	case "023":
		return "TS-590SG"
	case "024":
		// This pair's other member, printed WITH its name — "024: TS-890S"
		// (890:2733) — which 022 is not.
		return "TS-890S"
	default:
		return ""
	}
}

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ts990Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a caller
// of this driver to receive them. A nil l is ignored (the engine's default is
// kept). TestWithTransportLogger_ReachesTheEngine is the pin.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ts990Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose SESSIONS
// carry the consent transform: at session-capability assembly every write-side
// spec.Unverified label becomes spec.ConsentedUnverified, which
// FieldSupport.CanWrite opens.
//
// Consent is a statement about a SESSION, never about the radio: this driver's
// STATIC Capabilities is untouched by the option, and only the set Open
// assembles carries the state. It is deliberately not sufficient on its own —
// an unrecognised Profile stays on the untransformed fail-safe even WITH the
// option, spec.FieldErase is exempt inside the transform itself, and nothing
// here consults writeTrialsComplete.
//
// CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY (matrix §2.1):
// every pre-wire refusal of the write ladder still fires ahead of it.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ts990Driver) { d.Consented = true }
}

// withTiming overrides the transport deadlines every read this driver's
// sessions issue carries. It exists for FOCUSED TESTS against a net.Pipe,
// which answers instantly and would otherwise spend the fleet's radio-paced
// DefaultSettle on every frame of a hundred-slot bank walk; production
// deliberately takes the transport's own defaults until a TS-990S has been
// measured. The core/driver/ts590 shape.
func withTiming(readTimeout, settle time.Duration) Option {
	return func(d *ts990Driver) {
		d.readTimeout, d.settle = readTimeout, settle
	}
}

// New builds the TS-990S driver for profile. RealHardware — the ZERO Profile
// — selects the all-Unverified capability set while writeTrialsComplete is
// false, and ANY unrecognised Profile value deliberately selects the same
// fail-safe. Options: WithTransportLogger, WithConsentedUnverifiedWrites.
//
// THERE IS NO Row ARGUMENT, and its absence is the design rather than a
// simplification (spec decision 2, matrix §5). core/driver/ts590 serves two
// rows from one package because the TS-590S and the TS-590SG share a book, a
// 50-byte grid, a mode legend and both tone charts. This package's radio
// shares none of those with its sibling: seventeen divergences, of which the
// frame length, the parameter count, the mode legend, the number of tone
// tuples, the Main/Sub grammar and the channel-type byte are each on their own
// sufficient reason for two packages.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ts990Driver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ts990Driver implements driver.Driver for the TS-990S.
type ts990Driver struct {
	driver.Base
	// transportLogger is threaded into every Session's transport.Engine at
	// Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// Non-zero only in focused tests; see withTiming.
	readTimeout time.Duration
	settle      time.Duration
}

// Model implements driver.Driver.
func (d *ts990Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile. There is NO discovery on any Kenwood row (matrix §3.4),
// so a Session's effective set differs from this one only by the consent
// transform.
func (d *ts990Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No TS-990S has ever been written to over its PC-command port by
		// this project — none has ever been ASKED anything at all:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select.
		return CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence of the flip's state.
		return CapabilitiesUnverified()
	}
}

// StopBits implements driver.SerialFramingReporter, and it is a SESSION
// PRECONDITION rather than a nicety (matrix §3.1, plan P10):
// transport.DefaultStopBits is 2 — the Yaesu family's framing — and this book
// specifies ONE, in a table this driver's value is read from: "Start Bit 1"
// (990:16), "Data Bit 8" (990:17), "Stop Bit 1" (990:18), "Parity Bit None"
// (990:20). A driver that omitted this interface would open every session at
// the wrong framing and fail like a dead port.
//
// THE ONE PRINTED EXCEPTION COSTS NOTHING, and stating why is the point: the
// stop-bit row's own parenthesis is "2 is available only when using 4800 bps"
// (990:18), so the answer would be conditional if 4800 were on offer — and it
// is not (§1.11, for its own reason, the USB-B exclusion at 990:23). No
// session this driver opens can be at that rate, so this answer is
// unconditional rather than a simplification.
func (d *ts990Driver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe: the prefix, this family's
// own SIX-byte answer length, and one retry — an identity read is idempotent
// and Open should survive a single swallowed reply.
//
// THE LENGTH IS core/kw's, never a literal here. kw.IDAnswerLen is six where
// core/cat's is seven, which is one of the dialect axes that make this a
// separate codec at all.
func (d *ts990Driver) idSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("ID", kw.IDAnswerLen), 1)
}

// fvSpec is the transport spec for the "FV;" probe, on idSpec's terms.
func (d *ts990Driver) fvSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("FV", kw.FVAnswerLen), 1)
}

// readSpec builds a read spec around the ANSWER MATCHER its caller supplies,
// applying this driver's test-only timing overrides in ONE place.
//
// THE MATCHER IS A PARAMETER RATHER THAN A PREFIX AND A LENGTH even though on
// this row every matcher is in fact a prefix and a length. read.go's ma0Spec
// passes kw.PrefixLenMatcher("MA0"+slot, 57) and the probes pass their own, so
// the parameter buys nothing today — what it buys is that each command states
// its own correlation rule at its own call site, which is where the sibling
// row's length RANGE lives and where a reader will look for this one's exact
// width.
func (d *ts990Driver) readSpec(match func(frame []byte) bool, retries int) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      match,
		RetryReads: retries,
		Timeout:    d.readTimeout,
		Settle:     d.settle,
	}
}

// wireFailure types the TWO WIRE EVENTS a Kenwood exchange can meet, so every
// frame this driver sends reports them the same way: a "?;" as
// *kw.RejectionError, which names the two indistinguishable causes the book
// prints for it and cites the transient-suppression sentence, and silence as
// *kw.TimeoutError, which says in as many words that it is not an inference of
// absence.
//
// IT IS ONE FUNCTION BECAUSE THE RULE IS ONE RULE, and the probe's ID; and FV;
// are frames like any other: a reader who met the typed error on an MA0 and
// the transport's bare error on an ID would reasonably conclude the two wire
// events mean something different there.
//
// Anything else is returned unchanged, as is either wire event on a book
// core/kw does not name — unreachable, since this driver's book is a constant,
// and an error quoting a document this session was never told it was speaking
// to is what kw's fallible constructors exist to prevent.
func wireFailure(command string, err error) error {
	switch {
	case errors.Is(err, transport.ErrRejected):
		if rej, nerr := kw.NewRejectionError(kw.Book990, command); nerr == nil {
			return rej
		}
	case errors.Is(err, transport.ErrTimeout):
		if to, nerr := kw.NewTimeoutError(kw.Book990, command); nerr == nil {
			return to
		}
	}
	return err
}

// Open implements driver.Driver: it builds a transport.Engine over port bound
// to this row's codec layout, establishes the session (Init: AI0; and
// drain-to-quiet), then runs the matrix §3.5 identity probe — "ID;", and on
// this row's own identity "FV;".
//
// THREE FRAMES GO OUT AND THE PROBE IS TWO OF THEM (plan P9). AI0; is
// transport.Engine.Init's own frame and the matrix files it under §3.6 as
// session init rather than as part of the probe. A WRONG RADIO therefore
// receives exactly two frames — the preamble and "ID;" — and nothing more.
//
// THE AI0; IS PER CONNECTOR, and that is the one operational fact this
// choreography carries into radiotext: "The AI function can be set separately
// for USB connector, COM connector, or LAN connector" (990:187-188), so a
// session that turns AI off does so on the connector it is using and leaves
// the other two alone — a user watching a LAN-connected logger will not see it
// stop.
//
// NO DISCOVERY FRAME OF ANY KIND IS EVER BUILT (matrix §3.4), on two
// independent grounds: the book says the NAK is unreliable, so a probe's
// silence carries no information at all; and the slot space is fully printed
// (990:2893-2896), so a probe would be asking a question the book answers.
// Every bank is static.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close releases
// it on success, and Open itself closes it before returning an error.
func (d *ts990Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	layout := layout()
	framing, err := ma.NewFramingFor(layout)
	if err != nil {
		// NEVER kw.NewFraming(book): that is the envelope-only framing,
		// whose gate admits any well-formed Kenwood frame — including the
		// MA5 deletion shape this programme refuses to build.
		// NewFramingFor puts this row's own grammars in front of the
		// envelope, so the outbound gate a session enforces is the one
		// belonging to the radio it is for.
		_ = port.Close()
		return nil, fmt.Errorf("ts990: Open: framing: %w", err)
	}
	var opts []transport.Option
	if d.transportLogger != nil {
		opts = append(opts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngineWith(port, framing, opts...)
	if err != nil {
		// NewEngineWith has not taken the port on this path, so closing it
		// here is Open's own ownership obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ts990: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, layout, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in exactly
// one place.
func (d *ts990Driver) open(ctx context.Context, eng *transport.Engine, layout ma.Layout, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts990: Open: %w", err)
	}

	got, err := d.probeID(ctx, eng, layout)
	if err != nil {
		return nil, err
	}
	if got != catID {
		// WantModel is always populated — this driver knows its own name;
		// GotModel only where a document prints one (siblingModelName).
		// Nothing more is sent and the port closes.
		return nil, &driver.WrongRadioError{
			Want:      catID,
			Got:       got,
			WantModel: modelName,
			GotModel:  siblingModelName(got),
		}
	}
	id.CATID = got

	if err := d.probeFV(ctx, eng, layout); err != nil {
		return nil, err
	}

	return &Session{
		eng:         eng,
		layout:      layout,
		id:          id,
		caps:        d.SessionCaps(d.Capabilities()),
		newReadSpec: d.readSpec,
	}, nil
}

// probeID sends "ID;" and returns the three printed digits it answered.
func (d *ts990Driver) probeID(ctx context.Context, eng *transport.Engine, layout ma.Layout) (string, error) {
	cmd, err := layout.BuildIDRead()
	if err != nil {
		return "", fmt.Errorf("ts990: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return "", fmt.Errorf("ts990: Open: ID probe: %w", wireFailure("ID", err))
	}
	got, err := layout.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts990: Open: ID probe: %w", err)
	}
	return got, nil
}

// probeFV sends "FV;" and DISCARDS the four characters it answers with.
//
// THE FRAME IS SENT FOR WHAT ITS ABSENCE WOULD MEAN, NOT FOR ITS CONTENT, and
// that is this row's difference from pair 1 (matrix §3.5). The TS-590S gates
// byte 28's write policy on its firmware version, so that driver carries the
// answer into the session and publishes it. NOTHING ON THIS ROW BRANCHES ON
// THE VERSION: this radio has no field whose meaning depends on firmware, and
// the only firmware sentence in the whole book concerns an AI parameter this
// programme never sets (990:189). Keeping a value nothing consults would be a
// surface a later reader would have to check for callers.
//
// What the frame still does is refuse a session. The EXISTENCE of FV is
// printed for this row, with a Read chart, an Answer chart and a worked
// example — "for firmware version 1.00, it reads FV1.00;" (the block at
// 990:2527-2536; the Read grid at 990:2532, the Answer grid at 990:2536 and
// the example at 990:2533) — so
// a radio that answered "ID;" with 022 and then rejects or ignores "FV;" is
// outside a document that is complete here, and the standing rule is to REFUSE
// where the document is complete and we are outside it. The 7-byte structural
// parse in between (kw.FVAnswerLen; six is the ID answer's, kw.IDAnswerLen) is
// the ENVELOPE's, so its failure is a malformed frame and fails Open like any
// other.
//
// A13 — that the four-character grid holds for every firmware this radio has
// shipped — is therefore load-bearing on the ENVELOPE here and on nothing
// else: a five-character version would fail Open, where on pair 1 it merely
// took the conservative branch of a write gate.
func (d *ts990Driver) probeFV(ctx context.Context, eng *transport.Engine, layout ma.Layout) error {
	cmd, err := layout.BuildFVRead()
	if err != nil {
		return fmt.Errorf("ts990: Open: FV probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.fvSpec())
	if err != nil {
		return fmt.Errorf("ts990: Open: FV probe: %w", wireFailure("FV", err))
	}
	if _, err := layout.ParseFVAnswer(frame); err != nil {
		return fmt.Errorf("ts990: Open: FV probe: %w", err)
	}
	return nil
}

// Session is one open, identity-probed TS-990S connection. Safe for
// concurrent use.
//
// IT CARRIES AN OPERATION MUTEX (plan P12/P13). transport.Engine serialises
// each individual EXCHANGE, not a whole driver operation, and this driver's
// operations are not all single exchanges: Open's probe is three frames, and
// T14's write is a read and a Set that must decide against ONE radio state.
// opMu guards ONE DRIVER OPERATION, so a concurrent caller cannot land in the
// middle of one — and holding it for every operation, rather than only for the
// several-frame ones, is what makes the rule "one driver operation at a time"
// rather than "one driver operation at a time, except the short ones".
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to core/clone, as
// the driver seam assigns it, and holding a driver lock across it would
// serialise two operations the seam deliberately keeps separate.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// layout is the codec this session encodes and decodes with. It is
	// copied from the driver that Opened it — which is also where the
	// engine's outbound gate came from — so a session can never encode with
	// one book's layout while its transport gates with another's.
	layout ma.Layout
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
	// newReadSpec builds this session's read specs, carrying the driver's
	// test-only timing overrides. A func rather than the two durations so
	// there is one construction site for a CommandSpec in this package.
	// NAMED FOR THE PACKAGE IT MUST NOT BE MISTAKEN FOR: these files also
	// use spec.Bank and spec.Capabilities two lines away.
	newReadSpec func(match func(frame []byte) bool, retries int) transport.CommandSpec
}

// Identity implements driver.Session: the probed CATID plus the
// caller-supplied port path and USB serial from Open.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the effective capability set as a
// deep copy per call. The copy is load-bearing for the write gate exactly as
// in the sibling drivers — a caller mutating what it was handed must never
// alter what WriteChannel enforces.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable. It satisfies the optional
// driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// WriteChannel implements driver.Session — AND IS A PLACEHOLDER THAT TASK 14
// REPLACES WHOLE. It exists in this file because driver.Session requires the
// method and *Session must satisfy that interface for Open to return one at
// all; it deliberately does NOT attempt a partial choreography. Pair 1's
// core/driver/ts590 carried the identical placeholder for the identical
// reason, and its own write task replaced it and its pin together.
//
// Every call is refused with a typed *driver.WriteRefusedError BEFORE any
// frame is built or any byte reaches the wire — which is the correct behaviour
// for the RealHardware and fail-safe profiles regardless (their capability
// gate would refuse anyway, writeTrialsComplete being false), and a temporary,
// visible gap for the Simulated profile.
//
// What replaces it is the eleven-rung ladder of plan P7 in that order — the
// slot, the bank, the EMPTY candidate, driver.CheckFieldStates, THE CAPABILITY
// GATE, then this row's own pre-wire refusals — and then, after ONE pre-write
// MA0 read held with the Set under opMu, the two read-dependent rungs: the
// empty predicate, and the secondary-side comparison that refuses when any of
// P10-P14 differs from what the Set would emit and refuses UNCONDITIONALLY
// when P16 = 1. Then one 57-byte MA0 Set, reported Sent, never Confirmed.
// TestWriteChannel_RefusedUntilTask14 pins this placeholder and is replaced
// along with it.
func (s *Session) WriteChannel(_ context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{
		Slot:   ch.Slot,
		Reason: "the TS-990S driver's write path is not implemented yet (Stage 2 task 14 lands the single MA0 Set and its refusal ladder); no frame is built and nothing reaches the wire",
	}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close already
// guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }
