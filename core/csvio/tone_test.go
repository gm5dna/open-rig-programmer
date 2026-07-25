// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"errors"
	"testing"
)

// TestParseExactToneDeciHz covers the shared exact (non-floating-point)
// decimal-Hz-to-decihertz parser both csvio tone paths (own-schema
// ctcss_tone and CHIRP rToneFreq/cToneFreq) are built on: an integer
// part plus AT MOST one significant decimal digit. More precision than
// that (e.g. "88.54") must be rejected outright, never silently rounded,
// because a rounded value could differ from what was actually written
// and this value may end up sent to a radio. A trailing zero beyond the
// first decimal place (e.g. "88.50") is exactly representable at
// one-decimal precision and is accepted.
func TestParseExactToneDeciHz(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int
		wantErr error
	}{
		{"one decimal place", "88.5", 885, nil},
		{"whole number, no dot", "88", 880, nil},
		{"trailing dot, no fractional digits, zero-padded", "88.", 880, nil},
		{"trailing zero beyond one place is exactly representable", "88.50", 885, nil},
		{"more zeros beyond one place still exactly representable", "88.500", 885, nil},
		{"more than one decimal place with a non-zero remainder is rejected", "88.54", 0, errToneMorePrecision},
		{"lowest table tone", "67.0", 670, nil},
		{"highest table tone", "254.1", 2541, nil},
		{"enormous integer part overflows range", "922337203685477580.0", 0, errToneRange},
		{"sane upper bound (300000)", "300000", 3000000, nil},
		{"just over upper bound", "300001", 0, errToneRange},
		{"empty string", "", 0, errToneFormat},
		{"non-digit integer part", "abc.5", 0, errToneFormat},
		{"non-digit fractional part", "88.ab", 0, errToneFormat},
		{"negative sign rejected (tones are never negative)", "-88.5", 0, errToneFormat},
		{"double dot is malformed", "88.5.5", 0, errToneFormat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExactToneDeciHz(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseExactToneDeciHz(%q) error = %v, want %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseExactToneDeciHz(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseExactToneDeciHz(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
