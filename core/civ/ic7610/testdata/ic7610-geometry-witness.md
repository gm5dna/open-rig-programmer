# IC-7610 CI-V — memory-record data-block geometry witness

## Source

- Document title, as printed on the cover (PDF page 1, read from the render):
  black panel with the Icom logo and, beneath it, `CI-V REFERENCE GUIDE`; lower
  on the same page, `HF/50MHz TRANSCEIVER` above `IC-7610`, and `Icom Inc.` at
  the foot.
- Revision code, as printed: `A7380-7EX-4`. It is printed at the foot of the
  left-hand column of the last page (PDF page 17), immediately above
  `© 2017–2025 Icom Inc.    Sep. 2025`. No revision code is printed on the
  cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7610_civ_ENG_4.pdf`
- Page count: 17 PDF pages. Confirmed by rendering: page 17 renders, and a
  render of page 18 is refused with `Wrong page range given: the first page
  (18) can not be after the last page (17)`.

## Extent

Pages rendered and read (printed folio taken from the page foot on each
render):

| PDF page | printed folio | rendered at | what it contributed |
|---|---|---|---|
| 1 | (none printed) | 200 dpi | cover title, model, publisher |
| 11 | 10 | 400 dpi | what precedes the transcribed material |
| 12 | 11 | 400 dpi, 600 dpi | **all transcribed values** |
| 13 | 12 | 400 dpi | what follows the transcribed material |
| 17 | (none printed) | 300 dpi | revision code, copyright line |

An earlier orientation render of PDF pages 10–14 at 300 dpi was also made and
page 12 of it was viewed; every value recorded was afterwards read from the
400 dpi and 600 dpi renders described under `## Method`.

Where the transcribed material begins and ends on PDF page 12 (printed folio
11). The page's running head is `Remote control`; below it the section marker
`◇ Command formats`. The transcribed material is the sub-section headed
`• Memory content` / `Command: 1A 00`, which is the first sub-section on the
page — immediately above it there is nothing but the `◇ Command formats`
marker, and the material immediately before it in the document is the last
sub-section of PDF page 11 (folio 10), `• Band stacking register` /
`Command: 1A 01`, whose closing sentence is `For example, when
sending/reading the oldest contents in the 21 MHz band, the code "07 03" is
used.`

The transcribed material ends with the right-hand column's
`To clear the memory channel contents on 1A 00:` list, whose last line is
`④:      None`. Immediately after it on the same page, in the left-hand
column, comes the next sub-section `• Codes for character entries` /
`Command: 1A 00,` with its two character-code tables; that sub-section and its
tables are not part of the memory-record data-block diagrams and were not
transcribed. The next PDF page, 13 (folio 12), opens with
`• Data mode with filter width settings` / `Command: 1A 06` and carries no
memory-record data-block diagram.

Three diagrams on PDF page 12 belong to the memory record and are transcribed:

- **D1** — the byte strip printed directly under the heading printed verbatim
  as `• Memory content` with `Command: 1A 00` on the line below it. The strip
  itself carries no caption of its own; it is defined here by that heading.
- **D2** — the enlarged one-byte box under the heading printed verbatim as
  `③ Select memory setting` (left column).
- **D3** — the enlarged one-byte box under the heading printed verbatim as
  `⑪ Data mode and tone type settings` (right column).

D2 and D3 are enlargements of single bytes of the D1 record. They are numbered
diagrams of the memory record printed on this page, so they are recorded; each
is measured in its own right, from its own first cell, and the divergence
between its printed index and its measured position is recorded, not resolved
(see STOP 4 and STOP 5).

No other numbered data-block diagram appears on PDF page 12. The rest of the
page is prose, the two character-code tables and the `Cmd. / Sub cmd. / Set
item selectable characters` table.

## Method

- **Locate**, 300 dpi: `pdftoppm -png -r 300 -f 10 -l 14 <pdf> <out>/r300/p`,
  and the renders read as images to find the section headed `• Memory
  content`. Adjacent sections (`• Band stacking register` on the previous
  page, `• Data mode with filter width settings` on the next) were checked by
  eye and carry no memory-record diagram.
- **Read, pass 1**, 400 dpi: `pdftoppm -png -r 400 -f 11 -l 13 <pdf>
  <out>/pass1/p400`. Crops, all with ImageMagick 7 (`magick`), which is
  available and was used:
  - `magick pass1/p400-12.png -crop 2560x260+580+840 +repage -resize 200%` — whole strip
  - `magick pass1/p400-12.png -crop 900x260+580+840 +repage -resize 400%` — strip, left third
  - `magick pass1/p400-12.png -crop 900x260+1400+840 +repage -resize 400%` — strip, middle third
  - `magick pass1/p400-12.png -crop 900x260+2180+840 +repage -resize 400%` — strip, right third
  - `magick pass1/p400-12.png -crop 900x480+330+1430 +repage -resize 250%` — D2
  - `magick pass1/p400-12.png -crop 1280x380+1700+1150 +repage -resize 250%` — D3
  - `magick pass1/last-17.png -crop 2481x700+0+2808 +repage` — revision code (300 dpi render)
- **Read, pass 2 (second independent pass, required)**, 600 dpi:
  `pdftoppm -png -r 600 -f 12 -l 12 <pdf> <out>/pass2/p600`. Different raster
  (600 dpi rather than 400 dpi), different crop windows (four 1060 px windows
  starting at x = 870, 1870, 2870, 3870, cut at boundaries that fall in
  different places from pass 1's three windows) and a different enlargement
  (300% rather than 200%/400%):
  `magick pass2/p600-12.png -crop 1060x400+<x>+1260 +repage -resize 300%`.
  Pass 2 was carried out by re-counting cells and re-reading every circled
  numeral and every brace terminus on those four windows.
- **Pass 2 also included a mechanical column profile of the 600 dpi raster**
  (`PIL`, reading the same PNG), used as a cross-check on the cell count and
  cell boundaries, not as a substitute for reading: for every image column
  between x = 700 and x = 4900 the fraction of dark pixels between the strip's
  top rule (y = 1414–1417) and bottom rule (y = 1532–1535) was measured. It
  returned exactly 19 full-height vertical rules, at
  x = 966, 1132, 1297, 1462, 1628, 1793, 1958, 2124, 2289, 2455, 2620, 2785,
  2951, 3116, 3281, 3447, 3612, 3777, 3941 — that is, 18 cells of equal width
  (164–166 px). The same profile shows a half-height dotted divider at the
  midpoint of every cell except cells 5 and 17, which have none: those two are
  the dashed `...` abbreviation cells. Brace termini were located the same way
  (dark clusters in the band just above the strip, y = 1390–1400), landing at
  x ≈ 971, 1289, 1466, 1958–1966, 2281, 2459, 2944–2956, 3438–3451, 3934 —
  each within a few pixels of a cell rule listed above. Every one of these
  mechanical results was confirmed by eye on the enlarged crops before being
  relied on.
- **Two-pass result:** both passes were done. **No cell disagreed between the
  two passes.** Cell count (18), which cells are `...` cells (5 and 17), the
  shading of every cell, every circled numeral (`①`, `②`, `③`, `④`, `⑧`,
  `⑨`, `⑩`, `⑪`, `⑫`, `⑭`, `⑮`, `⑰`, `⑱`, `㉗`), and the cell rule on
  which each brace terminates were identical in pass 1 and pass 2. No third
  render was needed.
- **tesseract** is installed and available. It was **not** used: every numeral
  and label recorded here was read by eye from the enlarged crops, which are
  clean vector renders with the glyphs well clear of their neighbours.
- **`pdftotext` was NOT run at all**, in any form, on this or any file.
  Navigation was done entirely by reading page renders.
- Housekeeping note, for a second reader retracing this: the first crop
  directory written during pass 1 was no longer present on the next tool call
  and was re-created; all crops cited above were re-made and exist under
  `pass1/` and `pass2/`. A directory `r300/` beside them was later found to
  contain renders of all 17 pages, which this leg did not create and did not
  read; only `r300/p-12.png`, produced by the 300 dpi locate step above, was
  viewed, and only for orientation. The only directories listed at any point
  were the render and crop directories this leg itself created inside
  `.../scratchpad/evidence/ic7610`, in order to confirm that the renders had
  been written; no directory outside that path was listed, opened or searched.

## Position arithmetic, per diagram

### D1 — `• Memory content`, `Command: 1A 00`

D1 numbers its own byte positions: a circled numeral (or a circled-numeral
range under a brace) is printed above every cell group, running `①` to `㉗`
without gap, repeat or reordering. The `first_byte`/`last_byte` recorded in
the CSV are those printed numerals, read off the render. The strip is drawn in
18 equal cells, of which 16 are `X:X` byte cells and 2 are dashed `...`
abbreviation cells; the drawn-cell count is given below beside the printed
numbering so both can be checked.

Running position by the **printed numbering** (bytes):

| field, as printed | starts at byte | printed extent | ends at byte | next field starts at |
|---|---|---|---|---|
| `①, ②` | 1 | 2 | 2 | 3 |
| `③` | 3 | 1 | 3 | 4 |
| `④ ~ ⑧` | 4 | 5 | 8 | 9 |
| `⑨, ⑩` | 9 | 2 | 10 | 11 |
| `⑪` | 11 | 1 | 11 | 12 |
| `⑫ ~ ⑭` | 12 | 3 | 14 | 15 |
| `⑮ ~ ⑰` | 15 | 3 | 17 | 18 |
| `⑱ ~ ㉗` | 18 | 10 | 27 | — (end of strip) |

2 + 1 + 5 + 2 + 1 + 3 + 3 + 10 = 27, and the last printed index is `㉗` = 27.
By the printed numbering there is no gap and no overlap.

Running position by **counting the drawn cells** from the strip's own first
cell:

| field, as printed | starts at drawn cell | measured extent (cells) | ends at drawn cell | next field starts at | contains a `...` cell |
|---|---|---|---|---|---|
| `①, ②` | 1 | 2 | 2 | 3 | no |
| `③` | 3 | 1 | 3 | 4 | no |
| `④ ~ ⑧` | 4 | 3 | 6 | 7 | yes — drawn cell 5 |
| `⑨, ⑩` | 7 | 2 | 8 | 9 | no |
| `⑪` | 9 | 1 | 9 | 10 | no |
| `⑫ ~ ⑭` | 10 | 3 | 12 | 13 | no |
| `⑮ ~ ⑰` | 13 | 3 | 15 | 16 | no |
| `⑱ ~ ㉗` | 16 | 3 | 18 | — (end of strip) | yes — drawn cell 17 |

2 + 1 + 3 + 2 + 1 + 3 + 3 + 3 = 18, and the strip is drawn in 18 cells. By
the drawn-cell count there is likewise no gap and no overlap.

Where the two disagree:

- `④ ~ ⑧`: printed extent 5, measured drawn extent 3 → **STOP 1**.
- `⑱ ~ ㉗`: printed extent 10, measured drawn extent 3 → **STOP 2**.
- Totals: printed 27, drawn 18; from `⑨` onward every printed index sits two
  cells to the right of its drawn-cell position, and after drawn cell 17 the
  offset is nine → **STOP 3**.
- `①, ②`, `③`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭`, `⑮ ~ ⑰`: printed extent equals
  measured drawn extent (2, 1, 2, 1, 3, 3 respectively).

Nibbles. Every `X:X` cell in D1 is divided at its midpoint by a dotted
vertical rule into two halves, each printed with a single `X`. The two halves
carry **no** printed nibble label, number or name anywhere in D1. Every D1
field therefore begins at nibble 1 of its first byte and ends at nibble 2 of
its last byte; that is what the CSV records. The two dashed `...` cells carry
no dotted divider and no `X`.

Repeated blocks, measured separately as required. `⑫ ~ ⑭` (`Repeater tone
frequency setting`) and `⑮ ~ ⑰` (`Tone squelch frequency setting`) are two
3-byte blocks of identical drawn shape; each was counted separately on the
render and each was found to occupy 3 drawn cells — `⑫ ~ ⑭` at drawn cells
10–12, `⑮ ~ ⑰` at drawn cells 13–15. Neither measurement was taken from the
other. Likewise, byte `③` was measured twice — once as drawn cell 3 of D1,
once as the single cell of D2 — and byte `⑪` twice — once as drawn cell 9 of
D1, once as the single cell of D3.

### D2 — `③ Select memory setting`

D2 is one box, one cell wide, drawn with a dotted vertical rule at its
midpoint. Its own first cell is its only cell.

| field, as printed | starts at | measured extent | ends at | next field starts at |
|---|---|---|---|---|
| `③` | byte 1, nibble 1 | 1 byte (2 nibbles) | byte 1, nibble 2 | — (end of diagram) |

1 = 1 drawn cell; no gap, no overlap. The printed index above the box is `③`,
which is 3, while the cell measured is cell 1 of this diagram's own block →
**STOP 4**.

Nibbles: the diagram does not number its nibbles. It labels them by leader
arrow instead: the left half prints `0` and its arrow leads down to the word
`Fixed`; the right half prints `X` and its arrow leads to the bracketed list
`0=OFF` / `1= ★1` / `2= ★2` / `3= ★3` printed to the right. Beneath the
diagram is printed `Set 0 for P1 and P2.` preceded by the encircled-i note
mark.

### D3 — `⑪ Data mode and tone type settings`

D3 is one box, one cell wide, drawn with a dotted vertical rule at its
midpoint. Its own first cell is its only cell.

| field, as printed | starts at | measured extent | ends at | next field starts at |
|---|---|---|---|---|
| `⑪` | byte 1, nibble 1 | 1 byte (2 nibbles) | byte 1, nibble 2 | — (end of diagram) |

1 = 1 drawn cell; no gap, no overlap. The printed index above the box is `⑪`,
which is 11, while the cell measured is cell 1 of this diagram's own block →
**STOP 5**.

Nibbles: the diagram does not number its nibbles. Both halves print `X`, and
each carries an upward leader arrow. Followed by eye, the arrows cross: the
arrow from the **right** half turns right and runs into the **upper** printed
label `0: OFF, 1: TONE, 2: TSQL`, and the arrow from the **left** half runs
below it, further right, into the **lower** printed label `0: OFF, 1: DATA 1,
2: DATA 2, 3: DATA 3`.

## Hazards encountered

- (a) Numeral styling may vary within one diagram — **NOT ENCOUNTERED**. Every
  index numeral in D1, D2 and D3 is drawn in one and the same style: a plain
  numeral inside a thin open circle, black on white, not filled, not reversed,
  not bracketed, not bold. `①` through `㉗` in D1, `③` in D2 and `⑪` in D3
  are all drawn that way; the two-digit ones (`⑩` … `㉗`) are the same open
  circle with two digits inside it. No second style appears anywhere in the
  three diagrams.
- (b) Diagrams may be vector groups with rotated labels — **NOT ENCOUNTERED**.
  Every label, numeral and legend in D1, D2 and D3 is printed horizontally;
  no rotated or vertical label appears in any of the three. (Rotated,
  bottom-to-top labels do appear on the adjacent PDF page 11 diagrams, but
  those are not memory-record diagrams and were not transcribed.) The text
  layer was not consulted at all — `pdftotext` was never run — so every
  position here was read from the picture regardless.
- (c) Leader-line label order may be reversed — **ENCOUNTERED**, in D3. The
  two leader arrows out of the `⑪` box cross each other: the arrow from the
  right-hand nibble lands on the label printed **above**
  (`0: OFF, 1: TONE, 2: TSQL`), and the arrow from the left-hand nibble lands
  on the label printed **below** (`0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`).
  Reading the two labels in printed order and assigning them to the nibbles in
  printed order would swap them. Each arrow was followed by eye from the
  nibble to the label at 250% enlargement of a 400 dpi render and again on the
  600 dpi raster. In D2 the arrows do not cross (left nibble → `Fixed` printed
  below-left, right nibble → the bracketed list printed to the right).
- (d) A printed index may differ from a field's measured position —
  **ENCOUNTERED**, in three places, all recorded and none reconciled.
  (i) In D1, because drawn cells 5 and 17 are dashed `...` abbreviation cells,
  the printed index of every field from `⑨` onward is larger than that
  field's drawn-cell position — `⑨` is drawn cell 7, `⑪` is drawn cell 9,
  `⑫ ~ ⑭` are drawn cells 10–12, `⑮ ~ ⑰` drawn cells 13–15, `⑱ ~ ㉗` drawn
  cells 16–18 (STOP 1, STOP 2, STOP 3). (ii) In D2 the printed index is `③`
  while the measured cell is cell 1 of D2's own block (STOP 4). (iii) In D3
  the printed index is `⑪` while the measured cell is cell 1 of D3's own
  block (STOP 5). No printed index was altered to fit a measured position and
  no measured position was altered to fit a printed index.

## STOP findings

1. **PDF page 12** — D1, the byte strip under `• Memory content` /
   `Command: 1A 00`; the group under the brace labelled `④ ~ ⑧`, which begins
   at the shaded `X:X` cell immediately right of the unshaded `③` cell and
   ends at the V vertex above the rule before the `⑨` cell. What is printed:
   a brace spanning **three** drawn cells — a shaded `X:X` cell, a dashed
   `...` cell with no nibble divider, a shaded `X:X` cell — labelled with the
   index range `④ ~ ⑧`, which spans **five** printed indices. Why it stops:
   the measured extent (3 cells) does not equal the printed extent (5 byte
   positions). Both are recorded; neither is resolved. CSV row `D1,④ ~ ⑧`
   carries the printed byte positions 4 and 8 and states the measured
   drawn-cell extent in `notes`.
2. **PDF page 12** — D1; the rightmost group, under the brace labelled
   `⑱ ~ ㉗`, running from the V after the `⑮ ~ ⑰` block to the right-hand end
   of the strip. What is printed: a brace spanning **three** drawn cells — a
   shaded `X:X` cell, a dashed `...` cell with no nibble divider, a shaded
   `X:X` cell — labelled with the index range `⑱ ~ ㉗`, which spans **ten**
   printed indices. Why it stops: measured extent (3 cells) does not equal
   printed extent (10 byte positions). Both recorded; neither resolved. CSV
   row `D1,⑱ ~ ㉗` carries the printed byte positions 18 and 27 and states the
   measured drawn-cell extent in `notes`.
3. **PDF page 12** — D1, the strip as a whole, from its left-hand end rule to
   its right-hand end rule. What is printed: a strip of **18** equal cells
   (16 `X:X` byte cells and 2 dashed `...` cells), numbered above by circled
   indices whose last is `㉗`, i.e. **27**. Why it stops: the total does not
   match its parts — counting the drawn cells gives 18, the printed numbering
   gives 27, and the two run apart by 2 from `⑨` onward and by 9 after the
   second `...` cell. Both totals are recorded above under `## Position
   arithmetic, per diagram`; neither is resolved. Rows `D1,⑨, ⑩` through
   `D1,⑱ ~ ㉗` carry `STOP 3` in `notes`.
4. **PDF page 12** — D2, the enlarged one-byte box under the heading
   `③ Select memory setting`, left column; the box printed `0` · dotted rule ·
   `X`, with a circled `③` centred above it. What is printed: the index `③`
   above a diagram that is one cell wide. Why it stops: the printed index is 3
   while the cell measured, counted from this diagram's own first cell, is
   cell 1. Both are recorded — `field_index` is `③` as printed, `first_byte`
   and `last_byte` are the measured 1 — and neither is reinterpreted in the
   light of the other.
5. **PDF page 12** — D3, the enlarged one-byte box under the heading
   `⑪ Data mode and tone type settings`, right column; the box printed `X` ·
   dotted rule · `X`, with a circled `⑪` centred above it. What is printed:
   the index `⑪` above a diagram that is one cell wide. Why it stops: the
   printed index is 11 while the cell measured, counted from this diagram's
   own first cell, is cell 1. Both are recorded; neither is reinterpreted.

## Observed disagreements

- The shading of D1's cells does not consistently mark the boundary between
  one indexed group and the next. Reading left to right the cells are shaded
  grey, grey, white, grey, grey (`...`), grey, white, white, grey, white,
  white, white, grey, grey, grey, grey, grey (`...`), grey. That separates
  `①, ②` from `③`, `③` from `④ ~ ⑧`, and so on down to `⑫ ~ ⑭` from
  `⑮ ~ ⑰` — but `⑮ ~ ⑰` and `⑱ ~ ㉗` are both shaded grey, with no change of
  shading at the rule between drawn cells 15 and 16 where the one ends and the
  other begins. Only the brace V vertex marks that boundary. Recorded as seen;
  the cell rules and brace termini, which were measured, are unambiguous
  there.
- The prose beside D1 does not name a field for every index group in the same
  words the diagram uses. `①, ②` is glossed `Memory channel numbers` with the
  values `00 01 ~ 00 99:   Memory channel 01 ~ 99`, `01 00:   Programmed scan
  edge P1`, `01 01:   Programmed scan edge P2`; `⑪` is glossed `Data mode and
  tone type settings`; `③` is glossed `Select memory setting` and the note
  `Set 0 for P1 and P2.`; `④ ~ ⑧` is glossed `Operating frequency setting`
  with a cross-reference `See "• Operating frequency."`; `⑨, ⑩` is glossed
  `Operating mode setting`, `See "• Operating mode."`; `⑫ ~ ⑭` and `⑮ ~ ⑰`
  share one cross-reference, `See "• Repeater tone/tone squelch settings."`;
  `⑱ ~ ㉗` is glossed `Memory name settings`, `Up to 10 characters.`,
  `See "• Codes for character entries."`. None of these is printed as a
  heading over a block of the diagram, so `block_label_verbatim` is `-`
  throughout.
- The right-hand column prints, under `To clear the memory channel contents on
  1A 00:`, the indices `①, ②`, `③` and `④` with values
  `Memory channel (00 01~00 99)`, `"FF"` and `None`. This re-uses index `④`
  for a shorter command form in which only four indices exist, whereas in D1
  `④` is the first byte of the five-byte group `④ ~ ⑧`. Recorded as printed;
  it is prose beside the diagram, not a numbered field of a data-block
  diagram, so it is not a CSV row.
- The `~` between the two ends of an index range is printed as a swung dash
  (`~`) throughout, including in `00 01 ~ 00 99` in the prose, and is
  transcribed as such.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource
was opened, and no directory was listed.
