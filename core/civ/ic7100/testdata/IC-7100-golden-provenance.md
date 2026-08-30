# IC-7100 golden vectors — provenance

Quarantine leg G. Built by hand from rendered page images of one PDF.

Index-style notation used throughout: a plain number in parentheses, e.g. `(5)`,
stands for an **outlined** circled numeral as drawn; a number followed by `f`,
e.g. `5f`, stands for the **filled/reversed** circled numeral (white digit on a
solid black disc) as drawn. The two styles are never normalised to one.

## Source

- Document title as printed on the cover (PDF p.1): **FULL MANUAL**, above the
  line `HF/VHF/UHF ALL MODE TRANSCEIVER` and the model name `IC-7100`, with
  `Icom Inc.` at the foot. The cover carries no revision code.
- Revision code as printed: **A7085-2EX-5**, printed at the foot of the left
  column of the last page (PDF p.387), immediately above `© 2013–2021 Icom Inc.
  May 2021`.
- File path:
  `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7100_civ_FM_5.pdf`
- Page count: **387** PDF pages (A4, 595.276 × 841.89 pt).

The CI-V material is Section 20, `CONTROL COMMAND`. Its folios read `20-n` and
the PDF page is `359 + n`; both are cited in every row of the CSV
(`pdf_page` is always the PDF page number, never the folio).

## Extent

Rendered at 300 dpi: PDF pages **359–380**. Re-rendered at 400 dpi: **361, 362,
363, 370, 371, 372, 373, 375, 376**. Re-rendered at 500 dpi: **370, 375**. Cover
and colophon rendered at 200 dpi: **1, 2, 387**.

Pages actually read as images, with folio and contribution:

| PDF page | Folio | Contributed |
|---|---|---|
| 1 | (cover, unfoliated) | Cover title, model, publisher. |
| 359 | 19-14 | Read only to fix the folio-to-PDF-page offset; it is the last page of Section 19 (`MAINTENANCE`), so Section 20 begins at PDF p.360. Contributed no byte. |
| 361 | 20-2 | `◇ Data format`: the `Controller to IC-7100` and `IC-7100 to controller` frame diagrams — preamble `FE FE`, transceiver's default address `88`, controller's default address `E0`, `Cn`, `Sc`, `Data area`, `FD`. Source of every `structural` run. |
| 362 | 20-3 | `◇ Command table` first page (commands `00`–`14`). Navigated only; contributed no byte. |
| 363 | 20-4 | Command table continued, and footnote `*2` with the worked example `Example: When operating with 4800 bps`. Source of `manual-example-1`. |
| 364 | 20-5 | Command table continued: the rows `19 / 00 / (Data column empty) / Read the transceiver ID` and `1A / 00 / see p. 20-16 / Send/read the Memory channel contents`. |
| 370 | 20-11 | `• Operating frequency` (5-byte digit map) and `• Operating mode` (mode and filter enums). |
| 371 | 20-12 | `• Character code setting`, whose command list begins `1A 00` — the ASCII table used for the memory name. |
| 372 | 20-13 | `• Duplex Offset frequency setting` (3-byte digit map). |
| 373 | 20-14 | `• Repeater tone/tone squelch frequency setting`, `• DTCS code and polarity setting`, `• Digital code squelch setting`, and the call-sign character table under `• DV TX call signs setting`. |
| 375 | 20-16 | `• Memory content setting / Command: 1A 00` — the two-row data-block bar, all field legends, and the hatched NOTE. The whole record layout. |
| 387 | (colophon, unfoliated) | Revision code. |

Pages rendered but **not** read as images: 360, 365–369, 374, 376–380. They were
passed over after navigation showed they carry no material this leg needs; none
contributed a byte.

Where the transcribed material begins and ends. On PDF p.375 the black bar
`Remote jack (CI-V) information` is followed by `◇ Data content description
(Continued)`, and immediately after that comes the heading `• Memory content
setting` with `Command: 1A 00` beneath it — that is where the record layout
begins. It ends at the foot of the same page with the hatched `NOTE:` block
whose last line is `that you set the same data as (5)–(51).`. The next page
(PDF p.376, folio 20-17) opens a different section, `• RIT frequency settings /
Command: 21 00`, so nothing of the memory record continues past PDF p.375.

## Method

1. **Locate, 300 dpi.** `pdftoppm -png -r 300 -f 359 -l 380 <pdf> r300/p` into the
   fresh directory `…/legs-out/ic7100/G/r300`, created for this leg. The renders
   were read as images to find the sections named in the task. Section headings on
   adjacent pages are near-identical (`Remote jack (CI-V) information`,
   `Remote jack (CI-V) information (Continued)`, `◇ Data content description`,
   `◇ Data content description (Continued)`); every value below was taken only
   from inside the section whose bold heading matches the field being read.
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f <n> -l <n> <pdf> r400/p` for pages
   361, 362, 363, 370, 371, 372, 373, 375, 376. Every recorded value was read at
   400 dpi or higher.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`
   and `convert`) and was used. Each diagram, numbered band, legend and table was
   cropped into its own image and enlarged before reading, e.g.

   ```
   magick r400/p-375.png -crop 2950x260+220+900   +repage -resize 200% crops/row1-full.png
   magick r400/p-375.png -crop 2000x260+220+1140  +repage -resize 300% crops/row2-full.png
   magick r400/p-375.png -crop 1100x430+200+2470  +repage -resize 250% crops/f04.png
   magick r400/p-375.png -crop 1300x380+1650+1470 +repage -resize 250% crops/f14.png
   magick r400/p-375.png -crop 1400x330+1700+3200 +repage -resize 250% crops/name-text.png
   magick r400/p-373.png -crop 1150x620+400+3290  +repage -resize 220% crops/rptone.png
   magick r400/p-373.png -crop 1200x680+1850+930  +repage -resize 220% crops/dtcs.png
   magick r400/p-372.png -crop 1200x920+440+3650  +repage -resize 200% crops/dupoffset.png
   magick r400/p-371.png -crop 1250x1400+200+1080 +repage -resize 190% crops/chartable.png
   magick r400/p-370.png -crop 1400x1000+250+930  +repage -resize 210% crops/freq2.png
   magick r400/p-361.png -crop 1450x1200+480+2550 +repage -resize 200% crops/dataformat.png
   magick r400/p-363.png -crop 1500x450+1680+3830 +repage -resize 300% crops/example-4800.png
   magick r400/p-363.png -crop 580x130+2330+3945  +repage -resize 700% crops/EO-vs-01.png
   ```

   The last of these was cut deliberately so that the example's `E _` cell and its
   `0 1` cell sit in one frame at 700 %, to compare the two round glyphs directly
   (see STOP 3).
4. **`pdftotext -layout` — run, navigational only.** It was run once, as
   `pdftotext -layout -f 359 -l 387 <pdf> nav.txt`, and used only to find which
   PDF page a heading is on and to confirm that the CI-V chapter ends at folio
   20-17. **It was the source of no recorded value**: no byte position, nibble
   label, numeral, field index, width, label or enum value in the CSV came from
   it. Its output for these diagrams is demonstrably unusable as evidence — see
   hazard (b) below.
5. **`tesseract`** was available but **was not used**. Every glyph was resolved by
   eye on an enlarged crop, so no OCR value needed confirming.
6. **Second independent pass — done.** After the first pass was complete, every
   value was re-read from a different raster. The second raster differed in three
   ways at once: a different dpi (**500 dpi**, `pdftoppm -r 500`, giving
   4134 × 5847 px for p.375 against 3308 × 4678 px at 400 dpi), **different crop
   windows** (the two bar rows were re-cut into five overlapping slices whose
   boundaries fall in different places from the first pass's slices, so that no
   cell sits at the same position in its frame), and a **different enlargement**
   (a uniform 350 % against the first pass's 200–300 %):

   ```
   magick r500/p-375.png -crop 1100x340+250+1120  +repage -resize 350% crops2/r1a.png
   magick r500/p-375.png -crop 1150x340+1300+1120 +repage -resize 350% crops2/r1b.png
   magick r500/p-375.png -crop 1200x340+2400+1120 +repage -resize 350% crops2/r1c.png
   magick r500/p-375.png -crop 1250x380+250+1400  +repage -resize 350% crops2/r2a.png
   magick r500/p-375.png -crop 1400x380+1400+1400 +repage -resize 350% crops2/r2b.png
   magick r500/p-375.png -crop 1500x350+2050+3960 +repage -resize 300% crops2/nametext.png
   magick r500/p-370.png -crop 1400x560+280+2770  +repage -resize 280% crops2/modetable.png
   ```

   **Cells where the two passes disagreed: none.** Both passes read the row-1
   groups as `(1)`, `(2),(3)`, `(4)`, `(5)~(9)`, `(10),(11)`, `(12)`, `(13)`,
   `(14)`, `(15)~(17)`, `(18)~(20)`, `(21)~(23)`, `(24)`, `(25)~(27)`; both read
   the row-2 groups as `(28)~(35)`, `(36)~(43)`, `(44)~(51)`, `5f~51f`,
   `(52)~(60)`; both read the body text as `(52)–(67) Memory name setting / 16
   characters (Fixed)`; both read `05: FM` and `01: FIL1`. No third render was
   needed.

Byte-level arithmetic was checked mechanically after the fact: every byte and
every nibble of all four vectors is covered by exactly one CSV row, and each
row's `bytes_hex` was verified equal to the corresponding slice of the
`.golden` line.

Two further declarations, so that the attestation below is read with the right
scope. **`pdfinfo` was run once on this same PDF**, for its page count and page
size only — both of which this leg's own brief also states — and it was the
source of no byte, byte position, nibble label, numeral, field index, width,
label or enum value; every such value came from a rendered page image.
**The only directories touched were this leg's own output directories** under
`…/legs-out/ic7100/G` (`r300`, `r400`, `r500`, `crops`, `crops2`), created by
this leg and containing nothing but the renders and files it made. No
repository, fixture, manual or other directory was listed or browsed, and no
file outside that output directory was opened other than the single PDF named
in the brief.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — ENCOUNTERED.** The single
  `1A 00` data-block bar on PDF p.375 draws its index numerals in two styles.
  Every index in row 1, and `(28)~(35)`, `(36)~(43)`, `(44)~(51)` and `(52)~(60)`
  in row 2, are **outlined** circled numerals. Between `(44)~(51)` and
  `(52)~(60)` sits a group whose two indices are **filled/reversed** — a solid
  black disc with the digits reversed out in white — reading `5f ~ 51f`. The same
  two styles recur in the hatched NOTE on the same page, whose first bullet reads
  `The same data as (5)–(51) are stored in 5f–51f.` Both styles are recorded as
  drawn, in the CSV's `field_index` column, using the `f` suffix defined at the
  head of this file. No meaning has been inferred from either style.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** Every
  digit-map diagram used here (operating frequency on 20-11, duplex offset on
  20-13, repeater tone and DTCS on 20-14, and the whole `Data format` figure on
  20-2) sets its leader labels rotated 90°. `pdftotext -layout` flattens the
  p.375 bar to two lines of bare `X` characters with the index groups collapsed
  onto a single preceding line, and it renders the row-2 groups as
  `@8〜#5   #6〜$3   $4〜%1   t〜%1   %2〜^0` — in which the transmit block's
  filled `5f` extracts as the token `t`, the **same** token the text layer uses
  for the outlined `(5)` in row 1, and the filled `51f` extracts as `%1`, the same
  token as the outlined `(51)`. The receive block and the transmit block are
  therefore indistinguishable in the text layer, exactly as the hazard warns.
  Every position recorded here was read from the picture.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In the `(4)`
  sub-diagram on p.375 the label list prints `0: Select memory OFF / 1: Select
  memory ON` first and `0: Split OFF, 1: Split ON` second, but following each
  leader by eye shows the first label lands on the **right** half of the byte and
  the second on the **left** half — the list order runs opposite to the positions.
  The `(13)` sub-diagram on the same page behaves identically (`0: OFF, 1: Tone /
  2: TSQL, 3: DTCS` lands on the right half; `0: Duplex OFF / 1: Duplex–, 2:
  Duplex+` on the left), as does `• Digital code squelch setting` on 20-14
  (`Second digit` on the right half, `First digit` on the left). Separately, in
  the `Data format` figure on 20-2 the leaders for `88` and `E0` physically
  **cross** between the upper and lower frames, so that `88` is the transceiver's
  address in the controller-to-radio frame and sits in the third position in the
  radio-to-controller frame. Each leader was followed by eye from label to cell.
- **(d) A printed index may differ from a field's measured position —
  ENCOUNTERED.** The `5f–51f` block repeats the `(5)–(51)` block. Both readings
  are recorded for every field and are not reconciled:

  | Printed index, as printed | Measured position in the 114-byte data block | Measured position in the 121-byte write frame |
  |---|---|---|
  | `5f`–`9f` (operating frequency) | 52–56 | 58–62 |
  | `10f`,`11f` (operating mode) | 57–58 | 63–64 |
  | `12f`,`13f`,`14f` | 59–61 | 65–67 |
  | `15f`–`17f` (repeater tone) | 62–64 | 68–70 |
  | `18f`–`20f` (tone squelch) | 65–67 | 71–73 |
  | `21f`–`23f` (DTCS) | 68–70 | 74–76 |
  | `24f` (digital code squelch) | 71 | 77 |
  | `25f`–`27f` (duplex offset) | 72–74 | 78–80 |
  | `28f`–`35f` (destination call sign) | 75–82 | 81–88 |
  | `36f`–`43f` (R1 call sign) | 83–90 | 89–96 |
  | `44f`–`51f` (R2 call sign) | 91–98 | 97–104 |

  The same divergence affects the name field, whose printed index begins at
  `(52)` while its measured position in the data block begins at 99 — the
  diagram numbers the name as though the 47-byte transmit block did not exist.

## STOP findings

1. **The memory name field's width is printed two different ways.**
   *PDF page:* 375 (folio 20-16).
   *Visual anchors:* (i) the second row of the `1A 00` data-block bar, its
   rightmost index group, printed `(52)〜(60)` above the last three drawn cells
   (`X:X`, a dotted ellipsis cell, `X:X`); (ii) the right-hand column of the same
   page, the legend entry printed `(52)–(67) Memory name setting` with `16
   characters (Fixed)` on the line beneath it and `See ‘• Character code
   setting.’ (p. 20-12)` beneath that.
   *What is printed:* the bar says the field runs from index 52 to index 60,
   which is **9** bytes. The body text says it runs from index 52 to index 67 and
   states `16 characters (Fixed)`, which is **16** bytes, and 67 − 52 + 1 = 16, so
   the body text agrees with itself.
   *Why it stops:* nine bytes and sixteen bytes cannot both be right, and the
   choice changes the total length of every complete-record write frame — 107
   data bytes against 114, i.e. a 114-byte or a 121-byte frame.
   *What was done:* built from the body text, which is the clearer of the two —
   it states an index range and a character count that corroborate each other,
   whereas the bar's `(60)` corroborates nothing on the page and the drawn cells
   under it are an ellipsis that conveys no count. The run (frame bytes 105–120 of
   `set-record-name-with-space`) carries `STOP 1` in its `notes`. Not resolved:
   both readings stand as printed.
2. **Indices 5 to 51 are printed twice in the same diagram, in different
   styling.**
   *PDF page:* 375 (folio 20-16).
   *Visual anchor:* the second row of the data-block bar, where the group printed
   `5f〜51f` in filled/reversed circled numerals sits between the outlined
   `(44)〜(51)` group and the outlined `(52)〜(60)` group; the outlined `(5)〜(9)`
   group appears in row 1 of the same bar.
   *What is printed:* the index run 5–51 appears once as outlined circled
   numerals and once as filled/reversed circled numerals, in one diagram.
   *Why it stops:* it is a repeat in the index sequence and an index printed twice
   with different styling — the STOP rules name both.
   *What was done:* recorded as drawn, with the two styles distinguished by the
   `f` suffix and never normalised; the eleven transmit-block runs of
   `set-record-name-with-space` (frame bytes 58–104) each carry `STOP 2`. No
   meaning has been inferred from the styling; the block's contents come from the
   printed NOTE, not from the styling.
3. **The worked example prints the controller's default address with a capital-O
   glyph.**
   *PDF page:* 363 (folio 20-4).
   *Visual anchor:* the box beneath `Example: When operating with 4800 bps`, the
   cell under index `(3)`, whose legend two lines below reads `(3) Controller’s
   default address`.
   *What is printed:* the cell contains `E` followed by a wide, round, circular
   glyph. Enlarged to 700 % beside the `0 1` cell under index `(5)` in the same
   box, the two are visibly different letterforms: the `(5)` cell's zero is a
   narrow oval, the `(3)` cell's glyph is a broad circle — a capital letter O, not
   a digit zero. On PDF p.361 (folio 20-2) the same field in the `Controller to
   IC-7100` frame is printed `E0` with an unambiguous zero.
   *Why it stops:* `EO` is not a byte, and the two pages print the same field
   differently.
   *What was done:* built from folio 20-2, which is the clearer of the two — it is
   the normative frame diagram and its glyph is unambiguous. Frame byte 11 of
   `manual-example-1` is `E0` and carries `STOP 3`. Not resolved.
4. **The worked example's index sequence has a gap.**
   *PDF page:* 363 (folio 20-4).
   *Visual anchor:* the same `Example: When operating with 4800 bps` box — the
   indices printed above its cells read, left to right, `(1) (2) (3) (4) (5) (7)`
   — and the legend beneath it, which lists `(1) Preamble code (fixed)`,
   `(2) Transceiver’s default address`, `(3) Controller’s default address`,
   `(4) Command number`, `(5) Sub command number`, `(7) End of message code
   (fixed)`.
   *What is printed:* index `(6)` appears in neither the box nor the legend, while
   the frame diagram on folio 20-2 assigns `(6)` to the `Data area`.
   *Why it stops:* it is a gap in the printed index sequence.
   *What was done:* transcribed exactly as printed — the example's frame carries
   no data area, so the vector has no byte between the sub command number and
   `FD`. Frame byte 14 of `manual-example-1` carries `STOP 4`. Not resolved and
   not interpolated.

## Observed disagreements

These are odd or self-contradictory things this leg noticed and did not act on.
They are recorded as printed and are not resolved.

- On PDF p.375 (folio 20-16), the block headed `About clearing operation:` lists
  `(2), (3): Memory channel 0 to 99`, `(4): FF` and `(5) or later: None`. Two
  things about it are odd. First, `Memory channel 0 to 99` contradicts the field
  legend higher on the same page, which prints `0001–0099: Memory channel 1 to
  99` — channel 0 against channel 1 as the first channel. Second, index `(1)`,
  the bank number, is not mentioned at all, so the block does not say whether a
  clearing frame carries a bank byte. Neither stopped this leg because it builds
  no clear or erase frame of any kind.
- On PDF p.370 (folio 20-11), the hatched note under `• Operating mode` states
  that the filter setting `(2)` `can be skipped with command 01 and 06`, and on
  PDF p.373 (folio 20-14) the byte `(1)` of `• Repeater tone/tone squelch
  frequency setting` is marked `*Not necessary when setting a frequency.` Both
  describe optional bytes — but both are stated for other commands (`01`, `06`,
  `1B 00`, `1B 01`), not for `1A 00`. Inside the `1A 00` record the same material
  is drawn as the fixed index ranges `(10),(11)` and `(15)~(17)`, three drawn
  cells wide, with no note attached. This leg therefore treats no field of the
  `1A 00` record as having a conditional printed width, and writes a single
  `set-record-name-with-space` vector rather than one per derived total.
- On PDF p.375 the `(14)` cell in the main bar is drawn `X` then a round `0`
  glyph, while every other cell in the bar is drawn `X` then `X`. The `(14)`
  sub-diagram on the same page draws the same byte as `X` then a literal `0`, so
  the two agree; it is noted only because the bar's low-nibble `0` is easy to
  mistake for an `X` at low magnification.
- Section 20 is 17 folios long (20-1 to 20-17, PDF pages 360–376) and contains
  exactly one worked example frame — the 4800 bps power-ON frame on folio 20-4.
  The other passage headed `Example:` in the chapter, on folio 20-12 under
  `• Band stacking register` (`When reading the oldest contents in the 21 MHz
  band, the code “0703” is used.`), gives a two-byte code and not a frame, so it
  yields no `manual-example-` vector.

## The vectors

All four are written to `IC-7100-vectors.golden`, one per line, as
`name<TAB>hex-bytes`. Every frame is a controller-to-transceiver frame built on
the `Controller to IC-7100` layout printed on PDF p.361 (folio 20-2):
`FE FE` preamble, `88` transceiver's default address, `E0` controller's default
address, `Cn` command number, `Sc` sub command number, data area, `FD` end of
message. No clear or erase frame and no transceive frame of any kind was built.

### `read-record` — 10 bytes

```
FE FE 88 E0 1A 00 01 00 01 FD
```

Reads one memory record: the `1A 00` command addressed to bank A, memory
channel 1.

| Frame byte | Hex | Carries | CSV row |
|---|---|---|---|
| 1–2 | `FE FE` | preamble code (fixed) | `structural`, p.361 |
| 3 | `88` | transceiver's default address | `structural`, p.361 |
| 4 | `E0` | controller's default address | `structural`, p.361 |
| 5 | `1A` | command number | `manual_documented`, p.364 |
| 6 | `00` | sub command number | `manual_documented`, p.364 |
| 7 | `01` | field `(1)`, bank A | `inherited_assumed` — see the register |
| 8–9 | `00 01` | fields `(2)`,`(3)`, memory channel 1 | `inherited_assumed` — see the register |
| 10 | `FD` | end of message code (fixed) | `structural`, p.361 |

### `set-record-name-with-space` — 121 bytes

```
FE FE 88 E0 1A 00
01 00 01 00
00 00 50 45 01 05 01 00 00 00 00 08 85 00 08 85 00 00 23 00 00 60 00
43 51 43 51 43 51 20 20  20 20 20 20 20 20 20 20  20 20 20 20 20 20 20 20
00 00 50 45 01 05 01 00 00 00 00 08 85 00 08 85 00 00 23 00 00 60 00
43 51 43 51 43 51 20 20  20 20 20 20 20 20 20 20  20 20 20 20 20 20 20 20
48 4F 4D 45 20 42 41 53 45 20 20 20 20 20 20 20
FD
```

(The line breaks above are for reading only; the `.golden` line is one
space-separated run of 121 pairs.)

Writes one complete memory record whose name field carries a space in the middle
of the name: `HOME BASE`, with the space at character 5 of 16.

**Length.** 6 framing and command bytes + 114 data bytes + 1 end-of-message byte
= **121**. The 114 data bytes are the printed index ranges summed: `(1)` = 1,
`(2),(3)` = 2, `(4)` = 1, `(5)~(9)` = 5, `(10),(11)` = 2, `(12)` = 1, `(13)` = 1,
`(14)` = 1, `(15)~(17)` = 3, `(18)~(20)` = 3, `(21)~(23)` = 3, `(24)` = 1,
`(25)~(27)` = 3, `(28)~(35)` = 8, `(36)~(43)` = 8, `(44)~(51)` = 8 — that is
51 bytes for indices 1 to 51, which the bar's own numbering confirms; plus the
repeated block `5f~51f` = 51 − 5 + 1 = 47; plus the name at 16 (STOP 1). 51 + 47
+ 16 = 114.

**Only one vector of this name is written.** No field of the `1A 00` record has a
conditional printed width — every one is drawn as a fixed index range and the
three call-sign fields and the name field are each labelled `fixed` — so the
record has exactly one derived total. The conditional-byte notes that do appear
in the chapter belong to other commands; see `## Observed disagreements`.

Byte-by-byte walk, keyed to `IC-7100-golden-assumptions.csv`:

| Frame byte | Hex | Printed index | What it carries and why this value |
|---|---|---|---|
| 1–4 | `FE FE 88 E0` | — | Framing and addressing, folio 20-2. |
| 5–6 | `1A 00` | — | Command and sub command, folio 20-5. |
| 7 | `01` | `(1)` | Bank A. `01: A` is printed. |
| 8–9 | `00 01` | `(2)`,`(3)` | Memory channel 1. `0001` is printed for `Memory channel 1`. |
| 10 | `00` | `(4)` | Split OFF (left half) and Select memory OFF (right half), both `0` in the printed enums. |
| 11–14 | `00 00 50 45` | `(5)`–`(8)` | 145.500000 MHz. Working: 10 Hz = 0 and 1 Hz = 0 → `00`; 1 kHz = 0 and 100 Hz = 0 → `00`; 100 kHz = 5 and 10 kHz = 0 → `50`; 10 MHz = 4 and 1 MHz = 5 → `45`. |
| 15 | `01` | `(9)` | Split across two CSV rows: the high half is the printed fixed `0` for the 1000 MHz digit; the low half is the 100 MHz digit, 1, inside the printed range 0–4. |
| 16–17 | `05 01` | `(10)`,`(11)` | FM (`05: FM`) with FIL1 (`01: FIL1`). |
| 18 | `00` | `(12)` | Data mode OFF (`00: Data mode OFF`). |
| 19 | `00` | `(13)` | Duplex OFF (left half) and tone OFF (right half). |
| 20 | `00` | `(14)` | Split across two CSV rows: the high half is the digital-squelch enum, `0` for OFF; the low half is the printed literal `0`. |
| 21 | `00` | `(15)` | Both halves printed `0 (fixed)`. |
| 22–23 | `08 85` | `(16)`,`(17)` | Repeater tone 88.5 Hz: 100 Hz = 0 and 10 Hz = 8 → `08`; 1 Hz = 8 and 0.1 Hz = 5 → `85`. |
| 24 | `00` | `(18)` | Both halves printed `0 (fixed)`. |
| 25–26 | `08 85` | `(19)`,`(20)` | Tone squelch 88.5 Hz, same digit map. |
| 27 | `00` | `(21)` | Transmit polarity Normal (`0`) and receive polarity Normal (`0`). |
| 28 | `00` | `(22)` | Split across two CSV rows: high half printed `0 (fixed)`; low half is the first DTCS digit, 0. |
| 29 | `23` | `(23)` | Second and third DTCS digits, 2 and 3 — DTCS code 023. |
| 30 | `00` | `(24)` | Digital code squelch code 00: first digit 0, second digit 0. |
| 31–32 | `00 60` | `(25)`,`(26)` | Duplex offset 0.600000 MHz: 1 kHz = 0 and 100 Hz = 0 → `00`; 100 kHz = 6 and 10 kHz = 0 → `60`. |
| 33 | `00` | `(27)` | Split across two CSV rows: high half printed `10 MHz digit: 0 (fixed)`; low half is the 1 MHz digit, 0. |
| 34–41 | `43 51 43 51 43 51 20 20` | `(28)`–`(35)` | Destination call sign `CQCQCQ` padded to the fixed 8 characters. C = 41 + 2 = 43; Q = 41 + 16 = 51; space = 20. |
| 42–49 | `20` ×8 | `(36)`–`(43)` | R1 call sign left blank: eight spaces filling the fixed 8-character field. |
| 50–57 | `20` ×8 | `(44)`–`(51)` | R2 call sign left blank, likewise. |
| 58–104 | copy of bytes 11–57 | `5f`–`51f` | The transmit block. The NOTE on the same page states `The same data as (5)–(51) are stored in 5f–51f` and `We recommend that you set the same data as (5)–(51)`, so the 47 bytes are copied verbatim. Split into eleven CSV rows matching the receive block's field groups; every one carries `STOP 2`. |
| 105–120 | `48 4F 4D 45 20 42 41 53 45 20 20 20 20 20 20 20` | `(52)`–`(67)` | Memory name `HOME BASE`, sixteen characters. H = 41 + 7 = 48; O = 41 + 14 = 4F; M = 41 + 12 = 4D; E = 41 + 4 = 45; space = 20; B = 41 + 1 = 42; A = 41; S = 41 + 18 = 53; E = 45; then seven trailing spaces to fill the field. The space sits at character 5, in the middle of the name, which is what this vector exists to exercise. Carries `STOP 1`. |
| 121 | `FD` | — | End of message code (fixed). |

### `read-transceiver-id` — 7 bytes

```
FE FE 88 E0 19 00 FD
```

The transceiver-identification read. The command table row on folio 20-5 reads
`19 / 00 / (Data column empty) / Read the transceiver ID`; the empty Data column
is why no data area follows the sub command number. Every byte of this vector is
either framing printed on folio 20-2 or a command byte printed on folio 20-5;
nothing in it is assumed.

### `manual-example-1` — 14 bytes

```
FE FE FE FE FE FE FE FE FE 88 E0 18 01 FD
```

The one worked example frame the document prints, transcribed as printed:
`Example: When operating with 4800 bps` on folio 20-4, the power-ON frame. Its
bold-ruled leading box holds `F E` and is marked `× 7` beneath, and the footnote
`*2` immediately above it reads `When sending the power ON command (18 01), the
command “FE” must be sent before the basic format.` followed by
`• 19200 bps: 25, • 9600 bps: 13, • 4800 bps: 7, • 1200 bps: 3, • 300 bps: 2` —
so at 4800 bps seven `FE` bytes precede the basic format. Bytes 8–14 are the
basic format as the box draws it: `(1)` `FE FE`, `(2)` `88`, `(3)` the
capital-O-glyph cell recorded as `E0` under STOP 3, `(4)` `18`, `(5)` `01`,
`(7)` `FD`. Index `(6)` is absent — STOP 4.

## Assumption register

One assumption was made. It is consolidated here because its two CSV runs are two
halves of a single decision.

**Runs:** `read-record` frame byte 7 (`01`) and frame bytes 8–9 (`00 01`) — the
whole data area of the read request.

**What was assumed.** That a `1A 00` read request carries the record's
identifying fields — the bank number `(1)` and the two-byte memory channel
number `(2),(3)` — and then ends, with no byte from `(4)` onward.

**Why these values and not others.** The document is silent on the format of a
`1A 00` read request. Folio 20-16 prints one layout only, the complete record,
and introduces it as `Command: 1A 00` without distinguishing a read from a write;
folio 20-5 gives the command table row `1A / 00 / see p. 20-16 / Send/read the
Memory channel contents`, which names both directions but points at that same
single layout. Nowhere does the document print a shortened read form, state which
fields a read request must carry, or give a worked example of one. So the
composition is mine. Within that composition the two byte values are not
arbitrary: `(1)` is `01` because folio 20-16 prints `01: A` for bank A, and
`(2),(3)` are `00 01` because folio 20-16 prints `0001–0099: Memory channel 1 to
99`, making `0001` memory channel 1. Bank A and channel 1 were chosen as the
lowest values the printed enums admit, so that the frame exercises the encoding
without depending on any range the page does not state. The prefix was cut after
`(3)` rather than after `(4)` because `(1)`, `(2)` and `(3)` are the only fields
on the page that identify *which* record is meant — `(4)` onward are all record
contents — and a read must identify a record. That reasoning is mine; the
document does not state it.

**The one capture that would settle it.** A single **Stage R** capture on a real
IC-7100 — call it **Stage R capture `R-7100-1A00-BANKA-CH1`** — in which the
frame `FE FE 88 E0 1A 00 01 00 01 FD` is put on the CI-V bus of an IC-7100 whose
transceiver address is the default `88`, and everything the radio puts back on
the bus is recorded. That capture, by itself, observes exactly one thing:
whether this IC-7100 answers this frame with a memory-record reply or with the
`FE FE E0 88 FA FD` NG message. An answer settles that bank number plus
two-byte channel number is an accepted read request for bank A, channel 1 on
this radio; an NG settles that it is not. It observes nothing about any other
bank, any other channel, any other command, or any other model, and this register
claims nothing beyond it.

Everything else in all four vectors is either `structural` (the framing and
addressing printed on folio 20-2), `manual_documented` (a value the page prints)
or `manual_derived` (a value computed by the working shown above from a printed
encoding). No other byte in this leg is assumed.

## Hardware status

UNVERIFIED. No IC-7100 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.

## Attestation

Every value recorded here was read from this single PDF's rendered page images. `pdftotext -layout` was run on this same PDF for navigation only and was the source of no recorded value. Nothing else was consulted: no other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.
