// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// EVERY EXPECTED REPLY IN THIS PACKAGE'S TESTS IS RECOMPUTED INDEPENDENTLY —
// as a literal string, or by the test-local assembler in parser_test.go which
// places bytes at the positions the chart numbers — never by calling this
// package's own builders. That independence is the whole point: it must be
// possible for a builder to have a bug and still be CAUGHT, which cannot
// happen if the expectation comes from the same buggy function.

const testTimeout = 2 * time.Second

// newTestRadio constructs a *Radio for a test, registers its Close() as
// cleanup, and returns both the Radio (for ChannelState assertions) and its
// Port().
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
// matters — net.Pipe's Write is a rendezvous with whichever Read is CURRENTLY
// blocked, so an abandoned goroutine's Read could swallow a later call's
// reply. SetReadDeadline cancels the same Read call and leaves no goroutine
// behind.
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

// exchange writes one frame and returns the reply.
func exchange(t *testing.T, conn io.ReadWriteCloser, send string) string {
	t.Helper()
	writeFrame(t, conn, send)
	return mustReadFrame(t, conn)
}

// assertRejected writes send and requires EXACTLY the single unattributed NAK
// and nothing after it. This book's error table prints "?;" as the whole of
// the failure vocabulary for a command outcome (890:106-112), so a passing
// rejection is not merely "the first frame back is '?;'" but that no second
// frame follows it.
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// --- ID (890:2730-2738) ---

// TestID_AnswersThisRowsPrintedNumber. "024: TS-890S" (890:2733) is the only
// value this book prints, and the answer's shape is the five-cell grid
// "I D P1 P1 P1 ;" (890:2738). It is what makes a probe against this fake
// succeed for this registry row and produce a wrong-radio refusal for any
// other.
func TestID_AnswersThisRowsPrintedNumber(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "ID;"), "ID024;"; got != want {
		t.Errorf("ID; -> %q, want %q (890:2733)", got, want)
	}
}

// TestID_HasNoSetDirection. The ID block prints a Read and an Answer and no
// Set at all (890:2730-2738), so anything between "ID" and ';' is simply
// unknown.
func TestID_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"ID024;", "ID0;"} {
		assertRejected(t, conn, send)
	}
}

// --- FV (890:2650-2659) ---

// TestFV_AnswersTheBooksWorkedExample. The book gives the field no grammar,
// only a four-cell width (890:2659) and one worked example, "FV1.00;"
// (890:2657) — doc.go's register entry THE DEFAULT FIRMWARE STRING.
func TestFV_AnswersTheBooksWorkedExample(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "FV;"), "FV1.00;"; got != want {
		t.Errorf("FV; -> %q, want %q (890:2657)", got, want)
	}
	if len(strings.TrimSuffix(strings.TrimPrefix("FV1.00;", "FV"), ";")) != firmwareFieldLen {
		t.Errorf("the worked example's P1 field is not %d cells wide (890:2659)", firmwareFieldLen)
	}
}

func TestFV_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "FV1.00;")
}

// --- AI (890:172-184) ---

// TestAI_ReadAnswersOffAtConstruction. core/transport.Engine.Init opens every
// session with an AI-off Set, so this handler's silent-accept path is on the
// critical path of every fake session; the READ is what shows the state moved.
func TestAI_ReadAnswersOffAtConstruction(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; -> %q, want %q (890:175)", got, want)
	}
}

// TestAI_SetIsFireAndForgetAndMovesTheState over the three values this book
// prints WITH A MEANING: "0: AI OFF" (890:175), "2: AI ON (Not back up the ON
// state)" (890:179) and "4: AI ON (Back up the ON state)" (890:181).
func TestAI_SetIsFireAndForgetAndMovesTheState(t *testing.T) {
	for _, v := range []string{"0", "2", "4"} {
		t.Run(v, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, "AI"+v+";")
			assertNoReply(t, conn)
			if got, want := exchange(t, conn, "AI;"), "AI"+v+";"; got != want {
				t.Errorf("after AI%s;, AI; -> %q, want %q", v, got, want)
			}
		})
	}
}

// TestAI_RefusesTheTwoValuesPrintedNotUsed. "1: Not used" (890:177) and
// "3: Not used" (890:180) are printed in the legend and given no meaning, and
// what a radio does with one is unprinted — doc.go's register entry THE AI
// VALUES PRINTED "NOT USED" ARE REFUSED. Nothing here normalises, so the
// refusal is the strict direction: a driver that sent one would otherwise pass
// its own tests and fail on hardware.
func TestAI_RefusesTheTwoValuesPrintedNotUsed(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"AI1;", "AI3;", "AI5;", "AIx;", "AI00;"} {
		assertRejected(t, conn, send)
	}
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("after the refusals, AI; -> %q, want %q — a refused Set must not move the state", got, want)
	}
}

// TestAI_NothingIsEverPushedUnsolicited. The book says an AI-ON radio outputs
// a response whenever a parameter changes (890:183-185), and no TS-890S has
// been observed by this project; modelling silence is the honest default —
// doc.go's register entry AUTOMATIC-INFORMATION SUPPRESSION.
func TestAI_NothingIsEverPushedUnsolicited(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "AI4;")
	assertNoReply(t, conn)
	writeFrame(t, conn, plainFields("000").frame())
	assertNoReply(t, conn)
}

// --- Framing and dispatch ---

// TestCommandNamesAreAcceptedInEitherCase. A MANUAL FACT of this radio:
// "A command consists of 2 to 5 alphanumeric characters. You may use either
// lower or upper case characters." (890:76-80). The mixed-case form is a
// CONSEQUENCE of folding each name byte independently, not a separate invented
// leniency.
func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"id;", "Id;", "iD;"} {
		if got, want := exchange(t, conn, send), "ID024;"; got != want {
			t.Errorf("%q -> %q, want %q", send, got, want)
		}
	}
	// The three-character name folds the same way, and the digit is
	// untouched.
	if got := exchange(t, conn, "ma0000;"); len(got) != 40 {
		t.Errorf("ma0000; -> %q (%d bytes), want a 40-byte blank answer", got, len(got))
	}
}

// TestFieldValuesRemainCaseSensitive. The sentence at 890:76-80 is about the
// command NAME; extending it to parameters would be an invented leniency. The
// mode nibbles A-F (890:3987-3992) are the field this can be shown on.
func TestFieldValuesRemainCaseSensitive(t *testing.T) {
	_, conn := newTestRadio(t)
	f := plainFields("005")
	f.mode = "a"
	assertRejected(t, conn, f.frame())
}

// TestUnknownCommandsAreRejected, including the sibling family's own frames: a
// TS-590's MR read and MW set, and the MA-family members this milestone does
// not model.
func TestUnknownCommandsAreRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{
		"MR0000;",                             // the 590 pair's memory read
		"MC000;",                              // the 590 pair's channel selector
		"MA1" + strings.Repeat("0", 14) + ";", // MA1 Direct Write, off the roster
		"MA5000;",                             // MA5 Channel Deletion, off the roster
		"MA;",                                 // no such command: the family is MA0..MA7
		"MN007;",                              // Memory Name Set, off the roster (doc.go)
		"MN;",                                 // Memory Name Read, off the roster (doc.go)
		"M;",                                  // shorter than any name
		";",                                   // a bare terminator
		"XY;",
	} {
		assertRejected(t, conn, send)
	}
}

// TestAccumulatorOverflowRejectsOnceAndResyncs pins this package's own
// bounded-input policy — doc.go's register entry THE FRAME ACCUMULATOR'S CAP
// AND RESYNC. No Kenwood book prints a buffer size; what this one prints is
// that a receive-buffer overrun produces "O;" (890:121-123), a different event
// modelled by WithStreamError.
func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, strings.Repeat("A", maxAccumulatorBytes+1))
	if got := mustReadFrame(t, conn); got != "?;" {
		t.Errorf("overflow -> %q, want %q", got, "?;")
	}
	// Everything up to and including the next ';' is discarded, then normal
	// framing resumes.
	writeFrame(t, conn, "junk;ID;")
	if got, want := mustReadFrame(t, conn), "ID024;"; got != want {
		t.Errorf("after the resync, ID; -> %q, want %q", got, want)
	}
}

// TestFramesMayArriveSplitAcrossWrites: the reassembler's whole job.
func TestFramesMayArriveSplitAcrossWrites(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "I")
	writeFrame(t, conn, "D")
	writeFrame(t, conn, ";")
	if got, want := mustReadFrame(t, conn), "ID024;"; got != want {
		t.Errorf("split ID; -> %q, want %q", got, want)
	}
}

// TestClose_IsPromptDespiteAPendingLatency — the promptness
// internal/wiring's OpenFakeSessionFor relies on for every fake rig.
func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	conn := r.Port()
	writeFrame(t, conn, "ID;")

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- r.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: unexpected error: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("Close took %v with a 30s latency pending — the wait is not interruptible", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s while a 30s latency was pending")
	}
}

// TestClose_IsIdempotent.
func TestClose_IsIdempotent(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestPortIsStable: repeated calls return the same connection.
func TestPortIsStable(t *testing.T) {
	r, conn := newTestRadio(t)
	if r.Port() != conn {
		t.Error("Port() returned a different connection on the second call")
	}
}
