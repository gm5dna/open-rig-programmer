// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
)

func TestRecordCommonHeadEncodesAndDecodesEveryMappedField(t *testing.T) {
	p := icr8600.Profile()
	rec := civ.MemoryRecord{
		Address:             civ.ChannelAddress{Group: 12, Channel: 34},
		Select:              civ.Available("SEL9"),
		RXFreqHz:            civ.Available(uint64(145_500_000)),
		Mode:                civ.Available("AM"),
		Filter:              civ.Available("FIL2"),
		Duplex:              civ.Available("DUP+"),
		OffsetHz:            civ.Available(uint64(123_456_000)),
		TuningStepEnabled:   civ.Available("ON"),
		TuningStep:          civ.Available("12.5 kHz"),
		ProgramTuningStepHz: civ.Available(uint64(987_600)),
		AttenuatorDB:        civ.Available(uint64(30)),
		Preamp:              civ.Available("ON"),
		Antenna:             civ.Available("ANT3"),
		IPPlus:              civ.Available("ON"),
		Name:                civ.Available("COMMON HEAD"),
	}

	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	wantRecord := []byte{
		0x09,
		0x00, 0x00, 0x50, 0x45, 0x01,
		0x02, 0x02, 0x02,
		0x60, 0x45, 0x23, 0x01,
		0x01, 0x10,
		// B's non-monotonic digit weights are 1 kHz, 100 Hz,
		// 100 kHz, 10 kHz, so 987.6 kHz is 76 98 rather than 98 76.
		0x76, 0x98,
		0x30, 0x01, 0x02, 0x01,
		'C', 'O', 'M', 'M', 'O', 'N', ' ', 'H', 'E', 'A', 'D', ' ', ' ', ' ', ' ', ' ',
	}
	gotRecord := frame[len(frame)-1-len(wantRecord) : len(frame)-1]
	if !bytes.Equal(gotRecord, wantRecord) {
		t.Fatalf("common-head record =\n% X\nwant\n% X", gotRecord, wantRecord)
	}

	answer := append([]byte(nil), frame...)
	answer[2], answer[3] = answer[3], answer[2]
	back, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if back != rec {
		t.Errorf("decode = %+v, want %+v", back, rec)
	}
}

func TestTailTemplatesAndFMToneFieldsEncodeEveryDeclaredClass(t *testing.T) {
	tests := []struct {
		name  string
		mode  string
		tail  []byte
		amend func(*civ.MemoryRecord)
	}{
		{"none", "AM", nil, nil},
		// D-STAR deliberately uses D.SQL wire code 2; code 1 is skipped
		// by the guide and must never be synthesised.
		{"d-star skipped code one", "D-STAR", []byte{0x02, 0x12}, nil},
		{"p25", "P25", []byte{0x01, 0x02, 0x09, 0x03}, nil},
		{"nxdn-vn", "NXDN-VN", []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00}, nil},
		{"nxdn-n", "NXDN-N", []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00}, nil},
		{"fm", "FM", []byte{0x01, 0x00, 0x08, 0x85, 0x00, 0x00, 0x23}, func(rec *civ.MemoryRecord) {
			rec.ToneMode = civ.Available("TSQL")
			rec.ToneRXDeciHz = civ.Available(uint64(885))
			rec.DTCSCode = civ.Available(uint64(23))
			rec.DTCSPolarity = civ.Available("Normal")
		}},
		{"dcr", "DCR", []byte{0x01, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00}, nil},
		{"dpmr", "dPMR", []byte{0x01, 0x01, 0x23, 0x12, 0x00, 0x00, 0x00, 0x00}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := commonRecord(tc.mode)
			if tc.amend != nil {
				tc.amend(&rec)
			}
			cmd, err := icr8600.Profile().BuildMemorySet(rec)
			if err != nil {
				t.Fatalf("BuildMemorySet: %v", err)
			}
			frame := cmd.Bytes()
			recordLength := icr8600.Profile().BuildRecordLengthFor(tc.mode)
			record := frame[len(frame)-1-recordLength : len(frame)-1]
			if got := record[37:]; !bytes.Equal(got, tc.tail) {
				t.Errorf("tail = % X, want % X", got, tc.tail)
			}

			answer := append([]byte(nil), frame...)
			answer[2], answer[3] = answer[3], answer[2]
			back, err := icr8600.Profile().ParseMemoryAnswer(answer)
			if err != nil {
				t.Fatalf("ParseMemoryAnswer: %v", err)
			}
			if back != rec {
				t.Errorf("round trip = %+v, want %+v", back, rec)
			}
		})
	}
}

func TestFMAndDCRUseModeNotTheirSharedLength(t *testing.T) {
	fm := commonRecord("FM")
	fm.ToneMode = civ.Available("DTCS")
	fm.ToneRXDeciHz = civ.Available(uint64(885))
	fm.DTCSCode = civ.Available(uint64(23))
	fm.DTCSPolarity = civ.Available("Reverse")
	fmRecord := builtRecord(t, fm)
	dcrRecord := builtRecord(t, commonRecord("DCR"))
	if len(fmRecord) != 44 || len(dcrRecord) != 44 {
		t.Fatalf("FM/DCR record lengths = %d/%d, want 44/44", len(fmRecord), len(dcrRecord))
	}
	fmLayout, err := icr8600.Profile().LayoutForRecord(fmRecord)
	if err != nil || fmLayout.ModeClass != "FM" {
		t.Fatalf("LayoutForRecord(FM) = %+v, %v", fmLayout, err)
	}
	dcrLayout, err := icr8600.Profile().LayoutForRecord(dcrRecord)
	if err != nil || dcrLayout.ModeClass != "DCR" {
		t.Fatalf("LayoutForRecord(DCR) = %+v, %v", dcrLayout, err)
	}
	if bytes.Equal(fmRecord[37:], dcrRecord[37:]) {
		t.Errorf("FM and DCR same-length tails unexpectedly agree: % X", fmRecord[37:])
	}
}

func commonRecord(mode string) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:             civ.ChannelAddress{Group: 0, Channel: 1},
		Select:              civ.Available("OFF"),
		RXFreqHz:            civ.Available(uint64(145_500_000)),
		Mode:                civ.Available(mode),
		Filter:              civ.Available("FIL1"),
		Duplex:              civ.Available("OFF"),
		OffsetHz:            civ.Available(uint64(0)),
		TuningStepEnabled:   civ.Available("ON"),
		TuningStep:          civ.Available("5 kHz"),
		ProgramTuningStepHz: civ.Available(uint64(9_000)),
		AttenuatorDB:        civ.Available(uint64(0)),
		Preamp:              civ.Available("ON"),
		Antenna:             civ.Available("ANT1"),
		IPPlus:              civ.Available("OFF"),
		Name:                civ.Available("TEST NAME"),
	}
}

func completeRecord(mode string) civ.MemoryRecord {
	rec := commonRecord(mode)
	if mode == "FM" {
		rec.ToneMode = civ.Available("OFF")
		rec.ToneRXDeciHz = civ.Available(uint64(0))
		rec.DTCSPolarity = civ.Available("Normal")
		rec.DTCSCode = civ.Available(uint64(0))
	}
	return rec
}

func builtRecord(t *testing.T, rec civ.MemoryRecord) []byte {
	t.Helper()
	cmd, err := icr8600.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%v): %v", rec.Mode, err)
	}
	frame := cmd.Bytes()
	length := icr8600.Profile().BuildRecordLengthFor(mustGet(rec.Mode))
	return append([]byte(nil), frame[len(frame)-1-length:len(frame)-1]...)
}

func mustGet[T comparable](value civ.Optional[T]) T {
	got, ok := value.Get()
	if !ok {
		panic("test optional is unavailable")
	}
	return got
}

func TestModeLayoutsSelectSevenTailsAndSixRecordLengths(t *testing.T) {
	p := icr8600.Profile()
	wantLayouts := []struct {
		class  string
		length int
		modes  map[string]byte
	}{
		{"NONE", 37, map[string]byte{
			"LSB": 0x00, "USB": 0x01, "AM": 0x02, "CW": 0x03,
			"FSK": 0x04, "WFM": 0x06, "CW-R": 0x07, "FSK-R": 0x08,
			"S-AM (D)": 0x11, "S-AM (L)": 0x14, "S-AM (U)": 0x15,
		}},
		{"D-STAR", 39, map[string]byte{"D-STAR": 0x17}},
		{"P25", 41, map[string]byte{"P25": 0x16}},
		{"NXDN", 43, map[string]byte{"NXDN-VN": 0x19, "NXDN-N": 0x20}},
		{"FM", 44, map[string]byte{"FM": 0x05}},
		{"DCR", 44, map[string]byte{"DCR": 0x21}},
		{"dPMR", 45, map[string]byte{"dPMR": 0x18}},
	}

	layouts := p.Layouts()
	if len(layouts) != len(wantLayouts) {
		t.Fatalf("len(Layouts()) = %d, want seven mode classes", len(layouts))
	}
	for i, want := range wantLayouts {
		got := layouts[i]
		if got.ModeClass != want.class || got.Length != want.length {
			t.Errorf("Layouts()[%d] = class %q length %d, want %q/%d", i, got.ModeClass, got.Length, want.class, want.length)
		}
		modeSpan := fieldSpan(t, got, civ.FieldMode)
		if modeSpan.Offset != 6 || modeSpan.Length != 1 || modeSpan.Encoding != civ.EncodingEnum {
			t.Errorf("Layouts()[%d] mode span = %+v, want record offset 6 whole-byte enum", i, modeSpan)
		}
		if len(got.ModeValues) != len(want.modes) {
			t.Errorf("Layouts()[%d].ModeValues = % X, want %d values", i, got.ModeValues, len(want.modes))
		}
		for name, wire := range want.modes {
			// ASSUMED: icr8600-mode-wire-codes. The guide prints two-digit
			// codes without saying BCD or binary; Stage R lifts this by
			// capturing DCR, FM and D-STAR mode bytes from an IC-R8600.
			if gotName := modeSpan.Enum[wire]; gotName != name {
				t.Errorf("Layouts()[%d] mode wire %#02x = %q, want %q — ASSUMED, icr8600-mode-wire-codes", i, wire, gotName, name)
			}
		}
	}

	if got, want := p.RecordLengths(), []int{37, 39, 41, 43, 44, 45}; !reflect.DeepEqual(got, want) {
		t.Errorf("RecordLengths() = %v, want deduplicated %v", got, want)
	}
	if got := p.BuildRecordLength(); got != 0 {
		t.Errorf("BuildRecordLength() = %d, want 0 for a mode-keyed profile", got)
	}
	for _, layout := range wantLayouts {
		for mode := range layout.modes {
			if got := p.BuildRecordLengthFor(mode); got != layout.length {
				t.Errorf("BuildRecordLengthFor(%q) = %d, want %d", mode, got, layout.length)
			}
		}
	}
	if got := p.BuildRecordLengthFor("undeclared"); got != 0 {
		t.Errorf("BuildRecordLengthFor(undeclared) = %d, want 0", got)
	}
	if _, ok := p.LayoutFor(44); ok {
		t.Error("LayoutFor(44) selected a mode-keyed layout even though FM and DCR share the length")
	}
}

func TestModeLayoutSelectionRefusesUndeclaredAndMismatchedRecords(t *testing.T) {
	p := icr8600.Profile()

	undeclared := make([]byte, 44)
	undeclared[6] = 0x09
	if _, err := p.LayoutForRecord(undeclared); err == nil || !strings.Contains(err.Error(), "undeclared mode") {
		t.Fatalf("LayoutForRecord(undeclared mode) error = %v, want undeclared-mode refusal", err)
	}

	mismatch := make([]byte, 44)
	mismatch[6] = 0x02 // AM selects the 37-byte NONE layout.
	_, err := p.LayoutForRecord(mismatch)
	var lengthErr *civ.RecordLengthError
	if !errors.As(err, &lengthErr) || lengthErr.Mode != "NONE" || lengthErr.Got != 44 || !reflect.DeepEqual(lengthErr.Want, []int{37}) {
		t.Fatalf("LayoutForRecord(mode/length mismatch) error = %#v, want NONE length 37 got 44", err)
	}
}

func fieldSpan(t *testing.T, layout civ.RecordLayout, id civ.FieldID) civ.FieldSpan {
	t.Helper()
	for _, span := range layout.Fields {
		if span.Field == id {
			return span
		}
	}
	t.Fatalf("layout %q has no %s span", layout.ModeClass, id)
	return civ.FieldSpan{}
}
