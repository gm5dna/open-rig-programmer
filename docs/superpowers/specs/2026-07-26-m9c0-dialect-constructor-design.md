# M9c-0 — the exported `Dialect` constructor

**Date:** 26/07/2026
**Status:** design, awaiting approval
**Milestone:** M9c-0, an enabler executed before M9c (FTdx10 vertical slice)

## Why this exists

M9b delivered a dialect-parameterised `core/cat`. The M9c roadmap (A1) says
new-model packages `core/cat/ftdx10` and `core/cat/ftdx101` each hold their
tables plus `func Dialect() cat.Dialect`.

**They cannot.** Every field of `cat.Dialect` is unexported and `slotSpace`
is an unexported type, so no package outside `core/cat` can construct one.
M9c stalls at its first task.

This was recorded in the M9b ledger as deferred hardening — "an exported
Dialect constructor (which should also close the pmsCap and exP4Max
silent-clamp class)". That framing understates it: it is a structural
prerequisite, not a nice-to-have.

**Everything else a second dialect needs is already exported.** Verified:
`EXAddress{P1, P2, P3 uint8}` is a plain literal-constructible struct,
`EXItem` has all fields exported, `Mode` is an exported byte type. There is
no chicken-and-egg with `Dialect.NewEXAddress` — an external package builds
`EXAddress` values as literals and never needs the method. The unexported
`Dialect` fields are the single blocker.

## Scope

**In:** an exported constructor with validation; MT tag width promoted to
dialect data; FT-710 behaviour unchanged.

**Out:** the FTdx10 dialect itself (M9c); per-command frame-shape variants
(M9c, approved departure §2.1); `Slot` predicates and `Mode.String` still
FT-710-scoped; cross-dialect negative property tests (M9c).

## What the audit found

An AST sweep of every package-level `const`/`var` referenced inside a
`Dialect` method returned 35 distinct identifiers, in three classes:

| Class | Count | Examples | Disposition |
|---|---|---|---|
| Internal enums, radio-independent | 8 | `slotKind*`, `KindMemory`, `ModeUnset` | Not defects. Leave. |
| Frame-shape and field-width constants | 24 | `mem*Offset`, `memoryFrameLen`, `mrReadLen`, `memFreqMax` | Approved §2.1 deferral. M9c's frame-variant work. |
| MT tag width | 3 | `mtAnswerMaxLen`, `mtAnswerMinLen`, `mtClearTag` | Promoted to dialect data here. |

Two notes on method, because both nearly produced a wrong answer:

- The first sweep reported 36 including `modeNames`, which would have meant
  `Configured`/`ModeName`/`ValidMode` read a package global. **False
  positive.** `ast.Inspect` visits `SelectorExpr.Sel` as a bare `Ident`, so
  the *field* `d.modeNames` reads as the package var of the same name. This
  is the identical trap `internal/guards/engine_construction_test.go`
  documents at its manual-walk comment. The corrected sweep skips `Sel`.
- `memFreqMax` looked like band-limit data, which would have made it
  radio-varying and in scope. It is `999_999_999`, "the largest FreqHz
  value that fits the 9-digit P2 field" — a field width, so Class B.

**Honest limit on what promoting MT achieves.** `mtTagMaxBytes` is not
categorically different from the other 24 field widths. It is singled out
because it bounds the outbound **write gate** (`validMTCommand`), not
because it is uniquely variable — the manual research has FTdx10/101 also
using 12-character tags, so it may not vary across the classic family at
all. This closes the write-gate-adjacent case. It does **not** close the
frame-shape class, and nothing here should be read as claiming it does.

## The API

```go
// SlotSpace is the exported description of a family's slot numbering.
type SlotSpace struct {
    MemoryLo, MemoryHi int    // inclusive; MemoryHi 0 = no memory range
    SixtyLo, SixtyHi   int    // inclusive; both 0 = absent
    PMSPairs           int    // 0..9; 0 = absent
    EmergencyWire      string // "" = absent
    NoneWire           string // "" = absent
}

type DialectConfig struct {
    CATID         string
    ModeNames     map[Mode]string
    Slots         SlotSpace
    EXItems       []EXItem
    MTTagMaxBytes int
}

func NewDialect(cfg DialectConfig) (Dialect, error)
```

A flat config struct, not functional options and not a builder. The type's
own doc says it "carries DATA, not frame shapes"; a flat config matches
that, reviews as a table, and — the deciding reason — makes "is every
required field set?" a question the constructor can answer exhaustively.
Options leave a dialect half-built with no single place to check it.

`NewDialect` copies every slice and map it is given, and derives
`exMembers`, `exByTriple` and `exP4Max` itself. A caller mutating its input
afterwards must not be able to change a constructed dialect.

## Validation

Each rule exists because of a concrete failure, not for tidiness.

| # | Rule | Failure it prevents |
|---|---|---|
| V1 | `CATID` exactly 4 bytes | The ID answer is `"ID"+4+";"` = 7 bytes. A dialect whose CATID is another width could never match a real answer, so connect-time identification silently never succeeds. |
| V2 | `ModeNames` non-empty; every name non-empty **and unique** | `core/driver/ft710/caps.go`'s `modeByName` is a display-name → `Mode` reverse lookup on the **write path** (`write.go:210`). Duplicate names make it ambiguous: one Mode silently wins and a channel is written with the wrong mode nibble. |
| V3 | `PMSPairs` in 0..9 — **reject, do not clamp** | The wire pair digit is one ASCII byte. A Codex review measured an uncapped `pmsPairs` building `"P12L"` forms that the *same dialect's* `ParseSlot` then rejected. `pmsCap()` currently clamps silently, hiding a transcription error. |
| V4 | `EmergencyWire`/`NoneWire` either `""` or exactly 3 bytes | `classifySlot` returns `slotKindInvalid` for any length ≠ 3, so a 2- or 4-byte form is dead configuration that silently never matches. |
| V5 | Memory range coherent: if `MemoryHi > 0` then `1 ≤ MemoryLo ≤ MemoryHi ≤ 999` | An inverted or out-of-range pair yields a dialect that accepts nothing, or accepts wire forms that cannot be expressed in 3 digits. |
| V6 | 60m range: both 0, or `1 ≤ SixtyLo ≤ SixtyHi ≤ 999`; must not overlap the memory range | An overlap makes `classifySlot`'s ordered switch decide by accident: memory wins, and every colliding slot is silently misclassified. |
| V7 | **Shadowing:** an all-digit `NoneWire`/`EmergencyWire` must not fall inside the memory or 60m range; `NoneWire != EmergencyWire` | `classifySlot` tests `noneWire` **first**. A dialect with `MemoryLo: 0` and `NoneWire: "000"` silently loses slot 000 to the none-form. This is the class a second dialect is most likely to hit and nothing currently catches it. |
| V8 | `EXItems`: no duplicate `Addr`; every `Digits ≥ 1` | A duplicate address makes `exByTriple` order-dependent. `Digits ≥ 1` also makes `exP4MaxBytes`' floor unreachable for a non-empty inventory, so the floor covers only the genuinely inventory-less case it was written for. |
| V9 | `MTTagMaxBytes` in 1..64 | It bounds the outbound write gate. Unbounded, a transcription typo authorises a pathological gate bound. 64 is deliberately generous — no classic-family radio documents beyond 12 — and is a sanity ceiling, not a protocol fact. |

Errors are returned, never panicked, and name the offending field and value.

## FT-710 stays a literal

`cat.FT710` is **not** rebuilt through `NewDialect`. It remains the struct
literal it is today, and a test asserts that `NewDialect` fed FT-710's own
data produces an identical `Dialect`.

This gets both properties that matter — the constructor is proven
expressive enough for a real radio, and FT-710's data is proven to pass its
own validation — without putting a panic in package `init`. If the literal
ever drifts into a state the constructor would reject, the test fails.

## Verification

1. **FT-710 byte-identity.** The four M9b baseline artefacts that must match
   with no normalisation (`probe-fake.txt`, `settings.txt`, `export.csv`,
   `help.txt`) re-captured and diffed; the frame, parser, allowlist and
   evidence-literal corpora re-run unchanged. `export.csv` is load-bearing
   because it renders every channel's mode string.
2. **The constructor is sufficient.** `seconddialect_test.go`'s peer
   fixtures are rebuilt through `NewDialect` instead of unexported literals.
   If the exported API cannot express them, that is the API being wrong, and
   this is where it shows.
3. **Every validation rule has a rejecting test and an accepting test.**
   Task 57's lesson: a file of "rejects X" assertions all pass if the
   rejection helper is stubbed to `return false`. Each rule needs both
   directions.
4. **MT tag width is receiver-derived.** A peer dialect with a tag width
   that is *not* 12 must round-trip through its own `BuildMTSet`/
   `ParseMTAnswer` and be accepted by its own gate — and FT-710's effective
   limit must be unchanged at 12.
5. **Mutation.** The receiver must be load-bearing: a mutation that reads
   FT-710's data instead of the receiver's must fail a test.

## Risks

1. **A silent behaviour change in FT-710's MT path** while moving the tag
   width off a const. Mitigated by the frame corpus (357 records, never
   regenerated) and the byte-identity comparison.
2. **The constructor is expressive enough for FT-710 but not for a real
   second radio** — not fully falsifiable until M9c writes one. The peer
   fixtures reduce it; they do not eliminate it. Stated rather than hidden.
3. **Validation rejecting a legitimate future dialect.** V9's 64-byte
   ceiling and V1's 4-byte CATID are the candidates. Both are recorded here
   as judgements with their reasoning, so a future milestone can revisit
   them on evidence rather than rediscovering the argument.
