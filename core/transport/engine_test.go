// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// This file's tests exercise Engine against fakeradio — a real,
// independently-implemented CAT peer at the other end of an in-memory
// connection — rather than mocking the Port. See fakeradio's doc.go for why
// that independence matters. Every expected wire value below is a literal
// straight from core/cat's own golden vectors / fakeradio's factory image,
// not computed by calling the code under test.

const testCtxTimeout = 5 * time.Second

// newTestEngine constructs a fakeradio.Radio and an Engine over its Port,
// registering both for cleanup, and returns the Radio (for SlotState/fault
// setup) alongside the Engine.
func newTestEngine(t *testing.T, radioOpts []fakeradio.Option, engineOpts ...Option) (*fakeradio.Radio, *Engine) {
	t.Helper()
	r := fakeradio.New(radioOpts...)
	t.Cleanup(func() { _ = r.Close() })
	e := NewEngine(r.Port(), engineOpts...)
	t.Cleanup(func() { _ = e.Close() })
	return r, e
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// --- CommandSpec validation (pure, no I/O) ---

func TestCommandSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    CommandSpec
		wantErr bool
	}{
		{"read with retries: ok", CommandSpec{ExpectPrefix: "MR", RetryReads: 2}, false},
		{"read with zero retries: ok", CommandSpec{ExpectPrefix: "MR"}, false},
		{"fire-and-forget, zero retries: ok", CommandSpec{}, false},
		{"fire-and-forget WITH retries: invalid", CommandSpec{RetryReads: 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidSpec) {
				t.Errorf("validate() = %v, want errors.Is match against ErrInvalidSpec", err)
			}
		})
	}
}

func TestCommandSpec_WithDefaults(t *testing.T) {
	got := CommandSpec{}.withDefaults()
	if got.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", got.Timeout, DefaultTimeout)
	}
	if got.ErrorWindow != DefaultErrorWindow {
		t.Errorf("ErrorWindow = %v, want %v", got.ErrorWindow, DefaultErrorWindow)
	}
	if got.Settle != DefaultSettle {
		t.Errorf("Settle = %v, want %v", got.Settle, DefaultSettle)
	}

	explicit := CommandSpec{Timeout: time.Second, ErrorWindow: time.Millisecond, Settle: time.Millisecond}.withDefaults()
	if explicit.Timeout != time.Second || explicit.ErrorWindow != time.Millisecond || explicit.Settle != time.Millisecond {
		t.Errorf("withDefaults overrode explicit values: %+v", explicit)
	}
}

func TestCommandSpec_Matches(t *testing.T) {
	tests := []struct {
		name  string
		spec  CommandSpec
		frame string
		want  bool
	}{
		{"prefix match, variable len", CommandSpec{ExpectPrefix: "MT"}, "MT0011CALLING FREQ;", true},
		{"prefix mismatch", CommandSpec{ExpectPrefix: "MR"}, "MT0011CALLING FREQ;", false},
		{"prefix match, exact len ok", CommandSpec{ExpectPrefix: "ID", ExpectLen: 7}, "ID0800;", true},
		{"prefix match, exact len mismatch", CommandSpec{ExpectPrefix: "ID", ExpectLen: 8}, "ID0800;", false},
		{"frame shorter than prefix", CommandSpec{ExpectPrefix: "MR001"}, "MR;", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.matches([]byte(tt.frame)); got != tt.want {
				t.Errorf("matches(%q) = %v, want %v", tt.frame, got, tt.want)
			}
		})
	}
}

// --- Golden round trips against fakeradio ---

func TestEngine_MR_GoldenRoundTrip(t *testing.T) {
	_, eng := newTestEngine(t, nil)
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd, err := cat.FT710.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	got, err := eng.Do(ctx, cmd, CommandSpec{ExpectPrefix: "MR", ExpectLen: 28})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	// G4 (factory image, M-01): MR001007000000+000000110000;
	want := "MR001007000000+000000110000;"
	if string(got) != want {
		t.Errorf("Do returned %q, want %q", got, want)
	}
}

func TestEngine_MW_FireAndForget_Success(t *testing.T) {
	_, eng := newTestEngine(t, nil)
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(5)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	mode, err := cat.FT710.ParseMode('2') // USB
	if err != nil {
		t.Fatalf("ParseMode: %v", err)
	}
	ctcss, err := cat.ParseCTCSSState('0')
	if err != nil {
		t.Fatalf("ParseCTCSSState: %v", err)
	}
	shift, err := cat.ParseShift('0')
	if err != nil {
		t.Fatalf("ParseShift: %v", err)
	}
	cmd, err := cat.FT710.BuildMWSet(cat.MemoryData{
		Slot:   slot,
		FreqHz: 14250000,
		Mode:   mode,
		Kind:   cat.KindMemory,
		CTCSS:  ctcss,
		Shift:  shift,
	})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	got, err := eng.Do(ctx, cmd, CommandSpec{ErrorWindow: 60 * time.Millisecond})
	if err != nil {
		t.Fatalf("Do (fire-and-forget MW): unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("Do (fire-and-forget) returned %q, want nil", got)
	}

	// Prove it actually landed: MR005; must now echo it back.
	readCmd, err := cat.FT710.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	answer, err := eng.Do(ctx, readCmd, CommandSpec{ExpectPrefix: "MR", ExpectLen: 28})
	if err != nil {
		t.Fatalf("Do (MR verify): unexpected error: %v", err)
	}
	want := "MR005014250000+000000210000;"
	if string(answer) != want {
		t.Errorf("MR005; after MW = %q, want %q", answer, want)
	}
}

func TestEngine_MT_VariableLengthAnswer(t *testing.T) {
	_, eng := newTestEngine(t, nil)
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	setCmd, err := cat.FT710.BuildMTSet(slot, true, "CALLING FREQ")
	if err != nil {
		t.Fatalf("BuildMTSet: %v", err)
	}
	if _, err := eng.Do(ctx, setCmd, CommandSpec{ErrorWindow: 60 * time.Millisecond}); err != nil {
		t.Fatalf("Do (MT set): unexpected error: %v", err)
	}

	readCmd, err := cat.FT710.BuildMTRead(slot)
	if err != nil {
		t.Fatalf("BuildMTRead: %v", err)
	}
	got, err := eng.Do(ctx, readCmd, CommandSpec{ExpectPrefix: "MT"}) // ExpectLen 0: variable
	if err != nil {
		t.Fatalf("Do (MT read): unexpected error: %v", err)
	}
	want := "MT0011CALLING FREQ;"
	if string(got) != want {
		t.Errorf("MT read = %q, want %q", got, want)
	}
}

func TestEngine_MR_EmptySlot_Rejected(t *testing.T) {
	_, eng := newTestEngine(t, nil)
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(7) // not in the factory image
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd, err := cat.FT710.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	_, err = eng.Do(ctx, cmd, CommandSpec{ExpectPrefix: "MR", ExpectLen: 28})
	if !errors.Is(err, cat.ErrRejected) {
		t.Errorf("Do(empty slot) = %v, want errors.Is match against cat.ErrRejected", err)
	}
}

// --- A pre-cancelled ctx must never reach the wire (Do entry) ---

// TestEngine_Do_PreCancelledCtx_NeverTransmits pins down that Do checks
// ctx.Err() BEFORE writing anything — not merely that it eventually returns
// ctx's error after a doomed write. FaultGarbleReply(1) pins down which
// exchange the FIRST real command actually becomes: if Do genuinely writes
// NOTHING for a pre-cancelled ctx, the SUBSEQUENT, legitimate ID; read is
// still exchange 1 (hit by the fault, so it times out); if Do had
// (incorrectly) transmitted anyway despite the dead ctx, that write would
// itself consume exchange 1, and this ID; would land as exchange 2 —
// untouched by the fault — and succeed normally instead.
func TestEngine_Do_PreCancelledCtx_NeverTransmits(t *testing.T) {
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultGarbleReply(1)),
	})

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // already dead before Do is even called

	idCmd := cat.FT710.BuildIDRead()
	_, err := eng.Do(cancelledCtx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: time.Second})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do (pre-cancelled ctx) = %v, want errors.Is match against context.Canceled", err)
	}

	ctx := testCtx(t)
	_, err = eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 300 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do(ID;) after a pre-cancelled Do = %v, want errors.Is match against ErrTimeout (proving the pre-cancelled Do transmitted NOTHING — this ID; is still exchange 1, hit by FaultGarbleReply(1))", err)
	}
}

// --- Disallowed command never reaches the wire (safety obligation 1) ---

func TestEngine_Do_DisallowedCommandNeverWritten(t *testing.T) {
	radio, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultGarbleReply(1)), // see below: pins down which exchange number "ID;" ends up as
	})
	ctx := testCtx(t)

	// The zero Command: IsZero() == true, Bytes() returns an empty slice,
	// which cat.AllowedCommand rejects (len < 3). Do must refuse it
	// BEFORE writing anything — proven by checking that a SUBSEQUENT
	// real command is still exchange 1 (garbled), not exchange 2 (which
	// it would be if the disallowed command had actually reached the
	// fake radio as an exchange).
	var zero cat.Command
	_, err := eng.Do(ctx, zero, CommandSpec{ExpectPrefix: "X"})
	if !errors.Is(err, ErrDisallowedCommand) {
		t.Fatalf("Do(zero Command) = %v, want errors.Is match against ErrDisallowedCommand", err)
	}

	idCmd := cat.FT710.BuildIDRead()
	_, err = eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 300 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do(ID;) after a refused disallowed command = %v, want ErrTimeout (proving ID; was exchange 1, hit by FaultGarbleReply(1) — the disallowed command must not have consumed exchange 1)", err)
	}
	_ = radio
}
