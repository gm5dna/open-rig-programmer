// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// EVERY EXPECTED REPLY IN THIS PACKAGE'S TESTS IS RECOMPUTED INDEPENDENTLY — as
// a literal string, or by a test-local assembler that places bytes at the
// positions the manual's charts number (parser_test.go's combinedFrame) — never
// by calling this package's own builders (buildMTAnswer, buildMRAnswer,
// buildMCAnswer, buildIDAnswer, ...). That independence is the whole point of a
// golden test: it must be possible for a builder to have a bug and still be
// CAUGHT, which cannot happen if the expectation comes from the same buggy
// function.

const testTimeout = 2 * time.Second

// ctor is one of this package's two constructors, so that a test which must run
// against BOTH RADIOS can name them in a table rather than duplicating itself.
type ctor func(...Option) *Radio

// models is the pair every per-model test walks: the constructor, the model's
// name for failure messages, and the CAT ID its ID answer must carry — the
// latter written out here as a literal from ID's own P1 legend (layout 1070 and
// 1072) rather than taken from the package's catIDD/catIDMP constants, which
// are the values under test.
var models = []struct {
	name  string
	new   ctor
	catID string
}{
	{"FTDX101D", NewD, "0681"},
	{"FTDX101MP", NewMP, "0682"},
}

// newTestRadio constructs a fake FTDX101D for a test, registers its Close() as
// cleanup, and returns both the Radio (for SlotState/CurrentChannel assertions)
// and its Port().
//
// The D is the default for tests that are not ABOUT the model distinction,
// because one of the two has to be and the D is the model the manual's own
// title names first. Every such test's subject — the memory-channel surface —
// is printed once for both radios (matrix §2.5), and
// TestTheTwoModelsDifferOnlyInTheIDAnswer is what proves choosing one here
// costs nothing.
func newTestRadio(t *testing.T, opts ...Option) (*Radio, io.ReadWriteCloser) {
	t.Helper()
	return newTestRadioFrom(t, NewD, opts...)
}

// newTestRadioFrom is newTestRadio for a named constructor.
func newTestRadioFrom(t *testing.T, build ctor, opts ...Option) (*Radio, io.ReadWriteCloser) {
	t.Helper()
	r := build(opts...)
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
// goroutine's Read could swallow a later call's reply. SetReadDeadline cancels
// the same Read call and leaves no goroutine behind.
type deadliner interface {
	SetReadDeadline(time.Time) error
}

// readOneFrame reads until it has accumulated one complete ';'-terminated
// frame, or until timeout elapses with nothing arriving (timedOut true), or the
// connection reports a non-timeout error (e.g. io.EOF after a close).
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

// --- ID: the two identities, and the one place these radios differ ---

// TestID_AnswersTheModelsOwnCATID is this package's flagship per-model
// assertion. The expectations are hand-written from the manual's ID chart
// (layout 1069-1078): "ID" + the four-digit CAT ID + ';', with P1's legend
// printing "0681: FTDX101D" and "0682: FTDX101MP".
//
// The values are what make an FTdx101 driver accept its own simulator: an
// FTDX101D driver opened against a NewMP radio must refuse it as a
// *driver.WrongRadioError naming both models, and vice versa. Neither is the
// FT-710's 0800 or the FTdx10's 0761.
func TestID_AnswersTheModelsOwnCATID(t *testing.T) {
	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			_, conn := newTestRadioFrom(t, m.new)

			want := "ID" + m.catID + ";"
			if len(want) != 7 {
				t.Fatalf("the hand-written literal is %d bytes, want 7 per the ID position chart", len(want))
			}
			if got := exchange(t, conn, "ID;"); got != want {
				t.Errorf("ID; -> %q, want %q", got, want)
			}
		})
	}
}

func TestID_MalformedBodyRejected(t *testing.T) {
	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			_, conn := newTestRadioFrom(t, m.new)
			// ID has no Set direction on either radio (availability X O O X,
			// layout 304), so a body of any kind is not a command — including
			// this radio's own identity echoed back at it.
			assertRejected(t, conn, "ID"+m.catID+";")
			assertRejected(t, conn, "ID0;")
			assertRejected(t, conn, "ID06810;")
		})
	}
}

// TestTheTwoModelsDifferOnlyInTheIDAnswer is the property plan decision D1
// rests on: ONE parameterised implementation, two thin constructors, and a
// difference of exactly four bytes in exactly one command.
//
// It matters beyond tidiness. The capability matrix's §2 values are identical
// for the D and the MP because the memory-channel surface is printed once for
// both (§2.5's sweep), and core/driver/ftdx101 carries ONE read path, ONE write
// path and ONE settings descriptor for the pair on the strength of that. If
// this fake ever grew a second per-model difference, it would be simulating a
// distinction the manual does not document and the driver does not implement.
func TestTheTwoModelsDifferOnlyInTheIDAnswer(t *testing.T) {
	_, connD := newTestRadioFrom(t, NewD, With5xx(), WithEMG(), WithSlot("010", ordinaryState()))
	_, connMP := newTestRadioFrom(t, NewMP, With5xx(), WithEMG(), WithSlot("010", ordinaryState()))

	// Read-shaped frames of every command class this fake answers, plus the
	// refusal paths, plus the case leniency.
	shared := []string{
		"AI;",
		"MC;",
		"MT001;",    // a populated memory channel
		"MT010;",    // the overlaid channel, with a tag and a clarifier
		"MTP1L;",    // a PMS half
		"MT501;",    // a 5 MHz channel
		"MTEMG;",    // the emergency channel
		"MR010;",    // the same channel through the other read command
		"MT050;",    // empty — "?;"
		"MT100;",    // out of inventory — "?;"
		"mt010;",    // lower case, accepted (layout 204-205)
		"MTp1l;",    // lower-case FIELD, refused — register entry 12
		"ZZ;",       // unknown command
		"EX010101;", // not modelled yet — "?;"
	}
	for _, send := range shared {
		gotD := exchange(t, connD, send)
		gotMP := exchange(t, connMP, send)
		if gotD != gotMP {
			t.Errorf("%q: D answered %q, MP answered %q — the two models differ on this wire in the ID answer and nowhere else", send, gotD, gotMP)
		}
	}

	// A Set and its read-back: state changes must be identical too, not merely
	// the read-only answers.
	set := ordinaryChannel("042", mtSetKindFixed).frame()
	writeFrame(t, connD, set)
	assertNoReply(t, connD)
	writeFrame(t, connMP, set)
	assertNoReply(t, connMP)
	if gotD, gotMP := exchange(t, connD, "MT042;"), exchange(t, connMP, "MT042;"); gotD != gotMP {
		t.Errorf("after an identical Set, D answered %q and MP answered %q", gotD, gotMP)
	}

	// And the one difference is present, in the one command.
	gotD, gotMP := exchange(t, connD, "ID;"), exchange(t, connMP, "ID;")
	if gotD == gotMP {
		t.Fatalf("both models answered ID; with %q — the CAT ID is the ONE thing that must differ", gotD)
	}
	if gotD != "ID0681;" || gotMP != "ID0682;" {
		t.Errorf("ID answers = %q (D) and %q (MP), want %q and %q", gotD, gotMP, "ID0681;", "ID0682;")
	}
}

// TestNewRadio_RejectsAMalformedCATID pins the constructor's own guard. Both
// real call sites are constants of this package, so the guard can only fire on
// a programming error — which is exactly why it panics rather than degrading
// into a fake that answers a wrongly sized ID frame nobody would recognise.
func TestNewRadio_RejectsAMalformedCATID(t *testing.T) {
	for _, bad := range []string{"", "068", "06810", "068A", "abcd"} {
		t.Run(bad, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("newRadio(%q) did not panic — a CAT ID that is not four digits must stop the programme", bad)
				}
			}()
			r := newRadio(bad)
			_ = r.Close()
		})
	}
}

// --- AI: accepted, readable, and silent on Set ---

func TestAI_SetIsSilentAndReadReportsIt(t *testing.T) {
	_, conn := newTestRadio(t)

	// OFF at construction, and on these radios that is a MANUAL FACT rather
	// than an assumption: "This parameter is set to '0' (OFF) automatically
	// when the transceiver is turned 'OFF'" (layout 384) — doc.go's "What is
	// NOT in this register".
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

	// Hand-written from the MC chart (layout 1224-1233): "MC" + the 3-byte
	// current channel + ';'. "000" is the DIALECT's ASSUMED none form (its
	// "SlotSpace.NoneWire" entry), which this fake reports until an MC-set.
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; on a fresh radio -> %q, want %q", got, want)
	}
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("the selection reads back as %q, want %q", got, want)
	}
}

// selectionOverTheWire reads the selection back over the WIRE and returns the
// slot the answer names, so a test asserting on the selection asserts on what a
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
	assertRejected(t, conn, "FA014250000;") // a real FTdx101 VFO command this fake does not model
	assertRejected(t, conn, "M;")           // one byte of a command name
	assertRejected(t, conn, ";")            // an empty frame
}

// TestCommandNamesAreAcceptedInEitherCase pins the leniency this manual states
// in terms — "A command consists of 2 alphabetical characters. You may use
// either lower or upper case characters." (layout 204-205) — and pins that it
// stops at the command NAME.
//
// It asserts the opposite of internal/fakedx10's
// TestCommandNamesAreUpperCaseOnly, so a reviewer meeting the two together
// should find this note rather than an unexplained contradiction: THIS test
// rests on the sentence quoted above, in this radio's own manual, and on
// nothing else. Why the sibling requires the opposite of its own fake is a
// question about that package and is recorded for milestone review.
func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)

	if got, want := exchange(t, conn, "id;"), "ID0681;"; got != want {
		t.Errorf("id; -> %q, want %q", got, want)
	}
	lower := exchange(t, conn, "mt001;")
	upper := exchange(t, conn, "MT001;")
	if lower != upper {
		t.Errorf("mt001; -> %q but MT001; -> %q — the command name's case must not matter", lower, upper)
	}
	if lower == "?;" {
		t.Errorf("MT001; -> %q for both cases, so the equality above proves nothing", lower)
	}
	if got, want := exchange(t, conn, "Mt001;"), upper; got != want {
		t.Errorf("Mt001; -> %q, want %q — mixed case is two alphabetical characters like any other", got, want)
	}

	// FIELD values are a different claim, and an ASSUMED one — doc.go register
	// entry 12. The manual's statement is about the command name only.
	assertRejected(t, conn, "MTp1l;")
	if got := exchange(t, conn, "MTP1L;"); got == "?;" {
		t.Error("MTP1L; -> \"?;\" — the lower-case field rejection above proves nothing if the upper-case form is refused too")
	}
}

// TestEXNotModelledYet pins the interim behaviour of the EX (MENU) command:
// this fake holds no menu inventory yet, so every EX read falls through
// handleFrame's default arm to the same "?;" an unknown command draws.
//
// It exists to make the projection's arrival VISIBLE: ex.go replaces this test
// with one asserting that a known address answers and an out-of-inventory one
// is refused, exactly as internal/fakedx10's TestEXIsDispatched replaced its
// own predecessor of this name.
func TestEXNotModelledYet(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"EX010101;", "EX999999;", "EX;"} {
		assertRejected(t, conn, send)
	}
}

func TestFramingSplitsAcrossWritesAndCoalescedFrames(t *testing.T) {
	_, conn := newTestRadio(t)

	// One frame arriving in three writes.
	writeFrame(t, conn, "I")
	writeFrame(t, conn, "D")
	writeFrame(t, conn, ";")
	if got, want := mustReadFrame(t, conn), "ID0681;"; got != want {
		t.Errorf("split ID; -> %q, want %q", got, want)
	}

	// Three frames arriving in one write, answered in order.
	writeFrame(t, conn, "ID;MC;ID;")
	for i, want := range []string{"ID0681;", "MC000;", "ID0681;"} {
		if got := mustReadFrame(t, conn); got != want {
			t.Errorf("coalesced reply %d = %q, want %q", i, got, want)
		}
	}
}

func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)

	// doc.go register entry 15 (this package's own bounded-input policy, not a
	// radio claim): more than maxAccumulatorBytes without a ';' draws one "?;",
	// then bytes are discarded up to and including the next ';'.
	garbage := strings.Repeat("X", maxAccumulatorBytes+1)
	writeFrame(t, conn, garbage)
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow -> %q, want %q", got, want)
	}

	// The tail of the overflowed frame is swallowed, up to its terminator, and
	// framing then resumes: the ID; after it is answered normally, and the
	// swallowed remainder produces no second reply.
	writeFrame(t, conn, "XXXX;ID;")
	if got, want := mustReadFrame(t, conn), "ID0681;"; got != want {
		t.Errorf("after resync, ID; -> %q, want %q (a second \"?;\" would mean the discard window was wrong)", got, want)
	}
}

// --- Close semantics ---

func TestClose_HostSeesEOF(t *testing.T) {
	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			r := m.new()
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
		})
	}
}

func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := NewD(WithLatency(30 * time.Second))
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
