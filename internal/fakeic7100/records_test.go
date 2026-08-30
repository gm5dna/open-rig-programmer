// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"bytes"
	"testing"
)

// The record layer's tests. The geometry is DERIVED HERE, from the printed
// field-group widths, and every expectation is recomputed by hand rather than
// by calling the constants it is checking. TestRecordGeometryIsDerivedFromThe
// PrintedFieldWidths is the one that matters: it re-does the whole addition
// from the group table, so a constant that drifted would have to drift in two
// places written in two different forms.

// printedGroups is the record diagram's field-group table, transcribed by hand
// from the two artefacts PROVENANCE.md names: the semantic leg's field_index /
// width_bytes columns, corroborated group for group by the geometry witness's
// cell run. It is deliberately written as widths and printed indices, NOT as
// offsets — the offsets are what this test derives.
//
// The two groups after ㊹～(51) are the ones whose printed index and measured
// position part company, and both artefacts record that parting rather than
// reconciling it: the semantic leg's notes give measured positions 52–98 for
// the group printed ❺～(51) and 99 onward for the group printed (52)～…, and
// the witness records the same divergence as its STOP 14 and STOP 15/16.
var printedGroups = []struct {
	printedIndex string
	width        int
	note         string
}{
	{"(1)", 1, "Bank number — 01: A … 05: E"},
	{"(2),(3)", 2, "Memory channel number, packed BCD, 0001–0099"},
	{"(4)", 1, "Split and Select memory settings"},
	{"(5)–(9)", 5, "Operating frequency setting"},
	{"(10),(11)", 2, "Operating mode setting: mode byte, filter byte"},
	{"(12)", 1, "Data mode setting"},
	{"(13)", 1, "Duplex and Tone settings"},
	{"(14)", 1, "Digital squelch setting — low nibble printed a literal 0"},
	{"(15)–(17)", 3, "Repeater tone frequency setting"},
	{"(18)–(20)", 3, "Tone squelch frequency setting"},
	{"(21)–(23)", 3, "DTCS code setting"},
	{"(24)", 1, "Digital code squelch setting"},
	{"(25)–(27)", 3, "Duplex offset frequency setting"},
	{"(28)–(35)", 8, "UR destination call sign (8 characters; fixed)"},
	{"(36)–(43)", 8, "R1 access repeater call sign (8 characters; fixed)"},
	{"(44)–(51)", 8, "R2 gateway/link repeater call sign (8 characters; fixed)"},
	{"5f–51f", 47, "the transmit duplicate, printed in FILLED numerals, no label anywhere"},
	{"(52)–(67)", 16, "Memory name setting, 16 characters (Fixed)"},
}

func TestRecordGeometryIsDerivedFromThePrintedFieldWidths(t *testing.T) {
	// Step 1: the receive part. The printed indices (1) to (51) are one byte
	// each and their group widths add to 51, which is the one place on the page
	// where the printed index and the measured position still agree.
	receive := 0
	for _, g := range printedGroups[:16] {
		receive += g.width
	}
	if receive != 51 {
		t.Fatalf("indices (1)–(51) add to %d bytes, want 51 — the group table above disagrees with the printed index run", receive)
	}

	// Step 2: the transmit duplicate's width, stated as arithmetic rather than
	// counted. The band draws it as ONE dashed elision box, so its extent
	// cannot be counted on the render; its width comes from its printed index
	// span, 5 through 51 inclusive.
	if want := 51 - 5 + 1; printedGroups[16].width != want {
		t.Fatalf("the transmit duplicate is %d bytes, want %d = 51 - 5 + 1", printedGroups[16].width, want)
	}
	// And that span must equal the receive payload it duplicates: the printed
	// NOTE says "The same data as (5)–(51) are stored in 5f–51f", so the two
	// must be the same number of bytes or the sentence could not be true.
	rxPayload := receive - printedGroups[0].width - printedGroups[1].width - printedGroups[2].width
	if rxPayload != printedGroups[16].width {
		t.Fatalf("fields (5)–(51) are %d bytes but the duplicate is %d — the printed NOTE requires them equal", rxPayload, printedGroups[16].width)
	}

	// Step 3: the whole data area, in WIRE order, which is what a 1A 00 answer
	// carries after its command and sub-command bytes.
	dataArea := 0
	for _, g := range printedGroups {
		dataArea += g.width
	}
	if dataArea != 114 {
		t.Fatalf("the data area adds to %d bytes, want 114", dataArea)
	}
	if dataArea != dataAreaLength {
		t.Errorf("dataAreaLength = %d, want %d", dataAreaLength, dataArea)
	}

	// Step 4: the address is the first three bytes of that data area — the bank
	// byte and the two channel bytes — and the record proper is the rest.
	address := printedGroups[0].width + printedGroups[1].width
	if address != addressLength {
		t.Errorf("addressLength = %d, want %d (the bank byte plus the two channel bytes)", addressLength, address)
	}
	if want := dataArea - address; want != recordLength {
		t.Errorf("recordLength = %d, want %d = %d data-area bytes less the %d address bytes", recordLength, want, dataArea, address)
	}
	if want := 111; recordLength != want {
		t.Errorf("recordLength = %d, want %d", recordLength, want)
	}
}

func TestBlockOffsetsWithinTheRecord(t *testing.T) {
	// The record's own byte 0 is data-area byte 4, field (4). Walking the group
	// table from there gives every offset this package needs, and each is
	// checked against the constant it is supposed to have produced.
	//
	// The two offsets that are ASSUMED rather than counted — the duplicate at
	// data-area 52 and the name at data-area 99 — are exactly the two the
	// artefacts flag: the printed indices say 5 and 52, the measured positions
	// say 52 and 99. This package takes the MEASURED positions. See doc.go,
	// register entries 6 (ic7100-tx-block-mandatory) and 7 (ic7100-wire-order).
	tests := []struct {
		name          string
		dataAreaFirst int // 1-based, as both artefacts count
		width         int
		gotOffset     int
		gotLen        int
	}{
		{"the split/select byte (4)", 4, 1, splitSelectOffset, 1},
		{"the receive payload (5)–(51)", 5, 47, rxBlockOffset, rxBlockLength},
		{"the transmit duplicate, measured at 52–98", 52, 47, txBlockOffset, txBlockLength},
		{"the memory name, measured at 99–114", 99, 16, nameOffset, nameLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A data-area position counted from 1 becomes a record offset
			// counted from 0 by dropping the three address bytes and the
			// 1-based origin: offset = position - 1 - 3.
			wantOffset := tt.dataAreaFirst - 1 - addressLength
			if tt.gotOffset != wantOffset {
				t.Errorf("offset = %d, want %d (data-area byte %d)", tt.gotOffset, wantOffset, tt.dataAreaFirst)
			}
			if tt.gotLen != tt.width {
				t.Errorf("length = %d, want %d", tt.gotLen, tt.width)
			}
		})
	}

	// The four blocks must tile the record exactly: no gap, no overlap, nothing
	// past the end.
	if got := splitSelectOffset; got != 0 {
		t.Errorf("the record does not begin at the split/select byte: offset %d", got)
	}
	if got, want := rxBlockOffset, splitSelectOffset+1; got != want {
		t.Errorf("gap or overlap before the receive payload: %d, want %d", got, want)
	}
	if got, want := txBlockOffset, rxBlockOffset+rxBlockLength; got != want {
		t.Errorf("gap or overlap before the transmit duplicate: %d, want %d", got, want)
	}
	if got, want := nameOffset, txBlockOffset+txBlockLength; got != want {
		t.Errorf("gap or overlap before the name: %d, want %d", got, want)
	}
	if got, want := nameOffset+nameLength, recordLength; got != want {
		t.Errorf("the record ends at %d, want %d", got, want)
	}
}

func TestChannelAddress(t *testing.T) {
	tests := []struct {
		name    string
		bank    int
		channel int
		want    []byte
		wantErr bool
	}{
		// The G leg's own read-request vector is FE FE 88 E0 1A 00 01 00 01 FD:
		// bank A, memory channel 1.
		{name: "bank A channel 1 — the printed lowest values", bank: 1, channel: 1, want: []byte{0x01, 0x00, 0x01}},
		{name: "bank A channel 99", bank: 1, channel: 99, want: []byte{0x01, 0x00, 0x99}},
		{name: "bank E channel 99 — the far corner", bank: 5, channel: 99, want: []byte{0x05, 0x00, 0x99}},
		{name: "bank C channel 42 is packed BCD, not binary", bank: 3, channel: 42, want: []byte{0x03, 0x00, 0x42}},
		{name: "bank C channel 10 would be 0x0A in binary", bank: 3, channel: 10, want: []byte{0x03, 0x00, 0x10}},

		{name: "bank 0 is not a printed bank code", bank: 0, channel: 1, wantErr: true},
		{name: "bank 6 is not a printed bank code", bank: 6, channel: 1, wantErr: true},
		{name: "channel 0 is not in the field legend's range", bank: 1, channel: 0, wantErr: true},
		{name: "channel 100 is a programmed scan edge, out of scope", bank: 1, channel: 100, wantErr: true},
		{name: "channel 106 is a call channel, out of scope", bank: 1, channel: 106, wantErr: true},
		{name: "channel 110 is not printed at all", bank: 1, channel: 110, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := channelAddress(tt.bank, tt.channel)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("channelAddress(%d, %d) = %s, want an error", tt.bank, tt.channel, hexBytes(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("channelAddress(%d, %d): %v", tt.bank, tt.channel, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("channelAddress(%d, %d) = %s, want %s", tt.bank, tt.channel, hexBytes(got), hexBytes(tt.want))
			}
		})
	}
}

func TestAddressIsInScope(t *testing.T) {
	tests := []struct {
		name string
		addr []byte
		want bool
	}{
		{"bank A channel 1", []byte{0x01, 0x00, 0x01}, true},
		{"bank E channel 99", []byte{0x05, 0x00, 0x99}, true},
		{"bank 00 is not a printed bank", []byte{0x00, 0x00, 0x01}, false},
		{"bank 06 is not a printed bank", []byte{0x06, 0x00, 0x01}, false},
		{"channel 0000 is outside the legend's range", []byte{0x01, 0x00, 0x00}, false},
		{"channel 0100, programmed scan edge 1A — out of scope", []byte{0x01, 0x01, 0x00}, false},
		{"channel 0106, call channel 144-C1 — out of scope", []byte{0x01, 0x01, 0x06}, false},
		{"channel byte 0x9A is not packed BCD", []byte{0x01, 0x00, 0x9A}, false},
		{"channel byte 0xA0 is not packed BCD", []byte{0x01, 0x00, 0xA0}, false},
		{"high channel byte 0x1A is not packed BCD", []byte{0x01, 0x1A, 0x00}, false},
		{"two bytes is not an address", []byte{0x01, 0x00}, false},
		{"four bytes is not an address", []byte{0x01, 0x00, 0x01, 0x00}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addressIsInScope(tt.addr); got != tt.want {
				t.Errorf("addressIsInScope(%s) = %v, want %v", hexBytes(tt.addr), got, tt.want)
			}
		})
	}
}

func TestTXBlockMatchesRX(t *testing.T) {
	// A record whose transmit duplicate carries the same bytes as the receive
	// payload, and one whose does not, differing at each end and in the middle.
	base := make([]byte, recordLength)
	for i := range base {
		base[i] = byte(i % 251)
	}
	copy(base[txBlockOffset:txBlockOffset+txBlockLength], base[rxBlockOffset:rxBlockOffset+rxBlockLength])
	if !txBlockMatchesRX(base) {
		t.Fatal("a record whose duplicate was copied from the receive payload does not match itself")
	}

	for _, at := range []int{0, 1, txBlockLength / 2, txBlockLength - 2, txBlockLength - 1} {
		spoilt := append([]byte(nil), base...)
		spoilt[txBlockOffset+at] ^= 0x01
		if txBlockMatchesRX(spoilt) {
			t.Errorf("a duplicate differing at byte %d of %d still matched", at, txBlockLength)
		}
	}

	// The name and the split byte are outside both blocks and must not affect
	// the comparison — a record is not disqualified by its name.
	renamed := append([]byte(nil), base...)
	copy(renamed[nameOffset:], []byte("HOME BASE       "))
	renamed[splitSelectOffset] = 0x10
	if !txBlockMatchesRX(renamed) {
		t.Error("changing the name or the split byte broke the transmit-duplicate comparison")
	}

	// A record of the wrong length has no blocks to compare and cannot match.
	if txBlockMatchesRX(base[:recordLength-1]) {
		t.Error("a short record reported a matching duplicate")
	}
}

func TestIsClearForm(t *testing.T) {
	// PDF p.375 (folio 20-16), "About clearing operation": the block prints
	// (2),(3) as the memory channel, (4) as FF, and "(5) or later: None" — and
	// omits field (1) entirely, so the document never says whether a clear
	// frame carries a bank byte. Both readings are refused; see doc.go,
	// register entry 8.
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"with the bank byte: three address bytes then FF", []byte{0x01, 0x00, 0x01, 0xFF}, true},
		{"without it, as the block itself prints: two channel bytes then FF", []byte{0x00, 0x01, 0xFF}, true},
		{"three address bytes and nothing after is a read, not a clear", []byte{0x01, 0x00, 0x01}, false},
		{"a byte that is not FF is not the clear form", []byte{0x01, 0x00, 0x01, 0x00}, false},
		{"address plus a whole record is a set", append([]byte{0x01, 0x00, 0x01}, make([]byte, recordLength)...), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClearForm(tt.payload); got != tt.want {
				t.Errorf("isClearForm(%s) = %v, want %v", hexBytes(tt.payload), got, tt.want)
			}
		})
	}
}
