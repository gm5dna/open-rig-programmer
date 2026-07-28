// SPDX-License-Identifier: GPL-3.0-or-later

package radiotext_test

import (
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

// TestFor_UnknownModel: any model other than the exact string "FT-710"
// — including a near-miss case, and the empty string — returns the zero
// Text and false. Callers must never mistake a zero Text for real
// advisory copy.
func TestFor_UnknownModel(t *testing.T) {
	for _, model := range []string{"", "FT-DX10", "ft-710", "FT-710 "} {
		got, ok := radiotext.For(model)
		if ok {
			t.Errorf("For(%q) ok = true, want false", model)
		}
		if got != (radiotext.Text{}) {
			t.Errorf("For(%q) = %#v, want the zero Text", model, got)
		}
	}
}
