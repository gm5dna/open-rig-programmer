# M9c-5 registration enablers — Implementation Plan (revision 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Revision 2 (29/07/2026).** Revision 1 was reviewed by Codex
> (NEEDS-REVISION: 2 CRITICAL, 5 HIGH) and Fable (NEEDS-REVISION:
> 1 CRITICAL, 2 HIGH); adjudication in
> `.superpowers/sdd/m9c5-plan-review-adjudication.md`. The convergent
> CRITICAL: the E1 task split could not compile as sequenced. The
> adjudicated shape (Fable's, with Codex's safety objection closed
> inside it) is below: Task 1 carries the type flip PLUS the repo-wide
> mechanical propagation PLUS the driver's safety refusal, ending fully
> green; Tasks 2-5 keep genuine semantic REDs. Codex's second CRITICAL:
> Task 11's manifest procedure was not executable (`import-out.json`
> never existed — the historical import leg exits 3 writing nothing);
> rewritten. Do not implement revision 1.

**Goal:** the six enablers of
`docs/superpowers/specs/2026-07-29-m9c5-registration-enablers-design.md`
(revision 2 + the Task-11 obligation correction — READ IT IN FULL).

## Global Constraints

British English; SPDX on new files; `gofmt -l .` silent per task;
`git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go
core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go` per task
(`core/cat` is untouched — any diff under it is a STOP); no golden
regeneration; no push; every task's named test commands green at its
commit. **Frontend staleness is sanctioned mid-branch** (recorded): at
Tasks 1-4 the wailsjs `boolean` typing and the JS usage are coherently
wrong together (svelte-check/vitest green); runtime GUI incoherence
exists only if `wails dev` is run mid-branch; Task 5 ends it.

## Verified global facts (for the implementers)

As revision 1's list, plus: both legacy loaders use
`DisallowUnknownFields` recursively (`file.go:405,440`); `loadV1`
decodes v1 channels into the CURRENT struct (`file.go:424-429`);
`inertBlockReason`'s shape and the `"; "` parts join (`diff.go:246-259`,
`:475`); `NewEngine` has ONE production call (`ft710.go:186`) and TEN
test call expressions (`allowfunc_test.go:28,59,85,123`,
`engine_test.go:31`, `engine_lifecycle_test.go:46,134,376,414,457`);
`Journal.Append` json.Marshals the map it is given (a raw
`[]WriteStep` would emit Go field names — the journal must PROJECT);
the historical import leg exits 3 and writes NO output file.

---

### Task 1: E1 lands compilable — the flip, the propagation, the safety refusal

**One commit, fully green (`go build ./...`, `go test ./... -count=1`,
frontend svelte-check/vitest/build).** Contents:

- **The type flip**: `ChannelData.TagDisplay codeplug.BoolField`
  (`json:"tag_display"` — NO omitempty; the field is always an object
  now).
- **Schema 3 + frozen legacy shapes, COMPLETE** (Codex 2): frozen
  strict decode structs for v1 AND v2 — channel wrapper
  (`slot`,`data`), channel data with EVERY current key
  (`freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,
  tag_display *bool,scan_skip`), the v1 top level with raw `menus`, the
  v2 top level with typed `menus`. The two versions MAY share one
  channel-data shape (documented: their channel shapes are provably
  identical today). Live leaf types (`ToneField`, `BoolField` for
  scan_skip, `RadioInfo`, `MenuSnapshot`) are reused — recorded as
  acceptable BECAUSE this milestone does not change them; embedding
  the live `ChannelData` is FORBIDDEN (un-freezes the struct).
  Migration rule: present bool (true OR false) → `{Known, v}`; absent
  (nil) → `{Known, false}` with the spec's behaviour-preservation
  justification in the comment. Duplicate-slot/menu validation
  preserved; a test proves an actually-unknown legacy field is still
  rejected; `ErrSchemaTooNew` for 4.
- **`Validate`**: `TagDisplay.Valid()` slots directly after
  `ScanSkip.Valid()` (the fixed issue order — doc comment updated);
  refuses `{Unknown, true}`/`{Unavailable, true}`.
- **The mechanical propagation, behaviour-preserving** (each site with
  a one-line comment where non-obvious):
  - `core/driver/ft710/read.go:243` → `codeplug.BoolField{State:
    codeplug.Known, Value: display}`.
  - `core/driver/ft710/write.go`: **THE SAFETY REFUSAL LANDS HERE, NOT
    IN TASK 3** (the adjudication's closure of Codex's staging
    objection): `TagDisplay.Valid()` joins `WriteChannel`'s FieldState
    sanity block, and at the TOP of `buildWriteCommands`, immediately
    after `data := *ch.Data`, `data.TagDisplay.State != codeplug.Known`
    → `WriteRefusedError{Fields: []spec.Field{spec.FieldTagDisplay}}`
    before ANY other field mapping; only then `data.TagDisplay.Value`
    flows to `BuildMTSet`. From this commit no path sends non-Known.
  - `core/csvio/export.go:134` → `yesEmpty(d.TagDisplay.Value)`
    (interim — Task 4 replaces the spelling); `import.go:325-329` →
    the parsed bool wrapped `{Known, v}` (interim).
  - `core/csvio/chirp.go` → explicit `TagDisplay:
    codeplug.BoolField{State: codeplug.Known, Value: false}` with the
    comment naming it the pre-E1 manufactured false (Fable's trap:
    without this, chirp compiles into invalid `{State:""}` and every
    import fails the new Validate). Task 4's RED flips it to Unknown.
  - **The repo-wide fixture audit** (Codex 3 — bigger than
    `.TagDisplay` greps): EVERY `ChannelData{` composite literal gains
    an explicit state by provenance — radio-read/hand-built-as-real →
    `{Known, v}` preserving the old bool; CHIRP-derived fixtures →
    keep Known-false at this task (they flip with Task 4). The gate is
    `grep -rn "ChannelData{" --include=*.go .` — every hit either sets
    TagDisplay explicitly or is a deliberate zero-value (listed in the
    report). Inspected-no-change sites recorded in the commit message:
    `internal/fakeradio` (its TagDisplay is wire state, not
    ChannelData), `app/convert.go`, `app/uispec.go` bankCoreFields,
    `internal/csvmerge` production code.
- Tests: the migration matrix (v1/v2 × present-true/present-false/
  absent), schema-3 round-trips (all four states), the Validate
  refusals, the driver refusal unit test, unknown-legacy-field
  rejection.

Commit: `M9c-5 task 1: TagDisplay is a BoolField — schema 3, frozen
legacy migration, repo-wide propagation, and the pre-wire refusal`.

### Task 2: E1b — the plan-time gate in Diff; clone verifies mutual knowledge

- **The block is a NEW ORDERED GATE** (Codex 4): after the bank/erase
  gates, BEFORE the generic per-field aggregation, Diff reads
  `caps.FieldSupport(bankID, spec.FieldTagDisplay).Write` DIRECTLY
  (once TagDisplay leaves `touched` on non-Known, the generic loop no
  longer consults it — the read must be explicit); if
  `TagDisplay.State != Known` and Write != Unsupported → Blocked with
  EXACTLY `"tag display unknown — set On or Off before sending"`, and
  later gates do not run for the channel. Applies to Added and
  Modified. A test pins the gate ORDER and the combined case (Unknown
  TagDisplay + another unwritable/Inert field: the mandated reason,
  not a generic `"; "` merge — the gate stops first).
- `addedFields`: the Known-conditional at TagDisplay's CURRENT position
  (after Tag, before tone/skip); membership AND order pinned.
  `changedFields` untouched.
- `core/clone`'s `writableFieldsMismatch`: mutual-Known comparison; the
  exclusion-list doc extended.
- The end-to-end blocked-send test (one Unknown among Known siblings —
  blocked with the exact reason, siblings send, nothing on the wire for
  it).

Commit: `M9c-5 task 2: the diff's ordered TagDisplay gate; clone
verifies only mutual knowledge`.

### Task 3: E1c — the driver's remaining pieces

(The refusal landed in Task 1.) `requestedFields` gains the
Known-conditional at the preserved position; the refusal-PRIORITY test
(a multiply-invalid channel reports TagDisplay first — the top-of-
function placement pinned); **the MW+MT wire-identity test** (reference
Known-true and Known-false channels' frames byte-equal their pre-E1
literals); the read-produces-Known PIN (green since Task 1 — labelled a
pin, not a RED, per the adjudication).

Commit: `M9c-5 task 3: requestedFields honours state; refusal priority
and wire identity pinned`.

### Task 4: E1d — csvio's four states

As revision 1 (export via `exportBoolField`; `parseBoolFieldCell`
parameterised; import's four spellings; chirp flips to
`{State: Unknown}` — THE RED of this task, with the import-validate
consequence tested: a CHIRP import now carries Unknown and the diff
blocks it until set), plus the downstream pins Codex 3 named:
`cmd/rigprog` import tests and `app/importexport_test.go` assert the
Unknown provenance.

Commit: `M9c-5 task 4: csvio speaks TagDisplay's four states; CHIRP
imports honestly Unknown`.

### Task 5: E1e — frontend and the digest docs

As revision 1 (regen; columns.js's five behaviours; the Svelte toggle;
digest/radioinfo doc updates), closing the sanctioned mid-branch
staleness.

Commit: `M9c-5 task 5: the frontend renders TagDisplay's states; the
digest docs tell the truth`.

### Task 6: E2 — the baud (as revision 1, unchanged)

### Task 7: E3 — Engine binds to one dialect (the enumerated migration)

`NewEngine(port, d cat.Dialect, opts...)`; gate = `d.AllowedCommand`
internally; `Init` uses `d.BuildAISet(false)`; **a NEW sentinel
`ErrUnconfiguredDialect`** for a zero/unconfigured dialect at
construction (distinct from `ErrNoAllowlist`, which remains the
hand-built/nil-`e.allow` `Do`-path sentinel); error text reworded.
The ELEVEN call expressions, prescribed per case (Codex 6):
- `ft710.go:186` → `d.dialect`.
- Routine test constructors (`engine_test.go:31`,
  `engine_lifecycle_test.go:46,134,376,414,457`,
  `allowfunc_test.go:28`) → `cat.FT710`.
- `allowfunc_test.go:59` (nil-gate refusal) → a zero `cat.Dialect`,
  asserting `ErrUnconfiguredDialect` before the reader starts.
- `allowfunc_test.go:85` (permissive custom gate) and `:123` (the
  recording-refusing gate) → redesigned around a configured dialect
  plus commands it admits/rejects respectively, PRESERVING the no-wire
  assertion (`:123`'s recorder becomes a port-level write recorder —
  the refusal must show zero port writes); the in-package
  `e.allow = nil` override (precedent `allowfunc_test.go:90`) remains
  for the `Do`-path `ErrNoAllowlist` test, seeded from a
  `cat.FT710`-constructed engine.
The constructor test proving gate and init from ONE dialect; "AI0;"
bytes unchanged; the guard green; the engine doc + handoff P9
correction as revision 1.

Commit: `M9c-5 task 7: Engine takes its dialect whole; unconfigured
dialects refuse at construction`.

### Task 8: E4 — app model-awareness (as revision 1, unchanged)

### Task 9: E5 — the fake table interface (as revision 1, unchanged)

### Task 10: E6 — WriteResult goes step-neutral (the pinned encoding)

`driver.go`: the spec's `WriteStep`/`WriteResult`; four bools deleted.
`write.go`: **both FT-710 steps PREALLOCATED after successful command
construction, before the first `Do`** (`Steps: []WriteStep{{Command:
"MW"}, {Command: "MT"}}`), flags set at the existing points — the
outcome table pinned by test: success → MW true/true + MT true/true;
MW rejection → MW true/false + MT false/false; MT rejection → MW
true/true + MT true/false; transport-ambiguous Sent stays false as
today; a pre-build refusal returns `Steps: []driver.WriteStep{}` —
EXPLICIT EMPTY, never nil (nil marshals `null`). `core/clone`
**PROJECTS** steps into the journal map (Codex 7 — raw struct marshal
would emit Go field names): each step →
`{"command": s.Command, "sent": s.Sent, "confirmed": s.Confirmed}`,
plus `"write_result_format": 2`. Grep gate, ALL EIGHT legacy names:
`MWSent|MWConfirmed|MTSent|MTConfirmed|mw_sent|mw_confirmed|mt_sent|
mt_confirmed` → zero hits outside historical docs. Journal tests for
the three outcomes in both WriteResult and journal form.

Commit: `M9c-5 task 10: WriteResult reports neutral steps; the journal
projects step records`.

### Task 11: the manifest — EXECUTABLE this time

(Codex's second CRITICAL: revision 1's procedure could not run.)

- **Base worktree build at the fork commit** (the M9c-3 procedure) —
  both sides' BYTES preserved; recorded hashes authenticate the base
  run but the DIFFS are the evidence.
- The artefact list, corrected: the historical import leg contributes
  stdout/stderr/exit ONLY (it exits 3 writing no file — expected
  UNCHANGED on all three); a NEW deterministic successful-import
  recipe (a minimal valid CHIRP CSV fixture committed under the
  manifest's own fixtures dir, imported into a fake-read codeplug,
  output written) evidences the import direction's changed class on
  BOTH sides.
- Procedure per artefact: `cmp` the unchanged set (probe.*, both
  stderr, all exits, import leg's three); `read.stdout` diff empty
  after removing ONLY the digest line; both JSONs — normalise
  `read_at` FIRST (the recorded command; the declared noise), then
  diff empty after removing ONLY schema/`tag_display`/
  `baseline_digest`; `export.csv` diff empty after removing ONLY the
  `tag_display` column; the new import output the same class. Raw
  diffs recorded separately, timestamp noise included. The count
  stated honestly; the gate-at-final-code-tip invariant per the M9c-4
  note.
- The schema-2→3 load round-trip; full local gate; guards -v; the
  handoff updates per the spec's acceptance item 4.

Commit: `M9c-5 task 11: the executable enumerated-carve-out manifest
and full gate`.

## Self-review

Coverage as revision 1 with the adjudicated reshapes. Ordering: T1 is
the keystone (fully green, safety-complete); T2-T5 semantic; T6-T10
after T1 (T10 after T2 for clone); T11 last. The spec's Task-11
obligation wording is corrected in the same commit as this plan.
Placeholders: none.
