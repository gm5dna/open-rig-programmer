// SPDX-License-Identifier: GPL-3.0-or-later

package spec

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
}

// standardShiftOptions is the repeater shift vocabulary shared across the
// radio family this project targets, in the FT-710 CAT manual's own P4
// order (SHIFT command).
//
// Unexported: callers get at it only through StandardShiftOptions (a
// fresh slice copy every call), matching StandardCTCSSTones' own
// pattern.
var standardShiftOptions = []string{"SIMPLEX", "PLUS", "MINUS"}

// StandardShiftOptions returns a copy of the repeater shift vocabulary
// shared across the radio family this project targets — see
// standardShiftOptions for its provenance. Every call returns an
// independently-allocated slice, so a caller is free to mutate its own
// copy without affecting this package's data or any other caller's copy.
func StandardShiftOptions() []string {
	out := make([]string, len(standardShiftOptions))
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
	{Value: "OFF", RequiresTone: false},
	{Value: "ENC-DEC", RequiresTone: true},
	{Value: "ENC", RequiresTone: true},
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
