# Group-boundary ledger — FT-991A CAT manual menu chart

Date: 05/09/2026

## Document

- Title as printed on the front cover (PDF page 1): `HF/VHF/UHF ALL MODE TRANSCEIVER` /
  `FT-991A` / `CAT Operation Reference Manual` (the "A" of FT-991A is printed as a white
  letter A in a solid black square).
- Running footer on every chart page prints the document's short title as
  `FT-991A CAT Operation Reference Book` — again with the boxed A. Note that the cover says
  *Manual* and the footer says *Book*; both are recorded as printed, neither is resolved here.
- Revision code: `1711-D`, printed at the foot of the right-hand column of the **back cover**
  (PDF page 20), below the YAESU UK address block. The same page carries
  `Copyright 2017 / YAESU MUSEN CO., LTD. / All rights reserved.`

## Pages used

| PDF page | Printed folio | Content used |
|---|---|---|
| 1 | (none) | front cover — document title |
| 8 | 7 | `EX` `MENU` command block, then chart rows 001–042 |
| 9 | 8 | chart rows 043–124 |
| 10 | 9 | chart rows 125–153, then the unrelated `FA` block |
| 20 | (none) | back cover — revision code `1711-D` |

PDF page 7 (folio 6) was opened only far enough to confirm it carries no part of the menu
chart: it ends with the `DT` `DATE AND TIME` command block, and the chart's first ruled row
(`001`) is on PDF page 8. The chart's other end is closed on PDF page 10 itself, where the
row `153` rule is followed by white space and then the unrelated `FA` `FREQUENCY VFO-A`
command block, so no page beyond PDF page 10 was needed.

## Method

- Located the chart on the supplied 300 dpi renders (`p-01.png` … `p-20.png`), downscaled to
  850 px wide for orientation only.
- Re-rendered PDF pages 8–10 from the PDF at **600 dpi** with
  `pdftoppm -r 600 -f 8 -l 10 -png` (4961 × 7016 px per page). Every reading below was taken
  from those 600 dpi renders.
- **Pass 1 (menu numbers + Function text).** Cropped the `P1` + `Function` columns only
  (x = 405, width 1250 px) in vertical chunks and resized to 1000 px wide — an enlargement of
  0.80 × of the 600 dpi render, i.e. an effective ~480 dpi with glyph heights of ~28 px. At
  that size `0`/`8`/`6` and `1`/`l`/`I` are unambiguous (the `0`s in `001`–`009` are visibly
  open ovals, the `1`s bare vertical strokes with a short flag and no foot serif).
- **Pass 2 (independent re-read of the numbers, plus the Digits column).** Cropped the `P1`
  column (x = 405, w 260) and the `Digits` column (x = 4285, w 240) separately and
  `+append`ed them into a single strip, then resized to 460 px wide (≈0.90 × of 600 dpi).
  This re-read every menu number against a different crop geometry and paired each number
  with its Digits cell without relying on eye-tracking across the full page width.
- **Pass 3 (full-width legends).** Cropped x = 405, width 3900 (P1 + Function + P2) in
  vertical chunks and resized to 1900 px (0.487 × of 600 dpi, ≈290 dpi) to read the P2
  parameter legends; individual suspect rows were then re-cropped at 0.73 × of 600 dpi
  (single-row strips 1900 px wide) for confirmation.
- **Pass 4 (ruling).** Read a one-pixel-wide vertical column of each rendered page at x = 600
  (inside the `P1` cell, clear of all glyphs) and listed every dark run, giving the exact
  y-position and thickness of every horizontal rule the chart crosses. This counted the
  ruled cells independently of any reading of the numbers, and measured whether any rule is
  thicker than its neighbours.
- Reconciliation: passes 1, 2 and 4 agree exactly on the row counts for all three pages
  (42 / 82 / 29). **No discrepancy arose between the passes**, so no fourth look was needed to
  settle one. Pass 4's rule pitch is a constant 73.5 px on all three pages, which is what
  confirms that no row was missed behind a wrapped cell.

**Nothing else was consulted.** No text layer was extracted from the PDF (no `pdftotext`, no
`pdfinfo`, no copy-paste of PDF text); no other file in the repository or elsewhere was
opened; no filesystem search was run; no web access was made. Every value below was read off
the rendered images of the printed ruled table. Nothing remembered about other Yaesu radios'
menus was used.

## Grouped or flat?

**ONE FLAT SEQUENCE.** The evidence, from the print alone:

1. The first column prints a plain three-digit number on every row, `001` through `153`,
   increasing by exactly one at every rule, across all three pages and across both page
   seams. There is no separator, dot, dash or prefix inside any of the numbers.
2. There is no group-heading row anywhere in the body of the chart: pass 4 found the rule
   pitch constant at 73.5 px from the first data row on PDF page 8 to the last on PDF page 10,
   with no taller cell and no shaded cell other than the repeated column header.
3. There is no thicker rule anywhere. Measured rule thicknesses at 600 dpi:
   PDF page 8 — 44 rules, every one 5 or 6 px; PDF page 9 — 84 rules, every one 5 or 6 px;
   PDF page 10 — 31 rules, every one 5 or 6 px. The 5-versus-6 px variation is
   rasterisation of a single constant hairline, and it falls on no pattern that tracks the
   numbering.
4. The `EX` `MENU` command block immediately above the chart on PDF page 8 (folio 7) prints
   `P1  : 001 - 153 (MENU Number)` and `P2  : Parameter (See Table)`, i.e. one contiguous
   number space with no group structure. (Recorded as corroboration only; the CSV is built
   from the chart.)

Because the chart is flat, the CSV carries **one row per PDF page the chart occupies**, and
its `p1` field is the PDF page number.

## The chart's column headers

Printed exactly, left to right, in a grey-shaded header cell:

```
P1        Function        P2        Digits
```

`P1` and `Digits` are centred; `Function` and `P2` are centred over wide columns. The header
is repeated at the top of each of PDF pages 8, 9 and 10 and has **not** been counted as a
chart row on any of them.

**The chart prints NO group-label or sub-group-label column.** There are exactly four
columns and no fifth; there is no heading row inside the body; there is no marginal label,
tab or bracket beside the table; and no group name is printed anywhere in or beside the
chart. No group name has been inferred from anywhere else.

## Totals

- Total row count (sum of `row_count`): **153** (42 + 82 + 29).
- Number of groups: **not applicable — the chart is flat.** The CSV therefore has **3 rows**,
  one per PDF page (8, 9, 10).

## Digits column

Set of distinct values printed in the `Digits` column, across all 153 rows:

```
1, 2, 3, 4, 5, 8, and a single hyphen "-"
```

The hyphen is the printed content of exactly one cell (menu 087) and is recorded as printed,
not interpreted.

Every row whose `Digits` value is greater than 4:

| Menu number | Function (verbatim) | Digits | PDF page (folio) |
|---|---|---|---|
| 027 | `TIME ZONE` | 5 | 8 (7) |
| 064 | `OTHER DISP (SSB)` | 5 | 9 (8) |
| 065 | `OTHER SHIFT (SSB)` | 5 | 9 (8) |
| 083 | `RPT SHIFT 430MHz` | 5 | 9 (8) |
| 151 | `PRESET FREQUENCY` | 8 | 10 (9) |

(Menu 087 `RADIO ID` is listed under defects below; its Digits cell prints `-`, which is not
a number and therefore is not "greater than 4".)

## Free-text parameters

**None.** No row's P2 legend uses the words "characters", "character", "ASCII", "text
string", "call sign", "callsign", or shows a call sign as an example value. The three
nearest things, recorded so the reader can see they were checked and rejected:

- 018–022 `CW MEMORY 1` … `CW MEMORY 5` print `0: TEXT    1: MESSAGE`. `TEXT` here is one of
  two numbered option labels, not a description of a free-text parameter; the legend gives no
  character count, no character set and no example.
- 005 `MY CALL INDICATION` names a call in the Function column but its P2 legend is
  `0 ~ 5 sec` — a duration, Digits 1.
- 087 `RADIO ID` prints its P2 cell as ten hyphens and its Digits cell as one hyphen; it
  supplies no parameter legend at all, free text or otherwise.

## STOP findings

**None.**

- No row's menu number fails to increase down the chart. The sequence runs 001, 002, … 153
  with a step of exactly 1 at every rule, including across both page seams (042 → 043 at the
  PDF page 8/9 seam, 124 → 125 at the PDF page 9/10 seam).
- No menu number is duplicated. 153 rules-bounded rows carry 153 distinct numbers.
- No row's number prefix disagrees with the ruled grouping, because the numbers carry no
  prefix and the ruling defines no groups (see "Grouped or flat?" above).
- No place in the ruling is ambiguous. Pass 4 measured every horizontal rule the chart
  crosses on all three pages: 44 + 84 + 31 = 159 rules bounding 43 + 83 + 30 = 156 cells,
  which is 3 repeated header cells plus 153 data rows. Every rule is 5–6 px at 600 dpi and
  the pitch is a constant 73.5 px, so no rule is missing, doubled, heavier or lighter, and no
  cell is taller than one row. No chart row wraps its parameter text onto a second line.

## Printing defects noticed (recorded, not resolved)

Each was confirmed on a single-row crop at 0.73 × of the 600 dpi render.

1. **Menu 100 `RTTY SHIFT FREQ`, PDF page 9 (folio 8) — a legend with two identical index
   numbers.** Printed: `1: 170 Hz    1: 200 Hz    2: 425 Hz    3: 850 Hz`. The index `1`
   appears twice and there is no index `0`. Crop: `work/d100.png`.
2. **Menu 028 `GPS/232C SELECT`, PDF page 8 (folio 7) — a gap in the option list.** Printed:
   `0: GPS1    1: GPS2    3: RS232C`. Index `2` is not printed. Crop: `work/d028b.png`.
3. **Menu 101 `RTTY MARK FREQ`, PDF page 9 (folio 8) — option list starts at 1.** Printed:
   `1: 1275 Hz    2: 2125 Hz`; no index `0`.
4. **Menus 072 `DATA PORT SELECT` and 077 `FM PKT PORT SELECT`, PDF page 9 (folio 8) —
   option lists start at 1 where the parallel rows start at 0.** Both print
   `1: DATA    2: USB`, whereas 048 `AM PORT SELECT` and 109 `SSB PORT SELECT` print
   `0: DATA    1: USB`. Crop: `work/d072.png`.
5. **Menu 116 `SCP SPAN FREQ`, PDF page 9 (folio 8) — option list starts at 03.** Printed:
   `03: 50 kHz    04: 100 kHz    05: 200 kHz    06: 500 kHz    07: 1000 kHz`; indices `00`,
   `01` and `02` are not printed.
6. **Menu 061 `QSK DELAY TIME`, PDF page 9 (folio 8) — misspelling.** Printed:
   `0: 15 msec    1: 20 msec    2: 25 mesc    3: 30 msec`. The third entry reads `mesc`; the
   other three read `msec`. Crop: `work/d061.png`.
7. **Menu 069 `DATA HCUT SLOPE`, PDF page 9 (folio 8) — Digits disagrees with its own
   legend and with every identical sibling.** Its P2 legend is `0: 6 dB/oct    1: 18 dB/oct`
   and its Digits cell prints `2`. Every other row printing that identical legend prints
   Digits `1`: 042, 044, 051, 053, 067, 093, 095, 103, 105. Confirmed on a dedicated crop
   (`work/q69.png`) that pairs the P1 and Digits cells for rows 066–071.
8. **Menu 087 `RADIO ID`, PDF page 9 (folio 8) — the only non-numeric Digits cell.** P2 is
   printed as ten spaced hyphens `- - - - - - - - - -` and Digits as a single hyphen `-`.
   Confirmed on `work/q87.png`.
9. **Menu 027 `TIME ZONE`, PDF page 8 (folio 7) — a range with no P2 encoding.** Printed:
   `UTC -12:00 ~ +14:00`, Digits `5`. Unlike its neighbours the legend gives no
   `(P2 = …)` form, and the literal printed endpoint `-12:00` is six characters against a
   Digits value of 5. Recorded as printed; not resolved. Crop: `work/d028.png` (which
   landed on row 027 and shows it in full).
10. **Menu 147 `DATA VOX DELAY`, PDF page 10 (folio 9) — a range with no step.** Printed:
    `30 ~ 3000 msec (P2 = 0030 ~ 3000)`. The parallel row 144 `VOX DELAY` prints
    `30 ~ 3000 msec (P2 = 0030 ~ 3000, 10 msec/step)`.
11. **Inconsistent spacing before the colon in the `00: OFF` legends.** Menus 119, 125, 128
    and 134 print `00 : OFF` (space before the colon); menus 122 and 131 print `00: OFF`.
12. **Inconsistent spacing before the unit in the LCUT/HCUT legends.** Menus 092 and 094
    print `19: 1000Hz` and `67: 4000Hz` with no space, whereas the identically-structured
    rows 041, 043, 050, 052, 066, 068, 102 and 104 print `19: 1000 Hz` and `67: 4000 Hz`.
13. **Inconsistent spacing around the `P2 =` in the bracketed encodings.** Menus 001–003
    print `(P2= 0020 ~ 4000, 20 msec/step)` and menus 035, 036 print `(P2= …)`, whereas
    menus 010, 011, 015, 025, 026, 046, 057 and others print `(P2 = …)` with a space.

## Scratch crops

All working crops are under
`/private/tmp/claude-501/-Users-stuart-coding-ft710-programmer/5cb2182d-0a2a-4696-ac2b-c1e34bfcbd2a/scratchpad/quarantine/991-L/work/`
(`hi-08.png`–`hi-10.png` are the 600 dpi page renders; `p8L-*`, `p9L-*`, `p10L-*` are the
pass-1 left-column crops; `pd8-*`, `pd9-*`, `pd10-*` are the pass-2 P1+Digits strips;
`w8-*`, `w9-*`, `w10-*` are the pass-3 full-width crops; `d*.png`, `q*.png` are the
single-row confirmation crops).
