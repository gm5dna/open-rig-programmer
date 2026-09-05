// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "testing"

// TestPrefixLenMatcher_FixedLengthFamilies pins the ordinary branch: an
// answer must start with the prefix AND be exactly exactLen bytes.
func TestPrefixLenMatcher_FixedLengthFamilies(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		exactLen int
		frame    string
		want     bool
	}{
		{"ID answer, 6 bytes", "ID", 6, "ID023;", true},
		{"a Yaesu 7-byte ID answer refused", "ID", 6, "ID0800;", false},
		{"FV answer, 7 bytes", "FV", 7, "FV1.00;", true},
		{"TY answer, 6 bytes", "TY", 6, "TY001;", true},
		{"a five-byte TY refused", "TY", 6, "TY01;", false},
		{"MR answer, 50 bytes", "MR", 50, "MR" + "0" + "0" + "07" + "00014250000" + "0" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "NAME    " + ";", true},
		{"wrong prefix", "MR", 50, "MW0007;", false},
		{"shorter than the prefix", "MR", 50, "M", false},
		{"the bare terminator a noisy line delivers", "MR", 50, ";", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrefixLenMatcher(tt.prefix, tt.exactLen)([]byte(tt.frame)); got != tt.want {
				t.Errorf("matcher(%q) = %v, want %v (prefix %q, exactLen %d)", tt.frame, got, tt.want, tt.prefix, tt.exactLen)
			}
		})
	}
}

// TestPrefixLenMatcher_VariableLengthBranchIsGenuine pins the exactLen <= 0
// branch, which on the Yaesu side only MT uses and which here carries the ONE
// variable-length frame this milestone parses: the EX answer, whose P5 is
// declared "variable length" with no printed ceiling (590:555-556, 480:409-411).
func TestPrefixLenMatcher_VariableLengthBranchIsGenuine(t *testing.T) {
	m := PrefixLenMatcher("EX000000", 0)
	for _, frame := range []string{"EX000000" + "1" + ";", "EX000000" + "12" + ";", "EX000000" + "1.00" + ";"} {
		if !m([]byte(frame)) {
			t.Errorf("matcher(%q) = false, want true — the EX answer's P5 width is not fixed", frame)
		}
	}
	if m([]byte("EX00000")) {
		t.Error("matcher accepted a frame shorter than the prefix")
	}
}

// TestPrefixLenMatcher_FullAddressObligation is the negative-space proof the
// matcher's doc comment names, and it is the reason the EX prefix must carry
// the whole address.
//
// Every one of the TS-590SG's 100, the TS-590S's 88 and the TS-480's 61 menu
// addresses answers with a frame starting "EX". A bare "EX" prefix therefore
// correlates ANY EX answer as this read's answer — a different address's
// reply still in flight, say — and returns the wrong address's data silently.
// The pair below is the whole finding: the bare prefix accepts a foreign
// answer; the full-address prefix refuses it and still accepts our own.
func TestPrefixLenMatcher_FullAddressObligation(t *testing.T) {
	const ours = "EX000000" // menu 000, P2 "00", P3 '0'
	const foreign = "EX056000"

	bare := PrefixLenMatcher("EX", 0)
	if !bare([]byte(foreign + "3;")) {
		t.Fatal("the bare-prefix matcher did not accept a foreign address's answer — this test's premise has gone stale")
	}

	full := PrefixLenMatcher(ours, 0)
	if full([]byte(foreign + "3;")) {
		t.Error("the full-address matcher accepted address 056's answer while reading 000 — the prefix must carry the whole three-digit menu number")
	}
	if !full([]byte(ours + "5;")) {
		t.Error("the full-address matcher refused our own address's answer")
	}
}

// TestPrefixLenMatcher_DoesNotRetainTheFrame pins the CONTRACT ON frame: the
// matcher is handed the engine's own live receive buffer, so it may read it
// and must never retain it.
func TestPrefixLenMatcher_DoesNotRetainTheFrame(t *testing.T) {
	m := PrefixLenMatcher("ID", 6)
	frame := []byte("ID023;")
	if !m(frame) {
		t.Fatal("matcher refused a well-formed ID answer")
	}
	frame[2] = '9'
	if !m(frame) {
		t.Error("matcher's verdict changed after the caller mutated its own buffer — it is holding state it should not")
	}
}
