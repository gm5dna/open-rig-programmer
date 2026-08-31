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
// TEN FAMILIES AND ELEVEN REGISTERED ROWS as of the additions tier's
// FOURTH and LAST registration (Tier 4b): the Icom tier's six families,
// plus core/civ/ic7851, which serves the IC-7851 and IC-7850 rows ALIKE,
// plus core/civ/ic7760, core/civ/ic7100 and core/civ/icr8600. A family is
// a PROFILE; a row is a registry key; tierRegistrationCoverage below is
// the map between them, and it is the reason the two counts may
// legitimately differ.
//
// THE TENTH FAMILY IS A RECEIVER, and it is the first family here whose
// profile is MODE-KEYED: core/civ/icr8600 accepts six record-only lengths
// {37, 39, 41, 43, 44, 45} rather than one or two, with FM and DCR BOTH
// at 44 and told apart by the mode byte (civ.DiscriminatorModeByte)
// rather than by length. NOTHING IN THIS FILE NEEDED WIDENING FOR THAT:
// measureShape has read civ.Profile.RecordLengths as a SET since the
// tier's close, and disjointLengths compares sets, so a six-member set
// arrives here as an ordinary value.
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
// TEN FAMILIES' CI-V addresses are already distinct — 98h, 94h, B6h,
// A4h, A2h, ACh, 8Eh, B2h, 88h, 96h — so IN THE FIELD a wrong radio on the port does not
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
// TWO SETS OF THREE SHARE A RECORD-ONLY LENGTH, and they fail
// differently — and the tenth family, the IC-R8600, shares lengths with
// two more families and is SEPARABLE from both, which is worth stating
// before the declarations because it is the case this file would rather
// report.
//
// The IC-7610, the IC-7851 and the IC-7760 share BOTH properties — 25
// bytes record-only and a two-byte flat address — so all THREE of that
// set's pairings are DECLARED indistinguishable in the table below rather
// than proven apart; see that table's own comment.
//
// The IC-705, the IC-9700 and the IC-7100 all accept exactly {111}, so a
// fingerprint that compared record-only lengths ALONE could not tell any
// of the three apart. Here the ADDRESS WIDTH does part of the work and
// not all of it: the IC-705 uses FOUR address bytes where the other two
// use THREE, so BOTH of its pairings are proven apart — and the section
// below traces that separation in full, because it is the one this file
// can demonstrate exhaustively. THE THIRD PAIRING IS NOT SEPARABLE. The
// IC-9700 and the IC-7100 agree on both properties: three address bytes
// each, and the leading byte is a small index in both cases (a band 01–03
// against a bank 01–05), which is nothing the wire can tell apart. That
// pairing is DECLARED in the table below, citing spec D5's 111 B row and
// spec D2.1, and it is the only reason the walk below does not fail on
// this set. What follows describes the SEPARABLE half.
//
// THE IC-R8600 OVERLAPS THE IC-7300 PAIR AND IS PROVEN APART. Its
// accepted set {37, 39, 41, 43, 44, 45} CONTAINS both 39 (the IC-7300's
// only length) and 45 (the IC-7300MK2's), so a fingerprint comparing
// lengths alone could not separate either pair — the same failure the two
// declared sets have. What rescues it is the ADDRESS WIDTH, and by a
// wider margin than the 111 B row's: the IC-R8600 addresses a channel in
// FOUR bytes (civ.AddressFormWideGroupChannel, two group bytes then two
// channel bytes) where both IC-7300s use a two-byte FLAT address, so the
// pairwise walk below separates both pairings and prints its reason. The
// remaining seven pairings are separated by LENGTH alone, the strong
// form: the set is disjoint from {25} (three profiles), from {111}
// (three) and from {64, 65} (one), which is what additions spec D5's own
// IC-R8600 row claims ("yes (set disjoint from {64, 65}; geometry from
// the 2/3 B profiles)"). The spec's clause is slightly generous to
// itself, and this file MEASURES rather than repeats it: the two 3 B
// profiles are separated by LENGTH here and not by geometry, the three
// 25 B profiles likewise, and the IC-7300 pair alone needs the address
// width.
//
// SO THE IC-R8600 DECLARES NOTHING, and it is the first additions family
// of which that is true. `indistinguishable` below is unchanged by its
// arrival, and TestTierRecordShapes_DistinctOrDeclared's table shows it
// distinct from all nine.
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
// DESIGN — not merely because this pair pre-empts it. Its doc.go says so in
// as many words ("No driver.WrongRadioError is ever minted here"), and that
// package's TestUnexpectedLengthIsRefusedWithoutAttribution pins it: naming
// a model from {111} would be the cross-model claim no per-model worktree
// may make, which is the reason this file exists. This file mints nothing
// of its own either.
//
// IT DOES NOW MATCH driver.ErrWrongRadio, WHICH IS NOT THE SAME THING, and
// the earlier wording here — that its probe "wraps every error from
// MemoryAnswerRecord generically" — stopped being true when it did.
// core/driver/ic9700/ic9700.go:464 wraps a record-length refusal in a
// *RecordLengthMismatchError whose multi-error Unwrap returns
// driver.ErrWrongRadio ALONGSIDE the original *civ.RecordLengthError, so
// errors.Is(err, driver.ErrWrongRadio) succeeds against it and
// errors.As(err, &civLengthErr) still does too. The refusal stays
// ANONYMOUS: no model name, no driver.WrongRadioError value, no table of
// other models' record lengths — the driver has none to consult. What
// changed is only that a caller can ask "is this a wrong-radio refusal?"
// and be told yes; the rule above is untouched.
//
// THE IC-7100 MINTS NONE EITHER, BY THE SAME REASONING AND IN ITS OWN
// WORDS. core/driver/ic7100/doc.go's Wave-4 hand-off section records that
// "measuring 111 record bytes proves the radio is not an IC-7610 or an
// IC-7300, and proves NOTHING about whether it is an IC-7100 or an
// IC-9700", so its probe refuses a foreign record length with a
// driver.WrongRadioError carrying the two RECORD-ONLY lengths and NO
// model name. A name appears there only when a caller injects one through
// WithSiblingRecordLengths — which no registered composition passes, so
// the refusal renders as the ID-only form, exactly as the IC-905's does.
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
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7100"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/spec"
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
	// The additions tier's second family (Tier 4b), and the third and
	// last member of spec D5's 25 B / Flat set. Its branch is merged, so
	// the PLACE the IC-7851's registration left here is now a ROW, and
	// both of its pairings — against the IC-7610 and against the IC-7851
	// — are declared in `indistinguishable` below with their own
	// citations. Until they were, this line alone failed the pairwise
	// walk, which is the mechanism working.
	ic7760.Profile(),
	// The additions tier's THIRD family (Tier 4b), and the third member of
	// spec D5's 111 B row. Its branch is merged, so it enters as a ROW
	// with no PLACE stage: its one colliding pairing, against the IC-9700,
	// is declared in `indistinguishable` below with its own citation.
	// Until it was, this line alone failed the pairwise walk, which is the
	// mechanism working.
	ic7100.Profile(),
	// The additions tier's FOURTH and LAST family (Tier 4b), and the only
	// row of spec D5's {37, 39, 41, 43, 44, 45} line. Its branch is
	// merged, so it enters as a ROW with no PLACE stage — and unlike the
	// three families above it, it DECLARES NOTHING: all nine of its
	// pairings are proven apart below, seven by disjoint lengths and the
	// two against the IC-7300 pair by address width (four bytes against
	// two), their length sets overlapping at 39 and at 45. It is also the
	// first MODE-KEYED profile in this list, which this file needed no
	// change to accept: RecordLengths is a set here as it is everywhere
	// else.
	icr8600.Profile(),
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
	// The additions tier's second registration: ONE key, ONE family, the
	// ordinary case again.
	"IC-7760": "IC-7760",
	// And its third: ONE key, ONE family.
	"IC-7100": "IC-7100",
	// And its fourth and last, the tier's only RECEIVER: ONE key, ONE
	// family. Being receive-only is a spec.Capabilities property
	// (additions spec D4.2) and nothing this file measures — a receiver's
	// record has a length and an address geometry like any other radio's,
	// and those are the two properties ruled on here.
	"IC-R8600": "IC-R8600",
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

// icomModels filters models to the ones the registry itself declares
// Icom, via capsFor — normally wiring.StaticCapabilities.
//
// THE SIGNAL IS A PROXY, NOT A DECLARATION: neither spec.Capabilities nor
// internal/wiring carries a Vendor/Manufacturer field (fix round 1, F6 —
// checked again here rather than assumed), and no registry surface
// exposes a row's civ.Profile or otherwise says "this is CI-V" directly.
// In its absence icomModels reads spec.Capabilities.CTCSSToneRange, whose
// own doc comment (core/spec/capabilities.go) states it is non-nil for
// "every CI-V model in the Icom tier" and nil for every registered Yaesu
// one — the closest thing to a vendor declaration this registry has,
// FALLING OUT of the CI-V tone encoding rather than stating vendor as a
// fact. A strings.HasPrefix(model, "IC-") check reads the model NAME
// instead, and would miss a real Icom model registered under the
// family's other prefix — Icom's D-STAR handhelds are "ID-51", "ID-52",
// "ID-5100" — leaving it out of tierRegistrationCoverage's guard
// entirely. TestTierRecordShapes_IcomModelsKeyOnVendorNotPrefix pins a
// fake "ID-52" row being caught; TestTierRecordShapes_IcomModelsMatchesRegistryRowCountAndNames
// pins today's real registry selection (count and names), so a drift in
// the proxy's behaviour is visible even though nothing enforces it
// structurally; TestTierRecordShapes_EveryModelDeclaresExactlyOneToneShape
// is the guard against the proxy's one known failure mode — a model
// declaring NEITHER CTCSSTones nor CTCSSToneRange, which
// spec.Capabilities.AdmitsTone explicitly tolerates ("fails closed when a
// radio declares neither") and which would silently vanish from
// `registered` rather than fail loudly.
func icomModels(t testing.TB, models []string, capsFor func(string) (spec.Capabilities, error)) []string {
	t.Helper()
	var out []string
	for _, model := range models {
		caps, err := capsFor(model)
		if err != nil {
			t.Fatalf("capabilities for registered model %q: %v", model, err)
		}
		if caps.CTCSSToneRange != nil {
			out = append(out, model)
		}
	}
	return out
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
// pair among those six families collided. It has FOUR entries now, and
// each is an admission written down rather than a licence — every one of
// them is still logged on every run, and a NEW collision from a family
// registered later still FAILS unless somebody rules on it here.
//
// THE FIRST THREE ENTRIES ARE ONE FINDING: the 25-byte flat set (additions spec
// D5's first table row) is COMPLETE, and a set of three models yields
// three pairings. The IC-7610, the IC-7851 and the IC-7760 all accept the
// same record-only length, 25 bytes, over the same two-byte flat address
// geometry, because all three documents draw the SAME 27-byte data area —
// 2 channel bytes + 25 — which is exactly what spec D1.1 predicted and
// what the three profiles, each derived from its OWN evidence legs,
// independently produced. So neither of the two properties this file's
// probe has in hand can separate any pair of them, and no arithmetic will
// make one.
//
// WHAT THIS DOES AND DOES NOT MEAN. In the field the three are separated
// by their CI-V ADDRESSES — 98h, 8Eh and B2h — which are all distinct, so
// a wrong radio at its own factory address does not answer and the open
// times out. The declaration is about the FINGERPRINT alone: any one of
// the three MOVED onto another's address would answer a record of the
// length that profile expects, and this programme could not tell. That is
// a SAME-ADDRESS confusion, the very case the fingerprint exists for, and
// across this set the fingerprint is spent. All three drivers' doc
// comments say so in their own words, and none of them mints a
// driver.WrongRadioError from a length it cannot attribute.
//
// THAT SET IS NOW CLOSED, and that is worth stating because an earlier
// version of this comment recorded a member that was missing. Spec D5
// names {IC-7610, IC-7851/7850, IC-7760} at 25 B / Flat and this table
// carries all three pairings; a FOURTH member would need its own three
// declarations, and the pairwise walk below would fail until it had them.
//
// THE FOURTH ENTRY IS A SECOND, SEPARATE FINDING, in spec D5's 111 B row
// and with a DIFFERENT shape: that row holds three profiles too, but only
// ONE of its three pairings collides. The IC-705, the IC-9700 and the
// IC-7100 all accept exactly {111} record-only bytes, so the length alone
// separates none of them — but the IC-705 addresses a channel in FOUR
// bytes where the other two use THREE, so both of its pairings are
// separated by geometry and are proven apart below rather than declared.
// What is left is the IC-9700 against the IC-7100: three address bytes
// each, both leading with a small index byte (a band 01–03 on one, a bank
// 01–05 on the other) that the wire cannot tell apart, and the same
// record length behind it. Neither property this file's probe holds
// separates them, and no arithmetic will make one.
//
// AND NOTE WHAT THE 111 B ROW COSTS THAT THE 25 B ROW DOES NOT. Those
// three radios do NOT share a record layout: the IC-7100's own document
// draws a 47-byte transmit duplicate and sixteen name bytes at record
// offset 95 (core/civ/ic7100/profile.go), and nothing here claims the
// three are clones the way spec D1.1 claims it of the 25 B set. The
// collision is in the two properties a PROBE has in hand — length and
// address width — and those are exactly what this table rules on. It is
// therefore possible for two of these radios to disagree byte for byte
// inside the record and still be indistinguishable to the fingerprint,
// which is a sharper statement of the limitation than the 25 B row needs.
//
// KEYS ARE IN tierProfilePopulation ORDER, which the "does not name a
// canonical population pair" arm below enforces: ic7610 precedes ic7851,
// which precedes ic7760, so the IC-7760's two keys read "IC-7610|IC-7760"
// and "IC-7851|IC-7760" and neither may be written the other way round;
// ic9700 precedes ic7100, so the fourth entry reads "IC-9700|IC-7100".
//
// THE DECLARATION IS ABOUT THE FINGERPRINT, NOT ABOUT THE LAYOUTS BEING
// THE SAME. TestTierRecordShapes_7610CloneFamily below measures how far
// spec D1.1's "expected to be byte-identical" actually holds across the
// 25 B three, and it does NOT hold uniformly — the IC-7851 excludes its
// printed fixed pad cells from three spans where the other two include
// them. That divergence changes nothing here: the accepted LENGTH and the
// ADDRESS GEOMETRY are what this table rules on, and those three
// pairings collide whatever the spans inside the record do.
var indistinguishable = map[string]string{
	"IC-7610|IC-7851": "additions spec D5 (docs/superpowers/specs/2026-08-28-icom-additions-design.md, the 25 B / Flat row: \"NO — declared indistinguishable\") and D1.1's shared-record finding; the two capability matrices under docs/superpowers/icom-matrices/ (the IC-7610's §1/§3 record reading and the IC-7851's §3.16 one, derived independently); core/driver/ic7851/doc.go's Wave-4 hand-off section, which names this set; and core/driver/ic7610/ic7610.go:142-145, which already refuses to mint a driver.WrongRadioError for any same-address collision. The two radios' factory addresses (98h and 8Eh) differ, so this limitation is reachable only on a radio moved onto the other's address.",
	"IC-7610|IC-7760": "additions spec D5 (the same 25 B / Flat row, whose declared set is {IC-7610, IC-7851/7850, IC-7760}) and D1.1's shared-record finding; the two capability matrices under docs/superpowers/icom-matrices/ (the IC-7610's §1/§3 record reading and the IC-7760's §3.11/§1b one, whose 25 B record-only and 2 B address width were derived from that radio's own L/W/B/G legs and its own document, the IC-7760 CI-V Reference Guide revision 2, A7788-8EX-2); core/driver/ic7760/doc.go, which admits no cross-model length table; and core/driver/ic7610/ic7610.go:142-145, which already refuses to mint a driver.WrongRadioError for any same-address collision. The two radios' factory addresses (98h and B2h) differ, so this limitation is reachable only on a radio moved onto the other's address.",
	"IC-7851|IC-7760": "additions spec D5 (the same 25 B / Flat row) and D1.1; the IC-7851's §3.16 record reading and the IC-7760's §3.11/§1b one, derived from two different documents by two different evidence-leg sets — the IC-7850/IC-7851 Instruction Manual section 18, and the IC-7760 CI-V Reference Guide revision 2; core/driver/ic7851/doc.go's Wave-4 hand-off section, which names this set, and core/driver/ic7760/doc.go. NEITHER driver mints a driver.WrongRadioError for a length it cannot attribute, so a same-address collision here fails the open without naming a model. The factory addresses (8Eh and B2h) differ, so the limitation is reachable only on a moved radio — and note that the IC-7851 row's own sibling, the IC-7850, shares 8Eh with it AT FACTORY DEFAULTS, which is a separate and stronger limitation recorded in this file's header.",
	"IC-9700|IC-7100": "additions spec D5 (docs/superpowers/specs/2026-08-28-icom-additions-design.md, the 111 B row: \"9700 vs 7100: NO — both 3 B with a leading 01–05 index byte — declared indistinguishable\") and D2.1, which derives the IC-7100's 111 B record-only length over a 3 B address and states in as many words that it is \"the SAME record-only length as the IC-705 and IC-9700, at the 9700's address width\"; the two capability matrices under docs/superpowers/icom-matrices/ (the IC-9700's §3.11 record arithmetic, from the IC-9700 CI-V Reference Guide, and the IC-7100's §3.11 one, from section 20 of the IC-7100 full manual revision A7085-2EX-5 — two different documents, two different evidence-leg sets, and the IC-7100's matrix §4 states explicitly that it makes no cross-model claim of its own). NEITHER driver mints a driver.WrongRadioError for a length it cannot attribute: core/driver/ic9700's doc.go says \"No driver.WrongRadioError is ever minted here\", and core/driver/ic7100's doc.go carries the Wave-4 hand-off naming this very pair — \"measuring 111 record bytes proves the radio is not an IC-7610 or an IC-7300, and proves NOTHING about whether it is an IC-7100 or an IC-9700\" — so a same-address collision here fails the open without naming a model. The third member of spec D5's 111 B row, the IC-705, IS separable from both by address width (four bytes against three) and is proven apart by the pairwise walk rather than declared here. The two radios' factory addresses (A2h and 88h) differ, so this limitation is reachable only on a radio moved onto the other's address.",
}

// TestTierRecordShapes_DistinctOrDeclared is the tier-close check: every
// pair in tierProfilePopulation is either separated by today's exact
// length/geometry proof or named in indistinguishable with a citation.
//
// It prints the measured table on every run, pass or fail. The table is
// the deliverable as much as the verdict is — TEN families' record
// geometry in one place is exactly what no per-model worktree could
// write down.
//
// AND THE TABLE IS WHERE THE IC-R8600'S DISTINCTNESS IS SHOWN. That row
// declares nothing in `indistinguishable`, so the only record of its
// separation is the walk's own printed reasons: seven pairings by
// disjoint lengths, and its two against the IC-7300 pair by address
// width, {37, 39, 41, 43, 44, 45} containing both of those profiles'
// single lengths. A regression that widened its address form to two bytes
// would turn both of those into failures here rather than into silence.
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
	registered := icomModels(t, wiring.SupportedModels(), wiring.StaticCapabilities)
	var population []string
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
	t.Log("HONESTLY RECORDED: every length above is an ASSUMED derivation from printed field widths — no document in this tier prints a record total and no radio has confirmed one — and the length fingerprint is DEFENCE IN DEPTH behind the `19 00` address probe every one of the TEN families' drivers runs first, since the ten CI-V addresses are already distinct and a wrong radio simply does not answer. The ONE exception is the IC-7851/IC-7850 pair, which shares 8Eh at factory defaults; see this file's header.")

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

// TestTierRecordShapes_IcomModelsKeyOnVendorNotPrefix pins that icomModels
// selects on the registry's own Icom declaration (CTCSSToneRange
// non-nil), not on the model NAME. "ID-52" is a real Icom model number
// (the D-STAR handhelds carry the "ID-" prefix, not "IC-"), and a
// strings.HasPrefix(model, "IC-") filter would silently drop a row
// registered under it, taking it out of tierRegistrationCoverage's guard
// entirely. "FT-710" stands in for a registered Yaesu row, which
// icomModels must still exclude.
func TestTierRecordShapes_IcomModelsKeyOnVendorNotPrefix(t *testing.T) {
	icomToneRange := &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1}
	capsFor := func(model string) (spec.Capabilities, error) {
		switch model {
		case "IC-7610", "ID-52":
			return spec.Capabilities{Model: model, CTCSSToneRange: icomToneRange}, nil
		case "FT-710":
			return spec.Capabilities{Model: model}, nil // Yaesu: declares a CTCSSTones list instead
		default:
			return spec.Capabilities{}, fmt.Errorf("capsFor: unexpected model %q", model)
		}
	}
	got := icomModels(t, []string{"IC-7610", "ID-52", "FT-710"}, capsFor)
	want := []string{"IC-7610", "ID-52"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("icomModels = %v, want %v — a fake Icom row named %q must be covered whatever its name looks like", got, want, "ID-52")
	}
}

// TestTierRecordShapes_IcomModelsMatchesRegistryRowCountAndNames pins the
// PROXY's live behaviour (fix round 1, F6) against the real registry: it
// is not enough that icomModels selects on a typed capability rather than
// a string prefix — since nothing enforces "Icom ⇔ CTCSSToneRange"
// structurally, this asserts the exact row count and names the proxy
// selects today, so a driver quietly declaring the wrong tone shape (or a
// new Yaesu entry that happened to satisfy the proxy) is caught by name
// rather than passing silently because the COUNT still matched.
func TestTierRecordShapes_IcomModelsMatchesRegistryRowCountAndNames(t *testing.T) {
	got := icomModels(t, wiring.SupportedModels(), wiring.StaticCapabilities)
	want := []string{
		"IC-705", "IC-7100", "IC-7300", "IC-7300MK2", "IC-7610", "IC-7760",
		"IC-7850", "IC-7851", "IC-905", "IC-9700", "IC-R8600",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("icomModels(wiring.SupportedModels()) = %v (%d rows),\nwant %v (%d rows) — the registered Yaesu/Icom split has changed; update this list deliberately if it is a real registration, not a proxy regression", got, len(got), want, len(want))
	}
}

// TestTierRecordShapes_EveryModelDeclaresExactlyOneToneShape guards the
// one failure mode icomModels' proxy has (fix round 1, F6): a registered
// model declaring NEITHER CTCSSTones nor CTCSSToneRange would satisfy
// neither the Icom test (CTCSSToneRange != nil) nor look like a Yaesu row
// either — it would simply vanish from `registered` in
// TestTierRecordShapes_DistinctOrDeclared, with nothing to say so.
// spec.Capabilities.AdmitsTone's own doc comment states this is possible
// ("fails closed when a radio declares neither"); this test makes it an
// error for a REGISTERED row rather than a silently tolerated one.
// spec.Validate already refuses BOTH being declared at once
// (core/spec/validate.go:588), so only the "neither" half needs pinning
// here.
func TestTierRecordShapes_EveryModelDeclaresExactlyOneToneShape(t *testing.T) {
	for _, model := range wiring.SupportedModels() {
		caps, err := wiring.StaticCapabilities(model)
		if err != nil {
			t.Fatalf("StaticCapabilities(%q): %v", model, err)
		}
		hasList := len(caps.CTCSSTones) > 0
		hasRange := caps.CTCSSToneRange != nil
		if hasList == hasRange {
			t.Errorf("%s declares CTCSSTones (non-empty: %v) and CTCSSToneRange (non-nil: %v) — want exactly one; a model declaring neither would silently drop out of icomModels' proxy and vanish from the registration-coverage guard", model, hasList, hasRange)
		}
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
// distinguishable must REFUSE a pair the ten real profiles never
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

// cloneFamily is spec D1.1's "7610-shape clone" set — the THREE profiles
// the additions spec expected to be byte-identical to core/civ/ic7610's
// layout, in tierProfilePopulation order.
//
// It is a SEPARATE list from tierProfilePopulation rather than a slice of
// it: this set is a claim about record GEOMETRY, and a future family that
// joined the population without being a 7610 clone must not silently be
// dragged into the comparison below.
var cloneFamily = []civ.Profile{ic7610.Profile(), ic7851.Profile(), ic7760.Profile()}

// spanKey is one field span reduced to the properties spec D1.1's
// "byte-identical layout" claim is about: which field, where, how wide,
// which half-byte, and how the bytes carry the value.
type spanKey struct {
	field    civ.FieldID
	offset   int
	length   int
	nibble   civ.NibbleSel
	encoding civ.EncodingKind
	order    civ.ByteOrder
	scale    uint64
}

func keySpan(f civ.FieldSpan) spanKey {
	return spanKey{field: f.Field, offset: f.Offset, length: f.Length, nibble: f.Nibble, encoding: f.Encoding, order: f.Order, scale: f.Scale}
}

func (k spanKey) String() string {
	return fmt.Sprintf("offset %d length %d nibble %v encoding %v order %v scale %d", k.offset, k.length, k.nibble, k.encoding, k.order, k.scale)
}

// declaredCloneDivergences names every place spec D1.1's byte-identical
// expectation does NOT hold across cloneFamily, with the reason. The key
// is the exact rendering TestTierRecordShapes_7610CloneFamily produces;
// an entry with an empty value is not a ruling.
//
// EACH OF THE THREE IS THE SAME DECISION, TAKEN ONCE PER PRINTED PAD
// CELL. The IC-7851's document draws a literal "0" in both halves of the
// fifth frequency cell and of the first cell of each repeater-tone
// triple, and core/civ/ic7851/profile.go EXCLUDES those three bytes from
// their spans so the layout's Fixed template owns them — which is what
// makes a radio answering a digit there fail the READ instead of being
// re-encoded with the byte quietly zeroed (that file's own comment, and
// register entries ic7851-fixed-nibble-reencode and
// ic7851-tone-fixed-byte). The IC-7610 and the IC-7760 draw the SAME
// printed pads and INCLUDE them in their spans, bounding the value at the
// capability layer instead (core/civ/ic7610/profile.go's layout table;
// core/driver/ic7760/caps.go's MaxEncodableFreqHz = 69,999,999 and
// MaxToneDeciHz = 2999).
//
// IT IS A FINDING, WHICH IS EXACTLY WHAT SPEC D1.1 ASKED FOR — "the
// profiles are EXPECTED to be byte-identical ... a tier-level test PINS
// that expectation ... so a future divergence is a finding, not a silent
// drift". The divergence is therefore recorded here rather than
// papered over or resolved by editing a frozen, evidence-bound profile,
// which no registration may do. THE COST IS BOUNDED AND ONE-SIDED: the
// three radios still accept the same 25-byte record over the same two-byte
// flat address, so nothing about the tier fingerprint or the
// `indistinguishable` rulings above depends on this; what differs is which
// layer refuses an out-of-range value, and the IC-7851's arrangement is
// the stricter of the two.
//
// A NEW divergence — a span the three disagree on that is not listed here
// — FAILS the test, which is the whole mechanism.
var declaredCloneDivergences = map[string]string{
	"rx_frequency: IC-7610 has offset 1 length 5 nibble NibbleWhole encoding EncodingBCDNumber order OrderLittleEndian scale 1; IC-7851 has offset 1 length 4 nibble NibbleWhole encoding EncodingBCDNumber order OrderLittleEndian scale 1": "core/civ/ic7851/profile.go excludes the printed fifth frequency cell — drawn \"0 : 0\" with rotated \"1000 MHz digit: 0 (Fixed)\" and \"100 MHz digit: 0 (Fixed)\" leaders — from the span, so the layout's Fixed template owns that byte and a radio answering a digit in it fails the read (matrix §3.16.3, register entry ic7851-fixed-nibble-reencode). The IC-7610 and IC-7760 include the same printed cell and bound the value at the capability layer instead.",
	"tone_tx: IC-7610 has offset 9 length 3 nibble NibbleWhole encoding EncodingBCDNumber order OrderBigEndian scale 1; IC-7851 has offset 10 length 2 nibble NibbleWhole encoding EncodingBCDNumber order OrderBigEndian scale 1":           "the same decision at the repeater-tone triple's FIRST cell, drawn with two \"Fixed digit: 0*\" leaders (matrix §3.16.4, register entry ic7851-tone-fixed-byte): core/civ/ic7851/profile.go excludes it, so the span starts one byte later and is one byte narrower. The two remaining bytes still carry the whole printed 000.0–999.9 Hz domain.",
	"tone_rx: IC-7610 has offset 12 length 3 nibble NibbleWhole encoding EncodingBCDNumber order OrderBigEndian scale 1; IC-7851 has offset 13 length 2 nibble NibbleWhole encoding EncodingBCDNumber order OrderBigEndian scale 1":          "the TSQL triple's first cell, the same ruling as tone_tx immediately above and drawn identically in the same diagram.",
}

// TestTierRecordShapes_7610CloneFamily is additions spec D1.1's tier-level
// pin, DEFERRED by the IC-7851/IC-7850 registration (69e3a5b) because the
// spec defines it over THREE layouts and only two families were merged
// then — a two-way comparison would have had to be rewritten when the
// third landed. This is that registration.
//
// WHAT IT ASSERTS, in four parts:
//
//  1. THE SHARED SHAPE, on all three: one layout, 25 bytes record-only, a
//     flat two-byte address MEASURED off two frames each profile builds,
//     ten-byte names padded with 0x20, a single-length discriminator, a
//     25-byte all-zero Fixed template, and the same seven mapped fields in
//     the same order. This is the half of D1.1 that holds without
//     qualification, and it is the half the tier fingerprint and the
//     `indistinguishable` table above actually rest on.
//  2. THE IC-7610 AND THE IC-7760 ARE BYTE-IDENTICAL, layout for layout,
//     enums included — spec D1.1's expectation MET, between two profiles
//     built from two different documents by two different evidence-leg
//     sets. Asserted as exact equality, so a one-byte drift in either is a
//     failure.
//  3. THE IC-7851 DIVERGES IN EXACTLY THREE SPANS, each declared in
//     declaredCloneDivergences with its citation. Every divergence found
//     must be declared, and every declaration must still be a real
//     divergence — both directions, so this table cannot rot into a
//     blanket exemption any more than `indistinguishable` can.
//  4. THE NAME CHARSETS HOLD THE SAME 95 BYTES, in different printed
//     ORDER. Order is not a wire property — core/civ builds a byte
//     membership table from it — so the set is what a "same record"
//     claim can be about, and the difference is logged rather than
//     asserted away.
//
// EVERY FIGURE IS READ OFF A PROFILE. Nothing here restates a per-model
// package's own table back at it, for measureShape's reason.
func TestTierRecordShapes_7610CloneFamily(t *testing.T) {
	if len(cloneFamily) != 3 {
		t.Fatalf("cloneFamily has %d members, want the 3 additions spec D1.1 names — {IC-7610, IC-7851/7850, IC-7760}", len(cloneFamily))
	}

	// Part 1 — the shared shape.
	var layouts []civ.RecordLayout
	var fieldOrder []civ.FieldID
	for i, p := range cloneFamily {
		shape := measureShape(t, p)
		if shape.addressBytes != 2 {
			t.Errorf("%s: measured address width %d, want 2 — spec D1.1's set is defined at a two-byte FLAT address", p.Model(), shape.addressBytes)
		}
		if got := p.AddressForm(); got != civ.AddressFormFlat {
			t.Errorf("%s: AddressForm() = %v, want AddressFormFlat", p.Model(), got)
		}
		if !equalLengths(shape.recordLengths, []int{25}) {
			t.Errorf("%s: RecordLengths() = %v, want [25] — the record-only figure spec D1.1 names, under the tier's Erratum 1 convention", p.Model(), shape.recordLengths)
		}
		if got := p.BuildRecordLength(); got != 25 {
			t.Errorf("%s: BuildRecordLength() = %d, want 25", p.Model(), got)
		}
		if got := p.Discriminator(); got != civ.DiscriminatorSingleLength {
			t.Errorf("%s: Discriminator() = %v, want DiscriminatorSingleLength — one accepted length means one layout", p.Model(), got)
		}
		if got := p.NameLength(); got != 10 {
			t.Errorf("%s: NameLength() = %d, want 10", p.Model(), got)
		}
		if got := p.NamePad(); got != 0x20 {
			t.Errorf("%s: NamePad() = %#x, want 0x20", p.Model(), got)
		}
		ls := p.Layouts()
		if len(ls) != 1 {
			t.Fatalf("%s: %d layouts, want exactly 1 — a clone-family member with two record shapes is not the thing D1.1 describes", p.Model(), len(ls))
		}
		l := ls[0]
		if l.Length != 25 {
			t.Errorf("%s: layout Length = %d, want 25", p.Model(), l.Length)
		}
		if len(l.Fixed) != 25 {
			t.Errorf("%s: Fixed template is %d bytes, want 25 — a template that is not the record's width cannot be what E6 compares against", p.Model(), len(l.Fixed))
		}
		for off, b := range l.Fixed {
			if b != 0 {
				t.Errorf("%s: Fixed[%d] = %#x, want 0 — every unmapped region on all three of these records has an OFF value of zero", p.Model(), off, b)
				break
			}
		}
		order := make([]civ.FieldID, 0, len(l.Fields))
		for _, f := range l.Fields {
			order = append(order, f.Field)
		}
		if i == 0 {
			fieldOrder = order
		} else if !reflect.DeepEqual(order, fieldOrder) {
			t.Errorf("%s maps %v, but %s maps %v — the three records are claimed to carry the SAME seven fields in the same positions, and a difference in WHICH fields is a bigger finding than any span offset", p.Model(), order, cloneFamily[0].Model(), fieldOrder)
		}
		layouts = append(layouts, l)
	}
	if len(fieldOrder) != 7 {
		t.Errorf("the clone family maps %d fields (%v), want the 7 additions spec D1.1 lists for the 27-byte data area", len(fieldOrder), fieldOrder)
	}

	// Part 2 — the IC-7610 and the IC-7760, byte for byte.
	if !reflect.DeepEqual(layouts[0], layouts[2]) {
		t.Errorf("the %s's and the %s's layouts are NOT identical:\n  %s: %+v\n  %s: %+v\nadditions spec D1.1 expects these two to be byte-identical, and they were derived from two different documents by two different evidence-leg sets — a difference here is a finding in one of those derivations, not something to relax", cloneFamily[0].Model(), cloneFamily[2].Model(), cloneFamily[0].Model(), layouts[0], cloneFamily[2].Model(), layouts[2])
	} else {
		t.Logf("%s and %s: layouts byte-identical (spans, enums and Fixed template) — spec D1.1's expectation MET", cloneFamily[0].Model(), cloneFamily[2].Model())
	}

	// Part 3 — the IC-7851's declared divergences, both directions.
	found := map[string]bool{}
	base, other := layouts[0], layouts[1]
	for i := range base.Fields {
		if i >= len(other.Fields) {
			break
		}
		a, b := base.Fields[i], other.Fields[i]
		if a.Field != b.Field {
			continue // already reported by the field-order check above
		}
		if !reflect.DeepEqual(a.Enum, b.Enum) {
			t.Errorf("%s and %s disagree on the %v ENUM: %v against %v — an enum divergence is a wire-vocabulary difference and is not covered by declaredCloneDivergences, which rules only on span geometry", cloneFamily[0].Model(), cloneFamily[1].Model(), a.Field, a.Enum, b.Enum)
		}
		ka, kb := keySpan(a), keySpan(b)
		if ka == kb {
			continue
		}
		key := fmt.Sprintf("%v: %s has %s; %s has %s", a.Field, cloneFamily[0].Model(), ka, cloneFamily[1].Model(), kb)
		found[key] = true
		citation, declared := declaredCloneDivergences[key]
		switch {
		case !declared:
			t.Errorf("UNDECLARED divergence from the 7610 layout:\n  %s\nadditions spec D1.1 expects these layouts to be byte-identical; if this is a documented reading of that radio's own page rather than a transcription error, record it in declaredCloneDivergences with the citation", key)
		case citation == "":
			t.Errorf("declaredCloneDivergences[%q] has no documentation citation", key)
		default:
			t.Logf("DIVERGENCE (declared, not a regression) %s — %s", key, citation)
		}
	}
	for key := range declaredCloneDivergences {
		if !found[key] {
			t.Errorf("declaredCloneDivergences[%q] is stale: the three layouts no longer diverge there. If a profile was corrected, delete the entry; do not leave a ruling with no measurement under it", key)
		}
	}
	if len(found) == 0 {
		t.Log("no divergence found anywhere in the clone family — all three layouts are byte-identical, which is spec D1.1's expectation in full; declaredCloneDivergences should be empty, and the stale check above says whether it is")
	}

	// Part 4 — the name charsets: same members, different printed order.
	baseSet := charsetSet(cloneFamily[0].NameCharset())
	for _, p := range cloneFamily[1:] {
		set := charsetSet(p.NameCharset())
		if !reflect.DeepEqual(set, baseSet) {
			t.Errorf("%s's name charset holds different BYTES from the %s's — the printed order is a document's own choice and does not matter to core/civ, which builds a membership table from it, but the MEMBERS are a claim about which characters the radio will store", p.Model(), cloneFamily[0].Model())
		}
		if string(p.NameCharset()) != string(cloneFamily[0].NameCharset()) {
			t.Logf("%s and %s print the same %d charset bytes in a DIFFERENT ORDER — recorded, not a defect: core/civ consults membership only", cloneFamily[0].Model(), p.Model(), len(set))
		}
	}
}

// charsetSet reduces a charset to its membership, the only part of it
// core/civ's nameByteValid consults.
func charsetSet(b []byte) map[byte]bool {
	out := make(map[byte]bool, len(b))
	for _, x := range b {
		out[x] = true
	}
	return out
}
