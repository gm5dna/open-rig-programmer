// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// diffObservedArg is the reserved first positional that selects "rigprog
// settings diff-observed" — Runbook R step 2 (plan.md, task h1): diff a
// settings-read file's per-address P4 wire WIDTH and SHAPE against the
// pinned hardware baseline, core/cat/table2-observed.csv. Reserved on the
// same precedent as unverifiedWritesArg: a codeplug file called
// "diff-observed" cannot be given to plain "rigprog settings" and must be
// named something else.
const diffObservedArg = "diff-observed"

// diffObservedTextWidth is the fixed wire width of an EX text item's raw
// P4 field — the FT-710 profile's own TextWidth, and the same value the
// removed internal/extable/observe tool used under its own textWidth
// constant (git history, task-h1 brief). See that tool's doc comment for
// why this is evidence-collection policy, not manual schema, and why a
// mismatch fails closed rather than summarising a width.
const diffObservedTextWidth = 12

// classifyObservedShape is copied from the removed internal/extable/observe
// tool's classify function (0f62564~1:internal/extable/observe/main.go),
// not reimported — that tool's package was deleted in v1.4.1 and this
// diff tool does not resurrect it, only its shape lexicon. raw is a
// MenuEntry.Value; isText comes from table2.csv's own Text column for the
// same address. Returned errors never quote raw (privacy rule).
func classifyObservedShape(raw string, isText bool) (string, error) {
	switch {
	case isText:
		if len(raw) != diffObservedTextWidth {
			return "", fmt.Errorf("text item is %d bytes, want %d", len(raw), diffObservedTextWidth)
		}
		return "text", nil
	case strings.HasPrefix(raw, "+"), strings.HasPrefix(raw, "-"):
		if !isAllDigits(raw[1:]) {
			return "", errors.New("signed item is not a sign followed by digits")
		}
		return "signed", nil
	default:
		if !isAllDigits(raw) {
			return "", errors.New("numeric item is not digits-only")
		}
		return "numeric", nil
	}
}

// isAllDigits reports whether s is one or more ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// diffObservedRow is one address whose file width/shape disagrees with
// the observed-CSV baseline. Deliberately value-free (privacy rule): it
// carries a width and a shape class, never a MenuEntry.Value.
type diffObservedRow struct {
	id                       string
	observedWidth, fileWidth int
	observedShape, fileShape string
}

// diffObservedReport is diffAgainstObserved's result: every disagreeing
// address, plus how many addresses were compared at all.
type diffObservedReport struct {
	rows     []diffObservedRow
	compared int
}

// diffAgainstObserved compares menus' known entries against the hardware
// baseline decoded from observedCSV (core/cat/table2-observed.csv's own
// format, via extable.ParseObservedCSV), classifying each entry's shape
// with manualCSV's Text column (core/cat/table2.csv, via
// extable.ParseCSV) the same way the removed observe tool once did.
// WIDTH and SHAPE only: Value itself is read only long enough to measure
// and classify it, and never appears in the returned report.
//
// An entry that is not MenuKnown (Unavailable/Unsupported) has no wire
// answer to measure and is skipped, not reported as a difference. An
// entry whose ID is not in the observed baseline at all is skipped too —
// nothing pinned to diff it against — rather than manufactured into a
// false difference.
func diffAgainstObserved(menus *codeplug.MenuSnapshot, manualCSV, observedCSV []byte) (diffObservedReport, error) {
	if menus == nil || len(menus.Entries) == 0 {
		return diffObservedReport{}, errors.New(`carries no settings snapshot; re-read with "rigprog read --settings" to capture one`)
	}

	profile := extable.FT710Profile()
	manualRows, err := extable.ParseCSV(profile, manualCSV)
	if err != nil {
		return diffObservedReport{}, fmt.Errorf("parsing manual CSV: %w", err)
	}
	isText := make(map[string]bool, len(manualRows))
	for _, r := range manualRows {
		isText[fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)] = r.Text
	}

	observed, err := extable.ParseObservedCSV(profile, observedCSV)
	if err != nil {
		return diffObservedReport{}, fmt.Errorf("parsing observed CSV: %w", err)
	}

	var report diffObservedReport
	for _, e := range menus.Entries {
		if e.State != codeplug.MenuKnown {
			continue
		}
		obs, ok := observed[e.ID]
		if !ok {
			continue
		}
		shape, err := classifyObservedShape(e.Value, isText[e.ID])
		if err != nil {
			return diffObservedReport{}, fmt.Errorf("address %s: %w", e.ID, err)
		}
		width := len(e.Value)
		report.compared++
		if width != obs.ReadWidth || shape != obs.ReadShape {
			report.rows = append(report.rows, diffObservedRow{
				id:            e.ID,
				observedWidth: obs.ReadWidth,
				observedShape: obs.ReadShape,
				fileWidth:     width,
				fileShape:     shape,
			})
		}
	}
	sort.Slice(report.rows, func(i, j int) bool { return report.rows[i].id < report.rows[j].id })
	return report, nil
}

// writeDiffObservedReport writes r to w: one line per differing address —
// id, the observed-CSV width/shape, then the file's — followed by a
// summary line. Never a value (privacy rule): diffObservedRow carries
// none.
func writeDiffObservedReport(w io.Writer, r diffObservedReport) {
	for _, row := range r.rows {
		fmt.Fprintf(w, "%s  observed=%d/%s  file=%d/%s\n", row.id, row.observedWidth, row.observedShape, row.fileWidth, row.fileShape)
	}
	fmt.Fprintf(w, "Addresses compared: %d\n", r.compared)
	fmt.Fprintf(w, "Differences:        %d\n", len(r.rows))
}

// settingsDiffObserved implements "rigprog settings diff-observed FILE"
// (task-h1 brief, Runbook R step 2): load FILE (a "rigprog read
// --settings" capture) strictly, then diff its known entries' P4 wire
// width/shape against --observed-csv, classifying via --manual-csv's Text
// column. OFFLINE, like every other "rigprog settings" mode — no
// --port/--fake here either. Exit 0 on zero differences, 1 on any
// difference or load/parse failure, 2 on a usage error.
func settingsDiffObserved(args []string, stdout, stderr io.Writer) int {
	const name = "settings diff-observed"
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	observedCSVPath := fs.String("observed-csv", "core/cat/table2-observed.csv", "hardware observation CSV to diff FILE against")
	manualCSVPath := fs.String("manual-csv", "core/cat/table2.csv", "manual Table 2 CSV, consulted only to tell text items from numeric/signed ones")

	if ok, code := parseArgs(fs, args, name, printSettingsDiffObservedUsage, stdout, stderr); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(stderr, "rigprog %s: exactly one FILE argument is required\n", name)
		printSettingsDiffObservedUsage(stderr)
		return exitUsage
	}
	file := fs.Arg(0)

	cp, code := loadCodeplugStrict(stderr, name, "", file)
	if cp == nil {
		return code
	}

	manualCSV, err := os.ReadFile(*manualCSVPath)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog %s: reading %s: %v\n", name, *manualCSVPath, err)
		return exitError
	}
	observedCSV, err := os.ReadFile(*observedCSVPath)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog %s: reading %s: %v\n", name, *observedCSVPath, err)
		return exitError
	}

	report, err := diffAgainstObserved(cp.Menus, manualCSV, observedCSV)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog %s: %s: %v\n", name, file, err)
		return exitError
	}

	writeDiffObservedReport(stdout, report)
	if len(report.rows) > 0 {
		return exitError
	}
	return exitSuccess
}
