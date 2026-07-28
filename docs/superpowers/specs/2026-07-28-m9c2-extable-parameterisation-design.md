# M9c-2 — parameterising the EX inventory generator

**Date:** 28/07/2026
**Status:** revision 2.1, approved (plan-review corrections folded)
**Milestone:** M9c-2, the second enabler before M9c-3 (the FTdx10 vertical slice)

> **2.1 correction (28/07/2026, from the adversarial PLAN reviews — Codex
> NEEDS-REVISION 3 HIGH, Fable APPROVE-WITH-FIXES; adjudication at
> `.superpowers/sdd/m9c2-plan-review-adjudication.md`).** Three design
> refinements over revision 2, all in the plan:
>
> 1. **The `TypeQual string` + `ImportPath string` pair is replaced by a
>    zero-invalid `Types TypeRefPolicy` (`TypesLocal`/`TypesImported`) with
>    `ImportPath`/`ImportAlias`.** Revision 2's both-empty pair was
>    simultaneously the FT-710's legitimate value and what an omitted pair
>    produces — this spec's own M9c-1 rule applied to its own new field.
>    Under `TypesImported` the type qualifier IS the emitted import alias,
>    so qualifier and import cannot drift.
> 2. **`Validate` additionally refuses an `OutFile` equal to the profile's
>    own `ManualCSV`/`ObservedCSV`, and the registry refuses one profile's
>    output naming another's input within a package** — the generator
>    writes `OutFile` unconditionally, so this was a validated path to
>    destroying a committed source CSV.
> 3. **`ParseCSV`, `ParseObservedCSV` and `RenderGo` each validate their
>    profile themselves.** Nothing forces a caller through the registry.
>
> Two claims of revision 2 are narrowed, deliberately: `OutFile`/`VarName`
> uniqueness is enforced **per package**, not globally (the ftdx10 profile
> will legitimately reuse the filename `exinventory_gen.go` in its own
> directory); and duplicate registry lookup names are delivered by map
> construction (duplicate literal keys are a compile error), not by a
> check.

> **Revision 2 (28/07/2026).** Revision 1 was reviewed adversarially by
> **Codex** (NEEDS-REVISION, 11 findings, 6 HIGH) and by **Fable**
> (APPROVE-WITH-FIXES, 9 findings, 2 HIGH). Both are adjudicated in
> `.superpowers/sdd/m9c2-spec-review-adjudication.md`. Four findings were
> raised independently by both reviewers and are treated as certain.
>
> **Revision 1's central architectural argument was wrong and is
> withdrawn.** It departed from roadmap A1 — which gives a second model its
> own `core/cat/ftdx10` package — on the ground that the FTdx10's different
> `MT` frame shape forced the codec work into `core/cat` anyway. That
> reasoning does not survive contact with the record: the ~50-byte combined
> `MT` frame was documented in `.superpowers/sdd/m9-plan-codex-review.md:7`
> (finding 1, HIGH) **before A1 was approved**, so it established no new
> fact; and it is in any case a non-sequitur, since a subpackage can own its
> model table and call `cat.MustNewDialect` while frame-shape code stays in
> `core/cat`. **A1 is restored**, and with it the package-name and
> type-qualifier parameterisation revision 1 cut from scope.
>
> Revision 1 also claimed the FTX-1 text-width requirement was untraceable.
> **That claim was false** — see "The FTX-1 text widths" below.
>
> **Do not implement revision 1.**

## Why this exists

`internal/extable` transcodes a radio's menu chart into a generated Go
inventory. It was written at M8a for one radio, and every fact about that
radio is a literal inside it. The M9c handoff lists it as the next work
item: it "cannot emit a second model's inventory".

## The claim, stated precisely

### What is genuinely blocked

Four hardcode classes, not one.

1. **Identity.** `RenderGo` writes `package cat`, `var exItemsGen`, an
   unqualified `EXItem` type and an FT-710/M8c provenance comment
   (`internal/extable/extable.go:253-263`). A second model in its own
   package needs all four to move, plus an import of `core/cat`.

2. **Bounds.** `parseRecord` requires text rows to carry exactly `Digits
   == 12` and non-text rows `1..4` (`extable.go:138-144`);
   `maxObservedWidth` is a package constant `12` (`extable.go:154`). These
   are facts about the FT-710's Table 2, not about EX.

3. **Observation coverage.** `RenderGo` requires the observation map to
   cover the inventory exactly, in both directions (`extable.go:236-237`).
   No FTdx10 exists to observe: Stuart owns none, and the M8c
   characterisation was of one UK FT-710. **A second model therefore cannot
   be rendered at all**, even after every naming fix in class 1.

4. **Completeness.** Nothing in `extable` knows how many rows an inventory
   should have. `RenderGo` compares the two supplied sets against *each
   other* only, so deleting the same address from both CSVs — or emptying
   both — renders happily. The FT-710 is protected today only because
   separate `core/cat` tests pin 296 rows; a second model would inherit no
   such gate. Codex finding 5; revision 1's claim that "no state leaves a
   half-complete artefact passing" was **overstated**.

### What is not blocked

**The EX-inventory consumer API needs no M9c-2 change.**
`cat.DialectConfig.EXItems []EXItem` is exported
(`core/cat/dialectconfig.go:108,125`), `cat.NewDialect` is exported
(`:196`), and `core/cat/dialectexternal_test.go:42` already constructs a
dialect from outside the package using `[]cat.EXItem` — including a
16-digit text item. M9c-0 did this work, specifically to enable A1.

That is a narrower claim than revision 1's "the consumer side is already
finished" (Codex finding 11). The FTdx10 consumer side as a whole is **not**
finished: `Dialect` still carries data rather than frame shapes, and its own
comments defer the different `MT` implementation to M9c-3.

## Placement: roadmap A1, unchanged

A second model's inventory and dialect live in **`core/cat/ftdx10`**,
constructed through the exported `cat.MustNewDialect`. Frame-shape code
stays in `core/cat` where the builders are; that was always true and is not
an argument about where the table lives.

Two properties make A1 the right call, both surfaced by review:

- **An out-of-package model cannot bypass validation.** Every `Dialect`
  field is unexported, so a subpackage *must* go through
  `NewDialect`/`MustNewDialect` and its eleven validation rules. An
  in-package sibling could be a second unvalidated literal exactly as
  `cat.FT710` is today (`core/cat/dialect.go:126-151`, and `:155-157`
  documents the bypass) — reopening the hole M9c-0 was built to close.
- **It is what the accepted roadmap says**, and roadmap A1 is backed by an
  accepted HIGH finding (M9 plan review finding 8) that explicitly requires
  "package/type qualification" from this generator.

Consequence: the generator must emit into an arbitrary package with an
optionally-qualified type and an optional import. That is scope revision 1
wrongly cut.

## Scope

**In:** a per-model `Profile` carrying every fact that differs between
radios; `ParseCSV`, `ParseObservedCSV` and `RenderGo` all taking it; package
/ type-qualifier / import emission; an independent observation ceiling; a
declared observation regime so a manual-only model can render; a declared
expected row count; address-component range validation; a registry with a
stable enumeration API; a synthetic second-model fixture that disagrees in
every parameterised dimension.

**Out and recorded:** the FTdx10 Table 2 transcription and the FTdx10
dialect itself (M9c-3); folding `observe`'s text width into the profile (see
below — deliberately **not** done); registration preconditions 8–11 and the
deferred zero-value class, all of which remain open exactly as the handoff
records them.

## Design

### The `Profile`

One exported value per model in a registry inside `internal/extable`. It is
the single source of every differing fact, and the generator and the
staleness tests all read the same value, so they cannot drift.

```go
type ObservationPolicy int

const (
    // ObservationsRequired: the observation CSV must cover the inventory
    // exactly, in both directions. Today's rule, unweakened.
    ObservationsRequired ObservationPolicy = iota + 1
    // ObservationsAbsent: no hardware exists for this model, so the
    // observation map must be EMPTY — not partial.
    ObservationsAbsent
)

type Profile struct {
    Model     string // "FT-710", used in error text
    Package   string // "cat"  — the generated file's package clause
    TypeQual  string // ""     — "cat." when emitting outside core/cat
    ImportPath string // ""    — set iff TypeQual != ""
    VarName   string // "exItemsGen"
    OutFile   string // "exinventory_gen.go"
    ManualCSV string // "table2.csv"
    ObservedCSV string // "" iff Observations == ObservationsAbsent

    MinDigits        int // 1
    MaxDigits        int // 4
    TextWidth        int // 12 — MANUAL-SCHEMA fact only; see below
    MaxObservedWidth int // 12 — INDEPENDENT hardware-evidence bound
    ExpectedRows     int // 296

    Observations ObservationPolicy
    DocLines     []string // the generated file's descriptive prose
}
```

Paths are relative to the profile's own package directory, which is the
working directory for both `go:generate` and that package's staleness test.

### `TextWidth` and `MaxObservedWidth` are separate facts

Revision 1 derived the observation ceiling as `max(MaxDigits, TextWidth)`.
**Both reviewers rejected this, and they are right.** `Digits` and
`TextWidth` are manual-schema facts; an observed width is independent
hardware evidence, and this repository already holds proof that the two
categories disagree — `core/cat/table2-corrections.csv` records TONE FREQ
declaring two digits and answering three. The FT-710 escapes only because
its 12-byte text items dominate its ceiling; nothing generalises that.

So `MaxObservedWidth` is its own positive, zero-invalid field. For the
FT-710 it is 12, which is what the constant it replaces is today.

**`TextWidth` governs manual-CSV validation only** — the exact `Digits` a
text row must carry. It is not an evidence bound and must never be used as
one.

### `observe`'s text width is deliberately NOT folded in

Revision 1 proposed folding `internal/extable/observe/main.go:67`'s private
`const textWidth = 12` into the profile, calling it "the same number".
**Withdrawn** (Codex finding 3, Fable finding 9).

It is the same *number* and a different *fact*. `observe` deliberately lets
numeric observations disagree with the manual and records their actual
length; only text answers are held to exact equality
(`observe/main.go:90-91`). Routing that through `Profile.TextWidth` would
merge a manual-schema bound with an evidence-collection policy and make the
tool structurally incapable of ever recording a text-width correction — on a
project whose one hardware deviation to date *was* a width correction.

`observe` also only ever runs for a model that has hardware, and the FT-710
is the only one. Its constant stays where it is, with a comment recording
that the coincidence with `TextWidth` is a coincidence. This is the
`MTPolicy.PadByte` lesson applied in the opposite direction: revision 1
proposed the conflation, and the fix is to decline it rather than to name it.

### Zero values and validation

`ObservationPolicy`'s zero value is deliberately not a valid regime
(`iota + 1`), so an omitted policy is refused rather than defaulting. This
follows the M9c-1 ruling on `ShiftDirection` and `ToneSemantics`.

Registry construction refuses, per profile:

- blank `Model`, `Package`, `VarName`, `OutFile` or `ManualCSV`;
- a `VarName` or `Package` that is not a valid non-keyword Go identifier;
- `TypeQual` and `ImportPath` set inconsistently (either both or neither);
- non-positive `MinDigits`, `MaxDigits`, `TextWidth`, `MaxObservedWidth` or
  `ExpectedRows`;
- `MinDigits > MaxDigits`;
- `MaxDigits` or `TextWidth` above `core/cat`'s `maxEXDigits` (247,
  `core/cat/dialectvalidate.go:18`), so an inventory that `NewDialect` would
  later refuse at V8 is refused here instead of two packages downstream;
- an unknown (not merely zero) `ObservationPolicy`;
- `ObservedCSV` set under `ObservationsAbsent`, or unset under
  `ObservationsRequired`;
- empty `DocLines`, or any entry that is blank or contains a newline;
- a `ManualCSV`, `ObservedCSV` or `OutFile` that is not a clean relative
  path.

And across profiles, because the registry is a shared namespace:

- duplicate lookup names, `OutFile`s, or `VarName`-within-`Package` pairs.

Revision 1 claimed "the same rule applies to every other field" and then
omitted `Model`, `OutFile` and `ManualCSV`, and had no cross-profile rule at
all (Codex finding 9, Fable finding 4).

### Registry API

`Lookup(name) (Profile, bool)` for the generator, and
`RegisteredProfiles() []NamedProfile` — sorted, defensive copies, with
`DocLines` copied rather than shared — for enumeration. Registry
construction refuses an empty registry, so a table-driven consumer cannot
pass vacuously (Codex finding 8).

### The two observation regimes, plus completeness

`ObservationsRequired` keeps today's rule verbatim: set-equal in both
directions. `ObservationsAbsent` requires the map to be **empty**, not
partial; a non-empty map is an error naming the profile and the count, never
a row's contents.

Neither regime detects a *jointly* truncated pair of sources, so
`ExpectedRows` is enforced after parsing and before rendering, under both
regimes. For the FT-710 it is 296 — verified as the row count of
`table2.csv`, of `table2-observed.csv`, and of the committed generated file.

The rejected alternative was to make observations simply optional, which
would let a row dropped from `table2-observed.csv` render as "no
observation" instead of failing.

### Address-component range

`ParseCSV` gains `0..99` validation on P1, P2 and P3. It has none today
(`extable.go:98-106` is bare `strconv.Atoi`), and is protected only
accidentally: the observation CSV's exactly-two-digits rule
(`extable.go:192-196`) rejects the corresponding row. Under
`ObservationsAbsent` that join disappears and a component of 100 would
render into an `EXAddress` whose `Wire()` is seven digits (Codex finding
10). Source validation must not depend on a downstream consumer.

## Components changed

| File | Change |
|---|---|
| `internal/extable/profile.go` | New. `Profile`, `ObservationPolicy`, `NamedProfile`, the registry, `Lookup`, `RegisteredProfiles`, and all validation above. |
| `internal/extable/extable.go` | `ParseCSV(p, data)`, `ParseObservedCSV(p, data)`, `RenderGo(p, rows, observed)`. Bounds, identity, package/type/import emission, `ExpectedRows` and the address range all come from `p`. `maxObservedWidth` is deleted. |
| `internal/extable/gen/main.go` | `-profile ft710` replaces `-csv`/`-observed`/`-out`; those paths are profile data now. |
| `core/cat/exinventory.go:5` | The `go:generate` directive shortens to match. |
| `internal/extable/observe/main.go` | Its two `ParseCSV` call sites take the FT-710 profile. `textWidth` stays. |
| `internal/extable/observe/main_test.go:40,155` | Two further `ParseCSV` call sites (Codex finding 7 — absent from revision 1's list). |
| `core/cat/exinventory_stale_test.go` | Profile argument threaded through its three `extable` calls. **Nothing else.** See below. |

**The call-site list above is a hypothesis, not an inventory.** Per the
standing M9c-1 lesson, the verification gate is a repository-wide search for
all three function names, not this table.

### The staleness test must not disturb the evidence-literal guard

`core/cat/testdata/evidence-literals.golden:541-554` pins **all fourteen**
string literals of `exinventory_stale_test.go` by file *and ordinal* —
`"table2.csv"`, `"exinventory_gen.go"`, the full stale message and the rest —
and `TestEvidenceLiterals_OrderedRecordsSurvive` fails if any moves or
disappears. The handoff forbids regenerating that golden (Codex finding 6).

Revision 1 proposed rewriting this test to be table-driven over registered
profiles, which would have deleted or reordered most of those fourteen
literals. **That is withdrawn**, and A1 makes it unnecessary: the FTdx10's
generated file lives in `core/cat/ftdx10`, so *its* staleness test lives in
that package and is not walked by `core/cat`'s evidence collector at all.

The FT-710 test therefore keeps its own path literals at their existing
ordinals; the only change is threading a `Profile` argument, which
introduces an identifier rather than a literal and so leaves the golden
untouched. **Acceptance check:** `go test ./core/cat -run
TestEvidenceLiterals` passes without the golden being regenerated.

## Testing and the acceptance bar

### Byte identity, and an honest word about the header

**`core/cat/exinventory_gen.go` must come out byte-identical.** Regenerate
via `go generate ./core/cat`; `git diff` must be empty.

Revision 1 said the composed generated-by line "must keep matching Go's
`^// Code generated .* DO NOT EDIT\.$` convention". **That was false**, and
both reviewers caught it. The current marker is physically split across two
lines (`exinventory_gen.go:3-4`), `go/format` does not join comment lines,
and `grep -c '^// Code generated .* DO NOT EDIT\.$'` on that file returns
**0**. The file has never satisfied the convention.

**Ruling: byte identity wins.** The renderer reproduces the existing wrapped
two-line marker for a two-source profile, and the non-conformance is
recorded as pre-existing and deliberately not fixed here — fixing it would
change the header, re-mint the baseline and break the two literal markers
`extable_test.go:193-195` pins, all for a cosmetic gain in a milestone whose
entire acceptance bar is that nothing changed. A one-source profile
(`ObservationsAbsent`) names one CSV, so the marker is composed from the
profile's paths rather than fixed — it simply follows the same wrapped
style.

### The disagreeing fixture

A profile used only in tests, differing in *every* parameterised dimension:
different `Package`, `TypeQual`/`ImportPath` set, different `VarName`, digit
bounds `2..6`, `TextWidth` 8, `MaxObservedWidth` 9, `ExpectedRows` a
different count, and different `DocLines`.

It exists in **two variants differing only in `Observations`** — one
`ObservationsRequired`, one `ObservationsAbsent`. A single
`ObservationsAbsent` fixture would never reach the observation ceiling at
all.

Neither variant is registered, so no staleness consumer goes looking for a
generated file that does not exist.

### The tests that bite

A fixture that merely differs proves little. Each of these fails if the
corresponding bound is still read from a constant:

- `Digits == 12` with `Text == true` — legal for the FT-710, illegal under
  the fixture's `TextWidth` of 8 — **rejected** under the fixture.
- `Digits == 6` — illegal under the FT-710's `1..4`, legal under `2..6` —
  **accepted** under the fixture.
- `Digits == 1` — legal for the FT-710, below the fixture's minimum —
  **rejected** under the fixture.
- An observation of width 10 — legal under the FT-710's ceiling of 12,
  above the fixture's `MaxObservedWidth` of 9 — **rejected** under the
  required-observations variant.
- An observation of width **9** — **accepted** under that same variant.
  This is the test that kills revision 1's derivation: the fixture's
  `MaxDigits` is 6 and its `TextWidth` is 8, so a ceiling still computed as
  `max(MaxDigits, TextWidth)` would be 8 and would wrongly reject 9. Only an
  independently-read `MaxObservedWidth` accepts it. The rejection test above
  passes under *either* implementation and so proves nothing on its own —
  the pair is the point.
- A row count one short of `ExpectedRows`, with observations consistently
  truncated to match — **rejected** under both variants. This is the
  jointly-truncated case revision 1 missed.
- Both sources comment-only — **rejected** under both variants.
- P1/P2/P3 of 99 **accepted**; of 100 or negative **rejected**, under
  `ObservationsAbsent` where no observation-side gate exists.
- Emission: the fixture's rendered output must contain its own `package`
  clause, its `import` of `ImportPath`, and `TypeQual`-qualified `EXItem`
  and `EXAddress`; the FT-710's must contain none of those.

### Everything else

**Profile validation tests** — one per refusal listed above, including the
omitted policy, an out-of-range policy, the cross-profile duplicates, the
`maxEXDigits` ceiling, and `RegisteredProfiles` returning copies a caller
cannot mutate into the registry.

**Existing tests** (`TestParseCSV_Strictness`, `TestParseCSV_FieldsParsed`,
`TestRenderGo_Deterministic`, `TestParseObservedCSV_*`,
`TestRenderGo_EmitsObservedFields`, and
`TestRenderGo_RequiresExactObservationCoverage`) take the FT-710 profile and
otherwise keep their current assertions.

**Gate:** `gofmt -l .`, `go build ./...`, `go vet ./...`, the full test
suite, the guards, and `git diff --exit-code -- core/cat/exinventory_gen.go`
after `go generate ./core/cat`.

## The FTX-1 text widths — revision 1's claim was false

Revision 1 said the handoff's "FTX-1 needs 6-16" had no traceable source.
**It does:** `.superpowers/sdd/m9-plan-codex-review.md:21`, finding 8, a
HIGH — *"The FTX-1 manual includes text widths of 6, 8, 9, 10, 12, and
16."* Revision 1's grep searched for the range "6-16" and the source lists
six discrete widths, so it missed it. (Codex's review repeated the error,
probably because `.superpowers/sdd/` is gitignored; Fable found it, and it
was verified directly against the file.)

The correction matters beyond provenance. **Six distinct text widths within
one model cannot be expressed by `TextWidth int` under any value.** Revision
1's remedy — "a one-field change when there is evidence" — was therefore also
wrong.

Deferring the FTX-1 remains right: it is not M9c-3, and its manual is not
acquired. But it is deferred on the true ground, recorded here for whoever
writes that profile: **the text rule's shape will have to change** — to a set
of permitted widths, a range, or a per-row width — and `TextWidth int` is
the thing that will need replacing.

Same source, same finding, useful for M9c-3: roughly **197 FTdx10** and
**193 FTdx101** EX entries, against the roadmap's claimed 150–180.

## Deliberately not built, and recorded

- **Folding `observe`'s `textWidth`** — see above; declining the conflation
  is the fix.
- **A model with no text rows.** Such a profile must still supply a positive
  `TextWidth`, which would be invented. Not reachable in M9c-2 or M9c-3 (the
  FTdx10 has text items); recorded so whoever meets it decides deliberately
  rather than inventing a number. Fable finding 7.
- **Registration preconditions 8–11** — hardwired baud, hardwired
  `Engine.Init` frame, `app/` consumer drift, concretely-typed fake driver
  table. All fail closed; all remain open; none touched here.
- **The ceiling's residual honesty.** `MaxObservedWidth` refuses an
  observation wider than the profile declares. If that refusal ever fires
  against real hardware, the response is a recorded profile-level decision
  with evidence — never a silent widening. Fable finding 5.

## M9c-3 acceptance items this milestone creates

- The FTdx10 dialect must be added to `allTestDialects()`
  (`core/cat/seconddialect_test.go:45-52`), which is a hand-curated list
  whose comment wrongly implies a new dialect cannot skip the generic tests.
  Both reviewers raised this independently.
- The FTdx10's own staleness test lives in `core/cat/ftdx10`, driven from
  `RegisteredProfiles()`.
- The FTdx10 profile must populate every `spec.Capabilities` field
  explicitly, per the handoff's standing instruction on the open zero-value
  class.

## Decisions recorded

- **Placement** (28/07/2026): **roadmap A1 retained** — `core/cat/ftdx10`.
  Revision 1's departure is withdrawn as unsound; see the revision banner.
- **Proof strategy** (Stuart, 28/07/2026): prove the parameterisation with a
  synthetic disagreeing fixture in M9c-2, leaving the real FTdx10 Table 2
  transcription to M9c-3.
- **Header conformance** (28/07/2026): byte identity wins; the marker's
  non-conformance is pre-existing and stays.
