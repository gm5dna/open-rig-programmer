// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"errors"
	"strconv"
	"strings"
)

// Sentinel errors distinguishing parseExactToneDeciHz's failure modes.
var (
	// errToneFormat means s is not shaped like a plain, non-negative
	// decimal number at all (a missing/non-digit integer part, or a
	// non-digit fractional part).
	errToneFormat = errors.New("not a valid decimal Hz value")
	// errToneMorePrecision means s parses as a decimal number, but its
	// fractional part carries a non-zero digit beyond the first decimal
	// place — more precision than a CTCSS tone (one decimal place, e.g.
	// "88.5") can represent.
	errToneMorePrecision = errors.New("has more than one decimal place of precision")
	// errToneRange means the integer part of the tone is so large that
	// multiplying by 10 would overflow, or exceeds any sane Hz bound.
	errToneRange = errors.New("tone frequency is out of range")
)

// isDecimalDigits reports whether s is empty or contains only ASCII
// digits '0'-'9' (no sign, no separators) — deliberately stricter than
// isCHIRPDigits's own copy of this check is named for: this one backs a
// tone parser shared by BOTH csvio import paths (own-schema and CHIRP),
// not CHIRP alone.
func isDecimalDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseExactToneDeciHz parses a CTCSS tone cell (decimal Hz, e.g. "88.5")
// EXACTLY, without floating point, shared by both csvio tone paths (the
// own-schema ctcss_tone column and CHIRP's rToneFreq/cToneFreq): the
// integer part must be present and all-digits; the fractional part, if
// any, may hold at most ONE significant decimal digit. Any further digit
// must be zero, or the value carries more precision than a CTCSS tone can
// represent (e.g. "88.54") and is rejected (errToneMorePrecision) rather
// than silently rounded — a rounded value could differ from what was
// actually written, and this value may end up sent to a radio. A
// trailing zero beyond the first decimal place (e.g. "88.50") IS
// accepted: it is exactly representable at one-decimal precision,
// carrying no information rounding would lose.
//
// Returns the value in decihertz (tenths of a hertz — spec.Tone's own
// unit). This function does not check the result against
// spec.StandardCTCSSTones; that remains every caller's own job.
func parseExactToneDeciHz(s string) (int, error) {
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if intPart == "" || !isDecimalDigits(intPart) {
		return 0, errToneFormat
	}

	deciDigit := byte('0')
	if hasDot && fracPart != "" {
		if !isDecimalDigits(fracPart) {
			return 0, errToneFormat
		}
		deciDigit = fracPart[0]
		for i := 1; i < len(fracPart); i++ {
			if fracPart[i] != '0' {
				return 0, errToneMorePrecision
			}
		}
	}

	whole, err := strconv.Atoi(intPart)
	if err != nil {
		// Only reachable for an intPart too long to fit int:
		// isDecimalDigits already guarantees intPart is all-digits.
		return 0, errToneFormat
	}

	// Reject any value that would exceed a sane tone bound (e.g., 300000 Hz)
	// or would overflow when multiplied by 10.
	if whole > 300000 {
		return 0, errToneRange
	}

	return whole*10 + int(deciDigit-'0'), nil
}
