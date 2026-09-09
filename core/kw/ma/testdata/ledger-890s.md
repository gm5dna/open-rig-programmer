# Boundary ledger — TS-890S "EX Command Parameter Lists"

**Date:** 08/09/2026

## Source

- **Document title as printed (cover, PDF p.1):** "TS-890S — PC CONTROL COMMAND Reference Guide", JVCKENWOOD Corporation.
- **Revision/date as printed:** the cover prints the date **January/30/2019**. No revision number is printed anywhere on the cover or on the chart pages; the running head on every chart page reads only "PC CONTROL COMMAND REFERENCE GUIDE".
- **File read:** `docs/fixtures-private/manuals/ts890s_pc_rev1.pdf` (77 pages). The "rev1" in the filename is *not* printed in the document.

## Pages used

The chart runs under the heading **"EX Command Parameter Lists"** (printed once, at the top of PDF p.26) and ends where the heading changes to "PF Key Assignment Lists" at the top of PDF p.31.

| PDF page | Printed folio |
|---|---|
| 26 | – 25 – |
| 27 | – 26 – |
| 28 | – 27 – |
| 29 | – 28 – |
| 30 | – 29 – |

PDF p.25 was checked and is blank below its folio; PDF p.31 was checked and carries the next heading. Neither is part of the chart.

## Method

- Worked from pre-rendered page images at **300 dpi** (2481 × 3508 px, A4) — one PNG per PDF page.
- **Pass 1:** whole-page view of each chart page to establish the row structure, then left-hand crops (P1/P2/P3 + Function, x ≈ 170–1120 px) in three bands per page, enlarged **200 %** with `magick` — an effective **600 dpi**, at which 0/8/6 and l/1/I are unambiguous (the digit body is ~60 px tall on screen).
- **Pass 2:** independent re-read of the same address columns using **different band boundaries** (offsets 700/1750/2750 instead of 300/1300/2300), enlarged **250 %** (effective 750 dpi), so no page seam fell in the same place twice.
- **Grid pass:** for the last-populated column, a composite strip was built for each page by cropping the P3 column and the two right-hand columns (`005`, `006 ~`) and appending them side by side, enlarged **170 %**, so every row's rightmost filled cell could be read against its own P3 value.
- Targeted spot crops at **400 %** were used for the em-dash cell, the trailing-colon Function text and the blank-`000` rows.
- Ruled rows were followed continuously across the four page seams (p.26→27 inside P2 = 02; p.27→28 inside P2 = 05; p.28→29 inside P2 = 08; p.29→30 at the P1 change).
- **Reconciliation:** passes 1 and 2 agreed on every address and on every last-populated column; **no discrepancy arose, so no third look was needed to settle one.** Where a band seam hid a single row in pass 2 (P3 = 21 on p.26; P2 = 05 / P3 = 00 on p.27), that row was read from the overlapping pass-1 band rather than inferred.
- Tools run: `magick` (crop/resize/append) and the image viewer only. No text-layer extraction, no other file, no search, no web access. **Nothing outside this PDF and its renders was consulted, and nothing remembered about Kenwood radios was used.**

## Column headers exactly as printed

Top merged band: **`Menu`** on PDF pp.26–29, **`Advanced Menu`** on PDF p.30.

Second band: **`P1`** | **`P2`** | **`P3`** | **`Function`** | **`P5`** (the `P5` cell spans the whole grid).

Third band (the P5 sub-headers, left to right):

`000` `001` `002` `003` `004` `005` `006 ~`

The header band is repeated at the top of every chart page. `P1`, `P2`, `P3` and `Function` are printed once, vertically merged across the second and third bands.

## Where P1 changes, and the two lists

- P1 = **0** for every row on PDF pp.26–29. The merged title band over those pages reads **`Menu`**.
- P1 = **1** from the **first body row of PDF p.30** (`1 00 00`, "Indication Signal Type (External Meter 1)") to the end of the chart. The merged title band on that page reads **`Advanced Menu`**.
- The change falls exactly on a page break: p.29 ends at `0 09 03` and p.30 opens at `1 00 00`. Both lists are titled, by those merged band cells.

## Row totals (addressed rows only)

| P1 value | List title | Rows |
|---|---|---|
| 0 | Menu | **135** |
| 1 | Advanced Menu | **27** |
| — | **Overall** | **162** |

Per page: 26 → 49, 27 → 37, 28 → 44, 29 → 5, 30 → 27. Sum 162; the P1 = 0 pages sum to 135.

The em-dash-addressed row on p.30 is excluded from all of the above.

## Lowest and highest address; contiguity

**List 1 — Menu (P1 = 0)**

- Lowest: `0 00 00`. Highest: `0 09 03`.
- P2 runs 00, 01, 02, 03, 04, 05, 06, 07, 08, 09 with no missing and no repeated value.
- Within each P2 group the P3 sequence starts at 00 and increments by 1 with **no gap and no duplicate**:
  - 00 → 00–32 (33 rows) · 01 → 00–06 (7) · 02 → 00–17 (18) · 03 → 00–13 (14) · 04 → 00–05 (6) · 05 → 00–16 (17) · 06 → 00–15 (16) · 07 → 00–11 (12) · 08 → 00–07 (8) · 09 → 00–03 (4).
- Groups that straddle a page seam were followed across it: 02 (…08 on p.26 / 09… on p.27), 05 (…07 on p.27 / 08… on p.28), 08 (…06 on p.28 / 07 on p.29). All three continue without a break.
- **Sequence is contiguous. No gaps, no duplicates.**

**List 2 — Advanced Menu (P1 = 1)**

- Lowest: `1 00 00`. Highest addressed: `1 00 26`.
- P2 is 00 for every addressed row. P3 runs 00–26 with no gap and no duplicate.
- The one em-dash row continues the printed P3 numbering at 27, immediately after 26.
- **Sequence is contiguous. No gaps, no duplicates.**

**STOP FINDINGS (gaps/duplicates in the address sequences): none.**

## Dash-addressed rows (recorded, not counted)

| Address as printed | Function | Grid |
|---|---|---|
| `1` `—` `27` | Firmware Version | one merged prose cell: "Reading command only" |

That is the **only** row in the whole chart whose address cell prints a dash instead of digits. The dash is an **em dash** in the P2 cell; P1 and P3 print digits.

## "Does not correspond to a command" rows (addressed — counted, and recorded here)

All four are on PDF p.30 (folio 29), all with digits in P1/P2/P3, and each prints the single merged cell "Does not correspond to a command" across the whole grid:

| Address | Function |
|---|---|
| `1 00 23` | Touchscreen Calibration |
| `1 00 24` | Software License Agreement |
| `1 00 25` | Important Notices concerning Free Open Source |
| `1 00 26` | About Various Software License Agreements |

## TEXT rows (prose printed across the grid in place of cells)

| Address | Function | Character count the prose states |
|---|---|---|
| `0 00 05` | Screen Saver Message | "Up to 10 alphanumeric characters" — **10** |
| `0 00 06` | Power-on Message | "Up to 15 alphanumeric characters" — **15** |
| `0 05 12` | Contest Number | "0001 ~ 9999 (Must be a 4-digit number)" — **4** digits |
| `1 00 05` | Reference Oscillator Calibration | "Parameter value of 0000 ~ 1000, corresponding to setting values of -500 ~ +500" — **no character count is stated**; the parameter is printed as a four-digit value |

## Prose-cell rows (NOT text rows — recorded per the brief)

- `0 00 15` … `0 00 31` — **17 rows**, each printing the shared cell "Refer to the list of function allotment numbers for the PF key" across the grid: PF A/B/C Key Assignment (15–17), External PF 1–8 (18–25), Microphone PF 1–4 (26–29), Microphone DOWN (30), Microphone UP (31).
- `1 00 23` … `1 00 26` — 4 rows, "Does not correspond to a command" (listed above).
- `1` `—` `27` — "Reading command only" (listed above).

## Rows whose grid reaches a header of more than three characters, or an "Over"/"~" column

The only header longer than three characters is the last one, **`006 ~`** (five characters, and the only header carrying a tilde). No column is headed "Over". Twenty-eight rows put content in it:

| PDF page | Address | Function | Header | Cell as printed |
|---|---|---|---|---|
| 26 | `0 00 13` | Long Press Duration of Panel Keys | `006 ~` | Up to 2000 [ms] (in steps of 100) |
| 26 | `0 01 00` | Beep Volume | `006 ~` | Up to 20 |
| 26 | `0 01 01` | Voice Message Volume (Play) | `006 ~` | Up to 20 |
| 26 | `0 01 02` | Sidetone Volume | `006 ~` | Up to 20 |
| 26 | `0 01 03` | Voice Guidance Volume | `006 ~` | Up to 20 |
| 26 | `0 02 00` | FFT Scope Averaging (RTTY Decode) | `006 ~` | Up to 9 |
| 27 | `0 02 10` | FFT Scope Averaging (PSK Decode) | `006 ~` | Up to 9 |
| 27 | `0 03 03` | FM Mode Frequency Step Size (Multi/Channel Control) | `006 ~` | 006: 25 [kHz] / 007: 30 [kHz] / 008: 50 [kHz] / 009: 100 [kHz] |
| 27 | `0 03 04` | AM Mode Frequency Step Size (Multi/Channel Control) | `006 ~` | 006: 25 [kHz] / 007: 30 [kHz] / 008: 50 [kHz] / 009: 100 [kHz] |
| 27 | `0 03 08` | Tuning Speed Control | `006 ~` | Up to 10 |
| 27 | `0 03 09` | Tuning Speed Control Sensitivity | `006 ~` | Up to 10 |
| 28 | `0 05 08` | CW Keying Weight Ratio | `006 ~` | Up to 4.0 (in steps of 0.1) |
| 28 | `0 05 14` | Channel Number (Countup Message) | `006 ~` | 006: Channel 6 / 007: Channel 7 / 008: Channel 8 |
| 28 | `0 05 16` | CW/ Voice Message Retransmit Interval Time | `006 ~` | Up to 60 [s] |
| 28 | `0 06 07` | TX Filter High Cut (SSB/AM) | `006 ~` | 006: 3500 [Hz] / 007: 4000 [Hz] |
| 28 | `0 06 09` | TX Filter High Cut (SSB-DATA/AM-DATA) | `006 ~` | 006: 3500 [Hz] / 007: 4000 [Hz] |
| 28 | `0 07 06` | USB: Audio Input Level | `006 ~` | Up to 100 |
| 28 | `0 07 07` | ACC 2: Audio Input Level | `006 ~` | Up to 100 |
| 28 | `0 07 08` | USB: Audio Output Level | `006 ~` | Up to 100 |
| 28 | `0 07 09` | ACC 2: Audio Output Level | `006 ~` | Up to 100 |
| 28 | `0 08 04` | Waterfall Gradation Level | `006 ~` | Up to 10 |
| 28 | `0 08 05` | Tuning Assist Line (SSB Mode) | `006 ~` | 006: 800 [Hz] / 007: 1000 [Hz] / 008: 1500 [Hz] / 009: 2210 [Hz] |
| 29 | `0 09 03` | Repeat Speed (USB Keyboard) | `006 ~` | Up to 32 |
| 30 | `1 00 00` | Indication Signal Type (External Meter 1) | `006 ~` | 006: SWR |
| 30 | `1 00 01` | Indication Signal Type (External Meter 2) | `006 ~` | 006: SWR |
| 30 | `1 00 02` | Output Level (External Meter 1) | `006 ~` | Up to 100 [%] |
| 30 | `1 00 03` | Output Level (External Meter 2) | `006 ~` | Up to 100 [%] |
| 30 | `1 00 10` | Microphone Gain (FM Mode) | `006 ~` | Up to 100 |

## Printing defects and anomalies (recorded, not resolved)

1. **PDF p.27 (folio 26), `0 03 07`** — the Function text prints as "**Tuning Control :**", a trailing colon with nothing printed after it. Verified at 400 % enlargement; the cell is otherwise clean and the colon is not a stray mark.
2. **PDF p.27, `0 03 08` Tuning Speed Control — non-monotonic option list.** The `000` cell is blank; `001` reads "Off"; `002`–`005` read 2, 3, 4, 5; `006 ~` reads "Up to 10". The value **1 never appears**, so the numeric run jumps 001→"Off", 002→2. Its sibling row `0 03 09` (Tuning Speed Control Sensitivity) prints the ordinary 1, 2, 3, 4, 5 under `001`–`005`.
3. **Blank `000` cell with the option list starting at `001`** (six rows): `0 00 08` Meter Response Speed (Analog) — 1–4 under 001–004; `0 01 04` Voice Guidance Speed — 1–4 under 001–004; `0 03 08` (see 2); `0 03 09` — 1–5 under 001–005; `0 08 04` Waterfall Gradation Level — 1–5 under 001–005; `0 09 02` Repeat Delay Time (USB Keyboard) — 1–4 under 001–004.
4. **PDF p.29 (folio 28), `0 09 03` Repeat Speed (USB Keyboard) — hole in the middle of a filled run.** `000`–`004` read 1, 2, 3, 4, 5; **`005` is blank**; `006 ~` reads "Up to 32". Every comparable row on p.26/p.28 fills `005` before reaching `006 ~`.
5. **PDF p.26 (folio 25), `0 01 02` Sidetone Volume — option list offset by one column** relative to its neighbours: `000` = "Linked with Monitor Control", `001` = Off, `002` = 1, `003` = 2, `004` = 3, `005` = 4, `006 ~` = "Up to 20". The adjacent volume rows (`0 01 00`, `0 01 01`, `0 01 03`) print 5 in `005` for the same "Up to 20" ceiling.
6. **PDF p.30 (folio 29) — unanchored footnote.** Below the table a footnote reads "◆ P2 is any value." No ◆ symbol is printed against any row, column header or cell inside the table, so the footnote's referent is not marked on the page.

No duplicated header band, no cell straddling two columns, and no unreadable cell was found on any of the five pages.
