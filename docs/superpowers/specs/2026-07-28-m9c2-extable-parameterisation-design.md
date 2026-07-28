# M9c-2 — parameterising the EX inventory generator

**Date:** 28/07/2026
**Status:** revision 1, awaiting approval
**Milestone:** M9c-2, the second enabler before M9c-3 (the FTdx10 vertical slice)

## Why this exists

`internal/extable` transcodes a radio's menu chart into a generated Go
inventory. It was written at M8a for one radio and every fact about that
radio is a literal inside it. The M9c handoff lists it as the next work
item: it "cannot emit a second model's inventory".

That is true, but the handoff's diagnosis is incomplete in one direction
and overstated in another. Both corrections are below, because they change
what this milestone builds.

## The claim, stated precisely

### What is genuinely blocked

Three separate hardcode classes, not one.

1. **Identity.** `RenderGo` writes `package cat`, `var exItemsGen`, an
   unqualified `EXItem` type and an FT-710/M8c provenance comment
   (`internal/extable/extable.go:253-263`).

2. **Bounds.** `parseRecord` requires text rows to carry exactly `Digits
   == 12` and non-text rows `1..4` (`extable.go:138-144`);
   `maxObservedWidth` is a package constant `12` (`extable.go:154`); and
   `internal/extable/observe/main.go:67` holds a **third** copy of the same
   number as a private `const textWidth = 12`. These are facts about the
   FT-710's Table 2, not about EX.

3. **Observation coverage — not previously listed anywhere.** `RenderGo`
   requires the observation map to cover the inventory exactly, in both
   directions, and errors otherwise (`extable.go:236-237`). No FTdx10
   exists to observe: Stuart owns none, and the M8c characterisation was of
   one UK FT-710. **A second model therefore cannot be rendered at all,
   even after every naming fix in class 1.** This is the blocking one.

### What is not blocked

The consumer side is already finished and needs nothing.
`cat.DialectConfig.EXItems []EXItem` is exported
(`core/cat/dialectconfig.go:108,125`), `cat.NewDialect` is exported
(`:196`), and `core/cat/dialectexternal_test.go:42` already constructs a
dialect from outside the package using `[]cat.EXItem`. M9c-0 did this work.

## Placement: a sibling in `core/cat`, not a subpackage

The handoff sketched `core/cat/ftdx10` for the second model, and the M9c
roadmap (A1) says the same. **This spec departs from that**, and the
departure is why hardcode class 1 shrinks to a variable name and a doc
comment rather than a package name and a type qualifier.

The reason is a fact established after those documents were written. The
FTdx10's `MT` is not a variant of the FT-710's — it is a different command
with a different frame shape, a 51-byte combined record where the FT-710
has a short slot+display+tag form (HANDOFF-m9c.md, manual finding 1).
Frame shapes are builder code and builders live in `core/cat`. So the
FTdx10's codec work lands in `core/cat` whatever happens to its EX table,
and putting only the table in a subpackage draws a boundary that no other
part of the design respects.

Two further reasons:

- `core/driver/ftdx10` is already the per-radio boundary, mirroring
  `core/driver/ft710/ft710.go:52`'s `var catDialect = cat.FT710`. A second
  per-radio boundary inside `core/cat` duplicates it.
- It is what M9b's thesis actually claimed — that adding a radio is writing
  a table, not forking the codec. Tables belong where the first table is.

**Consequence, stated plainly so nobody restates it stronger:** this
milestone does **not** make the generator able to emit into an arbitrary
package with a qualified type. It makes it able to emit a second inventory
into `core/cat`. The package-name and type-qualifier parameterisation the
handoff describes is deliberately not built, because after this decision no
consumer wants it.

## Scope

**In:** a per-model `Profile` carrying every fact that differs between
radios; `ParseCSV`/`RenderGo` reading their bounds and identity from it; a
declared observation regime so a manual-only model can render; the third
copy of `textWidth` folded into the profile; a staleness test that is
table-driven over registered profiles; a synthetic second-model fixture
that disagrees in every parameterised dimension.

**Out and recorded:** the FTdx10 Table 2 transcription (M9c-3); package and
type-qualifier parameterisation (no consumer, per the placement decision);
the FTX-1 width range (see below); registration preconditions 8–11 and the
deferred zero-value class, both of which remain open exactly as the handoff
records them.

## Design

### The `Profile`

One exported value per model, in a registry inside `internal/extable`. It
is the single source of every differing fact, and **the generator and the
staleness test both read the same value.**

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
    Model        string   // "FT-710", used only in error text
    VarName      string   // "exItemsGen"
    OutFile      string   // "exinventory_gen.go"
    ManualCSV    string   // "table2.csv"
    ObservedCSV  string   // "" when Observations is ObservationsAbsent
    MinDigits    int      // 1
    MaxDigits    int      // 4
    TextWidth    int      // 12
    Observations ObservationPolicy
    DocLines     []string // the generated file's header prose, verbatim
}
```

`TextWidth` is the exact `Digits` a text row must carry, and the exact wire
width `observe` requires of a text item's raw P4 (`observe/main.go:90`).

**The observation ceiling is derived, not stored.** `maxObservedWidth`
(`extable.go:154`) is documented as "the largest P4 width any EX item can
have", and for the FT-710 that is 12 because its text items are its widest
rows. That coincidence does not generalise: a model with `MaxDigits` 20 and
`TextWidth` 8 has a ceiling of 20. So the ceiling is computed as
`max(p.MaxDigits, p.TextWidth)` rather than becoming a fourth field.

Deriving it is the point. `MTPolicy.PadByte` exists in `core/cat` because
answer-side padding and the empty-tag encoding were two different facts
that happened to coincide for the FT-710, and conflating them destroyed
data on a dialect where they do not. Storing a ceiling that equals
`TextWidth` would recreate that shape; computing it cannot drift from the
bounds it is a function of.

Paths are relative to `core/cat`, because `gen` runs with that as its
working directory under `go:generate` (`core/cat/exinventory.go:5`).

**`DocLines` is the prose only.** The generated file's header has three
parts and they are not all profile data: the SPDX line is fixed; the
`// Code generated by … DO NOT EDIT.` line must keep matching Go's
`^// Code generated .* DO NOT EDIT\.$` convention and is composed from the
profile's CSV names, so a manual-only model names one source rather than
claiming a second that does not exist; only the descriptive paragraph
beneath comes from `DocLines`. Composing the generated-by line rather than
letting `DocLines` carry it prevents a profile from writing a header that
names an observation CSV it has none of.

The zero value of `ObservationPolicy` is deliberately not a valid regime,
so a profile that omits it is refused rather than defaulting to one. This
follows the M9c-1 ruling on `ShiftDirection` and `ToneSemantics`, for the
same reason: an omitted field must not read as a plausible answer. The same
rule applies to every other field — a blank `VarName`, a non-positive
`MinDigits`/`MaxDigits`/`TextWidth`, `MinDigits > MaxDigits`, an
`ObservedCSV` set under `ObservationsAbsent` or unset under
`ObservationsRequired`, and empty `DocLines` are each a refusal at
registry-construction time.

**Why one value rather than flags on `go:generate`.** The generator and the
staleness test must agree on the bounds. If both take flags, they can
drift. A single value they both consult makes drift impossible. This is the
exact defect shape that appeared four times across M9b — the bound
consulted from the receiver, the datum taken from a global — and the cheap
structural fix is to have only one place to read.

### The two observation regimes

`ObservationsRequired` keeps today's rule verbatim: set-equal in both
directions, an error either way. The FT-710's completeness guarantee is not
weakened by this milestone.

`ObservationsAbsent` requires the map to be **empty**. A non-empty map is an
error naming the profile and the count, never a row's contents. Rows then
render `ObservedReadWidth: 0` and `ObservedReadShape: ""`, which
`EXItem`'s existing documentation already defines as "no observation"
(`core/cat/exinventory.go:58-76`); no new sentinel is introduced.

The rejected alternative was to make observations simply optional. That
would let a row silently dropped from `table2-observed.csv` render as "no
observation" instead of failing the generator — trading the FT-710's
guarantee for the FTdx10's convenience. Two strict rules leave no state in
which a half-complete artefact passes.

## Components changed

| File | Change |
|---|---|
| `internal/extable/profile.go` | New. `Profile`, `ObservationPolicy`, the registry, `Lookup(name)`, and the validation above. |
| `internal/extable/extable.go` | `ParseCSV` and `RenderGo` take a `Profile`. Digit bounds, text width, variable name and doc prose come from it. `maxObservedWidth` becomes `max(p.MaxDigits, p.TextWidth)`. |
| `internal/extable/gen/main.go` | `-profile ft710` replaces `-csv`/`-observed`/`-out`; those paths are profile data now. |
| `core/cat/exinventory.go:5` | The `go:generate` directive shortens to match. |
| `internal/extable/observe/main.go` | Its private `textWidth` comes from the FT-710 profile, removing the third copy. |
| `core/cat/exinventory_stale_test.go` | Table-driven over every registered profile. |

The staleness test's change is a forcing function, not tidying: today it
names `table2.csv` and `exinventory_gen.go` as literals
(`exinventory_stale_test.go:22,30,43`), so a future model's generated file
would have no staleness guard until somebody remembered to add one. Driven
from the registry, registering a model gets it one. Only `ft710` is
registered in M9c-2, so behaviour today is unchanged.

The synthetic fixture below is built inline in `internal/extable`'s own
tests and is deliberately **not** registered, so the staleness test never
looks in `core/cat` for a generated file that does not exist.

## Testing and the acceptance bar

**The bar: `core/cat/exinventory_gen.go` must come out byte-identical.**
Regenerate via `go generate ./core/cat`; `git diff` must be empty. This
pins that the FT-710 profile reproduces every literal it replaced —
including the provenance comment, character for character. It is the same
byte-identity discipline used at M9b, M9c-0 and M9c-1, applied to the one
artefact this milestone can disturb.

**The disagreeing fixture.** A profile used only in tests that differs in
*every* parameterised dimension: a different `VarName`, digit bounds
`2..6`, `TextWidth` 8, and different `DocLines`. Rendering it must show
every one of those differences in the output.

It exists in **two variants differing only in `Observations`** — one
`ObservationsRequired`, one `ObservationsAbsent`. A single
`ObservationsAbsent` fixture would never reach the observation-width
ceiling at all, leaving the `maxObservedWidth` change untested; the
required-variant is what exercises it.

**The tests that actually bite.** A fixture that merely differs proves
little; the M9b lesson is that only a fixture built to *disagree* ever
found anything. So, in both directions:

- A row with `Digits == 12` and `Text == true` — legal for the FT-710, and
  illegal under the fixture's `TextWidth` of 8 — must be **rejected** under
  the fixture.
- A row with `Digits == 6` — illegal for the FT-710's `1..4`, legal under
  the fixture's `2..6` — must be **accepted** under the fixture.
- A row with `Digits == 1` — legal for the FT-710, illegal under the
  fixture's minimum of 2 — must be **rejected** under the fixture.
- An observation of width 9 — legal under the FT-710's ceiling of 12,
  illegal under the fixture's derived ceiling of 8 — must be **rejected**
  under the required-observations variant.

Each of these fails if the corresponding bound is still read from a
constant. A test that only asserted the variable name changed would pass
with every bound hardcoded.

**Observation-regime tests.** A non-empty map under `ObservationsAbsent` is
an error; an empty map under `ObservationsRequired` is an error for a
non-empty inventory; the existing `TestRenderGo_RequiresExactObservationCoverage`
continues to hold for the FT-710 profile unchanged.

**Profile validation tests.** One per refusal listed under `Profile` above,
including the omitted `ObservationPolicy`.

Existing `internal/extable` tests (`TestParseCSV_Strictness`,
`TestParseCSV_FieldsParsed`, `TestRenderGo_Deterministic`,
`TestParseObservedCSV_*`, `TestRenderGo_EmitsObservedFields`) are updated
to pass the FT-710 profile and must otherwise keep their current
assertions.

## Deliberately not built

- **The FTX-1's "6-16" text-width range.** The handoff cites it as a
  roadmap A4/F8 requirement. Grepping the repository and the approved M9
  roadmap finds no source for it other than the handoff quoting itself, and
  the FTX-1 CAT manual has not been acquired. `TextWidth` is a profile
  field, so an FTX-1 profile can carry whatever its manual turns out to
  say; no range is invented here on the strength of an untraceable claim.
  If a range rather than a single width is genuinely needed, that is a
  one-field change at the time there is evidence for it.
- **Package-name and type-qualifier emission**, per the placement decision.
- **Registration preconditions 8–11** (hardwired baud, hardwired
  `Engine.Init` frame, `app/` consumer drift, concretely-typed fake driver
  table). All fail closed; all remain open; none is touched here.

## Decisions recorded

- **Placement** (this session): sibling inventory in `core/cat`, departing
  from roadmap A1 and the handoff sketch, on the `MT` frame-shape evidence
  above.
- **Proof strategy** (Stuart, 28/07/2026): prove the parameterisation with a
  synthetic disagreeing fixture in M9c-2, and leave the real FTdx10 Table 2
  transcription to M9c-3 where the handoff placed it. The alternatives —
  folding the ~300-row transcription in now, or shipping the mechanism with
  the mechanism never run against a second model — were both rejected.
