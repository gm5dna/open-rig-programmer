// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// settingsWriteStubSession is a driver.Session double carrying both
// driver.SettingsReader and driver.SettingsWriter, with a caller-supplied
// descriptor (so a test can mint CanSetEX-characterised items directly, via
// SettingItem.Write) and caller-supplied ReadSetting/WriteSetting
// behaviour.
//
// A real *ft710.Session cannot exhibit a characterised write address at
// all from OUTSIDE core/driver/ft710: table2-write-observed.csv is still
// empty in production (spec §3), and that package exposes no way to
// inject a characterised write descriptor short of its own unexported
// withDialectForTest test seam (core/driver/ft710/settings_test.go) —
// this extends settings_test.go's stubSession/stubSettingsSession
// carve-out (see that file's doc comments) to cover the write side too.
type settingsWriteStubSession struct {
	caps         spec.Capabilities
	descriptor   driver.SettingsDescriptor
	readSetting  func(ctx context.Context, id string) (driver.SettingValue, error)
	writeSetting func(ctx context.Context, id, value string) (driver.SettingWriteResult, error)
}

func (s *settingsWriteStubSession) Identity() driver.Identity       { return driver.Identity{} }
func (s *settingsWriteStubSession) Capabilities() spec.Capabilities { return s.caps }

func (s *settingsWriteStubSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	return codeplug.Channel{}, errors.New("settingsWriteStubSession: ReadChannel not implemented")
}

func (s *settingsWriteStubSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{}, errors.New("settingsWriteStubSession: WriteChannel not implemented")
}

func (s *settingsWriteStubSession) Close() error { return nil }

func (s *settingsWriteStubSession) SettingsDescriptor() driver.SettingsDescriptor {
	return s.descriptor
}

func (s *settingsWriteStubSession) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	return s.readSetting(ctx, id)
}

func (s *settingsWriteStubSession) WriteSetting(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
	return s.writeSetting(ctx, id, value)
}

// settingsWriteStubCodeplug builds a minimal *codeplug.Codeplug carrying
// no channels (caps.Banks is empty on settingsWriteStubSession, so an
// empty candidate always matches an empty baseline) and the given Menus.
// Channels is an explicit EMPTY (non-nil) slice, not a nil one:
// codeplug.Digest json-marshals its input, and encoding/json renders a nil
// slice as "null" but an empty one as "[]" — copyChannels (plan.go) always
// hands PrepareSend/Execute's own re-digest an empty NON-NIL slice via
// make(...), so a nil Channels here would make Execute's dual-digest
// recheck (obligation 3) see two different digests for the same content.
func settingsWriteStubCodeplug(model string, menus *codeplug.MenuSnapshot) *codeplug.Codeplug {
	return &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: generatorID,
		Radio:     codeplug.RadioInfo{Model: model},
		Channels:  []codeplug.Channel{},
		Menus:     menus,
	}
}

// TestPrepareSend_SettingsDeltaDiffsCanSetEXAddressesOnly: a descriptor
// with one CanSetEX-characterised item (Write: spec.Supported) and two
// Unverified items — one standing in for "admitted but not yet
// characterised" (no Session W row), one for "denied" — both states the
// descriptor represents identically (SettingItem.Write draws no
// distinction between them, see its doc comment). The candidate's Menus
// snapshot supplies a differing Wanted value for all three IDs. Only the
// characterised address's delta appears in Settings(), and only its ID is
// ever read: settingsDeltas must skip the live read entirely for an
// uncharacterised or denied address, not merely discard its result.
func TestPrepareSend_SettingsDeltaDiffsCanSetEXAddressesOnly(t *testing.T) {
	const characterised = "010101"
	const uncharacterised = "010102" // admitted, no Session W row yet
	const denied = "010103"          // denied — same Write value as uncharacterised

	descriptor := driver.SettingsDescriptor{
		Version: "stub@1",
		Menus: []driver.SettingMenu{{
			ID: "01", Label: "M1",
			Groups: []driver.SettingGroup{{
				ID: "0101", Label: "G1",
				Items: []driver.SettingItem{
					{ID: characterised, Label: "A", Write: spec.Supported},
					{ID: uncharacterised, Label: "B", Write: spec.Unverified},
					{ID: denied, Label: "C", Write: spec.Unverified},
				},
			}},
		}},
	}

	var readCalls []string
	sess := &settingsWriteStubSession{
		caps:       spec.Capabilities{Model: "STUB-1"},
		descriptor: descriptor,
		readSetting: func(ctx context.Context, id string) (driver.SettingValue, error) {
			readCalls = append(readCalls, id)
			return driver.SettingValue{ID: id, State: driver.SettingKnown, Raw: "111"}, nil
		},
	}
	svc := NewService(sess, newStore(t))

	file := settingsWriteStubCodeplug("STUB-1", &codeplug.MenuSnapshot{
		Descriptor: "stub@1", Complete: true,
		Entries: []codeplug.MenuEntry{
			{ID: characterised, Value: "222", State: codeplug.MenuKnown},
			{ID: uncharacterised, Value: "333", State: codeplug.MenuKnown},
			{ID: denied, Value: "444", State: codeplug.MenuKnown},
		},
	})

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}

	got := plan.Settings()
	want := []SettingDelta{{ID: characterised, Wanted: "222", Observed: "111"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("Settings() = %+v, want %+v (only the CanSetEX-characterised address)", got, want)
	}
	if len(readCalls) != 1 || readCalls[0] != characterised {
		t.Errorf("ReadSetting calls = %v, want exactly [%q] — an uncharacterised/denied address must never be read", readCalls, characterised)
	}
}

// TestExecute_SettingsWriteThenVerify: three characterised addresses,
// A/B/C. WriteSetting accepts A, verify-mismatches on B, and would fail
// the test outright if ever called for C. Execute must: abort with
// *AbortedError naming B; journal exactly
// setting_write_attempt/setting_write_result per attempted address, ending
// in "abort" (never "completion"); and retain both A's and B's results
// (but not C's) on Report.Settings, in order.
func TestExecute_SettingsWriteThenVerify(t *testing.T) {
	const addrA, addrB, addrC = "010101", "010102", "010103"

	descriptor := driver.SettingsDescriptor{
		Version: "stub@1",
		Menus: []driver.SettingMenu{{
			ID: "01", Label: "M1",
			Groups: []driver.SettingGroup{{
				ID: "0101", Label: "G1",
				Items: []driver.SettingItem{
					{ID: addrA, Label: "A", Write: spec.Supported},
					{ID: addrB, Label: "B", Write: spec.Supported},
					{ID: addrC, Label: "C", Write: spec.Supported},
				},
			}},
		}},
	}

	observed := map[string]string{addrA: "100", addrB: "200", addrC: "300"}
	wanted := map[string]string{addrA: "111", addrB: "222", addrC: "333"}

	var writeCalls []string
	sess := &settingsWriteStubSession{
		caps:       spec.Capabilities{Model: "STUB-1"},
		descriptor: descriptor,
		readSetting: func(ctx context.Context, id string) (driver.SettingValue, error) {
			return driver.SettingValue{ID: id, State: driver.SettingKnown, Raw: observed[id]}, nil
		},
		writeSetting: func(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
			writeCalls = append(writeCalls, id)
			switch id {
			case addrA:
				return driver.SettingWriteResult{ID: id, Wanted: value, Observed: value, Outcome: driver.SettingWriteAccepted}, nil
			case addrB:
				const mismatchObserved = "999"
				return driver.SettingWriteResult{ID: id, Wanted: value, Observed: mismatchObserved, Outcome: driver.SettingWriteVerifyMismatch},
					&driver.SettingVerifyMismatchError{ID: id, Wanted: value, Observed: mismatchObserved}
			default:
				t.Fatalf("WriteSetting called for %q, want the batch to have aborted at %q first", id, addrB)
				return driver.SettingWriteResult{}, nil
			}
		},
	}
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	file := settingsWriteStubCodeplug("STUB-1", &codeplug.MenuSnapshot{
		Descriptor: "stub@1", Complete: true,
		Entries: []codeplug.MenuEntry{
			{ID: addrA, Value: wanted[addrA], State: codeplug.MenuKnown},
			{ID: addrB, Value: wanted[addrB], State: codeplug.MenuKnown},
			{ID: addrC, Value: wanted[addrC], State: codeplug.MenuKnown},
		},
	})

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}
	if len(plan.Settings()) != 3 {
		t.Fatalf("plan.Settings() = %+v, want 3 deltas (test's own setup assumption)", plan.Settings())
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest())

	var aborted *AbortedError
	if !errors.As(err, &aborted) {
		t.Fatalf("Execute error = %v (%T), want *AbortedError", err, err)
	}
	if aborted.Slot != addrB {
		t.Errorf("AbortedError.Slot = %q, want %q (the mismatching address named)", aborted.Slot, addrB)
	}
	if report == nil {
		t.Fatal("report is nil, want a partial report")
	}
	if !report.Aborted {
		t.Error("report.Aborted = false, want true")
	}

	if len(report.Settings) != 2 {
		t.Fatalf("report.Settings = %+v, want exactly 2 (A accepted, B mismatch; C never attempted)", report.Settings)
	}
	if report.Settings[0].ID != addrA || report.Settings[0].Outcome != driver.SettingWriteAccepted {
		t.Errorf("report.Settings[0] = %+v, want ID=%q Outcome=SettingWriteAccepted", report.Settings[0], addrA)
	}
	if report.Settings[1].ID != addrB || report.Settings[1].Outcome != driver.SettingWriteVerifyMismatch {
		t.Errorf("report.Settings[1] = %+v, want ID=%q Outcome=SettingWriteVerifyMismatch", report.Settings[1], addrB)
	}
	if len(writeCalls) != 2 || writeCalls[0] != addrA || writeCalls[1] != addrB {
		t.Errorf("WriteSetting calls = %v, want exactly [%q, %q]", writeCalls, addrA, addrB)
	}

	events := readJournalEvents(t, report.JournalPath)
	wantTail := []string{"setting_write_attempt", "setting_write_result", "setting_write_attempt", "setting_write_result", "abort"}
	if len(events) < len(wantTail) {
		t.Fatalf("journal events = %v, too short to contain the expected tail %v", events, wantTail)
	}
	gotTail := events[len(events)-len(wantTail):]
	for i, want := range wantTail {
		if gotTail[i] != want {
			t.Errorf("journal tail[%d] = %q, want %q (full tail: %v)", i, gotTail[i], want, gotTail)
		}
	}
}
