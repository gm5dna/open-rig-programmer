<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakets990` — where every byte it answers comes from

**Hardware status: UNVERIFIED.** No TS-990S has ever been asked anything by
this project. Nothing in this directory is an observation, and this fake
agreeing with `core/driver/ts990` proves the two agree — nothing about the
radio.

## The one source

Every byte offset, field width, legend value and validation rule in this
package is transcribed from the **TS-990S PC Control Command Reference Guide,
revision 2 (January/30/2019)**, and from nothing else — the sibling book
included. It is cited throughout as `990:NNNN`, naming a line of the text
layout of that document; the manual itself is gitignored
(`docs/fixtures-private/manuals/`), so the citations name where a chart is
rather than linking to it — the convention `core/kw/ma/doc.go` uses.

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
   at all**: all eighteen are live fields, where the 590 pair's `MR`/`MW` record
   carries three printed constant runs;
2. one of the **three printed example frequencies** — `00007000000`, the front
   matter's `FA` worked example ("Example: Command to set the Main Band VFO to
   7 MHz", `990:86`, quoted again at `990:119` and `990:126`), `00014175000`
   from the `AS2` block ("for example, 14.175 MHz is displayed as
   00014175000", `990:344-345`) and `00014195000` from `FA`'s own parameter
   note ("For example, enter 00014195000 for 14.195 MHz", `990:2352`, and
   `FB`'s at `990:2374`). **THE 890S PRINTS ONLY TWO**, which is why no image
   may be carried between the rows. Any other frequency would be an invented
   byte. `00000000000` is not a fourth: it is the printed *state* of a single
   memory channel's frequency-2 parameters ("When reading a single memory
   channel, all parameters for frequency 2 become 0.", `990:2964-2965`);
3. a **printed legend value** — the memory channel type (`990:2898-2900`), a
   mode nibble from the `OM` P2 legend the `MA0` chart refers to
   (`990:3707-3730`, **twenty-four values** where the 890S's prints sixteen),
   the FM wide/narrow flags (`990:2913-2914`, `990:2933-2934`), the tone
   functions (`990:2916-2919`, `990:2936-2939`), a tone index from the `TN`
   chart (`990:4949-4974`) or the `CN` chart (`990:1240-1267`), the split flag
   (`990:2947-2948`), the dual-reception flag (`990:2950-2951`), the scan
   lockout (`990:2952-2954` — **1/2, not 0/1**, erratum **E8**);
4. a **name character from the printed `KY` set** (`990:2770-2778`) — upper and
   lower case, digits, space and the punctuation the table prints. The set is
   used ONLY as a source of characters that certainly exist in this radio's
   coding; it is **not** evidence for what a memory name accepts, which is A2.
   The printed dash is deliberately unused: it renders in the layout text as an
   EN DASH, not an ASCII byte, and this book remaps 80h–FFh by Menu 9-01
   (`990:32-39`), so which byte a radio would accept for it is unknown;
5. **A6's blank spelling**, ASCII space, in the blank record and in the unused
   cells of a name window.

`TestDefaultImage_EveryByteIsAPrintedValue` holds that list mechanically.

## The claim this posture supports, at the strength the design settled on

This is the design's **A22**, quoted verbatim:

> The fake's complete `MA0` record images are compositions of printed VALUES,
> not records any radio has produced.

Every byte value in them is printed, but **no literal complete `MA0` answer
appears in this book**, so the combination is this project's. A fake channel
combines a frequency from `FA`/`AS2`, values from several legends and name
characters from `KY` into a record shape no manual and no radio has ever
produced, and a fake-versus-driver pass over such a record must not be read as
evidence that the combination is one a radio would give.

**Per row: the 890S's image and the 990S's are two compositions**, and two
entries' worth of risk. A22's lift is **a complete real `MA0` answer from that
row**, which hardware item 2 (L-HW-1) produces as a by-product of its read-back
— and it lifts **for that row and no other**.

## The family-level entries an image rides on, by number

An image's assumption surface is the union of these, not one row:

- **A1** — **TS-990S only**: the channel-name field is padded with ASCII space
  to its fixed ten-byte window on write, and trailing spaces are not part of
  the name on read. The book prints "Channel Name (Up to 10 digits.)" against a
  FIXED ten-cell window (`990:2955-2956`, erratum **E13**) and states no pad
  byte. **This fake neither pads nor trims** — it carries the ten bytes
  verbatim, `doc.go`'s register entry THE NAME WINDOW IS TEN BYTES, CARRIED
  VERBATIM — so the assumption an image rides on is what a *reader* makes of
  `"CW        "`, which is the codec's and the driver's business.
- **A2** — a memory name may contain the printable-ASCII characters this design
  writes, inside 0x20–0x7E and excluding `;`. Neither book prints a charset for
  the memory name; both give only the global ASCII coding rule with 80h–FFh
  remapped by Menu 9-01 (`990:32-39`).
- **A5** — the `MA` family spells the channel number back as three zero-padded
  ASCII digits, never space-padded. The ANSWER direction only; the SET
  direction is documented by the chart's own three P1 cells over "000 ~ 119"
  (`990:2894`).
- **A6** — "blank", where the `MA0` notes use it, means ASCII space 0x20.
  Neither `MA0` block defines it; this book defines it for `QR`, and more
  explicitly than the 890S's does: "this setting is blank <0x20>"
  (`990:4081-4082`).
- **A16** — **NOT ASSUMED, DOCUMENTED, and listed here for safety**: reading a
  single memory channel zeroes the second frequency's parameters
  (`990:2964-2965`). Every single-memory record this fake ships carries the
  all-zero frequency-2 side on that authority, so a reader can find the
  sentence rather than re-derive it. It is also what `classFor` reads to decide
  the P2 a record answers with.
- **A22** — the record shape, above.

**A4, A17 and A21 are deliberately NOT on this list.** All three are **TS-890S
only** and they exist because that book's blank-channel note stops at P12
(`890:3215-3216`, erratum **E4**), leaving its name window and even its frame
LENGTH unprinted. This book's note covers "P2 to P18" (`990:2962-2963`) inside
a frame whose terminator is nailed to position 57, so the blank answer here is
printed modulo A6 — which is why this fake ships ONE blank fixture and
`internal/fakets890` ships two.

**A9 and A10 are deliberately NOT on this list either.** No image is shipped
for slots 100–119 — the classes those numbers name are published in no bank of
this registry row and the book explains neither (erratum **E18**) — so nothing
here rides on what a Programmable VFO slot or an E channel holds. An `MA0` read
of one is refused through the same out-of-domain path as any other number. See
`doc.go`'s register entry SLOTS 100-119 ARE NOT SERVED, which is what a later
reader has to change on the day they lift.

**A14 is not on it, and the reason is worth stating.** That entry is about what
a HOST may emit in P2 on a Set. This fake emits no Set; it *discards* the P2 it
is sent, because the chart says the radio does — "this parameter is ignored.
Enter a dummy value" (`990:2901-2903`) — and re-derives the class a record
ANSWERS with from the frequency-2 side the same sentence names. What is assumed
in that derivation is one thing only, and it is `doc.go`'s own register entry
THE SET'S P2 IS IGNORED AND THE ANSWER'S CLASS FOLLOWS FREQUENCY 2: that the
class a Set can produce besides Single is **Dual** rather than **Section
defined**. The book never gives the rule separating those two; what it does say
is that a section is registered with `MA1` or `MI` (`990:3060-3061`).

## The one blank fixture, and why there is one

The blank channel is this fake's most valuable fixture, and on this row it is
its *least* assumption-laden rather than its most: `MA0`, the slot, **fifty
ASCII spaces** and the terminator — the whole of P2 to P18 (`990:2962-2963`) at
A6's reading of "blank", inside a frame the chart's own ruler fixes at 57
(`990:2915`). It is reached by **every unpopulated channel**, through the one
path `handleMA0Read` takes.

`internal/fakets890` needs a second fixture — a blank record carrying a
residual name — because its note stops at P12 and A21 is the separate
assumption that such a residue is not channel content. **There is no
counterpart here and none may be invented**: this book settles the name window.

## The Section defined channel this image does not compose

"2: Section defined Memory channel" is a printed P2 value (`990:2900`) and the
default image carries no record with it. What such a channel's P9 ("frequency
2") holds is **not** printed: the section's end frequency is `MA6`'s
(`990:3051-3059`), and the design's **A8** records the assumption that P9 does
not carry it. Composing one would put an unevidenced combination into an image
whose whole posture is that every byte is printed. `WithChannel` is the seam
for staging one when a driver's refusal path needs it.

## What is deliberately small, and why

The default image is four channels. That is a consequence of the posture rather
than a gap: an image that populated a hundred channels would turn every "this
channel is blank" assertion into a fixture accident. What it does carry is
SHAPE — a plain simplex channel with a blank name window, a second holding the
other printed value of every live two-value byte on the first frequency's side
and a full ten-character name, a third with a LIVE frequency 2 (the only class
of record in which half this grid is not zeroes, and the only way P2's second
value appears at all), and a fourth carrying the last of the four tone types.

**Each of the four two-digit index windows carries a distinct non-zero printed
index somewhere.** P7, P8, P13 and P14 sit at positions 22–23, 24–25, 40–41 and
42–43, and the `TN` and `CN` charts print identical frequencies at indices
00–49, so an image leaving them all `00` reads the same whether a driver takes
the right window, a neighbouring one, or none at all.

## The EX menu inventory — one chart, one copy, one cross-check

The second evidence leg of a fake in this project is its **independently
transcribed menu inventory** — transcription B, copied into the fake that serves
it, so that the fake's inventory and the codec's come from two different
readings of one chart and a transport cross-check can assert they agree.

- `transcription-b-990s.csv` is a **byte copy** of the quarantined artefact
  committed at `core/kw/ma/testdata/transcription-b-990s.csv`, SHA-256
  `b234f34ed45939bcde0ac7c90aad18c2ed857eb1b0b7f32d322aaff73ad3775a`. It is a
  COPY, not a move: `core/kw/ma/crosscheck_test.go` keeps reading the original
  as an artefact it binds and hashes by name.
  `core/transport/ex_crosscheck_ts990_test.go` asserts the copy is still
  byte-identical to it, because "the codec from A versus the fake from B" holds
  only for as long as this side's copy really is B.
- `exinventory.go` embeds the copy and projects it at init. It imports nothing
  project-internal — in particular not `internal/extable`, which generates the
  CODEC's side — so a shared parsing bug cannot reproduce itself identically
  into both inventories and be invisible. `imports_test.go` enforces that
  recursively, this directory and every one beneath it. There is **no
  generator** and no generated file (the plan's P19).
- `core/transport/ex_crosscheck_ts990_test.go` compares the two sides address
  for address and width for width, and drives every address over the wire.

**If the cross-check ever fires: report the diff. Do NOT edit either table to
make it pass.** Which side is wrong — or whether the printed chart is — is an
arbitration against the PDF, and an edit that merely restores agreement destroys
the evidence the agreement was worth.

### This chart has no excluded address

`internal/fakets890`'s projection removes four Advanced Menu rows its chart
prints with real addresses and the body "Does not correspond to a command"
(`890:2273-2280`, erratum **E16**). **This book prints no such row.** Every one
of transcription B's 194 addressed rows carries a width, the production
inventory excludes none, and this projection therefore removes nothing:
`TestEXDefaults_CarriesEveryAddressTranscriptionBPrints` holds it in both
directions, as a set difference against B's own addresses rather than as a
count.

### Ruling R-B, the PF key width — the one place this projection corrects the leg

Transcription B reads **three** digits on all **eighteen** PF key rows of this
chart (`0/00/15` … `0/00/32`) and transcription A reads **four**. The `EX`
block's own P5 note settles it: *"PF key settings use 4 digits (refer to the PF
Key assignment ID lists)."* (`990:1747-1748`), and the list it sends the reader
to — the "PF Key Assignment Lists" (`990:2287` onwards) — prints every function
allotment ID as a four-digit number, so the book settles the width **twice**.

**The run is eighteen rows and it is this chart's own.** The 990S inserts Voice
(Main Band) and Voice (Sub Band) at items 17 and 18 (`990:1793-1810`), so its
run ends at `0/00/32` where the 890S's seventeen end at `0/00/31`. Copying the
sibling's list would leave two of this chart's PF rows answering three bytes.

The leg is **frozen evidence**, so the error is corrected in `exinventory.go`'s
projection and **never in the CSV**. Two things keep that from being "editing a
table to make the cross-check pass":

- the authority is this book's own printed sentence, read at this side. The
  projection consults neither transcription A, nor the generated inventory, nor
  `internal/extable`, so the two sides still meet in `core/transport` as two
  derivations;
- the correction **refuses to apply** unless every one of the eighteen is
  present AND still carries the three the ruling records. A leg that has moved
  is an arbitration, not a correction to apply blind.

`TestEXInventoryCrossCheck_TS990PFKeyWidthsAgreeUnderRulingRB` asserts all three
facts together: both inventories carry four, the leg carries three, and the
correction touches those eighteen addresses and no others.

This correction is the **same class** as `internal/fakets890`'s four excluded
addresses: a fact this book prints that the leg does not carry, applied at this
side from an independently written list — not the same move as this file's own
text-flag omission below, which DECLINES to project a *convention* column
rather than SUBSTITUTING a datum the leg records. Same conclusion, different
class.

### What the menu VALUES are, and what they are not

Each menu's default raw P5 is its **printed width in `0` bytes** — an INVENTED
placeholder, `doc.go`'s register entry THE EX MENU VALUES ARE INVENTED. The
parameter lists print each menu's available settings and never a shipped default
anywhere. What is transcribed is the **width**, and only the width.

The **wire** answer is wider than that value, and that is this radio's frame
rather than its data: the Answer diagram holds P5 to fifteen cells and nails
the terminator to position 24 (`990:1738-1747`, erratum **E19**), so a narrower
value is padded into the window with the byte this book defines "blank" as
(`doc.go`'s register entry THE FIXED FORM'S PAD BYTE IS A SPACE). `core/kw/ma`
returns P5 verbatim and leaves the pad to its caller.

Two family-level entries the EX surface rides on, by number:

- **A19** — an `EX` answer's P5 never exceeds the width the parameter list
  prints for that menu number. The book prints widths per item class — 3
  normally, 4 for PF keys, 8 for a frequency setting, 0–15 for a power-on
  message, 0–10 for screen-saver text (`990:1746-1752`) — and no general
  ceiling. It is a CEILING, so a short answer is admitted on both sides and
  `WithEXSetting` can script an over-wide one.
- **A2** — the printable-ASCII charset, extended from P18's name window to EX's
  P5. `WithEXSetting` stores what it is given; the charset is enforced by the
  codec, on the side that has to read a real radio's bytes.

The **text flag** of transcription B is deliberately **not projected** into this
fake's table. The repository's ruling is that the flag is a CONVENTION and the
digits are the datum: A and B were briefed with different definitions of "text
row", and on this chart the two legs differ at **twenty-nine** addresses — the
28 Fixed Mode band-limit rows and Contest Number, all flagged text by B — where
the ruling is that only a row reading "Up to N alphanumeric characters" is a
text row (`990:1778-1779`), which leaves the Screen Saver Message and the
Power-on Message as the only two. A fake that projected B's flag would answer
spaces at twenty-nine addresses the repository has ruled numeric. The flag is
not a WIDTH, so neither reading changes an address's byte count. See
`exinventory.go`'s `widthFor`.
