// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        Model,
	RadioAddress: RadioAddress,
	// ControllerAddress left zero: civ defaults it to
	// ControllerAddressDefault (0xE0), which is what PDF p.3 (folio 2)
	// prints in cell (3) above "Controller's (PC's) default address".
	NameLength:  16,
	NameCharset: nameCharset,
	NamePad:     ' ',

	// (1),(2) memory group and (3),(4) memory channel: four packed-BCD
	// bytes (PDF p.19, folio 18, left legend). Groups counts 101 —
	// the hundred memory groups 00 00 ~ 00 99 plus the Call channel
	// group 01 00, which as a two-byte big-endian packed BCD group is
	// index 100, consecutive with them. Group indices are WIRE indices
	// (E4), base 0.
	//
	// The NEW four-byte form, never AddressFormGroupChannel, which E4
	// keeps at three bytes and byte-identical for the models already
	// using it.
	//
	// GroupBase 0: this radio calls its first memory group 00 00, so the
	// wire indices run 0..100 and base + Groups - 1 = 100 fits the
	// form's two-byte BCD width. (The 9700 is the base-1 model; the
	// landed civtest is base-aware for exactly that reason.)
	AddressForm: civ.AddressFormWideGroupChannel,
	GroupBase:   0,
	Groups:      101,
	ChannelLo:   0,
	ChannelHi:   99,

	Layouts:       []civ.RecordLayout{layoutFor(5), layoutFor(6)},
	Discriminator: civ.DiscriminatorRecordLength,
	// See length.go and doc.go: the shape the diagram draws.
	BuildLength: RecordLengthShort,
})

// Profile is the IC-905's CI-V dialect.
//
// A value, not a pointer, built once from one config literal: the
// package holds no mutable state and no init-order hazard, and
// MustNewProfile panics at init rather than shipping a profile
// NewProfile would have refused.
func Profile() civ.Profile { return profile }

// layoutFor builds one of this model's two record layouts.
//
// freqBytes is the width of the operating-frequency field — five in the
// shape PDF p.19 (folio 18) draws, six in the 10 GHz form (matrix
// section 3.11 Condition B, ASSUMED, lift ic905-R-06). EVERY OTHER
// FIELD IS FIXED WIDTH, which is why one generator serves both and why
// a read needs no discriminator beyond the length (matrix Erratum 8).
//
// Offsets are 0-based from the start of the RECORD, i.e. after the four
// channel-address bytes. The printed index each span carries is in its
// comment and is the join key crosscheck_test.go uses.
func layoutFor(freqBytes int) civ.RecordLayout {
	off := func(afterFreq int) int { return afterFreq + freqBytes - 5 }
	length := 64 + freqBytes - 5

	fixed := make([]byte, length)
	for i := off(24); i < off(48); i++ {
		// (29)~(36), (37)~(44), (45)~(52): the UR, R1 and R2 call
		// signs, eight characters fixed apiece. Neither civ.FieldID nor
		// codeplug.ChannelData has a home for a call sign, so they are
		// unmapped and their template value is the space this
		// document's own call-sign character table prints, "(Space) |
		// 20" (PDF p.24, folio 23). See doc.go, "What this profile
		// cannot express".
		fixed[i] = 0x20
	}

	return civ.RecordLayout{
		Length: length,
		Fixed:  fixed,
		Fields: []civ.FieldSpan{
			// (6)~(10): Operating frequency setting. Packed BCD, least
			// significant pair first: the p.17 (folio 16) diagram's
			// rotated labels run "10 Hz digit", "1 Hz digit", "1 kHz",
			// "100 Hz", "100 kHz", "10 kHz", "10 MHz", "1 MHz",
			// "1 GHz", "100 MHz".
			{Field: civ.FieldRXFrequency, Offset: 1, Length: freqBytes, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			// (11): Operating mode setting. PDF p.17 (folio 16),
			// "Operating mode", column "(1)Operating mode". Code 06 is
			// absent from the printed table with no note (matrix
			// section 6, EC-4) and is therefore absent here.
			{Field: civ.FieldMode, Offset: off(6), Length: 1, Encoding: civ.EncodingEnum, Enum: modeNames},
			// (12): Filter setting, same table, column "(2)Filter
			// setting".
			{Field: civ.FieldFilter, Offset: off(7), Length: 1, Encoding: civ.EncodingEnum, Enum: filterNames},
			// (13): Data mode setting. PDF p.19 legend: "1 byte data
			// (XX)" / "00: Data mode OFF" / "01: Data mode ON".
			{Field: civ.FieldDataMode, Offset: off(8), Length: 1, Encoding: civ.EncodingEnum, Enum: dataModeNames},
			// (14) high nibble: Duplex. PDF p.19 "(14): Duplex and Tone
			// settings" breakout, LEFT nibble leader.
			{Field: civ.FieldDuplex, Offset: off(9), Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: duplexNames},
			// (14) low nibble: Tone mode, same breakout, RIGHT nibble
			// leader. Four legs and the matrix followed the two leaders
			// independently and all agree on which list serves which
			// half.
			{Field: civ.FieldToneMode, Offset: off(9), Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeNames},
			// (16)~(18): the FIRST of the two identically-labelled
			// three-byte tone blocks. Big-endian BCD in TENTHS of a
			// hertz: the p.24 (folio 23) diagram prints "0 : 0" fixed,
			// then "100 Hz : 10 Hz", then "1 Hz : 0.1 Hz".
			//
			// WHICH BLOCK IS THE TX TONE IS ASSUMED. PDF p.19 prints
			// "Repeater tone frequency setting" for BOTH (16)~(18) and
			// (19)~(21), under one pointer to a section whose title
			// names two settings. All four quarantine legs raise it as
			// a STOP (matrix section 6, EC-3). Register:
			// ic905.tone_block_assignment. Lift: ic905-R-07. Both
			// golden vectors carry 00 08 85 in BOTH blocks, so no
			// delivered byte depends on this being right.
			{Field: civ.FieldToneTX, Offset: off(11), Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// (19)~(21): the second block. Same assumption, same lift.
			{Field: civ.FieldToneRX, Offset: off(14), Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// (22): DTCS polarity, one nibble per direction. HIGH
			// nibble is TRANSMIT, LOW is RECEIVE — matrix Erratum 3
			// re-read cell (1) of PDF p.24's "DTCS code and polarity
			// setting" at 600 dpi and found the leaders NEST rather
			// than cross, and confirmed that assignment. ASSUMED
			// because the roles are read off artwork rather than a
			// printed byte value. Register:
			// ic905.dtcs_polarity_nibbles. Lift: ic905-R-08.
			{Field: civ.FieldDTCSPolarity, Offset: off(17), Length: 1, Encoding: civ.EncodingEnum, Enum: dtcsPolarityNames},
			// (23),(24): DTCS code, three octal digits — byte (23) is
			// "0 (fixed)" then "First digit: 0 ~ 7", byte (24) is
			// "Second digit" / "Third digit". Big-endian BCD, so code
			// 023 is 00 23 and decodes to 23.
			//
			// THE 0-7 DIGIT RANGE IS NOT ENFORCED HERE: civ's BCD
			// encoder accepts 0-9 and civ.Profile has no digit-subset
			// policy. core/driver/ic905 enforces it. See doc.go.
			{Field: civ.FieldDTCSCode, Offset: off(18), Length: 2, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// (26)~(28): Duplex offset. Little-endian BCD at 100 Hz
			// resolution: PDF p.18 (folio 17), "Duplex Offset frequency
			// setting", "Command: 0C, 0D", prints "1 kHz digit /
			// 100 Hz digit", "100 kHz / 10 kHz", "10 MHz / 1 MHz".
			{Field: civ.FieldOffset, Offset: off(21), Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 100},
			// 53~68: Memory name setting, sixteen characters fixed.
			{Field: civ.FieldName, Offset: off(48), Length: 16, Encoding: civ.EncodingName},
		},
	}
}

// modeNames is PDF p.17 (folio 16), "Operating mode", column
// "(1)Operating mode", transcribed as printed. 0x06 is absent from the
// table with no note explaining the gap; it is absent here for the same
// reason (matrix section 6, EC-4).
var modeNames = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R", 0x17: "DV",
	0x22: "DD", 0x23: "ATV",
}

// filterNames is the same table's column "(2)Filter setting".
var filterNames = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}

var dataModeNames = map[byte]string{0x00: "OFF", 0x01: "ON"}

// duplexNames is the LEFT nibble of PDF p.19 (folio 18)'s "(14): Duplex
// and Tone settings" breakout. RPS is Repeater Simplex and is
// 905-specific; the page's own note reads "RPS can be set when DD mode
// is selected, and Duplex (+, -) can be set when other than DD mode is
// selected."
var duplexNames = map[byte]string{0x0: "OFF", 0x1: "DUP-", 0x2: "DUP+", 0x3: "RPS"}

// toneModeNames is the RIGHT nibble of the same breakout: eight values,
// which is why spec.CTCSSStates' three-value vocabulary is left empty on
// this model and FieldToneMode carries it instead.
var toneModeNames = map[byte]string{
	0x0: "OFF", 0x1: "TONE", 0x2: "TSQL", 0x3: "DTCS", 0x4: "DTCS(T)",
	0x5: "TONE(T)/DTCS(R)", 0x6: "DTCS(T)/TSQL(R)", 0x7: "TONE(T)/TSQL(R)",
}

// dtcsPolarityNames spells the two directions in transmit-then-receive
// order: N for Normal, R for Reverse (PDF p.24, folio 23, cell (1),
// "Transmit polarity: 0=Normal 1=Reverse" / "Receive polarity: 0=Normal
// 1=Reverse").
var dtcsPolarityNames = map[byte]string{0x00: "NN", 0x01: "NR", 0x10: "RN", 0x11: "RR"}

// CallChannels is how many channels the CALL bank holds: twelve, two per
// band over the radio's six bands (PDF p.19, folio 18, the (3),(4)
// legend — "00 00, 00 01: 144 C1, C2" through "00 10, 00 11: 10G C1,
// C2").
const CallChannels = 12

// CallGroup is the CALL bank's WIRE group index. The radio prints and
// sends the group as `01 00`, which as two packed-BCD bytes read most
// significant first is 100 — one past the hundred memory groups and
// consecutive with them.
const CallGroup = 100

// CallSlot renders the canonical slot identifier for the n-th CALL
// channel, 0-based on the wire and 1-based in the name: "C01" .. "C12".
//
// IT IS A DISTINCT NAMESPACE FROM MEM'S, DELIBERATELY (ruling R4). MEM's
// sparse space spells itself with spec.SparseSlot's "G%02d-%03d", and
// spec.ParseSparseSlot refuses any string without a leading "G" — so no
// CALL slot can ever be read as a MEM address and no MEM address can
// ever render as a CALL slot. The disjointness is structural rather than
// an arithmetic accident of where the CALL group happens to sit, which
// is what profile_test.go proves over the whole 10,000-address space.
func CallSlot(n int) string { return fmt.Sprintf("C%02d", n+1) }

// ParseCallSlot decodes a slot string built by CallSlot, returning the
// 0-based channel index and true.
//
// STRICT, on spec.ParseSparseSlot's rule: a string is accepted only when
// re-rendering the decoded index through CallSlot reproduces it byte for
// byte, so "C1" and "C001" are refused rather than admitted as second
// names for one slot.
func ParseCallSlot(slot string) (n int, ok bool) {
	var idx int
	if _, err := fmt.Sscanf(slot, "C%d", &idx); err != nil {
		return 0, false
	}
	n = idx - 1
	if n < 0 || n >= CallChannels {
		return 0, false
	}
	if CallSlot(n) != slot {
		return 0, false
	}
	return n, true
}
