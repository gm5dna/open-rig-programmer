// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300mk2

import (
	"fmt"
	"testing"
)

// The expected numbers below are written out as LITERALS, recomputed by hand
// from the transcription's printed field widths, rather than derived from the
// package's own constants. A test that says offName == offName proves nothing.

// TestRecordLengthIsFortyFive pins the derivation recorded in doc.go's ASSUMED
// register entry 1: diagram D1's nine printed index blocks total 47 indices,
// the first two of which are the channel address the 1A 00 command carries
// ahead of the record.
func TestRecordLengthIsFortyFive(t *testing.T) {
	printed := []struct {
		block string
		n     int
	}{
		{"①, ② Memory channel number", 2},
		{"③ Split and Select memory setting", 1},
		{"④ ~ ⑧ Operating frequency setting", 5},
		{"⑨, ⑩ Operating mode setting", 2},
		{"⑪ Data mode and tone type settings", 1},
		{"⑫ ~ ⑭ Repeater tone frequency setting", 3},
		{"⑮ ~ ⑰ Tone squelch frequency setting", 3},
		{"❹ ~ ⓱ (transmit side)", 14},
		{"⑱ ~ ㉝ Memory name settings", 16},
	}
	total := 0
	for _, p := range printed {
		total += p.n
	}
	if total != 47 {
		t.Fatalf("the printed index counts total %d, want 47 — the transcription's width_bytes column", total)
	}
	if got, want := recordLen, total-2; got != want {
		t.Errorf("recordLen = %d, want %d (47 printed indices less the two channel-address bytes)", got, want)
	}
	if recordLen != 45 {
		t.Errorf("recordLen = %d, want 45", recordLen)
	}
}

// TestFieldOffsets pins every offset as a literal, so that a change to one
// width cannot slide the rest along unnoticed.
func TestFieldOffsets(t *testing.T) {
	got := map[string]int{
		"③":               offSplitSelect,
		"④ ~ ⑧":           offFrequency,
		"⑨, ⑩":            offMode,
		"⑪":               offDataTone,
		"⑫ ~ ⑭":           offRepeaterTone,
		"⑮ ~ ⑰":           offToneSquelch,
		"❹ ~ ⓱":           offShadow,
		"❹ ~ ⓱ frequency": offShadowFrequency,
		"❹ ~ ⓱ mode":      offShadowMode,
		"❹ ~ ⓱ data/tone": offShadowDataTone,
		"❹ ~ ⓱ rpt tone":  offShadowRepeaterTone,
		"❹ ~ ⓱ tone sql":  offShadowToneSquelch,
		"⑱ ~ ㉝":           offName,
	}
	want := map[string]int{
		"③":               0,
		"④ ~ ⑧":           1,
		"⑨, ⑩":            6,
		"⑪":               8,
		"⑫ ~ ⑭":           9,
		"⑮ ~ ⑰":           12,
		"❹ ~ ⓱":           15,
		"❹ ~ ⓱ frequency": 15,
		"❹ ~ ⓱ mode":      20,
		"❹ ~ ⓱ data/tone": 22,
		"❹ ~ ⓱ rpt tone":  23,
		"❹ ~ ⓱ tone sql":  26,
		"⑱ ~ ㉝":           29,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("offset of %s = %d, want %d", k, got[k], w)
		}
	}
}

// TestFieldTableIsContiguousAndComplete checks that recordFields tiles the
// whole record exactly once: no gap, no overlap, no byte outside a field.
// Without this a validator could be silently skipping a field.
func TestFieldTableIsContiguousAndComplete(t *testing.T) {
	covered := make([]int, recordLen)
	for _, f := range recordFields {
		if f.off < 0 || f.off+f.width > recordLen {
			t.Fatalf("field %q spans %d..%d, outside the %d-byte record", f.name, f.off, f.off+f.width, recordLen)
		}
		for i := f.off; i < f.off+f.width; i++ {
			covered[i]++
		}
	}
	for i, n := range covered {
		if n != 1 {
			t.Errorf("record byte %d is covered by %d fields, want exactly 1", i, n)
		}
	}
	if len(recordFields) != 12 {
		t.Errorf("recordFields has %d entries, want 12 (six receive-side fields, five in the ❹ ~ ⓱ block, and the name)", len(recordFields))
	}
}

// TestSlotAddressRoundTrip walks every address the ①, ② legend prints and
// checks the wire bytes against hand-written expectations.
func TestSlotAddressRoundTrip(t *testing.T) {
	for n := 1; n <= 99; n++ {
		slot := fmt.Sprintf("%03d", n)
		wire, ok := slotWire(slot)
		if !ok {
			t.Fatalf("slotWire(%q) refused a printed address", slot)
		}
		wantLo := byte(n/10)<<4 | byte(n%10)
		if wire[0] != 0x00 || wire[1] != wantLo {
			t.Errorf("slotWire(%q) = %02X %02X, want 00 %02X", slot, wire[0], wire[1], wantLo)
		}
		back, ok := slotFromWire(wire[0], wire[1])
		if !ok || back != slot {
			t.Errorf("slotFromWire(%02X, %02X) = %q, %v; want %q, true", wire[0], wire[1], back, ok, slot)
		}
	}

	edges := []struct {
		slot   string
		b0, b1 byte
	}{
		{"P1", 0x01, 0x00},
		{"P2", 0x01, 0x01},
	}
	for _, e := range edges {
		wire, ok := slotWire(e.slot)
		if !ok || wire[0] != e.b0 || wire[1] != e.b1 {
			t.Errorf("slotWire(%q) = %02X %02X, %v; want %02X %02X, true", e.slot, wire[0], wire[1], ok, e.b0, e.b1)
		}
		back, ok := slotFromWire(e.b0, e.b1)
		if !ok || back != e.slot {
			t.Errorf("slotFromWire(%02X, %02X) = %q, %v; want %q, true", e.b0, e.b1, back, ok, e.slot)
		}
		if !isScanEdge(e.slot) {
			t.Errorf("isScanEdge(%q) = false, want true", e.slot)
		}
	}
	if isScanEdge("001") {
		t.Error("isScanEdge(\"001\") = true, want false")
	}
}

// TestSlotFromWireRefusesAnyFourthForm pins "three forms, and no fourth".
func TestSlotFromWireRefusesAnyFourthForm(t *testing.T) {
	bad := []struct {
		b0, b1 byte
		why    string
	}{
		{0x00, 0x00, "channel 00 is not printed: the legend starts at 00 01"},
		{0x00, 0x0A, "not packed BCD — no hexadecimal digit above 9 is used"},
		{0x00, 0xA0, "not packed BCD"},
		{0x00, 0x9A, "not packed BCD"},
		{0x01, 0x02, "there is no scan edge P3"},
		{0x01, 0x10, "there is no scan edge P10"},
		{0x02, 0x00, "there is no third bank"},
		{0xFF, 0xFF, "not an address at all"},
	}
	for _, b := range bad {
		if slot, ok := slotFromWire(b.b0, b.b1); ok {
			t.Errorf("slotFromWire(%02X, %02X) = %q, true; want refused (%s)", b.b0, b.b1, slot, b.why)
		}
	}
}

// TestCanonicalSlot covers the spellings Channel and the seeding options
// accept, and the ones they must not.
func TestCanonicalSlot(t *testing.T) {
	good := map[string]string{
		"001":  "001",
		"099":  "099",
		"042":  "042",
		"P1":   "P1",
		"p2":   "P2",
		" P1 ": "P1",
	}
	for in, want := range good {
		got, ok := canonicalSlot(in)
		if !ok || got != want {
			t.Errorf("canonicalSlot(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "0", "1", "01", "000", "100", "0001", "P0", "P3", "PP", "abc", "0x1"} {
		if got, ok := canonicalSlot(in); ok {
			t.Errorf("canonicalSlot(%q) = %q, true; want refused", in, got)
		}
	}
}

// validTestRecord is a 45-byte record every field of which carries a value the
// transcription prints: SELECT ★1, 14.250000 MHz, CW/FIL1, TONE, 88.5 Hz tone
// squelch, the transmit side set to the same data as the NOTE box recommends,
// and a sixteen-byte name padded with the printed space.
func validTestRecord() []byte {
	rec := make([]byte, recordLen)
	rec[offSplitSelect] = 0x01 // SPLIT 0=OFF, SELECT 1=★1

	freq := []byte{0x00, 0x00, 0x25, 0x14, 0x00} // 14.250000 MHz
	mode := []byte{0x03, 0x01}                   // CW, FIL1
	dataTone := byte(0x01)                       // DATA 0=OFF, TONE 1=TONE
	rptTone := []byte{0x00, 0x08, 0x85}          // 088.5, in ⑮ ~ ⑰'s shape
	toneSql := []byte{0x00, 0x08, 0x85}          // 088.5 Hz

	copy(rec[offFrequency:], freq)
	copy(rec[offMode:], mode)
	rec[offDataTone] = dataTone
	copy(rec[offRepeaterTone:], rptTone)
	copy(rec[offToneSquelch:], toneSql)

	copy(rec[offShadowFrequency:], freq)
	copy(rec[offShadowMode:], mode)
	rec[offShadowDataTone] = dataTone
	copy(rec[offShadowRepeaterTone:], rptTone)
	copy(rec[offShadowToneSquelch:], toneSql)

	copy(rec[offName:], []byte("MK2 TEST        "))
	return rec
}

func TestValidTestRecordIsValid(t *testing.T) {
	rec := validTestRecord()
	if len(rec) != recordLen {
		t.Fatalf("validTestRecord() is %d bytes, want %d", len(rec), recordLen)
	}
	if bad := badField(rec); bad != "" {
		t.Fatalf("validTestRecord() rejected at %s — the fixture the rest of the suite depends on is wrong", bad)
	}
}

// TestFieldVocabularies drives one bad byte into each field in turn and
// requires the record to be refused, naming the field it was refused at.
func TestFieldVocabularies(t *testing.T) {
	tests := []struct {
		name  string
		off   int
		value byte
		field string
	}{
		{"③ SPLIT nibble 2 is a diagonally ruled blank", offSplitSelect, 0x20, "③ Split and Select memory setting"},
		{"③ SELECT nibble 4 is not printed", offSplitSelect, 0x04, "③ Split and Select memory setting"},
		{"④ ~ ⑧ 10 Hz digit A is not decimal", offFrequency, 0xA0, "④ ~ ⑧ Operating frequency setting"},
		{"④ ~ ⑧ 10 MHz digit 8 is above the printed 0 ~ 7", offFrequency + 3, 0x84, "④ ~ ⑧ Operating frequency setting"},
		{"④ ~ ⑧ 1 GHz digit is 0 (Fixed)", offFrequency + 4, 0x10, "④ ~ ⑧ Operating frequency setting"},
		{"④ ~ ⑧ 100 MHz digit is 0 (Fixed)", offFrequency + 4, 0x01, "④ ~ ⑧ Operating frequency setting"},
		{"⑨ mode 06 is not printed", offMode, 0x06, "⑨, ⑩ Operating mode setting"},
		{"⑨ mode 09 is not printed", offMode, 0x09, "⑨, ⑩ Operating mode setting"},
		{"⑩ filter 00 is not printed", offMode + 1, 0x00, "⑨, ⑩ Operating mode setting"},
		{"⑩ filter 04 is not printed", offMode + 1, 0x04, "⑨, ⑩ Operating mode setting"},
		{"⑪ DATA nibble 2 is a diagonally ruled blank", offDataTone, 0x20, "⑪ Data mode and tone type settings"},
		{"⑪ TONE nibble 3 is not printed", offDataTone, 0x03, "⑪ Data mode and tone type settings"},
		{"⑮ first byte is printed as literal 00", offToneSquelch, 0x01, "⑮ ~ ⑰ Tone squelch frequency setting"},
		{"⑯ 100 Hz digit 3 is above the printed 0 ~ 2", offToneSquelch + 1, 0x30, "⑮ ~ ⑰ Tone squelch frequency setting"},
		{"⑰ 0.1 Hz digit A is not decimal", offToneSquelch + 2, 0x0A, "⑮ ~ ⑰ Tone squelch frequency setting"},
		{"❹ ~ ⓱ frequency is checked too", offShadowFrequency, 0xA0, "❹ ~ ⓱ (transmit side of ④ ~ ⑧)"},
		{"❹ ~ ⓱ mode is checked too", offShadowMode, 0x06, "❹ ~ ⓱ (transmit side of ⑨, ⑩)"},
		{"❹ ~ ⓱ data/tone is checked too", offShadowDataTone, 0x03, "❹ ~ ⓱ (transmit side of ⑪)"},
		{"❹ ~ ⓱ tone squelch is checked too", offShadowToneSquelch, 0x01, "❹ ~ ⓱ (transmit side of ⑮ ~ ⑰)"},
		{"⑱ NUL is not a printed character code", offName, 0x00, "⑱ ~ ㉝ Memory name settings"},
		{"㉝ DEL is not a printed character code", offName + 15, 0x7F, "⑱ ~ ㉝ Memory name settings"},
		{"⑱ ~ ㉝ high-bit bytes are not printed", offName + 3, 0xC3, "⑱ ~ ㉝ Memory name settings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validTestRecord()
			rec[tt.off] = tt.value
			got := badField(rec)
			if got != tt.field {
				t.Errorf("badField() = %q, want %q", got, tt.field)
			}
			if validRecord(rec) {
				t.Error("validRecord() = true, want false")
			}
		})
	}
}

// TestRepeaterToneAcceptsAnything pins doc.go's ASSUMED register entry 4 — the
// entry most likely to need changing. If a later reading tightens
// repeaterToneOK, THIS TEST IS THE ONE THAT MUST BE EDITED, and its failure is
// the reminder to edit the register entry with it.
func TestRepeaterToneAcceptsAnything(t *testing.T) {
	for _, b := range []byte{0x00, 0x01, 0x0A, 0x99, 0xAB, 0xFF} {
		for _, off := range []int{offRepeaterTone, offRepeaterTone + 1, offRepeaterTone + 2, offShadowRepeaterTone, offShadowRepeaterTone + 2} {
			rec := validTestRecord()
			rec[off] = b
			if bad := badField(rec); bad != "" {
				t.Errorf("byte %02X at offset %d was refused at %s — ⑫ ~ ⑭ prints no vocabulary at all (doc.go ASSUMED 4)", b, off, bad)
			}
		}
	}
}

// TestNameVocabularyIsThePrintedCodes checks the set built from page 18's two
// character tables plus the assumed space, member by member, against a
// hand-written list of what must and must not be in it.
func TestNameVocabularyIsThePrintedCodes(t *testing.T) {
	for c := 0x20; c <= 0x7E; c++ {
		if !nameCodes[byte(c)] {
			t.Errorf("code %02X is not accepted — every code page 18 prints, plus the assumed space, covers 20 to 7E", c)
		}
	}
	for _, c := range []byte{0x00, 0x07, 0x0A, 0x0D, 0x1F, 0x7F, 0x80, 0xA0, 0xFF} {
		if nameCodes[c] {
			t.Errorf("code %02X is accepted — page 18 prints no such code", c)
		}
	}
	// The two codes STOP 2 draws with an identical glyph are BOTH members
	// (doc.go ASSUMED 6).
	if !nameCodes[0x27] || !nameCodes[0x60] {
		t.Error("codes 27 and 60 must both be accepted — page 18 prints both, drawn with the same glyph")
	}
}

// TestBroadcastPayloadIsAWellFormedFrequency checks that the unsolicited
// frames carry something the frequency vocabulary itself accepts, so a reader
// that parses them gets a sane answer (doc.go ASSUMED 19).
func TestBroadcastPayloadIsAWellFormedFrequency(t *testing.T) {
	if len(broadcastPayload) != 5 {
		t.Fatalf("broadcastPayload is %d bytes, want 5", len(broadcastPayload))
	}
	if !frequencyOK(broadcastPayload) {
		t.Errorf("broadcastPayload %X is not a value the frequency field's printed digits allow", broadcastPayload)
	}
	want := []byte{0x00, 0x00, 0x25, 0x14, 0x00} // 14.250000 MHz, hand-packed
	for i := range want {
		if broadcastPayload[i] != want[i] {
			t.Errorf("broadcastPayload = %X, want %X (14.250000 MHz)", broadcastPayload, want)
			break
		}
	}
}
