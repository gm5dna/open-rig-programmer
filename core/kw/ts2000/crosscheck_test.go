// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts2000"
)

// TestCrosscheck_FiveLiveAxesRoundTrip is this package's own proof, beyond
// kwtest.Run's generic walk, that the five axes lift K minted FOR THIS ROW
// (P10/P11/P12/P13/P15) actually carry a value through BuildMWSet and back
// through ParseMRAnswer unchanged — the thing the matrix pins by byte
// position (§2) and this test proves by round trip.
func TestCrosscheck_FiveLiveAxesRoundTrip(t *testing.T) {
	l := ts2000.TS2000
	slot, err := l.NewSlot(3, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(3): %v", err)
	}
	rec := kw.Record{
		Slot:       slot,
		FreqHz:     14250000,
		Mode:       kw.ModeUSB,
		Byte19:     '0',
		ToneMode:   kw.ToneModeOff,
		ToneIndex:  0,
		CTCSSIndex: 0,
		DCSCode:    23,     // P10, "See QC command" (ts2000:10711-10712)
		Byte28:     '1',    // P11 REVERSE ON (ts2000:10713-10714)
		Byte3940:   "03",   // P14 step index
		Byte41:     '5',    // P15 Memory Group 5 (ts2000:10723-10724)
		Shift:      '1',    // P12 "+" (ts2000:10715-10716, legend ts2000:10935-10938)
		OffsetHz:   600000, // P13 offset frequency
		Name:       "REPEATER",
	}

	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	if len(frame) != int(kw.RecordLen) {
		t.Fatalf("built frame is %d bytes, want %d", len(frame), kw.RecordLen)
	}
	answer := append([]byte{}, frame...)
	answer[0], answer[1] = 'M', 'R'

	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer of the frame this row's own BuildMWSet built: %v", err)
	}
	if got.DCSCode != rec.DCSCode {
		t.Errorf("DCSCode round-tripped as %d, want %d", got.DCSCode, rec.DCSCode)
	}
	if got.Byte28 != rec.Byte28 {
		t.Errorf("Byte28 (P11 REVERSE) round-tripped as %q, want %q", got.Byte28, rec.Byte28)
	}
	if got.Shift != rec.Shift {
		t.Errorf("Shift (P12) round-tripped as %q, want %q", got.Shift, rec.Shift)
	}
	if got.OffsetHz != rec.OffsetHz {
		t.Errorf("OffsetHz (P13) round-tripped as %d, want %d", got.OffsetHz, rec.OffsetHz)
	}
	if got.Byte41 != rec.Byte41 {
		t.Errorf("Byte41 (P15 Memory Group) round-tripped as %q, want %q", got.Byte41, rec.Byte41)
	}
}

// TestCrosscheck_ScanBandIsTheTenProgramScanChannels proves the 290-299
// split against the codec's own gate: slots 289 and 290 must resolve to
// DIFFERENT classes, and only a SCAN slot demands a half (ts2000:10726-
// 10727, ts2000:10797-10798).
func TestCrosscheck_ScanBandIsTheTenProgramScanChannels(t *testing.T) {
	l := ts2000.TS2000

	s, err := l.NewSlot(289, kw.ScanHalfNone)
	if err != nil || s.Class() != kw.SlotMemory {
		t.Errorf("slot 289: got class %v, err %v — want SlotMemory (ts2000:5614, \"00 ~ 289\")", s.Class(), err)
	}
	if _, err := l.NewSlot(290, kw.ScanHalfNone); err == nil {
		t.Error("slot 290 with no half was accepted — a Program Scan channel holds a start and an end frequency and a record naming it must say which (ts2000:10726-10727)")
	}
	lower, err := l.NewSlot(290, kw.ScanLower)
	if err != nil || lower.Class() != kw.SlotScan {
		t.Errorf("slot 290 (lower): got class %v, err %v — want SlotScan (ts2000:5614, \"290 ~ 299\")", lower.Class(), err)
	}
	upper, err := l.NewSlot(299, kw.ScanUpper)
	if err != nil || upper.Class() != kw.SlotScan {
		t.Errorf("slot 299 (upper): got class %v, err %v", upper.Class(), err)
	}
	if _, err := l.NewSlot(300, kw.ScanHalfNone); err == nil {
		t.Error("slot 300 was accepted — this row's slot space stops at 299 (ts2000:5614)")
	}
}

// TestCrosscheck_TYNotFV pins the identity-probe consequence of choosing
// Book480 for this row (layout.go's own doc comment): BuildTYRead succeeds
// and BuildFVRead is refused, on every row.
func TestCrosscheck_TYNotFV(t *testing.T) {
	for name, l := range map[string]kw.Layout{
		"TS-2000": ts2000.TS2000, "TS-2000X": ts2000.TS2000X, "TS-B2000": ts2000.TSB2000,
	} {
		if _, err := l.BuildTYRead(); err != nil {
			t.Errorf("%s: BuildTYRead: %v — this document prints TY (ts2000:11678-11693), not FV", name, err)
		}
		if _, err := l.BuildFVRead(); err == nil {
			t.Errorf("%s: BuildFVRead succeeded — this document prints no FV command anywhere", name)
		}
	}
}
