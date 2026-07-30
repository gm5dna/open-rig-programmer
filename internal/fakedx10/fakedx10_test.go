// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// EVERY EXPECTED REPLY IN THIS PACKAGE'S TESTS IS RECOMPUTED INDEPENDENTLY —
// as a literal string, or by a test-local assembler that places bytes at the
// positions the manual's charts number (parser_test.go's combinedFrame) —
// never by calling this package's own builders (buildMTAnswer, buildMRAnswer,
// buildMCAnswer, ...). That independence is the whole point of a golden test:
// it must be possible for a builder to have a bug and still be CAUGHT, which
// cannot happen if the expectation comes from the same buggy function.

const testTimeout = 2 * time.Second

// newTestRadio constructs a *Radio for a test, registers its Close() as
// cleanup, and returns both the Radio (for SlotState/CurrentChannel
// assertions) and its Port().
func newTestRadio(t *testing.T, opts ...Option) (*Radio, io.ReadWriteCloser) {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	return r, r.Port()
}

func writeFrame(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatalf("Write(%q): unexpected error: %v", s, err)
	}
}

// deadliner is satisfied by net.Pipe's Conn: a real, cancelling read deadline
// rather than a goroutine-based "abandon and hope" timeout. The distinction
// matters — fakeradio's own helper records it at length: net.Pipe's Write is a
// rendezvous with whichever Read is CURRENTLY blocked, so an abandoned
// goroutine's Read could swallow a later call's reply. SetReadDeadline
// cancels the same Read call and leaves no goroutine behind.
type deadliner interface {
	SetReadDeadline(time.Time) error
}

// readOneFrame reads until it has accumulated one complete ';'-terminated
// frame, or until timeout elapses with nothing arriving (timedOut true), or
// the connection reports a non-timeout error (e.g. io.EOF after a close).
func readOneFrame(t *testing.T, r io.Reader, timeout time.Duration) (frame []byte, err error, timedOut bool) {
	t.Helper()
	d, ok := r.(deadliner)
	if !ok {
		t.Fatalf("readOneFrame: %T does not implement deadliner (SetReadDeadline)", r)
	}
	if err := d.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("SetReadDeadline: unexpected error: %v", err)
	}
	defer func() { _ = d.SetReadDeadline(time.Time{}) }()

	buf := make([]byte, 256)
	var acc []byte
	for {
		n, rerr := r.Read(buf)
		acc = append(acc, buf[:n]...)
		if len(acc) > 0 && acc[len(acc)-1] == ';' {
			return acc, nil, false
		}
		if rerr != nil {
			var ne net.Error
			if errors.As(rerr, &ne) && ne.Timeout() {
				return acc, nil, true
			}
			return acc, rerr, false
		}
	}
}

func mustReadFrame(t *testing.T, r io.Reader) string {
	t.Helper()
	frame, err, timedOut := readOneFrame(t, r, testTimeout)
	if timedOut {
		t.Fatalf("readOneFrame: timed out after %v waiting for a reply", testTimeout)
	}
	if err != nil && len(frame) == 0 {
		t.Fatalf("readOneFrame: unexpected error: %v", err)
	}
	return string(frame)
}

// assertNoReply confirms nothing arrives within a short window — how
// fire-and-forget success is verified (doc.go register entry 11: an accepted
// Set produces no acknowledgement).
func assertNoReply(t *testing.T, r io.Reader) {
	t.Helper()
	frame, _, timedOut := readOneFrame(t, r, 150*time.Millisecond)
	if !timedOut {
		t.Fatalf("expected no reply, got %q", frame)
	}
}

// exchange writes one frame and returns the reply. For frames that are
// answered; use assertNoReply for the fire-and-forget ones.
func exchange(t *testing.T, conn io.ReadWriteCloser, send string) string {
	t.Helper()
	writeFrame(t, conn, send)
	return mustReadFrame(t, conn)
}

// assertRejected writes send and requires the single unattributed NAK.
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
}

// --- ID: the CAT identity, and the one byte-level difference a probe uses ---

func TestID_AnswersThisRadiosCATID(t *testing.T) {
	_, conn := newTestRadio(t)

	// Hand-written from the manual's ID chart (lines 976-984): "ID" + the
	// four-digit CAT ID + ';'. 0761 is the FTdx10's, and it is deliberately
	// NOT the FT-710's 0800 — core/driver/ftdx10's Open turns any other value
	// into a *driver.WrongRadioError, so this literal is what makes a probe
	// against this fake succeed and a probe against fakeradio fail.
	if got, want := exchange(t, conn, "ID;"), "ID0761;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
}

func TestID_MalformedBodyRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	// ID has no Set direction at all on this radio (availability X O O X), so
	// a body of any kind is not a command.
	assertRejected(t, conn, "ID0761;")
	assertRejected(t, conn, "ID0;")
}

// --- AI: accepted, readable, and silent on Set ---

func TestAI_SetIsSilentAndReadReportsIt(t *testing.T) {
	_, conn := newTestRadio(t)

	// ASSUMED OFF at construction — doc.go register entry 14.
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; on a fresh radio -> %q, want %q", got, want)
	}

	// The Set core/transport.Engine.Init opens every session with. Silence is
	// success (register entry 11).
	writeFrame(t, conn, "AI0;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "AI1;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "AI;"), "AI1;"; got != want {
		t.Errorf("AI; after AI1; -> %q, want %q", got, want)
	}

	writeFrame(t, conn, "AI0;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; after AI0; -> %q, want %q", got, want)
	}
}

func TestAI_MalformedRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "AI2;")  // outside the '0'/'1' flag vocabulary
	assertRejected(t, conn, "AI00;") // two flag bytes
}

// --- MC: the current-channel answer, and recall ---

func TestMC_ReadAnswersTheNoneFormUntilARecall(t *testing.T) {
	_, conn := newTestRadio(t)

	// Hand-written from the MC chart (manual lines 1130-1140): "MC" + the
	// 3-byte current channel + ';'. "000" is the DIALECT's ASSUMED none form
	// (its register entry 3), which this fake reports until an MC-set.
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; on a fresh radio -> %q, want %q", got, want)
	}
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("the selection reads back as %q, want %q", got, want)
	}
}

// selectionOverTheWire reads the selection back over the WIRE and returns the slot
// the answer names, so a test asserting on the selection asserts on what a
// host would see rather than on package state. (SlotState-style inspection has
// its own uses; this is not one of them.)
func selectionOverTheWire(t *testing.T, conn io.ReadWriteCloser) string {
	t.Helper()
	got := exchange(t, conn, "MC;")
	if len(got) != 6 || !strings.HasPrefix(got, "MC") || got[5] != ';' {
		t.Fatalf("MC; -> %q, want a 6-byte \"MC\" answer", got)
	}
	return got[2:5]
}

func TestMC_RecallOfAPopulatedSlotIsSilentAndMovesTheSelection(t *testing.T) {
	r, conn := newTestRadio(t)

	writeFrame(t, conn, "MC001;") // M-01 is populated in the default image
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "MC;"), "MC001;"; got != want {
		t.Errorf("MC; after MC001; -> %q, want %q", got, want)
	}
	if got, want := r.CurrentChannel(), "001"; got != want {
		t.Errorf("CurrentChannel() = %q, want %q", got, want)
	}
}

func TestMC_RecallOfAnEmptySlotIsRejected(t *testing.T) {
	_, conn := newTestRadio(t)

	// ASSUMED — doc.go register entry 1: a channel with no stored data cannot
	// be recalled, paired with the read rule.
	assertRejected(t, conn, "MC050;")
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("a rejected recall moved the selection to %q, want it left at %q", got, want)
	}
}

func TestMC_MalformedRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MC000;")  // the answer-only none form is never a request
	assertRejected(t, conn, "MC100;")  // grammatical, out of inventory
	assertRejected(t, conn, "MC01;")   // two-byte slot
	assertRejected(t, conn, "MC0011;") // four-byte slot
}

// --- Dispatch: unknown commands, case, framing ---

func TestUnknownCommandRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ZZ;")
	assertRejected(t, conn, "FA014250000;") // a real FT-710/FTdx10 VFO command this fake does not model
	assertRejected(t, conn, "M;")           // one byte of a command name
	assertRejected(t, conn, ";")            // an empty frame
}

func TestCommandNamesAreUpperCaseOnly(t *testing.T) {
	_, conn := newTestRadio(t)

	// doc.go register entry 12: fakeradio accepts either case on the FT-710
	// manual's explicit statement; no such statement about the FTdx10 is cited
	// in this repository, so lower case is refused here rather than accepted
	// on an invented leniency.
	assertRejected(t, conn, "id;")
	assertRejected(t, conn, "mt001;")
	assertRejected(t, conn, "Mt001;")
	// The upper-case form of the same read answers, so the rejection above is
	// about the case and not about the slot.
	if got := exchange(t, conn, "MT001;"); got == "?;" {
		t.Errorf("MT001; -> %q, want an answer — the lower-case rejections above prove nothing if the upper-case form is refused too", got)
	}
}

func TestEXNotModelledYet(t *testing.T) {
	_, conn := newTestRadio(t)
	// Task 5 replaces handleEX entirely (doc.go register entry 4). Until then
	// every EX read is refused, which core/driver/ftdx10's ReadSetting maps to
	// driver.SettingUnavailable with no error.
	assertRejected(t, conn, "EX010101;")
	assertRejected(t, conn, "EX999999;")
}

func TestFramingSplitsAcrossWritesAndCoalescedFrames(t *testing.T) {
	_, conn := newTestRadio(t)

	// One frame arriving in three writes.
	writeFrame(t, conn, "I")
	writeFrame(t, conn, "D")
	writeFrame(t, conn, ";")
	if got, want := mustReadFrame(t, conn), "ID0761;"; got != want {
		t.Errorf("split ID; -> %q, want %q", got, want)
	}

	// Three frames arriving in one write, answered in order.
	writeFrame(t, conn, "ID;MC;ID;")
	for i, want := range []string{"ID0761;", "MC000;", "ID0761;"} {
		if got := mustReadFrame(t, conn); got != want {
			t.Errorf("coalesced reply %d = %q, want %q", i, got, want)
		}
	}
}

func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)

	// doc.go register entry 16 (this package's own bounded-input policy, not a
	// radio claim): more than maxAccumulatorBytes without a ';' draws one
	// "?;", then bytes are discarded up to and including the next ';'.
	garbage := strings.Repeat("X", maxAccumulatorBytes+1)
	writeFrame(t, conn, garbage)
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow -> %q, want %q", got, want)
	}

	// The tail of the overflowed frame is swallowed, up to its terminator, and
	// framing then resumes: the ID; after it is answered normally, and the
	// swallowed remainder produces no second reply.
	writeFrame(t, conn, "XXXX;ID;")
	if got, want := mustReadFrame(t, conn), "ID0761;"; got != want {
		t.Errorf("after resync, ID; -> %q, want %q (a second \"?;\" would mean the discard window was wrong)", got, want)
	}
}

// --- Close semantics ---

func TestClose_HostSeesEOF(t *testing.T) {
	r := New()
	conn := r.Port()

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Idempotent.
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	buf := make([]byte, 8)
	_, err := conn.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("Read after Close = %v, want io.EOF — only the RADIO's pipe end is closed, precisely so the host sees EOF rather than io.ErrClosedPipe", err)
	}
}

func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	conn := r.Port()

	writeFrame(t, conn, "ID;")
	// Let serve() reach the latency wait. A short sleep is enough: the reply is
	// parked for 30 seconds either way, so this cannot race into a pass.
	time.Sleep(50 * time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- r.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(testTimeout):
		t.Fatalf("Close did not return within %v while a %v latency was pending — the wait is not interruptible, and internal/wiring's fake-rig teardown depends on it being so", testTimeout, 30*time.Second)
	}
}
