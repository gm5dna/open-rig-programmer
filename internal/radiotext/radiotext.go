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

// UnverifiedWriteWarningTemplate is the arming dialogue's body — the text
// a user is shown at the moment they are asked to consent to writes this
// project has never proved on a real radio. %s is the model name,
// substituted by the app layer (the CLI states the same facts in its own
// confirmation line, which is shaped by a terminal rather than a dialogue).
//
// Deliberately NOT a Text field, and this is the one string in this
// package that is not: it is model-generic by construction — the model
// name is the only per-model fact in it — and the per-model Text structs
// are pinned by VERBATIM whole-struct tests plus a non-borrowing loop that
// refuses two models sharing a string. An identical sentence in four
// entries would fight both, and four hand-copied near-identical
// paragraphs about hardware risk is exactly the drift that discipline
// exists to prevent. A radio that one day needs its OWN wording here can
// have a Text field then; nothing about this const forecloses it.
//
// All four elements the consent spec requires are present: it names the
// radio (the substitution), states that this project has never written to
// one, notes that every write is read back and compared, and warns that a
// misinterpreted frame could corrupt the targeted memory channel. See
// internal/radiotext's own
// TestUnverifiedWriteWarningTemplate_CarriesItsFourElements.
const UnverifiedWriteWarningTemplate = "This project has never written to a real %s. Enabling unverified writes sends memory-write commands that are documented in the manufacturer's CAT reference and exercised against a simulator, but have not been proven on real hardware. Every write is read back and compared, and stops on any mismatch — but a misinterpreted frame could corrupt the targeted memory channel. You can revoke this at any time."

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
	// §3.12; the passage is at layout 75-79, exactly as the matrix cites it —
	// 75 is the two-ports sentence quoted below, 76 "These ports offer the
	// following functions:", 77-78 the two function bullets, 79 the worked
	// COM5/COM6 example. An earlier version of this comment cited a lower
	// range and flagged a discrepancy with the matrix; the M9d-2 milestone
	// review settled it by re-measuring the extraction directly, the matrix
	// was right, and the flag is gone): the manual states that the radio
	// "contains two virtual COM ports, an Enhanced COM Port and a Standard
	// COM Port", the Enhanced one for CAT communications and the Standard
	// one for TX control (PTT, CW keying, digital-mode operation). This
	// project opens whichever port it is given and this manual gives no way
	// to detect which one, so a user on the Standard port gets silence that
	// looks exactly like a wrong baud rate or a framing mismatch — and probe
	// is where that silence is first met. The firmware halves are the same two
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

// ic7610Text is the IC-7610's entry (Wave 4 task R1, this project's first
// non-Yaesu registration), landed with that model's wiring registration —
// internal/wiring's TestEverySupportedModelHasRadiotext refuses a
// registered model with no prose, which is what makes this entry part of
// registration rather than a later nicety, exactly as it was for the
// three Yaesu registrations above.
//
// THE HONESTY RULE APPLIES UNCHANGED, and doubly here: NOTHING BELOW IS
// INVENTED, and every field says what is actually known, including where
// something is not known. No IC-7610 has ever been asked anything by this
// project (core/driver/ic7610/doc.go) — every value in that driver comes
// from the IC-7610 CI-V Reference Guide rev 4 and the project's own
// capability matrix, none from a hardware finding — and no write trial has
// happened (writeTrialsComplete, core/driver/ic7610/caps.go, is false).
// This entry borrows nothing from ft710Text, ftdx10Text, ftdx101dText or
// ftdx101mpText: those are Yaesu radios' CAT evidence, and this is a
// different manufacturer on a different wire protocol (CI-V, not CAT) —
// copying a Yaesu hedge or claim across would misattribute one radio's
// evidence to a radio in an entirely different family.
// TestRadiotext_IC7610Verbatim pins every string, and its own
// non-borrowing check refuses any field that is byte-identical to, or
// carries a particular of, any of the four Yaesu entries.
//
// WHAT IS DIFFERENT ABOUT THIS RADIO, AND WHY IT SHOWS IN THE PROSE:
//
//   - The protocol is CI-V, not CAT, and every field below says so rather
//     than reusing the Yaesu radios' "CAT" wording — a small word, but the
//     kind of borrowed vocabulary that would misdescribe this radio's own
//     transport to a reader who never opens core/driver/ic7610/doc.go.
//   - This driver's CI-V address is fixed at 98h, built into every frame
//     it sends, with no --civ-address option to change it and no way to
//     detect a radio set to a different address (core/driver/ic7610/
//     doc.go, "THE TWO LIMITATIONS, STATED PLAINLY"). A radio at any other
//     address — including a DIFFERENT Icom model at ITS factory address —
//     times out identically to no radio being there at all, which
//     ProbeFirmwareNote states because probe is where a user meets that
//     silence first.
//   - The default baud, 19200, is an ASSUMED choice among six the document
//     names and defaults none of (core/driver/ic7610/doc.go, "THE DEFAULT
//     BAUD (OQ2)") — recorded as assumed here for the same reason the
//     driver's own doc.go records it, and because a user meeting silence
//     at Open needs to know the guess could be the reason.
//   - Tone IS mapped on the wire (the 1A 00 record's tone-mode nibble and
//     two tone-frequency spans, core/driver/ic7610/caps.go's bankFields) —
//     unlike every Yaesu radio registered so far, where tone_mode/tone_tx/
//     tone_rx are outside the CAT frame entirely — so this radio's tone
//     note says the opposite of theirs. Scan Skip is NOT mapped: the
//     nearest wire nibble on this radio is a four-valued SELECT-group
//     marker (matrix §3.16 ADDED-1), not a skip flag, so a Scan Skip value
//     is refused before anything reaches the radio rather than being
//     written as something it is not (adjudication R6, ruling E6).
var ic7610Text = Text{
	// The CI-V protocol has an erase command SHAPE (1A 00 <ch> FF, and a
	// separate command 0B) — unlike every Yaesu radio registered so far,
	// which has none at all — but this build sends neither: no IC-7610 has
	// ever confirmed what either does, and sending an unconfirmed erase
	// command risks clearing the wrong channel rather than the intended
	// one. FieldErase carries the zero FieldSupport here on both banks
	// (core/driver/ic7610/caps.go's bankFields, ruling E6's third
	// unmapped-region citation), which is what makes core/clone's
	// DiffErased branch unreachable for this model. No IC-7610 operating
	// manual is held here either, so the front-panel procedure is unknown
	// and the user is sent to the document that has it.
	EraseProcedure: "The IC-7610's CI-V protocol has an erase command form, but this build never sends it: no IC-7610 has ever confirmed what it does, and sending an unconfirmed erase command risks clearing the wrong channel. This build does not describe a front-panel procedure either — no IC-7610 operating manual is held here — so follow the memory-channel clear procedure in the radio's own operating manual.",
	// No minimum-firmware fact is established for this radio: the IC-7610
	// CI-V Reference Guide names no firmware-version command anywhere
	// (searched: "firmware" appears nowhere in it), so there is no CI-V
	// query either, on the same footing as every Yaesu radio's own
	// firmware note.
	FirmwareGuidance: "No minimum firmware version is established for the IC-7610: nothing this project holds states one, and no IC-7610 has been asked. There is no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone IS read and written for this radio (unlike every Yaesu radio
	// registered so far) — over CI-V, unverified against real hardware,
	// since no IC-7610 has ever answered a frame. Scan Skip is not: this
	// radio's nearest wire nibble is a select-group marker, not a skip flag,
	// and setting one is refused before anything reaches the radio.
	ToneScanSkipNote: "Tone is read and written for the IC-7610 over CI-V by this build, but unverified against real hardware — no IC-7610 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
	// DELIBERATELY EMPTY, exactly as every Yaesu entry's is and for the
	// same reason: this field states what IS and is NOT hardware-verified
	// about preservation across a rewrite, and with writeTrialsComplete
	// false there is no verification of any kind to report. Any sentence
	// here would be a hardware claim about a radio this project has never
	// written to. internal/wiring's TestEverySupportedModelHasRadiotext
	// requires EraseProcedure, FirmwareGuidance and ProbeFirmwareNote and
	// deliberately excludes this field; the IC-7610's own write trials are
	// what fill it in.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as every other model's is: the
	// delete dialog and the blocked-erase review answer the same question,
	// and splitting the wording would only invite one copy to drift into a
	// procedure the other refuses to state.
	EraseDialogNote: "The IC-7610's CI-V protocol has an erase command form, but this build never sends it: no IC-7610 has ever confirmed what it does, and sending an unconfirmed erase command risks clearing the wrong channel. This build does not describe a front-panel procedure either — no IC-7610 operating manual is held here — so follow the memory-channel clear procedure in the radio's own operating manual.",
	// The two tooltips DIFFER, unlike every Yaesu entry's identical pair —
	// because the evidence differs between them on this radio: Tone is on
	// the CI-V surface (unverified) and Scan Skip structurally is not (no
	// mapped field at all, so there is nothing to preserve or fail to).
	PreservationTooltips: PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7610 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — this radio's nearest wire nibble is a select-group marker, not a skip flag",
	},
	// A placeholder LABEL, not an example, on the same footing as the
	// FTdx10's and FTdx101 pair's: no IC-7610 version string has been seen
	// here and none is printed in the CI-V reference.
	FirmwarePlaceholder: "as shown on the radio's display",
	// Restates the no-CI-V-query and no-minimum-version facts (probe's
	// report is read by someone who may never open the send flow), and
	// adds the two facts this radio's probe failure mode turns on: the
	// fixed 98h address with no --civ-address option, and the ASSUMED
	// 19200 default baud — both from core/driver/ic7610/doc.go, both
	// reasons a probe could meet silence that have nothing to do with a
	// wrong port.
	ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7610: this build knows of none to require. This driver talks only to CI-V address 98h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is itself ASSUMED, not read off the radio, since the reference guide names six rates and marks no default. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

// ic7300Text is the IC-7300's entry (Wave 4 task R3, this project's
// second Icom registration and first Icom PAIR), landed with that model's
// wiring registration for the same reason ic7610Text was: internal/wiring's
// TestEverySupportedModelHasRadiotext refuses a registered model with no
// prose.
//
// THE HONESTY RULE APPLIES UNCHANGED. NOTHING BELOW IS INVENTED: no
// IC-7300 has ever been asked anything by this project
// (core/driver/ic7300/doc.go) — every value comes from the IC-7300 FULL
// MANUAL, through core/civ/ic7300's profile and this project's own
// capability matrix, none from a hardware finding — and no write trial has
// happened (writeTrialsComplete, core/driver/ic7300/caps.go, is false).
//
// THIS ENTRY CARRIES NO OTHER RADIO'S EVIDENCE OR PARTICULARS, ic7610Text
// and the IC-7300MK2's own ic7300mk2Text included: core/driver/ic7300/
// doc.go's own package comment states that the IC-7300's and IC-7300MK2's
// manuals are MUTUALLY SILENT about each other and that "no lift in one
// lifts anything for the sibling", which is exactly why this pair
// registers as two driver packages, two fakes and, here, two independently
// written Text values rather than one shared with a model-name
// substitution. The sentence SKELETONS below (an erase-form paragraph, a
// tone/scan-skip paragraph, a firmware paragraph) are shared house style
// across every entry in this file, ic7610Text's included, and that is
// deliberate and not a borrowing this doc comment or its test disputes —
// what must never cross a model boundary is the EVIDENCE inside those
// skeletons: an address, a baud fact, a hedge, a claim about what has or
// has not been confirmed.
// TestRadiotext_IC7300Verbatim pins every string, and its own non-borrowing
// check refuses any field that is byte-identical to, or carries a
// particular of, any of the six other registered models — the four Yaesu
// entries, IC-7610, AND its own IC-7300MK2 sibling.
//
// WHAT IS DIFFERENT ABOUT THIS RADIO, AND WHY IT SHOWS IN THE PROSE:
//
//   - The protocol is CI-V, not CAT, on the same footing as ic7610Text.
//   - This driver's CI-V address is fixed at 94h (core/civ/ic7300/
//     profile.go), with no --civ-address option to change it and no way to
//     detect a radio set to a different address — the same limitation
//     ic7610Text's ProbeFirmwareNote states, restated here for THIS
//     radio's own address.
//   - The default baud, 19200, is a CHOICE among the six [USB] rates this
//     document prints (core/driver/ic7300/caps.go), none of them marked
//     default: the highest rate present in BOTH the [USB] list and the
//     [REMOTE] list, so CI-V still works when the port is linked to
//     [REMOTE]. That is a narrower derivation than the IC-7610's bare
//     ASSUMED six-rate list, and the prose says so rather than reusing the
//     IC-7610's wording.
//   - Tone IS mapped on the wire (the 1A 00 record's tone-mode nibble and
//     two tone-frequency spans, core/driver/ic7300/caps.go's bankFields),
//     on the same footing as the IC-7610. Scan Skip is NOT: the nearest
//     wire nibble is a SELECT-group marker, group membership rather than a
//     skip flag, and mapping it as skip is forbidden (matrix erratum 9 /
//     plan decision D4).
//   - This radio's own operating manual IS held by this project (the IC-7300
//     FULL MANUAL, core/driver/ic7300/doc.go's Provenance section) — unlike
//     the IC-7610, whose entry says "no IC-7610 operating manual is held
//     here". The erase note below cites that manual by name rather than
//     saying no manual exists, which is the one place this entry's wording
//     could not simply follow ic7610Text's shape without becoming false.
var ic7300Text = Text{
	// The CI-V protocol prints two erase command forms — a 1A 00 set whose
	// SELECT byte is FF, and a separate command 0B (core/driver/ic7300/
	// doc.go's "Erase: two printed forms, neither shipped") — but this
	// build sends neither: no IC-7300 has ever confirmed what either does,
	// and sending an unconfirmed erase command risks clearing the wrong
	// channel rather than the intended one. FieldErase carries the zero
	// FieldSupport on both banks, which is what makes core/clone's
	// DiffErased branch unreachable for this model. Unlike the IC-7610,
	// this project DOES hold the IC-7300's own full operating manual, so
	// the user is pointed at ITS clear procedure rather than told none is
	// held here.
	EraseProcedure: "The IC-7300's CI-V protocol prints two erase command forms — a 1A 00 set with a SELECT byte of FF, and a separate command 0B — but this build sends neither: no IC-7300 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure printed in the IC-7300's own full operating manual.",
	// No minimum-firmware fact is established for this radio: the IC-7300
	// full manual names no firmware-version CI-V command anywhere
	// (core/driver/ic7300/doc.go's own register carries no such entry), so
	// there is no CI-V query either, on the same footing as ic7610Text's
	// own firmware note.
	FirmwareGuidance: "No minimum firmware version is established for the IC-7300: nothing this project holds states one, and no IC-7300 has been asked. The IC-7300's own full manual names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone IS read and written for this radio — over CI-V, unverified
	// against real hardware, since no IC-7300 has ever answered a frame.
	// Scan Skip is not: this radio's nearest wire nibble is a SELECT-group
	// marker, not a skip flag, and setting one is refused before anything
	// reaches the radio (core/driver/ic7300/caps.go's bankFields).
	ToneScanSkipNote: "Tone is read and written for the IC-7300 over CI-V by this build, but unverified against real hardware — no IC-7300 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
	// DELIBERATELY EMPTY, exactly as every other registered model's is and
	// for the same reason: writeTrialsComplete is false, so there is no
	// hardware-preservation verification of any kind to report.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as every other model's is: the
	// delete dialog and the blocked-erase review answer the same question.
	EraseDialogNote: "The IC-7300's CI-V protocol prints two erase command forms — a 1A 00 set with a SELECT byte of FF, and a separate command 0B — but this build sends neither: no IC-7300 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. Follow the memory-channel clear procedure printed in the IC-7300's own full operating manual.",
	// The two tooltips DIFFER, exactly as ic7610Text's do and for the same
	// reason: the evidence differs between Tone (on the CI-V surface,
	// unverified) and Scan Skip (structurally not mapped at all).
	PreservationTooltips: PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7300 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-7300's nearest wire nibble is a select-group marker, not a skip flag",
	},
	// A placeholder LABEL, not an example: no IC-7300 version string has
	// been seen here. Names the model rather than the ic7610Text/generic
	// "the radio's display" wording, so this field is not byte-identical
	// to either the IC-7610's or the IC-7300MK2's own placeholder.
	FirmwarePlaceholder: "as shown on the IC-7300's own display",
	// Restates the no-CI-V-query and no-minimum-version facts, and adds the
	// two facts this radio's probe failure mode turns on: the fixed 94h
	// address with no --civ-address option, and the CHOSEN 19200 default
	// baud — both from core/driver/ic7300/caps.go and doc.go.
	ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7300: this build knows of none to require. This driver talks only to CI-V address 94h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is a CHOICE — the highest rate this radio's document lists on both its [USB] and [REMOTE] ports — not a value read off the radio. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

// ic7300mk2Text is the IC-7300MK2's entry, landed alongside ic7300Text in
// the same Wave 4 task R3 registration commit — the second half of this
// project's first Icom PAIR.
//
// THE HONESTY RULE APPLIES UNCHANGED, and the pairing sharpens it rather
// than loosening it: no IC-7300MK2 has ever been asked anything by this
// project (core/driver/ic7300mk2/doc.go) — every value comes from the
// IC-7300MK2 CI-V REFERENCE GUIDE, through core/civ/ic7300mk2's profile,
// none from a hardware finding — and no write trial has happened
// (writeTrialsComplete, core/driver/ic7300mk2/caps.go, is its OWN
// constant, false for its OWN reasons: that package's own comment states
// "The registered sibling's FALSE is not stated here").
//
// THIS ENTRY CARRIES NO OTHER RADIO'S EVIDENCE OR PARTICULARS FROM
// ic7300Text, and that is the one fact this doc comment exists to say
// loudest: the two Icom documents this pair is built from are MUTUALLY
// SILENT about each other (core/driver/ic7300mk2/doc.go's own package
// comment), so an entry that read like ic7300Text with the model name
// substituted would misattribute one radio's evidence to the other —
// exactly the failure mode the FTdx101D/MP pair is EXEMPT from (their
// SHARED manual makes a substitution proof correct for them) and this
// pair is NOT. The shared sentence SKELETON both entries use (house
// style, see ic7300Text's own doc comment) is not itself a borrowing;
// what must never cross is the evidence each skeleton carries.
// TestRadiotext_IC7300MK2Verbatim's own non-borrowing check covers the
// pair against each other, the same way the FTdx101 pair's
// TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName test structure pairs
// its two Verbatim tests — except where THAT test proves near-identity,
// THIS pair's non-borrowing check proves the opposite: distinctness.
//
// WHAT IS DIFFERENT ABOUT THIS RADIO, AND WHY IT SHOWS IN THE PROSE:
//
//   - The protocol is CI-V, not CAT, on the same footing as every other
//     Icom entry.
//   - This driver's CI-V address is fixed at B6h (core/civ/ic7300mk2/
//     profile.go) — the sibling answers at 94h, and IN THE FIELD THE TWO
//     CANNOT CONFUSE EACH OTHER (core/driver/ic7300mk2/doc.go's "The
//     wrong-sibling fingerprint" section) — with no --civ-address option
//     to change it and no way to detect a radio set to a different
//     address.
//   - The default baud, 19200, is a CONSERVATIVE DERIVATION from a table
//     this document prints for an ENTIRELY DIFFERENT PURPOSE — the `18 01`
//     wake-up-command FE-count table, NOT a supported-rate list
//     (core/driver/ic7300mk2/doc.go's own emphatic section on this). This
//     document — a CI-V REFERENCE GUIDE, not a full operating manual —
//     prints NO rate list and NO factory default at all, which is a
//     WEAKER evidential footing than the IC-7300's own six-rate [USB]
//     list, and the prose says so rather than borrowing the sibling's
//     stronger "a CHOICE among six printed rates" wording.
//   - Tone IS mapped on the wire, on the same footing as the sibling. Scan
//     Skip is NOT: this radio's own §3.16 A10 reaches the SAME conclusion
//     as the IC-7300's matrix erratum, independently, on its own reading —
//     the SELECT nibble is group membership, not a skip flag.
//   - This document is a CI-V REFERENCE GUIDE, not a full operating
//     manual, and prints no front-panel procedure for anything — a
//     narrower source than the IC-7300's own full manual, which this
//     entry's erase note says plainly rather than pointing at a manual
//     this project does not hold.
var ic7300mk2Text = Text{
	// The CI-V protocol prints two erase command forms — a 1A 00 set with
	// a truncated data area, and a separate command 0B, whose own printed
	// row states plainly that P1 and P2 cannot be cleared
	// (core/driver/ic7300mk2/doc.go's "Erase: two printed forms, neither
	// shipped") — but this build sends neither: no IC-7300MK2 has ever
	// confirmed what either does, and sending an unconfirmed erase command
	// risks clearing the wrong channel rather than the intended one. This
	// build does not describe a front-panel procedure either — this
	// document is a CI-V reference guide, not a full operating manual — so
	// the user is sent to the radio's own operating manual, which this
	// project does not hold.
	EraseProcedure: "The IC-7300MK2's CI-V protocol prints two erase command forms — a 1A 00 set with a truncated data area, and a separate command 0B, whose own printed row states that P1 and P2 cannot be cleared — but this build sends neither: no IC-7300MK2 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This build does not describe a front-panel procedure either — this document is a CI-V reference guide, not a full operating manual — so follow the memory-channel clear procedure in the radio's own operating manual.",
	// No minimum-firmware fact is established for this radio: this CI-V
	// reference guide names no firmware-version command anywhere, so there
	// is no CI-V query either.
	FirmwareGuidance: "No minimum firmware version is established for the IC-7300MK2: nothing this project holds states one, and no IC-7300MK2 has been asked. The IC-7300MK2's CI-V reference guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone IS read and written for this radio — over CI-V, unverified
	// against real hardware, since no IC-7300MK2 has ever answered a
	// frame. Scan Skip is not: this radio's nearest wire nibble is a
	// SELECT-group marker, not a skip flag, and setting one is refused
	// before anything reaches the radio.
	ToneScanSkipNote: "Tone is read and written for the IC-7300MK2 over CI-V by this build, but unverified against real hardware — no IC-7300MK2 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble is a select-group marker, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
	// DELIBERATELY EMPTY, on the same footing as every other registered
	// model's: writeTrialsComplete is false, so there is no
	// hardware-preservation verification of any kind to report.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as every other model's is.
	EraseDialogNote: "The IC-7300MK2's CI-V protocol prints two erase command forms — a 1A 00 set with a truncated data area, and a separate command 0B, whose own printed row states that P1 and P2 cannot be cleared — but this build sends neither: no IC-7300MK2 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This build does not describe a front-panel procedure either — this document is a CI-V reference guide, not a full operating manual — so follow the memory-channel clear procedure in the radio's own operating manual.",
	// The two tooltips DIFFER, on the same footing as every other Icom
	// entry's: Tone is on the CI-V surface (unverified) and Scan Skip
	// structurally is not (no mapped field at all).
	PreservationTooltips: PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-7300MK2 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-7300MK2's nearest wire nibble is a select-group marker, not a skip flag",
	},
	// A placeholder LABEL, not an example: no IC-7300MK2 version string
	// has been seen here. Names the model rather than the ic7610Text/
	// generic "the radio's display" wording, so this field is not
	// byte-identical to either the IC-7610's or the IC-7300's own
	// placeholder.
	FirmwarePlaceholder: "as shown on the IC-7300MK2's own display",
	// Restates the no-CI-V-query and no-minimum-version facts, and adds
	// the two facts this radio's probe failure mode turns on: the fixed
	// B6h address with no --civ-address option, and the default baud of
	// 19200, itself a conservative derivation from a wake-up-command table
	// this document prints for a different purpose — this reference guide
	// names no rate list and no factory default at all.
	ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-7300MK2: this build knows of none to require. This driver talks only to CI-V address B6h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is a conservative derivation from a wake-up-command table this document prints for an unrelated purpose — this reference guide names no baud list and no factory default at all. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

// ic705Text is the IC-705's entry (Wave 4 task R4, this project's third
// Icom registration, and its second LONE model since the IC-7610 — no
// sibling, no pairing rationale to restate).
//
// THE HONESTY RULE APPLIES UNCHANGED. NOTHING BELOW IS INVENTED: no
// IC-705 has ever been asked anything by this project
// (core/driver/ic705/doc.go) — every value comes from the IC-705 CI-V
// REFERENCE GUIDE (the sole layout authority) and, for three narrow
// values that document does not carry, the IC-705 BASIC MANUAL, admitted
// for those three values only and for nothing else — never from a
// hardware finding — and no write trial has happened
// (writeTrialsComplete, core/driver/ic705/caps.go, is false).
//
// THIS ENTRY CARRIES NO OTHER RADIO'S EVIDENCE OR PARTICULARS: it is a
// fourth manufacturer-and-model combination in this file, and
// TestRadiotext_IC705Verbatim's own non-borrowing check refuses any field
// that is byte-identical to, or carries a particular of, any of the
// SEVEN other registered models — the four Yaesu entries and all three
// other registered Icom ones (IC-7610, IC-7300, IC-7300MK2). The shared
// sentence SKELETON several entries use (house style, see ic7300Text's
// own doc comment) is not itself a borrowing; what must never cross is
// the evidence each skeleton carries.
//
// WHAT IS DIFFERENT ABOUT THIS RADIO, AND WHY IT SHOWS IN THE PROSE:
//
//   - The protocol is CI-V, not CAT, on the same footing as every other
//     Icom entry.
//   - This driver's CI-V address is fixed at A4h (core/civ/ic705/
//     profile.go's RadioAddress), with no --civ-address option to change
//     it and no way to detect a radio set to a different address.
//   - THE BAUD GRADE IS WEAKER THAN EVERY OTHER ICOM ENTRY'S. The
//     IC-7610's six-rate list is manual-evidenced and only its default is
//     assumed; the IC-7300's default is a CHOICE among six printed rates;
//     the IC-7300MK2's is a conservative derivation from an unrelated
//     table. The IC-705's CI-V Reference Guide prints NO baud information
//     for the CI-V port at all — not a rate list, not a default — so BOTH
//     the six-value list and the 19200 default are ASSUMED PLACEHOLDERS
//     (core/driver/ic705/caps.go's own comment), and the prose says so
//     rather than borrowing any sibling's stronger wording. The one
//     related fact admitted from the IC-705 BASIC MANUAL is a NEGATIVE:
//     the microUSB CI-V port is baud-agnostic ("You can communicate
//     regardless of the PC software's baud rate setting"), which lowers
//     the cost of a wrong guess without being evidence of a default.
//   - Tone IS mapped on the wire (the 1A 00 record's tone-mode nibble and
//     two tone-frequency spans, core/driver/ic705/caps.go's bankFields),
//     on the same footing as every other registered Icom model. Scan Skip
//     is NOT: the nearest wire nibble marks a channel into one of three
//     select-scan groups, not a skip flag, and mapping it as skip is
//     forbidden (matrix erratum 17 / plan decision O-6).
//   - THIS RECORD MAPS MORE OF THE TIER THAN ANY OTHER REGISTERED ICOM
//     MODEL: unlike the IC-7610 and the IC-7300 pair, none of which maps
//     duplex, offset or DTCS, the IC-705's 111-byte record carries all
//     thirteen of matrix §2's rw-graded rows, including duplex, offset,
//     tx_frequency, dtcs_code and dtcs_polarity — so this radio's grid
//     reaches every one of the tier's ten added fields, not four or six.
//   - This project holds no full IC-705 operating manual, only the CI-V
//     Reference Guide and a Basic Manual admitted for three unrelated
//     values — so, like the IC-7300MK2, this entry's erase note points at
//     the radio's own operating manual rather than naming one this
//     project holds.
var ic705Text = Text{
	// The CI-V protocol prints two erase command forms — a 1A 00 set
	// carrying FF at the fifth data position, and a separate command 0B
	// (core/driver/ic705/doc.go's "Erase: the wire forms exist here, and
	// are shipped nowhere") — but this build sends neither: no IC-705 has
	// ever confirmed what either does, and sending an unconfirmed erase
	// command risks clearing the wrong channel rather than the intended
	// one. FieldErase carries the zero FieldSupport on both banks, which
	// is what makes core/clone's DiffErased branch unreachable for this
	// model. This project's own copy of the IC-705 Basic Manual is
	// admitted for three values unrelated to erasing a channel, so it
	// names no front-panel procedure here either.
	EraseProcedure: "The IC-705's CI-V protocol prints two erase command forms — a 1A 00 set carrying FF at the fifth data position, and a separate command 0B — but this build sends neither: no IC-705 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This project's own copy of the IC-705 Basic Manual is admitted for three unrelated values only, so it names no front-panel clear procedure — follow the memory-channel clear procedure in the radio's own full operating manual.",
	// No minimum-firmware fact is established for this radio: the IC-705
	// CI-V Reference Guide names no firmware-version command anywhere, so
	// there is no CI-V query either, on the same footing as every other
	// registered Icom entry's own firmware note.
	FirmwareGuidance: "No minimum firmware version is established for the IC-705: nothing this project holds states one, and no IC-705 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone IS read and written for this radio — over CI-V, unverified
	// against real hardware, since no IC-705 has ever answered a frame.
	// Scan Skip is not: this radio's nearest wire nibble marks a channel
	// into one of three select-scan groups, not a skip flag, and setting
	// one is refused before anything reaches the radio.
	ToneScanSkipNote: "Tone is read and written for the IC-705 over CI-V by this build, but unverified against real hardware — no IC-705 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three select-scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
	// DELIBERATELY EMPTY, exactly as every other registered model's is
	// and for the same reason: writeTrialsComplete is false, so there is
	// no hardware-preservation verification of any kind to report.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as every other model's is: the
	// delete dialog and the blocked-erase review answer the same question.
	EraseDialogNote: "The IC-705's CI-V protocol prints two erase command forms — a 1A 00 set carrying FF at the fifth data position, and a separate command 0B — but this build sends neither: no IC-705 has ever confirmed what either does, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This project's own copy of the IC-705 Basic Manual is admitted for three unrelated values only, so it names no front-panel clear procedure — follow the memory-channel clear procedure in the radio's own full operating manual.",
	// The two tooltips DIFFER, exactly as every other Icom entry's do and
	// for the same reason: the evidence differs between Tone (on the
	// CI-V surface, unverified) and Scan Skip (structurally not mapped at
	// all).
	PreservationTooltips: PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-705 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-705's nearest wire nibble marks select-scan group membership, not a skip flag",
	},
	// A placeholder LABEL, not an example: no IC-705 version string has
	// been seen here. Names the model rather than a generic "the radio's
	// display" wording, so this field is not byte-identical to any other
	// registered model's own placeholder.
	FirmwarePlaceholder: "as shown on the IC-705's own display",
	// Restates the no-CI-V-query and no-minimum-version facts, and adds
	// the two facts this radio's probe failure mode turns on: the fixed
	// A4h address with no --civ-address option, and the fact that BOTH
	// the baud list and the default are ASSUMED — see this var's own doc
	// comment for why that is a weaker grade than every other registered
	// Icom entry's baud claim.
	ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-705: this build knows of none to require. This driver talks only to CI-V address A4h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole six-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no baud information for the CI-V port at all, and the one related fact admitted from the Basic Manual is a negative: the microUSB CI-V port is baud-agnostic, which lowers the cost of a wrong guess without being evidence of one. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

// ic9700Text is the IC-9700's entry (Wave 4 task R5, this project's
// fourth Icom registration, and its second LONE model since the IC-705 —
// no sibling, no pairing rationale to restate).
//
// THE HONESTY RULE APPLIES UNCHANGED. NOTHING BELOW IS INVENTED: no
// IC-9700 has ever been asked anything by this project
// (core/driver/ic9700/doc.go) — every value comes from the IC-9700 CI-V
// REFERENCE GUIDE (the sole layout authority; this project holds no full
// IC-9700 operating manual), never from a hardware finding — and no write
// trial has happened (writeTrialsComplete, core/driver/ic9700/caps.go, is
// false).
//
// THIS ENTRY CARRIES NO OTHER RADIO'S EVIDENCE OR PARTICULARS: it is a
// fifth manufacturer-and-model combination in this file, and
// TestRadiotext_IC9700Verbatim's own non-borrowing check refuses any field
// that is byte-identical to, or carries a particular of, any of the EIGHT
// other registered models — the four Yaesu entries and all four other
// registered Icom ones (IC-7610, IC-7300, IC-7300MK2, IC-705). The shared
// sentence SKELETON several entries use (house style, see ic7300Text's
// own doc comment) is not itself a borrowing; what must never cross is
// the evidence each skeleton carries.
//
// WHAT IS DIFFERENT ABOUT THIS RADIO, AND WHY IT SHOWS IN THE PROSE:
//
//   - The protocol is CI-V, not CAT, on the same footing as every other
//     Icom entry.
//   - This driver's CI-V address is fixed at A2h (caps.go's CATID, echoed
//     by ic9700.go's runtime Identity().CATID), with no --civ-address
//     option to change it and no way to detect a radio set to a different
//     address.
//   - THE BAUD GRADE IS ITS OWN SHAPE, neither the IC-7610's plain silence
//     nor the IC-705's total absence. The IC-9700's CI-V Reference Guide
//     DOES print the six-rate list (PDF p.13 footnote *4, caps.go's
//     baudRates) — as manual-evidenced as the IC-7610's own list — but it
//     ACTIVELY DEFERS the factory default to a SEPARATE document, the
//     radio's instruction manual (PDF p.4 "Preparing" names a speed and
//     points elsewhere), which this project does not hold. That is a
//     document POINTING AWAY, not a document simply silent about a
//     default the way the IC-7610's is, and caps.go's own defaultBaud
//     comment marks 19200 ASSUMED on that basis: the middle of the printed
//     six, and the rate Icom most commonly ships — a guess about the
//     radio, not a reading of this document. LIFTED BY: register entry
//     `ic9700-factory-default-baud`, lift R2 — `19 00` attempted at each
//     printed rate on a factory radio.
//   - Tone IS mapped on the wire (the 1A 00 record's ⑬ duplex/tone-mode
//     nibbles and two tone-frequency spans, caps.go's bankFields), on the
//     same footing as every other registered Icom model. Scan Skip is
//     NOT: field ④'s LOW nibble (core/civ/ic9700/profile.go's
//     selectNames) marks a channel into one of three SELECT-memory scan
//     groups — OFF, ★1, ★2, ★3 — not a skip flag, and mapping it as skip
//     is forbidden (matrix erratum 13 / OQ-4).
//   - THIS RECORD MAPS ALL TEN OF THE TIER'S ADDED FIELDS, on the same
//     footing as the IC-705 — an independent fact about THIS radio's own
//     111-byte record (caps.go's bankFields carries all thirteen of
//     matrix §2's rw-graded rows, including duplex, offset, tx_frequency,
//     dtcs_code and dtcs_polarity), not a repetition of the IC-705's own
//     claim: the two documents are unrelated and neither lifts anything
//     for the other.
//   - THE OFFSET SCALE IS AN OPEN QUESTION, not a settled one. Matrix
//     Erratum 14 records an unresolved disagreement between §1b's printed
//     digit places (which would read the golden's duplex offset bytes as
//     6 MHz) and this driver's own reading of 600 kHz — a factor of ten
//     apart, and unresolved by any of the matrix's seventeen errata
//     (core/driver/ic9700/doc.go's `ic9700-offset-scale-100hz` register
//     entry). The 100 Hz-scale reading this driver implements is ASSUMED,
//     not settled, and no advisory here claims otherwise.
//   - The printed clear form is a SINGLE form here — `1A 00 <addr> FF`
//     (matrix §3.13) — unlike the IC-705's and IC-7300's own two forms
//     (a set plus a separate command 0B): this document names no separate
//     erase command at all. It is still deliberately unshipped: no
//     builder exists, the gate has no branch that could admit one, and
//     the consent transform exempts erase structurally.
//   - This project holds no full IC-9700 operating manual, only the CI-V
//     Reference Guide — so, like the IC-7610 and the IC-7300MK2, this
//     entry's erase note points at the radio's own operating manual
//     rather than naming one this project holds.
//   - THREE BANKS, not one or two: MEM, SCAN and CALL, all DENSE and
//     addressed by the same `1A 00` form (caps.go's banks). Nothing below
//     is bank-specific — bankFields grades all three identically — so
//     this entry needed no third variant of any field; the bank COUNT is
//     app/uispec.go's own fact to state, not this package's.
var ic9700Text = Text{
	// The CI-V protocol prints one memory clear form — `1A 00 <addr> FF`
	// (matrix §3.13) — but this build sends it to no channel: no builder
	// exists in this driver, and sending an unconfirmed erase command
	// risks clearing the wrong channel rather than the intended one.
	// FieldErase carries the zero FieldSupport on all three banks, which
	// is what makes core/clone's DiffErased branch unreachable for this
	// model. This document is a CI-V reference guide, not a full
	// operating manual, and prints no front-panel clear procedure either,
	// so the user is sent to the radio's own operating manual, which this
	// project does not hold.
	EraseProcedure: "The IC-9700's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF at the address's data position — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
	// No minimum-firmware fact is established for this radio: the IC-9700
	// CI-V Reference Guide names no firmware-version command anywhere, so
	// there is no CI-V query either, on the same footing as every other
	// registered Icom entry's own firmware note.
	FirmwareGuidance: "No minimum firmware version is established for the IC-9700: nothing this project holds states one, and no IC-9700 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone IS read and written for this radio — over CI-V, unverified
	// against real hardware, since no IC-9700 has ever answered a frame.
	// Scan Skip is not: this radio's nearest wire nibble marks a channel
	// into one of three SELECT-memory scan groups, not a skip flag.
	ToneScanSkipNote: "Tone is read and written for the IC-9700 over CI-V by this build, but unverified against real hardware — no IC-9700 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT-memory scan groups (★1/★2/★3), not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not.",
	// DELIBERATELY EMPTY, exactly as every other registered model's is
	// and for the same reason: writeTrialsComplete is false, so there is
	// no hardware-preservation verification of any kind to report.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as every other model's is: the
	// delete dialog and the blocked-erase review answer the same question.
	EraseDialogNote: "The IC-9700's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF at the address's data position — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
	// The two tooltips DIFFER, exactly as every other Icom entry's do and
	// for the same reason: the evidence differs between Tone (on the
	// CI-V surface, unverified) and Scan Skip (structurally not mapped at
	// all).
	PreservationTooltips: PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-9700 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-9700's nearest wire nibble marks one of three SELECT-memory scan groups, not a skip flag",
	},
	// A placeholder LABEL, not an example: no IC-9700 version string has
	// been seen here. Names the model rather than a generic "the radio's
	// display" wording, so this field is not byte-identical to any other
	// registered model's own placeholder.
	FirmwarePlaceholder: "as shown on the IC-9700's own display",
	// Restates the no-CI-V-query and no-minimum-version facts, and adds
	// the two facts this radio's probe failure mode turns on: the fixed
	// A2h address with no --civ-address option, and the ASSUMED default
	// baud — see this var's own doc comment for why the IC-9700's grade
	// differs from every other registered Icom entry's baud claim.
	ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-9700: this build knows of none to require. This driver talks only to CI-V address A2h, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200 is ASSUMED — the middle of the six rates this document prints, and the rate Icom most commonly ships, not a value this document itself names as the default: it defers the factory setting to the radio's own instruction manual, which this project does not hold. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
}

// ic905Text is the IC-905's entry (Wave 4 task R6, this project's FIFTH
// Icom registration, and the tier's LAST — see internal/wiring's
// IC905Model doc comment). It is the third LONE model since the IC-705 —
// no sibling, no pairing rationale to restate.
//
// THE HONESTY RULE APPLIES UNCHANGED. NOTHING BELOW IS INVENTED: no
// IC-905 has ever been asked anything by this project
// (core/driver/ic905/doc.go) — every value comes from the IC-905 CI-V
// REFERENCE GUIDE (the sole layout authority; this project holds no full
// IC-905 operating manual), never from a hardware finding — and no write
// trial has happened (writeTrialsComplete, core/driver/ic905/caps.go, is
// false).
//
// THIS ENTRY CARRIES NO OTHER RADIO'S EVIDENCE OR PARTICULARS: it is a
// tenth manufacturer-and-model combination in this file, and
// TestRadiotext_IC905Verbatim's own non-borrowing check refuses any field
// that is byte-identical to, or carries a particular of, any of the NINE
// other registered models — the four Yaesu entries and all five other
// registered Icom ones (IC-7610, IC-7300, IC-7300MK2, IC-705, IC-9700).
// The shared sentence SKELETON several entries use (house style, see
// ic7300Text's own doc comment) is not itself a borrowing; what must
// never cross is the evidence each skeleton carries.
//
// WHAT IS DIFFERENT ABOUT THIS RADIO, AND WHY IT SHOWS IN THE PROSE:
//
//   - The protocol is CI-V, not CAT, on the same footing as every other
//     Icom entry.
//   - This driver's CI-V address is fixed at ACh (caps.go's CATID, echoed
//     by ic905.go's runtime Identity().CATID, "AC:" plus the observed 19
//     00 token), with no --civ-address option to change it and no way to
//     detect a radio set to a different address.
//   - THE BAUD GRADE IS THE IC-705's SHAPE, not the IC-9700's: this
//     document prints no rate figure ANYWHERE — no data-bit count, no
//     stop-bit count, no parity, and no rate on any command-table page
//     (core/driver/ic905/doc.go's own sweep) — so both the five-rate list
//     (one fewer than every other registered Icom entry's own list, all
//     of which offer six) and the 19200 default are ASSUMED PLACEHOLDERS,
//     not a choice among printed rates. Registers: ic905.bauds (lift
//     ic905-R-04) and ic905.default_baud (lift ic905-R-03).
//   - Tone IS mapped on the wire (the 1A 00 record's ⑭ duplex/tone-mode
//     nibbles and two tone-frequency spans, caps.go's bankFields), on the
//     same footing as every other registered Icom model. Scan Skip is
//     NOT: byte ⑤'s LOW nibble enumerates one of three SELECT-memory scan
//     groups — OFF, ★1, ★2, ★3 — not a skip flag, and mapping it as skip
//     is forbidden (caps.go's own bankFields doc comment).
//   - DTCS HAS ITS OWN CONSEQUENCE, unique among this tier's registered
//     entries: its three digits are OCTAL (PDF p.24, folio 23), and a
//     code this build cannot read as three octal digits comes back
//     Unknown rather than a number (core/driver/ic905/read.go's
//     dtcsCodeField). Because this codec has no preserve-by-cache for a
//     mapped field it cannot read as Known, a channel whose DTCS code is
//     Unknown cannot be written AT ALL — not merely that one field —
//     until it is corrected to a valid octal value (write.go's rung 4,
//     mandatoryKnownFields).
//   - THIS RECORD MAPS NINE OF THE TIER'S TEN ADDED FIELDS, not all ten
//     like the IC-705's and the IC-9700's: caps.go's bankFields zeroes
//     tx_frequency (MANUAL-EVIDENCED ABSENCE — exactly one frequency
//     field, no duplicated TX block), so this radio's grid reaches
//     duplex, offset, tone_mode, tone_tx, tone_rx, dtcs_code,
//     dtcs_polarity, filter and data_mode, and no more.
//   - THE DEFAULT OPEN DISCOVERS A BOUNDED WALK, not the whole 100 x 100
//     space: group 0 in full, then one channel per group elsewhere,
//     descending only where that channel answered
//     (core/driver/ic905/read.go's discoverInventory) — a scattered
//     channel outside that walk needs a session opened with this
//     driver's own ic905.WithFullInventoryWalk() option, which
//     internal/wiring's registry row deliberately does not pass (see
//     NewIC905RealDriver's own doc comment).
//   - The printed clear form admits memory groups 00 00 ~ 00 99 but
//     explicitly excludes the CALL group (PDF p.19, folio 18: "You cannot
//     specify group '01 00'"), and it is still deliberately unshipped: no
//     builder exists, the gate has no branch that could admit one, and
//     the consent transform exempts erase structurally.
//   - This project holds no full IC-905 operating manual, only the CI-V
//     Reference Guide — so, like the IC-7610, the IC-7300MK2 and the
//     IC-9700, this entry's erase note points at the radio's own
//     operating manual rather than naming one this project holds.
var ic905Text = Text{
	// The CI-V protocol prints one memory clear form — a 1A 00 set
	// carrying FF after the group and channel bytes, for memory groups
	// 00 00 ~ 00 99 only, the CALL group excluded by the document's own
	// words — but this build sends it to no channel: no builder exists in
	// this driver, and sending an unconfirmed erase command risks
	// clearing the wrong channel rather than the intended one.
	// FieldErase carries the zero FieldSupport on both banks, which is
	// what makes core/clone's DiffErased branch unreachable for this
	// model. This document is a CI-V reference guide, not a full
	// operating manual, and prints no front-panel clear procedure either,
	// so the user is sent to the radio's own operating manual, which this
	// project does not hold.
	EraseProcedure: "The IC-905's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF after the group and channel bytes, for memory groups 00 00 ~ 00 99 only, the CALL group being excluded by the document's own words — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
	// No minimum-firmware fact is established for this radio: the IC-905
	// CI-V Reference Guide names no firmware-version command anywhere, so
	// there is no CI-V query either, on the same footing as every other
	// registered Icom entry's own firmware note.
	FirmwareGuidance: "No minimum firmware version is established for the IC-905: nothing this project holds states one, and no IC-905 has been asked. This driver's CI-V Reference Guide names no CI-V query for the version either — read it off the radio's own display and enter it here, where it is recorded with the send rather than checked against a threshold nobody has established.",
	// Tone IS read and written for this radio — over CI-V, unverified
	// against real hardware, since no IC-905 has ever answered a frame.
	// Scan Skip is not: this radio's nearest wire nibble marks a channel
	// into one of three SELECT-memory scan groups, not a skip flag. DTCS
	// carries its own consequence: an out-of-range code reads back
	// Unknown, and — because this codec cannot synthesise a value it
	// never read — a channel in that state cannot be written at all until
	// the code is corrected.
	ToneScanSkipNote: "Tone is read and written for the IC-905 over CI-V by this build, but unverified against real hardware — no IC-905 has ever answered a frame. Scan Skip is not: this radio's nearest CI-V nibble marks a channel into one of three SELECT-memory scan groups, not a skip flag, so a Scan Skip value is refused before anything reaches the radio rather than being sent as something it is not. DTCS is mapped too, but its three digits are OCTAL: a code this build cannot read as three octal digits comes back Unknown rather than a number, and — because this codec has no preserve-by-cache — a channel whose DTCS code is Unknown cannot be written at all until it is corrected to a valid octal value.",
	// DELIBERATELY EMPTY, exactly as every other registered model's is
	// and for the same reason: writeTrialsComplete is false, so there is
	// no hardware-preservation verification of any kind to report.
	ToneScanSkipVerification: "",
	// Byte-identical to EraseProcedure, as every other model's is: the
	// delete dialog and the blocked-erase review answer the same question.
	EraseDialogNote: "The IC-905's CI-V protocol prints one memory clear form — a 1A 00 set carrying FF after the group and channel bytes, for memory groups 00 00 ~ 00 99 only, the CALL group being excluded by the document's own words — but this build sends it to no channel: no builder exists in this driver, and sending an unconfirmed erase command risks clearing the wrong channel rather than the intended one. This document is a CI-V reference guide, not a full operating manual, and prints no front-panel clear procedure either, so follow the memory-channel clear procedure in the radio's own operating manual.",
	// The two tooltips DIFFER, exactly as every other Icom entry's do and
	// for the same reason: the evidence differs between Tone (on the
	// CI-V surface, unverified) and Scan Skip (structurally not mapped at
	// all).
	PreservationTooltips: PreservationTooltips{
		Tone:     "read and written over CI-V by this build — unverified against real hardware, since no IC-905 has ever answered a frame",
		ScanSkip: "not read or written over CI-V by this build — the IC-905's nearest wire nibble marks one of three SELECT-memory scan groups, not a skip flag",
	},
	// A placeholder LABEL, not an example: no IC-905 version string has
	// been seen here. Names the model rather than a generic "the radio's
	// display" wording, so this field is not byte-identical to any other
	// registered model's own placeholder.
	FirmwarePlaceholder: "as shown on the IC-905's own display",
	// Restates the no-CI-V-query and no-minimum-version facts, adds the
	// fixed ACh address with no --civ-address option, this radio's own
	// five-rate ASSUMED baud grade (see this var's own doc comment for
	// why it is the IC-705's shape, not the IC-9700's), and the default
	// open's BOUNDED discovery walk — the one fact no other registered
	// Icom entry's own ProbeFirmwareNote states, because no other
	// registered model's Open takes this driver's own
	// WithFullInventoryWalk() opt-out.
	ProbeFirmwareNote: "Firmware version has no CI-V query — check the radio's display. No minimum version is established for the IC-905: this build knows of none to require. This driver talks only to CI-V address ACh, with no --civ-address option to change it and no way to detect a radio set to a different address; and its default baud of 19200, along with the whole five-rate list it is chosen from, is ASSUMED — this radio's CI-V Reference Guide prints no rate figure anywhere, on any port's command-table page. Opening this radio also discovers its MEM bank's occupied slots by a BOUNDED walk — group 0 in full, then one channel per group beyond it — not the whole 100x100 space, so a channel scattered outside that walk needs a session opened with this driver's own WithFullInventoryWalk() option to be seen at all. If nothing answers, check the radio's address and speed before assuming the port is wrong.",
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
	"FT-710":     ft710Text,
	"FTdx10":     ftdx10Text,
	"FTdx101D":   ftdx101dText,
	"FTdx101MP":  ftdx101mpText,
	"IC-7610":    ic7610Text,
	"IC-7300":    ic7300Text,
	"IC-7300MK2": ic7300mk2Text,
	"IC-705":     ic705Text,
	"IC-9700":    ic9700Text,
	"IC-905":     ic905Text,
}

// For returns model's radio-specific prose. "FT-710", "FTdx10", "FTdx101D",
// "FTdx101MP", "IC-7610", "IC-7300", "IC-7300MK2", "IC-705", "IC-9700"
// and "IC-905" are populated — the TEN models internal/wiring registers
// AS OF Wave 4 task R6, the tier's LAST registration, a count an eleventh
// registration would falsify; any other model —
// including "", a future driver not yet given an entry, or a near-miss
// typo ("FT-DX10", "IC7610", "IC7300", "IC705", "IC9700" or "IC905", say) —
// returns the zero Text and false.
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
