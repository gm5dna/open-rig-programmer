# ic7300 golden vectors — provenance

## Source

Document title as printed on the cover (PDF page 1): **IC-7300**, above it
`HF/50 MHz TRANSCEIVER`, and in the black band `FULL MANUAL`; publisher line
`Icom Inc.` at the foot. There is no revision code on the cover.

Revision code as printed: **`A7292-4EX-12b`**, printed on the back cover
(PDF page 180, no printed folio) at the foot of the left column, immediately
above `© 2016–2024 Icom Inc.    Aug. 2024`.

File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300_fullmanual_ENG_12b.pdf`

Page count: **180 pages** (A4, 595.276 × 841.89 pt).

## Extent

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none | Cover: document title, model, `FULL MANUAL`. Read only for the `## Source` section. |
| 126 | `12-10` | Set-mode item **CI-V Address (Default: 94h)**, `"94h" is the default address of IC-7300`; also CI-V Baud Rate, CI-V Transceive, CI-V USB→REMOTE Transceive Address. Corroborates the transceiver address byte. |
| 160 | `19-2` | **`◇ Data format`** — the four frame diagrams. `Controller to IC-7300` supplies every framing and addressing byte: `FE FE` ①, `94` ②, `E0` ③, `Cn` ④, `Sc` ⑤, `Data area` ⑥, `FD` ⑦. |
| 162 | `19-4` | **`◇ Command table`** — the row whose cells read `19`, `00`, blank Data, `Read the transceiver ID`, and the row below it, whose cells read `1A*`, `00`, `p. 19-11`, `Send/read memory contents`. |
| 167 | `19-9` | **`• Operating frequency`** (five-cell BCD diagram) and **`• Operating mode`** (two-cell diagram plus the mode / filter code table). These are the sections PDF page 169 points at for record fields ④–⑧ and ⑨,⑩. |
| 168 | `19-10` | **`• Codes for character entries`** — the *Letters and Numbers* table (`A–Z 41–5A`, `0–9 30–39`, `a-z 61–7A`), the *Symbols* table, and the `Command / Set item/selectable characters` table whose first row reads `1A`, `00`, `Memory name` / `All characters are usable.` |
| 169 | `19-11` | **`• Memory content / Command : 1A 00`** — the memory-record data block and its whole legend. This is the primary page for this leg. |
| 171 | `19-13` | **`• Repeater tone/tone squelch frequency settings / Command : 1B 00, 1B 01`** — the three-cell diagram PDF page 169 points at for record fields ⑫–⑭ and ⑮–⑰. Also, incidentally, `• Codes for CW message contents` (see `## Observed disagreements`). |
| 180 | none | Back cover: revision code. Read only for the `## Source` section. |

Where the transcribed material begins and ends on the primary page (PDF 169,
folio `19-11`):

- Immediately before it: the black section band `Remote control (CI-V)
  information`, and above that the running head `19  CONTROL COMMAND`.
- The material itself: the bold bullet heading `• Memory content`, the line
  `Command : 1A 00`, the single-row data-block diagram, and the two-column
  legend beneath it (`①, ②` through `⑱~㉗`, the clear-command recipe, and
  the `NOTE:` box).
- Immediately after it: nothing. The lower half of PDF page 169 is blank
  below the `NOTE:` box; the next printed thing on the page is the folio
  `19-11` centred at the foot.

The adjacent-heading hazard was live here and was checked. On PDF page 168
the heading `• Band stacking register / Command: 1A 01` sits directly above
`• Offset frequency settings / Command : 1A 05 …`, and on PDF page 171
`• Repeater tone/tone squelch frequency settings` sits between `• RIT
frequency settings / Command : 21 00` and `• UTC Offset setting / Command :
1A 05  00 96`. Every diagram transcribed here was matched to its own
`Command :` line on the render before any value was taken from it.

## Method

Every recorded value was read from a rendered page image. Steps, in order:

1. **Locate, 300 dpi.** Fresh directory `renders300/` under
   `evidence/ic7300-G`, nothing pre-existing:
   `pdftoppm -png -r 300 -f <p> -l <p> <pdf> renders300/p`
   for pages 126, 160, 162, 167, 168, 169, 171, plus the cover and back cover
   at 200 dpi. Whole pages read as images to find the named sections.
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f <p> -l <p> <pdf> renders400/p`
   for pages 160, 162, 167, 168, 169, 171. Page size at 400 dpi: 3308 × 4678 px.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`
   and `/opt/homebrew/bin/convert`) and was used throughout. First-pass crops,
   into `crops/`:
   - `magick renders400/p-169.png -crop 1300x230+440+900 +repage -resize 250% crops/d169_left.png`
   - `magick renders400/p-169.png -crop 1300x230+1700+900 +repage -resize 250% crops/d169_right.png`
   - `magick renders400/p-169.png -crop 700x230+1450+900 +repage -resize 400% crops/d169_mid.png`
   - `magick renders400/p-169.png -crop 1200x700+230+1500 +repage -resize 200% crops/p169_field3b.png`
   - `magick renders400/p-169.png -crop 1300x600+1750+1300 +repage -resize 200% crops/p169_field11.png`
   - `magick renders400/p-169.png -crop 1400x900+1700+1650 +repage -resize 180% crops/p169_rightcol.png`
   - `magick renders400/p-167.png -crop 1100x1050+180+1130 +repage -resize 230% crops/p167_freq.png`
   - `magick renders400/p-167.png -crop 1000x750+380+2280 +repage -resize 200% crops/p167_mode.png`
   - `magick renders400/p-168.png -crop 1400x1000+1700+740 +repage -resize 200% crops/p168_charA.png`
   - `magick renders400/p-168.png -crop 1400x1300+1700+1600 +repage -resize 200% crops/p168_charB.png`
   - `magick renders400/p-171.png -crop 1250x1300+230+2680 +repage -resize 180% crops/p171_repeater.png`

   Every numeral, rule and glyph was legible with clear separation at these
   enlargements; nothing had to be re-cropped for legibility, only for
   coverage.
4. **`pdftotext -layout`: run, navigational only.** It was run once,
   `pdftotext -layout -f 155 -l 175 <pdf> nav.txt`, and its output was used
   for exactly one purpose: to find which PDF page carries the headings
   `19 00 / Read the transceiver ID`, `• Operating frequency`, `• Operating
   mode`, `• Repeater tone/tone squelch frequency settings` and the CI-V
   `Data format` diagrams, by counting form feeds. **It was the source of no
   recorded value.** No byte position, nibble label, numeral, field index,
   width, label or enum value in the `.csv` or in this `.md` came from it;
   every such value was read by eye from a render.
5. **`tesseract`.** Available at `/opt/homebrew/bin/tesseract`. **It was not
   used.** Every value was legible by eye on the crops, so no OCR aid was
   needed and none was run.
6. **Second independent pass — done.** After the first pass was complete, a
   **different raster** was produced: pages 160, 162, 167, 168, 169 and 171
   re-rendered at **500 dpi** (`renders500/`, page size 4134 × 5847 px) and
   re-cropped with **different crop windows and different enlargement
   factors** into `crops2/`, deliberately not matching any first-pass window:
   - `magick renders500/p-169.png -crop 1900x300+560+1110 +repage -resize 170% crops2/A_left.png`
   - `magick renders500/p-169.png -crop 1900x300+2100+1110 +repage -resize 170% crops2/A_right.png`
   - `magick renders500/p-160.png -crop 1700x300+580+4020 +repage -resize 200% crops2/B_frame.png`
   - `magick renders500/p-162.png -crop 1720x120+280+2000 +repage -resize 300% crops2/C_1900.png`
   - `magick renders500/p-167.png -crop 1650x180+300+1280 +repage -resize 250% crops2/D_freqcells.png`
   - `magick renders500/p-171.png -crop 1300x200+500+3690 +repage -resize 280% crops2/E_tonecells.png`
   - `magick renders500/p-168.png -crop 1640x290+2180+1090 +repage -resize 220% crops2/F_letters.png`

   For exactness about the attestation below: the only directory listings run
   were `ls` and `magick identify` over `renders300/`, `renders400/`,
   `renders500/`, `crops/` and `crops2/` — directories this leg created
   itself, inside `evidence/ic7300-G`, containing nothing but renders of this
   PDF — to confirm which renders and crops had been produced and at what
   pixel size. No repository directory, no manual directory and no other
   location was listed, searched or browsed, and no file outside this PDF and
   this leg's own outputs was opened.

   The second pass re-read: the whole index sequence and every cell of the
   memory-content data block; the eight framing cells of `Controller to
   IC-7300`; the `19 | 00 | Read the transceiver ID` row; the five frequency
   cells including the two literal `0`s in the fifth; the three tone cells
   including the two literal `0`s in the first; and the Letters and Numbers
   table.

   **Disagreements between the two passes: none.** Every cell agreed:
   the cell counts 2 / 1 / (cell,ellipsis,cell) / 2 / 1 / 3 / 3 /
   (dotted placeholder) / (cell,ellipsis,cell); the index sequence
   ①,② ③ ④–⑧ ⑨,⑩ ⑪ ⑫–⑭ ⑮–⑰ ❹–⓱ ⑱–㉗ including the two numeral styles;
   `FE FE 94 E0 Cn Sc Data area FD`; `19` / `00`; `A–Z 41–5A`, `0–9 30–39`;
   the literal fixed `0` digits in both diagrams. No third render was needed.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — ENCOUNTERED.** The one
  data-block diagram on PDF page 169 draws its index numerals in **two
  styles**: outlined circled numerals for ①, ②, ③, ④–⑧, ⑨, ⑩, ⑪, ⑫–⑭, ⑮–⑰
  and ⑱–㉗, and **filled/reversed** circled numerals (white numeral on a
  solid black disc) for ❹–⓱. Both styles are recorded as drawn; they are not
  normalised, and no meaning is inferred for either style beyond what the
  page's own `NOTE:` box states in words.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.**
  The frequency diagram (PDF page 167), the tone diagram (PDF page 171) and
  the frame diagrams (PDF page 160) all carry their legends as labels rotated
  90°, one per leader arrow. Position was read from the picture in every
  case. The text layer was never consulted for any of them (see `## Method`
  step 4).
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** Twice, both
  on PDF page 169. For field ③ the label printed **higher** on the page (the
  bracketed list `0=OFF / 1= ★1 / 2= ★2 / 3= ★3`) is the one whose leader
  lands on the **right/second** nibble, while the label printed **lower**
  (`0=Split OFF, 1=Split ON`) lands on the **left/first** nibble. For field
  ⑪ the same reversal: the higher label `0: OFF, 1: TONE, 2: TSQL` lands on
  the **second** nibble and the lower label `0=Data mode OFF / 1=Data mode
  ON` lands on the **first**. Each leader was followed by eye from label to
  cell on `crops/p169_field3b.png` and `crops/p169_field11.png`, at 200%
  enlargement of a 400 dpi render.
- **(d) A printed index may differ from a field's measured position —
  ENCOUNTERED.** The block ❹–⓱ repeats the block ④–⑰. Recorded for every
  field of the repeating block, side by side and **not reconciled**:
  *printed index* ❹ … ⓱, i.e. the numerals 4 through 17 in filled/reversed
  style; *measured position* data-block bytes 18 through 31, that is frame
  bytes 24 through 37 of `set-record-name-with-space`. The CSV carries one
  row per field of that block with both facts. A caveat on the word
  "measured": the ❹–⓱ region is drawn as a single **undivided dotted
  placeholder** with no internal cell rules, so the per-field positions were
  taken from the bracket's extent and the count of printed indices within it
  (14), not from cell boundaries on the render; there are no cell boundaries
  there to measure. This is stated rather than smoothed over.

## STOP findings

1. **PDF page 169, folio `19-11`.** Visual anchor: the single-row data-block
   diagram under `• Memory content / Command : 1A 00`, and the row of index
   brackets printed immediately above it. What is printed, left to right:
   `①, ②` `③` `④–⑧` `⑨, ⑩` `⑪` `⑫–⑭` `⑮–⑰` `❹–⓱` `⑱–㉗`. **Indices 4 to 17
   are printed twice in one index sequence** — once as outlined circled
   numerals over discrete cells, and once, after ⑰ and before ⑱, as
   filled/reversed circled numerals over a dotted placeholder region. This
   trips three of the listed conditions at once: a repeat, an out-of-order
   index (❹ follows ⑮–⑰), and an index printed twice with different styling.
   It stops because a byte-position table keyed on the printed index cannot
   be single-valued: index 4 names both data-block byte 4 and data-block byte
   18. Transcribed exactly as seen: the fourteen CSV rows for frame bytes 24
   to 37 carry the printed indices `4 (filled/reversed circled numeral)`
   through `17 (filled/reversed circled numeral)` **and** their measured
   positions, with `STOP 1` in `notes`. Nothing was repaired, interpolated or
   renumbered. Note for the reader, not a resolution: the page's own `NOTE:`
   box states in words `The same data as ④–⑰ are stored in ❹–⓱`, which is
   why the repeated block's *values* could be derived — but the printed index
   sequence remains as printed, and the STOP stands.

No other STOP arose. Reasons for confidence on the rest:

- Every field's printed extent matches its printed index count. Counted on
  the render, twice, from two rasters: ①,② over 2 cells; ③ over 1; ⑨,⑩ over
  2; ⑪ over 1; ⑫–⑭ over 3 white cells; ⑮–⑰ over 3 shaded cells. No overlap,
  no gap, no bracket landing mid-cell.
- ④–⑧ (5 indices) and ⑱–㉗ (10 indices) are drawn abbreviated — first cell,
  a dashed-border `…` cell, last cell — so their drawn width is deliberately
  not their byte count. That is the diagram's own stated convention (the `…`
  cell is drawn differently from a data cell, with a dashed rather than solid
  border) and is not an arithmetic disagreement. The ❹–⓱ placeholder is the
  same convention taken further: a dotted region containing only a row of
  dots.
- The parts sum to the whole: 2 + 1 + 5 + 2 + 1 + 3 + 3 + 14 + 10 = 41 data
  bytes, and 27 + 14 = 41 printed index positions. Both counts were taken
  independently and agree.
- Every value transcribed was read cleanly at 400 dpi enlarged, and re-read
  cleanly at 500 dpi with different windows. Nothing was marginal; no cell is
  `UNREADABLE`.

## Observed disagreements

Recorded as printed. None of these stopped the work; none is resolved here.

1. **A cross-reference title that does not match the heading it points at.**
   PDF page 169 prints `See "• Repeater tone/tone squelch settings."` The
   heading actually printed on PDF page 171 is `• Repeater tone/tone squelch
   frequency settings` — the word `frequency` is present in the heading and
   absent from the cross-reference. There is no other section in the pages
   read whose heading is closer, so the cross-reference was followed to PDF
   page 171.
2. **A footnote on the referenced page whose scope is a different command.**
   The diagram on PDF page 171 that fields ⑫–⑭ and ⑮–⑰ are referred to is
   headed `Command : 1B 00,  1B 01`, and its first cell carries `①*` with the
   footnote `*Not necessary when setting a frequency.` and the two rotated
   labels `Fixed digit: 0*` twice. So on PDF page 171 that leading byte is
   printed as optional, while on PDF page 169 the corresponding record fields
   are printed as three numbered cells each (⑫,⑬,⑭ and ⑮,⑯,⑰) with no
   optionality marked and with ⑱ following immediately. Both statements are
   transcribed above as printed. They are recorded as odd rather than as a
   conflict because they are scoped to different commands — the footnote sits
   under `Command : 1B 00, 1B 01`, whereas the record is `1A 00` — and see
   `## Derived totals` below for what this does and does not do to the frame
   length.
3. **A gap in an enumeration.** The `① Operating mode` table on PDF page 167
   prints `00: LSB`, `01: USB`, `02: AM`, `03: CW`, `04: RTTY`, `05: FM`,
   `07: CW-R`, `08: RTTY-R`. **`06` is not printed.** The two spare cells in
   that column are struck through with a diagonal rule. Recorded as printed;
   no value was invented for 06 and none was used.
4. **A code for space printed in one character table but not the other.**
   The section PDF page 169 sends the memory-name field to,
   `• Codes for character entries` on PDF page 168, prints two tables — the
   Letters and Numbers table and the Symbols table — and **neither contains a
   row for a space**; the Symbols table's last row is `~ | 7E` and `@ | 40`.
   Yet the `Set item/selectable characters` table on the same page prints,
   for `1A 05 00 91 Opening message`, `Uppercase letters, numbers, symbols
   (− / . @) and space are usable.` — naming space as usable without giving
   its code. Separately, PDF page 171 prints, under the different heading
   `• Codes for CW message contents / Command : 17`, a table containing the
   row `Space | 20`. Recorded as printed; what was done about it is in
   `## Assumption register` A1.
5. **A blank Data cell in the command table.** On PDF page 162 the row
   whose cells read `19`, `00`, blank, `Read the transceiver ID` has an empty
   Data column, unlike its neighbours, whose Data column carries a value or a
   page reference such as `p. 19-11`. Read as "no data area", which is what
   `read-transceiver-id` was built to.

## Attestation

Every value recorded here was read from this single PDF's rendered page images. `pdftotext -layout` was run on this same PDF for navigation only and was the source of no recorded value. Nothing else was consulted: no other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

## Derived totals

Read as: how many `set-record-name-with-space` vectors this document
supports. **One.** The record has a single derived total length, so the
vector is named `set-record-name-with-space` with no numeric suffix.

The derivation, field by field, from the printed index brackets on PDF
page 169:

| field | printed indices | bytes |
|---|---|---|
| Memory channel numbers | ①, ② | 2 |
| Split and Select memory setting | ③ | 1 |
| Operating frequency setting | ④–⑧ | 5 |
| Operating mode setting | ⑨, ⑩ | 2 |
| Data mode and tone type settings | ⑪ | 1 |
| Repeater tone frequency setting | ⑫–⑭ | 3 |
| Tone squelch frequency setting | ⑮–⑰ | 3 |
| repeat of ④–⑰ | ❹–⓱ | 14 |
| Memory name settings | ⑱–㉗ | 10 |
| **data area total** | | **41** |

Frame total: `FE FE` (2) + `94` (1) + `E0` (1) + `Cn` (1) + `Sc` (1) +
41 + `FD` (1) = **48 bytes**.

Two candidate conditional widths were considered and both were rejected, on
the record so the reader can disagree with the judgement rather than have to
reconstruct it:

- **`⑱~㉗ Memory name settings / Up to 10 characters.`** The diagram prints
  ten index positions, ⑱ through ㉗. "Up to 10" is a maximum on the content,
  not a second documented width: the document names no shorter width, no
  padding character and no terminator, so there is no second total to derive
  from it, only a family of nine unstated ones. One width, 10 bytes. The
  vector accordingly carries a name of exactly ten characters so that nothing
  has to be padded.
- **The `*Not necessary when setting a frequency.` footnote on PDF page 171.**
  This is `## Observed disagreements` item 2. It is printed under
  `Command : 1B 00, 1B 01`, not under `1A 00`. Within the `1A 00` record the
  corresponding bytes carry their own printed index numbers — ⑫ and ⑮ — and
  dropping either would renumber every later field of the record, including
  the ⑱–㉗ name block and the ❹–⓱ repeat, none of which the page shows as
  variable. The record's printed widths are 3 and 3. Had this been read the
  other way it would have produced up to three further totals (46, 45 and 44
  bytes); none is written, because none is documented for `1A 00`.

**No `manual-example-<n>` vector was written**, because this document prints
no worked example frame. Two candidates were examined:

- PDF page 160 prints two frames with every byte literal and no placeholder:
  `FE FE E0 94 FB FD` labelled `OK message to controller` and
  `FE FE E0 94 FA FD` labelled `NG message to controller`. These were
  rejected as *format diagrams, not worked examples*: they sit under the
  heading `◇ Data format` alongside the `Controller to IC-7300` diagram,
  which is drawn identically but carries the placeholders `Cn`, `Sc` and
  `Data area`. The manual is printing a format there, not working an example.
  They are also both transceiver-to-controller responses.
- PDF page 168 prints the prose `For example, when sending/reading the oldest
  contents in the 21 MHz band, the code "07 03" is used.` That is a worked
  example of a two-byte **data-area code** for command `1A 01`, not a frame:
  it has no preamble, no addresses, no command byte and no terminator.

If the adjudicator reads either candidate as a worked example frame, the
transcriptions are above, byte for byte as printed.

## The vectors

### `read-record` — 9 bytes

`FE FE 94 E0 1A 00 00 01 FD`

Reads one memory record: the `1A 00` command with a data area containing only
the memory channel number, addressed from the controller to the transceiver.

| frame bytes | hex | what | CSV row |
|---|---|---|---|
| 1–2 | `FE FE` | preamble, index ① of the `Controller to IC-7300` frame diagram | `read-record,1,…` `structural`, PDF 160 |
| 3 | `94` | transceiver's default address, index ② | `structural`, PDF 160 |
| 4 | `E0` | controller's default address, index ③ | `structural`, PDF 160 |
| 5 | `1A` | command number `Cn`, index ④ | `manual_documented`, PDF 169 |
| 6 | `00` | sub command number `Sc`, index ⑤ | same row |
| 7–8 | `00 01` | data area, index ⑥: record fields ①,② — memory channel 01 | `manual_derived`, PDF 169 |
| 9 | `FD` | end of message, index ⑦ | `structural`, PDF 160 |

**Assumed frame shape, carrying no byte of its own.** The document prints no
read-*request* frame for `1A 00`. It prints the full data block, and it
prints one truncated form explicitly — `To clear the memory channel contents
on 1A 00: ①,②: Memory channel (00 01~00 99) / ③: "FF" / ④: None` — which
shows that a `1A 00` frame with a short data area is meaningful and that the
document specifies such a form by listing exactly which fields are present.
For a read it lists nothing, and a read must still name a channel, so the
data area here is fields ①,② and nothing more. That inference decides where
the frame *ends*; it does not choose the value of any byte, so it produces no
`inherited_assumed` run. It is flagged here so that it is not mistaken for
something the page states. The clear form itself was not built, as
instructed.

### `set-record-name-with-space` — 48 bytes

`FE FE 94 E0 1A 00 00 01 00 00 00 25 14 00 01 01 00 00 08 85 00 08 85 00 00 25 14 00 01 01 00 00 08 85 00 08 85 54 45 53 54 20 43 48 41 4E 31 FD`

Writes one complete memory record — all 41 data bytes, every printed field
present — into memory channel 01. The memory name is `TEST CHAN1`: ten
characters, with a space as the fifth, in the middle of the name.

Length: 6 framing and command bytes + 41 data bytes + 1 terminator = 48.
The derivation of 41 is the table under `## Derived totals`.

Byte-by-byte, keyed to the assumptions CSV (all runs are whole-byte — in this
record each printed index covers exactly one byte, so no run carries only
part of a byte and every `first_nibble`/`last_nibble` cell is `-`; where one
byte's two nibbles carry two separately labelled *sub*-settings, as in fields
③ and ⑪, both nibbles share one status and one source and so form one run,
with the split described in that row's `notes`):

| frame bytes | hex | field | status | source |
|---|---|---|---|---|
| 1–2 | `FE FE` | — | `structural` | PDF 160, preamble |
| 3 | `94` | — | `structural` | PDF 160, corroborated PDF 126 |
| 4 | `E0` | — | `structural` | PDF 160 |
| 5–6 | `1A 00` | — | `manual_documented` | PDF 169 `Command : 1A 00` |
| 7–8 | `00 01` | ①,② | `manual_derived` | PDF 169, channel 01 of `00 01–00 99` |
| 9 | `00` | ③ | `manual_derived` | PDF 169; nibble 1 `0`=Split OFF, nibble 2 `0`=select memory OFF |
| 10–13 | `00 00 25 14` | ④–⑦ | `manual_derived` | PDF 167; 14.250000 MHz in the printed nibble order |
| 14 | `00` | ⑧ | `manual_documented` | PDF 167; both nibbles printed as literal `0`, labelled Fixed |
| 15–16 | `01 01` | ⑨,⑩ | `manual_derived` | PDF 167; `01: USB`, `01: FIL1` |
| 17 | `00` | ⑪ | `manual_derived` | PDF 169; nibble 1 `0`=Data mode OFF, nibble 2 `0`=tone type OFF |
| 18 | `00` | ⑫ | `manual_documented` | PDF 171; both nibbles printed as literal `0` |
| 19–20 | `08 85` | ⑬,⑭ | `manual_derived` | PDF 171; repeater tone 088.5 Hz |
| 21 | `00` | ⑮ | `manual_documented` | PDF 171; same printed encoding |
| 22–23 | `08 85` | ⑯,⑰ | `manual_derived` | PDF 171; tone squelch 088.5 Hz |
| 24–37 | `00 00 25 14 00 01 01 00 00 08 85 00 08 85` | ❹–⓱ | `manual_derived`, 14 rows, **`STOP 1`** | PDF 169 `NOTE:` box |
| 38–41 | `54 45 53 54` | ⑱–㉑ | `manual_derived` | PDF 168, `A–Z 41–5A` |
| 42 | `20` | ㉒ | **`inherited_assumed`** | see A1 |
| 43–46 | `43 48 41 4E` | ㉓–㉖ | `manual_derived` | PDF 168, `A–Z 41–5A` |
| 47 | `31` | ㉗ | `manual_derived` | PDF 168, `0–9 30–39` |
| 48 | `FD` | — | `structural` | PDF 160 |

Workings shown for every `manual_derived` run:

- **Channel (bytes 7–8).** `00 01–00 99: Memory channel 01 to 99` is printed.
  Channel 01 chosen, giving `00 01`. Marked derived, not documented, because
  the channel was my choice even though this particular pair is also printed
  as the low end of the range.
- **Frequency (bytes 10–14).** The printed nibble legend, read left to right
  off the render, is: 10 Hz, 1 Hz | 1 kHz, 100 Hz | 100 kHz, 10 kHz | 10 MHz,
  1 MHz | 1000 MHz (Fixed 0), 100 MHz (Fixed 0). For 14.250000 MHz the digits
  are 1 Hz 0, 10 Hz 0, 100 Hz 0, 1 kHz 0, 10 kHz 5, 100 kHz 2, 1 MHz 4,
  10 MHz 1, 100 MHz 0, 1000 MHz 0. Packing two digits per byte in that order:
  `00`, `00`, `25`, `14`, `00`. The 10 MHz digit 1 is inside the printed range
  `0–6`; every other digit is inside `0–9`. Byte 14 is the one that is
  documented rather than derived, its two nibbles being printed as literal
  `0`s in the cell.
- **Mode (bytes 15–16).** `01: USB` from the `① Operating mode` column and
  `01: FIL1` from the `② Filter setting` column, both chosen from the printed
  tables.
- **Tone fields (bytes 18–23).** Printed nibble legend: `Fixed digit: 0*`,
  `Fixed digit: 0*` | `100Hz digit: 0–2`, `10 Hz digit: 0–9` | `1 Hz digit:
  0–9`, `0.1 Hz digit: 0–9`. For 088.5 Hz: 100 Hz digit 0, 10 Hz digit 8,
  1 Hz digit 8, 0.1 Hz digit 5, giving `00 08 85`. Used for both the repeater
  tone and the tone squelch. Every digit is inside its printed range.
- **Repeat block (bytes 24–37).** The `NOTE:` box on PDF page 169 states
  `The same data as ④–⑰ are stored in ❹–⓱`, and adds `Even if the Split
  function is OFF, enter the data into ❹–⓱ to match your transceiver. We
  recommend that you set the same data as ④–⑰.` The block is therefore the
  byte-for-byte copy of frame bytes 10–23. Fourteen CSV rows, one per printed
  index, each carrying both the printed index and the measured position, each
  marked `STOP 1`.
- **Name (bytes 38–47).** From the printed range `A–Z = 41–5A`, an offset
  arithmetic: `A` = 41, so `T` = 41 + 19 = 54, `E` = 41 + 4 = 45, `S` = 41 +
  18 = 53, `C` = 41 + 2 = 43, `H` = 41 + 7 = 48, `N` = 41 + 13 = 4E. From
  `0–9 = 30–39`: `1` = 30 + 1 = 31. Ten characters, so nothing is padded and
  no padding convention has to be assumed. The fifth, the space, is A1.

### `read-transceiver-id` — 7 bytes

`FE FE 94 E0 19 00 FD`

Reads the transceiver identification. No data area: the Data column of the
command-table row is blank.

| frame bytes | hex | what | status |
|---|---|---|---|
| 1–2 | `FE FE` | preamble | `structural`, PDF 160 |
| 3 | `94` | transceiver's default address | `structural`, PDF 160 |
| 4 | `E0` | controller's default address | `structural`, PDF 160 |
| 5–6 | `19 00` | `Cn` and `Sc`, from the command-table row whose cells read `19`, `00`, blank Data, `Read the transceiver ID` | `manual_documented`, PDF 162 |
| 7 | `FD` | end of message | `structural`, PDF 160 |

## Assumption register

One `inherited_assumed` run in the whole leg.

### A1 — `set-record-name-with-space`, frame byte 42, value `20`

**The byte.** Frame byte 42, one byte, hex `20`. Record field ㉒, the fifth
character of the ten-character memory name `TEST CHAN1` — the space.

**What was assumed.** That the code a controller sends in a memory-name
character position to store a space is `20`.

**Why the document does not settle it.** PDF page 169 sends this field to
`See "• Codes for character entries"`. That section, on PDF page 168, prints
two tables — Letters and Numbers, and Symbols — and **the document is silent**
on a code for a space in either: the Letters and Numbers table prints only
`A–Z 41–5A`, `0–9 30–39` and `a-z 61–7A`, and the Symbols table's rows run
from `! 21` to `~ 7E` and `@ 40` without a space row. The `Set item/selectable
characters` table on the same page prints, against `1A 00 Memory name`, only
`All characters are usable.` — which does not exclude a space, and does not
give its code either. The document is silent on the value.

**Why `20` and not another value.** Three reasons, in order of weight.
First, this same PDF does print a code for a space once, in a table headed
`• Codes for CW message contents / Command : 17` on PDF page 171, as the row
`Space | 20`; that table is scoped to a different command and so was not
treated as documenting this field — hence `inherited_assumed` and not
`manual_documented` — but it is the only value this document associates with
a space anywhere, and choosing a different one would contradict the only
figure the document prints. Second, the two ranges the memory-name field
*is* given, `A–Z = 41–5A` and `0–9 = 30–39`, are the ASCII code points for
those characters, and `20` is the ASCII code point for a space; a value
consistent with the encoding the field's own tables use is a better guess
than one that is not. Third, no other value is available to choose: nothing
in the pages read prints a printable placeholder, a pad character or an
alternative space code for a memory name. `FF` was rejected because PDF
page 169 gives `FF` a different job — it is the value field ③ takes to clear
a memory channel — and `00` was rejected because the document nowhere
associates it with a character.

**The one capture that would settle it.** A single **Stage R** capture on an
ic7300: with a memory channel already holding a name that contains a space in
a known position, send `FE FE 94 E0 1A 00 <channel> FD` and record the bytes
the radio returns in the ten-byte name field. Whatever byte appears at that
known position is the code this radio uses for a space in a memory name, read
from the radio itself. That single capture observes exactly that and no more:
it settles frame byte 42 of this vector, and it does not establish anything
about any other field, any other command, or any radio other than the one
captured.

## Hardware status

UNVERIFIED. No ic7300 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.
