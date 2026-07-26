# M9c-0 — Exported `Dialect` Constructor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Revision 2 (26/07/2026).** Revision 1 was reviewed adversarially by Codex and returned **NEEDS-REVISION with 12 findings, 7 HIGH**. All twelve verified against source and **accepted**; the adjudication table is at the end and the transcript is at `.superpowers/sdd/m9c0-codex-plan-review.md`. Revision 1's task split, its baseline commands, its equivalence test and its gate property were all defective. **Do not execute revision 1** (git `960b0d0`).

**Goal:** Give `cat.Dialect` an exported, validating constructor so `core/cat/ftdx10` can exist, and promote the five FT-710 facts that reach the outbound write gate into dialect data — with FT-710 behaviour unchanged.

**Architecture:** A flat `DialectConfig` (plus `SlotSpace`, `MTPolicy`, `ClarifierPolicy`) validated by `NewDialect`, which copies every input and derives the EX indices. `MustNewDialect` serves compile-time model tables so roadmap A1's `func Dialect() cat.Dialect` signature stands. Five gate-affecting constants move onto the receiver; frame offsets stay put.

**Tech Stack:** Go 1.25, stdlib only in `core/`, existing table-driven test style, `go/parser`+`go/ast` for guards.

**Design spec:** `docs/superpowers/specs/2026-07-26-m9c0-dialect-constructor-design.md` (**revision 2.1**). Where this plan and the spec disagree, **the spec wins and the plan is wrong** — say so rather than guessing.

## Global Constraints

- **stdlib only in `core/`.** No new dependencies anywhere.
- **SPDX header** `// SPDX-License-Identifier: GPL-3.0-or-later` on every new file.
- **British English** in user-facing copy. "Snapshot", never "backup".
- **FT-710 behaviour is unchanged**, stated precisely: CAT frames, golden and hardware-derived vectors, codeplug JSON, digest, schema (stays 2) and CLI output are byte-identical. This is a read/render + corpus claim; no hardware runs.
- **Never regenerate a golden.** `core/cat/testdata/` is not to be rewritten. If a corpus fails, the change is wrong, not the corpus.
- **The receiver must be load-bearing.** For any datum this milestone promotes, no package-level fallback may remain — in the predicate *or in the diagnostic it emits*.
- **The gate may never be weakened.** No accepted config may cause a byte outside the permitted wire-byte domain to appear in a frame's **interior**. The domain is printable ASCII `0x20`–`0x7E` excluding `';'`; the terminator is the one `';'` a frame is required to end with.
- **Every rule needs a rejecting test AND an accepting test, per clause** — not per numbered rule. A file of "rejects X" assertions all pass when the rejection helper is stubbed to `return false`.
- **Assert on error content, not just non-nil.** A validator returning a generic error must fail its test.
- **Run every command before writing it into a report.** Revision 1's baseline commands were written without being executed and did not work. The sequence in Task 60 has been run; if you change it, run it again.
- CI is billing-dead; the full local gate substitutes.

## Process

- Branch: `m9c0-dialect-constructor`; merge to `main` at milestone end.
- Task numbering continues the ledger. Next free: **task-60**.
- Each task: brief → fresh implementer → TDD → opus review gate → report, in `.superpowers/sdd/`.
- Milestone end: Codex adversarial review → adjudicate → fix wave → re-review → merge.

## File Structure

| File | Responsibility |
|---|---|
| `core/cat/dialectconfig.go` **(new)** | `SlotSpace`, `MTPolicy`, `ClarifierPolicy`, `DialectConfig`, `NewDialect`, `MustNewDialect` |
| `core/cat/dialectvalidate.go` **(new)** | Wire-byte domain helpers and the eleven rules, one function each |
| `core/cat/dialect.go` | `Dialect` gains `mt`, `clar`, `mwWriteKind`, `modeByName`; `ModeByName` accessor |
| `core/cat/mt.go` | `mtTagMaxBytes`/`mtClearTag` deleted; tag validation, clear emission, clear **decoding** and diagnostics all receiver-borne |
| `core/cat/memdata.go`, `core/cat/mr.go` | `clarMaxAbsHz`/`clarStepHz` deleted; `validClarHz` and its diagnostics receiver-borne |
| `core/cat/mw.go` | `KindMemory` literal replaced by `d.mwWriteKind`; diagnostic derived |
| `core/driver/ft710/caps.go`, `write.go` | Write path resolves modes through `Dialect.ModeByName` |
| `core/cat/seconddialect_test.go` | Peer fixtures rebuilt through `NewDialect` |
| `core/cat/dialectexternal_test.go` **(new, `package cat_test`)** | Proves an *external* package can construct a dialect |
| `internal/guards/dialectglobals_test.go` **(new)** | Reproducible transitive audit |
| `docs/superpowers/m9c0-baseline-manifest.md` **(new, tracked)** | Baseline hashes and the verified command sequence |

---

### Task 60: Baselines before anything moves

**Files:** Create `docs/superpowers/m9c0-baseline-manifest.md`. No source changes.

**This exact sequence has been run and produces seven non-empty artefacts.** Revision 1's did not work: `settings` has no `--fake` and takes a file argument (`cmd/rigprog/settings.go:32`), `read` uses `--out` not `-o` (`cmd/rigprog/read.go:115`), and `export` uses `--csv OUT FILE` (`cmd/rigprog/export.go:23`).

- [ ] **Step 1: Capture, failing fast.** `set -e` matters: without it a failed command leaves an empty file that gets hashed as a baseline.

```bash
set -e
git rev-parse HEAD                       # record as BASE in the manifest
B=.superpowers/sdd/m9c0-baselines        # relative, inside the repo, git-ignored
mkdir -p "$B"
go run ./cmd/rigprog probe --fake                                            > "$B/probe-fake.txt" 2>&1
go run ./cmd/rigprog read --fake --settings --out "$B/read-fake.json"        > "$B/read-fake.txt"  2>&1
go run ./cmd/rigprog settings "$B/read-fake.json"                            > "$B/settings.txt"   2>&1
go run ./cmd/rigprog export --csv "$B/export.csv" "$B/read-fake.json"        > "$B/export.txt"     2>&1
go run ./cmd/rigprog help                                                    > "$B/help.txt"       2>&1
```

- [ ] **Step 2: Prove nothing silently failed.**

```bash
find "$B" -size 0 -print          # MUST print nothing
cd "$B" && shasum -a 256 *
```

Expected orders of magnitude from the verified run: `export.csv` ≈ 2.8 kB, `read-fake.json` ≈ 35 kB, `settings.txt` ≈ 13 kB, `help.txt` ≈ 0.9 kB. A file far from these is a failed command, not a baseline.

- [ ] **Step 3: Hash the corpora too.** `shasum -a 256 core/cat/testdata/*.golden`

- [ ] **Step 4: Write the manifest** — BASE commit, every hash, the sequence above verbatim, and the **normalisation rule**: `read-fake.json`'s `read_at` and the echoed `Output:` paths in `read-fake.txt`/`export.txt` are the *only* permitted differences. **Schema and baseline digest stay inside the comparison** — they are exactly what a mistake would change. Any other difference is a defect, never a baseline to update.

- [ ] **Step 5: Commit.** `"m9c0: task 60 — baselines before the constructor"`

**Depends on:** nothing.

---

### Task 61: Config types, the wire-byte domain, and the eleven validators

**Files:** Create `core/cat/dialectconfig.go` (types only), `core/cat/dialectvalidate.go`, `core/cat/dialectvalidate_test.go`.

Types exactly as the spec's API block, noting **`MWWriteKind byte`** — there is no `Kind` type; `MemoryData.Kind` is a plain `byte` and the `Kind*` constants are `byte` (`core/cat/memdata.go:76`, `:82`).

- [ ] **Step 1: The domain, tested over all 256 bytes.** Sampling six rejects would let an untested control byte through.

```go
func TestValidWireByte_EveryByteValue(t *testing.T) {
	for i := 0; i < 256; i++ {
		b := byte(i)
		want := b >= 0x20 && b <= 0x7E && b != ';'
		if got := validWireByte(b); got != want {
			t.Errorf("validWireByte(%#02x) = %v, want %v", b, got, want)
		}
	}
}
```

- [ ] **Step 2: Run, confirm it fails**, then implement `validWireByte`/`validWireString`. `';'` is excluded because it TERMINATES a frame: a dialect datum carrying one would split a single command into two on the wire.

- [ ] **Step 3: Write the eleven validators**, one function per rule, each returning an error that **names the field and the offending value**. Rules and their reasons are the spec's validation table — read it; do not infer them from this plan.

- [ ] **Step 4: Test every CLAUSE, both directions, asserting on error text.** V2 alone has four clauses (empty map, empty name, duplicate name, key outside the domain); V4 has four (two fields × absence form, length, domain). A `wantErr bool` check alone passes for a validator that returns a generic error from the wrong branch.

```go
tests := []struct {
	name       string
	mutate     func(*DialectConfig)
	wantErrHas string // "" = must succeed
}{
	{"baseline is valid", func(*DialectConfig) {}, ""},
	{"V2 empty mode map", func(c *DialectConfig) { c.ModeNames = map[Mode]string{} }, "ModeNames"},
	{"V2 empty mode name", func(c *DialectConfig) { c.ModeNames[Mode('2')] = "" }, "ModeNames"},
	{"V2 duplicate mode name", func(c *DialectConfig) { c.ModeNames[Mode('9')] = c.ModeNames[Mode('2')] }, "duplicate"},
	{"V2 mode key outside wire domain", func(c *DialectConfig) { c.ModeNames[Mode(0x00)] = "NUL" }, "0x00"},
	{"V3 pmsPairs 10 rejected not clamped", func(c *DialectConfig) { c.Slots.PMSPairs = 10 }, "PMSPairs"},
	{"V5 MemoryLo 0 is PERMITTED", func(c *DialectConfig) { c.Slots.MemoryLo = 0 }, ""},
	{"V5 dead absence 99..0", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.MemoryHi = 99, 0 }, "MemoryLo"},
	{"V7 noneWire shadows memory", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.NoneWire = 0, "000" }, "NoneWire"},
	{"V7 emergencyWire collides with PMS", func(c *DialectConfig) { c.Slots.PMSPairs, c.Slots.EmergencyWire = 1, "P1L" }, "EmergencyWire"},
	{"V8 EX component over 99", func(c *DialectConfig) { c.EXItems[0].Addr.P1 = 100 }, "P1"},
	// ...every remaining clause of every rule, both directions
}
```

**`MemoryLo: 0` must be ACCEPTED.** `noneWireDialect` depends on it, and revision 1's `>= 1` rule would have rejected the fixture the sufficiency proof needs.

- [ ] **Step 5: Commit.** `"m9c0: task 61 — config types, wire-byte domain, eleven validators"`

**Depends on:** Task 60.

---

### Task 62: `NewDialect`, `MustNewDialect`, storage and initialisation

**Files:** `core/cat/dialectconfig.go`, `core/cat/dialect.go`, `core/cat/seconddialect_test.go`, tests.

Revision 1 said Task 62 "gains no new fields" *and* "lands all three fields", then told Tasks 63–65 to add them again — following it literally produced duplicate fields. **This task adds and populates all four new fields; Tasks 64–67 only reroute consumers.**

- [ ] **Step 1: Add the fields** to `Dialect`: `mt MTPolicy`, `clar ClarifierPolicy`, `mwWriteKind byte`, `modeByName map[string]Mode`.

- [ ] **Step 2: `NewDialect`** validates (Task 61's rules), **copies** `ModeNames` and `EXItems` into fresh containers, then derives `exMembers`, `exByTriple`, `exP4Max` **and** `modeByName` from the copies.

- [ ] **Step 3: `MustNewDialect`** panics on error. Its doc says it exists for compile-time-constant model tables where failure is a build-time programming error, and **forbids** its use on caller-supplied data.

- [ ] **Step 4: Initialise every existing configured literal.** This is the step revision 1 missed entirely: once clarifier validation is receiver-based, a literal with a zero `StepHz` reaches a modulo by zero. Populate the new fields on `FT710` (`core/cat/dialect.go:78`) and on all three fixtures — `testDialect` (`:62`), `noneWireDialect` (`:93`), `peerDialect` (`:143`).

- [ ] **Step 5: Test both `MustNewDialect` outcomes** — a valid config returns a working dialect; an invalid one panics (`defer recover()`).

- [ ] **Step 6: Run the full package.** Green, and `git status` shows `core/cat/testdata/` untouched.

- [ ] **Step 7: Commit.** `"m9c0: task 62 — NewDialect, MustNewDialect, field storage and initialisation"`

**Depends on:** Task 61.

---

### Task 63: Equivalence, input independence, external construction

**Files:** `core/cat/dialectconfig_test.go`, new `core/cat/dialectexternal_test.go` (`package cat_test`).

- [ ] **Step 1: FT-710 equivalence, from genuinely independent data.** Revision 1 passed `modeNames` and `exItemsGen` — *the very objects the production literal uses* (`dialect.go:80`, `:88`) — so it compared the literal against itself. Write the mode table and EX items out as fresh literals in the test.

- [ ] **Step 2: Make the helper complete.** Revision 1's would pass while `FT710` had zero policies or a corrupted EX index. `assertDialectsBehaveIdentically` must compare:
  - `CATID()`; `ValidMode`/`ModeName` over **all 256** byte values;
  - `classifySlot` over an **exhaustive** corpus: every 3-digit numeric `000`–`999`, every `P<1-9><L|U>`, both special wires, and a sample of malformed forms;
  - `EXItems()` **exactly** — length, order, and every field including metadata;
  - `EXAddresses()` order;
  - **both** EX lookup paths — `KnownEXAddress` and `NewEXAddress`/`ParseEXAddress`, which is how `exByTriple` becomes observable;
  - `exP4MaxBytes()` and all three policies.

- [ ] **Step 3: Input independence.** Mutate the caller's map and slice after construction; assert unchanged behaviour through **all three** derived EX structures (`exMembers`, `exByTriple`, `exP4Max`) plus `modeByName`.

- [ ] **Step 4: External construction proof**, in `package cat_test`, so it cannot reach package internals:

```go
// Proves the EXPORTED API alone is sufficient — the property M9c depends
// on. An in-package test can pass while quietly using an unexported field.
func TestExternalPackageCanConstructADialect(t *testing.T) { /* cat.NewDialect(...) */ }
```

- [ ] **Step 5: Commit.** `"m9c0: task 63 — equivalence, input independence, external construction"`

**Depends on:** Task 62.

---

### Tasks 64–67: reroute consumers onto the receiver

These four are **independent of one another** and may run in any order or in parallel: their production paths are MT, clarifier, MW Kind and the driver's mode lookup respectively, and Task 62 already landed every field. Each follows the same shape — **write the failing peer test first, confirm it fails, then move the datum.**

Each task must also **derive its diagnostics from receiver policy** while leaving FT-710's rendered strings byte-identical. Moving only the predicate yields correct acceptance with false peer-facing error text (`mt.go:103` "0-12 bytes", `mr.go:109` and `mw.go:114` "10 Hz … 9990").

#### Task 64: MT tag policy

- [ ] Peer with `TagMaxBytes: 6`, `ClearTagByte: '-'`. Assert it accepts a 6-byte tag, rejects 7, and clears with six `'-'`.
- [ ] **Move BOTH directions.** Revision 1 moved emission (`mt.go:106`) and said nothing about decoding — but `ParseMTAnswer` trims **spaces only** (`mt.go:201`), so the peer's cleared tag parses back as `"------"`, not empty, failing the spec's round-trip requirement. Clear-form decoding must recognise `TagMaxBytes` repetitions of `ClearTagByte` as empty, preserving FT-710's existing space behaviour exactly.
- [ ] **Round-trip test:** build → gate → parse, asserting an empty tag survives as empty for both dialects.
- [ ] Commit: `"m9c0: task 64 — MT tag policy onto the receiver"`

#### Task 65: Clarifier policy

- [ ] Peer with `StepHz: 1, MaxAbsHz: 9999`. **Test 7 Hz AND 9999 Hz.** 7 Hz alone proves only `StepHz`: it sits inside both ranges, so an implementation taking the step from the receiver while keeping the global `clarMaxAbsHz` passes. **9999 is the load-bearing range mutation.**
- [ ] For each value assert: builder succeeds, parser round-trips, the peer's own gate admits it, and FT-710 rejects it.
- [ ] Commit: `"m9c0: task 65 — clarifier policy onto the receiver"`

#### Task 66: MW write Kind

- [ ] Peer with `MWWriteKind: KindPMS` accepts a PMS-kind MW write its own gate admits, while FT-710 still rejects that exact frame.
- [ ] `mw.go:98`'s `m.Kind != KindMemory` becomes `m.Kind != d.mwWriteKind`. **Preserve the M5b hardware evidence in substance** — a PMS write carrying `KindPMS` is rejected by the real radio — and label it explicitly FT-710-specific. Revision 1 said "preserve verbatim" *and* "rescope", which is self-contradictory: rescoping is the point, and the evidence must survive it.
- [ ] Commit: `"m9c0: task 66 — MW write Kind onto the receiver"`

#### Task 67: `ModeByName` and the driver write path

- [ ] `func (d Dialect) ModeByName(name string) (Mode, bool)`, served from the index Task 62 built.
- [ ] Route `core/driver/ft710/write.go:210` through `s.dialect.ModeByName`. This is what makes V2's uniqueness rule *mean* something: today `modeByName` is built from the driver's own `modeTable`, independent of the dialect.
- [ ] Keep `modeTable` for capability rendering, and **pin its equivalence to the dialect** — it remains a second transcription.
- [ ] Extend `assertDialectsBehaveIdentically` to cover `ModeByName`.
- [ ] Commit: `"m9c0: task 67 — ModeByName; the write path resolves through the dialect"`

**All four depend on:** Task 63.

---

### Task 68: Peer fixtures through the public API, and gate integrity

**Files:** `core/cat/seconddialect_test.go`, new `core/cat/dialectgate_test.go`.

- [ ] **Step 1: Rebuild the three peer fixtures through `NewDialect`.** If any cannot be expressed, **stop and report** — that is the API being wrong, and it is the finding this task exists to produce. Any fixture that must stay a literal is named in the file with its reason.

- [ ] **Step 2: The gate property, correctly stated.** Revision 1's would not have caught the NUL finding it was written for: every existing fixture already uses safe bytes, so deleting V2/V4's domain checks left it green. And "no frame contains a byte outside the domain" is **literally false** — every frame ends with the `';'` the domain excludes.

  - Drive **adversarial configs** varying mode keys, special slot wires, CATID and clear bytes — including ones `NewDialect` must reject.
  - For each **accepted** config, build the affected command and scan `frame[:len(frame)-1]` for domain violations.
  - Separately assert the frame ends with **exactly one** `';'` and contains no other.
  - Assert a **non-vacuity count** per builder and per dialect: a property that ran zero times passes silently.

- [ ] **Step 3: Prove each assertion fails for its own reason.** Temporarily break one builder, confirm exactly the expected test goes red, restore.

- [ ] **Step 4: Commit.** `"m9c0: task 68 — peer fixtures through the public API; gate integrity"`

**Depends on:** Tasks 64–67.

---

### Task 69: Reproducible audit, byte-identity, gate, docs

**Files:** new `internal/guards/dialectglobals_test.go`; manifest comparison; ledger.

- [ ] **Step 1: Make the transitive audit reproducible.** The spec quotes a result with no committed way to re-derive it — exactly the "claim written without running the mechanism" failure it warns about. Write a guard that walks from each `Dialect` method **transitively through the package-level functions it calls** (never descending into `SelectorExpr.Sel`, which is a field name, not a package identifier) and reports the reachable `const`/`var` set.

  Assert **usage, not absence**: `KindMemory` stays legitimately reachable via `validKindByte`. The check is that `validateMWFields`, `validClarHz` and `validMTTag` read the **receiver**.

- [ ] **Step 2: Re-capture all seven artefacts** with Task 60's sequence and compare. Revision 1 compared only four and never hashed the codeplug JSON — so schema, digest and JSON content could all have changed silently. Normalise `read_at` and echoed paths **only**.

- [ ] **Step 3: Confirm no golden was regenerated:** `git log --oneline -- core/cat/testdata/` shows no new commit.

- [ ] **Step 4: Full local gate.**

```sh
gofmt -l .                                   # empty
go vet ./... && go build ./... && go test ./...
go test ./internal/guards/ -v
go test -race ./core/...
cd app && wails generate module && git diff --exit-code frontend/wailsjs
cd frontend && npm run check && npm run test && npm run build
```

- [ ] **Step 5: Milestone summary** at `.superpowers/sdd/m9c0-milestone-summary.md`, scoping every claim: what the peer fixtures prove (the receiver is consulted) versus what they do not (that any real radio differs).

- [ ] **Step 6: Ledger, commit, Codex adversarial milestone review.**

**Depends on:** Task 68.

## Dependency graph

```
60 -> 61 -> 62 -> 63 -> (64 | 65 | 66 | 67) -> 68 -> 69
```

Tasks 64–67 are genuinely independent once Task 62 has landed and populated the fields — revision 1 serialised them on a conflict rationale that contradicted its own claim that Task 62 prevented reopening the struct.

## Verification

The spec's Verification section is authoritative. In summary: FT-710 byte-identity across all seven artefacts against Task 60's manifest; the peer fixtures rebuilt through the public constructor; an external-package construction proof; gate integrity under adversarial caller-built dialects with non-vacuity counts; every validator clause tested both directions with error-content assertions; input independence across all derived structures; and a reproducible transitive audit asserting receiver use.

## Codex plan review — adjudication

All twelve findings **accepted**; seven were HIGH.

| # | Sev | Disposition |
|---|---|---|
| 1 | HIGH | Task 60's commands were wrong and untested. Replaced with a sequence **that has been run**, producing seven non-empty artefacts; `set -e` and an empty-file check added. |
| 2 | HIGH | `MWWriteKind Kind` referenced a nonexistent type. Spec corrected to `byte` (revision 2.1); no alias introduced. |
| 3 | HIGH | The field lifecycle was self-contradictory and would have produced duplicate fields. Task 62 now adds **and populates** all four, including `FT710` and all three fixtures; 64–67 only reroute. |
| 4 | HIGH | The equivalence test compared the literal against itself and omitted policies, EX metadata, ordering and `exByTriple`. Rewritten with independent literals and an exhaustive helper. |
| 5 | HIGH | 7 Hz proves only `StepHz`. 9999 Hz added as the range mutation. |
| 6 | HIGH | MT clear **decoding** was never moved; `ParseMTAnswer` trims spaces only, so the peer's cleared tag would not round-trip. Both directions now in scope. |
| 7 | HIGH | The gate property would not have caught finding 2, and was literally false about the terminator. Rewritten: adversarial configs, interior-only scan, exactly-one-terminator, non-vacuity counts; Task 61 now exhausts all 256 bytes. |
| 8 | MED | Testing is now per **clause** with error-content assertions, both `MustNewDialect` outcomes, `exByTriple` in the mutation test, and an external `package cat_test` proof. |
| 9 | MED | All seven artefacts captured and compared, not four; schema and digest stay inside the comparison. |
| 10 | MED | Task 69 adds a committed transitive-audit guard asserting **receiver use**, not identifier absence. |
| 11 | MED | Tasks 64–66 must derive diagnostics from receiver policy; Task 66's "verbatim **and** rescoped" contradiction resolved in favour of preserving the evidence's substance while labelling it FT-710-specific. |
| 12 | MED | Re-split: old Task 61 folded into the validation slice; old Task 62 split three ways (61/62/63); 64–67 unserialised. |
