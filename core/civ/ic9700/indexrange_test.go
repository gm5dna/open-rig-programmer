// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import "testing"

func TestParseIndexRange(t *testing.T) {
	for _, tc := range []struct {
		in          string
		first, last int
		filled      bool
	}{
		{"1", 1, 1, false},
		{"①", 1, 1, false},
		{"2, 3", 2, 3, false},
		{"②, ③", 2, 3, false},
		{"5 ~ 9", 5, 9, false},
		{"⑤ ~ ⑨", 5, 9, false},
		{"㉑ ~ ㉓", 21, 23, false},
		{"㊹ ~ (51)", 44, 51, false},
		{"(52) ~ (67)", 52, 67, false},
		{"52 ~ 67", 52, 67, false},
		{"5 ~ 51", 5, 51, true}, // leg L records the filled pair in index_style
		{"❺ ~ [51]", 5, 51, true},
		{"❺ ~ (51 filled)", 5, 51, true},
	} {
		first, last, filled, err := parseIndexRange(tc.in)
		if err != nil || first != tc.first || last != tc.last || filled != tc.filled {
			t.Errorf("parseIndexRange(%q) = %d,%d,%v,%v; want %d,%d,%v",
				tc.in, first, last, filled, err, tc.first, tc.last, tc.filled)
		}
	}
	if _, _, _, err := parseIndexRange("not an index"); err == nil {
		t.Error("parseIndexRange must refuse text it cannot read, never guess")
	}
}

// TestParseIndexRangeRefusesRatherThanGuesses pins the refusals, because a
// normaliser that quietly returned a zero for text it could not read would
// make every crosscheck below agree about nothing.
func TestParseIndexRangeRefusesRatherThanGuesses(t *testing.T) {
	for _, in := range []string{
		"",                // nothing at all
		"~ 9",             // an open range
		"5 ~",             // the other open range
		"9 ~ 5",           // descending
		"0",               // the strip's indices start at 1
		"68",              // and stop at 67
		"(51",             // an unclosed fallback
		"51)",             // and an unopened one
		"[51",             // the filled fallback, unclosed
		"(51 outlined)",   // a qualifier this convention does not define
		"5 ~ 9 ~ 12",      // two tilde separators
		"2, 3, 4",         // three comma-separated indices
		"⑤ ~ ⑨ extra",     // trailing text
		"Ⅴ",               // a Roman numeral is not a circled numeral
		"⓵",               // a double-circled numeral is a different glyph class
		"5 ~ 51 (filled)", // the qualifier belongs INSIDE the fallback's parens
	} {
		if _, _, _, err := parseIndexRange(in); err == nil {
			t.Errorf("parseIndexRange(%q) succeeded; it must refuse, never guess", in)
		}
	}
}

// TestEveryCircledGlyphClassIsDecoded walks the three Unicode runs the
// printed strip uses, so a run whose arithmetic is off by one fails here
// rather than silently mis-numbering one field in a crosscheck.
func TestEveryCircledGlyphClassIsDecoded(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   int
		filled bool
	}{
		{"①", 1, false}, {"⑳", 20, false}, // U+2460..U+2473
		{"㉑", 21, false}, {"㉟", 35, false}, // U+3251..U+325F
		{"㊱", 36, false}, {"㊾", 49, false}, // U+32B1..U+32BF
		{"❶", 1, true}, {"❿", 10, true}, // U+2776..U+277F
		{"⓫", 11, true}, {"⓴", 20, true}, // U+24EB..U+24F4
	} {
		first, last, filled, err := parseIndexRange(tc.in)
		if err != nil {
			t.Errorf("parseIndexRange(%q): %v", tc.in, err)
			continue
		}
		if first != tc.want || last != tc.want || filled != tc.filled {
			t.Errorf("parseIndexRange(%q) = %d,%d,%v; want %d,%d,%v",
				tc.in, first, last, filled, tc.want, tc.want, tc.filled)
		}
	}
}
