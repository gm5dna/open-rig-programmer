# IC-7300MK2 CI-V reference — memory-record transcription (leg B)

## Source

- Title as printed on the cover (PDF page 1): the grey band reads `CI-V REFERENCE GUIDE`; below it, in the white field, `HF/50 MHz TRANSCEIVER` above `IC-7300MK2`, with `Icom Inc.` at the foot.
- Revision code as printed: `A7841-8EX`, printed at the foot of the back cover (PDF page 27), in the right-hand block above `© 2025  Icom Inc.    Oct. 2025`. No revision code is printed on the front cover, and none is printed in the running head or footer of the transcribed pages.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300mk2_civ_ref_0.pdf`
- Page count: 27 PDF pages (read from `pdfinfo` on this same PDF; PDF page 27 renders as the back cover, consistent with that count).

## Extent

Rendered and read:

| PDF page | printed folio | rendered at | what it contributed |
|---|---|---|---|
| 1 | none (front cover) | 200 dpi | cover title block for `## Source` |
| 8 | 8 | 300 dpi | navigation only; not transcribed |
| 9 | 9 | 300 dpi | set-mode/menu material — see below |
| 10–15 | 10–15 | 300 dpi | navigation only; not transcribed |
| 16 | 16 | 300, 400, 600 dpi | semantics for `④ ~ ⑧` (• Operating frequency) and `⑨, ⑩` (• Operating mode) |
| 17 | 17 | 300, 400, 600 dpi | the memory-record data-block diagram D1 and its field headings, tables and NOTE box |
| 18 | 18 | 300, 400, 600 dpi | the character tables — see below |
| 19 | 19 | 300 dpi | navigation only; not transcribed |
| 23 | 23 | 400, 600 dpi | semantics for `⑮ ~ ⑰` (• Repeater tone/tone squelch frequency settings) |
| 27 | none (back cover) | 200 dpi | revision code for `## Source` |

**Diagram D1** — printed caption verbatim: the bold bullet heading `• Memory channel content`, with `Command: 1A 00` set on the line beneath it. Position: PDF page 17, spanning the full width of the type area, immediately below that caption and above the two-column block of field headings; it is the only data-block diagram on the page.

Two further small diagrams appear on page 17 — a single byte box with `③` above it and SPLIT/SELECT arrows beneath, and a single byte box with `⑪` above it and DATA/TONE arrows beneath. They carry no caption of their own and sit inside the field explanations for `③` and `⑪`; they are treated as parts of those field entries rather than as separate diagrams, and no `D2` was assigned.

**Where the transcribed material begins and ends.** On page 17 it begins with the bold bullet heading `• Memory channel content`, which is preceded on the page by the running head `REMOTE CONTROL`, the grey section bar `Remote control (CI-V) information` and the line `◇ Command formats`. It ends with the grey NOTE box in the right column whose last line reads `to match your transceiver.`; below that the page is blank down to the folio `17`. Immediately before the material, PDF page 16 ends with `* When obtaining the edge number (by the command “02”), the edge number ① is not returned.` in the left column and the `13 “FE”s` example in the right column. Immediately after it, PDF page 18 opens with the bold bullet heading `• Codes for character entries`.

**The character table — was it printed at all, and what did it contribute.** It was printed. PDF page 18 (folio 18) prints, under `• Codes for character entries` and `Command: 1A 00, / 1A 05  01 07, 01 14, 01 28, 01 35`, two tables in the left column headed `- Character codes— Letters and Numbers` and `- Character codes— Symbols`, each with paired `Character` / `ASCII code` columns; and in the right column a `Cmd. / Sub cmd. / Setting item` table whose first row is `1A 00 … Memory name (⑱ ~ ㉝) (up to 16 characters)` with a `ⓘUsable characters:` list. It contributed the whole of `values_verbatim` for `⑱ ~ ㉝`, corroborated the printed width of 16, and produced STOP 2.

**The set-mode/menu pages — were they printed at all, and what did they contribute.** PDF page 9 (folio 9) was printed and read. It is headed `◇ Command table` and tabulates `1A* 05` SET-mode items under the sub-headings `SET > Connectors > ACC AF/IF Output`, `SET > Connectors > LAN AF/IF Output`, `SET > Connectors > MOD Input`, `SET > Connectors > External Keypad`, `SET > Connectors > CI-V`, `SET > Connectors`, and `SET > Connectors > USB SEND/Keying`. **It contributed nothing to this transcription.** No field of the memory-record diagram on page 17 refers to a set-mode or menu item: the only cross-references printed against the fields are to `“Operating frequency.” (p. 16)`, `“Operating mode.” (p. 16)`, `“Repeater tone/tone squelch frequency settings.” (p. 23)` and `“Codes for character entries.” (p. 18)`. No value in the CSV came from page 9.

## Method

- **Locate.** `pdftoppm -png -r 300 -f 8 -l 19 <pdf> r300/p` into a fresh directory `…/evidence/ic7300mk2-B/r300/`, which was created empty for this task. Page 9 was rendered separately as `r300/p9-09.png`; cover and back cover at 200 dpi as `r300/cover-01.png` and `r300/last-27.png`. The 300 dpi whole-page renders of pages 17 and 18 were read as images to find the diagram and the character tables and to confirm the running head and section bar matched the section named in the task.
- **Read.** Pages 16, 17, 18 and 23 were re-rendered at 400 dpi (`pdftoppm -png -r 400 …` into `r400/`). Every first-pass value was read from those 400 dpi rasters.
- **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`) and was used throughout. First-pass crops (into `crops/`), for example:
  - `magick r400/p-17.png -crop 2500x330+400+980 +repage -resize 200% crops/d1_full.png`
  - `magick r400/p-17.png -crop 900x330+400+980 +repage -resize 300% crops/d1_a.png` (and `+1250`, `+2050` for the other thirds)
  - `magick r400/p-17.png -crop 1500x520+230+1250 +repage -resize 200% crops/p17_L1.png` (and the other left/right column bands)
- **Measuring the byte cells.** The byte row's horizontal rules and vertical cell borders were located numerically rather than by eye, by dumping the raster with `magick … -colorspace Gray -depth 8 -negate txt:-` and counting dark pixels per column within the row's vertical extent. First pass, 400 dpi: top rule y=1132–1134, bottom rule y=1211–1213; 20 solid vertical borders at x = 481.5, 592, 702, 812, 922.5, 1033, 1143, 1253, 1363.5, 1474, 1584, 1694, 1804.5, 1915, 2025, 2130, 2461, 2571, 2681.5, 2791 — i.e. 19 drawn cells at a pitch of 110.4 px.
- **tesseract** was available (`/opt/homebrew/bin/tesseract`) but **was not used**. Every value was read by eye from the rasters; no OCR value was recorded.
- **`pdftotext` was never run**, in any form, on this or any other file. Navigation was done entirely by reading the 300 dpi whole-page renders.
- Other commands run against the PDF: `pdfinfo` (page count and title metadata only, quoted in `## Source`) and `pdftoppm`. No source directory was listed or searched: the only shell listings were of the render and crop directories this task itself created under `…/evidence/ic7300mk2-B/`, to confirm my own output files existed. Nothing outside that directory and the one PDF was opened, listed or searched.

**Second independent pass.** A full second pass was made after the first was complete. The second raster differed in three ways: a different dpi (600 dpi, `r600/`, versus 400 dpi for the first pass), different crop windows (offsets `+700`, `+1700`, `+2700`, `+3650` across the byte row instead of `+400`, `+1250`, `+2050`; separate per-byte crops of the page 16 and page 23 diagrams instead of half-page crops), and different enlargements (250 % and 200 %–900 % versus 200 %–300 %). The byte-cell measurement was repeated independently at 600 dpi over a different vertical band, and produced 20 borders at 600 dpi x = 722.5, 888, 1053.5, 1218.5, 1384.5, 1549.5, 1714.5, 1880.5, 2045.5, 2210.5, 2376.5, 2541.5, 2706.5, 2872.5, 3037.5, 3195.5, 3691.5, 3857, 4022.5, 4186.5 — which, divided by 1.5, give 481.7, 592.0, 702.3, 812.3, 923.0, 1033.0, 1143.0, 1253.7, 1363.7, 1473.7, 1584.3, 1694.3, 1804.3, 1915.0, 2025.0, 2130.3, 2461.0, 2571.3, 2681.7, 2791.0.

**Cells where the two passes disagreed: none.** Every border agreed to within 0.7 px at 400 dpi scale (well under one border stroke width); every cell-to-bracket assignment agreed; every label, index, width, table cell and enum value agreed. No third render was needed to settle anything.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** Diagram D1's numbered band draws two styles: outlined circled numerals for `①`, `②`, `③`, `④`, `⑧`, `⑨`, `⑩`, `⑪`, `⑫`, `⑭`, `⑮`, `⑰`, `⑱`, `㉝`, and filled/reversed (white-on-black) circled numerals for `❹` and `⓱`. Both styles also occur in the grey NOTE box lower on the page, where `④ ~ ⑰` is outlined and `❹ ~ ⓱` is filled. Each index is recorded in the style it is drawn in; the two styles have not been normalised and no meaning has been inferred from the styling beyond what the NOTE box prints.
- **(b) Vector groups with rotated labels — ENCOUNTERED.** Not on D1 itself, whose bracket labels are all set horizontally, but on both pages this transcription's cross-references lead to: on page 16 (`• Operating frequency`) all ten nibble labels and all ten ranges are set rotated 90° anticlockwise beneath the byte row, and on page 23 (`• Repeater tone/tone squelch frequency settings`) all six are. Every one of those labels was read from the render by following its arrow up to the nibble it points at, not from any text order.
- **(c) Leader-line label order reversed — NOT ENCOUNTERED.** On page 17 the `③` sub-diagram's two arrows run SPLIT → left nibble and SELECT → right nibble, and the `⑪` sub-diagram's run DATA → left nibble and TONE → right nibble; in both, label order on the page matches the nibble order the arrows land on. On pages 16 and 23 each arrow rises vertically from its own rotated label to the nibble immediately above it, with no crossing. Every leader was followed by eye from label to cell at 600 dpi.
- **(d) Printed index differs from measured position — ENCOUNTERED.** The block bracketed `❹ ~ ⓱` repeats printed indices 4 to 17, which have already been used earlier in the same row by `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭` and `⑮ ~ ⑰`. Its measured position is the abbreviated region at x = 2130–2461 (400 dpi), lying after drawn cell 15 (the cell under `⑰`) and before drawn cell 16 (the first cell under `⑱`). Both the printed index and the measured position are recorded in that row's `field_index` and `notes`; they have not been reconciled, and neither has been reinterpreted in the light of the other. Every other field's row likewise carries its measured drawn-cell span in `notes`.

## STOP findings

1. **Index sequence repeats, in a different numeral style.** PDF page 17, folio 17. Visual anchor: the numbered band immediately above the byte row of the `• Memory channel content` data block, between the bracket labelled `⑮ ~ ⑰` and the bracket labelled `⑱ ~ ㉝`. What is printed: the brackets run, left to right, `①, ②` — `③` — `④ ~ ⑧` — `⑨, ⑩` — `⑪` — `⑫ ~ ⑭` — `⑮ ~ ⑰` — `❹ ~ ⓱` — `⑱ ~ ㉝`. Indices 4 to 17 are therefore printed twice in one diagram: once as outlined circled numerals and once as filled white-on-black circled numerals. Why it stops: the rules require a STOP for any repeat in the index sequence and for any index printed twice with different styling, and both occur here. Transcribed as seen: `❹ ~ ⓱` has its own row in the CSV, carrying `STOP 1` in `notes`, with its printed index, its measured position and the NOTE-box text recorded and not reconciled against the outlined `④ ~ ⑰` rows.

2. **Two different ASCII codes are printed for what is drawn as the same character.** PDF page 18, folio 18. Visual anchor: the table headed `- Character codes— Symbols`, fifth data row (the row immediately below the row containing `?` / `3F` / `”` / `22`). What is printed: the left `Character` cell and the right `Character` cell of that row are drawn with an identical glyph — a right single quotation mark — while their `ASCII code` cells read `27` and `60` respectively. Enlarged to 900 % at 600 dpi, the two glyphs are pixel-for-pixel the same shape; neither is drawn as a grave/left single quotation mark. Why it stops: this is one thing printed contradicting another thing printed on the same page — a single glyph assigned two different codes in one table. Transcribed as seen: the `⑱ ~ ㉝` row's `values_verbatim` contains both `’: 27` and `’: 60`, in printed order, un-merged and un-corrected, with `STOP 2` in that row's `notes`.

## Observed disagreements

Recorded as printed; not resolved.

- `⑫ ~ ⑭ Repeater tone frequency setting` is printed on page 17 as a bare heading with nothing under it — no value, no code, no encoding statement, no cross-reference. The `ⓘ` cross-reference in that area, `ⓘ See “Repeater tone/tone squelch frequency settings.” (p. 23)`, is set under the *next* heading, `⑮ ~ ⑰ Tone squelch frequency setting`, although the title it names covers both. Nothing has been carried across from `⑮ ~ ⑰` or from page 23; `values_verbatim` for `⑫ ~ ⑭` is empty and its `encoding` is `unstated`.
- The drawn cell under `⑰` (drawn cell 15) measures 105 px at 400 dpi / 158 px at 600 dpi, against a pitch of 110.4 px / 165.5 px for the other eighteen cells — about 4 % narrower. Its right-hand border is also the left edge of the dashed `❹ ~ ⓱` region. The cells remain contiguous (no gap, no overlap) and every field's drawn-cell count still matches its printed index count, so this is recorded here rather than as a STOP.
- Three of the nine spans are drawn abbreviated: `④ ~ ⑧` as `XX` `...` `XX`, `⑱ ~ ㉝` as `XX` `...` `XX`, and `❹ ~ ⓱` as a single dashed region containing a dotted line and no byte cells at all. The drawn cell counts (3, 3 and 0) are therefore smaller than the printed index counts (5, 16 and 14). The `...` cells and the dotted region are explicit abbreviation marks, drawn with dashed borders unlike every other cell, so this is not treated as a contradiction; both the index count and the measured drawn span are recorded in each row's `width_bytes` and `notes`.
- The two page 16 sections cross-referenced from page 17 are headed for different commands: `• Operating frequency / Command: 00, 03, 05, 1C 03` and `• Operating mode / Command: 01, 04, 06`. Neither prints `1A 00`. Page 23's `• Repeater tone/tone squelch frequency settings` is headed `Command: 1B 00, 1B 01`, again not `1A 00`. Only page 18's `• Codes for character entries` lists `1A 00` among its commands. The page 16 filter note is worded for commands 01 and 06 specifically.
- On page 16 the `②Filter` column of the operating-mode table is headed with its own `②`, and on page 23 the first byte is indexed `①*`; these local index numbers are unrelated to the page 17 indices `④ ~ ⑧`, `⑨, ⑩` and `⑮ ~ ⑰` that point at them. Both numbering schemes are recorded in the affected rows' `notes` and have not been merged.
- In the page 18 symbols table the glyph coded `2D` is drawn as a mid-height horizontal bar noticeably longer than a hyphen, while the `ⓘUsable characters:` list on the same page uses a short hyphen in `+, - .`. Both are transcribed as drawn.
- Page 17's `③` entry prints `ⓘ Set 00 for P1 and P2.`, a two-digit value, whereas the table beside it prints only single-nibble codes under the two column headings `SPLIT` and `SELECT`. The two are recorded separately in that row — the nibble codes in `values_verbatim`, the `ⓘ` line in `notes` — as printed, and no two-digit value has been synthesised from them.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.
