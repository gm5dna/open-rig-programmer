// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

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
// positions the two charts number (parser_test.go's recordFrame) — never by
// calling this package's own builders (buildMRAnswer, buildMCAnswer, ...).
// That independence is the whole point: it must be possible for a builder to
// have a bug and still be CAUGHT, which cannot happen if the expectation
// comes from the same buggy function.

const testTimeout = 2 * time.Second

// newTestRadio constructs a *Radio for a test, registers its Close() as
// cleanup, and returns both the Radio (for ChannelState/CurrentChannel
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
// fire-and-forget success is verified (doc.go's register entry AN ACCEPTED
// SET PRODUCES NO REPLY).
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

// assertRejected writes send and requires EXACTLY the single unattributed
// NAK and nothing after it within the latency window. This book's error
// table prints "?;" as the whole of the failure vocabulary (480:126-138), so
// a passing rejection is not merely "the first frame back is '?;'" but that
// no second frame follows it.
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// --- ID: the byte a probe turns into a wrong-radio refusal (480:676-687) ---

// TestID_AnswersThePrintedNumber writes the answer as a literal hand-copied
// from the ID block: "020: TS-480" (480:678) in the six-byte answer frame
// "I D P1 P1 P1 ;" (480:687).
//
// 021, 022, 023 and 024 are deliberately NOT producible by this fake: they
// are the nearby Kenwood identities a probe must refuse BY NAME, and a fake
// that could answer one would make the driver's wrong-radio pin a test of
// its own script.
func TestID_AnswersThePrintedNumber(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "ID;"), "ID020;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
}

// TestID_HasNoSetDirection. The ID block prints a Set LABEL over an EMPTY
// chart (480:679) — erratum E17's shape, where a transcriber who reads the
// label as evidence invents a command — so anything between "ID" and ';' is
// simply unknown.
func TestID_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ID020;")
}

// --- TY: the hardware variant, which is NOT a firmware version
// (480:1621-1634) ---

// TestTY_AnswersTheDefaultVariant pins the six-byte answer frame
// "T Y P1 P1 P2 ;" (480:1634): two reserved bytes and one variant digit.
// The shipped default is "00" and '0' — doc.go's register entry THE DEFAULT
// TY ANSWER — because P1 has no legend at all (480:1623) and '0' is P2's
// first printed variant, "0: TS-480HX (200 W)" (480:1626).
func TestTY_AnswersTheDefaultVariant(t *testing.T) {
	_, conn := newTestRadio(t)
	got := exchange(t, conn, "TY;")
	if want := "TY000;"; got != want {
		t.Errorf("TY; -> %q, want %q", got, want)
	}
	if len(got) != 6 {
		t.Errorf("TY answer is %d bytes, want 6 — two command bytes, two reserved, one variant and the terminator (480:1634)", len(got))
	}
}

// TestTY_HasNoSetDirection. TY is headed "Sets or reads the microprocessor
// fimware type" and its Set chart is EMPTY (480:1621, 480:1625) — erratum
// E10 — so the heading does not make a Set exist and this fake refuses one.
func TestTY_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "TY000;")
}

// TestWithTYAnswer_IsAnsweredVerbatim. The option exists to make two driver
// paths reachable through a real fake rather than through a scripted
// transcript: the session refusal of a FIFTH variant (P2='4', which the
// document does not print — 480:1626-1629 prints exactly four), and P1's
// OPAQUE bytes, which are the one field in this family admitted above 0x7E.
//
// Nothing is validated but the width and the terminator: interpreting P1 is
// no part of this radio's own document, and a fake that applied a grammar to
// it would be asserting one.
func TestWithTYAnswer_IsAnsweredVerbatim(t *testing.T) {
	for _, tt := range []struct {
		name     string
		reserved string
		variant  byte
		want     string
	}{
		{"the second printed variant", "00", '1', "TY001;"},
		{"the fourth printed variant", "00", '3', "TY003;"},
		{"a fifth variant the book does not print", "00", '4', "TY004;"},
		{"reserved bytes above 0x7E", "\x7f\xff", '0', "TY\x7f\xff0;"},
		{"reserved bytes that are ordinary text", "AB", '2', "TYAB2;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t, WithTYAnswer(tt.reserved, tt.variant))
			if got := exchange(t, conn, "TY;"); got != tt.want {
				t.Errorf("TY; -> %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWithTYAnswer_RefusesAFixtureNoRadioCouldSend. P1 is two bytes on the
// wire (480:1634), so a fixture of any other width could not be sent by any
// radio; and a ';' anywhere in the answer is a SECOND FRAME to the host's
// own reassembler, not a byte of this one. Panicking is the reasoning
// core/kw.MustNewLayout applies to the same kind of argument: the value is a
// fixture constant, known at compile time, so a bad one is a programming
// error and must stop the programme rather than be threaded through an
// error nobody can act on.
func TestWithTYAnswer_RefusesAFixtureNoRadioCouldSend(t *testing.T) {
	for _, tt := range []struct {
		name     string
		reserved string
		variant  byte
	}{
		{"one reserved byte", "0", '0'},
		{"three reserved bytes", "000", '0'},
		{"a terminator inside P1", "0;", '0'},
		{"a terminator as the variant", "00", ';'},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("accepted, want a panic")
				}
			}()
			_ = New(WithTYAnswer(tt.reserved, tt.variant))
		})
	}
}

// TestFV_IsNotACommandOfThisRadio. "FV" appears NOWHERE in the 2003 TS-480
// document — no chart, no mention — so refusing it is a fact about this
// radio and not a modelling gap: this book's nearest equivalent is TY, which
// reports a hardware variant rather than a version, and the document carries
// no firmware statement at all (erratum E15).
//
// The distinction matters: ex_test.go's
// TestEX_MalformedAndSetShapedBodiesAreRefused refuses a DIRECTION of a
// command this book prints and this fake serves, and that one is a gap.
func TestFV_IsNotACommandOfThisRadio(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"FV;", "FV1.00;"} {
		assertRejected(t, conn, send)
	}
}

// --- AI: four values, and a legend that is NOT the 590 pair's
// (480:183-198) ---

// TestAI_InitialStateIsZeroAndASetIsSilent pins three things at once:
// construction leaves AI at '0', an accepted Set answers nothing, and a Read
// reports what the Set stored.
//
// core/transport.Engine.Init opens every session with an AI-off Set, so the
// silent accept path is on the critical path of every fake session.
func TestAI_InitialStateIsZeroAndASetIsSilent(t *testing.T) {
	_, conn := newTestRadio(t)

	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Fatalf("AI; -> %q, want %q at construction", got, want)
	}

	writeFrame(t, conn, "AI2;")
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "AI;"), "AI2;"; got != want {
		t.Errorf("AI; -> %q after AI2;, want %q", got, want)
	}
}

// TestAI_AdmitsOnlyThisBooksPrintedValues. This book prints FOUR
// CONSECUTIVE values — "0: AI OFF / 1: Only old AI format is ON / 2: Only
// extended AI format is ON / 3: Both formats are ON" (480:185-190) — where
// the TS-590S/SG book prints 0, 2 and 4 with different meanings and no 1 or
// 3 at all. There is no non-zero value that means the same thing on both
// radios, which is why no Kenwood fake may borrow another's tables.
func TestAI_AdmitsOnlyThisBooksPrintedValues(t *testing.T) {
	for _, ok := range []string{"0", "1", "2", "3"} {
		_, conn := newTestRadio(t)
		writeFrame(t, conn, "AI"+ok+";")
		assertNoReply(t, conn)
		if got, want := exchange(t, conn, "AI;"), "AI"+ok+";"; got != want {
			t.Errorf("AI%s; then AI; -> %q, want %q", ok, got, want)
		}
	}
	for _, bad := range []string{"4", "5", "9", " ", "00"} {
		_, conn := newTestRadio(t)
		assertRejected(t, conn, "AI"+bad+";")
		if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
			t.Errorf("after the refused AI%s;, AI; -> %q, want the unchanged %q", bad, got, want)
		}
	}
}

// --- The frame grammar itself ---

// TestCommandNamesAreAcceptedInEitherCase pins the front matter's own
// sentence: "A command consists of 2 alphabetical characters. You may use
// either lower or upper case characters." (480:76-78). A MANUAL FACT, and
// the mixed-case form is a CONSEQUENCE of folding each name byte
// independently rather than a separate leniency.
//
// FIELD VALUES REMAIN CASE-SENSITIVE: the sentence is about the command name
// and says nothing about parameters.
func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	for _, send := range []string{"ID;", "id;", "iD;", "Id;"} {
		_, conn := newTestRadio(t)
		if got, want := exchange(t, conn, send), "ID020;"; got != want {
			t.Errorf("%q -> %q, want %q", send, got, want)
		}
	}
}

// TestUnknownCommandIsRejected: every command this fake does not serve draws
// the one unattributed NAK (480:126-138).
//
// THE ENTRIES BELOW ARE DIFFERENT KINDS OF THING. "FV;" is a command this
// book does not contain at all, so refusing it is what a real TS-480 must do
// (see TestFV_IsNotACommandOfThisRadio). "FA;" and "IF;" ARE printed here
// (480:546, 480:694) and are simply not modelled: no layer above this fake
// sends either, and refusing them is this package's unknown-command path,
// not a claim about the radio.
func TestUnknownCommandIsRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"XX;", "FA;", "IF;", ";", "M;", "FV;"} {
		assertRejected(t, conn, send)
	}
}

// The EX (MENU) surface has its own file: ex.go and ex_test.go. Until this
// milestone's task 17 it was a modelling gap pinned here as
// TestEX_IsNotModelledYet; the gap that remains — the EX SET — is pinned by
// ex_test.go's TestEX_MalformedAndSetShapedBodiesAreRefused, beside the read
// it is a gap in.

// --- The two serial-line tokens, and the transient "?;" (480:126-144) ---

// TestWithStreamError_ReplacesThatExchangesReply pins both tokens, in both
// places a driver can meet them: instead of an ANSWER, and instead of a
// fire-and-forget silence. They are not command outcomes — "E;" is "A
// communication error occurred such as an overrun or framing error during a
// serial data transmission" (480:140-142) and "O;" is "Receive data was sent
// but processing was not completed" (480:143-144) — so they replace whatever
// the exchange would have produced.
func TestWithStreamError_ReplacesThatExchangesReply(t *testing.T) {
	for _, tt := range []struct {
		kind StreamError
		want string
	}{
		{StreamErrorE, "E;"},
		{StreamErrorO, "O;"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			// In place of an answer.
			_, conn := newTestRadio(t, WithStreamError(tt.kind, 1))
			if got := exchange(t, conn, "ID;"); got != tt.want {
				t.Errorf("ID; -> %q, want %q", got, tt.want)
			}
			// The next exchange is untouched.
			if got, want := exchange(t, conn, "ID;"), "ID020;"; got != want {
				t.Errorf("the second ID; -> %q, want the ordinary %q", got, want)
			}

			// In place of a fire-and-forget silence: the AI Set at exchange
			// 2 answers nothing on an ordinary radio.
			_, conn2 := newTestRadio(t, WithStreamError(tt.kind, 2))
			if got, want := exchange(t, conn2, "ID;"), "ID020;"; got != want {
				t.Fatalf("exchange 1 -> %q, want %q", got, want)
			}
			if got := exchange(t, conn2, "AI0;"); got != tt.want {
				t.Errorf("the silent AI0; at exchange 2 -> %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWithStreamError_RefusesAnUnsetTokenOrAZeroExchange: the two tokens
// have different printed causes (480:140-144), so a scripted fault that
// defaulted to one of them would put a test on the wrong sentence of the
// book.
func TestWithStreamError_RefusesAnUnsetTokenOrAZeroExchange(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func()
	}{
		{"the unset token", func() { _ = New(WithStreamError(StreamErrorUnset, 1)) }},
		{"exchange 0", func() { _ = New(WithStreamError(StreamErrorE, 0)) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("accepted, want a panic")
				}
			}()
			tt.call()
		})
	}
}

// TestWithTransientNAKSuppressed_DropsTheNAKAndNothingElse plays the Note
// printed under the "?;" row itself: "Occasionally this message may not
// appear due to microprocessor transients in the transceiver."
// (480:136-138). A Kenwood host therefore cannot read silence as "the radio
// did not refuse" — a timeout and a rejection are the same event seen twice
// — and this option is what lets a driver's typed read failure be pinned
// against that branch through a real fake.
//
// Answers and accepted Sets are untouched: the sentence is about the "?;"
// message alone.
func TestWithTransientNAKSuppressed_DropsTheNAKAndNothingElse(t *testing.T) {
	_, conn := newTestRadio(t, WithTransientNAKSuppressed())

	writeFrame(t, conn, "XX;")
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "ID;"), "ID020;"; got != want {
		t.Errorf("ID; -> %q, want the untouched %q", got, want)
	}
	writeFrame(t, conn, "AI0;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; -> %q, want %q — an accepted Set is not affected", got, want)
	}
}

// TestWithStreamError_IsNotSuppressedByTheTransientOption: the transient
// note is about "?;" and says nothing about the two serial-line tokens, so a
// composition of the two options must still put "E;" on the wire.
func TestWithStreamError_IsNotSuppressedByTheTransientOption(t *testing.T) {
	_, conn := newTestRadio(t, WithTransientNAKSuppressed(), WithStreamError(StreamErrorE, 1))
	if got, want := exchange(t, conn, "XX;"), "E;"; got != want {
		t.Errorf("XX; -> %q, want %q", got, want)
	}
}

// --- Close, and the pipe direction ---

// TestClose_HostSeesEOF pins the direction internal/fakeradio's reasoning
// establishes: Close() closes only the RADIO's end, so a pending or later
// Read on the host end sees io.EOF — "the radio went away" — rather than
// io.ErrClosedPipe.
func TestClose_HostSeesEOF(t *testing.T) {
	r := New()
	conn := r.Port()
	if err := r.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
	buf := make([]byte, 8)
	if _, err := conn.Read(buf); !errors.Is(err, io.EOF) {
		t.Errorf("host Read after Close: %v, want io.EOF", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close: %v, want nil — Close is idempotent", err)
	}
}

// TestClose_IsPromptDespiteAPendingLatency: internal/wiring's
// OpenFakeSessionFor closes every fake rig on the way out, so a scripted
// multi-second latency must not become a multi-second teardown.
func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	conn := r.Port()
	writeFrame(t, conn, "ID;")

	done := make(chan struct{})
	go func() {
		_ = r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Close did not return within the test timeout while a latency wait was pending")
	}
}

// TestAccumulatorOverflowRejectsOnceAndResyncs: the reassembler's cap is
// this package's own bounded-input policy (register entry THE FRAME
// ACCUMULATOR'S CAP AND RESYNC), not a figure this book prints. One "?;" per
// overflow, then everything up to and including the next ';' is discarded
// and framing resumes.
func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)

	writeFrame(t, conn, strings.Repeat("A", maxAccumulatorBytes+8))
	if got := mustReadFrame(t, conn); got != "?;" {
		t.Fatalf("overflow -> %q, want exactly one %q", got, "?;")
	}
	assertNoReply(t, conn)

	// The rest of the runaway frame, then a good one: only the good one is
	// answered.
	writeFrame(t, conn, "still rubbish;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "ID;"), "ID020;"; got != want {
		t.Errorf("after resync, ID; -> %q, want %q", got, want)
	}
}
