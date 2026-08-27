// SPDX-License-Identifier: GPL-3.0-or-later

package radiotext_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
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
//   - ToneScanSkipNote: the first sentence of ChannelGrid.svelte's
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
		ToneScanSkipNote:         "Tone and Scan Skip aren't carried by the FT-710's CAT protocol — set them on the radio.",
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
		ToneScanSkipNote: "Tone and Scan Skip are not read or written for the FTdx10 by this build — its memory frame has no tone-number or scan-skip field (only a CTCSS on/off state byte, unverified on real hardware) — so set both on the radio.",
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

	// The FTdx10's prose must not have become the FT-710's by copy. Every
	// FIELD that both models populate must differ, and no FTdx10 string may
	// carry an FT-710 hardware particular — the two mistakes a later edit
	// would actually make.
	ft710, ok := radiotext.For("FT-710")
	if !ok {
		t.Fatal(`For("FT-710") ok = false, want true — sanity check failed`)
	}
	for _, tc := range []struct{ field, ftdx10Val, ft710Val string }{
		{"EraseProcedure", got.EraseProcedure, ft710.EraseProcedure},
		{"FirmwareGuidance", got.FirmwareGuidance, ft710.FirmwareGuidance},
		{"ToneScanSkipNote", got.ToneScanSkipNote, ft710.ToneScanSkipNote},
		{"EraseDialogNote", got.EraseDialogNote, ft710.EraseDialogNote},
		{"PreservationTooltips.Tone", got.PreservationTooltips.Tone, ft710.PreservationTooltips.Tone},
		{"PreservationTooltips.ScanSkip", got.PreservationTooltips.ScanSkip, ft710.PreservationTooltips.ScanSkip},
		{"FirmwarePlaceholder", got.FirmwarePlaceholder, ft710.FirmwarePlaceholder},
		{"ProbeFirmwareNote", got.ProbeFirmwareNote, ft710.ProbeFirmwareNote},
	} {
		if tc.ftdx10Val == tc.ft710Val {
			t.Errorf("%s is byte-identical to the FT-710's — one radio's prose must never be served as another's", tc.field)
		}
	}
	for _, particular := range []string{"V01-10", "[V/M]", "[ERASE]", "FT-710", "hardware-verified"} {
		for field, val := range map[string]string{
			"EraseProcedure":                got.EraseProcedure,
			"FirmwareGuidance":              got.FirmwareGuidance,
			"ToneScanSkipNote":              got.ToneScanSkipNote,
			"ToneScanSkipVerification":      got.ToneScanSkipVerification,
			"EraseDialogNote":               got.EraseDialogNote,
			"PreservationTooltips.Tone":     got.PreservationTooltips.Tone,
			"PreservationTooltips.ScanSkip": got.PreservationTooltips.ScanSkip,
			"FirmwarePlaceholder":           got.FirmwarePlaceholder,
			"ProbeFirmwareNote":             got.ProbeFirmwareNote,
		} {
			if strings.Contains(val, particular) {
				t.Errorf("FTdx10 %s contains %q — an FT-710 particular in the FTdx10's prose is that radio's evidence claimed for this one", field, particular)
			}
		}
	}
}

// ftdx101Fields returns every Text field of t as a named map, so the
// non-borrowing and cross-model loops below iterate one list rather than
// four hand-maintained copies. ToneScanSkipVerification is included: it is
// empty for the FTdx101s today, and a loop that skipped it would stop
// noticing the day somebody populated it with a borrowed sentence.
func ftdx101Fields(txt radiotext.Text) map[string]string {
	return map[string]string{
		"EraseProcedure":                txt.EraseProcedure,
		"FirmwareGuidance":              txt.FirmwareGuidance,
		"ToneScanSkipNote":              txt.ToneScanSkipNote,
		"ToneScanSkipVerification":      txt.ToneScanSkipVerification,
		"EraseDialogNote":               txt.EraseDialogNote,
		"PreservationTooltips.Tone":     txt.PreservationTooltips.Tone,
		"PreservationTooltips.ScanSkip": txt.PreservationTooltips.ScanSkip,
		"FirmwarePlaceholder":           txt.FirmwarePlaceholder,
		"ProbeFirmwareNote":             txt.ProbeFirmwareNote,
	}
}

// wantFTdx101D is the FTDX101D's entry, pinned VERBATIM, and it is shared
// by the verbatim test and by the D-vs-MP substitution test so that neither
// can pass against a stale copy of the other's expectation.
var wantFTdx101D = radiotext.Text{
	EraseProcedure:   "The FTdx101D's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101D's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	FirmwareGuidance: "No minimum firmware version is established for the FTdx101D: nothing this project holds states one, and no FTdx101D has been asked. Its CAT command list carries no firmware-version query either, so read the version off the radio's own display and enter it here — it travels with the send as a record, and is not weighed against a threshold nobody has set.",
	ToneScanSkipNote: "Tone and Scan Skip are neither read nor written for the FTdx101D by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101D has ever been asked to confirm. Set both at the radio.",
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
	ToneScanSkipNote:         "Tone and Scan Skip are neither read nor written for the FTdx101MP by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101MP has ever been asked to confirm. Set both at the radio.",
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
	assertFTdx101NotBorrowed(t, "FTdx101D", got)
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
	assertFTdx101NotBorrowed(t, "FTdx101MP", got)
}

// assertFTdx101NotBorrowed runs both non-borrowing checks for one FTdx101
// model: no field may be byte-identical to the FT-710's or the FTdx10's,
// and no field may carry either of those radios' PARTICULARS.
//
// BOTH DIRECTIONS MATTER AND THEY CATCH DIFFERENT MISTAKES. The
// byte-identity loop catches a wholesale copy — the edit that fills a field
// by pasting a neighbouring model's. The particulars loop catches a partial
// one, where a sentence was reworded but kept "V01-10" or "FTdx10" inside
// it, which byte-identity would sail past.
//
// The FTdx10 is in the particulars list as a literal STRING, which needs a
// word: "FTdx10" is a prefix of nothing here, since this package's own
// model names are "FTdx101D" and "FTdx101MP" and neither of THOSE appears
// in the FTdx10's prose — but "FTdx101D" does contain "FTdx10" as a
// substring, so the check is applied to the FTdx10's own name only after
// the FTdx101's model names are removed from the field. Without that step
// every field naming this radio would fail against its own name.
func assertFTdx101NotBorrowed(t *testing.T, model string, got radiotext.Text) {
	t.Helper()

	for _, other := range []string{"FT-710", "FTdx10"} {
		otherText, ok := radiotext.For(other)
		if !ok {
			t.Fatalf("For(%q) ok = false, want true — sanity check failed", other)
		}
		otherFields := ftdx101Fields(otherText)
		for field, val := range ftdx101Fields(got) {
			if val == "" {
				// ToneScanSkipVerification is empty on the FTdx10 too, and
				// two deliberate emptinesses are not a copy.
				continue
			}
			if val == otherFields[field] {
				t.Errorf("%s %s is byte-identical to the %s's — one radio's prose must never be served as another's", model, field, other)
			}
		}
	}

	// Particulars. The FT-710's are its hardware evidence; the FTdx10's are
	// its own name and its own manual's absence of one.
	for field, val := range ftdx101Fields(got) {
		bare := strings.ReplaceAll(val, model, "")
		for _, particular := range []string{"V01-10", "[V/M]", "[ERASE]", "FT-710", "hardware-verified", "FTdx10"} {
			if strings.Contains(bare, particular) {
				t.Errorf("%s %s contains %q — another radio's particular in this one's prose is that radio's evidence claimed for this one", model, field, particular)
			}
		}
	}
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

	dFields := ftdx101Fields(d)
	naming := 0
	for field, mpVal := range ftdx101Fields(mp) {
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
// ftdx101Fields (above) is reused unchanged — it is generic over any
// radiotext.Text value, not FTdx101-specific despite its name.
func TestRadiotext_IC7610Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:   "The IC-7610's CI-V protocol has an erase command form, but this build never sends it: no IC-7610 has ever confirmed what it does, and sending an unconfirmed erase command risks clearing the wrong channel. This build does not describe a front-panel procedure either — no IC-7610 operating manual is held here — so follow the memory-channel clear procedure in the radio's own operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7610: nothing this project holds states one, and no IC-7610 has been asked. There is no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		ToneScanSkipNote: "Tone is read and written for the IC-7610 over CI-V by this build, but unverified against real hardware — no IC-7610 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
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

	// Non-borrowing, against all four Yaesu entries: no field may be
	// byte-identical to, or carry a particular of, any of them.
	for _, other := range []string{"FT-710", "FTdx10", "FTdx101D", "FTdx101MP"} {
		otherText, ok := radiotext.For(other)
		if !ok {
			t.Fatalf("For(%q) ok = false, want true — sanity check failed", other)
		}
		otherFields := ftdx101Fields(otherText)
		for field, val := range ftdx101Fields(got) {
			if val == "" {
				// ToneScanSkipVerification is empty on every Yaesu entry
				// too, and shared emptiness is not a copy.
				continue
			}
			if val == otherFields[field] {
				t.Errorf("IC-7610 %s is byte-identical to the %s's — one radio's prose must never be served as another's", field, other)
			}
		}
	}
	for field, val := range ftdx101Fields(got) {
		for _, particular := range []string{
			"V01-10", "[V/M]", "[ERASE]", "FT-710", "hardware-verified",
			"FTdx10", "FTdx101D", "FTdx101MP", "CAT manual", "CAT command", "CAT query",
			// The bare token too (R1 review, fix round 1): this radio's
			// protocol is CI-V, not CAT, and the entry says so throughout
			// rather than borrowing the Yaesu radios' "CAT" vocabulary —
			// reviewer-confirmed that nothing legitimate in ic7610Text
			// contains this substring, so it is safe to check standalone
			// rather than only in the two-word forms above.
			"CAT",
		} {
			if strings.Contains(val, particular) {
				t.Errorf("IC-7610 %s contains %q — another radio's particular in this one's prose is that radio's evidence claimed for this one", field, particular)
			}
		}
	}
}

// ic7300Particulars and ic7300mk2Particulars are the non-borrowing
// particulars lists for the IC-7300 pair — extending the six shared with
// TestRadiotext_IC7610Verbatim (the Yaesu vocabulary tokens and the bare
// "CAT" token, since this pair is CI-V throughout too) with EACH MODEL'S
// OWN address hex and a check for the sibling.
//
// THE PREFIX HAZARD RUNS ONLY ONE DIRECTION, and the two lists differ
// because of it: "IC-7300" is a byte-for-byte PREFIX of "IC-7300MK2", so
// checking the IC-7300MK2's OWN prose for the bare substring "IC-7300"
// would fault on every one of its own self-references (its own entry says
// "The IC-7300MK2's..." throughout, which itself contains "IC-7300") —
// ic7300mk2Particulars therefore checks the POSSESSIVE form, "IC-7300's",
// which is NOT a substring of "IC-7300MK2's" (after "IC-7300" the MK2's
// own text always continues "MK2's", never "'s" directly), so it catches
// a genuine borrowing of the sibling's sentences without faulting on the
// MK2's own self-reference. The IC-7300's OWN prose never mentions its
// sibling at all, so ic7300Particulars has no such self-reference to
// avoid and checks the bare model name, "IC-7300MK2" — strictly STRONGER
// than the possessive form, since it also matches a possessive
// occurrence, and safe here because the hazard the possessive form exists
// to dodge does not run in this direction.
var ic7300Particulars = []string{
	"V01-10", "[V/M]", "[ERASE]", "FT-710", "hardware-verified",
	"FTdx10", "FTdx101D", "FTdx101MP", "CAT manual", "CAT command", "CAT query", "CAT",
	"IC-7610", "98h",
	"IC-7300MK2", "B6h",
}

var ic7300mk2Particulars = []string{
	"V01-10", "[V/M]", "[ERASE]", "FT-710", "hardware-verified",
	"FTdx10", "FTdx101D", "FTdx101MP", "CAT manual", "CAT command", "CAT query", "CAT",
	"IC-7610", "98h",
	"IC-7300's", "94h",
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
// THE NON-BORROWING CHECK RUNS AGAINST SIX OTHER MODELS, not four: the
// four Yaesu entries, IC-7610 (this project's first Icom registration),
// AND its own IC-7300MK2 sibling — because core/driver/ic7300/doc.go's own
// package comment states the two Icom documents this pair is built from
// are mutually silent about each other, so the sibling is exactly as much
// a borrowing risk as any Yaesu radio's prose. ftdx101Fields is reused
// unchanged — it is generic over any radiotext.Text value.
func TestRadiotext_IC7300Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:   "The IC-7300's CI-V protocol prints two erase command forms — a 1A 00 set with a SELECT byte of FF, and a separate command 0B — but this build sends neither: no IC-7300 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure printed in the IC-7300's own full operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-7300: nothing this project holds states one, and no IC-7300 has been asked. The IC-7300's own full manual names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		ToneScanSkipNote: "Tone is read and written for the IC-7300 over CI-V by this build, but unverified against real hardware — no IC-7300 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
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

	for _, other := range []string{"FT-710", "FTdx10", "FTdx101D", "FTdx101MP", "IC-7610", "IC-7300MK2"} {
		otherText, ok := radiotext.For(other)
		if !ok {
			t.Fatalf("For(%q) ok = false, want true — sanity check failed", other)
		}
		otherFields := ftdx101Fields(otherText)
		for field, val := range ftdx101Fields(got) {
			if val == "" {
				continue
			}
			if val == otherFields[field] {
				t.Errorf("IC-7300 %s is byte-identical to the %s's — one radio's prose must never be served as another's", field, other)
			}
		}
	}
	for field, val := range ftdx101Fields(got) {
		for _, particular := range ic7300Particulars {
			if strings.Contains(val, particular) {
				t.Errorf("IC-7300 %s contains %q — another radio's particular in this one's prose is that radio's evidence claimed for this one", field, particular)
			}
		}
	}
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
		ToneScanSkipNote: "Tone is read and written for the IC-7300MK2 over CI-V by this build, but unverified against real hardware — no IC-7300MK2 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
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

	for _, other := range []string{"FT-710", "FTdx10", "FTdx101D", "FTdx101MP", "IC-7610", "IC-7300"} {
		otherText, ok := radiotext.For(other)
		if !ok {
			t.Fatalf("For(%q) ok = false, want true — sanity check failed", other)
		}
		otherFields := ftdx101Fields(otherText)
		for field, val := range ftdx101Fields(got) {
			if val == "" {
				continue
			}
			if val == otherFields[field] {
				t.Errorf("IC-7300MK2 %s is byte-identical to the %s's — one radio's prose must never be served as another's", field, other)
			}
		}
	}
	for field, val := range ftdx101Fields(got) {
		for _, particular := range ic7300mk2Particulars {
			if strings.Contains(val, particular) {
				t.Errorf("IC-7300MK2 %s contains %q — another radio's particular in this one's prose is that radio's evidence claimed for this one", field, particular)
			}
		}
	}
}

// ic705Particulars is the non-borrowing particulars list for the IC-705
// (Wave 4 task R4) — extending the Yaesu-vocabulary-plus-bare-"CAT" set
// TestRadiotext_IC7610Verbatim's own list carries with the address hex and
// bare model name of EVERY OTHER registered Icom entry: the IC-705 has no
// sibling of its own to worry a prefix hazard over (unlike the IC-7300
// pair), and none of "IC-7610", "IC-7300" or "IC-7300MK2" is a substring of
// this radio's own self-references ("The IC-705's..."), so the bare forms
// are safe to check directly.
var ic705Particulars = []string{
	"V01-10", "[V/M]", "[ERASE]", "FT-710", "hardware-verified",
	"FTdx10", "FTdx101D", "FTdx101MP", "CAT manual", "CAT command", "CAT query", "CAT",
	"IC-7610", "98h",
	"IC-7300", "94h",
	"IC-7300MK2", "B6h",
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
// THE NON-BORROWING CHECK RUNS AGAINST SEVEN OTHER MODELS: the four Yaesu
// entries, and all three other registered Icom ones (IC-7610, IC-7300,
// IC-7300MK2) — this radio has no sibling of its own, so every other
// registered model is exactly as much a borrowing risk as any other.
// ftdx101Fields is reused unchanged — it is generic over any
// radiotext.Text value.
func TestRadiotext_IC705Verbatim(t *testing.T) {
	want := radiotext.Text{
		EraseProcedure:   "The IC-705's CI-V protocol prints two erase command forms — a 1A 00 set carrying FF at the fifth data position, and a separate command 0B — but this build sends neither: no IC-705 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This project's own copy of the IC-705 Basic Manual is admitted for three unrelated values only, so it names no front-panel clear procedure — follow the memory-channel clear procedure in the radio's own full operating manual.",
		FirmwareGuidance: "No minimum firmware version is established for the IC-705: nothing this project holds states one, and no IC-705 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
		ToneScanSkipNote: "Tone is read and written for the IC-705 over CI-V by this build, but unverified against real hardware — no IC-705 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three select-scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
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

	for _, other := range []string{"FT-710", "FTdx10", "FTdx101D", "FTdx101MP", "IC-7610", "IC-7300", "IC-7300MK2"} {
		otherText, ok := radiotext.For(other)
		if !ok {
			t.Fatalf("For(%q) ok = false, want true — sanity check failed", other)
		}
		otherFields := ftdx101Fields(otherText)
		for field, val := range ftdx101Fields(got) {
			if val == "" {
				continue
			}
			if val == otherFields[field] {
				t.Errorf("IC-705 %s is byte-identical to the %s's — one radio's prose must never be served as another's", field, other)
			}
		}
	}
	for field, val := range ftdx101Fields(got) {
		for _, particular := range ic705Particulars {
			if strings.Contains(val, particular) {
				t.Errorf("IC-705 %s contains %q — another radio's particular in this one's prose is that radio's evidence claimed for this one", field, particular)
			}
		}
	}
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
