// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// bankCoreFields is the seven memory-channel fields the grid actually
// edits via UpdateChannel/UpdateChannels (task-17 brief's controller
// amendment): frequency, mode, clarifier, ctcss_state, shift, tag,
// tag_display. Deliberately excludes ctcss_tone/scan_skip/erase — the
// CAT protocol can never read OR write ctcss_tone/scan_skip on ANY bank,
// on ANY profile (core/driver/ft710/caps.go's bankFields doc comment:
// "the CAT protocol has no way to read OR write a memory channel's tone
// or scan skip"), and erase is a distinct write-time concern — so
// folding any of the three into this test would make a bank's
// editability turn on fields the grid never renders as an editable
// column, for the wrong reason.
var bankCoreFields = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldCTCSSState, spec.FieldShift, spec.FieldTag, spec.FieldTagDisplay,
}

// bankReadOnly reports whether the bank identified by id is READ-ONLY as
// a PERMANENT protocol fact: true iff every one of bankCoreFields has
// Write == spec.Unsupported for that bank (spec.Capabilities.FieldSupport
// already returns the zero FieldSupport — Unsupported/Unsupported — for
// a bank absent from caps entirely, or a field absent from a present
// bank's Fields map, so both cases fall out of the same loop with no
// special-casing).
//
// This is deliberately NOT "has this field been hardware-verified yet".
// spec.Unverified — documented in the manual/assumed by analogy, but
// not proven against a physical FT-710 — is a DIFFERENT state from
// spec.Unsupported (the radio structurally cannot accept a write here
// at all, e.g. the discovered 60m/EMG banks, whose Write is
// unconditionally forced to Unsupported by
// core/driver/ft710.effectiveCapabilities, on every profile, verified
// or not); and so is spec.Inert (the M5b-added transmitted-but-ignored
// state the clarifier now carries — an Inert column stays editable
// too, with a CHANGED value caught at send time by codeplug.Diff, not
// by locking the cell). Treating Unverified as ReadOnly would have
// locked MEM/PMS editing before the very M5b hardware trials that
// unlocked it (13/07/2026: writeTrialsComplete flipped;
// core/driver/ft710/caps.go) — breaking the offline clone workflow this
// project exists for. Send-time write gating
// (spec.FieldSupport.CanWrite, false for Unverified and Inert alike) is
// a separate, already-enforced concern (codeplug.Diff, clone.Service,
// Session.WriteChannel) — this derivation only answers "can the grid
// let the user type into this cell at all", not "will a send actually
// reach the radio".
func bankReadOnly(caps spec.Capabilities, id spec.BankID) bool {
	for _, f := range bankCoreFields {
		if caps.FieldSupport(id, f).Write != spec.Unsupported {
			return false
		}
	}
	return true
}

// slotViewsFor maps a bare slot-identifier list (a spec.Bank.Slots value)
// into display-form SlotViews, preserving order.
func slotViewsFor(slots []string) []SlotView {
	out := make([]SlotView, 0, len(slots))
	for _, s := range slots {
		out = append(out, SlotView{Slot: s, Display: codeplug.DisplaySlot(s)})
	}
	return out
}

// bankSlotViews builds bank's []SlotView. See GetUISpec's doc comment for
// the full three-branch rule this implements; live is whether caps (and
// therefore bank) came from a connected session's own effective
// capabilities (authoritative — reflects discovered inventory), and
// working is the App's current working copy (nil if none loaded).
func bankSlotViews(bank spec.Bank, live bool, working *codeplug.Codeplug) []SlotView {
	if live || working == nil {
		// Connected: caps' own slot list is authoritative, regardless of
		// what the working copy holds. Disconnected with nothing loaded:
		// the static baseline's own list, as-is (possibly empty/absent
		// for 60M/EMG — see GetUISpec's doc comment).
		return slotViewsFor(bank.Slots)
	}
	// Disconnected with a working copy loaded: classify the working
	// copy's OWN slots against the static bank's membership, so the
	// grid's rows agree exactly with what ReadRadio/LoadFile produced —
	// not the static list wholesale (which could include a slot the
	// working copy does not actually carry, though in practice it always
	// will for MEM/PMS).
	member := make(map[string]bool, len(bank.Slots))
	for _, s := range bank.Slots {
		member[s] = true
	}
	var out []SlotView
	for _, ch := range working.Channels {
		if member[ch.Slot] {
			out = append(out, SlotView{Slot: ch.Slot, Display: codeplug.DisplaySlot(ch.Slot)})
		}
	}
	return out
}

// synthesiseDiscoveredBanks builds read-only 60M/EMG BankViews for the
// OFFLINE + working-copy case (controller adjudication on the task-17
// GetUISpec work: loaded data must never be invisible in the UI). The
// static offline baseline carries no 60M/EMG bank definitions at all —
// their inventory is region-dependent and only ever DISCOVERED per live
// session — so a working copy loaded from an earlier read of, say, a
// US-region radio holds 60m/EMG channels that no caps bank claims while
// disconnected. Since the grid's tabs come solely from BankViews, those
// channels would otherwise render nowhere.
//
// Task 41 (M9a-5, the GUI-backend neutralisation) migrates this off the
// local cat.ParseSlot-based classification onto
// wiring.SynthesiseDiscoveredBanks(wiring.DefaultModel, ...) — the
// driver.DiscoveredBankSynthesizer capability (core/driver/optional.go),
// introduced task 37 for exactly this call site. That function already
// excludes any slot claimed by wiring.DefaultModel's own static banks
// (MEM/PMS) before classifying the rest, which subsumes the OLD
// alreadyPresent double-emission guard: a future static profile that DID
// define a 60M/EMG bank would simply leave nothing left to synthesise for
// it, never a duplicate. ok is false only for an unrecognised model or one
// whose driver lacks the capability — neither true for wiring.DefaultModel
// today — in which case this returns nil (no synthesised banks) rather
// than inventing any.
//
// ReadOnly is unconditionally true for every synthesised bank: MW cannot
// target 5xx/EMG slots at all (the wire-protocol fact
// core/driver/ft710.effectiveCapabilities' Field map already encodes by
// forcing every Write to Unsupported for these banks, on every profile,
// whenever a live session does discover them).
func synthesiseDiscoveredBanks(working *codeplug.Codeplug) []BankView {
	slots := make([]string, len(working.Channels))
	for i, ch := range working.Channels {
		slots[i] = ch.Slot
	}
	discovered, ok := wiring.SynthesiseDiscoveredBanks(wiring.DefaultModel, slots)
	if !ok {
		return nil
	}
	out := make([]BankView, 0, len(discovered))
	for _, b := range discovered {
		out = append(out, BankView{
			ID:       string(b.ID),
			Label:    b.Label,
			ReadOnly: true,
			Slots:    slotViewsFor(b.Slots),
		})
	}
	return out
}

// GetUISpec returns everything the frontend channel grid needs to render
// bank tabs, per-slot rows, and edit-column option lists WITHOUT
// hardcoding any FT-710 protocol fact in JS (task-17 brief's controller
// amendment: "the original text claimed the views already exposed
// writability — they did not"): per-bank writability, slot display
// mapping, and the Mode/Shift/CTCSS-state/tone edit vocabularies.
//
// Capabilities source (UISpecView.Live reports which one was used, same
// rule as currentCaps, restated here rather than shared since
// currentCaps' "advisory" bool carries Validate-specific meaning): the
// connected session's OWN effective capabilities (authoritative —
// includes discovered 60m/EMG inventory) when connected, otherwise the
// static offline baseline (ft710.New(ft710.RealHardware).Capabilities()).
//
// BankView.ReadOnly: see bankReadOnly's doc comment — a PERMANENT
// protocol fact, never merely "not yet hardware-verified".
//
// BankView.Slots (kept deliberately simple, per bank):
//   - Connected (Live true): bank.Slots (from caps) is authoritative —
//     it already reflects the session's own discovered inventory — used
//     as-is regardless of whether a working copy is loaded.
//   - Disconnected with a working copy loaded: the working copy's own
//     slots, filtered to membership in the STATIC baseline's bank slot
//     list; PLUS synthesised read-only 60M/EMG banks for any 60m/EMG
//     slots the working copy holds (e.g. loaded from an earlier read of
//     a US-region radio) — the static baseline defines no 60M/EMG bank,
//     and loaded channels must never be invisible in the grid. See
//     synthesiseDiscoveredBanks. Every working-copy slot therefore
//     appears in exactly one BankView, so the grid's rows agree with
//     what ReadRadio/LoadFile actually produced.
//   - Disconnected with no working copy: the static baseline's bank slot
//     list, as-is (no 60M/EMG banks — there is nothing loaded that could
//     need them, and no discovered inventory to assert).
func (a *App) GetUISpec() (UISpecView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	caps, advisory := currentCaps(a.conn)
	live := !advisory

	banks := make([]BankView, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		banks = append(banks, BankView{
			ID:       string(b.ID),
			Label:    b.Label,
			ReadOnly: bankReadOnly(caps, b.ID),
			Slots:    bankSlotViews(b, live, a.working),
		})
	}
	if !live && a.working != nil {
		// wiring.SynthesiseDiscoveredBanks (called inside
		// synthesiseDiscoveredBanks) already excludes any slot claimed by
		// wiring.DefaultModel's own static banks before classifying the
		// rest, so no separate "already present" guard is needed here —
		// see that function's doc comment.
		banks = append(banks, synthesiseDiscoveredBanks(a.working)...)
	}

	tones := make([]ToneView, 0, len(caps.CTCSSTones))
	for _, t := range caps.CTCSSTones {
		tones = append(tones, ToneView{Decihertz: int(t), Display: t.String()})
	}

	// ShiftOptions/CTCSSStateOptions come straight from caps (task 41,
	// M9a-5: previously restated literals cross-checked against
	// core/codeplug/validate.go's Validate by
	// TestGetUISpec_VocabMatchesValidate — task 38 landed
	// spec.Capabilities.ShiftOptions/CTCSSStates as the same vocabulary's
	// authoritative, driver-populated home, so this now reads it rather
	// than restating it. CTCSSStateOptions extracts each ToneState's Value,
	// preserving caps' own order; the RequiresTone fact each state also
	// carries is not needed by the grid's option list today.
	shiftOptions := append([]string(nil), caps.ShiftOptions...)
	ctcssStateOptions := make([]string, len(caps.CTCSSStates))
	for i, s := range caps.CTCSSStates {
		ctcssStateOptions[i] = s.Value
	}

	// Prose fields (task 41, M9a-5): served from internal/radiotext rather
	// than hardcoded in this package or the frontend — see UISpecView's
	// doc comment (types.go) for what each field is and its exact source.
	// radiotext.For(wiring.DefaultModel) cannot fail for the hardcoded
	// FT-710 entry in practice; a future model with no radiotext entry yet
	// would fall back to the zero Text (every field empty) rather than
	// invented wording.
	text, _ := radiotext.For(wiring.DefaultModel)

	return UISpecView{
		Live:              live,
		Banks:             banks,
		Modes:             append([]string(nil), caps.Modes...),
		ShiftOptions:      shiftOptions,
		CTCSSStateOptions: ctcssStateOptions,
		Tones:             tones,
		TagMaxBytes:       caps.TagLen,
		ClarMaxHz:         caps.ClarMaxHz,
		ClarStepHz:        caps.ClarStepHz,
		ToneScanSkipNote:  text.ToneScanSkipNote,
		EraseDialogNote:   text.EraseDialogNote,
		PreservationTooltips: PreservationTooltipsView{
			Tone:     text.PreservationTooltips.Tone,
			ScanSkip: text.PreservationTooltips.ScanSkip,
		},
		FirmwarePlaceholder: text.FirmwarePlaceholder,
	}, nil
}
