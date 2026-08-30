# IC-7100 — quarantine leg B — memory-record transcription

Companion to `IC-7100-transcription-b.csv` (19 data rows, one diagram, `D1`).

## Source

- **Title as printed on the cover** (PDF page 1): `IC-7100` beneath `HF/VHF/UHF ALL MODE TRANSCEIVER`, with `FULL MANUAL` in the black band at the left and `Icom Inc.` at the foot. The chapter list on the same cover runs `INTRODUCTION`, `1 PANEL DESCRIPTION` … `20 CONTROL COMMAND`, `21 SPECIFICATIONS AND OPTIONS`, `INDEX`.
- **Revision code:** none is printed on the cover. No revision or part code appears anywhere on the cover page at 300 dpi — the foot of the cover carries only the `Icom Inc.` logotype and its rule. No other page was rendered to hunt for one, because the brief names the pages I may open. **The revision code is therefore not recorded: an absent statement, not a gap I have filled.**
- **File path:** `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7100_civ_FM_5.pdf`
- **Page count:** 387 PDF pages.
- **Chapter and folio convention:** the CI-V material sits in chapter `20 CONTROL COMMAND`; the running head on every page read is `20  CONTROL COMMAND` and the folio at the foot reads `20-n`. On every page I read, PDF page = 359 + n, exactly as the leg brief states. Both numbers are cited throughout.

## Extent

### Pages rendered and read

| PDF page | Printed folio | Rendered at | Read? | What it contributed |
|---|---|---|---|---|
| 1 (cover) | — (no folio) | 150 dpi, then 300 dpi | yes | Cover title and the absence of a revision code — rendered solely to satisfy the `## Source` requirement above. |
| 364 | 20-5 | 300 dpi | yes | `◇ Command table (Continued)`. Contributed the command-table row `1A` / `00` / `see p. 20-16` / `Send/read the Memory channel contents`, which is the pointer to the record page. |
| 365 | 20-6 | 300 dpi | yes | `◇ Command table (Continued)`. Contributed nothing to any field. |
| 366 | 20-7 | 300 dpi | yes | `◇ Command table (Continued)`. Contributed nothing to any field. |
| 367 | 20-8 | 300 dpi | yes | `◇ Command table (Continued)`. Contributed nothing to any field. |
| 368 | 20-9 | 300 dpi | yes | `◇ Command table (Continued)`. Contributed nothing to any field. |
| 369 | 20-10 | 300 dpi | yes | `◇ Command table (Continued)`. Contributed nothing to any field. |
| 370 | 20-11 | 300 + 400 dpi | yes | `◇ Data content description` → `• Operating frequency` and `• Operating mode`. Contributed the half-byte digit labels for field `5~9` and the mode/filter code table for field `10、11`. |
| 371 | 20-12 | 300 + 400 dpi | yes | `◇ Data content description (Continued)` → `• Character code setting`. Contributed the full Character/ASCII-code table used as `values_verbatim` for the name field row `52–67`. |
| 372 | 20-13 | 300 + 400 dpi | yes | `◇ Data content description (Continued)` → `• Duplex Offset frequency setting`. Contributed the half-byte digit labels for field `25~27`. |
| 373 | 20-14 | 300 + 400 dpi | yes | `◇ Data content description (Continued)` → `• Repeater tone/tone squelch frequency setting`, `• DTCS code and polarity setting`, `• Digital code squelch setting`, `• DV TX call signs setting`. Contributed the values for fields `15~17`, `18~20`, `21~23`, `24`, `28~35`, `36~43`, `44~51`. |
| 374 | 20-15 | 300 dpi | yes | `◇ Data content description (Continued)` → `• DV RX call sign setting`, `• DV RX message setting`, `• DV RX Status setting`. Contributed nothing to any field; read only to establish what precedes the transcribed material. |
| 375 | 20-16 | 300 + 400 + 600 dpi | yes | **The record page.** `• Memory content setting`, `Command: 1A 00`, the two-band data-block diagram, the three legend boxes, the `About clearing operation:` block and the `NOTE:` box. Every `field_index`, `width_bytes` and band position in the CSV comes from here. |
| 376 | 20-17 | 300 dpi | **no** | Rendered incidentally by the first batch command (`-f 364 -l 376`) and never opened. Not read, because the brief does not name it. |

### The character table and the set-mode pages

- **Character table — PRINTED.** It is printed on PDF page 371 (folio 20-12), left column, headed `• Character code setting`, `Command: 1A 00, 1A 05 0200, 1A 05 0201, 1A 05 0206, 1A 05 0207, 1A 05 0208, 1A 05 0209, 1A 05 0211, 1F 02, 20 0001, 20 0002`. It is a four-column table (`Character | ASCII code | Character | ASCII code`) of 18 printed rows. **It contributed** the whole of `values_verbatim` for the body-text name row `52–67`, which cross-references it as `See ‘• Character code setting.’ (p. 20-12)`. A *second, smaller* character table is printed on PDF page 373 (folio 20-14) under `• Character’s code of the call sign` (four entries only); **it contributed** `values_verbatim` for the three call-sign rows `28~35`, `36~43` and `44~51`, which cross-reference `See ‘• DV TX call signs setting.’ (p. 20-14)`. The two tables are different, and each row cites the one its own cross-reference points at.
- **Set-mode / menu material — NOT PRINTED on the pages named.** PDF pages 364–374 do not carry a set-mode or menu section. Pages 364–369 (folios 20-5 to 20-10) are `◇ Command table (Continued)` — a CI-V command index, not set-mode material — and pages 370–374 are `◇ Data content description`. **No field on the record page refers to any set-mode or menu item**; every cross-reference on page 375 points into `◇ Data content description` on pages 370, 371, 372 or 373. Pages 364–369 contributed only the command-table row identifying `1A 00` (cited above); pages 370–373 contributed the encodings and values listed in the table. Page 374 contributed nothing.

### Where the transcribed material begins and ends

The material begins on PDF page 375 (folio 20-16). Printed immediately before it, top of the same page: the blue `Previous view` button, the running head `20  CONTROL COMMAND`, the black section band `Remote jack (CI-V) information`, and the line `◇ Data content description (Continued)`. The first line of the material itself is the bold heading `• Memory content setting`, then `Command: 1A 00`, then the upper band of the data block.

It ends on the same page with the `NOTE:` box in the right column, whose last line reads `that you set the same data as (5)–(51).`, above the folio `20-16`. Nothing of the memory record continues onto another page: the lower band of the diagram terminates at the group bracketed `52 ~ 60`, and the last indexed text entry is `(52)–(67) Memory name setting`. The section immediately preceding, on PDF page 374 (folio 20-15), ends with `• DV RX Status setting` (right column) and the hatched note `“FF” stands for no call sign receiving after turning ON the transceiver.` (left column). What follows on PDF page 376 (folio 20-17) is **not recorded**: that page was not read.

## Method

**Everything recorded in the CSV was read from a rendered page image of this PDF.** Nothing was read from a text layer.

### dpi at each step

1. **Locate — 300 dpi.** Into a fresh directory `.../legs-out/ic7100/B/r300/`, created for this leg and empty beforehand:
   `pdftoppm -png -r 300 -f 364 -l 376 <pdf> r300/p`
   and `pdftoppm -png -r 150 -f 1 -l 1 <pdf> r300/cover` (later re-rendered at 300 dpi as `r300/cover300`). The renders were read as images to find the sections. Only the section whose printed heading matches was worked in: `• Memory content setting` on page 375, and, for each cross-reference, the section whose bullet heading the cross-reference names — several adjacent bullet headings on pages 372–373 resemble one another (`• Split offset frequency setting` vs `• Duplex Offset frequency setting`; `• Digital code squelch setting` vs `• DTCS code and polarity setting`), so each was matched on its full printed heading and its `Command:` line before anything was read from it.
2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f 370 -l 375 <pdf> r400/p`. Every value in the first pass was read from these.
3. **Crop and enlarge — ImageMagick was available and was used** (`/opt/homebrew/bin/magick`, also `convert`). Each band, numbered bracket and legend was cropped and enlarged, e.g.
   `magick r400/p-375.png -crop 1100x260+200+930 +repage -resize 250% crops/top_seg1.png`
   `magick r400/p-375.png -crop 1250x290+230+1170 +repage -resize 260% crops/bot_seg1.png`
   `magick r400/p-375.png -crop 340x110+1740+1180 +repage -resize 700% crops/name_bar_idx.png`
   `magick r400/p-373.png -crop 300x780+1900+1250 +repage -rotate 90 -resize 320% crops/p373_polarity2.png` (the referenced diagrams set their leader labels rotated 90°; they were rotated upright before reading)
   Segments overlapped so that every cell boundary was seen in at least two crops. Crops are kept in `crops/` (first pass) and `crops2/` (second pass).
4. **`pdftotext` was NEVER run**, in any form, navigational or otherwise. Page numbers came from the leg brief’s folio rule (PDF page = 359 + n) and were confirmed by reading the printed folio on each render.
5. **tesseract was available but was NOT used.** Every value was read by eye from the renders; no OCR value appears in the CSV.
6. **Objective pixel measurement** was used once, as an aid to the eye and not as a substitute for it, to settle the glyph in the low half-byte of field `14` (see the second-pass record). Ink-column profiles and trimmed bounding boxes were taken from the 600 dpi render with `magick … -threshold 50% … gray:-` piped to a short node script.

### Transcription conventions (declared, because they affect what the CSV can carry)

- **`field_index`** transcribes the printed numerals and the printed separator between them: the band joins ranges with a wave dash `~` and pairs with an ideographic comma `、`; the body text joins ranges with an en dash `–` and pairs with a comma and space. Both forms are preserved where they occur. The circled and filled numerals themselves are written as plain digits, because Unicode has no circled forms above 50 and no filled forms above 10, and mixing glyphs with digits would itself normalise the styles. **The drawn STYLE of every index is instead recorded verbatim in that row’s `notes`**, since the header has no style column. No two styles are merged anywhere.
- **`pdf_page`** carries one page where the whole of a field’s semantics is printed on the record page, and two, `"375; 370"` form, where the label and cross-reference are on 375 and the encoding and values are on the referenced page. The `visual_anchor` names both locations in words.
- **Elision cells.** The band draws a long group as `cell — dashed elision cell — cell`, three drawn boxes standing for the whole run. `width_bytes` is therefore the printed index span, not the count of drawn boxes; the measured position of every field is given in `notes`. The convention is used identically for `5~9`, `28~35`, `36~43`, `44~51`, `5~51` (filled) and `52~60`, so it is the diagram’s own convention and not an inconsistency.
- **Diagrams on the record page.** `D1` is the memory-record data block itself — printed caption `• Memory content setting`, sub-caption `Command: 1A 00`, drawn as two bands across the top third of page 375. It is the only diagram on the page carrying a numbered field band, and every CSV row belongs to it. Three further boxes are printed on the page — the legend box under `(4) Split and Select memory settings` (left column, upper), the legend box under `(13) Duplex and Tone settings` (left column, foot), and the legend box under `(14) Digital squelch setting` (right column, head). Each is a one-byte legend for a `D1` field, repeating that field’s own index above it and carrying no field of its own; **their content is transcribed into the `D1` row for the field they legend**, anchored there, rather than given duplicate rows. This is a judgement call and is recorded as one.

### Second independent pass — done

Both passes were completed. The second pass re-read every value from a different raster and did not consult the first pass’s numbers while reading.

- **How the second raster differed:** a fresh 600 dpi render of page 375 (`pdftoppm -r 600`, 4961 × 7016 px, against 3308 × 4678 at 400 dpi), cut into **different crop windows** — four overlapping 1250 px windows across the upper band and two across the lower band, offset so that every group that fell mid-window in pass 1 fell mid-crop in pass 2 — and enlarged 200 % rather than 250 %/260 %. Field `14` was additionally enlarged 600 % and measured in pixels.
- **Cells where the passes disagreed — two, both settled by a third render:**
  1. **Field `14`, low half-byte, band cell and legend box.** Pass 1 (300 dpi whole page, then 400 dpi at 250 %) read the band glyph as a capital `O` and the legend glyph as a digit `0`. Pass 2 (600 dpi at 600 %) read both as the digit `0`. **Third render and measurement settled it as the digit `0`:** on the 600 dpi raster the band glyph’s ink is 44 px wide against a 50 px `X` in the same cell (ratio 0.88), and 44 × 58 px overall (aspect 0.76); the legend glyph is 36 px against a 46 px `X` (ratio 0.78) and 36 × 55 px (aspect 0.65). A capital `O` in this typeface is *wider* than an `X` (ratio ≈ 1.16, aspect ≈ 1.03); a digit zero is narrower (ratio ≈ 0.78, aspect ≈ 0.70). Both glyphs are the digit zero. Recorded as `0` and the measurement is quoted in the row’s `notes`. **Not a STOP** — a third render settled it.
  2. **Page 373, `• DTCS code and polarity setting`, transmit-polarity leader label.** Pass 1, reading the label in its printed 90°-rotated orientation, appeared to show `1: Reverese`. Pass 2, on the same crop rotated upright and enlarged 320 %, read `1: Reverse`. **Third render settled it as `1: Reverse`** — the apparent extra `e` was a stroke of the rotated `s` at low effective resolution. Recorded as `1: Reverse`. **Not a STOP.**
- **Cells where the passes agreed:** every drawn cell count in both bands (upper band: 25 drawn boxes, one of them a dashed elision box, standing for 27 bytes; lower band: 13 drawn boxes, four of them dashed elision boxes, one of those being the wide transmit elision); every bracket span; every index numeral including `52` and `60` in the band and `52` and `67` in the body text; the filled/reversed `5` and `51`; and every value list on pages 370–373.

### Disclosures

For completeness, and so the attestation below is not read as claiming more than it should: `pdfinfo` was run once on this same PDF and is the source of the page count `387` quoted in `## Source` (nothing else); `ls` was run on this same PDF’s path and on this leg’s own output directory under `.../legs-out/ic7100/B`, and on nothing else. No other file was opened by any means.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** The band draws its indices in two styles. Every index from `1` to `51` in the receive part, and `52`/`60` in the name group, is an **outline circled numeral**. The transmit group’s bracket is labelled with **filled / reversed numerals** — a white `5` and a white `51` on solid black discs — and the `NOTE:` box uses the same two styles side by side in one sentence (`The same data as (5)–(51) are stored in [filled]5–[filled]51.`). The styles are recorded per row in `notes` and are nowhere merged; no meaning is inferred for either style beyond what the `NOTE:` box itself prints.
- **(b) Vector groups with rotated labels — ENCOUNTERED.** Every leader label on the referenced diagrams (page 370 `• Operating frequency`; page 372 `• Duplex Offset frequency setting`; page 373 `• Repeater tone/tone squelch frequency setting` and `• DTCS code and polarity setting`) is set rotated 90°, running bottom-to-top. Positions were read from the picture: each crop was rotated upright with `magick -rotate 90` and each label traced from its own arrowhead back to the half-byte it touches. No text-layer order was consulted anywhere in this leg, so the extraction-order trap could not arise.
- **(c) Leader-line label order reversed — ENCOUNTERED,** in four places, all of the same shape: the label sitting **higher** on the page belongs to the **right** half-byte, and the lower label to the left half-byte. On page 375: the `(4)` legend (left half-byte → the lower `0: Split OFF, 1: Split ON`; right half-byte → the upper `0: Select memory OFF` / `1: Select memory ON`) and the `(13)` legend (left → lower `0: Duplex OFF` / `1: Duplex–, 2: Duplex+`; right → upper `0: OFF, 1: Tone` / `2: TSQL, 3: DTCS`). On page 373: `• Digital code squelch setting` (left → lower `First digit: 0–9`; right → upper `Second digit: 0–9`). Each leader was followed by eye from label to the cell it lands on, and each reversal is stated in that row’s `notes`. `values_verbatim` keeps the printed top-to-bottom order in every case; it is not re-sorted into positional order.
- **(d) Printed index differs from measured position — ENCOUNTERED,** in the lower band. For every field of the receive part the printed index and the measured byte position agree (`1` at 1, `2、3` at 2–3, … `44~51` at 44–51). For the two groups after them they do not: the group printed with filled indices `5 ~ 51` occupies **measured byte positions 52–98**, and the group printed `52 ~ 60` (body text `52–67`) begins at **measured byte position 99**. Both readings are recorded for both groups — printed index in `field_index`, measured position in `notes` — and neither is reconciled to, or reinterpreted in the light of, the other. The consequences are logged as STOP 3 and STOP 4.

## STOP findings

1. **Memory channel number — two printed ranges for the same field contradict each other.**
   PDF page 375, folio 20-16. Anchor: left text column, second entry `(2), (3) Memory channel number`, first value line; against the `About clearing operation:` block in the right text column, its second line.
   What is printed: the field entry prints `0001–0099:  Memory channel 1 to 99`. The clearing block prints `(2), (3):        Memory channel 0 to 99`.
   Why it stops: the same two-byte field is given a range starting at channel **1** in one place and channel **0** in the other, and no code is printed for a channel 0 anywhere on the page. Both are transcribed as printed — the field row’s `values_verbatim` carries `0001–0099: Memory channel 1 to 99` and its `notes` carry the clearing block’s wording. Neither is repaired.

2. **Memory name field — the diagram band and the body text print different index spans and different widths.**
   PDF page 375, folio 20-16. Anchors: (i) lower band of the data block, the last group at the extreme right, its bracket labelled with outline circled numerals reading `52 ~ 60`; (ii) right text column, the last indexed entry before `About clearing operation:`, reading `(52)–(67) Memory name setting` with `16 characters (Fixed)` flush left beneath it.
   What is printed: the band prints `52 ~ 60` — nine bytes. The body text prints `52–67` and `16 characters (Fixed)` — sixteen bytes.
   Why it stops: nine and sixteen cannot both be the width of one field, and 52→60 and 52→67 cannot both be its span. **Both readings are transcribed, each in its own row** (`field_index` `52~60`, `width_bytes` 9; and `field_index` `52–67`, `width_bytes` 16), each anchored to the place it is printed. The two are not reconciled and neither is preferred. The band index `60` was re-read at 700 % on the 400 dpi raster and again on the 600 dpi raster; the body-text `67` was re-read at 250 % and again at 600 dpi. Both are unambiguous.

3. **Index sequence repeats: indices 5 to 51 are printed twice, in two different styles.**
   PDF page 375, folio 20-16. Anchor: lower band of the data block, the wide white dashed elision cell between the group bracketed `44~51` and the group bracketed `52~60`; its bracket is labelled with a solid black disc bearing a white `5`, a wave dash, and a solid black disc bearing a white `51`.
   What is printed: reading the whole band left to right, the index sequence runs `1 … 27` (upper band), then `28~35`, `36~43`, `44~51`, then **`5~51` again** in filled/reversed numerals, then `52~60`. The `NOTE:` box prints `The same data as (5)–(51) are stored in [filled]5–[filled]51.`
   Why it stops: it is both a repeat and an out-of-order index, and the repeated indices are drawn in a different style from their first appearance — three of the listed STOP triggers at once. The group is transcribed as printed (`field_index` `5~51`, style recorded in `notes`) and the `NOTE:` box wording is quoted in that row. Nothing is renumbered.

4. **A printed index disagrees with the position it occupies.**
   PDF page 375, folio 20-16. Anchors: the filled-index group described in STOP 3, and the group bracketed `52 ~ 60` immediately right of it.
   What is printed and what is measured: the filled group is printed `5 ~ 51` but, counted along the band from the first cell of the upper band, occupies **positions 52–98**. The group printed `52 ~ 60` begins at **position 99**, not 52. Measured by counting drawn cells and the byte spans of the elision cells: upper band 1–27, then `28~35`, `36~43`, `44~51` bring the count to 51, then the filled group’s 47 bytes bring it to 98.
   Why it stops: printed index and measured position cannot both be right, and the arithmetic of the record’s total length depends on which is taken. **Both are recorded and neither is reconciled** — printed index in `field_index`, measured position in `notes`, on both rows. No total length is asserted anywhere in this leg.

No value on any page was unreadable; there is no `UNREADABLE` cell in the CSV.

## Observed disagreements

Printed oddities that did not stop the transcription. Recorded as printed; not resolved.

1. **The band and the body text use different separators for the same index groups, systematically.** The band joins a range with a wave dash (`5 ~ 9`) and a pair with an ideographic comma (`2、3`); the body text joins the same range with an en dash (`(5)–(9)`) and the same pair with a comma and space (`(2), (3)`). Both forms are preserved in `field_index` and `notes`. This is a typographic difference, not a difference of content.
2. **Field `14` is the only cell in either band that prints a literal digit** — `X : 0` where every other cell prints `X : X`. Its legend box repeats `X : 0`, and only the left half-byte carries a leader. Nothing is printed anywhere about what the right half-byte means or whether a value other than `0` is accepted; the CSV records the enumerated codes for the left half-byte and states the silence.
3. **The transmit group has no label anywhere.** It appears in the band and in three `NOTE:` bullets, but there is no `[filled]5–[filled]51 <name>` entry in either text column, so `label_verbatim` for that row is empty. That absence is the finding.
4. **The cross-referenced heading is capitalised differently from the cross-reference.** Page 375 prints `(25)–(27) Duplex offset frequency setting` and, one line below, `See ‘• Duplex Offset frequency setting.’ (p. 20-13)`; the heading actually printed on page 372 is `• Duplex Offset frequency setting`, with a capital `O`.
5. **The command table names the record differently from the record page.** PDF page 364 (folio 20-5) prints `1A` / `00` / `see p. 20-16` / `Send/read the Memory channel contents`; page 375 heads the same material `• Memory content setting`.
6. **One referenced diagram serves two different fields.** Fields `15~17` (`Repeater tone frequency setting`) and `18~20` (`Tone squelch frequency setting`) both cross-reference the single diagram on page 373 headed `• Repeater tone/tone squelch frequency setting`, `Command: 1B 00, 1B 01`. Nothing distinguishing the two is printed against them, so their transcribed value sets read identically. They were transcribed independently, not copied from one another; the identity is what the page prints.
7. **Two glyphs in the page-371 character table are drawn in unexpected forms.** The character printed against code `7C` is a **broken** vertical bar, not a solid one; the character printed against `7E` is a short raised horizontal stroke rather than a tilde; the character printed against `2A` is a heavy asterisk; the character printed against `27` is a right single quotation mark. They are transcribed as drawn (`¦`, `‾`, `✱`, `’`) with the codes as printed.
8. **Four yellow highlight annotations sit on the pages read** — over `Bank` on page 375, over `bandwidth` on page 370, over `Duplex` and `UTC` on page 372, and over `Repeater`, `DTCS` and `call sign` on page 373. They are an annotation layer over the page, not printed ink, and they are noted only so that a second reader is not surprised by them.
9. **The band’s first group carries no bracket.** Index `1` is printed bare above a single cell, whereas every multi-cell group is bracketed. Single-cell groups `4`, `12`, `13`, `14` and `24` are likewise bare. Consistent within the diagram; noted because it is the only cue distinguishing a one-byte field from a group.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

(See the **Disclosures** paragraph in `## Method` for the one metadata call, `pdfinfo` on this same PDF, that supplied the page count, and for the `ls` calls on this PDF’s own path and on this leg’s own output directory. `pdftotext` was never run.)
