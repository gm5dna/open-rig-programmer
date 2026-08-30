# IC-R8600 memory-record transcription — leg B

Companion to `IC-R8600-transcription-b.csv` (50 data rows, 8 diagrams D1–D8).

## Source

- Title as printed on the cover (PDF page 1): the black cover panel prints
  `CI-V REFERENCE GUIDE`; below the rule the cover prints
  `COMMUNICATIONS RECEIVER` over `IC-R8600`, and at the foot `Icom Inc.`
- Revision code as printed: `A7375-2EX-3a`, printed at the foot of the
  left half of PDF page 28 (the last page), on the line immediately above
  `© 2017–2018 Icom Inc.` No revision code is printed anywhere on the cover.
- File: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/icr8600_civ_3a.pdf`
- Page count: 28 PDF pages.
- Folio relation confirmed on every page read: the printed folio is the PDF
  page number minus one (PDF 12 prints folio `11`, PDF 15 prints folio `14`,
  and so on). The cover (PDF 1) prints no folio.

## Extent

Pages rendered and read (folio in brackets):

| PDF page | folio | read? | what it contributed |
|---|---|---|---|
| 1 | none | read | cover title, publisher; no revision code |
| 6 | 5 | read | `◇ Command table (Continued)`, Cmd `1A` / Sub cmd `05*`, sub-commands `0038`–`0090`. Set-mode/menu material. **Contributed nothing**: no field of the memory record refers to it. |
| 7 | 6 | read | same table continued, sub-commands `0091`–`0142`. Set-mode/menu material. **Contributed nothing**: no field of the memory record refers to it. |
| 8, 9, 10, 16 | 7, 8, 9, 15 | rendered only, **not read** | rendered at 300 dpi in the locating sweep; never opened, and the source of no value here |
| 11 | 10 | read | `● Character entries` table (right column). The `Cmd 1A / Sub Cmd 00 / MEMORY NAME` row supplied the selectable-character list and the `Total character number` `16` used for `㉖ ~ ㊶`. The left column (`RX HPF/LPF setting`, `UTC Offset setting`, `Scope/FSK … color`) contributed nothing. |
| 12 | 11 | read | D1 — the whole common part of the record, indices `①`–`㊶` |
| 13 | 12 | read | D2 (FM), D3 (P25), D4 (D-STAR) |
| 14 | 13 | read | D5 (dPMR), D6 (NXDN) |
| 15 | 14 | read | D7 (DCR) and D8 (the clear-a-channel format) |
| 28 | 27 | read | revision code only |

**Was the character table printed at all?** Yes. PDF page 11 (folio 10), right
column, headed `● Character entries`, `Command: 1A 00,` / `1A 05 0107, 0114,
0128, 0134`. It is a five-row table with column heads `Cmd`, `Sub Cmd`,
`Edit items` / `Selectable characters`, `Total character number`. Its first row
(`1A` / `00` / `MEMORY NAME`) is the only row that bears on this record; the
other four rows (`NETWORK NAME`, `NETWORK RADIO NAME`, `OPENING COMMENT`,
`NTP SERVER ADDRESS`) belong to other commands and were not transcribed. The
table gives no byte-level encoding, only the permitted characters and a count.

**Were the set-mode/menu pages printed at all?** Yes — PDF pages 6 and 7 are two
full pages of the `1A 05` command table. **They contributed nothing to this
transcription**: not one field of the memory record refers to a set-mode item.
The only cross-references the memory record makes are to `"Receiving frequency"
(p. 9)`, `"Receiving mode" (p. 9)`, `"Offset frequency" (p. 9)`, `command 10
(p. 3)`, `"Character entries" (p. 10)`, `"TSQL frequency" (p. 18)` and
`"DTCS code" (p. 18)`. Of these only `p. 10` (= PDF page 11) is among the pages
this brief named; the others (folios 9, 3 and 18 = PDF pages 10, 4 and 19) were
**not read**, and the cells they would have filled are left empty with the
cross-reference recorded verbatim in `notes`. That absence is a finding, not a
gap filled from elsewhere.

**Where the transcribed material begins and ends.** It begins on PDF page 12
with the bullet heading `● Memory channel content` / `Command: 1A 00`;
immediately above it is printed `◇ Command formats (Continued)`, under the
running head `Remote control`. It ends at the foot of the left column of PDF
page 15 with the line `⑥ ~ :    None`; immediately after it the left column is
blank to the folio. The next printed matter in reading order is the right
column of PDF page 15, headed `● Programmable scan start (remote) data` /
`Command: 1A 0B 00` — a different record, excluded (see judgement calls).

## Method

Every recorded value was read from a rendered page image. No value was read
from a text layer.

1. **Locate, 300 dpi.** Into a fresh directory `…/legs-out/icr8600/B/r300`:
   `pdftoppm -png -r 300 -f 1 -l 1 <pdf> p` and
   `pdftoppm -png -r 300 -f 6 -l 16 <pdf> p` (later `-f 28 -l 28` for the
   revision code). The whole-page renders were read as images to find the
   blocks whose printed headings match the task.
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f 11 -l 15 <pdf> q` into
   `…/B/r400`. Every first-pass value was read from these.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`,
   `/opt/homebrew/bin/convert`) and was used throughout. Every band, numbered
   sub-diagram and leader legend was cropped into its own image and enlarged,
   e.g.
   `magick r400/q-12.png -crop 1250x760+390+840 +repage -resize 220% crops/p12_bands.png`,
   `magick r400/q-12.png -crop 900x520+1690+1690 +repage -resize 400% crops/p12_f2021b.png`,
   `magick r400/q-13.png -crop 1100x620+1700+1740 +repage -resize 320% crops/p13_nac.png`,
   `magick r400/q-14.png -crop 1400x800+300+3230 +repage -resize 280% crops/p14_scrmkey2.png`,
   `magick r400/q-15.png -crop 1400x1300+300+1250 +repage -resize 240% crops/p15_dcr_a.png`,
   `magick r400/q-11.png -crop 1450x520+1700+1120 +repage -resize 300% crops/p11_memname.png`.
   Enlargements ran from 190 % to 400 %; every numeral, rule, arrowhead and
   glyph recorded here sits clear of its neighbours at the magnification used.
4. **`pdftotext` was NEVER run**, in any form, for navigation or otherwise.
   Navigation was done entirely by reading the 300 dpi whole-page renders.
5. **`tesseract` was available but was NOT used.** No OCR was run at any point;
   every value was read by eye off the render.
6. **Second independent pass — done.** After the first pass was complete, a
   different raster was produced: `pdftoppm -png -r 500 -f 12 -l 15 <pdf> s`
   into `…/B/r500` (500 dpi instead of 400), cropped on a **different geometry**
   — whole column strips (e.g. `-crop 1700x1900+380+1150 +repage -resize 165%`,
   `-crop 1750x2400+2150+1150 +repage -resize 150%`) rather than the first
   pass's per-diagram crops — and at different enlargements (150–190 %
   instead of 220–400 %). Every index, box count, label, code and leader
   attachment was re-read from those strips.
   **Disagreements between the two passes: none.** Both passes returned the
   same box counts (band 1 = 6 solid boxes + ellipsis + 1 = 10 byte positions;
   band 2 = 4 + ellipsis + 3 = 9; band 3 = 7 + ellipsis + 1 = 22; FM 7, P25 4,
   D-STAR 2, dPMR 8, NXDN 6, DCR 7), the same leader-to-nibble attachments
   including all ten reversed legends, and the same character strings including
   the misspelling `100th posiiton`.

### Other tooling disclosed

Two acts fall outside "reading a render" and are declared here so the
attestation below is read against the full record. Neither opened any source
other than this PDF, and neither was the source of any transcribed value.

- `pdfinfo` was run **once**, on this same PDF. It is the source of the page
  count (28) reported under `## Source`, and of nothing else. The cover title
  and the revision code were read from renders, not from it. No other document
  was queried.
- `ls -la` was run on my own render output directories (`…/B/r300`, `…/B`) to
  confirm which renders had been produced. No repository, manual, fixture or
  prior-output directory was listed, searched or browsed; the attestation's
  "no directory was listed" is true of every directory outside my own output
  tree.

### Transcription conventions

- **Circled indices** are recorded with the Unicode circled-number characters
  (`①`–`⑳`, `㉑`–`㊶`, `㊷`–`㊾`), which is the closest available match to the
  printed glyphs. Separators are as printed: `①, ②` for a pair, `⑥ ~ ⑩` for a
  range.
- **Row granularity.** One row per printed index token. Where the field's
  sub-diagram prints each index separately over its own box and gives it its
  own semantics (`⑱`/`⑲`, `⑳`/`㉑`, the NAC, COM ID, UC code and key blocks),
  each printed index gets its own row; where the document treats the token
  jointly and prints no per-index breakdown (`①, ②`, `⑥ ~ ⑩`, `㊸ ~ ㊺` Tone
  squelch frequency, `㉖ ~ ㊶`), the token is one row. `label_verbatim` on such
  split rows carries the printed heading label of the pair or range; the
  per-index annotation (`TS function`, `Tuning step setting`) is carried in
  `values_verbatim`.
- **Space runs.** The page pads leader legends into columns
  (`10th position:    0 ~ 9`). Runs of spaces used only for column alignment
  are normalised to one space; no other character is altered.
- **Glyphs recorded by nearest character.** `1=DUP–` — the dash is drawn as a
  long dash and is recorded as U+2013. `★1 ~ ★9` — filled stars, recorded as
  U+2605. The equals sign is drawn at two visibly different widths on the same
  page (compare `00=OFF` with `10＝10dB` in `㉒`); all are recorded as a plain
  `=`, and the variation is listed under observed disagreements.
- **Widths.** Every box in every band is drawn as two nibbles, so one printed
  index = one byte. Widths were counted on the render from the brackets over
  the bands; no width is printed as a number anywhere in this material. No
  field's width is conditional, so no field needed the one-row-per-documented-
  width treatment.

### Judgement calls (recorded, not resolved)

1. **`mode_class` column.** The Tier 4b hazard clause asks for the mode classes
   to be "joined by a `mode_class` column"; the CSV rules in the same brief
   require the header line "exactly as given (no spaces after commas, no extra
   columns, no reordering)" and "this header line and no other". These conflict.
   I kept the header exactly as specified, because it is stated as an absolute
   and because a tenth column would break the contract the downstream reader is
   written against. The mode class is carried instead in two places that lose
   nothing: each mode class has its own `diagram_id` (D2 FM, D3 P25, D4 D-STAR,
   D5 dPMR, D6 NXDN, D7 DCR), and **every row's `notes` begins with
   `mode class: …`**, so a one-line derivation reconstitutes the column.
2. **Scope of PDF page 15.** The brief names PDF pages 12–15 as the memory-record
   diagrams. Page 15's right column holds a *different* record —
   `● Programmable scan start (remote) data`, `Command: 1A 0B 00` — whose index
   sequence restarts at `①` and runs to `㉕`, and which cross-references the
   memory record's own fields. I excluded it: merging it would have put a second
   `①`–`㉕` sequence into the same file with different semantics. It is described
   under observed disagreements so the exclusion is visible.
3. **Inclusion of D8.** The block at the foot of page 15's left column
   (`Command 1A 00 clears a memory channel by sending the command in the
   following format.`) has no caption and no drawn band, but it does print four
   numbered field entries of this same record. I transcribed it as D8 rather
   than discard printed per-index content; it carries empty `width_bytes`
   because no width is printed or drawable there.
4. **`㉖ ~ ㊶` values.** The character list in `values_verbatim` is printed on
   PDF page 11, not page 12. `pdf_page` is given as `12` (where the field, its
   label, its width and its cross-reference are printed) and the `notes` state
   plainly that the list was transcribed from PDF page 11's Character entries
   table, MEMORY NAME row.
5. **Cross-referenced pages not read.** Folios 3, 9 and 18 (PDF pages 4, 10 and
   19) were not among the pages the brief named and were not opened. The cells
   they would have filled are empty, with the cross-reference quoted in `notes`.

## Hazards encountered

- **(a) Numeral styling varies within one diagram** — **NOT ENCOUNTERED.** Every
  index numeral in all eight blocks is drawn the same way: a thin outlined
  circle around the numeral, unfilled, unbracketed, not bold, not reversed; the
  two-digit indices (`⑩` upward) sit in a slightly wider oval purely to fit two
  digits, and that widening is applied uniformly to every two-digit index. No
  index appears in two styles, and no index is drawn twice with different
  styling. (The `ⓘ` glyph that opens two notes on PDF page 12 and one on page 15
  is a note bullet, not a field index, and was not treated as one.)
- **(b) Vector groups with rotated labels** — **NOT ENCOUNTERED** in the
  transcribed blocks. Every label, index, leader legend and code line on PDF
  pages 12–15 is set horizontally; no rotated or vertical text appears in any of
  the eight blocks. The hazard is demonstrably real in this document — the
  `Scope/FSK … color` diagram on PDF page 11 does set its nibble legends
  vertically — but that diagram is outside this task. The extraction-order half
  of the hazard could not be tested and did not need to be: no text layer was
  read at any point, so no extraction order could have influenced a value.
- **(c) Leader-line label order reversed** — **ENCOUNTERED**, in ten
  sub-diagrams. In every multi-leader legend the labels are printed top to
  bottom in the *exact reverse* of the drawn left-to-right nibble order. Each
  leader was followed by eye from its label back to the cell it lands on, twice,
  on two different rasters. The ten: `⑳, ㉑` Programmable tuning step (PDF 12);
  P25 `㊸ ~ ㊺` NAC (PDF 13); D-STAR `㊸` CSQL code (PDF 13); dPMR `㊸, ㊹` COM ID,
  `㊺` CC and `㊼ ~ ㊾` Scrambler key (PDF 14); NXDN `㊸` RAN code and `㊺ ~ ㊼`
  Encryption key (PDF 14); DCR `㊸, ㊹` UC code and `㊻ ~ ㊽` Encryption key
  (PDF 15). `⑤` on PDF 12 is the one multi-leader legend that is *not* reversed:
  its left-nibble list is printed to the left and below, its right-nibble list to
  the right, and both leaders land where the layout suggests.
- **(d) Printed index differs from measured position** — **NOT ENCOUNTERED.**
  Blocks that repeat another block do occur: the six mode-class tails
  (D2–D7) each restart at `㊷` and each was transcribed independently from what
  is printed against it, never copied from a counterpart. For every field in
  those six blocks the CSV `notes` record **both** the printed index and the
  byte position measured on the render (`measured byte position N of this
  band`), and the two never diverged: in each block the printed indices run in
  the same left-to-right order as the boxes they sit over, with no gap, repeat
  or transposition. The measured positions are recorded for the D1 rows as well.

## STOP findings

**None.**

Reasons for confidence:

1. **The arithmetic adds up everywhere.** D1's three bands carry `①`–`⑩`
   (10 byte positions: 6 drawn boxes, an ellipsis covering `⑦⑧⑨`, then `⑩`),
   `⑪`–`⑲` (9 positions: 4 boxes, an ellipsis covering `⑮⑯`, then 3 boxes) and
   `⑳`–`㊶` (22 positions: 7 boxes, an ellipsis covering `㉗`–`㊵`, then `㊶`) —
   a continuous 1…41 with no gap, no overlap and no repeat. Every mode tail's
   drawn box count equals its printed index span exactly: FM `㊷`–`㊽` = 7 boxes,
   P25 `㊷`–`㊺` = 4, D-STAR `㊷`–`㊸` = 2, dPMR `㊷`–`㊾` = 8, NXDN `㊷`–`㊼` = 6,
   DCR `㊷`–`㊽` = 7. `㉖ ~ ㊶` measures 16 bytes, which matches both the printed
   `(Up to 16 characters)` and the `Total character number` `16` in the
   Character entries table.
2. **No index sequence is discontinuous within any one diagram.** Each of the
   eight diagrams' indices is strictly increasing with no repeat and no gap.
   `㊷` recurring as the first index of six diagrams, and `①, ②` / `③, ④` / `⑤`
   recurring in D8, are separate index sequences belonging to separate printed
   blocks — exactly the structure the document sets out ("In the modes other
   than FM and Digital, `㊷` and or later is not used") — not a discontinuity
   inside one sequence.
3. **Every value was legible.** No cell is `UNREADABLE`. Every numeral, code and
   label was resolved at 400 dpi enlarged, and again independently at 500 dpi on
   a different crop geometry.
4. **The two passes agreed on every cell.** No disagreement arose, so no third
   render was needed.
5. **Nothing printed contradicts anything else printed.** The oddities found are
   listed below; each is an isolated printed statement with no printed
   counter-statement to contradict.

## Observed disagreements

Recorded exactly as printed; not resolved.

1. **`⑳, ㉑` Programmable tuning step — the digit weights are not in order.**
   Following each of the four leaders by eye on both rasters, the drawn nibble
   order left to right is `1 kHz digit`, `100 Hz digit`, `100 kHz digit`,
   `10 kHz digit`. That is neither ascending nor descending by magnitude, and it
   is not the order the four labels are printed in (which is `10 kHz`,
   `100 kHz`, `100 Hz`, `1 kHz`, top to bottom — the reverse of the drawn
   order). Both readings are recorded exactly; nothing else printed states an
   order, so this is not a contradiction and is not a STOP.
2. **Misspelling `100th posiiton`** — PDF page 13, right column, third leader
   legend from the bottom of the `㊸ ~ ㊺ NAC` sub-diagram. Printed
   `100th posiiton: 0 ~ F`. Transcribed as printed. The corresponding entries
   in every other block are spelled `100th position`.
3. **`㊷ and or later`** — PDF page 12, the `ⓘ` note above the field list:
   `ⓘIn the modes other than FM and Digital, ㊷ and or later is not used. In the
   FM and Digital modes, entering ㊷ and or later can be omitted.` The phrase
   `and or later` appears twice and is not idiomatic; nothing is printed to
   clarify it.
4. **D-STAR `㊷` skips code 1.** PDF page 13 prints `0=OFF` and `2=CSQL` for the
   D-STAR Digital squelch type, with no code `1`. Every other block's `㊷` uses
   consecutive codes from `0` (FM `0`/`1`/`2`; P25 `0`/`1`; dPMR `0`/`1`/`2`;
   NXDN `0`/`1`; DCR `0`/`1`).
5. **`0 =OFF`** — PDF page 12, the right-nibble legend of `⑤`, is printed with a
   space between the code and the equals sign. Every other code line on that page
   is set tight (`0=OFF`).
6. **Two widths of equals sign on one page.** Within `㉒` on PDF page 12,
   `00=OFF` uses a narrow equals while `10＝10dB`, `20＝20dB` and `30＝30dB` use a
   visibly wider one; the same mixture appears in `1＝ON` on PDF page 14. All are
   recorded as a plain `=`.
7. **The MEMORY NAME character list contains a vertical bar.** The Character
   entries table (PDF page 11) prints `{ | }` among the selectable characters,
   which collides with the ` | ` entry separator this CSV uses. That one cell is
   a **single** entry and must not be split on ` | `; the row's `notes` warn of it.
8. **A second, differently indexed record shares PDF page 15.** The right column
   prints `● Programmable scan start (remote) data`, `Command: 1A 0B 00`, whose
   indices restart at `①` and run to `㉕`, and which refers back to this record's
   own fields (`Refer to "Duplex setting (⑬)" (p. 11)`, and so on). It is a
   different record and is excluded from this CSV.
9. **`⑥ ~ :` is an open-ended index token.** In D8 the last entry's index is
   printed as `⑥ ~` with no closing index, against the value `None`. Recorded
   verbatim as `⑥ ~`.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.
