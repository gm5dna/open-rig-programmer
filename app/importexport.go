// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/internal/csvmerge"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	csvFileFilters   = []wailsruntime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}}
	chirpFileFilters = []wailsruntime.FileFilter{{DisplayName: "CHIRP CSV", Pattern: "*.csv"}}
)

// ImportCSV prompts for a native-format CSV via an open dialog and
// merges it onto the working copy (internal/csvmerge.MergeCSV — the
// SAME merge semantics cmd/rigprog's "rigprog import --csv" uses: exact
// slot-inventory match, full replace, refused wholesale on any
// mismatch).
//
// See ImportResultView's doc comment for why every content-level
// outcome (parse error, refused merge, success) is encoded in the
// returned view with a NIL error, rather than via the (T, error)
// error path: Wails drops a bound method's return value whenever it
// also returns a non-nil error, so a structured refusal reason would
// never reach the frontend if it travelled that way instead. This
// method's own error return is reserved for operational failures with
// nothing structured to preserve (the dialog itself failing, an
// unreadable file).
func (a *App) ImportCSV() (ImportResultView, error) {
	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return ImportResultView{}, err
	}
	working := a.working
	a.mu.Unlock()
	if working == nil {
		return ImportResultView{}, ErrNothingLoaded
	}

	path, err := a.dialogs.OpenFile(wailsruntime.OpenDialogOptions{Title: "Import CSV", Filters: csvFileFilters})
	if err != nil {
		return ImportResultView{}, fmt.Errorf("app: open dialog: %w", err)
	}
	if path == "" {
		return ImportResultView{Cancelled: true}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return ImportResultView{Path: path, ParseError: err.Error()}, nil
	}
	imported, err := csvio.Import(f)
	_ = f.Close()
	if err != nil {
		return ImportResultView{Path: path, ParseError: err.Error()}, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	// Fix 2 (adjudicated HIGH, Codex M6 #2): re-checked immediately
	// before the merge mutates a.working — the reservation could have
	// been taken after the up-front check above, while the file dialog
	// was open or the CSV was being parsed.
	if err := a.checkNotBusyLocked(); err != nil {
		return ImportResultView{}, err
	}
	if a.working == nil {
		return ImportResultView{}, ErrNothingLoaded
	}
	if err := csvmerge.MergeCSV(a.working, imported); err != nil {
		return ImportResultView{Path: path, RefusalReason: err.Error()}, nil
	}
	a.bumpWorkingRevLocked() // Fix 4: working-copy channels merged
	a.dirty = true

	caps, _ := currentCaps(a.conn, a.working)
	issues := codeplug.Validate(a.working, caps)
	return ImportResultView{Path: path, Merged: true, Issues: issuesToView(issues), Dirty: true}, nil
}

// ImportCHIRP prompts for a CHIRP-format CSV via an open dialog and
// merges it onto the working copy (internal/csvmerge.MergeCHIRP — the
// SAME sparse by-slot merge semantics cmd/rigprog's "rigprog import
// --chirp" uses: unknown target slots refuse, duplicate imported
// Locations refuse, a hard parse error dominates a blocking loss
// report, nothing merges on any refusal). See ImportCSV's doc comment
// for why the loss report/refusal reason travel via the returned view
// with a nil error rather than the error return.
func (a *App) ImportCHIRP() (ImportResultView, error) {
	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return ImportResultView{}, err
	}
	working := a.working
	a.mu.Unlock()
	if working == nil {
		return ImportResultView{}, ErrNothingLoaded
	}

	path, err := a.dialogs.OpenFile(wailsruntime.OpenDialogOptions{Title: "Import CHIRP CSV", Filters: chirpFileFilters})
	if err != nil {
		return ImportResultView{}, fmt.Errorf("app: open dialog: %w", err)
	}
	if path == "" {
		return ImportResultView{Cancelled: true}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return ImportResultView{Path: path, ParseError: err.Error()}, nil
	}
	// Fix B2 (Codex fix-B review, MEDIUM): a.conn/a.working are guarded by
	// a.mu (Disconnect, connect, ReadRadio, and LoadFile all mutate one or
	// both of them under it) — snapshotted under the lock here rather
	// than read live, which used to race a concurrent Disconnect (a
	// genuine data race under `go test -race`; see
	// TestImportCHIRP_ConnReadIsSynchronised).
	a.mu.Lock()
	caps, _ := currentCaps(a.conn, a.working)
	a.mu.Unlock()
	imported, report, err := csvio.ImportCHIRP(f, caps)
	_ = f.Close()
	lossEntries := lossEntriesToView(report)

	// csvio.ImportCHIRP's contract (core/csvio/chirp.go): ALWAYS returns
	// the fullest Channels/LossReport it can build, even alongside a
	// non-nil error or a Blocking entry — so lossEntries above is always
	// populated regardless of what follows. A hard parse error dominates
	// a blocking report (matching cmd/rigprog/import.go's cmdImport
	// precedence): checked first.
	if err != nil {
		return ImportResultView{Path: path, LossEntries: lossEntries, HasBlockingLoss: report.HasBlocking(), ParseError: err.Error()}, nil
	}
	if report.HasBlocking() {
		return ImportResultView{Path: path, LossEntries: lossEntries, HasBlockingLoss: true, RefusalReason: "CHIRP import has blocking loss entries above; resolve them and re-import"}, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	// Fix 2 (adjudicated HIGH, Codex M6 #2): re-checked immediately
	// before the merge mutates a.working — see ImportCSV's identical
	// comment.
	if err := a.checkNotBusyLocked(); err != nil {
		return ImportResultView{}, err
	}
	if a.working == nil {
		return ImportResultView{}, ErrNothingLoaded
	}
	// Fix B2 (Codex fix-B review, MEDIUM): caps above was captured under
	// a.mu, then the lock was released for the (potentially slow)
	// parse/transform above — the connection or the working copy's own
	// model may have changed underneath in that window (a Disconnect, a
	// reconnect to a different session, or a concurrent LoadFile/
	// ReadRadio replacing the working copy). imported was already
	// transformed against the OLD caps; merging it against a codeplug
	// that now describes a DIFFERENT target would recreate Fix B1's bug
	// by another route. Re-resolve now and refuse outright on any
	// disagreement — refuse, never corrupt — rather than merging
	// possibly-stale data. See TestImportCHIRP_RefusesOnStaleCapabilities.
	if fresh, _ := currentCaps(a.conn, a.working); !reflect.DeepEqual(fresh, caps) {
		return ImportResultView{Path: path, LossEntries: lossEntries, RefusalReason: "the target radio's capabilities changed while this import was in progress (reconnected, disconnected, or the codeplug was replaced); re-import to continue"}, nil
	}
	if err := csvmerge.MergeCHIRP(a.working, imported); err != nil {
		return ImportResultView{Path: path, LossEntries: lossEntries, RefusalReason: err.Error()}, nil
	}
	a.bumpWorkingRevLocked() // Fix 4: working-copy channels merged
	a.dirty = true

	// caps was fetched above, ahead of the csvio.ImportCHIRP call, and
	// just reconfirmed unchanged above.
	issues := codeplug.Validate(a.working, caps)
	return ImportResultView{Path: path, Merged: true, LossEntries: lossEntries, Issues: issuesToView(issues), Dirty: true}, nil
}

// ExportCSV prompts for a destination via a save dialog and exports
// EVERY slot of the working copy (csvio.Export) there. O_EXCL semantics
// are not needed: the save dialog itself already implies the user's
// consent to overwrite (task-15 brief §2). Returns ("", nil) if the
// user cancels the dialog.
func (a *App) ExportCSV() (string, error) {
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
		Title:           "Export CSV",
		DefaultFilename: csvFilenameFor(defaultPath, "codeplug.csv"),
		Filters:         csvFileFilters,
	})
	if err != nil {
		return "", fmt.Errorf("app: save dialog: %w", err)
	}
	if path == "" {
		return "", nil
	}

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("app: creating %s: %w", path, err)
	}
	defer f.Close()

	a.mu.Lock()
	channels := copyChannels(a.working.Channels)
	a.mu.Unlock()

	if err := csvio.Export(f, channels); err != nil {
		return "", fmt.Errorf("app: exporting %s: %w", path, err)
	}
	return path, nil
}
