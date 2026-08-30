// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"bytes"
	"testing"
)

// The wire layer's tests. Every expectation here is written out BY HAND from
// the printed frame diagrams named in PROVENANCE.md — never by calling the
// package's own builders to work out what the answer should be, which would
// only prove that buildFrame agrees with itself.

func TestBuildFrame_IsThePrintedControllerToRadioForm(t *testing.T) {
	// PDF p.361 (folio 20-2), "Controller to IC-7100": FE FE, 88, E0, Cn, Sc,
	// data area, FD. This is the read-transceiver-ID frame, whose data area is
	// empty because the command table's Data column for 19 00 is blank
	// (PDF p.364, folio 20-5).
	got := buildFrame(radioAddress, controllerAddress, cmdTransceiverID, subTransceiverID)
	want := []byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(got, want) {
		t.Errorf("buildFrame(19 00) = %s, want %s", hexBytes(got), hexBytes(want))
	}
}

func TestOKAndNGFrames_AreThePrintedAcknowledgements(t *testing.T) {
	// PDF p.361 (folio 20-2), right-hand pair: "OK message to controller"
	// FE FE E0 88 FB FD with FB labelled "OK code (fixed)", and "NG message to
	// controller" FE FE E0 88 FA FD with FA labelled "NG code (fixed)".
	if got, want := okFrame(controllerAddress, radioAddress), []byte{0xFE, 0xFE, 0xE0, 0x88, 0xFB, 0xFD}; !bytes.Equal(got, want) {
		t.Errorf("okFrame = %s, want %s", hexBytes(got), hexBytes(want))
	}
	if got, want := ngFrame(controllerAddress, radioAddress), []byte{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}; !bytes.Equal(got, want) {
		t.Errorf("ngFrame = %s, want %s", hexBytes(got), hexBytes(want))
	}
}

func TestParseFrame(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		wantOK   bool
		wantTo   byte
		wantFrom byte
		wantData []byte
	}{
		{
			name:     "the printed 1A 00 read request's body",
			body:     []byte{0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01},
			wantOK:   true,
			wantTo:   0x88,
			wantFrom: 0xE0,
			wantData: []byte{0x1A, 0x00, 0x01, 0x00, 0x01},
		},
		{
			name:     "a command with no sub-command and no data",
			body:     []byte{0x88, 0xE0, 0x0B},
			wantOK:   true,
			wantTo:   0x88,
			wantFrom: 0xE0,
			wantData: []byte{0x0B},
		},
		{name: "two address bytes and no command byte is not a frame", body: []byte{0x88, 0xE0}},
		{name: "one byte is not a frame", body: []byte{0x88}},
		{name: "nothing is not a frame", body: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ok := parseFrame(tt.body)
			if ok != tt.wantOK {
				t.Fatalf("parseFrame(%s) ok = %v, want %v", hexBytes(tt.body), ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if f.to != tt.wantTo || f.from != tt.wantFrom {
				t.Errorf("parseFrame(%s) addresses = %02X/%02X, want %02X/%02X", hexBytes(tt.body), f.to, f.from, tt.wantTo, tt.wantFrom)
			}
			if !bytes.Equal(f.data, tt.wantData) {
				t.Errorf("parseFrame(%s) data = %s, want %s", hexBytes(tt.body), hexBytes(f.data), hexBytes(tt.wantData))
			}
		})
	}
}

func TestAccumulator(t *testing.T) {
	tests := []struct {
		name  string
		feeds [][]byte
		want  [][]byte
	}{
		{
			name:  "one whole frame in one write",
			feeds: [][]byte{{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD}},
			want:  [][]byte{{0x88, 0xE0, 0x19, 0x00}},
		},
		{
			name: "one frame chopped one byte at a time",
			feeds: [][]byte{
				{0xFE}, {0xFE}, {0x88}, {0xE0}, {0x19}, {0x00}, {0xFD},
			},
			want: [][]byte{{0x88, 0xE0, 0x19, 0x00}},
		},
		{
			name:  "two frames in one write",
			feeds: [][]byte{{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD, 0xFE, 0xFE, 0x88, 0xE0, 0x0B, 0xFD}},
			want:  [][]byte{{0x88, 0xE0, 0x19, 0x00}, {0x88, 0xE0, 0x0B}},
		},
		{
			name: "the manual's own power-ON example: seven extra preamble bytes at 4800 bps",
			// PDF p.363 (folio 20-4), footnote *2 and the worked example:
			// FE FE FE FE FE FE FE FE FE 88 E0 18 01 FD — seven FE bytes ahead
			// of the basic format's own two.
			feeds: [][]byte{{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x88, 0xE0, 0x18, 0x01, 0xFD}},
			want:  [][]byte{{0x88, 0xE0, 0x18, 0x01}},
		},
		{
			name:  "noise before the preamble pair is dropped",
			feeds: [][]byte{{0x11, 0x22, 0x33, 0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD}},
			want:  [][]byte{{0x88, 0xE0, 0x19, 0x00}},
		},
		{
			name:  "an end-of-message byte outside a frame is dropped",
			feeds: [][]byte{{0xFD, 0xFD, 0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD}},
			want:  [][]byte{{0x88, 0xE0, 0x19, 0x00}},
		},
		{
			name: "a preamble byte inside a body abandons that body and starts the next frame",
			// A preamble byte cannot appear inside a frame's data, so one that
			// does appear there means the frame in hand was lost on the line.
			feeds: [][]byte{{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0xFE, 0xFE, 0x88, 0xE0, 0x0B, 0xFD}},
			want:  [][]byte{{0x88, 0xE0, 0x0B}},
		},
		{
			name:  "a lone preamble byte introduces nothing",
			feeds: [][]byte{{0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD}},
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAccumulator()
			var got [][]byte
			for _, f := range tt.feeds {
				got = append(got, a.feed(f)...)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d frames %v, want %d %v", len(got), hexFrames(got), len(tt.want), hexFrames(tt.want))
			}
			for i := range got {
				if !bytes.Equal(got[i], tt.want[i]) {
					t.Errorf("frame %d = %s, want %s", i, hexBytes(got[i]), hexBytes(tt.want[i]))
				}
			}
		})
	}
}

func TestAccumulator_DropsARunTooLongToBeAFrame(t *testing.T) {
	// The document states no frame-length limit. The cap is a property of a
	// reader that must not grow without bound on a line that has come up
	// mid-frame; see doc.go, register entry 14.
	a := newAccumulator()
	a.feed([]byte{0xFE, 0xFE})
	if got := a.feed(bytes.Repeat([]byte{0x00}, maxFrameBody+16)); len(got) != 0 {
		t.Fatalf("an over-long run completed %d frames, want 0", len(got))
	}
	// And the reader resynchronises on the next real frame rather than staying
	// wedged.
	got := a.feed([]byte{0xFD, 0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD})
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0x88, 0xE0, 0x19, 0x00}) {
		t.Fatalf("after the over-long run the reader gave %v, want one frame 88 E0 19 00", hexFrames(got))
	}
}

func TestCanonicalFrame_NormalisesThePreambleRun(t *testing.T) {
	got := canonicalFrame([]byte{0x88, 0xE0, 0x18, 0x01})
	want := []byte{0xFE, 0xFE, 0x88, 0xE0, 0x18, 0x01, 0xFD}
	if !bytes.Equal(got, want) {
		t.Errorf("canonicalFrame = %s, want %s", hexBytes(got), hexBytes(want))
	}
}

func TestHexBytes(t *testing.T) {
	if got, want := hexBytes([]byte{0xFE, 0x00, 0x1A}), "FE 00 1A"; got != want {
		t.Errorf("hexBytes = %q, want %q", got, want)
	}
	if got, want := hexBytes(nil), "(empty)"; got != want {
		t.Errorf("hexBytes(nil) = %q, want %q", got, want)
	}
}

// hexFrames renders a slice of frames for a failure message.
func hexFrames(fs [][]byte) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = hexBytes(f)
	}
	return out
}
