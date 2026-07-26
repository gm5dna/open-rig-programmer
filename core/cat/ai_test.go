// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// TestBuildAISet_G2: golden vector G2, "AI0;" (auto-information off), plus
// the "on" counterpart.
func TestBuildAISet_G2(t *testing.T) {
	tests := []struct {
		on   bool
		want string
	}{
		{false, "AI0;"},
		{true, "AI1;"},
	}
	for _, tc := range tests {
		if got := string(FT710.BuildAISet(tc.on).Bytes()); got != tc.want {
			t.Errorf("BuildAISet(%v) = %q, want %q", tc.on, got, tc.want)
		}
	}
}

func TestParseAIAnswer(t *testing.T) {
	tests := []struct {
		name    string
		frame   string
		wantOn  bool
		wantErr bool
	}{
		{"G2: AI0; -> off", "AI0;", false, false},
		{"AI1; -> on", "AI1;", true, false},
		{"too short", "AI0", true, true},
		{"too long", "AI00;", true, true},
		{"empty", "", true, true},
		{"wrong prefix", "XX0;", true, true},
		{"lower case prefix", "ai0;", true, true},
		{"missing terminator", "AI0X", true, true},
		{"garbage state digit", "AI2;", true, true},
		{"garbage state byte", "AIX;", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			on, err := FT710.ParseAIAnswer([]byte(tc.frame))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseAIAnswer(%q): want error, got on=%v", tc.frame, on)
				}
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Errorf("ParseAIAnswer(%q): error is %T, want *ParseError", tc.frame, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAIAnswer(%q): unexpected error: %v", tc.frame, err)
			}
			if on != tc.wantOn {
				t.Errorf("ParseAIAnswer(%q) = %v, want %v", tc.frame, on, tc.wantOn)
			}
		})
	}
}

// TestAISet_RoundTrip: BuildAISet output must parse back to the same on/off
// state via ParseAIAnswer (the Set and Answer frames share the same shape
// per the reference: "Set/Read/Answer all exist").
func TestAISet_RoundTrip(t *testing.T) {
	for _, on := range []bool{true, false} {
		frame := FT710.BuildAISet(on).Bytes()
		got, err := FT710.ParseAIAnswer(frame)
		if err != nil {
			t.Fatalf("ParseAIAnswer(BuildAISet(%v)) unexpected error: %v", on, err)
		}
		if got != on {
			t.Errorf("ParseAIAnswer(BuildAISet(%v)) = %v, want %v", on, got, on)
		}
	}
}
