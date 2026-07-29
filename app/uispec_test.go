// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// findBank returns the BankView with the given ID, or fails the test.
func findBank(t *testing.T, banks []BankView, id string) BankView {
	t.Helper()
	for _, b := range banks {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no bank with ID %q in %v", id, bankIDs(banks))
	return BankView{}
}

func bankIDs(banks []BankView) []string {
	out := make([]string, len(banks))
	for i, b := range banks {
		out[i] = b.ID
	}
	return out
}

func slotSet(slots []SlotView) map[string]string {
	out := make(map[string]string, len(slots))
	for _, s := range slots {
		out[s.Slot] = s.Display
	}
	return out
}

// TestBankReadOnly_Table is a pure unit test of bankReadOnly against
// hand-built spec.Capabilities, independent of App/session plumbing —
// table-driven per the task's TDD requirement.
func TestBankReadOnly_Table(t *testing.T) {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	unverified := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	unsupported := spec.FieldSupport{}

	allWritable := map[spec.Field]spec.FieldSupport{}
	allUnverified := map[spec.Field]spec.FieldSupport{}
	allUnsupported := map[spec.Field]spec.FieldSupport{}
	mixedOneWritable := map[spec.Field]spec.FieldSupport{}
	inertClarifier := map[spec.Field]spec.FieldSupport{}
	for _, f := range bankCoreFields {
		allWritable[f] = rw
		allUnverified[f] = unverified
		allUnsupported[f] = unsupported
		mixedOneWritable[f] = unsupported
		inertClarifier[f] = rw
	}
	mixedOneWritable[spec.FieldFrequency] = rw
	// The M5b real shape: every core field writable except the clarifier,
	// whose Write is Inert (HW-CONFIRMED transmitted-but-ignored). Inert
	// is not Unsupported, so the bank — and with it the grid's clarifier
	// column — stays editable; a CHANGED clarifier is caught at send time
	// by codeplug.Diff's Inert gate, not by locking the cell.
	inertClarifier[spec.FieldClarifier] = spec.FieldSupport{Read: spec.Supported, Write: spec.Inert}

	tests := []struct {
		name   string
		fields map[spec.Field]spec.FieldSupport
		want   bool
	}{
		{"all Supported -> not read-only", allWritable, false},
		{"all Unverified -> not read-only (awaiting hardware trials, not locked)", allUnverified, false},
		{"all Unsupported -> read-only", allUnsupported, true},
		{"one field writable among Unsupported rest -> not read-only", mixedOneWritable, false},
		{"Inert clarifier among Supported rest -> not read-only (M5b real shape; clarifier column stays editable)", inertClarifier, false},
		{"absent bank entirely -> vacuously read-only (zero FieldSupport everywhere)", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := spec.Capabilities{}
			if tc.fields != nil {
				caps.Banks = []spec.Bank{{ID: "TEST", Fields: tc.fields}}
			}
			got := bankReadOnly(caps, "TEST")
			if got != tc.want {
				t.Errorf("bankReadOnly() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBankReadOnly_ExcludesUnexpressibleFields pins that
// FieldCTCSSTone/FieldScanSkip/FieldErase (never CAT-expressible on any
// bank, per core/driver/ft710/caps.go's bankFields) are NOT among the
// fields bankReadOnly tests: a bank whose only writable field is one of
// those must still read ReadOnly (those never move it out of that
// state), and their being Unsupported must never itself be read as "this
// bank is protocol-read-only" when a core field IS writable.
func TestBankReadOnly_ExcludesUnexpressibleFields(t *testing.T) {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	fields := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  {}, // Unsupported
		spec.FieldMode:       {},
		spec.FieldClarifier:  {},
		spec.FieldCTCSSState: {},
		spec.FieldShift:      {},
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},
		// The three fields bankReadOnly must NOT consult:
		spec.FieldCTCSSTone: rw,
		spec.FieldScanSkip:  rw,
		spec.FieldErase:     rw,
	}
	caps := spec.Capabilities{Banks: []spec.Bank{{ID: "TEST", Fields: fields}}}
	if !bankReadOnly(caps, "TEST") {
		t.Error("bankReadOnly() = false, want true (only unexpressible fields are writable; every core field is Unsupported)")
	}
}

// TestGetUISpec_Disconnected_StaticBaseline pins the offline/no-session
// shape: Live false, only MEM/PMS banks (the static baseline never
// carries 60M/EMG), both editable, slot lists exactly the static bank
// definitions, vocab/tone/limit fields straight from the static caps.
func TestGetUISpec_Disconnected_StaticBaseline(t *testing.T) {
	a, _ := newTestApp(t)

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}

	staticCaps, err := wiring.StaticCapabilities(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.DefaultModel, err)
	}

	if len(got.Banks) != 2 {
		t.Fatalf("len(Banks) = %d, want 2 (MEM, PMS only — static baseline has no 60M/EMG); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	mem := findBank(t, got.Banks, "MEM")
	if mem.ReadOnly {
		t.Error("MEM.ReadOnly = true while disconnected (static caps), want false — Unverified must not lock the grid")
	}
	pms := findBank(t, got.Banks, "PMS")
	if pms.ReadOnly {
		t.Error("PMS.ReadOnly = true while disconnected (static caps), want false — Unverified must not lock the grid")
	}

	memBank, _ := staticCaps.Bank(spec.BankMemory)
	if len(mem.Slots) != len(memBank.Slots) {
		t.Fatalf("len(MEM.Slots) = %d, want %d", len(mem.Slots), len(memBank.Slots))
	}
	memSlots := slotSet(mem.Slots)
	if disp, ok := memSlots["001"]; !ok || disp != "M-01" {
		t.Errorf("MEM.Slots[\"001\"].Display = %q, ok=%v, want \"M-01\"", disp, ok)
	}
	if disp, ok := memSlots["099"]; !ok || disp != "M-99" {
		t.Errorf("MEM.Slots[\"099\"].Display = %q, ok=%v, want \"M-99\"", disp, ok)
	}

	pmsBank, _ := staticCaps.Bank(spec.BankPMS)
	if len(pms.Slots) != len(pmsBank.Slots) {
		t.Fatalf("len(PMS.Slots) = %d, want %d", len(pms.Slots), len(pmsBank.Slots))
	}
	pmsSlots := slotSet(pms.Slots)
	if disp, ok := pmsSlots["P1L"]; !ok || disp != "P1L" {
		t.Errorf("PMS.Slots[\"P1L\"].Display = %q, ok=%v, want \"P1L\" (unchanged — not the M-/5- pattern)", disp, ok)
	}

	if len(got.ShiftOptions) != 3 || got.ShiftOptions[0] != "SIMPLEX" || got.ShiftOptions[1] != "PLUS" || got.ShiftOptions[2] != "MINUS" {
		t.Errorf("ShiftOptions = %v, want [SIMPLEX PLUS MINUS]", got.ShiftOptions)
	}
	if len(got.CTCSSStateOptions) != 3 || got.CTCSSStateOptions[0] != "OFF" || got.CTCSSStateOptions[1] != "ENC-DEC" || got.CTCSSStateOptions[2] != "ENC" {
		t.Errorf("CTCSSStateOptions = %v, want [OFF ENC-DEC ENC]", got.CTCSSStateOptions)
	}
	if len(got.Modes) != len(staticCaps.Modes) {
		t.Errorf("len(Modes) = %d, want %d", len(got.Modes), len(staticCaps.Modes))
	}
	if got.TagMaxBytes != staticCaps.TagLen {
		t.Errorf("TagMaxBytes = %d, want %d", got.TagMaxBytes, staticCaps.TagLen)
	}
	if got.ClarMaxHz != staticCaps.ClarMaxHz {
		t.Errorf("ClarMaxHz = %d, want %d", got.ClarMaxHz, staticCaps.ClarMaxHz)
	}
	if got.ClarStepHz != staticCaps.ClarStepHz {
		t.Errorf("ClarStepHz = %d, want %d", got.ClarStepHz, staticCaps.ClarStepHz)
	}
	if len(got.Tones) != len(staticCaps.CTCSSTones) {
		t.Fatalf("len(Tones) = %d, want %d", len(got.Tones), len(staticCaps.CTCSSTones))
	}
	if got.Tones[0].Decihertz != 670 || got.Tones[0].Display != "67.0 Hz" {
		t.Errorf("Tones[0] = %+v, want {670 \"67.0 Hz\"}", got.Tones[0])
	}
}

// TestGetUISpec_ConnectedSimulated pins the connected/live shape against
// a real simulated session (openTestSimSession) over fakeradio with
// ImageUS (so EMG is present alongside the full 15-channel 60m set):
// Live true, four banks, MEM/PMS editable, 60M/EMG read-only, and their
// slot counts matching the discovered inventory.
func TestGetUISpec_ConnectedSimulated(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithFactoryImage(fakeradio.ImageUS))
	connectDirect(t, a, sess, nil)

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false while connected, want true")
	}
	if len(got.Banks) != 4 {
		t.Fatalf("len(Banks) = %d, want 4 (MEM, PMS, 60M, EMG); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	mem := findBank(t, got.Banks, "MEM")
	if mem.ReadOnly {
		t.Error("MEM.ReadOnly = true while connected (Simulated), want false")
	}
	pms := findBank(t, got.Banks, "PMS")
	if pms.ReadOnly {
		t.Error("PMS.ReadOnly = true while connected (Simulated), want false")
	}
	sixty := findBank(t, got.Banks, "60M")
	if !sixty.ReadOnly {
		t.Error("60M.ReadOnly = false while connected, want true (discovered banks are always write-Unsupported)")
	}
	if len(sixty.Slots) != 15 {
		t.Errorf("len(60M.Slots) = %d, want 15 (ImageUS)", len(sixty.Slots))
	}
	// Label pins shared with the offline-synthesis test
	// (TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks): both
	// sides assert the same literal, so if either the driver's discovered
	// -bank labels or app/'s synthesised ones drift, a test fails.
	if sixty.Label != "60 m channels" {
		t.Errorf("60M.Label = %q, want %q", sixty.Label, "60 m channels")
	}
	emg := findBank(t, got.Banks, "EMG")
	if !emg.ReadOnly {
		t.Error("EMG.ReadOnly = false while connected, want true")
	}
	if emg.Label != "Emergency (EMG)" {
		t.Errorf("EMG.Label = %q, want %q", emg.Label, "Emergency (EMG)")
	}
	if len(emg.Slots) != 1 {
		t.Errorf("len(EMG.Slots) = %d, want 1", len(emg.Slots))
	}
	emgSlots := slotSet(emg.Slots)
	if disp, ok := emgSlots["EMG"]; !ok || disp != "EMG" {
		t.Errorf("EMG.Slots[\"EMG\"].Display = %q, ok=%v, want \"EMG\"", disp, ok)
	}
}

// TestGetUISpec_SlotClassification_OfflineWorkingCopy pins the "offline
// with a working copy" branch: slots come from the WORKING COPY —
// MEM/PMS filtered to membership in the static baseline's bank
// definitions (not the raw static list), and 60m/EMG slots grouped under
// SYNTHESISED read-only banks (controller adjudication: loaded data must
// never be invisible in the UI, and the static baseline carries no
// 60M/EMG bank to group them under).
func TestGetUISpec_SlotClassification_OfflineWorkingCopy(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{
			{Slot: "001"},
			{Slot: "050"},
			{Slot: "P1L"},
			{Slot: "501"}, // no static bank -> synthesised 60M bank
		},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}
	if len(got.Banks) != 3 {
		t.Fatalf("len(Banks) = %d, want 3 (MEM, PMS + synthesised 60M); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	mem := findBank(t, got.Banks, "MEM")
	memSlots := slotSet(mem.Slots)
	if len(memSlots) != 2 {
		t.Fatalf("MEM.Slots = %v, want exactly {001, 050}", memSlots)
	}
	if memSlots["001"] != "M-01" || memSlots["050"] != "M-50" {
		t.Errorf("MEM.Slots = %v, want {001:M-01 050:M-50}", memSlots)
	}

	pms := findBank(t, got.Banks, "PMS")
	pmsSlots := slotSet(pms.Slots)
	if len(pmsSlots) != 1 || pmsSlots["P1L"] != "P1L" {
		t.Errorf("PMS.Slots = %v, want exactly {P1L:P1L}", pmsSlots)
	}

	sixty := findBank(t, got.Banks, "60M")
	if !sixty.ReadOnly {
		t.Error("synthesised 60M.ReadOnly = false, want true")
	}
	sixtySlots := slotSet(sixty.Slots)
	if len(sixtySlots) != 1 || sixtySlots["501"] != "5-01" {
		t.Errorf("60M.Slots = %v, want exactly {501:5-01}", sixtySlots)
	}
}

// TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks pins the
// controller-adjudicated offline synthesis in full: a working copy
// holding 60m AND EMG channels (e.g. loaded from an earlier read of a
// US-region radio) gets synthesised read-only 60M/EMG BankViews with the
// SAME labels a live session's discovered banks carry, correct Display
// forms, and — the invariant — every working-copy slot appearing in
// exactly one BankView (no orphans, no duplicates, nothing invented).
func TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks(t *testing.T) {
	a, _ := newTestApp(t)
	workingSlots := []string{"001", "050", "P1L", "P9U", "501", "502", "515", "EMG"}
	channels := make([]codeplug.Channel, 0, len(workingSlots))
	for _, s := range workingSlots {
		channels = append(channels, codeplug.Channel{Slot: s})
	}
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: channels,
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}
	if len(got.Banks) != 4 {
		t.Fatalf("len(Banks) = %d, want 4 (MEM, PMS + synthesised 60M, EMG); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	sixty := findBank(t, got.Banks, "60M")
	if !sixty.ReadOnly {
		t.Error("synthesised 60M.ReadOnly = false, want true")
	}
	if sixty.Label != "60 m channels" {
		t.Errorf("synthesised 60M.Label = %q, want %q (must match the live session's discovered-bank label)", sixty.Label, "60 m channels")
	}
	sixtySlots := slotSet(sixty.Slots)
	if len(sixtySlots) != 3 || sixtySlots["501"] != "5-01" || sixtySlots["502"] != "5-02" || sixtySlots["515"] != "5-15" {
		t.Errorf("60M.Slots = %v, want exactly {501:5-01 502:5-02 515:5-15}", sixtySlots)
	}

	emg := findBank(t, got.Banks, "EMG")
	if !emg.ReadOnly {
		t.Error("synthesised EMG.ReadOnly = false, want true")
	}
	if emg.Label != "Emergency (EMG)" {
		t.Errorf("synthesised EMG.Label = %q, want %q (must match the live session's discovered-bank label)", emg.Label, "Emergency (EMG)")
	}
	emgSlots := slotSet(emg.Slots)
	if len(emgSlots) != 1 || emgSlots["EMG"] != "EMG" {
		t.Errorf("EMG.Slots = %v, want exactly {EMG:EMG}", emgSlots)
	}

	// The invariant: the union of every BankView's slots is EXACTLY the
	// working copy's slot set — each working-copy slot in exactly one
	// bank, no orphans, no bank slot the working copy does not hold.
	seen := make(map[string]int)
	total := 0
	for _, b := range got.Banks {
		for _, s := range b.Slots {
			seen[s.Slot]++
			total++
		}
	}
	if total != len(workingSlots) {
		t.Errorf("total slots across all BankViews = %d, want %d (the working copy's slot count)", total, len(workingSlots))
	}
	for _, s := range workingSlots {
		if seen[s] != 1 {
			t.Errorf("working-copy slot %q appears in %d BankViews, want exactly 1", s, seen[s])
		}
	}
}

// TestGetUISpec_OfflineWorkingCopy_PreservesDuplicateEMG pins the offline
// synthesis's duplicate-input behaviour (M9a Codex-review finding 1): a
// working copy holding the EMG slot more than once — reachable via
// LoadFile, which validates only AFTER loading a semantically invalid file
// — must have every occurrence rendered, never collapsed to one, so loaded
// rows are never silently dropped from the grid. This matches the pre-M9a
// synthesis, and the single physical EMG slot a live session probes can
// never produce such a duplicate.
func TestGetUISpec_OfflineWorkingCopy_PreservesDuplicateEMG(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{{Slot: "EMG"}, {Slot: "EMG"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}
	emg := findBank(t, got.Banks, "EMG")
	if len(emg.Slots) != 2 {
		t.Fatalf("EMG.Slots = %v, want 2 entries (both occurrences preserved, not collapsed)", emg.Slots)
	}
	for i, sv := range emg.Slots {
		if sv.Slot != "EMG" || sv.Display != "EMG" {
			t.Errorf("EMG.Slots[%d] = %+v, want {Slot:EMG Display:EMG}", i, sv)
		}
	}
}

// TestGetUISpec_SlotClassification_LivePrefersCapsOverWorkingCopy pins
// that, when connected, the bank's caps slot list is authoritative even
// if the working copy (e.g. mid-edit, or stale) does not exactly match
// it — unlike the offline case, live does not filter by/restrict to the
// working copy's own channels.
func TestGetUISpec_SlotClassification_LivePrefersCapsOverWorkingCopy(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithFactoryImage(fakeradio.ImageUS))
	connectDirect(t, a, sess, nil)

	// A working copy missing almost everything MEM/PMS caps would list.
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	mem := findBank(t, got.Banks, "MEM")
	if len(mem.Slots) != 99 {
		t.Errorf("len(MEM.Slots) = %d, want 99 (caps authoritative while live, ignoring the sparse working copy)", len(mem.Slots))
	}
}

// TestGetUISpec_NoWorkingCopy_NoConnection pins the third branch
// explicitly (offline, nothing loaded at all): banks come back with
// their static slot lists as-is (already covered in the disconnected
// baseline test above, restated narrowly here as its own case per the
// task's three-branch requirement).
func TestGetUISpec_NoWorkingCopy_NoConnection(t *testing.T) {
	a, _ := newTestApp(t)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	mem := findBank(t, got.Banks, "MEM")
	if len(mem.Slots) != 99 {
		t.Errorf("len(MEM.Slots) = %d, want 99", len(mem.Slots))
	}
}

// TestGetUISpec_ToneFormatting is a table-driven check of the
// Decihertz->Display mapping against known CTCSS chart values.
func TestGetUISpec_ToneFormatting(t *testing.T) {
	a, _ := newTestApp(t)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	byDeci := make(map[int]string, len(got.Tones))
	for _, tv := range got.Tones {
		byDeci[tv.Decihertz] = tv.Display
	}

	tests := []struct {
		deci int
		want string
	}{
		{670, "67.0 Hz"},
		{885, "88.5 Hz"},
		{1000, "100.0 Hz"},
		{2541, "254.1 Hz"},
	}
	for _, tc := range tests {
		got, ok := byDeci[tc.deci]
		if !ok {
			t.Errorf("no tone with Decihertz=%d in Tones", tc.deci)
			continue
		}
		if got != tc.want {
			t.Errorf("tone %d Display = %q, want %q", tc.deci, got, tc.want)
		}
	}
}

// TestGetUISpec_VocabMatchesValidate cross-checks that every literal
// GetUISpec exposes for Mode/Shift/CTCSS-state is one codeplug.Validate
// itself accepts for that field — i.e. the grid's option lists can never
// offer a value Validate would then reject.
func TestGetUISpec_VocabMatchesValidate(t *testing.T) {
	a, _ := newTestApp(t)
	uiSpec, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	caps, err := wiring.StaticCapabilities(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.DefaultModel, err)
	}

	baseData := func() codeplug.ChannelData {
		return codeplug.ChannelData{
			FreqHz:     7_000_000,
			Mode:       "USB",
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}
	}

	assertNoFieldIssue := func(t *testing.T, cp *codeplug.Codeplug, field spec.Field, value string) {
		t.Helper()
		issues := codeplug.Validate(cp, caps)
		for _, is := range issues {
			if is.Slot == "001" && is.Field == field {
				t.Errorf("Validate flagged %s=%q on slot 001: %s", field, value, is.Msg)
			}
		}
	}

	for _, mode := range uiSpec.Modes {
		d := baseData()
		d.Mode = mode
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001", Data: &d}}}
		assertNoFieldIssue(t, cp, "mode", mode)
	}
	for _, shift := range uiSpec.ShiftOptions {
		d := baseData()
		d.Shift = shift
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001", Data: &d}}}
		assertNoFieldIssue(t, cp, "shift", shift)
	}
	for _, ctcss := range uiSpec.CTCSSStateOptions {
		d := baseData()
		d.CTCSS = ctcss
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001", Data: &d}}}
		assertNoFieldIssue(t, cp, "ctcss_state", ctcss)
	}
}

// TestGetUISpec_ServesProse pins the served sentences server-side (task
// 41, M9a-5): every radiotext-sourced UISpecView field is byte-equal to
// internal/radiotext.For(wiring.DefaultModel)'s own value — not merely
// non-empty — so a future edit to either side (this call site or
// radiotext's own FT-710 entry) that let them drift would fail here
// first. Checked both offline and connected: since M9c-5 (E4) the prose
// is keyed off currentModel's resolved model rather than
// wiring.DefaultModel, and both branches resolve to the FT-710 here — the
// simulated session's own model offline-or-not — so both must still serve
// exactly these strings.
func TestGetUISpec_ServesProse(t *testing.T) {
	want, ok := radiotext.For(wiring.DefaultModel)
	if !ok {
		t.Fatalf("radiotext.For(%q): ok = false, want true", wiring.DefaultModel)
	}

	assertProse := func(t *testing.T, got UISpecView) {
		t.Helper()
		if got.ToneScanSkipNote != want.ToneScanSkipNote {
			t.Errorf("ToneScanSkipNote = %q, want %q", got.ToneScanSkipNote, want.ToneScanSkipNote)
		}
		if got.ToneScanSkipVerification != want.ToneScanSkipVerification {
			t.Errorf("ToneScanSkipVerification = %q, want %q", got.ToneScanSkipVerification, want.ToneScanSkipVerification)
		}
		if got.EraseDialogNote != want.EraseDialogNote {
			t.Errorf("EraseDialogNote = %q, want %q", got.EraseDialogNote, want.EraseDialogNote)
		}
		if got.PreservationTooltips.Tone != want.PreservationTooltips.Tone {
			t.Errorf("PreservationTooltips.Tone = %q, want %q", got.PreservationTooltips.Tone, want.PreservationTooltips.Tone)
		}
		if got.PreservationTooltips.ScanSkip != want.PreservationTooltips.ScanSkip {
			t.Errorf("PreservationTooltips.ScanSkip = %q, want %q", got.PreservationTooltips.ScanSkip, want.PreservationTooltips.ScanSkip)
		}
		if got.FirmwarePlaceholder != want.FirmwarePlaceholder {
			t.Errorf("FirmwarePlaceholder = %q, want %q", got.FirmwarePlaceholder, want.FirmwarePlaceholder)
		}
	}

	t.Run("offline", func(t *testing.T) {
		a, _ := newTestApp(t)
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		assertProse(t, got)
	})

	t.Run("connected", func(t *testing.T) {
		a, _ := newTestApp(t)
		sess := openTestSimSession(t)
		connectDirect(t, a, sess, nil)
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		assertProse(t, got)
	})
}

// TestGetUISpec_ProseFollowsResolvedModel is the prose cluster's
// threading pin (M9c-5 E4): the served sentences follow the model
// currentModel resolves, and radiotext.For's ok is honoured — a model
// with no radiotext entry gets SILENCE (every prose field empty), never
// the FT-710's own wording attributed to a different radio.
//
// testModel is admitted through the capsForModel seam only, so
// internal/radiotext genuinely has no entry for it; the empty fields
// below are therefore the honest served value for a model whose prose has
// not been written yet, and they are observably different from the
// FT-710's (pinned non-empty first, so this cannot pass vacuously).
func TestGetUISpec_ProseFollowsResolvedModel(t *testing.T) {
	ft710Text, ok := radiotext.For(wiring.DefaultModel)
	if !ok || ft710Text.ToneScanSkipNote == "" || ft710Text.EraseDialogNote == "" {
		t.Fatalf("test setup: radiotext.For(%q) ok=%v with empty prose — the contrast below would be vacuous", wiring.DefaultModel, ok)
	}
	recogniseTestModel(t)

	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: testModel},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	for _, f := range []struct {
		name string
		got  string
	}{
		{"ToneScanSkipNote", got.ToneScanSkipNote},
		{"ToneScanSkipVerification", got.ToneScanSkipVerification},
		{"EraseDialogNote", got.EraseDialogNote},
		{"PreservationTooltips.Tone", got.PreservationTooltips.Tone},
		{"PreservationTooltips.ScanSkip", got.PreservationTooltips.ScanSkip},
		{"FirmwarePlaceholder", got.FirmwarePlaceholder},
	} {
		if f.got != "" {
			t.Errorf("%s = %q for a model radiotext has no entry for, want \"\" (silence, never another radio's wording)", f.name, f.got)
		}
	}
}

// TestGetUISpec_UnrecognisedWorkingModelStillSynthesisesBanks pins the
// reason the bank-synthesis site consumes the RESOLVER rather than the
// working copy's raw Radio.Model (M9c-5 E4's recorded design note): a
// legacy or hand-edited file naming a model no driver is registered for
// must still show its 60m/EMG channels. Handed the raw name,
// wiring.SynthesiseDiscoveredBanks would report ok == false and those
// channels would vanish from the grid — loaded but invisible. The
// resolver falls back to wiring.DefaultModel, so they stay.
func TestGetUISpec_UnrecognisedWorkingModelStillSynthesisesBanks(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "NoSuchRadioModel"},
		Channels: []codeplug.Channel{
			{Slot: "001"}, {Slot: "P1L"}, {Slot: "501"}, {Slot: "EMG"},
		},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	sixty := findBank(t, got.Banks, "60M")
	if s := slotSet(sixty.Slots); len(s) != 1 || s["501"] != "5-01" {
		t.Errorf("synthesised 60M.Slots = %v, want exactly {501:5-01}", s)
	}
	emg := findBank(t, got.Banks, "EMG")
	if s := slotSet(emg.Slots); len(s) != 1 || s["EMG"] != "EMG" {
		t.Errorf("synthesised EMG.Slots = %v, want exactly {EMG:EMG}", s)
	}
}
