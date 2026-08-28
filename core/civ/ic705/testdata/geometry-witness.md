# IC-705 CI-V reference guide — memory-record data-block geometry witness

## Source

Title as printed on the cover (PDF page 1): the black cover panel prints
`CI-V REFERENCE GUIDE`; below it, in the ruled title block, `HF/VHF/UHF ALL MODE
TRANSCEIVER` above the model name `IC-705`, and `Icom Inc.` at the foot.

Revision code as printed: `A7560-8EX-6`, printed at the lower left of PDF page 31
(the unnumbered back cover), immediately above the line `© 2020–2023  Icom Inc.
Jan. 2023`. No revision code is printed on the cover.

File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic705_civ_rev6.pdf`

Page count: 31 PDF pages.

## Extent

| PDF page | Printed folio | Rendered | Read | Contribution |
| --- | --- | --- | --- | --- |
| 1 | none printed (cover) | 200 dpi | yes | Cover title for `## Source`. |
| 17 | 16 | 300 dpi | yes | Context only: `◇ Command table`, a command table with no data-block diagram. |
| 18 | 17 | 300, 400 dpi | yes | Context only: `◇ Command formats` — Operating frequency, Operating mode, Band edge frequency settings, Duplex Offset frequency setting, Codes for CW message contents. No memory-record data block. |
| 19 | 18 | 300, 400, 600 dpi | yes — every recorded value comes from this page | The memory-record data-block diagram and its three nibble legends. |
| 20 | 19 | 300, 400 dpi | yes | Context only: `◇ Command formats` — Codes for character entries, Band stacking register. No memory-record data block. |
| 21 | 20 | 300 dpi | yes | Context only: Keyer memory character entries, Keyer memory content, IF filter width settings, AGC time constant settings, RX HPF/LPF, SSB passband, Split offset frequency setting. No memory-record data block. |
| 31 | none printed (back cover) | 200, 600 dpi | yes | Revision code for `## Source`. |

All transcribed material is on PDF page 19, printed folio 18, inside the section
whose grey banner heading reads `Remote control (CI-V) information` and whose
sub-heading reads `◇ Command formats`.

Immediately before the material: the bulleted caption `• Memory content` and,
beneath it, `Command: 1A 00`. The two-row data-block diagram sits directly under
that caption.

Immediately after the material: the two-column legend that begins
`①, ②: Memory group number` and ends, in the right-hand column, with the grey
`NOTE:` panel whose last bullet reads `Even if the Split function is OFF, enter
the data into ❻ ~ ❺❷ to match your transceiver. We recommend that you set the
same data as ⑥ ~ 52.` The page then ends with the folio `18`.

### Diagrams defined

- **D1** — printed caption, verbatim, on two lines: `• Memory content` /
  `Command: 1A 00`. One record drawn as two rows: the upper row runs left to
  right and ends in a wrap arrow that loops down and back to the left, where an
  arrowhead enters the start of the lower row. The two rows are one continuous
  record, and the byte numbering is continuous across the wrap.
- **D2** — printed caption, verbatim: `⑤: Split and Select memory setting`.
- **D3** — printed caption, verbatim: `⑭: Duplex and Tone settings`.
- **D4** — printed caption, verbatim: `⑮: Digital squelch setting`.

D2, D3 and D4 are each a single two-nibble box with leader lines, i.e. a nibble
legend for one byte of D1, not a record of their own. They are recorded because
each carries a printed index above the box; per the CSV convention their own
first (and only) byte is byte 1, and the notes column states which D1 byte each
one expands.

### What the diagram does and does not number

D1 numbers its byte positions. Every drawn cell is one byte, printed as two
characters separated by a dotted vertical rule (`X⋮X`), i.e. two nibbles; the
circled numerals above the cells are byte indices, either printed singly over one
cell, or as a pair or a range carried on a square bracket that spans a group of
cells. Those printed numerals are what I read off the render for byte positions,
except where noted.

The diagram does **not** label its nibbles. It divides every byte into two halves
with a dotted rule, and D2/D3/D4 attach leader lines to individual halves, but no
nibble anywhere carries a number, a letter or a name. Every field in D1 is
therefore recorded as a whole byte, nibble 1 to nibble 2.

Ranges longer than three bytes are drawn compressed: an `X⋮X` cell, then a
dash-outlined cell containing a short row of dots, then an `X⋮X` cell — three
drawn cells standing for the whole range. Where a range is three bytes or fewer
it is drawn as one cell per byte with no ellipsis cell. Cell fills alternate
white and grey between adjacent fields; the fill carries no printed meaning and
is used here only to describe where a cell sits.

### Index-glyph transcription convention

Indices are printed as numerals inside circles. Where Unicode has the matching
circled character I use it (`①`–`㊿`, and the black-circled `❻` = U+2776+5). Unicode
has no circled or black-circled form for 52, 53 or 68, so those are written as
bare digits in `field_index` and their printed style is recorded in `notes`. The
range separator is transcribed `~` for the short tilde and `〜` for the one
visibly longer wave (see `## Observed disagreements`); spacing around separators
is normalised away (see `## Method`, second pass).

## Method

1. **Locate.** `pdftoppm -png -r 300 -f 17 -l 21 <pdf> r300/p` into the fresh
   directory `…/evidence/ic705-W/r300/`. The five page images were read as images
   to find the section whose banner reads `Remote control (CI-V) information` and
   whose caption reads `• Memory content`. PDF page 19 is the only one of the five
   carrying a memory-record data block.
2. **Read.** `pdftoppm -png -r 400 -f 18 -l 20 <pdf> r400/p`. Every first-pass
   value was read from `r400/p-19.png` (3308×4678).
3. **Crop and enlarge.** ImageMagick 7 was available (`/opt/homebrew/bin/magick`)
   and used throughout. First-pass crops, all from `r400/p-19.png`:
   - `magick r400/p-19.png -crop 3308x460+0+1020 +repage -resize 150% crops/A_both_rows.png`
   - `magick r400/p-19.png -crop 970x180+{250,1200,2150}+1040 +repage -resize 300% crops/R1_seg{0,1,2}.png`
   - `magick r400/p-19.png -crop 1460x180+250+1040 +repage -resize 200% crops/R1_halfA.png`
   - `magick r400/p-19.png -crop 1460x180+1620+1040 +repage -resize 200% crops/R1_halfB.png`
   - `magick r400/p-19.png -crop 1500x200+300+1290 +repage -resize 200% crops/R2_halfA.png`
   - `magick r400/p-19.png -crop 1500x200+1700+1290 +repage -resize 200% crops/R2_halfB.png`
   - `magick r400/p-19.png -crop 560x190+1560+1290 +repage -resize 400% crops/R2_junction_52_black.png`
   - `magick r400/p-19.png -crop 420x110+1880+1290 +repage -resize 500% crops/R2_black_label.png`
   - `magick r400/p-19.png -crop 700x190+2200+1290 +repage -resize 400% crops/R2_5368_cells.png`
   - `magick r400/p-19.png -crop 900x90+900+1050 +repage -resize 400% crops/R1_waves.png`
   - `magick r400/p-19.png -crop 1150x90+1780+1050 +repage -resize 350% crops/R1_waves2.png`
   - `magick r400/p-19.png -crop 1000x90+950+1290 +repage -resize 400% crops/R2_waves.png`
   - `magick r400/p-19.png -crop 700x90+560+1290 +repage -resize 500% crops/R2_wave_2628.png`
   - nibble legends: `-crop 1150x520+200+2280`, `-crop 1200x620+200+3480`,
     `-crop 1200x560+200+4040`, each `-resize 200%` → `crops/sub5.png`,
     `crops/sub14.png`, `crops/sub15.png`.
   At 400 dpi enlarged 200–500%, every numeral, bracket vertex, cell rule and
   dotted nibble divider stood clear of its neighbours.
4. **`pdftotext`.** Not run at any point. Navigation was done entirely by reading
   the 300 dpi page images. For completeness: the only shell listings performed
   (`ls`, `magick identify`) were of my own render and crop directories beneath
   `…/evidence/ic705-W`, to confirm that the renders had been written and at what
   pixel dimensions. No other directory was listed and no other file was opened.
5. **`tesseract`.** Available at `/opt/homebrew/bin/tesseract` but **not used**.
   Every numeral was read by eye off the enlarged crops.
6. **Second independent pass.** After the first pass was complete, PDF page 19 was
   re-rendered at 600 dpi (`pdftoppm -png -r 600 -f 19 -l 19 <pdf> r600/p`,
   4961×7016) and re-read from a different raster with different crop windows and
   a different enlargement, into a separate directory `crops2/`:
   - `magick r600/p-19.png -crop 1560x300+{375,1900,3400}+1545 +repage -resize 160% crops2/P2_R1{a,b,c}.png`
   - `magick r600/p-19.png -crop 1300x330+{430,1700}+1920 +repage -resize 180% crops2/P2_R2{a,b}.png`
   - `magick r600/p-19.png -crop 1400x330+2950+1920 +repage -resize 180% crops2/P2_R2c.png`
   The second pass differed in dpi (600 vs 400), in window boundaries (thirds
   with boundaries at different cells, so that every boundary the first pass had
   split now fell mid-window) and in enlargement (160–180% vs 200–500%).

   **Result: the two passes agree on every recorded value** — every cell count,
   every index numeral, every numeral style, every bracket span, the wrap
   arrow, and the fact that the black-labelled band contains no `X⋮X` cell.
   Recorded disagreement, one only, and it concerns typography rather than a
   recorded value: the amount of white space printed between a range's numerals
   and its separator. In the first pass `⑯~⑱` looked tight and `⑲ ~ ㉑` spaced; at
   600 dpi `⑯ ~ ⑱` also looked spaced. Because the two passes could not be made to
   agree on inter-glyph spacing, spacing around separators is normalised away in
   `field_index` (written `⑯~⑱`, `⑲~㉑`, …) and the normalisation is declared
   here. This is a recording convention, not a claim about the print. Comma pairs
   (`①, ②`, `③, ④`) are printed with an unmistakable comma and space in both
   passes and are recorded as such. The separator *glyph shape* difference at
   `㉙〜㊱` was visible in both passes and is recorded, not normalised — see
   `## Observed disagreements`.

## Position arithmetic, per diagram

### D1 — `• Memory content` / `Command: 1A 00`

Positions are counted from D1's own first cell (the leftmost white `X⋮X` of the
upper row) = byte 1. "Drawn" is the number of cells actually inked; "extent" is
the number of byte positions the field occupies. Where drawn = extent I counted
cells directly. Where a run is drawn compressed (`X⋮X`, ellipsis cell, `X⋮X`) the
extent is read from the two printed numerals on that run's own bracket, because
the elided bytes are not drawn.

| # | printed index | starts at byte | drawn cells | extent (bytes) | ends at byte | next field starts at byte |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `①, ②` | 1 | 2 | 2 | 2 | 3 |
| 2 | `③, ④` | 3 | 2 | 2 | 4 | 5 |
| 3 | `⑤` | 5 | 1 | 1 | 5 | 6 |
| 4 | `⑥~⑩` | 6 | 3 (compressed) | 5 | 10 | 11 |
| 5 | `⑪` | 11 | 1 | 1 | 11 | 12 |
| 6 | `⑫` | 12 | 1 | 1 | 12 | 13 |
| 7 | `⑬` | 13 | 1 | 1 | 13 | 14 |
| 8 | `⑭` | 14 | 1 | 1 | 14 | 15 |
| 9 | `⑮` | 15 | 1 | 1 | 15 | 16 |
| 10 | `⑯~⑱` | 16 | 3 | 3 | 18 | 19 |
| 11 | `⑲~㉑` | 19 | 3 | 3 | 21 | 22 |
| 12 | `㉒~㉔` | 22 | 3 | 3 | 24 | 25 (via the wrap arrow, on the lower row) |
| 13 | `㉕` | 25 | 1 | 1 | 25 | 26 |
| 14 | `㉖~㉘` | 26 | 3 | 3 | 28 | 29 |
| 15 | `㉙〜㊱` | 29 | 3 (compressed) | 8 | 36 | 37 |
| 16 | `㊲~㊹` | 37 | 3 (compressed) | 8 | 44 | 45 |
| 17 | `㊺~52` | 45 | 3 (compressed) | 8 | 52 | 53 |
| 18 | `❻~52` (black-filled circles) | 53 | 1 band, no `X⋮X` cell | 47 | 99 | 100 |
| 19 | `53~68` | 100 | 3 (compressed) | 16 | 115 | — end of diagram |

Row check. Upper row: 2+2+1+3+1+1+1+1+1+3+3+3 = 22 drawn cells, covering byte
positions 1–24 (the `⑥~⑩` run's 3 drawn cells stand for 5 bytes, so 22 − 3 + 5 =
24). Counted independently on both passes: 22 cells inked in the upper row, the
last of them immediately left of the wrap arrow.

Lower row: 1+3+3+3+3 = 13 drawn cells before the black-labelled band, then the
band itself (one dash-outlined grey region, no internal rules, roughly three
cell-widths wide), then 3 drawn cells for `53~68` — 17 drawn regions in all.

Running total. 24 (upper row) + 1 (`㉕`) + 3 (`㉖~㉘`) + 8 + 8 + 8 = 52 at the end of
`㊺~52`. Adding the black-labelled band's 47 positions gives 99. Adding `53~68`'s
16 gives **115 byte positions in the whole record**. Nothing printed anywhere on
the page states a total, so there is nothing for 115 to contradict; it is the sum
of the measured parts, given here so the sums can be checked without the images.

Two places where the running position and the printed numbering disagree, both
recorded as STOPs and neither resolved:

- Field 18: running position 53–99, printed index `❻~52`.
- Field 19: running position 100–115, printed index `53~68`.

Both occurrences of the apparently repeated block were measured separately.
The first occurrence is fields 4–17 above, printed `⑥~⑩` … `㊺~52` in outlined
circles, drawn as 30 cells (3+1+1+1+1+1+3+3+3+1+3+3+3+3) and measured at bytes
6–52, i.e. 5+1+1+1+1+1+3+3+3+1+3+8+8+8 = 47 byte positions.
The second occurrence is field 18 alone: one undivided band carrying the
black-filled indices `❻` and `52`, drawing no cells at all, measured at bytes
53–99, i.e. 47 byte positions. That the two extents come to the same number is an
observation made after measuring each on its own; nothing about the second was
assumed from the first, and the second's extent rests on its own printed end
numerals (STOP 2), not on the first's cell count.

### D2 — `⑤: Split and Select memory setting`

One box, two nibbles, no elision. Starts at its own byte 1 nibble 1; extent 1
byte; ends at byte 1 nibble 2; nothing follows. Printed index `⑤`, one index for
the whole box. The two nibbles are separately leadered but neither is numbered.
The same field is byte 5 of D1.

### D3 — `⑭: Duplex and Tone settings`

One box, two nibbles, no elision. Starts at its own byte 1 nibble 1; extent 1
byte; ends at byte 1 nibble 2; nothing follows. Printed index `⑭`. The same field
is byte 14 of D1.

### D4 — `⑮: Digital squelch setting`

One box, two nibbles, no elision; the second nibble prints the numeral `0` rather
than `X`. Starts at its own byte 1 nibble 1; extent 1 byte; ends at byte 1 nibble
2; nothing follows. Printed index `⑮`. The same field is byte 15 of D1, whose
cell in D1 likewise prints `X 0`.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** D1 draws
  seventeen of its nineteen index labels as outlined circles (`①` … `㊺`, `52`,
  `53`, `68`), and one label — the eighteenth field — as two BLACK-FILLED circles
  with the digits reversed out in white, reading `6` and `52`. The styles are
  recorded as drawn in every row's `notes` and are not normalised to one another;
  no meaning is inferred for the filled style here. (The right-hand legend column
  on the same page also prints both styles, but that is prose, not the diagram.)
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED on the transcribed
  diagram.** Every label on D1, D2, D3 and D4 is set horizontally; none is
  rotated. (Rotated labels do occur in this manual — the Operating frequency and
  Band edge diagrams on PDF page 18, and the Split offset diagram on PDF page 21,
  set their leader labels vertically — but none of those is a memory-record data
  block and none was transcribed.) No text layer was consulted at any point, so
  extraction order could not have influenced anything recorded.
- **(c) Leader-line label order reversed — ENCOUNTERED.** In D2 and D3 the two
  label groups sit to the right and below the box, and their vertical order runs
  opposite to the nibbles they point at: the UPPER label group belongs to the
  RIGHT (second) nibble, and the LOWER label group belongs to the LEFT (first)
  nibble. Each leader was followed by eye from label to the arrowhead that lands
  on the nibble. In D2 the left nibble takes `0=Split OFF,1=Split ON` (printed
  lowest) and the right nibble takes `0=OFF*` / `1= ★1` / `2= ★2` / `3= ★3`
  (printed highest). In D3 the left nibble takes `0=Duplex OFF` / `1=Duplex-` /
  `2=Duplex+` and the right nibble takes `0=OFF` / `1=TONE` / `2=TSQL` / `3=DTCS`,
  which is printed higher on the page. D4 is the same pattern in miniature:
  `Fixed`, which belongs to the right nibble, is printed above the three lines
  that belong to the left nibble. No label group was assigned by its reading
  order.
- **(d) A printed index differs from a field's measured position — ENCOUNTERED.**
  D1's eighteenth field prints `❻~52` where the running count gives bytes 53–99,
  and its nineteenth prints `53~68` where the running count gives bytes 100–115.
  Both the printed index and the measured position are recorded for every field
  and neither has been reconciled to the other; no printed index was adjusted to
  fit a measured position, and no measured position was adjusted to fit a printed
  index. See STOP 1, STOP 2 and STOP 3.

## STOP findings

1. **Repeated index range, in a different numeral style.** PDF page 19, D1 lower
   row, the wide grey dash-outlined band between the last `X⋮X` cell of `㊺~52`
   and the first `X⋮X` cell of `53~68`; the bracket above it is labelled with two
   black-filled circles. What is printed: `❻ ~ ❺❷` — the digits `6` and `52`
   reversed out of solid black circles. Indices 6 to 52 have already been printed
   once in this same diagram, in outlined circles, over cells earlier in the same
   record (`⑥~⑩` … `㊺~52`). Why it stops: a repeat in the index sequence, the
   repeat distinguished from the original only by numeral styling, whose meaning
   I am not told and do not infer. Transcribed into the CSV as seen:
   `field_index` = `❻~52`, with the black-filled styling of both numerals stated
   in `notes`.
2. **A field whose extent cannot be counted from cells.** Same anchor as STOP 1.
   What is printed: one dash-outlined grey band, about three cell-widths wide,
   containing a single long row of dots and no internal vertical rules — no
   `X⋮X` cell at all, not even a first or last one, unlike every other compressed
   run on this diagram, each of which draws its first and last byte. Why it
   stops: the band's extent therefore cannot be obtained by counting cells; the
   only measurement available on the render is the pair of numerals on its own
   bracket, `6` and `52`, which span 47 byte positions. That number is recorded
   as measured, with this limitation stated, rather than silently counted.
   Transcribed into the CSV as seen: `first_byte` 53, `last_byte` 99, with the
   basis stated in `notes`.
3. **Running position and printed numbering disagree.** PDF page 19, D1 lower
   row, the last three drawn cells of the diagram, under the bracket labelled
   `53~68`, immediately right of the black-labelled band. What is printed: `53`
   and `68`, both in outlined circles. Counting from D1's first cell, this run
   begins at byte position 100 and ends at byte position 115, because the
   black-labelled band ahead of it occupies 47 positions. Why it stops: the
   printed numbering resumes at 53, i.e. as though the black-labelled band
   occupied no byte positions at all, while the measured running total says
   otherwise. Both are recorded: `field_index` = `53~68` as printed, `first_byte`
   100 and `last_byte` 115 as measured. Neither is resolved, and neither has been
   adjusted towards the other.

## Observed disagreements

- **Two different wave-dash glyphs are used as the range separator inside one
  diagram.** Every range label on D1 uses a short, tight tilde — `⑥~⑩`, `⑯~⑱`,
  `⑲~㉑`, `㉒~㉔`, `㉖~㉘`, `㊲~㊹`, `㊺~52`, `❻~52`, `53~68` — except
  `㉙〜㊱`, whose separator is a visibly longer, lower, wider wave. Both passes,
  at 400 dpi and at 600 dpi, showed the same difference, and the crops
  `crops/R2_wave_2628.png` and `crops2/P2_R2a.png` place `㉖ ~ ㉘` and `㉙ 〜 ㊱`
  side by side for comparison. Transcribed as seen (`~` and `〜`); not resolved,
  and no meaning inferred.
- **One cell of D1 prints a literal digit where every other cell prints `X`.**
  Cell 15 reads `X 0`, not `X X`; its dotted nibble divider is drawn as usual.
  D4 labels that second nibble `Fixed`. Recorded in the notes for that row; the
  index `⑮` sits over the whole cell, so the field is recorded as byte 15 nibble
  1 to byte 15 nibble 2.
- **The two rows of D1 are drawn with the second row starting further from the
  left margin than the first**, the space being taken by the wrap arrow's
  arrowhead. The arrowhead is not a cell and was not counted; the first cell of
  the lower row is the one marked `㉕`.
- **Cell fills alternate white and grey between adjacent fields but not
  consistently with the index groups**: `⑤` is white, `⑥~⑩` grey, `⑪` white,
  `⑫` grey, `⑬` white, `⑭` grey, `⑮` white, and so on. The fill appears to be a
  visual separator only. Nothing on the page states what the fill means, and
  nothing was inferred from it; it is used in `visual_anchor` purely to say where
  a cell sits.
- **`㊺~52`, `53~68` and the black `52` are printed in circles for which Unicode
  has no character.** This is a limitation of the recording, not of the print:
  the page prints all three inside circles, exactly as it does `①`–`㊿`. See the
  transcription convention in `## Extent`.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.
