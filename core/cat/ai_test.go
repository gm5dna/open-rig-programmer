// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"testing"
)

// TestBuildAISet_G2: golden vector G2, "AI0;" (auto-information off), plus
// the "on" counterpart.
func TestBuildAISet_G2(t *testing.T) {
	tests := []struct {
		on   bool
		want string
	}{
		{false, "AI0;"},
		{true, "AI1;"},
	}
	for _, tc := range tests {
		if got := string(FT710.BuildAISet(tc.on).Bytes()); got != tc.want {
			t.Errorf("BuildAISet(%v) = %q, want %q", tc.on, got, tc.want)
		}
	}
}
