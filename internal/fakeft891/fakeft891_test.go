// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

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
// matters — internal/fakeradio's own helper records it at length: net.Pipe's
// Write is a rendezvous with whichever Read is CURRENTLY blocked, so an
// abandoned goroutine's Read could swallow a later call's reply.
// SetReadDeadline cancels the same Read call and leaves no goroutine behind.
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
// fire-and-forget success is verified (doc.go's register entry AN ACCEPTED SET
// PRODUCES NO REPLY).
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

// --- ID: the CAT identity, and the byte-level difference a probe uses ---

func TestID_AnswersThisRadiosCATID(t *testing.T) {
	_, conn := newTestRadio(t)

	// Hand-written from the manual's ID chart (ft891_layout.txt:762-770):
	// "ID" + the four-digit CAT ID + ';'. The ID block prints
	// "P1 0650: FT-891" (763), and 0650 is deliberately NOT the FTdx10's
	// 0761, the FTdx101's or the FT-710's 0800 — the FT-891 driver's Open
	// turns any other value into a *driver.WrongRadioError, so this literal
	// is what makes a probe against this fake succeed and a probe against any
	// sibling fake fail.
	if got, want := exchange(t, conn, "ID;"), "ID0650;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
}

func TestID_MalformedBodyRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	// ID has no Set direction at all on this radio — the command list gives it
	// "X O O X" (ft891_layout.txt:147) — so a body of any kind is not a
	// command.
	assertRejected(t, conn, "ID0650;")
	assertRejected(t, conn, "ID0;")
}

// --- AI: accepted, readable, and silent on Set ---

func TestAI_SetIsSilentAndReadReportsIt(t *testing.T) {
	_, conn := newTestRadio(t)

	// OFF at construction, a MANUAL FACT of this radio — "This parameter is
	// set to '0' (OFF) automatically when the transceiver is turned 'OFF'"
	// (ft891_layout.txt:231, inside AI's own block at 226-235). See doc.go's
	// "What is NOT in this register, and why".
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; on a fresh radio -> %q, want %q", got, want)
	}

	// The Set core/transport.Engine.Init opens every session with. Silence is
	// success (the register's AN ACCEPTED SET PRODUCES NO REPLY entry).
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

// --- Dispatch: unknown commands, case, framing ---

func TestUnknownCommandRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ZZ;")
	assertRejected(t, conn, "FA014250000;") // a real FT-891 VFO command this fake does not model
	assertRejected(t, conn, "M;")           // one byte of a command name
	assertRejected(t, conn, ";")            // an empty frame
}

// TestCommandNamesAreUpperCaseOnly pins the narrower of the two readings this
// repository can defend, and it is an ASSUMPTION — doc.go's register entry
// COMMAND NAMES ARE UPPER CASE ONLY.
//
// The FTdx10's manual states the leniency in terms ("You may use either lower
// or upper case characters", ftdx10_layout.txt:160-161) and internal/fakedx10
// accepts either case on that line. NOTHING IN THIS REPOSITORY CITES SUCH A
// SENTENCE FOR THE FT-891: the transcription of this manual's own folio-2
// "Control Command" prose (core/cat/ft891/testdata/provenance.md, "Pages
// read", PDF page 3) records the two-character command name, the parameters,
// the terminator and the inapplicable-digit note, and no statement about
// case at all.
//
// Strict is the fail-LOUD direction: a fake stricter than the radio makes a
// test that expected acceptance fail visibly, where a fake more lenient than
// the radio lets a test pass that real hardware would refuse. Nothing is lost
// meanwhile — every frame core/cat builds is upper case.
func TestCommandNamesAreUpperCaseOnly(t *testing.T) {
	_, conn := newTestRadio(t)

	assertRejected(t, conn, "id;")
	assertRejected(t, conn, "Id;")
	if got, want := exchange(t, conn, "ID;"), "ID0650;"; got != want {
		t.Errorf("ID; -> %q, want %q — the lower-case rejections above prove nothing if the upper-case form is refused too", got, want)
	}
}

func TestFramingSplitsAcrossWritesAndCoalescedFrames(t *testing.T) {
	_, conn := newTestRadio(t)

	// One frame arriving in three writes.
	writeFrame(t, conn, "I")
	writeFrame(t, conn, "D")
	writeFrame(t, conn, ";")
	if got, want := mustReadFrame(t, conn), "ID0650;"; got != want {
		t.Errorf("split ID; -> %q, want %q", got, want)
	}

	// Three frames arriving in one write, answered in order.
	writeFrame(t, conn, "ID;AI;ID;")
	for i, want := range []string{"ID0650;", "AI0;", "ID0650;"} {
		if got := mustReadFrame(t, conn); got != want {
			t.Errorf("coalesced reply %d = %q, want %q", i, got, want)
		}
	}
}

func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)

	// doc.go's register entry THE FRAME ACCUMULATOR'S CAP AND RESYNC (this
	// package's own bounded-input policy, not a radio claim): more than
	// maxAccumulatorBytes without a ';' draws one "?;", then bytes are
	// discarded up to and including the next ';'.
	garbage := strings.Repeat("X", maxAccumulatorBytes+1)
	writeFrame(t, conn, garbage)
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow -> %q, want %q", got, want)
	}

	// The tail of the overflowed frame is swallowed, up to its terminator, and
	// framing then resumes: the ID; after it is answered normally, and the
	// swallowed remainder produces no second reply.
	writeFrame(t, conn, "XXXX;ID;")
	if got, want := mustReadFrame(t, conn), "ID0650;"; got != want {
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

// --- MC: the current-channel answer, and recall (availability 160; frames
// 906-915) ---

func TestMC_ReadAnswersTheNoneFormUntilARecall(t *testing.T) {
	_, conn := newTestRadio(t)

	// Hand-written from the MC chart (ft891_layout.txt:906-915): "MC" + the
	// 3-byte current channel + ';'. "000" is the DIALECT's ASSUMED none form
	// (core/cat/ft891/doc.go's register entry "SlotSpace.NoneWire"), which
	// this fake reports until an MC-set.
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; on a fresh radio -> %q, want %q", got, want)
	}
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("the selection reads back as %q, want %q", got, want)
	}
}

// selectionOverTheWire reads the selection back over the WIRE and returns the
// slot the answer names, so a test asserting on the selection asserts on what
// a host would see rather than on package state.
func selectionOverTheWire(t *testing.T, conn io.ReadWriteCloser) string {
	t.Helper()
	got := exchange(t, conn, "MC;")
	if len(got) != 6 || !strings.HasPrefix(got, "MC") || got[5] != ';' {
		t.Fatalf("MC; -> %q, want a 6-byte \"MC\" answer", got)
	}
	return got[2:5]
}

func TestMC_RecallOfAPopulatedSlotIsSilentAndMovesTheSelection(t *testing.T) {
	r, conn := newTestRadio(t, WithSlot("001", ordinaryState()))

	writeFrame(t, conn, "MC001;")
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

	// ASSUMED — doc.go's register entry EMPTY-SLOT ANSWERS: a channel with no
	// stored data cannot be recalled, paired with the read rule.
	assertRejected(t, conn, "MC050;")
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("a rejected recall moved the selection to %q, want it left at %q", got, want)
	}
}

// TestMC_RefusesTheSlotClassesItsLegendDoesNotName is the fake's twin of
// core/cat/ft891's TestDifferencePinMCSelects. This radio's MC block prints
// TWO slot classes only — "001 - 099: Regular Memory Channel" and "P1L - P9U
// (PMS)" (ft891_layout.txt:907-909) — where every registered sibling's MC
// legend prints 5xx and EMG as well. So a 5 MHz channel this fake will happily
// answer over MR cannot be recalled by MC, and that is the legend rather than
// a policy.
func TestMC_RefusesTheSlotClassesItsLegendDoesNotName(t *testing.T) {
	for _, slot := range []string{"501", "510", "EMG"} {
		r, conn := newTestRadio(t, WithSlot(slot, ordinaryState()))

		// Populated, and MR proves it in the same session, so the MC refusal
		// below cannot be an empty slot in disguise.
		if got := exchange(t, conn, "MR"+slot+";"); got == "?;" {
			t.Fatalf("MR%s; -> %q, so the MC refusal proves nothing", slot, got)
		}
		assertRejected(t, conn, "MC"+slot+";")
		if got, want := r.CurrentChannel(), "000"; got != want {
			t.Errorf("a refused recall moved the selection to %q, want %q", got, want)
		}
	}
}

func TestMC_MalformedRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MC000;")  // the answer-only none form is never a request
	assertRejected(t, conn, "MC100;")  // grammatical, out of inventory
	assertRejected(t, conn, "MC01;")   // two-byte slot
	assertRejected(t, conn, "MC0011;") // four-byte slot
}

// TestSetsDoNotMoveTheSelection pins the DELIBERATE NON-BORROWING doc.go's
// register entry A SET DOES NOT MOVE THE SELECTED CHANNEL records:
// internal/fakeradio's MW moves the FT-710's selection on that radio's own
// hardware finding, and inventing the same side effect for a radio nobody has
// written to is exactly the borrowed-fact class this package refuses.
func TestSetsDoNotMoveTheSelection(t *testing.T) {
	r, conn := newTestRadio(t)

	writeFrame(t, conn, ordinaryChannel("007", '0').frame())
	assertNoReply(t, conn)
	if _, ok := r.SlotState("007"); !ok {
		t.Fatal("the Set did not land, so the selection assertion below proves nothing")
	}
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("an MT Set moved the selection to %q, want it left at %q", got, want)
	}
}

// --- EX (MENU): deliberately absent, for now ---

// TestEX_IsDeliberatelyAbsentUntilItsGenerator pins the STUB as a stub. Every
// EX frame — a well-formed four-digit read of the chart's first and last
// addresses, a six-digit read of the shape every registered sibling uses, a
// Set-shaped body, a malformed one — draws the same "?;" as an unknown
// command, because this fake holds no menu state at all yet.
//
// It is here so that the absence reads as a decision rather than as an
// oversight, and so that the task which lands ex.go, this package's own copy
// of transcription B and the generator behind it REPLACES a test rather than
// merely adding one. See doc.go's "What this fake deliberately does NOT
// model".
//
// Note what this test does NOT claim: that a real FT-891 refuses EX. It
// documents this fake's state, and the "?;" it asserts is the stub's, not the
// radio's.
func TestEX_IsDeliberatelyAbsentUntilItsGenerator(t *testing.T) {
	_, conn := newTestRadio(t)

	// This radio's EX Read is SEVEN bytes — "EX P1 P1 P1 P1 ;", four address
	// digits (ft891_layout.txt:513-522) — where every registered sibling's is
	// nine. 0101 and 1803 are the chart's first and last rows
	// (core/cat/ft891/testdata/provenance.md §EX).
	for _, frame := range []string{
		"EX0101;",     // this radio's own four-digit read, first row
		"EX1803;",     // ... and last row
		"EX010101;",   // the siblings' six-digit shape
		"EX0101011;",  // a Set-shaped body
		"EX;",         // no address at all
		"EX99999999;", // nonsense
	} {
		assertRejected(t, conn, frame)
	}

	// Indistinguishable from an unknown command, which is the whole of the
	// convention while the arm is a stub.
	if got, want := exchange(t, conn, "EX0101;"), exchange(t, conn, "ZZ;"); got != want {
		t.Errorf("EX0101; -> %q but ZZ; -> %q — while EX is unmodelled the two must be the same answer", got, want)
	}
}
