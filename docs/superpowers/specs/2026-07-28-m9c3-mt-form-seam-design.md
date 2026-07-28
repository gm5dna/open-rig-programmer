# M9c-3 — the MT frame-form seam

**Date:** 28/07/2026 (revision 2: 29/07/2026)
**Status:** revision 2, review findings folded
**Milestone:** M9c-3, the last codec enabler before the FTdx10 dialect
(M9c-4) and its driver/registration (M9c-5)

> **Revision 2 (29/07/2026).** Revision 1 was reviewed adversarially by
> **Codex** (NEEDS-REVISION: 4 HIGH, 3 MEDIUM must-fix, 3 record/defer)
> and **Fable** (APPROVE-WITH-FIXES: 2 HIGH, 3 MEDIUM must-fix, 5
> record/defer), adjudicated in
> `.superpowers/sdd/m9c3-spec-review-adjudication.md`. The reviews
> converged, from different directions, on revision 1 committing the
> PadByte-conflation shape TWICE — in the P7 unification and in reusing
> `PadByte` as a fixed-width fill — in the same document that cites the
> lesson. Both are redesigned below. Two further revision-1 errors:
> "fixed-width tag" was asserted as manual fact when the manual says "up
> to 12 characters" (the FT-710's own hardware golden `MT005040M;`
> proves Yaesu grids draw maximal forms while radios accept variable
> Sets); and the inherited M9c-2 acceptance item "add the FTdx10 dialect
> to `allTestDialects()`" is structurally impossible — an import cycle —
> and is replaced. **Do not implement revision 1.**

## Decomposition decision, recorded

The remaining M9c work is three milestones, each with its own spec,
adversarially reviewed plan, and byte-identity gate:

- **M9c-3 (this spec):** the MT frame-form seam in `core/cat`, proven
  with disagreeing in-package fixtures only — no FTdx10-named code. Plus
  two live receiver-vs-global fixes in the FT-710 driver's write path.
- **M9c-4:** `core/cat/ftdx10` — the Table 2 transcription (~197
  entries, two independent transcriptions), the dialect via
  `cat.MustNewDialect`, the `ObservationsAbsent` extable profile, the
  generated inventory and staleness test, an **external conformance
  suite** (below — NOT `allTestDialects()`), and manual-derived
  long-frame goldens (ASSUMED until Stage R). The transcription must
  also **verify MC and every other reused command's table against the
  manual before the FT-710 codec is reused for it** — revision 1
  claimed "no form variance exists in evidence" for commands the
  evidence set did not cover.
- **M9c-5:** `core/driver/ftdx10` (`writeTrialsComplete=false`, every
  `Capabilities` field explicit), `internal/fakedx10`, wiring/CLI/GUI
  registration with the radiotext prose gate, preconditions 8–11 — and
  **two named design obligations this review created** (see "M9c-5
  obligations", below).

## The evidence

Re-read visually from the CAT manual PDF (rev 2308-F, page 16) on
28/07/2026, correcting the handoff's earlier 51-byte figure:

- **FTdx10 `MT` Set and Answer occupy positions 1-41**: `MT`(2) +
  slot(3) + frequency(9) + clarifier sign+magnitude(5) + P4 RX-clar(1) +
  P5 TX-clar(1) + P6 mode(1) + P7(1) + P8 CTCSS(1) + P9 "00" fixed(2) +
  P10 shift(1) + P11 "0" fixed(1) + P12 tag(12) + `;`(41st). The grid's
  41-50 header row holds only the terminator — the origin of the old
  miscount. P7 on a Set is documented "0: (Fixed)"; on Read, "0: VFO
  1: Memory". **The tag legend says "up to 12 characters"** — the same
  variable-length wording as the FT-710's, whose hardware accepts short
  Sets (`MT005040M;`, 10 bytes, pinned three times in `mt_test.go`).
  **Whether the FTdx10 answers fixed-width is therefore ASSUMED until
  Stage R**, exactly like M9c-4's goldens.
- **FTdx10 `MT` Read is 6 bytes** — identical to the FT-710's.
- **FTdx10 `MW` Set is 28 bytes, field-for-field identical in layout to
  the FT-710's memory frame** (`memdata.go`'s offsets apply unchanged);
  no Read, no Answer. Its P7 is "0: (Fixed)".
- **FTdx10 `MR` is identical to the FT-710's** (6-byte read, 28-byte
  answer, same offsets; P7 read vocabulary "0: VFO 1: Memory" — narrower
  than the FT-710's 0-5, harmless under read-side leniency for MR).

**Consequence: the frame-shape seam is exactly one command's form.**

## The claim, stated precisely

What blocks a combined-MT dialect today (verified at source): the MT
length window is package constants with FT-710 values
(`mt.go:12,27-30`, read at `mt.go:145`, `mt.go:223-224` — window AND
error text — and `allowlist.go:201`); `ParseMTAnswer` hardcodes the
short-form offsets (`mt.go:232-247`); `BuildMTSet` builds only the short
form; the gate's MT branch discriminates by short-form lengths; and
`Dialect` carries no form discriminator (`MTPolicy`'s own doc scopes it
to "the SHORT form only" — that doc, and `TagMaxBytes`'s, are rewritten
per-form by this milestone).

What does NOT need a seam: MR and MW layouts (one shared offset block),
the MW write-kind policy, the mode table, slot space, EX machinery.

### Two live receiver-vs-global defects, fixed here

`core/driver/ft710/write.go`'s `buildWriteCommands` takes `dialect
cat.Dialect` as a parameter and then ignores it twice: `Kind:
cat.KindMemory` at `:256` and the ±9990 literal at `:237`. That is the
M9b doctrine violation in a function that already holds the receiver —
the justification, precisely stated (revision 1's "would refuse a
correct FTdx10 write" was counterfactual, since this ft710-private
function never sees another dialect). Both fixes consult the dialect
through new exported accessors; FT-710 bytes are provably unchanged
(its dialect's values equal the old literals).

## Design

### `MTForm`, and per-form field ownership on `MTPolicy`

```go
type MTForm int

const (
    MTFormUnspecified MTForm = iota // zero: refused (M9c-1 ruling)
    // MTFormShort: MT<slot><display><tag 0..TagMaxBytes>; — the FT-710.
    MTFormShort
    // MTFormCombined: MT<the shared 28-position memory field block's
    // fields><P11 '0'><tag><;> — the FTdx10 family. This value pins
    // EXACTLY that layout: the classic memory block at memdata.go's
    // offsets, then P11, then the tag. A future radio whose combined
    // frame differs in its FIELD BLOCK is a new form (or, if slot/mode/
    // policy sharing has broken down entirely, a sibling codec) — it is
    // NOT a parameterisation of this one.
    MTFormCombined
)
```

`MTPolicy` becomes `{Form, TagMaxBytes, ClearTagByte, PadByte, TagFill}`
— still comparable (scalars only), so `dialectequiv_test.go:197`'s `!=`
compiles. **Each field belongs to one form, and V9 enforces ownership
both ways** (the M9c-2 `ObservedCSV`-under-`ObservationsAbsent`
pattern — an inapplicable field must be explicitly zero, an applicable
one explicitly valid):

| Field | Short | Combined |
|---|---|---|
| `Form` | `MTFormShort` | `MTFormCombined` (zero refused) |
| `TagMaxBytes` | longest accepted tag | longest accepted tag AND the frame's tag-field width — see "the recorded bet" |
| `ClearTagByte` | required valid wire byte | **must be 0** (no distinct clear encoding is documented for the combined form) |
| `PadByte` | 0 (no padding) or valid wire byte — unchanged | **must be 0** (answer trimming is `TagFill`'s job) |
| `TagFill` | **must be 0** | required valid wire byte (zero refused — an omitted fill must not silently emit NUL) |

V9 additionally proves each form's derived maximum frame length fits
`DefaultMaxFrame` — its current rationale covers only the short form.

Why enum-plus-branches and not function fields or a sibling codec:
`MTPolicy` comparability; `DialectConfig`'s flat-config doctrine
(`dialectconfig.go:103-107`); one command's two evidenced layouts need
no dynamic dispatch; a sibling codec forks the gate, corpora and guard
posture. **The extension path, honestly:** a third form is only another
enum member if the memory field block, slot semantics and policies
remain genuinely shared; evidence decides between a new form and a
sibling codec when (if) the FTX-1 arrives.

### Geometry: derived, never stored — and the recorded bet

Combined frame length = `29 + TagMaxBytes` (28 fixed positions + tag +
terminator; 41 for a 12-byte tag). No `41` constant anywhere.

**The bet, recorded:** deriving the tag-FIELD width from `TagMaxBytes`
gives one field two meanings (policy bound; frame geometry) that
coincide on the FTdx10 at 12/12. A combined radio whose field is wider
than its accepted tag length is inexpressible until a `TagFieldWidth`
split, which is additive. Taken deliberately: the alternative — two
independently configurable facts that must always agree — is the worse
default, and no counter-radio is in evidence. (Both reviewers examined
this from opposite sides; the adjudication records the resolution.)

### Tag semantics for the combined form

- **Build always emits the full-width field**: the tag padded to exactly
  `TagMaxBytes` with `TagFill`. Correct under BOTH the fixed-width and
  variable-width readings of the manual (the FT-710 accepts its own
  full-width padded form).
- **An empty tag is the all-fill field.** There is no distinct clear
  encoding; decode of an all-fill field yields `""`.
- **Build refuses a tag with trailing `TagFill` bytes** (and, as with
  the short form, over-length and bad charset) — validate, don't
  canonicalise. With that refusal, build → parse round-trips are EXACT
  for every accepted tag; the short form's answer-side pad-trim
  adjudication is not imported.
- **Parse and gate require the exact derived length — ASSUMED until
  Stage R.** If FTdx10 hardware answers variable-width (the FT-710
  precedent makes this live), the contingency is a parser/gate window of
  `30..29+TagMaxBytes` — a contained, recorded change, not a schema
  change. The gate's exactness is safe regardless: it checks outbound
  frames, and we only ever build full-width.

### P7: a form-schema constant, NOT a policy

The combined Set's P7 is the form's schema constant `'0'` ("(Fixed)"),
emitted and gate-required as such. **It is deliberately NOT derived from
`mwWriteKind`** — revision 1 did that, and it is two command-specific
facts (MT-Set P7, MW-Set P7) coinciding on the evidence radio: the
PadByte shape. If hardware ever divorces them, an MT-specific policy
field is added then, additively.

`BuildMTSetCombined` validates `m.Kind == the Set constant`
(validate-don't-rewrite, MW's posture) and documents that Set-direction
`'0'` means "(Fixed)", not "VFO". `ParseMTAnswerCombined` accepts
exactly `{'0','1'}` (the documented read vocabulary) — NOT MR's 0-5
`validKindByte`, which would turn undocumented frames into valid data.

The disagreeing fixture **proves the decoupling**: it carries an MW
write kind different from `'0'` while its combined MT Set still builds
and gates with P7 `'0'`.

### API surface

Short-form API unchanged in signature, behaviour AND error text for the
FT-710 — the parser corpus bakes `"MT answer must be 7-19 bytes"` into
ten golden lines, and the derived window must render those bytes.
**Stated plainly: deriving the short window (`7..7+TagMaxBytes`) changes
non-FT-710 acceptance** — the existing 6-byte-tag peer fixture's parser
today accepts up to 19 bytes and will refuse >13. Correct, intended, and
the plan must expect churn in existing peer tests.

New, each refusing symmetrically on the wrong form:

```go
func (d Dialect) BuildMTSetCombined(m MemoryData, tag string) (Command, error)
func (d Dialect) ParseMTAnswerCombined(frame []byte) (MemoryData, string, error)

// MTAnswerBounds returns the validated answer-length bounds for this
// dialect's MT form (equal min/max for the combined form's exact
// length). An error, not zeros, on an unconfigured dialect — so a
// driver building a transport CommandSpec cannot read a plausible
// zero. This is the receiver-derived geometry M9c-5's spec factories
// consume instead of hardcoding 41 or re-deriving 29+TagMaxBytes.
func (d Dialect) MTAnswerBounds() (min, max int, err error)
```

`BuildMTRead` is form-independent (6 bytes on both radios). P11 is
emitted `'0'` and required `'0'` on parse. The combined field block
reuses `memdata.go`'s one offset set — the plan chooses the extraction
shape under the constraint that MR/MW behaviour and error text are
byte-identical.

### The gate

`validMTCommand` branches on `d.mt.Form` after the form-independent
6-byte read check. Short: today's logic, unchanged bytes. Combined:
exact derived length; per-field validation reusing the same checks the
MW branch uses; P7 == `'0'`; P11; tag charset at full width with
trailing-fill refusal consistent with the builder. Zero form refuses.

**The Set/Answer collision narrows**: Set and Answer share the wire form
in both MT forms (`allowlist_test.go:138`'s documented exception), but a
combined answer carrying P7 `'1'` is refused by the gate's constant
check — the residual collision is only the P7-`'0'` case. No legitimate
outbound frame is refused (every legitimate combined Set has P7 `'0'`;
the gate is outbound-only and has no engine-correlation consequence).
The exception-table entry records the narrowed form.

### Real-dialect conformance replaces the impossible `allTestDialects()` item

`allTestDialects()` lives in an in-package `package cat` test; a real
`core/cat/ftdx10` dialect importing back would be an import cycle — the
M9c-2 spec's acceptance item was impossible as written and is
**withdrawn**. Instead:

- M9c-3's disagreeing fixtures are in-package and join
  `allTestDialects()` as always.
- **M9c-3 delivers the generic dialect-conformance properties as an
  exported-API test suite runnable from an external package** (the
  subset of today's generic walks expressible through exported API —
  builders, parsers, gate, round-trips), and M9c-4's acceptance item
  becomes: run that suite over the real FTdx10 dialect from
  `package cat_test` or the ftdx10 package's own tests.

### Guards

`internal/guards`' `importgraph_test.go` write-composition selector
fence currently recognises exactly `BuildMWSet` and `BuildMTSet`; a new
public Set builder outside the fence would be callable from an
unauthorised package undetected. **`BuildMTSetCombined` joins the
fence, its documentation, and the non-vacuity accounting.**
`dialectglobals_test.go`'s `promotedConstants` and
`gateReachingValidators` hold; the new combined gate-reaching
validators join the latter; no new package-level MT geometry constant
may appear.

### Tests and proof obligations

- **Byte identity, the milestone bar:** all four `core/cat/testdata`
  goldens byte-identical (never regenerated); `probe --fake`,
  `read --fake` and CHIRP import byte-identical from compiled binaries
  vs a pre-milestone worktree (M9c-1 manifest method);
  `exinventory_gen.go` untouched.
- **Disagreeing fixtures:** a combined-form fixture with `TagMaxBytes`
  ≠ 12 (e.g. 6 → 35-byte frames) in `allTestDialects()`; a second
  combined fixture differing in `TagFill`, MW write kind (≠ '0', the
  decoupling proof) and slot/mode attributes per the peer posture.
- **Form-refusal matrix**, both directions, each case failing if the
  form check is deleted.
- **Round trip:** exact for every accepted tag, including empty →
  all-fill → `""`, on both fixtures; trailing-fill refusal tested at
  build AND gate.
- **Gate-admissibility walk:** the non-vacuity counters become
  form-aware — each dialect's own-form builders must contribute, and
  wrong-form builders must be SEEN to refuse.
- **`MTAnswerBounds`:** FT-710 (7,19,nil); combined fixture equal
  bounds; zero dialect errors.
- **V9:** one refusal test per ownership rule in the table above, plus
  the derived-max-vs-`DefaultMaxFrame` proof for both forms. The
  tightening's blast radius is ~14 existing `MTPolicy` literals
  including one OUTSIDE core/cat
  (`core/driver/ft710/modebyname_test.go:54`) — the plan enumerates
  them; the repo-wide grep is the gate.
- **Driver fixes:** FT-710 output bytes pinned unchanged; a unit test
  proving the MW frame's kind byte and the clarifier bound follow the
  dialect.

### Error handling

Typed `*ParseError`/gate refusals in the existing style, naming the
form mismatch or offending field, never dumping a frame. Wrong-form API
use is an error return, not a panic.

## M9c-5 obligations created by this review (recorded, not built here)

1. **`TagDisplay` is unrepresentable for the FTdx10** — its MT has no
   display flag, and `codeplug.Channel.TagDisplay bool`'s omission
   reads as a plausible `false` through diff (`diff.go:174-201`), clone
   verification (`execute.go:112-141`), CSV (`export.go:112-135`,
   `import.go:325-329`) and CHIRP. M9c-5's spec must make a model
   decision — an evidence-backed constant semantic, or an
   unknown/unsupported state with capability-aware consumers — possibly
   as its own prerequisite seam. Marking the capability unsupported is
   NOT sufficient while diff/verify treat the field as known.
2. **The FTdx10 write/read choreography is undefined** — combined MT
   duplicates every memory field, so MT-only vs MW+MT (a double write?)
   vs MR+MT with an authoritative answer must be DECIDED, and
   `driver.WriteResult`'s journal-persisted `MWSent`/`MTSent` semantics
   made intentional for the chosen shape.
3. **Transport `CommandSpec`s** use `MTAnswerBounds()` and a
   slot-qualified `ExpectPrefix` (`"MT"+slot.Wire()`), per the engine
   doc's requirement when responses share a command prefix.

## Out of scope, and recorded

Anything named FTdx10; `driver.WriteResult` field names; transport spec
factories; `internal/fakeradio` (stays short-form; its swap point is
reserved at `parser.go:484-486`); `internal/fakedx10`; MC and every
other command (no additional seam is **currently evidenced** — M9c-4's
transcription must verify each reused command's table before reuse).

## Acceptance

1. Full local gate green (gofmt, build, vet, full suite, guards).
2. All four goldens byte-identical; `exinventory_gen.go` and the
   evidence golden untouched.
3. Probe/read/CHIRP byte-identity manifest vs a pre-milestone worktree,
   committed as `docs/superpowers/m9c3-baseline-manifest.md`.
4. Every combined-form path exercised via the generic walks and the new
   exported conformance suite — not only bespoke tests.
5. The two driver fixes land with FT-710 byte-identity proven.
