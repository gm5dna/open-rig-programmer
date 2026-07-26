// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// --- BuildMRRead: reference "MR — MEMORY CHANNEL READ ... Read frame (6
// bytes): MR P0 P0 P0 ;". Golden vector G3: "MR007;". ---

func TestBuildMRRead_G3(t *testing.T) {
	s, err := FT710.MemorySlot(7)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := FT710.BuildMRRead(s)
	if err != nil {
		t.Fatalf("BuildMRRead(%q): unexpected error: %v", s.Wire(), err)
	}
	got := cmd.Bytes()
	want := "MR007;"
	if string(got) != want {
		t.Errorf("BuildMRRead(%q) = %q, want %q", s.Wire(), got, want)
	}
}

func TestBuildMRRead_AcceptsAllReadableKinds(t *testing.T) {
	slots := []Slot{}
	if s, err := FT710.MemorySlot(1); err == nil {
		slots = append(slots, s)
	}
	if s, err := FT710.PMSSlot(1, false); err == nil {
		slots = append(slots, s)
	}
	// 5xx and EMG are explicitly readable per the reference's slot table
	// ("MR read" column: ✓ for both) — MR read has no write-direction
	// hardware-verification concern, unlike MW/MT set.
	if s, err := FT710.SixtyMSlot(1); err == nil {
		slots = append(slots, s)
	}
	slots = append(slots, FT710.EMGSlot())

	for _, s := range slots {
		t.Run(s.Wire(), func(t *testing.T) {
			cmd, err := FT710.BuildMRRead(s)
			if err != nil {
				t.Fatalf("BuildMRRead(%q): unexpected error: %v", s.Wire(), err)
			}
			got := cmd.Bytes()
			want := "MR" + s.Wire() + ";"
			if string(got) != want {
				t.Errorf("BuildMRRead(%q) = %q, want %q", s.Wire(), got, want)
			}
		})
	}
}

func TestBuildMRRead_RejectsNone(t *testing.T) {
	none, err := FT710.ParseSlot("000")
	if err != nil {
		t.Fatal(err)
	}
	_, err = FT710.BuildMRRead(none)
	if err == nil {
		t.Fatal("BuildMRRead(\"000\"): want error, reference says do not emit")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("BuildMRRead(\"000\"): error is %T, want *ParseError", err)
	}
}

func TestBuildMRRead_RejectsZeroValueSlot(t *testing.T) {
	_, err := FT710.BuildMRRead(Slot{})
	if err == nil {
		t.Fatal("BuildMRRead(Slot{}): want error for uninitialised/invalid slot")
	}
}

// --- ParseMRAnswer: golden vectors G4, G6, G7 (G7 built via MW; see
// mw_test.go's round-trip test, which shares this decoder's field layout). ---

func TestParseMRAnswer_G4(t *testing.T) {
	frame := "MR001007000000+000000110000;"
	got, err := FT710.ParseMRAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", frame, err)
	}
	wantSlot, _ := FT710.MemorySlot(1)
	want := MemoryData{
		Slot:   wantSlot,
		FreqHz: 7_000_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeLSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
	if got != want {
		t.Errorf("ParseMRAnswer(%q) = %+v, want %+v", frame, got, want)
	}
}

func TestParseMRAnswer_G6(t *testing.T) {
	frame := "MRP1L001810000+000000150000;"
	got, err := FT710.ParseMRAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", frame, err)
	}
	wantSlot, _ := FT710.PMSSlot(1, false)
	want := MemoryData{
		Slot:   wantSlot,
		FreqHz: 1_810_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeLSB,
		Kind:   KindPMS,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
	if got != want {
		t.Errorf("ParseMRAnswer(%q) = %+v, want %+v", frame, got, want)
	}
}

// TestParseMRAnswer_G7SharedLayout parses G7's byte string (constructed
// with the MW prefix) after swapping the command prefix to MR, proving MR
// answer and MW set share the identical 28-byte body layout, per
// reference: "MW ... Set frame: identical 28-byte layout with MW".
func TestParseMRAnswer_G7SharedLayout(t *testing.T) {
	frame := "MR099052354000-012010411002;" // G7 with MW -> MR
	got, err := FT710.ParseMRAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", frame, err)
	}
	wantSlot, _ := FT710.MemorySlot(99)
	want := MemoryData{
		Slot:   wantSlot,
		FreqHz: 52_354_000,
		ClarHz: -120,
		RxClar: true,
		TxClar: false,
		Mode:   ModeFM,
		Kind:   KindMemory,
		CTCSS:  CTCSSEncDec,
		Shift:  ShiftMinus,
	}
	if got != want {
		t.Errorf("ParseMRAnswer(%q) = %+v, want %+v", frame, got, want)
	}
}

func TestParseMRAnswer_RejectTable(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"too short", "MR001007000000+00000011000;"},
		{"too long", "MR001007000000+0000001100000;"},
		{"empty", ""},
		{"wrong prefix", "XX001007000000+000000110000;"},
		{"lower case prefix", "mr001007000000+000000110000;"},
		{"missing terminator", "MR001007000000+000000110000X"},
		{"bad slot: 100 out of range", "MR100007000000+000000110000;"},
		{"bad slot: garbage", "MRXYZ007000000+000000110000;"},
		{"freq not digits", "MR00100X000000+000000110000;"},
		{"clar bad sign", "MR001007000000X000000110000;"},
		{"clar not digits", "MR001007000000+00X000110000;"},
		{"clar not a 10Hz step (9995)", "MR001007000000+999500110000;"},
		{"clar out of range (magnitude 9999, still fails >9990)", "MR001007000000+999910110000;"},
		{"rx clar garbage", "MR001007000000+0000X0110000;"},
		{"tx clar garbage", "MR001007000000+00000X110000;"},
		{"mode garbage (G)", "MR001007000000+000000G10000;"},
		{"kind garbage (6)", "MR001007000000+000000160000;"},
		{"kind garbage (7)", "MR001007000000+000000170000;"},
		{"kind garbage (8)", "MR001007000000+000000180000;"},
		{"kind garbage (9)", "MR001007000000+000000190000;"},
		{"ctcss garbage (3)", "MR001007000000+000000113000;"},
		{"P9 not fixed 00", "MR001007000000+0000001101X0;"},
		{"shift garbage (3)", "MR001007000000+000000110030;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FT710.ParseMRAnswer([]byte(tc.frame))
			if err == nil {
				t.Fatalf("ParseMRAnswer(%q): want error, got none", tc.frame)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseMRAnswer(%q): error is %T, want *ParseError", tc.frame, err)
			}
		})
	}
}

// TestParseMRAnswer_AcceptsKindUnset: reference P7 row "4 \"-\" (documented
// placeholder — parsers must ACCEPT it as unset, builders reject)". Mirrors
// ModeUnset's '0' = "-" convention (mode.go). This is the acceptance
// counterpart of the '4' case that used to sit in the reject table before
// the reference was updated to document P7=4's meaning.
func TestParseMRAnswer_AcceptsKindUnset(t *testing.T) {
	frame := "MR001007000000+000000140000;"
	got, err := FT710.ParseMRAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", frame, err)
	}
	if got.Kind != KindUnset {
		t.Errorf("ParseMRAnswer(%q).Kind = %q, want KindUnset (%q)", frame, got.Kind, KindUnset)
	}
}

// FuzzParseMRAnswer requires ParseMRAnswer never panics and only ever
// returns a typed *ParseError on failure.
func FuzzParseMRAnswer(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(""),
		[]byte("MR001007000000+000000110000;"), // G4
		[]byte("MRP1L001810000+000000150000;"), // G6
		[]byte("MR099052354000-012010411002;"), // G7 (MW->MR)
		[]byte("MR;"),
		[]byte("MR007;"),
		[]byte("MR100007000000+000000110000;"),
		[]byte("MR001007000000+000000140000;"),
		[]byte("MR001007000000X000000110000;"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		got, err := FT710.ParseMRAnswer(frame)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseMRAnswer(%q) returned non-ParseError: %T (%v)", frame, err, err)
			}
			return
		}
		if len(got.Slot.Wire()) != 3 {
			t.Fatalf("ParseMRAnswer(%q) succeeded with malformed Slot %q", frame, got.Slot.Wire())
		}
	})
}
