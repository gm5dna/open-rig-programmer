// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/clone"
)

// Sentinel errors this App's bound methods return, distinct from
// whatever core/clone, core/codeplug, or internal/wiring return
// directly — a caller (today: this package's own tests; eventually the
// Svelte frontend, matching on the error string wails hands it) can
// errors.Is against these.
var (
	// ErrNotConnected is returned by any bound method that requires an
	// open radio session (ReadRadio, DiffAgainstRadio, PrepareSend,
	// ConfirmSend, Disconnect, ReadSettingsRadio) when there is none.
	ErrNotConnected = errors.New("app: not connected to a radio")
	// ErrAlreadyConnected is returned by Connect/ConnectDemo when a
	// session is already open — Disconnect first.
	ErrAlreadyConnected = errors.New("app: already connected; disconnect first")
	// ErrNothingLoaded is returned by any bound method that needs a
	// working codeplug (GetCodeplug, UpdateChannel(s), Validate,
	// DiffAgainstRadio, PrepareSend, SaveFile(As), ImportCSV/CHIRP,
	// ExportCSV, GetSettings, ReadSettingsRadio) before ReadRadio or
	// LoadFile has ever populated one.
	ErrNothingLoaded = errors.New("app: no codeplug loaded yet")
	// ErrTransferRunning is returned by Disconnect, LoadFile, and
	// ConfirmSend while a send transfer is already in progress.
	ErrTransferRunning = errors.New("app: a transfer is already running")
	// ErrNoTransferRunning is returned by CancelTransfer when there is
	// nothing to cancel.
	ErrNoTransferRunning = errors.New("app: no transfer is running")
	// ErrNoActivePlan is returned by ConfirmSend when PrepareSend has
	// not been called (or its plan was already consumed/cleared) since
	// the last successful call.
	ErrNoActivePlan = errors.New("app: no send plan is active; call PrepareSend first")
	// ErrEmptyPath is returned by SaveFile when path is empty.
	ErrEmptyPath = errors.New("app: path is empty")
)

// UnknownSlotError is returned by UpdateChannel/UpdateChannels when a
// supplied channel's Slot does not exist in the working codeplug's
// inventory (task-15 brief §2: "slot must exist in working's
// inventory"). Nothing is applied — see updateChannels' doc comment.
type UnknownSlotError struct {
	// Slot is the offending (wire-form) slot identifier.
	Slot string
}

func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("app: slot %q is not part of the working codeplug's inventory", e.Slot)
}

// DigestMismatchError is ConfirmSend's synchronous pre-check failure
// (task-15 brief §2's "digest mismatch pre-check"): the caller's
// confirmationDigest does not match currentPlan's own
// ConfirmationDigest(). Checked BEFORE spawning the transfer goroutine
// so a stale/wrong confirmation never even reaches Execute — though
// Execute would refuse it identically (clone.ErrConfirmationMismatch)
// if this check were skipped.
type DigestMismatchError struct {
	// Want is currentPlan's own ConfirmationDigest().
	Want string
	// Got is the confirmationDigest the caller actually supplied.
	Got string
}

func (e *DigestMismatchError) Error() string {
	return fmt.Sprintf("app: confirmation digest %q does not match the active plan's digest %q — the caller must confirm exactly what the user was shown", e.Got, e.Want)
}

// friendlyErr rewrites a *clone.BusyError into the "another operation is
// running" wording task-15 brief §2 requires ("surface clone's
// *BusyError as a friendly 'another operation is running' error"),
// preserving errors.Is(_, clone.ErrBusy) via %w. It also rewrites a
// *clone.SettingsUnsupportedError (task 35) into plain-language wording,
// preserving errors.Is(_, clone.ErrSettingsUnsupported) via %w — the
// connected session's driver exposes no menu/EX settings surface at all
// (see driver.SettingsReader's doc comment; today this can only arise for
// a future, non-FT-710 driver — every ft710.Session implements it
// unconditionally). Every other error is returned unchanged — friendlyErr
// is applied at every point an error from internal/wiring or core/clone
// crosses into a bound method's return value.
func friendlyErr(err error) error {
	if err == nil {
		return nil
	}
	var busy *clone.BusyError
	if errors.As(err, &busy) {
		return fmt.Errorf("app: another operation is running (%s): %w", busy.InProgress, err)
	}
	var unsupported *clone.SettingsUnsupportedError
	if errors.As(err, &unsupported) {
		return fmt.Errorf("app: this radio's driver does not expose a menu/EX settings surface (%s): %w", unsupported.Model, err)
	}
	return err
}
