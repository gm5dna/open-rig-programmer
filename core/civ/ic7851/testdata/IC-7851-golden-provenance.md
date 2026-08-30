# IC-7851 golden vectors — provenance

Quarantine leg G. Three vectors built by hand from one PDF, from rendered page
images only.

## Source

- Title, as printed on the cover (PDF page 1): **"THE TRANSCEIVERS / IC-7850 /
  IC-7851 / Instruction Manual"**. The cover carries no chapter or section text.
- Revision code, as printed: **"A7205H-1EX-3"**, printed in small type at the
  foot of the cover page, in the lower right corner, on the first of three lines
  reading "A7205H-1EX-3 / Printed in Japan / © 2015–2018 Icom Inc."
- File path:
  `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7851_civ_IM_3.pdf`
- Page count: **283 PDF pages** (reported by `pdfinfo` on this same PDF; see
  `## Method`).
- The CI-V material is chapter 18, "CONTROL COMMAND". Its folios read "18-n" and
  the PDF page is 249 + n, confirmed by eye on every page I read (PDF 252 →
  "18-3" … PDF 263 → "18-14").

**Judgement call.** The brief names PDF pages 252–263 and instructs me to render
only the pages it names, yet `## Source` requires the title and revision code
"as printed on the cover". I rendered PDF page 1 at 150 dpi for that one purpose
and for nothing else. No byte of any vector comes from it. No other page outside
252–263 was rendered or read.

## Extent

Every page below is a PDF page number; the folio printed on it follows.

| PDF page | Printed folio | Rendered at | Read? | What it contributed |
|---|---|---|---|---|
| 1 | (none; cover) | 150 dpi | yes | `## Source` only: cover title, revision code A7205H-1EX-3. No byte. |
| 252 | 18-3 | 300 dpi | yes | Start of "◇ Command table". Nothing transcribed. |
| 253 | 18-4 | 300, 400, 500 dpi | yes | The command-table row `19` / `00` / "Read the transceiver ID" — the whole of `read-transceiver-id`'s command pair. Also the row `1A†` / `00` / "see p. 18-12" / "Send/read memory contents". |
| 254 | 18-5 | 300 dpi | yes | "Command table (continued)", 1A 05 set-mode items. Nothing transcribed. |
| 255 | 18-6 | 300 dpi | heading/folio band only | Command table continued. Nothing transcribed. |
| 256 | 18-7 | 300 dpi | heading/folio band only | Command table continued. Nothing transcribed. |
| 257 | 18-8 | 300 dpi | heading/folio band only | Command table continued. Nothing transcribed. |
| 258 | 18-9 | 300 dpi | heading/folio band only | Command table continued. Nothing transcribed. |
| 259 | 18-10 | 300 dpi | yes | End of the command table. Nothing transcribed. Establishes that no CI-V frame-format diagram falls inside 252–263. |
| 260 | 18-11 | 300, 400, 500 dpi | yes | "• Operating frequency" 5-byte diagram (record fields ④–⑧); "• Operating mode" 2-byte diagram and its enum table (fields ⑨, ⑩); the "- Character codes" table under "• Memory keyer content" which prints `space | 20 | Word space` (field ㉓). |
| 261 | 18-12 | 300, 400, 500 dpi | yes | "- Character codes— Letters" (`A–Z | 41–5A`) and "- Character codes— Symbols"; the "Command | Set item/usable characters" table row `1A 00 | Memory name / All characters are usable.` (fields ⑱–㉒, ㉔–㉗). |
| 262 | 18-13 | 300, 400, 500 dpi | yes | "• Repeater tone/tone squelch frequency settings" 3-byte diagram (fields ⑫–⑭ and ⑮–⑰); the CW-message table which prints `Space | 20` a second time. |
| 263 | 18-14 | 300, 400, 600 dpi | yes | "• Memory content setting / Command: 1A 00" and its 18-cell data-block diagram with brackets ①–㉗; the ⑪ "Data mode and tone type settings" sub-diagram; the field texts for ①②, ③, ④–⑧, ⑨⑩, ⑫–⑭, ⑮–⑰, ⑱–㉗. |

**Where the transcribed material begins and ends.**

- PDF 263: the memory-record material begins with the running head "18 CONTROL
  COMMAND" at the top of the page, then "◇ Data content description (continued)"
  / "• Memory content setting" / "Command: 1A 00", then the data-block diagram.
  It ends with the paragraph "⑱~㉗ Memory name settings / Up to 10 characters. /
  See '• Codes for the memory name, opening message, NTP server address, CLOCK2
  name, network name, and network radio name contents.'" in the right column.
  The next thing printed after it in the left column is "• Main or Sub band's
  frequency settings / Command : 25"; the right column ends there.
- PDF 260: the operating-frequency material begins immediately under the running
  head "CONTROL COMMAND 18" with "◇ Data content description" / "• Operating
  frequency" / "Command: 00, 03, 05, 1C 03". It is followed by "• Operating
  mode / Command: 01, 04, 06". The memory-keyer character-code table is
  immediately preceded by "- Character codes" under "• Memory keyer content /
  Command: 1A 02" and is the last item in the left column.
- PDF 261: the character-code tables begin with "• Codes for the memory name,
  opening message, NTP server address, CLOCK2 name, network name, and network
  radio name contents" at the head of the right column and end with the
  "Command | Set item/usable characters" table; the next printed item is "• RX
  HPF/LPF setting for each operating mode".
- PDF 262: the repeater-tone material is the first item in the left column,
  preceded only by the running head, and is followed by "• SSB/SSB-D
  transmission passband width settings".
- PDF 253: the `19 00` row sits between "18 | 01 | Turn ON the transceiver*2"
  above it and "1A† | 00 | see p. 18-12 | Send/read memory contents" below it.

## Method

**Tools.** ImageMagick was available (`/opt/homebrew/bin/magick`) and was used
for every crop and enlargement. `tesseract` was available
(`/opt/homebrew/bin/tesseract`) but was **not used**: every value was legible by
eye on an enlarged render, so no OCR aid was needed and no OCR value was
recorded. **`pdftotext` was never run**, in any form, on this or any other file.

**Locate — 300 dpi.** A fresh directory `r300/` was created beneath this leg's
output directory and populated with

```
pdftoppm -png -r 300 -f 252 -l 263 <pdf> r300/p
```

Those twelve renders were read as images to find which page carries which
section. PDF 263 carries the memory-record data block; PDF 261 the memory-name
character codes; PDF 260 the operating-frequency and operating-mode structures
the record refers out to; PDF 262 the repeater-tone/tone-squelch structure; PDF
253 the `19 00` command-table row. PDF 252–259 are the command table and
contributed nothing but that negative finding. I worked only inside the section
whose printed heading matched: for the record, the heading "• Memory content
setting"; the adjacent "• Main or Sub band's frequency settings" (Command 25)
and "• Band stacking register" (Command 1A 01) blocks resemble it and were not
used as a source for any byte.

**Read — 400 dpi.** Pages 253 and 260–263 were re-rendered into `r400/` with
`-r 400`, and every recorded value was read from those.

**Crop and enlarge.** Each diagram, numbered band and legend was cropped into its
own image under `crops/` and enlarged, e.g.

```
magick r400/p-263.png -crop 2400x300+500+660  +repage -resize 200% crops/p263-block-full.png
magick r400/p-263.png -crop  820x300+540+660  +repage -resize 300% crops/p263-blk-a.png
magick r400/p-263.png -crop  820x300+1300+660 +repage -resize 300% crops/p263-blk-b.png
magick r400/p-263.png -crop  820x300+2060+660 +repage -resize 300% crops/p263-blk-c.png
magick r400/p-263.png -crop 1500x420+1600+940 +repage -resize 250% crops/p263-field11.png
magick r400/p-260.png -crop 1400x720+280+750  +repage -resize 200% crops/p260-opfreq.png
magick r400/p-260.png -crop 1250x700+400+1860 +repage -resize 220% crops/p260-opmode.png
magick r400/p-261.png -crop 1360x620+1660+560 +repage -resize 200% crops/p261-letters.png
magick r400/p-261.png -crop 1360x1200+1660+1140 +repage -resize 180% crops/p261-symbols.png
magick r400/p-261.png -crop 1400x1250+1660+2330 +repage -resize 180% crops/p261-setitem.png
magick r400/p-262.png -crop 1200x780+400+730  +repage -resize 230% crops/p262-tone.png
magick r400/p-253.png -crop 1500x260+1620+1640 +repage -resize 230% crops/p253-cmd19.png
```

At those enlargements every numeral, rule, dotted nibble divider, bracket tick
and glyph stood clear of its neighbours.

**Second independent pass — required, and done.** With the first pass complete,
every value was re-read from a **different raster**: PDF 263 re-rendered at
**600 dpi** and PDF 253 and 260–262 at **500 dpi**, with **different crop
windows and different enlargement factors** from the first pass. The record data
block, cut into thirds at 400 dpi in pass 1, was cut into halves at a different
split point at 600 dpi in pass 2; the operating-frequency diagram, read whole at
400 dpi in pass 1, was cut into two overlapping halves at 500 dpi in pass 2; and
so on. Pass-2 crops live under `pass2/`, e.g.

```
magick r600/p-263.png -crop 1500x520+850+990  +repage -resize 250% pass2/blk-A.png
magick r600/p-263.png -crop 1550x520+2250+990 +repage -resize 250% pass2/blk-B.png
magick r600/p-263.png -crop 1900x520+2550+1540 +repage -resize 220% -sharpen 0x1 pass2/f11.png
magick r500/p-260.png -crop  900x1000+340+900 +repage -resize 200% pass2/opfreq-L.png
magick r500/p-260.png -crop  950x1000+1180+900 +repage -resize 200% pass2/opfreq-R.png
magick r500/p-262.png -crop 1430x920+470+880  +repage -resize 190% pass2/tone.png
magick r500/p-261.png -crop 1780x810+2020+690 +repage -resize 190% pass2/letters.png
magick r500/p-253.png -crop 1800x400+2000+2000 +repage -resize 200% pass2/cmd19.png
```

**Cells where the two passes disagreed: none.** Every cell agreed — the count of
drawn cells in the record block (18), each bracket's span, every index numeral
and its style, every nibble label on the operating-frequency, repeater-tone and
⑪ diagrams, the direction of both leader lines in the ⑪ sub-diagram, the
operating-mode and filter enum values, the `A–Z | 41–5A` row, the absence of a
space row in the PDF 261 symbols table, the `space | 20 | Word space` row on PDF
260 and the `Space | 20` row on PDF 262, and the `19 | 00 | Read the transceiver
ID` row.

**Other commands run.** `pdfinfo` was run once on this same PDF; its only use was
the page count (283) quoted in `## Source`. No recorded byte came from it.
`ls` and `magick identify` were run on directories I had just created inside this
leg's own output directory, to confirm my own renders existed and their pixel
dimensions; no directory of source material was listed and no other file was
opened.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every
  index numeral in the PDF 263 record data-block diagram is drawn in one style:
  an outlined circle enclosing plain digits — ①, ②, ③, ④, ⑧, ⑨, ⑩, ⑪, ⑫, ⑭, ⑮,
  ⑰, ⑱, ㉗. Two-digit indices sit in the same outlined circle as one-digit
  indices, at the same size; none is filled, reversed, bracketed or bold. The
  sub-diagrams on PDF 260, 262 and 263 use the same outlined-circle style
  throughout. Nothing was normalised, because nothing differed.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** The
  operating-frequency diagram (PDF 260) and the repeater-tone diagram (PDF 262)
  carry their digit labels rotated 90° and read bottom-to-top, one label per
  nibble, joined to their nibble by a vertical arrow. Every one of those labels
  was read from the render by following its arrow up into the box, at 400 dpi in
  pass 1 and 500 dpi in pass 2. I cannot report on how a text layer would order
  them, because no text extraction of any kind was performed.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In the ⑪ "Data
  mode and tone type settings" sub-diagram (PDF 263, right column), the two
  labels sit to the right of the box and their printed order runs opposite to
  the nibble order they point at. The **upper** label, "0: OFF, 1: TONE, 2:
  TSQL", is joined by a short elbow to the arrow entering the **right** (second)
  nibble. The **lower** label, "0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3", runs
  left along a longer line to the arrow entering the **left** (first) nibble.
  Each leader was followed by eye from label to cell, at 400 dpi and again at
  600 dpi. So: nibble 1 = data mode, nibble 2 = tone type. Reading the labels in
  printed order would invert the assignment.
- **(d) A printed index may differ from a field's measured position — NOT
  ENCOUNTERED.** The record's ⑮–⑰ block repeats the structure of its ⑫–⑭ block
  (both are the three-byte repeater-tone/tone-squelch frequency structure of PDF
  262). Both the printed index and the measured byte position are recorded for
  every field in the table under `## The vectors`. For every one of the 27
  fields the printed index equals the measured position; they are recorded
  separately regardless, and neither has been reinterpreted in the light of the
  other.

## STOP findings

1. **PDF page 260 (folio 18-11), right column: the index ⑪ is printed twice, for
   two different things, inside one numbering sequence.** Under "• Band stacking
   register / Command: 1A 01" the fields run ①, ② then ③–⑦, ⑧, ⑨, ⑩, ⑪–⑬,
   ⑭–⑯. The paragraph heading printed there is "⑩ Data mode and tone setting /
   1 byte data (XX)" — but the sub-diagram directly beneath that heading is
   captioned **⑪**, and the very next paragraph heading is "⑪–⑬ Repeater tone
   frequency setting". So ⑪ labels both the byte introduced as ⑩ and the first
   byte of the repeater-tone triple. This is a repeat of an index within one
   sequence and it stops.
   The same artwork — box, two arrows, two labels — is reprinted on PDF 263,
   where it is captioned ⑪ under the heading "⑪ Data mode and tone type
   settings" and is consistent. The nibble assignment recorded in the CSV is
   read from the **PDF 263** instance, transcribed exactly as seen there, and
   the two rows that carry it (`set-record-name-with-space`, byte 17, nibbles 1
   and 2) are marked `STOP 1`. Nothing has been repaired, interpolated or
   carried over.

No other STOP arose. My reasons for confidence on the rest: the record block's
18 drawn cells and its bracket spans agreed exactly between a 400 dpi and a 600
dpi raster cut at different split points; the index sequence ① … ㉗ on PDF 263 is
continuous, unrepeated and in order, and is independently corroborated by the
left-column note "(instead of the data ③ to ㉗)"; the arithmetic closes — 2 + 1 +
5 + 2 + 1 + 3 + 3 + 10 = 27 indices, drawn as 18 cells of which two are
explicit dotted ellipsis cells standing for the 3 elided frequency bytes and the
7 elided name bytes; and every numeral, rule and glyph I recorded was legible by
eye at 400 dpi enlarged, with no reliance on OCR.

## Observed disagreements

Recorded as printed, not resolved, and none of these stopped me.

1. **PDF 253 (18-4), command table, row `1A† | 00`: the "Data" cell reads "see
   p. 18-12", but the "• Memory content setting / Command: 1A 00" data-block
   diagram is printed on folio 18-14 (PDF 263), not 18-12.** Folio 18-12 (PDF
   261) does carry the memory-name character codes that the record's ⑱–㉗ field
   points at, so the reference is not empty, but it does not lead to the data
   block. The neighbouring rows are exact: `1A 01` says "see p. 18-11" and the
   band stacking register is on 18-11; `1A 02` says "see p. 18-11" and the
   memory keyer content is on 18-11. I judged this **not** a STOP because it
   contradicts no value I recorded — no byte of any vector depends on which
   folio the cross-reference names — and because both statements can be true of
   different parts of the same command's documentation.
2. **PDF 261 (18-12) prints no code for the space character, in either of the
   two character-code tables the memory-name field is sent to, while admitting
   space to that field by implication.** The "- Character codes— Letters" table
   prints only `A–Z | 41–5A` and `a-z | 61–7A`; the "- Character codes— Symbols"
   table prints 34 symbol rows and no space row. Yet the same page's "Command |
   Set item/usable characters" table says of `1A 00 | Memory name`: "All
   characters are usable.", and two other rows on that table ("Opening message",
   "Network radio name") end "and space are usable." The code for space is
   printed twice elsewhere in this chapter — `space | 20 | Word space` on PDF
   260 and `Space | 20` on PDF 262 — but for other commands' character sets.
3. **PDF 261 prints no code for the digits 0–9 in either table**, although
   "numbers" are named as usable in four of the six rows of the same page's
   "Set item/usable characters" table. (The digit codes `0–9 | 30–39` are
   printed on PDF 260 and PDF 262, again for other commands.) This is why the
   name chosen for the write vector contains no digit.
4. **The same index range is printed with two different dashes on one page.**
   On PDF 263 the diagram brackets read "④–⑧", "⑫–⑭", "⑮–⑰", "⑱–㉗" with an en
   dash, while the body text below reads "④~⑧", "⑫~⑭", "⑮~⑰", "⑱~㉗" with a wave
   dash. Recorded as printed; purely typographic and it changes no value.
5. **Two worked examples are printed in the rendered extent, but neither is a
   frame**, so neither produced a `manual-example-<n>` vector. On PDF 260: "For
   example, when sending/reading the oldest contents in the 21 MHz band, the
   code '0703' is used." — that is two data bytes of the 1A 01 band-stacking
   register, printed without command, addressing or terminator. Also on PDF 260,
   in the memory-keyer character table: "^ | 5E | e.g., to send B̄T̄, enter
   ^4254" — that is character data, again with no framing. I found no worked
   example frame of any kind anywhere in the pages I read, and so wrote no
   `manual-example` vector.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other
file, manual, transcription, source file, generated artefact or web resource was
opened, and no directory was listed.

(For the avoidance of doubt, and consistent with `## Method`: `pdftotext` was
never run; `pdfinfo` was run once on this same PDF for the page count alone and
was the source of no recorded value; and the only directories touched by `ls`
were ones I had just created inside this leg's own output directory to hold my
own renders.)

## The vectors

### Field-index table for the `1A 00` memory record (PDF 263)

Printed index as printed, and byte position measured on the render by counting
drawn cells left to right and expanding the two dotted ellipsis cells. Recorded
separately; not reconciled.

| Printed index | Drawn cell (of 18) | Measured byte position in the data block | What the page says it is |
|---|---|---|---|
| ① | 1 (shaded) | 1 | Memory channel numbers, high digit pair |
| ② | 2 (shaded) | 2 | Memory channel numbers, low digit pair |
| ③ | 3 (unshaded) | 3 | Select memory setting |
| ④ | 4 (shaded) | 4 | Operating frequency setting |
| ⑤ | (inside the dotted ellipsis cell 5) | 5 | Operating frequency setting |
| ⑥ | (inside the dotted ellipsis cell 5) | 6 | Operating frequency setting |
| ⑦ | (inside the dotted ellipsis cell 5) | 7 | Operating frequency setting |
| ⑧ | 6 (shaded) | 8 | Operating frequency setting |
| ⑨ | 7 (unshaded) | 9 | Operating mode setting |
| ⑩ | 8 (unshaded) | 10 | Operating mode setting |
| ⑪ | 9 (shaded) | 11 | Data mode and tone type settings |
| ⑫ | 10 (unshaded) | 12 | Repeater tone frequency setting |
| ⑬ | 11 (unshaded) | 13 | Repeater tone frequency setting |
| ⑭ | 12 (unshaded) | 14 | Repeater tone frequency setting |
| ⑮ | 13 (shaded) | 15 | Tone squelch frequency setting |
| ⑯ | 14 (shaded) | 16 | Tone squelch frequency setting |
| ⑰ | 15 (shaded) | 17 | Tone squelch frequency setting |
| ⑱ | 16 (unshaded) | 18 | Memory name settings |
| ⑲–㉖ | (⑲–㉕ inside the dotted ellipsis cell 17; ㉖ is not separately drawn) | 19–26 | Memory name settings |
| ㉗ | 18 (unshaded) | 27 | Memory name settings |

The ⑱–㉗ bracket spans three drawn cells: an unshaded byte cell, a dotted
ellipsis cell, and a final unshaded byte cell, exactly as the ④–⑧ bracket does.
Measured this way the block is **27 bytes**, which is what the bracket labels
say (① through ㉗) and what the left-column clearing note corroborates: "add the
code 'FF' after the memory channel number. (instead of the data ③ to ㉗)".

### Was any field's printed width conditional?

No. The only field that could be read as variable is the memory name, and the
page prints exactly one width for it: ten indices, ⑱ through ㉗, three drawn
cells with an ellipsis. The accompanying sentence "Up to 10 characters."
constrains what may be *put* in that field, not how many bytes the diagram
allots to it, and the document prints no second width and no shorter form. So
the record has **one** derived total length, and there is **one**
`set-record-name-with-space` vector, unsuffixed. The name chosen therefore fills
all ten bytes and needs no padding rule — the document prints none.

**Deliberately not built**, per the brief: no memory-clear frame (the page does
print one, at "To clear the memory channel contents, add the code 'FF' after the
memory channel number"), and no transceive frame of any kind (commands 00, 01
and 06 are printed as transceive commands on PDF 252).

### `read-record` — 9 bytes

The `1A 00` request that reads one memory record. Reads memory channel 1.

```
FE FE 8E E0 1A 00 00 01 FD
```

| Byte | Hex | CSV row | Why |
|---|---|---|---|
| 1–2 | FE FE | rows 1 | Assumed lead-in pair — A1. |
| 3 | 8E | row 2 | Assumed destination address — A2. |
| 4 | E0 | row 3 | Assumed source address — A3. |
| 5–6 | 1A 00 | row 4 | Printed verbatim on PDF 263 as "Command: 1A 00". |
| 7 | 00 | row 5 | ① — high pair of the four printed channel digits; 0001 = memory channel 1, chosen from the printed range 0001–0099. |
| 8 | 01 | row 6 | ② — low pair of the same four digits. |
| 9 | FD | row 7 | Assumed end-of-message byte — A4. |

The document prints no read form separate from the write form: the command-table
row is "Send/read memory contents" and there is one data-content diagram for
both. The read request here carries the command and the channel number ①②
identifying the record, and stops there. That shape follows the page's own
treatment of the channel number as separable from the rest of the record — the
clearing note replaces "the data ③ to ㉗" while keeping ①② — but the page does
not print the read frame itself, and the two channel bytes are marked
`manual_derived` (documented encoding, value chosen by me), not
`manual_documented`.

### `set-record-name-with-space` — 34 bytes

A `1A 00` write of one complete 27-byte record whose ten-character name is
`ALPHA BETA`: one space, at character 6, in the middle of the name.

```
FE FE 8E E0 1A 00 00 01 00 00 00 25 14 00 01 01 00 00 08 85 00 10 00 41 4C 50 48 41 20 42 45 54 41 FD
```

6 framing/command bytes + 27 data bytes + 1 terminator = 34.

| Byte | Hex | Index | CSV row | Why |
|---|---|---|---|---|
| 1–2 | FE FE | – | row 8 | Assumed lead-in pair — A1. |
| 3 | 8E | – | row 9 | Assumed destination address — A2. |
| 4 | E0 | – | row 10 | Assumed source address — A3. |
| 5–6 | 1A 00 | – | row 11 | Printed verbatim, PDF 263. |
| 7 | 00 | ① | row 12 | Memory channel 0001, high pair. |
| 8 | 01 | ② | row 13 | Memory channel 0001, low pair. Channel 1, from the printed range 0001–0099. |
| 9 | 00 | ③ | row 14 | Select memory setting = 00, chosen from the printed enum 00: OFF / 01: ★1 / 02: ★2 / 03: ★3. |
| 10 | 00 | ④ | row 15 | Operating frequency 14.250000 MHz: 10 Hz digit 0, 1 Hz digit 0. |
| 11 | 00 | ⑤ | row 15 | 1 kHz digit 0, 100 Hz digit 0. |
| 12 | 25 | ⑥ | row 15 | 100 kHz digit 2, 10 kHz digit 5. |
| 13 | 14 | ⑦ | row 15 | 10 MHz digit 1 (printed range 0–6), 1 MHz digit 4. |
| 14 | 00 | ⑧ | row 16 | Both halves printed as the literal digit 0: "1000 MHz digit: 0 (Fixed)", "100 MHz digit: 0 (Fixed)". `manual_documented`. |
| 15 | 01 | ⑨ | row 17 | Operating mode = 01: USB, from the printed table. |
| 16 | 01 | ⑩ | row 18 | Filter setting = 01: FIL1, from the printed table. |
| 17 n1 | 0 | ⑪ | row 19 | Data mode half (left nibble, printed first) = 0: OFF. `STOP 1`. |
| 17 n2 | 0 | ⑪ | row 20 | Tone type half (right nibble) = 0: OFF. `STOP 1`. |
| 18 | 00 | ⑫ | row 21 | Repeater tone, cell ① — both halves printed as the literal digit 0 ("Fixed digit: 0*"). `manual_documented`. |
| 19 | 08 | ⑬ | row 22 | 100 Hz digit 0 (printed range 0–2), 10 Hz digit 8. |
| 20 | 85 | ⑭ | row 22 | 1 Hz digit 8, 0.1 Hz digit 5 → repeater tone 88.5 Hz. |
| 21 | 00 | ⑮ | row 23 | Tone squelch, cell ① — literal 0, 0. `manual_documented`. |
| 22 | 10 | ⑯ | row 24 | 100 Hz digit 1, 10 Hz digit 0. |
| 23 | 00 | ⑰ | row 24 | 1 Hz digit 0, 0.1 Hz digit 0 → tone squelch 100.0 Hz. |
| 24–28 | 41 4C 50 48 41 | ⑱–㉒ | row 25 | `A L P H A`, from the printed range `A–Z | 41–5A` on PDF 261. |
| 29 | 20 | ㉓ | row 26 | The space in the middle of the name. |
| 30–33 | 42 45 54 41 | ㉔–㉗ | row 27 | `B E T A`, same printed range. |
| 34 | FD | – | row 28 | Assumed end-of-message byte — A4. |

**Working shown for the derived values.**

- *Frequency.* The record's bracket ④–⑧ spans five bytes and refers out to "•
  Operating frequency", whose diagram is five cells; record index ④ maps to
  frequency cell ①, ⑤ to ②, ⑥ to ③, ⑦ to ④, ⑧ to ⑤, in printed order.
  14.250000 MHz written out digit by digit is 1|4|2|5|0|0|0|0 for
  10 MHz|1 MHz|100 kHz|10 kHz|1 kHz|100 Hz|10 Hz|1 Hz. Placing each digit against
  its printed label gives 00 (10 Hz 0, 1 Hz 0), 00 (1 kHz 0, 100 Hz 0), 25
  (100 kHz 2, 10 kHz 5), 14 (10 MHz 1, 1 MHz 4), 00 (both fixed). The 10 MHz
  digit 1 sits inside the printed range 0–6.
- *Repeater tone 88.5 Hz.* Digits are 0|8|8|5 for 100 Hz|10 Hz|1 Hz|0.1 Hz, so
  cell ② = 08 and cell ③ = 85; cell ① is the printed fixed 00. The 100 Hz digit 0
  sits inside the printed range 0–2.
- *Tone squelch 100.0 Hz.* Digits are 1|0|0|0, so cell ② = 10 and cell ③ = 00;
  cell ① is the printed fixed 00.
- *Name.* PDF 261 prints the range `A–Z | 41–5A` for the memory name's character
  set, and admits the field with "1A 00 | Memory name / All characters are
  usable." Counting from the printed start of the range, A=41, B=42, E=45, H=48,
  L=4C, P=50, T=54 — all inside 41–5A. Ten characters exactly fill ⑱–㉗, so no
  padding rule is needed and none is assumed.
- *The space, byte 29.* This is the one character in the name whose code is not
  printed on the page the memory-name field points at. PDF 261's two
  character-code tables have no space row at all. The code is printed twice
  elsewhere in the same chapter and in the same "ASCII code" column heading:
  `space | 20 | Word space` on PDF 260 (memory keyer content, command 1A 02) and
  `Space | 20` on PDF 262 (CW message contents, command 17). Joining those to
  PDF 261's "1A 00 | Memory name / All characters are usable." gives 20 for a
  space in a memory name. That is a derivation from printed statements, so the
  byte is `manual_derived` and cited to PDF 260, not `manual_documented` — and
  the gap is written up as Observed disagreement 2. It is not an unmarked
  assumption: the value 20 is printed in this document.
- *Field ⑪, byte 17.* Both halves are 0. The assignment of which half means what
  was read off the leader lines, not off the label order — see Hazard (c). Left
  nibble = data mode (0: OFF), right nibble = tone type (0: OFF). Because both
  halves are 0 the byte would be 00 either way; the CSV records the assignment
  explicitly all the same, one row per nibble, so that a reader can see which
  half is which.

*Note on reconstructing the frame from the CSV.* Byte 17 appears in two rows,
once per nibble, so `bytes_hex` across all rows of this vector is not a plain
concatenation: the nibble columns must be honoured. Every other byte appears in
exactly one row. Byte coverage is complete and contiguous, 1 to 34, with no gap
and no overlap other than that one deliberate nibble split.

### `read-transceiver-id` — 7 bytes

The `19 00` transceiver-identification read.

```
FE FE 8E E0 19 00 FD
```

| Byte | Hex | CSV row | Why |
|---|---|---|---|
| 1–2 | FE FE | row 29 | Assumed lead-in pair — A1. |
| 3 | 8E | row 30 | Assumed destination address — A2. |
| 4 | E0 | row 31 | Assumed source address — A3. |
| 5–6 | 19 00 | row 32 | PDF 253 command table: Cmd. `19`, Sub Cmd. `00`, "Read the transceiver ID". |
| 7 | FD | row 33 | Assumed end-of-message byte — A4. |

That row's "Data" cell is empty, so no data byte follows the sub-command. The
`19` in its "Cmd." cell carries no dagger, unlike the `1A†` in the row
immediately below it; the footnote at the foot of the same page reads
"† Send/read data".

## Assumption register

Four runs are `inherited_assumed`. All four are framing or addressing. **The
document is silent** on every one of them within the pages this leg rendered
(PDF 252–263, the whole of the command table and the whole of the data-content
description). No preamble, address or end-of-message byte is printed anywhere in
that extent, and none of the four values below is stated by the document. They
are chosen so that the three vectors are plausible frames, and they are marked
here so that no reader mistakes them for evidence. I have not used any
recollection of this protocol or this radio as a substitute for a printed
statement.

### A1 — bytes 1 and 2 of every vector: `FE FE`

- **What was assumed:** that each frame opens with a two-byte lead-in, and that
  both bytes take the value FE.
- **Why that value and not another:** a lead-in has to be a value that cannot be
  confused with the start of a data field, and every data field in this chapter
  is printed as BCD digit pairs or as ASCII codes in the ranges 20–7E — so an
  0xFx value is the class of value a lead-in can safely take, and FE is the
  highest such value that is not the all-ones byte. I emphasise that this is my
  reasoning, not the document's: the document is silent.
- **The one capture that would settle it:** **Stage R capture "IC-7851 lead-in
  read"** — with a line monitor on the CI-V port of one IC-7851, record the
  bytes the radio itself transmits as it answers one single request. That
  capture would show the lead-in bytes at the head of that one reply. It would
  settle no more than that: it would not show what lead-in the radio accepts on
  an inbound frame, and it says nothing about any other model.

### A2 — byte 3 of every vector: `8E`

- **What was assumed:** that the third byte addresses the transceiver, and that
  an IC-7851 answers to 8E.
- **Why that value and not another:** the frame needs a destination address
  distinct from the controller's, and it must not collide with the lead-in or
  the terminator; 8E is in the range left free by those. The choice of 8E in
  particular is arbitrary — the document is silent, and I decline to justify it
  from memory.
- **The one capture that would settle it:** **Stage W capture "IC-7851 address
  probe"** — send one `1A 00` write frame to a single IC-7851 with 8E in the
  third byte and record whether that one frame is acted on. That capture would
  show only whether 8E addressed that particular radio in its then-current
  configuration. It would not establish a factory default, would not show what
  the radio does with any other address, and says nothing about any other model.

### A3 — byte 4 of every vector: `E0`

- **What was assumed:** that the fourth byte carries the controller's own
  address, and that E0 is an acceptable value for a controller to use.
- **Why that value and not another:** the controller's address must differ from
  the transceiver's and must not collide with the lead-in or terminator; E0 is
  in the range left free. Again the particular value is my choice; the document
  is silent.
- **The one capture that would settle it:** **Stage R capture "IC-7851 reply
  destination"** — send a single `19 00` request to one IC-7851 with E0 in the
  fourth byte and read the destination byte of the reply that comes back. That
  capture would show only which address that radio placed in that one reply's
  destination position. It would not establish that E0 is required, permitted in
  general, or used by any other model.

### A4 — the last byte of every vector: `FD`

- **What was assumed:** that a frame is closed by a single end-of-message byte,
  and that its value is FD.
- **Why that value and not another:** a terminator must be a value that cannot
  occur inside the data, which rules out the BCD and ASCII ranges every field in
  this chapter uses, and it must differ from the lead-in chosen in A1; FD is the
  next value below FE that meets both conditions. The document is silent.
- **The one capture that would settle it:** **Stage R capture "IC-7851
  terminator read"** — record the final byte of one single reply transmitted by
  an IC-7851. That capture would show only the byte that terminated that one
  reply. It would not show what terminator the radio requires on an inbound
  frame, and says nothing about any other model.

## Hardware status

UNVERIFIED. No IC-7851 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.
