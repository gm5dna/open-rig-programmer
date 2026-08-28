// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
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

// TestImportCSV_NormalisesExplicitAbsentTierField pins the CSV-import root's
// capability pass: csvio correctly preserves an explicit "absent" cell, but
// an FT-710 cannot reach tx_frequency, so the merged working copy must carry
// the same Unavailable state as a loaded file (the regression for task R2's
// deferred CSV-import gap).
func TestImportCSV_NormalisesExplicitAbsentTierField(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	importSource := buildImportBase()
	importSource.Channels[0].Data.TxFreqHz = codeplug.FreqField{State: codeplug.Absent}
	importSource.Channels[0].Data.Duplex = codeplug.StringField{State: codeplug.Unknown}
	csvPath := filepath.Join(t.TempDir(), "explicit-absent.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		t.Fatalf("creating CSV fixture: %v", err)
	}
	if err := csvio.Export(f, importSource.Channels); err != nil {
		t.Fatalf("csvio.Export fixture: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing CSV fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = csvPath

	result, err := a.ImportCSV()
	if err != nil {
		t.Fatalf("ImportCSV: unexpected error: %v", err)
	}
	if !result.Merged {
		t.Fatalf("ImportCSV(explicit absent) = %+v, want Merged=true", result)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if got := a.working.Channels[0].Data.TxFreqHz.State; got != codeplug.Unavailable {
		t.Errorf("merged tx_frequency state = %q, want Unavailable", got)
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

// TestImportCHIRP_MergedChannelCarriesUnknownTagDisplay is M9c-5 E1d's
// downstream pin at the app seam (the plan names this file explicitly):
// a clean CHIRP merge puts an honestly UNRESOLVED tag_display into the
// working copy, beside radio-read siblings that keep their Known one.
//
// This is the state the GUI must be able to show and let the user set
// (task 5) and the state codeplug.Diff blocks a send on until they do
// (task 2) — so the app layer is where a regression to the pre-E1
// manufactured false would do its damage, silently switching the front
// panel display off for every imported channel.
//
// The merge is also asserted to leave Validate quiet: {Unknown, false} is
// well-formed data (only {Unknown, true} is a contradiction), so the
// import completes normally and the friction arrives at SEND time, not
// here.
func TestImportCHIRP_MergedChannelCarriesUnknownTagDisplay(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	chirpPath := filepath.Join(t.TempDir(), "clean.csv")
	// Location 2 -> slot "002", empty in buildImportBase.
	body := chirpHeaderLine + "\n" + "2,MYCALL,7.100000,,,,,USB,\n"
	if err := os.WriteFile(chirpPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = chirpPath

	result, err := a.ImportCHIRP()
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !result.Merged {
		t.Fatalf("ImportCHIRP(clean CHIRP row) = %+v, want Merged=true", result)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	var imported, untouched codeplug.Channel
	for _, ch := range a.working.Channels {
		switch ch.Slot {
		case "002":
			imported = ch
		case "001":
			untouched = ch
		}
	}
	if want := (codeplug.BoolField{State: codeplug.Unknown}); imported.Data == nil || imported.Data.TagDisplay != want {
		t.Errorf("merged slot 002 TagDisplay = %+v, want %+v (CHIRP says nothing about the display flag)", imported.Data, want)
	}
	if untouched.Data == nil || untouched.Data.TagDisplay.State != codeplug.Known {
		t.Errorf("untouched slot 001 TagDisplay = %+v, want a Known state (a CHIRP merge must not disturb its neighbours' provenance)", untouched.Data)
	}
	for _, issue := range result.Issues {
		if strings.Contains(issue.Field, "tag_display") {
			t.Errorf("Validate reported %+v for an Unknown tag_display, want silence: {Unknown, false} is well-formed data and the send gate is Diff's, not Validate's", issue)
		}
	}
}

// buildFTdx10ImportBase is buildImportBase's FTdx10 sibling: the same
// MEM 001-099 + PMS P1L-P9U inventory (that radio's static banks carry
// exactly those slots too), with slot 001 populated in the shape a real
// FTdx10 READ produces — tag_display Unavailable, since its combined MT
// record has no display flag, and tone/skip Unknown.
//
// The Radio.Model is what makes this test an end-to-end one: currentModel
// resolves it through real registration (the FTdx10 has been a registered
// model since M9c-6), so currentCaps hands ImportCHIRP that driver's OWN
// capability data and nothing here has to describe the radio.
func buildFTdx10ImportBase() *codeplug.Codeplug {
	channels := make([]codeplug.Channel, 0, 99+18)
	channels = append(channels, codeplug.Channel{Slot: "001", Data: &codeplug.ChannelData{
		FreqHz:     7_000_000,
		Mode:       "USB",
		CTCSS:      "OFF",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
		Shift:      "SIMPLEX",
		Tag:        "MYCALL",
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
	}})
	for n := 2; n <= 99; n++ {
		channels = append(channels, codeplug.Channel{Slot: fmt.Sprintf("%03d", n)})
	}
	for pair := 1; pair <= 9; pair++ {
		channels = append(channels,
			codeplug.Channel{Slot: fmt.Sprintf("P%dL", pair)},
			codeplug.Channel{Slot: fmt.Sprintf("P%dU", pair)},
		)
	}
	return &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FTdx10", CATID: "0761"},
		Channels: channels,
	}
}

// TestImportCHIRP_FTdx10MergedChannelCarriesUnavailableTagDisplay is
// M9c-6 D-tagdisplay at the app seam, and the counterpart of
// TestImportCHIRP_MergedChannelCarriesUnknownTagDisplay above: the same
// CHIRP row, the same code path, a different registered radio — and
// therefore a different, honest answer.
//
// Nothing here says what the FTdx10 can do. The working copy names the
// model, currentModel resolves it through real registration, currentCaps
// hands ImportCHIRP that driver's own capability data, and core/csvio
// derives tag_display from the target bank's FieldTagDisplay support. An
// Unknown here would be a question the user cannot answer — the radio has
// no display flag to set — and D5b's in-cell route would then let them
// manufacture one anyway. Unavailable is what the whole chain exists to
// produce.
func TestImportCHIRP_FTdx10MergedChannelCarriesUnavailableTagDisplay(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildFTdx10ImportBase()
	a.mu.Unlock()

	chirpPath := filepath.Join(t.TempDir(), "ftdx10.csv")
	// Location 2 -> slot "002", empty in the base.
	body := chirpHeaderLine + "\n" + "2,MYCALL,7.100000,,,,,USB,\n"
	if err := os.WriteFile(chirpPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = chirpPath

	result, err := a.ImportCHIRP()
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !result.Merged {
		t.Fatalf("ImportCHIRP(clean CHIRP row into an FTdx10 working copy) = %+v, want Merged=true", result)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	var imported codeplug.Channel
	for _, ch := range a.working.Channels {
		if ch.Slot == "002" {
			imported = ch
		}
	}
	if want := (codeplug.BoolField{State: codeplug.Unavailable}); imported.Data == nil || imported.Data.TagDisplay != want {
		t.Errorf("merged slot 002 TagDisplay = %+v, want %+v (this radio's memory frame has no display flag — Unknown would pose a question it cannot answer)", imported.Data, want)
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

// changingCapsSession is a minimal driver.Session stub whose Capabilities
// method returns caps[0] on its first call, caps[1] on its second, and
// holds at caps[len(caps)-1] thereafter. It exists solely so
// TestImportCHIRP_RefusesOnStaleCapabilities can reproduce "the target's
// capabilities changed between the pre-parse snapshot and the merge-time
// recheck" (Fix B2) DETERMINISTICALLY and without any concurrency at all:
// ImportCHIRP calls currentCaps exactly twice when connected (once before
// csvio.ImportCHIRP's transform, once again immediately before the
// merge), so a call-counted stub changes the answer between those two
// calls with no timing dependency whatsoever. Every other Session method
// is unreachable from ImportCHIRP's own code path (it never reads a
// channel, writes one, or asks for Identity) and panics if ever called,
// so a wiring mistake in the test itself fails loudly rather than
// silently returning a zero value.
type changingCapsSession struct {
	mu    sync.Mutex
	calls int
	caps  []spec.Capabilities
}

func (s *changingCapsSession) Capabilities() spec.Capabilities {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.calls
	if idx >= len(s.caps) {
		idx = len(s.caps) - 1
	}
	s.calls++
	return s.caps[idx]
}

func (s *changingCapsSession) Identity() driver.Identity {
	panic("changingCapsSession: Identity is not reachable from ImportCHIRP")
}

func (s *changingCapsSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	panic("changingCapsSession: ReadChannel is not reachable from ImportCHIRP")
}

func (s *changingCapsSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	panic("changingCapsSession: WriteChannel is not reachable from ImportCHIRP")
}

func (s *changingCapsSession) Close() error { return nil }

// TestImportCHIRP_RefusesOnStaleCapabilities is Fix B2's (Codex fix-B
// review, MEDIUM) functional regression test: capabilities captured
// before the (possibly slow) parse/transform must be reconfirmed before
// they are used to merge — a mismatch must refuse the merge outright
// (refuse, never corrupt), not merge data transformed against
// capabilities that no longer describe the live target. Uses
// changingCapsSession to force exactly this disagreement between
// ImportCHIRP's two currentCaps calls without any goroutines or timing at
// all.
func TestImportCHIRP_RefusesOnStaleCapabilities(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	capsBefore, err := wiring.StaticCapabilities(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities: unexpected error: %v", err)
	}
	capsAfter := capsBefore
	capsAfter.TagLen = capsBefore.TagLen + 1 // any observable difference

	sess := &changingCapsSession{caps: []spec.Capabilities{capsBefore, capsAfter}}
	a.mu.Lock()
	a.conn = &connectionState{session: sess}
	a.mu.Unlock()

	chirpPath := filepath.Join(t.TempDir(), "stale.csv")
	body := chirpHeaderLine + "\n" + "2,MYCALL,7.100000,,,,,USB,\n"
	if err := os.WriteFile(chirpPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = chirpPath

	result, err := a.ImportCHIRP()
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if result.Merged {
		t.Error("ImportCHIRP with capabilities that changed mid-import: Merged = true, want false (refuse, never corrupt)")
	}
	if result.RefusalReason == "" {
		t.Error("ImportCHIRP with capabilities that changed mid-import: RefusalReason empty, want a diagnostic")
	}
	if a.IsDirty() {
		t.Error("ImportCHIRP refused for stale capabilities set dirty, want working untouched")
	}
	if sess.calls != 2 {
		t.Errorf("changingCapsSession.Capabilities() called %d times, want exactly 2 (pre-parse snapshot + merge-time recheck)", sess.calls)
	}
}

// TestImportCHIRP_ConnReadIsSynchronised is Fix B2's (Codex fix-B review,
// MEDIUM) race-detector regression test: "The capabilities hoist added
// earlier in this milestone reads [a.conn] at app/importexport.go ~:120
// without holding the lock, while Disconnect ... mutates and closes it
// ... a genuine Go data race." Two goroutines are started independently
// (never joined to each other, only to the test via wg.Wait() at the very
// end, so Go's happens-before rules establish NO ordering between them)
// and run concurrently for many iterations: one hammering ImportCHIRP,
// the other toggling a.conn under a.mu exactly as connect/Disconnect do.
// Before this fix, `go test -race ./app/ -run
// TestImportCHIRP_ConnReadIsSynchronised` flags a DATA RACE on a.conn
// between this test's writer goroutine and ImportCHIRP's unsynchronised
// read; after it, the same run is race-clean. This test asserts nothing
// beyond "no panic" — its entire value is what -race observes, not its
// pass/fail outcome under a race-less `go test` run.
func TestImportCHIRP_ConnReadIsSynchronised(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = buildImportBase()
	a.mu.Unlock()

	chirpPath := filepath.Join(t.TempDir(), "race.csv")
	body := chirpHeaderLine + "\n" + "2,MYCALL,7.100000,,,,,USB,\n"
	if err := os.WriteFile(chirpPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = chirpPath

	sess := openTestSimSession(t)

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_, _ = a.ImportCHIRP()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			a.mu.Lock()
			if a.conn == nil {
				a.conn = &connectionState{session: sess}
			} else {
				a.conn = nil
			}
			a.mu.Unlock()
		}
	}()
	wg.Wait()
}
