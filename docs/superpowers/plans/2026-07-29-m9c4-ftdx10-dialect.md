# M9c-4 FTdx10 dialect — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `core/cat/ftdx10` — the FTdx10's dialect and chart-transcribed
EX inventory, proven by conformance, identity/difference pins and
assumption-itemised goldens — plus the four guard/API closures that
constrain it.

**Architecture:** per the spec (revision 2, `docs/superpowers/specs/
2026-07-29-m9c4-ftdx10-dialect-design.md` — read it IN FULL first; it
is the authority on every evidence rule). The plan's distinctive
mechanics are ORCHESTRATION-ENFORCED INDEPENDENCE: Task 2 (ledger),
Task 3 (transcription A), Task 4 (transcription B) and Task 7 (goldens)
are executed by DIFFERENT implementer agents whose briefs are
constructed so B never sees A or the ledger, and the golden session
never sees the codec. The orchestrator, not any implementer,
arbitrates cross-check mismatches against the PDF.

**Tech Stack:** Go 1.25, standard library only.

## Global Constraints

- British English; SPDX on every new `.go` file; `gofmt -l .` silent
  per task; never regenerate any golden (`core/cat/testdata` stays at
  two commits; the new `ftdx10/testdata` gets the same discipline from
  birth); `git diff --exit-code -- core/cat/testdata/
  core/cat/exinventory_gen.go` per task.
- The evidence-literal model: new files are unconstrained; the ONLY
  golden-recorded file this plan touches is via Task 1's rename inside
  `core/cat` NON-test files (unconstrained) and `dialecttest`
  (excluded-package); `mtcombined_test.go` has no golden records
  (M9c-3 precedent). No re-mint.
- The manual is `docs/fixtures-private/manuals/ftdx10_cat_2308-F.pdf`
  (Table 2 on the pages whose footers read 9-12) and
  `ftdx10_layout.txt` (chart at lines 652-899). Layout-text hazards on
  record: floating P1/P2 group labels; wrapped row names with P3 on
  the continuation line ("MOUSE POINTER / 05 / SPEED"); "MY CALL."
  carries a trailing full stop VERBATIM.
- STOP conditions are real: PDF ambiguity in Task 2, a numeric Digits
  > 4 in Task 3/4, cross-check mismatch in Task 5, any reused-command
  frame deviation — all stop-and-report to the orchestrator, never
  resolved by implementer consensus.

## File Structure

| File | Task | Responsibility |
|---|---|---|
| `core/cat/mtcombined.go` + call sites | 1 | `combinedMTSetKind` → exported `CombinedMTSetKind` (rename, no synonym) |
| `core/cat/mtcombined_test.go` | 1 | the incidental-equality test (`== KindVFO`, caveat comment) |
| `core/cat/dialecttest/dialecttest.go` | 1 | call site + comment migrate to the new name |
| `internal/guards/importgraph_test.go` | 1 | fence carve-out narrows to exact `core/cat` + `core/cat/dialecttest` |
| `internal/guards/composition_imports_test.go` | 1 | dialecttest import pin (rule 3); Rule 2 → `core/cat` prefix |
| `core/cat/ftdx10/testdata/group-ledger.md` | 2 | the PDF-derived group-boundary ledger |
| `core/cat/ftdx10/table2.csv` | 3 | transcription A + provenance header (header-vs-chart anomaly recorded) |
| `core/cat/ftdx10/exinventory.go`, `doc.go` | 3 | go:generate directive; package doc; reused-command verification record |
| `internal/extable/profile.go` | 3 | the `"ftdx10"` registration (the spec's exact literal) |
| `core/cat/ftdx10/exinventory_gen.go` | 3 | generated |
| `core/cat/ftdx10/staleness_test.go` | 3 | package-local, registry-selected (`Package == "ftdx10"` only) |
| `core/cat/ftdx10/testdata/transcription-b.csv` | 4 | the semantic tuple, PDF-primary |
| `core/cat/ftdx10/crosscheck_test.go` | 5 | A ≡ B ≡ ledger, full tuple + cardinalities |
| `core/cat/ftdx10/dialect.go` | 6 | the DialectConfig literal + `Dialect()` |
| `core/cat/ftdx10/dialect_test.go` | 6 | conformance, identity pins, difference pins, EX answer bound |
| `core/cat/ftdx10/testdata/*.golden` + `golden_test.go` | 7 | assumption-itemised manual-derived vectors |

---

### Task 1: the four closures (one implementer)

- [ ] Rename `combinedMTSetKind` → `const CombinedMTSetKind byte = '0'`
  (exported, doc comment carrying the M9c-3 schema rationale plus the
  Set-direction "(Fixed)" meaning; NOT defined via `KindVFO`). Replace
  every internal use (`mtcombined.go`, `allowlist.go`); migrate
  `dialecttest.go`'s `cat.KindVFO` call site and its comment. Append to
  `mtcombined_test.go`: `TestCombinedMTSetKindCoincidesWithKindVFO`
  asserting equality WITH the incidental-coincidence caveat comment.
- [ ] Narrow the Set-builder fence: `inTree(relDir, "core/cat")` →
  `relDir == "core/cat" || relDir == "core/cat/dialecttest"`, comment
  updated (the old prefix commentary is now false). Red-proof: transient
  non-test decoy in a new `core/cat/ftdx10/` dir calling
  `d.BuildMTSet(...)` → fence FIRES naming it; delete decoy; green.
- [ ] Composition guard: Rule 2's exact match `core/cat` → prefix match
  (Rule 1's trailing-slash technique), red-proof with a transient
  `app/` decoy importing a `core/cat/x` path; new Rule 3 forbidding
  `.../core/cat/dialecttest` from every walked production file,
  red-proof with a transient production decoy import. Record both
  failing outputs verbatim.
- [ ] Gates: full `go test ./core/cat/... ./internal/guards/`, gofmt,
  golden diffs. Commit: `M9c-4 task 1: CombinedMTSetKind exported by
  rename; three guard closures with red-proofs`.

### Task 2: the group-boundary ledger (implementer L — PDF only)

Brief contains ONLY: the PDF path, the target file, the format. NO
layout text, NO other task's output.

- [ ] Read the PDF's Table 2 pages VISUALLY (the Read tool renders
  them). For every P1 group and every P2 subgroup: label text, first
  (P3, name), last (P3, name), row count, PDF page number, and the
  visual anchor that fixes the boundary (the ruled cell the label sits
  in). Sum the per-group counts to a total. Record any point where the
  ruled table is genuinely ambiguous as a STOP finding instead of
  choosing.
- [ ] Write `core/cat/ftdx10/testdata/group-ledger.md` with a
  provenance header (manual 2308-F, PDF pages, date, method:
  visual-from-rendered-PDF, nothing else consulted). Commit:
  `M9c-4 task 2: the PDF-derived group-boundary ledger`.
- [ ] Orchestrator check (not the implementer): the ledger's total is
  expected to be 197 (two independent review counts); a different
  total is a STOP-and-arbitrate, not an edit.

### Task 3: transcription A + profile + generation (implementer A)

- [ ] Transcribe the chart layout-text-led, PDF-checked, into
  `core/cat/ftdx10/table2.csv` — the extable 10-column shape
  (p1,p2,p3,p1_label,p2_label,name,p4,digits,text,manual_line;
  manual_line = the layout-text line; P4 verbatim as audit trail;
  names VERBATIM including "MY CALL."'s full stop; the wrapped-name
  rows joined correctly). Provenance header records: manual 2308-F,
  layout-led-PDF-checked method, date, AND the header-vs-chart anomaly
  (header "P1: 01-05"/"P3: 01-23" vs chart P1 01-04 / P3 max 21, with
  the FT-710 precedent pointer).
- [ ] Register the `"ftdx10"` profile in `internal/extable/profile.go`
  — the spec's exact literal (`ExpectedRows: 197` FROM THE LEDGER —
  read it; if the CSV's row count differs from the ledger, STOP).
  Append registry tests for the new entry (both-profiles enumeration,
  ftdx10 lookup) to `profile_test.go`.
- [ ] `core/cat/ftdx10/exinventory.go` (`//go:generate go run
  .../internal/extable/gen -profile ftdx10`), `doc.go` (provenance,
  ASSUMED register skeleton, the reused-command verification record:
  MC/ID/AI/MR/MW/EX-grammar tables each checked against the manual
  section with layout line refs — any deviation is a STOP), run
  `go generate ./core/cat/ftdx10`, commit the generated file.
- [ ] `staleness_test.go`: registry-selected (`RegisteredProfiles()`
  filtered to `Package == "ftdx10"`, must find EXACTLY one), re-derive,
  byte-compare. The FT-710's staleness test untouched.
- [ ] Gates + commit: `M9c-4 task 3: transcription A, the ftdx10
  extable profile, and the generated inventory`.

### Task 4: transcription B (implementer B — PDF ONLY, blind)

Brief contains ONLY: the PDF path, the output path and CSV shape, the
verbatim-names rule, the STOP conditions. **It must NOT contain or
reference table2.csv, the generated file, the ledger, or any row
count.** B works from the rendered PDF's ruled cells.

- [ ] Produce `core/cat/ftdx10/testdata/transcription-b.csv`:
  `p1,p2,p3,p1_label,p2_label,name,digits,text` (the semantic tuple —
  no P4, no manual_line), one row per chart cell row, provenance
  header (PDF-primary visual method, pages, date, nothing else
  consulted). Digits > 4 on a numeric row = STOP finding in the
  report, still recorded in the CSV as seen.
- [ ] Commit: `M9c-4 task 4: transcription B — the PDF-primary
  semantic tuple`.

### Task 5: the cross-check (implementer, after 2+3+4)

- [ ] `crosscheck_test.go`: parse A (via `extable.ParseCSV` with the
  registered profile), parse B (file-local parser), parse the ledger's
  per-group cardinalities (light regexp over the committed md). Bind:
  A ≡ B on the FULL tuple per address (labels, name, digits, text —
  exact string equality); address sets equal; per-(P1,P2) cardinalities
  of BOTH equal the ledger's; totals equal `ExpectedRows`; the text-row
  set is exactly {(4,1,1)}-or-whatever-the-agreed-address-is for MY
  CALL. — derived from the data, not hardcoded beyond the count 1.
- [ ] **On ANY mismatch: STOP.** Report the exact rows to the
  orchestrator, who arbitrates against the PDF and directs the fix to
  A or B (never both silently).
- [ ] Gates + commit: `M9c-4 task 5: the A/B/ledger cross-check binds
  the full tuple`.

### Task 6: the dialect + pins (implementer, after 5)

- [ ] `dialect.go`:

```go
var dialect = cat.MustNewDialect(cat.DialectConfig{
    CATID: "0761",
    ModeNames: modeNames, // fresh transcription, 1..F per the manual;
    // ModeUnset '0' ("-") included as an ASSUMED member — absent from
    // every FTdx10 legend; parsers must accept the placeholder.
    Slots: cat.SlotSpace{
        MemoryLo: 1, MemoryHi: 99,
        SixtyLo: 501, SixtyHi: 599,
        PMSPairs: 9,
        EmergencyWire: "EMG",
        NoneWire: "000", // ASSUMED — appears in no FTdx10 slot legend
    },
    EXItems: exItems, // the generated inventory
    MT: cat.MTPolicy{Form: cat.MTFormCombined, TagMaxBytes: 12,
        TagFill: ' ' /* ASSUMED */},
    Clarifier: cat.ClarifierPolicy{StepHz: 10 /* ASSUMED — no step
        stated anywhere in the manual; 9990 range supports-not-proves */,
        MaxAbsHz: 9990},
    MWWriteKind: cat.CombinedMTSetKind, // the FTdx10's MW P7 "(Fixed)"
    // — equal to the combined MT Set constant AS A FACT OF THIS RADIO,
    // not a rule; see the difference pins.
})

func Dialect() cat.Dialect { return dialect }
```

  (Exact ASSUMED comments per the spec; every field explicit.)
- [ ] `dialect_test.go`: `dialecttest.Run(t, Dialect())`; mode identity
  (256-sweep `ValidMode`/`ModeName` vs `cat.FT710`, `ModeByName`
  round-trips, ASSUMED members noted); slot identity (behavioural:
  `ParseSlot` over 000-999 + P1L..P9U + EMG wire forms, acceptance and
  `Wire()` equality vs `cat.FT710`; constructors; NO `Slot.IsXxx`);
  difference pins (`CATID`, `MWWriteKind() == cat.CombinedMTSetKind`
  with caveat, `MTForm`, `MTAnswerBounds() == (41,41)`,
  `KnownEXAddress` disjointness BOTH directions per the spec's
  chart-true addresses); the EX answer bound (independent
  `max(Digits)` == 12 from `EXItems()`, a 12-byte P4 answer accepted
  via `ParseEXAnswer`, 13 rejected).
- [ ] Gates + commit: `M9c-4 task 6: the FTdx10 dialect, conformance,
  and the identity/difference pins`.

### Task 7: goldens (implementer G — separate session, PDF + spec-free)

Brief contains: the PDF path, the manual's Answer-chart location (the
pages), the vector classes wanted, the provenance-header requirements,
the output paths. **NOT: mtcombined.go, the generated inventory, this
plan's offset arithmetic, or the spec.** G derives every byte from the
manual's position charts alone.

- [ ] Hand-derive: combined-MT Answer vectors (≥3: a full-width tag, a
  short tag padded — the padding bytes marked INHERITED-ASSUMED — and a
  cleared tag), MR/MW vectors, EX read vectors for 3 sample addresses,
  each with the per-class provenance itemisation the spec requires.
  Write `testdata/*.golden` + the header.
- [ ] `golden_test.go` (may be written by the SAME implementer AFTER
  the vectors are frozen): build/parse each vector through the dialect;
  gate-admissibility for the Set-direction ones; byte equality.
  A mismatch between a golden and the codec is a STOP (either the
  derivation or the codec misreads the manual — orchestrator
  arbitrates).
- [ ] Gates + commit: `M9c-4 task 7: assumption-itemised manual-derived
  goldens`.

### Task 8: full gate + byte-identity re-run

- [ ] Full local gate (gofmt/build/vet/full suite/guards -v/golden
  diffs); the FT-710 byte-identity quick re-run (probe/read from HEAD
  vs the M9c-3 manifest hashes — expected trivially identical; note in
  a short section appended to the M9c-3 manifest? NO — write
  `docs/superpowers/m9c4-baseline-note.md` recording the check and its
  scope). Commit.

## Self-review

Spec coverage: closures→T1; ledger→T2; A+profile+generation+staleness→
T3; B→T4; cross-check→T5; dialect+pins+EX bound→T6; goldens→T7;
gate→T8. Independence enforced by brief construction (L, A, B, G
disjoint agents; B and G blind). Orchestrator arbitration points: T2
total ≠ 197, T5 mismatches, T7 golden-vs-codec mismatch, any STOP.
Ordering: T1 independent; T2 before T3 (ExpectedRows) and T5; T3/T4
parallel-safe (different agents, disjoint outputs) after T2; T5 after
3+4; T6 after 5; T7 after 6 (needs the dialect for golden_test only —
the VECTORS need nothing); T8 last. Placeholders: none; the
transcription contents are the work, not placeholders.
