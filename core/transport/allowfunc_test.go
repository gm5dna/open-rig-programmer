// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
)

// TestNewEngine_GateAndInitComeFromOneDialect is E3's constructor proof
// (M9c-5). NewEngine takes a cat.Dialect, not a gate function, and BOTH
// the outbound write gate and the AI init frame are derived from that ONE
// value — so an Engine cannot gate for one radio while initialising for
// another. Before this change the two arrived by different routes (an
// injected AllowFunc parameter; a package-level cat.FT710 reached for
// inside Init), and nothing structural said they had to agree.
//
// One construction, both halves observed on the wire:
//
//   - Init writes exactly "AI0;" — the dialect's own BuildAISet(false).
//   - a frame that same dialect ADMITS then reaches the port too, which
//     is only true if the gate came from it (the companion test
//     TestEngineDo_RejectedFrameIsNeverWritten shows the refusing half,
//     from a dialect that does not admit the frame).
//
// The ID read is expected to TIME OUT: the stub port answers nothing, and
// this test is about which bytes left, not about any reply. What matters
// is that the failure is ErrTimeout and not ErrDisallowedCommand.
func TestNewEngine_GateAndInitComeFromOneDialect(t *testing.T) {
	port := newStubPort("") // no replies: AI0's error window and both drains just see silence
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngine(port, cat.FT710)
	if err != nil {
		t.Fatalf("NewEngine(port, cat.FT710): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := e.Init(ctx); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}
	_, err = e.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 100 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do = %v, want ErrTimeout (the stub answers nothing); an ErrDisallowedCommand here would mean the gate did not come from the dialect that built the frame", err)
	}

	port.mu.Lock()
	writes := append([][]byte(nil), port.writes...)
	port.mu.Unlock()

	if len(writes) != 2 {
		t.Fatalf("port saw %d writes (%q), want exactly 2 — the init frame and the gated read", len(writes), writes)
	}
	if got, want := string(writes[0]), string(cat.FT710.BuildAISet(false).Bytes()); got != want {
		t.Errorf("init frame = %q, want %q — Init must build from the Engine's own dialect", got, want)
	}
	if got, want := string(writes[0]), "AI0;"; got != want {
		t.Errorf("init frame = %q, want the literal %q — E3 changes no bytes", got, want)
	}
	if got, want := string(writes[1]), "ID;"; got != want {
		t.Errorf("second write = %q, want %q — a frame the same dialect admits must reach the wire", got, want)
	}
}

// TestNewEngine_UnconfiguredDialectIsRefused: NewEngine cannot RETURN an
// Engine that speaks for no radio. That is a claim about this constructor,
// not about the type — Engine is exported, so a hand-built zero value
// compiles in any package; it fails closed at Do instead, on its nil allow
// (see TestEngineDo_RefusesWithNoAllowlist, the defence-in-depth half).
//
// The refusal is ErrUnconfiguredDialect, NOT ErrNoAllowlist, and the
// distinction is load-bearing: cat.Dialect is a struct, so a zero one
// yields a perfectly non-nil AllowedCommand method value that the old
// nil-AllowFunc check could not have seen at all. Configured() is what
// catches it.
//
// The reader goroutine must not have started either — the check runs
// before it, so a refused construction leaves nothing running. The stub
// port's Read blocks until Close and hands its canned bytes to the first
// Read regardless of any write, so a leaked reader would be observable BY
// those bytes going missing.
func TestNewEngine_UnconfiguredDialectIsRefused(t *testing.T) {
	const reply = "ID0800;" // a reader that ran would consume this
	port := newStubPort(reply)
	t.Cleanup(func() { _ = port.Close() })

	var unconfigured cat.Dialect
	if unconfigured.Configured() {
		t.Fatal("sanity check failed: a zero cat.Dialect reports Configured() == true")
	}

	e, err := NewEngine(port, unconfigured)
	if err == nil {
		t.Fatal("NewEngine(port, zero dialect) returned no error — NewEngine must not RETURN an Engine bound to no radio (a hand-built one still compiles; it fails closed at Do instead)")
	}
	if !errors.Is(err, ErrUnconfiguredDialect) {
		t.Errorf("NewEngine(port, zero dialect) error = %v, want it to wrap ErrUnconfiguredDialect", err)
	}
	if errors.Is(err, ErrNoAllowlist) {
		t.Error("NewEngine(port, zero dialect) reported ErrNoAllowlist — a dialect that describes no radio is not a MISSING gate, and conflating them would send a diagnostic after the wrong bug")
	}
	if e != nil {
		t.Error("NewEngine returned a non-nil Engine alongside its error")
	}

	// Give any (wrongly) started reader goroutine a chance to run.
	time.Sleep(50 * time.Millisecond)

	// A refused construction touches the port not at all: nothing written,
	// and nothing read either — the latter being the black-box form of
	// "the reader goroutine never started".
	port.mu.Lock()
	writes := len(port.writes)
	remaining := len(port.toRead)
	port.mu.Unlock()
	if writes != 0 {
		t.Errorf("refused NewEngine wrote %d frames to the port, want 0", writes)
	}
	if remaining != len(reply) {
		t.Errorf("port had %d unread bytes, want %d — a reader goroutine was started despite the refused construction", remaining, len(reply))
	}
}

// TestEngineDo_RefusesWithNoAllowlist is the defence-in-depth half: even
// if an Engine reaches Do without a gate, nothing reaches the wire.
// Unreachable through NewEngine by construction — checked anyway, exactly
// as ErrDisallowedCommand's own doc comment argues for the layer below.
//
// The engine is built normally, from cat.FT710 (a configured dialect
// whose gate ADMITS the ID; frame sent below), and its gate is then
// cleared in-package. Both halves are deliberate: the frame must be one
// that would otherwise have been written, or an ErrNoAllowlist here would
// prove nothing; and the only way to reach the missing-gate state at all
// is this override, since NewEngine now always sets allow from a
// configured dialect.
func TestEngineDo_RefusesWithNoAllowlist(t *testing.T) {
	port := newStubPort("ID0800;")
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngine(port, cat.FT710)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	e.allow = nil // in-package: simulate the unreachable state

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = e.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{ExpectPrefix: "ID", ExpectLen: 7})
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("Do with a nil allowlist returned %v, want ErrNoAllowlist", err)
	}
	if errors.Is(err, ErrDisallowedCommand) {
		t.Error("Do with a nil allowlist reported ErrDisallowedCommand — a missing gate is a composition bug, and must not be blamed on the frame")
	}

	port.mu.Lock()
	writes := port.writes
	port.mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("Do wrote %q with no allowlist in force; want nothing to reach the wire", writes)
	}
}

// TestEngineDo_RejectedFrameIsNeverWritten pins the OTHER half of the
// gate's contract, now on the dialect seam: a frame the Engine's OWN
// dialect refuses is reported as ErrDisallowedCommand and never written.
//
// It is a genuine cross-dialect refusal rather than a hand-written
// always-false stub. The engine is bound to the FTdx10's dialect, whose
// MT frame form is the COMBINED one; the frame offered is an FT-710
// SHORT-form MT Set, which cat.FT710's own gate admits (asserted below,
// so a frame that was simply malformed could not pass this test
// vacuously). Only an Engine consulting the dialect it was constructed
// with can refuse it.
//
// The no-wire assertion is made at the PORT, not through a recording
// gate: the gate is no longer a value a test can pass in, so "the refusal
// happened" is proven by ErrDisallowedCommand and "nothing reached the
// radio" by the port having seen zero writes. That is the property that
// mattered — the recorder was only ever the instrument.
func TestEngineDo_RejectedFrameIsNeverWritten(t *testing.T) {
	port := newStubPort("")
	t.Cleanup(func() { _ = port.Close() })

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd, err := cat.FT710.BuildMTSet(slot, true, "CALLING")
	if err != nil {
		t.Fatalf("BuildMTSet: %v", err)
	}
	if !cat.FT710.AllowedCommand(cmd.Bytes()) {
		t.Fatalf("sanity check failed: cat.FT710 refuses its own builder's frame %q — the refusal below would prove nothing about the dialect seam", cmd.Bytes())
	}

	e, err := NewEngine(port, ftdx10.Dialect())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = e.Do(ctx, cmd, CommandSpec{})
	if !errors.Is(err, ErrDisallowedCommand) {
		t.Fatalf("Do = %v, want errors.Is match against ErrDisallowedCommand — the FTdx10's dialect must refuse an FT-710 short-form MT Set", err)
	}

	port.mu.Lock()
	writes := port.writes
	port.mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("Do wrote %q despite the Engine's own dialect refusing it", writes)
	}
}
