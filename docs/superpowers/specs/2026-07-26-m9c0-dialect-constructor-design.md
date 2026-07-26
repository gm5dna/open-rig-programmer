# M9c-0 — the exported `Dialect` constructor

**Date:** 26/07/2026
**Status:** revision 2.1, awaiting approval

> **2.1 correction (26/07/2026, Codex plan review finding 2).** The API
> block said `MWWriteKind Kind`. **There is no `Kind` type.**
> `MemoryData.Kind` is a plain `byte` and every `Kind*` constant is
> declared `byte = '0'`…`'5'` (`core/cat/memdata.go:76`, `:82`), so the
> field is `MWWriteKind byte` and Task 62 would not have compiled as
> written. No alias is introduced: the constants are already `byte`, and
> inventing `type Kind = byte` here would add a name the rest of the
> package does not use.
**Milestone:** M9c-0, an enabler executed before M9c (FTdx10 vertical slice)

> **Revision 2 (26/07/2026).** Revision 1 was reviewed adversarially by
> Codex and returned **NEEDS-REVISION with 9 findings, 6 blocking**. All
> nine verified against source and **accepted**; the adjudication is at the
> end and the transcript is at `.superpowers/sdd/m9c0-codex-spec-review.md`.
> Revision 1's audit was also **wrong in method** and is redone here.
> **Do not implement revision 1.**

## Why this exists

M9b delivered a dialect-parameterised `core/cat`. The M9c roadmap (A1) says
new-model packages `core/cat/ftdx10` and `core/cat/ftdx101` each hold their
tables plus `func Dialect() cat.Dialect`.

**They cannot.** Every field of `cat.Dialect` is unexported and `slotSpace`
is an unexported type, so no package outside `core/cat` can construct one.
M9c stalls at its first task.

### The claim, stated precisely

Revision 1 said the unexported fields were "the only blocker". That is true
of **construction** and false of **correctness**, and the difference matters:

- **Construction.** Confirmed. `EXAddress{P1,P2,P3 uint8}`, every `EXItem`
  field, and `Mode` are all externally constructible, so once the fields can
  be set, an external package can express a dialect. Codex verified this
  field by field and found no other unexported stored type in the way.
- **Correctness.** False as originally written. Several FT-710 facts are
  hardcoded in shared validators that a second radio would inherit —
  including in the outbound write gate. Those are enumerated below.

## Scope

**In:** the exported constructor with validation; the FT-710 facts that
**affect the outbound write gate** promoted to dialect data; a receiver-side
mode-name reverse lookup; FT-710 behaviour unchanged.

**Out and recorded:** the FTdx10 dialect itself; per-command frame-shape
variants and all pure frame offsets/lengths (approved departure §2.1);
`Slot` predicates and `Mode.String` still FT-710-scoped; cross-dialect
negative property tests.

The dividing line is deliberate. A wrong FT-710 assumption in the **gate**
can authorise bytes that reach a radio. A wrong frame offset merely fails to
parse. This milestone closes the first class and records the second.

## The audit, redone

**Revision 1's audit was wrong in method and its headline number was wrong.**
It walked only identifiers appearing *directly inside* `Dialect` methods.
The rule it was testing — from `Dialect`'s own doc comment — is "every
method here, **and every helper those methods delegate to**". A direct sweep
cannot test that rule.

Redone transitively (walking from each `Dialect` method through the
package-level functions it calls): **44 reachable package-level `const`/`var`,
not 35.** The nine the direct sweep missed:

| Missed identifier | Reached via | Class |
|---|---|---|
| `mtTagMaxBytes` | `BuildMTSet → validMTTag`, **`validMTCommand → validMTTag`** | gate-affecting |
| `clarMaxAbsHz`, `clarStepHz` | `parseMemoryFrame → validClarHz`, **`validateMWFields → validClarHz`** | gate-affecting |
| `KindVFO`, `KindMemory`, `KindMemTune`, `KindQMB`, `KindUnset`, `KindPMS` | `parseMemoryFrame → validKindByte` | enum |
| `maxParseErrorFrameLen` | `→ newParseError` | diagnostics |

`clarMaxAbsHz`/`clarStepHz` are a finding the corrected method produced and
neither revision 1 nor Codex had: Codex reported "no genuinely data-like
item among the stated 24", correctly, because these two were never in the 35.
`9990` is a 4-digit-field consequence, but `clarStepHz = 10` is a radio
characteristic — a rig stepping 1 Hz in the same field would cap at 9999 —
and `validateMWFields` puts it in the write gate.

Two method errors are recorded rather than quietly fixed, because both are
instances of failure modes this project has named:

1. The first sweep reported `modeNames` as a global read by
   `Configured`/`ModeName`/`ValidMode`. **False positive**: `ast.Inspect`
   visits `SelectorExpr.Sel` as a bare `Ident`, so the *field* `d.modeNames`
   reads as the package var of the same name — the identical trap
   `internal/guards/engine_construction_test.go` documents at its
   manual-walk comment.
2. The corrected sweep was then presented as exhaustive when it was
   **lexical and non-transitive**. That is "a plausible mechanism claim
   written without running the mechanism", applied to a measuring
   instrument.

**Standing rule for this repo:** a sweep testing a rule about delegation
must itself be transitive, and its reach must be stated with its result.

### Classification of the 44

| Class | Count | Disposition |
|---|---|---|
| Internal enums and diagnostics | 15 | Not defects. Leave. |
| Frame offsets, lengths, field widths | 24 | §2.1 deferral. M9c. |
| **Gate-affecting dialect policy** | 5 | **Promoted here.** |

The five promoted: `mtTagMaxBytes`, `mtClearTag`, `clarMaxAbsHz`,
`clarStepHz`, and the `KindMemory` **write policy** (the identifier is an
enum; its use as the *sole accepted P7 on every MW write* is dialect policy —
`core/cat/mw.go:98`, hardware-confirmed for the FT-710 only, and reached by
the gate through `validMWCommand → validateMWFields`).

## The API

```go
type SlotSpace struct {
    MemoryLo, MemoryHi int    // absent = exactly (0,0)
    SixtyLo, SixtyHi   int    // absent = exactly (0,0)
    PMSPairs           int    // 0..9; 0 = absent
    EmergencyWire      string // "" = absent
    NoneWire           string // "" = absent
}

// MTPolicy carries the MT short form's dialect-varying dimensions.
// Frame-shape variants (the FTdx10/101 combined record frame) are M9c's;
// this describes the short form only.
type MTPolicy struct {
    TagMaxBytes  int  // FT-710: 12
    ClearTagByte byte // FT-710: ' ' — an empty tag is padded with this
}

// ClarifierPolicy bounds MemoryData.ClarHz.
type ClarifierPolicy struct {
    StepHz   int // FT-710: 10
    MaxAbsHz int // FT-710: 9990
}

type DialectConfig struct {
    CATID       string
    ModeNames   map[Mode]string
    Slots       SlotSpace
    EXItems     []EXItem
    MT          MTPolicy
    Clarifier   ClarifierPolicy
    MWWriteKind byte // FT-710: KindMemory
}

func NewDialect(cfg DialectConfig) (Dialect, error)
func MustNewDialect(cfg DialectConfig) Dialect // panics; see below
```

`MTPolicy` exists because `mtClearTag` is an **encoding policy, not a
width** (Codex finding 6): "an empty tag becomes twelve spaces" bundles a
width with a padding byte, and only the FT-710's is evidenced. Carrying both
makes the assumption explicit instead of deriving one from the other.

New receiver method, so that name-uniqueness has a real consequence:

```go
func (d Dialect) ModeByName(name string) (Mode, bool)
```

`NewDialect` copies every slice and map it is given and derives
`exMembers`, `exByTriple` and `exP4Max` itself. A caller mutating its input
afterwards must not be able to change a constructed dialect.

**`MustNewDialect` resolves roadmap A1** (Codex finding 8). A1 specifies
`func Dialect() cat.Dialect` for model packages, which cannot propagate an
error. Rather than change A1 and thread errors through registration, model
packages use `MustNewDialect` for their compile-time-constant tables, where
a failure is a build-time programming error, never a runtime condition. Its
doc says exactly that and forbids its use on any caller-supplied data.

## Validation

The **permitted wire-byte domain** used below: printable ASCII `0x20`–`0x7E`
excluding `';'`. Every byte a dialect can cause to be emitted must be in it.

| # | Rule | Failure it prevents |
|---|---|---|
| V1 | `CATID` exactly 4 bytes, all in the permitted domain | The ID answer is `"ID"+4+";"`. Another width can never match a real answer, so identification silently never succeeds. |
| V2 | `ModeNames` non-empty; names non-empty and unique; **every `Mode` key in the permitted domain** | Keys: `BuildMWSet` writes the key byte straight into the frame (`mw.go:39`), and the gate validates through the same receiver — so `Mode(0x00): "NUL"` would produce a **gate-approved frame containing a NUL** (Codex finding 2). Names: uniqueness makes display-name → `Mode` invertible, which `ModeByName` now provides. |
| V3 | `PMSPairs` in 0..9 — **reject, do not clamp** | The wire pair digit is one ASCII byte. An uncapped value builds `"P12L"` forms the same dialect's `ParseSlot` rejects; `pmsCap()` clamps silently, hiding a transcription error. |
| V4 | `EmergencyWire`/`NoneWire`: `""` or exactly 3 bytes, **all in the permitted domain** | `classifySlot` matches these by exact string **before** applying any character grammar, so `EmergencyWire: "\x00AB"` would yield a gate-approved, side-effecting `MC\x00AB;` (Codex finding 2). Lengths ≠ 3 are dead configuration that silently never matches. |
| V5 | Memory range: absent = exactly `(0,0)`; else `0 ≤ MemoryLo ≤ MemoryHi ≤ 999` | **`MemoryLo: 0` must be permitted** — `noneWireDialect` uses it deliberately so `"000"` is an ordinary channel, and revision 1's `≥ 1` rule would have rejected the very fixture the sufficiency proof depends on (Codex finding 3, found independently). Requiring exactly `(0,0)` for absence rejects dead configurations like `MemoryLo: 99, MemoryHi: 0` (finding 5). |
| V6 | 60m range: absent = exactly `(0,0)`; else `0 ≤ SixtyLo ≤ SixtyHi ≤ 999`; must not overlap memory | `classifySlot` checks memory before 60m, so an overlap is decided by ordering accident and memory silently wins. |
| V7 | **Shadowing:** no special wire may collide with an active memory or 60m numeric range, **or with any PMS encoding**; `NoneWire != EmergencyWire` | `classifySlot` tests both special wires **first**. `MemoryLo: 0` + `NoneWire: "000"` loses slot 000. `PMSPairs: 1` + `EmergencyWire: "P1L"` makes `PMSSlot(1,false)` return a wire the same dialect classifies as EMG — constructor and parser disagreeing semantically (finding 5). |
| V8 | `EXItems`: unique `Addr`; every component `0..99`; `Digits ≥ 1` and small enough that the answer frame fits the transport frame limit | `EXAddress.Wire` uses `%02d`, **minimum** width, so `P1: 100` renders a 7-byte address whose frame the dialect's own gate rejects and whose parser can never reconstruct it. Unbounded `Digits` overflows `exAnswerMaxLen`'s addition; `> 247` exceeds the 256-byte accumulator (finding 4). |
| V9 | `MT.TagMaxBytes` in 1..64; `MT.ClearTagByte` in the permitted domain | Bounds the outbound write gate. 64 keeps the short frame (71 bytes) inside `DefaultMaxFrame`. **This is a resource bound, not a protocol fact** — no classic-family radio documents beyond 12. |
| V10 | `Clarifier.StepHz ≥ 1`; `MaxAbsHz ≥ 0`; `MaxAbsHz % StepHz == 0`; `MaxAbsHz` fits the 4-digit P3 field | An indivisible pair makes `validClarHz` reject values the dialect's own range implies are legal. |
| V11 | `MWWriteKind` must be a documented P7 value (`validKindByte`) | An unlisted byte would be written into P7 and admitted by the dialect's own gate. |

Errors are returned, never panicked, and name the offending field and value.

## FT-710 stays a literal

`cat.FT710` remains a struct literal; a test asserts `NewDialect` fed
FT-710's data produces an identical `Dialect`.

**What this does and does not buy** (Codex finding 9). It proves the
constructor is expressive enough for a real radio and that FT-710's data
passes its own validation. It is **test-time equivalence, not enforcement**:
the literal bypasses validation in any build where tests are not run. That
is accepted, because the alternative puts a panic in package `init`. The
equivalence config is built from **independent literals**, never by reading
`FT710`'s fields back into a config — an expectation derived from the code
under test moves with it and proves nothing. This is the same discipline
`ft710P4MaxBytes` already follows in `seconddialect_test.go`.

## Verification

1. **FT-710 byte-identity.** The four M9b baselines that must match with no
   normalisation (`probe-fake.txt`, `settings.txt`, `export.csv`,
   `help.txt`) re-captured and diffed; frame, parser, allowlist and
   evidence-literal corpora re-run unchanged. `export.csv` is load-bearing
   because it renders every mode string.
2. **The constructor is sufficient.** `seconddialect_test.go`'s peer
   fixtures are rebuilt through `NewDialect`. If the exported API cannot
   express them, that is the API being wrong, and this is where it shows.
   Any fixture that must stay an internal literal is named with its reason.
3. **Gate integrity under a caller-built dialect.** For every accepted
   config in the test set: every builder output passes that dialect's own
   gate, and contains no byte outside the permitted domain. This is the
   test that would have caught finding 2.
4. **Every rule gets a rejecting AND an accepting test.** Task 57's lesson:
   a file of "rejects X" assertions all pass if the rejection helper is
   stubbed to `return false`.
5. **Input independence.** After construction, mutating the caller's input
   map and slice must not change the dialect — including derived EX
   membership and width (finding 9).
6. **The promoted data is receiver-derived.** A peer dialect differing in
   *each* promoted datum — MT tag width and clear byte, clarifier step and
   range, MW write Kind — must round-trip through its own builders and be
   accepted by its own gate, with FT-710's behaviour unchanged.
7. **Mutation.** A mutation reading FT-710's data instead of the receiver's
   must fail a test.

## Risks

1. **Silent behaviour change in FT-710's MT, clarifier or MW path** while
   moving five facts off consts. Mitigated by the frame corpus (357 records,
   never regenerated) and the byte-identity comparison.
2. **Expressive enough for FT-710 but not a real second radio.** Not fully
   falsifiable until M9c writes one. The peer fixtures reduce this; they do
   not eliminate it.
3. **Validation rejecting a legitimate future dialect.** V9's 64-byte
   ceiling, V1's 4-byte CATID and V8's component range are the candidates.
   Recorded here as judgements with reasoning so a later milestone can
   revisit them on evidence.
4. **A fictional peer proves plumbing, not protocol** (Codex finding 6). A
   peer whose MT tag is not 12 shows the receiver is consulted; it does not
   show any real radio differs. The manual research has FTdx10/101 also at
   12 characters. No claim is made that these five facts *do* vary — only
   that the FT-710's values are FT-710's.

## Codex review — adjudication

All nine findings **accepted**; six were blocking. Where each landed:

| # | Sev | Disposition |
|---|---|---|
| 1 | HIGH | `MWWriteKind` added to the config; the "single blocker" claim split into construction vs correctness. |
| 2 | HIGH | Permitted wire-byte domain added; V2 and V4 now constrain mode keys and special-slot bytes; verification 3 added. |
| 3 | HIGH | V5 permits `MemoryLo: 0`. Found independently before the review returned. |
| 4 | MED | V8 gains component range `0..99` and a `Digits` frame-fit bound. |
| 5 | MED | V7 gains PMS collision; V5/V6 require exactly `(0,0)` for absence. |
| 6 | MED | `MTPolicy` carries width *and* clear-tag byte; the sweep's transitive miss of `validMTTag` is recorded in the audit. |
| 7 | MED | **My stated mechanism was wrong.** `modeByName` is built from the driver's own `modeTable`, not from `Dialect.modeNames`, so validating the config protected nothing. `Dialect.ModeByName` added so the rule has a real consequence, and the write path routes through it. |
| 8 | MED | `MustNewDialect` provided; A1's signature stands. |
| 9 | LOW | Equivalence described as test-time only; config built from independent literals; input-mutation tests added as verification 5. |
