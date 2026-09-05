# Transcription B — TS-590SG EX Command Parameter List

**Date:** 05/09/2026

## Source, as printed

- Cover (PDF page 1): KENWOOD logo; **TS-590S / TS-590SG**; **PC CONTROL COMMAND Reference Guide**;
  **JVCKENWOOD Corporation**; **January/30/2019**.
- Back page (PDF page 35): "© 2019 JVCKENWOOD Corporation"; part code **B5A-0316-20/01**.
- **No revision number ("Rev.3" or otherwise) is printed on the cover, the running header, or the
  back page.** The running header on every body page reads "PC CONTROL COMMAND REFERENCE GUIDE".
  The "(Rev.3)" in the task brief is not something I found printed in the document.
- Chart heading transcribed, verbatim: **"EX Command Parameter List (for TS-590SG)"**, printed
  immediately below the end of the TS-590S list on PDF page 10 (folio 9).

## Pages used

| PDF page | Printed folio | Contents used |
|---|---|---|
| 10 | – 9 – | SG heading + table header + rows 000–021 (bottom half of the page) |
| 11 | – 10 – | repeated table header + rows 022–069 (whole page) |
| 12 | – 11 – | repeated table header + rows 070–099, table ends; the **FA / FB** command box follows |

PDF page 1 and PDF page 35 were opened only for the title/date/part-code statement above, and
PDF page 8 (folio 7) only far enough to fix where the *TS-590S* list begins so that I could be
certain the list I read is the SG one and not that one. I did not read the TS-590S list's rows.

## Method

- Base renders: the supplied 300 dpi page PNGs (`pdf-pages-590/p-NN.png`).
- Working renders: `pdftoppm -r 600 -png` for PDF pages 10, 11 and 12 (4961 × 7016 px each).
- Crops with `magick -crop`, all written under `.../quarantine/B-590sg/work/`.
- **Pass 1 (content):** the 600 dpi pages cut into full-width horizontal strips ~1750–1900 px tall
  (≈ 1/4 page each), viewed at roughly 255 dpi effective — every column of every row read together
  so each cell stays under its own header.
- **Pass 2 (numbers):** the menu-number column alone cropped as tall narrow strips (700 px wide),
  viewed at ≈ 364 dpi effective, i.e. ~1.2× the printed size on screen. Read independently of
  pass 1, top to bottom.
- **Pass 2 (last populated column):** the right-hand grid columns alone (headers 7, 8, 9 and
  "10 ~", 1550 px wide) cropped as tall strips and read independently, purely to list which rows
  reach the "10 ~" column.
- **Pass 3 (spot checks):** 200 %–250 % enlargements of the 600 dpi image (≈ 1200–1500 dpi
  effective) for the individual cells and captions noted under FINDINGS.
- At every magnification used, 0/8/6 and l/1/I are unambiguous; the typeface is a plain grotesque
  with an open-tailed 6, a fully closed 8, and a 1 with a distinct flag.
- Ruled rows were followed continuously across the two page seams (021→022 and 069→070). The
  repeated "Menu (P1) / Function / Command Parameter (P5)" header at the top of PDF pages 11 and 12
  was **not** counted as a row.
- **Reconciliation:** pass 1 and pass 2 agree on all 100 menu numbers and on the set of rows
  reaching the "10 ~" column. **No discrepancy arose, so no third look was needed** other than the
  spot checks listed below.

## Nothing else consulted

Only the PDF named in the brief was opened, and only by rendering its pages as images. No text
layer was extracted (no `pdftotext`, no `pdfinfo`, no copy-paste from the PDF); no other file in
the repository or elsewhere was read; no directory was listed except my own scratch work
directory; no web access; no reference to any other radio's menu chart, and no reference to the
TS-590S list printed earlier in this same document. Everything below comes from the printed ruled
table on PDF pages 10–12.

---

## FINDINGS

### TEXT rows (prose across the grid) — 2

| Menu | Name | Prose printed across the grid | digits |
|---|---|---|---|
| 000 | Firmware Version | `Version information (4 ASCII characters) read only` | 4 |
| 001 | Power on message | `Power on Message (up to 8 ASCII characters)` | 8 |

Both state a character count, so neither is a STOP.

### Rows with `digits` ≥ 2, and what the last cell prints — 11

All eleven reach the final `10 ~` column header (2 characters).

| Menu | Name | Cell printed under `10 ~` |
|---|---|---|
| 005 | Beep volume | `~ 20 (steps of 1)` |
| 006 | Sidetone volume | `~ 20 (steps of 1)` |
| 007 | Message playback volume | `~ 20 (steps of 1)` |
| 008 | Voice guide volume | `~ 20 (steps of 1)` |
| 018 | MULTI/CH control step change for AM (kHz) | `P5=10: 100` |
| 019 | MULTI/CH control step change for FM (kHz) | `P5=10: 100` |
| 040 | Side tone/ pitch frequency setting (Hz) | `up to 1000 (steps of 50)` |
| 042 | Keying weight ratio | `up to 4.0 (steps of 0.1)` |
| 063 | Voice/ message playback repeat duration (seconds) | `up to 60 (steps of 1)` |
| 077 | DATA VOX delay | `up to 100 (steps of 5)` |
| 087–099 | (see the next section) | no grid cells at all — see below |

Note the two printings differ in kind: 005–008 print a bare `~ 20`, whereas 018/019 print
`P5=10: 100`, which names the P5 code explicitly and so is unambiguous. Rows 040, 042, 063 and 077
print `up to N (steps of M)` without naming the code. Recorded, not resolved.

Rows 087–099 are given `digits` 3; my reasoning and the ambiguity in it are set out immediately
below, and this is the one judgement in the CSV a reviewer should check first.

### Rows 087–099: one merged prose cell spanning the whole grid

Menu numbers **087, 088, 089, 090, 091, 092, 093, 094, 095, 096, 097, 098, 099** (thirteen
consecutive rows) keep their own ruled Menu and Function cells, but the whole `Command Parameter
(P5)` grid to their right is a **single merged cell**, with the column rules absent for the full
height of the block. It prints, on two lines:

```
000 ~ 255 (3-digit)
Refer to the TS-590SG instruction manual for the numbers and functions. (When the function is turned
OFF, 255 is used.)
```

This is prose across the grid, so by the brief's reading rule these are "TEXT rows"; but the
prose describes a **3-digit numeric range**, not "characters/ASCII/a message/a version string",
which is the narrower wording of the `text` column's definition. **I recorded `text` = 0 and
`digits` = 3** — the prose does state a character width ("3-digit"), and the P5 space here is an
enumerated numeric code, not a free-text string like 000/001. Flagging it because the brief's two
definitions pull in opposite directions on this block; if the intent is "no ruled cells ⇒ text=1",
these thirteen rows flip to `text` = 1 with `digits` unchanged at 3.

### Menu-number sequence

- 100 rows, numbers **000 … 099**, strictly increasing by 1 with **no gap, no duplicate and no
  out-of-order number**. Both independent passes agree. Every number is printed as three digits
  with its leading zeros.
- The list is bounded above and below cleanly: the heading "EX Command Parameter List (for
  TS-590SG)" opens it on PDF page 10, and the ruled table closes after 099 on PDF page 12, followed
  by the "FA / FB" command box.

### Cells that straddle columns, or that I read with less than full confidence

- **064 Split transfer function** (PDF page 11, crop `work/zoom-064.png`, 250 % of 600 dpi). The
  cells read `OFF` / `A-T R` / `A-SUB R` / `B` under headers 0/1/2/3. Header 1 prints `A-T R` with
  a visible space before the final R; header 2 prints it wrapped over two lines as `A-SUB` then
  `R`, centred. Whether the intended strings are `A-TR`/`A-SUBR` or `A-T R`/`A-SUB R` cannot be
  settled from the printing. It does not affect `digits` (last populated header is `3`, so 1).
- **016 and 017, MULTI/CH control step change for SSB / for CW/ FSK (kHz)** (PDF page 10, crop
  `work/zoom-16-17.png`, 180 %). Both rows print **`0.5` twice**, under header `1` and again under
  header `2`, then `1`, `2.5`, `5`, `10`. Read three times at increasing magnification; the two
  cells are identical, there is no `0.25`, no `0.05` and no differing suffix. Almost certainly a
  printing error in the source, but recorded as printed. Again it does not affect `digits`
  (last populated header is `6`, so 1).
- **084 PSQ control signal output condition**: the cells under headers 4 and 5 print `BSY-SND` and
  `SQL-SND` in a visibly **smaller point size** than every other cell in the table, squeezed to fit
  the column width. The characters themselves are legible and unambiguous.
- No cell anywhere in the SG list straddles two column rules (087–099 are the merged block
  described above, not a straddle). No broken, faint, or overprinted glyphs; the scan is clean
  throughout pages 10–12.

### Spacing in the Function column, as printed

The brief asks for verbatim spacing, so these are recorded explicitly.

- **Real double spaces, kept in the CSV** (checked at 200 % in `work/zoom-mic.png` and
  `work/zoom-028.png`; in both cases the line ends short of the cell edge, so justification cannot
  explain the gap):
  - `028` — `Low Cut/ Low Cut and Width/ Shift  change (SSB)` (two spaces before "change").
    Its twin `029` prints a single space in the same position: `Shift change (SSB-DATA)`.
  - `095`, `096`, `097`, `098`, `099` — `Mic  PF 2 function` etc., two spaces after "Mic".
    `094` alone prints a single space: `Mic PF 1 function`.
- **Wide gaps that are justification, normalised to one space in the CSV** (the line is stretched
  to the full cell width, and every inter-word gap on that line is widened equally):
  - `022` line 2, `standard/ Extension  memory` → recorded as
    `Temporary variable of the standard/ Extension memory frequency`.
  - `057` line 1, `TX  hold  when  AT  completes` → recorded as
    `TX hold when AT completes the tuning`.
- **Space after a solidus** is Kenwood's own inconsistency and is reproduced as printed: `CW/ FSK`,
  `Low Cut/ Low Cut`, `Side tone/ pitch`, `Voice/ message`, `standard/ Extension` all print a space
  after the slash, whereas `MULTI/CH`, `SSB/AM`, `SEND/PTT`, `ON/OFF`, `dot/dash`, `SSB-DATA` and
  `(exclude CW mode)` do not.
- Multi-line Function texts were joined with one space, per the brief.

### STOP findings

**None.**
