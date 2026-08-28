// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// TestGate_RefusesEveryMutatingOrLookalikeCommand names, by command byte
// and by what it does, every frame this document prints that could
// mutate an IC-905 or be mistaken for a memory record. civtest's own
// corpus refuses a generic set of seventeen; these six are this MODEL's
// (matrix section 3.16, ADDED entries A1-A4), and none of them is
// producible by any core/civ builder.
func TestGate_RefusesEveryMutatingOrLookalikeCommand(t *testing.T) {
	p := ic905.Profile()
	for _, tc := range []struct {
		name  string
		frame []byte
	}{
		{"1A 01 band stacking register — 47 bytes of this same record shape (A1)",
			[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x01, 0x01, 0x01, 0xFD}},
		{"1A 02 memory keyer contents (A4)",
			[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x02, 0x01, 0xFD}},
		{"09 Memory write (A2)", []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x09, 0xFD}},
		{"0A Memory copy to VFO (A2)", []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x0A, 0xFD}},
		{"0B Memory clear (A2)", []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x0B, 0xFD}},
		{"A0 Select the Memory group (A3)",
			[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0xA0, 0x00, 0x00, 0xFD}},
		{"1A 05 set mode — the whole Icom settings surface, out of scope",
			[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x05, 0x01, 0x42, 0x00, 0xFD}},
		{"the documented 1A 00 CLEAR form — this tier ships no erase path",
			[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x01, 0xFF, 0xFD}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p.AllowedCommand(tc.frame) {
				t.Errorf("AllowedCommand(% X) = true — nothing in this tier may send it", tc.frame)
			}
		})
	}
}
