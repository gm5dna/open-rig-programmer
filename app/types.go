// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "github.com/gm5dna/open-rig-programmer/core/codeplug"

// This file holds every struct a bound method returns or an event
// carries (task-15 brief §2) — the exact shapes `wails generate module`
// turns into wailsjs/go/models.ts for the Svelte frontend (Task 16) to
// build against. Field comments here are the authoritative source for
// this task's report.

// PortEntry is transport.PortInfo mapped to a display struct for
// ListPorts — VID/PID/USBSerial are deliberately dropped (not needed by
// a port picker; the real Identity is only known once Connect succeeds).
type PortEntry struct {
	Path        string
	Description string
	Score       int
	Hints       []string
}

// ConnectionInfo is what Connect/ConnectDemo return on success.
type ConnectionInfo struct {
	Model     string
	CATID     string
	Port      string
	USBSerial string
	Region    string
	Demo      bool
	// NeedsUnverifiedConsent is true when this radio is CONSENT-ELIGIBLE:
	// when its real-hardware baseline still carries a write-side unverified
	// field, so a recorded consent could open a write gate on it at all
	// (internal/wiring.NeedsUnverifiedConsent — the SAME predicate the CLI's
	// "settings unverified-writes" uses, so the two surfaces cannot
	// disagree about which radios consent even applies to). It describes
	// the RADIO, never the user's answer, so it does not change when a
	// decision is taken.
	//
	// FALSE on a demo connection, whatever the model: a simulated session
	// takes no consent option and spends no consent, so there is nothing to
	// ask about (see connect).
	NeedsUnverifiedConsent bool
	// UnverifiedConsentRecorded is true when the user has already ANSWERED
	// the consent question for this radio — either way. It is the flag a
	// prompt must be gated on: "never asked" and "asked and declined" are
	// different states (internal/userconfig's first rule), and a user who
	// has declined must not be asked again at every connection.
	//
	// Whether consent is IN FORCE is deliberately not here. That is read
	// from the session's own capabilities (UISpecView's
	// UnverifiedWritesConsented), which cannot drift from what the running
	// session will actually let through; this pair says only what to ask,
	// not what is armed. FALSE on a demo connection, which never reads the
	// store.
	UnverifiedConsentRecorded bool
}

// UnverifiedWriteConsentView is one radio's consent state, as
// GetUnverifiedWriteConsent returns it and ListUnverifiedWriteConsents
// lists it — the grants panel's row, and the arming dialogue's data.
//
// The three booleans are not redundant. NeedsConsent describes the RADIO
// (is there an unverified write here at all?); Recorded says whether the
// user has answered; Granted is their answer. Together they name every
// state a surface has to render differently: nothing to ask (NeedsConsent
// false), never asked (Recorded false), declined (Recorded true, Granted
// false) and granted.
type UnverifiedWriteConsentView struct {
	// Model is the canonical model name — the same spelling
	// GetSupportedModels lists and Connect accepts.
	Model string
	// NeedsConsent mirrors ConnectionInfo.NeedsUnverifiedConsent: this
	// radio is consent-eligible. A model whose writes are hardware-verified
	// reports false, and SetUnverifiedWriteConsent refuses it.
	NeedsConsent bool
	// Granted is the user's recorded answer. False both for a model never
	// asked about and for one declined — the two are told apart by
	// Recorded, not by this.
	Granted bool
	// Recorded is true once a decision has been taken and stored, so a
	// caller can tell "never asked" from "asked and said no" and prompt
	// only in the first case.
	Recorded bool
	// Warning is the arming dialogue's body for THIS radio:
	// internal/radiotext.UnverifiedWriteWarningTemplate with the model name
	// substituted. Empty when NeedsConsent is false — there is no
	// unverified write to warn about, and inventing a caution for a
	// hardware-verified radio would teach a user to dismiss the real one.
	Warning string
}

// CodeplugView is a read view of the working codeplug — returned by
// ReadRadio, GetCodeplug, and LoadFile. Channels/Radio are defensive
// deep copies (see copyChannels): mutating the returned value can never
// reach back into the App's own working copy. Menus is still deliberately
// NOT exposed on this view: the M8b menu/EX settings viewer adds its own
// bound methods for the typed MenuSnapshot instead, keeping the channel
// view unchanged. SaveFile/ExportCSV still carry Menus through
// transparently via the App's own internal working copy, unaffected by
// this view being menus-less.
//
// WorkingPath/Dirty/BaselineStale are included alongside the codeplug
// content itself so the frontend does not need three additional round
// trips just to render a title bar; IsDirty() still exists as its own
// bound method too (task-15 brief §2's SaveFile bullet) for callers
// that only need that one flag.
type CodeplugView struct {
	Schema      int
	Generator   string
	Radio       codeplug.RadioInfo
	Channels    []codeplug.Channel
	WorkingPath string
	Dirty       bool
	// BaselineStale is true once a send has touched the radio (fully or
	// partially) since the last ReadRadio: the App does not know the
	// radio's resulting state (Execute's candidate is not necessarily
	// what the radio now holds — see ConfirmSend's doc comment), so this
	// flags "re-read recommended" rather than silently presenting a
	// baseline that may no longer be accurate.
	BaselineStale bool
}

// IssueView mirrors one codeplug.Issue for display: Field/Severity are
// rendered as plain strings (not spec.Field/codeplug.Severity) to keep
// the wailsjs surface simple.
type IssueView struct {
	Slot     string
	Field    string
	Severity string
	Msg      string
}

// EditResult is UpdateChannel/UpdateChannels' return value.
type EditResult struct {
	Issues []IssueView
	Dirty  bool
}

// ValidationView is Validate's return value: Advisory is true when
// Issues were computed against the static, disconnected capability
// baseline (task-15 brief §2: "static caps lack discovered regional
// banks, so offline results are advisory").
type ValidationView struct {
	Issues   []IssueView
	Advisory bool
}

// DiffEntryView mirrors one codeplug.DiffEntry for display. SlotDisplay
// is the codeplug.DisplaySlot form (e.g. "M-01") — computed here because
// the frontend has no Go access to that mapping (task-15 brief §2 notes
// the same for transfer:progress's Slot field).
type DiffEntryView struct {
	Slot        string
	SlotDisplay string
	Bank        string
	Kind        string
	Before      *codeplug.ChannelData
	After       *codeplug.ChannelData
	Blocked     bool
	BlockReason string
}

// DiffCounts mirrors codeplug.DiffResult's summary counts.
type DiffCounts struct {
	Added, Modified, Erased, Blocked, Unchanged int
}

// DiffSummaryView groups a DiffResult's entries by kind (Unchanged
// entries are never listed, only counted — matching the CLI's own
// writeDiffReport grouping), shared by DiffView and SendPlanView so
// both present diffs identically.
type DiffSummaryView struct {
	Added    []DiffEntryView
	Modified []DiffEntryView
	Erased   []DiffEntryView
	Counts   DiffCounts
}

// DiffView is DiffAgainstRadio's return value.
type DiffView struct {
	Diff DiffSummaryView
}

// SendPlanView is PrepareSend's return value.
type SendPlanView struct {
	Diff DiffSummaryView
	// SnapshotPath is where PrepareSend saved the fresh baseline it
	// diffed against.
	SnapshotPath string
	// BaselineDigestShort is the plan's baseline digest, truncated to 12
	// hex characters for display (matching the CLI's truncateDigest).
	BaselineDigestShort string
	// ConfirmationDigest is the opaque token ConfirmSend must be given
	// back verbatim — never derive or re-truncate it for display.
	ConfirmationDigest string
	// NothingToSend is true when the diff has zero unblocked
	// Added/Modified entries (matching the CLI's countSendable == 0
	// case).
	NothingToSend bool
	// FirmwareRequired is a BEST-EFFORT prediction of whether Execute
	// will need ExecuteOptions.FirmwareConfirmed, computed from
	// working/baseline's RadioInfo.FirmwareConfirmed emptiness (task-15
	// brief §2) — clone.Service's own firmware-confirmed flag is private
	// and per-Service (reset on every reconnect), so this cannot be
	// authoritative. If the prediction is wrong, Execute's own
	// ErrFirmwareUnconfirmed refusal still surfaces via transfer:done
	// (Outcome "refused") and the frontend re-prompts.
	FirmwareRequired bool
	// FirmwareGuidance is the FT-710-specific prose explaining WHY a
	// confirmed firmware version is being asked for and how to find it
	// (send.go's firmwareGuidance func, which serves it from
	// internal/radiotext) — non-empty iff FirmwareRequired is true. Fix 6
	// (adjudicated LOW, Codex M6 #6): the frontend renders this verbatim
	// rather than hardcoding any FT-710 protocol fact (the V01-10 threshold,
	// the absence of a CAT firmware query) of its own.
	FirmwareGuidance string
}

// SlotResultView mirrors one clone.SlotResult for display.
type SlotResultView struct {
	Slot        string
	SlotDisplay string
	Action      string
	VerifyOK    bool
	Detail      string
}

// ReportView mirrors clone.Report for display, plus SnapshotPath (which
// Report itself does not carry — only the plan does, so ConfirmSend's
// goroutine attaches it from the plan it captured before Execute ran).
type ReportView struct {
	FirmwareConfirmed string
	Written           int
	Verified          int
	SkippedBlocked    int
	Unchanged         int
	Slots             []SlotResultView
	Aborted           bool
	AbortReason       string
	JournalPath       string
	SnapshotPath      string
}

// ProgressEvent is transfer:progress's payload, shared by every read/write
// operation's progress callback (send.go's progressCallback). Every event
// carries BOTH the original, channel-specific Slot field AND the
// generalised Target* fields task 35 added:
//
//   - a channel event (ReadRadio/DiffAgainstRadio/PrepareSend/ConfirmSend's
//     per-slot progress): Slot and TargetDisplay carry the IDENTICAL
//     codeplug.DisplaySlot value — Slot stays exactly as it always was (JS
//     compatibility: TestChannelProgress_TargetFieldsAndSlotCompat pins
//     this), TargetKind is "channel", TargetID is the slot's WIRE form;
//   - a settings event (ReadSettingsRadio's per-item progress, phase
//     "read-settings"): Slot is empty (there is no slot at all), TargetKind
//     is "setting", TargetID is the 6-digit setting ID, TargetDisplay is
//     the descriptor's own Display string for that ID (e.g. "01-01-01").
type ProgressEvent struct {
	Phase string
	Done  int
	Total int
	// Slot is already in DISPLAY form (codeplug.DisplaySlot applied) — the
	// pre-task-35 field, populated for a channel event exactly as before,
	// empty for a settings event.
	Slot string
	// TargetKind is "channel" or "setting" — see this type's doc comment.
	TargetKind string
	// TargetID is the stable identifier this progress step just touched:
	// the slot's WIRE form (e.g. "001") for a channel event, or the
	// 6-digit setting ID for a settings event.
	TargetID string
	// TargetDisplay is TargetID's human-readable form — see this type's
	// doc comment for the exact source per TargetKind.
	TargetDisplay string
}

// TransferDoneEvent is transfer:done's payload, emitted exactly once per
// ReadRadio/DiffAgainstRadio/ReadSettingsRadio call and once per
// ConfirmSend transfer (task-15 brief §2; ReadSettingsRadio added by task
// 35). Report is nil unless Kind=="send" AND the radio was actually
// touched (a pure pre-write refusal carries no Report, exactly like
// clone.Service.Execute's own (nil, err) refusal return) — Report is
// therefore ALWAYS nil for Kind=="settings": a settings read never
// produces a clone.Report at all (that type is Execute's own).
type TransferDoneEvent struct {
	// Kind is "read", "diff", "send", or "settings" — which bound method's
	// operation this event reports on. PrepareSend does NOT emit this
	// event: it is synchronous and returns its own SendPlanView/error
	// directly.
	Kind string
	// Outcome is "ok", "aborted", "refused", "error", or "cancelled" —
	// see classifyExecuteOutcome/classifyReadDiffOutcome's doc comments
	// for the exact mapping from clone's error taxonomy (send.go).
	Outcome string
	Report  *ReportView
	// Message is a human-readable explanation, empty on a plain "ok".
	// Abort/transfer wording always says "snapshot", never "backup".
	Message string
}

// ImportResultView is ImportCSV/ImportCHIRP's return value. Every
// content-level outcome (parse error, blocking CHIRP loss, a refused
// merge, or success) is encoded HERE with a nil bound-method error —
// see ImportCSV's doc comment for why: Wails drops a bound method's
// return value entirely when it also returns a non-nil error (the JS
// Promise rejects with only the error string), so the loss
// report/refusal reason would be lost if either travelled via the error
// return instead. The bound method's error return is reserved for
// operational failures with no structured content to preserve (the
// dialog itself failing, an unreadable file).
type ImportResultView struct {
	// Path is the file the user picked; empty if Cancelled.
	Path string
	// Cancelled is true if the user dismissed the file-open dialog
	// without picking anything — nothing else in this struct is
	// meaningful when this is true.
	Cancelled bool
	// ParseError is non-empty iff csvio.Import/ImportCHIRP itself
	// returned a hard error (malformed CSV, missing core columns) —
	// nothing was merged.
	ParseError string
	// LossEntries is ImportCHIRP's loss report, always populated
	// (possibly empty) regardless of what else happened — csvio.
	// ImportCHIRP's own contract (always return the fullest report it
	// can build). Always empty for ImportCSV.
	LossEntries []LossEntryView
	// HasBlockingLoss mirrors csvio.LossReport.HasBlocking().
	HasBlockingLoss bool
	// RefusalReason is non-empty iff the merge itself refused
	// (csvmerge.MergeCSV/MergeCHIRP's inventory-mismatch/unknown-slot/
	// duplicate-slot errors, or a blocking CHIRP loss report) — nothing
	// was merged.
	RefusalReason string
	// Merged is true iff the import was actually applied to the working
	// codeplug.
	Merged bool
	// Issues/Dirty are populated only when Merged is true: the same
	// advisory/authoritative validation UpdateChannel(s) runs, against
	// the working copy as it now stands.
	Issues []IssueView
	Dirty  bool
}

// LossEntryView mirrors one csvio.LossEntry for display.
type LossEntryView struct {
	Line     int
	Column   string
	Value    string
	Action   string
	Detail   string
	Blocking bool
}

// SlotView is one memory slot within a BankView: its canonical wire-form
// identifier (what UpdateChannel/UpdateChannels expect back in
// codeplug.Channel.Slot) and its human-readable display form
// (codeplug.DisplaySlot), e.g. Slot "001" / Display "M-01".
type SlotView struct {
	Slot    string
	Display string
}

// BankView describes one bank of memory slots for the grid (task-17
// brief's controller amendment): its display label, whether the FT-710
// protocol permits writing to it at all (see GetUISpec's doc comment for
// exactly what ReadOnly does and does not mean), and the slots it
// currently holds, in display form.
type BankView struct {
	// ID is the spec.BankID string, e.g. "MEM", "PMS", "60M", "EMG".
	ID    string
	Label string
	// ReadOnly is a PERMANENT protocol fact (every core field's Write is
	// spec.Unsupported), not a hardware-verification status — see
	// bankReadOnly's doc comment. Never true merely because a field is
	// spec.Unverified.
	ReadOnly bool
	Slots    []SlotView
	// TagDisplayDefault is the codeplug.BoolField a row ADDED in this bank
	// must carry for tag_display, derived from this bank's own
	// FieldTagDisplay support — see bankTagDisplayDefault's doc comment
	// for the rule and why the default is per-BANK rather than per-model.
	//
	// It is a codeplug.BoolField, not a bespoke view struct, deliberately:
	// the frontend copies this value STRAIGHT into the ChannelData it
	// sends back (app/frontend/src/lib/grid/columns.js's newChannelData),
	// so it must marshal to the identical JSON shape
	// ChannelData.TagDisplay does — same keys, same omitempty on value.
	// A parallel view type would be one rename away from a default the
	// grid could no longer use.
	TagDisplayDefault codeplug.BoolField
}

// ToneView is one CTCSS tone option: Decihertz is the raw value
// ChannelData.CTCSSTone/UpdateChannel(s) expect back; Display is the
// human-readable form, e.g. "88.5 Hz" (spec.Tone.String()'s own format,
// one decimal place).
type ToneView struct {
	Decihertz int
	Display   string
}

// PreservationTooltipsView mirrors internal/radiotext.PreservationTooltips
// for display: the channel grid's per-column preserved-cell tooltip text
// for the Tone and Scan Skip columns — see UISpecView.PreservationTooltips'
// own doc comment.
type PreservationTooltipsView struct {
	Tone     string
	ScanSkip string
}

// UISpecView is GetUISpec's return value: every piece of FT-710-specific
// data the frontend grid needs (bank writability, slot display mapping,
// the Mode/Shift/CTCSS-state/tone edit vocabularies, and — task 41, M9a-5
// — the grid's radio-specific PROSE) so none of it need be hardcoded in
// JS. See GetUISpec's doc comment for the full derivation.
type UISpecView struct {
	// Live is true when this UISpecView was built from a connected
	// session's own effective capabilities (authoritative — includes
	// discovered 60m/EMG inventory); false when built from the static
	// offline baseline.
	Live bool
	// UnverifiedWritesConsented is true when the CONNECTED session's own
	// capabilities carry a write-side consented-unverified label anywhere —
	// the amber state the frontend renders while a session is armed to
	// write fields this project has never proved on hardware.
	//
	// Derived from the session's LABELS, never from the settings file and
	// never from the model: consent is spent when a session is
	// CONSTRUCTED, so a decision changed afterwards (by the CLI, mid-
	// session — userconfig's documented last-writer-wins) does not change
	// what the running session will let through, and the indicator must
	// follow the session rather than the file. Always false offline and in
	// demo: a static baseline describes the radio, not a decision, and a
	// simulator session takes no consent option.
	UnverifiedWritesConsented bool
	Banks                     []BankView
	Modes                     []string
	ShiftOptions              []string
	CTCSSStateOptions         []string
	Tones                     []ToneView
	TagMaxBytes               int
	ClarMaxHz                 int
	ClarStepHz                int
	// ToneScanSkipNote is the channel grid's standing legend explaining
	// why the Tone/Scan Skip columns exist but cannot be read back over
	// CAT — served from internal/radiotext.Text.ToneScanSkipNote and
	// rendered by the frontend from this field.
	ToneScanSkipNote string
	// ToneScanSkipVerification is the grid legend's second sentence,
	// stating what is and is not hardware-verified about Tone/Scan Skip
	// preservation across a rewrite for this radio — served from
	// internal/radiotext.Text.ToneScanSkipVerification. Left in the
	// frontend by task 41 when it migrated ToneScanSkipNote; moved here
	// to close ledger minor m42a, since it is a claim about THIS radio's
	// write trials that a future model pinned at
	// writeTrialsComplete=false could not truthfully make.
	ToneScanSkipVerification string
	// EraseDialogNote is the "no CAT erase command" explanation shown when
	// a user asks to delete a channel, or reviews a blocked-erase entry
	// before sending — served from internal/radiotext.Text.EraseDialogNote
	// (task 41).
	EraseDialogNote string
	// PreservationTooltips holds the per-column preserved-cell tooltips
	// the channel grid shows for its Tone and Scan Skip columns — served
	// from internal/radiotext.Text.PreservationTooltips (task 41).
	PreservationTooltips PreservationTooltipsView
	// FirmwarePlaceholder is the send-flow firmware-version input's
	// placeholder text — served from
	// internal/radiotext.Text.FirmwarePlaceholder (task 41).
	FirmwarePlaceholder string
}

// SettingItemView is one leaf setting within a SettingGroupView: the unit
// GetSettings/ReadSettingsRadio's SettingEntryView.ID addresses. Display is
// the driver's own human-facing rendering of the setting's position (e.g.
// the FT-710's "01-01-01", driver.SettingItem.Display's own value) — for
// display only, never re-parsed as an ID.
type SettingItemView struct {
	ID      string
	Label   string
	Display string
}

// SettingGroupView is one subgroup within a SettingMenuView (e.g. the
// FT-710 EX subgroup P1=01/P2=01 "MODE SSB").
type SettingGroupView struct {
	ID    string
	Label string
	Items []SettingItemView
}

// SettingMenuView is one top-level settings menu (e.g. the FT-710 EX menu
// P1=01 "RADIO SETTING") — mirrors driver.SettingsDescriptor's own
// two-level Menu/Group/Item hierarchy (core/driver/settings.go) with no
// protocol fact added or invented here (see GetSettingsSpec's doc
// comment).
type SettingMenuView struct {
	ID     string
	Label  string
	Groups []SettingGroupView
}

// SettingsSpecView is GetSettingsSpec's return value: the settings tree's
// STRUCTURE only — menus, groups, and items, never a VALUE (see
// SettingsView for content). Live mirrors UISpecView.Live's own
// connected/disconnected split: true when built from the connected
// session's own driver.SettingsReader descriptor (authoritative), false
// when built from the static ft710.SettingsDescriptor() baseline (works
// offline — always available, even with no session). DescriptorVersion is
// the descriptor's own Version string (e.g. "ft710-ex@1") — the same
// value a MenuSnapshot a ReadSettingsRadio built from THIS descriptor
// would carry as its own Descriptor field.
type SettingsSpecView struct {
	Live              bool
	DescriptorVersion string
	Menus             []SettingMenuView
}

// SettingEntryView mirrors one codeplug.MenuEntry for display. State is
// one of "known" (Value is the radio's last successfully read raw value),
// "unavailable" (the radio rejected the read; Value is empty), or
// "unsupported" (this ID is not in the current build's descriptor but was
// carried forward from an older snapshot by codeplug.MergeMenuSnapshots —
// Value is whatever that older read had captured, possibly empty).
type SettingEntryView struct {
	ID    string
	Value string
	State string
}

// SettingsView is GetSettings/ReadSettingsRadio's return value: the
// CONTENT of the working copy's menu/EX settings snapshot (never its
// structure — see SettingsSpecView for that). HasSnapshot is false only
// when the working copy has never had its settings read at all
// (a.working.Menus == nil) — settings acquisition is opt-in (task 33), so
// this is an entirely ordinary state, not an error. HasLegacy mirrors
// whether the snapshot carries a migrated v1 opaque payload
// (codeplug.MenuSnapshot.Legacy non-empty) — the raw legacy bytes
// themselves are NEVER shipped to JS, only this boolean. Entries are
// mapped 1:1 from the snapshot, in its own stored order.
type SettingsView struct {
	HasSnapshot bool
	Descriptor  string
	Complete    bool
	HasLegacy   bool
	Entries     []SettingEntryView
}
