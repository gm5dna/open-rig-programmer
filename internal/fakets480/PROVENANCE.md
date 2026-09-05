<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakets480` — where every byte it answers comes from

**Hardware status: UNVERIFIED.** No TS-480 has ever been asked anything by
this project. Nothing in this directory is an observation, and this fake
agreeing with `core/driver/ts480` proves the two agree — nothing about the
radio. On this row that sentence is doing more work than on any other in the
programme: the row is **built and not registered**, and its release gate is a
single wire observation (**A4**, lift `L-HW-3`).

## The one source

Every byte offset, field width, legend value and validation rule in this
package is transcribed from the **PC CONTROL COMMAND REFERENCE FOR THE
TS-480HX/SAT TRANSCEIVER (2003)**, and from nothing else. It is cited
throughout as `480:NNNN`, naming a line of the text layout of that document;
the manual itself is gitignored (`docs/fixtures-private/manuals/`), so the
citations name where a chart is rather than linking to it — the convention
`core/kw/doc.go` uses.

This package imports nothing project-internal, in this directory or any
beneath it, and `imports_test.go` enforces that mechanically and recursively.
The reason is in `doc.go` under THE HARD RULE, and it is the whole value of a
fake: a test double built from the production codec cannot disagree with it,
so a systematic bug in that codec — an off-by-one in a field offset, a
validation rule subtly wrong — would be applied identically on both sides of
every "send a command, check the reply" test and would never surface.

**It also imports nothing from `internal/fakets590`, and that matters here
more than the general rule does.** The two books print the same 50-byte grid
and give four of its bytes different jobs, print different tone-mode and AI
legends, and give the `O;` token different causes (erratum `E13`). A single
borrowed table would be a silent wrong answer, not a missing one.

## The evidence posture of a memory image

Every byte of every record `image.go` ships is exactly one of four things, and
this note says which:

1. a **printed constant** from the position charts — `0` (P2, `480:910`),
   `000` (P10, `480:929`), `0` (P11, `480:931`), `0` (P12, `480:933`),
   `000000000` (P13, `480:935`), `0` (P15, `480:939`), the `MR`/`MW` prefix,
   the `;`;
2. the **printed example frequency** `00014195000` — "For example,
   00014195000 for 14.195 MHz." (`480:549`, and again at `480:572` and
   `480:1746`);
3. a **printed legend value** — a mode nibble from the `MD` legend
   (`480:843-854`), the lockout's `0`/`1` (`480:920`), a tone mode from P7's
   three (`480:922`), a tone index from `TN`'s printed range (`480:1557`) or
   `CN`'s (`480:337`), a step index printed in **both** of `ST`'s
   mode-conditional legends (`480:1494-1500`);
4. the **name field**, filled with **eight spaces**.

`TestDefaultImage_EveryByteIsAPrintedValue` holds that list mechanically.

**The name field is weaker evidence here than on the 590 pair.** That book at
least prints one name state — "If the selected channel is empty … P16 will be
blank" (`590:1492-1493`) — and this one prints nothing about an empty channel
anywhere; all it says of P16 is "A maximum of 8 characters." (`480:941`).
Eight spaces on this row is **A3**, and A3's TS-480 half is unlifted.

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
  right-trimmed on read. Neither book states the rule for P16; the 480's `KY`
  gives a same-document precedent for a different command (`480:785-787`).
  *(This package does not itself apply A1: it stores and answers the eight
  bytes of the field verbatim, precisely so that a round trip through the
  codec can contradict A1 rather than confirm it on both sides.)*
- **A3** — an empty channel's P16 comes back as 8 spaces. On this row the
  document says nothing about an empty channel at all, so A3 here is a whole
  assumption rather than a reading of an under-defined word.
- **A4** — an `MR` of an empty channel answers, with P4–P15 zero, rather than
  rejecting. Documented for the 590SG (`590:1492-1493`); **entirely unprinted
  for this radio**, and this is the row where that half is unlifted. It is
  **the release gate**: lift `L-HW-3` is one read of one unwritten channel on
  a real TS-480, repeated as hardware item 3 specifies because a single trial
  is inconclusive.
- **A24** — a printed-fixed byte is REQUIRED ON PARSE on this row, not merely
  emitted on build. Every record this fake answers carries all six hard-wired
  runs verbatim, and this book's own general note permits a Set to fill a
  parameter "not applicable to this transceiver" with "any character except
  the ASCII control codes (00 to 1Fh) and the terminator (;)"
  (`480:108-111`) — so a radio answering one of those bytes differently would
  not necessarily be faulty. Strictness is this programme's choice, and a
  single counter-example on real hardware turns it into "accepted and
  normalised". **TS-480 row only.**
- **A27** — the record shape, above.

**A10 is deliberately NOT on this list.** Its own text says the TS-480 is
unclaimed by it and stays so: `MR`/`MW` P2 is a printed constant on this
radio (`480:910`, `480:953`), not a hundreds digit with a space-or-zero
spelling rule, so there is nothing here for A10 to be about. This fake
**refuses** the space spelling the 590 pair accept —
`TestMR_RefusesABankByteOtherThanZero`.

**A18a is deliberately NOT on this list either.** It is documentary fact on
the 590 pair alone — a mode nibble `0` in an `MR` answer read as the
empty-channel marker — and this book prints nothing of the kind. Nibble `0`
here is "No mode (Not used for the TS-480)" (`480:843`). The TS-480 half of
that question is A4, above.

## What is deliberately small, and why

The default image is three channels, and only their **P1=0 halves**. That is a
consequence of the posture rather than a gap:

- With one printed frequency and one name state available, a larger image
  would repeat itself; and an image that populated a hundred channels would
  turn every "this channel is empty" assertion into a fixture accident — on
  the one row where that assertion **is** the release gate.
- Decision 15 gives this row **one flat MEM bank, `00`…`99`, and no scan
  bank**: the record has no bank field (`480:827`), so the `P1=1` half of a
  channel is not a published slot and **P19 ships no image for a slot the row
  does not publish**. The book's own start/end overload on channels 90–99
  (`480:943-944`) is still SERVED, because the frame is a documented read; it
  answers the zero record.

What the image does carry is SHAPE: every printed value of both live legends
— the lockout's two (`480:920`) and the tone mode's three (`480:922`) — plus
two mode nibbles and two step indices, so that no read behaviour on any of
them is a fixture accident.

## The EX menu inventory — one copy, one generator, one cross-check

The second evidence leg of a fake in this project is its **independently
transcribed menu inventory** — transcription B, copied into the fake that
serves it, so that the fake's inventory and the codec's come from two
different readings of one chart and a transport cross-check can assert they
agree. This book prints the best-documented menu legend of any radio in this
programme (`480:424-539`), and the whole of it is now transcribed twice.

- `transcription-b-480.csv` is a **byte copy** of the quarantined artefact
  committed at `core/kw/ts480/testdata/`. It is a COPY, not a move:
  `core/kw/ts480/crosscheck_test.go` keeps reading the original as an artefact
  it binds and hashes by name.
  `core/transport/ex_crosscheck_ts480_test.go` asserts the copy is still
  byte-identical to it, because "the codec from A versus the fake from B" holds
  only for as long as this side's copy really is B.
- `internal/fakets480/gen` projects the copy into `exinventory_gen.go`. It
  imports nothing project-internal — in particular not `internal/extable`,
  which generates the CODEC's side — so a shared parsing bug cannot reproduce
  itself identically into both inventories and be invisible.
  `imports_test.go` enforces that recursively, `gen/` included.
- `core/transport/ex_crosscheck_ts480_test.go` compares the two sides address
  for address and width for width, and drives every address over the wire.

**If the cross-check ever fires: report the diff. Do NOT edit either table to
make it pass.** Which side is wrong — or whether the printed chart is — is an
arbitration against the PDF, and an edit that merely restores agreement
destroys the evidence the agreement was worth.

### One printed defect rides through unchanged

The EX block's prose lists the two-digit menus as "Menu No. 32, 35 and 48 ~ 52"
(`480:411`) and **omits menu 034**, whose grid row reaches the chart's second
parameter column all the same. Both quarantined derivations read the GRID and
record 034 as two digits; `core/kw/ts480`'s errata and its `crosscheck_test.go`
pin it as a defect of the printed block. This fake answers **two** bytes there,
because that is what the transcription says, and no TS-480 has been asked which
the radio answers. Three faithful readings of one chart agree perfectly, so no
comparison in this repository can catch this class — which is why the state is
pinned rather than merely obtained.

### What the menu VALUES are, and what they are not

Each menu's default raw P5 is its **printed width in `0` bytes** — an INVENTED
placeholder, `doc.go`'s register entry THE EX MENU VALUES ARE INVENTED. The
parameter list prints each menu's available settings and never a shipped
default. What is transcribed is the **width**, and only the width.

One address is not arbitrary, and the coincidence is recorded rather than leant
on: at menu 000 the book prints a complete worked ANSWER, `EX00000000;
(Display illumination OFF)` (`480:415`), whose P5 is exactly this placeholder
byte. It is the only complete EX answer either Kenwood book prints, and it is
the only byte sequence on this surface that can be checked against a literal.

Two family-level entries the EX surface rides on, by number:

- **A19** — an `EX` answer's P5 never exceeds the width the parameter list
  prints for that menu number. Both books call P5 "variable length" and print
  no ceiling (`480:409-411`). It is a CEILING, so a short answer is admitted on
  both sides and `WithEXSetting` can script one.
- **A2** — the printable-ASCII charset, bounded at `0x7E`. `WithEXSetting`
  stores what it is given; the charset is enforced by the codec, on the side
  that has to read a real radio's bytes.

The **text flag** of transcription B is deliberately **not projected** into this
fake's table. `core/kw/ts480/crosscheck_test.go` records the orchestrator's
ruling that the flag is a CONVENTION and the digits are the datum: B applied a
structural test and flagged menus 048-052, whose merged cell prints the NUMERIC
legend `00 ~ 99 (2-digit)`, while transcription A marks no text row at all on
this chart. A fake that projected B's flag would answer spaces at five
addresses the repository has ruled numeric. See `gen/main.go`'s `widthToken`.
