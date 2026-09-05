// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

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
// fire-and-forget success is verified. Silence on an accepted Set is the
// DIALECT's assumption, cited by name: "THE ACKNOWLEDGEMENT CONVENTIONS"
// (core/cat/ft991a/doc.go's register), which records that this manual "never
// says whether an accepted Set answers at all".
//
// The 150 ms window is inherited verbatim from internal/fakeft891's own
// convention and is this suite's whole -race time budget: every rejection
// this package pins pays it once, so a task adding more assertRejected cases
// (an EX suite, say) adds 150 ms each. Worth knowing before it does; not a
// reason to shrink the window now.
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

// assertRejected writes send and requires EXACTLY the single unattributed NAK
// and nothing after it within the latency window: a passing rejection is not
// merely "the first frame back is '?;'", but that no second frame follows it.
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// --- ID: the CAT identity, and the byte-level difference a probe uses ---

func TestID_AnswersThisRadiosCATID(t *testing.T) {
	_, conn := newTestRadio(t)
	// "P1 0670: FT-991A" (ft991a_layout.txt:772), the seven-byte Answer chart
	// evidence leg G recorded verbatim (mc-vectors.golden's ID section).
	if got, want := exchange(t, conn, "ID;"), "ID0670;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
	// The FT-891's, which a probe against this fake must never see.
	if strings.Contains(exchange(t, conn, "ID;"), "0650") {
		t.Error("this fake answered the FT-891's CAT ID")
	}
}

func TestID_MalformedBodyRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ID0;")
	assertRejected(t, conn, "ID0670;")
}

// --- AI ---

// TestAI_SetIsSilentAndReadReportsIt covers the manual fact that AI is OFF at
// construction (ft991a_layout.txt:242, inside AI's own block at 237-246) and
// the register entry AUTOMATIC-INFORMATION SUPPRESSION: the flag is stored and
// read back, and nothing follows from it.
func TestAI_SetIsSilentAndReadReportsIt(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; at construction -> %q, want %q", got, want)
	}

	writeFrame(t, conn, "AI1;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "AI;"), "AI1;"; got != want {
		t.Errorf("AI; after AI1; -> %q, want %q", got, want)
	}

	// With AI on, an ordinary exchange still produces exactly one frame and
	// the fake volunteers nothing: an AI-on session and an AI-off session are
	// byte-identical on the wire here.
	if got, want := exchange(t, conn, "ID;"), "ID0670;"; got != want {
		t.Errorf("ID; with AI on -> %q, want %q", got, want)
	}
	assertNoReply(t, conn)

	writeFrame(t, conn, "AI0;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; after AI0; -> %q, want %q", got, want)
	}
}

func TestAI_MalformedRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "AI2;")
	assertRejected(t, conn, "AI01;")
}

// --- Framing ---

// TestUnknownCommandRejected covers EX explicitly, because its absence is a
// DECISION of this task rather than an accident: this radio documents EX
// (availability 155, block 519-528) and this fake does not model it yet, so an
// EX frame is an unknown command until the task that adds the inventory.
func TestUnknownCommandRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, frame := range []string{
		"ZZ;",       // no such command in the list at all
		"EX001;",    // documented, deliberately not modelled here
		"EX0011;",   // an EX Set shape, likewise
		"MW001;",    // documented Set-only, deliberately not modelled
		";",         // a bare terminator
		"M;",        // one command byte
		"MTMTMTMT;", // the right two letters, nonsense after them
	} {
		assertRejected(t, conn, frame)
	}
}

// TestCommandNamesAreAcceptedInEitherCase is a MANUAL FACT of this radio: "A
// command consists of 2 alphabetical characters. You may use either lower or
// upper case characters." (ft991a_layout.txt:113-114, under the "Alphabetical
// Commands" heading at 112). Mixed case is admitted as a CONSEQUENCE of
// folding each of the two command bytes independently; the sentence licenses
// neither mixing nor field folding, and FIELD values stay case-sensitive.
func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"id;", "Id;", "iD;", "ID;"} {
		if got, want := exchange(t, conn, send), "ID0670;"; got != want {
			t.Errorf("%q -> %q, want %q", send, got, want)
		}
	}
	for _, send := range []string{"mt001;", "Mt001;"} {
		if got := exchange(t, conn, send); len(got) != 41 {
			t.Errorf("%q -> %q, want a 41-byte combined answer", send, got)
		}
	}
	// A field value is not a command name: the mode nibble's hex letters stay
	// case-sensitive, so a lower-case 'b' is not the legend's "B: FM-N".
	assertRejected(t, conn, ordinaryChannel("050", mtSetKindFixed).with(func(f *combinedFrame) { f.mode = 'b' }).frame())
}

func TestFramingSplitsAcrossWritesAndCoalescedFrames(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "I")
	writeFrame(t, conn, "D")
	writeFrame(t, conn, ";")
	if got, want := mustReadFrame(t, conn), "ID0670;"; got != want {
		t.Errorf("split write -> %q, want %q", got, want)
	}

	writeFrame(t, conn, "ID;AI;")
	if got, want := mustReadFrame(t, conn), "ID0670;"; got != want {
		t.Errorf("first of two coalesced frames -> %q, want %q", got, want)
	}
	if got, want := mustReadFrame(t, conn), "AI0;"; got != want {
		t.Errorf("second of two coalesced frames -> %q, want %q", got, want)
	}
}

// TestAccumulatorOverflowRejectsOnceAndResyncs is the register entry THE FRAME
// ACCUMULATOR'S CAP AND RESYNC — this package's own bounded-input policy, not
// a radio claim.
func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, strings.Repeat("A", maxAccumulatorBytes+1))
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow -> %q, want %q", got, want)
	}
	// The rest of the overrun, up to and including its terminator, is
	// discarded rather than re-parsed as a frame.
	writeFrame(t, conn, "GARBAGE;")
	assertNoReply(t, conn)
	// Normal framing resumes.
	if got, want := exchange(t, conn, "ID;"), "ID0670;"; got != want {
		t.Errorf("after resync -> %q, want %q", got, want)
	}
}

func TestClose_HostSeesEOF(t *testing.T) {
	r := New()
	host := r.Port()
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	buf := make([]byte, 8)
	_, err := host.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("host Read after Close: %v, want io.EOF", err)
	}
}

// TestClose_IsPromptDespiteAPendingLatency pins what WithLatency's
// interruptible wait exists for: internal/wiring's fake-session teardown must
// not have to wait out a scripted delay.
func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(10 * time.Second))
	conn := r.Port()
	writeFrame(t, conn, "ID;")

	done := make(chan struct{})
	go func() {
		_ = r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return promptly while a latency wait was pending")
	}
}

// --- MC: MEMORY CHANNEL (recall) ---

// TestMC_ReadAnswersTheNoneFormUntilARecall. The none form's wire spelling is
// the DIALECT's ASSUMED value, cited not re-derived: its register entry
// `SlotSpace.NoneWire = "000"` records that "000" appears in no FT-991A slot
// legend.
func TestMC_ReadAnswersTheNoneFormUntilARecall(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; before any recall -> %q, want %q", got, want)
	}
}

func selectionOverTheWire(t *testing.T, conn io.ReadWriteCloser) string {
	t.Helper()
	got := exchange(t, conn, "MC;")
	if len(got) != 6 || got[:2] != "MC" || got[5] != ';' {
		t.Fatalf("not a 6-byte MC answer: %q", got)
	}
	return got[2:5]
}

// TestMC_RecallsTheWholeSlotSpace holds MC's own legend: "P1 001 - 117: Memory
// Channel Number" (ft991a_layout.txt:913), the ONE legend in this manual that
// decomposes the span into memory and the nine PMS pairs (916). Evidence leg G
// reduced it to four hand-derived frames — MC012;, MC099;, MC100;, MC117;
// (core/cat/ft991a/testdata/mc-vectors.golden) — and those four are the cases
// below.
func TestMC_RecallsTheWholeSlotSpace(t *testing.T) {
	for _, slot := range []string{"012", "099", "100", "117"} {
		t.Run(slot, func(t *testing.T) {
			r, conn := newTestRadio(t, WithSlot(slot, ordinaryState()))
			writeFrame(t, conn, "MC"+slot+";")
			assertNoReply(t, conn)
			if got := selectionOverTheWire(t, conn); got != slot {
				t.Errorf("selection after MC%s; = %q", slot, got)
			}
			if got := r.CurrentChannel(); got != slot {
				t.Errorf("CurrentChannel() = %q, want %q", got, slot)
			}
		})
	}
}

// TestMC_RecallOfAnEmptySlotIsRejected — the register entry EMPTY-SLOT
// ANSWERS: a channel with no stored data cannot be recalled.
func TestMC_RecallOfAnEmptySlotIsRejected(t *testing.T) {
	r, conn := newTestRadio(t)
	if _, ok := r.SlotState("050"); ok {
		t.Fatal("slot 050 is populated in the default image — this test needs an absent one")
	}
	assertRejected(t, conn, "MC050;")
	if got := r.CurrentChannel(); got != slotNoneWire {
		t.Errorf("a refused recall moved the selection to %q", got)
	}
}

func TestMC_MalformedRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MC118;") // past the printed ceiling
	assertRejected(t, conn, "MC000;") // the answer-only none form
	assertRejected(t, conn, "MCP1L;") // the siblings' PMS spelling
	assertRejected(t, conn, "MC01;")
	assertRejected(t, conn, "MC0011;")
}

// TestSetsDoNotMoveTheSelection is the register entry A SET DOES NOT MOVE THE
// SELECTED CHANNEL: only an MC-set changes what "MC;" answers. The FT-710's
// contrary HARDWARE finding is that radio's, on its own MW, and is not
// borrowed.
func TestSetsDoNotMoveTheSelection(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MC001;")
	assertNoReply(t, conn)

	writeFrame(t, conn, ordinaryChannel("002", mtSetKindFixed).frame())
	assertNoReply(t, conn)

	if got := selectionOverTheWire(t, conn); got != "001" {
		t.Errorf("an MT Set moved the selection to %q, want it left at %q", got, "001")
	}
	if got := r.CurrentChannel(); got != "001" {
		t.Errorf("CurrentChannel() = %q, want %q", got, "001")
	}
}
