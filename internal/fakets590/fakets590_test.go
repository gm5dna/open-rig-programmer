// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

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
func newTestRadio(t *testing.T, row Row, opts ...Option) (*Radio, io.ReadWriteCloser) {
	t.Helper()
	r := New(row, opts...)
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
// NAK and nothing after it within the latency window. The 590SG's error
// table prints "?;" as the whole of the failure vocabulary (590:93-108), so
// a passing rejection is not merely "the first frame back is '?;'" but that
// no second frame follows it.
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// --- The row argument, which has no zero value ---

// TestNew_RefusesAnUnsetRow pins the fail-closed constructor. The plan's P1
// makes the row REQUIRED on both the driver and the fake ("model argument
// likewise REQUIRED"), because the two siblings share one book and one
// 50-byte grid and differ on the two axes this fake can actually show — the
// ID answer (590:1114-1116) and byte 28's liveness (590:1478). A zero value
// that quietly meant "the S" would make every SG test a fixture accident.
//
// A panic rather than an error return is MustNewLayout's reasoning one layer
// down: every call site passes a compile-time-known constant, so an unset row
// is a programming error in a fixture and must stop the programme rather than
// be threaded through an error nobody can act on.
func TestNew_RefusesAnUnsetRow(t *testing.T) {
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("New(RowUnset) returned instead of panicking — an unset row must fail closed")
		}
		if msg, ok := got.(string); !ok || !strings.Contains(msg, "row") {
			t.Errorf("panic value %v does not name the row", got)
		}
	}()
	_ = New(RowUnset)
}

// TestRow_String pins the two names refusals and test failures print.
func TestRow_String(t *testing.T) {
	for _, tt := range []struct {
		row  Row
		want string
	}{
		{RowUnset, "RowUnset"},
		{RowS, "TS-590S"},
		{RowSG, "TS-590SG"},
		{Row(99), "RowUnset"},
	} {
		if got := tt.row.String(); got != tt.want {
			t.Errorf("Row(%d).String() = %q, want %q", int(tt.row), got, tt.want)
		}
	}
}

// --- ID: the byte-level difference a probe uses (590:1111-1119) ---

// TestID_AnswersThisRowsPrintedNumber writes the two answers as literals
// hand-copied from the ID block: "021: TS-590S" and "023: TS-590SG"
// (590:1114-1116) in the five-byte answer frame "I D P1 P1 P1 ;"
// (590:1119).
//
// 022 and 024 are deliberately NOT produced by any row of this fake: the
// spec's error handling names them as the two nearby Kenwood identities a
// probe must refuse BY NAME, and a fake that could answer one would make the
// driver's wrong-radio pin a test of its own script.
func TestID_AnswersThisRowsPrintedNumber(t *testing.T) {
	for _, tt := range []struct {
		row  Row
		want string
	}{
		{RowS, "ID021;"},
		{RowSG, "ID023;"},
	} {
		t.Run(tt.row.String(), func(t *testing.T) {
			_, conn := newTestRadio(t, tt.row)
			if got := exchange(t, conn, "ID;"); got != tt.want {
				t.Errorf("ID; -> %q, want %q", got, tt.want)
			}
		})
	}
}

// TestID_HasNoSetDirection: the ID block prints a Read and an Answer and
// nothing else (590:1111-1119), so anything between "ID" and ';' is unknown.
func TestID_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)
	assertRejected(t, conn, "ID023;")
}

// --- FV: four characters, and the one printed example (590:1030-1037) ---

// TestFV_AnswersFourCharacters pins the width and the default value. The
// chart's answer row is "F V P1 P1 P1 P1 ;" — four P1 bytes (590:1037) — and
// the book's one worked example is "FV1.00;" (590:1035). Menu 000 gives the
// width a second printed corroborator, "Version information (4 ASCII
// characters) read only" (590:749).
func TestFV_AnswersFourCharacters(t *testing.T) {
	for _, row := range []Row{RowS, RowSG} {
		t.Run(row.String(), func(t *testing.T) {
			_, conn := newTestRadio(t, row)
			got := exchange(t, conn, "FV;")
			if want := "FV1.00;"; got != want {
				t.Errorf("FV; -> %q, want %q", got, want)
			}
			if len(got) != 7 {
				t.Errorf("FV answer is %d bytes, want 7 — two command bytes, four P1 bytes and the terminator (590:1037)", len(got))
			}
		})
	}
}

// --- AI: the initial state is a MANUAL FACT (590:81-82) ---

// TestAI_InitialStateIsOffAndASetIsSilent pins three things at once:
// construction leaves AI OFF — "Turn this function on using the AI command
// (the initial state is OFF)" (590:81-82), a manual fact and not an
// assumption — an accepted Set answers nothing, and a Read reports what the
// Set stored.
//
// core/transport.Engine.Init opens every session with "AI0;", so the silent
// accept path is on the critical path of every fake session.
func TestAI_InitialStateIsOffAndASetIsSilent(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Fatalf("AI; -> %q, want %q at construction", got, want)
	}

	writeFrame(t, conn, "AI2;")
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "AI;"), "AI2;"; got != want {
		t.Errorf("AI; -> %q after AI2;, want %q", got, want)
	}
}

// TestAI_AdmitsOnlyThe590PairsPrintedValues. This book prints "0: AI OFF /
// 2: AI ON (without backup) / 4: AI ON (with backup)" (590:159-162) and no 1
// or 3 at all — where the TS-480's own AI legend prints 0, 1, 2 and 3
// (480:185-190). The two value sets are different, which is why nothing in
// this package may be borrowed by the 480's fake.
func TestAI_AdmitsOnlyThe590PairsPrintedValues(t *testing.T) {
	for _, ok := range []string{"0", "2", "4"} {
		_, conn := newTestRadio(t, RowSG)
		writeFrame(t, conn, "AI"+ok+";")
		assertNoReply(t, conn)
		if got, want := exchange(t, conn, "AI;"), "AI"+ok+";"; got != want {
			t.Errorf("AI%s; then AI; -> %q, want %q", ok, got, want)
		}
	}
	for _, bad := range []string{"1", "3", "5", "9", " ", "00"} {
		_, conn := newTestRadio(t, RowSG)
		assertRejected(t, conn, "AI"+bad+";")
		if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
			t.Errorf("after the refused AI%s;, AI; -> %q, want the unchanged %q", bad, got, want)
		}
	}
}

// --- The frame grammar itself ---

// TestCommandNamesAreAcceptedInEitherCase pins the front matter's own
// sentence: "A command consists of 2 or 3 characters. You may use either
// lower or upper case characters." (590:62-63). A MANUAL FACT, and the same
// sentence the Yaesu books print, so the mixed-case form is a CONSEQUENCE of
// folding each name byte independently rather than a separate leniency.
//
// FIELD VALUES REMAIN CASE-SENSITIVE: the sentence is about the command
// name and says nothing about parameters.
func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	for _, send := range []string{"ID;", "id;", "iD;", "Id;"} {
		_, conn := newTestRadio(t, RowS)
		if got, want := exchange(t, conn, send), "ID021;"; got != want {
			t.Errorf("%q -> %q, want %q", send, got, want)
		}
	}
}

// TestUnknownCommandIsRejected: every command this fake does not serve draws
// the one unattributed NAK (590:93-108).
//
// TWO OF THE ENTRIES BELOW ARE DIFFERENT KINDS OF THING, and the difference
// matters. "TY;" is a command this book does not contain at all — it is the
// TS-480's microprocessor-type read (480:1626-1629), and the probe sends it
// only to a radio that answered ID "020" — so refusing it is what a real
// TS-590 must do and is a fact about the radio. "FA;" and "IF;" ARE printed
// here (590:959, 590:1128) and are simply not modelled: no layer above this
// fake sends either, and refusing them is this package's unknown-command
// path, not a claim about the radio.
func TestUnknownCommandIsRejected(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)
	for _, send := range []string{"XX;", "FA;", "IF;", ";", "M;", "TY;"} {
		assertRejected(t, conn, send)
	}
}

// The EX (MENU) surface has its own file: ex.go and ex_test.go. Until this
// milestone's task 17 it was a modelling gap pinned here as
// TestEX_IsNotModelledYet; the gap that remains — the EX SET — is pinned by
// ex_test.go's TestEX_MalformedAndSetShapedBodiesAreRefused, beside the read
// it is a gap in.

// --- The two serial-line tokens, and the transient "?;" (590:93-113) ---

// TestWithStreamError_ReplacesThatExchangesReply pins both tokens, in both
// places a driver can meet them: instead of an ANSWER, and instead of a
// fire-and-forget silence. They are not command outcomes — "E;" is a
// communication error such as an overrun or framing error (590:110-112) and
// "O;" a receive buffer overrun (590:113) — so they replace whatever the
// exchange would have produced.
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
			_, conn := newTestRadio(t, RowSG, WithStreamError(tt.kind, 1))
			if got := exchange(t, conn, "ID;"); got != tt.want {
				t.Errorf("ID; -> %q, want %q", got, tt.want)
			}
			// The next exchange is untouched.
			if got, want := exchange(t, conn, "ID;"), "ID023;"; got != want {
				t.Errorf("the second ID; -> %q, want the ordinary %q", got, want)
			}

			// In place of a fire-and-forget silence: the AI Set at exchange
			// 2 answers nothing on an ordinary radio.
			_, conn2 := newTestRadio(t, RowSG, WithStreamError(tt.kind, 2))
			if got, want := exchange(t, conn2, "ID;"), "ID023;"; got != want {
				t.Fatalf("exchange 1 -> %q, want %q", got, want)
			}
			if got := exchange(t, conn2, "AI0;"); got != tt.want {
				t.Errorf("the silent AI0; at exchange 2 -> %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWithStreamError_RefusesAnUnsetTokenOrAZeroExchange: the two tokens have
// different printed causes (590:110-113), so a scripted fault that defaulted
// to one of them would put a test on the wrong sentence of the book.
func TestWithStreamError_RefusesAnUnsetTokenOrAZeroExchange(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func()
	}{
		{"the unset token", func() { _ = New(RowSG, WithStreamError(StreamErrorUnset, 1)) }},
		{"exchange 0", func() { _ = New(RowSG, WithStreamError(StreamErrorE, 0)) }},
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
// printed under the "?;" row itself: "Occasionally, this message may not
// appear due to microprocessor transients in the transceiver."
// (590:106-108). A Kenwood host therefore cannot read silence as "the radio
// did not refuse" — a timeout and a rejection are the same event seen twice —
// and this option is what lets a driver's typed read failure be pinned
// against that branch through a real fake.
//
// Answers and accepted Sets are untouched: the sentence is about the "?;"
// message alone.
func TestWithTransientNAKSuppressed_DropsTheNAKAndNothingElse(t *testing.T) {
	_, conn := newTestRadio(t, RowSG, WithTransientNAKSuppressed())

	writeFrame(t, conn, "XX;")
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "ID;"), "ID023;"; got != want {
		t.Errorf("ID; -> %q, want the untouched %q", got, want)
	}
	writeFrame(t, conn, "AI0;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; -> %q, want %q — an accepted Set is not affected", got, want)
	}
}

// TestWithStreamError_IsNotSuppressedByTheTransientOption: the transient note
// is about "?;" and says nothing about the two serial-line tokens, so a
// composition of the two options must still put "E;" on the wire.
func TestWithStreamError_IsNotSuppressedByTheTransientOption(t *testing.T) {
	_, conn := newTestRadio(t, RowSG, WithTransientNAKSuppressed(), WithStreamError(StreamErrorE, 1))
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
	r := New(RowSG)
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
	r := New(RowSG, WithLatency(30*time.Second))
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

// TestAccumulatorOverflowRejectsOnceAndResyncs: the reassembler's cap is this
// package's own bounded-input policy (register entry THE FRAME ACCUMULATOR'S
// CAP AND RESYNC), not a figure either book prints. One "?;" per overflow,
// then everything up to and including the next ';' is discarded and framing
// resumes.
func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t, RowS)

	writeFrame(t, conn, strings.Repeat("A", maxAccumulatorBytes+8))
	if got := mustReadFrame(t, conn); got != "?;" {
		t.Fatalf("overflow -> %q, want exactly one %q", got, "?;")
	}
	assertNoReply(t, conn)

	// The rest of the runaway frame, then a good one: only the good one is
	// answered.
	writeFrame(t, conn, "still rubbish;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "ID;"), "ID021;"; got != want {
		t.Errorf("after resync, ID; -> %q, want %q", got, want)
	}
}
