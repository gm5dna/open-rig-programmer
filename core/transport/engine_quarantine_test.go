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

// This file exercises the Task 9 quarantine fix: Engine.Do's
// inter-transaction quarantine, closing the gap where a reply belonging
// to an abandoned exchange could be consumed as the answer to whatever
// Do sends NEXT. See doc.go for the semantics pinned here.

// --- Trigger (a): late "?;" after a fire-and-forget window has already
// elapsed. Rule 1: every fire-and-forget OUTCOME (success or rejection)
// runs a best-effort post-write quarantine drain, using a FRESH bounded
// context, before releasing the mutex. ---

func TestEngine_LateRejectionAfterFireAndForgetWindow_QuarantinedByPostWriteDrain(t *testing.T) {
	// FaultDelayedRejection(1, delay) makes exchange 1 (our MW write) get
	// a LATE "?;" — after delay, which is chosen comfortably AFTER the
	// ErrorWindow Do itself listens for (so Do's own window genuinely
	// elapses first, declaring success), but comfortably inside the
	// post-write quarantine drain's budget (2*QuietPeriod from when the
	// drain starts, ~ErrorWindow+Settle) so the drain catches it before
	// giving up.
	errorWindow := 40 * time.Millisecond
	delay := 150 * time.Millisecond
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(1, delay)),
	})
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(5)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	mode, _ := cat.FT710.ParseMode('2')
	ctcss, _ := cat.ParseCTCSSState('0')
	shift, _ := cat.ParseShift('0')
	mwCmd, err := cat.FT710.BuildMWSet(cat.MemoryData{Slot: slot, FreqHz: 14250000, Mode: mode, Kind: cat.KindMemory, CTCSS: ctcss, Shift: shift})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	start := time.Now()
	_, err = eng.Do(ctx, mwCmd, CommandSpec{Class: ClassWrite, ErrorWindow: errorWindow, Settle: time.Millisecond})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do (fire-and-forget MW): unexpected error: %v (the late \"?;\" must be quarantined, not surfaced as a rejection of THIS write)", err)
	}
	if elapsed < QuietPeriod {
		t.Errorf("Do returned after %v, want >= QuietPeriod (%v) — the post-write quarantine drain must genuinely have run and waited for quiet", elapsed, QuietPeriod)
	}

	if n := eng.UnexpectedFrames(); n != 1 {
		t.Errorf("UnexpectedFrames() = %d, want 1 (the late \"?;\" consumed by the post-write quarantine drain)", n)
	}

	// The critical assertion (finding (a)): the NEXT Do must get its OWN
	// correct answer, and must NOT be spuriously rejected by the stale
	// "?;" that belonged to the abandoned MW exchange.
	idCmd := cat.FT710.BuildIDRead()
	got, err := eng.Do(ctx, idCmd, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7)})
	if err != nil {
		t.Fatalf("Do (ID; after quarantine): unexpected error: %v", err)
	}
	if string(got) != "ID0800;" {
		t.Errorf("Do (ID; after quarantine) = %q, want %q", got, "ID0800;")
	}
}

// --- Trigger (b): a slow, genuinely-matching answer arriving after a
// read's FINAL timeout (retries exhausted). Rule 2+3: the terminal
// timeout marks the engine "suspect"; the NEXT Do's entry runs a full
// drain-to-quiet BEFORE transmitting, so the stale answer cannot be
// mistaken for a DIFFERENT slot's read. This is the worst-case scenario
// from the finding. ---

func TestEngine_SlowAnswerAfterFinalTimeout_NeverContaminatesDifferentSlotRead(t *testing.T) {
	timeout := 80 * time.Millisecond
	delay := 180 * time.Millisecond // > timeout, but well inside the suspect-drain's 2*QuietPeriod budget
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDelayedReply(1, delay)),
	})
	ctx := testCtx(t)

	slot1, err := cat.FT710.MemorySlot(1) // factory image: 7.000000 MHz LSB
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd1, err := cat.FT710.BuildMRRead(slot1)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	_, err = eng.Do(ctx, cmd1, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: timeout})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do (slot 1, delayed reply): %v, want errors.Is match against ErrTimeout", err)
	}

	slotP1L, err := cat.FT710.PMSSlot(1, false) // factory image: P1L, 1.810000 MHz LSB
	if err != nil {
		t.Fatalf("PMSSlot: %v", err)
	}
	cmd2, err := cat.FT710.BuildMRRead(slotP1L)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	start := time.Now()
	got, err := eng.Do(ctx, cmd2, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: time.Second})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do (slot P1L, after suspect drain): unexpected error: %v", err)
	}
	if elapsed < QuietPeriod {
		t.Errorf("Do (slot P1L) returned after %v, want >= QuietPeriod (%v) — the entry suspect drain must genuinely have run", elapsed, QuietPeriod)
	}

	wantSlot1 := "MR001007000000+000000110000;"
	wantSlot2 := "MRP1L001810000+000000150000;"
	if string(got) == wantSlot1 {
		t.Fatalf("Do (slot P1L) returned slot 1's STALE data (%q) — quarantine failed, exactly the worst-case scenario from the finding", got)
	}
	if string(got) != wantSlot2 {
		t.Errorf("Do (slot P1L) = %q, want %q", got, wantSlot2)
	}

	if n := eng.UnexpectedFrames(); n < 1 {
		t.Errorf("UnexpectedFrames() = %d, want >= 1 (the purged stale MR001 answer)", n)
	}
}

// --- Trigger (c): ctx cancellation mid-Do. Rule 2+3 again, but the
// suspect flag is set WITHOUT attempting a drain using the caller's own
// (already-dead) ctx — the drain is deferred to the next Do's entry,
// using a fresh context. ---

func TestEngine_CtxCancelAfterWrite_NextDoRunsSuspectDrainAndSucceeds(t *testing.T) {
	ctx1Timeout := 40 * time.Millisecond
	delay := 150 * time.Millisecond // > ctx1Timeout, inside the suspect-drain's budget
	var records []string
	_, eng := newTestEngine(t,
		[]fakeradio.Option{fakeradio.WithFault(fakeradio.FaultDelayedReply(1, delay))},
		WithLogger(recordingLogger{records: &records}),
	)

	slot1, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd1, err := cat.FT710.BuildMRRead(slot1)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), ctx1Timeout)
	defer cancel1()
	_, err = eng.Do(ctx1, cmd1, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: 2 * time.Second})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Do (slot 1, ctx cancelled mid-wait) = %v, want errors.Is match against context.DeadlineExceeded", err)
	}

	slotP1L, err := cat.FT710.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("PMSSlot: %v", err)
	}
	cmd2, err := cat.FT710.BuildMRRead(slotP1L)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	ctx2 := testCtx(t)
	start := time.Now()
	got, err := eng.Do(ctx2, cmd2, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: time.Second})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do (slot P1L, after ctx-cancel suspect drain): unexpected error: %v", err)
	}

	// Clock assertion: the entry suspect drain must genuinely have run
	// (waited out a QuietPeriod), not been skipped.
	if elapsed < QuietPeriod {
		t.Errorf("Do (slot P1L) returned after %v, want >= QuietPeriod (%v)", elapsed, QuietPeriod)
	}

	want := "MRP1L001810000+000000150000;"
	if string(got) != want {
		t.Errorf("Do (slot P1L) = %q, want %q", got, want)
	}

	if n := eng.UnexpectedFrames(); n < 1 {
		t.Errorf("UnexpectedFrames() = %d, want >= 1 (the purged stale MR001 answer from the cancelled exchange)", n)
	}

	// Log assertion: the purged stale frame must actually have been
	// logged (safety obligation 3), not merely counted.
	found := false
	for _, r := range records {
		if containsAll(r, "MR001") {
			found = true
		}
	}
	if !found {
		t.Errorf("logger records = %v, want one mentioning the purged stale MR001 frame", records)
	}
}

// --- Rule 4: non-blocking purge at Do entry. A frame already sitting
// buffered when Do starts must be purged for free (no waiting) before
// transmitting. Constructed deterministically (no timing races): a
// spurious frame is scripted to arrive BEFORE exchange 1's own real
// reply (fakeradio always writes the real reply too, right after any
// spurious frames — see FaultSpuriousFrame's doc comment), and the
// spurious frame is deliberately shaped to also satisfy the read's own
// ExpectPrefix/ExpectLen, so Do1 matches on IT and returns — leaving
// exchange 1's OWN real MR001 answer sitting in the buffer, entirely
// untouched, once Do1 returns. That is exactly "a frame sits buffered
// between transactions" with no artificial delay required. ---

func TestEngine_EntryPurge_BufferedStaleFrame_NextDoStillGetsOwnAnswer(t *testing.T) {
	fakeFrame := []byte("MR001001000000+000000110000;") // 28 bytes, MR-shaped, deliberately wrong data
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultSpuriousFrame(fakeFrame, 1)),
	})
	ctx := testCtx(t)

	slot1, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd1, err := cat.FT710.BuildMRRead(slot1)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	// Settle gives the concurrently-running fakeradio+reader goroutines
	// time to finish delivering exchange 1's OWN real reply into
	// e.events before Do1 actually returns — a generous default is
	// plenty (the real work here takes microseconds).
	got1, err := eng.Do(ctx, cmd1, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Settle: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("Do (exchange 1): unexpected error: %v", err)
	}
	if string(got1) != string(fakeFrame) {
		t.Fatalf("Do (exchange 1) = %q, want %q (test setup invariant: must match the SPURIOUS frame first — see comment)", got1, fakeFrame)
	}

	before := eng.UnexpectedFrames()

	slotP1L, err := cat.FT710.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("PMSSlot: %v", err)
	}
	cmd2, err := cat.FT710.BuildMRRead(slotP1L)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	start := time.Now()
	got2, err := eng.Do(ctx, cmd2, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: time.Second})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do (exchange 2, after entry purge): unexpected error: %v", err)
	}
	// Entry purge is non-blocking ("free, no waiting") — Do2 must not
	// have paid anything like a QuietPeriod-scale cost for it.
	if elapsed >= QuietPeriod {
		t.Errorf("Do (exchange 2) took %v, want well under QuietPeriod (%v) — entry purge must be non-blocking", elapsed, QuietPeriod)
	}

	want2 := "MRP1L001810000+000000150000;"
	if string(got2) != want2 {
		t.Errorf("Do (exchange 2) = %q, want %q — must be P1L's own answer, never the stale buffered MR001 reply", got2, want2)
	}
	if n := eng.UnexpectedFrames(); n != before+1 {
		t.Errorf("UnexpectedFrames() grew by %d, want 1 (the entry-purged stale MR001 reply)", n-before)
	}
}

// --- Post-write quarantine drain itself fails on its own 2*QuietPeriod
// deadline (the stream was observed genuinely non-quiet, not just closed
// or contaminated): this must mark e.suspect, so the NEXT Do runs a full
// entry drain before trusting anything as its own answer — exactly the
// same discipline a terminal read timeout or a ctx cancellation already
// gets (see the three triggers above). Before this fix, quarantineAfterWrite
// only logged the failure and left e.suspect untouched. ---

func TestEngine_PostWriteQuarantineDrainExceedsBudget_MarksSuspectForNextDo(t *testing.T) {
	// A burst of spurious frames, spaced closer together than QuietPeriod
	// but composed (via repeated FaultSpuriousFrame registrations, all
	// fired unconditionally right when exchange 1 — our MW write — is
	// received) to keep re-arming drainToQuietLocked's internal timer for
	// LONGER than quarantineAfterWrite's own fixed 2*QuietPeriod budget.
	// WithLatency spaces the spurious writes out in real time (rawWrite
	// honours it for every write, including spurious ones — see
	// fakeradio's doc comments), giving a deterministic, non-random
	// "traffic never lets up long enough to go quiet" scenario.
	gap := 80 * time.Millisecond // « QuietPeriod: each keeps postponing "quiet"
	spurious := []byte("ID0800;")
	var opts []fakeradio.Option
	const n = 6 // 6*gap = 480ms, comfortably longer than the 2*QuietPeriod (400ms) budget
	for i := 0; i < n; i++ {
		opts = append(opts, fakeradio.WithFault(fakeradio.FaultSpuriousFrame(spurious, 1)))
	}
	opts = append(opts, fakeradio.WithLatency(gap))

	var records []string
	_, eng := newTestEngine(t, opts, WithLogger(recordingLogger{records: &records}))
	ctx := testCtx(t)

	slot, err := cat.FT710.MemorySlot(5)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	mode, _ := cat.FT710.ParseMode('2')
	ctcss, _ := cat.ParseCTCSSState('0')
	shift, _ := cat.ParseShift('0')
	mwCmd, err := cat.FT710.BuildMWSet(cat.MemoryData{Slot: slot, FreqHz: 14250000, Mode: mode, Kind: cat.KindMemory, CTCSS: ctcss, Shift: shift})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	// errorWindow chosen small so Do's OWN fire-and-forget wait elapses
	// (declaring success) BEFORE the first spurious frame even arrives
	// (gap=80ms) — the write's own outcome must not be affected by any of
	// this; only the POST-write quarantine drain is under test here.
	start := time.Now()
	_, err = eng.Do(ctx, mwCmd, CommandSpec{Class: ClassWrite, ErrorWindow: 50 * time.Millisecond, Settle: time.Millisecond})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do (fire-and-forget MW): unexpected error: %v (a failed BEST-EFFORT post-write drain must never change the write's own outcome)", err)
	}
	if elapsed < 2*QuietPeriod {
		t.Errorf("Do returned after %v, want >= 2*QuietPeriod (%v) — the post-write quarantine drain must genuinely have run its full budget and failed on it", elapsed, 2*QuietPeriod)
	}

	foundAttribution := false
	for _, r := range records {
		if containsAll(r, "post-write quarantine drain") {
			foundAttribution = true
		}
	}
	if !foundAttribution {
		t.Errorf("logger records = %v, want one attributing the failure to the post-write quarantine drain", records)
	}

	// The critical assertion: the NEXT Do, of any kind, must observe
	// e.suspect and run a full entry drain BEFORE transmitting — proven by
	// its own elapsed time, not just by it eventually succeeding.
	idCmd := cat.FT710.BuildIDRead()
	start2 := time.Now()
	got, err := eng.Do(ctx, idCmd, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7), Timeout: time.Second})
	elapsed2 := time.Since(start2)
	if err != nil {
		t.Fatalf("Do (ID; after the failed post-write drain): unexpected error: %v", err)
	}
	if elapsed2 < QuietPeriod {
		t.Errorf("Do (ID;) returned after %v, want >= QuietPeriod (%v) — e.suspect must have been set, forcing a full entry drain", elapsed2, QuietPeriod)
	}
	if string(got) != "ID0800;" {
		t.Errorf("Do (ID;) = %q, want %q", got, "ID0800;")
	}
}

// --- Suspect drain failure at entry surfaces as a typed error, and does
// not transmit. ---

func TestEngine_EntrySuspectDrainFailure_ReturnsTypedQuarantineError(t *testing.T) {
	timeout := 80 * time.Millisecond
	delay := 180 * time.Millisecond
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDelayedReply(1, delay)),
		// The pipe closes right after exchange 1's (delayed, now stale)
		// reply is sent — i.e. WHILE Do2's entry suspect drain is
		// waiting on it, not before Do2 even starts.
		fakeradio.WithFault(fakeradio.FaultDisconnect(1)),
	})
	ctx := testCtx(t)

	slot1, err := cat.FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	cmd1, err := cat.FT710.BuildMRRead(slot1)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}

	_, err = eng.Do(ctx, cmd1, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: timeout})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do (slot 1): %v, want errors.Is match against ErrTimeout", err)
	}

	idCmd := cat.FT710.BuildIDRead()
	_, err = eng.Do(ctx, idCmd, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7), Timeout: time.Second})
	if !errors.Is(err, ErrQuarantineFailed) {
		t.Fatalf("Do (entry suspect drain, port closes mid-drain) = %v, want errors.Is match against ErrQuarantineFailed", err)
	}
	if !errors.Is(err, ErrPortClosed) {
		t.Errorf("Do = %v, want errors.Is match against ErrPortClosed too (reachable via QuarantineFailedError's Unwrap chain)", err)
	}
	if n := eng.UnexpectedFrames(); n < 1 {
		t.Errorf("UnexpectedFrames() = %d, want >= 1 (the stale MR001 answer purged before the disconnect was observed)", n)
	}
}
