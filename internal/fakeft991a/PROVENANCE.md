<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `transcription-b.csv` in `internal/fakeft991a` — what it is, and why it is a copy

## The file

`internal/fakeft991a/transcription-b.csv` is a **byte-identical copy** of

```
core/cat/ft991a/testdata/transcription-b.csv
```

SHA-256 (both files, 05/09/2026):
`676002b6a751bbe3c4ea93d97d7ee6f2dd1fd0fc439d912e3195d02f8499313f`

That is the same hash `core/cat/ft991a/crosscheck_test.go`'s
`frozenEvidenceSHA256` records for the dialect's copy. The two facts are one
fact, and this note records it a second time only so that a reader of THIS
directory can check the copy without first learning the dialect's test layout.

It is the ONLY source of this package's EX (MENU) inventory. `exinventory.go`
embeds it and projects it at init; `ex.go` expands that into the
address → raw-P4 map the fake answers EX reads from.

## What the artefact is

**Transcription B of the FT-991A CAT Operation Reference Manual's MENU chart**,
manual revision 1711-D. It was derived **PDF-primary by a quarantined agent**
that never opened this repository, never saw transcription A, and was told no
row count and no address. Its delivered schema is

```
menu_number,name,digits
```

— three columns, and its companion `transcription-b.md` (frozen beside the
dialect's copy) records the method: PDF pages 8–10 re-rendered at 600 and 900
dpi, the number and Digits columns composited side by side with the parameter
column dropped, read in two independent passes, plus a third pass over the
Function column after the two disagreed on one name (`GM DISPLY`).

**153 data rows, 001–153 with no gap and no repeat, no text row, and one row
that names no field at all.**

## This chart is not the FT-891's, though the two files have the same header

`internal/fakeft891` is this package's architectural exemplar, and its own
transcription B carries the **same three column names**, because both were
produced under one brief. That resemblance is the reason `exinventory.go` is
written rather than copied: three structural facts differ, and each is a
property of the printed chart rather than a choice made here.

* **THE ADDRESS IS A SINGLE COMPONENT.** This chart prints a **three-digit**
  MENU Number that is the whole address — `087` is `P1=87`, with P2 and P3 zero
  (`cat.EXAddressSingle`) — where the FT-891's four digits are a `(P1,P2)` pair.
  So there are **no groups**: the FT-891 generator's group key, its per-group
  widths string and its two group rules have no counterpart, and the projection
  is a flat list of one entry per address. What replaces them is the property
  this chart does have and theirs does not: **one consecutive run of addresses**
  from `001`, which `checkRun` enforces rather than assumes.
  Because the FT-891's header is identical, the header check cannot separate the
  two radios' files — **the address width does**: `parseMenuNumber` refuses
  anything that is not exactly three ASCII digits, so an FT-891 row is rejected
  at its first data line rather than read as a plausible FT-991A address.
* **ONE ROW PRINTS NO PARAMETER AT ALL.** `087 RADIO ID` prints a single hyphen
  for its Digits and ten spaced hyphens for its parameter legend, both confirmed
  at 900 dpi (`transcription-b.md` §2(b)). It is transcribed and counted, and
  **excluded** from the inventory: a menu number naming no field is not an
  address an EX frame could read or write. The FT-891's chart has no such row.
  **The inventory is therefore 152 items for a 153-row chart, on both sides.**
* **THE WIDTH ALPHABET RUNS TO EIGHT.** From exactly one row, `151 PRESET
  FREQUENCY`, whose `00030000 ~ 47000000` parameter is eight digits wide. The
  FT-891's alphabet stops at 5 and the FTdx10's at 4. Four rows are five wide
  (`027`, `064`, `065`, `083`). `core/cat/ft991a/crosscheck_test.go` pins the
  widest address and width from the **A** side as literals; `exinventory_test.go`'s
  `TestParseB_TheOnlyEightWideRowIs151` pins them from the **B** side.

The fourth fact is shared, and it is a silence rather than a difference:
**there is no text row, and no column that could identify one.** B carries no
parameter-legend column, so the FTdx10 generator's text discriminator —
`Digits == 12` **and** a P4 cell beginning `"Up to"` — is not merely unnecessary
here, it is **inexpressible**. Every row is projected as numeric, which is a
statement about the delivered SCHEMA and not a claim about the radio. (The
quarantined agent did look and did report finding no free-text legend,
`transcription-b.md` §2(a) — but that is a sentence in a report, not a column in
the file, and the generator can only read the file.) What would catch a genuine
text row is the cross-check: the dialect's inventory is generated from A, which
HAS a text column, and `core/transport/ex_crosscheck_ft991a_test.go` compares
the two shapes.

Consequently the width alphabet this package's generator emits is `'1'..'8'`,
with no `'T'` token at any width. A width outside `1..8` is a REFUSAL in the
generator, never a token invented to fit.

## The `?` and the `-` are one printed hyphen — plan decision P18

Row 087's Digits cell **prints a hyphen**. The two derivations spell it
differently, because they were written under different transcription
conventions:

| side | source | spelling of 087's Digits | who keys on it |
| --- | --- | --- | --- |
| dialect | transcription **A** (`table2.csv`) | `-`, the raw glyph | `internal/extable`'s `ParameterlessExcluded`, `ft991aProfile.ParameterlessAddresses` |
| this fake | transcription **B** (this file) | `?`, the brief's token for a non-integer cell | `exinventory.go`'s `parameterlessToken` and `parameterlessAddrs` |

Both are right, and neither is consulted from the other. **Each generator reads
the spelling of the artefact it generates from** — the project's standing rule
that a bound is consulted from the same place as its datum. Teaching this
generator `extable`'s `-` would break that rule in the one direction that
matters: it would make this side of the cross-check depend on the other side's
reading of the page. The milestone's plan states the ruling (P18) rather than
leaving the two generators, written weeks apart, to meet it separately, and the
cross-check of the two transcriptions normalises `?` to `-` on this one address
at comparison time — never by editing either artefact.

The rule is enforced in **both** directions, with a red proof each way
(`exinventory_test.go`): a `?` on `087` is excluded; a `?` on any other address is
**refused**. A third direction is enforced too — `087` with a numeric cell is
refused — so that a declaration which no longer describes its artefact fails
loudly instead of silently dropping a real address.

## COPY, NOT MOVE — and what the copy buys

The dialect's copy **does not move**: `core/cat/ft991a/crosscheck_test.go` keeps
reading `testdata/transcription-b.csv` as one of the artefacts it binds
together, and hashes that path by name. Moving the file would break the FT-991A
dialect's own agreement evidence. Nothing under `core/cat/ft991a/` is touched by
this package.

The duplication is the **mechanism** of the FT-991A's two-source cross-check,
not an accident of layout:

| side | source | generator |
| --- | --- | --- |
| dialect (`core/cat/ft991a/exinventory_gen.go`) | transcription **A** (`table2.csv`) | `internal/extable` |
| this fake (`exItems`) | transcription **B** (this file) | `exinventory.go` (stdlib only) |

`core/transport/ex_crosscheck_ft991a_test.go` then proves the two inventories
agree — address for address and width for width, and over the wire. Because the
two sides share **neither the transcription nor the generator**, a defect in
either transcription, or in either generator, surfaces as a **cross-check
mismatch**. If this fake derived its inventory from A, from the dialect, or with
`extable`'s parser, both sides would rest on one reading of the chart and one
parser, and a shared mistake would reproduce itself identically into both tables
and be invisible. That is why `exinventory.go` is stdlib-only and why
`imports_test.go`'s `TestNoCoreImports` walks this directory and every one
beneath it.

**If the cross-check fires, report the diff — do not edit either table to make
it pass.** Which side is wrong, or whether the printed chart itself is, is an
arbitration against the PDF; that is the standing instruction the FT-710's, the
FTdx10's and the FT-891's equivalent tests already carry.

**If the dialect's copy is ever corrected** (by such an arbitration), this copy
must be re-copied and the inventory regenerated.
`core/transport/ex_crosscheck_ft991a_test.go`'s
`TestFT991ATranscriptionBCopy_ByteIdenticalToTheDialects` fails until it is, so
the two copies cannot silently diverge into a "B" that is no longer B.

## What no comparison can catch

All the derivations read the same printed chart, so a defect **printed in the
chart** is transcribed faithfully by each and is invisible to every comparison,
including this fake's. The FT-991A's known printing defects are recorded from
the A side (`core/cat/ft991a/testdata/ledger.md`) and from the B side
(`transcription-b.md` §4), not here. What this fake answers is what the
transcription says, in every case, because no FT-991A has been asked what it
answers.

## Values-free, tool-derived

This artefact records, for each menu item, **its address, its printed name and
its field width — and no setting of any radio**. It is a transcription of a
printed chart, produced by reading a PDF; no FT-991A was consulted, then or
since. **No FT-991A hardware has ever been asked anything by this project.**

Two consequences worth stating plainly:

* The chart documents each item's **valid range** and its option legends and
  **never a shipped default**, so there is nothing here to source a factory
  value from. The values this fake answers with are therefore **invented** —
  a numeric item answers *n* × `'0'` — by `internal/fakeradio`'s convention.
  That is `doc.go`'s register entry **THE EX MENU VALUES ARE INVENTED**, and it
  matters beyond the test suite: once task 15a registers the model,
  `rigprog read --settings --fake --model FT-991A` will render those bytes to a
  user, who must not read them as what an FT-991A ships with.
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
variant of it — and would break
`core/transport/ex_crosscheck_ft991a_test.go`'s
`TestFT991ATranscriptionBCopy_ByteIdenticalToTheDialects`, the check that binds
this copy to the dialect's. It is covered by the repository licence like every
other file, and this adjacent note is where that is recorded — the same
treatment `core/cat/ft991a/table2.csv` and the `testdata/` artefacts get.

The underlying chart is Yaesu Musen Co., Ltd.'s, from the FT-991A CAT Operation
Reference Manual rev 1711-D. The manual PDF is **not** in this repository
(`docs/fixtures-private/manuals/`, gitignored). What is committed here is a
transcription of factual protocol data — addresses, names and field widths — for
interoperability.
