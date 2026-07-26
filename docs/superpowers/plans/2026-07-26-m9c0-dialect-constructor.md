# M9c-0 — Exported `Dialect` Constructor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `cat.Dialect` an exported, validating constructor so `core/cat/ftdx10` can exist, and promote the five FT-710 facts that reach the outbound write gate into dialect data — with FT-710 behaviour unchanged.

**Architecture:** A flat `DialectConfig` (plus `SlotSpace`, `MTPolicy`, `ClarifierPolicy`) validated by `NewDialect`, which copies every input and derives the EX indices. `MustNewDialect` serves compile-time model tables so roadmap A1's `func Dialect() cat.Dialect` signature stands. Five gate-affecting constants move onto the receiver; frame offsets stay put.

**Tech Stack:** Go 1.25, stdlib only in `core/`, existing table-driven test style, `go/parser`+`go/ast` for guards.

**Design spec:** `docs/superpowers/specs/2026-07-26-m9c0-dialect-constructor-design.md` (**revision 2** — do not implement revision 1). Where this plan and the spec disagree, **the spec wins and the plan is wrong** — say so rather than guessing.

## Global Constraints

- **stdlib only in `core/`.** No new dependencies anywhere.
- **SPDX header** `// SPDX-License-Identifier: GPL-3.0-or-later` on every new file.
- **British English** in all user-facing copy. "Snapshot", never "backup".
- **FT-710 behaviour is unchanged**, stated precisely: CAT frames, golden and hardware-derived vectors, codeplug JSON, digest, schema (stays 2) and CLI output are byte-identical. This is a read/render + corpus claim; no hardware runs.
- **Never regenerate a golden.** `core/cat/testdata/` is not to be rewritten. If a corpus fails, the change is wrong, not the corpus.
- **The receiver must be load-bearing.** A method taking a `Dialect` that reads a package global has the shape of a seam and none of the substance. For any datum this milestone promotes, no package-level fallback may remain.
- **The gate may never be weakened.** `Dialect.AllowedCommand` is what stands between this program and a radio. No accepted config may cause a byte outside the permitted wire-byte domain (printable ASCII `0x20`–`0x7E` excluding `';'`) to be emitted or admitted.
- **Every validation rule needs a rejecting test AND an accepting test.** A file of "rejects X" assertions all pass when the rejection helper is stubbed to `return false`.
- **Errors are returned, never panicked** — except `MustNewDialect`, which exists for that purpose and is forbidden on caller-supplied data.
- CI is billing-dead; the full local gate substitutes.

## Process

- Branch: `m9c0-dialect-constructor`; merge to `main` at milestone end.
- Task numbering continues the ledger. Next free: **task-60**.
- Each task: brief → fresh implementer → TDD → opus review gate → report, all in `.superpowers/sdd/`.
- Milestone end: Codex adversarial review → adjudicate → fix wave → re-review → merge.

## File Structure

| File | Responsibility |
|---|---|
| `core/cat/dialectconfig.go` **(new)** | `SlotSpace`, `MTPolicy`, `ClarifierPolicy`, `DialectConfig`, `NewDialect`, `MustNewDialect` |
| `core/cat/dialectvalidate.go` **(new)** | The eleven rules, one function each, plus the wire-byte domain helpers |
| `core/cat/dialect.go` | `Dialect` gains `mt`, `clar`, `mwWriteKind` fields and their accessors; `ModeByName` |
| `core/cat/mt.go` | `mtTagMaxBytes`/`mtClearTag` deleted; `validMTTag` becomes a method |
| `core/cat/memdata.go` | `clarMaxAbsHz`/`clarStepHz` deleted; `validClarHz` becomes a method |
| `core/cat/mw.go` | `KindMemory` literal replaced by `d.mwWriteKind` |
| `core/driver/ft710/caps.go`, `write.go` | Write path routes through `Dialect.ModeByName` |
| `core/cat/seconddialect_test.go` | Peer fixtures rebuilt through `NewDialect` |
| `docs/superpowers/m9c0-baseline-manifest.md` **(new, tracked)** | Baseline hashes and reproduction commands |

---

### Task 60: Baselines before anything moves

**Files:** Create `docs/superpowers/m9c0-baseline-manifest.md`. No source changes.

- [ ] **Step 1: Record the base commit.** `git rev-parse HEAD` — this is the byte-identity reference. Put it in the manifest.

- [ ] **Step 2: Capture the four no-normalisation artefacts** into a scratch dir (NOT the repo):

```bash
go run ./cmd/rigprog probe --fake            > probe-fake.txt
go run ./cmd/rigprog settings --fake         > settings.txt
go run ./cmd/rigprog read --fake -o /tmp/m9c0-read.json && \
  go run ./cmd/rigprog export /tmp/m9c0-read.json -o export.csv
go run ./cmd/rigprog --help                  > help.txt
```

- [ ] **Step 3: Hash them and the four corpora.**

```bash
shasum -a 256 probe-fake.txt settings.txt export.csv help.txt
shasum -a 256 core/cat/testdata/*.golden
```

- [ ] **Step 4: Write the manifest** with every hash, the base commit, and the exact commands. Record any artefact with a known noise floor (a timestamp, an echoed path) as a **mismatch to normalise**, never adjusted away.

- [ ] **Step 5: Commit.** `git commit -m "m9c0: task 60 — baselines before the constructor"`

**Depends on:** nothing.

---

### Task 61: The permitted wire-byte domain

**Files:** Create `core/cat/dialectvalidate.go`, `core/cat/dialectvalidate_test.go`.

**Interfaces produced:** `validWireByte(b byte) bool`, `validWireString(s string) bool` — consumed by Tasks 62–65.

- [ ] **Step 1: Write the failing test.**

```go
func TestValidWireByte_Domain(t *testing.T) {
	// ACCEPT: the printable ASCII range, endpoints included.
	for b := byte(0x20); b <= 0x7E; b++ {
		if b == ';' {
			continue
		}
		if !validWireByte(b) {
			t.Errorf("validWireByte(%#02x %q) = false, want true", b, b)
		}
	}
	// REJECT: the terminator, and everything outside printable ASCII.
	for _, b := range []byte{';', 0x00, 0x1F, 0x7F, 0x80, 0xFF} {
		if validWireByte(b) {
			t.Errorf("validWireByte(%#02x) = true, want false", b)
		}
	}
}
```

- [ ] **Step 2: Run it, confirm it fails** with "undefined: validWireByte".

- [ ] **Step 3: Implement.**

```go
// validWireByte reports whether b may appear inside a CAT frame this
// package builds. The domain is printable ASCII excluding ';'.
//
// ';' is excluded because it TERMINATES a frame: a byte of dialect data
// carrying one would split a single command into two on the wire, and the
// gate's whole-frame checks count semicolons rather than re-deriving
// structure. Non-printable bytes are excluded because no CAT field in any
// reference documents one, and admitting them would let a caller-built
// dialect emit a gate-approved frame containing a NUL (Codex spec review,
// finding 2).
func validWireByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E && b != ';'
}

// validWireString reports whether every byte of s is in the domain. Empty
// is true: callers decide separately whether empty is permitted.
func validWireString(s string) bool {
	for i := 0; i < len(s); i++ {
		if !validWireByte(s[i]) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run, confirm pass.**

- [ ] **Step 5: Commit.** `git commit -m "m9c0: task 61 — the permitted wire-byte domain"`

**Depends on:** Task 60.

---

### Task 62: `DialectConfig`, `NewDialect`, and rules V1–V8

**Files:** Create `core/cat/dialectconfig.go`; extend `core/cat/dialectvalidate.go` and its test; new `core/cat/dialectconfig_test.go`.

**Interfaces produced:** the config types and both constructors, exactly as the spec's API section gives them. `Dialect` gains no new fields in this task — `MT`, `Clarifier` and `MWWriteKind` are accepted and validated here but only *stored* from Task 63 on. Land them as fields on `Dialect` now (unused is fine; `go vet` does not object to unused struct fields) so Tasks 63–65 do not each re-open the struct.

- [ ] **Step 1: Write the config and constructor** per the spec's API block verbatim. `NewDialect` must:
  1. validate (V1–V11; V9–V11 are cheap and belong here even though their data is not consumed until 63–65),
  2. **copy** `ModeNames` into a fresh map and `EXItems` into a fresh slice,
  3. derive `exMembers`, `exByTriple`, `exP4Max` from the *copy*.

- [ ] **Step 2: Write the eleven rules**, one function each, in `dialectvalidate.go`. Each returns a descriptive `error` naming the field and the offending value. Exact rules and their reasons are the spec's validation table — **read it, do not infer them from this plan.**

- [ ] **Step 3: Write the rejecting AND accepting tests.** One table per rule. Every rule needs a config that trips it and a minimally-different config that does not:

```go
// Each entry names the ONE field it perturbs from a known-good baseline,
// so a failure identifies the rule rather than the fixture.
tests := []struct {
	name    string
	mutate  func(*DialectConfig)
	wantErr bool
}{
	{"baseline is valid", func(*DialectConfig) {}, false},
	{"V2 mode key outside wire domain", func(c *DialectConfig) { c.ModeNames[Mode(0x00)] = "NUL" }, true},
	{"V2 duplicate mode name", func(c *DialectConfig) { c.ModeNames[Mode('9')] = c.ModeNames[Mode('2')] }, true},
	{"V3 pmsPairs 10 rejected not clamped", func(c *DialectConfig) { c.Slots.PMSPairs = 10 }, true},
	{"V5 MemoryLo 0 is PERMITTED", func(c *DialectConfig) { c.Slots.MemoryLo = 0 }, false},
	{"V5 dead absence 99..0", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.MemoryHi = 99, 0 }, true},
	{"V7 noneWire shadows memory", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.NoneWire = 0, "000" }, true},
	{"V7 emergencyWire collides with PMS", func(c *DialectConfig) { c.Slots.PMSPairs, c.Slots.EmergencyWire = 1, "P1L" }, true},
	{"V8 EX component over 99", func(c *DialectConfig) { c.EXItems[0].Addr.P1 = 100 }, true},
	// ... one per rule, both directions
}
```

**`MemoryLo: 0` must be ACCEPTED.** It is not an oversight: `noneWireDialect` depends on it and revision 1's rule rejected the very fixture the sufficiency proof needs.

- [ ] **Step 4: The FT-710 equivalence test.**

```go
// TestNewDialect_ReproducesFT710 proves the exported API is expressive
// enough for a real radio, and that FT710's own data passes its own
// validation.
//
// The config is built from INDEPENDENT LITERALS, never by reading FT710's
// fields back into a DialectConfig: an expectation derived from the code
// under test moves with it and proves nothing. Same discipline as
// ft710P4MaxBytes in seconddialect_test.go.
func TestNewDialect_ReproducesFT710(t *testing.T) {
	got, err := NewDialect(DialectConfig{
		CATID:     "0800",
		ModeNames: modeNames, // copied by the constructor; compared below
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs: 9, EmergencyWire: "EMG", NoneWire: "000",
		},
		EXItems:     exItemsGen,
		MT:          MTPolicy{TagMaxBytes: 12, ClearTagByte: ' '},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MWWriteKind: KindMemory,
	})
	if err != nil {
		t.Fatalf("NewDialect with FT-710's data: %v", err)
	}
	// Compare behaviour, not reflect.DeepEqual: the literal and a
	// constructed dialect may legitimately differ in nil-vs-empty derived
	// maps while behaving identically.
	assertDialectsBehaveIdentically(t, FT710, got)
}
```

Write `assertDialectsBehaveIdentically` to compare `CATID()`, every mode's `ValidMode`/`ModeName` over all 256 byte values, `classifySlot` over a slot corpus, `KnownEXAddress` over both inventories, and `exP4MaxBytes()`.

- [ ] **Step 5: Input-independence test.** Mutate the caller's map and slice after construction; assert the dialect is unchanged, **including derived EX membership and width**.

- [ ] **Step 6: Run the full package.** Confirm green and that `core/cat/testdata/` is untouched (`git status`).

- [ ] **Step 7: Commit.** `git commit -m "m9c0: task 62 — DialectConfig, NewDialect, MustNewDialect, rules V1-V11"`

**Depends on:** Task 61.

---

### Task 63: MT tag width and clear-tag byte onto the receiver

**Files:** `core/cat/mt.go`, `core/cat/dialect.go`, `core/cat/mt_test.go`, `core/cat/seconddialect_test.go`.

- [ ] **Step 1: Write the failing peer test first.**

```go
// TestPeerDialect_MTTagPolicyIsItsOwn proves the MT tag bound and the
// clear-tag encoding come from the RECEIVER. A peer whose tag is 6 bytes
// wide and whose clear byte is '-' must accept its own 6-byte tag, reject
// 7, and clear with six '-' — none of which the FT-710's 12-and-space
// policy would produce.
func TestPeerDialect_MTTagPolicyIsItsOwn(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Run it, confirm it fails** (the peer will be bound by 12 and pad with spaces).

- [ ] **Step 3: Move the data.** Delete `mtTagMaxBytes` and `mtClearTag` from `mt.go`. Add to `Dialect`: `mt MTPolicy`. Turn `validMTTag` into `func (d Dialect) validMTTag(tag string) bool` — **it is reached by the gate through `validMTCommand`, which is why it must take the receiver**. Replace `mtAnswerMaxLen` with a method deriving from `d.mt.TagMaxBytes`. Build the clear tag by repeating `d.mt.ClearTagByte`.

- [ ] **Step 4: Run.** Peer test passes; FT-710's `mt_test.go` unchanged and green.

- [ ] **Step 5: Verify FT-710 is untouched.** `go test ./core/cat/ -run 'MT|Corpus'` and confirm `frame-corpus.golden` still matches without regeneration.

- [ ] **Step 6: Commit.** `git commit -m "m9c0: task 63 — MT tag policy onto the receiver"`

**Depends on:** Task 62.

---

### Task 64: Clarifier step and range onto the receiver

**Files:** `core/cat/memdata.go`, `core/cat/dialect.go`, tests.

Same shape as Task 63. Delete `clarMaxAbsHz`/`clarStepHz`; add `clar ClarifierPolicy` to `Dialect`; `validClarHz` becomes a method. **It is reached by the gate through `validateMWFields`.**

- [ ] **Step 1:** Failing peer test — a dialect with `StepHz: 1, MaxAbsHz: 9999` must accept a clarifier of 7 Hz that the FT-710 rejects, and the FT-710 must still reject it.
- [ ] **Step 2:** Confirm it fails.
- [ ] **Step 3:** Move the data; `validClarHz` takes the receiver.
- [ ] **Step 4:** Run; FT-710 unchanged.
- [ ] **Step 5:** Commit — `"m9c0: task 64 — clarifier policy onto the receiver"`

**Depends on:** Task 63.

---

### Task 65: MW write Kind onto the receiver

**Files:** `core/cat/mw.go`, `core/cat/dialect.go`, tests.

`mw.go`'s `if m.Kind != KindMemory` becomes `if m.Kind != d.mwWriteKind`. **Preserve the hardware-evidence comment verbatim** — it records an M5b hardware finding (a PMS write carrying `KindPMS` is rejected by the real radio) and must be rescoped to "the FT-710's policy", not deleted.

- [ ] **Step 1:** Failing peer test — a dialect with `MWWriteKind: KindPMS` accepts a PMS-kind MW write its own gate admits, while FT-710 still rejects that exact frame.
- [ ] **Step 2:** Confirm it fails.
- [ ] **Step 3:** Move the datum; rescope the comment.
- [ ] **Step 4:** Run; FT-710 unchanged; `frame-corpus.golden` unregenerated.
- [ ] **Step 5:** Commit — `"m9c0: task 65 — MW write Kind onto the receiver"`

**Depends on:** Task 64.

---

### Task 66: `Dialect.ModeByName` and the driver write path

**Files:** `core/cat/dialect.go`, `core/driver/ft710/caps.go`, `core/driver/ft710/write.go`, tests.

This is the task that makes V2's uniqueness rule *mean* something. Today `modeByName` is built from the driver's own `modeTable`, independent of the dialect — so validating the config protected nothing (Codex finding 7).

- [ ] **Step 1: Add the method.**

```go
// ModeByName is the inverse of ModeName for this dialect's own table.
// NewDialect rejects duplicate names, so the inverse is well defined for
// any constructed dialect. The zero value has no modes and reports false.
func (d Dialect) ModeByName(name string) (Mode, bool)
```

Build the reverse index once, at construction, beside `exMembers`.

- [ ] **Step 2: Failing test** — `core/driver/ft710`'s write path resolves a mode through the dialect, so a dialect whose `ModeNames` differ resolves differently.
- [ ] **Step 3:** Route `write.go:210` through `s.dialect.ModeByName`. Keep `modeTable` for capability rendering; **pin its equivalence to the dialect** with a test, since it remains a second transcription.
- [ ] **Step 4:** Run; `TestModes_MatchCatModeNames` still green.
- [ ] **Step 5:** Commit — `"m9c0: task 66 — ModeByName; the write path resolves through the dialect"`

**Depends on:** Task 65.

---

### Task 67: Peer fixtures through the public API, and gate integrity

**Files:** `core/cat/seconddialect_test.go`, new `core/cat/dialectgate_test.go`.

This is the milestone's sufficiency proof.

- [ ] **Step 1: Rebuild `testDialect`, `noneWireDialect` and `peerDialect` through `NewDialect`** instead of unexported struct literals. If any cannot be expressed, **stop and report** — that is the API being wrong, and it is the finding this task exists to produce. Any fixture that must stay a literal is named in the file with its reason.

- [ ] **Step 2: The gate-integrity property.** For every dialect in the test set:

```go
// TestEveryDialect_BuildersProduceOnlyGateAdmissibleWireBytes is the test
// that would have caught Codex spec-review finding 2: a config accepted by
// NewDialect must not be able to emit a frame containing a byte outside the
// permitted domain, and every builder output must pass its OWN dialect's
// gate.
func TestEveryDialect_BuildersProduceOnlyGateAdmissibleWireBytes(t *testing.T) { /* ... */ }
```

- [ ] **Step 3: Run.** Confirm each assertion fails for its own reason — temporarily break one builder and check exactly the expected test goes red.

- [ ] **Step 4: Commit.** `"m9c0: task 67 — peer fixtures through the public API; gate-integrity property"`

**Depends on:** Task 66.

---

### Task 68: Byte-identity, gate, docs, ledger

- [ ] **Step 1: Re-capture Task 60's four artefacts** and diff against the manifest. Any difference that is not a recorded noise field is a **defect, not a baseline to update**.
- [ ] **Step 2: Confirm no golden was regenerated:** `git log --oneline -- core/cat/testdata/` must show no new commit.
- [ ] **Step 3: Run the full local gate.**

```sh
gofmt -l .                                   # empty
go vet ./... && go build ./... && go test ./...
go test ./internal/guards/ -v
go test -race ./core/...
cd app && wails generate module && git diff --exit-code frontend/wailsjs
cd frontend && npm run check && npm run test && npm run build
```

- [ ] **Step 4: Re-run the transitive audit** from the spec and record the new count — the five promoted constants must be gone from the gate-affecting class.
- [ ] **Step 5: Write the milestone summary** at `.superpowers/sdd/m9c0-milestone-summary.md`, scoping every claim: what the peer fixtures prove (plumbing) versus what they do not (that any real radio differs).
- [ ] **Step 6: Ledger, commit, then Codex adversarial milestone review.**

**Depends on:** all prior.

## Dependency graph

```
60 -> 61 -> 62 -> 63 -> 64 -> 65 -> 66 -> 67 -> 68
```

Strictly sequential: Tasks 63–66 each add a field to `Dialect` and touch
`dialect.go`, so parallelising them only manufactures conflicts. Task 62
lands all three new fields at once precisely so 63–65 need not re-open the
struct.

## Verification

The spec's Verification section is authoritative. In summary: FT-710
byte-identity against Task 60's manifest; the peer fixtures rebuilt through
the public constructor; gate integrity under a caller-built dialect; both
directions tested for every rule; input independence; and a mutation that
reads FT-710's data instead of the receiver's must fail a test.

## Self-review notes

- **Spec coverage:** V1–V11 → Task 62; the five promoted data → 63, 64, 65;
  `ModeByName` → 66; verification 1 → 68; 2 and 3 → 67; 4 → 62; 5 → 62;
  6 → 63–65; 7 → 68.
- **Type consistency:** `MTPolicy`/`ClarifierPolicy`/`SlotSpace`/
  `DialectConfig` are used with identical field names in every task above.
- **Known risk carried into execution:** Task 67 may discover the API cannot
  express a fixture. That is a real possible outcome, not a failure of the
  plan — the instruction is to stop and report rather than weaken the fixture.
