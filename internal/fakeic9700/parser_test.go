// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"bytes"
	"testing"
)

// The frames in this file are written out byte by byte from the wire facts the
// brief quotes, not built with the helpers under test, so that a helper cannot
// prove itself right by agreeing with itself.

func TestBuildFrame_PreambleAddressesAndEnd(t *testing.T) {
	got := buildFrame(controllerAddress, radioAddress, 0x1A, 0x00, 0x01)
	want := []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x1A, 0x00, 0x01, 0xFD}
	if !bytes.Equal(got, want) {
		t.Errorf("buildFrame = % 02X, want % 02X", got, want)
	}
}

// TestOKAndNGFrames pins the two codes exactly as the brief quotes them —
// "FE FE E0 A2 FB FD" and "FE FE E0 A2 FA FD".
func TestOKAndNGFrames(t *testing.T) {
	if got, want := okFrame(controllerAddress), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0xFB, 0xFD}; !bytes.Equal(got, want) {
		t.Errorf("okFrame = % 02X, want % 02X", got, want)
	}
	if got, want := ngFrame(controllerAddress), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0xFA, 0xFD}; !bytes.Equal(got, want) {
		t.Errorf("ngFrame = % 02X, want % 02X", got, want)
	}
}

func TestParseFrame(t *testing.T) {
	f, ok := parseFrame([]byte{0xA2, 0xE0, 0x19, 0x00})
	if !ok {
		t.Fatal("parseFrame refused a well-formed body")
	}
	if f.to != 0xA2 || f.from != 0xE0 {
		t.Errorf("to/from = %02X/%02X, want A2/E0", f.to, f.from)
	}
	if want := []byte{0x19, 0x00}; !bytes.Equal(f.data, want) {
		t.Errorf("data = % 02X, want % 02X", f.data, want)
	}
}

func TestParseFrame_TooShort(t *testing.T) {
	for _, body := range [][]byte{nil, {0xA2}, {0xA2, 0xE0}} {
		if _, ok := parseFrame(body); ok {
			t.Errorf("parseFrame(% 02X) accepted a body with no command byte", body)
		}
	}
}

func TestAccumulator(t *testing.T) {
	tests := []struct {
		name string
		feed []byte
		want [][]byte
	}{
		{
			name: "one plain frame",
			feed: []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD},
			want: [][]byte{{0xA2, 0xE0, 0x19, 0x00}},
		},
		{
			name: "leading noise before the preamble is discarded",
			feed: []byte{0x11, 0x22, 0x33, 0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD},
			want: [][]byte{{0xA2, 0xE0, 0x19, 0x00}},
		},
		{
			name: "a single FE is not a preamble, so what follows is noise",
			feed: []byte{0xFE, 0x11, 0x22, 0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD},
			want: [][]byte{{0xA2, 0xE0, 0x19, 0x00}},
		},
		{
			name: "a stray end-of-message byte outside a frame is discarded",
			feed: []byte{0xFD, 0xFD, 0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD},
			want: [][]byte{{0xA2, 0xE0, 0x19, 0x00}},
		},
		{
			name: "two frames back to back",
			feed: []byte{
				0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD,
				0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x07, 0xFD,
			},
			want: [][]byte{
				{0xA2, 0xE0, 0x19, 0x00},
				{0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x07},
			},
		},
		{
			name: "an incomplete frame yields nothing yet",
			feed: []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x19},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newAccumulator().feed(tt.feed)
			assertBodies(t, got, tt.want)
		})
	}
}

// TestAccumulator_RepeatedPreambleBytes covers the brief's "A frame may be
// preceded by repeated FE bytes (up to 119 at the lowest rate); a receiver
// normalises them." 119 is the printed worst case, so it is the case tested.
func TestAccumulator_RepeatedPreambleBytes(t *testing.T) {
	for _, n := range []int{2, 3, 10, 119, 200} {
		var feed []byte
		for i := 0; i < n; i++ {
			feed = append(feed, 0xFE)
		}
		feed = append(feed, 0xA2, 0xE0, 0x19, 0x00, 0xFD)

		got := newAccumulator().feed(feed)
		assertBodies(t, got, [][]byte{{0xA2, 0xE0, 0x19, 0x00}})
	}
}

// TestAccumulator_SplitAcrossFeeds proves the accumulator is a stream reader
// and not a whole-buffer parser: the fake reads from a pipe that may hand it
// any split at all, including one that lands between the two preamble bytes.
func TestAccumulator_SplitAcrossFeeds(t *testing.T) {
	whole := []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x07, 0xFD}
	for split := 0; split <= len(whole); split++ {
		acc := newAccumulator()
		got := acc.feed(whole[:split])
		got = append(got, acc.feed(whole[split:])...)
		assertBodies(t, got, [][]byte{{0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x07}})
	}
}

// TestAccumulator_PreambleInsideABodyResyncs pins the consequence of the wire
// fact that FE may not appear inside data: a body carrying one is not data, it
// is a lost frame followed by the start of the next one.
func TestAccumulator_PreambleInsideABodyResyncs(t *testing.T) {
	feed := []byte{
		0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, // truncated: no FD
		0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD,
	}
	assertBodies(t, newAccumulator().feed(feed), [][]byte{{0xA2, 0xE0, 0x19, 0x00}})
}

func TestAccumulator_OverlongBodyIsAbandoned(t *testing.T) {
	feed := []byte{0xFE, 0xFE, 0xA2, 0xE0}
	for i := 0; i < maxFrameBody+10; i++ {
		feed = append(feed, 0x11)
	}
	feed = append(feed, 0xFD)
	feed = append(feed, 0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD)

	assertBodies(t, newAccumulator().feed(feed), [][]byte{{0xA2, 0xE0, 0x19, 0x00}})
}

// TestCanonicalFrame proves the normalisation the transcript relies on: however
// many preamble bytes arrived, the recorded frame carries exactly two.
func TestCanonicalFrame(t *testing.T) {
	got := canonicalFrame([]byte{0xA2, 0xE0, 0x19, 0x00})
	want := []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(got, want) {
		t.Errorf("canonicalFrame = % 02X, want % 02X", got, want)
	}
}

func assertBodies(t *testing.T, got, want [][]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d frames %v, want %d %v", len(got), hexAll(got), len(want), hexAll(want))
	}
	for i := range got {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("frame %d = % 02X, want % 02X", i, got[i], want[i])
		}
	}
}

func hexAll(bs [][]byte) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = hexBytes(b)
	}
	return out
}
