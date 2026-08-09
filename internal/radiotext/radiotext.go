// SPDX-License-Identifier: GPL-3.0-or-later

// Package radiotext holds the radio-specific USER-FACING PROSE this
// project's callers (the CLI, the GUI) print or display — erase
// guidance, firmware advisories, grid legends, tooltip text — keyed by
// radio model, so that a second driver can supply its own strings without
// any caller choosing between models by import or by protocol knowledge —
// which is exactly what the FTdx10's entry did at M9c-6 (a second key here,
// and not one call site changed) and what the FTDX101D's and FTDX101MP's did
// again at M9d-2 (two more keys, still not one call site).
// None of this is a wire-protocol fact: it lives
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
// (M9c-6) and the FTDX101D's and FTDX101MP's (M9d-2) were written HERE
// first, for radios this project has never connected to anything, so the
// per-field doc comments below describe the
// FT-710's provenance while ftdx10Text's, ftdx101dText's and
// ftdx101mpText's own comments record what each of
// their strings may and may not claim. The four entries share the struct,
// not an evidence base — and the two FTdx101 entries share an evidence base
// with each other and with nothing else, since one manual covers both.
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
	// bank and read back Unknown). The frame-level truth, per the
	// driver's own register (core/driver/ftdx10/doc.go): the combined MT
	// record carries a CTCSS STATE byte (off/enc-dec/enc, with P9
	// documented fixed "00"), and NO tone-number byte and NO scan-skip
	// flag exist anywhere in it — so a per-channel tone frequency or
	// skip marking cannot travel over this frame at all, and whether the
	// state byte means anything live is unverified. An earlier wording
	// here claimed the frame "carries the bytes" for both fields; the
	// M9c-6 milestone review caught it contradicting the register.
	ToneScanSkipNote: "Tone and Scan Skip are not read or written for the FTdx10 by this build — its memory frame has no tone-number or scan-skip field (only a CTCSS on/off state byte, unverified on real hardware) — so set both on the radio.",
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

// ftdx101dText and ftdx101mpText are the FTDX101D's and FTDX101MP's entries
// (M9d-2 task 7, landed with those models' wiring registration —
// internal/wiring's TestEverySupportedModelHasRadiotext refuses a registered
// model with no prose, which is what makes these entries part of
// registration rather than a later nicety).
//
// THE HONESTY RULE APPLIES UNCHANGED, and doubly here. NOTHING BELOW IS
// INVENTED. No FTDX101 of either model has ever been asked anything by this
// project (core/driver/ftdx101/doc.go), no FTDX101 OPERATING manual is held
// — only the CAT Operation Reference Manual, rev 2308-L, which Yaesu prints
// ONCE for the pair — and no write trial has happened on either radio
// (writeTrialsCompleteD and writeTrialsCompleteMP are both false). Every
// string therefore says what is actually known, including where something
// is NOT known, and borrows the wording of neither the FT-710 (whose
// hedgeless sentences are ITS hardware evidence) nor the FTdx10 (whose
// hedges are about a different radio and a different manual, and would
// become claims about these two the moment they were copied).
// assertFTdx101NotBorrowed (radiotext_test.go), called from BOTH verbatim
// tests below, pins the non-borrowing mechanically and field by field: no
// field may be byte-identical to the FT-710's or the FTdx10's, and none may
// carry either radio's particulars.
//
// WHAT THESE ENTRIES CAN SAY THAT THE FTdx10's CANNOT is the point of
// writing them fresh rather than adapting: this radio's CAT manual carries
// a complete command availability table (layout 236-337), so the absence of
// an erase command and the absence of a firmware-version query are
// MANUAL-EVIDENCED here rather than merely unclaimed, and the same table's
// connection section documents a two-port arrangement that changes what
// silence on a port means.
//
// D AND MP PROSE ARE IDENTICAL EXCEPT WHERE THEY NAME THE MODEL (plan D8),
// and that is a claim about the EVIDENCE, not a shortcut: the two radios
// share one manual, one dialect config, one simulator and one driver
// implementation, and differ on the wire in the ID answer alone. There is
// no fact in any of these eight fields that is true of one and not the
// other, so any divergence beyond the model's name would be an invention.
// TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName pins it by
// substitution — replacing "FTdx101MP" with "FTdx101D" throughout the MP's
// entry must reproduce the D's exactly — so adding a genuinely
// model-specific sentence later is a deliberate edit there, not a drift.
//
// TestRadiotext_FTdx101DVerbatim and TestRadiotext_FTdx101MPVerbatim pin
// every string, so a well-meant later edit that firmed up a hedge fails
// there rather than quietly telling a user something no one established.
var ftdx101dText = Text{
	// The erase absence is MANUAL-EVIDENCED for this radio, not merely
	// unclaimed: the CAT manual's command availability table (layout
	// 236-337) lists the whole command set and contains no erase command
	// for a memory channel (matrix §2.3). What is NOT held is the
	// OPERATING manual, so the front-panel procedure is unknown and the
	// user is sent to the document that has it. The FT-710's [V/M]/[ERASE]
	// sequence is an FT-710 operating-manual fact and is not transferable.
	EraseProcedure: "The FTdx101D's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101D's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	// Two separate absences, and both are stated because they answer
	// different questions. There is no minimum-version THRESHOLD for this
	// radio (nothing this project holds states one), and there is no CAT
	// QUERY for the version either — the same command availability table
	// carries no firmware-version command, and neither RI (RADIO
	// INFORMATION) nor RS (RADIO STATUS) is one (their frame pages at
	// layout 1637 and 1675 define P1 as Hi-SWR/REC/PLAY and
	// NORMAL/MENU mode). core/clone's firmware gate enforces PRESENCE of a
	// confirmed version only (ExecuteOptions.FirmwareConfirmed), never a
	// threshold, so "recorded with the send" describes what actually
	// happens to the value rather than promising a check.
	FirmwareGuidance: "No minimum firmware version is established for the FTdx101D: nothing this project holds states one, and no FTdx101D has been asked. Its CAT command list carries no firmware-version query either, so read the version off the radio's own display and enter it here — it travels with the send as a record, and is not weighed against a threshold nobody has set.",
	// The frame-level facts are the driver's own (core/driver/ftdx101 and
	// matrix §2.2): the combined MT record accounts for all 41 of its
	// positions, it carries a CTCSS STATE byte (P8) and a P9 documented
	// fixed "00", and it has NO tone-number byte and NO scan-skip flag
	// anywhere. So a per-channel tone frequency or skip marking cannot
	// travel over this frame at all. What is ASSUMED, and therefore not
	// claimed here, is that no OTHER command in this manual could reach
	// them — the FT-710's finding that none can is that radio's, and the
	// hedge "by this build" is what keeps this sentence true either way.
	ToneScanSkipNote: "Tone and Scan Skip are neither read nor written for the FTdx101D by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101D has ever been asked to confirm. Set both at the radio.",
	// DELIBERATELY EMPTY, exactly as the FTdx10's is and for the same
	// reason: this field states what IS and is NOT hardware-verified about
	// preservation across a rewrite, and with writeTrialsCompleteD false
	// there is no verification of any kind to report. Any sentence here
	// would be a hardware claim about a radio this project has never
	// written to. internal/wiring's TestEverySupportedModelHasRadiotext
	// requires EraseProcedure, FirmwareGuidance and ProbeFirmwareNote and
	// deliberately excludes this field; the FTDX101D's own write trials are
	// what fill it in, and the MP's will not fill it in for the D.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as the FT-710's and the FTdx10's
	// are: the delete dialog and the blocked-erase review answer the same
	// question, and splitting the wording would only invite one copy to
	// drift into a procedure the other refuses to state.
	EraseDialogNote: "The FTdx101D's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101D's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	// IDENTICAL to each other, because the evidence is identically absent —
	// unlike the FT-710's pair, which differ precisely because its Tone
	// finding is hardware-verified and its Scan Skip one is not. They must
	// stop being identical the moment a trial on THIS model distinguishes
	// them; the verbatim pin is what forces that edit to be deliberate.
	PreservationTooltips: PreservationTooltips{
		Tone:     "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
		ScanSkip: "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
	},
	// A placeholder LABEL, not an example. The FT-710's "e.g. V01-10" shows
	// that radio's documented version format; no FTDX101 version string has
	// been seen here and none is printed in the CAT manual, so there is no
	// format to exemplify.
	FirmwarePlaceholder: "whatever the radio displays",
	// THE TWO-PORT CAVEAT IS THE POINT OF THIS FIELD FOR THIS RADIO (matrix
	// §3.12; the passage itself is at layout 73-76, which is where the
	// sentences quoted below actually sit — the matrix cites 75-79, and the
	// discrepancy is raised for the milestone review rather than resolved
	// here): the manual states that the radio "contains two
	// virtual COM ports, an Enhanced COM Port and a Standard COM Port", the
	// Enhanced one for CAT communications and the Standard one for TX
	// control (PTT, CW keying, digital-mode operation). This project opens
	// whichever port it is given and this manual gives no way to detect
	// which one, so a user on the Standard port gets silence that looks
	// exactly like a wrong baud rate or a framing mismatch — and probe is
	// where that silence is first met. The firmware halves are the same two
	// absences FirmwareGuidance states, kept here because probe's report is
	// read by someone who may never open the send flow.
	ProbeFirmwareNote: "Firmware version has no CAT query on the FTdx101D, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.",
}

// ftdx101mpText is the FTDX101MP's entry. See ftdx101dText's doc comment
// for the honesty rule, the evidence base and the D8 identical-prose rule
// that governs both — every per-field justification there applies here
// unchanged, with writeTrialsCompleteMP in place of writeTrialsCompleteD,
// and is deliberately not restated field by field so the two cannot drift
// into disagreeing about their own reasons.
//
// The ONE thing worth restating: a capture from an FTDX101D lifts nothing
// here. The two radios share a manual, not a serial port.
var ftdx101mpText = Text{
	EraseProcedure:           "The FTdx101MP's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101MP's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	FirmwareGuidance:         "No minimum firmware version is established for the FTdx101MP: nothing this project holds states one, and no FTdx101MP has been asked. Its CAT command list carries no firmware-version query either, so read the version off the radio's own display and enter it here — it travels with the send as a record, and is not weighed against a threshold nobody has set.",
	ToneScanSkipNote:         "Tone and Scan Skip are neither read nor written for the FTdx101MP by this build: its memory frame has no tone-number byte and no scan-skip flag, only a CTCSS on/off state byte that no FTdx101MP has ever been asked to confirm. Set both at the radio.",
	ToneScanSkipVerification: "",
	EraseDialogNote:          "The FTdx101MP's CAT command set has no erase command — its CAT manual lists the whole set, and there is none — so a memory channel can only be cleared at the radio itself. This build does not say how: the FTdx101MP's operating manual is not held here, and inventing front-panel key presses for a radio nobody here has touched would be worse than admitting the gap. Use the memory-channel erase procedure in the radio's own operating manual.",
	PreservationTooltips: PreservationTooltips{
		Tone:     "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
		ScanSkip: "outside this build's CAT surface — no trial has established whether a rewrite leaves it alone",
	},
	FirmwarePlaceholder: "whatever the radio displays",
	ProbeFirmwareNote:   "Firmware version has no CAT query on the FTdx101MP, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.",
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
// The keys are BARE STRING LITERALS, deliberately, and not
// internal/wiring's exported model constants: this package must not import
// internal/wiring (the dependency runs the other way — wiring's
// TestEverySupportedModelHasRadiotext consults radiotext.For, so importing
// back would be a cycle), and it holds no model constants of its own to
// share. The two spellings are kept in agreement by that test, which walks
// every registered model and fails on a missing entry.
var texts = map[string]Text{
	"FT-710":    ft710Text,
	"FTdx10":    ftdx10Text,
	"FTdx101D":  ftdx101dText,
	"FTdx101MP": ftdx101mpText,
}

// For returns model's radio-specific prose. "FT-710", "FTdx10", "FTdx101D"
// and "FTdx101MP" are populated — the four models internal/wiring registers;
// any other model — including "", a future driver not yet given an entry, or
// a near-miss typo ("FT-DX10", say) — returns the zero Text and false.
// Callers must never treat a zero Text as if it were real advisory copy.
//
// THE MATCH IS EXACT AND CASE-SENSITIVE, and for the FTDX101 pair that is
// load-bearing rather than incidental: "FTDX101D" is the spelling the
// radio's own CAT manual uses throughout, so it is the near-miss a person
// is most likely to type, and it must return false. A silent fall-through
// to the D's prose would serve MP users the D's model name; a silent
// fall-through to a zero Text would serve blank advisories. Neither is
// acceptable, so the only safe answer to a spelling this registry does not
// hold is "no".
func For(model string) (Text, bool) {
	t, ok := texts[model]
	return t, ok
}
