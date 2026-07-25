// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// --- BuildMCSet: golden vector G11 ---

func TestBuildMCSet_G11(t *testing.T) {
	s, err := MemorySlot(99)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMCSet(s)
	if err != nil {
		t.Fatalf("BuildMCSet: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MC099;"
	if string(got) != want {
		t.Errorf("BuildMCSet() = %q, want %q", got, want)
	}
}

// TestBuildMCSet_AllowsWiderSlotSetThanMW: unlike MW, MC set explicitly
// allows 5xx and EMG per the reference's slot table ("MC set" column: ✓
// for both).
func TestBuildMCSet_AllowsWiderSlotSetThanMW(t *testing.T) {
	sixtyM, err := SixtyMSlot(1)
	if err != nil {
		t.Fatal(err)
	}
	if cmd, err := BuildMCSet(sixtyM); err != nil {
		t.Errorf("BuildMCSet(5xx): unexpected error: %v", err)
	} else if got, want := cmd.Bytes(), "MC501;"; string(got) != want {
		t.Errorf("BuildMCSet(5xx) = %q, want %q", got, want)
	}
	if cmd, err := BuildMCSet(EMGSlot()); err != nil {
		t.Errorf("BuildMCSet(EMG): unexpected error: %v", err)
	} else if got, want := cmd.Bytes(), "MCEMG;"; string(got) != want {
		t.Errorf("BuildMCSet(EMG) = %q, want %q", got, want)
	}
}

func TestBuildMCSet_RejectsNoneAnd000(t *testing.T) {
	none, err := ParseSlot("000")
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildMCSet(none)
	if err == nil {
		t.Fatal("BuildMCSet(\"000\"): want error, reference MC set column: ✗")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("BuildMCSet(\"000\"): error is %T, want *ParseError", err)
	}
}

func TestBuildMCSet_RejectsZeroValueSlot(t *testing.T) {
	if _, err := BuildMCSet(Slot{}); err == nil {
		t.Fatal("BuildMCSet(Slot{}): want error")
	}
}

// --- BuildMCRead ---

func TestBuildMCRead(t *testing.T) {
	got := BuildMCRead().Bytes()
	want := "MC;"
	if string(got) != want {
		t.Errorf("BuildMCRead() = %q, want %q", got, want)
	}
}

// --- ParseMCAnswer: golden vector G11's shape (Answer mirrors Set) ---

func TestParseMCAnswer_G11(t *testing.T) {
	got, err := ParseMCAnswer([]byte("MC099;"))
	if err != nil {
		t.Fatalf("ParseMCAnswer: unexpected error: %v", err)
	}
	want, _ := MemorySlot(99)
	if got != want {
		t.Errorf("ParseMCAnswer(%q) = %q, want %q", "MC099;", got.Wire(), want.Wire())
	}
}

func TestParseMCAnswer_Allows5xxAndEMG(t *testing.T) {
	got, err := ParseMCAnswer([]byte("MC501;"))
	if err != nil {
		t.Fatalf("ParseMCAnswer(5xx): unexpected error: %v", err)
	}
	want, _ := SixtyMSlot(1)
	if got != want {
		t.Errorf("ParseMCAnswer(5xx) = %q, want %q", got.Wire(), want.Wire())
	}

	got, err = ParseMCAnswer([]byte("MCEMG;"))
	if err != nil {
		t.Fatalf("ParseMCAnswer(EMG): unexpected error: %v", err)
	}
	if got != EMGSlot() {
		t.Errorf("ParseMCAnswer(EMG) = %q, want %q", got.Wire(), EMGSlot().Wire())
	}
}

func TestParseMCAnswer_RejectTable(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"too short", "MC99;"},
		{"too long", "MC0099;"},
		{"empty", ""},
		{"wrong prefix", "XX099;"},
		{"lower case prefix", "mc099;"},
		{"missing terminator", "MC099X"},
		{"bad slot: out of range", "MC100;"},
		{"bad slot: garbage", "MCXYZ;"},
		{"slot 000 rejected: reference MC set column ✗", "MC000;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseMCAnswer([]byte(tc.frame))
			if err == nil {
				t.Fatalf("ParseMCAnswer(%q): want error, got none", tc.frame)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseMCAnswer(%q): error is %T, want *ParseError", tc.frame, err)
			}
		})
	}
}

// FuzzParseMCAnswer requires ParseMCAnswer never panics and only ever
// returns a typed *ParseError on failure.
func FuzzParseMCAnswer(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(""),
		[]byte("MC099;"), // G11
		[]byte("MC;"),
		[]byte("MC501;"),
		[]byte("MCEMG;"),
		[]byte("MC000;"),
		[]byte("MC100;"),
		[]byte("MCXYZ;"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		got, err := ParseMCAnswer(frame)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseMCAnswer(%q) returned non-ParseError: %T (%v)", frame, err, err)
			}
			return
		}
		if len(got.Wire()) != 3 {
			t.Fatalf("ParseMCAnswer(%q) succeeded with malformed Slot %q", frame, got.Wire())
		}
	})
}
