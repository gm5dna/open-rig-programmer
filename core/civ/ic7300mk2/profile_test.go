// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
)

func TestProfileNumbers(t *testing.T) {
	p := ic7300mk2.Profile()
	if !p.Configured() {
		t.Fatal("Profile() is not Configured — MustNewProfile built an inert profile")
	}
	if got := p.Model(); got != "IC-7300MK2" {
		t.Errorf("Model() = %q, want %q", got, "IC-7300MK2")
	}
	if got := p.RadioAddress(); got != 0xB6 {
		t.Errorf("RadioAddress() = %#02x, want 0xB6 (matrix §3.4, PDF p.3 index ② \"Transceiver's default address\")", got)
	}
	if got := p.ControllerAddress(); got != 0xE0 {
		t.Errorf("ControllerAddress() = %#02x, want 0xE0", got)
	}
	// Spec Erratum 1: RECORD-ONLY. 45 record bytes, 47 data-area bytes,
	// 2-byte channel address.
	if got := p.RecordLengths(); len(got) != 1 || got[0] != 45 {
		t.Errorf("RecordLengths() = %v, want [45] (record-only; the data area is 47 = 45 + the 2-byte channel address)", got)
	}
	if got := p.BuildRecordLength(); got != 45 {
		t.Errorf("BuildRecordLength() = %d, want 45", got)
	}
	if p.AcceptsRecordLength(47) {
		t.Error("AcceptsRecordLength(47) = true — 47 is the DATA-AREA figure, not the record-only length")
	}
	if p.AcceptsRecordLength(39) {
		t.Error("AcceptsRecordLength(39) = true — 39 is the IC-7300's record-only length, and this profile must refuse it")
	}
	if got := p.NameLength(); got != 16 {
		t.Errorf("NameLength() = %d, want 16 (matrix §1 #5: three printed statements)", got)
	}
	if got := p.NamePad(); got != 0x20 {
		t.Errorf("NamePad() = %#02x, want 0x20 (D5 entry 3, pad half, ASSUMED — lift MK2-W3)", got)
	}
	if got := len(p.NameCharset()); got != 95 {
		t.Errorf("len(NameCharset()) = %d, want 95", got)
	}
	if got := p.MaxFrame(); got != 64 {
		t.Errorf("MaxFrame() = %d, want 64 — the bound must admit the IC-7300's 48-byte answer so a foreign record fails as a RecordLengthError, not an ErrFrameTooLong", got)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 101 {
		t.Errorf("ChannelRange() = (%d, %d), want (1, 101)", lo, hi)
	}
	if got := p.AddressForm(); got != civ.AddressFormFlat {
		t.Errorf("AddressForm() = %v, want AddressFormFlat", got)
	}
	if got := p.Discriminator(); got != civ.DiscriminatorSingleLength {
		t.Errorf("Discriminator() = %v, want DiscriminatorSingleLength", got)
	}
}

func TestLayoutFieldOrderAndWidths(t *testing.T) {
	layout, ok := ic7300mk2.Profile().LayoutFor(45)
	if !ok {
		t.Fatal("LayoutFor(45) missing")
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
		{civ.FieldName, 29, 16, civ.NibbleWhole},
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
	if len(layout.Fixed) != 45 {
		t.Fatalf("len(Fixed) = %d, want 45 — V8 permits an EMPTY template or one of exactly Length bytes, and this profile needs a template to declare the one unmapped nibble", len(layout.Fixed))
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
	layout, _ := ic7300mk2.Profile().LayoutFor(45)
	for _, sp := range layout.Fields {
		if sp.Offset == 0 && sp.Nibble != civ.NibbleLow {
			t.Errorf("a span at offset 0 has Nibble %v — byte ③'s HIGH nibble is the split flag and must stay UNMAPPED under the Fixed template (enablers E6; plan D14)", sp.Nibble)
		}
	}
}
