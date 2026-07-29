// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// findDiffEntry returns the DiffEntry for slot, failing the test if absent.
func findDiffEntry(t *testing.T, result DiffResult, slot string) DiffEntry {
	t.Helper()
	for _, e := range result.Entries {
		if e.Slot == slot {
			return e
		}
	}
	t.Fatalf("no DiffEntry for slot %q in %+v", slot, result.Entries)
	return DiffEntry{}
}

// TestDiffKinds covers Added/Modified/Erased/Unchanged, per-kind
// blocking (erase Unverified on MEM, bank-read-only on 60M), and the
// aggregate counts, all from one multi-change scenario:
//   - "001" untouched                              -> Unchanged, not blocked
//   - "002" erased (was populated, now empty)       -> Erased, blocked (erase Unverified)
//   - "003" populated (was empty)                   -> Added, not blocked (MEM is writable)
//   - "P1L"/"P1U" untouched                         -> Unchanged, not blocked
//   - "501" modified (mode changed)                 -> Modified, blocked (60M read-only)
func TestDiffKinds(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()

	for i := range file.Channels {
		switch file.Channels[i].Slot {
		case "002":
			file.Channels[i].Data = nil
		case "003":
			file.Channels[i].Data = &ChannelData{
				FreqHz: 14100000, Mode: "USB", CTCSS: "OFF",
				CTCSSTone: ToneField{State: Unknown}, Shift: "SIMPLEX",
				TagDisplay: BoolField{State: Known, Value: false},
				ScanSkip:   BoolField{State: Known},
			}
		case "501":
			file.Channels[i].Data.Mode = "LSB"
		}
	}

	caps := testCapabilities()
	result, err := Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}

	if len(result.Entries) != len(baseline.Channels) {
		t.Fatalf("len(Entries) = %d, want %d (one per baseline slot)", len(result.Entries), len(baseline.Channels))
	}

	cases := []struct {
		slot        string
		wantKind    DiffKind
		wantBank    spec.BankID
		wantBlocked bool
		wantReason  string
	}{
		{"001", DiffUnchanged, spec.BankMemory, false, ""},
		{"002", DiffErased, spec.BankMemory, true, "erase not supported on this radio"},
		{"003", DiffAdded, spec.BankMemory, false, ""},
		{"P1L", DiffUnchanged, spec.BankPMS, false, ""},
		{"P1U", DiffUnchanged, spec.BankPMS, false, ""},
		{"501", DiffModified, spec.Bank60m, true, "bank 60M is read-only"},
	}
	for _, tc := range cases {
		t.Run(tc.slot, func(t *testing.T) {
			e := findDiffEntry(t, result, tc.slot)
			if e.Kind != tc.wantKind {
				t.Errorf("Kind = %v, want %v", e.Kind, tc.wantKind)
			}
			if e.Bank != tc.wantBank {
				t.Errorf("Bank = %v, want %v", e.Bank, tc.wantBank)
			}
			if e.Blocked != tc.wantBlocked {
				t.Errorf("Blocked = %v, want %v", e.Blocked, tc.wantBlocked)
			}
			if e.BlockReason != tc.wantReason {
				t.Errorf("BlockReason = %q, want %q", e.BlockReason, tc.wantReason)
			}
		})
	}

	if result.Added != 1 {
		t.Errorf("Added = %d, want 1", result.Added)
	}
	if result.Modified != 1 {
		t.Errorf("Modified = %d, want 1", result.Modified)
	}
	if result.Erased != 1 {
		t.Errorf("Erased = %d, want 1", result.Erased)
	}
	if result.Unchanged != 3 {
		t.Errorf("Unchanged = %d, want 3", result.Unchanged)
	}
	// Blocked entries still count under their own Kind (checked above)
	// AND in Blocked: "002" (erase) + "501" (bank read-only) = 2.
	if result.Blocked != 2 {
		t.Errorf("Blocked = %d, want 2", result.Blocked)
	}
}

// TestDiffFieldStateOnlyChangeIsModified covers the brief's specific
// example: a tone going Known -> Unknown, with every other field
// identical, must be reported as Modified — field STATE is part of
// equality, not just Value.
func TestDiffFieldStateOnlyChangeIsModified(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()

	for i := range baseline.Channels {
		if baseline.Channels[i].Slot == "002" {
			baseline.Channels[i].Data.CTCSS = "ENC"
			baseline.Channels[i].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(670)}
		}
	}
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.CTCSS = "ENC"
			file.Channels[i].Data.CTCSSTone = ToneField{State: Unknown}
		}
	}

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Errorf("Kind = %v, want %v (tone State Known->Unknown is a modification)", e.Kind, DiffModified)
	}
}

// TestDiffInventoryMismatchIsError covers the file/baseline slot-inventory
// mismatch: it must be an error, not a panic or a silently-wrong diff, and
// the message must explain that a re-read is needed.
func TestDiffInventoryMismatchIsError(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	file.Channels = append(file.Channels, Channel{Slot: "999"})

	result, err := Diff(baseline, file, testCapabilities())
	if err == nil {
		t.Fatal("Diff() error = nil, want an error for mismatched slot inventories")
	}
	if !strings.Contains(err.Error(), "re-read") {
		t.Errorf("Diff() error = %q, want it to mention re-reading the radio", err.Error())
	}
	if !reflect.DeepEqual(result, DiffResult{}) {
		t.Errorf("Diff() result = %+v, want zero value on error", result)
	}
}

// TestDiffBaselineDigest checks DiffResult.BaselineDigest equals
// Digest(baseline.Channels) — a confirmation must be bound to this value.
func TestDiffBaselineDigest(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	want := Digest(baseline.Channels)
	if result.BaselineDigest != want {
		t.Errorf("BaselineDigest = %q, want %q (Digest(baseline.Channels))", result.BaselineDigest, want)
	}
}

// TestDiffCandidateDigest checks DiffResult.CandidateDigest equals
// Digest(file.Channels) — a sender must recompute and compare BOTH
// digests immediately before transmission (see the doc comment).
func TestDiffCandidateDigest(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		// testBaselineCodeplug's "002" is already Mode "LSB"; use "USB"
		// so this mutation is an actual change, not a no-op.
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.Mode = "USB"
		}
	}

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	want := Digest(file.Channels)
	if result.CandidateDigest != want {
		t.Errorf("CandidateDigest = %q, want %q (Digest(file.Channels))", result.CandidateDigest, want)
	}
	// Sanity: the baseline and candidate digests must differ here, since
	// file actually differs from baseline in this scenario.
	if result.CandidateDigest == result.BaselineDigest {
		t.Errorf("CandidateDigest == BaselineDigest (%q), want different digests for differing images", result.CandidateDigest)
	}
}

// TestDiffDefensiveCopies checks that DiffEntry.Before/After are copies:
// mutating them through the returned DiffResult must not mutate the
// Codeplug values passed in to Diff.
func TestDiffDefensiveCopies(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.Mode = "LSB"
		}
	}

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Before == nil || e.After == nil {
		t.Fatalf("Before/After = %v/%v, want both non-nil for a Modified entry", e.Before, e.After)
	}

	e.Before.Tag = "TAMPERED"
	e.After.Tag = "TAMPERED"

	for _, ch := range baseline.Channels {
		if ch.Slot == "002" && ch.Data.Tag == "TAMPERED" {
			t.Error("mutating DiffEntry.Before mutated the baseline Codeplug's own ChannelData")
		}
	}
	for _, ch := range file.Channels {
		if ch.Slot == "002" && ch.Data.Tag == "TAMPERED" {
			t.Error("mutating DiffEntry.After mutated the file Codeplug's own ChannelData")
		}
	}
}

// toneOnlyChangeCodeplugs builds a baseline/file pair whose ONLY
// difference, anywhere, is slot "002"'s CTCSS tone VALUE (CTCSS state
// stays "ENC" on both sides) — used by the per-field gate tests below to
// isolate FieldCTCSSTone as the sole changed field.
func toneOnlyChangeCodeplugs() (baseline, file *Codeplug) {
	baseline = testBaselineCodeplug()
	file = testBaselineCodeplug()
	for i := range baseline.Channels {
		if baseline.Channels[i].Slot == "002" {
			baseline.Channels[i].Data.CTCSS = "ENC"
			baseline.Channels[i].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(670)}
		}
	}
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.CTCSS = "ENC"
			file.Channels[i].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(693)}
		}
	}
	return baseline, file
}

// TestDiffPerFieldGate_ToneOnlyChangeUnverifiedBlocks is the brief's
// named regression: a tone-only change in an otherwise fully-writable
// bank (MEM) must be Blocked when FieldCTCSSTone.Write is Unverified —
// before Fix 4, only bank-level FieldFrequency and erasures gated
// blocking, so this would have wrongly reported Modified/unblocked.
func TestDiffPerFieldGate_ToneOnlyChangeUnverifiedBlocks(t *testing.T) {
	baseline, file := toneOnlyChangeCodeplugs()

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffModified)
	}
	if !e.Blocked {
		t.Error("Blocked = false, want true (ctcss_tone write is Unverified)")
	}
	if !strings.Contains(e.BlockReason, "ctcss_tone") {
		t.Errorf("BlockReason = %q, want it to name ctcss_tone", e.BlockReason)
	}
}

// TestDiffPerFieldGate_ToneOnlyChangeSupportedNotBlocked is the same
// tone-only change, but against a Capabilities where FieldCTCSSTone.Write
// is Supported: it must NOT be blocked.
func TestDiffPerFieldGate_ToneOnlyChangeSupportedNotBlocked(t *testing.T) {
	baseline, file := toneOnlyChangeCodeplugs()

	caps := testCapabilities()
	for i, b := range caps.Banks {
		if b.ID == spec.BankMemory {
			caps.Banks[i].Fields[spec.FieldCTCSSTone] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
		}
	}

	result, err := Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Blocked {
		t.Errorf("Blocked = true (reason %q), want false (ctcss_tone write is now Supported)", e.BlockReason)
	}
}

// TestDiffPerFieldGate_AddedChannelKnownToneBlocks covers an Added
// channel (previously-empty slot) whose CTCSSTone is Known: that is a
// requested write, so it must be gated the same as a Modified entry's
// tone change.
func TestDiffPerFieldGate_AddedChannelKnownToneBlocks(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "003" {
			file.Channels[i].Data = &ChannelData{
				FreqHz:     14100000,
				Mode:       "USB",
				CTCSS:      "ENC",
				CTCSSTone:  ToneField{State: Known, Value: spec.Tone(670)},
				Shift:      "SIMPLEX",
				TagDisplay: BoolField{State: Known, Value: false},
				ScanSkip:   BoolField{State: Known},
			}
		}
	}

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "003")
	if e.Kind != DiffAdded {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffAdded)
	}
	if !e.Blocked {
		t.Error("Blocked = false, want true (Known ctcss_tone on an Added channel is a requested write, and ctcss_tone write is Unverified)")
	}
	if !strings.Contains(e.BlockReason, "ctcss_tone") {
		t.Errorf("BlockReason = %q, want it to name ctcss_tone", e.BlockReason)
	}
}

// TestDiffPerFieldGate_AddedChannelUnknownToneNotBlockedByTone covers the
// brief's specific carve-out: an Added channel's tone field in state
// Unknown is NOT a requested write (FieldState's write rule: only Known
// is ever sent), so it must not contribute to blocking even though
// ctcss_tone write is Unverified in this bank.
func TestDiffPerFieldGate_AddedChannelUnknownToneNotBlockedByTone(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "003" {
			file.Channels[i].Data = &ChannelData{
				FreqHz:     14100000,
				Mode:       "USB",
				CTCSS:      "OFF",
				CTCSSTone:  ToneField{State: Unknown},
				Shift:      "SIMPLEX",
				TagDisplay: BoolField{State: Known, Value: false},
				ScanSkip:   BoolField{State: Known},
			}
		}
	}

	result, err := Diff(baseline, file, testCapabilities())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "003")
	if e.Blocked {
		t.Errorf("Blocked = true (reason %q), want false (Unknown tone introduces no write request)", e.BlockReason)
	}
}

// TestDiffPerFieldGate_ModifiedUnchangedFieldStillGates (M3 Codex-review
// fix wave, Fix 4): before this fix, a DiffModified entry was gated only
// against changedFields — the fields whose VALUE actually differs between
// baseline and file. But MW+MT rewrite every wire-transmitted field on a
// write, whether or not its value changed, so an unwritable field that
// happens to be unchanged in THIS particular edit would still be clobbered
// by the write and must still block. Here "002"'s ONLY changed field is
// Mode; FieldTag is made unwritable but is NOT itself edited — the entry
// must still be Blocked, naming tag (and explicitly noting it as
// unchanged), while NOT naming mode (mode itself remains write-Supported).
func TestDiffPerFieldGate_ModifiedUnchangedFieldStillGates(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			// testBaselineCodeplug's "002" defaults to Mode "LSB"; this is
			// the only field this edit actually changes. Tag ("NET") is
			// left untouched on both sides.
			file.Channels[i].Data.Mode = "USB"
		}
	}

	caps := testCapabilities()
	for i, b := range caps.Banks {
		if b.ID == spec.BankMemory {
			caps.Banks[i].Fields[spec.FieldTag] = spec.FieldSupport{Read: spec.Supported, Write: spec.Unverified}
		}
	}

	result, err := Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffModified)
	}
	if !e.Blocked {
		t.Fatal("Blocked = false, want true (tag is unwritable, and MW/MT rewrite it regardless of whether THIS edit changed it)")
	}
	if !strings.Contains(e.BlockReason, "tag") {
		t.Errorf("BlockReason = %q, want it to name tag", e.BlockReason)
	}
	if !strings.Contains(e.BlockReason, "rewritten by MW even though unchanged") {
		t.Errorf("BlockReason = %q, want it to note tag is unchanged but still rewritten by the write", e.BlockReason)
	}
	if strings.Contains(e.BlockReason, "mode") {
		t.Errorf("BlockReason = %q, want it NOT to name mode (mode itself is write-Supported — only tag gates this entry)", e.BlockReason)
	}
}

// inertClarifierCaps is testCapabilities with MEM's FieldClarifier
// marked Write: Inert — the HW-CONFIRMED 2026-07-13 state of the
// FT-710's clarifier field class (value + RxClar + TxClar): transmitted
// on every MW write, silently ignored by the radio (read back zeros
// every time). Used by the Inert-gate tests below.
func inertClarifierCaps() spec.Capabilities {
	caps := testCapabilities()
	for i, b := range caps.Banks {
		if b.ID == spec.BankMemory {
			caps.Banks[i].Fields[spec.FieldClarifier] = spec.FieldSupport{Read: spec.Supported, Write: spec.Inert}
		}
	}
	return caps
}

// TestDiffInertGate_ChangedClarifierBlocks: an Inert field whose value
// DIFFERS Before->After blocks the entry — the radio provably ignores
// the transmitted value (HW-CONFIRMED 2026-07-13), so a changed
// clarifier is an intent that cannot be honoured (and, unblocked, would
// abort every send at the verify step instead: written value never reads
// back). The BlockReason must be the precise Inert wording, not the
// generic not-writable one.
func TestDiffInertGate_ChangedClarifierBlocks(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.ClarHz = 100
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffModified)
	}
	if !e.Blocked {
		t.Fatal("Blocked = false, want true (a changed clarifier value cannot be honoured — the radio ignores it)")
	}
	want := "clarifier changes are ignored by the radio and cannot be sent"
	if e.BlockReason != want {
		t.Errorf("BlockReason = %q, want %q", e.BlockReason, want)
	}
}

// TestDiffInertGate_ChangedClarFlagBlocks: the Inert clarifier field
// class covers the Rx/Tx clarifier FLAGS too, not just the offset value
// — all three are FieldClarifier, and the radio ignores all three
// (HW-CONFIRMED: flags accepted on MW, read back zeros).
func TestDiffInertGate_ChangedClarFlagBlocks(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.RxClar = true
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if !e.Blocked {
		t.Fatal("Blocked = false, want true (a changed RxClar flag is a clarifier-class change the radio ignores)")
	}
	if !strings.Contains(e.BlockReason, "clarifier") {
		t.Errorf("BlockReason = %q, want it to name clarifier", e.BlockReason)
	}
}

// TestDiffInertGate_UnchangedClarifierFlows is the Inert rule's other
// half — the half whose absence made the ORIGINAL (pre-amendment)
// "clarifier Write: Unsupported" remedy block every write in the
// repository: an Inert field whose value does NOT change must not block.
// The write transmits the baseline value, the radio ignores it, and the
// read-back verify still matches — nothing unhonourable was requested.
// Here the ONLY edit is the tag; the clarifier matches the baseline
// (zero on both sides).
func TestDiffInertGate_UnchangedClarifierFlows(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.Tag = "RENAMED"
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffModified)
	}
	if e.Blocked {
		t.Errorf("Blocked = true (reason %q), want false — an unchanged Inert clarifier transmits the baseline value, which the radio ignores; nothing unhonourable is requested", e.BlockReason)
	}
}

// TestDiffInertGate_UnchangedNonZeroClarifierFlows pins the unchanged-
// Inert rule's LOGIC in isolation: it compares Before against After — NOT
// After against zero — so a channel whose baseline value happens to be
// non-zero still flows when the candidate keeps that value unchanged.
// Clarified (opus review, Codex M5b fix wave, Fix 6): this scenario is
// PRODUCTION-UNREACHABLE today, not a real-radio case this test claims to
// reproduce — every channel M5b actually probed already had a zero
// stored clarifier (docs/hardware-notes.md §Clarifier: "every probed
// channel's STORED clarifier was already zero"), so a genuinely non-zero
// baseline clarifier has never been observed, and whether one could even
// arise (e.g. via the front panel) is unconfirmed. The test exists to
// pin Diff's own comparison logic defensively — Before-vs-After, never
// After-vs-zero — against the day that assumption turns out wrong, not
// to assert this input shape occurs today.
func TestDiffInertGate_UnchangedNonZeroClarifierFlows(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for _, cp := range []*Codeplug{baseline, file} {
		for i := range cp.Channels {
			if cp.Channels[i].Slot == "002" {
				cp.Channels[i].Data.ClarHz = 250
			}
		}
	}
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.Tag = "RENAMED"
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Blocked {
		t.Errorf("Blocked = true (reason %q), want false — the clarifier value is unchanged Before->After (250 on both sides)", e.BlockReason)
	}
}

// TestDiffInertGate_AddedZeroClarifierFlows: an Added entry has no
// baseline, so its Inert clarifier is compared against the zero value —
// a freshly-created channel reads back a zero clarifier (HW-CONFIRMED
// 2026-07-13: the live empty-slot MW create on slot 096 read back
// "+0000"/flags 0), so a zero-clarifier Added channel requests nothing
// the radio will not honour.
func TestDiffInertGate_AddedZeroClarifierFlows(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "003" {
			file.Channels[i].Data = &ChannelData{
				FreqHz: 14100000, Mode: "USB", CTCSS: "OFF",
				CTCSSTone: ToneField{State: Unknown}, Shift: "SIMPLEX",
				TagDisplay: BoolField{State: Known, Value: false},
				ScanSkip:   BoolField{State: Known},
			}
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "003")
	if e.Kind != DiffAdded {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffAdded)
	}
	if e.Blocked {
		t.Errorf("Blocked = true (reason %q), want false (zero clarifier on an Added channel matches what the radio will store)", e.BlockReason)
	}
}

// TestDiffInertGate_AddedNonZeroClarifierBlocks: an Added channel
// requesting a NON-zero clarifier is an unhonourable intent exactly like
// a Modified entry's changed value — the fresh channel will read back
// zeros regardless of what was transmitted.
func TestDiffInertGate_AddedNonZeroClarifierBlocks(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "003" {
			file.Channels[i].Data = &ChannelData{
				FreqHz: 14100000, Mode: "USB", CTCSS: "OFF",
				CTCSSTone: ToneField{State: Unknown}, Shift: "SIMPLEX",
				ClarHz:     -150,
				TagDisplay: BoolField{State: Known, Value: false},
				ScanSkip:   BoolField{State: Known},
			}
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "003")
	if !e.Blocked {
		t.Fatal("Blocked = false, want true (a non-zero clarifier on an Added channel cannot be honoured)")
	}
	want := "clarifier changes are ignored by the radio and cannot be sent"
	if e.BlockReason != want {
		t.Errorf("BlockReason = %q, want %q", e.BlockReason, want)
	}
}

// TestDiffInertGate_CombinesWithUnwritableReason: an entry with BOTH an
// unwritable-field request (a Known tone change; ctcss_tone Write is
// Unverified in testCapabilities) AND a changed Inert clarifier must
// name both problems — neither may mask the other.
func TestDiffInertGate_CombinesWithUnwritableReason(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range baseline.Channels {
		if baseline.Channels[i].Slot == "002" {
			baseline.Channels[i].Data.CTCSS = "ENC"
			baseline.Channels[i].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(670)}
		}
	}
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.CTCSS = "ENC"
			file.Channels[i].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(693)}
			file.Channels[i].Data.ClarHz = 100
		}
	}

	result, err := Diff(baseline, file, inertClarifierCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if !e.Blocked {
		t.Fatal("Blocked = false, want true")
	}
	if !strings.Contains(e.BlockReason, "ctcss_tone") {
		t.Errorf("BlockReason = %q, want it to name ctcss_tone (the unwritable request)", e.BlockReason)
	}
	if !strings.Contains(e.BlockReason, "clarifier changes are ignored by the radio") {
		t.Errorf("BlockReason = %q, want it to carry the Inert clarifier wording too", e.BlockReason)
	}
}

// frequencyInertCaps is testCapabilities with MEM's FieldFrequency marked
// Write: Inert — Codex M5b fix wave, Fix 4 (adjudicated MEDIUM): a
// HYPOTHETICAL adversarial case, not a hardware finding (frequency has
// never been observed Inert on any real FT-710 profile — every profile
// marks it Supported/Unverified/Unsupported). Used to exercise Diff's
// bank-level writability gate (this function's doc comment, point 1),
// which used FieldFrequency.CanWrite() as a proxy for "is this bank
// writable at all" — CanWrite() is false for Inert exactly as it is for
// Unsupported/Unverified, so if frequency ever DID become Inert, the
// bank gate would have wrongly Blocked every entry as "bank ... is
// read-only", even a tag-only edit with an unchanged frequency, before
// the generic per-field Inert gate ever got a chance to decide. See the
// Inert-gate tests below.
func frequencyInertCaps() spec.Capabilities {
	caps := testCapabilities()
	for i, b := range caps.Banks {
		if b.ID == spec.BankMemory {
			caps.Banks[i].Fields[spec.FieldFrequency] = spec.FieldSupport{Read: spec.Supported, Write: spec.Inert}
		}
	}
	return caps
}

// TestDiffInertGate_FrequencyInert_TagOnlyEditFlows is Fix 4's main case:
// with frequency Inert and UNCHANGED, a tag-only edit must flow through
// the bank-level gate (point 1) — an Inert field counts as transmissible
// there, exactly as it does for the generic per-field gate — leaving the
// generic changed-Inert gate (point 3) to correctly decide nothing
// blocks, since frequency itself never changed.
func TestDiffInertGate_FrequencyInert_TagOnlyEditFlows(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.Tag = "RENAMED"
		}
	}

	result, err := Diff(baseline, file, frequencyInertCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffModified)
	}
	if e.Blocked {
		t.Errorf("Blocked = true (reason %q), want false — an unchanged Inert frequency must not be treated as \"bank is read-only\"; the tag-only edit has nothing unhonourable to send", e.BlockReason)
	}
}

// TestDiffInertGate_FrequencyInert_ChangedFrequencyBlocks is Fix 4's
// other half: a CHANGED Inert frequency must still block — same as any
// other changed Inert field — via the generic Inert wording, once the
// bank gate correctly lets the entry through to it.
func TestDiffInertGate_FrequencyInert_ChangedFrequencyBlocks(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		// testBaselineCodeplug's "002" starts at 14300000 Hz — pick a
		// genuinely different value so this is a real, non-no-op change.
		if file.Channels[i].Slot == "002" {
			file.Channels[i].Data.FreqHz = 7_100_000
		}
	}

	result, err := Diff(baseline, file, frequencyInertCaps())
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	e := findDiffEntry(t, result, "002")
	if e.Kind != DiffModified {
		t.Fatalf("Kind = %v, want %v", e.Kind, DiffModified)
	}
	if !e.Blocked {
		t.Fatal("Blocked = false, want true (a changed Inert frequency cannot be honoured — the radio ignores it)")
	}
	want := "frequency changes are ignored by the radio and cannot be sent"
	if e.BlockReason != want {
		t.Errorf("BlockReason = %q, want %q", e.BlockReason, want)
	}
}

// TestDiffDeterministic runs Diff twice over the same inputs and requires
// identical results, including entry order.
func TestDiffDeterministic(t *testing.T) {
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	for i := range file.Channels {
		switch file.Channels[i].Slot {
		case "002":
			file.Channels[i].Data = nil
		case "003":
			file.Channels[i].Data = &ChannelData{
				FreqHz: 14100000, Mode: "USB", CTCSS: "OFF",
				CTCSSTone: ToneField{State: Unknown}, Shift: "SIMPLEX",
				TagDisplay: BoolField{State: Known, Value: false},
				ScanSkip:   BoolField{State: Known},
			}
		}
	}
	caps := testCapabilities()

	result1, err1 := Diff(baseline, file, caps)
	result2, err2 := Diff(baseline, file, caps)
	if err1 != nil || err2 != nil {
		t.Fatalf("Diff() errors = %v, %v, want nil", err1, err2)
	}
	if !reflect.DeepEqual(result1, result2) {
		t.Errorf("Diff() not deterministic:\nrun1: %+v\nrun2: %+v", result1, result2)
	}
	for i, e := range result1.Entries {
		if e.Slot != baseline.Channels[i].Slot {
			t.Errorf("Entries[%d].Slot = %q, want %q (baseline slot order)", i, e.Slot, baseline.Channels[i].Slot)
		}
	}
}
