// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// EVERY EXPECTED REPLY IN THIS PACKAGE'S TESTS IS RECOMPUTED INDEPENDENTLY —
// as a literal string, or by a test-local frame assembler — never by calling
// this package's own builders (buildMRAnswer). That independence is the
// whole point: a builder bug must be CAUGHT, which cannot happen if the
// expectation comes from the same buggy function.

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
// rather than a goroutine-based timeout.
type deadliner interface {
	SetReadDeadline(time.Time) error
}

// readOneFrame reads until it has accumulated one complete ';'-terminated
// frame, or until timeout elapses with nothing arriving (timedOut true), or
// the connection reports a non-timeout error.
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
// fire-and-forget success is verified (doc.go's register entry AN ACCEPTED MW
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

// assertRejected writes send and requires EXACTLY the single unattributed
// NAK and nothing after it.
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// --- ID: the identity a probe turns into a wrong-radio refusal
// (ts870s:8986-9009) ---

// TestID_AnswersThePrintedNumber hand-copies Format 16's own value, "The
// TS-870S number is 015." (ts870s:8300-8302), into the six-byte answer frame
// "I D P1 P1 P1 ;" (ts870s:9009).
func TestID_AnswersThePrintedNumber(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "ID;"), "ID015;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
}

// TestID_HasNoSetDirection: the ID block's diagram has no Set content for
// ID at all (ts870s:8991), so anything between "ID" and ';' is unknown.
func TestID_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ID015;")
}

// --- AI: Auto Information (ts870s:8612-8639) ---

// TestAI_ReadsTheInitialOffValue: this book prints no power-on value for AI,
// and this fake's construction-time value is the legend's own first entry,
// "0: AI OFF".
func TestAI_ReadsTheInitialOffValue(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; -> %q, want %q", got, want)
	}
}

// TestAI_SetAndReadRoundTrip drives every one of Format 32's three printed
// values through a Set and reads each one back.
func TestAI_SetAndReadRoundTrip(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, v := range []byte{'0', '1', '2'} {
		assertNoReply2(t, conn, "AI"+string(v)+";")
		if got, want := exchange(t, conn, "AI;"), "AI"+string(v)+";"; got != want {
			t.Errorf("after AI%c;, AI; -> %q, want %q", v, got, want)
		}
	}
}

// assertNoReply2 writes send (an accepted fire-and-forget Set) and confirms
// silence.
func assertNoReply2(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	writeFrame(t, conn, send)
	assertNoReply(t, conn)
}

// TestAI_RefusesAValueOutsideTheThreeValueLegend: Format 32 prints only
// "0", "1" and "2" — no fourth value.
func TestAI_RefusesAValueOutsideTheThreeValueLegend(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "AI3;")
}

// TestAI_RefusesAMalformedWidth.
func TestAI_RefusesAMalformedWidth(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "AI01;")
}

// --- MR: the memory read (ts870s:9067-9106) ---

// recordFrame assembles the 22-byte MR answer this package's own tables
// describe, independently of buildMRAnswer, by placing each field at the
// position ts870s:9112-9113's diagram prints.
func recordFrame(channel int, half byte, freq string, mode, lockout, toneMode byte, toneNo string) string {
	return "MR" + string(half) +
		string(rune('0'+(channel/10)%10)) + string(rune('0'+channel%10)) +
		freq + string(mode) + string(lockout) + string(toneMode) + toneNo + ";"
}

// TestMR_DefaultImage_Channel0AndChannel1 pins DefaultImage's two shipped
// records against a frame built independently of buildMRAnswer.
func TestMR_DefaultImage_Channel0AndChannel1(t *testing.T) {
	_, conn := newTestRadio(t)

	if got, want := exchange(t, conn, "MR000;"), recordFrame(0, '0', "00014230000", '4', '0', '0', "01"); got != want {
		t.Errorf("MR000; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, "MR001;"), recordFrame(1, '0', "00014230000", '2', '1', '1', "39"); got != want {
		t.Errorf("MR001; -> %q, want %q", got, want)
	}
}

// TestMR_UnwrittenChannelAnswersTheZeroRecord: DOCUMENTARY — "For a vacant
// channel, the Answer command sends '0' for all parameters except the
// memory channel number." (ts870s:9101-9104). Channel 50 is populated by
// neither of DefaultImage's two records.
func TestMR_UnwrittenChannelAnswersTheZeroRecord(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "MR050;"), recordFrame(50, '0', zeroFreq, '0', '0', '0', "00"); got != want {
		t.Errorf("MR050; -> %q, want %q", got, want)
	}
}

// TestMR_TXOrEndHalfOfAnUnwrittenChannelAlsoAnswersTheZeroRecord: doc.go's
// register entry AN UNWRITTEN OR VACANT CHANNEL'S EITHER HALF ANSWERS THE
// ZERO RECORD extends the documentary read-side sentence to P1=1 too.
func TestMR_TXOrEndHalfOfAnUnwrittenChannelAlsoAnswersTheZeroRecord(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "MR150;"), recordFrame(50, '1', zeroFreq, '0', '0', '0', "00"); got != want {
		t.Errorf("MR150; -> %q, want %q", got, want)
	}
}

// TestMR_RefusesAP1ByteOtherThanZeroOrOne.
func TestMR_RefusesAP1ByteOtherThanZeroOrOne(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MR250;")
}

// TestMR_RefusesEveryOtherWidth: the Read frame's body is exactly P1 plus
// two channel digits (three bytes) — nothing shorter or longer.
func TestMR_RefusesEveryOtherWidth(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MR05;")   // one channel digit short
	assertRejected(t, conn, "MR0050;") // one channel digit long
}

// TestMR_WithMemoryReadUnsupported plays the error table's second cause
// (ts870s:8434-8438) — a state, not a claim about any TS-870S.
func TestMR_WithMemoryReadUnsupported(t *testing.T) {
	_, conn := newTestRadio(t, WithMemoryReadUnsupported())
	assertRejected(t, conn, "MR004;")
}

// --- MW: the memory write (ts870s:9139-9163) ---

// TestMW_RoundTrip writes a fresh channel and reads it back, independently
// asserting both the fire-and-forget silence on the write and the answer's
// exact bytes on the read.
func TestMW_RoundTrip(t *testing.T) {
	_, conn := newTestRadio(t)
	write := "MW" + "0" + "42" + "00007000000" + "3" + "1" + "1" + "07" + ";"
	assertNoReply2(t, conn, write)

	want := recordFrame(42, '0', "00007000000", '3', '1', '1', "07")
	if got := exchange(t, conn, "MR042;"); got != want {
		t.Errorf("MR042; after write -> %q, want %q", got, want)
	}
}

// TestMW_TheTwoHalvesAreSeparateRecords writes channel 5's RX/Start and
// TX/End halves with two different frequencies and confirms an MR of each
// reads back its own.
func TestMW_TheTwoHalvesAreSeparateRecords(t *testing.T) {
	_, conn := newTestRadio(t)
	assertNoReply2(t, conn, "MW0"+"05"+"00007000000"+"400"+"00;")
	assertNoReply2(t, conn, "MW1"+"05"+"00014000000"+"200"+"00;")

	if got, want := exchange(t, conn, "MR005;"), recordFrame(5, '0', "00007000000", '4', '0', '0', "00"); got != want {
		t.Errorf("MR005; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, "MR105;"), recordFrame(5, '1', "00014000000", '2', '0', '0', "00"); got != want {
		t.Errorf("MR105; -> %q, want %q", got, want)
	}
}

// TestMW_AllZeroFrequencyErasesTheChannel: doc.go's register entry A WRITE
// WITH ALL FREQUENCY DIGITS ZERO MARKS THE CHANNEL VACANT — "The memory
// channel becomes a vacant channel if all frequency digits are '0'. ...
// Other parameters are ignored." (ts870s:9155-9163). A deliberately
// out-of-legend mode nibble ('9') and lockout byte ('9') are supplied to
// prove they really are ignored on this branch, not merely untested.
func TestMW_AllZeroFrequencyErasesTheChannel(t *testing.T) {
	r, conn := newTestRadio(t)

	// Populate channel 9 first, so there is something to erase.
	assertNoReply2(t, conn, "MW0"+"09"+"00007000000"+"400"+"00;")
	if _, ok := r.ChannelState(9, HalfRXOrStart); !ok {
		t.Fatal("channel 9 was not stored by the populating write")
	}

	assertNoReply2(t, conn, "MW0"+"09"+zeroFreq+"991"+"ZZ;") // ignored garbage in every other field
	if _, ok := r.ChannelState(9, HalfRXOrStart); ok {
		t.Error("channel 9 is still stored after an all-zero-frequency write — it should have been erased to vacant")
	}
	if got, want := exchange(t, conn, "MR009;"), recordFrame(9, '0', zeroFreq, '0', '0', '0', "00"); got != want {
		t.Errorf("MR009; after erase -> %q, want %q", got, want)
	}
}

// TestMW_RefusesAnOutOfLegendModeNibbleUnlessErasing: an out-of-digit mode
// byte is refused on an ordinary (non-erasing) write.
func TestMW_RefusesAnOutOfLegendModeNibbleUnlessErasing(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW0"+"09"+"00007000000"+"X00"+"00;")
}

// TestMW_AdmitsBothModeHoles: 0 ("No mode") and 8 ("Tune") are both digits
// Format 2 prints and both are stored rather than refused — doc.go's
// register entry SET-DIRECTION FIELD STRICTNESS ON EVERY OTHER WRITE says
// only non-digit bytes are refused here.
func TestMW_AdmitsBothModeHoles(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, hole := range []byte{'0', '8'} {
		ch := "1" + string(hole)
		write := "MW0" + ch + "00007000000" + string(hole) + "00" + "00;"
		assertNoReply2(t, conn, write)
		want := recordFrame(10+int(hole-'0'), '0', "00007000000", hole, '0', '0', "00")
		if got := exchange(t, conn, "MR0"+ch+";"); got != want {
			t.Errorf("mode hole %q: got %q, want %q", hole, got, want)
		}
	}
}

// TestMW_RefusesALockoutByteOutsideItsTwoValues.
func TestMW_RefusesALockoutByteOutsideItsTwoValues(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW0"+"09"+"00007000000"+"4"+"2"+"0"+"00;")
}

// TestMW_RefusesAToneModeByteOutsideItsTwoValues: this row's tone-mode byte
// is "0: OFF, 1: ON" (Format 1, ts870s:8256) only — no third value, where
// the 480/590 pair print three or four.
func TestMW_RefusesAToneModeByteOutsideItsTwoValues(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW0"+"09"+"00007000000"+"4"+"0"+"2"+"00;")
}

// TestMW_ToneNumberIsStoredNotRangeChecked: Format 14 prints "01~39", but
// this fake stores any two ASCII digits, including ones the printed chart
// does not reach — doc.go's register entry THE TONE NUMBER IS STORED, NOT
// RANGE-CHECKED.
func TestMW_ToneNumberIsStoredNotRangeChecked(t *testing.T) {
	_, conn := newTestRadio(t)
	assertNoReply2(t, conn, "MW0"+"09"+"00007000000"+"400"+"99;")
	want := recordFrame(9, '0', "00007000000", '4', '0', '0', "99")
	if got := exchange(t, conn, "MR009;"); got != want {
		t.Errorf("MR009; -> %q, want %q", got, want)
	}
}

// TestMW_RefusesEveryOtherWidth: the Set frame's body is fixed at 19 bytes
// (P1 + P3 + P4 + P5 + P6 + P7 + P8).
func TestMW_RefusesEveryOtherWidth(t *testing.T) {
	_, conn := newTestRadio(t)
	validBody := "0" + "09" + "00007000000" + "4" + "0" + "0" + "00" // 19 bytes
	if len(validBody) != 19 {
		t.Fatalf("test setup: validBody is %d bytes, want 19", len(validBody))
	}
	assertRejected(t, conn, "MW"+validBody[:len(validBody)-1]+";") // one byte short
	assertRejected(t, conn, "MW"+validBody+"9"+";")                // one byte long
}

// TestMW_RefusesNonDigitFrequency.
func TestMW_RefusesNonDigitFrequency(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW0"+"09"+"0000700000X"+"400"+"00;")
}

// --- Unknown commands, framing and options shared across the dispatch ---

// TestUnknownCommand_Refused: "FV;" appears nowhere in this document (a grep
// of the whole extraction returns nothing), so refusing it is a fact about
// the radio, not a modelling gap.
func TestUnknownCommand_Refused(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "FV;")
}

// TestCommandNamesAreAcceptedInEitherCase: "A command may consist of either
// lower or upper case alphabetical characters." (ts870s:8214).
func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, name := range []string{"id", "ID", "Id", "iD"} {
		if got, want := exchange(t, conn, name+";"), "ID015;"; got != want {
			t.Errorf("%s; -> %q, want %q", name, got, want)
		}
	}
}

// TestAccumulatorOverflowRejectsOnceAndResyncs: over maxAccumulatorBytes
// bytes with no ';' produces exactly one "?;", and framing resumes cleanly
// afterwards — doc.go's register entry THE FRAME ACCUMULATOR'S CAP AND
// RESYNC.
func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)

	garbage := make([]byte, maxAccumulatorBytes+1)
	for i := range garbage {
		garbage[i] = 'Z'
	}
	writeFrame(t, conn, string(garbage))
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow reply = %q, want %q", got, want)
	}
	assertNoReply(t, conn) // exactly one "?;" — nothing further before resync

	// The resync discards up to and including the next ';', then framing
	// resumes: a following well-formed command answers normally.
	writeFrame(t, conn, "garbage-tail;")
	if got, want := exchange(t, conn, "ID;"), "ID015;"; got != want {
		t.Errorf("after resync, ID; -> %q, want %q", got, want)
	}
}

// TestWithTransientNAKSuppressed: the book's own note under "?;" —
// "Occasionally this message may not appear due to microprocessor
// transients in the transceiver." (ts870s:8440-8442).
func TestWithTransientNAKSuppressed(t *testing.T) {
	_, conn := newTestRadio(t, WithTransientNAKSuppressed())
	writeFrame(t, conn, "FV;")
	assertNoReply(t, conn)
}

// TestWithChannel_OverlaysOneHalf.
func TestWithChannel_OverlaysOneHalf(t *testing.T) {
	_, conn := newTestRadio(t, WithChannel(20, MemState{
		Freq: "00021000000", Mode: '3', Lockout: '1', ToneMode: '0', ToneNo: "05",
	}))
	want := recordFrame(20, '0', "00021000000", '3', '1', '0', "05")
	if got := exchange(t, conn, "MR020;"); got != want {
		t.Errorf("MR020; -> %q, want %q", got, want)
	}
}

// TestWithSplitChannel_OverlaysBothHalves: unlike internal/fakets480's row,
// the TX/End half is a published field here (matrix §2, FieldTxFrequency:
// rw), so a direct staging option earns its keep.
func TestWithSplitChannel_OverlaysBothHalves(t *testing.T) {
	_, conn := newTestRadio(t, WithSplitChannel(30,
		MemState{Freq: "00007100000", Mode: '2', Lockout: '0', ToneMode: '0', ToneNo: "00"},
		MemState{Freq: "00007200000", Mode: '2', Lockout: '0', ToneMode: '0', ToneNo: "00"},
	))
	if got, want := exchange(t, conn, "MR030;"), recordFrame(30, '0', "00007100000", '2', '0', '0', "00"); got != want {
		t.Errorf("MR030; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, "MR130;"), recordFrame(30, '1', "00007200000", '2', '0', '0', "00"); got != want {
		t.Errorf("MR130; -> %q, want %q", got, want)
	}
}

// TestWithEmptyChannel_ForcesTheZeroRecord.
func TestWithEmptyChannel_ForcesTheZeroRecord(t *testing.T) {
	_, conn := newTestRadio(t, WithEmptyChannel(0)) // channel 0 is populated by DefaultImage
	want := recordFrame(0, '0', zeroFreq, '0', '0', '0', "00")
	if got := exchange(t, conn, "MR000;"); got != want {
		t.Errorf("MR000; -> %q, want %q", got, want)
	}
}

// TestClose_IsPromptDespiteAPendingLatency proves WithLatency's wait is
// interruptible, the property Close's promptness depends on.
func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(5 * time.Second))
	conn := r.Port()
	writeFrame(t, conn, "ID;") // scheduled, but delayed 5s

	done := make(chan error, 1)
	go func() { done <- r.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: unexpected error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Close did not return promptly despite a pending 5s latency")
	}
}

// TestWithStreamError_ScriptsEAndO scripts both tokens the error table
// prints beside "?;" — "E;", "A communication error occurred such as an
// overrun or framing error during a serial data transmission."
// (ts870s:8445-8447), and "O;", "Receive data was sent but processing was
// not completed." (ts870s:8434-8450 for the whole table). Each replaces an
// otherwise-silent MW's reply outright, at the scripted exchange only —
// doc.go's register entry STREAM ERRORS ARE SCRIPTABLE.
func TestWithStreamError_ScriptsEAndO(t *testing.T) {
	tests := []struct {
		name string
		kind StreamError
		want string
	}{
		{"E — communication error (ts870s:8445-8447)", StreamErrorE, "E;"},
		{"O — processing not completed (ts870s:8449-8450)", StreamErrorO, "O;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t, WithStreamError(tt.kind, 1))

			// Exchange 1 would otherwise be a silent, accepted MW —
			// exercising the "REPLACES a fire-and-forget silence" half of
			// the option's own doc comment, not merely a rejection.
			write := "0" + "09" + "00007000000" + "4" + "0" + "0" + "00"
			writeFrame(t, conn, "MW"+write+";")
			if got := mustReadFrame(t, conn); got != tt.want {
				t.Errorf("scripted exchange 1 -> %q, want %q", got, tt.want)
			}

			// Exchange 2 is unaffected: the script names one exchange only.
			if got, want := exchange(t, conn, "ID;"), "ID015;"; got != want {
				t.Errorf("exchange 2 (ID;) -> %q, want %q — the script must not leak past its own exchange", got, want)
			}
		})
	}
}

// TestWithStreamError_PanicsOnTheZeroKindOrANonPositiveExchange pins the
// same two guards internal/fakets480's own WithStreamError carries: a
// script must name a real token and a real exchange.
func TestWithStreamError_PanicsOnTheZeroKindOrANonPositiveExchange(t *testing.T) {
	mustPanic := func(t *testing.T, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Error("expected a panic, got none")
			}
		}()
		f()
	}
	mustPanic(t, func() { WithStreamError(StreamErrorUnset, 1) })
	mustPanic(t, func() { WithStreamError(StreamErrorE, 0) })
}

// --- The Open sequence core/driver/ts870s now sends: "AI0;" then "ID;" ---

// TestOpenSequence_AI0ThenID confirms this fake answers the exact two-frame
// sequence core/driver/ts870s.Open now sends over a live session
// (reviews/driver-ts870s.md's `## Follow-up`): the family's `Engine.Init`
// preamble "AI0;" (fire-and-forget, Format 32's own first legend value, "0:
// AI OFF"), followed immediately by an "ID;" probe that must answer this
// row's own identity for Open to accept the port as a TS-870S.
func TestOpenSequence_AI0ThenID(t *testing.T) {
	_, conn := newTestRadio(t)

	assertNoReply2(t, conn, "AI0;")
	if got, want := exchange(t, conn, "ID;"), "ID015;"; got != want {
		t.Errorf("ID; after AI0; -> %q, want %q", got, want)
	}
}
