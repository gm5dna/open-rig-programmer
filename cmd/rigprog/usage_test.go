// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestWriteUsageText_FlagsPrecedeFile pins Fix 6 (LOW, Codex M4 #6):
// writeUsageText used to advertise "write --port <path> FILE [--yes]
// [--firmware VER] [--snapshot-dir DIR]", but stdlib flag.Parse stops
// parsing flags at the first non-flag argument — a caller typing that
// EXACT documented invocation had every flag after FILE rejected as an
// unexpected extra argument. The synopsis must list every optional flag
// BEFORE FILE, and the text must say so explicitly.
func TestWriteUsageText_FlagsPrecedeFile(t *testing.T) {
	for _, want := range []string{
		"rigprog write --port <path> [--model NAME] [--yes] [--firmware VER] [--snapshot-dir DIR] FILE",
		"rigprog write --fake [--model NAME] [--yes] [--firmware VER] [--snapshot-dir DIR] FILE",
	} {
		if !strings.Contains(writeUsageText, want) {
			t.Errorf("writeUsageText = %q, want it to contain the flags-first synopsis line %q", writeUsageText, want)
		}
	}
	if !strings.Contains(strings.ToLower(writeUsageText), "flags") || !strings.Contains(strings.ToLower(writeUsageText), "precede") {
		t.Errorf("writeUsageText = %q, want a line stating flags must precede the FILE argument", writeUsageText)
	}
}

// TestReadUsage_MentionsSettings pins task-34's read usage text update:
// the new opt-in --settings flag must be documented, both in the flags
// list and in the synopsis lines (which must still list every flag
// BEFORE the required --out/--port/--fake, per Fix 6's flags-first rule
// this file otherwise pins).
func TestReadUsage_MentionsSettings(t *testing.T) {
	if !strings.Contains(readUsageText, "--settings") {
		t.Errorf("readUsageText = %q, want it to mention --settings", readUsageText)
	}
	for _, want := range []string{
		"rigprog read --port <path> --out <file> [--settings]",
		"rigprog read --fake --out <file> [--settings]",
	} {
		if !strings.Contains(readUsageText, want) {
			t.Errorf("readUsageText = %q, want it to contain the synopsis fragment %q", readUsageText, want)
		}
	}
}

// TestSettingsUsageText_FlagsFirstAndExitCodes pins task-34's settings
// usage text: the flags-first synopsis (export's own precedent), OFFLINE
// (no --port/--fake), and that 3/4/5 are explicitly documented as unused.
func TestSettingsUsageText_FlagsFirstAndExitCodes(t *testing.T) {
	if !strings.Contains(settingsUsageText, "rigprog settings [--csv OUT] [--model NAME] [--force] FILE") {
		t.Errorf("settingsUsageText = %q, want the flags-first synopsis line", settingsUsageText)
	}
	if !strings.Contains(settingsUsageText, "OFFLINE") {
		t.Errorf("settingsUsageText = %q, want it to say OFFLINE", settingsUsageText)
	}
	if !strings.Contains(settingsUsageText, "3/4/5") {
		t.Errorf("settingsUsageText = %q, want it to say exit codes 3/4/5 are unused", settingsUsageText)
	}
}

// TestUsageText_DiffExport_AlreadyFlagsFirst pins that diff's and
// export's synopses — the other two subcommand usage texts combining
// optional/required flags with a bare positional argument — do NOT
// share write's bug: every flag already precedes the positional
// placeholder in both. Regression coverage for Fix 6's "sweep ALL
// subcommand usage texts" requirement (LOW, Codex M4 #6): read and
// import take no positional argument at all (every value is supplied
// via a --flag), so they have no equivalent trailing-flags failure mode
// to pin here.
func TestUsageText_DiffExport_AlreadyFlagsFirst(t *testing.T) {
	cases := []struct {
		name, text, flagsBeforeFile string
	}{
		{"diff", diffUsageText, "--port <path> [--model NAME] <file>"},
		{"diff", diffUsageText, "--fake [--model NAME] <file>"},
		{"export", exportUsageText, "--csv OUT [--force] FILE"},
	}
	for _, tc := range cases {
		if !strings.Contains(tc.text, tc.flagsBeforeFile) {
			t.Errorf("%s usage text = %q, want it to contain the flags-first synopsis fragment %q", tc.name, tc.text, tc.flagsBeforeFile)
		}
	}
}

// TestPrintUsage_RegistryDriven pins task 40's neutralisation of the
// top-level description: it is built from wiring.SupportedModels() at
// print time, not a hand-written "FT-710" literal baked into the const —
// so it names every currently-supported model, in wiring.SupportedModels'
// own sorted order, and says "radios" (plural, model-neutral) rather than
// naming one radio outright.
//
// "Yaesu and Icom radios", not "Yaesu radios", since Wave 4 task R1
// registered the IC-7610: the manufacturer word itself is a hand-written
// literal too, exactly like the "FT-710" this doc comment already
// describes replacing, and it was rewritten deliberately rather than left
// to quietly misdescribe a mixed registry. This pin is what makes that
// rewrite a visible, intentional edit rather than a drift.
func TestPrintUsage_RegistryDriven(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()

	if !strings.Contains(out, "Yaesu and Icom radios") {
		t.Errorf("printUsage output = %q, want it to say \"Yaesu and Icom radios\" (manufacturer-neutral across both registered families)", out)
	}
	for _, model := range wiring.SupportedModels() {
		if !strings.Contains(out, model) {
			t.Errorf("printUsage output = %q, want it to name supported model %q", out, model)
		}
	}
}

// TestUsageTexts_ModelFlagDocumented pins task 40's "document --model on
// every command that has it" requirement: probe/read/write/diff/import/
// settings each mention --model in their own usage text; ports/export
// (which never accept it) do not.
func TestUsageTexts_ModelFlagDocumented(t *testing.T) {
	withModel := map[string]string{
		"probe":    probeUsageText,
		"read":     readUsageText,
		"write":    writeUsageText,
		"diff":     diffUsageText,
		"import":   importUsageText,
		"settings": settingsUsageText,
	}
	for name, text := range withModel {
		if !strings.Contains(text, "--model") {
			t.Errorf("%s usage text = %q, want it to document --model", name, text)
		}
	}

	withoutModel := map[string]string{
		"ports":  portsUsageText,
		"export": exportUsageText,
	}
	for name, text := range withoutModel {
		if strings.Contains(text, "--model") {
			t.Errorf("%s usage text = %q, want it NOT to mention --model (this command never accepts it)", name, text)
		}
	}
}

// TestUsageTexts_NoHardcodedFT710Mentions pins task 40's neutral-rewording
// requirement for ports and settings (the "other FT-710 mentions" the
// brief names): both used to name "FT-710" directly in prose that had
// nothing to do with the --model flag/default — that prose is now
// model-neutral. (probe/read/write/diff/import's own usage texts are
// free to say "FT-710" ONLY inside their "--model NAME ... (default:
// FT-710)" flag descriptions, which this test does not forbid.)
func TestUsageTexts_NoHardcodedFT710Mentions(t *testing.T) {
	if strings.Contains(portsUsageText, "FT-710") {
		t.Errorf("portsUsageText = %q, want no hardcoded FT-710 mention (ports takes no --model at all)", portsUsageText)
	}
	for _, line := range strings.Split(settingsUsageText, "\n") {
		if strings.Contains(line, "FT-710") && !strings.Contains(line, "--model") {
			t.Errorf("settingsUsageText line = %q, want any FT-710 mention confined to the --model flag's own description", line)
		}
	}
}
