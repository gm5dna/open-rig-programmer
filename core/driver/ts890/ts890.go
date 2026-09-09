// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// siblingModelName names the model a Kenwood ID token belongs to, for a
// wrong-radio refusal, and returns "" where this project has no name to give.
//
// ALL FIVE PRINTED KENWOOD IDENTITIES ARE NOW KNOWN TO THE PROGRAMME (plan
// P9), so a probe answering any of them is refused BY NAME rather than
// falling through to an unknown-model sentence. This book prints exactly one
// of the five — "024: TS-890S" (890:2733) — and the other four are named from
// the rows this project itself registers or builds, which is a statement
// about this programme's registry and not an inference about a manufacturer's
// spelling from a book that does not carry it.
//
// 022 IS NAMED HERE AND IS NOT NAMED IN core/driver/ts590, and the difference
// is real rather than an inconsistency. That package left it blank because at
// pair 1 the token appeared only in another book, printed BARE with no model
// name beside it (990:2612), so supplying one would have been a cross-
// document inference. THIS MILESTONE BINDS 022 TO "TS-990S" AS A REGISTRY KEY
// — a project CHOICE over a documentary silence, recorded as such in matrix
// §1.1 — and naming this row's own sibling by the key the programme gives it
// is what P9 asks for. core/driver/ts990 mints the same string as its own
// modelName; this one is a refusal message and binds nothing.
//
// 024 IS ABSENT DELIBERATELY: it is this row's own identity, which the probe
// ACCEPTS, so no wrong-radio error is ever built for it.
func siblingModelName(id string) string {
	switch id {
	case "020":
		// Built by pair 1 and deliberately NOT registered.
		return "TS-480"
	case "021":
		return "TS-590S"
	case "022":
		return "TS-990S"
	case "023":
		return "TS-590SG"
	default:
		return ""
	}
}

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ts890Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a caller
// of this driver to receive them. A nil l is ignored (the engine's default is
// kept).
//
// THIS DRIVER ALSO WRITES ONE LINE OF ITS OWN TO IT, and that is A21's
// requirement rather than a convenience: an empty TS-890S channel may carry a
// residue in its name window, and the residue must be CARRIED rather than
// discarded (plan P14). The logger is the only sink a driver has —
// codeplug.Channel has no per-channel note and driver.SessionDiagnostics is a
// counter — so the read path reports it there. See read.go.
//
// The note is reachable only when a caller supplies a logger; nothing
// outside core/driver does so today, so T17's registration must wire one
// for this row or the residue is unobservable in the shipping programme.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ts890Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose SESSIONS
// carry the consent transform: at session-capability assembly every
// write-side spec.Unverified label becomes spec.ConsentedUnverified, which
// FieldSupport.CanWrite opens.
//
// Consent is a statement about a SESSION, never about the radio: this
// driver's STATIC Capabilities is untouched by the option, and only the set
// Open assembles carries the state. It is deliberately not sufficient on its
// own — an unrecognised Profile stays on the untransformed fail-safe even
// WITH the option, spec.FieldErase is exempt inside the transform itself, and
// nothing here consults writeTrialsComplete.
//
// CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY (matrix §2.1):
// every pre-wire refusal of the write ladder still fires ahead of it.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ts890Driver) { d.Consented = true }
}

// withTiming overrides the transport deadlines every read this driver's
// sessions issue carries. It exists for FOCUSED TESTS against a net.Pipe,
// which answers instantly and would otherwise spend the fleet's radio-paced
// DefaultSettle on every frame of a 100-slot bank walk; production
// deliberately takes the transport's own defaults until a TS-890S has been
// measured. The core/driver/ts590 shape.
func withTiming(readTimeout, settle time.Duration) Option {
	return func(d *ts890Driver) {
		d.readTimeout, d.settle = readTimeout, settle
	}
}

// New builds the TS-890S driver for profile. RealHardware — the ZERO Profile
// — selects the all-Unverified capability set while writeTrialsComplete is
// false, and ANY unrecognised Profile value deliberately selects the same
// fail-safe. Options: WithTransportLogger, WithConsentedUnverifiedWrites.
//
// THERE IS NO Row ARGUMENT, and that is the structural difference from
// core/driver/ts590: this package serves ONE registry row, because the
// TS-890S and the TS-990S have separate books and separate MA0 grids with
// nothing to share.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ts890Driver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ts890Driver implements driver.Driver for the TS-890S.
type ts890Driver struct {
	driver.Base
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine — see WithTransportLogger.
	transportLogger transport.Logger
	// Non-zero only in focused tests; see withTiming.
	readTimeout time.Duration
	settle      time.Duration
}

// Model implements driver.Driver: the registry key, which is necessarily
// equal to Capabilities().Model (core/driver.Driver's contract).
func (d *ts890Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile. There is NO discovery on any Kenwood row (matrix §3.4),
// so a Session's effective set differs from this one only by the consent
// transform.
func (d *ts890Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No TS-890S has ever been written to over its PC-command port by
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
// transport.DefaultStopBits is 2 — the Yaesu family's framing — and this
// book specifies ONE: "Start Bit 1", "Data Bit 8", "Stop Bit 1", "Parity Bit
// None" (890:15-18). A driver that omitted this interface would open every
// session at the wrong framing and fail like a dead port.
//
// The book's parenthesis — two stop bits ARE available, but only at 4800 bps
// (890:17) — costs nothing, because 4800 is not published (§1.11): the rate
// is omitted from this row rather than offered with framing this transport
// would not set.
//
// THE SECOND LEG OF P10 IS NOT REACHABLE FROM THIS PACKAGE. internal/wiring's
// stopBitsFor is unexported, so what a registered row's SESSION actually
// opens with is pinned at registration (T17); this test's half is the
// concrete-driver assertion, and it compares drv.StopBits() rather than
// type-switching, because a type switch passes when the interface is absent.
func (d *ts890Driver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe: the prefix, this family's
// own SIX-byte answer length, and one retry — an identity read is idempotent
// and Open should survive a single swallowed reply.
//
// THE LENGTH IS core/kw's, never a literal here. kw.IDAnswerLen is six where
// core/cat's is seven, and this book prints the same six: "ID;" is three
// bytes and its answer "I D P1 P1 P1 ;" (890:2735, 890:2738).
func (d *ts890Driver) idSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("ID", kw.IDAnswerLen), 1)
}

// fvSpec is the transport spec for the "FV;" probe, on idSpec's terms. The
// answer is a fixed SEVEN bytes carrying four characters (890:2653-2659), so
// a prefix-and-exact-length matcher is exact.
func (d *ts890Driver) fvSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("FV", kw.FVAnswerLen), 1)
}

// readSpec builds a read spec around the ANSWER MATCHER its caller supplies,
// applying this driver's test-only timing overrides in ONE place.
//
// THE MATCHER IS A PARAMETER RATHER THAN A PREFIX AND A LENGTH, because this
// row has two matching rules and not one. ID, FV and the EX read are
// correlated by kw.PrefixLenMatcher on a prefix and an EXACT width. The MA0
// answer is not: its terminator floats at 40 + len(name) (890:3181-3182), so
// its width is a RANGE of 40 to 50, and read.go passes
// ma.Layout.MA0AnswerMatcher instead. Taking the predicate here keeps one
// construction site for a CommandSpec in this package while letting each
// command state its own correlation rule.
func (d *ts890Driver) readSpec(match func(frame []byte) bool, retries int) transport.CommandSpec {
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
// *kw.TimeoutError, which says in as many words that it is not an inference
// of absence.
//
// IT IS ONE FUNCTION BECAUSE THE RULE IS ONE RULE, and the probe's ID; and
// FV; are frames like any other: a reader who met the typed error on an MA0
// and the transport's bare error on an ID would reasonably conclude the two
// wire events mean something different there.
//
// Anything else is returned unchanged, as is either wire event on a layout
// naming no book — unreachable, since a configured layout always names one,
// and an error quoting a document this session was never told it was speaking
// to is what kw's fallible constructors exist to prevent.
func wireFailure(book kw.Book, command string, err error) error {
	switch {
	case errors.Is(err, transport.ErrRejected):
		if rej, nerr := kw.NewRejectionError(book, command); nerr == nil {
			return rej
		}
	case errors.Is(err, transport.ErrTimeout):
		if to, nerr := kw.NewTimeoutError(book, command); nerr == nil {
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
// transport.Engine.Init's own frame, and the matrix files it under §3.6 as
// session init rather than as part of the probe. A WRONG RADIO therefore
// receives exactly two frames — the preamble and "ID;" — and nothing more.
//
// THE AI SET'S SCOPE IS PER CONNECTOR and is worth stating because a user can
// see it: this radio has three (COM, USB-B and LAN), a session that sends
// AI0; turns Auto Information off on the connector it is using, and the other
// two are left alone.
//
// NO DISCOVERY FRAME OF ANY KIND IS EVER BUILT (matrix §3.4), on two
// independent grounds: the book says the NAK is unreliable, so a probe's
// silence carries no information at all; and the slot space is fully printed,
// "000 ~ 119" (890:3167), so a probe would be asking a question the book
// answers. Every bank is static.
//
// NOTHING BRANCHES ON THE FIRMWARE VERSION ON THIS ROW, and that is a
// difference from pair 1 worth recording: the 590S's byte 28 policy turns on
// FV, and this radio has no field whose meaning depends on firmware. FV is
// probed because the book prints it complete — a Read chart, an Answer chart
// and a worked example, "FV1.00;" (890:2653-2659) — and the standing rule is
// to refuse where the document is complete and we are outside it.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close releases
// it on success, and Open itself closes it before returning an error.
func (d *ts890Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	l := layout()
	framing, err := ma.NewFramingFor(l)
	if err != nil {
		// NEVER kw.NewFraming(book): that is the envelope-only framing,
		// whose gate admits any well-formed Kenwood frame — including an
		// MA5 deletion. ma.NewFramingFor puts this row's own grammars in
		// front of the envelope, so the outbound gate a session enforces is
		// the one belonging to the radio it is for.
		_ = port.Close()
		return nil, fmt.Errorf("ts890: Open: framing: %w", err)
	}
	opts := []transport.Option{}
	if d.transportLogger != nil {
		opts = append(opts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngineWith(port, framing, opts...)
	if err != nil {
		// NewEngineWith has not taken the port on this path, so closing it
		// here is Open's own ownership obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ts890: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, l, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in exactly
// one place.
func (d *ts890Driver) open(ctx context.Context, eng *transport.Engine, l ma.Layout, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts890: Open: %w", err)
	}

	got, err := d.probeID(ctx, eng, l)
	if err != nil {
		return nil, err
	}
	if got != catID {
		// WantModel is always populated — this driver knows its own row's
		// name; GotModel wherever this programme has a name for the token
		// (see siblingModelName). Nothing more is sent and the port closes.
		return nil, &driver.WrongRadioError{
			Want:      catID,
			Got:       got,
			WantModel: modelName,
			GotModel:  siblingModelName(got),
		}
	}
	id.CATID = got

	fv, err := d.probeFV(ctx, eng, l)
	if err != nil {
		return nil, err
	}

	return &Session{
		eng:         eng,
		layout:      l,
		id:          id,
		caps:        d.SessionCaps(d.Capabilities()),
		newReadSpec: d.readSpec,
		logger:      d.transportLogger,
		fvAnswer:    fv,
	}, nil
}

// probeID sends "ID;" and returns the three printed digits it answered.
func (d *ts890Driver) probeID(ctx context.Context, eng *transport.Engine, l ma.Layout) (string, error) {
	cmd, err := l.BuildIDRead()
	if err != nil {
		return "", fmt.Errorf("ts890: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return "", fmt.Errorf("ts890: Open: ID probe: %w", wireFailure(l.Book(), "ID", err))
	}
	got, err := l.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts890: Open: ID probe: %w", err)
	}
	return got, nil
}

// probeFV sends "FV;" and returns P1's four characters VERBATIM.
//
// THE ANSWER IS CARRIED AND NOT PARSED, which is the whole difference from
// pair 1. The 590 rows read the version as A13's assumed M.NN form because
// their write gate consults it; NOTHING ON THIS ROW BRANCHES ON IT (matrix
// §3.5), so there is no grammar to assume and no refusal to hang on a failed
// reading. That the four-character grid holds for every firmware this radio
// has shipped is A13, whose lift is L-HW-10 and which this driver does not
// depend on: the four characters go into the session verbatim whatever they
// spell.
//
// A frame that does not arrive is different: the book is COMPLETE here, so a
// transport failure fails Open like any other. The 7-byte structural parse —
// prefix, width, printable ASCII (kw.FVAnswerLen; six is the ID answer's
// length) — is the ENVELOPE's, so its failure is a malformed frame.
func (d *ts890Driver) probeFV(ctx context.Context, eng *transport.Engine, l ma.Layout) (string, error) {
	cmd, err := l.BuildFVRead()
	if err != nil {
		return "", fmt.Errorf("ts890: Open: FV probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.fvSpec())
	if err != nil {
		return "", fmt.Errorf("ts890: Open: FV probe: %w", wireFailure(l.Book(), "FV", err))
	}
	answer, err := l.ParseFVAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts890: Open: FV probe: %w", err)
	}
	return answer, nil
}

// Session is one open, identity-probed TS-890S connection. Safe for
// concurrent use.
//
// IT CARRIES AN OPERATION MUTEX (plan P13). transport.Engine serialises each
// individual EXCHANGE, not a whole driver operation, and this driver's
// operations are not all single exchanges: Open's probe is three frames, the
// channel WRITE is a read and a Set that must not be split (T12), and the
// settings read is another. opMu guards ONE DRIVER OPERATION, so a concurrent
// caller cannot land in the middle of one. Every operation takes it, rather
// than only the several-frame ones, so the rule is "one driver operation at a
// time" and not "one driver operation at a time, except the short ones".
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to core/clone,
// as the driver seam assigns it, and holding a driver lock across it would
// serialise two operations the seam deliberately keeps separate.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// layout is the radio this session is for. It is copied from the driver
	// that Opened it — which is also where the engine's outbound gate came
	// from — so a session can never encode with one layout while its
	// transport gates with another.
	layout ma.Layout
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
	// newReadSpec builds this session's read specs, carrying the driver's
	// test-only timing overrides. A func rather than the two durations so
	// there is one construction site for a CommandSpec in this package.
	// NAMED FOR THE PACKAGE IT MUST NOT BE MISTAKEN FOR: these files also
	// use spec.Bank and spec.Capabilities two lines away.
	newReadSpec func(match func(frame []byte) bool, retries int) transport.CommandSpec
	// logger is the driver's own note sink, nil unless WithTransportLogger
	// was given. read.go uses it for A21's name residue; nothing else in
	// this package writes to it.
	logger transport.Logger

	// fvAnswer is the FV probe's P1, four characters VERBATIM. Read out
	// through FirmwareAnswer.
	fvAnswer string
}

// Identity implements driver.Session: the probed CATID plus the
// caller-supplied port path and USB serial from Open.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the effective capability set as a
// deep copy per call. The copy is load-bearing for the write gate exactly as
// in the sibling drivers — a caller mutating what it was handed must never
// alter what WriteChannel enforces.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// FirmwareAnswer returns the FV probe's P1 EXACTLY AS THE RADIO ANSWERED IT —
// the four characters verbatim — and "" only on a session that never got one,
// which Open makes unreachable: an FV that does not answer refuses the
// session.
//
// IT EXISTS BECAUSE THE ANSWER IS A REPORTED FACT, NOT A PRIVATE ONE, and on
// this row it is ONLY that: nothing here branches on the version (matrix
// §3.5), so the accessor's whole purpose is that an owner can see what their
// radio said. Without it the datum is unreachable outside this package and no
// probe note can carry it. Rendering it is the registration task's.
func (s *Session) FirmwareAnswer() string { return s.fvAnswer }

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable. It satisfies the optional
// driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close already
// guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }
