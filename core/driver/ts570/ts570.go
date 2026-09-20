// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts570 "github.com/gm5dna/open-rig-programmer/core/kw/ts570"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Compile-time assertions this package still satisfies every seam
// internal/wiring will type-assert for on the day these rows register.
var (
	_ driver.Driver                = (*ts570Driver)(nil)
	_ driver.SerialFramingReporter = (*ts570Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// modelParams is everything that differs between this package's three
// rows: a display name, a CAT ID and a layout. core/driver/ftdx101's
// modelParams is the shape precedent this package's own doc comment names
// (matrix §5) — a Yaesu two-row package built over one CAT ID probe, which
// is the closer template than core/driver/ts590's single row-parametrised
// New.
type modelParams struct {
	name   string
	catID  string
	layout kw.Layout
}

// modelD is the TS-570D: CATID "017" (matrix §2, Parameter Table's own
// MODEL NUMBER format, "TS-570D: 017").
var modelD = modelParams{name: "TS-570D", catID: "017", layout: kwts570.LayoutD()}

// modelS is the TS-570S: CATID "018" (matrix §2, "TS-570S: 018").
var modelS = modelParams{name: "TS-570S", catID: "018", layout: kwts570.LayoutS()}

// modelDG is the TS-570DG: CATID ASSUMED, not printed anywhere — see doc.go
// and register ts570-dg-catid-assumed. It shares modelD's own "017" rather
// than inventing a fourth value, on the same reasoning core/kw/ts570's
// LayoutDG carries every other cell of this row by: there is nothing more
// independent to assume.
var modelDG = modelParams{name: "TS-570DG", catID: modelD.catID, layout: kwts570.LayoutDG()}

// siblingModelName names the model a Kenwood ID token belongs to, for a
// wrong-radio refusal — core/driver/ts480's own siblingModelName shape.
// Only this package's own three rows are named; a foreign ID resolves to
// "" and the refusal falls back to its ID-only text, exactly as every
// other Kenwood driver's does for an ID this package cannot name.
func siblingModelName(id string) string {
	switch id {
	case modelS.catID:
		return modelS.name
	case modelD.catID:
		// "017" NAMES BOTH TS-570D AND TS-570DG (doc.go): this function
		// can say no more than the document does, and the document names
		// neither over the other for this ID. TS-570D is returned as the
		// evidenced row of the pair.
		return modelD.name
	default:
		return ""
	}
}

// Option configures the driver NewD, NewS or NewDG builds — and, through
// it, every Session its Open call establishes.
type Option func(*ts570Driver)

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose
// SESSIONS carry the consent transform (see driver.Base.SessionCaps).
func WithConsentedUnverifiedWrites() Option {
	return func(d *ts570Driver) { d.Consented = true }
}

// withTiming overrides the transport deadlines every read this driver's
// sessions issue carries — a focused-test seam, core/driver/ts480's own
// withTiming shape.
func withTiming(readTimeout, settle time.Duration) Option {
	return func(d *ts570Driver) { d.readTimeout, d.settle = readTimeout, settle }
}

// NewD builds the TS-570D driver for profile. RealHardware — the zero
// Profile — selects the all-Unverified capability set while
// writeTrialsComplete is false, and any unrecognised Profile value
// deliberately selects the same fail-safe.
func NewD(profile driver.Profile, opts ...Option) driver.Driver {
	return newDriver(modelD, profile, opts...)
}

// NewS builds the TS-570S driver, on NewD's terms in every respect.
func NewS(profile driver.Profile, opts ...Option) driver.Driver {
	return newDriver(modelS, profile, opts...)
}

// NewDG builds the TS-570DG driver, on NewD's terms in every respect. See
// modelDG and doc.go for the CATID it shares with the D and the ambiguity
// that follows from it.
func NewDG(profile driver.Profile, opts ...Option) driver.Driver {
	return newDriver(modelDG, profile, opts...)
}

// newDriver is the one implementation the three exported constructors
// wrap. There is no bare New: three rows and none is a fallback for
// another.
func newDriver(m modelParams, profile driver.Profile, opts ...Option) driver.Driver {
	d := &ts570Driver{model: m, Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ts570Driver implements driver.Driver for all three TS-570 rows. ONE type
// serves them: they differ in a name, a CAT ID and a layout value, all
// carried in model, exactly as core/driver/ftdx101's ftdx101Driver carries
// its own two rows' difference.
type ts570Driver struct {
	driver.Base
	model modelParams
	// Non-zero only in focused tests; see withTiming.
	readTimeout time.Duration
	settle      time.Duration
}

// Model implements driver.Driver.
func (d *ts570Driver) Model() string { return d.model.name }

// Capabilities implements driver.Driver: the static baseline for this row
// and profile. There is no discovery on any Kenwood row (matrix §3
// Banks — one flat MEM bank, fully printed), so a Session's effective set
// differs from this one only by the consent transform.
func (d *ts570Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated(d.model)
	case RealHardware:
		return capabilitiesUnverified(d.model)
	default:
		// Any unrecognised Profile value fails safe through its own
		// explicit arm, exactly as every other driver package's switch
		// does.
		return capabilitiesUnverified(d.model)
	}
}

// StopBits implements driver.SerialFramingReporter: this book prints ONE
// stop bit, the same framing sentence the family shares (core/driver/ts480
// cites 480:22-23; this document's own equivalent front-matter states the
// identical 1-start/8-data/1-stop shape for its own COM port).
func (d *ts570Driver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe.
func (d *ts570Driver) idSpec() transport.CommandSpec {
	return d.readSpec(kw.PrefixLenMatcher("ID", kw.IDAnswerLen), 1)
}

func (d *ts570Driver) readSpec(match func(frame []byte) bool, retries int) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      match,
		RetryReads: retries,
		Timeout:    d.readTimeout,
		Settle:     d.settle,
	}
}

// wireFailure types the two wire events a Kenwood exchange can meet — see
// core/driver/ts480's namesake, which this is byte-for-byte, parametrised
// on the layout's own book so the typed errors quote Book570's citation
// (or, until one is transcribed, at least name Book570 rather than
// defaulting to another document's sentence).
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
// bound to this row's codec layout, establishes the session (Init: AI0;
// and drain-to-quiet), then probes identity with the ONE frame this book
// prints one of: "ID;" (doc.go — neither FV nor TY exists in this
// document, and core/kw.Layout refuses both by book on a Book570 layout).
//
// Open takes ownership of port on both outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
func (d *ts570Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	l := d.model.layout
	framing, err := kw.NewFramingFor(l)
	if err != nil {
		// NEVER kw.NewFraming(book): see core/kw/framing.go's own doc
		// comment and internal/guards' TestKenwoodDriversUseNewFramingFor.
		_ = port.Close()
		return nil, fmt.Errorf("ts570: Open: framing: %w", err)
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ts570: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, l, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

func (d *ts570Driver) open(ctx context.Context, eng *transport.Engine, l kw.Layout, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts570: Open: %w", err)
	}

	got, err := d.probeID(ctx, eng, l)
	if err != nil {
		return nil, err
	}
	if got != d.model.catID {
		return nil, &driver.WrongRadioError{
			Want:      d.model.catID,
			Got:       got,
			WantModel: d.model.name,
			GotModel:  siblingModelName(got),
		}
	}
	id.CATID = got

	return &Session{
		eng:         eng,
		layout:      l,
		model:       d.model,
		id:          id,
		caps:        d.SessionCaps(d.Capabilities()),
		newReadSpec: d.readSpec,
	}, nil
}

// probeID sends "ID;" and returns the three printed digits it answered.
func (d *ts570Driver) probeID(ctx context.Context, eng *transport.Engine, l kw.Layout) (string, error) {
	cmd, err := l.BuildIDRead()
	if err != nil {
		return "", fmt.Errorf("ts570: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return "", fmt.Errorf("ts570: Open: ID probe: %w", wireFailure(l, "ID", err))
	}
	got, err := l.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ts570: Open: ID probe: %w", err)
	}
	return got, nil
}

// Session is one open, identity-probed TS-570 connection of some row. Safe
// for concurrent use.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — core/driver/ts480's own
	// reasoning: one ReadChannel, one WriteChannel, one probe.
	opMu        sync.Mutex
	layout      kw.Layout
	model       modelParams
	id          driver.Identity
	caps        spec.Capabilities // effective; never mutated after Open
	newReadSpec func(match func(frame []byte) bool, retries int) transport.CommandSpec
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a deep copy per call, load-bearing
// for the write gate exactly as every sibling driver's.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }
