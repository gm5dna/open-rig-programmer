// SPDX-License-Identifier: GPL-3.0-or-later

package spec

// ShiftDirection is the semantic content of a repeater shift option:
// which way the transmit frequency moves relative to receive, if at all.
// Generic code (a CSV importer mapping a foreign dialect's "+"/"-", the
// UI) needs this fact about a shift value; re-deriving it from the
// wire-form string would put radio vocabulary literals straight back into
// the neutral layer task 38 removed them from.
//
// ShiftUnspecified is deliberately the zero value: a Capabilities author
// who omits Direction on a ShiftOption gets a value Validate REJECTS,
// not one that silently reads as simplex. Before this, ShiftNone was the
// zero value, so an omitted Direction on (say) an up-shift option
// silently became "no shift" — a semantic value the author never wrote,
// accepted because it was still a member of the declared vocabulary. See
// the M9c1 registration-gate review, finding A1.
type ShiftDirection int

const (
	// ShiftUnspecified is the zero value: not a real shift semantic, and
	// rejected by Validate wherever it appears.
	ShiftUnspecified ShiftDirection = iota
	// ShiftNone is simplex: transmit and receive on one frequency.
	ShiftNone
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
	// The zero value, ShiftUnspecified, is not a valid direction — every
	// ShiftOption must set this explicitly — see ShiftDirection's doc
	// comment.
	Direction ShiftDirection
}

// ToneSemantics is the semantic content of a CTCSS state: whether a
// channel in that state transmits a tone, requires a matching received
// tone, or neither.
//
// ToneSemanticsUnspecified is deliberately the zero value, for the same
// reason ShiftUnspecified is: (Encodes: false, Decodes: false) — CTCSS
// off — used to BE the zero value of the old Encodes/Decodes-bool shape,
// so a ToneState whose semantics were simply omitted silently read as
// "off" rather than being rejected. See the M9c1 registration-gate
// review, finding A1.
type ToneSemantics int

const (
	// ToneSemanticsUnspecified is the zero value: not a real tone
	// semantic, and rejected by Validate wherever it appears.
	ToneSemanticsUnspecified ToneSemantics = iota
	// ToneOff means this state neither transmits nor requires a CTCSS
	// tone.
	ToneOff
	// ToneEncode means a channel in this state TRANSMITS a CTCSS tone
	// but does not require one to open squelch on receive.
	ToneEncode
	// ToneEncodeDecode means a channel in this state both transmits a
	// CTCSS tone and requires a matching received tone before it will
	// open squelch.
	ToneEncodeDecode
)

// ToneState is one CTCSS state a memory channel's CTCSS field may hold —
// for example "OFF", "ENC", "ENC-DEC" — together with the semantic fact
// generic code (validation, the UI) needs about it, rather than having to
// re-derive it from the wire-form string by hand.
type ToneState struct {
	// Value is the wire-form CTCSS state string, e.g. "OFF", "ENC",
	// "ENC-DEC".
	Value string
	// Semantics is what this state means for encoding/decoding a CTCSS
	// tone. The zero value, ToneSemanticsUnspecified, is not valid —
	// every ToneState must set this explicitly — see ToneSemantics' doc
	// comment.
	Semantics ToneSemantics
}

// RequiresTone reports whether a channel in this CTCSS state must carry
// a known CTCSS tone for the state to make sense — e.g. an encoder
// (ENC) or encoder+decoder (ENC-DEC) state needs a tone to encode or
// decode; the off state does not. It is a method, not a stored field,
// because it is fully derivable from Semantics: there is no
// representable ToneState for which it could disagree.
func (t ToneState) RequiresTone() bool {
	return t.Semantics == ToneEncode || t.Semantics == ToneEncodeDecode
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
	{Value: "OFF", Semantics: ToneOff},
	{Value: "ENC-DEC", Semantics: ToneEncodeDecode},
	{Value: "ENC", Semantics: ToneEncode},
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
