# IC-7100 field ledger — quarantine leg L

Companion to `IC-7100-field-ledger.csv` (21 rows, four diagrams, all on one PDF page).

## Source

- **Document title, as printed on the cover (PDF page 1):** `FULL MANUAL`, in the black
  banner beneath the Icom logo; beneath it, `HF/VHF/UHF ALL MODE TRANSCEIVER` above the
  model mark `IC-7100`; `Icom Inc.` at the foot. The chapter list on the same cover page
  gives `20  CONTROL COMMAND`.
- **Revision code, as printed:** `A7085-2EX-5`, at the foot of the left-hand column of the
  back page (PDF page 387), printed immediately above `© 2013–2021 Icom Inc.    May 2021`.
- **File path:** `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7100_civ_FM_5.pdf`
- **Page count:** 387 PDF pages.

## Extent

The CI-V chapter is §20 `CONTROL COMMAND`; folios in it read `20-n` and the PDF page is
`359 + n`. That mapping was confirmed by eye on every page rendered: PDF 374 carries folio
`20-15`, PDF 375 carries folio `20-16`, PDF 376 carries folio `20-17`.

| PDF page | printed folio | rendered at | read? | what it contributed |
|---|---|---|---|---|
| 1 | *(cover, no folio)* | 150 dpi | yes | cover title and model, for `## Source` only |
| 374 | 20-15 | 300 dpi | yes, for context only | establishes what is printed immediately **before** the transcribed material; carries `• DV RX call sign setting`, `• DV RX message setting`, `• DV RX Status setting`. No memory-record data-block diagram. Nothing transcribed from it. |
| **375** | **20-16** | **300, 400 and 600 dpi** | **yes — every CSV row comes from this page** | the whole of `• Memory content setting`, command `1A 00`: the two-row data bar and its three sub-diagram boxes, plus the legend list that labels them |
| 376 | 20-17 | 300 dpi | yes, for context only | establishes what is printed immediately **after**; carries `• RIT frequency settings`. No memory-record data-block diagram. Nothing transcribed from it. |
| 387 | *(back page, no folio)* | 150 dpi | yes | revision code, for `## Source` only |

**Where the transcribed material begins and ends.** It is wholly contained on PDF page 375
(folio 20-16). Immediately before it on that page are the black section rule
`Remote jack (CI-V) information`, then `◇ Data content description (Continued)`, then the
bold heading `• Memory content setting` and the line `Command: 1A 00`. Immediately after
the last transcribed matter come the bold sub-heading `About clearing operation:` in the
right column and the hatched `NOTE:` box below it, then the folio `20-16`. The material
does not begin on PDF 374 and does not continue onto PDF 376: the preceding page ends with
the `• DV RX Status setting` table, and the following page opens a new heading,
`• RIT frequency settings`.

**Section discipline.** Pages 374, 375 and 376 all carry the identical black section rule
`Remote jack (CI-V) information` and the identical line `◇ Data content description
(Continued)`; only the bold `•` heading below them differs. Every row was taken from
within the heading `• Memory content setting` alone.

### Diagram identities

Assigned `D1`…`D4` in page order. Page 375 is set in two columns below the full-width bar,
so page order after `D1` is taken as reading order — the left column complete, then the
right column. That ordering agrees with the document's own index sequence (4, then 13,
then 13's neighbour 14), which is why 14's material begins at the head of the right column.
Ordering the sub-diagrams by raw vertical position instead would place `D4` above `D2`;
this is a judgement call and is flagged as such.

| id | printed caption, verbatim | position on the page |
|---|---|---|
| `D1` | `• Memory content setting` — with `Command: 1A 00` printed on the line below it, immediately above the bar | full page width, top of the page below the two-column rule; drawn as **two stacked bars**, an upper bar of 25 cells (indices 1 to 27) and, below it, a lower bar of 12 cells plus one bare dotted run (indices 28 to 60 and the filled group). Treated as **one** diagram: the lower bar is the continuation of the upper, the numbering running unbroken across the wrap. |
| `D2` | `④Split and Select memory settings` *(no space is printed between the circled numeral and `Split`)* | left column, below the line `0109:  Call channel 430-C2`; a two-cell box `X ⋮ X` with two upward leader arrows beneath |
| `D3` | `⑬ Duplex and Tone settings` | left column, at its foot, below the line `01: Data mode ON`; a two-cell box `X ⋮ X` with two upward leader arrows beneath |
| `D4` | `⑭ Digital squelch setting` | right column, at its head, directly below the two-column rule; a two-cell box printed `X ⋮ 0` with a single upward leader arrow from its left-hand cell |

`D2`, `D3` and `D4` are sub-diagrams of the same memory record, each expanding one byte of
`D1`. Including them is the completeness-favouring reading of "every memory-record
data-block diagram": each draws a data block and carries a numbered field. Their rows
therefore repeat indices `4`, `13` and `14` already recorded against `D1`. That repetition
is recorded, not merged — a duplicate is a fact about the page. The value enumerations on
their leader lines (`0: Split OFF, 1: Split ON`, and so on) are encodings and meanings, and
are deliberately **not** transcribed.

## Method

Every value in the CSV was read from a rendered page image. No text layer was used.

1. **Locate — 300 dpi.** Into a fresh directory `renders300/` created for this leg:

   ```
   pdftoppm -png -r 300 -f 374 -l 376 <pdf> renders300/p
   ```

   The three renders were read as images to find `• Memory content setting` and to confirm
   the folio-to-PDF-page mapping and the section boundaries recorded under `## Extent`.
   Cover and back page were located separately at 150 dpi into `cover/`
   (`-f 1 -l 1` and `-f 387 -l 387`) for the `## Source` section only.

   The only directories listed at any point in this leg were this leg's own output
   directories, created by this leg (`renders300/`, `renders400/`, `renders600/`, `crops/`,
   `pass2/`, `measure/`, `cover/`), listed solely to confirm that the renders had been
   produced. No repository directory was listed, searched or browsed, and no file outside
   this leg's output directory was opened other than the PDF itself.

2. **Read — 400 dpi.** The single page transcribed was re-rendered on its own:

   ```
   pdftoppm -png -r 400 -f 375 -l 375 <pdf> renders400/p     # 3308 x 4678
   ```

   Every first-pass value was read from this raster.

3. **Crop and enlarge.** ImageMagick **was** available (`/opt/homebrew/bin/magick`, and
   `convert`) and was used throughout. Each numbered band, each bar segment and each legend
   block was cropped into its own image and enlarged until every numeral, rule and glyph
   stood clear of its neighbours. Representative commands, all of the form
   `-crop … +repage -resize …`:

   ```
   magick renders400/p-375.png -crop 3000x420+180+920   +repage -resize 150% crops/D1_both_rows.png
   magick renders400/p-375.png -crop  950x85+200+930    +repage -resize 300% crops/D1_r1_A.png
   magick renders400/p-375.png -crop  950x85+1100+930   +repage -resize 300% crops/D1_r1_B.png
   magick renders400/p-375.png -crop 1000x85+2010+930   +repage -resize 300% crops/D1_r1_C.png
   magick renders400/p-375.png -crop  620x85+200+1145   +repage -resize 350% crops/D1_r2_A.png
   magick renders400/p-375.png -crop  620x85+780+1145   +repage -resize 350% crops/D1_r2_B.png
   magick renders400/p-375.png -crop  420x80+1560+1150  +repage -resize 500% crops/D1_r2_last_group.png
   magick renders400/p-375.png -crop  700x80+1100+1150  +repage -resize 400% crops/D1_r2_filled_group.png
   magick renders400/p-375.png -crop 1420x780+230+1370  +repage -resize 200% crops/legendL_1.png
   magick renders400/p-375.png -crop 1420x780+1690+1840 +repage -resize 200% crops/legendR_2.png
   ```

   (Ten of the twelve `crops/` commands are shown; the remainder are the same form at
   further offsets down each column.) The single highest enlargement, 500 per cent on the
   last group of the lower bar, was made specifically to settle the numeral flagged in the
   hazard clause.

4. **Cell-position measurement — 400 dpi, pixel scan.** Because hazard (d) requires a
   *measured* position and not an inferred one, the bar's rules were measured rather than
   counted by eye. A Pillow script (`measure/scan5.py`) scanned each bar band column by
   column and reported every column dark over ≥45 per cent of the band height:

   - upper bar: horizontal rules at y = 1017 and y = 1097; **26 vertical rules**, from
     x = 237 to x = 2994, hence **25 cells**, at a uniform pitch of **110.3 px** (measured
     gaps: 110 or 111 px throughout, no exception);
   - lower bar: horizontal rules at y = 1236 and y = 1316; **14 vertical rules**, from
     x = 237 to x = 1896, at the same 110–111 px pitch **except** for a single gap of
     **337 px** between x = 1229 and x = 1566 — hence **12 cells and one bare run**.

   The measured 25 cells of the upper bar reconcile exactly with the printed indices: 27
   printed indices drawn as 25 cells, the one ellipsis cell in the `5~9` group absorbing
   the two omitted indices 6 to 8. This arithmetic checking out is why no STOP is raised
   against the upper bar.

5. **`pdftotext -layout` — NOT RUN.** It was not run at any point, for navigation or
   otherwise. Navigation was done entirely from the 300 dpi renders. No text layer of this
   or any other document was read.

6. **`tesseract`.** Available at `/opt/homebrew/bin/tesseract` but **not used**. Every
   numeral and label was legible by eye at 400 dpi enlarged, and again at 600 dpi, so no
   OCR aid was needed and no value rests on one.

7. **Second independent pass — done.** With the first pass complete, the page was
   re-rendered at a **different dpi** and re-read through **different crop windows at
   different enlargements**:

   ```
   pdftoppm -png -r 600 -f 375 -l 375 <pdf> renders600/p     # 4961 x 7016
   ```

   The second raster differed from the first in three ways: 600 dpi rather than 400 dpi;
   the upper index band cut into **two** windows rather than three, and at 180 per cent
   rather than 300 per cent, so that no group fell at the same distance from a crop edge as
   before; and the legend re-cut into seven windows at 110–250 per cent whose boundaries
   fall in different places from the first pass's five-per-column split. The three
   sub-diagram index numerals were re-read from their own dedicated 250 per cent windows
   (`pass2/box4.png`, `pass2/box13.png`, `pass2/box14.png`).

   **Result: the two passes agreed in every cell.** There is no disagreement to report,
   and therefore no third render was needed and no two-pass STOP arises. Specifically, both
   passes independently read the last group of the lower bar as `52` and `60`, and both read
   the corresponding legend key as `52` and `67`.

### Transcription conventions for `field_index`

Recorded verbatim, uncollapsed and unnormalised, with two renderings stated openly so the
crosscheck can rely on them:

- The **range separator** printed in the bar is a wave dash — a long, gently waved rule at
  mid-height, not the short ASCII tilde. It is written `~` in the CSV, matching the shared
  convention's own worked example (`4~8`). The legend list prints the same ranges with an
  **en dash** instead (`5–9`, `52–67`); that difference is recorded in each row's `notes`
  and under `## Observed disagreements`, and is never carried into `field_index`, which
  always reports the **bar** as printed.
- The **pair separator** printed in the bar is an **ideographic comma** `、`, not a Western
  comma. It is written verbatim: `2、3`, `10、11`. The legend list prints `2, 3` and
  `10, 11` with a Western comma and a space. A crosscheck joining on the form `2, 3` should
  read it as this leg's `2、3`; the glyph was not normalised, because normalising it would
  destroy a fact the page prints.
- `index_style` describes how the **numerals** are drawn. The separator glyphs themselves
  carry no styling.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — ENCOUNTERED.** `D1` draws its index
  numerals in two distinct ways. Seventeen of its eighteen groups are `circled`: black
  digits inside an open circular outline on white. One group, the fourth on the lower bar,
  is `filled`: white digits reversed out of a solid black disc — a filled 5 and a filled 51.
  Both styles
  are recorded as drawn; neither has been normalised to the other, and no meaning has been
  inferred from either. The same two styles recur in the NOTE box below, where the outlined
  pair and the filled pair are printed in the same sentence.

- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** No label on
  page 375 is rotated; every index numeral, caption and legend line is set horizontally.
  (Rotated labels do occur nearby — the `• RIT frequency settings` diagram on PDF 376 sets
  its leader labels vertically — but nothing was transcribed from that page.) Position was
  in any case read from the picture throughout, and the bar's cell positions were measured
  by pixel scan rather than taken from any extraction order.

- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In `D2`, the two upward
  arrows cross: the arrow from the **right-hand** cell rises to the **upper** label
  (`0: Select memory OFF` / `1: Select memory ON`), whilst the arrow from the **left-hand**
  cell runs down and across to the **lower** label (`0: Split OFF, 1: Split ON`). The
  printed top-to-bottom order of the labels therefore runs opposite to the left-to-right
  order of the cells they point at. `D3` repeats the same reversal exactly: the upper label
  (`0: OFF, 1: Tone` / `2: TSQL, 3: DTCS`) belongs to the right-hand cell and the lower
  (`0: Duplex OFF` / `1: Duplex–, 2: Duplex+`) to the left. `D4` has a single leader and no
  reversal. Each leader was followed by eye from the label to the cell it lands on. No
  recorded value was affected, because these leader labels are value enumerations and were
  not transcribed; the hazard is recorded because it occurred, not because it changed a cell.

- **(d) A printed index may differ from a field's measured position — ENCOUNTERED.** The
  lower bar's filled group repeats a block already printed: the NOTE states that the data of
  the filled pair 5 to 51 is the same data as the outlined pair 5 to 51, and the filled
  group's printed range `5~51` reproduces exactly the indices already drawn as the outlined
  groups `5~9` through `44~51`. For every field in the repeating block and in the block it repeats,
  the CSV's `notes` therefore carry **both** the printed index (in `field_index`, as
  printed) **and** the position measured on the render, as the drawn-cell ordinals of 37 and,
  for the filled group, as a measured extent in pixels. The two do not agree and have not
  been reconciled: the outlined block occupies measured drawn cells 5–7 and 26–34, whilst
  the filled block that repeats it occupies **no drawn cells at all** — it is a bare run of
  dots of measured extent 337 px, 3.05 times the bar's measured 110.3 px cell pitch, whilst
  its printed index range spans 47 byte indices. No individual field's byte position can be
  measured inside it, because the drawing gives it no rules to measure. See STOP 2.

## STOP findings

1. **Index-sequence discontinuity: the sequence runs backwards, and two indices are printed
   twice in two different styles.** PDF page 375 (folio 20-16), lower bar of `D1`, the
   fourth group from the left — the long bare run of dots between the bracket closing after
   `44~51` and the bracket opening before `52`. What is printed there is the numeral 5, a
   wave dash, and the numeral 51 — both as white digits reversed out of solid black discs.
   (No Unicode character exists for a circled or filled 51, so this document spells such
   numerals out rather than attempting a glyph.) Reading the bar in
   printed order, the indices run 1, 2, 3, 4, 5~9, 10, 11, 12, 13, 14, 15~17, 18~20, 21~23,
   24, 25~27, 28~35, 36~43, 44~51, **5~51**, 52~60. This stops the transcription on three
   counts at once, each of them a listed discontinuity: the sequence goes **backwards**,
   from 51 to 5; the whole span 5 to 51 is a **repeat** of indices already used earlier in
   the same diagram; and the indices 5 and 51 are each **printed twice with different
   styling**, outlined at their first appearance and filled at their second. Printed order
   has been followed, not numeric order: the row for `5~51` sits between `44~51` and
   `52~60` in the CSV, exactly where the page puts it. Nothing has been reordered, merged
   or renumbered. CSV row: `D1`, `5~51`, `filled`.

2. **Printed index disagrees with measured extent, in the block that repeats another
   block.** Same anchor as STOP 1. The printed range `5~51` spans 47 byte indices. Measured
   on the 400 dpi render, that group is drawn with **no cell rules whatsoever**: the pixel
   scan of the lower bar found consecutive vertical rules at 110–111 px throughout except
   for one gap of **337 px**, between x = 1229 and x = 1566, which is this group. 337 px is
   3.05 times the bar's uniform measured cell pitch of 110.3 px. So the block that the NOTE
   says holds the same 47 bytes is drawn three cells wide and with no internal divisions,
   whilst the outlined block it repeats is drawn with real cells (measured drawn cells 5–7
   and 26–34 of 37). Both readings are recorded, in `field_index` and in `notes`
   respectively, and neither has been reinterpreted in the light of the other. No byte
   position can be measured for any individual field inside this group, and none has been
   invented. CSV row: `D1`, `5~51`, `filled`.

3. **The bar and the body text disagree about the last group's index range.** PDF page 375
   (folio 20-16). Two things are printed, and they contradict each other:
   - **In the diagram bar** — `D1`, lower bar, the fifth and last group, at the extreme
     right-hand end, immediately right of the bare run of dots: circled `52`, a wave dash,
     circled `60`. Read at 400 dpi enlarged to 500 per cent and
     independently again at 600 dpi; both digits of both numerals are unambiguous, and the
     second numeral is `60`, not `67` — the second digit is a closed `0`, with no ascender
     and no crossbar.
   - **In the legend list** — right column, the entry immediately below `See ‘• DV TX call
     signs setting.’ (p. 20-14)`: `52–67 Memory name setting`, with an en dash, and on the
     line below it, at the left margin, `16 characters (Fixed)`.

   The arithmetic does not add up. `52` to `67` inclusive is 16 indices, which agrees with
   the legend's own `16 characters (Fixed)`; `52` to `60` inclusive is 9. The two printed
   statements cannot both be true. Neither has been resolved, corrected or preferred. The
   CSV records the **bar's** reading, `52~60`, in `field_index`, because that row is a row
   of the diagram; the legend's `52–67` and its `16 characters (Fixed)` line are recorded
   verbatim in that row's `notes`, with their anchor. The label `Memory name setting` is
   recorded because it is the only label printed against this group, notwithstanding that
   the key it is printed against reads `52–67`. The drawing itself cannot arbitrate: the
   group is drawn as first cell, ellipsis cell, last cell, which absorbs either 9 or 16
   indices without any change in measured width, so there is no third measurement that
   settles it. CSV row: `D1`, `52~60`, `circled`.

No other STOP arises. In particular there is **no** unreadable value (every numeral was
legible by eye at 400 dpi enlarged and again at 600 dpi, so no cell is `UNREADABLE`), and
**no** two-pass disagreement (the two passes agreed in every cell). The upper bar's
arithmetic reconciles exactly — 27 printed indices, 25 measured cells, one ellipsis cell
absorbing the two omitted indices — and so raises nothing.

## Observed disagreements

Printed as described; odd or inconsistent, but not blocking, and not resolved.

1. **The bar and the legend use different range separators for the same groups.** The bar
   prints a wave dash (`5~9`, `15~17`, `28~35`, `52~60`); the legend list prints an en dash
   for the identical groups (`5–9`, `15–17`, `28–35`, `52–67`). Both forms appear on the
   same page for the same fields.
2. **The bar and the legend use different pair separators for the same groups.** The bar
   prints an ideographic comma with no space (`2、3`, `10、11`); the legend prints a Western
   comma and a space (`2, 3`, `10, 11`). Again, same page, same fields.
3. **The filled group has no legend entry.** Every other group on the bar is keyed in the
   legend list. The filled group `5~51` is not; the only text referring to it is the hatched
   `NOTE:` box, whose first bullet reads `The same data as 5–51 are stored in 5–51.` — the
   first pair set as outlined circled numerals, the second as filled ones, which is the only
   thing distinguishing the two halves of that sentence. Two further bullets below it use
   the same pair of styles the same way. Its `label_verbatim` is therefore empty — nothing
   is printed against it — rather than `-`.
4. **A yellow highlight annotation survives in the published file.** On page 375 the word
   `Bank` in the legend line `①Bank number` is printed on a yellow ground. The same appears
   on page 374, where `call sign` in `③–⑩ Caller station’s call sign (8 characters; fixed)`
   is highlighted the same way. These look like editorial marks left in the released PDF.
   They do not alter the words, and the label has been transcribed as `Bank number`.
5. **Inconsistent spacing after the circled numeral in the legend.** `①Bank number` and
   `④Split and Select memory settings` are set with no space after the numeral; `⑫ Data
   mode setting`, `⑬ Duplex and Tone settings`, `⑭ Digital squelch setting` and the rest are
   set with one. Transcribed labels begin at the first letter either way.
6. **The `14` cell prints a fixed nibble where every other cell prints `X`.** In the upper
   bar, drawn cell 12 is printed `X ⋮ 0` rather than `X ⋮ X`; sub-diagram `D4` shows the same
   byte the same way, `X ⋮ 0`, and gives its single leader to the left-hand nibble only. The
   glyph in the bar is a closed round form matching the `0` in `D4`'s box, read as a zero in
   both places. This is consistent between the two, and is noted only because it is the sole
   cell in `D1` whose second nibble is not an `X`.
7. **`D2` and `D3` are drawn identically but describe different bytes.** Both are a
   two-cell box `X ⋮ X` with two crossing upward leaders, at the same size and with the same
   arrow geometry; only the circled index above the box and the labels differ. Recorded as
   two diagrams because the page prints two, in two places, under two captions.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual,
transcription, source file, generated artefact or web resource was opened, and no directory
was listed.
