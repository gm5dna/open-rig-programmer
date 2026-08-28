# IC-905 memory-record data-block field ledger (leg L)

## Source

Document title as printed on the cover (PDF page 1): `CI-V REFERENCE GUIDE`, above
`ALL MODE TRANSCEIVER` and `IC-905`, with `Icom Inc.` at the foot.

Revision code as printed: `A7711-9EX-2`, printed at the bottom-left of the back cover
(PDF page 31), directly above the line `© 2023–2024  Icom Inc.      May 2024`.

File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic905_civ_2.pdf`

Page count: 31 PDF pages.

## Extent

Rendered: PDF pages 17–21 at 300 dpi (survey), 18–20 at 400 dpi (reading), 19 at 600 dpi
(second pass), 1 at 200 dpi and 31 at 200 and 500 dpi (cover / revision code only).

Read for transcription: **PDF page 19 only**, printed folio `18`.

| PDF page | printed folio | contribution |
|---|---|---|
| 18 | 17 | Boundary check only. Ends with `• Codes for CW message contents` and its ASCII-code table; the last thing printed before the transcribed material. No memory-record data block. |
| 19 | 18 | All transcribed material: `• Memory content`, `Command: 1A 00`, the wrapped two-row data-block diagram, its numbered legend, and three one-byte detail boxes. |
| 20 | 19 | Boundary check only. Begins with `• Codes for character entries` (left column) and `• Band stacking register` (right column); the first thing printed after the transcribed material. Its own numbered diagram (`①`, `②` over four `X` cells) belongs to `Band stacking register`, command `1A 01`, not to a memory record, so it is out of scope. |
| 17, 21 | — | Rendered in the 300 dpi survey to confirm the section boundaries; not read for values. |

The transcribed material begins immediately below the section rule `Remote control (CI-V)
information` / `◇ Command formats` on PDF page 19, at the bold caption `• Memory content`,
and ends with the line `⑤: “FF,” ⑥ ~ : None` at the foot of the right-hand column of the
same page. Nothing of it continues onto page 20.

Diagrams indexed (in page order; page reads left column then right column):

- **D1** — the wrapped two-row data block under the printed caption `• Memory content`
  / `Command: 1A 00`, spanning the full page width below that caption. 18 numbered fields.
- **D2** — the one-byte box (`0 : X`) in the left column, under the printed heading line
  `⑤: Split and Select memory setting`. No caption of its own. 1 numbered field.
- **D3** — the one-byte box (`X : X`) in the left column, under the printed heading line
  `⑭: Duplex and Tone settings`. No caption of its own. 1 numbered field.
- **D4** — the one-byte box (`X : 0`) at the top of the right column, under the printed
  heading line `⑮: Digital squelch setting`. No caption of its own. 1 numbered field.

D2–D4 are included because each is a boxed data block carrying a printed numbered index of
its own, and because the task requires every numbered field of every such block, including
blocks that appear to duplicate another. Each of them repeats an index already drawn in
the D1 index band; that repetition is recorded, not tidied (STOP 3).

Label sourcing, stated plainly so a second reader can reproduce it: the D1 index band
prints indices only, with no text against them. For D1 the `label_verbatim` is the text of
the legend entry printed below the diagram that opens with the identical printed index
(e.g. `㉙~㊱: UR (Destination) call sign setting (8 characters, fixed)` → label
`UR (Destination) call sign setting (8 characters, fixed)`), with a two-line label joined
by one space and the trailing `ⓘ See …` cross-reference lines and enumerated value lines
(`00: Data mode OFF` and the like) excluded. For D2–D4 the label is the heading line
printed immediately above the box. The `- Character codes—` tables and the `To clear the
memory channel contents on 1A 00:` list on this page are prose/tables, not data-block
diagrams, and contribute no rows; the latter is noted under Observed disagreements because
it reprints indices `①, ②`, `③, ④`, `⑤` and `⑥` against different text.

## Method

Every recorded value was read by eye from a rendered page image, at the dpi and
enlargement given below. ImageMagick (`/opt/homebrew/bin/magick`) was available and used
for all crops and enlargements. `tesseract` was available but was **not** used: every
numeral and every label on this page was legible by eye at the enlargements below, so no
OCR aid was needed and no OCR value entered the ledger. `pdftotext` was **not run at all**,
in any mode, at any point.

Steps:

1. Fresh output directory `…/scratchpad/evidence/ic905-L` created (`rm -rf` of the render
   subdirectories first) so no pre-existing file could be mistaken for evidence.
2. Survey at 300 dpi:
   `pdftoppm -png -r 300 -f 17 -l 21 <pdf> r300/p` — read as whole-page images to locate
   the section whose printed heading is `• Memory content` and to fix the boundaries.
3. Reading raster at 400 dpi:
   `pdftoppm -png -r 400 -f 18 -l 20 <pdf> r400/p` (page 19 = 3308 × 4678 px).
4. First pass crops (400 dpi source, `+repage`, `-resize 200%` unless stated):
   - index band row 1: `-crop 2700x230+230+1030 … -resize 200%`
   - index band row 2: `-crop 2100x230+230+1280 … -resize 200%`
   - row 1 label band, left half: `-crop 1400x110+230+1040 … -resize 400%`
   - row 1 label band, right half: `-crop 1400x110+1500+1040 … -resize 400%`
   - row 2 label band, head/tail: `-crop 900x110+600+1290` and `-crop 900x110+1450+1290`,
     both `-resize 500%`
   - left legend column in four slices: `-crop 1500x700+230+1500`, `+230+2180`,
     `+230+2880`, and `-crop 1500x900+230+3550`
   - right legend column in four slices: `-crop 1400x750+1690+1500`, `+1690+2230`,
     `+1690+2960`, and `-crop 1400x800+1690+3650`
5. **Second independent pass**, from a different raster: page 19 re-rendered at 600 dpi
   (`pdftoppm -png -r 600 -f 19 -l 19 <pdf> r600/p`, 4961 × 7016 px) and re-tiled with
   different windows and different enlargements, so no crop boundary coincided with a first
   pass boundary:
   - row 1 in three overlapping tiles including the box row, `-crop 1330x340+340+1560`,
     `+1650+1560`, `+2960+1560`, each `-resize 250%`
   - row 2 in two tiles, `-crop 1600x360+340+1930` and `+1900+1930`, each `-resize 250%`
   - the three one-byte boxes, `-crop 1500x480+330+3760`, `-crop 1500x520+330+5520`,
     `-crop 1500x480+2540+2280`, each `-resize 250%`
   - the contested legend lines, `-crop 2100x360+2530+3020 -resize 200%`,
     `-crop 2100x420+2530+3230 -resize 180%`, `-crop 2100x700+2530+4520 -resize 180%`
   - the caption, `-crop 1800x300+300+1280 -resize 200%`
   The second pass re-read all 21 index cells, all 21 labels and the caption.
   **Disagreements between the two passes: none.** Every index, every index style and
   every label matched cell for cell.
6. The only directories touched were this leg's own output directories under
   `…/scratchpad/evidence/ic905-L` (`r300/`, `r400/`, `r600/`, `crops/`, `pass2/`,
   `cover/`), listed to confirm the renders had been written; no other directory was
   listed and no file outside that directory was opened except the source PDF itself.
7. No width, byte-position or extent arithmetic was performed, because the task excludes
   widths, byte positions and encodings from this ledger; consequently no arithmetic STOP
   is possible on this leg. The one arithmetic property that is inside scope — the index
   sequence itself — was checked: the D1 band runs `1, 2` `3, 4` `5` `6~10` `11` `12` `13`
   `14` `15` `16~18` `19~21` `22~24` `25` `26~28` `29~36` `37~44` `45~52` `53~68`, which is
   continuous from 1 to 68 with no gap, no overlap and no out-of-order index.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every one of the
  21 indices, in all four diagrams, is drawn identically: plain black digits enclosed in a
  thin open circular outline, on white ground. Nothing is filled, reversed, bracketed or
  bold, and no index is drawn twice in two different styles. All 21 rows are recorded
  `circled`, and this was confirmed independently at 400 dpi (×2–×5) and at 600 dpi (×2.5).
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** Every index
  label and every legend label on PDF page 19 is set horizontally; nothing on this page is
  rotated (the rotated `1 kHz digit: 0 ~ 9` style labels seen on PDF page 18 are on a
  different diagram, outside the transcribed extent). Position was in any case read from
  the picture only: no text layer was extracted at any point, so the extraction order was
  never seen and could not have influenced a value.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In the one-byte boxes the
  side labels do not sit in the order of the cells they point at, and the two boxes mirror
  each other: in D2 (`⑤`) the label `Fixed` leads to the **left** nibble (printed `0`) and
  the `0=OFF*` / `1= ★1` / `2= ★2` / `3= ★3` list leads to the **right** nibble, whereas in
  D4 (`⑮`) `Fixed` leads to the **right** nibble (printed `0`) and the `0=Digital squelch
  function OFF` list leads to the **left**; in D3 (`⑭`) the right-hand `0=OFF` … `7=TONE(T)/
  TSQL(R)` list runs a leader leftwards under the box to the **second** nibble while the
  `0=Duplex OFF` … `3=RPS` list, printed further left and lower, leads to the **first**.
  Each leader was followed by eye from label to arrowhead. No recorded value depended on
  it — this ledger records no per-nibble value — so the hazard changed no cell, but a leg
  that does record nibble meanings must not read these boxes by label position.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED, in
  the form the hazard describes.** No block of fields in these diagrams repeats another
  block of fields: D2, D3 and D4 each expand a *single* byte of D1 into its two nibbles
  rather than restating a run of fields, so there is no second block against which a
  printed index could be measured. Byte positions were therefore neither measured nor
  recorded — the task forbids them here and the CSV has no column for them. What *is*
  present is the bare repetition of the indices `5`, `14` and `15` across diagrams, which
  is recorded as printed under STOP 3 and reconciled nowhere.

## STOP findings

1. **PDF page 19 (folio 18) — index band of D1 versus its legend, indices `⑪` and `⑫`.**
   The index band prints two separate single indices: a circled `11` centred over one white
   `X X` cell and a circled `12` centred over the shaded `X X` cell immediately to its
   right, each with no bracket. The legend below prints them as a single grouped entry,
   `⑪, ⑫: Operating mode setting`, exactly the grouping style the same legend uses for
   `⑪, ⑫`'s neighbours `①, ②` and `③, ④` — which the band *does* draw as grouped brackets.
   The page therefore renders the same two fields with two different groupings. This stops
   because `field_index` is the row-level join key: a reader taking the band gets `11` and
   `12`, a reader taking the legend gets `11, 12`, and the two ledgers will not join. Both
   values are transcribed exactly as seen — two rows, `11` and `12`, from the band (the
   diagram, which is what this ledger indexes), each carrying the legend's label
   `Operating mode setting`, with `STOP 1` in `notes`. Nothing is merged and nothing is
   split.
2. **PDF page 19 (folio 18) — right column legend, the two lines beginning `⑯~⑱` and
   `⑲~㉑`.** Two consecutive lines print two different, non-overlapping index ranges
   against a character-for-character identical label:
   `⑯~⑱: Repeater tone frequency setting`
   `⑲~㉑: Repeater tone frequency setting`
   The single cross-reference note printed beneath both reads
   `ⓘ See “Repeater tone/tone squelch frequency setting.” (p. 23)`, naming two different
   settings for the two ranges. What is printed against `⑲~㉑` is therefore contradicted by
   the note that serves it. Transcribed exactly as printed: both rows carry
   `Repeater tone frequency setting`, with `STOP 2` in `notes`. The duplicate label is left
   standing and no substitution ("tone squelch") is made.
3. **PDF page 19 (folio 18) — indices `5`, `14` and `15` each printed twice on the page.**
   Circled `5` is drawn over the fifth cell of the D1 index band (the white `0 X` cell) and
   again above the one-byte box D2 in the left column; circled `14` over the D1 cell right
   of `13` and again above box D3; circled `15` over the D1 `X 0` cell and again above box
   D4. Each repeat is drawn in the same style as its first appearance (circled), so this is
   a repeat within the page's index sequence, not a styling conflict. Both appearances are
   transcribed, in printed order, as six rows (three in D1, one each in D2, D3, D4), with
   `STOP 3` in `notes` on all six. Neither appearance is suppressed and no attempt is made
   to decide which one is "the" field.

## Observed disagreements

Recorded exactly as printed; not resolved, and none of these stopped the transcription.

- `㊲~㊹`'s second line prints `(8 characters, fixed.)` — with a full stop before the
  closing bracket — while the parallel lines for `㉙~㊱` and `㊺~52` print
  `(8 characters, fixed)` without one. Transcribed as printed, full stop included.
- The tilde in the index band is optically tighter in some spans than others: `⑥~⑩` and
  `⑯~⑱` sit close, while `⑲ ~ ㉑`, `㊺ ~ 52` and `53 ~ 68` show visible whitespace either
  side of the tilde. Every legend line below the diagram sets the same ranges tight
  (`⑯~⑱:`, `⑲~㉑:`, `㊺~52:`). Read as kerning of one full-width tilde character rather
  than printed spaces, all range indices are recorded with a single `~` and no spaces —
  the one place in this ledger where a purely optical judgement was made, stated here so it
  can be checked.
- The legend groups its entries inconsistently against the band in one more place than
  STOP 1: the band draws `①, ②` and `③, ④` as bracketed pairs and the legend agrees, but
  the band's separate `⑬`, `⑭`, `⑮` are each single in both, so the `⑪, ⑫` case is the
  lone mismatch rather than a systematic difference.
- Below the legend the page reprints indices already used, against different text, under
  the heading `To clear the memory channel contents on 1A 00:` —
  `①, ②: Memory channel group (00 00 ~ 00 99)` (note `channel`, where the diagram's own
  legend says `①, ②: Memory group number`), `③, ④: Memory channel (00 00 ~ 00 99)`, and
  `⑤: “FF,” ⑥ ~ : None`, the last of which prints an open-ended range `⑥ ~` with no end
  index. This list is prose, not a data-block diagram, so it contributes no CSV row; it is
  recorded here because a reader scanning the page for numbered indices will meet these and
  because `①, ②` carries a different label there from the one this ledger records.
- The `⑤` box (D2) prints an asterisked value, `0=OFF*`, whose footnote `* Set 0 for Call
  channel.` is printed clear of the box, on its own line below it and above the `⑥~⑩`
  legend entry, rather than adjacent to the value it serves.
- PDF page 20 (folio 19) carries a cross-reference back to this material, read at 300 dpi
  as `* See ⑥ ~ ⑤② on “Memory content.” (p. 18)` with two circled indices. It is on an
  adjacent page, outside the transcribed extent; its end index was not re-rendered or
  verified at 400 dpi and no value was taken from it. It is noted only because it points
  at, and folio-agrees with, the page indexed here.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual,
transcription, source file, generated artefact or web resource was opened, and no directory
was listed.
