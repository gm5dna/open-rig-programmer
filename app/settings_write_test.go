// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeStubSession is a driver.Session double carrying both
// driver.SettingsReader and driver.SettingsWriter, with a caller-supplied
// descriptor and writeSetting behaviour — this package's own copy of
// core/clone/settings_write_test.go's settingsWriteStubSession (unexported
// there, in a different package). A real *ft710.Session cannot exhibit a
// characterised write address from outside core/driver/ft710 at all
// (table2-write-observed.csv is still empty in production, and that
// package's own withDialectForTest test seam is unexported — see
// core/driver/ft710/settings_test.go), so app's own WriteSetting tests use
// this stub instead, injected via connectDirect (send_test.go) exactly as
// (f1)/(f2)'s core/clone and cmd/rigprog tests use their own local stubs.
type writeStubSession struct {
	caps         spec.Capabilities
	descriptor   driver.SettingsDescriptor
	writeSetting func(ctx context.Context, id, value string) (driver.SettingWriteResult, error)
}

func (s *writeStubSession) Identity() driver.Identity       { return driver.Identity{} }
func (s *writeStubSession) Capabilities() spec.Capabilities { return s.caps }

func (s *writeStubSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	return codeplug.Channel{}, errors.New("writeStubSession: ReadChannel not implemented")
}

func (s *writeStubSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{}, errors.New("writeStubSession: WriteChannel not implemented")
}

func (s *writeStubSession) Close() error { return nil }

func (s *writeStubSession) SettingsDescriptor() driver.SettingsDescriptor {
	return s.descriptor
}

func (s *writeStubSession) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	return driver.SettingValue{}, errors.New("writeStubSession: ReadSetting not implemented")
}

func (s *writeStubSession) WriteSetting(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
	return s.writeSetting(ctx, id, value)
}

// oneItemDescriptor builds a single-menu/group/item driver.
// SettingsDescriptor naming exactly one item, id, at the given Write
// support — the smallest tree settingEditable/WriteSetting need to
// exercise their gate.
func oneItemDescriptor(id string, write spec.Support) driver.SettingsDescriptor {
	return driver.SettingsDescriptor{
		Version: "stub@1",
		Menus: []driver.SettingMenu{{
			ID: "01", Label: "M1",
			Groups: []driver.SettingGroup{{
				ID: "0101", Label: "G1",
				Items: []driver.SettingItem{{ID: id, Label: "A", Write: write}},
			}},
		}},
	}
}

// TestApp_WriteSetting_NotConnected pins the typed refusal when there is
// no connection at all — mirrors TestReadSettingsRadio_NotConnected.
func TestApp_WriteSetting_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	_, err := a.WriteSetting("010101", "005")
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("WriteSetting (not connected): err = %v, want ErrNotConnected", err)
	}
}

// TestApp_WriteSetting_RefusesWhenNotEditable: an id whose descriptor
// entry is Write: spec.Unverified (no Session W row yet) is refused with
// a typed *SettingNotEditableError BEFORE any wire traffic — the stub's
// writeSetting func is never called (it panics if it were).
func TestApp_WriteSetting_RefusesWhenNotEditable(t *testing.T) {
	a, _ := newTestApp(t)
	sess := &writeStubSession{
		caps:       spec.Capabilities{Model: "STUB-1"},
		descriptor: oneItemDescriptor("010101", spec.Unverified),
		writeSetting: func(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
			t.Fatal("WriteSetting: driver.SettingsWriter.WriteSetting called for a non-editable item")
			return driver.SettingWriteResult{}, nil
		},
	}
	connectDirect(t, a, sess, nil)

	view, err := a.WriteSetting("010101", "005")
	var notEditable *SettingNotEditableError
	if !errors.As(err, &notEditable) {
		t.Fatalf("WriteSetting (not editable): err = %v, want *SettingNotEditableError", err)
	}
	if notEditable.ID != "010101" {
		t.Errorf("SettingNotEditableError.ID = %q, want \"010101\"", notEditable.ID)
	}
	if view != (SettingWriteResultView{}) {
		t.Errorf("WriteSetting (not editable): view = %+v, want zero value", view)
	}
}

// TestApp_WriteSetting_RefusesUnknownID: an id absent from the connected
// session's own descriptor altogether is refused the same way as a
// present-but-unedatable one — settingEditable's "absent" branch.
func TestApp_WriteSetting_RefusesUnknownID(t *testing.T) {
	a, _ := newTestApp(t)
	sess := &writeStubSession{
		caps:       spec.Capabilities{Model: "STUB-1"},
		descriptor: oneItemDescriptor("010101", spec.Supported),
		writeSetting: func(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
			t.Fatal("WriteSetting: driver.SettingsWriter.WriteSetting called for an unknown item")
			return driver.SettingWriteResult{}, nil
		},
	}
	connectDirect(t, a, sess, nil)

	_, err := a.WriteSetting("999999", "005")
	var notEditable *SettingNotEditableError
	if !errors.As(err, &notEditable) {
		t.Fatalf("WriteSetting (unknown id): err = %v, want *SettingNotEditableError", err)
	}
}

// TestApp_WriteSetting_SurfacesAllFourOutcomes drives one editable
// (Write: spec.Supported) item through all four driver.SettingWriteOutcome
// values via the stub's writeSetting func, asserting each renders as a
// nil Go error and a view naming the exact Outcome string spec §7
// requires — never collapsed to a boolean, and Err non-empty for every
// outcome but accepted-and-verified.
func TestApp_WriteSetting_SurfacesAllFourOutcomes(t *testing.T) {
	const id = "010101"

	tests := []struct {
		name         string
		result       driver.SettingWriteResult
		err          error
		wantOutcome  string
		wantErrEmpty bool
	}{
		{
			name:         "accepted",
			result:       driver.SettingWriteResult{ID: id, Wanted: "005", Observed: "005", Outcome: driver.SettingWriteAccepted},
			err:          nil,
			wantOutcome:  "accepted-and-verified",
			wantErrEmpty: true,
		},
		{
			name:        "refused",
			result:      driver.SettingWriteResult{ID: id, Wanted: "005", Outcome: driver.SettingWriteRefused},
			err:         errors.New("stub: rejected by radio"),
			wantOutcome: "refused",
		},
		{
			name:        "verify-mismatch",
			result:      driver.SettingWriteResult{ID: id, Wanted: "005", Observed: "006", Outcome: driver.SettingWriteVerifyMismatch},
			err:         &driver.SettingVerifyMismatchError{ID: id, Wanted: "005", Observed: "006"},
			wantOutcome: "verify-mismatch",
		},
		{
			name:        "outcome-unknown",
			result:      driver.SettingWriteResult{ID: id, Wanted: "005", Outcome: driver.SettingWriteOutcomeUnknown},
			err:         errors.New("stub: transport failure"),
			wantOutcome: "outcome-unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			sess := &writeStubSession{
				caps:       spec.Capabilities{Model: "STUB-1"},
				descriptor: oneItemDescriptor(id, spec.Supported),
				writeSetting: func(ctx context.Context, gotID, gotValue string) (driver.SettingWriteResult, error) {
					if gotID != id || gotValue != "005" {
						t.Errorf("WriteSetting called with (%q, %q), want (%q, \"005\")", gotID, gotValue, id)
					}
					return tt.result, tt.err
				},
			}
			connectDirect(t, a, sess, nil)

			view, err := a.WriteSetting(id, "005")
			if err != nil {
				t.Fatalf("WriteSetting: unexpected Go error = %v, want nil (outcome %s is DATA, not a refusal)", err, tt.wantOutcome)
			}
			if view.ID != id || view.Wanted != tt.result.Wanted || view.Observed != tt.result.Observed {
				t.Errorf("view = %+v, want ID/Wanted/Observed = %q/%q/%q", view, id, tt.result.Wanted, tt.result.Observed)
			}
			if view.Outcome != tt.wantOutcome {
				t.Errorf("view.Outcome = %q, want %q", view.Outcome, tt.wantOutcome)
			}
			if tt.wantErrEmpty && view.Err != "" {
				t.Errorf("view.Err = %q, want empty for accepted-and-verified", view.Err)
			}
			if !tt.wantErrEmpty && view.Err == "" {
				t.Errorf("view.Err is empty, want the driver's error text for outcome %s", tt.wantOutcome)
			}
		})
	}
}

// TestApp_WriteSetting_NoSettingsWriter: a connected session whose
// concrete type does not implement driver.SettingsWriter at all answers a
// clear, typed refusal — never a panic. openTestSimSession's underlying
// *ft710.Session DOES implement SettingsWriter (so its own item is
// editable-gated instead — see TestApp_WriteSetting_RefusesWhenNotEditable
// for that path); this test needs a session that implements
// driver.SettingsReader (so settingEditable has a descriptor to consult)
// but NOT driver.SettingsWriter.
func TestApp_WriteSetting_NoSettingsWriter(t *testing.T) {
	a, _ := newTestApp(t)
	sess := &readOnlySettingsStubSession{
		caps:       spec.Capabilities{Model: "STUB-1"},
		descriptor: oneItemDescriptor("010101", spec.Supported),
	}
	connectDirect(t, a, sess, nil)

	_, err := a.WriteSetting("010101", "005")
	if err == nil {
		t.Fatal("WriteSetting (no SettingsWriter): err = nil, want a clear not-supported error")
	}
	if !errors.Is(err, clone.ErrSettingsUnsupported) {
		t.Errorf("WriteSetting (no SettingsWriter): err = %v, want errors.Is(_, clone.ErrSettingsUnsupported)", err)
	}
}

// readOnlySettingsStubSession implements driver.SettingsReader but
// deliberately NOT driver.SettingsWriter — TestApp_WriteSetting_
// NoSettingsWriter's fixture for the "driver without SettingsWriter"
// branch.
type readOnlySettingsStubSession struct {
	caps       spec.Capabilities
	descriptor driver.SettingsDescriptor
}

func (s *readOnlySettingsStubSession) Identity() driver.Identity       { return driver.Identity{} }
func (s *readOnlySettingsStubSession) Capabilities() spec.Capabilities { return s.caps }

func (s *readOnlySettingsStubSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	return codeplug.Channel{}, errors.New("readOnlySettingsStubSession: ReadChannel not implemented")
}

func (s *readOnlySettingsStubSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{}, errors.New("readOnlySettingsStubSession: WriteChannel not implemented")
}

func (s *readOnlySettingsStubSession) Close() error { return nil }

func (s *readOnlySettingsStubSession) SettingsDescriptor() driver.SettingsDescriptor {
	return s.descriptor
}

func (s *readOnlySettingsStubSession) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	return driver.SettingValue{}, errors.New("readOnlySettingsStubSession: ReadSetting not implemented")
}

// TestApp_WriteSetting_BusyExclusion mirrors TestReadSettingsRadio_
// BusyExclusion both ways: a concurrently-running holder refuses
// WriteSetting, and a concurrently-running WriteSetting itself refuses a
// subsequent ReadRadio, naming "WriteSetting".
func TestApp_WriteSetting_BusyExclusion(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t)
	connectDirect(t, a, sess, nil)

	a.mu.Lock()
	a.opBusy = "ReadRadio"
	a.mu.Unlock()
	t.Cleanup(func() {
		a.mu.Lock()
		a.opBusy = ""
		a.mu.Unlock()
	})

	_, err := a.WriteSetting("010101", "005")
	checkOperationBusy(t, "WriteSetting", err, "ReadRadio")

	a.mu.Lock()
	a.opBusy = "WriteSetting"
	a.mu.Unlock()

	_, err = a.ReadRadio()
	checkOperationBusy(t, "ReadRadio", err, "WriteSetting")
}
