// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// FieldState records how confidently a single field's value is known: read
// from (or destined for) the radio, not yet read, or not reachable at all
// via this radio/protocol.
//
// The write rule this type exists to enforce: only a Known value is ever
// sent to a radio. Unknown and Unavailable both mean "preserve whatever
// the radio currently has" — a write path must treat the two identically
// in that respect and must never synthesise or guess a value for either.
//
// That "preserve" guarantee is this project's own contract for what it
// sends: an Unknown/Unavailable field is never present on the wire.
// Whether the radio itself actually leaves the field untouched when an
// UNRELATED field on the same channel is written is a separate,
// currently unverified question for fields the CAT protocol cannot read
// back at all (CTCSSTone, ScanSkip) — see docs/hardware-notes.md's "M5b
// write-trial protocol (planned)", which exists specifically to answer
// it before any real write capability is enabled.
type FieldState string

const (
	// Known means this value was read from, or is destined to be written
	// to, the radio. Only a Known value is ever sent to a radio.
	Known FieldState = "known"
	// Unknown means this field has not been read yet — for example
	// because the CAT protocol has no way to read it. A write path must
	// never send this field: Unknown means "preserve whatever the radio
	// has".
	Unknown FieldState = "unknown"
	// Unavailable means this radio/protocol has no such field at all. A
	// write path must never send this field: Unavailable means "preserve
	// whatever the radio has" (which, for this field, is nothing this
	// project models).
	Unavailable FieldState = "unavailable"
)

// ToneField holds a CTCSS tone together with how confidently it is known.
// See FieldState for the write rule: only a Known ToneField is ever sent
// to a radio.
type ToneField struct {
	// State says how confidently Value is known.
	State FieldState `json:"state"`
	// Value is the tone. It is meaningful only when State == Known; Valid
	// requires it to be the zero value otherwise.
	Value spec.Tone `json:"value,omitempty"`
}

// Valid reports whether f is internally consistent against caps: State
// must be one of the three FieldState constants; Value must be the zero
// Tone unless State == Known; and a Known Value must appear in
// caps.CTCSSTones — THIS radio's own CTCSS chart, and the only tones its
// CAT protocol can express, so a Known tone outside that chart can never
// have come from (or be sendable to) it. caps.CTCSSTones is authoritative
// here, not spec.StandardCTCSSTones: two radios in this project's family
// can have different charts (see spec.Capabilities.CTCSSTones), and a
// codeplug is only ever sendable to the one radio caps describes. An
// empty caps.CTCSSTones is deliberately NOT "anything goes" — it fails
// closed, rejecting every Known tone, consistent with this project's
// refuse-never-corrupt posture: "no chart known" must never be treated as
// "no chart needed".
func (f ToneField) Valid(caps spec.Capabilities) error {
	switch f.State {
	case Known:
		if toneInChart(f.Value, caps.CTCSSTones) {
			return nil
		}
		return fmt.Errorf("codeplug: ToneField: Known value %v is not in this radio's CTCSS chart", f.Value)
	case Unknown, Unavailable:
		if f.Value != 0 {
			return fmt.Errorf("codeplug: ToneField: State %q must have zero Value, got %v", f.State, f.Value)
		}
		return nil
	default:
		return fmt.Errorf("codeplug: ToneField: invalid State %q", f.State)
	}
}

// toneInChart reports whether t appears in chart (a radio's
// caps.CTCSSTones). An empty chart matches nothing — see Valid's doc
// comment on why that is a deliberate fail-closed choice, not a bug.
func toneInChart(t spec.Tone, chart []spec.Tone) bool {
	for _, x := range chart {
		if x == t {
			return true
		}
	}
	return false
}

// BoolField holds a boolean value together with how confidently it is
// known. See FieldState for the write rule: only a Known BoolField is
// ever sent to a radio.
type BoolField struct {
	// State says how confidently Value is known.
	State FieldState `json:"state"`
	// Value is the flag. It is meaningful only when State == Known; Valid
	// requires it to be false otherwise.
	Value bool `json:"value,omitempty"`
}

// Valid reports whether f is internally consistent: State must be one of
// the three FieldState constants, and Value must be false unless State ==
// Known (a Known Value of false is legitimate — false is as real a known
// value as true).
func (f BoolField) Valid() error {
	switch f.State {
	case Known:
		return nil
	case Unknown, Unavailable:
		if f.Value {
			return fmt.Errorf("codeplug: BoolField: State %q must have zero Value, got %v", f.State, f.Value)
		}
		return nil
	default:
		return fmt.Errorf("codeplug: BoolField: invalid State %q", f.State)
	}
}

// Absent is the ZERO FieldState, and it is NOT one of the three states
// above: it means the field is not present in this codeplug at all.
//
// It exists because the Icom tier (design D4) added ten Fields to a
// model that already had ten, and every codeplug written before that
// tier — every schema-3 file, every hand-built ChannelData in a test —
// says nothing whatever about the new ones. "Says nothing" is a
// different fact from Unavailable ("this radio has no such field"),
// Unknown ("not read yet") and Known, and conflating it with any of them
// would put a claim into data that never made one.
//
// The rules that follow from that, and they are what keep the pre-tier
// behaviour byte-identical:
//
//   - It is the state a field has when nobody set one, so the ten
//     pre-tier Fields can never carry it (they are always set) and the
//     ten new ones carry it on every Yaesu channel.
//   - Valid() REJECTS it, on every field type, exactly as it always
//     rejected an unrecognised state — and that is safe because the
//     neutral checks that call Valid on a tier-added field are
//     capability-keyed (core/codeplug.Validate): a field this radio
//     cannot reach is not judged at all.
//   - An Absent field is never "touched" by a write (core/codeplug's
//     touchedFields) and never present for the file writer
//     (schemaFor), which is what lets a v3-representable codeplug keep
//     emitting schema 3.
const Absent FieldState = ""

// Present reports whether s says anything at all — i.e. is not Absent.
func (s FieldState) Present() bool { return s != Absent }

// FreqField holds a frequency in hertz together with how confidently it
// is known — the FieldTxFrequency and FieldOffset shape. See FieldState
// for the write rule: only a Known FreqField is ever sent to a radio.
//
// uint64, matching ChannelData.FreqHz: an offset is small, but a split
// transmit frequency is a frequency and must reach as far as one.
type FreqField struct {
	// State says how confidently Value is known.
	State FieldState `json:"state"`
	// Value is the frequency in hertz. It is meaningful only when
	// State == Known; Valid requires it to be zero otherwise.
	Value uint64 `json:"value,omitempty"`
}

// Valid reports whether f is internally consistent: State must be one of
// the three FieldState constants (Absent is not one — see Absent), and
// Value must be zero unless State == Known.
func (f FreqField) Valid() error {
	switch f.State {
	case Known:
		return nil
	case Unknown, Unavailable:
		if f.Value != 0 {
			return fmt.Errorf("codeplug: FreqField: State %q must have zero Value, got %v", f.State, f.Value)
		}
		return nil
	default:
		return fmt.Errorf("codeplug: FreqField: invalid State %q", f.State)
	}
}

// StringField holds a vocabulary value together with how confidently it
// is known — the FieldDuplex, FieldToneMode, FieldDTCSPolarity and
// FieldFilter shape. See FieldState for the write rule.
type StringField struct {
	// State says how confidently Value is known.
	State FieldState `json:"state"`
	// Value is the wire-form vocabulary string. It is meaningful only
	// when State == Known; Valid requires it to be empty otherwise.
	Value string `json:"value,omitempty"`
}

// Valid reports whether f is internally consistent against vocab — this
// radio's own vocabulary for the field, supplied by the caller because
// only the caller knows WHICH vocabulary this field draws on. State must
// be one of the three FieldState constants; Value must be empty unless
// State == Known; and a Known Value must appear in vocab.
//
// An EMPTY vocab is deliberately not "anything goes": it fails closed,
// rejecting every Known value, exactly as ToneField.Valid treats an
// empty CTCSS chart, and for the same reason — "no vocabulary known"
// must never be read as "no vocabulary needed".
func (f StringField) Valid(vocab []string) error {
	switch f.State {
	case Known:
		for _, v := range vocab {
			if v == f.Value {
				return nil
			}
		}
		return fmt.Errorf("codeplug: StringField: Known value %q is not one of this radio's values for the field", f.Value)
	case Unknown, Unavailable:
		if f.Value != "" {
			return fmt.Errorf("codeplug: StringField: State %q must have empty Value, got %q", f.State, f.Value)
		}
		return nil
	default:
		return fmt.Errorf("codeplug: StringField: invalid State %q", f.State)
	}
}

// IntField holds a whole number together with how confidently it is
// known — the FieldDTCSCode shape (the code NUMBER, 23 for "023"). See
// FieldState for the write rule.
type IntField struct {
	// State says how confidently Value is known.
	State FieldState `json:"state"`
	// Value is the number. It is meaningful only when State == Known;
	// Valid requires it to be zero otherwise.
	Value int `json:"value,omitempty"`
}

// Valid reports whether f is internally consistent against table — this
// radio's own table of legal values. The empty-table rule is
// StringField.Valid's, for the same reason.
func (f IntField) Valid(table []int) error {
	switch f.State {
	case Known:
		for _, v := range table {
			if v == f.Value {
				return nil
			}
		}
		return fmt.Errorf("codeplug: IntField: Known value %d is not one of this radio's values for the field", f.Value)
	case Unknown, Unavailable:
		if f.Value != 0 {
			return fmt.Errorf("codeplug: IntField: State %q must have zero Value, got %d", f.State, f.Value)
		}
		return nil
	default:
		return fmt.Errorf("codeplug: IntField: invalid State %q", f.State)
	}
}
