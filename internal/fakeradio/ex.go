// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"fmt"
	"strings"
)

// This file is fakeradio's own, independent model of the FT-710's EX
// (MENU) command — READ ONLY this phase (see doc.go register: set-shaped
// bodies are a deliberate phase-scoped gap). It is derived directly from
// the FT-710 CAT protocol reference's "EX" grammar and "Table 2 (MENU
// Chart)" — NOT from core/cat — for the same independence reason as
// parser.go (see doc.go, THE HARD RULE).
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
// Set frame: "EX" + address(6) + P4(n) + ";" — NOT modelled this phase;
// see handleEX and doc.go register for the deliberate gap.
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
// before the trailing ';'). READ ONLY this phase (brief, "Behaviour" —
// see doc.go register): any body that is not exactly 6 ASCII digits
// naming a KNOWN address is rejected with "?;", state unchanged. This
// includes both malformed bodies (wrong length, non-digit bytes) and
// set-shaped bodies (a valid 6-digit address immediately followed by a
// P4 payload, e.g. "EX0301051;") — the fake does not distinguish a
// too-long body from an intentional Set; both fall through the same
// length check to the one generic NAK. This is a deliberate
// phase-scoped modelling gap, not a claim that real hardware rejects
// EX-set (doc.go register: KNOWN-DIVERGENT from the documented Set
// grammar).
func (r *Radio) handleEX(body []byte) []byte {
	addr := string(body)
	if !isEXAddr(addr) {
		return rejection
	}
	r.mu.Lock()
	p4, ok := r.exSettings[addr]
	r.mu.Unlock()
	if !ok {
		// Valid-shape but out-of-inventory address (e.g. no P1=05 group,
		// or a P3 beyond a group's item count) — "?;", the same generic
		// NAK as MR's out-of-inventory slots (parser.go, handleMR).
		// OBSERVED at M8c against a real radio for six such addresses,
		// including both probed P1=05 ones (doc.go register item 23).
		return rejection
	}
	return buildEXAnswer(addr, p4)
}
