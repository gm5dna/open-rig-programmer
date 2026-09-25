// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import "testing"

// TestToneIndexRoundTrip pins ToneForIndex/IndexForTone as exact inverses
// over the whole 50-entry chart (formerly duplicated per-driver in
// ft450d/ft950/ftdx9000; one copy here covers all three's shared body).
func TestToneIndexRoundTrip(t *testing.T) {
	for i := 0; i < 50; i++ {
		tone, ok := ToneForIndex(uint8(i))
		if !ok {
			t.Fatalf("ToneForIndex(%d) refused, want ok", i)
		}
		idx, ok := IndexForTone(tone)
		if !ok || idx != uint8(i) {
			t.Errorf("IndexForTone(ToneForIndex(%d)) = %d, %v, want %d, true", i, idx, ok, i)
		}
	}
	if _, ok := ToneForIndex(50); ok {
		t.Error("ToneForIndex(50) succeeded, want refused (chart is 0-49)")
	}
}
