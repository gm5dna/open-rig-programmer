// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import "github.com/gm5dna/open-rig-programmer/core/civ"

// Profile returns the IC-705's CI-V dialect.
//
// It is the package's ONLY exported symbol, and that is the design: this
// is a DATA package. Everything a driver does with the profile — probing,
// reading, building a write, deciding what a tone number means — belongs
// to core/driver/ic705, and a model package that reached for
// BuildMemorySet would be doing the driver's job at the layer that has no
// session, no gate audit and no user in front of it (internal/guards rule
// 4 refuses it outright).
//
// The returned Profile is a VALUE. civ.Profile holds no pointer a caller
// could mutate through — its layouts and charset are copied out on
// demand — so handing the package variable back directly is safe.
func Profile() civ.Profile { return profile }

// nameCharset is every byte an IC-705 memory name may carry: 0x20..0x7E.
//
// IT IS AN ENUMERATION THAT HAPPENS TO BE CONTIGUOUS, not a range this
// package chose. Matrix §3.9 as corrected by erratum 2 records the printed
// tables as A~Z (41~5A), a~z (61~7A), 0~9 (30~39) and thirty-two symbols;
// those thirty-two are exactly ASCII's punctuation, so the enumerated set
// plus the ASSUMED space (0x20 — lift L-NAME-SPACE) adds up to precisely
// 0x20..0x7E, which also reconciles the same page's flat statement that
// "All characters are usable." The observation is worth writing down and
// is NOT a licence to widen: a byte outside this run is refused, and the
// space is an assumption rather than a printed row.
const nameCharset = ` !"#$%&'()*+,-./0123456789:;<=>?@` +
	`ABCDEFGHIJKLMNOPQRSTUVWXYZ[\]^_` + "`" +
	`abcdefghijklmnopqrstuvwxyz{|}~`

// txBlockDelta is how far the duplicated TX block sits from the RX area
// it mirrors.
//
// PDF p.19's NOTE panel: "The same data as ⑥ ~ 52 are stored in ❻ ~ ❺❷.
// / When the Split function is ON, the data of ❻ ~ ❺❷ is used for
// transmit." Data-area positions 6..52 are record offsets 1..47 and the
// block begins at data-area position 53 = record offset 48, so the copy
// of any span sits exactly 47 bytes further on.
const txBlockDelta = 47

// dup emits a field's RX span and its copy inside the duplicated TX block.
//
// EVERY DUPLICATED FIELD IS DECLARED ONCE. civ encodes both copies and
// requires them to AGREE on decode (spec D5 entry 4), so a layout that
// spelled the two spans out separately could drift by a byte in one of
// them and produce records this radio would read as two different
// channels — which is exactly the class of error the crosscheck test
// cannot see, because it checks the RX span against the evidence and the
// evidence prints no indices at all for the block (transcription B's row
// for `●6 ~ ●52` has an EMPTY label: nothing is printed against it
// anywhere on the page).
func dup(sp civ.FieldSpan) []civ.FieldSpan {
	cp := sp
	cp.Offset += txBlockDelta
	return []civ.FieldSpan{sp, cp}
}

// fields is the record layout, in data-area order.
//
// OFFSETS ARE 0-BASED FROM THE START OF THE RECORD, which is the data
// area minus its four address bytes: offset = data-area position − 5.
// Every position below is `geometry-witness.csv`'s MEASURED first_byte,
// never a printed index — the witness's STOP 3 is that the diagram's last
// bracket prints "53~68" over cells it measures at 100..115, so a layout
// trusting the printed label would put the name forty-seven bytes early.
var fields = concat(
	// Data-area position 5 (record offset 0) is UNMAPPED, both nibbles:
	// Split OFF/ON in the high nibble and the ★n Select marking in the
	// low (witness D2; transcription B's `(5)`). O-6 refuses to map the
	// Select nibble — spec.FieldScanSkip is a two-valued skip flag and
	// this is a four-valued marking INTO a select group, and civ's
	// FieldSelect could only be written by synthesising a value from a
	// ChannelData that has no Select member. So the byte belongs to the
	// template, and a channel carrying Split ON or a ★n marking is
	// REFUSED by the preservation check rather than silently demoted.

	// 6–10 → offset 1: the RX frequency. DECLARED SINGLY, not through
	// dup: its counterpart at offset 48 is a DIFFERENT neutral field
	// (tx_frequency), and civ requires copies of the SAME field to agree.
	// Requiring these two to agree would be wrong — when Split is ON the
	// block's frequency IS the transmit one (PDF p.19's NOTE panel).
	[]civ.FieldSpan{{
		Field: civ.FieldRXFrequency, Offset: 1, Length: 5,
		Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1,
	}},

	// 11 → offset 6: operating mode. Transcription B merges ⑪ and ⑫ into
	// one two-byte legend entry ("Operating mode setting"); the witness
	// and the field ledger both carry them as two separate one-byte
	// cells, and the second is the filter.
	dup(civ.FieldSpan{
		Field: civ.FieldMode, Offset: 6, Length: 1,
		Encoding: civ.EncodingEnum, Enum: modes,
	}),
	// 12 → offset 7: filter.
	dup(civ.FieldSpan{
		Field: civ.FieldFilter, Offset: 7, Length: 1,
		Encoding: civ.EncodingEnum, Enum: map[byte]string{
			0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3",
		},
	}),
	// 13 → offset 8: data mode. "1 byte data (XX) | 00: Data mode OFF |
	// 01: Data mode ON" — the one field on this page whose width is
	// printed in words.
	dup(civ.FieldSpan{
		Field: civ.FieldDataMode, Offset: 8, Length: 1,
		Encoding: civ.EncodingEnum, Enum: map[byte]string{
			0x00: "OFF", 0x01: "ON",
		},
	}),
	// 14 → offset 9, split by nibble (witness D3). The LEFT (high) nibble
	// leads to the duplex list and the RIGHT (low) to the tone-mode list;
	// the golden set vector's 0x11 is therefore DUP− with TONE.
	dup(civ.FieldSpan{
		Field: civ.FieldDuplex, Offset: 9, Length: 1, Nibble: civ.NibbleHigh,
		Encoding: civ.EncodingEnum, Enum: map[byte]string{
			0x00: "OFF", 0x01: "DUP-", 0x02: "DUP+",
		},
	}),
	dup(civ.FieldSpan{
		Field: civ.FieldToneMode, Offset: 9, Length: 1, Nibble: civ.NibbleLow,
		Encoding: civ.EncodingEnum, Enum: map[byte]string{
			0x00: "OFF", 0x01: "TONE", 0x02: "TSQL", 0x03: "DTCS",
		},
	}),
	// 15 → offset 10 is UNMAPPED: the digital squelch setting, whose
	// second nibble the witness records as the literal 0 that diagram D4
	// labels "Fixed". Neutral fields have nowhere to carry DSQL/CSQL, so
	// the whole byte is the template's and a channel holding either is
	// REFUSED.

	// 16–18 → offset 11 and 19–21 → offset 14: the two three-byte tone
	// fields. WHICH IS WHICH IS AN ARGUED ASSUMPTION (O-5, register entry
	// ic705-tone-field-roles, lift L-TONE-ROLE): the page prints the
	// IDENTICAL label "Repeater tone frequency setting" over both, and
	// all three legs that read the legend recorded the disagreement
	// without resolving it. The cross-reference names "Repeater tone/tone
	// squelch" for commands 1B 00, 1B 01 in that order, so the first
	// field is read as the repeater tone and the second as the tone
	// squelch. The golden vector cannot discriminate: it carries 88.5 Hz
	// in both.
	//
	// THE ENCODING IS LOSSLESS AND SEMANTICS-FREE (T1 layer 1). These
	// spans decode and encode plain BCD deciHertz over the whole
	// encodable range, ZERO INCLUDED — the value the radio uses for "no
	// tone set". civ says nothing about whether 0 is a tone; the declared
	// capability floor (CTCSSToneRange{1, 2999, 1}) and the
	// Known/Unknown decision live in the driver, and putting them here
	// would cost the gate its byte-identity re-encode.
	dup(civ.FieldSpan{
		Field: civ.FieldToneTX, Offset: 11, Length: 3,
		Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1,
	}),
	dup(civ.FieldSpan{
		Field: civ.FieldToneRX, Offset: 14, Length: 3,
		Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1,
	}),
	// 22–24 → offsets 17..19: transcription B carries all three bytes as
	// one legend entry, "DTCS code setting". Matrix §1b splits them —
	// polarity in the first byte, the three-digit code in the other two —
	// and that split is ASSUMED (lift L-DTCS-POLARITY), the page printing
	// only a cross-reference to "DTCS code and polarity setting".
	dup(civ.FieldSpan{
		Field: civ.FieldDTCSPolarity, Offset: 17, Length: 1,
		Encoding: civ.EncodingEnum, Enum: map[byte]string{
			0x00: "NN", 0x01: "NR", 0x10: "RN", 0x11: "RR",
		},
	}),
	dup(civ.FieldSpan{
		Field: civ.FieldDTCSCode, Offset: 18, Length: 2,
		Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1,
	}),
	// 25 → offset 20 is UNMAPPED: the DV digital code squelch setting.

	// 26–28 → offset 21: the duplex offset, documented in 100 Hz units,
	// so Scale 100 reaches the neutral Hz. The golden vector's 00 60 00
	// little-endian is 6000 × 100 = 600 kHz.
	dup(civ.FieldSpan{
		Field: civ.FieldOffset, Offset: 21, Length: 3,
		Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 100,
	}),
	// 29–36, 37–44, 45–52 → offsets 24..47 are UNMAPPED: the UR, R1 and
	// R2 DV call signs, eight characters each. Their template value is
	// 0x20 (ASSUMED — an unset call sign reads back as spaces; lift
	// L-CALLSIGN-BLANK), and their TX-block copies at 71..94 with them.

	// 53–57 → offset 48: the TX frequency, matrix ERRATUM 1's positions
	// (53–57, not the body's 54–58). Declared singly, for the reason
	// given at rx_frequency above.
	[]civ.FieldSpan{{
		Field: civ.FieldTXFrequency, Offset: 48, Length: 5,
		Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1,
	}},

	// 100–115 → offset 95: the sixteen-byte name. The witness MEASURES
	// these positions; the diagram PRINTS "53~68" over them.
	[]civ.FieldSpan{{
		Field: civ.FieldName, Offset: 95, Length: 16,
		Encoding: civ.EncodingName,
	}},
)

// modes is the operating-mode enum, verbatim from PDF p.18 (folio 17),
// `• Operating mode`.
//
// THE DV CODE IS AN ASSUMPTION. Every other code printed there is
// identical read as decimal or as hex; `17` is not, and reading it as hex
// 0x17 is register entry ic705-dv-mode-code, lift L-DV-MODE. The lift is
// one observation: set a channel to DV from the front panel, read it, and
// record record byte 11's value.
var modes = map[byte]string{
	0x00: "LSB",
	0x01: "USB",
	0x02: "AM",
	0x03: "CW",
	0x04: "RTTY",
	0x05: "FM",
	0x06: "WFM",
	0x07: "CW-R",
	0x08: "RTTY-R",
	0x17: "DV",
}

// fixedTemplate is the value of every record byte no span claims: 0x20
// across the three call-sign areas and their TX-block copies, 0x00
// everywhere else.
//
// IT IS THE WRITE GUARD'S OTHER HALF (E6). The encoder fills every
// unmapped byte from this template, so a record whose unmapped areas hold
// anything else cannot be rebuilt — which is precisely why the driver
// REFUSES such a slot with the reason named rather than writing the
// template over the radio's own D-STAR routing. The 0x20 runs are what
// make a channel with no call signs set writable at all.
var fixedTemplate = buildFixedTemplate()

func buildFixedTemplate() []byte {
	t := make([]byte, 111)
	for _, r := range [][2]int{{24, 47}, {71, 94}} {
		for i := r[0]; i <= r[1]; i++ {
			t[i] = 0x20
		}
	}
	return t
}

// concat flattens the span groups above, so that dup's two-span result
// and a singly-declared field's one-span result can sit in one list
// without an append chain obscuring which is which.
func concat(groups ...[]civ.FieldSpan) []civ.FieldSpan {
	var out []civ.FieldSpan
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// profile is the IC-705's dialect, built once at init.
//
// MustNewProfile rather than NewProfile: a model table that does not
// validate is a transcription error in this file, not a runtime condition
// a caller could handle, and failing at init is how it is found by the
// first test to import the package rather than by the first user to open
// a radio.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "IC-705",
	RadioAddress: 0xA4, // PDF p.3 (folio 2), Controller to IC-705, cell ②.
	// ControllerAddress is omitted: 0xE0 is civ.ControllerAddressDefault
	// and the same anchor's cell ③.

	// MaxFrame: this model's longest frame is the 122-byte memory set,
	// and 128 is the smallest power of two above it, so the accumulator
	// refuses contamination as tightly as this radio allows. A CHOICE,
	// argued, not evidence — no document prints a frame ceiling.
	MaxFrame: 128,

	// The four-byte address form, enabler E4's (Task 1 proves it
	// reproduces this radio's own read frames).
	AddressForm: civ.AddressFormWideGroupChannel,

	// Groups is a COUNT over WIRE indices: 0..99 are the memory groups
	// the manual prints as 0000~0099, and 100 is the CALL group it prints
	// as 0100 (matrix §1b). GroupBase is OMITTED deliberately — it
	// defaults to 0, which is where this radio starts counting.
	Groups:    101,
	ChannelLo: 0,
	// ChannelHi is MEM's range. The CALL group's true range is 0..3, and
	// civ carries ONE range for every group — see O-9, ruled DEFERRED on
	// 24/08/2026. The consequences are covered twice over in the driver:
	// slotToAddress maps only G101-001..G101-004 and refuses a higher
	// CALL channel before any builder is reached, and every write passes
	// the bank check first. Narrowing this to 3 instead would make 96 of
	// every MEM group's channels unaddressable.
	ChannelHi: 99,

	NameLength:  16,
	NameCharset: nameCharset,
	NamePad:     0x20, // ASSUMED — lift L-NAME-SPACE/L-NAME-PAD.

	// ONE declared length, and it is the RECORD-ONLY number. Spec
	// Erratum 1: profiles carry record-only lengths. 111 = the 115-byte
	// `1A 00` data area the witness measures, less its four address
	// bytes. 115 must never appear here.
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   111,
	Layouts: []civ.RecordLayout{{
		Length: 111,
		Fields: fields,
		Fixed:  fixedTemplate,
	}},
})
