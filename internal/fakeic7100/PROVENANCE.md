<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakeic7100` — where this fake's knowledge came from

## The short version

Two kinds of fact, from the artefacts named below, and no third place.

**Wire facts** — the frame, the two addresses, the OK and NG codes, the two
commands, the long preamble run — come from the frame diagrams and the command
table quoted in the golden leg's provenance record. They are in `parser.go`.
None of them is a memory-record fact.

**Record facts** — the field-group widths, the printed index runs, the measured
positions, and the two places the page contradicts itself — come from the
semantic transcription and the geometry witness named below. The derivation
they support is written out in full at the top of `records.go`.

**No IC-7100 was ever asked anything by this project.** Nothing in this package
is a reading taken from a radio; every byte it answers with is either a byte a
test seeded into it, a byte a test wrote to it, or a byte assembled from the
printed wire facts above.

## The artefacts

```
core/civ/ic7100/testdata/IC-7100-transcription-b.csv
core/civ/ic7100/testdata/IC-7100-transcription-b.md
core/civ/ic7100/testdata/IC-7100-geometry-witness.csv
core/civ/ic7100/testdata/IC-7100-geometry-witness.md
core/civ/ic7100/testdata/IC-7100-golden-provenance.md
```

All of them describe the same document: the IC-7100 **FULL MANUAL**, revision
`A7085-2EX-5` (May 2021), section 20 `CONTROL COMMAND`, whose printed folios
read `20-n` at PDF page `359 + n`. The memory record is the single diagram on
PDF page 375 (folio 20-16), `• Memory content setting / Command: 1A 00`.

| leg | what it carries | how it was made |
| --- | --- | --- |
| **B** (`IC-7100-transcription-b.*`) | each field's **meaning, printed values and printed index**, plus its measured byte position in the row notes | read by eye off page images rendered at 400 dpi and re-read at 600 dpi through different crop windows; `pdftotext` never run, `tesseract` never used |
| **W** (`IC-7100-geometry-witness.*`) | each group's **measured extent in drawn cell-widths**, counted from each bar's own first cell, with every elision box recorded as an elision box | read by eye off page images at 400 dpi and re-read at 600 dpi through different crop windows; `pdftotext` never run, `tesseract` never used |
| **G** (`IC-7100-golden-provenance.md`) | the **frame diagrams and the command table** — the wire facts — plus the byte-by-byte derivation of the two frames quoted in the tests | read by eye off page images at 400 dpi and re-read at 500 dpi; `pdftotext` run once for navigation only and the source of no recorded value |

Each leg's own header records its method, extent, hazards, STOPs and observed
disagreements in full, and each attests that nothing beyond that one PDF's
rendered page images was consulted.

**These files were not copied into this package**, and nothing here reads them
at run time or at test time. What was taken from them is written into
`records.go` and `parser.go` by hand, with the printed evidence quoted beside
each claim — which is why `imports_test.go`'s fence is the only thing keeping
the independence honest.

## What was actually taken

### From B and W — the record

The full derivation is at the top of `records.go`. In summary: the eighteen
printed field groups' widths add to a **114-byte data area**, whose first three
bytes are the address — the bank byte `(1)` and the two packed-BCD channel bytes
`(2),(3)` — leaving a **111-byte record**. Within that record the transmit
duplicate sits at data-area bytes 52–98 and the sixteen-byte name at 99–114.

The two legs were read for different things and agree where they overlap. B
counts a group at its printed index span; W counts drawn cell-widths and records
every elision box as one. Neither leg asserts a total anywhere — B says so
explicitly — so the total is this package's own arithmetic, and `records_test.go`
re-does it from the group table rather than from the constants.

Three of the eighteen lines needed a decision, and each is recorded in
`records.go` as the decision it is: the transmit duplicate's width is
arithmetic rather than a count (its box has nothing countable in it); the
printed indices and the measured positions part company after `(51)`, and this
package follows the measured positions; and the name is sixteen bytes from the
body text rather than nine from the diagram bar's label. All three are also
register entries in `doc.go` (7 and 8), because a derivation is not a printed
fact however good the arithmetic is.

### From G — the wire

`parser.go` carries the `Controller to IC-7100` and `IC-7100 to controller`
frames, the `OK message`/`NG message` pair, and the two command-table rows. Two
frames from that leg's own byte-by-byte walk are hand-transcribed into
`fake_test.go` and used as independent checks on this package's geometry: the
ten-byte read request `FE FE 88 E0 1A 00 01 00 01 FD`, and the 121-byte
complete-record set whose 114 data bytes this fake must accept if — and only if
— its record geometry is right.

Those two frames were transcribed **from the provenance record's prose walk**,
by hand. The `.golden` vector file itself was not read, so the test's
expectation and the frozen vector are two transcriptions of one printed page
rather than one file compared with itself.

## Hardware status

**UNVERIFIED.** No IC-7100 has ever been connected to this project, read by it
or written to by it. Every entry in `doc.go`'s assumed register stands until a
capture retires it.
