// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// FakeSessionOpts holds extra fakeradio.Option values applied, on top of
// the always-empty production default, to every OpenFakeSessionFor call
// in this process. It exists solely as a minimal test-only seam (task-12
// brief §3, moved verbatim from cmd/rigprog/wiring.go's
// openFakeSessionOpts by task-15's extraction): an in-process
// inventory-mismatch case needs a caller to run against a NON-default
// factory image (e.g. fakeradio.ImageUS) through the EXACT SAME code
// path a real "--fake"/demo invocation uses, rather than hand-building a
// session that bypasses this package's constructor entirely.
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
	// newRadio builds a fresh fake rig session opts are applied to —
	// fakeradio.New's own signature (a closure calling fakeradio.New,
	// not the function value itself; see fakeDrivers' doc comment for
	// why).
	newRadio func(opts ...fakeradio.Option) *fakeradio.Radio
}

// fakeDrivers is the model-keyed table OpenFakeSessionFor looks up:
// model name -> (simulated-profile driver constructor, fake-rig
// constructor). This is the ONLY place in this repository — non-test
// file, repo-wide — that references ft710.Simulated (task-11 brief §3,
// pinned by internal/guards' TestSimulatedProfileTokensConfinement since
// task-15's extraction — folded from the single-driver guard task-15
// originally extended into this data-driven guard at Task 58) and the
// sole place a fakeradio.Radio is
// constructed for a live session: the wire pattern proven at
// core/clone/helpers_test.go:194 —
// fakeradio.New() -> ft710.New(ft710.Simulated).Open(ctx, r.Port(), ...).
//
// newRadio is deliberately a closure CALLING fakeradio.New, not
// fakeradio.New assigned directly — internal/guards'
// TestSimulatedProfileTokensConfinement's AST walk looks for an actual
// fakeradio.New(...) CALL expression in this file, not merely a
// reference to the function value, so the call must stay textually
// present here.
var fakeDrivers = map[string]fakeDriverEntry{
	DefaultModel: {
		newDriver: func() driver.Driver { return ft710.New(ft710.Simulated) },
		newRadio:  func(opts ...fakeradio.Option) *fakeradio.Radio { return fakeradio.New(opts...) },
	},
}

// OpenFakeSessionFor opens a session against a fresh in-process fake rig
// for model, via model's own entry in fakeDrivers. See FakeSessionOpts
// for the one test-only seam on this construction, applied identically
// regardless of model.
//
// An unrecognised model fails with *UnknownModelError BEFORE any fake rig
// is constructed. The returned close function releases the session first,
// then the fake rig (fakeradio's Close is prompt — see fakeradio's
// interruptible scripted delays), returning the session's error if both
// fail.
func OpenFakeSessionFor(ctx context.Context, model string) (driver.Session, func() error, error) {
	entry, ok := fakeDrivers[model]
	if !ok {
		return nil, nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}

	r := entry.newRadio(FakeSessionOpts...)

	reg, err := NewRegistry(entry.newDriver())
	if err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	drv, _ := reg.Get(model) // just registered under this exact key

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
