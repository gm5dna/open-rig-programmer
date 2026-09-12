// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

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

// Compile-time assertions that this package satisfies every seam
// internal/wiring will type-assert for. Here rather than in a test because
// a lost interface still COMPILES.
var (
	_ driver.Driver                = (*ts2000Driver)(nil)
	_ driver.SerialFramingReporter = (*ts2000Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// Option configures the driver New<Variant> builds — and, through it, every
// Session its Open call establishes.
type Option func(*ts2000Driver)

// WithConsentedUnverifiedWrites records the user's consent to writing this
// row's Unverified fields, on core/driver/ts480's terms exactly. ON THIS
// PACKAGE IT REACHES NOTHING EITHER: write.go refuses every channel write
// before any frame is built (three raw bytes with no honest value), so a
// consented session passes the capability gate and meets that refusal one
// rung lower — which is the point, not a waste: without the option there
// would be no way to reach the refusal on a RealHardware session at all.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ts2000Driver) { d.Consented = true }
}

// WithSimulatedProfile selects the in-process fake capability arm — the
// ic7851 shape (New7851/New7850 take no profile parameter; RealHardware,
// the zero Profile, is the default a caller gets by supplying no option at
// all, which is the safe direction driver.Profile's own doc comment
// states). Intended only for quarantined fake/e2e tests.
func WithSimulatedProfile() Option {
	return func(d *ts2000Driver) { d.Profile = Simulated }
}

// withTiming overrides the transport deadlines every read this driver's
// sessions issue carries — for focused tests against a net.Pipe (the
// core/driver/ts480 shape).
func withTiming(readTimeout, settle time.Duration) Option {
	return func(d *ts2000Driver) { d.readTimeout, d.settle = readTimeout, settle }
}

// newDriver is the one constructor body every New<Variant> calls, differing
// only in which modelParams it closes over — the ic7851 shape (New7851/
// New7850 over one unexported struct), not ts590's bare New(row, profile):
// this brief's "no bare New" for a multi-row Kenwood package.
func newDriver(p modelParams, opts ...Option) driver.Driver {
	d := &ts2000Driver{p: p, Base: driver.Base{Profile: RealHardware}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// NewTS2000 builds the TS-2000 row: CATID "019" (MANUAL-EVIDENCED,
// ts2000:10429).
func NewTS2000(opts ...Option) driver.Driver { return newDriver(paramsTS2000, opts...) }

// NewTS2000X builds the TS-2000X row: CATID "019" (ASSUMED — see
// modelParams' own doc comment).
func NewTS2000X(opts ...Option) driver.Driver { return newDriver(paramsTS2000X, opts...) }

// NewTSB2000 builds the TS-B2000 row: CATID "019" (ASSUMED).
func NewTSB2000(opts ...Option) driver.Driver { return newDriver(paramsTSB2000, opts...) }

// ts2000Driver implements driver.Driver for one of the three rows.
type ts2000Driver struct {
	driver.Base
	p modelParams
	// Non-zero only in focused tests; see withTiming.
	readTimeout time.Duration
	settle      time.Duration
}

// Model implements driver.Driver.
func (d *ts2000Driver) Model() string { return d.p.name }

// Capabilities implements driver.Driver: the static baseline for this row
// and profile. There is no discovery on any Kenwood row.
func (d *ts2000Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated(d.p)
	case RealHardware:
		// No TS-2000/2000X/B2000 has ever been asked anything by this
		// project: writeTrialsComplete is false, so there is no
		// hardware-verified profile for this arm to select.
		return CapabilitiesUnverified(d.p)
	default:
		// Any unrecognised Profile fails safe through its own explicit
		// arm, the ts480/ic7851 shape.
		return CapabilitiesUnverified(d.p)
	}
}

// StopBits implements driver.SerialFramingReporter: 1, on the family's
// standing reading of "1 start bit, 8 data bits, and 1 stop bit" — this
// document's own hardware section (ts2000:9505-9507) states the same
// framing note the TS-480's does, "4800 bps must be configured as 2 stop
// bits" as a caveat rather than a refusal (matrix §4, Bauds).
func (d *ts2000Driver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe.
func (d *ts2000Driver) idSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("ID", kw.IDAnswerLen), 1)
}

// tySpec is the transport spec for the "TY;" probe — TY, not FV: see
// core/kw/ts2000's own doc comment on layout.go for why this row's Layout
// names Book480.
func (d *ts2000Driver) tySpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("TY", kw.TYAnswerLen), 1)
}

// readSpec builds a Kenwood read spec around the answer matcher its caller
// supplies, applying this driver's test-only timing overrides in one place.
func (d *ts2000Driver) readSpec(match func(frame []byte) bool, retries int) transport.CommandSpec {
	return transport.CommandSpec{
		Class: transport.ClassRead, Match: match, RetryReads: retries,
		Timeout: d.readTimeout, Settle: d.settle,
	}
}

// wireFailure types the two wire events a Kenwood exchange can meet — the
// core/driver/ts480 shape, restated because this package's layout carries
// its OWN book (Book480, chosen on this row's own evidence — see
// core/kw/ts2000/layout.go) and a driver may not reach across a package
// boundary for the function that consults it.
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

// Open implements driver.Driver: builds a transport.Engine over port bound
// to this row's codec layout, establishes the session (Init: AI0; and
// drain-to-quiet), then probes "ID;" and "TY;" — the core/driver/ts480
// choreography exactly, TY rather than FV for the reason layout.go states.
//
// Open takes ownership of port on both outcomes.
func (d *ts2000Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	l := d.p.layout
	framing, err := kw.NewFramingFor(l)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ts2000: Open: framing: %w", err)
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ts2000: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, l, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path closes eng in exactly one
// place.
func (d *ts2000Driver) open(ctx context.Context, eng *transport.Engine, l kw.Layout, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts2000: Open: %w", err)
	}

	got, err := d.probeID(ctx, eng, l)
	if err != nil {
		return nil, err
	}
	if got != d.p.catID {
		return nil, &driver.WrongRadioError{
			Want: d.p.catID, Got: got,
			WantModel: d.p.name, GotModel: siblingModelName(got),
		}
	}
	id.CATID = got

	ty, err := d.probeTY(ctx, eng, l)
	if err != nil {
		return nil, err
	}

	return &Session{
		eng: eng, layout: l, id: id,
		caps: d.SessionCaps(d.Capabilities()),
		p:    d.p, newReadSpec: d.readSpec, ty: ty,
	}, nil
}

// probeID sends "ID;" and returns the three printed digits it answered.
func (d *ts2000Driver) probeID(ctx context.Context, eng *transport.Engine, l kw.Layout) (string, error) {
	cmd, err := l.BuildIDRead()
	if err != nil {
		return "", fmt.Errorf("ts2000: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return "", fmt.Errorf("ts2000: Open: ID probe: %w", wireFailure(l, "ID", err))
	}
	got, err := l.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts2000: Open: ID probe: %w", err)
	}
	return got, nil
}

// probeTY sends "TY;" and returns the decoded answer.
//
// AN UNEXPECTED P2 REFUSES THE SESSION (core/kw's own ParseTYAnswer): the
// grammar it enforces is the TS-480's, '0'..'3', a wider domain than this
// document's own three-value legend ("0: Overseas type / 1: Japanese 100 W
// type / 2: Japanese 20 W type", ts2000:11683-11685) — core/kw cannot be
// narrowed per row without editing its own file, which this package may
// not do, so a hypothetical P2='3' would be accepted opaquely rather than
// refused. No radio has ever answered this probe, so the gap is recorded
// here rather than exercised.
func (d *ts2000Driver) probeTY(ctx context.Context, eng *transport.Engine, l kw.Layout) (kw.TYAnswer, error) {
	cmd, err := l.BuildTYRead()
	if err != nil {
		return kw.TYAnswer{}, fmt.Errorf("ts2000: Open: TY probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.tySpec())
	if err != nil {
		return kw.TYAnswer{}, fmt.Errorf("ts2000: Open: TY probe: %w", wireFailure(l, "TY", err))
	}
	answer, err := l.ParseTYAnswer(frame)
	if err != nil {
		return kw.TYAnswer{}, fmt.Errorf("ts2000: Open: TY probe: %w", err)
	}
	return answer, nil
}

// Session is one open, identity-probed connection to one of the three rows.
// Safe for concurrent use.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — core/driver/ts480's own
	// reasoning: transport.Engine serialises each individual exchange, not
	// a whole driver operation, and Open's probe is already two frames.
	opMu   sync.Mutex
	layout kw.Layout
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
	p      modelParams
	// newReadSpec builds this session's read specs, carrying the driver's
	// test-only timing overrides.
	newReadSpec func(match func(frame []byte) bool, retries int) transport.CommandSpec
	// ty is the probe's TY answer: the hardware variant this radio
	// reported and P1's two opaque reserved bytes.
	ty kw.TYAnswer
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the effective set, a deep copy
// per call — load-bearing for the write gate, exactly as the sibling
// drivers.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Variant returns the probe's TY answer: P1's two opaque reserved bytes and
// P2's hardware-type digit ('0' Overseas, '1' Japanese 100 W, '2' Japanese
// 20 W — ts2000:11683-11685). Nothing in this driver branches on it.
//
// A CALLER THAT RENDERS TYAnswer.Reserved FOR A HUMAN MUST %q-QUOTE IT —
// kw.TYAnswer's own doc comment carries the obligation.
func (s *Session) Variant() kw.TYAnswer { return s.ty }

// Diagnostics reports this session's transport-level health counters.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }
