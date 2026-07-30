<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `transcription-b.csv` in `internal/fakedx10` — what it is, and why it is a copy

## The file

`internal/fakedx10/transcription-b.csv` is a **byte-identical copy** of

```
core/cat/ftdx10/testdata/transcription-b.csv
```

SHA-256 (both files, 30/07/2026):
`e412e73c49cc2d388dd9085942588bd05ad5b4957fd809812a798d9dd38e1a78`

It is the ONLY source of this package's EX (MENU) inventory. The generator
under `gen/` reads it and emits `exinventory_gen.go`; `ex.go` expands that into
the address → raw-P4 map the fake answers EX reads from.

## What the artefact is

**Transcription B of the FTdx10 CAT manual's Table 2 "MENU Chart"**, manual
revision 2308-F. It was produced at M9c-4 task 4, derived **PDF-primary by a
quarantined agent** that never opened this repository, never saw transcription A
or the group-boundary ledger, and was told no row count and no address. Its
delivered schema is

```
P1,P2,P3,Function,P4,Digits
```

— full printed group labels in the P1/P2 cells (`01 (RADIO SETTING)`,
`01 (MODE SSB)`), a verbatim P4 value-legend column, and no text flag. That is
not the schema its brief asked for: a mid-task stall/resume lost the briefed
header, and the orchestrator accepted the delivered file **verbatim** — evidence
integrity over format compliance. Every adaptation therefore lives in the code
that reads it, never in the artefact. `core/cat/ftdx10/crosscheck_test.go`
records the three adjudications in full (the schema adapter, label
normalisation, and P4 deliberately not being bound); this package's `gen/`
re-derives the same two things it needs — the wire digits inside the label cells,
and the text discriminator `Digits == 12` **and** `P4` beginning `"Up to"` — from
**B's own columns**.

197 data rows, 18 `(P1,P2)` subgroups, exactly one text item (`04 01 01`
`MY CALL.`).

## COPY, NOT MOVE — and what the copy buys

The dialect's copy **does not move**: `core/cat/ftdx10/crosscheck_test.go:102`
keeps reading `testdata/transcription-b.csv` as one of the three artefacts it
binds together, and moving the file would break the FTdx10 dialect's own
three-way agreement evidence. Nothing under `core/cat/ftdx10/` is touched by
this package.

The duplication is the **mechanism** of the FTdx10's two-source cross-check, not
an accident of layout:

| side | source | generator |
| --- | --- | --- |
| dialect (`core/cat/ftdx10/exinventory_gen.go`) | transcription **A** (`table2.csv`) | `internal/extable` |
| this fake (`exinventory_gen.go`) | transcription **B** (this file) | `internal/fakedx10/gen` (stdlib only) |

`core/transport/ex_crosscheck_ftdx10_test.go` then proves the two inventories
agree — address for address, width for width, and over the wire. Because the two
sides share **neither the transcription nor the generator**, a defect in either
transcription, or in either generator, surfaces as a **cross-check mismatch**. If
this fake derived its inventory from A, from the dialect, or with `extable`'s
parser, both sides would rest on one reading of the chart and one parser, and a
shared mistake would reproduce itself identically into both tables and be
invisible. That is why `gen/` is stdlib-only and why
`imports_test.go`'s `TestNoCoreImports` walks subdirectories: the fence is what
keeps the independence mechanical rather than a matter of good intentions.

**If the cross-check fires, report the diff — do not edit either table to make
it pass.** Which side is wrong, or whether the printed chart itself is, is an
arbitration against the PDF; that is the standing instruction the FT-710's
equivalent test already carries.

**If the dialect's copy is ever corrected** (by such an arbitration), this copy
must be re-copied and the inventory regenerated.
`core/transport/ex_crosscheck_ftdx10_test.go`'s
`TestFTdx10TranscriptionBCopy_ByteIdenticalToTheDialects` fails until it is, so
the two copies cannot silently diverge into a "B" that is no longer B.

## Values-free, tool-derived

This artefact records, for each menu item, **its address, its printed name, its
value legend and its field width — and no setting of any radio**. It is a
transcription of a printed chart, produced by reading a PDF; no FTdx10 was
consulted, then or since. **No FTdx10 hardware has ever been asked anything by
this project.**

Two consequences worth stating plainly:

* The chart documents each item's **valid range** and its option legends and
  **never a shipped default**, so there is nothing here to source a factory
  value from. The values this fake answers with are therefore **invented** — a
  numeric item answers *n* × `'0'`, the text item answers 12 spaces — by
  `internal/fakeradio`'s convention. That is `doc.go` **register entry 4**, and
  it matters beyond the test suite: `rigprog read --settings --fake --model
  FTdx10` renders those bytes to a user, who must not read them as what an
  FTdx10 ships with.
* Only **widths and shapes** are modelled. Nothing in this package interprets
  what a menu item means, and the four printing defects the quarantined agent
  found blind in the chart's value legends (recorded in
  `core/cat/ftdx10/doc.go`) are invisible to every projection made from this
  file, by design.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use.

The CSV itself carries **no SPDX header, deliberately**: it is a byte-identical
copy of a committed evidential artefact, and adding a line to it would break
exactly the byte identity that makes it that artefact rather than a variant of
it. It is covered by the repository licence like every other file, and this
adjacent note is where that is recorded — the same treatment
`core/cat/ftdx10/table2.csv` and the `testdata/` artefacts get.

The underlying chart is Yaesu Musen Co., Ltd.'s, from the FTdx10 CAT Operation
Reference Manual rev 2308-F. The manual PDF is **not** in this repository
(`docs/fixtures-private/manuals/`, gitignored). What is committed here is a
transcription of factual protocol data — addresses, names and field widths — for
interoperability.
