// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/internal/csvmerge"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// mergeCSV/mergeCHIRP used to be defined in this file (task-13). Task-15
// extracted both into internal/csvmerge (exported as csvmerge.MergeCSV,
// csvmerge.MergeCHIRP) so app/'s ImportCSV/ImportCHIRP bound methods can
// reuse the exact same merge policy rather than a second,
// independently-drifting copy — see internal/csvmerge's package doc
// comment for why this is a shared internal package rather than
// core/codeplug (it encodes CLI/GUI product policy, not model
// semantics). internal/csvmerge's own Error() text is deliberately
// generic (it has no "--csv"/"--into" flags to name — a GUI caller has
// none either); these aliases translate its typed errors back to this
// command's ORIGINAL, unchanged user-facing wording (task-15 brief's
// hard constraint: cmd's user-facing strings must not change), via
// errors.As against the structured fields rather than string-matching.
func mergeCSV(base *codeplug.Codeplug, imported []codeplug.Channel) error {
	err := csvmerge.MergeCSV(base, imported)
	var mismatch *csvmerge.InventoryMismatchError
	if errors.As(err, &mismatch) {
		return fmt.Errorf("imported --csv slot inventory differs from --into's inventory (missing: %s; extra: %s)", joinOrNone(mismatch.Missing), joinOrNone(mismatch.Extra))
	}
	return err
}

func mergeCHIRP(base *codeplug.Codeplug, imported []codeplug.Channel) error {
	err := csvmerge.MergeCHIRP(base, imported)
	var unknown *csvmerge.UnknownSlotsError
	if errors.As(err, &unknown) {
		return fmt.Errorf("CHIRP row(s) target slot(s) not in --into's inventory: %s", strings.Join(unknown.Slots, ", "))
	}
	// *csvmerge.DuplicateSlotsError's Error() text is already identical
	// to this command's original wording — passed through unchanged.
	return err
}

// joinOrNone renders items as a comma-joined list, or "none" when empty
// — mergeCSV's own restatement of internal/csvmerge's unexported helper
// of the same name/shape, needed only to reconstruct this command's
// original --csv/--into wording from *csvmerge.InventoryMismatchError's
// structured fields.
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// writeLossReport renders report to w (task-13 brief §2: "line, column,
// value, action, detail; mark Blocking entries prominently"), called
// unconditionally for every --chirp import regardless of what follows —
// ImportCHIRP's contract is that it always returns the fullest report it
// can build, even alongside a non-nil error (see csvio.ImportCHIRP's doc
// comment).
func writeLossReport(w io.Writer, report csvio.LossReport) {
	if len(report.Entries) == 0 {
		fmt.Fprintln(w, "No CHIRP import loss.")
		return
	}
	fmt.Fprintln(w, "CHIRP import loss:")
	for _, e := range report.Entries {
		marker := ""
		if e.Blocking {
			marker = " [BLOCKING]"
		}
		fmt.Fprintf(w, "  line %d, column %s, value %q: %s — %s%s\n", e.Line, e.Column, e.Value, e.Action, e.Detail, marker)
	}
}

// writeIssues renders issues to w (task-13 brief §2, AMENDED: "print ALL
// issues — errors and warnings — under a clear advisory label"), in the
// order codeplug.Validate returned them. The offline, static baseline
// this runs against lacks the radio's discovered regional inventory
// (60m/EMG — see cmdImport's caps comment), so it inevitably reports
// spurious "not part of any bank" errors for slots only a live session
// can vouch for; the label makes clear this output never gates the exit
// code, and codeplug.Save runs regardless of what it says.
func writeIssues(w io.Writer, issues []codeplug.Issue) {
	fmt.Fprintln(w, "offline validation notes — authoritative validation happens at write time against the connected radio:")
	if len(issues) == 0 {
		fmt.Fprintln(w, "  none.")
		return
	}
	for _, is := range issues {
		slot := is.Slot
		if slot == "" {
			slot = "-"
		}
		field := string(is.Field)
		if field == "" {
			field = "-"
		}
		fmt.Fprintf(w, "  [%s] slot %s, field %s: %s\n", strings.ToUpper(string(is.Severity)), slot, field, is.Msg)
	}
}

// cmdImport implements "rigprog import" (task-13 brief §2): merge a
// native or CHIRP CSV onto an existing codeplug file (--into) and save
// the result to --out. OFFLINE — like export, no --port/--fake flags
// exist here at all.
//
// Imported CSVs carry no radio identity and (for CHIRP) no slot
// inventory, so import is a MERGE onto an existing codeplug, never a
// standalone constructor: --into supplies the Radio identity, Schema,
// and slot inventory the import merges onto.
func cmdImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	csvIn := fs.String("csv", "", "native-format CSV file to import (mutually exclusive with --chirp)")
	chirpIn := fs.String("chirp", "", "CHIRP-format CSV file to import (mutually exclusive with --csv)")
	into := fs.String("into", "", "existing codeplug JSON to merge onto (required)")
	out := fs.String("out", "", "output codeplug file path (required)")
	model := fs.String("model", wiring.DefaultModel, "radio model to validate against")
	force := fs.Bool("force", false, "overwrite --out if it already exists (required if --out equals --into)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printImportUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog import: %v\n", err)
		printImportUsage(stderr)
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "rigprog import: unexpected argument %q\n", fs.Arg(0))
		printImportUsage(stderr)
		return exitUsage
	}

	if !validateModel(stderr, "import", *model, printImportUsage) {
		return exitUsage
	}

	haveCSV := *csvIn != ""
	haveCHIRP := *chirpIn != ""
	if haveCSV == haveCHIRP { // both set, or neither
		fmt.Fprintln(stderr, "rigprog import: exactly one of --csv or --chirp is required")
		printImportUsage(stderr)
		return exitUsage
	}
	if *into == "" {
		fmt.Fprintln(stderr, "rigprog import: --into is required")
		printImportUsage(stderr)
		return exitUsage
	}
	if *out == "" {
		fmt.Fprintln(stderr, "rigprog import: --out is required")
		printImportUsage(stderr)
		return exitUsage
	}

	// Refuse-overwrite is checked before --into is even loaded (same
	// shared helper as read --out / export --csv). This is also what
	// enforces "--out may equal --into only with --force": when the two
	// paths are the same, --into obviously already exists, so this check
	// refuses it exactly like any other pre-existing --out would, unless
	// --force is given.
	refused, err := checkOverwrite(*out, *force)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog import: checking %s: %v\n", *out, err)
		return exitError
	}
	if refused {
		fmt.Fprintf(stderr, "rigprog import: %s already exists; use --force to overwrite\n", *out)
		return exitError
	}

	base, code := loadCodeplugStrict(stderr, "import", "--into", *into)
	if base == nil {
		return code
	}

	if haveCSV {
		f, err := os.Open(*csvIn)
		if err != nil {
			fmt.Fprintf(stderr, "rigprog import: opening --csv %s: %v\n", *csvIn, err)
			return exitError
		}
		imported, err := csvio.Import(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "rigprog import: parsing --csv %s: %v\n", *csvIn, err)
			return exitError
		}
		if err := mergeCSV(base, imported); err != nil {
			fmt.Fprintf(stderr, "rigprog import: %v\n", err)
			return exitError
		}
	} else {
		f, err := os.Open(*chirpIn)
		if err != nil {
			fmt.Fprintf(stderr, "rigprog import: opening --chirp %s: %v\n", *chirpIn, err)
			return exitError
		}
		imported, report, err := csvio.ImportCHIRP(f)
		f.Close()

		// ImportCHIRP's contract: ALWAYS print the fullest report it
		// built, even alongside Blocking entries or a non-nil error
		// (task-13 brief §2) — it may explain a failure that follows.
		writeLossReport(stdout, report)

		// A hard parse error is checked FIRST and dominates a blocking
		// report (task-13 brief §2, and the controller's adjudication of
		// this precedence): an incomplete parse must not masquerade as a
		// content gate. Only once err is nil does report.HasBlocking()
		// get to gate at exit 3.
		if err != nil {
			fmt.Fprintf(stderr, "rigprog import: parsing --chirp %s: %v\n", *chirpIn, err)
			return exitError
		}
		if report.HasBlocking() {
			fmt.Fprintln(stderr, "rigprog import: CHIRP import has blocking loss entries above; resolve them and re-run")
			return exitBlocked
		}
		if err := mergeCHIRP(base, imported); err != nil {
			fmt.Fprintf(stderr, "rigprog import: %v\n", err)
			return exitError
		}
	}

	// This is a NEW artefact this command produced, not a re-save of
	// whatever produced --into — always set Generator, unconditionally
	// (unlike read's applyDefaultGenerator, which only fills an empty
	// one).
	base.Generator = cliGeneratorID

	// The offline static baseline (task-13 brief §2; post-M5b-flip this
	// is model's hardware-verified real-hardware profile — its WRITE
	// supports are irrelevant here, since import only Validates values):
	// honest for an artefact with no live session behind it yet. Note
	// this never asserts a 60m/EMG bank (those are discovered per
	// session, not static — core/driver/ft710/caps.go for the FT-710),
	// so a codeplug carrying real 60m/EMG channels will always report
	// them here as "not part of any bank this radio supports". AMENDED by
	// the controller: this is ADVISORY ONLY — never exit-gating. Gating
	// on it would fail every legitimate file from a 60m-region radio;
	// real write-gating happens at send time against the live session's
	// discovered Capabilities.
	//
	// wiring.StaticCapabilities(*model) (task 40: no core/driver/ft710
	// import needed here any more) already validated model is supported
	// above — this call cannot fail on that account, but errors are still
	// handled rather than assumed, since a future model's registry
	// construction could in principle fail for other reasons
	// (wiring.RegisterDriverError).
	caps, err := wiring.StaticCapabilities(*model)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog import: %v\n", err)
		return exitError
	}
	issues := codeplug.Validate(base, caps)
	writeIssues(stdout, issues)

	// Fix 3 (adjudicated MEDIUM, Codex M4 #3): no-clobber enforced AT THE
	// COMMIT (saveCodeplugNoClobber), not just via the checkOverwrite Stat
	// near the top of this function — that earlier check is a fast-fail
	// optimisation only.
	if err := saveCodeplugNoClobber(*out, base, *force); err != nil {
		if errors.Is(err, errDestExists) {
			fmt.Fprintf(stderr, "rigprog import: %s already exists; use --force to overwrite\n", *out)
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog import: saving %s: %v\n", *out, err)
		return exitError
	}
	fmt.Fprintf(stdout, "Output: %s\n", *out)

	// A successful merge+save exits 0 regardless of what the advisory
	// validation notes above said (AMENDED — the original brief gated
	// exit 3 on a SeverityError issue here; the controller removed that
	// gating once it was clear the offline baseline's own inventory gap
	// makes it produce false positives for every legitimate 60m file).
	return exitSuccess
}
