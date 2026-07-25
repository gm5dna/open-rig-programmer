// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
)

// buildImportBase mirrors cmd/rigprog/import_test.go's buildValidBase:
// a synthetic Codeplug matching the ft710 driver's static offline
// Capabilities exactly (MEM "001".."099" + PMS "P1L".."P9U"), M-01 and
// every PMS pair populated, everything else empty — MYCALL-style
// synthetic fixture only.
func buildImportBase() *codeplug.Codeplug {
	channels := make([]codeplug.Channel, 0, 99+18)
	channels = append(channels, writableChannel("001", 7_000_000, "MYCALL"))
	for n := 2; n <= 99; n++ {
		channels = append(channels, codeplug.Channel{Slot: fmt.Sprintf("%03d", n)})
	}
	for pair := 1; pair <= 9; pair++ {
		channels = append(channels,
			writableChannel(fmt.Sprintf("P%dL", pair), 14_000_000, ""),
			writableChannel(fmt.Sprintf("P%dU", pair), 14_100_000, ""),
		)
	}
	return &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: channels,
	}
}

func TestImportCSV_MergeSuccess(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	// Export the CURRENT working copy, then re-import it: a lossless
	// round trip that should merge cleanly (task-15 brief §2's --csv
	// full-inventory exact-match rule).
	csvPath := filepath.Join(t.TempDir(), "roundtrip.csv")
	a.dialogs.(*fakeDialogs).saveFilePath = csvPath
	if _, err := a.ExportCSV(); err != nil {
		t.Fatalf("ExportCSV: unexpected error: %v", err)
	}

	a.dialogs.(*fakeDialogs).openFilePath = csvPath
	result, err := a.ImportCSV()
	if err != nil {
		t.Fatalf("ImportCSV: unexpected error: %v", err)
	}
	if !result.Merged || result.RefusalReason != "" || result.ParseError != "" {
		t.Errorf("ImportCSV(roundtrip) = %+v, want Merged=true, no refusal/parse error", result)
	}
	if !a.IsDirty() {
		t.Error("ImportCSV(merged) did not set dirty")
	}
}

func TestImportCSV_InventoryMismatchRefuses(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001"}, {Slot: "002"}}}
	a.mu.Unlock()

	// A CSV with a DIFFERENT slot inventory ("001","003" instead of
	// "001","002") — built via csvio.Export itself so the file is
	// always syntactically valid; only the inventory mismatch should
	// cause the refusal.
	csvPath := filepath.Join(t.TempDir(), "mismatched.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		t.Fatalf("creating CSV fixture: %v", err)
	}
	if err := csvio.Export(f, []codeplug.Channel{{Slot: "001"}, {Slot: "003"}}); err != nil {
		t.Fatalf("csvio.Export fixture: %v", err)
	}
	_ = f.Close()
	a.dialogs.(*fakeDialogs).openFilePath = csvPath

	result, err := a.ImportCSV()
	if err != nil {
		t.Fatalf("ImportCSV: unexpected error (refusal should travel via the view, not the error return): %v", err)
	}
	if result.Merged {
		t.Error("ImportCSV(inventory mismatch): Merged = true, want false")
	}
	if result.RefusalReason == "" {
		t.Error("ImportCSV(inventory mismatch): RefusalReason empty, want it to name the mismatch")
	}
	if a.IsDirty() {
		t.Error("ImportCSV(refused) set dirty, want working untouched")
	}
}

func TestImportCSV_Cancelled(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()
	// fakeDialogs zero value: OpenFile returns ("", nil) — cancelled.
	result, err := a.ImportCSV()
	if err != nil {
		t.Fatalf("ImportCSV(cancelled): unexpected error: %v", err)
	}
	if !result.Cancelled {
		t.Errorf("ImportCSV(cancelled) = %+v, want Cancelled=true", result)
	}
}

// chirpHeaderLine matches cmd/rigprog/import_test.go's own CHIRP fixture
// header exactly (a proven-good column set/order for csvio.ImportCHIRP —
// column order does not matter to the parser, but keeping it identical
// avoids any doubt).
const chirpHeaderLine = "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip"

func TestImportCHIRP_DuplicateLocationRefuses(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	chirpPath := filepath.Join(t.TempDir(), "dup.csv")
	body := chirpHeaderLine + "\n" +
		"2,MYCALL1,7.100000,,,,,USB,\n" +
		"2,MYCALL2,7.150000,,,,,USB,\n"
	if err := os.WriteFile(chirpPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = chirpPath

	result, err := a.ImportCHIRP()
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if result.Merged {
		t.Error("ImportCHIRP(duplicate Location): Merged = true, want false")
	}
	if !strings.Contains(result.RefusalReason, "M-02") {
		t.Errorf("ImportCHIRP(duplicate Location): RefusalReason = %q, want it to name M-02", result.RefusalReason)
	}
	if a.IsDirty() {
		t.Error("ImportCHIRP(refused) set dirty, want working untouched")
	}
}

func TestImportCHIRP_BlockingLossRefuses(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	chirpPath := filepath.Join(t.TempDir(), "blocking.csv")
	// An out-of-band Location (not in the target's inventory at all) is
	// an unknown-slot refusal, not a loss-report one — use a value CHIRP
	// itself cannot represent to force a Blocking loss entry instead: an
	// unsupported Mode CHIRP has no FT-710 equivalent for ("DV").
	body := chirpHeaderLine + "\n" + "3,MYCALL,7.100000,,,,,DV,\n"
	if err := os.WriteFile(chirpPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = chirpPath

	result, err := a.ImportCHIRP()
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if result.Merged {
		t.Error("ImportCHIRP(blocking loss): Merged = true, want false")
	}
	if !result.HasBlockingLoss || len(result.LossEntries) == 0 {
		t.Errorf("ImportCHIRP(blocking loss) = %+v, want HasBlockingLoss=true and a populated loss report", result)
	}
}

func TestExportCSV_WritesEverySlot(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	path := filepath.Join(t.TempDir(), "export.csv")
	a.dialogs.(*fakeDialogs).saveFilePath = path

	got, err := a.ExportCSV()
	if err != nil {
		t.Fatalf("ExportCSV: unexpected error: %v", err)
	}
	if got != path {
		t.Errorf("ExportCSV returned %q, want %q", got, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading exported CSV: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// header + 99 MEM + 18 PMS = 118 lines.
	if len(lines) != 118 {
		t.Errorf("ExportCSV: %d lines, want 118 (header + 99 MEM + 18 PMS)", len(lines))
	}
}

func TestExportCSV_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ExportCSV(); err == nil {
		t.Error("ExportCSV with nothing loaded: err = nil, want ErrNothingLoaded")
	}
}
