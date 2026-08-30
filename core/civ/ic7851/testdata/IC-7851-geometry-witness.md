# IC-7851 geometry witness — quarantine leg W

## Source

- Document title, as printed on the cover (PDF page 1): "THE TRANSCEIVERS" above "IC-7850" and "IC-7851", above "Instruction Manual".
- Revision code, as printed: `A7205H-1EX-3`. It is printed at the foot of the cover page (PDF page 1), lower right, on the first of three lines that read "A7205H-1EX-3" / "Printed in Japan" / "© 2015–2018 Icom Inc.".
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7851_civ_IM_3.pdf`
- Page count: 283 PDF pages.

The back cover (PDF page 283) carries no revision code; it prints only "Count on us!" at the head and "Icom Inc." with a Hirano-ku, Osaka address at the foot.

## Extent

PDF pages rendered and read:

| PDF page | Printed folio | Rendered at | What it contributed |
|---|---|---|---|
| 1 | none printed | 150 dpi | The `## Source` fields only: cover title and revision code. No measured value came from it. |
| 263 | `18-14` | 300, 400 and 600 dpi | Every measured value in the CSV. |
| 283 | none printed | 150 dpi | Checked for a revision code (there is none). No measured value came from it. |

No other page of the manual was rendered or read. The brief named PDF page 263; pages 1 and 283 were rendered solely because the `## Source` section this brief requires asks for the cover title and the revision code, which cannot be read from page 263. This is declared here so a second reader can see exactly what was opened. Adjacent pages 262 and 264 were **not** rendered, so this leg says nothing about what precedes or follows page 263 in the manual.

The running head on page 263 is "**18** CONTROL COMMAND"; the folio at the foot is `18-14`, which matches the brief's rule that the PDF page is 249 + n.

### The material transcribed, and what bounds it on page 263

Immediately before the transcribed material, at the top of the left column, three printed lines:

```
◇ Data content description (continued)
• Memory content setting
Command: 1A 00
```

Then the data-block band (diagram D1), which spans the full width of the type area across both columns. Immediately after it, in the left column, the prose entry "①, ② Memory channel numbers".

Diagram D2 sits in the right-hand column, beneath the prose heading "⑪ Data mode and tone type settings" and above "⑫~⑭ Repeater tone frequency setting".

### Diagram identifiers

- **D1** — printed caption, verbatim, over two lines: "• Memory content setting" / "Command: 1A 00". The wide horizontal band of byte cells directly beneath those two lines.
- **D2** — printed caption, verbatim: "⑪ Data mode and tone type settings". The single two-nibble box drawn beneath that heading in the right-hand column, with the circled numeral ⑪ printed above the box itself.

### Numbering convention used

D1 prints **no byte-position numerals** along its band: there is no numbered scale above, below or inside the cells, and no address is printed against any cell. Positions in the CSV are therefore counted as cells **from the diagram's own first (leftmost) cell**, which is recorded as byte 1, exactly as the brief directs for that case. D2 likewise prints no byte numerals; its single cell is counted as byte 1 of D2.

D1 draws **18 cells** in total. Sixteen of them contain `X`, a dotted vertical rule, `X`. Two of them (drawn cells 5 and 17) contain a printed ellipsis `...` instead of any `X`, and have dashed rather than solid top and bottom rules. Those two ellipsis cells have each been counted as one drawn cell, because one cell is what is drawn; see STOP 3 for what that does and does not license.

### Nibble labelling in the document

Neither D1 nor D2 prints a nibble number, a nibble name, or any word such as "nibble", "upper", "lower", "high" or "low". The only thing that marks a nibble in either diagram is a **dotted vertical rule** halving each byte cell between its two `X` glyphs. In D2 the two halves are additionally distinguished by two arrows and two horizontal leader lines running to two value legends; those legends carry no index numeral. All nibble numbers in the CSV are therefore the recording convention the brief supplies (1 = the half printed first, i.e. leftmost; 2 = the other), not a label read off the page.

### Diagrams on page 263 that were judged out of scope

Two further diagrams appear lower on page 263, and were deliberately **not** transcribed, because neither is a memory-record data block:

- Under "• Main or Sub band's frequency settings" / "Command : 25" — a band of cells whose only labels are two arrows pointing to "00: MAIN / 01: SUB" and "Operating frequency data (See p. 18-11)". It prints no circled index numerals at all.
- Under "• Main or Sub band's operating mode and filter settings" / "Command : 26" — a band of four cells with circled ①, ② and ③ and an accompanying three-column table ("① Operating mode", "② Data mode", "③ Filter"). It carries circled indices, but it is a mode/filter command frame, not the memory record.

This exclusion is a judgement call and is flagged as such so it can be reversed by a follow-up dispatch if the caller intended a wider net. Nothing from either diagram has been measured, and no value from either appears in the CSV.

## Method

Every value in the CSV was read from a rendered page image of this PDF. No text extractor was run at any point.

**dpi at each step**

1. **Locate — 300 dpi.** Into a fresh directory created for this leg:
   `pdftoppm -png -r 300 -f 263 -l 263 <pdf> r300/p`
   Read as an image to find the section whose printed heading matches, and to confirm the folio (`18-14`) and running head ("**18** CONTROL COMMAND").
2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f 263 -l 263 <pdf> r400/p` → 3308 × 4678 px. Every first-pass value was read from crops of this raster.
3. **Second pass — 600 dpi.** `pdftoppm -png -r 600 -f 263 -l 263 <pdf> r600/p` → 4961 × 7016 px.
4. Cover and back cover — 150 dpi, for the `## Source` fields only.

**ImageMagick** was available (`/opt/homebrew/bin/magick`, and `convert` also present) and was used for every crop and enlargement. The commands, verbatim:

First pass, on the 400 dpi raster:

```
magick r400/p-263.png -crop 2500x260+480+690  +repage -resize 200% crops/D1_full.png
magick r400/p-263.png -crop  700x250+520+700  +repage -resize 400% crops/D1_seg1.png
magick r400/p-263.png -crop  700x250+1180+700 +repage -resize 400% crops/D1_seg2.png
magick r400/p-263.png -crop  700x250+1840+700 +repage -resize 400% crops/D1_seg3.png
magick r400/p-263.png -crop  700x250+2400+700 +repage -resize 400% crops/D1_seg4.png
magick r400/p-263.png -crop  900x160+560+710  +repage -resize 500% crops/idx_A.png
magick r400/p-263.png -crop  900x160+1400+710 +repage -resize 500% crops/idx_B.png
magick r400/p-263.png -crop  900x160+2100+710 +repage -resize 500% crops/idx_C.png
magick r400/p-263.png -crop 1450x430+1640+950 +repage -resize 250% crops/D2_full.png
magick r400/p-263.png -crop  620x300+1750+1110 +repage -resize 500% crops/D2_leaders.png
magick r400/p-263.png -crop 1200x260+180+490  +repage -resize 300% crops/caption.png
magick r400/p-263.png -crop  700x120+190+1990 +repage -resize 400% crops/prose_48.png
magick r400/p-263.png -crop  800x230+1640+1390 +repage -resize 400% crops/prose_1217.png
magick r400/p-263.png -crop 1350x330+180+1200 +repage -resize 280% crops/note_clear.png
```

Second pass, on the 600 dpi raster:

```
magick r600/p-263.png -crop 2000x400+880+1040  +repage -resize 175% crops/P2_left.png
magick r600/p-263.png -crop 2000x400+2500+1040 +repage -resize 175% crops/P2_right.png
```

Every numeral, rule and glyph in the enlarged crops sits clear of its neighbours; no value had to be read from an unenlarged whole-page render.

**`pdftotext`** — **not run**, in any form, at any point. It was not needed: the section was located by reading the 300 dpi render. The `## Attestation` below is therefore the first (renders-only) form.

**`pdfinfo`** — run once on this same PDF, for the "283 pages" figure quoted in `## Source` only. It reads the file's metadata, not its page content; it was the source of no measured value.

**`tesseract`** — available at `/opt/homebrew/bin/tesseract` but **not used**. Every glyph on this page was legible by eye once enlarged, so no OCR aid was invoked and no OCR value was recorded.

### Second independent pass — record

Both passes were done.

- **Pass 1** — 400 dpi raster, band cropped in four 700 px windows at 400 % enlargement (`D1_seg1`–`D1_seg4`), then re-checked with three differently-placed 900 px windows at 500 % enlargement (`idx_A`, `idx_B`, `idx_C`); D2 at 250 % and its leaders at 500 %.
- **Pass 2** — a different raster in every respect: **600 dpi** instead of 400, **two** 2000 px windows instead of four/three, at **175 %** enlargement instead of 400 %/500 %, with crop boundaries deliberately placed so that no pass-2 window edge falls where a pass-1 window edge fell. Cells were re-counted from the band's own left end and each bracket's feet re-located, without reference to the pass-1 figures.
- **Disagreements: none.** Pass 2 returned the same 18 drawn cells in the same order, the same two ellipsis cells at ordinals 5 and 17, the same shading pattern, and the same eight bracket spans. As a numerical cross-check, the cell-boundary positions independently located in pass 2 agreed with pass 1 to within about 1–2 px at 400 dpi equivalent, against a cell width of about 110 px — i.e. two orders of magnitude inside one cell. No third render was needed.
- D2's two leader lines were followed by eye on both the 250 % pass-1 crop and the 500 % `D2_leaders` crop, and gave the same assignment both times.

## Position arithmetic, per diagram

Positions below are counted drawn cells, 1-based from each diagram's own first cell, as described under `## Extent`. "Extent" is the number of drawn cells the bracket or numeral covers.

### D1 — "• Memory content setting" / "Command: 1A 00"

Running total starts at byte 1.

| Printed index | Starts at byte | Measured extent (drawn cells) | Ends at byte | Next field starts at | Printed index count | Agrees? |
|---|---|---|---|---|---|---|
| ①, ② | 1 | 2 | 2 | 3 | 2 | yes |
| ③ | 3 | 1 | 3 | 4 | 1 | yes |
| ④–⑧ | 4 | 3 (cells 4, 5 = `...`, 6) | 6 | 7 | 5 | **NO — STOP 1** |
| ⑨, ⑩ | 7 | 2 | 8 | 9 | 2 | yes |
| ⑪ | 9 | 1 | 9 | 10 | 1 | yes |
| ⑫–⑭ | 10 | 3 | 12 | 13 | 3 | yes |
| ⑮–⑰ | 13 | 3 | 15 | 16 | 3 | yes |
| ⑱–㉗ | 16 | 3 (cells 16, 17 = `...`, 18) | 18 | — (band ends) | 10 | **NO — STOP 2** |

Sums, written out:

- 1 + 2 = 3 → ③ starts at 3. ✔
- 3 + 1 = 4 → ④–⑧ starts at 4. ✔
- 4 + 3 = 7 → ⑨, ⑩ starts at 7. ✔ (arithmetic on drawn cells only)
- 7 + 2 = 9 → ⑪ starts at 9. ✔
- 9 + 1 = 10 → ⑫–⑭ starts at 10. ✔
- 10 + 3 = 13 → ⑮–⑰ starts at 13. ✔
- 13 + 3 = 16 → ⑱–㉗ starts at 16. ✔
- 16 + 3 = 19 → one past the last drawn cell; the band ends at drawn cell 18. ✔

Total drawn cells: 2 + 1 + 3 + 2 + 1 + 3 + 3 + 3 = 18, which matches the 18 cells counted directly along the band in both passes. The eight bracket/numeral spans tile the band with **no gap and no overlap**: every one of the 18 cells belongs to exactly one printed index or index range, and the feet of consecutive brackets land on the same cell edge each time.

Against the printed indices, the same band totals 2 + 1 + 5 + 2 + 1 + 3 + 3 + 10 = **27**, ending at ㉗, which is a continuous sequence ① … ㉗ with no gap, no repeat and no out-of-order index. The document therefore prints 27 indices over 18 drawn cells. Six of the eight groups run at exactly one index per drawn cell; the two that do not are exactly the two groups containing an ellipsis cell. **Both figures are recorded; neither is resolved, and no ellipsis cell has been assigned a byte count.**

### D2 — "⑪ Data mode and tone type settings"

D2 draws exactly one byte cell, halved by a dotted vertical rule.

| Field | Starts at | Measured extent | Ends at | Next starts at |
|---|---|---|---|---|
| ⑪ (whole box) | byte 1, nibble 1 | 1 byte = 2 nibbles | byte 1, nibble 2 | — (diagram ends) |
| left half (no index printed) | byte 1, nibble 1 | 1 nibble | byte 1, nibble 1 | byte 1, nibble 2 |
| right half (no index printed) | byte 1, nibble 2 | 1 nibble | byte 1, nibble 2 | — (diagram ends) |

Sums: 1 nibble + 1 nibble = 2 nibbles = the 1 byte the box draws, and the ⑪ span over the whole box covers both halves with no gap and no overlap. ✔

D2's own byte 1 is the same printed byte as D1's drawn cell 9. That correspondence is stated here only as an observation about which cell D2 expands; the CSV records D2's positions within D2's own diagram, as counted, and does not carry D1's ordinal across.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every index numeral in D1 and D2 is drawn in one single style: a black outlined circle (an outlined oval for the two-digit ⑩, ⑪ … ㉗), with a plain black numeral inside it, on white. Checked glyph by glyph at 400 dpi/500 % and again at 600 dpi. None is filled or reversed, none is bracketed, none is bold, none is plain-uncircled. The circled ⑪ over D1's drawn cell 9, the circled ⑪ above D2's box and the circled ⑪ in D2's prose heading are all the same style. The only styling variation found anywhere near the indices is in the *range separator*, not the numerals — see `## Observed disagreements`.
- **(b) Diagrams may be vector groups with rotated labels — NOT ENCOUNTERED.** No rotated, vertical or mirrored label appears in D1 or D2; every numeral and every legend is set horizontally, left to right. I make no claim about the underlying text layer's extraction order, because no text extractor was run in this leg; the question does not arise, since every value here was read off the picture as the method requires.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In D2, the two legends are printed one above the other, but they attach to the nibbles in the opposite order to the order they are printed. The **upper** legend, "0: OFF, 1: TONE, 2: TSQL", is joined by a short leader to the arrow rising into the **right-hand** (second-printed) nibble. The **lower** legend, "0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3", is joined by a long leader running left along the page to the arrow rising into the **left-hand** (first-printed) nibble, whose stem descends past the level of the upper leader to reach it. Each leader was followed by eye from label to arrowhead on two separate rasters (400 dpi at 250 % and at 500 %; the same assignment both times). The two horizontal leaders do not physically intersect — the upper one begins to the right of the lower one's vertical stem — so what is reversed is the *reading order of the labels*, not the drawn lines. Taking the printed labels in top-to-bottom order and assigning them to the nibbles in left-to-right order would give the wrong nibble for both legends.
- **(d) A printed index may differ from a field's measured position — ENCOUNTERED.** Two groups in D1 print more indices than the diagram draws cells for them: ④–⑧ (five printed indices, three drawn cells) and ⑱–㉗ (ten printed indices, three drawn cells). Both are recorded twice over — the printed index exactly as printed in `field_index`, and the measured drawn-cell positions in the four position columns — and the two are not reconciled anywhere in this leg. See STOP 1, STOP 2 and STOP 3. Separately, D1 contains two structurally identical three-cell blocks, ⑫–⑭ (three white cells) and ⑮–⑰ (three grey cells); each was measured independently on both passes rather than one being assumed to match the other, and each independently came to three drawn cells. The circled ⑪ likewise occurs both in D1's band and as D2's own label, and each occurrence was measured within its own diagram rather than carried across.

## STOP findings

1. **PDF page 263 — D1 band, the bracket labelled ④–⑧ and the three drawn cells beneath it (grey `X:X`, then a grey cell printed `...` with dashed top and bottom rules, then grey `X:X`; the cell immediately to their left is the white cell labelled ③).** What is printed: a bracket labelled "④–⑧", i.e. five index numerals, whose feet land on the left edge of drawn cell 4 and the right edge of drawn cell 6 — an extent of three drawn cells. Why it stops: the measured extent does not agree with the printed index count. Everywhere else in this same diagram the two agree exactly, one index per drawn cell (①,② over 2 cells; ③ over 1; ⑨,⑩ over 2; ⑪ over 1; ⑫–⑭ over 3; ⑮–⑰ over 3), so the mismatch here is a disagreement inside the document's own convention, not an artefact of how I counted. Both figures stand as found. CSV row: `D1,④–⑧` recorded as measured, `4,1` to `6,2`, `notes` = `STOP 1 | STOP 3`.
2. **PDF page 263 — D1 band, the bracket labelled ⑱–㉗ and the three drawn cells beneath it at the right-hand end (white `X:X`, then a white cell printed `...` with dashed top and bottom rules, then white `X:X`, which is the last cell of the band).** What is printed: a bracket labelled "⑱–㉗", i.e. ten index numerals, whose feet land on the left edge of drawn cell 16 and the right edge of drawn cell 18 — an extent of three drawn cells. Why it stops: as STOP 1, the measured extent does not agree with the printed index count. Also printed nearby, in the right-hand column under the heading "⑱~㉗ Memory name settings": "Up to 10 characters." That line is recorded here as part of what the page prints beside this bracket; it is **not** used to derive, adjust or confirm any position, and the three drawn cells stand as measured. CSV row: `D1,⑱–㉗` recorded as measured, `16,1` to `18,2`, `notes` = `STOP 2 | STOP 3`.
3. **PDF page 263 — D1 band, drawn cells 5 and 17, the two cells that print an ellipsis `...` instead of `X`, with dashed top and bottom rules (cell 5 grey-filled, inside the ④–⑧ bracket; cell 17 unfilled, inside the ⑱–㉗ bracket).** What is printed: a single cell in each case, containing an ellipsis and no `X`, with no numeral, no count and no width printed anywhere against it to say how many bytes it abbreviates. Why it stops: every position counted to the right of drawn cell 5 is therefore a **drawn-cell ordinal, not an asserted byte address** — if either ellipsis cell stands for more than one omitted byte, every position from `④–⑧` rightwards shifts, and nothing printed on this page fixes by how much. Counting each ellipsis cell as one cell is what the picture draws and is what has been recorded; it is not a claim that one byte is what it means. No interpolation, averaging or carry-over has been performed to close the gap. Every affected row carries `STOP 3` in `notes`: `④–⑧`, `⑨, ⑩`, `⑪`, `⑫–⑭`, `⑮–⑰` and `⑱–㉗`. The rows for `①, ②` and `③` are to the left of the first ellipsis and are unaffected.

No `UNREADABLE` cell appears in the CSV: every glyph, rule, bracket foot and leader on this page was read with confidence at 400 dpi enlarged, and confirmed at 600 dpi.

## Observed disagreements

Recorded exactly as printed; not resolved.

1. **The range separator differs between diagram and prose, for every range on the page.** Inside the D1 band the ranges are set with a straight horizontal dash — "④–⑧", "⑫–⑭", "⑮–⑰", "⑱–㉗". In the prose entries beneath, the same ranges are set with a wave dash — "④~⑧ Operating frequency setting", "⑫~⑭ Repeater tone frequency setting", "⑮~⑰ Tone squelch frequency setting", "⑱~㉗ Memory name settings". The endpoints are identical in every case, so this did not stop me. The CSV's `field_index` values reproduce the **diagram's** form, since the diagram is what was measured.
2. **The two-index pairs use a comma in both places, and are not ranges.** "①, ②" and "⑨, ⑩" are printed with a comma and a space in the band and in the prose alike, whereas the three-or-more groups use a dash/wave dash. Recorded verbatim.
3. **The two ellipsis cells are drawn differently from one another.** Drawn cell 5 (inside ④–⑧) is grey-filled; drawn cell 17 (inside ⑱–㉗) is unfilled. Both have dashed top and bottom rules and both print `...`. Nothing on the page says whether the fill difference carries meaning; it follows the shading of the neighbouring cells in each group, which are grey around cell 5 and white around cell 17.
4. **D2's heading orders its two fields the opposite way to its legend stack.** The heading reads "⑪ Data mode and tone type settings" — data mode first — and the leaders do put the data-mode legend on the first (left) nibble. But the legend block beneath the box prints the *tone* legend on the upper line and the *data mode* legend on the lower line, i.e. tone first. Nothing is contradicted once the leaders are followed, but a reader who took the legend stack's order as the nibble order would invert both assignments. This is the same fact as hazard (c), recorded here because it is a disagreement between two things printed on the same page.
5. **A prose note names an index range that is not one of the bracketed groups.** In the left-hand column, the shaded note reads: "To clear the memory channel contents, add the code "FF" after the memory channel number. (instead of the data ③ to ㉗) This completes the memory clearing." The range "③ to ㉗" spans across several of the band's bracketed groups and uses the word "to" rather than either dash form. It is consistent with the band (③ is the first index after the ①, ② memory-channel-number pair, and ㉗ is the last index printed), and it is recorded here only because it is a third way of writing a range on one page.
6. **D1's caption is split over two printed lines.** "• Memory content setting" and "Command: 1A 00" are separate lines; the CSV cannot hold an embedded newline, so where the caption is referred to in a cell the two lines are joined with a single space, per the CSV rules.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

Two disclosures, so that the sentence above is read exactly as true and not one word wider:

- `pdfinfo` was run once on this same PDF, and is where the "283 pages" figure in `## Source` comes from. It is not `pdftotext`, it reads no page content, and it was the source of no measured value, no numeral, no index and no position in the CSV. Every measured value came from a render.
- The only directories listed were the render and crop subdirectories that this leg itself created inside `/private/tmp/claude-501/-Users-stuart-Documents-working-coding-ft710-programmer-nosync/b1f63348-8eaa-4174-bb05-d1f10e3b04fb/scratchpad/legs-out/ic7851/W`, to confirm that `pdftoppm` and `magick` had written the files expected. No directory outside that output directory was listed, searched or browsed, and the repository was not touched.
