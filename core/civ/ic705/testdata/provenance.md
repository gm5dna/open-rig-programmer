# IC-705 CI-V golden vectors — provenance

## Source

Document title as printed on the cover (PDF page 1): the black band prints
**CI-V REFERENCE GUIDE**; below it the page prints **HF/VHF/UHF ALL MODE
TRANSCEIVER** and, in display type, **IC-705**; the foot of the cover prints
**Icom Inc.**

Revision code as printed: **A7560-8EX-6**, printed at the foot of the left-hand
column of PDF page 31 (the back cover, which carries no folio), immediately
above the line `© 2020–2023  Icom Inc.    Jan. 2023`. The cover itself prints no
revision code.

File path:
`/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic705_civ_rev6.pdf`

Page count: 31 PDF pages.

## Extent

Rendered at 300 dpi: PDF pages 1, 2, 3, 4, 5, 6, 8–22, 23, 24, 25, 31.
Re-rendered at 400 dpi: PDF pages 3, 6, 9, 17, 18, 19, 20, 21, 23, 24.
Re-rendered at 600 dpi for the second pass: PDF pages 3, 6, 18, 19, 20, 23, 24, 31.

Pages actually read, with the folio printed on each:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none (cover) | document title as printed |
| 3 | 2 | the frame structure: `FE FE` `A4` `E0` `Cn` `Sc` `Data area` `FD` for a controller-to-IC-705 message |
| 6 | 5 | command table rows `19*1 / 00 / (blank) / Read the transceiver ID` and `1A* / 00 / See pp. 18 and 19. / Send/read memory contents` |
| 9 | 8 | set-mode/menu material: `1A 05` sub-commands 0116–0135 under `SET > Connectors` and 0136–0144 under `SET > Display`. **Contributed no byte to any vector** — it carries no memory-record field, no character code and no framing |
| 17 | 16 | the command-table footnote key: `*(Asterisk) Send/read data`, `*1 Read only data`, `*2 Send only data` |
| 18 | 17 | Operating frequency (5 bytes), Operating mode + Filter setting (2 bytes), Duplex Offset frequency setting (3 bytes) |
| 19 | 18 | the memory-record data block for `1A 00` — the whole of it |
| 20 | 19 | Codes for character entries: the Letters-and-Numbers table, the Symbols table, and the `1A 00 / Memory name / All characters are usable.` set-item row |
| 21 | 20 | inspected for a printed worked example frame; the only "Example:" is `to send BT, enter ^4254` inside the keyer character table (command `1A 02`) — a character string, not a frame |
| 23 | 22 | Repeater tone/tone squelch frequency settings (`1B 00`, `1B 01`) and DTCS code and polarity setting (`1B 02`) |
| 24 | 23 | DV Digital code squelch setting (`1B 07`), DV TX call signs setting (`1F 01`) and the `Character's code of the call sign` table |
| 25 | 24 | inspected for a printed worked example frame; the only "Example:" is a received-call-sign illustration, not a frame |
| 31 | none (back cover) | revision code |

Where the transcribed material begins and ends. The memory-record material on
PDF page 19 begins at the bold bullet heading `• Memory content` with
`Command: 1A 00` on the line under it. Printed immediately above that heading
are the line `◇ Command formats` and, above that, the reversed grey band
`Remote control (CI-V) information`. The material ends with the grey NOTE box
whose last bullet closes `...We recommend that you set the same data as ⑥ ~ 52.`;
printed immediately after it is the folio `18` and nothing else. In the left
column the last thing printed before the folio is the ⑮ legend line
`2=Digital code squelch function ON (CSQL)`.

The frame material on PDF page 3 begins at the bold heading `◇ About the data
format` (printed immediately below `◇ Preparing`) and ends with the bold caption
`NG message to controller` under the lower right-hand frame; printed
immediately after is the folio `2`.

## Method

1. **Locate.** `pdftoppm -png -r 300 -f <first> -l <last> <pdf> <out>/p` into a
   fresh directory `.../evidence/ic705-G/r300/`, created for this leg and empty
   before the first render. Whole-page renders were read as images to find the
   headings the task names.
2. **Read.** Every page from which a value was taken was re-rendered at 400 dpi
   (`pdftoppm -png -r 400`) into `.../r400/`, and the first-pass values were read
   from those.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`,
   `/opt/homebrew/bin/convert`) and was used throughout. Representative commands:
   - `magick r400/p-19.png -crop 1000x300+200+1030 +repage -resize 300% crop/r1a.png`
   - `magick r400/p-19.png -crop 800x420+220+2280 +repage -resize 300% crop/f5.png`
   - `magick r400/p-18.png -crop 1350x760+230+1060 +repage -resize 220% crop/opfreq.png`
   - `magick r400/p-23.png -crop 1050x680+1710+2740 +repage -resize 250% crop/tone.png`
   - `magick r400/p-20.png -crop 1400x760+230+2040 +repage -resize 200% crop/chartab-sym1.png`
   Each numbered band, legend diagram and table was cropped into its own image and
   enlarged until every numeral, rule and glyph stood clear of its neighbours.
4. **`pdftotext -layout` was run**, once, on this same PDF, writing
   `.../evidence/ic705-G/nav.txt`. It was used **for navigation only** — to find
   which PDF page carries the `Command table`, the `About the data format`
   heading, the command-table footnote key and the `Example:` strings. No byte
   position, nibble label, numeral, field index, width, label or enum value in the
   CSV or in this file came from it. Its output for PDF page 3 is visibly
   scrambled (see hazard (b) below), which is why it was not trusted.
5. **`tesseract` was available** (`/opt/homebrew/bin/tesseract`) but **was not
   used**. Every crop was legible by eye at the enlargements above, so no OCR aid
   was needed and no OCR value was recorded.
6. **Second independent pass.** After the first pass was complete, every recorded
   value was re-read from a different raster: the pages were re-rendered at
   **600 dpi** (not 400) into `.../r600/`, and cropped with **different crop
   windows and different enlargement factors** into `.../crop2/` — the memory
   block was cut into four overlapping quarters instead of the first pass's
   three-plus-three, the symbols table was cut into two full-height columns
   instead of two half-height bands, and each diagram was enlarged at a different
   percentage. Representative second-pass commands:
   - `magick r600/p-19.png -crop 2300x330+300+1580 +repage -resize 200% crop2/p19-row1-left.png`
   - `magick r600/p-20.png -crop 1000x2200+350+3080 +repage -resize 90% crop2/p20-symcol1.png`
   - `magick r600/p-23.png -crop 1550x1000+2570+4130 +repage -resize 170% crop2/p23-tone.png`

   **Both passes were done.** The second pass re-read: the frame diagram on PDF 3;
   the `19*1 / 00` and `1A* / 00` command-table rows on PDF 6; the whole memory
   block on PDF 19 including the cell count of every bracketed group, the numeral
   style of every index, the ⑤/⑭/⑮ nibble roles and the legend and NOTE text; the
   operating-frequency, operating-mode and duplex-offset diagrams on PDF 18; both
   character tables on PDF 20; the tone and DTCS diagrams on PDF 23; the DV code
   squelch diagram and the call-sign character table on PDF 24; and the revision
   code on PDF 31.

   **Cells where the two passes disagreed: none.** Every value read in the second
   pass matched the first, including the counts 24 cells in the upper band, 1 + 3
   + 8 + 8 + 8 + (one elided cell) + 16 drawn groups in the lower band, the two
   numeral styles, and the absence of any space row in the Symbols table.

**Status convention used in the CSV.** `manual_documented` was reserved for
bytes and half-bytes whose value the document fixes with no freedom left to me —
the digits printed literally as `0` with a `(fixed)` leader, and the command and
sub-command values printed in the command table. Wherever I selected a value
from a printed range, table or enumeration — a frequency digit, a letter, a mode
code, a polarity — the run is `manual_derived` and the arithmetic is shown in
the walk below. `structural` covers the preamble, the two addresses and the
end-of-message byte, all four of which the document states directly on PDF page
3. This is the conservative reading of the task's parenthetical and it never
records a value I chose as though the page had chosen it for me.

**Nibble-run convention.** Where a run carries only one half of a byte, the
`bytes_hex` cell prints the whole containing byte (the format has no half-byte
form) and the `first_nibble`/`last_nibble` columns say which half the run is
about. Nibble 1 is the half printed on the left in these left-to-right diagrams.
The verification script confirms that for every such byte, both nibbles are
claimed exactly once across the rows.

## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** The memory
  content diagram on PDF page 19 draws its index numerals in two styles: outline
  circled numerals (a black ring round a black numeral on white) for every group
  from ① through 68, and **filled reversed** circles (a solid black disc with a
  white numeral) for the single group labelled ❻ ~ ❺❷ in the lower band. Both
  styles are recorded exactly as drawn — the CSV's `field_index` cell for that run
  says `printed 6 ~ 52 in filled reversed circled numerals`. No meaning is
  inferred for either style; the NOTE box beneath, itself printing ❻ ~ ❺❷ in the
  filled style, is quoted rather than interpreted.
- **(b) Vector groups with rotated labels extracting out of order — ENCOUNTERED.**
  The navigational `pdftotext -layout` output for PDF page 3 emits the
  data-format diagram's rotated leader labels in an order unrelated to the page:
  the fragments `OK code`, `(fixed)`, `(see the command table)`, `Sub command
  number`, `BCD code data for`, `Command number`, `Preamble  Transceiver's
  Controller's ... End of message`, `frequency or memory`, `number entry`, `NG
  code` arrive interleaved and out of sequence, and the two frame rows come out
  before and after the labels rather than around them. Every value was read from
  the 400 dpi and 600 dpi renders instead.
- **(c) Leader-line label order reversed — ENCOUNTERED.** On PDF page 3 the
  leaders for `Transceiver's default address` and `Controller's default address`
  **cross** in the middle of the figure: the label sitting to the left points
  **up** to cell ② (`A4`) of the upper frame and **down** to cell ③ (`A4`) of the
  lower frame, and its neighbour does the mirror image. Each leader was followed
  by eye from label to the cell it lands on; the upper frame, whose caption reads
  `Controller to IC-705`, is the one used for all three vectors, giving `A4` at
  byte 3 and `E0` at byte 4.
- **(d) Printed index differs from measured position — ENCOUNTERED.** The
  filled-numeral group in the lower band of the memory content diagram prints the
  indices **6 ~ 52** but sits, measured along the drawn band, at data-block byte
  positions **53 to 99** — after the group bracketed ㊺ ~ 52 and before the group
  bracketed 53 ~ 68. Both are recorded, unreconciled, in that run's `field_index`
  cell. It is also recorded as STOP 1 below.

## STOP findings

1. **PDF page 19, memory content data block, lower band — an index printed twice
   with different styling, and the outline sequence resuming out of order.**
   The lower band prints, left to right: a group bracketed ㊺ ~ 52 in outline
   circled numerals; then a wide shaded dotted cell whose bracket is labelled
   **❻ ~ ❺❷** in filled reversed circles; then a group bracketed **53 ~ 68** in
   outline circled numerals. The printed indices 6 to 52 therefore appear twice in
   one diagram in two different numeral styles, and after the filled 6 ~ 52 group
   the outline sequence resumes at 53 rather than continuing from 52. Measured
   along the band, the filled group occupies data-block positions 53 to 99 and the
   outline 53 ~ 68 group occupies positions 100 to 115.

   Why it stops: the task's STOP rules name "a repeat" and "an index printed twice
   with different styling" explicitly, and this diagram does both at once. It is
   recorded, not resolved. Nothing was renumbered, reconciled or carried over: the
   CSV row at frame bytes 59–105 states both the printed index range and the
   measured positions and carries `STOP 1`, and the first name row at frame bytes
   106–107 carries `STOP 1` as well, marking the point where the outline sequence
   resumes.

   For the record, the grey NOTE box on the same page prints `The same data as ⑥ ~
   52 are stored in ❻ ~ ❺❷.` and `Even if the Split function is OFF, enter the
   data into ❻ ~ ❺❷ to match your transceiver.` — which is why the write vector
   carries the 47-byte copy at all. The width 47 is derived from the printed index
   range (52 − 6 + 1), not measured, because the region is drawn as a single
   elided cell; that derivation is stated in the run's `notes`.

No other STOP arose. Reasons for confidence on the rest: every bracketed group's
drawn cell count equals its printed index span (2, 2, 1, 5-with-ellipsis, 1, 1,
1, 1, 1, 3, 3, 3 in the upper band; 1, 3, 8-with-ellipsis, 8-with-ellipsis,
8-with-ellipsis, one elided cell, 16-with-ellipsis in the lower band); the three
call-sign groups each span 8 indices and are each labelled `(8 characters,
fixed)`; the name group spans 16 indices and is labelled `(16 characters,
fixed)`; the upper band sums to 24 and the lower to 91, giving 115, with no
overlap and no gap; and each sub-format the legend points at prints exactly as
many cells as the memory block reserves for it (5 for the frequency, 2 for the
mode, 3 for each tone field, 3 for DTCS, 1 for the DV code squelch, 3 for the
duplex offset). Every numeral was read cleanly at 400 dpi enlarged and again at
600 dpi with different crops.

## Observed disagreements

Recorded as printed, not resolved.

1. **PDF page 19, right column: two consecutive legend entries carry the same
   name.** The page prints `⑯~⑱: Repeater tone frequency setting` and, on the very
   next line, `⑲~㉑: Repeater tone frequency setting` — the identical wording for
   two different three-byte fields. Both then share one reference,
   `ⓘSee "Repeater tone/tone squelch frequency setting." (p. 22)`, and the section
   at that reference is headed `Repeater tone/tone squelch frequency settings` for
   **two** commands, `1B 00, 1B 01`. Nothing printed says which of the two fields
   is the repeater tone and which the tone squelch. Both passes read both lines the
   same way. The write vector therefore encodes the same frequency, 88.5 Hz, into
   both fields, and neither run claims a role the page does not print.

2. **PDF page 23, the tone diagram's conditional first byte, against the memory
   block's fixed three cells.** The `Repeater tone/tone squelch frequency
   settings` diagram (`Command: 1B 00, 1B 01`) labels its cell ① with an asterisk
   and prints the footnote `*Not necessary when setting a frequency.` — so for
   those two commands the field is either three bytes or two. The memory content
   diagram, whose heading is `• Memory content / Command: 1A 00`, nevertheless
   prints ⑯ and ⑲ as their own separately numbered cells inside a numbered
   sequence that runs unbroken to 68. Because the footnote's own diagram is headed
   by commands `1B 00, 1B 01` and not by `1A 00`, and because within the section
   whose printed heading matches the task the width is printed as three numbered
   cells, **no field of the `1A 00` record has a conditional printed width**, the
   record has exactly one derived total, and there is exactly one
   `set-record-name-with-space` vector rather than a numbered family. The optional
   byte is nonetheless sent as `00`, the value the diagram prints in that cell.

3. **PDF page 20: `All characters are usable.` against a symbol table that omits
   the space.** The set-item table at the foot of the left column prints
   `1A | 00 | Memory name / All characters are usable.` The two tables above it —
   `Character codes— Letters and Numbers` and `Character codes— Symbols`, which
   together are the whole of `Codes for character entries`, the section the memory
   name field points at — print A~Z, a~z, 0~9 and thirty-four symbols, and print
   **no row for a space**. Both passes counted the symbol table the same way
   (17 rows of two entries: `!` `#` `$` `%` `&` `\` `?` `"` `'` `` ` `` `^` `+`
   `−` `*` `/` `.` `,` `:` `;` `=` `<` `>` `(` `)` `[` `]` `{` `}` `|` `_` `~` `@`
   with their codes). This is why the two space bytes inside the memory name are
   `inherited_assumed`; see A2.

4. **The same document prints `Space = 20` twice, but never for `1A 00`.**
   PDF page 18 prints `Space | 20` in the table `Codes for CW message contents`,
   whose heading reads `Command: 17 (Up to 30 characters)`; PDF page 21 prints
   `Space | 20 | Word space` in `Keyer memory character entries`, headed
   `Command: 1A 02`; PDF page 24 prints `(Space) | 20` in `Character's code of the
   call sign`, which the memory record's call-sign fields do point at. The memory
   **name** field points at neither of the first two. The call-sign fields'
   spaces are therefore recorded against PDF page 24, and the memory name's
   spaces are recorded as assumed.

## The vectors

### `read-record` — 11 bytes

Reads one memory record. Frame:

```
FE FE A4 E0 1A 00 00 00 00 12 FD
```

| frame bytes | value | what it is | CSV rows |
|---|---|---|---|
| 1–2 | `FE FE` | preamble, cell ① of the `Controller to IC-705` diagram | row 1 |
| 3 | `A4` | transceiver's default address, cell ② | row 2 |
| 4 | `E0` | controller's default address, cell ③ | row 3 |
| 5 | `1A` | command number, `Cn`; the command table prints `1A*` | row 4 |
| 6 | `00` | sub-command number, `Sc`; the same row prints `00` | row 5 |
| 7–10 | `00 00 00 12` | data area: memory group number (fields ①②) then memory channel number (fields ③④). **Assumed** — see A1 | row 6 |
| 11 | `FD` | end of message, cell ⑦ | row 7 |

Working for bytes 7–10. PDF page 19 prints, for ①②, `0000 ~ 0099: Memory
channel group` and `0100: Call channel group`; group **0000** was chosen, whose
four digits `0 0 0 0` pack two per byte into `00 00`. For ③④ it prints
`When Memory channel group is selected, 0000 ~ 0099: 00 ~ 99`; memory channel
**12** was chosen, whose four digits `0 0 1 2` pack into `00 12`. What is not
printed anywhere is that a read request stops after ④; that is A1.

### `set-record-name-with-space` — 122 bytes

Writes one complete memory record whose 16-character name is `MY REPEATER CH01`,
with a space at name position 3 and another at name position 12 — both in the
middle of the name, neither trailing. Frame:

```
FE FE A4 E0 1A 00
00 00 00 12 00 00 00 50 45 01 05 01 00 11 00 00 08 85 00 08 85 00 00 23 00 00 60 00
43 51 43 51 43 51 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20
00 00 50 45 01 05 01 00 11 00 00 08 85 00 08 85 00 00 23 00 00 60 00
43 51 43 51 43 51 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20
4D 59 20 52 45 50 45 41 54 45 52 20 43 48 30 31
FD
```

(written on one line in the `.golden` file).

**How the length 122 is derived.** The upper band of the data block draws, left
to right: ① ② (2 cells), ③ ④ (2), ⑤ (1), ⑥~⑩ (5, drawn as first cell, elided
cell, last cell), ⑪ (1), ⑫ (1), ⑬ (1), ⑭ (1), ⑮ (1), ⑯~⑱ (3), ⑲~㉑ (3), ㉒~㉔ (3)
= **24 bytes**. The lower band draws ㉕ (1), ㉖~㉘ (3), ㉙~㊱ (8), ㊲~㊹ (8), ㊺~52
(8), the filled ❻ ~ ❺❷ region (52 − 6 + 1 = 47, derived from the printed index
range because the region is drawn as one elided cell), 53~68 (16) = **91 bytes**.
24 + 91 = **115** data bytes. The frame adds `FE FE A4 E0` (4) + `Cn Sc` (2)
before it and `FD` (1) after it: 4 + 2 + 115 + 1 = **122**. Because no field of
the record has a conditional printed width (see Observed disagreement 2), 122 is
the only derived total and the vector carries no `-<n>` suffix.

Byte-by-byte walk, keyed to the CSV rows for `set-record-name-with-space`:

| frame bytes | value | field | source and working |
|---|---|---|---|
| 1–2 | `FE FE` | — | preamble, PDF 3 |
| 3 | `A4` | — | transceiver address, PDF 3 |
| 4 | `E0` | — | controller address, PDF 3 |
| 5 | `1A` | — | command, PDF 6 |
| 6 | `00` | — | sub-command, PDF 6 |
| 7–8 | `00 00` | ①② | memory group **0000** chosen from `0000 ~ 0099: Memory channel group` |
| 9–10 | `00 12` | ③④ | memory channel **12** chosen from `0000 ~ 0099: 00 ~ 99` |
| 11 nibble 1 | `0` | ⑤ | Split OFF, from `0=Split OFF,1=Split ON` |
| 11 nibble 2 | `0` | ⑤ | Select memory OFF, from `0=OFF*` |
| 12–15 | `00 00 50 45` | ⑥⑦⑧⑨ | **145.500000 MHz** chosen. Digits: 1 Hz 0, 10 Hz 0, 100 Hz 0, 1 kHz 0, 10 kHz 0, 100 kHz 5, 1 MHz 5, 10 MHz 4. Packed as (10 Hz, 1 Hz)=`00`, (1 kHz, 100 Hz)=`00`, (100 kHz, 10 kHz)=`50`, (10 MHz, 1 MHz)=`45` |
| 16 nibble 1 | `0` | ⑩ | printed `0` with leader `1 GHz digit: (fixed)` |
| 16 nibble 2 | `1` | ⑩ | 100 MHz digit of 145.5 MHz = 1, inside the printed range `0 ~ 4` |
| 17 | `05` | ⑪ | `05:FM` |
| 18 | `01` | ⑫ | `01:FIL1` |
| 19 | `00` | ⑬ | `00: Data mode OFF` |
| 20 | `11` | ⑭ | nibble 1 `1` = `1=Duplex−`; nibble 2 `1` = `1=TONE` |
| 21 nibble 1 | `0` | ⑮ | `0=Digital squelch function OFF` |
| 21 nibble 2 | `0` | ⑮ | printed `0`, leader `Fixed` |
| 22 | `00` | ⑯ | both halves printed `0`, leaders `Fixed digit: 0*` |
| 23–24 | `08 85` | ⑰⑱ | **88.5 Hz** chosen. 100 Hz digit 0, 10 Hz digit 8 → `08`; 1 Hz digit 8, 0.1 Hz digit 5 → `85` |
| 25 | `00` | ⑲ | same fixed pair, second tone field |
| 26–27 | `08 85` | ⑳㉑ | 88.5 Hz again, same working |
| 28 | `00` | ㉒ | transmit polarity `0=Normal`, receive polarity `0=Normal` |
| 29 nibble 1 | `0` | ㉓ | printed `0`, leader `0 (fixed)` |
| 29 nibble 2 | `0` | ㉓ | DTCS **023** chosen; first digit 0, inside `0 ~ 7` |
| 30 | `23` | ㉔ | second digit 2, third digit 3 |
| 31 | `00` | ㉕ | DV digital code squelch **00**; first digit 0, second digit 0, both inside `0 ~ 9` |
| 32–33 | `00 60` | ㉖㉗ | **600 kHz** offset chosen. 1 kHz 0, 100 Hz 0 → `00`; 100 kHz 6, 10 kHz 0 → `60` |
| 34 nibble 1 | `0` | ㉘ | printed `0`, leader `10 MHz digit: (fixed)` |
| 34 nibble 2 | `0` | ㉘ | 1 MHz digit of a 600 kHz offset = 0 |
| 35–40 | `43 51 43 51 43 51` | ㉙–㉞ | UR call sign `CQCQCQ`. From `A ~ Z / 41 ~ 5A`: C is the 3rd letter, 0x41+2 = 0x43; Q is the 17th, 0x41+16 = 0x51 |
| 41–42 | `20 20` | ㉟㊱ | UR padded to the printed `8 characters, fixed`, from `(Space) / 20` |
| 43–50 | `20` ×8 | ㊲–㊹ | R1 left blank, eight spaces, filling `8 characters, fixed` |
| 51–58 | `20` ×8 | ㊺–52 | R2 left blank, eight spaces, filling `8 characters, fixed` |
| 59–105 | 47 bytes | printed ❻ ~ ❺❷ / measured 53–99 | byte-for-byte copy of frame bytes 12–58, per `The same data as ⑥ ~ 52 are stored in ❻ ~ ❺❷.` **STOP 1** |
| 106–107 | `4D 59` | 53, 54 | `MY`. M is the 13th letter, 0x41+12 = 0x4D; Y is the 25th, 0x41+24 = 0x59. **STOP 1** |
| 108 | `20` | 55 | the space. **Assumed — A2** |
| 109–116 | `52 45 50 45 41 54 45 52` | 56–63 | `REPEATER`. R = 0x41+17 = 0x52; E = 0x41+4 = 0x45; P = 0x41+15 = 0x50; A = 0x41; T = 0x41+19 = 0x54 |
| 117 | `20` | 64 | the second space. **Assumed — A2** |
| 118–119 | `43 48` | 65, 66 | `CH`. C = 0x43; H = 0x41+7 = 0x48 |
| 120–121 | `30 31` | 67, 68 | `01`, from `0 ~ 9 / 30 ~ 39` |
| 122 | `FD` | — | end of message, PDF 3 |

Name-length check: 2 + 1 + 8 + 1 + 2 + 2 = 16 characters, matching the printed
`(16 characters, fixed)`, and frame bytes 106 to 121 inclusive are 16 bytes.

### `read-transceiver-id` — 7 bytes

Reads the transceiver identification. Frame:

```
FE FE A4 E0 19 00 FD
```

| frame bytes | value | what it is | CSV rows |
|---|---|---|---|
| 1–2 | `FE FE` | preamble, PDF 3 | row 1 |
| 3 | `A4` | transceiver's default address, PDF 3 | row 2 |
| 4 | `E0` | controller's default address, PDF 3 | row 3 |
| 5 | `19` | command number; the command table prints `19*1` | row 4 |
| 6 | `00` | sub-command number; the same row prints `00` | row 5 |
| 7 | `FD` | end of message, PDF 3 | row 6 |

There is no data area, and none is assumed: the `Data` column cell of that
command-table row is **blank**, which the table states directly by leaving it
empty while filling that column on the rows above and below. The superscript key
on PDF page 17 reads `*1 Read only data`, consistent with a read-only command.

## Assumption register

Three runs in the CSV carry the status `inherited_assumed` — one in
`read-record` and two in `set-record-name-with-space` — and they raise two
distinct assumptions, A1 and A2, set out here in full. A2 covers both of the
`set-record-name-with-space` runs, which are the same assumption made twice in
one name. None of the three has a `pdf_page` or a `visual_anchor` in the CSV,
because nothing is printed for them.

### A1 — `read-record`, frame bytes 7–10, `00 00 00 12`

**What was assumed.** That a `1A 00` read request consists of the preamble, the
two addresses, `1A 00`, then exactly fields ① ② ③ ④ — the memory group number
and the memory channel number — and then `FD`, with no further data bytes; and
that the record so addressed is group `0000`, channel `0012`.

**Why that and not another.** The document is silent on the read form of
`1A 00`: PDF page 19 prints the full record for a write and a truncated form for
the clear operation, but never a read form, and PDF page 6 says only
`Send/read memory contents` with `See pp. 18 and 19.` Two things printed on
PDF page 19 bound the choice. First, ① ② ③ ④ are the only fields the legend
describes as identifying which record is meant — `Memory group number` and
`Memory channel numbers` — so any shorter prefix cannot name a record. Second,
the clear form printed on the same page, `①, ②: Memory channel group
(0000~0099) … ③, ④: Memory channel (0000~0099) … ⑤: "FF," ⑥ ~ : None`, shows
that this command does accept a data area shorter than the full 115 bytes and
that the truncation point sits immediately after this same address prefix. I
stopped at ④ rather than continuing to ⑤ because ⑤ carries a Split and Select
memory **setting**, a value a read has no business asserting, and because the
clear form's ⑤ is the specific value `FF` that means clear — sending any ⑤ at
all risks being read as that operation. The group and channel values were then
chosen inside the printed ranges: `0000` from `0000 ~ 0099: Memory channel
group`, and `0012` from `0000 ~ 0099: 00 ~ 99`, giving memory channel 12. The
document is silent; nothing here rests on what other radios do.

**The one capture that would settle it.** **Stage R capture** on an ic705: send
`FE FE A4 E0 1A 00 00 00 00 12 FD` and record what comes back on the CI-V line.
That single capture observes exactly one thing — whether this four-byte data
area is accepted as a read request for that record on that radio, shown by a
returned `1A 00` memory-content data block for channel 12, or refused, shown by
the NG message. It says nothing about any other command, any other data-area
length, or any other model.

### A2 — `set-record-name-with-space`, frame bytes 108 and 117, `20` and `20`

**What was assumed.** That the space character inside a `1A 00` memory name is
coded `20`.

**Why that and not another.** The memory name field's own reference is
`ⓘSee "Codes for character entries." (p. 19)`, and that section — the two tables
on PDF page 20 — prints `A ~ Z / 41 ~ 5A`, `a ~ z / 61 ~ 7A`, `0 ~ 9 / 30 ~ 39`
and thirty-four symbols, and prints **no row for a space**. The set-item table
on the same page says of `1A 00 / Memory name` only `All characters are
usable.`, which widens the permitted set without giving a code for any character
the tables omit. The document is silent on the code for a space in a memory
name. `20` was chosen because it is the value this same document prints for a
space in every other character set it does spell out — `Space | 20 | Word space`
for keyer memory character entries on PDF page 21, headed `Command: 1A 02`;
`Space | 20` in `Codes for CW message contents` on PDF page 18, headed
`Command: 17`; and `(Space) | 20` in `Character's code of the call sign` on
PDF page 24 — but each of those tables is headed by a different command and none
of them governs `1A 00`, so none is evidence for this field and the byte is
recorded as assumed rather than documented. No other value was plausible: every
other code printed in the `Codes for character entries` tables is already bound
to a visible glyph, so choosing one of them would put that glyph in the name
instead of a space.

**The one capture that would settle it.** **Stage R capture** on an ic705 whose
memory channel 12 has already been named, from the front panel, `MY REPEATER
CH01` — the two spaces typed as spaces: send `FE FE A4 E0 1A 00 00 00 00 12 FD`
and record the returned data block's bytes at name positions 3 and 12. That
single capture observes exactly one thing — which byte value that radio itself
stores in a memory-name position that displays as a space. It says nothing about
any other field, any other command, or any other model.

## Hardware status

UNVERIFIED. No ic705 has ever been asked anything by this project. Every vector
here is derived from printed documentation alone.

## Attestation

Every value recorded here was read from this single PDF's rendered page images.
`pdftotext -layout` was run on this same PDF for navigation only and was the
source of no recorded value. Nothing else was consulted: no other file, manual,
transcription, source file, generated artefact or web resource was opened, and
no directory was listed.
