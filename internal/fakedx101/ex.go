// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"fmt"
	"strings"
)

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/fakedx101/gen -csv transcription-b.csv -out exinventory_gen.go

// This file is fakedx101's own, independent model of the FTDX101D's and
// FTDX101MP's EX (MENU) command — READ ONLY, exactly as internal/fakedx10
// models the FTdx10's and internal/fakeradio the FT-710's: the manual documents
// a Set form and this fake does not implement it (doc.go register entry 17).
//
// # ONE INVENTORY, TWO RADIOS
//
// The manual prints Table 2 "MENU Chart" ONCE for both models, and the
// capability matrix (§4) records that the manual's whole model-distinguishing
// surface is three places, none of them this chart. So a NewD radio and a NewMP
// radio share this projection byte for byte, and answer every EX read
// identically; the two differ on this wire in the ID answer and nowhere else
// (doc.go, "Two radios, one fake"). That claim is not left to this comment:
// core/transport/ex_crosscheck_ftdx101_test.go's ID-divergence leg drives the
// whole inventory at both constructors and requires identical bytes, and
// fakedx101_test.go's TestTheTwoModelsDifferOnlyInTheIDAnswer carries EX frames
// in its shared list.
//
// # WHERE THE INVENTORY COMES FROM, AND WHY IT IS GENERATED
//
// The widths table this file expands (exGroups) is NOT hand-typed here. It is
// GENERATED, by the directive above, from this package's OWN COPY of
// TRANSCRIPTION B — transcription-b.csv beside this file, with PROVENANCE.md
// recording where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of the FTdx101's two-source cross-check, and it is
// worth stating in full because it is the reason this package does not simply
// read the dialect's inventory:
//
//   - the DIALECT's inventory (core/cat/ftdx101/exinventory_gen.go) is generated
//     from TRANSCRIPTION A (core/cat/ftdx101/table2.csv) by internal/extable;
//   - THIS inventory is generated from TRANSCRIPTION B by
//     internal/fakedx101/gen, which imports nothing project-internal at all —
//     not extable, not core/cat (the recursive fence in imports_test.go enforces
//     it, gen/ included, and it was standing before gen/ existed);
//   - core/transport/ex_crosscheck_ftdx101_test.go proves the two agree, address
//     for address, width for width and shape for shape, and drives every address
//     over the wire at both models.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined agent
// with no repository access, no sight of A or of the group-boundary ledger, and
// no row count or address given — its own header block records that no text
// layer was consulted at all. core/cat/ftdx101/crosscheck_test.go binds those
// two to each other AND to the ledger, a third blind derivation. So a mis-read
// row in either transcription, or a defect in either generator, surfaces as a
// cross-check MISMATCH rather than as two tables quietly agreeing on the same
// wrong number. Deriving this fake's inventory from A — or from the dialect, or
// with extable's parser — would collapse both sides onto one source and throw
// that property away.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF, exactly as it is for the FT-710 and the FTdx10
// (core/transport/ex_crosscheck_test.go's own standing instruction), and exactly
// the STOP core/cat/ftdx101/crosscheck_test.go declares for its three-way legs.
//
// # WHAT THE TABLE MODELS
//
// Wire behaviour only: how many P3 items each (P1,P2) subgroup has, and each
// item's raw P4 reply WIDTH. It records nothing about what a menu item MEANS —
// no name, no enum legend, no valid range. The names live in the dialect's
// inventory, which is the layer that has a reason to know them; this fake
// answers reads.
//
// # EX GRAMMAR (manual rev 2308-L)
//
// Read frame (9 bytes): "EX" + P1(2) + P2(2) + P3(2) + ";" — a six-digit wire
// address.
// Answer frame: "EX" + address(6) + P4(n) + ";", where n is the address's own
// width: 1-4 raw ASCII digits, or the one 12-byte text field (MY CALL., at
// address 040101 — this chart has exactly one, as the FTdx10's does, where the
// FT-710's has six).
// Set frame: same shape with a P4 payload — NOT modelled; see handleEX.

// exAddr builds a six-digit wire address from two-digit P1/P2/P3 strings.
func exAddr(p1, p2, p3 string) string { return p1 + p2 + p3 }

// exDefaultDigit is the digit byte a numeric EX item's raw P4 defaults to.
//
// INVENTED — doc.go register entry 4, which landed with the fake's core one task
// ahead of this file deliberately, so that the honesty was on record before the
// first invented value existed. The FTdx101 chart documents each item's VALID
// RANGE and its option legends and never a shipped default, so there is nothing
// to source a real one from; and `rigprog read --settings --fake --model
// FTdx101D` renders these bytes to a user, who must not read them as what an
// FTDX101D or an FTDX101MP ships with. It is fakeradio's convention (its
// exDefaultDigit), adopted because a placeholder that is obviously uniform is
// harder to mistake for evidence than a plausible-looking spread of values.
const exDefaultDigit = '0'

// exTextWidth is the fixed wire width of a 'T' (text) EX item's raw P4 field:
// the 12 B's digits column prints for MY CALL., the one row whose text flag is
// true. A text item defaults to that many SPACES rather than zeros — an empty
// call-sign field, not the digit zero twelve times.
const exTextWidth = 12

// buildEXDefaults expands the generated exGroups into the full address ->
// default raw P4 map: a numeric width n defaults to n x exDefaultDigit; a text
// item defaults to exTextWidth spaces.
//
// It PANICS on a malformed width token, mirroring defaultState's panics for
// factory-image constants (image.go) and newRadio's for a bad CAT ID. exGroups
// is a generated package-level table, so a token outside {'1'..'4','T'} is a
// defect in the generator or a hand-edit of its output — a programming error to
// catch at init, never a runtime input. gen/main.go's widthToken refuses to emit
// one, and gen/main_test.go's staleness check refuses a generated file that has
// drifted from the CSV, so reaching this panic means one of those two was
// bypassed.
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
				panic(fmt.Sprintf("fakedx101: exGroups P1=%s P2=%s item %d: malformed width token %q — regenerate with `go generate ./internal/fakedx101`", g.p1, g.p2, i+1, w))
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
// -> 12 spaces). It is the SAME map for a D and for an MP, which is the point of
// the "one inventory, two radios" section above.
//
// THERE IS NO EXRuntimeDefaults HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with the M8c hardware observations overlaid — because it HAS hardware
// observations, and folding them into its transcription would have destroyed the
// very cross-check that transcription exists for. No FTdx101 of either model has
// ever been asked anything by this project, so there is nothing to overlay: what
// this function returns IS what a *Radio answers, and a second, empty table
// would be ceremony. When FTdx101 evidence does arrive, fakeradio's split is the
// pattern to copy — a separate overrides table with its own citation, PER MODEL
// (doc.go's register preamble: a capture from one radio is never evidence about
// the other), never an edit to exGroups or to the CSV it is generated from.
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

// buildEXAnswer concatenates addr and the stored raw P4 into an EX answer frame:
// "EX" + addr(6) + p4(n) + ";".
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
// SET-SHAPED bodies alike (a valid address immediately followed by a P4 payload,
// e.g. "EX0101011;", is simply a too-long body to this handler). Both of those
// last two are doc.go register entry 17: the "?;" for an out-of-inventory
// address is ASSUMED here, as it is for the FTdx10, where the FT-710's was
// OBSERVED at M8c — and it is assumed twice over on these radios, since the
// rejection convention ITSELF is unattested for them (register entry 16: the
// layout-preserved extraction of rev 2308-L contains no '?' character at all).
//
// The six-digit check does NOT consult any P1/P2/P3 range: membership comes from
// the chart's own rows, via the generated inventory. An EX read of 05xxxx
// answers "?;" because no such item was transcribed, not because a range was
// enforced — the same construction internal/fakedx10 uses, and it carries the
// same anomaly with it. THIS MANUAL'S EX GRAMMAR BLOCK AND ITS TABLE 2 DISAGREE:
// the block states "P1 : 01 - 05" (layout 700) and the chart populates 01-04
// only, ending at (04,03,02) PIXEL (layout 962). core/cat/ftdx101/doc.go's
// "header-vs-chart anomaly" section records that UNRESOLVED — unlike the
// FT-710's it cannot be put to hardware — so what a real FTdx101 answers to
// 050101 is doubly unknown, and this fake's "?;" is "I hold no such item"
// rather than an enforcement of either bound (doc.go register entry 17).
//
// It is deliberately a METHOD taking no model into account. Both radios hold the
// same seeded exSettings map, so the D and the MP answer identically; the only
// per-model reply builder in this package is buildIDAnswer (doc.go).
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
