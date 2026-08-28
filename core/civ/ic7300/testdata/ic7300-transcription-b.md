# IC-7300 memory-record transcription — leg B

## Source

- Document title as printed on the cover (PDF page 1): **`IC-7300`**, above it `HF/50 MHz TRANSCEIVER`, and in the black band to the left of the chapter list `FULL MANUAL`. The publisher line at the foot of the cover reads `Icom Inc.`
- Revision code as printed: **`A7292-4EX-12b`**, printed at the foot of the left-hand column of the final page (PDF page 180), immediately above `© 2016–2024 Icom Inc.    Aug. 2024`.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300_fullmanual_ENG_12b.pdf`
- Page count: 180 PDF pages (A4, 595.276 × 841.89 pt).

## Extent

Pages rendered and read (PDF page number → printed folio):

| PDF page | Printed folio | Rendered at | Read? | Contribution |
|---|---|---|---|---|
| 1 | (none) | 150 dpi | yes | Cover title, for `## Source` only. |
| 126 | `12-10` | 300 dpi | yes | **Set-mode material: printed, but contributed nothing.** See below. |
| 160 | `19-2` | 300 dpi | yes | Set-mode-adjacent CI-V material: printed, contributed nothing to any recorded cell. See below. |
| 166 | `19-8` | 300 dpi | navigation only | Establishes that the section before the transcribed material is the end of `◇ Command table`; contains no memory-record diagram. |
| 167 | `19-9` | 300 dpi | navigation only | Locates the cross-referenced sections `• Operating frequency` and `• Operating mode`. No value was taken from it; the two fields that point at it are recorded with the cross-reference itself as their printed value. |
| 168 | `19-10` | 300 dpi + 400 dpi | yes | **Character table: printed.** See below. |
| **169** | **`19-11`** | **300 dpi + 400 dpi + 600 dpi** | **yes — the transcribed page** | The single memory-record data block (D1) and all of its printed semantics. |
| 170 | `19-12` | 300 dpi | navigation only | Establishes what follows the transcribed material. |
| 180 | (none) | 150 dpi | yes | Revision code, for `## Source` only. |

**Where the transcribed material begins and ends.** All of it is on PDF page 169 (folio `19-11`), the whole of the printed content of that page. Immediately before it, at the top of the same page, are the chapter head `19  CONTROL COMMAND` and the black section band `Remote control (CI-V) information`; the material itself opens with the bold caption `• Memory content` and the line `Command : 1A 00`. It ends with the third bullet of the `NOTE:` box in the right-hand column (`• Even if the Split function is OFF, …`); below that the page is blank down to the folio `19-11`. The preceding page (168, folio `19-10`) ends with `• Data mode with filter width settings`; the following page (170, folio `19-12`) opens with `• Memory keyer character entries`.

**Diagrams.** Exactly ONE memory-record data block is printed on page 169, and it is `D1`:

- **`D1`** — printed caption verbatim: `• Memory content` / `Command : 1A 00`. Position: upper third of PDF page 169, spanning the full width of both columns, directly beneath the black band `Remote control (CI-V) information`; a single horizontal band of byte cells with an index band of circled numerals and brackets above it.

Two further boxed figures are printed on page 169 — a one-cell box under `③ Split and Select memory setting` (left column) and a one-cell box under `⑪ Data mode and tone type settings` (right column). These are **legends belonging to D1's fields ③ and ⑪**, each redrawing that one cell of D1's band at a larger size with its leader lines; neither is a distinct memory record and neither carries an index of its own beyond the ③ / ⑪ it repeats. They are therefore not given separate `diagram_id`s; their content is transcribed into the ③ and ⑪ rows of D1 and their position is given in those rows' `visual_anchor`.

**Character table (PDF page 168, folio `19-10`) — PRINTED.** The section `• Codes for character entries` occupies the top of the right-hand column and is what D1's field `⑱–㉗` points at. It contributed: (a) the `encoding` value `ascii` for that field — the table's own column heading is `ASCII code`, and its rows read `A–Z 41–5A`, `a-z 61–7A`, `0–9 30–39` under `- Character codes— Letters and Numbers`, followed by `- Character codes— Symbols`, a 40-entry two-column table of symbol/ASCII-code pairs (`! 21`, `# 23`, `$ 24`, `% 25`, … `~ 7E`, `@ 40`); and (b) corroboration from the small `Command | Set item/selectable characters` table below it, whose first row reads `1A 00 | Memory name / All characters are usable.` (the second row, `1A 05 00 91 | Opening message / Uppercase letters, numbers, symbols (− / . @) and space are usable.`, is a different command and was not used). No numeral, width or index in the CSV came from page 168; only the `encoding` cell of the `⑱–㉗` row, and the corroborating quotations in that row's `notes`.

**Set-mode pages (PDF pages 126 and 160) — PRINTED, but they contributed nothing.**

- PDF page 126, folio `12-10`, is headed `12  SET MODE` with the black band `Connectors`. It prints `DATA OFF MOD`, `DATA MOD`, `External Keypad VOICE / KEYER / RTTY`, `CI-V Baud Rate`, `CI-V Address`, `CI-V Transceive`, `CI-V USB→REMOTE Transceive Address`, `CI-V Output (for ANT)` and `CI-V USB Port`. **No field of D1 refers to it**, and no cell of the CSV was taken from it.
- PDF page 160, folio `19-2`, is headed `19  CONTROL COMMAND` / `Remote control (CI-V) information` and prints `◇ CI-V connection`, `◇ Preparing` (which says the CI-V settings "are set in Set mode") and `◇ Data format`. Its `Controller to IC-7300` frame diagram labels field ⑥ `Data area` as `BCD code data for frequency or memory number entry`. That statement is about the CI-V frame's data area in general, is not printed against any field of D1, and was deliberately **not** used as the `encoding` for any row — the fields it might have been stretched to cover are recorded as `unstated`.

**Absent statements (findings, not gaps).**

1. D1's fields `④–⑧`, `⑨, ⑩`, `⑫–⑭`, `⑮–⑰` and `⑱–㉗` have **no encoding and no code list printed against them**. All that is printed is a cross-reference (`See "• Operating frequency."`, `See "• Operating mode."`, `See "• Repeater tone/tone squelch settings."`, `See "• Codes for character entries"`). The cross-reference is what the CSV records as their value; four of the five are `encoding = unstated` in consequence.
2. D1's group `❹–⓱` has **no label printed at all** (its `label_verbatim` cell is empty), and **no value, code or encoding printed at all** (its `values_verbatim` cell is empty). The only text printed about it anywhere on the page is the three-bullet `NOTE:` box, transcribed verbatim into that row's `notes`.
3. The `To clear the memory channel contents on 1A 00:` list prints entries for `①,②`, `③` and `④` and then **stops**; nothing is printed about `⑤` or anything after it under that condition.
4. `• Repeater tone/tone squelch settings`, the section that `⑫~⑭` and `⑮~⑰` point at, is **not printed on any page named in this task**, and none of pages 166–170 carries that heading. No value was invented for those fields; both rows carry the cross-reference as printed and `encoding = unstated`.

## Method

1. **Locate, 300 dpi.** `pdftoppm -png -r 300 -f 166 -l 172 <pdf> r300/p`, plus `-f 126 -l 126` and `-f 160 -l 160`, into the fresh directory `…/evidence/ic7300-B/r300/`. Every rendered page was read as an image to find the section whose printed heading matches (`• Memory content`, `• Codes for character entries`, `12 SET MODE / Connectors`).
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f 169 -l 169` and `-f 168 -l 168` into `r400/` (3308 × 4678 px). Every first-pass value was read from these.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`) and was used. First-pass crops, into `crops/`:
   - `magick r400/p-169.png -crop 2560x340+380+840 +repage -resize 200% crops/band_full.png`
   - `magick r400/p-169.png -crop 1300x340+380+840 +repage -resize 300% crops/band_left.png`
   - `magick r400/p-169.png -crop 1300x340+1640+840 +repage -resize 300% crops/band_right.png`
   - `magick r400/p-169.png -crop 1400x300+1150+880 +repage -resize 300% crops/band_mid.png`
   - `magick r400/p-169.png -crop 900x300+2050+880 +repage -resize 400% crops/band_tail.png`
   - `magick r400/p-169.png -crop 1200x620+250+1580 +repage -resize 250% crops/leg3.png`
   - `magick r400/p-169.png -crop 1100x430+1750+1200 +repage -resize 300% crops/leg11.png`
   - `magick r400/p-169.png -crop 1250x300+230+1090 +repage -resize 280% crops/body12.png`
   - `magick r400/p-169.png -crop 1250x420+230+2180 +repage -resize 250% crops/body48.png`
   - `magick r400/p-169.png -crop 1450x520+1700+1650 +repage -resize 250% crops/body1217.png`
   - `magick r400/p-169.png -crop 1500x470+1700+2140 +repage -resize 280% crops/clear2.png`
   - `magick r400/p-169.png -crop 1550x700+1700+2620 +repage -resize 230% crops/note2.png`
   - `magick r400/p-168.png -crop 1400x350+1700+840 +repage -resize 280% crops/p168_letters.png`
   - `magick r400/p-168.png -crop 1400x480+1700+2350 +repage -resize 280% crops/p168_cmdtable.png`
4. **`pdftotext -layout` was NEVER run**, on this or any other file. Navigation was done by reading the 300 dpi renders as images.
5. **tesseract** was available (`/opt/homebrew/bin/tesseract`) and was used as a reading aid on exactly two crops, `crops/leg3.png` and `crops/leg11.png` (`--psm 6`). Everything it returned was confirmed by eye on the 400 dpi and 600 dpi renders before being recorded; its habitual `0`/`O` and `★`/`*` confusions (it returned `O=OFF`, `1= *1`, `to. OFF`) were rejected in favour of what the render shows (`0=OFF`, `1= ★1`, `0: OFF`). No value rests on OCR.
6. **Second independent pass — done.** After the first pass was complete, page 169 was re-rendered at a different dpi and re-cropped with different windows and enlargements, and every value was re-read from those rasters:
   - `pdftoppm -png -r 600 -f 169 -l 169 <pdf> r600/p` (4961 × 7016 px, i.e. 1.5× the first-pass raster).
   - `magick r600/p-169.png -crop 1900x520+520+1280 +repage -resize 250% -sharpen 0x1 crops2/band_A.png` — a different split of the index band (the first pass split it at x = 1640 of the 400 dpi raster; this one splits it at a different point, so the two halves overlap differently and the ⑫–⑭ group falls in a different half).
   - `magick r600/p-169.png -crop 1900x520+2350+1280 +repage -resize 250% -sharpen 0x1 crops2/band_B.png`
   - `magick r600/p-169.png -crop 900x700+900+2500 +repage -resize 300% crops2/leg3_zoom.png` — the ③ legend at roughly 3× the first-pass magnification, cropped to the leaders and label stack alone rather than the whole figure.
   - `magick r600/p-169.png -crop 900x500+2900+1950 +repage -resize 300% crops2/leg11_zoom.png` — likewise for ⑪.
   - **Disagreements between the two passes: none.** Every index, bracket span, cell count, shading state, leader destination and label string read the same on both rasters, including the two leader assignments in ③ and ⑪ (the hazard most likely to flip), the filled-versus-outlined numeral styles, and the dash-versus-tilde difference between the index band and the body headings.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** D1's index band draws two distinct styles. Every index from ① to ㉗ inclusive is a black numeral inside an **outlined** circle. The eighth group, `❹–⓱`, is drawn as **white numerals reversed out of solid black filled circles**, and the same filled style is reused in the `NOTE:` box, which in one sentence sets both styles side by side: `• The same data as ④–⑰ are stored in ❹–⓱.` Each row's `notes` records the style as drawn for that row. No inference is drawn here about what the styling means.
- **(b) Rotated labels in a vector group — NOT ENCOUNTERED.** Every label on page 169 is set horizontally; nothing on this page is rotated. (The neighbouring pages 167 and 168 do carry vertically-set labels, but nothing was transcribed from them.) Position was read from the picture regardless.
- **(c) Leader-line label order reversed — ENCOUNTERED, twice.** In the ③ legend the label stack runs, top to bottom, `0=OFF / 1= ★1 / 2= ★2 / 3= ★3` then `0=Split OFF, 1=Split ON`; following the leaders by eye, the upper (bracketed) block belongs to the **right** half-byte and the lower single line to the **left** half-byte — the opposite of the label order. In the ⑪ legend the stack runs `0: OFF, 1: TONE, 2: TSQL` then `0=Data mode OFF / 1=Data mode ON`, and again the upper label belongs to the **right** half-byte, the lower to the **left**. Both assignments were traced leader by leader on the 400 dpi crops and re-traced independently on the 600 dpi crops, and agreed.
- **(d) Printed index differs from measured position — ENCOUNTERED.** D1 contains a repeated block: `❹–⓱` repeats the indices of `④`–`⑰`. Its printed index is 4–17; its measured position along the band, accumulating the printed index-range widths cell group by cell group from the left, is bytes 18–31. The group that follows it prints indices 18–27 but measures at bytes 32–41. Both numbers are recorded in the CSV `notes` for those two rows, side by side and unreconciled; neither has been reinterpreted in the light of the other. See STOP 2.

## STOP findings

1. **Discontinuity in the index sequence — a repeat, out of order, and printed twice in a different style.** PDF page 169, D1 index band, the eighth bracket, immediately to the right of the last shaded `⑮–⑰` cell and immediately to the left of the shaded `⑱` cell. What is printed there is `❹−⓱`: indices 4 to 17 again, in white numerals on solid black circles, after the sequence has already run ① ② ③ ④–⑧ ⑨ ⑩ ⑪ ⑫–⑭ ⑮–⑰ and before it resumes at ⑱. Every index from 4 to 17 is therefore printed twice on this one diagram, in two different numeral styles. This stops because the index sequence is not monotonic and cannot be read as one: the same fourteen index numbers denote two different regions of the same record. Transcribed as seen: the row with `field_index` `❹–⓱`, `notes` marked `STOP 1`. Nothing was renumbered and the second block's fields were not copied from the first — the first block's rows carry the labels and values printed against them, and the `❹–⓱` row carries the empty label and empty value cell that the page actually prints, plus the `NOTE:` text.
2. **Arithmetic: printed index does not match measured position for the last two groups.** PDF page 169, D1 index band, eighth and ninth brackets (the dotted elision region, and the final `X:X / … / X:X` trio at the far right of the band). Accumulating the printed index-range widths from the left of the band — ①=1, ②=2, ③=3, ④–⑧=4–8, ⑨=9, ⑩=10, ⑪=11, ⑫–⑭=12–14, ⑮–⑰=15–17 — the next region begins at byte 18. The region printed there is indexed 4–17, and the region after it, printed 18–27, begins at byte 32. This stops because index and position agree for every group up to ⑰ and then disagree by exactly 14 for the rest of the diagram. Transcribed as seen: both the printed index and the measured position appear in the `notes` of the `❹–⓱` and `⑱–㉗` rows, marked `STOP 2`; the printed index is what is in the `field_index` cells, unaltered.

No other STOP arose. In particular:

- Every numeral, index and label on page 169 was legible at 400 dpi enlarged, and again at 600 dpi; nothing is recorded as `UNREADABLE`.
- The elision cells do **not** create a measured-versus-printed width contradiction, so no STOP was raised for them. In this diagram a byte cell is drawn as a solid-ruled box containing `X:X`, whilst an elision is drawn differently and distinguishably: a dashed-outline box containing `...` (in the `④–⑧` and `⑱–㉗` groups) or a dotted-outline region containing a row of dots and no cell divisions at all (the whole `❹–⓱` group). An elision marker is not a byte cell and was not counted as one, so there is nothing to contradict the widths taken from the printed index ranges. Where a width could not be counted on the drawing at all — `❹–⓱`, which has no byte cells whatever — the row's `notes` say so explicitly.
- The two documented widths for the operating-frequency field (5 bytes in the band; `None` under the stated clear condition) are transcribed as two rows per the conditional rule, not as a contradiction: the condition is stated on the page, in terms that are not self-contradictory.

## Observed disagreements

Recorded exactly as printed; not resolved.

1. **Dash versus tilde for the same index ranges.** D1's index band prints its ranges with a dash — `④−⑧`, `⑫−⑭`, `⑮−⑰`, `❹−⓱`, `⑱−㉗` — whilst the body headings below print the same ranges with a tilde: `④~⑧ Operating frequency setting`, `⑫~⑭ Repeater tone frequency setting`, `⑮~⑰ Tone squelch frequency setting`, `⑱~㉗ Memory name settings`. Both forms were confirmed on the 400 dpi and 600 dpi rasters. The CSV's `field_index` cells use the index band's form, because the row is a field of that band; each affected row's `notes` gives the body form as well.
2. **Comma spacing in the same pair of indices.** The left-hand column heading prints `①, ②` with a space after the comma; the clear-command list in the right-hand column prints `①,②` with none. Both are transcribed as printed, in their own rows.
3. **Two renderings of the same channel range.** The `①, ②` section prints `00 01–00 99:` with a dash; the clear-command list prints the same range as `(00 01~00 99)` with a tilde.
4. **`④` treated as a whole field in one place and as the head of a range in another.** The index band and the body heading treat bytes 4 to 8 as one field (`④−⑧` / `④~⑧ Operating frequency setting`); the clear-command list addresses `④` alone and gives it the value `None`.
5. **The clear-command list is silent from ⑤ onwards.** It runs `①,②`, `③`, `④` and stops, printing nothing about the remaining thirty-odd bytes of the record under that condition.
6. **One `See` line serves two headings.** `⑫~⑭ Repeater tone frequency setting` and `⑮~⑰ Tone squelch frequency setting` are printed as two consecutive bold headings with a single indented `See "• Repeater tone/tone squelch settings."` beneath them. It is transcribed into both rows.
7. **Cross-reference target not present in the pages examined.** No page among those rendered carries the heading `• Repeater tone/tone squelch settings`, which `⑫~⑭` and `⑮~⑰` point at. Recorded, not chased.
8. **Hyphen versus en dash on the contributing page.** On PDF page 168 the character table prints `A–Z`, `41–5A`, `61–7A`, `0–9` and `30–39` with en dashes but `a-z` with a hyphen.
9. **`- Character codes— Letters and Numbers`** (PDF page 168) sets an em dash hard against the following space, with no space before it; the same styling recurs in `- Character codes— Symbols`.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

*(Clarification, offered so the sentence above is not read more broadly than it is true: the only directories whose contents were listed at any point were `…/evidence/ic7300-B/r300`, `…/r400`, `…/r600`, `…/crops` and `…/crops2` — the render and crop directories created by this task from this PDF, and the output directory this task writes to. No repository directory, and no directory outside `…/evidence/ic7300-B`, was listed, searched or browsed.)*
