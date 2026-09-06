# Transcription B — FT-991A CAT manual menu chart (PDF-primary, visual)

**Date:** 05/09/2026

## Source

- **Document title, as printed on the cover (PDF page 1):**
  `HF / VHF / UHF ALL MODE TRANSCEIVER` / `FT-991A` / `CAT Operation Reference Manual`
  (the model is set as `FT-991` followed by a boxed capital **A** glyph).
- **Running footer on every chart page, as printed:** `FT-991A CAT Operation Reference Book`
  — i.e. the cover says *Manual* and the footer says *Book*. Recorded, not resolved.
- **Revision code, as printed:** `1711-D`
  Found at the **bottom right of PDF page 20**, the unnumbered back cover, below the
  YAESU UK address block. The same page prints `Copyright 2017 / YAESU MUSEN CO., LTD.`
  Verified by a 600 dpi crop enlarged 400 % (`work/rev4.png`) so that `1` and `l`/`I`
  cannot be confused.
- **File:** `docs/fixtures-private/manuals/ft991a_cat_1711-D.pdf`

## Pages used

| Content | PDF page | Printed folio |
|---|---|---|
| Cover (title) | 1 | unnumbered |
| `EX` (MENU) command block + chart rows 001–042 | 8 | 7 |
| Chart rows 043–124 | 9 | 8 |
| Chart rows 125–153 (chart ends; `FA` block follows) | 10 | 9 |
| Back cover (revision code) | 20 | unnumbered |

The `EX` block on PDF page 8 prints `P1 : 001 - 153 (MENU Number)` and
`P2 : Parameter (See Table)`, which matches the 153 chart rows transcribed.

## Method

- Base renders: the supplied 300 dpi page images, used only to locate the chart.
- **Working renders: `pdftoppm -r 600` and `pdftoppm -r 900`** of PDF pages 8–10, plus a
  600 dpi render of page 20 for the revision code.
- Because the parameter (P2) column is not wanted, I built **column composites** with
  ImageMagick: the P1 (number) strip appended directly to the Function strip and to the
  Digits strip, dropping the P2 column. This puts the number and the value being read
  side by side in one image with no intervening text.
- **Magnification actually read:** the 600 dpi composites were viewed at native size
  (≈ 67 px row pitch, cap height ≈ 30 px); the 900 dpi number+Digits composites were
  viewed at native size (≈ 100 px row pitch, cap height ≈ 45 px). At 900 dpi a `0`, `6`
  and `8` are unmistakable, as are `1`, `l` and `I`. Individual doubtful cells were
  additionally cropped at 900 dpi and, for the revision code, at 600 dpi × 400 %.
- Ruled rows were followed continuously across the two page seams (042→043 at the
  foot of folio 7 / head of folio 8; 124→125 at the foot of folio 8 / head of folio 9).
  Neither seam splits a row and neither repeats one; the repeated
  `P1 | Function | P2 | Digits` header row at the top of folios 8 and 9 is **not**
  transcribed as a chart row.
- **Pass 1** (600 dpi): number + Function + Digits together, chart order, all three pages.
- **Pass 2** (900 dpi, independent, different chunk boundaries, Function column hidden):
  number + Digits only, all three pages.
- **Pass 3** (900 dpi, Function column, whole chart): run after pass 2 exposed a name
  discrepancy — see Discrepancies below.
- Scratch crops written only under
  `/private/tmp/claude-501/-Users-stuart-coding-ft710-programmer/5cb2182d-0a2a-4696-ac2b-c1e34bfcbd2a/scratchpad/quarantine/991-B/work/`.
- **Nothing else was consulted.** No text-layer extraction (no `pdftotext`, no copy/paste
  from the PDF), no other file in this repository or anywhere else, no filesystem search,
  no web access, and nothing recalled about any other Yaesu radio's menu. Every value
  below was read off the printed ruled table in the rendered images.

## Reconciliation of the two number/Digits passes

- **Number column:** both passes read `001` … `153`, strictly increasing by one, no gaps
  and no repeats. **No discrepancy.**
- **Digits column:** both passes agreed on all 153 cells, including the five values
  above 4 and the non-numeric cell at 087. **No discrepancy.**
- **Discrepancy found outside the two required passes:** at 600 dpi (pass 1) I read row
  088 as `GM DISPLAY`. The 900 dpi crop of the same row reads `GM DISPLY`. A **third
  look** — a dedicated 900 dpi crop of rows 087–089 (`work/v088b.png`) and then a full
  900 dpi third pass over the Function column of all three pages — settled it: the chart
  prints **`GM DISPLY`**, with no `A` before the `Y`. The CSV carries the printed form.
  This is the only cell where the passes differed, and it is why pass 3 was run over the
  whole Function column rather than just that row; pass 3 confirmed every other name.

## Findings

### 1. Rows whose Digits value exceeds 4

| Number | Name | Digits |
|---|---|---|
| 027 | TIME ZONE | 5 |
| 064 | OTHER DISP (SSB) | 5 |
| 065 | OTHER SHIFT (SSB) | 5 |
| 083 | RPT SHIFT 430MHz | 5 |
| 151 | PRESET FREQUENCY | 8 |

All five transcribed exactly as printed.

### 2. STOP findings

**(a) Free-text parameter legends: none.**
No row's P2 legend uses "characters", "ASCII", or describes a call sign or name entry.
The closest candidate, 087 RADIO ID, prints no wording at all (see (b)); rows 018–022
CW MEMORY 1–5 print `0: TEXT    1: MESSAGE`, which is a two-way choice of *which* stored
item to send, not a free-text field, and each carries Digits `1`.

**(b) Unreadable Digits cell — 087 RADIO ID — STOP FINDING.**
Row 087's Digits cell does **not** print an integer: it prints a **single hyphen `-`**.
Its P2 legend likewise prints no parameter description, only **ten spaced hyphens**,
`- - - - - - - - - -`. Both were confirmed at 900 dpi (`work/z087d.png` for the Digits
cell, `work/z087leg.png` for the legend). Per the brief the Digits field is written as
`?` in the CSV. Page: PDF 9 / folio 8. The row is transcribed as printed in every other
respect (`087,RADIO ID,?`). Note for downstream: this row therefore carries **no printed
field width at all**, so `EX087` cannot be sized from this chart.

### 3. Numbering monotonicity

The first column runs `001` to `153` with no duplicate and no decrease. Nothing to report.

### 4. Printing defects noticed in the legends while locating rows

Recorded, not resolved.

1. **Row 100 RTTY SHIFT FREQ — two identical index numbers in one legend.** The legend
   prints `1: 170 Hz    1: 200 Hz    2: 425 Hz    3: 850 Hz`. Index `1` appears twice and
   index `0` never appears, so the four options carry only three distinct indices.
   Confirmed at 900 dpi (`work/z100.png`). PDF page 9 / folio 8.
2. **Rows 068 / 069 — Digits values transposed relative to their own legends.**
   068 DATA HCUT FREQ has a two-digit legend (`00: OFF   01: 700 Hz ~ 67: 4000 Hz
   (50 Hz steps)`) but prints Digits `1`; 069 DATA HCUT SLOPE has a one-digit legend
   (`0: 6 dB/oct   1: 18 dB/oct`) but prints Digits `2`. Every other LCUT/HCUT
   FREQ/SLOPE pair in the chart (041/042, 043/044, 050/051, 052/053, 066/067, 092/093,
   094/095, 102/103, 104/105) prints FREQ = 2 and SLOPE = 1. Both cells were read the
   same way in both independent passes, so this is what the page prints. PDF page 9.
3. **Row 028 GPS/232C SELECT — gap in the option list.** Prints
   `0: GPS1   1: GPS2   3: RS232C`: index `2` is absent, so the list is not contiguous.
   PDF page 8 / folio 7.
4. **Row 083 RPT SHIFT 430MHz — range printed without its zero-padded form while its
   neighbours print one.** The legend reads `0 ~ 10000 kHz (P2 = 0000 ~ 10000,
   10 kHz/step)`: the low end of the P2 range is padded to four digits (`0000`) while the
   high end is five (`10000`) and the Digits cell says `5`. The sibling rows 080–082 print
   `P2 = 0000 ~ 1000` / `0000 ~ 4000` against Digits `4`, i.e. consistently padded.
   PDF page 9.
5. **Rows 072 and 077 — option lists that start at 1 where their siblings start at 0.**
   072 DATA PORT SELECT and 077 FM PKT PORT SELECT both print `1: DATA   2: USB`, whereas
   048 AM PORT SELECT and 109 SSB PORT SELECT print `0: DATA   1: USB`. All four carry
   Digits `1`. PDF page 9.
6. **Row 116 SCP SPAN FREQ — option list starts at 03.** Prints `03: 50 kHz  04: 100 kHz
   05: 200 kHz  06: 500 kHz  07: 1000 kHz`; indices 00–02 are not printed. PDF page 9.
7. **Row 061 QSK DELAY TIME — misspelling in the legend.** Prints
   `0: 15 msec   1: 20 msec   2: 25 mesc   3: 30 msec` — the third option reads `mesc`.
   PDF page 9.
8. **Row 088 GM DISPLY — misspelling in the Function column** (see Reconciliation above).
   Transcribed as printed. PDF page 9.
9. **Minor typographic inconsistencies in the legends** (noted for completeness):
   092 and 094 print `1000Hz` / `4000Hz` with no space before `Hz`, whereas the
   equivalent rows 041, 043, 050, 052, 066, 068, 102 and 104 print `1000 Hz` / `4000 Hz`;
   and 119, 125, 128 and 134 print `00 : OFF` with a space before the colon whereas
   122 and 131 print `00: OFF`.

### 5. Cells that could not be read with confidence

Only one: the **Digits cell of row 087 RADIO ID**, PDF page 9 / folio 8, crop
`work/z087d.png` (900 dpi). It is not illegible — it is legibly a hyphen — but it is not
an integer, so it is recorded as `?` in the CSV and as a STOP finding at 2(b) above.

Every other cell in the number, Function and Digits columns was read cleanly at 900 dpi
and agreed across the passes described above.
