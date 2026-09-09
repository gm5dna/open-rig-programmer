<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakets890` — where every byte it answers comes from

**Hardware status: UNVERIFIED.** No TS-890S has ever been asked anything by
this project. Nothing in this directory is an observation, and this fake
agreeing with `core/driver/ts890` proves the two agree — nothing about the
radio.

## The one source

Every byte offset, field width, legend value and validation rule in this
package is transcribed from the **TS-890S PC Control Command Reference Guide,
revision 1 (January/30/2019)**, and from nothing else. It is cited throughout
as `890:NNNN`, naming a line of the text layout of that document; the manual
itself is gitignored (`docs/fixtures-private/manuals/`), so the citations name
where a chart is rather than linking to it — the convention `core/kw/ma/doc.go`
uses.

This package imports nothing project-internal, in this directory or any
beneath it, and `imports_test.go` enforces that mechanically and recursively.
The reason is in `doc.go` under THE HARD RULE, and it is the whole value of a
fake: a test double built from the production codec cannot disagree with it, so
a systematic bug in that codec — an off-by-one in a field offset, a validation
rule subtly wrong — would be applied identically on both sides of every "send a
command, check the reply" test and would never surface.

## The evidence posture of a memory image

Every byte of every record `image.go` ships is exactly one of five things, and
this note says which:

1. a **printed constant** of the frame — the `MA0` prefix and the `;`. There is
   nothing else in this class, because **this grid has no hard-wired parameter
   at all**: all thirteen are live fields, where the 590 pair's `MR`/`MW` record
   carries three printed constant runs;
2. one of the **two printed example frequencies** — `00007000000`, the front
   matter's `FA` worked example ("Example: Command to set the VFO A to 7 MHz",
   `890:86`, quoted again at `890:118` and `890:125`), and `00014175000` from
   the `AS0` block (`890:342`). **THE 890S PRINTS ONLY TWO.** Any other
   frequency would be an invented byte. `00000000000` is not a third: it is the
   printed *state* of a single memory channel's split-transmission parameters
   ("When reading a single memory channel, all parameters for Split
   Transmission become 0.", `890:3217-3218`);
3. a **printed legend value** — a mode nibble from the `OM` P2 legend the `MA0`
   chart refers to (`890:3976-3992`), the FM narrow flag (`890:3177-3178`,
   `890:3199-3200`), the tone type (`890:3181-3185`), a tone index from the `TN`
   chart (`890:5149-5163`) or the `CN` chart (`890:1354-1369`), the split flag
   (`890:3202-3203`), the lockout flag (`890:3206-3207`);
4. a **name character from the printed `KY` set** (`890:2891-2900`) — upper and
   lower case, digits, space and the punctuation the table prints. The set is
   used ONLY as a source of characters that certainly exist in this radio's
   coding; it is **not** evidence for what a memory name accepts, which is A2.
   The printed dash is deliberately unused: it renders in the layout text as an
   EN DASH, not an ASCII byte, and this book remaps 80h–FFh by Menu 9-01
   (`890:31-43`), so which byte a radio would accept for it is unknown;
5. **A6's blank spelling**, ASCII space, in the blank records alone.

`TestDefaultImage_EveryByteIsAPrintedValue` holds that list mechanically.

## The claim this posture supports, at the strength the design settled on

This is the design's **A22**, quoted verbatim:

> The fake's complete `MA0` record images are compositions of printed VALUES,
> not records any radio has produced.

Every byte value in them is printed, but **no literal complete `MA0` answer
appears in this book**, so the combination is this project's. A fake channel
combines a frequency from `FA`/`AS0`, values from several legends and name
characters from `KY` into a record shape no manual and no radio has ever
produced, and a fake-versus-driver pass over such a record must not be read as
evidence that the combination is one a radio would give.

**Per row: the 890S's image and the 990S's are two compositions**, and two
entries' worth of risk. A22's lift is **a complete real `MA0` answer from that
row**, which hardware item 2 (L-HW-1) produces as a by-product of its read-back
— and it lifts **for that row and no other**.

## The family-level entries an image rides on, by number

An image's assumption surface is the union of these, not one row:

- **A2** — a memory name may contain the printable-ASCII characters this design
  writes, inside 0x20–0x7E and excluding `;`. Neither book prints a charset for
  the memory name; both give only the global ASCII coding rule with 80h–FFh
  remapped by Menu 9-01 (`890:31-43`).
- **A4** — on the 890S, a blank channel's P13 window is blank too, so a blank
  answer is 40 bytes. The note covers "parameters P2 to P12" (`890:3215-3216`)
  and stops; the 990S's equivalent covers "P2 to P18", which is what makes the
  omission visible rather than a reading (erratum **E4**).
- **A5** — the `MA` family spells the channel number back as three zero-padded
  ASCII digits, never space-padded. The ANSWER direction only; the SET
  direction is documented by the chart's own three P1 cells over "000 ~ 119"
  (`890:3167`).
- **A6** — "blank", where the `MA0` notes use it, means ASCII space 0x20.
  Neither `MA0` block defines it; this book defines it for `QR`
  (`890:4362-4363`).
- **A16** — **NOT ASSUMED, DOCUMENTED, and listed here for safety**: reading a
  single memory channel zeroes the second frequency's parameters
  (`890:3217-3218`). Every simplex record this fake ships carries the all-zero
  split side on that authority, so a reader can find the sentence rather than
  re-derive it.
- **A17** — the 890S's minimum `MA0` frame is 40 bytes, a zero-character name.
  Derived from "Up to 10 characters" (`890:3208-3209`) plus the floating `x`
  (`890:3181-3182`); no worked short frame is printed anywhere in the book.
- **A21** — a blank channel carries no meaningful content in its P13 name
  window, so ignoring P13 in the empty predicate discards nothing. A4 assumes
  the window is blank; **A21 is the separate assumption that a residue, if one
  exists, is not channel content** — and it is the one the design acts on,
  because `core/clone`'s `ReadAll` abandons the whole read on any channel error.
- **A22** — the record shape, above.

**A1 is deliberately NOT on this list.** Its pad-and-trim rule is **TS-990S
only**: on this row the terminator floats after the name (`890:3181-3182`), so
`AB ;` and `AB;` are distinct frames, a trailing space is content rather than
padding, and both this fake and `core/kw/ma` carry P13 **verbatim** in both
directions (the Stage 1 close's C-MED-1 reversal). Nothing in an image here
rides on a pad byte.

**A9 and A10 are deliberately NOT on this list either.** No image is shipped
for slots 100–119 — the classes those numbers name are published in no bank of
this registry row and the book explains neither (erratum **E18**) — so nothing
here rides on what a Programmable VFO slot or an E channel holds. An `MA0` read
of one is refused through the same out-of-domain path as any other number. See
`doc.go`'s register entry SLOTS 100-119 ARE NOT SERVED, which is what a later
reader has to change on the day they lift.

## The two blank fixtures, and why there are two

The blank channel is this fake's most valuable fixture and its most
assumption-laden, and each blank image is bound to an **explicit choice**.
Neither is "an unspecified name window": both are concrete byte strings.

- **The 40-byte frame with NO name window** — `MA0`, the slot, 33 ASCII spaces
  and the terminator. It is A4's reading, and it is what **every unpopulated
  channel** answers, through the one path `handleMA0Read` takes.
- **A blank record carrying a RESIDUAL NAME in P13** — channel `004` of the
  default image. A21 says a radio may produce one and the empty predicate must
  be shown ignoring it, so the fixture exists to be read back through the driver
  as "empty channel" and **not** as "parse error".

The 990S needs no second fixture: its blank channel is `MA0` + slot + 50 spaces
+ `;`, which is *printed* (modulo A6). The 890S's is not, which is why this row
has two.

## What is deliberately small, and why

The default image is five channels. That is a consequence of the posture rather
than a gap: with two printed frequencies available, a larger image would repeat
itself, and an image that populated a hundred channels would turn every "this
channel is blank" assertion into a fixture accident. What it does carry is
SHAPE — a plain simplex channel with no name at all, a second holding the other
printed value of every live two-value byte and a full ten-character name, a
split channel whose P8–P10 side is populated, a fourth carrying the last of the
four tone types, and the blank record with its residue.

## The EX menu inventory — one chart, one copy, one cross-check

The second evidence leg of a fake in this project is its **independently
transcribed menu inventory** — transcription B, copied into the fake that serves
it, so that the fake's inventory and the codec's come from two different
readings of one chart and a transport cross-check can assert they agree.

- `transcription-b-890s.csv` is a **byte copy** of the quarantined artefact
  committed at `core/kw/ma/testdata/transcription-b-890s.csv`, SHA-256
  `f45226d2bce269c76b966de596909b3783757477a5b09a66c3a955d9e6d12560`. It is a
  COPY, not a move: `core/kw/ma/crosscheck_test.go` keeps reading the original
  as an artefact it binds and hashes by name.
  `core/transport/ex_crosscheck_ts890_test.go` asserts the copy is still
  byte-identical to it, because "the codec from A versus the fake from B" holds
  only for as long as this side's copy really is B.
- `exinventory.go` embeds the copy and projects it at init. It imports nothing
  project-internal — in particular not `internal/extable`, which generates the
  CODEC's side — so a shared parsing bug cannot reproduce itself identically
  into both inventories and be invisible. `imports_test.go` enforces that
  recursively, this directory and every one beneath it. There is **no
  generator** and no generated file (the plan's P19).
- `core/transport/ex_crosscheck_ts890_test.go` compares the two sides address
  for address and width for width, and drives every address over the wire.

**If the cross-check ever fires: report the diff. Do NOT edit either table to
make it pass.** Which side is wrong — or whether the printed chart is — is an
arbitration against the PDF, and an edit that merely restores agreement destroys
the evidence the agreement was worth.

### The four excluded addresses, and why this side excludes them itself

The chart prints four Advanced Menu rows with **real addresses** and the body
"Does not correspond to a command" — `1 00 23` Touchscreen Calibration,
`1 00 24` Software License Agreement, `1 00 25` Important Notices concerning
Free Open Source, `1 00 26` About Various Software License Agreements
(`890:2273-2280`). That is erratum **E16**, and it is why this row is the
registry's first `ParameterlessExcluded` profile with four addresses.

Transcription B **carries all four** — they are printed, they are addressed, and
the evidence leg's ledger counts them — but the production inventory does not:
`internal/extable` excludes them by address and then checks the count that
follows. **A literal projection of B would therefore be four entries longer than
the codec's inventory, and the cross-check would fail on a correct
implementation.**

So `exinventory.go` applies its **own, independently written exclusion**, the
four addresses spelt out in its source with `890:2273-2280` beside them, while
transcription B and its ledger keep all four rows. That is what preserves the
independent-evidence chain rather than deriving this side's exclusion from the
other side's. `TestEXDefaults_OmitsExactlyTheFourExcludedAddresses` asserts the
projection omits exactly those four and nothing else, as a set difference
against B's own addresses rather than as a count — a count-only assertion is
satisfied by an inventory that dropped the wrong four rows.

### What the menu VALUES are, and what they are not

Each menu's default raw P5 is its **printed width in `0` bytes** — an INVENTED
placeholder, `doc.go`'s register entry THE EX MENU VALUES ARE INVENTED. The
parameter lists print each menu's available settings and never a shipped default
anywhere. What is transcribed is the **width**, and only the width.

Two family-level entries the EX surface rides on, by number:

- **A19** — an `EX` answer's P5 never exceeds the width the parameter list
  prints for that menu number. The book prints widths per item class — 3
  normally, 4 for PF keys, 0–15 for a power-on message, 0–10 for screen-saver
  text (`890:1916-1921`) — and no general ceiling. It is a CEILING, so a short
  answer is admitted on both sides and `WithEXSetting` can script one.
- **A2** — the printable-ASCII charset, extended from P13's name field to EX's
  P5. `WithEXSetting` stores what it is given; the charset is enforced by the
  codec, on the side that has to read a real radio's bytes.

The **text flag** of transcription B is deliberately **not projected** into this
fake's table. `core/kw/ma/crosscheck_test.go` records the repository's ruling
that the flag is a CONVENTION and the digits are the datum: A and B were briefed
with different definitions of "text row", and on this chart the two legs differ
at **two** addresses — Contest Number (`0/05/12`) and Reference Oscillator
Calibration (`1/00/05`), both flagged text by B and not by A — where the ruling
is that only a row reading "Up to N alphanumeric characters" is a text row,
which makes A right at both. A fake that projected B's flag would answer spaces
at two addresses the repository has ruled numeric. See `exinventory.go`'s
`widthFor`.
