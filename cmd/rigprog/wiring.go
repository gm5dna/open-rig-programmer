// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// openRealSession, openFakeSession, and validateModel used to be defined
// in this file alongside ft710Model/newRegistry/newRealDriver
// (task-11/task-12), then task-15 extracted the session-construction
// plumbing into internal/wiring so app/ (the M6 GUI) could share it. Task
// 40 (M9a-4, the CLI neutralisation) migrated this file's own two
// aliases off internal/wiring's Task-39 compatibility wrappers
// (wiring.OpenRealSession/wiring.OpenFakeSession, which were
// DefaultModel-only and returned the concrete *ft710.Session) onto the
// model-keyed wiring.OpenRealSessionFor/wiring.OpenFakeSessionFor, which
// return driver.Session — so cmd/rigprog no longer needs to import
// core/driver/ft710 at all. Task 41 (M9a-5, app/'s own neutralisation)
// migrated app/ off those same wrappers too and, once grep confirmed
// nothing anywhere still called them, deleted the wrappers and their
// UnexpectedSessionTypeError/UnexpectedFakeSessionTypeError types from
// internal/wiring outright.
//
// newRegistry/newRealDriver, this file's own thin aliases of
// internal/wiring's NewRegistry/NewRealDriver, were untouched by that
// migration but had already gone unused by any caller (openRealSession
// and openFakeSession call wiring.NewRegistry/wiring.NewRealDriver
// directly via wiring.OpenRealSessionFor/wiring.OpenFakeSessionFor) —
// task 44 deleted both, confirmed dead by grep repo-wide first.

// openRealSession opens a session against a real radio of model, attached
// at portPath. See internal/wiring's OpenRealSessionFor. Translates
// internal/wiring's typed errors back to this command's ORIGINAL,
// pre-extraction wording (Fix 7, adjudicated LOW, Codex M6 #7: the
// task-15 extraction's stated contract was unchanged user-facing
// wording; internal/wiring's own Error() text is deliberately generic —
// see RegisterDriverError's doc comment — so it is reconstructed here via
// errors.As against the structured Cause). The registry failure can also
// arise transitively via internal/wiring.NewRegistry inside
// OpenRealSessionFor itself — that path returns wiring's OWN wording, so
// it is translated again here alongside the serial-open/session-open
// cases. Returns the driver.Session interface — never a concrete
// *ft710.Session — so this file (and every caller of it) needs no
// core/driver/ft710 import.
func openRealSession(ctx context.Context, model, portPath string) (driver.Session, func() error, error) {
	sess, closer, err := wiring.OpenRealSessionFor(ctx, model, portPath)
	if err == nil {
		return sess, closer, nil
	}

	var regErr *wiring.RegisterDriverError
	if errors.As(err, &regErr) {
		return nil, nil, fmt.Errorf("cmd/rigprog: register driver: %w", regErr.Cause)
	}
	var serialErr *wiring.OpenSerialError
	if errors.As(err, &serialErr) {
		return nil, nil, fmt.Errorf("cmd/rigprog: open serial port %q: %w", serialErr.Port, serialErr.Cause)
	}
	var sessionErr *wiring.OpenSessionError
	if errors.As(err, &sessionErr) {
		return nil, nil, fmt.Errorf("cmd/rigprog: open session on %q: %w", sessionErr.Port, sessionErr.Cause)
	}
	return nil, nil, err
}

// openFakeSession opens a session against a fresh in-process
// internal/fakeradio.Radio for model. See internal/wiring's
// OpenFakeSessionFor. Translates internal/wiring's typed errors back to
// this command's ORIGINAL, pre-extraction wording (Fix 7 — see
// openRealSession's doc comment for the full rationale).
func openFakeSession(ctx context.Context, model string) (driver.Session, func() error, error) {
	sess, closer, err := wiring.OpenFakeSessionFor(ctx, model)
	if err == nil {
		return sess, closer, nil
	}

	var regErr *wiring.RegisterDriverError
	if errors.As(err, &regErr) {
		return nil, nil, fmt.Errorf("cmd/rigprog: register driver: %w", regErr.Cause)
	}
	var openErr *wiring.OpenFakeSessionError
	if errors.As(err, &openErr) {
		return nil, nil, fmt.Errorf("cmd/rigprog: open fake session: %w", openErr.Cause)
	}
	return nil, nil, err
}

// validateModel checks model against wiring.SupportedModels(), printing a
// usage-style diagnostic (naming every supported model, via the typed
// *wiring.UnknownModelError's own Error() text) plus cmd's own usage text
// to stderr when model is unrecognised. Every subcommand that accepts
// --model calls this immediately after flag parsing (task 40 brief:
// "Unknown model -> usage-style error, exit 2, message names supported
// models") — before any side-effecting step (a directory created, a
// session opened) — rather than relying on the eventual
// OpenRealSessionFor/OpenFakeSessionFor/StaticCapabilities/
// StaticSettingsDescriptor call's own *wiring.UnknownModelError, which
// would surface the same failure only after that step had already run.
// Returns true when model is supported (the caller proceeds unchanged).
func validateModel(stderr io.Writer, cmdName, model string, printUsage func(io.Writer)) bool {
	for _, m := range wiring.SupportedModels() {
		if m == model {
			return true
		}
	}
	err := &wiring.UnknownModelError{Model: model, Supported: wiring.SupportedModels()}
	fmt.Fprintf(stderr, "rigprog %s: %v\n", cmdName, err)
	printUsage(stderr)
	return false
}
