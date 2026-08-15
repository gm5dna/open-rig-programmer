// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// snapshotSuffix and journalSuffix name the two files SnapshotStore places
// side by side for one PrepareSend call: "snapshot-<model>-<catid>-<ts>
// .orp.json" (a codeplug.Save'd JSON file, obligation 9) and the same stem
// with ".jsonl" instead (the append-only journal, obligation 8).
const (
	snapshotSuffix = ".orp.json"
	journalSuffix  = ".jsonl"
)

// SnapshotStore is where PrepareSend saves the fresh baseline read (a
// snapshot, obligation 9) and where Execute's journal (obligation 8) lives,
// beside it. The zero value is unusable — Dir must name a directory that
// already exists and is writable.
type SnapshotStore struct {
	// Dir is the directory snapshots and journals are written into.
	Dir string
}

// sanitizeForFilename replaces every byte outside [A-Za-z0-9._-] with '_',
// so a radio-reported Model or CATID can never smuggle a path separator (or
// anything else filesystem-hostile) into a snapshot filename. "unknown" for
// an empty input, so the filename is still well-formed.
func sanitizeForFilename(s string) string {
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// snapshotFileName builds "snapshot-<model>-<catid>-<timestamp>.orp.json"
// per obligation 9. The timestamp is UTC, to nanosecond precision — but
// nanosecond precision only distinguishes two calls whose Now readings
// actually differ. Two SaveSnapshot calls given the EXACT SAME injected
// instant (a static test clock reused across calls, e.g. WithNow(func()
// time.Time { return fixedNow })) format to the identical string and so
// collide on the identical path: codeplug.Save's atomic os.Rename simply
// OVERWRITES the first file with the second's content — no error, no
// distinct file, and no way to recover the first snapshot's content
// afterwards. This is a non-issue with any real clock (time.Now, or
// stepClock in this package's own tests): two calls close enough in
// wall-clock time to share a nanosecond reading are not achievable in
// practice. It only matters for a test deliberately holding Now static
// across two SaveSnapshot calls — see
// TestSnapshotStore_SaveSnapshot_SameInstantOverwrites, which pins this
// down directly, and stepClock (helpers_test.go), which is what this
// package's OWN tests use instead whenever two artefacts from the same
// Execute/PrepareSend pair need distinguishable timestamps.
func snapshotFileName(model, catID string, now time.Time) string {
	ts := now.UTC().Format("20060102T150405.000000000Z")
	return fmt.Sprintf("snapshot-%s-%s-%s%s", sanitizeForFilename(model), sanitizeForFilename(catID), ts, snapshotSuffix)
}

// journalPathFor derives a journal's path from its snapshot's path: same
// directory and stem, ".jsonl" instead of ".orp.json".
func journalPathFor(snapshotPath string) string {
	return strings.TrimSuffix(snapshotPath, snapshotSuffix) + journalSuffix
}

// SaveSnapshot writes cp (the fresh baseline read) to s.Dir as an atomic,
// durable codeplug JSON file (codeplug.Save) and returns the path it was
// written to. now is the injected clock's reading at PrepareSend time —
// see snapshotFileName.
func (s SnapshotStore) SaveSnapshot(cp *codeplug.Codeplug, now time.Time) (string, error) {
	path := filepath.Join(s.Dir, snapshotFileName(cp.Radio.Model, cp.Radio.CATID, now))
	if err := codeplug.Save(path, cp); err != nil {
		return "", fmt.Errorf("clone: SaveSnapshot: %w", err)
	}
	return path, nil
}

// OpenJournal returns the Journal that lives beside the snapshot at
// snapshotPath. It does not touch the filesystem itself — Append does, on
// each call — so OpenJournal may be called repeatedly (PrepareSend once,
// Execute again later) without holding anything open across the gap
// between them.
func (s SnapshotStore) OpenJournal(snapshotPath string) *Journal {
	return &Journal{path: journalPathFor(snapshotPath)}
}

// Journal is the append-only *.jsonl file recording every step of one
// PrepareSend/Execute pair (obligation 8): "prepare", the snapshot path,
// each per-channel write attempt and its result, each verify result, an
// abort, and completion.
//
// The "prepare" line also carries "consented_unverified" (see
// capsConsented, plan.go): true when this SESSION'S capability set
// carries a ConsentedUnverified write label ANYWHERE — a user's recorded
// consent to unverified writes for this radio model opened the write
// gate — and false otherwise. It is deliberately session-wide, not
// plan-scoped: it does not claim that any field THIS plan writes was
// consented, only that this session was permitted to write on that
// basis. Present on every prepare line, both values, so an absent field
// and a false one are never confused in an append-only file.
type Journal struct {
	path string
}

// Path returns the journal's file path.
func (j *Journal) Path() string { return j.path }

// Append writes one JSON line — {"t":<now>,"event":<event>, ...fields} —
// to the journal, creating it on first use.
//
// Durability/cost tradeoff, deliberately accepted: each call independently
// opens the file in append mode, writes the line, fsyncs, and closes it,
// rather than holding one handle open across PrepareSend and Execute (which
// may be arbitrarily far apart in wall-clock time — a user reviewing a diff
// — and are not necessarily even the same process). The open+write+fsync+
// close cost is paid on every call, but journal entries are small (one
// line) and rare (at most a few per channel, for at most a few hundred
// channels), so the aggregate cost is negligible next to what it buys:
// every line is durable on disk before Execute relies on the event it
// records, even across a crash between two calls.
func (j *Journal) Append(now time.Time, event string, fields map[string]any) error {
	rec := make(map[string]any, len(fields)+2)
	for k, v := range fields {
		rec[k] = v
	}
	rec["t"] = now.UTC()
	rec["event"] = event

	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("clone: journal %s: marshal %q event: %w", j.path, event, err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("clone: journal %s: open: %w", j.path, err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("clone: journal %s: write %q event: %w", j.path, event, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("clone: journal %s: fsync %q event: %w", j.path, event, err)
	}
	return nil
}
