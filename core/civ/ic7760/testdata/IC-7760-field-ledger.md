# IC-7760 memory-record field ledger — leg L

Companion to `IC-7760-field-ledger.csv`.

## Source

- Document title as printed on the cover (PDF page 1): `CI-V REFERENCE GUIDE`, printed
  in a black band beneath the ICOM logo. Beneath it, on the lower half of the cover:
  `HF/50 MHz TRANSCEIVER` above the model name `IC-7760`, with `Icom Inc.` at the foot.
- Revision code as printed: `A7788-8EX-2`. It is printed at the bottom left of the back
  cover (PDF page 28), directly above the line `© 2024–2025  Icom Inc.      May 2025`.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7760_civ_2.pdf`
- Page count: 28 PDF pages (A4, 595.276 × 841.89 pt).

## Extent

Rendered: PDF pages 18–22 at 300 dpi (location sweep); PDF page 20 at 400 dpi and again
at 600 dpi (transcription and second pass); PDF pages 1 and 28 at 150 dpi, and PDF page 28
again at 400 dpi, for the `## Source` details only.

Read as images:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | (none printed) | Cover title, model, publisher. |
| 19 | 18 | Boundary before. Section `Remote control (CI-V) information` → `◇ Command formats` → `• Band stacking register`, `Command: 1A 01`. A different command block; no memory-record data block. |
| 20 | 19 | **All transcribed material.** Section `Remote control (CI-V) information` → `◇ Command formats` → `• Memory content`, `Command: 1A 00`. |
| 21 | 20 | Boundary after (next page). `• Keyer memory character entries` / `• Keyer memory content` (1A 02), `• IF filter width settings` (1A 03), `• AGC time constant settings` (1A 04), etc. Contains data-block diagrams, but none of a memory record. |
| 28 | (none printed) | Revision code and copyright line. |

PDF pages 18 and 22 were rendered in the location sweep but not read; pages 19–21 settled
the section boundaries.

The printed folio is the PDF page number minus one throughout (PDF 19 → folio 18,
PDF 20 → folio 19, PDF 21 → folio 20).

Where the transcribed material begins and ends, all on PDF page 20:

- Immediately before it: the bold sub-heading `• Memory content` and the line
  `Command: 1A 00`, themselves under `◇ Command formats` and the grey section bar
  `Remote control (CI-V) information`.
- The material itself: the numbered band and cell row of the 1A 00 data block (D1), its
  legend paragraphs in the left and right columns, and the two boxed sub-diagrams
  (D2 for index 3, D3 for index 11), ending with the block headed
  `To clear the memory channel contents on 1A 00:` in the right column.
- Immediately after it: the bold sub-heading `• Codes for character entries` and the line
  `Command: 1A 00,` in the left column, beginning the character-code tables that occupy
  the lower two-thirds of the page.

## Diagrams

Three distinct diagrams, numbered in page order (top to bottom, left column before right).

- **D1** — printed caption verbatim: `• Memory content` / `Command: 1A 00`. Position: top
  of PDF page 20, spanning the full page width beneath the caption; a single horizontal row
  of 18 two-nibble cell-pairs with a band of circled indices and square leader brackets
  above it. Eight numbered entries. Its labels are printed as legend paragraphs below the
  block, running down the left column (`1, 2` … `9, 10`) and continuing at the top of the
  right column (`11` … `18 ~ 27`).
- **D2** — printed caption verbatim: `3: Select memory setting` (the `3` circled). Position:
  left column of PDF page 20, immediately below that caption line and above the note
  `Set 0 for P1 and P2.`; a single two-nibble box holding `0` and `X`, with a circled `3`
  centred above it, an arrow from the left nibble to `Fixed`, and an arrow from the right
  nibble to a braced enum list `0=OFF` / `1= ★1` / `2= ★2` / `3= ★3`. One numbered entry.
- **D3** — printed caption verbatim: `11: Data mode and tone type settings` (the `11`
  circled). Position: top of the right column of PDF page 20, immediately below that caption
  line and above `12 ~ 14: Repeater tone frequency setting`; a single two-nibble box holding
  `X` and `X`, with a circled `11` centred above it and two leader arrows to the enum lines
  `0: OFF, 1: TONE, 2: TSQL` and `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`. One numbered
  entry.

### Scope judgement (recorded, not resolved)

The right column of PDF page 20 also carries a block headed
`To clear the memory channel contents on 1A 00:` whose three lines begin with circled
indices `1, 2`, `3` and `4`. It is a text list: no cells, no box, no band, nothing drawn.
I have therefore **not** given it a `diagram_id` and it contributes no CSV rows, on the
reading that a *data-block diagram* is one with drawn cells. Its contents are transcribed
verbatim under `## Observed disagreements`, and the one point on which it differs in form
from D1 is recorded as STOP 3 against the D1 row it bears on. A reader who takes the wider
reading should add it as a fourth block; nothing in it has been discarded.

### Label-source convention (recorded, not resolved)

Nothing is printed *inside* the numbered band of D1 except the indices themselves; the
labels for those eight indices are the legend lines printed below the block, each of which
repeats the index in the same circled style and follows it with a colon. Those legend
labels are what `label_verbatim` carries for the D1 rows, taken as the text after the
colon (so `1, 2: Memory group number` → `Memory group number`).

In D2 and D3 the circled index above the box has no label printed against it — the naming
line sits above the whole sub-diagram and is simultaneously D1's legend entry for that same
index. Those two `label_verbatim` cells are therefore **empty** (meaning: nothing is printed
here), with the heading line recorded in `notes` and `visual_anchor`.

## Method

- **dpi at each step.** Location sweep: `pdftoppm -png -r 300 -f 18 -l 22 <pdf> renders300/p`.
  First transcription pass: `pdftoppm -png -r 400 -f 20 -l 20 <pdf> renders400/p`
  (3308 × 4678 px). Second pass: `pdftoppm -png -r 600 -f 20 -l 20 <pdf> renders600/q`
  (4961 × 7016 px). Source details: `pdftoppm -png -r 150` on pages 1 and 28, plus
  `pdftoppm -png -r 400 -f 28 -l 28` for the revision code.
- **ImageMagick was available** (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`)
  and was used for every crop. First pass, from the 400 dpi raster:
  - `magick renders400/p-20.png -crop 2400x420+450+860 +repage -resize 200% crops/band_A.png`
  - `magick renders400/p-20.png -crop 1050x120+600+890 +repage -resize 400% crops/band_left.png`
  - `magick renders400/p-20.png -crop 1050x120+1600+890 +repage -resize 400% crops/band_right.png`
  - `magick renders400/p-20.png -crop 1350x340+180+1130 +repage -resize 250% crops/leftcol_1.png`
  - `magick renders400/p-20.png -crop 1350x520+180+1440 +repage -resize 250% crops/leftcol_2.png`
  - `magick renders400/p-20.png -crop 1350x420+180+1940 +repage -resize 250% crops/leftcol_3.png`
  - `magick renders400/p-20.png -crop 1500x560+1690+1130 +repage -resize 250% crops/rightcol_1.png`
  - `magick renders400/p-20.png -crop 1500x560+1690+1670 +repage -resize 250% crops/rightcol_2.png`
  - `magick renders400/p-20.png -crop 1500x520+1690+2200 +repage -resize 250% crops/rightcol_3.png`

  Second pass, from the 600 dpi raster (different windows, different enlargements):
  - `magick renders600/q-20.png -crop 1100x270+850+1300 +repage -resize 300% crops2/b1.png`
  - `magick renders600/q-20.png -crop 1100x270+1850+1300 +repage -resize 300% crops2/b2.png`
  - `magick renders600/q-20.png -crop 1200x270+2850+1300 +repage -resize 300% crops2/b3.png`
  - `magick renders600/q-20.png -crop 1600x200+900+1450 +repage -resize 250% crops2/cells1.png`
  - `magick renders600/q-20.png -crop 1600x200+2450+1450 +repage -resize 250% crops2/cells2.png`
  - `magick renders600/q-20.png -crop 2100x900+260+1690 +repage -resize 180% crops2/lc1.png`
  - `magick renders600/q-20.png -crop 2100x1000+260+2560 +repage -resize 180% crops2/lc2.png`
  - `magick renders600/q-20.png -crop 2300x900+2530+1690 +repage -resize 180% crops2/rc1.png`
  - `magick renders600/q-20.png -crop 2300x1200+2530+2560 +repage -resize 180% crops2/rc2.png`
  - `magick renders400/back-28.png -crop 900x220+100+4340 +repage -resize 300% crops2/rev2.png`

  Every numeral, tilde, comma, bracket descender and cell rule sat clear of its neighbours
  at these enlargements.
- **tesseract was available** (`/opt/homebrew/bin/tesseract`) and was **not used**. Every
  value was read by eye from the renders; nothing here rests on OCR.
- **`pdftotext` was never run**, in any form, on this or any file. Navigation to the section
  was done entirely by reading the 300 dpi page images of PDF pages 19, 20 and 21 and
  matching the printed headings. Accordingly the first attestation form applies.
- `pdfinfo` was run once, on this same PDF, for the page count and page size quoted under
  `## Source`. It was the source of no transcribed value, no index, no label and no position.
- Shell housekeeping, disclosed for completeness: `which` was run to establish which of
  `magick`, `convert`, `tesseract`, `pdftoppm`, `pdftotext` and `pdfinfo` exist on this
  machine; `ls -l` was run on the PDF's own absolute path; and the only directory listed at
  any point was my own output directory beneath
  `…/scratchpad/legs-out/ic7760/L`, to confirm the renders had been written. No repository
  directory, manual directory or other location was listed, searched or browsed.
- **Second independent pass.** Done, in full. The first pass read the numbered band from
  400 dpi crops split into two halves at x = 1600 and enlarged 400%, and the legends from
  400 dpi crops enlarged 250%. The second pass read the same material from a 600 dpi raster
  — a different render, not a rescale of the first — cropped into *three* band windows at
  different boundaries (so that every group that had straddled a first-pass crop edge sat
  mid-window the second time), enlarged 300%, plus two dedicated cell-row windows at 250%
  and four legend windows at 180%. The second pass reproduced all ten index strings, all ten
  index styles and all eight labels. **Cells where the two passes disagreed: none.** No third
  render was needed.
- Cell-row accounting, read off `crops2/cells1.png` and `crops2/cells2.png`: the block draws
  18 two-nibble cell-pairs, of which 16 print `X X` and 2 are dashed-outline cells printing
  an ellipsis. Left to right, with shading as printed: 1 shaded, 2 shaded, 3 unshaded,
  4 shaded, 5 shaded ellipsis, 6 shaded, 7 unshaded, 8 unshaded, 9 shaded, 10 unshaded,
  11 unshaded, 12 unshaded, 13 shaded, 14 shaded, 15 shaded, 16 shaded, 17 shaded ellipsis,
  18 shaded. The brackets and ellipses account for indices 1–27 with no gap and no overlap.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** All ten recorded
  indices, in all three diagrams, and every repetition of them in the legend lines, are drawn
  the same way: an unfilled circle of uniform thin stroke enclosing black numerals, with the
  circle widening into an ellipse for the two-digit numbers (10, 11, 12, 14, 15, 17, 18, 27).
  Nothing filled, reversed, outlined, bracketed or bold appears; `index_style` is `circled`
  on every row and no style was normalised to reach that.
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** Every label,
  index, enum line and caption on PDF page 20 is set horizontally; no rotated or vertical
  text appears anywhere in D1, D2 or D3. Position was in any case read from the picture
  throughout. I can say nothing about text-layer extraction order because no text layer was
  extracted: `pdftotext` was never run.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In D3 the two arrows rising
  from the box cross: the arrow from the **left** nibble runs down and right to the **lower**
  label line `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`, while the arrow from the **right**
  nibble runs to the **upper** line `0: OFF, 1: TONE, 2: TSQL`. The vertical order of those
  two labels is thus the reverse of the horizontal order of the cells they point at. In D2 a
  milder form: the arrow from the right nibble travels right, passing over the word `Fixed`,
  to reach the braced enum list, while `Fixed` belongs to the left nibble. Each leader was
  followed by eye from label to cell. No numbered index is affected — both sub-diagrams carry
  exactly one index each — so no CSV row changes, but the crossing is recorded here as a fact
  about the page.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED.** No
  block of fields on this page repeats another block: D2 and D3 are single-field expansions
  of D1's indices 3 and 11, not repeated blocks, and D1's own band contains no repeated run.
  Counting cell-pairs left to right on the render and expanding the two dashed ellipsis cells
  as instructed by the brackets that span them, each printed index falls at the ordinal
  position its number implies, from 1 at the left-hand cell-pair to 27 at the right-hand one.
  No measurement was reconciled against a printed index because none conflicted, and no byte
  positions are recorded, the task having excluded them.

## STOP findings

1. **PDF page 20 — index `3` is printed as an index in two different diagrams.** Visual
   anchor: the circled `3` standing alone, without a bracket, above the 3rd (unshaded)
   cell-pair of the D1 band near the top of the page; and, lower down the left column,
   the circled `3` centred above the two-nibble box of D2. What is
   printed: the same circled numeral `3`, in the same style, in both places (and a third and
   fourth time in the left-column legend line `3: Select memory setting` and in the
   `To clear the memory channel contents on 1A 00:` list). Why it stops: the STOP rules count
   a repeated index as a discontinuity in the index sequence, and index 3 is printed more than
   once on the page. Both occurrences are transcribed exactly as seen, as `D1,3` and `D2,3`,
   each carrying `STOP 1` in `notes`. Not resolved: I have not decided that one occurrence is
   the "real" one.
2. **PDF page 20 — index `11` is printed as an index in two different diagrams.** Visual
   anchor: the circled `11` standing alone, without a bracket, above the 9th (shaded)
   cell-pair of the D1 band; and the circled `11` centred above the two-nibble box of D3 at
   the top of the right column, under the line `11: Data mode and tone type settings`. What is
   printed: the same circled numeral `11` in both places. Why it stops: as STOP 1 — a repeat
   in the index sequence. Both occurrences transcribed as seen, `D1,11` and `D3,11`, each
   carrying `STOP 2` in `notes`.
3. **PDF page 20 — index `4` is printed alone where the diagram prints it as the start of a
   range.** Visual anchor: the right column, third line of the block headed
   `To clear the memory channel contents on 1A 00:`, immediately below the line `3:      “FF”`
   and immediately above white space; and, for comparison, the D1 band entry at the top of the
   page and its legend line in the left column. What is printed: in the clear-contents list,
   `4:      None`; in the band and legend, `4 ~ 8` and `4 ~ 8: Operating frequency setting`.
   Why it stops: the same circled index glyph is printed on one part of the page as a bare
   single index and on another as the lower bound of a five-field range, both within the same
   page and the same command, without any note reconciling them. It is
   transcribed exactly as seen: the CSV row `D1,4 ~ 8` carries the range exactly as the band
   prints it, with `STOP 3` in `notes`; the bare `4:      None` is transcribed verbatim under
   `## Observed disagreements`. Neither form has been altered, averaged or preferred.

## Observed disagreements

Recorded exactly as printed; not resolved.

1. The block in the right column of PDF page 20 headed `To clear the memory channel contents
   on 1A 00:` re-uses three of D1's indices with different labels from the ones its own
   legend gives them. Transcribed verbatim, spacing as printed:

   ```
   To clear the memory channel contents on 1A 00:
   1, 2:  Memory channel (00 01~00 99)
   3:      “FF”
   4:      None
   ```

   (indices circled in each case). Against D1's legend, which reads
   `1, 2: Memory group number`, `3: Select memory setting` and
   `4 ~ 8: Operating frequency setting`. The `4` line is separately recorded as STOP 3.
2. Tilde spacing is inconsistent between the two blocks for what appears to be the same
   range. The left-column legend prints `00 01 ~ 00 99: Memory channel 01 ~ 99` with a space
   each side of every tilde; the clear-contents list prints `Memory channel (00 01~00 99)`
   with no spaces. Both were read at 600 dpi and both readings held on the second pass.
3. Within D1's legend, the label given for indices `1, 2` is `Memory group number`, but the
   three lines that immediately follow and qualify it are about a *channel*:
   `00 01 ~ 00 99: Memory channel 01 ~ 99`, `01 00:            Programmed scan edge P1`,
   `01 01:            Programmed scan edge P2`. Group and channel are used of the same two
   fields on the same page.
4. D2's caption calls the field a *setting* and the box beneath it prints a literal `0` in the
   left nibble with the word `Fixed` below the arrow that points at it, so only the right
   nibble of that field is variable. Recorded because the ledger's single `D2,3` row cannot
   express it.
5. The enum list in D2 prints `0=OFF` with no space either side of the equals sign, but
   `1= ★1`, `2= ★2`, `3= ★3` with a space after it. The `★` glyph is drawn as a solid
   five-pointed star and was read from the render.
6. From the boundary sweep, not from the transcribed material: the NOTE box on PDF page 19
   (folio 18) points into this page with `* See 4 ~ 17 on "Memory content." (p. 19)`, the
   indices circled. That range crosses five of D1's eight labelled groups without naming
   them, and it cites the page by its printed folio (19), which is PDF page 20. Noted only
   because it references D1's indices; it was not used as evidence for any recorded value.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual,
transcription, source file, generated artefact or web resource was opened, and no directory
was listed.
