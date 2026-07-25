// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import "testing"

func TestImageUK_M01(t *testing.T) {
	slots := ImageUK()
	s, ok := slots["001"]
	if !ok || !s.Populated {
		t.Fatalf("ImageUK()[\"001\"] populated = %v, ok = %v, want true, true", s.Populated, ok)
	}
	if s.Freq != "007000000" || s.Mode != modeLSB {
		t.Errorf("ImageUK()[\"001\"] = %+v, want Freq=007000000 Mode=LSB (golden vector G4)", s)
	}
}

func TestImageUK_PMS_P1L_MatchesGoldenVectorG6(t *testing.T) {
	slots := ImageUK()
	s, ok := slots["P1L"]
	if !ok || !s.Populated {
		t.Fatalf("ImageUK()[\"P1L\"] populated = %v, ok = %v, want true, true", s.Populated, ok)
	}
	if s.Freq != "001810000" || s.Mode != modeLSB || s.Kind != kindPMS {
		t.Errorf("ImageUK()[\"P1L\"] = %+v, want Freq=001810000 Mode=LSB Kind=PMS(5) (golden vector G6)", s)
	}
}

func TestImageUK_AllEighteenPMSSlotsPopulated(t *testing.T) {
	slots := ImageUK()
	for pair := 1; pair <= 9; pair++ {
		for _, half := range []byte{'L', 'U'} {
			slot := pmsSlot(pair, half)
			s, ok := slots[slot]
			if !ok || !s.Populated {
				t.Errorf("ImageUK()[%q] populated = %v, ok = %v, want true, true", slot, s.Populated, ok)
			}
			if s.Kind != kindPMS {
				t.Errorf("ImageUK()[%q].Kind = %q, want PMS(5)", slot, s.Kind)
			}
		}
	}
}

func TestImageUK_NoSixtyMetreBank(t *testing.T) {
	// HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §60m regional
	// finding): Stuart's UK FT-710 has NO factory 5xx bank at all —
	// overturns the former 501-507 synthetic placeholder set.
	slots := ImageUK()
	for n := 501; n <= 515; n++ {
		slot := sixtyMetreChannel(n - 500)
		if _, ok := slots[slot]; ok {
			t.Errorf("ImageUK()[%q] exists, want absent (HW-CONFIRMED: no 5xx bank on the UK variant)", slot)
		}
	}
}

func TestImageUK_EMGAbsent(t *testing.T) {
	// HW-CONFIRMED 2026-07-13: front-panel confirmed no EMG channel on
	// Stuart's UK FT-710, consistent with the absent 5xx bank.
	slots := ImageUK()
	if _, ok := slots["EMG"]; ok {
		t.Error("ImageUK()[\"EMG\"] exists, want absent (HW-CONFIRMED regional variation)")
	}
}

func TestImageUS_60mChannels501to515Populated(t *testing.T) {
	slots := ImageUS()
	for n := 501; n <= 515; n++ {
		slot := sixtyMetreChannel(n - 500)
		s, ok := slots[slot]
		if !ok || !s.Populated {
			t.Errorf("ImageUS()[%q] populated = %v, ok = %v, want true, true", slot, s.Populated, ok)
		}
	}
	if _, ok := slots["516"]; ok {
		t.Error("ImageUS()[\"516\"] exists, want absent (US 60m set is 501-515)")
	}
}

func TestImageUS_EMGPopulated(t *testing.T) {
	slots := ImageUS()
	s, ok := slots["EMG"]
	if !ok || !s.Populated {
		t.Fatalf("ImageUS()[\"EMG\"] populated = %v, ok = %v, want true, true", s.Populated, ok)
	}
	if s.Freq != "005167500" {
		t.Errorf("ImageUS()[\"EMG\"].Freq = %q, want \"005167500\" (5.1675 MHz)", s.Freq)
	}
}

func TestImage_CallsReturnIndependentMaps(t *testing.T) {
	a := ImageUK()
	b := ImageUK()
	aState := a["001"]
	aState.Freq = "999999999"
	a["001"] = aState

	if b["001"].Freq == "999999999" {
		t.Error("mutating one ImageUK() call's map affected a second call's map — images must be independent")
	}
}

func TestNew_DefaultsToImageUK(t *testing.T) {
	r, conn := newTestRadio(t)
	_ = r
	writeFrame(t, conn, "MR001;")
	want := "MR001007000000+000000110000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("New() with no WithFactoryImage: MR001; -> %q, want %q (default should be ImageUK)", got, want)
	}
}

func TestWithFactoryImage_US(t *testing.T) {
	_, conn := newTestRadio(t, WithFactoryImage(ImageUS))
	writeFrame(t, conn, "MREMG;")
	want := "MREMG005167500+000000210000;" // USB(2), kind Memory-like(1) per kind60mEMG
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MREMG; on a US-image radio -> %q, want %q", got, want)
	}
}

func TestWithFactoryImage_UK_EMGEmpty(t *testing.T) {
	_, conn := newTestRadio(t, WithFactoryImage(ImageUK))
	writeFrame(t, conn, "MREMG;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("MREMG; on a UK-image radio -> %q, want %q", got, want)
	}
}

func TestWithSlot_OverlaysOntoDefaultImage(t *testing.T) {
	custom := MemState{
		Freq: "003700000", ClarSign: '+', ClarMag: "0000",
		Mode: modeUSB, Kind: kindMemory, CTCSS: '0', Shift: '0', Populated: true,
	}
	_, conn := newTestRadio(t, WithSlot("050", custom))

	// The default M-01 must still be present (WithFactoryImage was not
	// given, so the ImageUK default still applies).
	writeFrame(t, conn, "MR001;")
	if got, want := mustReadFrame(t, conn), "MR001007000000+000000110000;"; got != want {
		t.Errorf("MR001; with WithSlot overlay present -> %q, want %q", got, want)
	}

	writeFrame(t, conn, "MR050;")
	if got, want := mustReadFrame(t, conn), "MR050003700000+000000210000;"; got != want {
		t.Errorf("MR050; (WithSlot overlay) -> %q, want %q", got, want)
	}
}
