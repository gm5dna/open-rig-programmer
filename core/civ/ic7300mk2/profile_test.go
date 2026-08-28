// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"bytes"
	"strconv"
	"strings"
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

// charsetFromB parses the code table THIS MODEL'S B LEG TRANSCRIBED, out
// of the frozen artefact, and returns the byte set it declares.
//
// THE MK2'S EVIDENCE IS DIFFERENT IN KIND FROM ITS SIBLING'S, and that is
// why this test parses where the IC-7300's writes a table out. PDF p.18
// prints an ASCII code against every glyph and the B leg carried all
// thirty-two across, so nothing here is derived: `ic7300mk2-transcription-b.csv`'s
// `⑱ ~ ㉝` row IS the charset's provenance, and a membership pin that
// restated the bytes in Go would be checking the profile against a second
// copy of itself rather than against the evidence.
//
// The row's values_verbatim reads, in printed order, three RANGES
// (`A ~ Z: 41 ~ 5A`, `a ~ z: 61 ~ 7A`, `0 ~ 9: 30 ~ 39`), then thirty-two
// single `<glyph>: <code>` pairs, then the usable-characters ⓘ note, all
// joined by " | ". The GLYPHS are deliberately not read: two of them are
// the same shape (D13 — `27` and `60` are drawn alike), so only the codes
// are transcribable without a capture.
func charsetFromB(t *testing.T) (set map[byte]string, ranges, singles int) {
	t.Helper()
	var values string
	for _, r := range onlyD1(readCSV(t, "ic7300mk2-transcription-b.csv")) {
		if normaliseKey(r["field_index"]) == "18-33" {
			values = r["values_verbatim"]
		}
	}
	if values == "" {
		t.Fatal("B has no ⑱ ~ ㉝ row, or its values_verbatim is empty — that row is this charset's whole provenance")
	}

	hexByte := func(s string) byte {
		t.Helper()
		v, err := strconv.ParseUint(strings.TrimSpace(s), 16, 8)
		if err != nil {
			t.Fatalf("B's ⑱ ~ ㉝ row carries the code %q, which is not a hex byte: %v", s, err)
		}
		return byte(v)
	}

	set = map[byte]string{}
	add := func(b byte, why string) {
		t.Helper()
		if prev, dup := set[b]; dup {
			t.Errorf("B's ⑱ ~ ㉝ row declares %#02x twice (%s, then %s)", b, prev, why)
		}
		set[b] = why
	}

	// The trailing usable-characters note is PROSE about the same field and
	// carries no codes. It is cut off WHOLESALE at its own ⓘ marker rather
	// than skipped part by part, because the note itself prints the glyphs
	// `{ | } ~` — so it contains the very separator this row is joined by,
	// and splitting first would shatter it into fragments that look like
	// malformed pairs.
	if i := strings.Index(values, "ⓘ"); i >= 0 {
		values = values[:i]
	}

	for _, part := range strings.Split(values, " | ") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		i := strings.LastIndex(part, ": ")
		if i < 0 {
			t.Fatalf("B's ⑱ ~ ㉝ row has the fragment %q, which is neither a range nor a <glyph>: <code> pair", part)
		}
		spec := part[i+2:]
		if lo, hi, ok := strings.Cut(spec, " ~ "); ok {
			ranges++
			l, h := hexByte(lo), hexByte(hi)
			if l > h {
				t.Fatalf("B's ⑱ ~ ㉝ row has the range %q, whose low byte is above its high one", spec)
			}
			for b := int(l); b <= int(h); b++ {
				add(byte(b), "printed range "+spec)
			}
			continue
		}
		singles++
		add(hexByte(spec), "printed symbol code "+spec)
	}
	return set, ranges, singles
}

// TestNameCharsetMembership pins the charset's CONTENTS against the B leg,
// not merely its size.
//
// A cardinality pin (len == 95) is satisfied by any ninety-five bytes: a
// reviewer swapped 0x60 — the exact byte D13 exists to flag — for 0x7F and
// this package stayed green. Every byte is now bound to the artefact that
// declares it, and every byte the artefact does not declare is refused.
func TestNameCharsetMembership(t *testing.T) {
	p := ic7300mk2.Profile()

	want, ranges, singles := charsetFromB(t)
	if ranges != 3 {
		t.Errorf("B's ⑱ ~ ㉝ row declares %d code ranges, want 3 (A ~ Z, a ~ z, 0 ~ 9)", ranges)
	}
	if singles != 32 {
		t.Errorf("B's ⑱ ~ ㉝ row declares %d single symbol codes, want 32 — matrix Erratum 4 corrects §3.9's \"34 individually coded punctuation marks\" to thirty-two", singles)
	}
	if len(want) != 94 {
		t.Fatalf("B's ⑱ ~ ㉝ row declares %d distinct bytes, want 94 (10 digits + 52 letters + 32 symbols) — the SPACE is not among them and is added below", len(want))
	}

	// The SPACE is the one member the printed tables do NOT carry: p.18's
	// usable-characters note names "(space)" in terms while neither table
	// prints a row for it, and the byte 20 is printed twice in this
	// document for OTHER commands (17 and 1A 02) with their own charset
	// tables. ASSUMED — D5 entry 3's space half, lift MK2-R9.
	if _, printed := want[0x20]; printed {
		t.Error("B's ⑱ ~ ㉝ row declares 0x20 — if the document does print a space code for the memory name after all, D5 entry 3's space half is no longer ASSUMED on this model and the register entry must change")
	}
	want[0x20] = "space, ASSUMED (D5 entry 3, lift MK2-R9)"
	if len(want) != 95 {
		t.Fatalf("the expected charset has %d bytes, want 95", len(want))
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
			t.Errorf("NameCharset() carries %#02x, which B's ⑱ ~ ㉝ row does not declare — every member is a printed range, a printed symbol code, or the ASSUMED space, and nothing else", b)
		}
	}

	// D13's byte, named so the intent survives a later edit.
	if !got[0x60] {
		t.Error("0x60 is not in NameCharset() — PDF p.18's Symbols table prints it as a usable code, and D13 is that its GLYPH is unknown (the same shape is drawn against 27), NOT that the byte is inadmissible. Dropping it would silently narrow what a name may carry")
	}
	if got[0x7F] {
		t.Error("0x7F is in NameCharset() — DEL appears in no printed table and could only have arrived by an edit nothing was checking")
	}

	if !got[p.NamePad()] {
		t.Errorf("NamePad() = %#02x is not in NameCharset()", p.NamePad())
	}
}

// TestToneSpansCarryTheirOwnByteOrder binds each tone span's BYTE ORDER at
// the codec layer, with values that cannot hide a reversal.
//
// THIS TEST EXISTS BECAUSE THE GOLDEN VECTOR CANNOT DO THE JOB. This
// model's set vector holds a tone squelch of 1000 deciHz, whose packed BCD
// is 00 10 00 — a PALINDROME — so flipping ⑮ ~ ⑰ to little-endian encodes
// identically and every golden assertion stays green. That is a property
// of the G leg's chosen sample value, not a defect in the vector, and the
// fix belongs here rather than in a frozen artefact: no artefact is
// touched, and the probe uses two DIFFERENT values, each ASYMMETRIC under
// reversal.
//
// The ⑫ ~ ⑭ span carries a second reason to be bound: its heading is
// printed with nothing beneath it (§3.16 A6), so its encoding is ASSUMED
// from p.23 (ic7300mk2-tone-tx-encoding, lift MK2-R17). An assumption
// nothing pins is one a later edit can change without a test noticing.
func TestToneSpansCarryTheirOwnByteOrder(t *testing.T) {
	p := ic7300mk2.Profile()

	// 885 packs as 00 08 85 and reverses to 85 08 00; 1234 packs as
	// 00 12 34 and reverses to 34 12 00.
	const toneTX, toneRX = 885, 1234
	rec := civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 1},
		RXFreqHz:     civ.Available[uint64](14_100_000),
		TXFreqHz:     civ.Available[uint64](14_100_000),
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
	if len(record) != 45 {
		t.Fatalf("the probe's record is %d bytes, want 45", len(record))
	}

	for _, span := range []struct {
		what string
		off  int
		want []byte
	}{
		{"⑫ ~ ⑭ (tone_tx)", 9, []byte{0x00, 0x08, 0x85}},
		{"⑮ ~ ⑰ (tone_rx)", 12, []byte{0x00, 0x12, 0x34}},
		{"⓬ ~ ⓮ (tone_tx, TX block)", 23, []byte{0x00, 0x08, 0x85}},
		{"⓯ ~ ⓱ (tone_rx, TX block)", 26, []byte{0x00, 0x12, 0x34}},
	} {
		got := record[span.off : span.off+3]
		if !bytes.Equal(got, span.want) {
			rev := []byte{span.want[2], span.want[1], span.want[0]}
			t.Errorf("%s at record offset %d is % X, want % X — packed BCD, MOST significant pair first (PDF p.23's per-nibble weights). % X would be the same value with the span's byte order reversed",
				span.what, span.off, got, span.want, rev)
		}
	}

	answer := make([]byte, 0, len(frame))
	answer = append(answer, 0xFE, 0xFE, 0xE0, 0xB6, 0x1A, 0x00, frame[6], frame[7])
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
