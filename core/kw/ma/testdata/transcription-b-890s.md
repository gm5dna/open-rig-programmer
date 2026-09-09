# Transcription B — TS-890S "EX Command Parameter Lists"

**Date of transcription:** 08/09/2026

## Source, as printed

- Title page: **TS-890S — PC CONTROL COMMAND Reference Guide**, JVCKENWOOD Corporation.
- Date printed on the title page: **January/30/2019**. No revision number is printed anywhere on the
  title page or on the chart pages; "rev1" appears only in the supplied filename, not in the book.
- Running head on every chart page: "PC CONTROL COMMAND REFERENCE GUIDE".

## Pages used

| PDF page | Printed folio | Content |
|---|---|---|
| 26 | – 25 – | Heading "EX Command Parameter Lists"; table head "Menu"; rows 0/00/00 – 0/02/08 |
| 27 | – 26 – | table head "Menu"; rows 0/02/09 – 0/05/07 |
| 28 | – 27 – | table head "Menu"; rows 0/05/08 – 0/08/06 |
| 29 | – 28 – | table head "Menu"; rows 0/08/07 – 0/09/03 (list 1 ends; rest of page blank) |
| 30 | – 29 – | table head "Advanced Menu"; rows 1/00/00 – 1/—/27, then the footnote "◆ P2 is any value." (list 2 ends; rest of page blank) |

PDF page 31 (folio – 30 –) begins a different family, "PF Key Assignment Lists", so the chart is
exactly PDF pages 26–30.

The book prints the chart as **two lists**, separated by the P1 value and by the grey banner over the
table head:

- **List 1 — P1 = 0**, banner "Menu", PDF pp. 26–29.
- **List 2 — P1 = 1**, banner "Advanced Menu", PDF p. 30.

## Method

- Worked from the pre-rendered 300 dpi page images `p-26.png` … `p-30.png`, viewed as images only.
- **Pass 1:** each of the five pages read whole (2481 × 3508 px, displayed at 1414 px wide, i.e. ≈ 0.57×
  of the render) to establish row order, page seams and the shape of each row's grid.
- **Pass 2 (independent):** each page cut with `magick` into native-resolution crops that were **not**
  downsampled when viewed — a 900 px-wide left block (`P1 | P2 | P3 | Function`) and a 1300 px-wide
  right block (the P5 grid), each 1550 px tall, two vertical bands per page (`work/L-*.png`,
  `work/R-*.png`). At 900 px for a ~340 px-wide address block this is roughly **3.5× the printed size**
  at 300 dpi, at which 0/8/6 and l/1/I are unmistakable.
- **Targeted re-renders at 600 dpi** with `pdftoppm -r 600` for five specific characters where a
  transcription error would be silent: the dash in 1/00/19 vs 1/00/20, the em dash in the P2 cell of the
  1/—/27 row, the trailing colon of 0/03/07, and the slash-space in 0/05/16. These are `work/dash19-30.png`,
  `work/dash20-30.png`, `work/p2dashC-30.png`, `work/tc-27.png`, `work/cwv3-28.png`.
- The address columns and the grid's last-populated column were each read twice (pass 1 whole-page, pass 2
  native crops). **The two passes reconciled with no disagreement**; the CSV was then machine-checked for
  duplicate addresses, non-increasing addresses, gaps within each P2 group and stray commas — all clean
  (see "Address sequence" below).
- Ruled rows were followed continuously across the four page seams (26→27 inside P2=02; 27→28 inside
  P2=05; 28→29 inside P2=08; 29→30 = the list-1/list-2 boundary). No row is split across a seam.

**Nothing else was consulted.** No text-layer extraction was performed (`pdftotext`/`pdfinfo` were never
run and no text was copied out of the PDF); no other file in this repository or elsewhere was opened; no
directory was listed other than the render directory and my own scratch directory; no web access; and no
prior knowledge of Kenwood menus or of any other Kenwood document was used. Every value below comes from
the printed ruled table alone. Only image render/crop commands and my own file writes were run, and no
command was left running.

## Counts

- List 1 (P1 = 0): **135** addressed rows — P2=00: 33, 01: 7, 02: 18, 03: 14, 04: 6, 05: 17, 06: 16,
  07: 12, 08: 8, 09: 4.
- List 2 (P1 = 1): **27** addressed rows — all P2=00.
- **Total 162 addressed rows** in `transcription-b.csv`.
- Plus **1 dash-addressed row**, not counted and not in the CSV (see below).

Lowest / highest address: list 1 **0/00/00 → 0/09/03**; list 2 **1/00/00 → 1/00/26** (the highest P3
printed in list 2 is 27, but that row is dash-addressed and therefore excluded).

## FINDINGS

### 1. The P5 column headers

Both lists use the same seven headers, printed identically on all five pages:

`000  001  002  003  004  005  006 ~`

The seventh header prints as `006 ~` — "006", a space, then a tilde — five characters as set, but the
**code** it carries is three digits. Every explicit code printed inside that column elsewhere in the chart
is likewise three digits (006, 007, 008, 009, **010**). `digits` is therefore **3** for every non-TEXT row
in the chart, including every row that reaches the `006 ~` column. No header in this chart is wider than
three digits, so no row produced a value other than 3.

### 2. TEXT rows (`text` = 1) — four in total

| Address | Function | Grid prose as printed | `digits` |
|---|---|---|---|
| 0/00/05 | Screen Saver Message | "Up to 10 alphanumeric characters" | 10 |
| 0/00/06 | Power-on Message | "Up to 15 alphanumeric characters" | 15 |
| 0/05/12 | Contest Number | "0001 ~ 9999 (Must be a 4-digit number)" | 4 |
| 1/00/05 | Reference Oscillator Calibration | "Parameter value of 0000 ~ 1000, / corresponding to setting values of -500 ~ +500" (two printed lines) | 4 |

**Judgement recorded, not resolved:** the first two match the brief's example exactly. The last two print
prose across the whole grid in the same way, but the prose describes a *numeric* field rather than
"alphanumeric characters"; I classified them as TEXT rows on the shape rule ("prose across the grid
instead of cells") and took the stated character count — "4-digit number" → 4, and "0000 ~ 1000" → 4. A
reconciler who reads "describing characters/a message" strictly would instead mark these two `text=0`,
`digits=3`. Flagged for that reason.

### 3. Shared-prose-cell rows (`text` = 0, `digits` = 3) — 21 in total

- **0/00/15 through 0/00/31 (17 consecutive rows)**, PDF p. 26: the whole grid is one merged cell reading
  "Refer to the list of function allotment numbers for the PF key". These are the PF A/B/C, External PF
  1–8, Microphone PF 1–4, Microphone DOWN and Microphone UP key-assignment rows. Recorded per the brief as
  prose-cell rows, not TEXT rows.
- **1/00/23, 1/00/24, 1/00/25, 1/00/26 (4 rows)**, PDF p. 30: the whole grid is one merged cell reading
  "Does not correspond to a command". See item 4 — I treated these the same way (prose-cell, `digits`=3),
  because the prose is a statement about the address rather than a description of a value's characters,
  and so states no character count. **Judgement recorded:** had I classed them as TEXT rows the brief
  would require `digits` = `?` plus four STOP findings; I did not, so there are no STOP findings.

### 4. "Does not correspond to a command" rows — 4, all counted and in the CSV

| Address | Function | Grid |
|---|---|---|
| 1/00/23 | Touchscreen Calibration | "Does not correspond to a command" |
| 1/00/24 | Software License Agreement | "Does not correspond to a command" |
| 1/00/25 | Important Notices concerning Free Open Source | "Does not correspond to a command" |
| 1/00/26 | About Various Software License Agreements | "Does not correspond to a command" |

Their P1/P2/P3 cells all print digits, so per the counting rule they **are** addressed rows: counted, and
transcribed into the CSV. Note the printed phrase sits in the **grid**, not in the Function text; the
brief's wording anticipates it in the Function text. The intent is unambiguous and the shape rule
(address cells print digits) decides it.

### 5. Dash-addressed rows — 1, NOT counted and NOT in the CSV

| Address as printed | Function | Grid |
|---|---|---|
| `1` / `—` / `27` | Firmware Version | "Reading command only" |

The P2 cell prints an **em dash** (confirmed at 600 dpi, `work/p2dashC-30.png`); P1 and P3 print digits.
The footnote immediately under the table explains it: "◆ P2 is any value." This is the only dash in any
address cell anywhere in the chart. It is also the only row in either list whose grid reads "Reading
command only".

### 6. Rows reaching the `006 ~` column, and what that last cell prints — 29 in total

PDF p. 26:

| Address | Last cell (`006 ~`) |
|---|---|
| 0/00/13 Long Press Duration of Panel Keys | "Up to 2000 [ms] (in steps of 100)" |
| 0/01/00 Beep Volume | "Up to 20" |
| 0/01/01 Voice Message Volume (Play) | "Up to 20" |
| 0/01/02 Sidetone Volume | "Up to 20" |
| 0/01/03 Voice Guidance Volume | "Up to 20" |
| 0/02/00 FFT Scope Averaging (RTTY Decode) | "Up to 9" |

PDF p. 27:

| Address | Last cell (`006 ~`) |
|---|---|
| 0/02/10 FFT Scope Averaging (PSK Decode) | "Up to 9" |
| 0/03/03 FM Mode Frequency Step Size | "006: 25 [kHz] / 007: 30 [kHz] / 008: 50 [kHz] / 009: 100 [kHz]" (four printed lines) |
| 0/03/04 AM Mode Frequency Step Size | "006: 25 [kHz] / 007: 30 [kHz] / 008: 50 [kHz] / 009: 100 [kHz]" |
| 0/03/08 Tuning Speed Control | "Up to 10" |
| 0/03/09 Tuning Speed Control Sensitivity | "Up to 10" |

PDF p. 28:

| Address | Last cell (`006 ~`) |
|---|---|
| 0/05/08 CW Keying Weight Ratio | "Up to 4.0 (in steps of 0.1)" |
| 0/05/14 Channel Number (Countup Message) | "006: Channel 6 / 007: Channel 7 / 008: Channel 8" |
| 0/05/16 CW/ Voice Message Retransmit Interval Time | "Up to 60 [s]" |
| 0/06/07 TX Filter High Cut (SSB/AM) | "006: 3500 [Hz] / 007: 4000 [Hz]" |
| 0/06/09 TX Filter High Cut (SSB-DATA/AM- DATA) | "006: 3500 [Hz] / 007: 4000 [Hz]" |
| 0/07/06 USB: Audio Input Level | "Up to 100" |
| 0/07/07 ACC 2: Audio Input Level | "Up to 100" |
| 0/07/08 USB: Audio Output Level | "Up to 100" |
| 0/07/09 ACC 2: Audio Output Level | "Up to 100" |
| 0/08/04 Waterfall Gradation Level | "Up to 10" |
| 0/08/05 Tuning Assist Line (SSB Mode) | "006: 800 [Hz] / 007: 1000 [Hz] / 008: 1500 [Hz] / 009: 2210 [Hz]" |

PDF p. 29:

| Address | Last cell (`006 ~`) |
|---|---|
| 0/09/01 Keyboard Language (USB Keyboard) | "006: Portuguese / 007: Portuguese (Brazilian) / 008: Spanish / 009: Spanish (Latin American) / **010: Italian**" |
| 0/09/03 Repeat Speed (USB Keyboard) | "Up to 32" |

PDF p. 30:

| Address | Last cell (`006 ~`) |
|---|---|
| 1/00/00 Indication Signal Type (External Meter 1) | "006: SWR" |
| 1/00/01 Indication Signal Type (External Meter 2) | "006: SWR" |
| 1/00/02 Output Level (External Meter 1) | "Up to 100 [%]" |
| 1/00/03 Output Level (External Meter 2) | "Up to 100 [%]" |
| 1/00/10 Microphone Gain (FM Mode) | "Up to 100" |

0/09/01 is the only cell in the chart that prints a code above 009 — **010** — and it is still three
digits, which is why `digits` = 3 throughout.

### 7. Address sequence

Machine-checked over the finished CSV: **no duplicate address, no non-increasing address, and no gap.**
Every P2 group runs contiguously from 00 to its maximum:

`0/00: 00–32 · 0/01: 00–06 · 0/02: 00–17 · 0/03: 00–13 · 0/04: 00–05 · 0/05: 00–16 · 0/06: 00–15 ·
0/07: 00–11 · 0/08: 00–07 · 0/09: 00–03 · 1/00: 00–26 (+27 dash-addressed)`

P2 itself also runs contiguously 00–09 in list 1, and list 2 has the single group 00.

The one sequence remark worth recording: in list 2 the printed P3 sequence reaches **27**, but 27 is the
dash-addressed row, so the highest *addressed* P3 in list 2 is 26. Nothing is missing — the dash row
occupies the 27 slot.

### 8. Function text — printing oddities recorded verbatim

- **0/03/07 "Tuning Control :"** — the Function cell ends with a space and a colon and nothing after it
  (confirmed at 600 dpi, `work/tc-27.png`). Reproduced verbatim in the CSV, trailing colon included. This
  looks like a truncated title in the source book, not a render artefact.
- **0/05/16 "CW/ Voice Message Retransmit Interval Time"** — there is a real space **after** the slash,
  on the same printed line (confirmed at 600 dpi, `work/cwv3-28.png`). Not a wrap. Reproduced verbatim.
- **Dash inconsistency in the four Virtual COM Port rows, PDF p. 30**, confirmed at 600 dpi
  (`work/dash19-30.png`, `work/dash20-30.png`):
  - 1/00/17 "Virtual Standard COM Port **–** RTS" (en dash)
  - 1/00/18 "Virtual Standard COM Port **–** DTR" (en dash)
  - 1/00/19 "Virtual Enhanced COM Port **–** RTS" (en dash)
  - 1/00/20 "Virtual Enhanced COM Port **-** DTR" (**hyphen-minus** — visibly shorter, different from the
    other three). Reproduced as printed. A typesetting slip in the book.
- **0/00/06 "Power-on Message"** uses a hyphen; contrast 1/00/06 "TX Power Down with Transverter Enabled".
  Both reproduced as printed.

### 9. Wrapped Function texts joined with one space — five joins where no space is printed

The brief's rule is to join a wrapped Function with ONE space. Applied mechanically to every wrap. In
**five** rows the printed line break falls at a slash or a hyphen with **no space in the printed text**,
so the mechanical join inserts a space that the book does not print. Recorded, not resolved — the CSV
holds the mechanical join:

| Address | Printed as (line 1 / line 2) | CSV `name` |
|---|---|---|
| 0/03/00 | "Frequency Rounding Off (Multi/" / "Channel Control)" | `Frequency Rounding Off (Multi/ Channel Control)` |
| 0/03/03 | "FM Mode Frequency Step Size (Multi/" / "Channel Control)" | `FM Mode Frequency Step Size (Multi/ Channel Control)` |
| 0/03/04 | "AM Mode Frequency Step Size (Multi/" / "Channel Control)" | `AM Mode Frequency Step Size (Multi/ Channel Control)` |
| 0/06/08 | "TX Filter Low Cut (SSB-DATA/AM-" / "DATA)" | `TX Filter Low Cut (SSB-DATA/AM- DATA)` |
| 0/06/09 | "TX Filter High Cut (SSB-DATA/AM-" / "DATA)" | `TX Filter High Cut (SSB-DATA/AM- DATA)` |

A reconciler applying "verbatim spacing" instead would render these as `(Multi/Channel Control)` and
`(SSB-DATA/AM-DATA)`. Compare 0/03/01, 0/03/02 and 0/03/05, whose wraps fall at ordinary word spaces and
which therefore read `(Multi/Channel Control)` with no inserted space — the difference between those rows
and 0/03/00/03/04 is purely where the line happened to break.

All other wrapped Functions break at ordinary word spaces, so the one-space join is exact for them:
0/01/05, 0/02/17, 0/03/12, 0/04/01, 0/05/06, 0/05/16, 0/06/11, 0/06/12, 0/07/05, 0/08/07, 0/09/00,
1/00/16, 1/00/25, 1/00/26.

### 10. Rows whose 000 cell is printed blank — 5

The value list begins at code 001, with 000 left empty:

| Address | Function | 001 onward |
|---|---|---|
| 0/00/08 | Meter Response Speed (Analog) | 1, 2, 3, 4 (001–004) |
| 0/01/04 | Voice Guidance Speed | 1, 2, 3, 4 (001–004) |
| 0/03/08 | Tuning Speed Control | **Off**, 2, 3, 4, 5, "Up to 10" |
| 0/03/09 | Tuning Speed Control Sensitivity | 1, 2, 3, 4, 5, "Up to 10" |
| 0/08/04 | Waterfall Gradation Level | 1, 2, 3, 4, 5, "Up to 10" |

0/03/08 is the odd one: 001 prints "Off" and 002 prints "2", so no cell in the row prints the value 1.
Recorded, not resolved.

### 11. Straddling / unreadable cells, printing defects

- **No cell anywhere in the chart straddles two columns ambiguously.** Every merged cell is a clean
  full-grid merge across all seven P5 columns (the TEXT rows of item 2 and the prose-cell rows of item 3),
  and each is bounded by the table's outer rules, so there is no ambiguity about which columns it covers.
- **No cell was unreadable with confidence.** Every address digit and every last-populated-column
  boundary was legible at the native-crop magnification; the five characters where a mistake would have
  been silent were additionally re-rendered at 600 dpi (item 8).
- **No render or scan defect** — no skew, no bleed-through, no broken glyphs, no missing page. All 77
  supplied renders were present; pages 26–30 rendered cleanly.
- The only defects found are typographic, in the source book: the truncated "Tuning Control :" (item 8),
  the hyphen/en-dash inconsistency in the Virtual COM Port rows (item 8), and the missing value 1 in
  0/03/08 (item 10).

### STOP findings

**None.** No TEXT row failed to state a character count, and no address, Function or grid boundary was
unreadable.
