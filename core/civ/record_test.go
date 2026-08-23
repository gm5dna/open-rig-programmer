// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"strings"
	"testing"
)

// TestRecordRoundTripsOverEveryProfileAndLength is the codec's central
// property, walked over profiles that disagree about layout, name width,
// pad byte and record length.
func TestRecordRoundTripsOverEveryProfileAndLength(t *testing.T) {
	for _, np := range allTestProfiles() {
		for _, length := range np.p.RecordLengths() {
			t.Run(np.name+"/"+itoa(length), func(t *testing.T) {
				p := np.p
				rec := sampleRecord(t, p, length)

				wire, err := p.encodeRecord(rec, length)
				if err != nil {
					t.Fatalf("encodeRecord: %v", err)
				}
				if len(wire) != length {
					t.Fatalf("encodeRecord produced %d bytes, want %d", len(wire), length)
				}
				for i, b := range wire {
					if b == PreambleByte || b == EndByte {
						t.Fatalf("encodeRecord put %#02x at offset %d — a record byte that is a framing byte splits the frame on the wire", b, i)
					}
				}

				back, err := p.decodeRecord(wire, rec.Address)
				if err != nil {
					t.Fatalf("decodeRecord(% 02x): %v", wire, err)
				}
				if back != rec {
					t.Fatalf("round trip changed the record:\n got %+v\nwant %+v", back, rec)
				}

				// And re-encoding what was decoded reproduces the same
				// bytes — the property AllowedCommand rests on.
				again, err := p.encodeRecord(back, length)
				if err != nil {
					t.Fatalf("re-encode: %v", err)
				}
				if string(again) != string(wire) {
					t.Fatalf("re-encode differs:\n got % 02x\nwant % 02x", again, wire)
				}
			})
		}
	}
}

// TestRecordUnmappedFieldsStayUnavailable pins the tri-state: a field this
// profile's layout does not map comes back Unavailable, never as a
// plausible zero.
func TestRecordUnmappedFieldsStayUnavailable(t *testing.T) {
	p := bandProfile // no name, no tone group, no duplex
	rec := sampleRecord(t, p, p.BuildRecordLength())
	wire, err := p.encodeRecord(rec, p.BuildRecordLength())
	if err != nil {
		t.Fatalf("encodeRecord: %v", err)
	}
	back, err := p.decodeRecord(wire, rec.Address)
	if err != nil {
		t.Fatalf("decodeRecord: %v", err)
	}
	for _, tc := range []struct {
		name string
		got  bool
	}{
		{"Name", back.Name.Unavailable()},
		{"ToneMode", back.ToneMode.Unavailable()},
		{"Duplex", back.Duplex.Unavailable()},
		{"TXFreqHz", back.TXFreqHz.Unavailable()},
		{"DTCSCode", back.DTCSCode.Unavailable()},
	} {
		if !tc.got {
			t.Errorf("%s is present on a profile whose layout does not map it — an absent field must be Unavailable, not a plausible zero", tc.name)
		}
	}
	if back.RXFreqHz.Unavailable() {
		t.Error("RXFreqHz is Unavailable on a profile whose layout DOES map it — this check would pass on a decoder that sets nothing")
	}
}

func TestEncodeRecord_RefusesAMappedFieldWithNoValue(t *testing.T) {
	p := flatProfile
	rec := sampleRecord(t, p, p.BuildRecordLength())
	rec.Mode = Optional[string]{} // mapped by the layout, now absent

	if _, err := p.encodeRecord(rec, p.BuildRecordLength()); err == nil {
		t.Fatal("encodeRecord accepted a record missing a field its own layout maps — the byte would go out as zero, which for an enum is a value the layout may not even define")
	}
}

func TestEncodeRecord_RefusesAValueTheLayoutDoesNotMap(t *testing.T) {
	p := bandProfile
	rec := sampleRecord(t, p, p.BuildRecordLength())
	rec.Name = Available("HELLO")

	if _, err := p.encodeRecord(rec, p.BuildRecordLength()); err == nil {
		t.Fatal("encodeRecord accepted a value this profile's layout has nowhere to put — silently dropping it would write a record the caller did not ask for")
	}
}

func TestEncodeRecord_RefusesAnUnknownEnumName(t *testing.T) {
	p := flatProfile
	rec := sampleRecord(t, p, p.BuildRecordLength())
	rec.Mode = Available("RTTY-R") // not in this profile's mode table

	_, err := p.encodeRecord(rec, p.BuildRecordLength())
	if err == nil {
		t.Fatal("encodeRecord accepted a mode this profile does not know")
	}
	if !strings.Contains(err.Error(), "RTTY-R") {
		t.Fatalf("error %q does not name the offending value", err)
	}
}

func TestEncodeRecord_RefusesAValueThatDoesNotFitOrScale(t *testing.T) {
	p := flatProfile
	length := p.BuildRecordLength()

	tooBig := sampleRecord(t, p, length)
	tooBig.RXFreqHz = Available[uint64](10_000_000_000) // 11 digits into a 5-byte field
	if _, err := p.encodeRecord(tooBig, length); err == nil {
		t.Fatal("encodeRecord accepted a frequency wider than its own field — a truncated frequency is a frame naming a different channel")
	}

	notAMultiple := sampleRecord(t, p, length)
	// The offset field's scale is 100, so 12 345 Hz is not representable.
	notAMultiple.OffsetHz = Available[uint64](12_345)
	if _, err := p.encodeRecord(notAMultiple, length); err == nil {
		t.Fatal("encodeRecord accepted a value that is not a multiple of its field's scale — rounding it would write an offset the caller did not ask for")
	}
}

func TestName_CharsetPadAndTrimming(t *testing.T) {
	for _, np := range allTestProfiles() {
		p := np.p
		if p.NameLength() == 0 {
			continue
		}
		t.Run(np.name, func(t *testing.T) {
			length := p.BuildRecordLength()
			rec := sampleRecord(t, p, length)

			// A short name is padded to the field width with THIS
			// profile's pad byte, and comes back trimmed to what went in.
			short := sampleName(p)[:1]
			rec.Name = Available(short)
			wire, err := p.encodeRecord(rec, length)
			if err != nil {
				t.Fatalf("encodeRecord: %v", err)
			}
			sp := nameSpan(t, p, length)
			for i := 1; i < sp.Length; i++ {
				if got := wire[sp.Offset+i]; got != p.NamePad() {
					t.Fatalf("name field byte %d = %#02x, want this profile's pad byte %#02x", i, got, p.NamePad())
				}
			}
			back, err := p.decodeRecord(wire, rec.Address)
			if err != nil {
				t.Fatalf("decodeRecord: %v", err)
			}
			if got, _ := back.Name.Get(); got != short {
				t.Fatalf("name round trip: got %q, want %q", got, short)
			}

			// A name too long for the field is refused, not truncated.
			rec.Name = Available(strings.Repeat(short, p.NameLength()+1))
			if _, err := p.encodeRecord(rec, length); err == nil {
				t.Fatal("encodeRecord accepted a name longer than the field")
			}

			// A byte outside this profile's charset is refused.
			rec.Name = Available("\x01")
			if _, err := p.encodeRecord(rec, length); err == nil {
				t.Fatal("encodeRecord accepted a name byte outside this profile's charset")
			}
		})
	}
}

// TestName_PadByteInsideAName is spec D5 entry 3's awkward case — the
// name pad byte and space handling. On a profile whose pad byte is also a
// legitimate name character (the space), an INTERIOR one must survive and
// a TRAILING one must not be distinguishable from padding.
func TestName_PadByteInsideAName(t *testing.T) {
	p := flatProfile
	if !p.nameByteValid(p.NamePad()) {
		t.Skip("this fixture's pad byte is not in its charset")
	}
	length := p.BuildRecordLength()
	rec := sampleRecord(t, p, length)
	rec.Name = Available("A B")

	wire, err := p.encodeRecord(rec, length)
	if err != nil {
		t.Fatalf("encodeRecord: %v", err)
	}
	back, err := p.decodeRecord(wire, rec.Address)
	if err != nil {
		t.Fatalf("decodeRecord: %v", err)
	}
	if got, _ := back.Name.Get(); got != "A B" {
		t.Fatalf("an interior pad byte was lost: got %q, want %q", got, "A B")
	}

	// A TRAILING one is indistinguishable from padding on the wire, and
	// this package says so rather than pretending otherwise.
	rec.Name = Available("AB ")
	wire, err = p.encodeRecord(rec, length)
	if err != nil {
		t.Fatalf("encodeRecord: %v", err)
	}
	back, err = p.decodeRecord(wire, rec.Address)
	if err != nil {
		t.Fatalf("decodeRecord: %v", err)
	}
	if got, _ := back.Name.Get(); got != "AB" {
		t.Fatalf("a trailing pad byte: got %q, want %q — padding erases the data-versus-fill distinction on the wire", got, "AB")
	}
}

func TestDecodeRecord_RefusesAnUnknownEnumValue(t *testing.T) {
	p := flatProfile
	length := p.BuildRecordLength()
	rec := sampleRecord(t, p, length)
	wire, err := p.encodeRecord(rec, length)
	if err != nil {
		t.Fatalf("encodeRecord: %v", err)
	}
	wire[10] = 0x0C // no mode is defined for this byte

	if _, err := p.decodeRecord(wire, rec.Address); err == nil {
		t.Fatal("decodeRecord accepted a mode byte this profile does not define — guessing at it would report a mode the radio never named")
	}
}

func TestDecodeRecord_RefusesAWrongLength(t *testing.T) {
	p := flatProfile
	rec := sampleRecord(t, p, p.BuildRecordLength())
	wire, err := p.encodeRecord(rec, p.BuildRecordLength())
	if err != nil {
		t.Fatalf("encodeRecord: %v", err)
	}

	_, err = p.decodeRecord(wire[:len(wire)-1], rec.Address)
	if err == nil {
		t.Fatal("decodeRecord accepted a record one byte short")
	}
	if !errors.Is(err, ErrRecordLength) {
		t.Fatalf("error %v does not match ErrRecordLength", err)
	}
	var rl *RecordLengthError
	if !errors.As(err, &rl) {
		t.Fatalf("error %v is not a *RecordLengthError", err)
	}
	if rl.Got != len(wire)-1 {
		t.Errorf("RecordLengthError.Got = %d, want %d", rl.Got, len(wire)-1)
	}
	if len(rl.Want) == 0 {
		t.Error("RecordLengthError.Want is empty — the error must name the accepted set")
	}
}

// TestDecodeRecord_DiscriminatesByLength is the two-length profile's own
// property: the SAME bytes are read against a DIFFERENT layout depending
// on their length, and each length round-trips against its own layout.
func TestDecodeRecord_DiscriminatesByLength(t *testing.T) {
	p := groupProfile
	lengths := p.RecordLengths()
	if len(lengths) != 2 {
		t.Fatalf("this test needs the two-length fixture, got %v", lengths)
	}

	shortRec := sampleRecord(t, p, lengths[0])
	longRec := sampleRecord(t, p, lengths[1])

	shortWire, err := p.encodeRecord(shortRec, lengths[0])
	if err != nil {
		t.Fatalf("short encode: %v", err)
	}
	longWire, err := p.encodeRecord(longRec, lengths[1])
	if err != nil {
		t.Fatalf("long encode: %v", err)
	}

	// The 30-byte layout maps a duplex field the 31-byte layout does not:
	// so the shorter record has one and the longer does not, from the same
	// profile.
	shortBack, err := p.decodeRecord(shortWire, shortRec.Address)
	if err != nil {
		t.Fatalf("short decode: %v", err)
	}
	longBack, err := p.decodeRecord(longWire, longRec.Address)
	if err != nil {
		t.Fatalf("long decode: %v", err)
	}
	if shortBack.Duplex.Unavailable() {
		t.Error("the 30-byte layout maps duplex, but the decoded record has none")
	}
	if !longBack.Duplex.Unavailable() {
		t.Error("the 31-byte layout does not map duplex, but the decoded record has one")
	}
	// And the frequency fields are genuinely different widths.
	if len(shortWire) == len(longWire) {
		t.Fatal("the two forms produced the same length")
	}
}

// TestOptional_ZeroValueIsUnavailable pins the tri-state's zero value,
// which every unmapped field relies on.
func TestOptional_ZeroValueIsUnavailable(t *testing.T) {
	var s Optional[string]
	if !s.Unavailable() {
		t.Error("the zero Optional[string] is not Unavailable")
	}
	if v, ok := s.Get(); ok || v != "" {
		t.Errorf("the zero Optional[string].Get() = %q, %v", v, ok)
	}
	if s.String() != "unavailable" {
		t.Errorf("the zero Optional[string].String() = %q", s.String())
	}

	n := Available[uint64](0)
	if n.Unavailable() {
		t.Error("Available(0) reports Unavailable — a present zero and an absent field are different things")
	}
	if v, ok := n.Get(); !ok || v != 0 {
		t.Errorf("Available(0).Get() = %d, %v", v, ok)
	}
	if n.String() != "0" {
		t.Errorf("Available[uint64](0).String() = %q", n.String())
	}
}

// TestEveryFieldIDIsReachable pins that the record's accessors are
// EXHAUSTIVE over the field vocabulary. A FieldID a layout may name but
// the record cannot store is a field that silently vanishes.
func TestEveryFieldIDIsReachable(t *testing.T) {
	ids := AllFieldIDs()
	if len(ids) == 0 {
		t.Fatal("AllFieldIDs() is empty — every check here would be vacuous")
	}
	var rec MemoryRecord
	for _, id := range ids {
		kind, ok := id.kind()
		if !ok {
			t.Errorf("%s is in AllFieldIDs() but has no kind", id)
			continue
		}
		switch kind {
		case fieldNumeric:
			rec.setNumeric(id, 7)
			got, ok := rec.numeric(id)
			if !ok {
				t.Errorf("%s: numeric() does not reach it", id)
				continue
			}
			if v, present := got.Get(); !present || v != 7 {
				t.Errorf("%s: setNumeric/numeric did not round trip (%v, %v)", id, v, present)
			}
		case fieldText:
			rec.setText(id, "X")
			got, ok := rec.text(id)
			if !ok {
				t.Errorf("%s: text() does not reach it", id)
				continue
			}
			if v, present := got.Get(); !present || v != "X" {
				t.Errorf("%s: setText/text did not round trip (%q, %v)", id, v, present)
			}
		}
	}
	// An id outside the vocabulary must be refused by both accessors,
	// rather than silently landing nowhere.
	bogus := FieldID("not_a_field")
	if _, ok := rec.numeric(bogus); ok {
		t.Error("numeric() accepted an unknown field id")
	}
	if _, ok := rec.text(bogus); ok {
		t.Error("text() accepted an unknown field id")
	}
	if _, ok := bogus.kind(); ok {
		t.Error("an unknown field id reported a kind")
	}
}

// nameSpan finds p's name field in the layout for length.
func nameSpan(t *testing.T, p Profile, length int) FieldSpan {
	t.Helper()
	layout, ok := p.LayoutFor(length)
	if !ok {
		t.Fatalf("%s has no layout for %d", p.Model(), length)
	}
	for _, sp := range layout.Fields {
		if sp.Field == FieldName {
			return sp
		}
	}
	t.Fatalf("%s has no name field at length %d", p.Model(), length)
	return FieldSpan{}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
