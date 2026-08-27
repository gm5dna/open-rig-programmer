# IC-7300 — memory-record data-block geometry witness

## Source

- Document title as printed on the cover (PDF page 1): `IC-7300`, above it `HF/50 MHz TRANSCEIVER`, and in the black band `FULL MANUAL`. The publisher line at the foot of the cover reads `Icom Inc.`
- Revision code as printed: `A7292-4EX-12b`, printed at the foot of the left-hand column of the back cover (PDF page 180), directly above `© 2016–2024 Icom Inc.     Aug. 2024`.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300_fullmanual_ENG_12b.pdf`
- Page count: 180 PDF pages (A4, 595.276 × 841.89 pt).

## Extent

Rendered at 300 dpi: PDF pages 167, 168, 169, 170, 171 (locating pass).
Rendered at 400 dpi: PDF page 169 (first reading pass).
Rendered at 500 dpi: PDF page 169 (second reading pass), and PDF page 180 (revision code only).
Rendered at 150 dpi: PDF pages 1 and 180 (cover and back cover, for `## Source` only).

Read as images, and what each contributed:

| PDF page | Printed folio | Contribution |
|---|---|---|
| 1 | (none printed) | Cover title, model, `FULL MANUAL`. |
| 167 | 19-9 | Rendered in the locating pass; not read for any value. |
| 168 | 19-10 | Read only to establish the section boundary immediately before the transcribed material. Carries `• Band stacking register` (1A 01), `• Offset frequency settings` (1A 05 …), `• Codes for character entries`, `• Data mode with filter width settings` (1A 06). None of these is a memory-record data block; no value was taken from this page. |
| 169 | 19-11 | The page transcribed. Everything in the CSV was read from this page. |
| 170 | 19-12 | Read only to establish the section boundary immediately after. Carries `• Memory keyer character entries` (1A 02), `• Memory keyer content` (1A 02), `• [VOX/BK-IN] setting`, `• [AUTOTUNE] setting`, `• [▲]/[▼] setting`, `• MIC Key Customize setting`. No value was taken from this page. |
| 171 | 19-13 | Rendered in the locating pass; not read for any value. |
| 180 | (none printed) | Revision code and copyright line. |

Section heading matched: PDF page 169 carries the running head `19  CONTROL COMMAND`, then the reversed black band `Remote control (CI-V) information`, then the sub-heading `• Memory content` with `Command : 1A 00` beneath it. The identical black band `Remote control (CI-V) information` also heads PDF pages 168 and 170, so the band alone does not identify the section; the sub-heading `• Memory content` does.

Where the transcribed material begins and ends on PDF page 169:

- Immediately before D1: the two lines `• Memory content` and `Command : 1A 00`, set full width across the top of the two-column body.
- D1 is the single horizontal byte strip beneath those two lines, spanning both columns.
- Immediately after D1: the left column resumes with `①, ② Memory channel numbers` / `00 01–00 99: Memory channel 01 to 99` …, and the right column with `⑪ Data mode and tone type settings`.
- D2 sits in the left column beneath `③ Split and Select memory setting`, and is followed by `ⓘSet both 0 for P1 and P2.`
- D3 sits in the right column beneath `⑪ Data mode and tone type settings`, and is followed by `⑫~⑭ Repeater tone frequency setting`.
- Nothing after `⑱~㉗ Memory name settings` / the `NOTE:` box on that page is a data-block diagram; the lower half of PDF page 169 is blank.

Diagrams, defined by their printed captions verbatim:

- **D1** — caption printed above it: `• Memory content` and, on the next line, `Command : 1A 00`.
- **D2** — caption printed above it: `③ Split and Select memory setting`.
- **D3** — caption printed above it: `⑪ Data mode and tone type settings`.

D2 and D3 are each a single-cell diagram that decomposes one byte into its two nibbles. They are included because each carries a printed index numeral of its own and each is the only place on the page where a nibble division is given a meaning. Their byte positions are counted within their own diagram, per the CSV convention that the data block's first byte is 1; they are *not* re-expressed as positions inside D1.

## Method

- **Locate.** `pdftoppm -png -r 300 -f 167 -l 171 <pdf> <out>/p` into a fresh directory `evidence/ic7300-W/r300/`. The five renders were read as images to find the section and the diagrams.
- **Read, first pass.** `pdftoppm -png -r 400 -f 169 -l 169 <pdf> r400/p` (3308 × 4678 px). All first-pass values were read from crops of `r400/p-169.png`.
- **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`) and was used throughout. First-pass commands:
  - `magick r400/p-169.png -crop 2600x260+400+880 +repage -resize 200% crops/d1_full.png` (whole strip, orientation)
  - `magick r400/p-169.png -crop 900x260+400+880 +repage -resize 400% crops/d1_a.png`
  - `magick r400/p-169.png -crop 900x260+1250+880 +repage -resize 400% crops/d1_b.png`
  - `magick r400/p-169.png -crop 900x260+2100+880 +repage -resize 400% crops/d1_c.png`
  - `magick r400/p-169.png -crop 420x200+2090+960 +repage -resize 700% crops/d1_bigregion.png` (the long dashed-edged region under ❹–⓱)
  - `magick r400/p-169.png -crop 1200x700+250+1500 +repage -resize 250% crops/d2.png`
  - `magick r400/p-169.png -crop 1250x480+1700+1180 +repage -resize 250% crops/d3.png`
  At 400 % every numeral, rule, dotted nibble divider and bracket tick sits clear of its neighbours.
- **Pixel-position aid.** To record the strip's geometry numerically for the arithmetic section below, dark-pixel column profiles of `r400/p-169.png` were taken across the strip's interior (rows 1002–1076, the band between the strip's top rule at y = 999–1001 and its bottom rule at y = 1077–1079). This located the solid vertical cell rules and the dotted nibble rules. It is a measurement *of the render*, not a text extraction, and every boundary it reported was independently confirmed by eye on the enlarged crops before being used. No cell count in the CSV rests on it alone.
- **Read, second pass.** See the second-pass record below.
- **tesseract** was available (`/opt/homebrew/bin/tesseract`) and was run **once**, on `crops2/labelband.png` (`magick r500/p-169.png -crop 3000x140+560+1130 +repage -resize 200%`, `tesseract … --psm 7`). It returned `PV, @) @ p~ OO V9, OF © — BY Vy 8-0 -\— 8-8 —)\—_ 8 7`, which is unusable: it cannot read circled or reversed-disc numerals. **No value in the CSV came from tesseract.** Every index numeral was read by eye from the enlarged renders.
- **`pdftotext -layout` was NOT run** at any point, in any mode, on this PDF. Navigation was done entirely by reading 300 dpi page renders.
- **How the printed index glyphs are transcribed into `field_index`.** Each printed index is one glyph on the page and is transcribed as one Unicode character wherever one exists, so that the count of glyphs in the cell matches the count of glyphs on the page. Outlined circled numerals → U+2460–U+2473 (`①` … `⑱`) and U+3257 (`㉗`, CIRCLED NUMBER TWENTY SEVEN). The two filled black discs → U+2779 (`❹`, DINGBAT NEGATIVE CIRCLED DIGIT FOUR) and U+24F1 (`⓱`, NEGATIVE CIRCLED NUMBER SEVENTEEN); the page prints the second as a **single** disc containing the white numerals `17`, not as two discs, and U+24F1 is the single character matching that glyph. The short thick horizontal bar the diagram sets between the members of a range is transcribed as U+2013 EN DASH; the comma-and-space in `①, ②` and `⑨, ⑩` is transcribed as printed. No ASCII hyphen-minus is used anywhere in `field_index`.
- Two further disclosures, for completeness. `pdfinfo` was run once on this same PDF; its output supplied only the page count and page size quoted in `## Source`, and it was the source of no byte position, nibble, numeral, index, width, label or enum value. And the only directory listings made were `ls` on `evidence/ic7300-W` itself and on the freshly created output directories beneath it (`r300/`, `cover/`), to confirm that the renders and the two deliverables had been written; no directory outside `evidence/ic7300-W` was listed, searched or browsed.

### Second independent pass — record

Both passes were done. The second pass re-read every value from a different raster:

- **Different dpi**: 500 dpi (`pdftoppm -png -r 500 -f 169 -l 169 <pdf> r500/p`, 4134 × 5847 px) instead of 400 dpi.
- **Different crop windows**: four overlapping windows tiled across the strip at a different enlargement (300 % rather than 200 %/400 %), cut on different boundaries from the first pass so that no cell fell at the same place in the frame:
  - `magick r500/p-169.png -crop 820x300+560+1130 +repage -resize 300% crops2/w560.png`
  - `magick r500/p-169.png -crop 820x300+1300+1130 +repage -resize 300% crops2/w1300.png`
  - `magick r500/p-169.png -crop 820x300+2040+1130 +repage -resize 300% crops2/w2040.png`
  - `magick r500/p-169.png -crop 820x300+2740+1130 +repage -resize 300% crops2/w2740.png`
- Each window includes the bracket band and the strip together, so every bracket tick could be followed to the cell rule it lands on within the same frame.

**Cells where the two passes disagreed: none.** Both passes independently gave the same drawn-cell sequence for D1 (19 drawn regions, in the order and with the fills and borders set out below), the same bracket spans, the same index numerals, and the same styling (outlined circles throughout except the two filled black discs). D2 and D3 were re-read from the 500 dpi raster at a different window and agreed with the first pass in every particular (single box, single dotted mid-point rule, two upward leader arrows, and the leader crossing described under hazard (c)). No third render was needed, and no two-pass disagreement is outstanding.

## Position arithmetic, per diagram

### D1 — `• Memory content` / `Command : 1A 00`

D1 prints **no byte-position numerals** of any kind: there is no numbered band above or below the strip, and no numerals inside the cells (every readable cell prints `X X`). Byte positions below are therefore **counted from the diagram's own first cell**, which is the leftmost drawn cell of the strip. Nibble 1 is the `X` to the left of a cell's dotted rule, nibble 2 the `X` to its right.

The strip is drawn as **19 regions**, boundary to boundary. Measured x-positions are given at 400 dpi on `r400/p-169.png` so that the widths can be checked; the counted position is the ordinal of the drawn region.

| Counted position | x (400 dpi) | Width px | Fill / border | Printed inside | Dotted nibble rule at x |
|---|---|---|---|---|---|
| 1 | 480–590 | 110 | grey, solid | `X X` | 535 |
| 2 | 590–700 | 110 | grey, solid | `X X` | 645 |
| 3 | 700–810 | 110 | white, solid | `X X` | 755 |
| 4 | 810–921 | 111 | grey, solid | `X X` | 866 |
| 5 | 921–1031 | 110 | grey, **dashed** top/bottom | `...` | **none** |
| 6 | 1031–1141 | 110 | grey, solid | `X X` | 1086 |
| 7 | 1141–1251 | 110 | white, solid | `X X` | 1196 |
| 8 | 1251–1362 | 111 | white, solid | `X X` | 1306 |
| 9 | 1362–1472 | 110 | grey, solid | `X X` | 1417 |
| 10 | 1472–1582 | 110 | white, solid | `X X` | 1527 |
| 11 | 1582–1692 | 110 | white, solid | `X X` | 1637 |
| 12 | 1692–1803 | 111 | white, solid | `X X` | 1747 |
| 13 | 1803–1913 | 110 | grey, solid | `X X` | 1858 |
| 14 | 1913–2023 | 110 | grey, solid | `X X` | 1968 |
| 15 | 2023–2133 | 110 | grey, solid | `X X` | 2078 |
| 16 | 2133–2463 | **330** | white, **dashed** top/bottom, no side rules of its own | a single horizontal dotted line | **none** |
| 17 | 2463–2573 | 110 | grey, solid | `X X` | 2518 |
| 18 | 2573–2684 | 111 | grey, **dashed** top/bottom | `...` | **none** |
| 19 | 2684–2794 | 110 | grey, solid | `X X` | 2739 |

Mean cell pitch over positions 1–15: (2133 − 480) / 15 = **110.2 px**. Region 16 is 330 px, i.e. 2.99 pitches, but carries **no vertical divisions at all** — it is one undivided region.

Bracket ticks, read by eye and confirmed at both dpi (x at 400 dpi): left tick 480; right tick 700; left tick 810; V 1141; right tick 1362; left tick 1472; V 1803; V 2133; V 2463; right tick 2794. Every tick lands exactly on a cell rule.

Running position of every field:

1. **`①, ②`** — starts at drawn position **1**, nibble 1 (strip's left edge). Measured extent: **2 drawn cells** (positions 1–2, 480→700, 220 px = 2.0 pitches). Ends at position **2**, nibble 2. Printed range names 2 fields; measured extent is 2 cells. **Agrees.** Next field starts at position 3.
2. **`③`** — starts at position **3**, nibble 1. Measured extent: **1 drawn cell** (700→810, 110 px = 1.0 pitch). Ends at position **3**, nibble 2. Printed index names 1 field; measured extent is 1 cell. **Agrees.** Next field starts at position 4.
3. **`④–⑧`** — starts at position **4**, nibble 1. Measured extent: **3 drawn cells** (positions 4, 5, 6; 810→1141, 331 px = 3.0 pitches), of which the middle one prints `...` and has no nibble rule. Ends at position **6**, nibble 2. Printed range names **5** fields (④ ⑤ ⑥ ⑦ ⑧); measured extent is **3** drawn cells. **DISAGREES — STOP 1.** Both recorded, neither resolved. Next field starts at position 7.
4. **`⑨, ⑩`** — starts at position **7**, nibble 1. Measured extent: **2 drawn cells** (1141→1362, 221 px = 2.0 pitches). Ends at position **8**, nibble 2. Printed range names 2 fields; measured extent is 2 cells; **agrees locally**, but the counted position of both cells lies downstream of the `...` at position 5 — **STOP 2**. Next field starts at position 9.
5. **`⑪`** — starts at position **9**, nibble 1. Measured extent: **1 drawn cell** (1362→1472, 110 px = 1.0 pitch). Ends at position **9**, nibble 2. Agrees locally; downstream of the position-5 ellipsis — **STOP 2**. Next field starts at position 10.
6. **`⑫–⑭`** — starts at position **10**, nibble 1. Measured extent: **3 drawn cells** (1472→1803, 331 px = 3.0 pitches). Ends at position **12**, nibble 2. Printed range names 3 fields; measured extent is 3 cells; agrees locally; downstream of the position-5 ellipsis — **STOP 2**. Next field starts at position 13.
7. **`⑮–⑰`** — starts at position **13**, nibble 1. Measured extent: **3 drawn cells** (1803→2133, 330 px = 3.0 pitches). Ends at position **15**, nibble 2. Printed range names 3 fields; measured extent is 3 cells; agrees locally; downstream of the position-5 ellipsis — **STOP 2**. Next field starts at position 16.
8. **`❹–⓱`** — starts at position **16**. Measured extent: **1 undivided drawn region** (2133→2463, 330 px = 3.0 pitches), with no vertical rules and no nibble rule inside it. Ends at position **16**. Nibbles cannot be read: none is drawn — **UNREADABLE, STOP 7**. Printed range names **14** fields (❹ … ⓱); measured extent is **1** drawn region, or **3** cell-pitches if pitch rather than drawn regions is counted (which would put it at positions 16–18). **DISAGREES — STOP 3.** Its printed indices repeat 4–17, already used at counted positions 4–15 — **STOP 4, STOP 5.** Both the printed indices and the measured position are recorded; neither is reconciled to the other. Next field starts at position 17 (drawn-region count) or 19 (pitch count) — both recorded, neither resolved; the CSV records the drawn-region count.
9. **`⑱–㉗`** — starts at position **17**, nibble 1. Measured extent: **3 drawn cells** (positions 17, 18, 19; 2463→2794, 331 px = 3.0 pitches), of which the middle one prints `...` and has no nibble rule. Ends at position **19**, nibble 2 — the strip's right-hand edge, where the bracket's right tick lands. Printed range names **10** fields (⑱ … ㉗); measured extent is **3** drawn cells. **DISAGREES — STOP 6.** Nothing is drawn to the right of position 19; the strip ends there.

Total measured extent of D1: 19 drawn regions, 480 → 2794 = 2314 px = 21.0 cell-pitches. Sum of the nine fields' drawn extents: 2 + 1 + 3 + 2 + 1 + 3 + 3 + 1 + 3 = 19 drawn regions, with no overlap and no gap between consecutive fields — every bracket tick lands on the rule where the next field's first cell begins, and every one of the 19 drawn regions is claimed by exactly one label. Positions 3 and 9 carry no bracket; they are claimed instead by the un-bracketed circled numerals ③ and ⑪, each centred over its single cell (③'s circle centre measured at x = 753 against the cell centre 755; ⑪'s at x = 1414 against the cell centre 1417).

### D2 — `③ Split and Select memory setting`

D2 prints no byte-position numerals. Counted from its own first cell.

- The diagram is **one** outlined box, x 315–630 at 400 dpi (width 315 px), y 1630–1710, with a single dotted vertical rule at x = 472 — the exact mid-point (315 + 157.5 = 472.5).
- **`③`** — starts at position **1**, nibble 1 (the `X` left of the dotted rule). Measured extent: **1 drawn cell**, **2 nibbles** (the dotted rule divides it once and only once). Ends at position **1**, nibble 2. No next field: the diagram has no further cell.
- Two upward arrows leave the box's underside, one beneath each `X`. Nibble 1's leader runs down and right to `0=Split OFF, 1=Split ON`; nibble 2's leader runs down and right to the bracketed list `0=OFF` / `1= ★1` / `2= ★2` / `3= ★3`. See hazard (c).

### D3 — `⑪ Data mode and tone type settings`

D3 prints no byte-position numerals. Counted from its own first cell.

- The diagram is **one** outlined box, x 1858–2173 at 400 dpi (width 315 px), y 1306–1387, with a single dotted vertical rule at x = 2015 — the mid-point (1858 + 157.5 = 2015.5).
- **`⑪`** — starts at position **1**, nibble 1. Measured extent: **1 drawn cell**, **2 nibbles**. Ends at position **1**, nibble 2. No next field.
- Two upward arrows leave the box's underside, one beneath each `X`. Nibble 1's leader runs down and right to `0=Data mode OFF` / `1=Data mode ON`; nibble 2's leader runs down and right to `0: OFF, 1: TONE, 2: TSQL`. See hazard (c).

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — ENCOUNTERED.** D1 draws its index numerals in two distinct styles. Eight of the nine labels are **outlined circled numerals** (thin black circle, black numeral, white ground): `①`, `②`, `③`, `④`, `⑧`, `⑨`, `⑩`, `⑪`, `⑫`, `⑭`, `⑮`, `⑰`, `⑱`, `㉗`. One label, over the long dashed-edged region at counted position 16, is drawn as **filled black discs with the numeral reversed out in white**: `❹` and `⓱`. Both styles are recorded exactly as drawn in `field_index`; they have not been normalised to one, and no meaning has been inferred for either style. D2 and D3 each use the outlined circled style (`③`, `⑪`).
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** Every label in D1, D2 and D3 is set horizontally: the index numerals above the strip, the circled numerals above the D2 and D3 boxes, and all legend text (`0=Split OFF, 1=Split ON`, `0: OFF, 1: TONE, 2: TSQL`, and the rest) run left to right, upright. No label is rotated. The text-extraction half of this hazard cannot have affected this leg because no text layer was read at any point: `pdftotext` was never run, and every position was read from the picture.
- **(c) Leader-line label order may be reversed — ENCOUNTERED, in both D2 and D3.** In **D2** the legends are printed one above the other to the right of the box: the bracketed list `0=OFF / 1= ★1 / 2= ★2 / 3= ★3` is printed **first** (upper), and `0=Split OFF, 1=Split ON` **second** (lower). Following each arrow by eye from the box downwards: the arrow under the **left** `X` (nibble 1) drops well below the box, turns right and runs to the **lower** legend, `0=Split OFF, 1=Split ON`; the arrow under the **right** `X` (nibble 2) drops a shorter distance, turns right and runs to the **upper** bracketed list. The printed top-to-bottom order of the legends is therefore the reverse of the left-to-right order of the nibbles they point at. In **D3** the same reversal occurs: `0: OFF, 1: TONE, 2: TSQL` is printed first (upper) and is reached by the arrow under the **right** `X` (nibble 2), while `0=Data mode OFF / 1=Data mode ON` is printed second (lower) and is reached by the arrow under the **left** `X` (nibble 1). Each leader was traced by eye on the 400 dpi and then again on the 500 dpi render.
- **(d) A printed index may differ from a field's measured position — ENCOUNTERED.** D1 contains a block whose printed indices repeat an earlier block's: the label `❹–⓱` sits over the region at counted position **16**, while the printed indices `④` and `⑰` already occur at counted positions **4** and **15** respectively. Both occurrences have been measured separately and both are recorded: `④–⑧` at counted positions 4–6, `⑨, ⑩` at 7–8, `⑪` at 9, `⑫–⑭` at 10–12, `⑮–⑰` at 13–15, and `❹–⓱` at 16. Neither the printed indices nor the measured positions have been adjusted to fit the other, and the second occurrence was not assumed to match the first — the first is drawn as fifteen separate bracketed cells with individual nibble rules, whereas the second is a single undivided dashed region with none.

## STOP findings

1. **PDF page 169, D1 byte strip, the bracket printed `④–⑧`.** Visual anchor: the fourth, fifth and sixth drawn cells from the strip's left-hand end — grey `X X`, then a grey cell with dashed top and bottom edges printed `...`, then grey `X X`; the bracket's left tick is on the rule just right of the white `③` cell and its right tick is the V shared with the `⑨, ⑩` bracket. What is printed: an index range naming **five** fields (④, ⑤, ⑥, ⑦, ⑧) above **three** drawn cells, the middle of which prints an ellipsis rather than `X X` and has no dotted nibble rule. Why it stops: the measured extent (3 drawn cells, 331 px against a 110.2 px cell pitch) does not add up to the printed numbering (5 fields), and no cell is drawn for ⑤, ⑥ or ⑦. Both are recorded in the CSV — first_byte 4, last_byte 6, alongside the verbatim printed index `④–⑧` — and neither is resolved.
2. **PDF page 169, D1 byte strip, the drawn cell at counted position 5.** Visual anchor: the grey, dashed-edged cell printed `...`, one cell-pitch wide, sitting between the grey `X X` cell that opens the `④–⑧` bracket and the grey `X X` cell that closes it. What is printed: an ellipsis, with no `X`, no dotted nibble rule and no index numeral of its own. Why it stops: the index sequence has a gap here — indices ⑤, ⑥ and ⑦ appear nowhere on the strip — so every counted byte position from position 6 rightwards is a count of *drawn* cells and cannot be reconciled with the printed numbering without an assumption about how many bytes this one cell stands for. That assumption is not made. Every field lying at or beyond position 6 carries this STOP.
3. **PDF page 169, D1 byte strip, the region under the label `❹–⓱`.** Visual anchor: the long region with dashed top and bottom edges, white fill and a single horizontal dotted line inside, lying between the last grey `⑮–⑰` cell and the first grey `⑱–㉗` cell. What is printed: an index range naming **fourteen** fields (❹ … ⓱) above a region with **no vertical divisions of any kind** — no cell rules, no nibble rules — measuring 330 px at 400 dpi against a measured cell pitch of 110.2 px, i.e. exactly three cell-pitches. Why it stops: counting drawn regions puts the whole label at counted position 16 (1 region); counting by cell-pitch would put it at counted positions 16–18 (3 cells); the printed numbering names 14 fields. Three mutually inconsistent counts. The CSV records the drawn-region count (16 to 16) with the printed index verbatim; the pitch measurement is recorded here; none is resolved.
4. **PDF page 169, D1 byte strip, the label above the region at counted position 16 versus the labels above counted positions 4 and 15.** Visual anchor: the two filled black discs bearing white numerals `4` and `17`, joined by a short bar, centred over the long dashed-edged region; compare the outlined circled `④` over the fourth drawn cell and the outlined circled `⑰` closing the bracket over the fifteenth. What is printed: the numerals 4 and 17 appear twice each in the same diagram, once as outlined circled numerals and once as white numerals reversed out of filled black discs. Why it stops: an index printed twice with different styling. Both stylings are transcribed as drawn (`④–⑧`, `⑮–⑰`, `❹–⓱`); no meaning is inferred for either.
5. **PDF page 169, D1 byte strip, the whole span from counted position 4 to counted position 16.** Visual anchor: the five labels `④–⑧`, `⑨, ⑩`, `⑪`, `⑫–⑭`, `⑮–⑰` running left to right over counted positions 4–15, and the single label `❹–⓱` over counted position 16. What is printed: the index numbers 4 to 17 occur twice over in one diagram, first spread across twelve drawn cells plus one ellipsis cell, then compressed into one undivided region. Why it stops: a repeat in the index sequence. Printed index 4 is measured at counted position 4 and also at counted position 16; printed index 17 at counted position 15 and also at counted position 16. Both occurrences are measured and recorded separately and neither is reconciled to the other.
6. **PDF page 169, D1 byte strip, the bracket printed `⑱–㉗`.** Visual anchor: the last three drawn cells at the strip's right-hand end — grey `X X`, then a grey dashed-edged cell printed `...`, then grey `X X` — the bracket's right tick landing on the strip's right-hand edge. What is printed: an index range naming **ten** fields (⑱ … ㉗) above **three** drawn cells, the middle of which prints an ellipsis and has no dotted nibble rule. Why it stops: the measured extent (3 drawn cells, 331 px = 3.0 pitches) does not add up to the printed numbering (10 fields), and no cell is drawn for ⑲ … ㉖. Both recorded; neither resolved.
7. **PDF page 169, D1 byte strip, the interior of the region under `❹–⓱`.** Visual anchor: the white interior of the long dashed-edged region, between its dashed top edge and its dashed bottom edge, containing only a horizontal dotted line. What is printed: no vertical dotted rule anywhere inside the region, and no `X` glyphs. Why it stops: nibble positions are read from the dotted vertical rule that divides a cell's two `X` halves; this region prints none, so no first or last nibble can be counted for this field. The CSV records `UNREADABLE` in both nibble cells for that row rather than carrying a value over from a neighbouring field.

## Observed disagreements

- The prose beneath D1 uses a **tilde** where the diagram uses a **dash** for the same ranges: the diagram prints `④–⑧`, `⑫–⑭`, `⑮–⑰`, `⑱–㉗` with a short thick horizontal bar between the circled numerals, whereas the prose in the two columns below prints `④~⑧ Operating frequency setting`, `⑫~⑭ Repeater tone frequency setting`, `⑮~⑰ Tone squelch frequency setting` and `⑱~㉗ Memory name settings`. In the CSV the diagram's bar is transcribed as an en dash (U+2013), which is the closest match to the glyph as drawn; the tilde form is not used, because the CSV records what the *diagram* prints.
- Two labels in D1 join their members with a **comma** rather than a dash — `①, ②` and `⑨, ⑩` — while the other four multi-member labels use the dash. Recorded verbatim, comma included.
- The `NOTE:` box at the foot of the right-hand column of PDF page 169 prints, verbatim: `• The same data as ④–⑰ are stored in ❹–⓱.`, `• When the Split function is ON, the data of ❹–⓱ is used for transmit.`, and `• Even if the Split function is OFF, enter the data into ❹–⓱ to match your transceiver. We recommend that you set the same data as ④–⑰.` This is recorded because it is printed matter bearing on the repeated index block of STOP 4 and STOP 5. It changed no measurement: no byte position, extent or nibble in the CSV was derived from it, and it was not used to reconcile the printed indices with the measured positions.
- The clearing instructions printed above the `NOTE:` box read `①,②:  Memory channel (00 01~00 99)`, `③:  “FF”`, `④:  None` — here the pair `①,②` is set with no space after the comma, whereas the diagram's own label is set `①, ②` with a space. Recorded as an inconsistency in setting only; the CSV follows the diagram.
- D1's grey/white cell shading does not track the field boundaries in any single consistent way: counted positions 1–2 are grey (`①, ②`), 3 is white (`③`), 4–6 grey (`④–⑧`), 7–8 white (`⑨, ⑩`), 9 grey (`⑪`), 10–12 white (`⑫–⑭`), 13–15 grey (`⑮–⑰`), 16 white (`❹–⓱`), 17–19 grey (`⑱–㉗`). The alternation is per field, not per byte, and the two ellipsis cells at positions 5 and 18 take the grey of the field they sit inside while the large region at 16 is white despite sitting under a label of its own. Noted; it did not stop the count, because every field boundary is fixed by a bracket tick landing on a cell rule, not by the shading.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.
