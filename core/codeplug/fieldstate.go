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
// Tone unless State == Known; and a Known Value must be one THIS radio can
// express, which is spec.Capabilities.AdmitsTone's question.
//
// caps IS AUTHORITATIVE, not spec.StandardCTCSSTones: radios in this
// project's families have different tone domains, and a codeplug is only
// ever sendable to the one radio caps describes.
//
// THE PREDICATE IS SHARED, and since the Icom tier (E3) that is the point
// of it. This method used to carry its own list-only loop over
// caps.CTCSSTones, and core/csvio's CHIRP import carried an identical
// second copy — so a radio whose tone domain is a numeric RANGE rather
// than a chart (every CI-V model in the Icom tier) would have had every
// one of its perfectly expressible tones refused by two independently
// written checks. Both now ask caps.AdmitsTone, which knows about both
// shapes.
//
// FAIL-CLOSED IS UNCHANGED. A radio declaring NEITHER a list nor a range
// admits nothing, exactly as an empty caps.CTCSSTones always has:
// "no chart known" must never be treated as "no chart needed", consistent
// with this project's refuse-never-corrupt posture.
func (f ToneField) Valid(caps spec.Capabilities) error {
	switch f.State {
	case Known:
		if caps.AdmitsTone(f.Value) {
			return nil
		}
		return fmt.Errorf("codeplug: ToneField: Known value %v is not a tone this radio can express", f.Value)
	case Unknown, Unavailable:
		if f.Value != 0 {
			return fmt.Errorf("codeplug: ToneField: State %q must have zero Value, got %v", f.State, f.Value)
		}
		return nil
	default:
		return fmt.Errorf("codeplug: ToneField: invalid State %q", f.State)
	}
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
// above: it is what a field holds when nobody has set one at all.
//
// It exists because the Icom tier (design D4) added ten Fields to a
// model that already had ten, so every hand-built ChannelData that
// predates them — a test fixture, a value assembled by the GUI for a
// radio with no such field — leaves the new ones at their zero value. A
// FieldState the code has never assigned is not a claim about anything,
// and Valid() REJECTS it on every field type, exactly as it always
// rejected an unrecognised state. That is safe because the checks that
// call Valid on a tier-added field are capability-keyed
// (core/codeplug.Validate): a field this radio cannot reach is not
// judged at all.
//
// Absent is deliberately NOT the state a radio read or a file load
// produces for a field the radio lacks — those produce Unavailable, the
// positive statement "this radio/protocol has no such field". See
// FieldState.Recorded for why the two nevertheless answer the file
// writer's question identically.
const Absent FieldState = ""

// Recorded reports whether s carries something a codeplug FILE has to
// write down: Known (a value) or Unknown (an open question about a field
// the radio does have).
//
// Absent and Unavailable both answer false, and that pairing is the
// hinge of the Icom tier's byte-identity guarantee (design D4). Schema 3
// has no key for any tier-added field, and the absence of a key says
// exactly what Unavailable says — this codeplug has nothing to store
// here. So a channel read from a Yaesu radio, whose ten tier fields all
// come back Unavailable (the TagDisplay precedent), is fully
// representable in schema 3 and is written there, byte for byte as it
// was before the tier existed. A load of such a file reproduces
// Unavailable, so the round trip is stable and, just as importantly, a
// codeplug loaded from an old file still compares EQUAL to a fresh read
// of the same radio — which is what keeps codeplug.Diff from reporting
// every channel as modified.
//
// Only Known and Unknown force schema 4, because only they say something
// no schema-3 file could hold.
func (s FieldState) Recorded() bool { return s == Known || s == Unknown }

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
