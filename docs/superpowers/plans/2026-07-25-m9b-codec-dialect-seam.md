# M9b Codec Dialect Seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `core/cat` from a single FT-710-flavoured codec into a dialect-parameterised one, and move the outbound allowlist out of `core/transport` into an injected, fail-closed gate — so that adding the FTdx10 in M9c is *write a table and register it*, not *fork the codec*.

**Architecture:** An exported `cat.Dialect` struct carries the four things that vary across the classic NEWCAT family (mode-nibble set, EX inventory, slot-space rules, CAT ID). One package-level instance, `cat.FT710`. Grammar entry points become methods on it. `core/transport` gains `type AllowFunc func([]byte) bool` and takes it at construction; `Dialect.AllowedCommand` matches that signature exactly, so the driver passes the method value with no adapter.

**Tech Stack:** Go 1.25 (stdlib only in `core/` — no new dependencies), `go/parser` and `go/ast` for the guard tests, existing table-driven test style.

**Design spec:** `docs/superpowers/specs/2026-07-25-m9b-codec-dialect-seam-design.md`. Read it before starting. Where this plan and the spec disagree, the spec wins and the plan is wrong — say so rather than guessing.

## Global Constraints

Every task's requirements implicitly include all of these.

- **stdlib only in `core/`.** No new module dependencies anywhere in this milestone.
- **SPDX header** on every new file: `// SPDX-License-Identifier: GPL-3.0-or-later` as the first line, then a blank line, then `package …`.
- **British English** in all user-facing copy and comments (`-ise`, `-our`, `behaviour`, `serialise`).
- **The word "snapshot", never "backup"** for saved radio contents.
- **`internal/fakeradio` imports nothing from `core/`.** `TestNoCoreImports` enforces this and must stay green and untouched. The fake's Table 2 transcription is an *independent* second source; if it ever read from `cat.Dialect`, `core/transport/ex_crosscheck_test.go` would be comparing a value with itself.
- **Expected byte literals in evidence tests never change.** The G1–G12 manual-derived golden vectors, the M5a/M5b hardware-derived vectors, the EX observed-CSV pins, and `TestAllowedCommand_RejectsGoldenAnswerFrames`' answer-frame corpus keep their expected values character-for-character. Only *invocations* change. Task 51 makes this mechanically enforced; if that check fails, stop and escalate rather than updating the golden file.
- **No menu-write path.** `Dialect.AllowedCommand` must continue to reject every EX Set/Answer-shaped frame. This is shipped policy, not a phase restriction (`docs/menu-write-decision.md`). Do not "improve" it.
- **CI is billing-dead.** The full local gate in Task 57 substitutes. Do not add or edit workflow files in this milestone.
- **Branch:** `m9b-dialect-seam`, cut from `main`. Merge at milestone end with `--no-ff`.

## File Structure

| File | Responsibility |
| --- | --- |
| `core/cat/dialect.go` | **New.** The `Dialect` struct, its unexported fields, and the package-level `FT710` value. Nothing else. |
| `core/cat/dialect_test.go` | **New.** Dialect construction and accessor tests. |
| `core/cat/framecorpus_test.go` | **New.** Drives every builder over a fixed corpus; compares against a committed golden file. |
| `core/cat/testdata/frame-corpus.golden` | **New.** One built frame per line. A reviewable diff, not a digest. |
| `core/cat/evidence_literals_test.go` | **New.** Inventories string literals in `core/cat`'s tests; asserts the committed set survives. |
| `core/cat/testdata/evidence-literals.golden` | **New.** Sorted unique string literals from `core/cat/*_test.go` at the pre-refactor state. |
| `core/cat/slot.go`, `mode.go`, `exinventory.go` | Membership rules become dialect-backed. |
| `core/cat/mw.go`, `mt.go`, `mc.go`, `mr.go`, `ex.go` | Builders/parsers become `Dialect` methods. |
| `core/cat/allowlist.go` | `AllowedCommand` becomes a `Dialect` method. |
| `core/transport/engine.go`, `errors.go` | `AllowFunc`, the new `NewEngine` signature, `ErrNoAllowlist`. |
| `core/driver/ft710/*.go` | Holds a `cat.Dialect`; passes `dialect.AllowedCommand` to `NewEngine`. |
| `internal/guards/importgraph_test.go` | Write-path matcher rewritten; **byte-identical pin formally amended here.** |
| `internal/guards/engine_construction_test.go` | **New.** `transport.NewEngine` referenced only from `core/driver/**`. |
| `internal/guards/simulated_tokens_test.go` | The folded-in single-driver guard; duplicate retired. |

## Task ordering and why

Task 51 must be first and must be its own commit: a pin minted after the refactor records whatever the refactor produced and proves nothing. Task 52 introduces the dialect with the old package-level functions delegating to it, so the tree stays green with zero call-site churn — every later task can then move one group at a time. Tasks 53 and 54 are the bulk churn (553 call sites between them). Task 55 is the security change. Task 56 is guards. Task 57 verifies and gates.

---

### Task 51: Mint the evidence pins

**Files:**
- Create: `core/cat/framecorpus_test.go`
- Create: `core/cat/testdata/frame-corpus.golden`
- Create: `core/cat/evidence_literals_test.go`
- Create: `core/cat/testdata/evidence-literals.golden`

**Interfaces:**
- Consumes: nothing.
- Produces: two golden files that every later task must leave intact. No exported Go symbols.

**Why this task exists.** M9b rewrites 553 call sites. The frame corpus proves no builder's *output bytes* moved. The literal inventory proves no test's *expected value* was quietly edited to match a change. Neither is meaningful unless minted before any signature moves.

- [ ] **Step 1: Write the frame-corpus test, golden file absent**

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

// frameCorpusPath is the committed golden file: one built frame per line,
// in the order buildFrameCorpus produces them.
const frameCorpusPath = "testdata/frame-corpus.golden"

// buildFrameCorpus drives every frame-producing builder in this package
// over a fixed, deliberately boring input set and returns the wire bytes,
// one frame per element, rendered as Go-quoted strings.
//
// It exists for exactly one reason: M9b moves every builder onto
// cat.Dialect and rewrites ~553 call sites, and this is the artefact that
// proves the BYTES did not move while the call syntax did. Keep the
// inputs stable. Adding a case is fine (append only, and regenerate);
// changing an existing one destroys the comparison this file exists for.
func buildFrameCorpus(t *testing.T) []string {
	t.Helper()

	var out []string
	add := func(label string, c Command, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("corpus %s: unexpected builder error: %v", label, err)
		}
		out = append(out, label+"\t"+string(c.Bytes()))
	}

	// Infallible builders.
	out = append(out, "ID.read\t"+string(BuildIDRead().Bytes()))
	out = append(out, "AI.set.on\t"+string(BuildAISet(true).Bytes()))
	out = append(out, "AI.set.off\t"+string(BuildAISet(false).Bytes()))
	out = append(out, "MC.read\t"+string(BuildMCRead().Bytes()))

	// Slot-taking builders, over a fixed slot set spanning every kind.
	for _, sc := range corpusSlots(t) {
		c, err := BuildMRRead(sc.slot)
		add("MR.read."+sc.label, c, err)

		c, err = BuildMTRead(sc.slot)
		add("MT.read."+sc.label, c, err)

		if c, err := BuildMCSet(sc.slot); err == nil {
			add("MC.set."+sc.label, c, nil)
		}
		if c, err := BuildMTSet(sc.slot, true, "TAG"); err == nil {
			add("MT.set.tag."+sc.label, c, nil)
		}
		if c, err := BuildMTSet(sc.slot, false, ""); err == nil {
			add("MT.set.clear."+sc.label, c, nil)
		}
	}

	// MW over a fixed MemoryData, one per writable slot.
	for _, sc := range corpusSlots(t) {
		md := corpusMemoryData(sc.slot)
		if c, err := BuildMWSet(md); err == nil {
			add("MW.set."+sc.label, c, nil)
		}
	}

	// Every EX read in the inventory — all 296.
	for _, a := range EXAddresses() {
		c, err := BuildEXRead(a)
		add("EX.read."+a.Wire(), c, err)
	}

	return out
}

type corpusSlot struct {
	label string
	slot  Slot
}

// corpusSlots spans every slot kind the codec knows: memory, PMS, 60m and
// EMG, at their boundaries.
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
		{"sixty501", must(SixtyMSlot(501))},
		{"emg", EMGSlot()},
	}
}

// corpusMemoryData returns a fixed MemoryData for slot s. Every field is a
// constant so the resulting frame depends only on the slot.
func corpusMemoryData(s Slot) MemoryData {
	return MemoryData{
		Slot:   s,
		FreqHz: 14_250_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeUSB,
		// KindMemory ('1') for every slot, memory or PMS alike — the
		// pairing rule M5b confirmed on hardware. See MemoryData.Kind.
		Kind:  KindMemory,
		CTCSS: CTCSSOff,
		Shift: ShiftSimplex,
	}
}

// TestFrameCorpus_MatchesGolden is the byte-identity pin for M9b. A
// failure means a builder's OUTPUT changed, which during a
// call-site-only refactor is a bug, not an expected diff. Do not
// regenerate the golden file to make this pass.
func TestFrameCorpus_MatchesGolden(t *testing.T) {
	got := strings.Join(buildFrameCorpus(t), "\n") + "\n"

	want, err := os.ReadFile(filepath.FromSlash(frameCorpusPath))
	if err != nil {
		t.Fatalf("reading %s: %v (generate it with -update on first run)", frameCorpusPath, err)
	}

	if got != string(want) {
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(string(want), "\n")
		for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("frame corpus diverged at line %d:\n  golden: %q\n  built:  %q\n\nA builder's output bytes changed. During M9b this is a bug, not a diff to accept.", i+1, wantLines[i], gotLines[i])
			}
		}
		t.Fatalf("frame corpus length differs: golden %d lines, built %d lines", len(wantLines), len(gotLines))
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./core/cat/ -run TestFrameCorpus_MatchesGolden -v`
Expected: FAIL — `reading testdata/frame-corpus.golden: ... no such file or directory`

If it instead fails to compile because `KindMemory`, `MemoryData.Kind` or `MemoryData.ClarHz` do not exist under those names, read `core/cat/memdata.go` and use the real field names. Do not invent fields.

- [ ] **Step 3: Generate the golden file**

Add a temporary generator test, run it, then delete it:

```bash
mkdir -p core/cat/testdata
```bash
cat >> core/cat/framecorpus_test.go <<'EOF'

// TestGenerateFrameCorpus is a one-shot generator. DELETE THIS FUNCTION
// after the golden file is committed — the pin is worthless if the thing
// it pins can regenerate itself on demand.
func TestGenerateFrameCorpus(t *testing.T) {
	if os.Getenv("GENERATE_FRAME_CORPUS") == "" {
		t.Skip("set GENERATE_FRAME_CORPUS=1 to regenerate")
	}
	body := strings.Join(buildFrameCorpus(t), "\n") + "\n"
	if err := os.WriteFile(filepath.FromSlash(frameCorpusPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
EOF
GENERATE_FRAME_CORPUS=1 go test ./core/cat/ -run TestGenerateFrameCorpus -v
```

- [ ] **Step 4: Delete the generator and verify the pin passes**

Remove the `TestGenerateFrameCorpus` function you just appended. Then:

Run: `go test ./core/cat/ -run TestFrameCorpus_MatchesGolden -v`
Expected: PASS

Sanity-check the golden file by eye: it should contain ~330 lines, every EX line should look like `EX.read.010101\tEX010101;`, and no line should contain a value that looks like a real callsign or frequency from anyone's radio.

- [ ] **Step 5: Write the evidence-literal inventory test**

Create `core/cat/evidence_literals_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// evidenceLiteralsPath is the committed inventory of every string literal
// appearing in this package's tests at the pre-M9b state.
const evidenceLiteralsPath = "testdata/evidence-literals.golden"

// collectTestStringLiterals returns every distinct string literal in this
// package's _test.go files, sorted.
//
// The point is narrow and worth stating: M9b rewrites ~553 call sites
// mechanically. A rename touches IDENTIFIERS. It must not touch expected
// VALUES. Every golden vector, hardware-derived frame and reject-table
// entry in this package is a string literal, so a rewrite that changed
// one — to make a test pass, say — shows up here as a missing entry.
func collectTestStringLiterals(t *testing.T) map[string]bool {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}

	lits := map[string]bool{}
	fset := token.NewFileSet()
	seenFiles := 0

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		// This file and the frame corpus are themselves tooling, not
		// evidence; their literals would churn as the tooling evolves.
		if name == "evidence_literals_test.go" || name == "framecorpus_test.go" {
			continue
		}
		seenFiles++

		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			lits[bl.Value] = true
			return true
		})
	}

	if seenFiles < 10 {
		t.Fatalf("only scanned %d test files — the walker or its filter is broken, and this check would pass vacuously", seenFiles)
	}
	return lits
}

// TestEvidenceLiterals_CommittedSetSurvives asserts that every string
// literal present before M9b is still present. It is deliberately a
// SUPERSET check: adding a test (new literals) is fine, changing or
// deleting an existing expected value is not.
//
// If this fails, do not regenerate the golden file. A missing literal
// means a mechanical rewrite edited an expected value — find out which
// and why.
func TestEvidenceLiterals_CommittedSetSurvives(t *testing.T) {
	current := collectTestStringLiterals(t)

	raw, err := os.ReadFile(filepath.FromSlash(evidenceLiteralsPath))
	if err != nil {
		t.Fatalf("reading %s: %v", evidenceLiteralsPath, err)
	}

	var missing []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !current[line] {
			missing = append(missing, line)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		shown := missing
		if len(shown) > 20 {
			shown = shown[:20]
		}
		t.Fatalf("%d expected literal(s) from the pre-M9b evidence no longer appear in this package's tests.\nFirst %d:\n  %s\n\nA call-site rewrite must not change expected values. Do NOT regenerate the golden file.",
			len(missing), len(shown), strings.Join(shown, "\n  "))
	}
}
```

- [ ] **Step 6: Generate the literal inventory and verify**

```bash
cat >> core/cat/evidence_literals_test.go <<'EOF'

// TestGenerateEvidenceLiterals is a one-shot generator. DELETE THIS
// FUNCTION after the golden file is committed.
func TestGenerateEvidenceLiterals(t *testing.T) {
	if os.Getenv("GENERATE_EVIDENCE_LITERALS") == "" {
		t.Skip("set GENERATE_EVIDENCE_LITERALS=1 to regenerate")
	}
	lits := collectTestStringLiterals(t)
	out := make([]string, 0, len(lits))
	for l := range lits {
		out = append(out, l)
	}
	sort.Strings(out)
	body := strings.Join(out, "\n") + "\n"
	if err := os.WriteFile(filepath.FromSlash(evidenceLiteralsPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
EOF
GENERATE_EVIDENCE_LITERALS=1 go test ./core/cat/ -run TestGenerateEvidenceLiterals -v
```

Then delete `TestGenerateEvidenceLiterals` and run:

Run: `go test ./core/cat/ -run 'TestEvidenceLiterals|TestFrameCorpus' -v`
Expected: PASS, both.

- [ ] **Step 7: Verify the pins actually bite**

Prove they are not vacuous before trusting them for the rest of the milestone.

```bash
# Temporarily corrupt one expected byte in a builder and confirm the corpus fails.
# Revert immediately afterwards.
```

Edit `core/cat/id.go`'s `BuildIDRead` to emit `"ID ;"` instead of `"ID;"`, run `go test ./core/cat/ -run TestFrameCorpus_MatchesGolden`, and confirm it FAILS naming the line. Then `git checkout core/cat/id.go`.

Do the same for the literal pin: change any expected string in `core/cat/mt_test.go` (e.g. a golden frame) by one character, confirm `TestEvidenceLiterals_CommittedSetSurvives` FAILS, then revert.

Record both results in the task report. A pin that has never been seen to fail is not a pin.

- [ ] **Step 8: Full package test and commit**

Run: `gofmt -l . && go vet ./core/cat/ && go test ./core/cat/`
Expected: gofmt silent, vet clean, all tests pass.

```bash
git add core/cat/framecorpus_test.go core/cat/evidence_literals_test.go core/cat/testdata/
git commit -m "m9b: task 51 — mint the frame-corpus and evidence-literal pins

Minted BEFORE any signature moves, which is the only order in which they
prove anything: a pin taken after the refactor records whatever the
refactor produced.

frame-corpus.golden holds every builder's output over a fixed input set,
one frame per line so a diff is reviewable rather than a digest mismatch.
evidence-literals.golden holds every string literal in this package's
tests; the check is a SUPERSET assertion, so adding a test is fine and
changing an expected value is not.

Both were verified to fail when deliberately broken — a pin never seen to
fail is not a pin."
```

---

### Task 52: Introduce `cat.Dialect` with delegating package-level functions

**Files:**
- Create: `core/cat/dialect.go`
- Create: `core/cat/dialect_test.go`
- Modify: `core/cat/mode.go` (mode set moves into the dialect), `core/cat/slot.go` (slot-space rules), `core/cat/exinventory.go` (inventory accessor)

**Interfaces:**
- Consumes: Task 51's pins (must stay green).
- Produces:
  - `type Dialect struct` — exported type, unexported fields.
  - `var FT710 Dialect` — the single package-level instance.
  - `func (d Dialect) CATID() string`
  - `func (d Dialect) ModeName(m Mode) string`
  - `func (d Dialect) ValidMode(m Mode) bool`
  - `func (d Dialect) EXItems() []EXItem`
  - `func (d Dialect) KnownEXAddress(a EXAddress) bool`

**Key constraint for this task:** the existing package-level functions keep working, delegating to `FT710`. **Zero call sites change.** The tree must be green with the full existing test suite untouched. Tasks 53–54 do the migration.

- [ ] **Step 1: Write the failing dialect test**

Create `core/cat/dialect_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// TestFT710Dialect_CarriesTheRadioSpecificData pins what the dialect is
// FOR: the four things that vary across the classic NEWCAT family. If a
// fifth thing turns out to vary, it belongs here, not in a package-level
// var.
func TestFT710Dialect_CarriesTheRadioSpecificData(t *testing.T) {
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

// TestFT710Dialect_EXItemsReturnsFreshCopies mirrors the existing
// TestEXItems_ReturnsFreshCopies guarantee: a caller mutating the
// returned slice must not corrupt the inventory for everyone else.
func TestFT710Dialect_EXItemsReturnsFreshCopies(t *testing.T) {
	first := FT710.EXItems()
	if len(first) == 0 {
		t.Fatal("FT710.EXItems() returned nothing")
	}
	original := first[0]
	first[0].Name = "MUTATED"

	second := FT710.EXItems()
	if second[0].Name == "MUTATED" {
		t.Error("FT710.EXItems() shares backing storage between calls — a caller's mutation leaked into the inventory")
	}
	if second[0] != original {
		t.Errorf("FT710.EXItems()[0] = %+v after a caller mutated an earlier copy, want %+v", second[0], original)
	}
}

// TestPackageLevelFunctions_DelegateToFT710 is this task's whole point:
// the old API still works and answers identically, so no call site has to
// change yet.
func TestPackageLevelFunctions_DelegateToFT710(t *testing.T) {
	if len(EXItems()) != len(FT710.EXItems()) {
		t.Error("EXItems() and FT710.EXItems() disagree on length")
	}
	for _, a := range EXAddresses() {
		if KnownEXAddress(a) != FT710.KnownEXAddress(a) {
			t.Fatalf("KnownEXAddress(%s) and FT710.KnownEXAddress disagree", a.Wire())
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./core/cat/ -run TestFT710Dialect -v`
Expected: FAIL — `undefined: FT710`

- [ ] **Step 3: Create the dialect**

Create `core/cat/dialect.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// Dialect is one radio family's CAT variation: everything this codec
// needs that differs between models sharing the classic NEWCAT grammar.
//
// It carries DATA, not frame shapes. That is a deliberate M9b scope
// decision, recorded in
// docs/superpowers/specs/2026-07-25-m9b-codec-dialect-seam-design.md: the
// FTdx10/101 manuals document a combined ~50-byte MT record frame against
// the FT-710's short form, but that difference is unverified against
// hardware, and the FT-710's own MT is the precedent for a manual being
// wrong about exactly this. Per-command frame-shape variants are M9c's,
// where a second radio exists to shape them.
//
// Fields are unexported and there is no exported constructor, so FT710
// below is the only instance that can exist. M9c adds a constructor when
// it has a real second caller to shape one — see the design document's
// "Architecture" section, which records this as a known gap rather than
// an oversight.
type Dialect struct {
	catID string

	// modeNames is the mode-nibble set: the P6 wire bytes this radio
	// family emits and accepts, mapped to their display names.
	modeNames map[Mode]string

	// exItems is the generated EX (MENU) inventory. Validity is
	// membership of this table, never a numeric range — see
	// KnownEXAddress.
	exItems []EXItem
}

// FT710 is the Yaesu FT-710 dialect: the only one that exists.
var FT710 = Dialect{
	catID:     "0800",
	modeNames: modeNames,
	exItems:   exItemsGen,
}

// CATID is the four-character identity this radio answers to an "ID;"
// read with. core/driver/ft710 compares it against what the radio
// actually said; a mismatch is the wrong radio.
func (d Dialect) CATID() string { return d.catID }

// ValidMode reports whether m is a mode nibble this dialect knows.
func (d Dialect) ValidMode(m Mode) bool {
	_, ok := d.modeNames[m]
	return ok
}

// ModeName returns the display name for m, or a diagnostic placeholder
// for a Mode this dialect does not know.
func (d Dialect) ModeName(m Mode) string {
	if name, ok := d.modeNames[m]; ok {
		return name
	}
	return unknownModeName(m)
}

// EXItems returns a fresh copy of the EX inventory. Fresh because callers
// have historically been free to mutate what they get back, and one
// caller's mutation must never become everyone's inventory.
func (d Dialect) EXItems() []EXItem {
	out := make([]EXItem, len(d.exItems))
	copy(out, d.exItems)
	return out
}

// KnownEXAddress reports whether a is a member of this dialect's EX
// inventory. MEMBERSHIP, never a numeric range: the FT-710's Table 2 is
// sparse and its P1 groups are not contiguous.
func (d Dialect) KnownEXAddress(a EXAddress) bool {
	for _, it := range d.exItems {
		if it.Addr == a {
			return true
		}
	}
	return false
}
```

Note: if `KnownEXAddress` in `exinventory.go` currently uses a map or binary search rather than a linear scan, mirror **that** implementation here rather than the linear scan above — it is on the hot path for the 296-address sweep. Read the existing function first.

- [ ] **Step 4: Add `unknownModeName` and make the package-level functions delegate**

In `core/cat/mode.go`, extract the placeholder rendering so both `Mode.String()` and `Dialect.ModeName` share it:

```go
// unknownModeName renders a Mode that no dialect recognises. Shared by
// Mode.String and Dialect.ModeName so the two cannot drift.
func unknownModeName(m Mode) string {
	return fmt.Sprintf("Mode(%#02x)", byte(m))
}
```

and change `Mode.String()` to:

```go
// String returns the FT-710 display name for m. DIAGNOSTIC ONLY: it reads
// the FT-710's table regardless of dialect, because a Mode value has no
// way to reach the dialect it came from. User-facing mode display comes
// from spec.Capabilities.Modes (see core/driver/ft710/caps.go), never
// from here.
func (m Mode) String() string {
	return FT710.ModeName(m)
}
```

In `core/cat/exinventory.go`, make the package-level accessors delegate:

```go
// EXItems returns a fresh copy of the FT-710 EX inventory.
//
// Deprecated for internal use: prefer Dialect.EXItems. This wrapper
// exists so Task 52 lands green with no call-site churn; Task 53 removes
// it along with its callers.
func EXItems() []EXItem { return FT710.EXItems() }

// KnownEXAddress reports whether a is in the FT-710 EX inventory.
//
// Deprecated for internal use: prefer Dialect.KnownEXAddress. Same
// rationale as EXItems.
func KnownEXAddress(a EXAddress) bool { return FT710.KnownEXAddress(a) }
```

- [ ] **Step 5: Run the dialect tests**

Run: `go test ./core/cat/ -run TestFT710Dialect -v && go test ./core/cat/ -run TestPackageLevelFunctions -v`
Expected: PASS

- [ ] **Step 6: Run the whole suite and both pins**

Run: `go test ./core/cat/ && go test ./... `
Expected: PASS everywhere. **The frame corpus and literal inventory must both still pass** — this task changed no builder output and no expected value.

If `TestEXInventoryGenerated_NotStale` fails, the generator's output and `exItemsGen` have diverged; do not edit the generated file by hand — re-run the generator per `internal/extable/gen`.

- [ ] **Step 7: Commit**

```bash
git add core/cat/dialect.go core/cat/dialect_test.go core/cat/mode.go core/cat/exinventory.go
git commit -m "m9b: task 52 — cat.Dialect, with the package-level API delegating to it

Introduces the type and the single FT710 instance carrying the four
things that vary across the classic NEWCAT family: CAT ID, mode-nibble
set, EX inventory, and (task 53) slot-space rules.

Deliberately changes no call sites: the existing package-level functions
delegate to FT710, so the tree stays green and the migration can proceed
one group at a time. Fields are unexported with no constructor, so FT710
is the only instance that can exist — M9c adds a constructor when it has
a second caller to shape one.

Mode.String() now reads FT710's table and says so: a Mode value cannot
reach its own dialect, and user-facing mode display comes from
spec.Capabilities.Modes, not from here."
```

---

### Task 53: Move slot-space and EX membership onto the dialect

**Files:**
- Modify: `core/cat/dialect.go`, `core/cat/slot.go`, `core/cat/ex.go`, `core/cat/exinventory.go`
- Modify: every call site of `ParseSlot`, `MemorySlot`, `PMSSlot`, `SixtyMSlot`, `EMGSlot`, `ParseMode`, `EXItems`, `EXAddresses`, `KnownEXAddress`, `ParseEXAddress`, `NewEXAddress` — approximately 217 across the repo

**Interfaces:**
- Consumes: `Dialect`, `FT710` from Task 52.
- Produces:
  - `func (d Dialect) ParseSlot(wire string) (Slot, error)`
  - `func (d Dialect) MemorySlot(n int) (Slot, error)`
  - `func (d Dialect) PMSSlot(pair int, upper bool) (Slot, error)`
  - `func (d Dialect) SixtyMSlot(n int) (Slot, error)`
  - `func (d Dialect) EMGSlot() Slot`
  - `func (d Dialect) ParseMode(c byte) (Mode, error)`
  - `func (d Dialect) EXAddresses() []EXAddress`
  - `func (d Dialect) ParseEXAddress(wire string) (EXAddress, error)`
  - `func (d Dialect) NewEXAddress(p1, p2, p3 int) (EXAddress, error)`

`Slot`'s own predicates (`Wire`, `IsMemory`, `IsPMS`, `Is60m`, `IsEMG`, `IsNone`, `Writable`) stay methods on `Slot`. A `Slot` is already canonical by construction, so its predicates need no dialect. **Do not move them.**

**An honesty note this task must not paper over.** The design document's architecture table lists "slot-space rules" among the things `Dialect` *carries*. After this task the dialect **owns the entry points** but the rules themselves are still the package-level `classifySlotWire`, hardcoded to the FT-710's 3-digit space — the dialect struct grows no slot field. That is deliberate: a slot-space descriptor designed now would have exactly one instance and no second shape to answer to, which is the same speculation the milestone declined for frame variants. Parameterising it is M9c's, when the FTdx10 (3-digit, same space) and eventually the FTX-1 (5-digit) give it something to vary against. Record it in the deferred list and the task report; do not silently leave the discrepancy for a reviewer to find.

- [ ] **Step 1: Write the failing test for dialect-scoped slot parsing**

Add to `core/cat/dialect_test.go`:

```go
// TestFT710Dialect_SlotSpace pins the slot-space rules as dialect data.
// The FTX-1 uses 5-digit slots and the classic family 3-digit; that is
// exactly the kind of difference this seam exists to carry.
func TestFT710Dialect_SlotSpace(t *testing.T) {
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
		{"000", true},   // "none" — parses, means VFO/MT/QMB
		{"100", false},  // above the memory range, below 60m
		{"00001", false}, // FTX-1 shape: not this dialect's
	}
	for _, tc := range cases {
		_, err := FT710.ParseSlot(tc.wire)
		if (err == nil) != tc.want {
			t.Errorf("FT710.ParseSlot(%q): err == nil is %v, want %v", tc.wire, err == nil, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./core/cat/ -run TestFT710Dialect_SlotSpace -v`
Expected: FAIL — `FT710.ParseSlot undefined`

- [ ] **Step 3: Add the methods, keeping package-level delegates**

In `core/cat/slot.go`, add a method for each constructor that simply calls the existing unexported logic, e.g.:

```go
// ParseSlot parses a 3-byte wire slot code under this dialect's slot
// space. Reference: "Slot codes (3 bytes on the wire)".
func (d Dialect) ParseSlot(wire string) (Slot, error) {
	if classifySlotWire(wire) == slotKindInvalid {
		return Slot{}, newParseError([]byte(wire), "invalid slot code")
	}
	return Slot{wire: wire}, nil
}
```

Mirror the existing `ParseSlot` body exactly — read it first and copy its error text verbatim, because those strings are in the evidence-literal inventory.

Keep the package-level `ParseSlot` as `func ParseSlot(w string) (Slot, error) { return FT710.ParseSlot(w) }` for the duration of this step.

- [ ] **Step 4: Migrate the call sites**

Within `core/cat` (in-package, unqualified calls):

```bash
cd core/cat
# Verify the count first so you can check it afterwards.
grep -c 'ParseSlot(' *.go
```

Rewrite each unqualified call to `FT710.ParseSlot(` — **by hand or with a scripted replacement you then read in full**. Do the same for `MemorySlot`, `PMSSlot`, `SixtyMSlot`, `EMGSlot`, `ParseMode`, `EXItems`, `EXAddresses`, `KnownEXAddress`, `ParseEXAddress`, `NewEXAddress`.

Outside `core/cat`, calls are qualified (`cat.ParseSlot(`) and become `cat.FT710.ParseSlot(`. Affected trees: `core/driver/ft710/`, `core/codeplug/`, `core/csvio/`, `app/`, `internal/`.

**After each file, re-run `go test ./core/cat/ -run 'TestEvidenceLiterals|TestFrameCorpus'`.** If either fails you have changed a value, not a call. Stop and find out which.

- [ ] **Step 5: Delete the package-level delegates**

Remove the package-level `ParseSlot`, `MemorySlot`, `PMSSlot`, `SixtyMSlot`, `EMGSlot`, `ParseMode`, `EXItems`, `EXAddresses`, `KnownEXAddress`, `ParseEXAddress`, `NewEXAddress`.

Run: `go build ./... && go vet ./...`
Expected: clean. Any remaining compile error is a call site you missed — fix it, do not restore the delegate.

- [ ] **Step 6: Full suite and pins**

Run: `gofmt -l . && go test ./...`
Expected: everything passes, **including both Task 51 pins**.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "m9b: task 53 — slot space, mode set and EX membership become dialect methods

Moves the membership rules onto cat.Dialect and migrates every call site
(~217). Slot's own predicates stay methods on Slot: a Slot is canonical
by construction, so IsMemory/Writable/Wire need no dialect.

Both task-51 pins stayed green throughout, which is the claim that
matters: identifiers moved, expected values did not."
```

---

### Task 54: Move builders and parsers onto the dialect

**Files:**
- Modify: `core/cat/mw.go`, `mt.go`, `mc.go`, `mr.go`, `ex.go`, `id.go`, `ai.go`
- Modify: every call site — approximately 316 across the repo

**Interfaces:**
- Consumes: everything from Tasks 52–53.
- Produces:
  - `func (d Dialect) BuildMWSet(m MemoryData) (Command, error)`
  - `func (d Dialect) BuildMTSet(s Slot, display bool, tag string) (Command, error)`
  - `func (d Dialect) BuildMTRead(s Slot) (Command, error)`
  - `func (d Dialect) BuildMRRead(s Slot) (Command, error)`
  - `func (d Dialect) BuildMCSet(s Slot) (Command, error)`
  - `func (d Dialect) BuildMCRead() Command`
  - `func (d Dialect) BuildEXRead(addr EXAddress) (Command, error)`
  - `func (d Dialect) BuildIDRead() Command`
  - `func (d Dialect) BuildAISet(on bool) Command`
  - `func (d Dialect) ParseMRAnswer(frame []byte) (MemoryData, error)`
  - `func (d Dialect) ParseMTAnswer(frame []byte) (Slot, bool, string, error)`
  - `func (d Dialect) ParseMCAnswer(frame []byte) (Slot, error)`
  - `func (d Dialect) ParseEXAnswer(frame []byte) (EXAddress, string, error)`
  - `func (d Dialect) ParseIDAnswer(frame []byte) (radioID string, err error)`
  - `func (d Dialect) ParseAIAnswer(frame []byte) (on bool, err error)`

`BuildIDRead`, `BuildAISet`, `ParseIDAnswer` and `ParseAIAnswer` consult no dialect data today. They become methods anyway, for the reason the design gives: M9c's job should be writing a table, not re-plumbing signatures. Their doc comments must say so, so a later reader does not "tidy" them back to package level.

- [ ] **Step 1: Write the failing test**

Add to `core/cat/dialect_test.go`:

```go
// TestFT710Dialect_BuildersProduceTheSameBytes is a belt-and-braces
// companion to the frame corpus: it proves the METHOD form produces what
// the golden corpus recorded from the package-level form.
func TestFT710Dialect_BuildersProduceTheSameBytes(t *testing.T) {
	s, err := FT710.MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}

	c, err := FT710.BuildMRRead(s)
	if err != nil {
		t.Fatalf("FT710.BuildMRRead: %v", err)
	}
	if got := string(c.Bytes()); got != "MR001;" {
		t.Errorf("FT710.BuildMRRead(001) = %q, want %q", got, "MR001;")
	}

	if got := string(FT710.BuildIDRead().Bytes()); got != "ID;" {
		t.Errorf("FT710.BuildIDRead() = %q, want %q", got, "ID;")
	}
	if got := string(FT710.BuildAISet(true).Bytes()); got != "AI1;" {
		t.Errorf("FT710.BuildAISet(true) = %q, want %q", got, "AI1;")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./core/cat/ -run TestFT710Dialect_BuildersProduceTheSameBytes -v`
Expected: FAIL — `FT710.BuildMRRead undefined`

- [ ] **Step 3: Convert each builder and parser to a method**

For each function, change `func BuildX(` to `func (d Dialect) BuildX(` and replace any internal call to a now-moved function with `d.`-qualified form. Example, `core/cat/mr.go`:

```go
// BuildMRRead builds the MR read request for slot s.
func (d Dialect) BuildMRRead(s Slot) (Command, error) {
	if !readableSlot(s) {
		return Command{}, newParseError([]byte(s.Wire()), "slot is not readable")
	}
	return newCommand("MR" + s.Wire() + ";"), nil
}
```

Copy each existing body verbatim; change only the receiver and internal call qualification. Error strings are in the literal inventory — retyping one from memory will fail the pin, which is the point.

For the four dialect-independent ones, add the explanatory comment:

```go
// BuildIDRead builds the "ID;" identity read.
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio: uniform method form means M9c adds a dialect by writing a table,
// not by re-plumbing signatures (see the M9b design document). Do not
// "tidy" this back to a package-level function.
func (d Dialect) BuildIDRead() Command {
	return newCommand("ID;")
}
```

- [ ] **Step 4: Update the frame corpus to the method form**

`core/cat/framecorpus_test.go` calls the package-level builders. Change each call to `FT710.`-qualified form. **Change nothing else in that file — not the inputs, not the labels, not the golden path.**

Run: `go test ./core/cat/ -run TestFrameCorpus_MatchesGolden -v`
Expected: PASS, against the unchanged golden file. This is the milestone's central claim.

If it fails, a builder's behaviour changed during conversion. Find it; do not regenerate.

- [ ] **Step 5: Migrate the remaining call sites**

Same discipline as Task 53: in-package unqualified calls become `FT710.X(`; out-of-package `cat.X(` becomes `cat.FT710.X(`. The heaviest files are `core/cat/mt_test.go` (51), `core/cat/mw_test.go` (40), `core/driver/ft710/*.go` (11 production).

Re-run both pins after each file.

- [ ] **Step 6: Delete the package-level builder and parser functions**

Run: `go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 7: Full suite and commit**

Run: `gofmt -l . && go test ./... && go test -race ./core/...`
Expected: all pass.

```bash
git add -A
git commit -m "m9b: task 54 — builders and parsers become dialect methods

The bulk of the migration (~316 call sites). Bodies copied verbatim;
only the receiver and internal call qualification changed.

The frame corpus passes against the UNCHANGED golden file from task 51,
which is this milestone's central claim: the call syntax moved and the
wire bytes did not.

BuildIDRead/BuildAISet/ParseIDAnswer/ParseAIAnswer take a receiver they
do not use, deliberately and with a comment saying so — uniform method
form is what makes M9c a table rather than a re-plumbing."
```

---

### Task 55: Injected, fail-closed allowlist

**Files:**
- Modify: `core/cat/allowlist.go` (`AllowedCommand` becomes a method)
- Modify: `core/transport/engine.go`, `core/transport/errors.go`
- Modify: `core/driver/ft710/ft710.go:155` and the driver's dialect plumbing
- Modify: `core/transport/*_test.go` (6 `NewEngine` call sites)

**Interfaces:**
- Consumes: `Dialect` from Tasks 52–54.
- Produces:
  - `func (d Dialect) AllowedCommand(frame []byte) bool`
  - `type AllowFunc func(frame []byte) bool` in `core/transport`
  - `func NewEngine(p Port, allow AllowFunc, opts ...Option) (*Engine, error)`
  - `var ErrNoAllowlist = errors.New(...)` in `core/transport`

- [ ] **Step 1: Write the failing fail-closed tests**

Create `core/transport/allowfunc_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"testing"
)

// TestNewEngine_NilAllowFuncIsRefused pins the primary fail-closed
// property: an Engine without a gate cannot be constructed at all.
func TestNewEngine_NilAllowFuncIsRefused(t *testing.T) {
	e, err := NewEngine(nopPort{}, nil)
	if err == nil {
		t.Fatal("NewEngine(port, nil) returned no error — an Engine with no allowlist must not be constructable")
	}
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("NewEngine(port, nil) error = %v, want it to wrap ErrNoAllowlist", err)
	}
	if e != nil {
		t.Error("NewEngine(port, nil) returned a non-nil Engine alongside its error")
	}
}

// TestEngineDo_RefusesWithNoAllowlist is the defence-in-depth half: even
// if an Engine somehow reaches Do without a gate, nothing reaches the
// wire. Unreachable through NewEngine by construction — checked anyway,
// exactly as ErrDisallowedCommand's own doc comment argues for the layer
// below this one.
func TestEngineDo_RefusesWithNoAllowlist(t *testing.T) {
	p := &recordingPort{}
	e := &Engine{port: p} // deliberately bypassing NewEngine
	e.initForTest()

	_, err := e.Do(context.Background(), someReadCommand(t), someSpec())
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("Do with a nil allowlist returned %v, want ErrNoAllowlist", err)
	}
	if len(p.written) != 0 {
		t.Errorf("Do with a nil allowlist wrote %q to the port — nothing may reach the wire", p.written)
	}
}
```

Read `core/transport/engine_test.go` first for the existing test helpers (port doubles, command and spec constructors) and use those real names instead of the placeholders `nopPort`, `recordingPort`, `someReadCommand`, `someSpec`, `initForTest`. Do not add new helpers if equivalents exist.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./core/transport/ -run 'TestNewEngine_NilAllowFunc|TestEngineDo_RefusesWithNoAllowlist' -v`
Expected: FAIL to compile — `undefined: ErrNoAllowlist`, and `NewEngine` takes 1 argument not 2.

- [ ] **Step 3: Add `AllowFunc`, the sentinel, and the new constructor**

In `core/transport/errors.go`:

```go
// ErrNoAllowlist means an Engine was asked to transmit with no allowlist
// configured. Distinct from ErrDisallowedCommand deliberately: that one
// means "this frame is not permitted", this one means "this Engine was
// misassembled". Both refuse, but conflating them would have a
// diagnostic blame the frame for a composition bug.
var ErrNoAllowlist = errors.New("transport: engine has no allowlist, refusing to transmit")
```

In `core/transport/engine.go`:

```go
// AllowFunc is the outbound gate: it reports whether frame may be written
// to the radio. It is injected rather than imported so that each driver
// supplies its OWN dialect's allowlist, and so nothing can construct an
// Engine that transmits without one.
//
// cat.Dialect.AllowedCommand has exactly this signature; a driver passes
// the method value directly.
type AllowFunc func(frame []byte) bool

// NewEngine constructs an Engine over p, gated by allow.
//
// FAIL-CLOSED, twice over. A nil allow is refused here, before the read
// goroutine starts, so an ungated Engine cannot exist; and Do re-checks
// before every write, because this is the last defence before a physical
// radio sees these bytes and defence in depth is cheap.
func NewEngine(p Port, allow AllowFunc, opts ...Option) (*Engine, error) {
	if allow == nil {
		return nil, ErrNoAllowlist
	}
	e := &Engine{
		port:       p,
		allow:      allow,
		logger:     nopLogger{},
		clk:        realClock{},
		events:     make(chan readerEvent, 16),
		closeCh:    make(chan struct{}),
		readerDone: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(e)
	}

	go e.readLoop()
	return e, nil
}
```

Add `allow AllowFunc` to the `Engine` struct, and change the check at `engine.go:436`:

```go
		frame := cmd.Bytes()
		if e.allow == nil {
			return nil, ErrNoAllowlist
		}
		if !e.allow(frame) {
			return nil, fmt.Errorf("%w: %s", ErrDisallowedCommand, cmd.String())
		}
```

Update the doc comments at `engine.go:317` and in `core/transport/doc.go` that name `cat.AllowedCommand`: they now describe an injected gate. Keep the safety-obligation-1 wording ("exactly one Bytes() call per transmission, the SAME slice checked and written") — that property is unchanged and load-bearing.

- [ ] **Step 4: Make `AllowedCommand` a dialect method**

In `core/cat/allowlist.go`, change `func AllowedCommand(frame []byte) bool` to `func (d Dialect) AllowedCommand(frame []byte) bool`, and `d.`-qualify its internal calls to the now-moved validators. Preserve every doc comment, especially the EX paragraph recording the M8d no-go.

- [ ] **Step 5: Wire the driver**

In `core/driver/ft710`, give the session a `dialect cat.Dialect` field initialised to `cat.FT710`, and at `ft710.go:155`:

```go
	eng, err := transport.NewEngine(port, dialect.AllowedCommand, engOpts...)
	if err != nil {
		return nil, fmt.Errorf("opening session: %w", err)
	}
```

- [ ] **Step 6: Make `caps.go` read the CAT ID from the dialect**

The design makes the CAT ID single-source. `core/driver/ft710/caps.go:103` currently declares `catID = "0800"` as its own const, and `ft710.go:183` uses it in `driver.WrongRadioError{Want: catID, …}`.

Replace the const with the dialect's value:

```go
// catID is the CAT identity an FT-710 answers "ID;" with. Sourced from
// the codec's dialect rather than restated here, so there is one place
// this string exists — the value M9c's driver registration will read
// too.
var catID = cat.FT710.CATID()
```

Add a test in `core/driver/ft710` pinning the linkage:

```go
// TestCATID_ComesFromTheDialect pins the single-source rule: if someone
// reintroduces a local literal, this fails.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != cat.FT710.CATID() {
		t.Errorf("catID = %q, want cat.FT710.CATID() = %q", catID, cat.FT710.CATID())
	}
	if catID != "0800" {
		t.Errorf("catID = %q, want the FT-710's documented %q — golden vector G1", catID, "0800")
	}
}
```

Run: `go test ./core/driver/ft710/ -run TestCATID_ComesFromTheDialect -v`
Expected: PASS

- [ ] **Step 7: Update the six test call sites**

Each `transport.NewEngine(p, opts...)` in `core/transport/*_test.go` becomes `transport.NewEngine(p, cat.FT710.AllowedCommand, opts...)` with its error checked. If a test needs a permissive gate for a case unrelated to allowlisting, use an explicit local `func([]byte) bool { return true }` with a comment saying why — never a nil.

- [ ] **Step 8: Confirm the allowlist property tests survived in method form**

These already exist and are the gate's real coverage. After the move they must still be present and passing, now as `FT710.AllowedCommand`:

```bash
go test ./core/cat/ -run 'TestAllowedCommand' -v
```

Expected, all passing: `TestAllowedCommand_PropertyEveryBuilderOutput` (every builder's output passes its own gate), `TestAllowedCommand_RejectsGoldenAnswerFrames` (answer-only shapes refused), `TestAllowedCommand_AcceptsAllowlistedSingleFrames` (which holds the audited Set/Answer-indistinguishable exceptions — MT, and MC), `TestAllowedCommand_EXAnswersRejectedOutboundAll296`, and the two `HWDerived_M5b` pairs.

If any of these has *disappeared* rather than been converted, restore it. Count them before and after and put both numbers in the task report.

**Cross-dialect negatives are NOT added here.** With one dialect such a test cannot fail; see the deferred list.

- [ ] **Step 9: Check the EX cross-check still cross-checks**

`core/transport/ex_crosscheck_test.go` calls into `core/cat` and needed migrating in Task 53. Its whole value is that `internal/fakeradio` transcribes Table 2 independently.

```bash
go test ./core/transport/ -run TestEXCrossCheck -v
go test ./internal/fakeradio/ -run TestNoCoreImports -v
```

Expected: both PASS. If `TestNoCoreImports` fails, someone has pointed the fake at `core/cat` and the cross-check is now comparing a value with itself — stop and escalate; this is a correctness-of-evidence failure, not a build error.

- [ ] **Step 10: Run everything**

Run: `go test ./... && go test -race ./core/...`
Expected: PASS, both Task 51 pins included.

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "m9b: task 55 — the outbound allowlist becomes injected and fail-closed

core/transport no longer reaches for cat.AllowedCommand: it takes an
AllowFunc at construction, and cat.Dialect.AllowedCommand matches that
signature so the driver passes the method value with no adapter.

Fail-closed twice: NewEngine refuses a nil allow before starting the read
goroutine, so an ungated Engine cannot exist; and Do re-checks before
every write anyway, because this is the last defence before a physical
radio sees these bytes.

ErrNoAllowlist is distinct from ErrDisallowedCommand on purpose — 'this
frame is not permitted' and 'this Engine was misassembled' are different
faults, and conflating them would have a diagnostic blame the frame for a
composition bug.

core/transport still imports core/cat for the frame accumulator, the '?;'
rejection and the AI init frame. Only the gate is decoupled; the init
frame stays a noted seam."
```

---

### Task 56: Guards, and the formal pin amendment

**Files:**
- Modify: `internal/guards/importgraph_test.go` — **this is the byte-identical-pinned file; this task is where the pin is formally amended**
- Create: `internal/guards/engine_construction_test.go`
- Modify: `internal/guards/simulated_tokens_test.go`
- Modify: `.superpowers/sdd/progress.md` (ledger entry)

**Interfaces:**
- Consumes: the method-form builders from Task 54, the new `NewEngine` from Task 55.
- Produces: no Go symbols outside tests.

**Read first:** the design document's "Guards, and the pin amendment" section. The write-path guard currently requires `sel.X` to be an `*ast.Ident` naming the `core/cat` import (`importgraph_test.go:251-255`). `cat.FT710.BuildMWSet(…)` is a *nested* selector, so that check no longer matches, and `sawDriverBuildMW` will fail with "the walker or its filters are broken, and every check above passed vacuously". That failure is expected and is the reason this task exists.

- [ ] **Step 1: Confirm the guard is failing for the expected reason**

Run: `go test ./internal/guards/ -run TestWritePathReachableOnlyThroughDriver -v`
Expected: FAIL with the `sawDriverBuildMW` vacuity message.

Record the exact message in the task report. If it fails for any *other* reason, stop — something else is wrong.

- [ ] **Step 2: Rewrite the builder matcher**

In `internal/guards/importgraph_test.go`, replace the package-qualified match with a name-only match:

```go
		// (a) BuildMWSet / BuildMTSet, as a method on a cat.Dialect.
		//
		// Matched by METHOD NAME alone, whatever the receiver. Before
		// M9b these were package-level functions and this check required
		// sel.X to be an *ast.Ident naming the core/cat import; the
		// dialect seam made every call a nested selector
		// (cat.FT710.BuildMWSet, or s.dialect.BuildMWSet), which that
		// check silently stopped matching — caught by sawDriverBuildMW
		// below, exactly as intended.
		//
		// Name-only matching is LOOSER but strictly MORE inclusive: it
		// cannot admit a call the old check caught. It is also the same
		// approximation this guard already applies to WriteChannel (see
		// (b) and this test's doc comment), so it is house precedent
		// rather than a new compromise.
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

Update this test's own doc comment (the paragraph at `importgraph_test.go:194`) to describe name-only matching for the builders, matching how it already describes `WriteChannel`.

- [ ] **Step 3: Verify the guard passes and is not vacuous**

Run: `go test ./internal/guards/ -v`
Expected: PASS, all guards.

Then prove it still bites: temporarily add `_ = cat.FT710.BuildMWSet` to a file in `core/clone/`, run the guard, confirm it FAILS naming that file, and revert. Record in the report.

- [ ] **Step 4: Add the NewEngine construction guard**

Create `internal/guards/engine_construction_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"testing"
)

// TestNewEngineReachableOnlyFromDriver pins the other half of M9b's
// fail-closed story. NewEngine now takes the outbound allowlist as a
// parameter, which means whoever calls it CHOOSES THE GATE. That choice
// belongs to the driver layer and nowhere else: a call site in app/ or
// cmd/ could hand it a permissive func and bypass every policy layer
// above it.
//
// core/transport itself is excluded (it defines the thing), as are test
// files, which legitimately construct engines against fake ports.
func TestNewEngineReachableOnlyFromDriver(t *testing.T) {
	files := parseRepo(t)

	sawDriverConstruction := false
	scanned := 0

	for _, pf := range files {
		if inTree(pf.relDir, "core/transport") {
			continue
		}
		scanned++

		ast.Inspect(pf.file, func(n ast.Node) bool {
			sel, isSel := n.(*ast.SelectorExpr)
			if !isSel || sel.Sel.Name != "NewEngine" {
				return true
			}
			if inTree(pf.relDir, "core/driver") {
				sawDriverConstruction = true
				return true
			}
			t.Errorf("%s: references transport.NewEngine — an Engine's allowlist is chosen at construction, so only core/driver/** may construct one", pf.relPath)
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("scanned no files outside core/transport — the walker or its filters are broken, and this check passed vacuously")
	}
	if !sawDriverConstruction {
		t.Error("never saw core/driver/** call transport.NewEngine — the walker or its filters are broken, and this check passed vacuously")
	}
}
```

`parseRepo`, `inTree` and `parsedFile` already exist in `importgraph_test.go` in the same package; do not redefine them.

- [ ] **Step 5: Fold in the duplicate simulated-token guard**

`internal/guards/simulated_tokens_test.go`'s doc comment says the pinned single-driver `TestSimulatedTokenSingleNonTestFileRepoWide` is folded into the data-driven table "when the pin lifts (M9b)". The pin lifts in this task, so: delete `TestSimulatedTokenSingleNonTestFileRepoWide` from `importgraph_test.go`, and update the `PIN-LIFT LEDGER NOTE` in `simulated_tokens_test.go` to record that it happened and when.

Run: `go test ./internal/guards/ -v`
Expected: PASS with four guard tests, not five.

- [ ] **Step 6: Ledger the amendment**

Append to `.superpowers/sdd/progress.md`:

```
**M9b task 56 — importgraph_test.go BYTE-IDENTICAL PIN FORMALLY AMENDED.** The pin held from M3 to M9a and is lifted here, as the M8 roadmap always intended M9b (or M8e, now cancelled) to do. What changed: (1) the Set-frame builder matcher moved from package-qualified (sel.X must be an Ident naming core/cat) to method-name-only, because cat.FT710.BuildMWSet is a NESTED selector the old form silently stopped matching — the sawDriverBuildMW non-vacuity counter caught it, which is exactly what it was added for at M9a; (2) TestSimulatedTokenSingleNonTestFileRepoWide deleted, folded into the data-driven TestSimulatedProfileTokensConfinement per that file's own PIN-LIFT LEDGER NOTE. SEMANTICS NOT WEAKENED: name-only matching is strictly MORE inclusive than package-qualified, so it cannot admit a call the old check caught, and it is the same approximation the guard already applied to WriteChannel. Both changes were verified to still bite by deliberately introducing a violation and seeing the guard fail. New baseline for any future byte-identity check: the initial commit ed728b9 (the 38b3087 baseline no longer exists after the 25/07/2026 history reset).
```

- [ ] **Step 7: Commit**

```bash
git add internal/guards/ .superpowers/sdd/progress.md
git commit -m "m9b: task 56 — guards updated, importgraph pin formally amended

The dialect seam turned every builder call into a nested selector, which
the write-path guard's package-qualified matcher stopped matching. It
failed loudly rather than silently, via the sawDriverBuildMW vacuity
counter added at M9a — the guard telling us it had gone blind, which is
the whole point of that counter.

Matcher is now method-name-only: looser, but strictly more inclusive, so
it cannot admit a call the old form caught, and it is the same
approximation already applied to WriteChannel.

New guard: only core/driver/** may call transport.NewEngine, because
NewEngine now takes the allowlist and whoever calls it chooses the gate.

Retires TestSimulatedTokenSingleNonTestFileRepoWide into the data-driven
table, as that file's own pin-lift note planned. Amendment ledgered with
an explicit not-weakened argument."
```

---

### Task 57: Documentation, ledger, and the milestone gate

**Files:**
- Modify: `core/cat/doc.go` (if present — check), `core/transport/doc.go`
- Modify: `README.md` (repository-layout table's `core/` row, if it describes the codec as FT-710-specific)
- Modify: `.superpowers/sdd/progress.md`
- Create: `.superpowers/sdd/m9b-milestone-summary.md`

- [ ] **Step 1: Update the package documentation**

`core/transport/doc.go` describes `cat.AllowedCommand` as the gate in at least three places (lines ~112, ~133, ~150). Rewrite those to describe the injected `AllowFunc`, keeping every safety-obligation statement intact. Add a paragraph stating plainly what did *not* change: transport still imports `core/cat` for the frame accumulator, `IsRejection`/`ErrRejected` and the AI init frame.

- [ ] **Step 2: Check the README's claim**

Run: `grep -n "core/" README.md | grep -i "cat\|codec\|protocol"`

If the repository-layout table calls `core/cat` the "CAT protocol codec" without qualification, that is still true and needs no change. If it says FT-710-specific, update it to say the codec is dialect-parameterised with one dialect (FT-710) today. Do not claim multi-radio support: there is still exactly one driver.

- [ ] **Step 3: Run the full local gate**

CI is billing-dead; this is the substitute. Run each and record the output in the milestone summary:

```bash
gofmt -l .                                    # expect: empty
go vet ./...
go build ./...
go test ./...
go test ./internal/guards/ -v
go test -race ./core/...
cd app && wails generate module && git diff --exit-code frontend/wailsjs && cd ..
cd app/frontend && npm run check && npm run test && npm run build && cd ../..
go run ./cmd/rigprog probe --fake
go run ./cmd/rigprog read --fake --settings --out /tmp/m9b-e2e.json
go run ./cmd/rigprog version
```

`probe --fake` output must be byte-identical to `main`'s. Capture it on `main` before merging and `diff` the two.

- [ ] **Step 4: Verify both Task 51 pins one final time, explicitly**

Run: `go test ./core/cat/ -run 'TestFrameCorpus_MatchesGolden|TestEvidenceLiterals_CommittedSetSurvives' -v`
Expected: PASS.

State in the milestone summary that `testdata/frame-corpus.golden` and `testdata/evidence-literals.golden` were **never regenerated** during the milestone. Confirm with `git log --oneline -- core/cat/testdata/` — it should show exactly one commit, Task 51's.

- [ ] **Step 5: Write the milestone summary for the Codex review**

Create `.superpowers/sdd/m9b-milestone-summary.md` covering: what changed and why, the two scope departures from the roadmap (data-only dialect; API compatibility dropped) with their reasoning, the pin evidence, the guard amendment with its not-weakened argument, the deferred items (frame-shape variants, cross-dialect negative property tests, the exported dialect constructor), and the full gate output.

- [ ] **Step 6: Ledger and commit**

Append the milestone record to `.superpowers/sdd/progress.md`, then:

```bash
git add -A
git commit -m "m9b: task 57 — docs, ledger, milestone gate"
```

- [ ] **Step 7: Codex adversarial milestone review**

Standing project convention: Codex reviews the milestone before merge. Use repo-relative paths only in the prompt — an absolute path outside the workspace hangs the job silently (this cost 2.5 hours during M8c; see the project memory note). Save the transcript to `.superpowers/sdd/m9b-codex-review.md`, adjudicate every finding explicitly, fix wave, opus re-review, then merge to `main` with `--no-ff`.

---

## Deferred, and ledgered as such

These are **not** oversights. Each is recorded in the design document with its reasoning:

1. **Per-command frame-shape variants** — M9c, where the FTdx10 gives them a second implementation to answer to.
2. **Cross-dialect negative property tests** — M9c. With one dialect they cannot fail, and a test that cannot fail reads as coverage it does not provide.
3. **Slot-space rules as dialect data.** After M9b the dialect owns the slot entry points, but `classifySlotWire` is still hardcoded to the FT-710's 3-digit space. Parameterising it needs a second slot space to answer to — the FTX-1's 5-digit form is the real forcing case.
4. **An exported `Dialect` constructor** — M9c. Roadmap A1's `core/cat/ftdx10` package needs `func Dialect() cat.Dialect`, which unexported fields make impossible from outside the package. M9b has one in-package dialect, so a constructor now would be shaped by guesswork.
5. **Making transport's AI init frame injectable** — roadmap risk 10. A seam noted, not built, until a rig actually differs.
