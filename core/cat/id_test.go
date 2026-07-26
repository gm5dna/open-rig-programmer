// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// TestBuildIDRead: golden vector G1's request half, "ID;".
func TestBuildIDRead(t *testing.T) {
	got := FT710.BuildIDRead().Bytes()
	want := "ID;"
	if string(got) != want {
		t.Errorf("BuildIDRead() = %q, want %q", got, want)
	}
}

// TestParseIDAnswer_G1: golden vector G1, "ID; -> ID0800;" — the FT-710's
// fixed radio ID.
func TestParseIDAnswer_G1(t *testing.T) {
	id, err := FT710.ParseIDAnswer([]byte("ID0800;"))
	if err != nil {
		t.Fatalf("ParseIDAnswer(%q): unexpected error: %v", "ID0800;", err)
	}
	if id != "0800" {
		t.Errorf("ParseIDAnswer(%q) = %q, want %q", "ID0800;", id, "0800")
	}
}

func TestParseIDAnswer_RejectTable(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"too short (6 bytes)", "ID080;"},
		{"too long (8 bytes)", "ID08000;"},
		{"empty", ""},
		{"wrong prefix", "XX0800;"},
		{"lower case prefix", "id0800;"},
		{"missing terminator", "ID0800X"},
		{"missing terminator, truncated", "ID0800"},
		{"only prefix and terminator", "ID;"},
		{"garbage bytes, wrong shape (too short)", "ID\x00\x01;"},
		{"garbage bytes, wrong shape (no terminator)", "ID\x00\x01\x02\x03X"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FT710.ParseIDAnswer([]byte(tc.frame))
			if err == nil {
				t.Fatalf("ParseIDAnswer(%q): want error, got none", tc.frame)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseIDAnswer(%q): error is %T, want *ParseError", tc.frame, err)
			}
		})
	}
}

// TestParseIDAnswer_AcceptsAnyFourByteBody: the brief specifies structural
// validation only (length/prefix/terminator) for the 4-character ID body,
// not its charset, so any 4 bytes there — including non-digits and control
// bytes — are accepted once the surrounding shape is exactly right.
func TestParseIDAnswer_AcceptsAnyFourByteBody(t *testing.T) {
	tests := []struct {
		frame  string
		wantID string
	}{
		{"ID0800;", "0800"},
		{"IDABCD;", "ABCD"},
		{"ID\x00\x01\x02\x03;", "\x00\x01\x02\x03"},
	}
	for _, tc := range tests {
		id, err := FT710.ParseIDAnswer([]byte(tc.frame))
		if err != nil {
			t.Fatalf("ParseIDAnswer(%q): unexpected error: %v", tc.frame, err)
		}
		if id != tc.wantID {
			t.Errorf("ParseIDAnswer(%q) = %q, want %q", tc.frame, id, tc.wantID)
		}
	}
}

// FuzzParseIDAnswer requires ParseIDAnswer never panics and only ever
// returns a typed *ParseError on failure.
func FuzzParseIDAnswer(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(""),
		[]byte("ID0800;"),
		[]byte("ID;"),
		[]byte("ID080;"),
		[]byte("ID08000;"),
		[]byte("XX0800;"),
		[]byte("id0800;"),
		[]byte("ID0800X"),
		[]byte("ID\x00\x01\x02\x03;"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		id, err := FT710.ParseIDAnswer(frame)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseIDAnswer(%q) returned non-ParseError: %T (%v)", frame, err, err)
			}
			return
		}
		if len(id) != 4 {
			t.Fatalf("ParseIDAnswer(%q) succeeded with id %q of length %d, want 4", frame, id, len(id))
		}
	})
}
