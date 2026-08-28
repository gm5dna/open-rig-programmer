# IC-9700 CI-V memory-record transcription (leg B)

## Source

- Document title as printed on the cover (PDF page 1): the black cover panel prints the Icom logo, then in a black band beneath it `CI-V REFERENCE GUIDE`; lower on the same page, above the model name, `VHF/UHF ALL MODE TRANSCEIVER`, then `IC-9700`, and at the foot `Icom Inc.`
- Revision code as printed: `A7508-3EX-4`, printed at the bottom-left of the back cover (PDF page 28), directly above `© 2019–2023 Icom Inc.    Mar. 2023`. No revision code is printed on the front cover.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic9700_civ_rev4.pdf`
- Page count: 28 PDF pages. Confirmed from the renderer itself: `pdftoppm -f 29 -l 29` refused with `Wrong page range given: the first page (29) can not be after the last page (28)`.

### Transcription conventions for the index glyphs

The document draws its field indices as circled numerals. Unicode has circled
numerals only to 50, and filled/reversed circled numerals only to 20, so:

- outlined circled numerals 1–50 are written with the matching Unicode glyph: `①` … `㊿`;
- outlined circled numerals above 50 are written `(51)`, `(52)`, `(67)`;
- filled/reversed circled numerals are written with the matching Unicode glyph where one exists (`❺`) and `[51]` where none does.

Every CSV row additionally names, in `notes`, the style in which its index is
drawn ("outlined circled numeral" / "filled/reversed circled numerals"), so the
style is recorded independently of the glyph used to write it.

## Extent

Rendered: PDF pages 1, 8, 15, 16 and 28. Read: PDF pages 1, 8, 15, 16, 28.
Transcribed: PDF pages 15 and 16 only.

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none printed | Cover title, model, publisher — `## Source` only. |
| 8 | `7` | Nothing. See below. |
| 15 | `14` | Diagram D1 and all D1 field semantics. |
| 16 | `15` | The character tables (values for `(52) ~ (67)`) and diagram D2 with its two field tables. |
| 28 | none printed | Revision code `A7508-3EX-4` — `## Source` only. |

**Set-mode / menu material (PDF page 8, folio 7): printed, but contributed
nothing.** The page is printed and legible. Its heading is `◇ Command table
(Continued)`, and it prints a two-column command table of `1A*` `05`
sub-commands `0090` to `0138` with `SET > …` menu paths and their data ranges.
No field of either transcribed diagram refers to it: every cross-reference
printed against a D1 field points to printed folio 13, folio 20 or folio 15,
and the D2 note points to folio 14. No cell of the CSV was taken from PDF page
8.

**Character table (PDF page 16, folio 15): printed, and used.** The section
`• Codes for character entries` prints three tables — `- Character codes—
Letters and Numbers`, `- Character codes— Symbols`, and an unlabelled
`Command` / `Set item/selectable characters` table whose first row is `1A 00` /
`Memory name` / `All characters are usable.` These supplied the entire
`values_verbatim` cell for field `(52) ~ (67)`, reached by that field's own
printed cross-reference `See “Codes for character entries.” (p. 15)`. The same
page also prints a separate `Memory keyer character entries` table under
`Command: 1A 02`; that table belongs to command 1A 02, not to the memory
record, and was not used.

### Where the transcribed material begins and ends

**PDF page 15 (folio 14).** Immediately before the material: the running head
`Remote control` under a rule, then `◇ Command formats (Continued)`. The
material begins at `• Memory content` / `Command: 1A 00` and the two wrapped
rows of the data block. It ends at the foot of both columns — in the left
column with `ⓘ RPS can be set when DD mode is selected, and Duplex (−, +) can
be set when other than DD mode is selected.`, in the right column with the grey
`NOTE:` box whose last words are `you set the same data as ⑤ ~ (51).`
Immediately after: the folio `14`, centred at the foot.

**PDF page 16 (folio 15).** Immediately before the D2 material, at the top of
the right column: nothing — `• Band stacking register` / `Command: 1A 01` is
the first thing in that column, the running head `Remote control` and `◇
Command formats (Continued)` spanning above. Immediately after the D2 material
(`Example: … use code “0202.”`): `• Memory keyer character entries` /
`Command: 1A 02`. In the left column, the character-entry material begins at
`• Codes for character entries` / `Command: 1A 00, …` and ends with the
`Command` / `Set item/selectable characters` table, last row `0182` / `SET >
Time Set > Date/Time > NTP Server Address`. Immediately after: white space,
then the folio `15`.

### Diagrams

- **D1** — printed caption verbatim: `• Memory content`, with `Command: 1A 00`
  on the line beneath. Position: upper third of PDF page 15, spanning the full
  text width; the block is drawn as two wrapped rows joined left-to-right by a
  curved arrow that leaves the right end of the upper row and enters the left
  end of the lower row.
- **D2** — printed caption verbatim: `• Band stacking register`, with `Command:
  1A 01` on the line beneath. Position: top of the right-hand column of PDF
  page 16, a single two-byte block centred above the grey `NOTE:` box.

D2 is the only numbered data-block diagram printed on PDF page 16. It is a
register-content block rather than a memory-channel record; it is transcribed
here because it is the numbered data-block diagram the task's page range names,
and this note records that judgement rather than hiding it.

### Sub-diagrams inside D1

Three D1 fields (`④`, `⑬`, `⑭`) are explained with a small one-byte diagram
drawn inside their text entry, each showing the byte split into two nibble
cells with leader arrows. These carry no caption of their own beyond a circled
index above the box, so they were not assigned a diagram id; they are recorded
as the semantics of their parent field. Their nibble-level content is in the
`values_verbatim` and `notes` of rows `④`, `⑬` and `⑭`.

### Measured extent

Counting one byte per drawn byte cell, and taking the length of each elided
group from its printed index range, D1 measures 114 bytes: positions 1–24 in
the upper wrapped row (fields `①` to `㉔`), 25–51 for `㉕ ~ ㉗` through `㊹ ~
(51)`, 52–98 for `❺ ~ [51]`, 99–114 for `(52) ~ (67)`. Each row's measured
byte position is recorded in its `notes`. **The document prints no total and no
byte addresses**; 114 is my measurement, not a printed figure, and nothing
printed contradicts it.

## Method

1. **Locate — 300 dpi.** `pdftoppm -png -r 300 -f 8 -l 8` and `-f 15 -l 16`
   into a fresh `evidence/ic9700-B/r300/`. Pages read as images to find the
   sections. Cover and back cover later rendered at 200 dpi into the same
   directory (`cover-01.png`, `last-28.png`).
2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f 15 -l 16` into
   `evidence/ic9700-B/r400/`. All first-pass values read from these.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`)
   and was used throughout. First-pass crops (into `crops/`), e.g.
   `magick r400/p-15.png -crop 1400x220+300+800 +repage -resize 250% crops/r1a.png`
   (upper row, left half), `-crop 1400x220+1600+800 … r1b.png` (upper row, right
   half), `-crop 1300x260+400+1060 … r2a.png` and `-crop 1300x260+1600+1060 …
   r2b.png` (lower row), plus `-crop 900x520+280+2080 … -resize 300% f4.png`,
   `-crop 900x560+280+3450 … f13.png`, `-crop 900x520+1700+1300 … f14.png` for
   the three nibble mini-diagrams, and further crops of both text columns and
   the grey NOTE box.
4. **`pdftotext` — NOT RUN.** `pdftotext -layout` was never invoked, on this or
   any file. Navigation was done entirely by reading the rendered page images.
5. **`tesseract` — available but NOT used.** `/opt/homebrew/bin/tesseract` is
   installed; no OCR was run. Every value was read by eye from an enlarged
   render.
6. **`pdfinfo`** was run once, on this same PDF, and reported the page count and
   the fact that the file is a 28-page A4 document. No value in the CSV came
   from it, and the page count was independently confirmed from `pdftoppm`'s own
   refusal of page 29 (see `## Source`).

### Second-pass record

**Both passes were done.** The second pass re-read every value from a different
raster: **600 dpi** (`pdftoppm -png -r 600 -f 15 -l 16` into `r600/`), with
**different crop windows** — the two wrapped rows of D1 cut into overlapping
**thirds** rather than halves (`-crop 1450x330+450+1200`, `+1800`, `+3150` for
the upper row; `-crop 1450x400+…+1600` for the lower row), each at `-resize
200%`; D2 re-cut at `-crop 1100x420+2700+1080 -resize 320%`; the symbols table
re-cut into two full-width halves at `-resize 170%`, the letters/numbers and
`Command` tables likewise. Different dpi, different window boundaries,
different enlargement factors, so no cell boundary fell in the same place twice.

**Disagreements between the two passes: none.** Every field index, every index
style, every drawn byte-cell count, every bracket span, every enum value, every
ASCII code and both mini-diagram nibble assignments read identically in both
passes. No third render was needed.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** D1 draws
  every index as an *outlined* circled numeral except one group in the lower
  wrapped row, whose bracket is labelled with *filled/reversed* circled
  numerals: filled 5 `~` filled 51, sitting between `㊹ ~ (51)` and `(52) ~
  (67)`. Both styles are recorded per row in `notes`; neither is normalised to
  the other, and no meaning is inferred for the filled style beyond what the
  grey NOTE box prints. See STOP 2.
- **(b) Vector groups with rotated labels — NOT ENCOUNTERED.** Every index
  label, bracket label and leader label on both diagrams is set horizontally,
  upright, above or beside the cell it belongs to. Nothing was read from a text
  layer in any case: `pdftotext` was never run.
- **(c) Leader-line label order reversed — NOT ENCOUNTERED.** The only leader
  lines are in the three one-byte mini-diagrams. Each was followed by eye from
  label to cell at 300% enlargement: in `④` the label `Fixed` sits below-left
  and its arrow lands on the **left** nibble (which prints the literal `0`),
  while the `0=OFF*` … `3= ★3` list sits to the right and its leader runs left
  along a horizontal rule to the arrow under the **right** nibble; in `⑬` the
  `0=Duplex OFF` … `3=RPS` list sits below-left and lands on the **left**
  nibble, the `0=OFF` … `3=DTCS` list sits to the right and lands on the
  **right** nibble; in `⑭` the enum list sits below-left and lands on the
  **left** nibble `X`, while `Fixed` sits to the **right** of the box and its
  leader runs left to the arrow under the **right** nibble (which prints the
  literal `0`). No label pointed at a cell other than the one nearest its own
  leader.
- **(d) Printed index differs from measured position — ENCOUNTERED.** The
  filled-numeral group is printed as indices 5 to 51 but sits at measured byte
  positions 52–98; the group printed `(52) ~ (67)` sits at measured byte
  positions 99–114. Both the printed index and the measured position are
  recorded for every row (index in `field_index`, position in `notes`), and
  they are not reconciled: the printed indices are left exactly as printed and
  the measured positions are left exactly as measured.

## STOP findings

1. **PDF page 15, folio 14, left column, third field entry — the `④` section's
   mini-diagram is captioned `③`.** The entry is headed `④ Select memory
   setting`. Below that heading, centred above the one-byte box drawn `0 : X`,
   the page prints an outlined circled **3**, not an outlined circled 4.
   Confirmed at 400 dpi enlarged 300% and again at 600 dpi: the glyph is a
   single digit 3 in an outlined circle, unmistakably narrower than the `④` in
   the heading two lines above it, and the neighbouring mini-diagrams in the
   same block are captioned `⑬` (under the `⑬` heading) and `⑭` (under the
   `⑭` heading), so the convention elsewhere is that the caption repeats its
   own heading. This stops because it is an index printed twice with different
   referents — outlined circled 3 already labels the second byte of the memory
   channel number field — and because the caption contradicts the heading it
   sits under. Transcribed as seen: the CSV row keeps `field_index` `④` (the
   heading and the diagram bracket both say 4) and records the caption verbatim
   in `notes` with `STOP 1`. Nothing was repaired.

2. **PDF page 15, folio 14, lower wrapped row of the D1 data block — the index
   sequence repeats 5 to 51 in a different numeral style.** Reading the lower
   row left to right, the bracket labels run `㉕ ~ ㉗`, `㉘ ~ ㉟`, `㊱ ~ ㊸`,
   `㊹ ~ (51)`, then **filled/reversed circled 5 `~` filled/reversed circled
   51**, then `(52) ~ (67)`. Indices 5 through 51 therefore appear twice in one
   block — once outlined (upper row and start of lower row), once filled — and
   the sequence runs 51, then back to 5, then forward to 52. The filled group is
   drawn as a single long dashed outline containing a dotted ellipsis with **no
   byte cells at all**, and no label, no width and no values are printed against
   it anywhere on the page; the only prose about it is the grey NOTE box. This
   stops as a repeat, an out-of-order index, and an index printed twice with
   different styling. Transcribed as seen: its own CSV row, `field_index` `❺ ~
   [51]`, empty `label_verbatim` and empty `values_verbatim` because the page
   prints neither, width `47` from the printed index range, and `STOP 2` in
   `notes` alongside the verbatim NOTE text and the measured byte positions
   52–98. Nothing was carried over from the outlined `⑤ ~ (51)` fields.

No other STOP arose. Reasons for confidence on the rest: every non-elided group
has as many drawn byte cells as its printed index range has indices (checked
cell by cell in both passes: 1, 2, 1, 2, 1, 1, 1, 3, 3, 3, 1, 3); every elided
group is drawn in the same idiom (two byte cells with a dashed ellipsis cell
between) so a short drawing is the document's convention, not a contradiction;
the groups tile the block end to end with no overlap and no gap; and the index
sequence is otherwise strictly ascending and complete from 1 to 67.

## Observed disagreements

Recorded exactly as printed, not resolved.

1. PDF page 16 (folio 15), grey NOTE box in the right column, prints `* See ⑤
   to (51) on ‘Memory content setting.’ (p. 14)`. The section it points at, on
   PDF page 15 (folio 14), is headed `• Memory content` — without `setting`.
2. `1.2 GHz` on PDF page 15 (folio 14), field `①`, third value, printed with a
   space; `1.2GHz` on PDF page 16 (folio 15), frequency band codes table, code
   `03`, printed without one.
3. PDF page 16's `- Character codes— Symbols` table pairs typographic glyphs
   with straight-ASCII codes: a right double quotation mark `”` against `22`
   (ASCII 22 is the straight quotation mark), a right single quotation mark `’`
   against `27` (ASCII 27 is the straight apostrophe), and a long dash `−`
   against `2D` (ASCII 2D is the hyphen-minus). Transcribed as the glyphs are
   drawn.
4. `④` prints its fixed nibble first (`0 : X`, `Fixed` on the left nibble)
   while `⑭` prints its fixed nibble second (`X : 0`, `Fixed` on the right
   nibble). Both are recorded as drawn.
5. In `- Character codes— Letters and Numbers` the second data row prints
   `0 ~ 9` / `30 ~ 39` in the left column pair, and its two right-hand cells
   (`Character` and `ASCII code`) are each struck through with a diagonal rule
   running bottom-left to top-right rather than left blank. Nothing is printed
   in them, so nothing was transcribed from them.
6. Field `(52) ~ (67)` is printed `(16 characters; fixed)` while the
   `Set item/selectable characters` table entry for the same command, `1A 00` /
   `Memory name`, gives no length at all, only `All characters are usable.` —
   unlike the `1A 05` rows beneath it, which each print `(up to 15 characters)`
   or `(up to 16 characters)`.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.

*Tooling disclosure, outside the attestation and additional to it:* `ls` was run
once on my own render output directory inside `evidence/ic9700-B/`, and
`pdfinfo` and `pdftoppm` were run on this same PDF (page count and rendering
only, as recorded in `## Method` and `## Source`). `pdftotext` was never run. No
value in the CSV came from anything but the rendered page images.
