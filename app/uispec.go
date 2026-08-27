// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// bankCoreCandidates is the CANDIDATE universe bankCoreFields derives a
// bank's core set from: the nine spec.Fields the channel grid renders as
// editable DATA columns and sends back through
// UpdateChannel/UpdateChannels (task-17 brief's controller amendment).
// It is the grid's own column list minus the Slot column, in
// app/frontend/src/lib/grid/columns.js's COLUMNS order — a STRUCTURAL
// fact about this application's UI, not a claim about any radio.
//
// spec.FieldErase is excluded STRUCTURALLY, and the exclusion cannot be
// left to the zero-value test below (M9c-6 D5a). Erase is not a
// codeplug.ChannelData field and not a grid column at all: it is a
// write-time concern with its own gate (codeplug.Diff's erase check), and
// no cell anywhere edits it. It is also not reliably zero — the FT-710's
// fail-safe profile declares MEM erase {Read: Unsupported, Write:
// Unverified} (core/driver/ft710/caps.go's CapabilitiesUnverified), which
// is NON-zero — so a derivation that admitted every non-zero field would
// quietly re-admit erase on exactly that profile and make a bank's
// editability turn on a field the grid never renders. Membership must be
// decided by what the grid EDITS first, and only then by what the radio
// supports.
var bankCoreCandidates = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldShift, spec.FieldCTCSSState, spec.FieldCTCSSTone,
	spec.FieldScanSkip, spec.FieldTag, spec.FieldTagDisplay,
}

// bankCoreFields derives the core field set of the bank identified by id:
// every bankCoreCandidates entry whose FieldSupport on THAT BANK is
// non-zero, in candidate order.
//
// Non-zero means "this radio's memory frame carries the field on this
// bank", in either direction and to any degree of confidence — Unverified,
// ConsentedUnverified, Inert and read-but-not-write all count, since each
// describes a field that EXISTS (consent changes whose word the confidence
// rests on, the user's rather than the hardware's, never whether the frame
// has the field). Only the zero FieldSupport (Unsupported both ways) says the
// frame has no such field, and spec.Capabilities.FieldSupport returns
// exactly that for a bank absent from caps entirely or a field absent from
// a present bank's map, so "says nothing" and "says no" answer alike with
// no special-casing.
//
// PER BANK, from the capability data, replacing the fixed seven-field list
// this file carried until M9c-6 (D5a). That list was right for its one
// radio and justified by an FT-710 doc-comment citation — "the CAT
// protocol has no way to read OR write a memory channel's tone or scan
// skip" — which is a fact about the FT-710's protocol, not a universal
// truth, and reading it as one is what a second registered radio makes
// visible: the FTdx10's memory frame has no display flag at all
// (core/driver/ftdx10/caps.go's bankFields), so tag_display belongs in ITS
// core set no more than tone does in the FT-710's. A radio whose frame DID
// carry a writable tone would, symmetrically, have to have it counted.
// Derivation answers all three cases from the same rule, and the citation
// is no longer load-bearing for any radio but the one it describes.
//
// What this deliberately is NOT: per-CELL editability from capabilities.
// The grid's per-cell rules stay state-based (columns.js's isCellEditable
// — a FieldState question), and that is ledgered with the model picker,
// not narrowed here. This derivation feeds bankReadOnly's WHOLE-BANK
// verdict and nothing else.
func bankCoreFields(caps spec.Capabilities, id spec.BankID) []spec.Field {
	out := make([]spec.Field, 0, len(bankCoreCandidates))
	for _, f := range bankCoreCandidates {
		if caps.FieldSupport(id, f) != (spec.FieldSupport{}) {
			out = append(out, f)
		}
	}
	return out
}

// bankReadOnly reports whether the bank identified by id is READ-ONLY as
// a PERMANENT protocol fact: true iff every field in that bank's derived
// core set (bankCoreFields) has Write == spec.Unsupported. A bank whose
// derived set is EMPTY — nothing the grid edits exists in this radio's
// frame there at all, which is also what an absent bank looks like — is
// vacuously read-only, and rightly: there is nothing to type into.
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
// by locking the cell); and so, a fortiori, is spec.ConsentedUnverified
// (a consented session's write label — a state the user has explicitly
// asked to be able to write, which could hardly justify locking the
// cell, and which this Write != Unsupported test admits for the same
// reason it admits the other three). Treating Unverified as ReadOnly would have
// locked MEM/PMS editing before the very M5b hardware trials that
// unlocked it (13/07/2026: writeTrialsComplete flipped;
// core/driver/ft710/caps.go) — breaking the offline clone workflow this
// project exists for. Send-time write gating
// (spec.FieldSupport.CanWrite, false for Unverified and Inert alike) is
// a separate, already-enforced concern (codeplug.Diff, clone.Service,
// Session.WriteChannel) — this derivation only answers "can the grid
// let the user type into this cell at all", not "will a send actually
// reach the radio".
//
// M9c-6 divergence, recorded rather than silently resolved: the milestone
// spec's D5a states as a CONSEQUENCE that a real (RealHardware-profile)
// FTdx10 yields ReadOnly true on every bank — "a read-only grid pre-trials
// is CORRECT". Under the rule above it does not, and cannot: that
// profile's MEM/PMS fields are Write spec.Unverified
// (core/driver/ftdx10's writeTrialsComplete is false, so RealHardware
// selects CapabilitiesUnverified), Unverified is not Unsupported, and the
// paragraph above is the standing adjudication that says so. Making the
// spec's sentence true would mean re-testing on CanWrite() instead —
// reversing that adjudication for every radio, re-locking the FT-710's own
// fail-safe profile, and contradicting this package's own pinned case
// ("all Unverified -> not read-only (awaiting hardware trials, not
// locked)", TestBankReadOnly_Table). D5a's operative rule changes the
// candidate SET only, so the set is all this implements; the observed
// per-bank verdicts for a registered FTdx10 are pinned exactly as they
// are by TestBankReadOnly_RegisteredFTdx10_RealHardwareProfile, and which
// of the two rules the project wants is an adjudication, not an
// implementation detail.
func bankReadOnly(caps spec.Capabilities, id spec.BankID) bool {
	for _, f := range bankCoreFields(caps, id) {
		if caps.FieldSupport(id, f).Write != spec.Unsupported {
			return false
		}
	}
	return true
}

// bankTagDisplayDefault derives the tag_display value a row ADDED in the
// bank identified by id must carry (BankView.TagDisplayDefault), from that
// bank's own spec.FieldTagDisplay support and nothing else:
//
//   - Read AND Write both spec.Unsupported (spec.FieldSupport.Unreachable)
//     → codeplug.Unavailable. This radio's memory frame has no display
//     flag at all, so there is no value to hold: Unavailable is what
//     BoolField means by that (see core/codeplug/fieldstate.go), and it is
//     never sent.
//   - anything else → {codeplug.Known, false}. The flag exists in the
//     frame, so a blank row states it, and states it OFF.
//
// The asymmetry is deliberate and it is the whole point: only "absent from
// the frame in BOTH directions" justifies Unavailable. A field that is
// merely unwritable (the discovered 60M/EMG banks, whose Write is forced
// Unsupported while Read is inherited from MEM), merely unproven
// (spec.Unverified), unproven-but-consented (spec.ConsentedUnverified),
// or transmitted-but-ignored (spec.Inert) is still a field this radio's
// frame carries, and a blank row must state it rather than claim the
// radio has no such flag.
//
// Known-false, rather than the honest-provenance codeplug.Unknown, because
// tag_display is a MANDATORY wire field wherever it exists: an Unknown one
// blocks its channel at plan time (codeplug.Diff's TagDisplay gate), so a
// factory that produced Unknown would create rows the send plan
// immediately refuses. That was already the frontend's rule; what changes
// here (M9c-5 review W1) is only WHERE the value comes from — this
// derivation, per bank, instead of a JS literal that spoke for the FT-710
// on every radio's behalf.
//
// PER-BANK, not per-model, and that is finer than the design text asked
// for ("the GUI's blank-row factory defaults per-model"): support is
// declared per bank by spec.Capabilities, a radio may perfectly well carry
// the flag on one bank and not another, and answering per model would have
// had to pick one bank's truth for all of them. Per bank subsumes per
// model at no cost — every bank of a radio that carries the flag
// everywhere gets the same answer.
//
// The zero-value lookup does the work for both "bank absent from caps
// entirely" and "bank present but not listing the field"
// (spec.Capabilities.FieldSupport returns the zero FieldSupport for
// either), so both fall out as Unavailable with no special-casing: caps
// that say nothing about a display flag are not evidence that one exists.
//
// The two-comparison predicate itself is spec.FieldSupport.Unreachable
// since M9d-2 task 8, shared with core/csvio's chirpTagDisplay (which
// M9c-6 deliberately duplicated, noting that a THIRD caller should force
// the move) and its new chirpScanSkip, which was that third caller. Only
// the QUESTION is shared: what each site answers still differs, and this
// one's Known-false answer is justified above.
func bankTagDisplayDefault(caps spec.Capabilities, id spec.BankID) codeplug.BoolField {
	if caps.FieldSupport(id, spec.FieldTagDisplay).Unreachable() {
		return codeplug.BoolField{State: codeplug.Unavailable}
	}
	return codeplug.BoolField{State: codeplug.Known, Value: false}
}

// tierFields is every spec.Field the Icom tier added (design D4), in
// codeplug.ChannelData's own declaration order — which is the order the
// grid renders their columns in.
//
// It is a list rather than a derivation because there is no way to ask
// spec "which fields did the Icom tier add": the distinction is
// historical, and the reason it matters here is that the pre-tier ten
// have unconditional columns while these ten do not.
var tierFields = []spec.Field{
	spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
	spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
	spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter,
	spec.FieldDataMode,
}

// bankTierFields returns, in tierFields order, every tier-added field the
// bank identified by id can REACH — spec.FieldSupport.Unreachable false,
// so "the frame has this field" in either direction and to any degree of
// confidence, exactly the test bankCoreFields and bankTagDisplayDefault
// already use for their own questions.
//
// The zero-value lookup covers "bank absent from caps entirely" and
// "bank present but not listing the field" alike, so both answer "not
// reachable" with no special-casing — caps that say nothing about a
// field are not evidence that the radio has one.
//
// AN OPEN LIST, NOT A SNAPSHOT: every bank of all four registered Yaesu
// models reaches none of the ten tier fields, so their grids' column sets
// stay empty; every registered Icom model returns its OWN bank's own
// reachable set, independently derived from that model's own record —
// four for the IC-7610 (tone_mode, tone_tx, tone_rx, filter, pinned by
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay), and six
// each for the IC-7300 and IC-7300MK2 (the IC-7610's four plus
// tx_frequency and data_mode, pinned by
// TestGetUISpec_RegisteredIC7300_EveryBankFieldsAndTagDisplay and its
// MK2 mirror). A future Icom registration extends this same list with
// its own model-specific set; nothing here needs to change for it to.
func bankTierFields(caps spec.Capabilities, id spec.BankID) []string {
	var out []string
	for _, f := range tierFields {
		if caps.FieldSupport(id, f).Unreachable() {
			continue
		}
		out = append(out, string(f))
	}
	return out
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
// local cat.Dialect.ParseSlot-based classification onto
// wiring.SynthesiseDiscoveredBanks — the driver.DiscoveredBankSynthesizer
// capability (core/driver/optional.go), introduced task 37 for exactly
// this call site. That function already excludes any slot claimed by
// model's own static banks (MEM/PMS) before classifying the rest, which
// subsumes the OLD alreadyPresent double-emission guard: a future static
// profile that DID define a 60M/EMG bank would simply leave nothing left
// to synthesise for it, never a duplicate. ok is false only for an
// unrecognised model or one whose driver lacks the capability, in which
// case this returns nil (no synthesised banks) rather than inventing any.
//
// model is currentModel's resolved answer (M9c-5 E4), passed in rather
// than re-derived here, and that indirection is load-bearing: the raw
// working.Radio.Model would be handed straight through to
// wiring.SynthesiseDiscoveredBanks, so a working copy naming a model no
// driver is registered for — a legacy or hand-edited file — would take
// the ok == false branch and silently drop its 60m/EMG channels out of
// the grid entirely, which is the very outcome this function exists to
// prevent. currentModel falls back to wiring.DefaultModel for exactly
// that case, so those channels stay visible.
//
// ReadOnly is unconditionally true for every synthesised bank: MW cannot
// target 5xx/EMG slots at all (the wire-protocol fact
// core/driver/ft710.effectiveCapabilities' Field map already encodes by
// forcing every Write to Unsupported for these banks, on every profile,
// whenever a live session does discover them).
func synthesiseDiscoveredBanks(model string, working *codeplug.Codeplug) []BankView {
	slots := make([]string, len(working.Channels))
	for i, ch := range working.Channels {
		slots[i] = ch.Slot
	}
	discovered, ok := wiring.SynthesiseDiscoveredBanks(model, slots)
	if !ok {
		return nil
	}
	// The synthesised banks' OWN capabilities, for the per-bank
	// tag-display derivation below. It has to be these rather than the
	// static baseline caps GetUISpec holds: the baseline defines no 60M/EMG
	// bank at all, so looking the field up there would answer Unavailable
	// for every synthesised bank — claiming, of the very same radio, that
	// its 60m channels have no display flag while a LIVE session's
	// discovered banks (which inherit MEM's read supports — see
	// core/driver/ft710.effectiveCapabilities) report that they do. The
	// synthesis exists precisely to agree with live discovery, and the
	// spec.Bank values wiring.SynthesiseDiscoveredBanks returns carry the
	// same Fields maps live discovery would have produced.
	discoveredCaps := spec.Capabilities{Banks: discovered}
	out := make([]BankView, 0, len(discovered))
	for _, b := range discovered {
		out = append(out, BankView{
			ID:                string(b.ID),
			Label:             b.Label,
			ReadOnly:          true,
			Slots:             slotViewsFor(b.Slots),
			TagDisplayDefault: bankTagDisplayDefault(discoveredCaps, b.ID),
			Fields:            bankTierFields(discoveredCaps, b.ID),
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
// static offline baseline of the model currentModel resolves.
//
// BankView.ReadOnly: see bankReadOnly's doc comment — a PERMANENT
// protocol fact, never merely "not yet hardware-verified".
//
// BankView.TagDisplayDefault: see bankTagDisplayDefault's doc comment —
// the blank-row tag_display value, derived per bank from this radio's own
// FieldTagDisplay support, so the grid's Added-row factory no longer
// speaks for the FT-710 on every radio's behalf (M9c-5 review W1).
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

	caps, advisory := currentCaps(a.conn, a.working)
	live := !advisory
	// The ONE resolver (M9c-5 E4), consulted ONCE per call: the
	// synthesised-bank classification below and the prose lookup further
	// down must describe the same radio as caps, and resolving twice would
	// let them disagree.
	model := currentModel(a.conn, a.working)

	banks := make([]BankView, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		banks = append(banks, BankView{
			ID:                string(b.ID),
			Label:             b.Label,
			ReadOnly:          bankReadOnly(caps, b.ID),
			Slots:             bankSlotViews(b, live, a.working),
			TagDisplayDefault: bankTagDisplayDefault(caps, b.ID),
			Fields:            bankTierFields(caps, b.ID),
		})
	}
	if !live && a.working != nil {
		// wiring.SynthesiseDiscoveredBanks (called inside
		// synthesiseDiscoveredBanks) already excludes any slot claimed by
		// the resolved model's own static banks before classifying the
		// rest, so no separate "already present" guard is needed here —
		// see that function's doc comment.
		banks = append(banks, synthesiseDiscoveredBanks(model, a.working)...)
	}

	// THE TONE PICKER IS LIST-DRIVEN, AND ON A RANGE-DECLARING RADIO IT
	// IS EMPTY. That is a RECORDED COST of the Icom tier's E3, not an
	// oversight: spec.Capabilities gained an optional numeric
	// CTCSSToneRange for models whose tone field is a number rather than
	// an index into a chart, and this picker enumerates a chart. A
	// range-declaring model's grid still SHOWS and ROUND-TRIPS whatever
	// tones its channels carry — validation and CHIRP import both ask
	// spec.Capabilities.AdmitsTone, which knows both shapes — but this
	// list has nothing to offer, so the user cannot PICK one here. A
	// numeric tone editor is the Wave-4 item that closes it; enumerating
	// a range into a pick-list of hundreds of entries is not it.
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
	// than restating it. ShiftOptions extracts each ShiftOption's Value,
	// preserving caps' own order; the Direction each option also carries
	// is not needed by the grid's option list today. CTCSSStateOptions
	// extracts each ToneState's Value, preserving caps' own order; the
	// RequiresTone fact each state also carries is not needed by the
	// grid's option list today.
	shiftOptions := make([]string, len(caps.ShiftOptions))
	for i, o := range caps.ShiftOptions {
		shiftOptions[i] = o.Value
	}
	ctcssStateOptions := make([]string, len(caps.CTCSSStates))
	for i, s := range caps.CTCSSStates {
		ctcssStateOptions[i] = s.Value
	}

	// Prose fields (task 41, M9a-5): served from internal/radiotext rather
	// than hardcoded in this package or the frontend — see UISpecView's
	// doc comment (types.go) for what each field is and its exact source.
	// Keyed off the resolved model (M9c-5 E4), and radiotext.For's ok is
	// HONOURED rather than discarded: a model with no radiotext entry
	// leaves every prose field empty — silence — exactly as cmd/rigprog's
	// own prose sites do (probe.go's ProbeFirmwareNote, write.go's
	// EraseProcedure, both `if text, ok := radiotext.For(model); ok`).
	// Never another radio's wording, and never a fabricated generic
	// sentence.
	var text radiotext.Text
	if t, ok := radiotext.For(model); ok {
		text = t
	}

	return UISpecView{
		Live: live,
		// The amber state, read from the capability set in hand and nothing
		// else — see UISpecView.UnverifiedWritesConsented and
		// consentedUnverifiedWrites (consent.go). Offline and demo answer
		// false through the same call: only a session a driver assembled
		// under a spent consent carries the label.
		UnverifiedWritesConsented: consentedUnverifiedWrites(caps),
		Banks:                     banks,
		Modes:                     append([]string(nil), caps.Modes...),
		ShiftOptions:              shiftOptions,
		CTCSSStateOptions:         ctcssStateOptions,
		Tones:                     tones,
		TagMaxBytes:               caps.TagLen,
		ClarMaxHz:                 caps.ClarMaxHz,
		ClarStepHz:                caps.ClarStepHz,
		ToneScanSkipNote:          text.ToneScanSkipNote,
		ToneScanSkipVerification:  text.ToneScanSkipVerification,
		EraseDialogNote:           text.EraseDialogNote,
		PreservationTooltips: PreservationTooltipsView{
			Tone:     text.PreservationTooltips.Tone,
			ScanSkip: text.PreservationTooltips.ScanSkip,
		},
		FirmwarePlaceholder: text.FirmwarePlaceholder,
	}, nil
}
