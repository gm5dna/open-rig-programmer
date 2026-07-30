// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import (
	"fmt"
	"strings"
)

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/fakedx10/gen -csv transcription-b.csv -out exinventory_gen.go

// This file is fakedx10's own, independent model of the FTdx10's EX (MENU)
// command — READ ONLY, exactly as internal/fakeradio models the FT-710's (its
// register item 24): the manual documents a Set form and this fake does not
// implement it (register entry 17 below/in doc.go).
//
// # WHERE THE INVENTORY COMES FROM, AND WHY IT IS GENERATED
//
// The widths table this file expands (exGroups) is NOT hand-typed here. It is
// GENERATED, by the directive above, from this package's OWN COPY of
// TRANSCRIPTION B — transcription-b.csv beside this file, with PROVENANCE.md
// recording where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of the FTdx10's two-source cross-check, and it is
// worth stating in full because it is the reason this package does not simply
// read the dialect's inventory:
//
//   - the DIALECT's inventory (core/cat/ftdx10/exinventory_gen.go) is generated
//     from TRANSCRIPTION A (core/cat/ftdx10/table2.csv) by internal/extable;
//   - THIS inventory is generated from TRANSCRIPTION B by internal/fakedx10/gen,
//     which imports nothing project-internal at all — not extable, not core/cat
//     (the recursive fence in imports_test.go enforces it, gen/ included);
//   - core/transport/ex_crosscheck_ftdx10_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined agent
// with no repository access and no sight of A, the group-boundary ledger or any
// row count (core/cat/ftdx10/crosscheck_test.go records all three artefacts and
// their adjudications). So a mis-read row in either transcription, or a defect
// in either generator, surfaces as a cross-check MISMATCH rather than as two
// tables quietly agreeing on the same wrong number. Deriving this fake's
// inventory from A — or from the dialect, or with extable's parser — would
// collapse both sides onto one source and throw that property away.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF, exactly as it is for the FT-710
// (core/transport/ex_crosscheck_test.go's own standing instruction).
//
// # WHAT THE TABLE MODELS
//
// Wire behaviour only: how many P3 items each (P1,P2) subgroup has, and each
// item's raw P4 reply WIDTH. It records nothing about what a menu item MEANS —
// no name, no enum legend, no valid range. The names live in the dialect's
// inventory, which is the layer that has a reason to know them; this fake
// answers reads.
//
// # EX GRAMMAR (manual rev 2308-F, availability line 233, frames 636-645)
//
// Read frame (9 bytes): "EX" + P1(2) + P2(2) + P3(2) + ";" — a six-digit wire
// address.
// Answer frame: "EX" + address(6) + P4(n) + ";", where n is the address's own
// width: 1-4 raw ASCII digits, or the one 12-byte text field (MY CALL., at
// address 040101 — the FTdx10's chart has exactly one, where the FT-710's has
// six).
// Set frame: same shape with a P4 payload — NOT modelled; see handleEX.

// exAddr builds a six-digit wire address from two-digit P1/P2/P3 strings.
func exAddr(p1, p2, p3 string) string { return p1 + p2 + p3 }

// exDefaultDigit is the digit byte a numeric EX item's raw P4 defaults to.
//
// INVENTED — doc.go register entry 4, which landed with the fake's core ahead
// of this file deliberately. The FTdx10 chart documents each item's VALID RANGE
// and its option legends and never a shipped default, so there is nothing to
// source a real one from; and `rigprog read --settings --fake --model FTdx10`
// renders these bytes to a user, who must not read them as what an FTdx10 ships
// with. It is fakeradio's convention (its exDefaultDigit), adopted because a
// placeholder that is obviously uniform is harder to mistake for evidence than
// a plausible-looking spread of values.
const exDefaultDigit = '0'

// exTextWidth is the fixed wire width of a 'T' (text) EX item's raw P4 field:
// the 12 of B's "Up to 12 characters" cell for MY CALL., which is also what its
// Digits column prints. A text item defaults to that many SPACES rather than
// zeros — an empty call-sign field, not the digit zero twelve times.
const exTextWidth = 12

// buildEXDefaults expands the generated exGroups into the full address ->
// default raw P4 map: a numeric width n defaults to n x exDefaultDigit; a text
// item defaults to exTextWidth spaces.
//
// It PANICS on a malformed width token, mirroring defaultState's panics for
// factory-image constants (image.go). exGroups is a generated package-level
// table, so a token outside {'1'..'4','T'} is a defect in the generator or a
// hand-edit of its output — a programming error to catch at init, never a
// runtime input. gen/main.go's widthToken refuses to emit one, and
// gen/main_test.go's staleness check refuses a generated file that has drifted
// from the CSV, so reaching this panic means one of those two was bypassed.
func buildEXDefaults() map[string]string {
	out := make(map[string]string)
	for _, g := range exGroups {
		for i, w := range g.widths {
			p3 := fmt.Sprintf("%02d", i+1)
			addr := exAddr(g.p1, g.p2, p3)
			switch {
			case w == 'T':
				out[addr] = strings.Repeat(" ", exTextWidth)
			case w >= '1' && w <= '4':
				out[addr] = strings.Repeat(string(exDefaultDigit), int(w-'0'))
			default:
				panic(fmt.Sprintf("fakedx10: exGroups P1=%s P2=%s item %d: malformed width token %q — regenerate with `go generate ./internal/fakedx10`", g.p1, g.p2, i+1, w))
			}
		}
	}
	return out
}

// exDefaults is the package-level default menu state, computed once from
// exGroups at package init.
var exDefaults = buildEXDefaults()

// EXDefaults returns a fresh copy of the fake's default menu state: six-digit
// wire address -> default raw P4 (numeric width n -> n x '0'; the one text item
// -> 12 spaces).
//
// THERE IS NO EXRuntimeDefaults HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with the M8c hardware observations overlaid — because it HAS hardware
// observations, and folding them into its transcription would have destroyed
// the very cross-check that transcription exists for. No FTdx10 has ever been
// asked anything by this project, so there is nothing to overlay: what this
// function returns IS what a *Radio answers, and a second, empty table would be
// ceremony. When FTdx10 evidence does arrive, fakeradio's split is the pattern
// to copy — a separate overrides table with its own citation, never an edit to
// exGroups or to the CSV it is generated from.
//
// Test-inspection API, and the fake's half of the cross-check: every call
// returns an independent map, so mutating one call's result can never affect
// another call's, nor any *Radio's own stored exSettings.
func EXDefaults() map[string]string {
	out := make(map[string]string, len(exDefaults))
	for k, v := range exDefaults {
		out[k] = v
	}
	return out
}

// --- EX command handler ---

// exAddrLen is the wire length of an EX read body: P1(2) + P2(2) + P3(2).
const exAddrLen = 6

// isEXAddr reports whether s is a syntactically valid six-digit EX wire address
// (it says nothing about whether that address names a known menu item).
func isEXAddr(s string) bool {
	if len(s) != exAddrLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// buildEXAnswer concatenates addr and the stored raw P4 into an EX answer
// frame: "EX" + addr(6) + p4(n) + ";".
func buildEXAnswer(addr, p4 string) []byte {
	out := make([]byte, 0, 2+len(addr)+len(p4)+1)
	out = append(out, 'E', 'X')
	out = append(out, addr...)
	out = append(out, p4...)
	out = append(out, ';')
	return out
}

// handleEX validates and answers an EX body (the frame bytes after "EX", before
// the trailing ';').
//
// READ ONLY. Any body that is not exactly six ASCII digits naming a KNOWN
// address draws "?;" with the state unchanged — which covers malformed bodies
// (wrong length, non-digit bytes), addresses the chart never enumerated, and
// SET-SHAPED bodies alike (a valid address immediately followed by a P4
// payload, e.g. "EX0301051;", is simply a too-long body to this handler). Both
// of those last two are doc.go register entry 17: the "?;" for an
// out-of-inventory address is ASSUMED here where the FT-710's was OBSERVED at
// M8c, and the Set rejection is a deliberate modelling gap rather than a claim
// that a real FTdx10 refuses EX Set.
//
// Note what the six-digit check does NOT do: it does not consult the EX grammar
// block's own P1/P2/P3 ranges. Those disagree with the chart on this radio
// (the block says "P1 : 01 - 05"; the chart populates 01-04 and has no P1=05
// group at all — core/cat/ftdx10/doc.go records the anomaly UNRESOLVED, since
// unlike the FT-710's it cannot be put to hardware). Membership here comes from
// the chart's own rows, via the generated inventory, so an EX read of 05xxxx
// answers "?;" because no such item was transcribed — not because a range was
// enforced.
func (r *Radio) handleEX(body []byte) []byte {
	addr := string(body)
	if !isEXAddr(addr) {
		return rejection
	}
	r.mu.Lock()
	p4, ok := r.exSettings[addr]
	r.mu.Unlock()
	if !ok {
		return rejection // register entry 17
	}
	return buildEXAnswer(addr, p4)
}
