// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
)

// legalRecord is a record every field of which is inside what the printed
// diagrams admit: 50.125000 MHz is under the radio's 60 MHz receiver
// ceiling and inside the ④~⑦ digit domain, and both tones are on the
// printed 67.0–254.1 Hz chart.
//
// THE FIXTURE IS NOT ARBITRARY. Its predecessor was 145.5 MHz, which
// needs a `01` in frequency byte ⑧ — a byte matrix §3.16.3 prints as
// "1000 MHz digit: 0 (Fixed)" over "100 MHz digit: 0 (Fixed)". A geometry
// test that expected that byte to carry a digit was pinning the very
// defect it was meant to catch.
func legalRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:  civ.ChannelAddress{Channel: 1},
		RXFreqHz: civ.Available(uint64(50125000)), Mode: civ.Available("USB"), Filter: civ.Available("FIL2"),
		ToneMode: civ.Available("TONE"), ToneTXDeciHz: civ.Available(uint64(885)), ToneRXDeciHz: civ.Available(uint64(1000)),
		Name: civ.Available("ALPHA 1"),
	}
}

func TestGeometryAndFixedRegions(t *testing.T) {
	p := ic7851.Profile()
	cmd, err := p.BuildMemorySet(legalRecord())
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01,
		0x00,                   // ③ select memory, E6-unmapped, template zero
		0x00, 0x50, 0x12, 0x50, // ④~⑦ frequency, little-endian packed BCD
		0x00,       // ⑧ the printed "(Fixed)" zero pair (matrix §3.16.3)
		0x01, 0x02, // ⑨ mode USB, ⑩ filter FIL2
		0x01,       // ⑪ high nibble data mode (E6-unmapped, zero), low nibble TONE
		0x00,       // ⑫ the printed "Fixed digit: 0" pair (matrix §3.16.4)
		0x08, 0x85, // ⑬⑭ 88.5 Hz
		0x00,       // ⑮ the same fixed pair for the tone-squelch triple
		0x10, 0x00, // ⑯⑰ 100.0 Hz
		0x41, 0x4c, 0x50, 0x48, 0x41, 0x20, 0x31, 0x20, 0x20, 0x20, // ⑱~㉗
		0xfd,
	}
	if got := cmd.Bytes(); string(got) != string(want) {
		t.Errorf("BuildMemorySet = % X, want % X", got, want)
	}
	for _, b := range p.Layouts()[0].Fixed {
		if b != 0 {
			t.Fatalf("fixed template contains %#x", b)
		}
	}
	for _, sp := range p.Layouts()[0].Fields {
		if sp.Offset == ic7851.SelectNibbleOffset {
			t.Errorf("select byte is mapped by %s", sp.Field)
		}
	}
	if p.Layouts()[0].Fields[3].Nibble != civ.NibbleLow {
		t.Error("tone mode is not the low nibble")
	}
}

// TestFixedBytesLieUnderNoMappedSpan is F1's structural half.
//
// A byte the document prints as a fixed zero must not be covered by a
// mapped span, because civ.FieldSpan carries no numeric domain: a mapped
// BCD span will happily encode a digit into it and re-encode it
// identically at the gate, so the layout's Fixed template — which only
// supplies bytes no span maps — never gets a say. Excluding the byte from
// the span is what gives the template the byte back.
func TestFixedBytesLieUnderNoMappedSpan(t *testing.T) {
	layout := ic7851.Profile().Layouts()[0]
	covered := map[int]civ.FieldID{}
	for _, sp := range layout.Fields {
		for i := 0; i < sp.Length; i++ {
			covered[sp.Offset+i] = sp.Field
		}
	}
	for _, tc := range []struct {
		offset int
		what   string
	}{
		{ic7851.FreqFixedOffset, "frequency byte ⑧ (matrix §3.16.3: 1000 MHz and 100 MHz digits, both printed \"0 (Fixed)\")"},
		{ic7851.ToneTXFixedOffset, "repeater-tone byte ⑫ (matrix §3.16.4: two printed \"Fixed digit: 0\" nibbles)"},
		{ic7851.ToneRXFixedOffset, "tone-squelch byte ⑮ (the same printed fixed pair)"},
	} {
		if f, mapped := covered[tc.offset]; mapped {
			t.Errorf("record byte %d is covered by the %s span, but it is %s — a mapped span would encode a digit there", tc.offset, f, tc.what)
		}
		if layout.Fixed[tc.offset] != 0 {
			t.Errorf("the Fixed template carries %#02x at record byte %d, want 0", layout.Fixed[tc.offset], tc.offset)
		}
	}
}

// TestBuilderRefusesValuesNeedingAFixedByte is F1's builder half: a value
// that could only be written by putting a digit in a printed-fixed byte
// must not be buildable at all.
func TestBuilderRefusesValuesNeedingAFixedByte(t *testing.T) {
	p := ic7851.Profile()
	for _, tc := range []struct {
		name string
		rec  civ.MemoryRecord
	}{
		{"a frequency needing frequency byte ⑧", func() civ.MemoryRecord {
			r := legalRecord()
			r.RXFreqHz = civ.Available(uint64(145500000)) // 145.5 MHz: needs a 1 in the 100 MHz digit
			return r
		}()},
		{"a repeater tone needing tone byte ⑫", func() civ.MemoryRecord {
			r := legalRecord()
			r.ToneTXDeciHz = civ.Available(uint64(10000)) // 1000.0 Hz: needs a digit in ⑫
			return r
		}()},
		{"a tone-squelch frequency needing tone byte ⑮", func() civ.MemoryRecord {
			r := legalRecord()
			r.ToneRXDeciHz = civ.Available(uint64(10000))
			return r
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if cmd, err := p.BuildMemorySet(tc.rec); err == nil {
				t.Fatalf("BuildMemorySet admitted % X; a value that needs a printed-fixed byte must be refused", cmd.Bytes())
			}
		})
	}
}

// TestGateRefusesNonZeroFixedBytes is F1's gate half. Each case takes the
// frame the builder DOES produce and flips one printed-fixed byte; the
// gate must refuse every one of them, because its re-encode leg puts the
// template's zero back and the bytes then differ.
func TestGateRefusesNonZeroFixedBytes(t *testing.T) {
	p := ic7851.Profile()
	cmd, err := p.BuildMemorySet(legalRecord())
	if err != nil {
		t.Fatal(err)
	}
	base := cmd.Bytes()
	if !p.AllowedCommand(base) {
		t.Fatalf("the builder's own frame % X is refused by its own gate", base)
	}
	// The record starts after FE FE <to> <from> 1A 00 and the two
	// selector bytes, so record offset N is frame index N+8.
	const recordBase = 8
	for _, tc := range []struct {
		offset int
		name   string
	}{
		{ic7851.FreqFixedOffset, "frequency byte ⑧"},
		{ic7851.ToneTXFixedOffset, "repeater-tone byte ⑫"},
		{ic7851.ToneRXFixedOffset, "tone-squelch byte ⑮"},
	} {
		for _, v := range []byte{0x01, 0x10, 0x11, 0x99} {
			frame := append([]byte(nil), base...)
			frame[recordBase+tc.offset] = v
			if p.AllowedCommand(frame) {
				t.Errorf("the gate admitted a set carrying %#02x in %s: % X", v, tc.name, frame)
			}
		}
	}
}
