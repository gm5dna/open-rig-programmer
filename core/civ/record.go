// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"fmt"
	"sort"
	"strconv"
)

// Optional is a tri-state field value: PRESENT with a value, or
// UNAVAILABLE. The zero value is Unavailable.
//
// It exists because "this radio's memory record has no tone field" and
// "this channel's tone is off" are different facts, and a plain zero
// cannot tell them apart. Spec D1 requires absent fields to be
// Unavailable, and the tri-state is how a decoder says so without
// inventing a value the radio never sent.
//
// Optional[T] is comparable whenever T is, which is what lets MemoryRecord
// be compared with == in the round-trip tests.
type Optional[T comparable] struct {
	v       T
	present bool
}

// Available returns an Optional holding v. Available of a ZERO value is
// still present: a tone of 0 and no tone field at all are not the same.
func Available[T comparable](v T) Optional[T] { return Optional[T]{v: v, present: true} }

// Get returns the value and whether it is present.
func (o Optional[T]) Get() (T, bool) { return o.v, o.present }

// Unavailable reports whether this field is absent.
func (o Optional[T]) Unavailable() bool { return !o.present }

// String renders the value, or the word "unavailable".
func (o Optional[T]) String() string {
	if !o.present {
		return "unavailable"
	}
	return fmt.Sprint(o.v)
}

// FieldID names one NEUTRAL memory-record field: the vocabulary a
// per-model layout maps its bytes onto.
//
// NEUTRAL means these are the fields spec D1's MemoryRecord carries, not
// the names any one Icom document prints. A layout says "byte 10's low
// nibble is the data mode"; nothing about the mapping leaks a model's own
// numbering into this package.
type FieldID string

// The field vocabulary. It is spec D1's MemoryRecord list exactly: RX
// frequency, TX frequency or duplex+offset, mode+filter, data mode, tone
// mode, TX tone, RX tone, DTCS code+polarity, name, select/skip.
//
// Adding one is a DELIBERATE act: MemoryRecord must gain a matching field,
// the accessors below must reach it, and AllFieldIDs must list it —
// TestEveryFieldIDIsReachable fails on any of the three being missed.
const (
	FieldRXFrequency  FieldID = "rx_frequency"
	FieldTXFrequency  FieldID = "tx_frequency"
	FieldOffset       FieldID = "offset"
	FieldToneTX       FieldID = "tone_tx"
	FieldToneRX       FieldID = "tone_rx"
	FieldDTCSCode     FieldID = "dtcs_code"
	FieldDuplex       FieldID = "duplex"
	FieldMode         FieldID = "mode"
	FieldFilter       FieldID = "filter"
	FieldDataMode     FieldID = "data_mode"
	FieldToneMode     FieldID = "tone_mode"
	FieldDTCSPolarity FieldID = "dtcs_polarity"
	FieldName         FieldID = "name"
	FieldSelect       FieldID = "select"
)

// AllFieldIDs returns the whole vocabulary, in a stable order.
func AllFieldIDs() []FieldID {
	return []FieldID{
		FieldRXFrequency, FieldTXFrequency, FieldOffset,
		FieldToneTX, FieldToneRX, FieldDTCSCode,
		FieldDuplex, FieldMode, FieldFilter, FieldDataMode,
		FieldToneMode, FieldDTCSPolarity, FieldName, FieldSelect,
	}
}

// fieldKind is whether a field holds a NUMBER or TEXT in the neutral
// record. It decides which encodings a layout may use for the field, and
// which accessor reaches it.
type fieldKind int

const (
	fieldNumeric fieldKind = iota + 1
	fieldText
)

// kind reports a field's neutral kind. The bool is false for an id outside
// the vocabulary, which is how profile validation refuses a layout naming
// a field this package cannot store.
func (f FieldID) kind() (fieldKind, bool) {
	switch f {
	case FieldRXFrequency, FieldTXFrequency, FieldOffset,
		FieldToneTX, FieldToneRX, FieldDTCSCode:
		return fieldNumeric, true
	case FieldDuplex, FieldMode, FieldFilter, FieldDataMode,
		FieldToneMode, FieldDTCSPolarity, FieldName, FieldSelect:
		return fieldText, true
	default:
		return 0, false
	}
}

// AddressForm names how a model addresses a memory channel.
//
// The three forms are spec D1's: the IC-7610 numbers channels flat, the
// IC-705 and IC-905 address (group, channel), the IC-9700 addresses
// (band, channel). The zero value is not a form, so a config omitting it
// is refused rather than silently given the flat one — a grouped radio
// addressed flat reads a different channel.
type AddressForm int

const (
	// AddressFormUnspecified is the zero value: refused, never a form.
	AddressFormUnspecified AddressForm = iota
	// AddressFormFlat is a bare channel number: two packed-BCD bytes.
	AddressFormFlat
	// AddressFormGroupChannel is one packed-BCD group byte then the
	// two-byte channel.
	AddressFormGroupChannel
	// AddressFormBandChannel is one packed-BCD band byte then the two-byte
	// channel. It is a DISTINCT form from AddressFormGroupChannel despite
	// the identical wire width: the two index different things, a
	// codeplug's slot strings render them differently, and collapsing them
	// would make a band-addressed radio indistinguishable from a
	// group-addressed one in every diagnostic.
	AddressFormBandChannel
	// AddressFormBankChannel is one packed-BCD bank byte then the two-byte
	// channel. It deliberately remains distinct from BandChannel: equal
	// wire widths do not make a bank and a band the same address identity
	// (TestAddressFormBankChannelIsDistinctAndThreeBytesWide).
	AddressFormBankChannel
	// AddressFormWideGroupChannel is TWO packed-BCD group bytes, most
	// significant first, then the two-byte channel: a four-byte address
	// field.
	//
	// IT EXISTS BECAUSE A HUNDREDTH GROUP DOES NOT FIT IN ONE BYTE. The
	// IC-705 and the IC-905 both number a CALL group at wire index 100,
	// which the radio prints and sends as `01 00`; one packed-BCD byte
	// stops at 99. A model with 101 groups is not an edge case to round
	// off — it is two of the six radios in this tier — and the alternative
	// (renumbering the CALL group to something that fits) would have this
	// program address a channel by an index its radio has never heard of.
	AddressFormWideGroupChannel
)

func (f AddressForm) String() string {
	switch f {
	case AddressFormUnspecified:
		return "AddressFormUnspecified"
	case AddressFormFlat:
		return "AddressFormFlat"
	case AddressFormGroupChannel:
		return "AddressFormGroupChannel"
	case AddressFormBandChannel:
		return "AddressFormBandChannel"
	case AddressFormBankChannel:
		return "AddressFormBankChannel"
	case AddressFormWideGroupChannel:
		return "AddressFormWideGroupChannel"
	default:
		return "AddressForm(" + strconv.Itoa(int(f)) + ")"
	}
}

// addressBytes is the wire width of this form's address field. ASSUMED —
// see doc.go's register entry for the address encoding.
func (f AddressForm) addressBytes() int {
	return f.groupBytes() + 2
}

// groupBytes is the wire width of this form's GROUP or BAND index, in
// packed-BCD bytes: 0 for a flat form, 1 for the two three-byte forms, 2
// for the wide one. addressBytes is this plus the two-byte channel every
// form carries.
//
// A separate function because the group width is what VALIDATION needs —
// "does base + count − 1 fit?" is a question about the index field alone —
// and deriving it back out of addressBytes would be arithmetic standing in
// for a fact.
func (f AddressForm) groupBytes() int {
	switch f {
	case AddressFormGroupChannel, AddressFormBandChannel, AddressFormBankChannel:
		return 1
	case AddressFormWideGroupChannel:
		return 2
	default:
		// Including AddressFormUnspecified, whose addressBytes is then 2.
		// Nothing reaches either: NewProfile refuses the zero form (V3),
		// and every path here runs on a constructed profile.
		return 0
	}
}

// groupCapacity is how many DISTINCT group indices this form can encode:
// 100 for one packed-BCD byte, 10000 for two, 0 for a flat form which
// encodes none. The highest encodable index is groupCapacity() - 1.
//
// It is the FORM's half of E4's rule. The form declares what its BCD width
// can hold; the PROFILE declares the base its radio numbers from
// (ProfileConfig.GroupBase), because one form serves radios that disagree
// about that — the IC-9700 numbers its three groups 1..3 while the IC-705
// and IC-905 number theirs 0..100 — and a base baked into the form could
// not describe both. Validation joins the two halves: base + count − 1
// must be an index this width can carry.
func (f AddressForm) groupCapacity() int {
	switch f.groupBytes() {
	case 1:
		return 100
	case 2:
		return 10000
	default:
		return 0
	}
}

// grouped reports whether this form carries a group or band index.
func (f AddressForm) grouped() bool {
	return f.groupBytes() > 0
}

// AddressRange is one inclusive rectangle in a profile's address space.
// Extra ranges stay as separate rectangles so validAddress cannot silently
// admit the holes in their rectangular closure; the builder, parser and
// gate refusal is pinned by TestExtraRangesAreAUnionNotARectangularClosure.
type AddressRange struct {
	GroupLo, GroupHi     int
	ChannelLo, ChannelHi int
}

func (r AddressRange) contains(a ChannelAddress) bool {
	return a.Group >= r.GroupLo && a.Group <= r.GroupHi &&
		a.Channel >= r.ChannelLo && a.Channel <= r.ChannelHi
}

// ChannelAddress is one memory channel's address under a profile's own
// address form.
//
// Group carries the group or band index AS THE RADIO NUMBERS IT — the WIRE
// index, what the radio prints and what it sends — and must be zero under
// AddressFormFlat: a flat profile has nowhere to encode it, and silently
// dropping it would read or write a channel the caller did not name.
//
// THE EARLIER "NUMBERED FROM 0" CLAIM HERE WAS REFUTED, and by its own
// argument. It read: the index is one packed-BCD byte, and a model with
// 100 groups — the IC-705 and IC-905 — has no hundredth value to number if
// counting starts at 1, therefore indices run 0..Groups-1 on every model.
// Both halves turned out to be wrong about the radios. The 705 and 905
// have ONE HUNDRED AND ONE groups, the last of them a CALL group the radio
// itself numbers 100 and sends as `01 00` — which one packed-BCD byte
// cannot hold at ANY base, so the premise "the index is one packed-BCD
// byte" was the thing to fix (AddressFormWideGroupChannel), not the
// numbering. And the IC-9700 numbers its three groups 1, 2 and 3: a
// zero-based rule would have this program address group 1 as 0, reading
// and writing a group the operator did not name while every diagnostic
// printed the wrong number back at them.
//
// So the BASE is profile data (ProfileConfig.GroupBase, defaulting to 0,
// which is what every profile written before this had), Profile.Groups
// remains a COUNT, and the valid indices are
// GroupBase..GroupBase+Groups-1.
type ChannelAddress struct {
	Group   int
	Channel int
}

// String renders the address for diagnostics. It always prints both
// components, including a zero group: this type does not know its
// profile's address form, so it must not imply one by hiding a field.
func (a ChannelAddress) String() string {
	return "g" + strconv.Itoa(a.Group) + "/ch" + strconv.Itoa(a.Channel)
}

// MemoryRecord is one memory channel, in NEUTRAL terms: the fields spec
// D1 names, each tri-state so a model that has no such field reports
// Unavailable rather than a value it never sent.
//
// Frequencies are Hz. Tones are TENTHS of a Hz (an Icom CTCSS tone of
// 88.5 Hz is 885), because that is the unit the wire carries and rounding
// at the codec would lose the .5. DTCS codes are the printed code as a
// decimal integer.
//
// It is COMPARABLE, deliberately: the round-trip property this package
// rests on is stated as `back == rec`, and a struct carrying a slice or a
// map could not be.
type MemoryRecord struct {
	// Address is the channel this record belongs to, under the profile's
	// own address form.
	Address ChannelAddress

	RXFreqHz     Optional[uint64]
	TXFreqHz     Optional[uint64]
	OffsetHz     Optional[uint64]
	ToneTXDeciHz Optional[uint64]
	ToneRXDeciHz Optional[uint64]
	DTCSCode     Optional[uint64]

	Duplex       Optional[string]
	Mode         Optional[string]
	Filter       Optional[string]
	DataMode     Optional[string]
	ToneMode     Optional[string]
	DTCSPolarity Optional[string]
	Name         Optional[string]
	Select       Optional[string]
}

// numeric returns the numeric field id names, and whether id is a numeric
// field at all.
func (r MemoryRecord) numeric(id FieldID) (Optional[uint64], bool) {
	switch id {
	case FieldRXFrequency:
		return r.RXFreqHz, true
	case FieldTXFrequency:
		return r.TXFreqHz, true
	case FieldOffset:
		return r.OffsetHz, true
	case FieldToneTX:
		return r.ToneTXDeciHz, true
	case FieldToneRX:
		return r.ToneRXDeciHz, true
	case FieldDTCSCode:
		return r.DTCSCode, true
	default:
		return Optional[uint64]{}, false
	}
}

// setNumeric stores v in the numeric field id names. It is a no-op for a
// non-numeric id; callers reach it only after kind() has agreed.
func (r *MemoryRecord) setNumeric(id FieldID, v uint64) {
	switch id {
	case FieldRXFrequency:
		r.RXFreqHz = Available(v)
	case FieldTXFrequency:
		r.TXFreqHz = Available(v)
	case FieldOffset:
		r.OffsetHz = Available(v)
	case FieldToneTX:
		r.ToneTXDeciHz = Available(v)
	case FieldToneRX:
		r.ToneRXDeciHz = Available(v)
	case FieldDTCSCode:
		r.DTCSCode = Available(v)
	}
}

// text returns the text field id names, and whether id is a text field.
func (r MemoryRecord) text(id FieldID) (Optional[string], bool) {
	switch id {
	case FieldDuplex:
		return r.Duplex, true
	case FieldMode:
		return r.Mode, true
	case FieldFilter:
		return r.Filter, true
	case FieldDataMode:
		return r.DataMode, true
	case FieldToneMode:
		return r.ToneMode, true
	case FieldDTCSPolarity:
		return r.DTCSPolarity, true
	case FieldName:
		return r.Name, true
	case FieldSelect:
		return r.Select, true
	default:
		return Optional[string]{}, false
	}
}

// setText stores v in the text field id names.
func (r *MemoryRecord) setText(id FieldID, v string) {
	switch id {
	case FieldDuplex:
		r.Duplex = Available(v)
	case FieldMode:
		r.Mode = Available(v)
	case FieldFilter:
		r.Filter = Available(v)
	case FieldDataMode:
		r.DataMode = Available(v)
	case FieldToneMode:
		r.ToneMode = Available(v)
	case FieldDTCSPolarity:
		r.DTCSPolarity = Available(v)
	case FieldName:
		r.Name = Available(v)
	case FieldSelect:
		r.Select = Available(v)
	}
}

// EncodingKind names how a layout field's bytes carry their value.
//
// Three kinds, and no more, because three is what the whole tier's record
// geometry needs: a packed-BCD number, an enumerated byte or nibble, and a
// fixed-width name. A model needing a fourth is a change to this type with
// its own validation and its own round-trip tests, not a reinterpretation
// of one of these.
//
// The zero value is not a kind: a layout omitting it is refused rather
// than defaulting to one, since every default here would silently
// misread somebody's record.
type EncodingKind int

const (
	// EncodingUnspecified is the zero value: refused, never a kind.
	EncodingUnspecified EncodingKind = iota
	// EncodingBCDNumber is packed BCD over Length bytes, in the span's own
	// ByteOrder, multiplied by the span's Scale to reach the neutral unit.
	EncodingBCDNumber
	// EncodingEnum is one byte, or one NIBBLE of one byte, mapped to a
	// neutral name by the span's Enum.
	EncodingEnum
	// EncodingName is fixed-width ASCII text over Length bytes, using the
	// PROFILE's charset and pad byte (not the span's — a radio has one
	// name convention, and giving each span its own would let two name
	// fields on one model disagree about what a name is).
	EncodingName
)

func (e EncodingKind) String() string {
	switch e {
	case EncodingUnspecified:
		return "EncodingUnspecified"
	case EncodingBCDNumber:
		return "EncodingBCDNumber"
	case EncodingEnum:
		return "EncodingEnum"
	case EncodingName:
		return "EncodingName"
	default:
		return "EncodingKind(" + strconv.Itoa(int(e)) + ")"
	}
}

// NibbleSel selects which half of a byte an EncodingEnum span occupies.
//
// Nibble packing is everywhere in Icom records — a filter and a data mode
// sharing one byte is the common case — and a layout that could only
// address whole bytes would have to describe such a pair as one opaque
// enum with a cross product of names.
//
// The zero value is NibbleWhole, which is the safe default here (unlike
// every other zero value in this package): a span that says nothing about
// nibbles means the whole byte, which is what it looks like it means.
type NibbleSel int

const (
	// NibbleWhole is the whole byte.
	NibbleWhole NibbleSel = iota
	// NibbleHigh is bits 7..4.
	NibbleHigh
	// NibbleLow is bits 3..0.
	NibbleLow
)

func (n NibbleSel) String() string {
	switch n {
	case NibbleWhole:
		return "NibbleWhole"
	case NibbleHigh:
		return "NibbleHigh"
	case NibbleLow:
		return "NibbleLow"
	default:
		return "NibbleSel(" + strconv.Itoa(int(n)) + ")"
	}
}

// FieldSpan maps one neutral field onto a run of record bytes: spec D1's
// "field id, byte/nibble span, encoding kind, enum map".
type FieldSpan struct {
	// Field is the neutral field this span carries.
	Field FieldID
	// Offset is the span's first byte, from the start of the RECORD (not
	// of the frame).
	Offset int
	// Length is the span's width in bytes. Exactly 1 for EncodingEnum;
	// the profile's NameLength for EncodingName.
	Length int
	// Nibble selects half a byte, for EncodingEnum only.
	Nibble NibbleSel
	// Encoding is how the bytes carry the value.
	Encoding EncodingKind
	// Order is the packed-BCD byte order, for EncodingBCDNumber only.
	Order ByteOrder
	// Scale multiplies the wire value to reach the neutral unit: 1 for a
	// frequency in Hz, 100 for an offset field documented in 100 Hz units.
	// EncodingBCDNumber only, and never 0.
	Scale uint64
	// Enum maps wire values to neutral names, for EncodingEnum only. It
	// must be injective — two wire values sharing a name would make the
	// encode direction ambiguous.
	Enum map[byte]string
}

// clone returns a deep copy of the span, so a profile's layout can be
// handed out without handing out its enum map.
func (s FieldSpan) clone() FieldSpan {
	out := s
	if s.Enum != nil {
		out.Enum = make(map[byte]string, len(s.Enum))
		for k, v := range s.Enum {
			out.Enum[k] = v
		}
	}
	return out
}

// RecordLayout is one accepted record SHAPE and the field spans that
// decode it.
//
// Under the two length-keyed discriminator kinds a profile carries one
// layout per accepted length; under DiscriminatorModeByte (additions
// design D3.3) layouts are keyed by ModeClass and two may share a length
// — the IC-R8600's FM and DCR tails. The length-keyed reading is what makes
// the "accepted record lengths as a SET with a discriminator rule" of spec
// D1 decidable: the length IS the discriminator, and each length brings
// its own geometry. The IC-905's documented five- and six-byte frequency
// forms are two layouts here, not one parameterised layout — the six-byte
// form shifts every field after the frequency, so they are different
// geometries rather than one geometry with a variable.
type RecordLayout struct {
	// Length is the record's exact width in bytes.
	Length int
	// ModeClass is the neutral discriminator class for a mode-keyed
	// layout. Empty for the two length discriminators.
	ModeClass string
	// ModeValues are the wire values that select this layout. Keeping the
	// declaration beside the layout prevents a tail from being paired with
	// a mode table elsewhere by position.
	ModeValues []byte
	// Fields maps neutral fields onto this record's bytes. A field may
	// appear MORE THAN ONCE: spec D5 entry 4 records a duplicated TX
	// block that three models require on write, and two spans naming the
	// same field is how a layout says so. Encoding writes both; decoding
	// requires them to AGREE, and refuses the record if they do not.
	Fields []FieldSpan
	// Fixed is an optional template of exactly Length bytes giving the
	// value of every byte NO field span maps — a reserved marker, a
	// documented constant. Empty means those bytes are zero.
	//
	// Bytes under a mapped span must be zero here: the span decides them,
	// and a template that also claimed one would make the precedence
	// silent.
	Fixed []byte
}

// clone returns a deep copy.
func (l RecordLayout) clone() RecordLayout {
	out := RecordLayout{Length: l.Length, ModeClass: l.ModeClass, ModeValues: copyBytes(l.ModeValues)}
	if l.Fields != nil {
		out.Fields = make([]FieldSpan, len(l.Fields))
		for i, f := range l.Fields {
			out.Fields[i] = f.clone()
		}
	}
	if l.Fixed != nil {
		out.Fixed = copyBytes(l.Fixed)
	}
	return out
}

// RecordDiscriminator names the rule that decides WHICH accepted record
// length a given record has.
//
// Spec D1 asks for "accepted record lengths as a SET with a discriminator
// rule". The set is the profile's layouts; this names the rule, and it is
// declared rather than inferred so that a model table says out loud which
// case it is in — a two-layout profile that meant to have one is a
// transcription error, and an inferred rule would hide it.
type RecordDiscriminator int

const (
	// DiscriminatorUnspecified is the zero value: refused.
	DiscriminatorUnspecified RecordDiscriminator = iota
	// DiscriminatorSingleLength: exactly one accepted length. Every model
	// in this tier but the IC-905.
	DiscriminatorSingleLength
	// DiscriminatorRecordLength: two or more accepted lengths, each with
	// its own layout, told apart by the record's own length. The IC-905's
	// documented five- and six-byte frequency forms.
	DiscriminatorRecordLength
	// DiscriminatorModeByte selects a layout from ProfileConfig.ModeKey.
	// Length remains an accepted-set fingerprint but never identifies a
	// layout, because two modes may intentionally share it.
	DiscriminatorModeByte
)

func (d RecordDiscriminator) String() string {
	switch d {
	case DiscriminatorUnspecified:
		return "DiscriminatorUnspecified"
	case DiscriminatorSingleLength:
		return "DiscriminatorSingleLength"
	case DiscriminatorRecordLength:
		return "DiscriminatorRecordLength"
	case DiscriminatorModeByte:
		return "DiscriminatorModeByte"
	default:
		return "RecordDiscriminator(" + strconv.Itoa(int(d)) + ")"
	}
}

// sortedEnumNames returns an enum's names in a stable order.
func sortedEnumNames(m map[byte]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// enumValueFor returns the wire value whose name is name. The enum is
// injective (profile validation requires it), so at most one matches.
func enumValueFor(m map[byte]string, name string) (byte, bool) {
	// Sorted iteration so a malformed enum that slipped past validation
	// would at least fail the same way every run.
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		if m[byte(k)] == name {
			return byte(k), true
		}
	}
	return 0, false
}
