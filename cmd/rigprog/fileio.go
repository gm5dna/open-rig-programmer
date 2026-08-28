// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// checkOverwrite reports whether path already exists (refused == true
// means: do not proceed without force). This is the ONE place the
// project's shared "never overwrite an existing output file without
// --force" rule lives (task-12 brief §1's read --out rule; task-13
// brief's export --csv and import --out reuse it verbatim rather than
// duplicating the Stat dance): an existing path is refused unless force
// is true; any OTHER Stat error (a permissions problem, a bad parent
// directory, etc. — not just "the file doesn't exist yet") is returned
// as err rather than silently treated as "safe to write".
func checkOverwrite(path string, force bool) (refused bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return !force, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}

// errDestExists is the sentinel saveCodeplugNoClobber and openCSVCommit
// return when the FINAL commit finds the destination already exists
// (Fix 3, adjudicated MEDIUM, Codex M4 #3): checkOverwrite's early Stat
// is a fast-fail optimisation only — a TOCTOU race window against
// whatever a long radio read spends its time on means the destination
// can be created AFTER that check passed and BEFORE the commit actually
// happens. Callers map this to the same "already exists; use --force"
// refusal checkOverwrite's own early check already produces.
var errDestExists = errors.New("destination already exists")

// openCSVCommit opens path for a CSV subcommand's final write (export
// --csv), enforcing no-clobber AT THE COMMIT rather than only via an
// earlier checkOverwrite Stat (Fix 3, adjudicated MEDIUM, Codex M4 #3):
// force=false opens O_EXCL, which refuses atomically (errDestExists,
// wrapping the underlying os.IsExist error) if path now exists — the
// file is never truncated or touched in that case. force=true opens
// O_TRUNC, the consented-overwrite case, unconditionally.
func openCSVCommit(path string, force bool) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if !force && os.IsExist(err) {
			return nil, fmt.Errorf("%s: %w: %w", path, errDestExists, err)
		}
		return nil, err
	}
	return f, nil
}

// saveCodeplugNoClobber saves cp to path for a codeplug-JSON subcommand's
// final write (read --out, import --out), enforcing no-clobber AT THE
// COMMIT rather than only via an earlier checkOverwrite Stat (Fix 3,
// adjudicated MEDIUM, Codex M4 #3).
//
// force=true saves straight to path via codeplug.Save: its own
// temp-file + fsync + rename semantics are exactly right for the
// consented-overwrite case, so no extra dance is needed.
//
// force=false instead: saves to a private temp filename in path's own
// directory (via codeplug.Save, so that write is itself atomic/durable),
// then os.Link()s the temp file onto path — a hard link fails with
// EEXIST if path now exists, exactly the atomic "create path only if it
// does not already exist" primitive plain os.Rename cannot offer (Rename
// always replaces silently). On that race, the temp file is removed and
// errDestExists is returned; path itself is never opened, truncated, or
// otherwise touched. On success the temp file's own name is removed too
// (path now holds the only remaining link to that inode) and the
// containing directory is fsynced, best-effort, exactly like
// codeplug.Save's own directory-fsync policy — see its doc comment for
// why a failure there is non-fatal.
func saveCodeplugNoClobber(path string, cp *codeplug.Codeplug, force bool) error {
	if force {
		return codeplug.Save(path, cp)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".rigprog-commit-*.tmp")
	if err != nil {
		return fmt.Errorf("creating private temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing private temp file %s: %w", tmpName, err)
	}
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmpName)
		}
	}()

	if err := codeplug.Save(tmpName, cp); err != nil {
		return err
	}

	if err := os.Link(tmpName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s: %w: %w", path, errDestExists, err)
		}
		return fmt.Errorf("linking %s to %s: %w", tmpName, path, err)
	}

	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}

// loadCodeplugStrict loads path via codeplug.Load on cmdName's behalf,
// rendering a distinct message for the schema-too-new case (task-12
// brief §2) — shared by every subcommand that strictly loads a codeplug
// file: diff's FILE argument, export's FILE argument, and import's
// --into. label names which argument this path came from (e.g.
// "--into"), included in the message for clarity; pass "" to omit it
// (diff's original phrasing, e.g. a bare positional FILE, needs no
// label).
//
// On success it returns (cp, exitSuccess) having written nothing to
// stderr. On failure it has already written stderr's message and
// returns (nil, exitError); a caller need only check for a nil cp.
func loadCodeplugStrict(stderr io.Writer, cmdName, label, path string) (*codeplug.Codeplug, int) {
	cp, err := codeplug.Load(path)
	if err == nil {
		// Deviation (c)'s CLI composition root (Wave 4 task R2): the
		// capability-keyed Absent-to-Unavailable pass codeplug.Load cannot
		// run itself, against the capabilities of the model the FILE
		// names. See codeplug.NormaliseTierFields for the rule and why it
		// lives there rather than in Load, and app/fileio.go's
		// normaliseLoadedTierFields — the GUI's identical half — for why
		// an unrecognised model is left alone rather than degraded to a
		// default one. A schema-1/2/3 file arrives here with nothing left
		// to normalise.
		if caps, capsErr := wiring.StaticCapabilities(cp.Radio.Model); capsErr == nil {
			codeplug.NormaliseTierFields(cp, caps)
		}
		return cp, exitSuccess
	}

	what := path
	if label != "" {
		what = label + " " + path
	}
	if errors.Is(err, codeplug.ErrSchemaTooNew) {
		fmt.Fprintf(stderr, "rigprog %s: %s: file format is newer than this version of rigprog supports: %v\n", cmdName, what, err)
	} else {
		fmt.Fprintf(stderr, "rigprog %s: loading %s: %v\n", cmdName, what, err)
	}
	return nil, exitError
}

// resolveSnapshotDir returns the snapshot/journal directory a radio-
// touching subcommand should use: override verbatim if non-empty,
// otherwise "<os.UserConfigDir()>/rigprog/snapshots" (task-12 brief §1's
// default). It does not touch the filesystem — callers create the
// directory (mode 0700) on demand. Originally read.go-only (task 12);
// moved here (task 14) since write.go needs it too and this file is
// where cross-subcommand file/path helpers live (checkOverwrite,
// loadCodeplugStrict) — behaviour unchanged.
//
// model then decides whether that base directory is used directly or
// namespaced (task-7, D9): wiring.DefaultModel stays at the base
// directory unchanged — byte-identical to the pre-task-7 behaviour — so
// every snapshot written before per-model subdirectories existed is
// still found. Any other model gets its own <base>/<model-slug>/
// subdirectory, applied to an explicit override too, since two models
// sharing one explicitly-named directory is exactly the collision this
// rule exists to prevent. A model whose ModelSlug is "" (no
// alphanumeric characters at all) is refused with an error rather than
// silently falling back to the base directory — filepath.Join drops
// empty elements, so an unguarded empty slug would resolve to exactly
// wiring.DefaultModel's own path, the very collision this rule exists
// to prevent. Deliberately duplicated from internal/wiring's own
// ResolveSnapshotDir rather than shared: see that function's doc
// comment (internal/wiring/wiring.go) for why.
func resolveSnapshotDir(override, model string) (string, error) {
	base := override
	if base == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("determining default snapshot directory: %w", err)
		}
		base = filepath.Join(cfgDir, "rigprog", "snapshots")
	}
	if model == wiring.DefaultModel {
		return base, nil
	}
	slug := wiring.ModelSlug(model)
	if slug == "" {
		return "", fmt.Errorf("resolving snapshot directory: model %q has no filesystem-safe characters to slug — refusing to fall back to the base directory and collide with %s's", model, wiring.DefaultModel)
	}
	return filepath.Join(base, slug), nil
}
