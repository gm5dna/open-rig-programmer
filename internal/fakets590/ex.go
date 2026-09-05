// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"fmt"
	"strings"
)

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/fakets590/gen -csv transcription-b-590s.csv -out exinventory590s_gen.go -var exWidths590S
//go:generate go run github.com/gm5dna/open-rig-programmer/internal/fakets590/gen -csv transcription-b-590sg.csv -out exinventory590sg_gen.go -var exWidths590SG

// This file is fakets590's own, independent model of the EX (MENU) command on
// both 590 rows — READ ONLY, exactly as internal/fakeft891 models the
// FT-891's: the book documents a Set form and this fake does not implement it
// (doc.go's "What this fake deliberately does NOT model").
//
// # WHERE THE INVENTORIES COME FROM, AND WHY THEY ARE GENERATED
//
// The two widths tables this file expands are NOT hand-typed here. They are
// GENERATED, by the directives above, from this package's OWN COPIES of
// TRANSCRIPTION B — transcription-b-590s.csv and transcription-b-590sg.csv
// beside this file, with PROVENANCE.md recording where the copies came from
// and why they are copies rather than moves.
//
// That is the whole mechanism of this family's two-source cross-check, and it
// is worth stating in full because it is the reason this package does not
// simply read the codec's inventory:
//
//   - the CODEC's inventories (core/kw/ts590/exinventory590s_gen.go and
//     exinventory590sg_gen.go) are generated from TRANSCRIPTION A
//     (core/kw/ts590/menu590s.csv and menu590sg.csv) by internal/extable;
//   - THESE inventories are generated from TRANSCRIPTION B by
//     internal/fakets590/gen, which imports nothing project-internal at all
//     (the recursive fence in imports_test.go enforces it, gen/ included, and
//     TestNoCoreImports_ReachesTheGenerator proves the scan really gets
//     there);
//   - core/transport/ex_crosscheck_ts590_test.go proves the two agree,
//     address for address and width for width, on BOTH rows, and drives every
//     address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined
// agent with no repository access and no sight of A, the page ledger or any
// row count (core/kw/ts590/crosscheck_test.go records all three artefacts and
// hashes them). So a mis-read row in either transcription, or a defect in
// either generator, surfaces as a cross-check MISMATCH rather than as two
// tables quietly agreeing on the same wrong number. Deriving these tables from
// A — or from the codec, or with extable's parser — would collapse both sides
// onto one source and throw that property away.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF.
//
// # TWO TABLES, NOT ONE, AND THAT IS THE POINT OF THE ROW ARGUMENT
//
// The book prints TWO parameter lists over COLLIDING addresses: "EX Command
// Parameter List (for TS-590S)" (590:564) and the TS-590SG's (590:744). The
// SG's is the S's shifted by two with a read-only version row at the top, so
// every address from 000 up means something different on the two radios —
// menu 000 is Display brightness on the S (590:569) and Version information on
// the SG (590:749). A fake that served one list to both rows would answer
// plausible bytes for the wrong menu, everywhere, silently; the row is
// required here for the same reason it is required at New.
//
// # WHAT THE TABLES MODEL
//
// Wire behaviour only: which menu numbers each row's chart has, and each
// one's raw P5 reply WIDTH. They record nothing about what a menu MEANS — no
// name, no legend, no valid range. The names live in the codec's inventories,
// which are the layer with a reason to know them; this fake answers reads.
//
// # EX GRAMMAR (590:540-561)
//
// Read frame (10 bytes): "E X P1 P1 P1 P2 P2 P3 P4 ;" — a THREE-digit menu
// number in P1 (590:543-544), then the three printed constants P2 "00"
// (590:546-547), P3 '0' (590:548-550) and P4 '0' (590:551-553).
// Answer frame: the same ten positions with P5 — "String of alphanumeric
// characters for the Menu setting (variable length)" (590:554-556) — inserted
// before the terminator.
// Set frame: the same shape as the answer — NOT modelled; see handleEX.

// exWire renders a menu number as the three ASCII digits the frame carries.
func exWire(menu int) string { return fmt.Sprintf("%03d", menu) }

// exDefaultDigit is the digit byte a menu's raw P5 defaults to.
//
// INVENTED — doc.go's register entry THE EX MENU VALUES ARE INVENTED. The two
// 590 charts print each menu's available SETTINGS and never a shipped default,
// so there is nothing to source a real one from; and `rigprog read --settings
// --fake` renders these bytes to a user, who must not read them as what a
// TS-590 ships with. A placeholder that is obviously uniform is harder to
// mistake for evidence than a plausible-looking spread of values.
const exDefaultDigit = '0'

// exMaxWidth is the widest raw P5 either 590 chart declares, and the top of
// the width alphabet buildEXDefaults accepts.
//
// EIGHT, from the one free-text row each list prints — "Power on message …
// up to 8 ASCII characters", menu 087 on the S and the same string renumbered
// to 001 on the SG (590:741, 590:750) — which is also the widest P5 the
// printed frame grid has room for, reaching position 17 before the ';'
// (590:545-547). gen/main.go's maxWidth carries the same number on the
// generator's side, and gen/main_test.go pins it from the committed CSVs.
const exMaxWidth = 8

// widthsFor returns the generated widths table for row: one width token per
// menu number, indexed by the menu number itself.
//
// It PANICS on any other row, as New does, and for the same reason: every call
// site passes a compile-time-known constant, and a defaulted table would serve
// one sibling's menu chart under the other's name.
func widthsFor(row Row) string {
	switch row {
	case RowS:
		return exWidths590S
	case RowSG:
		return exWidths590SG
	default:
		panic(fmt.Sprintf("fakets590: no EX inventory for row %v — the row is REQUIRED and has no default (the book prints two disjoint parameter lists, 590:564 and 590:744)", row))
	}
}

// buildEXDefaults expands a generated widths table into the full address ->
// default raw P5 map: a width token n defaults to n x exDefaultDigit.
//
// It PANICS on a malformed width token, mirroring defaultState's panics for
// factory-image constants (image.go). The widths tables are generated
// package-level constants, so a token outside '1'..'8' is a defect in the
// generator or a hand-edit of its output — a programming error to catch at
// init, never a runtime input. gen/main.go's widthToken REFUSES to emit one
// (with the offending CSV line named, which is why the alphabet's top is
// proved there rather than discovered here), and gen/main_test.go's staleness
// check refuses a generated file that has drifted from the CSV, so reaching
// this panic means one of those two was bypassed.
func buildEXDefaults(row Row) map[string]string {
	widths := widthsFor(row)
	out := make(map[string]string, len(widths))
	// Indexed by BYTE, not by rune: the table is one ASCII digit per menu
	// (gen/main.go's widthToken), so a byte index is the intended menu number
	// and `range` over the string would give a rune index instead — the same
	// value here because the alphabet is ASCII, but not what the loop means to
	// compute.
	for menu := 0; menu < len(widths); menu++ {
		w := widths[menu]
		if w < '1' || w > '0'+exMaxWidth {
			panic(fmt.Sprintf("fakets590: %v menu %03d: malformed width token %q — regenerate with `go generate ./internal/fakets590`", row, menu, w))
		}
		out[exWire(menu)] = strings.Repeat(string(exDefaultDigit), int(w-'0'))
	}
	return out
}

// The two rows' default menu states, computed once at package init.
var (
	exDefaultsS  = buildEXDefaults(RowS)
	exDefaultsSG = buildEXDefaults(RowSG)
)

// EXDefaults returns a fresh copy of row's default menu state: three-digit
// wire address -> default raw P5 (width n -> n x '0').
//
// THERE IS NO RUNTIME-OVERRIDE TABLE HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with hardware observations overlaid — because it HAS observations, and
// folding them into its transcription would have destroyed the very
// cross-check that transcription exists for. No TS-590S and no TS-590SG has
// ever been asked anything by this project, so there is nothing to overlay:
// what this function returns IS what a *Radio answers. When 590 evidence does
// arrive, fakeradio's split is the pattern to copy — a separate overrides
// table with its own citation, never an edit to a widths table or to the CSV
// it is generated from.
//
// Test-inspection API, and the fake's half of the cross-check: every call
// returns an independent map, so mutating one call's result can never affect
// another call's, nor any *Radio's own stored exSettings.
func EXDefaults(row Row) map[string]string {
	var src map[string]string
	switch row {
	case RowS:
		src = exDefaultsS
	case RowSG:
		src = exDefaultsSG
	default:
		// widthsFor's panic, reached through the same door: EXDefaults is the
		// cross-check's entry point, and a defaulted row there would compare
		// the codec's S inventory against whichever table happened to be
		// first.
		widthsFor(row)
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// --- EX command handler ---

// exBodyLen is the length of an EX READ body — the frame bytes after "EX" and
// before the ';': P1(3) + P2(2) + P3(1) + P4(1). SEVEN, making the whole read
// frame ten bytes (590:549-552).
const exBodyLen = 7

// The three printed constants of the EX frame, named once because the answer
// builder emits them and the read validator requires them (590:546-553).
const (
	exP2Printed = "00"
	exP3Printed = '0'
	exP4Printed = '0'
)

// isEXReadBody reports whether body is a syntactically valid EX read body: a
// three-digit menu number followed by the three printed constants. It says
// nothing about whether that menu number names a row this radio has.
//
// NOTE WHAT THE EXACT-LENGTH CHECK ALSO EXCLUDES: an FT-891's seven-byte read
// frame and an FTdx10's nine-byte one. A check written as "at least three
// digits" would answer a Yaesu frame with a Kenwood menu value, which is the
// wrong answer given confidently — the class of mistake a shared "fake core"
// would have made structural (doc.go, THE HARD RULE).
func isEXReadBody(body []byte) bool {
	if len(body) != exBodyLen {
		return false
	}
	return allDigits(string(body[:3])) &&
		string(body[3:5]) == exP2Printed &&
		body[5] == exP3Printed &&
		body[6] == exP4Printed
}

// buildEXAnswer assembles an answer frame: the read's ten positions with P5
// inserted before the terminator (590:554-560).
func buildEXAnswer(addr, p5 string) []byte {
	out := make([]byte, 0, 2+len(addr)+len(exP2Printed)+2+len(p5)+1)
	out = append(out, 'E', 'X')
	out = append(out, addr...)
	out = append(out, exP2Printed...)
	out = append(out, exP3Printed, exP4Printed)
	out = append(out, p5...)
	out = append(out, ';')
	return out
}

// handleEX validates and answers an EX body (the frame bytes after "EX",
// before the trailing ';').
//
// READ ONLY. Any body that is not the printed ten-position read naming a menu
// number THIS ROW's chart has draws "?;" with the state unchanged — which
// covers malformed bodies, a P2/P3/P4 that is not the printed constant, menu
// numbers outside this row's own domain, and SET-SHAPED bodies alike (the
// read's seven bytes followed by a P5 payload is simply a too-long body to
// this handler). The out-of-inventory refusal is doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;", ASSUMED here as everywhere on
// this pair; the Set rejection is a deliberate modelling gap rather than a
// claim that a real TS-590 refuses EX Set.
//
// MEMBERSHIP COMES FROM THE CHART'S OWN ROWS, via the generated inventory —
// not from the printed range. Each row's EX block prints a domain, "000 ~ 087"
// and "000 ~ 099" (590:543-544), and it happens to be exactly the first and
// last rows transcribed; enforcing it separately would add a second, redundant
// authority over the same fact and would disagree with the inventory the
// moment the two were ever edited apart.
func (r *Radio) handleEX(body []byte) []byte {
	if !isEXReadBody(body) {
		return rejection
	}
	addr := string(body[:3])
	r.mu.Lock()
	p5, ok := r.exSettings[addr]
	r.mu.Unlock()
	if !ok {
		return rejection // register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;"
	}
	return buildEXAnswer(addr, p5)
}
