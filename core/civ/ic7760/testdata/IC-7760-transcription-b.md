# IC-7760 — memory-record transcription, leg B

Companion to `IC-7760-transcription-b.csv` (13 rows, one header line).

## Source

- **Document title, as printed on the cover (PDF page 1, no folio):** a black band
  reading `CI-V REFERENCE GUIDE`; beneath it `HF/50 MHz TRANSCEIVER` over the model
  name `IC−7760` (the hyphen is drawn as a long dash in the display face); the foot of
  the cover reads `Icom Inc.`
- **Revision code, as printed:** `A7788-8EX-2`, at the bottom left of the back cover
  (PDF page 28, no folio), on the line directly above `© 2024–2025  Icom Inc.      May 2025`.
- **File:** `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7760_civ_2.pdf`
- **Page count:** 28 PDF pages. Taken from `pdfinfo` run on this same PDF, and
  confirmed against the renders: `pdftoppm -f 28 -l 29` produced a page 28 image and
  no page 29 image.
- **Folio relation:** the printed folio is the PDF page number minus one throughout the
  numbered body (PDF 18 → folio 17, PDF 20 → folio 19, PDF 24 → folio 23). The cover
  (PDF 1) and the back cover (PDF 28) carry no folio.

## Extent

### Pages rendered

| dpi | PDF pages rendered |
|---|---|
| 300 | 1, 5–11, 18–25, 28 |
| 400 | 18, 20, 24 |
| 500 | 18, 24 (second pass) |
| 600 | 20 (second pass) |

### Pages read, and what each contributed

| PDF page | folio | read as image | contribution |
|---|---|---|---|
| 1 | none | yes | Cover; document title. |
| 6 | 5 | yes | `◇ Command table`. Navigation only: the row `1A* / 00 / See p. 19. / Send/read memory contents` fixes the memory-content format at folio 19 = PDF page 20. |
| 7 | 6 | yes | `◇ Command table`, SET > Function rows. Nothing used. |
| 8 | 7 | yes | `◇ Command table`, SET > Function / Front Key Customize / MIC Key Customize / Connectors rows. Nothing used. |
| 9 | 8 | yes | `◇ Command table`, SET > Connectors rows. Nothing used. |
| 10 | 9 | yes | `◇ Command table`, SET > Connectors / Network rows. Nothing used. |
| 18 | 17 | yes | `• Operating frequency` and `• Operating mode` — the two sections cross-referenced from fields ④ ~ ⑧ and ⑨, ⑩. Source of those two rows' `values_verbatim`. |
| 19 | 18 | yes | `• Band stacking register` — the section printed immediately **before** the transcribed material. Not transcribed. |
| 20 | 19 | yes | **The transcribed page.** The `• Memory content` data block, the ③ and ⑪ sub-diagrams, the whole left/right legend, and the `• Codes for character entries` tables. |
| 21 | 20 | yes | `• Keyer memory character entries` etc. — printed immediately **after** the transcribed material. Not transcribed. |
| 23 | 22 | yes | Checked only to establish that no section titled `• Repeater tone/tone squelch settings` is printed there (see STOP 1). |
| 24 | 23 | yes | `• Repeater tone/tone squelch frequency settings` — the section cross-referenced from ⑫ ~ ⑭ and ⑮ ~ ⑰. Source of those rows' `values_verbatim`. |
| 25 | 24 | yes | Checked only for the same reason as page 23 (STOP 1). |
| 28 | none | yes | Back cover; revision code. |

PDF pages 5, 11 and 22 were rendered as boundary context and were **not** read.

### Where the transcribed material begins and ends

On PDF page 20 (folio 19), under the running head `REMOTE CONTROL`, the band
`Remote control (CI-V) information` and the heading `◇ Command formats`, the material
begins at `• Memory content` / `Command: 1A 00` and runs to the end of the
`To clear the memory channel contents on 1A 00:` block in the right column. Immediately
before it, on the previous page (PDF 19, folio 18), is `• Band stacking register`;
immediately after it, lower on the same page, is `• Codes for character entries`, and on
the following page (PDF 21, folio 20) `• Keyer memory character entries`.

### The character table

**Printed: yes**, on the transcribed page itself (PDF page 20, folio 19), under
`• Codes for character entries` / `Command: 1A 00,` / `1A 05   01 76, 01 83, 01 98, 02 03, 02 07, 03 71`.
Two tables are printed: `- Character codes— Letters and Numbers` (three populated rows,
two cells struck through with a diagonal rule) and `- Character codes— Symbols`
(16 rows × 2 Character/ASCII-code pairs). It contributed the whole of the
`values_verbatim` cell for field ⑱ ~ ㉗. The right-hand table on the same page
(`Cmd. / Sub cmd. / Set item/selectable characters`) prints `Memory name*` against
`1A` / `00`, and its asterisk footnote — the usable-character list — is recorded verbatim
in that row's `notes` (it is kept out of `values_verbatim` because it contains a vertical
bar, which is the separator used there).

### The set-mode / menu pages

**Printed: yes.** PDF pages 6–10 (folios 5–9) are the `◇ Command table`, an index of
commands including all the `SET > …` menu items. **Contribution: navigation only.** None
of the transcribed fields refers to a SET-mode or menu item; the fields' cross-references
point instead at `• Operating frequency` (p. 17), `• Operating mode` (p. 17),
`• Repeater tone/tone squelch settings` (p. 23) and `• Codes for character entries` (same
page). The one thing these pages gave is the row on folio 5 that sends the reader to
folio 19 for `Send/read memory contents`. No field label, width, encoding or value in the
CSV came from them.

## Method

Every value in the CSV was read from a rendered page image. Nothing was read from a text
layer.

1. **Locate — 300 dpi.** Into the fresh directory
   `…/legs-out/ic7760/B/renders300` (created empty by this leg):

   ```
   pdftoppm -png -r 300 -f 1  -l 1  <pdf> renders300/p
   pdftoppm -png -r 300 -f 5  -l 11 <pdf> renders300/p
   pdftoppm -png -r 300 -f 18 -l 22 <pdf> renders300/p
   pdftoppm -png -r 300 -f 23 -l 25 <pdf> renders300/p
   pdftoppm -png -r 300 -f 28 -l 29 <pdf> renders300/z
   ```

   The renders were read as images to find the sections named in the brief and to confirm
   the section boundaries either side of the transcribed material. The adjacent
   `◇ Command formats` sections on folios 18 and 20 do resemble the target one; the
   transcribed block is the one whose bold sub-heading reads exactly `• Memory content`.

2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f 20 -l 20`, and the same for pages 18 and
   24, into `…/renders400`. Every first-pass value was read from these.

3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, and
   `convert`) and was used. Representative commands:

   ```
   magick renders400/p-20.png -crop 2300x320+500+860  +repage -resize 200% crops/d1_band.png
   magick renders400/p-20.png -crop 800x260+560+890   +repage -resize 350% crops/d1_seg1.png
   magick renders400/p-20.png -crop 800x260+1320+890  +repage -resize 350% crops/d1_seg2.png
   magick renders400/p-20.png -crop 800x260+2050+890  +repage -resize 350% crops/d1_seg3.png
   magick renders400/p-20.png -crop 1500x620+240+1140 +repage -resize 220% crops/L_top.png
   magick renders400/p-20.png -crop 560x320+1770+1260 +repage -resize 400% crops/R_leaders.png
   magick renders400/p-18.png -crop 1350x900+230+1080 +repage -rotate 90 -resize 190% crops/F_freq_rot2.png
   magick renders400/p-24.png -crop 1050x720+220+1080 +repage -rotate 90 -resize 220% crops/T_tone_rot.png
   ```

   The rotated crops were needed because the digit labels on folios 17 and 23 are set
   vertically; `-rotate 90` (not `-rotate -90`, which delivers them upside down) puts them
   the right way up.

   Cell shading in the main block was additionally measured rather than eyeballed, by
   sampling the background colour just inside the top rule of each of the 18 drawn cells:
   `magick renders400/p-20.png -format "%[pixel:p{X,1030}]" info:` at
   X = 700, 810, 918, 1028, 1140, 1253, 1362, 1471, 1580, 1690, 1799, 1908, 2024, 2133,
   2243, 2354, 2463, 2573. Result, left to right:
   grey, grey, white, grey, grey, grey, white, white, grey, white, white, white, grey,
   grey, grey, grey, grey, grey (`srgb(220,221,221)` vs `srgb(255,255,255)`).

4. **`pdftotext -layout` was NOT run.** It was not used for navigation and it was not
   used for anything else. Navigation was done by reading 300 dpi renders.

5. **`tesseract`** was available (`/opt/homebrew/bin/tesseract`) but **was not used**.
   Every value was read by eye off the renders.

6. **Second independent pass — done.** With the first pass complete, every value was
   re-read from a different raster, before any comparison with the first pass's numbers:

   - Page 20 re-rendered at **600 dpi** (`renders600/p-20.png`, 4961×7016) instead of 400.
   - Pages 18 and 24 re-rendered at **500 dpi** (`renders600/q-18.png`, `q-24.png`).
   - **Different crop windows and different enlargements** throughout. The main data
     block was cut into three windows split at different points from pass 1
     (`-crop 1400x420+900+1290`, `+2200+1290`, `-crop 1000x420+3300+1290`, at 160 % and
     220 %, against pass 1's three 800-px windows at 350 %). The legend was cut into four
     windows at 145 % (`+330+1680`, `+330+2740`, `+2500+1690`, `+2500+2560`) against pass
     1's four at 220 %/300 %. The symbols table was split after its 8th row in pass 1 and
     at a different row in pass 2, at 170 % rather than 250 %. The rotated diagrams were
     re-cut at 500 dpi with different offsets and 165 %/175 % enlargement.

   **Disagreements between the two passes: none.** Every field index, every brace span,
   every cell count, every shading state, every label, every code and every meaning read
   the same on both rasters, including the two readings most at risk — the `0 ~ 6` on the
   `10 MHz digit` label (which an upside-down rotation renders as `0 ~ 9`) and the
   crossing leader lines under the ⑪ sub-diagram. No third render was needed.

### Two conventions used in the CSV, stated here so they are not mistaken for readings

- **`pdf_page` is the page where the field's semantics are printed**, per the brief's
  definition of that column — so it reads 18 for ④ ~ ⑧ and ⑨, ⑩, and 24 for ⑫ ~ ⑭ and
  ⑮ ~ ⑰, because page 20 prints for those fields only a label and a cross-reference. Each
  such row's `visual_anchor` names **both** locations: where the field sits in the block on
  page 20, and where its values are printed.
- **Half-byte rows carry an empty `field_index`.** The ③ and ⑪ sub-diagrams (D2, D3) index
  the byte as a whole with a circled numeral above the box; nothing is printed against the
  individual half-byte cells, so `field_index` is empty there and the circled index is
  recorded in `notes`. The three diagrams are:
  - **D1** — printed caption `• Memory content` (bold), with `Command: 1A 00` beneath it;
    the wide 18-cell block spanning the top of PDF page 20 under those two lines.
  - **D2** — no caption of its own; the two-cell box in the **left** column of PDF page 20,
    below the legend line `③: Select memory setting`, headed by a circled 3.
  - **D3** — no caption of its own; the two-cell box at the top of the **right** column of
    PDF page 20, below the legend line `⑪: Data mode and tone type settings`, headed by a
    circled 11.

### Arithmetic check

Widths from the printed index groups: 2 + 1 + 5 + 2 + 1 + 3 + 3 + 10 = **27**, which is
the highest printed index, ㉗. The block draws 18 cells, of which two (the 5th and the
17th) are dashed ellipsis cells standing for 3 and 8 elided bytes respectively; 16 drawn
byte cells + 3 + 8 = 27. Measured brace spans on the render agree with the printed groups
at every boundary. No gap, no overlap, no shortfall.

### Other commands run

`pdfinfo` on this same PDF (page count, page size). `ls -la` on the PDF's own path and
`ls` on this leg's own render directory, to confirm the renders were produced. `file`,
`xxd` and a `csv.reader` round-trip on the CSV this leg wrote, to confirm it is UTF-8
without a BOM and parses as 14 records of 9 fields. Nothing else in the repository was
listed, opened or searched.

Stated plainly so the Attestation below is read correctly: the only paths those two `ls`
calls touched were the target PDF itself and this leg's own output directory — both inside
the permitted set. No other directory, and nothing in the repository, was listed.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every index in
  the main block (①, ②, ③, ④, ⑧, ⑨, ⑩, ⑪, ⑫, ⑭, ⑮, ⑰, ⑱, ㉗) and both sub-diagram
  headers (③, ⑪) are drawn in one style: an outlined circle with a black numeral inside,
  no fill, no reversal, no brackets, no bold. Checked at 400 dpi and again at 600 dpi.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** The two
  referenced diagrams on folios 17 and 23 set all their digit labels rotated 90°, and the
  main block's braces and indices sit above the cells as separate vector elements. Every
  position here was read from the picture: each rotated crop was re-rotated with
  ImageMagick and each label followed to the half-cell its arrow lands on. No text layer
  was consulted at any point.
- **(c) Leader-line label order may be reversed — ENCOUNTERED, in the ⑪ sub-diagram (D3).**
  Two arrows rise from beneath the two-cell box and two label lines stand to the right.
  The arrow under the **left** digit drops the full depth and runs right to the **lower**
  line, `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`; the arrow under the **right** digit
  drops a short distance and runs right to the **upper** line, `0: OFF, 1: TONE, 2: TSQL`.
  Read the labels top-to-bottom and you get the two half-bytes the wrong way round. Each
  leader was followed by eye at 400 dpi (crop `R_leaders.png`, 400 % enlargement) and
  again at 600 dpi in pass 2, with the same result. The ③ sub-diagram (D2) is **not**
  reversed: its left arrow goes straight down to `Fixed` and its right arrow to the
  bracketed `0=OFF / 1= ★1 / 2= ★2 / 3= ★3` list.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED, but
  both values are recorded anyway.** The one place a block repeats another is
  ⑮ ~ ⑰ (tone squelch), which repeats the structure of ⑫ ~ ⑭ (repeater tone). Both rows
  carry the printed index and, in `notes`, the position measured on the render — the 10th,
  11th and 12th drawn cells for ⑫ ~ ⑭ and the 13th, 14th and 15th for ⑮ ~ ⑰, out of 18
  drawn cells of which the 5th and 17th are dashed ellipsis cells. The two are not
  reconciled in the CSV; for the record, they do not conflict, because the ellipsis cells
  account for the difference between drawn-cell number and byte number.

## STOP findings

1. **A quoted cross-reference title does not match the heading it points at.**
   PDF page 20 (folio 19), right column, on the line immediately below
   `⑮ ~ ⑰: Tone squelch frequency setting` and serving both that field and
   `⑫ ~ ⑭: Repeater tone frequency setting`, the document prints:
   `ⓘ See “• Repeater tone/tone squelch settings.” (p. 23)`.
   The heading actually printed at folio 23 (PDF page 24), top of the left column, is
   `• Repeater tone/tone squelch frequency settings` — the word *frequency* is present
   there and absent from the quotation. No section titled
   `• Repeater tone/tone squelch settings` is printed on folio 22 (PDF 23), folio 23 or
   folio 24 (PDF 25), which were rendered and checked. This stops because it is one
   printed string contradicting another printed string about the same section. It does
   **not** put the transcribed values in doubt: folio 23 holds the only
   repeater-tone/tone-squelch format diagram in the document, and its three bytes match
   the three-byte extent of each field. Both affected rows carry `STOP 1` in `notes` and
   the quotation is transcribed exactly as seen.

2. **The same two-byte field is given two different names on one page.**
   PDF page 20 (folio 19). In the left-column legend, immediately under the data block,
   the first line reads `①, ②: Memory group number`, and its own value list beneath it
   reads `00 01 ~ 00 99: Memory channel 01 ~ 99`. Lower on the same page, in the right
   column under the heading `To clear the memory channel contents on 1A 00:`, the first
   line reads `①, ②:  Memory channel (00 01~00 99)`. The same index pair on the same page
   is labelled *Memory group number* in one place and *Memory channel* in the other, and
   the value list printed under the first label describes memory channels. This stops
   because it is printed material contradicting printed material. The codes themselves are
   not in doubt and are transcribed exactly as printed. The row for `①, ②` carries
   `STOP 2` in `notes`; the label recorded in `label_verbatim` is the one printed against
   the field in the data block's own legend, `Memory group number`.

No other STOP arose. Confidence in the remainder rests on: the width arithmetic closing
exactly on 27 (see Method); an unbroken, in-order, single-styled index run ① … ㉗; every
brace boundary landing on a drawn cell boundary when measured on the render; and a second
pass at a different dpi with different crop windows returning no disagreement in any cell.

## Observed disagreements

Recorded as printed, not resolved.

1. **The block's alternating shading breaks at the last two field groups.** Measured
   background colours (Method, step 3) run grey, grey / white / grey, grey, grey / white,
   white / grey / white, white, white — one shade per field group, alternating — and then
   grey, grey, grey for ⑮ ~ ⑰ **and grey again** for all three drawn cells of ⑱ ~ ㉗. The
   last two groups are not distinguished by shading the way every earlier pair is.

2. **The `To clear the memory channel contents on 1A 00:` block re-uses the indices
   ①, ②, ③ and ④** (right column, PDF page 20) for a different statement about the same
   command, in the same circled style as the data block's own indices. It also prints
   `④` on its own, where the data block prints only the range `④ ~ ⑧`, and gives it the
   value `None`; that is recorded as a conditional-extent row in the CSV (row 4), not as a
   STOP, following the brief's rule for conditional widths. The index run **inside** the
   data block is unbroken and in order, ① … ㉗, which is why this re-use is filed here and
   not under STOP findings.

3. **The `③` line's two printed statements sit in different places.** The value set
   (`0=OFF`, `1= ★1`, `2= ★2`, `3= ★3`) is printed only inside the sub-diagram, while the
   qualifier `ⓘ Set 0 for P1 and P2.` is printed below it, and the clear-contents block
   assigns the same index the value `FF`. Three separate printed statements about one
   byte, none of which references the others.

4. **The operating-mode code set printed on folio 17 is sparse.** It runs
   `00, 01, 02, 03, 04, 05, 07, 08, 12, 13`; codes `06` and `09` to `11` are not printed,
   with no note explaining the omission. Transcribed as printed, not expanded.

5. **Inconsistent spacing in two adjacent labels on folio 17.** The frequency diagram
   prints `1 GHz digit: 0  (Fixed)` with two spaces before the bracket and
   `100 MHz digit: 0 (Fixed)` with one. Recorded in the CSV with a single space in both,
   since the wider gap is the leader-line gutter, not a space run carrying meaning.

6. **Two typographic glyphs are printed in the symbols table where ASCII names a
   different character.** The entry against `22` is drawn as a closing curly double quote
   `”`, and the entry against `27` as a closing curly single quote `’`; the entry against
   `2D` is drawn as a dash of en-dash length. Transcribed as drawn.

7. **The `- Character codes— Letters and Numbers` table leaves two cells empty and rules
   them through** with a diagonal line, rather than printing a dash or leaving them blank.
   No value is printed in them and none is recorded.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file,
manual, transcription, source file, generated artefact or web resource was opened, and no
directory was listed.
