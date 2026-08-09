# M9d-2 capability matrix — the FTdx101D and the FTdx101MP

Originally dated 08/08/2026. **THIS IS REVISION 4 (09/08/2026)** — rev
3's three numbered errata stand at **§6**, none renumbered. Rev 4
re-points §1.3.5's citations by symbol, corrects one span bound,
re-wraps one paragraph, and appends THREE tagged notes inside erratum 3:
a navigational grep warning, a discharge marker, and one
**erratum-weight correction** to a rev-3 sentence that implied a
citation drift which did not occur. No capability value, `FieldSupport`
cell, evidence grade, layout citation or count moves anywhere in rev 4.
Status: pre-plan artefact, since CONSUMED by the executed M9d-2 plan.
Milestone: M9d-2 (`core/driver/ftdx101`, `internal/fakedx101`,
registration).

**Revisions.** Rev 1, 08/08/2026 (`432fd4e`, as written). Rev 2,
08/08/2026 (`2745e14`, review fix round 1 — the misquote, the citations,
the sixth slot legend). Rev 3, 09/08/2026 (§6's three errata) **(rev 4:
this entry read "(this one; §6)", which only rev 3 could say)**. Rev 4,
09/08/2026 (this one): §1.3.5's neighbour citations re-pointed BY SYMBOL
— the fold-in erratum 3 leaves to "a future revision" — the stale
`mtSlotValid` doc-comment span bound corrected, erratum 3 given the
grep-trap warning the frozen `core/cat/mw.go` literal needs, and one
§3.6 paragraph re-wrapped. Those four were fixed as the standing REV-4
LIST at the follow-ups wave's closing dual review
(`.superpowers/sdd/m9d-followups-ledger.md`, closing-review paragraph).
Rev 4's own fix round added a fifth item its review turned up — erratum
3's disclosure of the neighbour drift is half wrong, and rev 4 corrects
it at erratum weight in place
(`.superpowers/sdd/m9d-minors-task2-review.md`, findings F1 and F2).

Because this document was consumed by an executed plan, **nothing here
is silently rewritten**: every correction after rev 2 is recorded, at a
weight set by what it does to a reader. A correction that SUPERSEDES AN
ASSERTION ALREADY CONSUMED OR SHIPPED carries the full erratum
apparatus — what stood, what now stands, and the record that
adjudicated it — and every site it corrects carries a tag pointing
back; rev 3's three are numbered in §6 and tagged `(rev 3 erratum N)`,
and rev 4's one is recorded at the site it corrects, inside erratum 3,
for the reason given there. The two conditions are distinct and NEITHER
ALONE COVERS BOTH CASES: rev 3 corrected rev-1/2 text that an executed
plan had CONSUMED though nothing had yet shipped (rev 3, `36e1763`, is
an ancestor of v1.0.0, `2d3b03a`), whilst rev 4 corrects rev-3 text
that has SHIPPED though no plan consumed it. A correction that
supersedes NO assertion — a pointer to unmoved code, a pure addition, a
re-wrap — is recorded in this paragraph with a `(rev 4)` tag at the
site, which carries the number or wording it replaced; a correction to
a revision's OWN in-flight text, before it is either consumed or
shipped, needs NEITHER — which is why rev 4's fix rounds amend rev 4's
own tags freely. **(rev 4: this rule read "every correction after rev 2
is an erratum in §6"; rev 4 grades it by whether the superseded
assertion had already been consumed or shipped, and argues the grading
where it applies it.)** The one untagged edit is the §3.6 re-wrap,
which changed no word and so has nothing to point back at; it is named
here instead.

## What this is

This document is the per-model evidence/assumption matrix the M9d
spec (rev 2) requires to exist before the M9d-2 plan may be written:

> **(Rev 2, A4.) The capability matrix comes FIRST.** Before the
> M9d-2 plan is written, a per-model evidence/assumption matrix over
> EVERY `spec.Capabilities` field (and the framing/control-line/
> discovery behaviours no field expresses): each entry either
> MANUAL-EVIDENCED with its layout citation, or ASSUMED with its
> register entry and named Stage R lift. Capabilities are never
> evidence for themselves; tests derived from them pin choices, not
> facts.
> — `docs/superpowers/specs/2026-08-05-m9d-ftdx101-design.md`, M9d-2

It is what the spec's Testing section means by "bank core fields
confirmed from the capability MATRIX (A4)". It is a *pre-plan*
artefact: it describes what the M9d-2 driver will carry and why, so
that the plan can be written against evidence rather than against the
FTdx10 driver's shape.

**NO FTdx101 OF EITHER MODEL HAS EVER BEEN ASKED ANYTHING BY THIS
PROJECT.** There is no `docs/hardware-notes.md` section for either
radio, no captured frame from either, and no Stage R or Stage W
session. Every "value" column below is what the driver will *claim*,
and every claim is either a reading of a manual or a written-down
assumption. Neither model has a hardware-verified capability profile
and neither will have one at M9d-2's close.

## Sources and their status

- **Manual.** Yaesu FTDX101MP/FTDX101D CAT Operation Reference Manual,
  edition **2308-L**, 26 pages
  (`docs/fixtures-private/manuals/ftdx101_cat_2308-L.pdf`, gitignored —
  Yaesu copyright). SHA-256
  `4f54e60a…d81e8bc5`, recorded in full at
  `docs/fixtures-private/manuals/m9d-manual-provenance.md` and verified
  against the file on 08/08/2026 before this matrix was written.
  Citations of the form "layout N" are line numbers in the
  layout-preserved extraction `ftdx101_layout.txt` (2,051 lines,
  `pdftotext -layout`, same directory, also gitignored). Both files are
  gitignored, so these are citations, not links; "printed page N" is
  the manual's own folio where it is useful.
- **Committed FTdx101 evidence.** `core/cat/ftdx101/` — `doc.go` (the
  provenance record, the three model distinctions, the chart-defect
  record, the seven-entry ASSUMED register, and the reused-command
  verification verdict), `table2.csv` and its generated
  `exinventory_gen.go`, `testdata/geometry-witness.csv` and `.md`,
  `testdata/group-ledger.csv` and `.md`, `testdata/provenance.md`.
  Each of these carries its own layout citations, so a citation to one
  is MANUAL-EVIDENCED; where this matrix cites one it cites the
  artefact **and** the layout lines underneath it.
- **Shape precedents, NEVER evidence.** `core/driver/ftdx10/caps.go`
  and `doc.go`, `core/driver/ft710/caps.go`. A value that appears in
  one of those is a *structural* precedent for how the FTdx101 driver
  should be written; it is never evidence that an FTdx101 behaves that
  way. Where an inherited value's honest status is ASSUMED, this matrix
  says ASSUMED even though the FTdx10 carries the identical value with
  the identical status.
- **The spec.** Where the spec fixes a CHOICE — the discovery budget
  shape, MT-only writes, `TagDisplay` `Unavailable`, `writeTrialsComplete`
  false — the entry records the choice *and* its separate evidence
  status. The spec fixing a choice is not evidence that the radio
  behaves that way.

## How to read an entry

Every entry carries three things: the **value** the M9d-2 driver will
carry, stated for the **D and the MP separately**; the **status**; and
either a citation or a register home plus a named lift.

- **MANUAL-EVIDENCED** — the value is what this manual says, with the
  layout line(s) to check it against.
- **ASSUMED** — the value is inherited or structurally required and
  this manual does not yield it. Every ASSUMED entry names (a) its
  **register home** — either an existing entry of the DIALECT register
  (`core/cat/ftdx101/doc.go`, seven entries, cited by entry name) or a
  named entry of the **M9d-2 DRIVER register**, which does not exist
  yet and is to be created at M9d-2 (`core/driver/ftdx10/doc.go`'s
  nine-entry register is the shape precedent) — and (b) the **ONE**
  named Stage R or Stage W capture that lifts it, **per model**.
- **CHOICE** appears as a qualifier inside an entry, never as a status
  on its own: it marks a project decision (a display label, an
  ordering, a conservative policy) that is not a claim about the radio
  at all. Every CHOICE entry still states the evidence status of the
  radio-facing claim, if any, underneath it.

**Per-model discipline.** A capture taken from an FTDX101D lifts the
D's entry only. The two models share a manual, not a serial port. Where
one value serves both models, the entry says *why* that is safe: either
the applicability attestation for Table 2 material
(`core/cat/ftdx101/testdata/group-ledger.md`, "NO row's stored
properties are model-conditional"), or the specific unconditional
manual text. §4 shows the whole of the model-distinguishing surface and
which of it touches a capability value.

**No value legends are interpreted.** M9d-1's boundary stands: this
matrix models capability values, not menu-value semantics.

---

## 1. `spec.Capabilities`, field by field

`core/spec/capabilities.go` declares **fifteen** fields. All fifteen
appear below, by name, in struct order. All fifteen will be populated
EXPLICITLY in the driver (the zero-value class the spec names): a zero
left in a capability field is not a neutral omission — a zero
`MaxFreqHz` reads as "no ceiling" to every validator
(`core/spec/validate.go:123-125`), a zero `TagLen` makes `core/csvio`'s
CHIRP import truncate every imported name to `b[:caps.TagLen]` and so
silently discard it, reported as an approximated loss rather than
refused (`validate.go:178-184`), and a NON-POSITIVE entry in `Bauds`
would reach `SerialConfig.Baud`, which `transport.OpenSerial` treats as
"unset" and silently replaces with its own `DefaultBaud` of 38400
(`validate.go:126-131`).

`Bauds` and `DefaultBaud` are worth stating precisely, because the
hazard is not the one it looks like: an EMPTY `Bauds` is not a silent
guess — it fails `spec.Validate` loudly, on the "DefaultBaud must appear
in Bauds" rule (`validate.go:249-251`), since a positive `DefaultBaud`
cannot be a member of an empty list. It is the non-positive entry, not
the empty list, that `Validate` exists to catch before the transport
layer substitutes for it.

### 1.1 `Model string`

| | Value |
|---|---|
| D | `FTdx101D` |
| MP | `FTdx101MP` |

**Status: CHOICE over a MANUAL-EVIDENCED fact.** That two distinct
models exist and are named `FTDX101D` and `FTDX101MP` is a manual fact
(layout 1070 and 1072, ID's P1 legend, printed page 14; also the cover,
and `core/cat/ftdx101/testdata/geometry-witness.md` §Source). The
project's **spelling** of the registry key is a choice, fixed by the
spec ("producing registered models `FTdx101D` and `FTdx101MP`") and not
by the manual, which sets both names in full capitals throughout. The
precedent is the FTdx10, whose `FTdx10` spelling was likewise a project
key and whose near-miss `FT-DX10` is deliberately kept unknown by
`internal/radiotext`'s own test. `internal/extable`'s profile already
carries the joint form `FTdx101D/MP` for the shared inventory
(`internal/extable/profile.go:370`); that is the *inventory's* model
string, not a driver registry key, and M9d-2 introduces the two
per-model keys separately.

**No divergence risk:** `Model` must equal the registry key and must
differ between the two registrations, so it is model-conditional by
construction.

### 1.2 `CATID string`

| | Value |
|---|---|
| D | `0681` |
| MP | `0682` |

**Status: MANUAL-EVIDENCED. THIS IS A GENUINE D-vs-MP DIVERGENCE.**
ID's P1 legend prints "0681: FTDX101D" and "0682: FTDX101MP" (layout
1070 and 1072, printed page 14; ID's frame block 1069-1078, its
availability row X O O X at layout 304). This is the first of the three
places the manual distinguishes the models (§4), and it is the one this
project's *dialect data* already carries: `core/cat/ftdx101/dialect.go`
builds two instances over one config for exactly this reason.

The driver sources the value from the dialect rather than restating it
(`catDialect.CATID()`, the FTdx10 driver's shape at `caps.go:71`), so
the string the ID probe compares against is the string the capability
data advertises.

### 1.3 `Banks []Bank`

| | Value (static baseline) |
|---|---|
| D | MEM `001`..`099` (99 slots); PMS `P1L`..`P9U` (18 slots) |
| MP | identical |

Plus, per session, up to two DISCOVERED read-only banks: 60M and EMG
(§1.3.4 and §3.4). No 5xx or EMG bank is asserted statically.

**One value serves both models because the memory-channel surface is
printed once, unconditionally, for both.** This manual prints **SIX**
slot legends, and none carries a model qualifier: MC's P1 (layout
1225-1227), IF's P0 (1082-1083), MR's P0 (1278-1279), MT's P0/1
(1312-1313), MW's P1 (1353) and **OI's P1 (1436-1437)**. §4's sweep
shows the manual's only three model-conditional places, and none of them
is a slot legend.

**Six, not five.** `core/cat/ftdx101/doc.go`'s `SlotSpace.NoneWire`
entry enumerates the first five, because those are the five that entry
needs; OI's is a sixth legend of the same kind — the same parameter
class as IF's P0, which that enumeration does count — carrying the full
"001-099 (Memory Channel), P1L -P9U (PMS), 5xx (5MHz BAND), EMG
(EMERGENCY CH)" vocabulary. It is named here so that a later reader
running the sweep finds six and does not conclude one of the two records
is wrong. **No conclusion below changes:** OI's legend contains no QMB
form (so §1.3.3's absence stands) and it includes 5xx and EMG (so
§1.3.4's "MW alone excludes them" stands, and more strongly — MW is one
legend out of six, not one out of five).

#### 1.3.1 MEM bank

- **Slots `001`..`099`: MANUAL-EVIDENCED.** "001-099 (Memory Channel)"
  in all six slot legends above. The driver builds them through the
  dialect's own `MemorySlot`, walking until it refuses, so the advertised
  wire forms are exactly those the dialect's slot space accepts
  (`MemoryLo: 1, MemoryHi: 99`, `core/cat/ftdx101/dialect.go:95`), never
  a locally formatted string `ParseSlot` might later reject.
- **`Label: "Memories"`: CHOICE.** A display string; not a protocol
  fact.
- **`NoBlank: false`, stated explicitly: CHOICE, conservative.** Nothing
  in this manual says a memory channel must stay populated. A `NoBlank`
  MEM bank would make `codeplug.Validate` refuse every candidate with a
  single blank channel. The one channel this driver claims must stay
  populated is `RequiredSlots`' `001` (§1.13), which is the per-slot
  mechanism, not the per-bank one.

#### 1.3.2 PMS bank

- **Slots `P1L`..`P9U` (nine pairs, eighteen slots): MANUAL-EVIDENCED.**
  "P1L -P9U (PMS)" in the same six legends (the chart sets the MC
  legend with letter tracking, which the extraction renders as
  spaced-out characters — `core/cat/ftdx101/doc.go` records that it was
  read from the rendered page instead). Built through the dialect's
  `PMSSlot` walking until refusal (`PMSPairs: 9`, dialect.go:100).
- **`Label: "Scan limits (PMS)"`: CHOICE.**
- **`NoBlank: false`, stated explicitly: CHOICE, conservative, and
  deliberately NOT the FT-710's original guess.** Nothing establishes
  that an FTdx101 ships with its PMS pairs populated. The FT-710's own
  `NoBlank` PMS bank was REMOVED at M5b for exactly the failure a wrong
  guess here causes: real radios shipped all-PMS-empty, so
  `codeplug.Validate` rejected every real-derived candidate before
  `Diff` ever ran, including MEM-only edits. A populated PMS slot going
  back to empty stays blocked regardless, by `FieldErase` never being
  writable (§2.3).

#### 1.3.3 Slot families this project does NOT model as banks

Recorded so that a later reader does not mistake the omission for an
oversight:

- **QMB (Quick Memory Bank).** The radio has one — `QI` (QMB STORE) and
  `QR` (QMB RECALL) are in the availability table (layout 256-257), the
  menu item (03,01,14) `QMB CH` selects 5 or 10 channels (layout 867),
  and IF's and OI's P7 legends name it as a *source state*, "3: Quick
  Memory Bank (QMB)" (layout 1092-1093 and 1447-1448). It appears in
  **no** slot legend: all six — MC, IF, MR, MT, MW and OI — address only
  001-099, P1L-P9U and (MW excepted) 5xx and EMG, and `QI`/`QR` take no
  slot parameter at all.
  **MANUAL-EVIDENCED absence:** there is no CAT slot form for a QMB
  channel, so there is nothing for a bank to enumerate, and no way to
  read or write one.
- **`MEM GROUP` (03,01,15, layout 869)** is a front-panel grouping
  toggle over the same 001-099 space, not a separate slot family.

#### 1.3.4 The DISCOVERED banks (60M and EMG)

- **Existence of a 5xx family and of EMG: MANUAL-EVIDENCED.** "5xx
  (5MHz BAND), EMG (EMERGENCY CH)" appears in **five of the six** slot
  legends — MC (1225-1227), IF (1082-1083), MR (1278-1279), MT
  (1312-1313) and OI (1436-1437) — and NOT in MW's (1353), which is the
  same MW restriction the FT-710 and the FTdx10 carry.
- **The numbering 501..599: ASSUMED.** The legends say only "5xx (5MHz
  BAND)"; the start at 501 rather than 500, the ceiling at 599 and
  therefore the channel count are interpretation.
  **Register home:** DIALECT register, entry *"SlotSpace.SixtyLo/SixtyHi
  = 501/599"* (`core/cat/ftdx101/doc.go`).
  **Lift, per model:** an enumeration of the 5xx range INCLUDING 500 —
  which wire numbers answer as populated-or-empty channels and which
  answer "?;" fixes the real bounds. **The dialect register words this as
  an MR enumeration**, because a dialect describes the radio's protocol
  and MR is the natural read for it; **§3.8.1 words its own lift as an
  MT enumeration**, because the driver's discovery walk sends MT and
  never MR (§3.5). *Either command's enumeration retires this entry* —
  what the capture must establish is which wire numbers in and around
  5xx answer at all, and both commands ask that question. See §3.8.1 for
  the relationship between the two register entries one such session can
  retire together.
- **The EMG wire form `EMG`: MANUAL-EVIDENCED** (the same legends).
- **Both discovered banks `NoBlank: true`: CHOICE, and a statement about
  the PROTOCOL surface this project offers, not about the radio's
  factory contents.** Those channels exist in a session because they
  answered a read, and this driver offers no way to blank them: no erase
  command exists in this radio's command set (§2.3), and `core/cat`
  refuses 5xx/EMG slots in both write builders (§1.3.5).
- **Both discovered banks read-only (every `Write` forced
  `Unsupported`): CHOICE, and see §1.3.5 for the precision this needs.**
- **Labels (`60 m channels`, `Emergency (EMG)`): CHOICE.** The manual's
  own words are "5xx (5MHz BAND)" and "EMG (EMERGENCY CH)"; "60 m" is
  the band's amateur name.

#### 1.3.5 A precision about 5xx/EMG writability

`core/cat`'s combined-MT write policy refuses 5xx and EMG slots — the
predicate is `Dialect.mtSlotValid` (`core/cat/mt.go`, cited by symbol),
reached by `validateCombinedMTFields` (`core/cat/mtcombined.go`, cited
by symbol) **(rev 4: both read as line numbers, 115 and 105. Only
`mtSlotValid` drifted, 115 → 118 at M9d follow-up 2 `706f680`.
`core/cat/mtcombined.go:105` never moved: it is the
`if !d.mtSlotValid(m.Slot) {` line at which that function REACHES the
predicate, which is what the sentence cited it for, and it was exact at
rev 1 and is exact today. Erratum 3's "cited as 105" measured it
against the DECLARATION at `:104` and read a drift into it — corrected
at erratum weight in §6. Both are by symbol now, so the question cannot
arise again.)** — and `cat.Dialect.writableSlot`
(`core/cat/slot.go`, cited by symbol), consulted by `validateMWFields`
(`core/cat/mw.go`), excludes them from MW **(rev 3 erratum 3: this read
`cat.Slot.Writable`, a symbol since DELETED)**. **The MT half is a
PROJECT POLICY, and `core/cat` says so in terms.** `mtSlotValid`'s own
doc comment runs `core/cat/mt.go:100-117` **(rev 4: this read
`:100-108`, which was already six lines short WHEN REV 1 WROTE IT — at
`432fd4e` the comment ran `:100-114`, `:108` ending its second
paragraph and `:110-114` being a third the bound excluded — and M9d
follow-up 2 `706f680`, the only commit to touch this file since, then
grew it to `:117`. A rev-1 mis-measurement, later widened; not a drift.
The quoted `:103-106` opens that second paragraph (`:103-108`) and is
unmoved throughout, re-verified byte-exact)**; its middle
(`:103-106`) is the statement:

> The manual's slot table marks 5xx and EMG as ✓ for MT — but reference
> §MT states explicitly: "our policy: reject sets to 5xx/EMG until
> hardware-verified" (a project decision, not a manual requirement,
> repeated verbatim in the Task 3 brief).

The rejection a caller actually sees says the same thing more tersely —
"MT: slot must be memory (001-099) or PMS (P1L-P9U); 5xx/EMG rejected by
project policy pending M5a, \"000\"/invalid rejected per reference"
(`validateCombinedMTFields`' own `newParseError`) — and the comment
above the validator calls it "5xx/EMG refused by project decision
pending hardware verification" (that function's doc comment, first
bullet) **(rev 4: these read `core/cat/mtcombined.go:106` and
`:85-86`; both numbers were still current when re-measured, and are
re-pointed by symbol so that neither can drift)**.

**Attribution matters in that quote:** the "manual" and the "reference
§MT" `mt.go` speaks of are the **FT-710's**, because that is the radio
`core/cat` was written for and the policy was adopted against. The
FTdx101's manual is a second, independent document that happens to say
the same thing about MT's slot vocabulary — which is what makes the
policy's project-decision status carry across rather than needing to be
re-established.

**What the FTdx101's own manual says.** Its MT slot legend carries the
full vocabulary, "001-099 (Memory Channel), P1L -P9U (PMS), 5xx (5MHz
BAND), EMG (EMERGENCY CH)" (layout 1312-1313), against MW's restricted
"001-099 (Memory Channel), P1L -P9U (PMS)" (layout 1353). **One degree
of hedging is owed here:** that legend is headed **"P0/1"**, merging the
Read direction's slot parameter (P0) with the Set direction's (P1) under
one vocabulary, so the manual does not separately state that an MT
**Set** may address 5xx or EMG — it states it of MT's slot parameter
generally. That is weaker than "the manual permits MT Sets to 5xx", and
it is deliberately all this matrix claims. `mtSlotValid`'s doc comment
concedes the same reading for the FT-710 — "the manual's slot table
marks 5xx and EMG as ✓ for MT", the first line of the paragraph quoted
above, a slot-table fact, not a Set-direction one **(rev 4: this cited
`core/cat/mt.go:103`, still current when re-measured, and is now
pointed by symbol)**. So:

- "MW cannot address 5xx/EMG" is MANUAL-EVIDENCED **for this radio**
  (layout 1353, the FTdx101's own MW legend, unambiguously the Set
  direction's P1).
- "MT cannot address 5xx/EMG" is **not** a manual fact for this radio;
  it is this project's conservative policy, adopted for the FT-710 and
  inherited here, and nothing in the FTdx101's manual requires it.

The read-only discovered banks are therefore correct and conservative,
but the *reason* must be stated as policy. The FTdx10 driver's
`readOnlyFields` comment compresses the two into one clause; M9d-2 must
not inherit that compression.

### 1.4 `Modes []string`

| | Value |
|---|---|
| D | `LSB USB CW-U FM AM RTTY-L CW-L DATA-L RTTY-U DATA-FM FM-N DATA-U AM-N PSK DATA-FM-N` (15, wire-code order '1'..'F') |
| MP | identical |

**Status: MANUAL-EVIDENCED.** The mode legend is printed beside FIVE
commands in this manual, all five identical and all five running 1 to F
with no other member: MD's P2 (layout 1240-1243), IF's P6 (1089-1091),
MR's P6 (1286-1288), MT's P6 (1321-1323), MW's P6 (1361-1363). None
carries a model qualifier. `core/cat/ftdx101/dialect.go:42-65`
transcribed them fresh from this manual rather than copying the
FT-710's or the FTdx10's table.

**A sixth legend exists and is NOT sourced from.** OI's P6 (layout
1443-1446) misnumbers its last two members, "D: AM-N **E: PSK E:**
DATA-FM-N" — a duplicated "E:" prefix where "F:" belongs. Recorded as a
printing defect in `core/cat/ftdx101/doc.go` and excluded from sourcing
rather than reconciled.

**Exclusion of `cat.ModeUnset` ('0', "-") from the advertised list:
CHOICE**, and the right one — it is a parse-accept-only placeholder that
core/cat refuses to emit in any Set frame, so offering it as a
selectable mode would invite a user to write a value the codec will
reject. Its *presence in the dialect's table* is separately ASSUMED:

- **Register home:** DIALECT register, entry *"the `cat.ModeUnset`
  member of the mode table"*.
- **Lift, per model:** one MR read of an EMPTY memory channel, one that
  radio has never had written to; the P6 byte of that answer is what it
  says for "no mode".

The driver derives the names by enumerating the dialect (the FTdx10
driver's `modeNames()` shape) rather than keeping a local table, so a
transcription drift between driver and dialect is unrepresentable
rather than merely tested.

### 1.5 `TagLen int`

| | Value |
|---|---|
| D | `12` |
| MP | `12` |

**Status: MANUAL-EVIDENCED.** MT's P12 legend: "TAG Characters (up to
12 characters) (ASCII)" (layout 1330, printed page 16), unconditional
for both models. The geometry witness independently confirms the field
occupies positions 29-40 — twelve — counted off 300 dpi raster renders
with no access to the layout text
(`core/cat/ftdx101/testdata/geometry-witness.csv`, rows
`MT,set,P12,29,40` and `MT,answer,P12,29,40`), and `geometry_test.go`
binds that count to the dialect. `MTPolicy.TagMaxBytes: 12`
(dialect.go:107) matches.

**The WIDTH is manual-evidenced; the FILL BYTE is not.** The byte the
radio pads a short tag with is separately ASSUMED:

- **Register home:** DIALECT register, entry *"MTPolicy.TagFill = ' '"*.
- **Lift, per model:** one MT Set of a tag SHORTER than 12 characters to
  a memory channel, then an MT read of that channel — the bytes the
  radio returns after the written characters ARE the fill.

`TagLen` itself does not depend on the fill byte, so this entry's status
is MANUAL-EVIDENCED without qualification.

### 1.6 `ClarMaxHz int`

| | Value |
|---|---|
| D | `9990` |
| MP | `9990` |

**Status: MANUAL-EVIDENCED.** "Clarifier Offset: 0000 - 9990 (Hz)" is
printed in five frame blocks — IF (layout 1086), MR (1282), MT (1317),
MW (1357), OI (1440) — and the two dedicated clarifier commands agree:
RD (CLAR DOWN) P1 "0000 - 9990 (Hz)" at layout 1602, and RU (RX
CLARIFIER PLUS OFFSET) P1 the same at layout 1700. Seven unconditional
statements, no model qualifier on any of them.
`ClarifierPolicy.MaxAbsHz: 9990` (dialect.go:114).

### 1.7 `ClarStepHz int`

| | Value |
|---|---|
| D | `10` |
| MP | `10` |

**Status: ASSUMED.** NO step is stated anywhere in this manual. The
0000-9990 range that all seven citations in §1.6 agree on *supports* the
inherited value without proving it: a 20 Hz radio could not reach its own
stated 9990, a 10 Hz one can, and a 1 Hz one would be free to stop at
9999 and does not.

- **Register home:** DIALECT register, entry *"ClarifierPolicy.StepHz =
  10"*. Cited here, deliberately not re-registered as the driver's own —
  correcting it is a dialect change.
- **Lift, per model:** one MW Set carrying a clarifier offset that is NOT
  a multiple of 10 — 0005 Hz — followed by an MR read of the same
  channel. A radio answering 0005 has a finer step and the value drops
  for that model; a radio answering 0000 or 0010 has quantised, and 10 is
  confirmed for it at that resolution.

`core/cat` enforces this as a multiple-of-step rule on every MW and
combined-MT Set, so the value is load-bearing on the write path even
while nothing may be written.

### 1.8 `CTCSSTones []Tone`

| | Value |
|---|---|
| D | `spec.StandardCTCSSTones()` — the 50-tone chart, indices 000-049, 67.0-254.1 Hz |
| MP | identical |

**Status: MANUAL-EVIDENCED.** This manual prints its own "Table 1
(CTCSS Tone Chart)" (heading at layout 566, body 567-575, printed page
8), reached from CN's P3 legend "000 - 049: Tone Frequency Number (See
Table 1)" (layout 559). The chart was compared **element by element,
all fifty**, against `spec.standardCTCSSTones`
(`core/spec/tones.go:39-46`) while writing this matrix: every index
000-049 agrees, 67.0 Hz at 000 through 254.1 Hz at 049, with no
insertion, omission or reordering. No model qualifier appears on the
chart or on CN's frame block.

The index convention is the same one `core/spec` documents: the array
index IS the CAT tone number P3.

**A separate question, deliberately not conflated:** whether a memory
channel's tone NUMBER is reachable over CAT at all on this radio. It is
not — see §2.2 — and that is why `FieldCTCSSTone` is the zero
`FieldSupport` in every bank. `CTCSSTones` describes the vocabulary the
radio's tone chart uses; it does not claim the memory record can carry
a tone number.

### 1.9 `Bauds []int`

| | Value |
|---|---|
| D | `{4800, 9600, 19200, 38400}` |
| MP | identical |

**Status: MANUAL-EVIDENCED.** Menu item (03,01,11) `CAT RATE`, P4
legend "0: 4800 bps 1: 9600 bps 2: 19200 bps 3: 38400 bps" — layout
**863**, printed page 11; the row is committed at
`core/cat/ftdx101/table2.csv:253` and generated into
`core/cat/ftdx101/exinventory_gen.go`. Four rates, **no 115200** — the
same divergence from the FT-710's five that the FTdx10 shows, but
established here from the FTdx101's own chart. The Table 2 applicability
sweep attests that no row's stored properties are model-conditional
(`core/cat/ftdx101/testdata/group-ledger.md` §Attestation), and this row
carries no model qualifier in its P4 legend either (§4).

**Which port these rates describe.** This radio exposes CAT over two
physical paths, and the chart has a separate rate menu for each:
(03,01,09) `232C RATE` for the rear RS-232C jack (layout 861) and
(03,01,11) `CAT RATE` (layout 863). The two legends print the same four
rates. `Bauds` describes the **CAT** menu, which is the USB/virtual-COM
path this project opens; see §3.12 for what else differs between the two
paths.

### 1.10 `DefaultBaud int`

| | Value |
|---|---|
| D | `38400` |
| MP | `38400` |

**Status: ASSUMED.** **The factory default is not in this CAT manual at
all.** Table 2's column headers are "P1 P2 P3 Function P4 Digits"
(layout 716, printed page 10) — there is no factory-default column, and
the trailing `1` on layout 863 is the **Digits** field, which is exactly
how the committed inventory reads it (`table2.csv:253`, `digits=1`).
This is the misreading the FTdx10 spec made once and recorded so it
could not recur; it is recorded again here for the same reason.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"DefaultBaud 38400"**.
- **Lift, per model:** the baud a FACTORY-CONFIGURED radio of that model
  answers an ID exchange at — try 38400 first, then the other three in
  turn. The answering rate is the fact; a radio whose CAT RATE has been
  changed by its owner cannot settle it, so the capture must record
  whether the menu was known-untouched.

It matters operationally: `internal/wiring`'s `OpenRealSessionFor` opens
a real radio at exactly the driver's `DefaultBaud`. The FT-710's 38400
is an *operating*-manual fact for that radio; this project holds no
FTdx101 operating manual, so 38400 here is the same-generation family
default and nothing more.

### 1.11 `MinFreqHz uint32`

| | Value |
|---|---|
| D | `30_000` |
| MP | `30_000` |

### 1.12 `MaxFreqHz uint32`

| | Value |
|---|---|
| D | `75_000_000` |
| MP | `75_000_000` |

**Status (both): ASSUMED — one register entry covering the pair.** This
manual carries NO storable-frequency range statement. Every frequency
legend says only "VFO-A Frequency (Hz)" or "Frequency (Hz)" over a
9-digit field (layout 1084 for IF, 1280 for MR, 1314 for MT, 1354 for
MW, 1438 for OI's VFO-B), which bounds the ENCODING at 999999999 and
says nothing about what either radio will store.

The nearest thing to a range is BS (BAND SELECT)'s P1 legend, which
enumerates band *buttons* — "00: 1.8 MHz … 10: 50 MHz, 11: GEN, 17: 70
MHz" (layout 506-514). **It is not evidence for these fields:** it names
transmit bands and a general-coverage position, not a receive floor or a
storable ceiling, and reading a 70 MHz entry as a `MaxFreqHz` would be
inventing a number. It is recorded here so a later reader does not
"find" it and quietly promote this entry.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"MinFreqHz 30_000 / MaxFreqHz 75_000_000"**.
- **Lift, per model:** the specifications page of that model's OPERATING
  manual (a document, not a session — the cheapest lift available), or,
  failing that, edge probes: MT Sets at the claimed floor and ceiling and
  just outside them, to a sacrificial channel, recording which are
  accepted. Note that the D and the MP are different radios in this
  respect *in principle* — a per-model document or a per-model probe set
  is required, and the D's specifications page is not evidence about the
  MP.

`MaxFreqHz` is additionally the ledgered dangerous-zero field: a zero
there reads as "no ceiling" to every validator, so it MUST be populated.
This entry is the honesty about where the number came from, not a licence
to leave it empty.

### 1.13 `RequiredSlots []string`

| | Value |
|---|---|
| D | `{"001"}` |
| MP | `{"001"}` |

**Status: ASSUMED.** That memory channel 001 must never be empty. This
manual states no such rule for either model anywhere. Claiming it makes
`codeplug` validation refuse a candidate whose 001 is blank, which is
the conservative direction — refuse rather than write a state the radio
may not tolerate — but it IS a claim.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"RequiredSlots {\"001\"}"**.
- **Lift, per model:** observation of channel 001 on a real radio of
  that model — whether it ships with 001 populated, and whether the
  front panel will erase it at all. A radio that erases 001 happily drops
  this from its `RequiredSlots`.

**Relationship to `Bank.NoBlank` (§2.4):** these are two different
mechanisms and the driver uses exactly one of each. `RequiredSlots` is
per-*slot* and is used for 001 only; `NoBlank` is per-*bank* and is
false on MEM and PMS, true on the two discovered banks. Neither implies
the other, and asserting `NoBlank` on MEM in order to protect 001 would
be the M5b failure repeated.

### 1.14 `ShiftOptions []ShiftOption`

| | Value |
|---|---|
| D | `spec.StandardShiftOptions()` — SIMPLEX (`ShiftNone`), PLUS (`ShiftUp`), MINUS (`ShiftDown`) |
| MP | identical |

**Status: MANUAL-EVIDENCED (vocabulary); CHOICE (order).** The P10
legend "0: Simplex 1: Plus Shift 2: Minus Shift" is printed in five
frame blocks — IF (layout 1097), MR (1294), MT (1327), MW (1367), OI
(1452) — all identical, none model-qualified. Three values, exactly the
three `spec.StandardShiftOptions()` carries, and `core/cat` parses this
radio's P10 through the same three-value vocabulary (the
reused-command verification verdict, `core/cat/ftdx101/doc.go`). The
*display order* is the shared standard's, which happens to match the
legend's own 0/1/2 order.

### 1.15 `CTCSSStates []ToneState`

| | Value |
|---|---|
| D | `spec.StandardCTCSSStates()` — OFF (`ToneOff`), ENC-DEC (`ToneEncodeDecode`), ENC (`ToneEncode`) |
| MP | identical |

**Status: MANUAL-EVIDENCED (vocabulary); CHOICE (display spellings and
order).** The P8 legend is printed in five frame blocks, none of them
model-qualified: MR (layout 1291), MT (1325), MW (1365) and OI (1449)
print it identically as "0: CTCSS \"OFF\" 1: CTCSS ENC/DEC 2: CTCSS
ENC"; IF (1095) prints the same three values with its off state
abbreviated, "0: OFF 1: CTCSS ENC/DEC 2: CTCSS ENC". The difference is
typographic — same three values, same three indices — and is noted so a
later reader diffing the five P8 legends against one another does not
take IF's for a fourth state or a different vocabulary. Three values,
matching `spec.StandardCTCSSStates()`'s three;
the project's spellings ("ENC-DEC") and their order are the shared
standard's, not the manual's punctuation.

**What this entry does NOT claim.** That the state byte *does anything
live* on either radio is unverified — see §2.2 and the driver-register
entry "TONE AND SCAN-SKIP UNREACHABILITY". `CTCSSStates` describes the
vocabulary the wire protocol expresses; `FieldCTCSSState`'s support
level (§2.1) describes how much this project trusts it.

---

## 2. Bank core fields — the per-bank `Fields` map

`core/spec/field.go` declares **ten** `spec.Field` constants. All ten
appear explicitly in every bank's map, including the four that are the
zero `FieldSupport`: a field left OUT of the map reads identically to a
field deliberately zeroed (`Capabilities.FieldSupport` returns the zero
value for an absent key), and only a written-down zero is legible as a
decision. `core/driver/ftdx10/caps.go`'s `bankFields` is the SHAPE
precedent for this — and only for the shape.

### 2.1 The matrix

`rw` = the profile's read/write support pair; `clar` = the clarifier's
own pair, kept separate so profiles can differ on it without the rest
moving. Values are **identical for D and MP** throughout §2; the reason
is §2.5.

| `spec.Field` | MEM | PMS | 60M (discovered) | EMG (discovered) | Status of "the combined MT record can/cannot express this" |
|---|---|---|---|---|---|
| `FieldFrequency` | `rw` | `rw` | read-only | read-only | MANUAL-EVIDENCED — MT P2, 9 digits at positions 6-14 |
| `FieldMode` | `rw` | `rw` | read-only | read-only | MANUAL-EVIDENCED — MT P6 at 22 |
| `FieldClarifier` | `clar` | `clar` | read-only | read-only | MANUAL-EVIDENCED (positions) — MT P3 sign+magnitude at 15-19, P4/P5 flags at 20-21. **The SIGN BYTE is not** — see the note below the table |
| `FieldCTCSSState` | `rw` | `rw` | read-only | read-only | MANUAL-EVIDENCED — MT P8 at 24 |
| `FieldCTCSSTone` | `{}` | `{}` | `{}` | `{}` | ASSUMED (driver register) — no tone-NUMBER byte in the record; P9 fixed |
| `FieldShift` | `rw` | `rw` | read-only | read-only | MANUAL-EVIDENCED — MT P10 at 27 |
| `FieldTag` | `rw` | `rw` | read-only | read-only | MANUAL-EVIDENCED — MT P12 at 29-40 |
| `FieldTagDisplay` | `{}` | `{}` | `{}` | `{}` | **MANUAL-EVIDENCED absence** — see §3.7 |
| `FieldScanSkip` | `{}` | `{}` | `{}` | `{}` | ASSUMED (driver register) — no skip flag in the record |
| `FieldErase` | `{}` | `{}` | `{}` | `{}` | **MANUAL-EVIDENCED absence** — no erase command exists |

"read-only" means the discovered bank's map is derived from MEM's with
every `Write` forced to `spec.Unsupported` (§1.3.5).

**The clarifier's POSITIONS are manual-evidenced; its MINUS-DIRECTION
BYTE is not** — the same split §1.5 draws between `TagLen`'s width and
`TagFill`'s byte, and for the same reason. P3 is five positions wide
(15-19) against a four-digit offset, so exactly one position carries the
direction; the plus direction is printed as one unambiguous glyph. But
the manual prints the MINUS direction as a **two-hyphen glyph** — "+:
Plus Shift, --: Minus Shift" — identically in all five frame pages that
carry it (layout 1085 IF, 1281 MR, **1316 MT**, 1355 MW, 1439 OI), and
the quarantined golden deriver recorded that glyph as UNREADABLE rather
than resolving it (`core/cat/ftdx101/testdata/provenance.md`, note N2,
which reasons out the one-position width and then declines to guess
which byte occupies it). The ASCII HYPHEN-MINUS 0x2D `core/cat` writes
and accepts there is INHERITED from the FT-710/FTdx10 convention.

- **Register home:** DIALECT register, entry *"The CLARIFIER'S
  MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D ('-')"* — the
  seventh, added at the M9d-1 milestone review.
- **Lift, per model:** one MR or MT Answer captured from a channel
  carrying a NEGATIVE clarifier offset — or, failing a channel already so
  written, an MW or combined-MT Set of a negative offset that the radio
  ACCEPTS, followed by a read of that channel. The byte the radio puts
  in, or takes at, the P3 sign position IS the direction.

**This does not change the `FieldClarifier` cell's value.** A
`FieldSupport` pair says whether the field is readable and writable, not
which byte encodes its sign, so no support level in this matrix depends
on the assumption — which is exactly why the entry lives in the dialect
register and is cited, not re-registered, here. The cell is annotated so
that it does not read as certifying a byte the dialect records as
unread.

**Profile values (both models):**

| Profile | `rw` | `clar` |
|---|---|---|
| `CapabilitiesUnverified` (RealHardware, and any unrecognised profile) | `{Read: Unverified, Write: Unverified}` | `{Read: Unverified, Write: Unverified}` |
| `CapabilitiesSimulated` (`internal/fakedx101` only) | `{Read: Supported, Write: Supported}` | `{Read: Supported, Write: Supported}` |

- **`Unverified` reads, not `Supported`, in the RealHardware profile:
  CHOICE, and the honest one.** The read path will have been exercised
  against a fake and a manual, and no FTdx101 of either model will have
  answered a frame. `Unverified` makes `FieldSupport.CanWrite` false, so
  this profile blocks every write project-wide.
- **`Supported` in the Simulated profile is a claim about
  `internal/fakedx101` and about nothing else**, and is true of it.
- **The clarifier is NOT `spec.Inert` in any profile.** `Inert` is the
  FT-710's HARDWARE finding (13/07/2026: a real FT-710 accepted MW frames
  carrying non-zero clarifier values without rejection and read back
  zeros every time). Neither FTdx101 has ever been asked, so there is no
  finding to record and borrowing one would be answering a question about
  one radio with another radio's evidence. If a Stage W session ever
  shows an FTdx101 ignoring the clarifier, the change is a per-profile
  `Inert` in the driver PLUS the same change in the fake, for that model
  — never one of the two, and never both models.

### 2.2 The two ASSUMED zeroes: `FieldCTCSSTone` and `FieldScanSkip`

**Status: ASSUMED.** What is *structural and manual-evidenced* here is
that the combined MT record accounts for every one of its 41 positions —
slot, frequency, clarifier sign and magnitude, the two clarifier flags,
mode, kind, CTCSS state, the fixed "00" P9, shift, the fixed "0" P11,
the 12-byte tag, terminator (`core/cat/ftdx101/testdata/geometry-witness.csv`
rows `MT,set,*` and `MT,answer,*`, counted off 300 dpi raster renders;
legends at layout 1312-1330) — that P9 is documented fixed "00" (layout
1326), and that no command this driver sends carries a tone NUMBER or a
scan-skip flag for a memory channel at all.

What is **assumed** is the step from that to "these fields are
unreachable on this radio": nothing verifies that the CTCSS-state byte
means anything live here, and whether some OTHER command in this manual
could reach a memory channel's tone number is not established either
way. The FT-710's answer — that none can, and that its P9 reads fixed
"00" with a tone demonstrably set and active — is that radio's hardware
finding and is not borrowed.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"TONE AND SCAN-SKIP UNREACHABILITY"**.
- **Lift, per model:** one channel set to a known CTCSS tone from the
  front panel of that radio, then read over CAT — if any byte of the
  answer tracks the tone number, the entry is refuted and the capability
  opens; if P9 reads "00" as documented, the entry closes as a confirmed
  protocol limit rather than an assumption. The scan-skip half needs the
  same experiment with the front-panel skip flag.

**Consequence for M9d-2's CHIRP fold.** `FieldScanSkip` being the zero
`FieldSupport` on both new models is what the spec's caps-aware
blank-Skip construction keys on: on a radio whose `FieldScanSkip` is
`Unsupported` in both directions — today, every registered radio, and
both FTdx101s — a blank CHIRP Skip cell yields `{Unknown}` and an "S"
cell yields `{Unknown}` plus a non-blocking `LossEntry`. This matrix
confirms the FTdx101s join that class; the fold itself is the plan's.

### 2.3 `FieldErase` — a manual-evidenced absence

**Status: MANUAL-EVIDENCED absence, both models.** The command
availability table (layout 236-337) lists this radio's entire CAT
command set, and it contains **no erase command** for a memory channel.
There is therefore no erase for this driver to offer, in either
direction, on any bank, in any profile.

`Unsupported` is additionally the direction that needs no evidence:
whether some Set frame has an erasing side effect on this radio is
unknown and deliberately not claimed, and keeping `FieldErase`
unwritable keeps a populated channel going back to empty permanently
blocked (`codeplug.Diff` gates on `FieldErase`, not on `Bank.NoBlank`).

### 2.4 `NoBlank` per bank, and its relationship to `RequiredSlots`

| Bank | `NoBlank` | Status |
|---|---|---|
| MEM | `false` | CHOICE, stated explicitly (§1.3.1) |
| PMS | `false` | CHOICE, stated explicitly (§1.3.2) |
| 60M (discovered) | `true` | CHOICE — a statement about the write surface this project offers, not about factory contents (§1.3.4, §1.3.5) |
| EMG (discovered) | `true` | as above |

Identical for D and MP.

`RequiredSlots` (§1.13) is the **per-slot** mechanism and carries
`"001"` only; `NoBlank` is the **per-bank** mechanism. The two are
independent by design (`core/spec/capabilities.go`'s own field comment
draws the distinction), and this driver deliberately uses the per-slot
one for its single required channel rather than reaching for the
per-bank one.

### 2.5 Why one value serves both models throughout §2

The memory-channel surface is printed ONCE for both radios and carries
no model qualifier anywhere:

- the MT frame block, its position chart and every one of its P-legends
  (layout 1311-1345) — no model name appears in it;
- the MR (1277-1294) and MW (1352-1367) blocks likewise;
- the slot legends (§1.3);
- and the Table 2 applicability sweep's explicit attestation, **"NO
  row's stored properties are model-conditional"**
  (`core/cat/ftdx101/testdata/group-ledger.md` §Attestation), which
  inspected all 193 rows cell by cell at 400 dpi for asterisks, daggers,
  superscripts, footnote markers, model names and bracketed qualifiers,
  plus the margins on all three chart pages.

§4 enumerates the entire model-distinguishing surface of this document
and shows that none of its three members lands on any field in §2.

---

## 3. Behaviours no `spec.Capabilities` field expresses

Each of these is a real decision the M9d-2 driver makes, with no
capability field to carry it. Each gets the same evidence discipline as
a capability field. §3.1-§3.11 are the behaviours the M9d-2 spec section
names; §3.12 is ADDED by this matrix.

### 3.1 Serial framing — 8 data bits, no parity, TWO stop bits

**Value (both models): 8-N-2**, inherited from `core/transport`'s
`DefaultStopBits = 2` (`core/transport/port.go:27`), which every session
this driver opens will use. There is no framing field in
`spec.Capabilities` and, per the E2-recorded decision, none is added
without hardware evidence.

**Status: ASSUMED.** **This manual is SILENT on framing.** It states no
stop-bit count, no parity and no data-bit width anywhere: the connection
sections (layout 16-126) describe cables, virtual COM ports and a
level converter but no line discipline, and the serial menu block
carries only `232C RATE`, `232C TIME OUT TIMER`, `CAT RATE`, `CAT TIME
OUT TIMER` and `CAT RTS` (layout 861-865). 8-N-2 is the FT-710's own
documented framing, inherited here as a same-generation working value
by way of a transport default.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"FRAMING: 8 data bits, no parity, TWO stop bits"**.
- **Lift, per model:** an ID exchange at the answering baud with that
  radio's port opened 8-N-2 — a clean "ID0681;" (D) or "ID0682;" (MP)
  confirms the framing is at least compatible. **THE LIFT MUST
  DISTINGUISH FRAMING FROM THE CONTROL LINES (§3.2):** silence at a
  known-correct baud is NOT evidence about stop bits until `CAT RTS` has
  been toggled at the radio and the exchange retried, because a handshake
  refusal and a framing mismatch present identically as nothing coming
  back. Try 8-N-1 only after that has been done, and record which of the
  two changed the outcome.

### 3.2 Control-line policy — RTS and DTR driven low at open

**Value (both models):** `core/transport.OpenSerial` drives RTS and DTR
low (`SetRTS(false)`, `SetDTR(false)`) immediately after opening any
port — safety obligation 4, `core/transport/port.go:107-119` — for every
model, with no per-radio policy. The FTdx101 driver adds none.

**Status: ASSUMED (that this is safe on these radios).**

**This radio has a CAT RTS menu item of its own:** (03,01,13) `CAT RTS`,
P4 legend "0: DISABLE 1: ENABLE" (layout **865**, printed page 11;
committed at `core/cat/ftdx101/table2.csv:255` and generated into
`exinventory_gen.go`). So the radio evidently has an opinion about the
line that this project's transport does not consult. What that menu's
factory setting is, and whether a low RTS with it ENABLEd stalls the CAT
link, is unknown for both models.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"CONTROL-LINE POLICY"**.
- **Lift, per model:** one ID exchange in each `CAT RTS` menu state on
  that radio, everything else held constant. If the exchange answers in
  both, the policy is safe on that model and the entry closes for it; if
  it answers in only one, the transport needs a per-radio control-line
  policy and this becomes a spec'd capability rather than an assumption.
  **Take this capture BEFORE concluding anything about §3.1.**

Note the FTdx10's `CAT RTS` sits at (03,01,10) and this radio's at
(03,01,13); the addresses are not interchangeable, and the FTdx101's is
established here from its own chart.

### 3.3 Factory-default baud — distinct from the rate list

Covered as a capability field at **§1.10** (`DefaultBaud`), because the
field exists; it is listed here because the *fact* it encodes — which
rate a factory-configured radio actually answers at — is nowhere in this
CAT manual and is a distinct question from the four-entry `CAT RATE`
list at §1.9. Status **ASSUMED**; DRIVER register entry **"DefaultBaud
38400"**; lift per model as at §1.10.

The distinction matters because §1.9 is MANUAL-EVIDENCED and §1.10 is
not, and the two sit one line apart in the driver source. A reader who
reads the `Bauds` citation as covering both would promote an assumption
by proximity.

### 3.4 Discovery — the 5xx/EMG walk

**Value (both models):** `Open` probes EVERY slot the dialect's
`SlotSpace` declares — **501 through 599, ascending** — and then EMG.
No contiguity assumption, no sentinel, no early termination, no cap. A
well-formed answer means the slot exists; a "?;" rejection means it does
not (§3.8). Probes use MT reads, for the same reason the read path is
MT-only (§3.5).

**Termination policy: CHOICE, and deliberately the FTdx10's, not the
FT-710's.** The FT-710's rules — stop at the first rejection, cap at 15,
probe one sentinel past the cap — are FT-710 HARDWARE facts about a
radio whose factory 5xx channels are believed contiguous and
non-erasable. None of that is known for either FTdx101. A sparse bank (a
populated 503 after an empty 502) must not be silently truncated, and
with no evidence the only way to be sure is to ask every declared slot.

**Range: the 501..599 bound is ASSUMED** — DIALECT register entry
*"SlotSpace.SixtyLo/SixtyHi = 501/599"*, lift per model as at §1.3.4.
The 5xx family's and EMG's *existence* is MANUAL-EVIDENCED (§1.3.4).

**Cost shape:** 99 + 1 = **100 exchanges per Open**, about 2-2.5 s at
the engine's default per-exchange settle, paid by every FTdx101 session
this project opens including in tests — the same budget shape the M9c-6
plan accepted for the FTdx10, and the shape the M9d spec names. It is
ACCEPTED and budgeted. **NOBODY narrows it** — settle override, early
termination, range shrink — without an orchestrator-adjudicated change.
The M9d-2 plan should carry the FTdx10's pin shape: a test asserting the
full ordered probe transcript, so that a regression is a test failure
rather than a silently shorter walk. **Two models means two sessions'
worth of test time**, which the plan must budget for.

### 3.5 Read choreography — MT-only, atomic; MR unused

**Value (both models):** ONE combined 41-byte MT read carries the whole
field block AND the tag. MR is never sent by this driver — not by
`ReadChannel`, and not by discovery.

**Status: MANUAL-EVIDENCED that the combined form carries everything;
CHOICE that MR is therefore unused.** The MT Set/Answer position chart
runs to 41 — the 28 shared positions, P11 fixed "0" at 28, a 12-character
P12 tag at 29-40, ';' at 41 — independently counted off 300 dpi raster
renders (`core/cat/ftdx101/testdata/geometry-witness.csv`,
`MT,answer,*`) and matching the legends at layout 1311-1330. MT's
availability row is O O O X (layout 334) and MR's is X O O X (layout
331), so both directions exist. A combined answer is an ATOMIC snapshot:
a two-frame stitch (field block from one radio state, tag from a later
one) is structurally impossible, whereas the FT-710's MR+MT pair has to
hold a session-wide lock across the gap to avoid tearing.

This is a DESIGN DECISION, not an omission, and M9d-2 must write it down
where the FTdx10 driver does, so that nobody later "completes" the read
path by adding an MR exchange. MR stays fully covered at the DIALECT
level (golden vectors, `dialecttest`, the fake's answers), because a
dialect describes a radio's protocol, not one driver's use of it.

**One assumption rides on the answer width:** that the radio answers at
the FULL 41 bytes. The printed grid is a maximal frame, and the FT-710
precedent — hardware accepting short MT Sets against a maximal grid —
makes a variable-width ANSWER live.

- **Register home:** DIALECT register, entry *"The combined MT answer's
  EXACT length (consumed here as `MTAnswerBounds() = (41, 41)`)"*.
- **Lift, per model:** one MT READ of a channel carrying a tag SHORTER
  than 12 characters, the raw answer captured whole. A 41-byte answer
  confirms exactness for that model; anything shorter converts the parser
  and the gate to the recorded 30..41 window contingency.

**No per-class P7 kind narrowing, and a legend that invites one.** The
FT-710's driver keeps its own P7 kind vocabulary per bank because its
leniencies are ITS live observations; this driver has none, so it adds
no check of its own and lets `cat.Dialect.ParseMTAnswerCombined` narrow
P7 to the combined record's OWN documented read pair — MT's P7 legend
reads "Set: 0: (Fixed) / Read: **0: VFO 1: Memory**" (layout 1324), and
MR's answer P7 the same two values (layout 1290). **This manual prints a
SIX-value P7 elsewhere** — IF's and OI's, "0: VFO 1: Memory 2: Memory
Tune 3: Quick Memory Bank (QMB) 4: - 5: PMS" (layout 1092-1093 and
1447-1448). Those are IF's and OI's own parameters on their own frames,
NOT the memory record's, and they must not be read across: widening MT's
accepted P7 to six values on the strength of a different command's
legend would be inventing a distinction this manual does not draw for
MT. An out-of-vocabulary byte surfaces from `core/cat` as a
`*cat.ParseError`, which the driver wraps with the slot it was reading.

**Session mutex:** the one-exchange-per-operation property is why the
FTdx10 session carries no operation mutex — `transport.Engine` already
serialises each individual exchange. The FTdx101 session inherits the
same property and the same caveat: a future operation needing two frames
needs an `opMu` with it.

### 3.6 Write choreography — MT-only; one combined Set, including for an empty slot

**Value (both models):** ONE combined MT Set per channel. MW is never
sent. The Set is sent fire-and-forget — the transport spec waits for no
reply — because an ACCEPTED Set is assumed to draw no reply at all and a
REJECTED one exactly one "?;". That is an INHERITED framing
convention, not a reading of this manual, and its register home is
`core/driver/ftdx101/doc.go`'s ASSUMED register entry 9, second half.
MT's availability row (layout 334) gives Set O, Read O, Answer O, AI X
and grounds NONE of it: the `Ans.` column marks the existence of the
command's ANSWER FORM — the frame a READ draws — which is why read-only
MR carries Answer O too (X O O X, layout 331). **(Rev 3 erratum 1.)**

**Status: MANUAL-EVIDENCED that one frame carries everything; ASSUMED
that the radio accepts it as a complete channel definition; ASSUMED, as
an inherited convention with no line in this manual behind it, that an
accepted Set is answered with silence and a rejected one with "?;"
(rev 3 erratum 1).** The 41-byte Set carries the full field block and
the tag (layout 1311-1330; geometry witness rows `MT,set,*`), so MW
would write the same fields redundantly with a strictly smaller frame
(28 bytes, layout 1352-1367). MW's own restriction to "001-099, P1L-P9U"
(layout 1353) is a second reason not to reach for it.

Whether either radio accepts the combined Set as a complete channel
definition — and whether it does so for a slot that does not yet exist —
is unverified. The FT-710's empty-slot create is HW-CONFIRMED for ITS
two-frame MW+MT choreography, which is not this one.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"A SINGLE COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL,
  INCLUDING AN EMPTY ONE"**. It must land WITH the driver skeleton, one
  task ahead of the write path, because it is the assumption the whole
  MT-only choreography rests on and it must not arrive later than the
  design it justifies. **As built, that entry carries TWO ASSUMED
  halves** — its title continues **"… — AND AN ACCEPTED SET DRAWS NO
  REPLY WHILE A REJECTED ONE DRAWS \"?;\""** — because one design
  decision (the zero `transport.CommandSpec` on the Set) rests on both
  (rev 3 erratum 1).
- **Lift, per model:** the FIRST write trial on that radio — one combined
  MT Set to a sacrificial EMPTY channel, then an MT read back, then the
  same against an already-populated channel, **with the port watched
  between the Set and the read-back**, which is what lifts the
  acknowledgement half (rev 3 erratum 1). Byte-faithful read-back on
  both is the lift; anything else (rejection, partial field application,
  tag written without the field block) converts the write path to a
  two-frame choreography and this entry to a finding.

**Nothing may be written at M9d-2 regardless** — see §3.11.

**Two supporting facts, both MANUAL-EVIDENCED and both easy to
conflate:** MT-Set P7 is the FORM's constant, "Set: 0: (Fixed)" (layout
1324), and MW-Set P7 is separately "0: (Fixed)" (layout 1364), which is
what `MWWriteKind: cat.CombinedMTSetKind` carries (dialect.go:116). They
are two independent facts of this radio that happen to agree, not one
fact used twice — `core/cat`'s own tests refuse to let the combined
Set's P7 be sourced from the MW write kind.

### 3.7 `TagDisplay` — the P11 position

**Value (both models):** `spec.FieldTagDisplay` is the ZERO
`FieldSupport` — Read AND Write `Unsupported` — on every bank and in
every profile, and every read reports `codeplug.Unavailable` ("this
radio has no such field"), which is a different statement from
`Unknown` ("this radio has it, we did not learn it").

**Status: MANUAL-EVIDENCED absence.** This is a fact, not an assumption.
The combined MT record has **no display flag**. Its P11 is "0: (Fixed)"
(layout **1329**), a single fixed position at byte 28, and the frame's
41 positions are fully accounted for by the independent geometry witness
(`core/cat/ftdx101/testdata/geometry-witness.csv`: MT set/answer rows
running MT, P1, P2, P3, P4, P5, P6, P7, P8, P9, P10, P11, P12, ';' over
1-41 with no gap). `cat.Dialect.BuildMTSetCombined`'s signature takes no
display argument because there is nowhere to put one.

**The contrast that makes this legible:** on the FT-710, MT's display
flag is MANDATORY and its write path refuses a non-`Known` `TagDisplay`
outright. Here the frame carries no such flag, so the FTdx10's NAMED
INVERSION applies — the write ladder must treat `Unavailable` as
acceptable rather than refusing on it — and M9d-2 must document that
inversion at the site, as the FTdx10 driver does at `buildWriteCommand`.

The spec fixes this as a choice ("`TagDisplay` reported `Unavailable`
(P11 fixed — the frame carries no display flag)"); the evidence is
independent of the spec having said so.

### 3.8 The two "?;" interpretations

"?;" is the protocol's **SINGLE unattributed NAK** (`cat.ErrRejected`'s
own doc comment): it is also what an unknown command, a bad parameter
and a wrong radio state get. Reading anything more specific out of it is
an interpretation, and this driver makes two different ones.

#### 3.8.1 "?;" on a 5xx/EMG discovery probe means ABSENT FROM THIS RADIO

**Value (both models):** discovery treats a rejection as "this radio does
not have that channel" and a well-formed answer as "it does".

**Status: ASSUMED.** The FT-710's equivalent interpretation is
hardware-confirmed for that radio (live probes of a never-populated and
an out-of-inventory slot both answered "?;"). Neither FTdx101's is.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"\"?;\" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO"**.
- **Lift, per model:** an MT enumeration of the whole 5xx range on a
  radio of that model with a POPULATED 5xx bank, cross-checked against
  the channels the front panel shows. Which wire numbers answer and which
  reject is then a fact.

**TWO REGISTER ENTRIES, ONE POSSIBLE CAPTURE — record both explicitly.**
A 5xx enumeration extended to include 500 speaks to this driver entry
*and* to the dialect's 501..599 numbering entry (§1.3.4). **They are
separate assumptions**: this one is about what a rejection MEANS, that
one is about where the range STARTS AND STOPS. One session can retire
both for one model, and the FTdx10 driver register's entry 7 is the
precedent for saying so — but the capture note must name both, because a
session that answers only one leaves the other open, and an entry
retired by implication is an entry nobody can audit. The two entries'
lifts are worded with different commands (MT here, MR in the dialect's)
for the reason §1.3.4 gives; either command's enumeration serves, and a
single MT walk extended to 500 serves both entries at once.

#### 3.8.2 "?;" on a combined-MT read of a slot means EMPTY CHANNEL

**Value (both models):** a rejection on `ReadChannel` maps to an EMPTY
`codeplug.Channel` — `Data` nil, the slot carried through — rather than
an error, so a read of a blank channel is an ordinary outcome.

**Status: ASSUMED.** The FT-710's equivalent was verified for its MR
read, NOT for this frame, and neither FTdx101's combined MT read of an
empty channel has ever been seen. **The failure mode if this is wrong is
quiet and bad:** a transport-level problem manifesting as "?;" would be
recorded as a genuinely empty channel and could later be written back as
one.

- **Register home:** DRIVER register, to be created at M9d-2, entry:
  **"\"?;\" ON A COMBINED-MT READ OF AN EMPTY SLOT"**.
- **Lift, per model:** one MT read of a channel known empty from that
  radio's front panel, and one of a channel known populated, in the same
  session. Two different answers confirm the mapping; two rejections mean
  the driver has been reading blank channels out of a broken link.

**The two interpretations share one wire signal and one mechanism**, and
the plan must keep them as two register entries: a capture that settles
one does not settle the other.

### 3.9 The settings (EX) read

**Value (both models): 193 items**, in four menus (P1 ∈ {01, 02, 03,
04}) and **18 groups** ((P1,P2) pairs), built from the dialect's
`EXItems()` in inventory order.

**Status: MANUAL-EVIDENCED, via the committed blind-transcription
chain.** The count is the real committed inventory's, not an estimate:
`core/cat/ftdx101/table2.csv` carries 193 data rows and
`core/cat/ftdx101/exinventory_gen.go` generates 193 `cat.EXItem` values,
established by the three-leg blind chain (PDF-primary boundary ledger,
layout-led transcription A, PDF-primary transcription B) with zero
cross-check mismatches, over Table 2 "MENU Chart" at layout 715-962
(printed pages 10-12). The menu/group partition was computed from
`table2.csv` while writing this matrix: P1 ∈ {01, 02, 03, 04}, 18
distinct (P1,P2) pairs.

**The spec's "197-item-class settings read" is a CLASS description, not
a count.** 197 is the FTdx10's inventory size; the FTdx101's is 193. The
spec's own wording — "197-item-class settings read **sized by the real
inventory count**" — anticipates exactly this. The M9d-2 plan must use
193 and must not carry 197 anywhere.

**Two related facts, both MANUAL-EVIDENCED:**

- **The EX grammar.** Read "EX P1 P1 P2 P2 P3 P3 ;" is nine bytes;
  Answer "EX P1 P1 P2 P2 P3 P3 P4 ~ P4 ;" is the six-digit address, a
  variable P4 and the terminator (frames at layout 699-708; availability
  row O O O O at layout 286). This matches `core/cat`'s
  `exReadLen`/`exAnswerMinLen` exactly — the reused-command verification
  verdict in `core/cat/ftdx101/doc.go`.
- **The header-vs-chart anomaly, UNRESOLVED.** EX's grammar block states
  "P1 : 01 - 05" on layout **700** (its three range statements run
  700-703, and the P4 line at 704 completes the block) while Table 2
  populates P1 01-04 only,
  ending at (04,03,02) PIXEL (layout 962). The inventory follows the
  CHART. The FT-710's analogous anomaly could be put to hardware; this
  one cannot be, and `core/cat/ftdx101/doc.go` records it unresolved. It
  bounds nothing in the settings read, because membership comes from the
  chart — but a Stage R session should probe a P1=05 address and record
  the answer.

**Raw values only, no value semantics.** The descriptor carries an
address, two labels and a human display form per item; `ReadSetting`
returns the P4 body verbatim. That is why the chart printing defects
`core/cat/ftdx101/doc.go` records — the AM MAX POWER self-inconsistency,
CW BK-IN DELAY's truncated legend, SHIFT FREQUENCY's and MARK
FREQUENCY's missing index 0, DECODE AFC RANGE's non-monotonic legend,
KEYBOARD LANGUAGE's twelfth entry "11: LEVEL", QSK DELAY TIME's "mesc" —
do not bite this surface. Every one of them lives in a value legend, and
this driver interprets no legend. M9d-1's boundary stands.

### 3.10 Probe identity — two CAT IDs, and the cross-model evidence rule

**Value:** D answers `ID;` with `ID0681;`, MP with `ID0682;`. The probe
refuses the wrong sibling's ID with BOTH names spelled out ("radio
identifies as FTdx101MP; you selected FTdx101D"), refusing BEFORE any
discovery traffic and closing the port.

**Status: MANUAL-EVIDENCED (the IDs); CHOICE (the refusal and its
wording).** Layout 1070 and 1072; frames 1069-1078; availability X O O X
at layout 304; ID's Answer is seven bytes, matching `core/cat`'s
`idAnswerLen`. **This is the second D-vs-MP divergent entry in this
matrix** (with §1.2, which is the same fact seen from the capability
side).

**The rule this entry exists to state:** *a capture from one model is
NEVER evidence about the other.* The two radios share a manual, not a
serial port. "The D answered 0005 to a non-multiple-of-10 clarifier Set"
is evidence about the D's `ClarStepHz` and about nothing of the MP's.
Every ASSUMED entry in this matrix therefore names a **per-model** lift,
and the M9d-2 driver register is tracked **per model** — an entry stays
open for whichever model has not been asked. This is the register's one
difference from the FTdx10's, where there is a single radio and a single
lifting.

**Mechanism note (spec A5, a plan concern, recorded here for
completeness):** `driver.WrongRadioError` carries IDs only and
`cmd/rigprog/probe.go` has no ID→name mapping, so the both-names refusal
needs a defined additive extension, chosen so the shared type's
`Error()` text for the FT-710 and the FTdx10 is PINNED UNCHANGED.

### 3.11 `writeTrialsComplete` — pinned FALSE, per model

**Value: `false` for the D and `false` for the MP.**

**Status: not an evidence question — a FACT ABOUT THIS PROJECT.** No
FTdx101 of either model has ever been written to. There is no
`docs/hardware-notes.md` section for either, no write-trial protocol
run, and no captured frame from either.

While it is false there is no hardware-verified capability profile for
this driver to select AT ALL — deliberately not even a placeholder one.
A `RealHardware` session gets `CapabilitiesUnverified`, nothing is
writable anywhere, and the capability gate refuses every write before a
frame is built. Any unrecognised `Profile` value fails the same way: the
failure direction is always "nothing writable".

It must be consulted by no production code, and that is the point.
Flipping it is a TWO-PART change — the constant AND a
`CapabilitiesRealHardware` profile built field class by field class from
that model's trial evidence, AND the `Capabilities` switch rewritten to
select it — with the evidence linked and the pin test rewritten so the
flip is a visible, reviewable test change. Making the constant
load-bearing on its own would mean a one-character edit could unlock a
write.

**Per model** means the pin must assert both halves for **each**
registration: the constant is false, AND a `RealHardware` session for
that model is genuinely nothing-writable. A D trial can never flip the
MP's. Flipping any `writeTrialsComplete` is an explicit M9d non-goal.

### 3.12 ADDED — CAT reachability differs between the radio's two ports

Found while surveying the manual for §3.1 and §3.2; it fits neither the
capability list nor the spec's named behaviours, so it is ADDED rather
than skipped.

**Value:** this project opens one serial port and speaks CAT over it.
This radio offers two paths, and they are **not equivalent**.

**Status: MANUAL-EVIDENCED, both models.**

- The radio "contains two virtual COM ports, an Enhanced COM Port and a
  Standard COM Port"; the **Enhanced** port is for CAT communications
  and the **Standard** port for TX control (PTT, CW keying, digital
  modes) — layout 75-79 (**rev 3 erratum 2**: this range was DISPUTED
  during M9d-2's execution and CONFIRMED CORRECT by direct measurement;
  it stands unchanged). A user pointing this project at the Standard
  port will get silence that looks exactly like a wrong baud or a
  framing mismatch.
- **AI is USB-only.** AI's frame block carries the note "The AI command
  is available only when PC is connected with USB cable" (layout **381**)
  and "This parameter is set to \"0\" (OFF) automatically when the
  transceiver is turned \"OFF\"" (layout **384**). The driver's `Open`
  choreography begins with an AI0 Set; over the rear RS-232C jack that
  command is documented as unavailable. AI0 is sent fire-and-forget on
  the same INHERITED acknowledgement convention as the MT Set (driver
  register entry 9's second half, §3.6) and **not** on the strength of
  its availability row, whose O O O X at layout 244 says AI has Set,
  Read and Answer FORMS and says nothing about what a Set draws
  (**rev 3 erratum 1**) — so a session would not
  obviously fail — it would simply not have disarmed auto-information.
- The RS-232C jack additionally cannot be used while the external
  antenna tuner is selected (layout 111-112, restated as a NOTE at 123),
  and the PS (POWER SWITCH) command is unavailable over it (layout
  105-106, restated on PS's own frame page at 1542-1544).

**Why this belongs in the matrix:** none of it is a capability value, but
all of it changes what "the radio did not answer" means during a Stage R
session, and §3.1's and §3.2's lifts both depend on being able to
attribute silence. **Any Stage R capture must record which port it was
taken on.** A capture taken over RS-232C cannot lift the AI-related half
of the open choreography at all.

**No action for M9d-2's code** — this project already opens whatever port
it is given and this manual gives no way to detect which one — but the
plan should carry the port question into `internal/radiotext`'s
per-model prose (`ProbeFirmwareNote` is the natural home), and the Stage
R protocol must record it.

---

## 4. The three places this manual distinguishes the models

Yaesu prints ONE CAT manual for the FTDX101MP and the FTDX101D. The two
models are distinguished in exactly **THREE** places
(`core/cat/ftdx101/doc.go`, "One manual, two radios"). All three are
named here, and each is checked against this matrix:

| # | Place | Layout | Touches a capability value? |
|---|---|---|---|
| 1 | **The ID answer** — "0681: FTDX101D", "0682: FTDX101MP" | 1070-1072 | **YES** — `CATID` (§1.2), and `Model` (§1.1) by construction. These are the ONLY two model-conditional values in this entire matrix. |
| 2 | **The P4 VALUE ranges of three MAX POWER rows** in Table 2 — (03,04,01) HF MAX POWER (927), (03,04,02) 50M MAX POWER (928), (03,04,04) AM MAX POWER (931). (03,04,03) 70M MAX POWER (929) is NOT conditional — "5 ~ 50 (P4 = 005 ~ 050)" for both — which is why the count is three and not four. | 927, 928, 931 | **NO.** P4 SEMANTICS are not stored: an `EXItem` models the address, the labels, the name, the `Digits` width and the text flag, and all five are printed identically for both models on all three rows. The three rows are ordinary members of the 193-item inventory (§3.9) and nothing in §1 or §2 reads a P4 legend. |
| 3 | **The PC (POWER CONTROL) command's P1 range** — "005 - 100 (FTDX101D)" / "005 - 200 (FTDX101MP)" | 1496, 1498 (under the PC heading at 1495) | **NO — and OFF THIS PROJECT'S SURFACE entirely.** `core/cat` has no PC builder, no PC parser and no PC entry in any gate, and M9d-2 adds none. Whichever milestone adds PC inherits a model-conditional range and must carry it per dialect, exactly as `CATID` is carried now. |

**Everything else in the document is unconditional.** A full sweep of
the layout extraction for the strings `FTDX101D` and `FTDX101MP` while
writing this matrix returned exactly the occurrences above plus:

- joint prose forms — "the FTDX101MP/FTDX101D transceiver" — at layout
  11, 17, 62, 75, 109, 190-193 and 224, none of which conditions
  anything;
- **two rear-panel illustration labels reading "FTDX101D"** (layout 41
  and 161), in the USB-cable and RS-232C-cable connection diagrams.
  These are picture captions on a chassis drawing, not a fourth
  distinction: the surrounding text of both sections addresses
  "FTDX101MP/FTDX101D" jointly, and no protocol statement is attached to
  either. Recorded so a later reader who runs the same grep does not
  count five places and conclude the register is wrong.

**Therefore:** the sole model-conditional capability value in this
matrix is the CAT ID (and the `Model` string that names it). Every other
value in §1, every cell in §2, and every behaviour in §3 is identical
between the D and the MP — but the **evidence status** of each is
tracked per model regardless, because an assumption is lifted by a
capture and a capture comes from one radio.

---

## 5. Completeness claim

**Every `spec.Capabilities` field appears.** `core/spec/capabilities.go`
declares fifteen: `Model`, `CATID`, `Banks`, `Modes`, `TagLen`,
`ClarMaxHz`, `ClarStepHz`, `CTCSSTones`, `Bauds`, `DefaultBaud`,
`MinFreqHz`, `MaxFreqHz`, `RequiredSlots`, `ShiftOptions`,
`CTCSSStates`. All fifteen have an entry in §1, by name, in struct
order, each with a value stated for the D and the MP separately and each
with a status.

**Every `spec.Field` appears, in every bank.** `core/spec/field.go`
declares ten: `FieldFrequency`, `FieldMode`, `FieldClarifier`,
`FieldCTCSSState`, `FieldCTCSSTone`, `FieldShift`, `FieldTag`,
`FieldTagDisplay`, `FieldScanSkip`, `FieldErase`. All ten appear in
§2.1's matrix across MEM, PMS, 60M and EMG, including the four that are
the zero `FieldSupport`. `NoBlank` is stated for all four banks (§2.4)
and its relationship to `RequiredSlots` is stated at §1.13 and §2.4.

**Every no-field behaviour the M9d-2 spec section names appears:**
serial framing (§3.1), control-line policy (§3.2), factory-default baud
(§3.3, with §1.10), the 5xx/EMG discovery walk with its range,
termination policy and per-Open cost shape (§3.4), read choreography
(§3.5), write choreography (§3.6), `TagDisplay`/P11 (§3.7), both "?;"
interpretations (§3.8.1, §3.8.2), the settings read sized by the real
inventory count (§3.9), probe identity and the cross-model evidence rule
(§3.10), and `writeTrialsComplete` pinned false per model (§3.11).

**One item was ADDED, not skipped:** the two-port CAT reachability
finding (§3.12), which fits neither list but changes what silence means
during every Stage R capture this matrix depends on.

**The M9d-2 DRIVER register this matrix implies has NINE entries**, to
be created at M9d-2 and tracked per model:

1. FRAMING: 8 data bits, no parity, TWO stop bits (§3.1)
2. CONTROL-LINE POLICY (§3.2)
3. DefaultBaud 38400 (§1.10, §3.3)
4. MinFreqHz 30_000 / MaxFreqHz 75_000_000 (§1.11, §1.12)
5. RequiredSlots {"001"} (§1.13)
6. TONE AND SCAN-SKIP UNREACHABILITY (§2.2)
7. "?;" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT (§3.8.1)
8. "?;" ON A COMBINED-MT READ OF AN EMPTY SLOT (§3.8.2)
9. A SINGLE COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL,
   INCLUDING AN EMPTY ONE (§3.6)

**Six of the DIALECT register's seven entries are CITED and not
re-registered** — they are `core/cat/ftdx101/doc.go`'s, and correcting
one is a dialect change, not a driver change:
`ClarifierPolicy.StepHz = 10` (§1.7), `SlotSpace.SixtyLo/SixtyHi =
501/599` (§1.3.4, §3.4), the `cat.ModeUnset` mode-table member (§1.4),
`MTPolicy.TagFill = ' '` (§1.5), the combined MT answer's exact 41-byte
length (§3.5), and the clarifier's minus-direction byte 0x2D (§2.1).
The seventh, `SlotSpace.NoneWire = "000"`, is a dialect-level fact that
**no capability value and no `FieldSupport` cell in this matrix depends
on**, and is named here so the count of seven is visibly complete.

**THE PLAN MUST NOT READ THAT AS "the driver register cites six".** The
count above is what the *matrix* reaches, and the matrix models
capability values; the *driver* reaches further, and the seventh entry
is exactly where it does. The FTdx10 driver's
register cites **all six** of its dialect's six entries — "the DIALECT's
six entries … are separate and are CITED below where **this driver
depends on them** — MTPolicy.TagFill, ClarifierPolicy.StepHz,
**SlotSpace.NoneWire**, the cat.ModeUnset table member, the 501..599
numbering, and the combined answer's exact 41-byte width"
(`core/driver/ftdx10/doc.go:156-162`) — and the `NoneWire` dependence is
real and pinned: `BuildMTRead` refuses the answer-only none form, which
that driver documents at the call site
(`core/driver/ftdx10/read.go:117-120`, "the answer-only none form
(\"000\" for this dialect — its own ASSUMED register entry 3):
grammatical per ParseSlot, never a legal read target").

**M9d-2's driver inherits the same read and write rejection ladders**,
and so the same `NoneWire` dependence. So the M9d-2 driver register
should expect to cite **all seven** dialect entries at its own
dependence sites — the six this matrix reaches, plus `NoneWire` at the
`BuildMTRead`/`BuildMTSetCombined` refusal sites — which is *more* than
this matrix cites, not the same number. A driver register that cited six
because this section says six would be under-citing against its own
precedent.

**Neither register may absorb the other.**

**No contradiction with any committed evidence artefact was found.**
Every value in this matrix was checked against `table2.csv`, the
generated `exinventory_gen.go`, `core/cat/ftdx101/doc.go`'s register and
citations, `testdata/geometry-witness.csv`, `testdata/group-ledger.md`
and `dialect.go`, and against the manual's own layout lines. Where this
matrix says something the FTdx10 driver's comments compress — the
5xx/EMG write exclusion being project policy rather than a manual fact
(§1.3.5) — it says the longer thing, and adds nothing to the FTdx101's
committed record.

That claim was true when it was written and is **qualified by §6**: rev 3
found one inference in this matrix that did not hold (erratum 1) and one
citation that had gone stale under a later deletion (erratum 3). Neither
changes a capability value, a `FieldSupport` cell or a count.

---

## 6. Errata (revision 3, 09/08/2026 — three rev-4 notes appended inside erratum 3, one of them at erratum weight)

This document was the M9d-2 milestone's capability authority and was
CONSUMED by an executed plan, so it is corrected under erratum
discipline, not by rewriting: each entry below states **what stood**,
**what now stands**, and **the record that adjudicated it**, and each
corrected site in the body carries a `(rev 3 erratum N)` tag pointing
here. Nothing in §1's values, §2's cells, §3's other behaviours, §4's
three model distinctions or §5's counts — including the 193-item EX
inventory — is touched by this revision.

**(Rev 4: the numbered entries below are rev 3's, and rev 4 numbers
none of its own. It appends three tagged notes inside erratum 3 — a
grep warning, a discharge marker, and one erratum-weight correction to
erratum 3's closing disclosure — each marked `(Rev 4: …)` at its site.
Nothing in errata 1 and 2 is touched, and no numbered entry's text is
altered.)**

### Erratum 1 — §3.6 (and §3.12's AI0 aside): the acknowledgement convention is ASSUMED, not read off the availability row

**What stood** (§3.6, value paragraph, rev 1-2):

> *"The Set is fire-and-forget: MT's availability row (layout 334) gives
> Set O, Read O, Answer O, AI X, and a Set produces no answer, so the
> transport spec waits for none."*

**What now stands.** The fire-and-forget SHAPE is unchanged — the
transport spec still waits for no reply, and no code behaviour moves —
but its ground is restated and given its own evidence grade. An accepted
Set drawing no reply, and a rejected one drawing exactly one "?;", is
an **INHERITED framing convention, ASSUMED**, registered as
`core/driver/ftdx101/doc.go`'s ASSUMED register **entry 9, second half**,
with a per-model Stage W lift (the first write trial's capture with the
port watched between the Set and the read-back). §3.6's Status line now
grades it explicitly, its register-home bullet records that entry 9
carries two halves, and its lift bullet names the port watch. The
availability row is cited for what it does say and disclaimed for what it
does not.

**What was wrong with the old ground.** The availability table's own
header, at layout **236**, reads `Command Function Set Read Ans. AI` over
each of its two command columns: the `Ans.` column marks the EXISTENCE OF
THE COMMAND'S ANSWER FORM — the frame a READ draws — which is why MR, a
read-only command, carries Answer O too (X O O X, layout 331). The column
grounds nothing whatever about what a Set produces. The inference from
Answer O to "a Set produces no answer" is a non sequitur. This manual
(rev 2308-L) states neither half of the convention: it describes Set,
Read and Answer commands and the terminator (layout 190-193, 227-229) and
never says what a radio returns to a Set it honours or to one it cannot,
and its layout-preserved extraction contains no `?` character anywhere.
The convention came from the FT-710's manual, where it is stated as a
general framing rule, and was adopted here without a register entry.

**MANUAL-EVIDENCED and UNAFFECTED:** that ONE frame carries everything —
the 41-byte Set carries the full field block and the tag (layout
1311-1330; geometry-witness `MT,set,*` rows) against MW's 28 bytes
(layout 1352-1367). That half of §3.6 stood and stands.

**Second site, same defect.** §3.12's AI0 aside made the identical
inference ("AI0 is fire-and-forget (its availability row is O O O X,
layout 244)"). It is corrected the same way and tagged to this erratum:
AI0 is sent fire-and-forget on the same inherited convention, not on the
strength of layout 244. The driver's own `Open` doc comment already says
so and cites this matrix section, so leaving the matrix's version
standing would have put the two records in contradiction.

**Records.** M9d-2 milestone review finding **F1**, adjudicated CONFIRMED
in `.superpowers/sdd/m9d2-milestone-review-adjudication.md`; the durable
record is `docs/superpowers/m9d2-baseline-manifest.md` **Note 6**
("matrix §3.6 states the fire-and-forget shape without an evidence line
(ERRATUM OWED)"), which held this defect precisely because the matrix is
not edited mid-milestone. The code fixes landed at `80e0c30`
(`core/driver/ftdx101/doc.go` entry 9, `write.go`'s `mtSetSpec`,
`ftdx101.go`'s AI0-init note). The paired analyses on the fake's side —
`internal/fakedx101/doc.go` register entries 11 and 16 — are CITED, not
absorbed. **This erratum discharges the work Note 6 records as owed.**

### Erratum 2 — §3.12's "layout 75-79": DISPUTED, then CONFIRMED CORRECT by measurement. No text change

**What stood, and what now stands:** the same citation. The two-COM-ports
passage is at **layout 75-79** — 75 the "contains two virtual COM ports"
sentence, 76 "These ports offer the following functions:", 77-78 the two
function bullets, 79 the worked COM5/COM6 example. Layout 73 is a
device-manager string, not the passage.

**Why an erratum with no correction.** During M9d-2's execution this
citation was asserted to be wrong by **three** agents in succession (a
task-7 fix implementer, its re-reviewer, and the Codex milestone
reviewer), all naming 73; a "fix" was applied to
`internal/radiotext/radiotext.go` moving it to 73-76 and raising a
discrepancy warning against this matrix. Two parties measured the
extraction directly and got 75 — the Fable milestone reviewer and the
orchestrator — and it was the orchestrator's own read of layout 72-79
that settled it: **the matrix was right all along**, the task-7
"fix" was wrong, and its re-review's "re-confirmed line 73" was a false
verification. The code was reverted to 75-79 and the discrepancy warning
removed. This entry exists so that the next reader who greps, lands near
73 and starts to "correct" the matrix has the adjudication in front of
them before they do.

**The process lesson, recorded because it generalises:** a disputed
numeric citation is settled by running the measurement, never by counting
verifiers. Three concurring agents were wrong and two measurements were
right.

**Records.** M9d-2 milestone review findings **F2 (CONFIRMED)** and **C3
(REJECTED)**, and process lesson 1, in
`.superpowers/sdd/m9d2-milestone-review-adjudication.md`; triage item (g)
closes INVERTED from the ledger's framing there. The code side is
`internal/radiotext/radiotext.go`'s `ProbeFirmwareNote` comment (cited by
symbol), which now carries the measurement line by line, restored at
`80e0c30`. `core/driver/ftdx101/doc.go` also cites 75-79 and always did.

### Erratum 3 — §1.3.5's MW gate cited a symbol that has since been deleted

**What stood** (§1.3.5, first paragraph, in rev 2's form; rev 1 named the
same symbol without a line range):

> *"— and `cat.Slot.Writable` (`core/cat/slot.go:159-162`) excludes them
> from MW."*

**What now stands:** "— and `cat.Dialect.writableSlot`
(`core/cat/slot.go`, cited by symbol), consulted by `validateMWFields`
(`core/cat/mw.go`), excludes them from MW." **(Rev 4:** grepping
`Writable()` to check this still hits `validateMWFields`' rejection
message in `core/cat/mw.go` — and the golden lines and the frozen test
literals mirroring it — whose wording is FROZEN, not a survival of the
method, as the comment directly above it says.**)**

**Why.** `Slot.Writable` was **DELETED** at the M9d follow-up 2 wave's
dialect-tagged-Slot task (`706f680..efd81d9`; the removal is `1bf00cd`,
"Slot.Writable removed — the write rule spelled once"). The value-form
method answered for the FT-710 on every dialect before the dialect tag
and for the BUILDING dialect after it, and neither is the question the MW
path asks — "will the dialect I am about to write through accept this" —
so an exported predicate that looked like a write-gate answer, one import
from the outbound write gate, was removed rather than kept correct-looking.
The write-direction slot rule is now spelled in exactly one place,
`Dialect.writableSlot`, reached from `validateMWFields`, which serves
both `BuildMWSet` and `AllowedCommand`'s MW grammar check. The
statement §1.3.5 was making — that MW cannot address 5xx/EMG, and that
this half IS manual-evidenced for this radio (layout 1353) whilst the MT
half is project policy — is **unchanged**; only the symbol it points at
moved.

**Cited by symbol, not by line**, per the standing lesson. The old
citation's line range was in fact still exact the day the method was
deleted (`func (s Slot) Writable()` sat at `slot.go:159` from rev 1
through `706f680^`), which is the point: a line number can be perfectly
accurate and still point at nothing, because what invalidated it was the
disappearance of the thing, not a re-flow. A symbol name at least fails
loudly — it greps to zero hits.

**Noted, NOT corrected in this revision** (recorded here rather than
silently fixed): the same paragraph's two other citations have drifted by
a few lines — `Dialect.mtSlotValid` is now at `core/cat/mt.go:118` (cited
as 115) and `validateCombinedMTFields` at `core/cat/mtcombined.go:104`
(cited as 105). Both symbols exist, both still do exactly what §1.3.5
says, and the drift is within a reader's eye-line, so it is left standing
and disclosed here rather than being swept into an erratum that is about
a deletion. A future revision that re-points them should do it by symbol.
**(Rev 4: done — §1.3.5 now cites both by symbol, and its three other
code line-number citations with them, EXCEPT the doc-comment span
`core/cat/mt.go:100-117` and its quoted `:103-106`, which the prose
needs to say which part of a comment it is quoting and which are
current. Both numbers stated above are true when re-measured, but only
one of them is a drift — see the correction immediately below.)**

**(Rev 4 — ERRATUM-WEIGHT CORRECTION to the paragraph above. Recorded
with the full apparatus, because unlike rev 4's other items this one
supersedes an assertion that has SHIPPED.**

**What stood (rev 3's disclosure):** that "the same paragraph's two other
citations have drifted by a few lines", with `validateCombinedMTFields`
"at `core/cat/mtcombined.go:104` (cited as 105)" offered as the second of
the two drifts.

**What now stands (rev 4):** only ONE of the two drifted. Both facts in
the parenthetical are true — the declaration is at `:104`, and §1.3.5 did
cite `:105` — but grouping them under "have drifted" implies `:105` was
therefore stale, and it was not. §1.3.5 read "`Dialect.mtSlotValid`,
**defined at** `core/cat/mt.go:115`, **reached by**
`validateCombinedMTFields` **at** `core/cat/mtcombined.go:105`", and
`mtcombined.go:105` is `if !d.mtSlotValid(m.Slot) {` — the line at which
that function reaches the predicate. Read as the call-site citation its
own sentence makes of it, `:105` was exact at rev 1 and is exact today.
`mtSlotValid`'s 115 → 118 is a genuine drift and stands.

**The measurement.** `validateCombinedMTFields` has sat at
`core/cat/mtcombined.go:104` since `344538c` (29/07/2026), ten days before
this matrix was written; that file's last commit is `344538c`, and
`:104`/`:105` are byte-identical at `432fd4e`, `2745e14`, `706f680` and
today. There was no drift to disclose. Separately, and by the same
measurement, the OTHER stale number in this paragraph's neighbourhood was
not a drift in origin either: `:100-108` was six lines short when rev 1
wrote it, and follow-up 2 later widened the gap by three. See §1.3.5's
span-bound tag.

**The record.** Rev 4's fix-round review,
`.superpowers/sdd/m9d-minors-task2-review.md` findings **F1** (the span
bound) and **F2** (this one), each re-measured against the repository
before this correction was written; the runs on record are that review's
own transcripts and the measurement tables in
`.superpowers/sdd/m9d-minors-task2-report.md`. This is erratum 2's process
lesson — "a disputed numeric citation is settled by running the
measurement, never by counting verifiers" — in the form this case takes:
never by inheriting a previous reader's reading, either.

**Why recorded here and not as a numbered erratum 4.** §6's numbered
entries are rev 3's set, and the section heading and intro — shipped
text — say so, and still do under rev 4's annotation of the heading; a
fourth number would falsify both and would put the correction three
screens from the sentence it corrects. The apparatus is what makes an
erratum, not the numbering, so rev 4 gives the full apparatus at the
site, names it in the header and the Revisions paragraph, and flags it
in §6's heading.**)**

**Records.** The deletion's rationale is in `core/cat/slot.go`'s own
"THERE IS NO `Slot.Writable`" note and in `validateMWFields`' SEAM NOTE
(`core/cat/mw.go`); the wave is `706f680..efd81d9` on `m9d-followups`.
