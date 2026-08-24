# IC-7300MK2 CI-V — memory-record data-block geometry witness

## Source

- Document title, as printed on the cover (PDF page 1): the Icom logo band, then
  `CI-V REFERENCE GUIDE`; below it `HF/50 MHz TRANSCEIVER` and `IC-7300MK2`; at the
  foot `Icom Inc.`
- Revision code, as printed: `A7841-8EX`. It is printed at the foot of the back cover
  (PDF page 27), in the right-hand block, on the line immediately above
  `© 2025  Icom Inc.     Oct. 2025`. No revision code is printed on the front cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300mk2_civ_ref_0.pdf`
- Page count: 27 PDF pages.

## Extent

Rendered:

| PDF page | dpi rendered | printed folio | contribution |
|---|---|---|---|
| 1 | 200 | none printed | cover title, read for `## Source` only |
| 15 | 300 | not read | rendered as part of the locating sweep; not read for any value |
| 16 | 300, 400 | `16` | read to establish what precedes the material; carries `◇ Command formats` diagrams for Operating frequency, Operating mode, Band edge frequency settings, and the right column ending in the `Turning the transceiver ON` example. No memory-record data block here. |
| 17 | 300, 400, 600 | `17` | **the page transcribed.** Carries the one memory-record data-block diagram. |
| 18 | 300, 400 | `18` | read to establish what follows; carries `• Codes for character entries`, two character-code tables and the `Cmd. / Sub cmd. / Setting item` table. No memory-record data block here. |
| 19 | 300 | not read | rendered as part of the locating sweep; not read for any value |
| 27 | 200, 400 | none printed | back cover, read for the revision code only |

On PDF pages 16, 17 and 18 the printed folio equals the PDF page number; that was checked
on the render of each of those three pages and on no others.

The material transcribed begins and ends on PDF page 17:

- Immediately above it, in printed order: the running head `REMOTE CONTROL`; the reversed
  grey band `Remote control (CI-V) information`; `◇ Command formats`; then
  `• Memory channel content` and, on the next line, `Command: 1A 00`.
- Then the diagram transcribed: a single horizontal byte strip with a band of bracketed,
  circled index numerals above it.
- Immediately below it, in printed order: the two-column legend, beginning at the left with
  `①, ② Memory channel number` and at the right with `⑪ Data mode and tone type settings`.
  The page's last printed matter is the grey `NOTE:` box in the right column, then the folio
  `17`. The lower half of the page is blank.

**Diagram identifiers.** There is exactly one memory-record data-block diagram on PDF page 17.

- **D1** — the byte strip printed directly beneath the caption `• Memory channel content`
  and its second caption line `Command: 1A 00`. No numbered or lettered caption of its own
  is printed; those two lines are the only printed heading it has, and they are the caption
  by which D1 is defined here.

Two further boxed figures appear on the page — a one-cell box under `③ Split and Select
memory setting` with upward arrows to `SPLIT` and `SELECT`, and a one-cell box under
`⑪ Data mode and tone type settings` with upward arrows to `DATA` and `TONE`. Neither is a
record diagram: each shows a single byte, each is an expansion of one cell of D1, and
neither contains any numbered field inside it — the halves are labelled with names, not with
indices. They therefore contribute no CSV row. They were read, and they are the evidence for
the nibble-order statement below.

**Nibble labelling in this document.** D1 prints no nibble numerals of any kind. Each byte
cell contains two `X` glyphs separated by a dotted vertical rule at the cell's centre; that
dotted rule is the only nibble marking. The two expansion boxes name the halves instead of
numbering them, with vertical arrows that rise straight from the label to the half above it
and do not cross: in the `③` box the left-printed half is `SPLIT` and the right-printed half
is `SELECT`; in the `⑪` box the left-printed half is `DATA` and the right-printed half is
`TONE`. So the diagram distinguishes the two halves of a byte and orders them left-to-right,
but never numbers them. Nibble 1 / nibble 2 in the CSV is therefore purely the recording
convention given in the task (nibble 1 = the half printed first, i.e. leftmost), not
anything the document prints.

**Byte-position numbering in this document.** D1 prints no byte-position numerals. The
numerals it prints (`①` … `㉝`, and a second `❹` … `⓱`) are field indices carried on
brackets and on two standalone circles, not byte addresses. Positions in the CSV were
therefore obtained by counting cells from the diagram's own first cell, as the task directs
for that case. The counting rule used is stated in full under `## Position arithmetic`.

## Method

1. **Locate.** `pdftoppm -png -r 300 -f 15 -l 19 <pdf> <out>/r300/p` into the fresh
   directory `.../evidence/ic7300mk2-W/r300`, which did not exist before this task. The five
   renders were read as images; PDF page 17 was the only one carrying a memory-record data
   block, and its printed section heading `Remote control (CI-V) information` /
   `◇ Command formats` / `• Memory channel content` was matched by eye before any value was
   taken. Cover and back cover were rendered separately at 200 dpi for `## Source`.
2. **Read (first pass).** `pdftoppm -png -r 400 -f 16 -l 18 <pdf> <out>/r400/p`. Every
   first-pass value was read from `r400/p-17.png` (3308 × 4678 px).
3. **Crop and enlarge.** ImageMagick was available (`/opt/homebrew/bin/magick`) and used.
   First-pass crops:
   - `magick r400/p-17.png -crop 2500x300+420+1000 +repage -resize 200% crops/strip_full.png`
   - `for i in 0 1 2 3; do x=$((460 + i*600)); magick r400/p-17.png -crop 640x260+${x}+1030 +repage -resize 300% crops/seg${i}.png; done`
   Second-pass crops (different window origins, different tile width, different enlargement,
   different dpi):
   - `pdftoppm -png -r 600 -f 17 -l 17 <pdf> <out>/r600/p` (4961 × 7016 px)
   - `for i in 0 1 2 3 4; do x=$((640 + i*760)); magick r600/p-17.png -crop 800x420+${x}+1540 +repage -resize 250% crops2/s${i}.png; done`
   - `magick r600/p-17.png -crop 900x600+520+2500 +repage -resize 200% crops2/box3.png`
   - `magick r600/p-17.png -crop 900x600+2650+2050 +repage -resize 200% crops2/box11.png`
   - `magick r400/back-27.png -crop 1000x160+2350+4290 +repage -resize 300% crops2/revcode.png`
   At these enlargements every circled numeral, every cell rule, every dotted nibble divider
   and every bracket leg sits clear of its neighbours.
4. **tesseract.** Available (`/opt/homebrew/bin/tesseract`) but **not used**. No OCR was run
   on any crop; every numeral recorded was read by eye off the enlarged renders.
5. **`pdftotext`.** **Not run at all**, in any form, on this or any other file. Navigation to
   PDF page 17 was done entirely by reading the 300 dpi page images.
6. **Edge measurement as a reading aid.** A short Pillow script was run over the rendered
   PNGs to report the x-coordinates of the strip's full-height rules, its half-height dotted
   rules, the y of the strip's top and bottom borders, the cell fill greys, and the x at
   which each bracket leg meets the strip. This measures the *same rendered image* the eye
   read — it is not a text extraction and not an OCR — and every coordinate it returned was
   confirmed against the enlarged crops before being used. It is reported here because the
   coordinates appear in this document.
7. **Scope of file access.** The only file opened for reading was the PDF named in
   `## Source`. The only directories listed were `r300/`, `r400/`, `r600/` and `crops/`,
   `crops2/` — the render and crop directories created by this task inside
   `.../evidence/ic7300mk2-W` — and those listings were `ls` of files this task had just
   written, to confirm the renders existed. No repository, manual, transcription, prior
   output or other directory was listed or opened.

### Second independent pass

Both passes were done. The second pass re-read every value from a different raster:
**600 dpi instead of 400 dpi** (a re-render from the PDF, not an upscale of the first),
**five 800 px tiles at 250 % starting at x = 640 with a 760 px stride**, against the first
pass's **four 640 px tiles at 300 % starting at x = 460 with a 600 px stride**, so no tile
boundary of the second pass fell where a tile boundary of the first pass had fallen, and no
cell was cut at the same place twice. The edge-coordinate script was re-run independently
against the 600 dpi raster.

Cross-check of the two rasters (400 dpi coordinate → 600 dpi coordinate; expected ratio 1.5):

| feature | 400 dpi x | 600 dpi x | ratio |
|---|---|---|---|
| strip left outer border | 481 | 722 | 1.501 |
| divider, cell 2 / cell 3 | 701 | 1053 | 1.502 |
| divider, elision cell / next cell | 1032 | 1549 | 1.501 |
| right end of the solid-bordered run | 2129 | 3195 | 1.501 |
| left border of the right-hand group | 2460 | 3691 | 1.500 |
| strip right outer border | 2790 | 4186 | 1.500 |

Both passes returned: 16 full-height rules bounding 15 cell positions in the left-hand run;
14 half-height dotted nibble dividers in that run, with the divider missing from exactly one
cell (the `...` cell); a 3-cell-pitch-wide dotted region; then 4 full-height rules bounding
3 cell positions in the right-hand group, with 2 dotted nibble dividers and the divider
missing from exactly one cell (the second `...` cell); 10 bracket-leg touch-points; and the
two standalone circled numerals centred over cell 3 and cell 8 respectively.

**Cells where the two passes disagreed: none.** Every cell inventory item, every bracket
extent, every index numeral, every shading value and every nibble divider matched between
the passes. No third render was needed to settle anything.

## Position arithmetic, per diagram

### D1 — `• Memory channel content` / `Command: 1A 00` (PDF page 17)

**Counting rule used, stated so the sums can be checked without the images.** The strip is
drawn as a run of equal-width cells. A cell that contains two `X` glyphs and a dotted centre
divider is a byte cell and is counted. A cell that contains an ellipsis (`...`) or a long
dotted rule, has no `X` and no dotted centre divider, and is bounded by dashed rather than
solid rules is an **elision mark**: it stands for an unstated number of omitted cells and is
therefore **not counted as a byte**. Byte positions below are the ordinal of each byte cell,
counting from the diagram's own first cell. Nothing was calculated from prose, from a printed
width, or from another field's position.

**Cell inventory, left to right, as measured.** Boundary x is the centre of the full-height
rule, at 400 dpi (600 dpi in brackets). Mean cell pitch measured 109.9 px at 400 dpi and
165.3 px at 600 dpi — i.e. the same 6.99 mm cell at both rasters.

| # | left rule | right rule | width (px, 400 dpi) | fill | nibble divider | content | counted as |
|---|---|---|---|---|---|---|---|
| A | 481 [722] | 591 [888] | 110 | grey (221) | yes, x 537 | `X:X` | byte 1 |
| B | 591 [888] | 701 [1053] | 110 | grey (221) | yes, x 647 | `X:X` | byte 2 |
| C | 701 [1053] | 811 [1218] | 110 | white (255) | yes, x 757 | `X:X` | byte 3 |
| D | 811 [1218] | 922 [1384] | 111 | grey (221) | yes, x 867 | `X:X` | byte 4 |
| E | 922 [1384] | 1032 [1549] | 110 | grey (221) | **none** | `...` | elision mark — not a byte |
| F | 1032 [1549] | 1142 [1714] | 110 | grey (221) | yes, x 1088 | `X:X` | byte 5 |
| G | 1142 [1714] | 1252 [1880] | 110 | white (255) | yes, x 1198 | `X:X` | byte 6 |
| H | 1252 [1880] | 1363 [2045] | 111 | white (255) | yes, x 1308 | `X:X` | byte 7 |
| I | 1363 [2045] | 1473 [2210] | 110 | grey (221) | yes, x 1418 | `X:X` | byte 8 |
| J | 1473 [2210] | 1583 [2376] | 110 | white (255) | yes, x 1529 | `X:X` | byte 9 |
| K | 1583 [2376] | 1693 [2541] | 110 | white (255) | yes, x 1639 | `X:X` | byte 10 |
| L | 1693 [2541] | 1804 [2706] | 111 | white (255) | yes, x 1749 | `X:X` | byte 11 |
| M | 1804 [2706] | 1914 [2872] | 110 | grey (221) | yes, x 1859 | `X:X` | byte 12 |
| N | 1914 [2872] | 2024 [3037] | 110 | grey (221) | yes, x 1970 | `X:X` | byte 13 |
| O | 2024 [3037] | 2129 [3195] | 105 | grey (221) | yes, x 2080 | `X:X` | byte 14 |
| — | 2129 [3195] | 2460 [3691] | 331 (= 3.01 pitches) | white (255) | **none** | one long dotted rule, dashed outline | elision mark — not a byte |
| P | 2460 [3691] | 2570 [3857] | 110 | grey (221) | yes, x 2516 | `X:X` | byte 15 |
| Q | 2570 [3857] | 2681 [4022] | 111 | grey (221) | **none** | `...` | elision mark — not a byte |
| R | 2681 [4022] | 2790 [4186] | 109 | grey (221) | yes, x 2737 | `X:X` | byte 16 |

Total byte cells drawn in D1: **16**. Total elision marks: **3** (E, the wide one, Q).

**Bracket and standalone-label extents, as measured.** x is where the bracket leg meets the
top of the strip (400 dpi [600 dpi]); a `V` means the closing leg of one bracket and the
opening leg of the next meet at one point.

| printed index | numeral styling | left leg at | right leg at | cells enclosed |
|---|---|---|---|---|
| `①, ②` | outlined circles | 484 [727] = A's left outer border | 697 [1045] = B/C divider | A, B |
| `③` | outlined circle, no bracket | circle centred x 1136 [600 dpi]; cell C centre x 1135.5 | — | C |
| `④ ~ ⑧` | outlined circles | 813 [1221] = C/D divider | 1143 [1715] = F/G divider (`V`) | D, E, F |
| `⑨, ⑩` | outlined circles | 1143 [1715] (same `V`) | 1358 [2037] = H/I divider | G, H |
| `⑪` | outlined circle, no bracket | circle centred x 2128 [600 dpi]; cell I centre x 2127.5 | — | I |
| `⑫ ~ ⑭` | outlined circles | 1476 [2215] = I/J divider | 1804 [2706] = L/M divider (`V`) | J, K, L |
| `⑮ ~ ⑰` | outlined circles | 1804 [2706] (same `V`) | 2129 [3195] = end of solid run (`V`) | M, N, O |
| `❹ ~ ⓱` | **filled black discs, reversed white numerals** | 2129 [3195] (same `V`) | 2461 [3692] = P's left border (`V`) | none — the wide elision mark only |
| `⑱ ~ ㉝` | outlined circles | 2461 [3692] (same `V`) | 2787 [4181] = R's right outer border | P, Q, R |

**Running position, field by field.** "Measured" is the counting rule above. "Printed" is the
index range as it is printed on the render. The two are set side by side and are **not**
reconciled.

| step | printed index | measured start | measured extent | measured end | next measured start | printed index span |
|---|---|---|---|---|---|---|
| 1 | `①, ②` | byte 1, nibble 1 | 2 byte cells (A, B) | byte 2, nibble 2 | byte 3 | 2 indices (1–2) |
| 2 | `③` | byte 3, nibble 1 | 1 byte cell (C) | byte 3, nibble 2 | byte 4 | 1 index (3) |
| 3 | `④ ~ ⑧` | byte 4, nibble 1 | 2 byte cells (D, F) + 1 elision mark (E) | byte 5, nibble 2 | byte 6 | 5 indices (4–8) |
| 4 | `⑨, ⑩` | byte 6, nibble 1 | 2 byte cells (G, H) | byte 7, nibble 2 | byte 8 | 2 indices (9–10) |
| 5 | `⑪` | byte 8, nibble 1 | 1 byte cell (I) | byte 8, nibble 2 | byte 9 | 1 index (11) |
| 6 | `⑫ ~ ⑭` | byte 9, nibble 1 | 3 byte cells (J, K, L) | byte 11, nibble 2 | byte 12 | 3 indices (12–14) |
| 7 | `⑮ ~ ⑰` | byte 12, nibble 1 | 3 byte cells (M, N, O) | byte 14, nibble 2 | byte 15 | 3 indices (15–17) |
| 8 | `❹ ~ ⓱` | UNREADABLE — no byte cell is drawn | 0 byte cells, 1 elision mark 3.01 pitches wide | UNREADABLE | byte 15 | 14 indices (4–17) |
| 9 | `⑱ ~ ㉝` | byte 15, nibble 1 | 2 byte cells (P, R) + 1 elision mark (Q) | byte 16, nibble 2 | end of strip | 16 indices (18–33) |

**Where the running total and the printed numbering agree.** Steps 1, 2 and the *start* of
step 3: the measured byte ordinal equals the printed index at bytes 1, 2, 3 and 4. That
agreement is what fixes the counting origin: index `①` is measured on the strip's first cell.

**Where they disagree — every one of these is a STOP, recorded and not resolved:**

- step 3, end: printed `⑧`, measured byte 5 (difference 3, the width of the elision mark E
  expressed in indices — stated as an observation, not as a resolution).
- step 4: printed `⑨, ⑩`, measured bytes 6, 7.
- step 5: printed `⑪`, measured byte 8.
- step 6: printed `⑫ ~ ⑭`, measured bytes 9, 10, 11.
- step 7: printed `⑮ ~ ⑰`, measured bytes 12, 13, 14.
- step 8: printed `❹ ~ ⓱`, no measurable position at all; and these fourteen indices are
  printed for the second time in this one diagram.
- step 9: printed `⑱ ~ ㉝`, measured bytes 15, 16; and the printed range names 16 indices
  where 2 byte cells are drawn.

**Sum check.** Measured: 2 + 1 + 2 + 2 + 1 + 3 + 3 + 0 + 2 = 16 byte cells, which equals the
16 byte cells counted in the inventory. Printed: 2 + 1 + 5 + 2 + 1 + 3 + 3 + 14 + 16 = 47
indices. 16 ≠ 47. The difference, 31, is carried entirely by the three elision marks, which
are drawn 1, 3 and 1 cell pitches wide (5 pitches in total) and are therefore **not drawn to
scale for the number of cells they elide**. Recorded; not resolved.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** D1 draws its index
  numerals in two distinct ways. `①`–`③`, `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭`, `⑮ ~ ⑰` and
  `⑱ ~ ㉝` are drawn as black outlined circles with white interiors and black numerals. The
  single range `❹ ~ ⓱` is drawn as solid filled black discs with the numerals reversed out
  in white. The styling difference is unmistakable at 600 dpi enlarged (see
  `crops2/s3.png`). Both styles are recorded exactly as drawn; no inference is drawn here as
  to what the filled style means, and the two styles are not normalised to one.
- **(b) Vector group with rotated labels defeating text extraction — CANNOT DETERMINE.** No
  text extraction was performed at any point (`pdftotext` was never run, tesseract was never
  run), so whether this document's text layer would extract the diagram out of order was
  never observed either way. Every position recorded was read from the picture regardless.
  No rotated label appears anywhere in D1: all nine index labels sit upright above the strip.
- **(c) Leader-line label order reversed — NOT ENCOUNTERED.** D1 uses brackets whose legs
  descend directly onto the cell boundaries they mark, and two standalone circles centred
  directly over the cell they mark; every leg was followed by eye from label to landing
  point and its landing x measured on both rasters. The only true leader-arrow labels on the
  page are in the two one-cell expansion boxes (`SPLIT`/`SELECT` and `DATA`/`TONE`), where
  the arrows rise vertically and do not cross: the left label points at the left half and the
  right label at the right half in both boxes.
- **(d) A printed index differs from a field's measured position — ENCOUNTERED.** The block
  `❹ ~ ⓱` repeats indices 4 to 17, which are already printed earlier in the same strip as
  `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭` and `⑮ ~ ⑰`, and it sits at a wholly different measured
  position — to the right of `⑮ ~ ⑰`, not at bytes 4–14. Both occurrences were measured
  separately and both are recorded separately (rows 3–7 and row 8 of the CSV); the second was
  not assumed to match the first, and neither printed index was adjusted to fit any measured
  position. From step 4 onwards every printed index also differs numerically from its
  measured byte ordinal, as tabulated above.

## STOP findings

1. **PDF page 17, D1, the `④ ~ ⑧` bracket.** Anchor: the bracket whose left leg lands on the
   divider between the white `③` cell and the shaded cell to its right (x 813 at 400 dpi),
   and whose right leg closes in a `V` at x 1143. Printed: the index range `④ ~ ⑧`, naming
   five indices. Measured: the bracket encloses three cell positions — a byte cell (D), a
   shaded `...` cell with no `X` and no nibble divider (E), and a byte cell (F) — so two byte
   cells, ending at measured byte 5. Why it stops: the measured extent (2 byte cells) does
   not add up to the printed span (5 indices). CSV row carries the measured values 4/1 to
   5/2 exactly as counted, with the printed index `④ ~ ⑧` verbatim beside them. Not resolved.
2. **PDF page 17, D1, the `⑨, ⑩` bracket.** Anchor: the two unshaded `X:X` cells immediately
   right of the `V` at x 1143. Printed: indices 9 and 10. Measured: byte cells 6 and 7,
   counted from the strip's first cell. Why it stops: the running measured total and the
   printed numbering disagree. Both recorded; neither resolved.
3. **PDF page 17, D1, the `⑪` circle.** Anchor: the shaded `X:X` cell at 400 dpi x 1363–1473,
   with an outlined circled 11 centred above it (circle centre measured x 2128 at 600 dpi
   against a cell centre of x 2127.5). Printed: index 11. Measured: byte cell 8. Why it
   stops: running total and printed numbering disagree.
4. **PDF page 17, D1, the `⑫ ~ ⑭` bracket.** Anchor: the run of three unshaded `X:X` cells
   beginning immediately right of the shaded `⑪` cell. Printed: indices 12 to 14. Measured:
   byte cells 9 to 11. Why it stops: running total and printed numbering disagree. The extent
   itself does add up — 3 cells for 3 indices.
5. **PDF page 17, D1, the `⑮ ~ ⑰` bracket.** Anchor: the run of three shaded `X:X` cells that
   ends the solid-bordered part of the strip, at 400 dpi x 1804–2129. Printed: indices 15 to
   17. Measured: byte cells 12 to 14. Why it stops: running total and printed numbering
   disagree. The extent itself does add up — 3 cells for 3 indices.
6. **PDF page 17, D1, the `❹ ~ ⓱` bracket — repeated index range.** Anchor: the bracket
   between the `V` at 400 dpi x 2129 and the `V` at x 2461, labelled with a filled black disc
   `4`, a swung dash, and a filled black disc `17`. Printed: indices 4 to 17, which are the
   same numerals already printed on this same strip at `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭` and
   `⑮ ~ ⑰`, there in outlined style, here in reversed filled style. Why it stops: the index
   sequence is discontinuous — it runs 1…17, then restarts at 4 and runs to 17 again, then
   resumes at 18; and fourteen indices are printed twice in one diagram with two different
   stylings. Both occurrences are recorded, in rows 3–7 and row 8; neither is adjusted.
7. **PDF page 17, D1, the `❹ ~ ⓱` bracket — no cell to count.** Anchor: the same bracket; the
   region beneath it, at 400 dpi x 2129–2460, is a dashed-outlined white region containing a
   single long dotted rule, with no `X` glyph, no cell rule and no nibble divider anywhere
   inside it. Printed: fourteen indices. Measured: zero byte cells; the region is 331 px wide
   at 400 dpi = 3.01 cell pitches. Why it stops: no cell can be counted, so no byte or nibble
   position can be measured for any field in this block. CSV row carries `UNREADABLE` in all
   four position columns. Not interpolated from the neighbouring blocks, and not calculated
   from the printed index range.
8. **PDF page 17, D1, the `⑱ ~ ㉝` bracket.** Anchor: the three cells at the extreme right of
   the strip — shaded `X:X`, shaded `...`, shaded `X:X` — under the bracket whose right leg
   lands on the strip's right outer border at 400 dpi x 2787. Printed: indices 18 to 33,
   naming sixteen indices. Measured: two byte cells (P and R) with one elision mark (Q)
   between them, at measured byte ordinals 15 and 16. Why it stops: the measured extent (2
   byte cells) does not add up to the printed span (16 indices); and the printed index `⑱`
   follows `⑰` numerically while sitting positionally after the whole `❹ ~ ⓱` block. Both
   recorded; not resolved.
9. **PDF page 17, D1, the three elision marks — widths that do not add up.** Anchors: the
   shaded `...` cell at 400 dpi x 922–1032; the dashed white region at x 2129–2460; the
   shaded `...` cell at x 2570–2681. Measured widths, against a mean cell pitch of 109.9 px
   at 400 dpi and 165.3 px at 600 dpi: 1.00, 3.01 and 1.01 pitches respectively — 5.02 cell
   pitches in total. The printed index ranges they sit inside require them to stand for 3,
   14 and 14 omitted indices respectively, 31 in total. Why it stops: the marks are drawn at
   whole-cell widths that are neither equal to nor proportional to the number of cells they
   elide, so no cell count can be recovered from their width; the arithmetic 16 measured byte
   cells versus 47 printed indices does not close. Referenced from the notes of the three CSV
   rows that contain an elision mark (rows 3, 8 and 9). Not resolved.

## Observed disagreements

- **Cell O is drawn narrow.** The last cell of the solid-bordered run measures 105 px wide at
  400 dpi and 158 px at 600 dpi, against a mean pitch of 109.9 / 165.3 — about 4–5 % short.
  The shortfall reproduces identically on both rasters, so it is in the document rather than
  in the rendering. The cell is nonetheless unambiguous: it is solidly bordered, shaded, and
  carries both `X` glyphs and a dotted nibble divider at x 2080 (400 dpi), 54 px from its left
  rule and 51 px from its right. It was counted as one byte cell. Did not stop the reading.
- **Shading alternates by block, except across the wide elision.** Fill grey (level 221)
  versus white (255) alternates block by block down the strip: `①,②` grey, `③` white,
  `④ ~ ⑧` grey, `⑨, ⑩` white, `⑪` grey, `⑫ ~ ⑭` white, `⑮ ~ ⑰` grey — and then
  `⑱ ~ ㉝` grey as well, two grey blocks in succession, separated only by the `❹ ~ ⓱`
  elision region, whose interior is white. Recorded as seen; no meaning inferred.
- **Two label idioms for the same thing.** A block of one cell is labelled with a bare
  circled numeral centred over the cell (`③`, `⑪`); a block of two or more cells is labelled
  with a bracket carrying either a comma-separated pair (`①, ②`; `⑨, ⑩`) or a swung-dash
  range (`④ ~ ⑧`; `⑫ ~ ⑭`; `⑮ ~ ⑰`; `❹ ~ ⓱`; `⑱ ~ ㉝`). Both idioms were transcribed
  verbatim into `field_index`; neither was normalised to the other.
- **`⑨, ⑩` is a comma pair over two adjacent cells whereas `⑫ ~ ⑭` is a range over three.**
  Recorded as printed; the choice between comma and swung dash appears to follow the count,
  but nothing printed says so and nothing here assumes it.
- **The elision mark between P and R prints `...` while the `❹ ~ ⓱` region prints a long
  dotted rule.** Two different elision glyphs are used in one diagram. Recorded as seen.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual,
transcription, source file, generated artefact or web resource was opened, and no directory
was listed.
