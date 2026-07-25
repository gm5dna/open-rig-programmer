// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"bytes"
	"testing"
)

// TestSplitFrames_EmptyBuffer: nothing in, nothing out.
func TestSplitFrames_EmptyBuffer(t *testing.T) {
	frames, rest := SplitFrames(nil)
	if len(frames) != 0 {
		t.Errorf("frames = %v, want none", frames)
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q, want empty", rest)
	}
}

// TestSplitFrames_OneExactFrame: a single complete frame, nothing trailing.
// Uses golden vector G1's answer, "ID0800;".
func TestSplitFrames_OneExactFrame(t *testing.T) {
	frames, rest := SplitFrames([]byte("ID0800;"))
	if len(frames) != 1 || string(frames[0]) != "ID0800;" {
		t.Errorf("frames = %v, want [\"ID0800;\"]", framesAsStrings(frames))
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q, want empty", rest)
	}
}

// TestSplitFrames_FramePlusPartialRest: a complete frame followed by an
// unterminated partial frame, which must be returned as rest, not frames.
func TestSplitFrames_FramePlusPartialRest(t *testing.T) {
	frames, rest := SplitFrames([]byte("ID0800;AI"))
	if len(frames) != 1 || string(frames[0]) != "ID0800;" {
		t.Errorf("frames = %v, want [\"ID0800;\"]", framesAsStrings(frames))
	}
	if string(rest) != "AI" {
		t.Errorf("rest = %q, want %q", rest, "AI")
	}
}

// TestSplitFrames_MultipleFramesOneRead: several complete frames arriving
// in a single read. Uses golden vectors G1 (answer), G2, and G12.
func TestSplitFrames_MultipleFramesOneRead(t *testing.T) {
	frames, rest := SplitFrames([]byte("ID0800;AI0;?;"))
	want := []string{"ID0800;", "AI0;", "?;"}
	if len(frames) != len(want) {
		t.Fatalf("frames = %v, want %v", framesAsStrings(frames), want)
	}
	for i, w := range want {
		if string(frames[i]) != w {
			t.Errorf("frames[%d] = %q, want %q", i, frames[i], w)
		}
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q, want empty", rest)
	}
}

// TestSplitFrames_PartialOnly: no terminator at all, so everything is rest.
func TestSplitFrames_PartialOnly(t *testing.T) {
	frames, rest := SplitFrames([]byte("AI0"))
	if len(frames) != 0 {
		t.Errorf("frames = %v, want none", framesAsStrings(frames))
	}
	if string(rest) != "AI0" {
		t.Errorf("rest = %q, want %q", rest, "AI0")
	}
}

// TestSplitFrames_RejectionMixedBetweenAnswers: G12 ("?;") interleaved
// between normal answers must come through as its own frame.
func TestSplitFrames_RejectionMixedBetweenAnswers(t *testing.T) {
	frames, rest := SplitFrames([]byte("ID0800;?;AI0;"))
	want := []string{"ID0800;", "?;", "AI0;"}
	if len(frames) != len(want) {
		t.Fatalf("frames = %v, want %v", framesAsStrings(frames), want)
	}
	for i, w := range want {
		if string(frames[i]) != w {
			t.Errorf("frames[%d] = %q, want %q", i, frames[i], w)
		}
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q, want empty", rest)
	}
	if !IsRejection(frames[1]) {
		t.Errorf("IsRejection(%q) = false, want true", frames[1])
	}
}

// TestSplitFrames_ToleratesEmptyFrames: a stray leading ';' produces a
// 1-byte frame containing only the terminator; SplitFrames must not panic
// or drop it, per the brief's "tolerate empty frames" requirement.
func TestSplitFrames_ToleratesEmptyFrames(t *testing.T) {
	frames, rest := SplitFrames([]byte(";;ID0800;"))
	want := []string{";", ";", "ID0800;"}
	if len(frames) != len(want) {
		t.Fatalf("frames = %v, want %v", framesAsStrings(frames), want)
	}
	for i, w := range want {
		if string(frames[i]) != w {
			t.Errorf("frames[%d] = %q, want %q", i, frames[i], w)
		}
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q, want empty", rest)
	}
}

// TestSplitFrames_ByteAtATimeReassembly simulates a stream arriving one
// byte at a time: feed SplitFrames a growing buffer built from the
// previous call's rest plus one more byte, and confirm every frame is
// eventually recovered, in order, with nothing lost or duplicated.
func TestSplitFrames_ByteAtATimeReassembly(t *testing.T) {
	full := []byte("ID0800;AI0;?;MC099;")
	wantFrames := []string{"ID0800;", "AI0;", "?;", "MC099;"}

	var pending []byte
	var got []string
	for i := 0; i < len(full); i++ {
		pending = append(pending, full[i])
		frames, rest := SplitFrames(pending)
		for _, f := range frames {
			got = append(got, string(f))
		}
		pending = rest
	}
	if len(pending) != 0 {
		t.Errorf("leftover pending = %q, want empty after full stream consumed", pending)
	}
	if len(got) != len(wantFrames) {
		t.Fatalf("reassembled frames = %v, want %v", got, wantFrames)
	}
	for i, w := range wantFrames {
		if got[i] != w {
			t.Errorf("frame[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestIsRejection: golden vector G12, "?;", plus negative cases.
func TestIsRejection(t *testing.T) {
	tests := []struct {
		name  string
		frame string
		want  bool
	}{
		{"G12 exact rejection", "?;", true},
		{"answer frame is not a rejection", "ID0800;", false},
		{"empty frame is not a rejection", "", false},
		{"question mark without terminator", "?", false},
		{"rejection with leading garbage", "X?;", false},
		{"rejection embedded, not exact", "?;;", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRejection([]byte(tc.frame)); got != tc.want {
				t.Errorf("IsRejection(%q) = %v, want %v", tc.frame, got, tc.want)
			}
		})
	}
}

func framesAsStrings(frames [][]byte) []string {
	out := make([]string, len(frames))
	for i, f := range frames {
		out[i] = string(f)
	}
	return out
}

// FuzzSplitFrames requires that SplitFrames never panics on arbitrary
// input, and that concatenating every returned frame plus rest exactly
// reconstructs the input (no bytes lost, duplicated, or reordered).
func FuzzSplitFrames(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(""),
		[]byte(";"),
		[]byte(";;"),
		[]byte("ID0800;"),
		[]byte("ID0800;AI0;?;"),
		[]byte("AI0"),
		[]byte("ID0800;AI"),
		[]byte("?;"),
		[]byte("\x00;\x00;"),
		[]byte(";;;;;;"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, buf []byte) {
		frames, rest := SplitFrames(buf)

		var reconstructed []byte
		for _, fr := range frames {
			reconstructed = append(reconstructed, fr...)
		}
		reconstructed = append(reconstructed, rest...)
		if !bytes.Equal(reconstructed, buf) {
			t.Fatalf("SplitFrames(%q) frames+rest = %q, want exact reconstruction", buf, reconstructed)
		}
		for _, fr := range frames {
			if len(fr) == 0 || fr[len(fr)-1] != ';' {
				t.Fatalf("SplitFrames(%q): frame %q does not end with ';'", buf, fr)
			}
		}
		if len(rest) > 0 && rest[len(rest)-1] == ';' {
			t.Fatalf("SplitFrames(%q): rest %q should not itself be terminator-complete", buf, rest)
		}
		// IsRejection must never panic on fuzzer-supplied bytes either.
		_ = IsRejection(buf)
	})
}
