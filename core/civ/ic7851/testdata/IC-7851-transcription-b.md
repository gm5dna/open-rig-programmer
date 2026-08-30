# IC-7851 — quarantine leg B — memory-record transcription

Companion to `IC-7851-transcription-b.csv` (11 rows, 9 columns, RFC 4180, UTF-8, no BOM).

## Source

- Title as printed on the cover (PDF page 1, centred): `THE TRANSCEIVERS` / `IC-7850` / `IC-7851` / `Instruction Manual`.
- Revision code as printed: `A7205H-1EX-3`, set in small type at the **foot of the right-hand side of the cover page (PDF page 1)**, on the first of three lines; the two lines beneath it read `Printed in Japan` and `© 2015–2018 Icom Inc.`
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7851_civ_IM_3.pdf`
- Page count: 283 PDF pages.
- The chapter carrying this material is `18 CONTROL COMMAND`; its running head appears on every page read. Folios are printed as `18-n` and PDF page = 249 + n.

## Extent

Rendered at 300 dpi (survey pass): PDF pages **252–263**, plus PDF page 1 for the cover. Re-rendered at 400 dpi: 260–263. Re-rendered at 500 dpi: 263. Re-rendered at 600 dpi: 260, 261, 262, 263 and page 1.

| PDF page | Printed folio | Read how | What it contributed |
|---|---|---|---|
| 1 | (no folio; cover) | 150 dpi survey, 600 dpi crop of the foot | Cover title and the revision code `A7205H-1EX-3`. Nothing else. |
| 252 | 18-3 | 300 dpi, top band inspected | Heading `◇ Command table`. Contributed nothing to any row. |
| 253 | 18-4 | 300 dpi, top band inspected | `◇ Command table (continued)`. Nothing. |
| 254 | 18-5 | 300 dpi, top band inspected | `◇ Command table (continued)`. Nothing. |
| 255 | 18-6 | 300 dpi, top band inspected | `◇ Command table (continued)`. Nothing. |
| 256 | 18-7 | 300 dpi, top band inspected | `◇ Command table (continued)`. Nothing. |
| 257 | 18-8 | 300 dpi, top band inspected | `◇ Command table (continued)`. Nothing. |
| 258 | 18-9 | 300 dpi, top band inspected | `◇ Command table (continued)`. Nothing. |
| 259 | 18-10 | 300 dpi, whole page read | `◇ Command table (continued)`; the last page before the data-content chapter. Contributed nothing to any row; establishes what precedes the transcribed material. |
| 260 | 18-11 | 300 / 400 / 600 dpi, cropped and enlarged | The `◇ Data content description` heading (the section opens here); the `• Operating frequency` diagram (values for D1 `④~⑧`) and the `• Operating mode` diagram and table (values for D1 `⑨, ⑩`). |
| 261 | 18-12 | 300 / 400 / 600 dpi, cropped and enlarged | The character table (values for D1 `⑱~㉗`) — see below. |
| 262 | 18-13 | 300 / 400 / 600 dpi, cropped and enlarged | The `• Repeater tone/tone squelch frequency settings` diagram (values for D1 `⑫~⑭` and `⑮~⑰`). |
| 263 | 18-14 | 300 / 400 / 500 / 600 dpi, cropped and enlarged | **The material transcribed**: the `• Memory content setting` / `Command: 1A 00` data-block diagram (D1) and the `⑪ Data mode and tone type settings` sub-diagram (D2), with the numbered field text of both columns. |

**Where the transcribed material begins and ends.** Immediately before it, at the top of PDF page 263 under the running head `18 CONTROL COMMAND`, are the three lines `◇ Data content description (continued)`, `• Memory content setting`, `Command: 1A 00`; the D1 byte band is drawn directly beneath them. The material ends with the last block of the right-hand column, `⑱~㉗ Memory name settings` / `Up to 10 characters.` / the `See "• Codes for the memory name, …"` cross-reference. Immediately after it, in the left-hand column of the same page, comes `• Main or Sub band's frequency settings` / `Command : 25`. Nothing of the memory record runs onto PDF page 264, and PDF page 264 was neither rendered nor opened.

**The character table — was it printed at all?** Yes. PDF page 261 (folio 18-12), right-hand column, prints `• Codes for the memory name, opening message, NTP server address, CLOCK2 name, network name, and network radio name contents`, and under it two tables, `- Character codes— Letters` (one data row: `A–Z / 41–5A`, `a-z / 61–7A`) and `- Character codes— Symbols` (four columns, sixteen data rows, thirty-two symbol/code pairs), followed by a `Command / Set item/usable characters` table whose first row is `1A 00 / Memory name / All characters are usable.` This is the page the `⑱~㉗` cross-reference points at, and it is the sole source of that row's `values_verbatim`. What it does **not** print is a code for the digits `0–9` or for the space character — an absent statement, recorded as such below and not filled in from anywhere.

**The set-mode / menu pages.** PDF pages 252–262 were rendered. They are not set-mode/menu material: PDF pages 252–259 (folios 18-3 to 18-10) are the `◇ Command table` and its continuations, and PDF pages 260–262 (folios 18-11 to 18-13) are `◇ Data content description`. Of these, only 260, 261 and 262 contributed anything, and only through the three cross-references the memory-record fields themselves print (`• Operating frequency`, `• Operating mode`, `• Repeater tone/tone squelch settings`, and the memory-name codes). Pages 252–259 contributed nothing to any cell.

## Method

Page images only. Every value in the CSV was read from a rendered page image; nothing was read from a text layer.

1. **Survey, 300 dpi.** Into a fresh directory `…/legs-out/ic7851/B/r300` created for this leg:
   `pdftoppm -png -r 300 -f 252 -l 263 <pdf> r300/p`
   and `pdftoppm -png -r 150 -f 1 -l 1 <pdf> r300/cover`. Read as images to locate the `• Memory content setting` block and the sections its fields refer to. The headings across PDF pages 260–262 do resemble one another (`• Operating frequency` and `• Operating mode` on 260; `• Offset frequency settings` and `• Bandscope edge frequency settings` on 261; `• Repeater tone/tone squelch frequency settings` and `• Band edge frequency settings` on 262), so each section was matched on its full printed heading, on the page image, before anything was read out of it.
2. **Read, 400 dpi and above.** `pdftoppm -png -r 400 -f 260 -l 263`, `-r 500 -f 263 -l 263`, `-r 600 -f 260 -l 263` and `-r 600 -f 1 -l 1`, into `r400/`, `r500/`, `r600/`. All recorded values were read at 400 dpi or higher.
3. **Crop and enlarge.** ImageMagick **was** available (`/opt/homebrew/bin/magick`, and `convert`) and was used throughout. Representative commands (all `+repage` after cropping, as required):
   - band of the D1 diagram: `magick r600/p-263.png -crop 3500x420+780+1020 +repage crops/d1_band_600.png`, then thirds at `-crop 1200x420+0+0 / +1150+0 / 1250x420+2250+0 +repage -resize 200%`;
   - whole band for cell counting: the same crop `-resize 1750x210`;
   - numbered text blocks: `-crop 2200x1000+280+1380 +repage -resize 150%` (and four sibling windows) for the left column, `-crop 2200x700+2450+1380 +repage -resize 180%` for the `⑪` block, `-crop 2200x1200+2450+2050 +repage -resize 150%` for `⑫~⑭` / `⑮~⑰` / `⑱~㉗`;
   - the crossed leader lines: `magick r600/p-263.png -crop 750x400+2680+1640 +repage -resize 400%`;
   - referenced diagrams: `-crop 2100x1500+280+1100 +repage -resize 130%` and `-crop 1400x700+320+3100 +repage -resize 200%` on `r600/p-260.png`, `-crop 2050x1350+400+860 +repage -resize 150%` on `r600/p-262.png`, and four windows on `r600/p-261.png` at `-resize 160%` with two rows re-cropped at `-resize 350%`.
   Every numeral, rule and glyph recorded was enlarged until it stood clear of its neighbours.
4. **`pdftotext`.** `pdftotext` was **not run at all**, in any form, on this or any other file. No text layer was consulted.
5. **`tesseract`.** Available at `/opt/homebrew/bin/tesseract`. It was **not used**. No value in the CSV came from OCR; every value was read by eye off an enlarged render.
6. **Second independent pass.** Done, and it was a genuinely different raster. The first pass read the D1 band from a **600 dpi** colour render, cropped into three windows at `+780+1020` and enlarged 200 %; the `⑪` sub-diagram from that same 600 dpi render at 400 %. The second pass read the same material from a **500 dpi** render of the page, converted to **greyscale**, **unsharp-masked** (`-colorspace Gray -sharpen 0x1`), cut into **four** overlapping windows on different boundaries (`-crop 780x360+640+840`, `+1320`, `+2000`, `+2680`) and enlarged **250 %**, so that no cell boundary or bracket end fell where it had in pass one; the `⑪` sub-diagram was re-read from the 500 dpi greyscale render at 260 %. The second pass was carried out by re-deriving cell shading, cell count, bracket span, leader landing and every printed value from those images.
   **Disagreements between the two passes: none.** Both passes independently returned: eighteen drawn cells; group shading grey-grey / white / grey-elision-grey / white-white / grey / white-white-white / grey-grey-grey / white-elision-white; bracket ends on drawn cells 2, 6, 8, 12, 15 and 18; and, for the `⑪` sub-diagram, the left cell landing on the lower text line and the right cell on the upper text line. (`③` and `⑪` are labelled by a bare circled numeral above a single cell, with no bracket; the other six groups each carry a bracket with a hooked end at both limits.) The referenced diagrams on PDF pages 260 and 262 and the tables on 261 were likewise read twice, at 300 dpi and again at 600 dpi cropped and enlarged, with no disagreement.
7. **Other tools, declared for completeness.** `pdfinfo` was run once on this same PDF to confirm the page count (283) already stated in the brief; it reads that one PDF's own container metadata, it is not a text extractor, and it was the source of no transcribed value. `magick identify`, `file`, `awk`, `head` and `ls` were used only on the renders and the CSV that this leg itself created inside its own output directory, and a short Python script (`_build_csv.py`, written by this leg) emitted and round-tripped the CSV. No document other than the single PDF named in the brief was opened at any point, and no directory outside this leg's own output directory was listed. The Attestation below is stated on that basis.

## Hazards encountered

- **(a) Numeral styling varying within one diagram — NOT ENCOUNTERED.** Every index in D1 is drawn the same way: an outline circle enclosing plain digits, one digit for `①`–`⑨` and two for `⑩`–`㉗`, at one size, none filled, reversed, bracketed or bold. The body-text repetitions of the same indices, and the `③ to ㉗` inside the "FF" clearing note, use that identical style. The `⑪` of the D2 sub-diagram matches. Checked at 600 dpi and again on the 500 dpi greyscale pass.
- **(b) Vector groups with rotated labels — ENCOUNTERED, on the cross-referenced diagrams only.** The D1 band on PDF page 263 has no rotated text. The two diagrams its fields refer to do: on PDF page 260 (`• Operating frequency`) and PDF page 262 (`• Repeater tone/tone squelch frequency settings`), every digit-place label is set rotated 90° and reads bottom-to-top beneath its own arrow. Each label was matched to its cell by following its arrow upward by eye on the render. No text layer was read at any point, so extraction order could not have misled this leg.
- **(c) Leader-line label order reversed — ENCOUNTERED.** In the `⑪ Data mode and tone type settings` sub-diagram (D2) the two leaders run to the two text lines in the reverse of their printed order. The **left** (first) nibble's arrow drops from the cell, continues down past the upper line's horizontal rule, and turns right onto the **second, lower** line `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`. The **right** (second) nibble's arrow drops a short distance and turns right onto the **first, upper** line `0: OFF, 1: TONE, 2: TSQL`. Followed by eye from cell to line at 400 %, and confirmed independently on the 500 dpi greyscale raster at 260 %. This is what the CSV records; taking the lines in printed order would have swapped the two nibbles.
- **(d) Printed index differing from measured position — NOT ENCOUNTERED.** D1 contains a repeated block: `⑫~⑭` (repeater tone frequency) and `⑮~⑰` (tone squelch frequency) are two three-byte blocks with the same structure, sent to one and the same referenced diagram. Both the printed index and the position measured on the render are recorded in `notes` for every D1 row. Measured by counting drawn cells and expanding the two elisions: `①,②` = bytes 1–2; `③` = 3; `④~⑧` = 4–8; `⑨,⑩` = 9–10; `⑪` = 11; `⑫~⑭` = 12–14; `⑮~⑰` = 15–17; `⑱~㉗` = 18–27. Every printed index equals its measured position, and 2+1+5+2+1+3+3+10 = 27, which is the highest printed index. Nothing was reconciled, because nothing needed to be.

## STOP findings

**None.**

Reasons for that confidence, item by item against the five STOP triggers:

1. *Arithmetic.* The eight printed field widths sum to 27 and the last printed index is `㉗`. The eighteen drawn cells account for exactly those 27 bytes with the two dashed elision cells expanded (3 drawn cells spanning 5 bytes for `④~⑧`; 3 drawn cells spanning 10 bytes for `⑱~㉗`); the remaining 12 drawn cells are one byte each. No overlap, no gap, no leftover cell. The referenced diagrams corroborate the widths they are referenced for without being needed to establish them: 5 bytes on PDF page 260 for the operating frequency, 2 bytes on PDF page 260 for mode+filter, 3 bytes on PDF page 262 for a tone frequency, and `Up to 10 characters.` printed on PDF page 263 for the name.
2. *Legibility.* Every numeral, bracket end, shading boundary, dotted nibble rule and leader was read at 400 dpi or higher and enlarged 150–400 % until clear of its neighbours; nothing was recorded as `UNREADABLE`.
3. *Index sequence.* `①,② ③ ④~⑧ ⑨,⑩ ⑪ ⑫~⑭ ⑮~⑰ ⑱~㉗` is strictly ascending, contiguous, with no repeat, no gap, no out-of-order entry and one styling throughout. The `⑪` that heads the D2 sub-diagram is the same field as D1's `⑪`, printed once in each diagram, not a second `⑪` inside one diagram.
4. *Two-pass disagreement.* None arose; see `## Method` step 6.
5. *Printed contradictions.* The oddities found are recorded under `## Observed disagreements`; each is a difference in wording, glyph or scope between two places, not two statements that cannot both be true of the same field, so none of them stopped the transcription.

## Observed disagreements

Recorded exactly as printed; not resolved.

1. **The range separator differs between the diagram and the body text.** The brackets over the D1 band print `④—⑧`, `⑫—⑭`, `⑮—⑰`, `⑱—㉗` with a horizontal dash; the body text under the same diagram prints `④~⑧`, `⑫~⑭`, `⑮~⑰`, `⑱~㉗` with a swung dash. The pair forms agree (`①, ②` and `⑨, ⑩` in both). The CSV carries the body-text form, because that is the printing that also carries the label and the values.
2. **The cross-reference names a heading that is not the heading printed.** PDF page 263 prints `See "• Repeater tone/tone squelch settings."`; the heading actually printed on PDF page 262 is `• Repeater tone/tone squelch frequency settings`. PDF page 260 prints a third variant of the same reference, `See "• Repeater tone/tone squelch setting."` Three wordings, one target.
3. **The Letters table mixes dash forms.** PDF page 261 prints `A–Z` with an en dash but `a-z` with a hyphen, while both code cells use en dashes (`41–5A`, `61–7A`).
4. **Two symbol glyphs do not look like the ASCII characters their codes name.** PDF page 261 prints, against ASCII code `2A`, a heavy six-pointed asterisk, and against ASCII code `7E`, a short raised bar sitting at cap height. Transcribed as drawn (`✻`, `‾`).
5. **The memory-name character tables print no code for the digits and none for space,** yet the same page's `1A 00 / Memory name` row states `All characters are usable.` The separate `• Codes for CW message contents` table on PDF page 262 does print `0–9 / 30–39` and `Space / 20`, but that table is headed `Command: 17` and is not the one the `⑱~㉗` field refers to. Nothing was carried across.
6. **The operating-mode code list has holes.** PDF page 260 prints `00 01 02 03 04 05 07 08 12 13`; codes `06`, `09`, `10` and `11` are not printed, and no note explains their absence.
7. **A conditional printed for a different command.** PDF page 262 marks the first byte of the tone-frequency diagram `①*` and prints `*Not necessary when setting a frequency.`, under the heading `Command: 1B 00, 1B 01`. The `1A 00` diagram on PDF page 263 draws three full bytes for `⑫~⑭` and three for `⑮~⑰` and prints no such asterisk or note. No conditional width was therefore recorded for those fields; the condition as printed is scoped to command 1B.
8. **A skip allowance printed for a different command.** PDF page 260 prints `Filter setting (②) can be skipped with command 01 and 06.`, and the `Command : 26` diagram lower on PDF page 263 prints `Both data and filter settings can be skipped.` Neither statement is printed against `1A 00`, whose diagram draws `⑨` and `⑩` as two full bytes.
9. **Colon spacing is inconsistent on PDF page 263.** `Command: 1A 00` (no space before the colon) against `Command : 25` and `Command : 26` (space before it) lower on the same page; and `③`'s values are printed `00 : OFF`, `01 : ★1` with a space either side, where `①, ②`'s are printed `0100:` with none.
10. **Digit-place labels are spaced inconsistently.** PDF page 262 prints `100Hz digit: 0–2` closed up, between `10 Hz digit: 0–9` and `1 Hz digit: 0–9`, which are not.
11. **The same one-byte construction is headed and labelled differently on PDF page 260, and the index `⑪` is printed there twice.** In the band-stacking-register block on PDF page 260 (`• Band stacking register`, `Command: 1A 01`) the heading reads `⑩ Data mode and tone setting` and `1 byte data (XX)`, but the byte box drawn immediately beneath that heading is labelled `⑪`; and the two headings that follow it on the same page are `⑪–⑬ Repeater tone frequency setting` and `⑭–⑯ Tone squelch frequency setting`, so `⑪` appears once as the `⑩` block's byte label and again as the first index of the repeater-tone range. That page-260 block belongs to a different command and none of its rows were transcribed, so this is not a STOP against anything in the CSV; it is recorded because it is printed. Note also that page 260's ranges use a horizontal dash (`⑪–⑬`) where page 263's body text uses a swung dash (`⑫~⑭`), and that page 263 adds a word and a plural: `Data mode and tone type settings`.
12. **The page-260 sub-diagram confirms the crossed leaders independently.** That block draws the identical two-nibble box with the identical two text lines and the identical crossing: left cell to the lower `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`, right cell to the upper `0: OFF, 1: TONE, 2: TSQL`. Recorded as an observation only; the CSV's `⑪` rows were read from PDF page 263 and not from this page.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.
