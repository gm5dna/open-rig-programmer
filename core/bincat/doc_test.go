// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import "testing"

// TestOpcodeTablePinsTheEvidence pins doc.go's and profile.go's central
// claim — the nine opcodes this milestone ships, and no others — against
// the FT-890/900 CAT Commands table's own hex values, so a future edit
// that silently renumbers one is caught here rather than only at a
// hardware trial.
func TestOpcodeTablePinsTheEvidence(t *testing.T) {
	want := map[string]byte{
		"OpStore":        0x03,
		"OpABSelect":     0x05,
		"OpClarifier":    0x09,
		"OpSetFreq":      0x0A,
		"OpSetMode":      0x0C,
		"OpStatusUpdate": 0x10,
		"OpShift":        0x84,
		"OpTone":         0x90,
		"OpOffset":       0xF9,
	}
	got := map[string]byte{
		"OpStore":        OpStore,
		"OpABSelect":     OpABSelect,
		"OpClarifier":    OpClarifier,
		"OpSetFreq":      OpSetFreq,
		"OpSetMode":      OpSetMode,
		"OpStatusUpdate": OpStatusUpdate,
		"OpShift":        OpShift,
		"OpTone":         OpTone,
		"OpOffset":       OpOffset,
	}
	for name, wantByte := range want {
		if got[name] != wantByte {
			t.Errorf("%s = %#02x, want %#02x", name, got[name], wantByte)
		}
	}
}
