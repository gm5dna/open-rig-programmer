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
