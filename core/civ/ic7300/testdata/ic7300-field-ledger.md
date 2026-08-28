# IC-7300 field ledger — memory-record data-block diagrams

## Source

- Document title as printed on the cover (PDF page 1, rendered at 150 dpi):
  `FULL MANUAL` in the black banner, with `HF/50 MHz TRANSCEIVER` and the model
  wordmark `IC-7300` beneath it, and `Icom Inc.` at the foot.
- Revision code as printed: `A7292-4EX-12b`, printed at the bottom left of the
  final page (PDF page 180), directly above `© 2016–2024 Icom Inc.   Aug. 2024`.
  No revision code is printed on the cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300_fullmanual_ENG_12b.pdf`
- Page count: 180 PDF pages. Confirmed from the renderer: `pdftoppm -f 181 -l 181`
  refuses with "the first page (181) can not be after the last page (180)",
  while page 180 renders.

## Extent

| PDF page | printed folio | rendered at | what it contributed |
|---|---|---|---|
| 1 | (none printed) | 150 dpi | Cover title, model, `FULL MANUAL` banner. Source section only. |
| 168 | `19-10` | 300 dpi | Context only: establishes what is printed immediately before the transcribed material. No value recorded from it. |
| 169 | `19-11` | 300, 400 and 600 dpi | The only page transcribed. All three diagrams and the whole legend. |
| 170 | `19-12` | 300 dpi | Context only: establishes what is printed immediately after. No value recorded from it. |
| 180 | (none printed) | 150 dpi | Revision code. Source section only. |

The transcribed material sits on PDF page 169 (printed folio `19-11`), inside
the section whose black banner heading reads `Remote control (CI-V) information`,
under the chapter running head `19  CONTROL COMMAND`.

- It begins with the bold bullet caption `• Memory content` and the line
  `Command : 1A 00`, followed immediately by the data-block strip. Nothing of
  the memory record is printed above that caption on page 169; the banner
  heading is the only thing between it and the running head.
- It ends with the `NOTE:` block in the right column, whose last line is
  `that you set the same data as ④–⑰.` The remainder of page 169 below that
  block is blank.
- Immediately before, on PDF page 168 (folio `19-10`), the last items printed
  are `• Offset frequency settings` (left column) and
  `• Data mode with filter width settings` (right column). Neither is a
  memory-record data block.
- Immediately after, on PDF page 170 (folio `19-12`), the first item printed is
  `• Memory keyer character entries`, `Command: 1A 02`.

### Diagrams identified, in page order

Page order is taken as the two-column reading order of the page: left column
top to bottom, then right column top to bottom. The manual's own field
numbering runs in exactly that order down the page (①,② → ③ → ④~⑧ → ⑨,⑩ in
the left column, then ⑪ → ⑫~⑭ → ⑮~⑰ → ⑱~㉗ in the right), which is why that
order was used rather than raw vertical position.

- **D1** — caption printed verbatim: `• Memory content` with `Command : 1A 00`
  on the line below. Position: the full-width byte strip immediately beneath
  that caption, spanning both columns across the upper third of the page.
  Nine numbered groups are printed above the strip.
- **D2** — no caption of its own; the heading printed verbatim immediately
  above it is `③ Split and Select memory setting`. Position: left column,
  upper-middle of the page, a single two-nibble box drawn `X : X` with the
  circled index above it and two leader arrows below it.
- **D3** — no caption of its own; the heading printed verbatim immediately
  above it is `⑪ Data mode and tone type settings`. Position: right column,
  upper part of the page, level with the `③ Split and Select memory setting`
  heading in the left column; a single two-nibble box drawn `X : X` with the
  circled index above it and two leader arrows below it.

### Where the labels come from

D1's strip prints only the index groups above the byte cells; no label text is
printed inside or beside the strip. The labels recorded for D1 are the legend
entries printed below the strip, which repeat each index and then give its
name (`①, ② Memory channel numbers`, `③ Split and Select memory setting`, and
so on). Each such pairing is by the printed index, not by inference. The one
group with no legend entry anywhere on the page — `❹–⓱` — is recorded with an
empty label. For D2 and D3 the label recorded is the section heading printed
immediately above the box; nothing is printed against the numeral inside those
boxes.

## Method

1. **Locate.** `pdftoppm -png -r 300 -f 168 -l 170 <pdf> r300/p` into a fresh
   directory `evidence/ic7300-L/r300/`. Pages 168, 169 and 170 read as images
   to confirm which page carries the memory-record material and where the
   section begins and ends. Cover and last page rendered separately at 150 dpi
   (`-f 1 -l 1`, `-f 180 -l 180`) for the Source section.
2. **Read (first pass).** `pdftoppm -png -r 400 -f 169 -l 169 <pdf> r400/p`
   (3308×4678 px). Every first-pass value was read from crops of that raster.
3. **Crop and enlarge.** ImageMagick was available (`/opt/homebrew/bin/magick`,
   `/opt/homebrew/bin/convert`) and was used. First-pass commands, all against
   `r400/p-169.png`:
   - strip, left half: `-crop 1300x260+400+870 +repage -resize 250%`
   - strip, right half: `-crop 1300x260+1650+870 +repage -resize 250%`
   - caption: `-crop 900x180+230+740 +repage -resize 350%`
   - legend `①, ②`: `-crop 800x140+230+1120 +repage -resize 400%`
   - legend `③`: `-crop 800x140+230+1440 +repage -resize 400%`
   - legend `④~⑧`: `-crop 900x300+230+2140 +repage -resize 300%`
   - legend `⑨, ⑩`: `-crop 900x130+230+2385 +repage -resize 350%`
   - legend `⑪`: `-crop 1300x110+1700+1120 +repage -resize 350%`
   - legend `⑫~⑭`/`⑮~⑰`: `-crop 1400x290+1700+1660 +repage -resize 250%`
   - legend `⑱~㉗`: `-crop 1400x110+1700+1955 +repage -resize 350%`
   - `To clear …` list: `-crop 1400x110+1700+2290 +repage -resize 350%` and
     `-crop 1400x300+1700+2320 +repage -resize 250%`
   - `NOTE:` block: `-crop 1500x480+1700+2610 +repage -resize 200%`
   - D2 box: `-crop 1100x680+280+1520 +repage -resize 250%`
   - D3 box: `-crop 1100x420+1740+1200 +repage -resize 250%`
4. **`pdftotext`** was **not** run at any point, in any form. No text layer was
   extracted, examined or consulted.
5. **`tesseract`** is installed (`/opt/homebrew/bin/tesseract`) but was **not**
   used. Every glyph was legible by eye on the enlarged crops, so no OCR aid
   was needed and no OCR value entered the ledger.
6. **Second independent pass, done.** Page 169 was re-rendered at a different
   dpi — `pdftoppm -png -r 600 -f 169 -l 169 <pdf> r600/p` (4961×7016 px) — and
   every recorded value was re-read from that raster through different crop
   windows and different enlargements, deliberately splitting the strip at
   different points so that no group fell at the same place in the frame as in
   the first pass:
   - strip in three overlapping thirds: `-crop 1300x420+600+1280 +repage -resize 180%`,
     `-crop 1300x420+1850+1280 +repage -resize 180%`,
     `-crop 1400x420+3050+1280 +repage -resize 180%`
     (first pass had used two halves at 400 dpi and 250%)
   - each legend heading individually: `-crop 1600x150+340+1650`,
     `-crop 1600x160+340+2130`, `-crop 1600x160+340+3290`,
     `-crop 1600x150+340+3550`, `-crop 1900x160+2560+1650`,
     `-crop 1900x200+2560+2470`, `-crop 1900x160+2560+2570`,
     `-crop 1900x160+2560+2915`, all `+repage -resize 200%`
   - D2 and D3 index numerals in isolation: `-crop 700x300+560+2260` and
     `-crop 700x300+2800+1830`, both `+repage -resize 300%`
   - the `④–⑧` separator glyph at extreme magnification:
     `-crop 320x140+1370+1420 +repage -resize 600%`

   **Result: the two passes agreed on every cell.** No disagreement arose, so
   no third render was needed to settle one, and no cell is recorded from a
   contested reading.
7. Other tools touching this PDF: `pdfinfo` was run once against the same file
   while establishing the page count; the page count recorded above was then
   confirmed independently from the renderer as described in Source, and no
   value in the CSV came from `pdfinfo`. `ls` was run on my own render output
   directory (`evidence/ic7300-L/r300/`) to learn the filenames `pdftoppm` had
   produced — that is, a listing of files this leg itself created from this
   PDF, inside the working directory the task assigns. No repository
   directory, no manual directory and no other directory anywhere was listed,
   and no file outside `evidence/ic7300-L/` was opened; that is the sense in
   which the attestation below is given.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — ENCOUNTERED.** D1 draws
  its index numerals in two ways: eight groups use outlined circled numerals
  (black numeral inside a thin black circle on white), and one group, `❹–⓱`,
  uses negative/reversed numerals (white numeral on a solid black disc). Both
  styles appear above the same strip, roughly 5 cm apart. Recorded as `circled`
  and `filled` respectively; neither has been normalised to the other, and no
  meaning has been inferred for the difference. The same two styles recur in
  the `NOTE:` prose, which prints both `④–⑰` (outlined) and `❹–⓱` (filled) in
  the same sentence.
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.**
  No rotated or vertical label appears anywhere in the three diagrams on page
  169; every index and label is set horizontally. (Page 168 does carry rotated
  labels, but nothing was recorded from page 168.) In any case no text layer
  was consulted, so extraction order could not have influenced any row: every
  position was read from the picture.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In both D2 and
  D3 the leader arrows cross. In D2 the arrow rising from the **right** nibble
  runs to the **upper** label group (`0=OFF`, `1= ★1`, `2= ★2`, `3= ★3`), while
  the arrow from the **left** nibble runs down and across to the **lower**
  label (`0=Split OFF, 1=Split ON`). D3 is the same shape: the **right**
  nibble's arrow lands on the upper label (`0: OFF, 1: TONE, 2: TSQL`) and the
  **left** nibble's arrow on the lower one (`0=Data mode OFF` / `1=Data mode
  ON`). Reading top-to-bottom would attach both labels to the wrong nibble.
  Each arrow was followed by eye at 300% enlargement. No CSV row depends on
  this (those labels hang off unnumbered nibbles, not off numbered fields), but
  it is recorded because it is exactly the reversal the hazard describes.
- **(d) A printed index may differ from a field's measured position —
  CANNOT DETERMINE.** D1 does contain a block that repeats another: `❹–⓱`
  repeats the indices `④` to `⑰`, and the `NOTE:` beneath states in as many
  words that "The same data as ④–⑰ are stored in ❹–⓱." The repeated block,
  however, is not drawn as discrete cells at all: it is a dashed-outline region
  containing a single row of dots, with no `X:X` cells inside it, so there is
  no extent on the render from which a position could be measured for any of
  its fields. Nothing was measured, nothing was reconciled, and nothing was
  reinterpreted; the printed index is recorded exactly as printed. The task's
  ledger schema also deliberately excludes measured positions ("no widths, no
  byte positions"), so none is recorded for any row.

## STOP findings

1. **PDF page 169; top data-block strip (D1), the label above the dashed
   abbreviated region between the `⑮–⑰` bracket and the `⑱–㉗` bracket.**
   Printed there is `❹–⓱`. Reading the strip left to right the printed index
   sequence runs `①, ②` · `③` · `④–⑧` · `⑨, ⑩` · `⑪` · `⑫–⑭` · `⑮–⑰` ·
   `❹–⓱` · `⑱–㉗`. This is a discontinuity on two counts: the indices 4 to 17
   are printed a second time, having already been printed in the `④–⑧`,
   `⑨, ⑩`, `⑪`, `⑫–⑭` and `⑮–⑰` groups; and the sequence goes backwards, from
   17 to 4 and then forward again to 18. Printed order was followed, not
   numeric order. Transcribed into the CSV exactly as seen, as `❹–⓱` in the
   eighth D1 row, with `STOP 1` in that row's notes.

2. **PDF page 169; same strip, same label `❹–⓱`, compared with the `④–⑧` group
   earlier in the same strip.** The numerals 4 and 17 are each printed twice in
   one diagram in two different styles: outlined circled (`④` in `④–⑧`, `⑰` in
   `⑮–⑰`) and filled/reversed (`❹` and `⓱`). An index printed twice with
   different styling is a stop by rule. Both styles are transcribed as drawn —
   `circled` for the eight outlined groups, `filled` for `❹–⓱` — and neither
   was normalised. `STOP 2` in the notes of the `❹–⓱` row.

3. **PDF page 169; the four ranged groups of the strip (D1) against the legend
   entries printed below it.** The strip prints its ranges with a dash and the
   legend prints the same ranges with a tilde. Printed on the strip:
   `④–⑧`, `⑫–⑭`, `⑮–⑰`, `⑱–㉗`. Printed in the legend for the same four
   ranges: `④~⑧ Operating frequency setting`, `⑫~⑭ Repeater tone frequency
   setting`, `⑮~⑰ Tone squelch frequency setting`, `⑱~㉗ Memory name
   settings`. Two printings of the same index disagree, and because
   `field_index` is a verbatim join key the choice is load-bearing rather than
   cosmetic. Both readings were confirmed on the 600 dpi second pass, including
   the `④–⑧` separator at 600% enlargement, where the strip's separator is
   unambiguously a single horizontal bar (drawn in a heavier weight than the
   circle outlines beside it) and the legend's is unambiguously a tilde. The
   CSV records the form printed on the diagram itself — the dash — in
   `field_index`, and names the legend's tilde form in each affected row's
   notes. `STOP 3` in the notes of the `④–⑧`, `⑫–⑭`, `⑮–⑰` and `⑱–㉗` rows.
   Caveat stated plainly: a raster cannot distinguish U+2013 EN DASH from
   U+2212 MINUS SIGN or a drawn rule of the same length. The glyph is
   transcribed as `–` (en dash), matching the dash used elsewhere on the page
   in `00 01–00 99:` and in the `NOTE:` line `④–⑰`. What is certain from the
   render is that it is a single horizontal stroke and not a tilde.

## Observed disagreements

Recorded as printed; not resolved, and none of these stopped the transcription.

- The bullet caption on this page prints a space before its colon,
  `Command : 1A 00`, whereas the caption immediately preceding it on PDF page
  168 prints `Command: 1A 01` with no space. Page 168 itself prints both forms
  (`Command: 1A 01` and `Command : 1A 05    00 31,  00 32`).
- The legend entries are not uniform in weight. `①, ② Memory channel numbers`,
  `③ Split and Select memory setting`, `④~⑧ Operating frequency setting`,
  `⑨, ⑩ Operating mode setting` and `⑪ Data mode and tone type settings` are
  set in regular weight; `⑫~⑭ Repeater tone frequency setting`,
  `⑮~⑰ Tone squelch frequency setting` and `⑱~㉗ Memory name settings` are set
  in bold. The index numerals themselves are outlined circled in all eight.
- The comma spacing of the two-index pair differs between places on the page.
  The strip and the legend print `①, ②` with a space after the comma; the
  `To clear the memory channel contents on 1A 00:` list in the right column
  prints `①,②:` with no space and a trailing colon.
- That same list prints `④:` followed by `None`. Index `④` is used there for
  something other than the `④~⑧ Operating frequency setting` field of the
  legend. The list is not a diagram and contributes no CSV row; it is noted
  because it is a third printing of index 4 on the page.
- The list also prints the channel range as `Memory channel (00 01~00 99)` with
  a tilde, while the legend eight lines above prints the same range as
  `00 01–00 99:` with a dash — the same dash/tilde inconsistency as STOP 3, in
  a place that carries no field index.
- D1's `③` and `⑪` groups are drawn without a bracket: the circled numeral
  simply sits above its single cell, whereas every multi-cell group carries a
  bracket. This is consistent within the diagram (one cell, no bracket) and is
  recorded only as an observation.
- Cell counts on the strip agree with the index ranges wherever the strip draws
  its cells out in full: `①, ②` two cells, `⑨, ⑩` two, `③` one, `⑪` one,
  `⑫–⑭` three, `⑮–⑰` three. The two long ranges are drawn abbreviated —
  `④–⑧` as three cells whose middle cell is a dashed `...` cell, and `⑱–㉗`
  likewise — so no arithmetic disagreement arises from them; the ellipsis is
  the diagram's own stated abbreviation, not a shortfall.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource
was opened, and no directory was listed.
