# IC-9700 CI-V — memory-record data-block geometry witness

## Source

- Document title, as printed on the cover (PDF page 1): `CI-V REFERENCE GUIDE`, above `VHF/UHF ALL MODE TRANSCEIVER` / `IC-9700` / `Icom Inc.`
- Revision code, as printed: `A7508-3EX-4`. It is printed at the foot of the left-hand side of the last page (PDF page 28), directly above `© 2019–2023 Icom Inc.    Mar. 2023`. No revision code is printed on the cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic9700_civ_rev4.pdf`
- Page count: 28 PDF pages (established by rendering the whole file at 12 dpi and counting the 28 resulting PNGs, `t-01` … `t-28`).

## Extent

Rendered: PDF pages 14, 15, 16, 17 at 300 dpi (locating); PDF pages 15 and 16 at 400 dpi and again at 600 dpi (reading); PDF page 1 at 200 dpi and PDF page 28 at 200 dpi then 600 dpi (cover title and revision code only); all 28 pages at 12 dpi (page count only).

| PDF page | Printed folio | Contribution |
|---|---|---|
| 14 | `13` | Located the section boundary only. Prints `◇Command formats` with `• Operating frequency`, `• Operating mode`, `• Band edge frequency settings`, `• Duplex Offset frequency setting`, `• Codes for CW message contents`. No memory-record data block. Nothing on this page was transcribed. |
| 15 | `14` | All of D1, D2, D3, D4. |
| 16 | `15` | D5. |
| 17 | `16` | Located the section boundary only. Prints `◇ Command formats (Continued)` with `• Memory keyer content`, `• IF filter width settings`, … No memory-record data block. Nothing on this page was transcribed. |

Where the transcribed material begins and ends:

- **PDF page 15 (folio 14).** The running head is `Remote control`; below it `◇ Command formats (Continued)`, then the sub-heading `• Memory content` and `Command: 1A 00`. **D1 begins immediately below that `Command: 1A 00` line** — it is the two-row block of cells with the wrap-round arrow. **D1 ends** at the right-hand end of its lower row (the last grey `X X` cell under the `52 ~ 67` bracket); immediately below it, in two columns, begins the numbered legend whose first entry is `① Frequency band setting`.
- Within that legend on the same page sit the three single-byte expansion insets: **D2** below the heading `④ Select memory setting` and above the footnote `* For program scan edge channel, call channel, set to "0."`; **D3** below the heading `⑬ Duplex and Tone settings` and above the note beginning `RPS can be set when DD mode is selected…` at the foot of the left column; **D4** below the heading `⑭ Digital squelch setting` and above `0=Digital squelch function OFF`, near the top of the right column. The page ends with the grey `NOTE:` box and the folio `14`.
- **PDF page 16 (folio 15).** Running head `Remote control`, then `◇ Command formats (Continued)`. The left column prints `• Codes for character entries` and its character/ASCII tables (tables, not data-block diagrams — not transcribed). In the right column, `• Band stacking register` and `Command: 1A 01`. **D5 begins immediately below that `Command: 1A 01` line** and **ends** at the right edge of its second cell; immediately below it begins the grey `NOTE:` box that starts `When sending the contents, the codes, such as operating frequency and operating mode*, should be added after the frequency band code and the register code, as shown below.` The page ends with `• Memory keyer character entries` / `Command: 1A 02` and its table, and the folio `15`.

### The diagrams, defined by their printed captions

Diagram ids run in page order; within page 15, in reading order (main block first, then the left column top-to-bottom, then the right column).

- **D1** — the two-row block of cells printed immediately under the captions `• Memory content` and `Command: 1A 00` (PDF page 15). Its upper row ends in a wrap-round arrow that leads into its lower row; the two rows are one block.
- **D2** — the one-byte box printed under the caption `④ Select memory setting` (PDF page 15). The numeral printed above the box itself is `③`.
- **D3** — the one-byte box printed under the caption `⑬ Duplex and Tone settings` (PDF page 15).
- **D4** — the one-byte box printed under the caption `⑭ Digital squelch setting` (PDF page 15).
- **D5** — the two-byte box printed under the captions `• Band stacking register` and `Command: 1A 01` (PDF page 16).

### What the diagrams do and do not number

**These diagrams do not number their byte positions.** No numeral, ruler, offset or address is printed along, above or below any cell row. The only numerals printed over D1, D2, D3, D4 and D5 are the circled **field indices** (`①`, `②, ③`, `⑤ ~ ⑨`, …), carried on brackets or centred over a cell. Accordingly, **every `first_byte` / `last_byte` in the CSV is a cell ordinal counted by eye from that diagram's own first (leftmost) cell**, 1-based, exactly as the task's fallback directs. For D1 the count runs continuously across the wrap: the lower row's first cell is cell 23, because the printed arrow carries the block from the end of the upper row into the start of the lower row.

**The diagrams do not label nibbles.** Each solid cell is divided into two halves by a printed dotted vertical line, and each half carries one printed character (`X`, or a literal `0`). Nothing names, numbers or letters those halves. `first_nibble` / `last_nibble` therefore use the recording convention given in the task — 1 for the half printed first (leftmost), 2 for the other, and 1 to 2 for a whole cell. The only cell in which no dotted divider is printed is D1's `❺ ~ (51 filled)` region (see STOP 6); the 1 and 2 recorded for it are the whole-cell convention, not a printed division.

**Counting includes the continuation cells.** Several ranges are drawn as `solid cell — dotted box containing "……" — solid cell`. The dotted `……` box is a drawn cell of the picture and is counted as one cell, because that is what is on the page; it plainly stands for an unstated number of omitted bytes, which is why every such range is a STOP.

## Method

1. **Locate — 300 dpi.** `pdftoppm -png -r 300 -f 14 -l 17 <pdf> r300/p` into the fresh directory `…/evidence/ic9700-W/r300/`. The four page images were read as images to find the section whose printed heading matches (`• Memory content`, `Command: 1A 00`) and to confirm that pages 14 and 17 carry no memory-record data block.
2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f 15 -l 16 <pdf> r400/q` (3308 × 4678 px per page). First-pass values were read from these.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`) and was used throughout. First-pass crops (into `crops/`):
   - `magick r400/q-15.png -crop 2700x520+280+770 +repage -resize 150% crops/d1_full.png` (whole of D1)
   - `magick r400/q-15.png -crop 1000x260+280+780 +repage -resize 300% crops/d1_r1a.png`, `… -crop 1000x260+1230+780 … d1_r1b.png`, `… -crop 900x260+2100+780 … d1_r1c.png` (D1 upper row in thirds, with the index band)
   - `magick r400/q-15.png -crop 1000x300+400+1000 +repage -resize 300% crops/d1_r2a.png`, `… +1330+1000 … d1_r2b.png`, `… +2200+1000 … d1_r2c.png` (D1 lower row in thirds)
   - `magick r400/q-15.png -crop 900x200+1750+1090 +repage -resize 350% crops/d1_black5.png` (the filled-circle region)
   - `magick r400/q-15.png -crop 700x520+220+2130 +repage -resize 250% crops/ins4.png`, `… -crop 750x560+200+3500 … ins13.png`, `… -crop 800x520+1600+1330 … ins14.png` (D2, D3, D4)
   - `magick r400/q-16.png -crop 900x400+1950+680 +repage -resize 300% crops/d5_bsr.png` (D5)
   - `magick r400/q-15.png -crop 1500x900+1700+2680 +repage -resize 150% crops/right_prose_a.png` and `… +1700+3500 … right_prose_b.png` (the legend's printed widths and the `NOTE:` box)
   At these enlargements every numeral, rule, dotted divider and cell border stands clear of its neighbours.
4. **`pdftotext`.** **It was never run**, on this or any file. Nothing in this leg came from a text layer.
5. **`tesseract`.** Available at `/opt/homebrew/bin/tesseract` but **not used**. Every numeral was read by eye off the enlarged renders.
6. **Second independent pass — done.** After the first pass was complete, pages 15 and 16 were re-rendered at **600 dpi** (`pdftoppm -png -r 600 -f 15 -l 16 <pdf> r600/s`, 4961 × 7016 px) — a different raster from the 400 dpi one — and re-read through **different crop windows**: D1's upper row cut into **four** quarters instead of three (`magick r600/s-15.png -crop 1150x400+{400,1450,2500,3550}+1080 +repage -resize 180%`), D1's lower row likewise cut into four (`-crop 950x420+{600,1470,2340,3210}+1450 +repage -resize 200%`), and fresh windows at fresh enlargements for D2 (`-crop 900x700+300+3120 -resize 180%`), D3 (`-crop 950x700+280+5230 -resize 180%`), D4 (`-crop 1000x650+2350+2050 -resize 180%`) and D5 (`magick r600/s-16.png -crop 1100x500+2900+1030 -resize 200%`), into `crops2/`. Every quarter boundary of the second pass falls inside a cell that the first pass's thirds had shown whole, so no cell boundary was inherited from the first pass's framing.
   **Disagreements between the two passes: none.** Both passes counted 22 drawn cells in D1's upper row and 16 in its lower row (38 in total), placed every bracket end on the same cell boundary, read the same index numerals, and read the same literal characters (`0 X` in D1 cell 4, `X 0` in D1 cell 12, `0 X` in D2, `X X` in D3, `X 0` in D4). Both passes read `③` above D2's box under the heading `④`, and both read the filled black `❺ ~ 51` pair in D1's lower row. No third render was needed.

## Position arithmetic, per diagram

Positions are drawn-cell ordinals counted from each diagram's own first cell (see *Extent*). "Extent" is the number of drawn cells the printed bracket or numeral covers; an ellipsis cell (a dotted box printed `……`) is counted as one drawn cell and is flagged.

### D1 — `• Memory content`, `Command: 1A 00` (PDF page 15)

Upper row, left to right:

| # | Printed index | Starts at cell | Measured extent | Ends at cell | Next starts at |
|---|---|---|---|---|---|
| 1 | `①` | 1 | 1 cell | 1 | 2 |
| 2 | `②, ③` | 2 | 2 cells | 3 | 4 |
| 3 | `④` | 4 | 1 cell | 4 | 5 |
| 4 | `⑤ ~ ⑨` | 5 | 3 cells (cell 6 is an `……` box) | 7 | 8 |
| 5 | `⑩, ⑪` | 8 | 2 cells | 9 | 10 |
| 6 | `⑫` | 10 | 1 cell | 10 | 11 |
| 7 | `⑬` | 11 | 1 cell | 11 | 12 |
| 8 | `⑭` | 12 | 1 cell | 12 | 13 |
| 9 | `⑮ ~ ⑰` | 13 | 3 cells | 15 | 16 |
| 10 | `⑱ ~ ⑳` | 16 | 3 cells | 18 | 19 |
| 11 | `㉑ ~ ㉓` | 19 | 3 cells | 21 | 22 |
| 12 | `㉔` | 22 | 1 cell | 22 | 23 (via the wrap arrow, first cell of the lower row) |

Running total, upper row: 1 + 2 + 1 + 3 + 2 + 1 + 1 + 1 + 3 + 3 + 3 + 1 = **22 drawn cells**.

Lower row, left to right, continuing the same count:

| # | Printed index | Starts at cell | Measured extent | Ends at cell | Next starts at |
|---|---|---|---|---|---|
| 13 | `㉕ ~ ㉗` | 23 | 3 cells | 25 | 26 |
| 14 | `㉘ ~ ㉟` | 26 | 3 cells (cell 27 is an `……` box) | 28 | 29 |
| 15 | `㊱ ~ ㊸` | 29 | 3 cells (cell 30 is an `……` box) | 31 | 32 |
| 16 | `㊹ ~ (51)` | 32 | 3 cells (cell 33 is an `……` box) | 34 | 35 |
| 17 | `❺ ~ (51 filled)` | 35 | 1 region (one wide dotted box of dots; no cell divisions, no nibble divider) | 35 | 36 |
| 18 | `(52) ~ (67)` | 36 | 3 cells (cell 37 is an `……` box) | 38 | — end of the diagram |

Running total, lower row: 3 + 3 + 3 + 3 + 1 + 3 = **16 drawn cells**. Whole diagram: 22 + 16 = **38 drawn cells**, no gaps and no overlaps — every cell of the picture is claimed by exactly one printed bracket or numeral, and each bracket's right end lands on the same cell boundary at which the next bracket's left end begins.

**Where the running total and the printed numbering disagree.** The printed indices run `1 … 67` with a further printed block `5 … 51` in filled circles. Read as one index per byte, that is 67 + 47 = 114 positions; the picture draws 38 cells. Field by field, the first printed index against the measured start cell:

| Printed first index | 1 | 2 | 4 | 5 | 10 | 12 | 13 | 14 | 15 | 18 | 21 | 24 | 25 | 28 | 36 | 44 | 5 (filled) | 52 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Measured start cell | 1 | 2 | 4 | 5 | 8 | 10 | 11 | 12 | 13 | 16 | 19 | 22 | 23 | 26 | 29 | 32 | 35 | 36 |

They agree through `⑤` and diverge from `⑩` onward, the divergence widening at each `……` box that has been passed: −2 at `⑩` (one `……` box passed) and still −2 at `⑫`, `⑬`, `⑭`, `⑮`, `⑱`, `㉑`, `㉔`, `㉕` and `㉘`; −7 at `㊱` (a second `……` box passed); −12 at `㊹` (a third); −16 at `(52)` (a fourth `……` box and the one-region filled block passed). Both figures are recorded; neither is resolved (STOP 2, and STOPs 1, 3, 4, 5, 7 for the individual ranges).

**The repeated block, both occurrences measured separately (hazard (d)).** The page's `NOTE:` prints `The same data as ⑤ ~ (51) are stored in ❺ ~ (51 filled)` — i.e. the outlined range `⑤ ~ (51)` and the filled range `❺ ~ (51 filled)` are stated to hold the same data. Measured on the picture, and not reconciled:
- the outlined occurrence `⑤ … (51)` runs from cell **5** (first cell under the `5 ~ 9` bracket) to cell **34** (last cell under the `44 ~ 51` bracket) = **30 drawn cells**, of which 4 are `……` boxes;
- the filled occurrence `❺ ~ (51 filled)` occupies cell **35** alone = **1 drawn region**, containing no solid cells at all.

Thirty drawn cells against one; 47 printed indices in each. Recorded as measured, twice, and left unreconciled (STOP 6).

### D2 — `④ Select memory setting` (PDF page 15)

| Printed index | Starts at cell | Measured extent | Ends at cell | Next starts at |
|---|---|---|---|---|
| `③` (the numeral printed above the box) | 1 | 1 cell (2 nibbles: `0` then `X`) | 1 | — end of the diagram |

Running total: 1 drawn cell, 2 printed nibble halves. The diagram consists of that one box and nothing else, so counting from its own first cell puts it at byte 1. The caption above it prints `④`; the numeral over the box prints `③` (STOP 8). Both are recorded as printed; neither is adjusted to the other.

### D3 — `⑬ Duplex and Tone settings` (PDF page 15)

| Printed index | Starts at cell | Measured extent | Ends at cell | Next starts at |
|---|---|---|---|---|
| `⑬` | 1 | 1 cell (2 nibbles: `X` then `X`) | 1 | — end of the diagram |

Running total: 1 drawn cell. Caption numeral and box numeral both print `13`.

### D4 — `⑭ Digital squelch setting` (PDF page 15)

| Printed index | Starts at cell | Measured extent | Ends at cell | Next starts at |
|---|---|---|---|---|
| `⑭` | 1 | 1 cell (2 nibbles: `X` then `0`) | 1 | — end of the diagram |

Running total: 1 drawn cell. Caption numeral and box numeral both print `14`.

### D5 — `• Band stacking register`, `Command: 1A 01` (PDF page 16)

| Printed index | Starts at cell | Measured extent | Ends at cell | Next starts at |
|---|---|---|---|---|
| `①` | 1 | 1 cell | 1 | 2 |
| `②` | 2 | 1 cell | 2 | — end of the diagram |

Running total: 1 + 1 = **2 drawn cells**, 4 printed halves, no gap and no overlap: the outer box is divided by one solid vertical rule into two cells, each halved by a dotted rule. The printed indices `1` and `2` and the measured start cells `1` and `2` agree throughout.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — ENCOUNTERED.** D1 draws its indices in two styles: outlined circles (white disc, black rule, black digits) for `①` through `(67)`, and **filled/reversed circles** (solid black disc, white digits) for the pair `❺ ~ (51 filled)` between the `44 ~ 51` and `52 ~ 67` brackets in the lower row. The two styles are recorded as drawn and are not normalised; no meaning is inferred for either. The same two styles recur in the page's `NOTE:` box. D2–D5 use outlined circles only.
- **(b) Diagrams may be vector groups with rotated labels — CANNOT DETERMINE.** No text layer was consulted (`pdftotext` was never run), so nothing can be said about extraction order; every position here was read from the picture, which is the point. No rotated label appears in any of D1–D5 — all index numerals and all cell characters are printed upright.
- **(c) Leader-line label order may be reversed — NOT ENCOUNTERED.** D1's index brackets sit directly over the cells they span, each with a plain right-angle tick at its left end and a `V` at its right end landing exactly on a cell boundary; each was followed by eye. In D2, D3 and D4 the labels sit below the box with arrows pointing up into a specific nibble, and each arrow was followed from label to arrowhead: D2 `Fixed` → left half, enum list → right half; D3 duplex list → left half, tone list → right half; D4 enum list → left half, `Fixed` → right half. In every case the label's position matches the half its arrow lands on; no reversal.
- **(d) A printed index may differ from a field's measured position — ENCOUNTERED.** Two separate instances. (i) Throughout D1 the printed index numbering and the measured cell ordinal diverge from `⑩` onward, because `……` continuation boxes stand in for omitted cells (table above; STOP 2). (ii) D1 prints a repeat block: the filled `❺ ~ (51 filled)` region, which the `NOTE:` says carries the same data as the outlined `⑤ ~ (51)`. Both occurrences were measured separately — 30 drawn cells for the outlined one, 1 drawn region for the filled one — and are recorded as measured, without adjusting either printed index to fit, and without reinterpreting either measurement in the light of the other (STOP 6).

## STOP findings

1. **PDF page 15 — D1, upper row, cells 5 to 7, under the bracket printed `⑤ ~ ⑨`.** What is printed: a grey `X X` cell, then a grey dotted box containing `……`, then a grey `X X` cell; the bracket over them prints five indices, `5` to `9`. Why it stops: the measured extent (3 drawn cells) cannot be reconciled with the printed index count (5) without inventing a count for the `……` box, which prints no cell divisions of its own. Recorded in the CSV row `D1,⑤ ~ ⑨` as measured, cells 5 to 7.
2. **PDF page 15 — D1, from the pair of white cells under the bracket printed `⑩, ⑪` (upper row, cells 8 and 9) to the end of the diagram.** What is printed: index numbering that continues `10, 11, 12, …, 67` while the cell count, carried forward from the diagram's own first cell, has fallen behind at every `……` box. Why it stops: the running total and the printed numbering disagree — `⑩` sits on cell 8, `㉘` on cell 26, `㊹` on cell 32, `(52)` on cell 36 (full table above). Both are recorded, neither resolved. Tagged `STOP 2` in every affected CSV row, from `D1,"⑩, ⑪"` to `D1,(52) ~ (67)`.
3. **PDF page 15 — D1, lower row, cells 26 to 28, under the bracket printed `㉘ ~ ㉟`.** What is printed: grey `X X`, a grey `……` box, grey `X X` — three drawn cells for eight printed indices; and in the legend on the same page, `㉘ ~ ㉟ UR (Destination) call sign setting (8 characters; fixed)`. Why it stops: the measured extent (3 cells) disagrees with the printed width (`8 characters; fixed`) and with the printed index count (8).
4. **PDF page 15 — D1, lower row, cells 29 to 31, under the bracket printed `㊱ ~ ㊸`.** What is printed: white `X X`, a white `……` box, white `X X`; and in the legend, `㊱ ~ ㊸ R1 (Access repeater) call sign setting (8 characters; fixed)`. Why it stops: measured extent 3 cells against a printed width of 8 characters and 8 printed indices.
5. **PDF page 15 — D1, lower row, cells 32 to 34, under the bracket printed `㊹ ~ 51`.** What is printed: grey `X X`, a grey `……` box, grey `X X`; and in the legend, `㊹ ~ (51) R2 (Gateway/Link repeater) call sign setting (8 characters; fixed)`. Why it stops: measured extent 3 cells against a printed width of 8 characters and 8 printed indices.
6. **PDF page 15 — D1, lower row, cell 35: the wide box with a dotted outline holding one long row of dots, lying between the grey cell that ends the `44 ~ 51` bracket and the first grey cell under the `52 ~ 67` bracket; the bracket above it prints two solid black discs bearing white digits, `5` and `51`, separated by a tilde.** Why it stops, on four counts: (i) the index sequence is discontinuous — `5` and `51` have already been printed earlier in this same diagram, in outlined circles, so both numerals appear twice in one diagram in two different styles; (ii) the numerals are drawn in a different style from every other index in the diagram; (iii) the region is drawn with no cell divisions and no nibble divider at all, so its extent in bytes is not counted anywhere on the picture — one drawn region stands where the `NOTE:` says 47 indices' worth of data is stored; (iv) the same page's `NOTE:` prints `The same data as ⑤ ~ (51) are stored in ❺ ~ (51 filled)`, a repeat of a block measured elsewhere in this diagram as 30 drawn cells. Recorded as measured — cell 35 to cell 35 — with the 1/2 nibbles being the whole-cell recording convention, since no divider is printed. Nothing is reconciled.
7. **PDF page 15 — D1, lower row, cells 36 to 38, under the bracket printed `52 ~ 67`.** What is printed: grey `X X`, a grey `……` box, grey `X X` — three drawn cells for sixteen printed indices; and in the legend, `(52) ~ (67) Memory name setting (16 characters; fixed)`, i.e. `52 ~ 67`. Why it stops: measured extent 3 cells against a printed width of 16 characters and 16 printed indices.
8. **PDF page 15 — D2, the one-byte box in the left column.** What is printed: the caption immediately above the inset reads `④ Select memory setting`, with an outlined circled `4`; the numeral centred directly above the box itself is an outlined circled `3`. Why it stops: two printed things contradict each other about which field this inset expands, and the same index `③` is thereby printed twice on the page in the same style for two different fields (the `②, ③` bracket in D1 and this box). Recorded in the CSV as `③`, exactly as printed above the box, with the caption's `④` noted; neither is adjusted.

### A note on notation (a convention of this record, not of the page)

Unicode encodes outlined circled numerals only to 50 and filled ones only to 20. Every index printed in D1–D5 within those ranges is written here with the actual glyph (`①`, `❺`, `㊿`, …). For the four printed glyphs beyond them the CSV writes `(51)` for the outlined circled 51, `(51 filled)` for the filled circled 51, and `(52)` and `(67)` for the outlined circled 52 and 67. The parentheses are this record's fallback; they are not printed on the page. Each affected CSV row says so in its `notes`, and every row's `notes` states the style — outlined or filled/reversed — in which its index is drawn.

## Observed disagreements

Recorded as printed; not resolved, and none of them stopped a measurement.

- The `……` continuation boxes are not shaded consistently within one diagram. In D1's `⑤ ~ ⑨` and `㉘ ~ ㉟` and `㊹ ~ 51` and `52 ~ 67` groups the `……` box is grey like its neighbours; in the `㊱ ~ ㊸` group it is white, as are that group's two solid cells. The `❺ ~ (51 filled)` region is white.
- Cell shading in D1 alternates by group (white, grey, grey, white, grey, …) but the grouping it marks is not stated anywhere on the page, and it does not track the brackets one-for-one: `②` and `③`, two indices under one bracket, are both grey, while `⑮ ~ ⑰`, three indices under one bracket, are all white, and `⑩` and `⑪`, under one bracket, are both white where the neighbouring single index `⑫` is grey.
- D1 cell 4 prints `0 X` and D1 cell 12 prints `X 0`, i.e. a literal `0` in one nibble, where every other solid cell in D1 prints `X X`. The insets confirm the same literals (`0 X` in D2, `X 0` in D4) and label those halves `Fixed`.
- The legend entry for D2 is headed `④ Select memory setting` and its box is labelled `③` (STOP 8), but the same page's clearing instructions print `④ : "FF," ⑤ ~ :None`, using `④` for the same field — so the page uses `④` twice and `③` once for what the layout presents as one field.
- The page's `NOTE:` box refers to the filled block twice as `❺ ~ (51 filled)` and to the outlined block twice as `⑤ ~ (51)`, but the diagram itself never draws an outlined bracket spanning `⑤ ~ (51)` as one range; on the picture that span is divided into the thirteen brackets `⑤ ~ ⑨`, `⑩, ⑪`, `⑫`, `⑬`, `⑭`, `⑮ ~ ⑰`, `⑱ ~ ⑳`, `㉑ ~ ㉓`, `㉔`, `㉕ ~ ㉗`, `㉘ ~ ㉟`, `㊱ ~ ㊸`, `㊹ ~ (51)`.
- D5's `NOTE:` prints `* See ⑤ to (51) on 'Memory content setting.' (p. 14)`, whereas the caption of D1 on that page (folio 14) reads `• Memory content`, not `Memory content setting`.

## Attestation

"Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed."

*Clarification, offered so the sentence above is not read wider than it is true: `pdftotext` was never run, and the only files opened were the target PDF itself and the renders and crops this leg made from it. The only directory listings performed were `ls` of the render directories this leg itself created beneath `…/evidence/ic9700-W/` (`r300/`, `r400/`, `r600/`, `crops/`, `crops2/`, `pagecount/`), to confirm that `pdftoppm` had produced the images and, in the case of `pagecount/`, to count the 28 pages. No repository directory, and no directory containing any other document, was listed, searched or browsed.*
