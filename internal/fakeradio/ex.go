// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"fmt"
	"strings"
)

// This file is fakeradio's own, independent model of the FT-710's EX
// (MENU) command — reads, and, from task (e), Sets at whatever width
// exSetWidths characterises for an address (see doc.go register item 24
// for the phase history: read-only through task (d), Set added here). It
// is derived directly from the FT-710 CAT protocol reference's "EX"
// grammar and "Table 2 (MENU Chart)" — NOT from core/cat — for the same
// independence reason as parser.go (see doc.go, THE HARD RULE).
//
// The compact inventory below (exGroups) models WIRE BEHAVIOUR ONLY: how
// many P3 items each (P1,P2) menu subgroup has, and each item's raw P4
// reply WIDTH. It deliberately does NOT record what any menu item MEANS
// (its human name, its enum values, its semantic range) — a full,
// semantic CAT inventory is a separate, independently-derived milestone
// deliverable built from the same manual pages, by design (see the task
// brief's HARD INDEPENDENCE RULE): this table exists so the two
// transcriptions can be cross-checked against each other later, neither
// having consulted the other's output.

// --- EX grammar (reference, "EX" section) ---
//
// Read frame (9 bytes): "EX" + P1(2) + P2(2) + P3(2) + ";" — a 6-digit
// wire address.
// Answer frame: "EX" + address(6) + P4(n) + ";" — P4's width n is
// per-address: 1-4 raw ASCII digits (a sign counts in the width, e.g.
// "-20".."+10" is width 3), or a fixed 12-byte text field.
// Set frame: "EX" + address(6) + P4(n) + ";" — from task (e), accepted at
// exSetWidths[address]'s width for any inventory address; see handleEX
// and doc.go register item 24.
//
// The grammar's own P1/P2/P3 ranges ("P1: 01-04, 05"; "P2: 01-05"; "P3:
// 01-26") are the OUTER bounds seen across every group, not a per-group
// range: P2's widest span is 01-05 (P1=01's five per-mode subgroups); P3's
// widest span is 01-26 (P1=03/P2=01's 26 items). Table 2 itself has no
// P1=05 group at all — its fifth P1 value is 06 (EXTENSION SETTING) — see
// doc.go register for that anomaly.

// exAddr builds a 6-digit wire address from 2-digit P1/P2/P3 strings.
func exAddr(p1, p2, p3 string) string { return p1 + p2 + p3 }

// modeAudioPrefix is the widths token sequence for P3 items 1-10, shared
// byte-for-byte across every per-mode subgroup of P1=01 (RADIO SETTING)
// and P1=02/P2=01 (CW SETTING's own per-mode-shaped subgroup): three
// width-3 items, three width-4 items, then two width-2/width-1 pairs
// (Table 2 — every such group's first 10 rows are identical in shape).
const modeAudioPrefix = "3334442121"

// presetWidths is the 18-item widths sequence Table 2 prints ONCE for
// P1=06 (EXTENSION SETTING, manual lines ~895-915) but which applies
// identically to each of the five P2 subgroups 01-05 (PRESET1..PRESET5):
// the Function/P4/Digits columns are not re-printed per preset, only the
// P2 label changes down the page.
const presetWidths = "T11144421213311331"

// exGroups: one entry per (P1,P2) subgroup — 21 total across the five P1
// menus (01, 02, 03, 04, 06; there is no P1=05 in Table 2 — see doc.go
// register). widths holds one token per P3 item, in P3 order starting at
// 01: '1'..'4' = numeric digit width (a sign counts in the width:
// "-20".."+10" is width 3), 'T' = 12-byte text item. Hand-derived
// independently from Table 2 (manual lines ~645-915, PDF pages 10-13);
// see the task-29 report for the per-group manual line range and item
// count this milestone's independence cross-check relies on.
var exGroups = []struct{ p1, p2, widths string }{
	{"01", "01", modeAudioPrefix + "331133121"},   // 19 items, lines ~646-667
	{"01", "02", modeAudioPrefix + "3311331"},     // 17 items, lines ~668-686
	{"01", "03", modeAudioPrefix + "33133114412"}, // 21 items, lines ~687-711
	{"01", "04", modeAudioPrefix + "3311331214"},  // 20 items, lines ~712-733
	{"01", "05", modeAudioPrefix + "3312111"},     // 17 items, lines ~741-760
	{"02", "01", modeAudioPrefix + "3312111111"},  // 20 items, lines ~761-783
	{"02", "02", "11214111112"},                   // 11 items, lines ~784-796
	{"03", "01", "31111111111111211322222221"},    // 26 items, lines ~797-827
	{"03", "02", "111132"},                        // 6 items, lines ~836-841
	{"03", "03", "1232232232232232232"},           // 19 items, lines ~842-864
	{"03", "04", "33331111"},                      // 8 items, lines ~865-872
	{"03", "05", "111111"},                        // 6 items, lines ~873-878
	{"04", "01", "T11122"},                        // 6 items, lines ~879-884
	{"04", "02", "1111"},                          // 4 items, lines ~885-888
	{"04", "03", "1111"},                          // 4 items, lines ~889-892
	{"04", "04", "11"},                            // 2 items, lines ~893-894
	{"06", "01", presetWidths},                    // 18 items, lines ~895-915
	{"06", "02", presetWidths},                    // 18 items, lines ~895-915 (shared print, see presetWidths)
	{"06", "03", presetWidths},                    // 18 items, lines ~895-915 (shared print, see presetWidths)
	{"06", "04", presetWidths},                    // 18 items, lines ~895-915 (shared print, see presetWidths)
	{"06", "05", presetWidths},                    // 18 items, lines ~895-915 (shared print, see presetWidths)
}

// exDefaultDigit is the digit byte a fresh numeric EX item's raw P4
// defaults to. ASSUMED (see doc.go register): the fake's own placeholder,
// not a claimed factory value.
const exDefaultDigit = '0'

// exTextWidth is the fixed wire width of a 'T' (text) EX item's raw P4
// field (reference, Table 2's "Up to 12 characters" entries: MY CALL,
// PRESET NAME x5).
const exTextWidth = 12

// buildEXDefaults expands exGroups into the full address -> default raw
// P4 map: a numeric width n defaults to n x '0'; a text item defaults to
// exTextWidth spaces. Panics on a malformed width token (mirroring
// defaultState's panics for factory-image constants, image.go) —
// exGroups is a package-level constant table, so a malformed token is a
// programming error caught at init, never a runtime input.
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
				panic(fmt.Sprintf("fakeradio: exGroups P1=%s P2=%s item %d: malformed width token %q", g.p1, g.p2, i+1, w))
			}
		}
	}
	return out
}

// exDefaults is the package-level default menu state, computed once from
// exGroups at package init.
var exDefaults = buildEXDefaults()

// EXDefaults returns a fresh copy of the fake's MANUAL default menu state:
// 6-digit wire address -> default raw P4 (numeric width n -> n x '0'; the
// text items -> 12 spaces), exactly as this package's own transcription of
// Table 2 describes them.
//
// This is the manual-transcription view, NOT what the fake answers on the
// wire: core/transport/ex_crosscheck_test.go compares it against core/cat's
// independent transcription of the same table, which is only meaningful
// while both sides stay manual-derived. For what a *Radio actually answers
// — the manual table with the M8c hardware observations overlaid — use
// EXRuntimeDefaults.
//
// Test-inspection API: every call returns an independent map, so mutating
// the result of one call can never affect another call's result, nor any
// *Radio's own stored exSettings.
func EXDefaults() map[string]string {
	out := make(map[string]string, len(exDefaults))
	for k, v := range exDefaults {
		out[k] = v
	}
	return out
}

// exHardwareOverrides adjusts the default state a *Radio starts from so its
// RUNTIME answers match what a real FT-710 answered during the M8c
// read-characterisation: two successive full EX sweeps of one UK radio, CAT
// ID 0800, firmware V01-12, in one configuration, on 24/07/2026
// (docs/hardware-notes.md).
//
// It is deliberately SEPARATE from exGroups. Those widths are this
// package's own independent transcription of Table 2, and
// core/transport/ex_crosscheck_test.go compares them against core/cat's
// independent transcription of the same table — a cross-check that only
// has value while neither side has been "corrected" from the other or from
// hardware. Editing exGroups to match the radio would silently destroy it.
// So hardware evidence lives here, with its own citation, and changes only
// what the fake ANSWERS — never what it claims the manual says.
//
// The VALUES below are this fake's own synthetic placeholders (register
// item 21: no real radio's settings are ever claimed); only their WIDTH and
// SHAPE are hardware-derived, and each placeholder is deliberately chosen
// NOT to be a value the M8c capture actually contained, so this table
// cannot become a back door for committing one. Nothing here describes EX
// Set behaviour, which M8c did not probe (register item 24).
var exHardwareOverrides = map[string]string{
	// WIDTH: Table 2 prints Digits 2 for TONE FREQ; the radio answered a
	// 3-byte P4 ("EX010321012;" — the one width finding the operator
	// consented to publish).
	"010321": "000",
	// SHAPE: these 26 addresses answered with an explicit leading sign
	// counted inside the manual's own 3-byte width. The magnitude here is
	// this fake's own, not the radio's.
	"010101": "+01", "010102": "+01", "010103": "+01",
	"010201": "+01", "010202": "+01", "010203": "+01",
	"010301": "+01", "010302": "+01", "010303": "+01",
	"010401": "+01", "010402": "+01", "010403": "+01",
	"010501": "+01", "010502": "+01", "010503": "+01",
	"020101": "+01", "020102": "+01", "020103": "+01",
	"030118": "+01", "030205": "+01", "030303": "+01",
	"030306": "+01", "030309": "+01", "030312": "+01",
	"030315": "+01", "030318": "+01",
}

// exRuntimeDefaults is the package-level runtime menu state: the manual
// defaults with exHardwareOverrides applied, computed once at package init.
// A malformed override (an address the manual table does not have) is a
// programming error caught at init, exactly as buildEXDefaults treats a
// malformed width token.
var exRuntimeDefaults = buildEXRuntimeDefaults()

// buildEXRuntimeDefaults overlays exHardwareOverrides onto the manual
// defaults.
func buildEXRuntimeDefaults() map[string]string {
	out := buildEXDefaults()
	for addr, p4 := range exHardwareOverrides {
		if _, ok := out[addr]; !ok {
			panic(fmt.Sprintf("fakeradio: exHardwareOverrides names address %s, which is not in the manual inventory", addr))
		}
		out[addr] = p4
	}
	return out
}

// EXRuntimeDefaults returns a fresh copy of the default menu state a
// *Radio actually starts from and answers with: EXDefaults with the M8c
// hardware observations overlaid (see exHardwareOverrides). Use this for
// anything comparing the fake against hardware evidence; use EXDefaults for
// the manual-transcription cross-check. Every call returns an independent
// map, exactly as EXDefaults does.
func EXRuntimeDefaults() map[string]string {
	out := make(map[string]string, len(exRuntimeDefaults))
	for k, v := range exRuntimeDefaults {
		out[k] = v
	}
	return out
}

// exSetWidths is fakeradio's own transcription of the EX Set frame's P4
// width, per address, from Session W's hardware write-characterisation —
// deliberately NOT a read of core/cat's ft710ExWrite (exwrite_gen.go),
// for the identical independence reason exHardwareOverrides gives: the
// two transcriptions must stay derived from the same evidence without
// ever consulting each other, or core/transport/ex_crosscheck_test.go's
// value collapses.
//
// It is a SEPARATE table from exHardwareOverrides on purpose: that one
// carries READ-direction observations (Table 2 default state), this one
// SET-direction ones (Session W, spec §3), and
// conflating them would let a read-width fact silently stand in for a
// write-width one nobody has actually observed.
//
// Filled from Session W (18/10/2026, UK FT-710, CAT ID 0800): the 238
// admitted addresses, each at the width the radio accepted a Set at (010321
// TONE FREQ is 3, not the manual's 2). Any other address is uncharacterised
// and handleEX refuses a Set-shaped body for it (see its own doc comment).
// Tests overlay entries with WithEXSetWidth.
var exSetWidths = map[string]int{
	"010101": 3,
	"010102": 3,
	"010103": 3,
	"010104": 4,
	"010105": 4,
	"010106": 4,
	"010107": 2,
	"010108": 1,
	"010109": 2,
	"010110": 1,
	"010111": 3,
	"010112": 3,
	"010113": 1,
	"010115": 3,
	"010116": 3,
	"010118": 2,
	"010119": 1,
	"010201": 3,
	"010202": 3,
	"010203": 3,
	"010204": 4,
	"010205": 4,
	"010206": 4,
	"010207": 2,
	"010208": 1,
	"010209": 2,
	"010210": 1,
	"010211": 3,
	"010212": 3,
	"010213": 1,
	"010215": 3,
	"010216": 3,
	"010301": 3,
	"010302": 3,
	"010303": 3,
	"010304": 4,
	"010305": 4,
	"010306": 4,
	"010307": 2,
	"010308": 1,
	"010309": 2,
	"010310": 1,
	"010311": 3,
	"010312": 3,
	"010314": 3,
	"010315": 3,
	"010317": 1,
	"010318": 4,
	"010319": 4,
	"010320": 1,
	"010321": 3,
	"010401": 3,
	"010402": 3,
	"010403": 3,
	"010404": 4,
	"010405": 4,
	"010406": 4,
	"010407": 2,
	"010408": 1,
	"010409": 2,
	"010410": 1,
	"010411": 3,
	"010412": 3,
	"010413": 1,
	"010415": 3,
	"010416": 3,
	"010418": 2,
	"010419": 1,
	"010420": 4,
	"010501": 3,
	"010502": 3,
	"010503": 3,
	"010504": 4,
	"010505": 4,
	"010506": 4,
	"010507": 2,
	"010508": 1,
	"010509": 2,
	"010510": 1,
	"010511": 3,
	"010512": 3,
	"010514": 2,
	"010515": 1,
	"010516": 1,
	"010517": 1,
	"020101": 3,
	"020102": 3,
	"020103": 3,
	"020104": 4,
	"020105": 4,
	"020106": 4,
	"020107": 2,
	"020108": 1,
	"020109": 2,
	"020110": 1,
	"020111": 3,
	"020112": 3,
	"020114": 2,
	"020116": 1,
	"020117": 1,
	"020118": 1,
	"020119": 1,
	"020120": 1,
	"020201": 1,
	"020202": 1,
	"020203": 2,
	"020204": 1,
	"020205": 4,
	"020206": 1,
	"020207": 1,
	"020208": 1,
	"020209": 1,
	"020210": 1,
	"020211": 2,
	"030101": 3,
	"030102": 1,
	"030112": 1,
	"030113": 1,
	"030114": 1,
	"030116": 1,
	"030117": 1,
	"030118": 3,
	"030119": 2,
	"030120": 2,
	"030121": 2,
	"030122": 2,
	"030123": 2,
	"030124": 2,
	"030125": 2,
	"030126": 1,
	"030201": 1,
	"030202": 1,
	"030203": 1,
	"030204": 1,
	"030205": 3,
	"030206": 2,
	"030301": 1,
	"030302": 2,
	"030303": 3,
	"030304": 2,
	"030305": 2,
	"030306": 3,
	"030307": 2,
	"030308": 2,
	"030309": 3,
	"030310": 2,
	"030311": 2,
	"030312": 3,
	"030313": 2,
	"030314": 2,
	"030315": 3,
	"030316": 2,
	"030317": 2,
	"030318": 3,
	"030319": 2,
	"030405": 1,
	"030408": 1,
	"030501": 1,
	"030502": 1,
	"030503": 1,
	"030504": 1,
	"030505": 1,
	"030506": 1,
	"040102": 1,
	"040103": 1,
	"040104": 1,
	"040105": 2,
	"040106": 2,
	"040201": 1,
	"040202": 1,
	"040203": 1,
	"040204": 1,
	"040301": 1,
	"040302": 1,
	"040303": 1,
	"040304": 1,
	"040401": 1,
	"040402": 1,
	"060105": 4,
	"060106": 4,
	"060107": 4,
	"060108": 2,
	"060109": 1,
	"060110": 2,
	"060111": 1,
	"060112": 3,
	"060113": 3,
	"060114": 1,
	"060116": 3,
	"060117": 3,
	"060205": 4,
	"060206": 4,
	"060207": 4,
	"060208": 2,
	"060209": 1,
	"060210": 2,
	"060211": 1,
	"060212": 3,
	"060213": 3,
	"060214": 1,
	"060216": 3,
	"060217": 3,
	"060305": 4,
	"060306": 4,
	"060307": 4,
	"060308": 2,
	"060309": 1,
	"060310": 2,
	"060311": 1,
	"060312": 3,
	"060313": 3,
	"060314": 1,
	"060316": 3,
	"060317": 3,
	"060405": 4,
	"060406": 4,
	"060407": 4,
	"060408": 2,
	"060409": 1,
	"060410": 2,
	"060411": 1,
	"060412": 3,
	"060413": 3,
	"060414": 1,
	"060416": 3,
	"060417": 3,
	"060505": 4,
	"060506": 4,
	"060507": 4,
	"060508": 2,
	"060509": 1,
	"060510": 2,
	"060511": 1,
	"060512": 3,
	"060513": 3,
	"060514": 1,
	"060516": 3,
	"060517": 3,
}

// cloneEXSetWidths returns a fresh copy of exSetWidths for a new *Radio to
// start from — the same independent-copy-per-instance discipline
// EXRuntimeDefaults gives exSettings, so a WithEXSetWidth option on one
// Radio can never leak into another's or into the package-level table.
func cloneEXSetWidths() map[string]int {
	out := make(map[string]int, len(exSetWidths))
	for k, v := range exSetWidths {
		out[k] = v
	}
	return out
}

// --- EX command handler ---

// exAddrLen is the wire length of an EX read body: P1(2) + P2(2) + P3(2).
const exAddrLen = 6

// isEXAddr reports whether s is a syntactically valid 6-digit EX wire
// address (says nothing about whether it names a known menu item).
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

// handleEX validates and answers an EX body (frame bytes after "EX",
// before the trailing ';'), for both the Read and Set forms — a
// set-shaped body is accepted from task (e) onward (doc.go register item
// 24: read-only through task (d)).
//
// A body shorter than 6 bytes, or whose first 6 bytes are not ASCII
// digits, is malformed and refused with "?;", state unchanged — this
// covers every too-short body and every non-digit address field,
// regardless of what (if anything) follows.
//
// Exactly 6 digits is a Read: a KNOWN address (present in r.exSettings)
// answers with its stored raw P4; an unknown one — valid-shape but
// out-of-inventory (e.g. no P1=05 group, or a P3 beyond a group's item
// count) — is refused "?;", the same generic NAK as MR's out-of-inventory
// slots (parser.go, handleMR). OBSERVED at M8c against a real radio for
// six such addresses, including both probed P1=05 ones (doc.go register
// item 23).
//
// More than 6 digits is a Set: "EX" + addr(6) + P4(n) + ";". Accepted at
// the width exSetWidths[addr] records, for ANY inventory address —
// admitted or denied or held under core/cat's project policy makes no
// difference here, because this package never consults that policy (doc.go,
// THE HARD RULE: fakeradio does not import core/cat) — the fake models
// wire behaviour, not who is allowed to ask for it. Refused "?;" for an
// out-of-inventory address (the same membership check the Read form
// uses), or for an inventory address whose exSetWidths entry is absent
// (uncharacterised — exSetWidths is empty until Session W runs) or whose
// payload length does not equal the characterised width. An accepted Set
// stores the payload verbatim, with no shape/domain validation of its
// own — that is core/cat's write-descriptor Domain's job, never this
// package's (exSetP4OK, core/cat/ex.go) — and answers with no reply at
// all, fire-and-forget, mirroring handleMW's success case.
func (r *Radio) handleEX(body []byte) []byte {
	if len(body) < exAddrLen || !isEXAddr(string(body[:exAddrLen])) {
		return rejection
	}
	addr := string(body[:exAddrLen])
	p4Set := body[exAddrLen:]

	r.mu.Lock()
	defer r.mu.Unlock()
	p4, known := r.exSettings[addr]
	if !known {
		return rejection
	}
	if len(p4Set) == 0 {
		return buildEXAnswer(addr, p4)
	}

	width, characterised := r.exSetWidths[addr]
	if !characterised || len(p4Set) != width {
		return rejection
	}
	if !r.exSetStuck[addr] {
		r.exSettings[addr] = string(p4Set)
	}
	return nil
}
