// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

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
// pins both texts verbatim.
func siblingModelName(catID string) string {
	switch catID {
	case "020":
		// Built by this milestone and deliberately NOT registered (P3).
		return "TS-480"
	case catIDS:
		return modelNameS
	case catIDSG:
		return modelNameSG
	case "024":
		// The tier's second pair, which this milestone does not build.
		return "TS-890S"
	default:
		return ""
	}
}

// ErrRowUnset is the sentinel a caller compares against (via errors.Is) when
// a driver built with the zero Row is asked to Open. The error actually
// returned is a *RowUnsetError.
var ErrRowUnset = errors.New("ts590: the driver was built without a row, so it names no radio")

// RowUnsetError reports that New was called with the zero Row.
//
// NO FRAME IS SENT AND THE PORT IS CLOSED. The two rows differ on byte 28's
// write policy and on their slot ceilings, so there is no frame this driver
// could honestly emit without first knowing which radio it is for.
type RowUnsetError struct{}

// Error implements the error interface.
func (e *RowUnsetError) Error() string {
	return "ts590: Open: this driver was built with the zero Row, which names neither the TS-590S nor the TS-590SG — the two rows differ in what byte 28 may carry on a write (A14) and in where their slot space stops (A12), so no frame can be built until a caller says which radio this is"
}

// Unwrap lets errors.Is(err, ErrRowUnset) match.
func (e *RowUnsetError) Unwrap() error { return ErrRowUnset }

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes.
type Option func(*ts590Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination — fall
// into the engine's own drop-everything default with no way for a caller of
// this driver to receive them. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ts590Driver) {
		if l != nil {
			d.transportOptions = append(d.transportOptions, transport.WithLogger(l))
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
	return func(d *ts590Driver) { d.consentUnverifiedWrites = true }
}

// withTiming overrides the transport deadlines every read this driver's
// sessions issue carries. It exists for FOCUSED TESTS against a net.Pipe,
// which answers instantly and would otherwise spend the fleet's radio-paced
// DefaultSettle on every frame of a 120-slot bank walk; production
// deliberately takes the transport's own defaults until a Kenwood radio has
// been measured. The core/driver/icr8600 shape.
func withTiming(readTimeout, settle time.Duration) Option {
	return func(d *ts590Driver) {
		d.readTimeout, d.settle = readTimeout, settle
	}
}

// New builds the TS-590 driver for row and profile. row is REQUIRED and has
// no usable zero (see Row); RealHardware — the ZERO Profile — selects the
// all-Unverified capability set while writeTrialsComplete is false, and ANY
// unrecognised Profile value deliberately selects the same fail-safe.
// Options: WithTransportLogger, WithConsentedUnverifiedWrites.
func New(row Row, profile Profile, opts ...Option) driver.Driver {
	d := &ts590Driver{row: row, profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ts590Driver implements driver.Driver for one of the two TS-590 rows.
type ts590Driver struct {
	row     Row
	profile Profile
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default.
	consentUnverifiedWrites bool
	transportOptions        []transport.Option
	// Non-zero only in focused tests; see withTiming.
	readTimeout time.Duration
	settle      time.Duration
}

// Model implements driver.Driver. An unset row names no radio and therefore
// matches no registry key.
func (d *ts590Driver) Model() string { return modelNameFor(d.row) }

// Capabilities implements driver.Driver: the static baseline for this
// driver's row and profile. There is NO discovery on any Kenwood row (matrix
// §3.4), so a Session's effective set differs from this one only by the
// consent transform.
func (d *ts590Driver) Capabilities() spec.Capabilities {
	if _, ok := layoutFor(d.row); !ok {
		// An unset row: the zero capability set, which spec.Validate
		// refuses and which grades nothing writable. Fail-closed rather
		// than a guess at which sibling was meant.
		return spec.Capabilities{}
	}
	switch d.profile {
	case Simulated:
		return CapabilitiesSimulated(d.row)
	case RealHardware:
		// No TS-590 has ever been written to over its PC-command port by
		// this project — none has ever been ASKED anything at all:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select.
		return CapabilitiesUnverified(d.row)
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence of the flip's state.
		return CapabilitiesUnverified(d.row)
	}
}

// StopBits implements driver.SerialFramingReporter, and it is a SESSION
// PRECONDITION rather than a nicety (matrix §3.1, plan P10):
// transport.DefaultStopBits is 2 — the Yaesu family's framing — and both
// Kenwood books specify ONE: "Start Bit 1", "Data Bit 8", "Stop Bit 1 (2 is
// available only when using 4800 bps)", "Parity Bit None" (590:54-59). A
// driver that omitted this interface would open every session at the wrong
// framing and fail like a dead port.
//
// It answers 1 for an unset row too: the framing is the FAMILY's, printed
// once for both radios, and a row-conditional answer here would invent a
// difference the book does not print. An unset row is refused at Open, which
// is where the refusal belongs.
//
// The 4800 bps parenthesis costs nothing because 4800 is not published
// (matrix §1.11, M-E4): the TS-480's own sentence makes two stop bits
// MANDATORY at that rate, so the rate is omitted from every row rather than
// offered with framing this transport would not set.
func (d *ts590Driver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe: the prefix, this family's
// own SIX-byte answer length, and one retry — an identity read is idempotent
// and Open should survive a single swallowed reply.
//
// THE LENGTH IS core/kw's, never a literal here. kw.IDAnswerLen is six where
// core/cat's is seven, which is one of the seven dialect axes that make this
// a separate codec at all.
func (d *ts590Driver) idSpec() transport.CommandSpec {
	return d.readSpec("ID", kw.IDAnswerLen, 1)
}

// fvSpec is the transport spec for the "FV;" probe, on idSpec's terms.
func (d *ts590Driver) fvSpec() transport.CommandSpec {
	return d.readSpec("FV", kw.FVAnswerLen, 1)
}

// readSpec builds a Kenwood read spec: the prefix and exact answer length
// correlate the answer (kw.PrefixLenMatcher), and this driver's test-only
// timing overrides are applied in ONE place.
func (d *ts590Driver) readSpec(prefix string, exactLen, retries int) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      kw.PrefixLenMatcher(prefix, exactLen),
		RetryReads: retries,
		Timeout:    d.readTimeout,
		Settle:     d.settle,
	}
}

// wireFailure types the TWO WIRE EVENTS a Kenwood exchange can meet, so
// every frame this driver sends reports them the same way: a "?;" as
// *kw.RejectionError, which names the two indistinguishable causes the books
// print for it and cites the transient-suppression sentence, and silence as
// *kw.TimeoutError, which says in as many words that it is not an inference
// of absence. ReadChannel's doc comment carries the substance of both.
//
// IT IS ONE FUNCTION BECAUSE THE RULE IS ONE RULE. The design states it
// unqualified for this family rather than for the read path only, and the
// probe's ID; and FV; are frames like any other: a reader who met the typed
// error on an MR and the transport's bare error on an ID would reasonably
// conclude the two wire events mean something different there.
// TestOpen_AProbeFrameThatDoesNotANSWERRefusesTheSession pins both probe
// frames and TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence and
// TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence the read.
//
// Anything else is returned unchanged, as is either wire event on a layout
// naming no book — unreachable, since a configured layout always names one,
// and an error quoting a document this session was never told it was speaking
// to is what kw's fallible constructors exist to prevent.
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

// Open implements driver.Driver: it builds a transport.Engine over port
// bound to THIS ROW'S codec layout, establishes the session (Init: AI0; and
// drain-to-quiet), then runs the matrix §3.5 identity probe — "ID;", and on
// this row's own identity "FV;".
//
// THREE FRAMES GO OUT AND THE PROBE IS TWO OF THEM (plan P9). AI0; is
// transport.Engine.Init's own frame, byte-identical to the Yaesu family's in
// a different package, and the matrix files it under §3.6 as session init
// rather than as part of the probe. A WRONG RADIO therefore receives exactly
// two frames — the preamble and "ID;" — and nothing more.
//
// NO DISCOVERY FRAME OF ANY KIND IS EVER BUILT (decision 5, matrix §3.4), on
// two independent grounds: the books say the NAK is unreliable, so a probe's
// silence carries no information at all; and the slot space is fully printed
// on both rows, so a probe would be asking a question the book answers. Every
// bank is static.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close releases
// it on success, and Open itself closes it before returning an error.
func (d *ts590Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	layout, ok := layoutFor(d.row)
	if !ok {
		_ = port.Close()
		return nil, &RowUnsetError{}
	}
	framing, err := kw.NewFramingFor(layout)
	if err != nil {
		// NEVER kw.NewFraming(book): that is the envelope-only framing,
		// whose gate admits any well-formed Kenwood frame including the
		// 42-byte erase shape. NewFramingFor puts this row's eight
		// grammars in front of the envelope, so the outbound gate a
		// session enforces is the one belonging to the radio it is for.
		_ = port.Close()
		return nil, fmt.Errorf("ts590: Open: framing: %w", err)
	}
	eng, err := transport.NewEngineWith(port, framing, d.transportOptions...)
	if err != nil {
		// NewEngineWith has not taken the port on this path, so closing it
		// here is Open's own ownership obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ts590: Open: %w", err)
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
func (d *ts590Driver) open(ctx context.Context, eng *transport.Engine, layout kw.Layout, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts590: Open: %w", err)
	}

	got, err := d.probeID(ctx, eng, layout)
	if err != nil {
		return nil, err
	}
	if got != catIDFor(d.row) {
		// WantModel is always populated — this driver knows its own row's
		// name; GotModel only where a document prints one (see
		// siblingModelName). Nothing more is sent and the port closes.
		return nil, &driver.WrongRadioError{
			Want:      catIDFor(d.row),
			Got:       got,
			WantModel: modelNameFor(d.row),
			GotModel:  siblingModelName(got),
		}
	}
	id.CATID = got

	fv, err := d.probeFV(ctx, eng, layout)
	if err != nil {
		return nil, err
	}

	s := &Session{
		eng:         eng,
		row:         d.row,
		layout:      layout,
		id:          id,
		caps:        d.sessionCapabilities(),
		newReadSpec: d.readSpec,
	}
	s.fvAnswer = fv
	s.fvMajor, s.fvMinor, s.fvGrammarOK = parseFirmwareVersion(fv)
	return s, nil
}

// probeID sends "ID;" and returns the three printed digits it answered.
func (d *ts590Driver) probeID(ctx context.Context, eng *transport.Engine, layout kw.Layout) (string, error) {
	cmd, err := layout.BuildIDRead()
	if err != nil {
		return "", fmt.Errorf("ts590: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return "", fmt.Errorf("ts590: Open: ID probe: %w", wireFailure(layout, "ID", err))
	}
	got, err := layout.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts590: Open: ID probe: %w", err)
	}
	return got, nil
}

// probeFV sends "FV;" and returns P1's four characters VERBATIM.
//
// A FRAME THAT DOES NOT ARRIVE AND A FRAME THAT DOES NOT PARSE ARE NOT THE
// SAME THING, and this is the design's own asymmetry (§"Error handling"):
//
//   - The EXISTENCE of FV is printed for both rows, with a Read chart, an
//     Answer chart and a worked example (590:1034-1037). A radio that
//     answered "ID;" with this row's identity and then rejects or ignores
//     "FV;" is outside a document that is complete here, and the standing
//     rule is to REFUSE where the document is complete and we are outside
//     it. So a transport failure here fails Open.
//   - The GRAMMAR of the answer is A13, assumed from a single worked
//     example. A four-character answer this programme cannot read as M.NN
//     does NOT fail the session: refusing there would make a legitimate
//     TS-590 completely unreadable on the strength of an assumption. The
//     raw bytes are carried verbatim into the session and the consequence
//     is confined to the write path (A13, A14; the ladder is T12's).
//
// The 7-byte structural parse in between — prefix, width, printable ASCII —
// (kw.FVAnswerLen; six is the ID answer's length, kw.IDAnswerLen)
// is the ENVELOPE's, not the grammar's, so its failure is a malformed frame
// and fails Open like any other.
func (d *ts590Driver) probeFV(ctx context.Context, eng *transport.Engine, layout kw.Layout) (string, error) {
	cmd, err := layout.BuildFVRead()
	if err != nil {
		return "", fmt.Errorf("ts590: Open: FV probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.fvSpec())
	if err != nil {
		return "", fmt.Errorf("ts590: Open: FV probe: %w", wireFailure(layout, "FV", err))
	}
	answer, err := layout.ParseFVAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts590: Open: FV probe: %w", err)
	}
	return answer, nil
}

// parseFirmwareVersion reads A13's assumed M.NN form: one digit, a dot, then
// two digits. It reports the major and minor numbers and whether the four
// characters had that shape at all.
//
// THE GRAMMAR IS THE ASSUMPTION AND THE WIDTH IS NOT (A13). The answer chart
// pins four characters (590:1037); the only format statement anywhere is
// "for firmware version 1.00, it reads FV1.00;" (590:1035). So this function
// must be able to say "I could not read this", and its caller must survive
// that answer — which is what makes A13 load-bearing on a REFUSAL path
// rather than on a success path: an unreadable version takes the
// conservative branch of the TS-590S write gate (A14), which T12's ladder
// builds on this result.
func parseFirmwareVersion(answer string) (major, minor int, ok bool) {
	if len(answer) != kw.FVChars {
		return 0, 0, false
	}
	if answer[1] != '.' {
		return 0, 0, false
	}
	for _, i := range []int{0, 2, 3} {
		if answer[i] < '0' || answer[i] > '9' {
			return 0, 0, false
		}
	}
	return int(answer[0] - '0'), int(answer[2]-'0')*10 + int(answer[3]-'0'), true
}

// sessionCapabilities is the ONE place a session's effective capability set
// is assembled: this row's profile baseline, then — only when this driver was
// built with WithConsentedUnverifiedWrites AND its profile is one of the
// declared constants — the consent transform. An unrecognised profile stays
// untransformed even with the option, so the fail-safe direction ("no value a
// caller can pass produces a writable session") survives consent.
//
// There is no discovery term: no Kenwood bank is discovered (matrix §3.4).
func (d *ts590Driver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// profileRecognised reports whether this driver's profile is one of the
// package's declared Profile constants — the same set the capability switch
// names explicitly, restated here so the consent gate cannot drift open for a
// profile the switch would fail safe on.
func (d *ts590Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// Session is one open, identity-probed TS-590 connection. Safe for concurrent
// use.
//
// IT CARRIES AN OPERATION MUTEX (plan P13/P14, matrix M-E2). transport.Engine
// serialises each individual EXCHANGE, not a whole driver operation, and this
// driver's operations are not all single exchanges: Open's probe is three
// frames, and the settings read T13 adds is another. opMu guards ONE DRIVER
// OPERATION — the probe's frames, one ReadChannel, one WriteChannel, one
// ReadSetting — so a concurrent caller cannot land in the middle of one.
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to core/clone,
// as the driver seam assigns it (P14), and holding a driver lock across it
// would serialise two operations the seam deliberately keeps separate.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// row and layout are the radio this session is for. layout is copied
	// from the driver that Opened it — which is also where the engine's
	// outbound gate came from — so a session can never encode with one
	// row's layout while its transport gates with another's.
	row    Row
	layout kw.Layout
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
	// newReadSpec builds this session's read specs, carrying the driver's
	// test-only timing overrides. A func rather than the two durations so
	// there is one construction site for a CommandSpec in this package.
	// NAMED FOR THE PACKAGE IT MUST NOT BE MISTAKEN FOR: these files also
	// use spec.Bank and spec.Capabilities two lines away.
	newReadSpec func(prefix string, exactLen, retries int) transport.CommandSpec

	// The probe's FV answer and what this programme could read of it.
	//
	// fvAnswer is P1's four characters VERBATIM and is what a probe note
	// reports, whatever the grammar did — so an owner of a 2.xx TS-590S can
	// see the version even where this programme could not parse it. It is
	// read out through FirmwareAnswer, whose doc comment carries the design
	// requirement and says where the rendering lands.
	// fvGrammarOK false means A13's assumed M.NN form did not hold, and
	// fvMajor/fvMinor are then zero and must not be read.
	//
	// NOTHING ON THE SG ROW BRANCHES ON THEM (matrix §4, divergence 2): the
	// SG's byte 28 is live by construction, so its firmware answers no
	// question this programme asks. On the S row they are what A14's write
	// gate consults, and that gate is the write path's (T12).
	fvAnswer    string
	fvMajor     int
	fvMinor     int
	fvGrammarOK bool
}

// Identity implements driver.Session: the probed CATID plus the
// caller-supplied port path and USB serial from Open.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the effective capability set as a
// deep copy per call. The copy is load-bearing for the write gate exactly as
// in the sibling drivers — a caller mutating what it was handed must never
// alter what WriteChannel enforces.
func (s *Session) Capabilities() spec.Capabilities { return cloneCapabilities(s.caps) }

// FirmwareAnswer returns the FV probe's P1 EXACTLY AS THE RADIO ANSWERED IT —
// the four characters verbatim, whether or not this programme could read them
// as A13's assumed M.NN form, and "" only on a session that never got one
// (which Open makes unreachable: an FV that does not answer refuses the
// session).
//
// IT EXISTS BECAUSE THE ANSWER IS A REPORTED FACT, NOT A PRIVATE ONE. The
// design requires the raw bytes to reach the probe note regardless of parse,
// "so an owner of a 2.xx TS-590S can see that their radio has a capability
// this row does not publish" — the stated mitigation for the published cost
// of the S row's firmware-blind filter grade (§"Firmware and the filter
// field"; §"Error handling"; §"Refusals and their shapes"; matrix §2.7,
// §3.5). Without an accessor the datum is unreachable outside this package
// and no note can carry it.
//
// WHAT THIS METHOD IS NOT, AND WHERE THE REST LANDS. Rendering it is the
// registration task's (T18): "rigprog probe" prints internal/radiotext's
// per-model prose, which is STATIC and so cannot carry a per-session value,
// and the fleet's per-session probe surfaces are optional interfaces a
// caller type-asserts (driver.RegionReporter, driver.DiagnosticsReporter).
// Those interfaces were each named on the neutral seam AFTER the method
// existed on a driver's own Session — core/driver/optional.go says so of
// both — so this is that shape's first half and deliberately no more. The
// Icom route of appending a token to Identity.CATID is NOT available here:
// internal/wiring's per-model identity check accepts a prefix match only
// where the static CATID is a bare two-character CI-V address, and this
// family's is three characters (core/driver/driver.go's Identity).
//
// TestOpen_TheFVAnswerIsReadableForTheProbeNote pins it on both rows, parsed
// and unparsed.
func (s *Session) FirmwareAnswer() string { return s.fvAnswer }

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
//
// WriteChannel — the other half of driver.Session — is write.go's: ONE
// 50-byte MW Set behind the refusal ladder of plan P7.
func (s *Session) Close() error { return s.eng.Close() }
