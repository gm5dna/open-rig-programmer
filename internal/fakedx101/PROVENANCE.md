<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `transcription-b.csv` in `internal/fakedx101` — what it is, and why it is a copy

## The file

`internal/fakedx101/transcription-b.csv` is a **byte-identical copy** of

```
core/cat/ftdx101/testdata/transcription-b.csv
```

SHA-256 (both files, 09/08/2026):
`28e17a3658873002e937c9485c96149df1b15613077c1d5aa054ac2d910eb3dd`

It is the ONLY source of this package's EX (MENU) inventory. The generator
under `gen/` reads it and emits `exinventory_gen.go`; `ex.go` expands that into
the address → raw-P4 map the fake answers EX reads from.

## What the artefact is

**Transcription B of the FTDX101MP/FTDX101D CAT Operation Reference Manual's
Table 2 "MENU Chart"**, manual revision 2308-L. It was produced at M9d-1 task 4,
derived **PDF-primary by a quarantined agent** that never opened this
repository, never saw transcription A or the group-boundary ledger, and was told
no row count and no address. Its own header block records the method in full:
every value was read from rasterised pages at 400 dpi, and **no text layer was
consulted at all** — `pdftotext` and every other text-layer tool were left
unrun.

Its delivered schema is

```
p1,p2,p3,p1_label,p2_label,name,digits,text
```

— eight columns, **exactly the ones its brief asked for**, with BARE group
labels and an explicit boolean text flag. That is worth stating because the
FTdx10's B is not like this: that transcription's briefed header was lost to a
mid-task stall/resume and arrived as six columns with the group labels still
wrapped (`01 (RADIO SETTING)`) and no text flag at all, so
`internal/fakedx10/gen` has to strip the wrapper and reconstruct the text flag
from a value-legend prefix. **Neither adaptation is needed here**, and
`core/cat/ftdx101/crosscheck_test.go`'s adjudication (a) records the same thing
from the dialect's side: B's tuple is read straight out of its columns, and its
text flag is B's OWN reading of the chart rather than something inferred from a
legend. B is the stronger witness of the two for that flag.

The file opens with a `#`-commented provenance block (source document, printed
revision code, chart pages, method, verbatim policy). `gen/` reads it with
`csv.Reader.Comment = '#'`, the same way `core/cat/ftdx101/crosscheck_test.go`
and `internal/extable` both do.

**193 data rows, 18 `(P1,P2)` subgroups, exactly one text item** (`04 01 01`
`MY CALL.`, 12 digits).

## COPY, NOT MOVE — and what the copy buys

The dialect's copy **does not move**: `core/cat/ftdx101/crosscheck_test.go:106`
keeps reading `testdata/transcription-b.csv` as one of the **three** artefacts
it binds together (transcription A, transcription B, and the group-boundary
ledger), and moving the file would break the FTdx101 dialect's own three-way
agreement evidence. Nothing under `core/cat/ftdx101/` is touched by this
package.

The duplication is the **mechanism** of the FTdx101's two-source cross-check,
not an accident of layout:

| side | source | generator |
| --- | --- | --- |
| dialect (`core/cat/ftdx101/exinventory_gen.go`) | transcription **A** (`table2.csv`) | `internal/extable` |
| this fake (`exinventory_gen.go`) | transcription **B** (this file) | `internal/fakedx101/gen` (stdlib only) |

`core/transport/ex_crosscheck_ftdx101_test.go` then proves the two inventories
agree — address for address, width for width, shape for shape, and over the
wire, against a `NewD` radio **and** a `NewMP` one. Because the two sides share
**neither the transcription nor the generator**, a defect in either
transcription, or in either generator, surfaces as a **cross-check mismatch**.
If this fake derived its inventory from A, from the dialect, or with `extable`'s
parser, both sides would rest on one reading of the chart and one parser, and a
shared mistake would reproduce itself identically into both tables and be
invisible. That is why `gen/` is stdlib-only and why `imports_test.go`'s
`TestNoCoreImports` walks subdirectories: the fence is what keeps the
independence mechanical rather than a matter of good intentions. It was
deliberately landed one task BEFORE this directory existed, so that the
generator arrived inside a fence rather than in front of one.

**If the cross-check fires, report the diff — do not edit either table to make
it pass.** Which side is wrong, or whether the printed chart itself is, is an
arbitration against the PDF; that is the standing instruction the FT-710's and
the FTdx10's equivalent tests already carry, and the same STOP
`core/cat/ftdx101/crosscheck_test.go` declares for its own three-way legs.

**If the dialect's copy is ever corrected** (by such an arbitration), this copy
must be re-copied and the inventory regenerated.
`core/transport/ex_crosscheck_ftdx101_test.go`'s
`TestFTdx101TranscriptionBCopy_ByteIdenticalToTheDialects` fails until it is, so
the two copies cannot silently diverge into a "B" that is no longer B.

## One inventory, two radios

The FTDX101D and the FTDX101MP share this chart. The manual prints Table 2 once
for both models, and `docs/superpowers/m9d2-capability-matrix.md` §4 (a private evidence record, not in the repository) records
that the manual's whole model-distinguishing surface is three places, none of
them the MENU chart. So this ONE projection serves both `NewD` and `NewMP`
radios, exactly as `core/cat/ftdx101`'s single `exItems` serves both
`DialectD()` and `DialectMP()`. The cross-check drives the wire leg against both
constructors and requires byte-identical answers, which is where that shared
inventory is proved rather than assumed.

## Values-free, tool-derived

This artefact records, for each menu item, **its address, its two group labels,
its printed name, its field width and whether the field is text — and no
setting of any radio**. It is a transcription of a printed chart, produced by
reading rasterised PDF pages; **no FTDX101D and no FTDX101MP has ever been
asked anything by this project**, then or since.

Two consequences worth stating plainly:

* The chart documents each item's **valid range** and its option legends and
  **never a shipped default**, so there is nothing here to source a factory
  value from. The values this fake answers with are therefore **invented** — a
  numeric item answers *n* × `'0'`, the text item answers 12 spaces — by
  `internal/fakeradio`'s convention. That is `doc.go` **register entry 4**, and
  it matters beyond the test suite: `rigprog read --settings --fake --model
  FTdx101D` renders those bytes to a user, who must not read them as what an
  FTdx101 ships with. **Every lift is per model** (doc.go's register preamble):
  a full EX sweep of one model at factory defaults is not evidence about the
  other.
* Only **widths and shapes** are modelled. Nothing in this package interprets
  what a menu item means; the projection drops B's `name` column entirely, and
  the names live in the dialect's inventory, which is the layer that has a
  reason to know them.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use.

The CSV itself carries **no SPDX header, deliberately**: it is a byte-identical
copy of a committed evidential artefact, and adding a line to it would break
exactly the byte identity that makes it that artefact rather than a variant of
it. It is covered by the repository licence like every other file, and this
adjacent note is where that is recorded — the same treatment
`core/cat/ftdx101/table2.csv` and the `testdata/` artefacts get, and the same
treatment `internal/fakedx10/PROVENANCE.md` records for its own copy.

The underlying chart is Yaesu Musen Co., Ltd.'s, from the FTDX101MP/FTDX101D
CAT Operation Reference Manual rev 2308-L. The manual PDF is **not** in this
repository (`docs/fixtures-private/manuals/`, gitignored). What is committed
here is a transcription of factual protocol data — addresses, names and field
widths — for interoperability.
