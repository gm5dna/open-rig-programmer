// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// EVERY EXPECTED REPLY IN THIS PACKAGE'S TESTS IS RECOMPUTED INDEPENDENTLY —
// as a literal string built from the matrix's own byte positions — never by
// calling this package's own builders (buildMRAnswer, buildMCAnswer, ...).
// That independence is the whole point of a golden test: it must be possible
// for a builder to have a bug and still be CAUGHT.

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

func assertRejected(t *testing.T, conn io.ReadWriteCloser, send string) {
	t.Helper()
	if got := exchange(t, conn, send); got != "?;" {
		t.Errorf("%q -> %q, want %q", send, got, "?;")
	}
	assertNoReply(t, conn)
}

// mwFrame assembles an MW Set frame from the matrix §2 field positions,
// independently of appendFieldBlock.
func mwFrame(slot, freq8 string, clarSign byte, clarMag4 string, rx, tx bool, mode byte, ctcss byte, tone2 string, shift byte) string {
	var b strings.Builder
	b.WriteString("MW")
	b.WriteString(slot)
	b.WriteString(freq8)
	b.WriteByte(clarSign)
	b.WriteString(clarMag4)
	b.WriteByte(boolFlagByte(rx))
	b.WriteByte(boolFlagByte(tx))
	b.WriteByte(mode)
	b.WriteByte('0') // P7 Set: fixed
	b.WriteByte(ctcss)
	b.WriteString(tone2)
	b.WriteByte(shift)
	b.WriteByte(';')
	return b.String()
}

// mrAnswer assembles the expected MR Answer frame the same way, with P7
// always the answer's kindMemory.
func mrAnswer(slot, freq8 string, clarSign byte, clarMag4 string, rx, tx bool, mode byte, ctcss byte, tone2 string, shift byte) string {
	var b strings.Builder
	b.WriteString("MR")
	b.WriteString(slot)
	b.WriteString(freq8)
	b.WriteByte(clarSign)
	b.WriteString(clarMag4)
	b.WriteByte(boolFlagByte(rx))
	b.WriteByte(boolFlagByte(tx))
	b.WriteByte(mode)
	b.WriteByte('1') // P7 Answer: Memory
	b.WriteByte(ctcss)
	b.WriteString(tone2)
	b.WriteByte(shift)
	b.WriteByte(';')
	return b.String()
}

// --- ID ---

func TestID_DefaultsTo0101AndOptionOverrides(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "ID;"), "ID0101;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}

	_, conn2 := newTestRadio(t, WithCATID(CATID9000MP))
	if got, want := exchange(t, conn2, "ID;"), "ID0103;"; got != want {
		t.Errorf("ID; with WithCATID(0103) -> %q, want %q", got, want)
	}
}

func TestWithCATID_PanicsOnAnUnlistedID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithCATID(\"9999\") did not panic")
		}
	}()
	WithCATID("9999")
}

// --- AI ---

func TestAI_DefaultsOffThenSetAndRead(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; -> %q, want %q", got, want)
	}
	writeFrame(t, conn, "AI1;")
	assertNoReply(t, conn) // fire-and-forget
	if got, want := exchange(t, conn, "AI;"), "AI1;"; got != want {
		t.Errorf("AI; after AI1; -> %q, want %q", got, want)
	}
	assertRejected(t, conn, "AI2;") // outside {0,1}
}

// --- MR / MW round trip ---

func TestMW_ThenMR_RoundTripsAMemoryChannel(t *testing.T) {
	_, conn := newTestRadio(t)
	frame := mwFrame("005", "07123456", '+', "0012", true, false, '3', '1', "05", '1')
	writeFrame(t, conn, frame)
	assertNoReply(t, conn) // MW is fire-and-forget

	want := mrAnswer("005", "07123456", '+', "0012", true, false, '3', '1', "05", '1')
	if got := exchange(t, conn, "MR005;"); got != want {
		t.Errorf("MR005; -> %q, want %q", got, want)
	}
}

func TestMW_ThenMR_RoundTripsAPMSSlot(t *testing.T) {
	_, conn := newTestRadio(t)
	frame := mwFrame("100", "18068000", '-', "0000", false, false, '2', '0', "00", '0')
	writeFrame(t, conn, frame)
	assertNoReply(t, conn)

	// PMS slots answer P7 '1' (register entry PMS SLOTS ANSWER P7 '1'), same
	// as a memory channel.
	want := mrAnswer("100", "18068000", '-', "0000", false, false, '2', '0', "00", '0')
	if got := exchange(t, conn, "MR100;"); got != want {
		t.Errorf("MR100; -> %q, want %q", got, want)
	}
}

func TestMR_EmptySlotIsRejected(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MR050;") // "050" is not in DefaultImage
}

func TestMR_HasNoSetDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	// A 27-byte frame in the MR-Answer shape: this radio's MR has no Set
	// direction at all, so it is simply an unknown frame.
	assertRejected(t, conn, mrAnswer("001", "07000000", '+', "0000", false, false, '1', '0', "00", '0'))
}

func TestMW_HasNoReadOrAnswerDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW001;")
}

func TestMW_RejectsOffVocabularyAndLeavesTheChannelUntouched(t *testing.T) {
	// P7's block offset (blkKind) counted from the start of the frame: 2
	// (command) + blkKind.
	badP7 := []byte(mwFrame("010", "07000000", '+', "0000", false, false, '1', '0', "00", '0'))
	badP7[2+blkKind] = '1' // only '0' is legal on a Set

	cases := []struct {
		name  string
		frame string
	}{
		{"mode outside 1-9,A-C", mwFrame("010", "07000000", '+', "0000", false, false, 'D', '0', "00", '0')},
		{"CTCSS outside 0-2", mwFrame("010", "07000000", '+', "0000", false, false, '1', '3', "00", '0')},
		{"tone index 50, outside 00-49", mwFrame("010", "07000000", '+', "0000", false, false, '1', '0', "50", '0')},
		{"shift outside 0-2", mwFrame("010", "07000000", '+', "0000", false, false, '1', '0', "00", '3')},
		{"P7 not the Set-direction fixed '0'", string(badP7)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			assertRejected(t, conn, tc.frame)
			assertRejected(t, conn, "MR010;") // never written: still empty
		})
	}
}

// --- MC: recall and current-channel ---

func TestMC_RecallsAPopulatedSlotAndReportsIt(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; before any recall -> %q, want %q", got, want)
	}
	writeFrame(t, conn, "MC001;") // populated by DefaultImage
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "MC;"), "MC001;"; got != want {
		t.Errorf("MC; after MC001; -> %q, want %q", got, want)
	}
}

func TestMC_EmptySlotIsRejectedAndDoesNotMoveTheSelection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MC050;")
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; after a rejected recall -> %q, want %q", got, want)
	}
}

func TestMW_DoesNotMoveTheSelectedChannel(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MC001;")
	assertNoReply(t, conn)

	frame := mwFrame("002", "14250000", '+', "0000", false, false, '2', '0', "00", '0')
	writeFrame(t, conn, frame)
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "MC;"), "MC001;"; got != want {
		t.Errorf("MC; after an MW to a different slot -> %q, want %q", got, want)
	}
}

// --- Framing ---

func TestAccumulatorOverflow_RejectsOnceAndResyncs(t *testing.T) {
	_, conn := newTestRadio(t)
	junk := strings.Repeat("X", maxAccumulatorBytes+1)
	writeFrame(t, conn, junk)
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow reply = %q, want %q", got, want)
	}
	// The rest of the overrun, up to and including its terminator, is
	// discarded rather than re-parsed as a frame.
	writeFrame(t, conn, "GARBAGE;")
	assertNoReply(t, conn)

	// Framing resumes normally afterwards.
	if got, want := exchange(t, conn, "AI;"), "AI0;"; got != want {
		t.Errorf("AI; after resync -> %q, want %q", got, want)
	}
}

func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "id;"), "ID0101;"; got != want {
		t.Errorf("id; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, "Ai;"), "AI0;"; got != want {
		t.Errorf("Ai; -> %q, want %q", got, want)
	}
}

// --- Default image and options ---

func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	a := DefaultImage()
	b := DefaultImage()
	a["001"] = MemState{Freq: "99999999"}
	if b["001"].Freq == "99999999" {
		t.Fatal("mutating one DefaultImage() map's result changed another's")
	}
}

func TestWithSlot_OverlaysOntoTheDefaultImage(t *testing.T) {
	custom := MemState{
		Freq: "12345678", Mode: '1', Kind: kindMemory,
		CTCSS: '0', Tone: "00", Shift: '0', ClarSign: '+', ClarMag: "0000",
	}
	r, conn := newTestRadio(t, WithSlot("050", custom))
	want := mrAnswer("050", "12345678", '+', "0000", false, false, '1', '0', "00", '0')
	if got := exchange(t, conn, "MR050;"); got != want {
		t.Errorf("MR050; -> %q, want %q", got, want)
	}
	// The default image's own channel 001 is still present: WithSlot overlays
	// rather than replacing the whole map.
	if _, ok := r.SlotState("001"); !ok {
		t.Error("WithSlot(\"050\", ...) removed DefaultImage's own channel 001")
	}
}
