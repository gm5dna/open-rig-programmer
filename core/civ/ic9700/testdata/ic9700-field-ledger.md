# IC-9700 CI-V — memory-record field ledger (leg L)

## Source

- Document title, as printed on the cover (PDF page 1): the black cover band reads
  `CI-V REFERENCE GUIDE`; below it `VHF/UHF ALL MODE TRANSCEIVER` over `IC-9700`,
  with `Icom Inc.` at the foot. No revision code is printed on the cover.
- Revision code, as printed: `A7508-3EX-4`, at the bottom left of the last page
  (PDF page 28), on the line immediately above `© 2019–2023 Icom Inc.   Mar. 2023`.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic9700_civ_rev4.pdf`
- Page count: 28 PDF pages. Confirmed from the renderer itself: page 28 renders,
  and a request for page 29 is refused with "the first page (29) can not be after
  the last page (28)".

## Extent

Rendered: PDF pages 14, 15, 16, 17 at 300 dpi (location sweep); PDF pages 15 and 16
at 400 dpi (first-pass reading) and again at 600 dpi (second-pass reading); PDF
page 1 at 150 dpi and PDF page 28 at 300 dpi (Source section only).

| PDF page | printed folio | contribution |
|---|---|---|
| 1 | none printed | cover title only |
| 14 | 13 | context only — establishes what precedes the material; nothing transcribed |
| 15 | 14 | D1, D2, D3, D4 — all rows transcribed here |
| 16 | 15 | D5 — both rows transcribed here |
| 17 | 16 | context only — establishes what follows; nothing transcribed |
| 28 | none printed | revision code only |

Both transcribed pages carry the running head `Remote control` and the section line
`◇ Command formats (Continued)`.

The material begins on PDF page 15 (folio 14) under the bold bullet heading
`• Memory content` / `Command: 1A 00`. Immediately before it, at the foot of PDF
page 14 (folio 13), is printed: `* When obtaining the edge number (by command
"02"), the edge number (①) is not returned.` — the tail of a different diagram, and
outside this ledger.

The material ends on PDF page 16 (folio 15) with the four-cell block under
`• Band stacking register` / `Command: 1A 01`, upper right. Immediately after that
block comes the grey `NOTE:` panel beginning `When sending the contents, the codes,
such as operating frequency and operating mode*, should be added after the frequency
band code and the register code, as shown below.` Immediately after the transcribed
extent, PDF page 17 (folio 16) opens a different section, `• Memory keyer content` /
`Command: 1A 02`, which is outside the pages this task names.

### Diagrams indexed

| id | printed caption, verbatim | position on page |
|---|---|---|
| D1 | `• Memory content` / `Command: 1A 00` | PDF p15, top of page, full page width, drawn as two cell rows joined by a wrap arrow from the right end of the upper row to the left end of the lower row |
| D2 | (no caption of its own) heading immediately above: `④ Select memory setting` | PDF p15, left column, upper middle — a two-nibble box printed `0` `X` |
| D3 | (no caption of its own) heading immediately above: `⑬ Duplex and Tone settings` | PDF p15, left column, lower — a two-nibble box printed `X` `X` |
| D4 | (no caption of its own) heading immediately above: `⑭ Digital squelch setting` | PDF p15, right column, top — a two-nibble box printed `X` `0` |
| D5 | `• Band stacking register` / `Command: 1A 01` | PDF p16, right column, top — a four-cell block, centred |

Scope note: D5 is a register-content data block, not a memory-channel record. It is
the only numbered data-block diagram printed on PDF page 16, which the task names as
in scope, so it is indexed here; a crosscheck that reads "memory-record" more
narrowly should expect these two rows to be extra rather than missing.

### Conventions used for `label_verbatim`

D1 and D5 print their index numerals bare above the cells — no text is set against
the numerals inside either block. Both pages then print a numbered field list
against those same indices (D1: the two-column list below the strip; D5: the two
headed tables below the NOTE panel). `label_verbatim` takes the label from that
numbered entry, with any second line joined by one space, and stops before the
following `See "…" (p. NN)` cross-reference sentence, which is recorded in `notes`
instead.

D2, D3 and D4 carry a numeral above the box. For D3 and D4 the heading printed
immediately above the box carries the same index and its text is used as the label.
For D2 nothing is printed against the numeral that is actually above the box
(a circled 3) — the heading above it reads `④ Select memory setting` — so the label
cell is empty and STOP 1 is recorded.

`field_index` is recorded with the spacing as printed: a tilde range is printed with
one space either side (`5 ~ 9`), a comma pair with one space after the comma
(`2, 3`). A crosscheck comparing against a leg that strips spaces should compare on
digits plus separator.

## Method

1. Location sweep: `pdftoppm -png -r 300 -f 14 -l 17 <pdf> r300/p`, then each of
   `r300/p-14.png` … `r300/p-17.png` read as an image to find the section whose
   printed heading matches.
2. First-pass reading raster: `pdftoppm -png -r 400 -f 15 -l 16 <pdf> r400/p`
   (3308 × 4678 px per page).
3. ImageMagick was available (`/opt/homebrew/bin/magick`) and used throughout.
   First-pass crops, all `+repage`d and enlarged, e.g.:
   - `magick r400/p-15.png -crop 2700x420+280+800 +repage -resize 200% crops/strip_row1_full.png` (whole D1 strip, both rows)
   - `magick r400/p-15.png -crop 900x120+280+800 +repage -resize 400% crops/r1a.png` (and `+1100`, `+1900` for the rest of the upper index band)
   - `magick r400/p-15.png -crop 750x180+440+1040 +repage -resize 400% crops/r2a.png` (and `+1160`, `+1880` for the lower index band)
   - `magick r400/p-15.png -crop 1420x900+280+1290 +repage -resize 180% crops/legL1.png` (and further windows down both legend columns)
   - `magick r400/p-16.png -crop 1450x480+1690+680 +repage -resize 250% crops/p16_bsr.png` (D5)
4. `pdftotext` was **not** run at all — neither `-layout` nor any other mode. No
   text-layer extraction of any kind was used. `pdfinfo` was run once on this same
   PDF and reported 28 pages and the InDesign metadata title; nothing in the CSV
   comes from it, and the page count in `## Source` was independently confirmed from
   the renderer's own page-range behaviour.
5. `tesseract` was available and used once, as an aid only, on the second-pass crop
   `pass2/legR_mid.png` (`--psm 6`). It mangled every circled numeral (returning
   forms such as `61)`, `62)`, `6)`), so nothing it returned was recorded; each label
   on that crop was confirmed by eye at 600 dpi before being written down. No value
   in the CSV rests on OCR.
6. Directory listings: the only directories listed were the render and crop output
   directories I created myself beneath the evidence path
   (`r300/`, `r400/`, `r600/`, `crops/`, `pass2/`, `cover/`). No directory of the
   repository or of any other material was listed, searched or browsed.

### Second independent pass

Done, over the whole ledger. The second raster differed in three ways at once:
resolution 600 dpi instead of 400 dpi (page 15 rendered 4961 × 7016 px), different
crop windows with different boundaries — the upper index band was cut into four
windows at x = 420/1400/2400/3400 instead of three at x = 280/1100/1900, and the
lower band into three at x = 660/1700/2800 instead of 440/1160/1880, so every group
that had sat at the edge of a first-pass crop sat mid-crop in the second — and
different enlargements (250 %, 220 %, 170 %, 140 % rather than 200 %, 400 %, 180 %).
Each of the three detail boxes and the D5 block was re-cropped separately
(`pass2/box3.png`, `pass2/box13.png`, `pass2/box14.png`, `pass2/bsr.png`), as were
all the legend entries (`pass2/legL_top.png`, `pass2/legL_12.png`,
`pass2/legR_tone.png`, `pass2/legR_upper.png`, `pass2/legR_mid.png`,
`pass2/p16_codes2.png`).

**Disagreements between the passes: none.** Every index numeral, every index style,
every label and every printed position matched. In particular the second pass
independently confirmed, at 600 dpi, that the numeral above the box under
`④ Select memory setting` is a circled **3** (STOP 1), and that the fifth group of
the lower row is two solid black discs with reversed white numerals reading **5** and
**51** (STOP 2). No third render was needed.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** D1 draws
  seventeen of its eighteen index groups as outline-circled numerals and one group,
  the fifth group of the lower row, as white numerals reversed out of solid black
  discs. Recorded as `circled` and `filled` respectively; the two styles are not
  merged, and no meaning is inferred from the styling. The same two styles reappear
  in the grey NOTE panel lower right of PDF page 15, which prints outline-circled and
  filled-disc numerals side by side in the same sentences (`The same data as ⑤ ~ 51
  are stored in ❺ ~ ❺❶.`).
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED.** Every index numeral
  and every label on PDF pages 15 and 16 is set horizontally. (Rotated column labels
  do occur on PDF page 14, outside this ledger.) Position was in any case read from
  the renders, never from a text layer, since `pdftotext` was never run.
- **(c) Leader-line label order reversed — ENCOUNTERED, but it touches no recorded
  value.** No numbered index on these pages is attached by a leader line; all
  numerals sit directly above the cells they mark. Leader arrows do occur inside D2,
  D3 and D4, and in two of them the vertical order of the annotation blocks is
  opposite to the cells they land on: in D4 the word `Fixed`, printed above the enum
  list, points to the **right** nibble while the list below it points to the
  **left** nibble; in D3 the upper enum block (`0=OFF` … `3=DTCS`) points to the
  **right** nibble while the lower block (`0=Duplex OFF` … `3=RPS`) points to the
  **left**. Each was followed by eye from annotation to cell. None of these
  annotations is a field label, so none is recorded in the CSV.
- **(d) Printed index differs from measured position — ENCOUNTERED (repeating
  block), position deliberately not measured.** D1's lower row prints a block whose
  indices, `5 ~ 51` in filled discs, repeat indices already spent by the circled
  groups `5 ~ 9` … `44 ~ 51` earlier in the same strip; the grey NOTE panel states
  that the same data is stored in both. The hazard note asks for a measured byte
  position alongside the printed index, but this task's brief forbids widths and byte
  positions and the CSV schema has no column for one, so only the printed index is
  recorded. What is recorded instead is the block's printed position in the strip: it
  is the fifth of six groups in the lower row, between the `44 ~ 51` brace and the
  `52 ~ 67` brace, and unlike every other group it has no brace of its own and no
  solid cells drawn beneath it — only a long run of dots. The printed index and the
  block's position are recorded as seen and are not reconciled (STOP 2).

## STOP findings

1. **PDF page 15 (folio 14), D2 — detail box index disagrees with its own heading.**
   Left column, upper middle. The heading reads `④ Select memory setting` with a
   circled 4; the two-nibble box printed `0` `X` directly beneath it is captioned
   with a **circled 3**, centred above the box. A circled 3 has already been used, in
   the strip and in the field list, for the second half of `②, ③ Memory channel
   number`. This is an index printed twice for two different things, and an
   out-of-order index within its own heading's scope, so it stops. Transcribed into
   the CSV exactly as seen: `D2,3,circled,` with `STOP 1` in `notes`. Not repaired to
   4. Confirmed on both passes (400 dpi at 180 %, 600 dpi at 220 %).
2. **PDF page 15 (folio 14), D1 — index sequence repeats and runs backwards in the
   lower row.** Reading the lower row left to right, the printed order of the groups
   is `25 ~ 27`, `28 ~ 35`, `36 ~ 43`, `44 ~ 51`, then **`5 ~ 51` in filled discs**,
   then `52 ~ 67`. The fifth group drops from 51 back to 5 and repeats the whole
   range 5–51 already used earlier in the strip, in a different numeral style, and it
   is the only group in the diagram with no field-list entry printed against it and
   no brace. Rows are written in printed order, not numeric order. Transcribed as
   seen: `D1,5 ~ 51,filled,` (empty label) with `STOP 2` in `notes`. Confirmed on
   both passes.

No cell was unreadable: nothing is recorded as `UNREADABLE`.

## Observed disagreements

Recorded as printed; not resolved, and none of these stopped the transcription.

- The grey NOTE panel at the lower right of PDF page 15 is the only text on the page
  that names the filled-disc block, and it names it only in prose: `The same data as
  ⑤ ~ 51 are stored in ❺ ~ ❺❶.` The block has no entry in the numbered field list,
  so its `label_verbatim` cell is empty — nothing is printed there to record.
- Indices ⑬ and ⑭ each appear twice on PDF page 15 in agreement — once over a cell
  pair in the D1 strip and once over their own detail box (D3, D4). Index ③ appears
  three times: in the strip's `②, ③` group, in the field list entry `②, ③ Memory
  channel number`, and above the D2 box, where it does not agree (STOP 1).
- Spacing after the circled numeral is not consistent between the two pages. PDF page
  15 sets its field list with a space — `① Frequency band setting` — while PDF page
  16 sets its two entries tight — `①Frequency band codes`, `②Register codes`. This
  affects no recorded value, because `label_verbatim` records the label text only.
- The `See "…"` cross-reference sentences are set at the left margin, on the line
  after the label they belong to, and one of them (`See "Repeater tone/tone squelch
  frequency setting." (p. 20)`) serves two consecutive entries, ⑮ ~ ⑰ and ⑱ ~ ⑳.
  They are recorded in `notes`, not in `label_verbatim`.
- Within one cross-reference the capitalisation differs from the heading it points
  at: `See "Duplex Offset frequency setting." (p. 13)` for the entry printed
  `㉕ ~ ㉗ Duplex offset frequency setting`.
- In the D1 strip the cell runs for `5 ~ 9`, `28 ~ 35`, `36 ~ 43`, `44 ~ 51` and
  `52 ~ 67` are each abbreviated by a dashed ellipsis cell rather than drawn out in
  full, and the filled `5 ~ 51` block is drawn only as dots. Cell counts are
  therefore not deducible from the picture — consistent with this ledger recording no
  widths or positions.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file,
manual, transcription, source file, generated artefact or web resource was opened,
and no directory was listed.
