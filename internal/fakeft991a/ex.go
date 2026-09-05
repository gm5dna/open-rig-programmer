// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import "strings"

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/fakeft991a/gen -csv transcription-b.csv -out exinventory_gen.go

// This file is fakeft991a's own, independent model of the FT-991A's EX (MENU)
// command — READ ONLY, exactly as internal/fakeft891 models the FT-891's,
// internal/fakedx10 the FTdx10's and internal/fakeradio the FT-710's: the
// manual documents a Set form and this fake does not implement it (doc.go's
// "What this fake deliberately does NOT model").
//
// # WHERE THE INVENTORY COMES FROM, AND WHY IT IS GENERATED
//
// The table this file expands (exItems) is NOT hand-typed here. It is
// GENERATED, by the directive above, from this package's OWN COPY of
// TRANSCRIPTION B — transcription-b.csv beside this file, with PROVENANCE.md
// recording where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of the FT-991A's two-source cross-check, and it
// is worth stating in full because it is the reason this package does not
// simply read the dialect's inventory:
//
//   - the DIALECT's inventory (core/cat/ft991a/exinventory_gen.go) is generated
//     from TRANSCRIPTION A (core/cat/ft991a/table2.csv) by internal/extable;
//   - THIS inventory is generated from TRANSCRIPTION B by
//     internal/fakeft991a/gen, which imports nothing project-internal at all —
//     not extable, not core/cat (the recursive fence in imports_test.go
//     enforces it, gen/ included, and TestNoCoreImports_ReachesTheGenerator
//     proves the scan really gets there);
//   - core/transport/ex_crosscheck_ft991a_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined agent
// with no repository access and no sight of A or of any row count
// (core/cat/ft991a/crosscheck_test.go records and hashes the artefacts). So a
// mis-read row in either transcription, or a defect in either generator,
// surfaces as a cross-check MISMATCH rather than as two tables quietly agreeing
// on the same wrong number. Deriving this fake's inventory from A — or from the
// dialect, or with extable's parser — would collapse both sides onto one source
// and throw that property away.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF, exactly as it is for the FT-710
// (core/transport/ex_crosscheck_test.go's own standing instruction).
//
// # ROW 087 IS ABSENT FROM THIS INVENTORY, AS IT IS FROM THE DIALECT'S
//
// 152 items for a 153-row chart, on BOTH sides, reached independently. 087
// RADIO ID prints a single hyphen for its Digits and ten spaced hyphens for its
// parameter, so it names no field an EX frame could read or write. The dialect
// omits it under internal/extable's ParameterlessExcluded, keyed on
// transcription A's raw '-'; this fake omits it under gen/main.go's
// parameterlessAddrs, keyed on transcription B's '?'. ONE GLYPH IS PRINTED ON
// THE PAGE and the two transcriptions spell it under different conventions —
// this milestone's plan decision P18, stated in full at gen/main.go's
// parameterlessToken. An EX read of 087 therefore draws "?;", the same answer
// as an address the chart never carried, which is doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;" and not a separate behaviour.
//
// # WHAT THE TABLE MODELS
//
// Wire behaviour only: which addresses answer, and each one's raw P4 reply
// WIDTH. It records nothing about what a menu item MEANS — no name, no enum
// legend, no valid range. The names live in the dialect's inventory, which is
// the layer that has a reason to know them; this fake answers reads.
//
// # EX GRAMMAR (manual rev 1711-D, availability line 155, frames 519-528)
//
// Read frame (6 bytes): "EX" + P1(3) + ";" — a THREE-digit wire address, the
// NARROWEST in the family, where the FT-891's is four and every other
// registered sibling's is six. The chart's MENU Number is the whole address:
// 087 is P1=87, with P2 and P3 zero (core/cat's EXAddressSingle).
// Answer frame: "EX" + address(3) + P4(n) + ";", where n is the address's own
// width: 1-8 raw ASCII digits. There is no text item on this chart, and its
// transcription carries no column that could describe one (gen/main.go's
// widthToken).
// Set frame: same shape with a P4 payload — NOT modelled; see handleEX.

// exItem is one generated inventory entry: a three-digit wire address and the
// width token transcription B's digits column prints for it. The type is
// declared HERE and the values in exinventory_gen.go, so that the generated
// file is data alone.
type exItem struct {
	// addr is the three-digit wire address, as the frame carries it.
	addr string
	// width is the width token: '1'..'8', a numeric field of that many raw
	// ASCII bytes. There is no text token — gen/main.go's widthToken says what
	// that does and does not claim.
	width byte
}

// exDefaultDigit is the digit byte a numeric EX item's raw P4 defaults to.
//
// INVENTED — doc.go's register entry THE EX MENU VALUES ARE INVENTED. The
// FT-991A chart documents each item's VALID RANGE and its option legends and
// never a shipped default, so there is nothing to source a real one from; and
// `rigprog read --settings --fake --model FT-991A` renders these bytes to a
// user, who must not read them as what an FT-991A ships with. It is
// fakeradio's convention, adopted because a placeholder that is obviously
// uniform is harder to mistake for evidence than a plausible-looking spread of
// values.
const exDefaultDigit = '0'

// exMaxWidth is the widest raw P4 field this chart declares, and the top of the
// width alphabet expandEXItems accepts.
//
// EIGHT, where the FT-891's numeric alphabet stops at five and the FTdx10's at
// four. It comes from exactly ONE row — 151 PRESET FREQUENCY, whose
// "00030000 ~ 47000000" parameter is eight digits wide — and both sides of the
// evidence pin that address independently: gen/main_test.go's
// TestParseB_TheOnlyEightWideRowIs151 from B, and
// core/cat/ft991a/crosscheck_test.go's widestRowAddr/widestRowDigits from A.
//
// THERE IS NO 'T' TOKEN AND NO exTextWidth. The FTdx10's inventory has a
// twelve-byte text item (MY CALL.) that answers spaces rather than zeros; this
// chart has no such row, and — the sharper point — its transcription carries no
// column from which one could be identified, so the generator refuses a width
// it cannot classify rather than inventing a token.
const exMaxWidth = 8

// expandEXItems expands a generated inventory into the full address -> default
// raw P4 map: a numeric width n defaults to n x exDefaultDigit.
//
// It PANICS on a malformed width token, mirroring defaultState's panics for
// factory-image constants (image.go). exItems is a generated package-level
// table, so a token outside '1'..'8' is a defect in the generator or a
// hand-edit of its output — a programming error to catch at init, never a
// runtime input. gen/main.go's widthToken REFUSES to emit one (with the
// offending CSV line named, which is why the '8' is proved there rather than
// discovered here), and gen/main_test.go's staleness check refuses a generated
// file that has drifted from the CSV, so reaching this panic means one of those
// two was bypassed.
//
// It takes its table as a PARAMETER where internal/fakeft891's equivalent reads
// the package variable directly, for one reason: a panic that cannot be reached
// from a test is a claim rather than a guard, and mutating the generated table
// to reach it would be worse than not testing it
// (TestBuildEXDefaults_PanicsOnAMalformedWidthToken).
func expandEXItems(items []exItem) map[string]string {
	out := make(map[string]string, len(items))
	for _, it := range items {
		if it.width < '1' || it.width > '0'+exMaxWidth {
			panic("fakeft991a: exItems " + it.addr + ": malformed width token " + string(it.width) + " — regenerate with `go generate ./internal/fakeft991a`")
		}
		out[it.addr] = strings.Repeat(string(rune(exDefaultDigit)), int(it.width-'0'))
	}
	return out
}

// exDefaults is the package-level default menu state, computed once from
// exItems at package init.
var exDefaults = expandEXItems(exItems)

// EXDefaults returns a fresh copy of the fake's default menu state: three-digit
// wire address -> default raw P4 (numeric width n -> n x '0').
//
// THERE IS NO EXRuntimeDefaults HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with the M8c hardware observations overlaid — because it HAS hardware
// observations, and folding them into its transcription would have destroyed
// the very cross-check that transcription exists for. No FT-991A has ever been
// asked anything by this project, so there is nothing to overlay: what this
// function returns IS what a *Radio answers, and a second, empty table would be
// ceremony. When FT-991A evidence does arrive, fakeradio's split is the pattern
// to copy — a separate overrides table with its own citation, never an edit to
// exItems or to the CSV it is generated from.
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

// exAddrLen is the wire length of an EX read body: P1(3). THREE, and the whole
// address — this radio's read frame is six bytes where the FT-891's is seven
// and every other registered sibling's is nine (ft991a_layout.txt:519-528).
const exAddrLen = 3

// isEXAddr reports whether s is a syntactically valid three-digit EX wire
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
// frame: "EX" + addr(3) + p4(n) + ";".
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
// READ ONLY. Any body that is not exactly three ASCII digits naming a KNOWN
// address draws "?;" with the state unchanged — which covers malformed bodies
// (wrong length, non-digit bytes), addresses the chart never enumerated, the
// one address it enumerated but that names no field (087, plan P18), and
// SET-SHAPED bodies alike (a valid address immediately followed by a P4
// payload, e.g. "EX0010001;", is simply a too-long body to this handler). The
// refusal of an unknown address is doc.go's register entry AN OUT-OF-INVENTORY
// EX ADDRESS ANSWERS "?;", ASSUMED here as everywhere on this radio; the Set
// rejection is a deliberate modelling gap rather than a claim that a real
// FT-991A refuses EX Set.
//
// NOTE WHAT THE THREE-DIGIT CHECK ALSO EXCLUDES: the FT-891's four-digit read
// frame and every other sibling's six-digit one. A length check written as "at
// least three digits" would answer one of those with an FT-991A's menu value,
// which is the wrong answer given confidently — the class of mistake a shared
// "fake core" would have made structural (doc.go, "A SIBLING of
// internal/fakeft891, not a refactor of it").
//
// Membership comes from the chart's own rows, via the generated inventory. The
// EX block's printed bound — "P1 : 001 - 153 (MENU Number)" — is not enforced
// as a range: it happens to be exactly the first and last rows transcribed, so
// enforcing it would add a second, redundant authority over the same fact, and
// it would ADMIT 087, which the inventory rightly does not.
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
