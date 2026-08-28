// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"fmt"
	"strconv"
	"strings"
)

// THE THREE EVIDENCE LEGS SPELL THE SAME PRINTED INDEX THREE WAYS, and
// this file is the one place that reconciles them.
//
// The memory-content strip on PDF p.15 numbers its fields with CIRCLED
// NUMERALS, outlined for the record proper and FILLED (white digits on
// solid black discs) for the duplicated TX block. Unicode carries circled
// numerals only to 50, so the legs that transcribe the glyphs fall back to
// parenthesised digits above that, and each declares its own fallback:
//
//   - leg L writes plain ASCII throughout — `1`, `2, 3`, `5 ~ 9`,
//     `5 ~ 51`, `52 ~ 67` — and carries the outlined/filled distinction
//     in a separate `index_style` column instead;
//   - leg B writes the glyphs, `(51)` for the OUTLINED 51 and `[51]` for
//     the FILLED one, its own stated convention;
//   - leg W writes the glyphs, `(51)` for the outlined 51 and
//     `(51 filled)` for the filled one.
//
// A crosscheck that reconciled these with a regexp buried inside it would
// be a normaliser nobody could test. This one is code, and
// indexrange_test.go exercises every spelling the three legs actually use
// plus the refusals.
//
// IT REFUSES WHAT IT CANNOT READ AND NEVER GUESSES. A normaliser that
// returned a quiet zero for unreadable text would make every crosscheck
// downstream agree about nothing.

// indexLo and indexHi bound the printed strip's own numbering: the
// memory-content diagram runs ① through 67 and prints nothing outside
// that. A token outside the range is a transcription error, not an index.
const (
	indexLo = 1
	indexHi = 67
)

// parseIndexRange reads one leg's spelling of a printed index range and
// returns the first and last index it names, and whether the glyphs are
// the FILLED class.
//
// The accepted forms are a single index (`4`, `①`, `❺`), a printed comma
// pair (`2, 3`, `②, ③`) and a printed tilde range (`5 ~ 9`, `㊹ ~ (51)`).
// The comma pair is how the page prints two adjacent one-byte fields
// under one bracket; it is a RANGE of two for every purpose here.
func parseIndexRange(s string) (first, last int, filled bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false, fmt.Errorf("index range is empty")
	}

	var parts []string
	switch {
	case strings.Contains(s, "~"):
		parts = strings.Split(s, "~")
	case strings.Contains(s, ","):
		parts = strings.Split(s, ",")
	default:
		parts = []string{s}
	}
	if len(parts) > 2 {
		return 0, 0, false, fmt.Errorf("index range %q has %d separators; the page prints at most one", s, len(parts)-1)
	}

	var values []int
	for _, part := range parts {
		n, tokenFilled, tokenErr := parseIndexToken(strings.TrimSpace(part))
		if tokenErr != nil {
			return 0, 0, false, fmt.Errorf("index range %q: %w", s, tokenErr)
		}
		values = append(values, n)
		// The legs mark the filled class on whichever token carries a
		// glyph for it, and a fallback spelling may only be available for
		// one end of the range — leg B's `❺ ~ [51]` marks BOTH ends, leg
		// W's `❺ ~ (51 filled)` likewise, but neither can be relied on.
		filled = filled || tokenFilled
	}

	first, last = values[0], values[len(values)-1]
	if first > last {
		return 0, 0, false, fmt.Errorf("index range %q descends from %d to %d", s, first, last)
	}

	// THE ONE RANGE PLAIN ASCII CANNOT DISTINGUISH, and it is leg L's.
	//
	// L writes no glyphs, so `5 ~ 51` arrives here with no filled marker
	// at all — and `5 ~ 51` is precisely the duplicated TX block. The
	// outlined record proper is NEVER printed as a single 5-to-51 range
	// anywhere in the source: the strip draws it as thirteen separate
	// bracketed groups (⑤~⑨, ⑩,⑪, ⑫ … ㊹~51), and only the filled block
	// carries one bracket spanning the whole span. So a bare 5~51 can be
	// nothing else.
	//
	// It is asserted rather than assumed: readLedger checks this flag
	// against leg L's own index_style column on every row, so if the
	// inference were ever wrong for some later transcription the
	// disagreement is a failure, not a silent reinterpretation.
	if !filled && first == 5 && last == 51 {
		filled = true
	}
	return first, last, filled, nil
}

// parseIndexToken reads ONE printed index: a circled glyph, a filled
// circled glyph, a parenthesised or bracketed fallback, or plain digits.
func parseIndexToken(s string) (n int, filled bool, err error) {
	switch {
	case strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") && len(s) > 2:
		// Leg B's fallback for a FILLED numeral above 50.
		n, err = parseDecimalIndex(strings.TrimSpace(s[1 : len(s)-1]))
		return n, true, err

	case strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && len(s) > 2:
		inner := strings.TrimSpace(s[1 : len(s)-1])
		// Leg W's fallback for a FILLED numeral above 50 carries the
		// class as a word inside the parens. Any other qualifier is a
		// convention this file does not know, and is refused rather than
		// read past.
		if rest, ok := strings.CutSuffix(inner, " filled"); ok {
			n, err = parseDecimalIndex(strings.TrimSpace(rest))
			return n, true, err
		}
		if strings.ContainsAny(inner, " \t") {
			return 0, false, fmt.Errorf("index token %q carries a qualifier this convention does not define", s)
		}
		n, err = parseDecimalIndex(inner)
		return n, false, err

	default:
		if runes := []rune(s); len(runes) == 1 {
			if n, filled, ok := decodeCircled(runes[0]); ok {
				return n, filled, nil
			}
		}
		n, err = parseDecimalIndex(s)
		return n, false, err
	}
}

// parseDecimalIndex reads plain digits and bounds them to the strip's own
// numbering.
func parseDecimalIndex(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("index token is empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("index token %q is not a printed index", s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("index token %q: %w", s, err)
	}
	if n < indexLo || n > indexHi {
		return 0, fmt.Errorf("index token %q is %d, outside the printed strip's %d..%d", s, n, indexLo, indexHi)
	}
	return n, nil
}

// decodeCircled maps one circled-numeral rune to its value and its glyph
// class.
//
// THE RUNS ARE NOT CONTIGUOUS AND THERE IS NO CHARACTER ABOVE 50, which
// is why the legs write `(51)` at all. Outlined: U+2460..U+2473 are ①–⑳,
// U+3251..U+325F continue 21–35 and U+32B1..U+32BF continue 36–50.
// Filled: U+2776..U+277F are ❶–❿ and U+24EB..U+24F4 are ⓫–⓴.
func decodeCircled(r rune) (n int, filled bool, ok bool) {
	switch {
	case r >= '①' && r <= '⑳':
		return int(r-'①') + 1, false, true
	case r >= '㉑' && r <= '㉟':
		return int(r-'㉑') + 21, false, true
	case r >= '㊱' && r <= '㊿':
		return int(r-'㊱') + 36, false, true
	case r >= '❶' && r <= '❿':
		return int(r-'❶') + 1, true, true
	case r >= '⓫' && r <= '⓴':
		return int(r-'⓫') + 11, true, true
	default:
		return 0, false, false
	}
}
