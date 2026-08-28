// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7610

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"
)

// Every expectation in this file is BUILT BY HAND from the framing skeleton
// printed in the IC-7610 CI-V Reference Guide:
//
//	request  FE FE 98 E0 <cn> [<sc>] <data...> FD
//	answer   FE FE E0 98 <cn> [<sc>] <data...> FD
//	OK       FE FE E0 98 FB FD
//	NG       FE FE E0 98 FA FD
//
// None of it is produced by calling answerFrame, okAnswer or ngAnswer. A test
// that checked the fake against the fake's own builder would pass whatever the
// builder did.

const (
	// replyWait is how long a test waits for an answer it expects. Generous:
	// a slow CI machine failing here should mean "no answer", not "not yet".
	replyWait = 2 * time.Second
	// silenceWait is how long a test waits to be satisfied that NO answer is
	// coming. Short enough to keep the suite quick, long enough that a reply
	// dispatched immediately would have arrived.
	silenceWait = 250 * time.Millisecond
)

// --- hand-built frame construction -----------------------------------------

// request wraps payload in the request-direction frame the guide prints.
func request(payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, 0x98, 0xE0}
	f = append(f, payload...)
	return append(f, 0xFD)
}

// requestTo is request with an arbitrary destination address, for the frames a
// test needs the radio to ignore.
func requestTo(to byte, payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, to, 0xE0}
	f = append(f, payload...)
	return append(f, 0xFD)
}

// answer wraps payload in the answer-direction frame the guide prints.
func answer(payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, 0xE0, 0x98}
	f = append(f, payload...)
	return append(f, 0xFD)
}

// ng and ok are the guide's two fixed codes, framed, typed out here rather than
// taken from the package.
func ng() []byte { return []byte{0xFE, 0xFE, 0xE0, 0x98, 0xFA, 0xFD} }
func ok() []byte { return []byte{0xFE, 0xFE, 0xE0, 0x98, 0xFB, 0xFD} }

// testRecord builds a record of n distinct, recognisable bytes.
//
// Every byte is kept below 0xF0, so none of them is 0xFD or 0xFE. Those two
// would truncate or resynchronise the frame carrying the record — a real
// property of unescaped CI-V framing, documented at doc.go's "Framing", and not
// what any test in this file is about.
func testRecord(seed byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte((int(seed) + i*7) % 0xF0)
	}
	return b
}

// --- radio and port helpers ------------------------------------------------

func newTestRadio(t *testing.T, opts ...Option) *Radio {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// send writes raw bytes to the radio's port.
func send(t *testing.T, r *Radio, b []byte) {
	t.Helper()
	if err := r.Port().SetWriteDeadline(time.Now().Add(replyWait)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if _, err := r.Port().Write(b); err != nil {
		t.Fatalf("writing % X to the port: %v", b, err)
	}
	if err := r.Port().SetWriteDeadline(time.Time{}); err != nil {
		t.Fatalf("clearing write deadline: %v", err)
	}
}

// receive reads one write from the radio, or returns nil if none arrives
// within d.
//
// One Read with a buffer larger than any frame this package sends returns
// exactly one of the radio's writes: net.Pipe hands a reader at most one
// pending write, and consumes it whole when the reader's buffer is big enough.
func receive(t *testing.T, r *Radio, d time.Duration) []byte {
	t.Helper()
	if err := r.Port().SetReadDeadline(time.Now().Add(d)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	defer func() { _ = r.Port().SetReadDeadline(time.Time{}) }()

	buf := make([]byte, 4096)
	n, err := r.Port().Read(buf)
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("reading from the port: %v", err)
	}
	if n == 0 {
		return nil
	}
	return buf[:n]
}

// exchange sends a frame and requires exactly one answer back.
func exchange(t *testing.T, r *Radio, req []byte) []byte {
	t.Helper()
	send(t, r, req)
	got := receive(t, r, replyWait)
	if got == nil {
		t.Fatalf("no answer to % X within %v", req, replyWait)
	}
	return got
}

// requireSilence sends a frame and requires that nothing at all comes back.
func requireSilence(t *testing.T, r *Radio, req []byte) {
	t.Helper()
	send(t, r, req)
	if got := receive(t, r, silenceWait); got != nil {
		t.Fatalf("% X drew an answer % X; it must draw none at all", req, got)
	}
}

// --- the derived record length ---------------------------------------------

// TestRecordLengthIsTheSumOfTheD1FieldWidthsLessTheSelector re-does, in the
// test, the arithmetic RecordLen claims — so that a change to the constant has
// to argue with the numbers rather than just move them.
//
// The widths are the width_bytes column of the D1 rows of
// core/civ/ic7610/testdata/ic7610-transcription-b.csv. The first row, "①, ②
// Memory channel numbers", is the TWO SELECTOR BYTES: that CSV counts them as a
// field of the record, and the record that follows a selector on the wire does
// not contain them.
func TestRecordLengthIsTheSumOfTheD1FieldWidthsLessTheSelector(t *testing.T) {
	widths := []struct {
		field string
		bytes int
	}{
		{"①, ② memory channel numbers (the SELECTOR — excluded below)", 2},
		{"③ select memory setting", 1},
		{"④ ~ ⑧ operating frequency setting", 5},
		{"⑨, ⑩ operating mode setting", 2},
		{"⑪ data mode and tone type settings", 1},
		{"⑫ ~ ⑭ repeater tone frequency setting", 3},
		{"⑮ ~ ⑰ tone squelch frequency setting", 3},
		{"⑱ ~ ㉗ memory name settings", 10},
	}

	total := 0
	for _, w := range widths {
		total += w.bytes
	}
	if total != 27 {
		t.Fatalf("the D1 widths sum to %d, want 27 — the last printed index on that strip is ㉗ = 27, and the geometry witness's own arithmetic reaches the same total", total)
	}

	selector := widths[0].bytes
	if selector != 2 {
		t.Fatalf("the selector is %d bytes, want 2", selector)
	}
	if want := total - selector; RecordLen != want {
		t.Errorf("RecordLen = %d, want %d (27 record bytes less the 2 selector bytes)", RecordLen, want)
	}
	if NameLen != 10 {
		t.Errorf("NameLen = %d, want 10 — the ⑱ ~ ㉗ row's width_bytes, and the page's own \"Up to 10 characters.\"", NameLen)
	}
}

// --- channel selectors ------------------------------------------------------

// TestChannelSelectorsAreThePrintedForms pins the three addressable forms the
// page prints, and pins that nothing else is addressable.
//
// The memory-channel low byte is checked AS PRINTED — channel 99 is 0x99, not
// 0x63 — because the transcription records that the page "prints whole-byte
// codes against meanings and states no numeric encoding". A binary reading
// would make 0x99 unaddressable and channel 99's selector 00 63, neither of
// which the page prints.
func TestChannelSelectorsAreThePrintedForms(t *testing.T) {
	addressable := []struct {
		ch     int
		hi, lo byte
	}{
		{1, 0x00, 0x01},
		{9, 0x00, 0x09},
		{10, 0x00, 0x10},
		{42, 0x00, 0x42},
		{99, 0x00, 0x99},
		{ChanP1, 0x01, 0x00},
		{ChanP2, 0x01, 0x01},
	}
	for _, tc := range addressable {
		hi, lo, ok := selectorFor(tc.ch)
		if !ok || hi != tc.hi || lo != tc.lo {
			t.Errorf("selectorFor(%d) = %02X %02X (ok=%v), want %02X %02X (ok=true)", tc.ch, hi, lo, ok, tc.hi, tc.lo)
		}
		got, ok := channelFor(tc.hi, tc.lo)
		if !ok || got != tc.ch {
			t.Errorf("channelFor(%02X, %02X) = %d (ok=%v), want %d (ok=true)", tc.hi, tc.lo, got, ok, tc.ch)
		}
	}

	unaddressable := []struct {
		name   string
		hi, lo byte
	}{
		{"00 00 — the page's memory range starts at 00 01", 0x00, 0x00},
		{"00 0A — a low nibble above 9 is not a printed code", 0x00, 0x0A},
		{"00 A0 — nor is a high nibble above 9", 0x00, 0xA0},
		{"01 02 — the page prints only P1 (01 00) and P2 (01 01)", 0x01, 0x02},
		{"02 01 — no third selector group is printed", 0x02, 0x01},
		{"FF FF", 0xFF, 0xFF},
	}
	for _, tc := range unaddressable {
		if got, ok := channelFor(tc.hi, tc.lo); ok {
			t.Errorf("channelFor(%02X, %02X) = %d, addressable — %s", tc.hi, tc.lo, got, tc.name)
		}
	}

	for _, ch := range []int{0, -3, 100, 1000} {
		if _, _, ok := selectorFor(ch); ok {
			t.Errorf("selectorFor(%d) reports addressable; only 1..99, ChanP1 and ChanP2 are", ch)
		}
	}
}

// --- framing ----------------------------------------------------------------

// TestFrameNotAddressedToThisRadioIsIgnoredEntirely is the test that makes a
// driver's own address filter provable.
//
// A frame whose `to` is not 0x98 is not for this radio. This fake gives it NO
// ANSWER, changes NO state and logs NO command — exactly as a real bus does,
// where a second radio's traffic simply is not yours. A fake that answered it
// anyway would let a driver with a broken address filter pass every test it
// ever ran.
func TestFrameNotAddressedToThisRadioIsIgnoredEntirely(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0x11, RecordLen)
	// A perfectly well-formed memory SET, addressed to 0x94 instead of 0x98.
	elsewhere := requestTo(0x94, append([]byte{0x1A, 0x00, 0x00, 0x07}, rec...)...)
	requireSilence(t, r, elsewhere)

	if _, set := r.SlotState(7); set {
		t.Error("channel 7 was written by a frame addressed to another radio")
	}
	if log := r.CommandLog(); len(log) != 0 {
		t.Errorf("CommandLog = %v, want empty — a frame addressed elsewhere is not seen by this radio", log)
	}
	// The bytes DID arrive: BytesWritten is the record of the wire, not of
	// what the radio made of it.
	if got := r.BytesWritten(); !bytes.Equal(got, elsewhere) {
		t.Errorf("BytesWritten = % X, want % X", got, elsewhere)
	}

	// And the radio is still alive and still framing: the very next frame,
	// correctly addressed, is answered.
	if got, want := exchange(t, r, request(0x19, 0x00)), answer(0x19, 0x00, 0xA5); !bytes.Equal(got, want) {
		t.Errorf("after ignoring a frame addressed elsewhere, ID answer = % X, want % X", got, want)
	}
}

// TestLeadingNoiseBeforeThePreambleIsSkipped: bytes before the first FE FE are
// line noise. A radio does not answer them and must not lose the frame that
// follows them.
func TestLeadingNoiseBeforeThePreambleIsSkipped(t *testing.T) {
	r := newTestRadio(t)

	noisy := append([]byte{0x00, 0x12, 0xFF, 0x7E, 0xFD, 0x34}, request(0x19, 0x00)...)
	got := exchange(t, r, noisy)

	if want := answer(0x19, 0x00, 0xA5); !bytes.Equal(got, want) {
		t.Errorf("answer after leading noise = % X, want % X", got, want)
	}
	if log := r.CommandLog(); len(log) != 1 || log[0] != [2]byte{0x19, 0x00} {
		t.Errorf("CommandLog = %v, want exactly one 19 00 — the noise must not have been read as a command", log)
	}
}

// TestExtraPreamblePaddingIsTolerated: a run of more than two FE is padding and
// carries no meaning. The guide's own worked example frame is padded.
func TestExtraPreamblePaddingIsTolerated(t *testing.T) {
	r := newTestRadio(t)

	for _, pad := range []int{2, 3, 5, 12} {
		frame := make([]byte, 0, pad+5)
		for i := 0; i < pad; i++ {
			frame = append(frame, 0xFE)
		}
		frame = append(frame, 0x98, 0xE0, 0x19, 0x00, 0xFD)

		got := exchange(t, r, frame)
		if want := answer(0x19, 0x00, 0xA5); !bytes.Equal(got, want) {
			t.Errorf("with %d preamble bytes, answer = % X, want % X", pad, got, want)
		}
	}
}

// TestAFrameSplitAcrossWritesReassembles: framing is a property of the byte
// stream, not of how the host chose to chunk it.
func TestAFrameSplitAcrossWritesReassembles(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0x40, RecordLen)
	whole := request(append([]byte{0x1A, 0x00, 0x00, 0x12}, rec...)...)
	for i := 1; i < len(whole); i++ {
		send(t, r, whole[i-1:i])
	}
	send(t, r, whole[len(whole)-1:])

	got := receive(t, r, replyWait)
	if want := ok(); !bytes.Equal(got, want) {
		t.Fatalf("byte-at-a-time set answered % X, want % X", got, want)
	}
	if m, set := r.SlotState(12); !set || !bytes.Equal(m.Raw, rec) {
		t.Errorf("channel 12 = %v (set=%v), want % X", m.Raw, set, rec)
	}
}

// TestAFrameAddressedHereButCarryingNoCommandAnswersNG. It is addressed to this
// radio and there is nothing in it to act on, so the radio refuses it. Silence
// is reserved for frames that are not this radio's business at all.
func TestAFrameAddressedHereButCarryingNoCommandAnswersNG(t *testing.T) {
	r := newTestRadio(t)
	if got, want := exchange(t, r, request()), ng(); !bytes.Equal(got, want) {
		t.Errorf("empty frame answered % X, want % X", got, want)
	}
}

// --- 19 00, the identity command --------------------------------------------

// TestIDRequestAnswersTheIDToken.
//
// THE COMMAND IS MANUAL-EVIDENCED AND THE REPLY VALUE IS NOT. The guide prints
// 19 00 and prints no reply value for it anywhere, so the default token here is
// INVENTED (0xA5) and this test pins an invention, not a fact. What it really
// proves is the shape — the answer is 19 00 followed by the configured token —
// and that WithIDToken is the way a consumer with a real value supplies one.
func TestIDRequestAnswersTheIDToken(t *testing.T) {
	t.Run("the invented default", func(t *testing.T) {
		r := newTestRadio(t)
		got := exchange(t, r, request(0x19, 0x00))
		if want := answer(0x19, 0x00, 0xA5); !bytes.Equal(got, want) {
			t.Errorf("ID answer = % X, want % X", got, want)
		}
	})

	t.Run("a token supplied by the consumer", func(t *testing.T) {
		tok := []byte{0x12, 0x34, 0x56}
		r := newTestRadio(t, WithIDToken(tok))
		got := exchange(t, r, request(0x19, 0x00))
		if want := answer(0x19, 0x00, 0x12, 0x34, 0x56); !bytes.Equal(got, want) {
			t.Errorf("ID answer = % X, want % X", got, want)
		}
		// The option copied it: mutating the caller's slice afterwards must
		// not reach into the radio.
		tok[0] = 0xEE
		got = exchange(t, r, request(0x19, 0x00))
		if want := answer(0x19, 0x00, 0x12, 0x34, 0x56); !bytes.Equal(got, want) {
			t.Errorf("after mutating the caller's slice, ID answer = % X, want % X", got, want)
		}
	})

	t.Run("19 with another sub-command, and 19 00 carrying data, are refused", func(t *testing.T) {
		r := newTestRadio(t)
		for _, req := range [][]byte{
			request(0x19, 0x01),
			request(0x19),
			// The guide prints this request's Data cell BLANK, so a 19 00
			// with data in it is not the request the guide prints.
			request(0x19, 0x00, 0x00),
		} {
			if got, want := exchange(t, r, req), ng(); !bytes.Equal(got, want) {
				t.Errorf("% X answered % X, want % X", req, got, want)
			}
		}
	})
}

// --- 1A 00, the memory record -----------------------------------------------

// TestMemorySetStoresTheRecordAndAnswersOK.
func TestMemorySetStoresTheRecordAndAnswersOK(t *testing.T) {
	r := newTestRadio(t)

	cases := []struct {
		name   string
		ch     int
		hi, lo byte
	}{
		{"memory channel 1", 1, 0x00, 0x01},
		{"memory channel 42", 42, 0x00, 0x42},
		{"memory channel 99", 99, 0x00, 0x99},
		{"programmed scan edge P1", ChanP1, 0x01, 0x00},
		{"programmed scan edge P2", ChanP2, 0x01, 0x01},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testRecord(tc.hi+tc.lo+1, RecordLen)
			req := request(append([]byte{0x1A, 0x00, tc.hi, tc.lo}, rec...)...)
			if got, want := exchange(t, r, req), ok(); !bytes.Equal(got, want) {
				t.Fatalf("set answered % X, want % X", got, want)
			}
			m, set := r.SlotState(tc.ch)
			if !set {
				t.Fatalf("channel %d not set after a set that answered OK", tc.ch)
			}
			if !bytes.Equal(m.Raw, rec) {
				t.Errorf("stored record = % X, want % X", m.Raw, rec)
			}
		})
	}
}

// TestMemoryReadAnswersTheStoredRecord.
//
// The READ REQUEST FORM ITSELF IS ASSUMED: the document prints no 1A 00 read
// request at all. What is pinned here is that a selector alone reads, that the
// answer repeats the selector, and that the record comes back at RecordLen
// bytes, byte for byte.
func TestMemoryReadAnswersTheStoredRecord(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0x21, RecordLen)
	r.SetSlot(42, MemState{Raw: rec})

	got := exchange(t, r, request(0x1A, 0x00, 0x00, 0x42))
	want := answer(append([]byte{0x1A, 0x00, 0x00, 0x42}, rec...)...)
	if !bytes.Equal(got, want) {
		t.Errorf("read answered % X, want % X", got, want)
	}
	// FE FE E0 98 1A 00 <hi> <lo> ... FD: eight bytes of frame and command
	// before the record, one terminator after it.
	if n := len(got) - 9; n != RecordLen {
		t.Errorf("the answer carries %d record bytes, want %d", n, RecordLen)
	}
}

// TestMemoryReadRoundTripsWhatTheWireWrote: a record set over the wire reads
// back over the wire, unchanged. The two halves of the surface agree.
func TestMemoryReadRoundTripsWhatTheWireWrote(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0x60, RecordLen)
	if got, want := exchange(t, r, request(append([]byte{0x1A, 0x00, 0x01, 0x01}, rec...)...)), ok(); !bytes.Equal(got, want) {
		t.Fatalf("set P2 answered % X, want % X", got, want)
	}
	got := exchange(t, r, request(0x1A, 0x00, 0x01, 0x01))
	if want := answer(append([]byte{0x1A, 0x00, 0x01, 0x01}, rec...)...); !bytes.Equal(got, want) {
		t.Errorf("read P2 answered % X, want % X", got, want)
	}
}

// TestMemoryReadOfAnUnsetChannelAnswersNG.
//
// ASSUMED, and the assumption is WIDER THAN THE CAPTURE BEHIND IT. The document
// prints the NG code but never says an unwritten channel provokes it, and the
// single capture named for this covers ONE unwritten MEMORY channel. The P1 and
// P2 sub-tests below are this fake asserting the same behaviour for the two
// scan edges, which no named capture covers: the memory capture says nothing
// about the scan edges, and the capture that covers P1 says nothing about P2.
// PROVENANCE.md records that as an assumption in its own right.
func TestMemoryReadOfAnUnsetChannelAnswersNG(t *testing.T) {
	cases := []struct {
		name   string
		hi, lo byte
	}{
		{"an unwritten memory channel — the assumption's own scope", 0x00, 0x05},
		{"an unset P1 — WIDER than any named capture", 0x01, 0x00},
		{"an unset P2 — WIDER still; the P1 capture says nothing about P2", 0x01, 0x01},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRadio(t)
			got := exchange(t, r, request(0x1A, 0x00, tc.hi, tc.lo))
			if want := ng(); !bytes.Equal(got, want) {
				t.Errorf("read of an unset channel answered % X, want % X", got, want)
			}
		})
	}
}

// TestClearSlotMakesAChannelUnsetAgain. ClearSlot is the Go-side control the
// wire deliberately does not offer.
func TestClearSlotMakesAChannelUnsetAgain(t *testing.T) {
	r := newTestRadio(t)

	r.SetSlot(3, MemState{Raw: testRecord(0x01, RecordLen)})
	if _, set := r.SlotState(3); !set {
		t.Fatal("SetSlot did not set channel 3")
	}
	r.ClearSlot(3)
	if _, set := r.SlotState(3); set {
		t.Error("ClearSlot did not unset channel 3")
	}
	if got, want := exchange(t, r, request(0x1A, 0x00, 0x00, 0x03)), ng(); !bytes.Equal(got, want) {
		t.Errorf("read of a cleared channel answered % X, want % X", got, want)
	}
}

// TestMemorySetWithAWrongLengthRecordAnswersNG. One record has ONE accepted
// length; anything else is refused and nothing is stored.
func TestMemorySetWithAWrongLengthRecordAnswersNG(t *testing.T) {
	lengths := []int{2, RecordLen - 1, RecordLen + 1, 2 * RecordLen}
	for _, n := range lengths {
		r := newTestRadio(t)
		rec := testRecord(0x30, n)
		req := request(append([]byte{0x1A, 0x00, 0x00, 0x08}, rec...)...)
		if got, want := exchange(t, r, req), ng(); !bytes.Equal(got, want) {
			t.Errorf("a %d-byte record answered % X, want % X", n, got, want)
		}
		if _, set := r.SlotState(8); set {
			t.Errorf("a %d-byte record was stored anyway", n)
		}
		_ = r.Close()
	}
}

// TestUnaddressableSelectorAnswersNG. The page prints three addressable forms;
// this radio addresses nothing else.
func TestUnaddressableSelectorAnswersNG(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0x50, RecordLen)
	cases := []struct {
		name   string
		hi, lo byte
	}{
		{"00 00 — the memory range starts at 00 01", 0x00, 0x00},
		{"00 0A — a nibble above 9 is not a printed code", 0x00, 0x0A},
		{"00 9A", 0x00, 0x9A},
		{"01 02 — only P1 (01 00) and P2 (01 01) are printed", 0x01, 0x02},
		{"03 00 — no third selector group is printed", 0x03, 0x00},
	}
	for _, tc := range cases {
		t.Run(tc.name+" (read)", func(t *testing.T) {
			if got, want := exchange(t, r, request(0x1A, 0x00, tc.hi, tc.lo)), ng(); !bytes.Equal(got, want) {
				t.Errorf("answered % X, want % X", got, want)
			}
		})
		t.Run(tc.name+" (set)", func(t *testing.T) {
			req := request(append([]byte{0x1A, 0x00, tc.hi, tc.lo}, rec...)...)
			if got, want := exchange(t, r, req), ng(); !bytes.Equal(got, want) {
				t.Errorf("answered % X, want % X", got, want)
			}
		})
	}

	t.Run("a 1A 00 with a truncated selector", func(t *testing.T) {
		for _, req := range [][]byte{request(0x1A, 0x00), request(0x1A, 0x00, 0x00)} {
			if got, want := exchange(t, r, req), ng(); !bytes.Equal(got, want) {
				t.Errorf("% X answered % X, want % X", req, got, want)
			}
		}
	})
}

// --- the five refused forms --------------------------------------------------

// TestTheFiveRefusedFormsAnswerNG.
//
// Three of these are DELIBERATE DIVERGENCES from the page — the two clear forms
// and the power-ON command — and a real IC-7610 would very likely honour all
// three. They are refused so that any code path which ever emits one fails
// loudly in a test rather than silently clearing a channel or asserting a power
// state a fake does not have. The remaining two are 1A 05, the menu surface
// this tier does not ship; refusing that is not a divergence, it is what the
// tier means.
//
// The final assertion is the one that matters for the clear forms: after a
// refusal, THE CHANNEL IS STILL THERE.
func TestTheFiveRefusedFormsAnswerNG(t *testing.T) {
	rec := testRecord(0x70, RecordLen)

	forms := []struct {
		name    string
		payload []byte
		cn, sc  byte
	}{
		{
			name:    "1A 00 <ch> FF — the clear form the page prints (DIVERGENCE: refused, not honoured)",
			payload: []byte{0x1A, 0x00, 0x00, 0x04, 0xFF},
			cn:      0x1A, sc: 0x00,
		},
		{
			name:    "0B — \"Memory clear\" (DIVERGENCE: refused, not honoured)",
			payload: []byte{0x0B},
			cn:      0x0B, sc: 0x00,
		},
		{
			name:    "1A 05 as a read — the menu surface this tier does not ship",
			payload: []byte{0x1A, 0x05, 0x01, 0x33},
			cn:      0x1A, sc: 0x05,
		},
		{
			name:    "1A 05 as a set — likewise",
			payload: []byte{0x1A, 0x05, 0x01, 0x33, 0x01},
			cn:      0x1A, sc: 0x05,
		},
		{
			name:    "18 01 — power ON (DIVERGENCE: refused; a fake has no power state)",
			payload: []byte{0x18, 0x01},
			cn:      0x18, sc: 0x01,
		},
	}

	for _, f := range forms {
		t.Run(f.name, func(t *testing.T) {
			r := newTestRadio(t)
			r.SetSlot(4, MemState{Raw: rec})

			got := exchange(t, r, request(f.payload...))
			if want := ng(); !bytes.Equal(got, want) {
				t.Fatalf("answered % X, want % X", got, want)
			}

			// Refused, but SEEN: a consumer proving it never sends a clear
			// needs the log to show the absence, so a refusal must be logged.
			log := r.CommandLog()
			if len(log) != 1 || log[0] != [2]byte{f.cn, f.sc} {
				t.Errorf("CommandLog = %v, want exactly one %02X %02X", log, f.cn, f.sc)
			}

			m, set := r.SlotState(4)
			if !set || !bytes.Equal(m.Raw, rec) {
				t.Errorf("channel 4 = %v (set=%v) after the refusal; it must be untouched — % X", m.Raw, set, rec)
			}
		})
	}
}

// --- the reassembler, directly ------------------------------------------------

// TestReassemblerDropsAnOverlongAccumulation: a stream that never frames is
// bounded, and the drop is silent — CI-V has no code for "I could not find a
// frame".
func TestReassemblerDropsAnOverlongAccumulation(t *testing.T) {
	a := newReassembler(64)

	junk := make([]byte, 200)
	for i := range junk {
		junk[i] = 0x11
	}
	if got := a.push(junk); len(got) != 0 {
		t.Fatalf("junk produced %d frames, want 0", len(got))
	}
	if len(a.buf) > 64 {
		t.Errorf("accumulator holds %d bytes, want at most 64", len(a.buf))
	}

	// And it still frames afterwards.
	frames := a.push(request(0x19, 0x00))
	if len(frames) != 1 {
		t.Fatalf("after the drop, got %d frames, want 1", len(frames))
	}
	if frames[0].to != 0x98 || frames[0].from != 0xE0 {
		t.Errorf("frame addresses = %02X %02X, want 98 E0", frames[0].to, frames[0].from)
	}
	if !bytes.Equal(frames[0].data, []byte{0x19, 0x00}) {
		t.Errorf("frame data = % X, want 19 00", frames[0].data)
	}
}

// TestReassemblerReturnsEveryFrameInOneWrite: several frames arriving in one
// read are all delivered, in order.
func TestReassemblerReturnsEveryFrameInOneWrite(t *testing.T) {
	a := newReassembler(maxAccumulatorBytes)

	stream := append(request(0x19, 0x00), request(0x1A, 0x00, 0x00, 0x01)...)
	stream = append(stream, requestTo(0x94, 0x0B)...)

	frames := a.push(stream)
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want 3", len(frames))
	}
	if !bytes.Equal(frames[0].data, []byte{0x19, 0x00}) {
		t.Errorf("frame 0 data = % X, want 19 00", frames[0].data)
	}
	if !bytes.Equal(frames[1].data, []byte{0x1A, 0x00, 0x00, 0x01}) {
		t.Errorf("frame 1 data = % X, want 1A 00 00 01", frames[1].data)
	}
	if frames[2].to != 0x94 {
		t.Errorf("frame 2 to = %02X, want 94 — the reassembler frames it; handleFrame is what ignores it", frames[2].to)
	}
}
