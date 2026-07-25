// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// checkOperationBusy is TestOperationBusyError_RefusesEditsAndOtherOpsWhileReserved's
// shared assertion: err must be a *OperationBusyError naming wantHolder.
func checkOperationBusy(t *testing.T, name string, err error, wantHolder string) {
	t.Helper()
	var busy *OperationBusyError
	if !errors.As(err, &busy) {
		t.Errorf("%s while a.opBusy=%q: err = %v, want *OperationBusyError", name, wantHolder, err)
		return
	}
	if busy.InProgress != wantHolder {
		t.Errorf("%s: InProgress = %q, want %q", name, busy.InProgress, wantHolder)
	}
}

// TestOperationBusyError_RefusesEditsAndOtherOpsWhileReserved pins Fix 2
// (Codex M6 #2, adjudicated HIGH): while a.opBusy holds the App-level
// exclusive-operation reservation ReadRadio/DiffAgainstRadio/PrepareSend
// take for their duration, every operation the remedy names —
// UpdateChannel(s), SaveFile(As), LoadFile (and its loadFilePath
// direct-path variant), ImportCSV/CHIRP, ExportCSV, Disconnect, another
// radio op, and ConfirmSend — must be refused with a typed
// *OperationBusyError naming the reservation holder, never silently
// interleaved. a.opBusy is set directly (white-box, same package)
// rather than genuinely racing a real ReadRadio call, so this is
// deterministic and never risks tearing down a real in-flight session —
// the companion race test below covers the genuine-concurrency case.
func TestOperationBusyError_RefusesEditsAndOtherOpsWhileReserved(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t)
	connectDirect(t, a, sess, nil)

	a.mu.Lock()
	a.working = matchingCandidateFile(sess.Capabilities(), minimalFactoryPopulated(), nil)
	a.mu.Unlock()

	a.mu.Lock()
	a.opBusy = "ReadRadio"
	a.mu.Unlock()
	t.Cleanup(func() {
		a.mu.Lock()
		a.opBusy = ""
		a.mu.Unlock()
	})

	t.Run("UpdateChannel", func(t *testing.T) {
		_, err := a.UpdateChannel(writableChannel("001", 7_100_000, ""))
		checkOperationBusy(t, "UpdateChannel", err, "ReadRadio")
	})
	t.Run("UpdateChannels", func(t *testing.T) {
		_, err := a.UpdateChannels([]codeplug.Channel{writableChannel("001", 7_100_000, "")})
		checkOperationBusy(t, "UpdateChannels", err, "ReadRadio")
	})
	t.Run("SaveFile", func(t *testing.T) {
		err := a.SaveFile(filepath.Join(t.TempDir(), "x.json"))
		checkOperationBusy(t, "SaveFile", err, "ReadRadio")
	})
	t.Run("SaveFileAs", func(t *testing.T) {
		_, err := a.SaveFileAs()
		checkOperationBusy(t, "SaveFileAs", err, "ReadRadio")
	})
	t.Run("LoadFile", func(t *testing.T) {
		_, err := a.LoadFile()
		checkOperationBusy(t, "LoadFile", err, "ReadRadio")
	})
	t.Run("loadFilePath", func(t *testing.T) {
		// A nonexistent path: the busy refusal must fire BEFORE any
		// attempt to read it, so this never surfaces a file-not-found
		// error instead.
		_, err := a.loadFilePath(filepath.Join(t.TempDir(), "nonexistent.json"))
		checkOperationBusy(t, "loadFilePath", err, "ReadRadio")
	})
	t.Run("ImportCSV", func(t *testing.T) {
		_, err := a.ImportCSV()
		checkOperationBusy(t, "ImportCSV", err, "ReadRadio")
	})
	t.Run("ImportCHIRP", func(t *testing.T) {
		_, err := a.ImportCHIRP()
		checkOperationBusy(t, "ImportCHIRP", err, "ReadRadio")
	})
	t.Run("ExportCSV", func(t *testing.T) {
		_, err := a.ExportCSV()
		checkOperationBusy(t, "ExportCSV", err, "ReadRadio")
	})
	t.Run("DiffAgainstRadio", func(t *testing.T) {
		_, err := a.DiffAgainstRadio()
		checkOperationBusy(t, "DiffAgainstRadio", err, "ReadRadio")
	})
	t.Run("PrepareSend", func(t *testing.T) {
		_, err := a.PrepareSend()
		checkOperationBusy(t, "PrepareSend", err, "ReadRadio")
	})
	t.Run("Disconnect", func(t *testing.T) {
		err := a.Disconnect()
		checkOperationBusy(t, "Disconnect", err, "ReadRadio")
	})
	t.Run("ConfirmSend", func(t *testing.T) {
		err := a.ConfirmSend("whatever", "")
		checkOperationBusy(t, "ConfirmSend", err, "ReadRadio")
	})
	t.Run("another ReadRadio", func(t *testing.T) {
		_, err := a.ReadRadio()
		checkOperationBusy(t, "ReadRadio", err, "ReadRadio")
	})
	t.Run("ReadSettingsRadio", func(t *testing.T) {
		_, err := a.ReadSettingsRadio()
		checkOperationBusy(t, "ReadSettingsRadio", err, "ReadRadio")
	})
}

// TestReadSettingsRadio_BusyExclusion pins Fix 2's reservation working
// BOTH ways for ReadSettingsRadio's new holder value (task 35): a
// concurrently-running ReadRadio refuses ReadSettingsRadio (added to
// TestOperationBusyError_RefusesEditsAndOtherOpsWhileReserved's table
// above) — and, the direction that table cannot cover since it only ever
// sets a.opBusy="ReadRadio", a concurrently-running ReadSettingsRadio
// itself refuses a subsequent ReadRadio, naming "ReadSettingsRadio". Like
// that test, a.opBusy is set directly (white-box, same package) rather
// than genuinely racing a real ReadSettingsRadio call, for the identical
// determinism reasons.
func TestReadSettingsRadio_BusyExclusion(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t)
	connectDirect(t, a, sess, nil)

	a.mu.Lock()
	a.working = matchingCandidateFile(sess.Capabilities(), minimalFactoryPopulated(), nil)
	a.mu.Unlock()

	a.mu.Lock()
	a.opBusy = "ReadSettingsRadio"
	a.mu.Unlock()
	t.Cleanup(func() {
		a.mu.Lock()
		a.opBusy = ""
		a.mu.Unlock()
	})

	_, err := a.ReadRadio()
	checkOperationBusy(t, "ReadRadio", err, "ReadSettingsRadio")
}

// TestUpdateChannel_ConcurrentWithPrepareSend_RaceClean is the task's
// required race test: hammer UpdateChannel concurrently with a running
// PrepareSend (against a real fakeradio session, so PrepareSend's own
// internal ReadAll takes genuine wall-clock time) and run the whole
// thing under `go test -race`. Before Fix 2, PrepareSend read `working`
// outside a.mu (send.go's old PrepareSend passed a.working itself,
// unlocked, into svc.PrepareSend) while UpdateChannel concurrently
// mutated the SAME underlying struct under a.mu — a genuine, unsynchronized
// concurrent read/write the race detector catches. After Fix 2,
// PrepareSend takes a deep copy under mu AND UpdateChannel is refused
// outright (via the reservation) for the whole reserved window, so every
// UpdateChannel call must either succeed cleanly (before/after
// PrepareSend's reserved window) or be refused with a typed busy error —
// never torn, never racy.
func TestUpdateChannel_ConcurrentWithPrepareSend_RaceClean(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithSlot("P1L", pmsModifiableSeed))
	connectDirect(t, a, sess, nil)

	a.mu.Lock()
	a.working = prepareTwoDeltaCandidate(sess)
	a.mu.Unlock()

	stop := make(chan struct{})
	var okCount, busyCount int64
	var wg sync.WaitGroup
	const hammerers = 8
	for i := 0; i < hammerers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, err := a.UpdateChannel(writableChannel("001", 7_123_456, "HAMMER"))
				switch {
				case err == nil:
					atomic.AddInt64(&okCount, 1)
				case errors.As(err, new(*OperationBusyError)):
					atomic.AddInt64(&busyCount, 1)
				default:
					t.Errorf("UpdateChannel concurrent with PrepareSend: unexpected error: %v", err)
				}
			}
		}()
	}

	if _, err := a.PrepareSend(); err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}
	close(stop)
	wg.Wait()

	if atomic.LoadInt64(&okCount) == 0 {
		t.Error("UpdateChannel never succeeded across the whole run (before/after PrepareSend's reservation) — test setup suspicious")
	}
	if atomic.LoadInt64(&busyCount) == 0 {
		t.Error("UpdateChannel was never refused as busy — PrepareSend's reserved window may not have overlapped any edit attempt; the reservation may not be exercised by this test")
	}
}
