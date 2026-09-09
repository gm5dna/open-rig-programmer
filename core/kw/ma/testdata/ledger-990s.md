# L-990 — boundary ledger for the TS-990S EX Command Parameter Lists

Date: 08/09/2026

## Source

- Document title as printed on the cover (PDF page 1): **TS-990S / PC CONTROL COMMAND / Reference Guide**.
- Publisher and date as printed on the cover: **JVCKENWOOD Corporation**, **January/30/2019**. No revision number is
  printed anywhere on the cover; the file name carries "rev2" but the printed page does not, so no printed revision is
  recorded.
- Running head on every chart page: **PC CONTROL COMMAND REFERENCE GUIDE**.

## Pages used

The chart printed under the heading **EX Command Parameter Lists** occupies six pages:

| PDF page | printed folio |
|---|---|
| 23 | – 22 – |
| 24 | – 23 – |
| 25 | – 24 – |
| 26 | – 25 – |
| 27 | – 26 – |
| 28 | – 27 – |

The heading "EX Command Parameter Lists" is printed once, above the table on PDF page 23. PDF page 22 (folio – 21 –) does
not carry the chart. PDF page 29 (folio – 28 –) opens a different heading, **PF Key Assignment Lists**, so the chart ends
with PDF page 28; the ruled table on that page closes with a heavy bottom rule and roughly half the page is blank below
it.

## Method

- Pages were read from pre-rendered 300 dpi PNG page images (2481 × 3508 px, A4). No text layer was extracted; no
  `pdftotext`, `pdfinfo` or copy-paste was used at any point.
- **Pass 1 (whole page).** Each of the six pages was viewed as a whole 300 dpi render (presented at 1414 × 2000, roughly
  0.57× of native) to establish the row order, the banner titles, the column headers and the grid contents.
- **Pass 2 (address columns).** Each page was cropped with `magick` into three 1020 × 1010 px tiles covering the P1, P2,
  P3 and Function columns (source x = 150–1170, y = 300–3310) and each tile was read at approximately 1:1 with the
  300 dpi render — i.e. an effective enlargement of about 3.4× over pass 1 for the same glyphs. At that size the digit
  pairs 0/8/6 and the strokes 1/l/I are unambiguous: the 0 is a plain open oval, the 8 has a closed upper bowl, the 6 has
  a closed lower bowl only, and the 1 carries the Helvetica-style flag with no serifs.
- **Pass 2b (last-populated grid column).** For the same three vertical bands of every page a composite crop was built by
  appending the address columns (x = 175, w = 365) to the 005 and 006 ~ columns (x = 1965, w = 350), so that every cell
  in the rightmost columns could be attributed to a specific address without relying on row-height arithmetic.
- **Third looks.** Two items were re-cropped at 300 % enlargement of the 300 dpi render (effectively ~900 dpi of
  apparent detail): the merged address cell at 0 03 01 on PDF page 24, and the dash-addressed rows at the foot of PDF
  page 28. Both are described under STOP findings below.
- **Reconciliation.** The per-page row counts from pass 1 were reconciled against a re-derivation from the address
  sequences read in pass 2 (see "Totals" below). The two agreed exactly, 167 rows for P1 = 0 and 27 for P1 = 1; no
  discrepancy arose that needed a third look to settle. The only judgement call in the counts is the merged 0 03 01
  cell, resolved by the brief's shape rule and recorded below.
- Nothing outside the PDF at
  `docs/fixtures-private/manuals/ts990s_pc_rev2.pdf` and its rendered pages was consulted: no other file in the
  repository, no directory listing, no web access, no prior knowledge of Kenwood menus or of any other Kenwood document.
  Only image cropping/rendering commands and writes of these two output files were run.

## Column headers, exactly as printed

The table has a two-deck header. The top deck spans the whole table with a single banner giving the list name. Below it:

```
P1 | P2 | P3 | Function |                    P5
                        | 000 | 001 | 002 | 003 | 004 | 005 | 006 ~
```

`P5` is printed once as a spanning header over seven sub-headers: `000`, `001`, `002`, `003`, `004`, `005`, `006 ~`.
The last sub-header is printed as the three digits `006` followed by a space and a swung dash `~`. The header block is
repeated in full at the top of every one of the six pages.

## The two lists

The book prints the list name in the banner above the P1/P2/P3 header row:

- **Menu** — banner on PDF pages 23, 24, 25, 26 and 27. All its rows print `0` in the P1 cell.
- **Advanced Menu** — banner on PDF page 28. All its addressed rows print `1` in the P1 cell.

The P1 value changes at a page boundary, not mid-page: the Menu list's last addressed row is `0 09 03`
(Repeat Speed (USB Keyboard)), the last row on PDF page 27; the Advanced Menu list's first addressed row is `1 00 00`
(Indication Signal Type (Main Band)), the first row on PDF page 28. There is no page on which both P1 values appear.

## Totals

Row counts are of ADDRESSED rows (P1/P2/P3 cells printing digits). A wrapped Function is one row; the repeated page
header is not a row; the four dash-addressed rows on PDF page 28 are not counted.

| PDF page | list | row_count |
|---|---|---|
| 23 | Menu | 39 |
| 24 | Menu | 30 |
| 25 | Menu | 35 |
| 26 | Menu | 35 |
| 27 | Menu | 28 |
| 28 | Advanced Menu | 27 |

- **P1 = 0 (Menu): 167 addressed rows.**
- **P1 = 1 (Advanced Menu): 27 addressed rows.**
- **Overall: 194 addressed rows.**

Cross-check by P2 block (this is the independent re-derivation referred to under Method):

| P1 P2 | P3 range | rows | pages |
|---|---|---|---|
| 0 00 | 00 – 34 | 35 | 23 |
| 0 01 | 00 – 08 | 9 | 23 (00–03), 24 (04–08) |
| 0 02 | 00 – 14 | 15 | 24 |
| 0 03 | 00 – 14 | 15 | 24 (00–09), 25 (10–14) |
| 0 04 | 00 – 05 | 6 | 25 |
| 0 05 | 00 – 15 | 16 | 25 |
| 0 06 | 00 – 10 | 11 | 25 (00–07), 26 (08–10) |
| 0 07 | 00 – 19 | 20 | 26 |
| 0 08 | 00 – 35 | 36 | 26 (00–11), 27 (12–35) |
| 0 09 | 00 – 03 | 4 | 27 |
| 1 00 | 00 – 26 | 27 | 28 |

35 + 9 + 15 + 15 + 6 + 16 + 11 + 20 + 36 + 4 = 167, matching 39 + 30 + 35 + 35 + 28 = 167. 27 matches the single
Advanced Menu page.

## Ranges and contiguity

- **Menu (P1 = 0):** lowest address `0 00 00`, highest address `0 09 03`. The P2 values run `00` through `09` with no
  value skipped and none repeated. Within every P2 block the P3 values run from `00` upward in steps of 1 with no gap and
  no duplicate. The sequence is **contiguous**. (The blocks are of unequal length — 35, 9, 15, 15, 6, 16, 11, 20, 36, 4 —
  which is how the book prints them, not a gap.)
- **Advanced Menu (P1 = 1):** lowest address `1 00 00`, highest address `1 00 26`. Only one P2 value, `00`, is printed,
  and its P3 values run `00` through `26` with no gap and no duplicate. The sequence is **contiguous**.

No gaps and no duplicate addresses were found in either list, so there are no STOP findings of that class.

## Dash-addressed rows (recorded, not counted)

All four are at the foot of PDF page 28, immediately after `1 00 26`, and all four print `Does not correspond to a
command` as a single prose cell spanning the whole P5 grid. Addresses are given exactly as printed, with `—` for the long
dash and `–` for the shorter one:

| P1 | P2 | P3 | Function |
|---|---|---|---|
| — | — | — | Touchscreen Calibration |
| — | — | – | Software License Agreement |
| — | — | – | Important Notices Concerning Free Open Source |
| — | – | — | About Various Software License Agreements |

(The fourth Function is printed hyphenated across two lines as "About Various Software License Agree-ments"; joined here
as one Function.)

## Rows whose Function says the address "does not correspond to a command"

**None.** No row that prints digits in its P1/P2/P3 cells carries that text. The phrase appears only in the P5 grid of
the four dash-addressed rows listed above, and it is printed in the grid, not in the Function column.

## TEXT rows (prose printed across the grid instead of cells)

Rows whose prose states a character or digit count:

| Address | Function | count the prose states |
|---|---|---|
| 0 00 06 | Screen Saver Message | "Up to 10 alphanumeric characters" — 10 characters |
| 0 00 07 | Power-on Message | "Up to 15 alphanumeric characters" — 15 characters |
| 0 05 11 | Contest Number | "0001 ~ 9999 (Must be a 4-digit number)" — 4 digits |
| 0 08 05 – 0 08 11 (7 rows, PDF page 26) and 0 08 12 – 0 08 32 (21 rows, PDF page 27) — 28 rows in all | the Fixed Mode band Lower/Upper Limit rows, from "Fixed Mode LF Band Lower Limit (min. 0.03 MHz)" through "Fixed Mode 50 MHz Band Upper Limit (max. 60 MHz)" | each prints the identical sentence "8-digit frequency (in Hz) with unused digits entered as 0 (in steps of 1 kHz)" — 8 digits |
| 1 00 05 | Reference Oscillator Calibration | "Parameter value of 000 ~ 510, corresponding to setting values of -255 ~ +255 (in steps of 1)" — the parameter is stated as a 3-digit value; no character count is worded |
| 1 00 07 | Attenuation (Additional Roofing Filter) | "Parameter value of 000 ~ 040, corresponding to setting values of -20 ~ +20 (in steps of 1)" — 3-digit value; no character count is worded |

The 28 Fixed Mode rows repeat one identical sentence, which is the "shared prose cell" shape; they are listed here as
TEXT rows because the sentence does state a digit count, and the fact that they share it is recorded so that a later
reader can classify them either way without re-reading the book.

## Prose-cell rows (shared prose, no count stated — NOT text rows)

| Addresses | Function range | prose printed across the grid |
|---|---|---|
| 0 00 15 – 0 00 32 (18 rows, PDF page 23) | PF A: Key Assignment; PF B: Key Assignment; Voice (Main Band): Key Assignment; Voice (Sub Band): Key Assignment; External PF 1 – 8: Key Assignment; Microphone PF 1 – 4: Key Assignment; Microphone Down: Key Assignment; Microphone Up: Key Assignment | "Refer to the list of function allotment numbers for the PF key" |
| the four dash-addressed rows (PDF page 28) | see above | "Does not correspond to a command" |

## Rows reaching a header of more than three characters (the `006 ~` column)

The only sub-header longer than three characters is `006 ~`; there is no column headed "Over". Thirty-four addressed rows
print a discrete cell under it:

**PDF page 23 (5 rows)**

| Address | Function | 006 ~ cell |
|---|---|---|
| 0 00 12 | Long Press Duration of Panel Keys | Up to 2000 [ms] (in steps of 100) |
| 0 01 00 | Beep Volume | Up to 20 (in steps of 1) |
| 0 01 01 | Voice Message Volume (Play) | Up to 20 (in steps of 1) |
| 0 01 02 | Sidetone Volume | Up to 20 (in steps of 1) |
| 0 01 03 | Voice Guidance Volume | Up to 20 (in steps of 1) |

**PDF page 24 (5 rows)**

| Address | Function | 006 ~ cell |
|---|---|---|
| 0 01 07 | Headphones Mixing Balance | Up to 10 (in steps of 1) |
| 0 02 00 | FFT Scope Averaging (RTTY Decode) | Up to 9 (in steps of 1) |
| 0 02 09 | FFT Scope Averaging (PSK Decode) | Up to 9 (in steps of 1) |
| 0 03 02 | AM Mode Frequency Step Size (Multi/Channel Control) | 006: 25 / 007: 30 / 008: 50 / 009: 100 |
| 0 03 03 | FM Mode Frequency Step Size (Multi/Channel Control) | 006: 25 / 007: 30 / 008: 50 / 009: 100 |

**PDF page 25 (7 rows)**

| Address | Function | 006 ~ cell |
|---|---|---|
| 0 03 11 | Tuning Speed Control (Main) <Firmware version 1.20 or later> | Up to 10 (in steps of 1) |
| 0 03 12 | Sensitivity (Main) <Firmware version 1.20 or later> | Up to 10 (in steps of 1) |
| 0 03 13 | Tuning Speed Control (Sub) <Firmware version 1.20 or later> | Up to 10 (in steps of 1) |
| 0 03 14 | Tuning Speed Control Sensitivity (Sub) <Firmware version 1.20 or later> | Up to 10 (in steps of 1) |
| 0 05 07 | CW Keying Weight Ratio | Up to 4.0 (in steps of 0.1) |
| 0 05 13 | Channel Number (Count-up Message) | 006: Ch 6 / 007: Ch 7 / 008: Ch 8 |
| 0 05 15 | CW/ Voice Message Retransmit Interval Time | Up to 60 [s] (in steps of 1) |

**PDF page 26 (10 rows)**

| Address | Function | 006 ~ cell |
|---|---|---|
| 0 07 05 | USB: Audio Input Level | Up to 100 (in steps of 1) |
| 0 07 06 | ACC 2: Audio Input Level | Up to 100 (in steps of 1) |
| 0 07 07 | Optical: Audio Input Level | Up to 100 (in steps of 1) |
| 0 07 08 | USB: Audio Output Level (Main Band) | Up to 100 (in steps of 1) |
| 0 07 09 | USB: Audio Output Level (Sub Band) | Up to 100 (in steps of 1) |
| 0 07 10 | ACC 2: Audio Output Level (Main Band) | Up to 100 (in steps of 1) |
| 0 07 11 | ACC 2: Audio Output Level (Sub Band) | Up to 100 (in steps of 1) |
| 0 07 12 | Optical: Audio Output Level (Main Band) | Up to 100 (in steps of 1) |
| 0 07 13 | Optical: Audio Output Level (Sub Band) | Up to 100 (in steps of 1) |
| 0 08 03 | Marker Offset Frequency (SSB Mode) | 006: 800 [Hz] / 007: 1000 [Hz] / 008: 1500 [Hz] / 009: 2200 [Hz] |

**PDF page 27 (2 rows)**

| Address | Function | 006 ~ cell |
|---|---|---|
| 0 09 01 | Keyboard Language (USB Keyboard) | 006: Portuguese / 007: Portuguese (Brazilian) / 008: Spanish / 009: Spanish (Latin American) / 010: Italian |
| 0 09 03 | Repeat Speed (USB Keyboard) | Up to 32 (in steps of 1) |

**PDF page 28 (5 rows)**

| Address | Function | 006 ~ cell |
|---|---|---|
| 1 00 00 | Indication Signal Type (Main Band) | 006: SWR |
| 1 00 02 | Output Level (Main Band) | Up to 100 [%] (in steps of 1) |
| 1 00 03 | Output Level (Sub Band) | Up to 100 [%] (in steps of 1) |
| 1 00 06 | Bandwidth (Additional Roofing Filter) | Up to 3500 [Hz] (in steps of 100) |
| 1 00 13 | Microphone Gain (FM Mode) | Up to 100 (in steps of 1) |

Separately, every prose row listed in the two sections above (the 18 PF-key rows, the 28 Fixed Mode rows, 0 00 06,
0 00 07, 0 05 11, 1 00 05, 1 00 07 and the four dash rows) prints its prose as a single unruled cell running the full
width of the P5 grid, so its ink physically reaches under the `006 ~` header too. Those are recorded in their own
sections rather than here, because they have no discrete cell in that column.

## STOP findings — printing defects and shape anomalies (recorded, not resolved)

1. **Merged address cell at 0 03 01, PDF page 24.** The address `0 03 01` is printed once in a single tall cell that
   spans two ruled Function cells, the upper reading "SSB Mode Frequency Step Size (Multi/Channel Control) <Firmware
   version 1.20 or later>" and the lower "SSB/CW/FSK/PSK Mode Frequency Step Size (Multi/Channel Control) <Firmware
   version 1.13 or lower>". Each Function cell has its own row of P5 values (both read 0.5 [kHz] / 1 [kHz] / 2.5 [kHz] /
   5 [kHz] / 10 [kHz] under 000–004). The digits sit level with the rule dividing the two Function cells. A 300 %
   enlargement confirmed one address cell, not two stacked cells. Following the brief's shape rule, this has been
   counted as **one** addressed row and not merged into either neighbour. Which of the two Function texts is the
   canonical one for this address is left unresolved here.
2. **Green ink at 0 00 34, PDF page 23.** The row "Data Mode Numbers <Firmware version 1.20 or later>" is printed
   entirely in green — the `0`, the `00`, the `34`, the Function text and its P5 values `1`, `2`, `3` under 001–003.
   Every other row on all six pages is black. No key or legend for the colour is printed on any of the six chart pages.
3. **Inconsistent dash glyphs in the four dash-addressed rows, PDF page 28.** A 300 % enlargement shows two distinct
   dash widths used within the same block: `— — —` (Touchscreen Calibration), `— — –` (Software License Agreement),
   `— — –` (Important Notices Concerning Free Open Source), `— – —` (About Various Software License Agreements). The
   shorter dash appears in the P3 cell of rows 2 and 3 and in the P2 cell of row 4, with no discernible pattern.
4. **Option lists that start at 001, with the 000 cell left blank.** Nine addressed rows print no cell under `000` and
   begin their options under `001`: `0 00 09` Meter Response Speed (1/2/3/4 under 001–004); `0 00 34` Data Mode Numbers
   (1/2/3 under 001–003); `0 03 11`, `0 03 12`, `0 03 13`, `0 03 14` (PDF page 25); `0 07 19` Antenna Numbers (PDF page
   26, 1/2/3/4 under 001–004); `0 09 02` Repeat Delay Time (USB Keyboard) and `0 09 03` Repeat Speed (USB Keyboard)
   (PDF page 27); `1 00 13` Microphone Gain (FM Mode) (PDF page 28). Whether code 000 is unused or simply unprinted is
   not stated on any of the six pages.
5. **Non-monotonic pairing between 1 00 00 and 1 00 01, PDF page 28.** "Indication Signal Type (Main Band)" runs
   Automatic / TX Power / ALC / Drain Voltage (Vd) / Compression Level (COMP) / Current (Id) / 006: SWR across
   000–006 ~, while "Indication Signal Type (Sub Band)" on the very next row runs TX Power / ALC / Drain Voltage (Vd) /
   Compression Level (COMP) / Current (Id) / SWR across 000–005 — the same option list shifted one code lower, with no
   "Automatic" entry and with SWR in a ruled 005 cell rather than in the 006 ~ column. Recorded as printed.
6. **Reduced type overrunning ruled cells.** Several cells are set in a visibly smaller size than their neighbours to
   fit, and the ink touches or crosses the cell rule. The clearest is `0 02 13` CW/RTTY/PSK Time Stamp, PDF page 24,
   whose 002 cell reads "Time Stamp + Frequency" with "Frequency" shrunk and running to the right-hand rule; the 001
   cell of `0 02 14` ("Secondary Clock") and the 001 cell of `0 08 02` Bandscope Maximum Hold ("Continuous", PDF page
   26) are shrunk the same way. No cell was found whose text is genuinely straddled across two columns' worth of grid,
   and no cell was unreadable at the magnifications used.
7. **Enumerated cells extending past the printed header range.** Four `006 ~` cells enumerate codes beyond 006 —
   `0 03 02` and `0 03 03` reach 009, `0 08 03` reaches 009, `0 05 13` reaches 008, and `0 09 01` reaches **010**, a
   three-digit code above 009 for which no column exists. Recorded as printed. The `0 08 03` cell also line-wraps
   mid-label ("006:" then "800 [Hz]" on the next line), and `0 09 01` wraps "Portuguese (Brazilian)" and "Spanish
   (Latin American)" across several lines within the one cell.
8. **Hyphenated Function texts broken across lines.** `1 00 08` prints "TX Power Down with Transverter En-abled" and the
   fourth dash row prints "About Various Software License Agree-ments". Joined with the hyphen removed when quoted
   above; recorded here so the printed form is not lost.

No other defect — duplicated header, unreadable cell, or cell straddling two columns — was found on the six pages.
