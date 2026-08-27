<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakeic705` — what it was built from, what it invents, and what nobody has checked

This note is the artefact side of `doc.go`. `doc.go` records what the fake
**does** and where it had to guess; this records what was **read** to build it,
what was deliberately **not** read, every byte it **makes up**, and the state of
the hardware evidence.

## Hardware status: UNVERIFIED

**No IC-705 has ever been asked anything by this project.** Not one frame has
been put to one, and nothing this package does has been observed. Everything
below rests on a printed page, read by two quarantined transcribers, plus a
frame grammar stated as fact in this package's brief.

Two consequences, stated plainly because a simulator is exactly where they get
forgotten:

* Nothing in this package is evidence about an IC-705. Where a test asserts on
  this fake's behaviour, it is asserting on a decision recorded in `doc.go`'s
  ASSUMED register, not on a radio.
* A value this fake answers with may be rendered to a user by `--fake` mode.
  Nobody may read one as what an IC-705 ships with. See "Every byte this
  package invents" below.

## What was read

Four files, and nothing else in this repository:

| path | what it contributed |
| --- | --- |
| `core/civ/ic705/testdata/transcription-b.csv` | The memory-record field table: 18 rows, field widths, the group and channel vocabularies. |
| `core/civ/ic705/testdata/transcription-b.md` | Its method and STOP findings; the note that `1A 01` is a separate, non-memory command. |
| `core/civ/ic705/testdata/geometry-witness.csv` | The independently measured byte positions of every field, 1 to 115. |
| `core/civ/ic705/testdata/geometry-witness.md` | Its position arithmetic and its three STOP findings. |

Both artefacts are transcriptions of **PDF page 19 (printed folio 18) of the
IC-705 `CI-V REFERENCE GUIDE`, document number `A7560-8EX-6`, Jan. 2023** — the
page whose caption reads `• Memory content` / `Command: 1A 00`. Each was
produced by a quarantined reading of rasterised page images at 400 and 600 dpi,
with no text layer consulted at all; each records its own crops, its own second
independent pass and its own disagreements. **They agree on every field extent
and on the total of 115 byte positions**, having been measured separately.

`internal/fakedx101`'s `doc.go`, `fakedx101.go`, `imports_test.go` and
`PROVENANCE.md` were read as a **structural pattern only**: the
pipe-and-goroutine `Radio`, the bounded reassembler, the option shape, the
`Image` contract, the register's form, and the recursive import fence, which is
copied with its `modulePrefix` constant, its `TestIsForbiddenImport` table, its
`TestNoCoreImports` with both vacuity guards, and its red proof
`TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory`. **That
package's protocol is ASCII CAT and this one's is binary CI-V. They share a
shape and not one byte of wire behaviour**, and every frame rule in this package
was written from the grammar rather than adapted from a sibling's code.

## What was deliberately NOT read

Named here because the value of this package rests on the omission, not on good
intentions:

* `core/civ/ic705/*.go` — the profile. **The record layout this fake would have
  been checked against.**
* Everything else under `core/civ` — the accumulator, the framing, the builders,
  the parsers, the allowlist, the BCD helpers.
* `core/driver/ic705` — the driver this fake exists to be run against.
* `core/civ/ic705/testdata/vectors.golden`, `field-ledger.*`,
  `golden-assumptions.csv`, `provenance.md` — every testdata file except the
  four named above.
* The implementation plan.
* Any other fake's content beyond the four structural files listed above.
* Any web resource.

Nothing was consulted from memory about this radio's record layout either. The
two transcripts are the record authority, and the fake's answer to "what is in
byte 40 of a record?" is that it does not know and has no way to find out.

## The one number derived, and how

`RecordLen = 111`, and it is arithmetic rather than semantics:

```
transcription-b.csv    2+2+1+5+2+1+1+1+3+3+3+1+3+8+8+8+47+16 = 115
geometry-witness.csv   measured positions 1 … 115, identical field extents
address field          positions 1-2 memory group, 3-4 memory channel
record                 115 - 4 = 111
```

`TestRecordLenIsTheDiagramsOwnArithmetic` pins the subtraction.

One further specific offset is cited by this package, and it belongs on this
list rather than only inside the argument it supports: **diagram byte 15's
second nibble prints the literal `Fixed`** — both `transcription-b.csv` and
`geometry-witness.csv` record it (the latter as `D4`). It is the source for
the zero-fill justification below (invention 2): the one printed literal in
the whole diagram, and the reason zero is the fill that asserts nothing.

**Beyond RecordLen and this one printed literal, this package knows nothing
else about the 111 bytes** — no field, no other offset, no vocabulary, no
encoding. It stores them, serves them and compares them.

## The STOPs, carried and not resolved

Both transcripts record, independently, that the printed diagram contradicts its
own measured geometry:

1. **Repeated index range, in a different numeral style** — the eighteenth field
   prints black-filled `6` and `52` where the running count gives byte positions
   53-99.
2. **A field whose extent cannot be counted from cells** — that same band draws
   no cell at all, so its 47 positions rest on its own printed end numerals.
3. **Running position and printed numbering disagree** — the nineteenth field
   prints `53~68` where the running count gives 100-115.

Transcription B adds a fourth of its own: **two adjacent fields carry the
identical printed label** over different three-byte ranges, under a
cross-reference that names two different settings.

**None of the four is worked around here, and none has to be.** They are all
questions about *which field lives where*, and this package holds no field
table. The one place a STOP would otherwise bite is the fixture records: putting
a recognisable value at a named position — a memory name, say — would require
choosing between the printed index and the measured position, which is exactly
the choice both transcripts refused to make. `BlankRecord` and `DefaultImage`
therefore place nothing at any named position. That is the whole of the STOPs'
influence on this package, and it is recorded so a later reader does not
"improve" the fixtures into a hidden adjudication.

## Every byte this package invents

Four inventions, each with the reason it is safe and the way it could be got
wrong:

### 1. The transceiver ID payload — `A4`

`parser.go`'s `transceiverIDPayload`. The answer to `19 00` carries one byte and
**that byte is this package's free choice**: neither transcript carries an ID
value (both read the memory-record pages), and no radio has been asked. The
design's own probe records the ID reply in diagnostics and **never matches it**,
so every value is behaviourally equivalent and only honesty is at stake. `A4` —
this radio's own default address — is chosen because a reader of a capture could
not mistake it for evidence about some other radio.

**A consumer that ever matches on this value is testing this package's
invention**, and the match will be a false pass. `doc.go` register entry 7.

### 2. The fixture records — 111 zero bytes

`image.go`'s `BlankRecord`, and every record in `DefaultImage`. The diagram
documents each field's vocabulary and its valid range and **never a shipped
default**, so there is nothing to source a factory value from. Zero is chosen
because it is the only fill that asserts nothing: a legal packed-BCD digit, and
what the diagram's one printed literal — the second nibble of diagram byte 15,
labelled `Fixed` — already prints. `doc.go` register entry 8.

`DefaultImage`'s **shape** is not an invention about a radio but a deliberate
fixture property: sparse, two memory groups, a gap inside one of them, and all
four call channels, so that a walk has something to find, something to skip and
a group boundary to cross.

### 3. The unsolicited frame — `FE FE <to> A4 1A 01 00 00 FD`

`parser.go`'s `buildUnsolicited`, emitted by `WithNeverQuiet`,
`WithBroadcastEvery` and `WithAddressedFlood`. **Made up entirely.** No IC-705
has been observed with transceive on, so what such a radio volunteers, on what
triggers, in what order and with what interleaving against a command in flight
are four unknowns.

Its shape is chosen so that it cannot be mistaken for an answer this radio owes
anyone: `1A 01` is the band stacking register, which transcription B records as
a separate diagram that is **not** a memory record, and which the design's gate
refuses outright. Its two data bytes mean nothing whatever. **A consumer may
assert on its `to` byte — which is the whole point of having two flood options,
one addressed `00` and one `E0` — and must not assert on anything else in it.**
`doc.go` register entry 9.

### 4. The accumulator cap — 512 bytes

`parser.go`'s `maxAccumulatorBytes`. Not a radio claim and not liftable by any
capture: this package's own bounded-input policy, a generous multiple of the
122-byte longest frame this radio can produce, with a one-`FA`-then-discard
resync inherited from `internal/fakeradio`. `doc.go` register entry 6.

### What is NOT an invention

The frame grammar, the two addresses, the two reply codes, the two commands, the
record-length rule, and the group and channel vocabularies. Those are
manual-derived facts and are listed under `doc.go`'s "What is NOT in this
register, and why", so that their absence from the ASSUMED register reads as a
decision. **What a radio does when one of them is violated is a different
question, and every one of those answers is in the register.**

## The import fence

`imports_test.go` forbids every project-internal import, in this directory and
every directory beneath it. It is `internal/fakedx101`'s file copied, and it is
copied **because the rule it enforces forbids importing anything to share**.

The fence is what keeps this package's independence mechanical rather than a
matter of good intentions, and it matters more here than in the sibling: the two
readings being kept apart are two readings of a diagram whose own printed
indices disagree with its own measured geometry. If this fake consulted
`core/civ/ic705`, both sides of every end-to-end test would resolve that
disagreement the same way, and a wrong resolution would be invisible.

The scan **walks subdirectories**, where `internal/fakeradio`'s original uses a
non-recursive `parser.ParseDir(".")`. This package has no subdirectory today;
the fence lands recursive anyway, so anything added later arrives inside it.
`TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory` proves the
scan would bite before any such directory exists, and it was additionally run
**red against a real forbidden import inserted into `state.go`**, which it
caught by file and by path, before that import was reverted.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use.

The underlying diagram is Icom Inc.'s, from the IC-705 `CI-V REFERENCE GUIDE`,
document `A7560-8EX-6`, © 2020–2023. The manual PDF is **not** in this
repository (`docs/fixtures-private/manuals/`, gitignored). What is committed
here is a simulator written against transcriptions of factual protocol data —
frame grammar, addresses, field widths — for interoperability.
