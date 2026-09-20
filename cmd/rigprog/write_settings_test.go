// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeSettingsStubSession is a driver.Session double carrying both
// driver.SettingsReader and driver.SettingsWriter, styled directly on
// core/clone/settings_write_test.go's settingsWriteStubSession — that
// file's own doc comment explains why a stub is used rather than a real
// *ft710.Session: table2-write-observed.csv is still empty in production
// (spec §3), and core/driver/ft710's own dialect-injection seam
// (withDialectForTest) is unexported, reachable only from inside that
// package's own tests. Channels/caps.Banks stay empty here — this file's
// tests are about the settings pipeline, not the channel one, which (e)'s
// core/clone/settings_write_test.go and write_inprocess_test.go's own
// channel-round-trip tests already cover.
type writeSettingsStubSession struct {
	caps         spec.Capabilities
	descriptor   driver.SettingsDescriptor
	readCalls    []string
	writeCalls   []string
	readSetting  func(ctx context.Context, id string) (driver.SettingValue, error)
	writeSetting func(ctx context.Context, id, value string) (driver.SettingWriteResult, error)
}

func (s *writeSettingsStubSession) Identity() driver.Identity       { return driver.Identity{} }
func (s *writeSettingsStubSession) Capabilities() spec.Capabilities { return s.caps }

func (s *writeSettingsStubSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	return codeplug.Channel{}, errors.New("writeSettingsStubSession: ReadChannel not implemented")
}

func (s *writeSettingsStubSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{}, errors.New("writeSettingsStubSession: WriteChannel not implemented")
}

func (s *writeSettingsStubSession) Close() error { return nil }

func (s *writeSettingsStubSession) SettingsDescriptor() driver.SettingsDescriptor {
	return s.descriptor
}

func (s *writeSettingsStubSession) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	s.readCalls = append(s.readCalls, id)
	return s.readSetting(ctx, id)
}

func (s *writeSettingsStubSession) WriteSetting(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
	s.writeCalls = append(s.writeCalls, id)
	return s.writeSetting(ctx, id, value)
}

// writeSettingsStubCodeplug mirrors core/clone/settings_write_test.go's
// settingsWriteStubCodeplug: an explicit EMPTY (non-nil) Channels slice —
// Execute's dual-digest recheck (obligation 3) distinguishes a nil slice
// (marshals "null") from an empty one (marshals "[]"), and copyChannels
// always hands back the latter.
func writeSettingsStubCodeplug(model string, menus *codeplug.MenuSnapshot) *codeplug.Codeplug {
	return &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "rigprog-test",
		Radio:     codeplug.RadioInfo{Model: model},
		Channels:  []codeplug.Channel{},
		Menus:     menus,
	}
}

// writeSettingsDescriptor builds a one-menu, one-group descriptor with a
// single CanSetEX-characterised item (Write: spec.Supported) plus an
// uncharacterised and a denied item — both represented identically
// (spec.Unverified), exactly like core/clone's own
// TestPrepareSend_SettingsDeltaDiffsCanSetEXAddressesOnly.
func writeSettingsDescriptor(characterised, uncharacterised, denied string) driver.SettingsDescriptor {
	return driver.SettingsDescriptor{
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
}

// TestRunWrite_SettingsFlagDiffsAndWritesCanSetEXOnly pins task f2's core
// contract: --settings extends the send to admitted, hardware-
// characterised settings addresses ONLY, and a settings-only diff (zero
// channel changes, caps.Banks is empty here) still reaches the
// confirmation gate and executes.
func TestRunWrite_SettingsFlagDiffsAndWritesCanSetEXOnly(t *testing.T) {
	const characterised = "010101"
	const uncharacterised = "010102"
	const denied = "010103"
	descriptor := writeSettingsDescriptor(characterised, uncharacterised, denied)

	newSess := func() *writeSettingsStubSession {
		return &writeSettingsStubSession{
			caps:       spec.Capabilities{Model: "STUB-1"},
			descriptor: descriptor,
			readSetting: func(ctx context.Context, id string) (driver.SettingValue, error) {
				return driver.SettingValue{ID: id, State: driver.SettingKnown, Raw: "111"}, nil
			},
			writeSetting: func(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
				return driver.SettingWriteResult{ID: id, Wanted: value, Observed: value, Outcome: driver.SettingWriteAccepted}, nil
			},
		}
	}

	file := writeSettingsStubCodeplug("STUB-1", &codeplug.MenuSnapshot{
		Descriptor: "stub@1", Complete: true,
		Entries: []codeplug.MenuEntry{
			{ID: characterised, Value: "222", State: codeplug.MenuKnown},
			{ID: uncharacterised, Value: "333", State: codeplug.MenuKnown},
			{ID: denied, Value: "444", State: codeplug.MenuKnown},
		},
	})

	t.Run("--settings: only the characterised address is read and written; settings-only diff still executes", func(t *testing.T) {
		sess := newSess()
		var stdout, stderr bytes.Buffer
		got := runWrite(testCtx(t), "STUB-1", sess, t.TempDir(), file, true, true, false, strings.NewReader(""), &stdout, &stderr)
		if got != exitSuccess {
			t.Fatalf("runWrite(--settings) = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
		}
		if len(sess.readCalls) != 1 || sess.readCalls[0] != characterised {
			t.Errorf("ReadSetting calls = %v, want exactly [%q] (uncharacterised/denied never read)", sess.readCalls, characterised)
		}
		if len(sess.writeCalls) != 1 || sess.writeCalls[0] != characterised {
			t.Errorf("WriteSetting calls = %v, want exactly [%q] (uncharacterised/denied never written)", sess.writeCalls, characterised)
		}
		if !strings.Contains(stdout.String(), "Settings:") {
			t.Errorf("stdout = %q, want the settings delta rendered in the plan summary", stdout.String())
		}
		if !strings.Contains(stdout.String(), "accepted-and-verified") {
			t.Errorf("stdout = %q, want the settings result rendered after execution", stdout.String())
		}
	})

	t.Run("without --settings: zero settings traffic, FILE's Menus entirely ignored", func(t *testing.T) {
		sess := newSess()
		var stdout, stderr bytes.Buffer
		got := runWrite(testCtx(t), "STUB-1", sess, t.TempDir(), file, false, true, false, strings.NewReader(""), &stdout, &stderr)
		if got != exitSuccess {
			t.Fatalf("runWrite(no --settings) = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
		}
		if len(sess.readCalls) != 0 {
			t.Errorf("ReadSetting calls = %v, want none — --settings was not given", sess.readCalls)
		}
		if len(sess.writeCalls) != 0 {
			t.Errorf("WriteSetting calls = %v, want none — --settings was not given", sess.writeCalls)
		}
		if strings.Contains(stdout.String(), "Settings:") || strings.Contains(stdout.String(), "Settings results:") {
			t.Errorf("stdout = %q, want no settings section at all", stdout.String())
		}
		if !strings.Contains(stdout.String(), "Nothing to send.") {
			t.Errorf("stdout = %q, want \"Nothing to send.\" (no channel changes, settings ignored)", stdout.String())
		}
	})
}

// TestRunWrite_SettingsOutcomesAllFourRender pins plan m7: every
// SettingWriteOutcome value renders after execution, not just the two
// refusal paths — a direct unit test on the rendering function itself
// (styled on write_test.go's other renderer pins, e.g.
// TestWriteExecuteSummary), since a single Execute batch can only ever
// reach one non-accepted outcome before aborting (writeSettingsBatch,
// core/clone/execute.go).
func TestRunWrite_SettingsOutcomesAllFourRender(t *testing.T) {
	results := []driver.SettingWriteResult{
		{ID: "010101", Wanted: "1", Observed: "1", Outcome: driver.SettingWriteAccepted},
		{ID: "010102", Wanted: "2", Outcome: driver.SettingWriteRefused},
		{ID: "010103", Wanted: "3", Observed: "9", Outcome: driver.SettingWriteVerifyMismatch},
		{ID: "010104", Wanted: "4", Outcome: driver.SettingWriteOutcomeUnknown},
	}
	var buf bytes.Buffer
	writeSettingsResults(&buf, results)
	out := buf.String()
	for _, want := range []string{
		"010101: accepted-and-verified",
		"010102: refused",
		"010103: verify-mismatch",
		"010104: outcome-unknown",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("writeSettingsResults output = %q, want it to contain %q", out, want)
		}
	}
}
