# M9c-1 — the second-model registration gate

**Date:** 27/07/2026
**Status:** revision 1, awaiting approval
**Milestone:** M9c-1, an enabler executed before the M9c FTdx10 vertical slice

## Why this exists

The ledger records a set of **registration preconditions**: things that must
be true before any second model registers. They are scattered across three
sources and no single document holds them all.

`.superpowers/sdd/progress.md:352` (M9a plan, deferred-by-design):

> M9c REGISTRATION PRECONDITIONS (Codex F7): model-aware csvio/chirp.go +
> complete radiotext coverage before any second model registers.

The Codex M9a milestone review adds four more:

> All 21 deferred minors OK-TO-DEFER (m39a/m39b/m40d/m42a flagged as
> second-model registration preconditions — fold into the M9c entry
> checklist).

And the M9 plan defers one further item to this point:

> D9 snapshot-dir rule → new models get `<dir>/<model-slug>/` at M9c, FT-710
> stays at root.

`.superpowers/sdd/HANDOFF-m9c.md` carried only the first of the seven. This
spec is the whole checklist.

### The unifying claim

**Nothing in a model-neutral package may decide anything by knowing it is
the FT-710.** Each of the seven items is a place where that is false today,
and each has the same failure mode if left alone: a second model registers,
the code keeps working, and it is quietly wrong — a tag truncated to the
wrong width, a shift value the radio does not use, an advisory sentence
claiming hardware verification that never happened.

Every change here is **behaviour-preserving for the FT-710**. Byte-identity
of the FT-710's outputs is the acceptance bar, not a nice-to-have.

## Two handoff corrections

Both shrink the work, and both were verified against source before this spec
was written.

1. **`Capabilities.TagLen` already exists** (`core/spec/capabilities.go:21`).
   The handoff says "the tag width needs a capability too, because
   `core/csvio` is neutral and must not import `core/cat`". The premise is
   right and the conclusion is already implemented — M9a put it there. No new
   capability field is needed for tag width.
2. **`spec.BankMemory` already exists** (`core/spec/bank.go:12`), and
   `Bank.Slots` holds canonical wire-form slot strings. Slot mapping has a
   neutral home; no new capability is needed for it either.

## Scope

Seven items, one branch, one milestone review.

| # | Item | Source | Section |
|---|------|--------|---------|
| 1 | Vocabulary semantics in `core/spec` | prerequisite for 2 | §1 |
| 2 | Model-aware `core/csvio/chirp.go` | F7 | §2 |
| 3 | `core/csvio/doc.go` neutral prose | m40d | §2 |
| 4 | Complete `radiotext` coverage | F7, m42a | §3 |
| 5 | `StaticCapabilities` discarded `ok` | m39a | §4 |
| 6 | Driver-table key invariant pinned | m39b | §4 |
| 7 | Snapshot directory per model | D9 | §4 |

**Out of scope**, and deliberately so:

- **The FTdx10's own prose.** Its erase procedure needs the manual, its
  firmware gate is unknown, and with `writeTrialsComplete=false` its
  preservation claims are unverified. §3 lands the forcing function; the
  content is written in the FTdx10 slice, where the facts are.
- **A GUI model picker, and any change to `currentCaps`.** See §5.
- **`internal/extable` generator parameterisation.** A real precondition for
  the FTdx10 slice, but not a *registration* precondition — nothing silently
  misbehaves without it; the generator simply cannot emit a second model.
  It is the handoff's next item after this one.

---

## §1 — `core/spec`: vocabulary semantics

### The problem

`core/csvio/chirp.go` must answer "which of *this* radio's values means
+shift, or encode-only, or CW-upper?". Today it cannot ask:

- `Capabilities.ShiftOptions []string` carries no semantics at all
- `Capabilities.Modes []string` likewise
- `ToneState{Value, RequiresTone}` distinguishes OFF from the rest, but
  **not ENC from ENC-DEC** — the exact distinction CHIRP's `Tone`/`TSQL`
  needs

Task 38 set the standing rule for this: no wire-vocabulary literal in the
neutral layer, which is why `ToneState` replaced a plain CTCSS string list
in the first place. `ShiftOptions` never got the same treatment.

### The change

`ShiftOptions` becomes symmetric with `CTCSSStates` — a shape this codebase
already knows, so every consumer has its own worked example adjacent.

```go
// ShiftDirection is the semantic content of a repeater shift option:
// which way the transmit frequency moves, if at all.
type ShiftDirection int

const (
    ShiftNone ShiftDirection = iota // simplex — no offset
    ShiftUp                         // transmit above receive
    ShiftDown                       // transmit below receive
)

// ShiftOption is one repeater shift value this radio's wire protocol
// expresses, together with the semantic fact generic code needs about it
// rather than having to re-derive it from the wire string by hand.
type ShiftOption struct {
    Value     string         // wire form, e.g. "PLUS"
    Direction ShiftDirection // e.g. ShiftUp
}

type ToneState struct {
    Value        string
    RequiresTone bool
    Encodes      bool // transmits a CTCSS tone
    Decodes      bool // requires a matching received tone to open squelch
}
```

`Capabilities.ShiftOptions` becomes `[]ShiftOption`.
`StandardShiftOptions()` returns the same three values in the same order,
now carrying direction:

```go
{Value: "SIMPLEX", Direction: ShiftNone}
{Value: "PLUS",    Direction: ShiftUp}
{Value: "MINUS",   Direction: ShiftDown}
```

`standardCTCSSStates` (`core/spec/vocab.go:48`) keeps its legacy literal
order and gains the two fields:

```go
{Value: "OFF",     RequiresTone: false, Encodes: false, Decodes: false}
{Value: "ENC-DEC", RequiresTone: true,  Encodes: true,  Decodes: true}
{Value: "ENC",     RequiresTone: true,  Encodes: true,  Decodes: false}
```

### Why modes are treated differently

`Modes` stays `[]string`. Giving 15 mode values a semantic descriptor
(`Family`, `Sideband`, `Narrow`) is a large addition to `core/spec` that
ripples into `codeplug.Validate`, the UI mode lists and every capabilities
fixture — to describe a set that M9c manual finding 4 says is **already
identical** between the FT-710 and the FTdx10.

The principled split: `core/csvio` owns *CHIRP-side* knowledge (that CHIRP's
`CW` should become an upper-sideband CW mode), `Capabilities` owns
*radio-side* knowledge (which display modes exist). §2 keeps the CHIRP
mapping table and membership-checks its result. A radio whose mode names
differ then gets a blocking loss entry, not a wrong mode — refusal, not
corruption.

### `spec.Validate` gains three checks

These are what make the seam trustworthy rather than decorative.

1. **`RequiresTone == Encodes || Decodes`.** The field is now derivable, so
   the invariant is enforced instead of left to drift.
2. **At most one `ShiftOption` per `Direction`**, and at most one `ToneState`
   per `(Encodes, Decodes)` pair. §2's `shiftFor(caps, ShiftUp)` needs a
   *unique* answer; an ambiguous capability set must fail at the driver, not
   silently resolve to whichever entry happens to be first.
3. **`ShiftOptions` routed through the existing `validateVocab`** via a value
   extractor — non-empty, no blanks, no duplicate values — exactly as
   `CTCSSStates` already is at `core/spec/validate.go:183-187`.

### Consumers

All mechanical, and each has the `CTCSSStates` version of itself adjacent:

| Site | Change |
|------|--------|
| `core/spec/validate.go:181` | value extractor before `validateVocab` |
| `core/codeplug/validate.go:339,342` | `containsString` → find-by-value; `quotedList` over extracted values. **Message text unchanged.** |
| `core/driver/ft710/ft710.go:527` | clone slice element type |
| `core/driver/ft710/caps.go:338` | unchanged — still `spec.StandardShiftOptions()` |
| `app/uispec.go:229` | flatten to `[]string`, mirroring line 230's `CTCSSStates` loop |

**`app/types.go:352` `UISpecView.ShiftOptions` stays `[]string`.** No
frontend change and no wailsjs regeneration arises from this section — the
flattening happens on the Go side, and the view type the frontend binds to
is untouched.

---

## §2 — `core/csvio/chirp.go`, and `doc.go` with it

### Signature

```go
func ImportCHIRP(rd io.Reader, caps spec.Capabilities) ([]codeplug.Channel, LossReport, error)
```

`core/csvio` already imports `core/spec`, and `codeplug.Validate` already
takes a `spec.Capabilities` — so this is the established shape, not a new
dependency.

### The six hardcoded facts

| Today | Becomes |
|-------|---------|
| `locN < 1 \|\| locN > 99` (`:305`), `fmt.Sprintf("%03d", locN)` (`:311`) | index into `caps.Bank(spec.BankMemory).Slots`: CHIRP Location *N* → `Slots[N-1]` |
| `len(b) > 12` (`:260`) | `caps.TagLen` |
| `"SIMPLEX"`/`"PLUS"`/`"MINUS"` (`:348-371`) | `shiftFor(caps, spec.ShiftNone/ShiftUp/ShiftDown)` |
| `"OFF"`/`"ENC"`/`"ENC-DEC"` (`:386-435`) | `toneStateFor(caps, encodes, decodes)` |
| `spec.ValidTone` (`:223`) | membership in `caps.CTCSSTones` |
| `"FT-710"` in ~10 `LossEntry.Detail` strings | `caps.Model` |

### What deliberately stays

- **`chirpModeMap` (`:124`).** CHIRP→display-name is migration policy, not a
  radio fact. Its *result* is now membership-checked against `caps.Modes`,
  producing a blocking `ActionUnsupported` entry distinct from "CHIRP mode
  has no equivalent at all".
- **`chirpTagByteOK` (`:234`).** Printable ASCII 0x20–0x7E excluding `;` is a
  family-wide CAT fact — `;` is the protocol terminator, not an FT-710
  quirk. Only its prose stops naming the FT-710.
- **No frequency range check.** `caps.MinFreqHz`/`MaxFreqHz` exist, but
  import is syntactic and `codeplug.Validate` is the semantic gate. Adding a
  range check here would duplicate a Validate rule, which `doc.go:35`
  explicitly forbids.

### Failure discipline

Every missing or ambiguous capability produces a **blocking `LossEntry`,
never a silent wrong value**: no `BankMemory`; a Location beyond the bank's
slot count; no `ShiftOption` with the needed `Direction`; no `ToneState` with
the needed encode/decode pair; a mapped mode absent from `caps.Modes`. This
preserves the project's standing posture — refuse, do not corrupt.

### `LossEntry.Detail` strings must stay byte-identical for the FT-710

This constraint is easy to miss and the byte-identity baseline will catch it
loudly, so it is stated here as a design rule rather than left to the
implementer.

`Detail` strings are *output*. Substituting `caps.Model` and `caps.TagLen`
into them must reproduce the FT-710's current text **exactly**, which means
each template is chosen to do so:

| Today | Template | FT-710 result |
|-------|----------|---------------|
| `"FT-710 memory slots are 001-099; Location is out of range"` | `"%s memory slots are %s-%s; …"` with model and the bank's first/last slot | identical |
| `"exceeds the FT-710's 12-byte tag limit"` | `"exceeds the %s's %d-byte tag limit"` with model and `TagLen` | identical |
| `"FT-710 stores no per-channel repeater offset; …"` | `"%s stores no per-channel repeater offset; …"` | identical |

Where a message names an implementation detail that genuinely no longer
applies — `"not in the FT-710's standard CTCSS chart
(spec.StandardCTCSSTones)"` now consults `caps.CTCSSTones`, so the
parenthetical is wrong — the wording **does** change. Every such deliberate
change must be enumerated in the implementation plan and appear in the
reviewed baseline diff. An unenumerated diff is a defect, not a surprise.

### The doc contract (this is item m40d)

`ImportCHIRP`'s contract paragraph (`:503-507`) and `doc.go:28-35` both say
the import consults no Capabilities. That becomes false and must be
rewritten — but **not simply deleted**, because the distinction it protects
is still real and still load-bearing:

> `ImportCHIRP` now consults `Capabilities` for **vocabulary and shape** —
> slot space, tag width, shift and CTCSS vocabulary, mode and tone
> membership. It still does **not** run `codeplug.Validate`, and a channel
> with no blocking entries against it is still not thereby guaranteed valid
> for the radio (band limits, per-field write support, radio identity).
> `Validate` remains the one semantic gate before a send.

`doc.go:17`'s "fields the FT-710 has no equivalent for" is neutralised in
the same edit. m40d lands here rather than as a separate cosmetic pass,
because it is the same sentence.

### Call sites

Both already have capabilities in scope; both need only a hoist.

- `cmd/rigprog/import.go:211` — `wiring.StaticCapabilities(*model)` is
  computed at `:263`, below the import branch. Hoist above it.
- `app/importexport.go:120` — `currentCaps(a.conn)` is called at `:154`.
  Hoist above the import.

---

## §3 — `radiotext`: complete coverage

### (a) The last hardcoded string — m42a

`app/frontend/src/lib/ChannelGrid.svelte:1020-1024` renders the grid legend
as `{appState.uiSpec.ToneScanSkipNote}` followed by a **second sentence
still hardcoded in Svelte**:

> Preservation across a rewrite is hardware-verified for Tone; Scan Skip
> preservation is not yet verified (see each cell's tooltip).

That claim is specific to Stuart's FT-710 write trials (13/07/2026). For a
radio pinned at `writeTrialsComplete=false` it is an **outright false
statement about hardware**, which is why it cannot stay in JS. `Text` gains:

```go
// ToneScanSkipVerification states what is and is not hardware-verified
// about Tone/Scan Skip preservation across a rewrite for this radio.
ToneScanSkipVerification string
```

FT-710 entry: the sentence verbatim. Served through `UISpecView` exactly as
`ToneScanSkipNote` already is (`app/uispec.go:242`).

**This is the only section that touches the frontend, and it needs a
wailsjs regeneration** — checked from the repo root, per the standing
convention.

### (b) The forcing function

A test pinning that **every model in `wiring.SupportedModels()` has a
`radiotext.For` entry**. Registering the FTdx10 without prose then fails a
test rather than silently degrading to blank advisories at
`cmd/rigprog/write.go:116`, `cmd/rigprog/probe.go:140` and
`app/uispec.go:242`.

`internal/wiring` owns `SupportedModels`; `internal/radiotext` is
stdlib-only, so the dependency direction is legal.

**Not a contradiction:** `cmd/rigprog/write_test.go:155,175` asserts that an
FTdx10 — a model `radiotext.For` has no entry for — produces no front-panel
erase procedure. That test stays valid and stays green. The guard covers
*registered* models only, and the FTdx10 is not registered.

---

## §4 — wiring and snapshots

### m39a — `internal/wiring/wiring.go:216`

```go
drv, _ := reg.Get(model) // just registered under this exact key
return drv.Capabilities(), nil
```

The discarded `ok` nil-panics if a future table key differs from its
driver's `Model()`. Propagate the error instead of panicking. Sibling
lookups (`StaticSettingsDescriptor`) call the driver directly and do not
have this shape.

### m39b — the invariant that makes m39a provably safe

A test pinning that `realDrivers` (`wiring.go:63`) and `fakeDrivers`
(`fake.go:70`) have **matching key sets**, and that **each key equals its
driver's `Model()`**. Codex called this "a natural guard when model #2
lands". It converts m39a's fix from a defence into a proof: the condition
cannot arise.

### D9 — snapshot directory per model

**Rule:** the model slug is appended for any **non-default** model, to
whichever base directory is in force — including an explicit
`--snapshot-dir`.

Applying it to the explicit flag as well as the derived default is
deliberate. Collision avoidance is the entire point of D9, and two models
sharing one explicitly-named directory is exactly the case where they would
collide.

- `DefaultModel` ("FT-710"): `<UserConfigDir>/rigprog/snapshots` — **byte
  identical to today**, so existing snapshots keep working
- any other model: `<base>/<model-slug>/`
- slug: lowercased, each run of non-alphanumeric characters collapsed to a
  single `-`

Touches `cmd/rigprog/fileio.go:175` and `internal/wiring/wiring.go:291`. The
two `--snapshot-dir` help strings (`read.go:119`, `write.go:666`) and
`usage.go:101,168` continue to show the FT-710 default and are unchanged.

---

## §5 — Explicitly unchanged: `currentCaps`

`app/app.go:236` returns the connected session's capabilities when
connected, and `wiring.StaticCapabilities(wiring.DefaultModel)` when not —
marked advisory. A working copy carries its own `codeplug.Radio.Model`
(`core/codeplug/radioinfo.go:11`), which `currentCaps` ignores.

**Decision: leave it alone.** CHIRP import uses whatever `currentCaps`
already returns, exactly as `codeplug.Validate` does at
`app/importexport.go:83-84`. `core/codeplug/validate.go:152` already raises
a model-mismatch issue when the working copy disagrees with the caps, so a
wrong-model import cannot pass unnoticed.

Resolving capabilities from the working copy's model belongs with the GUI
model picker in the FTdx10 slice. Doing it here would change `Validate`'s
behaviour as well as import's, widening this slice from plumbing into a
behaviour change.

---

## Verification

### Acceptance bar: FT-710 byte-identity

Fresh M9c baselines minted per `HANDOFF-m9c.md:108-109` (M9c-0's are at
`docs/superpowers/m9c0-baseline-manifest.md`; this slice mints its own),
covering `probe --fake`, `read --fake` and CHIRP import output — stdout,
stderr and exit code, per the m40a lesson that stdout-only comparison misses
drift.

### The anti-vacuity requirement

`core/csvio/chirp_test.go`'s existing 792 lines must all pass with FT-710
capabilities threaded in — that proves behaviour preservation. It does
**not** prove the capabilities are consulted rather than threaded through
and ignored.

So: **new cases drive a synthetic second capabilities value** with a
different `TagLen`, a different slot count, and renamed shift/CTCSS
vocabulary, asserting the import follows *it* and not the FT-710. A test
suite that would still pass if `chirp.go` ignored its new parameter has not
tested this change.

Matching negative cases: capabilities with no `BankMemory`, with two
`ShiftOption`s sharing a `Direction` (must fail `spec.Validate`), and with
no `ToneState` for a needed encode/decode pair.

### Gate

TDD throughout, per convention. Full local gate substitutes for CI (Actions
still billing-dead): gofmt, vet, build, all packages, all guards verbose,
importgraph and driver-seam pins, wailsjs regeneration no-diff, frontend
svelte-check plus vitest plus build, `-race` on `app`, and
`go test -race ./core/...` **in the background** — it exceeds a 10-minute
foreground limit.

## Sequencing

Branch `m9c1-registration-gate` off `main` (`ce7fcee`), merged `--no-ff`.

1. `core/spec` vocabulary semantics and the three `Validate` invariants
2. mechanical consumers — `codeplug/validate`, ft710 clone, uispec flatten
3. `chirp.go`, its two call sites, and `doc.go` (m40d)
4. `radiotext` widening (m42a), frontend, wailsjs regeneration
5. `radiotext` coverage guard, m39a, m39b
6. D9 snapshot directory
7. full gate, then Codex milestone review

Steps 1-2 are one unit: the type change does not compile until its consumers
follow.

## Risks

**The likely failure is mechanical, not architectural.** The `ShiftOptions`
type change touches many fixtures, so a slip that shifts FT-710 behaviour is
far more probable than a wrong design. Byte-identity baselines are what
catch that, which is why they are the acceptance bar rather than a
formality.

**Second: doc drift.** Three documents assert `ImportCHIRP` consults no
capabilities (`chirp.go:505-507`, `doc.go:28-35`, and `doc.go:17`'s FT-710
prose). Missing one leaves a false claim in the tree — the exact pattern the
M9a fix-wave re-review caught at `app/types.go:361`.

**Process lesson carried from M9c-0.** M9c-0 spent three of five review
rounds hardening an AST guard that approximated what behavioural tests
already covered, and a process review judged that disproportionate. The
`radiotext` coverage guard in §3 is a data check over a map, not an AST
walk — but the standing rule applies regardless: **"an approximate guard has
another bypass" is non-blocking, not "the product is wrong".**

---

## Post-review amendment (28/07/2026)

Recorded after this spec's body was written and implemented, following the
Codex adversarial milestone review and its three dispatched fix-up rounds
(A, B, C). The body above is left as originally approved; this note
records where subsequent findings changed its conclusions.

- **§5's `currentCaps` exclusion was REVERSED.** §5 decided "leave it
  alone": `currentCaps` would keep resolving its disconnected baseline from
  `wiring.StaticCapabilities(wiring.DefaultModel)` regardless of the
  working copy's own `Radio.Model`, on the grounds that fixing it belonged
  with the GUI model picker in the FTdx10 slice. Dispatch B's fix B1
  reversed this: an offline CHIRP import against a non-FT-710 working copy
  was transforming data against the wrong radio's vocabulary, with the
  mismatch clearing itself the moment the user reconnected — corrupted
  data passing the send gate with no trace. `currentCaps` now resolves the
  disconnected baseline from `working.Radio.Model` when it is non-empty and
  recognised, falling back to `wiring.DefaultModel` otherwise. FT-710
  behaviour is unchanged (see `app/app.go`'s `currentCaps` doc comment and
  `TestCurrentCaps_DisconnectedFT710WorkingIsByteIdentical`).
- **Two findings from the final review are carried forward, not closed.**
  Both are recorded as explicit, named prerequisites for the FTdx10 slice
  in `.superpowers/sdd/HANDOFF-m9c.md`'s "STILL OPEN" section
  (preconditions 10 and 11), not fixed by this milestone:
  - `app/`'s own consumers (`connection.go`, `uispec.go`, `send.go`,
    `settings.go`) still assume `wiring.DefaultModel`/the FT-710 in several
    places even though `currentCaps` itself is now model-aware — a drift
    between what capabilities resolve to and what the rest of `app/`
    assumes. `app/settings.go`'s `currentSettingsDescriptor` additionally
    carried a factually false doc comment (claiming a zero-descriptor
    fallback that the code never actually took), corrected in place by fix
    C3.
  - `internal/wiring/fake.go`'s `fakeDriverEntry` is concretely typed to
    `*fakeradio.Radio`/`[]fakeradio.Option` — the FT-710 simulator — so the
    planned `internal/fakedx10` cannot be added as merely another table
    row; the table abstraction itself needs to change first, and the
    existing tests would not catch a mismatched pairing if it were added
    anyway.
- **The tone-chart ownership question (§ absent from this spec's original
  scope, raised by the milestone review) was ruled on and closed**, not
  carried forward: `codeplug.ToneField.Valid` now takes the radio's
  `spec.Capabilities` and checks against `caps.CTCSSTones` rather than the
  package-global `spec.ValidTone` (fix C1). See
  `.superpowers/sdd/HANDOFF-m9c.md`'s corresponding "RESOLVED" note.
