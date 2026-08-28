# IC-7610 CI-V — memory-record data-block field ledger (L leg)

## Source

- Document title as printed on the cover (PDF page 1): the black cover panel prints the Icom logo, then `CI-V REFERENCE GUIDE`; below the rule, `HF/50MHz TRANSCEIVER` above `IC-7610`, and `Icom Inc.` at the foot.
- Revision code as printed: `A7380-7EX-4`. It is printed at the foot of the last page (PDF page 17), at the left-hand end of the bottom band, immediately above the line `© 2017–2025 Icom Inc.    Sep. 2025`. No revision code is printed on the cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7610_civ_ENG_4.pdf`
- Page count: 17 PDF pages.

## Extent

Rendered and read:

| PDF page | printed folio | contributed |
|---|---|---|
| 1 | (none printed) | cover — document title, model, publisher. Read for `## Source` only. |
| 11 | 10 | Read to establish where the material begins. Contains `◇ Command formats` with `• Operating frequency`, `• Operating mode`, `• Codes for CW message contents`, `• Band edge frequency settings`, `• Band stacking register`. **No memory-record data block.** |
| 12 | 11 | **The only page transcribed.** Contains `• Memory content` / `Command: 1A 00` — the byte band, its numbered legend, the two numbered sub-diagrams, and the clear-list. |
| 13 | 12 | Read to establish where the material ends. Contains `• Data mode with filter width settings`, `• IF filter width settings`, `• AGC time constant settings`, `• SSB transmission passband width settings`, `• SSB-DATA transmission passband width setting`, `• RX HPF/LPF setting for each operating mode`, `• Bandscope edge frequency settings`, `• Color settings`, `• Offset frequency settings`. **No memory-record data block.** |
| 17 | (none printed) | foot of page only, for the revision code. |

The transcribed material begins on PDF page 12 immediately after the section marker `◇ Command formats`, at the heading `• Memory content` / `Command: 1A 00`, and ends at the last line of the clear-list, `④:      None`, in the right-hand column.

Printed immediately before it: the running head `Remote control` (rule above it) and the line `◇ Command formats`.
Printed immediately after it: in the left-hand column, the heading `• Codes for character entries` / `Command: 1A 00,`; in the right-hand column, white space and then the table headed `Cmd. | Sub cmd. | Set item/selectable characters`. Neither carries a numbered field index, so neither is in scope.

### Diagrams indexed

| id | printed caption, verbatim | position on PDF page 12 |
|---|---|---|
| D1 | `• Memory content` / `Command: 1A 00` | Top of the page under `◇ Command formats`. The byte band of X-cells runs full width across the top; its numbered legend continues down the left column (four entries) and then down the right column (four entries). |
| D2 | `③ Select memory setting` | Left-hand column, middle third — a two-cell box printing `0` and `X`, with one circled index above it. |
| D3 | `⑪ Data mode and tone type settings` | Right-hand column, upper third — a two-cell box printing `X` and `X`, with one circled index above it. |
| D4 | `To clear the memory channel contents on 1A 00:` | Right-hand column, below the `⑱ ~ ㉗ Memory name settings` legend entry — a three-line numbered list re-stating indices ①, ②, ③ and ④ for the clear case. |

D4 is a numbered field list rather than a box-and-cells picture. It is included because it re-uses the same field indices for the same 1A 00 memory record and so is a block that duplicates part of D1; the task requires such a block to be recorded, not tidied away.

D1's indices carry no text inside the band itself — the band prints only `X`, `:` dividers and two `...` ellipsis cells. Each D1 index's label is the legend entry printed against the same circled index below the band; that legend is part of the same captioned block and is what `label_verbatim` records. Where a legend entry is followed by an indented `See "…"` cross-reference line or a value list, that following material is **not** treated as part of the label; it is recorded in `notes`.

## Method

- **Locate, 300 dpi.** `pdftoppm -png -r 300 -f 11 -l 13 <pdf> <out>/p` into a fresh directory. All three renders read as images. Confirmed the memory-record material sits on PDF page 12 alone.
- **Read, 400 dpi.** `pdftoppm -png -r 400 -f 12 -l 12 <pdf> <out>/p` (page raster 3308 × 4678). First pass read entirely from this raster.
- **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`, and `convert`). First-pass crops, e.g.:
  - `magick r400/p-12.png -crop 2450x340+560+780 +repage -resize 200% band_full.png`
  - `magick r400/p-12.png -crop 680x120+560+830 +repage -resize 400% idx_seg0.png` (and at x = 1200, 1840, 2450) — the numbered band in four overlapping segments
  - `magick r400/p-12.png -crop 1400x760+270+1080 +repage -resize 200% left_A.png`, `-crop 1400x700+270+1800` left_B, `-crop 1500x700+1650+1080` right_A, `-crop 1500x760+1650+1780` right_B
  Every numeral, circle outline, bracket and rule sat clear of its neighbours at these enlargements.
- **tesseract** was available but **was not used**. Every value was read by eye from the enlarged crops, so no OCR value needed confirming.
- **`pdftotext` was never run** — not with `-layout`, not at all. Navigation was done by reading the 300 dpi page images. `pdfinfo` was run once, for the page count and page size only; it produced no recorded field index, label or style.
- **Second independent pass.** After the first pass was complete, the page was re-rendered at **600 dpi** (`pdftoppm -png -r 600 -f 12 -l 12`, raster 4961 × 7016) and every value re-read from crops with **different windows and different enlargement** (250 %–300 % instead of 200 %–400 %), cut at different boundaries so that no crop reproduced a first-pass crop: the band in two halves split at x ≈ 2760 instead of four segments split at x ≈ 1200/1840/2450, and each legend line cropped individually rather than as a column block. Examples:
  - `magick r600/p-12.png -crop 2000x200+820+1230 +repage -resize 250% band_L.png` and `-crop 2000x200+2760+1230` band_R
  - `magick r600/p-12.png -crop 1700x260+380+1600 +repage -resize 250% lg_12.png`, `-crop 1900x290+380+3050` (4 ~ 8 line), `-crop 1900x300+380+3700` (9, 10 line), `-crop 1900x150+380+2130` (③ heading), `-crop 1700x160+380+2280` (③ box)
  - `magick r600/p-12.png -crop 2300x180+2400+1640 +repage -resize 250% rg_11h2.png`, `-crop 1400x260+2500+1740 -resize 300%` (⑪ box), `-crop 2200x180+2450+2350` (12 ~ 14 line), `-crop 2300x150+2400+2470` (15 ~ 17 line), `-crop 2300x160+2400+2790` (18 ~ 27 line), `-crop 2300x330+2400+3260` (clear-list)
  - **Both passes were done. There were no disagreements between them** — every `field_index`, `index_style` and `label_verbatim` in the CSV read identically on the 400 dpi and the 600 dpi rasters. No cell required a third render.
- **Working-directory note.** The task directs renders into a fresh directory beneath `…/scratchpad/evidence/ic7610`. Whilst this leg was running, another process repeatedly created files in that directory and then deleted its entire contents, including files this leg had just written. Renders and crops were therefore made in a private directory, `…/scratchpad/L-ledger-work-ic7610/` (subdirectories `r300/`, `r400/`, `r600/`, `p1/`, `p2/`), created fresh at the start of that work, so that no file written by another process could be mistaken for this leg's evidence. No file created by that other process was opened or read. Only the two deliverables were written into `…/scratchpad/evidence/ic7610`.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every field index on PDF page 12, in all four blocks, is drawn the same way: a plain-weight numeral inside a thin open circle outline, black on white. Verified numeral by numeral at 400 % and again at 250–300 % on the 600 dpi raster; no filled or reversed, bracketed, bold, or bare-plain index appears anywhere in these diagrams. (The `ⓘ` glyph opening the note `ⓘSet 0 for P1 and P2.` is a circled letter, not a field index, and is not recorded.)
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** Nothing on PDF page 12 is set at an angle: every index, legend entry and leader label is horizontal. (Rotated labels do occur elsewhere in this document — the frequency-digit labels on PDF pages 11 and 13 — but not on the transcribed page, and those pages carry no memory-record block.) The question does not arise in any case, because the text layer was never consulted: `pdftotext` was not run, and position was read from the picture.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In D3 the two leader labels sit to the right of the box and their printed order runs opposite to the cells they point at. Followed by eye: the **upper** label `0: OFF, 1: TONE, 2: TSQL` is reached by the short leader rising from the arrow into the **right** cell, whilst the **lower** label `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3` is reached by the long leader that runs left, beneath the box, to the arrow into the **left** cell. Both crossings were confirmed on the 400 dpi and the 600 dpi crops. No CSV row is affected — neither leader label is a field index — but both are recorded in the `notes` of the D3 row with the cell each lands on stated. In D2 the leader order is not reversed: `Fixed` points at the left cell and the `0=OFF … 3= ★3` list at the right cell.
- **(d) A printed index may differ from a field's measured position — ENCOUNTERED, positions not recordable.** Blocks do repeat: D4 re-prints indices ①, ②, ③ and ④ that D1 has already printed, and D2 and D3 re-print D1's ③ and ⑪. All repeats are recorded as printed, in both places, and are STOPs 1–4 below. No byte position is recorded for any of them: this ledger's columns are index, style, label and location only, the task expressly forbids widths, byte positions and encodings here, and in any event D2, D3 and D4 print no cell run against D1's band from which a position could be measured. Nothing was reconciled and no index was reinterpreted in the light of another.

## STOP findings

1. **PDF page 12, left-hand column, middle third.** The two-cell box under the heading `③ Select memory setting` carries a circled `③` above it. `③` has already been printed as a field index in D1's byte band (third bracket group from the left). The same index is therefore printed twice on the page, in two different diagrams — a repeat in the index sequence. Both occurrences are transcribed exactly as seen: D1 row `3` and D2 row `3`, each with `STOP 1` in `notes`. Nothing is reconciled; the two are recorded as separate rows under separate `diagram_id`s.
2. **PDF page 12, right-hand column, upper third.** The two-cell box under the heading `⑪ Data mode and tone type settings` carries a circled `⑪` above it. `⑪` has already been printed as a field index in D1's byte band (sixth bracket group). A repeat, as at STOP 1. Both occurrences transcribed as seen: D1 row `11` and D3 row `11`, each with `STOP 2` in `notes`.
3. **PDF page 12, right-hand column, below the `⑱ ~ ㉗ Memory name settings` entry.** Under the heading `To clear the memory channel contents on 1A 00:` the page prints, verbatim:
   ```
   ①, ②:  Memory channel (00 01~00 99)
   ③:      “FF”
   ④:      None
   ```
   Indices ①, ②, ③ and ④ have all already been printed in D1, against different labels (`Memory channel numbers`, `Select memory setting`, and `④` only as the lower end of the range `④ ~ ⑧ Operating frequency setting`). This is a block that duplicates part of another block, and the labels differ between the two. All rows are transcribed exactly as seen, with the trailing colon included in `field_index` as printed, and `STOP 3` in `notes` on the D4 rows and on the D1 rows they duplicate. Neither label is carried over to the other block and neither is reinterpreted.
4. **PDF page 12, right-hand column, last line of the clear-list.** `④:      None` prints `④` as a standalone field index. Nowhere in D1 does `④` appear alone: the band and the legend print it only as the lower end of the range `④ ~ ⑧`, and no bracket group in the band is labelled `④` on its own. An index appearing singly in one block and only inside a range in another is an out-of-order/incomplete index sequence across the page. Transcribed exactly as seen — D4 row `4:` and D1 row `4 ~ 8`, each with `STOP 4` in `notes`. Neither is expanded, split or reconciled against the other.

No value on this page was unreadable, and no two-pass disagreement arose; there is no `UNREADABLE` cell in the CSV.

## Observed disagreements

Recorded as printed, not resolved.

- **Tilde spacing is inconsistent within the same section.** The band, and every legend entry, prints the range tilde with a space either side: `④ ~ ⑧`, `⑫ ~ ⑭`, `⑮ ~ ⑰`, `⑱ ~ ㉗`, and in the value list `00 01 ~ 00 99:   Memory channel 01 ~ 99`. The clear-list four lines lower prints the same range closed up: `Memory channel (00 01~00 99)`. `field_index` and `label_verbatim` follow the page in each place, so the CSV carries `4 ~ 8` (spaced) and `00 01~00 99` (closed up).
- **The clear-list indices carry a trailing colon; D1's do not.** The page prints `①, ②:`, `③:`, `④:` in D4 against `①, ②`, `③`, `④ ~ ⑧` in D1. The colon is transcribed verbatim as part of `field_index`, per the rule that trailing punctuation is included.
- **D1's labels are not printed inside the diagram.** The byte band prints only `X`, dotted cell dividers and two `...` ellipsis cells; nothing names a field within the band. Every D1 label comes from the legend entry printed against the same circled index below the band.
- **One cross-reference line serves two legend entries.** `⑫ ~ ⑭ Repeater tone frequency setting` and `⑮ ~ ⑰ Tone squelch frequency setting` are printed on consecutive lines with a single following line, `See "• Repeater tone/tone squelch settings."`, whereas every other legend entry that has a cross-reference has its own.
- **Singular/plural is not consistent across the legend.** `Select memory setting`, `Operating frequency setting`, `Operating mode setting`, `Repeater tone frequency setting` and `Tone squelch frequency setting` are singular; `Memory channel numbers`, `Data mode and tone type settings` and `Memory name settings` are plural. Transcribed as printed in each case.
- **The two `...` ellipsis cells sit inside numbered groups, not between them.** The first falls within the span of `④ ~ ⑧` and the second within the span of `⑱ ~ ㉗`, so the number of drawn cells under those two brackets is smaller than the number of indices the bracket names. No measurement or count is recorded here: this ledger records indices, not widths.
- **The information glyph differs from the field-index circles.** `ⓘSet 0 for P1 and P2.` uses a circled letter of the same open-outline construction as the field indices but is not a numbered field; it is not recorded as a row.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

*Scope note on the sentence above, so that it is not read as claiming more than is true:* directory listings were made, but only of this leg's own render and output directories (`…/scratchpad/L-ledger-work-ic7610/` and `…/scratchpad/evidence/ic7610`), to create them, to confirm which renders had been produced, and to establish that another process was deleting files there. No repository directory was listed, no file created by any other process was opened, and no listing was the source of any recorded value. `pdfinfo` was run once on this same PDF for the page count; `pdftotext` was not run at all.
