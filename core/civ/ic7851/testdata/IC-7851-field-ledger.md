# IC-7851 memory-record field ledger — quarantine leg L

Companion to `IC-7851-field-ledger.csv`.

## Diagram identities

- **D1** — printed caption verbatim: `• Memory content setting` / `Command: 1A 00`
  (two lines, immediately under the sub-heading `◇ Data content description (continued)`).
  Position: top of PDF page 263, spanning the full width of the two-column text
  area; a single horizontal strip of `X` byte cells with a band of circled
  callouts above it, followed by the numbered field explanations printed in the
  two columns beneath it (left column: indices 1 to 10; right column: indices 11
  to 27). The strip and its numbered explanations are one diagram: the callout
  band prints the indices, the explanations print the labels against them.
- **D2** — the sub-diagram inside D1's index-11 explanation entry. It has no
  caption of its own; the caption line above it reads `⑪ Data mode and tone type
  settings`. Position: right-hand column of PDF page 263, upper third, directly
  below that heading — a single box divided by a dashed vertical rule into two
  `X` nibbles, a circled `11` printed above it, and two arrowed leader lines
  rising from beneath it to two label lines on the right.

Diagrams present on PDF page 263 but **not** transcribed, and why: the
`• Main or Sub band's frequency settings` / `Command : 25` diagram (mid page,
left column) carries no numbered indices at all; the `• Main or Sub band's
operating mode and filter settings` / `Command : 26` diagram and the three-column
table below it (lower half of the page) do carry circled indices 1, 2 and 3, but
they are not memory-record data blocks — they belong to two separate commands
that set the live Main/Sub band, not a memory record. The brief scopes this leg
to "every memory-record data-block diagram", so they are excluded. This is the
one scoping judgement call in this leg and it is recorded here so it can be
overturned without re-reading the page.

## Source

- Document title as printed on the cover (PDF page 1, centred): `THE
  TRANSCEIVERS` / `IC-7850` / `IC-7851` / `Instruction Manual`.
- Revision code as printed: `A7205H-1EX-3`, printed in the bottom right corner
  of the cover (PDF page 1), on the line above `Printed in Japan` and
  `© 2015–2018 Icom Inc.`
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7851_civ_IM_3.pdf`
- Page count: 283 PDF pages.

## Extent

| PDF page | Printed folio | Rendered | Read | Contribution |
|---|---|---|---|---|
| 1 | (none printed) | yes, 150 dpi and 600 dpi | yes | Cover only: document title and revision code for `## Source`. No transcribed value comes from it. |
| 263 | `18-14` (bottom centre) | yes, 300, 400 and 600 dpi | yes | The whole of the transcription. |

The folio on PDF page 263 reads `18-14`, consistent with the brief's rule that
the CI-V chapter is §18 and the PDF page is 249 + n.

Judgement call: the brief names PDF page 263 only, but the required `## Source`
section asks for the cover title and the revision code, which are not printed on
page 263. PDF page 1 of the same PDF was therefore rendered and read for those
two facts alone. PDF page 283 was rendered at the same time on the assumption
the revision code might sit on the back cover; the cover carried it, so that
render was deleted unread and contributed nothing.

Where the transcribed material begins and ends, all on PDF page 263:

- Immediately before it, at the head of the page: the running chapter head `18
  CONTROL COMMAND`, then the sub-heading `◇ Data content description
  (continued)`.
- The material begins with the caption `• Memory content setting` /
  `Command: 1A 00` and its byte-cell strip.
- It ends with the last line of the index-18-to-27 explanation, `and network
  radio name contents."`, at the foot of the right-hand column.
- Immediately after it, in the left-hand column below the index-9,-10 entry:
  the caption `• Main or Sub band's frequency settings` / `Command : 25`, which
  begins the next command and is outside this leg's scope.

Nothing in D1 or D2 runs onto an adjacent page; the memory-record block opens
and closes on PDF page 263.

## Method

Working directory, created fresh for this leg:
`/private/tmp/claude-501/-Users-stuart-Documents-working-coding-ft710-programmer-nosync/b1f63348-8eaa-4174-bb05-d1f10e3b04fb/scratchpad/legs-out/ic7851/L`
(sub-directories `r300/`, `r400/`, `r600/`, `crops/`, `crops2/`, `cover/`). No
pre-existing file was present in it.

**Step 1 — locate, 300 dpi.**

    pdftoppm -png -r 300 -f 263 -l 263 <pdf> r300/p

`r300/p-263.png` was read as an image. The printed heading `◇ Data content
description (continued)` and the caption `• Memory content setting` /
`Command: 1A 00` were matched by eye; the folio `18-14` at the foot confirmed
the page. No other page was searched.

**Step 2 — read, 400 dpi.**

    pdftoppm -png -r 400 -f 263 -l 263 <pdf> r400/p     # 3308 x 4678

Every first-pass value was read from `r400/p-263.png` and crops of it.

**Step 3 — crop and enlarge.** ImageMagick was available (`/opt/homebrew/bin/magick`)
and used throughout. First-pass crops, all `+repage`d:

    magick r400/p-263.png -crop 1200x320+560+680  +repage -resize 250% crops/d1_band_left.png
    magick r400/p-263.png -crop 1200x320+1700+680 +repage -resize 250% crops/d1_band_right.png
    magick r400/p-263.png -crop 1300x1400+180+930 +repage -resize 200% crops/legend_L1.png
    magick r400/p-263.png -crop 1300x300+180+1980 +repage -resize 300% crops/legend_L_4to8.png
    magick r400/p-263.png -crop 1300x380+1700+980 +repage -resize 300% crops/d2_nibble.png
    magick r400/p-263.png -crop 1450x600+1700+1370 +repage -resize 250% crops/legend_R.png

At these enlargements every circled numeral, every range separator and every
bracket sat clear of its neighbours.

**Step 4 — `pdftotext`.** `pdftotext` was **not run at all**, in any form, on
this or any other file. The page was located from the 300 dpi render alone.

**Step 5 — `tesseract`.** Available (`/opt/homebrew/bin/tesseract`) and used
twice, as a reading aid only, on `crops2/d2_leaders_p2.png` and
`crops2/lg_R1.png`. It returned `0: OFF, 1: TONE, 2: TSQL` /
`0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3` and `(2~d4 Repeater tone frequency
setting` / `15~17) Tone squelch frequency setting`. It mangled the circled
numerals, as expected. Every value it returned was confirmed by eye on the
render before being recorded, and no value rests on OCR: the circled indices
were read from the enlargements, not from tesseract.

Scope note on the attestation below: the only directories listed during this leg
were this leg's own output sub-directories, immediately after creating a render
or a crop in them, to confirm the file had been written. No directory outside
this leg's output directory was listed, searched or browsed, and the repository
was not touched beyond opening the one named PDF by its absolute path.

**Step 6 — second independent pass.** The whole ledger was re-read from a
different raster: PDF page 263 re-rendered at **600 dpi** (`r600/p-263.png`,
4961 x 7016) and cropped through **different windows** from the first pass — the
callout band split into three overlapping thirds rather than two halves, and the
explanation headings cropped as individual one-line strips rather than as whole
column blocks:

    pdftoppm -png -r 600 -f 263 -l 263 <pdf> r600/p
    magick r600/p-263.png -crop 1200x340+840+1000  +repage -resize 200% crops2/b_A.png
    magick r600/p-263.png -crop 1250x340+1980+1000 +repage -resize 200% crops2/b_B.png
    magick r600/p-263.png -crop 1400x340+3050+1000 +repage -resize 200% crops2/b_C.png
    magick r600/p-263.png -crop 1900x120+270+1400  +repage -resize 200% crops2/lg_1.png
    magick r600/p-263.png -crop 1900x120+270+2440  +repage -resize 200% crops2/lg_3.png
    magick r600/p-263.png -crop 1900x130+270+2960  +repage -resize 200% crops2/lg_48.png
    magick r600/p-263.png -crop 1900x130+270+3250  +repage -resize 200% crops2/lg_910.png
    magick r600/p-263.png -crop 2100x260+2460+2050 +repage -resize 200% crops2/lg_R1.png
    magick r600/p-263.png -crop 2100x160+2460+2495 +repage -resize 200% crops2/lg_R2b.png
    magick r600/p-263.png -crop 1300x300+2560+1470 +repage -resize 260% crops2/d2_nibble_p2.png
    magick r600/p-263.png -crop 2200x330+2500+1680 +repage -resize 190% crops2/d2_leaders_p2.png

Four of the second-pass left-column strips were then appended into one image for
reading, and three of the right-column strips likewise:

    magick lg_1.png lg_3.png lg_48.png lg_910.png -background white -append stack_left.png
    magick lg_11.png lg_1217.png lg_1827.png     -background white -append stack_right.png

`stack_right.png` cropped the left edge of the circled numerals, so the
right-column headings were re-cropped with a wider left margin
(`crops2/lg_R1.png`, `crops2/lg_R2b.png`) and read from those.

Both passes were done. **No cell disagreed between the two passes.** All nine
CSV rows — the eight D1 indices, their eight labels and their styles, and the D2
index — came out identical, and the second pass independently reproduced the two
findings recorded below as STOP 1 and STOP 2 and the crossed-leader reading in
D2. No third render was needed.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram** — NOT ENCOUNTERED. Every
  index on this page, in the D1 callout band, in the D1 explanation headings and
  in the D2 sub-diagram, is drawn one way: an Arabic numeral inside a thin open
  circle, black on white, no fill, no bracket, no bold variant. All nine rows
  are therefore `circled`. Checked again at 600 dpi in the second pass; the
  circled `27` in the parenthetical note `(instead of the data ③ to ㉗)` is drawn
  the same way as the rest. What does vary is not the numeral style but the
  separator between the two numerals of a range — see STOP 1 — which is recorded
  as a STOP rather than as an `index_style` difference.
- **(b) Diagrams may be vector groups with rotated labels** — NOT ENCOUNTERED,
  in the sense that no label on page 263 is rotated; every label runs
  horizontally left to right. The point is moot here in any case because
  `pdftotext` was never run: all positions were read from the renders.
- **(c) Leader-line label order may be reversed** — ENCOUNTERED, in D2. The two
  arrowed leaders rising from the two-nibble box cross each other. Read by eye
  from arrowhead down the stem to the horizontal run and along to the text: the
  **left** nibble's arrow drops to the **lower** of the two horizontal runs and
  lands on `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`; the **right** nibble's
  arrow drops to the **upper** run and lands on `0: OFF, 1: TONE, 2: TSQL`. So
  the top-to-bottom order of the two label lines runs opposite to the
  left-to-right order of the nibbles they point at. Confirmed independently at
  400 dpi (`crops/d2_nibble.png`) and 600 dpi (`crops2/d2_leaders_p2.png`).
  Neither nibble carries a printed index, so this produces no CSV row of its
  own; it is recorded in the D2 row's `notes`.
- **(d) A printed index may differ from a field's measured position** — CANNOT
  DETERMINE. Index `11` is printed twice on this page — once in the D1 callout
  band and once in the D2 sub-diagram — which is the repeating-block condition
  hazard (d) describes, and it is recorded as STOP 2. But hazard (d) asks for
  the measured byte position alongside the printed index, and the task
  statement of this leg expressly forbids that ("no widths, no byte positions,
  no encodings, no meanings"). The task statement was followed and no position
  was measured, so whether printed index and measured position agree cannot be
  determined from this ledger. The conflict between the two instructions is
  recorded here rather than resolved.

## STOP findings

1. **PDF page 263, D1 — the same range index is printed two different ways.**
   Visual anchor: the callout band above the byte-cell strip, top of the page,
   versus the numbered explanation headings in the two columns below it. What is
   printed: in the band the four ranged callouts read `④–⑧`, `⑫–⑭`, `⑮–⑰` and
   `⑱–㉗`, with a straight, roughly horizontal, slightly bold **en dash** between
   the two circled numerals. In the explanation headings the same four indices
   read `④~⑧`, `⑫~⑭`, `⑮~⑰` and `⑱~㉗`, with a **wavy tilde**. The two circled
   numerals are identical in both places; only the separator differs. Why it
   stops: the brief requires `field_index` verbatim, forbids normalising a range
   dash or tilde, and makes `field_index` a join key — and this page offers two
   verbatim forms for each of these four indices, so no single verbatim
   transcription can be right for both places. Transcribed into the CSV: the
   tilde form, `4~8`, `12~14`, `15~17`, `18~27`, in the four affected rows, each
   carrying `STOP 1` in `notes`. The tilde form was chosen because it is the form
   printed against the label, and the brief's own worked example of this
   convention is `4~8`; the en dash form is recorded here so the choice is
   visible and reversible. The two-nibble pair indices are unaffected: `①, ②` and
   `⑨, ⑩` are printed with a comma and a space in both places, identically.
2. **PDF page 263 — index `11` is printed twice, in two diagrams.** Visual
   anchor: the sixth callout of the D1 band, a bare circled `11` above a single
   shaded byte cell between the `⑨, ⑩` bracket and the `⑫–⑭` bracket; and again
   in the D2 sub-diagram in the right-hand column, a circled `11` printed above
   the two-nibble box, one line below the heading `⑪ Data mode and tone type
   settings`. What is printed: `⑪` in both places, in the same circled style.
   Why it stops: the STOP rules make a repeated index a discontinuity to be
   recorded, and the ledger accordingly carries two rows whose `field_index` is
   `11`, distinguished only by `diagram_id`. Transcribed into the CSV as printed:
   `D1,11,circled,Data mode and tone type settings` and `D2,11,circled,` (empty
   label), both carrying `STOP 2` in `notes`. It is not treated as an error: the
   repetition reads on the page as the sub-diagram expanding field 11 of the
   record, but that reading is an interpretation and is not recorded as fact.

## Observed disagreements

- A **third** notation for a range appears on the same page, in the shaded note
  in the left-hand column: `(instead of the data ③ to ㉗)` — the word `to`
  between two circled numerals, where the band uses an en dash and the
  explanation headings use a tilde. Recorded as printed; not reconciled with
  STOP 1. This one is prose rather than an index label, which is why it is here
  and not a STOP.
- The D1 explanation headings are not spaced uniformly. `⑫~⑭ Repeater tone
  frequency setting` and `⑮~⑰ Tone squelch frequency setting` are printed on
  consecutive lines with no blank line between them and share a single following
  line, `See "• Repeater tone/tone squelch settings."`, whereas every other
  explanation entry on the page is separated from its neighbour by a blank line
  and carries its own `See` line. Two indices therefore share one cross-reference.
  Recorded as printed; each still has its own label line, so each still has its
  own CSV row.
- The D1 byte-cell strip carries two dotted-outline ellipsis cells, one inside
  the `④–⑧` span and one inside the `⑱–㉗` span, drawn as a cell containing `...`
  with a dashed border rather than the `X`-pair of every other cell. No index is
  printed against either, so neither produces a row. Noted because a reader
  counting cells against indices will meet them.
- Cell shading in the strip alternates between grey and white but the runs of
  shading do not line up with the callout brackets — for example the `③` cell is
  white while the `①, ②` pair to its left is grey, and the block shaded under the
  `⑮–⑰` bracket extends one cell to the left of that bracket's left edge.
  Recorded as seen; no meaning is inferred, and no byte positions were measured.
- The `③ Select memory setting` value list prints `01 : ★1`, `02 : ★2`,
  `03 : ★3` using a solid five-pointed star glyph. Recorded here because the
  glyph is unusual; it is a value list, not a field label, so it appears in no
  CSV row.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.
