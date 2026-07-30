// SPDX-License-Identifier: GPL-3.0-or-later

// Package radiotext holds the radio-specific USER-FACING PROSE this
// project's callers (the CLI, the GUI) print or display — erase
// guidance, firmware advisories, grid legends, tooltip text — keyed by
// radio model, so that a second driver can supply its own strings without
// any caller choosing between models by import or by protocol knowledge —
// which is exactly what the FTdx10's entry did at M9c-6: a second key here,
// and not one call site changed. None of this is a wire-protocol fact: it lives
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
//
// A model registered LATER has no such source to copy. The FTdx10's entry
// (M9c-6) was written HERE first, for a radio this project has never
// connected to anything, so the per-field doc comments below describe the
// FT-710's provenance while ftdx10Text's own comments record what each of
// its strings may and may not claim. The two entries share the struct, not
// an evidence base.
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

// ftdx10Text is the FTdx10's entry (M9c-6 task 6, landed with that model's
// wiring registration — internal/wiring's
// TestEverySupportedModelHasRadiotext refuses a registered model with no
// prose, which is what makes this entry part of registration rather than a
// later nicety).
//
// THE HONESTY RULE, and it is the whole character of this entry: NOTHING
// HERE IS INVENTED. No FTdx10 has ever been asked anything by this project
// (core/driver/ftdx10/doc.go), no FTdx10 operating manual is held, and no
// write trial has happened (that driver's writeTrialsComplete is false).
// Every string below therefore says what is actually known — including,
// repeatedly, that something is NOT known — and never borrows the FT-710's
// wording, which is that radio's evidence and would become this radio's
// claim the moment it were copied. A future Stage R/W session is what
// replaces "never tested" with a finding; until then this prose is correct
// precisely because it promises nothing.
//
// TestRadiotext_FTdx10Verbatim pins every string, so a well-meant later
// edit that firmed up a hedge would fail there rather than quietly tell a
// user something no one established.
var ftdx10Text = Text{
	// No CAT erase command (this driver claims none: core/driver/ftdx10's
	// write ladder refuses an erase naming spec.FieldErase), and NO
	// front-panel procedure is stated. The FT-710's [V/M]/[ERASE] sequence
	// is an FT-710 OPERATING-manual fact; the FTdx10's operating manual is
	// not held, so the user is sent to it rather than handed invented key
	// presses for a radio nobody here has touched.
	EraseProcedure: "The FTdx10 has no CAT erase command, so a channel can only be deleted at the radio itself. This build does not describe how: no FTdx10 operating manual is held here, and inventing front-panel key presses would be worse than saying nothing — follow the memory-channel erase procedure in the radio's own operating manual.",
	// No minimum-firmware fact is established for this radio, and that is
	// exactly what this says. The FT-710's V01-10 threshold came from ITS
	// documentation; nothing states an FTdx10 equivalent, and core/clone's
	// firmware gate enforces PRESENCE of a confirmed version only (see
	// ExecuteOptions.FirmwareConfirmed), never a threshold — so recording
	// what the front panel shows is a true description of what happens to
	// the value, not a promise that it is checked.
	FirmwareGuidance: "No minimum firmware version is established for the FTdx10: nothing this project holds states one, and no FTdx10 has been asked. There is no CAT query for the version either — read it off the radio's front panel and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone/Scan Skip: this DRIVER does not read or write them over CAT
	// (spec D-tone-skip — both fields carry a zero FieldSupport on every
	// bank and read back Unknown). Deliberately NOT the FT-710's claim
	// that the protocol does not carry them: the FTdx10's combined memory
	// frame HAS those bytes, and what a radio does with them is simply
	// unverified. The distinction is the difference between a protocol
	// fact and this build's own scope.
	ToneScanSkipNote: "Tone and Scan Skip are not read or written for the FTdx10 by this build — its memory frame carries the bytes, but no radio has been asked what it does with them — so set both on the radio.",
	// DELIBERATELY EMPTY, and it is the one field that must stay empty for
	// now. It states what IS and is NOT hardware-verified about
	// preservation across a rewrite, and for a model pinned at
	// writeTrialsComplete=false there is no verification of any kind to
	// report: any sentence here would be a hardware claim. internal/wiring's
	// TestEverySupportedModelHasRadiotext anticipates exactly this — it
	// requires EraseProcedure, FirmwareGuidance and ProbeFirmwareNote and
	// deliberately excludes this field, naming this radio as the reason
	// (see its doc comment). The FTdx10's own write trials are what fill
	// it in.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as the FT-710's is: the delete
	// dialog and the blocked-erase review answer the same question, and
	// splitting the wording would only invite one copy to drift into a
	// procedure the other refuses to state.
	EraseDialogNote: "The FTdx10 has no CAT erase command, so a channel can only be deleted at the radio itself. This build does not describe how: no FTdx10 operating manual is held here, and inventing front-panel key presses would be worse than saying nothing — follow the memory-channel erase procedure in the radio's own operating manual.",
	// The two tooltips are IDENTICAL because the evidence is identically
	// absent — unlike the FT-710's pair, which differ precisely because its
	// Tone finding is hardware-verified and its Scan Skip one is not. They
	// must stop being identical the moment a trial distinguishes them; the
	// verbatim pin is what forces that edit to be deliberate.
	PreservationTooltips: PreservationTooltips{
		Tone:     "not read or written over CAT by this build — whether a rewrite preserves it has never been tested",
		ScanSkip: "not read or written over CAT by this build — whether a rewrite preserves it has never been tested",
	},
	// A placeholder LABEL, not an example: the FT-710's "e.g. V01-10"
	// shows that radio's documented version format, and no FTdx10 version
	// string has been seen here, so there is no format to exemplify.
	FirmwarePlaceholder: "as shown on the radio",
	// The no-CAT-query half is a CHART fact — the FTdx10 CAT manual, like
	// the FT-710's, documents no firmware-version command — and it is the
	// only part of the FT-710's note whose substance transfers. The
	// threshold half does not, so it is replaced by the absence.
	ProbeFirmwareNote: "Firmware version has no CAT query — check the front panel. No minimum version is established for the FTdx10: this build knows of none to require.",
}

// texts is the registry For consults, keyed by the exact model string a
// driver.Driver.Model() (or driver.Identity/spec.Capabilities.Model)
// call returns, e.g. "FT-710".
//
// EVERY model internal/wiring registers must appear here — that package's
// TestEverySupportedModelHasRadiotext fails a registration whose prose is
// missing, because a caller with no entry serves BLANK advisories rather
// than failing (cmd/rigprog's erase procedure, probe's firmware note,
// app/uispec.go's grid legend all degrade to "").
var texts = map[string]Text{
	"FT-710": ft710Text,
	"FTdx10": ftdx10Text,
}

// For returns model's radio-specific prose. "FT-710" and "FTdx10" are
// populated — the two models internal/wiring registers; any other model —
// including "", a future driver not yet given an entry, or a near-miss typo
// ("FT-DX10", say) — returns the zero Text and false.
// Callers must never treat a zero Text as if it were real advisory copy.
func For(model string) (Text, bool) {
	t, ok := texts[model]
	return t, ok
}
