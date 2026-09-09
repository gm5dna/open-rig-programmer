// SPDX-License-Identifier: GPL-3.0-or-later

package kw

// EXItem is one transcribed row of a Kenwood EX parameter list: an address
// plus the chart's own function name and field width.
//
// EVERY FIELD NAME IS FIXED BY internal/extable's RENDERER, which emits
// each item as a literal naming all eight (extable.go's item Fprintf). Two
// of the eight are structurally present and always empty on this family,
// and each says so below rather than being quietly tolerated.
//
// THERE ARE THREE OF THESE INVENTORIES, NOT TWO. The TS-590S and the
// TS-590SG do NOT share a menu table: the book prints two separate lists,
// "EX Command Parameter List (for TS-590S)" at 590:564 and the SG's at
// 590:744, over COLLIDING addresses with different meanings — the SG's list
// is the S's shifted by two with a new read-only row at the top, so every
// address from 000 up means something different on the two radios
// (590:569 vs 590:749). The TS-480 has its own again (480:424-539).
type EXItem struct {
	// Addr is the menu address. Its P2 and P3 are zero on every Kenwood
	// row; see EXAddress.
	Addr EXAddress
	// P1Label and P2Label are the group labels a Yaesu MENU chart prints.
	// They are ALWAYS EMPTY here: all five Kenwood profiles register
	// LabelsAbsent, because these parameter lists print a menu number, a
	// function name and a parameter legend, and no group hierarchy at
	// all. internal/extable's ParseCSV requires the columns to be blank
	// under that policy and RenderGo emits "" for both.
	P1Label string
	P2Label string
	// Name is the chart's Function column, verbatim.
	Name string
	// Digits is the chart's printed field width for this row's parameter.
	//
	// It is a WIDTH CLAIM AND NOTHING MORE. The TS-590SG's menu 000 —
	// "Version information (4 ASCII characters) read only" (590:749) — is
	// carried here as Digits 4, Text false, on the FT-891's precedent for
	// version rows; that asserts four characters and asserts NOTHING about
	// the string's grammar, which is A13 and stays assumed.
	Digits int
	// Text marks the chart's free-text row — the one a transcriber must
	// stop at — not "any row whose parameter happens to be characters".
	// There is exactly one on each 590 row (menu 087 on the S, the same
	// string renumbered to 001 on the SG, both "up to 8 ASCII
	// characters", 590:741 and 590:750) and NONE on the TS-480, whose
	// profile registers TextRowsAbsent.
	Text bool
	// ObservedReadWidth and ObservedReadShape carry a hardware
	// observation's P4 width and shape. They are ALWAYS ZERO AND EMPTY on
	// this family: all three profiles register ObservationsAbsent, because
	// no Kenwood radio has been on a wire for this project. A19 is the
	// assumption they would lift, and its lift is an exhaustive EX sweep
	// of the row's whole printed domain — a ceiling is not lifted by a
	// sample.
	ObservedReadWidth int
	ObservedReadShape string
}

// CopyEXItems returns an independent copy of src, so a caller may mutate
// what it is handed with no effect on the source.
//
// IT IS EXPORTED FOR THE THREE PACKAGE ACCESSORS — ts590.EXItemsS,
// ts590.EXItemsSG and ts480.EXItems — which each stand over a generated
// package-level slice and must never hand it out directly. Those three
// packages hold BOOTSTRAP inventories that are empty until lane P generates
// them, so a copy test written in either of them would assert nothing today
// while looking as though it did; pinning the helper HERE, where a
// non-empty slice can be constructed, is what makes their behaviour
// genuinely covered. TestCopyEXItems_ReturnsAnIndependentSlice is that pin.
//
// EXItem contains no pointers and no slices, so a shallow element copy is a
// deep one. If a field is ever added that does contain one, this function
// is the single place that has to change.
func CopyEXItems(src []EXItem) []EXItem {
	out := make([]EXItem, len(src))
	copy(out, src)
	return out
}
