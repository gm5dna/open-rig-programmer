# IC-7760 — geometry witness (quarantine leg W)

## Source

- Document title, as printed on the cover: **CI-V REFERENCE GUIDE**, above
  **HF/50 MHz TRANSCEIVER** and the model mark **IC-7760**; publisher mark
  **Icom Inc.** at the foot of the cover.
- Revision code, as printed: **A7788-8EX-2**. It is printed at the
  bottom-left of the last page (PDF page 28), on the line above
  `© 2024–2025  Icom Inc.      May 2025`.
- File path:
  `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7760_civ_2.pdf`
- Page count: 28 PDF pages.

## Extent

Diagram identifiers, each defined by the printed caption above it, verbatim:

- **D1** — caption `• Memory content` with `Command: 1A 00` on the line
  beneath it, both sitting under the sub-heading `◇ Command formats`.
  The horizontal row of byte cells with the circled index brackets above it.
- **D2** — caption `③: Select memory setting`. The single two-part box
  printed `0` `X` with a circled 3 above it, in the left column.
- **D3** — caption `⑪: Data mode and tone type settings`. The single
  two-part box printed `X` `X` with a circled 11 above it, in the right
  column.

Pages rendered and read:

| PDF page | Printed folio | Rendered | Read | What it contributed |
|---|---|---|---|---|
| 1 | (no folio printed on the cover) | 150 dpi | yes | Cover title, model, publisher, for `## Source`. |
| 19 | 18 | 300 dpi | folio strip only | The printed folio only, establishing the folio offset. The page body was not read; that the material does not begin earlier was established from page 20 itself, where its section band and sub-heading are printed. |
| 20 | 19 | 300, 400 and 600 dpi | yes — the only page transcribed | D1, D2 and D3, and all recorded values. |
| 21 | 20 | 300 dpi | folio strip only | The printed folio only, establishing the folio offset. The page body was not read; that the material does not continue past page 20 was established from page 20 itself, where the next sub-heading begins. |
| 28 | (no folio printed) | 200 dpi | bottom strip only | Revision code, for `## Source`. |

The folio is the PDF page number minus one on every page checked
(PDF 19 → 18, PDF 20 → 19, PDF 21 → 20).

Where the transcribed material begins and ends, on PDF page 20:

- Immediately before it, in reading order: the running head `REMOTE
  CONTROL`, the reversed section band `Remote control (CI-V) information`,
  then the sub-heading `◇ Command formats`. D1's caption `• Memory
  content` / `Command: 1A 00` follows directly.
- The material runs from D1 through the two-column explanatory block that
  contains D2 (left column) and D3 (right column), and ends with the
  four-line block `To clear the memory channel contents on 1A 00:` … `④:
        None`.
- Immediately after it: the bold sub-heading `• Codes for character
  entries`, then `Command: 1A 00,` / `1A 05    01 76, 01 83, 01 98, 02 03,
  02 07, 03 71`, then `- Character codes— Letters and Numbers` and its
  table. Nothing in that later material is a memory-record data-block
  diagram, and nothing from it was transcribed.

Judgement call recorded here rather than resolved: the brief asks for
"every numbered field in every memory-record data-block diagram" on the
page. D1 is unambiguously such a diagram. D2 and D3 are single-byte
expansions of two of D1's own fields, drawn as their own boxes with their
own circled index above them and their own dotted nibble rule; they are
where this page prints nibble-level structure. They have therefore been
included as separate diagrams rather than dropped. Each contributes one
row, because each prints exactly one index numeral; the nibble-level
labelling inside them is recorded in the `notes` column and below, since
neither nibble carries a printed index of its own and inventing one is not
open to me.

## Method

Every value recorded in the CSV was read from a rendered page image.

1. **Locate, 300 dpi.** Into the fresh directory `…/legs-out/ic7760/W`,
   created for this leg:

       pdftoppm -png -r 300 -f 19 -l 21 <pdf> r300/p
       pdftoppm -png -r 150 -f 1  -l 1  <pdf> r300/cover
       pdftoppm -png -r 200 -f 28 -l 28 <pdf> r300/last

   The whole-page render of `r300/p-20.png` was read as an image to find
   the section headed `Remote control (CI-V) information` / `◇ Command
   formats` and, within it, D1, D2 and D3. The adjacent bold sub-heading
   on the same page, `• Codes for character entries`, was checked and
   excluded: it heads character-code tables, not a data-block diagram.

2. **Read, 400 dpi.**

       pdftoppm -png -r 400 -f 20 -l 20 <pdf> r400/p     # 3308 × 4678 px

   First-pass values were read from crops of this raster.

3. **Crop and enlarge.** ImageMagick was available (`/opt/homebrew/bin/magick`,
   and `convert`) and was used throughout. First-pass crops:

       magick r400/p-20.png -crop 2250x260+540+890  +repage -resize 200% crops/D1_full.png
       magick r400/p-20.png -crop 1080x190+630+930  +repage -resize 300% crops/D1_left.png
       magick r400/p-20.png -crop 1080x190+1660+930 +repage -resize 300% crops/D1_right.png
       magick r400/p-20.png -crop 900x480+280+1450  +repage -resize 250% crops/D2_sel.png
       magick r400/p-20.png -crop 1250x400+1750+1180 +repage -resize 250% crops/D3_datamode.png

   At 300 % every cell boundary, dotted nibble rule, dashed ellipsis-cell
   border, bracket leg and circled numeral stood clear of its neighbours.

4. **`pdftotext`.** Not run at all. It was not used to navigate, and no
   text layer of this or any other document was consulted. The first of
   the two attestation forms is therefore the true one.

5. **`tesseract`.** Available at `/opt/homebrew/bin/tesseract`, but not
   used. Every numeral, glyph and rule was read by eye off the enlarged
   crops; nothing needed an OCR aid, so nothing required OCR confirmation.

6. **Second independent pass.** After the first pass was complete, the
   page was re-rastered at a different resolution and re-cut with
   different crop windows, and every value re-read from those without
   reference to the first pass's figures:

       pdftoppm -png -r 600 -f 20 -l 20 <pdf> r600/p         # 4961 × 7016 px
       magick r600/p-20.png -crop 1080x310+940+1385  +repage -resize 200% crops2/A.png
       magick r600/p-20.png -crop 1080x310+2000+1385 +repage -resize 200% crops2/B.png
       magick r600/p-20.png -crop 1120x310+3020+1385 +repage -resize 200% crops2/C.png
       magick r600/p-20.png -crop 1400x760+400+2150  +repage -resize 160% crops2/D2b.png
       magick r600/p-20.png -crop 1900x620+2600+1760 +repage -resize 160% crops2/D3b.png

   How the second raster differed: 600 dpi rather than 400 dpi (1.5× the
   pixel pitch), and D1 cut into three overlapping thirds with boundaries
   deliberately placed inside different cells from the two-half cut of the
   first pass, so that no cell boundary or bracket leg fell at the same
   position relative to a crop edge in both passes. D2 and D3 were re-cut
   with different windows and a different enlargement factor.

   **Cells where the two passes disagreed: none.** Both passes counted 18
   drawn cells in D1; both put the same bracket legs on the same cell
   boundaries; both read the same circled numerals, the same shading, the
   same two dashed ellipsis cells, the same dotted nibble rule inside every
   X:X cell, and the same leader-line destinations in D2 and D3. No third
   render was needed.

   Cell boundaries were also cross-checked numerically between the two
   passes by converting each crop's pixel positions back to page
   coordinates: cell pitch measured ≈ 110 px at 400 dpi and ≈ 165 px at
   600 dpi, i.e. the same physical pitch, and the eighteen boundaries
   landed on the same page coordinates in both passes to within a few
   pixels.

## Position arithmetic, per diagram

Two independent readings are given for D1 and neither is adjusted to the
other, per the brief: the **printed numbering** (the circled numerals the
diagram itself prints above its cells, which are byte positions of the
record) and the **drawn-cell running count** (counting the boxes actually
drawn, left to right, from the diagram's own first cell). The CSV's
`first_byte` / `last_byte` carry the printed numbering, because this
diagram does number its byte positions.

Every drawn X:X cell contains exactly two X glyphs separated by a dotted
vertical rule, i.e. one byte of two nibbles. No field in any of the three
diagrams begins or ends part-way through a byte, so every field runs
nibble 1 to nibble 2 of its bytes.

### D1 — `• Memory content`, `Command: 1A 00`

Cells as drawn, left to right, with shading and content as seen:

| Drawn cell | Content | Fill | Border |
|---|---|---|---|
| 1 | X:X | shaded | solid |
| 2 | X:X | shaded | solid |
| 3 | X:X | white | solid |
| 4 | X:X | shaded | solid |
| 5 | … (ellipsis, no nibble rule) | shaded | **dashed** |
| 6 | X:X | shaded | solid |
| 7 | X:X | white | solid |
| 8 | X:X | white | solid |
| 9 | X:X | shaded | solid |
| 10 | X:X | white | solid |
| 11 | X:X | white | solid |
| 12 | X:X | white | solid |
| 13 | X:X | shaded | solid |
| 14 | X:X | shaded | solid |
| 15 | X:X | shaded | solid |
| 16 | X:X | shaded | solid |
| 17 | … (ellipsis, no nibble rule) | shaded | **dashed** |
| 18 | X:X | shaded | solid |

Running position, field by field:

1. `①, ②` — printed: starts byte 1, extent 2 bytes, ends byte 2; next
   printed index starts at 3. Drawn: starts cell 1, extent 2 cells, ends
   cell 2; next group starts cell 3. **Agree.**
2. `③` — printed: starts 3, extent 1 byte, ends 3; next printed starts 4.
   Drawn: starts cell 3, extent 1 cell, ends cell 3; next group starts
   cell 4. **Agree.**
3. `④ ~ ⑧` — printed: starts 4, extent 5 bytes, ends 8; next printed
   starts 9. Drawn: starts cell 4, extent **3** cells (cell 4, the dashed
   ellipsis cell 5, cell 6), ends cell 6; next group starts cell 7.
   **Disagree — 3 drawn cells against a printed span of 5 bytes. STOP 1.**
   From here on the drawn running count is 2 behind the printed numbering.
4. `⑨, ⑩` — printed: starts 9, extent 2 bytes, ends 10; next printed
   starts 11. Drawn: starts cell 7, extent 2 cells, ends cell 8; next
   group starts cell 9. **Disagree in position (7–8 against 9–10);
   extents agree. STOP 2.**
5. `⑪` — printed: starts 11, extent 1 byte, ends 11; next printed starts
   12. Drawn: starts cell 9, extent 1 cell, ends cell 9; next group starts
   cell 10. **Disagree in position (9 against 11); extents agree. STOP 3.**
6. `⑫ ~ ⑭` — printed: starts 12, extent 3 bytes, ends 14; next printed
   starts 15. Drawn: starts cell 10, extent 3 cells, ends cell 12; next
   group starts cell 13. **Disagree in position (10–12 against 12–14);
   extents agree. STOP 4.**
7. `⑮ ~ ⑰` — printed: starts 15, extent 3 bytes, ends 17; next printed
   starts 18. Drawn: starts cell 13, extent 3 cells, ends cell 15; next
   group starts cell 16. **Disagree in position (13–15 against 15–17);
   extents agree. STOP 5.**
8. `⑱ ~ ㉗` — printed: starts 18, extent 10 bytes, ends 27; nothing
   follows. Drawn: starts cell 16, extent **3** cells (cell 16, the dashed
   ellipsis cell 17, cell 18), ends cell 18; nothing follows. **Disagree
   in both position and extent. STOP 6.**

Totals. Printed: the highest index printed anywhere in the row is ㉗, so
the printed numbering totals 27 bytes, and 2 + 1 + 5 + 2 + 1 + 3 + 3 + 10
= 27, which is internally consistent. Drawn: 2 + 1 + 3 + 2 + 1 + 3 + 3 + 3
= 18 cells, and 18 cells were counted end to end in both passes, which is
also internally consistent. **27 against 18 — the two totals disagree.
STOP 7.** Both are recorded; neither is resolved here.

No index is repeated, no index is skipped, and the sequence runs strictly
1, 2, 3, 4–8, 9, 10, 11, 12–14, 15–17, 18–27 in left-to-right page order,
so there is no discontinuity in the index sequence itself.

### D2 — `③: Select memory setting`

One box, one drawn cell, divided by a dotted vertical rule into two halves.

- Printed: the box carries a single circled `3` above it, and nothing else
  is numbered in this diagram. Starts byte 3, extent 1 byte, ends byte 3.
- Drawn: counted from this diagram's own first cell, the box is cell 1,
  extent 1 cell, ends cell 1. **Disagree — cell 1 against printed 3.
  STOP 8.** Both recorded; neither resolved.
- Nibbles, as the diagram itself labels them: the diagram does **not**
  print nibble numbers. It labels the halves by upward arrows. The arrow
  under the **left** half (which is printed as the literal digit `0`, not
  `X`) drops to the word `Fixed`. The arrow under the **right** half
  (printed `X`) runs down, then right, to a square-bracketed list of four
  lines: `0=OFF`, `1= ★1`, `2= ★2`, `3= ★3`. The two leaders do not
  cross. In the recording convention, `Fixed` is nibble 1 and the ★ list
  is nibble 2. The ★ glyph is present and legible on the render in all
  three of `1= ★1`, `2= ★2`, `3= ★3`.

### D3 — `⑪: Data mode and tone type settings`

One box, one drawn cell, divided by a dotted vertical rule into two halves,
both printed `X`.

- Printed: the box carries a single circled `11` above it. Starts byte 11,
  extent 1 byte, ends byte 11.
- Drawn: counted from this diagram's own first cell, the box is cell 1,
  extent 1 cell, ends cell 1. **Disagree — cell 1 against printed 11.
  STOP 9.** Both recorded; neither resolved.
- Nibbles, as the diagram itself labels them: again no nibble numbers are
  printed; two upward arrows carry the labelling. Followed by eye on both
  rasters, the arrow under the **left** half descends the further of the
  two and turns right along the **lower** horizontal line to `0: OFF, 1:
  DATA 1, 2: DATA 2, 3: DATA 3`. The arrow under the **right** half
  descends only as far as the **upper** horizontal line and turns right to
  `0: OFF, 1: TONE, 2: TSQL`. The two labels therefore read down the page
  in the opposite order to the cells they point at: the first label
  printed belongs to the second nibble. In the recording convention, the
  DATA list is nibble 1 and the TONE/TSQL list is nibble 2.

## Hazards encountered

- **(a) Numeral styling varying within one diagram — NOT ENCOUNTERED.**
  Every index numeral on the page, in D1's bracket band and in the D2 and
  D3 captions and boxes alike, is drawn in one single style: an outlined
  thin circle enclosing plain, unbolded digits. Checked at 300 % on the
  400 dpi raster and again at 200 % on the 600 dpi raster across all
  fourteen numerals actually drawn (①, ②, ③, ④, ⑧, ⑨, ⑩, ⑪, ⑫, ⑭, ⑮, ⑰, ⑱,
  ㉗). None is filled, reversed, bracketed or bold; the two-digit ones are
  the same outlined circle with a slightly wider glyph pair inside.
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED, as far as the
  render shows; the text-layer half of the hazard is CANNOT DETERMINE.**
  On the render, every label in D1, D2 and D3 is set horizontally; nothing
  is rotated. Whether the text layer extracts them in an order unrelated
  to the page could not be determined, because no text layer was consulted
  at any point in this leg — `pdftotext` was never run. All positions here
  were read from the picture regardless, which is what the hazard asks for.
- **(c) Leader-line label order reversed — ENCOUNTERED, in D3.** In
  `⑪: Data mode and tone type settings`, the label printed first (upper
  line, `0: OFF, 1: TONE, 2: TSQL`) is led from the **right** half of the
  box, and the label printed second (lower line, `0: OFF, 1: DATA 1, 2:
  DATA 2, 3: DATA 3`) is led from the **left** half; the left half's leader
  runs down past the upper label's line before turning right. Reading the
  two labels in printed order would attach each to the wrong nibble. Each
  leader was followed by eye from the arrowhead to its label on both
  rasters. In D2 the same construction occurs but the leaders do **not**
  cross: `Fixed` belongs to the left half and the ★ list to the right.
- **(d) A printed index differing from a field's measured position —
  ENCOUNTERED, throughout D1 and in both sub-diagrams.** No block of
  fields repeats another block anywhere on this page, so the repeat case
  did not arise; but the printed index and the measured drawn-cell
  position diverge from `④ ~ ⑧` onward in D1 because two groups are drawn
  with a dashed ellipsis cell standing in for omitted bytes, and they
  diverge in D2 and D3 because each is a one-cell box carrying its parent
  record's index. Both readings are recorded for every affected field, in
  `## Position arithmetic, per diagram` above and in the `notes` column of
  the CSV. Neither has been reconciled to the other and no printed index
  has been adjusted to fit a measured position.

## STOP findings

1. **PDF page 20, D1, the group under the bracket labelled `④ ~ ⑧`.**
   Printed: a bracket whose left leg meets the left edge of the fourth
   drawn cell and whose V-shaped right leg meets the right edge of the
   sixth, labelled `④ ~ ⑧` — a span of five byte indices. Drawn under it:
   three cells only, the middle one (cell 5) a dashed-border cell
   containing an ellipsis `…` and no nibble rule. Why it stops: the
   measured extent, 3 cells, does not equal the printed span, 5 bytes.
   CSV row `D1` / `④ ~ ⑧` carries the printed span, 4 to 8, with `STOP 1`
   in `notes`.

2. **PDF page 20, D1, the group under the bracket labelled `⑨, ⑩`.**
   Printed: indices 9 and 10. Measured: the two cells under that bracket
   are the seventh and eighth drawn cells from the diagram's own first
   cell. Why it stops: the running drawn-cell position, 7–8, disagrees
   with the printed numbering, 9–10. Recorded as printed, 9 to 10, with
   `STOP 2` in `notes`.

3. **PDF page 20, D1, the single cell headed by a bare circled `11`.**
   Printed: index 11. Measured: the ninth drawn cell. Why it stops: 9
   against 11. Recorded as printed, 11 to 11, with `STOP 3` in `notes`.

4. **PDF page 20, D1, the group under the bracket labelled `⑫ ~ ⑭`.**
   Printed: indices 12 to 14. Measured: drawn cells 10 to 12. Why it
   stops: 10–12 against 12–14, though the extents (3 and 3) agree.
   Recorded as printed, 12 to 14, with `STOP 4` in `notes`.

5. **PDF page 20, D1, the group under the bracket labelled `⑮ ~ ⑰`.**
   Printed: indices 15 to 17. Measured: drawn cells 13 to 15. Why it
   stops: 13–15 against 15–17, though the extents agree. Recorded as
   printed, 15 to 17, with `STOP 5` in `notes`.

6. **PDF page 20, D1, the rightmost group, under the bracket labelled
   `⑱ ~ ㉗`.** Printed: a span of ten byte indices, 18 to 27. Drawn under
   it: three cells — a shaded X:X cell, a dashed-border ellipsis cell
   (cell 17), and the final shaded X:X cell at the right end of the row —
   occupying drawn positions 16 to 18. Why it stops: the measured extent,
   3 cells, does not equal the printed span, 10 bytes, and the measured
   start, 16, does not equal the printed start, 18. Recorded as printed,
   18 to 27, with `STOP 6` in `notes`.

7. **PDF page 20, D1, the cell row taken as a whole, from the leftmost
   shaded cell to the rightmost shaded cell.** Printed: the highest index
   drawn anywhere above the row is ㉗, and the printed spans sum to 27
   bytes. Measured: 18 cells were counted end to end, twice, on two
   different rasters. Why it stops: a total that does not match its parts —
   27 printed byte positions are carried by 18 drawn cells. Both figures
   are recorded; the `⑱ ~ ㉗` row carries `STOP 7` alongside `STOP 6` in
   `notes`.

8. **PDF page 20, D2, the two-part box under `③: Select memory setting`.**
   Printed above the box: a circled `3`. Measured: counting from this
   diagram's own first cell, the box is cell 1. Why it stops: the running
   count within the diagram, 1, disagrees with the printed numbering, 3.
   The CSV carries the printed numeral, 3 to 3, with both readings in
   `notes` and `STOP 8`.

9. **PDF page 20, D3, the two-part box under `⑪: Data mode and tone type
   settings`.** Printed above the box: a circled `11`. Measured: counting
   from this diagram's own first cell, the box is cell 1. Why it stops:
   1 against 11. The CSV carries the printed numeral, 11 to 11, with both
   readings in `notes` and `STOP 9`.

10. **PDF page 20, the explanatory text below D1, left column, first line
    after the diagram; and the block headed `To clear the memory channel
    contents on 1A 00:` in the right column.** The left column prints
    `①, ②: Memory group number`, immediately followed by `00 01 ~ 00 99:
    Memory channel 01 ~ 99`, `01 00:   Programmed scan edge P1`, `01 01:
    Programmed scan edge P2`. The right column prints, for the same index
    pair, `①, ②:  Memory channel (00 01~00 99)`. Why it stops: something
    printed contradicts something else printed — the same two bytes are
    named `Memory group number` in one place and `Memory channel` in the
    other. Nothing is repaired; the D1 `①, ②` row carries `STOP 10` in
    `notes` and its measured and printed positions are unaffected.

## Observed disagreements

- The two dashed-border cells in D1 (drawn cells 5 and 17) are drawn to
  the same width as the X:X cells around them and are shaded like their
  neighbours, but they contain a centred ellipsis `…` and no dotted nibble
  rule. Nothing printed on the page says how many bytes each stands for;
  the count can only be inferred from the bracket labels above them, which
  is the inference the STOP findings above decline to make.
- The shading of D1's cells does not group with the brackets throughout.
  It alternates group by group for the first six groups — shaded (`①, ②`),
  white (`③`), shaded (`④ ~ ⑧`), white (`⑨, ⑩`), shaded (`⑪`), white
  (`⑫ ~ ⑭`) — and then does not: `⑮ ~ ⑰` and `⑱ ~ ㉗` are both shaded, so
  cells 13 through 18 are one unbroken run of shading spanning two
  bracketed groups. Shading was therefore not used as evidence of any
  boundary; every boundary recorded here was taken from a bracket leg.
- In D2, the left half of the box is printed as the literal digit `0`
  where every other cell on the page prints `X`, and is labelled `Fixed`.
  Recorded as printed.
- In the right column, `④:      None` appears in the clear-the-channel
  block, where in D1 `④` is the first index of the five-index group
  `④ ~ ⑧`. Recorded as printed; no reading of it is offered.
- D2's list uses `=` (`0=OFF`, `1= ★1`) whilst D3's lists use `:`
  (`0: OFF, 1: TONE`). Recorded as printed.
- The spacing inside D2's list is uneven as printed: `0=OFF` closes up,
  whilst `1= ★1`, `2= ★2` and `3= ★3` each carry a space between the `=`
  and the ★. Recorded as printed.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No
other file, manual, transcription, source file, generated artefact or web
resource was opened, and no directory was listed.
