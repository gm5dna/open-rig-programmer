// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import (
	"bytes"
	"testing"
)

// Every expectation in this file is BUILT BY HAND from the frame grammar
// (doc.go's "The wire this fake speaks"), never by calling this package's own
// builders. A test that asked buildNAK() what a rejection looks like would
// agree with a mistyped buildNAK().

// nakBytes is the six-byte NG frame, written out: preamble, controller,
// radio, NG code, terminator.
var nakBytes = []byte{0xFE, 0xFE, 0xE0, 0xA4, 0xFA, 0xFD}

// ackBytes is the six-byte OK frame, written out the same way.
var ackBytes = []byte{0xFE, 0xFE, 0xE0, 0xA4, 0xFB, 0xFD}

func TestReassembler_SplitsOnTheTerminator(t *testing.T) {
	acc := newReassembler(64)

	evs := acc.push([]byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD})
	if len(evs) != 1 {
		t.Fatalf("push of one whole frame produced %d events, want 1", len(evs))
	}
	if evs[0].overflow {
		t.Fatalf("a 7-byte frame overflowed a 64-byte accumulator")
	}
	want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(evs[0].frame, want) {
		t.Errorf("frame = % X, want % X", evs[0].frame, want)
	}
}

func TestReassembler_ReassemblesAcrossWrites(t *testing.T) {
	acc := newReassembler(64)

	if evs := acc.push([]byte{0xFE, 0xFE, 0xA4}); len(evs) != 0 {
		t.Fatalf("a partial frame produced %d events, want 0", len(evs))
	}
	if evs := acc.push([]byte{0xE0, 0x19}); len(evs) != 0 {
		t.Fatalf("a still-partial frame produced %d events, want 0", len(evs))
	}
	evs := acc.push([]byte{0x00, 0xFD})
	if len(evs) != 1 {
		t.Fatalf("the completing write produced %d events, want 1", len(evs))
	}
	want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(evs[0].frame, want) {
		t.Errorf("frame = % X, want % X", evs[0].frame, want)
	}
}

func TestReassembler_DeliversEveryFrameInOneWrite(t *testing.T) {
	acc := newReassembler(64)

	evs := acc.push([]byte{
		0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD,
		0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD,
		0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD,
	})
	if len(evs) != 3 {
		t.Fatalf("three concatenated frames produced %d events, want 3", len(evs))
	}
	for i, ev := range evs {
		if ev.overflow {
			t.Errorf("event %d reported overflow", i)
		}
		if len(ev.frame) != 7 {
			t.Errorf("event %d frame is %d bytes, want 7", i, len(ev.frame))
		}
	}
}

// TestReassembler_OverflowsThenResyncsOnTheNextTerminator pins register entry
// 6: the cap is this package's own bounded-input policy, and the recovery is
// "one NG, then discard to the next FD".
func TestReassembler_OverflowsThenResyncsOnTheNextTerminator(t *testing.T) {
	acc := newReassembler(8)

	evs := acc.push(bytes.Repeat([]byte{0x00}, 9))
	if len(evs) != 1 || !evs[0].overflow {
		t.Fatalf("9 unterminated bytes into an 8-byte accumulator gave %+v, want exactly one overflow event", evs)
	}
	if evs[0].frame != nil {
		t.Errorf("an overflow event carried a frame (% X); it must carry none", evs[0].frame)
	}

	// Still discarding: more junk, still no second overflow report.
	if evs := acc.push(bytes.Repeat([]byte{0x00}, 100)); len(evs) != 0 {
		t.Fatalf("junk during the discard produced %d events, want 0 — an overflow reports once", len(evs))
	}

	// The next terminator ends the discard, and the frame after it is whole.
	if evs := acc.push([]byte{0xFD}); len(evs) != 0 {
		t.Fatalf("the resync terminator produced %d events, want 0 — the discarded bytes are not a frame", len(evs))
	}
	evs = acc.push([]byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD})
	if len(evs) != 1 || evs[0].overflow {
		t.Fatalf("after resync the next whole frame gave %+v, want one frame event", evs)
	}
}

// TestParseFrame_TakesTheGrammarApart works over the raw byte runs the
// reassembler hands up.
func TestParseFrame_TakesTheGrammarApart(t *testing.T) {
	tests := []struct {
		name        string
		raw         []byte
		wantOK      bool
		wantTo      byte
		wantFrom    byte
		wantPayload []byte
	}{
		{
			name:        "the plain two-byte preamble",
			raw:         []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD},
			wantOK:      true,
			wantTo:      0xA4,
			wantFrom:    0xE0,
			wantPayload: []byte{0x19, 0x00},
		},
		{
			name:        "one extra FE of padding is tolerated",
			raw:         []byte{0xFE, 0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD},
			wantOK:      true,
			wantTo:      0xA4,
			wantFrom:    0xE0,
			wantPayload: []byte{0x19, 0x00},
		},
		{
			name:        "a long run of padding is tolerated too",
			raw:         []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD},
			wantOK:      true,
			wantTo:      0xA4,
			wantFrom:    0xE0,
			wantPayload: []byte{0x19, 0x00},
		},
		{
			name:        "an empty payload still parses; the FA ladder judges it",
			raw:         []byte{0xFE, 0xFE, 0xA4, 0xE0, 0xFD},
			wantOK:      true,
			wantTo:      0xA4,
			wantFrom:    0xE0,
			wantPayload: []byte{},
		},
		{
			name:   "one FE is not a preamble",
			raw:    []byte{0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD},
			wantOK: false,
		},
		{
			name:   "no preamble at all",
			raw:    []byte{0xA4, 0xE0, 0x19, 0x00, 0xFD},
			wantOK: false,
		},
		{
			name:   "preamble but no addresses",
			raw:    []byte{0xFE, 0xFE, 0xFD},
			wantOK: false,
		},
		{
			name:   "preamble and one address only",
			raw:    []byte{0xFE, 0xFE, 0xA4, 0xFD},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			to, from, payload, ok := parseFrame(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("parseFrame(% X) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if to != tt.wantTo {
				t.Errorf("to = %02X, want %02X", to, tt.wantTo)
			}
			if from != tt.wantFrom {
				t.Errorf("from = %02X, want %02X", from, tt.wantFrom)
			}
			if !bytes.Equal(payload, tt.wantPayload) {
				t.Errorf("payload = % X, want % X", payload, tt.wantPayload)
			}
		})
	}
}

// TestAddressFilter_OnlyFramesAddressedToA4AreAnswered is the address filter's
// own test, at the handler rather than over the wire: a frame addressed
// anywhere else draws SILENCE, not a rejection. A rejection would be this
// radio talking on a conversation that is not its own.
func TestAddressFilter_OnlyFramesAddressedToA4AreAnswered(t *testing.T) {
	tests := []struct {
		name      string
		to        byte
		wantReply []byte
	}{
		{"addressed to this radio", 0xA4, nil}, // 19 00 answers; checked elsewhere
		{"addressed to the controller", 0xE0, nil},
		{"a broadcast", 0x00, nil},
		{"another Icom's default address", 0x94, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			defer r.Close()

			raw := []byte{0xFE, 0xFE, tt.to, 0xE0, 0x19, 0x00, 0xFD}
			reply := r.handleFrame(raw)
			if tt.to == 0xA4 {
				if reply == nil {
					t.Fatal("a frame addressed to A4 drew silence; it must be answered")
				}
				return
			}
			if reply != nil {
				t.Errorf("a frame addressed to %02X drew % X, want silence", tt.to, reply)
			}
		})
	}
}

// TestMalformedFramesDrawSilence: a byte run that is not a CI-V frame carries
// no address, so this radio cannot know the run was meant for it. Register
// entry 5.
func TestMalformedFramesDrawSilence(t *testing.T) {
	runs := [][]byte{
		{0xFD},
		{0xFE, 0xFD},
		{0xFE, 0xFE, 0xFD},
		{0xFE, 0xFE, 0xA4, 0xFD},
		{0x00, 0x01, 0x02, 0xFD},
	}
	for _, raw := range runs {
		r := New()
		if reply := r.handleFrame(raw); reply != nil {
			t.Errorf("% X drew % X, want silence", raw, reply)
		}
		r.Close()
	}
}

// TestTheFARejectionLadder walks every rung the manual-derived rules name.
// Each want is written out by hand.
func TestTheFARejectionLadder(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"a frame carrying no command byte at all", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0xFD}},
		{"a command byte with no sub-command", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0xFD}},
		{"1A with no sub-command", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0xFD}},
		{"an unknown command", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x03, 0x00, 0xFD}},
		{"19 01 is not 19 00", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x01, 0xFD}},
		{"1A 01, the band stacking register, is not a memory record", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x01, 0x00, 0x00, 0xFD}},
		{"1A 05, set mode, is refused", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x05, 0x01, 0x68, 0xFD}},
		{"19 00 must carry no data area", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0x00, 0xFD}},
		{"a 1A 00 data area shorter than the address", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0xFD}},
		{"a 1A 00 with no data area at all", []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, 0xFD}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			defer r.Close()

			reply := r.handleFrame(tt.raw)
			if !bytes.Equal(reply, nakBytes) {
				t.Errorf("% X drew % X, want the NG frame % X", tt.raw, reply, nakBytes)
			}
		})
	}
}

// TestAddressDecoding_RangesAndBCD pins the group/channel rule directly.
func TestAddressDecoding_RangesAndBCD(t *testing.T) {
	tests := []struct {
		name        string
		addr        []byte
		wantOK      bool
		wantGroup   int
		wantChannel int
	}{
		{"memory group 0, channel 0", []byte{0x00, 0x00, 0x00, 0x00}, true, 0, 0},
		{"memory group 99, channel 99", []byte{0x00, 0x99, 0x00, 0x99}, true, 99, 99},
		{"memory group 1, channel 42", []byte{0x00, 0x01, 0x00, 0x42}, true, 1, 42},
		{"the call channel group, first channel", []byte{0x01, 0x00, 0x00, 0x00}, true, 100, 0},
		{"the call channel group, last channel", []byte{0x01, 0x00, 0x00, 0x03}, true, 100, 3},
		{"the call channel group has no channel 4", []byte{0x01, 0x00, 0x00, 0x04}, false, 0, 0},
		{"the call channel group has no channel 99", []byte{0x01, 0x00, 0x00, 0x99}, false, 0, 0},
		{"memory group 100 is not a memory group", []byte{0x01, 0x00, 0x00, 0x00}, true, 100, 0},
		{"group 101 is out of range", []byte{0x01, 0x01, 0x00, 0x00}, false, 0, 0},
		{"group 9999 is out of range", []byte{0x99, 0x99, 0x00, 0x00}, false, 0, 0},
		{"memory channel 100 is out of range", []byte{0x00, 0x00, 0x01, 0x00}, false, 0, 0},
		{"a non-decimal nibble in the group", []byte{0x00, 0x0A, 0x00, 0x00}, false, 0, 0},
		{"a non-decimal nibble in the channel", []byte{0x00, 0x00, 0xF0, 0x00}, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, c, ok := decodeAddress(tt.addr)
			if ok != tt.wantOK {
				t.Fatalf("decodeAddress(% X) ok = %v, want %v", tt.addr, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if g != tt.wantGroup || c != tt.wantChannel {
				t.Errorf("decodeAddress(% X) = (%d, %d), want (%d, %d)", tt.addr, g, c, tt.wantGroup, tt.wantChannel)
			}
		})
	}
}

// TestEncodeAddress_RoundTrips checks the packed-BCD encoder against
// hand-written bytes, and then against its own decoder.
func TestEncodeAddress_RoundTrips(t *testing.T) {
	tests := []struct {
		group, channel int
		want           []byte
	}{
		{0, 0, []byte{0x00, 0x00, 0x00, 0x00}},
		{1, 1, []byte{0x00, 0x01, 0x00, 0x01}},
		{99, 99, []byte{0x00, 0x99, 0x00, 0x99}},
		{100, 3, []byte{0x01, 0x00, 0x00, 0x03}},
		{12, 34, []byte{0x00, 0x12, 0x00, 0x34}},
	}
	for _, tt := range tests {
		got := encodeAddress(tt.group, tt.channel)
		if !bytes.Equal(got, tt.want) {
			t.Errorf("encodeAddress(%d, %d) = % X, want % X", tt.group, tt.channel, got, tt.want)
		}
		g, c, ok := decodeAddress(got)
		if !ok || g != tt.group || c != tt.channel {
			t.Errorf("decodeAddress(encodeAddress(%d, %d)) = (%d, %d, %v)", tt.group, tt.channel, g, c, ok)
		}
	}
}
