# M9b Codec Dialect Seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Revision 2 (25/07/2026).** Revision 1 was reviewed by Codex before execution and returned NEEDS-REVISION with 8 HIGH, 2 MEDIUM and 1 LOW finding. All eleven were verified against the source and accepted; the adjudication table is at the end of this document and the transcript is at `.superpowers/sdd/m9b-plan-codex-review.md`. **Do not execute revision 1** (git `0b72cec`).

**Goal:** Turn `core/cat` from a single FT-710-flavoured codec into a dialect-parameterised one, and move the outbound allowlist out of `core/transport` into an injected, fail-closed gate — so that adding the FTdx10 in M9c is *write a table and register it*, not *fork the codec*.

**Architecture:** An exported `cat.Dialect` struct carries the four things that vary across the classic NEWCAT family: mode-nibble set, EX inventory with its own membership index, slot-space rules, and CAT ID. One configured instance, `cat.FT710`. Every grammar entry point *and every helper it delegates to* consults the receiver. `core/transport` gains `type AllowFunc func([]byte) bool` and takes it at construction; `Dialect.AllowedCommand` matches that signature exactly.

**Tech Stack:** Go 1.25 (stdlib only in `core/` — no new dependencies), `go/parser` and `go/ast` for the guard tests, existing table-driven test style.

**Design spec:** `docs/superpowers/specs/2026-07-25-m9b-codec-dialect-seam-design.md`. Read it before starting. Where this plan and the spec disagree, the spec wins and the plan is wrong — say so rather than guessing.

## The one idea this milestone turns on

**The receiver must be load-bearing, not cosmetic.** A method that takes a `Dialect` and then consults a package-level global has the *shape* of a seam and none of the substance — and every existing test still passes, because the one configured dialect and the globals hold identical data. Revision 1 shipped exactly that mistake in three places (Codex findings 3, 5 and 6).

So the milestone's central verification is not a golden file. It is **Task 57: a second, deliberately different dialect that must produce different answers.** If a helper is still hardwired to `FT710`, that test fails and nothing else will. Every earlier task exists to make it possible; the corpora in Task 51 are supporting checks, not the main one.

## Global Constraints

Every task's requirements implicitly include all of these.

- **stdlib only in `core/`.** No new module dependencies anywhere in this milestone.
- **SPDX header** on every new file: `// SPDX-License-Identifier: GPL-3.0-or-later`, blank line, `package …`.
- **British English** in all user-facing copy and comments (`-ise`, `-our`, `behaviour`).
- **The word "snapshot", never "backup"** for saved radio contents.
- **`internal/fakeradio` imports nothing from `core/`.** `TestNoCoreImports` enforces this and stays green and untouched. The fake's Table 2 transcription is an *independent* second source; if it ever read from `cat.Dialect`, `core/transport/ex_crosscheck_test.go` would compare a value with itself. If that test goes red, stop and escalate — a correctness-of-evidence failure, not a build error.
- **Expected byte literals in evidence tests never change.** The G1–G12 manual-derived golden vectors, the M5a/M5b hardware-derived vectors, and the EX observed-CSV pins keep their expected values character-for-character. Only *invocations* change. If a Task 51 corpus fails, stop and escalate rather than regenerating it.
- **No menu-write path.** `Dialect.AllowedCommand` must continue to reject every EX Set/Answer-shaped frame — shipped policy, not a phase restriction (`docs/menu-write-decision.md`). Do not "improve" it.
- **Every task ends with `go test ./...` green.** The ordering below is arranged so this is achievable; if you find a task where it is not, stop and say so rather than leaving a red tree for the next implementer.
- **CI is billing-dead.** The full local gate in Task 59 substitutes. Do not add or edit workflow files.
- **Branch:** `m9b-dialect-seam`, cut from `main`. Merge at milestone end with `--no-ff`.

## Task ordering, and why it is this order

| # | Task | Why here |
| --- | --- | --- |
| 51 | Baselines and corpora | A pin minted after the refactor records whatever the refactor produced. Must be first. |
| 52 | Guard matcher amendment | **Moved earlier (Codex F4).** The name-only matcher handles both the old and new call shapes, so amending it *before* the churn is what keeps Tasks 54–56 green. |
| 53 | `Dialect` with real data | Slot space, EX membership index, modes, CAT ID — as data, with zero-value semantics defined. |
| 54 | Convert the helper chain | **The load-bearing task (F3, F5).** Every helper a grammar entry point delegates to must consult the receiver. |
| 55 | Migrate call sites | Mechanical, once 53–54 are right. |
| 56 | Inject the gate; plumb the driver | Fail-closed construction; dialect lives on `ft710Driver` (F7). |
| 57 | **The second-dialect proof** | The test that makes the seam real rather than cosmetic. |
| 58 | `NewEngine` guard; retire the duplicate | Needs 56 to exist first. |
| 59 | Docs, byte-identity comparison, gate | Compares against Task 51's baselines. |

---

### Task 51: Baselines and corpora

**Files:**
- Create: `core/cat/framecorpus_test.go`, `core/cat/testdata/frame-corpus.golden`
- Create: `core/cat/parsercorpus_test.go`, `core/cat/testdata/parser-corpus.golden`
- Create: `core/cat/evidence_literals_test.go`, `core/cat/testdata/evidence-literals.golden`
- Create: `.superpowers/sdd/m9b-baselines/` (git-ignored; CLI and codeplug baselines)

**Interfaces:** Consumes nothing. Produces three golden files and a baseline directory every later task must leave intact.

**Why three corpora, not one.** The frame corpus covers builders. Codex F2/F3 established that this leaves *parsers* and *error paths* unguarded — and the receiver-hardwiring bug in F3 lives precisely there, because builders and parsers share the membership helpers. The parser corpus closes it.

- [ ] **Step 1: Write the frame corpus**

Create `core/cat/framecorpus_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const frameCorpusPath = "testdata/frame-corpus.golden"

// corpusSlot pairs a stable label with a Slot, so the golden file reads
// as documentation rather than as opaque wire bytes.
type corpusSlot struct {
	label string
	slot  Slot
}

// corpusSlots spans every slot kind the codec knows, at its boundaries.
//
// NOTE on SixtyMSlot: it takes a 60m channel ORDINAL (1-99) and formats
// it as wire "501".."599" — see core/cat/slot.go. Passing 501 is an
// error. Revision 1 of this plan got that wrong (Codex F8).
func corpusSlots(t *testing.T) []corpusSlot {
	t.Helper()
	must := func(s Slot, err error) Slot {
		t.Helper()
		if err != nil {
			t.Fatalf("corpus slot construction failed: %v", err)
		}
		return s
	}
	return []corpusSlot{
		{"mem001", must(MemorySlot(1))},
		{"mem050", must(MemorySlot(50))},
		{"mem099", must(MemorySlot(99))},
		{"pms1L", must(PMSSlot(1, false))},
		{"pms9U", must(PMSSlot(9, true))},
		{"sixty501", must(SixtyMSlot(1))},
		{"sixty599", must(SixtyMSlot(99))},
		{"emg", EMGSlot()},
	}
}

// corpusMemoryData returns a fixed MemoryData for slot s. Every field is
// a constant so the frame depends only on the slot. CTCSS and Shift are
// set explicitly because their zero values are not legal wire bytes.
func corpusMemoryData(s Slot) MemoryData {
	return MemoryData{
		Slot:   s,
		FreqHz: 14_250_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeUSB,
		// KindMemory ('1') for memory and PMS slots alike — the pairing
		// rule the M5b write trials confirmed on hardware.
		Kind:  KindMemory,
		CTCSS: CTCSSOff,
		Shift: ShiftSimplex,
	}
}

// corpusLine is one parsed golden line.
type corpusLine struct {
	label    string
	frame    string
	rejected bool
}

// splitCorpusLine parses a line buildFrameCorpus emitted. Task 57 uses it
// to feed built frames to a zero dialect.
func splitCorpusLine(line string) corpusLine {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return corpusLine{label: line, rejected: true}
	}
	if strings.HasPrefix(parts[1], "REJECTED: ") {
		return corpusLine{label: parts[0], rejected: true}
	}
	return corpusLine{label: parts[0], frame: parts[1]}
}

// recordOrReject records either the built frame or the exact rejection.
// A builder that STOPS rejecting something is as much a regression as one
// whose bytes change, and revision 1's "skip on error" hid exactly that.
func recordOrReject(t *testing.T, out *[]string, label string, build func() (Command, error)) {
	t.Helper()
	c, err := build()
	if err != nil {
		*out = append(*out, label+"\tREJECTED: "+err.Error())
		return
	}
	*out = append(*out, label+"\t"+string(c.Bytes()))
}

// buildFrameCorpus drives every frame-producing builder over a fixed
// input set. Keep the inputs stable: appending a case is fine (and needs
// a regenerate), changing one destroys the comparison this exists for.
func buildFrameCorpus(t *testing.T) []string {
	t.Helper()

	var out []string
	emit := func(label, frame string) { out = append(out, label+"\t"+frame) }

	emit("ID.read", string(BuildIDRead().Bytes()))
	emit("AI.set.on", string(BuildAISet(true).Bytes()))
	emit("AI.set.off", string(BuildAISet(false).Bytes()))
	emit("MC.read", string(BuildMCRead().Bytes()))

	for _, sc := range corpusSlots(t) {
		recordOrReject(t, &out, "MR.read."+sc.label, func() (Command, error) { return BuildMRRead(sc.slot) })
		recordOrReject(t, &out, "MT.read."+sc.label, func() (Command, error) { return BuildMTRead(sc.slot) })
		recordOrReject(t, &out, "MC.set."+sc.label, func() (Command, error) { return BuildMCSet(sc.slot) })
		recordOrReject(t, &out, "MT.set.tag."+sc.label, func() (Command, error) { return BuildMTSet(sc.slot, true, "TAG") })
		recordOrReject(t, &out, "MT.set.clear."+sc.label, func() (Command, error) { return BuildMTSet(sc.slot, false, "") })
		recordOrReject(t, &out, "MW.set."+sc.label, func() (Command, error) { return BuildMWSet(corpusMemoryData(sc.slot)) })
	}

	for _, a := range EXAddresses() {
		recordOrReject(t, &out, "EX.read."+a.Wire(), func() (Command, error) { return BuildEXRead(a) })
	}

	return out
}

// TestFrameCorpus_MatchesGolden is the builder half of the byte-identity
// pin. A failure means a builder's output or its rejection changed, which
// during a call-site-only refactor is a bug. Do not regenerate.
func TestFrameCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, frameCorpusPath, strings.Join(buildFrameCorpus(t), "\n")+"\n")
}

// assertGolden compares got against the committed file at path and
// reports the first diverging line. Shared with the parser corpus.
func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("%s diverged at line %d:\n  golden: %q\n  now:    %q\n\nThis is a behaviour change, not a diff to accept.", path, i+1, wantLines[i], gotLines[i])
		}
	}
	t.Fatalf("%s length differs: golden %d lines, now %d lines", path, len(wantLines), len(gotLines))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./core/cat/ -run TestFrameCorpus_MatchesGolden -v`
Expected: FAIL — `reading testdata/frame-corpus.golden: … no such file or directory`.

If it fails to *compile*, a name in `corpusMemoryData` is wrong. Read `core/cat/memdata.go` and use the real field and constant names. Do not invent fields.

- [ ] **Step 3: Write the parser corpus**

Create `core/cat/parsercorpus_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
	"testing"
)

const parserCorpusPath = "testdata/parser-corpus.golden"

// parserInputs is a fixed set of frames spanning every parser, valid and
// invalid. The invalid ones matter as much as the valid: a receiver
// hardwired to the wrong data changes which inputs are REJECTED long
// before it changes a successful parse.
func parserInputs() []struct{ label, frame string } {
	return []struct{ label, frame string }{
		{"ID.valid", "ID0800;"},
		{"ID.other", "ID0761;"},
		{"ID.short", "ID080;"},
		{"AI.on", "AI1;"},
		{"AI.off", "AI0;"},
		{"AI.garbage", "AI2;"},
		{"MC.mem", "MC001;"},
		{"MC.pms", "MCP1L;"},
		{"MC.sixty", "MC501;"},
		{"MC.emg", "MCEMG;"},
		{"MC.outofspace", "MC100;"},
		{"MT.short", "MT0011TAG;"},
		{"MT.emptytag", "MT0011;"},
		{"MT.badslot", "MT1001TAG;"},
		{"MR.badlen", "MR001;"},
		{"EX.valid", "EX010101000;"},
		{"EX.nonmember", "EX050101000;"},
	}
}

// buildParserCorpus records what every parser returns for every input —
// the value on success, the error text on failure. Error strings are part
// of the contract: they are what a user sees.
func buildParserCorpus(t *testing.T) []string {
	t.Helper()
	var out []string

	for _, in := range parserInputs() {
		f := []byte(in.frame)

		id, err := ParseIDAnswer(f)
		out = append(out, record("ParseIDAnswer."+in.label, id, err))

		on, err := ParseAIAnswer(f)
		out = append(out, record("ParseAIAnswer."+in.label, fmt.Sprintf("%v", on), err))

		slot, err := ParseMCAnswer(f)
		out = append(out, record("ParseMCAnswer."+in.label, slotWire(slot), err))

		s, disp, tag, err := ParseMTAnswer(f)
		out = append(out, record("ParseMTAnswer."+in.label, fmt.Sprintf("%s|%v|%q", slotWire(s), disp, tag), err))

		md, err := ParseMRAnswer(f)
		out = append(out, record("ParseMRAnswer."+in.label, fmt.Sprintf("%s|%d|%c|%c", slotWire(md.Slot), md.FreqHz, md.Mode.Wire(), md.Kind), err))

		addr, raw, err := ParseEXAnswer(f)
		out = append(out, record("ParseEXAnswer."+in.label, fmt.Sprintf("%s|%q", addr.Wire(), raw), err))
	}

	// A real MR answer, taken from the existing golden vectors rather
	// than invented — read core/cat/mr_test.go for G4/G6 and use one of
	// those frames verbatim here.
	//
	// IMPLEMENTER: replace the placeholder below with the actual G6 frame
	// string from mr_test.go. Do not construct one by hand.
	for _, gv := range goldenMRFramesForCorpus() {
		md, err := ParseMRAnswer([]byte(gv.frame))
		out = append(out, record("ParseMRAnswer.golden."+gv.label,
			fmt.Sprintf("%s|%d|%c|%c|%d", slotWire(md.Slot), md.FreqHz, md.Mode.Wire(), md.Kind, md.ClarHz), err))
	}

	// Membership rules, most at risk of being silently hardwired.
	for _, w := range []string{"001", "099", "100", "P1L", "P9U", "P0L", "501", "599", "600", "EMG", "000", "00001", "abc"} {
		s, err := ParseSlot(w)
		out = append(out, record("ParseSlot."+w, slotWire(s), err))
	}
	for _, c := range []byte{'0', '1', '9', 'A', 'F', 'G', 'a', '!'} {
		m, err := ParseMode(c)
		out = append(out, record(fmt.Sprintf("ParseMode.%c", c), string(m.Wire()), err))
	}
	for _, w := range []string{"010101", "010321", "050101", "999999", "01010"} {
		a, err := ParseEXAddress(w)
		out = append(out, record("ParseEXAddress."+w, a.Wire(), err))
	}

	return out
}

// goldenMRFramesForCorpus returns real 28-byte MR answer frames copied
// verbatim from this package's existing golden vectors.
//
// IMPLEMENTER: populate from core/cat/mr_test.go's G4/G6/G7 vectors.
// Copy the frame strings character-for-character — they are hardware- and
// manual-derived evidence, and the literal pin will catch a retyped one.
func goldenMRFramesForCorpus() []struct{ label, frame string } {
	return []struct{ label, frame string }{
		// {"G4", "MR..."},
		// {"G6", "MR..."},
	}
}

// record renders one parser outcome as a single golden line.
func record(label, value string, err error) string {
	if err != nil {
		return label + "\tERR: " + err.Error()
	}
	return label + "\tOK: " + value
}

// slotWire renders a Slot safely, including the zero value.
func slotWire(s Slot) string {
	if s.Wire() == "" {
		return "<zero>"
	}
	return s.Wire()
}

// TestParserCorpus_MatchesGolden is the parser half of the pin — the
// check that catches a helper still consulting FT710 through a Dialect
// receiver. The frame corpus cannot see that, because builders and
// parsers share the membership helpers (Codex F3).
func TestParserCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, parserCorpusPath, strings.Join(buildParserCorpus(t), "\n")+"\n")
}
```

**`goldenMRFramesForCorpus` is the one place this plan leaves you to fetch a value.** It is deliberate: the MR golden frames are hardware- and manual-derived evidence, and a frame typed from memory into a plan would be exactly the corruption everything here exists to prevent. Open `core/cat/mr_test.go`, find the G4/G6/G7 vectors, and copy them verbatim. If any signature above does not match reality, correct the *test*, never the production code.

- [ ] **Step 4: Write the evidence-literal pin as ordered per-file records**

Revision 1 stored a flat set. Codex F1 showed that cannot round-trip a multiline raw string — `core/cat/exobserved_test.go`'s `observedHeader` is one — and F2 showed a set loses attachment: a literal deleted from `id_test.go` passes if the same spelling appears anywhere else, and Task 53 itself introduces `"0800"`.

Create `core/cat/evidence_literals_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const evidenceLiteralsPath = "testdata/evidence-literals.golden"

// literalRecord is one string literal identified by WHERE it is and WHICH
// occurrence it is — not merely by its spelling. That is what lets the
// pin detect a literal being moved, orphaned or deleted while an
// identical spelling survives elsewhere.
type literalRecord struct {
	file    string
	ordinal int
	token   string // strconv.Quote'd, so every record is exactly one line
}

func (r literalRecord) String() string {
	return fmt.Sprintf("%s\t%d\t%s", r.file, r.ordinal, r.token)
}

// collectTestStringLiterals walks this package's evidence test files in a
// stable order, recording every string literal with its file and ordinal.
func collectTestStringLiterals(t *testing.T) []literalRecord {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, "_test.go") {
			continue
		}
		// The pin's own tooling is not evidence; its literals would churn
		// as the tooling evolves.
		switch n {
		case "evidence_literals_test.go", "framecorpus_test.go", "parsercorpus_test.go",
			"dialect_test.go", "seconddialect_test.go":
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)

	if len(names) < 10 {
		t.Fatalf("only found %d evidence test files — the walker or its filter is broken, and this check would pass vacuously", len(names))
	}

	var out []literalRecord
	fset := token.NewFileSet()
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		n := 0
		ast.Inspect(f, func(node ast.Node) bool {
			bl, ok := node.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			// bl.Value is the raw source token including its quotes or
			// backticks. Unquote then re-quote, so a backtick and a
			// double-quoted form of the same value compare equal and
			// every record occupies one line.
			val, err := strconv.Unquote(bl.Value)
			if err != nil {
				val = bl.Value
			}
			out = append(out, literalRecord{file: name, ordinal: n, token: strconv.Quote(val)})
			n++
			return true
		})
	}
	return out
}

// TestEvidenceLiterals_OrderedRecordsSurvive asserts every pre-M9b
// literal is still in the same file at the same ordinal.
//
// If this fails because a task legitimately ADDED a literal mid-file,
// that is a conversation to have, not a golden file to regenerate.
func TestEvidenceLiterals_OrderedRecordsSurvive(t *testing.T) {
	current := collectTestStringLiterals(t)

	raw, err := os.ReadFile(filepath.FromSlash(evidenceLiteralsPath))
	if err != nil {
		t.Fatalf("reading %s: %v", evidenceLiteralsPath, err)
	}

	byKey := map[string]string{}
	for _, r := range current {
		byKey[fmt.Sprintf("%s\t%d", r.file, r.ordinal)] = r.token
	}

	var problems []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			t.Fatalf("malformed golden line %q", line)
		}
		key := parts[0] + "\t" + parts[1]
		got, present := byKey[key]
		switch {
		case !present:
			problems = append(problems, fmt.Sprintf("%s: literal #%s is gone (was %s)", parts[0], parts[1], parts[2]))
		case got != parts[2]:
			problems = append(problems, fmt.Sprintf("%s: literal #%s changed\n    was: %s\n    now: %s", parts[0], parts[1], parts[2], got))
		}
	}

	if len(problems) > 0 {
		shown := problems
		if len(shown) > 15 {
			shown = shown[:15]
		}
		t.Fatalf("%d expected literal(s) changed or vanished:\n  %s\n\nA call-site rewrite must not touch expected VALUES. Do NOT regenerate the golden file.",
			len(problems), strings.Join(shown, "\n  "))
	}
}
```

- [ ] **Step 5: Generate all three golden files, then test them**

```bash
mkdir -p core/cat/testdata
cat >> core/cat/framecorpus_test.go <<'EOF'

// TestGenerateCorpora is a one-shot generator. DELETE after committing —
// a pin that can regenerate itself on demand is not a pin.
func TestGenerateCorpora(t *testing.T) {
	if os.Getenv("GENERATE_M9B_CORPORA") == "" {
		t.Skip("set GENERATE_M9B_CORPORA=1 to regenerate")
	}
	write := func(path, body string) {
		if err := os.WriteFile(filepath.FromSlash(path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(frameCorpusPath, strings.Join(buildFrameCorpus(t), "\n")+"\n")
	write(parserCorpusPath, strings.Join(buildParserCorpus(t), "\n")+"\n")

	var lits []string
	for _, r := range collectTestStringLiterals(t) {
		lits = append(lits, r.String())
	}
	write(evidenceLiteralsPath, strings.Join(lits, "\n")+"\n")
}
EOF
GENERATE_M9B_CORPORA=1 go test ./core/cat/ -run TestGenerateCorpora -v
```

Delete `TestGenerateCorpora`, then:

Run: `go test ./core/cat/ -run 'TestFrameCorpus|TestParserCorpus|TestEvidenceLiterals' -v`
Expected: all three PASS.

**This step is Codex F1's remedy: test the freshly generated artefact before committing it.** If the literal pin fails immediately, the encoding is still wrong — fix it now.

Eyeball the golden files: `frame-corpus.golden` should hold ~330 lines with EX entries like `EX.read.010101	EX010101;`; no line in any of the three should contain anything resembling a real callsign or a personal frequency.

- [ ] **Step 6: Prove each corpus bites**

A pin never seen to fail is not a pin. For each, break something, observe, revert:

1. Change `BuildIDRead` to emit `"ID ;"` → `TestFrameCorpus_MatchesGolden` must fail naming the line. `git checkout core/cat/id.go`.
2. Change `ParseMode` to accept `'G'` → `TestParserCorpus_MatchesGolden` must fail on `ParseMode.G`. Revert.
3. Change any expected string in `core/cat/mt_test.go` by one character → `TestEvidenceLiterals_OrderedRecordsSurvive` must fail naming that file and ordinal. Revert.

Record all three failure messages in the task report.

- [ ] **Step 7: Capture the CLI and codeplug baselines**

Codex F10: the design's byte-identity scope covers codeplug JSON, `Digest`, schema and CLI output, and revision 1 compared only `probe --fake`.

```bash
mkdir -p .superpowers/sdd/m9b-baselines
B=.superpowers/sdd/m9b-baselines
go run ./cmd/rigprog probe --fake > "$B/probe-fake.txt" 2>&1
go run ./cmd/rigprog read --fake --settings --out "$B/read-fake.json" > "$B/read-fake.txt" 2>&1
go run ./cmd/rigprog settings "$B/read-fake.json" > "$B/settings.txt" 2>&1
go run ./cmd/rigprog export --csv "$B/export.csv" "$B/read-fake.json" > "$B/export.txt" 2>&1
go run ./cmd/rigprog help > "$B/help.txt" 2>&1
git rev-parse HEAD > "$B/BASELINE-COMMIT"
```

`.superpowers/` is git-ignored, so these stay local — fine, they are a within-milestone comparison. Note in the report that `read-fake.json` carries the codeplug JSON, its `Digest` and its schema number, all three of which Task 59 compares.

- [ ] **Step 8: Full suite and commit**

Run: `gofmt -l . && go vet ./... && go test ./...`

```bash
git add core/cat/framecorpus_test.go core/cat/parsercorpus_test.go core/cat/evidence_literals_test.go core/cat/testdata/
git commit -m "m9b: task 51 — mint three evidence corpora before anything moves

Frame corpus (builders, including their REJECTIONS — a builder that stops
rejecting is as much a regression as one whose bytes change), parser
corpus (return values AND error text, the half revision 1 lacked), and an
ordered per-file literal inventory.

The literal pin records (file, ordinal, quoted token), not a flat set: a
set cannot round-trip a multiline raw string, and cannot tell a deleted
literal from one whose spelling survives in another file.

All three were verified to fail when deliberately broken."
```

---

### Task 52: Amend the guard matcher first

**Files:** Modify `internal/guards/importgraph_test.go` — **the byte-identically-pinned file; this is the formal amendment.** Modify `.superpowers/sdd/progress.md`.

**Why this is now task 52 and not 56.** Codex F4: the moment builders become methods, `cat.FT710.BuildMWSet` is a nested selector and the current matcher (`importgraph_test.go:251-255`, which requires `sel.X` to be an `*ast.Ident`) stops seeing it — so `go test ./...` fails from Task 54 onward and two tasks land red. The name-only matcher recognises **both** shapes, so amending it now costs nothing and keeps every later task green.

- [ ] **Step 1: Confirm the guard currently passes**

Run: `go test ./internal/guards/ -v`
Expected: PASS. Record the test-function count per file (`grep -c "^func Test" internal/guards/*_test.go`) — there are five in total, and Task 58 checks that number again.

- [ ] **Step 2: Rewrite the builder matcher to match by method name**

In `internal/guards/importgraph_test.go`, replace the package-qualified block at ~251-255:

```go
		// (a) BuildMWSet / BuildMTSet, matched by NAME alone, whatever
		// the receiver.
		//
		// Amended at M9b. Before the dialect seam these were
		// package-level functions and this check required sel.X to be an
		// *ast.Ident naming the core/cat import. The seam makes every
		// call a nested selector — cat.FT710.BuildMWSet, or
		// s.dialect.BuildMWSet — which that form silently stops
		// matching. Amended AHEAD of the migration precisely so no task
		// lands on a red tree; this form recognises both shapes.
		//
		// Name-only is LOOSER but strictly MORE INCLUSIVE: every call the
		// old form caught, this one catches. It is also the approximation
		// this guard already applies to WriteChannel in (b) below, so it
		// is house precedent rather than a new compromise.
		ast.Inspect(pf.file, func(n ast.Node) bool {
			sel, isSel := n.(*ast.SelectorExpr)
			if !isSel {
				return true
			}
			if sel.Sel.Name != "BuildMWSet" && sel.Sel.Name != "BuildMTSet" {
				return true
			}
			if inTree(pf.relDir, "core/driver") {
				sawDriverBuildMW = true
				return true
			}
			t.Errorf("%s: references .%s — the Set-frame builders may be used outside core/cat only from core/driver/** (composition-root discipline; see this test's doc comment)", pf.relPath, sel.Sel.Name)
			return true
		})
```

Update this test's own doc comment (the paragraph around line 194) so its description of builder matching says "by method name, whatever the receiver", matching how it already describes `WriteChannel`.

- [ ] **Step 3: Verify it passes and still bites**

Run: `go test ./internal/guards/ -v`
Expected: PASS.

Then prove non-vacuity: temporarily add `_ = cat.BuildMWSet` to a non-test file under `core/clone/`, run the guard, confirm it FAILS naming that file, revert. Record the message.

- [ ] **Step 4: Ledger the amendment**

Append to `.superpowers/sdd/progress.md`:

```
**M9b task 52 — importgraph_test.go BYTE-IDENTICAL PIN FORMALLY AMENDED.** Held from M3 to M9a; lifted here, as the M8 roadmap always intended M9b (or M8e, now cancelled) to do. Change: the Set-frame builder matcher moves from package-qualified (sel.X must be an Ident naming core/cat) to method-name-only, because the dialect seam makes every builder call a NESTED selector the old form cannot see. Done BEFORE the migration, not after, on Codex plan-review finding F4 — revision 1 had it after, which would have left tasks 54 and 55 landing on a red tree. SEMANTICS NOT WEAKENED: name-only is strictly MORE inclusive, so it cannot admit a call the old form caught, and it is the same approximation the guard already applies to WriteChannel; Codex independently confirmed it found no call shape the new form misses. Verified to still bite by introducing a violation in core/clone. New baseline for any future byte-identity check: initial commit ed728b9 (38b3087 no longer exists after the 25/07/2026 history reset).
```

- [ ] **Step 5: Commit**

```bash
git add internal/guards/importgraph_test.go .superpowers/sdd/progress.md
git commit -m "m9b: task 52 — amend the write-path guard matcher ahead of the migration

The dialect seam turns every builder call into a nested selector the
package-qualified matcher cannot see. Amending FIRST, not last: the
name-only form recognises both old and new shapes, so no later task has
to land on a red tree.

Strictly more inclusive than what it replaces, and the same approximation
already applied to WriteChannel. Formally amends the byte-identical pin
held since M3; ledgered with the not-weakened argument."
```

---

### Task 53: `Dialect` carrying real data

**Files:** Create `core/cat/dialect.go`, `core/cat/dialect_test.go`. Modify `core/cat/slot.go`, `core/cat/mode.go`, `core/cat/exinventory.go`.

**Interfaces produced:**
- `type Dialect struct` (unexported fields), `var FT710 Dialect`
- `func (d Dialect) Configured() bool`, `CATID() string`
- `func (d Dialect) ValidMode(m Mode) bool`, `ModeName(m Mode) string`
- `func (d Dialect) EXItems() []EXItem`, `EXAddresses() []EXAddress`, `KnownEXAddress(a EXAddress) bool`
- `func (d Dialect) ParseSlot(wire string) (Slot, error)` and the slot constructors
- unexported: `func (d Dialect) classifySlot(wire string) slotKind`

**This task fixes Codex F5 and F6.** The dialect carries slot-space rules and its own EX membership index as *data*, not as methods over package globals. Revision 1 deferred the slot rules, which was an unapproved third scope departure and would have left the receiver cosmetic for every slot-validating path.

- [ ] **Step 1: Write the failing tests, including zero-value semantics**

Create `core/cat/dialect_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// TestFT710Dialect_CarriesTheRadioSpecificData pins what the dialect is
// FOR: the things that vary across the classic NEWCAT family.
func TestFT710Dialect_CarriesTheRadioSpecificData(t *testing.T) {
	if !FT710.Configured() {
		t.Fatal("FT710.Configured() = false — the package's own dialect must be configured")
	}
	if got := FT710.CATID(); got != "0800" {
		t.Errorf("FT710.CATID() = %q, want %q", got, "0800")
	}
	if !FT710.ValidMode(ModeUSB) {
		t.Error("FT710.ValidMode(ModeUSB) = false, want true")
	}
	if got := FT710.ModeName(ModeDATAFMN); got != "DATA-FM-N" {
		t.Errorf("FT710.ModeName(ModeDATAFMN) = %q, want %q", got, "DATA-FM-N")
	}
	if n := len(FT710.EXItems()); n != 296 {
		t.Errorf("len(FT710.EXItems()) = %d, want 296", n)
	}
}

// TestZeroDialect_IsUnconfiguredAndKnowsNothing is the fail-closed
// property at the codec layer.
//
// Codex plan-review F6: an exported struct always has a constructible
// zero value, unexported fields or not. `var d cat.Dialect` compiles, and
// d.AllowedCommand is a non-nil method value that would satisfy
// transport.NewEngine's nil check. So the zero dialect must be INERT by
// construction — no slot space, no modes, no inventory — and therefore
// able neither to build nor to accept anything.
func TestZeroDialect_IsUnconfiguredAndKnowsNothing(t *testing.T) {
	var d Dialect

	if d.Configured() {
		t.Error("zero Dialect reports Configured() = true")
	}
	if d.CATID() != "" {
		t.Errorf("zero Dialect CATID() = %q, want empty", d.CATID())
	}
	if d.ValidMode(ModeUSB) {
		t.Error("zero Dialect claims to know ModeUSB")
	}
	if len(d.EXItems()) != 0 {
		t.Error("zero Dialect returned EX items")
	}
	for _, w := range []string{"001", "P1L", "501", "EMG", "000"} {
		if _, err := d.ParseSlot(w); err == nil {
			t.Errorf("zero Dialect parsed slot %q — it has no slot space", w)
		}
	}
}

// TestFT710Dialect_SlotSpaceIsDialectData pins slot classification as
// something the dialect OWNS. The FTX-1's 5-digit slots are the eventual
// forcing case; the classic family is 3-digit.
func TestFT710Dialect_SlotSpaceIsDialectData(t *testing.T) {
	cases := []struct {
		wire string
		want bool
	}{
		{"001", true},
		{"099", true},
		{"P1L", true},
		{"P9U", true},
		{"501", true},
		{"EMG", true},
		{"000", true},
		{"100", false},
		{"00001", false},
		{"abc", false},
	}
	for _, tc := range cases {
		_, err := FT710.ParseSlot(tc.wire)
		if (err == nil) != tc.want {
			t.Errorf("FT710.ParseSlot(%q): err == nil is %v, want %v", tc.wire, err == nil, tc.want)
		}
	}
}

// TestFT710Dialect_EXMembershipIsPerDialect pins that membership consults
// the dialect's own index rather than a package global. Task 57's second
// dialect proves it beyond doubt; this is the cheap first check.
func TestFT710Dialect_EXMembershipIsPerDialect(t *testing.T) {
	addrs := FT710.EXAddresses()
	if len(addrs) == 0 {
		t.Fatal("FT710.EXAddresses() is empty")
	}
	if !FT710.KnownEXAddress(addrs[0]) {
		t.Errorf("FT710.KnownEXAddress(%s) = false for its own first address", addrs[0].Wire())
	}

	var zero Dialect
	if zero.KnownEXAddress(addrs[0]) {
		t.Errorf("zero Dialect claims to know EX address %s — membership is reading a package global, not the receiver", addrs[0].Wire())
	}
}

// TestFT710Dialect_EXItemsReturnsFreshCopies mirrors the existing
// TestEXItems_ReturnsFreshCopies guarantee.
func TestFT710Dialect_EXItemsReturnsFreshCopies(t *testing.T) {
	first := FT710.EXItems()
	if len(first) == 0 {
		t.Fatal("FT710.EXItems() returned nothing")
	}
	original := first[0]
	first[0].Name = "MUTATED"

	second := FT710.EXItems()
	if second[0].Name == "MUTATED" {
		t.Error("FT710.EXItems() shares backing storage between calls")
	}
	if second[0] != original {
		t.Errorf("FT710.EXItems()[0] = %+v, want %+v", second[0], original)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./core/cat/ -run 'TestFT710Dialect|TestZeroDialect' -v`
Expected: FAIL — `undefined: FT710`.

- [ ] **Step 3: Create the dialect with real data**

Create `core/cat/dialect.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// slotSpace describes one radio family's memory slot numbering: which
// 3-byte wire forms exist and what each means. DATA, not code, so a
// second dialect is a different table rather than a different function.
type slotSpace struct {
	memoryLo, memoryHi int    // inclusive decimal range, e.g. 1..99
	sixtyLo, sixtyHi   int    // inclusive decimal range, e.g. 501..599; 0,0 if absent
	pmsPairs           int    // e.g. 9 -> P1L..P9U; 0 if absent
	emgWire            string // "" if this family has no emergency channel
	noneWire           string // the "VFO or MT or QMB" form, e.g. "000"
}

// Dialect is one radio family's CAT variation: everything this codec
// needs that differs between models sharing the classic NEWCAT grammar.
//
// It carries DATA, not frame shapes. A deliberate M9b scope decision
// recorded in the design document: the FTdx10/101 manuals document a
// combined ~50-byte MT record frame against the FT-710's short form, but
// that difference is unverified against hardware, and the FT-710's own MT
// is the precedent for a manual being wrong about exactly this.
// Per-command frame-shape variants are M9c's.
//
// THE RECEIVER IS LOAD-BEARING. Every method here, and every helper those
// methods delegate to, must read this struct rather than a package-level
// global. A method that takes a Dialect and consults a global has the
// shape of a seam and none of the substance, and while only one dialect
// exists no ordinary test catches it — see seconddialect_test.go, which
// is the test that does.
//
// The ZERO VALUE IS INERT, deliberately. An exported struct always has a
// constructible zero value, so `var d cat.Dialect` compiles and
// d.AllowedCommand is a non-nil method value that would satisfy
// transport.NewEngine's nil check. A zero Dialect therefore carries no
// slot space, no modes and no inventory, and consequently builds nothing
// and accepts nothing.
type Dialect struct {
	catID     string
	modeNames map[Mode]string
	slots     slotSpace

	exItems   []EXItem
	exMembers map[EXAddress]bool // this dialect's OWN membership index
}

// FT710 is the Yaesu FT-710 dialect: the only configured one that exists.
var FT710 = Dialect{
	catID:     "0800",
	modeNames: modeNames,
	slots: slotSpace{
		memoryLo: 1, memoryHi: 99,
		sixtyLo: 501, sixtyHi: 599,
		pmsPairs: 9,
		emgWire:  "EMG",
		noneWire: "000",
	},
	exItems:   exItemsGen,
	exMembers: buildEXMembers(exItemsGen),
}

// buildEXMembers indexes items for membership tests.
func buildEXMembers(items []EXItem) map[EXAddress]bool {
	m := make(map[EXAddress]bool, len(items))
	for _, it := range items {
		m[it.Addr] = true
	}
	return m
}

// Configured reports whether this Dialect carries data. False for the
// zero value; see the type's doc comment.
func (d Dialect) Configured() bool { return d.catID != "" && d.modeNames != nil }

// CATID is the four-character identity this radio answers "ID;" with.
func (d Dialect) CATID() string { return d.catID }

// ValidMode reports whether m is a mode nibble this dialect knows.
func (d Dialect) ValidMode(m Mode) bool {
	_, ok := d.modeNames[m]
	return ok
}

// ModeName returns the display name for m under this dialect, or a
// diagnostic placeholder for a Mode it does not know.
func (d Dialect) ModeName(m Mode) string {
	if name, ok := d.modeNames[m]; ok {
		return name
	}
	return unknownModeName(m)
}

// EXItems returns a fresh copy of this dialect's EX inventory: callers
// have always been free to mutate what they get back, and one caller's
// mutation must never become everyone's inventory.
func (d Dialect) EXItems() []EXItem {
	out := make([]EXItem, len(d.exItems))
	copy(out, d.exItems)
	return out
}

// EXAddresses returns this dialect's EX addresses in inventory order.
func (d Dialect) EXAddresses() []EXAddress {
	out := make([]EXAddress, len(d.exItems))
	for i, it := range d.exItems {
		out[i] = it.Addr
	}
	return out
}

// KnownEXAddress reports whether a is in THIS dialect's inventory.
// MEMBERSHIP, never a numeric range: Table 2 is sparse and its P1 groups
// are not contiguous.
func (d Dialect) KnownEXAddress(a EXAddress) bool { return d.exMembers[a] }

// classifySlot reports what kind of slot, if any, wire represents under
// this dialect's slot space. Every slot-taking method routes through it.
func (d Dialect) classifySlot(wire string) slotKind {
	if len(wire) != 3 {
		return slotKindInvalid
	}
	if d.slots.noneWire != "" && wire == d.slots.noneWire {
		return slotKindNone
	}
	if d.slots.emgWire != "" && wire == d.slots.emgWire {
		return slotKindEMG
	}

	allDigits := true
	for i := 0; i < len(wire); i++ {
		if wire[i] < '0' || wire[i] > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		n := int(wire[0]-'0')*100 + int(wire[1]-'0')*10 + int(wire[2]-'0')
		switch {
		case d.slots.memoryHi > 0 && n >= d.slots.memoryLo && n <= d.slots.memoryHi:
			return slotKindMemory
		case d.slots.sixtyHi > 0 && n >= d.slots.sixtyLo && n <= d.slots.sixtyHi:
			// ASSUMED: the reference marks 5xx numbering as unverified.
			return slotKind60m
		default:
			return slotKindInvalid
		}
	}

	if d.slots.pmsPairs > 0 &&
		wire[0] == 'P' &&
		wire[1] >= '1' && wire[1] <= byte('0'+d.slots.pmsPairs) &&
		(wire[2] == 'L' || wire[2] == 'U') {
		return slotKindPMS
	}

	return slotKindInvalid
}
```

- [ ] **Step 4: Move the slot constructors and `ParseSlot` onto the dialect**

In `core/cat/slot.go`, add dialect methods whose bodies are the existing package-level ones with the range constants replaced by `d.slots` fields, e.g.:

```go
// ParseSlot parses a 3-byte wire slot code under this dialect's slot
// space. Reference: "Slot codes (3 bytes on the wire)".
func (d Dialect) ParseSlot(wire string) (Slot, error) {
	if d.classifySlot(wire) == slotKindInvalid {
		return Slot{}, newParseError([]byte(wire), "invalid slot code")
	}
	return Slot{wire: wire}, nil
}

// MemorySlot builds the Slot for memory channel n under this dialect's
// memory range.
func (d Dialect) MemorySlot(n int) (Slot, error) {
	if n < d.slots.memoryLo || n > d.slots.memoryHi || d.slots.memoryHi == 0 {
		return Slot{}, newParseError([]byte(fmt.Sprintf("MemorySlot(%d)", n)), "memory channel out of range 1-99")
	}
	return Slot{wire: fmt.Sprintf("%03d", n)}, nil
}
```

**Copy every error string character-for-character from the existing function** — they are in the literal inventory, and a retyped one fails the pin. Do the same for `PMSSlot`, `SixtyMSlot` and `EMGSlot`. `EMGSlot` on a dialect with no `emgWire` must return the zero `Slot`.

`Slot`'s own predicates (`Wire`, `IsMemory`, `IsPMS`, `Is60m`, `IsEMG`, `IsNone`, `Writable`) stay methods on `Slot`: a `Slot` is canonical by construction. **However**, those predicates currently call `classifySlotWire`. Redefine that package-level helper as `func classifySlotWire(wire string) slotKind { return FT710.classifySlot(wire) }` and leave a comment saying the predicates are FT-710-scoped until M9c gives `Slot` a dialect tag — record it in the deferred list too.

- [ ] **Step 5: Add `unknownModeName`; keep package-level delegates**

In `core/cat/mode.go`:

```go
// unknownModeName renders a Mode no dialect recognises. Shared by
// Mode.String and Dialect.ModeName so the two cannot drift.
func unknownModeName(m Mode) string {
	return fmt.Sprintf("Mode(%#02x)", byte(m))
}
```

Leave `Mode.String()` alone for now — Task 56 handles it, because Codex F7 found it *is* on a user-visible path (`core/driver/ft710/read.go:228` renders a channel's mode with it), contrary to what revision 1 assumed.

Add delegates so nothing else changes yet — `EXItems`, `KnownEXAddress`, `EXAddresses`, `ParseSlot`, `MemorySlot`, `PMSSlot`, `SixtyMSlot`, `EMGSlot`, each a one-liner forwarding to `FT710`, each carrying a comment saying it is a migration scaffold removed in Task 55.

- [ ] **Step 6: Run the dialect tests, then everything**

Run: `go test ./core/cat/ -run 'TestFT710Dialect|TestZeroDialect' -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS, all three Task 51 corpora included — this task changed no builder output, no parser result and no expected value.

- [ ] **Step 7: Commit**

```bash
git add core/cat/
git commit -m "m9b: task 53 — cat.Dialect carrying real data

Slot space, mode set, EX inventory AND its own membership index, CAT ID.
As DATA: a second dialect is a different table, not a different function.

Revision 1 deferred the slot rules and would have mirrored the global
exMembers map, which Codex plan-review F5 correctly called an unapproved
third scope departure — it would have left the receiver cosmetic for every
slot-validating path.

The zero value is deliberately inert (F6): an exported struct always has a
constructible zero value, so var d cat.Dialect compiles and
d.AllowedCommand is a non-nil method value that would satisfy NewEngine's
nil check. A zero dialect carries nothing and so accepts nothing.

Changes no call sites: the package-level functions delegate."
```

---

### Task 54: Convert the helper chain to consult the receiver

**Files:** `core/cat/slot.go`, `mr.go`, `mw.go`, `mt.go`, `mc.go`, `ex.go`, `allowlist.go`

**This is the task the milestone turns on (Codex F3, F5).** Revision 1 converted only the exported entry points and told the implementer to copy bodies verbatim — which would have left `parseMemoryFrame` (shared by `ParseMRAnswer` *and* `AllowedCommand`'s MW check, `core/cat/mr.go:58-69`) calling `FT710.ParseSlot` and `FT710.ParseMode` from inside a method with a `Dialect` receiver. Both corpora stay green while the seam is fiction.

- [ ] **Step 1: Inventory the helper dependency graph**

Before changing anything:

```bash
cd core/cat
grep -nE "^func (classifySlotWire|readableSlot|parseMemoryFrame|validateMWFields|mtSlotValid|validMTTag|mcValid|validIDCommand|validAICommand|validMRCommand|validMWCommand|validMTCommand|validMCCommand|validEXRead)" *.go
grep -n "classifySlotWire(\|readableSlot(\|parseMemoryFrame(\|validateMWFields(\|mtSlotValid(\|mcValid(\|ParseSlot(\|ParseMode(" *.go | grep -v _test
```

Write the resulting graph into the task report. **Every function in it that consults slot space, modes or EX membership becomes a `Dialect` method.** If you find one this plan did not name, add it and say so — the list was derived by reading, not by exhaustive proof.

- [ ] **Step 2: Convert each helper to a method, leaves first**

For each: `func name(` becomes `func (d Dialect) name(`, and every internal call to another converted helper is qualified with `d.`. Example:

```go
func (d Dialect) parseMemoryFrame(frame []byte, wantPrefix string) (MemoryData, error) {
	// length / prefix / terminator checks unchanged

	slot, err := d.ParseSlot(string(frame[memSlotOffset : memSlotOffset+3]))
	if err != nil {
		return MemoryData{}, err
	}
	// ... every other membership call likewise d.-qualified ...
}
```

Copy each body verbatim apart from the receiver and the `d.` qualification. **Error strings are pinned by the literal inventory** — retyping one fails the pin, which is the point.

- [ ] **Step 3: Convert the exported entry points**

Builders, parsers and `AllowedCommand` all become `func (d Dialect) …`, calling the converted helpers through `d.`. Keep package-level delegates so the tree stays green; Task 55 removes them with their callers.

For the four that consult no dialect data — `BuildIDRead`, `BuildAISet`, `ParseIDAnswer`, `ParseAIAnswer` — add:

```go
// Takes a dialect receiver even though nothing about this frame varies by
// radio: uniform method form means M9c adds a dialect by writing a table
// rather than by re-plumbing signatures. Do not "tidy" this back to a
// package-level function.
```

Preserve `AllowedCommand`'s doc comments exactly, especially the EX paragraph recording the M8d no-go.

- [ ] **Step 4: Verify no converted helper still reads a global**

```bash
cd core/cat
grep -n "exMembers\[" *.go | grep -v dialect.go
grep -n "classifySlotWire(" *.go | grep -v _test
grep -n "FT710\." *.go | grep -v _test | grep -v "^dialect.go"
```

Every remaining hit must be one of the one-line package-level delegates or inside `dialect.go`. Anything else is a hardwired helper — convert it.

- [ ] **Step 5: Run everything**

Run: `go test ./... && go test -race ./core/...`
Expected: PASS, all three corpora included. The parser corpus is the one that matters here: it exercises paths the frame corpus cannot see.

- [ ] **Step 6: Commit**

```bash
git add core/cat/
git commit -m "m9b: task 54 — the whole helper chain consults the receiver

The task the seam turns on. Revision 1 converted only the exported entry
points and said to copy bodies verbatim, which Codex plan-review F3 showed
would leave parseMemoryFrame — shared by ParseMRAnswer AND
AllowedCommand's MW check — calling FT710.ParseSlot from inside a method
with a Dialect receiver. Both corpora would have stayed green while the
seam was fiction.

Every helper consulting slot space, modes or EX membership is now a
Dialect method. Bodies copied verbatim apart from receiver and
d.-qualification; error strings are pinned by the literal inventory, so a
retyped one fails."
```

---

### Task 55: Migrate the call sites

**Files:** every caller of the package-level codec API — `core/cat/*_test.go`, `core/driver/ft710/*.go`, `core/codeplug/`, `core/csvio/`, `core/transport/ex_crosscheck_test.go`, `app/`, `internal/`

**Scale.** Revision 1 claimed 553; Codex F11 counted roughly 220 *executable* references, the difference being declarations, comments and doc references caught by a naive grep. Establish the real number first and put it in the report:

```bash
for f in BuildMWSet BuildMTSet BuildMTRead BuildMRRead BuildMCSet BuildMCRead \
         BuildEXRead BuildIDRead BuildAISet ParseMRAnswer ParseMTAnswer \
         ParseMCAnswer ParseEXAnswer ParseIDAnswer ParseAIAnswer ParseSlot \
         MemorySlot PMSSlot SixtyMSlot EMGSlot ParseMode EXItems EXAddresses \
         KnownEXAddress ParseEXAddress NewEXAddress AllowedCommand; do
  n=$(grep -rn "\b$f(" --include="*.go" . | grep -v ":func " | wc -l | tr -d ' ')
  printf "%-16s %s\n" "$f" "$n"
done
```

- [ ] **Step 1: Migrate, file by file**

In-package (`core/cat/*_test.go`): `BuildMWSet(` becomes `FT710.BuildMWSet(`.
Out-of-package: `cat.BuildMWSet(` becomes `cat.FT710.BuildMWSet(`.

**After each file: `go test ./core/cat/ -run 'TestFrameCorpus|TestParserCorpus|TestEvidenceLiterals'`.** A failure means you changed a value, not a call. Stop and find out which.

- [ ] **Step 2: Update the corpora's own call sites**

`framecorpus_test.go` and `parsercorpus_test.go` call the package-level API. Change the calls to `FT710.`-qualified form and **nothing else** — not the inputs, not the labels, not the golden paths.

Run: `go test ./core/cat/ -run 'TestFrameCorpus|TestParserCorpus' -v`
Expected: PASS against the unchanged golden files. This is the milestone's central byte-identity claim.

- [ ] **Step 3: Delete the package-level delegates**

Remove every delegate added in Tasks 53–54, except `classifySlotWire`, which `Slot`'s own predicates still need.

Run: `go build ./... && go vet ./...`
Expected: clean. A remaining compile error is a call site you missed — fix it, do not restore the delegate.

- [ ] **Step 4: Confirm the allowlist property tests survived**

```bash
go test ./core/cat/ -run TestAllowedCommand -v
```

Expected, all passing: `TestAllowedCommand_PropertyEveryBuilderOutput`, `TestAllowedCommand_RejectsGoldenAnswerFrames`, `TestAllowedCommand_AcceptsAllowlistedSingleFrames` (holding the audited MT and MC Set/Answer-indistinguishable exceptions), `TestAllowedCommand_EXAnswersRejectedOutboundAll296`, and both `HWDerived_M5b` pairs. Count before and after; put both numbers in the report. If one has *vanished* rather than been converted, restore it.

- [ ] **Step 5: Confirm the EX cross-check still cross-checks**

```bash
go test ./core/transport/ -run TestEXCrossCheck -v
go test ./internal/fakeradio/ -run TestNoCoreImports -v
```

Both must PASS. If `TestNoCoreImports` fails, the fake has been pointed at `core/cat` and the cross-check now compares a value with itself — stop and escalate.

- [ ] **Step 6: Full suite and commit**

Run: `gofmt -l . && go test ./... && go test -race ./core/...`

```bash
git add -A
git commit -m "m9b: task 55 — migrate every call site to the dialect

The mechanical bulk. Both frame and parser corpora pass against the
UNCHANGED golden files from task 51: the call syntax moved and the
behaviour did not."
```

---

### Task 56: Inject the gate; plumb the driver

**Files:** `core/transport/engine.go`, `errors.go`, `doc.go`; `core/driver/ft710/ft710.go`, `caps.go`, `read.go` and the driver's other codec callers; `core/transport/*_test.go` (six `NewEngine` call sites).

**Interfaces produced:** `type AllowFunc func(frame []byte) bool`; `func NewEngine(p Port, allow AllowFunc, opts ...Option) (*Engine, error)`; `var ErrNoAllowlist`.

- [ ] **Step 1: Write the failing fail-closed tests**

Create `core/transport/allowfunc_test.go`. **Read `core/transport/engine_test.go` first** and use its existing port doubles and command/spec constructors — do not invent parallel helpers.

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"testing"
)

// TestNewEngine_NilAllowFuncIsRefused: an Engine without a gate cannot be
// constructed at all.
func TestNewEngine_NilAllowFuncIsRefused(t *testing.T) {
	// Use whatever port double this package's tests already provide.
	e, err := NewEngine(newTestPort(t), nil)
	if err == nil {
		t.Fatal("NewEngine(port, nil) returned no error — an ungated Engine must not be constructable")
	}
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("NewEngine(port, nil) error = %v, want it to wrap ErrNoAllowlist", err)
	}
	if e != nil {
		t.Error("NewEngine returned a non-nil Engine alongside its error")
	}
}

// TestEngineDo_RefusesWithNoAllowlist is the defence-in-depth half: even
// if an Engine reaches Do without a gate, nothing reaches the wire.
// Unreachable through NewEngine by construction — checked anyway, exactly
// as ErrDisallowedCommand's own doc comment argues for the layer below.
func TestEngineDo_RefusesWithNoAllowlist(t *testing.T) {
	e, err := NewEngine(newTestPort(t), func([]byte) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	e.allow = nil // in-package: simulate the unreachable state

	// Use the package's existing command/spec constructors here.
	_, err = e.Do(t.Context(), someReadCommand(t), someSpec())
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("Do with a nil allowlist returned %v, want ErrNoAllowlist", err)
	}
}
```

Replace `newTestPort`, `someReadCommand` and `someSpec` with the real helper names. Add an assertion that nothing was written, using whatever recording facility the existing port double offers.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./core/transport/ -run 'TestNewEngine_NilAllowFunc|TestEngineDo_RefusesWithNoAllowlist' -v`
Expected: FAIL to compile — `NewEngine` takes one argument; `ErrNoAllowlist` undefined.

- [ ] **Step 3: Add `AllowFunc`, the sentinel and the constructor**

`core/transport/errors.go`:

```go
// ErrNoAllowlist means an Engine was asked to transmit with no allowlist.
// Distinct from ErrDisallowedCommand deliberately: that one means "this
// frame is not permitted", this one means "this Engine was misassembled".
// Both refuse, but conflating them would have a diagnostic blame the
// frame for a composition bug.
var ErrNoAllowlist = errors.New("transport: engine has no allowlist, refusing to transmit")
```

`core/transport/engine.go`: add `allow AllowFunc` to `Engine`; define `AllowFunc` with a doc comment noting `cat.Dialect.AllowedCommand` matches it exactly; change `NewEngine` to return `(*Engine, error)`, rejecting a nil gate *before* `go e.readLoop()`; and change the write-path check:

```go
		frame := cmd.Bytes()
		if e.allow == nil {
			return nil, ErrNoAllowlist
		}
		if !e.allow(frame) {
			return nil, fmt.Errorf("%w: %s", ErrDisallowedCommand, cmd.String())
		}
```

Keep safety obligation 1 intact — exactly one `Bytes()` call per transmission, the same slice checked and written.

- [ ] **Step 4: Plumb the dialect through the driver**

Codex F7: `transport.NewEngine` is called in `ft710Driver.Open` (`core/driver/ft710/ft710.go:155`) *before* any `Session` exists, so the dialect belongs on **`ft710Driver`**, not `Session`.

Add `dialect cat.Dialect` to `ft710Driver`, initialised to `cat.FT710` in its constructor, then:

```go
	eng, err := transport.NewEngine(port, d.dialect.AllowedCommand, engOpts...)
	if err != nil {
		return nil, fmt.Errorf("opening session: %w", err)
	}
```

Pass the dialect into `Session` and route **every** driver codec call through the stored field rather than `cat.FT710` — including helpers in `read.go`, `write.go`, `mc.go` and `settings.go`:

```bash
grep -rn "cat\.FT710" core/driver/ft710/ | grep -v _test
```

Only the driver's own construction site should remain.

- [ ] **Step 5: Fix the CAT ID and the mode rendering**

Replace `catID = "0800"` at `core/driver/ft710/caps.go:103`:

```go
// catID is the identity an FT-710 answers "ID;" with, sourced from the
// codec's dialect rather than restated here — one place this string
// exists, and the value M9c's driver registration will read too.
var catID = cat.FT710.CATID()
```

Add a linkage test in `core/driver/ft710`:

```go
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != cat.FT710.CATID() {
		t.Errorf("catID = %q, want cat.FT710.CATID() = %q", catID, cat.FT710.CATID())
	}
	if catID != "0800" {
		t.Errorf("catID = %q, want the documented %q — golden vector G1", catID, "0800")
	}
}
```

**Then fix the mode rendering.** Codex F7 found `core/driver/ft710/read.go:228` renders a channel's user-visible mode with `m.Mode.String()`, which reads the FT-710 table regardless of dialect — so revision 1's "diagnostic only" claim was false. Change it to render through the session's dialect (`s.dialect.ModeName(m.Mode)`), and give `Mode.String` a doc comment saying plainly that it is a diagnostic fallback reading the FT-710 table, and that the driver renders through the dialect.

CLI output must be unchanged: the FT-710 dialect's table is the same table, so `read --fake` still byte-matches Task 51's baseline. Verify now rather than at Task 59:

```bash
go run ./cmd/rigprog read --fake --settings --out /tmp/m9b-check.json > /tmp/m9b-check.txt 2>&1
diff .superpowers/sdd/m9b-baselines/read-fake.json /tmp/m9b-check.json && echo "codeplug identical"
```

- [ ] **Step 6: Update the six test call sites**

Each becomes `transport.NewEngine(p, cat.FT710.AllowedCommand, opts...)` with its error checked. Where a test genuinely needs a permissive gate for a reason unrelated to allowlisting, use an explicit local `func([]byte) bool { return true }` with a comment saying why — never nil.

- [ ] **Step 7: Run everything**

Run: `go test ./... && go test -race ./core/...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "m9b: task 56 — injected fail-closed gate, dialect plumbed through the driver

core/transport takes an AllowFunc at construction;
cat.Dialect.AllowedCommand matches that signature so the driver passes the
method value with no adapter. Fail-closed twice: NewEngine refuses a nil
gate before starting the read goroutine, and Do re-checks before every
write.

The dialect lives on ft710Driver, not Session — NewEngine is called in
Open before a Session exists (Codex F7). Every driver codec call routes
through the stored field.

Also on F7: read.go rendered a channel's user-visible mode with
Mode.String(), so revision 1's 'diagnostic only' claim was false. Modes now
render through the dialect; CLI output is byte-identical because the
FT-710's table is the same table.

core/transport still imports core/cat for the frame accumulator, the '?;'
rejection and the AI init frame. Only the gate is decoupled."
```

---

### Task 57: The second-dialect proof

**Files:** Create `core/cat/seconddialect_test.go`

**This is the milestone's real verification.** Every preceding task is arranged to make it possible. It is the only thing that can prove the receiver is load-bearing rather than cosmetic — no golden file can, because with one dialect the globals and the dialect hold identical data.

- [ ] **Step 1: Write the test**

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// testDialect is a deliberately WRONG dialect: a fictional radio whose
// every varying attribute differs from the FT-710's. It exists only in
// this file and is never exported.
//
// Its whole purpose is to fail if any code path consults package-level
// FT-710 data through a Dialect receiver. If someone converts a helper's
// signature but leaves its body reading a global, these tests go red and
// nothing else in the suite does.
var testDialect = Dialect{
	catID: "9999",
	modeNames: map[Mode]string{
		ModeLSB: "LOWER", // the FT-710 calls this "LSB"
		ModeUSB: "UPPER", // the FT-710 calls this "USB"
		// Deliberately omits every other mode the FT-710 knows.
	},
	slots: slotSpace{
		memoryLo: 1, memoryHi: 5, // FT-710: 1-99
		sixtyLo: 0, sixtyHi: 0, // no 60m bank at all
		pmsPairs: 2,  // FT-710: 9
		emgWire:  "", // no emergency channel
		noneWire: "000",
	},
	exItems:   nil,
	exMembers: map[EXAddress]bool{},
}

// TestSecondDialect_SlotSpaceIsHonoured fails if slot classification
// reads a global instead of the receiver.
func TestSecondDialect_SlotSpaceIsHonoured(t *testing.T) {
	if _, err := testDialect.ParseSlot("050"); err == nil {
		t.Error("testDialect parsed slot 050 — its memory range is 1-5, so this is reading the FT-710's slot space")
	}
	if _, err := testDialect.ParseSlot("003"); err != nil {
		t.Errorf("testDialect rejected slot 003, inside its own memory range: %v", err)
	}
	if _, err := testDialect.ParseSlot("501"); err == nil {
		t.Error("testDialect parsed slot 501 — it has no 60m bank")
	}
	if _, err := testDialect.ParseSlot("EMG"); err == nil {
		t.Error("testDialect parsed EMG — it has no emergency channel")
	}
	if _, err := testDialect.ParseSlot("P3L"); err == nil {
		t.Error("testDialect parsed P3L — it has only 2 PMS pairs")
	}
}

// TestSecondDialect_ModeSetIsHonoured fails if mode validation or
// rendering reads the package-level modeNames map.
func TestSecondDialect_ModeSetIsHonoured(t *testing.T) {
	if got := testDialect.ModeName(ModeLSB); got != "LOWER" {
		t.Errorf("testDialect.ModeName(ModeLSB) = %q, want %q — this is reading the FT-710's table", got, "LOWER")
	}
	if testDialect.ValidMode(ModeDATAFMN) {
		t.Error("testDialect claims to know ModeDATAFMN, which is not in its mode set")
	}
	if !FT710.ValidMode(ModeDATAFMN) {
		t.Error("FT710 lost a mode it should have — the two dialects have been conflated")
	}
}

// TestSecondDialect_EXMembershipIsHonoured fails if EX membership reads
// the package-level index.
func TestSecondDialect_EXMembershipIsHonoured(t *testing.T) {
	ft710Addrs := FT710.EXAddresses()
	if len(ft710Addrs) == 0 {
		t.Fatal("FT710 has no EX addresses")
	}
	if testDialect.KnownEXAddress(ft710Addrs[0]) {
		t.Errorf("testDialect claims to know EX address %s — its inventory is empty, so this is reading a global", ft710Addrs[0].Wire())
	}
	if _, err := testDialect.BuildEXRead(ft710Addrs[0]); err == nil {
		t.Error("testDialect built an EX read for an address it does not have")
	}
}

// TestSecondDialect_BuildersHonourTheirReceiver walks the slot-taking
// builders, where a hardwired validator hides most easily.
func TestSecondDialect_BuildersHonourTheirReceiver(t *testing.T) {
	ft710Slot, err := FT710.MemorySlot(50) // valid for FT710, not testDialect
	if err != nil {
		t.Fatal(err)
	}

	if _, err := testDialect.BuildMRRead(ft710Slot); err == nil {
		t.Error("testDialect.BuildMRRead accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMTRead(ft710Slot); err == nil {
		t.Error("testDialect.BuildMTRead accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMCSet(ft710Slot); err == nil {
		t.Error("testDialect.BuildMCSet accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMTSet(ft710Slot, true, "TAG"); err == nil {
		t.Error("testDialect.BuildMTSet accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMWSet(corpusMemoryData(ft710Slot)); err == nil {
		t.Error("testDialect.BuildMWSet accepted slot 050, outside its slot space")
	}
}

// TestSecondDialect_ParsersHonourTheirReceiver covers the path Codex
// finding F3 identified: parseMemoryFrame is shared between ParseMRAnswer
// and AllowedCommand's MW check, and a hardwired helper there is
// invisible to a builder-only corpus.
func TestSecondDialect_ParsersHonourTheirReceiver(t *testing.T) {
	if _, err := testDialect.ParseMCAnswer([]byte("MC050;")); err == nil {
		t.Error("testDialect.ParseMCAnswer accepted slot 050 — the parser is reading the FT-710's slot space")
	}
	if _, err := FT710.ParseMCAnswer([]byte("MC050;")); err != nil {
		t.Errorf("FT710.ParseMCAnswer rejected its own slot 050: %v", err)
	}
	if _, _, _, err := testDialect.ParseMTAnswer([]byte("MT0501TAG;")); err == nil {
		t.Error("testDialect.ParseMTAnswer accepted slot 050")
	}
}

// TestSecondDialect_AllowlistHonoursItsReceiver is the security half: the
// gate must refuse frames that are legal only under another dialect.
func TestSecondDialect_AllowlistHonoursItsReceiver(t *testing.T) {
	ft710Slot, err := FT710.MemorySlot(50)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := FT710.BuildMRRead(ft710Slot)
	if err != nil {
		t.Fatal(err)
	}

	if !FT710.AllowedCommand(cmd.Bytes()) {
		t.Fatal("FT710 refuses its own builder's output — the property test should already have caught this")
	}
	if testDialect.AllowedCommand(cmd.Bytes()) {
		t.Errorf("testDialect ACCEPTED %q, a frame for a slot outside its space — the gate is reading a global", cmd.Bytes())
	}
}

// TestZeroDialectRejectsEveryCorpusFrame is the fail-closed property the
// gate design rests on (Codex F6). A zero Dialect is constructible by any
// caller and its AllowedCommand is a non-nil method value satisfying
// transport.NewEngine's nil check — so it must accept NOTHING.
func TestZeroDialectRejectsEveryCorpusFrame(t *testing.T) {
	var zero Dialect

	checked := 0
	for _, line := range buildFrameCorpus(t) {
		cl := splitCorpusLine(line)
		if cl.rejected || cl.frame == "" {
			continue
		}
		checked++
		if zero.AllowedCommand([]byte(cl.frame)) {
			t.Errorf("zero Dialect ACCEPTED %q (%s) — an unconfigured dialect must accept nothing", cl.frame, cl.label)
		}
	}
	if checked == 0 {
		t.Fatal("checked no frames — the corpus or its parser is broken, and this passed vacuously")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./core/cat/ -run 'TestSecondDialect|TestZeroDialect' -v`

**Expected on a correct Task 54: PASS.** If any fails, a helper is still consulting a package-level global through a `Dialect` receiver — the bug this milestone exists to avoid. Fix it in `core/cat`, never by weakening this test.

- [ ] **Step 3: Prove the test would catch a regression**

Temporarily revert one converted helper to read a global — e.g. make `Dialect.ParseSlot` call `classifySlotWire` rather than `d.classifySlot` — run `TestSecondDialect_SlotSpaceIsHonoured`, confirm it FAILS, and revert. Record the message: this is the evidence that the seam is real.

- [ ] **Step 4: Commit**

```bash
git add core/cat/seconddialect_test.go
git commit -m "m9b: task 57 — the second-dialect proof

A deliberately wrong dialect (5 memory channels, no 60m bank, no EMG, 2
PMS pairs, two renamed modes, empty EX inventory) that must behave
DIFFERENTLY from the FT-710 at every varying attribute — including at the
allowlist, which must refuse a frame legal only under another dialect.

This is the milestone's real verification. No golden file can prove the
receiver is load-bearing while only one dialect exists, because the
globals and the dialect hold identical data — which is exactly how
revision 1 would have shipped a cosmetic seam with every test green
(Codex F3/F5).

Includes TestZeroDialectRejectsEveryCorpusFrame: an unconfigured dialect
is constructible by any caller and its AllowedCommand satisfies
NewEngine's nil check, so it must accept nothing (F6).

Verified to catch a regression by reverting one helper to a global."
```

---

### Task 58: The `NewEngine` construction guard; retire the duplicate

**Files:** Create `internal/guards/engine_construction_test.go`; modify `internal/guards/importgraph_test.go`, `internal/guards/simulated_tokens_test.go`.

- [ ] **Step 1: Write the guard, hardened per Codex F9**

Revision 1's version matched only a `SelectorExpr` named `NewEngine` and skipped the whole `core/transport` tree — so a dot-import would evade it, and a production wrapper inside `core/transport` could construct an Engine with any gate and hand it out.

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"testing"
)

// TestNewEngineReachableOnlyFromDriver pins the other half of M9b's
// fail-closed story. NewEngine takes the outbound allowlist as a
// parameter, so WHOEVER CALLS IT CHOOSES THE GATE. That choice belongs to
// the driver layer: a call site in app/ or cmd/ could pass a permissive
// func and bypass every policy layer above it.
//
// Matches both qualified calls (transport.NewEngine) and bare identifiers
// (a dot-import, which the plan review flagged as an evasion the first
// draft missed). core/transport is SCANNED, not skipped — only the
// declaration itself is exempt, so a wrapper constructor there cannot
// hide.
func TestNewEngineReachableOnlyFromDriver(t *testing.T) {
	files := parseRepo(t)

	sawDriverConstruction := false
	scanned := 0

	for _, pf := range files {
		scanned++
		ast.Inspect(pf.file, func(n ast.Node) bool {
			// Exempt the declaration itself.
			if fd, ok := n.(*ast.FuncDecl); ok && fd.Name.Name == "NewEngine" && inTree(pf.relDir, "core/transport") {
				return false
			}

			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			default:
				return true
			}
			if name != "NewEngine" {
				return true
			}

			if inTree(pf.relDir, "core/driver") {
				sawDriverConstruction = true
				return true
			}
			t.Errorf("%s: calls NewEngine — an Engine's allowlist is chosen at construction, so only core/driver/** may construct one", pf.relPath)
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("scanned no files — the walker or its filters are broken, and this check passed vacuously")
	}
	if !sawDriverConstruction {
		t.Error("never saw core/driver/** call NewEngine — the walker or its filters are broken, and this check passed vacuously")
	}
}
```

`parseRepo`, `inTree` and `parsedFile` already exist in this package — do not redefine them. `parseRepo` skips `_test.go` files, so test constructions are already out of scope; confirm that by reading it before relying on it.

- [ ] **Step 2: Run it, and prove it bites**

Run: `go test ./internal/guards/ -run TestNewEngineReachableOnlyFromDriver -v`
Expected: PASS.

Temporarily add a `transport.NewEngine(...)` call to a non-test file under `app/`, confirm the guard FAILS naming it, revert. Record the message.

- [ ] **Step 3: Retire the duplicate simulated-token guard**

`internal/guards/simulated_tokens_test.go`'s `PIN-LIFT LEDGER NOTE` says the pinned single-driver `TestSimulatedTokenSingleNonTestFileRepoWide` folds into the data-driven table when the pin lifts. It lifted in Task 52, so: delete that test from `importgraph_test.go` and update the note to record that it happened and when.

- [ ] **Step 4: Confirm the guard count**

Run: `go test ./internal/guards/ -v` and `grep -c "^func Test" internal/guards/*_test.go`

There were **five** guard test functions before M9b. Adding this one and deleting the duplicate leaves **five** — revision 1 said four, which Codex F11 corrected. Put the per-file counts in the report.

- [ ] **Step 5: Commit**

```bash
git add internal/guards/
git commit -m "m9b: task 58 — NewEngine construction guard; duplicate token guard retired

NewEngine now takes the allowlist, so whoever calls it chooses the gate.
Only core/driver/** may. Matches bare identifiers as well as qualified
calls (a dot-import would otherwise evade it) and scans core/transport
rather than skipping it, exempting only the declaration — both gaps the
plan review found in the first draft.

TestSimulatedTokenSingleNonTestFileRepoWide folded into the data-driven
table, as its own pin-lift note planned. Five guard tests before, five
after."
```

---

### Task 59: Documentation, byte-identity comparison, and the milestone gate

**Files:** `core/transport/doc.go`; `README.md` (only if it misdescribes the codec); `.superpowers/sdd/progress.md`; create `.superpowers/sdd/m9b-milestone-summary.md`.

- [ ] **Step 1: Update the transport package documentation**

`core/transport/doc.go` names `cat.AllowedCommand` as the gate around lines 112, 133 and 150. Rewrite those to describe the injected `AllowFunc`, keeping every safety-obligation statement intact. Add a paragraph stating plainly what did *not* change: transport still imports `core/cat` for the frame accumulator, `IsRejection`/`ErrRejected`, and `cat.BuildAISet` for the AI init frame at `engine.go:294`.

- [ ] **Step 2: Check the README's description**

Run: `grep -n "core/" README.md | grep -i "cat\|codec\|protocol"`

If the layout table calls `core/cat` the "CAT protocol codec", that remains accurate. **Do not claim multi-radio support:** there is exactly one driver and one configured dialect.

- [ ] **Step 3: Compare against every Task 51 baseline**

Codex F10's remedy — the design's byte-identity scope covers codeplug JSON, `Digest`, schema and CLI output, not just `probe --fake`.

```bash
B=.superpowers/sdd/m9b-baselines
go run ./cmd/rigprog probe --fake > /tmp/m9b-probe.txt 2>&1
diff "$B/probe-fake.txt" /tmp/m9b-probe.txt && echo "probe: identical"

go run ./cmd/rigprog read --fake --settings --out /tmp/m9b-read.json > /tmp/m9b-read.txt 2>&1
diff "$B/read-fake.txt" /tmp/m9b-read.txt   && echo "read stdout: identical"
diff "$B/read-fake.json" /tmp/m9b-read.json && echo "codeplug JSON, digest and schema: identical"

go run ./cmd/rigprog settings /tmp/m9b-read.json > /tmp/m9b-settings.txt 2>&1
diff "$B/settings.txt" /tmp/m9b-settings.txt && echo "settings: identical"

go run ./cmd/rigprog export --csv /tmp/m9b-export.csv /tmp/m9b-read.json > /tmp/m9b-export.txt 2>&1
diff "$B/export.csv" /tmp/m9b-export.csv && echo "CSV export: identical"

go run ./cmd/rigprog help > /tmp/m9b-help.txt 2>&1
diff "$B/help.txt" /tmp/m9b-help.txt && echo "help: identical"
```

Every one must be identical. A difference in the codeplug JSON means `Digest` or schema moved, which is out of scope for this milestone — stop and escalate rather than accepting it.

- [ ] **Step 4: Confirm no golden file was ever regenerated**

```bash
git log --oneline -- core/cat/testdata/
```

Expected: exactly one commit, Task 51's. State it explicitly in the milestone summary. More than one voids the milestone's central claim and must be explained.

- [ ] **Step 5: Run the full local gate**

```bash
gofmt -l .                                    # expect: empty
go vet ./...
go build ./...
go test ./...
go test ./internal/guards/ -v
go test -race ./core/...
cd app && wails generate module && git diff --exit-code frontend/wailsjs && cd ..
cd app/frontend && npm run check && npm run test && npm run build && cd ../..
go run ./cmd/rigprog version
```

- [ ] **Step 6: Write the milestone summary**

Create `.superpowers/sdd/m9b-milestone-summary.md` covering: what changed and why; the two approved scope departures, and that revision 1's third (deferring slot rules) was withdrawn on review; the three corpora and the second-dialect proof, with the evidence each was seen to fail; the guard amendment with its not-weakened argument; every baseline comparison; and the deferred list below.

- [ ] **Step 7: Ledger, commit, and send for Codex milestone review**

Append the milestone record to `.superpowers/sdd/progress.md`, commit, then run the Codex adversarial milestone review. **Repo-relative paths only** — an absolute path outside the workspace hangs the job silently (2.5 hours lost during M8c). Save to `.superpowers/sdd/m9b-codex-milestone-review.md`, adjudicate every finding explicitly, fix wave, opus re-review, then merge to `main` with `--no-ff`.

---

## Deferred, and ledgered as such

1. **Per-command frame-shape variants** — M9c, where the FTdx10 gives them a second implementation to answer to.
2. **`Slot`'s own predicates are FT-710-scoped.** `Wire`, `IsMemory`, `Is60m`, `Writable` and friends classify through the package-level `classifySlotWire`, which forwards to `FT710`. Giving `Slot` a dialect tag is M9c's, when a second slot space exists.
3. **Cross-dialect negative property tests over the full allowlist** — M9c. Task 57 covers the membership paths; a full property sweep wants a second dialect that is a real radio rather than a fixture.
4. **An exported `Dialect` constructor** — M9c. Roadmap A1's `core/cat/ftdx10` package needs `func Dialect() cat.Dialect`, which unexported fields make impossible from outside the package.
5. **Making transport's AI init frame injectable** — roadmap risk 10. A seam noted, not built, until a rig actually differs.

## Codex plan-review adjudication (revision 1 → 2)

Transcript: `.superpowers/sdd/m9b-plan-codex-review.md`. Verdict NEEDS-REVISION; 8 HIGH, 2 MEDIUM, 1 LOW. **All 11 verified against the source and ACCEPTED.**

| # | Sev | Finding | Resolution |
| --- | --- | --- | --- |
| 1 | HIGH | Literal pin cannot pass: multiline raw strings (`exobserved_test.go`'s `observedHeader`) break newline-delimited storage | Task 51 records `strconv.Quote`d single-line tokens, and tests the generated artefact before committing it |
| 2 | HIGH | A flat literal set loses attachment — a deleted literal passes if the spelling survives elsewhere, and Task 53 itself adds `"0800"` | Records are `(file, ordinal, token)`; a parser/error corpus added |
| 3 | HIGH | Both pins miss receiver hardwiring: `parseMemoryFrame` would keep calling `FT710.ParseSlot` from inside a `Dialect` method | Task 54 converts the whole helper chain; Task 57's second dialect proves it |
| 4 | HIGH | Task 54 could not land green — the guard breaks the moment builders become methods | Guard amendment moved to Task 52, before the churn |
| 5 | HIGH | Deferring slot rules was an unapproved third scope departure, leaving the receiver cosmetic | Withdrawn. Task 53 carries slot space and a per-dialect EX index as data |
| 6 | HIGH | A zero `Dialect` is constructible and its `AllowedCommand` satisfies the nil check | Zero value inert by construction; `TestZeroDialectRejectsEveryCorpusFrame` added |
| 7 | HIGH | Driver plumbing wrong: `NewEngine` is called before a `Session` exists; snippet used an undeclared value; `read.go:228` renders modes via `Mode.String` | Dialect lives on `ft710Driver`; every driver codec call routed through it; mode rendering moved to `Dialect.ModeName` |
| 8 | HIGH | `SixtyMSlot(501)` is an error — it takes an ordinal 1–99 | Corpus uses `SixtyMSlot(1)` and `SixtyMSlot(99)` |
| 9 | MED | `NewEngine` guard missed dot-imports and skipped `core/transport` | Matches `Ident` and `SelectorExpr`; scans transport, exempting only the declaration |
| 10 | MED | Byte-identity scope not established — only `probe --fake` compared | Task 51 captures codeplug JSON, digest, schema, CSV and CLI baselines; Task 59 diffs them all |
| 11 | LOW | Counts wrong (533 vs 553 vs ~220 real); five guard tests not four; `cat.BuildAISet` at `engine.go:294` falsifies the confinement claim | Counts re-derived in Task 55; guard count corrected; confinement claim narrowed to the Set-frame builders |

Codex also **confirmed** three things this plan asserts: the name-only matcher is strictly more inclusive than the package-qualified one (it found no call shape the new form misses); `internal/guards/importgraph_test.go:251-255`, `core/driver/ft710/ft710.go:155` and `core/driver/ft710/caps.go:103` are cited correctly; and `NewEngine` has exactly one production call site and six in tests.
