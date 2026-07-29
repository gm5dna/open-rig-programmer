// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// fixedNow is a stable timestamp used throughout this package's tests —
// UTC, deterministic, no reliance on the wall clock.
var fixedNow = time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

// TestSnapshotStore_SaveSnapshot_NamesAndRoundtrips: the file is named
// "snapshot-<model>-<catid>-<timestamp>.orp.json" (obligation 9) and its
// content round-trips through codeplug.Load unchanged.
func TestSnapshotStore_SaveSnapshot_NamesAndRoundtrips(t *testing.T) {
	dir := t.TempDir()
	store := SnapshotStore{Dir: dir}

	cp := &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: generatorID,
		Radio:     codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []codeplug.Channel{{Slot: "001"}},
	}

	path, err := store.SaveSnapshot(cp, fixedNow)
	if err != nil {
		t.Fatalf("SaveSnapshot: unexpected error: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("SaveSnapshot path dir = %q, want %q", filepath.Dir(path), dir)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "snapshot-FT-710-0800-") || !strings.HasSuffix(base, ".orp.json") {
		t.Errorf("SaveSnapshot filename = %q, want prefix \"snapshot-FT-710-0800-\" and suffix \".orp.json\"", base)
	}

	loaded, err := codeplug.Load(path)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): unexpected error: %v", path, err)
	}
	if loaded.Radio.Model != cp.Radio.Model || loaded.Radio.CATID != cp.Radio.CATID {
		t.Errorf("loaded Radio = %+v, want Model/CATID matching %+v", loaded.Radio, cp.Radio)
	}
	if len(loaded.Channels) != 1 || loaded.Channels[0].Slot != "001" {
		t.Errorf("loaded Channels = %+v, want one channel \"001\"", loaded.Channels)
	}
}

// TestSnapshotStore_SaveSnapshot_SanitisesModelAndCATID: a Model/CATID
// containing filesystem-hostile bytes (e.g. a path separator) must never
// reach the filename unsanitised.
func TestSnapshotStore_SaveSnapshot_SanitisesModelAndCATID(t *testing.T) {
	dir := t.TempDir()
	store := SnapshotStore{Dir: dir}

	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "../../evil", CATID: "0800"},
	}
	path, err := store.SaveSnapshot(cp, fixedNow)
	if err != nil {
		t.Fatalf("SaveSnapshot: unexpected error: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("SaveSnapshot escaped its directory: path = %q, want under %q", path, dir)
	}
	if strings.Contains(filepath.Base(path), "/") {
		t.Errorf("SaveSnapshot filename %q was not sanitised: contains a path separator", filepath.Base(path))
	}
	// Load must find the file exactly at path — the strongest possible
	// proof that SaveSnapshot did not escape dir via the unsanitised
	// Model.
	if _, err := codeplug.Load(path); err != nil {
		t.Errorf("codeplug.Load(%s): unexpected error: %v", path, err)
	}
}

// TestSnapshotStore_SaveSnapshot_DistinctNowValuesProduceDistinctFilenames:
// two snapshots saved under two DIFFERENT now values get distinct
// filenames — nanosecond timestamp precision in snapshotFileName is what
// buys this once the underlying instants actually differ. (Renamed from
// "...UnderStaticNow": that name claimed the SAME static now value would
// still produce distinct filenames, which is false — see
// snapshotFileName's doc comment and
// TestSnapshotStore_SaveSnapshot_SameInstantOverwrites, immediately
// below, for what a genuinely static now does instead. This test's body
// never actually reused fixedNow for its second call, so the rename
// corrects the name to match what it has always actually exercised.)
func TestSnapshotStore_SaveSnapshot_DistinctNowValuesProduceDistinctFilenames(t *testing.T) {
	dir := t.TempDir()
	store := SnapshotStore{Dir: dir}
	cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Radio: codeplug.RadioInfo{Model: "FT-710", CATID: "0800"}}

	p1, err := store.SaveSnapshot(cp, fixedNow)
	if err != nil {
		t.Fatalf("first SaveSnapshot: %v", err)
	}
	time.Sleep(time.Microsecond) // force a distinguishable wall-clock instant on any OS
	p2, err := store.SaveSnapshot(cp, time.Now())
	if err != nil {
		t.Fatalf("second SaveSnapshot: %v", err)
	}
	if p1 == p2 {
		t.Errorf("two SaveSnapshot calls under different now values produced the same path %q", p1)
	}
}

// TestSnapshotStore_SaveSnapshot_SameInstantOverwrites: two snapshots
// saved under the EXACT SAME now value collide on the exact same
// filename (snapshotFileName's doc comment) — SaveSnapshot does not
// error on this, it OVERWRITES: codeplug.Save's atomic os.Rename lands
// the second call's content at the same path the first call's occupied,
// leaving no trace of the first snapshot at all.
func TestSnapshotStore_SaveSnapshot_SameInstantOverwrites(t *testing.T) {
	dir := t.TempDir()
	store := SnapshotStore{Dir: dir}
	first := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{{Slot: "001", Data: &codeplug.ChannelData{
			FreqHz: 7_000_000, Mode: "LSB", CTCSS: "OFF", Shift: "SIMPLEX",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}}},
	}
	second := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{{Slot: "001", Data: &codeplug.ChannelData{
			FreqHz: 14_000_000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}}},
	}

	p1, err := store.SaveSnapshot(first, fixedNow)
	if err != nil {
		t.Fatalf("first SaveSnapshot: %v", err)
	}
	p2, err := store.SaveSnapshot(second, fixedNow)
	if err != nil {
		t.Fatalf("second SaveSnapshot (same instant): %v", err)
	}
	if p1 != p2 {
		t.Fatalf("SaveSnapshot under the same instant produced different paths %q and %q, want the same (colliding) path", p1, p2)
	}

	loaded, err := codeplug.Load(p2)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", p2, err)
	}
	if len(loaded.Channels) != 1 || loaded.Channels[0].Data == nil || loaded.Channels[0].Data.FreqHz != 14_000_000 {
		t.Errorf("loaded content = %+v, want the SECOND SaveSnapshot's content (14000000 Hz) — the first must have been overwritten, not preserved alongside it", loaded.Channels)
	}
}

// TestJournal_PathBesideSnapshot: OpenJournal derives the same directory
// and stem as the snapshot, swapping ".orp.json" for ".jsonl".
func TestJournal_PathBesideSnapshot(t *testing.T) {
	store := SnapshotStore{Dir: "/tmp/x"}
	j := store.OpenJournal("/tmp/x/snapshot-FT-710-0800-20260711T120000.000000000Z.orp.json")
	want := "/tmp/x/snapshot-FT-710-0800-20260711T120000.000000000Z.jsonl"
	if j.Path() != want {
		t.Errorf("Journal.Path() = %q, want %q", j.Path(), want)
	}
}

// TestJournal_Append_LinesAreOrderedJSON: appended lines parse as JSON, in
// call order, and the file exists at the reported path.
func TestJournal_Append_LinesAreOrderedJSON(t *testing.T) {
	dir := t.TempDir()
	store := SnapshotStore{Dir: dir}
	j := store.OpenJournal(filepath.Join(dir, "snapshot-FT-710-0800-x.orp.json"))

	if err := j.Append(fixedNow, "prepare", map[string]any{"snapshot": "x"}); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := j.Append(fixedNow.Add(time.Second), "write_attempt", map[string]any{"slot": "001"}); err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	if err := j.Append(fixedNow.Add(2*time.Second), "completion", map[string]any{"written": float64(1)}); err != nil {
		t.Fatalf("Append 3: %v", err)
	}

	if _, err := os.Stat(j.Path()); err != nil {
		t.Fatalf("journal file missing at reported path: %v", err)
	}

	raw, err := os.ReadFile(j.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("journal has %d lines, want 3: %q", len(lines), string(raw))
	}

	wantEvents := []string{"prepare", "write_attempt", "completion"}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d does not parse as JSON: %v (%q)", i, err, line)
		}
		if rec["event"] != wantEvents[i] {
			t.Errorf("line %d event = %v, want %q (out of order or wrong content)", i, rec["event"], wantEvents[i])
		}
		if _, ok := rec["t"]; !ok {
			t.Errorf("line %d missing \"t\" timestamp field", i)
		}
	}
	if lines[1] != "" {
		var rec map[string]any
		json.Unmarshal([]byte(lines[1]), &rec)
		if rec["slot"] != "001" {
			t.Errorf("write_attempt line slot = %v, want \"001\"", rec["slot"])
		}
	}
}

// TestJournal_Append_CreatesDirlessFileEachCallFsyncs is really just a
// smoke test that repeated Append calls do not clobber earlier lines (each
// call independently opens in append mode) and that the file is left
// non-empty and readable — see Append's doc comment for the fsync-per-line
// tradeoff this pins down operationally.
// TestJournal_Append_ReturnsErrorForUnwritablePath: Append must report a
// real error (never panic) when the journal file cannot be opened — the
// path Service.journalAppend relies on to decide whether to log a
// diagnostic.
func TestJournal_Append_ReturnsErrorForUnwritablePath(t *testing.T) {
	j := &Journal{path: filepath.Join(t.TempDir(), "no-such-subdir", "journal.jsonl")}
	if err := j.Append(fixedNow, "x", nil); err == nil {
		t.Error("Append into a nonexistent directory = nil error, want an error")
	}
}

func TestJournal_Append_AppendsRatherThanTruncates(t *testing.T) {
	dir := t.TempDir()
	store := SnapshotStore{Dir: dir}
	j := store.OpenJournal(filepath.Join(dir, "snapshot-FT-710-0800-x.orp.json"))

	for i := 0; i < 5; i++ {
		if err := j.Append(fixedNow, "step", map[string]any{"i": i}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	raw, err := os.ReadFile(j.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := strings.Count(string(raw), "\n")
	if got != 5 {
		t.Errorf("journal has %d lines, want 5 (append must not truncate)", got)
	}
}
