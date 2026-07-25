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

// ValidTone reports whether t appears in StandardCTCSSTones — the only
// tones this project's CAT protocol can express. Prefer this over
// fetching the whole table with StandardCTCSSTones when a caller only
// needs to answer "is this a real tone", e.g.
// codeplug.ToneField.Valid.
func ValidTone(t Tone) bool {
	for _, s := range standardCTCSSTones {
		if s == t {
			return true
		}
	}
	return false
}
