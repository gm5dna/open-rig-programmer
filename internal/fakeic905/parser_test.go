// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

import (
	"bytes"
	"testing"
)

// These tests exercise the REASSEMBLER and the FRAME SPLIT directly, below the
// port. Every expectation is written out as a literal here, by hand, from the
// four frame diagrams the task brief quotes from the IC-905 CI-V REFERENCE
// GUIDE, PDF p.3 (folio 2), "◇ About the data format":
//
//	Preamble           FE FE
//	End of message     FD
//	Frame, PC -> radio FE FE AC E0 <cn> [<sc>] [data] FD
//	Frame, radio -> PC FE FE E0 AC <cn> [<sc>] [data] FD
//
// Nothing below calls this package's own builders to compute what it then
// checks: doing so would let one mistake satisfy both sides (see doc.go).

// pushAll feeds chunks to a reassembler in order and returns every event, in
// order, that the whole sequence produced.
func pushAll(a *reassembler, chunks ...[]byte) []accEvent {
	var events []accEvent
	for _, c := range chunks {
		events = append(events, a.push(c)...)
	}
	return events
}

// wantFrames asserts that events are exactly the given complete frames, with no
// overflow event anywhere among them.
func wantFrames(t *testing.T, events []accEvent, want ...[]byte) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, ev := range events {
		if ev.overflow {
			t.Fatalf("event %d is an overflow, want the frame % X", i, want[i])
		}
		if !bytes.Equal(ev.frame, want[i]) {
			t.Errorf("event %d frame = % X, want % X", i, ev.frame, want[i])
		}
	}
}

// TestReassembler_AWellFormedFrameIsOneEvent is the base case: preamble, body,
// end-of-message, nothing else.
func TestReassembler_AWellFormedFrameIsOneEvent(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	got := pushAll(a, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
	wantFrames(t, got, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
}

// TestReassembler_LeadingNoiseBeforeThePreambleIsDiscarded — a real line comes
// up mid-stream, so the bytes before the first FE FE are not part of any frame
// and must not become part of one.
func TestReassembler_LeadingNoiseBeforeThePreambleIsDiscarded(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	got := pushAll(a, []byte{0x00, 0x11, 0x7F, 0xFD, 0x22, 0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
	wantFrames(t, got, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
}

// TestReassembler_ALonePreambleByteIsNoise — one FE is not a preamble. The
// preamble is TWO bytes ("Preamble code (fixed)"), so a single FE followed by
// ordinary bytes starts nothing.
func TestReassembler_ALonePreambleByteIsNoise(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	got := pushAll(a, []byte{0xFE, 0x11, 0x22, 0xFD, 0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
	wantFrames(t, got, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
}

// TestReassembler_ExtraPreamblePaddingIsCollapsedToTwo — some controllers pad
// the preamble. Any run of FE at least two long opens a frame, and the frame
// handed back carries the canonical two.
func TestReassembler_ExtraPreamblePaddingIsCollapsedToTwo(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	got := pushAll(a, []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
	wantFrames(t, got, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
}

// TestReassembler_APreambleMidBodyAbandonsThePartialFrame — FE cannot occur
// inside a body, because it is the preamble; seeing one means the frame in hand
// was truncated and a new one has begun.
func TestReassembler_APreambleMidBodyAbandonsThePartialFrame(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	got := pushAll(a, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
	wantFrames(t, got, []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD})
}

// TestReassembler_TwoFramesInOneChunk — one Read may deliver several frames.
func TestReassembler_TwoFramesInOneChunk(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	got := pushAll(a, []byte{
		0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD,
		0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFD,
	})
	wantFrames(t, got,
		[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD},
		[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFD},
	)
}

// TestReassembler_AFrameSplitAcrossChunksIsStillOneEvent — and one frame may
// take several Reads, byte at a time in the worst case.
func TestReassembler_AFrameSplitAcrossChunksIsStillOneEvent(t *testing.T) {
	frame := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFD}
	a := newReassembler(maxBodyBytes)
	var got []accEvent
	for _, b := range frame {
		got = append(got, a.push([]byte{b})...)
	}
	wantFrames(t, got, frame)
}

// TestReassembler_AnOverLengthFrameIsItsOwnEventAndTheReaderKeepsGoing is the
// brief's "surface overflow as its own event ... without wedging the reader":
// the run that never ends produces ONE overflow event, and the perfectly good
// frame that follows it is still reassembled.
func TestReassembler_AnOverLengthFrameIsItsOwnEventAndTheReaderKeepsGoing(t *testing.T) {
	a := newReassembler(maxBodyBytes)

	over := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A}
	for i := 0; i < maxBodyBytes+50; i++ {
		over = append(over, 0x11)
	}
	over = append(over, 0xFD)

	good := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD}
	got := pushAll(a, over, good)

	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (one overflow, then the good frame): %+v", len(got), got)
	}
	if !got[0].overflow {
		t.Errorf("event 0 = frame % X, want an overflow event", got[0].frame)
	}
	if got[0].frame != nil {
		t.Errorf("the overflow event carries a frame (% X); it must carry none", got[0].frame)
	}
	if got[1].overflow {
		t.Fatal("event 1 is an overflow; the reader is wedged")
	}
	if !bytes.Equal(got[1].frame, good) {
		t.Errorf("event 1 frame = % X, want % X", got[1].frame, good)
	}
}

// TestReassembler_OverflowFiresOnceNotOncePerByte — a very long run of rubbish
// must not produce an event per byte past the cap, or one noisy line would bury
// a consumer in rejections.
func TestReassembler_OverflowFiresOnceNotOncePerByte(t *testing.T) {
	a := newReassembler(maxBodyBytes)
	chunk := []byte{0xFE, 0xFE, 0xAC, 0xE0}
	for i := 0; i < maxBodyBytes*4; i++ {
		chunk = append(chunk, 0x11)
	}
	got := pushAll(a, chunk)
	if len(got) != 1 {
		t.Fatalf("got %d events, want exactly 1 overflow: %+v", len(got), got)
	}
	if !got[0].overflow {
		t.Errorf("event 0 = frame % X, want an overflow event", got[0].frame)
	}
}

// TestReassembler_TheCapIsNotHitByTheLongestFrameThisFakeCanReceive is the
// vacuity guard on the cap: a memory set carrying the printed 64-byte record —
// the longest legitimate frame the wire facts admit — must NOT overflow. A cap
// set below the traffic would make every other test here pass while the fake
// rejected real work.
func TestReassembler_TheCapIsNotHitByTheLongestFrameThisFakeCanReceive(t *testing.T) {
	frame := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x07}
	for i := 0; i < 64; i++ {
		frame = append(frame, 0x11)
	}
	frame = append(frame, 0xFD)

	a := newReassembler(maxBodyBytes)
	wantFrames(t, pushAll(a, frame), frame)
}

// TestParseFrame_SplitsAtFramingBoundariesOnly — parseFrame is allowed to know
// the preamble, the two address bytes and the end-of-message byte, and nothing
// else. It must NOT split the payload into cn / sc / data, because which bytes
// are sub-command and which are data is a per-command fact.
func TestParseFrame_SplitsAtFramingBoundariesOnly(t *testing.T) {
	tests := []struct {
		name        string
		frame       []byte
		wantOK      bool
		wantTo      byte
		wantFrom    byte
		wantPayload []byte
	}{
		{
			name:        "read transceiver ID, PC -> radio",
			frame:       []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD},
			wantOK:      true,
			wantTo:      0xAC,
			wantFrom:    0xE0,
			wantPayload: []byte{0x19, 0x00},
		},
		{
			name:        "OK code, radio -> PC",
			frame:       []byte{0xFE, 0xFE, 0xE0, 0xAC, 0xFB, 0xFD},
			wantOK:      true,
			wantTo:      0xE0,
			wantFrom:    0xAC,
			wantPayload: []byte{0xFB},
		},
		{
			name:        "memory contents read, four address bytes of data",
			frame:       []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFD},
			wantOK:      true,
			wantTo:      0xAC,
			wantFrom:    0xE0,
			wantPayload: []byte{0x1A, 0x00, 0x00, 0x00, 0x00, 0x07},
		},
		{
			name:        "a broadcast frame, to = 00",
			frame:       []byte{0xFE, 0xFE, 0x00, 0xAC, 0x00, 0x11, 0xFD},
			wantOK:      true,
			wantTo:      0x00,
			wantFrom:    0xAC,
			wantPayload: []byte{0x00, 0x11},
		},
		{
			name:   "no command byte at all",
			frame:  []byte{0xFE, 0xFE, 0xAC, 0xE0, 0xFD},
			wantOK: false,
		},
		{
			name:   "no address bytes at all",
			frame:  []byte{0xFE, 0xFE, 0xFD},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseFrame(tt.frame)
			if ok != tt.wantOK {
				t.Fatalf("parseFrame(% X) ok = %v, want %v", tt.frame, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.to != tt.wantTo {
				t.Errorf("to = %#02x, want %#02x", got.to, tt.wantTo)
			}
			if got.from != tt.wantFrom {
				t.Errorf("from = %#02x, want %#02x", got.from, tt.wantFrom)
			}
			if !bytes.Equal(got.payload, tt.wantPayload) {
				t.Errorf("payload = % X, want % X", got.payload, tt.wantPayload)
			}
		})
	}
}

// TestParseFrame_ReportsTheAddressOfATooShortFrameWhenThereIsOne — a frame
// carrying a `to` byte but no command is malformed, and the fake must still be
// able to see WHO it was addressed to, because a malformed frame addressed to
// AC is refused whilst one addressed elsewhere draws no reply at all.
func TestParseFrame_ReportsTheAddressOfATooShortFrameWhenThereIsOne(t *testing.T) {
	to, ok := frameAddressee([]byte{0xFE, 0xFE, 0xAC, 0xFD})
	if !ok {
		t.Fatal("frameAddressee reported no addressee for a frame that carries one")
	}
	if to != 0xAC {
		t.Errorf("to = %#02x, want AC", to)
	}
	if _, ok := frameAddressee([]byte{0xFE, 0xFE, 0xFD}); ok {
		t.Error("frameAddressee reported an addressee for a frame with no address byte")
	}
}

// TestBCD2_MatchesThePrintedValueLists pins the two-byte address encoding
// against the value lists printed in ic905-transcription-b.csv, hand-copied
// here. The page states no encoding (both rows read `unstated`), so this
// mapping is an ASSUMED lift — doc.go register entry 6 — and the printed lists
// are the whole of its evidence:
//
//	①, ②  "00 00 ~ 00 99: Memory channel group | 01 00: Call channel group"
//	③, ④  "00 00 ~ 00 99: 00 ~ 99"
//	       "00 10, 00 11: 10G C1, C2"
func TestBCD2_MatchesThePrintedValueLists(t *testing.T) {
	tests := []struct {
		n    int
		want [2]byte
		why  string
	}{
		{0, [2]byte{0x00, 0x00}, "the first of `00 00 ~ 00 99`"},
		{1, [2]byte{0x00, 0x01}, "`00 01` — the second Call channel of the 144 pair"},
		{9, [2]byte{0x00, 0x09}, "`00 09` — 5600 C2"},
		{10, [2]byte{0x00, 0x10}, "`00 10: 10G C1` — printed 10, not 0x0A"},
		{11, [2]byte{0x00, 0x11}, "`00 11: 10G C2`"},
		{99, [2]byte{0x00, 0x99}, "the last of `00 00 ~ 00 99`"},
		{100, [2]byte{0x01, 0x00}, "`01 00: Call channel group`"},
	}
	for _, tt := range tests {
		if got := bcd2(tt.n); got != tt.want {
			t.Errorf("bcd2(%d) = % X, want % X (%s)", tt.n, got[:], tt.want[:], tt.why)
		}
	}
}

// TestBCD2_PanicsOutsideTheTwoByteRange — an address that cannot be spelt in
// the printed two-byte field is a programming error in the test that asked for
// it, and must stop loudly rather than silently alias another channel.
func TestBCD2_PanicsOutsideTheTwoByteRange(t *testing.T) {
	for _, n := range []int{-1, 10000} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("bcd2(%d) returned normally, want a panic", n)
				}
			}()
			_ = bcd2(n)
		}()
	}
}
