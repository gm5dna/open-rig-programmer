# IC-7300MK2 CI-V — memory-record field ledger (leg L)

## Source

- Document title, as printed on the cover (PDF page 1, rendered): `CI-V REFERENCE GUIDE`, in the grey band beneath the Icom logo; below it, `HF/50 MHz TRANSCEIVER` over the model name `IC-7300MK2`; `Icom Inc.` at the foot.
- Revision code, as printed: `A7841-8EX`, at the foot of the back cover (PDF page 27), right-hand side, immediately above `© 2025  Icom Inc.` and `Oct. 2025`. No revision code is printed on the front cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300mk2_civ_ref_0.pdf`
- Page count: 27 PDF pages. Established from this same PDF by rendering: PDF page 27 renders as the back cover, and a render request for PDF page 28 is refused with `Wrong page range given: the first page (28) can not be after the last page (27)`. (The same figure appears in this PDF's `pdfinfo` metadata — see `## Method`.)

## Extent

Rendered: PDF pages 16, 17, 18 at 300 dpi; PDF page 17 again at 400 dpi and at 600 dpi; PDF pages 1 and 27 at 150 dpi and PDF page 27 again at 400 dpi (source/revision identification only).

Read and transcribed: **PDF page 17 only**. Printed folio at the foot of each page read: PDF page 16 → folio `16`; PDF page 17 → folio `17`; PDF page 18 → folio `18`. PDF page 1 (cover) and PDF page 27 (back cover) carry no folio.

Contribution of each page:

- **PDF page 17 (folio 17)** — the only page transcribed. Running head `REMOTE CONTROL`; section band `Remote control (CI-V) information`; sub-heading `◇ Command formats`; the bullet heading `• Memory channel content` and, under it, `Command: 1A 00`. Everything in the CSV comes from this page.
- **PDF page 16 (folio 16)** — read only to fix the section boundary. Same running head and same `◇ Command formats` sub-heading, but its bullet headings are `• Operating frequency`, `• Operating mode`, `• Band edge frequency settings`, `• Frequency span for ⊿F scanning`, `• Codes for CW message contents`, `• Turning the transceiver ON`. None of these is a memory-record data block, so nothing was transcribed from it.
- **PDF page 18 (folio 18)** — read only to fix the far boundary. Same running head and sub-heading; its bullet heading is `• Codes for character entries`, `Command: 1A 00, 1A 05 …`, followed by ASCII code tables. Nothing transcribed.

Where the transcribed material begins and ends on PDF page 17:

- Immediately **before** it: the bold bullet heading `• Memory channel content` and the line `Command: 1A 00`.
- The material itself: the numbered band and byte-cell row (diagram D1), then the two-column keyed explanations from `①, ② Memory channel number` down to `⑱ ~ ㉝ Memory name settings`, which supply the labels, and the two nibble-detail insets (D2 under `③ Split and Select memory setting`, D3 under `⑪ Data mode and tone type settings`).
- Immediately **after** it: the paragraph `To clear a memory channel content, send the command “1A 00 XX XX FF.”` with its two ⓘ notes, then the shaded `NOTE:` box whose three bullets end `to match your transceiver.` Below the NOTE box the lower ~40 per cent of the page is blank, down to the folio `17`.

Diagram ids assigned (page order; two-column body read left column then right column):

- **D1** — printed caption verbatim: `• Memory channel content` / `Command: 1A 00`. Position: full width of the page, directly under that caption, upper third of the page — a single row of `X⋮X` cells with a band of circled/filled indices and braces above it. 9 numbered groups.
- **D2** — printed caption verbatim: `③ Split and Select memory setting`. Position: left column, upper middle of the page; a two-nibble `X ⋮ X` box with a circled index above it and SPLIT/SELECT arrow labels below, to the left of the `SPLIT | SELECT` value table. 1 numbered index.
- **D3** — printed caption verbatim: `⑪ Data mode and tone type settings`. Position: right column, top of the page (level with the `①, ②` heading in the left column); a two-nibble `X ⋮ X` box with a circled index above it and DATA/TONE arrow labels below, to the left of the `DATA | TONE` value table. 1 numbered index.

D2 and D3 are included because each prints a numbered index of its own over its own data-block box. Reading order within the two-column body was taken as left column then right column, so D2 (left) precedes D3 (right) even though D3 sits higher on the page; this is a layout convention, not a reading of the page, and is recorded here so a second reader can re-order if their convention differs.

## Method

1. **Locate, 300 dpi.** `pdftoppm -png -r 300 -f 16 -l 18 <pdf> r300/p` into the fresh directory `.../evidence/ic7300mk2-L/r300/` (created empty for this task). Whole-page renders of PDF pages 16, 17 and 18 were read as images to confirm which page carries the `• Memory channel content` block and that the neighbouring pages carry different bullet headings under the same `◇ Command formats` sub-heading.
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f 17 -l 17 <pdf> r400/p` → `r400/p-17.png`, 3308 × 4678. All first-pass values were read from crops of this raster.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`) and was used throughout. First-pass commands (all on `r400/p-17.png`, output under `crops/`):
   - `magick r400/p-17.png -crop 2600x330+400+1000 +repage -resize 200% crops/band_full.png` — whole numbered band.
   - `magick r400/p-17.png -crop 900x160+400+1030 +repage -resize 400% crops/band_A.png`, `-crop 900x160+1250+1030 … band_B.png`, `-crop 900x160+2100+1030 … band_C.png` — the band in three overlapping segments at 400 per cent, enough separation to read each numeral, each circle outline and each disc fill clear of its neighbours.
   - `magick r400/p-17.png -crop 1250x600+220+1590 +repage -resize 250% crops/inset3.png` and `-crop 1250x600+1700+1250 … crops/inset11.png` — the two nibble-detail insets with their headings and tables.
   - `magick r400/p-17.png -crop 1500x120+220+1270 … crops/h_12.png`, `+220+2250 … h_48.png`, `+220+2490 … h_910.png`, `-crop 1600x120+1700+1800 … h_1214.png`, `+1700+1930 … h_1517.png`, `+1700+2190 … h_1833.png` — each keyed heading line at 250 per cent.
   - `magick r400/back-27.png -crop 900x180+2300+4180 +repage -resize 300% crops/revcode.png` — revision code.
4. **`pdftotext -layout` was NOT run.** It was not used for navigation or for anything else; navigation was done by reading the 300 dpi whole-page renders. `pdfinfo` **was** run once on this same PDF for its metadata (title, page count, page size, encryption flags); no field index, label, style or position in the CSV came from it. Only my own output directory under `.../evidence/ic7300mk2-L/` was listed (`ls`), to confirm my renders had been written.
5. **`tesseract` was available but was NOT used.** Every numeral and every label was read by eye from the enlarged crops; nothing needed an OCR aid, so no OCR value had to be confirmed or rejected.
6. **Second independent pass — done.** After the first pass was complete, PDF page 17 was re-rendered at a **different dpi (600 dpi**, `r600/p-17.png`, 4961 × 7016 — a different raster, not a resize of the 400 dpi one) and re-read through **different crop windows at different enlargements**: the band was cut into **four** overlapping segments with boundaries deliberately placed where the first pass's **three** segments had their middles (`-crop 1020x280+580+1520 … -resize 300%`, `1000x280+1500+1520`, `1000x280+2400+1520`, `1150x280+3300+1520`), and the keyed headings were re-read from two tall single-column crops at 150 per cent (`-crop 1800x1900+300+1900` and `-crop 2000x1600+2500+1850`) rather than from one narrow strip per heading; the `⑨, ⑩` heading, the diagram caption and the NOTE box were re-cut separately (`-crop 1900x260+300+3700 -resize 200%`, `-crop 1800x220+300+1330 -resize 200%`, `-crop 2100x800+2700+4300 -resize 180%`).
   **Disagreements between the two passes: none.** Every field index, every index style (circled vs filled), every label string and every position agreed cell for cell across the two rasters. No third render was needed.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** D1's band draws eight of its nine groups as black numerals inside thin outline circles (`circled`) and one group — the eighth, spanning the dotted-outlined stretch of the byte row — as white numerals reversed out of solid black discs (`filled`). Both styles are recorded exactly as drawn; neither has been normalised to the other and no meaning has been inferred from either. The same two styles recur in the NOTE box at the foot of the right column, where circled `④ ~ ⑰` and filled `❹ ~ ⓱` appear in the same sentence.
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED.** Nothing on this page's diagrams is set at an angle: every index, every label and every table entry on PDF page 17 is horizontal. (Rotated, vertically-set leader labels do occur on PDF page 16's frequency diagrams, but nothing was transcribed from that page.) Position was in any case read from the renders, not from any text layer.
- **(c) Leader-line label order reversed — NOT ENCOUNTERED.** D1's indices sit above the byte row with square braces dropping onto the cells they cover; each brace was followed by eye from its index down to its cells, and the braces run strictly left to right in the same order as the indices printed above them, with no crossing. In D2 and D3 the two up-arrows below each box point straight up to the nibble immediately above them (SPLIT→left, SELECT→right; DATA→left, TONE→right) with no crossing.
- **(d) Printed index differs from measured position where a block repeats — ENCOUNTERED (printed index recorded; byte positions deliberately not measured).** D1's eighth group prints `❹ ~ ⓱`, repeating indices already printed earlier in the same band as `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭` and `⑮ ~ ⑰`, while sitting further to the right along the byte row than any of them. The printed index is recorded exactly as printed and is not reconciled with, or reinterpreted in the light of, its position. This task's scope explicitly excludes widths and byte positions and the ledger schema has no column for one, so no byte position was measured; the repeat itself is recorded as STOP 1.

## STOP findings

1. **PDF page 17 — D1 numbered band, eighth group (the black-disc pair over the dotted-outlined stretch of the byte row, between the `⑮ ~ ⑰` brace and the `⑱ ~ ㉝` brace).** What is printed: `❹ ~ ⓱` — a `4` and a `17` in white on solid black discs, joined by a tilde, over a brace. Why it stops: it is a discontinuity in the index sequence on three counts at once. (i) It repeats indices already printed in this same band — 4 through 17 have all appeared to its left. (ii) It is out of numeric order: the band reads `1, 2 · 3 · 4~8 · 9, 10 · 11 · 12~14 · 15~17` and then drops back to `4` before continuing to `18~33`. (iii) The indices `4` and `17` are each printed twice in this band **with different styling** — outline-circled at their first appearance (`④ ~ ⑧`, `⑮ ~ ⑰`) and filled-disc here. Transcribed into the CSV exactly as seen, as `4~17` with `index_style` `filled`, `notes` carrying `STOP 1`; the two earlier rows whose indices it repeats (`4~8`, `15~17`) carry a cross-reference to STOP 1 in their notes. Nothing has been renumbered, merged or reconciled. Corroborating text printed elsewhere on the same page, in the NOTE box, and not treated as a resolution: `The same data as ④ ~ ⑰ are stored in ❹ ~ ⓱.`

No other STOP applies: every numeral, circle, disc, brace and label on this page was read cleanly at 400 dpi enlarged and again at 600 dpi through different windows, with no disagreement between passes, nothing illegible, no `UNREADABLE` cell, and no arithmetic in scope (no widths, positions or totals were transcribed).

## Observed disagreements

Recorded as printed, not resolved:

1. The eighth group of D1, `❹ ~ ⓱`, is the only group in the band with **no keyed heading anywhere on the page**. Every other group has a bold heading in the two-column body (`①, ② Memory channel number` … `⑱ ~ ㉝ Memory name settings`); the filled group is named only inside the shaded `NOTE:` box. Its `label_verbatim` is therefore empty.
2. The keyed headings are inconsistent between singular and plural for parallel items: `⑪ Data mode and tone type settings` and `⑱ ~ ㉝ Memory name settings` print **settings**, whilst `③ Split and Select memory setting`, `④ ~ ⑧ Operating frequency setting`, `⑨, ⑩ Operating mode setting`, `⑫ ~ ⑭ Repeater tone frequency setting` and `⑮ ~ ⑰ Tone squelch frequency setting` print **setting**.
3. Two different join conventions are printed within the one band: comma pairs (`①, ②` and `⑨, ⑩`) and tilde ranges (`④ ~ ⑧`, `⑫ ~ ⑭`, `⑮ ~ ⑰`, `❹ ~ ⓱`, `⑱ ~ ㉝`), with two groups printed as a bare single index and no join at all (`③`, `⑪`).
4. On the page the tilde and comma forms are set with spaces around them (`④ ~ ⑧`, `①, ②`). The `field_index` cells follow the exact forms the task specifies as the shared L/W/B convention (`4~8`, `1, 2`), i.e. no spaces around the tilde, one space after the comma. Only that inter-character spacing differs from the page; no digit, tilde, comma or ordering has been altered, and nothing has been collapsed to a hyphen.
5. Indices `③` and `⑪` are each printed a further two times on the page beyond their appearance in D1's band — once in their bold keyed heading and once above their own nibble-detail box (rows D2 and D3). All three occurrences are drawn in the same outline-circled style, so this is a repetition, not a styling conflict.
6. Two groups in D1's band (`③` and `⑪`) are drawn with no brace or leader at all — the circled index simply sits above a single cell — whereas the other seven groups all carry a square brace onto the cells they cover.
7. D1's byte row itself mixes three cell treatments — shaded `X⋮X` cells, unshaded `X⋮X` cells, shaded ellipsis (`…`) cells and a dotted-outlined stretch of heavy dots under the filled group. This is noted as a fact of the drawing only; no meaning is attributed to the shading, and no widths or positions were transcribed.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.
