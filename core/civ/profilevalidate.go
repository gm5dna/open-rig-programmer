// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"math"
	"sort"
)

// maxChannelDecimal is the largest channel number a two-byte packed-BCD
// field can express. DERIVED from the address encoding, not chosen.
const maxChannelDecimal = 9999

// maxGroupCount is the largest group or band COUNT a profile may declare.
// DERIVED from the one-byte packed-BCD index: indices run 0..99, so 100
// groups is the ceiling.
const maxGroupCount = 100

// maxNameLength is the widest name field a profile may declare.
//
// A RESOURCE bound rather than a protocol fact — no Icom document in this
// tier prints a name longer than 16 — and it exists because NameLength
// reaches the OUTBOUND WRITE GATE through the record encoder. Without a
// ceiling, one mistyped digit would authorise the gate to admit a
// pathologically long memory set. 64 leaves four times the widest
// documented name.
const maxNameLength = 64

// maxRecordLength is the widest record a profile may declare, for
// maxNameLength's reason: it bounds what the gate can be asked to admit.
// The widest ASSUMED record in this tier is the IC-705's 115 bytes.
const maxRecordLength = 512

// validateProfileConfig runs every rule and returns the FIRST failure.
//
// Errors name the FIELD and the OFFENDING VALUE. That is a contract the
// tests assert on, not a nicety: a validator returning a generic error
// from the wrong branch passes any test that only checks for non-nil, and
// core/cat has shipped exactly that kind of silently-correct-looking check
// before (dialectvalidate.go's own note).
func validateProfileConfig(cfg ProfileConfig) error {
	for _, rule := range []func(ProfileConfig) error{
		validateModel,         // V1
		validateAddresses,     // V2
		validateAddressSpace,  // V3
		validateNamePolicy,    // V4
		validateLayoutSet,     // V5
		validateLayoutFields,  // V6
		validateLayoutOverlap, // V7
		validateFixedTemplate, // V8
		validateFrameBound,    // V9
	} {
		if err := rule(cfg); err != nil {
			return err
		}
	}
	return nil
}

// validateModel is V1. Model is this package's Configured() witness as
// well as its diagnostic label, so an empty one would build a profile that
// reports itself unconfigured and refuses everything — a silent failure
// rather than a loud one.
func validateModel(cfg ProfileConfig) error {
	if cfg.Model == "" {
		return invalidProfile("Model is empty — it is the Configured() witness as well as the diagnostic label, so an unlabelled profile would silently refuse everything")
	}
	return nil
}

// validateAddresses is V2.
//
// PreambleByte and EndByte are refused in EITHER address because both
// bytes sit inside the frame where WellFormed's interior rule applies: a
// radio address of 0xFD would terminate every frame at its third byte, and
// one of 0xFE would let a receiver resynchronise mid-frame. 0x00 is
// refused because it is the CI-V broadcast address, which no station owns
// — and because ProfileConfig uses zero as ControllerAddress's "omitted"
// marker.
func validateAddresses(cfg ProfileConfig) error {
	if cfg.RadioAddress == 0 {
		return invalidProfile("RadioAddress is 0x00, the broadcast address — no radio owns it")
	}
	for _, a := range []struct {
		field string
		value byte
	}{
		{"RadioAddress", cfg.RadioAddress},
		{"ControllerAddress", cfg.ControllerAddress},
	} {
		if a.value == PreambleByte || a.value == EndByte {
			return invalidProfile("%s is %#02x, a framing byte — it sits inside the frame, where a preamble or terminator byte splits or resynchronises it", a.field, a.value)
		}
	}
	ctrl := cfg.ControllerAddress
	if ctrl == 0 {
		ctrl = ControllerAddressDefault
	}
	if cfg.RadioAddress == ctrl {
		return invalidProfile("RadioAddress %#02x is also the controller address — every frame this program sent would look like a frame addressed to it, and echo removal and answer matching would both be undecidable", cfg.RadioAddress)
	}
	return nil
}

// validateAddressSpace is V3: the address form and the channel space it
// has to encode.
func validateAddressSpace(cfg ProfileConfig) error {
	switch cfg.AddressForm {
	case AddressFormFlat:
		if cfg.Groups != 0 {
			return invalidProfile("Groups is %d under AddressFormFlat — a flat address has nowhere to encode a group, so an inapplicable field must be explicitly zero", cfg.Groups)
		}
	case AddressFormGroupChannel, AddressFormBandChannel:
		if cfg.Groups < 1 {
			return invalidProfile("Groups is %d under %v — a grouped address form needs at least one group", cfg.Groups, cfg.AddressForm)
		}
		if cfg.Groups > maxGroupCount {
			return invalidProfile("Groups is %d, want <= %d — the group index is one packed-BCD byte, so indices run 0..99", cfg.Groups, maxGroupCount)
		}
	default:
		return invalidProfile("AddressForm %v must be set explicitly — the zero value is not a form, and a grouped radio addressed flat reads a different channel", cfg.AddressForm)
	}
	if cfg.ChannelLo < 0 {
		return invalidProfile("ChannelLo is %d, want >= 0", cfg.ChannelLo)
	}
	if cfg.ChannelHi < cfg.ChannelLo {
		return invalidProfile("ChannelLo..ChannelHi is %d..%d, want Lo <= Hi", cfg.ChannelLo, cfg.ChannelHi)
	}
	if cfg.ChannelHi > maxChannelDecimal {
		return invalidProfile("ChannelHi is %d, want <= %d — the channel number is two packed-BCD bytes", cfg.ChannelHi, maxChannelDecimal)
	}
	return nil
}

// validateNamePolicy is V4.
//
// The charset's byte domain is the write-gate rule here: a name byte goes
// straight into a memory set frame, and the gate re-validates through the
// same predicate the builder uses, so a charset carrying a framing byte
// yields a gate-approved frame that splits on the wire.
func validateNamePolicy(cfg ProfileConfig) error {
	if cfg.NameLength < 0 {
		return invalidProfile("NameLength is %d, want >= 0 (0 means this model has no name field)", cfg.NameLength)
	}
	if cfg.NameLength > maxNameLength {
		return invalidProfile("NameLength is %d, want <= %d — it bounds the outbound write gate, so an unbounded value would authorise a pathologically long memory set", cfg.NameLength, maxNameLength)
	}
	if cfg.NameLength == 0 {
		if cfg.NameCharset != "" {
			return invalidProfile("NameCharset is %q with NameLength 0 — a model with no name field must leave the charset empty, or the config reads as having one", cfg.NameCharset)
		}
		if cfg.NamePad != 0 {
			return invalidProfile("NamePad is %#02x with NameLength 0 — an inapplicable field must be explicitly zero", cfg.NamePad)
		}
		return nil
	}
	if cfg.NameCharset == "" {
		return invalidProfile("NameCharset is empty with NameLength %d — no name would be expressible", cfg.NameLength)
	}
	seen := make(map[byte]bool, len(cfg.NameCharset))
	for i := 0; i < len(cfg.NameCharset); i++ {
		b := cfg.NameCharset[i]
		if b == PreambleByte || b == EndByte {
			return invalidProfile("NameCharset contains the framing byte %#02x — a channel name carrying one produces a gate-approved frame that splits on the wire", b)
		}
		if seen[b] {
			return invalidProfile("NameCharset repeats %#02x — a duplicate is a transcription error, and silently collapsing it hides the mistake where it is easiest to find", b)
		}
		seen[b] = true
	}
	if !seen[cfg.NamePad] {
		return invalidProfile("NamePad %#02x is not in NameCharset — every byte this profile writes into a name field must be one it declares legal, padding included", cfg.NamePad)
	}
	return nil
}

// validateLayoutSet is V5: the accepted lengths, the discriminator that
// picks among them, and the length the builder emits.
func validateLayoutSet(cfg ProfileConfig) error {
	if len(cfg.Layouts) == 0 {
		return invalidProfile("Layouts is empty — a profile with no record geometry can neither read nor write a channel")
	}
	seen := make(map[int]int, len(cfg.Layouts))
	for i, l := range cfg.Layouts {
		if l.Length < 1 || l.Length > maxRecordLength {
			return invalidProfile("Layouts[%d].Length is %d, want 1..%d", i, l.Length, maxRecordLength)
		}
		if prev, dup := seen[l.Length]; dup {
			return invalidProfile("Layouts[%d].Length %d repeats Layouts[%d] — the record length IS the discriminator, so two layouts sharing one are undecidable", i, l.Length, prev)
		}
		seen[l.Length] = i
		if len(l.Fields) == 0 {
			return invalidProfile("Layouts[%d] (length %d) has no Fields — a record nothing maps decodes to an empty channel on every read", i, l.Length)
		}
	}

	switch cfg.Discriminator {
	case DiscriminatorSingleLength:
		if len(cfg.Layouts) != 1 {
			return invalidProfile("Discriminator is %v with %d layouts — declare DiscriminatorRecordLength, or the extra layouts are a transcription error nothing would report", cfg.Discriminator, len(cfg.Layouts))
		}
	case DiscriminatorRecordLength:
		if len(cfg.Layouts) < 2 {
			return invalidProfile("Discriminator is %v with %d layout — the rule tells lengths apart, so a single-length profile must say DiscriminatorSingleLength", cfg.Discriminator, len(cfg.Layouts))
		}
	default:
		return invalidProfile("Discriminator %v must be set explicitly — spec D1 asks for the accepted lengths as a SET WITH A RULE, and an inferred rule would hide a two-layout profile that meant to have one", cfg.Discriminator)
	}

	if _, ok := seen[cfg.BuildLength]; !ok {
		lengths := make([]int, 0, len(seen))
		for n := range seen {
			lengths = append(lengths, n)
		}
		sort.Ints(lengths)
		return invalidProfile("BuildLength is %d, which is not one of the accepted lengths %v — the builder would emit a record this profile's own parser refuses", cfg.BuildLength, lengths)
	}
	return nil
}

// validateLayoutFields is V6: every span, on its own terms.
//
// EVERY CLAUSE HERE REACHES THE OUTBOUND WRITE GATE. AllowedCommand
// decodes a memory set with these spans and re-encodes it with them, so a
// span that could produce a framing byte, or an enum whose wire value does
// not fit its nibble, is a gate-approved frame carrying bytes no document
// defines.
func validateLayoutFields(cfg ProfileConfig) error {
	for li, l := range cfg.Layouts {
		for fi, sp := range l.Fields {
			where := func() string {
				return "Layouts[" + itoaSmall(li) + "].Fields[" + itoaSmall(fi) + "]"
			}
			kind, known := sp.Field.kind()
			if !known {
				return invalidProfile("%s names Field %q, which is not in this package's neutral vocabulary — see AllFieldIDs", where(), string(sp.Field))
			}
			if sp.Offset < 0 {
				return invalidProfile("%s (%s) has Offset %d, want >= 0", where(), sp.Field, sp.Offset)
			}
			if sp.Length < 1 {
				return invalidProfile("%s (%s) has Length %d, want >= 1", where(), sp.Field, sp.Length)
			}
			if sp.Offset+sp.Length > l.Length {
				return invalidProfile("%s (%s) has Offset %d and Length %d, spanning bytes %d..%d of a %d-byte record — a field running past the record would read whatever follows it", where(), sp.Field, sp.Offset, sp.Length, sp.Offset, sp.Offset+sp.Length-1, l.Length)
			}

			switch sp.Encoding {
			case EncodingBCDNumber:
				if kind != fieldNumeric {
					return invalidProfile("%s encodes the TEXT field %s as a number — the neutral record has nowhere to put the result", where(), sp.Field)
				}
				if sp.Length > maxBCDBytes {
					return invalidProfile("%s (%s) has Length %d, want <= %d — a wider packed-BCD field can overflow the uint64 it decodes to", where(), sp.Field, sp.Length, maxBCDBytes)
				}
				if sp.Order == OrderUnspecified {
					return invalidProfile("%s (%s) has no Order — CI-V uses both wire orders and the choice is per field, so an omitted one must refuse rather than default", where(), sp.Field)
				}
				if sp.Order != OrderLittleEndian && sp.Order != OrderBigEndian {
					return invalidProfile("%s (%s) has Order %v, which is not a wire order", where(), sp.Field, sp.Order)
				}
				if sp.Scale == 0 {
					return invalidProfile("%s (%s) has Scale 0 — the decoder multiplies by it, so every value would read as zero", where(), sp.Field)
				}
				// THE READ PATH MULTIPLIES, so Scale and the field's own
				// width together decide whether it can wrap. The decoder
				// computes raw * Scale, and the widest raw a Length-byte
				// packed-BCD field can hold is 10^(2*Length) - 1; a Scale
				// past MaxUint64 divided by that would let a record the
				// gate's re-encode WOULD refuse still come back from
				// ParseMemoryAnswer as a silently wrapped value. Refused
				// at construction, where there is no radio in the loop.
				if limit := math.MaxUint64 / maxBCDValue(sp.Length); sp.Scale > limit {
					return invalidProfile("%s (%s) has Scale %d with a %d-byte field, want Scale <= %d — the widest value the field holds is %d, and the decoder's raw*Scale would wrap past MaxUint64", where(), sp.Field, sp.Scale, sp.Length, limit, maxBCDValue(sp.Length))
				}
				if sp.Nibble != NibbleWhole {
					return invalidProfile("%s (%s) selects %v under EncodingBCDNumber — nibble spans are enum-only", where(), sp.Field, sp.Nibble)
				}
				if len(sp.Enum) != 0 {
					return invalidProfile("%s (%s) carries an Enum under EncodingBCDNumber — an inapplicable field must be empty", where(), sp.Field)
				}
			case EncodingEnum:
				if kind != fieldText {
					return invalidProfile("%s encodes the NUMERIC field %s as an enum — the neutral record has nowhere to put the result", where(), sp.Field)
				}
				if sp.Field == FieldName {
					return invalidProfile("%s encodes %s as an enum — a name is text of the profile's own width, not one of a fixed set", where(), sp.Field)
				}
				if sp.Length != 1 {
					return invalidProfile("%s (%s) has Length %d under EncodingEnum, want exactly 1 — an enum is one byte or one nibble of one", where(), sp.Field, sp.Length)
				}
				if len(sp.Enum) == 0 {
					return invalidProfile("%s (%s) has an empty Enum — every wire value would be unknown, so no record could ever be decoded", where(), sp.Field)
				}
				if sp.Scale != 0 || sp.Order != OrderUnspecified {
					return invalidProfile("%s (%s) carries Scale/Order under EncodingEnum — inapplicable fields must be zero", where(), sp.Field)
				}
				names := make(map[string]byte, len(sp.Enum))
				keys := make([]int, 0, len(sp.Enum))
				for k := range sp.Enum {
					keys = append(keys, int(k))
				}
				sort.Ints(keys)
				for _, k := range keys {
					v := byte(k)
					name := sp.Enum[v]
					if name == "" {
						return invalidProfile("%s (%s) Enum[%#02x] is empty — a nameless value could never be written back", where(), sp.Field, v)
					}
					if prev, dup := names[name]; dup {
						return invalidProfile("%s (%s) Enum maps both %#02x and %#02x to %q — the encode direction would be ambiguous", where(), sp.Field, prev, v, name)
					}
					names[name] = v
					if v == PreambleByte || v == EndByte {
						return invalidProfile("%s (%s) Enum carries the framing byte %#02x — writing it produces a gate-approved frame that splits on the wire", where(), sp.Field, v)
					}
					if sp.Nibble != NibbleWhole && v > 0x0F {
						return invalidProfile("%s (%s) Enum value %#02x does not fit the %v it is declared on — the high bits would silently land in the neighbouring field", where(), sp.Field, v, sp.Nibble)
					}
				}
			case EncodingName:
				if sp.Field != FieldName {
					return invalidProfile("%s encodes %s with EncodingName — the name encoding uses the profile's charset and pad byte, which belong to the name field alone", where(), sp.Field)
				}
				if cfg.NameLength == 0 {
					return invalidProfile("%s maps a name field but NameLength is 0 — the profile says this model has none", where())
				}
				if sp.Length != cfg.NameLength {
					return invalidProfile("%s has Length %d but NameLength is %d — the field width and the name policy must be the same number, or a name that validates would not fit", where(), sp.Length, cfg.NameLength)
				}
				if sp.Nibble != NibbleWhole {
					return invalidProfile("%s selects %v — a Nibble is meaningless for text", where(), sp.Nibble)
				}
				if sp.Scale != 0 || sp.Order != OrderUnspecified || len(sp.Enum) != 0 {
					return invalidProfile("%s carries Scale/Order/Enum under EncodingName — inapplicable fields must be zero", where())
				}
			default:
				return invalidProfile("%s (%s) has Encoding %v — the zero value is not a kind, and every default here would silently misread somebody's record", where(), sp.Field, sp.Encoding)
			}
		}
	}
	return nil
}

// validateLayoutOverlap is V7: no two spans may claim the same nibble.
//
// The encoder ORs nibble spans into a shared byte and assigns whole-byte
// spans outright, so two spans on one nibble would silently corrupt each
// other in a direction depending on field order — and the gate's
// re-encode check would then refuse this profile's own builder's output,
// with no message saying why.
//
// A field MAY appear twice at DIFFERENT positions: spec D5 entry 4's
// duplicated TX block is exactly that, and the decoder requires the copies
// to agree.
func validateLayoutOverlap(cfg ProfileConfig) error {
	for li, l := range cfg.Layouts {
		// Two claim maps, high nibble and low, so a whole-byte span and a
		// nibble span on the same byte collide correctly.
		claimedHigh := make(map[int]int, l.Length)
		claimedLow := make(map[int]int, l.Length)
		for fi, sp := range l.Fields {
			for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
				high, low := true, true
				if sp.Encoding == EncodingEnum {
					switch sp.Nibble {
					case NibbleHigh:
						low = false
					case NibbleLow:
						high = false
					}
				}
				if high {
					if prev, dup := claimedHigh[off]; dup {
						return invalidProfile("Layouts[%d]: fields %d (%s) and %d (%s) overlap at byte %d — two spans claiming one nibble corrupt each other in an order-dependent direction", li, prev, l.Fields[prev].Field, fi, sp.Field, off)
					}
					claimedHigh[off] = fi
				}
				if low {
					if prev, dup := claimedLow[off]; dup {
						return invalidProfile("Layouts[%d]: fields %d (%s) and %d (%s) overlap at byte %d — two spans claiming one nibble corrupt each other in an order-dependent direction", li, prev, l.Fields[prev].Field, fi, sp.Field, off)
					}
					claimedLow[off] = fi
				}
			}
		}
	}
	return nil
}

// maxBCDValue is the largest value an n-byte packed-BCD field can hold:
// 10^(2n) - 1. n is at most maxBCDBytes (9), so 10^18 - 1 fits a uint64
// with room to spare, which is why maxBCDBytes is derived rather than
// chosen.
func maxBCDValue(n int) uint64 {
	v := uint64(1)
	for i := 0; i < 2*n; i++ {
		v *= 10
	}
	return v - 1
}

// validateFixedTemplate is V8.
func validateFixedTemplate(cfg ProfileConfig) error {
	for li, l := range cfg.Layouts {
		if len(l.Fixed) == 0 {
			continue
		}
		if len(l.Fixed) != l.Length {
			return invalidProfile("Layouts[%d].Fixed is %d bytes for a %d-byte record — the template describes the whole record or none of it", li, len(l.Fixed), l.Length)
		}
		mapped := make(map[int]bool, l.Length)
		for _, sp := range l.Fields {
			for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
				mapped[off] = true
			}
		}
		for i, b := range l.Fixed {
			if b == PreambleByte || b == EndByte {
				return invalidProfile("Layouts[%d].Fixed[%d] is the framing byte %#02x — every record this profile builds would split on the wire", li, i, b)
			}
			if mapped[i] && b != 0 {
				return invalidProfile("Layouts[%d].Fixed[%d] is %#02x under a mapped field span — the span decides that byte, and a template also claiming it would make the precedence silent", li, i, b)
			}
		}
	}
	return nil
}

// validateFrameBound is V9: this profile's own frame ceiling must fit the
// frames this very profile builds.
//
// A profile whose memory set exceeds its own MaxFrame could send a write
// its own accumulator could never assemble the answer to — and, worse, its
// own gate would admit the frame while its own reader discarded the
// exchange as contamination.
func validateFrameBound(cfg ProfileConfig) error {
	max := cfg.MaxFrame
	if max <= 0 {
		max = DefaultMaxFrame
	}
	longest := 0
	for _, l := range cfg.Layouts {
		if l.Length > longest {
			longest = l.Length
		}
	}
	// FE FE to from cn sc <address> <record> FD
	need := 7 + cfg.AddressForm.addressBytes() + longest
	if max < need {
		return invalidProfile("MaxFrame is %d but this profile's own longest memory set is %d bytes — its gate would admit a frame its own accumulator discards as contamination", max, need)
	}
	return nil
}

// itoaSmall renders a small non-negative int without pulling strconv into
// every error path's format string.
func itoaSmall(n int) string {
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
