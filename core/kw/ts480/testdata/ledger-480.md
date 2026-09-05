# Boundary ledger — TS-480 menu chart (L-480)

**Date:** 05/09/2026

## Source document (as printed)

- Cover title, verbatim over three lines: `PC CONTROL COMMAND` / `REFERENCE FOR THE` /
  `TS-480HX/ SAT TRANSCEIVER`.
- Publisher line: `KENWOOD CORPORATION`.
- Revision/date as printed on the cover: `© 2003/11/28`, followed by the printing-year ladder
  `09 08 07 06 05 04 03 02 01 00`. **No part number, edition number or revision letter is
  printed anywhere on the cover.** That copyright date is the only revision marker found.
- Running head on every chart page: `PC CONTROL COMMAND`.

## Pages used

| PDF page | Printed folio | Content |
|---|---|---|
| 1 | (unnumbered cover) | title and revision line only |
| 7 | 6 | the `EX` command block — read only to locate the chart that follows it |
| 8 | 7 | chart rows 000–037 |
| 9 | 8 | chart rows 038–060 |

The PDF page number and the printed folio differ by one throughout (PDF page = folio + 1).

## Method

- Worked from the pre-rendered 300 dpi page images (`p-01.png` … `p-26.png`) and from fresh
  600 dpi renders of PDF pages 1, 7, 8 and 9 made with `pdftoppm -r 600`.
- **Pass 1 (locate and read whole):** a contact sheet of all 26 pages at ~55 dpi to find the
  chart, then each chart page read whole at 1500 px wide (≈181 dpi for an A4 page).
- **Pass 2 (independent re-read of the number column and the Function column):** 600 dpi crops
  1400 px wide covering the `Menu No.` and `Function` columns in overlapping vertical bands,
  displayed at or near 1:1. At this magnification `0`/`8`/`6` and `l`/`1`/`I` are unambiguous —
  every menu number was re-read from the glyph, not inferred from its neighbours.
- **Pass 2 (independent re-read of the grid and of its last-populated column):** 600 dpi crops
  2760 px wide covering headers `0` … `Over` in overlapping bands (≈438 dpi effective), plus a
  **separate dedicated 1000 px-wide crop of the `Over` column alone** run down the full height of
  each page, so the last-populated column was counted a second time in isolation from the rest of
  the grid.
- **Reconciliation:** the two passes agreed on every menu number, on the row counts (38 and 23),
  and on the set of rows reaching the `Over` column. **No discrepancy arose, so no third look was
  needed.**
- Nothing was extracted from the PDF text layer. No `pdftotext`, no `pdfinfo`, no copy-paste.
- **Nothing else was consulted** — no other file in the repository, no other page of this document
  beyond those listed above, no web access, no prior knowledge of any other radio's menu list.

## The chart's column headers, exactly as printed

The header is two tiers, shaded grey:

- Tier 1 (left to right): `Menu` / `No.` (one cell, set on two lines), `Function`, and
  `EX command parameter P5` spanning the whole grid.
- Tier 2 (the grid's own headers, under `EX command parameter P5`):
  `0`, `1`, `2`, `3`, `4`, `5`, `6`, `7`, `8`, `9`, `Over`.

That is the complete header set: **eleven grid columns, ten single-digit ones and `Over`.**
The identical header is reprinted at the top of PDF page 9; it is a repeated page header and is
**not** counted as a row.

**Group-label column / group headings: there are none.** The chart has exactly three structural
parts — the menu-number column, the Function column, and the P5 grid. No group column, no group
heading row, no shaded band or rule dividing the sequence into named sections, no numbering
prefix. The chart is one flat numeric sequence from top of PDF page 8 to bottom of PDF page 9.
(Recorded as observed; nothing inferred.)

## Totals

- **Rows:** 38 (PDF page 8) + 23 (PDF page 9) = **61**.
- **Pages occupied by the chart:** 2 (PDF pages 8 and 9; printed folios 7 and 8).

## Sequence

- **Lowest menu number:** `000` (Display brightness).
- **Highest menu number:** `060` (Transmit with the audio input on the DATA terminal).
- **Contiguous: yes.** 000 … 060 inclusive, step 1, no gap and no duplicate. 61 printed numbers
  for 61 sequence positions. The seam between the pages is continuous: 037 is the last row of
  PDF page 8 and 038 the first row of PDF page 9.
- **STOP findings arising from the sequence: none.**

## TEXT rows (prose printed across the grid instead of ruled option cells)

Five rows, all on PDF page 9, share **one** merged cell that spans the full width of the grid
(columns `0` through `Over`) and the full height of all five rows:

| Menu No. | Function |
|---|---|
| 048 | Remote Control panel PF key |
| 049 | Microphone PF1 key |
| 050 | Microphone PF2 key |
| 051 | Microphone PF3 key |
| 052 | Microphone PF4 key |

The prose in that merged cell, verbatim on two lines:

```
00 ~ 99 (2-digit)
Refer to page 64 of the TS-480 instruction manual for the numbers and functions.
```

No other row in the chart prints prose across the grid. There is no "read only" row and no
"ASCII characters" row anywhere in this chart.

## Rows whose grid reaches a column header of two or more characters

**First, the boundary fact:** this chart has **no numeric column header of 10 or above.** The grid
headers stop at `9`. The only header of two or more characters is `Over`, and it is the eleventh
and last column. So the criterion resolves here to "rows that populate the `Over` column" — ten
rows in all, listed below with the highest header their grid reaches.

These ten split into two distinct uses of the same column, which the chart does not distinguish
typographically:

**(a) `Over` carrying a genuine continuation of the option list (P5 values above 9 exist):**

| Menu No. | Function | Highest header used | `Over` cell, verbatim |
|---|---|---|---|
| 032 | Interval time for repeating the playback | Over | `~ 60` / `(in steps of 1 s)` |
| 034 | CW RX pitch/ TX sidetone frequency | Over | `~ 1000` / `(in steps of 50)` |
| 035 | CW keying dot, dash weight ratio | Over | `~ 4.0` / `(in steps of 0.1)` |

**(b) `Over` carrying only a unit annotation for the row (no additional option meaning):**

| Menu No. | Function | Highest header used | `Over` cell, verbatim |
|---|---|---|---|
| 003 | Tuning control adjustment rate | Over | `(Hz)` |
| 009 | Slow down frequency range for the Program scan | Over | `(Hz)` |
| 022 | Time-out timer | Over | `(minutes)` |
| 041 | FSK shift | Over | `(Hz)` |
| 043 | FSK tone frequency | Over | `(Hz)` |
| 056 | COM port communication speed | Over | `(bps)` |
| 059 | APO (Auto Power Off) function | Over | `(minutes)` |

Recorded as printed; the two uses are not resolved into one another here.

For completeness, the rows that reach the highest *single-digit* header, `9`, are 012, 013, 014,
032, 034, 035, 046 and 047. Row 015 stops at `7`; rows 018 and 019 stop at `7`.

## Printing defects and irregularities (recorded, not resolved)

1. **`Over` column does double duty.** As set out above, in seven rows the `Over` cell holds only a
   unit label — `(Hz)`, `(minutes)`, `(bps)` — which is an annotation for the whole row and not the
   meaning of a P5 code, while in three rows (032, 034, 035) it holds a real continuation of the
   option list. Nothing in the ruling, shading or type distinguishes the two.
2. **Menu 034 is missing from the EX block's list of 2-digit menus.** The `EX` command block on
   PDF page 7 (folio 6) states, verbatim: `Menu No. 32, 35 and 48 ~ 52 use 2-digit parameters.`
   The chart's `Over` cell for menu 034 (`~ 1000` / `(in steps of 50)`) requires P5 values above 9
   for menu 034 as well, yet 034 is not in that list. The two statements are inconsistent as
   printed. Recorded; not resolved.
3. **Five rows share one cell.** For 048–052 the horizontal rules that separate the rows stop at
   the right-hand edge of the `Function` column; across the grid there is a single tall cell. The
   row identity of 048–052 therefore rests entirely on the `Menu No.` and `Function` columns.
4. **Cell text set to fit.** In menu 056 the value `115200` in column 5 is printed in a visibly
   smaller/condensed face than the neighbouring `4800 … 57600`, to fit the cell width. Typographic
   only; the digits are unambiguous at 600 dpi.
5. **Option lists that are not a plain ascending run.** Menu 035 begins with `AUTO` in column 0 and
   only then runs 2.5, 2.6 … 3.3; menu 011 prints the two lowercase abbreviations `to` (col 0) and
   `co` (col 1); menus 018 and 019 print `OFF, Hb1, Hb2, FP, bb1, bb2, c, U` in columns 0–7; menu
   027 prints `At1`/`At2`. The mixed upper/lower case in these cells is exactly as printed (they
   are the radio's LCD legends) and is not a rendering artefact — each was checked at 600 dpi.
6. **Value/header offset.** Menu 044 (`Mic gain for FM`) prints `1`, `2`, `3` under headers `0`,
   `1`, `2`, so the cell value is one greater than the P5 code; menu 000 prints `OFF, 1, 2, 3, 4`
   under `0`–`4`. Recorded as a reading hazard, not as an error.
7. No duplicated header, no cell straddling two columns, and no unreadable cell was found on
   either page.
