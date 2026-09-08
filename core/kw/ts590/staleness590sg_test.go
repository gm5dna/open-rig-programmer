// SPDX-License-Identifier: GPL-3.0-or-later

package ts590_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// sgProfile returns the TS-590SG's registration.
//
// BY ITS EXACT LOOKUP NAME, NEVER BY Package. Two registrations deliberately
// share the package clause "ts590" — the book prints two EX parameter lists
// over colliding addresses — so a package-only selection would return
// whichever of the pair the registry happened to yield first and would pass
// against the wrong radio's inventory half the time. Lookup takes the
// registry's own name and nothing else, which is why the name is spelt here.
func sgProfile(t *testing.T) extable.Profile {
	t.Helper()
	p, ok := extable.Lookup("ts590sg")
	if !ok {
		t.Fatal("extable.Lookup(\"ts590sg\"): not registered — the TS-590SG's menu table cannot be re-derived without its profile")
	}
	if p.Package != "ts590" || p.VarName != "exItems590SG" {
		t.Fatalf("Lookup(\"ts590sg\") returned Package %q VarName %q, want ts590/exItems590SG — this test would otherwise re-derive the SIBLING radio's table", p.Package, p.VarName)
	}
	return p
}

// TestEXInventory590SGGenerated_NotStale re-derives this radio's generated
// inventory from its ONE source — menu590sg.csv, the manual transcription —
// and byte-compares the result with the committed exinventory590sg_gen.go. It
// is the CI guard for the generator: CI runs plain `go test ./...` and never
// `go generate`, so without this test an edit to menu590sg.csv that was not
// regenerated, or a hand-edit of the generated file, would ship silently. On
// failure, run `go generate ./core/kw/ts590` and commit the result.
//
// IT IS ALSO WHAT MAKES TASK 5's BOOTSTRAP FILE IMPOSSIBLE TO SHIP. That file
// declared exItems590SG as an empty slice so the package would compile before
// this transcription existed; an empty inventory is byte-unequal to the
// rendered one, so an ungenerated or half-generated tree fails here rather
// than presenting an empty menu table as a valid one.
func TestEXInventory590SGGenerated_NotStale(t *testing.T) {
	p := sgProfile(t)

	csv, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, csv)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ObservationsAbsent: RenderGo requires the observation map to be EMPTY
	// rather than partial, so nil is the correct argument and not a stand-in
	// for an unread file.
	want, err := extable.RenderGo(p, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}

	got, err := os.ReadFile(p.OutFile)
	if err != nil {
		t.Fatalf("reading %s: %v", p.OutFile, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale relative to %s; run `go generate ./core/kw/ts590` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, len(want), len(got))
	}
}

// TestEXItemsSG_LengthIsTheProfilesExpectedRows is the count gate. ExpectedRows
// comes from a boundary ledger derived from the rendered PDF before any
// transcription existed, so this is the one assertion the transcription cannot
// satisfy by agreeing with itself: a row silently dropped from menu590sg.csv
// renders happily — RenderGo compares only the sets it is handed — and is
// caught here.
//
// The length is read through the package's own accessor, which is what every
// consumer sees, rather than from the generated variable directly.
func TestEXItemsSG_LengthIsTheProfilesExpectedRows(t *testing.T) {
	p := sgProfile(t)
	if got := len(ts590.EXItemsSG()); got != p.ExpectedRows {
		t.Errorf("EXItemsSG() has %d items, want the ts590sg profile's ExpectedRows (%d)", got, p.ExpectedRows)
	}
}

// TestParseCSV590SG_DeletingARowIsCaught is the count gate's red proof, run
// against the committed CSV rather than a fixture so it exercises the real
// file: strike one data row and the parsed row count no longer equals
// ExpectedRows.
//
// The deletion is done in memory. Nothing here writes to menu590sg.csv.
func TestParseCSV590SG_DeletingARowIsCaught(t *testing.T) {
	p := sgProfile(t)
	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	full, err := extable.ParseCSV(p, data)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	if len(full) != p.ExpectedRows {
		t.Fatalf("%s parses to %d rows, want ExpectedRows %d", p.ManualCSV, len(full), p.ExpectedRows)
	}

	lines := strings.Split(string(data), "\n")
	cut := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "42,0,0,") { // 042 Keying weight ratio
			cut = i
			break
		}
	}
	if cut < 0 {
		t.Fatal("menu590sg.csv no longer carries a data row for menu 042; this red proof needs a row that exists")
	}
	short := append(append([]string{}, lines[:cut]...), lines[cut+1:]...)
	rows, err := extable.ParseCSV(p, []byte(strings.Join(short, "\n")))
	if err != nil {
		t.Fatalf("ParseCSV on the shortened CSV: %v", err)
	}
	if len(rows) == p.ExpectedRows {
		t.Fatal("deleting a data row left the parsed count at ExpectedRows — the count gate would not notice a dropped menu entry")
	}
	if len(rows) != p.ExpectedRows-1 {
		t.Errorf("the shortened CSV parses to %d rows, want %d", len(rows), p.ExpectedRows-1)
	}
}

// TestParseCSV590SG_RefusesANonZeroP2OrP3 pins the address form on THIS
// radio's own profile. The chart prints one three-digit menu number and that
// number is the whole address, so a second or third component names something
// no EX frame can express. AddressSingle REFUSES it rather than dropping it: a
// value silently discarded here would reach the generated inventory as a 0
// that nothing recorded having changed.
func TestParseCSV590SG_RefusesANonZeroP2OrP3(t *testing.T) {
	p := sgProfile(t)
	for _, tc := range []struct{ name, row string }{
		{"non-zero p2", "42,1,0,,,Keying weight ratio,0: AUTO,2,false,851"},
		{"non-zero p3", "42,0,1,,,Keying weight ratio,0: AUTO,2,false,851"},
	} {
		if _, err := extable.ParseCSV(p, []byte(tc.row+"\n")); err == nil {
			t.Errorf("%s: ParseCSV accepted the row under %v; want a refusal", tc.name, p.Addresses)
		}
	}
	// The same row with both components 0 parses, so the refusals above are
	// the address rule biting and not some other field being rejected.
	if _, err := extable.ParseCSV(p, []byte("42,0,0,,,Keying weight ratio,0: AUTO,2,false,851\n")); err != nil {
		t.Errorf("ParseCSV refused the well-formed control row: %v", err)
	}
}

// TestParseCSV590SG_RefusesASecondTextWidth is the reason menu 000 is
// digits=4 text=false rather than a text row.
//
// This profile's TextWidths names ONE width and this chart prints strings of
// two: 000 "Version information (4 ASCII characters) read only" and 001
// "Power on Message (up to 8 ASCII characters)". Flagging both as text is what
// ParseCSV refuses — so the choice was never "which of the two is the text
// row" left to a transcriber's taste; the validator settles it, and the
// repository's existing treatment of a version string (the FT-891's
// 18/01/00 MAIN VERSION, digits=4 text=false) says which one gives way.
func TestParseCSV590SG_RefusesASecondTextWidth(t *testing.T) {
	p := sgProfile(t)
	const asPrinted = "1,0,0,,,Power on message,Power on Message (up to 8 ASCII characters),8,true,750\n"
	if _, err := extable.ParseCSV(p, []byte(asPrinted)); err != nil {
		t.Fatalf("ParseCSV refused the chart's one real text row: %v", err)
	}
	const versionAsText = "0,0,0,,,Firmware Version,Version information (4 ASCII characters) read only,4,true,749\n"
	if _, err := extable.ParseCSV(p, []byte(versionAsText)); err == nil {
		t.Errorf("ParseCSV accepted a text row of width 4 alongside a profile whose TextWidths is %v; want a refusal", p.TextWidths)
	}
	// And the version row as this transcription actually carries it — the same
	// width, NOT flagged text — parses, because a non-text row is bounded by
	// MinDigits..MaxDigits and MaxDigits is 4 for exactly this row.
	const versionAsPrinted = "0,0,0,,,Firmware Version,Version information (4 ASCII characters) read only,4,false,749\n"
	if _, err := extable.ParseCSV(p, []byte(versionAsPrinted)); err != nil {
		t.Errorf("ParseCSV refused menu 000 as digits=4 text=false: %v", err)
	}
}
