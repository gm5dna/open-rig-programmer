// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// TestPrefixLenMatcher is the table core/transport's own
// TestCommandSpec_Matches used to run against the engine's inline
// prefix/len comparison, moved here with the rule itself (D2). Every case
// is carried over verbatim so the matching semantics are pinned as
// UNCHANGED across the move, not merely re-asserted afterwards.
func TestPrefixLenMatcher(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		exactLen int
		frame    string
		want     bool
	}{
		{"prefix match, variable len", "MT", 0, "MT0011CALLING FREQ;", true},
		{"prefix mismatch", "MR", 0, "MT0011CALLING FREQ;", false},
		{"prefix match, exact len ok", "ID", 7, "ID0800;", true},
		{"prefix match, exact len mismatch", "ID", 8, "ID0800;", false},
		{"frame shorter than prefix", "MR001", 0, "MR;", false},
		{"empty prefix admits anything", "", 0, "anything;", true},
		{"negative exactLen behaves as variable", "ID", -1, "ID0800;", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := PrefixLenMatcher(tt.prefix, tt.exactLen)
			if got := match([]byte(tt.frame)); got != tt.want {
				t.Errorf("PrefixLenMatcher(%q, %d)(%q) = %v, want %v", tt.prefix, tt.exactLen, tt.frame, got, tt.want)
			}
		})
	}
}

// TestPrefixLenMatcher_FullAddressDiscriminatesEXAnswers pins the safety
// obligation PrefixLenMatcher's doc comment states: a FULL-address EX
// prefix refuses another address's answer, and the bare command name does
// not. The hazard half is the negative space — it is asserted here so the
// contrast is a test, not a comment.
func TestPrefixLenMatcher_FullAddressDiscriminatesEXAnswers(t *testing.T) {
	const mine = "EX010101"
	const theirs = "EX010102"

	full := PrefixLenMatcher(mine, 0)
	if !full([]byte(mine + "1;")) {
		t.Errorf("full-address matcher rejected its OWN answer %q", mine+"1;")
	}
	if full([]byte(theirs + "1;")) {
		t.Errorf("full-address matcher accepted a DIFFERENT address's answer %q — that is the wrong-address correlation hazard", theirs+"1;")
	}

	bare := PrefixLenMatcher("EX", 0)
	if !bare([]byte(theirs + "1;")) {
		t.Errorf("bare \"EX\" matcher rejected %q — the hazard this contrast documents did not reproduce, so the doc comment's warning is no longer pinned by anything", theirs+"1;")
	}
}

// TestPrefixLenMatcher_DoesNotRetainFrame pins the contract: the matcher
// reads the frame it is handed and keeps no reference, so the engine may
// reuse or mutate that slice afterwards without changing a later verdict.
func TestPrefixLenMatcher_DoesNotRetainFrame(t *testing.T) {
	match := PrefixLenMatcher("ID", 7)
	frame := []byte("ID0800;")
	if !match(frame) {
		t.Fatalf("match(%q) = false, want true", frame)
	}
	copy(frame, "XX")
	if match(frame) {
		t.Errorf("match(%q) = true after the caller mutated the slice — the matcher must judge what it is handed, each call", frame)
	}
}
