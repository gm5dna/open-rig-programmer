// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestProfileConstructionPins(t *testing.T) {
	p := icr8600.Profile()
	if !p.Configured() {
		t.Fatal("Profile() is not configured")
	}
	if got := p.Model(); got != icr8600.Model {
		t.Errorf("Model() = %q, want %q", got, icr8600.Model)
	}
	if got := p.RadioAddress(); got != icr8600.RadioAddress {
		t.Errorf("RadioAddress() = %#02x, want %#02x", got, icr8600.RadioAddress)
	}
	if icr8600.ControllerAddress != 0xE0 {
		t.Errorf("ControllerAddress constant = %#02x, want 0xE0", icr8600.ControllerAddress)
	}
	if icr8600.ControllerAddress != civ.ControllerAddressDefault {
		t.Errorf("ControllerAddress constant = %#02x, want civ default %#02x", icr8600.ControllerAddress, civ.ControllerAddressDefault)
	}
	if got := p.ControllerAddress(); got != icr8600.ControllerAddress {
		t.Errorf("ControllerAddress() = %#02x, want default %#02x", got, icr8600.ControllerAddress)
	}
	if got := p.MaxFrame(); got != icr8600.MaxFrame || got < 55 {
		t.Errorf("MaxFrame() = %d, want %d and enough for the 55-byte full G witness", got, icr8600.MaxFrame)
	}
	if got := p.Discriminator(); got != civ.DiscriminatorModeByte {
		t.Errorf("Discriminator() = %v, want DiscriminatorModeByte", got)
	}
}

func TestNamePolicyPins(t *testing.T) {
	p := icr8600.Profile()
	if icr8600.NameLength != 16 {
		t.Errorf("NameLength constant = %d, want 16", icr8600.NameLength)
	}
	if got := p.NameLength(); got != icr8600.NameLength {
		t.Errorf("NameLength() = %d, want 16", got)
	}
	if icr8600.NamePad != ' ' {
		t.Errorf("NamePad constant = %#02x, want ASCII space", icr8600.NamePad)
	}
	if got := p.NamePad(); got != icr8600.NamePad {
		// ASSUMED: register icr8600-name-pad. Stage R lift: read a
		// short front-panel name and record every trailing field byte.
		t.Errorf("NamePad() = %#02x, want ASCII space — ASSUMED, icr8600-name-pad", got)
	}

	charset := p.NameCharset()
	if got, want := string(charset), icr8600.NameCharset; got != want {
		t.Fatalf("NameCharset() = %q, want exact printable ASCII %q", got, want)
	}
	if len(charset) != 95 {
		t.Fatalf("len(NameCharset()) = %d, want 95", len(charset))
	}
	for i, got := range charset {
		if want := byte(0x20 + i); got != want {
			// The guide prints glyphs but no codes. The byte codes are
			// ASSUMED under icr8600-name-charset-codes; Stage R lifts the
			// assumption by writing and reading representatives from every
			// printed character class.
			t.Fatalf("NameCharset()[%d] = %#02x, want %#02x — ASSUMED, icr8600-name-charset-codes", i, got, want)
		}
	}
	allowed := make(map[byte]bool, len(charset))
	for _, b := range charset {
		allowed[b] = true
	}
	if !allowed[';'] || !allowed['|'] {
		t.Error("NameCharset() must include both ';' and '|' from the printed character table")
	}

	caps := spec.Capabilities{TagCharset: icr8600.NameCharset}
	if !caps.TagByteOK(';') || !caps.TagByteOK('|') {
		t.Error("the stable capability TagCharset rejects ';' or '|'")
	}
}

func TestNameEncodingPadsAndNeverTruncates(t *testing.T) {
	p := icr8600.Profile()

	full := "A;|BCDEFGHIJKLMN"
	fullFrame, fullSpan := buildNamedSet(t, p, full)
	if got := fullFrame[fullSpan.start:fullSpan.end]; !bytes.Equal(got, []byte(full)) {
		t.Errorf("full-width encoded name = % X, want % X with no truncation or padding", got, []byte(full))
	}

	short := "A;|"
	shortFrame, shortSpan := buildNamedSet(t, p, short)
	wantShort := append([]byte(short), bytes.Repeat([]byte{' '}, icr8600.NameLength-len(short))...)
	if got := shortFrame[shortSpan.start:shortSpan.end]; !bytes.Equal(got, wantShort) {
		// ASSUMED: register icr8600-name-pad. Stage R lift: read a short
		// front-panel name and record the emitted tail.
		t.Errorf("short encoded name = % X, want % X — ASSUMED ASCII-space padding, icr8600-name-pad", got, wantShort)
	}

	rec := recordForName(t, p, strings.Repeat("X", icr8600.NameLength+1))
	cmd, err := p.BuildMemorySet(rec)
	if err == nil || !strings.Contains(err.Error(), "truncating") {
		t.Fatalf("BuildMemorySet(overlength name) error = %v, want explicit no-truncation failure", err)
	}
	if !cmd.IsZero() {
		t.Errorf("BuildMemorySet(overlength name) returned non-zero command % X", cmd.Bytes())
	}
}

func TestNameEncodingRejectsEveryByteOutsideTheDeclaredCharset(t *testing.T) {
	p := icr8600.Profile()
	for value := 0; value <= 0xFF; value++ {
		b := byte(value)
		if b >= 0x20 && b <= 0x7E {
			continue
		}
		t.Run(strings.ToUpper(hexByte(b)), func(t *testing.T) {
			rec := recordForName(t, p, string([]byte{b}))
			cmd, err := p.BuildMemorySet(rec)
			if err == nil || !strings.Contains(err.Error(), "charset") {
				t.Fatalf("BuildMemorySet(name byte %#02x) error = %v, want explicit charset failure", b, err)
			}
			if !cmd.IsZero() {
				t.Errorf("BuildMemorySet(name byte %#02x) returned non-zero command % X", b, cmd.Bytes())
			}
		})
	}
}

type byteSpan struct{ start, end int }

func buildNamedSet(t *testing.T, p civ.Profile, name string) ([]byte, byteSpan) {
	t.Helper()
	rec := recordForName(t, p, name)
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%q): %v", name, err)
	}
	frame := cmd.Bytes()
	layout := selectedLayout(t, p)
	recordStart := len(frame) - 1 - layout.Length
	for _, field := range layout.Fields {
		if field.Field == civ.FieldName {
			return frame, byteSpan{recordStart + field.Offset, recordStart + field.Offset + field.Length}
		}
	}
	t.Fatal("selected construction layout has no name field")
	return nil, byteSpan{}
}

func recordForName(t *testing.T, p civ.Profile, name string) civ.MemoryRecord {
	t.Helper()
	layout := selectedLayout(t, p)
	rec := civ.MemoryRecord{Address: civ.ChannelAddress{Group: 0, Channel: 0}}
	for _, field := range layout.Fields {
		switch field.Encoding {
		case civ.EncodingBCDNumber:
			setNumeric(t, &rec, field.Field, 0)
		case civ.EncodingEnum:
			value, ok := firstEnumValue(field, layout)
			if !ok {
				t.Fatalf("%s enum is empty", field.Field)
			}
			setText(t, &rec, field.Field, value)
		case civ.EncodingName:
			setText(t, &rec, field.Field, name)
		default:
			t.Fatalf("%s has unsupported encoding %v", field.Field, field.Encoding)
		}
	}
	return rec
}

func selectedLayout(t *testing.T, p civ.Profile) civ.RecordLayout {
	t.Helper()
	layouts := p.Layouts()
	if len(layouts) < 2 {
		t.Fatalf("Profile has %d layouts, want the mode-discriminator construction scaffold", len(layouts))
	}
	return layouts[0]
}

func firstEnumValue(field civ.FieldSpan, layout civ.RecordLayout) (string, bool) {
	if field.Field == civ.FieldMode && len(layout.ModeValues) > 0 {
		value, ok := field.Enum[layout.ModeValues[0]]
		return value, ok
	}
	for _, value := range field.Enum {
		return value, true
	}
	return "", false
}

func setNumeric(t *testing.T, rec *civ.MemoryRecord, field civ.FieldID, value uint64) {
	t.Helper()
	v := civ.Available(value)
	switch field {
	case civ.FieldRXFrequency:
		rec.RXFreqHz = v
	case civ.FieldTXFrequency:
		rec.TXFreqHz = v
	case civ.FieldOffset:
		rec.OffsetHz = v
	case civ.FieldToneTX:
		rec.ToneTXDeciHz = v
	case civ.FieldToneRX:
		rec.ToneRXDeciHz = v
	case civ.FieldDTCSCode:
		rec.DTCSCode = v
	case civ.FieldProgramTuningStep:
		rec.ProgramTuningStepHz = v
	case civ.FieldAttenuator:
		rec.AttenuatorDB = v
	default:
		t.Fatalf("unhandled numeric field %s", field)
	}
}

func setText(t *testing.T, rec *civ.MemoryRecord, field civ.FieldID, value string) {
	t.Helper()
	v := civ.Available(value)
	switch field {
	case civ.FieldDuplex:
		rec.Duplex = v
	case civ.FieldMode:
		rec.Mode = v
	case civ.FieldFilter:
		rec.Filter = v
	case civ.FieldDataMode:
		rec.DataMode = v
	case civ.FieldToneMode:
		rec.ToneMode = v
	case civ.FieldDTCSPolarity:
		rec.DTCSPolarity = v
	case civ.FieldName:
		rec.Name = v
	case civ.FieldSelect:
		rec.Select = v
	case civ.FieldTuningStepEnabled:
		rec.TuningStepEnabled = v
	case civ.FieldTuningStep:
		rec.TuningStep = v
	case civ.FieldPreamp:
		rec.Preamp = v
	case civ.FieldAntenna:
		rec.Antenna = v
	case civ.FieldIPPlus:
		rec.IPPlus = v
	default:
		t.Fatalf("unhandled text field %s", field)
	}
}

func hexByte(b byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&0x0F]})
}
