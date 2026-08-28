# IC-905 CI-V — memory-record data-block geometry witness

## Source

- Document title as printed on the cover (PDF page 1): the black cover panel prints
  `CI-V REFERENCE GUIDE`; below it, `ALL MODE TRANSCEIVER` above the model mark
  `IC-905`; at the foot, `Icom Inc.`
- Revision code as printed, and where: `A7711-9EX-2`, printed at the bottom-left of the
  back cover (PDF page 31), on the line immediately above
  `© 2023–2024  Icom Inc.      May 2024`. No revision code is printed on the front cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic905_civ_2.pdf`
- Page count: 31 PDF pages. Established from the renderer: page 31 renders as the back
  cover carrying the imprint, and a request for page 32 is refused with
  `Wrong page range given: the first page (32) can not be after the last page (31)`.

## Extent

Rendered at 300 dpi: PDF pages 17, 18, 19, 20, 21 (plus pages 1 and 31 at 150 dpi for the
cover and imprint). Rendered again at 400 dpi: PDF page 19 and page 31. Rendered again at
600 dpi: PDF page 19.

Read:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | (none printed) | cover title, model, publisher |
| 17 | 16 | scanned only to confirm the preceding section; contributed no recorded value |
| 18 | 17 | the material printed immediately BEFORE the transcribed diagram |
| 19 | 18 | the whole of the transcribed material — the only memory-record data-block diagram |
| 20 | 19 | the material printed immediately AFTER the transcribed diagram |
| 21 | 20 | scanned only to confirm the following section; contributed no recorded value |
| 31 | (none printed) | revision code, copyright line, page-count check |

The transcribed material begins on PDF page 19 (printed folio 18) under the grey section
bar `Remote control (CI-V) information`, the sub-heading `◇ Command formats`, and the
bold bullet caption `• Memory content` with `Command: 1A 00` on the line beneath it. The
data-block diagram is the two-row picture immediately below that line; it ends at the
right-hand end of its lower row, and the printed matter immediately after it is the
two-column legend beginning `①, ②: Memory group number` (left column) and
`⑮: Digital squelch setting` (right column), which runs to the foot of page 19 ending with
the note `ⓘ RPS can be set when DD mode is selected, and Duplex (+, -) can be set when
other than DD mode is selected.` and the folio `18`.

Immediately before the transcribed material, on PDF page 18 (folio 17), the page ends with
`• Codes for CW message contents` and its ASCII-code table — no memory record. Immediately
after, PDF page 20 (folio 19) opens with `• Codes for character entries` and, in the right
column, `• Band stacking register` whose diagram is a two-cell (①, ②) block, not a memory
record.

There is exactly ONE memory-record data-block diagram in this extent, and it is on PDF
page 19. It is defined here as:

- **D1** — printed caption verbatim: `• Memory content` (with `Command: 1A 00` printed on
  the line immediately below the caption and above the diagram).

D1 is a single data block drawn across two rows of the page: the upper row ends in a curved
arrow that turns down and left into an arrowhead at the start of the lower row, so the two
rows are one continuous record. Both rows were measured as one sequence.

Three further small boxes on page 19 (in the legend, under the indices ⑤, ⑭ and ⑮) redraw a
SINGLE byte of D1 each, with leader lines to enumerations. They are detail expansions of one
cell of D1, not memory-record data-block diagrams, so they receive no CSV rows of their own;
what they show about the two halves of their byte is recorded in the `notes` of the
corresponding D1 row.

### How D1 numbers its byte positions

D1 prints a numbered band directly above each row of cells. Every index in that band is a
numeral drawn inside a plain thin circle. Single-cell fields carry a bare circled numeral
centred over the cell; multi-cell fields carry a square bracket whose two arms descend onto
the outer edges of the field, with the circled first and last index of the range printed on
the bracket, separated either by a comma (`①, ②`) or by a swung dash (`⑥~⑩`).

So the diagram DOES number its byte positions, and the numerals were read off the render.
Byte 1 is the first cell of the upper row; the numbering runs 1…24 across the upper row and
25…68 across the lower row, continuously.

### How D1 shows nibbles

Every drawn cell of D1 is divided by a dotted vertical line into two equal halves, each
half carrying one printed glyph (`X`, or `0` where a half is fixed). The diagram does NOT
print a number or a label for either half — the dotted mid-line is the only nibble marking
in the data block itself. Nibble 1 (as this CSV records it) is therefore the left half of a
cell and nibble 2 the right half, the diagram running left to right.

Two cells print a `0` in one half: cell 5 prints `0 : X` and cell 13 of the upper row (byte
15) prints `X : 0`. In both cases the field's bracket/numeral covers the WHOLE cell, so the
field is recorded as spanning nibbles 1 to 2 of that byte, with the fixed half recorded in
`notes`.

### Recording convention for `field_index`

Every index printed in D1's band, and every index printed in the page-19 legend, is drawn
in one and the same style: a black numeral inside a plain thin circle outline, on white
(see `## Hazards encountered`, (a)). Unicode has no circled forms above 50, and D1's band
prints circled 52, 53 and 68. Rather than write some indices circled and others plain —
which would falsely suggest two printed styles — the CSV writes every index as its plain
numeral(s) with the printed separator preserved (`1, 2`; `6~10`; `53~68`). The circle is
uniform across all eighteen indices and is recorded here, once, rather than per row.

## Method

1. **Locate, 300 dpi.** Fresh directory
   `…/evidence/ic905-W/r300`, created for this leg and empty beforehand:
   `pdftoppm -png -r 300 -f 17 -l 21 <pdf> …/r300/p`
   The five renders were read as images to find the section whose printed heading is
   `Remote control (CI-V) information` / `◇ Command formats` / `• Memory content`. Pages 1
   and 31 were rendered at 150 dpi into the same directory (`cover`, `last`) for the cover
   and the imprint.
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f 19 -l 19 <pdf> …/r400/p` (3308 × 4678 px),
   and the same for page 31. Every first-pass value was read from these.
3. **Crop and enlarge.** ImageMagick IS available (`/opt/homebrew/bin/magick`,
   `/opt/homebrew/bin/convert`) and was used. First-pass crops from the 400 dpi page:
   - `magick r400/p-19.png -crop 2750x260+200+1020 +repage -resize 200% crops/row1_full.png`
   - `magick r400/p-19.png -crop 2750x260+200+1280 +repage -resize 200% crops/row2_full.png`
   - thirds of row 1: `magick crops/row1_full.png -crop 2000x400+0+60 +repage -resize 150% crops/row1_a.png`
     (and `+1800+60`, `+3500+60` for `row1_b.png`, `row1_c.png`)
   - pieces of row 2: `magick r400/p-19.png -crop 620x190+330+1300 +repage -resize 400% crops/row2_a.png`
     (and offsets `+900`, `+1420`, `+1330`, `+1620` for `row2_b/c/d/e.png`)
   - detail boxes: `-crop 900x420+180+2480 … -resize 220%` (⑤),
     `-crop 800x520+1690+1550 … -resize 220%` (⑮),
     `-crop 1100x580+180+3700 … -resize 200%` (⑭)
   - legend and caption: `-crop 1450x300+1700+2020 … -resize 200%`,
     `-crop 1500x900+1700+2280 … -resize 170%`, `-crop 1400x340+180+600 … -resize 200%`
   - imprint: `magick r400/last-31.png -crop 900x200+130+4380 +repage -resize 300% crops/revcode.png`
   At these enlargements every cell border, dotted mid-line, dashed ellipsis box, bracket
   arm and circled numeral stands clear of its neighbours.
4. **`pdftotext -layout` was NOT run** — not for navigation, not for anything. Navigation
   was done entirely by reading the 300 dpi page images. `pdfinfo` WAS run once on this same
   PDF at the start (it reported the title, producer, encryption flags and a page count);
   it was the source of no recorded value, and the page count recorded in `## Source` was
   independently established from the renderer as described there. Two shell listings were
   run, both confined to this leg's own workspace and to the target PDF's own path:
   `ls -la ic905_civ_2.pdf`, to confirm the named target file exists at the absolute path
   given, and a listing of the render directories this leg itself created beneath
   `…/evidence/ic905-W`. No directory holding any other document, manual, repository file or
   prior output was browsed or listed, and no file other than the target PDF and this leg's
   own renders and outputs was opened.
5. **`tesseract` was available (`/opt/homebrew/bin/tesseract`) but was NOT used.** Every
   numeral, glyph and rule recorded here was read by eye from the enlarged crops. No OCR
   value enters this witness.
6. **Second independent pass — done.** After the first pass was complete, page 19 was
   re-rendered at **600 dpi** (`pdftoppm -png -r 600 -f 19 -l 19`, 4961 × 7016 px) into
   `…/r600/`, and the diagram was re-cut into a DIFFERENT set of windows at a DIFFERENT
   enlargement — four quarters of the upper row
   (`-crop 1000x290+390+1550`, `+1320`, `+2250`, `+3180`, all `-resize 250%`) and three
   thirds of the lower row (`-crop 950x280+545+1950`, `+1440`, `+2335`, `-resize 250%`) —
   so that no crop boundary of the second pass fell where a crop boundary of the first pass
   had fallen. Every index numeral, every cell, every shading, every ellipsis box and every
   bracket-arm landing point was re-read from those rasters.
   **Result: the two passes agreed in every cell. There were no disagreements**, so no
   third render was needed to settle one. In particular both passes independently counted
   22 drawn units in the upper row and 16 drawn units in the lower row, both read the band
   as `1, 2 / 3, 4 / 5 / 6~10 / 11 / 12 / 13 / 14 / 15 / 16~18 / 19~21 / 22~24` and
   `25 / 26~28 / 29~36 / 37~44 / 45~52 / 53~68`, and both placed the single `0 : X` cell at
   drawn unit 5 and the single `X : 0` cell at drawn unit 13 of the upper row.

## Position arithmetic, per diagram

### D1 — `• Memory content` (Command: 1A 00), PDF page 19

D1 elides repeated cells: some fields are drawn as `cell — dashed ellipsis box — cell`
rather than as one cell per byte. The table below therefore gives, for each field in
printed order, three things measured separately: how many DRAWN UNITS the field's bracket
or numeral covers on the render (counted cell by cell), the printed byte range read off the
band, and the number of bytes the field's ellipsis box (if any) must stand for. The running
position column is the running total of the PRINTED byte ranges, which is what the CSV
records.

Upper row — 22 drawn units, left to right (D = drawn X:X cell, E = dashed ellipsis box):

| # | printed index | drawn units | fill | starts at byte | printed extent | ends at byte | next starts at | bytes hidden in E |
|---|---|---|---|---|---|---|---|---|
| 1 | `1, 2` | 2 (D D) | white | 1 | 2 | 2 | 3 | — |
| 2 | `3, 4` | 2 (D D) | grey | 3 | 2 | 4 | 5 | — |
| 3 | `5` | 1 (D, printed `0 : X`) | white | 5 | 1 | 5 | 6 | — |
| 4 | `6~10` | 3 (D E D) | grey | 6 | 5 | 10 | 11 | 3 |
| 5 | `11` | 1 (D) | white | 11 | 1 | 11 | 12 | — |
| 6 | `12` | 1 (D) | grey | 12 | 1 | 12 | 13 | — |
| 7 | `13` | 1 (D) | white | 13 | 1 | 13 | 14 | — |
| 8 | `14` | 1 (D) | grey | 14 | 1 | 14 | 15 | — |
| 9 | `15` | 1 (D, printed `X : 0`) | white | 15 | 1 | 15 | 16 | — |
| 10 | `16~18` | 3 (D D D) | grey | 16 | 3 | 18 | 19 | — |
| 11 | `19~21` | 3 (D D D) | white | 19 | 3 | 21 | 22 | — |
| 12 | `22~24` | 3 (D D D) | grey | 22 | 3 | 24 | 25 (lower row) | — |

Upper-row check: drawn units 2+2+1+3+1+1+1+1+1+3+3+3 = **22**, which is the number of units
counted on the render (and re-counted on the 600 dpi raster). Printed bytes
2+2+1+5+1+1+1+1+1+3+3+3 = **24**, and the band's last upper-row index is 24. The single
difference, 24 − 22 = 2 units, is exactly the elision: field `6~10` draws 2 real cells plus
1 ellipsis box for 5 bytes, so its ellipsis box stands for 3 bytes and replaces them with
1 box, a net saving of 2 units. The two counts reconcile with nothing left over.

Lower row — 16 drawn units, left to right, continuing the same record after the wrap arrow:

| # | printed index | drawn units | fill | starts at byte | printed extent | ends at byte | next starts at | bytes hidden in E |
|---|---|---|---|---|---|---|---|---|
| 13 | `25` | 1 (D) | white | 25 | 1 | 25 | 26 | — |
| 14 | `26~28` | 3 (D D D) | grey | 26 | 3 | 28 | 29 | — |
| 15 | `29~36` | 3 (D E D) | white | 29 | 8 | 36 | 37 | 6 |
| 16 | `37~44` | 3 (D E D) | grey | 37 | 8 | 44 | 45 | 6 |
| 17 | `45~52` | 3 (D E D) | white | 45 | 8 | 52 | 53 | 6 |
| 18 | `53~68` | 3 (D E D) | white | 53 | 16 | 68 | — (end of record) | 14 |

Lower-row check: drawn units 1+3+3+3+3+3 = **16**, the number counted on the render (and
re-counted at 600 dpi). Printed bytes 1+3+8+8+8+16 = **44**; 25 + 44 − 1 = 68, the band's
last index. Real drawn cells in the lower row = 16 − 4 ellipsis boxes = 12; bytes hidden in
the four ellipsis boxes = 6 + 6 + 6 + 14 = 32; 12 + 32 = **44**. Reconciles exactly.

Whole record: 24 + 44 = **68 bytes**, first byte 1, last byte 68; every byte from 1 to 68
is claimed by exactly one field, with no gap and no overlap between successive fields
(each field's `ends at byte` is one less than the next field's `starts at byte`, throughout,
across the row wrap as well as within a row).

Nibbles: every field's bracket or numeral covers whole cells, so every field begins at
nibble 1 of its first byte and ends at nibble 2 of its last byte. Two bytes have one half
printed as a fixed `0` (byte 5, left half; byte 15, right half), but the printed index in
both cases spans the whole cell, so the recorded extent is the whole byte.

Independent cross-checks against printed widths elsewhere on page 19 — all agree, none
stops:

- `13` — legend prints `1 byte data (XX)`; measured 1 cell. Agrees.
- `29~36` — legend prints `(8 characters, fixed)`; printed range 29…36 = 8 bytes. Agrees.
- `37~44` — legend prints `(8 characters, fixed.)`; printed range 37…44 = 8 bytes. Agrees.
- `45~52` — legend prints `(8 characters, fixed)`; printed range 45…52 = 8 bytes. Agrees.
- `53~68` — legend prints `(16 characters, fixed)`; printed range 53…68 = 16 bytes. Agrees.

Repeated blocks (hazard (d)): fields 15, 16 and 17 are three occurrences of the same 8-byte
call-sign block, drawn identically (`D E D`). Each was located and counted separately on the
render, from its own bracket arms, without reference to the others: field 15 occupies drawn
units 5–7 of the lower row and is printed `29~36`; field 16 occupies drawn units 8–10 and is
printed `37~44`; field 17 occupies drawn units 11–13 and is printed `45~52`. In all three,
the printed index and the measured running position agree; neither was adjusted to fit the
other.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram.** All eighteen indices in D1's numbered
  band are drawn in one single style — a black numeral inside a plain thin circle outline on
  white ground — with no circled index anywhere in D1 filled, reversed, bracketed, bold or
  plain-uncircled; the legend prose below the diagram repeats the same circled style
  (`①, ②: Memory group number`, `㊺~52: R2 (Gateway/Link repeater) call sign setting`), and
  the same style is used for the standalone `⑤`, `⑭` and `⑮` above the three detail boxes.
  The CSV writes plain numerals for the reason given under "Recording convention for
  `field_index`", not because two styles were seen. **NOT ENCOUNTERED**
- **(b) Diagrams may be vector groups with rotated labels.** No label in D1 is rotated;
  every index and every cell glyph is upright. Position was in any case read only from the
  picture, cell by cell, and the text layer was never extracted (`pdftotext` was not run at
  all). **NOT ENCOUNTERED**
- **(c) Leader-line label order may be reversed.** D1's own band uses bracket arms landing
  on cell edges, not leader lines. Leader lines with arrowheads do occur in the three detail
  boxes (⑤, ⑭, ⑮), and each was followed by eye from the label to the half it lands on:
  for ⑤ the word `Fixed` lands on the LEFT half (the `0`) and the enum list `0=OFF* …` on the
  RIGHT half (the `X`); for ⑮ the word `Fixed` lands on the RIGHT half (the `0`) and the enum
  list `0=Digital squelch function OFF …` on the LEFT half (the `X`); for ⑭ the duplex list
  lands on the LEFT half and the tone list on the RIGHT half, its leader running left past
  the box's midpoint before turning up. In every case the leader lands on the half its label
  describes; nothing was reversed. **NOT ENCOUNTERED**
- **(d) A printed index may differ from a field's measured position.** Repeating blocks are
  present — the three 8-byte call-sign blocks `29~36`, `37~44`, `45~52`, drawn identically —
  and all three were measured separately from their own bracket arms rather than assumed to
  match. For every one of the eighteen fields the printed index and the position measured by
  counting from the diagram's own first cell agree; no index was reinterpreted and no
  measurement adjusted. **ENCOUNTERED** (repeating blocks present and each measured
  separately; no printed index departed from its measured position)

## STOP findings

1. **PDF page 19, right-hand legend column, the two consecutive lines printed immediately
   below the detail box for ⑮ and immediately above `㉒~㉔: DTCS code setting`.** What is
   printed, verbatim, on those two lines and the note under them:

   ```
   ⑯~⑱: Repeater tone frequency setting
   ⑲~㉑: Repeater tone frequency setting
   ⓘ See “Repeater tone/tone squelch frequency setting.” (p. 23)
   ```

   Two different, adjacent, non-overlapping three-byte ranges of the same record are given
   word-for-word the same description, whilst the cross-reference beneath them names a
   section covering two distinct settings (`Repeater tone` and `tone squelch`). What is
   printed therefore contradicts itself: the two ranges cannot both be the repeater tone
   frequency setting if the referenced section distinguishes two settings for them. This is
   recorded, not resolved: no substitute wording is inferred for either line, and no
   geometry is changed by it — the bracket arms for `16~18` and for `19~21` are unambiguous
   on the render and both fields are transcribed exactly as measured (bytes 16–18 and
   19–21). `STOP 1` is carried in the `notes` of both affected CSV rows.

No other STOP arose. Reasons for confidence on the rest: the index sequence printed in the
band runs 1…68 with no repeat, no gap, no out-of-order index and no index printed twice;
the drawn-unit counts and the printed byte ranges reconcile exactly in both rows once each
dashed ellipsis box is allowed to stand for the bytes it replaces, leaving no gap and no
overlap anywhere in the record; every printed width stated in the legend agrees with the
printed range it labels; every value was legible without ambiguity at 400 dpi enlarged and
again at 600 dpi enlarged; and the two independent passes agreed in every cell.

## Observed disagreements

Recorded as printed, not resolved:

1. **The grey/white banding of D1 stops alternating at one boundary.** Through the whole of
   the upper row and most of the lower row, consecutive fields alternate grey fill and white
   fill (upper row: white, grey, white, grey, white, grey, white, grey, white, grey, white,
   grey; lower row: white, grey, white, grey, …). The last two fields break it: `45~52` is
   drawn white and `53~68`, immediately following it, is also drawn white, so no change of
   fill marks the boundary between them. The boundary is nevertheless unambiguous on the
   render — the right arm of the `45~52` bracket and the left arm of the `53~68` bracket both
   descend onto the same cell edge, and a solid cell border is drawn there. Confirmed in both
   passes (400 dpi crops and 600 dpi crops).
2. **Inconsistent punctuation of the identical parenthetical in the legend.** `㉙~㊱` and
   `㊺~52` are annotated `(8 characters, fixed)` whilst `㊲~㊹`, between them, is annotated
   `(8 characters, fixed.)` — with a full stop inside the closing bracket. Transcribed as
   printed in the CSV `notes`.
3. **The lower row of D1 has no caption of its own.** The caption `• Memory content` and the
   line `Command: 1A 00` stand above the upper row only; the lower row is joined to it solely
   by the curved wrap arrow. Nothing is printed to say the record continues, other than the
   arrow and the continuation of the index sequence from 24 to 25.
4. **D1 divides the record into no named or headed blocks.** Nothing is printed above, below
   or beside any run of cells that names a block; the only headings on the page are the
   section bar, `◇ Command formats` and the bullet caption. Every CSV row therefore carries
   `-` in `block_label_verbatim`.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual,
transcription, source file, generated artefact or web resource was opened, and no directory
was listed.
