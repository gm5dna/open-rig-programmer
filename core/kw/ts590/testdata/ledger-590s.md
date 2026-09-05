# Boundary ledger — "EX Command Parameter List (for TS-590S)"

Date: 05/09/2026

## Source

- Title as printed on the cover (PDF page 1): **TS-590S / TS-590SG — PC CONTROL COMMAND
  Reference Guide**, "JVCKENWOOD Corporation", "January/30/2019".
- Running header on every body page: "PC CONTROL COMMAND REFERENCE GUIDE".
- **Revision: no revision number is printed anywhere I looked.** The cover carries only the
  date "January/30/2019"; the back cover (PDF page 35) carries "© 2019 JVCKENWOOD
  Corporation" and the part code "B5A-0316-20/01". The "rev3" in the supplied filename is
  NOT printed in the document. Recorded, not resolved.
- File: `docs/fixtures-private/manuals/ts590sg_pc_rev3.pdf` (35 PDF pages).

## Pages used

The TS-590S chart occupies three PDF pages:

| PDF page | Printed folio | Rows |
|---|---|---|
| 8  | 7 | 000–010 (11) |
| 9  | 8 | 011–059 (49) |
| 10 | 9 | 060–087 (28) |

Folio = PDF page − 1 throughout (PDF page 2 is folio 1).

The chart's heading "EX Command Parameter List (for TS-590S)" is printed on PDF page 8, below
the `EX` command block and above the table's header band. The chart ends on PDF page 10, where
the next heading, "EX Command Parameter List (for TS-590SG)", begins the *other* radio's list —
that second list was not read (it is another agent's).

## Method

- Base renders: the supplied 300 dpi page PNGs (`p-01.png` … `p-35.png`), used only to locate
  the chart (low-resolution contact strips built with `magick`).
- Reading renders: the three chart pages re-rendered from the PDF at **600 dpi** with
  `pdftoppm -r 600` in vertical bands (page 8 bottom band; page 9 upper and lower bands; page 10
  upper and lower bands). At 600 dpi the digit forms `0`/`6`/`8` and the letters `l`/`1`/`I` are
  unambiguous.
- Third-look enlargement: the first and last Function cells of each page cropped from the 600 dpi
  renders and enlarged a further **300 %** (effective ~1800 dpi) to fix punctuation and wrapping.
- Pass 1: full-width reading of every band, transcribing the number column, Function column and
  the whole P5 grid.
- Pass 2 (independent): for each band, the Menu (P1) column and the last two grid columns ("9"
  and "10 ~") were cropped out separately and appended side by side, so the number column and the
  last-populated column were re-read without the intervening cells. This isolates exactly the two
  things the brief asks to reconcile.
- **Discrepancies between the two passes: none.** Both passes give 000–087 with no gap and no
  repeat, and both give the same four rows reaching the "10 ~" column. No third look was needed
  to settle a conflict; the third look was used only to confirm verbatim Function wording.
- No text-layer extraction was used at any point. Nothing outside the single PDF and its renders
  was consulted — no other repository file, no other manual, no web access, no directory listing
  of the manuals tree.

## Column headers, exactly as printed

The header band is two rows deep and identical on all three pages:

- Left column, stacked over two lines: `Menu` / `(P1)`
- Second column: `Function`
- The remaining width is one spanning cell, `Command Parameter (P5)`, subdivided beneath into
  eleven column headers:

  `0`  `1`  `2`  `3`  `4`  `5`  `6`  `7`  `8`  `9`  `10 ~`

The last header is printed `10 ~` (numeral, space, tilde) — not "10~ / Over".

**Group column / group headings: there are none.** The chart is one flat ruled table with no
grouping column, no shaded group-label rows and no sub-headings between rows. The only heading
is the single line above the table, "EX Command Parameter List (for TS-590S)". Nothing is
inferred beyond what is ruled and printed.

## Totals

- **Rows: 88** (11 + 49 + 28).
- **Pages: 3** (PDF 8–10, folios 7–9).
- Lowest menu number: **000**. Highest: **087**.
- **The sequence is contiguous**: every integer 000 … 087 appears exactly once, in ascending
  order, across the three pages and across both page seams (010 → 011, 059 → 060).
  **No gap and no duplicate — no STOP finding of this kind.**

## TEXT rows (prose across the grid instead of cells)

Two prose blocks, covering nine rows in total:

1. **079–086** — a single prose cell spanning the whole P5 grid and all eight of these ruled
   rows at once (the ruled row lines continue through the Menu and Function columns but stop at
   the grid's left edge). Printed text:
   `000 ~ 255 (3-digit)` / `Refer to the TS-590S instruction manual for the numbers and functions. (When the function is turned OFF, 255 is used.)`
   The eight Functions are:
   - 079 `Panel PF A function`
   - 080 `Panel PF B function`
   - 081 `Mic PF 1 function`
   - 082 `Mic  PF 2 function`
   - 083 `Mic  PF 3 function`
   - 084 `Mic  PF 4 function`
   - 085 `Mic  PF (DWN) function`
   - 086 `Mic  PF (UP) function`
   (Note the inconsistent spacing as printed: 081 sets `Mic PF 1 function` with one space, while
   082–086 set two spaces after `Mic`. Recorded as printed, not normalised.)
2. **087 `Power on message`** — prose cell spanning the whole grid:
   `Power on Message (up to 8 ASCII characters)`

There is **no** "read only" row and **no** "Firmware Version" row in this (TS-590S) chart; its
first row is 000 `Display brightness`.

## Rows whose grid reaches a header of two or more characters (the `10 ~` column)

Four rows, all with `10 ~` as the highest header used:

| Menu | Function | Cell printed under `10 ~` |
|---|---|---|
| 034 | Side tone/ pitch frequency setting (Hz) | `up to 1000 (steps of 50)` |
| 036 | Keying weight ratio | `up to 4.0 (steps of 0.1)` |
| 057 | Voice/ message playback repeat duration (seconds) | `up to 60 (steps of 1)` |
| 070 | DATA VOX delay | `up to 100 (steps of 5)` |

No other row has anything printed in that column. In particular 015 and 016
(`MULTI/CH control step change for AM (kHz)` / `... for FM (kHz)`) run 5, 6.25, 10, 12.5, 15,
20, 25, 30, 50, 100 and stop at header `9`; their `10 ~` cells are empty.

## Printing defects and oddities (recorded, not resolved)

- **No revision number printed** (see Source above). This is a gap between the filename and the
  document, recorded for the caller to settle.
- **Duplicated header band**: the full two-row header (`Menu (P1)` / `Function` /
  `Command Parameter (P5)` / `0 … 10 ~`) is reprinted at the top of PDF pages 9 and 10. This is
  a normal continued-table header; it is *not* counted as a chart row in `row_count`.
- **Row 077 `PSQ control signal output condition`**: the cells `BSY-SND` (header 4) and
  `SQL-SND` (header 5) are set in a visibly smaller/condensed face than every other cell in the
  table, to fit the column width. Legible; recorded as a typographic irregularity only.
- **Row 001 `Back light color`** offers only two options (headers 0 and 1: `1`, `2`). Short, but
  ruled and unambiguous — not a defect.
- **Inconsistent intra-cell spacing** in Functions 081–086 (`Mic PF 1` vs `Mic  PF 2`), see
  above.
- No cell was found straddling two grid columns other than the two deliberate full-width prose
  blocks (079–086 and 087). No non-monotonic option list was found: every numeric row ascends
  left to right. No unreadable cell.

**STOP findings: none.**
