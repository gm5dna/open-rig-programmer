// SPDX-License-Identifier: GPL-3.0-or-later

package radiotext_test

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestRadiotext_FT710Verbatim pins every FT-710 Text field against a
// literal copy of the string as it appears TODAY at its cited source
// (see radiotext.go's per-field doc comments for the exact origin of
// each): a byte-for-byte regression pin, not a re-derivation. If any of
// these sources' wording changes, this test and radiotext.go must be
// updated together, deliberately — never silently drift apart. Sources,
// confirmed byte-for-byte at task-37 time:
//   - EraseProcedure: cmd/rigprog/write.go's eraseFrontPanelProcedure
//     const, and the (identical, whitespace-normalised) prose in
//     DeleteConfirmDialog.svelte and SendFlowDialog.svelte.
//   - GridLegendNote: the first sentence of ChannelGrid.svelte's
//     grid-legend paragraph.
//   - ToneScanSkipVerification: the second sentence of ChannelGrid.svelte's
//     grid-legend paragraph (m42a: left behind in the frontend when task
//     41 captured the first sentence).
//   - PreservationTooltips: ChannelGrid.svelte's PRESERVED_TOOLTIP_TONE/
//     PRESERVED_TOOLTIP_SKIP consts.
//   - ProbeFirmwareNote: cmd/rigprog/probe.go's writeProbeReport.
func TestRadiotext_FT710Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].",
		GridLegendNote:           "Tone and Scan Skip aren't carried by the FT-710's CAT protocol — set them on the radio.",
		ToneScanSkipVerification: "Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip).",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not readable over CAT — preserved when writing (hardware-verified 13/07/2026)",
			ScanSkip: "not readable over CAT — preservation when writing is unverified (never probed)",
		},
		ProbeFirmwareNote: "Firmware version has no CAT query — check the front panel: memory CAT (read/write) requires firmware V01-10 or later.",
	}

	got, ok := radiotext.For("FT-710")
	if !ok {
		t.Fatal(`For("FT-710") ok = false, want true`)
	}
	if got != want {
		t.Errorf("For(\"FT-710\") = %#v,\nwant %#v", got, want)
	}
	assertNotBorrowedFromAnyOtherModel(t, "FT-710", got)
}

// TestRadiotext_FTdx10Verbatim is TestRadiotext_FT710Verbatim's sibling for
// the model M9c-6 registered, and it pins a DIFFERENT kind of fact. The
// FT-710's pin guards against drift from a cited source (a Svelte
// component, a CLI const) whose wording lives elsewhere. The FTdx10 has no
// such source: its prose was written in radiotext.go itself, for a radio
// this project has never connected to anything, under the honesty rule
// recorded at ftdx10Text — nothing invented, every absence stated as an
// absence.
//
// So what this test guards is the HEDGES. "No minimum firmware version is
// established", "no FTdx10 operating manual is held here", "has never been
// tested" are the load-bearing words: an editor tidying them into
// confident advisory copy — or copying the FT-710's V01-10 threshold and
// [V/M]/[ERASE] procedure across because the fields look empty-ish — would
// attribute one radio's evidence to another, which is the single failure
// mode this package's per-model keying exists to prevent. That edit fails
// here, deliberately loudly, and must be made in both places at once or
// not at all.
//
// ToneScanSkipVerification is asserted EMPTY, and that is not an omission:
// it is the only field the FTdx10 must not populate while
// core/driver/ftdx10's writeTrialsComplete is false, since any sentence in
// it would be a hardware-preservation claim. internal/wiring's
// TestEverySupportedModelHasRadiotext deliberately excludes this field from
// its non-blank requirement for exactly this radio (see its doc comment),
// so the two tests agree rather than contradict.
func TestRadiotext_FTdx10Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The FTdx10 has no CAT erase command, so a channel can only be deleted at the radio itself. This build does not describe how: no FTdx10 operating manual is held here, and inventing front-panel key presses would be worse than saying nothing — follow the memory-channel erase procedure in the radio's own operating manual.",
		GridLegendNote: "Tone and Scan Skip are not read or written for the FTdx10 by this build — its memory frame has no tone-number or scan-skip field (only a CTCSS on/off state byte, unverified on real hardware) — so set both on the radio.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not read or written over CAT by this build — whether a rewrite preserves it has never been tested",
			ScanSkip: "not read or written over CAT by this build — whether a rewrite preserves it has never been tested",
		},
		ProbeFirmwareNote: "Firmware version has no CAT query — check the front panel. No minimum version is established for the FTdx10: this build knows of none to require.",
	}

	got, ok := radiotext.For("FTdx10")
	if !ok {
		t.Fatal(`For("FTdx10") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"FTdx10\") = %#v,\nwant %#v", got, want)
	}

	// The FTdx10's prose must not have become the FT-710's by copy, or any
	// other registered model's.
	assertNotBorrowedFromAnyOtherModel(t, "FTdx10", got)
}

// textFields returns every Text field of t as a named map, so the
// non-borrowing and cross-model loops below iterate one list rather than
// four hand-maintained copies. ToneScanSkipVerification is included: it is
// empty for the FTdx101s today, and a loop that skipped it would stop
// noticing the day somebody populated it with a borrowed sentence.
func textFields(txt radiotext.Text) map[string]string {
	return map[string]string{
		"EraseProcedure":                txt.EraseProcedure,
		"GridLegendNote":                txt.GridLegendNote,
		"ToneScanSkipVerification":      txt.ToneScanSkipVerification,
		"PreservationTooltips.Tone":     txt.PreservationTooltips.Tone,
		"PreservationTooltips.ScanSkip": txt.PreservationTooltips.ScanSkip,
		"ProbeFirmwareNote":             txt.ProbeFirmwareNote,
	}
}

// countLeafFields recursively counts v's fields, flattening one level of
// any nested struct field (radiotext.Text's PreservationTooltips is the
// only such field today) rather than counting the struct itself as one
// field — the same flattening textFields performs by hand.
func countLeafFields(v reflect.Value) int {
	n := 0
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Struct {
			n += countLeafFields(f)
			continue
		}
		n++
	}
	return n
}

// TestTextFields_CoversEveryFieldOfText is the completeness pin renaming
// (minors-review.md F8, minors-fix1-rereview.md's carried item) asked
// for: without it, a field added to radiotext.Text escapes the
// byte-identity and non-borrowing loops below with no test noticing,
// since textFields is otherwise a hand-maintained map. This reads the
// struct's own shape via reflect rather than restating a field count, so
// the pin cannot drift from the type it is meant to cover.
func TestTextFields_CoversEveryFieldOfText(t *testing.T) {
	want := countLeafFields(reflect.ValueOf(radiotext.Text{}))
	got := len(textFields(radiotext.Text{}))
	if got != want {
		t.Fatalf("textFields returns %d entries, but radiotext.Text has %d leaf fields (nested structs flattened one level) — a field was added to or removed from Text without textFields being updated to match, so it would silently escape every non-borrowing and byte-identity loop in this file", got, want)
	}
}

// ---------------------------------------------------------------------
// The non-borrowing check, DERIVED rather than hand-copied per entry.
//
// Follow-up 9 (Tier 4b tier review): every entry below used to carry its
// OWN literal "every other registered model" list — both the byte-
// identity loop's slice of model names and the particulars slice checked
// by substring — frozen at whatever this project had registered on the
// day that entry was written. NOTHING EVER REVISITED an earlier entry's
// list when a later model registered, so IC-7610 through IC-905's checks
// never learned about IC-7760, IC-7851, IC-7850, IC-7100 or IC-R8600 (the
// five Tier 4b models), and even the Tier 4b entries only checked against
// whichever of THAT set existed on their own registration day — IC-7851/
// IC-7850's own lists stop at IC-905, IC-7760's stops at the 7851 pair,
// IC-7100's stops at IC-7760, and only IC-R8600's, the last of the five,
// happened to be complete by construction.
//
// assertNotBorrowedFromAnyOtherModel and particularsAgainstEveryOtherModel
// replace all of that with ONE mechanism, keyed off
// wiring.SupportedModels() — the registry itself — so EVERY LATER
// registration extends every existing entry's check on its next run, rather
// than needing this file edited once per existing entry. No count is written
// here, deliberately: a number in prose is one registration away from being
// false, and the mechanism this paragraph describes is what makes counting
// unnecessary.
// ---------------------------------------------------------------------

// yaesuModels is the SIX registered models whose radiotext entry predates
// (and is unrelated to) any CI-V vocabulary — the set catFamilyVocabulary
// below must NOT be checked against, since every one of these radios'
// prose legitimately says "CAT".
//
// The FT-991A (Tier 1) is the sixth, and it belongs here for exactly the
// reason the other five do and for no other: it speaks CAT, so its own
// prose says "CAT" throughout and the vocabulary check must skip it. It is
// otherwise an ordinary entry — one registered model, one driver package,
// one simulator, exactly as the FT-891 before it was.
var yaesuModels = map[string]bool{
	"FT-710": true, "FTdx10": true, "FTdx101D": true, "FTdx101MP": true,
	"FT-891": true, "FT-991A": true,
	// The FTdx5000 (v1.7.0 Kenwood/Yaesu wave, tenth row): it speaks CAT
	// too, so its own prose legitimately says "CAT" and the vocabulary
	// check must skip it, on the FT-891/FT-991A footing above.
	"FTdx5000": true,
}

// catFamilyVocabulary is the Yaesu CAT-protocol vocabulary every Icom
// entry's prose must never carry, on top of the other Yaesu models' own
// particulars (ownParticulars, below): these four tokens are not any ONE
// Yaesu model's evidence, they are the shared fact "this driver speaks
// CAT", which is true of all six Yaesu entries and false of every Icom
// one. Checked only when the model being checked is NOT itself Yaesu — a
// Yaesu entry's own prose legitimately contains "CAT" throughout.
var catFamilyVocabulary = []string{"CAT manual", "CAT command", "CAT query", "CAT"}

// sharedIC7851PairAddress is 8Eh, "the default address of IC-7850/
// IC-7851" (PDF p.229, folio 15-18) — the one particular that belongs to
// NEITHER radio exclusively. Every OTHER model's check must still refuse
// it (claiming that address would misattribute this pair's evidence), but
// checking it against the pair's OWN prose would fault on their own
// stated limitation, so it is added to particularsAgainstEveryOtherModel
// explicitly rather than living in either's ownParticulars entry.
const sharedIC7851PairAddress = "8Eh"

// skipByteIdenticalSibling is a small, explicit exception to the derived
// byte-identical loop below: the FTdx101D/FTdx101MP pair share one CAT
// manual and, deliberately, two fields whose text never names the model
// at all (PreservationTooltips.Tone, PreservationTooltips.ScanSkip — see
// wantFTdx101D/wantFTdx101MP). Their shared
// byte content is checked, more precisely, by
// TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName's substitution,
// which REQUIRES every non-naming field to already be equal; that is the
// mechanism this pair has always relied on, and the byte-identical loop
// (which cannot tell "shared by design" from "borrowed by accident") has
// never run the two against each other.
//
// THE IC-7851/IC-7850 PAIR CARRIES NO SUCH EXCEPTION: every one of their
// fields differs except where a substitution replaces the model name, so
// their own byte-identical check runs against the sibling exactly like
// any other model (see TestRadiotext_IC7851Verbatim's doc comment,
// "the sibling included, and for this pair the sibling is the most
// important of them").
var skipByteIdenticalSibling = map[string]string{
	"FTdx101D":  "FTdx101MP",
	"FTdx101MP": "FTdx101D",
}

// TestSkipByteIdenticalSibling_ExactlyThePair is the adjudicated pin on
// item 8's exception: the review approved skipByteIdenticalSibling on the
// strength that it names EXACTLY the FTdx101D/FTdx101MP pair and nothing
// else — a later maintainer silencing a real red by adding a third entry
// must fail this test rather than quietly widen the exception.
func TestSkipByteIdenticalSibling_ExactlyThePair(t *testing.T) {
	want := map[string]string{
		"FTdx101D":  "FTdx101MP",
		"FTdx101MP": "FTdx101D",
	}
	if len(skipByteIdenticalSibling) != len(want) {
		t.Fatalf("skipByteIdenticalSibling has %d entries, want exactly %d: %v", len(skipByteIdenticalSibling), len(want), skipByteIdenticalSibling)
	}
	for k, v := range want {
		if got := skipByteIdenticalSibling[k]; got != v {
			t.Errorf("skipByteIdenticalSibling[%q] = %q, want %q", k, got, v)
		}
	}
}

// ownParticulars is every registered model's own distinguishing evidence:
// its bare name and, for the Icom entries, its own fixed CI-V address hex
// — the tokens that identify THIS radio and no other, which must never
// appear in another model's prose. IC-7851's and IC-7850's entries omit
// 8Eh (see sharedIC7851PairAddress); FT-710's is exactly the five tokens
// TestRadiotext_FTdx10Verbatim always checked ("CAT" family tokens
// excluded — see catFamilyVocabulary).
var ownParticulars = map[string][]string{
	"FT-710":    {"FT-710", "V01-10", "[V/M]", "[ERASE]", "hardware-verified"},
	"FTdx10":    {"FTdx10"},
	"FTdx101D":  {"FTdx101D"},
	"FTdx101MP": {"FTdx101MP"},
	// The FT-891's own name, and nothing else. Its prose carries no
	// address hex (it is not a CI-V radio) and no hardware finding to
	// guard (no FT-891 has ever answered a frame), so the bare name is
	// the whole of this radio's distinguishing evidence — the three
	// FTdx entries' shape, not the FT-710's five-token one.
	"FT-891": {"FT-891"},
	// The FT-991A's own name, and nothing else, on the FT-891's terms
	// exactly: no address hex (it is not a CI-V radio) and no hardware
	// finding to guard (no FT-991A has ever answered a frame). Its
	// distinctive strings — "P-1L", "EX087;", "031 CAT RATE" — are facts
	// about this radio's MANUAL rather than particulars another entry
	// could plausibly borrow, and listing them here would guard nothing
	// the bare name does not.
	"FT-991A":    {"FT-991A"},
	"IC-7610":    {"IC-7610", "98h"},
	"IC-7300":    {"IC-7300", "94h"},
	"IC-7300MK2": {"IC-7300MK2", "B6h"},
	"IC-705":     {"IC-705", "A4h"},
	"IC-9700":    {"IC-9700", "A2h"},
	"IC-905":     {"IC-905", "ACh"},
	"IC-7851":    {"IC-7851"},
	"IC-7850":    {"IC-7850"},
	"IC-7760":    {"IC-7760", "B2h"},
	"IC-7100":    {"IC-7100", "88h"},
	"IC-R8600":   {"IC-R8600", "96h"},
	// The REGISTERED Kenwood rows (Tier 6). Each entry is the bare
	// name and nothing else: this family's prose carries no address hex
	// (it is not a CI-V family) and no finding from a real radio to
	// guard, so the name is the whole of each row's distinguishing
	// evidence — the three FTdx entries' shape, not the FT-710's
	// five-token one.
	//
	// "TS-590S" IS A STRICT PREFIX OF "TS-590SG", which no earlier pair
	// in this table is. stripOwnName's word-boundary match is what keeps
	// that honest in both directions: checking the SG's prose, "TS-590S"
	// is a particular of another model, and the SG's own name is stripped
	// by a pattern that does not fire inside it; checking the S's prose,
	// its own name is stripped by a pattern that does NOT match inside
	// "TS-590SG", so a genuine borrowing of the sibling's name is still
	// caught.
	//
	// NO "TS-480" ENTRY, deliberately: that row is not registered
	// (internal/wiring, plan decision P3), so
	// particularsAgainstEveryOtherModel — which ranges
	// wiring.SupportedModels() — never looks it up, and the panic that
	// would fire on a missing entry is exactly the loud failure edit 6 of
	// the registration commit's ten-edit list is meant to be.
	"TS-590S":  {"TS-590S"},
	"TS-590SG": {"TS-590SG"},
	// Tier 6's SECOND pair. Bare names again, for the 590 pair's reason —
	// no CI-V address hex and no hardware finding to guard — and neither
	// name is a prefix of anything else in this table.
	//
	// NEITHER ENTRY NAMES THE OTHER, and that is a discipline rather than
	// an accident of wording. Both rows' lockout flags are printed
	// differently ("0/1" against "1/2", erratum E8) and each entry says so
	// — but each says it about ITS OWN radio, describing the other as "the
	// other Kenwood radio in this model list" rather than naming it,
	// because a sentence naming the sibling would be this check's own
	// definition of a borrowed particular.
	"TS-890S": {"TS-890S"},
	"TS-990S": {"TS-990S"},
	// The v1.7.0 Icom wave. Own name and own CI-V address hex, on the
	// registered Icom rows' footing above.
	"IC-7800": {"IC-7800", "6Ah"},
	"IC-7600": {"IC-7600", "7Ah"},
	"IC-7410": {"IC-7410", "80h"},
	"IC-7700": {"IC-7700", "74h"},
	"IC-9100": {"IC-9100", "7Ch"},
	"IC-7200": {"IC-7200", "76h"},
	// v1.7.0 Kenwood/Yaesu wave, tenth row: bare name, no CI-V address (not
	// an Icom family) and no hardware finding to guard, on the FT-891/
	// FT-991A footing.
	"FTdx5000": {"FTdx5000"},
	// v1.7.0 Kenwood/Yaesu wave, first row: bare name, no address hex (not
	// a CI-V family) and no hardware finding to guard.
	"TS-2000": {"TS-2000"},
	// v1.7.0 Kenwood/Yaesu wave, second row: bare name. "TS-2000" IS a
	// strict prefix of "TS-2000X" (the TS-590S/TS-590SG shape), and
	// stripOwnName's word-boundary match is what keeps that honest: this
	// entry's own prose names only "TS-2000X" throughout (radiotext.go's
	// own doc comment), never the bare "TS-2000", so stripping "TS-2000X"
	// as a whole word leaves no "TS-2000" substring behind to trip the
	// borrowed-particular check.
	"TS-2000X": {"TS-2000X"},
	// v1.7.0 Kenwood/Yaesu wave, third and last ts2000 row: bare name, no
	// prefix-collision concern with any other registered model.
	"TS-B2000": {"TS-B2000"},
	// v1.7.0 Kenwood/Yaesu wave, fourth row: bare name.
	"TS-570D": {"TS-570D"},
	// v1.7.0 Kenwood/Yaesu wave, fifth row: bare name.
	"TS-570S": {"TS-570S"},
}

// particularsAgainstEveryOtherModel returns every particular model's own
// prose must not contain: every OTHER registered model's ownParticulars,
// unioned; the shared IC-7851/IC-7850 address, unless model IS one of
// that pair; and the Yaesu CAT vocabulary, if model is itself an Icom
// entry.
//
// DERIVED FROM wiring.SupportedModels(), not a hand-copied slice: a model
// registered in internal/wiring but missing here would panic on the
// map-miss below, which is deliberate — radiotext.For already refuses to
// serve a registered model with no prose (internal/wiring's
// TestEverySupportedModelHasRadiotext), and this table must stay in the
// same lockstep.
func particularsAgainstEveryOtherModel(model string) []string {
	var out []string
	if !yaesuModels[model] {
		out = append(out, catFamilyVocabulary...)
	}
	for _, other := range wiring.SupportedModels() {
		if other == model {
			continue
		}
		own, ok := ownParticulars[other]
		if !ok {
			panic(fmt.Sprintf("radiotext_test: %q is registered but ownParticulars carries no entry for it", other))
		}
		out = append(out, own...)
	}
	if model != "IC-7851" && model != "IC-7850" {
		out = append(out, sharedIC7851PairAddress)
	}
	return out
}

// assertNotBorrowedFromAnyOtherModel is the ONE non-borrowing check every
// entry's test below calls. It replaces each entry's former hand-
// maintained "every other model, as of this model's own registration"
// list — see this section's header comment for why that mattered.
//
// BOTH CHECKS RUN, AND THEY CATCH DIFFERENT MISTAKES, exactly as the
// per-entry versions this replaces always argued: the byte-identity loop
// catches a wholesale copy (a field filled by pasting a neighbour's), and
// the particulars loop catches a partial one (a sentence reworded but
// still carrying "94h" or "IC-7300" inside it), which byte-identity
// alone would sail past.
//
// model's OWN NAME is stripped from each field before the particulars
// scan (the FTdx101 pair's existing technique, generalised): every
// registered model name is checked as a particular of every OTHER model,
// and a model whose own name happens to be a substring of a longer
// registered name (IC-7300 inside IC-7300MK2) would otherwise fault on
// its own self-references.
func assertNotBorrowedFromAnyOtherModel(t *testing.T, model string, got radiotext.Text) {
	t.Helper()
	for _, other := range wiring.SupportedModels() {
		if other == model || skipByteIdenticalSibling[model] == other {
			continue
		}
		otherText, ok := radiotext.For(other)
		if !ok {
			t.Fatalf("For(%q) ok = false, want true — %q is registered in internal/wiring but radiotext carries no prose for it", other, other)
		}
		otherFields := textFields(otherText)
		for field, val := range textFields(got) {
			if val == "" {
				// Shared emptiness (ToneScanSkipVerification, on every
				// entry whose write-trial guard is false) is not a copy.
				continue
			}
			if val == otherFields[field] {
				t.Errorf("%s %s is byte-identical to the %s's — one radio's prose must never be served as another's", model, field, other)
			}
		}
	}
	particulars := particularsAgainstEveryOtherModel(model)
	for field, val := range textFields(got) {
		bare := stripOwnName(val, model)
		for _, particular := range particulars {
			if strings.Contains(bare, particular) {
				t.Errorf("%s %s contains %q — another radio's particular in this one's prose is that radio's evidence claimed for this one", model, field, particular)
			}
		}
	}
}

// stripOwnName removes model's own self-references from val before the
// particulars scan, WITHOUT masking a longer sibling name that happens to
// have model as a PREFIX ("IC-7300" is a prefix of "IC-7300MK2"; "FTdx10"
// is a prefix of "FTdx101D"/"FTdx101MP").
//
// F5 (fix round 1): a plain strings.ReplaceAll(val, model, "") strips
// EVERY occurrence of the substring model, including the "IC-7300" inside
// "IC-7300MK2" — so a genuine borrowing of the sibling's name would be
// silently reduced to "MK2" and the particulars scan below would never
// see "IC-7300MK2" to match against. A regexp word-boundary match on
// model does not have that problem: Go's \b fires only at a transition
// between a word character ([0-9A-Za-z_]) and a non-word one, and both
// the last character of "IC-7300" (a digit) and the first character of
// "MK2" (a letter) are word characters, so there is NO boundary between
// them — the pattern does not match inside "IC-7300MK2" at all, leaving
// it intact for the particulars scan. An ordinary self-reference like
// "IC-7300's" DOES have a boundary (the apostrophe is not a word
// character), so it is still stripped exactly as before.
func stripOwnName(val, model string) string {
	return regexp.MustCompile(`\b`+regexp.QuoteMeta(model)+`\b`).ReplaceAllString(val, "")
}

// TestStripOwnName_DoesNotMaskAPrefixSibling pins F5's fix directly: the
// IC-7300/IC-7300MK2 case a plain strings.ReplaceAll used to mangle.
func TestStripOwnName_DoesNotMaskAPrefixSibling(t *testing.T) {
	for _, tc := range []struct {
		name  string
		val   string
		model string
		want  string
	}{
		{
			"self-reference is stripped",
			"The IC-7300's own display shows the version.",
			"IC-7300",
			"The 's own display shows the version.",
		},
		{
			"a genuine sibling borrowing survives the strip",
			"borrowed sibling text IC-7300MK2 mention",
			"IC-7300",
			"borrowed sibling text IC-7300MK2 mention",
		},
		{
			"the FTdx10/FTdx101D pair has the identical hazard",
			"The FTdx10 has no CAT erase command, borrowed from FTdx101D",
			"FTdx10",
			"The  has no CAT erase command, borrowed from FTdx101D",
		},
		{
			"the longer sibling's own self-reference still strips in full",
			"The IC-7300MK2's own display shows the version.",
			"IC-7300MK2",
			"The 's own display shows the version.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripOwnName(tc.val, tc.model); got != tc.want {
				t.Errorf("stripOwnName(%q, %q) = %q, want %q", tc.val, tc.model, got, tc.want)
			}
		})
	}
}

// wantFTdx101D is the FTDX101D's entry, pinned VERBATIM, and it is shared
// by the verbatim test and by the D-vs-MP substitution test so that neither
// can pass against a stale copy of the other's expectation.
var wantFTdx101D = radiotext.Text{
	EraseProcedure: "The FTdx101D's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101D's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	GridLegendNote: "Tone and Scan Skip are neither read nor written for the FTdx101D by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101D has ever been asked to confirm. Set both at the radio.",
	// Deliberately empty — see TestRadiotext_FTdx101DVerbatim's doc comment.
	ToneScanSkipVerification: "",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
		ScanSkip: "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
	},
	ProbeFirmwareNote: "Firmware version has no CAT query on the FTdx101D, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.",
}

// wantFTdx101MP is the FTDX101MP's entry, pinned VERBATIM. It is written
// out in full rather than derived from wantFTdx101D by substitution,
// deliberately: deriving it would make the verbatim pin and the D8
// substitution pin the SAME assertion, and the substitution test would then
// prove only that strings.ReplaceAll works.
var wantFTdx101MP = radiotext.Text{
	EraseProcedure:           "The FTdx101MP's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101MP's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	GridLegendNote:           "Tone and Scan Skip are neither read nor written for the FTdx101MP by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101MP has ever been asked to confirm. Set both at the radio.",
	ToneScanSkipVerification: "",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
		ScanSkip: "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
	},
	ProbeFirmwareNote: "Firmware version has no CAT query on the FTdx101MP, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.",
}

// TestRadiotext_FTdx101DVerbatim is TestRadiotext_FTdx10Verbatim's sibling
// for the first of the two models M9d-2 registered, and it guards the same
// kind of fact: the HEDGES. This prose was written in radiotext.go itself,
// for a radio this project has never connected to anything, under the
// honesty rule recorded at ftdx101dText.
//
// "No minimum firmware version is established", "the FTdx101D's operating
// manual is not held here", "no trial has established" are the load-bearing
// words. An editor tidying them into confident advisory copy — or reaching
// for the FT-710's V01-10 threshold and [V/M]/[ERASE] procedure because the
// fields look thin — would attribute one radio's evidence to another. That
// edit fails here.
//
// TWO SENTENCES ARE POSITIVE CLAIMS rather than hedges, and both are
// manual-evidenced rather than assumed: that the CAT command set contains
// no erase command, and that it contains no firmware-version query. Both
// rest on the command availability table at layout 236-337 being this
// radio's complete command set (matrix §2.3), which is the project's
// recorded reading of that table. They are the two places this entry says
// more than the FTdx10's can, and they are cited at ftdx101dText.
//
// ToneScanSkipVerification is asserted EMPTY, and that is not an omission:
// it is the only field this model must not populate while
// core/driver/ftdx101's writeTrialsCompleteD is false, since any sentence
// in it would be a hardware-preservation claim. internal/wiring's
// TestEverySupportedModelHasRadiotext deliberately excludes this field from
// its non-blank requirement, so the two tests agree rather than contradict.
func TestRadiotext_FTdx101DVerbatim(t *testing.T) {
	got, ok := radiotext.For("FTdx101D")
	if !ok {
		t.Fatal(`For("FTdx101D") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != wantFTdx101D {
		t.Errorf("For(\"FTdx101D\") = %#v,\nwant %#v", got, wantFTdx101D)
	}
	assertNotBorrowedFromAnyOtherModel(t, "FTdx101D", got)
}

// TestRadiotext_FTdx101MPVerbatim is the same pin for the MP. It is a
// SEPARATE test rather than a subtest of the D's because the two entries
// are separate claims about separate radios: a capture from an FTDX101D
// lifts nothing for the MP, and the day one of these entries changes it
// must be visible which radio's prose moved.
func TestRadiotext_FTdx101MPVerbatim(t *testing.T) {
	got, ok := radiotext.For("FTdx101MP")
	if !ok {
		t.Fatal(`For("FTdx101MP") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != wantFTdx101MP {
		t.Errorf("For(\"FTdx101MP\") = %#v,\nwant %#v", got, wantFTdx101MP)
	}
	assertNotBorrowedFromAnyOtherModel(t, "FTdx101MP", got)
}

// TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName is plan D8, stated as
// a SUBSTITUTION: replacing every occurrence of "FTdx101MP" with "FTdx101D"
// throughout the MP's entry must reproduce the D's entry byte for byte.
//
// Why substitution rather than a field-by-field comparison of the fields
// that happen not to name the model: the interesting failure is not "two
// fields drifted apart", it is "somebody added a sentence to ONE model's
// entry" — a claim about the MP that no evidence distinguishes from the D,
// or vice versa. A comparison restricted to the model-naming fields cannot
// see that; this can, because the added sentence survives the substitution
// and breaks the equality.
//
// The direction is MP -> D and not the reverse, and that is forced:
// "FTdx101D" is a substring of nothing, but substituting D's name INTO the
// MP's would leave "FTdx101D" wherever the MP's name appeared and the two
// would never meet. One direction is well-defined; the other is not.
//
// NON-VACUITY: at least one field must actually name the model, or the
// substitution would be the identity function and this test would prove
// that the two entries are equal — which they are not, and must not be.
func TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName(t *testing.T) {
	d, ok := radiotext.For("FTdx101D")
	if !ok {
		t.Fatal(`For("FTdx101D") ok = false, want true`)
	}
	mp, ok := radiotext.For("FTdx101MP")
	if !ok {
		t.Fatal(`For("FTdx101MP") ok = false, want true`)
	}

	dFields := textFields(d)
	naming := 0
	for field, mpVal := range textFields(mp) {
		if strings.Contains(mpVal, "FTdx101MP") {
			naming++
		}
		if got := strings.ReplaceAll(mpVal, "FTdx101MP", "FTdx101D"); got != dFields[field] {
			t.Errorf("%s: the MP's text with its model name replaced by the D's is\n  %q\nbut the D's is\n  %q\n— D8: the two entries may differ ONLY where they name the model", field, got, dFields[field])
		}
	}
	if naming == 0 {
		t.Error("no MP field names the model — the substitution above is the identity function and this test asserted nothing")
	}

	// And the two entries are NOT equal: they name different radios, and a
	// user reading the MP's advisories must see the MP's name.
	if d == mp {
		t.Error("the FTdx101D's and FTdx101MP's entries are byte-identical — each radio's prose must name its own model")
	}
}

// TestRadiotext_IC7610Verbatim is TestRadiotext_FTdx10Verbatim's sibling
// for Wave 4 task R1's registration — this project's FIRST non-Yaesu
// model — and it guards the same kind of fact: the HEDGES. This prose was
// written in radiotext.go itself, for a radio this project has never
// connected to anything, under the honesty rule recorded at ic7610Text.
//
// "No minimum firmware version is established", "no IC-7610 operating
// manual is held here", "unverified against real hardware" are the
// load-bearing words. An editor tidying them into confident advisory
// copy — or reaching for any Yaesu radio's wording because the fields look
// thin, or because CI-V "looks like" CAT — would attribute one radio's
// (or one MANUFACTURER's) evidence to another. That edit fails here.
//
// ToneScanSkipVerification is asserted EMPTY for the same reason every
// other model's is: core/driver/ic7610's writeTrialsComplete is false, so
// there is no hardware-preservation verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST ALL SIX YAESU ENTRIES, not just
// the FT-710 and the FTdx10 the way the FTdx101 pair's does: this is the
// first model with no Yaesu sibling to be careful about specifically, so
// every prior entry is a borrowing risk, not just the two nearest ones.
// textFields (above) is reused unchanged — it is generic over any
// radiotext.Text value, not FTdx101-specific despite its name.
func TestRadiotext_IC7610Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-7610's CI-V protocol has an erase command form, but this build never sends it: no IC-7610 has ever confirmed what it does, and sending an unconfirmed erase command risks clearing the wrong channel. This build does not describe a front-panel procedure either — no IC-7610 operating manual is held here — so follow the memory-channel clear procedure in the radio's own operating manual.",
		GridLegendNote: "Tone is read and written for the IC-7610 over CI-V by this build, but unverified against real hardware — no IC-7610 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7610 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — this radio's nearest wire nibble is a select-group marker, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7610: this build knows of none to require. This driver talks only to CI-V address 98h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is itself ASSUMED, not read off the radio, since the reference guide names six rates and marks no default. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7610")
	if !ok {
		t.Fatal(`For("IC-7610") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7610\") = %#v,\nwant %#v", got, want)
	}

	// Non-borrowing, against every other registered model: no field may be
	// byte-identical to, or carry a particular of, any of them. R1 review
	// (fix round 1) confirmed that nothing legitimate in ic7610Text
	// contains the bare "CAT" token, which is why catFamilyVocabulary
	// checks it standalone rather than only in two-word forms.
	assertNotBorrowedFromAnyOtherModel(t, "IC-7610", got)
}

// TestRadiotext_IC7300Verbatim is TestRadiotext_IC7610Verbatim's sibling
// for Wave 4 task R3's registration — this project's SECOND Icom
// registration and FIRST Icom pair — and it guards the same kind of fact:
// the HEDGES. This prose was written in radiotext.go itself, for a radio
// this project has never connected to anything, under the honesty rule
// recorded at ic7300Text.
//
// "No minimum firmware version is established", "unverified against real
// hardware" are the load-bearing words, on the same footing as the
// IC-7610's own test. ToneScanSkipVerification is asserted EMPTY for the
// same reason every other model's is: core/driver/ic7300's
// writeTrialsComplete is false, so there is no hardware-preservation
// verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED MODEL,
// including its own IC-7300MK2 sibling — because core/driver/ic7300/
// doc.go's own package comment states the two Icom documents this pair is
// built from are mutually silent about each other, so the sibling is
// exactly as much a borrowing risk as any other radio's prose.
// assertNotBorrowedFromAnyOtherModel dodges the one hazard the sibling
// check runs into — "IC-7300" is a byte-for-byte PREFIX of "IC-7300MK2",
// so checking IC-7300MK2's OWN prose for the bare substring "IC-7300"
// would fault on its own self-references ("The IC-7300MK2's...") — by
// stripping the checked model's own name from each field before scanning
// particulars, so "IC-7300MK2" leaves no residual "IC-7300" behind.
// textFields is reused unchanged — it is generic over any
// radiotext.Text value.
func TestRadiotext_IC7300Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-7300's CI-V protocol prints two erase command forms — a 1A 00 set with a SELECT byte of FF, and a separate command 0B — but this build sends neither: no IC-7300 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure printed in the IC-7300's own full operating manual.",
		GridLegendNote: "Tone is read and written for the IC-7300 over CI-V by this build, but unverified against real hardware — no IC-7300 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7300 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7300's nearest wire nibble is a select-group marker, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7300: this build knows of none to require. This driver talks only to CI-V address 94h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is a CHOICE — the highest rate this radio's document lists on both its [USB] and [REMOTE] ports — not a value read off the radio. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7300")
	if !ok {
		t.Fatal(`For("IC-7300") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7300\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7300", got)
}

// TestRadiotext_IC7300MK2Verbatim is TestRadiotext_IC7300Verbatim's
// sibling, registered in the same commit. See ic7300mk2Text's own doc
// comment for why this entry's non-borrowing obligation runs the OPPOSITE
// direction from the FTdx101D/MP pair's: that pair shares one manual and
// is PROVEN near-identical by substitution
// (TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName); this pair's two
// manuals are mutually silent about each other, so distinctness — not
// near-identity — is what must be proven, and the non-borrowing check
// below (run from BOTH this test and TestRadiotext_IC7300Verbatim, on the
// same two-tests-cover-the-pair structure the FTdx101 pair uses) is that
// proof.
func TestRadiotext_IC7300MK2Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-7300MK2's CI-V protocol prints two erase command forms — a 1A 00 set with a truncated data area, and a separate command 0B, whose own printed row states that P1 and P2 cannot be cleared — but this build sends neither: no IC-7300MK2 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This build does not describe a front-panel procedure either — this document is a CI-V reference guide, not a full operating manual — so follow the memory-channel clear procedure in the radio's own operating manual.",
		GridLegendNote: "Tone is read and written for the IC-7300MK2 over CI-V by this build, but unverified against real hardware — no IC-7300MK2 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see TestRadiotext_IC7300Verbatim's doc
		// comment; the same reasoning applies to this model.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7300MK2 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7300MK2's nearest wire nibble is a select-group marker, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7300MK2: this build knows of none to require. This driver talks only to CI-V address B6h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is a conservative derivation from a wake-up-command table this document prints for an unrelated purpose — this reference guide names no baud list and no factory default at all. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7300MK2")
	if !ok {
		t.Fatal(`For("IC-7300MK2") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7300MK2\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7300MK2", got)
}

// TestRadiotext_IC705Verbatim is TestRadiotext_IC7610Verbatim's sibling for
// Wave 4 task R4's registration — this project's THIRD Icom registration,
// and its second LONE model since the IC-7610 — and it guards the same
// kind of fact: the HEDGES. This prose was written in radiotext.go itself,
// for a radio this project has never connected to anything, under the
// honesty rule recorded at ic705Text.
//
// "No minimum firmware version is established", "unverified against real
// hardware" are the load-bearing words, on the same footing as every
// other Icom entry's own test. ToneScanSkipVerification is asserted EMPTY
// for the same reason every other model's is: core/driver/ic705's
// writeTrialsComplete is false, so there is no hardware-preservation
// verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED MODEL: this
// radio has no sibling of its own, so every other registered model is
// exactly as much a borrowing risk as any other. textFields is reused
// unchanged — it is generic over any radiotext.Text value.
func TestRadiotext_IC705Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-705's CI-V protocol prints two erase command forms — a 1A 00 set carrying FF at the fifth data position, and a separate command 0B — but this build sends neither: no IC-705 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This project's own copy of the IC-705 Basic Manual is admitted for three unrelated values only, so it names no front-panel clear procedure — follow the memory-channel clear procedure in the radio's own full operating manual.",
		GridLegendNote: "Tone is read and written for the IC-705 over CI-V by this build, but unverified against real hardware — no IC-705 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three select-scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-705 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-705's nearest wire nibble marks select-scan group membership, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-705: this build knows of none to require. This driver talks only to CI-V address A4h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole six-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no baud information for the CI-V port at all, and the one related fact admitted from the Basic Manual is a negative: the microUSB CI-V port is baud-agnostic, which lowers the cost of a wrong guess without being evidence of one. Opening this radio also discovers its MEM bank's occupied slots by a BOUNDED walk — the first ten display groups, G01 through G10, each in full — not the whole 100-group by 100-channel space: the radio's own front panel fills groups from the bottom and its ASSUMED budget is 500 channels against 10,000 addresses, so a user whose memories sit above group ten needs the fuller walk, and nothing on this build's command line or in its window offers it (the driver's own WithFullInventoryWalk is a Go-level option no registered composition passes). A channel stored above group ten is simply not listed here, so its absence from the grid is not evidence that the radio's channel is empty; and a write to a slot the bounded walk never visited is refused rather than sent if the radio's own pre-write read finds a record already there. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-705")
	if !ok {
		t.Fatal(`For("IC-705") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-705\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-705", got)
}

// TestRadiotext_IC9700Verbatim is TestRadiotext_IC7610Verbatim's sibling
// for Wave 4 task R5's registration — this project's FOURTH Icom
// registration, and its second LONE model since the IC-705 — and it
// guards the same kind of fact: the HEDGES. This prose was written in
// radiotext.go itself, for a radio this project has never connected to
// anything, under the honesty rule recorded at ic9700Text.
//
// "No minimum firmware version is established", "unverified against real
// hardware" are the load-bearing words, on the same footing as every
// other Icom entry's own test. ToneScanSkipVerification is asserted EMPTY
// for the same reason every other model's is: core/driver/ic9700's
// writeTrialsComplete is false, so there is no hardware-preservation
// verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED MODEL: this
// radio has no sibling of its own, so every other registered model is
// exactly as much a borrowing risk as any other. textFields is reused
// unchanged — it is generic over any radiotext.Text value.
func TestRadiotext_IC9700Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-9700's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF at the address's data position — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
		GridLegendNote: "Tone is read and written for the IC-9700 over CI-V by this build, but unverified against real hardware — no IC-9700 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT-memory scan groups (★1/★2/★3), not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-9700 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-9700's nearest wire nibble marks one of three SELECT-memory scan groups, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-9700: this build knows of none to require. This driver talks only to CI-V address A2h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED — the middle of the six rates this document prints, and the rate Icom most commonly ships, not a value this document itself names as the default: it defers the factory setting to the radio's own instruction manual, which this project does not hold. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-9700")
	if !ok {
		t.Fatal(`For("IC-9700") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-9700\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-9700", got)
}

// TestRadiotext_IC905Verbatim is TestRadiotext_IC7610Verbatim's sibling
// for Wave 4 task R6's registration — this project's FIFTH Icom
// registration, and the TIER'S LAST — and it guards the same kind of
// fact: the HEDGES. This prose was written in radiotext.go itself, for a
// radio this project has never connected to anything, under the honesty
// rule recorded at ic905Text.
//
// "No minimum firmware version is established", "unverified against real
// hardware" are the load-bearing words, on the same footing as every
// other Icom entry's own test. ToneScanSkipVerification is asserted EMPTY
// for the same reason every other model's is: core/driver/ic905's
// writeTrialsComplete is false, so there is no hardware-preservation
// verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED MODEL: this
// radio has no sibling of its own, so every other registered model is
// exactly as much a borrowing risk as any other. textFields is reused
// unchanged — it is generic over any radiotext.Text value.
func TestRadiotext_IC905Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-905's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF after the group and channel bytes, for memory groups 00 00 ~ 00 99 only, the CALL group being excluded by the document's own words — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
		GridLegendNote: "Tone is read and written for the IC-905 over CI-V by this build, but unverified against real hardware — no IC-905 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT-memory scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. DTCS is mapped too, but its three digits are OCTAL: a code this build cannot read as three octal digits comes back Unknown rather than a number, and — because this codec has no preserve-by-cache — a channel whose DTCS code is Unknown cannot be written at all until it is corrected to a valid octal value.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-905 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-905's nearest wire nibble marks one of three SELECT-memory scan groups, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-905: this build knows of none to require. This driver talks only to CI-V address ACh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole five-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no rate figure anywhere, on any port's command-table page. Opening this radio also discovers its MEM bank's occupied slots by a BOUNDED walk — group 0 in full, then channel 00 of every other group, descending into the rest of a group only where its channel 00 answered — not the whole 100x100 space, and nothing on this build's command line or in its window widens it (the driver's own WithFullInventoryWalk is a Go-level option no registered composition passes): a channel stored outside that walk is simply not listed here, so its absence from the grid is not evidence that the radio's channel is empty. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-905")
	if !ok {
		t.Fatal(`For("IC-905") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-905\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-905", got)
}

// TestFor_UnknownModel: any model that is not EXACTLY one of this
// package's keys — "FT-710", "FTdx10" since M9c-6, and "FTdx101D"/
// "FTdx101MP" since M9d-2 — returns the zero Text and false. Callers must
// never mistake a zero Text for real advisory copy.
//
// The cases are near misses BY CONSTRUCTION, and the FTdx10's registration
// is what made that worth restating: "FT-DX10" is a plausible mis-spelling
// of a model that now really exists, and it must still miss, because the
// key is the exact string core/driver/ftdx10's Capabilities().Model
// returns. Case, punctuation and trailing whitespace are all significant —
// a lookup is a map lookup, not a fuzzy match.
//
// "FTDX101D" AND "FTDX101MP" ARE LIVE COLLISIONS, not hypothetical typos,
// and that is why they are here. Full capitals is how the radio's OWN CAT
// manual spells both models throughout — it is the spelling on every page a
// user would have open — so it is the single most likely thing for a person
// or a config file to carry. It must still miss. The project's registry key
// is "FTdx101D", and a fuzzy match that quietly resolved the manual's
// spelling would be a lookup that sometimes guessed; better a caller that
// fails loudly than one served the wrong radio's advisories. The same
// reasoning covers "FTdx101" (the FAMILY, which is not a model this
// project registers — there is no bare FTdx101 anywhere in this codebase,
// by design) and "FT-DX101D" (the hyphenated form other software uses).
func TestFor_UnknownModel(t *testing.T) {
	for _, model := range []string{
		"", "FT-DX10", "ft-710", "FT-710 ", "FTDX10", "ftdx10", "FTdx10 ",
		"FTDX101D", "FTDX101MP", "ftdx101d", "FTdx101", "FTdx101D ", "FT-DX101D",
		// IC-7610 near misses (Wave 4 task R1): "IC7610" (no hyphen, the
		// spelling other CI-V software commonly uses), "ic-7610"
		// (lowercase), a trailing- and a leading-space variant, and the
		// bare model number, which names nothing this registry keys on.
		"IC7610", "ic-7610", "IC-7610 ", " IC-7610", "7610",
		// IC-7300 and IC-7300MK2 near misses (Wave 4 task R3): each
		// model's no-hyphen spelling, a lowercase variant, a trailing-
		// and leading-space variant, and the bare model number — the same
		// five-shape set as the IC-7610's own near misses above. "IC7300"
		// is ALSO a prefix-collision risk against "IC-7300MK2" once the
		// hyphen is dropped from both, so both no-hyphen spellings are
		// listed explicitly rather than assumed distinct by construction.
		"IC7300", "ic-7300", "IC-7300 ", " IC-7300", "7300",
		"IC7300MK2", "ic-7300mk2", "IC-7300MK2 ", " IC-7300MK2", "IC-7300 MK2", "IC-7300-MK2",
		// IC-705 near misses (Wave 4 task R4): the same five-shape set as
		// the IC-7610's and IC-7300's own near misses above — no-hyphen
		// spelling, a lowercase variant, a trailing- and leading-space
		// variant, and the bare model number.
		"IC705", "ic-705", "IC-705 ", " IC-705", "705",
		// IC-9700 near misses (Wave 4 task R5): the same five-shape set as
		// the IC-7610's, IC-7300's and IC-705's own near misses above —
		// no-hyphen spelling, a lowercase variant, a trailing- and
		// leading-space variant, and the bare model number.
		"IC9700", "ic-9700", "IC-9700 ", " IC-9700", "9700",
		// IC-905 near misses (Wave 4 task R6, the tier's LAST
		// registration): the same five-shape set as the IC-7610's,
		// IC-7300's, IC-705's and IC-9700's own near misses above —
		// no-hyphen spelling, a lowercase variant, a trailing- and
		// leading-space variant, and the bare model number.
		"IC905", "ic-905", "IC-905 ", " IC-905", "905",
		// IC-7851 and IC-7850 near misses (Tier 4b): the same five-shape
		// set as every Icom model's above, for EACH row — no-hyphen
		// spelling, a lowercase variant, a trailing- and leading-space
		// variant, and the bare model number. Both rows are listed
		// explicitly rather than assumed distinct by construction: these
		// two names differ in ONE character, which is exactly the
		// circumstance in which a registry lookup that had been made
		// loose (a prefix match, say) would answer one row's prose to the
		// other row's typo.
		"IC7851", "ic-7851", "IC-7851 ", " IC-7851", "7851",
		"IC7850", "ic-7850", "IC-7850 ", " IC-7850", "7850",
		// IC-7760 near misses (Tier 4b's second registration): the same
		// five-shape set again.
		"IC7760", "ic-7760", "IC-7760 ", " IC-7760", "7760",
		// IC-7100 near misses (Tier 4b's third registration): the same
		// five-shape set once more.
		"IC7100", "ic-7100", "IC-7100 ", " IC-7100", "7100",
		// IC-R8600 near misses (Tier 4b's fourth and last registration):
		// the same five shapes, plus the one this model name invites that
		// no other does — dropping the R, which is the letter that says
		// "receiver".
		"ICR8600", "ic-r8600", "IC-R8600 ", " IC-R8600", "R8600", "IC-8600",
		// FT-891 near misses (Tier 1, the first Yaesu registration since
		// M9d-2): the no-hyphen spelling other Yaesu software commonly
		// uses, the lowercase slug this project's own ModelSlug produces
		// ("ft-891" — a real string in this build, which is exactly why it
		// must not resolve here), a trailing- and a leading-space variant,
		// the space-for-hyphen spelling Yaesu's own marketing uses, and
		// the bare model number.
		"FT891", "ft-891", "FT-891 ", " FT-891", "FT 891", "891",
		// FT-991A near misses (Tier 1's second registration): the same
		// five-shape set once more — the no-hyphen spelling, the lowercase
		// slug this project's own ModelSlug produces ("ft-991a", a real
		// string in this build, which is exactly why it must not resolve
		// here), a trailing- and a leading-space variant, the
		// space-for-hyphen spelling, and the bare model number.
		//
		// AND ONE THAT IS NOT A TYPO AT ALL. "FT-991" is a DIFFERENT REAL
		// RADIO — a shipping Yaesu product this project does not support —
		// and it is also a strict PREFIX of the key above it, which is the
		// circumstance in which a lookup that had been made loose would
		// serve one radio's prose to the other's owner. It must return
		// false for both reasons, and calling it a misspelling would be
		// untrue (plan-level decision: "FT-991" is never a registry key
		// and never a negative fixture outside this list).
		"FT991A", "ft-991a", "FT-991A ", " FT-991A", "FT 991A", "991A",
		"FT-991",
		// TS-590S and TS-590SG near misses (Tier 6, the first Kenwood
		// registration): the no-hyphen spellings, the lowercase slugs this
		// project's own ModelSlug produces ("ts-590s" and "ts-590sg" — real
		// strings in this build, which is exactly why they must not resolve
		// here), trailing- and leading-space variants, the space-for-hyphen
		// spelling, and the bare model number. Both rows are listed
		// explicitly rather than assumed distinct by construction: "TS-590S"
		// is a strict PREFIX of "TS-590SG", so a lookup that had been made
		// loose would answer the S's prose to an SG typo.
		"TS590S", "ts-590s", "TS-590S ", " TS-590S", "TS 590S", "590",
		"TS590SG", "ts-590sg", "TS-590SG ", " TS-590SG", "TS 590SG", "TS 590", "TS-590 ", "TS-590",
		// The TS-480's near misses — and NOT the registry spelling. See the
		// paragraph below for why "TS-480" itself must never join this list.
		"TS480", "ts-480", "TS-480 ", " TS-480", "TS 480", "480",
		// TS-890S and TS-990S near misses (Tier 6's second pair): the same
		// five-shape set as every row above — the no-hyphen spelling, the
		// lowercase slug this project's own ModelSlug produces ("ts-890s"
		// and "ts-990s", real strings in this build, which is exactly why
		// they must not resolve here), a trailing- and a leading-space
		// variant, the space-for-hyphen spelling, and the bare model number.
		// Both rows are listed explicitly rather than assumed distinct by
		// construction: the two names differ in ONE character, which is the
		// circumstance in which a lookup that had been made loose would
		// answer one radio's prose to the other's typo. The trailing S is
		// dropped in a sixth shape on each, because "TS-890" and "TS-990"
		// are what a person shortening the name would type and neither is a
		// radio this project registers.
		"TS890S", "ts-890s", "TS-890S ", " TS-890S", "TS 890S", "890", "TS-890",
		"TS990S", "ts-990s", "TS-990S ", " TS-990S", "TS 990S", "990", "TS-990",
	} {
		got, ok := radiotext.For(model)
		if ok {
			t.Errorf("For(%q) ok = true, want false", model)
		}
		if got != (radiotext.Text{}) {
			t.Errorf("For(%q) = %#v, want the zero Text", model, got)
		}
	}

	// THE ONE SPELLING THAT MUST NOT BE ON THE LIST ABOVE, asserted here
	// explicitly with its reason rather than left as a silent omission (Tier
	// 6, plan decision P17 / M4). "TS-480" is the REGISTRY spelling, and
	// this milestone lands its texts entry while deliberately NOT
	// registering the row, so For("TS-480") answers ok. The near-miss list
	// above carries "TS480" — which stays — and a later reader adding
	// "TS-480" beside it by reflex would be asserting the opposite of what
	// this package does.
	//
	// The assertion is stated in BOTH directions on purpose: it fails if the
	// entry is ever dropped from texts (the registration commit's edit 4
	// would then stop being a no-op), and it names the near-miss that must
	// keep missing, so the two facts cannot drift apart.
	if _, ok := radiotext.For("TS-480"); !ok {
		t.Error(`For("TS-480") ok = false, want true — this build carries the TS-480's prose although internal/wiring does not register the row (plan P17); dropping the entry would leave that prose unwritten and unreviewed until the registration commit`)
	}
	if _, ok := radiotext.For("TS480"); ok {
		t.Error(`For("TS480") ok = true, want false — the hyphenless spelling is a near miss and must keep missing, even though the hyphenated registry spelling now resolves`)
	}

	// THE TWO SPELLINGS TIER 6's SECOND PAIR MUST NOT PUT ON THE LIST ABOVE,
	// asserted here explicitly with the reason rather than left as a silent
	// omission — the sentinel sweep this registration ran names it as the one
	// place in this package a reflex edit could go wrong.
	//
	// "TS-890S" and "TS-990S" are REGISTRY spellings from this milestone on,
	// so For answers ok for both, and a later reader adding either beside
	// "TS890S" by reflex — alongside the hyphenless near misses just added —
	// would be asserting the opposite of what this package does. The
	// assertion is stated in BOTH directions on purpose, exactly as the
	// TS-480's above is: it fails if an entry is ever dropped from texts, and
	// it names the near miss that must keep missing, so the two facts cannot
	// drift apart.
	for _, model := range []string{"TS-890S", "TS-990S"} {
		if _, ok := radiotext.For(model); !ok {
			t.Errorf("For(%q) ok = false, want true — this row is registered in internal/wiring, so it must never be a near-miss fixture in the list above", model)
		}
	}
	for _, miss := range []string{"TS890S", "TS990S"} {
		if _, ok := radiotext.For(miss); ok {
			t.Errorf("For(%q) ok = true, want false — the hyphenless spelling is a near miss and must keep missing, even though the hyphenated registry spelling now resolves", miss)
		}
	}
}

// TestUnverifiedWriteWarningTemplate_CarriesItsFourElements pins the
// arming dialogue's body against the four things the consent spec
// requires it to say, and against its ONE substitution point.
//
// Substrings, not a verbatim whole-string copy, and deliberately so: this
// string has no prior home to be a byte-for-byte regression pin of (unlike
// every Text field above, which was copied from a live call site), and the
// requirement it has to meet is a requirement about MEANING — that a user
// reading it learns which radio is at stake, that this project has never
// written to one, that every write is read back and compared, and that a
// misinterpreted frame could still corrupt the targeted channel. Rewording
// is allowed; dropping one of the four is not.
//
// The %s count is pinned exactly because the app layer substitutes ONE
// value (the model name): a second verb would render as a stray
// "%!s(MISSING)" in front of a user being asked to authorise a write.
func TestUnverifiedWriteWarningTemplate_CarriesItsFourElements(t *testing.T) {
	tmpl := radiotext.UnverifiedWriteWarningTemplate

	if got := strings.Count(tmpl, "%"); got != 1 {
		t.Errorf("the template contains %d %% characters, want exactly 1 (the model-name substitution): %q", got, tmpl)
	}
	if got := strings.Count(tmpl, "%s"); got != 1 {
		t.Errorf("the template contains %d %%s verbs, want exactly 1: %q", got, tmpl)
	}

	// Element 1 (names the radio) is the substitution itself; the other
	// three are pinned by the phrase each turns on.
	for _, want := range []string{
		"never written to a real %s", // element 2: no hardware has ever seen this write
		"read back and compared",     // element 3: the one real mitigation
		"corrupt",                    // element 4: what a misinterpreted frame could do
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("the template does not contain %q — a required element is missing:\n%q", want, tmpl)
		}
	}

	// Element 1, proved by rendering: the model name has to land in the
	// text a user actually reads.
	rendered := fmt.Sprintf(tmpl, "FTdx10")
	if !strings.Contains(rendered, "FTdx10") {
		t.Errorf("rendered warning = %q, want it to name the model", rendered)
	}
	if strings.Contains(rendered, "%!") {
		t.Errorf("rendered warning = %q, want no formatting fault", rendered)
	}
}

// The IC-7851/IC-7850 pair's shared address, 8Eh, is handled by
// sharedIC7851PairAddress and particularsAgainstEveryOtherModel above: it
// is these two radios' OWN address — printed as "the default address of
// IC-7850/IC-7851" (PDF p.229, folio 15-18) — so checking for it against
// either radio's OWN prose would fault on their own stated fixed-address
// limitation, and it is checked against every OTHER model instead. What
// each entry's own ownParticulars DOES carry is the sibling's bare name,
// which is the check with teeth for this pair: neither entry may mention
// the other model, both because a user reading advice about the radio
// they chose should not be told about a different one, and because
// TestRadiotext_IC7851AndIC7850DifferOnlyInTheModelName's substitution
// depends on it. NO PREFIX HAZARD RUNS EITHER WAY, unlike the IC-7300/
// IC-7300MK2 pair's: "IC-7850" is not a substring of "IC-7851" nor the
// reverse (they differ in the last character).

// wantIC7851 and wantIC7850 are the two entries pinned VERBATIM, shared
// by each model's own verbatim test and by the substitution test below,
// so that neither can pass against a stale copy of the other's
// expectation — the wantFTdx101D/wantFTdx101MP arrangement, for the same
// reason.
var wantIC7851 = radiotext.Text{
	EraseProcedure:           "The IC-7851's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level command — but this build sends neither: no builder exists for either, and no IC-7851 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. Clear the channel at the radio instead, following the memory-channel clear procedure in its own instruction manual. The two programmed scan edges cannot be cleared at all: the radio's own memory-channel table prints their CLEAR column as \"No\".",
	GridLegendNote:           "Tone is read and written for the IC-7851 over CI-V by this build, but unverified against real hardware — no IC-7851 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT memory groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. The same holds for its data mode, with a wider consequence: a channel already set to DATA 1, DATA 2 or DATA 3 — or already in a SELECT group — cannot be written back by this build at all, because there is no honest value to preserve in a region it does not map.",
	ToneScanSkipVerification: "",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7851 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-7851's nearest wire nibble marks one of three SELECT memory groups, not a skip flag",
	},
	ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7851: this build knows of none to require. This driver talks only to CI-V address 8Eh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED, since both of this radio's printed CI-V speed settings ship on Auto and name no number to prefer. The six speeds offered are the USB port's list: on the remote-jack path with a level converter the radio stops at 19200, and this build cannot tell which path is wired. Note too that the IC-7851 and its sibling share one address, one manual and one frame shape, and this build cannot tell them apart — the model reported is the one you selected, not one it detected. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

var wantIC7850 = radiotext.Text{
	EraseProcedure:           "The IC-7850's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level command — but this build sends neither: no builder exists for either, and no IC-7850 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. Clear the channel at the radio instead, following the memory-channel clear procedure in its own instruction manual. The two programmed scan edges cannot be cleared at all: the radio's own memory-channel table prints their CLEAR column as \"No\".",
	GridLegendNote:           "Tone is read and written for the IC-7850 over CI-V by this build, but unverified against real hardware — no IC-7850 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT memory groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. The same holds for its data mode, with a wider consequence: a channel already set to DATA 1, DATA 2 or DATA 3 — or already in a SELECT group — cannot be written back by this build at all, because there is no honest value to preserve in a region it does not map.",
	ToneScanSkipVerification: "",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7850 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-7850's nearest wire nibble marks one of three SELECT memory groups, not a skip flag",
	},
	ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7850: this build knows of none to require. This driver talks only to CI-V address 8Eh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED, since both of this radio's printed CI-V speed settings ship on Auto and name no number to prefer. The six speeds offered are the USB port's list: on the remote-jack path with a level converter the radio stops at 19200, and this build cannot tell which path is wired. Note too that the IC-7850 and its sibling share one address, one manual and one frame shape, and this build cannot tell them apart — the model reported is the one you selected, not one it detected. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

// TestRadiotext_IC7851Verbatim and TestRadiotext_IC7850Verbatim are
// TestRadiotext_IC7610Verbatim's siblings for the additions tier's first
// registration (Tier 4b), and they guard the same kind of fact: the
// HEDGES. This prose was written in radiotext.go itself, for two radios
// this project has never connected to anything, under the honesty rule
// recorded at ic7851Text.
//
// "no IC-7851 has ever answered a frame", "no minimum version is
// established", "is ASSUMED", "unverified against real hardware" and
// "this build cannot tell them apart" are the load-bearing words. An
// editor tidying them into confident advisory copy — or reaching for a
// neighbouring model's wording because these two look thin — would
// attribute one radio's evidence to another. That edit fails here.
//
// ToneScanSkipVerification is asserted EMPTY for the same reason every
// other model's is: both of core/driver/ic7851's write-trial guards are
// false, so there is no hardware-preservation verification of any kind to
// report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED ENTRY, the
// sibling included, and for this pair the sibling is the most important
// of them: the two entries are meant to be near-copies of one another
// EXCEPT for the model name, so a field that forgot to substitute the
// name would be byte-identical to the sibling's and would serve one
// radio's advice under the other's title.
func TestRadiotext_IC7851Verbatim(t *testing.T) {
	assertIC7851PairEntry(t, "IC-7851", wantIC7851)
}

func TestRadiotext_IC7850Verbatim(t *testing.T) {
	assertIC7851PairEntry(t, "IC-7850", wantIC7850)
}

// assertIC7851PairEntry runs the verbatim pin and the non-borrowing check
// for one row of the IC-7851 pair.
func assertIC7851PairEntry(t *testing.T, model string, want radiotext.Text) {
	t.Helper()

	got, ok := radiotext.For(model)
	if !ok {
		t.Fatalf("For(%q) ok = false, want true — the model is registered in internal/wiring, so it must have prose", model)
	}
	if got != want {
		t.Errorf("For(%q) = %#v,\nwant %#v", model, got, want)
	}
	assertNotBorrowedFromAnyOtherModel(t, model, got)
}

// TestRadiotext_IC7851AndIC7850DifferOnlyInTheModelName is spec D1.2
// stated as a SUBSTITUTION, exactly as
// TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName states plan D8 for
// the Yaesu pair — and for a stronger reason here, because these two rows
// share not merely a manual but ONE driver implementation, ONE
// civ.Profile and ONE CI-V address. Replacing every occurrence of
// "IC-7850" with "IC-7851" throughout the IC-7850's entry must reproduce
// the IC-7851's entry byte for byte.
//
// Why substitution rather than a field-by-field comparison of the fields
// that happen not to name the model: the interesting failure is not "two
// fields drifted apart", it is "somebody added a sentence to ONE row's
// entry" — a claim about one of these radios that no evidence
// distinguishes from the other. A comparison restricted to the
// model-naming fields cannot see that; this can, because the added
// sentence survives the substitution and breaks the equality.
//
// EITHER DIRECTION IS WELL-DEFINED HERE, unlike the FTdx101 pair's (where
// "FTdx101D" is a substring of nothing but substituting it in would leave
// the MP's name unmatched): neither of these two names is a substring of
// the other. IC-7850 -> IC-7851 is the direction taken, arbitrarily and
// stated as arbitrary.
//
// NON-VACUITY: at least one field must actually name the model, or the
// substitution would be the identity function and this test would prove
// only that the two entries are equal — which they are not, and must not
// be.
func TestRadiotext_IC7851AndIC7850DifferOnlyInTheModelName(t *testing.T) {
	a, ok := radiotext.For("IC-7851")
	if !ok {
		t.Fatal(`For("IC-7851") ok = false, want true`)
	}
	b, ok := radiotext.For("IC-7850")
	if !ok {
		t.Fatal(`For("IC-7850") ok = false, want true`)
	}

	aFields := textFields(a)
	naming := 0
	for field, bVal := range textFields(b) {
		if strings.Contains(bVal, "IC-7850") {
			naming++
		}
		if got := strings.ReplaceAll(bVal, "IC-7850", "IC-7851"); got != aFields[field] {
			t.Errorf("%s: the IC-7850's text with its model name replaced by the IC-7851's is\n  %q\nbut the IC-7851's is\n  %q\n— spec D1.2: the two entries may differ ONLY where they name the model", field, got, aFields[field])
		}
	}
	if naming == 0 {
		t.Error("no IC-7850 field names the model — the substitution above is the identity function and this test asserted nothing")
	}

	// And the two entries are NOT equal: they name different radios, and
	// a user reading the IC-7850's advisories must see the IC-7850's name.
	if a == b {
		t.Error("the IC-7851's and IC-7850's entries are byte-identical — each row's prose must name its own model")
	}
}

// TestRadiotext_IC7760Verbatim is TestRadiotext_IC905Verbatim's sibling
// for the additions tier's SECOND registration, and it guards the same
// kind of fact: the HEDGES. This prose was written in radiotext.go
// itself, for a radio this project has never connected to anything, under
// the honesty rule recorded at ic7760Text.
//
// "no IC-7760 has ever answered a frame", "No minimum firmware version is
// established", "is ASSUMED" and "not printed anywhere" are the
// load-bearing words. An editor tidying them into confident advisory copy
// — or reaching for the IC-7610's or the IC-7851's wording, which this
// radio's own document happens to support almost sentence for sentence —
// would attribute one radio's evidence to another. That edit fails here.
//
// ToneScanSkipVerification is asserted EMPTY for the same reason every
// other model's is: core/driver/ic7760's writeTrialsComplete is false, so
// there is no hardware-preservation verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED ENTRY,
// including the two whose radios draw the SAME 27-byte data area as this
// one (additions spec D1.1). Those two are the borrowing risk this
// registration actually carries, and byte-identity is what catches a
// wholesale copy while the particulars catch a partial one.
func TestRadiotext_IC7760Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-7760's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level memory-clear command — but this build sends neither: no builder exists for either, and no IC-7760 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own manual. Whether the two programmed scan edges can be cleared at all is not printed anywhere: the clear block names the 99 memory channels and says nothing about P1 or P2.",
		GridLegendNote: "Tone is read and written for the IC-7760 over CI-V by this build, but unverified against real hardware — no IC-7760 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT memory groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. The same holds for its data mode, and the consequence there is wider than one column: a channel already set to DATA 1, DATA 2 or DATA 3 — or already in a SELECT group — cannot be written back by this build at all, because there is no honest value to preserve in a region it does not map.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7760 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7760's nearest wire nibble marks one of three SELECT memory groups, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7760: this build knows of none to require. This driver talks only to CI-V address B2h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole six-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no rate figure anywhere, about any port, and its own CI-V settings block carries no speed item at all. This radio is also two boxes, and which socket you use matters: the link this build supports is the controller's rear-panel USB B connection, which enumerates as TWO virtual COM ports, and which of the two answers is a radio setting the guide prints no default for — if one port is silent, try the other before concluding the radio is wrong. The RF deck's remote jack is a second path this build does not address. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7760")
	if !ok {
		t.Fatal(`For("IC-7760") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7760\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7760", got)
}

// TestRadiotext_IC7100Verbatim is TestRadiotext_IC7760Verbatim's sibling
// for the additions tier's THIRD registration, and it guards the same kind
// of fact: the HEDGES. This prose was written in radiotext.go itself, for a
// radio this project has never connected to anything, under the honesty
// rule recorded at ic7100Text.
//
// "no IC-7100 has ever answered a frame", "No minimum version is
// established", "is ASSUMED" and "this build does not read them rather
// than guess an address" are the load-bearing words. An editor tidying
// them into confident advisory copy — or reaching for the IC-705's or the
// IC-9700's wording, which this radio's own record resembles closely
// enough to tempt one — would attribute one radio's evidence to another.
// That edit fails here.
//
// ToneScanSkipVerification is asserted EMPTY for the same reason every
// other model's is: core/driver/ic7100's writeTrialsComplete is false, so
// there is no hardware-preservation verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED ENTRY,
// including the two whose radios accept the SAME 111-byte record as this
// one (additions spec D5's 111 B row). Those two are the borrowing risk
// this registration actually carries, and byte-identity is what catches a
// wholesale copy while the particulars catch a partial one.
func TestRadiotext_IC7100Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-7100's control-command chapter prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level memory-clear command — but this build sends neither: no builder exists for either, and no IC-7100 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. On this radio there is a further reason to leave them alone: the clearing block names \"memory channel 0 to 99\" where the address field itself is printed as 0001 to 0099 and omits the bank number altogether, so the printed form does not even say WHICH of the five banks it would clear. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone is read and written for the IC-7100 over CI-V by this build, but unverified against real hardware — no IC-7100 has ever answered a frame. Scan Skip is not: the nearest nibble in this radio's memory record marks a channel's SELECT-MEMORY membership, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. Two further states of a stored channel stop a write outright, because there is no honest value to preserve in a region this build does not map: a channel already switched INTO the select memory, and a channel stored with split on. So are the D-STAR call-sign fields and the two digital-squelch bytes — if a channel carries anything but the assumed template in those, the write is refused rather than blanking them.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7100 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7100's nearest wire nibble marks select-memory membership, not a skip flag",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7100: this build knows of none to require. This driver talks only to CI-V address 88h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED — it is the highest of the five speeds the manual prints, chosen because the radio's own CI-V speed item ships on Auto and names no number to prefer, and the manual warns that defaults differ between transceiver versions in any case. Two more things about this radio are worth knowing before blaming the port. Its memory list here holds the 495 ordinary channels, banks A to E, and NOTHING ELSE: the six programmed scan edges and four call channels are real channels on the radio, but the manual never says what bank number addresses them, so this build does not read them rather than guess an address. And CI-V Transceive ships ON, so the radio may be putting unsolicited frames on the bus of its own accord; they are counted and ignored, never acted on. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7100")
	if !ok {
		t.Fatal(`For("IC-7100") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7100\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7100", got)
}

// TestRadiotext_ICR8600Verbatim is TestRadiotext_IC7100Verbatim's sibling
// for the additions tier's FOURTH and last registration, and it guards
// the same kind of fact: the HEDGES. This prose was written in
// radiotext.go itself, for a receiver this project has never connected to
// anything, under the honesty rule recorded at icr8600Text.
//
// "no IC-R8600 has ever answered a frame", "No minimum version is
// established", "assumed on both halves" and "this build cannot tell you"
// are the load-bearing words. The speed hedge RANKS nothing, and must not:
// the superlative that stood here before — "a weaker guess here than on
// any other radio this build supports" — was false, the IC-7760's rate and
// list being both assumed as well, and it flattered that radio's evidence
// by implication. The no-other-model-named guard below forbids saying so
// in this receiver's own prose, so the clause states this receiver's
// evidence and stops.
// An editor tidying them into confident advisory copy would attribute
// evidence to this receiver that nobody holds. That edit fails here.
//
// AND ONE PHRASE IS LOAD-BEARING IN A SECOND WAY. GridLegendNote opens
// with "receiver — no transmit fields", which additions spec D4.2 asks
// for IN THOSE WORDS: it is the sentence that explains an absent column
// as anatomy rather than as an unwritable field, and it is served to the
// grid unchanged through app/uispec.go. A rewrite that dropped it would
// leave the first receiver's grid explaining nothing, so the verbatim
// comparison below is what holds the spec's wording in place.
//
// ProbeFirmwareNote ALSO CARRIES THE BOUNDED-WALK PARAGRAPH the IC-905's
// note carries (F1, this file's sibling task): this receiver's default
// Open leaves part of its memory space unwalked too, so "a channel
// stored outside that walk is simply not listed here" and "not evidence
// that the receiver's channel is empty" are load-bearing here for the
// same reason they are on the IC-905's entry, in this receiver's own
// words for its own walk — see core/driver/icr8600/read.go's discover.
//
// ToneScanSkipVerification is asserted EMPTY for the same reason every
// other model's is: core/driver/icr8600's writeTrialsComplete is false, so
// there is no hardware-preservation verification of any kind to report.
//
// THE NON-BORROWING CHECK RUNS AGAINST EVERY OTHER REGISTERED ENTRY. None
// of them describes a receiver, so wholesale borrowing here would be more
// visible than usual — which is exactly why the partial kind, a clause
// lifted from a transceiver's entry, is the one the particulars list
// catches.
func TestRadiotext_ICR8600Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The IC-R8600's CI-V Reference Guide DOES print a memory clear form — a memory-set frame carrying FF where the record would go — and this build does not send it: no builder exists for it, the outbound gate admits only the identity read, a memory read and a re-validated memory set, and no IC-R8600 has ever confirmed what the printed form does, so sending one risks clearing the wrong channel rather than the intended one. The printed form also excludes group 0102, the programmed scan edges, from what it may clear, which is a scope this build could not honour in any case: it does not address that group at all. Clear a memory from the receiver's own front panel instead, following the procedure in its instruction manual.",
		GridLegendNote: "This radio is a receiver — no transmit fields: an IC-R8600 has no transmitter, and its memory record carries no transmit frequency and no transmitted tone, so those columns are absent by anatomy rather than merely unwritable. Tone squelch IS read and written over CI-V by this build, but unverified against real hardware — no IC-R8600 has ever answered a frame — and only on an FM channel, the tone mode, received tone, DTCS code and DTCS polarity all living in the FM tail alone. Scan Skip is neither read nor written, and on this receiver that refuses TWO printed settings rather than one: the first record byte carries a three-valued scan-skip choice in one half and a ten-valued select-scan group in the other, and this build maps neither, so a channel holding anything but zero in the scan-skip half is refused rather than rewritten as zero. The five digital classes cost more again — a D-STAR, P25, NXDN, DCR or dPMR channel whose squelch bytes differ from the assumed template cannot be written back at all, and neither can a change of mode INTO one of those classes, because there is no honest value to put in a tail this build does not map.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build on FM channels only — unverified against real hardware, since no IC-R8600 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — this receiver's first record byte holds a printed scan-skip choice and a select-scan group, and neither half is mapped",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the receiver's display. No minimum version is established for the IC-R8600: this build knows of none to require. This driver talks only to CI-V address 96h, with no --civ-address option to change it and no way to detect a receiver set to a different address; and its opening speed of 19200 is assumed on both halves — this receiver's CI-V Reference Guide prints no factory default speed, mentions no automatic setting, and never lists the rates its menu offers, so the rate AND the list it was chosen from are both assumed. The guide's own advice is to set the address, the speed and the transceive function in the receiver's Set mode before controlling it, which is the first thing to check. Two more things about this receiver are worth knowing before blaming the port. It has FOUR possible control terminals — a remote jack, a front and a rear USB port, and a network connection — and this build talks over USB, so if one port is silent, check which terminal the receiver has been told to use before concluding the cable is wrong. Neither the transceive setting nor the echo-back setting of either USB port has a printed default, so this build cannot tell you whether unsolicited frames should be expected of the receiver's own accord; any that arrive are counted and ignored, never acted on. Opening this receiver also discovers its Memories bank's occupied slots by a BOUNDED walk — group 0 in full, then channel 00 of every other group, reading the rest of a group only where its channel 00 answered — not the whole 100x100 space, and nothing on this build's command line or in its window widens it (the driver's own WithFullInventoryWalk is a Go-level option no registered composition passes): a channel stored outside that walk is simply not listed here, so its absence from the grid is not evidence that the receiver's channel is empty. If nothing answers, check the receiver's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-R8600")
	if !ok {
		t.Fatal(`For("IC-R8600") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-R8600\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-R8600", got)
}

// TestRadiotext_ProbeNote_DoesNotOverstateTheWalkBound mirrors
// core/driver/icr8600/write_test.go's
// TestOccupiedSurprise_TheDiagnosticNamesTheWalkThisSessionRan (its
// "after the bounded walk" subtest, around write_test.go:511-513): that
// test fails the build if the write-refusal text carries the struck
// phrase "no setting that widens it", because icr8600.go:34 exports
// WithFullInventoryWalk and the phrase claims no setting exists at all.
//
// THE PIN FORBIDS THE STEM, "no setting that widens", NOT ONLY THE EXACT
// PHRASE (side lanes fix round 1, review icom-minors-review-opus.md
// LOW-2): a literal match on "no setting that widens it" let the IC-705's
// own write refusal (ic705/write.go:343, "no setting that widens the
// walk") say the identical false thing and walk straight past this test,
// because the tail word differed. The stem catches that variant and any
// future "no option that widens it" too.
// The probe note tells the same bounded-walk story and is held to the
// same honesty rule, so it is pinned here too, independently of the
// verbatim comparisons above (which would also catch a regression, but
// only by chance — this test names the exact hazard).
//
// THREE MODELS, ONE TABLE (radio-roadmap.md follow-ups, 04/09/2026): the
// IC-905's own ProbeFirmwareNote carried the identical struck phrase —
// ic905.go exports WithFullInventoryWalk exactly as icr8600.go does, and
// neither package's option is reachable from any registered composition,
// CLI flag or GUI control (checked internal/wiring and cmd/rigprog) — so
// the same negative pin now covers both, rather than leaving the IC-905
// free to regress into the wording the IC-R8600's own text already
// struck. The IC-705's newly added bounded-walk paragraph (Item B) is
// held to the identical rule from the moment it is written, for the
// identical reason: ic705.go also exports WithFullInventoryWalk, and no
// registered composition passes it either.
func TestRadiotext_ProbeNote_DoesNotOverstateTheWalkBound(t *testing.T) {
	for _, model := range []string{"IC-R8600", "IC-905", "IC-705"} {
		t.Run(model, func(t *testing.T) {
			got, ok := radiotext.For(model)
			if !ok {
				t.Fatalf("For(%q) ok = false, want true — the model is registered in internal/wiring, so it must have prose", model)
			}
			if strings.Contains(got.ProbeFirmwareNote, "no setting that widens") {
				t.Errorf("ProbeFirmwareNote still claims no setting widens the walk, which this model's own WithFullInventoryWalk export falsifies: %q", got.ProbeFirmwareNote)
			}
			for _, want := range []string{
				"command line",
				"WithFullInventoryWalk",
			} {
				if !strings.Contains(got.ProbeFirmwareNote, want) {
					t.Errorf("ProbeFirmwareNote = %q, want it to contain %q (the honest form used at core/driver/icr8600/write.go:215)", got.ProbeFirmwareNote, want)
				}
			}
		})
	}
}

// TestRadiotext_FT891Verbatim pins every FT-891 Text field byte for byte
// (Tier 1 task 7, landed with that model's wiring registration —
// internal/wiring's TestEverySupportedModelHasRadiotext refuses a registered
// model with no prose, which is what makes this entry part of registration
// rather than a later nicety).
//
// THE HONESTY RULE APPLIES UNCHANGED. No FT-891 has ever been asked anything
// by this project (core/driver/ft891/doc.go), no FT-891 OPERATING manual is
// held — only the CAT Operation Reference Manual, rev 1909-C — and no write
// trial has happened (that driver's writeTrialsComplete is false). Every
// string therefore says what is actually known, including where something is
// NOT known, and borrows no other entry's EVIDENCE — no other radio's fact,
// hedge or particular: not the FT-710's (whose hedgeless sentences are ITS
// hardware evidence), not the FTdx10's or the FTdx101 pair's (whose hedges
// are about different radios and different manuals), and not any Icom
// entry's. That is a claim about evidence and not about phrasing: the
// mechanical check is whole-field byte-identity plus particulars, and this
// entry does share a 38-word BUILD-fact run with the FT-991A's (see
// ft991aText's own doc comment).
//
// WHAT THIS ENTRY CAN SAY THAT ITS YAESU SIBLINGS' CANNOT, and why it is
// written fresh rather than adapted: this radio's CAT manual prints its whole
// command set in one Control Command List, so the ERASE absence is
// manual-evidenced here rather than merely unclaimed (matrix §2.6); its menu
// chart prints a CAT RATE row with four rates and no factory marking (matrix
// §1.11-1.12, erratum M-E4), so the ASSUMED default speed can be stated with
// the menu number a user would have to visit; its connection section
// describes a USB-to-DUAL-UART bridge and never says which endpoint carries
// CAT (matrix §3.13); and its MT block contradicts its own Control Command
// List about whether a memory channel may be READ at all (matrix §3.12,
// driver register entry 7 "MT READ IS SUPPORTED FOR MEMORY AND PMS"). The
// last two are the plan's named requirements for this field, alongside the
// baud sentence.
//
// assertNotBorrowedFromAnyOtherModel runs against every OTHER registered
// entry, derived from wiring.SupportedModels() rather than a list fixed at
// this registration, so a later registration is covered here too.
func TestRadiotext_FT891Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "The FT-891 has no CAT erase command, and on this radio that absence is documented rather than merely unclaimed: the CAT manual prints the whole command set in one Control Command List and no memory-erase command appears in it. A channel can therefore be cleared only at the radio itself, and this build does not describe how — no FT-891 operating manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel erase procedure in the radio's own operating manual.",
		GridLegendNote: "Tone and Scan Skip are neither read nor written for the FT-891 by this build: its combined memory record carries a CTCSS on/off state byte and nothing else of either kind — no tone-number byte and no scan-skip flag in any of the record's 41 positions — so set both at the radio. Two further columns behave differently here from the file you may be importing. A CHIRP file's CW, CWR and RTTY rows are not imported on this radio: they map to the sideband-specific names CW-U, CW-L and RTTY-U, and this radio's own mode legend prints CW, CW-R, RTTY-LSB and RTTY-USB instead, so such a row is blocked rather than written as a mode the radio has never been shown to have. And a transmit-clarifier flag carried in from another radio's file is refused at the write rather than sent: this radio's memory record prints that position as fixed, so there is no transmit clarifier here to set.",
		// Deliberately empty, exactly as every other model's is whose
		// write-trial guard is false: this field states what IS and is NOT
		// hardware-verified about preservation across a rewrite, and with
		// core/driver/ft891's writeTrialsComplete false there is no
		// verification of any kind to report. Any sentence here would be a
		// hardware claim about a radio nobody here has touched.
		ToneScanSkipVerification: "",
		// Byte-identical to EraseProcedure, as every other entry's is: the
		// delete dialogue and the blocked-erase review answer the same
		// question, and splitting the wording would only invite one copy to
		// drift into a procedure the other refuses to state.
		// The two tooltips DIFFER, unlike the FTdx10's identical pair,
		// because this radio's two absences are differently evidenced in the
		// record itself: no tone-NUMBER byte, and no scan-skip FLAG (matrix
		// §2.3). Neither claims a preservation finding — there is none.
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not read or written over CAT by this build — this radio's memory record has no tone-number byte at all, and whether a rewrite preserves the tone has never been tested",
			ScanSkip: "not read or written over CAT by this build — this radio's memory record has no scan-skip flag at all, and whether a rewrite preserves the marking has never been tested",
		},
		// A placeholder LABEL, not an example: no FT-891 version string has
		// been seen here, so there is no format to exemplify.
		ProbeFirmwareNote: "Firmware version has no CAT query in this build — check the radio's display. No minimum version is established for the FT-891: this build knows of none to require. Its opening speed of 38400 is ASSUMED, not read off the radio: this radio's CAT manual prints the four rates its CAT RATE menu row offers — 4800, 9600, 19200 and 38400 — and marks none of them as the factory setting, and neither this build's command line nor its window offers a way to open at another rate, so a radio set differently has to be put back at menu 0506 before it will answer. Two more things about this radio are worth knowing before blaming the port. Its rear-panel USB socket is a built-in USB-to-dual-UART bridge, so the radio enumerates TWO serial devices, and the manual mentions the second only in the word \"Dual\" — it never says which of the two carries CAT — so if one is silent, try the other before concluding the cable or the speed is wrong. And this manual contradicts itself about READING a memory channel: its Control Command List marks the combined MEMORY WRITE & TAG command settable only, while that same command's own detail block, on the same printed page, gives it a read request and a full answer chart. This build asks the detail block's question and cross-checks the answer against the plain memory read, so a read refused for a channel that is plainly occupied is the manual's own ambiguity surfacing, not a fault in the port — one such read of a channel you know is populated is what would settle it.",
	}

	got, ok := radiotext.For("FT-891")
	if !ok {
		t.Fatal(`For("FT-891") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"FT-891\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "FT-891", got)
}

// TestRadiotext_FT891ProbeNote_CarriesItsThreeNamedFacts pins the three
// things the Tier 1 plan (task 7) requires of THIS field by name, rather
// than leaving them to the verbatim comparison above, which would also
// catch a regression but only by accident — a reworded note that quietly
// dropped one of the three would still be "verbatim" once someone updated
// the literal.
//
// The three are: the ASSUMED opening speed with the menu row a user would
// have to visit to change the radio's own (matrix §1.12, erratum M-E4 —
// there is no baud override on this build's command line or in its window,
// so menu 0506 is the only remedy); the two-UART caveat (matrix §3.13 —
// the manual names the second endpoint only in the word "Dual"); and the
// MT-Read contradiction (matrix §3.12 — the Control Command List against
// the MT detail block, which is why a user meeting
// ft891.ErrMTReadRejectedForOccupiedSlot is entitled to know the ambiguity
// started in the manual and not in this software).
func TestRadiotext_FT891ProbeNote_CarriesItsThreeNamedFacts(t *testing.T) {
	got, ok := radiotext.For("FT-891")
	if !ok {
		t.Fatal(`For("FT-891") ok = false, want true`)
	}
	for _, want := range []string{
		// The baud sentence, and the menu number that is its only remedy.
		"38400 is ASSUMED",
		"menu 0506",
		// The two-UART caveat.
		"USB-to-dual-UART bridge",
		"never says which of the two carries CAT",
		// The MT-Read contradiction.
		"contradicts itself",
		"Control Command List",
	} {
		if !strings.Contains(got.ProbeFirmwareNote, want) {
			t.Errorf("ProbeFirmwareNote = %q,\nwant it to contain %q", got.ProbeFirmwareNote, want)
		}
	}
	// The note must not promise a way out this build does not offer: there
	// is no baud override anywhere in the CLI or the GUI (matrix §1.12), so
	// the sentence has to say so rather than implying a flag exists.
	if !strings.Contains(got.ProbeFirmwareNote, "neither this build's command line nor its window offers a way to open at another rate") {
		t.Errorf("ProbeFirmwareNote = %q,\nwant it to state that no baud override exists in either face of this build", got.ProbeFirmwareNote)
	}
}

// TestRadiotext_FT991AVerbatim pins the FT-991A's entry (Tier 1 task 15a,
// landed IN THE SAME COMMIT as that model's wiring registration —
// internal/wiring's TestEverySupportedModelHasRadiotext refuses a registered
// model with no prose, and ownParticulars' lockstep above PANICS on one).
//
// THE HONESTY RULE APPLIES UNCHANGED. No FT-991A has ever been asked
// anything by this project (core/driver/ft991a/doc.go), no FT-991A OPERATING
// manual is held — only the CAT Operation Reference Manual — and no write
// trial has happened (that driver's writeTrialsComplete is false). Every
// string therefore says what is actually known, including where something is
// NOT known, and borrows no other entry's EVIDENCE — no other radio's fact,
// hedge or particular. Shared PHRASING is a different matter and is not
// claimed against: some clauses are word-for-word the FT-891 entry's, and
// each of those is a statement about this BUILD rather than about either
// radio, identically true of both (see ft991aText's own doc comment). What
// assertNotBorrowedFromAnyOtherModel below pins mechanically is whole-field
// byte-identity plus particulars, not phrasing.
//
// WHAT THIS ENTRY CAN SAY THAT ITS YAESU SIBLINGS' CANNOT, and why it is
// written fresh rather than adapted: this radio's memory record carries a
// FIVE-state tone byte where every sibling's carries three, so the DCS CODE
// joins the tone number as radio-side (matrix §1.17, §2.4); its PMS slots
// are the wire NUMBERS 100-117 where its own front panel prints P-1L to
// P-9U (matrix §1.4.2, §3.13, plan decision P20); its menu chart's row 087
// is excluded, so the settings viewer shows 152 items for 153 printed rows
// (matrix §3.9, plan decision P15); its baud menu is 031 and 029 is a
// different port's (matrix §1.11-1.12, plan decision P11); and its USB
// socket enumerates two serial devices while a second RS-232C path is gated
// by a menu row whose printed legend has a hole in it (matrix §3.12).
//
// assertNotBorrowedFromAnyOtherModel runs against every OTHER registered
// entry, derived from wiring.SupportedModels() rather than a list fixed at
// this registration, so a later registration is covered here too.
func TestRadiotext_FT991AVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "There is no CAT erase command for the FT-991A, and here that absence is printed rather than merely unclaimed: this radio's Control Command List is the whole of its CAT vocabulary and holds no command that clears a memory channel — the nearest entries, QMB STORE and QMB RECALL, address the quick-memory bank instead. Clearing a channel is therefore something only the radio itself can do, and this build will not describe how: no FT-991A operating manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than saying so. Follow the memory-channel erase procedure in the radio's own operating manual.",
		GridLegendNote: "Tone and Scan Skip are neither read nor written for the FT-991A by this build. Its combined memory record carries a five-state tone byte — CTCSS off, CTCSS encode and decode, CTCSS encode, DCS encode and decode, DCS encode — and no more of either kind: the tone frequency number and the DCS code both live on a separate command that reports what the radio is doing now rather than what a memory channel holds, and no position anywhere in the record marks a channel for scan skip, so set the tone number, the DCS code and the skip marking at the radio. Two further things this window shows will not match what the radio prints. C4FM is one of this radio's fourteen modes and CHIRP has no name for it, so a C4FM channel cannot be carried out to a CHIRP file as itself. And the PMS pairs are shown here as the channel numbers 100 to 117, which is what this radio's own CAT record uses, while its front panel and its manual print the same eighteen slots as P-1L to P-9U — the numbers are the wire's and the letters are the panel's, and they name the same slots in the same order. One count is worth explaining before it surprises you: the settings list shows 152 items where this radio's menu chart prints 153 rows. Row 087, RADIO ID, is left out because the chart gives it neither a width nor a parameter — ten printed hyphens and nothing else — so this build cannot size an answer to it and will not send a question it cannot read. One EX087; read on a real radio would settle it either way.",
		// Deliberately empty, exactly as every other model's is whose
		// write-trial guard is false: this field states what IS and is NOT
		// verified on real hardware about preservation across a rewrite,
		// and with core/driver/ft991a's writeTrialsComplete false there is
		// no verification of any kind to report.
		ToneScanSkipVerification: "",
		// Byte-identical to EraseProcedure, as every other entry's is.
		// The two tooltips DIFFER because the two absences are differently
		// evidenced (matrix §2.4): the tone number and the DCS code are a
		// DIFFERENT COMMAND's live state, while the scan-skip marking has
		// no position in this record at all. Neither claims a preservation
		// finding — there is none.
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not read or written over CAT by this build — this radio's memory record carries a tone STATE but no tone frequency number and no DCS code, both of which are a different command's live state, and nothing has established what a rewrite does to either",
			ScanSkip: "not read or written over CAT by this build — no position anywhere in this radio's memory record marks a channel for scan skip, and nothing has established what a rewrite does to the marking",
		},
		// A placeholder LABEL, not an example: no FT-991A version string
		// has been seen here, so there is no format to exemplify.
		ProbeFirmwareNote: "Firmware version has no CAT query in this build — read it off the radio's display. No minimum version is established for the FT-991A: this build knows of none to require. The 38400 this build opens at is ASSUMED, not read off the radio. The menu row that sets the rate for the socket this programme uses is 031 CAT RATE, which prints 4800, 9600, 19200 and 38400 and marks none of them as the factory setting, and neither this build's command line nor its window offers a way to open at another rate, so a radio set differently has to be put back at menu 031 before it will answer. Menu 029 is NOT that row: 029 232C RATE sets the rate of the rear-panel RS-232C jack, which is a different port from the one this programme opens, so changing it will not make this radio answer here. Two more things are worth knowing before blaming the port. This radio's rear-panel USB socket is a built-in USB-to-dual-UART bridge, so it enumerates TWO serial devices and the manual never says which of the two carries CAT — if one is silent, try the other before concluding the cable or the speed is wrong. And there is a second, entirely separate CAT path on the rear-panel RS-232C jack, gated by menu 028 GPS/232C SELECT; that row's own printed option list is defective in the manual this build was written from — it prints 0: GPS1, 1: GPS2 and 3: RS232C, with key 2 missing — so a reader who goes looking for that setting should expect the printed list and the radio's own to disagree.",
	}

	got, ok := radiotext.For("FT-991A")
	if !ok {
		t.Fatal(`For("FT-991A") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"FT-991A\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "FT-991A", got)
}

// TestRadiotext_FT991ANamedFacts pins the facts the Tier 1 plan (task 15a)
// requires of THIS model's two long fields BY NAME, rather than leaving them
// to the verbatim comparison above, which would also catch a regression but
// only by accident — a reworded note that quietly dropped one of them would
// still be "verbatim" once someone updated the literal.
//
// ProbeFirmwareNote's three (plan decision P11, matrix §1.11-1.12 and
// §3.12): the ASSUMED opening speed named with menu 031 CAT RATE, with menu
// 029 EXCLUDED as the RS-232C jack's own rate rather than merely unmentioned
// — a user sent to 029 sets the wrong port's rate and the radio stays silent
// — the two enumerated USB serial devices, and the menu-028 RS-232C gate
// WITH that row's own printed defect, which is stated rather than silently
// corrected because nothing establishes whether the hole is the radio's or
// the chart's.
//
// GridLegendNote's four (plan decisions P15 and P20, matrix §2.4, §3.9 and
// §3.13): the tone number AND the DCS code as radio-side, C4FM as
// unmappable in CHIRP, the 087 exclusion as 152 items against 153 printed
// rows with the one EX087; capture that would settle it, and the PMS slots'
// two spellings — the wire numbers this build shows and the P-1L to P-9U the
// radio's own panel and manual print.
func TestRadiotext_FT991ANamedFacts(t *testing.T) {
	got, ok := radiotext.For("FT-991A")
	if !ok {
		t.Fatal(`For("FT-991A") ok = false, want true`)
	}
	for _, want := range []string{
		// The baud sentence, and the menu row that is its only remedy.
		"38400 this build opens at is ASSUMED",
		"031 CAT RATE",
		// Menu 029 named ONLY to exclude it. The needles are the DATUM
		// (the menu row's own number and printed label) and the shortest
		// statement of the exclusion, on the FT-891 test's granularity
		// above: a faithful reword survives both, a note that dropped the
		// exclusion loses both.
		"029 232C RATE",
		"a different port",
		// The two enumerated USB serial devices.
		"USB-to-dual-UART bridge",
		"never says which of the two carries CAT",
		// The RS-232C gate and its own printed defect — the row's number
		// and label, the printed key this build was written from, and the
		// hole in the printed list, each as its own needle rather than as
		// one long transcription of this note's own sentence.
		"028 GPS/232C SELECT",
		"3: RS232C",
		"key 2 missing",
	} {
		if !strings.Contains(got.ProbeFirmwareNote, want) {
			t.Errorf("ProbeFirmwareNote = %q,\nwant it to contain %q", got.ProbeFirmwareNote, want)
		}
	}
	for _, want := range []string{
		// The tone number AND the DCS code, both radio-side.
		"tone frequency number and the DCS code",
		"set the tone number, the DCS code and the skip marking at the radio",
		// C4FM, unmappable in CHIRP.
		"C4FM",
		"CHIRP has no name for it",
		// The 087 exclusion, its arithmetic and its one capture.
		"152 items",
		"153 rows",
		"Row 087",
		"EX087;",
		// The PMS slots' two spellings.
		"100 to 117",
		"P-1L to P-9U",
	} {
		if !strings.Contains(got.GridLegendNote, want) {
			t.Errorf("GridLegendNote = %q,\nwant it to contain %q", got.GridLegendNote, want)
		}
	}
}

// TestRadiotext_TS590SVerbatim pins every TS-590S Text field byte for byte
// (Tier 6 task 18, landed with that row's wiring registration —
// internal/wiring's TestEverySupportedModelHasRadiotext refuses a registered
// model with no prose, which is what makes this entry part of registration
// rather than a later nicety).
//
// THE HONESTY RULE APPLIES UNCHANGED, and this family starts further back
// than any registered before it. No Kenwood radio has ever answered a frame
// put to it by this project; only the PC CONTROL COMMAND REFERENCE is held
// here, not the instruction manual, which is why every "clear it at the
// radio" sentence declines to say how; and this row's write trials have not
// happened. Every string says what is actually known, including where
// something is not, and borrows the wording of no other entry — not the
// FT-710's (whose hedgeless sentences are ITS evidence from a real radio),
// not the hedged Yaesu entries' (different radios, different manuals), and
// not any Icom entry's.
//
// WHAT THIS ENTRY CAN SAY THAT NO EARLIER ONE COULD: tone and scan skip are
// READ AND WRITTEN here. Every registered radio before this family had at
// least one of the two unreachable, so every earlier tooltip and legend says
// some version of "not carried by this radio's protocol"; this record carries
// a tone mode, two tone numbers and a channel-lockout flag, so this entry has
// to say the opposite and could not have been adapted from any of them.
//
// THE VOCABULARY CHECK RUNS AGAINST THIS PROSE, unlike the six Yaesu
// entries' (plan decision P17): Kenwood is deliberately NOT in yaesuModels,
// because these books say "PC control command" and never the Yaesu family's
// word, so borrowing a Yaesu sentence would be caught by the vocabulary scan
// as well as by the byte-identity one.
func TestRadiotext_TS590SVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The TS-590S has no erase command this build will send, and the absence is a CHOICE over the weakest evidence in the book rather than a plain gap. The only clearing route this radio's own book prints is a side effect of a shortened memory-write frame — leave one digit of the name field unspecified, set every other parameter to zero, and the channel is erased — and the LENGTH of that short frame is a reading of the sentence rather than a number the book prints anywhere. This build therefore admits a memory-write frame of exactly 50 bytes and no other, so the short form cannot be sent even by accident. A channel can be cleared only at the radio itself, and this build does not describe how: no TS-590S instruction manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel clearing procedure in the radio's own instruction manual.",
		GridLegendNote:           "Tone and Scan Skip ARE read and written on the TS-590S, unlike the Yaesu radios this programme also supports: its 50-byte memory record carries a tone mode, separate transmit and receive tone numbers and a channel-lockout flag. ONLY FM CHANNELS ARE EVER WRITTEN, and that is the BROADEST refusal on this radio rather than a detail: the two bytes the record carries beside the mode have only two printed meanings, \"00: FM Normal\" and \"01: FM Narrow\", and the book never says what either means in SSB, CW, AM or FSK — so a channel in any other mode is refused at the write, naming register entry A23, rather than sent with a byte whose meaning this build does not hold. Reading is unaffected in every mode. ONE TONE VALUE IS STILL REFUSED, and it is a disagreement between this radio's own two printed tone charts rather than a limit of this build: 1750 Hz is the last entry of the chart the tone-number command prints and has no entry at all in the chart the tone-squelch command prints, so a 1750 Hz TRANSMIT tone is written normally while a 1750 Hz RECEIVE tone is refused at the write, naming decision 14, rather than sent as a number the receive chart does not print. A MEMORY CHANNEL READ OFF THIS RADIO IS NOT WRITTEN BACK UNTIL YOU SUPPLY ITS TRANSMIT FREQUENCY, and that is the cost worth knowing before you plan a round trip. This radio expresses a split as two frames over one channel number, this build sends one, and what the second frame answers on a simplex channel is printed nowhere in the book — so a read leaves the transmit frequency unavailable rather than guessing at it, and a channel is written only when its transmit disposition is known. Read the memories, edit a name and send them straight back and EVERY memory channel is refused, naming register entry A9; fill the transmit frequency in yourself and the channel writes, typing the receive frequency there being the simplex channel the single frame this build sends can express. A GENUINE SPLIT IS STILL REFUSED RATHER THAN FLATTENED even then: a channel whose transmit frequency differs from its receive frequency has no second frame to go in, so it is refused instead of being written as simplex and read back as though it had never been split. A SCAN RANGE IS NOT AFFECTED by any of this, because in that bank the second frame carries a range's end frequency rather than a transmit frequency and there is no transmit disposition to require. A9 is assumed per row, and until it is lifted on this row — which would take somebody reading a simplex channel's transmit side off a real TS-590S and reporting what came back — this is what a memory write costs here. THE FILTER COLUMN CANNOT BE SET ON THIS ROW AT ALL: the record position that carries it is guaranteed to be zero only on the 1.xx firmware, and one entry in the model list cannot say 'settable above 2.00', so this build declines to set it on any TS-590S and publishes the cost rather than hiding it. CHANNEL WRITES ARE REFUSED OUTRIGHT on a TS-590S reporting firmware 2.00 or later, and equally on one whose firmware answer this build cannot read as a version at all, because a version that cannot be compared cannot be shown to be a 1.xx one; the session reads normally either way, naming register entries A13/A14. AND A CHIRP FILE'S ORDINARY ROWS DO IMPORT ON THIS RADIO, with two exceptions. A CHIRP file's blank Duplex column is its ordinary simplex row, and although this radio declares no shift vocabulary for it to land in — its 50-byte record carries no duplex selector of any kind — a blank column asks for nothing this radio cannot do, so such a row is imported as simplex rather than refused. A Duplex column reading \"off\" IS refused, because that asserts a state distinct from simplex — no duplex configured at all — which this record cannot carry. And CW, CWR and RTTY rows are refused on the mode: those map to the sideband-specific names CW-U, CW-L and RTTY-U, and this radio's own mode legend prints CW, CW-R, FSK and FSK-R instead. AND AN IMPORTED CHIRP CHANNEL IS STILL REFUSED AT THE WRITE, on values CHIRP has no column for. The transmit frequency is no longer one of them: a blank Duplex column is the file's own simplex statement, and this record expresses simplex by transmitting where it receives, so an import now carries that. What is left unsaid is the data mode and both tone numbers — a blank Tone column states only that tone is switched off — and CHIRP has no data-mode column at all, so a CHIRP file alone can never complete a write here. Fill those in before writing, or write from this programme's own CSV, which carries them. This programme's own CSV import and export are unaffected.",
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over this radio's PC-control interface on a TS-590S, so nothing here is preserved: the 50-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over this radio's PC-control interface on a TS-590S, so nothing here is preserved: the 50-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version IS readable on this radio, and the probe reads it: two frames go out, an identity request and then a version request, and the four characters that come back are printed in the report above. THIS ROW IS THE ONE WHERE THE ANSWER CHANGES WHAT THIS BUILD WILL DO: a TS-590S reporting 2.00 or later, or reporting something that cannot be read as a version at all, is refused for channel writes, because the book's guarantee about the filter position in a memory record covers the 1.xx firmware and stops there. A version this build cannot parse gives a read-only session rather than a refused one — refusing the session outright would make a perfectly good radio unreadable on the strength of a grammar assumed from a single worked example in the book. Its opening speed of 9600 is ASSUMED, not read off the radio, and it is an operational assumption rather than a cautious one: neither Kenwood book prints a factory speed anywhere, a wrong speed is not a safe speed but an unreachable radio, and the symptom is a timeout that looks exactly like a dead port or a bad cable. This build offers NO way to open at another speed — not on the command line and not in the window — and it never probes the port at several speeds to find out, so a radio set to anything else has to be put back to 9600 at its own menu before it will answer. The printed rate list is 9600, 19200, 38400, 57600 and 115200; 4800 is on the radio's list too and is deliberately absent from this build's, because each book attaches a condition to it that a flat list of speeds cannot express.",
	}

	got, ok := radiotext.For("TS-590S")
	if !ok {
		t.Fatal(`For("TS-590S") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-590S\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-590S", got)
}

// TestRadiotext_TS590SGVerbatim is its sibling's twin, on the same terms.
// See TestRadiotext_TS590SVerbatim for the honesty rule this family is
// written under, and TestRadiotext_TS590SAndSGDifferInMoreThanTheModelName
// for what the two entries may NOT share.
func TestRadiotext_TS590SGVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The TS-590SG has no erase command this build will send, and the absence is a CHOICE over the weakest evidence in the book rather than a plain gap. The only clearing route this radio's own book prints is a side effect of a shortened memory-write frame — leave one digit of the name field unspecified, set every other parameter to zero, and the channel is erased — and the LENGTH of that short frame is a reading of the sentence rather than a number the book prints anywhere. This build therefore admits a memory-write frame of exactly 50 bytes and no other, so the short form cannot be sent even by accident. A channel can be cleared only at the radio itself, and this build does not describe how: no TS-590SG instruction manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel clearing procedure in the radio's own instruction manual.",
		GridLegendNote:           "Tone and Scan Skip ARE read and written on the TS-590SG, unlike the Yaesu radios this programme also supports: its 50-byte memory record carries a tone mode, separate transmit and receive tone numbers and a channel-lockout flag. ONLY FM CHANNELS ARE EVER WRITTEN, and that is the BROADEST refusal on this radio rather than a detail: the two bytes the record carries beside the mode have only two printed meanings, \"00: FM Normal\" and \"01: FM Narrow\", and the book never says what either means in SSB, CW, AM or FSK — so a channel in any other mode is refused at the write, naming register entry A23, rather than sent with a byte whose meaning this build does not hold. Reading is unaffected in every mode. ONE TONE VALUE IS STILL REFUSED, and it is a disagreement between this radio's own two printed tone charts rather than a limit of this build: 1750 Hz is the last entry of the chart the tone-number command prints and has no entry at all in the chart the tone-squelch command prints, so a 1750 Hz TRANSMIT tone is written normally while a 1750 Hz RECEIVE tone is refused at the write, naming decision 14, rather than sent as a number the receive chart does not print. THE FILTER COLUMN IS SETTABLE ON THIS ROW, which is the one memory-channel difference between the two TS-590 entries in the model list: the record position that selects filter A or B is live on every radio this book describes, with no firmware condition attached to it. A MEMORY CHANNEL READ OFF THIS RADIO IS NOT WRITTEN BACK UNTIL YOU SUPPLY ITS TRANSMIT FREQUENCY, and that is the cost worth knowing before you plan a round trip. This radio expresses a split as two frames over one channel number, this build sends one, and what the second frame answers on a simplex channel is printed nowhere in the book — so a read leaves the transmit frequency unavailable rather than guessing at it, and a channel is written only when its transmit disposition is known. Read the memories, edit a name and send them straight back and EVERY memory channel is refused, naming register entry A9; fill the transmit frequency in yourself and the channel writes, typing the receive frequency there being the simplex channel the single frame this build sends can express. A GENUINE SPLIT IS STILL REFUSED RATHER THAN FLATTENED even then: a channel whose transmit frequency differs from its receive frequency has no second frame to go in, so it is refused instead of being written as simplex and read back as though it had never been split. A SCAN RANGE IS NOT AFFECTED by any of this, because in that bank the second frame carries a range's end frequency rather than a transmit frequency and there is no transmit disposition to require. A9 is assumed per row, and until it is lifted on this row — which would take somebody reading a simplex channel's transmit side off a real TS-590SG and reporting what came back — this is what a memory write costs here. AND A CHIRP FILE'S ORDINARY ROWS DO IMPORT ON THIS RADIO, with two exceptions. A CHIRP file's blank Duplex column is its ordinary simplex row, and although this radio declares no shift vocabulary for it to land in — its 50-byte record carries no duplex selector of any kind — a blank column asks for nothing this radio cannot do, so such a row is imported as simplex rather than refused. A Duplex column reading \"off\" IS refused, because that asserts a state distinct from simplex — no duplex configured at all — which this record cannot carry. And CW, CWR and RTTY rows are refused on the mode: those map to the sideband-specific names CW-U, CW-L and RTTY-U, and this radio's own mode legend prints CW, CW-R, FSK and FSK-R instead. AND AN IMPORTED CHIRP CHANNEL IS STILL REFUSED AT THE WRITE, on values CHIRP has no column for. The transmit frequency is no longer one of them: a blank Duplex column is the file's own simplex statement, and this record expresses simplex by transmitting where it receives, so an import now carries that. What is left unsaid is the data mode, the filter and both tone numbers — a blank Tone column states only that tone is switched off — and CHIRP has no data-mode column and no filter column, so a CHIRP file alone can never complete a write here. Fill those in before writing, or write from this programme's own CSV, which carries them. This programme's own CSV import and export are unaffected.",
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over this radio's PC-control interface on a TS-590SG, so nothing here is preserved: the 50-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over this radio's PC-control interface on a TS-590SG, so nothing here is preserved: the 50-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version IS readable on this radio, and the probe reads it: two frames go out, an identity request and then a version request, and the four characters that come back are printed in the report above. NOTHING ON THIS ROW BRANCHES ON THE ANSWER — the filter position in a memory record is live on every TS-590SG the book describes — so the version is reported and not acted on. A version this build cannot parse still gives a working session: the grammar is assumed from a single worked example in the book, and refusing a radio over an assumption would cost more than it buys. Its opening speed of 9600 is ASSUMED, not read off the radio, and it is an operational assumption rather than a cautious one: neither Kenwood book prints a factory speed anywhere, a wrong speed is not a safe speed but an unreachable radio, and the symptom is a timeout that looks exactly like a dead port or a bad cable. This build offers NO way to open at another speed — not on the command line and not in the window — and it never probes the port at several speeds to find out, so a radio set to anything else has to be put back to 9600 at its own menu before it will answer. The printed rate list is 9600, 19200, 38400, 57600 and 115200; 4800 is on the radio's list too and is deliberately absent from this build's, because each book attaches a condition to it that a flat list of speeds cannot express.",
	}

	got, ok := radiotext.For("TS-590SG")
	if !ok {
		t.Fatal(`For("TS-590SG") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-590SG\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-590SG", got)
}

// TestRadiotext_TS480Verbatim is the ONE verbatim pin in this file whose
// model internal/wiring does NOT register (plan decision P17, Stuart
// decision row 5), and both halves of that need saying.
//
// IT IS WRITTEN AGAINST radiotext.For AND NOT AGAINST THE texts MAP. The map
// is unexported (radiotext.go) and this file is package radiotext_test, an
// external test package, so the map is not reachable from here at all. For
// resolves straight out of it independently of wiring.SupportedModels(), is
// exported, and is what a future caller will use — so it is both the only
// available surface and the honest one.
//
// THE ENTRY DELIBERATELY PRECEDES THE ROW. core/driver/ts480 is built and its
// registration is gated on an observation from a real radio that nobody has;
// landing the prose now is what keeps it under the same non-borrowing and
// vocabulary discipline as every other entry from birth, instead of arriving
// unreviewed inside the registration commit. Nothing in the shipped binary
// can reach it meanwhile: every caller of For passes a registered model.
func TestRadiotext_TS480Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The TS-480 has no erase command this build will send, and this radio's own book prints no clearing route for one to send: its memory-write section ends without the shortened-frame side effect the TS-590 book prints, and the only \"clear\" anywhere in its printed command set clears the RIT offset instead. This build admits a memory-write frame of exactly 50 bytes and no other in any case, so the TS-590's short form could not be sent here either. It would refuse a channel write in any case — no channel write of any kind is sent to this radio by this build — so clearing a channel is doubly a front-panel job here. This build does not describe how: no TS-480 instruction manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel clearing procedure in the radio's own instruction manual.",
		GridLegendNote:           "NO CHANNEL WRITE OF ANY KIND IS SENT TO A TS-480 BY THIS BUILD, so every column of this grid is a reading rather than an instruction: the step the memory record carries has TWO different printed legends on this radio, one for SSB, CW and FSK and one for AM and FM, so the same stored index means 0.5 kHz on one mode and 5 kHz on another, and the book never says which value means leave the step as it is — so a channel write would have to send a step nobody here can check. Reading is unaffected. Three further things a file can carry are lost on this radio and worth knowing before a round trip. MEMORY GROUP MEMBERSHIP CANNOT BE PRESERVED: this radio has ten memory groups and a command that chooses which of them are scanned, but no memory frame carries a channel's group and no command reads or writes one, so a channel saved to a file and sent back would lose which group it belonged to — there is nothing to refuse and no column to grade, which is why it is written here. THE TRANSMIT AND RECEIVE TONE NUMBERS ARE NOT SET either: this radio's two printed tone charts do not agree with one another about how many tones there are, so this build reads the tone mode and declines to choose a number. And a CHIRP file's ordinary rows DO import on this radio: a CHIRP file's blank Duplex column is its ordinary simplex row, and although this radio declares no shift vocabulary for it to land in — its memory record carries no duplex selector of any kind — a blank column asks for nothing this radio cannot do, so such a row is imported as simplex rather than refused. A Duplex column reading \"off\" IS refused, because that asserts a state distinct from simplex — no duplex configured at all — which this record cannot carry. CW, CWR and RTTY rows are refused a second time besides, on the mode: those map to the sideband-specific names CW-U, CW-L and RTTY-U, and the mode names this build publishes for this radio are CW, CW-R, FSK and FSK-R instead.",
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read but never written on a TS-480: the memory record carries a tone mode, which this build reads, and two tone numbers it declines to set because the radio's own two tone charts disagree about how many tones there are. Nothing is written to this radio at all, so nothing here can be lost by a rewrite",
			ScanSkip: "read but never written on a TS-480: the memory record carries a channel-lockout flag, which this build reads. Nothing is written to this radio at all, so nothing here can be lost by a rewrite",
		},
		ProbeFirmwareNote: "Firmware version is NOT readable on this radio and this build does not pretend otherwise: there is no version query in its printed command set at all, so check the radio's own display. Two frames still go out at the probe, an identity request and then a HARDWARE VARIANT request, and the second is where this radio differs from its two TS-590 stablemates — it reports which of the four printed variants answered, and an unexpected value REFUSES the session, where an unreadable version on a TS-590 only degrades it to reading. The asymmetry is deliberate: refuse where the book is complete and the radio is outside it, degrade where the book is thin and this build may have guessed its grammar wrong. Its opening speed of 9600 is ASSUMED, not read off the radio, and it is an operational assumption rather than a cautious one: neither Kenwood book prints a factory speed anywhere, a wrong speed is not a safe speed but an unreachable radio, and the symptom is a timeout that looks exactly like a dead port or a bad cable. This build offers NO way to open at another speed — not on the command line and not in the window — and it never probes the port at several speeds to find out, so a radio set to anything else has to be put back to 9600 at its own menu before it will answer. The printed rate list is 9600, 19200, 38400, 57600 and 115200; 4800 is on the radio's list too and is deliberately absent from this build's, because each book attaches a condition to it that a flat list of speeds cannot express.",
	}

	got, ok := radiotext.For("TS-480")
	if !ok {
		t.Fatal(`For("TS-480") ok = false, want true — this entry deliberately precedes the registry row (plan P17), so it must exist even though SupportedModels() does not name the model`)
	}
	if got != want {
		t.Errorf("For(\"TS-480\") = %#v,\nwant %#v", got, want)
	}

	// THE NON-BORROWING PIN, and it is the second of this entry's two pins
	// (plan decision P17). It runs exactly as every registered model's does
	// — assertNotBorrowedFromAnyOtherModel ranges wiring.SupportedModels(),
	// and "TS-480" is simply absent from that range, so there is no
	// self-comparison to skip and no ownParticulars entry needed for it.
	// What it proves is what matters: this radio's prose is not any other
	// registered radio's, byte for byte or particular by particular — its
	// Kenwood stablemates included, whose book it shares nothing with
	// but a manufacturer.
	assertNotBorrowedFromAnyOtherModel(t, "TS-480", got)
}

// TestRadiotext_TS590Pair_GridLegendCarriesItsPublishedCosts pins, on each
// 590 row, the write refusals and the CHIRP outcome a user meets in ordinary
// use and which the verbatim pins above would also catch — but only by
// accident, because a reworded legend that quietly dropped one of them would
// still be "verbatim" once somebody updated the literal. This is the FT-891's
// TestRadiotext_FT891ProbeNote_CarriesItsThreeNamedFacts applied to the
// field a user actually reads on the grid.
//
// THE FIRST IS THE A9 COST, and it is here because the Opus review of this
// task's registration found it published nowhere a user would look. A
// memory channel is written only when its transmit disposition is KNOWN
// (core/driver/ts590/write.go, the registerA9 rung, gated on the BANK
// publishing tx_frequency), and a channel read fresh off either radio never
// is: what an MR with P1=1 answers on a simplex channel is unprinted, so the
// read reports Unavailable rather than guessing. The consequence is the
// ordinary round trip — read the memories, edit, send them back — being
// refused on EVERY memory channel until the user fills the transmit
// frequency in. The old legend said only that a channel whose transmit
// frequency DIFFERS from its receive frequency is refused, which implies the
// simplex case writes; it is the one case that never does.
//
// THE SECOND IS DECISION 14's, which the same review found asserted the
// opposite of: the legend said the tone columns "behave like any other"
// while core/driver/ts590/write.go refuses a Known tone_rx of 1750 Hz
// (erratum M-E1 — TN's 43-entry chart admits it, CN's printed 00-41 does
// not, and one spec.Capabilities tone domain serves both directions).
//
// The SCAN clause is pinned with them because it is what keeps the A9
// sentence from over-claiming: that bank publishes no tx_frequency at all
// (matrix M-E2), so a scan-range write is not refused for this reason.
//
// THE THIRD IS A23, and it is the BROADEST refusal on this family rather
// than a footnote: core/driver/ts590/write.go's fmP14 refuses every
// published non-FM mode, so an SSB, CW, AM or FSK channel is refused even
// after consent and even with a transmit frequency supplied. The milestone
// close found it stated on the release notes and in docs/kenwood-models.md
// but on neither owner-facing surface a user reads at the moment of the
// refusal — this legend and docs/radio-notes.md.
//
// THE FOURTH IS THE CHIRP OUTCOME, and it has been through two corrections.
// The milestone close found the legend saying only CW, CWR and RTTY rows are
// not imported, where at that point EVERY row blocked: core/driver/ts590/
// caps.go publishes no ShiftOptions (§1.16 — the 50-byte record carries no
// duplex selector), so CHIRP's blank Duplex cell, its ordinary simplex row,
// met core/csvio's ShiftNone arm and was refused BLOCKING. The 07/09/2026
// fleet ruling then made that arm report NOTHING for a blank cell on a radio
// with no shift vocabulary — CHIRP's blank says nothing, so nothing is lost —
// and ordinary rows now import as simplex, leaving "off" and the three mode
// names as the only refusals (core/csvio/chirp_test.go's
// TestImportCHIRP_TS590PairBlocksCWAndRTTYRows, second and third subtests).
// The negative assertion below is what stops the intervening sentence coming
// back: a legend that says every row blocks is now the false one.
//
// THE FIFTH IS A13/A14's SECOND BRANCH, on the S row alone: channel writes
// are refused not only above firmware 2.00 but on an FV answer this
// programme cannot read as a version at all (write.go's
// firmwareBlocksWrites, whose !fvGrammarOK arm precedes the numeric test).
// The SG branches on no version at all, which is why the assertion is
// per-model rather than in the shared list.
func TestRadiotext_TS590Pair_GridLegendCarriesItsPublishedCosts(t *testing.T) {
	for _, model := range []string{"TS-590S", "TS-590SG"} {
		t.Run(model, func(t *testing.T) {
			got, ok := radiotext.For(model)
			if !ok {
				t.Fatalf("For(%q) ok = false, want true", model)
			}
			for _, want := range []string{
				// The A9 cost, its register name, and the way out.
				"A MEMORY CHANNEL READ OFF THIS RADIO IS NOT WRITTEN BACK UNTIL YOU SUPPLY ITS TRANSMIT FREQUENCY",
				"register entry A9",
				"until it is lifted on this row",
				// The clause that stops the sentence above over-claiming.
				"A SCAN RANGE IS NOT AFFECTED",
				// Decision 14's 1750 Hz receive tone.
				"decision 14",
				"1750 Hz",
				// A23: the broadest write refusal on this family.
				"ONLY FM CHANNELS ARE EVER WRITTEN",
				"register entry A23",
				// The CHIRP outcome: ordinary rows import, and the two
				// kinds that do not are both named.
				"ORDINARY ROWS DO IMPORT ON THIS RADIO",
				"imported as simplex rather than refused",
				"IS refused, because that asserts a state distinct from simplex",
			} {
				if !strings.Contains(got.GridLegendNote, want) {
					t.Errorf("GridLegendNote = %q,\nwant it to contain %q", got.GridLegendNote, want)
				}
			}
			// The legend must not still claim the tone columns are
			// unconditional: that is the sentence decision 14 contradicts.
			if strings.Contains(got.GridLegendNote, "so those columns behave like any other") {
				t.Errorf("GridLegendNote still says the tone and scan-skip columns %q — decision 14 refuses a Known tone_rx of 1750 Hz, so the unqualified claim is false", "behave like any other")
			}
			// The legend must not still say that every row blocks: that
			// was true only while the blank Duplex column was refused,
			// and it is now the inversion of the outcome rather than an
			// overstatement of it.
			if strings.Contains(got.GridLegendNote, "EVERY row is blocked") {
				t.Errorf("GridLegendNote still says %q — a blank Duplex column now imports as simplex on this family, so that sentence is false rather than merely blunt", "EVERY row is blocked")
			}
			// A13/A14's second branch, on the S row alone: the SG branches
			// on no firmware version at all, so it must NOT carry this.
			unreadable := strings.Contains(got.GridLegendNote, "cannot read as a version at all")
			if want := model == "TS-590S"; unreadable != want {
				t.Errorf("GridLegendNote mentions the unreadable-firmware refusal = %v, want %v — write.go's firmwareBlocksWrites refuses on !fvGrammarOK before it compares any number, and only on the S row", unreadable, want)
			}
		})
	}
}

// TestRadiotext_TS590SAndSGDifferInMoreThanTheModelName is the INVERSE of
// TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName, and the inversion is
// the point rather than a variation on a theme.
//
// The FTdx101 pair share one manual that prints their memory surface once,
// with no model qualifier, so their entries may differ ONLY where they name
// the model and a substitution proves it. The TS-590 pair share one manual
// that QUALIFIES BY ROW: the record position selecting filter A or B is
// guaranteed zero only on the S row's 1.xx firmware (590:1478, 590:1564), so
// the S declines to set the filter column and refuses channel writes above
// that firmware, and the SG does neither. An entry pair that differed only in
// the model name would therefore be WRONG — it would mean one of the two
// entries had been produced by substituting the other's, and one of those two
// radios would be told something the book does not say about it.
//
// So this test asserts the opposite of its FTdx101 counterpart: the
// substitution must FAIL, and it must fail on NAMED facts rather than
// anywhere at all, so that a later edit which flattened the pair into one
// prose block is caught with the specific difference it destroyed.
func TestRadiotext_TS590SAndSGDifferInMoreThanTheModelName(t *testing.T) {
	s, ok := radiotext.For("TS-590S")
	if !ok {
		t.Fatal(`For("TS-590S") ok = false, want true`)
	}
	sg, ok := radiotext.For("TS-590SG")
	if !ok {
		t.Fatal(`For("TS-590SG") ok = false, want true`)
	}
	if s == sg {
		t.Fatal("the TS-590S's and TS-590SG's entries are byte-identical — each radio's prose must at least name its own model")
	}

	// The substitution the FTdx101 pair's test requires to SUCCEED must fail
	// here. The direction is SG -> S for the reason that test's direction is
	// forced: "TS-590S" is a substring of "TS-590SG", so substituting the
	// S's name into the SG's prose would be ill-defined.
	sFields := textFields(s)
	sgFields := textFields(sg)
	substituted := 0
	for field, sgVal := range sgFields {
		if strings.ReplaceAll(sgVal, "TS-590SG", "TS-590S") == sFields[field] {
			substituted++
		}
	}
	// Some fields legitimately survive the substitution — a clearing
	// procedure and an input hint carry no row-qualified fact — so the count
	// is not pinned; naming it would be a maintenance trap. What matters is
	// that not ALL of them do, and that the fields carrying the FILTER and
	// FIRMWARE facts do not, which the named assertions below state directly.
	if substituted == len(sFields) {
		t.Error("every TS-590SG field reduces to the TS-590S's by substituting the model name — one of these entries was produced from the other, and the pair's row-qualified differences (the filter column, the firmware write refusal) have been lost")
	}

	// THE NAMED DIFFERENCES, so the assertion above cannot be satisfied by
	// some incidental wording change while the substantive facts drift into
	// agreement.
	sJoined := strings.Join(fieldValues(sFields), "\n")
	sgJoined := strings.Join(fieldValues(sgFields), "\n")
	for _, tc := range []struct {
		what  string
		sOnly string
		sgHas string
	}{
		{
			what:  "the filter column",
			sOnly: "THE FILTER COLUMN CANNOT BE SET ON THIS ROW AT ALL",
			sgHas: "THE FILTER COLUMN IS SETTABLE ON THIS ROW",
		},
		{
			what:  "the firmware write refusal",
			sOnly: "THIS ROW IS THE ONE WHERE THE ANSWER CHANGES WHAT THIS BUILD WILL DO",
			sgHas: "NOTHING ON THIS ROW BRANCHES ON THE ANSWER",
		},
	} {
		if !strings.Contains(sJoined, tc.sOnly) {
			t.Errorf("%s: the TS-590S's prose no longer says %q", tc.what, tc.sOnly)
		}
		if strings.Contains(sgJoined, tc.sOnly) {
			t.Errorf("%s: the TS-590SG's prose says %q, which is a fact about the S row alone", tc.what, tc.sOnly)
		}
		if !strings.Contains(sgJoined, tc.sgHas) {
			t.Errorf("%s: the TS-590SG's prose no longer says %q", tc.what, tc.sgHas)
		}
	}
}

// fieldValues returns a field map's values, for a whole-entry substring scan.
func fieldValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// TestRadiotext_TS890SVerbatim pins every TS-890S Text field byte for byte
// (Tier 6's second pair, landed with that row's wiring registration —
// internal/wiring's TestEverySupportedModelHasRadiotext refuses a registered
// model with no prose, which is what makes this entry part of registration
// rather than a later nicety).
//
// THE HONESTY RULE APPLIES UNCHANGED, and this row starts exactly where the
// TS-590 pair did: no Kenwood radio has ever answered a frame put to it by
// this project, only the PC control command reference is held here and not the
// instruction manual, and this row's write trials have not happened. Every
// string says what is actually known, including where something is not, and
// borrows the wording of no other entry — assertNotBorrowedFromAnyOtherModel
// is what enforces the last clause across the whole registry.
//
// WHAT THIS ENTRY CAN SAY THAT THE TS-590 PAIR'S COULD NOT: this radio can be
// written in every mode it publishes, and its transmit frequency comes back
// from the same frame as everything else — so there is no "supply the
// transmit frequency yourself" cost on a memory READ here, and no FM-only
// refusal. That is true of a read and false of a CHIRP import: a CHIRP row
// carries no transmit frequency either, and this radio's write path refuses
// it exactly as the read-back one is spared — so the entry states both. What
// REPLACES the read-side cost is the create-path cost: one frame carries a
// whole channel, so a blank target is refused rather than created, and the
// entry says so in the sentence Q10 requires.
//
// THE VOCABULARY CHECK RUNS AGAINST THIS PROSE (plan decision P16): Kenwood is
// deliberately NOT in yaesuModels, because these books say "PC control
// command" and never the Yaesu family's word.
func TestRadiotext_TS890SVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The TS-890S has no erase command this build will send, and on this radio the absence is a CHOICE OVER A PRINTED COMMAND rather than over an ambiguous side effect. This radio's own book prints a dedicated deletion command, MA5, \"Memory Channel (Channel Deletion)\", and this programme does not delete a user's channels — so it never builds that frame, and the outbound gate refuses any frame beginning MA5 besides, which means the command cannot be sent even by accident or by a caller that asked for it. That is stronger evidence than the TS-590 pair had, and it is refused for the opposite reason: there the clearing route was a reading of a sentence, here it is printed plainly and declined. A channel can be cleared only at the radio itself, and this build does not describe how: no TS-890S instruction manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel clearing procedure in the radio's own instruction manual.",
		GridLegendNote:           "Tone and Scan Skip ARE read and written on the TS-890S, as they are on the TS-590 pair and unlike the Yaesu radios this programme also supports: one memory record carries a tone mode, separate transmit and receive tone numbers and a channel-lockout flag, and one frame carries all of them. THE LOCKOUT FLAG ON THIS ROW READS \"0: Lockout OFF / 1: Lockout ON\", which is this radio's own spelling and NOT the whole family's: another Kenwood radio in this model list prints a different pair of values for the same flag, and a build that carried one radio's convention into another would be writing a byte the radio does not print. This row's codec accepts only the two values this radio's own memory-record chart prints. THIS ROW PUBLISHES SIXTEEN MODES, and every one of them can be written: unlike the TS-590 pair there is no FM-only restriction here, because the width byte this record carries has a printed meaning of its own rather than one the book confines to FM. ONE TONE VALUE IS STILL REFUSED, and it is a disagreement between this radio's own two printed tone charts rather than a limit of this build: 1750 Hz is the last entry of the chart the tone-number command prints and has no entry at all in the chart the tone-squelch command prints, so a 1750 Hz TRANSMIT tone is written normally while a 1750 Hz RECEIVE tone is refused at the write rather than sent as a number the receive chart does not print. REGISTERING THIS RADIO DOES NOT MEAN THIS BUILD CAN PROGRAMME A FRESH ONE, and that is the cost worth knowing before you plan anything. One frame carries a whole channel here, so this build reads the channel it is about to write and refuses when that read comes back blank: whether a memory-set frame can CREATE an unassigned channel is printed nowhere for this command, while five sibling commands in this same book each print an unassigned-channel prohibition of their own — so a channel that does not exist on the radio yet is refused, naming register entry A3, rather than created on an assumption. Channels the radio already holds are written normally. THE SECONDARY SIDE IS READ BUT NEVER REWRITTEN FROM NOTHING. One frame rewrites the whole record, primary side and secondary side together, and this build has a source for the primary side and none for the secondary — so before the write it compares the radio's own secondary parameters against what its frame would emit and refuses, naming the parameter and both values, rather than overwriting a side your file never described. A CHANNEL WRITTEN WHILE THE RADIO IS DISPLAYING IT MAY READ BACK OLD, and that is the radio's own printed behaviour rather than a fault in the write: \"When setting the channel currently being accessed, the new settings are reflected the next time that channel is accessed.\" A verification read of the channel on the radio's own display can therefore show the previous values; move off it and read again. AND A CHIRP FILE'S ORDINARY ROWS DO IMPORT ON THIS RADIO, with four costs. A CHIRP file's blank Duplex column is its ordinary simplex row, and although this radio declares no shift vocabulary for it to land in — its memory record carries no duplex selector of any kind — a blank column asks for nothing this radio cannot do, so such a row is imported as simplex rather than refused. A Duplex column reading \"off\" IS refused, because that asserts a state distinct from simplex — no duplex configured at all — which this record cannot carry. And CW, CWR and RTTY rows are refused on the mode: those map to the sideband-specific names CW-U, CW-L and RTTY-U, and this radio's own mode legend prints CW, CW-R, FSK and FSK-R instead. AND AN IMPORTED CHIRP CHANNEL NO LONGER NEEDS YOU TO SUPPLY EITHER TONE INDEX, since design 2026-09-12-chirp-b1 (symmetric B1, which supersedes the ruling B2 this note used to record): a CHIRP row's rToneFreq and cToneFreq columns are read even on a row whose own Tone mode does not use them, so an ordinary blank-Tone row's 88.5 fill values carry both, and one frame here carries the tone mode, the transmit tone and the receive tone together — all three are Known from the file alone, and the write reaches this radio's unverified-write consent gate rather than refusing on the tone rung. THE TRANSMIT FREQUENCY IS ALSO CARRIED, as before: a blank Duplex column is the file's own simplex statement, and this radio's memory record prints that a simplex channel's split parameters all read zero, so an import carries that value rather than leaving it unsaid. A row reading \"TSQL\" is still refused on the tone column outright, because CHIRP's tone squelch asks for a transmit-and-receive tone mode this radio's own memory chart does not print. This programme's own CSV carries all of this already. This programme's own CSV import and export are unaffected.",
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over this radio's PC-control interface on a TS-890S, so nothing here is preserved: one memory record carries a tone mode and separate transmit and receive tone numbers, and one frame carries both. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over this radio's PC-control interface on a TS-890S, so nothing here is preserved: one memory record carries a channel-lockout flag, printed \"0: Lockout OFF / 1: Lockout ON\". Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version IS readable on this radio, and the probe reads it: two frames go out, an identity request and then a version request, and the four characters that come back are printed in the report above. NOTHING ON THIS ROW BRANCHES ON THE ANSWER — no field of this radio's memory record has a firmware-conditional meaning — so the version is reported and not acted on, and a version this build cannot parse still gives a working session. THE PROBE ALSO TURNS AUTO INFORMATION OFF, AND IT DOES SO ON ONE CONNECTOR ONLY. This radio can be set separately for each of its connectors, so the session switches Auto Information off on the one it is using and leaves the others exactly as they were: a logger connected on another port will not see its own stream stop. Its opening speed of 9600 is ASSUMED, not read off the radio, and it is an operational assumption rather than a cautious one: no Kenwood book held here prints a factory speed anywhere, a wrong speed is not a safe speed but an unreachable radio, and the symptom is a timeout that looks exactly like a dead port or a bad cable. This build offers NO way to open at another speed — not on the command line and not in the window — and it never probes the port at several speeds to find out, so a TS-890S set to anything else has to be put back to 9600 at its own menu before it will answer.",
	}

	got, ok := radiotext.For("TS-890S")
	if !ok {
		t.Fatal(`For("TS-890S") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-890S\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-890S", got)
}

// TestRadiotext_TS990SVerbatim is its pair-mate's twin, on the same terms.
// See TestRadiotext_TS890SVerbatim for the honesty rule this family is
// written under, and
// TestRadiotext_TS890SAnd990SDifferInMoreThanTheModelName for what the two
// entries may NOT share.
func TestRadiotext_TS990SVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The TS-990S has no erase command this build will send, and on this radio the absence is a CHOICE OVER A PRINTED COMMAND rather than over an ambiguous side effect. This radio's own book prints a dedicated deletion command, MA5, \"Channel Deletion\", and this programme does not delete a user's channels — so it never builds that frame, and the outbound gate refuses any frame beginning MA5 besides, which means the command cannot be sent even by accident or by a caller that asked for it. That is stronger evidence than the TS-590 pair had, and it is refused for the opposite reason: there the clearing route was a reading of a sentence, here it is printed plainly and declined. A channel can be cleared only at the radio itself, and this build does not describe how: no TS-990S instruction manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel clearing procedure in the radio's own instruction manual.",
		GridLegendNote:           "Tone and Scan Skip ARE read and written on the TS-990S, as they are on the TS-590 pair and unlike the Yaesu radios this programme also supports: one memory record carries a tone mode, separate transmit and receive tone numbers and a channel-lockout flag, and one frame carries all of them. THE LOCKOUT FLAG ON THIS ROW READS \"1: Scan Lockout OFF / 2: Scan Lockout ON\", which is this radio's own spelling and not the family's — and not even this radio's own elsewhere, since its channel-lockout command a few pages later prints 0 and 1 for what looks like the same question. Another Kenwood radio in this model list prints \"0\" and \"1\" for the same flag. This build accepts only the two values the memory-record chart itself prints, so a byte from the other convention is refused rather than silently rewritten. THIS ROW PUBLISHES TWENTY-SIX MODES, and every one of them can be written: unlike the TS-590 pair there is no FM-only restriction here, because the width byte this record carries has a printed meaning of its own rather than one the book confines to FM. ONE TONE VALUE IS STILL REFUSED, and it is a disagreement between this radio's own two printed tone charts rather than a limit of this build: 1750 Hz is the last entry of the chart the tone-number command prints and has no entry at all in the chart the tone-squelch command prints, so a 1750 Hz TRANSMIT tone is written normally while a 1750 Hz RECEIVE tone is refused at the write rather than sent as a number the receive chart does not print. REGISTERING THIS RADIO DOES NOT MEAN THIS BUILD CAN PROGRAMME A FRESH ONE, and that is the cost worth knowing before you plan anything. One frame carries a whole channel here, so this build reads the channel it is about to write and refuses when that read comes back blank: whether a memory-set frame can CREATE an unassigned channel is printed nowhere for this command, while four sibling commands in this same book each print an unassigned-channel prohibition of their own — so a channel that does not exist on the radio yet is refused, naming register entry A3, rather than created on an assumption. Channels the radio already holds are written normally. THE SECONDARY SIDE IS READ BUT NEVER REWRITTEN FROM NOTHING. One frame rewrites the whole record, primary side and secondary side together, and this build has a source for the primary side and none for the secondary — so before the write it compares the radio's own secondary parameters against what its frame would emit and refuses, naming the parameter and both values, rather than overwriting a side your file never described. AND A CHANNEL WITH DUAL RECEPTION SWITCHED ON IS REFUSED OUTRIGHT: this record can flag a second RECEIVER over the second side, nothing in this programme's channel model names such a thing, and one frame rewrites the whole record — so a write to that channel would silently switch the second receiver off. It is refused instead, unconditionally, whatever the split flag beside it says. No other radio in the model list has this refusal, because no other radio in the model list has the flag. A CHANNEL WRITTEN WHILE THE RADIO IS DISPLAYING IT MAY READ BACK OLD, and that is the radio's own printed behaviour rather than a fault in the write: \"When setting the channel currently being accessed, the new settings are reflected the next time that channel is accessed.\" A verification read of the channel on the radio's own display can therefore show the previous values; move off it and read again. AND A CHIRP FILE'S ORDINARY ROWS DO IMPORT ON THIS RADIO, with four costs. A CHIRP file's blank Duplex column is its ordinary simplex row, and although this radio declares no shift vocabulary for it to land in — its memory record carries no duplex selector of any kind — a blank column asks for nothing this radio cannot do, so such a row is imported as simplex rather than refused. A Duplex column reading \"off\" IS refused, because that asserts a state distinct from simplex — no duplex configured at all — which this record cannot carry. And CW, CWR and RTTY rows are refused on the mode: those map to the sideband-specific names CW-U, CW-L and RTTY-U, and this radio's own mode legend prints CW, CW-R, FSK and FSK-R instead. AND AN IMPORTED CHIRP CHANNEL NO LONGER NEEDS YOU TO SUPPLY EITHER TONE INDEX, since design 2026-09-12-chirp-b1 (symmetric B1, which supersedes the ruling B2 this note used to record): a CHIRP row's rToneFreq and cToneFreq columns are read even on a row whose own Tone mode does not use them, so an ordinary blank-Tone row's 88.5 fill values carry both, and one frame here carries the tone mode, the transmit tone and the receive tone together — all three are Known from the file alone, and the write reaches this radio's unverified-write consent gate rather than refusing on the tone rung. THE TRANSMIT FREQUENCY IS ALSO CARRIED, as before: a blank Duplex column is the file's own simplex statement, and this radio's memory record prints that a simplex channel's split parameters all read zero, so an import carries that value rather than leaving it unsaid. A row reading \"TSQL\" is still refused on the tone column outright, because CHIRP's tone squelch asks for a transmit-and-receive tone mode this radio's own memory chart does not print. This programme's own CSV carries all of this already. This programme's own CSV import and export are unaffected.",
		ToneScanSkipVerification: "",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over this radio's PC-control interface on a TS-990S, so nothing here is preserved: one memory record carries a tone mode and separate transmit and receive tone numbers, and one frame carries both. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over this radio's PC-control interface on a TS-990S, so nothing here is preserved: one memory record carries a channel-lockout flag, printed \"1: Scan Lockout OFF / 2: Scan Lockout ON\". Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version IS readable on this radio, and the probe reads it: two frames go out, an identity request and then a version request, and the four characters that come back are printed in the report above. NOTHING ON THIS ROW BRANCHES ON THE ANSWER — no field of this radio's memory record has a firmware-conditional meaning — so the version is reported and not acted on, and a version this build cannot parse still gives a working session. THE PROBE ALSO TURNS AUTO INFORMATION OFF, AND IT DOES SO ON ONE CONNECTOR ONLY. This radio can be set separately for each of its connectors, so the session switches Auto Information off on the one it is using and leaves the others exactly as they were: a logger connected on another port will not see its own stream stop. Its opening speed of 9600 is ASSUMED, not read off the radio, and it is an operational assumption rather than a cautious one: no Kenwood book held here prints a factory speed anywhere, a wrong speed is not a safe speed but an unreachable radio, and the symptom is a timeout that looks exactly like a dead port or a bad cable. This build offers NO way to open at another speed — not on the command line and not in the window — and it never probes the port at several speeds to find out, so a TS-990S set to anything else has to be put back to 9600 at its own menu before it will answer.",
	}

	got, ok := radiotext.For("TS-990S")
	if !ok {
		t.Fatal(`For("TS-990S") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-990S\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-990S", got)
}

// TestRadiotext_TS890SAnd990SDifferInMoreThanTheModelName is the TS-590
// pair's test one pair over, and it asserts the same INVERSION for a stronger
// reason.
//
// The FTdx101 and IC-7851 pairs share one manual that prints their memory
// surface once, so their entries may differ ONLY where they name the model
// and a substitution proves it. The TS-590 pair share one manual that
// QUALIFIES BY ROW. This pair shares NEITHER: two books, two memory records
// of different shapes, two mode legends of different lengths. An entry pair
// that differed only in the model name would therefore be wrong twice over —
// it would mean one entry had been produced by substituting the other's, and
// one of these two radios would be told something its own book does not say.
//
// So the substitution must FAIL, and it must fail on NAMED facts, so that a
// later edit which flattened the pair into one prose block is caught with the
// specific difference it destroyed. The four named facts are the ones plan
// decision P16 requires the pair to differ in: the mode count, the lockout
// sentence, the dual-reception refusal and the create-path sentence's row
// scope.
func TestRadiotext_TS890SAnd990SDifferInMoreThanTheModelName(t *testing.T) {
	a, ok := radiotext.For("TS-890S")
	if !ok {
		t.Fatal(`For("TS-890S") ok = false, want true`)
	}
	b, ok := radiotext.For("TS-990S")
	if !ok {
		t.Fatal(`For("TS-990S") ok = false, want true`)
	}
	if a == b {
		t.Fatal("the TS-890S's and TS-990S's entries are byte-identical — each radio's prose must at least name its own model")
	}

	aFields := textFields(a)
	bFields := textFields(b)
	substituted := 0
	for field, bVal := range bFields {
		if strings.ReplaceAll(bVal, "TS-990S", "TS-890S") == aFields[field] {
			substituted++
		}
	}
	// Some fields legitimately survive the substitution — an input hint
	// carries no row-qualified fact — so the count is not pinned; naming it
	// would be a maintenance trap. What matters is that not ALL of them do,
	// and that the fields carrying the four named facts do not.
	if substituted == len(aFields) {
		t.Error("every TS-990S field reduces to the TS-890S's by substituting the model name — one of these entries was produced from the other, and this pair shares no book for that to be true of")
	}

	aJoined := strings.Join(fieldValues(aFields), "\n")
	bJoined := strings.Join(fieldValues(bFields), "\n")
	for _, tc := range []struct {
		what  string
		aOnly string
		bOnly string
	}{
		{
			what:  "the mode count",
			aOnly: "THIS ROW PUBLISHES SIXTEEN MODES",
			bOnly: "THIS ROW PUBLISHES TWENTY-SIX MODES",
		},
		{
			what:  "the lockout sentence",
			aOnly: `THE LOCKOUT FLAG ON THIS ROW READS "0: Lockout OFF / 1: Lockout ON"`,
			bOnly: `THE LOCKOUT FLAG ON THIS ROW READS "1: Scan Lockout OFF / 2: Scan Lockout ON"`,
		},
		{
			what: "the create-path sentence's row scope",
			// A3 is scoped PER ROW: five sibling commands print an
			// unassigned-channel prohibition in the 890S's book and four
			// in the 990S's, so each entry counts its own.
			aOnly: "while five sibling commands in this same book",
			bOnly: "while four sibling commands in this same book",
		},
	} {
		if !strings.Contains(aJoined, tc.aOnly) {
			t.Errorf("%s: the TS-890S's prose no longer says %q", tc.what, tc.aOnly)
		}
		if strings.Contains(bJoined, tc.aOnly) {
			t.Errorf("%s: the TS-990S's prose says %q, which is a fact about the 890S row alone", tc.what, tc.aOnly)
		}
		if !strings.Contains(bJoined, tc.bOnly) {
			t.Errorf("%s: the TS-990S's prose no longer says %q", tc.what, tc.bOnly)
		}
		if strings.Contains(aJoined, tc.bOnly) {
			t.Errorf("%s: the TS-890S's prose says %q, which is a fact about the 990S row alone", tc.what, tc.bOnly)
		}
	}

	// THE FOURTH NAMED FACT is one-sided by nature: the 890S's record has no
	// dual-reception flag at all, so its entry must not carry the refusal and
	// the 990S's must.
	const dual = "AND A CHANNEL WITH DUAL RECEPTION SWITCHED ON IS REFUSED OUTRIGHT"
	if !strings.Contains(bJoined, dual) {
		t.Errorf("the TS-990S's prose no longer says %q — the refusal is this row's most consequential one", dual)
	}
	if strings.Contains(aJoined, dual) {
		t.Errorf("the TS-890S's prose says %q, and this radio's record carries no dual-reception flag for it to be about", dual)
	}
}

// TestRadiotext_IC7800Verbatim pins the v1.7.0 Icom wave's first
// registration's prose byte-for-byte, and runs the standard non-borrowing
// checks against every other registered model — including the IC-7610,
// this radio's closest sibling by record shape (25 B / 2 B flat address),
// which is exactly the borrowing risk this registration carries.
func TestRadiotext_IC7800Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no CI-V memory-clear frame for the IC-7800: no builder for one exists, and no IC-7800 has ever confirmed what a clear command does, so sending one risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone is read and written for the IC-7800 over CI-V by this build, but unverified against real hardware — no IC-7800 has ever answered a frame. Scan Skip is not read or written: this radio's document maps no wire bit to it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7800 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7800's document maps no wire bit to it",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7800: this build knows of none to require. This driver talks only to CI-V address 6Ah, with no --civ-address option to change it and no way to detect a radio set to a different address. Its default baud of 19200 is unverified against real hardware, on the tier's usual footing. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7800")
	if !ok {
		t.Fatal(`For("IC-7800") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7800\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7800", got)
}

// TestRadiotext_IC7600Verbatim is TestRadiotext_IC7800Verbatim's sibling
// for the v1.7.0 Icom wave's second registration.
func TestRadiotext_IC7600Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no CI-V memory-clear frame for the IC-7600: no builder for one exists, and no IC-7600 has ever confirmed what a clear command does, so sending one risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone is read and written for the IC-7600 over CI-V by this build, but unverified against real hardware — no IC-7600 has ever answered a frame. Scan Skip is not read or written: this radio's document maps no wire bit to it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7600 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7600's document maps no wire bit to it",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7600: this build knows of none to require. This driver talks only to CI-V address 7Ah, with no --civ-address option to change it and no way to detect a radio set to a different address. Its default baud of 19200 is unverified against real hardware, on the tier's usual footing. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7600")
	if !ok {
		t.Fatal(`For("IC-7600") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7600\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7600", got)
}

// TestRadiotext_IC7410Verbatim pins the v1.7.0 Icom wave's third
// registration's prose byte-for-byte.
func TestRadiotext_IC7410Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no CI-V memory-clear frame for the IC-7410: no builder for one exists, and no IC-7410 has ever confirmed what a clear command does, so sending one risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone is read and written for the IC-7410 over CI-V by this build, but unverified against real hardware — no IC-7410 has ever answered a frame. A write that leaves the transmit frequency unset mirrors the receive frequency into it rather than refusing, per this radio's own document.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7410 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7410's document maps no wire bit to it",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7410: this build knows of none to require. This driver talks only to CI-V address 80h, with no --civ-address option to change it and no way to detect a radio set to a different address. Its default baud of 19200 is unverified against real hardware, on the tier's usual footing. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7410")
	if !ok {
		t.Fatal(`For("IC-7410") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7410\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7410", got)
}

// TestRadiotext_IC7700Verbatim pins the v1.7.0 Icom wave's fourth
// registration's prose byte-for-byte.
func TestRadiotext_IC7700Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no CI-V memory-clear frame for the IC-7700: no builder for one exists, and no IC-7700 has ever confirmed what a clear command does, so sending one risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone and the transmit (split) frequency are read and written for the IC-7700 over CI-V by this build, but unverified against real hardware — no IC-7700 has ever answered a frame. Scan Skip is not read or written: this radio's document maps no wire bit to it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7700 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7700's document maps no wire bit to it",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7700: this build knows of none to require. This driver talks only to CI-V address 74h, with no --civ-address option to change it and no way to detect a radio set to a different address. Its default baud of 19200 is unverified against real hardware, on the tier's usual footing. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7700")
	if !ok {
		t.Fatal(`For("IC-7700") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7700\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7700", got)
}

// TestRadiotext_IC9100Verbatim pins the v1.7.0 Icom wave's fifth
// registration's prose byte-for-byte.
func TestRadiotext_IC9100Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no CI-V memory-clear frame for the IC-9100: no builder for one exists, and no IC-9100 has ever confirmed what a clear command does, so sending one risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone is read and written for the IC-9100 over CI-V by this build, but unverified against real hardware — no IC-9100 has ever answered a frame. Scan Skip is not read or written: this radio's document maps no wire bit to it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-9100 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-9100's document maps no wire bit to it",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-9100: this build knows of none to require. This driver talks only to CI-V address 7Ch, with no --civ-address option to change it and no way to detect a radio set to a different address. Its default baud of 19200 is unverified against real hardware, on the tier's usual footing. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-9100")
	if !ok {
		t.Fatal(`For("IC-9100") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-9100\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-9100", got)
}

// TestRadiotext_IC7200Verbatim pins the v1.7.0 Icom wave's sixth and
// last registration's prose byte-for-byte.
func TestRadiotext_IC7200Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no CI-V memory-clear frame for the IC-7200: no builder for one exists, and no IC-7200 has ever confirmed what a clear command does, so sending one risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone and Scan Skip are not read or written for the IC-7200 over CI-V by this build: its 17-byte record maps no tone field of any kind and no scan-skip bit — this radio's own document maps no wire bit to either. This radio also has no channel-name field over CI-V at all, so this build shows no Tag column for it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not read or written over CI-V by this build — the IC-7200's document maps no wire bit to it",
			ScanSkip: "not read or written over CI-V by this build — the IC-7200's document maps no wire bit to it",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7200: this build knows of none to require. This driver talks only to CI-V address 76h, with no --civ-address option to change it and no way to detect a radio set to a different address. Its default baud of 19200 is unverified against real hardware, on the tier's usual footing. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
	}

	got, ok := radiotext.For("IC-7200")
	if !ok {
		t.Fatal(`For("IC-7200") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"IC-7200\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "IC-7200", got)
}

// TestRadiotext_FTdx5000Verbatim pins the v1.7.0 Kenwood/Yaesu wave's
// tenth row's prose byte-for-byte.
func TestRadiotext_FTdx5000Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no memory-clear frame for the FTdx5000: no builder for one exists, and no FTdx5000 has ever confirmed what a clear command does over CAT. Follow the memory-channel clear procedure in the radio's own manual instead.",
		GridLegendNote: "Tone is read and written for this radio as a live CTCSS-tone index — unlike every other registered CAT radio's fixed value — but this radio has no scan-skip position and no tag/name command anywhere in its manual, so no Tag column is shown for it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone: "read and written over CAT by this build, so nothing here is preserved: the 27-byte memory record carries a live CTCSS-tone index. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build for this radio — check the radio's display. Its default baud of 38400 is unverified against real hardware, on the tier's usual footing.",
	}

	got, ok := radiotext.For("FTdx5000")
	if !ok {
		t.Fatal(`For("FTdx5000") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"FTdx5000\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "FTdx5000", got)
}

// TestRadiotext_TS2000Verbatim pins the v1.7.0 Kenwood/Yaesu wave's first
// row's prose byte-for-byte.
func TestRadiotext_TS2000Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no memory-clear frame for the TS-2000: no builder for one exists, and no TS-2000 has ever confirmed what a clear command does over this interface. Follow the memory-channel clear procedure in the radio's own instruction manual instead.",
		GridLegendNote: "Tone and Scan Skip ARE read and written for this radio, unlike the Yaesu radios this programme also supports: its 50-byte memory record carries a channel-lockout flag and a tone mode with separate transmit and receive tone numbers, at printed positions this build's own 39-entry chart matches. A channel is written back once its transmit frequency, DCS code, REVERSE state and memory group are read from the radio first — this build preserves those raw values across a write rather than modelling a field for each.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over this radio's interface, so nothing here is preserved: the 50-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over this radio's interface, so nothing here is preserved: the 50-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build for this radio. Its opening speed of 9600 is ASSUMED, not read off the radio: no document held here prints a factory value, and a wrong speed is not a safe one but an unreachable radio — the symptom is a timeout indistinguishable from a dead port or a bad cable. This build offers no way to open at another speed and does not probe the port at several speeds to find out.",
	}

	got, ok := radiotext.For("TS-2000")
	if !ok {
		t.Fatal(`For("TS-2000") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-2000\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-2000", got)
}

// TestRadiotext_TS2000XVerbatim pins the v1.7.0 Kenwood/Yaesu wave's
// second row's prose byte-for-byte.
func TestRadiotext_TS2000XVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no memory-clear frame for the TS-2000X: no builder for one exists, and no TS-2000X has ever confirmed what a clear command does over this interface. Follow the memory-channel clear procedure in the radio's own instruction manual instead.",
		GridLegendNote: "Tone and Scan Skip ARE read and written for the TS-2000X, unlike the Yaesu radios this programme also supports: its 50-byte memory record carries a channel-lockout flag and a tone mode with separate transmit and receive tone numbers, at printed positions this build's own 39-entry chart matches. A channel is written back once its transmit frequency, DCS code, REVERSE state and memory group are read from the radio first — this build preserves those raw values across a write rather than modelling a field for each.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over the TS-2000X's interface, so nothing here is preserved: the 50-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over the TS-2000X's interface, so nothing here is preserved: the 50-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build for the TS-2000X. Its opening speed of 9600 is ASSUMED, not read off the radio: no document held here prints a factory value, and a wrong speed is not a safe one but an unreachable radio — the symptom is a timeout indistinguishable from a dead port or a bad cable. This build offers no way to open at another speed and does not probe the port at several speeds to find out.",
	}

	got, ok := radiotext.For("TS-2000X")
	if !ok {
		t.Fatal(`For("TS-2000X") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-2000X\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-2000X", got)
}

// TestRadiotext_TSB2000Verbatim pins the v1.7.0 Kenwood/Yaesu wave's
// third and last ts2000 row's prose byte-for-byte.
func TestRadiotext_TSB2000Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no memory-clear frame for the TS-B2000: no builder for one exists, and no TS-B2000 has ever confirmed what a clear command does over this interface. Follow the memory-channel clear procedure in the radio's own instruction manual instead.",
		GridLegendNote: "Tone and Scan Skip ARE read and written for the TS-B2000, unlike the Yaesu radios this programme also supports: its 50-byte memory record carries a channel-lockout flag and a tone mode with separate transmit and receive tone numbers, at printed positions this build's own 39-entry chart matches. A channel is written back once its transmit frequency, DCS code, REVERSE state and memory group are read from the radio first — this build preserves those raw values across a write rather than modelling a field for each.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over the TS-B2000's interface, so nothing here is preserved: the 50-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over the TS-B2000's interface, so nothing here is preserved: the 50-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build for the TS-B2000. Its opening speed of 9600 is ASSUMED, not read off the radio: no document held here prints a factory value, and a wrong speed is not a safe one but an unreachable radio — the symptom is a timeout indistinguishable from a dead port or a bad cable. This build offers no way to open at another speed and does not probe the port at several speeds to find out.",
	}

	got, ok := radiotext.For("TS-B2000")
	if !ok {
		t.Fatal(`For("TS-B2000") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-B2000\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-B2000", got)
}

// TestRadiotext_TS570DVerbatim pins the v1.7.0 Kenwood/Yaesu wave's
// fourth row's prose byte-for-byte.
func TestRadiotext_TS570DVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no memory-clear frame for the TS-570D: no builder for one exists, and no TS-570D has ever confirmed what a clear command does over this interface. Follow the memory-channel clear procedure in the radio's own instruction manual instead.",
		GridLegendNote: "Tone and Scan Skip ARE read and written for the TS-570D, unlike the Yaesu radios this programme also supports: its 28-byte memory record carries a channel-lockout flag and a tone mode with separate transmit and receive tone numbers, at printed positions this build's own 39-entry chart matches. This radio has no channel-name field over its interface at all, so this build shows no Tag column for it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over the TS-570D's interface, so nothing here is preserved: the 28-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over the TS-570D's interface, so nothing here is preserved: the 28-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build for the TS-570D. Its opening speed of 9600 is ASSUMED, not read off the radio: no document held here prints a factory value, and a wrong speed is not a safe one but an unreachable radio — the symptom is a timeout indistinguishable from a dead port or a bad cable. This build offers no way to open at another speed and does not probe the port at several speeds to find out.",
	}

	got, ok := radiotext.For("TS-570D")
	if !ok {
		t.Fatal(`For("TS-570D") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-570D\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-570D", got)
}

// TestRadiotext_TS570SVerbatim pins the v1.7.0 Kenwood/Yaesu wave's
// fifth row's prose byte-for-byte.
func TestRadiotext_TS570SVerbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure: "This program sends no memory-clear frame for the TS-570S: no builder for one exists, and no TS-570S has ever confirmed what a clear command does over this interface. Follow the memory-channel clear procedure in the radio's own instruction manual instead.",
		GridLegendNote: "Tone and Scan Skip ARE read and written for the TS-570S, unlike the Yaesu radios this programme also supports: its 28-byte memory record carries a channel-lockout flag and a tone mode with separate transmit and receive tone numbers, at printed positions this build's own 39-entry chart matches. This radio has no channel-name field over its interface at all, so this build shows no Tag column for it.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over the TS-570S's interface, so nothing here is preserved: the 28-byte memory record carries a tone mode and separate transmit and receive tone numbers. Whether a rewrite preserves them has never been tested on a real radio",
			ScanSkip: "read and written over the TS-570S's interface, so nothing here is preserved: the 28-byte memory record carries a channel-lockout flag. Whether a rewrite preserves it has never been tested on a real radio",
		},
		ProbeFirmwareNote: "Firmware version has no query in this build for the TS-570S. Its opening speed of 9600 is ASSUMED, not read off the radio: no document held here prints a factory value, and a wrong speed is not a safe one but an unreachable radio — the symptom is a timeout indistinguishable from a dead port or a bad cable. This build offers no way to open at another speed and does not probe the port at several speeds to find out.",
	}

	got, ok := radiotext.For("TS-570S")
	if !ok {
		t.Fatal(`For("TS-570S") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if got != want {
		t.Errorf("For(\"TS-570S\") = %#v,\nwant %#v", got, want)
	}

	assertNotBorrowedFromAnyOtherModel(t, "TS-570S", got)
}
