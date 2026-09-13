// SPDX-License-Identifier: GPL-3.0-or-later

package fakets2000

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// EVERY EXPECTED REPLY IN THIS PACKAGE'S TESTS IS RECOMPUTED INDEPENDENTLY —
// as a literal string built by hand from the position chart — never by
// calling this package's own builders (buildMRAnswer, buildMCAnswer). That
// independence is the whole point: a builder bug must be catchable, which
// cannot happen if the expectation comes from the same buggy function.

const testTimeout = 2 * time.Second

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

type deadliner interface {
	SetReadDeadline(time.Time) error
}

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
// fire-and-forget success is verified (doc.go's register entry 1).
func assertNoReply(t *testing.T, r io.Reader) {
	t.Helper()
	frame, _, timedOut := readOneFrame(t, r, 150*time.Millisecond)
	if !timedOut {
		t.Fatalf("expected no reply, got %q", frame)
	}
}

func exchange(t *testing.T, conn io.ReadWriteCloser, send string) string {
	t.Helper()
	writeFrame(t, conn, send)
	return mustReadFrame(t, conn)
}

// assertRejected requires EXACTLY the single unattributed NAK and nothing
// after it within the latency window — the error table prints "?;" as the
// whole of the failure vocabulary (ts2000:9600-9606).
func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// --- Construction and Model() ---

// TestNew_DefaultModelIsTS2000 pins doc.go's register entry 16: with no
// option, New's row is "TS-2000" — the row with a MANUAL-EVIDENCED CATID.
func TestNew_DefaultModelIsTS2000(t *testing.T) {
	r, _ := newTestRadio(t)
	if got := r.Model(); got != "TS-2000" {
		t.Errorf("Model() = %q, want %q", got, "TS-2000")
	}
}

// TestWithModelName_ChangesModelNotWire pins doc.go's register entry 16: the
// three rows share one wire identity. WithModelName changes Model() and
// nothing a probe can observe.
func TestWithModelName_ChangesModelNotWire(t *testing.T) {
	r, conn := newTestRadio(t, WithModelName("TS-B2000"))
	if got := r.Model(); got != "TS-B2000" {
		t.Errorf("Model() = %q, want %q", got, "TS-B2000")
	}
	if got := exchange(t, conn, "ID;"); got != "ID019;" {
		t.Errorf("ID answer = %q, want %q (unchanged by WithModelName)", got, "ID019;")
	}
}

// --- ID ---

func TestID_AnswersSameForAllThreeRows(t *testing.T) {
	for _, model := range []string{"TS-2000", "TS-2000X", "TS-B2000"} {
		t.Run(model, func(t *testing.T) {
			_, conn := newTestRadio(t, WithModelName(model))
			if got := exchange(t, conn, "ID;"); got != "ID019;" {
				t.Errorf("ID answer = %q, want %q", got, "ID019;")
			}
		})
	}
}

func TestID_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ID12345;")
}

// --- TY (Follow-up 2: core/driver/ts2000's Open sends this unconditionally
// after ID, per reviews/registration.md) ---

func TestTY_AnswersTheInventedDefault(t *testing.T) {
	_, conn := newTestRadio(t)
	if got := exchange(t, conn, "TY;"); got != "TY000;" {
		t.Errorf("TY read = %q, want %q", got, "TY000;")
	}
}

// TestTY_AnswersSameForAllThreeRows pins doc.go's register entry 17: the
// manual gives no separate TY answer for TS-2000X/TS-B2000, so all three
// rows answer the TS-2000 one.
func TestTY_AnswersSameForAllThreeRows(t *testing.T) {
	for _, model := range []string{"TS-2000", "TS-2000X", "TS-B2000"} {
		t.Run(model, func(t *testing.T) {
			_, conn := newTestRadio(t, WithModelName(model))
			if got := exchange(t, conn, "TY;"); got != "TY000;" {
				t.Errorf("TY answer = %q, want %q", got, "TY000;")
			}
		})
	}
}

// TestTY_HasNoSetDirection pins the TS-480-shaped erratum: the chart prints
// a "Se t" heading over an empty grid (ts2000:11678-11693), so a TY Set is
// simply unknown, not merely refused-with-a-value.
func TestTY_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "TY001;")
}

// --- MR: unwritten channel ---

// TestMR_UnwrittenChannelAnswersTheInventedEmptyRecord pins doc.go's register
// entry 3: the shape is invented, not read off a manual sentence, but the
// fake answers rather than refusing so the protocol has something to say.
func TestMR_UnwrittenChannelAnswersTheInventedEmptyRecord(t *testing.T) {
	_, conn := newTestRadio(t)
	want := "MR0" + "0" + "99" +
		"00000000000" + "0" + "0" + "0" + "00" + "00" + "000" + "0" + "0" +
		"000000000" + "00" + "0" + "        " + ";"
	got := exchange(t, conn, "MR0099;")
	if got != want {
		t.Errorf("MR of unwritten channel 099 = %q, want %q", got, want)
	}
}

// --- MW / MR round trip ---

// TestMW_ThenMR_RoundTripsAllSixteenFields writes a record exercising every
// one of the five NEW live fields (matrix §2) plus the eleven shared ones,
// and reads it back byte for byte — the cross-check the quarantine exists
// for.
func TestMW_ThenMR_RoundTripsAllSixteenFields(t *testing.T) {
	_, conn := newTestRadio(t)

	set := "MW" + "0" + "1" + "23" +
		"00014195000" + // P4 freq
		"3" + // P5 mode CW
		"1" + // P6 lockout ON
		"3" + // P7 tone mode DCS
		"05" + // P8 tone no
		"12" + // P9 CTCSS no
		"047" + // P10 DCS code (live)
		"1" + // P11 REVERSE (live, no legend — any digit)
		"2" + // P12 Shift '-' (live)
		"000014500" + // P13 offset freq (live)
		"04" + // P14 step
		"7" + // P15 memory group
		"N2000CH " + // P16 name, 8 bytes
		";"
	writeFrame(t, conn, set)
	assertNoReply(t, conn) // doc.go's register entry 1

	want := "MR0" + "1" + "23" +
		"00014195000" + "3" + "1" + "3" + "05" + "12" + "047" + "1" + "2" +
		"000014500" + "04" + "7" + "N2000CH " + ";"
	got := exchange(t, conn, "MR0123;")
	if got != want {
		t.Errorf("MR after MW = %q, want %q", got, want)
	}
}

// TestMW_SectionDefinedChannel_BothHalvesAreSeparateRecords pins channels
// 290-299's P1 overload (ts2000:10726-10727): P1=0 is the start frequency,
// P1=1 the end, and they are two records, not one.
func TestMW_SectionDefinedChannel_BothHalvesAreSeparateRecords(t *testing.T) {
	_, conn := newTestRadio(t)

	writeStart := "MW" + "0" + "2" + "95" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"
	writeEnd := "MW" + "1" + "2" + "95" + "00014000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"

	writeFrame(t, conn, writeStart)
	assertNoReply(t, conn)
	writeFrame(t, conn, writeEnd)
	assertNoReply(t, conn)

	gotStart := exchange(t, conn, "MR0295;")
	if want := "MR0" + "2" + "95" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"; gotStart != want {
		t.Errorf("start half = %q, want %q", gotStart, want)
	}
	gotEnd := exchange(t, conn, "MR1295;")
	if want := "MR1" + "2" + "95" + "00014000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"; gotEnd != want {
		t.Errorf("end half = %q, want %q", gotEnd, want)
	}
}

// --- MW field validation ---

func TestMW_RefusesToneModeOutsideTheFourPrintedValues(t *testing.T) {
	_, conn := newTestRadio(t)
	// Same as a valid write but with ToneMode '4', one past the printed
	// legend "0: OFF, 1: TONE, 2: CTCSS, 3: DCS" (ts2000:10702-10703).
	bad := "MW" + "0" + "0" + "05" + "00007000000" + "1" + "0" + "4" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"
	assertRejected(t, conn, bad)
}

func TestMW_RefusesAnOutOfDomainBankDigit(t *testing.T) {
	_, conn := newTestRadio(t)
	// Bank digit '3': the legend is "0 ~ 2" (ts2000:10592).
	bad := "MW" + "0" + "3" + "05" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"
	assertRejected(t, conn, bad)
}

func TestMW_AcceptsASpaceBankDigitAsBankZero(t *testing.T) {
	r, conn := newTestRadio(t)
	ok := "MW" + "0" + " " + "07" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"
	writeFrame(t, conn, ok)
	assertNoReply(t, conn)

	if _, exists := r.ChannelState(7, HalfRXOrStart); !exists {
		t.Fatal("a space bank digit did not store to channel 7 (bank 0) as doc.go's register entry 4 requires")
	}
}

func TestMW_RefusesUnrecognisedShiftValue(t *testing.T) {
	_, conn := newTestRadio(t)
	// Shift '4': the legend is "0: Simplex / 1: + / 2: - / 3: = (All
	// E-types)" (matrix's own citation of ts2000:10935-10938).
	bad := "MW" + "0" + "0" + "09" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "4" + "000000000" + "00" + "0" + "        " + ";"
	assertRejected(t, conn, bad)
}

// TestMW_StoresAnyReverseDigit pins doc.go's register entry 5: P11 prints no
// legend, so every ASCII digit is admitted rather than a two-value guess.
func TestMW_StoresAnyReverseDigit(t *testing.T) {
	r, conn := newTestRadio(t)
	ok := "MW" + "0" + "0" + "11" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "9" + "0" + "000000000" + "00" + "0" + "        " + ";"
	writeFrame(t, conn, ok)
	assertNoReply(t, conn)

	s, exists := r.ChannelState(11, HalfRXOrStart)
	if !exists {
		t.Fatal("write did not store")
	}
	if s.Reverse != '9' {
		t.Errorf("Reverse = %q, want %q (stored, not range-checked)", s.Reverse, '9')
	}
}

func TestMW_RefusesEveryOtherWidth(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW00099;") // 7 bytes, the MR Read shape
}

// --- MC ---

func TestMC_ReadReportsConstructionDefault(t *testing.T) {
	_, conn := newTestRadio(t)
	if got := exchange(t, conn, "MC;"); got != "MC000;" {
		t.Errorf("MC read at construction = %q, want %q", got, "MC000;")
	}
}

func TestMC_SetMovesTheSelection(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MC205;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != 205 {
		t.Errorf("CurrentChannel() = %d, want 205", got)
	}
	if got := exchange(t, conn, "MC;"); got != "MC205;" {
		t.Errorf("MC read after set = %q, want %q", got, "MC205;")
	}
}

func TestMW_DoesNotMoveTheSelectedChannel(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MC050;")
	assertNoReply(t, conn)

	ok := "MW" + "0" + "1" + "99" + "00007000000" + "1" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "        " + ";"
	writeFrame(t, conn, ok)
	assertNoReply(t, conn)

	if got := r.CurrentChannel(); got != 50 {
		t.Errorf("CurrentChannel() = %d after an MW, want 50 (unchanged) — doc.go's register entry 10", got)
	}
}

// --- AI ---

func TestAI_DefaultsToOff(t *testing.T) {
	_, conn := newTestRadio(t)
	if got := exchange(t, conn, "AI;"); got != "AI0;" {
		t.Errorf("AI read at construction = %q, want %q", got, "AI0;")
	}
}

func TestAI_SetIsFireAndForgetAndSticks(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "AI2;")
	assertNoReply(t, conn)
	if got := exchange(t, conn, "AI;"); got != "AI2;" {
		t.Errorf("AI read after set = %q, want %q", got, "AI2;")
	}
}

func TestAI_RefusesAValueOutsideTheFourPrintedOnes(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "AI4;")
}

// --- Stream errors and transient NAK suppression ---

func TestWithStreamError_ReplacesTheExchange(t *testing.T) {
	_, conn := newTestRadio(t, WithStreamError(StreamErrorO, 1))
	writeFrame(t, conn, "ID;")
	got := mustReadFrame(t, conn)
	if got != "O;" {
		t.Errorf("scripted exchange 1 = %q, want %q", got, "O;")
	}
}

func TestWithTransientNAKSuppressed_DropsTheRejection(t *testing.T) {
	_, conn := newTestRadio(t, WithTransientNAKSuppressed())
	writeFrame(t, conn, "ZZ;") // unknown command, would be "?;"
	assertNoReply(t, conn)
}

func TestWithMemoryReadUnsupported_RefusesMRButNotMC(t *testing.T) {
	_, conn := newTestRadio(t, WithMemoryReadUnsupported())
	assertRejected(t, conn, "MR00099;")
	if got := exchange(t, conn, "MC;"); got != "MC000;" {
		t.Errorf("MC read with WithMemoryReadUnsupported = %q, want %q (untouched)", got, "MC000;")
	}
}

// --- Unknown commands and case folding ---

func TestUnknownCommand_IsRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "ZZ;")
}

func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)
	if got := exchange(t, conn, "id;"); got != "ID019;" {
		t.Errorf("lower-case command = %q, want %q", got, "ID019;")
	}
	if got := exchange(t, conn, "Id;"); got != "ID019;" {
		t.Errorf("mixed-case command = %q, want %q", got, "ID019;")
	}
}

// --- Accumulator overflow ---

func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)
	overflow := strings.Repeat("A", maxAccumulatorBytes+1)
	writeFrame(t, conn, overflow)
	if got := mustReadFrame(t, conn); got != "?;" {
		t.Fatalf("overflow reply = %q, want %q", got, "?;")
	}
	assertNoReply(t, conn)

	writeFrame(t, conn, ";ID;")
	if got := mustReadFrame(t, conn); got != "ID019;" {
		t.Errorf("post-resync frame = %q, want %q", got, "ID019;")
	}
}

// --- Default image ---

func TestDefaultImage_BothChannelsUseOnlyPrintedOrInventedZeroValues(t *testing.T) {
	r, _ := newTestRadio(t)
	for _, ch := range []int{0, 1} {
		s, ok := r.ChannelState(ch, HalfRXOrStart)
		if !ok {
			t.Fatalf("channel %d missing from DefaultImage", ch)
		}
		if s.Freq != exampleFreq {
			t.Errorf("channel %d Freq = %q, want the manual's own example %q", ch, s.Freq, exampleFreq)
		}
		if s.Mode != '1' {
			t.Errorf("channel %d Mode = %q, want '1' (LSB)", ch, s.Mode)
		}
		if s.Name != "        " {
			t.Errorf("channel %d Name = %q, want eight spaces", ch, s.Name)
		}
	}
}
