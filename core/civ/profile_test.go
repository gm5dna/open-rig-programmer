// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// TestZeroProfileIsInert holds the zero Profile to its one property: it
// describes no radio, so it builds nothing, parses nothing and — the
// property that matters — ADMITS NOTHING.
//
// An exported struct always has a constructible zero value, so
// `var p civ.Profile` compiles and p.AllowedCommand is a perfectly
// non-nil method value: a gate that LOOKS installable while describing no
// radio at all. This is the test that says what it does instead.
func TestZeroProfileIsInert(t *testing.T) {
	var zero Profile

	if zero.Configured() {
		t.Fatal("the zero Profile reports Configured() == true — every refusal below rests on it not being configured")
	}
	if zero.Model() != "" {
		t.Errorf("zero profile: Model() = %q, want empty", zero.Model())
	}
	if n := len(zero.RecordLengths()); n != 0 {
		t.Errorf("zero profile: RecordLengths() has %d entries, want none", n)
	}
	if n := len(zero.Layouts()); n != 0 {
		t.Errorf("zero profile: Layouts() has %d entries, want none", n)
	}

	// Every builder refuses, returning the zero Command with its error.
	builders := []struct {
		what string
		cmd  Command
		err  error
	}{}
	add := func(what string, c Command, err error) {
		builders = append(builders, struct {
			what string
			cmd  Command
			err  error
		}{what, c, err})
	}
	idCmd, idErr := zero.BuildTransceiverIDRead()
	add("BuildTransceiverIDRead", idCmd, idErr)
	rdCmd, rdErr := zero.BuildMemoryRead(ChannelAddress{Channel: 1})
	add("BuildMemoryRead", rdCmd, rdErr)
	stCmd, stErr := zero.BuildMemorySet(MemoryRecord{})
	add("BuildMemorySet", stCmd, stErr)

	for _, b := range builders {
		if b.err == nil {
			t.Errorf("zero profile: %s SUCCEEDED, emitting %s — an unconfigured profile must build nothing", b.what, b.cmd)
			continue
		}
		if !b.cmd.IsZero() {
			t.Errorf("zero profile: %s returned a non-zero Command alongside its error", b.what)
		}
	}

	// Every parser refuses.
	if _, err := zero.ParseTransceiverID([]byte{0xFE, 0xFE, 0xE0, 0x94, 0x19, 0x00, 0x94, 0xFD}); err == nil {
		t.Error("zero profile: ParseTransceiverID accepted a well-formed answer — it has no address to attribute it to")
	}
	if _, err := zero.ParseMemoryAnswer([]byte{0xFE, 0xFE, 0xE0, 0x94, 0x1A, 0x00, 0x00, 0x01, 0x00, 0xFD}); err == nil {
		t.Error("zero profile: ParseMemoryAnswer accepted a memory answer — it has no layout to decode one with")
	}
	if _, _, err := zero.MemoryAnswerRecord([]byte{0xFE, 0xFE, 0xE0, 0x94, 0x1A, 0x00, 0x00, 0x01, 0x00, 0xFD}); err == nil {
		t.Error("zero profile: MemoryAnswerRecord split an answer — it has no address geometry to split one with")
	}

	// THE GATE. Every frame offered, including frames the CONFIGURED
	// fixtures build and their own gates admit.
	offered := [][]byte{
		{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
		{0xFE, 0xFE, 0x94, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFD},
		{0xFE, 0xFE, 0xE0, 0x94, AckByte, 0xFD},
	}
	for _, np := range allTestProfiles() {
		cmd, err := np.p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("%s: BuildTransceiverIDRead: %v", np.name, err)
		}
		offered = append(offered, cmd.Bytes())
	}
	refused := 0
	for _, frame := range offered {
		if zero.AllowedCommand(frame) {
			t.Errorf("zero profile: its gate ADMITTED %s — an unconfigured profile must authorise nothing, or a program holding one could put bytes on a wire to a radio it cannot describe", hexFrame(frame))
			continue
		}
		refused++
	}
	if refused != len(offered) {
		t.Errorf("zero profile: %d of %d offered frames were refused", refused, len(offered))
	}
	if refused == 0 {
		t.Error("zero profile: nothing was offered to the gate at all — this check would pass on a gate that admits everything")
	}
	t.Logf("zero profile: %d builders refused, %d frames refused at the gate", len(builders), refused)
}

// TestProfileAccessorsReportTheirOwnData walks every fixture and checks
// each accessor reports THAT profile's datum. It is the cheap half of the
// receiver-is-load-bearing property; the behavioural half is every
// table-driven test in this package running over allTestProfiles.
func TestProfileAccessorsReportTheirOwnData(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			if !p.Configured() {
				t.Fatal("fixture is not configured")
			}
			if p.Model() == "" {
				t.Error("Model() is empty")
			}
			if p.RadioAddress() == 0 {
				t.Error("RadioAddress() is 0")
			}
			if p.ControllerAddress() == 0 {
				t.Error("ControllerAddress() is 0")
			}
			if p.MaxFrame() <= 0 {
				t.Errorf("MaxFrame() = %d", p.MaxFrame())
			}
			if p.AddressForm() == AddressFormUnspecified {
				t.Error("AddressForm() is unspecified")
			}
			lengths := p.RecordLengths()
			if len(lengths) == 0 {
				t.Fatal("RecordLengths() is empty")
			}
			for i := 1; i < len(lengths); i++ {
				if lengths[i] <= lengths[i-1] {
					t.Fatalf("RecordLengths() = %v, want strictly ascending", lengths)
				}
			}
			found := false
			for _, n := range lengths {
				if n == p.BuildRecordLength() {
					found = true
				}
			}
			if !found {
				t.Errorf("BuildRecordLength() = %d is not in RecordLengths() %v", p.BuildRecordLength(), lengths)
			}
		})
	}
}

// TestProfileAccessorsReturnCopies pins that a caller cannot edit a
// constructed profile after the fact. A Profile is consulted by the
// outbound gate on every write, and a gate whose data can be edited by
// whoever holds it is not a gate.
func TestProfileAccessorsReturnCopies(t *testing.T) {
	p := flatProfile

	lengths := p.RecordLengths()
	lengths[0] = 999
	if got := p.RecordLengths(); got[0] == 999 {
		t.Error("RecordLengths() shares its slice with the profile")
	}

	layouts := p.Layouts()
	if len(layouts) == 0 {
		t.Fatal("Layouts() is empty")
	}
	layouts[0].Length = 999
	layouts[0].Fields[0].Offset = 999
	for k := range layouts[0].Fields[2].Enum {
		layouts[0].Fields[2].Enum[k] = "MUTATED"
	}
	again := p.Layouts()
	if again[0].Length == 999 || again[0].Fields[0].Offset == 999 {
		t.Error("Layouts() shares its slices with the profile")
	}
	for _, name := range again[0].Fields[2].Enum {
		if name == "MUTATED" {
			t.Error("Layouts() shares its enum maps with the profile — one caller's mutation would become every radio's mode table")
		}
	}

	cs := p.NameCharset()
	if len(cs) == 0 {
		t.Fatal("NameCharset() is empty for a profile with a name field")
	}
	cs[0] = 0xFF
	if p.NameCharset()[0] == 0xFF {
		t.Error("NameCharset() shares its slice with the profile")
	}
}

// TestNewProfile_Validation walks every rule, asserting that the failure
// names the FIELD and the OFFENDING VALUE and wraps ErrInvalidProfile.
func TestNewProfile_Validation(t *testing.T) {
	// base is a minimal VALID config; each case perturbs exactly one thing.
	base := func() ProfileConfig {
		return ProfileConfig{
			Model:         "BASE",
			RadioAddress:  0x94,
			AddressForm:   AddressFormFlat,
			ChannelLo:     1,
			ChannelHi:     99,
			NameLength:    4,
			NameCharset:   "ABC ",
			NamePad:       ' ',
			Discriminator: DiscriminatorSingleLength,
			BuildLength:   10,
			Layouts: []RecordLayout{{
				Length: 10,
				Fields: []FieldSpan{
					{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
					{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB"}},
					{Field: FieldName, Offset: 6, Length: 4, Encoding: EncodingName},
				},
			}},
		}
	}
	if _, err := NewProfile(base()); err != nil {
		t.Fatalf("the base config must be valid, got %v", err)
	}

	for _, tc := range []struct {
		name    string
		mutate  func(*ProfileConfig)
		wantSub string
	}{
		{"empty model", func(c *ProfileConfig) { c.Model = "" }, "Model"},
		{"radio address 0", func(c *ProfileConfig) { c.RadioAddress = 0 }, "RadioAddress"},
		{"radio address is the preamble byte", func(c *ProfileConfig) { c.RadioAddress = PreambleByte }, "RadioAddress"},
		{"radio address is the terminator", func(c *ProfileConfig) { c.RadioAddress = EndByte }, "RadioAddress"},
		{"controller address is the terminator", func(c *ProfileConfig) { c.ControllerAddress = EndByte }, "ControllerAddress"},
		{"radio and controller share an address", func(c *ProfileConfig) { c.RadioAddress = ControllerAddressDefault }, "RadioAddress"},
		{"max frame too small for the built frame", func(c *ProfileConfig) { c.MaxFrame = 12 }, "MaxFrame"},
		{"unspecified address form", func(c *ProfileConfig) { c.AddressForm = AddressFormUnspecified }, "AddressForm"},
		{"flat form with groups", func(c *ProfileConfig) { c.Groups = 4 }, "Groups"},
		{"group form with no groups", func(c *ProfileConfig) {
			c.AddressForm = AddressFormGroupChannel
			c.Groups = 0
		}, "Groups"},
		{"group count past the BCD field", func(c *ProfileConfig) {
			c.AddressForm = AddressFormGroupChannel
			c.Groups = 101
		}, "Groups"},
		{"inverted channel range", func(c *ProfileConfig) { c.ChannelLo, c.ChannelHi = 99, 1 }, "ChannelLo"},
		{"channel past the 4-digit field", func(c *ProfileConfig) { c.ChannelHi = 12345 }, "ChannelHi"},
		{"negative name length", func(c *ProfileConfig) { c.NameLength = -1 }, "NameLength"},
		{"name charset without a name field", func(c *ProfileConfig) {
			c.NameLength = 0
			c.Layouts[0].Fields = c.Layouts[0].Fields[:2]
			c.Layouts[0].Length = 6
			c.BuildLength = 6
		}, "NameCharset"},
		{"pad byte outside the charset", func(c *ProfileConfig) { c.NamePad = '!' }, "NamePad"},
		{"charset carrying the terminator", func(c *ProfileConfig) { c.NameCharset = "AB\xfd " }, "NameCharset"},
		{"charset carrying the preamble", func(c *ProfileConfig) { c.NameCharset = "AB\xfe " }, "NameCharset"},
		{"duplicate charset byte", func(c *ProfileConfig) { c.NameCharset = "AABC " }, "NameCharset"},
		{"no layouts", func(c *ProfileConfig) { c.Layouts = nil }, "Layouts"},
		{"duplicate record length", func(c *ProfileConfig) {
			c.Layouts = append(c.Layouts, c.Layouts[0])
			c.Discriminator = DiscriminatorRecordLength
		}, "Length"},
		{"single-length discriminator with two layouts", func(c *ProfileConfig) {
			second := RecordLayout{Length: 11, Fields: c.Layouts[0].Fields}
			c.Layouts = append(c.Layouts, second)
		}, "Discriminator"},
		{"record-length discriminator with one layout", func(c *ProfileConfig) {
			c.Discriminator = DiscriminatorRecordLength
		}, "Discriminator"},
		{"unspecified discriminator", func(c *ProfileConfig) { c.Discriminator = DiscriminatorUnspecified }, "Discriminator"},
		{"build length outside the accepted set", func(c *ProfileConfig) { c.BuildLength = 11 }, "BuildLength"},
		{"layout length 0", func(c *ProfileConfig) {
			c.Layouts[0].Length = 0
			c.BuildLength = 0
		}, "Length"},
		{"a layout with no fields", func(c *ProfileConfig) { c.Layouts[0].Fields = nil }, "Fields"},
		{"a field running past the record", func(c *ProfileConfig) { c.Layouts[0].Fields[0].Offset = 8 }, "Offset"},
		{"a negative offset", func(c *ProfileConfig) { c.Layouts[0].Fields[0].Offset = -1 }, "Offset"},
		{"an unknown field id", func(c *ProfileConfig) { c.Layouts[0].Fields[1].Field = FieldID("nonsense") }, "Field"},
		{"unspecified encoding", func(c *ProfileConfig) { c.Layouts[0].Fields[1].Encoding = EncodingUnspecified }, "Encoding"},
		{"a numeric field encoded as an enum", func(c *ProfileConfig) {
			c.Layouts[0].Fields[0].Encoding = EncodingEnum
			c.Layouts[0].Fields[0].Length = 1
			c.Layouts[0].Fields[0].Enum = map[byte]string{0: "X"}
		}, "rx_frequency"},
		{"a text field encoded as a number", func(c *ProfileConfig) {
			c.Layouts[0].Fields[1].Encoding = EncodingBCDNumber
			c.Layouts[0].Fields[1].Order = OrderLittleEndian
			c.Layouts[0].Fields[1].Scale = 1
			c.Layouts[0].Fields[1].Enum = nil
		}, "mode"},
		{"a BCD field with no byte order", func(c *ProfileConfig) { c.Layouts[0].Fields[0].Order = OrderUnspecified }, "Order"},
		{"a BCD field with scale 0", func(c *ProfileConfig) { c.Layouts[0].Fields[0].Scale = 0 }, "Scale"},
		{"a BCD field wider than the ceiling", func(c *ProfileConfig) {
			c.Layouts[0].Fields[0].Length = maxBCDBytes + 1
			c.Layouts[0].Length = 40
			c.BuildLength = 40
		}, "Length"},
		{"an enum field wider than a byte", func(c *ProfileConfig) { c.Layouts[0].Fields[1].Length = 2 }, "Length"},
		{"an empty enum", func(c *ProfileConfig) { c.Layouts[0].Fields[1].Enum = map[byte]string{} }, "Enum"},
		{"an enum with a duplicate name", func(c *ProfileConfig) {
			c.Layouts[0].Fields[1].Enum = map[byte]string{0x00: "LSB", 0x01: "LSB"}
		}, "Enum"},
		{"an enum with an empty name", func(c *ProfileConfig) {
			c.Layouts[0].Fields[1].Enum = map[byte]string{0x00: ""}
		}, "Enum"},
		{"an enum value that is the terminator byte", func(c *ProfileConfig) {
			c.Layouts[0].Fields[1].Enum = map[byte]string{EndByte: "BOOM"}
		}, "Enum"},
		{"an enum value that is the preamble byte", func(c *ProfileConfig) {
			c.Layouts[0].Fields[1].Enum = map[byte]string{PreambleByte: "BOOM"}
		}, "Enum"},
		{"a nibble enum whose value overflows the nibble", func(c *ProfileConfig) {
			c.Layouts[0].Fields[1].Nibble = NibbleHigh
			c.Layouts[0].Fields[1].Enum = map[byte]string{0x10: "TOOWIDE"}
		}, "Enum"},
		{"a name field of the wrong width", func(c *ProfileConfig) { c.Layouts[0].Fields[2].Length = 3 }, "NameLength"},
		{"a name field on a nibble", func(c *ProfileConfig) { c.Layouts[0].Fields[2].Nibble = NibbleLow }, "Nibble"},
		{"two whole-byte fields overlapping", func(c *ProfileConfig) { c.Layouts[0].Fields[1].Offset = 4 }, "overlap"},
		{"two fields on the same nibble", func(c *ProfileConfig) {
			c.Layouts[0].Fields = append(c.Layouts[0].Fields, FieldSpan{
				Field: FieldFilter, Offset: 5, Length: 1, Encoding: EncodingEnum,
				Enum: map[byte]string{0x00: "FIL1"},
			})
		}, "overlap"},
		{"a fixed template of the wrong length", func(c *ProfileConfig) { c.Layouts[0].Fixed = []byte{0x01} }, "Fixed"},
		{"a fixed template carrying the terminator", func(c *ProfileConfig) {
			f := make([]byte, 10)
			f[9] = EndByte
			c.Layouts[0].Fixed = f
		}, "Fixed"},
		{"a fixed template with a byte under a mapped field", func(c *ProfileConfig) {
			f := make([]byte, 10)
			f[0] = 0x01
			c.Layouts[0].Fixed = f
		}, "Fixed"},
		{"a scale that overflows the field's own width", func(c *ProfileConfig) {
			c.Layouts[0].Fields[0].Scale = math.MaxUint64/maxBCDValue(5) + 1
		}, "Scale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			_, err := NewProfile(cfg)
			if err == nil {
				t.Fatalf("NewProfile accepted a config with %s", tc.name)
			}
			if !errors.Is(err, ErrInvalidProfile) {
				t.Fatalf("error %v does not wrap ErrInvalidProfile", err)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not name %q — a validator returning a generic message from the wrong branch passes a test that only checks for non-nil", err, tc.wantSub)
			}
		})
	}
}

// TestScaleIsBoundedByItsFieldsOwnWidth is the BOUNDARY of the one
// arithmetic hole in the validator: the decoder computes raw * Scale, and
// nothing bounded the product.
//
// The limit is MaxUint64 divided by the widest value the field can hold —
// exactly at it is legal, one past it is refused, and a record read at the
// legal maximum comes back as the value it should rather than a wrapped
// one.
func TestScaleIsBoundedByItsFieldsOwnWidth(t *testing.T) {
	const width = 5
	widest := maxBCDValue(width) // 10 packed digits: 9,999,999,999
	limit := uint64(math.MaxUint64) / widest

	cfg := func(scale uint64) ProfileConfig {
		return ProfileConfig{
			Model:         "SCALE",
			RadioAddress:  0x94,
			AddressForm:   AddressFormFlat,
			ChannelLo:     1,
			ChannelHi:     99,
			Discriminator: DiscriminatorSingleLength,
			BuildLength:   width,
			Layouts: []RecordLayout{{
				Length: width,
				Fields: []FieldSpan{
					{Field: FieldRXFrequency, Offset: 0, Length: width, Encoding: EncodingBCDNumber, Order: OrderBigEndian, Scale: scale},
				},
			}},
		}
	}

	p, err := NewProfile(cfg(limit))
	if err != nil {
		t.Fatalf("NewProfile refused Scale exactly at the limit (%d): %v", limit, err)
	}
	if _, err := NewProfile(cfg(limit + 1)); err == nil {
		t.Fatalf("NewProfile accepted Scale %d, one past the limit for a %d-byte field — the decoder's raw*Scale wraps silently there", limit+1, width)
	} else if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("error %v does not wrap ErrInvalidProfile", err)
	} else if !strings.Contains(err.Error(), "Scale") {
		t.Fatalf("error %q does not name Scale", err)
	}

	// AND THE PRODUCT AT THE LIMIT IS THE RIGHT NUMBER. A profile at the
	// boundary must still read its own widest record back exactly.
	rec := MemoryRecord{Address: ChannelAddress{Channel: 1}, RXFreqHz: Available(widest * limit)}
	set, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet at the widest value the field holds: %v", err)
	}
	back, err := p.ParseMemoryAnswer(answerFor(set.Bytes()))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer at the boundary: %v", err)
	}
	if got, _ := back.RXFreqHz.Get(); got != widest*limit {
		t.Fatalf("RXFreqHz = %d, want %d — the read path wrapped at the very value validation permits", got, widest*limit)
	}
}

// TestFixedTemplateMayCarryAConstantBesideANibbleEnum is V8's widened
// rule, in both directions.
//
// A model whose record carries a documented constant in the nibble beside
// an enum was inexpressible while V8 keyed on whole bytes — and worse than
// inexpressible: a profile forced to omit the constant would have its own
// radio's records refused by the gate's re-encode, because the encoder
// writes a zero where the radio writes the constant.
func TestFixedTemplateMayCarryAConstantBesideANibbleEnum(t *testing.T) {
	cfg := func(fixed []byte) ProfileConfig {
		return ProfileConfig{
			Model:         "NIBBLE",
			RadioAddress:  0x94,
			AddressForm:   AddressFormFlat,
			ChannelLo:     1,
			ChannelHi:     99,
			Discriminator: DiscriminatorSingleLength,
			BuildLength:   3,
			Layouts: []RecordLayout{{
				Length: 3,
				Fields: []FieldSpan{
					{Field: FieldRXFrequency, Offset: 0, Length: 2, Encoding: EncodingBCDNumber, Order: OrderBigEndian, Scale: 1},
					// The HIGH nibble of byte 2 only.
					{Field: FieldMode, Offset: 2, Length: 1, Nibble: NibbleHigh, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x01: "USB"}},
				},
				Fixed: fixed,
			}},
		}
	}

	// THE UNMAPPED NIBBLE: a constant beside the enum is legal.
	p, err := NewProfile(cfg([]byte{0, 0, 0x07}))
	if err != nil {
		t.Fatalf("NewProfile refused a constant in the nibble NO span claims: %v", err)
	}

	rec := MemoryRecord{Address: ChannelAddress{Channel: 1}, RXFreqHz: Available[uint64](1234), Mode: Available("USB")}
	set, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := set.Bytes()
	record := frame[len(frame)-1-3 : len(frame)-1]
	if record[2] != 0x17 {
		t.Fatalf("record byte 2 = %#02x, want 0x17 — the enum in the high nibble and the template's constant in the low", record[2])
	}
	if !p.AllowedCommand(frame) {
		t.Fatal("its own gate refused the frame its own builder built")
	}
	back, err := p.ParseMemoryAnswer(answerFor(frame))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if back != rec {
		t.Fatalf("round trip: got %+v, want %+v", back, rec)
	}

	// And the constant is ENFORCED: altering it is a byte no builder would
	// write, so the gate's re-encode refuses it.
	mutated := copyBytes(frame)
	mutated[len(mutated)-2] = 0x18
	if p.AllowedCommand(mutated) {
		t.Error("the gate admitted a record whose documented constant was altered — the template nibble is not being re-encoded")
	}

	// THE MAPPED NIBBLE: still forbidden, and the message says WHICH half.
	if _, err := NewProfile(cfg([]byte{0, 0, 0x70})); err == nil {
		t.Fatal("NewProfile accepted a template byte under the nibble a span DOES claim — the precedence would be silent")
	} else if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("error %v does not wrap ErrInvalidProfile", err)
	} else if !strings.Contains(err.Error(), "HIGH") {
		t.Fatalf("error %q does not say which nibble is at fault", err)
	}

	// A whole-byte span is unchanged: both nibbles stay forbidden.
	if _, err := NewProfile(cfg([]byte{0x01, 0, 0})); err == nil {
		t.Fatal("NewProfile accepted a template byte under a whole-byte BCD span")
	}
}

// TestV8RefusesNibblesThatCombineIntoAFramingByte is the corner the V8
// widening opened, closed at construction.
//
// Neither half is a framing byte on its own — V6 bounds a nibble enum
// value at 0x0F and V8 refuses a template BYTE of 0xFE or 0xFD — but the
// byte the two halves finish as can be one, and 0xFE or 0xFD in a record
// cannot traverse CI-V framing at all. Left to the encoder's
// finished-bytes assert it would surface at BUILD time, per value, for a
// transcription error plainly visible in the table; and that assert's
// "cannot fire for a profile NewProfile built" would be false.
func TestV8RefusesNibblesThatCombineIntoAFramingByte(t *testing.T) {
	base := func(spans []FieldSpan, fixed []byte) ProfileConfig {
		return ProfileConfig{
			Model:         "COMBINE",
			RadioAddress:  0x94,
			AddressForm:   AddressFormFlat,
			ChannelLo:     1,
			ChannelHi:     99,
			Discriminator: DiscriminatorSingleLength,
			BuildLength:   3,
			Layouts: []RecordLayout{{
				Length: 3,
				Fields: append([]FieldSpan{
					{Field: FieldRXFrequency, Offset: 0, Length: 2, Encoding: EncodingBCDNumber, Order: OrderBigEndian, Scale: 1},
				}, spans...),
				Fixed: fixed,
			}},
		}
	}
	// TWO declared values, only the second of which combines badly: the
	// rule has to scan every value, not the first or the largest.
	modeNibble := func(nib NibbleSel, v byte) []FieldSpan {
		return []FieldSpan{{Field: FieldMode, Offset: 2, Length: 1, Nibble: nib, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB", v: "FM"}}}
	}

	for _, tc := range []struct {
		name   string
		cfg    ProfileConfig
		formed string
	}{
		{"enum 0x0f in the high nibble beside a template 0x0e", base(modeNibble(NibbleHigh, 0x0F), []byte{0, 0, 0x0E}), "0xfe"},
		{"enum 0x0f in the high nibble beside a template 0x0d", base(modeNibble(NibbleHigh, 0x0F), []byte{0, 0, 0x0D}), "0xfd"},
		{"enum 0x0e in the low nibble beside a template 0xf0", base(modeNibble(NibbleLow, 0x0E), []byte{0, 0, 0xF0}), "0xfe"},
		{"two nibble enums and NO template at all", base([]FieldSpan{
			{Field: FieldMode, Offset: 2, Length: 1, Nibble: NibbleHigh, Encoding: EncodingEnum, Enum: map[byte]string{0x0F: "FM"}},
			{Field: FieldFilter, Offset: 2, Length: 1, Nibble: NibbleLow, Encoding: EncodingEnum, Enum: map[byte]string{0x0E: "FIL1"}},
		}, nil), "0xfe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewProfile(tc.cfg)
			if err == nil {
				t.Fatal("NewProfile accepted a profile whose byte 2 finishes as a framing byte — the encoder would refuse it per value at build time instead, with no radio the profile can describe")
			}
			if !errors.Is(err, ErrInvalidProfile) {
				t.Fatalf("error %v does not wrap ErrInvalidProfile", err)
			}
			if !strings.Contains(err.Error(), tc.formed) {
				t.Fatalf("error %q does not name %s, the byte the two nibbles form", err, tc.formed)
			}
			if !strings.Contains(err.Error(), string(FieldMode)) {
				t.Fatalf("error %q does not name the field whose enum value is half of it", err)
			}
		})
	}

	// THE OTHER SIDE OF THE BOUNDARY. 0xFC is not a framing byte, so the
	// same shape one value lower is accepted — and builds, which is the
	// half of the boundary a rule written as "refuse anything near 0xFE"
	// would get wrong.
	p, err := NewProfile(base(modeNibble(NibbleHigh, 0x0F), []byte{0, 0, 0x0C}))
	if err != nil {
		t.Fatalf("NewProfile refused a combination of %#02x, which no framing byte is: %v", 0xFC, err)
	}
	rec := MemoryRecord{Address: ChannelAddress{Channel: 1}, RXFreqHz: Available[uint64](1234), Mode: Available("FM")}
	set, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v — the encoder's finished-bytes assert fired for a profile V8 accepted", err)
	}
	frame := set.Bytes()
	if got := frame[len(frame)-2]; got != 0xFC {
		t.Fatalf("record byte 2 = %#02x, want 0xfc — the enum's 0x0f in the high nibble and the template's 0x0c in the low", got)
	}
	if !p.AllowedCommand(frame) {
		t.Fatal("its own gate refused the frame its own builder built")
	}
	back, err := p.ParseMemoryAnswer(answerFor(frame))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if back != rec {
		t.Fatalf("round trip: got %+v, want %+v", back, rec)
	}
}

func TestNewProfile_DefaultsAndCopies(t *testing.T) {
	cfg := ProfileConfig{
		Model:         "DEFAULTS",
		RadioAddress:  0x94,
		AddressForm:   AddressFormFlat,
		ChannelLo:     1,
		ChannelHi:     9,
		Discriminator: DiscriminatorSingleLength,
		BuildLength:   6,
		Layouts: []RecordLayout{{
			Length: 6,
			Fields: []FieldSpan{
				{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
				{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB"}},
			},
		}},
	}
	p, err := NewProfile(cfg)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if p.ControllerAddress() != ControllerAddressDefault {
		t.Errorf("ControllerAddress() = %#02x, want the %#02x default", p.ControllerAddress(), ControllerAddressDefault)
	}
	if p.MaxFrame() != DefaultMaxFrame {
		t.Errorf("MaxFrame() = %d, want the %d default", p.MaxFrame(), DefaultMaxFrame)
	}

	// A caller mutating its input afterwards must not change the profile.
	cfg.Layouts[0].Fields[0].Offset = 99
	cfg.Layouts[0].Fields[1].Enum[0x00] = "MUTATED"
	if got := p.Layouts()[0].Fields[0].Offset; got != 0 {
		t.Errorf("mutating the config changed the profile's layout offset to %d", got)
	}
	if got := p.Layouts()[0].Fields[1].Enum[0x00]; got != "LSB" {
		t.Errorf("mutating the config changed the profile's enum to %q", got)
	}
}

func TestMustNewProfile_PanicsOnAMalformedTable(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustNewProfile returned normally for a malformed config")
		}
	}()
	MustNewProfile(ProfileConfig{})
}

func TestMustNewProfile_ReturnsAValidProfile(t *testing.T) {
	p := MustNewProfile(ProfileConfig{
		Model:         "MUST",
		RadioAddress:  0x94,
		AddressForm:   AddressFormFlat,
		ChannelLo:     1,
		ChannelHi:     9,
		Discriminator: DiscriminatorSingleLength,
		BuildLength:   5,
		Layouts: []RecordLayout{{
			Length: 5,
			Fields: []FieldSpan{
				{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
			},
		}},
	})
	if !p.Configured() {
		t.Fatal("MustNewProfile returned an unconfigured profile")
	}
}

// TestChannelAddressValidation walks each fixture's OWN address space and
// checks that the addresses its neighbours accept are refused here. Every
// fixture disagrees about the address form, so this is the property a
// package-level address check could not have.
func TestChannelAddressValidation(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			lo, hi := p.ChannelRange()

			if err := p.validAddress(ChannelAddress{Group: sampleAddress(p).Group, Channel: lo}); err != nil {
				t.Errorf("the profile's own lowest channel was refused: %v", err)
			}
			if err := p.validAddress(ChannelAddress{Group: sampleAddress(p).Group, Channel: hi}); err != nil {
				t.Errorf("the profile's own highest channel was refused: %v", err)
			}
			if err := p.validAddress(ChannelAddress{Group: sampleAddress(p).Group, Channel: hi + 1}); err == nil {
				t.Errorf("channel %d, one past this profile's own ceiling, was accepted", hi+1)
			}
			if lo > 0 {
				if err := p.validAddress(ChannelAddress{Group: sampleAddress(p).Group, Channel: lo - 1}); err == nil {
					t.Errorf("channel %d, one below this profile's own floor, was accepted", lo-1)
				}
			}

			switch p.AddressForm() {
			case AddressFormFlat:
				if err := p.validAddress(ChannelAddress{Group: 1, Channel: lo}); err == nil {
					t.Error("a flat profile accepted an address carrying a group — a group index it cannot encode would be silently dropped")
				}
			default:
				// THE BASE IS THE PROFILE'S, NOT ZERO (E4). Group carries
				// the WIRE index — what the radio prints — so a model
				// numbering its groups from 1 has no group 0, and the
				// first and last valid indices are base and
				// base+Groups-1.
				base := p.GroupBase()
				if err := p.validAddress(ChannelAddress{Group: base, Channel: lo}); err != nil {
					t.Errorf("a grouped profile refused group %d, which is its FIRST group: %v", base, err)
				}
				if base > 0 {
					if err := p.validAddress(ChannelAddress{Group: base - 1, Channel: lo}); err == nil {
						t.Errorf("group %d, one below this profile's own base of %d, was accepted", base-1, base)
					}
				}
				if err := p.validAddress(ChannelAddress{Group: base + p.Groups() - 1, Channel: lo}); err != nil {
					t.Errorf("a grouped profile refused group %d, which is its LAST group: %v", base+p.Groups()-1, err)
				}
				if err := p.validAddress(ChannelAddress{Group: -1, Channel: lo}); err == nil {
					t.Error("a grouped profile accepted group -1")
				}
				if err := p.validAddress(ChannelAddress{Group: base + p.Groups(), Channel: lo}); err == nil {
					t.Errorf("group %d, one past this profile's own %d groups from base %d, was accepted", base+p.Groups(), p.Groups(), base)
				}
			}
		})
	}
}
