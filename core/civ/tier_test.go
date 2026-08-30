// SPDX-License-Identifier: GPL-3.0-or-later

// Package civ_test's tier file is THE ONE PLACE the Icom PROFILE
// FAMILIES' record shapes are compared with each other, and it is
// deliberately not in any model's own package: no per-model worktree may
// claim a cross-model distinctness property, and every one of the
// drivers' doc comments says so in its own words (core/driver/ic9700/ic9700.go's
// probeFingerprint: "Cross-model record-length distinctness is a
// TIER-level Wave-4 check needing a registry-wide table of accepted
// lengths"; core/driver/ic7300mk2/doc.go's "The wrong-sibling
// fingerprint" section: "Cross-model record-length distinctness is a
// TIER-level check belonging to registration"). This file is that check.
//
// SEVEN FAMILIES AND EIGHT REGISTERED ROWS as of the additions tier's
// first registration (Tier 4b): the Icom tier's six families, plus
// core/civ/ic7851, which serves the IC-7851 and IC-7850 rows ALIKE. A
// family is a PROFILE; a row is a registry key; tierRegistrationCoverage
// below is the map between them, and it is the reason the two counts may
// legitimately differ.
//
// It lives in core/civ as an EXTERNAL test package. core/civ can never
// import core/civ/ic7610 and its siblings — they import core/civ,
// and the import would be a cycle — but a `package civ_test` file
// compiles separately and may, which is the same escape hatch
// core/civ/civtest's own tests use.
//
// # What the probe actually uses the length for
//
// EVERY ONE OF THESE DRIVERS PROBES THE ADDRESS FIRST. Open sends
// `19 00` and requires an ADDRESS-MATCHED reply (the value is
// undocumented on every one of these documents, spec D5 entry 7, so it is
// recorded and never compared), and only then walks a bounded run of
// memory channels for a record whose length confirms the profile. The
// SEVEN FAMILIES' CI-V addresses are already distinct — 98h, 94h, B6h,
// A4h, A2h, ACh, 8Eh — so IN THE FIELD a wrong radio on the port does not
// answer at all and the open times out. THE LENGTH FINGERPRINT IS
// THEREFORE DEFENCE IN DEPTH, NOT THE PRIMARY DISCRIMINATOR: it protects
// against SAME-ADDRESS confusion only — a radio moved onto this address,
// or a bus mis-set — which is exactly what core/driver/ic7300mk2/doc.go
// and core/driver/ic905's Open comment both already say.
//
// WITH ONE EXCEPTION, AND IT IS NOT A MOVED ADDRESS. The IC-7851 and the
// IC-7850 SHARE 8Eh AT FACTORY DEFAULTS (additions spec D1.2; PDF p.229,
// folio 15-18, '"8Eh" is the default address of IC-7850/IC-7851'), share
// one manual and one frame shape, and their `19 00` reply value is
// undocumented for both — so for that pair the field argument above does
// not hold and nothing recovers it. They are ONE FAMILY here, so the
// pairwise walk cannot see them at all; the honest record of the
// limitation is core/driver/ic7851/doc.go §1 and the registry row the
// user picks.
//
// # The limitation this file records honestly
//
// TWO PAIRS SHARE A RECORD-ONLY LENGTH, and only one of them is
// separable. The IC-7610 and the IC-7851 share BOTH properties — 25 bytes
// record-only and a two-byte flat address — and are therefore DECLARED
// indistinguishable in the table below rather than proven apart; see that
// table's own comment. The pair this section describes is the separable
// one.
//
// The IC-705 and the IC-9700
// both accept exactly {111}, so a fingerprint that compared record-only
// lengths ALONE could not tell those two apart. What separates them is
// the ADDRESS WIDTH — four bytes against three — and it does so in TWO
// independent places, of which only the first ever fires.
//
// THE FIRST REFUSAL IS THE ADDRESS, NOT THE LENGTH, and the ORDER inside
// civ.Profile.MemoryAnswerRecord (core/civ/parse.go:96) is what decides
// that: it takes the profile's OWN address width off the front of the
// data area, runs decodeAddress — and so validAddress
// (core/civ/profile.go:316) — at parse.go:105, and only reaches
// AcceptsRecordLength at parse.go:110 if that succeeded. THE TWO ADDRESS
// GEOMETRIES ARE MUTUALLY UNDECODABLE: read four bytes off a
// (band, channel) address and the leading `01`/`02`/`03` band byte lands
// in the wide form's two-byte group field, giving a group of 100, 200 or
// 300 and a channel of 100 or more; read three bytes off a
// (group, channel) address and the wide form's leading `00` becomes a
// band of 0, which the IC-9700 numbers from 1. Neither is in range, in
// either direction, for ANY address either radio has —
// TestTierRecordShapes_705And9700 sweeps all 321 of the IC-9700's and
// all 10,100 of the IC-705's and gets a *civ.ParseError every time, and
// a *civ.RecordLengthError never.
//
// THE LENGTH ARITHMETIC IS THE SECOND REFUSAL, and it is genuinely
// independent — a 114-byte data area stripped by four is a 110-byte
// record and a 115-byte one stripped by three is 112, neither of which
// the other profile accepts — but the address check pre-empts it in
// practice, so nothing observable ever reaches it. The test pins that
// arithmetic separately, as arithmetic, rather than claiming it is what
// fires.
//
// THE CONSEQUENCE IS IN ONE DRIVER, and only one. Of this pair, only
// core/driver/ic705/ic705.go:360-367 branches on errors.As(&lenErr) to
// mint a driver.WrongRadioError, and FOR THIS PAIR THAT BRANCH CAN NEVER
// FIRE: an IC-9700 moved onto A4h fails an IC-705 open with an
// unattributed address parse error, not with "wrong radio". The open
// still fails, which is the safety property, but the diagnosis a user
// sees names no model — a reporting limitation, recorded rather than
// papered over, and NOT a hole in the refusal.
//
// THE IC-9700 MINTS NO driver.WrongRadioError AT ALL, for any pair, BY
// DESIGN — not merely because this pair pre-empts it.
// core/driver/ic9700/ic9700.go's probeFingerprint wraps every error from
// MemoryAnswerRecord generically, its doc.go says so in as many words
// ("No driver.WrongRadioError is ever minted here"), and that package's
// TestUnexpectedLengthIsRefusedWithoutAttribution pins it: naming a model
// from {111} would be the cross-model claim no per-model worktree may
// make, which is the reason this file exists. Nothing here changes that,
// and this file mints nothing of its own.
//
// The three drivers that DO mint one on a length mismatch are the IC-7300
// and IC-7300MK2 (each with a one-entry sibling hint for the other,
// rendered *provisional*) and the IC-905 (whose attribution comes from a
// caller-supplied WithSiblingRecordLengths table — which no registered
// composition passes, so its refusal today populates neither model field
// and renders as the ID-only form).
//
// AND EVERY NUMBER HERE IS AN ASSUMED DERIVATION. No document in this
// tier prints a record total; each per-model package derives its length
// from printed field widths, and no radio has ever answered a frame to
// confirm one (each model's own register entry carries the lift). This
// file compares what those derivations say about one another; it does not
// turn a set of assumptions into a measurement.
package civ_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// tierShape is one model's measured record geometry: the two properties
// a CI-V memory answer can be told apart by, and the wire figure they
// combine into.
type tierShape struct {
	model string
	// addressBytes is the wire width of the address field ahead of the
	// record. MEASURED, not declared: civ.AddressForm.addressBytes is
	// unexported and unreachable from here, so measureShape reads it off
	// two frames this profile builds.
	addressBytes int
	// recordLengths is the profile's accepted RECORD-ONLY length set,
	// ascending — spec D1's set, and spec D3.2's length fingerprint in
	// one value (civ.Profile.RecordLengths).
	recordLengths []int
	// frameLengths is recordLengths with addressBytes added to each: the
	// `1A 00` DATA AREA a radio's answer carries, which is the figure a
	// human reading a wire capture sees and the one the two 111-byte
	// models differ in.
	frameLengths []int
}

// tierProfilePopulation is every Icom profile family in the tier, in
// registration order. Wave 4 extends this package-level list when it imports
// each new profile package; that one-line registration makes the complete
// pair walk below rule on every pairing involving the new family.
var tierProfilePopulation = []civ.Profile{
	ic7610.Profile(),
	ic7300.Profile(),
	ic7300mk2.Profile(),
	ic705.Profile(),
	ic9700.Profile(),
	ic905.Profile(),
	// The additions tier's first family (Tier 4b). ONE entry for TWO
	// registered rows: core/civ/ic7851 is the IC-7851's and the IC-7850's
	// shared profile, so this list — which is a list of FAMILIES, and
	// which must hold no model twice — carries it once, and
	// tierRegistrationCoverage below maps both rows onto it.
	ic7851.Profile(),
	// A PLACE, NOT A ROW, for the IC-7760: it is the third member of the
	// 25 B / Flat declared set (additions spec D5) and its branch is NOT
	// MERGED, so nothing about it may be pre-registered here. When it
	// lands, its family joins this list and its pairings against BOTH the
	// IC-7610 and the IC-7851 must be declared in `indistinguishable`
	// below with their own citations — the test will fail until they are,
	// which is the mechanism working.
}

// tierRegistrationCoverage ties the real driver registry to the profile
// family population above. Wave 4 adds each registered Icom model here and
// adds each new profile family to tierProfilePopulation; two model keys may
// deliberately name one family (IC-7850/IC-7851), but neither side may be
// omitted. The guard in TestTierRecordShapes_DistinctOrDeclared compares
// this table with internal/wiring.SupportedModels, so registration alone
// cannot silently bypass the pairwise ruling.
var tierRegistrationCoverage = map[string]string{
	"IC-7610":    "IC-7610",
	"IC-7300":    "IC-7300",
	"IC-7300MK2": "IC-7300MK2",
	"IC-705":     "IC-705",
	"IC-9700":    "IC-9700",
	"IC-905":     "IC-905",
	// The additions tier's pair: TWO registry keys naming ONE family,
	// which is the case this table's own doc comment above reserved a
	// sentence for before any model needed it. The family's name is the
	// profile's Model(), "IC-7851" (core/civ/ic7851/profile.go), and the
	// IC-7850's row names it too — not because the IC-7850 IS an IC-7851,
	// but because one profile is what both rows read and write with
	// (additions spec D1.2).
	"IC-7851": "IC-7851",
	"IC-7850": "IC-7851",
}

func registrationCoverageProblems(registered, population []string, coverage map[string]string) []string {
	var problems []string
	registeredSet := make(map[string]bool, len(registered))
	populationSet := make(map[string]bool, len(population))
	coveredFamilies := make(map[string]bool, len(population))
	for _, model := range registered {
		registeredSet[model] = true
		family, ok := coverage[model]
		if !ok {
			problems = append(problems, fmt.Sprintf("registered Icom model %q has no tierRegistrationCoverage ruling", model))
			continue
		}
		coveredFamilies[family] = true
	}
	for _, family := range population {
		populationSet[family] = true
	}
	for model, family := range coverage {
		if !registeredSet[model] {
			problems = append(problems, fmt.Sprintf("tierRegistrationCoverage model %q is not registered", model))
		}
		if !populationSet[family] {
			problems = append(problems, fmt.Sprintf("tierRegistrationCoverage[%q] names absent profile family %q", model, family))
		}
	}
	for _, family := range population {
		if !coveredFamilies[family] {
			problems = append(problems, fmt.Sprintf("profile family %q covers no registered Icom model", family))
		}
	}
	return problems
}

// measureShape reads one profile's shape through the EXPORTED API alone,
// hard-coding no number this file has not derived from the profile in
// hand.
//
// THE ADDRESS WIDTH IS THE DIFFERENCE BETWEEN TWO FRAMES. `19 00` and
// `1A 00 <address>` share their entire framing overhead — FE FE, the two
// address bytes, two command bytes and FD — so the whole of the length
// difference between them is the address field. That is a measurement of
// what this profile PUTS ON THE WIRE, which is the thing that matters
// here; a switch over civ.AddressForm's exported constants would only be
// this file restating record.go's own table back at it.
func measureShape(t *testing.T, p civ.Profile) tierShape {
	t.Helper()
	idRead, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("%s: BuildTransceiverIDRead: %v", p.Model(), err)
	}
	// The first addressable channel of the first group this radio has:
	// GroupBase is the WIRE index of that group (0 for the five
	// zero-based models, 1 for the IC-9700's A/B/C bands) and a flat
	// profile refuses any group but 0, which GroupBase already is.
	lo, _ := p.ChannelRange()
	memRead, err := p.BuildMemoryRead(civ.ChannelAddress{Group: p.GroupBase(), Channel: lo})
	if err != nil {
		t.Fatalf("%s: BuildMemoryRead(first channel): %v", p.Model(), err)
	}
	addressBytes := len(memRead.Bytes()) - len(idRead.Bytes())
	if addressBytes <= 0 {
		t.Fatalf("%s: measured address width %d — a `1A 00` read must be longer than a `19 00` read by exactly its address field", p.Model(), addressBytes)
	}
	lengths := p.RecordLengths()
	if len(lengths) == 0 {
		t.Fatalf("%s: RecordLengths() is empty — a profile accepting no record length fingerprints nothing", p.Model())
	}
	frames := make([]int, len(lengths))
	for i, n := range lengths {
		frames[i] = n + addressBytes
	}
	return tierShape{
		model:         p.Model(),
		addressBytes:  addressBytes,
		recordLengths: lengths,
		frameLengths:  frames,
	}
}

// distinguishable reports whether two shapes can be told apart by the two
// properties spec D3.2's probe actually has in hand, and says which one
// did it.
//
// THE ORDER IS THE PROBE'S ORDER. A disjoint accepted-length set is the
// strong form: the fingerprint alone separates the pair, with no appeal
// to the address. Only when the sets overlap does the address width carry
// the separation — which is a weaker claim, since it holds on the WIRE
// (through MemoryAnswerRecord's strip) rather than in the length
// comparison itself, and the reason string says so.
func distinguishable(a, b tierShape) (bool, string) {
	if disjointLengths(a.recordLengths, b.recordLengths) {
		return true, fmt.Sprintf("accepted record-only lengths are disjoint (%v / %v)", a.recordLengths, b.recordLengths)
	}
	if a.addressBytes != b.addressBytes {
		return true, fmt.Sprintf("record-only lengths overlap (%v / %v), separated by ADDRESS WIDTH (%d / %d) and so by wire data area (%v / %v)", a.recordLengths, b.recordLengths, a.addressBytes, b.addressBytes, a.frameLengths, b.frameLengths)
	}
	return false, fmt.Sprintf("record-only lengths overlap (%v / %v) at address width %d — neither property separates this pair", a.recordLengths, b.recordLengths, a.addressBytes)
}

// disjointLengths reports whether two ascending length sets share no
// member.
func disjointLengths(x, y []int) bool {
	for _, n := range x {
		for _, m := range y {
			if n == m {
				return false
			}
		}
	}
	return true
}

// indistinguishable names pairs this project has ADJUDICATED as
// indistinguishable and accepted, keyed "MODEL-A|MODEL-B" in
// tierProfilePopulation order. Each value must cite the documentation that
// records the limitation; an empty value is not a ruling.
//
// IT WAS EMPTY AT THE ICOM TIER'S CLOSE, and that was a measurement: no
// pair among those six families collided. It has ONE entry now, and that
// entry is an admission written down rather than a licence — the pair is
// still logged on every run, and a NEW collision from a family registered
// later still FAILS unless somebody rules on it here.
//
// THE ONE ENTRY IS THE 25-BYTE FLAT SET (additions spec D5's first table
// row). The IC-7610 and the IC-7851 accept the same record-only length,
// 25 bytes, over the same two-byte flat address geometry, because both
// documents draw the SAME 27-byte data area — 2 channel bytes + 25 —
// which is exactly what spec D1.1 predicted and what the two profiles,
// each derived from its OWN evidence legs, independently produced. So
// neither of the two properties this file's probe has in hand can
// separate them, and no arithmetic will make one.
//
// WHAT THIS DOES AND DOES NOT MEAN. In the field the two are separated by
// their CI-V ADDRESSES — 98h and 8Eh — which differ, so a wrong radio at
// its own factory address does not answer and the open times out. The
// declaration is about the FINGERPRINT alone: an IC-7851 MOVED onto 98h
// (or an IC-7610 onto 8Eh) would answer a record of the length the other
// profile expects, and this programme could not tell. That is a
// SAME-ADDRESS confusion, the very case the fingerprint exists for, and
// for this pair the fingerprint is spent. Both drivers' doc comments say
// so in their own words, and neither mints a driver.WrongRadioError from
// a length it cannot attribute.
//
// THE SET HAS A THIRD MEMBER THAT IS NOT HERE YET: spec D5 names
// {IC-7610, IC-7851/7850, IC-7760} at 25 B / Flat. The IC-7760's branch
// is NOT MERGED, so it is deliberately NOT pre-registered — declaring a
// pair against a family this file cannot measure would be a ruling with
// no measurement under it, and the "does not name a canonical population
// pair" arm below would fail it anyway. Its two declarations
// ("IC-7610|IC-7760" and "IC-7851|IC-7760", in whatever order the
// population then has) belong to its own registration.
var indistinguishable = map[string]string{
	"IC-7610|IC-7851": "additions spec D5 (docs/superpowers/specs/2026-08-28-icom-additions-design.md, the 25 B / Flat row: \"NO — declared indistinguishable\") and D1.1's shared-record finding; the two capability matrices under docs/superpowers/icom-matrices/ (the IC-7610's §1/§3 record reading and the IC-7851's §3.16 one, derived independently); core/driver/ic7851/doc.go's Wave-4 hand-off section, which names this set; and core/driver/ic7610/ic7610.go:142-145, which already refuses to mint a driver.WrongRadioError for any same-address collision. The two radios' factory addresses (98h and 8Eh) differ, so this limitation is reachable only on a radio moved onto the other's address.",
}

// TestTierRecordShapes_DistinctOrDeclared is the tier-close check: every
// pair in tierProfilePopulation is either separated by today's exact
// length/geometry proof or named in indistinguishable with a citation.
//
// It prints the measured table on every run, pass or fail. The table is
// the deliverable as much as the verdict is — six models' record
// geometry in one place is exactly what no per-model worktree could
// write down.
func TestTierRecordShapes_DistinctOrDeclared(t *testing.T) {
	profiles := tierProfilePopulation
	if len(profiles) == 0 {
		t.Fatal("tierProfilePopulation is empty — the pairwise proof would pass vacuously")
	}
	shapes := make([]tierShape, len(profiles))
	models := make(map[string]bool, len(profiles))
	for i, p := range profiles {
		shapes[i] = measureShape(t, p)
		if models[shapes[i].model] {
			t.Fatalf("tierProfilePopulation contains %q twice", shapes[i].model)
		}
		models[shapes[i].model] = true
	}
	var registered, population []string
	for _, model := range wiring.SupportedModels() {
		if strings.HasPrefix(model, "IC-") {
			registered = append(registered, model)
		}
	}
	for _, shape := range shapes {
		population = append(population, shape.model)
	}
	for _, problem := range registrationCoverageProblems(registered, population, tierRegistrationCoverage) {
		t.Error(problem)
	}

	// A declaration must name a real population pair in canonical order and
	// must remain an actual collision. This stops stale or misspelt entries
	// from becoming blanket exemptions.
	for key, citation := range indistinguishable {
		if citation == "" {
			t.Errorf("indistinguishable[%q] has no documentation citation", key)
		}
		found := false
		for i := 0; i < len(shapes); i++ {
			for j := i + 1; j < len(shapes); j++ {
				if key != shapes[i].model+"|"+shapes[j].model {
					continue
				}
				found = true
				if ok, why := distinguishable(shapes[i], shapes[j]); ok {
					t.Errorf("indistinguishable[%q] is stale: %s", key, why)
				}
			}
		}
		if !found {
			t.Errorf("indistinguishable[%q] does not name a canonical population pair", key)
		}
	}

	t.Log("measured record shapes (every figure read off the profile, none hard-coded here):")
	t.Logf("  %-12s %-14s %-18s %s", "MODEL", "ADDRESS BYTES", "RECORD-ONLY", "WIRE DATA AREA")
	for _, s := range shapes {
		// The length sets are rendered to a string FIRST: a width verb
		// applied to a slice pads every ELEMENT of it, which turns a
		// column meant to line up into "[25                ]".
		t.Logf("  %-12s %-14d %-18s %s", s.model, s.addressBytes, fmt.Sprint(s.recordLengths), fmt.Sprint(s.frameLengths))
	}
	t.Log("HONESTLY RECORDED: every length above is an ASSUMED derivation from printed field widths — no document in this tier prints a record total and no radio has confirmed one — and the length fingerprint is DEFENCE IN DEPTH behind the `19 00` address probe every one of the six drivers runs first, since the six CI-V addresses are already distinct and a wrong radio simply does not answer.")

	seen := 0
	for i := 0; i < len(shapes); i++ {
		for j := i + 1; j < len(shapes); j++ {
			seen++
			a, b := shapes[i], shapes[j]
			ok, why := distinguishable(a, b)
			if ok {
				t.Logf("%s vs %s: %s", a.model, b.model, why)
				continue
			}
			key := a.model + "|" + b.model
			if note, recorded := indistinguishable[key]; recorded {
				t.Logf("LIMITATION (recorded, not a regression) %s vs %s: %s — %s", a.model, b.model, why, note)
				continue
			}
			t.Errorf("%s vs %s: %s — a pair the probe cannot separate is a radio this program could open as the wrong model; if this is a documented fact rather than a profile mistake, record it in indistinguishable with the adjudication citation", a.model, b.model, why)
		}
	}
	if want := len(shapes) * (len(shapes) - 1) / 2; seen != want {
		t.Errorf("compared %d pairs, want %d — the pairwise walk is broken and this test passed vacuously", seen, want)
	}
}

func TestTierRecordShapes_RegistrationCoverageGuardIsNotVacuous(t *testing.T) {
	registered := []string{"IC-ONE", "IC-FUTURE"}
	population := []string{"IC-ONE"}
	coverage := map[string]string{"IC-ONE": "IC-ONE"}
	problems := registrationCoverageProblems(registered, population, coverage)
	if len(problems) != 1 || !strings.Contains(problems[0], "IC-FUTURE") {
		t.Fatalf("registrationCoverageProblems = %v, want one missing-ruling problem for IC-FUTURE", problems)
	}
}

// TestTierRecordShapes_705And9700 pins the ONE pair that shares a
// record-only length, and pins WHERE its separation comes from.
//
// A test that only asserted "all pairs distinguishable" would go on
// passing if the IC-705's address form were widened to the IC-9700's:
// the sets would still overlap, the widths would now agree, and the
// generic loop would report the collision — but nothing would say that
// this pair's separation was ever supposed to be the address width in
// the first place. This does.
func TestTierRecordShapes_705And9700(t *testing.T) {
	p705 := measureShape(t, ic705.Profile())
	p9700 := measureShape(t, ic9700.Profile())

	if !equalLengths(p705.recordLengths, p9700.recordLengths) {
		t.Fatalf("IC-705 record-only lengths %v, IC-9700 %v — this test exists BECAUSE the two share a length; if they no longer do, the plan's premise has changed and this test should be rewritten rather than relaxed", p705.recordLengths, p9700.recordLengths)
	}
	if disjointLengths(p705.recordLengths, p9700.recordLengths) {
		t.Fatalf("IC-705 %v and IC-9700 %v are disjoint after comparing equal — disjointLengths is broken", p705.recordLengths, p9700.recordLengths)
	}
	if p705.addressBytes == p9700.addressBytes {
		t.Fatalf("IC-705 and IC-9700 both address a channel in %d bytes AND accept the same record-only lengths %v — the ONLY thing separating these two radios has gone", p705.addressBytes, p705.recordLengths)
	}
	t.Logf("IC-705 and IC-9700 share record-only %v; address widths %d and %d separate them", p705.recordLengths, p705.addressBytes, p9700.addressBytes)

	// And the separation reaches the wire: the figure that actually
	// differs between these two radios' answers is the `1A 00` data area.
	if !disjointLengths(p705.frameLengths, p9700.frameLengths) {
		t.Errorf("IC-705 wire data areas %v and IC-9700's %v are not disjoint — with the record-only lengths equal, an answer from one radio would fingerprint as the other's", p705.frameLengths, p9700.frameLengths)
	}
	t.Logf("wire data areas: IC-705 %v, IC-9700 %v — disjoint, so an answer from one is refused by the other's profile", p705.frameLengths, p9700.frameLengths)

	// THE SECOND REFUSAL, PINNED AS ARITHMETIC. Strip each profile's own
	// address width off the OTHER's data area and the remainder is not a
	// record either profile accepts. This is a real and independent
	// property — but it is not the one that fires, and the sweep below is
	// what establishes which does.
	for _, tc := range []struct {
		reader   civ.Profile
		dataArea int
		width    int
	}{
		{ic705.Profile(), p9700.frameLengths[0], p705.addressBytes},
		{ic9700.Profile(), p705.frameLengths[0], p9700.addressBytes},
	} {
		remainder := tc.dataArea - tc.width
		if tc.reader.AcceptsRecordLength(remainder) {
			t.Errorf("%s accepts a record of %d bytes (%d-byte data area less its own %d-byte address) — the length arithmetic no longer separates this pair either", tc.reader.Model(), remainder, tc.dataArea, tc.width)
		}
		// AND THE GATE IS REACHABLE, which is what stops the sweep below
		// passing for the wrong reason. Give this profile its OWN address
		// geometry — so answerBody and decodeAddress both succeed — with
		// the foreign remainder as the record, and the length check is
		// what refuses it. Without this leg a crossAnswer frame that had
		// been rejected by answerBody's `from` check would also count as
		// a *civ.ParseError, and the sweep would report the wrong gate.
		selfAddr := civ.ChannelAddress{Group: tc.reader.GroupBase(), Channel: firstChannel(tc.reader)}
		_, _, err := tc.reader.MemoryAnswerRecord(answerFrame(t, tc.reader, tc.reader, selfAddr, remainder))
		var lenErr *civ.RecordLengthError
		if !errors.As(err, &lenErr) {
			t.Errorf("%s reading a well-addressed answer carrying a %d-byte record: err = %v (%T), want a *civ.RecordLengthError — the length gate is unreachable, so the sweep below cannot say which gate refused", tc.reader.Model(), remainder, err, err)
			continue
		}
		if lenErr.Got != remainder {
			t.Errorf("%s: RecordLengthError.Got = %d, want %d", tc.reader.Model(), lenErr.Got, remainder)
		}
		t.Logf("%s refuses a well-addressed %d-byte record with %v — the second gate is real and reachable", tc.reader.Model(), remainder, lenErr)
	}

	// THE FIRST REFUSAL, MEASURED RATHER THAN DESCRIBED. Every address
	// either radio has, in both directions, exhaustively: 321 for the
	// IC-9700 (3 bands x channels 1..107) and 10,100 for the IC-705 (101
	// groups x channels 0..99). Each one becomes the `1A 00` answer that
	// radio would put on the wire IF IT HAD BEEN MOVED ONTO THE OTHER'S
	// CI-V ADDRESS — the same-address confusion spec D3.2's fingerprint
	// exists for, and the only circumstance in which these two can meet
	// at all, since each answers at its own distinct address and a wrong
	// one does not answer.
	//
	// The verdict is uniform and it is the ADDRESS: *civ.ParseError from
	// decodeAddress every time, *civ.RecordLengthError not once.
	sweep := func(t *testing.T, src, dst civ.Profile, addrs []civ.ChannelAddress) {
		t.Helper()
		if len(addrs) == 0 {
			t.Fatalf("%s -> %s: no addresses swept — this leg would pass vacuously", src.Model(), dst.Model())
		}
		parseErrs, lenErrs, accepted := 0, 0, 0
		for _, a := range addrs {
			_, _, err := dst.MemoryAnswerRecord(crossAnswer(t, src, dst, a))
			var lenErr *civ.RecordLengthError
			var parseErr *civ.ParseError
			switch {
			case err == nil:
				accepted++
				if accepted == 1 {
					t.Errorf("%s's answer for %+v is ACCEPTED by the %s profile — one radio's memory answer read as the other's, which is the whole failure this pair's separation exists to prevent", src.Model(), a, dst.Model())
				}
			case errors.As(err, &lenErr):
				lenErrs++
			case errors.As(err, &parseErr):
				parseErrs++
			default:
				t.Errorf("%s's answer for %+v refused by the %s profile with an unexpected error type %T: %v", src.Model(), a, dst.Model(), err, err)
			}
		}
		if accepted != 0 {
			t.Errorf("%s -> %s: %d of %d answers accepted, want 0", src.Model(), dst.Model(), accepted, len(addrs))
		}
		if parseErrs != len(addrs) {
			t.Errorf("%s -> %s: %d of %d refusals were address parse errors, want all of them", src.Model(), dst.Model(), parseErrs, len(addrs))
		}
		if lenErrs != 0 {
			t.Errorf("%s -> %s: %d refusals were *civ.RecordLengthError — the doc block and the drivers' WrongRadioError branches both say this pair never reaches the length check, and that is no longer true", src.Model(), dst.Model(), lenErrs)
		}
		t.Logf("%s answers read by the %s profile: %d addresses, %d address parse errors, %d length errors, %d accepted", src.Model(), dst.Model(), len(addrs), parseErrs, lenErrs, accepted)
	}

	var ic9700Addrs []civ.ChannelAddress
	lo9700, hi9700 := ic9700.Profile().ChannelRange()
	for band := ic9700.Profile().GroupBase(); band < ic9700.Profile().GroupBase()+ic9700.Profile().Groups(); band++ {
		for ch := lo9700; ch <= hi9700; ch++ {
			ic9700Addrs = append(ic9700Addrs, civ.ChannelAddress{Group: band, Channel: ch})
		}
	}
	var ic705Addrs []civ.ChannelAddress
	lo705, hi705 := ic705.Profile().ChannelRange()
	for g := ic705.Profile().GroupBase(); g < ic705.Profile().GroupBase()+ic705.Profile().Groups(); g++ {
		for ch := lo705; ch <= hi705; ch++ {
			ic705Addrs = append(ic705Addrs, civ.ChannelAddress{Group: g, Channel: ch})
		}
	}
	if len(ic9700Addrs) != 321 {
		t.Fatalf("swept %d IC-9700 addresses, want 321 (3 bands x 107 channels)", len(ic9700Addrs))
	}
	if len(ic705Addrs) != 10100 {
		t.Fatalf("swept %d IC-705 addresses, want 10,100 (101 groups x 100 channels)", len(ic705Addrs))
	}
	sweep(t, ic9700.Profile(), ic705.Profile(), ic9700Addrs)
	sweep(t, ic705.Profile(), ic9700.Profile(), ic705Addrs)
}

// crossAnswer builds the `1A 00` memory answer a radio with src's address
// geometry and record length would put on the wire IF IT HAD BEEN MOVED
// ONTO dst's CI-V address.
//
// THE MOVED ADDRESS IS THE POINT, and it is why this cannot just hand
// dst's parser a frame src built. The six CI-V addresses are distinct, so
// a frame carrying src's own `from` byte is refused by answerBody
// (core/civ/parse.go:31) before any geometry is looked at — a THIRD and
// even earlier refusal, and the one that operates in the field. What spec
// D3.2's length fingerprint is FOR is the case that gets past it: a radio
// moved onto this address, or a bus mis-set. So the address bytes and the
// record width are src's, and only the two frame address bytes are dst's.
//
// It delegates the framing to answerFrame, which every leg of this file
// shares.
func crossAnswer(t *testing.T, src, dst civ.Profile, addr civ.ChannelAddress) []byte {
	t.Helper()
	return answerFrame(t, src, dst, addr, src.BuildRecordLength())
}

// answerFrame builds a `1A 00` answer carrying SRC's address bytes for
// addr and a zeroed record of recordLen bytes, framed as arriving from
// DST's radio to DST's controller.
//
// A zeroed record because every test here asks WHICH GATE REFUSES, and
// under both refusals the record's contents reach no field decoder at
// all. The address field is taken from src's OWN read frame rather than
// re-encoded here: `1A 00 <address>` has a fixed seven-byte overhead, so
// the bytes between the sub-command and the terminator are exactly what
// src's encoder produced, and this helper invents no BCD of its own.
func answerFrame(t *testing.T, src, dst civ.Profile, addr civ.ChannelAddress, recordLen int) []byte {
	t.Helper()
	read, err := src.BuildMemoryRead(addr)
	if err != nil {
		t.Fatalf("%s: BuildMemoryRead(%+v): %v", src.Model(), addr, err)
	}
	body := read.Bytes()
	addrBytes := body[6 : len(body)-1]

	frame := make([]byte, 0, 7+len(addrBytes)+recordLen)
	frame = append(frame, civ.PreambleByte, civ.PreambleByte, dst.ControllerAddress(), dst.RadioAddress(), civ.CmdMemory, civ.SubMemoryContents)
	frame = append(frame, addrBytes...)
	frame = append(frame, make([]byte, recordLen)...)
	return append(frame, civ.EndByte)
}

// firstChannel is the lowest channel number a profile addresses.
func firstChannel(p civ.Profile) int {
	lo, _ := p.ChannelRange()
	return lo
}

// TestTierRecordShapes_CheckIsNotVacuous is the permanent red proof:
// distinguishable must REFUSE a pair the six real profiles never
// produce.
//
// Two shapes that agree on both properties are what the check is for, and
// a check that answered "distinguishable" to everything would have passed
// the loop above in silence. The pair here is a LOCAL FIXTURE — the
// frozen per-model profiles are not touched, and could not be: a
// civ.Profile is immutable and built from a compile-time literal.
func TestTierRecordShapes_CheckIsNotVacuous(t *testing.T) {
	twin := tierShape{model: "TWIN-A", addressBytes: 3, recordLengths: []int{111}, frameLengths: []int{114}}
	other := twin
	other.model = "TWIN-B"
	if ok, why := distinguishable(twin, other); ok {
		t.Errorf("distinguishable(identical shapes) = true (%s), want false — the pairwise check would pass vacuously", why)
	}

	// One byte of address width is enough to separate them, and nothing
	// else about the pair changed.
	wider := other
	wider.addressBytes = 4
	wider.frameLengths = []int{115}
	if ok, _ := distinguishable(twin, wider); !ok {
		t.Error("distinguishable(same lengths, different address widths) = false, want true — this is exactly the IC-705/IC-9700 case")
	}

	// So is a disjoint length set at the same address width.
	longer := other
	longer.recordLengths = []int{112}
	longer.frameLengths = []int{115}
	if ok, _ := distinguishable(twin, longer); !ok {
		t.Error("distinguishable(disjoint lengths, same address width) = false, want true")
	}
}

// equalLengths reports whether two ascending length sets are identical.
func equalLengths(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
