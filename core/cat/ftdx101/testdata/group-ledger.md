# Table 2 (MENU Chart) — group-boundary ledger

## Source

- **Document title (as printed on the cover, PDF page 1):** `FTDX101MP` / `FTDX101D` (two stacked underlined logotype lines, "DX" set in small capitals), beneath which `CAT Operation Reference Manual`.
- **Revision code as printed:** `2308-L`. It is printed in the bottom right-hand corner of the last page (PDF page 26), below the YAESU UK address block, in the same serif face as the copyright notice. No other revision code is printed anywhere on the rendered pages.
- **File path:** `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ftdx101_cat_2308-L.pdf`
- **Page count:** 26 PDF pages (A4, 595.276 x 841.89 pt).

## Method

Everything below was decided by eye from rasterised page images. No text-layer tool was used at any point.

1. **Overview pass.** All 26 pages rendered with `pdftoppm -png -r 110`, then tiled into a single contact sheet with ImageMagick (`magick ... +append` / `-append`) and read as one image, purely to locate the chart. This showed the chart lives in the middle of the manual and nowhere else.
2. **Working pass.** PDF pages 9–24 and 26 re-rendered individually with `pdftoppm -png -r 400` (3308 x 4678 px per page). Page 1 rendered at 300 dpi for the cover title.
3. **Structure pass (left columns).** For each chart page, native-resolution crops `1320 x 640 px` at x = 270, stepping y in 600 px increments, were read one at a time. At 400 dpi this renders the P1 / P2 / P3 / Function cells large enough that every ruled line and every glyph is unambiguous. Group membership was taken **only** from where the printed rules start and stop, never from vertical whitespace.
4. **Second independent counting pass.** For each chart page, the P3 + Function columns alone were cropped (`580 x 4000 px` at x = 990, y = 400), sliced into four 1000 px bands and re-assembled side by side as a "ladder" image. Each subgroup's row count was re-counted from this ladder without reference to the first pass. Both passes agreed on every subgroup.
5. **Whole-page cross-check.** Each chart page was also viewed whole at 1000 px wide, to confirm the number and order of subgroups and that no subgroup had been missed at a crop seam.
6. **P4 / Digits sweep.** For each chart page the right-hand columns were cropped (`1600 x 4000 px` at x = 1500, y = 400), sliced and re-assembled in two half-page images and read at native resolution, specifically hunting for asterisks, daggers, footnote markers, model names and bracketed qualifiers.
7. **Targeted enlargements.** Individual rows were re-cropped at native resolution and upscaled (`-resize 1600x`) to settle specific readings: the TX GNRL power rows, CW BK-IN DELAY, QSK DELAY TIME, MARK/SHIFT FREQUENCY, CS DIAL, KEYBOARD LANGUAGE, the P1/P2 cell junctions on page 12, and a three-page column-alignment comparison crop.
8. **Boundary confirmation.** The page immediately before the chart (PDF page 10) and the two pages after (PDF pages 14, 15) were rendered and read to confirm where the chart begins and ends, and the band between the table's closing rule and the page footer on each chart page was inspected for footnote text.

**Nothing beyond this single PDF's rendered page images was consulted.** No `pdftotext` or other text-extraction tool was run; no other file, manual, source file, prior transcription or web resource was opened; no directory listing was taken.

## Date

05/08/2026

## Pages used

| PDF page | Printed folio | Role |
|---|---|---|
| 1 | (none printed) | Cover — document title only |
| 10 | 9 | Page before the chart. Ends with the `EX` / `MENU` command block, which is what refers the reader to Table 2. Read to confirm the chart does not start earlier and to check for footnotes attached to the reference. |
| **11** | **10** | **Chart, part 1** |
| **12** | **11** | **Chart, part 2** |
| **13** | **12** | **Chart, part 3 — chart ends here** |
| 14 | 13 | Page after the chart. Begins with the `FA` / `FREQUENCY MAIN BAND` command block — a different table entirely. |
| 15 | 14 | Read only to confirm the command tables simply continue (`ID` / `IDENTIFICATION`); no chart content. |
| 26 | (none printed) | Back cover — read for the revision code `2308-L`. |

Printed folio numbers run **two lower** than the PDF page numbers throughout the chart (PDF 11 = folio 10, PDF 12 = folio 11, PDF 13 = folio 12). Folios are centred in the black footer bar at the foot of each page.

**Where the chart ends.** On PDF page 13 (folio 12) the final ruled row is P1 `04` / P2 `03` / P3 `02` `PIXEL`. Beneath it a single full-width rule closes the P2, P1 and outer table borders simultaneously. Below that rule the lower half of the page is completely blank down to the black footer bar; there is no continuation marker, no footnote and no further ruled cell. PDF page 14 opens with an unrelated command table. The chart therefore occupies exactly PDF pages 11–13 and consists of 18 subgroups.

The chart's header block (`Table 2 (MENU Chart)` banner row, then `P1 | P2 | P3 | Function | P4 | Digits`) is reprinted at the top of each of the three pages.

## Result

**18 subgroups. 193 chart rows in total.**

### Per-P1-group summary

| P1 | P1 label | P2 | P2 label | Rows | PDF page |
|---|---|---|---|---|---|
| 01 | RADIO SETTING | 01 | MODE SSB | 14 | 11 |
| 01 | RADIO SETTING | 02 | MODE AM | 15 | 11 |
| 01 | RADIO SETTING | 03 | MODE FM | 16 | 11 |
| 01 | RADIO SETTING | 04 | MODE PSK/DATA | 16 | 11 |
| 01 | RADIO SETTING | 05 | MODE RTTY | 14 | 11 |
| 01 | RADIO SETTING | 06 | ENC/DEC PSK | 5 | 11 |
| 01 | RADIO SETTING | 07 | ENC/DEC RTTY | 6 | 12 |
| | **01 subtotal** | **7 subgroups** | | **86** | |
| 02 | CW SETTING | 01 | MODE CW | 17 | 12 |
| 02 | CW SETTING | 02 | KEYER | 13 | 12 |
| 02 | CW SETTING | 03 | DECODE CW | 1 | 12 |
| | **02 subtotal** | **3 subgroups** | | **31** | |
| 03 | OPERATION SETTING | 01 | GENERAL | 23 | 12 |
| 03 | OPERATION SETTING | 02 | RX-DSP | 5 | 12 |
| 03 | OPERATION SETTING | 03 | TX AUDIO | 20 | 13 |
| 03 | OPERATION SETTING | 04 | TX GNRL | 7 | 13 |
| 03 | OPERATION SETTING | 05 | TUNING | 7 | 13 |
| | **03 subtotal** | **5 subgroups** | | **62** | |
| 04 | DISPLAY SETTING | 01 | DISPLAY | 8 | 13 |
| 04 | DISPLAY SETTING | 02 | SCOPE | 4 | 13 |
| 04 | DISPLAY SETTING | 03 | EXT-MONITOR | 2 | 13 |
| | **04 subtotal** | **3 subgroups** | | **14** | |
| | **GRAND TOTAL** | **18 subgroups** | | **193** | |

Per-page row totals: PDF page 11 = 80, PDF page 12 = 65, PDF page 13 = 48.

### P3 contiguity

**Yes — P3 numbering is contiguous within every one of the 18 subgroups.** Every subgroup starts at `01` and increments by one with no gap, no repeat and no jump, up to a last value that equals the subgroup's printed row count. This was verified twice: once row by row on the left-column crops, and again independently on the P3/Function "ladder" images. In consequence the printed last-P3 value and the counted row count agree for all 18 subgroups.

### Page straddling

**No subgroup straddles a page break.** Each of the three chart pages both begins with a subgroup at P3 `01` and ends with a subgroup whose last row is closed by the table's bottom border on that same page.

**Two P1 groups do straddle page breaks**, and in each case the P1 label cell is *repeated* on the following page rather than continued:

- P1 `01 (RADIO SETTING)` — subgroups 01–06 on PDF page 11, subgroup 07 on PDF page 12. On page 12 a fresh ruled P1 box, six rows tall, again carries the printed label `01` / `(RADIO SETTING)`.
- P1 `03 (OPERATION SETTING)` — subgroups 01–02 on PDF page 12, subgroups 03–05 on PDF page 13. On page 13 a fresh ruled P1 box, thirty-four rows tall, again carries the printed label `03` / `(OPERATION SETTING)`.

P1 `02 (CW SETTING)` and P1 `04 (DISPLAY SETTING)` each sit wholly within one page.

## Model-applicability sweep

The manual covers two models, printed on the cover as `FTDX101MP` and `FTDX101D`. Every one of the 193 chart rows was inspected — the P1 cell, the P2 cell, the P3 cell, the Function cell, the P4 cell and the Digits cell — at native 400 dpi, looking for asterisks, daggers, superscripts, footnote markers, model names, bracketed model qualifiers or any other conditioning mark. The margins above, below and beside the table on all three chart pages were also inspected, as was the `EX` / `MENU` command block on PDF page 10 that refers the reader to Table 2.

**Markings that condition a row's existence, its P1/P2 labels, its Function name, its Digits value, or whether it is a text item: none found.**

- No asterisk, dagger, superscript, obelus or any other footnote glyph appears anywhere in the chart on PDF pages 11, 12 or 13.
- No footnote or note block is printed beneath, above or beside the table on any of the three pages. On PDF page 11 the space between the table's closing rule and the black footer bar is blank; on PDF pages 12 and 13 the whole area below the table is blank.
- No model name appears in any P1 cell, P2 cell, P3 cell, Function cell or Digits cell.
- No bracketed qualifier in any Function name names a model. The bracketed qualifiers that do occur are all band or mode qualifiers: `RPT SHIFT(28MHz)`, `RPT SHIFT(50MHz)`, `DATA SHIFT (SSB)`.
- Every Digits cell in the chart holds a single number (1, 2, 3, 4 or 12); none holds two model-dependent alternatives.
- The `EX` / `MENU` command block on PDF page 10 (folio 9) carries no model qualifier either; it reads `P1 : 01 - 05`, `P2 : 01 - 07`, `P3 : 01 - 23`, `P4 : Parameter (See Table 2)`.

### Parameter-VALUE ranges (P4 content) that differ by model

These are recorded here for completeness only. They are P4 value ranges, not markings on a row's stored properties. All three occur on PDF page 13 (folio 12), in P1 `03 (OPERATION SETTING)` / P2 `04 (TX GNRL)`. Quoted verbatim from the printed cells:

1. P3 `01`, Function `HF MAX POWER`, P4: `5 ~ 100 (P4 = 005 ~ 100) FTDX101D / 5 ~ 200 (P4 = 005 ~ 200) FTDX101MP` — Digits cell reads `3`.
2. P3 `02`, Function `50M MAX POWER`, P4: `5 ~ 100 (P4 = 005 ~ 100) FTDX101D / 5 ~ 200 (P4 = 005 ~ 200) FTDX101MP` — Digits cell reads `3`.
3. P3 `04`, Function `AM MAX POWER`, P4: `5 ~ 25 (P4 = 005 ~ 025) FTDX101D / 5 ~ 500 (P4 = 005 ~ 050) FTDX101MP` — Digits cell reads `3`.

(The adjacent row P3 `03`, Function `70M MAX POWER`, has P4 `5 ~ 50 (P4 = 005 ~ 050)` with no model qualifier at all, despite 70 MHz being a region-restricted band. Recorded, not resolved. See Observed disagreements for the internal inconsistency in item 3.)

### Attestation

NO row's stored properties are model-conditional

## STOP findings

**None.**

Reasons for confidence:

- Every group boundary in the chart is drawn as a continuous printed rule that visibly crosses the columns it terminates. At each of the 17 internal subgroup boundaries and the 3 internal P1 boundaries, the rule and the columns it does or does not cross were checked in a native-resolution crop that contained the rows on both sides of the boundary simultaneously, so the assignment never depended on comparing two separate images.
- No rule anywhere in the chart is broken, faint, dashed or partially printed at 400 dpi. Every P2 cell has a rule at its top and a rule at its bottom that reach both the P1 rule and the P3 rule.
- Every P1 and P2 label sits unambiguously inside a single ruled box, printed as two centred lines (code above, parenthesised label below). No label sits astride a rule or in a position where it could belong to either of two boxes.
- The three page-continuations are unambiguous because the P1 label is reprinted in full at the top of each continuation page, rather than being left blank for the reader to infer.
- Every P3 sequence runs 01..N without a gap, so the count taken from the rules and the count implied by the printed numbering corroborate each other in all 18 subgroups.
- Every Function name was legible at native 400 dpi without upscaling; the small number of two-line names (`MOUSE POINTER SPEED`) and multi-line P4 cells are enclosed by rules that make the single-row extent obvious.

## Observed disagreements

Recorded exactly as printed; not resolved.

1. **`EX` command's P1 range exceeds the chart.** PDF page 10 (folio 9), `EX` / `MENU` block, right-hand parameter list: `P1 : 01 - 05`. Table 2 prints only four P1 groups, `01` to `04`. (The same block's `P2 : 01 - 07` and `P3 : 01 - 23` do match the chart's maxima.)

2. **`AM MAX POWER` P4 is internally inconsistent.** PDF page 13 (folio 12), P1 `03` / P2 `04` / P3 `04`. Printed: `5 ~ 25 (P4 = 005 ~ 025) FTDX101D / 5 ~ 500 (P4 = 005 ~ 050) FTDX101MP`. The plain-language range `5 ~ 500` does not match its own parenthesised encoding `(P4 = 005 ~ 050)`. The Digits cell reads `3`, which fits `050` but not `500`.

3. **`CW BK-IN DELAY` P4 repeats an index and truncates.** PDF page 12 (folio 11), P1 `02` / P2 `01` / P3 `12`. Printed across two lines: `00:30    01:50    02:100    03:150    04:200    05:250    06:300    07:400    05:250 ....` / `31:2800    32:2900    33:3000msec`. The index `05:250` appears twice, the second time where `08:` would be expected, and the list breaks off with a four-dot ellipsis `....` before resuming at `31:`.

4. **`SHIFT FREQUENCY` P4 repeats index `1` and has no index `0`.** PDF page 11 (folio 10), P1 `01` / P2 `05` / P3 `14`. Printed: `1: 170 Hz    1: 200 Hz    2: 425 Hz    3: 850 Hz`. Four values, but the indices printed are 1, 1, 2, 3.

5. **`MARK FREQUENCY` P4 starts at index `1`, not `0`.** PDF page 11 (folio 10), P1 `01` / P2 `05` / P3 `13`. Printed: `1: 1275 Hz    2: 2125 Hz`. No `0:` entry, yet the Digits cell reads `1`.

6. **`QSK DELAY TIME` P4 contains a transposed spelling.** PDF page 12 (folio 11), P1 `02` / P2 `01` / P3 `16`. Printed: `0: 15 msec    1: 20 msec    2: 25 mesc    3: 30 msec` — `mesc` in the third value, `msec` in the other three.

7. **`LCUT SLOP` versus `HCUT SLOPE`.** Throughout the chart, the low-cut row is printed `LCUT SLOP` (no terminal E) while the matching high-cut row on the very next line is printed `HCUT SLOPE`. This pairing recurs identically in six subgroups: PDF page 11, P1 01 / P2 01, 02 and 03 (rows 05 and 07) and P1 01 / P2 04 and 05 (rows 07 and 09); and PDF page 12, P1 02 / P2 01 (rows 05 and 07).

8. **`KEYBOARD LANGUAGE` list ends with a non-language.** PDF page 12 (folio 11), P1 `03` / P2 `01` / P3 `23`. The twelve values are `00: JAPANESE`, `01: ENGLISH(US)`, `02: ENGLISH(UK)`, `03: FRENCH`, `04: FRENCH(CA)`, `05: GERMAN`, `06: PORTUGUESE`, `07: PORTUGUESE(BR)`, `08: SPANISH`, `09: SPANISH(LATAM)`, `10: ITALIAN`, `11: LEVEL`. `11: LEVEL` is also the last entry of the `CS DIAL` list printed in the row immediately above (P3 `22`).

9. **Function column indentation differs between pages.** On PDF page 13 (folio 12) every Function name is set with roughly one extra space of left indent compared with the same column on PDF pages 11 and 12. Verified by cropping the identical pixel window from all three pages and stacking them.

10. **`RX-DSP` block is indented relative to its neighbour.** PDF page 12 (folio 11), P1 `03` / P2 `02`. Its five Function names (`APF WIDTH`, `CONTOUR LEVEL`, `CONTOUR WIDTH`, `DNR LEVEL`, `IF NOTCH WIDTH`) are visibly indented relative to the `GENERAL` rows directly above them in the same column, within a single crop. In the CSV these names are transcribed with spacing normalised, i.e. without the leading space.

11. **Inconsistent spacing before the colon in P4 index lists.** On PDF page 13 (folio 12), several P4 cells print `00 : OFF` with a space before the colon (`PRMTRC EQ1 FREQ`, `PRMTRC EQ3 FREQ`, `P PRMTRC EQ1 FREQ`, `P PRMTRC EQ3 FREQ`) while others on the same page print `00: OFF` with no space (`PRMTRC EQ2 FREQ`, `P PRMTRC EQ2 FREQ`).

12. **Trailing full stops on some Function names but not others.** `MY CALL.`, `MAIN STEPS PER REV.` and `MPVD STEPS PER REV.` (all PDF page 13) carry a terminal full stop; `MY CALL TIME` on the very next row does not. Transcribed verbatim in the CSV.

13. **Bracket spacing inconsistent within Function names.** `RPT SHIFT(28MHz)` and `RPT SHIFT(50MHz)` (PDF page 11, P1 01 / P2 03) have no space before the opening bracket, whereas `DATA SHIFT (SSB)` (PDF page 11, P1 01 / P2 04, P3 05) does.

14. **`NUMBER STYLE` first value looks like data, not a label.** PDF page 12 (folio 11), P1 `02` / P2 `02` / P3 `06`. Printed: `0: 1290    1: AUNO    2: AUNT    3: A2NO    4: A2NT    5: 12NO    6: 12NT`.
