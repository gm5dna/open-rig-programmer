# M9c-4 — the FTdx10 dialect and its Table 2 transcription

**Date:** 29/07/2026
**Status:** revision 1, awaiting adversarial review
**Milestone:** M9c-4, the first real second-model consumer of everything
M9c-0..3 built. M9c-5 (driver, fake, registration) follows.

## What this milestone is

`core/cat/ftdx10` — a data-only model package holding the FTdx10's
dialect, built through `cat.MustNewDialect`, its EX inventory generated
from a committed Table 2 transcription through the M9c-2 extable
machinery, proven by the M9c-3 `dialecttest` conformance suite and by
manual-derived goldens marked ASSUMED until Stage R. **No driver, no
fake, no registration, no wiring** — M9c-5's.

Plus, first, **the three ledger items the M9c-3 reviews created**, done
before the model package exists because they constrain it:

1. **Narrow the Set-builder fence's carve-out** from the `core/cat/**`
   prefix to exactly `core/cat` + `core/cat/dialecttest` — so the new
   model package is NOT exempt. A data-only model package has no
   business calling Set builders, and after this change the guard, not a
   review checklist, enforces that (Fable milestone finding 2; the
   alternative — a standing review obligation — decays).
2. **Pin "no production package imports `core/cat/dialecttest`"** in the
   composition guard (it imports `testing`; the rule is currently a
   comment) (Fable finding 3).
3. **Give the combined Set's P7 an exported spelling**:
   `cat.CombinedMTSetKind byte = KindVFO`, documented as the
   Set-direction "(Fixed)" value whose byte happens to equal the
   read-side KindVFO — so `ftdx10`'s own tests (and M9c-5's driver) stop
   writing `cat.KindVFO` for a value the manual calls "(Fixed)" (both
   reviewers; additive).

## The evidence base

All from CAT manual rev 2308-F (acquired, verified, gitignored;
`docs/fixtures-private/manuals/`), read from the layout text with the
PDF as arbiter (the established extraction caveat: `-layout` only, and
Table 2's group labels float across columns):

- **Table 2 spans layout lines ~652-915**: rows are
  `P3 | Function | P4 | Digits` with P1/P2 group labels floating left
  (`01 (RADIO SETTING)`, `02 (MODE AM)`, …). EX addressing 2+2+2,
  ranges P1 01-05, P2 01-07, P3 01-23; the M9 plan review counted ~197
  entries (a sanity bound, not the truth — the transcription decides).
- **Text rows exist**: MY CALL ("Up to 12 characters", Digits 12,
  layout line 887) at minimum — so the profile's `TextWidth: 12` is a
  manual fact, not an invented value; the transcription enumerates the
  full text-row set.
- **CAT ID 0761. Mode nibble table and slot space identical to the
  FT-710's** (manual pages verified at M9c-3 scoping). **MW Set P7 "0:
  (Fixed)"** → `MWWriteKind: cat.KindVFO` — the first promoted policy
  to actually differ between radios, and `validMWWriteKindByte` admits
  it (verified at source).
- **MT is the combined form**: `MTPolicy{Form: MTFormCombined,
  TagMaxBytes: 12, TagFill: ' '}` → 41-byte frames. **`TagFill: ' '` is
  ASSUMED** (the FT-710 pads with spaces; the FTdx10's padding byte has
  never been observed) and is recorded as such beside the exact-length
  assumption M9c-3 already carries.
- **Clarifier**: the MR/MT/MW tables give the range 0000-9990 Hz; the
  STEP is not stated in those tables. The transcription task must
  locate the step in the manual (the RT/clarifier command pages) or
  record it ASSUMED-equal-to-the-FT-710's-10 Hz with the evidence gap
  named. `ClarifierPolicy{StepHz: <found or assumed>, MaxAbsHz: 9990}`.
- **Reused-command verification** (the M9c-3 spec obligation): before
  the dialect reuses the FT-710 codec for MC, ID, AI, MR, MW and the EX
  read/answer grammar, the transcription task verifies each command's
  table against the manual and records the verification (the
  availability table at layout line ~233 and each command's frame
  section). Any deviation found is a STOP-and-respec, not something to
  absorb.

## Design

### The package

```
core/cat/ftdx10/
  doc.go            — provenance, ASSUMED-until-Stage-R posture
  table2.csv        — transcription A (committed; provenance header)
  exinventory.go    — go:generate directive (-profile ftdx10); EX access
  exinventory_gen.go— generated ([]cat.EXItem via the M9c-2 profile)
  dialect.go        — the DialectConfig literal + Dialect() accessor
  ...tests
```

`func Dialect() cat.Dialect` returns a package-held value built by
`cat.MustNewDialect` at init — the A1 shape. Every config field is set
explicitly, including where the zero value happens to be right (the
standing zero-value instruction). The mode table and slot space are
FRESH transcriptions typed into this package (they cannot be copied —
`cat`'s are unexported — and that is a feature: a copy error is caught
by the identity tests below, a shared reference would prove nothing).

### The extable profile

Registered as `"ftdx10"` in `internal/extable`: `Package: "ftdx10"`,
`Types: TypesImported`, `ImportAlias: "cat"`, `VarName: "exItems"`,
`OutFile: "exinventory_gen.go"`, `ManualCSV: "table2.csv"`,
`Observations: ObservationsAbsent` (no hardware exists; the map must be
EMPTY), `MinDigits: 1`, `MaxDigits: 4`, `TextWidth: 12`,
`MaxObservedWidth: 12` (required positive, inert under Absent — the
recorded M9c-2 decision), `ExpectedRows: <the transcription's exact
count>`. The ftdx10 package carries its own staleness test driven from
`extable.RegisteredProfiles()` (the M9c-2 design), which also — being
registry-driven — begins covering BOTH profiles' generated files.

### Two independent transcriptions, the M8a convention

The convention's value is that transcription errors are caught by
disagreement, so the two must be independently produced:

- **Transcription A** → `table2.csv` (extable's 10-column shape:
  p1,p2,p3,p1_label,p2_label,name,p4,digits,text,manual_line; the P4
  column verbatim as audit trail; manual_line = the layout-text line).
- **Transcription B** → a COMPACT hand-derived inventory (the
  fakeradio `exGroups` widths-string shape) committed as test data in
  `core/cat/ftdx10`, produced by a DIFFERENT implementer in a separate
  session from the layout text + PDF, without sight of A.
- **The cross-check test** binds them exactly: address sets equal,
  per-address digits/width equal, text-row sets equal. Any mismatch is
  a transcription defect to resolve against the PDF — the test fails
  until the two agree. B is retained after the cross-check: M9c-5's
  `fakedx10` consumes it, keeping the fake's inventory independent of
  the production one exactly as fakeradio's is.

`ExpectedRows` is set from the AGREED count, and the ~197 estimate is
asserted only as a sanity band (>= 150, <= 250) in a comment, not code.

### Goldens — ASSUMED until Stage R

`core/cat/ftdx10/testdata/` gets its own golden corpus with a
provenance header stating every vector is MANUAL-DERIVED and UNVERIFIED
against hardware (Stage R lifts this): hand-derived combined-MT
build/parse vectors (each field placed by the manual's position table,
independently of `mtcombined.go`'s arithmetic — the golden derivation
must not reuse the code under test), MW/MR vectors, EX read vectors for
a sample of addresses, and a gate corpus (every builder's output
re-judged by the dialect's own gate). The FT-710's
`core/cat/testdata` two-commit rule is untouched; the ftdx10 corpus
gets the same never-regenerate discipline from birth, with its OWN
provenance rules.

### Proof obligations

- `dialecttest.Run(t, ftdx10.Dialect())` and the conformance suite's
  walk — the M9c-3 deliverable doing its job for the first real model.
- **Identity pins where the manual says "identical to the FT-710"**:
  the mode table and slot space are compared against `cat.FT710`
  through exported API (ModeName over the mode space; slot classifiers
  over the wire space) — a fresh-transcription typo fails loudly.
- **Difference pins where the manual says different**: `CATID() ==
  "0761"`; `MWWriteKind() == cat.CombinedMTSetKind` (the FTdx10's MW
  kind and the combined Set constant are the same byte '0' — assert
  with a comment that this equality is the FTdx10's fact, not a rule);
  `MTForm() == MTFormCombined`; `MTAnswerBounds() == (41, 41)`;
  EX inventory disjointness sample (P1=05 exists here, not on the
  FT-710; the FT-710's P1=06 absent here).
- The evidence-literal discipline: all new files; nothing pinned is
  touched.
- FT-710 byte identity: nothing in this milestone touches an FT-710
  path (`cat` changes are the three ledger closures, all additive or
  guard-side), but the full byte-identity gate runs anyway — the
  M9c-1 method, cheap insurance.

## Out of scope, and recorded

`core/driver/ftdx10`, `internal/fakedx10`, wiring/CLI/GUI registration,
radiotext prose, preconditions 8-11, the TagDisplay representability
decision and write choreography (all M9c-5, already ledgered); any
hardware claim (Stage R); the FTdx101D/MP (their own milestone, after
M9c closes, per Stuart's standing instruction).

## Acceptance

1. Both transcriptions agree exactly; the cross-check test is
   committed and green; `ExpectedRows` equals the agreed count.
2. `go generate ./core/cat/ftdx10` reproduces `exinventory_gen.go`
   byte-for-byte; the registry-driven staleness test covers it.
3. `dialecttest.Run` green over `ftdx10.Dialect()`; identity and
   difference pins green; goldens committed with the ASSUMED
   provenance header.
4. The three ledger closures land first, with guard red-proofs (the
   narrowed carve-out proven to fire on a decoy in the model package).
5. Full local gate green; FT-710 byte-identity manifest re-run
   (expected: trivially identical).
