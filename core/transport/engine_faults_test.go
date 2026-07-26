// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// --- Delayed rejection within the fire-and-forget error window ---

func TestEngine_MW_FireAndForget_DelayedRejection(t *testing.T) {
	// FaultDelayedRejection(1, d) makes exchange 1 (our MW write) get a
	// LATE "?;" instead of silence, after d — chosen comfortably inside
	// the ErrorWindow we configure below, so Do's own listen genuinely
	// catches it.
	delay := 40 * time.Millisecond
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(1, delay)),
	})
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	mode, _ := cat.FT710.ParseMode('2')
	ctcss, _ := cat.ParseCTCSSState('0')
	shift, _ := cat.ParseShift('0')
	cmd, err := cat.FT710.BuildMWSet(cat.MemoryData{Slot: slot, FreqHz: 7000000, Mode: mode, Kind: cat.KindMemory, CTCSS: ctcss, Shift: shift})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	start := time.Now()
	_, err = eng.Do(ctx, cmd, CommandSpec{ErrorWindow: 200 * time.Millisecond})
	elapsed := time.Since(start)

	if !errors.Is(err, cat.ErrRejected) {
		t.Fatalf("Do = %v, want errors.Is match against cat.ErrRejected", err)
	}
	if elapsed < delay {
		t.Errorf("Do returned after %v, want at least %v (the rejection must genuinely be LATE, not immediate)", elapsed, delay)
	}
}

// --- timeout -> drain -> retry succeeds for reads ---
//
// The brief suggests FaultDropReplies(1) for this scenario, but
// FaultDropReplies is documented as PERMANENT from the given exchange
// onward ("no reply from exchange N onward (inclusive)") — it cannot
// produce "exchange 1 fails, exchange 2 (the retry) succeeds" at all: the
// retry's re-transmission is itself a NEW fakeradio exchange (exchange 2),
// which FaultDropReplies(1) would ALSO silence, so RetryReads would only
// ever exhaust into ErrTimeout, never recover.
//
// FaultGarbleReply(1) is exchange-INDEXED (only exchange 1 is affected,
// exchange 2 is normal) and genuinely exercises a timeout: a garbled reply
// arrives WITHIN the Timeout window but its corrupted first byte no longer
// matches ExpectPrefix, so Do treats it as an unexpected frame (logged,
// counted, per safety obligation 3) and keeps waiting — nothing else
// arrives for that exchange, so Timeout genuinely elapses. Do then
// quarantines (DrainToQuiet) and retries: the retry is exchange 2, which
// FaultGarbleReply(1) leaves untouched, so it succeeds normally. This is
// the closest available fakeradio primitive to "first reply lost, retry
// succeeds" and is used here in place of the brief's literal suggestion.
func TestEngine_ReadTimeout_DrainThenRetry_Succeeds(t *testing.T) {
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultGarbleReply(1)),
	})
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd, err := cat.FT710.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	got, err := eng.Do(ctx, cmd, CommandSpec{
		ExpectPrefix: "MR",
		ExpectLen:    28,
		Timeout:      150 * time.Millisecond,
		RetryReads:   1,
	})
	if err != nil {
		t.Fatalf("Do: unexpected error (retry should have recovered): %v", err)
	}
	want := "MR001007000000+000000110000;"
	if string(got) != want {
		t.Errorf("Do (after retry) = %q, want %q", got, want)
	}
	if n := eng.UnexpectedFrames(); n != 1 {
		t.Errorf("UnexpectedFrames() = %d, want 1 (the garbled exchange-1 reply)", n)
	}
}

func TestEngine_ReadTimeout_ExhaustsRetries_ReturnsErrTimeout(t *testing.T) {
	// FaultGarbleReply corrupts exactly ONE exchange; with RetryReads: 1
	// (2 total attempts) and only exchange 1 garbled, attempt 2 (exchange
	// 2) should normally succeed (covered above). Here we instead prove
	// the OTHER side: zero retries permitted means the FIRST garbled
	// exchange's timeout is final.
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultGarbleReply(1)),
	})
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd, err := cat.FT710.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	_, err = eng.Do(ctx, cmd, CommandSpec{
		ExpectPrefix: "MR",
		ExpectLen:    28,
		Timeout:      150 * time.Millisecond,
		RetryReads:   0,
	})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do (no retries permitted) = %v, want errors.Is match against ErrTimeout", err)
	}
}

// --- write (fire-and-forget) timeout NEVER retries: exactly one MW hit the wire ---

func TestEngine_WriteFireAndForget_NeverRetries_ExactlyOneExchange(t *testing.T) {
	// Structural proof, via exchange-indexed fault: FaultGarbleReply(2)
	// corrupts ONLY exchange 2's reply. If the fire-and-forget MW below
	// consumes exactly ONE exchange (exchange 1, as required — a write is
	// NEVER retried, no matter what), then the immediately following ID;
	// read is exchange 2, and — because FaultGarbleReply flips the first
	// reply byte, which breaks ExpectPrefix "ID" matching — that read
	// must time out. If the engine had (incorrectly) retried/resent the
	// MW, "ID;" would instead land as exchange 3 (untouched by the fault)
	// and succeed normally. So: the ID read timing out IS the proof that
	// exactly one MW reached the fake radio.
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultGarbleReply(2)),
	})
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	mode, _ := cat.FT710.ParseMode('2')
	ctcss, _ := cat.ParseCTCSSState('0')
	shift, _ := cat.ParseShift('0')
	mwCmd, err := cat.FT710.BuildMWSet(cat.MemoryData{Slot: slot, FreqHz: 7000000, Mode: mode, Kind: cat.KindMemory, CTCSS: ctcss, Shift: shift})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	// The fire-and-forget spec forbids RetryReads structurally (tested
	// separately in TestEngine_Do_FireAndForgetWithRetryReads_Invalid);
	// here we confirm the RUNTIME behaviour with a legal spec: silence
	// within the window is success, and that is the ONLY write attempt
	// physically possible in this call.
	if _, err := eng.Do(ctx, mwCmd, CommandSpec{ErrorWindow: 60 * time.Millisecond}); err != nil {
		t.Fatalf("Do (fire-and-forget MW): unexpected error: %v", err)
	}

	idCmd := cat.FT710.BuildIDRead()
	_, err = eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 200 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do(ID;) = %v, want ErrTimeout (proving the MW write was exchange 1 only — no resend occurred)", err)
	}
}

func TestEngine_Do_FireAndForgetWithRetryReads_Invalid(t *testing.T) {
	// Static/structural enforcement: RetryReads set on a fire-and-forget
	// spec is refused BEFORE any I/O — proven by checking that a
	// subsequent, legitimate command is still exchange 1 (not preceded by
	// a phantom write from the rejected call).
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultGarbleReply(1)),
	})
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	mode, _ := cat.FT710.ParseMode('2')
	ctcss, _ := cat.ParseCTCSSState('0')
	shift, _ := cat.ParseShift('0')
	mwCmd, err := cat.FT710.BuildMWSet(cat.MemoryData{Slot: slot, FreqHz: 7000000, Mode: mode, Kind: cat.KindMemory, CTCSS: ctcss, Shift: shift})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	_, err = eng.Do(ctx, mwCmd, CommandSpec{RetryReads: 3})
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("Do(fire-and-forget, RetryReads=3) = %v, want errors.Is match against ErrInvalidSpec", err)
	}

	idCmd := cat.FT710.BuildIDRead()
	_, err = eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 200 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do(ID;) = %v, want ErrTimeout (proving the invalid MW call wrote NOTHING — ID; is still exchange 1, hit by FaultGarbleReply(1))", err)
	}
}

// --- Unexpected frame: logged and counted, not fatal ---

func TestEngine_UnexpectedFrame_LoggedAndCounted_NotFatal(t *testing.T) {
	spurious := []byte("FA00007000000;")
	var records []string
	_, eng := newTestEngine(t,
		[]fakeradio.Option{fakeradio.WithFault(fakeradio.FaultSpuriousFrame(spurious, 1))},
		WithLogger(recordingLogger{records: &records}),
	)
	ctx := testCtx(t)

	idCmd := cat.FT710.BuildIDRead()
	got, err := eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	if string(got) != "ID0800;" {
		t.Errorf("Do = %q, want %q", got, "ID0800;")
	}
	if n := eng.UnexpectedFrames(); n != 1 {
		t.Errorf("UnexpectedFrames() = %d, want 1", n)
	}
	if len(records) == 0 {
		t.Fatal("logger recorded nothing; the spurious frame must be logged")
	}
	found := false
	for _, r := range records {
		if containsAll(r, "FA00007000000") {
			found = true
		}
	}
	if !found {
		t.Errorf("logger records = %v, want one mentioning the spurious frame content", records)
	}
}

// recordingLogger implements Logger by appending every formatted message to
// *records, for tests that want to assert something was actually logged
// (safety obligation 3), not just counted.
type recordingLogger struct {
	records *[]string
}

func (l recordingLogger) Printf(format string, args ...any) {
	*l.records = append(*l.records, fmt.Sprintf(format, args...))
}

// --- Contamination -> DrainToQuiet -> recovery ---

func TestEngine_Contamination_DrainToQuiet_Recovers(t *testing.T) {
	// FaultGarbleReply cannot trigger cat.FrameAccumulator's
	// FrameTooLongError: it only flips bits within an otherwise
	// correctly-shaped, correctly-terminated, correctly-LENGTH reply, so
	// framing is untouched. Genuine contamination requires bytes that
	// never reach a ';' terminator within the accumulator's maxFrame
	// bound. FaultSpuriousFrame lets us inject exactly that: an
	// unterminated garbage burst, longer than a small WithMaxFrame we
	// configure for this test, written to the host BEFORE exchange 1's
	// own (normal) reply.
	garbage := make([]byte, 40) // no ';' anywhere in this slice
	for i := range garbage {
		garbage[i] = 'X'
	}
	_, eng := newTestEngine(t,
		[]fakeradio.Option{fakeradio.WithFault(fakeradio.FaultSpuriousFrame(garbage, 1))},
		WithMaxFrame(16),
	)
	ctx := testCtx(t)

	idCmd := cat.FT710.BuildIDRead()
	_, err := eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 300 * time.Millisecond})
	if !errors.Is(err, ErrContaminated) {
		t.Fatalf("Do = %v, want errors.Is match against ErrContaminated", err)
	}

	// A second Do call must ALSO fail fast with ErrContaminated, without
	// even attempting a write, until DrainToQuiet succeeds.
	_, err = eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 300 * time.Millisecond})
	if !errors.Is(err, ErrContaminated) {
		t.Fatalf("Do (while still contaminated) = %v, want ErrContaminated", err)
	}

	if err := eng.DrainToQuiet(ctx); err != nil {
		t.Fatalf("DrainToQuiet: unexpected error: %v", err)
	}

	// Recovered: a fresh ID; now succeeds normally.
	got, err := eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7})
	if err != nil {
		t.Fatalf("Do (after recovery): unexpected error: %v", err)
	}
	if string(got) != "ID0800;" {
		t.Errorf("Do (after recovery) = %q, want %q", got, "ID0800;")
	}
}

// --- Disconnect -> ErrPortClosed ---

func TestEngine_Disconnect_ReturnsErrPortClosed(t *testing.T) {
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDisconnect(1)),
	})
	ctx := testCtx(t)

	idCmd := cat.FT710.BuildIDRead()
	_, err := eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7})
	if err != nil {
		t.Fatalf("Do (exchange 1, before disconnect): unexpected error: %v", err)
	}

	// Give the disconnect a moment to land (the fake closes its pipe end
	// asynchronously right after handling exchange 1).
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		_, lastErr = eng.Do(ctx, idCmd, CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, Timeout: 100 * time.Millisecond})
		if errors.Is(lastErr, ErrPortClosed) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !errors.Is(lastErr, ErrPortClosed) {
		t.Fatalf("Do (after disconnect) = %v, want errors.Is match against ErrPortClosed", lastErr)
	}
}
