# Transcription B — FT-891 CAT menu chart (EX / MENU)

**Date:** 04/09/2026

## Source document

- **Title as printed on the cover (PDF page 1):** `FT-891` / `CAT Operation` / `Reference Book`,
  above `YAESU MUSEN CO., LTD.` The running footer on every numbered page prints
  `FT-891 CAT Operation Reference Book`.
- **Revision code as printed:** `1909-C`. Found at the **bottom right of PDF page 20**, the final
  unnumbered page carrying `Copyright 2019 / YAESU MUSEN CO., LTD.` and the Yaesu addresses.
- **File read:** `/Users/stuart/coding/ft710-programmer/docs/fixtures-private/manuals/ft891_cat_1909-C.pdf`

## Pages used

| PDF page | Printed folio | Content |
|---|---|---|
| 8 | 7 | `EX` `MENU` command block, then the chart header and rows `0101`–`0508` (31 rows) |
| 9 | 8 | chart rows `0509`–`1406` (82 rows), header repeated at the top |
| 10 | 9 | chart rows `1407`–`1803` (46 rows), header repeated at the top; chart ends above the `FA` block |

Pages 1 (cover) and 20 (colophon) were viewed only for the title and the revision code.
PDF page 1 is unnumbered; the folio is one less than the PDF page number throughout
(PDF 2 = folio 1 … PDF 20 = unnumbered).

The `EX` block on PDF page 8 prints `P1 : 0101 - 1803 (MENU Number)` and
`P2 : Parameter (See Table below)`, which bounds the chart at exactly the first and last rows
transcribed.

## Method

1. **Locating.** The 20 supplied 300 dpi page renders were reduced to 700 px wide, greyscaled,
   and assembled into a single 5×4 contact sheet
   (`work/sheet.png`). The `EX` block and the three chart pages were identified from it.
2. **Re-rendering.** PDF pages 8–10 (and later 20) were re-rendered from the PDF with
   `pdftoppm -r 600 -png`, giving 4961 × 7016 px A4 pages — **600 dpi, twice the supplied
   resolution**.
3. **Pass 1 (reading, whole rows).** Full-width horizontal bands of each chart page were cropped
   at 600 dpi and downsampled to 1500 px wide (≈ 0.30 of native, i.e. ≈ 181 dpi effective) so the
   whole row — number, Function, the P2 legend and Digits — could be read in one view. This pass
   produced the first reading of every column and was the only pass in which the legends were read.
4. **Pass 2 (independent, number and Digits columns only).** The `P1` + `Function` column strip
   (x 465–1275) and the `Digits` column strip (x 4100–4500) were cropped **at native 600 dpi** and
   butt-joined side by side, discarding the legend column entirely. Read in five bands
   (`work/pass2-08.png`, `work/pass2-09-540.png`, `-2560`, `-4580`, `work/pass2-10-540.png`,
   `-2320`). Effective magnification ≈ 3.3× that of pass 1; at this scale a `0`/`8`/`6` and an
   `l`/`1`/`I` are unmistakable — glyph strokes are 6–8 px wide.
5. **Pass 3 (targeted, 600 dpi, up to 300 % enlargement).** Individual cells that pass 1 and pass 2
   flagged as surprising were re-cropped and enlarged: the `0904`/`0905`/`0906` Digits cells at
   300 % (`work/z-0905-digits.png`), the `0904`–`0906` legends (`work/z-0905.png`), the
   `0407`–`0411` legends (`work/z-cwmem.png`), `1507` (`work/z-1507.png`), `0401`
   (`work/z-0401.png`), `0710` (`work/z-0710.png`), and the `EX` block (`work/z-exblock.png`).
6. **Reconciliation.** Pass 1 and pass 2 agreed on **every one of the 159 menu numbers and every
   one of the 159 Digits values**; there was no discrepancy for a third look to settle. The only
   value I deliberately re-examined a third time was `0905`, not because the two passes disagreed
   but because the printed value contradicts its own legend (see Findings 1). Row seams were
   followed continuously: crop bands were overlapped by ~20 px at 600 dpi so the last row of one
   band reappears as the first row of the next, and the page seams were checked by confirming that
   `0508` (last on folio 7) is followed by `0509` (first on folio 8) and `1406` (last on folio 8) by
   `1407` (first on folio 9).
7. **Row count.** 31 + 82 + 46 = **159 rows**, matching the CSV.

Only the ruled table as printed was transcribed. The repeated `P1 | Function | P2 | Digits` page
header was excluded. The `P2` legend column was not transcribed into the CSV; it was read only to
support the findings below.

**Nothing else was consulted.** No text layer was extracted (no `pdftotext`, no copy from the PDF);
no other file in this repository or anywhere else was opened; no directory was listed except the
supplied render directory and my own work directory; no web access was made; and nothing I may
know about other Yaesu radios' menu structures was used. Every value in the CSV was read off a
rendered image of this PDF's printed table.

## Findings

### 1. Rows whose Digits value exceeds 4

| Number | Name | Digits |
|---|---|---|
| `0803` | `OTHER DISP` | **5** |
| `0804` | `OTHER SHIFT` | **5** |

Both are transcribed exactly as printed. Their legends are identical in form:
`-3000 Hz - 0 - +3000 Hz (P2= -3000 - -0000 or +0000 - +3000) (10 Hz/steps)` — a sign character
followed by four digits, which accounts for the printed 5. No other row in the chart prints a
Digits value above 4.

### 2. Free-text / character-entry rows (STOP findings)

**None.** No row in the `EX` menu chart has a legend describing free text: the words
"characters", "character", "ASCII", "call sign", "callsign" or a name-entry instruction do not
appear anywhere in the chart's `P2` column on folios 7, 8 or 9.

One borderline case, recorded so the reader can judge it rather than take my word: rows `0407`
through `0411`, `CW MEMORY 1` … `CW MEMORY 5`, each print the legend `0: TEXT    1: MESSAGE`
(verified at 600 dpi, `work/z-cwmem.png`). This is a two-option selector — the word "TEXT" is the
*name of an option*, not a description of a free-text parameter — and each row prints Digits `1`,
consistent with a single-digit enumerated parameter. I therefore do **not** raise it as a STOP
finding, but it is the only place in the chart where a free-text-sounding word appears at all.

(For completeness: free text does exist elsewhere in this manual — outside the chart — but that is
another command's block on another page and outside this brief's scope; I have not transcribed or
relied on it.)

### 3. Numbers that do not increase, or are duplicated

**None.** The 159 numbers are strictly increasing from `0101` to `1803` with no repeats, verified
by a monotonicity check over the finished CSV. All are printed as four digits with leading zeros
preserved. The sequence is not contiguous — it is a two-level `GG` + `NN` scheme whose groups
restart at `01` (…`0207`, `0301`… ; …`0520`, `0601`… ; …`1204`, `1301`…) — but as printed numbers
each is larger than the one above it.

### 4. Printing defects noticed in the legends

Recorded, not resolved.

1. **`0905` `RPT SHIFT 50MHz` — Digits value contradicts its own legend.** The legend prints
   `0 - 4000 kHz (P2= 0000 - 4000) (10 kHz/step)`, an explicitly four-digit parameter, yet the
   Digits cell prints **`1`**. The row immediately above, `0904` `RPT SHIFT 28MHz`, prints the
   structurally identical legend `0 - 1000 kHz (P2= 0000 - 1000) (10 kHz/step)` and a Digits value
   of `4`. I re-read the `0905` Digits cell three times, the last at 300 % enlargement of the
   600 dpi render (`work/z-0905-digits.png`, PDF page 9 / folio 8, cell at x 4090–4510,
   y 4122–4170): the glyph is unambiguously a `1`. Transcribed as printed. This is the single most
   consequential oddity in the chart and I am not resolving it.
2. **`1507` `EQ3 FREQ` and `1516` `P-EQ3 FREQ` — a range collapsed into an option list.** Both
   legends run `00: OFF   01: 1500 Hz   02: 1600 Hz   03: 1700 Hz   04: 1800 Hz   05: 1900 Hz
   06: 2000 Hz -18: 3200 Hz`. Every other entry is a discrete `NN: value` pair; the final item
   splices a range onto entry `06` with a hyphen set tight against `18` (`Hz -18:`), so the list is
   not parseable by the same rule as its neighbours, and there is no way to tell from the print
   whether the intended reading is `06: 2000 Hz` … `18: 3200 Hz` or something else. Verified at
   600 dpi (`work/z-1507.png`, PDF page 10 / folio 9).
3. **`1504` `EQ2 FREQ` and `1513` `P-EQ2 FREQ` — missing space.** Entry `04: 1000Hz` is printed
   without the space before `Hz` that every other entry in the same legend has (`05: 1100 Hz`,
   `06: 1200 Hz`, …). These are also the two chart rows whose legends wrap onto a second printed
   line, giving them double-height cells.
4. **`1621` `DATA VOX DELAY` — a qualifier present on its twin and absent here.** Its legend prints
   `30 - 3000 msec (P2= 0030 - 3000)`, whereas `1618` `VOX DELAY`, with the identical range and the
   identical Digits value of `4`, prints `30 - 3000 msec (P2= 0030 - 3000) (10 msec/step)`. The
   step qualifier is missing from `1621` only.
5. **`0710` `CW WAVE SHAPE` — option list does not start at zero.** It prints `1: 2 msec
   2: 4 msec`; there is no `0:` entry, unlike every other single-digit enumerated row in the chart,
   which all begin at `0:`. Verified at 600 dpi (`work/z-0710.png`). Not a misread — the cell
   simply has no `0:` item.
6. **`0401` `KEYER TYPE` — verified, not a defect, but easy to misread.** The list prints
   `0: OFF   1: BUG   2: ELEKEY-A   3: ELEKEY-B   4: ELEKEY-Y   5: ACS`. The fourth entry really is
   `ELEKEY-Y`, not `ELEKEY-A` or a damaged `-B`; confirmed at 600 dpi (`work/z-0401.png`). I record
   it because the `-A` / `-B` / `-Y` sequence looks like a misprint at low magnification and is not.

No legend was found containing two identical index numbers, and no option list ran backwards.

### 5. Cells I could not read with confidence

**None.** Every one of the 159 number cells, 159 Function cells and 159 Digits cells was read
cleanly at 600 dpi in both passes, with the two passes in complete agreement. The `0905` Digits
cell is legible beyond doubt — the difficulty there is that the printed value disagrees with the
printed legend, which is a defect in the document rather than a limit on the reading.

## Working files retained

All scratch crops are under
`/private/tmp/claude-501/-Users-stuart-coding-ft710-programmer/f6ea7391-740b-412e-ab12-157eb3ef8363/scratchpad/quarantine/B/work/`:
`sheet.png` (contact sheet), `hi-08.png`–`hi-10.png`, `hi-20.png` (600 dpi renders),
`locate08.png`, `p09a/b.png`, `p10a/b.png` (pass 1), `pass2-*.png` (pass 2),
`z-*.png` (pass 3 targeted enlargements).
