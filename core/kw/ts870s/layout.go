// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

import "github.com/gm5dna/open-rig-programmer/core/kw"

// Layout is the TS-870S row, minted once and handed out by value — the
// same shape core/kw/ts480's layout480 var takes, over the second record
// type (kw.Layout870) rather than the family's kw.Layout.
var Layout = kw.MustNewLayout870(kw.Layout870Config{
	Model: "TS-870S", // matrix §1.1, the document's own printed name

	// MaxEXAddress: doc.go's own citation (ts870s:8468-8508), not the
	// matrix's — EX/menu inventory is out of this wave's scope, but the
	// field is structurally required (NewLayout870 refuses zero).
	MaxEXAddress: 68,

	// ModeNames: the eight nibbles that name a mode a channel can be in,
	// out of Format 2's ten (matrix §1.5). Nibbles 0 ("No mode") and 8
	// ("No Mode") are holes and are refused as legend entries by
	// NewLayout870 itself (kw.Mode.namesAMode() is false for both) — see
	// doc.go's A1 for why that refusal is this package's build-side
	// FieldMode-refusal register entry, not extra code here.
	ModeNames: map[kw.Mode]string{
		kw.ModeLSB:  "LSB",
		kw.ModeUSB:  "USB",
		kw.ModeCW:   "CW",
		kw.ModeFM:   "FM",
		kw.ModeAM:   "AM",
		kw.ModeFSK:  "FSK",
		kw.ModeCWR:  "CW-R",
		kw.ModeFSKR: "FSK-R",
	},

	// ChannelLo/ChannelHi: the one flat MEM bank, "00"-"99" (matrix §1.4).
	// Channel 99's P1=1 (Start/End frequency) half is unreachable through
	// this programme (matrix §1.4/§2.2, the TS-480 precedent); record870.go
	// itself refuses any P1 other than '0', which is what leaves that half
	// unreachable rather than anything this config states.
	ChannelLo: 0,
	ChannelHi: 99,
})
