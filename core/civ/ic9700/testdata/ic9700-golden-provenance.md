# IC-9700 CI-V golden vectors — provenance

## Source

- Document title as printed on the cover: **CI-V REFERENCE GUIDE**, above the ruled
  block printing **VHF/UHF ALL MODE TRANSCEIVER** and **IC-9700**, with **Icom Inc.**
  at the foot. The cover carries no revision code.
- Revision code as printed: **A7508-3EX-4**, printed at the foot of the left-hand
  column of the back cover (PDF page 28), immediately above the line
  "© 2019–2023 Icom Inc.   Mar. 2023".
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic9700_civ_rev4.pdf`
- Page count: 28 PDF pages.

Printed folios run one behind the PDF page number throughout the body: PDF page 2 is
folio 1 (Table of contents), PDF page 15 is folio 14. The cover (PDF page 1) and the
back cover (PDF page 28) carry no folio.


## Extent

All 28 PDF pages were rendered at 300 dpi so that no diagram could be missed. The
pages actually **read** were:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none | Cover: document title, model, publisher. |
| 2 | 1 | Table of contents; used only to fix the folio-to-PDF-page offset. |
| 4 | 3 | "About the data format": the two frame diagrams that give the preamble FE FE, the transceiver address A2, the controller address E0, the Cn/Sc positions and the end-of-message code FD. |
| 6 | 5 | Command table rows `19*1 / 00 / Read the transceiver ID` and `1A* / 00 / See pp. 14, 15 / Send/read memory contents`. |
| 8 | 7 | Set-mode material (command table for `1A 05`, sub commands 0090–0138). Read in full; it contributed no byte to any vector. The only entry that touches CI-V framing is `1A 05 0128`, "SET > Connectors > CI-V > CI-V USB/LAN→REMOTE Transceive Address (0000=00h to 0223=DFh in Hexadecimal)", which states a settable range and not a default, so it was not used. |
| 13 | 12 | Footnote key `*(Asterisk) Send/read data`, `*1 Read only data`, `*2 Send only data`; and inside footnote *4 the only worked example frame printed anywhere in the document. |
| 14 | 13 | "Operating frequency", "Operating mode" and "Duplex Offset frequency setting" formats, referenced by the memory-content legend. |
| 15 | 14 | "Memory content", Command: 1A 00 — the memory-record data-block diagram and its field legends. |
| 16 | 15 | "Codes for character entries", Command: 1A 00 — the letters/numbers and symbols tables and the "Command / Set item/selectable characters" table. |
| 21 | 20 | "Repeater tone/tone squelch frequency settings", "DTCS code and polarity setting", "DV Digital code squelch setting" and "Character code of the call sign", all referenced by the memory-content legend. |
| 28 | none | Back cover: revision code. |

Where the transcribed material begins and ends:

- **PDF page 15 (folio 14).** The material begins immediately below the section line
  `◇ Command formats (Continued)` and the bullet `• Memory content` / `Command: 1A 00`;
  what is printed immediately above is the running head **Remote control**. The
  two-row data-block diagram is followed by the field legends `(1) Frequency band
  setting` … `(52) ~ (67) Memory name setting`, the paragraph `To clear the memory
  channel contents on 1A 00:` and the grey **NOTE:** box; the page ends with the
  ⓘ note beginning "RPS can be set when DD mode is selected". The next printed
  bullet, at the top of PDF page 16, is `• Codes for character entries`.
- **PDF page 16 (folio 15).** The character material begins under
  `• Codes for character entries` / `Command: 1A 00, 1A 05 0144, 0151, 0182, 0259,
  0279, 0281, 0293, 0316 / 1A 05 0262 ~ 1A 05 0265, / 1A 05 0268 ~ 1A 05 0271` and
  runs through `- Character codes— Letters and Numbers`, `- Character codes— Symbols`
  and the `Command / Set item/selectable characters` table, whose last printed row is
  `0182 / SET > Time Set > Date/Time > NTP Server Address`. Immediately after it the
  left column is blank; the next printed bullet, in the right-hand column, is
  `• Memory keyer character entries` / `Command: 1A 02`.
- **PDF page 13 (folio 12).** The worked example sits inside footnote *4. Immediately
  above it is the bullet list ending `• 4800 bps: 5 "FE"s`; immediately below it is
  footnote `*5 To insert a counter, first clear the other channel's counter.`


## Method

1. **Locate.** `pdftoppm -png -r 300 -f 1 -l 28 <pdf> r300/p` into a freshly created
   directory `evidence/ic9700-G/r300`. Every page was read as an image at this
   resolution to find the diagrams and sections.
2. **Read.** Every page from which a value was taken was re-rendered at 400 dpi
   (`pdftoppm -png -r 400 -f <n> -l <n> <pdf> r400/p`) and every value was read from
   those renders or from crops of them.
3. **Crop and enlarge.** ImageMagick **was** available (`/opt/homebrew/bin/magick`)
   and was used throughout. Representative commands:
   - `magick r400/p-15.png -crop 1500x260+300+770 +repage -resize 250% crops/p15_row1a.png`
   - `magick r400/p-15.png -crop 900x520+250+2150 +repage -resize 250% crops/p15_field4box.png`
   - `magick r400/p-21.png -crop 1200x900+280+1780 +repage -resize 200% crops/p21_dtcs.png`
   - `magick r400/p-13.png -crop 1450x360+1680+1550 +repage -resize 220% crops/p13_example2.png`
   Each numbered band, each detail box and each legend was cropped into its own image
   and enlarged until every numeral, rule and glyph stood clear of its neighbours.
4. **`pdftotext -layout`.** It **was** run, on this same PDF only, twice: once over
   PDF pages 5–13 and once over the whole file, purely to find which page a heading or
   a footnote key was on (it is how footnote `*1 Read only data` and the phrase
   "Example: When using 4800 bps" were located). **It was navigational only and was
   the source of no recorded value.** No byte position, nibble label, numeral, field
   index, width, label or enum value in the CSV or in this note came from it.
5. **tesseract.** Available (`/opt/homebrew/bin/tesseract`) but **not used**. Every
   value was read by eye from an enlarged crop, so no OCR value needed confirming.
6. **Second independent pass.** After the first pass was complete, every value was
   re-read from a different raster: the pages were re-rendered at **600 dpi** into a
   separate directory `r600/`, and cropped into a separate directory `crops2/` with
   **different crop windows and different enlargement factors** from the first pass —
   for example the memory-content diagram, cropped in two windows at 400 dpi and
   250 %, was re-cropped in three overlapping windows at 600 dpi and 180 %, so that
   every cell fell at a different place in a differently sized image. The second pass
   re-read: the "Controller to IC-9700" and "IC-9700 to controller" cell rows; the
   command-table rows for `19 00` and `1A 00`; the footnote *4 example box; the
   operating-frequency, operating-mode and duplex-offset diagrams; every cell,
   bracket, index numeral and shading of both rows of the memory-content diagram; the
   letters/numbers and symbols tables and the `1A 00 / Memory name / All characters
   are usable.` row; the repeater-tone, DTCS and call-sign-character material; and the
   memory-content legends and NOTE box.
   **Cells where the two passes disagreed: none.** Every cell agreed.


## Hazards encountered

- **(a) Numeral styling varies within one diagram — ENCOUNTERED.** The memory-content
  diagram on PDF page 15 draws its index numerals in two styles. Most are outlined
  circled numerals: ①, ②, ③, ④, ⑤~⑨, ⑩, ⑪, ⑫, ⑬, ⑭, ⑮~⑰, ⑱~⑳, ㉑~㉓, ㉔ in
  row 1 and ㉕~㉗, ㉘~㉟, ㊱~㊸, ㊹~(51), (52)~(67) in row 2. One block, sitting
  between ㊹~(51) and (52)~(67), is drawn in filled/reversed circles: white digits on
  solid black discs, printed `❺ ~ ❺①`. The same two styles are used in the NOTE box
  below, which sets the outlined `⑤ ~ (51)` against the filled `❺ ~ ❺①` in one
  sentence. Both styles are recorded as drawn; `field_index` in the CSV writes the
  filled block as `5~51 (filled)` and the outlined one as `5~9`, `28~35` and so on. No
  meaning has been inferred for the styling beyond what the NOTE itself prints.
- **(b) Vector groups with rotated labels — ENCOUNTERED.** The leader labels on the
  frequency, mode, duplex-offset, repeater-tone, DTCS and DV-code-squelch diagrams
  (PDF pages 14 and 21) and on the frame diagrams (PDF page 4) are printed rotated 90°,
  running bottom-to-top. Every one of them was read from the render, following its
  leader from the label to the cell half it lands on. `pdftotext -layout` was run only
  for navigation, so its extraction order was never relied on; where its output
  happened to be visible during navigation it was plainly reordered relative to the
  page, which is consistent with the hazard.
- **(c) Leader-line label order reversed — ENCOUNTERED.** Twice. On PDF page 21, in
  "DTCS code and polarity setting", the two label texts are stacked with
  "Receive polarity: 0=Normal 1=Reverse" printed **above**
  "Transmit polarity: 0=Normal 1=Reverse", but their leaders run the other way: the
  upper label's leader lands on the **right** half of the first cell and the lower
  label's on the **left** half. Read in printed vertical order the labels would give
  the halves the wrong way round; followed by eye they give left = transmit,
  right = receive, which is what the CSV records. On PDF page 4 the two address labels
  sit between the two frame rows and their leaders cross on the way down, so
  "Transceiver's default address" points up to `A2` in position 3 of the
  Controller→IC-9700 row and down to `A2` in position 4 of the IC-9700→controller row.
  Both rows were read from the picture.
- **(d) Printed index differs from measured position — ENCOUNTERED.** The filled block
  prints the indices `5 ~ 51`, which repeat the outlined `5 ~ 51` of the first part of
  the record. Measured along the diagram it occupies record data bytes 52–98 (frame
  bytes 58–104), between ㊹~(51) which ends at record byte 51 and (52)~(67) which
  begins at record byte 99. Both are recorded and neither is reinterpreted in the light
  of the other: `field_index` carries the printed `5~51 (filled)` and the `notes`
  column carries the measured extent. The same is recorded as STOP 2.


## STOP findings

1. **PDF page 15, the detail box belonging to the heading "④ Select memory setting".**
   The heading in the left-hand column reads `(4) Select memory setting` in an outlined
   circle, but the two-half box drawn beneath it is captioned with an outlined circled
   **3**, not 4. Index ③ is already assigned, in the same diagram, to the second byte of
   the memory channel number (`(2), (3) Memory channel number`), so the caption both
   repeats an index already used and disagrees with the heading two lines above it. Why
   it stops: it is a discontinuity in the index sequence — an index printed twice, in
   two places, meaning two different fields. It is recorded, not resolved: the two CSV
   rows for frame byte 10 carry `field_index` = `4 | 3 (as captioned on the detail box)`
   and `STOP 1` in `notes`. The byte itself is unaffected — the main diagram band
   unambiguously prints `④` above the fourth cell, which is drawn `0:X`, matching the
   detail box's own contents.
2. **PDF page 15, the filled `❺ ~ ❺①` block in the second row of the memory-content
   diagram.** The indices 5 to 51 are printed twice in one diagram: once as outlined
   circled numerals over the first part of the record and once as filled/reversed
   circles over a block that sits, measured, at record bytes 52–98. Why it stops: the
   STOP rule names "an index printed twice with different styling" and "a repeat"
   explicitly. Nothing has been repaired: the CSV row for frame bytes 58–104 carries
   `field_index` = `5~51 (filled)`, the measured extent in `notes`, and `STOP 2`. The
   printed NOTE box on the same page does explain the intent ("The same data as ⑤ ~ (51)
   are stored in ❺ ~ ❺①"), and that NOTE is what the 47 bytes were derived from; the
   STOP records the styling repeat as seen, not a doubt about the derivation.

No other STOP arose. Reasons for confidence on the rest: the two rows of the
memory-content diagram were counted cell by cell in both passes and the index ranges
run 1…24 and 25…67 without gap, repeat (other than STOP 2) or reordering; the cell
counts and the printed index ranges agree everywhere (⑤~⑨ = 5 bytes drawn as
cell + ellipsis + cell, ㉘~㉟ = 8 drawn the same way, (52)~(67) = 16 drawn the same
way); the field totals add to 114 and the frame to 121 with nothing left over and no
overlap; and every numeral, `0`, `X` and rule was legible at 400 dpi enlarged and again
at 600 dpi enlarged.


## Observed disagreements

Recorded as printed, not resolved; none of these stopped the transcription.

- **PDF page 15, "To clear the memory channel contents on 1A 00:".** The paragraph
  lists `(2), (3) :Memory channel (0001~0099)` and `(4) : "FF," (5) ~ :None`, but says
  nothing about field ①, the frequency band setting, which the same diagram makes the
  first byte of the data area. No clear frame was built, so nothing turned on it.
- **PDF page 15 against PDF page 21, call-sign field names.** Page 15 calls ㊱~㊸
  "R1 (Access repeater) call sign setting" and ㊹~(51) "R2 (Gateway/Link repeater) call
  sign setting"; page 21 calls the same two fields "R1 (Access/Area repeater)" and
  "R2 (Link/Gateway repeater)".
- **PDF page 21, repeater-tone index ①*.** The first byte of the repeater-tone
  frequency carries the footnote "*Not necessary when setting a frequency", which reads
  as an optional byte — but that diagram's heading scopes it to "Command: 1B 00, 1B 01",
  and inside `1A 00` the same field is drawn as three separate cells with the printed
  range ⑮~⑰. The byte was included.
- **PDF page 16 against PDF page 21, character sets.** Three different character tables
  are printed for three different commands and they do not agree with one another: the
  `1A 00` set has letters, lower case, digits and 32 symbols but no space; the `1A 02`
  memory-keyer set has letters, digits, a space and seven symbols; the call-sign set
  has letters, digits, a space and `/` only. All three are printed within five pages of
  each other.
- **PDF page 4 against PDF page 13.** The frame diagram on page 4 shows a two-byte
  preamble, `FE FE`; the worked example on page 13 shows three `FE` cells, the first of
  them annotated `×5`, because footnote *4 requires extra `FE`s ahead of the standard
  format when sending the power-ON command `18 01` at a given baud rate. The two are
  about different things — the standard format and one command's exception — and the
  vectors of my own construction use the two-byte preamble of page 4.


## Attestation

Every value recorded here was read from this single PDF's rendered page images.
`pdftotext -layout` was run on this same PDF for navigation only and was the source of
no recorded value. Nothing else was consulted: no other file, manual, transcription,
source file, generated artefact or web resource was opened, and no directory was
listed.


## Conventions used in the CSV

- `field_index` gives the index **as printed on the diagram**. A run spanning several
  consecutive printed indices is written as the printed range, e.g. `5~9`, `28~35`.
  The repeated block whose indices are drawn as filled/reversed circles is written
  `5~51 (filled)` so that it is never confused with the outlined `5~51`.
- Where a run carries only one nibble, `bytes_hex` gives the whole byte that contains
  it; `first_nibble`/`last_nibble` say which half the run is about.
- `manual_documented` is used where the page prints that exact value for that exact
  field (a cell printed as a literal `0` with an arrow labelled "Fixed", or a command
  byte printed as `1A`/`00`). `manual_derived` is used where the page prints an
  encoding or a list of alternatives and **I** chose the value.
- In `manual-example-1` every byte is `manual_documented`, because that vector is a
  verbatim transcription of a printed frame rather than a frame I assembled; the
  `structural` status is reserved for framing and addressing bytes I placed into
  frames of my own construction on the authority of the PDF page 4 diagram.


## The vectors

### `read-record` — 10 bytes

Reads one memory record: the `1A 00` request naming a single memory channel.

| frame byte(s) | value | what it is |
|---|---|---|
| 1–2 | `FE FE` | Preamble code (fixed), PDF page 4. |
| 3 | `A2` | Transceiver's default address, PDF page 4. |
| 4 | `E0` | Controller's default address, PDF page 4. |
| 5–6 | `1A 00` | Command number and sub command number, "Send/read memory contents", PDF page 6. |
| 7 | `01` | Field ①, frequency band; **assumed to be present**, see A1. |
| 8–9 | `00 01` | Fields ②,③, memory channel 0001; **assumed to be present**, see A2. |
| 10 | `FD` | End of message code (fixed), PDF page 4. |

The document prints no read-request diagram for `1A 00`. It does print, for the
neighbouring command `1A 01`, "To read the contents, the register code should be added
after the frequency band code" — but that is a statement about `1A 01`, not about
`1A 00`, so it justifies nothing here. Bytes 7–9 are therefore marked
`inherited_assumed` and are entered in the Assumption register; they are the three
selector fields the memory-content diagram itself puts first (①, ②, ③), truncated at
the point where the record proper begins (④).

### `set-record-name-with-space` — 121 bytes

Writes one complete memory record whose 16-character name field contains a space in
the middle: `INVERNESS GB3CFR`, the space falling at name character 10 of 16, which is
printed index (61).

**Derivation of the length.** Every field in the memory-content diagram carries an
explicit printed index or index range, so each field's width is fixed and countable:

- Row 1: ① = 1, ②③ = 2, ④ = 1, ⑤~⑨ = 5, ⑩⑪ = 2, ⑫ = 1, ⑬ = 1, ⑭ = 1,
  ⑮~⑰ = 3, ⑱~⑳ = 3, ㉑~㉓ = 3, ㉔ = 1 → **24 bytes** (drawn as 22 cells; the ⑤~⑨
  group is drawn as cell + dotted ellipsis cell + cell).
- Row 2: ㉕~㉗ = 3, ㉘~㉟ = 8, ㊱~㊸ = 8, ㊹~(51) = 8, the filled ❺~❺① block =
  51 − 5 + 1 = 47, (52)~(67) = 16 → **90 bytes**.
- Data area = 24 + 90 = **114 bytes**.
- Frame = 2 preamble + 1 transceiver address + 1 controller address + 1 command +
  1 sub command + 114 data + 1 end-of-message = **121 bytes**.

**No field's printed width is conditional**, so there is exactly one derived total and
exactly one vector, unsuffixed. Three conditional widths **are** printed elsewhere in
this document, and none of them applies to `1A 00`:

- PDF page 14, "Operating mode": "Filter setting, (②) can be skipped with command 01
  and 06." Scoped to commands 01 and 06. In `1A 00` the legend prints "⑩, ⑪ Operating
  mode setting" and the diagram draws two cells, so both bytes are present.
- PDF page 21, "Repeater tone/tone squelch frequency settings": index ①* carries the
  footnote "*Not necessary when setting a frequency." Scoped to commands 1B 00 / 1B 01.
  In `1A 00` the diagram draws three cells for ⑮~⑰ and three for ⑱~⑳.
- PDF page 21, "DV TX call signs setting (24 characters or 8 characters)". Scoped to
  command 1F 01. In `1A 00` the legend prints "(8 characters; fixed)" three times, for
  ㉘~㉟, ㊱~㊸ and ㊹~(51).

**Byte-by-byte walk** (keyed to the rows of `ic9700-golden-assumptions.csv`, in the
same order):

- **1–2 `FE FE`**, **3 `A2`**, **4 `E0`** — framing and addressing, PDF page 4.
- **5–6 `1A 00`** — command and sub command; printed in the command table on PDF
  page 6 and again as "Command: 1A 00" on PDF page 15.
- **7 `01`** — ① frequency band. The legend prints `01: 144 MHz frequency band`,
  `02: 430 MHz`, `03: 1.2 GHz`; I chose 144 MHz.
- **8–9 `00 01`** — ②,③ memory channel. The legend prints `0001 ~ 0099: Memory channel
  1 to 99`; I chose channel 1, so the printed four-digit value 0001 packs two BCD
  digits per byte as `00 01`.
- **10 `00`** — ④ select memory setting. Left half is printed as a literal `0` with an
  arrow labelled "Fixed"; right half is chosen 0 = OFF from the bracketed list
  `0=OFF* / 1=★1 / 2=★2 / 3=★3`. Two CSV rows, one per nibble. See STOP 1.
- **11–15 `00 00 50 45 01`** — ⑤~⑨ operating frequency, 145.500000 MHz. Working:
  145 500 000 Hz has digit places 10⁹=0, 10⁸=1, 10⁷=4, 10⁶=5, 10⁵=5, 10⁴=0, 10³=0,
  10²=0, 10¹=0, 10⁰=0 (check: 1·10⁸ + 4·10⁷ + 5·10⁶ + 5·10⁵ = 145 500 000). The
  PDF page 14 diagram labels the halves of the five cells, in printed left-to-right
  order, 10 Hz / 1 Hz, 1 kHz / 100 Hz, 100 kHz / 10 kHz, 10 MHz / 1 MHz,
  1 GHz / 100 MHz — that is, cell *n* carries 10^(2n−1) in its left half and 10^(2n−2)
  in its right half. Hence `00`, `00`, `50`, `45`, `01`. 145.5 MHz lies inside the
  144.000000 ~ 148.000000 MHz range the PDF page 16 frequency-band table prints for
  code 01.
- **16 `05`** — ⑩ operating mode; PDF page 14 table row `05 :FM`.
- **17 `01`** — ⑪ filter setting; PDF page 14 table row `01:FIL1`.
- **18 `00`** — ⑫ data mode; legend prints `00: Data mode OFF`.
- **19 `00`** — ⑬ duplex and tone. Following the leaders by eye: the left half's arrow
  runs to `0=Duplex OFF / 1=Duplex− / 2=Duplex+ / 3=RPS`, the right half's to
  `0=OFF / 1=TONE / 2=TSQL / 3=DTCS`. Both chosen 0.
- **20 `00`** — ⑭ digital squelch. Left half chosen 0 = "Digital squelch function OFF";
  right half printed as a literal `0` with an arrow labelled "Fixed". Two CSV rows.
- **21 `00`** — ⑮, the first byte of the repeater-tone frequency; both halves printed
  as literal `0` with the rotated label "Fixed digit: 0*".
- **22–23 `08 85`** — ⑯,⑰. 88.5 Hz: 100 Hz digit 0, 10 Hz digit 8, 1 Hz digit 8,
  0.1 Hz digit 5, giving `08` and `85` in the printed half-order.
- **24 `00`**, **25–26 `08 85`** — ⑱~⑳ tone-squelch frequency, same printed format,
  same chosen 88.5 Hz.
- **27 `00`** — ㉑ DTCS polarity: left half transmit polarity 0 = Normal, right half
  receive polarity 0 = Normal (leaders followed by eye; see hazard (c)).
- **28 `00`** — ㉒: left half printed as literal `0` with the rotated label "0 (fixed)";
  right half chosen first digit 0. Two CSV rows.
- **29 `23`** — ㉓: second digit 2, third digit 3, i.e. DTCS code 023.
- **30 `00`** — ㉔ DV digital code squelch; first digit 0, second digit 0.
- **31–33 `00 60 00`** — ㉕~㉗ duplex offset, 600 kHz. Working: 600 000 Hz has
  100 Hz = 0, 1 kHz = 0, 10 kHz = 0, 100 kHz = 6, 1 MHz = 0, 10 MHz = 0; the PDF
  page 14 diagram labels the halves 1 kHz / 100 Hz, 100 kHz / 10 kHz, 10 MHz / 1 MHz,
  giving `00`, `60`, `00`.
- **34–41 `43 51 43 51 43 51 20 20`** — ㉘~㉟ UR call sign `CQCQCQ` + two spaces.
  From the PDF page 21 "Character code of the call sign" table: A~Z = 41~5A, so
  C = 41 + 2 = 43 and Q = 41 + 16 = 51; (Space) = 20 is printed in that same table.
- **42–49 `20` ×8** — ㊱~㊸ R1 call sign, blank.
- **50–57 `20` ×8** — ㊹~(51) R2 call sign, blank.
- **58–104** (47 bytes) — the filled ❺~❺① block, a byte-for-byte copy of frame bytes
  11–57 (record fields ⑤~(51)), on the authority of the printed NOTE "The same data
  as ⑤ ~ (51) are stored in ❺ ~ ❺①." See STOP 2.
- **105–113 `49 4E 56 45 52 4E 45 53 53`** — name characters 1–9, `INVERNESS`, from
  the PDF page 16 A~Z = 41~5A row.
- **114 `20`** — name character 10, the space. **Assumed**; see A3.
- **115–120 `47 42 33 43 46 52`** — name characters 11–16, `GB3CFR`: G, B, C, F, R
  from the A~Z row and 3 from the 0~9 = 30~39 row of the same PDF page 16 table.
- **121 `FD`** — end of message code (fixed).

The memory-name field is 16 characters fixed and the chosen name is exactly 16
characters long, so no padding convention had to be invented; the only space in the
frame's name field is the deliberate one in the middle.

### `read-transceiver-id` — 7 bytes

`FE FE A2 E0 19 00 FD`. Framing and addressing from PDF page 4; `19` and `00` printed
in the command table on PDF page 6 as `19*1 / 00 / Read the transceiver ID`, with the
key on PDF page 12 giving `*1 Read only data`. No data area, so no assumed byte.

### `manual-example-1` — 8 bytes

`FE FE FE A2 E0 18 01 FD`, transcribed cell by cell from the only worked example frame
printed in this document: PDF page 13, footnote *4, box headed **Example: When using
4800 bps**. Its cells, left to right, are: a bold `F E` cell whose header cell is blank
and beneath which `×5` is printed; two cells spanned by the header **Preamble**, `F E`
and `F E`; **9700's address** `A 2`; **Controller's address** `E 0`; **Command** `1 8`;
**Sub command** `0 1`; **Post amble** `F D`. Eight printed byte cells, transcribed as
eight bytes. The `×5` beneath the first cell is the printed repetition count that
footnote *4 explains ("• 4800 bps: 5 'FE's"); the vector reproduces what is printed in
the cells and does not expand it, and the CSV row for byte 1 says so.


## Assumption register

**A1 — `read-record` byte 7, `01`.**
*What was assumed.* That a `1A 00` read request carries field ① (frequency band) as its
first data byte, and that the value for the 144 MHz band is `01`.
*Why that value and not another.* The document is silent on the format of a `1A 00`
read request: PDF page 15 prints one diagram, and it is the record layout, not a
request layout. Within that layout ① is the first field and the legend prints exactly
three permitted values, `01: 144 MHz frequency band`, `02: 430 MHz frequency band`,
`03: 1.2 GHz frequency band`. `01` was chosen over `02` and `03` because the 144 MHz
band is the first printed and its range, 144.000000 ~ 148.000000 MHz on PDF page 16,
contains the operating frequency used in the write vector, so the two vectors address
the same band. Nothing printed says the byte belongs in a request at all; that is the
assumption.
*The one capture that would settle it.* **Stage R capture R-1 on an ic9700**: send
`FE FE A2 E0 1A 00 01 00 01 FD` on the CI-V bus and record every byte the radio sends
back. That single capture shows whether the radio answers this three-byte selector with
a memory-content reply or with the NG code `FA`, and, if a reply comes, what its own
field ① byte is. It settles nothing about any other command, any other band code or any
other radio.

**A2 — `read-record` bytes 8–9, `00 01`.**
*What was assumed.* That the same read request carries fields ②,③ (memory channel
number) as its second and third data bytes, and that memory channel 1 is written
`00 01`.
*Why that value and not another.* The channel number must identify the record, and ②,③
is the only field in the printed layout that does so; the legend prints
`0001 ~ 0099: Memory channel 1 to 99`, so 1 is written as the four BCD digits 0001,
packed two to a byte. Channel 1 was chosen over any other because it is the first
channel the printed range admits and because the paragraph "To clear the memory channel
contents on 1A 00" also uses the range 0001~0099, so a channel inside that range cannot
be one of the special program-scan-edge or call-channel numbers 0100~0107. As with A1,
the placement of these bytes in a request is not printed.
*The one capture that would settle it.* **Stage R capture R-1 on an ic9700** — the same
single capture named in A1: it observes whether the reply's own ②,③ bytes come back as
`00 01`. It cannot show what any other channel number, or any other model, would do.

**A3 — `set-record-name-with-space` byte 114, `20`.**
*What was assumed.* That the space character in a `1A 00` memory name is encoded `20`.
*Why that value and not another.* For command `1A 00` the document prints, under
"Codes for character entries", exactly two character tables: "Character codes— Letters
and Numbers" (A~Z = 41~5A, a~z = 61~7A, 0~9 = 30~39) and "Character codes— Symbols"
(32 rows: `!` 21, `#` 23, `$` 24, `%` 25, `&` 26, `\` 5C, `?` 3F, `"` 22, `'` 27,
`` ` `` 60, `^` 5E, `+` 2B, `−` 2D, `*` 2A, `/` 2F, `.` 2E, `,` 2C, `:` 3A, `;` 3B,
`=` 3D, `<` 3C, `>` 3E, `(` 28, `)` 29, `[` 5B, `]` 5D, `{` 7B, `}` 7D, `|` 7C, `_` 5F,
`~` 7E, `@` 40). **Neither table has a space row**, and the accompanying
"Command / Set item/selectable characters" table says only "1A 00 / Memory name /
All characters are usable." — so for this command the document is silent on the space.
The value `20` was chosen because it is the only code this document ever prints against
a space, and it prints it twice: PDF page 16, the "Memory keyer character entries"
table for command **1A 02**, row `space / 20 / Word space`; and PDF page 21, the
"Character code of the call sign" table used by command 1F 01 and by the call-sign
fields of this very record, row `(Space) / 20`. Neither of those tables is the `1A 00`
memory-name table, so applying `20` to the memory name is my inference and not a
printed statement. No other value was available to choose: no code in either `1A 00`
table is described as blank, and the byte cannot be omitted because the name field is
"16 characters; fixed".
*The one capture that would settle it.* **Stage R capture R-2 on an ic9700**: with
memory channel 1 named from the radio's own front panel so that its name contains a
space (for example `AB CD`), send `FE FE A2 E0 1A 00 01 00 01 FD` and record the
memory-content reply, then read the byte at the name position where the space was
typed. That single capture shows which byte value the radio itself uses for a space in
a `1A 00` memory name. It says nothing about any other character, any other field, any
other command or any other model.


## Hardware status

UNVERIFIED. No ic9700 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.
