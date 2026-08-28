# IC-705 field ledger — memory-record data-block diagrams, PDF page 19

## Source

- Title as printed on the cover (PDF page 1): `CI-V REFERENCE GUIDE`, printed in
  reversed type in the black band beneath the ICOM logo. Beneath it, in the
  ruled panel: `HF/VHF/UHF ALL MODE TRANSCEIVER` over `IC-705`, and at the foot
  `Icom Inc.`
- Revision code: **no revision code is printed on the cover.** The only
  revision-style code printed in the document is `A7560-8EX-6`, at the
  bottom-left of the back cover (PDF page 31), immediately above
  `© 2020–2023  Icom Inc.     Jan. 2023`. The trailing `-6` of that code is the
  only printed thing resembling the "rev6" in the file name; the file name is
  not printed content and is not treated as evidence here.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic705_civ_rev6.pdf`
- Page count: 31 PDF pages (A4, 595.276 × 841.89 pt).

## Extent

| PDF page | Printed folio | What it contributed |
| --- | --- | --- |
| 1 | none printed | Cover: document title; confirmation that no revision code is printed there. |
| 19 | `18` (centred at the foot of the page) | **The whole of the transcribed material.** All 22 CSV rows come from this page. |
| 31 | none printed | Back cover: the code `A7560-8EX-6` and the date line, for the `## Source` section only. No ledger value came from this page. |

Pages 1 and 31 were rendered and read for document identification only. Page 19
is the only page transcribed.

The material transcribed begins on PDF page 19 with the section heading
`◇ Command formats`, then the bold bullet `• Memory content` and the line
`Command: 1A 00`. Immediately below `Command: 1A 00` the two-row byte band (D1)
is drawn. Immediately after the band's lower row the two legend columns begin,
the left one opening `①, ②: Memory group number`. The transcribed material ends
with the last numbered index on the page, the circled `⑮` above the `X 0`
detail box (D4) in the left column; what is printed immediately after it is the
unnumbered value list `0=Digital squelch function OFF` /
`1=Digital call sign squelch function ON (DSQL)` /
`2=Digital code squelch function ON (CSQL)`, and then the folio `18`.

Nothing above `◇ Command formats` on page 19 (the running head
`REMOTE CONTROL` and the reversed section bar
`Remote control (CI-V) information`) contains a numbered field.

## Diagrams on the page

| id | Printed caption, verbatim | Position on the page |
| --- | --- | --- |
| D1 | `• Memory content` / `Command: 1A 00` | Full page width, directly beneath `Command: 1A 00`; two rows of byte cells joined left-to-right by a wrap arrow that leaves the right end of the upper row and re-enters the left end of the lower row. |
| D2 | no caption of its own; sits under the legend heading `⑤: Split and Select memory setting` | Left legend column, upper third, above the line `* Set 0 for Call channel.` |
| D3 | no caption of its own; sits under the legend heading `⑭: Duplex and Tone settings` | Left legend column, lower third, above the heading `⑮: Digital squelch setting`. |
| D4 | no caption of its own; sits under the legend heading `⑮: Digital squelch setting` | Left legend column, bottom of the page, above the value list beginning `0=Digital squelch function OFF`. |

Judgement recorded for an adjudicator: D2–D4 are single-field detail insets
drawn as data-block cells. They carry a printed numbered index (⑤, ⑭, ⑮
respectively, each set above the left-hand half of a two-nibble box), so they
are included here on the instruction to record every numbered field. They have
no captions of their own. If a crosscheck treats only D1 as a
"memory-record data-block diagram", D2–D4 are the three rows to drop; no D1 row
depends on them.

**Label convention used.** In D1 no text is printed inside or beside the band
itself; the indices are printed bare above their cells. `label_verbatim` is
therefore the legend text printed against the same index in the two legend
columns below the band — the text following the colon on the index's own line,
plus any indented continuation of that same sentence on the next line (joined
with one space), e.g. `UR (Destination) call sign setting (8 characters,
fixed)`. Excluded from the label: the separate `ⓘSee "…" (p. n)`
cross-reference lines, and the separate value lines that follow some entries
(`0000 ~ 0099: Memory channel group`, `1 byte data (XX)`, `00: Data mode OFF`
and the like). The same convention is applied to D2–D4, whose labels come from
the heading line immediately above each inset.

## Method

- **dpi.** Locate: 300 dpi, `pdftoppm -png -r 300 -f 19 -l 19 … renders300/p`.
  Read (first pass): 400 dpi, `pdftoppm -png -r 400 -f 19 -l 19 … renders400/p`
  (3308 × 4678 px). Read (second pass): 600 dpi,
  `pdftoppm -png -r 600 -f 19 -l 19 … renders600/p` (4961 × 7016 px).
  Cover and back cover: 200 dpi.
  All renders were written into a freshly created directory tree beneath
  `/private/tmp/claude-501/-Users-stuart-Documents-working-coding-ft710-programmer-nosync/ee1244d4-2a22-475a-8e74-73144a21d2b7/scratchpad/evidence/ic705-L`, so no pre-existing file could be mistaken for evidence.
- **ImageMagick: available and used** (`/opt/homebrew/bin/magick`).
  First-pass crops, from the 400 dpi render into `crops/`:
  - `-crop 1400x260+200+1010 +repage -resize 300%` (band upper row, left half)
  - `-crop 1400x260+1550+1010 +repage -resize 300%` (band upper row, right half)
  - `-crop 1300x300+250+1230 +repage -resize 300%` (band lower row, left half)
  - `-crop 1300x300+1450+1230 +repage -resize 300%` (band lower row, right half)
  - `-crop 1500x800+230+1500`, `-crop 1500x800+230+2280`,
    `-crop 1500x900+230+2780`, `-crop 1500x900+230+3600`, each `+repage
    -resize 200%` (left legend column, four overlapping bands)
  - `-crop 1450x750+1700+1500`, `-crop 1450x750+1700+2230`,
    `-crop 1450x750+1700+2960`, each `+repage -resize 200%` (right legend
    column) and `-crop 1450x520+1700+3560` (the grey NOTE box)
  - `-crop 2000x420+230+700 +repage -resize 200%` (the `◇ Command formats` /
    `• Memory content` / `Command: 1A 00` heading)
- **tesseract: available but NOT used.** Every glyph was legible by eye on the
  enlarged crops, so no OCR aid was needed and no value in this ledger came
  from OCR.
- **`pdftotext -layout`: NOT run.** Navigation was done entirely from the 300
  dpi whole-page render. No text-layer extraction of any kind was performed on
  this document.
- **Second independent pass.** After the first pass was complete the whole page
  was re-rendered at 600 dpi (a different raster from the 400 dpi first pass)
  and every index, index style and label was re-read from crops taken with
  different windows and different enlargements — the band split into three
  overlapping thirds per row at `-resize 250%` (`crops2/r1a`–`r1c`,
  `r2a`–`r2c`, each `1450x400`) instead of two halves at 300%, and the legend
  columns re-cut as four `2200`-wide blocks at `-resize 150%`
  (`crops2/legL_a`, `legL_b`, `legR_a`, `legR_b`) instead of the first pass's
  `1450`–`1500`-wide blocks at 200%.
  **Disagreements between the two passes: none.** Every field index, every
  index style (outline circle vs solid black disc) and every label string was
  read identically in both passes, including the two identically worded
  `Repeater tone frequency setting` lines, the solitary `⑪` and `⑫` over
  separate cells, and the reversed numerals of `❻ ~ ❺❷`.

## Hazards encountered

- **(a) Numeral styling varying within one diagram — ENCOUNTERED.** D1 draws
  its indices in two styles. Every index from `①, ②` through `㊺~52` and again
  at `53~68` is a plain numeral inside an open outline circle on white
  (`circled`). The single group `❻ ~ ❺❷`, between `㊺~52` and `53~68` in the
  lower row, is drawn as white reversed numerals inside solid black filled
  discs (`filled`). The two styles are recorded separately and are not
  normalised; no meaning is inferred for either.
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED.** All text in and
  around both the band and the three detail insets is set horizontally; no
  rotated or vertical label was seen at any magnification. The question of
  text-extraction order does not arise here because no text layer was read —
  every value was taken from the picture.
- **(c) Leader-line label order reversed — ENCOUNTERED.** In the D2 inset (⑤)
  two label groups are stacked at the right of the box: the upper bracketed
  group `0=OFF*` / `1= ★1` / `2= ★2` / `3= ★3` runs a leader left and up to an
  arrowhead beneath the box's **right**-hand nibble, whilst the lower single
  label `0=Split OFF,1=Split ON` runs a long leader left to an arrowhead
  beneath the box's **left**-hand nibble. Read top-to-bottom the labels
  therefore address the cells right-then-left. Each leader was followed by eye
  from label to arrowhead. No value recorded in the CSV depends on this: these
  value labels are not part of the ledger, which records field indices and
  their labels only. (The D3 inset (⑭) is drawn the same way but its two label
  groups sit left-lower → left cell and right-upper → right cell, so its order
  is not reversed.)
- **(d) A printed index differing from a field's measured position —
  ENCOUNTERED, but not measurable as a byte position.** The `❻ ~ ❺❷` group
  prints indices 6 to 52 at a position in the byte stream that lies *after*
  `㊺~52` and *before* `53~68`, i.e. its printed index cannot be its position
  in the block. The two are recorded side by side and are not reconciled:
  `field_index` is `6~52` exactly as printed, and `visual_anchor` states where
  in the row it actually sits. A per-field byte position cannot be measured
  because the block is drawn wholly elided — a single wide shaded
  dashed-outline cell containing only a run of dots, with no individual byte
  cells and no printed extent. This task's own brief also forbids recording
  byte positions ("no widths, no byte positions"), so none are recorded.

## STOP findings

1. **PDF page 19 — index-sequence discontinuity: indices 6 to 52 are printed
   twice.** Visual anchor: lower row of the D1 byte band, the bracket group
   sitting between the `㊺ ~ 52` group and the `53 ~ 68` group, above a single
   wide shaded dashed-outline cell of dots. What is printed there is
   `❻ ~ ❺❷`. Reading the band in printed order the index sequence runs
   `… ㊲~㊹, ㊺~52, ❻~❺❷, 53~68`, so the range 6–52 recurs, out of numeric
   order, after 45–52 and before 53–68. Printed order and numeric order
   disagree here; printed order has been followed, and the row is transcribed
   exactly as seen (`6~52`) in CSV row `D1,6~52`. Nothing has been reordered,
   renumbered or reconciled. Note also that this is the only index group on the
   page with **no** legend line printed against it: it is mentioned only inside
   the grey NOTE box at the foot of the right column
   (`• The same data as ⑥ ~ 52 are stored in ❻ ~ ❺❷.`), and its
   `label_verbatim` cell is therefore empty.
2. **PDF page 19 — the same index range printed twice in two different
   styles.** Visual anchor: the circled `⑥ ~ ⑩` group in the **upper** row of
   the D1 band (and the circled `⑥ ~ 52` references in the legend and NOTE
   text) against the filled `❻ ~ ❺❷` group in the **lower** row. The numerals 6
   and 52 appear on this page both as plain numerals in open outline circles
   and as reversed white numerals in solid black discs. Both stylings are
   recorded as drawn — `circled` for the upper-row and legend occurrences,
   `filled` for the lower-row group — and neither is normalised to the other,
   nor is any meaning inferred for the difference. Transcribed in CSV row
   `D1,6~52` with `index_style` = `filled`.

No other STOP arises. Reasons for confidence on the remainder: every numeral,
bracket, tilde and comma in D1 was legible without ambiguity at 400 dpi
enlarged 300% and again at 600 dpi enlarged 250%; the D1 sequence is otherwise
continuous with neither gap nor repeat from `①, ②` to `53~68`; and the four
counted ranges that the page itself annotates with a character count agree with
their printed indices (`㉙~㊱` and `㊲~㊹` and `㊺~52` each span 8 indices against
`(8 characters, fixed)`, and `53~68` spans 16 against `(16 characters,
fixed)`). No arithmetic was recorded that could fail to add up, because this
ledger records no widths, positions or totals.

## Observed disagreements

Recorded as printed; not resolved, and none of these stopped the transcription.

1. **Two different index groups carry an identical label.** The right legend
   column prints, on consecutive lines,
   `⑯~⑱: Repeater tone frequency setting` and
   `⑲~㉑: Repeater tone frequency setting` — word for word the same label
   against two different three-byte groups. The `ⓘSee` line that follows them
   both reads `See "Repeater tone/tone squelch frequency setting." (p. 22)`,
   naming two settings where the two labels name one. Both labels are recorded
   verbatim, twice.
2. **The band and the legend group ⑪ and ⑫ differently.** In the D1 band `⑪`
   and `⑫` are printed as two separate circled numerals, each over its own byte
   cell, with no bracket joining them — unlike `①, ②` and `③, ④`, which are
   each drawn under a single bracket with a comma between the numerals. The
   legend, however, prints them joined: `⑪, ⑫: Operating mode setting`. The
   band's presentation has been followed for `field_index` (two rows, `11` and
   `12`), and the single legend line supplies the label for both.
3. **`⑬`, `⑭` and `⑮` are likewise printed bare in the band**, each a single
   circled numeral over one cell with no bracket, whereas every multi-byte
   group in the band is bracketed. Recorded as printed.
4. **One cell in the band is printed `X 0` rather than `X X`.** In the upper
   row, the cell beneath the circled `⑮` prints an `X` in its left nibble and a
   numeral `0` in its right nibble; the `⑮` detail inset (D4) prints the same
   `X 0` with the leader `Fixed` pointing at the `0`. Every other data cell in
   the band prints `X X`. Recorded here as an observation only; no encoding or
   meaning is inferred.
5. **Indices ⑤, ⑭ and ⑮ each appear twice on the page** — once in the D1 band
   and once above their own detail inset (D2, D3, D4). This is not a
   discontinuity within any one diagram, so it is not raised as a STOP; each
   occurrence is recorded on its own row, with the repetition noted in that
   row's `notes`.
6. **`⑯~⑱` is shaded and `⑲~㉑` is unshaded** in the band, and the shading of
   cells alternates irregularly across both rows. Shading is not part of this
   ledger and no meaning is inferred from it; it is noted only because it is
   the most conspicuous unrecorded feature of the diagram.
7. **A second, differently scoped list of the same indices appears lower on the
   page**, under `To clear the memory channel contents on 1A 00:`, giving
   `①, ②: Memory channel group (0000~0099)`, `③, ④: Memory channel
   (0000~0099)` and `⑤: "FF," ⑥ ~ : None`. These lines label the same indices
   with different text from the main legend, and `⑥ ~ :` is printed with no
   closing index at all. That list is a set of clearing instructions rather
   than a diagram legend, so it was not used as a source of any
   `label_verbatim`; it is recorded here because it disagrees with the main
   legend.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource
was opened, and no directory was listed.

For completeness, the only directory listings performed at all were `ls` and
`magick identify` over my own freshly created render and crop directories
inside `…/evidence/ic705-L`, to confirm that the renders had been written and
at what pixel dimensions. No directory outside that tree was listed, and no
file outside that tree was opened other than the PDF named above.
