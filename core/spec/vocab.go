// SPDX-License-Identifier: GPL-3.0-or-later

package spec

// ShiftDirection is the semantic content of a repeater shift option:
// which way the transmit frequency moves relative to receive, if at all.
// Generic code (a CSV importer mapping a foreign dialect's "+"/"-", the
// UI) needs this fact about a shift value; re-deriving it from the
// wire-form string would put radio vocabulary literals straight back into
// the neutral layer task 38 removed them from.
type ShiftDirection int

const (
	// ShiftNone is simplex: transmit and receive on one frequency.
	ShiftNone ShiftDirection = iota
	// ShiftUp transmits above the receive frequency.
	ShiftUp
	// ShiftDown transmits below the receive frequency.
	ShiftDown
)

// ShiftOption is one repeater shift value this radio's wire protocol
// expresses, paired with the semantic fact generic code needs about it —
// the same Value-plus-semantics shape ToneState uses, for the same
// reason.
type ShiftOption struct {
	// Value is the wire-form shift string, e.g. "SIMPLEX", "PLUS".
	Value string
	// Direction is which way this option moves the transmit frequency.
	Direction ShiftDirection
}

// ToneState is one CTCSS state a memory channel's CTCSS field may hold —
// for example "OFF", "ENC", "ENC-DEC" — together with the semantic fact
// generic code (validation, the UI) needs about it, rather than having to
// re-derive it from the wire-form string by hand.
type ToneState struct {
	// Value is the wire-form CTCSS state string, e.g. "OFF", "ENC",
	// "ENC-DEC".
	Value string
	// RequiresTone is true iff a channel in this CTCSS state must carry
	// a known CTCSS tone for the state to make sense — e.g. an encoder
	// (ENC) or encoder+decoder (ENC-DEC) state needs a tone to encode or
	// decode; the off state does not.
	RequiresTone bool
	// Encodes is true iff a channel in this state TRANSMITS a CTCSS tone.
	Encodes bool
	// Decodes is true iff a channel in this state requires a matching
	// RECEIVED tone before it will open squelch.
	Decodes bool
}

// standardShiftOptions is the repeater shift vocabulary shared across the
// radio family this project targets, in the FT-710 CAT manual's own P4
// order (SHIFT command).
//
// Unexported: callers get at it only through StandardShiftOptions (a
// fresh slice copy every call), matching StandardCTCSSTones' own pattern.
var standardShiftOptions = []ShiftOption{
	{Value: "SIMPLEX", Direction: ShiftNone},
	{Value: "PLUS", Direction: ShiftUp},
	{Value: "MINUS", Direction: ShiftDown},
}

// StandardShiftOptions returns a copy of the repeater shift vocabulary
// shared across the radio family this project targets — see
// standardShiftOptions for its provenance. Every call returns an
// independently-allocated slice, so a caller is free to mutate its own
// copy without affecting this package's data or any other caller's copy.
func StandardShiftOptions() []ShiftOption {
	out := make([]ShiftOption, len(standardShiftOptions))
	copy(out, standardShiftOptions)
	return out
}

// standardCTCSSStates is the CTCSS state vocabulary shared across the
// radio family this project targets, in the FT-710 CAT manual's own P1
// order (CN/CT-adjacent commands): the off state first, then the two
// tone-bearing states.
//
// Unexported: callers get at it only through StandardCTCSSStates (a
// fresh slice copy every call), matching StandardCTCSSTones' own
// pattern.
var standardCTCSSStates = []ToneState{
	{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false},
	{Value: "ENC-DEC", RequiresTone: true, Encodes: true, Decodes: true},
	{Value: "ENC", RequiresTone: true, Encodes: true, Decodes: false},
}

// StandardCTCSSStates returns a copy of the CTCSS state vocabulary shared
// across the radio family this project targets — see standardCTCSSStates
// for its provenance. Every call returns an independently-allocated
// slice, so a caller is free to mutate its own copy without affecting
// this package's data or any other caller's copy.
func StandardCTCSSStates() []ToneState {
	out := make([]ToneState, len(standardCTCSSStates))
	copy(out, standardCTCSSStates)
	return out
}
