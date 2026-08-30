# IC-7100 geometry witness — quarantine leg W

Companion to `IC-7100-geometry-witness.csv`.

The seven sections the brief requires appear below in the order it gives them.
`## Position arithmetic, per diagram` — also required — is placed between
`## Method` and `## Hazards encountered`, because it is the working behind the
CSV's numbers and reads naturally there. No required section has been reordered
relative to another.

---

## Source

- **Document title as printed on the cover** (PDF page 1): the cover prints the
  Icom logo, then a black band reading **FULL MANUAL**, then, in the lower left
  block, **HF/VHF/UHF ALL MODE TRANSCEIVER** above the large product name
  **IC-7100**, and **Icom Inc.** at the foot. The right-hand column of the cover
  lists the chapter tabs, of which **20 CONTROL COMMAND** is the chapter this leg
  reads.
- **Revision code as printed, and where**: **`A7085-2EX-5`**, printed at the
  bottom-left corner of the back cover (**PDF page 387**), directly above
  **`© 2013–2021 Icom Inc.      May 2021`**. No revision code is printed on the
  front cover.
- **File path**:
  `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7100_civ_FM_5.pdf`
- **Page count**: 387 PDF pages (reported by `pdfinfo` on this same PDF; the back
  cover is PDF page 387, consistent with that count).

---

## Extent

| PDF page | printed folio | dpi rendered | what it contributed |
|---|---|---|---|
| 1 | (no folio printed) | 150 | Cover title, for `## Source` only. No recorded value. |
| 375 | **20-16** | 300, 400, 600 | **Every value in the CSV.** |
| 387 | (no folio printed) | 150 | Back-cover revision code, for `## Source` only. No recorded value. |

The Tier 4b relation holds on this page: the folio reads `20-16` and
359 + 16 = 375, the PDF page rendered.

**Where the transcribed material begins.** Reading down PDF page 375: a blue
`Previous view` navigation button at the very top (a PDF widget, not page
content); the chapter head `20  CONTROL COMMAND`; a solid black section bar
reading `Remote jack (CI-V) information`; the line `◇ Data content description
(Continued)`; the bold heading `• Memory content setting`; and the line
`Command: 1A 00`. The **upper bar** of the data-block diagram is printed
immediately below `Command: 1A 00`.

**Where it ends.** The **lower bar** is printed immediately below the upper bar,
separated only by white space. Immediately below the lower bar the page breaks
into two text columns: the left column opens `①Bank number`, the right column
opens `⑭ Digital squelch setting`. The three single-byte breakout boxes
(`④`, `⑬`, `⑭`) sit inside those columns. The page foot prints the folio
`20-16`.

Nothing on the page belongs to any other section: the only black section bar on
the page is `Remote jack (CI-V) information`, and the only field-diagram heading
is `• Memory content setting`.

### Diagram identifiers, by printed caption verbatim

- **D1** — caption printed verbatim: **`• Memory content setting`**, with
  **`Command: 1A 00`** on the line beneath it. D1 is **one** diagram printed as
  **two bars**: an upper bar of 25 cell-widths carrying indices `①` … `㉕〜㉗`,
  and a lower bar of 15 cell-widths carrying `㉘〜㉟` … `(52)〜(60)`. There is no
  second caption, no rule and no heading between the two bars; the lower bar's
  first printed index (`㉘`) follows the upper bar's last (`㉗`). The lower bar is
  therefore counted as a continuation of the upper bar, and D1's byte 1 is the
  upper bar's leftmost cell. **This is a judgement call and is flagged as such;
  it is the only structural inference in this leg.**
- **D2** — caption printed verbatim: **`④Split and Select memory settings`**
  (left column). One box, two nibbles.
- **D3** — caption printed verbatim: **`⑬ Duplex and Tone settings`**
  (left column). One box, two nibbles.
- **D4** — caption printed verbatim: **`⑭ Digital squelch setting`**
  (right column). One box, two nibbles, the right one printed `0`.

D2–D4 are included because each is a numbered field of the memory record drawn
as its own data block at nibble granularity, with its index printed above it.
Their `first_byte`/`last_byte` are measured **within their own one-box block**,
per the brief's rule that a data block's first byte is 1; the CSV `notes` say so
on every one of those three rows.

### How the diagram labels bytes and nibbles

- The diagram prints **no byte-position numerals and no byte ruler**. There is no
  numbered band. The circled numerals are **field indices printed above brackets**,
  not printed byte addresses. **All byte positions in the CSV were therefore
  counted as cells from the diagram's own first cell**, as the brief directs.
- **Nibbles are not labelled at all** — no `1`/`2`, no `H`/`L`, no `MSB`/`LSB`.
  Each byte cell is divided by a **dotted vertical rule at its exact mid-width**,
  splitting it into two half-cells each printing `X` (the `⑭` cell prints `X` then
  `0`). The nibble numbering in the CSV is purely the brief's recording
  convention: nibble 1 = the left half as printed, nibble 2 = the right half.
- The **ellipsis cells carry no nibble divider** (see `## STOP findings` 1 and 18).

### Recording convention for index glyphs that Unicode cannot represent

The page prints circled numerals up to 67. Unicode has outline-circled forms only
to 50 and filled/reversed-circled forms only to 20. Where a glyph exists it is
used verbatim in `field_index`. Where it does not:

- an **outline-circled** numeral is written **`(n)`** — `(51)`, `(52)`, `(60)`, `(67)`;
- a **filled/reversed-circled** numeral is written **`[n]`** — `[51]`.

This keeps `⑤〜(51)` (outline) and `❺〜[51]` (filled) distinct, which matters
here: they are two different printed labels for two different blocks. Every row's
`notes` states the drawn style in words regardless.

---

## Method — page images only

Every value recorded in the CSV was read from a rendered page image of this PDF.

**1. Locate (300 dpi).** Fresh directory, created for this leg:

```
mkdir -p .../legs-out/ic7100/W/r300
pdftoppm -png -r 300 -f 375 -l 375 <pdf> .../W/r300/p
```

Read `r300/p-375.png` as an image. Found the black section bar
`Remote jack (CI-V) information`, the heading `• Memory content setting`,
`Command: 1A 00`, and the two bars of D1 beneath it, plus the three breakout
boxes in the body columns.

**2. Read (400 dpi).** Every first-pass value came from this raster:

```
pdftoppm -png -r 400 -f 375 -l 375 <pdf> .../W/r400/p     # 3308 x 4678
```

**3. Crop and enlarge.** **ImageMagick was available and used** (`magick`,
`/opt/homebrew/bin/magick`; `convert` also present). Crops written to
`.../W/crops/`:

```
magick r400/p-375.png -crop 3308x300+0+900   +repage crops/band1_full.png
magick r400/p-375.png -crop 3308x280+0+1110  +repage crops/band2_full.png
magick r400/p-375.png -crop 830x240+200+920  +repage -resize 250% crops/b1_q0.png
magick r400/p-375.png -crop 830x240+990+920  +repage -resize 250% crops/b1_q1.png
magick r400/p-375.png -crop 830x240+1780+920 +repage -resize 250% crops/b1_q2.png
magick r400/p-375.png -crop 830x240+2570+920 +repage -resize 250% crops/b1_q3.png
magick r400/p-375.png -crop 1000x180+180+1150  +repage -resize 220% crops/b2_a.png
magick r400/p-375.png -crop 1000x180+1080+1150 +repage -resize 220% crops/b2_b.png
magick r400/p-375.png -crop 260x140+1420+1010  +repage -resize 500% crops/cell12_b1.png
magick r400/p-375.png -crop 420x80+1050+1160   +repage -resize 600% crops/lbl_4451.png
magick r400/p-375.png -crop 420x80+1290+1160   +repage -resize 600% crops/lbl_filled.png
magick r400/p-375.png -crop 420x80+1540+1160   +repage -resize 600% crops/lbl_5260.png
magick r400/p-375.png -crop 1150x600+200+2480  +repage -resize 180% crops/box4.png
magick r400/p-375.png -crop 1150x600+400+3900  +repage -resize 180% crops/box13.png
magick r400/p-375.png -crop 1400x600+1600+1420 +repage -resize 180% crops/box14.png
magick r400/p-375.png -crop 1600x420+1600+3160 +repage -resize 180% crops/rc_names.png
magick r400/p-375.png -crop 1650x760+1590+3500 +repage -resize 160% crops/rc_clear.png
magick r400/p-375.png -crop 1650x620+1590+4180 +repage -resize 160% crops/rc_note.png
```

At 500–600% every numeral, rule, dotted divider and shading boundary sits clear
of its neighbours; the outline-circled `51` and the filled `[51]` are
unmistakably distinct at 600%.

**Cell boundaries were also located by pixel measurement on the same renders**,
as a check on the eye and never as a substitute for it. A one-pixel-high scanline
was taken across each bar just below its top border and the dark runs listed;
every boundary so found was then confirmed by eye on an enlarged crop. On the
400 dpi raster the upper bar's box row runs y = 1017…1097 and its solid vertical
rules stand at x = 236, 346, 457, 567, 677, 788, 898, 1008, 1118, 1229, 1339,
1449, 1559, 1670, 1780, 1890, 2000, 2111, 2221, 2331, 2441, 2552, 2662, 2772,
2882, 2993 — **26 rules, hence 25 cell-widths, pitch 110.3 px**. The lower bar's
box row runs y = 1236…1316 with rules at x = 236, 346, 457, 567, 677, 788, 898,
1008, 1118, 1229, then a 337 px unruled gap, then 1566, 1676, 1786, 1896 —
**15 cell-widths on the identical pitch**, of which slots 10–12 are the one
undivided dashed box.

**4. `pdftotext`.** **`pdftotext -layout` was NOT run — not on this PDF, not on
anything.** No text-layer extraction of any kind was used at any point. The
Tier 4b warning that the receive and transmit blocks extract as one identical
token was taken as a reason to avoid the text layer entirely rather than as an
instruction to inspect it.

**5. `tesseract`.** Available on this machine but **not used**. Every numeral was
read by eye off the enlarged crops. No OCR value appears anywhere in this leg.

**6. Second independent pass — done.** After the 400 dpi pass was complete, the
page was re-rendered at **600 dpi** (`pdftoppm -png -r 600`, 4961 × 7016 — a
different raster at a different scale) and every value was re-derived from it:
fresh crops at a different enlargement (150%, and windows at different offsets
and widths from the first pass's), and fresh scanlines at the 600 dpi row
coordinates. The second pass re-read the cell counts, the shading runs, the
nibble dividers, the bracket leg positions and every index numeral.

Second-pass results, stated in 600 dpi pixels: upper bar solid rules at
355 … 4490 in 26 positions, pitch 165.4 px → **25 cell-widths**; nibble dividers
present at every cell midpoint **except** the cell between rules 1183 and 1348
(the 6th) → **one undivided ellipsis cell**; bracket leg tips at 846, 1514, 1838,
2831, 3325, 3822, 4485. Lower bar solid rules at 356 … 1844 then 2350 … 2844 →
**15 cell-widths** with slots 10–12 unruled; nibble dividers absent at slots 2,
5, 8, 10–12 and 14; bracket leg tips at 853, 1348, 1845, 2344, 2840. Multiplying
the first pass's 400 dpi figures by 1.5 reproduces every one of these to within
a pixel.

**Cells where the two passes disagreed: none.** Cell counts, ellipsis-cell
positions, nibble-divider presence, bracket spans, shading runs and all eighteen
D1 index labels agreed exactly. No third render was needed.

---

## Position arithmetic, per diagram

Positions are **counted cell-widths** on the printed grid, 1-based from the
diagram's own first cell, exactly as the CSV records them. The printed index is
shown beside them and **is not reconciled with them**.

### D1 — `• Memory content setting` / `Command: 1A 00`

Upper bar: left border, then 25 cell-widths, then right border. Lower bar: left
border, then 15 cell-widths, then right border. Counting continues across the bar
break (see the judgement call flagged in `## Extent`).

| # | printed index | measured start | measured extent (cells) | measured end | next starts | printed range spans | agrees? |
|---|---|---|---|---|---|---|---|
| 1 | `①` | 1 | 1 | 1 | 2 | 1 | yes |
| 2 | `②、③` | 2 | 2 | 3 | 4 | 2 | yes |
| 3 | `④` | 4 | 1 | 4 | 5 | 1 | yes |
| 4 | `⑤〜⑨` | 5 | 3 | 7 | 8 | **5** | **no — STOP 1** |
| 5 | `⑩、⑪` | 8 | 2 | 9 | 10 | 2 | **no — STOP 2** (extent agrees, position does not) |
| 6 | `⑫` | 10 | 1 | 10 | 11 | 1 | **no — STOP 3** |
| 7 | `⑬` | 11 | 1 | 11 | 12 | 1 | **no — STOP 4** |
| 8 | `⑭` | 12 | 1 | 12 | 13 | 1 | **no — STOP 5** |
| 9 | `⑮〜⑰` | 13 | 3 | 15 | 16 | 3 | **no — STOP 6** |
| 10 | `⑱〜⑳` | 16 | 3 | 18 | 19 | 3 | **no — STOP 7** |
| 11 | `㉑〜㉓` | 19 | 3 | 21 | 22 | 3 | **no — STOP 8** |
| 12 | `㉔` | 22 | 1 | 22 | 23 | 1 | **no — STOP 9** |
| 13 | `㉕〜㉗` | 23 | 3 | 25 | (upper bar ends) | 3 | **no — STOP 10** |
| 14 | `㉘〜㉟` | 26 | 3 | 28 | 29 | **8** | **no — STOP 11** |
| 15 | `㊱〜㊸` | 29 | 3 | 31 | 32 | **8** | **no — STOP 12** |
| 16 | `㊹〜(51)` | 32 | 3 | 34 | 35 | **8** | **no — STOP 13** |
| 17 | `❺〜[51]` | 35 | 3 | 37 | 38 | **47** | **no — STOP 14, 17, 18** |
| 18 | `(52)〜(60)` | 38 | 3 | 40 | (lower bar ends) | **9** | **no — STOP 15, 16** |

**Sums, checkable without the images.**

- Upper bar measured extents: 1 + 2 + 1 + 3 + 2 + 1 + 1 + 1 + 3 + 3 + 3 + 1 + 3
  = **25**, which is the number of cell-widths drawn between the upper bar's left
  and right borders. No gap and no overlap: every group starts on the cell-width
  immediately after the previous group's last.
- Lower bar measured extents: 3 + 3 + 3 + 3 + 3 = **15**, which is the number of
  cell-widths drawn between the lower bar's left and right borders. Again no gap
  and no overlap.
- **Measured total for D1: 25 + 15 = 40 cell-widths.**
- Printed indices, taken as an ascending outline-circled run and ignoring the
  filled restatement: `①` … `(60)` — **60 positions** on the diagram bar's own
  labelling; the body text instead carries the run to `(67)` — **67 positions**
  (STOP 16).
- **40 ≠ 60 and 40 ≠ 67.** The measured running total and the printed numbering
  disagree. Both are recorded above. **Neither is resolved.**

The shortfall is visible on the page rather than inferred: six of D1's forty
drawn cell-widths carry no `X:X` at all but a row of dots inside a dashed
outline — upper bar cell-width 6, lower bar cell-widths 27, 30, 33 and 39, and
the three-wide undivided box at 35–37. Those dashed cells are the diagram's own
mark for material it has not drawn. **How many bytes each dashed cell stands for
is not printed anywhere on the diagram, and no attempt has been made here to
work it out, from prose widths or otherwise.**

### D2 — `④Split and Select memory settings`

One box, one cell-width. Start 1, extent 1, end 1. Two half-cells split by a
dotted mid-rule: nibble 1 = 1, nibble 2 = 2. Nothing follows it inside this
diagram.

### D3 — `⑬ Duplex and Tone settings`

One box, one cell-width. Start 1, extent 1, end 1. Nibbles 1 and 2 as above.

### D4 — `⑭ Digital squelch setting`

One box, one cell-width. Start 1, extent 1, end 1. Nibbles 1 and 2 as above;
nibble 2 prints a fixed `0` rather than `X`.

---

## Hazards encountered

**(a) Numeral styling may vary within one diagram — `ENCOUNTERED`.** D1 draws its
index numerals in **two** distinct styles. Every index in the upper bar and the
first three groups of the lower bar is an **outline-circled** numeral (a thin
black ring round a black numeral on white). The fourth group of the lower bar is
labelled `❺〜[51]` in **filled / reversed-circled** style — white digits knocked
out of a solid black disc — confirmed at 600% enlargement on
`crops/lbl_4451.png` and again on the 600 dpi second pass. The same two styles
recur in the right-column NOTE (`⑤–(51)` outline, `❺–[51]` filled). The two
styles have **not** been normalised in the CSV, and no meaning has been inferred
for either: the `field_index` column keeps them apart by the `(n)` / `[n]`
convention set out in `## Extent`, and every row's `notes` names the style in
words.

**(b) Vector groups with rotated labels — `NOT ENCOUNTERED`.** No label anywhere
on PDF page 375 is rotated. Every circled index sits upright above its bracket or
its box; every legend line runs left to right. The second half of the hazard —
that the text layer extracts in an order unrelated to the page — was not tested,
because `pdftotext` was not run at any point in this leg; every position here
comes from the picture regardless, which is the hazard's own remedy.

**(c) Leader-line label order may be reversed — `ENCOUNTERED`.** In **D2**
(`④Split and Select memory settings`) two arrows rise from the box. Followed by
eye from label to cell: the arrow standing over the **left** nibble runs down and
across to the **lower** legend, `0: Split OFF, 1: Split ON`; the arrow over the
**right** nibble turns into the **upper** legend, `0: Select memory OFF /
1: Select memory ON`. The top-to-bottom order of the legends therefore runs
**opposite** to the left-to-right order of the nibbles they point at. **D3**
(`⑬ Duplex and Tone settings`) is drawn the same way: left nibble → lower legend
`0: Duplex OFF / 1: Duplex−, 2: Duplex+`; right nibble → upper legend
`0: OFF, 1: Tone / 2: TSQL, 3: DTCS`. **D4** has only one arrow, from the left
nibble, so the hazard cannot arise there.

**(d) A printed index may differ from a field's measured position —
`ENCOUNTERED`, and pervasively.** From `⑤〜⑨` onward every printed index in D1
differs from the cell-width position measured for it, because the diagram elides
cells behind dashed ellipsis boxes (see `## Position arithmetic`). Separately,
the lower bar's **fourth group repeats an earlier block**: `❺〜[51]` restates, in
filled style, the indices `⑤`–`(51)` already carried by the upper bar and the
first three lower-bar groups. **Both occurrences were measured separately and
both are recorded**: the outline occurrence as D1 rows 4–16 (cell-widths 5–34),
the filled occurrence as D1 row 17 (cell-widths 35–37). The second was measured
on its own drawn extent and was **not** assumed to match the first — and it does
not: the first occupies 30 drawn cell-widths, the second 3. No printed index has
been adjusted to fit any measured position, and no measured position has been
adjusted to fit a printed index.

---

## STOP findings

Each is recorded, none is resolved. Every one has a corresponding CSV row
carrying `STOP <n>` in `notes`, with the value transcribed exactly as seen.

1. **PDF page 375, upper bar of D1, the `⑤〜⑨` bracket.** The bracket opens on
   the cell-4/cell-5 boundary and closes at the V on the cell-7/cell-8 boundary,
   covering **three** cell-widths: a grey `X:X` cell, a **dashed-outline grey cell
   containing a row of dots and no nibble divider**, and a grey `X:X` cell. The
   printed index range `⑤〜⑨` spans **five** indices. Measured extent 3, printed
   span 5. Stops because a measured extent disagrees with the printed numbering.
   Recorded as measured (5,1 → 7,2) with the index verbatim.

2. **PDF page 375, upper bar, the `⑩、⑪` bracket.** The two white cells it covers
   are the **8th and 9th** cell-widths from the bar's left border. Printed
   indices 10 and 11. Stops because the running position and the printed
   numbering disagree.

3. **Upper bar, bracketless `⑫`.** The grey cell it labels is the **10th**
   cell-width. Printed index 12.

4. **Upper bar, bracketless `⑬`.** The white cell it labels is the **11th**
   cell-width. Printed index 13.

5. **Upper bar, bracketless `⑭`.** The grey cell it labels — the one printed
   `X : 0` — is the **12th** cell-width. Printed index 14.

6. **Upper bar, `⑮〜⑰`.** Three white cells, the **13th–15th** cell-widths.
   Printed 15–17.

7. **Upper bar, `⑱〜⑳`.** Three grey cells, the **16th–18th** cell-widths.
   Printed 18–20.

8. **Upper bar, `㉑〜㉓`.** Three white cells, the **19th–21st** cell-widths.
   Printed 21–23.

9. **Upper bar, bracketless `㉔`.** One grey cell, the **22nd** cell-width.
   Printed index 24.

10. **Upper bar, `㉕〜㉗`.** The last three white cells, the **23rd–25th**
    cell-widths; the bar's right border closes immediately after them. Printed
    25–27.

11. **Lower bar, `㉘〜㉟`.** Drawn extent **three** cell-widths (grey `X:X`,
    dashed grey dotted cell, grey `X:X`), continuing the count at 26–28. Printed
    index range spans **eight** indices.

12. **Lower bar, `㊱〜㊸`.** Drawn extent **three** cell-widths at 29–31. Printed
    range spans **eight**.

13. **Lower bar, `㊹〜(51)`.** Drawn extent **three** cell-widths at 32–34.
    Printed range spans **eight**. (`51` is printed as an outline-circled
    two-digit numeral.)

14. **Lower bar, `❺〜[51]`.** Drawn extent **three** cell-widths at 35–37 — a
    single dashed box three cell-widths wide holding one long row of dots.
    Printed range spans **forty-seven** indices. Stops on the same arithmetic
    ground as 1 and 11–13, and additionally because the extent had to be taken
    from the bar's own cell pitch (measured off the drawn cells either side)
    rather than from dividers inside the box, there being none.

15. **Lower bar, `(52)〜(60)`.** Drawn extent **three** cell-widths at 38–40; the
    bar's right border closes immediately after. Printed range spans **nine**
    indices.

16. **PDF page 375, `(52)〜(60)` on the lower bar versus `(52)–(67)` in the right
    column.** The diagram bar's last bracket is labelled, at 600% enlargement
    beyond doubt, **outline-circled 52, wave dash, outline-circled 60**
    (`crops/lbl_5260.png`). The right-hand body column, four lines below
    `See '• DV TX call signs setting.' (p. 20-14)`, prints
    **`(52)–(67) Memory name setting`** with `16 characters (Fixed)` beneath it
    (`crops/rc_names.png`). Two printed things contradict each other about where
    the last group ends. Both readings are recorded, with their anchors.
    **Neither is resolved.** The bar reading `(60)` is the one carried in the CSV
    `field_index`, because the CSV row describes the bar; the body-text reading
    `(67)` is recorded here and in that row's `notes`.

17. **Lower bar, index sequence discontinuity at `❺〜[51]`.** Read left to right
    the printed indices run `①`…`㉗` (upper bar), then `㉘`…`(51)` (lower bar),
    then **`❺〜[51]` — indices 5 to 51 printed a second time, in a different
    (filled/reversed) style** — then `(52)〜(60)`. Indices 5–51 are therefore
    printed twice with different styling, and the ascending run is interrupted
    between `(51)` and `(52)` by an out-of-order block. Stops as an index-sequence
    discontinuity. Recorded; not reinterpreted.

18. **Lower bar, `❺〜[51]`: nibble positions not countable.** The three-cell-wide
    dashed box carries **no nibble divider and no cell divider anywhere inside
    it** — unlike every other group in D1, whose first and last cells each carry
    a dotted mid-rule. There is nothing to count for this group's first and last
    nibble, so both are recorded as **`UNREADABLE`** in the CSV rather than being
    assumed to be 1 and 2 by analogy with its neighbours.

---

## Observed disagreements

Odd, inconsistent or self-contradictory things printed on PDF page 375 that did
not stop the measurement. Recorded as printed; not resolved.

- **Memory-channel range printed two ways.** Left column, under `②, ③ Memory
  channel number`: **`0001–0099:  Memory channel 1 to 99`**. Right column, under
  `About clearing operation:`: **`②, ③:        Memory channel 0 to 99`**.
  "1 to 99" against "0 to 99".

- **Index separators differ between the diagram and the prose.** The bars join
  paired indices with an **ideographic comma** (`②、③`, `⑩、⑪`) and ranges with a
  **wave dash** (`⑤〜⑨`). The body text uses an **ASCII comma** (`②, ③`) and an
  **en dash** (`⑮–⑰`, `⑤–(51)`, `❺–[51]`) for the same relations.

- **`⑭` cell glyph.** The 12th cell of the upper bar prints `X`, dotted divider,
  then a glyph read here as digit **`0`**. In this typeface digit zero and capital
  O are near-identical at any magnification; the reading rests on the identical
  glyph in the D4 breakout box, whose legend enumerates `0:`, `1:`, `2:`. Nothing
  in the recorded positions turns on which it is.

- **A single highlighted word.** In the left column the word **`Bank`** in
  `①Bank number` is printed on a **yellow highlight panel**. No other word on the
  page is highlighted, and nothing on the page explains the highlight.

- **`⑭` has only one leader.** D4's box draws one arrow, from its left nibble.
  Its right nibble prints a fixed `0` and no leader points at it, so the page
  never says in words what that nibble is. D2 and D3, by contrast, both draw two.

- **Six of D1's forty drawn cell-widths carry no data.** Upper bar cell-width 6;
  lower bar cell-widths 27, 30, 33 and 39; and the three-wide box at 35–37. All
  are dashed-outline and hold a row of dots. The diagram nowhere prints how many
  bytes any of them stands for.

- **Prose widths appear beside three of the elided groups** — `(8 characters;
  fixed)` under each of `㉘〜㉟`, `㊱〜㊸` and `㊹〜(51)`, and `16 characters
  (Fixed)` under the memory-name entry. **None of these was used for any
  measurement in this leg**; they are noted only because they are printed.

- **A PDF widget sits above the page content.** A blue `Previous view` button is
  rendered at the top of PDF page 375. It is a navigation control, not page
  content, and no value was taken from it.

---

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.

*Disclosure alongside the attestation, so that it is exact:* the only commands
run against anything other than my own renders were `pdfinfo` on this same PDF
(for its page count and page size, reported in `## Source`), and `ls` on the two
directories this leg itself created under its own output path, to confirm that
`pdftoppm` had written the files I was about to read. No repository file, no
other manual, no `*_layout.txt`, no prior leg's output and no web resource was
opened, searched or listed, and `pdftotext` was never invoked.
