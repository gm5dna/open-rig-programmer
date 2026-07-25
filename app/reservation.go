// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "fmt"

// OperationBusyError is returned by UpdateChannel(s), SaveFile(As),
// LoadFile (and its loadFilePath direct-path variant), ImportCSV/CHIRP,
// ExportCSV, Disconnect, and another ReadRadio/DiffAgainstRadio/
// PrepareSend/ReadSettingsRadio/ConfirmSend call when a.opBusy (Fix 2,
// adjudicated HIGH remedy for Codex M6 #2) is held by a
// concurrently-running ReadRadio/DiffAgainstRadio/PrepareSend/
// ReadSettingsRadio call (ReadSettingsRadio added by task 35) —
// InProgress names which one. A concurrently-running SEND transfer is
// reported via the existing, pre-existing ErrTransferRunning instead (see
// checkNotBusyLocked) — this type covers only the read-side reservation
// holders.
type OperationBusyError struct {
	// InProgress is the bound-method name holding the reservation:
	// "ReadRadio", "DiffAgainstRadio", "PrepareSend", or
	// "ReadSettingsRadio" (task 35).
	InProgress string
}

func (e *OperationBusyError) Error() string {
	return fmt.Sprintf("app: another operation is already running (%s)", e.InProgress)
}

// checkNotBusyLocked refuses if EITHER of Fix 2's two composing
// reservations is held: a.opBusy (ReadRadio/DiffAgainstRadio/PrepareSend/
// ReadSettingsRadio — a typed *OperationBusyError) or a.transfer.running
// (ConfirmSend's own, pre-existing send-transfer reservation — the
// existing ErrTransferRunning sentinel, unchanged wording so every caller
// already matching on it keeps working). Used by every operation Fix 2's
// remedy lists as refused while reserved: UpdateChannel(s), SaveFile(As),
// LoadFile/loadFilePath, ImportCSV/CHIRP, ExportCSV, Disconnect, and
// ConfirmSend's own opBusy half. Callers must hold a.mu.
func (a *App) checkNotBusyLocked() error {
	if a.opBusy != "" {
		return &OperationBusyError{InProgress: a.opBusy}
	}
	if a.transfer.running {
		return ErrTransferRunning
	}
	return nil
}

// reserveOpLocked reserves a.opBusy for name — used by ReadRadio,
// DiffAgainstRadio, and ReadSettingsRadio (task 35), which (unlike
// PrepareSend) have never checked transfer.running themselves; that
// asymmetry predates this fix (see
// TestConfirmSend_CancelMidTransfer_AndBusyExclusion, which pins that a
// ReadRadio/DiffAgainstRadio call made during a running send collides
// with clone.Service's OWN op lock instead, surfacing a friendlyErr-
// wrapped *clone.BusyError — left unchanged here) and is deliberately
// preserved: this checks ONLY a.opBusy, not a.transfer.running. Refuses
// with a typed *OperationBusyError if already held; otherwise reserves
// it for name and returns nil. Callers must hold a.mu and pair a
// successful reservation with a deferred releaseOp.
func (a *App) reserveOpLocked(name string) error {
	if a.opBusy != "" {
		return &OperationBusyError{InProgress: a.opBusy}
	}
	a.opBusy = name
	return nil
}

// releaseOp releases a.opBusy. Every successful reserveOpLocked call
// must defer this immediately (mirroring core/clone/service.go's
// acquireOp/releaseOp pattern, which this is deliberately modelled on).
func (a *App) releaseOp() {
	a.mu.Lock()
	a.opBusy = ""
	a.mu.Unlock()
}
