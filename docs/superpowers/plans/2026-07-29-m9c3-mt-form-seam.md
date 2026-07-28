# M9c-3 MT frame-form seam — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach `core/cat` a second MT frame form — the FTdx10 family's
combined record — behind a zero-invalid form discriminator, without moving
a byte of the FT-710's, and fix two live receiver-vs-global defects in the
FT-710 driver's write path.

**Architecture:** `MTPolicy` gains `Form MTForm` and per-form field
ownership (`TagFill` combined-only; `ClearTagByte`/`PadByte` short-only),
enforced by V9. The short-form API keeps its signatures and its exact
FT-710 bytes and error text; new `BuildMTSetCombined` /
`ParseMTAnswerCombined` / `MTAnswerBounds` serve the combined form,
reusing `memdata.go`'s one offset block through extracted field
encode/decode helpers. The gate branches per form; the Set/Answer
collision narrows to the P7-`'0'` case. Two disagreeing combined fixtures
join `allTestDialects()`; a new exported conformance package
(`core/cat/dialecttest`) lets M9c-4 test the real FTdx10 dialect without
the import cycle that makes `allTestDialects()` unreachable to it.

**Tech Stack:** Go 1.25, standard library only.

**Spec:** `docs/superpowers/specs/2026-07-28-m9c3-mt-form-seam-design.md`
(revision 2). Read it in full first. Adjudication:
`.superpowers/sdd/m9c3-spec-review-adjudication.md`.

## Global Constraints

- **British English** in all prose and comments. **SPDX header** on every
  new `.go` file.
- **`gofmt -l .` silent at the end of every task** (M9c-1's per-task
  reviews all missed drift; the gate is per-task now).
- **Never regenerate any golden.** `core/cat/testdata/` stays at exactly
  two commits (`ff5c19b`, `1d38941`). After every task:
  `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go`.
- **FT-710 byte identity is the milestone bar**: the four goldens, the
  parser-corpus error strings, and (task 10) probe/read/CHIRP outputs
  from compiled binaries against the pre-milestone worktree.
- **File lists are hypotheses; the repo-wide grep in each task is the
  gate** (five of five M9c-1 briefs enumerated incompletely).
- `go test -race ./core/...` exceeds ten minutes foreground; background
  it if run.

> ### ⚠️ THE PINNED-FILE EDITING DISCIPLINE — read before touching any `core/cat/*_test.go`
>
> `core/cat/evidence_literals_test.go` walks EVERY `*_test.go` in
> `core/cat` and `core/cat/testdata/evidence-literals.golden` pins each
> file's STRING/CHAR/INT literals by `(file, ordinal)`. The check is
> survival: a pinned literal that moves, changes or disappears fails, and
> the golden must never be regenerated. Therefore, in ANY existing
> `core/cat/*_test.go`:
>
> - **Adding, removing or reordering literals mid-file is FATAL** — every
>   later ordinal shifts.
> - **Identifier-only edits are safe** (e.g. adding `Form: MTFormShort`
>   to a composite literal — `MTFormShort` is a constant identifier, not
>   a literal; a field name is an identifier).
> - **Appending after a file's last literal is safe** (new ordinals
>   beyond the golden's records).
> - **New `*_test.go` files are safe** (no golden records; ordinals are
>   per-file).
>
> This plan is engineered so that every literal-bearing addition lands in
> NEW files (`mtform_test.go`, `mtcombined_test.go`,
> `dialecttest/…`) or as end-of-file appends, and every edit inside a
> pinned file is identifier-only. If a task seems to need a mid-file
> literal, stop and report — do not improvise.
>
> Also note from scoping (verified): the existing
> `TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible` needs **no
> edit** — its `build()` silently skips builders that return errors, so
> combined fixtures that refuse `BuildMTSet` contribute via their other
> builders and the per-builder floors stay satisfied by the short-form
> dialects.

## File Structure

| File | Responsibility |
|---|---|
| `core/cat/dialectconfig.go` | `MTForm`, `MTPolicy.{Form,TagFill}`, per-form doc rewrite. |
| `core/cat/dialectvalidate.go` | V9 ownership rules + frame-ceiling proof. |
| `core/cat/dialect.go` | FT-710 literal gains `Form: MTFormShort`. |
| `core/cat/mt.go` | Derived short window; form refusal on short APIs; `MTForm()` accessor; `MTAnswerBounds`. |
| `core/cat/mtcombined.go` | **New.** `BuildMTSetCombined`, `ParseMTAnswerCombined`, `decodeCombinedTag`, the Set-kind schema constant. |
| `core/cat/memdata.go` / `mr.go` / `mw.go` | Field-block extraction (`parseMemoryFields` / `encodeMemoryFields`); MR/MW byte-identical. |
| `core/cat/allowlist.go` | `validMTCommand` form branch; narrowed collision. |
| `core/cat/mtform_test.go` | **New.** V9 ownership tests, form-refusal matrix, `MTAnswerBounds`, derived-window tests. |
| `core/cat/mtcombined_test.go` | **New.** Combined build/parse/round-trip/gate tests; the form-aware walk with its own counters. |
| `core/cat/seconddialect_test.go` | Two combined fixtures appended; `allTestDialects()` body gains two identifiers. |
| `core/cat/dialecttest/dialecttest.go` | **New package.** Exported-API conformance suite (`Run(t, d)`). |
| `core/cat/dialecttest/dialecttest_test.go` | **New.** Runs the suite over `cat.FT710` and an externally built fixture. |
| `internal/guards/importgraph_test.go` | Selector fence += `BuildMTSetCombined`. |
| `internal/guards/dialectglobals_test.go` | `gateReachingValidators` += the new combined validators. |
| `core/driver/ft710/write.go` | The two receiver fixes. |
| `core/driver/ft710/modebyname_test.go` | One `MTPolicy` literal gains `Form:` (identifier-only; not a pinned package). |
| `docs/superpowers/m9c3-baseline-manifest.md` | **New.** Byte-identity manifest. |

Existing `core/cat` pinned files receiving **identifier-only** edits:
`seconddialect_test.go` (fixture `MTPolicy` literals gain
`Form: MTFormShort`; new fixtures appended at end),
`dialectequiv_test.go:62,317`, `dialectvalidate_test.go:103`,
`milestonefixes_test.go:133,245`, `clarifier_test.go:48`,
`mwkind_test.go:30`, `dialectexternal_test.go:46`, `mtpolicy_test.go:16`
(all: add `Form: MTFormShort`). The blast-radius list is a hypothesis —
Task 1's gate is `grep -rn "MTPolicy{" --include=*.go .`.

---

### Task 1: `MTForm`, `TagFill`, V9 ownership, and the blast radius

**Files:**
- Modify: `core/cat/dialectconfig.go` (MTPolicy), `core/cat/dialectvalidate.go` (V9), `core/cat/dialect.go:148`
- Modify (identifier-only): every `MTPolicy{...}` literal found by the gate grep
- Create: `core/cat/mtform_test.go` (V9 tests)
- Test also: `core/driver/ft710/modebyname_test.go:54`

**Interfaces:**
- Produces: `type MTForm int` with `MTFormUnspecified`/`MTFormShort`/`MTFormCombined` (String() in the ObservationPolicy style); `MTPolicy{Form, TagMaxBytes, ClearTagByte, PadByte, TagFill}` (comparable — scalars only); V9 rules below.

- [ ] **Step 1: write the failing V9 tests** in NEW file `core/cat/mtform_test.go` (`package cat`). Table-driven over a valid short base (the FT-710's policy plus `Form: MTFormShort`) and a valid combined base (`MTPolicy{Form: MTFormCombined, TagMaxBytes: 6, TagFill: ' '}`), asserting `NewDialect` refusal for each: omitted Form (zero); out-of-enum Form (`MTForm(9)`); short with `TagFill` set; combined with `ClearTagByte` set; combined with `PadByte` set; combined with `TagFill` zero; combined with invalid `TagFill` (`';'`, `0x1F`); combined whose `29+TagMaxBytes > DefaultMaxFrame` (TagMaxBytes 64 is fine — the existing `maxMTTagBytes` cap already keeps 29+64=93 under 256, so this rule needs a direct unit call on the validator with a synthetic ceiling check — assert the ERROR TEXT names both numbers); acceptance of both bases. Use full `DialectConfig` fixtures modelled on `dialectvalidate_test.go`'s existing pattern (copy a valid config, mutate `MT`).
- [ ] **Step 2: run** `go test ./core/cat/ -run 'TestValidateMTPolicy|TestMTForm' -v` — expect compile failure (`undefined: MTFormShort`).
- [ ] **Step 3: implement.** In `dialectconfig.go`: the enum + String(); add `Form MTForm` (first field) and `TagFill byte` (last) to `MTPolicy`; REWRITE the type's doc per-form — it currently says "This type describes the SHORT form only", which becomes false this commit; give `TagMaxBytes` its dual-meaning doc for the combined form INCLUDING the recorded bet from the spec ("policy bound AND tag-field width; coinciding on the evidenced radio; a TagFieldWidth split is additive if a counter-radio appears"); `ClearTagByte`/`PadByte` docs gain "short-form only; must be zero under MTFormCombined"; `TagFill` doc: "combined-form only: the outbound fill byte AND the answer trim byte; zero-invalid there — an omitted fill must not silently emit NUL". In `dialectvalidate.go` V9 (`validateMTPolicy`): keep the existing TagMaxBytes and per-byte checks, add the ownership switch:

```go
	switch cfg.MT.Form {
	case MTFormShort:
		if cfg.MT.TagFill != 0 {
			return fmt.Errorf("cat: MT.TagFill %#02x is set under MTFormShort — TagFill is combined-form data and an inapplicable field must be explicitly zero", cfg.MT.TagFill)
		}
	case MTFormCombined:
		if cfg.MT.ClearTagByte != 0 {
			return fmt.Errorf("cat: MT.ClearTagByte %#02x is set under MTFormCombined — no distinct clear encoding is documented for the combined form; an empty tag is the all-TagFill field", cfg.MT.ClearTagByte)
		}
		if cfg.MT.PadByte != 0 {
			return fmt.Errorf("cat: MT.PadByte %#02x is set under MTFormCombined — answer trimming is TagFill's job in this form", cfg.MT.PadByte)
		}
		if !validWireByte(cfg.MT.TagFill) {
			return fmt.Errorf("cat: MT.TagFill is %#02x under MTFormCombined, want printable ASCII 0x20-0x7E excluding ';' — it fills every outbound tag field, and zero would silently emit NUL", cfg.MT.TagFill)
		}
		if maxFrame := 29 + cfg.MT.TagMaxBytes; maxFrame > DefaultMaxFrame {
			return fmt.Errorf("cat: combined MT frame length %d (29+TagMaxBytes) exceeds the %d-byte transport ceiling", maxFrame, DefaultMaxFrame)
		}
	default:
		return fmt.Errorf("cat: MT.Form %v must be set explicitly — the zero value is not a form (an omitted form must refuse, not default)", cfg.MT.Form)
	}
```

Note the existing `ClearTagByte` unconditional check must become
short-form-only (move it into the `MTFormShort` case, unchanged text).
`dialect.go:148` gains `Form: MTFormShort` (identifier-only edit in a
non-test file — unconstrained).
- [ ] **Step 4: sweep the blast radius.** Gate: `grep -rn "MTPolicy{" --include=*.go .` — every composite literal gains `Form: MTFormShort` (identifier-only; NO literal added). Expected ≈14 sites; the grep decides. `TestNewDialect_ReproducesFT710` (`dialectequiv_test.go`) must pass unchanged — its `want.mt != got.mt` now compares five fields.
- [ ] **Step 5: full verify** — `go test ./core/cat/ ./internal/guards/ ./core/driver/ft710/`, `gofmt -l .`, `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go`. The evidence-literal test passing proves the identifier-only discipline held.
- [ ] **Step 6: commit** `M9c-3 task 1: MTForm and TagFill with per-form ownership enforced by V9`.

---

### Task 2: the derived short window and form refusal on the short API

**Files:** Modify `core/cat/mt.go`, `core/cat/allowlist.go:201`; append tests to `core/cat/mtform_test.go`.

**Interfaces:**
- Produces: `func (d Dialect) MTForm() MTForm` (exported accessor — dialecttest and M9c-4/5 consume it); unexported `func (d Dialect) mtShortAnswerMax() int` (= `mtAnswerMinLen - 1 + d.mt.TagMaxBytes`, i.e. `6 + ... `— **compute as `mtAnswerMinLen + d.mt.TagMaxBytes` MINUS the 0-tag: precisely `2+3+1+1 + d.mt.TagMaxBytes = 7 + TagMaxBytes`**; for the FT-710, 19); short APIs refuse on `MTFormCombined`.

- [ ] **Step 1: failing tests** (append to `mtform_test.go`): (a) FT-710's `ParseMTAnswer` error text for a 20-byte frame is EXACTLY `"MT answer must be 7-19 bytes"` (string-compare the `*ParseError` message — this pins the corpus bytes); (b) the 6-byte-tag short peer (reuse `mtpolicy_test.go`'s peer config pattern, built fresh here) REFUSES a 14-byte answer (`7+6=13` max) that the FT-710 accepts — the derived-window disagreement test; (c) `BuildMTSet`/`ParseMTAnswer` on a combined fixture (a minimal `MustNewDialect` combined config built in this file) return errors naming the form; (d) `MTForm()` returns the right value for FT710/combined/zero dialect.
- [ ] **Step 2: run, expect failures** ((b) passes today's global window — it must FAIL before the change; verify it does, then the others fail to compile).
- [ ] **Step 3: implement.** `mt.go`: delete `mtAnswerMaxLen` (the guard forbids new geometry constants, and this one becomes derived — `mtAnswerMinLen` stays, it is the short form's structural floor); `ParseMTAnswer` and `BuildMTSet` open with

```go
	if d.mt.Form != MTFormShort {
		return ... newParseError(frame, fmt.Sprintf("MT short-form API called on a %v dialect — use the combined-form API", d.mt.Form))
	}
```

(exact wording implementer's, but it must name the form); the window
check becomes `if len(frame) < mtAnswerMinLen || len(frame) > d.mtShortAnswerMax()`
with the SAME `fmt.Sprintf("MT answer must be %d-%d bytes", ...)` —
rendering `7-19` for the FT-710. `allowlist.go:201` uses the same method.
`BuildMTRead` stays form-independent. Add `MTForm()`.
Update `mt.go:14-26`'s constant commentary (it documented this exact
future).
- [ ] **Step 4: peer churn sweep.** Run `go test ./core/cat/`; fix any existing peer-dialect test that asserted acceptance inside the old global window (expected: `mtpolicy_test.go` region — identifier/expression edits only if the file is pinned; if an expectation literal must change, STOP and report per the boxed discipline). Then the corpus: `go test ./core/cat/ -run 'Corpus' -v` — byte-identical, no golden touched.
- [ ] **Step 5: verify + commit** (gates as Task 1). `M9c-3 task 2: the short MT window derives from the receiver; short APIs refuse a combined dialect`.

---

### Task 3: field-block extraction (MR/MW byte-identical)

**Files:** Modify `core/cat/mr.go`, `core/cat/mw.go`, `core/cat/memdata.go`.

**Interfaces:**
- Produces: `func (d Dialect) parseMemoryFields(frame []byte, wantPrefix string) (MemoryData, error)` — validates and decodes offsets 2-26 ONLY (no length, no terminator, no kind-vocabulary narrowing beyond today's `validKindByte`); `func encodeMemoryFields(frame []byte, m MemoryData)` — writes offsets 2-26 into a caller-sized buffer. `parseMemoryFrame` becomes: length==28 check + terminator-at-27 check + `parseMemoryFields`; `BuildMWSet`'s body writes via `encodeMemoryFields`.

- [ ] **Step 1**: this is a refactor with NO behaviour change — the proof is the existing suite plus the corpus goldens, not new tests. Extract exactly; keep every error message byte-identical (they all carry `wantPrefix`, so the extraction must thread it).
- [ ] **Step 2: verify hard** — full `go test ./core/cat/` (corpus + goldens + evidence literals), `go test ./core/driver/ft710/ ./internal/fakeradio/ ./core/transport/`, gates as Task 1.
- [ ] **Step 3: commit** `M9c-3 task 3: extract the shared memory field block encoder/decoder; MR/MW byte-identical`.

---

### Task 4: the combined form — build, parse, bounds

**Files:** Create `core/cat/mtcombined.go`, `core/cat/mtcombined_test.go`; modify `core/cat/mt.go` (MTAnswerBounds).

**Interfaces (consumed by Tasks 5-7 and M9c-4/5):**

```go
// combinedMTSetKind is the combined Set's P7 schema value, "0: (Fixed)".
// A FORM constant, deliberately NOT the dialect's mwWriteKind: MT-Set P7
// and MW-Set P7 are two command-specific facts that coincide on the
// evidenced radio, and deriving one from the other is the PadByte
// conflation (spec revision 2, adjudication). Set-direction '0' means
// "(Fixed)", not "VFO".
const combinedMTSetKind byte = '0'

func (d Dialect) BuildMTSetCombined(m MemoryData, tag string) (Command, error)
func (d Dialect) ParseMTAnswerCombined(frame []byte) (MemoryData, string, error)
func (d Dialect) MTAnswerBounds() (min, max int, err error) // in mt.go
```

Rules (all from the spec — implement exactly):
- Both refuse on `Form != MTFormCombined`, symmetric with Task 2.
- Build: `mtSlotValid` slot; `m.Kind == combinedMTSetKind` required
  (validate-don't-rewrite; error text explains Set-'0' is "(Fixed)");
  the same field validations `BuildMWSet` applies (via `validateMWFields`?
  NO — that checks `m.Kind == d.mwWriteKind`, which is exactly the
  coupling the spec forbids here; instead apply the memory-field checks
  directly: writableSlot is NOT the rule — `mtSlotValid` is (memory/PMS
  only), then mode via `d.ParseMode` round-trip as `BuildMWSet` does,
  clarifier via `d.validClarHz`, `FreqHz <= memFreqMax`, CTCSS/shift by
  their existing Valid predicates — mirror `validateMWFields` MINUS the
  kind rule, and say so in a comment); tag: charset per `validMTTagByte`,
  `len(tag) <= d.mt.TagMaxBytes`, and **refuse trailing `TagFill`**
  (`strings.HasSuffix(tag, string(d.mt.TagFill)) && tag != ""` → error
  "trailing fill byte would not round-trip; trim it") — validate, don't
  canonicalise. Emission: buffer of `29 + d.mt.TagMaxBytes`; `'M','T'`;
  `encodeMemoryFields`; `combinedMTSetKind` OVERWRITES the kind offset?
  NO — `m.Kind` already equals it (validated); P11 `'0'` at offset 27;
  tag + `TagFill` padding to exactly TagMaxBytes at 28..; `';'` last.
  Empty tag → all-fill field (no distinct clear form).
- Parse: exact length `29 + d.mt.TagMaxBytes`; `"MT"` prefix; terminator
  last; `parseMemoryFields`; then NARROW kind to `{'0','1'}` (the
  documented read vocabulary — MR's 0-5 `validKindByte` would turn
  undocumented frames into data): refuse 2-5 with a combined-specific
  message; P11 must be `'0'`; tag = `decodeCombinedTag(raw)`: all-fill →
  `""`, else `strings.TrimRight(raw, string(d.mt.TagFill))`.
- `MTAnswerBounds`: `MTFormShort` → `(mtAnswerMinLen, d.mtShortAnswerMax(), nil)`;
  `MTFormCombined` → equal bounds `29+TagMaxBytes`; zero/unspecified →
  `(0, 0, error)` — an error, never plausible zeros.

- [ ] **Step 1: failing tests** in NEW `mtcombined_test.go`, using a
  file-local `mustCombinedDialect(tagMax int, fill byte, mwKind byte)`
  helper built on `MustNewDialect` with the FT-710's slot/mode tables and
  a minimal EX config. Cases (concrete, all asserted):
  - Round trip: tag `"CQ DX"`, fill `' '`, TagMax 6 → frame length 35,
    tag field `"CQ DX "`, parses back to exactly `"CQ DX"` + the same
    `MemoryData`.
  - Empty tag → all-fill field → parses to `""`.
  - Trailing-fill tag `"AB "` (fill `' '`) → build REFUSES.
  - `m.Kind = '1'` → build refuses (schema constant).
  - Parse: P7 `'1'` ACCEPTED (answer, Memory); P7 `'2'`..`'5'` REFUSED;
    P11 `'1'` refused; length 34 and 36 refused (exact-length,
    ASSUMED-until-Stage-R per spec — say so in the test comment with the
    named contingency).
  - **The decoupling proof:** a fixture with `mwKind: '1'` still builds
    combined Sets with P7 `'0'` — assert the frame byte at the kind
    offset directly.
  - `MTAnswerBounds` for FT710 (7,19), combined (35,35), zero dialect
    (error).
- [ ] **Step 2** run → compile failure. **Step 3** implement per the
  rules above. **Step 4** run all; gates as Task 1. **Step 5: commit**
  `M9c-3 task 4: BuildMTSetCombined, ParseMTAnswerCombined and MTAnswerBounds`.

---

### Task 5: the gate's combined branch and the narrowed collision

**Files:** Modify `core/cat/allowlist.go` (`validMTCommand`); append tests to `mtcombined_test.go`.

- [ ] **Step 1: failing tests**: on a combined fixture — its own
  `BuildMTSetCombined` output is admitted; a combined answer-shape with
  P7 `'1'` is REFUSED (the narrowed collision — with a comment citing
  `allowlist_test.go:138`'s short-form precedent and stating the residual
  is only the P7-`'0'` case); a short-form MT Set frame is refused by the
  combined dialect's gate and vice versa; the 6-byte MT read is admitted
  by BOTH forms; a wrong-length combined frame refused; zero-form dialect
  refuses everything (already forced by `Configured()`, but assert the MT
  path specifically).
- [ ] **Step 2/3**: implement — `validMTCommand` keeps the read branch
  first (form-independent), then `switch d.mt.Form`: short = today's
  logic verbatim; combined = exact `29+TagMaxBytes` length, prefix,
  terminator, `parseMemoryFields`-based field validation + slot via
  `mtSlotValid` (write policy: memory/PMS only) + kind `== combinedMTSetKind`
  + P11 + tag charset with trailing-fill consistency; document the
  narrowed exception in the function comment and in `allowlist.go`'s
  exception-table commentary.
- [ ] **Step 4**: run `go test ./core/cat/` including the allowlist
  corpus (byte-identical); gates. **Step 5: commit** `M9c-3 task 5: the
  gate's combined MT branch; the Set/Answer collision narrows to P7-'0'`.

---

### Task 6: disagreeing fixtures in `allTestDialects()` and the form-aware walk

**Files:** Modify `core/cat/seconddialect_test.go` (APPEND two fixtures at end; add two identifiers to `allTestDialects()`'s body); append the form-aware walk to `mtcombined_test.go`.

- [ ] **Step 1**: append `combinedDialect()` (TagMax 6, TagFill `' '`,
  mwWriteKind `'1'` — the decoupling carrier; FT-710 slot/mode data) and
  `combinedPeerDialect()` (TagMax 12, TagFill `'_'`, mwWriteKind `'2'`,
  and the peer posture's disagreeing slot space) at the END of
  `seconddialect_test.go`, built via the file's existing
  `mustFixtureDialect` pattern. Add both identifiers to
  `allTestDialects()` (identifier-only edit). **Predicted effect on the
  existing walk: none** — `build()` skips erroring builders; verify
  `TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible` passes
  UNCHANGED, and state in the fixtures' doc comments that the
  form-aware counters live in `mtcombined_test.go`.
- [ ] **Step 2**: in `mtcombined_test.go`, add
  `TestEveryDialect_MTFormCoverage`: walks `allTestDialects()`; for every
  dialect, the OWN-form builders must contribute frames (counters
  `"MT set combined (tagged)"`, `"MT set combined (cleared)"`,
  `"MT set short (tagged)"`, `"MT set short (cleared)"` — per-builder
  floors ≥1 across the walk) and the WRONG-form builders must be SEEN to
  refuse (counted refusals ≥1 per combined dialect for `BuildMTSet`, and
  ≥1 per short dialect for `BuildMTSetCombined` — a silent skip is the
  vacuity this test exists to prevent). Round-trip every built combined
  frame through `ParseMTAnswerCombined` and the gate.
- [ ] **Step 3**: run everything; the zero-dialect corpus floor test and
  round-trip walks now cover the new fixtures automatically. Gates.
- [ ] **Step 4: commit** `M9c-3 task 6: combined fixtures join allTestDialects; the form-aware walk counts refusals`.

---

### Task 7: the `dialecttest` conformance package

**Files:** Create `core/cat/dialecttest/dialecttest.go`, `core/cat/dialecttest/dialecttest_test.go`.

**Interfaces:**
- Produces: `package dialecttest` with `func Run(t *testing.T, d cat.Dialect)` — the exported-API conformance subset: every built frame (ID/AI/MC/MR/MT-read/MT-set-by-form/MW/EX over the dialect's own tables) is clean, single-terminator, and admitted by the dialect's own gate; build→parse round-trips for MT (by form), MR is not round-trippable without a radio — skip; `MTAnswerBounds` coherent with the form; a zero `cat.Dialect` refuses everything it is offered. This is what M9c-4 runs over the real FTdx10 dialect (the in-package `allTestDialects()` is unreachable to it — import cycle, Codex M9c-3 spec finding 1).

- [ ] **Step 1: failing test**: `dialecttest_test.go` runs `Run(t, cat.FT710)` and `Run` over a combined dialect built EXTERNALLY via `cat.MustNewDialect` (the `dialectexternal_test.go` pattern — this doubles as proof the combined form is constructible from outside). 
- [ ] **Step 2/3**: implement `Run` using ONLY exported API (`MTForm()`, `MTAnswerBounds()`, builders, parsers, `AllowedCommand`, `EXAddresses`, `ParseSlot`, mode iteration via `AllModes()`-equivalent — check what is exported; where iteration needs unexported data, iterate wire space exhaustively as the gate walk does with `threeDigits`).
- [ ] **Step 4**: verify + guards (the new package imports `core/cat` — confirm `go list -deps` shows no cycle and the importgraph guard passes; if the guard's composition rules flag the new package, STOP and report rather than editing guard rules beyond Task 8's sanctioned additions). Gates. **Step 5: commit** `M9c-3 task 7: dialecttest — the exported conformance suite M9c-4 runs over the real dialect`.

---

### Task 8: guards

**Files:** Modify `internal/guards/importgraph_test.go` (selector fence), `internal/guards/dialectglobals_test.go` (`gateReachingValidators`).

- [ ] **Step 1**: add `BuildMTSetCombined` to the write-composition
  selector fence, its doc, and its non-vacuity accounting; add the new
  combined gate-reaching validator names (as implemented in Tasks 4-5)
  to `gateReachingValidators`. These files are NOT in `core/cat` — the
  evidence-literal discipline does not bind them; their own red-proof
  conventions do.
- [ ] **Step 2: red-proof both**: temporarily add a decoy
  `BuildMTSetCombined` call in an unauthorised package → fence fires;
  temporarily demote one new validator to a package function → guard
  fires. Revert both, show the evidence in the report.
- [ ] **Step 3**: full guard run green; gates; commit `M9c-3 task 8:
  guards learn BuildMTSetCombined and the combined gate validators`.

---

### Task 9: the two driver receiver fixes

**Files:** Modify `core/cat/dialect.go` or `dialectconfig.go` (accessors), `core/driver/ft710/write.go:237,256`; tests in `core/driver/ft710`.

- [ ] **Step 1: failing tests** (in ft710's existing test files —
  NOT pinned by core/cat's golden — or a new file): a `buildWriteCommands`
  unit test asserting the built MW frame's kind byte equals
  `dialect.MWWriteKind()` and that a clarifier just past
  `dialect.Clarifier().MaxAbsHz` is refused BEFORE any wire traffic;
  plus a byte-identity pin: the exact MW+MT frames for a reference
  channel are unchanged against their current literal expectations.
- [ ] **Step 2/3**: add exported `func (d Dialect) MWWriteKind() byte`
  and `func (d Dialect) Clarifier() ClarifierPolicy` (returns the
  policy by value — comparable, no mutation surface); replace
  `write.go:256`'s `Kind: cat.KindMemory` with
  `Kind: dialect.MWWriteKind()` and `:237`'s `9990` with
  `dialect.Clarifier().MaxAbsHz` (keep the error text byte-identical —
  it interpolates the bound; FT-710 renders the same bytes).
- [ ] **Step 4**: `go test ./core/driver/ft710/ ./core/clone/` green;
  gates; commit `M9c-3 task 9: the write path consults its receiver —
  MWWriteKind and Clarifier accessors replace the literals`.

---

### Task 10: baseline manifest and the full gate

- [ ] **Step 1**: build binaries at the branch base commit in a
  worktree (`git worktree add /tmp-eq/m9c3-base <base>` — use the
  scratchpad, record the SHA) and at HEAD; run `probe --fake`,
  `read --fake` (fixed fake image) and the CHIRP export path; byte-compare
  stdout, stderr AND exit codes. Write
  `docs/superpowers/m9c3-baseline-manifest.md` in the M9c-1 manifest
  style (hashes, commands, environment, the two SHAs).
- [ ] **Step 2**: the full local gate — gofmt silent; build; vet; full
  `go test ./...` foreground; guards verbose; `git diff --exit-code --
  core/cat/testdata/ core/cat/exinventory_gen.go`; the repo-wide greps
  (`MTPolicy{` all carrying Form; no `mtAnswerMaxLen` survivor; no new
  package-level MT geometry constant).
- [ ] **Step 3**: commit the manifest; remove the worktree.

## Self-review

**Spec coverage:** MTForm/ownership → T1; derived window + form refusal +
`MTForm()` → T2; field-block reuse → T3; combined API + bounds + P7
constant + tag semantics → T4; gate + narrowed collision → T5; fixtures +
decoupling proof + form-aware counters → T6; conformance suite (the
allTestDialects replacement) → T7; both guard obligations → T8; both
driver fixes → T9; byte-identity manifest → T10. The spec's M9c-5
obligations and ASSUMED markers are documentation, carried in the spec
itself. **Placeholders:** none; two implementer-choice points (short-API
refusal wording, ft710 test file placement) are bounded and stated.
**Type consistency:** `MTAnswerBounds() (min, max int, err error)`
declared in T4 and consumed in T7; `MTForm()` declared T2, consumed T7;
`combinedMTSetKind` declared T4, consumed T5. **Ordering:** T2 needs T1's
Form field; T4 needs T3's extraction; T5 needs T4's builder for
admissibility tests; T6 needs T4+T5; T7 needs T2's accessors; T8 needs
T4-5's names; T9 independent after T1; T10 last.
