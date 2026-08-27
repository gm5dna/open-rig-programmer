// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"bytes"
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

// theSymbolBytes is the IC-7300's thirty-two symbol code points, written
// out so that a membership pin exists at all.
//
// WHY THE LIST IS HERE RATHER THAN PARSED FROM AN ARTEFACT. This model's B
// leg records only the CROSS-REFERENCE for the name field
// (values_verbatim = `See "• Codes for character entries"`), so there is
// no per-symbol code table in this package's evidence to parse. Matrix
// §3.9 quotes the three printed RANGES and the Symbols table's three
// printed ENDPOINTS — `! 21`, `~ 7E` and `@ 40` — and enumerates the
// thirty-two symbol GLYPHS without quoting the remaining twenty-nine
// codes. Taking those twenty-nine at their ASCII code points is a
// DERIVATION, registered as `ic7300-name-charset-symbol-codes` with lift
// `ic7300-name-symbol-readback` (open question B).
//
// So this table IS the register entry, stated as a test. The three printed
// endpoints are marked; everything else is the derivation, and a capture
// is what will settle it.
var theSymbolBytes = []struct {
	b       byte
	printed bool // the code, not merely the glyph, is printed in the manual
}{
	{0x21, true}, // ! — printed endpoint
	{0x22, false}, {0x23, false}, {0x24, false}, {0x25, false}, {0x26, false},
	{0x27, false}, {0x28, false}, {0x29, false}, {0x2A, false}, {0x2B, false},
	{0x2C, false}, {0x2D, false}, {0x2E, false}, {0x2F, false}, {0x3A, false},
	{0x3B, false}, {0x3C, false}, {0x3D, false}, {0x3E, false}, {0x3F, false},
	{0x40, true}, // @ — printed endpoint
	{0x5B, false}, {0x5C, false}, {0x5D, false}, {0x5E, false}, {0x5F, false},
	{0x60, false}, {0x7B, false}, {0x7C, false}, {0x7D, false},
	{0x7E, true}, // ~ — printed endpoint
}

// TestNameCharsetMembership pins the charset's CONTENTS, not merely its
// size.
//
// A cardinality pin (len == 95) is satisfied by any ninety-five bytes: a
// reviewer swapped 0x60 for 0x7F and this package stayed green, which
// means the byte D13 exists to flag on the sibling — and one of open
// question B's twenty-nine derived codes here — was unbound. Every byte is
// now named, and every byte that is NOT in the charset is refused, so a
// single-byte substitution in either direction goes red.
func TestNameCharsetMembership(t *testing.T) {
	p := ic7300.Profile()

	want := map[byte]string{}
	// The SPACE. ASSUMED, and the one member no printed table carries:
	// neither p.168 table has a row for it, while the same page's command
	// table says of a memory name "All characters are usable."
	// (D5 entry 3, space half — lift ic7300-name-space.)
	want[0x20] = "space, ASSUMED (D5 entry 3, lift ic7300-name-space)"
	// The three PRINTED RANGES, PDF p.168 verbatim: `0–9 30–39`,
	// `A–Z 41–5A`, `a-z 61–7A`.
	for b := byte(0x30); b <= 0x39; b++ {
		want[b] = "digit, printed range 30–39"
	}
	for b := byte(0x41); b <= 0x5A; b++ {
		want[b] = "upper-case letter, printed range 41–5A"
	}
	for b := byte(0x61); b <= 0x7A; b++ {
		want[b] = "lower-case letter, printed range 61–7A"
	}
	// The thirty-two symbols.
	printed := 0
	for _, s := range theSymbolBytes {
		if s.printed {
			printed++
			want[s.b] = "symbol, code PRINTED (Symbols table endpoint)"
			continue
		}
		want[s.b] = "symbol, code DERIVED at its ASCII point (ic7300-name-charset-symbol-codes)"
	}
	if len(theSymbolBytes) != 32 {
		t.Fatalf("the symbol table has %d entries, want 32 — matrix §3.9 enumerates thirty-two symbol glyphs", len(theSymbolBytes))
	}
	if printed != 3 {
		t.Errorf("%d symbol codes are marked printed, want 3 — the Symbols table quotes `! 21`, `~ 7E` and `@ 40` and no other code, which is why the remaining twenty-nine are a DERIVATION and not a transcription", printed)
	}
	if derived := len(theSymbolBytes) - printed; derived != 29 {
		t.Errorf("%d symbol codes are derived, want 29 (32 glyphs less the 3 printed endpoints) — this is the arithmetic open question B rests on", derived)
	}
	if len(want) != 95 {
		t.Fatalf("the expected charset has %d bytes, want 95 — 1 space + 10 digits + 52 letters + 32 symbols", len(want))
	}

	got := map[byte]bool{}
	for _, b := range p.NameCharset() {
		if got[b] {
			t.Errorf("NameCharset() repeats %#02x", b)
		}
		got[b] = true
	}
	for b, why := range want {
		if !got[b] {
			t.Errorf("NameCharset() is MISSING %#02x (%s)", b, why)
		}
	}
	for b := range got {
		if _, ok := want[b]; !ok {
			t.Errorf("NameCharset() carries %#02x, which this model's evidence does not put in a memory name — every member is a printed range, a printed symbol endpoint, a derived symbol code point, or the ASSUMED space, and nothing else", b)
		}
	}

	// The two bytes the reviewer's mutation moved between, named so the
	// intent survives a later edit of the table above.
	if !got[0x60] {
		t.Error("0x60 is not in NameCharset() — it is one of the twenty-nine DERIVED symbol codes, and on the sibling it is the byte D13 exists to flag (the MK2's p.18 Symbols table draws the same glyph against 27 and 60)")
	}
	if got[0x7F] {
		t.Error("0x7F is in NameCharset() — DEL is not a glyph, appears in no printed table, and could only have arrived by an edit nothing was checking")
	}

	// The pad must be a member; civ's V4 requires it, and the profile
	// would not have constructed otherwise. Asserted anyway, because it is
	// the one member that is BOTH data and fill.
	if !got[p.NamePad()] {
		t.Errorf("NamePad() = %#02x is not in NameCharset()", p.NamePad())
	}
}

// TestToneSpansCarryTheirOwnByteOrder binds each tone span's BYTE ORDER at
// the codec layer, with a value that cannot hide a reversal.
//
// The golden vector alone cannot do this job on either model. This model's
// vector sets ToneTX == ToneRX == 885, so it cannot separate the two
// spans from each other; the sibling's sets its tone squelch to 1000,
// whose packed BCD 00 10 00 is a PALINDROME, so a reversed span encodes
// identically. Neither is a defect in the vectors — a G leg chooses sample
// values for other reasons — but a byte order nothing binds is a byte
// order a later edit can flip silently.
//
// So: two DIFFERENT values, each ASYMMETRIC under reversal, asserted as
// wire bytes and then round-tripped.
func TestToneSpansCarryTheirOwnByteOrder(t *testing.T) {
	p := ic7300.Profile()

	// 885 packs as 00 08 85 and reverses to 85 08 00; 1234 packs as
	// 00 12 34 and reverses to 34 12 00. Both differ from their own
	// reversal AND from each other.
	const toneTX, toneRX = 885, 1234
	rec := civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 1},
		RXFreqHz:     civ.Available[uint64](14_250_000),
		TXFreqHz:     civ.Available[uint64](14_250_000),
		ToneTXDeciHz: civ.Available[uint64](toneTX),
		ToneRXDeciHz: civ.Available[uint64](toneRX),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		Name:         civ.Available("PROBE"),
		Select:       civ.Available("OFF"),
	}

	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	record := frame[8 : len(frame)-1]
	if len(record) != 39 {
		t.Fatalf("the probe's record is %d bytes, want 39", len(record))
	}

	// ⑫–⑭ at offset 9 and ⑮–⑰ at offset 12, MOST SIGNIFICANT PAIR FIRST,
	// and the same two spans mirrored into the duplicated TX block at 23
	// and 26.
	for _, span := range []struct {
		what string
		off  int
		want []byte
	}{
		{"⑫–⑭ (tone_tx)", 9, []byte{0x00, 0x08, 0x85}},
		{"⑮–⑰ (tone_rx)", 12, []byte{0x00, 0x12, 0x34}},
		{"⓬–⓮ (tone_tx, TX block)", 23, []byte{0x00, 0x08, 0x85}},
		{"⓯–⓱ (tone_rx, TX block)", 26, []byte{0x00, 0x12, 0x34}},
	} {
		got := record[span.off : span.off+3]
		if !bytes.Equal(got, span.want) {
			rev := []byte{span.want[2], span.want[1], span.want[0]}
			t.Errorf("%s at record offset %d is % X, want % X — packed BCD, MOST significant pair first. % X would be the same value with the span's byte order reversed",
				span.what, span.off, got, span.want, rev)
		}
	}

	// And the round trip, so the decode direction is bound too.
	answer := make([]byte, 0, len(frame))
	answer = append(answer, 0xFE, 0xFE, 0xE0, 0x94, 0x1A, 0x00, frame[6], frame[7])
	answer = append(answer, record...)
	answer = append(answer, 0xFD)
	back, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer over the probe: %v", err)
	}
	if back != rec {
		t.Errorf("the probe did not round-trip:\n got %+v\nwant %+v", back, rec)
	}
	if toneTX == toneRX {
		t.Error("the probe's two tone values are equal — it would then not distinguish the two spans from each other")
	}
}
