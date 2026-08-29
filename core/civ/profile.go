// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"fmt"
	"sort"
)

// Profile is one Icom model's CI-V variation: everything this codec needs
// that differs between radios sharing the CI-V grammar. It is core/cat's
// Dialect analogue, and it carries DATA.
//
// THE RECEIVER IS LOAD-BEARING FOR PROFILE DATA. Every method here, and
// every helper those methods delegate to, must read this struct rather
// than a package-level datum for anything this struct carries: addresses,
// address form, channel range, name policy, record layouts, frame bound.
// A method that takes a Profile and consults a package global has the
// shape of a seam and none of the substance, and while every fixture
// agrees with every other, NO ordinary test can see the difference — which
// is why allTestProfiles (testprofiles_test.go) holds a profile built to
// DISAGREE at every attribute, the controller address included.
//
// THE CONTROLLER ADDRESS IS THE ONE MOST AT RISK. 0xE0 is the CI-V
// convention and this package declares it as ControllerAddressDefault, so
// it is exactly the datum a method would reach for directly. It is
// profile data because a bus with two controllers on it is a real
// configuration, and because a gate consulting the constant instead of the
// receiver would admit frames addressed on somebody else's behalf.
//
// THE ZERO VALUE IS INERT, deliberately. An exported struct always has a
// constructible zero value, so `var p civ.Profile` compiles and
// p.AllowedCommand is a perfectly non-nil method value: a gate that LOOKS
// installable while describing no radio at all. A zero Profile carries no
// address, no layout and no channel space, and consequently BUILDS
// NOTHING, PARSES NOTHING and ADMITS NOTHING — the last of those being
// the property that matters, since AllowedCommand is what stands between
// this program and a radio.
//
// Unlike core/cat's Dialect, NOT EVEN THE FIXED-LITERAL BUILDER emits from
// a zero Profile. BuildTransceiverIDRead's frame looks fixed —
// `19 00` carries no parameters — but its `to` and `from` bytes are
// profile data, so there is no radio-independent form of it to return, and
// it returns an error like the others.
type Profile struct {
	model              string
	radioAddr          byte
	controllerAddr     byte
	maxFrame           int
	addressForm        AddressForm
	groups             int
	groupBase          int
	channelLo          int
	channelHi          int
	extraRanges        []AddressRange
	nameLength         int
	nameCharset        []byte
	nameByteOK         map[byte]bool
	namePad            byte
	discriminator      RecordDiscriminator
	modeKey            FieldSpan
	buildLength        int
	layouts            []RecordLayout
	layoutByLength     map[int]int
	layoutByMode       map[string]int
	modeValueToClass   map[byte]string
	neutralModeLayout  map[string]int
	acceptedLengths    []int
	fieldsByIDByLayout map[int]map[FieldID][]FieldSpan
}

// NewProfile validates cfg and returns the Profile it describes.
//
// It COPIES every slice and map it is given, and derives its indices from
// the copies. A caller that mutates its input afterwards must not be able
// to change a constructed profile: a Profile is consulted by the outbound
// gate on every write, and a gate whose data can be edited after the fact
// by whoever built it is not a gate.
//
// Validation is exhaustive rather than advisory — see profilevalidate.go
// for each rule and the concrete failure it prevents. Several exist
// specifically because a caller-built profile could otherwise put a byte
// no Icom document defines inside a frame this program's own gate then
// approved: an enum wire value of 0xFD would terminate the frame early,
// and a name charset containing 0xFE would let a channel name start a new
// one.
func NewProfile(cfg ProfileConfig) (Profile, error) {
	if err := validateProfileConfig(cfg); err != nil {
		return Profile{}, err
	}

	p := Profile{
		model:          cfg.Model,
		radioAddr:      cfg.RadioAddress,
		controllerAddr: cfg.ControllerAddress,
		maxFrame:       cfg.MaxFrame,
		addressForm:    cfg.AddressForm,
		groups:         cfg.Groups,
		groupBase:      cfg.GroupBase,
		channelLo:      cfg.ChannelLo,
		channelHi:      cfg.ChannelHi,
		extraRanges:    append([]AddressRange(nil), cfg.ExtraRanges...),
		nameLength:     cfg.NameLength,
		nameCharset:    []byte(cfg.NameCharset),
		namePad:        cfg.NamePad,
		discriminator:  cfg.Discriminator,
		modeKey:        cfg.ModeKey.clone(),
		buildLength:    cfg.BuildLength,
	}
	if p.controllerAddr == 0 {
		p.controllerAddr = ControllerAddressDefault
	}
	if p.maxFrame <= 0 {
		p.maxFrame = DefaultMaxFrame
	}

	p.nameByteOK = make(map[byte]bool, len(p.nameCharset))
	for _, b := range p.nameCharset {
		p.nameByteOK[b] = true
	}

	p.layouts = make([]RecordLayout, len(cfg.Layouts))
	p.layoutByLength = make(map[int]int, len(cfg.Layouts))
	p.layoutByMode = make(map[string]int, len(cfg.Layouts))
	p.modeValueToClass = make(map[byte]string, len(cfg.Layouts))
	p.neutralModeLayout = make(map[string]int, len(cfg.Layouts))
	p.fieldsByIDByLayout = make(map[int]map[FieldID][]FieldSpan, len(cfg.Layouts))
	lengthSet := make(map[int]bool, len(cfg.Layouts))
	for i, l := range cfg.Layouts {
		cl := l.clone()
		p.layouts[i] = cl
		if p.discriminator != DiscriminatorModeByte {
			p.layoutByLength[cl.Length] = i
		}
		p.layoutByMode[cl.ModeClass] = i
		for _, value := range cl.ModeValues {
			p.modeValueToClass[value] = cl.ModeClass
		}
		if sp, ok := modeSpanInLayout(cl, p.modeKey); ok {
			for _, name := range sp.Enum {
				p.neutralModeLayout[name] = i
			}
		}
		lengthSet[cl.Length] = true
		byID := make(map[FieldID][]FieldSpan, len(cl.Fields))
		for _, sp := range cl.Fields {
			byID[sp.Field] = append(byID[sp.Field], sp)
		}
		p.fieldsByIDByLayout[i] = byID
	}
	p.acceptedLengths = make([]int, 0, len(lengthSet))
	for length := range lengthSet {
		p.acceptedLengths = append(p.acceptedLengths, length)
	}
	sort.Ints(p.acceptedLengths)

	return p, nil
}

// MustNewProfile is NewProfile for COMPILE-TIME-CONSTANT model tables, and
// panics if cfg is invalid.
//
// It exists so a per-model package can expose `func Profile() civ.Profile`
// without threading an error return through model registration for a
// failure that can only ever be a programming mistake in a literal.
//
// Do NOT call it on caller-supplied, file-derived or otherwise dynamic
// data. A malformed table baked into the binary is a build-time defect
// that should stop the programme loudly on first use; a malformed table
// read from a file at runtime is an ordinary error, and NewProfile is the
// function for that.
func MustNewProfile(cfg ProfileConfig) Profile {
	p, err := NewProfile(cfg)
	if err != nil {
		panic("civ: MustNewProfile: " + err.Error())
	}
	return p
}

// Configured reports whether this Profile carries data. False for the zero
// value; see the type's doc comment.
func (p Profile) Configured() bool {
	return p.model != "" && p.radioAddr != 0 && len(p.layouts) > 0
}

// Model is the radio this profile describes, for diagnostics.
func (p Profile) Model() string { return p.model }

// RadioAddress is this model's default CI-V address.
func (p Profile) RadioAddress() byte { return p.radioAddr }

// ControllerAddress is the address this program answers to under this
// profile — ControllerAddressDefault unless the profile says otherwise.
func (p Profile) ControllerAddress() byte { return p.controllerAddr }

// MaxFrame is the longest frame this profile's accumulator assembles.
func (p Profile) MaxFrame() int { return p.maxFrame }

// AddressForm is how this model addresses a memory channel.
func (p Profile) AddressForm() AddressForm { return p.addressForm }

// Groups is the COUNT of groups or bands, or 0 under a flat form. Valid
// indices are GroupBase()..GroupBase()+Groups()-1 — see ChannelAddress.
func (p Profile) Groups() int { return p.groups }

// GroupBase is the WIRE index of this model's first group or band: what
// the radio itself calls it. 0 for every model that numbers from zero and
// for every flat-addressed model, 1 for the IC-9700's A/B/C.
func (p Profile) GroupBase() int { return p.groupBase }

// ChannelRange is the inclusive channel range, per group where grouped.
func (p Profile) ChannelRange() (lo, hi int) { return p.channelLo, p.channelHi }

// NameLength is the width of this model's name field, or 0 for a model
// with none.
func (p Profile) NameLength() int { return p.nameLength }

// NamePad is the byte a short name is padded with and trimmed of.
func (p Profile) NamePad() byte { return p.namePad }

// NameCharset returns a fresh copy of every byte a name may carry.
func (p Profile) NameCharset() []byte { return copyBytes(p.nameCharset) }

// Discriminator names the rule picking among this profile's accepted
// record lengths.
func (p Profile) Discriminator() RecordDiscriminator { return p.discriminator }

// BuildRecordLength is the accepted length BuildMemorySet emits.
func (p Profile) BuildRecordLength() int { return p.buildLength }

// BuildRecordLengthFor returns the layout length selected by neutral mode,
// or zero when the profile is not mode-keyed or the mode is undeclared.
func (p Profile) BuildRecordLengthFor(mode string) int {
	if p.discriminator != DiscriminatorModeByte {
		return 0
	}
	i, ok := p.neutralModeLayout[mode]
	if !ok {
		return 0
	}
	return p.layouts[i].Length
}

// RecordLengths returns this profile's accepted record lengths — the SET
// of spec D1 — in ascending order, as a fresh slice.
func (p Profile) RecordLengths() []int {
	out := make([]int, len(p.acceptedLengths))
	copy(out, p.acceptedLengths)
	return out
}

// AcceptsRecordLength reports whether n is in this profile's accepted set.
// It is the probe's LENGTH FINGERPRINT (spec D3.2) in one call, and it is
// re-asked on every record read, so the fingerprint is continuous rather
// than one-shot.
func (p Profile) AcceptsRecordLength(n int) bool {
	i := sort.SearchInts(p.acceptedLengths, n)
	return i < len(p.acceptedLengths) && p.acceptedLengths[i] == n
}

// Layouts returns deep copies of this profile's record layouts, in
// configuration order. Callers may mutate what they get back: one caller's
// mutation must never become every radio's record geometry.
func (p Profile) Layouts() []RecordLayout {
	out := make([]RecordLayout, len(p.layouts))
	for i, l := range p.layouts {
		out[i] = l.clone()
	}
	return out
}

// LayoutFor returns a deep copy of the layout for record length n.
func (p Profile) LayoutFor(n int) (RecordLayout, bool) {
	if p.discriminator == DiscriminatorModeByte {
		return RecordLayout{}, false
	}
	i, ok := p.layoutByLength[n]
	if !ok {
		return RecordLayout{}, false
	}
	return p.layouts[i].clone(), true
}

// LayoutForRecord selects the one layout which can interpret rec. Length
// chooses under the two established discriminators; mode value chooses
// first under DiscriminatorModeByte and that layout then enforces length.
func (p Profile) LayoutForRecord(rec []byte) (RecordLayout, error) {
	i, err := p.layoutIndexForRecord(rec)
	if err != nil {
		return RecordLayout{}, err
	}
	return p.layouts[i].clone(), nil
}

func (p Profile) layoutIndexForRecord(rec []byte) (int, error) {
	if p.discriminator != DiscriminatorModeByte {
		i, ok := p.layoutByLength[len(rec)]
		if !ok {
			return 0, &RecordLengthError{Want: p.RecordLengths(), Got: len(rec)}
		}
		return i, nil
	}
	if p.modeKey.Offset < 0 || p.modeKey.Offset >= len(rec) {
		return 0, newParseError(rec, "%s: record is too short to contain mode key at offset %d", p.model, p.modeKey.Offset)
	}
	v := fieldSpanWireValue(rec[p.modeKey.Offset], p.modeKey.Nibble)
	class, ok := p.modeValueToClass[v]
	if !ok {
		return 0, newParseError(rec, "%s: undeclared mode value %#02x", p.model, v)
	}
	i := p.layoutByMode[class]
	want := p.layouts[i].Length
	if len(rec) != want {
		return 0, &RecordLengthError{Want: []int{want}, Got: len(rec), Mode: class}
	}
	return i, nil
}

func fieldSpanWireValue(b byte, nibble NibbleSel) byte {
	switch nibble {
	case NibbleHigh:
		return b >> 4
	case NibbleLow:
		return b & 0x0f
	default:
		return b
	}
}

func modeSpanInLayout(layout RecordLayout, key FieldSpan) (FieldSpan, bool) {
	for _, sp := range layout.Fields {
		if sameSpanPosition(sp, key) {
			return sp, true
		}
	}
	return FieldSpan{}, false
}

func sameSpanPosition(a, b FieldSpan) bool {
	return a.Field == b.Field && a.Offset == b.Offset && a.Length == b.Length &&
		a.Nibble == b.Nibble && a.Encoding == b.Encoding
}

// NewAccumulator returns a frame accumulator configured for THIS profile:
// its own frame bound and its own controller address.
//
// It exists so no caller has to remember to pass both, and so that neither
// can be taken from anywhere but the profile in hand.
func (p Profile) NewAccumulator() *FrameAccumulator {
	return NewFrameAccumulator(p.maxFrame, p.controllerAddr)
}

// nameByteValid reports whether b is in THIS profile's name charset.
func (p Profile) nameByteValid(b byte) bool { return p.nameByteOK[b] }

// validAddress reports whether a is an address this profile can encode and
// this radio has.
//
// GATE-REACHING: AllowedCommand routes every 1A 00 frame through it, and
// so does every builder, which is what stops "what the builders produce"
// and "what the gate admits" drifting apart. It is a Profile METHOD for
// that reason — a package-level version could not consult a profile at
// all, and would bind every model to one model's channel space at the
// point that decides what reaches a radio.
func (p Profile) validAddress(a ChannelAddress) error {
	if !p.Configured() {
		return fmt.Errorf("civ: unconfigured profile has no channel space")
	}
	groupLo, groupHi := 0, 0
	if p.addressForm.grouped() {
		groupLo, groupHi = p.groupBase, p.groupBase+p.groups-1
	}
	base := AddressRange{GroupLo: groupLo, GroupHi: groupHi, ChannelLo: p.channelLo, ChannelHi: p.channelHi}
	if base.contains(a) {
		return nil
	}
	for _, r := range p.extraRanges {
		if r.contains(a) {
			return nil
		}
	}
	return fmt.Errorf("civ: %s: address %s is outside base g%d..%d/ch%d..%d and extra ranges %v", p.model, a, base.GroupLo, base.GroupHi, base.ChannelLo, base.ChannelHi, p.extraRanges)
}

// encodeAddress renders a as this profile's own address field.
//
// THE ENCODING IS ASSUMED — see doc.go's register entry. Two packed-BCD
// bytes for the channel, most significant pair first, preceded by the
// form's own group or band index field where the form is grouped: one
// packed-BCD byte for the three-byte forms, TWO — most significant first —
// for AddressFormWideGroupChannel.
//
// The group is encoded as the WIRE index outright, with no base
// arithmetic: GroupBase says what the radio calls its first group, and
// what the radio calls a group is what goes on the wire. That is also why
// a zero-based profile's bytes are byte-identical to what this package
// emitted before GroupBase existed.
func (p Profile) encodeAddress(a ChannelAddress) ([]byte, error) {
	if err := p.validAddress(a); err != nil {
		return nil, err
	}
	out := make([]byte, 0, p.addressForm.addressBytes())
	if n := p.addressForm.groupBytes(); n > 0 {
		g, err := encodeBCDNumber(uint64(a.Group), n, OrderBigEndian)
		if err != nil {
			return nil, fmt.Errorf("civ: %s: group %d: %w", p.model, a.Group, err)
		}
		out = append(out, g...)
	}
	ch, err := encodeBCDNumber(uint64(a.Channel), 2, OrderBigEndian)
	if err != nil {
		return nil, fmt.Errorf("civ: %s: channel %d: %w", p.model, a.Channel, err)
	}
	return append(out, ch...), nil
}

// decodeAddress reads an address field back, refusing one this profile
// does not have.
func (p Profile) decodeAddress(b []byte) (ChannelAddress, error) {
	want := p.addressForm.addressBytes()
	if len(b) != want {
		return ChannelAddress{}, newParseError(b, "address field is %d bytes, want %d under %v", len(b), want, p.addressForm)
	}
	var a ChannelAddress
	rest := b
	if n := p.addressForm.groupBytes(); n > 0 {
		g, err := decodeBCDNumber(b[:n], OrderBigEndian)
		if err != nil {
			return ChannelAddress{}, newParseError(b, "group field: %v", err)
		}
		a.Group = int(g)
		rest = b[n:]
	}
	ch, err := decodeBCDNumber(rest, OrderBigEndian)
	if err != nil {
		return ChannelAddress{}, newParseError(b, "channel field: %v", err)
	}
	a.Channel = int(ch)
	if err := p.validAddress(a); err != nil {
		return ChannelAddress{}, newParseError(b, "%v", err)
	}
	return a, nil
}
