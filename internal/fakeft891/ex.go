// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import (
	"fmt"
	"strings"
)

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/fakeft891/gen -csv transcription-b.csv -out exinventory_gen.go

// This file is fakeft891's own, independent model of the FT-891's EX (MENU)
// command — READ ONLY, exactly as internal/fakedx10 models the FTdx10's and
// internal/fakeradio the FT-710's: the manual documents a Set form and this
// fake does not implement it (doc.go's "What this fake deliberately does NOT
// model").
//
// # WHERE THE INVENTORY COMES FROM, AND WHY IT IS GENERATED
//
// The widths table this file expands (exGroups) is NOT hand-typed here. It is
// GENERATED, by the directive above, from this package's OWN COPY of
// TRANSCRIPTION B — transcription-b.csv beside this file, with PROVENANCE.md
// recording where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of the FT-891's two-source cross-check, and it is
// worth stating in full because it is the reason this package does not simply
// read the dialect's inventory:
//
//   - the DIALECT's inventory (core/cat/ft891/exinventory_gen.go) is generated
//     from TRANSCRIPTION A (core/cat/ft891/table2.csv) by internal/extable;
//   - THIS inventory is generated from TRANSCRIPTION B by internal/fakeft891/gen,
//     which imports nothing project-internal at all — not extable, not core/cat
//     (the recursive fence in imports_test.go enforces it, gen/ included, and
//     TestNoCoreImports_ReachesTheGenerator proves the scan really gets there);
//   - core/transport/ex_crosscheck_ft891_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined agent
// with no repository access and no sight of A, the group-boundary ledger or any
// row count (core/cat/ft891/crosscheck_test.go records all three artefacts, and
// hashes them). So a mis-read row in either transcription, or a defect in
// either generator, surfaces as a cross-check MISMATCH rather than as two
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
// Wire behaviour only: how many P2 items each P1 group has, and each item's raw
// P4 reply WIDTH. It records nothing about what a menu item MEANS — no name, no
// enum legend, no valid range. The names live in the dialect's inventory, which
// is the layer that has a reason to know them; this fake answers reads.
//
// ONE PRINTED DEFECT RIDES THROUGH IT UNCHANGED, and it is recorded rather than
// repaired: 0905 RPT SHIFT 50MHz prints Digits 1 against a legend that needs
// four, where its twin 0904 prints 4 for the same shape of legend. Both
// quarantined derivations read the printed 1, core/cat/ft891/crosscheck_test.go
// pins it as a manual defect, and this fake answers ONE byte there — because
// that is what the transcription says and no FT-891 has been asked which the
// radio answers. Three faithful readings of one wrong printed cell agree
// perfectly, so no comparison in this repository can catch this class; the pin
// is what makes it a deliberate state.
//
// # EX GRAMMAR (manual rev 1909-C, availability line 142, frames 513-522)
//
// Read frame (7 bytes): "EX" + P1(2) + P2(2) + ";" — a FOUR-digit wire address,
// where every registered sibling's is six. The chart's MENU Number is the whole
// address: 0803 is (P1,P2) = (08,03), and every item's P3 is zero (core/cat's
// EXAddressPair, which exists to carry exactly this).
// Answer frame: "EX" + address(4) + P4(n) + ";", where n is the address's own
// width: 1-5 raw ASCII digits. There is no text item on this chart, and its
// transcription carries no column that could describe one (gen/main.go's
// widthToken).
// Set frame: same shape with a P4 payload — NOT modelled; see handleEX.

// exAddr builds a four-digit wire address from two-digit P1/P2 strings.
func exAddr(p1, p2 string) string { return p1 + p2 }

// exDefaultDigit is the digit byte a numeric EX item's raw P4 defaults to.
//
// INVENTED — doc.go's register entry THE EX MENU VALUES ARE INVENTED. The
// FT-891 chart documents each item's VALID RANGE and its option legends and
// never a shipped default, so there is nothing to source a real one from; and
// `rigprog read --settings --fake --model FT-891` renders these bytes to a
// user, who must not read them as what an FT-891 ships with. It is fakeradio's
// convention, adopted because a placeholder that is obviously uniform is harder
// to mistake for evidence than a plausible-looking spread of values.
const exDefaultDigit = '0'

// exMaxWidth is the widest raw P4 field this chart declares, and the top of the
// width alphabet buildEXDefaults accepts.
//
// FIVE, where the FTdx10's numeric alphabet stops at four. It comes from
// exactly two rows — 0803 OTHER DISP and 0804 OTHER SHIFT, whose signed
// "-3000 Hz - 0 - +3000 Hz" parameter counts its sign as a digit — and both
// sides of the evidence pin those two addresses independently:
// gen/main_test.go's TestParseB_TheOnlyFiveWideRowsAre0803And0804 from B, and
// core/cat/ft891/crosscheck_test.go's widestRowDigits from A.
//
// THERE IS NO 'T' TOKEN AND NO exTextWidth. The FTdx10's inventory has a
// twelve-byte text item (MY CALL.) that answers spaces rather than zeros; this
// chart has no such row, and — the sharper point — its transcription carries no
// column from which one could be identified, so the generator refuses a width
// it cannot classify rather than inventing a token. gen/main.go's widthToken
// states what that does and does not claim.
const exMaxWidth = 5

// buildEXDefaults expands the generated exGroups into the full address ->
// default raw P4 map: a numeric width n defaults to n x exDefaultDigit.
//
// It PANICS on a malformed width token, mirroring defaultState's panics for
// factory-image constants (image.go). exGroups is a generated package-level
// table, so a token outside '1'..'5' is a defect in the generator or a
// hand-edit of its output — a programming error to catch at init, never a
// runtime input. gen/main.go's widthToken REFUSES to emit one (with the
// offending CSV line named, which is why the '5' is proved there rather than
// discovered here), and gen/main_test.go's staleness check refuses a generated
// file that has drifted from the CSV, so reaching this panic means one of those
// two was bypassed.
func buildEXDefaults() map[string]string {
	out := make(map[string]string)
	for _, g := range exGroups {
		for i, w := range g.widths {
			p2 := fmt.Sprintf("%02d", i+1)
			addr := exAddr(g.p1, p2)
			if w < '1' || w > '0'+exMaxWidth {
				panic(fmt.Sprintf("fakeft891: exGroups P1=%s item %d: malformed width token %q — regenerate with `go generate ./internal/fakeft891`", g.p1, i+1, w))
			}
			out[addr] = strings.Repeat(string(exDefaultDigit), int(w-'0'))
		}
	}
	return out
}

// exDefaults is the package-level default menu state, computed once from
// exGroups at package init.
var exDefaults = buildEXDefaults()

// EXDefaults returns a fresh copy of the fake's default menu state: four-digit
// wire address -> default raw P4 (numeric width n -> n x '0').
//
// THERE IS NO EXRuntimeDefaults HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with the M8c hardware observations overlaid — because it HAS hardware
// observations, and folding them into its transcription would have destroyed
// the very cross-check that transcription exists for. No FT-891 has ever been
// asked anything by this project, so there is nothing to overlay: what this
// function returns IS what a *Radio answers, and a second, empty table would be
// ceremony. When FT-891 evidence does arrive, fakeradio's split is the pattern
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

// exAddrLen is the wire length of an EX read body: P1(2) + P2(2). FOUR, not
// six: this radio's read frame is seven bytes where its siblings' are nine
// (ft891_layout.txt:513-522).
const exAddrLen = 4

// isEXAddr reports whether s is a syntactically valid four-digit EX wire
// address (it says nothing about whether that address names a known menu item).
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
// frame: "EX" + addr(4) + p4(n) + ";".
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
// READ ONLY. Any body that is not exactly four ASCII digits naming a KNOWN
// address draws "?;" with the state unchanged — which covers malformed bodies
// (wrong length, non-digit bytes), addresses the chart never enumerated, and
// SET-SHAPED bodies alike (a valid address immediately followed by a P4
// payload, e.g. "EX01010001;", is simply a too-long body to this handler). The
// first of those is doc.go's register entry AN OUT-OF-INVENTORY EX ADDRESS
// ANSWERS "?;", ASSUMED here as everywhere on this radio; the Set rejection is
// a deliberate modelling gap rather than a claim that a real FT-891 refuses EX
// Set.
//
// NOTE WHAT THE FOUR-DIGIT CHECK ALSO EXCLUDES: the six-digit read frame every
// registered sibling uses. A length check written as "at least four digits"
// would answer an FTdx10's frame with an FT-891's menu value, which is the
// wrong answer given confidently — the class of mistake a shared "fake core"
// would have made structural (doc.go, "A SIBLING of internal/fakedx10, not a
// refactor of it").
//
// Membership comes from the chart's own rows, via the generated inventory. The
// EX block's printed bound — "P1 : 0101 - 1803 (MENU Number)" — is not enforced
// as a range: it happens to be exactly the first and last rows transcribed, so
// enforcing it would add a second, redundant authority over the same fact and
// would disagree with the inventory the moment the two were ever edited apart.
func (r *Radio) handleEX(body []byte) []byte {
	addr := string(body)
	if !isEXAddr(addr) {
		return rejection
	}
	r.mu.Lock()
	p4, ok := r.exSettings[addr]
	r.mu.Unlock()
	if !ok {
		return rejection // register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;"
	}
	return buildEXAnswer(addr, p4)
}
