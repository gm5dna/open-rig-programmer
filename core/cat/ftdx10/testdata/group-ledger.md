# Group-boundary ledger — FTDX10 "Table 2 (MENU Chart)"

**Date of transcription:** 29/07/2026

## Source

- **Document:** *FTDX10 CAT Operation Reference Manual*, YAESU MUSEN CO., LTD.
- **Revision as printed on the document:** `2308-F`, printed at the foot of the back cover (PDF page 25), alongside "Copyright 2023 YAESU MUSEN CO., LTD."
- **File:** `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ftdx10_cat_2308-F.pdf` (25 pages, A4).

## Method

Every value in `group-ledger.csv` was read **visually** from the rendered pages of that one PDF and from nothing else. No source code, no other document, no directory listing, and no text-layer extraction was consulted — the PDF is in any case flagged `copy:no`, so the rendered image is the only evidence used.

Pages were rasterised with `pdftoppm` at 400 dpi and then cropped/enlarged with ImageMagick so that the printed rules and the small chart type could be resolved. Three passes were made:

1. Whole-page thumbnails at 60 dpi to locate the chart.
2. Left-hand column bands (P1 / P2 / P3 / Function) at 400 dpi enlarged 200 per cent, read top to bottom, to transcribe every P3 number and Function name.
3. A full-height crop of the P1 and P2 columns only, for each chart page, so the ruled cell boundaries could be followed continuously from the top of the table to the bottom without a page-internal seam. Group membership was decided from those printed rules, not from whitespace.

Targeted 250–300 per cent enlargements were then taken of every subgroup's first and last row, of each P2 label cell, and of the table header, to confirm punctuation and case.

## Pages used

- **PDF pages 11, 12 and 13** carry "Table 2 (MENU Chart)". These bear the printed folio numbers **10, 11 and 12** respectively (the PDF page number runs one ahead of the printed page number throughout this document).
- **PDF page 10** (printed folio 9) was also viewed, for the `EX` / MENU command block that points at Table 2.
- **PDF pages 1 and 25** were viewed for the title and the revision code.
- PDF page 14 (printed 13) was checked and confirmed to be the resumption of the Control Command Tables (`FA`, `FB`, `FN`, …), i.e. the chart does not continue past PDF page 13.

The table header is repeated at the top of each of the three chart pages: `P1 | P2 | P3 | Function | P4 | Digits`, under a full-width `Table 2 (MENU Chart)` banner.

## Result

18 (P1, P2) subgroups, **197 chart rows in total**, distributed as:

| P1 group | subgroups | rows |
|---|---|---|
| 01 (RADIO SETTING) | 7 | 99 |
| 02 (CW SETTING) | 3 | 30 |
| 03 (OPERATION SETTING) | 5 | 57 |
| 04 (DISPLAY SETTING) | 3 | 11 |
| **Total** | **18** | **197** |

P3 numbering restarts at `01` in every subgroup and is contiguous to the subgroup's last row in all 18 cases — no gaps and no repeats were seen.

No subgroup straddles a page break. PDF page 11 closes with a full-width bottom rule under `01 / 04 (MODE PSK/DATA)` row 18; page 12 closes under `03 / 01 (GENERAL)` row 20; each following page opens a fresh cell for the continuing P1 group. Because of that, the P1 labels `01 (RADIO SETTING)` and `03 (OPERATION SETTING)` are each printed **twice**, once in the cell on the page where the group starts and once in a fresh cell on the page where it continues.

## STOP findings

**None.** Every row in the chart is enclosed by unbroken printed rules on all four sides, and every P1 and P2 label sits inside a cell whose top and bottom rules are visible and complete. There was no place where a grid line was missing or where a label's placement could not be resolved, so no rows have been excluded from the counts and no group assignment has been guessed.

## Observed disagreements (recorded, not resolved)

These are described exactly as they appear; no attempt has been made to decide which side is correct.

1. **`EX` header says P1 runs 01–05; the chart shows only 01–04.** The `EX` / MENU command block on PDF page 10 prints `P1 : 01 - 05`. Table 2 contains four P1 groups only — `01 (RADIO SETTING)`, `02 (CW SETTING)`, `03 (OPERATION SETTING)`, `04 (DISPLAY SETTING)`. No P1 group numbered `05` appears anywhere on PDF pages 11–13.

2. **`EX` header says P3 runs 01–23; the largest P3 printed in the chart is 21.** The same block prints `P3 : 01 - 23`. The deepest subgroup in the chart is `01 / 03 (MODE FM)`, which ends at P3 `21` (`ENC/DEC`); next deepest is `03 / 01 (GENERAL)` at P3 `20`. No row numbered 22 or 23 is printed.

3. The same block prints `P2 : 01 - 07`, and the chart's highest P2 is indeed `07 (ENC/DEC RTTY)` under P1 `01`.

4. **Duplicate index in a P4 list.** PDF page 12, `01 / 05 (MODE RTTY)`, P3 `16 SHIFT FREQUENCY`: the P4 cell reads `1: 170 Hz    1: 200 Hz    2: 425 Hz    3: 850 Hz` — index `1` is printed twice and no index `0` is offered. The Digits column for that row reads `1`.

5. **P4 list starting at 1.** Same subgroup, P3 `15 MARK FREQUENCY`: the P4 cell reads `1: 1275 Hz    2: 2125 Hz` — no index `0`.

6. **Non-language entry in a language list.** PDF page 12, `03 / 01 (GENERAL)`, P3 `20 KEYBOARD LANGUAGE`: the P4 cell lists `00: JAPANESE` through `10: ITALIAN` and then `11: LEVEL`.

7. **Inconsistent spelling within the chart.** The low-cut slope row is printed `LCUT SLOP` (no trailing E) in every mode subgroup that carries it, whilst the corresponding high-cut row in the same subgroups is printed `HCUT SLOPE`.

8. **Out-of-order P4 values.** PDF page 12, `01 / 06 (ENC/DEC PSK)`, P3 `02 DECODE AFC RANGE`: the P4 cell reads `0: 8    1: 1.5    2: 30 Hz`.

9. **Apparent typo in a P4 value.** PDF page 12, `02 / 01 (MODE CW)`, P3 `17 QSK DELAY TIME`: the P4 cell reads `… 2: 25 mesc    3: 30 msec`.

10. **Justified Function name.** PDF page 13, `04 / 01 (DISPLAY)`, P3 `05`: the Function cell is set with wide inter-letter spacing and wraps over two lines — `M O U S E   P O I N T E R` / `SPEED`. It has been transcribed as `MOUSE POINTER SPEED`.

11. **Trailing full stops.** `MY CALL.` (page 13, `04 / 01 (DISPLAY)` P3 01), `MAIN STEPS PER REV.` and `MPVD STEPS PER REV.` (page 13, `03 / 05 (TUNING)` P3 06 and 07) are printed with a trailing full stop; these have been kept verbatim in the CSV.
