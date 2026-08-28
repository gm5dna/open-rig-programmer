<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakeic9700` — where this fake's knowledge came from

## The short version

Two kinds of fact, from two places, and no third place.

**Wire facts** — the frame, the addresses, the codes, the two commands, the
broadcast address, the preamble run — were **quoted to this package in its own
brief** out of the IC-9700 CI-V Reference Guide (PDF p.4/folio 3, p.6/folio 5,
p.13/folio 12), precisely so that this package would not have to go and find
them. They are in `parser.go`. None of them is a memory-record fact.

**Record facts** — two of them, and only two — came from the two artefacts named
below, and from nothing else.

**No IC-9700 was ever asked anything by this project**, then or since. Nothing
in this package is a reading taken from a radio; every byte it answers with is
either a byte a test seeded into it or a byte assembled from the printed wire
facts above.

## The two artefacts

```
core/civ/ic9700/testdata/ic9700-transcription-b.csv
core/civ/ic9700/testdata/ic9700-transcription-b.md
core/civ/ic9700/testdata/ic9700-geometry-witness.csv
core/civ/ic9700/testdata/ic9700-geometry-witness.md
```

They are **two independent transcriptions of the same printed memory-record
diagrams** in the IC-9700 CI-V Reference Guide, revision `A7508-3EX-4` (Mar.
2023), PDF pages 15 and 16 (printed folios 14 and 15).

| leg | what it carries | how it was made |
| --- | --- | --- |
| **B** (`ic9700-transcription-b.*`) | each field's **meaning and values** — its printed label, its printed value list, its encoding | read by eye off page images rendered at 400 dpi and re-read at 600 dpi through different crop windows; `pdftotext` never run, `tesseract` never used |
| **W** (`ic9700-geometry-witness.*`) | each field's **measured byte and nibble positions** — cell ordinals counted from each diagram's own first cell | read by eye off page images rendered at 400 dpi and re-read at 600 dpi through different crop windows and a different number of crops; `pdftotext` never run, `tesseract` never used |

Each leg's own header block records its method, its extent, its hazards, its
STOPs and its observed disagreements in full, and each attests that nothing
beyond that one PDF's rendered page images was consulted.

**These files were not copied into this package.** They stay where they are;
`internal/fakedx101` had to copy its artefact because a generator reads it at
build time, and this package has no generator — what it took from B and W is two
sentences' worth of fact, written into `image.go` by hand with the printed
evidence quoted alongside. Nothing here reads those CSVs at run time or at test
time, which is also why `imports_test.go`'s fence is the only thing keeping the
independence honest.

## What was actually taken

Exactly two facts. They are set out at the top of `image.go` with the printed
evidence beside them; in summary:

1. **Where a 1A 00 data block names its channel.** Three bytes: a frequency-band
   byte, then a two-byte memory channel number in packed BCD.
   - B supplies the meanings: field `①` *Frequency band setting*, one byte,
     `01: 144 MHz frequency band | 02: 430 MHz frequency band | 03: 1.2 GHz
     frequency band`; fields `②, ③` *Memory channel number*, two bytes,
     `0001 ~ 0099: Memory channel 1 to 99` … `0106, 0107: Call channel C1, C2`,
     with the note that the printed values are four decimal digits and each byte
     cell is split by a dotted rule into two nibble cells.
   - W supplies the positions: D1 cell 1 for `①`, cells 2 and 3 for `②, ③`.
   - **The two legs agree here.** B's measured byte positions 1 and 2–3 are the
     same numbers as W's cell ordinals 1 and 2–3, because W records the printed
     index and the measured position as diverging only from `⑩` onward. This is
     the only part of the record this package needs, and it is a part the two
     legs are not in dispute about.

2. **What the clearing form looks like.** Under the printed heading *"To clear
   the memory channel contents on 1A 00:"* the page gives the same block with
   `②, ③ : Memory channel (0001~0099)`, `④ : "FF,"` and `⑤ ~ : None`. Both legs
   put field `④` at position 4, immediately after the address — B carries the
   `④ : "FF,"` entry in its `④` row, and W quotes the whole line `④ : "FF,"
   ⑤ ~ :None` among its observed disagreements. So a clear is the three address
   bytes, one `FF`, and nothing more.

Columns consumed, so that what was read is checkable: from **B**,
`field_index`, `label_verbatim`, `width_bytes`, `encoding`, `values_verbatim`
and `notes` (the last for the measured byte positions), on the `①`, `②, ③` and
`④` rows of D1; from **W**, `diagram_id`, `field_index`, `first_byte`,
`first_nibble`, `last_byte`, `last_nibble` and `notes`, on the same three rows,
plus the two prose sections *"What the diagrams do and do not number"* and
*"Position arithmetic, per diagram"*. No position and no value was taken from
any D2–D5 row (W's `D2,③` row is cited once below, and only for the disputed
index it records, never for a position), and nothing at all was taken from any
field past `④`.

## What was deliberately NOT taken — the record length STOP

**The two legs do not agree on how long a memory record is, and this package
does not resolve it.**

- **B** measures **114 bytes** — one per drawn byte cell, with each elided group
  taken at the length of its printed index range — and says in terms that *"the
  document prints no total and no byte addresses"*, so 114 is its measurement
  and not a printed figure.
- **W** counts what the picture draws: **38 cells**, several of them dotted
  continuation boxes standing for an unstated number of omitted bytes. It
  records seven separate STOPs over exactly this (its STOPs 1–7) rather than
  reconcile the two counts, and tabulates the divergence field by field: the
  printed indices and the measured cell ordinals agree through `⑤` and part
  company from `⑩` onward, the gap widening at each continuation box.

Neither leg claims a length the other confirms. Choosing one would be asserting
a fact the evidence does not carry, and a driver test that passed against a fake
built on that choice would be evidence of agreement with a guess.

So **this package has no record length.** It serves the length it was handed —
whatever `WithSlot` seeded, or whatever `WithRecordLength` said — and it judges
a write's length only once one of those has told it what length it serves. Given
neither, it enforces nothing, and `TestMemorySet_WithNoLengthKnownAnyLengthIsAccepted`
pins that as a behaviour rather than leaving it as an accident.

Two smaller things follow from the same STOP and are recorded here rather than
worked around:

- **No byte offset past position 4 is known here**, and none is used. The
  divergence above means an offset read off B and an offset read off W would not
  be the same offset anywhere past `⑤`.
- **The index printed over field `④`'s expansion inset is in dispute on the
  page itself**, and the two legs record that dispute differently. The inset
  drawn under the heading `④ Select memory setting` is captioned `③`. B keeps
  `④` in its `field_index` — the heading and the diagram bracket both say 4 —
  and records the caption verbatim as its STOP 1; W gives the inset a row of its
  own, `D2,③`, exactly as printed above the box, as its STOP 8. This package
  needs neither numeral: **both legs put the byte at position 4** — B's `④` row
  measures byte position 4, W's `D1,④` row measures cell 4 — and position 4 is
  the only thing the clearing form requires.

## The one invention, named as such

`recordPadByte` (`00`) is what a record is padded with when `WithRecordLength`
asks for answers longer than what was seeded. It is **not** claimed to be what
any slot of any IC-9700 holds. It is a byte that is neither the preamble nor the
end-of-message byte, so that a padded answer is still a legal frame, and that is
its whole justification. Anything a consumer renders to a user out of a padded
answer is this fake's filler and must never be read as a radio's contents.

## Why the fence, and why it landed first

`imports_test.go` forbids this package — and every directory beneath it —
importing anything from this module, and it was written and committed before any
of the code it guards.

A fake is worth having only because it is the **other** witness. The driver says
what it believes an IC-9700 will say; the fake says what it believes an IC-9700
will say; the test means something because the two beliefs were formed
separately, from separate readings of the same printed pages. Let this package
import `core/civ/ic9700`, its profile, its golden vectors or its field ledger,
and the two witnesses collapse into one: a systematic misreading of the record
would agree with itself end to end, and every test would go green while proving
nothing at all. The fence is what makes that separation mechanical rather than a
matter of good intentions.

It walks subdirectories rather than reading one directory, and
`TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory` proves it
would bite in a subdirectory that does not exist yet — so anything added later
arrives inside a fence rather than in front of one.

**If a driver test fires against this fake, report the disagreement — do not
edit this package to make it pass.** Which side is wrong, or whether the printed
diagram itself is, is an arbitration against the PDF and against B and W, and
that arbitration belongs upstairs. Editing the fake to agree with the driver
destroys the only thing the fake was for.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use, and every Go file in the package carries it as a line
comment.

The underlying diagrams are Icom Inc.'s, from the IC-9700 CI-V Reference Guide
rev `A7508-3EX-4`, © 2019–2023 Icom Inc. The manual PDF is **not** in this
repository (`docs/fixtures-private/manuals/`, gitignored). What is recorded here
is a handful of factual protocol positions — which byte names a band, which two
bytes name a channel, which byte the clearing form sets — for interoperability.
