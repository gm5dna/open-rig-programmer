// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"fmt"
	"strings"
)

// This file is fakets480's own, independent model of the TS-480's EX (MENU)
// command — READ ONLY, exactly as internal/fakeft891 models the FT-891's: the
// book documents a Set form and this fake does not implement it (doc.go's
// "What this fake deliberately does NOT model").
//
// # WHERE THE INVENTORY COMES FROM, AND WHY IT IS GENERATED
//
// The widths table this file expands (exWidths480) is NOT hand-typed here. It
// is GENERATED, by the directive above, from this package's OWN COPY of
// TRANSCRIPTION B — transcription-b-480.csv beside this file, with
// PROVENANCE.md recording where the copy came from and why it is a copy rather
// than a move.
//
// That is the whole mechanism of this radio's two-source cross-check, and it
// is worth stating in full because it is the reason this package does not
// simply read the codec's inventory:
//
//   - the CODEC's inventory (core/kw/ts480/exinventory_gen.go) is generated
//     from TRANSCRIPTION A (core/kw/ts480/menu480.csv) by internal/extable;
//   - THIS inventory is generated from TRANSCRIPTION B by
//     exinventory.go, which imports nothing project-internal at all
//     (the recursive fence in imports_test.go enforces it for this directory
//     and every one beneath it, and
//     TestNoCoreImports_ReachesTheGenerator proves the scan really gets
//     there);
//   - core/transport/ex_crosscheck_ts480_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined
// agent with no repository access and no sight of A, the page ledger or any
// row count (core/kw/ts480/crosscheck_test.go records all three artefacts and
// hashes them). So a mis-read row in either transcription, or a defect in
// either generator, surfaces as a cross-check MISMATCH rather than as two
// tables quietly agreeing on the same wrong number. Deriving this table from A
// — or from the codec, or with extable's parser — would collapse both sides
// onto one source and throw that property away.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF.
//
// # ONE PRINTED DEFECT RIDES THROUGH IT UNCHANGED, and it is recorded rather
// than repaired
//
// The EX block's prose lists the two-digit menus as "Menu No. 32, 35 and 48 ~
// 52" (480:411) and OMITS menu 034, whose grid row nonetheless reaches the
// chart's second parameter column. Both quarantined derivations read the GRID
// and record 034 as two digits; core/kw/ts480's errata and its
// crosscheck_test.go pin that as a defect of the printed block. This fake
// answers TWO bytes there — because that is what the transcription says, and
// no TS-480 has been asked which the radio answers. Three faithful readings of
// one chart agree perfectly, so no comparison in this repository can catch
// this class; the pin is what makes it a deliberate state.
//
// # WHAT THE TABLE MODELS
//
// Wire behaviour only: which menu numbers the chart has, and each one's raw P5
// reply WIDTH. It records nothing about what a menu MEANS — no name, no
// legend, no valid range. The names live in the codec's inventory, which is
// the layer with a reason to know them; this fake answers reads.
//
// # EX GRAMMAR (480:399-416)
//
// Read frame (10 bytes): "E X P1 P1 P1 P2 P2 P3 P4 ;" — a THREE-digit Menu
// No. in P1, "000 ~ 060" (480:401), then the three printed constants P2 "00",
// P3 '0' and P4 '0', each "Always … for the TS-480" (480:402-407).
// Answer frame: the same ten positions with P5 — "A string of characters
// (Variable length)" (480:409) — inserted before the terminator. This book
// prints two literal answers: "EX00000000; (Display illumination OFF)" and
// "EX00000003; (Display brightness level 3)" (480:415-416).
// Set frame: the same shape as the answer — NOT modelled; see handleEX.

// exWire renders a menu number as the three ASCII digits the frame carries.
func exWire(menu int) string { return fmt.Sprintf("%03d", menu) }

// exDefaultDigit is the digit byte a menu's raw P5 defaults to.
//
// It is INVENTED as a table-wide convention — doc.go's register entry THE EX
// MENU VALUES ARE INVENTED — but it is not arbitrary at menu 000, and that is
// worth recording: this book prints "EX00000000; (Display illumination OFF)"
// as a worked ANSWER (480:415), whose P5 is exactly this byte at exactly that
// address. Every other address takes the same placeholder because the chart
// prints each menu's available SETTINGS and never a shipped default, so there
// is nothing to source a real one from — and `rigprog read --settings --fake`
// renders these bytes to a user, who must not read them as what a TS-480 ships
// with. A placeholder that is obviously uniform is harder to mistake for
// evidence than a plausible-looking spread of values.
const exDefaultDigit = '0'

// exMaxWidth is the widest raw P5 this chart declares, and the top of the
// width alphabet buildEXDefaults accepts.
//
// TWO, where the FT-891's alphabet reaches five and the 590 pair's eight. The
// EX block prints the rule in prose — "A string of characters (Variable
// length) Normally 1-digit for the TS-480. Menu No. 32, 35 and 48 ~ 52 use
// 2-digit parameters." (480:409-411) — and the ceiling it names is two.
// exinventory.go's maxWidth carries the same number on the generator's side, and
// exinventory_test.go pins the set of two-wide rows from the committed CSV.
const exMaxWidth = 2

// buildEXDefaults expands the generated exWidths480 into the full address ->
// default raw P5 map: a width token n defaults to n x exDefaultDigit.
//
// It PANICS on a malformed width token, mirroring defaultState's panics for
// factory-image constants (image.go). exWidths480 is a generated package-level
// table, so a token outside '1'..'2' is a defect in the generator or a
// hand-edit of its output — a programming error to catch at init, never a
// runtime input. exinventory.go's widthToken REFUSES to emit one (with the
// offending CSV line named, which is why the alphabet's top is proved there
// rather than discovered here), and exinventory_test.go's staleness check refuses
// a generated file that has drifted from the CSV, so reaching this panic means
// one of those two was bypassed.
func buildEXDefaults() map[string]string {
	out := make(map[string]string, len(exWidths480))
	// Indexed by BYTE, not by rune: the table is one ASCII digit per menu
	// (exinventory.go's widthToken), so a byte index is the intended menu number
	// and `range` over the string would give a rune index instead — the same
	// value here because the alphabet is ASCII, but not what the loop means to
	// compute.
	for menu := 0; menu < len(exWidths480); menu++ {
		w := exWidths480[menu]
		if w < '1' || w > '0'+exMaxWidth {
			panic(fmt.Sprintf("fakets480: menu %03d: malformed width token %q — a defect in this package's projection of transcription B (exinventory.go)", menu, w))
		}
		out[exWire(menu)] = strings.Repeat(string(exDefaultDigit), int(w-'0'))
	}
	return out
}

// exDefaults is the package-level default menu state, computed once at init.
var exDefaults = buildEXDefaults()

// EXDefaults returns a fresh copy of the fake's default menu state:
// three-digit wire address -> default raw P5 (width n -> n x '0').
//
// THERE IS NO RUNTIME-OVERRIDE TABLE HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with hardware observations overlaid — because it HAS observations, and
// folding them into its transcription would have destroyed the very
// cross-check that transcription exists for. No TS-480 has ever been asked
// anything by this project, so there is nothing to overlay: what this function
// returns IS what a *Radio answers. When TS-480 evidence does arrive,
// fakeradio's split is the pattern to copy — a separate overrides table with
// its own citation, never an edit to exWidths480 or to the CSV it is generated
// from.
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

// exBodyLen is the length of an EX READ body — the frame bytes after "EX" and
// before the ';': P1(3) + P2(2) + P3(1) + P4(1). SEVEN, making the whole read
// frame the ten positions the book numbers (480:408-410).
const exBodyLen = 7

// The three printed constants of the EX frame, named once because the answer
// builder emits them and the read validator requires them: P2 "00: Always 00
// for the TS-480", P3 and P4 "0: Always 0 for the TS-480" (480:402-407).
const (
	exP2Printed = "00"
	exP3Printed = '0'
	exP4Printed = '0'
)

// isEXReadBody reports whether body is a syntactically valid EX read body: a
// three-digit Menu No. followed by the three printed constants. It says
// nothing about whether that menu number names a row this chart has.
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
// inserted before the terminator (480:413-416).
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
// number this chart has draws "?;" with the state unchanged — which covers
// malformed bodies, a P2/P3/P4 that is not the printed constant, menu numbers
// outside 000 ~ 060, and SET-SHAPED bodies alike (the read's seven bytes
// followed by a P5 payload is simply a too-long body to this handler). The
// out-of-inventory refusal is doc.go's register entry AN OUT-OF-INVENTORY EX
// ADDRESS ANSWERS "?;", ASSUMED here as everywhere on this radio; the Set
// rejection is a deliberate modelling gap rather than a claim that a real
// TS-480 refuses EX Set.
//
// MEMBERSHIP COMES FROM THE CHART'S OWN ROWS, via the generated inventory —
// not from the printed range. The EX block prints a domain, "000 ~ 060: Menu
// No." (480:401), and it happens to be exactly the first and last rows
// transcribed; enforcing it separately would add a second, redundant authority
// over the same fact and would disagree with the inventory the moment the two
// were ever edited apart.
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
