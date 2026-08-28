<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakeic905` — where every byte in this package came from

This package was written **quarantined**. What follows is the complete list of
what was read, the complete list of what was deliberately not read, what is
assumed, and the hardware status.

## Hardware status: UNVERIFIED

**No IC-905 has ever been asked anything by this project.** Not by this task,
not by any task before it. Every byte in this package descends from rasterised
pages of one PDF, read by eye. Nothing here has been put to a radio.

## What was read

### 1. The wire facts, supplied in the task brief

Quoted there from the **IC-905 CI-V REFERENCE GUIDE, PDF p.3 (folio 2)**,
`◇ About the data format`, which prints four complete frame diagrams, and from
**PDF p.6 (folio 5)**'s command table:

```
Preamble           FE FE            ("Preamble code (fixed)")
End of message     FD               ("End of message code (fixed)")
Radio address      AC               ("Transceiver's default address")
Controller address E0               ("Controller's (PC's) default address")
Frame, PC -> radio FE FE AC E0 <cn> [<sc>] [data] FD
Frame, radio -> PC FE FE E0 AC <cn> [<sc>] [data] FD
OK  (ack)          FE FE E0 AC FB FD   ("OK code (fixed)")
NG  (reject)       FE FE E0 AC FA FD   ("NG code (fixed)")
Read transceiver ID   cn=19 sc=00, request carries NO data bytes
Memory contents       cn=1A 00, symmetric send/read
Broadcast (transceive) frames carry to = 00
```

together with the **record-length rejection rule, stated without the layout**:
this model's records come in exactly two lengths, and a `1A 00` set whose record
length is not the length the fake holds for that channel is answered `FA`.

These are **framing** facts. They were the only protocol constants the brief
supplied, and the record-layout wall stood throughout.

### 2. `core/civ/ic905/testdata/ic905-transcription-b.csv` and `.md`

Transcription **leg B** of the memory-record data block: the section headed
`• Memory content` / `Command: 1A 00` on **PDF page 19 (printed folio 18)** of
the same guide, revision `A7711-9EX-2` (printed on the back page above
`© 2023–2024  Icom Inc.      May 2024`), with the character tables on PDF page
20 (folio 19).

Produced **PDF-primary by a quarantined agent**: 400 dpi first pass, an
independent 600 dpi second pass at different crop windows and different
enlargements, `pdftotext` **never run**, `tesseract` available and **not used**.
The second pass disagreed with the first in **no cell**.

21 data rows: 17 for diagram **D1** (the drawn record) and 4 for **D2** (the
`To clear the memory channel contents on 1A 00:` block, which has no drawn
diagram).

What this package took from it:

| taken | used for |
| --- | --- |
| the 17 D1 rows — printed index, verbatim legend label, first/last byte, width | `state.go`'s `d1RecordFields`, **naming only** |
| the 4 D2 rows, and `⑤: “FF,” ⑥ ~ : None` | `state.go`'s `d2ClearFields`, `clearFormLen`, `clearFormByte`; the clear form's refusal in `parser.go` |
| `①, ②` and `③, ④`: two bytes each, values `00 00 ~ 00 99` and `01 00` | `addressBytes = 4`, and therefore that the record is bytes 5–68 |
| the printed value lists on those two rows | the BCD reading in `bcd2` — **ASSUMED**, see below |
| `1A 01` = `• Band stacking register` (PDF p.20); `1A 05` heads the set-mode command table (PDF p.9) | the comments on `refusedPrefixes` |

### 3. `core/civ/ic905/testdata/ic905-geometry-witness.csv` and `.md`

An **independent** raster measurement of the same data block by a different
quarantined leg — 300 dpi to locate, 400 dpi to read, 600 dpi to re-read — which
counted drawn cells, bracket arms and ellipsis boxes rather than reading the
legend, and recorded first/last **byte and nibble** for each of the 18 measured
fields.

It was used as a **cross-check on leg B, not as a second source**: every offset
in `d1RecordFields` was transcribed from leg B and then read against the
witness's measured positions. **The two agree on every offset, every width and
every boundary**, and both state the same arithmetic independently —
`2+2+1+5+2+1+1+1+3+3+3+1+3+8+8+8+16 = 68`, equal to the highest printed index,
with no gap and no overlap. `TestTheRecordFieldTableIsGaplessAndSumsTo68` pins
that arithmetic against the table.

### 4. `internal/fakedx101/` — STRUCTURAL exemplar only

Read for **shape**, never for protocol: its `Radio`/`Option`/`Port`/`Close`
layout, its `net.Pipe` and `Close` reasoning, its separation of state, options,
image and parser into files, and its `imports_test.go`, which was **copied**
(the recursive version, with its vacuity guards and its red proof) and adapted
only in its package clause and its prose.

**Its protocol is ASCII CAT and this one is binary CI-V; not one protocol byte,
offset or rule crossed over.** `fakedx101` is not imported — it could not be,
and `TestNoCoreImports` proves it is not.

## What was NOT read

Deliberately, and this is the whole basis on which this package's agreement with
the dialect counts as evidence:

- `core/civ/ic905/*.go` — **not opened**
- `core/driver/ic905/*.go` — **not opened**
- `core/civ/*.go` — **not opened**
- any golden file, including `core/civ/ic905/testdata/ic905-vectors.golden` —
  **not opened**
- `core/civ/ic905/testdata/ic905-field-ledger.{csv,md}` and
  `ic905-golden-assumptions.csv` / `ic905-golden-provenance.md`, which sat in
  the same directory as the two artefacts that were read — **not opened**
- any plan, spec, capability matrix or review — **not opened**

Where this package and `core/civ/ic905` agree, they agree because two readings
of one printed document landed in the same place. If either had consulted the
other, the agreement would be a tautology.

## STOPs

**One**, and it is not this package's own: **STOP 1**, carried by *both*
artefacts, in the `notes` of both affected rows.

PDF page 19's right-hand legend prints, on consecutive lines:

```
⑯~⑱: Repeater tone frequency setting
⑲~㉑: Repeater tone frequency setting
ⓘ See “Repeater tone/tone squelch frequency setting.” (p. 23)
```

Two distinct, adjacent, non-overlapping three-byte ranges are given word-for-word
identical labels, whilst the pointer they share names *two* settings. Both legs
read it twice, at different resolutions and different crop windows, and both
confirmed the lines are identical in wording, spacing, capitalisation and
punctuation. Neither repaired it.

**It did not block this task.** This fake never interprets a record byte, so
what bytes 16–18 and 19–21 mean is a question it never asks; the *geometry* is
unambiguous in both artefacts and that is all this package uses. The two labels
are transcribed as printed into `d1RecordFields`, unrepaired, and
`TestTheTwoRepeatedLabelsAreTranscribedAsPrinted` fails if anyone tidies one of
them.

**No STOP of this task's own arose.** The two artefacts did not contradict each
other on any fact this package needed, and the brief determined every behaviour
it asked for.

### Observed disagreements carried through, not resolved

Both artefacts list several, and the ones that reach `state.go` are transcribed
as printed rather than reconciled:

- `(8 characters, fixed.)` at `37~44` against `(8 characters, fixed)` at `29~36`
  and `45~52` — the stray full stop is printed, and is in the table.
- `Duplex offset frequency setting` in the label against `Duplex Offset` in the
  pointer on the next line — the label is what is in the table.
- Indices 52, 53 and 68 are printed as circled numerals that Unicode has no
  forms for. The geometry witness's convention is followed: **every** index is
  written in plain numerals with its printed separator preserved, so that the
  table does not falsely suggest two printed styles. The circle is uniform
  across all eighteen indices and is recorded here, once.

## The ASSUMED register

The full register with its reasoning and lifts is in `doc.go`, which is where a
reader of the code will be. In brief, ten entries:

| # | assumed | lift |
| --- | --- | --- |
| 1 | a `1A 00` read of an unoccupied channel is answered `FA` | **ic905-R-14** |
| 2 | the transceive broadcast form, `to = 00` | **ic905-R-12** |
| 3 | the controller-addressed flood form, `to = E0` | the same capture as ic905-R-12 |
| 4 | the default identity token's value (`DE AD`) | a `19 00` exchange with a real radio |
| 5 | the default image: which channels are occupied, and 64 zero bytes each | a full memory read of a factory-reset radio |
| 6 | the two address fields read as packed BCD (`01 00` = group 100) | a printed statement of the encoding, or a capture |
| 7 | silence, not `FA`, for a frame addressed elsewhere | a frame to another address on a shared bus |
| 8 | refusing the printed clear form (`… FF FD`) | the day this tier sends a clear |
| 9 | refusing `1A 01`, `1A 02`, `1A 05`, `09`, `0A`, `0B`, `A0` | the tier learning to send any of them |
| 10 | the maximum frame length, and `FA` for an over-length run | an over-long run put to a real radio |

Entries **2, 3, 5** are the ones that can mislead a *user* rather than only a
test: a fake rig rendered to a screen shows entry 5's zeros, and a capture
window shows entries 2 and 3's invented frames. None of them is a fact about an
IC-905.

Entries **8** and **9** are worth reading twice, because they are the opposite
of a limitation: at least two of the refused commands are demonstrably real, and
the fake refuses them anyway. **It models a radio that refuses everything this
tier refuses to send**, so that a tier which starts sending one has to come back
here and say so.

### What is deliberately *not* in the register

The framing (`FE FE`, `FD`, `AC`, `E0`, `FB`, `FA`, the two frame orders,
`1A 00`'s symmetry) and the record geometry (68 bytes, indices 1–68, address
first) are **printed**. They are cited where used and are not assumptions. The
record-length rejection rule was supplied as a wire fact, stated without the
layout, and is not one either.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use.

The underlying reference is Icom Inc.'s, from the IC-905 CI-V REFERENCE GUIDE
rev `A7711-9EX-2`. The guide's PDF is **not** in this repository
(`docs/fixtures-private/manuals/`, gitignored). What is committed here is an
independent implementation of factual protocol data — framing bytes, command
codes and field widths — for interoperability.
