// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/userconfig"
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

// userConfigPath resolves this machine's shared settings file — the store
// holding the user's recorded consent decisions (internal/userconfig). It is
// a package-level variable rather than a direct call so that a test can point
// the whole command at a file under t.TempDir(): the production location is
// the REAL user's settings file, and no test may read, still less write, the
// settings of whoever runs "go test". The precedent is internal/wiring's own
// openSerial seam — a variable holding the production function, replaced only
// by tests, never by a flag.
var userConfigPath = userconfig.DefaultPath

// openRealSessionWith is the real-session constructor this command calls,
// held in a variable for one reason: a test cannot otherwise observe what
// consent does. Consent transforms the capability set a REAL-HARDWARE
// driver's Open assembles, so seeing it means opening a real-profile session,
// which means a serial port — and internal/wiring's own openSerial seam,
// which exists for exactly that, is package-private there. This is the
// nearest point on the same path that this package can name.
//
// PRODUCTION ALWAYS LEAVES IT AT wiring.OpenRealSessionWith. Nothing in this
// command writes to it, and no flag or configuration can: see
// cmd/rigprog/write_inprocess_test.go's ftdx10RealPortSeam for its one user
// and for what that test does, and does not, claim to prove.
var openRealSessionWith = wiring.OpenRealSessionWith

// sessionOptionsFor reads the user's recorded consent decision for model
// from the settings store and returns the wiring.SessionOptions a real
// session for that radio should be opened with.
//
// The error is RETURNED, never swallowed, and every caller treats it as fatal
// to the command (Codex #6). A settings file this build cannot read has two
// dishonest readings and no safe default: treating it as unconsented would
// refuse writes the user had authorised, and treating it as consented would
// authorise writes they had not. So the command stops, with
// internal/userconfig's own message — which names the file and says how to
// repair it — rather than a session opened on a guess.
//
// An ABSENT store is not that case: it is a first-run user, and
// userconfig.Load reports it as the zero Settings, which answers "never
// asked" for every model. That yields zero options, i.e. exactly the
// behaviour this command had before consent existed.
//
// The slug is wiring.ModelSlug(model) — the same filesystem-safe form the
// per-model snapshot directories use, so one radio has one name across
// everything this programme stores about it.
func sessionOptionsFor(model string) (wiring.SessionOptions, error) {
	path, err := userConfigPath()
	if err != nil {
		return wiring.SessionOptions{}, err
	}
	settings, err := userconfig.Load(path)
	if err != nil {
		return wiring.SessionOptions{}, err
	}
	// The "recorded" half is deliberately discarded here: this function
	// answers "may this session write unverified fields?", and both "never
	// asked" and "asked and declined" answer it identically. The difference
	// between them matters to whoever might PROMPT, not to the session.
	granted, _ := settings.UnverifiedWritesFor(wiring.ModelSlug(model))
	return wiring.SessionOptions{ConsentUnverifiedWrites: granted}, nil
}

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
// The user's recorded consent for model (sessionOptionsFor) is read here,
// once, and spent by wiring.OpenRealSessionWith — so every radio-touching
// subcommand that opens a real session (probe, read, diff, write) honours the
// same recorded decision without each having to ask. A store that cannot be
// read fails the whole call rather than defaulting either way; see
// sessionOptionsFor.
func openRealSession(ctx context.Context, model, portPath string) (driver.Session, func() error, error) {
	opts, err := sessionOptionsFor(model)
	if err != nil {
		return nil, nil, err
	}

	sess, closer, err := openRealSessionWith(ctx, model, portPath, opts)
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
