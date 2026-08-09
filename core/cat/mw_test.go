// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// validMW5 returns a valid, fully-populated MemoryData for the M-05 memory
// slot, matching golden vector G5 ("MW005014250000+000000210000;"). Tests
// mutate a copy of this to isolate exactly one bad field per rejection
// case.
func validMW5(t *testing.T) MemoryData {
	t.Helper()
	slot, err := FT710.MemorySlot(5)
	if err != nil {
		t.Fatal(err)
	}
	return MemoryData{
		Slot:   slot,
		FreqHz: 14_250_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeUSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
}

// --- BuildMWSet: golden vectors G5, G7 ---

func TestBuildMWSet_G5(t *testing.T) {
	cmd, err := FT710.BuildMWSet(validMW5(t))
	if err != nil {
		t.Fatalf("BuildMWSet: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MW005014250000+000000210000;"
	if string(got) != want {
		t.Errorf("BuildMWSet() = %q, want %q", got, want)
	}
}

func TestBuildMWSet_G7(t *testing.T) {
	slot, err := FT710.MemorySlot(99)
	if err != nil {
		t.Fatal(err)
	}
	m := MemoryData{
		Slot:   slot,
		FreqHz: 52_354_000,
		ClarHz: -120,
		RxClar: true,
		TxClar: false,
		Mode:   ModeFM,
		Kind:   KindMemory,
		CTCSS:  CTCSSEncDec,
		Shift:  ShiftMinus,
	}
	cmd, err := FT710.BuildMWSet(m)
	if err != nil {
		t.Fatalf("BuildMWSet: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MW099052354000-012010411002;"
	if string(got) != want {
		t.Errorf("BuildMWSet() = %q, want %q", got, want)
	}
}

// TestBuildMWSet_PMSAcceptsKindMemory_HWConfirmed pins the M5b-confirmed
// kind pairing (docs/hardware-notes.md): a PMS slot's MW write carries
// KindMemory ('1'), exactly like a memory slot — NOT KindPMS ('5'), as
// this project's former ASSUMED pairing (and the manual's own worked
// example) claimed. This test formerly asserted the OPPOSITE (KindPMS
// accepted on a PMS write) — that assumption is hardware-refuted; see
// TestBuildMWSet_RejectsKindPMSOnPMSSlot_HWConfirmed for the new
// rejection this task adds alongside it.
func TestBuildMWSet_PMSAcceptsKindMemory_HWConfirmed(t *testing.T) {
	slot, err := FT710.PMSSlot(1, false)
	if err != nil {
		t.Fatal(err)
	}
	m := validMW5(t)
	m.Slot = slot
	m.Kind = KindMemory
	cmd, err := FT710.BuildMWSet(m)
	if err != nil {
		t.Fatalf("BuildMWSet(PMS, Kind=Memory): unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MWP1L014250000+000000210000;" // kind digit '1' = KindMemory, matching G5's own kind byte
	if string(got) != want {
		t.Errorf("BuildMWSet() = %q, want %q", got, want)
	}
}

// TestBuildMWSet_RejectsKindPMSOnPMSSlot_HWConfirmed: HW-CONFIRMED
// 2026-07-13 (M5b write trials, docs/hardware-notes.md) — a PMS write
// carrying KindPMS ('5') is REJECTED by the real radio with an immediate
// "?;" (live vector: MWP1L007100000+000000150000;). BuildMWSet must
// refuse it too, the same way it refuses any other non-KindMemory value.
func TestBuildMWSet_RejectsKindPMSOnPMSSlot_HWConfirmed(t *testing.T) {
	slot, err := FT710.PMSSlot(1, false)
	if err != nil {
		t.Fatal(err)
	}
	m := validMW5(t)
	m.Slot = slot
	m.Kind = KindPMS
	if _, err := FT710.BuildMWSet(m); err == nil {
		t.Fatal("BuildMWSet(PMS, Kind=PMS): want error (HW-CONFIRMED rejected), got success")
	}
}

// TestBuildMWSet_RejectTable covers every rejection case the brief and
// reference list for BuildMWSet: slot not writable under the receiving
// dialect (501, EMG, 000; Dialect.writableSlot),
// Kind/slot pairing mismatch, ModeUnset, ClarHz not a 10 Hz step or out of
// range, FreqHz needing >9 digits or zero.
func TestBuildMWSet_RejectTable(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m MemoryData) MemoryData
		altSlot func(t *testing.T) Slot // used instead of validMW5's slot, if set
	}{
		{
			name: "slot 501 not writable: reference rejection list",
			altSlot: func(t *testing.T) Slot {
				s, err := FT710.SixtyMSlot(1)
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
		},
		{
			name: "slot EMG not writable: reference rejection list",
			altSlot: func(t *testing.T) Slot {
				return FT710.EMGSlot()
			},
		},
		{
			name: "slot 000 not writable: reference rejection list",
			altSlot: func(t *testing.T) Slot {
				s, err := FT710.ParseSlot("000")
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
		},
		{
			name: "Kind mismatch: PMS kind on a memory slot",
			mutate: func(m MemoryData) MemoryData {
				m.Kind = KindPMS
				return m
			},
		},
		{
			name: "Kind mismatch: PMS kind on a PMS slot (HW-CONFIRMED rejected, was wrongly accepted pre-M5b)",
			altSlot: func(t *testing.T) Slot {
				s, err := FT710.PMSSlot(1, false)
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			mutate: func(m MemoryData) MemoryData {
				m.Kind = KindPMS
				return m
			},
		},
		{
			name: "Kind mismatch: VFO kind on a memory slot",
			mutate: func(m MemoryData) MemoryData {
				m.Kind = KindVFO
				return m
			},
		},
		{
			name: "Kind mismatch: unused kind 4",
			mutate: func(m MemoryData) MemoryData {
				m.Kind = '4'
				return m
			},
		},
		{
			name: "ModeUnset rejected in a Set frame",
			mutate: func(m MemoryData) MemoryData {
				m.Mode = ModeUnset
				return m
			},
		},
		{
			name: "Mode forged out of range byte",
			mutate: func(m MemoryData) MemoryData {
				m.Mode = Mode('Z')
				return m
			},
		},
		{
			name: "ClarHz 9995 not a 10 Hz step: reference rejection list",
			mutate: func(m MemoryData) MemoryData {
				m.ClarHz = 9995
				return m
			},
		},
		{
			name: "ClarHz +10000 out of range: reference rejection list",
			mutate: func(m MemoryData) MemoryData {
				m.ClarHz = 10000
				return m
			},
		},
		{
			name: "ClarHz -10000 out of range: reference rejection list",
			mutate: func(m MemoryData) MemoryData {
				m.ClarHz = -10000
				return m
			},
		},
		{
			name: "FreqHz needing >9 digits (1,000,000,000): reference rejection list",
			mutate: func(m MemoryData) MemoryData {
				m.FreqHz = 1_000_000_000
				return m
			},
		},
		{
			name: "FreqHz zero",
			mutate: func(m MemoryData) MemoryData {
				m.FreqHz = 0
				return m
			},
		},
		{
			name: "CTCSS forged out of range byte",
			mutate: func(m MemoryData) MemoryData {
				m.CTCSS = CTCSSState('9')
				return m
			},
		},
		{
			name: "Shift forged out of range byte",
			mutate: func(m MemoryData) MemoryData {
				m.Shift = Shift('9')
				return m
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validMW5(t)
			if tc.altSlot != nil {
				m.Slot = tc.altSlot(t)
			}
			if tc.mutate != nil {
				m = tc.mutate(m)
			}
			out, err := FT710.BuildMWSet(m)
			if err == nil {
				t.Fatalf("BuildMWSet(%+v): want error, got %q", m, out.Bytes())
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("BuildMWSet(%+v): error is %T, want *ParseError", m, err)
			}
		})
	}
}

func TestBuildMWSet_RejectsZeroValueSlot(t *testing.T) {
	m := validMW5(t)
	m.Slot = Slot{}
	if _, err := FT710.BuildMWSet(m); err == nil {
		t.Fatal("BuildMWSet(zero-value Slot): want error")
	}
}

// TestParseMRAnswer_RoundTripsBuildMWSet feeds BuildMWSet's own G5/G7
// output back through ParseMRAnswer (swapping the MW prefix for MR, since
// they share byte-for-byte layout) and checks the MemoryData round-trips
// exactly.
func TestParseMRAnswer_RoundTripsBuildMWSet(t *testing.T) {
	cases := []MemoryData{
		validMW5(t),
	}
	if slot, err := FT710.MemorySlot(99); err == nil {
		cases = append(cases, MemoryData{
			Slot:   slot,
			FreqHz: 52_354_000,
			ClarHz: -120,
			RxClar: true,
			TxClar: false,
			Mode:   ModeFM,
			Kind:   KindMemory,
			CTCSS:  CTCSSEncDec,
			Shift:  ShiftMinus,
		})
	}
	if slot, err := FT710.PMSSlot(1, false); err == nil {
		m := validMW5(t)
		m.Slot = slot
		m.Kind = KindMemory // HW-CONFIRMED 2026-07-13: PMS writes carry KindMemory, not KindPMS
		cases = append(cases, m)
	}

	for _, want := range cases {
		t.Run(want.Slot.Wire(), func(t *testing.T) {
			cmd, err := FT710.BuildMWSet(want)
			if err != nil {
				t.Fatalf("BuildMWSet(%+v): unexpected error: %v", want, err)
			}
			built := cmd.Bytes()
			mrFrame := append([]byte("MR"), built[2:]...)
			got, err := FT710.ParseMRAnswer(mrFrame)
			if err != nil {
				t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", mrFrame, err)
			}
			if got != want {
				t.Errorf("round trip: got %+v, want %+v", got, want)
			}
		})
	}
}
