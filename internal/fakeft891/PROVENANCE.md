<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `transcription-b.csv` in `internal/fakeft891` — what it is, and why it is a copy

## The file

`internal/fakeft891/transcription-b.csv` is a **byte-identical copy** of

```
core/cat/ft891/testdata/transcription-b.csv
```

SHA-256 (both files, 04/09/2026):
`5bb41932712c8c11237ce9e1a44b782d3c133f54170cb2147a9bc6b23cf54bb5`

That is the same hash `core/cat/ft891/crosscheck_test.go`'s
`frozenEvidenceSHA256` records for the dialect's copy, transcribed there from
commit `adf3d21`'s freeze block. The two facts are one fact, and this note
records it a second time only so that a reader of THIS directory can check the
copy without first learning the dialect's test layout.

It is the ONLY source of this package's EX (MENU) inventory. The generator
under `gen/` reads it and emits `exinventory_gen.go`; `ex.go` expands that into
the address → raw-P4 map the fake answers EX reads from.

## What the artefact is

**Transcription B of the FT-891 CAT Operation Reference Book's MENU chart**,
manual revision 1909-C. It was produced at this milestone's Stage 0 evidence
wave, derived **PDF-primary by a quarantined agent** that never opened this
repository, never saw transcription A or the group-boundary ledger, and was
told no row count and no address. Its delivered schema is

```
menu_number,name,digits
```

— three columns, and its companion `transcription-b.md` (frozen beside it)
records the method: the chart pages re-rendered at 600 dpi, read in two
independent passes, the second of which discarded the legend column entirely
and read the MENU number and the Digits column alone.

**159 data rows, 18 `P1` groups, no text row.**

## The FT-891's schema is not the FTdx10's, and the differences are structural

`internal/fakedx10` is this package's architectural exemplar, and its copy of
its own radio's transcription B carries six columns
(`P1,P2,P3,Function,P4,Digits`). Three differences change what a generator over
this file can and cannot do, and each is a property of the printed chart rather
than a choice made here:

* **THE ADDRESS IS A PAIR.** This chart prints a four-digit MENU Number whose
  two halves are the whole address — `0803` is `(P1,P2) = (08,03)` — and every
  row's P3 is 0 (`cat.EXAddressPair`). B carries those four digits verbatim in
  one column, where the FTdx10's B carries three.
* **THERE ARE NO GROUP LABELS.** The chart prints no label columns at all, so
  there is no `NN (LABEL)` wrapper to split and nothing to normalise. The
  FTdx10 generator's `splitWrapped` has no counterpart here.
* **THERE IS NO TEXT ROW, AND NO COLUMN THAT COULD IDENTIFY ONE.** B carries no
  parameter-legend column, so the FTdx10 generator's text discriminator —
  `Digits == 12` **and** a P4 cell beginning `"Up to"` — is not merely
  unnecessary here, it is **inexpressible**. Every row of this file is
  therefore projected as numeric, which is a statement about the delivered
  SCHEMA and not a claim about the radio: if this chart did carry a text item,
  this file could not say so, and the projection would be wrong in a way no
  test in this package could see. What would catch it is the cross-check —
  the dialect's inventory is generated from A, which HAS a text column, and
  `core/transport/ex_crosscheck_ft891_test.go` compares the two shapes.

Consequently the width alphabet this package's generator emits is
`'1'..'5'` — five raw ASCII digits at the widest — with no `'T'` token at any
width. The `'5'` comes from exactly two rows, `0803 OTHER DISP` and
`0804 OTHER SHIFT`, whose signed `-3000 Hz - 0 - +3000 Hz` parameter counts its
sign as a digit; `core/cat/ft891/crosscheck_test.go` pins the same two
addresses from the A side as literals. A width outside `1..5` is a REFUSAL in
the generator, never a token invented to fit.

## COPY, NOT MOVE — and what the copy buys

The dialect's copy **does not move**: `core/cat/ft891/crosscheck_test.go` keeps
reading `testdata/transcription-b.csv` as one of the three artefacts it binds
together, and its `TestQuarantinedEvidenceFrozen` hashes that path by name.
Moving the file would break the FT-891 dialect's own three-way agreement
evidence. Nothing under `core/cat/ft891/` is touched by this package.

The duplication is the **mechanism** of the FT-891's two-source cross-check,
not an accident of layout:

| side | source | generator |
| --- | --- | --- |
| dialect (`core/cat/ft891/exinventory_gen.go`) | transcription **A** (`table2.csv`) | `internal/extable` |
| this fake (`exinventory_gen.go`) | transcription **B** (this file) | `internal/fakeft891/gen` (stdlib only) |

`core/transport/ex_crosscheck_ft891_test.go` then proves the two inventories
agree — address for address and width for width, and over the wire. Because the
two sides share **neither the transcription nor the generator**, a defect in
either transcription, or in either generator, surfaces as a **cross-check
mismatch**. If this fake derived its inventory from A, from the dialect, or with
`extable`'s parser, both sides would rest on one reading of the chart and one
parser, and a shared mistake would reproduce itself identically into both tables
and be invisible. That is why `gen/` is stdlib-only and why `imports_test.go`'s
`TestNoCoreImports` walks subdirectories: the fence is what keeps the
independence mechanical rather than a matter of good intentions.

**If the cross-check fires, report the diff — do not edit either table to make
it pass.** Which side is wrong, or whether the printed chart itself is, is an
arbitration against the PDF; that is the standing instruction the FT-710's and
the FTdx10's equivalent tests already carry.

**If the dialect's copy is ever corrected** (by such an arbitration), this copy
must be re-copied and the inventory regenerated.
`core/transport/ex_crosscheck_ft891_test.go`'s
`TestFT891TranscriptionBCopy_ByteIdenticalToTheDialects` fails until it is, so
the two copies cannot silently diverge into a "B" that is no longer B.

## What no comparison can catch

All three of this chart's derivations read the same printed chart, so a defect
PRINTED in the chart is transcribed faithfully by all three and is invisible to
every comparison — including this fake's. One such defect is known and recorded:
`0905 RPT SHIFT 50MHz` prints Digits 1 against a legend that needs four, where
its twin `0904 RPT SHIFT 28MHz` prints 4 for the same shape of legend.
`core/cat/ft891/crosscheck_test.go` pins the quirk from the A side and
`testdata/transcription-b.md` records three separate re-reads of that cell from
the B side. **This fake answers the printed 1**, because that is what the
transcription says, and no FT-891 has been asked which of the two the radio
answers. It is a recorded, deliberate state, not an unnoticed one.

## Values-free, tool-derived

This artefact records, for each menu item, **its address, its printed name and
its field width — and no setting of any radio**. It is a transcription of a
printed chart, produced by reading a PDF; no FT-891 was consulted, then or
since. **No FT-891 hardware has ever been asked anything by this project.**

Two consequences worth stating plainly:

* The chart documents each item's **valid range** and its option legends and
  **never a shipped default**, so there is nothing here to source a factory
  value from. The values this fake answers with are therefore **invented** —
  a numeric item answers *n* × `'0'` — by `internal/fakeradio`'s convention.
  That is `doc.go`'s register entry **THE EX MENU VALUES ARE INVENTED**, and it
  matters beyond the test suite: `rigprog read --settings --fake --model FT-891`
  renders those bytes to a user, who must not read them as what an FT-891 ships
  with.
* Only **widths and shapes** are modelled. Nothing in this package interprets
  what a menu item means; the names live in the dialect's inventory, which is
  the layer that has a reason to know them.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use.

The CSV itself carries **no SPDX header, deliberately**: it is a byte-identical
copy of a committed, hash-frozen evidential artefact, and adding a line to it
would break exactly the byte identity that makes it that artefact rather than a
variant of it — and would break `TestQuarantinedEvidenceFrozen`'s sibling check
here. It is covered by the repository licence like every other file, and this
adjacent note is where that is recorded — the same treatment
`core/cat/ft891/table2.csv` and the `testdata/` artefacts get.

The underlying chart is Yaesu Musen Co., Ltd.'s, from the FT-891 CAT Operation
Reference Book rev 1909-C. The manual PDF is **not** in this repository
(`docs/fixtures-private/manuals/`, gitignored). What is committed here is a
transcription of factual protocol data — addresses, names and field widths — for
interoperability.
