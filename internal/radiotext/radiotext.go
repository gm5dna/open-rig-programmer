// SPDX-License-Identifier: GPL-3.0-or-later

// Package radiotext holds the radio-specific USER-FACING PROSE this
// project's callers (the CLI, the GUI) print or display — erase
// guidance, firmware advisories, grid legends, tooltip text — keyed by
// radio model, so that a future second driver can supply its own
// strings without any caller choosing between models by import or by
// protocol knowledge. None of this is a wire-protocol fact: it lives
// here, not in core/driver or a driver subpackage, exactly because it is
// prose a human reads, never data a protocol layer consults.
//
// Task 37 (M9a-1, the radio-neutral core refactor) introduced this
// package with the FT-710's strings copied VERBATIM from their former
// homes (cmd/rigprog/write.go, cmd/rigprog/probe.go, app/send.go, and
// three of app/frontend/src/lib's Svelte components). Tasks 40-42 then
// migrated every Go call site onto radiotext.For — cmd/rigprog's write
// and probe commands, app/send.go, and app/uispec.go (which serves the
// grid/dialog prose to the frontend through UISpecView) — so this package
// is now the single authoritative home for these strings; the former Go
// consts that once held them have been deleted. Stdlib only.
package radiotext

// Text holds one radio model's user-facing prose. Every FT-710 field
// below is copied VERBATIM from where that string lives TODAY; see each
// field's own doc comment for its exact source.
type Text struct {
	// EraseProcedure is the front-panel procedure for deleting a channel
	// on the radio itself: no CAT erase command exists. Its original home
	// was cmd/rigprog/write.go's eraseFrontPanelProcedure const, since
	// deleted when that call site migrated onto radiotext.For.
	EraseProcedure string

	// FirmwareGuidance explains the firmware-version gate a session's
	// first write needs (memory CAT requires firmware >= V01-10; there
	// is no CAT query for it). Its original home was app/send.go's
	// firmwareGuidance const, since replaced by a firmwareGuidance func
	// that reads this field via radiotext.For.
	FirmwareGuidance string

	// ToneScanSkipNote is the channel grid's standing legend explaining
	// why the Tone/Scan Skip columns exist but cannot be read back over
	// CAT. Verbatim: the first sentence of
	// app/frontend/src/lib/ChannelGrid.svelte's grid-legend paragraph
	// (the one containing "aren't carried by the FT-710's CAT
	// protocol").
	ToneScanSkipNote string

	// ToneScanSkipVerification states what is and is not hardware-verified
	// about Tone/Scan Skip preservation across a rewrite for this radio.
	// Verbatim: the SECOND sentence of app/frontend/src/lib/
	// ChannelGrid.svelte's grid-legend paragraph, which task 41
	// deliberately left behind when it captured the first (ledger minor
	// m42a). It cannot stay in the frontend: it is a claim about THIS
	// radio's write trials, and for a model pinned at
	// writeTrialsComplete=false it would be an outright false statement
	// about hardware.
	ToneScanSkipVerification string

	// EraseDialogNote is the "no CAT erase command" explanation shown at
	// the moment a user asks to delete a channel
	// (DeleteConfirmDialog.svelte) or reviews a blocked-erase entry
	// before sending (SendFlowDialog.svelte) — the two components carry
	// byte-identical text (once their <strong> markup is read as plain
	// prose) today, hence one field here, not two.
	EraseDialogNote string

	// PreservationTooltips holds the per-column preserved-cell tooltips
	// the channel grid shows for its Tone and Scan Skip columns.
	// Verbatim from ChannelGrid.svelte's PRESERVED_TOOLTIP_TONE/
	// PRESERVED_TOOLTIP_SKIP consts.
	PreservationTooltips PreservationTooltips

	// FirmwarePlaceholder is the send-flow firmware-version input's
	// placeholder text. Verbatim from
	// app/frontend/src/lib/SendFlowDialog.svelte's firmware-input
	// placeholder attribute.
	FirmwarePlaceholder string

	// ProbeFirmwareNote is the firmware-version advisory "rigprog probe"
	// prints. Verbatim from cmd/rigprog/probe.go's writeProbeReport.
	ProbeFirmwareNote string
}

// PreservationTooltips models ChannelGrid.svelte's two hardware-specific,
// per-column preserved-cell tooltips as named fields — a small, fixed,
// known set; Tone and Scan Skip are the only two columns the source
// component's cellPreserved/preservedTooltip logic ever flags — rather
// than a map[string]string: named fields let a caller (and the compiler)
// catch a typo'd column key at BUILD time, where a string-keyed map
// would only surface the same mistake at runtime (a silently-empty
// lookup), and nothing here needs a map's open-ended iteration over an
// unknown key set.
type PreservationTooltips struct {
	// Tone is the tooltip for a preserved CTCSS-tone cell: not readable
	// over CAT, but hardware-verified to survive a same-data rewrite.
	// Verbatim from ChannelGrid.svelte's PRESERVED_TOOLTIP_TONE const.
	Tone string
	// ScanSkip is the tooltip for a preserved scan-skip cell: not
	// readable over CAT, and preservation across a rewrite has never
	// been probed. Verbatim from ChannelGrid.svelte's
	// PRESERVED_TOOLTIP_SKIP const.
	ScanSkip string
}

// ft710Text is the FT-710's entry — see each Text field's doc comment
// above for its exact source; every string here is byte-identical to
// its source today (TestRadiotext_FT710Verbatim pins this).
var ft710Text = Text{
	EraseProcedure:           "The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].",
	FirmwareGuidance:         "Memory CAT (read/write) requires firmware V01-10 or later. There is no CAT query for the firmware version — check the radio's front panel (or SD-card version screen) and enter it here before sending.",
	ToneScanSkipNote:         "Tone and Scan Skip aren't carried by the FT-710's CAT protocol — set them on the radio.",
	ToneScanSkipVerification: "Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip).",
	EraseDialogNote:          "The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].",
	PreservationTooltips: PreservationTooltips{
		Tone:     "not readable over CAT — preserved when writing (hardware-verified 13/07/2026)",
		ScanSkip: "not readable over CAT — preservation when writing is unverified (never probed)",
	},
	FirmwarePlaceholder: "e.g. V01-10",
	ProbeFirmwareNote:   "Firmware version has no CAT query — check the front panel: memory CAT (read/write) requires firmware V01-10 or later.",
}

// texts is the registry For consults, keyed by the exact model string a
// driver.Driver.Model() (or driver.Identity/spec.Capabilities.Model)
// call returns, e.g. "FT-710".
var texts = map[string]Text{
	"FT-710": ft710Text,
}

// For returns model's radio-specific prose. Only "FT-710" is populated
// today; any other model — including "", a future driver not yet given
// an entry, or a near-miss typo — returns the zero Text and false.
// Callers must never treat a zero Text as if it were real advisory copy.
func For(model string) (Text, bool) {
	t, ok := texts[model]
	return t, ok
}
