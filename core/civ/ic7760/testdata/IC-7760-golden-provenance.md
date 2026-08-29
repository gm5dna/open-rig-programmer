# IC-7760 golden vectors — provenance

Quarantine leg G. Built by hand from one PDF's rendered page images.

## Source

- Document title as printed on the cover (PDF page 1): **CI-V REFERENCE GUIDE**, above the rule
  **HF/50 MHz TRANSCEIVER / IC-7760**, with **Icom Inc.** at the foot.
- Revision code as printed: **A7788-8EX-2**, printed at the bottom-left of PDF page 28 (the back
  cover, which carries no folio), immediately above the line
  `© 2024–2025  Icom Inc.      May 2025`.
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7760_civ_2.pdf`
- Page count: **28** PDF pages.
- Folio relation: the printed folio is the PDF page number minus one throughout (PDF 3 carries
  folio 2; PDF 20 carries folio 19; PDF 24 carries folio 23). The cover (PDF 1) and back cover
  (PDF 28) carry no folio.

## Extent

Rendered: all 28 pages at 300 dpi (locating pass). Re-rendered at 400 dpi: PDF pages 3, 6, 18, 19,
20, 21, 24, 28. Re-rendered at 600 dpi (second pass): PDF pages 3, 6, 18, 20, 21, 24.

Read as images, and what each contributed:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | (none) | cover title, model, publisher |
| 3 | 2 | `◇ About the data format`: the frame skeleton `FE FE B2 E0 Cn Sc [Data area] FD` and the two address bytes |
| 6 | 5 | `◇ Command table`: the rows `19 / 00 / (blank Data) / Read the transceiver ID` and `1A* / 00 / See p. 19. / Send/read memory contents` |
| 18 | 17 | `• Operating frequency` (5-byte BCD digit map) and `• Operating mode` (mode/filter tables); also the `• Codes for CW message contents` table, whose Space row prints ASCII code 20 |
| 19 | 18 | checked for a worked example frame; contributed the 14 MHz band range 13.900000 ~ 14.499999 as a cross-check on the frequency I chose |
| 20 | 19 | `• Memory content` data block (the 18-cell band, indices ① ~ ㉗) and all its sub-diagrams; `• Codes for character entries` tables and the `* Usable characters` footnote |
| 21 | 20 | checked for a worked example frame; contributed the keyer-memory ASCII table's Space row, printing code 20 and the description "Word space" |
| 24 | 23 | `• Repeater tone/tone squelch frequency settings` (the 3-byte tone format) and the `② Data mode setting` table involved in STOP 1 |
| 28 | (none) | revision code |

Where the transcribed material begins and ends on PDF page 20 (folio 19):

- Immediately **before** it: the page header rule, `REMOTE CONTROL`, the reversed band
  `Remote control (CI-V) information`, then `◇ Command formats`. The heading of the material
  itself is `• Memory content`, followed by `Command: 1A 00`, then the 18-cell band.
- The memory-content material ends with the block
  `To clear the memory channel contents on 1A 00: / ①, ②: Memory channel (00 01~00 99) / ③: "FF" / ④: None`.
  **No clear/erase frame was built from it**, per the brief.
- The character material begins at `• Codes for character entries` and ends with the footnote
  `* Usable characters: A to Z, a to z, 0 to 9, (space), ! " # $ % & ' ( ) * +, - . / : ; < = > ? @ [ \ ] ^ _ ` { | } ~`.
- Immediately **after** it: PDF page 21 opens `REMOTE CONTROL / Remote control (CI-V) information /
  ◇ Command formats / • Keyer memory character entries`.

## Method

Page images only. Every value recorded was read from a rendered page image.

1. **Locate — 300 dpi.** Fresh directory `r300/` created beneath this leg's output directory:
   `pdftoppm -png -r 300 -f 1 -l 28 <pdf> r300/p`. Read PDF pages 1 and 20 as images to confirm
   the sections the task names.
2. **Read — 400 dpi.** `pdftoppm -png -r 400 -f <p> -l <p> <pdf> r400/p` for PDF pages 3, 6, 18,
   19, 20, 21, 24, 28. All first-pass values were read from these.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`) and was used.
   Representative commands:
   - `magick r400/p-20.png -crop 2700x220+520+900 +repage -resize 300% crops/c20_block2.png` (the numbered band with its index brackets)
   - `magick r400/p-20.png -crop 1400x220+520+900 +repage -resize 400% crops/c20_blockL.png` and `... +1820+900 ... crops/c20_blockR.png` (left and right halves of the band)
   - `magick r400/p-20.png -crop 1550x900+230+1130 +repage -resize 220% crops/c20_leftcol.png` (③ sub-diagram and the ★ list)
   - `magick r400/p-20.png -crop 1600x900+1700+1130 +repage -resize 220% crops/c20_rightcol.png` (⑪ sub-diagram, ⑫~⑰, ⑱~㉗)
   - `magick r400/p-20.png -crop 1350x1420+240+2980 +repage -resize 150% crops/c20_symbols.png` (symbols table)
   - `magick r400/p-18.png -crop 1500x1000+230+1000 +repage -resize 220% crops/c18_freq.png` (operating-frequency digit map)
   - `magick r400/p-06.png -crop 1360x180+230+2030 +repage -resize 300% crops/c06_19_1A.png` (the `19 00` and `1A 00` rows)
   Every numeral, rule and glyph sat clear of its neighbours at these enlargements.
4. **Geometry measured, not eyeballed.** The band's cell boundaries and cell fills were measured
   programmatically from a single horizontal scanline of the render (Python/PIL classifying each
   pixel as black rule / grey fill / white), and the bracket legs were located the same way. This
   was measurement **on the render**; it introduced no text-layer data.
5. **`pdftotext -layout` WAS run**, on this same PDF only, written to `nav.txt`, and used **for
   navigation only** — to find which PDF page carries `About the data format`, the `19 00` command
   row, `Operating frequency`, `Memory content`, `Repeater tone/tone squelch`, and to check whether
   the word `Example` occurs anywhere. It was the source of **no** recorded value: no byte
   position, nibble label, numeral, field index, width, label or enum value was taken from it. Its
   known-bad behaviour was visible in its own output — it drops the ★ glyph (rendering `1= ★1` as
   `1= 1`) and it interleaves the two page columns (a page-18 line reads
   `• Operating mode ... : 3A Space 20`, splicing left-column and right-column material).
6. **`tesseract` was available but was NOT used.** Every value was read by eye from the renders.
7. **Second independent pass — done.** With the first pass complete, every value was re-read from a
   different raster: **600 dpi** renders (`r600/`) with **different crop windows** and different
   enlargement factors from the first pass. Specifically:
   - the band's cell count, cell boundaries and fills were re-measured on the 600 dpi render at a
     different scanline height — one that cuts through the dotted nibble separators, so the second
     pass saw 32 half-cells plus 2 ellipsis cells where the first saw 18 whole cells;
   - the index brackets were re-read from two 600 dpi crops with windows offset from the first
     pass's (`-crop 1700x180+930+1400` and `+2500+1400`, at 250%);
   - the ③ and ⑪ sub-diagrams, the p.24 tone diagram, the p.3 frame diagram, the p.6 command rows,
     the operating-mode/filter table and the `A ~ Z / 41 ~ 5A` row were each re-cropped at 600 dpi
     at different magnifications;
   - the rotated leaders of the operating-frequency diagram were re-read from a **rotated** raster
     (`-rotate -90`), i.e. with the labels running horizontally instead of vertically.
   **Disagreements between the two passes: none.** Every cell agreed. No third render was needed.
8. **Scope of file access.** The only file opened for content was the PDF named above, plus the
   renders and crops I made from it and the files I wrote in this leg's output directory. No
   repository file was opened, listed, searched or browsed; the only directory listings performed
   were of my own `r300/`, `r400/`, `r600/` and `crops/` subdirectories inside this leg's output
   directory, to confirm which renders had been produced.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every index in the
  memory-content band and in every sub-diagram I read (PDF pages 18, 20, 24) is drawn in one single
  style: a plain outlined circle around a plain numeral. There are no filled, reversed, bracketed or
  bold indices anywhere in the material transcribed, and none printed twice.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** The operating-frequency
  and repeater-tone diagrams (PDF pages 18 and 24) carry their digit labels rotated 90°, and the
  navigational `pdftotext -layout` output splices left-column and right-column text on those pages
  (a page-18 line extracts as `• Operating mode ... : 3A Space 20`). All positions were read from
  the picture, and the frequency labels were additionally re-read from a rotated raster.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In the `⑪: Data mode and tone type
  settings` sub-diagram on PDF page 20, the two option lines are printed stacked, with
  `0: OFF, 1: TONE, 2: TSQL` **above** `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`. Following each
  leader by eye reverses that order: the **left** (first-printed) nibble's arrow descends past the
  upper line and joins the **lower** line (data mode), while the **right** nibble's short elbow
  joins the **upper** line (tone type). Confirmed on both passes (400 dpi at 220%, 600 dpi at 200%).
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED.** The one
  block that repeats another is `⑫ ~ ⑭` (repeater tone) against `⑮ ~ ⑰` (tone squelch), both three
  bytes drawn to the same 3-cell pattern. Printed indices and measured positions for both, recorded
  separately and not reconciled: `⑫ ~ ⑭` printed 12–14, measured at drawn cells 10, 11, 12 of the
  18-cell band (x ≈ 1639–1966 on the 400 dpi render); `⑮ ~ ⑰` printed 15–17, measured at drawn cells
  13, 14, 15 (x ≈ 1970–2297). Separately, and not a case of (d): two groups are drawn with an
  elision, so their printed index span exceeds their drawn cell count — `④ ~ ⑧` is printed over 3
  drawn cells (a cell, a dotted `...` cell, a cell) and `⑱ ~ ㉗` likewise. Both printed span and
  measured drawn count are recorded here; neither is reinterpreted in the light of the other.

## STOP findings

1. **PDF page 20 (folio 19) contradicts PDF page 24 (folio 23) on the data-mode code names.**
   - PDF page 20, `⑪: Data mode and tone type settings` sub-diagram, the lower of the two option
     lines beneath the two-half box, prints: `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3`.
   - PDF page 24, `• Main or Sub band's operating mode and filter settings` (Command: 26), the
     column headed `② Data mode setting`, prints, row by row:
     `00: Data mode OFF` / `01: Data mode 1 (D1)` / `02: Data mode 1 (D2)` / `03: Data mode 1 (D3)`.
   - Why it stops: the same four data-mode codes are named differently on the two pages. Code 02 is
     `DATA 2` on one page and `Data mode 1 (D2)` on the other; code 03 is `DATA 3` on one and
     `Data mode 1 (D3)` on the other. The page-24 column is additionally self-inconsistent, printing
     the words `Data mode 1` against three different parenthesised labels D1, D2 and D3. Both are
     transcribed above exactly as printed; neither has been repaired.
   - What I built from: the PDF page 20 statement, which I judge the clearer — it is the one printed
     inside the very sub-diagram that defines the field I am encoding, and its enumeration is
     internally consistent.
   - Effect on the CSV: byte 17 of `set-record-name-with-space` carries `STOP 1` in its `notes`. The
     **value is not in dispute**: I chose data mode OFF, and both statements print code 0/00 as OFF.
     The row's status stays `manual_derived` rather than `manual_documented` because the value was
     my choice from a printed encoding, which the brief classes as derived; downgrading it to
     `manual_documented` to satisfy the STOP wording would misstate where the byte came from.

No other STOP arose. Reasons for confidence on the rest:

- The band's arithmetic closes. Measured: 19 rule positions bounding **18** drawn cells across
  x = 644 … 2629 on the 400 dpi render, cells 107–108 px wide with no gap and no overlap
  (independently re-measured at 600 dpi as 32 nibble half-cells plus 2 ellipsis cells = the same 18).
  Bracket legs land exactly on measured cell boundaries. Group sizes 2 + 1 + (3 drawn = 5 indices)
  + 2 + 1 + 3 + 3 + (3 drawn = 10 indices) = 18 drawn cells and 27 indices; the index sequence runs
  ① … ㉗ with no repeat, no gap and no out-of-order or twice-printed index; and ㉗ = 27 =
  2 + 1 + 5 + 2 + 1 + 3 + 3 + 10.
- The letter range closes: `5A − 41 = 0x19 = 25`, i.e. 26 letters, matching `A ~ Z`.
- Every value was legible at 400 dpi enlarged, and no cell disagreed between the two passes.

## Observed disagreements

Recorded as printed, not resolved, and none of these stopped me.

1. **`(space)` is declared usable but has no code in the tables that govern it.** The footnote on
   PDF page 20 reads `* Usable characters: A to Z, a to z, 0 to 9, (space), ...` and its asterisk
   marks the row `1A / 00 / Memory name*`. Yet neither of the two `• Codes for character entries`
   tables on that same page prints a Space row: the `Letters and Numbers` table prints only
   `A ~ Z / 41 ~ 5A`, `a ~ z / 61 ~ 7A`, `0 ~ 9 / 30 ~ 39`, and the `Symbols` table's 32 rows run
   `!` 21 … `@` 40 with no Space among them. The code `20` for Space **is** printed in this same
   document, but in two other sections: PDF page 18 (`• Codes for CW message contents`, row
   `Space | 20`) and PDF page 21 (`• Keyer memory character entries`, row `Space | 20 | Word space`).
   Both tables are headed `ASCII code`, as is the page-20 one. Byte 29 is recorded as
   `manual_derived` on that combined footing, with all three PDF pages named in its `pdf_page` cell.
2. **Cross-reference title mismatch.** PDF page 20 sends the reader to
   `See "• Repeater tone/tone squelch settings." (p. 23)`, but the section printed on folio 23
   (PDF page 24) is headed `• Repeater tone/tone squelch frequency settings`, and the table of
   contents on PDF page 2 also prints the longer form. The word `frequency` is dropped in the
   cross-reference only.
3. **Field name against field values.** The ① ② heading on PDF page 20 reads
   `①, ②: Memory group number`, but the values printed underneath it are memory *channels* and scan
   edges: `00 01 ~ 00 99: Memory channel 01 ~ 99`, `01 00: Programmed scan edge P1`,
   `01 01: Programmed scan edge P2`. The clear-form block lower on the same page calls the same
   field `①, ②: Memory channel (00 01~00 99)`.
4. **Gaps in the operating-mode enumeration.** The `①Operating mode` table on PDF page 18 prints
   00, 01, 02, 03, 04, 05, 07, 08, 12, 13. Codes 06 and 09–11 are not printed. This is an
   enumeration in a table, not an index sequence in a diagram, so it is recorded here rather than as
   a STOP. My chosen value 01 (USB) is printed.
5. **No list of permitted tone frequencies.** PDF page 24 prints digit ranges for the repeater-tone
   and tone-squelch fields (`100Hz digit: 0 ~ 2`, `10 Hz digit: 0 ~ 9`, `1 Hz digit: 0 ~ 9`,
   `0.1 Hz digit: 0 ~ 9`) but no list of which tone frequencies the radio accepts. My chosen 088.5 Hz
   satisfies every printed digit range; whether the radio accepts that particular frequency is not
   printed anywhere in this document.
6. **The `1A 00` row of the character-entry table prints no length.** In the right-hand
   `Cmd. / Sub cmd. / Set item/selectable characters` table on PDF page 20, every `1A 05` row gives a
   count (`up to 15 characters`, `up to 16 characters`, `up to 10 characters`, `up to 3 characters`,
   `up to 64 characters`), but the `1A / 00 / Memory name*` row gives none. The count for the memory
   name is printed instead beside the block diagram, as `⑱ ~ ㉗: Memory name settings / Up to 10
   characters.` Both statements are consistent with the ten cells the band draws.

## The vectors

Three vectors. The memory-name field has exactly **one** documented width — ten, printed twice
(ten drawn cells indexed ⑱ ~ ㉗, and the sentence `Up to 10 characters.`) — so the record has one
derived total length and there is one `set-record-name-with-space` vector, not a numbered family.

No `manual-example-<n>` vector was written. The document prints **no worked example frame of any
kind**. The only two things it calls an example are data-area fragments, not frames: PDF page 19
prints `Example: When reading the frequency displayed in the center of the display in the 21 MHz
band, use code "07 02."` and PDF page 21 prints `Example: to send BT, enter "5E 42 54"`. Neither
carries a preamble, an address or an end-of-message byte. The `◇ About the data format` diagrams on
PDF page 3 are skeletons with `Cn`, `Sc` and `Data area` placeholders, not worked frames.

No clear/erase frame and no transceive frame of any kind was built, whether or not the document
prints one.

### `read-record` — 9 bytes

Reads one memory record. `FE FE B2 E0 1A 00 00 01 FD`.

| byte | hex | field | source |
|---|---|---|---|
| 1–2 | FE FE | — | PDF 3, preamble cells, `structural` |
| 3 | B2 | — | PDF 3, `Transceiver's default address`, `structural` |
| 4 | E0 | — | PDF 3, `Controller's (PC's) default address`, `structural` |
| 5–6 | 1A 00 | — | PDF 6, command table row `1A* / 00 / Send/read memory contents`, `manual_documented` |
| 7–8 | 00 01 | ① ② | **`inherited_assumed`** — see Assumption register A1 |
| 9 | FD | — | PDF 3, `End of message code (fixed)`, `structural` |

The data area is the two channel bytes and nothing more. That is a choice, not a reading: the
command table prints `Send/read memory contents` for `1A 00` but the document prints no read format
for it anywhere, and the only truncated form it does print is the clear form (`③: "FF"`, `④: None`),
which I was told not to build. Bytes 7–8 are therefore marked assumed, with empty `pdf_page` and
`visual_anchor`.

### `set-record-name-with-space` — 34 bytes

Writes one complete memory record whose ten-character name contains a space in the middle:
`ALPHA BETA`.

`FE FE B2 E0 1A 00 00 01 00 00 00 10 14 00 01 01 00 00 08 85 00 08 85 41 4C 50 48 41 20 42 45 54 41 FD`

Length arithmetic: 4 framing/addressing bytes + 2 command bytes + 27 data bytes + 1 end-of-message
byte = **34**. The 27 comes from the band's own groups: 2 (① ②) + 1 (③) + 5 (④ ~ ⑧) + 2 (⑨ ⑩) +
1 (⑪) + 3 (⑫ ~ ⑭) + 3 (⑮ ~ ⑰) + 10 (⑱ ~ ㉗) = 27 = ㉗.

| byte(s) | hex | field | what it is |
|---|---|---|---|
| 1–2 | FE FE | — | preamble (PDF 3) |
| 3 | B2 | — | transceiver's default address (PDF 3) |
| 4 | E0 | — | controller's default address (PDF 3) |
| 5–6 | 1A 00 | — | command and sub command (PDF 6) |
| 7–8 | 00 01 | ① ② | memory channel 01, derived from `00 01 ~ 00 99: Memory channel 01 ~ 99` (PDF 20) |
| 9 nibble 1 | 0 | ③ | the literal `0` printed in the box with the arrow labelled `Fixed` (PDF 20) |
| 9 nibble 2 | 0 | ③ | `0=OFF` chosen from the printed list `0=OFF / 1=★1 / 2=★2 / 3=★3` (PDF 20) |
| 10–13 | 00 00 10 14 | ④ ~ ⑦ | 14.100000 MHz in the printed digit map (PDF 18) |
| 14 | 00 | ⑧ | `1 GHz digit: 0 (Fixed)` and `100 MHz digit: 0 (Fixed)` (PDF 18) |
| 15–16 | 01 01 | ⑨ ⑩ | `01:USB` and `01:FIL1` (PDF 18) |
| 17 | 00 | ⑪ | data mode OFF (nibble 1) and tone type OFF (nibble 2) (PDF 20) — carries `STOP 1` |
| 18 | 00 | ⑫ | `Fixed digit: 0*` twice (PDF 24) |
| 19–20 | 08 85 | ⑬ ⑭ | repeater tone 088.5 Hz in the printed digit map (PDF 24) |
| 21 | 00 | ⑮ | `Fixed digit: 0*` twice (PDF 24) |
| 22–23 | 08 85 | ⑯ ⑰ | tone squelch 088.5 Hz (PDF 24) |
| 24–28 | 41 4C 50 48 41 | ⑱ ~ ㉒ | `A L P H A` from `A ~ Z / 41 ~ 5A` (PDF 20) |
| 29 | 20 | ㉓ | the mid-name space (PDF 20 footnote; code from PDF 18 and PDF 21) |
| 30–33 | 42 45 54 41 | ㉔ ~ ㉗ | `B E T A` from `A ~ Z / 41 ~ 5A` (PDF 20) |
| 34 | FD | — | end of message (PDF 3) |

Working shown for each derived run:

- **Frequency, bytes 10–14.** I chose 14.100000 MHz. Its ten digits are 1 GHz = 0, 100 MHz = 0,
  10 MHz = 1, 1 MHz = 4, 100 kHz = 1, 10 kHz = 0, 1 kHz = 0, 100 Hz = 0, 10 Hz = 0, 1 Hz = 0.
  Placing each digit in the nibble its printed leader points at — cell ① = [10 Hz][1 Hz],
  cell ② = [1 kHz][100 Hz], cell ③ = [100 kHz][10 kHz], cell ④ = [10 MHz][1 MHz],
  cell ⑤ = [1 GHz][100 MHz] — gives `00 00 10 14 00`. The 10 MHz digit is printed as `0 ~ 6`, and
  1 is within it. Cross-check from the same document: the band-stacking table on PDF page 19 prints
  the 14 band as `13.900000 ~ 14.499999`, which contains 14.100000.
- **Mode, bytes 15–16.** `01:USB` from the `①Operating mode` column and `01:FIL1` from the
  `②Filter setting` column, both on PDF page 18, give `01 01`. The note on that page says the
  filter byte may be skipped for commands 01 and 06; that permission is not offered for the memory
  record, whose band draws two cells for ⑨ ⑩, so both bytes are written.
- **Tones, bytes 18–23.** I chose 088.5 Hz for both. Per PDF page 24: cell ① = [Fixed 0][Fixed 0]
  → `00`; cell ② = [100 Hz digit][10 Hz digit] = 0, 8 → `08`; cell ③ = [1 Hz digit][0.1 Hz digit]
  = 8, 5 → `85`. Every digit is inside its printed range (100 Hz digit `0 ~ 2`, the rest `0 ~ 9`).
  The same three bytes are written for ⑮ ~ ⑰ because PDF page 20 sends both groups to the same
  diagram.
- **Name, bytes 24–33.** `ALPHA BETA` is exactly ten characters — the field's full drawn width — so
  no padding convention is needed and none is assumed. Counting within the printed range `A ~ Z =
  41 ~ 5A`: A = 41, B = 42, E = 45, H = 48, L = 4C, P = 50, T = 54. The range is self-consistent
  (`5A − 41 = 25`, i.e. 26 letters). The space at position ㉓ is `20`, on the footing set out in
  Observed disagreement 1.

### `read-transceiver-id` — 7 bytes

The `19 00` transceiver-identification read. `FE FE B2 E0 19 00 FD`.

| byte(s) | hex | field | source |
|---|---|---|---|
| 1–2 | FE FE | — | PDF 3, preamble, `structural` |
| 3 | B2 | — | PDF 3, transceiver's default address, `structural` |
| 4 | E0 | — | PDF 3, controller's default address, `structural` |
| 5–6 | 19 00 | — | PDF 6, command table row `19 / 00 / (blank) / Read the transceiver ID`, `manual_documented` |
| 7 | FD | — | PDF 3, end of message, `structural` |

The command table's Data cell for this row is printed empty — unlike the `1A*` rows beneath it,
which read `See p. 19.`, `See p. 18.`, `See p. 20.` — so no data area is carried and the frame ends
straight after the sub command. Nothing here is assumed.

## Assumption register

One `inherited_assumed` run.

**A1 — `read-record`, bytes 7–8, `00 01`.**

- *What was assumed.* That a read of one memory record is the command `1A 00` followed by the
  two-byte channel number and nothing else, and that the frame ends there.
- *Why that and not something else.* The document is silent on the read form of `1A 00`. The
  command table on PDF page 6 prints one row for both directions — `Send/read memory contents` —
  and points its Data cell at `See p. 19.`; the block on PDF page 20 draws the full 27-byte record
  and says nothing about what a read request contains. The only shortened `1A 00` payload the
  document does print is the clear form (`①, ②` then `③: "FF"` then `④: None`), which the brief
  forbids me to build and which in any case is a write. Two bytes and no more is the shortest form
  that identifies which record is wanted, and it is the prefix the printed clear form itself opens
  with, so it is the assumption that adds least beyond the page. The channel value `00 01` is the
  lower bound the page prints (`00 01 ~ 00 99: Memory channel 01 ~ 99`); the *value* is documented,
  the *extent of the frame* is not, and it is the extent that makes this run assumed.
- *The one capture that would settle it.* A single **Stage R** capture on an IC-7760: send
  `FE FE B2 E0 1A 00 00 01 FD` and record the bytes the radio puts on the wire in reply. That one
  capture shows whether this exact frame is accepted as a read of channel 01 — a record comes back —
  or refused (`FE FE E0 B2 FA FD`). It settles nothing else: not the read form of any other channel,
  not the write direction, not any other command, and not the behaviour of any other model.

Two things are **not** in this register because they are not assumed bytes, but they should be read
alongside it:

- The address byte `B2` is the *default* the document prints on PDF page 3. The same page's
  `◇ Preparing` paragraph says the address is set in the Set mode, so a radio whose address has been
  changed will not answer to `B2`. The byte is documented; the radio's current address is not
  something a document can state.
- The tone frequency 088.5 Hz conforms to every printed digit range, but the document prints no list
  of permitted tone frequencies, so whether the radio accepts that value is not stated anywhere in
  it. The document is silent.

## Hardware status

UNVERIFIED. No IC-7760 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.

## Attestation

Every value recorded here was read from this single PDF's rendered page images. `pdftotext -layout`
was run on this same PDF for navigation only and was the source of no recorded value. Nothing else
was consulted: no other file, manual, transcription, source file, generated artefact or web resource
was opened, and no directory was listed.
