// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// codeplugFileFilters is the file-picker filter shared by every
// codeplug-file dialog (SaveFileAs, LoadFile).
var codeplugFileFilters = []wailsruntime.FileFilter{{DisplayName: "Codeplug JSON", Pattern: "*.json"}}

// SaveFile saves the working copy to path (codeplug.Save against a DEEP
// COPY taken under mu — Fix 2, adjudicated HIGH, Codex M6 #2: the old
// shape captured a.working's own pointer under a quick lock and
// serialised it OUTSIDE mu, so a concurrent UpdateChannel could produce
// a torn save), clearing dirty and recording path as workingPath.
// Direct-path (no dialog) — SaveFileAs wraps this after resolving a path
// via the save dialog. Refused with a typed busy error while Fix 2's
// App-level reservation is held (see checkNotBusyLocked) — SaveFile does
// not itself reserve (it does not need exclusivity, only the coherent
// snapshot the deep copy already gives it), it is merely refused while
// something ELSE holds it.
func (a *App) SaveFile(path string) error {
	if path == "" {
		return ErrEmptyPath
	}
	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return err
	}
	if a.working == nil {
		a.mu.Unlock()
		return ErrNothingLoaded
	}
	workingCopy := deepCopyCodeplug(a.working)
	savedRev := a.workingRev // Fix 4: the revision this snapshot represents
	a.mu.Unlock()

	if err := codeplug.Save(path, workingCopy); err != nil {
		return fmt.Errorf("app: saving %s: %w", path, err)
	}

	a.mu.Lock()
	a.workingPath = path
	// Fix 4 (adjudicated MED, Codex M8b #4): clear dirty ONLY if the working
	// copy has not been mutated since the snapshot was taken. SaveFile does
	// not reserve a.opBusy, so a reservation holder like ReadSettingsRadio
	// can merge new settings into the working copy between the snapshot and
	// here — writing the OLD snapshot to disk while the NEWER working copy
	// would otherwise be marked clean, silently losing the refresh on quit.
	// The frontend save wrappers read the true state back via IsDirty().
	if a.workingRev == savedRev {
		a.dirty = false
	}
	a.mu.Unlock()
	return nil
}

// SaveFileAs prompts for a destination via a save dialog, then saves
// the working copy there (SaveFile). Returns ("", nil) if the user
// cancels the dialog. Refused with a typed busy error up front while
// Fix 2's reservation is held (Codex M6 #2) — before ever showing the
// dialog; SaveFile's own check covers the case the reservation is taken
// WHILE the dialog is open.
func (a *App) SaveFileAs() (string, error) {
	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return "", err
	}
	working := a.working
	defaultPath := a.workingPath
	a.mu.Unlock()
	if working == nil {
		return "", ErrNothingLoaded
	}

	path, err := a.dialogs.SaveFile(wailsruntime.SaveDialogOptions{
		Title:           "Save codeplug",
		DefaultFilename: defaultFilenameFor(defaultPath, "codeplug.json"),
		Filters:         codeplugFileFilters,
	})
	if err != nil {
		return "", fmt.Errorf("app: save dialog: %w", err)
	}
	if path == "" {
		return "", nil // user cancelled
	}
	if err := a.SaveFile(path); err != nil {
		return "", err
	}
	return path, nil
}

// LoadFile prompts for a codeplug file via an open dialog and replaces
// the working copy with it (task-15 brief §2). Refuses while a transfer
// is running, or while Fix 2's App-level exclusive-operation reservation
// is held (adjudicated HIGH, Codex M6 #2) — before ever showing the
// dialog; loadFilePath's own check covers the case the reservation is
// taken WHILE the dialog is open. If the working copy is dirty, the
// FRONTEND must ask the user first (see IsDirty) — this method does not.
func (a *App) LoadFile() (CodeplugView, error) {
	a.mu.Lock()
	err := a.checkNotBusyLocked()
	a.mu.Unlock()
	if err != nil {
		return CodeplugView{}, err
	}

	path, err := a.dialogs.OpenFile(wailsruntime.OpenDialogOptions{
		Title:   "Open codeplug",
		Filters: codeplugFileFilters,
	})
	if err != nil {
		return CodeplugView{}, fmt.Errorf("app: open dialog: %w", err)
	}
	if path == "" {
		return CodeplugView{}, nil // user cancelled: zero-value view, nil error
	}
	return a.loadFilePath(path)
}

// loadFilePath is LoadFile's direct-path variant, kept unexported for
// testability (task-15 brief §2's "direct-path variants for
// testability") without going through the OpenFile dialog. Replaces the
// working copy wholesale; does not touch baseline (a loaded file is
// independent of whatever the App last read from a radio, exactly like
// the CLI's "rigprog write FILE" taking any file). Re-checks Fix 2's
// busy reservation immediately before mutating a.working (adjudicated
// HIGH, Codex M6 #2) — the FINAL guard, since the reservation could have
// been taken after LoadFile's own up-front check but before the file
// finished loading; refused BEFORE codeplug.Load ever runs, so a busy
// refusal never masquerades as (or is masked by) a load error.
func (a *App) loadFilePath(path string) (CodeplugView, error) {
	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return CodeplugView{}, err
	}
	a.mu.Unlock()

	cp, err := codeplug.Load(path)
	if err != nil {
		return CodeplugView{}, fmt.Errorf("app: loading %s: %w", path, err)
	}
	normaliseTierFieldsForOwnModel(cp)

	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return CodeplugView{}, err
	}
	a.working = cp
	a.bumpWorkingRevLocked() // Fix 4: working-copy content replaced
	a.workingPath = path
	a.dirty = false
	view := a.codeplugViewLocked()
	a.mu.Unlock()
	return view, nil
}

// normaliseTierFieldsForOwnModel runs codeplug.NormaliseTierFields over a
// codeplug against the capabilities of the model THAT CODEPLUG names —
// the GUI half of the composition-root pass that function's doc comment
// describes (Wave 4 task R2, deviation (c)). A schema-1/2/3 file has
// nothing left for it to do; a schema-4 file whose tier keys are simply
// missing is where it bites.
//
// EVERY GUI root that resolves an Absent tier field calls this and only
// this: loadFilePath (a just-loaded file), ImportCSV (a merged "absent"
// cell) and applyEditsLocked (an edit carrying bare Absent). The last two
// used to normalise against currentCaps instead — the CONNECTED session's
// capabilities when connected — which is the same laundering by a
// different door, since this app lets the loaded working copy and the
// plugged-in radio be different radios (pinned by
// TestImportCSV_MismatchedConnectedModelKeepsReachableAbsent and
// TestUpdateChannel_MismatchedConnectedModelKeepsReachableAbsent). The
// capabilities those roots go on to Validate/merge with are a separate
// question with a separate answer, and they still ask currentCaps for it.
//
// THE CODEPLUG's own model, resolved through capsForModel, and nothing
// else:
//
//   - not the connected session's model, which may be a different radio
//     entirely: loading a file is independent of whatever is plugged in
//     (loadFilePath replaces the working copy wholesale and never touches
//     the baseline), and rewriting a file's fields against a radio it did
//     not come from is Fix B1's own failure — "an offline import
//     transforms data against the WRONG model's capabilities, and the
//     result can later pass the send gate". Reporting that mismatch is
//     Validate's radio-identity check's job, not this pass's;
//   - not wiring.DefaultModel as a fallback, which is why an unrecognised
//     or empty Radio.Model is left ALONE rather than degraded the way
//     currentModel degrades it. currentModel's fallback picks a safe
//     baseline for a QUESTION about the working copy; this pass CHANGES
//     the working copy, and "the FT-710 has no such field" is not
//     something to write into a file from a radio nobody here can name.
//     Left Absent, those fields are judged by nothing: currentCaps
//     resolves the same unrecognised model to the FT-710's baseline,
//     where every tier field is unreachable and Validate does not judge
//     it.
//
// The STATIC baseline, not a connected session's effective capabilities,
// even when the two models do match: what a tier field's reachability
// describes is the radio's memory record, which inventory discovery does
// not change. For the IC-7610 the two agree by test —
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay derives the
// same four-field list from the static baseline and from the registered
// fake's live session — and that radio discovers no bank at all.
func normaliseTierFieldsForOwnModel(cp *codeplug.Codeplug) {
	if cp == nil || cp.Radio.Model == "" {
		return
	}
	caps, err := capsForModel(cp.Radio.Model)
	if err != nil {
		return
	}
	codeplug.NormaliseTierFields(cp, caps)
}

// defaultFilenameFor returns the base name of path, or fallback if path
// is empty.
func defaultFilenameFor(path, fallback string) string {
	if path == "" {
		return fallback
	}
	return filepath.Base(path)
}

// csvFilenameFor returns a sibling ".csv" filename for path (swapping
// any existing extension), or fallback if path is empty.
func csvFilenameFor(path, fallback string) string {
	if path == "" {
		return fallback
	}
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base + ".csv"
}
