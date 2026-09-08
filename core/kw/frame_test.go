// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"bytes"
	"testing"
)

// TestSplitFrames_TerminatorInclusiveAndRestReturned pins the ';' splitter's
// two halves: every complete frame carries its own terminator, and whatever
// follows the last one is handed back for the next read to prepend.
func TestSplitFrames_TerminatorInclusiveAndRestReturned(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
		rest string
	}{
		{"one whole frame", "ID023;", []string{"ID023;"}, ""},
		{"two frames", "ID023;FV1.00;", []string{"ID023;", "FV1.00;"}, ""},
		{"trailing partial", "ID023;FV1.", []string{"ID023;"}, "FV1."},
		{"nothing complete", "MR0", nil, "MR0"},
		{"empty", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames, rest := SplitFrames([]byte(tt.in))
			if len(frames) != len(tt.want) {
				t.Fatalf("SplitFrames(%q) returned %d frames, want %d", tt.in, len(frames), len(tt.want))
			}
			for i, w := range tt.want {
				if string(frames[i]) != w {
					t.Errorf("frame %d = %q, want %q", i, frames[i], w)
				}
			}
			if string(rest) != tt.rest {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

// TestSplitFrames_ToleratesConsecutiveTerminators is the copied rule that
// matters most, and it is copied on the ROBUSTNESS ground the plan states
// (T5) and NOT on the 480's "; ; ; ; PS1;" wake-up, which is a non-goal and
// whose printed spacing may be PageMaker letter-spacing (E18).
//
// A noisy line can deliver ";;". The splitter yields a 1-byte frame holding
// only the terminator rather than raising an error, and the matcher — never
// this function — decides that such a frame answers nothing.
func TestSplitFrames_ToleratesConsecutiveTerminators(t *testing.T) {
	frames, rest := SplitFrames([]byte(";;ID023;"))
	want := []string{";", ";", "ID023;"}
	if len(frames) != len(want) {
		t.Fatalf("got %d frames, want %d: %q", len(frames), len(want), frames)
	}
	for i, w := range want {
		if string(frames[i]) != w {
			t.Errorf("frame %d = %q, want %q", i, frames[i], w)
		}
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q, want empty", rest)
	}
}

// TestSplitFrames_EveryByteAccountedFor pins the no-loss property the
// accumulator depends on: each input byte appears in exactly one frame or in
// rest.
func TestSplitFrames_EveryByteAccountedFor(t *testing.T) {
	in := []byte("MC 07;;?;E;MR0007;partial")
	frames, rest := SplitFrames(in)
	var seen bytes.Buffer
	for _, f := range frames {
		seen.Write(f)
	}
	seen.Write(rest)
	if !bytes.Equal(seen.Bytes(), in) {
		t.Errorf("frames+rest = %q, want the input %q", seen.Bytes(), in)
	}
}

// TestIsRejection_IsExactlyTheNAKAndNothingElse is the negative that carries
// the whole acknowledgement design: "?;" is the ONLY rejection. "E;" and
// "O;" are stream-health tokens and must never reach the engine's rejection
// path, where they would become ErrRejected — a refusal, not a link failure,
// and indistinguishable from "?;" (spec §"Acknowledgement conventions",
// route 3 of §"Where E; and O; live").
func TestIsRejection_IsExactlyTheNAKAndNothingElse(t *testing.T) {
	tests := []struct {
		frame string
		want  bool
	}{
		{"?;", true},
		{"E;", false},
		{"O;", false},
		{"?", false},
		{"??;", false},
		{"?;;", false},
		{";", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsRejection([]byte(tt.frame)); got != tt.want {
			t.Errorf("IsRejection(%q) = %v, want %v", tt.frame, got, tt.want)
		}
	}
}

// TestStreamErrorToken_NamesTheTwoTokensAndNothingElse pins the recogniser
// the framing's IsFatal is built on. All four books print the two tokens
// beside "?;" in one error-message table (590:97-113, 480:126-144,
// 890:106-123, 990:108-121); nothing else in any of them is a stream-health
// token.
func TestStreamErrorToken_NamesTheTwoTokensAndNothingElse(t *testing.T) {
	tests := []struct {
		frame string
		want  string
	}{
		{"E;", "E;"},
		{"O;", "O;"},
		{"?;", ""},
		{"e;", ""},
		{"o;", ""},
		{"E", ""},
		{"E;;", ""},
		{"EX000000;", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := streamErrorToken([]byte(tt.frame)); got != tt.want {
			t.Errorf("streamErrorToken(%q) = %q, want %q", tt.frame, got, tt.want)
		}
	}
}
