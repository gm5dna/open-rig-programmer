<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakets590` — where every byte it answers comes from

**Hardware status: UNVERIFIED.** No TS-590S and no TS-590SG has ever been
asked anything by this project. Nothing in this directory is an observation,
and this fake agreeing with `core/driver/ts590` proves the two agree — nothing
about either radio.

## The one source

Every byte offset, field width, legend value and validation rule in this
package is transcribed from the **TS-590S/TS-590SG PC Control Command
Reference Guide, revision 3**, and from nothing else. It is cited throughout
as `590:NNNN`, naming a line of the text layout of that document; the manual
itself is gitignored (`docs/fixtures-private/manuals/`), so the citations name
where a chart is rather than linking to it — the convention `core/kw/doc.go`
uses.

This package imports nothing project-internal, in this directory or any
beneath it, and `imports_test.go` enforces that mechanically and recursively.
The reason is in `doc.go` under THE HARD RULE, and it is the whole value of a
fake: a test double built from the production codec cannot disagree with it,
so a systematic bug in that codec — an off-by-one in a field offset, a
validation rule subtly wrong — would be applied identically on both sides of
every "send a command, check the reply" test and would never surface.

## The evidence posture of a memory image

Every byte of every record `image.go` ships is exactly one of four things, and
this note says which:

1. a **printed constant** from the position charts — `000` (P10, `590:1473`),
   `0` (P12, `590:1480`), `000000000` (P13, `590:1482`), the `MR`/`MW` prefix,
   the `;`;
2. the **printed example frequency** `00014195000` — "For example, enter
   00014195000 for 14.195 MHz." (`590:965`), which is the only worked
   frequency this document prints anywhere;
3. a **printed legend value** — a mode nibble from the `MD` legend
   (`590:1353-1363`), the `DA` flag (`590:447-449`), a tone mode from P7's
   four (`590:1464-1467`), a tone index from the `TN` chart (`590:2296-2306`)
   or the `CN` chart (`590:416-426`), byte 28's FILTER A/B
   (`590:1476-1477`), P14's FM Normal/Narrow (`590:1484-1485`), P15's lockout
   (`590:1487-1488`);
4. the **name field**, filled with **eight spaces** — the one name state this
   document describes: "If the selected channel is empty, P4 ~ P15 will be 0
   and P16 will be blank." (`590:1492-1493`).

`TestDefaultImage_EveryByteIsAPrintedValue` holds that list mechanically.

## The claim this posture supports, at the strength the design settled on

This is the design's **A27**, quoted verbatim:

> Every byte value in a fake image is a constant, example or legend value
> printed in the radio's own PC-command document. The 50-byte RECORD is a
> synthetic composition of those values: no MR, MW or MC frame is printed
> anywhere in either book, so no image's cross-field combination has ever been
> printed or observed. These are not observed contents and not factory
> defaults.

A27's own lift is the first real `MR` answer a registry row gives, and it
lifts **for that row and no other** — three rows, three first answers. Until a
row has one, this fake proves the codec self-consistent for that row and
proves nothing about that radio.

## The family-level entries an image rides on, by number

An image's assumption surface is the union of these, not one row:

- **A1** — the memory name is padded to 8 bytes with spaces on write and
  right-trimmed on read. Neither book states the rule for P16.
  *(This package does not itself apply A1: it stores and answers the eight
  bytes of the field verbatim, precisely so that a round trip through the
  codec can contradict A1 rather than confirm it on both sides.)*
- **A3** — an empty channel's P16 comes back as 8 spaces. `590:1492-1493` says
  P16 "will be blank" without defining blank.
- **A4** — an `MR` of an empty channel answers, with P4–P15 zero, rather than
  rejecting. Documented for the 590SG (`590:1492-1493`); it is the **TS-480**
  half of this entry that is unlifted, and that is the other fake's problem.
- **A10** — `MR`/`MW` P2 accepts `'0'` for a channel below 100 on the 590
  pair. Documented for `MC` Set (`590:1334-1335`); the memory charts only say
  "refer to the `MC` command".
- **A18a** — a mode nibble `0` in an `MR` **answer** is the empty-channel
  marker, tested as "P4–P15 all zero" and not as "P5 is zero". Documentary
  fact on this pair (`590:1492-1493`), kept in the register so that no later
  reader restores an earlier draft's refusal.
- **A27** — the record shape, above.

**A11 is deliberately NOT on this list.** Stuart ruled on 05/09/2026 that the
TS-590SG's extension channels 110–119 are omitted from the driver's published
banks until A11 is lifted, so this fake ships **no image for that range at
all** and rides on nothing about what an extension channel holds. An `MR`,
`MW` or `MC` naming 110–119 is refused here the same way any out-of-domain
channel number on any row is refused — one path, no special case. See
`doc.go`'s register entry THE SG'S EXTENSION CHANNELS ARE NOT SERVED, which
is what a later reader has to change on the day A11 lifts.

## What is deliberately small, and why

The default image is four channels. That is a consequence of the posture
rather than a gap: with one printed frequency and one name state available,
a larger image would repeat itself, and an image that populated a hundred
channels would turn every "this channel is empty" assertion into a fixture
accident. What it does carry is SHAPE — an ordinary memory channel, a second
one holding the other printed value of every live two-value byte, a non-FM
channel with DATA mode on, and BOTH halves of one section-defined channel, so
that the paired read at `590:1449-1451` meets two stored records rather than
one and a silence.

## The EX menu inventory — two charts, two copies, one cross-check

The second evidence leg of a fake in this project is its **independently
transcribed menu inventory** — transcription B, copied into the fake that
serves it, so that the fake's inventory and the codec's come from two
different readings of one chart and a transport cross-check can assert they
agree. The 590 pair needs **two** of them, because the two siblings print two
disjoint menu charts in one book, over colliding addresses with different
meanings (`590:564`, `590:744`).

Both have landed:

- `transcription-b-590s.csv` and `transcription-b-590sg.csv` are **byte copies**
  of the quarantined artefacts committed at `core/kw/ts590/testdata/`. They are
  COPIES, not moves: `core/kw/ts590/crosscheck_test.go` keeps reading the
  originals as artefacts it binds and hashes by name.
  `core/transport/ex_crosscheck_ts590_test.go` asserts the copies are still
  byte-identical to them, because "the codec from A versus the fake from B"
  holds only for as long as this side's copy really is B.
- `internal/fakets590/gen` projects each copy into `exinventory590s_gen.go` and
  `exinventory590sg_gen.go`. It imports nothing project-internal — in
  particular not `internal/extable`, which generates the CODEC's side — so a
  shared parsing bug cannot reproduce itself identically into both inventories
  and be invisible. `imports_test.go` enforces that recursively, `gen/`
  included.
- `core/transport/ex_crosscheck_ts590_test.go` compares the two sides address
  for address and width for width, on both rows, and drives every address over
  the wire.

**If the cross-check ever fires: report the diff. Do NOT edit either table to
make it pass.** Which side is wrong — or whether the printed chart is — is an
arbitration against the PDF, and an edit that merely restores agreement
destroys the evidence the agreement was worth.

### What the menu VALUES are, and what they are not

Each menu's default raw P5 is its **printed width in `0` bytes** — an INVENTED
placeholder, `doc.go`'s register entry THE EX MENU VALUES ARE INVENTED. Neither
parameter list prints a shipped default anywhere; both print each menu's
available settings. What is transcribed is the **width**, and only the width.

Two family-level entries the EX surface rides on, by number:

- **A19** — an `EX` answer's P5 never exceeds the width the parameter list
  prints for that menu number. Both books call P5 "variable length" and print
  no ceiling (`590:554-556`). It is a CEILING, so a short answer is admitted on
  both sides and `WithEXSetting` can script one.
- **A2** — the printable-ASCII charset, bounded at `0x7E`. `WithEXSetting`
  stores what it is given; the charset is enforced by the codec, on the side
  that has to read a real radio's bytes.

The **text flag** of transcription B is deliberately **not projected** into this
fake's tables. `core/kw/ts590/crosscheck_test.go` records the orchestrator's
ruling that the flag is a CONVENTION and the digits are the datum: A and B were
briefed with different definitions of "text row", and the one address where they
differ on this pair — the SG's menu 000, "Version information (4 ASCII
characters) read only" — is transcribed by A as a fixed-width numeric row. A
fake that projected B's flag would answer spaces at an address the repository
has ruled numeric. See `gen/main.go`'s `widthToken`.
