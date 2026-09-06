# Transcription B — TS-480 EX (Extension Menu) chart

**Date:** 05/09/2026

## Source, as printed

- Title (cover, PDF p.1): "PC CONTROL COMMAND REFERENCE FOR THE TS-480HX/ SAT TRANSCEIVER"
- Publisher line (cover): "KENWOOD CORPORATION"
- Revision/date as printed (cover, below publisher line): "© 2003/11/28" followed by the
  printing-code line "09 08 07 06 05 04 03 02 01 00". No other revision or part number is
  printed on the cover.
- File: `docs/fixtures-private/manuals/ts480_pc_2003.pdf`, 26 PDF pages.

## Pages used

| PDF page | Printed folio | Content used |
|---|---|---|
| 1 | (none) | cover — title, publisher, copyright date |
| 7 | 6 | the EX command block (context only: header/definition of P1…P5) |
| 8 | 7 | menu chart, rows 000–037 |
| 9 | 8 | menu chart, rows 038–060 |

Folio numbering runs one behind PDF page numbering throughout (PDF p.2 = folio 1).

The chart transcribed is the ruled table printed immediately after the EX command block. Its
header row reads: `Menu No.` | `Function` | `EX command parameter P5`, with the P5 group
subdivided into eleven column headers: `0 1 2 3 4 5 6 7 8 9 Over`.

**Note on the header:** this chart has **no `10` and no `10~` column.** The eleventh and last
grid column header is the word `Over`. Both pages of the chart repeat the identical header
(`Menu No.` / `Function` / `EX command parameter P5` + `0`…`9` / `Over`); the repeated header on
PDF p.9 was not counted as a chart row.

## Method

- Baseline: the supplied 300 dpi renders `p-01.png` … `p-26.png`. Pages 8 and 9 were re-rendered
  from the PDF at **600 dpi** with `pdftoppm -r 600` (4959 × 7009 px each) and cropped with
  `magick` into overlapping strips written under `…/quarantine/B-480/work/`.
- **Pass 1 (whole page, 300 dpi):** `p-08.png` and `p-09.png` read entire, establishing row order,
  row count and page seam.
- **Pass 2 (600 dpi strips, independent):** each page split into a left band
  (menu number + Function, 1460 px wide, viewed at ~1:1 → effective **≈600 dpi**, i.e. 2× the
  baseline) and a right band (the whole P5 grid, 2960 px wide, viewed at 2000 px → effective
  **≈405 dpi**). Four strip pairs for PDF p.8, three for PDF p.9.
- **Pass 3 (number column only, 600 dpi):** the `Menu No.` column alone was cropped as a narrow
  240 px band, split into three (p.8) and two (p.9) vertical chunks and appended side by side,
  viewed at ~1:1 (**≈600 dpi**), so the whole number sequence could be checked in one view.
- At ≈600 dpi the digits `0`, `6` and `8` and the glyphs `l`, `1`, `I` are separated by many pixels
  of stroke and counter; no character in the number column or in any Function string was
  ambiguous at that magnification.
- Ruled rows were followed continuously across the p.8 → p.9 seam: p.8 ends at 037, p.9 begins
  at 038, and the p.9 header was excluded.
- **Reconciliation.** Pass 1 and Pass 2 independently gave the same 61 numbers, the same order and
  the same last-populated grid column for every row. Pass 3 (number column in isolation)
  reproduced the same sequence a third time. **No discrepancy arose between the passes**, so no
  third look was needed to settle anything; Pass 3 was performed as a planned confirmation, not
  as a tie-break.
- Nothing except image rendering/cropping (`pdftoppm`, `magick`) and my own file writes was run.
  No command was left running.

## Nothing else consulted

Only the PDF named above — via rendered page images — was consulted. No text layer was extracted
(`pdftotext`, `pdfinfo` and any copy-from-PDF route were not used). No other file in the
repository or elsewhere was opened, no directory was listed except my own scratch output
directory and the supplied pre-render directory, no search was run, no web access was made, and
no prior knowledge of any other Kenwood radio's menu — or of the second radio variant named in
this document — was used. Every value below comes from the printed ruled table.

## Chart shape

- 61 rows, `000` through `060`, one per menu number, all printed with three digits and a leading
  zero.
- 38 rows on folio 7 (000–037), 23 rows on folio 8 (038–060).

---

# FINDINGS

## 1. TEXT rows (prose printed across the grid instead of cells)

There is exactly one prose block, and it spans **five** chart rows.

- **Rows 048, 049, 050, 051, 052** — `Remote Control panel PF key`, `Microphone PF1 key`,
  `Microphone PF2 key`, `Microphone PF3 key`, `Microphone PF4 key`.
  A single cell, merged across all eleven grid columns (`0`…`9` and `Over`) **and** down all five
  rows, prints on two lines:

  > `00 ~ 99 (2-digit)`
  > `Refer to page 64 of the TS-480 instruction manual for the numbers and functions.`

  Recorded as `text=1` (prose across the grid, per the brief's structural test) and `digits=2`
  (the character count the prose states: "(2-digit)").

  **Caveat recorded, not resolved:** the prose describes a two-digit *numeric* range (00–99), not
  a character/ASCII string, a message or a version string. A consumer that reads `text` as "the
  parameter is a character string" would want `text=0, digits=2` for these five rows. The brief's
  own structural definition ("a row that prints prose across the grid instead of cells is a TEXT
  row") and its TEXT-row digits rule ("the character count the prose states") both apply cleanly
  here, and the alternative rule (character width of the header above the last populated cell) is
  undefined for a cell that spans every header, so `text=1` was recorded. Flagging for
  reconciliation.

No other row prints prose across the grid. No row prints "read only", an ASCII-character
statement, a message description or a version string.

## 2. Rows recorded with `digits` ≥ 2, and what the `Over` cell prints

Three rows besides the 048–052 block:

| Menu | Function | cells under `0`…`9` | `Over` cell prints |
|---|---|---|---|
| 032 | Interval time for repeating the playback | 0 1 2 3 4 5 6 7 8 9 | `~ 60` / `(in steps of 1 s)` (two lines) |
| 034 | CW RX pitch/ TX sidetone frequency | 400 450 500 550 600 650 700 750 800 850 | `~ 1000` / `(in steps of 50)` (two lines) |
| 035 | CW keying dot, dash weight ratio | AUTO 2.5 2.6 2.7 2.8 2.9 3.0 3.1 3.2 3.3 | `~ 4.0` / `(in steps of 0.1)` (two lines) |

In each of these three the `Over` cell **continues the value sequence past code 9** (the row fills
every one of columns `0`…`9` first), so the largest P5 code the row uses is two characters wide.

Plus, for completeness, the five prose rows 048–052 carry `digits=2` from their printed
"(2-digit)".

## 3. The `Over` column also carries unit labels — seven rows where `digits` was recorded as 1

Seven rows stop well short of column `9` yet still print something in the `Over` column. In every
one of these the `Over` cell holds only a **unit label in parentheses**, not a P5 value:

| Menu | Function | last *value* cell | header above it | `Over` cell prints |
|---|---|---|---|---|
| 003 | Tuning control adjustment rate | `1000` | `2` | `(Hz)` |
| 009 | Slow down frequency range for the Program scan | `500` | `4` | `(Hz)` |
| 022 | Time-out timer | `30` | `5` | `(minutes)` |
| 041 | FSK shift | `850` | `3` | `(Hz)` |
| 043 | FSK tone frequency | `2125` | `1` | `(Hz)` |
| 056 | COM port communication speed | `115200` | `5` | `(bps)` |
| 059 | APO (Auto Power Off) function | `180` | `3` | `(minutes)` |

**Decision recorded, and the alternative stated.** The brief's governing definition is "the
character width of the LARGEST P5 code the row uses". A unit label is not a P5 code, so these
seven rows were recorded as `digits=1`. A strictly literal reading of the brief's operational
gloss — "the number of characters in the column HEADER above the row's last *populated* cell …
a row reaching the `Over` column → 2" — would instead give `digits=2` for all seven, since the
`Over` cell is literally populated. **The rows that would change under that reading are exactly:
003, 009, 022, 041, 043, 056, 059.** No other row is affected. Recorded, not resolved.

## 4. Numbering sequence

- The sequence runs `000, 001, 002, … 059, 060` with no exception: strictly increasing, step 1,
  no number repeated, no number missing, none out of order, across both pages and across the
  page seam.
- Confirmed three times (Passes 1, 2 and 3).

## 5. Cells straddling columns

- The 048–052 prose block is the only merged cell in the body of the chart: it straddles all
  eleven grid columns and five rows (see finding 1).
- In the header, `EX command parameter P5` straddles all eleven grid columns of the sub-header
  row; `Menu No.` and `Function` each straddle both header rows. These are header spans, not
  body rows.
- No other body cell straddles a column boundary. Every value cell sits squarely inside one
  column's rules.

## 6. Cells that could not be read with confidence

None. Every cell used to derive `menu_number`, `name`, `digits` and `text` was read at ≈600 dpi
(Function and number columns) or ≈405 dpi (grid), and every character was unambiguous.

## 7. Printing oddities and cell contents worth flagging

Recorded because they look like defects, not because they changed any CSV value.

- **Row 011 `Scan resume method`** — the cells under headers `0` and `1` print the lower-case
  fragments `to` and `co` respectively. Verified at 600 dpi: they are two-letter lower-case
  strings, not truncated by the crop, and there is nothing else in the cells. They read as
  abbreviations for the radio's own display legends. Flagged as a probable source defect
  (glyph/abbreviation) rather than a reading failure.
- **Rows 018 and 019 (`DSP RX equalizer`, `DSP TX equalizer`)** — cells print
  `OFF Hb1 Hb2 FP bb1 bb2 c U` under headers `0`…`7`. The mixed case (`Hb1`, `bb1`, lone `c`,
  lone `U`) is unusual but each glyph was legible at 600 dpi. Both rows are identical to each
  other, cell for cell.
- **Row 027 `Control method for the external AT`** — cells print `At1` / `At2` (capital A,
  lower-case t), verified at 600 dpi.
- **Row 056** — the cell `115200` under header `5` is set in a visibly smaller point size than its
  neighbours (`4800`, `9600`, `19200`, `38400`, `57600`) to fit the column width. Legible; noted
  as typography only.
- **Row 053** — the Function text wraps as `Split frequency transfer in master/` +
  `slave operation`; the slash sits at the end of the first line and is followed by a space in
  the joined form recorded in the CSV.
- **Row 034** — the Function text prints as `CW RX pitch/ TX sidetone frequency` on one line, with
  a space after the slash as printed.

## 8. Cross-page context, recorded not applied

The EX command block on folio 6 (PDF p.7) states, under `P5`:
"A string of characters (Variable length) / Normally 1-digit for the TS-480. /
Menu No. 32, 35 and 48 ~ 52 use 2-digit parameters."

This list omits **menu 034**, whose chart row demonstrably needs two-digit codes (values
400…1000 in steps of 50 = 13 values, i.e. codes 0…12, with the `Over` cell reading
`~ 1000 (in steps of 50)`). The CSV follows the **chart**, as instructed, and records `digits=2`
for 034. The discrepancy between the chart and the folio-6 note is recorded here, unresolved.
