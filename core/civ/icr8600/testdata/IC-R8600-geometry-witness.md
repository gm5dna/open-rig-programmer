# IC-R8600 — memory-record data-block geometry witness (leg W)

Companion to `IC-R8600-geometry-witness.csv`. Every value in both files was
read from rendered page images of the one PDF named below.

## Source

- Document title, as printed on the cover: **CI-V REFERENCE GUIDE**, above
  **COMMUNICATIONS RECEIVER** / **IC-R8600**, with the ICOM logo in the black
  panel at the head of the page and **Icom Inc.** at the foot.
- Revision code, as printed: **A7375-2EX-3a**. It is printed at the foot of the
  left-hand half of the last page (PDF page 28, the back cover), on the line
  immediately above `© 2017–2018 Icom Inc.`. No revision code is printed on the
  cover.
- File path:
  `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/icr8600_civ_3a.pdf`
- Page count: **28 PDF pages** (A4, 595.276 × 841.89 pt).

## Extent

Rendered and read:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none printed | rendered **and read** — cover; document title and model, for `## Source` |
| 2 | none printed | rendered only, not opened |
| 10 | 9 | rendered only, not opened |
| 11 | 10 | rendered only, not opened |
| **12** | **11** | rendered **and read** — **D1**, the main memory-channel data block: three linked strip rows, indices ① – ㊶ |
| **13** | **12** | rendered **and read** — **D2** (FM), **D3** (P25), **D4** (D-STAR) mode-tail strips |
| **14** | **13** | rendered **and read** — **D5** (dPMR), **D6** (NXDN) mode-tail strips |
| **15** | **14** | rendered **and read** — **D7** (DCR) mode-tail strip, left column only |
| 16 | 15 | rendered **and read**, for navigation only: to confirm that the adjacent `Programmable scan start (remote) data` section continues here and that no memory-record diagram does. Nothing transcribed from it |
| 17 | 16 | rendered only, not opened |
| 28 | none printed | rendered **and read** — back cover; revision code, for `## Source` |

The printed folio is the PDF page number minus one throughout the range read
(PDF 12 → folio 11, PDF 13 → 12, PDF 14 → 13, PDF 15 → 14), read off the page
foot of each render.

**Where the transcribed material begins.** PDF page 12. Immediately above the
first strip row, in this order: the running head `Remote control` with its rule;
the section marker `◇ Command formats (Continued)`; the bulleted heading
`●Memory channel content`; and the line `Command: 1A 00`. The three linked rows
of D1 follow directly beneath.

**Where it ends.** PDF page 15, left column. The DCR strip (D7) is the last
memory-record data-block diagram in the document's Memory channel content
section. Printed immediately after it are that mode's field-detail boxes (㊷
Digital squelch (D.SQL) type; ㊸, ㊹ UC code; ㊺ Encryption setting; ㊻ ~ ㊽
Encryption key), and then the paragraph beginning `Command 1A 00 clears a memory
channel by sending the command in the following format.` with its four lines
`①, ② :`, `③, ④ :`, `⑤ :` and `⑥ ~ :`.

**What is inside the pages named but deliberately excluded, and why.**

1. *The right-hand column of PDF page 15* carries a different bulleted section:
   `●Programmable scan start (remote) data` / `Command: 1A 0B 00`. It draws its
   own three-row data-block strip whose indices also start at ①, and it
   continues onto PDF page 16 with its own set of `For receiving a … signal`
   headings. Its heading closely resembles the in-scope one and its diagrams
   closely resemble the in-scope ones, but it is not a memory record: it belongs
   to command `1A 0B 00`, not `1A 00`. No row of the CSV comes from it.
2. *The field-detail boxes* that follow each strip (for example `⑤ Skip/Select
   Memory scan setting`, `⑱, ⑲ Tuning step (TS)`, `㊸ ~ ㊺ NAC`). These draw one
   field enlarged, with leader lines to nibble-level meanings; they are not
   data-block diagrams of the record and they carry no record-level byte
   position. They were read closely all the same, because they are what
   establishes the drawing convention below, and one observation from them is
   recorded under `## Observed disagreements`. **This scoping is a judgement
   call**, recorded here as one: the CSV holds only the record strips D1 – D7.

**The drawing convention, established by measurement, not assumed.** In every
in-scope diagram a solid-ruled box contains two `X` glyphs separated by a
*dashed* vertical rule, and adjacent boxes are separated by a *solid* vertical
rule. One circled index numeral sits over exactly one such box. The field-detail
boxes confirm the reading directly: `⑤ Skip/Select Memory scan setting` draws a
single box `X ¦ X` with the numeral ⑤ over the whole box and two leader arrows,
one to each `X`. So: **one solid-ruled box = one byte; one `X` = one nibble; one
circled index = one byte.** Every field in the CSV is therefore whole-byte
aligned, first nibble 1 and last nibble 2, and no field in any in-scope diagram
begins or ends mid-byte.

**A dotted-outline cell containing three centred dots (`•••`)** appears three
times in D1 and nowhere else in scope. It is drawn where cells have been elided.
See STOP 2.

**On the Tier 4b `mode_class` instruction.** The brief's hazard clause asks for
every mode class the document draws as its own row set, "joined by a
`mode_class` column"; the brief's CSV rules give a fixed header and forbid extra
columns. **Judgement call, recorded:** the fixed header wins, and the mode class
is carried instead by `diagram_id` (one id per mode class) and by
`block_label_verbatim`, which holds that mode block's printed heading verbatim
on every one of its rows. All six mode classes the document draws are present as
their own row sets; none is merged.

## Method

1. **Locate — 300 dpi.** Fresh directory
   `…/legs-out/icr8600/W/r300`, created empty for this leg:
   `pdftoppm -png -r 300 -f 1 -l 1 <pdf> …/r300/p` and
   `pdftoppm -png -r 300 -f 10 -l 17 <pdf> …/r300/p` (later also `-f 2 -l 2`
   and `-f 28 -l 28`). Read as images to find the sections, to check the
   printed folios, and to confirm which column of PDF page 15 belongs to which
   bulleted heading.
2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f 12 -l 15 <pdf> …/r400/p`
   (3308 × 4678 px per page). First pass of every recorded value read from
   these.
3. **Crop and enlarge.** ImageMagick **was** available (`/opt/homebrew/bin/magick`
   and `convert`) and was used. First-pass crops, into `…/W/crops`:
   - `magick r400/p-12.png -crop 1450x820+330+810 +repage -resize 200% crops/D1_main_400.png`
   - `magick r400/p-13.png -crop 1050x330+380+990 +repage -resize 300% crops/D2_FM_strip_400.png`
   - `magick r400/p-13.png -crop 800x330+1780+990 +repage -resize 300% crops/D3_P25_strip_400.png`
   - `magick r400/p-13.png -crop 700x330+380+2300 +repage -resize 300% crops/D4_DSTAR_strip_400.png`
   - `magick r400/p-14.png -crop 1150x340+380+990 +repage -resize 300% crops/D5_dPMR_strip_400.png`
   - `magick r400/p-14.png -crop 950x340+1780+990 +repage -resize 300% crops/D6_NXDN_strip_400.png`
   - `magick r400/p-15.png -crop 1100x340+380+990 +repage -resize 300% crops/D7_DCR_strip_400.png`

   At these enlargements every solid box rule, every dashed nibble rule, every
   bracket tick and every circled numeral sits clear of its neighbours.
4. **`pdftotext`.** **Not run at any point in this leg**, in any form. No text
   layer was extracted, navigationally or otherwise; navigation was done from
   the 300 dpi renders. The first attestation form below is therefore the true
   one.
5. **`tesseract`.** Present on the machine (`/opt/homebrew/bin/tesseract`) but
   **not used**. Every numeral, bracket and rule was read directly by eye off
   the enlarged renders; nothing was legible only to OCR.
6. **Second independent pass — done.** After the first pass was complete, every
   value was re-read from a different raster. **How the second raster differed:**
   a different resolution (600 dpi rather than 400 dpi, so 4962 × 7016 px per
   page), different crop windows (each of the three D1 rows cropped separately
   rather than the block as one image, and each mode strip cropped with a
   different origin and width), and a different enlargement (150 % / 160 % /
   200 % rather than 200 % / 300 %):
   - `magick r600/p-12.png -crop 1500x310+700+1260 +repage -resize 150% crops/P2_D1_row1_600.png`
   - `magick r600/p-12.png -crop 1620x300+620+1680 +repage -resize 150% crops/P2_D1_row2_600.png`
   - `magick r600/p-12.png -crop 1620x300+620+2050 +repage -resize 150% crops/P2_D1_row3_600.png`
   - `magick r600/p-13.png -crop 1300x270+560+1520 +repage -resize 160% crops/P2_D2_FM_600.png`
   - `magick r600/p-13.png -crop 1050x270+2700+1520 +repage -resize 160% crops/P2_D3_P25_600.png`
   - `magick r600/p-13.png -crop 500x330+620+3480 +repage -resize 200% crops/P2_D4_DSTAR_600.png`
   - `magick r600/p-14.png -crop 1450x270+560+1520 +repage -resize 160% crops/P2_D5_dPMR_600.png`
   - `magick r600/p-14.png -crop 1250x270+2700+1520 +repage -resize 160% crops/P2_D6_NXDN_600.png`
   - `magick r600/p-15.png -crop 1400x270+560+1520 +repage -resize 160% crops/P2_D7_DCR_600.png`

   In the second pass each strip was re-counted cell by cell and each bracket
   tick re-located against the box boundaries beneath it.

   **Cells where the two passes disagreed: none.** Every solid-cell count, every
   ellipsis cell, every bracket span and every index numeral matched. No third
   render was needed to settle anything.

   Auxiliary crops used to confirm printed wording (not positions):
   `AUX_p13_modeheadings_600.png`, `AUX_p14_modeheadings_600.png`,
   `AUX_p15_modeheadings_600.png`, `AUX_p13_dstar_heading_600.png`,
   `AUX_p13_headings_600.png`, `AUX_p13_NAC_600.png`, `AUX_p12_note_600.png`,
   `AUX_p12_item5_600.png`, `AUX_p15_clearformat_600.png`,
   `AUX_p15_scanstart_heading_600.png`, `AUX_p28_foot.png`, `AUX_p01_foot.png`. Every
   quotation of printed wording in `## Observed disagreements` was confirmed by
   eye on one of these at 600 dpi, enlarged.

## Position arithmetic, per diagram

Positions are 1-based within the memory-channel data block. Each circled index
numeral labels exactly one drawn byte cell, and the numbering runs continuously
from ① through the three linked rows of D1 (joined by the wrap-round arrows) and
on into whichever mode tail applies. Byte positions are therefore **read off the
printed numerals**, and the cell count under every bracket was **measured
independently** as a check. Both numbers are given below; where a field's cells
are elided by an ellipsis the two cannot be equal, which is STOP 2.

Notation: `extent` = bytes implied by the printed index range; `cells` = solid
byte cells actually drawn under that field's bracket or numeral.

### D1 — no per-diagram caption is printed. Defined by the section caption above it: `●Memory channel content` / `Command: 1A 00` (PDF page 12)

Drawn as three strip rows linked left-to-right by wrap-round arrows: row 1 ends
in a curve that re-enters row 2 from the left; row 2 ends in a curve that
re-enters row 3; row 3 leaves by a plain right-pointing arrow.

Row 1 — drawn as 6 solid cells, then a dotted ellipsis cell, then 1 solid cell
(7 solid cells in all):

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ①, ② | 1 | 2 | 2 (r1c1–r1c2) | 2 | 3 |
| ③, ④ | 3 | 2 | 2 (r1c3–r1c4) | 4 | 5 |
| ⑤ | 5 | 1 | 1 (r1c5) | 5 | 6 |
| ⑥ ~ ⑩ | 6 | 5 | 2 + ellipsis (r1c6, `•••`, r1c7) | 10 | 11 (row 2) |

Row 1 running total: 2 + 2 + 1 + 5 = **10 bytes**, ending at byte 10; row 1's
last printed index is ⑩. Agrees.

Row 2 — drawn as 4 solid cells, a dotted ellipsis cell, then 3 solid cells
(7 solid cells in all):

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ⑪, ⑫ | 11 | 2 | 2 (r2c1–r2c2) | 12 | 13 |
| ⑬ | 13 | 1 | 1 (r2c3) | 13 | 14 |
| ⑭ ~ ⑰ | 14 | 4 | 2 + ellipsis (r2c4, `•••`, r2c5) | 17 | 18 |
| ⑱, ⑲ | 18 | 2 | 2 (r2c6–r2c7) | 19 | 20 (row 3) |

Row 2 running total: 2 + 1 + 4 + 2 = **9 bytes**, 11 → 19; row 2's last printed
index is ⑲. Agrees. No gap and no overlap at the row 1 / row 2 join: row 1 ends
at 10, row 2 starts at 11.

Row 3 — drawn as 7 solid cells, a dotted ellipsis cell, then 1 solid cell
(8 solid cells in all):

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ⑳, ㉑ | 20 | 2 | 2 (r3c1–r3c2) | 21 | 22 |
| ㉒ | 22 | 1 | 1 (r3c3) | 22 | 23 |
| ㉓ | 23 | 1 | 1 (r3c4) | 23 | 24 |
| ㉔ | 24 | 1 | 1 (r3c5) | 24 | 25 |
| ㉕ | 25 | 1 | 1 (r3c6) | 25 | 26 |
| ㉖ ~ ㊶ | 26 | 16 | 2 + ellipsis (r3c7, `•••`, r3c8) | 41 | 42 (mode tail) |

Row 3 running total: 2 + 1 + 1 + 1 + 1 + 16 = **22 bytes**, 20 → 41; row 3's
last printed index is ㊶. Agrees. No gap and no overlap at the row 2 / row 3
join: row 2 ends at 19, row 3 starts at 20.

D1 whole-block running total: 10 + 9 + 22 = **41 bytes**, bytes 1 – 41, indices
① – ㊶. The block's own printed numbering ends at ㊶ and the note printed
directly beneath the strip refers to "㊷ and or later", so the next byte after
D1 is 42 in every mode tail.

Solid cells actually drawn across D1: 7 + 7 + 8 = **22**, against 41 bytes.
The shortfall of 19 is entirely accounted for by the three ellipsis cells
(3 + 2 + 14 elided bytes = 19). This is STOP 2.

### D2 — `For receiving an FM signal` (PDF page 13, left column)

Entered by a solid arrowhead at the left. 7 solid cells, no ellipsis.

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ㊷ | 42 | 1 | 1 (cell 1) | 42 | 43 |
| ㊸ ~ ㊺ | 43 | 3 | 3 (cells 2–4) | 45 | 46 |
| ㊻ ~ ㊽ | 46 | 3 | 3 (cells 5–7) | 48 | — (strip ends) |

Running total 1 + 3 + 3 = **7 bytes**, 42 → 48, and **7 solid cells** are drawn.
Extent and drawing agree exactly. Diagram-local cell ordinals 1 – 7 correspond
to printed indices ㊷ – ㊽; both are recorded, neither reconciled (STOP 1).

### D3 — `For receiving a P25 signal` (PDF page 13, right column)

Entered by a solid arrowhead at the left. 4 solid cells, no ellipsis.

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ㊷ | 42 | 1 | 1 (cell 1) | 42 | 43 |
| ㊸ ~ ㊺ | 43 | 3 | 3 (cells 2–4) | 45 | — (strip ends) |

Running total 1 + 3 = **4 bytes**, 42 → 45, and **4 solid cells** are drawn.
Agree exactly.

### D4 — `For receiving a D-STAR signal` (PDF page 13, left column, lower)

Entered by a solid arrowhead at the left. 2 solid cells, no ellipsis.

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ㊷ | 42 | 1 | 1 (cell 1) | 42 | 43 |
| ㊸ | 43 | 1 | 1 (cell 2) | 43 | — (strip ends) |

Running total 1 + 1 = **2 bytes**, 42 → 43, and **2 solid cells** are drawn.
Agree exactly. This is the shortest tail in the document.

### D5 — `For receiving a dPMR signal` (PDF page 14, left column)

Entered by a solid arrowhead at the left. 8 solid cells, no ellipsis.

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ㊷ | 42 | 1 | 1 (cell 1) | 42 | 43 |
| ㊸, ㊹ | 43 | 2 | 2 (cells 2–3) | 44 | 45 |
| ㊺ | 45 | 1 | 1 (cell 4) | 45 | 46 |
| ㊻ | 46 | 1 | 1 (cell 5) | 46 | 47 |
| ㊼ ~ ㊾ | 47 | 3 | 3 (cells 6–8) | 49 | — (strip ends) |

Running total 1 + 2 + 1 + 1 + 3 = **8 bytes**, 42 → 49, and **8 solid cells**
are drawn. Agree exactly. This is the longest tail in the document.

### D6 — `For receiving an NXDN signal` (PDF page 14, right column)

Entered by a solid arrowhead at the left. 6 solid cells, no ellipsis.

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ㊷ | 42 | 1 | 1 (cell 1) | 42 | 43 |
| ㊸ | 43 | 1 | 1 (cell 2) | 43 | 44 |
| ㊹ | 44 | 1 | 1 (cell 3) | 44 | 45 |
| ㊺ ~ ㊼ | 45 | 3 | 3 (cells 4–6) | 47 | — (strip ends) |

Running total 1 + 1 + 1 + 3 = **6 bytes**, 42 → 47, and **6 solid cells** are
drawn. Agree exactly. Note that ㊸ and ㊹ are drawn here as two separate
free-standing numerals, where D5 and D7 bracket the same two indices as a
comma-separated pair `㊸, ㊹`; the cell count is 2 either way.

### D7 — `For receiving a DCR signal` (PDF page 15, left column)

Entered by a solid arrowhead at the left. 7 solid cells, no ellipsis.

| field | starts | extent | cells drawn | ends | next starts |
|---|---|---|---|---|---|
| ㊷ | 42 | 1 | 1 (cell 1) | 42 | 43 |
| ㊸, ㊹ | 43 | 2 | 2 (cells 2–3) | 44 | 45 |
| ㊺ | 45 | 1 | 1 (cell 4) | 45 | 46 |
| ㊻ ~ ㊽ | 46 | 3 | 3 (cells 5–7) | 48 | — (strip ends) |

Running total 1 + 2 + 1 + 3 = **7 bytes**, 42 → 48, and **7 solid cells** are
drawn. Agree exactly.

### The six tails measured against one another

Each tail was measured separately, on its own render, without carrying any
figure across from another. The results, recorded and not reconciled:

| diagram | mode block heading | first byte | last byte | length |
|---|---|---|---|---|
| D2 | For receiving an FM signal | 42 | 48 | 7 |
| D3 | For receiving a P25 signal | 42 | 45 | 4 |
| D4 | For receiving a D-STAR signal | 42 | 43 | 2 |
| D5 | For receiving a dPMR signal | 42 | 49 | 8 |
| D6 | For receiving an NXDN signal | 42 | 47 | 6 |
| D7 | For receiving a DCR signal | 42 | 48 | 7 |

Every tail begins at printed index ㊷ and at its own first drawn cell. D2 and D7
are the same drawn length (7 cells) but are **not** the same block: D2 divides
its 7 cells as 1 + 3 + 3 and D7 as 1 + 2 + 1 + 3. Both were counted
independently; the coincidence of length is recorded, not assumed.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every
  index numeral in every in-scope diagram (D1 – D7), and in the field-detail
  boxes beside them, is drawn in one single style: a thin black outlined circle
  enclosing black digits on white, one or two digits, no fill and no reversal.
  Checked at 400 dpi and again at 600 dpi on ① – ㊶ across D1's three rows and on
  ㊷ – ㊾ across the six tails. No circled numeral is filled, reversed, bracketed
  or drawn plain; no index is printed twice with different styling within a
  diagram.
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.**
  Nothing in any in-scope diagram is drawn rotated: every index numeral, bracket
  label and leader label reads horizontally, left to right, on the renders. All
  positions in the CSV were read from the picture regardless. The text layer's
  extraction order was not examined at all, because `pdftotext` was never run in
  this leg.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In the
  field-detail boxes the stacked leader labels run in the *reverse* of the
  left-to-right order of the cells they point at. Clearest instance: `㊸ ~ ㊺
  NAC` on PDF page 13, right column, where six arrows rise from the six nibbles
  and the labels, read top to bottom, are `Once position: 0 ~ F`, `0 (Fixed)`,
  `10th position: 0 ~ F`, `0 (Fixed)`, `100th posiiton: 0 ~ F`, `0 (Fixed)` —
  i.e. the topmost label belongs to the *rightmost* nibble and the bottom label
  to the leftmost. The same reversal is drawn in `⑳, ㉑ Programmable tuning
  step` (PDF page 12), `㊼ ~ ㊾ Scrambler key` (PDF page 14) and `㊻ ~ ㊽
  Encryption key` (PDF page 15). Each leader was followed by eye from label to
  the cell it lands on. **No recorded value was affected**: the record strips
  D1 – D7 carry no leader labels, only brackets and numerals sitting directly
  over their cells, and the CSV holds only strip positions.
- **(d) A printed index may differ from a field's measured position —
  ENCOUNTERED.** Six blocks of fields repeat one another's index numbering:
  D2 – D7 each begin at printed index ㊷ while each begins at its own first drawn
  cell (diagram-local cell ordinal 1), and printed indices ㊸ – ㊺ likewise recur
  across several of them with different meanings and different bracket
  groupings. For every field the CSV carries the printed index verbatim in
  `field_index`, the byte position read off the printed numerals in
  `first_byte`/`last_byte`, and the independently measured diagram-local cell
  ordinal in `notes`. All three are recorded; none has been reconciled against
  another, and no printed index has been adjusted to fit a measured position.

## STOP findings

1. **Index sequence repeats across the six mode blocks.** PDF pages 13, 14 and
   15. Visual anchor: the first byte cell of each mode-tail strip — the cell
   immediately right of the incoming solid arrowhead under, in turn, `For
   receiving an FM signal` (p13 left), `For receiving a P25 signal` (p13 right),
   `For receiving a D-STAR signal` (p13 left, lower), `For receiving a dPMR
   signal` (p14 left), `For receiving an NXDN signal` (p14 right) and `For
   receiving a DCR signal` (p15 left). What is printed: the circled numeral ㊷
   over that first cell, in all six. Printed index ㊸ likewise recurs in all six;
   ㊺ in five; ㊻ in three; ㊽ in two. Why it stops: this is a repeat in the index
   sequence — the same index numeral is printed six times over six different
   drawn cells on three different pages — and at the same time the printed index
   (42) differs from the diagram-local cell ordinal (1) at which it is drawn.
   Both readings are transcribed and neither is resolved. Marked `STOP 1` on the
   ㊷ row of each of the six mode blocks, which is where the repeat begins and
   is exactly locatable; the full extent of the repeat is the table above.
   (Recording every affected row would have put `STOP 1` on all 20 mode-tail
   rows; that choice of marking is itself a judgement call, recorded here.)
2. **Drawn cell count is smaller than the printed index range in three D1
   fields.** PDF page 12. Visual anchors: (i) the ⑥ ~ ⑩ bracket at the
   right-hand end of the top strip row, (ii) the ⑭ ~ ⑰ bracket in the middle of
   the middle strip row, (iii) the ㉖ ~ ㊶ bracket at the right-hand end of the
   bottom strip row. What is printed in each: two solid byte cells with, between
   them, a cell whose outline is *dotted* rather than solid and which contains
   three centred dots `•••` instead of two `X` glyphs. Why it stops: counting
   solid cells on the picture gives an extent of 2 bytes for each of these
   fields, while the printed index range gives 5, 4 and 16 respectively; the two
   cannot both be right, and the arithmetic for D1 as a whole gives 22 drawn
   solid cells against 41 printed byte indices. The dotted ellipsis cell is
   plainly the diagram's own mark for elided cells, but that is an inference and
   is not resolved here. Transcribed into the CSV as the printed range gives
   them (6–10, 14–17, 26–41), with the drawn cell count stated in `notes` and
   `STOP 2` on each of the three rows.

No value anywhere in scope was unreadable, so no cell contains `UNREADABLE`.

## Observed disagreements

Recorded exactly as printed; not resolved, and none of these stopped a
measurement.

1. **`100th posiiton`** — PDF page 13, right column, the leader labels of the
   `㊸ ~ ㊺ NAC` detail box, fifth label from the top. Printed as
   `100th posiiton: 0 ~ F`. The neighbouring labels in the same box are printed
   `Once position:` and `10th position:`, and the corresponding labels in the
   dPMR, NXDN and DCR detail boxes on PDF pages 14 and 15 are all printed
   `100th position:`. Confirmed by eye at 600 dpi, enlarged, on
   `AUX_p13_NAC_600.png`.
2. **The same two indices are grouped differently in different tails.** `㊸, ㊹`
   is drawn as one comma-joined bracketed pair over two cells in D5 (dPMR, p14)
   and D7 (DCR, p15), but as two separate free-standing numerals `㊸` and `㊹`
   over their own cells in D6 (NXDN, p14). The cell count is 2 in all three.
3. **㊷ carries a different meaning in each tail** while occupying the same byte
   position: the field-detail box beneath each strip heads it `Tone squelch
   type` under FM (p13) and `Digital squelch (D.SQL) type` under P25, D-STAR,
   dPMR, NXDN and DCR (pp13–15). Position is identical; the printed name is not.
4. **The note beneath D1** (PDF page 12) reads, with a leading information
   symbol: `In the modes other than FM and Digital, ㊷ and or later is not used.
   In the FM and Digital modes, entering ㊷ and or later can be omitted. The
   default value is applied to the omitted items.` The phrase `and or later` is
   printed twice, as shown. No mode-tail strip is drawn for the non-FM,
   non-Digital modes anywhere on PDF pages 12 – 15.
5. **The clear-channel format on PDF page 15** re-uses D1's own indices with
   different content: `①, ② :  0000 ~ 0101 group` / `You cannot specify group
   "0102" (Program scan edge)`, `③, ④ :  Memory channel number`, `⑤ :  "FF"`,
   `⑥ ~ :  None`.
   It is prose, not a data-block diagram, and no CSV row comes from it; but it
   assigns ⑤ the value `"FF"` where the ⑤ detail box on PDF page 12 lists only
   `0=SKIP OFF / 1=SKIP / 2=PSKIP` for the first nibble and `0 =OFF /
   1 ~ 9=★1 ~ ★9` for the second.
6. **The adjacent section restarts at ①.** The right-hand column of PDF page 15,
   under `●Programmable scan start (remote) data` / `Command: 1A 0B 00`, draws a
   three-row strip whose first bracket is labelled `① ~ ⑤`. Its diagrams are
   drawn in the same style as the in-scope ones and its `For receiving a …
   signal` headings on the following page resemble the in-scope headings
   closely. Excluded from the CSV as a different command's record; noted here
   because the resemblance is the obvious way to misread these pages.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.

*(Clarification, offered so the sentence above is not read as claiming more than
is true: `ls` and `magick identify` were run on this leg's own output directory
`…/legs-out/icr8600/W`, which this leg created empty, in order to confirm that
the renders had been written and to read their pixel dimensions. No directory
outside that one was listed, and no file outside the PDF and this leg's own
renders and outputs was opened.)*
