// SPDX-License-Identifier: GPL-3.0-or-later

package civtest_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
)

// This package's own tests drive the suite over profiles built HERE, so
// that the suite is exercised exactly as a per-model package will exercise
// it: from outside core/civ, through the exported API alone.
//
// The two fixtures DISAGREE at every attribute, which is the point. A
// suite that quietly consulted a package-level datum — the 0xE0 controller
// address most of all — would pass on one and fail on the other, and a
// single fixture could never tell.

func flatProfile() civ.Profile {
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:         "CIVTEST-FLAT",
		RadioAddress:  0x94,
		MaxFrame:      64,
		AddressForm:   civ.AddressFormFlat,
		ChannelLo:     1,
		ChannelHi:     99,
		NameLength:    10,
		NameCharset:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 -/",
		NamePad:       ' ',
		Discriminator: civ.DiscriminatorSingleLength,
		BuildLength:   18,
		Layouts: []civ.RecordLayout{{
			Length: 18,
			Fields: []civ.FieldSpan{
				{Field: civ.FieldRXFrequency, Offset: 0, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
				{Field: civ.FieldMode, Offset: 5, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x01: "USB", 0x05: "FM"}},
				{Field: civ.FieldFilter, Offset: 6, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x01: "FIL1", 0x02: "FIL2"}},
				{Field: civ.FieldDataMode, Offset: 6, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x01: "ON"}},
				{Field: civ.FieldName, Offset: 7, Length: 10, Encoding: civ.EncodingName},
				// Byte 17 is reserved and zero: the byte that lets the
				// gate's re-encode rule have something to refuse.
			},
		}},
	})
}

// groupProfile answers to controller address 0xE1, addresses channels by
// group, pads its 16-byte names with '_' and accepts TWO record lengths.
func groupProfile() civ.Profile {
	fields := func(freqLen, base int) []civ.FieldSpan {
		return []civ.FieldSpan{
			{Field: civ.FieldRXFrequency, Offset: 0, Length: freqLen, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldTXFrequency, Offset: freqLen, Length: freqLen, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: base, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x07: "DV"}},
			{Field: civ.FieldToneMode, Offset: base + 1, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x01: "TONE"}},
			{Field: civ.FieldToneTX, Offset: base + 2, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: base + 5, Length: 16, Encoding: civ.EncodingName},
		}
	}
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:             "CIVTEST-GROUP",
		RadioAddress:      0x70,
		ControllerAddress: 0xE1,
		MaxFrame:          128,
		AddressForm:       civ.AddressFormGroupChannel,
		Groups:            100,
		ChannelLo:         0,
		ChannelHi:         99,
		NameLength:        16,
		NameCharset:       "abcdefghijklmnopqrstuvwxyz0123456789_",
		NamePad:           '_',
		Discriminator:     civ.DiscriminatorRecordLength,
		BuildLength:       33,
		Layouts: []civ.RecordLayout{
			{Length: 31, Fields: fields(5, 10)},
			{Length: 33, Fields: fields(6, 12)},
		},
	})
}

// bandProfile has NO name field, a band-addressed channel space, and a
// documented constant in the nibble BESIDE an enum — core/civ's V8 lets a
// layout say that, and this is the fixture that makes the mutation check
// prove the gate re-encodes such a nibble.
func bandProfile() civ.Profile {
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:         "CIVTEST-BAND",
		RadioAddress:  0xA2,
		MaxFrame:      32,
		AddressForm:   civ.AddressFormBandChannel,
		Groups:        3,
		ChannelLo:     1,
		ChannelHi:     99,
		Discriminator: civ.DiscriminatorSingleLength,
		BuildLength:   8,
		Layouts: []civ.RecordLayout{{
			Length: 8,
			Fields: []civ.FieldSpan{
				{Field: civ.FieldRXFrequency, Offset: 0, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
				{Field: civ.FieldMode, Offset: 5, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x05: "FM"}},
				// Byte 6's HIGH nibble is this enum; its LOW nibble is the
				// template's 0xA, and byte 7 is reserved and zero.
				{Field: civ.FieldFilter, Offset: 6, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x01: "FIL1", 0x03: "FIL3"}},
			},
			Fixed: []byte{0, 0, 0, 0, 0, 0, 0x0A, 0x00},
		}},
	})
}

// bankProfile pins that the conformance suite treats a bank index as a
// grouped, one-byte address without collapsing its semantic identity into
// the existing band form.
func bankProfile() civ.Profile {
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:         "CIVTEST-BANK",
		RadioAddress:  0x89,
		MaxFrame:      32,
		AddressForm:   civ.AddressFormBankChannel,
		Groups:        5,
		GroupBase:     1,
		ChannelLo:     1,
		ChannelHi:     99,
		Discriminator: civ.DiscriminatorSingleLength,
		BuildLength:   6,
		Layouts: []civ.RecordLayout{{
			Length: 6,
			Fields: []civ.FieldSpan{
				{Field: civ.FieldRXFrequency, Offset: 0, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
				{Field: civ.FieldMode, Offset: 5, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "FM"}},
			},
		}},
	})
}

// modeProfile has a six-byte common head and two eight-byte layouts with
// disagreeing tails. Length therefore cannot select FM from DCR.
func modeProfile() civ.Profile {
	head := func(values map[byte]string) []civ.FieldSpan {
		return []civ.FieldSpan{
			{Field: civ.FieldRXFrequency, Offset: 0, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 5, Length: 1, Encoding: civ.EncodingEnum, Enum: values},
		}
	}
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:         "CIVTEST-MODE",
		RadioAddress:  0x96,
		MaxFrame:      32,
		AddressForm:   civ.AddressFormFlat,
		ChannelLo:     0,
		ChannelHi:     9,
		Discriminator: civ.DiscriminatorModeByte,
		ModeKey:       civ.FieldSpan{Field: civ.FieldMode, Offset: 5, Length: 1, Encoding: civ.EncodingEnum},
		Layouts: []civ.RecordLayout{
			{Length: 6, ModeClass: "NONE", ModeValues: []byte{0}, Fields: head(map[byte]string{0: "AM"})},
			{Length: 8, ModeClass: "FM", ModeValues: []byte{1}, Fields: append(head(map[byte]string{1: "FM"}), civ.FieldSpan{Field: civ.FieldFilter, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "W"}})},
			{Length: 8, ModeClass: "DCR", ModeValues: []byte{2}, Fields: append(head(map[byte]string{2: "DCR"}), civ.FieldSpan{Field: civ.FieldDataMode, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "OFF"}})},
		},
	})
}

// wideProfile is the fixture E4 adds: a FOUR-byte address field (two
// packed-BCD group bytes before the channel pair) and a group base of 1,
// so its groups run 1..100.
//
// IT IS THE ONE THE SUITE ITSELF COULD NOT SEE. Until E4 the suite
// hardcoded group ZERO in both of its address-sampling paths, which every
// fixture before this one happened to have. A model numbering its groups
// from 1 — the IC-9700 — would have had the conformance suite ask its
// builder for group 0 and then report the refusal as a conformance
// failure, on a profile that was correct.
func wideProfile() civ.Profile {
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:         "CIVTEST-WIDE",
		RadioAddress:  0x88,
		MaxFrame:      64,
		AddressForm:   civ.AddressFormWideGroupChannel,
		Groups:        100,
		GroupBase:     1,
		ChannelLo:     1,
		ChannelHi:     99,
		NameLength:    4,
		NameCharset:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 ",
		NamePad:       ' ',
		Discriminator: civ.DiscriminatorSingleLength,
		BuildLength:   12,
		Layouts: []civ.RecordLayout{{
			Length: 12,
			Fields: []civ.FieldSpan{
				{Field: civ.FieldRXFrequency, Offset: 0, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
				{Field: civ.FieldMode, Offset: 5, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x07: "DV"}},
				{Field: civ.FieldDuplex, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "OFF", 0x10: "DUP-", 0x20: "DUP+"}},
				{Field: civ.FieldName, Offset: 7, Length: 4, Encoding: civ.EncodingName},
				// Byte 11 is reserved and zero.
			},
		}},
	})
}

func TestRun_OverDisagreeingProfiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    civ.Profile
	}{
		{"flat", flatProfile()},
		{"group (controller 0xE1, two record lengths)", groupProfile()},
		{"band (no name field)", bandProfile()},
		{"bank (distinct three-byte bank/channel form)", bankProfile()},
		{"mode byte (same length, disagreeing tails)", modeProfile()},
		{"wide (four-byte address, groups numbered from 1)", wideProfile()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			civtest.Run(t, tc.p)
		})
	}
}

func TestRunZeroValue(t *testing.T) {
	civtest.RunZeroValue(t)
}

// TestRunRefusesAnUnconfiguredProfile is the suite's own vacuity-trap
// proof, and the reason civtest.Run takes an interface rather than a
// *testing.T.
//
// A model package whose exported profile was never initialised — a failed
// init, a typo selecting the wrong var — reaches Run looking exactly like
// a radio. If Run silently switched to the refusal suite, that package
// would get a GREEN conformance report for a radio it cannot describe.
// This test is what says it does not.
func TestRunRefusesAnUnconfiguredProfile(t *testing.T) {
	rec := &recorder{}
	rec.run(func() { civtest.Run(rec, civ.Profile{}) })

	if !rec.fatal {
		t.Fatal("civtest.Run accepted an UNCONFIGURED profile — a model package whose exported var was never initialised would get a green conformance report for a radio it cannot describe")
	}
	if rec.errors != 0 {
		t.Errorf("Run reported %d ordinary failures before refusing — the refusal must be the FIRST thing it does, or a misuse looks like a conformance failure", rec.errors)
	}
}

// TestRecorderSeesOrdinaryFailures keeps the test above honest: the
// recorder must be capable of observing a plain Errorf, or "rec.errors
// == 0" would prove nothing.
func TestRecorderSeesOrdinaryFailures(t *testing.T) {
	rec := &recorder{}
	rec.run(func() { rec.Errorf("boom") })
	if rec.errors != 1 {
		t.Fatalf("the recorder saw %d errors, want 1 — it cannot observe failures, so the vacuity proof above is vacuous itself", rec.errors)
	}
	if rec.fatal {
		t.Error("the recorder reported a fatal for an ordinary Errorf")
	}
}

// recorder is the smallest thing satisfying civtest.T. Fatal and Fatalf
// unwind via panic, as *testing.T's do via runtime.Goexit.
type recorder struct {
	fatal  bool
	errors int
}

func (r *recorder) Helper()                           {}
func (r *recorder) Logf(string, ...any)               {}
func (r *recorder) Errorf(format string, args ...any) { r.errors++ }
func (r *recorder) Fatal(args ...any)                 { r.fatal = true; panic(sentinel) }
func (r *recorder) Fatalf(format string, args ...any) { r.fatal = true; panic(sentinel) }

// run calls f, absorbing the recorder's own Fatal panic and re-panicking
// on anything else — so a genuine bug in the suite is not swallowed.
func (r *recorder) run(f func()) {
	defer func() {
		if p := recover(); p != nil && p != sentinel {
			panic(p)
		}
	}()
	f()
}

const sentinel = "civtest recorder: Fatal"
