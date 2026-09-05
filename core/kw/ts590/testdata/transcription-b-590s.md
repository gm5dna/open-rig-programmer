# Transcription B — TS-590S EX Command Parameter List

**Date:** 05/09/2026

## Source, as printed

- Cover title (PDF page 1): **TS-590S / TS-590SG — PC CONTROL COMMAND Reference Guide**,
  "JVCKENWOOD Corporation", dated **January/30/2019**.
- **No revision number is printed anywhere on the cover or on the pages used.** The string
  "rev3" appears only in the supplied filename, not in the document itself. Recorded, not
  resolved.
- Chart heading, as printed on folio 7: **"EX Command Parameter List (for TS-590S)"**.

## Pages used

| PDF page | Printed folio | Content |
|---|---|---|
| 8 | – 7 – | Heading "EX Command Parameter List (for TS-590S)"; chart rows 000–010 (chart begins here, below the `EX` command box) |
| 9 | – 8 – | Chart rows 011–059 (repeated column header at top, not counted as a row) |
| 10 | – 9 – | Chart rows 060–087; chart ends at 087. Immediately below it the heading "EX Command Parameter List (for TS-590SG)" starts the second list, which was NOT read. |

The chart's ruled rows were followed continuously across both page seams: folio 7 ends at
010 and folio 8 opens at 011; folio 8 ends at 059 and folio 9 opens at 060. The repeated
"Menu (P1) / Function / Command Parameter (P5)" header band at the top of folios 8 and 9 was
excluded from the row count.

Column headers of the P5 grid, as printed: `0 1 2 3 4 5 6 7 8 9 10 ~` (the last header is the
two-character `10` followed by a tilde, printed as "10 ~").

## Method

- Pre-rendered 300 dpi page images were used only to locate the chart.
- All transcription was done from fresh crops rendered with `/opt/homebrew/bin/pdftoppm`
  directly from the PDF, written to
  `.../scratchpad/quarantine/B-590s/work/`:
  - **Pass 1** — 600 dpi (≈8.3× the printed size), six crops: `p07-tbl` (folio 7 table),
    `p08-a/-b/-c` (folio 8 in three overlapping bands), `p09-a/-b` (folio 9 in two
    overlapping bands). Crop boundaries were chosen to overlap by one full row so no ruled
    row could fall in a seam.
  - **Pass 2** — 500 dpi, six crops (`q07`, `q08-1/-2/-3`, `q09-1/-2`) with *different* band
    boundaries from pass 1, read independently.
  - **Third look** — 1200 dpi (≈16.7×) targeted crops for three items where character shape
    or word spacing was in question: `z-micpf` (rows 079–087 Function column), `z-051`
    (row 051 Function cell), `z-002`.
- At 500–600 dpi the glyphs are ≥40 px tall; `0`/`8`/`6` and `l`/`1`/`I` are unambiguous. The
  menu-number column and the grid's last-populated column were each read in both passes and
  reconciled row by row.
- **Reconciliation result: pass 1 and pass 2 agreed on every one of the 88 menu numbers and
  on every row's last-populated column. No discrepancy arose, so no third look was needed to
  settle a number or a column; the 1200 dpi crops above were taken for the spacing questions
  only.**

## Nothing else consulted

No text-layer extraction was used (no `pdftotext`, `pdfinfo`, or copy of the PDF text layer).
No other file in the repository or elsewhere was opened, no directory was listed apart from
the scratchpad render directory and my own work directory, no search was run, and no web
access was made. Nothing was carried in from other Kenwood radios' menus. The second chart in
the same PDF, "EX Command Parameter List (for TS-590SG)", was NOT transcribed; it was seen
only incidentally on folio 9 while establishing where the TS-590S chart ends, and none of its
content has been used.

## Row count and range

88 rows, menu numbers **000** to **087**, strictly increasing by 1 with no duplicate, no
missing number, and no out-of-order number.

## FINDINGS

### TEXT rows

- **087 "Power on message"** — the grid is replaced by prose spanning the full P5 width:
  `Power on Message (up to 8 ASCII characters)`. Recorded as `digits=8`, `text=1`.

- **079–086 (the eight PF-key rows) — AMBIGUOUS, the main item for reconciliation.**
  Rows 079 "Panel PF A function", 080 "Panel PF B function", 081 "Mic PF 1 function",
  082–084 "Mic  PF 2/3/4 function", 085 "Mic  PF (DWN) function", 086 "Mic  PF (UP) function"
  share **one merged cell** spanning all eleven P5 columns and all eight rows. It prints:

  > `000 ~ 255 (3-digit)`
  > `Refer to the TS-590S instruction manual for the numbers and functions. (When the function is turned OFF, 255 is used.)`

  Structurally these rows print prose across the grid instead of cells, which matches the
  brief's *structural* definition of a TEXT row. But the prose describes a **3-digit numeric
  code**, not "characters/ASCII/a message/a version string", which is the brief's gloss on
  the `text=1` flag. I have recorded them as **`digits=3`, `text=0`** — the character count
  the prose states is 3, and the field holds an enumerated numeric code rather than free
  text. **If transcription A read these as `text=1`, that is a definitional difference, not a
  reading difference: both agree the printed prose is "000 ~ 255 (3-digit)".** Recorded, not
  resolved.

- No "read only" row and no firmware/version row appears in the TS-590S chart. (The chart
  begins at 000 "Display brightness".)

### Rows with digits ≥ 2, and what the last cell prints

The `10 ~` header is two characters wide, so any row reaching it takes `digits=2`.

| Menu | Function | Last cell (under header `10 ~`) |
|---|---|---|
| 034 | Side tone/ pitch frequency setting (Hz) | `up to 1000 (steps of 50)` — grid runs 300, 350, 400, 450, 500, 550, 600, 650, 700, 750 under headers 0–9 |
| 036 | Keying weight ratio | `up to 4.0 (steps of 0.1)` — grid runs AUTO, 2.5, 2.6, 2.7, 2.8, 2.9, 3.0, 3.1, 3.2, 3.3 under headers 0–9 |
| 057 | Voice/ message playback repeat duration (seconds) | `up to 60 (steps of 1)` — grid runs 0–9 under headers 0–9 |
| 070 | DATA VOX delay | `up to 100 (steps of 5)` — grid runs 0, 5, 10, 15, 20, 25, 30, 35, 40, 45 under headers 0–9 |
| 079–086 | PF-key assignments | merged prose cell, `000 ~ 255 (3-digit)` — see the ambiguity note above; `digits=3` |
| 087 | Power on message | prose cell, `Power on Message (up to 8 ASCII characters)`; `digits=8`, `text=1` |

No other row reaches the `10 ~` column; every other row's last populated cell sits under a
single-character header (`0`…`9`), giving `digits=1`.

### Numbering

No gap, no duplicate, no non-increasing number. The sequence 000…087 is complete. (Note for
whoever reconciles: the TS-590S chart ends at 087, which matches the `EX` command box on
folio 7 printing "000 ~ 087: Menu number (TS-590S)".)

### Straddled or hard-to-read cells

- **Merged cell, rows 079–086** — one cell straddles all eleven P5 columns *and* eight ruled
  rows (folio 9, crops `p09-b-10.png` / `q09-2-10.png`). This is the only column-straddling
  cell in the chart; it is a deliberate merge, not a printing fault.
- **Row 087** — its prose cell likewise straddles all eleven P5 columns (same crops).
- No cell was left unread. Nothing in the chart required a confidence qualifier after the
  1200 dpi look.

### Printing / typesetting defects and quirks

1. **Inconsistent word spacing in the "Mic PF" names (rows 081–086).** At 1200 dpi
   (`work/z-micpf-10.png`) row **081 prints "Mic PF 1 function" with a single space**, while
   rows **082, 083, 084, 085, 086 print "Mic  PF …" with a visibly doubled space** between
   "Mic" and "PF". Rows 079 and 080 read "Panel PF A/B function" with single spaces. The CSV
   preserves this verbatim (double space in 082–086, single in 081). This is a typesetting
   inconsistency in the source, and it is a likely place for transcription A and B to differ.
2. **Row 051 is justified, not multiply spaced.** Its first line prints as
   `TX  hold  when  AT  completes` with stretched inter-word gaps because the line is
   justified to the cell width; the second line "the tuning" is set normally. Confirmed at
   1200 dpi (`work/z-051-09.png`). I have recorded the name with **single** spaces:
   `TX hold when AT completes the tuning`. Recorded so that a differing reading in A can be
   attributed to justification rather than to the copy.
3. **Space after the solidus in three names.** Printed as "Side tone**/ **pitch frequency
   setting (Hz)" (034) and "Voice**/ **message playback repeat" (056) / "…repeat duration
   (seconds)" (057) — a space follows the slash but none precedes it. Preserved verbatim.
   Other slashes in the chart carry no spaces ("MULTI/CH", "SSB/AM", "SSB/CW/FSK",
   "ON/OFF", "dot/dash").
4. **Header "10 ~"** is printed with a space between the numeral and the tilde; the
   equivalent in-cell ranges use "~" without spaces in the 079–086 block ("000 ~ 255" does
   have spaces). No consequence for the CSV.
5. Row 001 "Back light color" offers only two settings (1, 2) on the TS-590S. Not a defect —
   noted because it is unusually narrow for a colour setting and is the kind of row a
   reconciler may query.
6. No smudging, no broken glyphs, no skewed rules, and no OCR-style artefacts were seen at
   any magnification; the PDF renders as clean vector type throughout the three pages used.

### STOP findings

**None.** No row's `digits` had to be recorded as `?`; every prose row states its character
count.
