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

> ### ⚠️ THE EVIDENCE-LITERAL MODEL — read before touching any `core/cat/*_test.go`
>
> (Corrected by both plan reviewers — revision 1 materially
> over-claimed the pin's scope.)
>
> `core/cat/evidence_literals_test.go` walks `core/cat`'s `*_test.go`
> files EXCEPT an explicit exclusion list (`evidence_literals_test.go:
> 61-65`): `evidence_literals_test.go`, `framecorpus_test.go`,
> `allowlistcorpus_test.go`, `dialect_test.go` and
> **`seconddialect_test.go`** are NOT walked. The golden pins
> STRING/CHAR/INT literals by `(file, ordinal)` for the ~21 files that
> have records, and the check is survival-only: it verifies golden
> records still hold; it never checks literals the golden does not
> record. The golden must never be regenerated.
>
> Consequences for this plan:
>
> - **Files this plan edits that ARE golden-pinned**: `mt.go`'s tests
>   live in pinned files (`mt_test.go`, `mtpolicy_test.go`,
>   `dialectequiv_test.go`, `dialectvalidate_test.go`,
>   `milestonefixes_test.go`, `clarifier_test.go`, `mwkind_test.go`,
>   `dialectexternal_test.go` — but note some of THESE may hold no
>   records either; the golden, not this list, is the authority). In a
>   file with golden records: adding/removing/reordering literals
>   BEFORE a pinned one is fatal; identifier-only edits (e.g. adding
>   `Form: MTFormShort` — a constant identifier, not a literal) are
>   always safe; appends after the last pinned literal are safe.
> - **`seconddialect_test.go` is EXCLUDED from the walker** — Task 6's
>   fixtures and the `allTestDialects()` entries (which include string
>   literals) are unconstrained by the golden.
> - **New `*_test.go` files are always safe** (no golden records).
> - **A passing evidence test does NOT prove edits were literal-safe**
>   (it only proves pinned records survived). Each task's reviewer must
>   inspect the diff for literal changes in golden-recorded files
>   directly.
>
> If an edit genuinely requires disturbing a pinned literal, stop and
> report — do not improvise.
>
> Also note from scoping (verified by both reviewers across all five
> `allTestDialects()` consumers): the existing
> `TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible` needs **no
> edit** — its `build()` silently skips builders that return errors, so
> combined fixtures that refuse `BuildMTSet` contribute via their other
> builders, the MW loop's `Kind: nd.dia.mwWriteKind` works for kinds
> '1'/'2', and the per-builder floors stay satisfied by the short-form
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

Existing `core/cat` test files receiving **identifier-only** edits (safe
regardless of golden status): `seconddialect_test.go` (excluded from the
walker entirely; three `MTPolicy` literals gain `Form: MTFormShort`; new
fixtures appended at end), `dialectequiv_test.go:62,317`,
`dialectvalidate_test.go:103`, `milestonefixes_test.go:133,245`,
`clarifier_test.go:48`, `mwkind_test.go:30`, `mtpolicy_test.go:16` —
all add `Form: MTFormShort`. Two sites need the QUALIFIED form
`Form: cat.MTFormShort`: `dialectexternal_test.go:46` (`package
cat_test`) and `core/driver/ft710/modebyname_test.go:54`. Both reviewers
verified the repo-wide count is exactly **14** `MTPolicy{` composite
literals with no aliased or non-literal construction anywhere — but the
blast-radius list remains a hypothesis; Task 1's gate is
`grep -rn "MTPolicy{" --include=*.go .`.

---

### Task 1: `MTForm`, `TagFill`, V9 ownership, and the blast radius

**Files:**
- Modify: `core/cat/dialectconfig.go` (MTPolicy), `core/cat/dialectvalidate.go` (V9), `core/cat/dialect.go:148`
- Modify (identifier-only): every `MTPolicy{...}` literal found by the gate grep
- Create: `core/cat/mtform_test.go` (V9 tests)
- Test also: `core/driver/ft710/modebyname_test.go:54`

**Interfaces:**
- Produces: `type MTForm int` with `MTFormUnspecified`/`MTFormShort`/`MTFormCombined` (String() in the ObservationPolicy style); `MTPolicy{Form, TagMaxBytes, ClearTagByte, PadByte, TagFill}` (comparable — scalars only); V9 rules below.

- [ ] **Step 1: write the failing V9 tests** in NEW file `core/cat/mtform_test.go` (`package cat`). Table-driven over a valid short base (the FT-710's policy plus `Form: MTFormShort`) and a valid combined base (`MTPolicy{Form: MTFormCombined, TagMaxBytes: 6, TagFill: ' '}`), asserting `NewDialect` refusal for each: omitted Form (zero); out-of-enum Form (`MTForm(9)`); short with `TagFill` set; combined with `ClearTagByte` set; combined with `PadByte` set; combined with `TagFill` zero; combined with invalid `TagFill` (`';'`, `0x1F`); acceptance of both bases. Use full `DialectConfig` fixtures modelled on `dialectvalidate_test.go`'s existing pattern (copy a valid config, mutate `MT`).

  **The frame-ceiling proof is a constants-relationship invariant, NOT a
  runtime branch** (both reviewers: `maxMTTagBytes` = 64 makes any
  runtime ceiling check dead code — short max 7+64=71 and combined max
  29+64=93 are both static facts under `DefaultMaxFrame` 256, and no
  config can reach a ceiling error past the TagMaxBytes cap). Write one
  test asserting BOTH form invariants directly:

```go
// TestMTFrameCeilings_FitTransportFrame is the spec's "V9 proves each
// form's derived maximum fits DefaultMaxFrame", in the only honest
// shape: maxMTTagBytes caps TagMaxBytes at 64, so the maxima are
// compile-time facts, proven here rather than by an unreachable
// runtime branch.
func TestMTFrameCeilings_FitTransportFrame(t *testing.T) {
	if short := mtAnswerMinLen + maxMTTagBytes; short > DefaultMaxFrame {
		t.Errorf("short-form ceiling %d exceeds DefaultMaxFrame %d", short, DefaultMaxFrame)
	}
	if combined := 29 + maxMTTagBytes; combined > DefaultMaxFrame {
		t.Errorf("combined-form ceiling %d exceeds DefaultMaxFrame %d", combined, DefaultMaxFrame)
	}
}
```

  V9 itself carries a comment recording the same reasoning; it gains NO
  ceiling branch.
- [ ] **Step 2: run** `go test ./core/cat/ -run 'TestValidateMTPolicy|TestMTForm' -v` — expect compile failure (`undefined: MTFormShort`).
- [ ] **Step 3: implement.** In `dialectconfig.go`: the enum + String(); add `Form MTForm` (first field) and `TagFill byte` (last) to `MTPolicy`; REWRITE the type's doc per-form — it currently says "This type describes the SHORT form only", which becomes false this commit; give `TagMaxBytes` its dual-meaning doc for the combined form INCLUDING the recorded bet from the spec ("policy bound AND tag-field width; coinciding on the evidenced radio; a TagFieldWidth split is additive if a counter-radio appears"); `ClearTagByte`/`PadByte` docs gain "short-form only; must be zero under MTFormCombined"; `TagFill` doc: "combined-form only: the outbound fill byte AND the answer trim byte; zero-invalid there — an omitted fill must not silently emit NUL". In `dialectvalidate.go` V9 (`validateMTPolicy`): keep the existing TagMaxBytes and per-byte checks, add the ownership switch:

```go
	switch cfg.MT.Form {
	case MTFormShort:
		// The pre-existing short-form requirements, verbatim: ClearTagByte
		// must be a valid wire byte (it is emitted into every cleared
		// tag), and PadByte keeps its 0-or-valid rule. Only TagFill is
		// new here, as an ownership refusal.
		if !validWireByte(cfg.MT.ClearTagByte) {
			return fmt.Errorf("cat: MT.ClearTagByte is %#02x, which is outside printable ASCII 0x20-0x7E excluding ';' — it is emitted into every cleared tag", cfg.MT.ClearTagByte)
		}
		if cfg.MT.PadByte != 0 && !validWireByte(cfg.MT.PadByte) {
			return fmt.Errorf("cat: MT.PadByte is %#02x, want 0 (no padding) or a byte inside printable ASCII 0x20-0x7E excluding ';'", cfg.MT.PadByte)
		}
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
	default:
		return fmt.Errorf("cat: MT.Form %v must be set explicitly — the zero value is not a form (an omitted form must refuse, not default)", cfg.MT.Form)
	}
```

The pre-existing unconditional `ClearTagByte` and `PadByte` checks are
MOVED into the `MTFormShort` case with their text unchanged (shown
above) — the executable block is authoritative; there is no separate
"move it" step to interpret. The `TagMaxBytes` cap stays where it is,
before the switch. V9 gains a comment recording why there is no
runtime frame-ceiling branch (see Step 1). `dialect.go:148` gains
`Form: MTFormShort` (a non-test file — unconstrained).
- [ ] **Step 4: sweep the blast radius.** Gate: `grep -rn "MTPolicy{" --include=*.go .` — every composite literal gains `Form: MTFormShort` (identifier-only; NO literal added), with the QUALIFIED `cat.MTFormShort` at the two external sites (`dialectexternal_test.go:46`, `core/driver/ft710/modebyname_test.go:54`). Both reviewers verified the count is exactly 14 today; the grep decides. `TestNewDialect_ReproducesFT710` (`dialectequiv_test.go`) must pass unchanged — its `want.mt != got.mt` now compares five fields.
- [ ] **Step 5: full verify** — `go test ./core/cat/ ./internal/guards/ ./core/driver/ft710/`, `gofmt -l .`, `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go`. NOTE: a passing evidence-literal test does NOT prove the edits were literal-safe (it checks only golden-recorded literals) — additionally inspect `git diff` over the edited test files and confirm the only changes are the `Form:` identifier additions.
- [ ] **Step 6: commit** `M9c-3 task 1: MTForm and TagFill with per-form ownership enforced by V9`.

---

### Task 2: the derived short window and form refusal on the short API

**Files:** Modify `core/cat/mt.go`, `core/cat/allowlist.go:201`; append tests to `core/cat/mtform_test.go`.

**Interfaces:**
- Produces: `func (d Dialect) MTForm() MTForm`, `func (d Dialect) MWWriteKind() byte`, `func (d Dialect) Clarifier() ClarifierPolicy` (all exported — `dialecttest` (Task 7), the driver fixes (Task 9) and M9c-4/5 consume them; the accessors live HERE because Task 7 needs `MWWriteKind()` and is sequenced before Task 9); unexported `func (d Dialect) mtShortAnswerMax() int` returning exactly `mtAnswerMinLen + d.mt.TagMaxBytes` (`mtAnswerMinLen` is 7 and already carries the zero-length tag, so this is 19 for the FT-710 — both reviewers flagged revision 1's contradictory minus-one text; THIS formula is the only one); short APIs refuse on `MTFormCombined`.

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
rendering `7-19` for the FT-710. `allowlist.go:201` uses the same
method, and **`allowlist.go:179`'s comment** ("Set:
mtAnswerMinLen-mtAnswerMaxLen (7-19) bytes") is updated — it would
otherwise reference the deleted identifier. `BuildMTRead` stays
form-independent. Add the three accessors (`MTForm()`, `MWWriteKind()`,
`Clarifier()` — each a two-line method with a doc comment naming its
consumers). Update `mt.go:14-26`'s constant commentary (it documented
this exact future).
- [ ] **Step 4: peer churn sweep — expected result: NO churn.** Both reviewers verified every existing non-12-`TagMaxBytes` fixture already sits within its future derived window (the 6-byte peer's longest frames are exactly 13 bytes = its new max; the corpora are FT-710-only), so `go test ./core/cat/` should pass with NO test edits. Do not manufacture any. If a test DOES fail here, that is new information both reviews missed — STOP and report rather than editing expectations. Then the corpus: `go test ./core/cat/ -run 'Corpus' -v` — byte-identical, no golden touched.
- [ ] **Step 5: verify + commit** (gates as Task 1). `M9c-3 task 2: the short MT window derives from the receiver; short APIs refuse a combined dialect`.

---

### Task 3: field-block extraction (MR/MW byte-identical)

**Files:** Modify `core/cat/mr.go`, `core/cat/mw.go`, `core/cat/memdata.go`.

**Interfaces:**
- Produces: `func (d Dialect) parseMemoryFields(frame []byte, wantPrefix string) (MemoryData, error)` — validates and decodes offsets 2-26 ONLY, threading `wantPrefix` solely for error text (no length, no prefix, no terminator, no kind-vocabulary narrowing beyond today's `validKindByte`); `func encodeMemoryFields(frame []byte, m MemoryData)` — writes offsets 2-26 into a caller-sized buffer. **`parseMemoryFrame`'s order is EXACTLY today's: length → prefix → terminator → `parseMemoryFields`** — both reviewers caught that revision 1 omitted the prefix check and would have swapped prefix/terminator error order for doubly-bad frames. `ParseMTAnswerCombined` (Task 4) uses the same order: length → prefix → terminator → fields → kind-narrow/P11/tag. `BuildMWSet`'s body writes via `encodeMemoryFields`.

- [ ] **Step 1: pin the error order first.** In NEW file `core/cat/memfields_test.go`, write `TestParseMemoryFrame_DoublyInvalidFrameErrorOrder`: a 28-byte frame with BOTH a bad prefix and a bad terminator must yield the PREFIX error (exact `Reason` string-compare); a 28-byte frame with a good prefix and bad terminator yields the terminator error; a wrong-length frame yields the length error whatever else is wrong. Nothing pins this today (the reject tables assert error type only), which is exactly why the refactor must pin it before moving code. These pass against TODAY's code — write and run them green BEFORE extracting, then keep them green through the refactor.
- [ ] **Step 2**: extract exactly; keep every error message byte-identical (they all carry `wantPrefix`).
- [ ] **Step 3: verify hard** — full `go test ./core/cat/` (corpus + goldens + evidence literals + the new order pins), `go test ./core/driver/ft710/ ./internal/fakeradio/ ./core/transport/`, gates as Task 1.
- [ ] **Step 4: commit** `M9c-3 task 3: extract the shared memory field block encoder/decoder; MR/MW byte-identical, error order pinned`.

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
- Build validation is **one shared Dialect method,
  `validateCombinedMTFields(m MemoryData) error`** (the gate reuses it in
  Task 5, and Task 8 adds its name to `gateReachingValidators`). Its
  checklist is `validateMWFields`' COMPLETE list (`mw.go:85-151`) with
  exactly two substitutions and nothing silently dropped — both
  reviewers caught revision 1 omitting two rules:
  1. slot via `mtSlotValid` (MT's own write policy — memory/PMS only;
     behaviourally identical to `writableSlot` today, but the MT
     lineage);
  2. kind: `m.Kind == combinedMTSetKind` (the schema constant, NOT
     `d.mwWriteKind`; error text explains Set-'0' is "(Fixed)");
  3. mode via `d.ParseMode` round-trip **AND the `ModeUnset` refusal**
     (`mw.go:128-130` — revision 1 dropped it);
  4. clarifier via `d.validClarHz`;
  5. **`FreqHz != 0` AND `<= memFreqMax`** (`mw.go:136` checks both —
     revision 1 dropped the nonzero half);
  6. CTCSS and Shift by re-parse round-trip (`ParseCTCSSState`/
     `ParseShift` — there are no "Valid predicates"; revision 1
     misnamed them).
- Tag (builder INPUT rules — these apply to the LOGICAL tag only, see
  Task 5 for why the gate must not reuse them): charset per
  `validMTTagByte`; `len(tag) <= d.mt.TagMaxBytes`; refuse a NON-EMPTY
  logical tag ending in `TagFill` ("trailing fill byte would not
  round-trip; trim it") — a tag of ONLY fill bytes is thereby refused
  too, `""` being its canonical spelling. Validate, don't canonicalise.
- Emission: buffer of `29 + d.mt.TagMaxBytes`; `'M','T'`;
  `encodeMemoryFields` (m.Kind already equals the constant — validated,
  not overwritten); P11 `'0'` at offset 27; tag + `TagFill` padding to
  exactly TagMaxBytes at 28..; `';'` last. Empty tag → all-fill field
  (no distinct clear form).
- **Every refusal returns `(Command{}, err)`** — the zero-Command
  contract `command.go:50-52` documents for all fallible builders.
  `command_test.go`'s builder enumeration is golden-pinned and must NOT
  be spliced; instead `mtcombined_test.go` asserts, for each refusal
  case, both a non-nil error and `cmd.IsZero()` (Codex plan finding 2).
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
  rules above. **Step 4 — the documentation this task makes false**
  (Codex plan finding 10), updated in the same commit:
  `core/cat/doc.go:8-15` (the memory-mutating mechanisms list gains
  `BuildMTSetCombined`, consistent with Task 8's fence);
  `core/cat/dialect.go:21-26,44-46` ("carries DATA, not frame shapes"
  and "per-command frame shape is M9c's" — rewrite to record that M9c-3
  delivered exactly that seam, as `MTForm` data plus form branches);
  `core/cat/memdata.go:40-43` (`MemoryData` now also feeds the combined
  MT frame). **Step 5** run all; gates as Task 1. **Step 6: commit**
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
  terminator, then `parseMemoryFields` + `validateCombinedMTFields`
  (Task 4's shared method — slot/kind/mode/clarifier/freq/CTCSS/shift in
  one place) + P11 + **the RAW tag field validated per-byte with
  `validMTTagByte` ONLY — no trailing-fill rule at the gate.** Codex
  plan finding 8: every padded wire field necessarily ends in `TagFill`
  (an empty tag is ALL fill), and the wire erases the data-vs-padding
  distinction, so applying the builder's logical-input suffix refusal to
  the raw field would make the gate reject its own builder's every
  non-full-width tag. The builder-input rule and the gate-wire rule are
  different rules on different representations; say so in the gate's
  comment. Document the narrowed exception in the function comment and
  in `allowlist.go`'s exception-table commentary.
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
- [ ] **Step 2/3**: implement `Run` using ONLY exported API — all
  verified available by review: `MTForm()`, `MTAnswerBounds()`,
  `MWWriteKind()` (Task 2's accessor — this is why the accessors moved
  there; Fable plan finding 6 caught the original T7-before-T9 ordering
  hole), `ParseSlot`/`MemorySlot`/`PMSSlot`, `EXAddresses`,
  `ParseCTCSSState`, `ParseShift`, builders, parsers, `AllowedCommand`.
  There is no `AllModes` — iterate the mode wire space exhaustively
  through `ParseMode`, as the gate walk does with `threeDigits` for
  slots.
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
  `BuildMTSetCombined` call in an unauthorised package — **in a
  NON-test file** (the guard walker skips `_test.go` entirely, so a
  test-file decoy fires nothing; Fable plan finding 11) → fence fires;
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
- [ ] **Step 2/3**: the accessors already exist (Task 2). Replace
  `write.go:256`'s `Kind: cat.KindMemory` with
  `Kind: dialect.MWWriteKind()`, and rework the clarifier pre-check:
  today `:237` hardcodes the bound in the COMPARISON and `:240`
  independently hardcodes `"+/-9990"` in the FORMAT STRING (both
  reviewers caught revision 1's claim that the text already
  interpolates — it does not). Bind the policy once and interpolate:

```go
	clar := dialect.Clarifier()
	if data.ClarHz > clar.MaxAbsHz || data.ClarHz < -clar.MaxAbsHz {
		... fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz) ...
	}
```

  Byte-identical for the FT-710 (`MaxAbsHz` 9990 renders the same
  bytes), receiver-correct for everyone else. Match the surrounding
  error-construction style exactly.
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
declared in T4 and consumed in T7; `MTForm()`/`MWWriteKind()`/
`Clarifier()` declared T2, consumed T7 and T9;
`validateCombinedMTFields` declared T4, consumed T5, named in T8;
`combinedMTSetKind` declared T4, consumed T5. **Ordering:** T2 needs
T1's Form field; T4 needs T3's extraction; T5 needs T4's builder AND
its shared validator; T6 needs T4+T5; T7 needs T2's three accessors and
T4's bounds; T8 needs T4-5's names; T9 needs T2's accessors; T10 last.

## Plan review fold (29/07/2026)

Reviewed before execution by Codex (NEEDS-REVISION: 4 HIGH, 5 MEDIUM,
1 LOW) and Fable (APPROVE-WITH-FIXES: 2 HIGH, 4 MEDIUM, 5 LOW);
adjudication in `.superpowers/sdd/m9c3-plan-review-adjudication.md`.
Seven findings were convergent. The design-relevant outcomes, all folded
above: the evidence-literal model corrected (the golden's real scope;
`seconddialect_test.go` excluded; a passing pin test proves nothing
about new literals); the V9 ceiling became a constants-relationship
test (the runtime branch is dead code behind the 64-byte cap); the V9
sketch now carries the short-form `ClearTagByte`/`PadByte` rules
verbatim in the executable block; `parseMemoryFrame`'s order is pinned
(length → prefix → terminator → fields) with a new doubly-invalid
exact-text test BEFORE the refactor; the combined validation checklist
is complete (ModeUnset and `FreqHz != 0` restored) and shared as
`validateCombinedMTFields`; the gate validates the RAW tag per-byte
only — the builder's trailing-fill rule applies to logical input, and
applying it to the padded wire field would reject the builder's own
output; `BuildMTSetCombined` joins the zero-Command contract via
assertions in the new file (the pinned `command_test.go` enumeration is
not spliced); the accessors moved to Task 2 (Task 7 needed
`MWWriteKind()` before Task 9 existed); the driver's clarifier error
text is rewritten to interpolate the bound (it never did); doc
call-sites (`doc.go`, `dialect.go`, `memdata.go`) assigned to Task 4;
Task 2's expected churn corrected to NONE (verified by both reviewers —
a failing test there is new information, not something to edit around).
