// SPDX-License-Identifier: GPL-3.0-or-later

package radiotext_test

import (
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

// TestFor_UnknownModel: any model that is not EXACTLY one of this
// package's keys — "FT-710" and, since M9c-6, "FTdx10" — returns the zero
// Text and false. Callers must never mistake a zero Text for real advisory
// copy.
//
// The cases are near misses BY CONSTRUCTION, and the FTdx10's registration
// is what made that worth restating: "FT-DX10" is a plausible mis-spelling
// of a model that now really exists, and it must still miss, because the
// key is the exact string core/driver/ftdx10's Capabilities().Model
// returns. Case, punctuation and trailing whitespace are all significant —
// a lookup is a map lookup, not a fuzzy match.
func TestFor_UnknownModel(t *testing.T) {
	for _, model := range []string{"", "FT-DX10", "ft-710", "FT-710 ", "FTDX10", "ftdx10", "FTdx10 "} {
		got, ok := radiotext.For(model)
		if ok {
			t.Errorf("For(%q) ok = true, want false", model)
		}
		if got != (radiotext.Text{}) {
			t.Errorf("For(%q) = %#v, want the zero Text", model, got)
		}
	}
}
