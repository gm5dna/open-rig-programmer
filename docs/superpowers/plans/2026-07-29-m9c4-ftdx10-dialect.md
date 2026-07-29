# M9c-4 FTdx10 dialect — Implementation Plan (revision 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Revision 2 (29/07/2026).** Revision 1 was reviewed by Codex
> (NEEDS-REVISION, 11 findings, 1 CRITICAL) and Fable (NEEDS-REVISION,
> 9 findings, 2 HIGH); adjudication in
> `.superpowers/sdd/m9c4-plan-review-adjudication.md`. The blockers:
> the independence machinery was QUARANTINE THEATRE (the plan and spec
> themselves contain the 197 count and the tag facts, and the sub-skill
> flow hands implementers the plan by default), and the Rule 2 guard
> edit as directed would have silently dropped the bare `core/cat`
> match. Both redesigned below. Do not implement revision 1.

**Goal:** `core/cat/ftdx10` — the FTdx10's dialect and chart-transcribed
EX inventory, proven by conformance, identity/difference pins and
assumption-itemised goldens — plus the four guard/API closures.

**Spec:** `docs/superpowers/specs/2026-07-29-m9c4-ftdx10-dialect-design.md`
(revision 2) — the authority on every evidence rule. **Only the
orchestrator and the Task 1/3/5/6/8 implementers read it (or this
plan).**

## THE QUARANTINE (the central mechanism — read first)

Tasks 2 (ledger), 4 (transcription B) and 7a (golden vectors) are
executed by fresh-context agents — **L**, **B** and **G** — from
SELF-CONTAINED briefs the orchestrator writes. The quarantine is
MATERIAL, not instructional:

- **L, B and G never open the repository.** Each brief contains: the
  PDF's ABSOLUTE path (`docs/fixtures-private/manuals/
  ftdx10_cat_2308-F.pdf` under the repo root — the one permitted
  in-repo read), an output path in the session SCRATCHPAD, the exact
  output format, the verbatim-transcription rules, and the STOP
  conditions. Nothing else.
- **Forbidden to L, B and G, stated in each brief:** this plan, the
  spec, the adjudications, `ftdx10_layout.txt`, any other repository
  file, any row count, any prior task's output, and any orchestration
  history. Their briefs contain no counts and no addresses.
- **The orchestrator moves each scratchpad output into the tree,
  reviews it for format only (never "corrects" content), and commits
  it.** Hashes are recorded at commit time; any later change to a
  quarantined artefact is a STOP.
- L, B and G are DISJOINT agents, and disjoint from every other task's
  implementer.

## Global Constraints

- British English; SPDX on every new `.go` file; `gofmt -l .` silent
  per task; never regenerate any golden.
- Per-task mechanical gates, EVERY task from its creation onward:
  `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go`
  AND (once they exist) `git diff --exit-code --
  core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go` —
  the new artefacts get the never-regenerate discipline from birth.
- Per-task test commands are enumerated in each task (Codex 10) — no
  undefined "gates".
- Evidence-literal model: new files unconstrained; `mtcombined_test.go`
  has no golden records (M9c-3 precedent); no re-mint.
- STOPs are real and orchestrator-arbitrated: PDF ambiguity (Task 2), a
  numeric Digits > 4 (Tasks 3/4), the Task 3 preflight mismatch, any
  cross-check mismatch (Task 5), any golden-vs-codec mismatch (7b),
  any reused-command frame deviation (Task 3). **Arbitration may
  correct A, B, OR THE LEDGER** — never by consensus, always against
  the PDF, and A is never edited merely to satisfy `ExpectedRows`.
- Tasks execute SEQUENTIALLY (one shared worktree; no parallel
  commits — Codex 9).

## File Structure

| File | Task | Responsibility |
|---|---|---|
| `core/cat/mtcombined.go`, `core/cat/mtcombined_test.go`, `core/cat/dialecttest/dialecttest.go` | 1 | the `CombinedMTSetKind` rename (rg-driven — see task) |
| `internal/guards/importgraph_test.go` | 1 | fence carve-out → exact two-package set; BOTH comment blocks updated |
| `internal/guards/composition_imports_test.go` | 1 | Rule 2 → bare-OR-prefix; Rule 3 (dialecttest forbidden in production) |
| `docs/superpowers/m9c4-red-proofs.md` | 1 | the durable red-proof record (Codex 8) |
| `core/cat/ftdx10/testdata/group-ledger.csv` (+ `.md` companion) | 2 | the PDF-derived ledger — CSV authoritative, prose companion |
| `core/cat/ftdx10/table2.csv` | 3 | transcription A + provenance header |
| `internal/extable/profile.go` (+ `profile_test.go`) | 3 | the `"ftdx10"` registration |
| `core/cat/ftdx10/exinventory.go`, `doc.go`, `exinventory_gen.go`, `staleness_test.go` | 3 | directive; doc + reused-command record; generated; package-local staleness |
| `core/cat/ftdx10/testdata/transcription-b.csv` | 4 | the semantic tuple, PDF-primary |
| `core/cat/ftdx10/crosscheck_test.go` | 5 | A ≡ B ≡ ledger, full legs |
| `core/cat/ftdx10/dialect.go`, `dialect_test.go` | 6 | the literal; conformance; pins; EX answer bound |
| `core/cat/ftdx10/testdata/*.golden` | 7a | vectors (quarantined G) |
| `core/cat/ftdx10/golden_test.go` | 7b | the mechanical byte-compare tests |
| `docs/superpowers/m9c4-baseline-note.md` | 8 | the FULL 16-artefact byte-identity re-run |

---

### Task 1: the four closures (one implementer; reads plan+spec)

- [ ] **The rename, rg-driven** (revision 1's enumeration was wrong in
  both directions — `allowlist.go` has NO occurrence;
  `mtcombined_test.go` has many): `rg -n 'combinedMTSetKind' core/cat`
  finds every site; rename all (code, comments, tests) to the exported
  `const CombinedMTSetKind byte = '0'` (doc comment per the spec: the
  M9c-3 schema rationale + Set-direction "(Fixed)" meaning; NOT defined
  via `KindVFO`); migrate `dialecttest.go`'s `cat.KindVFO` call site
  and comment; end-gate `rg 'combinedMTSetKind' core/cat` returns
  NOTHING. Append `TestCombinedMTSetKindCoincidesWithKindVFO` (equality
  + the incidental-coincidence caveat) to `mtcombined_test.go`.
- [ ] **The fence carve-out**: at `importgraph_test.go:327`,
  `inTree(relDir, "core/cat")` →
  `relDir == "core/cat" || relDir == "core/cat/dialecttest"`; update
  BOTH comment blocks that assert prefix semantics (the doc comment at
  ~:229-251 AND the inline one at ~:296-326 — Fable 2).
- [ ] **Rule 2**: at `composition_imports_test.go:100`, the exact form
  is `rel == catPath || strings.HasPrefix(rel, catPath+"/")` — **NOT**
  "Rule 1's trailing-slash technique", which deliberately excludes the
  bare path (correct for `core/driver`, a regression here: bare
  `core/cat` must STAY forbidden).
- [ ] **Rule 3**: forbid
  `github.com/gm5dna/open-rig-programmer/core/cat/dialecttest` from
  every walked production file (the walk excludes `_test.go`, so
  ftdx10's test import stays legal).
- [ ] **Red-proofs, FIVE decoys, all recorded verbatim in
  `docs/superpowers/m9c4-red-proofs.md`** (source, command, exact
  failing output, deletion, green re-run): (a) non-test
  `core/cat/ftdx10` decoy calling `d.BuildMTSet` → fence fires; (b)
  `app/` decoy importing bare `core/cat` → Rule 2 fires; (c) `app/`
  decoy importing `core/cat/ftdx10` → Rule 2 fires (the prefix half);
  (d) production decoy importing `core/cat/dialecttest` → Rule 3
  fires; (e) confirm `dialecttest`'s real builder calls still pass
  under the narrowed fence (green, no decoy).
- [ ] Tests: `go test ./core/cat/... ./internal/guards/ -count=1`;
  gofmt; golden diffs. Commit:
  `M9c-4 task 1: CombinedMTSetKind exported by rename; three guard
  closures, five red-proofs recorded`.

### Task 2: the group-boundary ledger (quarantined L)

- [ ] Orchestrator writes L's self-contained brief per THE QUARANTINE:
  PDF absolute path; scratchpad output path; the EXACT format —
  **`group-ledger.csv`** with header
  `p1,p2,p1_label,p2_label,first_p3,first_name,last_p3,last_name,row_count,pdf_page,visual_anchor`
  (one row per (P1,P2) subgroup, labels/names VERBATIM including
  punctuation, from the rendered PDF's ruled cells only) plus a short
  `.md` prose companion (method, date, manual revision, pages,
  nothing-else-consulted statement, any STOP findings); ambiguity in
  the ruled table = STOP finding, never a choice.
- [ ] Orchestrator: move both files into `core/cat/ftdx10/testdata/`,
  format-review only, commit
  (`M9c-4 task 2: the PDF-derived group-boundary ledger`), record
  SHA-256s. THEN (orchestrator, not L): sum `row_count` — the expected
  total is 197 (three independent derivations on record); anything
  else is a STOP-and-arbitrate against the PDF before Task 3 launches.

### Task 3: transcription A + profile + generation (implementer A2 — reads plan+spec, NOT the ledger)

- [ ] Transcribe layout-text-led, PDF-checked, into
  `core/cat/ftdx10/table2.csv` (the extable 10-column shape; names
  VERBATIM; wrapped rows joined; provenance header with the
  header-vs-chart anomaly per the spec). **A2 does not open the
  ledger** (Fable 5) — the orchestrator's brief hands A2 the single
  number `ExpectedRows: 197` extracted from it.
- [ ] **The preflight, explicit and BEFORE generation** (Codex 5):
  a brief-mandated command counts A's data rows
  (`grep -vc '^#' table2.csv`) and compares to 197. Mismatch = STOP —
  report, do not touch the CSV. Only then: register the `"ftdx10"`
  profile (the spec's exact literal), append registry tests to
  `profile_test.go`, write `exinventory.go` (directive) and `doc.go`
  (provenance; ASSUMED register; **the reused-command verification
  record**: MC/ID/AI/MR/MW/EX-grammar each checked against its manual
  section with layout-line refs; any frame deviation = STOP), run
  `go generate ./core/cat/ftdx10`, commit the generated file. A
  RenderGo row-count refusal at this point IS the preflight STOP
  having been missed — do not add or remove rows to satisfy it.
- [ ] `staleness_test.go`: registry-selected
  (`RegisteredProfiles()` filtered to `Package == "ftdx10"`, exactly
  one), re-derive, byte-compare.
- [ ] Tests: `go test ./internal/extable/ ./core/cat/ftdx10/ -count=1`;
  gofmt; golden diffs. Commit: `M9c-4 task 3: transcription A, the
  ftdx10 extable profile, and the generated inventory`.

### Task 4: transcription B (quarantined B)

- [ ] Orchestrator writes B's self-contained brief per THE QUARANTINE:
  PDF absolute path; scratchpad output path; the CSV shape
  `p1,p2,p3,p1_label,p2_label,name,digits,text` (semantic tuple, no
  P4, no line refs); verbatim-names rule (punctuation included);
  work from the rendered PDF's ruled cells ONLY; a numeric digits
  value above 4 = STOP finding recorded in the report AND transcribed
  as seen. **No counts, no addresses, no other files.**
- [ ] Orchestrator: move to
  `core/cat/ftdx10/testdata/transcription-b.csv`, format-review only,
  commit (`M9c-4 task 4: transcription B — the PDF-primary semantic
  tuple`), record SHA-256.

### Task 5: the cross-check (implementer; reads plan+spec)

- [ ] `crosscheck_test.go`: parse A via
  `extable.ParseCSV(profile, ...)`; parse B and the LEDGER CSV with
  `encoding/csv` (no regexp over prose — Codex 3). Bind ALL legs:
  - A ≡ B per address on the FULL tuple (labels, name, digits, text —
    exact strings); address sets equal BOTH directions.
  - For A and for B against the ledger: (P1,P2) group-key sets equal
    BOTH directions; per-group `row_count` equal; per-group labels
    equal; per-group first/last `(P3, name)` equal.
  - Totals: A == B == ledger sum == the profile's `ExpectedRows`.
  - **The text-row pin, fully hardcoded** (Codex 4; revision 1's
    "or-whatever" was a placeholder): exactly one `text=true` row in
    EACH of A and B, at address (04,01,01), `p1_label` "DISPLAY
    SETTING", `p2_label` "DISPLAY", `name` "MY CALL." (verbatim,
    trailing full stop), `digits` 12 == the profile's TextWidth.
- [ ] ANY mismatch = STOP: report exact rows; the orchestrator
  arbitrates against the PDF and directs the fix to **A, B, or the
  ledger** (Fable 3), never both/all silently; a ledger fix re-runs
  Task 2's total check.
- [ ] Tests: `go test ./core/cat/ftdx10/ -count=1`; gofmt; golden
  diffs. Commit: `M9c-4 task 5: the A/B/ledger cross-check binds every
  leg`.

### Task 6: the dialect + pins (implementer; reads plan+spec)

- [ ] `dialect.go` — the literal, every field explicit INCLUDING the
  required zeros (Codex 7 / Fable 7):

```go
var dialect = cat.MustNewDialect(cat.DialectConfig{
    CATID:     "0761",
    ModeNames: modeNames, // fresh transcription, 1..F per the manual;
    // '0' (ModeUnset, "-") included as an ASSUMED member — absent
    // from every FTdx10 legend; parsers must accept the placeholder.
    Slots: cat.SlotSpace{
        MemoryLo: 1, MemoryHi: 99,
        SixtyLo: 501, SixtyHi: 599,
        PMSPairs:      9,
        EmergencyWire: "EMG",
        NoneWire:      "000", // ASSUMED — in no FTdx10 slot legend
    },
    EXItems: exItems,
    MT: cat.MTPolicy{
        Form:         cat.MTFormCombined,
        TagMaxBytes:  12,
        ClearTagByte: 0,   // must be 0 under MTFormCombined (V9)
        PadByte:      0,   // must be 0 under MTFormCombined (V9)
        TagFill:      ' ', // ASSUMED — the FTdx10's padding byte has
                           // never been observed
    },
    Clarifier: cat.ClarifierPolicy{
        StepHz:   10, // ASSUMED — no step stated anywhere in the
                      // manual; the 9990 range supports, not proves
        MaxAbsHz: 9990,
    },
    MWWriteKind: cat.CombinedMTSetKind, // the FTdx10's MW P7
    // "(Fixed)" — equal to the combined MT Set constant AS A FACT OF
    // THIS RADIO, not a rule; see the difference pins.
})

func Dialect() cat.Dialect { return dialect }
```

- [ ] `dialect_test.go`: `dialecttest.Run(t, Dialect())`; mode identity
  (256-sweep `ValidMode`/`ModeName` vs `cat.FT710`; `ModeByName`
  round-trips; ASSUMED members noted); slot identity (behavioural:
  `ParseSlot` over 000-999 + P1L..P9U + "EMG", acceptance and `Wire()`
  equality vs `cat.FT710`; the constructors; NO `Slot.IsXxx`);
  difference pins (`CATID() == "0761"`;
  `MWWriteKind() == cat.CombinedMTSetKind` with the caveat;
  `MTForm() == cat.MTFormCombined`; `MTAnswerBounds()` = (41, 41,
  nil); `KnownEXAddress` disjointness both directions per the spec's
  chart-true addresses); the EX answer bound (independent
  `max(Digits)` from `EXItems()` == 12; a 12-byte P4 answer accepted
  via `ParseEXAnswer`; 13 rejected).
- [ ] Tests: `go test ./core/cat/ftdx10/ ./core/cat/... -count=1`;
  gofmt; golden diffs. Commit: `M9c-4 task 6: the FTdx10 dialect,
  conformance, and the identity/difference pins`.

### Task 7a: golden vectors (quarantined G) · 7b: the tests

- [ ] **7a** — orchestrator writes G's self-contained brief per THE
  QUARANTINE: PDF absolute path; the Answer-chart guidance (the MT
  ANSWER position chart is the clean one; the Set chart interleaves
  with its legend); the vector classes (≥3 combined-MT Answer vectors:
  full-width tag, short tag padded — padding bytes marked
  INHERITED-ASSUMED — and a cleared tag; MR and MW vectors; 3 EX read
  vectors); the per-class provenance-itemisation requirements from the
  spec (manual rev/page/direction; session/date; no-code-consulted;
  manual-documented vs inherited-assumed bytes; UNVERIFIED status; the
  Stage R capture that lifts each). Scratchpad out; orchestrator moves
  to `core/cat/ftdx10/testdata/*.golden`, commits
  (`M9c-4 task 7a: assumption-itemised manual-derived golden
  vectors`), **records each file's SHA-256 in the commit message**
  (Codex 2 — the mechanical freeze).
- [ ] **7b** — a separate, code-aware implementer writes
  `golden_test.go`: parse/build each vector through the dialect;
  gate-admissibility for Set-direction vectors; byte equality. **It
  may not modify any vector** — the per-task diff gate over
  `ftdx10/testdata/` enforces this mechanically from 7a's commit
  onward. A golden-vs-codec mismatch = STOP (the derivation or the
  codec misreads the manual — orchestrator arbitrates).
  Tests: `go test ./core/cat/ftdx10/ -count=1`; gofmt; ALL golden
  diffs. Commit: `M9c-4 task 7b: the golden byte-compare tests`.

### Task 8: full gate + the FULL byte-identity re-run

- [ ] Full local gate: gofmt; `go build ./...`; `go vet ./...`; full
  `go test ./... -count=1` foreground; `go test ./internal/guards/ -v`
  (all guards by name); every golden diff gate.
- [ ] **The COMPLETE M9c-3 manifest recipe** (Codex 11 — not just
  probe/read): all 16 artefacts (probe/read/CHIRP-import/CSV-export ×
  stdout/stderr/exit/files, with the read_at normalisation), compared
  against the M9c-3 manifest's recorded hashes. Record in
  `docs/superpowers/m9c4-baseline-note.md`. Expected: trivially
  identical; any difference is a STOP.
- [ ] Commit the note.

## Self-review

Coverage: closures→T1; ledger→T2; A+profile+generation+staleness→T3;
B→T4; cross-check→T5; dialect+pins+EX bound→T6; vectors→7a;
golden tests→7b; gate+full byte-identity→T8. Quarantine: L/B/G never
open the repo; A2 never opens the ledger; G's freeze is hash-recorded;
every arbitration names all three fixable artefacts. Ordering: strictly
sequential; T2 before T3 (the extracted total) and T5; T4 after T3
(sequential worktree — blindness is material, not ordering-dependent);
T5 after 3+4; T6 after 5; 7a any time after T2 (needs nothing but the
PDF), 7b after T6; T8 last. Placeholders: none — the text-row pin is
fully hardcoded, the ledger format is pinned, every gate is an
enumerated command.
