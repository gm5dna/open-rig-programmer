// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Profile selects which capability description a driver value publishes.
// An alias, never a fresh named type — see driver.Base's own doc comment
// for why, and internal/guards.TestSimulatedProfileTokensConfinement for
// why this package keeps its own Simulated selector.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// modelParams is everything that differs between the FT-2000 and the
// FT-2000D: a display name and a CAT dialect. Matrix §4 is what makes a
// two-field struct sufficient — the two rows are byte-identical on every
// position but CATID (§1.6), so the sole model-conditional value besides
// the name lives inside the dialect rather than beside it. Mirrors
// core/driver/ftdx101's modelParams shape exactly (the closer template:
// also a Yaesu two-row package built over one CAT ID probe).
type modelParams struct {
	name    string
	dialect cat.Dialect
}

var modelFT2000 = modelParams{name: "FT-2000", dialect: dialectFT2000}
var modelFT2000D = modelParams{name: "FT-2000D", dialect: dialectFT2000D}

// siblingNames maps every CAT ID this package registers to its display
// name, so a wrong-radio refusal can name both models involved (spec A5).
var siblingNames = map[string]string{
	modelFT2000.dialect.CATID():  modelFT2000.name,
	modelFT2000D.dialect.CATID(): modelFT2000D.name,
}

// yaesuParams is the shared-body configuration (core/driver/internal/
// yaesu) both rows use: no 5xx/EMG inventory to discover (Probe stays
// yaesu.NoProbe, the zero value — matrix §2.4, this radio's manual prints
// neither bank at all), so Open's handshake never sends a discovery sweep.
var yaesuParams = yaesu.Params{
	Name: "ft2000",
}

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ft2000Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. A nil l is ignored.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft2000Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields — see driver.Base.SessionCaps
// and spec.ConsentUnverifiedWrites for what consent means. Consent is a
// statement about a SESSION, never about the radio: this driver's static
// Capabilities is untouched by the option, and only the set Open
// assembles carries the state.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft2000Driver) { d.Consented = true }
}

// NewFT2000 builds the FT-2000 driver for profile. NewFT2000D builds the
// FT-2000D's. NO BARE New: the package drives two radios and neither is
// the other's fallback (matrix §4, brief's multi-row mechanism).
//
// RealHardware — the zero value — selects the all-Unverified capability
// set while writeTrialsComplete is false (nothing writable; see that
// constant's doc comment in caps.go): NO FT-2000 OR FT-2000D HAS EVER BEEN
// ASKED ANYTHING BY THIS PROJECT. Any unrecognised Profile value
// deliberately selects the same fail-safe.
func NewFT2000(profile Profile, opts ...Option) driver.Driver {
	return newDriver(modelFT2000, profile, opts...)
}

// NewFT2000D builds the FT-2000D driver for profile. Same reasoning as
// NewFT2000 in every respect.
func NewFT2000D(profile Profile, opts ...Option) driver.Driver {
	return newDriver(modelFT2000D, profile, opts...)
}

func newDriver(m modelParams, profile Profile, opts ...Option) driver.Driver {
	d := &ft2000Driver{model: m, Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft2000Driver implements driver.Driver for both the FT-2000 and the
// FT-2000D: one type serves both rows, which differ only in model (a
// name and a CAT ID) — see modelParams.
type ft2000Driver struct {
	model modelParams
	driver.Base
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ft2000Driver) Model() string { return d.model.name }

// Capabilities implements driver.Driver: the static baseline for this
// driver's model and profile.
func (d *ft2000Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated(d.model)
	case RealHardware:
		return capabilitiesUnverified(d.model)
	default:
		// Any unrecognised Profile value fails safe: nothing writable.
		return capabilitiesUnverified(d.model)
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (AI0 init + drain-to-quiet), probes ID, and
// verifies this really is this driver's own model — a typed
// *driver.WrongRadioError otherwise. It runs no 5xx/EMG discovery sweep:
// this radio's manual declares neither bank (yaesuParams.Probe is
// yaesu.NoProbe, the zero value), so Session.Capabilities is exactly the
// static baseline, with no discovered banks appended.
//
// Open takes ownership of port on BOTH outcomes.
func (d *ft2000Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	eng, err := yaesu.NewEngine(port, d.model.dialect, d.transportLogger, &yaesuParams)
	if err != nil {
		return nil, err
	}
	sess, err := d.open(ctx, eng, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

func (d *ft2000Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	m := d.model
	got, err := yaesu.Handshake(ctx, eng, m.dialect, &yaesuParams, yaesu.Want{
		CATID: m.dialect.CATID(), Model: m.name,
		Sibling: func(catID string) string { return siblingNames[catID] },
	})
	if err != nil {
		return nil, err
	}
	id.CATID = got

	// yaesuParams.Probe is yaesu.NoProbe (the zero value), so this always
	// returns nothing — see yaesu.DiscoverInventory's own doc comment.
	// Called anyway, rather than skipped, so a Params that ever gained a
	// probe kind would not silently keep behaving as though it had none.
	if _, _, err := yaesu.DiscoverInventory(ctx, eng, m.dialect, &yaesuParams); err != nil {
		return nil, fmt.Errorf("ft2000: Open: %w", err)
	}

	return &Session{
		eng:     eng,
		dialect: m.dialect,
		id:      id,
		caps:    d.SessionCaps(d.Capabilities()),
	}, nil
}

// Session is an FT-2000/FT-2000D driver.Session: one open,
// identity-verified connection, for whichever of the two models Opened
// it. Safe for concurrent use — transport.Engine serialises every
// individual exchange, and everything else here is immutable after Open.
//
// ONE OPERATION MUTEX PER SESSION, taken by ReadChannel and WriteChannel
// and never re-entered — this radio's own operations happen to be ONE
// wire exchange each (a bare MR, a bare MW: there is no MT to sequence
// beside either), so the lock costs nothing here, but the rule is the
// fleet's one rule rather than a per-radio exception.
type Session struct {
	eng     *transport.Engine
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open

	opMu sync.Mutex
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a deep copy per call — see
// spec.Capabilities.Clone for why the copy is load-bearing.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters —
// the driver-layer surface for the engine's own accessors, which are
// otherwise unreachable. Satisfies the optional driver.DiagnosticsReporter
// capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

// ErrAnswerMismatch is the sentinel a caller compares against (via
// errors.Is) to ask "did a radio answer about the wrong channel?". The
// error actually returned is an *AnswerMismatchError naming both
// addresses.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports that an MR answer's decoded slot was not
// the one asked for.
type AnswerMismatchError = driver.AnswerMismatchError[string]

// KindMismatchError reports that an MR answer's P7 kind byte was not one
// of this radio's accepted read-side values (read.go's acceptedKinds).
type KindMismatchError struct {
	// Slot is the canonical wire-form slot that was read.
	Slot string
	// Got is the P7 kind byte the answer carried.
	Got byte
	// Want lists every kind byte this radio's read side accepts.
	Want []byte
}

// Error implements the error interface.
func (e *KindMismatchError) Error() string {
	want := make([]string, len(e.Want))
	for i, k := range e.Want {
		want[i] = fmt.Sprintf("%q", rune(k))
	}
	return fmt.Sprintf("ft2000: MR answer for slot %q carries kind %q, want one of {%s}", e.Slot, rune(e.Got), strings.Join(want, ","))
}
