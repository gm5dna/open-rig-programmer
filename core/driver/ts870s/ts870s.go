// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts870s is the driver half of the TS-870S registration: the
// static Capabilities baseline (caps.go), a live driver.Driver (this
// file), and the read/write choreography (read.go, write.go).
//
// # Why a live session is now possible
//
// core/kw's Book870S used to have no transcribed "E;"/"O;" stream-error
// citation (core/kw/errors.go's newStreamError panicked on it), and
// kw.NewFramingFor only ever accepted a kw.Layout, not the kw.Layout870
// this row's codec is built on — so Open here used to always refuse (see
// git history: the Lift K follow-up, commit e7515d0, closed both gaps).
// Both citations now exist and kw.NewFramingFor870(layout) is the live
// framing constructor this file uses, gated by Layout870's own
// AllowedCommand — which admits exactly the four grammars this driver
// needs: "AI0;" (transport.Engine.Init's own preamble), "ID;", an MR
// read, and an MW set (record870.go).
//
// # What this driver deliberately does not build yet
//
// FieldTxFrequency (the P1=1 TX-frequency half of a channel) is neither
// read nor written by this session. Reading is not the obstacle — P1=1 is
// a documented read on ordinary channels — but WRITING it is: this
// document does not establish what a genuine P1=1 MW's own P5-P8 fields
// (mode, lockout, tone) must carry, and inventing an answer would be
// exactly the fabrication this project's provenance rule forbids. ts590's
// own TxFreqHz is Unavailable for the analogous reason (its own A9
// register entry) — this is the same shape, not a shortcut invented for
// this row. WriteChannel refuses a Known TxFreqHz explicitly (write.go);
// ReadChannel never populates it (read.go). Capabilities keeps
// FieldTxFrequency graded rw (caps.go) because that describes the
// PROTOCOL, not this driver's present completeness — the ts480 A22
// precedent for keeping a capability grade and a driver-level refusal
// separate.
package ts870s

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts870s "github.com/gm5dna/open-rig-programmer/core/kw/ts870s"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Compile-time assertions that this package satisfies every seam
// internal/wiring will type-assert for on the day this row registers —
// the ts480/ts590 shape.
var (
	_ driver.Driver              = (*ts870sDriver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

// catID is this row's own printed CAT ID (matrix §1.2).
const catID = "015"

// modelName is this row's registry key (matrix §1.1).
const modelName = "TS-870S"

// siblingModelName names the model a Kenwood ID token belongs to, for a
// wrong-radio refusal — the ts480/ts590 shape. TS-870S has no registered
// sibling on this document (matrix "ONE ROW, ONE COLUMN"), so every arm
// but its own is a NEIGHBOURING Kenwood row this milestone happens to
// know the ID of, recorded so a wrong-radio message can name it rather
// than showing a bare ID; anything else returns "".
func siblingModelName(id string) string {
	switch id {
	case catID:
		return modelName
	case "020":
		return "TS-480"
	case "021":
		return "TS-590S"
	case "022":
		return "TS-990S"
	case "023":
		return "TS-590SG"
	case "024":
		return "TS-890S"
	default:
		return ""
	}
}

// New returns the TS-870S driver built with profile.
//
// NO MODEL ENUM AND NO New<Variant>: this is a single-row package (matrix
// "ONE ROW, ONE COLUMN"), so which radio a driver is for is fixed by the
// package rather than by a value a caller could get wrong — the ic7200
// shape.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ts870sDriver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes.
type Option func(*ts870sDriver)

// WithConsentedUnverifiedWrites records that the user has consented to
// writing this radio's Unverified fields — the ts480/ts590 shape. See
// package doc comment: FieldTxFrequency is refused regardless of consent,
// since this driver never builds that frame at all.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ts870sDriver) { d.Consented = true }
}

type ts870sDriver struct {
	driver.Base
}

// Model implements driver.Driver.
func (d *ts870sDriver) Model() string { return modelName }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile, before any radio has been probed.
func (d *ts870sDriver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	default:
		// writeTrialsComplete is false, so RealHardware and any
		// unrecognised profile both get the all-Unverified fail-safe.
		return CapabilitiesUnverified()
	}
}

// StopBits implements driver.SerialFramingReporter — a SESSION
// PRECONDITION, not a nicety (the ts480/ts590 reason): both Kenwood books
// this codec speaks specify one stop bit, and this row's own document is
// no exception (matrix §1.11: "1 start bit, 8 data bits, and 1 stop
// bit").
func (d *ts870sDriver) StopBits() int { return 1 }

// idSpec is the transport spec for the "ID;" probe: the prefix, this
// family's own SIX-byte answer length, and one retry — an identity read
// is idempotent and Open should survive a single swallowed reply.
func (d *ts870sDriver) idSpec() transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      kw.PrefixLenMatcher("ID", kw.IDAnswerLen),
		RetryReads: 1,
	}
}

// wireFailure types the two wire events a Kenwood exchange can meet — the
// ts480/ts590 shape, restated because Layout870 is not a kw.Layout and
// kw.NewRejectionError/kw.NewTimeoutError take only a Book, which
// Layout870.Book() supplies.
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

// Open implements driver.Driver: it builds a transport.Engine over port
// bound to kw.NewFramingFor870(ts870s.Layout), establishes the session
// (Init: AI0; and drain-to-quiet — transport.Engine's own frame, common
// to the whole Kenwood family), then runs the ID probe.
//
// ONLY ONE PROBE FRAME, UNLIKE ts480/ts590's SECOND (FV/TY): this
// document has neither command (no divergence table, no second identity
// read anywhere in this package). A wrong radio therefore receives
// exactly two frames — the preamble and "ID;" — and nothing more.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
func (d *ts870sDriver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	l := kwts870s.Layout
	framing, err := kw.NewFramingFor870(l)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ts870s: Open: framing: %w", err)
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ts870s: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, l, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

func (d *ts870sDriver) open(ctx context.Context, eng *transport.Engine, l kw.Layout870, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ts870s: Open: %w", err)
	}

	cmd, err := l.BuildIDRead()
	if err != nil {
		return nil, fmt.Errorf("ts870s: Open: ID probe: %w", err)
	}
	frame, err := eng.Do(ctx, cmd, d.idSpec())
	if err != nil {
		return nil, fmt.Errorf("ts870s: Open: ID probe: %w", wireFailure(l.Book(), "ID", err))
	}
	got, err := l.ParseIDAnswer(frame)
	if err != nil {
		return nil, fmt.Errorf("ts870s: Open: ID probe: %w", err)
	}
	if got != catID {
		return nil, &driver.WrongRadioError{
			Want:      catID,
			Got:       got,
			WantModel: modelName,
			GotModel:  siblingModelName(got),
		}
	}
	id.CATID = got

	return &Session{
		eng:    eng,
		layout: l,
		id:     id,
		caps:   d.SessionCaps(d.Capabilities()),
	}, nil
}

// Session is one open, ID-probed TS-870S connection. Safe for concurrent
// use.
//
// IT CARRIES AN OPERATION MUTEX (the ts480/ts590 reason): transport.Engine
// serialises each individual EXCHANGE, not a whole driver operation, and
// opMu guards ONE DRIVER OPERATION (one ReadChannel, one WriteChannel) so
// a concurrent caller cannot land in the middle of one.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc
	// comment.
	opMu   sync.Mutex
	layout kw.Layout870
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a deep copy per call, so a
// caller mutating what it was handed can never alter what WriteChannel
// enforces.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Diagnostics reports this session's transport-level health counters —
// the driver-layer surface for the engine's own accessors. Satisfies the
// optional driver.DiagnosticsReporter capability.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }
