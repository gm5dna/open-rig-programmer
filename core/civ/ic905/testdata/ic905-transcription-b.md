# IC-905 CI-V — memory-record transcription, leg B

Companion to `ic905-transcription-b.csv` (21 records, header + 21 lines, UTF-8, no BOM).

## Source

- Title as printed on the cover (PDF page 1): `CI-V REFERENCE GUIDE`, above the rule
  `ALL MODE TRANSCEIVER` / `IC-905`, with `Icom Inc.` at the foot. The cover carries no
  revision code.
- Revision code as printed: `A7711-9EX-2`, printed at the bottom-left of PDF page 31
  (the unnumbered back page), directly above `© 2023–2024  Icom Inc.      May 2024`.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic905_civ_2.pdf`
- Page count: 31 PDF pages, A4 (595.276 × 841.89 pt).

## Extent

Pages rendered and read (PDF page number → printed folio):

| PDF page | printed folio | rendered at | what it contributed |
|---|---|---|---|
| 1 | none (cover) | 200 dpi | Cover title only, for `## Source`. No field values. |
| 9 | `8` | 300 dpi | **Set-mode / menu material: printed, but contributed nothing.** See below. |
| 17 | `16` | 300 dpi | Located only, to confirm the section boundary. Not transcribed. |
| 18 | `17` | 300 dpi | Establishes what is printed immediately before the transcribed material. Not transcribed. |
| 19 | `18` | 300 / 400 / 600 dpi | **The memory-record data-block diagram and its entire legend.** Source of every D1 and D2 row. |
| 20 | `19` | 300 / 400 / 600 dpi | **The character table.** Source of the `values_verbatim` cell of the `53~68` row. |
| 21, 22 | `20`, `21` | 300 dpi | Located only, to confirm nothing of the Memory-content section runs on. Not transcribed. |
| 31 | none (back) | 200 dpi | Revision code, for `## Source`. No field values. |

**Where the transcribed material begins and ends.** On PDF page 19 (folio 18) the material
begins under the running head `REMOTE CONTROL`, the reversed grey band
`Remote control (CI-V) information`, the sub-heading `◇ Command formats` and the bold
section heading `• Memory content` / `Command: 1A 00`. Immediately before it, at the foot
of PDF page 18 (folio 17), the preceding section `• Codes for CW message contents` ends
with the note `ⓘ “^” is used to transmit a string of characters with no inter-character space.`
The material ends at the foot of the right-hand column of PDF page 19 with the last line of
the block `To clear the memory channel contents on 1A 00:`, which reads
`⑤: “FF,” ⑥ ~ : None`; below it only the folio `18` is printed. The next printed matter, at
the head of PDF page 20 (folio 19), is the bold section heading `• Codes for character entries`,
whose two tables supply the `53~68` values.

**The character table: printed, and what it contributed.** PDF page 20 (folio 19) prints it,
in the left-hand column, as two tables — `- Character codes— Letters and Numbers` and
`- Character codes— Symbols` — each with the column heads
`Character | ASCII code | Character | ASCII code`. It is the only field-value source outside
page 19 used in this transcription, and it supplies the whole `values_verbatim` cell of the
`53~68` (`Memory name setting (16 characters, fixed)`) row, which is the one field whose
legend entry points at it (`ⓘ See “Codes for character entries.” (p. 19)`).

**The set-mode / menu material: printed, and what it contributed — nothing.** PDF page 9
(folio 8) is printed and was read. Its heading is `◇ Command table`, and it is a two-column
command table whose every row is `1A* | 05 | 01 17 … 01 47` under the sub-header
`SET > Connectors` — set-mode items for USB/AV-OUT, LAN, MOD Input, SEND Output,
USB SEND/Keying, External Keypad, CI-V, USB (B) Function, MIC Jack 8V and REF OUT.
**No field of the memory record refers to it.** Every cross-reference printed against a
memory-record field points elsewhere (folios 16, 17, 19 and 23), and none points at a
set-mode item. It therefore contributed no cell to the CSV, and that absence is the finding,
not a gap.

**What on the named pages was deliberately not transcribed.** The right-hand column of
PDF page 20 carries a second, separate diagram under the bold heading
`• Band stacking register` / `Command: 1A 01`, with its own indices ① and ②. Its printed
heading is not the memory-record heading, and its own note directs the reader back to
`⑥ ~ ㊺ on “Memory content.” (p. 18)` — i.e. it borrows this record rather than being one.
It is recorded here but was not given a diagram id or CSV rows.

**Cross-references not followed.** Eight of the seventeen D1 fields print no values at all,
only a pointer to another section by printed folio: `(p. 16)` for ⑥~⑩ and ⑪, ⑫; `(p. 17)`
for ㉖~㉘; `(p. 23)` for ⑯~⑱, ⑲~㉑, ㉒~㉔, ㉕ and the three call-sign fields. Those folios
are outside the pages this leg was given, and were not opened. Their `values_verbatim`
cells are therefore empty — meaning nothing is printed *for that field at that place* — and
the pointer is quoted verbatim in `notes` so a reader can see exactly what the document
defers and where to.

## Method

1. **Locate — 300 dpi.** A fresh directory tree was created beneath
   `…/evidence/ic905-B` (`r300/`, `r400/`, `r600/`, `crops/`, `pass2/`, `cover/`), so no
   pre-existing file could be mistaken for evidence.
   `pdftoppm -png -r 300 -f 17 -l 22 <pdf> r300/p` and
   `pdftoppm -png -r 300 -f 8 -l 10 <pdf> r300/p`. The 300 dpi whole-page renders were
   read as images to find the sections named in the task and to confirm that the section
   whose printed heading matches — `• Memory content` — is on PDF page 19 and that the
   resembling neighbours (`• Codes for CW message contents` on PDF page 18,
   `• Band stacking register` on PDF page 20) are not it.
2. **Read, pass 1 — 400 dpi.** `pdftoppm -png -r 400 -f 19 -l 20 <pdf> r400/p`
   (3308 × 4678 px). Every pass-1 value was read from these.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`,
   `/opt/homebrew/bin/convert`) and was used throughout. Representative commands:
   - `magick r400/p-19.png -crop 2650x230+230+1030 +repage -resize 250% crops/row1_full.png`
   - `magick r400/p-19.png -crop 700x230+<x>+1030 +repage -resize 400% crops/row1_s<i>.png`
     (four windows, `x = 230 + i·670`) — cell-by-cell counting of the upper row.
   - `magick r400/p-19.png -crop 500x220+<x>+1280 +repage -resize 500% crops/row2_s<i>.png`
     (four windows, `x = 300 + i·470`) — cell-by-cell counting of the lower row.
   - `magick r400/p-19.png -crop 900x220+1350+1280 +repage -resize 350% crops/row2_mid.png`
     — to settle where the ㊺~52 / 53~68 notch falls.
   - `magick r400/p-19.png -crop 250x100+2200+1058 +repage -resize 900% crops/num_2122.png`
     and siblings at 600–900 % — index-numeral styling, one numeral at a time.
   - `magick r400/p-19.png -crop 1450x760+230+<y> +repage -resize 220% crops/legL_<n>.png`
     and `-crop 1420x680+1700+<y> … 240%` — the two legend columns, in bands.
   - `magick r400/p-20.png -crop 1420x850+230+<y> +repage -resize 260% crops/p20_tab2<n>.png`
     — the character tables.
   Every numeral, rule and glyph recorded was enlarged until it sat clear of its neighbours.
4. **`pdftotext` — never run.** It was not used at any point, for navigation or otherwise.
   Navigation was done entirely by reading 300 dpi whole-page renders.
5. **`tesseract` — available but not used.** `/opt/homebrew/bin/tesseract` is installed; no
   OCR was run on any crop. Every value in the CSV was read by eye from a render.
   (`pdfinfo` was run once on this same PDF for the page count and page size quoted in
   `## Source`; `ls` and `identify` were run only over the render directory this leg created,
   which contains nothing but renders of this PDF and the two output files.)
6. **Second independent pass — done.** After pass 1 was complete, PDF pages 19 and 20 were
   re-rendered at **600 dpi** (`pdftoppm -png -r 600 -f 19 -l 20 <pdf> r600/p`,
   4961 × 7016 px) into a separate directory, and every value was re-read from **different
   crop windows at different enlargements**: the data block was cut into two halves per row
   (`-crop 2100x330+330+1545 … 190%`, `-crop 2100x330+2280+1545`,
   `-crop 1700x330+400+1900 … 220%`, `-crop 1700x330+1950+1900`) instead of pass 1's four
   quarter-windows at 400 dpi; each legend column was cut into four bands at different
   boundaries (`-crop 2200x1000+330+2240 … 170%` and siblings) instead of pass 1's bands at
   400 dpi/220–240 %; the character tables were cut column-by-column
   (`-crop 1000x1300+350+2680 … 230%`, `-crop 1000x1300+1150+2680`) instead of pass 1's
   full-width bands. So the second raster differed in resolution (600 vs 400 dpi), in window
   geometry (halves and columns vs quarters and full-width bands) and in enlargement
   (170–230 % vs 220–500 %).

   **Second-pass result: no disagreements.** Every cell agreed — every cell count and
   shade in both rows of the data block, every bracket span and notch position, every index
   numeral and its styling, every legend label, every enumerated value in the ⑤, ⑭ and ⑮
   breakouts, every pointer and folio, and every character/ASCII pair in both tables on
   PDF page 20. In particular the two readings that most affect the record were re-confirmed
   independently: that ⑯~⑱ and ⑲~㉑ carry the **same** printed label (STOP 1), and that the
   upper-row index range ends `㊲ ~ ㊹` at 44 — read as `44`, not `43`, in both passes. No
   third render was needed anywhere.

**Arithmetic, checked on the render.** The block is drawn wrapped over two rows: 22 drawn
cells in the upper row (21 solid `X:X`, 1 dashed ellipsis) and 16 in the lower (12 solid,
4 dashed ellipses) — 38 drawn cells, of which 5 are ellipsis placeholders. Expanding each of
the five elided groups to its printed index range at one byte per index gives
2+2+1+5 = 10 through ⑩, 24 bytes for the upper row, 44 for the lower, and
2+2+1+5+2+1+1+1+3+3+3+1+3+8+8+8+16 = **68** in total, which equals the highest printed
index, `68`. Every printed index therefore lands on the byte position measured for it, and
no gap, overlap or shortfall exists. This is why no arithmetic STOP is recorded.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every index in
  both rows of the data block, in both breakout boxes, and in every legend entry is drawn
  in one and the same style: a thin outlined circle enclosing plain digits, unfilled,
  unbracketed, unbolded and without an underline. At 400 dpi and 250 % the lower-row
  numerals ㉕, ㉖, ㉘ and the upper-row ㉒, ㉔ appeared to carry a short underline; enlarged
  to 900 % (`crops/num_2122.png`, `crops/num_2224b.png`, `crops/num_25.png`) that mark
  resolved into the baseline feet of the digits `2`, `4`, `5` and `6`, and no numeral in the
  diagram carries an underline or any other distinguishing style. Confirmed again at 600 dpi.
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** No label
  anywhere in this section is rotated; all text runs horizontally. The question of extraction
  order did not arise, because no text layer was consulted: `pdftotext` was never run, and
  every position was read from the picture.
- **(c) Leader-line label order may be reversed — NOT ENCOUNTERED.** The three breakout
  boxes (⑤, ⑭, ⑮) are the only leader-line constructions here, and each leader was followed
  by eye from its arrowhead in the box to the list it lands on. ⑤: the left-hand nibble's
  leader runs down and right to `Fixed`; the right-hand nibble's leader runs down and right
  to the `0=OFF* … 3=★3` bracket, which sits further right — no crossing. ⑭: the left-hand
  nibble's leader lands on the nearer (left) bracket `0=Duplex OFF … 3=RPS`, the right-hand
  nibble's on the farther (right) bracket `0=OFF … 7=TONE(T)/TSQL(R)` — no crossing.
  ⑮: the left-hand nibble's leader runs straight down to the `0=…OFF / 1=…(DSQL) /
  2=…(CSQL)` list, the right-hand nibble's runs right to `Fixed`. Note that ⑤ and ⑮ are
  **mirror images** of one another — `Fixed` labels the left nibble in ⑤ and the right nibble
  in ⑮ — which matches the literal `0` drawn on the left in ⑤ and on the right in ⑮, so this
  is a genuine mirror in the data, not a reversed label.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED.**
  Both are recorded for every D1 row: the printed index in `field_index` and the measured
  byte position, counted left-to-right across the wrapped block on the render, in `notes`.
  They coincide for all seventeen fields (①→byte 1 … 53~68→bytes 53-68), and the two are
  recorded side by side rather than reconciled. The block that repeats indices ①–⑥ — the
  `To clear the memory channel contents on 1A 00:` list — is given its own diagram id `D2`
  and its own rows, transcribed from what is printed against it and not copied from D1; the
  differences that surfaced are listed under `## Observed disagreements`. D2 has no drawn
  diagram, so it has no measured position and no printed width; those cells are empty.

## STOP findings

1. **PDF page 19 (folio 18), right-hand legend column, the two lines immediately below the
   ⑮ breakout box.** The page prints, on consecutive lines:
   `⑯~⑱: Repeater tone frequency setting`
   `⑲~㉑: Repeater tone frequency setting`
   Two distinct three-byte fields at distinct measured positions (bytes 16-18 and bytes
   19-21) are given **word-for-word identical labels**, while the single pointer they share
   on the next line names *two* settings: `ⓘ See “Repeater tone/tone squelch frequency
   setting.” (p. 23)`. Either the second label contradicts the pointer, or the record
   contains the same setting twice; the page does not say which, and nothing on the pages
   given to this leg resolves it. It stops because a printed label contradicts other printed
   text on the same page. Read at 400 dpi and again at 600 dpi from a different crop
   window (`crops/legR_1621.png`, `pass2/legR_1.png`): both readings are identical, and
   there is no difference in wording, spacing, capitalisation or punctuation between the two
   lines. **Transcribed exactly as seen** in both rows — the second label is *not* repaired to
   "Tone squelch frequency setting" or anything else — with `STOP 1` in each row's `notes`.

## Observed disagreements

Recorded as printed; not resolved.

1. **`(8 characters, fixed.)` vs `(8 characters, fixed)`.** The three call-sign fields are a
   repeating block of three identical 8-byte structures, but their labels are not identical:
   ㉙~㊱ reads `(8 characters, fixed)`, ㊲~㊹ reads `(8 characters, fixed.)` — with a full
   stop before the closing bracket — and ㊺~52 reads `(8 characters, fixed)` again. Only the
   middle one has the stop. Each label was transcribed independently from its own printed
   line, in both passes.
2. **The diagram splits ⑪ and ⑫; the legend joins them.** In the data block ⑪ and ⑫ are drawn
   as two separate circled numerals over two separate cells with no bracket joining them —
   exactly like the genuinely single-byte ⑬, ⑭ and ⑮ — whereas ①,② and ③,④ *are* joined by
   a drawn bracket. The legend nevertheless prints `⑪, ⑫: Operating mode setting` as one
   two-byte field. The CSV follows the legend and records the diagram's treatment in `notes`.
3. **Cell shading is used but never explained.** Cells are drawn alternately white and
   grey-shaded — grey for ③④, the ⑥~⑩ group, ⑫, ⑭, ⑯~⑱, ㉒~㉔, ㉖~㉘ and ㊲~㊹; white for
   ①②, ⑤, ⑪, ⑬, ⑮, ⑲~㉑, ㉕, ㉙~㊱, ㊺~52 and 53~68. Nothing on the page states what the
   shading means, and no key is printed. It is recorded per row in `notes` and nothing is
   inferred from it.
4. **The D2 block restates ①–⑤ with different labels and different permitted values.**
   `③, ④` is `Memory channel numbers` in D1 but the singular `Memory channel` in D2;
   `①, ②` admits `01 00: Call channel group` in D1 but D2 says `You cannot specify group
   “01 00” (Call channel group)`; and ⑤, which in D1 takes `0`–`3`, takes `“FF,”` in D2.
   These are the clear-command semantics of the same bytes, printed without any statement
   that the two lists are alternatives. Both are transcribed, independently, as printed.
5. **`Duplex offset frequency setting` vs `Duplex Offset frequency setting`.** The ㉖~㉘
   label lower-cases *offset*; the pointer on the very next line capitalises it.
6. **Two dashes that a raster cannot separate.** In the ⑭ breakout, `1=Duplex−` is drawn
   with a mid-height bar noticeably wider than the hyphen in the note below it,
   `Duplex (+, -)`; likewise the character printed against ASCII code `2D` on PDF page 20 is
   drawn as a wide mid-height bar, not a hyphen. A rendered image cannot distinguish
   U+2013, U+2212 and a wide-drawn U+002D, so both are transcribed as a plain hyphen and
   the difference in drawn width is recorded here rather than guessed at.
7. **Typographic glyphs standing for straight ASCII codes.** On PDF page 20 the characters
   printed against codes `22`, `27` and `60` are directional/typographic forms — a right
   double quote for `22`, a right single quote for `27`, a grave-like stroke for `60` —
   rather than the straight `"`, `'` and `` ` `` those codes denote. Transcribed as drawn.
8. **No space character in the table.** The two character tables on PDF page 20 print no
   entry for a space, though the field they serve is a fixed-length 16-character name.
9. **Tilde spacing is inconsistent between the diagram and the legend.** The diagram's
   brackets are set with spaces (`⑲ ~ ㉑`, `㊲ ~ ㊹`, `53 ~ 68`) while the legend sets the same
   ranges tight (`⑲~㉑`, `㊲~㊹`, `53~68`). `field_index` follows the legend, which is where
   the labels are printed.
10. **A transcription limit, stated so it is not mistaken for the page.** Every index in this
    section is printed as a circled numeral, but Unicode has no circled forms above 50. The
    indices `52`, `53` and `68` are therefore written as plain digits in `field_index`
    (`㊺~52`, `53~68`); the circle is present on the page in all three cases and is recorded
    in those rows' `notes`.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual,
transcription, source file, generated artefact or web resource was opened, and no directory
was listed.
