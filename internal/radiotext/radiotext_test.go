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
//   - EraseProcedure/EraseDialogNote: cmd/rigprog/write.go's
//     eraseFrontPanelProcedure const, and the (identical, whitespace-
//     normalised) prose in DeleteConfirmDialog.svelte and
//     SendFlowDialog.svelte.
//   - FirmwareGuidance: app/send.go's firmwareGuidance const.
//   - GridLegendNote: the first sentence of ChannelGrid.svelte's
//     grid-legend paragraph.
//   - ToneScanSkipVerification: the second sentence of ChannelGrid.svelte's
//     grid-legend paragraph (m42a: left behind in the frontend when task
//     41 captured the first sentence).
//   - PreservationTooltips: ChannelGrid.svelte's PRESERVED_TOOLTIP_TONE/
//     PRESERVED_TOOLTIP_SKIP consts.
//   - FirmwarePlaceholder: SendFlowDialog.svelte's firmware-input
//     placeholder attribute.
//   - ProbeFirmwareNote: cmd/rigprog/probe.go's writeProbeReport.
func TestRadiotext_FT710Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:           "The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].",
		FirmwareGuidance:         "Memory CAT (read/write) requires firmware V01-10 or later. There is no CAT query for the firmware version — check the radio's front panel (or SD-card version screen) and enter it here before sending.",
		GridLegendNote:           "Tone and Scan Skip aren't carried by the FT-710's CAT protocol — set them on the radio.",
		ToneScanSkipVerification: "Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip).",
		EraseDialogNote:          "The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not readable over CAT — preserved when writing (hardware-verified 13/07/2026)",
			ScanSkip: "not readable over CAT — preservation when writing is unverified (never probed)",
		},
		FirmwarePlaceholder: "e.g. V01-10",
		ProbeFirmwareNote:   "Firmware version has no CAT query — check the front panel: memory CAT (read/write) requires firmware V01-10 or later.",
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
		EraseProcedure:   "The FTdx10 has no CAT erase command, so a channel can only be deleted at the radio itself. This build does not describe how: no FTdx10 operating manual is held here, and inventing front-panel key presses would be worse than saying nothing — follow the memory-channel erase procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the FTdx10: nothing this project holds states one, and no FTdx10 has been asked. There is no CAT query for the version either — read it off the radio's front panel and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone and Scan Skip are not read or written for the FTdx10 by this build — its memory frame has no tone-number or scan-skip field (only a CTCSS on/off state byte, unverified on real hardware) — so set both on the radio.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The FTdx10 has no CAT erase command, so a channel can only be deleted at the radio itself. This build does not describe how: no FTdx10 operating manual is held here, and inventing front-panel key presses would be worse than saying nothing — follow the memory-channel erase procedure in the radio's own operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "not read or written over CAT by this build — whether a rewrite preserves it has never been tested",
			ScanSkip: "not read or written over CAT by this build — whether a rewrite preserves it has never been tested",
		},
		FirmwarePlaceholder: "as shown on the radio",
		ProbeFirmwareNote:   "Firmware version has no CAT query — check the front panel. No minimum version is established for the FTdx10: this build knows of none to require.",
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
		"FirmwareGuidance":              txt.FirmwareGuidance,
		"GridLegendNote":                txt.GridLegendNote,
		"ToneScanSkipVerification":      txt.ToneScanSkipVerification,
		"EraseDialogNote":               txt.EraseDialogNote,
		"PreservationTooltips.Tone":     txt.PreservationTooltips.Tone,
		"PreservationTooltips.ScanSkip": txt.PreservationTooltips.ScanSkip,
		"FirmwarePlaceholder":           txt.FirmwarePlaceholder,
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
// wiring.SupportedModels() — the registry itself — so a sixteenth
// registration extends every existing entry's check on its next run,
// rather than needing this file edited once per existing entry.
// ---------------------------------------------------------------------

// yaesuModels is the FIVE registered models whose radiotext entry predates
// (and is unrelated to) any CI-V vocabulary — the set catFamilyVocabulary
// below must NOT be checked against, since every one of these radios'
// prose legitimately says "CAT".
//
// The FT-891 (Tier 1) is the fifth, and it belongs here for exactly the
// reason the other four do and for no other: it speaks CAT, so its own
// prose says "CAT" throughout and the vocabulary check must skip it. It is
// otherwise an ordinary entry — one registered model, one driver package,
// one simulator.
var yaesuModels = map[string]bool{
	"FT-710": true, "FTdx10": true, "FTdx101D": true, "FTdx101MP": true,
	"FT-891": true,
}

// catFamilyVocabulary is the Yaesu CAT-protocol vocabulary every Icom
// entry's prose must never carry, on top of the other Yaesu models' own
// particulars (ownParticulars, below): these four tokens are not any ONE
// Yaesu model's evidence, they are the shared fact "this driver speaks
// CAT", which is true of all four Yaesu entries and false of every Icom
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
// manual and, deliberately, three fields whose text never names the model
// at all (PreservationTooltips.Tone, PreservationTooltips.ScanSkip,
// FirmwarePlaceholder — see wantFTdx101D/wantFTdx101MP). Their shared
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
	"FT-891":     {"FT-891"},
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
	EraseProcedure:   "The FTdx101D's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101D's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	FirmwareGuidance: "No minimum firmware version is established for the FTdx101D: nothing this project holds states one, and no FTdx101D has been asked. Its CAT command list carries no firmware-version query either, so read the version off the radio's own display and enter it here — it travels with the send as a record, and is not weighed against a threshold nobody has set.",
	GridLegendNote:   "Tone and Scan Skip are neither read nor written for the FTdx101D by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101D has ever been asked to confirm. Set both at the radio.",
	// Deliberately empty — see TestRadiotext_FTdx101DVerbatim's doc comment.
	ToneScanSkipVerification: "",
	EraseDialogNote:          "The FTdx101D's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101D's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
		ScanSkip: "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
	},
	FirmwarePlaceholder: "whatever the radio displays",
	ProbeFirmwareNote:   "Firmware version has no CAT query on the FTdx101D, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.",
}

// wantFTdx101MP is the FTDX101MP's entry, pinned VERBATIM. It is written
// out in full rather than derived from wantFTdx101D by substitution,
// deliberately: deriving it would make the verbatim pin and the D8
// substitution pin the SAME assertion, and the substitution test would then
// prove only that strings.ReplaceAll works.
var wantFTdx101MP = radiotext.Text{
	EraseProcedure:           "The FTdx101MP's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101MP's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	FirmwareGuidance:         "No minimum firmware version is established for the FTdx101MP: nothing this project holds states one, and no FTdx101MP has been asked. Its CAT command list carries no firmware-version query either, so read the version off the radio's own display and enter it here — it travels with the send as a record, and is not weighed against a threshold nobody has set.",
	GridLegendNote:           "Tone and Scan Skip are neither read nor written for the FTdx101MP by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101MP has ever been asked to confirm. Set both at the radio.",
	ToneScanSkipVerification: "",
	EraseDialogNote:          "The FTdx101MP's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101MP's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
		ScanSkip: "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
	},
	FirmwarePlaceholder: "whatever the radio displays",
	ProbeFirmwareNote:   "Firmware version has no CAT query on the FTdx101MP, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.",
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
// THE NON-BORROWING CHECK RUNS AGAINST ALL FOUR YAESU ENTRIES, not just
// the FT-710 and the FTdx10 the way the FTdx101 pair's does: this is the
// first model with no Yaesu sibling to be careful about specifically, so
// every prior entry is a borrowing risk, not just the two nearest ones.
// textFields (above) is reused unchanged — it is generic over any
// radiotext.Text value, not FTdx101-specific despite its name.
func TestRadiotext_IC7610Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:   "The IC-7610's CI-V protocol has an erase command form, but this build never sends it: no IC-7610 has ever confirmed what it does, and sending an unconfirmed erase command risks clearing the wrong channel. This build does not describe a front-panel procedure either — no IC-7610 operating manual is held here — so follow the memory-channel clear procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7610: nothing this project holds states one, and no IC-7610 has been asked. There is no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-7610 over CI-V by this build, but unverified against real hardware — no IC-7610 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-7610's CI-V protocol has an erase command form, but this build never sends it: no IC-7610 has ever confirmed what it does, and sending an unconfirmed erase command risks clearing the wrong channel. This build does not describe a front-panel procedure either — no IC-7610 operating manual is held here — so follow the memory-channel clear procedure in the radio's own operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7610 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — this radio's nearest wire nibble is a select-group marker, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the radio's display",
		ProbeFirmwareNote:   "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7610: this build knows of none to require. This driver talks only to CI-V address 98h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is itself ASSUMED, not read off the radio, since the reference guide names six rates and marks no default. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-7300's CI-V protocol prints two erase command forms — a 1A 00 set with a SELECT byte of FF, and a separate command 0B — but this build sends neither: no IC-7300 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure printed in the IC-7300's own full operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7300: nothing this project holds states one, and no IC-7300 has been asked. The IC-7300's own full manual names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-7300 over CI-V by this build, but unverified against real hardware — no IC-7300 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-7300's CI-V protocol prints two erase command forms — a 1A 00 set with a SELECT byte of FF, and a separate command 0B — but this build sends neither: no IC-7300 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure printed in the IC-7300's own full operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7300 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7300's nearest wire nibble is a select-group marker, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-7300's own display",
		ProbeFirmwareNote:   "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7300: this build knows of none to require. This driver talks only to CI-V address 94h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is a CHOICE — the highest rate this radio's document lists on both its [USB] and [REMOTE] ports — not a value read off the radio. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-7300MK2's CI-V protocol prints two erase command forms — a 1A 00 set with a truncated data area, and a separate command 0B, whose own printed row states that P1 and P2 cannot be cleared — but this build sends neither: no IC-7300MK2 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This build does not describe a front-panel procedure either — this document is a CI-V reference guide, not a full operating manual — so follow the memory-channel clear procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7300MK2: nothing this project holds states one, and no IC-7300MK2 has been asked. The IC-7300MK2's CI-V reference guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-7300MK2 over CI-V by this build, but unverified against real hardware — no IC-7300MK2 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see TestRadiotext_IC7300Verbatim's doc
		// comment; the same reasoning applies to this model.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-7300MK2's CI-V protocol prints two erase command forms — a 1A 00 set with a truncated data area, and a separate command 0B, whose own printed row states that P1 and P2 cannot be cleared — but this build sends neither: no IC-7300MK2 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This build does not describe a front-panel procedure either — this document is a CI-V reference guide, not a full operating manual — so follow the memory-channel clear procedure in the radio's own operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7300MK2 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7300MK2's nearest wire nibble is a select-group marker, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-7300MK2's own display",
		ProbeFirmwareNote:   "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7300MK2: this build knows of none to require. This driver talks only to CI-V address B6h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is a conservative derivation from a wake-up-command table this document prints for an unrelated purpose — this reference guide names no baud list and no factory default at all. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-705's CI-V protocol prints two erase command forms — a 1A 00 set carrying FF at the fifth data position, and a separate command 0B — but this build sends neither: no IC-705 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This project's own copy of the IC-705 Basic Manual is admitted for three unrelated values only, so it names no front-panel clear procedure — follow the memory-channel clear procedure in the radio's own full operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-705: nothing this project holds states one, and no IC-705 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-705 over CI-V by this build, but unverified against real hardware — no IC-705 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three select-scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-705's CI-V protocol prints two erase command forms — a 1A 00 set carrying FF at the fifth data position, and a separate command 0B — but this build sends neither: no IC-705 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This project's own copy of the IC-705 Basic Manual is admitted for three unrelated values only, so it names no front-panel clear procedure — follow the memory-channel clear procedure in the radio's own full operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-705 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-705's nearest wire nibble marks select-scan group membership, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-705's own display",
		ProbeFirmwareNote:   "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-705: this build knows of none to require. This driver talks only to CI-V address A4h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole six-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no baud information for the CI-V port at all, and the one related fact admitted from the Basic Manual is a negative: the microUSB CI-V port is baud-agnostic, which lowers the cost of a wrong guess without being evidence of one. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-9700's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF at the address's data position — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-9700: nothing this project holds states one, and no IC-9700 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-9700 over CI-V by this build, but unverified against real hardware — no IC-9700 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT-memory scan groups (★1/★2/★3), not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-9700's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF at the address's data position — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-9700 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-9700's nearest wire nibble marks one of three SELECT-memory scan groups, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-9700's own display",
		ProbeFirmwareNote:   "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-9700: this build knows of none to require. This driver talks only to CI-V address A2h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED — the middle of the six rates this document prints, and the rate Icom most commonly ships, not a value this document itself names as the default: it defers the factory setting to the radio's own instruction manual, which this project does not hold. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-905's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF after the group and channel bytes, for memory groups 00 00 ~ 00 99 only, the CALL group being excluded by the document's own words — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-905: nothing this project holds states one, and no IC-905 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-905 over CI-V by this build, but unverified against real hardware — no IC-905 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT-memory scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. DTCS is mapped too, but its three digits are OCTAL: a code this build cannot read as three octal digits comes back Unknown rather than a number, and — because this codec has no preserve-by-cache — a channel whose DTCS code is Unknown cannot be written at all until it is corrected to a valid octal value.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-905's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF after the group and channel bytes, for memory groups 00 00 ~ 00 99 only, the CALL group being excluded by the document's own words — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-905 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-905's nearest wire nibble marks one of three SELECT-memory scan groups, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-905's own display",
		ProbeFirmwareNote:   "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-905: this build knows of none to require. This driver talks only to CI-V address ACh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole five-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no rate figure anywhere, on any port's command-table page. Opening this radio also discovers its MEM bank's occupied slots by a BOUNDED walk — group 0 in full, then channel 00 of every other group, descending into the rest of a group only where its channel 00 answered — not the whole 100x100 space, and this build offers no setting that widens it: a channel stored outside that walk is simply not listed here, so its absence from the grid is not evidence that the radio's channel is empty. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
	} {
		got, ok := radiotext.For(model)
		if ok {
			t.Errorf("For(%q) ok = true, want false", model)
		}
		if got != (radiotext.Text{}) {
			t.Errorf("For(%q) = %#v, want the zero Text", model, got)
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
	FirmwareGuidance:         "No minimum firmware version is established for the IC-7851: nothing this project holds states one, and no IC-7851 has been asked. This build implements no CI-V firmware-version query either — its whole admitted command set is the identity read and the memory record — so read the version off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	GridLegendNote:           "Tone is read and written for the IC-7851 over CI-V by this build, but unverified against real hardware — no IC-7851 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT memory groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. The same holds for its data mode, with a wider consequence: a channel already set to DATA 1, DATA 2 or DATA 3 — or already in a SELECT group — cannot be written back by this build at all, because there is no honest value to preserve in a region it does not map.",
	ToneScanSkipVerification: "",
	EraseDialogNote:          "The IC-7851's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level command — but this build sends neither: no builder exists for either, and no IC-7851 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. Clear the channel at the radio instead, following the memory-channel clear procedure in its own instruction manual. The two programmed scan edges cannot be cleared at all: the radio's own memory-channel table prints their CLEAR column as \"No\".",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7851 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-7851's nearest wire nibble marks one of three SELECT memory groups, not a skip flag",
	},
	FirmwarePlaceholder: "as shown on the IC-7851's own display",
	ProbeFirmwareNote:   "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7851: this build knows of none to require. This driver talks only to CI-V address 8Eh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED, since both of this radio's printed CI-V speed settings ship on Auto and name no number to prefer. The six speeds offered are the USB port's list: on the remote-jack path with a level converter the radio stops at 19200, and this build cannot tell which path is wired. Note too that the IC-7851 and its sibling share one address, one manual and one frame shape, and this build cannot tell them apart — the model reported is the one you selected, not one it detected. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

var wantIC7850 = radiotext.Text{
	EraseProcedure:           "The IC-7850's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level command — but this build sends neither: no builder exists for either, and no IC-7850 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. Clear the channel at the radio instead, following the memory-channel clear procedure in its own instruction manual. The two programmed scan edges cannot be cleared at all: the radio's own memory-channel table prints their CLEAR column as \"No\".",
	FirmwareGuidance:         "No minimum firmware version is established for the IC-7850: nothing this project holds states one, and no IC-7850 has been asked. This build implements no CI-V firmware-version query either — its whole admitted command set is the identity read and the memory record — so read the version off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	GridLegendNote:           "Tone is read and written for the IC-7850 over CI-V by this build, but unverified against real hardware — no IC-7850 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT memory groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. The same holds for its data mode, with a wider consequence: a channel already set to DATA 1, DATA 2 or DATA 3 — or already in a SELECT group — cannot be written back by this build at all, because there is no honest value to preserve in a region it does not map.",
	ToneScanSkipVerification: "",
	EraseDialogNote:          "The IC-7850's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level command — but this build sends neither: no builder exists for either, and no IC-7850 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. Clear the channel at the radio instead, following the memory-channel clear procedure in its own instruction manual. The two programmed scan edges cannot be cleared at all: the radio's own memory-channel table prints their CLEAR column as \"No\".",
	PreservationTooltips: radiotext.PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7850 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-7850's nearest wire nibble marks one of three SELECT memory groups, not a skip flag",
	},
	FirmwarePlaceholder: "as shown on the IC-7850's own display",
	ProbeFirmwareNote:   "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7850: this build knows of none to require. This driver talks only to CI-V address 8Eh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED, since both of this radio's printed CI-V speed settings ship on Auto and name no number to prefer. The six speeds offered are the USB port's list: on the remote-jack path with a level converter the radio stops at 19200, and this build cannot tell which path is wired. Note too that the IC-7850 and its sibling share one address, one manual and one frame shape, and this build cannot tell them apart — the model reported is the one you selected, not one it detected. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-7760's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level memory-clear command — but this build sends neither: no builder exists for either, and no IC-7760 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own manual. Whether the two programmed scan edges can be cleared at all is not printed anywhere: the clear block names the 99 memory channels and says nothing about P1 or P2.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7760: nothing this project holds states one, and no IC-7760 has been asked. Its CI-V Reference Guide names no version query either, and this build's whole admitted command set is the identity read and the memory record — so read the version off the controller's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone is read and written for the IC-7760 over CI-V by this build, but unverified against real hardware — no IC-7760 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT memory groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. The same holds for its data mode, and the consequence there is wider than one column: a channel already set to DATA 1, DATA 2 or DATA 3 — or already in a SELECT group — cannot be written back by this build at all, because there is no honest value to preserve in a region it does not map.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-7760's CI-V protocol prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level memory-clear command — but this build sends neither: no builder exists for either, and no IC-7760 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own manual. Whether the two programmed scan edges can be cleared at all is not printed anywhere: the clear block names the 99 memory channels and says nothing about P1 or P2.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7760 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7760's nearest wire nibble marks one of three SELECT memory groups, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-7760's own display",
		ProbeFirmwareNote:   "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7760: this build knows of none to require. This driver talks only to CI-V address B2h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole six-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no rate figure anywhere, about any port, and its own CI-V settings block carries no speed item at all. This radio is also two boxes, and which socket you use matters: the link this build supports is the controller's rear-panel USB B connection, which enumerates as TWO virtual COM ports, and which of the two answers is a radio setting the guide prints no default for — if one port is silent, try the other before concluding the radio is wrong. The RF deck's remote jack is a second path this build does not address. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-7100's control-command chapter prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level memory-clear command — but this build sends neither: no builder exists for either, and no IC-7100 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. On this radio there is a further reason to leave them alone: the clearing block names \"memory channel 0 to 99\" where the address field itself is printed as 0001 to 0099 and omits the bank number altogether, so the printed form does not even say WHICH of the five banks it would clear. Follow the memory-channel clear procedure in the radio's own manual instead.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7100: nothing this project holds states one, and no IC-7100 has been asked. Its control-command chapter names no version query either, and this build's whole admitted command set is the identity read and the memory record — so read the version off the radio's display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established. It is worth recording accurately on this radio: the manual warns that its own default settings differ between transceiver versions.",
		GridLegendNote:   "Tone is read and written for the IC-7100 over CI-V by this build, but unverified against real hardware — no IC-7100 has ever answered a frame. Scan Skip is not: the nearest nibble in this radio's memory record marks a channel's SELECT-MEMORY membership, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. Two further states of a stored channel stop a write outright, because there is no honest value to preserve in a region this build does not map: a channel already switched INTO the select memory, and a channel stored with split on. So are the D-STAR call-sign fields and the two digital-squelch bytes — if a channel carries anything but the assumed template in those, the write is refused rather than blanking them.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-7100's control-command chapter prints two memory clear forms — a 1A 00 set carrying FF in place of the record, and a separate top-level memory-clear command — but this build sends neither: no builder exists for either, and no IC-7100 has ever confirmed what either does, so sending one risks clearing the wrong channel rather than the intended one. On this radio there is a further reason to leave them alone: the clearing block names \"memory channel 0 to 99\" where the address field itself is printed as 0001 to 0099 and omits the bank number altogether, so the printed form does not even say WHICH of the five banks it would clear. Follow the memory-channel clear procedure in the radio's own manual instead.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7100 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — the IC-7100's nearest wire nibble marks select-memory membership, not a skip flag",
		},
		FirmwarePlaceholder: "as shown on the IC-7100's own display",
		ProbeFirmwareNote:   "Firmware version has no query in this build — check the radio's display. No minimum version is established for the IC-7100: this build knows of none to require. This driver talks only to CI-V address 88h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED — it is the highest of the five speeds the manual prints, chosen because the radio's own CI-V speed item ships on Auto and names no number to prefer, and the manual warns that defaults differ between transceiver versions in any case. Two more things about this radio are worth knowing before blaming the port. Its memory list here holds the 495 ordinary channels, banks A to E, and NOTHING ELSE: the six programmed scan edges and four call channels are real channels on the radio, but the manual never says what bank number addresses them, so this build does not read them rather than guess an address. And CI-V Transceive ships ON, so the radio may be putting unsolicited frames on the bus of its own accord; they are counted and ignored, never acted on. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
		EraseProcedure:   "The IC-R8600's CI-V Reference Guide DOES print a memory clear form — a memory-set frame carrying FF where the record would go — and this build does not send it: no builder exists for it, the outbound gate admits only the identity read, a memory read and a re-validated memory set, and no IC-R8600 has ever confirmed what the printed form does, so sending one risks clearing the wrong channel rather than the intended one. The printed form also excludes group 0102, the programmed scan edges, from what it may clear, which is a scope this build could not honour in any case: it does not address that group at all. Clear a memory from the receiver's own front panel instead, following the procedure in its instruction manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-R8600: nothing this project holds states one, and no IC-R8600 has been asked. Its CI-V Reference Guide names no version query either, and this build's whole admitted command set is the identity read and the memory record — so read the version off the receiver's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established. Note what that guide does open with: it tells you to set the receiver's address, its data communication speed and its transceive function in Set mode before controlling it at all, so a receiver that answers nothing may simply not have been set up yet.",
		GridLegendNote:   "This radio is a receiver — no transmit fields: an IC-R8600 has no transmitter, and its memory record carries no transmit frequency and no transmitted tone, so those columns are absent by anatomy rather than merely unwritable. Tone squelch IS read and written over CI-V by this build, but unverified against real hardware — no IC-R8600 has ever answered a frame — and only on an FM channel, the tone mode, received tone, DTCS code and DTCS polarity all living in the FM tail alone. Scan Skip is neither read nor written, and on this receiver that refuses TWO printed settings rather than one: the first record byte carries a three-valued scan-skip choice in one half and a ten-valued select-scan group in the other, and this build maps neither, so a channel holding anything but zero in the scan-skip half is refused rather than rewritten as zero. The five digital classes cost more again — a D-STAR, P25, NXDN, DCR or dPMR channel whose squelch bytes differ from the assumed template cannot be written back at all, and neither can a change of mode INTO one of those classes, because there is no honest value to put in a tail this build does not map.",
		// Deliberately empty — see this test's doc comment.
		ToneScanSkipVerification: "",
		EraseDialogNote:          "The IC-R8600's CI-V Reference Guide DOES print a memory clear form — a memory-set frame carrying FF where the record would go — and this build does not send it: no builder exists for it, the outbound gate admits only the identity read, a memory read and a re-validated memory set, and no IC-R8600 has ever confirmed what the printed form does, so sending one risks clearing the wrong channel rather than the intended one. The printed form also excludes group 0102, the programmed scan edges, from what it may clear, which is a scope this build could not honour in any case: it does not address that group at all. Clear a memory from the receiver's own front panel instead, following the procedure in its instruction manual.",
		PreservationTooltips: radiotext.PreservationTooltips{
			Tone:     "read and written over CI-V by this build on FM channels only — unverified against real hardware, since no IC-R8600 has ever answered a frame",
			ScanSkip: "not read or written over CI-V by this build — this receiver's first record byte holds a printed scan-skip choice and a select-scan group, and neither half is mapped",
		},
		FirmwarePlaceholder: "as shown on the IC-R8600's own display",
		ProbeFirmwareNote:   "Firmware version has no query in this build — check the receiver's display. No minimum version is established for the IC-R8600: this build knows of none to require. This driver talks only to CI-V address 96h, with no --civ-address option to change it and no way to detect a receiver set to a different address; and its opening speed of 19200 is assumed on both halves — this receiver's CI-V Reference Guide prints no factory default speed, mentions no automatic setting, and never lists the rates its menu offers, so the rate AND the list it was chosen from are both assumed. The guide's own advice is to set the address, the speed and the transceive function in the receiver's Set mode before controlling it, which is the first thing to check. Two more things about this receiver are worth knowing before blaming the port. It has FOUR possible control terminals — a remote jack, a front and a rear USB port, and a network connection — and this build talks over USB, so if one port is silent, check which terminal the receiver has been told to use before concluding the cable is wrong. Neither the transceive setting nor the echo-back setting of either USB port has a printed default, so this build cannot tell you whether unsolicited frames should be expected of the receiver's own accord; any that arrive are counted and ignored, never acted on. Opening this receiver also discovers its Memories bank's occupied slots by a BOUNDED walk — group 0 in full, then channel 00 of every other group, reading the rest of a group only where its channel 00 answered — not the whole 100x100 space, and nothing on this build's command line or in its window widens it (the driver's own WithFullInventoryWalk is a Go-level option no registered composition passes): a channel stored outside that walk is simply not listed here, so its absence from the grid is not evidence that the receiver's channel is empty. If nothing answers, check the receiver's address and speed before assuming the port is wrong.",
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

// TestRadiotext_ICR8600ProbeNote_DoesNotOverstateTheWalkBound mirrors
// core/driver/icr8600/write_test.go's
// TestOccupiedSurprise_TheDiagnosticNamesTheWalkThisSessionRan (its
// "after the bounded walk" subtest, around write_test.go:511-513): that
// test fails the build if the write-refusal text carries the struck
// phrase "no setting that widens it", because icr8600.go:34 exports
// WithFullInventoryWalk and the phrase claims no setting exists at all.
// The probe note tells the same bounded-walk story and is held to the
// same honesty rule, so it is pinned here too, independently of the
// verbatim comparison above (which would also catch a regression, but
// only by chance — this test names the exact hazard).
func TestRadiotext_ICR8600ProbeNote_DoesNotOverstateTheWalkBound(t *testing.T) {
	got, ok := radiotext.For("IC-R8600")
	if !ok {
		t.Fatal(`For("IC-R8600") ok = false, want true — the model is registered in internal/wiring, so it must have prose`)
	}
	if strings.Contains(got.ProbeFirmwareNote, "no setting that widens it") {
		t.Errorf("ProbeFirmwareNote still claims no setting widens the walk, which WithFullInventoryWalk (core/driver/icr8600/icr8600.go) falsifies: %q", got.ProbeFirmwareNote)
	}
	for _, want := range []string{
		"command line",
		"WithFullInventoryWalk",
	} {
		if !strings.Contains(got.ProbeFirmwareNote, want) {
			t.Errorf("ProbeFirmwareNote = %q, want it to contain %q (the honest form used at core/driver/icr8600/write.go:215)", got.ProbeFirmwareNote, want)
		}
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
// NOT known, and borrows the wording of no other entry: not the FT-710's
// (whose hedgeless sentences are ITS hardware evidence), not the FTdx10's or
// the FTdx101 pair's (whose hedges are about different radios and different
// manuals), and not any Icom entry's.
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
		EraseProcedure:   "The FT-891 has no CAT erase command, and on this radio that absence is documented rather than merely unclaimed: the CAT manual prints the whole command set in one Control Command List and no memory-erase command appears in it. A channel can therefore be cleared only at the radio itself, and this build does not describe how — no FT-891 operating manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel erase procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the FT-891: nothing this project holds states one, and no FT-891 has been asked. There is no version query in this build's CAT vocabulary for this radio either — it is the identity read, the memory read pair, the one combined memory set and the menu read, and nothing else — so read the version off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		GridLegendNote:   "Tone and Scan Skip are neither read nor written for the FT-891 by this build: its combined memory record carries a CTCSS on/off state byte and nothing else of either kind — no tone-number byte and no scan-skip flag in any of the record's 41 positions — so set both at the radio. Two further columns behave differently here from the file you may be importing. A CHIRP file's CW, CWR and RTTY rows are not imported on this radio: they map to the sideband-specific names CW-U, CW-L and RTTY-U, and this radio's own mode legend prints CW, CW-R, RTTY-LSB and RTTY-USB instead, so such a row is blocked rather than written as a mode the radio has never been shown to have. And a transmit-clarifier flag carried in from another radio's file is refused at the write rather than sent: this radio's memory record prints that position as fixed, so there is no transmit clarifier here to set.",
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
		EraseDialogNote: "The FT-891 has no CAT erase command, and on this radio that absence is documented rather than merely unclaimed: the CAT manual prints the whole command set in one Control Command List and no memory-erase command appears in it. A channel can therefore be cleared only at the radio itself, and this build does not describe how — no FT-891 operating manual is held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Follow the memory-channel erase procedure in the radio's own operating manual.",
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
		FirmwarePlaceholder: "as shown on the FT-891's own display",
		ProbeFirmwareNote:   "Firmware version has no CAT query in this build — check the radio's display. No minimum version is established for the FT-891: this build knows of none to require. Its opening speed of 38400 is ASSUMED, not read off the radio: this radio's CAT manual prints the four rates its CAT RATE menu row offers — 4800, 9600, 19200 and 38400 — and marks none of them as the factory setting, and neither this build's command line nor its window offers a way to open at another rate, so a radio set differently has to be put back at menu 0506 before it will answer. Two more things about this radio are worth knowing before blaming the port. Its rear-panel USB socket is a built-in USB-to-dual-UART bridge, so the radio enumerates TWO serial devices, and the manual mentions the second only in the word \"Dual\" — it never says which of the two carries CAT — so if one is silent, try the other before concluding the cable or the speed is wrong. And this manual contradicts itself about READING a memory channel: its Control Command List marks the combined MEMORY WRITE & TAG command settable only, while that same command's own detail block, on the same printed page, gives it a read request and a full answer chart. This build asks the detail block's question and cross-checks the answer against the plain memory read, so a read refused for a channel that is plainly occupied is the manual's own ambiguity surfacing, not a fault in the port — one such read of a channel you know is populated is what would settle it.",
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
