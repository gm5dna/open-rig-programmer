# IC-R8600 memory-record field ledger — leg L

Companion to `IC-R8600-field-ledger.csv` (87 rows, 35 diagrams, PDF pages 12–15).

## Source

- Document title, as printed on the cover (PDF page 1): the black cover panel
  carries the Icom logo and, in the black band beneath it, **CI-V REFERENCE
  GUIDE**; below the panel, on the ruled cover block, **COMMUNICATIONS
  RECEIVER** over **IC-R8600**, and **Icom Inc.** at the foot.
- Revision code, as printed: **A7375-2EX-3a**, at the foot of the left column of
  the last page (PDF page 28, unnumbered), immediately above
  `© 2017–2018 Icom Inc.`
- File: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/icr8600_civ_3a.pdf`
- Page count: 28 PDF pages (A4, 595.276 × 841.89 pt).
- Folio relation, verified by eye: the printed folio is the PDF page number minus
  one (PDF 11 → folio 10, PDF 12 → 11, PDF 13 → 12, PDF 14 → 13, PDF 15 → 14).

## Extent

| PDF page | printed folio | rendered at | read | what it contributed |
|---|---|---|---|---|
| 1 | none | 300 dpi | yes | cover title, model, publisher |
| 2 | none | 300 dpi | no | rendered while locating front matter; never opened |
| 11 | 10 | 300 dpi | yes | context only — establishes the folio offset and what is printed immediately before the transcribed material |
| 12 | 11 | 300, 400, 600 dpi | yes | D1–D9 (Memory channel content, 1A 00) |
| 13 | 12 | 300, 400, 600 dpi | yes | D10–D17 (FM, D-STAR, P25 tails) |
| 14 | 13 | 300, 400, 600 dpi | yes | D18–D28 (dPMR, NXDN tails) |
| 15 | 14 | 300, 400, 600 dpi | yes | D29–D35 (DCR tail; clear format; Programmable scan start data) |
| 16 | not read off the render | 300, 400 dpi | yes (top band only) | context only — establishes what is printed immediately after the transcribed material |
| 28 | none | 300 dpi | yes | revision code |

**Where the transcribed material begins.** On PDF page 12, under the running head
`Remote control` and the section line `◇ Command formats (Continued)`, at the
bullet heading `●Memory channel content` / `Command: 1A 00`. What is printed
immediately before it, on PDF page 11 (folio 10), is the
`●Scope/FSK FFT Scope waveform/FSK font color` figure at the foot of the left
column and the `●Character entries` table in the right column.

**Where it ends.** At the foot of the right column of PDF page 15, with the last
item of the `●Programmable scan start (remote) data` list:
`㉕ IP plus (IP+) function` / `Refer to "IP plus (IP+) function (㉕)" (p. 11)`.
What is printed immediately after it, at the top of PDF page 16, is
`●Programmable scan start (remote) data (continued)` with the sub-headings
`For receiving an SSB/CW/FSK/AM/WFM signal` and `For receiving a P25 signal`.
PDF page 16 is outside the pages this leg was given and nothing from it is in the
CSV.

## Method

Every value in the CSV was read from a rendered page image of this PDF. Nothing
was read from a text layer.

1. **Locate — 300 dpi.** Fresh directory
   `…/legs-out/icr8600/L/r300`, created for this leg and empty beforehand:

   ```
   pdftoppm -png -r 300 -f 1  -l 1  <pdf> r300/p
   pdftoppm -png -r 300 -f 11 -l 16 <pdf> r300/p
   pdftoppm -png -r 300 -f 28 -l 28 <pdf> r300/p
   pdftoppm -png -r 300 -f 2  -l 2  <pdf> r300/p
   ```

   These whole-page images were used only to find the sections whose printed
   headings match the task (`●Memory channel content`, the five
   `For receiving a … signal` sub-headings, and
   `●Programmable scan start (remote) data`) and to confirm the folio offset.

2. **Read — 400 dpi (pass 1).** `pdftoppm -png -r 400 -f 12 -l 16 <pdf> r400/p`
   (3308 × 4678 px per page). Every value recorded in the CSV was first read
   from these rasters and their crops.

3. **Crop and enlarge.** ImageMagick 7 was available (`/opt/homebrew/bin/magick`)
   and used throughout. Each numbered band, byte block, legend column and detail
   box was cropped into `crops/` with commands of the form

   ```
   magick r400/p-12.png -crop 1350x800+380+800   +repage -resize 200% crops/p12_block.png
   magick r400/p-12.png -crop 1150x620+1700+1690 +repage -resize 250% crops/p12_ptsbox2.png
   magick r400/p-13.png -crop 1000x260+400+1010  +repage -resize 250% crops/p13_fmblock_all.png
   magick r400/p-14.png -crop 1500x800+300+3200  +repage -resize 220% crops/p14_scrmkey2.png
   ```

   Enlargements of 200–400 per cent were used until every numeral, rule, arrow
   head and leader line stood clear of its neighbours.

4. **`pdftotext` was never run.** No text-layer extraction of any kind was
   performed on this PDF or on anything else, so the first form of the
   attestation below is the true one. `pdfinfo` was run once on this same PDF,
   for its page count and page size only; it returns no page content. No source
   directory was browsed, searched or listed: the only `ls` calls made were on
   the render and crop directories this leg itself created beneath
   `…/legs-out/icr8600/L`, which the brief places inside the permitted
   workspace, and they were made only to confirm that the renders had been
   written.

5. **`tesseract` was available but was not used.** Every glyph was legible by eye
   at 400 dpi enlarged, so no OCR aid was needed and none was run. No value in
   the CSV came from OCR.

6. **Second independent pass — 600 dpi (pass 2).**
   `pdftoppm -png -r 600 -f 12 -l 15 <pdf> r600/q` (4961 × 7016 px per page), a
   different raster at a different resolution, cropped into a separate directory
   `crops2/` with different crop windows and different enlargements (150–220 per
   cent, against 200–400 per cent in pass 1) — for example

   ```
   magick r600/q-12.png -crop 2100x300+560+1290 +repage -resize 150% crops2/p12_row1.png
   magick r600/q-15.png -crop 2300x180+2620+1330 +repage -resize 220% crops2/p15_psband1.png
   magick r600/q-13.png -crop 1600x900+2570+2620 +repage -resize 180% crops2/p13_nac.png
   ```

   Re-read in pass 2: every index token on all eight byte-block bands (D1, D10,
   D12, D15, D18, D24, D29, D35); the byte-cell counts of all eight blocks; the
   range/pair punctuation on the two multi-row blocks (D1 and D35); and the
   leader-to-cell mapping of the two boxes where the mapping carries the most
   risk — D5 (Programmable tuning step) and D17 (NAC).

   **Result: the two passes agree in every cell. There is no disagreement to
   record, and no third render was needed.** In particular pass 2 independently
   confirmed the wave dash and ideographic comma on the D35 band (STOP 1), the
   reversed leader stacks in D5 and D17 (hazard c), and the printed spelling
   `100th posiiton` in D17 (STOP 4).

### How the CSV was populated

- **A diagram** is one drawn figure. Two kinds appear, and both are in the CSV:
  a **record-layout figure** (a numbered band of circled indices over a run of
  byte cells, with its numbered item list beneath — D1, D10, D12, D15, D18, D24,
  D29, D35), and a **detail box** (the small enlarged box drawn under one item
  heading, with leader lines into its cells — all other Dn). D34 is neither: it
  is the numbered list of the `1A 00` clear format, which has no byte block; it
  is included because its entries are numbered fields of the same memory record.
- `diagram_id` runs D1…D35 in page order, and within a page in reading order:
  left column top to bottom, then right column top to bottom. Each id is defined
  by its printed caption verbatim and its position in the `visual_anchor` of
  every row that belongs to it.
- `field_index` is the token exactly as drawn on that diagram, never normalised.
  For a record-layout figure it is the token on the band over the byte block;
  where the item list beneath prints the same field with different punctuation,
  the band form is used and the difference is a STOP (STOP 1).
- `label_verbatim` is the label printed against that index: for a record-layout
  figure, the text of the item-list entry keyed by that same index token (index
  token stripped, since it is already in `field_index`); for a detail box, the
  heading printed immediately above the box. The per-cell leader annotations are
  **not** put in `label_verbatim` — they are recorded in `notes` as
  `cell leaders L-R: …`, in the left-to-right order of the cells they point at,
  with ` | ` between cells and ` / ` between stacked lines against one cell.
  This keeps the drawn cell order, which is the evidence hazard (c) is about.
- No width, byte position, offset, address, total or encoding was derived or
  recorded, as the task requires.

### Conflicts inside the brief, and how they were resolved

Two instructions in the brief contradict other instructions in the same brief.
Both were resolved in favour of the more specific, and both are recorded here
rather than acted on silently.

1. The Tier 4b clause asks for the mode classes to be "joined by a `mode_class`
   column", but the CSV rules require "the header line exactly as given (no
   spaces after commas, **no extra columns**, no reordering)" and name
   `diagram_id` + `field_index` as the crosscheck's join key. The mandated
   header was written unchanged and **no `mode_class` column was added**. Every
   mode class is instead carried by its own `diagram_id`, defined by the printed
   caption in the `visual_anchor` of each row: FM = D10/D11; D-STAR = D12–D14;
   P25 = D15–D17; dPMR = D18–D23; NXDN = D24–D28; DCR = D29–D33. Every mode
   class the document draws as its own row set is transcribed as its own row
   set; none is merged.
2. Hazard (d) asks, where a block of fields repeats another, for "BOTH the
   printed index … AND the byte position you measured", but the task statement
   says "no widths, no byte positions". No byte position was measured or
   recorded. See `## Hazards encountered` (d).

## Hazards encountered

- **(a) Numeral styling varying within one diagram — NOT ENCOUNTERED.** All 87
  field indices on PDF pages 12–15 are drawn identically: a plain numeral inside
  a thin outline circle, no fill, no reversal, no bracket, no bold. Every row is
  therefore `circled`, and none was normalised to reach that. The only other
  circled glyph on these pages is the solid `ⓘ` information mark that opens the
  two notes in the left column of page 12; it is not a field index and no row is
  made from it.
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED.** No rotated or
  vertically set label appears anywhere on PDF pages 12–15; all text on these
  pages is horizontal. (A rotated-label block does exist earlier in the document,
  in the `Scope/FSK FFT Scope waveform/FSK font color` figure on PDF page 11, but
  that page is outside the transcribed material and contributes no row.) No text
  layer was consulted at any point, so extraction order never entered the
  reading.
- **(c) Leader-line label order reversed — ENCOUNTERED.** In ten detail boxes the
  stacked leader labels are printed in the reverse of the left-to-right order of
  the cells their leaders land on: D5 (Programmable tuning step, page 12), D14
  (CSQL code, page 13), D17 (NAC, page 13), D20 (COM ID, page 14), D21 (CC, page
  14), D23 (Scrambler key, page 14), D26 (RAN code, page 14), D28 (Encryption
  key, page 14), D31 (UC code, page 15), D33 (Encryption key, page 15). Each was
  resolved by following every leader by eye from its label back to the cell it
  points at, at 400 dpi and again independently at 600 dpi; the two passes agree.
  The `notes` of those rows record the labels in drawn cell order and say that
  the printed list runs the other way. D5 is the clearest case: the box reads,
  left to right, `1 kHz digit` `100 Hz digit` `100 kHz digit` `10 kHz digit`,
  while the label stack beside it reads, top to bottom, `10 kHz digit`
  `100 kHz digit` `100 Hz digit` `1 kHz digit`.
- **(d) Printed index differing from measured byte position where a block repeats
  another — CANNOT DETERMINE.** Blocks that repeat one another do occur: the six
  mode-tail figures all restart at ㊷ and draw closely similar layouts (recorded
  as STOP 2), and indices ①–㉕ are used by three different printed formats
  (recorded as STOP 3). The printed index is recorded as printed for every field
  in all of them. The other half of this hazard cannot be reported by this leg:
  measuring a byte position would breach the task statement's "no widths, no
  byte positions", so no measured position exists to compare against. Nothing was
  reconciled and nothing was reinterpreted in the light of anything else. See
  `## Method`, "Conflicts inside the brief".

## STOP findings

1. **PDF page 15, `Programmable scan start (remote) data` figure (D35) — the same
   index token is printed twice in two different forms.** On the numbered band
   over the three-row byte block, ranges and pairs are set with a wide wave dash
   and an ideographic comma and no surrounding spaces: `①〜⑤`, `⑥〜⑩`, `⑪、⑫`,
   `⑭〜⑰`, `⑱、⑲`, `⑳、㉑`. In the item list printed immediately below the same
   block, the same six fields are set with a narrow tilde and an ordinary comma
   and spaces: `① ~ ⑤`, `⑥ ~ ⑩`, `⑪, ⑫`, `⑭ ~ ⑰`, `⑱, ⑲`, `⑳, ㉑` — the form
   used by every other numbered band and item list on PDF pages 12–15, including
   the structurally identical `1A 00` block on page 12. This stops because the
   join key of this ledger is the index token as printed, and this field has two
   printed tokens. Transcribed into the CSV exactly as drawn on the band, with
   `STOP 1` in the six affected rows' notes. Confirmed at 400 and 600 dpi.
2. **PDF pages 13–15, all six mode-tail figures — the index sequence restarts.**
   ㊷ opens the FM figure (page 13), the D-STAR figure (page 13), the P25 figure
   (page 13), the dPMR figure (page 14), the NXDN figure (page 14) and the DCR
   figure (page 15), and again each of their ㊷ detail boxes. The same index
   denotes a different field in each: ㊷ is `Tone squelch type` in the FM figure
   and `Digital squelch (D.SQL) type` in the other five; ㊸ is `Tone squelch
   frequency`, `Digital code squelch (CSQL) code`, the first byte of `NAC`, the
   first byte of `COM ID`, `Radio Access Number (RAN) code` and the first byte of
   `UC code` respectively. This is a repeat of indices, so it stops. Each
   occurrence is transcribed as its own row against its own diagram, with
   `STOP 2` in the notes; nothing was merged and no duplicate was tidied away.
3. **PDF pages 12 and 15 — indices ①–㉕ carry three different meanings, two of
   them under the same command.** ①–㊶ number the `1A 00` memory-channel record
   (D1, page 12); ①–⑥ number the `1A 00` clear format (D34, page 15); ①–㉕
   number the `1A 0B 00` programmable-scan record (D35, page 15). Within command
   `1A 00`, ⑤ is `Skip/Select Memory scan setting` in D1 and `"FF"` in D34 — one
   printed statement contradicting another. Transcribed as printed in both, with
   `STOP 3` in the notes of every D34 and D35 row.
4. **PDF page 13, `㊸ ~ ㊺ NAC` detail box (D17), right column foot — printed
   spelling contradicts the same box's other labels.** The leader landing on the
   second cell reads `100th posiiton: 0 ~ F`, where the same box prints `10th
   position:   0 ~ F` and `Once position:  0 ~ F`. Read as `posiiton` at 400 dpi
   enlarged 250 per cent and again at 600 dpi enlarged 180 per cent. Transcribed
   verbatim as printed, in the notes of the D17 `㊸` row, with `STOP 4`.
5. **PDF page 15, clear-format list (D34), foot of the left column — a range with
   no closing index.** The last entry is printed `⑥ ~ :   None`: a range opened
   with a circled ⑥ and a tilde, with no upper index. Transcribed exactly as
   printed, as `⑥ ~`, with `STOP 5` in the notes. (The aligned colon that follows
   each index in this list is the list's separator and is not treated as part of
   the index; that convention is stated in the same row's notes.)

Nothing on these pages was unreadable: every glyph resolved by eye at 400 dpi
enlarged, and no `UNREADABLE` cell appears in the CSV. No arithmetic STOP arose:
in each of the six mode-tail figures the byte cells drawn and the byte cells
claimed by the index spans agree exactly (FM 1+3+3 = 7 cells; P25 1+3 = 4;
D-STAR 1+1 = 2; dPMR 1+2+1+1+3 = 8; NXDN 1+1+1+3 = 6; DCR 1+2+1+3 = 7), each
count taken twice, at 400 and at 600 dpi. The two multi-row blocks (D1 and D35)
abbreviate long runs with dotted ellipsis cells, so their drawn extent cannot be
counted against their spans at all; nothing is asserted about them either way.

## Observed disagreements

Recorded as printed, not resolved, and not treated as STOPs.

- **PDF page 12, D5 (`⑳, ㉑ Programmable tuning step`).** The lower-order byte
  is drawn first: ⑳ covers `1 kHz digit` and `100 Hz digit`, ㉑ covers
  `100 kHz digit` and `10 kHz digit`. Within each byte the digits descend, but
  between the bytes they do not.
- **PDF page 13, D13 (D-STAR `㊷ Digital squelch (D.SQL) type`).** The value list
  is printed `0=OFF` then `2=CSQL`. There is no `1`.
- **PDF page 12, D2 (`⑤ Skip/Select Memory scan setting`).** The right cell's
  first value line is printed `0 =OFF`, with a space before the equals sign,
  where the sibling boxes on the same page print `0=OFF` closed up.
- **PDF page 12, D6 (`㉒ Attenuator setting`).** `00=OFF` appears to be set with a
  narrower equals glyph than the `10＝10dB`, `20＝20dB`, `30＝30dB` lines beneath
  it. All four are transcribed with `=` in the notes; the apparent glyph
  difference is reported here rather than encoded.
- **Item-list spacing.** `⑭  ~ ⑰ Offset frequency` is printed with a visibly
  wider gap after the circled ⑭ than the other range headings carry, on both
  page 12 and page 15; the corresponding band tokens are set `⑭ ~ ⑰` (page 12)
  and `⑭〜⑰` (page 15).
- **PDF page 12, the ⓘ note above the item list.** It reads "In the modes other
  than FM and Digital, ㊷ and or later is not used. In the FM and Digital modes,
  entering ㊷ and or later can be omitted." — "and or later" appears twice, and
  reads as a slip for "and/or later" or "㊷ or later". Recorded as printed; no
  row is made from it, since the ㊷ in it is a reference in running prose, not a
  numbered field of a diagram.
- **Leader stacks printed bottom-up.** See hazard (c): ten detail boxes list
  their cell labels in the reverse of the drawn cell order. Not a STOP, but it is
  the single largest source of transcription risk on these pages and is the
  reason the leader-to-cell mapping was re-followed by eye in the second pass.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.
