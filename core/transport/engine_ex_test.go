// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// This file pins the EX (MENU) full-address correlation convention
// documented on CommandSpec.ExpectPrefix: for a command family whose
// answer frames share a short prefix ("EX") across many addresses,
// ExpectPrefix must carry the FULL address ("EX"+addr.Wire()), never the
// bare command name. Every test below either proves that convention holds
// against Engine's existing, unmodified matching logic (no engine change
// was needed for this task — see CommandSpec.matches, engine.go), or —
// the negative-space test — proves WHY the convention exists at all: a
// bare "EX" ExpectPrefix genuinely correlates a wrong-address answer as
// if it were the read's own.

// exReadSpec builds the CommandSpec for reading addr, per the convention
// pinned on CommandSpec.ExpectPrefix's doc comment (task-30's brief):
// ExpectPrefix carries the FULL six-digit address; ExpectLen is
// deliberately left 0, so length-pinning cannot reject a genuine reply
// whose width differs from the manual's Digits column. M8c showed that is
// not a hypothetical: in those two sweeps one address (01 03 21 TONE FREQ)
// answered three bytes where the manual prints two — see
// core/cat/table2-corrections.csv and ParseEXAnswer's doc comment.
// RetryReads is set because a read is idempotent. M8b's driver is expected to build its own EX CommandSpecs
// the same way.
func exReadSpec(addr cat.EXAddress) CommandSpec {
	return CommandSpec{
		ExpectPrefix: "EX" + addr.Wire(),
		RetryReads:   1,
	}
}

// mustEXAddr looks up the Table 2 member address for (p1,p2,p3), failing
// the test immediately if the triple is not a known member — every
// address used below is expected to exist; a failure here means the test
// fixture itself picked a bad triple, not a genuine finding.
func mustEXAddr(t *testing.T, p1, p2, p3 int) cat.EXAddress {
	t.Helper()
	addr, err := cat.NewEXAddress(p1, p2, p3)
	if err != nil {
		t.Fatalf("NewEXAddress(%d,%d,%d): unexpected error: %v", p1, p2, p3, err)
	}
	return addr
}

func TestEngine_EXRead_FullAddressSpec_HappyPath(t *testing.T) {
	addr := mustEXAddr(t, 1, 1, 1) // "010101", a 3-digit numeric item (RADIO SETTING / MODE SSB group)

	t.Run("stock fakeradio default", func(t *testing.T) {
		_, eng := newTestEngine(t, nil)
		ctx := testCtx(t)

		cmd, err := cat.BuildEXRead(addr)
		if err != nil {
			t.Fatalf("BuildEXRead: %v", err)
		}
		got, err := eng.Do(ctx, cmd, exReadSpec(addr))
		if err != nil {
			t.Fatalf("Do: unexpected error: %v", err)
		}

		gotAddr, gotRaw, err := cat.ParseEXAnswer(got)
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", got, err)
		}
		if gotAddr != addr {
			t.Errorf("ParseEXAnswer address = %v, want %v", gotAddr, addr)
		}
		wantRaw := fakeradio.EXRuntimeDefaults()[addr.Wire()]
		if gotRaw != wantRaw {
			t.Errorf("ParseEXAnswer raw = %q, want %q (the fake's runtime default)", gotRaw, wantRaw)
		}
	})

	t.Run("WithEXSetting override", func(t *testing.T) {
		const overridden = "007"
		_, eng := newTestEngine(t, []fakeradio.Option{
			fakeradio.WithEXSetting(addr.Wire(), overridden),
		})
		ctx := testCtx(t)

		cmd, err := cat.BuildEXRead(addr)
		if err != nil {
			t.Fatalf("BuildEXRead: %v", err)
		}
		got, err := eng.Do(ctx, cmd, exReadSpec(addr))
		if err != nil {
			t.Fatalf("Do: unexpected error: %v", err)
		}

		gotAddr, gotRaw, err := cat.ParseEXAnswer(got)
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", got, err)
		}
		if gotAddr != addr {
			t.Errorf("ParseEXAnswer address = %v, want %v", gotAddr, addr)
		}
		if gotRaw != overridden {
			t.Errorf("ParseEXAnswer raw = %q, want %q (WithEXSetting override)", gotRaw, overridden)
		}
	})
}

// TestEngine_EXRead_WrongAddressInjectedFirst_NotConsumed injects a
// well-formed EX answer for address 010102, immediately before the real
// exchange for 010101 — modelling an unsolicited push (or a stale reply
// to some other, already-abandoned read) that happens to be a genuine EX
// answer frame, just for the wrong address. The full-address
// ExpectPrefix must reject it as a match.
func TestEngine_EXRead_WrongAddressInjectedFirst_NotConsumed(t *testing.T) {
	addr := mustEXAddr(t, 1, 1, 1) // "010101"

	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultSpuriousFrame([]byte("EX010102000;"), 1)),
	})
	ctx := testCtx(t)

	cmd, err := cat.BuildEXRead(addr)
	if err != nil {
		t.Fatalf("BuildEXRead: %v", err)
	}
	got, err := eng.Do(ctx, cmd, exReadSpec(addr))
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}

	gotAddr, gotRaw, err := cat.ParseEXAnswer(got)
	if err != nil {
		t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", got, err)
	}
	if gotAddr != addr {
		t.Errorf("ParseEXAnswer address = %v, want %v (must not have been correlated to the injected wrong-address frame)", gotAddr, addr)
	}
	wantRaw := fakeradio.EXRuntimeDefaults()[addr.Wire()]
	if gotRaw != wantRaw {
		t.Errorf("ParseEXAnswer raw = %q, want %q", gotRaw, wantRaw)
	}

	if n := eng.UnexpectedFrames(); n < 1 {
		t.Errorf("UnexpectedFrames() = %d, want >= 1 (the injected wrong-address EX frame)", n)
	}
}

// TestEngine_EXRead_AIChatterStorm_StillCorrelates injects a burst of
// varied, unsolicited traffic — a push-shaped EX answer for a different
// address, an FA-style (VFO frequency) push, and a bare "AI1;" stray —
// all immediately before the real exchange, modelling a radio in AI
// (Auto Information) mode chattering while a read is outstanding.
func TestEngine_EXRead_AIChatterStorm_StillCorrelates(t *testing.T) {
	addr := mustEXAddr(t, 1, 1, 1) // "010101"

	strays := [][]byte{
		[]byte("EX0201015;"),     // unsolicited EX push, a different address
		[]byte("FA00007000000;"), // FA-style push (VFO-A frequency report)
		[]byte("AI1;"),           // stray AI-mode announcement
	}
	var opts []fakeradio.Option
	for _, s := range strays {
		opts = append(opts, fakeradio.WithFault(fakeradio.FaultSpuriousFrame(s, 1)))
	}
	_, eng := newTestEngine(t, opts)
	ctx := testCtx(t)

	cmd, err := cat.BuildEXRead(addr)
	if err != nil {
		t.Fatalf("BuildEXRead: %v", err)
	}
	got, err := eng.Do(ctx, cmd, exReadSpec(addr))
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}

	gotAddr, gotRaw, err := cat.ParseEXAnswer(got)
	if err != nil {
		t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", got, err)
	}
	if gotAddr != addr {
		t.Errorf("ParseEXAnswer address = %v, want %v", gotAddr, addr)
	}
	wantRaw := fakeradio.EXRuntimeDefaults()[addr.Wire()]
	if gotRaw != wantRaw {
		t.Errorf("ParseEXAnswer raw = %q, want %q", gotRaw, wantRaw)
	}

	if n := eng.UnexpectedFrames(); n != int64(len(strays)) {
		t.Errorf("UnexpectedFrames() = %d, want %d (every stray frame counted, safety obligation 3)", n, len(strays))
	}
}

// TestEngine_EXRead_PrefixOnlySpec_DemonstratesWrongAddressHazard is
// negative-space: it deliberately uses a BARE "EX" ExpectPrefix — the
// shape CommandSpec.ExpectPrefix's doc comment now explicitly forbids for
// EX — against the exact same wrong-address injection as
// TestEngine_EXRead_WrongAddressInjectedFirst_NotConsumed, and proves the
// engine returns the WRONG frame. This pins down WHY the full-address
// convention exists, not merely that it is followed elsewhere.
func TestEngine_EXRead_PrefixOnlySpec_DemonstratesWrongAddressHazard(t *testing.T) {
	addr := mustEXAddr(t, 1, 1, 1) // "010101" — the address actually being read
	wrongFrame := "EX010102000;"   // a DIFFERENT address's own well-formed answer

	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultSpuriousFrame([]byte(wrongFrame), 1)),
	})
	ctx := testCtx(t)

	cmd, err := cat.BuildEXRead(addr)
	if err != nil {
		t.Fatalf("BuildEXRead: %v", err)
	}

	// The hazard: ExpectPrefix is the bare command name, not the full
	// address.
	got, err := eng.Do(ctx, cmd, CommandSpec{ExpectPrefix: "EX"})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}

	if string(got) != wrongFrame {
		t.Fatalf("Do (bare \"EX\" prefix) = %q, want %q — expected the hazard to reproduce: a prefix-only spec correlates ANY EX answer, including a different address's", got, wrongFrame)
	}

	gotAddr, _, err := cat.ParseEXAnswer(got)
	if err != nil {
		t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", got, err)
	}
	if gotAddr == addr {
		t.Fatalf("ParseEXAnswer address = %v, want it to be the WRONG address (010102) — that IS the hazard a bare \"EX\" ExpectPrefix creates", gotAddr)
	}
}

// TestEngine_EXRead_DelayedPriorAnswer_NeverCrossesToNextRead mirrors
// TestEngine_SlowAnswerAfterFinalTimeout_NeverContaminatesDifferentSlotRead
// (engine_quarantine_test.go) for EX: a read's answer arrives after its
// own final timeout (retries exhausted), marking the engine suspect; the
// NEXT read, for a DIFFERENT address, must never be contaminated by that
// stale answer once it finally lands.
func TestEngine_EXRead_DelayedPriorAnswer_NeverCrossesToNextRead(t *testing.T) {
	timeout := 80 * time.Millisecond
	delay := 120 * time.Millisecond // > timeout, well inside the suspect-drain's 2*QuietPeriod budget:
	// stray lands at ~120ms, quiet-drain completes at ~320ms (120+QuietPeriod),
	// budget expires at ~480ms (80+2*QuietPeriod) — ~160ms of margin, comfortably
	// more than the ~70ms a 120ms-timeout/250ms-delay pairing left under load.
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDelayedReply(1, delay)),
	})
	ctx := testCtx(t)

	addrA := mustEXAddr(t, 1, 1, 1) // "010101", 3-digit numeric
	cmdA, err := cat.BuildEXRead(addrA)
	if err != nil {
		t.Fatalf("BuildEXRead(A): %v", err)
	}
	specA := exReadSpec(addrA)
	specA.Timeout = timeout
	specA.RetryReads = 0

	_, err = eng.Do(ctx, cmdA, specA)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do (addr A, delayed reply) = %v, want errors.Is match against ErrTimeout", err)
	}

	addrB := mustEXAddr(t, 1, 1, 4) // "010104", 4-digit numeric — different address AND width from A
	cmdB, err := cat.BuildEXRead(addrB)
	if err != nil {
		t.Fatalf("BuildEXRead(B): %v", err)
	}
	specB := exReadSpec(addrB)
	specB.Timeout = time.Second

	start := time.Now()
	got, err := eng.Do(ctx, cmdB, specB)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do (addr B, after suspect drain): unexpected error: %v", err)
	}
	if elapsed < QuietPeriod {
		t.Errorf("Do (addr B) returned after %v, want >= QuietPeriod (%v) — the entry suspect drain must genuinely have run", elapsed, QuietPeriod)
	}

	wantA := "EX010101" + fakeradio.EXRuntimeDefaults()[addrA.Wire()] + ";"
	wantB := "EX010104" + fakeradio.EXRuntimeDefaults()[addrB.Wire()] + ";"
	if string(got) == wantA {
		t.Fatalf("Do (addr B) returned addr A's STALE answer (%q) — quarantine failed, exactly the worst-case scenario this test guards against", got)
	}
	if string(got) != wantB {
		t.Errorf("Do (addr B) = %q, want %q", got, wantB)
	}

	if n := eng.UnexpectedFrames(); n < 1 {
		t.Errorf("UnexpectedFrames() = %d, want >= 1 (the purged stale addr-A answer)", n)
	}
}
