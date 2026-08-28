// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "fmt"

// Tone is a CTCSS sub-audible tone frequency, stored in decihertz (tenths
// of a hertz) to keep the table exact-integer rather than floating point.
// For example Tone(670) is 67.0 Hz.
type Tone int

// Hz returns the tone frequency in hertz, e.g. Tone(670).Hz() == 67.0.
func (t Tone) Hz() float64 {
	return float64(t) / 10
}

// String returns the tone formatted as "<freq> Hz" with one decimal place,
// e.g. "67.0 Hz", matching how the manual and UI display CTCSS tones.
func (t Tone) String() string {
	return fmt.Sprintf("%.1f Hz", t.Hz())
}

// ToneRange is a CONTIGUOUS, evenly-stepped CTCSS tone domain: every tone
// from MinDeciHz to MaxDeciHz inclusive that MinDeciHz reaches in a whole
// number of StepDeciHz.
//
// IT IS THE SHAPE A RADIO WHOSE TONE FIELD IS A NUMBER NEEDS, and the
// Icom tier is full of them. A Yaesu CAT radio names a tone by its INDEX
// into a fifty-entry chart, so the chart is the domain and a list is the
// only honest way to write it down. A CI-V memory record carries the tone
// as packed BCD tenths of a hertz — a NUMBER — and a model whose manual
// says "67.0 to 254.1 Hz" is describing a range, not a table. Forcing
// such a model to enumerate 1,872 entries would be a transcription
// exercise with 1,872 chances to mistype, describing a domain its own
// document states in one line.
//
// IT IS DECLARED BY POINTER on Capabilities, so PRESENCE is the
// declaration. A zero ToneRange embedded by value would be
// indistinguishable from an author who never filled one in, and this is
// precisely a field where "not stated" and "stated as zero" must not be
// the same value: Capabilities.AdmitsTone fails CLOSED when neither a
// list nor a range is declared, and a value-typed zero range would have
// turned that into "admits nothing, for a radio that meant to admit
// everything in its chart" — or, worse under a laxer predicate, the
// reverse.
//
// A radio declares ONE or the OTHER, never both: see Validate.
type ToneRange struct {
	// MinDeciHz is the lowest admissible tone, in tenths of a hertz, and
	// is itself admissible. Must be greater than zero.
	MinDeciHz Tone
	// MaxDeciHz is the highest admissible tone, inclusive. Must be
	// greater than zero, at least MinDeciHz, and reachable from
	// MinDeciHz in a whole number of StepDeciHz — a maximum the step can
	// never land on is an author stating a bound their radio does not
	// have.
	MaxDeciHz Tone
	// StepDeciHz is the spacing between admissible tones, in tenths of a
	// hertz. Must be greater than zero. A radio accepting any tenth of a
	// hertz declares 1.
	//
	// IT IS REQUIRED RATHER THAN DEFAULTED, for ToneRange's own
	// pointer-presence reason: a zero step would have to mean either
	// "every tenth of a hertz" or "the author forgot", and the two are
	// not the same claim about a radio.
	StepDeciHz Tone
}

// admits reports whether t falls in this range: within both bounds, and
// on a step boundary measured FROM MinDeciHz.
//
// FROM MinDeciHz, not from zero. A radio whose chart starts at 67.0 Hz in
// 0.5 Hz steps admits 67.5 and not 67.2; measuring the step from zero
// would admit 67.0 and 67.5 alike only by coincidence of where the
// minimum happened to fall.
//
// It is unexported: the question callers ask is Capabilities.AdmitsTone,
// which knows about the list shape too, and a second exported predicate
// answering half the question is how the two consumers drifted apart in
// the first place.
func (r ToneRange) admits(t Tone) bool {
	if r.StepDeciHz <= 0 {
		// Unreachable for a Validate-d Capabilities, and refused rather
		// than divided by: a zero step describes no domain, so it admits
		// nothing.
		return false
	}
	if t < r.MinDeciHz || t > r.MaxDeciHz {
		return false
	}
	return (t-r.MinDeciHz)%r.StepDeciHz == 0
}

// AdmitsTone reports whether t is a tone THIS radio can express — the ONE
// predicate every consumer of a radio's tone domain asks, whichever shape
// that domain is declared in.
//
// IT IS ONE PREDICATE BECAUSE IT USED TO BE TWO. codeplug.ToneField.Valid
// and core/csvio's CHIRP import each carried their own "is t in
// caps.CTCSSTones" loop — identical, independent, and both list-only — so
// adding a range shape in one place would have left the other refusing
// every tone a range-declaring radio has. Both now call this.
//
// FAIL-CLOSED WHEN NEITHER IS DECLARED, and that is deliberate and
// unchanged. A radio that declares no list and no range admits NO tone.
// "No chart known" must never be treated as "no chart needed": this
// project refuses rather than corrupts, and a tone this program cannot
// prove the radio can express is a tone it must not send.
func (c Capabilities) AdmitsTone(t Tone) bool {
	if c.CTCSSToneRange != nil {
		return c.CTCSSToneRange.admits(t)
	}
	for _, x := range c.CTCSSTones {
		if x == t {
			return true
		}
	}
	return false
}

// standardCTCSSTones is the 50-tone CTCSS chart shared across the radio
// family this project targets. It is verified against the FT-710 CAT
// manual 2306-C, CN command, "Table 1 (CTCSS Tone Chart)": the array
// index IS the CAT tone number P3 (000-049), so standardCTCSSTones[0] is
// the tone the radio sends/expects for CAT tone number 000, and
// standardCTCSSTones[49] is tone number 049.
//
// This table lives in core/spec, not a driver, because it is
// radio-family-neutral in practice: it is the same 50-tone chart other
// Yaesu CAT radios of this generation use, so generic code (UI tone
// pickers, validation) can depend on it without hardcoding FT-710 facts.
//
// Unexported: callers get at it only through StandardCTCSSTones (a fresh
// array copy every call, so nothing can mutate this package's own copy)
// and ValidTone (for the common "is this a real tone" question, without
// needing the whole table at all).
var standardCTCSSTones = [50]Tone{
	670, 693, 719, 744, 770, 797, 825, 854, 885,
	915, 948, 974, 1000, 1035, 1072, 1109, 1148, 1188,
	1230, 1273, 1318, 1365, 1413, 1462, 1514, 1567, 1598, 1622,
	1655, 1679, 1713, 1738, 1773, 1799, 1835, 1862,
	1899, 1928, 1966, 1995, 2035, 2065, 2107, 2181, 2257,
	2291, 2336, 2418, 2503, 2541,
}

// StandardCTCSSTones returns a copy of the 50-tone CTCSS chart shared
// across the radio family this project targets — see standardCTCSSTones
// for its provenance and the CAT tone number P3 indexing convention.
// Every call returns an independent array (Go arrays are value types, so
// this is a genuine copy, not a shared reference): a caller is free to
// mutate its own copy without affecting this package's table or any
// other caller's copy.
func StandardCTCSSTones() [50]Tone {
	return standardCTCSSTones
}

// ValidTone reports whether t appears in StandardCTCSSTones. Prefer this
// over fetching the whole table with StandardCTCSSTones when a caller
// only needs to answer "is this a real tone in the standard family-wide
// chart" — a generic, radio-neutral question. It is NOT the check for
// whether a tone is sendable to a specific radio: that is
// codeplug.ToneField.Valid, which consults a Capabilities' own
// CTCSSTones, since not every radio in this family necessarily shares
// the standard chart.
func ValidTone(t Tone) bool {
	for _, s := range standardCTCSSTones {
		if s == t {
			return true
		}
	}
	return false
}
