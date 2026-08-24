// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
)

func TestProfileNumbers(t *testing.T) {
	p := ic7300.Profile()
	if !p.Configured() {
		t.Fatal("Profile() is not Configured — MustNewProfile built an inert profile")
	}
	if got := p.Model(); got != "IC-7300" {
		t.Errorf("Model() = %q, want %q", got, "IC-7300")
	}
	if got := p.RadioAddress(); got != 0x94 {
		t.Errorf("RadioAddress() = %#02x, want 0x94 (matrix §3.4, PDF p.126 CI-V Address Default: 94h)", got)
	}
	if got := p.ControllerAddress(); got != 0xE0 {
		t.Errorf("ControllerAddress() = %#02x, want 0xE0", got)
	}
	// Spec Erratum 1: profiles carry RECORD-ONLY lengths. 39 record bytes,
	// 41 data-area bytes, 2-byte channel address. A fingerprint built on 41
	// is a different test.
	if got := p.RecordLengths(); len(got) != 1 || got[0] != 39 {
		t.Errorf("RecordLengths() = %v, want [39] (record-only; the data area is 41 = 39 + the 2-byte channel address)", got)
	}
	if got := p.BuildRecordLength(); got != 39 {
		t.Errorf("BuildRecordLength() = %d, want 39", got)
	}
	if p.AcceptsRecordLength(41) {
		t.Error("AcceptsRecordLength(41) = true — 41 is the DATA-AREA figure, not the record-only length the profile carries")
	}
	if p.AcceptsRecordLength(45) {
		t.Error("AcceptsRecordLength(45) = true — 45 is the IC-7300MK2's record-only length, and this profile must refuse it")
	}
	if got := p.NameLength(); got != 10 {
		t.Errorf("NameLength() = %d, want 10 (matrix §3.9, PDF p.169 \"Up to 10 characters.\")", got)
	}
	if got := p.NamePad(); got != 0x20 {
		t.Errorf("NamePad() = %#02x, want 0x20 (D5 entry 3, pad half, ASSUMED — lift ic7300-name-pad)", got)
	}
	if got := len(p.NameCharset()); got != 95 {
		t.Errorf("len(NameCharset()) = %d, want 95 (space + 10 digits + 52 letters + 32 symbols)", got)
	}
	if got := p.MaxFrame(); got != 64 {
		t.Errorf("MaxFrame() = %d, want 64 — the bound must admit the IC-7300MK2's 54-byte answer so a foreign record fails as a RecordLengthError, not an ErrFrameTooLong", got)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 101 {
		t.Errorf("ChannelRange() = (%d, %d), want (1, 101): 1..99 = M-CH01..99, 100 = P1 (wire 01 00), 101 = P2 (wire 01 01)", lo, hi)
	}
	if got := p.AddressForm(); got != civ.AddressFormFlat {
		t.Errorf("AddressForm() = %v, want AddressFormFlat", got)
	}
	if got := p.Discriminator(); got != civ.DiscriminatorSingleLength {
		t.Errorf("Discriminator() = %v, want DiscriminatorSingleLength — this model documents no conditional field width", got)
	}
}

func TestLayoutFieldOrderAndWidths(t *testing.T) {
	layout, ok := ic7300.Profile().LayoutFor(39)
	if !ok {
		t.Fatal("LayoutFor(39) missing")
	}
	type want struct {
		field  civ.FieldID
		offset int
		length int
		nibble civ.NibbleSel
	}
	wants := []want{
		{civ.FieldSelect, 0, 1, civ.NibbleLow}, // D14: LOW nibble only; the high nibble is the unmapped split flag
		{civ.FieldRXFrequency, 1, 5, civ.NibbleWhole},
		{civ.FieldMode, 6, 1, civ.NibbleWhole},
		{civ.FieldFilter, 7, 1, civ.NibbleWhole},
		{civ.FieldDataMode, 8, 1, civ.NibbleHigh},
		{civ.FieldToneMode, 8, 1, civ.NibbleLow},
		{civ.FieldToneTX, 9, 3, civ.NibbleWhole},
		{civ.FieldToneRX, 12, 3, civ.NibbleWhole},
		{civ.FieldTXFrequency, 15, 5, civ.NibbleWhole},
		{civ.FieldMode, 20, 1, civ.NibbleWhole},
		{civ.FieldFilter, 21, 1, civ.NibbleWhole},
		{civ.FieldDataMode, 22, 1, civ.NibbleHigh},
		{civ.FieldToneMode, 22, 1, civ.NibbleLow},
		{civ.FieldToneTX, 23, 3, civ.NibbleWhole},
		{civ.FieldToneRX, 26, 3, civ.NibbleWhole},
		{civ.FieldName, 29, 10, civ.NibbleWhole},
	}
	if len(layout.Fields) != len(wants) {
		t.Fatalf("layout has %d spans, want %d", len(layout.Fields), len(wants))
	}
	for i, w := range wants {
		got := layout.Fields[i]
		if got.Field != w.field || got.Offset != w.offset || got.Length != w.length || got.Nibble != w.nibble {
			t.Errorf("span %d = {%s off=%d len=%d nib=%v}, want {%s off=%d len=%d nib=%v}",
				i, got.Field, got.Offset, got.Length, got.Nibble, w.field, w.offset, w.length, w.nibble)
		}
	}
	// D14/E6: the Fixed template is full-length and all zero, and its ONLY
	// job is to declare byte 0's high nibble — the split flag — as unmapped.
	if len(layout.Fixed) != 39 {
		t.Fatalf("len(Fixed) = %d, want 39 — V8 permits an EMPTY template or one of exactly Length bytes, and this profile needs a template to declare the one unmapped nibble", len(layout.Fixed))
	}
	for i, b := range layout.Fixed {
		if b != 0x00 {
			t.Errorf("Fixed[%d] = %#02x, want 0x00 — every mapped nibble must be zero in the template (V8), and the one unmapped nibble is the split flag OFF", i, b)
		}
	}
}

// TestSplitNibbleIsUnmapped is E6's premise, pinned. If a later edit maps the
// high nibble, the driver's unmapped-region check silently becomes a no-op.
func TestSplitNibbleIsUnmapped(t *testing.T) {
	layout, _ := ic7300.Profile().LayoutFor(39)
	for _, sp := range layout.Fields {
		if sp.Offset == 0 && sp.Nibble != civ.NibbleLow {
			t.Errorf("a span at offset 0 has Nibble %v — byte ③'s HIGH nibble is the split flag and must stay UNMAPPED under the Fixed template (enablers E6; plan D14)", sp.Nibble)
		}
	}
}
