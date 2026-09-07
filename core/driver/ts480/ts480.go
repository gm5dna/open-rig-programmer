// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Compile-time assertions that this package still satisfies every seam
// internal/wiring will type-assert for on the day this row registers. They are
// here rather than in a test because a lost interface still COMPILES: the
// registry would simply stop finding the capability.
// The settings pair — driver.StaticSettingsProvider and driver.SettingsReader
// — are asserted in settings.go, beside the methods that satisfy them.
var (
	_ driver.Driver                = (*ts480Driver)(nil)
	_ driver.SerialFramingReporter = (*ts480Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// siblingModelName names the model a Kenwood ID token belongs to, for a
// wrong-radio refusal, and returns "" where no document this milestone reads
// prints a name.
//
// 022 IS DELIBERATELY UNNAMED. The design requires 022 and 024 to be named
// explicitly when either is what answered, and both ARE — by their ID, which
// is the refusal's own Got field. What differs is that 024 additionally has a
// printed model name, "024: TS-890S" in the TS-890S book, while 022 is
// "printed bare in the TS-990S book with no model name beside it" (matrix
// §1.2). Supplying "TS-990S" here would put a name in a manufacturer's mouth
// on the strength of which book the token was found in, which is exactly the
// cross-document inference this milestone refuses; driver.WrongRadioError
// renders its ID-only sentence for an empty GotModel, and TestOpen_WrongRadio
// pins both shapes.
func siblingModelName(id string) string {
	switch id {
	case catID:
		return modelName
	case "021":
		return "TS-590S"
	case "023":
		return "TS-590SG"
	case "024":
		// The tier's second pair, which this milestone does not build.
		return "TS-890S"
	default:
		return ""
	}
}

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes.
type Option func(*ts480Driver)

// WithConsentedUnverifiedWrites records that the USER has consented to writing
// this radio's Unverified fields, and builds a driver whose SESSIONS carry the
// consent transform: at session-capability assembly every write-side
// spec.Unverified label becomes spec.ConsentedUnverified, which
// FieldSupport.CanWrite opens.
//
// ON THIS ROW IT REACHES NOTHING, AND THE OPTION EXISTS ANYWAY. A22 refuses
// every TS-480 channel write (write.go, decision 12), so a consented session
// passes the capability gate and is then refused one rung lower. That is the
// point rather than a waste: without the option there would be no way to reach
// A22 at all on a RealHardware session, and "every write is refused" would be
// satisfied vacuously by the capability gate — which is the one thing this
// row's whole write path exists to disprove.
//
// Consent is a statement about a SESSION, never about the radio: this driver's
// STATIC Capabilities is untouched by the option, and only the set Open
// assembles carries the state. An unrecognised Profile stays on the
// untransformed fail-safe even WITH the option.
//
// CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY (matrix §2.1).
func WithConsentedUnverifiedWrites() Option {
	return func(d *ts480Driver) { d.Consented = true }
}

// withTiming overrides the transport deadlines every read this driver's
// sessions issue carries. It exists for FOCUSED TESTS against a net.Pipe,
// which answers instantly and would otherwise spend the fleet's radio-paced
// DefaultSettle on every frame of a 100-slot bank walk; production
// deliberately takes the transport's own defaults until a Kenwood radio has
// been measured. The core/driver/icr8600 shape.
func withTiming(readTimeout, settle time.Duration) Option {
	return func(d *ts480Driver) {
		d.readTimeout, d.settle = readTimeout, settle
	}
}

// New builds the TS-480 driver for profile. RealHardware — the ZERO Profile —
// selects the all-Unverified capability set while writeTrialsComplete is
// false, and ANY unrecognised Profile value deliberately selects the same
// fail-safe. Option: WithConsentedUnverifiedWrites.
//
// IT TAKES NO ROW, where core/driver/ts590.New takes a required one. That
// package serves two registry rows out of one book; this serves one. TY's four
// printed variants (480:1626-1629) are NOT registry rows — the neutral memory
// model expresses none of the difference between them (decision 4) — so there
// is nothing for a caller to choose and nothing to fail closed on.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ts480Driver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ts480Driver implements driver.Driver for the single TS-480 row.
type ts480Driver struct {
	driver.Base
	// Non-zero only in focused tests; see withTiming.
	readTimeout time.Duration
	settle      time.Duration
}

// Model implements driver.Driver.
func (d *ts480Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this row and
// profile. There is NO discovery on any Kenwood row (matrix §3.4), so a
// Session's effective set differs from this one only by the consent transform.
func (d *ts480Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No TS-480 has ever been written to over its PC-command port by
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
// specifies ONE: "Each data is constructed with 1 start bit, 8 data bits, and
// 1 stop bit (4800 bps must be configured as 2 stop bits). No parity is used."
// (480:22-23). A driver that omitted this interface would open every session
// at the wrong framing and fail like a dead port.
//
// The 4800 bps parenthesis costs nothing because 4800 is not published
// (matrix §1.11, M-E4): THIS radio's sentence is the one that makes two stop
// bits MANDATORY at that rate, which is why the rate is omitted from every row
// of the family rather than offered with framing this transport would not set.
func (d *ts480Driver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe: the prefix, this family's
// own SIX-byte answer length, and one retry — an identity read is idempotent
// and Open should survive a single swallowed reply.
//
// THE LENGTH IS core/kw's, never a literal here. kw.IDAnswerLen is six where
// core/cat's is seven, which is one of the dialect axes that make this a
// separate codec at all.
func (d *ts480Driver) idSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("ID", kw.IDAnswerLen), 1)
}

// tySpec is the transport spec for the "TY;" probe, on idSpec's terms.
//
// TY STANDS WHERE THE 590 PAIR'S FV STANDS AND REPORTS SOMETHING ELSE. That
// book's third probe frame reads a FIRMWARE VERSION; this one reads a HARDWARE
// VARIANT (480:1621-1634), because THIS RADIO HAS NO CAT-READABLE FIRMWARE
// VERSION AT ALL — no revision number, no part code and no firmware statement
// anywhere in the book, which is erratum E15 and is why kw.Layout.BuildFVRead
// refuses a Book480 layout outright. TY's own heading, "Sets or reads the
// microprocessor fimware type" (480:1621), is E10 and not a counter-example:
// the Set chart is empty, so TY is read-only despite the heading.
func (d *ts480Driver) tySpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("TY", kw.TYAnswerLen), 1)
}

// readSpec builds a Kenwood read spec around the ANSWER MATCHER its caller
// supplies, applying this driver's test-only timing overrides in ONE place.
//
// THE MATCHER IS A PARAMETER RATHER THAN A PREFIX AND A LENGTH, because this
// family has two matching rules and not one. Every fixed singleton — ID and
// TY here — is correlated by kw.PrefixLenMatcher on a prefix and an exact
// width, and so is the EX read, whose caller carries the full three-digit
// address in the prefix (settings.go, and kw.PrefixLenMatcher's own
// full-address obligation). MR is neither: every memory answer is fifty bytes
// and starts "MR", and the channel number sits at P2/P3, so no prefix
// separates one channel's answer from another's and read.go's mrSpec passes
// kw.Layout.MRAnswerMatcher instead.
func (d *ts480Driver) readSpec(match func(frame []byte) bool, retries int) transport.CommandSpec {
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
// *kw.RejectionError, which names the two indistinguishable causes this book
// prints (480:130-135) and cites the transient-suppression sentence
// (480:136-138), and silence as *kw.TimeoutError, which says in as many words
// that it is not an inference of absence.
//
// THE BOOK MATTERS AND IS NOT DECORATION. Erratum E13: the two books give
// DIFFERENT causes for the same "O;" token — "A receive buffer overrun error
// occurred" on the 590 pair against "Receive data was sent but processing was
// not completed." here (480:143-144) — which is why a Book is a semantic in
// this codec and why these errors are constructed from the LAYOUT's book
// rather than from a package-level default.
//
// IT IS ONE FUNCTION BECAUSE THE RULE IS ONE RULE, and the probe's ID; and
// TY; are frames like any other: a reader who met the typed error on an MR and
// the transport's bare error on an ID would reasonably conclude the two wire
// events mean something different there.
//
// Anything else is returned unchanged, as is either wire event on a layout
// naming no book — unreachable, since a configured layout always names one.
func wireFailure(layout kw.Layout, command string, err error) error {
	switch {
	case errors.Is(err, transport.ErrRejected):
		if rej, nerr := kw.NewRejectionError(layout.Book(), command); nerr == nil {
			return rej
		}
	case errors.Is(err, transport.ErrTimeout):
		if to, nerr := kw.NewTimeoutError(layout.Book(), command); nerr == nil {
			return to
		}
	}
	return err
}

// Open implements driver.Driver: it builds a transport.Engine over port bound
// to this row's codec layout, establishes the session (Init: AI0; and
// drain-to-quiet), then runs the matrix §3.5 identity probe — "ID;", and on
// this row's own identity "TY;".
//
// THREE FRAMES GO OUT AND THE PROBE IS TWO OF THEM (plan P9). AI0; is
// transport.Engine.Init's own frame, byte-identical to the Yaesu family's in a
// different package, and the matrix files it under §3.6 as session init rather
// than as part of the probe. A WRONG RADIO therefore receives exactly two
// frames — the preamble and "ID;" — and nothing more.
//
// THE DRAIN IS SIZED FOR THIS RADIO (P20). The 480's old AI pushes an IF frame
// every 1.5 seconds while it is on and the IF parameters are changing
// (480:194-195); core/kw's DrainPolicy.Cap is a named decision sized to
// survive that reading taken UNCONDITIONALLY, because that is the conservative
// one.
//
// NO DISCOVERY FRAME OF ANY KIND IS EVER BUILT (decision 5, matrix §3.4), on
// two independent grounds: the book says the NAK is unreliable (480:136-138),
// so a probe's silence carries no information at all; and the slot space is
// fully printed (480:955), so a probe would ask a question the book answers.
// The bank is static.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close releases
// it on success, and Open itself closes it before returning an error.
func (d *ts480Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	l := layout()
	framing, err := kw.NewFramingFor(l)
	if err != nil {
		// NEVER kw.NewFraming(book): that is the envelope-only framing,
		// whose gate admits any well-formed Kenwood frame including the
		// 42-byte erase shape. NewFramingFor puts this row's eight
		// grammars in front of the envelope, so the outbound gate a
		// session enforces is the one belonging to the radio it is for.
		// internal/guards' TestKenwoodDriversUseNewFramingFor holds this
		// down mechanically for every non-test file under core/driver.
		_ = port.Close()
		return nil, fmt.Errorf("ts480: Open: framing: %w", err)
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		// NewEngineWith has not taken the port on this path, so closing it
		// here is Open's own ownership obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ts480: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, l, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in exactly one
// place.
func (d *ts480Driver) open(ctx context.Context, eng *transport.Engine, l kw.Layout, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts480: Open: %w", err)
	}

	got, err := d.probeID(ctx, eng, l)
	if err != nil {
		return nil, err
	}
	if got != catID {
		// WantModel is always populated — this driver knows its own row's
		// name; GotModel only where a document prints one (see
		// siblingModelName). Nothing more is sent and the port closes.
		return nil, &driver.WrongRadioError{
			Want:      catID,
			Got:       got,
			WantModel: modelName,
			GotModel:  siblingModelName(got),
		}
	}
	id.CATID = got

	ty, err := d.probeTY(ctx, eng, l)
	if err != nil {
		return nil, err
	}

	return &Session{
		eng:         eng,
		layout:      l,
		id:          id,
		caps:        d.SessionCaps(d.Capabilities()),
		newReadSpec: d.readSpec,
		ty:          ty,
	}, nil
}

// probeID sends "ID;" and returns the three printed digits it answered.
func (d *ts480Driver) probeID(ctx context.Context, eng *transport.Engine, l kw.Layout) (string, error) {
	cmd, err := l.BuildIDRead()
	if err != nil {
		return "", fmt.Errorf("ts480: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return "", fmt.Errorf("ts480: Open: ID probe: %w", wireFailure(l, "ID", err))
	}
	got, err := l.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts480: Open: ID probe: %w", err)
	}
	return got, nil
}

// probeTY sends "TY;" and returns the decoded answer — P1's two reserved bytes
// verbatim and P2's variant digit.
//
// AN UNEXPECTED P2 REFUSES THE SESSION, AND THAT IS THE OPPOSITE OF THE 590
// PAIR'S FV BRANCH — deliberately, and the asymmetry is the design's own rule
// stated once (spec §Error handling): REFUSE WHERE THE DOCUMENT IS COMPLETE
// AND WE ARE OUTSIDE IT; DEGRADE WHERE THE DOCUMENT IS THIN AND WE MAY HAVE
// GUESSED ITS GRAMMAR WRONG.
//
//   - TY's P2 is a PRINTED FOUR-VALUE LEGEND — "0: TS-480HX (200 W)",
//     "1: TS-480SAT (100 W + AT)", "2: Japanese 50 W type", "3: Japanese 20 W
//     type" (480:1626-1629). A fifth value means an UNREAD VARIANT whose
//     capability table this programme would be inventing (decision 4), so the
//     session is refused rather than opened on a guess. kw.ParseTYAnswer is
//     where that refusal lives; this function only propagates it.
//   - The 590 pair's FV grammar is A13, ASSUMED from a single worked example,
//     and an unparseable answer there yields a read-only fallback rather than a
//     session refusal — because refusing would make a legitimate TS-590
//     completely unreadable on the strength of an assumption.
//
// P1 IS CARRIED AND NOT INTERPRETED. The chart prints it "Reserved"
// (480:1623) and says nothing else about it anywhere, so kw.TYAnswer reports
// the two bytes opaquely — they may carry 0x7F-0xFF, which no other field in
// this codec admits — and this driver reports them onward through
// Session.Variant without claiming anything about them. A CALLER THAT RENDERS
// Reserved FOR A HUMAN MUST %q-QUOTE IT (decision 4, kw.TYAnswer's own doc
// comment); nothing in this package renders it, and the refusals that mention
// P1 at all are core/kw's, which do.
func (d *ts480Driver) probeTY(ctx context.Context, eng *transport.Engine, l kw.Layout) (kw.TYAnswer, error) {
	cmd, err := l.BuildTYRead()
	if err != nil {
		return kw.TYAnswer{}, fmt.Errorf("ts480: Open: TY probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.tySpec())
	if err != nil {
		return kw.TYAnswer{}, fmt.Errorf("ts480: Open: TY probe: %w", wireFailure(l, "TY", err))
	}
	answer, err := l.ParseTYAnswer(frame)
	if err != nil {
		return kw.TYAnswer{}, fmt.Errorf("ts480: Open: TY probe: %w", err)
	}
	return answer, nil
}

// Session is one open, identity-probed TS-480 connection. Safe for concurrent
// use.
//
// IT CARRIES AN OPERATION MUTEX (plan P13/P14, matrix M-E2). transport.Engine
// serialises each individual EXCHANGE, not a whole driver operation, and this
// driver's operations are not all single exchanges: Open's probe is three
// frames. opMu guards ONE DRIVER OPERATION — the probe's frames, one
// ReadChannel, one WriteChannel, one ReadSetting — so a concurrent caller
// cannot land in the middle of one.
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to core/clone, as
// the driver seam assigns it (P14), and holding a driver lock across it would
// serialise two operations the seam deliberately keeps separate.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// layout is the radio this session is for. It is copied from the driver
	// that Opened it — which is also where the engine's outbound gate came
	// from — so a session can never encode with one row's layout while its
	// transport gates with another's.
	layout kw.Layout
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
	// newReadSpec builds this session's read specs, carrying the driver's
	// test-only timing overrides. A func rather than the two durations so
	// there is one construction site for a CommandSpec in this package.
	// NAMED FOR THE PACKAGE IT MUST NOT BE MISTAKEN FOR: these files also
	// use spec.Bank and spec.Capabilities two lines away.
	newReadSpec func(match func(frame []byte) bool, retries int) transport.CommandSpec

	// ty is the probe's TY answer: the hardware VARIANT this radio reported
	// and P1's two opaque reserved bytes. Nothing in this driver branches on
	// it — the four printed variants are one registry row (decision 4) and
	// the neutral memory model expresses none of the difference — and it is
	// read out through Variant for the probe note.
	ty kw.TYAnswer
}

// Identity implements driver.Session: the probed CATID plus the
// caller-supplied port path and USB serial from Open.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the effective capability set as a
// deep copy per call. The copy is load-bearing for the write gate exactly as
// in the sibling drivers — a caller mutating what it was handed must never
// alter what WriteChannel enforces.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Variant returns the probe's TY answer: the hardware variant this radio
// reported and P1's two reserved bytes, both exactly as they arrived.
//
// IT EXISTS BECAUSE THE ANSWER IS A REPORTED FACT, NOT A PRIVATE ONE. Decision
// 4 makes the TS-480's four printed variants ONE registry row and puts the
// difference "in the probe note, never in the key" (matrix §1.1) — the
// TS-480HX and the TS-480SAT differ in transmit power and in whether an
// antenna tuner is fitted (480:1626-1629), which is a radio-level fact no
// memory record carries and no capability field expresses. Without an accessor
// the datum is unreachable outside this package and no note can carry it.
//
// WHAT THIS METHOD IS NOT, AND WHERE THE REST LANDS. Rendering it is the
// registration task's (T18): "rigprog probe" prints internal/radiotext's
// per-model prose, which is STATIC and so cannot carry a per-session value, and
// the fleet's per-session probe surfaces are optional interfaces a caller
// type-asserts. This is the same half core/driver/ts590.Session.FirmwareAnswer
// is, for the same reason and by the same ruling.
//
// A CALLER THAT RENDERS TYAnswer.Reserved FOR A HUMAN MUST %q-QUOTE IT.
// kw.TYAnswer's own doc comment carries the obligation and the reason: P1 is
// printed "Reserved" (480:1623) and admits bytes 0x7F-0xFF, which no other
// field in this codec does, so a bare %v or a log line would put a raw high
// byte into a GUI field. kw.TYAnswer deliberately has no String() so that the
// obligation cannot be discharged by accident.
func (s *Session) Variant() kw.TYAnswer { return s.ty }

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable. It satisfies the optional
// driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.eng.UnexpectedFrames()
	if n < 0 {
		// Unreachable (the engine only ever increments), but never let a
		// negative int64 wrap into an absurd uint64.
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close already
// guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }
