// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "slices"

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
// the same Value-plus-semantics shape ToneMode uses, for the same
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
	return slices.Clone(standardShiftOptions)
}

// standardToneModes is the CTCSS tone-mode vocabulary shared across the
// Yaesu radio family this project targets, in the FT-710 CAT manual's own
// P1 order (CN/CT-adjacent commands): the off state first, then the two
// tone-bearing states. It draws on ToneMode — the same vocabulary Icom
// drivers publish through Capabilities.ToneModes — since the Yaesu
// CTCSS-state vocabulary and the Icom tone-mode one were folded into a
// single enum; FieldCTCSSState (Yaesu) and FieldToneMode (Icom/Kenwood)
// stay separate Fields, but both now draw their Semantics from
// ToneModeSemantics. A model whose CTCSS field expresses more than these
// three (the FT-991A's five-state P8) builds its own list instead of
// calling this helper.
//
// Unexported: callers get at it only through StandardToneModes (a fresh
// slice copy every call), matching StandardCTCSSTones' own pattern.
var standardToneModes = []ToneMode{
	{Value: "OFF", Semantics: ToneModeOff},
	{Value: "ENC-DEC", Semantics: ToneModeCTCSSSquelch},
	{Value: "ENC", Semantics: ToneModeCTCSS},
}

// StandardToneModes returns a copy of the CTCSS tone-mode vocabulary
// shared across the Yaesu radio family this project targets — see
// standardToneModes for its provenance. Every call returns an
// independently-allocated slice, so a caller is free to mutate its own
// copy without affecting this package's data or any other caller's copy.
func StandardToneModes() []ToneMode {
	return slices.Clone(standardToneModes)
}

// DuplexDirection is the semantic content of an Icom-family duplex
// option: which way the transmit frequency moves relative to receive, if
// at all. It is the FieldDuplex analogue of ShiftDirection, and exists
// for the same reason — generic code (a CSV importer mapping CHIRP's
// "+"/"-", the UI) needs the fact without re-deriving it from a
// wire-form string.
//
// It is a SEPARATE type from ShiftDirection rather than a reuse of it,
// because the two vocabularies never coexist on one model (design D4)
// and a shared type would invite a shared lookup that silently answered
// for the wrong one.
type DuplexDirection int

const (
	// DuplexUnspecified is the zero value: not a real duplex semantic,
	// and rejected by Validate wherever it appears.
	DuplexUnspecified DuplexDirection = iota
	// DuplexOff is simplex: transmit and receive on one frequency.
	DuplexOff
	// DuplexUp transmits above the receive frequency, by the channel's
	// FieldOffset.
	DuplexUp
	// DuplexDown transmits below the receive frequency, by the
	// channel's FieldOffset.
	DuplexDown
)

// DuplexOption is one duplex value this radio's wire protocol expresses,
// paired with the semantic fact generic code needs about it — the same
// Value-plus-semantics shape ShiftOption and ToneState use.
type DuplexOption struct {
	// Value is the wire-form duplex string, e.g. "OFF", "DUP+", "DUP-".
	Value string
	// Direction is which way this option moves the transmit frequency.
	// The zero value, DuplexUnspecified, is not valid — every
	// DuplexOption must set this explicitly.
	Direction DuplexDirection
	// Canonical marks this option as THE answer to "which wire code does
	// this radio use for that direction?" — the question core/csvio's
	// CHIRP import asks when it maps a foreign dialect's "+"/"-" onto
	// this radio's vocabulary.
	//
	// REQUIRED ONLY WHERE A DIRECTION IS EXPRESSED MORE THAN ONCE, and
	// then EXACTLY ONE of those options must carry it (Validate). A
	// direction expressed by a single option needs no marking: there is
	// nothing to choose between, and demanding a flag on every entry of
	// every model's table would be ceremony rather than information.
	//
	// IT EXISTS BECAUSE MULTIPLICITY IS REAL AND SLICE ORDER IS NOT AN
	// ANSWER. A model can genuinely express one direction with two wire
	// codes; the reverse mapping used to return the FIRST match, so which
	// code an imported file produced depended on the order a driver
	// author happened to write the table in — a difference no test could
	// see and no reader would suspect.
	Canonical bool
}

// ToneModeSemantics is the semantic content of a tone-squelch mode: which
// mechanism a channel in that mode uses to encode and require a tone or
// code, whether CTCSS or DCS. One vocabulary now covers both field
// identities: Yaesu's FieldCTCSSState (Capabilities.ToneModes built from
// StandardToneModes, or a radio's own list) and Icom/Kenwood's
// FieldToneMode (which additionally spans the CROSS combinations and
// carries a code table) draw their Semantics from the same enum, though
// the two Fields stay separate — see capabilities.go's ToneModes doc for
// why a single Go type still needs two Field identities.
type ToneModeSemantics int

const (
	// ToneModeUnspecified is the zero value: not a real tone mode, and
	// rejected by Validate wherever it appears.
	ToneModeUnspecified ToneModeSemantics = iota
	// ToneModeOff means no tone or code squelch at all.
	ToneModeOff
	// ToneModeCTCSS TRANSMITS a CTCSS tone (FieldToneTx) and requires
	// none on receive — CHIRP's "Tone".
	ToneModeCTCSS
	// ToneModeCTCSSSquelch transmits a CTCSS tone and requires a
	// matching received tone (FieldToneRx) — CHIRP's "TSQL".
	ToneModeCTCSSSquelch
	// ToneModeCTCSSRxSquelch requires a matching received CTCSS tone and
	// transmits none. This is the receive-only TSQL semantic.
	ToneModeCTCSSRxSquelch
	// ToneModeDTCS uses a DTCS/DCS code (FieldDTCSCode,
	// FieldDTCSPolarity) in both directions — CHIRP's "DTCS".
	ToneModeDTCS
	// ToneModeCross combines two different mechanisms across the
	// transmit and receive directions — CHIRP's "Cross". A radio that
	// expresses it must also express the fields the chosen combination
	// needs; this project does not model the cross MODE string itself
	// beyond the vocabulary entry.
	ToneModeCross
	// ToneModeDCSEncodeDecode means a channel in this mode transmits a
	// DCS code and requires a matching received one before it will open
	// squelch — the CTCSS-and-DCS analogue of ToneModeCTCSSSquelch, for a
	// radio whose CTCSS-state field can name DCS as well (the FT-991A's
	// P8).
	//
	// APPENDED, not inserted: these two DCS members are the last two, so
	// no existing constant's value moves and nothing that has already
	// declared a semantic changes meaning.
	//
	// NOT ToneModeDTCS, DELIBERATELY. ToneModeDTCS's NeedsDTCS() is true,
	// which would make core/codeplug's validator demand a known
	// FieldDTCSCode — a field the FT-991A's P8 memory byte does not
	// carry: it names the DCS STATE only, never the code. NeedsTxTone and
	// NeedsRxTone report false for both of these members too, for the
	// same reason: a DCS state needs a CODE, not a CTCSS tone.
	// TestNeedsTone_IsNotWidenedByTheDCSMembers pins it.
	ToneModeDCSEncodeDecode
	// ToneModeDCSEncode means a channel in this mode transmits a DCS code
	// but does not require one to open squelch on receive — the DCS
	// analogue of ToneModeCTCSS.
	ToneModeDCSEncode
)

// ToneMode is one tone-squelch mode a memory channel's FieldToneMode
// (Icom/Kenwood) or FieldCTCSSState (Yaesu) may hold, together with the
// semantic fact generic code needs about it.
type ToneMode struct {
	// Value is the wire-form tone-mode string, e.g. "OFF", "TONE",
	// "TSQL", "DTCS".
	Value string
	// Semantics is what this mode means. The zero value,
	// ToneModeUnspecified, is not valid — every ToneMode must set this
	// explicitly.
	Semantics ToneModeSemantics
	// Canonical marks this mode as THE answer to "which wire code does
	// this radio use for that squelch mechanism?" — see
	// DuplexOption.Canonical, which carries the full argument. Required
	// only where a Semantics value is expressed more than once, and then
	// on exactly one of them.
	Canonical bool
}

// NeedsTxTone reports whether a channel in this tone mode must carry a
// known FieldToneTx value for the mode to make sense. It is a method, not
// a stored field, because it is fully derivable from Semantics. On the
// Yaesu family (FieldCTCSSState, StandardToneModes' three-member
// vocabulary or a radio's own list) this is the RequiresTone-style
// predicate core/codeplug's validator consults for its tone-pairing
// warning; the two DCS members report false for it, deliberately — see
// their own doc comment.
func (t ToneMode) NeedsTxTone() bool {
	return t.Semantics == ToneModeCTCSS || t.Semantics == ToneModeCTCSSSquelch
}

// NeedsRxTone reports whether a channel in this tone mode must carry a
// known FieldToneRx value for the mode to make sense.
func (t ToneMode) NeedsRxTone() bool {
	return t.Semantics == ToneModeCTCSSSquelch || t.Semantics == ToneModeCTCSSRxSquelch
}

// NeedsDTCS reports whether a channel in this tone mode must carry a
// known FieldDTCSCode (and FieldDTCSPolarity) for the mode to make
// sense.
func (t ToneMode) NeedsDTCS() bool {
	return t.Semantics == ToneModeDTCS
}
