# Byte-position witness: MR, MT and MW character-position charts

## Source

- **Document title (as printed on the cover, PDF page 1):**
  "FTDX101MP / FTDX101D — CAT Operation Reference Manual", Yaesu Musen Co., Ltd.
  (The cover prints the two model names stacked, `FTDX101MP` above `FTDX101D`,
  with the sub-title "CAT Operation Reference Manual" beneath them.)
- **Revision code as printed:** `2308-L`. It appears in the bottom-right corner of
  the inside back cover (PDF page 26 of 26), below the Yaesu UK address block and
  alongside the "Copyright 2023 YAESU MUSEN CO., LTD." notice. It is not printed on
  the cover or on the chart pages.
- **File path:**
  `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ftdx101_cat_2308-L.pdf`
  (26 pages, A4, PDF 1.7).

All counting below was done from raster renders only. No text-layer extraction
(`pdftotext` or equivalent) was used at any point, and no other file, repository
or external source was consulted.

## Date

05/08/2026

## Rendered pages

The alphabetical two-letter command run begins on PDF page 14 (printed folio 13,
first section `FA`). Locating pass: the whole document was rendered at 60 dpi
(`pdftoppm -r 60 -png … thumb`) purely to find which sheets carry the `M*`
sections; no positions were counted from those thumbnails.

Measuring renders, all at 300 dpi via
`pdftoppm -r 300 -png -f <n> -l <n> … p` (2481 x 3508 px per A4 sheet):

| Section | PDF page | Printed folio | Render file |
|---|---|---|---|
| MR | 17 | 16 | `p-17.png` |
| MT | 17 | 16 | `p-17.png` |
| MW | 18 | 17 | `p-18.png` |

Crops and enlargements (ImageMagick `magick`), all working files kept alongside
these outputs:

- `mr_band.png` — `p-17.png -crop 2481x900+400` (full-width band, unenlarged);
  first pass over the MR block plus the tail of the preceding `ML` block, used to
  confirm the section heading `MR  MEMORY CHANNEL READ` before counting.
- `mr_grid.png` — `p-17.png -crop 900x600+200+680 -resize 200%`; second,
  independent pass over the MR grid at 2x.
- `mt_band.png` — `p-17.png -crop 2481x1200+0+1500` (full-width band);
  first pass over the MT block, heading `MT  MEMORY CHANNEL WRITE/TAG` confirmed,
  and the tail of the preceding `MS` block visible above it.
- `mt_set.png` — `p-17.png -crop 1000x620+180+1680 -resize 180%`; second pass over
  the MT Set and Read grids at 1.8x.
- `mt_ans.png` — `p-17.png -crop 1000x700+180+2250 -resize 180%`; second pass over
  the MT Read (repeat) and Answer grids at 1.8x.
- `mw_band.png` — `p-18.png -crop 2481x1000+0+220` (full-width band); first pass
  over the MW block, heading `MW  MEMORY CHANNEL WRITE` confirmed, with the
  following `MX  MOX SET` block visible below as a boundary check.
- `mw_grid.png` — `p-18.png -crop 1000x520+180+270 -resize 190%`; second pass over
  the MW Set, Read and Answer grids at 1.9x.
- `cover-01.png`, `last-26.png` — 150 dpi renders of PDF pages 1 and 26, used only
  to read the printed title and the revision code.

Adjacent-section discipline: the immediate neighbours on these sheets are `ML`
(MONITOR LEVEL) and `MS` (METER SW) above MR/MT on PDF page 17, and `MX` (MOX SET)
below MW on PDF page 18. Each command block is enclosed in its own heavy rule with
a grey heading bar carrying the two-letter code in italic bold at the left. Every
count below was taken inside the rule of the block whose heading bar reads MR, MT
or MW respectively; the neighbouring blocks were rendered in the same crops
precisely so the block boundary was visible and could not be crossed by mistake.

## Grids found

Each command block is laid out as a stack of row-labelled forms in the left
margin (`Set`, `Read`, `Answer`), each form consisting of one or more *pairs* of
table rows: a thin numbered header row (1…10, 11…20, …) and, directly beneath it,
the token row whose cells align one-for-one with the numbers above. The legend
(P-parameter meanings) sits in a separate column to the right of the grid, outside
the cell rules.

**MR — MEMORY CHANNEL READ (PDF page 17)**

- `Set` — the row label and a numbered header 1…10 are printed, but the token row
  beneath is **completely empty** (ten blank cells). MR therefore provides **no Set
  frame**; nothing measured, no rows emitted.
- `Read` — present, one row pair, measured.
- `Answer` — present, three row pairs (1–10, 11–20, 21–30), measured.

**MT — MEMORY CHANNEL WRITE/TAG (PDF page 17)**

- `Set` — present, five row pairs (1–10, 11–20, 21–30, 31–40, 41–50), measured.
- `Read` — present, one row pair, measured.
- `Answer` — present, five row pairs (1–10, 11–20, 21–30, 31–40, 41–50), measured.

**MW — MEMORY CHANNEL WRITE (PDF page 18)**

- `Set` — present, three row pairs (1–10, 11–20, 21–30), measured.
- `Read` — the row label and a numbered header 1…10 are printed, token row
  **completely empty**. MW provides **no Read frame**; nothing measured.
- `Answer` — the row label and a numbered header 1…10 are printed, token row
  **completely empty**. MW provides **no Answer frame**; nothing measured.

So of the nine possible grids across the three sections, six carry tokens and were
measured (MR read, MR answer, MT set, MT read, MT answer, MW set); three are
printed as empty skeletons (MR set, MW read, MW answer).

## Position arithmetic, per grid

Method common to every grid: the numbered header row and the token row directly
below it share the same vertical cell rules — each numeral sits in its own cell and
the token cell immediately below is bounded by the same left and right rules, so a
token's position is read off by dropping vertically from the numeral above it. A
token spanning *k* consecutive cells therefore occupies *k* consecutive positions;
its `first_pos` is the numeral above its leftmost cell and its `last_pos` the
numeral above its rightmost cell. Where a form has several row pairs, the second
header restarts at 11, the third at 21, and so on, so positions continue without
a break across the wrap. Blank trailing cells after the `;` carry no token and are
recorded as nothing (the frame simply ends at the `;`).

### MR / read (PDF page 17)

Header row: `1 2 3 4 5 6 7 8 9 10`. Token row: `M  R  P0  P0  P0  ;` then four
blank cells.

- Cell 1 = `M`, cell 2 = `R`. These are the literal command letters, taken together
  as one token `MR` → first_pos 1, last_pos 2.
- Cells 3, 4, 5 each print `P0` → three consecutive cells → `P0` = 3–5.
- Cell 6 prints `;` → `;` = 6–6.
- Cells 7–10 blank. Frame length 6, no gap: 1–2, 3–5, 6 covers 1…6 contiguously.

### MR / answer (PDF page 17)

Row pair 1, header `1…10`, tokens: `M R P1 P1 P1 P2 P2 P2 P2 P2`.

- Cells 1–2 = `M`,`R` → token `MR` = 1–2.
- Cells 3,4,5 = `P1` → `P1` = 3–5.
- Cells 6,7,8,9,10 = `P2` (five cells, run continues into the next row pair).

Row pair 2, header `11…20`, tokens: `P2 P2 P2 P2 P3 P3 P3 P3 P3 P4`.

- Cells 11,12,13,14 = `P2`. Combined with 6–10 above, the `P2` run is 6…14 → nine
  positions → `P2` = 6–14. (Cross-check against the legend: P2 is "VFO-A Frequency
  (Hz)", a nine-digit field — consistent, though the CSV value comes from the cell
  count, not the legend.)
- Cells 15,16,17,18,19 = `P3` → five positions → `P3` = 15–19. (Legend: direction
  sign + "Clarifier Offset: 0000 - 9990" = 1 + 4 = 5 — consistent.)
- Cell 20 = `P4` → `P4` = 20–20.

Row pair 3, header `21…30`, tokens: `P5 P6 P7 P8 P9 P9 P10 ;` then two blanks.

- Cell 21 = `P5` → 21–21.
- Cell 22 = `P6` → 22–22.
- Cell 23 = `P7` → 23–23.
- Cell 24 = `P8` → 24–24.
- Cells 25 and 26 both = `P9` → two consecutive cells → `P9` = 25–26. (Legend:
  "P9 00: (Fixed)", two characters — consistent.)
- Cell 27 = `P10` → 27–27.
- Cell 28 = `;` → 28–28.
- Cells 29, 30 blank; frame ends at 28.

Coverage check: 1–2, 3–5, 6–14, 15–19, 20, 21, 22, 23, 24, 25–26, 27, 28 — the
positions run 1…28 with no gap and no overlap; total 28.

### MT / set (PDF page 17)

Row pair 1, header `1…10`: `M T P1 P1 P1 P2 P2 P2 P2 P2`.

- Cells 1–2 = `M`,`T` → token `MT` = 1–2.
- Cells 3–5 = `P1` → `P1` = 3–5.
- Cells 6–10 = `P2` (run continues).

Row pair 2, header `11…20`: `P2 P2 P2 P2 P3 P3 P3 P3 P3 P4`.

- Cells 11–14 = `P2`; with 6–10 the run is 6…14 → `P2` = 6–14 (nine positions).
- Cells 15–19 = `P3` → `P3` = 15–19 (five positions).
- Cell 20 = `P4` → 20–20.

Row pair 3, header `21…30`: `P5 P6 P7 P8 P9 P9 P10 P11 P12 P12`.

- 21 = `P5`, 22 = `P6`, 23 = `P7`, 24 = `P8` — each a single cell.
- 25 and 26 both `P9` → `P9` = 25–26.
- 27 = `P10` → 27–27.
- 28 = `P11` → 28–28.
- 29 and 30 = `P12` (run continues into row pair 4).

Row pair 4, header `31…40`: ten cells all printing `P12`.

- Cells 31…40 = `P12`. With 29–30 above, the `P12` run is 29…40 → twelve positions
  → `P12` = 29–40. (Legend: "P12 TAG Characters (up to 12 characters) (ASCII)" —
  consistent with a twelve-cell count.)

Row pair 5, header `41…50`: `;` in the first cell, cells 42–50 blank.

- Cell 41 = `;` → `;` = 41–41. Frame ends at 41.

Coverage check: 1–2, 3–5, 6–14, 15–19, 20, 21, 22, 23, 24, 25–26, 27, 28, 29–40,
41 — contiguous 1…41, no gap, no overlap; total 41.

### MT / read (PDF page 17)

Header `1…10`. Tokens: `M T P0 P0 P0 ;` then four blanks.

- Cells 1–2 → token `MT` = 1–2.
- Cells 3,4,5 = `P0` → `P0` = 3–5.
- Cell 6 = `;` → 6–6. Frame length 6.

### MT / answer (PDF page 17)

Five row pairs, cell-for-cell identical in layout to MT/set; counted independently
from the enlarged crop `mt_ans.png` rather than copied from the Set reading:

- Row pair 1 (`1…10`): `M`,`T` at 1–2 → token `MT` = 1–2; `P1` at 3,4,5 → 3–5;
  `P2` begins at 6 and fills 6…10.
- Row pair 2 (`11…20`): `P2` at 11,12,13,14 (so `P2` = 6–14); `P3` at 15,16,17,18,19
  (`P3` = 15–19); `P4` at 20.
- Row pair 3 (`21…30`): `P5` 21, `P6` 22, `P7` 23, `P8` 24, `P9` 25 and 26
  (`P9` = 25–26), `P10` 27, `P11` 28, `P12` 29 and 30.
- Row pair 4 (`31…40`): all ten cells `P12`, so `P12` = 29–40.
- Row pair 5 (`41…50`): `;` in cell 41 only, cells 42–50 blank → `;` = 41–41.

Coverage: contiguous 1…41 as for MT/set.

### MW / set (PDF page 18)

Row pair 1, header `1…10`: `M W P1 P1 P1 P2 P2 P2 P2 P2`.

- Cells 1–2 = `M`,`W` → token `MW` = 1–2.
- Cells 3,4,5 = `P1` → `P1` = 3–5.
- Cells 6–10 = `P2` (run continues).

Row pair 2, header `11…20`: `P2 P2 P2 P2 P3 P3 P3 P3 P3 P4`.

- Cells 11–14 = `P2`; with 6–10 the run is 6…14 → `P2` = 6–14 (nine positions).
- Cells 15–19 = `P3` → `P3` = 15–19 (five positions).
- Cell 20 = `P4` → 20–20.

Row pair 3, header `21…30`: `P5 P6 P7 P8 P9 P9 P10 ;` then two blanks.

- 21 = `P5`, 22 = `P6`, 23 = `P7`, 24 = `P8`.
- 25 and 26 both `P9` → `P9` = 25–26.
- 27 = `P10` → 27–27.
- 28 = `;` → 28–28.
- Cells 29, 30 blank. Frame ends at 28.

Coverage check: contiguous 1…28, no gap, no overlap; total 28 — the same shape as
the MR answer frame.

### Double-counting record

Every grid was counted twice from two different rasters (a full-width unenlarged
band, then an independent enlarged crop at 180–200 per cent), the second pass done
without reference to the first pass's numbers. The two passes agreed on every cell
of every grid; there was nothing to reconcile.

## Anomalies

Recorded as seen; none has been "repaired" in the CSV.

1. **MR prints an empty Set form.** The MR block reserves a `Set` row label and a
   numbered header 1…10, but the token row beneath is ten blank cells. The form is
   printed and then left unfilled rather than omitted. Same pattern in MW for both
   `Read` and `Answer` (numbered header printed, token row blank). The blank forms
   still consume vertical space in the block, so at a glance the three sections all
   look as though they have three forms each.
2. **MT's legend label `P0/1` does not match either token as printed in its grids.**
   The legend column for MT begins "P0/1  001-099 (Memory Channel), P1L -P9U (PMS),
   5xx (5MHz BAND), EMG (EMERGENCY CH)", i.e. a combined label. The MT Set and
   Answer grids print `P1` in cells 3–5, whereas the MT Read grid prints `P0` in
   cells 3–5. So the single legend entry `P0/1` covers two differently-named grid
   tokens. The CSV records the tokens exactly as printed in each grid (`P1` for
   set/answer, `P0` for read).
3. **MR uses `P0` in the Read grid but `P1` in the Answer grid,** and its legend
   lists them as separate entries ("P0 001-099 (Memory Channel), P1L -P9U (PMS),
   5xx (5MHz BAND), EMG (EMERGENCY CH)" then "P2 VFO-A Frequency (Hz)"). There is
   no legend entry named `P1` in the MR block even though `P1` appears in the MR
   Answer grid at positions 3–5; the reader is left to infer that MR's Answer `P1`
   is the same field as its Read `P0`. Recorded, not resolved.
4. **MR / MT number their memory-channel field differently from MW.** MR and MT
   legends both extend the channel field to "5xx (5MHz BAND), EMG (EMERGENCY CH)";
   the MW legend gives only "P1  001-099 (Memory Channel), P1L -P9U (PMS)". This is
   a legend difference only and does not affect any position count.
5. **P7 carries a dual meaning in MT.** The MT legend reads "P7  Set: 0: (Fixed) /
   Read: 0: VFO   1: Memory", i.e. one legend line describing two different
   semantics for the same position (28 in MR/MW terms, 23 here). MR's legend gives
   "P7  0: VFO 1: Memory" and MW's gives "P7  0: (Fixed)". Again legend-only.
6. **Legend text sits in its own column and does not intrude into the grids.** In
   all three blocks the P-parameter legend is separated from the cell matrix by a
   full-height vertical rule; no legend line overlaps a cell. This was checked
   specifically because the legend for MR and MT is tall enough to run past the
   bottom of the cell matrix. No interleaving was found.
7. **MT and MR legends are visually tracked-out** ("P0 001-099 (Memory Channel),
   P1L -P9U (PMS), 5xx (5MHz BAND)" is set with wide inter-character spacing to
   justify the line). This affects only the legend text, not the chart cells.
8. **Trailing blank cells after the terminating `;`.** Every measured form ends its
   final row pair with blank cells (MR read: 7–10; MR answer: 29–30; MT set and
   answer: 42–50; MW set: 29–30). These are layout padding to complete the
   ten-cell row, not frame positions, and no token is recorded for them.
9. No misaligned, broken or doubled cell rules were seen in any of the six measured
   grids; every token cell aligned cleanly under exactly one header numeral.

## STOP findings

**None.**

Reason for confidence: at 300 dpi every cell rule in the six measured grids renders
as an unbroken black line, and each token glyph sits wholly within one cell with
clear white gutters either side of the rules — no token straddles a rule, and no
rule is missing or doubled. Each header numeral is individually legible (including
the two-digit numerals 11–50) and sits directly above its token cell, so the drop
from numeral to token is unambiguous for every position. Both counting passes, on
two independently produced rasters at different magnifications, produced identical
token boundaries for all six grids. The three unmeasured forms (MR set, MW read,
MW answer) are not ambiguous either: their token rows render as plainly empty
cells, not as faint or clipped text — checked at 190–200 per cent enlargement,
where any partially-printed glyph would be obvious.
