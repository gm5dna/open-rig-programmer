// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import "fmt"

// validateRecordFields reports whether rec is a record THIS profile can
// write at length: every field the layout maps is present, no field it does
// not map is, and every value fits its own span.
//
// GATE-REACHING, and the reason this package can claim its gate admits
// exactly what its builders produce. BuildMemorySet calls it before
// encoding; AllowedCommand calls it on the record it decoded out of a
// candidate frame. One validator, two callers, so "what the builders
// produce" and "what the gate admits" cannot drift apart — core/cat's
// allowlist rule, restated for a binary protocol.
//
// It is a Profile METHOD for the reason every gate-reaching validator here
// is: a package-level version could not consult a profile at all, and
// would bind every model to one model's record policy at the point that
// decides what reaches a radio.
//
// BOTH DIRECTIONS ARE CHECKED, and the second is not pedantry. A mapped
// field with no value would go out as a zero byte — for an enum, a value
// the layout may not even define — and an unmapped field carrying one
// would be silently dropped, writing a record the caller did not ask for.
// Neither is a state a caller can distinguish from success afterwards,
// which is why both refuse here.
func (p Profile) validateRecordFields(rec MemoryRecord, length int) error {
	if !p.Configured() {
		return fmt.Errorf("civ: unconfigured profile validates no record")
	}
	byID, ok := p.fieldsByIDByLayout[length]
	if !ok {
		return &RecordLengthError{Want: p.RecordLengths(), Got: length}
	}

	for _, id := range AllFieldIDs() {
		spans := byID[id]
		kind, _ := id.kind()

		present := false
		switch kind {
		case fieldNumeric:
			v, _ := rec.numeric(id)
			present = !v.Unavailable()
		case fieldText:
			v, _ := rec.text(id)
			present = !v.Unavailable()
		}

		if len(spans) == 0 {
			if present {
				return fmt.Errorf("civ: %s: record carries %s, which this profile's %d-byte layout has nowhere to put — silently dropping it would write a record the caller did not ask for", p.model, id, length)
			}
			continue
		}
		if !present {
			return fmt.Errorf("civ: %s: record has no %s, which this profile's %d-byte layout maps — the byte would go out as zero, which for an enum is a value the layout may not even define", p.model, id, length)
		}
		for _, sp := range spans {
			if err := p.validateSpanValue(rec, sp); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSpanValue checks one field's value against one span.
func (p Profile) validateSpanValue(rec MemoryRecord, sp FieldSpan) error {
	switch sp.Encoding {
	case EncodingBCDNumber:
		opt, _ := rec.numeric(sp.Field)
		v, _ := opt.Get()
		if v%sp.Scale != 0 {
			return fmt.Errorf("civ: %s: %s is %d, which is not a multiple of its field's scale %d — rounding it would write a value the caller did not ask for", p.model, sp.Field, v, sp.Scale)
		}
		if _, err := encodeBCDNumber(v/sp.Scale, sp.Length, sp.Order); err != nil {
			return fmt.Errorf("civ: %s: %s is %d, which does not fit its %d-byte field: %w", p.model, sp.Field, v, sp.Length, err)
		}
	case EncodingEnum:
		opt, _ := rec.text(sp.Field)
		name, _ := opt.Get()
		if _, ok := enumValueFor(sp.Enum, name); !ok {
			return fmt.Errorf("civ: %s: %s is %q, which is not one of this profile's own values %v", p.model, sp.Field, name, sortedEnumNames(sp.Enum))
		}
	case EncodingName:
		opt, _ := rec.text(sp.Field)
		name, _ := opt.Get()
		return p.validName(name)
	}
	return nil
}

// validName reports whether s is a name THIS profile can write: no longer
// than its own field, every byte in its own charset.
//
// GATE-REACHING, a Profile method, and shared by the builder and the gate
// — validateRecordFields's rule, applied to the one field whose domain is
// profile data rather than layout data.
func (p Profile) validName(s string) error {
	if p.nameLength == 0 {
		return fmt.Errorf("civ: %s has no name field", p.model)
	}
	if len(s) > p.nameLength {
		return fmt.Errorf("civ: %s: name %q is %d bytes, want <= %d — truncating it would write a name the caller did not choose", p.model, s, len(s), p.nameLength)
	}
	for i := 0; i < len(s); i++ {
		if !p.nameByteValid(s[i]) {
			return fmt.Errorf("civ: %s: name %q has byte %#02x at offset %d, which is not in this profile's charset", p.model, s, s[i], i)
		}
	}
	return nil
}

// encodeRecord renders rec as this profile's length-byte memory record.
//
// Bytes no span maps take their value from the layout's Fixed template,
// or zero where it has none. The result is a fresh slice; nothing in it
// aliases the profile or the caller's record.
func (p Profile) encodeRecord(rec MemoryRecord, length int) ([]byte, error) {
	if err := p.validateRecordFields(rec, length); err != nil {
		return nil, err
	}
	layout, ok := p.layoutByLength[length]
	if !ok {
		return nil, &RecordLengthError{Want: p.RecordLengths(), Got: length}
	}

	out := make([]byte, length)
	if len(layout.Fixed) == length {
		copy(out, layout.Fixed)
	}

	for _, sp := range layout.Fields {
		switch sp.Encoding {
		case EncodingBCDNumber:
			opt, _ := rec.numeric(sp.Field)
			v, _ := opt.Get()
			b, err := encodeBCDNumber(v/sp.Scale, sp.Length, sp.Order)
			if err != nil {
				return nil, fmt.Errorf("civ: %s: %s: %w", p.model, sp.Field, err)
			}
			copy(out[sp.Offset:], b)
		case EncodingEnum:
			opt, _ := rec.text(sp.Field)
			name, _ := opt.Get()
			v, ok := enumValueFor(sp.Enum, name)
			if !ok {
				return nil, fmt.Errorf("civ: %s: %s is %q, which is not one of this profile's own values", p.model, sp.Field, name)
			}
			switch sp.Nibble {
			case NibbleHigh:
				out[sp.Offset] = out[sp.Offset]&0x0F | v<<4
			case NibbleLow:
				out[sp.Offset] = out[sp.Offset]&0xF0 | v&0x0F
			default:
				out[sp.Offset] = v
			}
		case EncodingName:
			opt, _ := rec.text(sp.Field)
			name, _ := opt.Get()
			for i := 0; i < sp.Length; i++ {
				if i < len(name) {
					out[sp.Offset+i] = name[i]
				} else {
					out[sp.Offset+i] = p.namePad
				}
			}
		}
	}

	// DEFENCE IN DEPTH, not a redundant check. Profile validation refuses
	// every enum value, charset byte and Fixed byte that could be a
	// framing byte, so this cannot fire for a profile NewProfile built —
	// but the cost of being wrong about that is a frame that splits on the
	// wire and a gate that approved it, so the encoder asserts it on the
	// finished bytes rather than trusting the argument.
	for i, b := range out {
		if b == PreambleByte || b == EndByte {
			return nil, fmt.Errorf("civ: %s: encoded record byte %d is the framing byte %#02x — the frame would split on the wire", p.model, i, b)
		}
	}
	return out, nil
}

// decodeRecord reads a memory record back into neutral terms, attributing
// it to addr.
//
// A length outside this profile's accepted set is an ERROR (spec D4,
// adjudication 13): the read FAILS and ReadAll aborts honestly. There is
// no partial parse and no fake Unavailable channel — the neutral seam has
// no such result shape, and a record of an unexpected length is evidence
// that this is not the model the profile describes, which is exactly what
// the probe's length fingerprint reads.
func (p Profile) decodeRecord(b []byte, addr ChannelAddress) (MemoryRecord, error) {
	if !p.Configured() {
		return MemoryRecord{}, fmt.Errorf("civ: unconfigured profile decodes no record")
	}
	layout, ok := p.layoutByLength[len(b)]
	if !ok {
		return MemoryRecord{}, &RecordLengthError{Want: p.RecordLengths(), Got: len(b)}
	}

	rec := MemoryRecord{Address: addr}
	// seen records which fields a span has already filled, so a DUPLICATED
	// span (spec D5 entry 4's TX block) is checked for agreement rather
	// than silently letting the last copy win.
	seenNum := map[FieldID]uint64{}
	seenText := map[FieldID]string{}

	for _, sp := range layout.Fields {
		switch sp.Encoding {
		case EncodingBCDNumber:
			raw, err := decodeBCDNumber(b[sp.Offset:sp.Offset+sp.Length], sp.Order)
			if err != nil {
				return MemoryRecord{}, newParseError(b, "%s: %v", sp.Field, err)
			}
			v := raw * sp.Scale
			if prev, dup := seenNum[sp.Field]; dup && prev != v {
				return MemoryRecord{}, newParseError(b, "%s appears twice in this layout with different values (%d and %d) — the duplicated copies must agree", sp.Field, prev, v)
			}
			seenNum[sp.Field] = v
			rec.setNumeric(sp.Field, v)
		case EncodingEnum:
			raw := b[sp.Offset]
			switch sp.Nibble {
			case NibbleHigh:
				raw >>= 4
			case NibbleLow:
				raw &= 0x0F
			}
			name, ok := sp.Enum[raw]
			if !ok {
				return MemoryRecord{}, newParseError(b, "%s: byte %#02x at offset %d is not a value this profile defines", sp.Field, raw, sp.Offset)
			}
			if prev, dup := seenText[sp.Field]; dup && prev != name {
				return MemoryRecord{}, newParseError(b, "%s appears twice in this layout with different values (%q and %q) — the duplicated copies must agree", sp.Field, prev, name)
			}
			seenText[sp.Field] = name
			rec.setText(sp.Field, name)
		case EncodingName:
			field := b[sp.Offset : sp.Offset+sp.Length]
			end := len(field)
			for end > 0 && field[end-1] == p.namePad {
				end--
			}
			name := string(field[:end])
			if err := p.validName(name); err != nil {
				return MemoryRecord{}, newParseError(b, "%v", err)
			}
			if prev, dup := seenText[sp.Field]; dup && prev != name {
				return MemoryRecord{}, newParseError(b, "%s appears twice in this layout with different values (%q and %q) — the duplicated copies must agree", sp.Field, prev, name)
			}
			seenText[sp.Field] = name
			rec.setText(sp.Field, name)
		}
	}
	return rec, nil
}
