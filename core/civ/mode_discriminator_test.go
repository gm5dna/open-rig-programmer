// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func modeProfileConfig() ProfileConfig {
	head := func(enum map[byte]string) []FieldSpan {
		return []FieldSpan{
			{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
			{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: enum},
		}
	}
	return ProfileConfig{
		Model:         "TEST-MODE-BYTE",
		RadioAddress:  0x96,
		AddressForm:   AddressFormFlat,
		ChannelLo:     0,
		ChannelHi:     99,
		Discriminator: DiscriminatorModeByte,
		ModeKey:       FieldSpan{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum},
		BuildLength:   0,
		Layouts: []RecordLayout{
			{Length: 6, ModeClass: "NONE", ModeValues: []byte{0x00}, Fields: head(map[byte]string{0x00: "AM"})},
			{Length: 8, ModeClass: "FM", ModeValues: []byte{0x01}, Fields: append(head(map[byte]string{0x01: "FM"}), FieldSpan{Field: FieldFilter, Offset: 6, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0: "W"}})},
			{Length: 8, ModeClass: "DCR", ModeValues: []byte{0x02}, Fields: append(head(map[byte]string{0x02: "DCR"}), FieldSpan{Field: FieldDataMode, Offset: 7, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0: "OFF"}})},
		},
	}
}

func TestModeByteLayoutSelectionAndLengths(t *testing.T) {
	p := MustNewProfile(modeProfileConfig())
	if got := p.RecordLengths(); !reflect.DeepEqual(got, []int{6, 8}) {
		t.Fatalf("RecordLengths() = %v, want deduplicated [6 8]", got)
	}
	if !p.AcceptsRecordLength(6) || !p.AcceptsRecordLength(8) || p.AcceptsRecordLength(7) {
		t.Errorf("AcceptsRecordLength does not describe the deduplicated set %v", p.RecordLengths())
	}
	if _, ok := p.LayoutFor(8); ok {
		t.Error("LayoutFor(8) selected a mode-keyed layout even though length 8 names two layouts")
	}
	if got := p.BuildRecordLength(); got != 0 {
		t.Errorf("BuildRecordLength() = %d, want 0 for a mode-keyed profile", got)
	}
	for mode, want := range map[string]int{"AM": 6, "FM": 8, "DCR": 8, "undeclared": 0} {
		if got := p.BuildRecordLengthFor(mode); got != want {
			t.Errorf("BuildRecordLengthFor(%q) = %d, want %d", mode, got, want)
		}
	}

	fm, err := p.LayoutForRecord([]byte{0, 0, 0, 0, 0, 0x01, 0, 0})
	if err != nil || fm.ModeClass != "FM" {
		t.Fatalf("LayoutForRecord(FM) = %+v, %v", fm, err)
	}
	dcr, err := p.LayoutForRecord([]byte{0, 0, 0, 0, 0, 0x02, 0, 0})
	if err != nil || dcr.ModeClass != "DCR" {
		t.Fatalf("LayoutForRecord(DCR) = %+v, %v", dcr, err)
	}
	if fm.Fields[len(fm.Fields)-1].Field == dcr.Fields[len(dcr.Fields)-1].Field {
		t.Fatal("same-length mode layouts did not preserve their disagreeing tails")
	}
}

func TestModeByteBuilderParserAndGateSelectTheDeclaredLayout(t *testing.T) {
	p := MustNewProfile(modeProfileConfig())
	tests := []struct {
		name string
		rec  MemoryRecord
		want int
	}{
		{"head only", MemoryRecord{Address: ChannelAddress{Channel: 0}, RXFreqHz: Available(uint64(145000000)), Mode: Available("AM")}, 6},
		{"FM tail", MemoryRecord{Address: ChannelAddress{Channel: 1}, RXFreqHz: Available(uint64(145000000)), Mode: Available("FM"), Filter: Available("W")}, 8},
		{"DCR tail", MemoryRecord{Address: ChannelAddress{Channel: 2}, RXFreqHz: Available(uint64(145000000)), Mode: Available("DCR"), DataMode: Available("OFF")}, 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := p.BuildMemorySet(tc.rec)
			if err != nil {
				t.Fatalf("BuildMemorySet: %v", err)
			}
			frame := cmd.Bytes()
			if got := len(frame) - 9; got != tc.want { // 7-byte envelope plus 2-byte flat address
				t.Fatalf("record length = %d, want %d", got, tc.want)
			}
			if !p.AllowedCommand(frame) {
				t.Fatal("gate refused a mode-keyed builder output")
			}
			answer := append([]byte(nil), frame...)
			answer[2], answer[3] = answer[3], answer[2]
			back, err := p.ParseMemoryAnswer(answer)
			if err != nil {
				t.Fatalf("ParseMemoryAnswer: %v", err)
			}
			if back != tc.rec {
				t.Errorf("round trip = %+v, want %+v", back, tc.rec)
			}
		})
	}
}

func TestModeByteRefusesUndeclaredModeAndModeLengthMismatch(t *testing.T) {
	p := MustNewProfile(modeProfileConfig())
	undeclared := []byte{0, 0, 0, 0, 0, 0x03, 0, 0}
	if _, err := p.LayoutForRecord(undeclared); err == nil || !strings.Contains(err.Error(), "undeclared mode") {
		t.Fatalf("LayoutForRecord(undeclared) error = %v, want undeclared mode", err)
	}

	mismatch := []byte{0, 0, 0, 0, 0, 0x00, 0, 0}
	_, err := p.LayoutForRecord(mismatch)
	var rle *RecordLengthError
	if !errors.As(err, &rle) || rle.Mode != "NONE" || !reflect.DeepEqual(rle.Want, []int{6}) || rle.Got != 8 {
		t.Fatalf("LayoutForRecord(mode/length mismatch) error = %#v, want mode NONE length 6 got 8", err)
	}
	if !strings.Contains(err.Error(), "mode NONE") {
		t.Errorf("RecordLengthError text = %q, want mode NONE", err)
	}

	read, err := p.BuildMemoryRead(ChannelAddress{Channel: 0})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	setFor := func(record []byte) []byte {
		frame := read.Bytes()
		return append(append(frame[:len(frame)-1:len(frame)-1], record...), EndByte)
	}
	if p.AllowedCommand(setFor(undeclared)) {
		t.Error("AllowedCommand admitted an undeclared mode")
	}
	if p.AllowedCommand(setFor(mismatch)) {
		t.Error("AllowedCommand admitted a mode/length mismatch")
	}
	answer := setFor(undeclared)
	answer[2], answer[3] = answer[3], answer[2]
	if _, raw, err := p.MemoryAnswerRecord(answer); err != nil || !reflect.DeepEqual(raw, undeclared) {
		t.Fatalf("MemoryAnswerRecord changed its raw contract: raw=% X err=%v", raw, err)
	}
	if _, err := p.ParseMemoryAnswer(answer); err == nil || !strings.Contains(err.Error(), "undeclared mode") {
		t.Fatalf("ParseMemoryAnswer(undeclared mode) error = %v", err)
	}
}

func TestModeByteValidatorNamesEachRule(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ProfileConfig)
		want string
	}{
		{"at least two layouts", func(c *ProfileConfig) { c.Layouts = c.Layouts[:1] }, "at least 2 layouts"},
		{"class required", func(c *ProfileConfig) { c.Layouts[1].ModeClass = "" }, "ModeClass is empty"},
		{"values required", func(c *ProfileConfig) { c.Layouts[1].ModeValues = nil }, "ModeValues is empty"},
		{"class unique", func(c *ProfileConfig) { c.Layouts[2].ModeClass = "FM" }, "ModeClass \"FM\" repeats"},
		{"wire value unique", func(c *ProfileConfig) {
			c.Layouts[2].ModeValues = []byte{0x01}
			c.Layouts[2].Fields[1].Enum = map[byte]string{0x01: "DCR"}
		}, "wire value 0x01 repeats"},
		{"neutral mode unique", func(c *ProfileConfig) { c.Layouts[2].Fields[1].Enum = map[byte]string{0x02: "FM"} }, "neutral mode \"FM\" repeats"},
		{"mode key mapped", func(c *ProfileConfig) {
			c.Layouts[1].Fields = append(c.Layouts[1].Fields[:1], c.Layouts[1].Fields[2:]...)
		}, "does not map ModeKey"},
		{"mode domain exact", func(c *ProfileConfig) { c.Layouts[1].ModeValues = []byte{0x01, 0x03} }, "domain does not equal ModeValues"},
		{"head span mismatch", func(c *ProfileConfig) { c.Layouts[1].Fields[0].Scale = 10 }, "head field spans differ"},
		{"head fixed mismatch", func(c *ProfileConfig) { c.Layouts[1].Fixed = make([]byte, 8); c.Layouts[1].Fixed[4] = 1 }, "head Fixed bytes differ"},
		{"mode build length zero", func(c *ProfileConfig) { c.BuildLength = 6 }, "BuildLength is 6, want 0"},
		{"length build length nonzero", func(c *ProfileConfig) {
			c.Discriminator = DiscriminatorRecordLength
			c.Layouts[2].Length = 9
			c.BuildLength = 0
		}, "BuildLength is 0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := modeProfileConfig()
			tc.edit(&cfg)
			_, err := NewProfile(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("NewProfile error = %v, want named failure containing %q", err, tc.want)
			}
		})
	}
}
