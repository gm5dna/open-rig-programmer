# M9c-4 — the FTdx10 dialect and its Table 2 transcription

**Date:** 29/07/2026
**Status:** revision 2, review findings folded
**Milestone:** M9c-4, the first real second-model consumer of everything
M9c-0..3 built. M9c-5 (driver, fake, registration) follows.

> **Revision 2 (29/07/2026).** Revision 1 was reviewed adversarially by
> **Codex** (NEEDS-REVISION: 11 findings) and **Fable** (NEEDS-REVISION:
> 12 findings), adjudicated in
> `.superpowers/sdd/m9c4-spec-review-adjudication.md`. The headline:
> **revision 1's "P1=05 exists here" difference pin was factually
> wrong** — it laundered the manual's anomalous EX command header
> ("P1: 01-05") into fact, when the chart populates P1 01-04 only, the
> exact header-vs-chart anomaly the FT-710 has on record and its
> hardware refuted. Fable re-extracted the PDF with a second tool:
> **no EXTENSION group exists anywhere in the FTdx10 manual**, the
> chart's P3 tops out at 21 (the header says 23), and the chart holds
> **exactly 197 rows with exactly ONE text row** ("MY CALL." — trailing
> full stop verbatim). Also corrected: the transcription's common-mode
> failure (both reviewers), the semantic scope of transcription B
> (Codex, verified: `core/driver/ft710/settings.go:59,64,70` consumes
> P1Label/P2Label/Name as user-facing text — Fable's contrary sweep
> missed `core/driver`), the unimplementable both-profiles staleness
> claim (both), the incomplete profile literal (both), and the golden
> provenance rules (both). One Fable finding was REJECTED with
> evidence: new `core/cat` test files entail no golden re-mint (M9c-3
> added three; the golden has zero records for them; all green).
> **Do not implement revision 1.**

## What this milestone is

`core/cat/ftdx10` — a data-only model package holding the FTdx10's
dialect, built through `cat.MustNewDialect`, its EX inventory generated
from a committed Table 2 transcription through the M9c-2 extable
machinery, proven by the M9c-3 `dialecttest` conformance suite and by
manual-derived goldens whose assumptions are itemised. **No driver, no
fake, no registration, no wiring** — M9c-5's. (Verified safe: the
package cannot self-register — `SupportedModels()` derives solely from
`internal/wiring`'s driver table.)

Plus, first, **four guard/API closures** (the M9c-3 ledger's three, one
added by this review), done before the model package exists because
they constrain it:

1. **Narrow the Set-builder fence's carve-out** from the `core/cat/**`
   prefix to exactly `core/cat` + `core/cat/dialecttest` — so the model
   package is NOT exempt. Verified to break nothing (the only
   subpackage builder calls are dialecttest's). Red-proof: a transient
   non-test decoy in `core/cat/ftdx10` fires the fence.
2. **Pin "no production package imports `core/cat/dialecttest`"** as a
   third exact-import rule in
   `internal/guards/composition_imports_test.go` (whose walk already
   excludes `_test.go`, so ftdx10's test import stays legal). A
   forbidden-import rule has no legitimate positive site, so the
   red-proof must be explicit: a transient production decoy import,
   recorded failing. (Codex 8.)
3. **`CombinedMTSetKind` is a RENAME, not an alias.** The unexported
   `combinedMTSetKind` (`mtcombined.go:16`) becomes the exported
   `const CombinedMTSetKind byte = '0'`; internal uses are replaced; no
   synonym pair survives; it is NOT defined in terms of `KindVFO` (the
   byte equality is incidental across different command meanings — the
   definition must not encode the coincidence). A test asserts
   `CombinedMTSetKind == KindVFO` with exactly that caveat in its
   comment, and `dialecttest`'s `cat.KindVFO` call site + comment
   migrate to the new name. (Codex 9 + Fable 7, merged.)
4. **Extend the CAT-isolation guard's Rule 2 to the `core/cat` prefix**
   (`internal/guards/composition_imports_test.go` matches the import
   path exactly today, so `app/` importing `core/cat/ftdx10` — which
   drags `core/cat` transitively — would fire nothing). Same
   trailing-slash technique as Rule 1's F10 fix; own red-proof.
   (Fable 4.)

## The evidence base — chart-verified, header anomalies on record

All from CAT manual rev 2308-F (gitignored,
`docs/fixtures-private/manuals/`; layout text arbitrated by the PDF):

- **Table 2 spans layout lines 652-899** (page footers 9-12 contiguous
  — no dropped page). Rows are `P3 | Function | P4 | Digits`; the
  P1/P2 group labels FLOAT across row boundaries (e.g. "04 (DISPLAY
  SETTING)" printed five rows into its block), and at least one row
  name wraps with its P3 on the continuation line ("MOUSE POINTER /
  05 / SPEED"). These are the transcription's two known hazards.
- **The chart populates P1 01-04 only** (RADIO / CW / OPERATION /
  DISPLAY SETTING). **The EX command header says "P1: 01 - 05" and
  "P3: 01 - 23"; the chart refutes both** (no P1=05 group; P3 max 21).
  This is the FTdx10's own instance of the header-vs-chart anomaly the
  FT-710 carries (`core/cat/dialect.go:295-310` — its header said
  "01-04, 05", its chart {1,2,3,4,6}, its hardware rejected P1=05).
  Recorded in the CSV provenance header and `doc.go`, the FT-710's
  established style — NOT in a corrections file (that format is for
  hardware observations).
- **197 rows, counted twice independently by review** (per-block manual
  count and trailing-Digits-token scan, both 197: groups sum
  16+17+21+18+16+5+6 / 18+11+1 / 20+4+19+7+7 / 5+4+2). **Numeric
  Digits are 1-4 only; the single wider row is the single text row, MY
  CALL. at Digits 12** (layout 887, trailing full stop verbatim). So
  `MinDigits: 1, MaxDigits: 4, TextWidth: 12` are chart-verified, not
  inherited — with the contingency stated: if transcription finds a
  numeric row above 4, that is a PROFILE hypothesis failure (STOP,
  verify against the PDF, respec the profile and B's width encoding —
  extable refuses loudly at parse, which is the right failure mode).
- **CAT ID 0761** (layout 977). **MW Set P7 "0: (Fixed)"** →
  `MWWriteKind: cat.CombinedMTSetKind` after closure 3's rename (the
  byte '0', admitted by `validMWWriteKindByte`, verified; plan-review
  correction — this bullet previously spelt it `cat.KindVFO`). **MT combined, 41 bytes** → `MTAnswerBounds() == (41,41)`
  (chart- and code-verified).
- **The slot literal, stated exactly** (not "identical"): memory
  001-099, 60m 501-599, nine PMS pairs, EMG wire `"EMG"`, none wire
  `"000"`. **Two members are ASSUMED, not FTdx10-manual facts**
  (Fable 6): the **none wire "000"** appears in no FTdx10 slot legend
  (it is the FT-710's MR-answer fact), and **mode '0' (ModeUnset,
  "-")** appears in no FTdx10 mode legend (all run 1-F). Both are
  included — parsers must accept the placeholder, and a none-wire is
  structurally required — and both are marked ASSUMED in the config's
  comments and in the identity-pin tests, which therefore compare
  tables that embed two assumed members and say so.
- **Clarifier**: range 0000-9990 Hz appears in the MR/MT/MW legends AND
  the RD/RU command pages (layout 1507, 1605 — RT itself is bare
  on/off; revision 1 pointed at the wrong command). **No step is
  stated anywhere in the manual** (verified by both this spec's author
  and review). `ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990}` with
  StepHz **ASSUMED**, supported-not-proven by the range topping out at
  9990 (a 20 Hz radio would fail V10's multiple-of-step rule against
  9990; a 10 Hz one does not).
- **`TagFill: ' '` ASSUMED** (the FT-710 pads with spaces; the
  FTdx10's padding byte has never been observed).
- **Reused-command verification**: before the dialect reuses the
  FT-710 codec for MC, ID, AI, MR, MW and the EX read/answer grammar,
  the transcription task verifies each command's frame table against
  the manual (availability table at layout ~233; each command's
  section) and records it in `doc.go`. A deviation is a
  STOP-and-respec.

## Design

### The package

```
core/cat/ftdx10/
  doc.go             — provenance; header anomalies; ASSUMED register
  table2.csv         — transcription A (committed; provenance header)
  exinventory.go     — go:generate directive (-profile ftdx10)
  exinventory_gen.go — generated ([]cat.EXItem, alias-qualified)
  dialect.go         — the DialectConfig literal + Dialect() accessor
  ...tests + testdata/ (goldens, transcription B)
```

`func Dialect() cat.Dialect` returns a package-held value built by
`cat.MustNewDialect` at init. Every config field set explicitly,
including where zero is right. Mode table and slot space are FRESH
transcriptions typed into this package (cat's are unexported; a copy
error is caught by the identity pins).

### The extable profile — complete this time

```go
"ftdx10": {
    Model: "FTdx10", Package: "ftdx10",
    Types: TypesImported,
    ImportPath: "github.com/gm5dna/open-rig-programmer/core/cat",
    ImportAlias: "cat",
    VarName: "exItems", OutFile: "exinventory_gen.go",
    ManualCSV: "table2.csv",
    MinDigits: 1, MaxDigits: 4, TextWidth: 12,
    MaxObservedWidth: 12, // inert API-required sentinel under Absent;
                          // carries NO hardware claim (documented so)
    ExpectedRows: 197,    // from the group-boundary ledger, NOT from
                          // A/B agreement — see below
    Observations: ObservationsAbsent,
    DocLines: [...],      // names `go generate ./core/cat/ftdx10`
}
```

**Staleness coverage, scoped honestly** (revision 1's "one
registry-driven test covers both profiles" was unimplementable —
`Profile` carries no package-directory datum and paths are
cwd-relative): the ftdx10 package's own staleness test selects ONLY its
registration (via `Lookup`/`RegisteredProfiles` filtered to
`Package == "ftdx10"`) and verifies its local files; `core/cat`'s
existing FT-710 test is unchanged; together the two package-local tests
cover both profiles. A `Profile.Dir` field is deliberately NOT added.

### The transcription — symmetry-broken, semantically bound

The floating group labels admit a common-mode failure: two transcribers
reading the same layout text can misattribute a boundary block
IDENTICALLY, and address-set equality then confirms the shared mistake.
Nothing downstream catches it until Stage R. The design breaks the
symmetry three ways (both reviewers, Codex's stronger form adopted):

1. **Transcription A** (`table2.csv`, the extable 10-column shape) may
   be layout-text-led, PDF-checked.
2. **Transcription B is PDF-PRIMARY**: derived visually from the
   rendered PDF's ruled table cells — which resolve group boundaries
   unambiguously — by a DIFFERENT implementer in a separate session,
   without sight of A or its computed addresses. **B is SEMANTIC, the
   full tuple** `(P1,P2,P3, P1Label, P2Label, Name, Digits, Text)` —
   not widths alone — because `EXItem`'s labels and names become
   user-facing menu text through the settings descriptor
   (`core/driver/ft710/settings.go:59,64,70`; the FTdx10 driver will
   mirror it). P4 remains single-sourced in A as audit trail, its
   accepted residual recorded.
3. **The group-boundary ledger**, committed alongside: every P1/P2
   group's label, first and last (P3, name), row count, PDF page, and
   visual boundary anchor — produced from the PDF before
   reconciliation. Every floating transition is reconciled against the
   ledger EVEN WHEN A and B agree; if the rendered PDF itself is
   ambiguous anywhere, STOP — no address is chosen by consensus.

**The cross-check test** binds A ≡ B on the full tuple, exactly, and
binds both to the ledger's per-group cardinalities. `ExpectedRows: 197`
is set from the ledger's counts (which review has already twice
independently derived), NOT from A/B agreement — the gate freezes an
externally established count; it does not create it. If A and B agree
on a number ≠ the ledger's, that is a STOP-and-arbitrate against the
PDF, not an ExpectedRows edit.

**B's afterlife** (Fable 8): B lives in ftdx10's `_test.go`/testdata
for M9c-4. At M9c-5, the fake's compact widths inventory is derived as
a PROJECTION of B and moves wholesale into `fakedx10`'s own production
code (preserving the fake's no-cat-imports rule), with the cross-check
re-pointed at it — the fakeradio/transport precedent.

### Goldens — assumption-itemised, separate-session derivation

`core/cat/ftdx10/testdata/` gets its golden corpus with the
never-regenerate discipline from birth. The derivation is performed in
a SEPARATE SESSION directly from manual rev 2308-F's position charts
(the MT **Answer** chart, layout 1237-1251, is the clean one — the Set
chart interleaves with its legend; the header names which was used),
without consulting `mtcombined.go`, generated output, or this spec's
computed offsets. The provenance header records, per vector class:
manual revision + page/table + direction; derivation session/date; the
no-code-consulted statement; which bytes are manual-documented and
which are INHERITED ASSUMPTIONS (space fill, exact answer width, 10 Hz
clarifier step); hardware status UNVERIFIED; and the exact Stage R
capture that would lift each assumption individually. These goldens
are frozen regression oracles for the project's manual interpretation
— never represented as hardware truth, and Stage R lifts claims
one by one, not the corpus wholesale. (Codex 10 + Fable 11.)

### Proof obligations

- `dialecttest.Run(t, ftdx10.Dialect())`. (`RunZeroValue` is
  universal — not re-run per model.)
- **Identity pins, precisely specified** (Codex 7): modes — exhaust
  all 256 bytes; compare `ValidMode` verdicts with `cat.FT710`'s;
  compare `ModeName` for valid modes; `ModeByName` round-trips; the
  two ASSUMED members noted. Slots — behavioural comparison over the
  wire space (`ParseSlot` over 000-999 and the PMS/EMG wire forms,
  acceptance and `Wire()` equality against `cat.FT710`), plus
  constructor acceptance (`MemorySlot`, `PMSSlot`, `SixtyMSlot`,
  `EMGSlot`); **`Slot.IsXxx` predicates are NOT used** (FT-710-scoped
  by the ledgered M9b deferral); no `SlotSpace()` accessor is added.
- **Difference pins, chart-true this time**: `CATID() == "0761"`;
  `MWWriteKind() == CombinedMTSetKind` (asserted as the FTdx10's fact
  with the incidental-equality caveat); `MTForm() == MTFormCombined`;
  `MTAnswerBounds() == (41,41)`; EX disjointness in BOTH directions —
  `KnownEXAddress(010601)`/(010701) true here (ENC/DEC PSK / RTTY:
  P2 06-07, beyond the FT-710's P2 max of 05) and false on `cat.FT710`;
  `KnownEXAddress(060101)` false here and true there (the FT-710's
  EXTENSION/PRESET group, absent from the FTdx10).
- **EX answer bound** (Codex 11): a test computes `max(Digits)` from
  `Dialect().EXItems()` independently, proves 12 (MY CALL), accepts a
  known-address EX answer with a 12-byte P4 through `ParseEXAnswer`,
  and rejects the identical answer at 13 bytes. The text-row set
  (exactly {MY CALL.}) is pinned by the cross-check.
- The four closures' red-proofs (above).
- FT-710 byte identity: nothing here touches an FT-710 path, but the
  full byte-identity gate runs anyway (M9c-1 method, cheap insurance).
  New `core/cat` test files (the rename's test) carry no
  evidence-golden records and entail NO re-mint — the M9c-3 precedent;
  the two-commit rule stands.

## Out of scope, and recorded

`core/driver/ftdx10`, `internal/fakedx10`, wiring/CLI/GUI
registration, radiotext prose, preconditions 8-11, TagDisplay
representability, write choreography (all M9c-5, ledgered); any
hardware claim (Stage R); FTdx101D/MP and FTX-1 (after M9c closes, per
Stuart's standing instruction).

## Acceptance

1. The group-boundary ledger committed; A ≡ B on the full tuple; both
   ≡ ledger cardinalities; `ExpectedRows` = the ledger's 197 (or the
   STOP was taken and the spec revised).
2. `go generate ./core/cat/ftdx10` reproduces `exinventory_gen.go`
   byte-for-byte; the package-local staleness test covers it; the
   FT-710's staleness test unchanged.
3. `dialecttest.Run` green; identity pins (with ASSUMED annotations)
   and chart-true difference pins green; the EX answer-bound test
   green; goldens committed with the itemised provenance header.
4. The four closures land first, each with its red-proof recorded.
5. Full local gate green; FT-710 byte-identity manifest re-run
   (expected: trivially identical).
