# M9c-3 — the MT frame-form seam

**Date:** 28/07/2026
**Status:** revision 1, awaiting adversarial review
**Milestone:** M9c-3, the last codec enabler before the FTdx10 dialect
(M9c-4) and its driver/registration (M9c-5)

## Decomposition decision, recorded

The remaining M9c work (the FTdx10 vertical slice) is executed as three
milestones, each with its own spec, adversarially reviewed plan, and
byte-identity gate:

- **M9c-3 (this spec):** the per-command frame-form seam in `core/cat`,
  proven with disagreeing test dialects only — no FTdx10-named code.
  Plus two live receiver-vs-global fixes in the FT-710 driver's write
  path (below).
- **M9c-4:** `core/cat/ftdx10` — the Table 2 transcription (~197 entries,
  two independent transcriptions per project convention), the dialect via
  `cat.MustNewDialect`, the `ObservationsAbsent` extable profile, the
  generated inventory and its staleness test, `allTestDialects()`
  membership, and manual-derived long-frame goldens (ASSUMED until Stage
  R, per the roadmap's community-evidence protocol).
- **M9c-5:** `core/driver/ftdx10` (`writeTrialsComplete=false` pinned,
  every `Capabilities` field explicit), `internal/fakedx10`
  (independently hand-derived), wiring/CLI/GUI registration with the
  radiotext prose gate, and registration preconditions 8–11.

Rationale: this matches how every M9 milestone has actually succeeded —
one architectural claim per milestone, a byte-identity bar each, and the
M9b precedent that a seam is proven by a fixture built to disagree before
any real second user exists.

## The evidence, corrected

**The handoff's "51-byte MT" was wrong.** Re-read visually from the CAT
manual PDF (rev 2308-F, page 16) on 28/07/2026:

- **FTdx10 `MT` Set and Answer are 41 bytes**: `MT`(2) + slot(3) +
  frequency(9) + clarifier sign+magnitude(5) + P4 RX-clar(1) + P5
  TX-clar(1) + P6 mode(1) + P7(1) + P8 CTCSS(1) + P9 fixed "00"(2) + P10
  shift(1) + **P11 fixed "0"**(1) + **fixed-width 12-character tag**(12) +
  `;`(1). The manual grid's 41-50 header row holds only the terminator at
  41 — that template row is where the 51/50 miscount came from. The tag
  is a fixed-width field, not the FT-710's 0-12 variable form. P7 on a
  Set is "0: (Fixed)"; on Read it reports 0: VFO / 1: Memory.
- **FTdx10 `MT` Read is 6 bytes** (`MT` + slot + `;`) — identical to the
  FT-710's.
- **FTdx10 `MW` Set is 28 bytes and field-for-field identical in layout
  to the FT-710's memory frame** (`core/cat/memdata.go`'s offsets apply
  unchanged); `MW` has no Read and no Answer on either radio. The only
  MW difference is the P7 write-kind byte — already dialect data
  (`mwWriteKind`, promoted at M9c-0).
- **FTdx10 `MR` is identical to the FT-710's** — 6-byte read, 28-byte
  combined answer, same offsets.

**Consequence: the frame-shape seam is exactly one command's form.** MT
has two forms across the two radios; MR, MW, MC, EX and everything else
share their shapes. The roadmap's fear of general per-command frame
variants does not materialise for this radio pair.

Corrections applied to `m9c-manual-provenance.md` and `HANDOFF-m9c.md`
(both local-only files) on 28/07/2026.

## The claim, stated precisely

### What blocks a combined-MT dialect today (all verified at source)

1. **The MT length window is package constants with FT-710 values.**
   `mtReadLen = 6`, `mtAnswerMinLen = 7`, `mtAnswerMaxLen = 19`
   (`core/cat/mt.go:12,27-30`), read at `mt.go:145` (build capacity),
   `mt.go:223-224` (parse window and its error text) and
   `allowlist.go:201` (gate window). `mt.go:14-26` itself records that a
   wider dialect "would need this constant revisited into a receiver
   method".
2. **`ParseMTAnswer` hardcodes the short-form offsets** (`frame[2:5]`,
   `frame[5]`, `frame[6:len-1]`, `mt.go:232-247`); there is no MT decoder
   indirection of any kind.
3. **`BuildMTSet` builds only the short form** (`mt.go:122-151`).
4. **The gate's MT branch** (`validMTCommand`, `allowlist.go:192-216`)
   discriminates read-vs-set by total length against the short-form
   constants.
5. **`Dialect` carries no form discriminator.** `MTPolicy` is
   `{TagMaxBytes, ClearTagByte, PadByte}` (`dialectconfig.go:50-84`) and
   its own doc scopes it to "the SHORT form only".

### What does NOT need a seam (and must not get one)

- MR, MW frame layouts — shared byte-for-byte (`memdata.go`'s single
  offset block); the FTdx10 reuses them unchanged.
- The MW write-kind policy — already dialect data.
- The mode table, slot space, EX machinery — already dialect data.

### Two live receiver-vs-global defects, found during scoping

Both are the M9b defect shape (bound on the receiver, datum from a
global), both in `core/driver/ft710/write.go`, both currently
behaviour-identical for the FT-710 and silently wrong for any second
model:

- `buildWriteCommands` hardcodes `Kind: cat.KindMemory` (`write.go:256`)
  instead of the dialect's own write kind — the exact value the FTdx10
  documents differently ('0'), and the exact promotion M9c-0 made.
  `validateMWFields` would REFUSE a correct FTdx10 write built this way.
- The clarifier pre-check hardcodes ±9990 (`write.go:237`) instead of the
  dialect's `ClarifierPolicy`.

M9c-3 fixes both, because they are defects now, not FTdx10 features:
the fix is to consult the receiver, and FT-710 behaviour is provably
unchanged (its dialect's values equal the old literals).

## Design

### `MTForm` — a zero-invalid form discriminator on `MTPolicy`

```go
type MTForm int

const (
    // MTFormUnspecified is the zero value and is NOT a valid form:
    // an omitted form must refuse, not default (the M9c-1 ruling).
    MTFormUnspecified MTForm = iota
    // MTFormShort: MT<slot><display><tag 0..TagMaxBytes>; — the FT-710.
    MTFormShort
    // MTFormCombined: MT<memory field block><P11 '0'><tag, FIXED
    // TagMaxBytes wide>; — the FTdx10 family.
    MTFormCombined
)
```

`MTPolicy` gains `Form MTForm`. It stays a comparable struct (int field),
so `dialectequiv_test.go:197`'s `want.mt != got.mt` still compiles — this
constraint is why the seam is an enum-plus-branches, **not**
function-valued fields or an interface: `DialectConfig`'s own doc
(`dialectconfig.go:103-107`) commits to "DATA, not behaviour", a flat
struct validated exhaustively in one place, and nothing about two forms
needs dynamic dispatch. A third form, if one ever exists, is added where
these two live, under the same validation. The sibling-codec alternative
(roadmap F1's fallback) is rejected as strictly heavier: it forks the
gate, the corpus tests and the guard posture for what is one command's
layout.

`cat.FT710`'s literal and `ft710DialectConfig` both set `MTFormShort`
explicitly. V9 (`validateMTPolicy`) rejects `MTFormUnspecified` and any
out-of-enum value.

### Frame geometry is derived, never stored

The combined form's frame length is `29 + TagMaxBytes` (2 command + 3
slot + 9 freq + 5 clar + 7 single-byte fields + 2 P9 + 1 terminator...
stated precisely: 2+3+9+5+1+1+1+1+1+2+1+1 = 28 fixed positions + a
TagMaxBytes-wide tag + 1 terminator = `29 + TagMaxBytes`). For the
FTdx10's 12 that is 41, matching the manual byte count. No `41` constant
appears anywhere; the FTdx10's number arrives in M9c-4 as
`TagMaxBytes: 12`, and a disagreeing fixture with a different width
proves the derivation in M9c-3.

The combined form's field offsets are the existing memory-block offsets
(`memdata.go:16-29`) — the manual shows the same fields at the same
positions — followed by P11 at the position after `memShiftOffset`, then
the tag. The implementation reuses the one offset block rather than
declaring a second copy; the plan decides the exact refactor shape
(extracting a field-block encoder/decoder that both `parseMemoryFrame`
and the combined MT use), under the constraint that MR/MW behaviour and
error text are byte-identical.

### API surface

**Short-form API: unchanged.** `BuildMTSet(Slot, bool, string)`,
`BuildMTRead(Slot)`, `ParseMTAnswer([]byte) (Slot, bool, string, error)`
keep their signatures and, for the FT-710, their exact behaviour AND
error text — the parser-corpus golden bakes the literal string
`"MT answer must be 7-19 bytes"` into ~10 lines
(`parser-corpus.golden:16,22,40,88`), so the short-form window message is
rendered from the derived values and must produce those same bytes.
Called on a combined-form dialect, `BuildMTSet` and `ParseMTAnswer`
**refuse** with an error naming the dialect's form — a short-form frame
sent to a combined-form radio would be garbage, and vice versa.

**Combined-form API: new.**

```go
// BuildMTSetCombined builds the combined write/tag frame. The tag is
// validated by the same charset rule as the short form and padded to
// exactly TagMaxBytes with PadByte; empty means the clear form.
func (d Dialect) BuildMTSetCombined(m MemoryData, tag string) (Command, error)

// ParseMTAnswerCombined decodes a combined answer into the memory
// fields and the decoded tag.
func (d Dialect) ParseMTAnswerCombined(frame []byte) (MemoryData, string, error)
```

Each refuses on a short-form dialect, symmetrically. `BuildMTRead` is
form-independent (both radios use the 6-byte read) and stays as is.

P11 is emitted as `'0'` and required to be `'0'` on parse — it is a
documented fixed field, and a non-'0' P11 is a finding, not data. P7 in
a combined Set carries the dialect's `mwWriteKind` (the manual's Set
value "0: (Fixed)" IS the FTdx10's write kind — one policy, not two);
on parse it is validated by the same read-side kind vocabulary MR uses.

### The gate

`validMTCommand` branches on `d.mt.Form` after the existing read-length
check (the 6-byte read is form-independent):

- `MTFormShort`: today's logic, unchanged bytes, unchanged messages.
- `MTFormCombined`: exact derived length; field validation reusing the
  same per-field checks the MW branch uses (slot via `d.mtSlotValid`,
  mode via `d.ParseMode`, clarifier via `d.validClarHz`, kind vs
  `d.mwWriteKind`, CTCSS/shift/P9 as in `parseMemoryFrame`), P11, then
  tag charset via `validMTTagByte` at fixed width.
- The Set/Answer collision NARROWS for the combined form, and the spec
  states this precisely so the exception table stays honest: Set and
  Answer share the wire form in both MT forms (`allowlist_test.go:138`'s
  documented exception), but a combined ANSWER may carry P7 `'1'`
  (Memory) where a combined SET must carry the write kind `'0'` — so the
  gate's kind check refuses every answer-shape whose P7 is not the write
  kind. The residual collision is only the P7-equal case. The exception
  table entry and its pinning test record the narrowed form. (Inbound
  parsing is different: `ParseMTAnswerCombined` accepts the read-side
  P7 vocabulary, exactly as `ParseMRAnswer` does.)
- A zero/unspecified form refuses (fail closed), which the zero-`Dialect`
  gate test already forces.

### Tests and proof obligations

- **Byte identity, the milestone bar:** all four `core/cat/testdata`
  goldens byte-identical (never regenerated); `probe --fake`,
  `read --fake` and CHIRP import byte-identical from compiled binaries
  against a pre-milestone worktree (the M9c-1 manifest method);
  `exinventory_gen.go` untouched.
- **The disagreeing fixtures** (the M9b lesson, applied twice):
  1. A combined-form fixture dialect with `TagMaxBytes` ≠ 12 (e.g. 6 →
     35-byte frames), joined to `allTestDialects()` so every generic
     walk (`TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible`,
     round-trip, zero-dialect floor) covers it.
  2. A second combined fixture differing from the first in the fields the
     first can't catch (clear byte, pad byte, write kind), mirroring the
     peer-dialect posture.
- **Form-refusal matrix:** every MT API called on the wrong-form dialect
  refuses, both directions, with tests that FAIL if the form check is
  deleted (each case's input is otherwise valid for the other form).
- **Round trip:** `BuildMTSetCombined` → gate-admissible → 
  `ParseMTAnswerCombined` reproduces `MemoryData` + tag exactly,
  including clear-form and pad-trim tag behaviour, on both fixtures.
- **The gate-admissibility walk's non-vacuity counters**
  (`dialectgate_test.go:222-231`) become form-aware: for each dialect,
  the builders OF ITS FORM must contribute ("MT set combined (tagged)"
  etc.), and the wrong-form builders must be seen to refuse — a counter
  that silently skips a refused builder would let a broken fixture pass
  vacuously.
- **Guard posture:** `internal/guards/dialectglobals_test.go`'s
  `promotedConstants` and `gateReachingValidators` lists hold unchanged;
  no new package-level MT geometry constant may appear for the combined
  form (the derivation rule above), and the guard's list gains the new
  gate-reaching combined validators.
- **Driver fixes:** `buildWriteCommands` consults new exported accessors
  (`Dialect.MWWriteKind() byte`, and the clarifier bound via the
  existing policy accessor or a new one — plan's choice) with tests that
  pin FT-710 output bytes unchanged, plus a unit test proving the built
  MW frame's kind byte follows the dialect, not the package constant.

### Error handling

Every new refusal is a typed `*ParseError`/gate refusal in the existing
style, naming the form mismatch or the offending field, never dumping a
full frame into an error a UI might show (matching `mt.go`'s current
practice). Wrong-form API use is an error return, not a panic — a
programming error upstream must still fail closed downstream.

## Out of scope, and recorded

- **Anything named FTdx10** — no dialect value, no package, no `41`
  anywhere. M9c-4's.
- **`driver.WriteResult`'s `MWSent`/`MTSent` field names** (persisted
  into clone journals, `core/clone/execute.go:456-457`). The FTdx10's
  write sequence and its reporting shape are M9c-5 driver design; the
  codec seam neither needs nor prejudges them.
- **`transport.CommandSpec` factories** (`mtSpec` etc.) — per-driver
  code, M9c-5.
- **`internal/fakeradio`** stays FT-710-short-form; its own doc already
  reserves the swap point (`parser.go:484-486`). `internal/fakedx10` is
  M9c-5.
- **MC and every other command** — no form variance exists in evidence.

## Acceptance

1. Full local gate green (gofmt, build, vet, full suite, guards).
2. All four `core/cat/testdata` goldens byte-identical, `git diff` clean
   on `exinventory_gen.go` and the evidence golden.
3. Probe/read/CHIRP byte-identity manifest vs a pre-milestone worktree,
   M9c-1 method, committed as `docs/superpowers/m9c3-baseline-manifest.md`.
4. Every new combined-form path exercised by the disagreeing fixtures
   through the generic dialect walks — not only by bespoke tests.
5. The two driver fixes land with FT-710 byte-identity proven.
