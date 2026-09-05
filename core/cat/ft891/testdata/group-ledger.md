# Group-boundary ledger — FT-891 CAT manual MENU chart

**Date:** 04/09/2026

## Source

- **Document title as printed** (front cover, PDF page 1): **FT-891 / CAT Operation / Reference Book**, with
  "YAESU MUSEN CO., LTD." at the foot. The running footer on every chart page prints
  **"FT-891 CAT Operation Reference Book"**.
- **Revision code as printed:** **`1909-C`** — found on the **back cover, PDF page 20**, set in a serif
  face at the lower right of the page, below the YAESU UK address block and to the right of it.
  Read at 300 dpi enlarged 400 per cent; the glyphs are unambiguous (`1`, `9`, `0`, `9`, hyphen, capital `C`).
- **File read:** `/Users/stuart/coding/ft710-programmer/docs/fixtures-private/manuals/ft891_cat_1909-C.pdf`
  (rendered pages only — see method).

## Pages used

| PDF page | Printed folio | Content used |
|---|---|---|
| 1 | (no folio) | front cover — document title |
| 8 | 7 | `EX` **MENU** command block, then the first chart page (rows 0101 – 0508) |
| 9 | 8 | chart continues (rows 0509 – 1406) |
| 10 | 9 | chart ends (rows 1407 – 1803); the `FA` FREQUENCY VFO-A block follows below it |
| 20 | (no folio) | back cover — revision code |

The chart begins immediately below the `EX` MENU command block on PDF page 8. That block's own legend
prints `P1 : 0101 - 1803 (MENU Number)` and `P2 : Parameter (See Table below)`, which brackets the chart
at exactly the first and last numbers found in it.

## Method

- Pages were read **only as images**. No text-layer extraction of any kind was performed
  (no `pdftotext`, no `pdfinfo`, no copy-out of PDF text). Every value below was read off a raster.
- Renders: the 20 supplied 300 dpi PNGs (2481 × 3508) were used to locate the chart. Pages 8, 9 and 10
  were then re-rendered from the PDF at **600 dpi** (4961 × 7016) with `pdftoppm -r 600`.
- Crops (all with `magick`, written only into the `work/` subdirectory):
  - **Pass 1 — full-width chart crops.** 4100 px-wide slices of each chart page, downsampled to 1400–1450 px
    wide, giving all four columns at once (`p8-chart-overview.png`, `p9-ov1/ov2.png`, `p10-ov1/ov2.png`).
  - **Pass 2 — left-column crops.** 800 × 1220 px slices of the P1 + Function columns at **native 600 dpi,
    no downsampling** — an enlargement of roughly ×2 over the 300 dpi renders and about ×8 over the printed
    page. At this size a menu number's digits are ~55 px tall, so `0`/`8`/`6` and `1`/`l`/`I` cannot be
    confused (`p8-L1/L2.png`, `p9-L1…L6.png`, `p10-L1…L3.png`).
  - **Pass 3 — P1 + Digits composite strips.** The P1 column and the Digits column were cropped separately
    at native 600 dpi and `+append`ed side by side, so every Digits value is read against its own menu
    number with no risk of a row slip across the wide P2 column (`d8-1/2.png`, `d9-1…6.png`, `d10-1…3.png`).
  - **Pass 4 — targeted zooms** at 180–400 per cent over specific rules and legends
    (`rule-0103-0201.png`, `rule-0520-0601.png`, `z-0904b.png`, `z-1507.png`, `z-0405.png`, `z-0519.png`,
    `z-rev.png`).
- Reconciliation: pass 1 (full-width) and pass 2 (left column) were transcribed independently and compared
  row by row across all three pages. **They agreed on every menu number and every Function name; no
  discrepancy arose, so no third look was needed to settle one.** The Digits column was read twice —
  once in pass 1 and once in pass 3 — and likewise agreed throughout.
- **Nothing else was consulted.** No other file in this repository or anywhere else was opened, no
  directory was listed beyond the two directories named in the brief, no search was run, no web access
  was made, and no recollection of any other Yaesu radio's menu was used. Every value here comes from the
  printed ruled table on PDF pages 8–10 of this one PDF.

## The chart itself

### Column headers, exactly as printed

The chart has **four** columns, headed (in the grey header band, repeated at the top of each of the three
chart pages):

```
P1 | Function | P2 | Digits
```

`P1` and `Digits` are centred; `Function` and `P2` are centred in the header but their cell contents are
left-aligned.

### Group labels

**The chart prints no group-label or sub-group-label column, and no group-label row.** There is no fifth
column, no spanning header, no merged cell, no caption and no marginal text that names or titles the
prefix groups. The only thing that marks a group is the two-digit prefix printed inside the `P1` value
itself. The boundaries in the CSV were therefore derived from the printed prefix alone, cross-checked
against the ruling; no name was inferred for any group.

### Ruling at the boundaries

Every horizontal rule between chart rows is of the **same single weight**, including at every prefix change.
Zooms across 0103→0201 (PDF page 8) and 0520→0601 (PDF page 9) at 200 per cent show the boundary rule to be
indistinguishable from the ordinary row rules on either side of it. There is no thicker rule, no double
rule, no shading, no vertical gap and no blank spacer row at any group boundary. The only heavier rules are
the chart's own outer border and the rule under the repeated grey header band.

The chart's cells run continuously across both page seams. The header band that repeats at the top of
PDF pages 9 and 10 was **not** counted as a chart row.

### Totals

- **Groups (distinct two-digit prefixes): 18** — `01`, `02`, `03`, `04`, `05`, `06`, `07`, `08`, `09`,
  `10`, `11`, `12`, `13`, `14`, `15`, `16`, `17`, `18`. Every value from 01 to 18 is present; none is skipped.
- **Total rows: 159** (sum of `row_count`).
- Per-page check: PDF page 8 (folio 7) 31 rows, PDF page 9 (folio 8) 82 rows, PDF page 10 (folio 9) 46 rows.
  31 + 82 + 46 = 159, matching the group sum.

### Digits column

- **Set of distinct values seen: {1, 2, 3, 4, 5}.** No other value appears; no Digits cell is blank.
- **Rows whose Digits value is greater than 4** — two, both on PDF page 9 (folio 8):

  | Menu number | Function | Digits |
  |---|---|---|
  | 0803 | OTHER DISP | 5 |
  | 0804 | OTHER SHIFT | 5 |

  Both were confirmed twice, in the full-width pass and again in the appended P1+Digits strip
  (`d9-3.png`), where the `5` is unambiguously a five and not a six.

### Free-text parameters

**None.** No row's `P2` legend describes free text. No cell anywhere in the chart contains the words
"characters", "character", "ASCII", "text string", "alphanumeric", or a call sign, and no cell shows a
length-in-characters. The nearest thing is rows 0407–0411 (`CW MEMORY 1` … `CW MEMORY 5`), whose legend is
the enumerated pair `0: TEXT   1: MESSAGE` — a two-value choice, not a free-text field — and 0406
`CONTEST NUMBER`, whose legend is the numeric range `0000 - 9999`.

## STOP findings

**None.**

Specifically, and each checked twice:

- **Non-monotonic menu numbers: none.** Read top to bottom across all three pages, every menu number is
  strictly greater than the one above it, at both page seams included (0508 → 0509 across the page 8/9 seam;
  1406 → 1407 across the page 9/10 seam).
- **Duplicated menu numbers: none.** All 159 numbers are distinct.
- **Prefix disagreeing with the ruled grouping: none** — and note that this could not arise here, because
  the ruling does not itself mark any grouping (see "Ruling at the boundaries"). Within every group the
  final two digits run consecutively from `01` upward with no gap: 01→03, 01→07, 01→02, 01→11, 01→20,
  01→07, 01→13, 01→12, 01→06, 01→11, 01→09, 01→04, 01→02, 01→07, 01→18, 01→23, 01→01, 01→03.
- **Ambiguous ruling: none.** Every row is closed by a full-width rule; both double-height rows (1504,
  1513) carry exactly one menu number and are bounded by exactly one rule above and one below.

## Printing defects noticed (recorded, not resolved)

1. **0905 `RPT SHIFT 50MHz`, PDF page 9 (folio 8)** — the `Digits` cell prints **`1`**, but the row's `P2`
   legend is `0 - 4000 kHz (P2= 0000 - 4000) (10 kHz/step)`, a four-digit parameter. The immediately
   preceding row 0904 `RPT SHIFT 28MHz` carries the same shape of legend
   (`0 - 1000 kHz (P2= 0000 - 1000) (10 kHz/step)`) and prints `Digits` = **`4`**. Verified in the
   appended P1+Digits strip `work/d9-4.png` and again full-width in `work/z-0904b.png`, where 0904/0905/0906
   appear together with their Digits cells: 4, 1, 1. Recorded as printed; not resolved.
2. **1507 `EQ3 FREQ` and 1516 `P-EQ3 FREQ`, PDF page 10 (folio 9)** — both legends end
   `06: 2000 Hz -18: 3200 Hz`. The run from index 06 to index 18 is set with no space before the `18` and
   with the same hyphen the manual uses for a minus sign elsewhere in this column
   (e.g. `-20 - 0 - +10` two rows above), so the token reads ambiguously as a negative index. Zoom at
   `work/z-1507.png`. Recorded as printed; not resolved.
3. **1504 `EQ2 FREQ` and 1513 `P-EQ2 FREQ`, PDF page 10 (folio 9)** — the option `04: 1000Hz` is printed
   with no space between value and unit, whilst every neighbouring option in the same cell prints a space
   (`03: 900 Hz`, `05: 1100 Hz`). Both rows are the chart's only double-height cells, wrapping their
   legend onto a second line ending `09: 1500 Hz`.
4. **0404 `BEACON INTERVAL`, PDF page 8 (folio 7)** — an unpadded range head against padded parameter
   values: `OFF/1 - 690 sec (P2= 000 - 690, 000: OFF)`, `Digits` = 3. The visible head `1` is not padded
   to `001` the way the parenthesised `P2=` values are. Zoom at `work/z-0405.png`.
5. **0803 `OTHER DISP` and 0804 `OTHER SHIFT`, PDF page 9 (folio 8)** — the step suffix is printed
   `(10 Hz/steps)`, plural, where the same construction elsewhere in the chart is singular
   (`(10 kHz/step)` at 0904/0905, `(10 msec/step)` at 0709 and 1618). Their `P2` also prints the signed-zero
   pair `-3000 - -0000 or +0000 - +3000`, i.e. two distinct spellings of zero.

No legend anywhere in the chart prints the same index number twice, and no option list runs out of order:
0405 `NUMBER STYLE` (0–6), 0519 `APO` (0–7), 0906 `DCS POLARITY` (0–3), 1615 `TUNER SELECT` (0–3) and
1109 `SSB TX BPF` (0–4) were each zoomed and checked index by index.
