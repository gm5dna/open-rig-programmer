// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import "testing"

// namedProfile pairs a profile with a label for table-driven tests.
type namedProfile struct {
	name string
	p    Profile
}

// allTestProfiles is every CONFIGURED profile this package can see. Tests
// asserting a property that must hold of ANY profile walk this, so adding
// a profile later cannot quietly skip them.
//
// The zero Profile is deliberately absent: it is unconfigured by design
// and its property is the opposite one — it must do nothing at all —
// tested separately by TestZeroProfileIsInert.
//
// THE FIXTURES ARE BUILT TO DISAGREE, and that is the whole reason this
// file exists. It is the M9b lesson from core/cat's seconddialect_test.go,
// restated for a package with no real model tables yet: a method that
// takes a Profile and consults a package-level datum has the shape of a
// seam and none of the substance, and while every fixture agrees with
// every other, NO ordinary test can tell the two apart. So the three below
// disagree at every attribute a Profile carries:
//
//	                 flatProfile   groupProfile     bandProfile   wideProfile
//	radio address    0x94          0x70             0xa2          0x88
//	CONTROLLER addr  0xe0          0xe1             0xe0          0xe0
//	address form     flat          group x channel  band x chan   WIDE group x chan
//	address bytes    2             3                3             4
//	groups (count)   0             100              3             100
//	GROUP BASE       n/a           0                0             1
//	channels         1..99         0..99            1..99         1..99
//	name length      10            16               none          4
//	name pad         ' '           '_'              n/a           ' '
//	record lengths   {37}          {30, 31}         {8}           {12}
//	max frame        64            128              18 (== need)  64
//
// THE WIDE FIXTURE CARRIES BOTH OF E4'S NEW FACTS AT ONCE, deliberately:
// a two-byte group index AND a non-zero base. Its groups run 1..100, so
// the last of them is the index one packed-BCD byte cannot hold — the
// IC-705/IC-905 CALL group's shape — while its first is not zero, which
// is the IC-9700's shape. A base-aware rule that had quietly kept a zero
// base, or a wide encoder that had quietly kept one group byte, fails on
// this fixture and on no other.
//
// THE CONTROLLER ADDRESS ROW IS THE LOAD-BEARING ONE. 0xE0 is the CI-V
// convention and appears as a package constant
// (ControllerAddressDefault), which makes it exactly the datum a method
// is most likely to reach for directly instead of through its receiver.
// groupProfile answers to 0xE1, so any such reach fails here and nowhere
// else.
func allTestProfiles() []namedProfile {
	return []namedProfile{
		{"flatProfile", flatProfile},
		{"groupProfile", groupProfile},
		{"bandProfile", bandProfile},
		{"wideProfile", wideProfile},
	}
}

// mustFixtureProfile builds a fixture profile, failing the whole package's
// tests loudly at init if the fixture itself is malformed — a fixture that
// silently became the zero Profile would make every table-driven test
// below vacuous.
func mustFixtureProfile(cfg ProfileConfig) Profile {
	p, err := NewProfile(cfg)
	if err != nil {
		panic("civ: malformed test fixture profile " + cfg.Model + ": " + err.Error())
	}
	return p
}

// flatCharset is the flat profile's name charset: upper case, digits,
// space and two punctuation marks. Space is INCLUDED and is also the pad
// byte, which is the case spec D5 entry 3 — the name pad byte and space
// handling — flags as the awkward one.
const flatCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 -/"

// flatProfile is a fictional flat-addressed radio with a wide record: a
// frequency pair, a nibble-shared filter/data-mode byte, the whole tone
// and DTCS group, a scaled offset and a name.
var flatProfile = mustFixtureProfile(ProfileConfig{
	Model:         "TEST-FLAT",
	RadioAddress:  0x94,
	MaxFrame:      64,
	AddressForm:   AddressFormFlat,
	ChannelLo:     1,
	ChannelHi:     99,
	NameLength:    10,
	NameCharset:   flatCharset,
	NamePad:       ' ',
	Discriminator: DiscriminatorSingleLength,
	BuildLength:   37,
	Layouts: []RecordLayout{{
		Length: 37,
		Fields: []FieldSpan{
			{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
			{Field: FieldTXFrequency, Offset: 5, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
			{Field: FieldMode, Offset: 10, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "LSB", 0x01: "USB", 0x03: "CW", 0x05: "FM",
			}},
			{Field: FieldFilter, Offset: 11, Length: 1, Nibble: NibbleHigh, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "FIL1", 0x01: "FIL2", 0x02: "FIL3",
			}},
			{Field: FieldDataMode, Offset: 11, Length: 1, Nibble: NibbleLow, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "OFF", 0x01: "ON",
			}},
			{Field: FieldToneMode, Offset: 12, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "OFF", 0x01: "TONE", 0x02: "TSQL", 0x03: "DTCS",
			}},
			{Field: FieldToneTX, Offset: 13, Length: 3, Encoding: EncodingBCDNumber, Order: OrderBigEndian, Scale: 1},
			{Field: FieldToneRX, Offset: 16, Length: 3, Encoding: EncodingBCDNumber, Order: OrderBigEndian, Scale: 1},
			{Field: FieldDTCSCode, Offset: 19, Length: 2, Encoding: EncodingBCDNumber, Order: OrderBigEndian, Scale: 1},
			{Field: FieldDTCSPolarity, Offset: 21, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "NN", 0x01: "NR", 0x02: "RN", 0x03: "RR",
			}},
			{Field: FieldDuplex, Offset: 22, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "OFF", 0x10: "DUP-", 0x20: "DUP+",
			}},
			{Field: FieldOffset, Offset: 23, Length: 3, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 100},
			{Field: FieldSelect, Offset: 26, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{
				0x00: "OFF", 0x01: "SEL1", 0x02: "SEL2",
			}},
			{Field: FieldName, Offset: 27, Length: 10, Encoding: EncodingName},
		},
	}},
})

// groupProfile is the DISAGREEING fixture. Its controller address is 0xE1
// rather than the CI-V convention's 0xE0; it addresses channels by group;
// its name is 16 bytes padded with '_' rather than 10 padded with space;
// and it accepts TWO record lengths, the IC-905's documented shape, with
// the shorter carrying five-byte frequencies and the longer six.
var groupProfile = mustFixtureProfile(ProfileConfig{
	Model:             "TEST-GROUP",
	RadioAddress:      0x70,
	ControllerAddress: 0xE1,
	MaxFrame:          128,
	AddressForm:       AddressFormGroupChannel,
	Groups:            100,
	ChannelLo:         0,
	ChannelHi:         99,
	NameLength:        16,
	NameCharset:       "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_",
	NamePad:           '_',
	Discriminator:     DiscriminatorRecordLength,
	BuildLength:       31,
	Layouts: []RecordLayout{
		{
			Length: 30,
			Fields: []FieldSpan{
				{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
				{Field: FieldTXFrequency, Offset: 5, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
				{Field: FieldMode, Offset: 10, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x01: "USB", 0x07: "DV"}},
				{Field: FieldFilter, Offset: 11, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x01: "W", 0x02: "M", 0x03: "N"}},
				{Field: FieldSelect, Offset: 12, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x01: "ON"}},
				{Field: FieldName, Offset: 13, Length: 16, Encoding: EncodingName},
				{Field: FieldDuplex, Offset: 29, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x01: "DUP-", 0x02: "DUP+"}},
			},
		},
		{
			Length: 31,
			Fields: []FieldSpan{
				{Field: FieldRXFrequency, Offset: 0, Length: 6, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
				{Field: FieldTXFrequency, Offset: 6, Length: 6, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
				{Field: FieldMode, Offset: 12, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x01: "USB", 0x07: "DV"}},
				{Field: FieldFilter, Offset: 13, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x01: "W", 0x02: "M", 0x03: "N"}},
				{Field: FieldSelect, Offset: 14, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x01: "ON"}},
				{Field: FieldName, Offset: 15, Length: 16, Encoding: EncodingName},
			},
		},
	},
})

// bandProfile is the minimal fixture: band x channel addressing, NO name
// field at all, and a short record with bytes NO field maps. A profile
// with no name is not a curiosity — it is what any model whose memory
// record omits one looks like, and it is the fixture that catches a name
// codec assuming there is always a name to write.
//
// ITS MaxFrame IS EXACTLY ITS OWN NEED, which is the third reason. V9
// permits MaxFrame == 7 + addressBytes + longest record, and this fixture
// takes that permission at its word: 7 + 3 + 8 = 18, so its memory set is
// 18 bytes and its bound is 18. A model package that computes its ceiling
// exactly ships that shape, and an accumulator off by one in its frame
// bound would discard this profile's own set — and the answer to it —
// as contamination while its own gate admitted the frame.
//
// ITS UNMAPPED BYTES ARE THE OTHER REASON IT EXISTS. Byte 6 is a Fixed
// template constant and byte 7 is unmapped and zero, so this is the only
// fixture that can show the gate refusing a record byte no builder would
// have written. The other two map every byte of their records, which
// makes the re-encode rule untestable on them — a gap the first cut of
// the gate tests found by admitting a mutation it should have refused.
var bandProfile = mustFixtureProfile(ProfileConfig{
	Model:         "TEST-BAND",
	RadioAddress:  0xA2,
	MaxFrame:      18,
	AddressForm:   AddressFormBandChannel,
	Groups:        3,
	ChannelLo:     1,
	ChannelHi:     99,
	Discriminator: DiscriminatorSingleLength,
	BuildLength:   8,
	Layouts: []RecordLayout{{
		Length: 8,
		Fields: []FieldSpan{
			{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
			{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x05: "FM"}},
		},
		// Byte 6 is a documented constant; byte 7 is reserved and zero.
		Fixed: []byte{0, 0, 0, 0, 0, 0, 0x5A, 0x00},
	}},
})

// wideProfile is E4's fixture: a TWO-byte packed-BCD group index and a
// base of 1, so its group space is 1..100 — the last index unreachable in
// one BCD byte at any base, and the first index not zero.
//
// It is in allTestProfiles(), so every property this package states about
// "any profile" is now stated about a wide, base-1 one too. That is the
// fixture's real job: before it, both halves of the address code could
// have kept a hardcoded 1-byte group and a hardcoded zero base and every
// test in the package would still have passed.
var wideProfile = mustFixtureProfile(ProfileConfig{
	Model:         "TEST-WIDE",
	RadioAddress:  0x88,
	MaxFrame:      64,
	AddressForm:   AddressFormWideGroupChannel,
	Groups:        100,
	GroupBase:     1,
	ChannelLo:     1,
	ChannelHi:     99,
	NameLength:    4,
	NameCharset:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 ",
	NamePad:       ' ',
	Discriminator: DiscriminatorSingleLength,
	BuildLength:   12,
	Layouts: []RecordLayout{{
		Length: 12,
		Fields: []FieldSpan{
			{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
			{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x01: "USB", 0x07: "DV"}},
			{Field: FieldDuplex, Offset: 6, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x10: "DUP-", 0x20: "DUP+"}},
			{Field: FieldSelect, Offset: 7, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x01: "ON"}},
			{Field: FieldName, Offset: 8, Length: 4, Encoding: EncodingName},
		},
	}},
})

// unmappedRecordBytes returns the offsets of p's length-byte record that
// no field span claims.
func unmappedRecordBytes(t *testing.T, p Profile, length int) []int {
	t.Helper()
	layout, ok := p.LayoutFor(length)
	if !ok {
		t.Fatalf("%s has no layout for length %d", p.Model(), length)
	}
	mapped := make(map[int]bool, length)
	for _, sp := range layout.Fields {
		for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
			mapped[off] = true
		}
	}
	var out []int
	for i := 0; i < length; i++ {
		if !mapped[i] {
			out = append(out, i)
		}
	}
	return out
}

// sampleRecord builds a record every one of p's mapped fields is present
// in and no unmapped field is — the shape encodeRecord requires — using
// only p's own exported data. It is the in-package twin of what
// civtest.Run does through the exported API.
func sampleRecord(t *testing.T, p Profile, length int) MemoryRecord {
	t.Helper()
	rec := MemoryRecord{Address: sampleAddress(p)}
	layout, ok := p.LayoutFor(length)
	if !ok {
		t.Fatalf("%s has no layout for length %d", p.Model(), length)
	}
	for _, sp := range layout.Fields {
		switch sp.Encoding {
		case EncodingBCDNumber:
			// A wire value using most of the field's own digit positions,
			// scaled up to the neutral unit — so it is a multiple of the
			// scale by construction and always fits.
			capacity := uint64(1)
			for i := 0; i < 2*sp.Length; i++ {
				capacity *= 10
			}
			wire := (capacity - 1) / 3
			rec.setNumeric(sp.Field, wire*sp.Scale)
		case EncodingEnum:
			names := sortedEnumNames(sp.Enum)
			rec.setText(sp.Field, names[len(names)-1])
		case EncodingName:
			rec.setText(sp.Field, sampleName(p))
		}
	}
	return rec
}

// sampleAddress returns an address valid under p's own address form —
// including its own group BASE, which is not necessarily zero and not
// necessarily one.
func sampleAddress(p Profile) ChannelAddress {
	lo, _ := p.ChannelRange()
	a := ChannelAddress{Channel: lo}
	if p.AddressForm() != AddressFormFlat {
		// The base itself, so this works for a single-group profile too.
		a.Group = p.GroupBase()
	}
	return a
}

// sampleName returns a name valid under p, never ending in the pad byte
// (which would come back trimmed and not round-trip exactly).
func sampleName(p Profile) string {
	n := p.NameLength()
	if n == 0 {
		return ""
	}
	charset := p.NameCharset()
	var out []byte
	for _, b := range charset {
		if b == p.NamePad() {
			continue
		}
		out = append(out, b)
		if len(out) == n {
			break
		}
	}
	return string(out)
}
