// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7600"
)

// TestProfileShape pins the values doc.go's table names, and pins BOTH
// length conventions on purpose (spec Erratum 1): a profile carries the
// RECORD-ONLY length, and the data-area figure is what the matrix's own
// S3.11 arithmetic produced.
func TestProfileShape(t *testing.T) {
	p := ic7600.Profile()

	if got, want := p.Model(), "IC-7600"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
	if got, want := p.RadioAddress(), byte(0x7A); got != want {
		t.Errorf("RadioAddress() = %#02x, want %#02x", got, want)
	}
	if got, want := p.ControllerAddress(), byte(0xE0); got != want {
		t.Errorf("ControllerAddress() = %#02x, want %#02x", got, want)
	}
	if got, want := p.AddressForm(), civ.AddressFormFlat; got != want {
		t.Errorf("AddressForm() = %v, want %v", got, want)
	}
	if got, want := p.Groups(), 0; got != want {
		t.Errorf("Groups() = %d, want %d", got, want)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 101 {
		t.Errorf("ChannelRange() = %d..%d, want 1..101 (99 memories, then P1 at 100 and P2 at 101)", lo, hi)
	}
	if got, want := p.NameLength(), 10; got != want {
		t.Errorf("NameLength() = %d, want %d", got, want)
	}
	if got, want := p.NamePad(), byte(0x20); got != want {
		t.Errorf("NamePad() = %#02x, want %#02x", got, want)
	}
	if got, want := p.Discriminator(), civ.DiscriminatorSingleLength; got != want {
		t.Errorf("Discriminator() = %v, want %v", got, want)
	}
	if got, want := p.BuildRecordLength(), 25; got != want {
		t.Errorf("BuildRecordLength() = %d, want %d (RECORD-ONLY, spec Erratum 1)", got, want)
	}
	if got := p.RecordLengths(); len(got) != 1 || got[0] != 25 {
		t.Errorf("RecordLengths() = %v, want [25]", got)
	}
	if ic7600.RecordOnlyLength != 25 || ic7600.DataAreaLength != 27 || ic7600.AddressBytes != 2 {
		t.Errorf("length constants are %d/%d/%d, want 25 record-only, 27 data-area, 2 address bytes",
			ic7600.RecordOnlyLength, ic7600.DataAreaLength, ic7600.AddressBytes)
	}
	if ic7600.RecordOnlyLength+ic7600.AddressBytes != ic7600.DataAreaLength {
		t.Error("the three length constants do not add up - the record-only figure plus the address width IS the data-area figure")
	}
}

// TestLayoutOffsetsFollowTheStatedRule binds "Offset = printed index - 3"
// to the code arithmetically, so a mis-stated offset fails here rather
// than merely at package init. The printed indices are literals (the
// evidence); the offsets are COMPUTED from them.
func TestLayoutOffsetsFollowTheStatedRule(t *testing.T) {
	const addressIndices = 2 // (1),(2) are the address, outside the record

	want := map[civ.FieldID]struct{ printedLo, printedHi int }{
		civ.FieldRXFrequency: {4, 8},
		civ.FieldMode:        {9, 9},
		civ.FieldFilter:      {10, 10},
		civ.FieldToneMode:    {11, 11},
		civ.FieldToneTX:      {12, 14},
		civ.FieldToneRX:      {15, 17},
		civ.FieldName:        {18, 27},
	}

	layouts := ic7600.Profile().Layouts()
	if len(layouts) != 1 {
		t.Fatalf("Layouts() has %d entries, want 1", len(layouts))
	}
	spans := layouts[0].Fields
	if len(spans) != len(want) {
		t.Fatalf("layout has %d spans, want %d - the two E6-unmapped regions carry NO span", len(spans), len(want))
	}
	for _, sp := range spans {
		w, ok := want[sp.Field]
		if !ok {
			t.Errorf("layout carries an unexpected span for %s", sp.Field)
			continue
		}
		wantOffset := w.printedLo - addressIndices - 1
		wantLength := w.printedHi - w.printedLo + 1
		if sp.Encoding == civ.EncodingEnum {
			wantLength = 1 // an enum span is one byte, whole or half
		}
		if sp.Offset != wantOffset {
			t.Errorf("%s: Offset = %d, want %d (printed index %d, minus %d address bytes, minus 1 for 0-based)",
				sp.Field, sp.Offset, wantOffset, w.printedLo, addressIndices)
		}
		if sp.Length != wantLength {
			t.Errorf("%s: Length = %d, want %d (printed indices %d..%d)", sp.Field, sp.Length, wantLength, w.printedLo, w.printedHi)
		}
	}
}

// TestUnmappedRegionsAreTheTwoE6Regions pins the E6 ruling's shape in the
// profile itself: byte 0 (printed (3)) is unmapped WHOLE - this radio's own
// page prints no nibble split for it, unlike the IC-7610's (profile.go's
// SelectByteOffset comment) - and byte 8's HIGH nibble (printed (11)'s left
// half) is the four-valued data mode. Neither has a faithful neutral home,
// so E6 rules them unmapped, carried by the Fixed template.
func TestUnmappedRegionsAreTheTwoE6Regions(t *testing.T) {
	fixed := ic7600.FixedTemplate()
	if len(fixed) != ic7600.RecordOnlyLength {
		t.Fatalf("FixedTemplate() is %d bytes, want %d - the template describes the whole record or none of it",
			len(fixed), ic7600.RecordOnlyLength)
	}
	for i, b := range fixed {
		if b != 0x00 {
			t.Errorf("FixedTemplate()[%d] = %#02x, want 0x00 - every unmapped region on this model is a printed or documented zero", i, b)
		}
	}
	if ic7600.SelectByteOffset != 0 || ic7600.DataModeNibbleOffset != 8 {
		t.Errorf("the E6-unmapped offsets are %d and %d, want 0 (printed (3)) and 8 (printed (11) high nibble)",
			ic7600.SelectByteOffset, ic7600.DataModeNibbleOffset)
	}

	// No span may claim either unmapped region.
	for _, sp := range ic7600.Profile().Layouts()[0].Fields {
		if sp.Offset == ic7600.SelectByteOffset {
			t.Errorf("%s claims record byte 0, which E6 rules unmapped on this model", sp.Field)
		}
		if sp.Offset == ic7600.DataModeNibbleOffset && sp.Nibble != civ.NibbleLow {
			t.Errorf("%s claims record byte 8 other than its LOW nibble; only tone_mode (low) is mapped there", sp.Field)
		}
	}
}

// TestChannelWireForms pins the three address forms the manual prints
// (matrix S1 row 5): memories 00 01 - 00 99, scan edge P1 at 01 00, P2 at
// 01 01.
func TestChannelWireForms(t *testing.T) {
	p := ic7600.Profile()
	tests := []struct {
		name    string
		channel int
		want    []byte
	}{
		{"memory 01", 1, []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFD}},
		{"memory 99", 99, []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, 0x00, 0x99, 0xFD}},
		{"scan edge P1", 100, []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0xFD}},
		{"scan edge P2", 101, []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, 0x01, 0x01, 0xFD}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: tc.channel})
			if err != nil {
				t.Fatalf("BuildMemoryRead(%d): %v", tc.channel, err)
			}
			if got := cmd.Bytes(); string(got) != string(tc.want) {
				t.Errorf("BuildMemoryRead(%d) = % X, want % X", tc.channel, got, tc.want)
			}
		})
	}
	for _, bad := range []int{0, 102, -1} {
		if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: bad}); err == nil {
			t.Errorf("BuildMemoryRead(%d) succeeded - the channel space is 1..101 and nothing else", bad)
		}
	}
	if _, err := p.BuildMemoryRead(civ.ChannelAddress{Group: 1, Channel: 1}); err == nil {
		t.Error("BuildMemoryRead with a group succeeded - a flat profile has nowhere to encode one")
	}
}

// TestNameCharsetIsTheIC7610Choice observes the charset profile.go adopted
// (a CHOICE, per that file's NameCharset comment) is exactly printable
// ASCII 0x20-0x7E - the same observation the IC-7610's own profile_test.go
// makes of its own charset, since this package's charset is that one,
// verbatim.
func TestNameCharsetIsTheIC7610Choice(t *testing.T) {
	got := map[byte]bool{}
	for i := 0; i < len(ic7600.NameCharset); i++ {
		b := ic7600.NameCharset[i]
		if got[b] {
			t.Errorf("charset carries %#02x twice - a duplicate is a transcription error", b)
		}
		got[b] = true
	}
	if len(got) != 95 {
		t.Errorf("charset has %d distinct bytes, want 95", len(got))
	}
	for b := 0x20; b <= 0x7E; b++ {
		if !got[byte(b)] {
			t.Errorf("charset omits %#02x (%q)", b, rune(b))
		}
	}
	for b := 0; b < 256; b++ {
		if (b < 0x20 || b > 0x7E) && got[byte(b)] {
			t.Errorf("charset carries %#02x, which is outside printable ASCII", b)
		}
	}
	// The default family rule EXCLUDES ';' (spec.Capabilities.TagByteOK),
	// because ';' terminates a NEWCAT frame. CI-V has no such terminator,
	// so the charset must be supplied explicitly rather than left to the
	// default - the IC-7610's own reasoning, inherited with the charset.
	if !got[';'] {
		t.Error("charset omits ';' - CI-V has no ';' terminator and the Yaesu default rule must not be inherited here")
	}
}

// TestConformance runs the tier's exported suite over this profile.
func TestConformance(t *testing.T) { civtest.Run(t, ic7600.Profile()) }

// TestZeroValueProfileRefusesEverything is the suite's fail-closed half.
func TestZeroValueProfileRefusesEverything(t *testing.T) { civtest.RunZeroValue(t) }
