// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"context"
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// FakeSessionOpts holds extra fakeradio.Option values applied, on top of
// the always-empty production default, to the FT-710's fake rig on every
// OpenFakeSessionFor call in this process. It exists solely as a minimal
// test-only seam (task-12 brief §3, moved verbatim from
// cmd/rigprog/wiring.go's openFakeSessionOpts by task-15's extraction):
// an in-process inventory-mismatch case needs a caller to run against a
// NON-default factory image (e.g. fakeradio.ImageUS) through the EXACT
// SAME code path a real "--fake"/demo invocation uses, rather than
// hand-building a session that bypasses this package's constructor
// entirely.
//
// FT-710-SPECIFIC BY DESIGN, recorded (M9c-5 E5). Its type names
// internal/fakeradio, which is the FT-710's simulator; it is captured by
// the FT-710 entry's own newRadio closure below, not applied by
// OpenFakeSessionFor to whatever rig a model happens to build. A second
// model's fake rig is a different type with different options, and it
// gets its own capture in its own closure — deliberately NOT a shared,
// generic option channel, which could only be typed by emptying it of
// meaning. The seam stays as narrow as the one test need that justifies
// it.
//
// The FTdx10's registration (M9c-6) is that design's first real test, and
// it held: FTdx10FakeSessionOpts below is a SEPARATE variable of a
// different element type ([]fakedx10.Option), read in its own closure. No
// generic option plumbing was needed, and neither model's options can
// reach the other's rig.
//
// No production flag or GUI control populates this — it does not add a
// second ft710.Simulated reference to any non-test file:
// TestSimulatedProfileTokensConfinement (internal/guards) keeps passing
// unchanged. It is analogous to core/clone's Service.openJournal field,
// which a test in that package overwrites directly for the same reason.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var FakeSessionOpts []fakeradio.Option

// FTdx10FakeSessionOpts is the FTdx10's own option source: extra
// fakedx10.Option values applied, on top of the always-empty production
// default, to the FTdx10's fake rig on every OpenFakeSessionFor call in
// this process. It is FakeSessionOpts' FTdx10 counterpart and NOT a
// generalisation of it (M9c-5 E5, realised at M9c-6 task 6) — a separate
// variable, of a different element type, read at CALL time inside the
// FTdx10 entry's own newRadio closure below.
//
// Its element type is the point. internal/fakedx10 simulates the FTdx10;
// its Option is a func(*fakedx10.Radio) and cannot configure an
// *fakeradio.Radio, nor the reverse. Two typed variables therefore make a
// crossed application a COMPILE error, where one shared generically-typed
// option channel would have made it a silent no-op at runtime.
//
// Its users today are a wiring test that opens an FTdx10 fake carrying a
// populated 5 MHz bank (fakedx10.With5xx) through the very code path a
// real "--fake --model FTdx10" invocation uses, and app/uispec_test.go's
// D5c acceptance tests, which reach GetUISpec through the same path with
// a discovered bank present — the discovery-through-wiring
// property no default-image session can express, since the default FTdx10
// image deliberately has no 5xx bank at all.
//
// No production flag or GUI control populates this — it adds no second
// ftdx10.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ftdx10 row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var FTdx10FakeSessionOpts []fakedx10.Option

// fakeRadio is everything OpenFakeSessionFor needs from a model's fake
// rig: a port to hand the driver, and a way to shut the rig down
// afterwards. Interface-typed rather than *fakeradio.Radio (M9c-5 E5)
// because internal/fakeradio simulates the FT-710 specifically — a second
// model's simulator is a different type, and a concretely-typed table
// could not hold it at all. The FTdx10's *fakedx10.Radio (M9c-6) is that
// second type, and it needed no change here: it satisfies this interface
// as written, which is what the interface was extracted for.
//
// Port returns io.ReadWriteCloser, NOT transport.Port, and that is
// forced: Go matches interface methods by EXACT signature, so a
// Port() transport.Port method would be unsatisfiable by
// *fakeradio.Radio, whose Port() returns io.ReadWriteCloser. Nothing is
// lost at the call site — transport.Port IS io.ReadWriteCloser
// (core/transport/port.go), so the returned value is assignable to the
// parameter driver.Driver.Open declares.
type fakeRadio interface {
	Port() io.ReadWriteCloser
	Close() error
}

// The compile-time proofs that each registered model's simulator satisfies
// the interface its table entry is typed by — so a change to either side is
// a build failure here rather than a surprise at the entry.
var (
	_ fakeRadio = (*fakeradio.Radio)(nil)
	_ fakeRadio = (*fakedx10.Radio)(nil)
)

// fakeDriverEntry pairs one model's simulated-profile driver constructor
// with the fake-rig constructor OpenFakeSessionFor uses to build a live
// session against it — the fake-table analogue of wiring.go's
// realDrivers, but kept wholly in this file (task 39 brief) because the
// entries themselves are where ft710.Simulated and fakeradio.New are
// referenced, and both tokens must stay confined to this one file
// (TestSimulatedProfileTokensConfinement, internal/guards).
type fakeDriverEntry struct {
	// newDriver builds this model's simulated-profile driver.Driver.
	newDriver func() driver.Driver
	// newRadio builds a fresh fake rig for this model, already carrying
	// whatever options that model's simulator takes — the closure
	// captures its OWN option source (M9c-5 E5), so no option value has
	// to be routed through OpenFakeSessionFor and typed generically to
	// pass a model it does not belong to. Takes no parameters for that
	// reason.
	newRadio func() fakeRadio
}

// fakeDrivers is the model-keyed table OpenFakeSessionFor looks up:
// model name -> (simulated-profile driver constructor, fake-rig
// constructor). This is the ONLY place in this repository — non-test
// file, repo-wide — that references ft710.Simulated or ftdx10.Simulated
// (task-11 brief §3, pinned per driver by internal/guards'
// TestSimulatedProfileTokensConfinement since task-15's extraction —
// folded from the single-driver guard task-15 originally extended into
// this data-driven guard at Task 58) and the sole place a fakeradio.Radio
// or a fakedx10.Radio is constructed for a live session: the wire pattern
// proven at core/clone/helpers_test.go:194 —
// fakeradio.New() -> ft710.New(ft710.Simulated).Open(ctx, r.Port(), ...).
//
// Each newRadio is deliberately a closure CALLING the simulator's own
// constructor, not that function assigned directly — internal/guards'
// TestSimulatedProfileTokensConfinement's AST walk looks for an actual
// fakeradio.New(...) / fakedx10.New(...) CALL expression in this file, not
// merely a reference to the function value, so both calls must stay
// textually present here. Each entry's closure is also where THAT model's
// option source is read — FakeSessionOpts for the FT-710,
// FTdx10FakeSessionOpts for the FTdx10, at CALL time, never captured at
// package init, so a test that sets one before OpenFakeSessionFor still
// takes effect — and reading them HERE rather than in OpenFakeSessionFor is
// what keeps each seam its own model's (M9c-5 E5; see those variables' own
// doc comments).
//
// The FTdx10's driver half is ftdx10.Simulated, and that profile is
// write-SUPPORTED (unlike its RealHardware half, where
// writeTrialsComplete=false leaves nothing writable). That is deliberate
// and it is a claim about the FAKE, not about any radio: this pairing is
// the only place ftdx10.Simulated is legal, and internal/fakedx10 stores
// and returns what the combined MT Set carries — see
// core/driver/ftdx10/doc.go's "The Simulated profile's clarifier is
// Supported, not Inert".
var fakeDrivers = map[string]fakeDriverEntry{
	DefaultModel: {
		newDriver: func() driver.Driver { return ft710.New(ft710.Simulated) },
		newRadio:  func() fakeRadio { return fakeradio.New(FakeSessionOpts...) },
	},
	FTdx10Model: {
		newDriver: func() driver.Driver { return ftdx10.New(ftdx10.Simulated) },
		newRadio:  func() fakeRadio { return fakedx10.New(FTdx10FakeSessionOpts...) },
	},
}

// OpenFakeSessionFor opens a session against a fresh in-process fake rig
// for model, via model's own entry in fakeDrivers. This function knows
// nothing about any model's simulator options: each entry's newRadio
// closure carries its own (M9c-5 E5 — see FakeSessionOpts and
// FTdx10FakeSessionOpts, the two test-only seams of that kind, one per
// registered model).
//
// An unrecognised model fails with *UnknownModelError BEFORE any fake rig
// is constructed. The returned close function releases the session first,
// then the fake rig (both simulators' Close is prompt — see their
// interruptible scripted delays), returning the session's error if both
// fail.
//
// An FTdx10 open is SLOW by design and that is not a defect to fix here:
// core/driver/ftdx10's Open probes the whole declared 5xx range plus EMG
// (~100 exchanges) because that radio has no verified discovery
// termination rule, so each call costs seconds rather than milliseconds
// (M9c-6 plan: acknowledged and budgeted). Anybody shortening it must read
// that driver's doc.go first — the shortening IS the assumption.
func OpenFakeSessionFor(ctx context.Context, model string) (driver.Session, func() error, error) {
	entry, ok := fakeDrivers[model]
	if !ok {
		return nil, nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}

	r := entry.newRadio()

	reg, err := NewRegistry(entry.newDriver())
	if err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	drv, ok := reg.Get(model)
	if !ok {
		// Unreachable while TestDriverTableKeysMatchDriverModel holds:
		// entry.newDriver() was just registered under its own Model(),
		// which that test pins equal to this table key. Returned rather
		// than ignored so a future table whose key drifted from its
		// driver's Model() fails with this package's own typed error
		// instead of a nil-pointer panic when drv is used below — and the
		// fake rig r, already constructed above, is closed first so it is
		// never leaked.
		_ = r.Close()
		return nil, nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}

	sess, err := drv.Open(ctx, r.Port(), driver.Identity{Port: "fake", USBSerial: "SIM0001"})
	if err != nil {
		// Open owns the port (r.Port()) on both outcomes: it is already
		// closed, but the fakeradio.Radio behind it is not — close it.
		_ = r.Close()
		return nil, nil, &OpenFakeSessionError{Cause: err}
	}

	closeAll := func() error {
		sessErr := sess.Close()
		radioErr := r.Close()
		if sessErr != nil {
			return sessErr
		}
		return radioErr
	}
	return sess, closeAll, nil
}

// OpenFakeSessionError is OpenFakeSessionFor's typed failure when the
// driver's own Open fails against the in-process fakeradio port. See
// wiring.go's RegisterDriverError doc comment for why Error() carries
// this package's own generic wording, and why a caller wanting different
// wording (Fix 7, adjudicated LOW, Codex M6 #7) should use Cause via
// errors.As instead.
type OpenFakeSessionError struct{ Cause error }

func (e *OpenFakeSessionError) Error() string {
	return fmt.Sprintf("wiring: open fake session: %v", e.Cause)
}

func (e *OpenFakeSessionError) Unwrap() error { return e.Cause }
