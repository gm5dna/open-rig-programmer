# Transcription B — TS-990S "EX Command Parameter Lists"

**Date of transcription:** 08/09/2026

## Source, as printed

- Cover title (PDF p.1): **TS-990S — PC CONTROL COMMAND Reference Guide**
- Publisher line as printed: **JVCKENWOOD Corporation**
- Date as printed on the cover: **January/30/2019**
- Revision: **no revision number is printed anywhere on the cover or on the "ABOUT THIS REFERENCE GUIDE"
  page (PDF p.2 / folio 1)**. The supplied file is named `ts990s_pc_rev2.pdf`; "rev2" is a filename, not a
  printed revision, and is recorded here only as provenance.
- Running head on every chart page: `PC CONTROL COMMAND REFERENCE GUIDE`

## Pages used

| PDF page | Printed folio | Content |
|---|---|---|
| 23 | – 22 – | Heading "EX Command Parameter Lists"; table header band "Menu"; rows 0/00/00 – 0/01/03 |
| 24 | – 23 – | "Menu"; rows 0/01/04 – 0/03/09 |
| 25 | – 24 – | "Menu"; rows 0/03/10 – 0/06/07 |
| 26 | – 25 – | "Menu"; rows 0/06/08 – 0/08/11 |
| 27 | – 26 – | "Menu"; rows 0/08/12 – 0/09/03 (end of the P1=0 list) |
| 28 | – 27 – | Table header band **"Advanced Menu"**; rows 1/00/00 – 1/00/26, then four dash-addressed rows |

The chart ends on PDF p.28. PDF p.29 (folio – 28 –) begins a different family, "PF Key Assignment Lists", so
p.28 is the last page transcribed. PDF p.22 (folio – 21 –) was checked and carries no part of this chart.

The two lists the brief describes are separated by the P1 column: **P1 = 0** under the header band "Menu"
(PDF pp.23–27) and **P1 = 1** under the header band "Advanced Menu" (PDF p.28).

## Method

- Pages read visually from the supplied 300 dpi renders (`p-23.png` … `p-28.png`), 2481 × 3508 px, shown at
  1414 × 2000 px, for the first (whole-page, shape and row-order) pass.
- Second pass at **600 dpi** on targeted crops cut with `pdftoppm -r 600` and written under
  `…/quarantine/B-990/work/`:
  - `addr-<page>-1..3` — the P1/P2/P3 columns only (780 × 2000 px slices), i.e. roughly **6× linear
    enlargement** over the whole-page view. At that size 0/8/6 and 1/l/I are unambiguous; every address digit
    in the CSV was re-read from these crops.
  - `fn-<page>-1..3` — the Function column only (1250 × 2000 px slices, same magnification), used to fix
    wording, punctuation and the exact line breaks.
  - `g6-<page>-1..3` — the last grid column ("006 ~") only (420 × 2000 px slices), used to decide, row by
    row, whether the row reaches that column and what it prints there.
  - `hdrb-23.png` — the "006 ~" column header at **900 dpi** (~14× linear), to settle the header's exact
    glyphs.
- Ruled rows were followed continuously across the five page seams (0/01/03→0/01/04, 0/03/09→0/03/10,
  0/06/07→0/06/08, 0/08/11→0/08/12, 0/09/03→1/00/00); no row is split across a seam.
- Two independent passes were made over the address columns (whole page, then the 600 dpi `addr-` crops) and
  over the grid's last populated column (whole page, then the 600 dpi `g6-` crops). The two passes agreed on
  every row; no address and no last-column classification needed a third look.
- Only image rendering/cropping commands (`pdftoppm`) and file writes were run.

## Nothing else consulted

This transcription was produced solely from the rendered pages of
`docs/fixtures-private/manuals/ts990s_pc_rev2.pdf`. No text layer was extracted (no `pdftotext`, no
`pdfinfo`, no copy-paste from the PDF); no other file in this repository or elsewhere was opened; no
directory was listed other than the scratchpad render directory and my own work directory; no web access was
made; and nothing recalled about Kenwood radios, their menus, or any other Kenwood document was used. Every
value below comes from the printed ruled table.

## Counts

| List | P1 | Header band | Addressed rows |
|---|---|---|---|
| 1 | 0 | Menu | 167 |
| 2 | 1 | Advanced Menu | 27 |
| **Total** | | | **194** |

Per P2 group (P1 = 0): 00 → 35 rows (00–34); 01 → 9 (00–08); 02 → 15 (00–14); 03 → 15 (00–14); 04 → 6
(00–05); 05 → 16 (00–15); 06 → 11 (00–10); 07 → 20 (00–19); 08 → 36 (00–35); 09 → 4 (00–03).
P1 = 1: P2 = 00 only → 27 rows (00–26).

Lowest / highest address per list: list 1 **0/00/00 → 0/09/03**; list 2 **1/00/00 → 1/00/26**.
Four dash-addressed rows follow 1/00/26 and are **not** counted or transcribed into the CSV.

---

# FINDINGS

## 1. TEXT rows (prose across the grid describing characters / a message) — `text = 1`

| Address | Function | Prose printed across the grid | `digits` |
|---|---|---|---|
| 0/00/06 | Screen Saver Message | `Up to 10 alphanumeric characters` | 10 |
| 0/00/07 | Power-on Message | `Up to 15 alphanumeric characters` | 15 |
| 0/05/11 | Contest Number | `0001 ~ 9999 (Must be a 4-digit number)` | 4 |
| 0/08/05 … 0/08/32 (28 rows) | Fixed Mode … Band Lower/Upper Limit | `8-digit frequency (in Hz) with unused digits entered as 0 (in steps of 1 kHz)` | 8 |

The 28 Fixed Mode rows are 0/08/05, 06, 07, 08, 09, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23,
24, 25, 26, 27, 28, 29, 30, 31, 32 — each printing the identical prose above, once per row (the prose is
**not** a single merged cell spanning the rows; each row prints its own copy, and each row's ruled boundaries
are complete).

**Judgement point recorded, not resolved — 0/05/11 Contest Number.** Its prose spans the grid and states a
character count (4), which is why I set `text = 1, digits = 4`. But unlike the other TEXT rows the entered
value is numeric-only, not alphanumeric text, so a reconciler may prefer `text = 0`. The printed prose is
given verbatim above; the classification is the only thing in doubt, not the reading.

## 2. Shared / spanning prose-cell rows (grid prints prose instead of coded cells, but the prose does **not** state a character count) — `digits = 3`, `text = 0`

- **0/00/15 … 0/00/32 (18 rows)** — PF A, PF B, Voice (Main Band), Voice (Sub Band), External PF 1–8,
  Microphone PF 1–4, Microphone Down, Microphone Up: each row's grid prints
  `Refer to the list of function allotment numbers for the PF key`.
  Per the brief these are recorded as `digits = 3`. Note that the referenced allotment IDs are printed
  elsewhere in the book as **4-digit** values (e.g. `0000`, `1040`, `9999` in the "PF Key Assignment Lists" on
  PDF p.29); the `3` recorded here follows the brief's rule for this shape and is **not** evidence that the
  P5 field is three characters wide for these rows.
- **1/00/05 Reference Oscillator Calibration** — grid prints
  `Parameter value of 000 ~ 510, corresponding to setting values of -255 ~ +255 (in steps of 1)`.
  The prose names three-character codes (`000 ~ 510`), hence `digits = 3`, and it describes a numeric range
  rather than characters/a message, hence `text = 0`.
- **1/00/07 Attenuation (Additional Roofing Filter)** — grid prints
  `Parameter value of 000 ~ 040, corresponding to setting values of -20 ~ +20 (in steps of 1)`.
  Same treatment: `digits = 3`, `text = 0`.

## 3. Dash-addressed rows (NOT transcribed into the CSV, NOT counted)

All four sit at the very end of the Advanced Menu list on PDF p.28 (folio – 27 –), immediately after
1/00/26, and all four print `Does not correspond to a command` across the grid.

| P1 as printed | P2 as printed | P3 as printed | Function |
|---|---|---|---|
| — (em dash) | — (em dash) | — (em dash) | Touchscreen Calibration |
| — (em dash) | — (em dash) | – (en dash, visibly shorter) | Software License Agreement |
| — (em dash) | — (em dash) | – (en dash, visibly shorter) | Important Notices Concerning Free Open Source |
| — (em dash) | – (en dash, visibly shorter) | — (em dash) | About Various Software License Agreements |

**Printing defect / inconsistency:** the dash glyph is not used consistently. Verified at 600 dpi
(`work/addr-28-3-28.png`, `work/p28-dash-28.png`): row 1 uses three em dashes; rows 2 and 3 use em, em, en;
row 4 uses em, **en**, em — i.e. the short dash lands in the P2 cell on the last row and in the P3 cell on
the two before it. Recorded, not resolved.

## 4. Rows whose Function text says the address "does not correspond to a command" **and** whose address cells print digits

**None.** Every "Does not correspond to a command" row in this chart is dash-addressed (section 3). No
addressed row carries that text.

## 5. Rows whose grid reaches a header wider than the plain three-digit headers

The last grid column's header prints as **`006 ~`** — the three digits `006`, a space, then a tilde
(confirmed at 900 dpi, `work/hdrb-23.png`). I have recorded `digits = 3` for every such row, i.e. the width
of the numeric code in the header; the ` ~` is a continuation marker, not part of any P5 code. Where the
cell under it enumerates further codes, the largest code enumerated is itself three characters (up to `010`),
which corroborates 3.

Rows reaching the `006 ~` column, and what that cell prints:

| Address | Function | Cell under `006 ~` |
|---|---|---|
| 0/00/12 | Long Press Duration of Panel Keys | `Up to 2000 [ms] (in steps of 100)` |
| 0/01/00 | Beep Volume | `Up to 20 (in steps of 1)` |
| 0/01/01 | Voice Message Volume (Play) | `Up to 20 (in steps of 1)` |
| 0/01/02 | Sidetone Volume | `Up to 20 (in steps of 1)` |
| 0/01/03 | Voice Guidance Volume | `Up to 20 (in steps of 1)` |
| 0/01/07 | Headphones Mixing Balance | `Up to 10 (in steps of 1)` |
| 0/02/00 | FFT Scope Averaging (RTTY Decode) | `Up to 9 (in steps of 1)` |
| 0/02/09 | FFT Scope Averaging (PSK Decode) | `Up to 9 (in steps of 1)` |
| 0/03/02 | AM Mode Frequency Step Size (Multi/Channel Control) | `006: 25` `007: 30` `008: 50` `009: 100` |
| 0/03/03 | FM Mode Frequency Step Size (Multi/Channel Control) | `006: 25` `007: 30` `008: 50` `009: 100` |
| 0/03/11 | Tuning Speed Control (Main) | `Up to 10 (in steps of 1)` |
| 0/03/12 | Sensitivity (Main) | `Up to 10 (in steps of 1)` |
| 0/03/13 | Tuning Speed Control (Sub) | `Up to 10 (in steps of 1)` |
| 0/03/14 | Tuning Speed Control Sensitivity (Sub) | `Up to 10 (in steps of 1)` |
| 0/05/07 | CW Keying Weight Ratio | `Up to 4.0 (in steps of 0.1)` |
| 0/05/13 | Channel Number (Count-up Message) | `006: Ch 6` `007: Ch 7` `008: Ch 8` |
| 0/05/15 | CW/ Voice Message Retransmit Interval Time | `Up to 60 [s] (in steps of 1)` |
| 0/07/05 | USB: Audio Input Level | `Up to 100 (in steps of 1)` |
| 0/07/06 | ACC 2: Audio Input Level | `Up to 100 (in steps of 1)` |
| 0/07/07 | Optical: Audio Input Level | `Up to 100 (in steps of 1)` |
| 0/07/08 | USB: Audio Output Level (Main Band) | `Up to 100 (in steps of 1)` |
| 0/07/09 | USB: Audio Output Level (Sub Band) | `Up to 100 (in steps of 1)` |
| 0/07/10 | ACC 2: Audio Output Level (Main Band) | `Up to 100 (in steps of 1)` |
| 0/07/11 | ACC 2: Audio Output Level (Sub Band) | `Up to 100 (in steps of 1)` |
| 0/07/12 | Optical: Audio Output Level (Main Band) | `Up to 100 (in steps of 1)` |
| 0/07/13 | Optical: Audio Output Level (Sub Band) | `Up to 100 (in steps of 1)` |
| 0/08/03 | Marker Offset Frequency (SSB Mode) | `006: 800 [Hz]` `007: 1000 [Hz]` `008: 1500 [Hz]` `009: 2200 [Hz]` |
| 0/09/01 | Keyboard Language (USB Keyboard) | `006: Portuguese` `007: Portuguese (Brazilian)` `008: Spanish` `009: Spanish (Latin American)` `010: Italian` |
| 0/09/03 | Repeat Speed (USB Keyboard) | `Up to 32 (in steps of 1)` |
| 1/00/00 | Indication Signal Type (Main Band) | `006: SWR` |
| 1/00/02 | Output Level (Main Band) | `Up to 100 [%] (in steps of 1)` |
| 1/00/03 | Output Level (Sub Band) | `Up to 100 [%] (in steps of 1)` |
| 1/00/06 | Bandwidth (Additional Roofing Filter) | `Up to 3500 [Hz] (in steps of 100)` |
| 1/00/13 | Microphone Gain (FM Mode) | `Up to 100 (in steps of 1)` |

That is 34 rows. Every other addressed, non-prose row has its last populated cell under a plain three-digit
header (`000`–`005`), so `digits = 3` for those too.

## 6. Addresses that do not increase, are duplicated, or are missing

**None.** Machine-checked over the finished CSV: within every (P1, P2) group P3 starts at `00` and increases
by exactly 1 to the group's last value, with no repeats and no gaps; P2 within P1 = 0 runs `00`…`09` with no
gaps; P1 takes only `0` and `1`.

Two observations that are **not** breaks in this chart's own sequence but are recorded because a reconciler
may trip on them:

- The Advanced Menu chart stops at 1/00/26, yet the "PF Key Assignment Lists" on PDF p.29 lists direct-call
  IDs `2000` … `2030` under the label "Advanced MENU 0" … "Advanced MENU 30". The chart therefore carries no
  rows for Advanced Menu numbers 27–30.
- P1 = 0, P2 = 09 stops at 0/09/03, which matches "Menu 09-03" / allotment `0903` as the last entry of the
  "Menus" block on PDF p.29.

## 7. The one address printed on a line of its own between two firmware-versioned Function descriptions

**0/03/01**, PDF p.24 (folio – 23 –). The P1/P2/P3 cells are single merged cells spanning two Function
sub-rows; the rule that separates the two Function descriptions runs **only** across the Function and grid
columns and stops at the P3/Function boundary — the address cells are not divided. Verified at 600 dpi
(`work/p24-0301-24.png`, `work/addr-24-2-24.png`).

The two Function descriptions, verbatim (each with its own complete set of P5 cells,
`0.5 [kHz] / 1 [kHz] / 2.5 [kHz] / 5 [kHz] / 10 [kHz]`, in both sub-rows):

1. upper: `SSB Mode Frequency Step Size (Multi/Channel Control)` `<Firmware version 1.20 or later>`
2. lower: `SSB/CW/FSK/PSK Mode Frequency Step Size (Multi/Channel Control)` `<Firmware version 1.13 or lower>`

The digits `0  03  01` are set on the vertical centre of the merged cell, which places them **on the
dividing line** — their baseline sits a few hundredths of an inch below the rule, between the two
descriptions rather than clearly beside either one.

Per the brief's counting rule (an addressed row is one whose P1/P2/P3 cells print digits) this is **one**
addressed row, and it is emitted once in the CSV. For `name` I have taken the **upper** description, i.e.
`SSB Mode Frequency Step Size (Multi/Channel Control) <Firmware version 1.20 or later>`, on the grounds that
it is the first in chart order. **This is recorded, not resolved:** "the text beside it" is genuinely
ambiguous here, and a reconciler wanting the 1.13-or-lower name has it verbatim above.

No other address in the chart is printed this way; every other multi-line cell is a single Function wrapped
over several lines.

## 8. Cells straddling two columns, or unreadable with confidence

**None unreadable.** Every address digit, every Function word and every last-populated-cell decision was read
at 600 dpi or better and matched between the two passes.

On straddling: the prose rows listed in sections 1 and 2, and the `Does not correspond to a command` rows in
section 3, print a single cell that spans the whole seven-column P5 grid — the vertical rules are absent
across that span. That is the printed design, not a defect, and those rows are recorded as TEXT rows or
prose-cell rows accordingly. No cell straddles only *some* of the columns, and no coded value sits on a
column boundary.

## 9. Printing defects, colour and typography

- **Green ink.** Row **0/00/34** is printed entirely in **green** — address digits `0  00  34`, the Function
  `Data Mode Numbers <Firmware version 1.20 or later>` and its three P5 cells (`1`, `2`, `3` under `001`,
  `002`, `003`). Every other row in the chart is black. Verified at 600 dpi (`work/addr-23-3-23.png`,
  `work/fn-23-3-23.png`). Recorded as printed; the colour carries no transcribable content.
- **Row 0/00/34 grid.** Its `000` cell is **blank**; the first populated cell is under `001`. The same shape
  occurs at 0/00/09 (Meter Response Speed), 0/01/04 (Voice Guidance Speed), 0/03/11–0/03/14, 0/09/02,
  0/09/03 and 1/00/13 — a blank `000`
  column with values starting at `001`. Recorded because it means the lowest valid P5 for those rows is not
  `000`.
- **Inconsistent "ACC 2" / "ACC2".** 0/07/06, 0/07/10 and 0/07/11 print `ACC 2:` with a space; 0/07/17 prints
  `ACC2:` without one. Both are transcribed exactly as printed. This is the book's inconsistency, not a
  reading error.
- **Line breaks inside a Function that a naive "join with one space" would corrupt.** The brief says a
  wrapped Function is joined with ONE space. That is what I did at every ordinary word-boundary wrap. Six
  wraps are *not* at word boundaries, and joining those with a space would insert a space the book does not
  print. In each case I joined with **no** space, because the identical phrase prints unbroken and unspaced
  elsewhere in the same chart. Every case is listed here so the choice can be reversed mechanically:

  | Address | Printed as (line 1 / line 2) | Recorded as |
  |---|---|---|
  | 0/03/00 | `Frequency Rounding Off (Multi/` / `Channel Control)` | `…(Multi/Channel Control)` |
  | 0/03/01 (upper) | `SSB Mode Frequency Step Size (Multi/` / `Channel Control)` / `<Firmware version 1.20 or later>` | `…(Multi/Channel Control) <Firmware…>` |
  | 0/03/02 | `AM Mode Frequency Step Size (Multi/` / `Channel Control)` | `…(Multi/Channel Control)` |
  | 0/03/03 | `FM Mode Frequency Step Size (Multi/` / `Channel Control)` | `…(Multi/Channel Control)` |
  | 0/06/08 | `Filter Control in SSB-Data Mode (High/` / `Low and Shift/Width)` | `…(High/Low and Shift/Width)` |
  | 1/00/08 | `TX Power Down with Transverter En-` / `abled` (a hyphenated word split) | `…Transverter Enabled` |

  Corroboration for the unspaced form: `9 kHz Step in AM Broadcast Band (Multi/Channel Control)` (0/03/05),
  `CW/FSK/PSK Mode Frequency Step Size (Multi/Channel Control)` (0/03/09) and the lower description at
  0/03/01 all print `(Multi/Channel Control)` unbroken on one line with no space; `Filter Control in SSB Mode
  (High/Low and Shift/Width)` (0/06/07) breaks after `(High/Low` and so is unaffected.
  The same hyphenated-split shape occurs in the fourth dash-addressed row, `About Various Software License
  Agree-` / `ments`, recorded in section 3 as `About Various Software License Agreements`.
- **Spaces the book *does* print after a slash** are kept verbatim and must not be normalised away:
  `CW/ Voice Message Retransmit Interval Time` (0/05/15) and
  `Touchscreen Tuning Step Correction (SSB/ CW/ FSK/ PSK) <Firmware version 1.20 or later>` (0/08/34).
- No torn, smudged, skewed or over-inked area was seen on PDF pp.23–28; the ruling is complete on every page
  and no row boundary is missing or doubled.

## 10. STOP findings

**None.** No TEXT row failed to state a character count, no address was unreadable, and no row's shape
prevented a decision. The two judgement points above (the `text` flag on 0/05/11, and which of the two
Function descriptions belongs to 0/03/01) are recorded for reconciliation, not raised as STOPs.
