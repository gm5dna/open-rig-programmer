# Provenance --- frame-geometry witness, FT-891 CAT manual

**Derivation date:** 04/09/2026

**Hardware status:** UNVERIFIED for every vector in all four `.golden` files. No
frame below has been sent to or received from an FT-891.

## Manual

| Item | Value | Where found |
| --- | --- | --- |
| Title | FT-891 CAT Operation Reference Book | cover, PDF page 1; and the running footer of every content page |
| Publisher | YAESU MUSEN CO., LTD. | cover, PDF page 1 |
| Copyright | 2019 | back cover, PDF page 20 |
| Printed revision | **1909-C** | back cover, PDF page 20, bottom-right corner. The front cover carries no revision mark. |
| Extent | 20 PDF pages: cover (PDF 1), printed folios 1–18 (PDF 2–19), back cover (PDF 20). PDF page = printed folio + 1 throughout. | folios read off the page footers |

## Pages read

| PDF page | Printed folio | What was taken from it |
| --- | --- | --- |
| 1 | (cover) | title, publisher |
| 2 | 1 | confirmed the folio/PDF offset |
| 3 | 2 | "Control Command" prose: command = 2 alphabetical characters, parameters, terminator ";"; the note on filling inapplicable parameter digits ("any character except the ASCII control codes (00 to 1Fh) and the terminator (;)") |
| 4 | 3 | the Control Command List: the Set/Read/Ans./AI row for EX, MR, MT, MW |
| 5 | 4 | chart-layout convention (10 numbered positions per printed row); AM block title |
| 7 | 6 | Table 1 (CTCSS Tone Chart), Table 2 (DCS Code Chart) — noted, not used |
| 8 | 7 | **EX command block**; start of the menu chart (0101 … 0508) |
| 9 | 8 | menu chart continued (0509 … 1406); the two Digits = 5 rows |
| 10 | 9 | menu chart end (1407 … 1803); FA/FB blocks, from which the only printed frequency range in the manual was inherited |
| 12 | 11 | MC block (PMS spelling); MD block title |
| 13 | 12 | **MR command block**; **MT command block** |
| 14 | 13 | **MW command block**; NB block title |
| 19 | 18 | last content page (ends at ZI); VM, VX, UL, TX block titles; the duplicated-terminator oddity |
| 20 | (back cover) | printed revision 1909-C, copyright |

## Method

- Pages were read from the supplied 300 dpi renders (`p-01.png` … `p-20.png`) for
  location and for the wide menu chart.
- Every chart that supplied a counted width was **re-rendered at 600 dpi**
  (`pdftoppm -r 600 -f N -l N -png`) and cropped strip-by-strip with `magick`,
  because at 300 dpi the numbered position cells of the MT (41 cells), MR (30
  cells), MW (30 cells) and EX charts interleave with the multi-line legend column
  and the digits are not unambiguous. Crops were written only under
  `.../quarantine/G/work/`.
- **Two independent counts of every chart.** Pass 1: left-to-right along each
  printed row (1–10, 11–20, 21–30, 31–40, 41). Pass 2: by per-parameter cell
  totals, summed. Both passes agreed on every chart; no third look was needed.
  - MT Set: pass 1 → 41; pass 2 → 2 + 3 + 9 + 1 + 4 + 1 + 1 + 1 + 1 + 1 + 2 + 1 + 1 + 12 + 1 = 41. ✔
  - MW Set: pass 1 → 28; pass 2 → 2 + 3 + 9 + 1 + 4 + 1 + 1 + 1 + 1 + 1 + 2 + 1 + 1 = 28. ✔
  - MR Answer: pass 1 → 28; pass 2 → same sum as MW = 28. ✔
  - MR Read: pass 1 → 6; pass 2 → 2 + 3 + 1 = 6. ✔
  - MT Read: pass 1 → 6; pass 2 → 2 + 3 + 1 = 6. ✔
  - EX Read: pass 1 → 7; pass 2 → 2 + 4 + 1 = 7. ✔
- The EX menu chart's **Digits column** was counted a second, independent way: the
  column was cropped out of each of the three chart pages and rotated 90°, so the
  whole column could be read in one pass without row-tracking error. Both readings
  agree: maximum 4 on folio 7, maximum **5** on folio 8 (rows 0803 and 0804 only),
  maximum 4 on folio 9.
- **No width was inferred from a legend.** Where a legend range happens to imply a
  width (e.g. EX "0101 - 1803"), the width recorded is still the counted cell
  count.
- **Nothing else was consulted.** No text-layer extraction (`pdftotext` or
  equivalent) was run; no other file, directory listing, filesystem search, web
  access, code, generator, or recollection of any other radio's frames was used.
  The only commands run were `pdftoppm`/`magick` renders and crops, the writes of
  the five output files, and one `awk` byte-count over the four `.golden` files I
  had just written — a self-check on my own output that read no other file and
  produced no frame content.

## Per-vector-class record

### MT — MEMORY WRITE & TAG

- **Chart used:** the MT **Set** chart (folio 12, PDF 13), because the vectors are
  Set-direction. Cross-checked against the MT **Answer** chart in the same block,
  which prints an identical 41-position layout.
- **Counted frame length:** **41** bytes, terminator included.
- **Field map:** 1 `M` · 2 `T` · 3–5 `P1` (slot) · 6–14 `P2` (frequency, 9) ·
  15 `+/-` · 16–19 `P3` (clarifier offset, 4) · 20 `P4` (CLAR off/on) ·
  21 `P5` (fixed) · 22 `P6` (mode) · 23 `P7` (fixed) · 24 `P8` (CTCSS) ·
  25–26 `P9` (fixed) · 27 `P10` (shift) · 28 `P11` (TAG off/on) ·
  29–40 `P12` (tag, 12) · 41 `;`
- **Manual-documented bytes:** positions 1–2, 21, 23, 25–26, 41 have their value
  fixed by the chart or the legend. For every other position the legend prints the
  legal set; the value chosen is mine from that set.
- **INHERITED-ASSUMED bytes:**
  1. *Tag padding*, positions 29–40 of the short-tag and cleared-tag vectors.
     Assumed ASCII space `0x20`, trailing (tag left-justified). The chart fixes 12
     tag cells; the legend says only "up to 12 characters"; the manual never says
     what fills the rest or on which side.
     **Settling capture:** one MT read (`MT001;`) of a channel whose tag was
     entered from the front panel as fewer than 12 characters — the reply's
     positions 29–40 show the pad byte and the justification in one shot.
  2. *Tag character subset*, positions 29–40 of all six vectors. Legend says only
     "(ASCII)". Assumed upper-case A–Z and digits.
     **Settling capture:** one MT write of a tag containing a lower-case letter and
     one punctuation character, then one MT read back.
  3. *Frequency range*, positions 6–14. The MT legend prints only "Frequency (Hz)".
     Inherited the FA/FB range "000030000 - 056000000 (Hz)" printed on folio 9.
     **Settling capture:** one MT write at each end of the claimed range, then one
     MT read back.
  4. *Clarifier consistency*, positions 15–20. Assumed "+0000" when P4 = "0".
     **Settling capture:** one MT read of a channel stored with the clarifier off —
     shows what the radio itself puts in positions 15–19.
  5. *Combination legality* (mode × shift × CTCSS). Assumed accepted.
     **Settling capture:** one MT write of the 51 MHz FM minus-shift CTCSS vector
     followed by one MT read back — a differing reply, or no stored channel, falsifies it.
  6. *Slot names MT accepts.* The MT P1 legend lists only 001–099 and P1L–P9U; no
     MT vector uses 501–510 or EMG (which MR's legend does list).
     **Settling capture:** one MT write to slot 501 on a U.K.-version radio, then one
     MT read of 501.

### MW — MEMORY CHANNEL WRITE

- **Chart used:** the MW **Set** chart (folio 13, PDF 14) — the only filled chart in
  the block; its Read and Answer value rows are printed blank. Because there is no
  second filled chart inside the block, the count was cross-checked against the MR
  **Answer** chart on folio 12, which prints the same 28-position layout with the
  same parameter labels. That is a cross-command check, not a within-block one.
- **Counted frame length:** **28** bytes, terminator included.
- **Field map:** identical to MT positions 1–27 with `W` in place of `T`, then
  28 `;`. There is no tag: the chart stops at 28 and the legend has no P11/P12.
- **Manual-documented bytes:** positions 1–2, 21, 23, 25–26, 28.
- **INHERITED-ASSUMED bytes:** items 3, 4, 5 and 6 above, identically (frequency
  range; clarifier consistency; combination legality; slot names). Same settling
  captures, with `MW` in place of `MT` for the write and `MR` for the read-back.

### MR — MEMORY CHANNEL READ

- **Charts used:** the MR **Read** chart for the request frames and the MR **Answer**
  chart for the answer frames (folio 12, PDF 13). The MR **Set** chart is printed
  with a blank value row.
- **Counted frame lengths:** Read request **6** bytes; Answer **28** bytes.
- **Field maps:** Read — 1 `M` · 2 `R` · 3–5 `P0` · 6 `;`.
  Answer — as MW, with `R` in place of `W`.
- **Manual-documented bytes:** Read — all six (positions 3–5 take one of the four
  printed slot forms). Answer — positions 1–2, 21, 25–26, 28.
- **INHERITED-ASSUMED bytes (Answer only; the Read requests contain none):**
  1. *Every data byte of an Answer is a prediction* — the manual prints no worked MR
     example anywhere, so the two answer frames are hand-derived shapes filled with
     legend-legal values, not observed replies.
     **Settling capture:** one `MR001;` against a channel programmed to known
     content — the whole 28-byte reply is settled at once.
  2. *P7 value*, position 23. Legend prints "0: VFO  1: Memory"; assumed `1`.
     **Settling capture:** the same single `MR001;` reply, position 23.
  3. *Frequency range*, positions 6–14 (inherited from FA/FB as above).
  4. *Clarifier consistency*, positions 15–20 (as above).
  5. *What slots 501–510 and EMG return.* Not printed; "501 - 510" is qualified
     "(5 MHz, U.S. and U.K. version only)".
     **Settling capture:** one `MR501;` and one `MREMG;` on a U.K.-version radio.

### EX — MENU

- **Chart used:** the EX **Read** chart (folio 7, PDF 8), because the vectors are
  Read-direction requests. The menu item numbers and Digits values come from the
  P1/Function/P2/Digits chart on folios 7–9 (PDF 8–10).
- **Counted frame length:** **7** bytes, terminator included — fixed for every menu
  item, because a Read request carries no P2.
- **Field map:** 1 `E` · 2 `X` · 3–6 `P1` (menu number, 4 counted cells) · 7 `;`
  (positions 8–10 printed empty).
- **Manual-documented bytes:** all seven.
- **INHERITED-ASSUMED bytes:** none in the four vectors. The Answer *shape* recorded
  as a comment in `ex-vectors.golden` does rest on one assumed reading — that the
  Set/Answer charts' open-ended `n` equals `6 + Digits + 1`, i.e. that the Digits
  column gives P2's width on the wire. The chart contradicts that reading in at
  least one row (see disagreement 4 below).
  **Settling capture:** one `EX0803;` and one `EX0905;` — the two replies show
  whether the Digits column really is the wire width, and settle the 0905 conflict
  at the same time.
- **Largest Digits value in the chart:** **5**, at rows **0803 OTHER DISP** and
  **0804 OTHER SHIFT** (folio 8). Every other row in the whole chart prints 1, 2, 3
  or 4. `EX0803;` is the vector chosen for that case.
- **First group on the chart:** the 01xx group; first row **0101 AGC FAST DELAY**.
  **Last group on the chart:** the 18xx group; last row **1803 LCD VERSION**.

## Disagreements found inside the manual — recorded, not resolved

1. **MT direction support: command list versus detail block.**
   - Command list, folio 3 (PDF 4), verbatim row:
     `MT | MEMORY WRITE & TAG | Set O | Read X | Ans. X | AI X`
   - MT detail block, folio 12 (PDF 13): the Set chart is filled (41 positions);
     the **Read chart is printed and filled**: `M T P0 P0 P0 ;`; the **Answer chart
     is printed and filled**, an identical 41-position layout to Set.
   The list says Set only; the block prints all three directions. Not resolved.

2. **The MT block's Read chart uses a parameter its own legend never defines.**
   The MT Read chart's positions 3–5 are labelled `P0`, but the MT legend column
   contains no `P0` entry — it starts at `P1`. The only `P0` legend anywhere is the
   MR block's combined `P0/1` entry on the same printed page. Not resolved.

3. **Two different legends for the same chart position 23.**
   - MR block, position 23: `P7  0: VFO  1: Memory`
   - MT block, position 23: `P7  0: (Fixed)`
   - MW block, position 23: `P7  0: (Fixed)`
   Same position in charts that are otherwise identical field-for-field. Not resolved.

4. **EX menu chart, row 0905: P2 range versus Digits column.**
   Verbatim: `0905 | RPT SHIFT 50MHz | 0 - 4000 kHz (P2= 0000 - 4000) (10 kHz/step) | 1`
   The P2 text gives a four-digit parameter; the Digits column prints `1`. The
   neighbouring row is internally consistent:
   `0904 | RPT SHIFT 28MHz | 0 - 1000 kHz (P2= 0000 - 1000) (10 kHz/step) | 4`.
   Not resolved.

5. **Slot-range legends differ between MR and MT/MW.**
   - MR `P0/1`: `001 - 099 (Regular Memory Channel)` / `P1L - P9U (PMS)` /
     `501 - 510 (5 MHz, U.S. and U.K. version only)` / `EMG (Emergency)`
   - MT `P1` and MW `P1`: only `001 - 099 (Regular Memory Channel)` / `P1L - P9U (PMS)`
   - MC `P1` (folio 11): only `001 - 099: Regular Memory Channel` / `P1L - P9U (PMS)`
     — and note MC prints a colon after "099" where MR/MT/MW use parentheses.
   Whether a slot readable by MR is writable by MT/MW is therefore unstated. Not resolved.

6. **Mode legend typography differs between blocks.** MR prints `MODE  1:LSB`
   (no space) where MT and MW print `MODE  1: LSB`; the remaining entries match
   value-for-value in all three. Cosmetic, but recorded because the two legends for
   one parameter are not byte-identical.

7. **Command-list function names disagree with detail-block titles**, for MR and MW
   among others. Observed pairs (list → block):
   - `MR  MEMORY READ` → `MR  MEMORY CHANNEL READ`
   - `MW  MEMORY WRITE` → `MW  MEMORY CHANNEL WRITE`
   - `MD  MODE` → `MD  OPERATING MODE`
   - `DN  DOWN` → `DN  MIC DWN`
   - `NB  NOISE BLANKER` → `NB  NOISE BLANKER STATUS`
   - `UL  UNLOCK` → `UL  PLL UNLOCK STATUS`
   - `VX  VOX` → `VX  VOX STATUS`
   - `VM  [V/M] KEY FUNCTION` → `VM  VFO-A TO MEMORY CHANNEL` (which is also the
     title the `AM` block carries on folio 4)
   - the list itself prints `OI  OPPOSITE BAND NFORMATION` (no `I`)
   EX and MT titles agree between list and block. Not resolved.

8. **Charts elsewhere in the manual print two terminators in one Set row.**
   Verified at 600 dpi on folio 18 (PDF 19): the `VX` Set chart prints
   `V | X | P1 | ; | (empty) | ;` — a `;` in position 4 *and* in position 6. The
   `VM` and `ZI` Set charts on the same folio show the same doubled terminator.
   None of the four commands in this witness is affected, but it is direct evidence
   that these charts do contain printing errors, which bears on how much weight
   disagreement 1 should carry. Not resolved.

## Notes on value choices

- No real call sign appears in any tag. Tags used: `FORTYMETERS1`, `TWENTY`,
  `PMSLOWEREDGE`, `FMREPEATER01`, and one all-pad (cleared) tag.
- The PMS slot is written `P1L`, using the legend's own spelling `P1L - P9U (PMS)`.
- **No 145 MHz vector exists** in these files: the only frequency range this manual
  prints anywhere is FA/FB's `000030000 - 056000000 (Hz)` on folio 9, and 145 MHz
  falls outside it. The FM/tone/shift case therefore uses 51.000000 MHz.

## Output files

- `mt-vectors.golden` — 6 MT Set frames, 41 bytes each
- `mw-vectors.golden` — 3 MW Set frames, 28 bytes each
- `mr-vectors.golden` — 4 MR Read requests (6 bytes) + 2 MR Answer frames (28 bytes)
- `ex-vectors.golden` — 4 EX Read requests, 7 bytes each
- `provenance.md` — this file
