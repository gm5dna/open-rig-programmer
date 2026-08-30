# IC-R8600 golden vectors — provenance

## Source

- Document title, as printed on the cover (PDF page 1): **CI-V REFERENCE GUIDE**, above the
  rule **COMMUNICATIONS RECEIVER / IC-R8600**, with **Icom Inc.** at the foot.
- Revision code, as printed: **A7375-2EX-3a**, printed at the foot of the left-hand column of
  PDF page 28 (the back cover, which carries no folio), directly above `© 2017–2018 Icom Inc.`
- File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/icr8600_civ_3a.pdf`
- Page count: 28 PDF pages.
- Folio relation: the printed folio is the PDF page number minus one throughout. Verified at
  four separate points — PDF page 2 prints folio `1`, PDF page 3 prints `2`, PDF page 12 prints
  `11`, PDF page 19 prints `18`. PDF page 1 (cover) and PDF page 28 (back cover) print no folio.

## Extent

Rendered at 300 dpi: PDF pages 1–28 (two runs, 1–16 then 17–28), into
`legs-out/icr8600/G/r300/`. Re-rendered at 400 dpi for transcription: PDF pages 3, 5, 9, 10,
11, 12, 13, 14, 15, 19, into `legs-out/icr8600/G/r400/`.

Read (that is, actually looked at as an image):

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none | Cover: document title, model, publisher. |
| 2 | 1 | Table of contents; used only to confirm the folio offset. |
| 3 | 2 | `◇ About the data format` — the two frame diagrams `Controller to IC-R8600` and `IC-R8600 to controller`. Source of every `structural` byte: `FE FE` preamble, `96` receiver address, `E0` controller address, `FD` end-of-message. |
| 4 | 3 | `◇ Command table` — the command-10 tuning-step sub-command list, source of the value `05` in field ⑲. |
| 5 | 4 | `◇ Command table (Continued)` — the row `19 / 00 / Read the receiver ID` and the row `1A / 00* / See pp. 11 ~ 14 / Send/read memory channel contents`. |
| 9 | 8 | `◇ Command table (Continued)`, right column, footnote `*1` and the block `Example: When using 4800 bps`. The only worked example frame printed anywhere in this guide; source of `manual-example-1`. |
| 10 | 9 | `◇ Command formats` — `Receiving frequency`, `Receiving mode` (mode/filter code table), `Offset frequency`. |
| 11 | 10 | `◇ Command formats (Continued)` — `Character entries`. This is the "character table" the brief points at: it lists which characters are selectable for MEMORY NAME and gives a total character number of 16, but it prints **no character codes**. |
| 12 | 11 | `● Memory channel content / Command: 1A 00` — the three-row data block ①–㊶ and the sub-diagrams for ⑤, ⑬, ⑱⑲, ⑳㉑, ㉒, ㉓, ㉔, ㉕. |
| 13 | 12 | Same heading continued — `For receiving an FM signal`, `For receiving a P25 signal`, `For receiving a D-STAR signal`. |
| 14 | 13 | Same heading continued — `For receiving a dPMR signal`, `For receiving an NXDN signal`. |
| 15 | 14 | Same heading continued — `For receiving a DCR signal`, and the memory-clear paragraph (read, **not** built). Right column is a different command (`1A 0B 00`) and was not used. |
| 19 | 18 | `Tone squelch (TSQL) frequency / Command: 1B 01` and `DTCS code / Command: 1B 02` — the encodings that PDF page 13 defers to for FM fields ㊸~㊺ and ㊻~㊽. |
| 21 | 20 | `GPS/D-PRS data— Position`, read only to check whether the guide ever names a character encoding. It does, for a different command family (`*9 ASCII characters (A ~ Z, 0 ~ 9, /, -, space)` under command `20 03`). No such annotation exists for `1A 00`. |
| 26 | 25 | `Scope waveform data`, checked for a second worked example. It contains prose (`Example: Sending the 5th data of divided into 11.`) but no example frame. |
| 28 | none | Back cover; revision code. |

Where the transcribed material begins and ends. The `1A 00` record layout begins on PDF page 12
immediately below the line `Command: 1A 00`, which itself sits below the bullet heading
`● Memory channel content` and below the section heading `◇ Command formats (Continued)`.
Immediately **before** it, on PDF page 11, is the `● Character entries` table (right column) and
the `● Scope/FSK FFT Scope waveform/FSK font color` diagram (left column). The layout ends on
PDF page 15 with the `For receiving a DCR signal` diagram and its five sub-diagrams; immediately
**after** it, in the same left column, is the paragraph beginning `Command 1A 00 clears a memory
channel by sending the command in the following format.` — the clear format, which this leg was
told not to build. The next bullet heading after that is `● Programmable scan start (remote)
data / Command: 1A 0B 00` in the right column, a different command.

## Method

Page images only. Every value recorded in the CSV was read from a rendered page image.

1. **Locate, 300 dpi.** `pdftoppm -png -r 300 -f 1 -l 16 <pdf> <out>/r300/p`, then
   `pdftoppm -png -r 300 -f 17 -l 28 <pdf> <out>/r300/p`, into a directory created fresh for
   this leg. Whole pages were read as images to find the sections whose printed headings match
   the task. The headings on PDF pages 12, 13, 14 and 15 are identical (`◇ Command formats
   (Continued)` / `● Memory channel content` / `Command: 1A 00`); the sections were told apart
   by their sub-headings (`For receiving an FM signal`, `… a P25 signal`, `… a D-STAR signal`,
   `… a dPMR signal`, `… an NXDN signal`, `… a DCR signal`) and by the running order of the
   circled indices.
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f <n> -l <n> <pdf> <out>/r400/p` for PDF pages 3,
   5, 9, 10, 11, 12, 13, 14, 15 and 19. Every recorded value was read from these.
3. **Crop and enlarge.** ImageMagick 7 was available (`/opt/homebrew/bin/magick`) and was used.
   Each diagram, cell row and leader legend was cut out and enlarged, e.g.

   ```
   magick r400/p-12.png -crop 1300x720+350+830  +repage -resize 220% crops/p12-block.png
   magick r400/p-12.png -crop 1300x1150+1650+1000 +repage -resize 200% crops/p12-ts.png
   magick r400/p-13.png -crop 1000x120+330+1090 +repage -resize 300% crops/p13-fm-row.png
   magick r400/p-14.png -crop 1000x540+1720+2700 +repage -resize 220% crops/p14-enckey.png
   magick r400/p-19.png -crop 1300x1000+1700+3020 +repage -resize 200% crops/p19-dtcs.png
   magick r400/p-10.png -crop 1300x1000+1750+830 +repage -resize 200% crops/p10-offset.png
   magick r400/p-09.png -crop 1400x350+1700+2540 +repage -resize 200% crops/p09-example.png
   ```

   All crops are kept in `legs-out/icr8600/G/crops/`. At these enlargements every numeral,
   box rule, dashed nibble divider and leader line stands clear of its neighbours.
4. **`pdftotext -layout`** *was* run, once, over the whole PDF into
   `legs-out/icr8600/G/nav.txt`, and was used **for navigation only** — to find which PDF page
   carried `FE FE`, `Example`, `Character entries`, `TSQL frequency`, `DTCS code` and `ASCII`.
   It was the source of no recorded value: no byte position, nibble label, numeral, field index,
   width, label or enum value in the CSV came from it. Every such value was read from a render.
5. **`tesseract`** was available but was **not** used. No OCR value appears in this leg.
6. **Second independent pass.** The first pass was made from the 300 dpi whole-page renders
   read at the reader's default downscale. The second pass was made from a different raster
   entirely: 400 dpi page renders, cut into per-diagram crops at 180–300 % enlargement, with
   crop windows chosen per diagram (so a different resolution, a different framing, and a
   different scale from the first pass). Both passes were done for every value recorded.

   Cells where the two passes disagreed:

   - **`Offset frequency`, PDF page 10, the eight rotated leader labels.** On the first pass
     (300 dpi whole page) the label block read as two adjacent lists and it was not possible to
     say with confidence whether `0 (Fixed)` was a label or a range, nor which nibble each label
     belonged to. On the second pass the crop `crops/p10-offset.png` shows eight straight,
     non-crossing vertical arrows, one per nibble, each carrying its own 90°-rotated label:
     nibble 1 `1 kHz digit: 0 ~ 9`, nibble 2 `100 Hz digit: 0 (Fixed)`, nibble 3 `100 kHz
     digit`, nibble 4 `10 kHz digit`, nibble 5 `10 MHz digit`, nibble 6 `1 MHz digit`,
     nibble 7 `1 GHz digit: 0 (Fixed)`, nibble 8 `100 MHz digit: 0 ~ 2`. The second raster
     settled it; the drawn box `X 0 | X X | X X | 0 X` corroborates it, since the two printed
     `0` nibbles fall exactly on the two `0 (Fixed)` labels. Not a STOP.
   - **`(20), (21) Programmable tuning step`, PDF page 12.** The first pass read the four
     labels in their printed top-to-bottom order (`10 kHz`, `100 kHz`, `100 Hz`, `1 kHz`) and
     could not tell which leader landed on which nibble. The second pass
     (`crops/p12-ts.png`, 200 %) shows the leftmost nibble's leader running down to the
     **bottom-most** label, so the drawn order is `1 kHz`, `100 Hz`, `100 kHz`, `10 kHz` — the
     exact reverse of the printed list. Settled by the second raster; corroborated by the
     `Offset frequency` diagram, which uses the same digit order for the same decades. Not a
     STOP.
   - **FM tail cell count, PDF page 13.** The first pass at 300 dpi gave an uncertain count
     between 7 and 8 cells. The third raster (`crops/p13-fm-row.png`, 400 dpi cropped to the
     cell row alone and enlarged 300 %) resolves eight solid vertical rules, hence seven cells,
     which agrees with the brackets ㊷ (1 cell) + ㊸~㊺ (3) + ㊻~㊽ (3). Settled. Not a STOP.

   No other cell disagreed between passes.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every field index on
  PDF pages 12, 13, 14 and 15 is drawn in exactly one style: a plain outlined circle enclosing
  a plain numeral, one to two digits, with no filled, reversed, bracketed or bold variant
  anywhere. The same style is used for the bracket end-points (`⑥ ~ ⑩`), for the standalone
  labels above single boxes, and for the indices of the neighbouring diagrams on PDF pages 10
  and 19. Nothing was normalised because nothing needed normalising.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** The `Receiving
  frequency` and `Offset frequency` diagrams on PDF page 10 draw every leader label rotated
  90° anticlockwise, one label per nibble, read bottom-to-top; the `Scope/FSK FFT Scope
  waveform/FSK font color` diagram on PDF page 11 does the same. Their positions were read from
  the picture, from the 400 dpi crops, and not from any text extraction.
- **(c) Leader-line label order may be reversed — ENCOUNTERED, repeatedly.** Wherever a
  diagram stacks its leader labels in a column to the right, the leftmost nibble's leader runs
  to the **bottom-most** label, so the printed top-to-bottom label order is the reverse of the
  drawn left-to-right nibble order. Confirmed by following each leader by eye on an enlarged
  crop for: `(20), (21) Programmable tuning step` (PDF 12), `(43) Digital code squelch (CSQL)
  code` and `(43) ~ (45) NAC` (PDF 13), `(43), (44) COM ID`, `(45) CC`, `(43) Radio Access
  Number (RAN) code` and `(45) ~ (47) Encryption key` (PDF 14), `(43), (44) UC code` and
  `(46) ~ (48) Encryption key` (PDF 15), `Tone squelch (TSQL) frequency` and `DTCS code`
  (PDF 19). In every case the drawn order is the ordinary big-endian-within-the-field order
  (100th, 10th, once), which the printed list prints backwards. The nibble numbers in the CSV
  are the **drawn** order, as the brief requires.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED.** Six
  blocks repeat one another: the six mode-class tails on PDF pages 13, 14 and 15 each restart
  at ㊷ immediately after the in-arrow. For every one, both were recorded: the printed index
  (㊷ in all six) and the measured byte position (the 42nd data byte, frame byte 48, in all
  six, measured by counting cells from the start of the three-row block on PDF page 12). They
  agree in all six cases, so nothing had to be reconciled. Note that other **commands** in this
  guide renumber the same content — `1A 0B 00` on PDF page 15 numbers the same settings ①~㉕,
  and `Memory scan start (remote) data` on PDF page 19 numbers the DCR tail ㉑~㉗ — but those
  are separate commands with their own diagrams, not a repeat inside the `1A 00` layout, and
  no byte here was taken from them.

## STOP findings

**None.**

Reasons for confidence:

1. **The arithmetic closes.** The three-row common block on PDF page 12 measures
   2 + 2 + 1 + 5 = 10 bytes in row 1, 2 + 1 + 4 + 2 = 9 in row 2, and
   2 + 1 + 1 + 1 + 1 + 16 = 22 in row 3 — 41 bytes, matching the printed index run ① … ㊶ with
   no gap, no repeat and nothing out of order. Each drawn cell carries exactly two nibbles, and
   each bracket's end-points land exactly on cell boundaries.
2. **Every tail closes too.** FM 1 + 3 + 3 = 7 cells for ㊷…㊽; P25 1 + 3 = 4 for ㊷…㊺;
   D-STAR 1 + 1 = 2 for ㊷㊸; dPMR 1 + 2 + 1 + 1 + 3 = 8 for ㊷…㊾; NXDN 1 + 1 + 1 + 3 = 6 for
   ㊷…㊼; DCR 1 + 2 + 1 + 3 = 7 for ㊷…㊽. In every case the cell count measured on the render
   equals the index count printed in the brackets.
3. **The index sequence is continuous everywhere.** ① to ㊶ in the common block, then ㊷ upward
   in each mode class, with no index printed twice, none skipped, and none printed in two
   different styles.
4. **Nothing printed contradicts anything else printed** among the material transcribed. The two
   oddities found are recorded below under `Observed disagreements`; neither is a contradiction.
5. **Every value was legible.** At 400 dpi enlarged 180–300 %, no numeral, letter, box rule or
   dashed divider was ambiguous. No cell is `UNREADABLE`.

## Observed disagreements

Recorded exactly as printed, not resolved.

1. **PDF page 13, right column, `(43) ~ (45) NAC`.** The third leader label is printed
   `100th posiiton: 0 ~ F` — `posiiton`, not `position`. The same diagram's other two labels
   read `10th position` and `Once position`, and every comparable diagram elsewhere in the guide
   spells it `position`. Transcribed as printed in the CSV `notes` for that run.
2. **PDF page 10, `Receiving mode` table, versus the record layout.** The table prints a long
   dash (`—`) in the `②Filter setting` column for `16:P25`, `17:D-STAR`, `18:dPMR`,
   `19:NXDN-VN`, `20:NXDN-N` and `21:DCR`, and the same dashes reappear on PDF page 26. Yet the
   record layout on PDF page 12 draws ⑪,⑫ as two bytes for every mode class without exception,
   and prints no rule for what ⑫ should carry when the mode is one of those six. The note
   beneath the table (`Filter setting (②) can be skipped with command 01 and 06`) is about
   commands 01 and 06, not about `1A 00`. This is why byte 18 is `inherited_assumed` in the
   five digital-mode vectors and `manual_derived` only in the AM and FM ones.
3. **PDF page 10, `Receiving mode` code list.** The codes are printed `00, 01, 02, 03, 04, 05,
   06, 07, 08, 11, 14, 15, 16, 17, 18, 19, 20, 21` — 09, 10, 12 and 13 are absent. The guide
   nowhere says whether these two-character codes are to be read as hexadecimal byte values or
   as decimal, and the gapped list is consistent with either reading. They are transcribed into
   the vectors exactly as the two characters printed (so D-STAR is the byte `17`, dPMR `18`,
   NXDN-VN `19`, DCR `21`), which is also what the neighbouring diagrams do with `00 = OFF`,
   `10 = 10dB`, `20 = 20dB`, `30 = 30dB` for ㉒.
4. **PDF page 12, the note under the three-row block.** It reads `In the modes other than FM
   and Digital, ㊷ and or later is not used. In the FM and Digital modes, entering ㊷ and or
   later can be omitted. The default value is applied to the omitted items.` — `and or later`,
   twice, where the sense is `and later` or `and those after it`. Transcribed as printed.
5. **PDF page 15, memory clear, versus PDF page 12, group numbers.** The clear paragraph says
   `①, ② : 0000 ~ 0101 group / You cannot specify group "0102" (Program scan edge)`, while the
   group list on PDF page 12 includes `0102: Programmable Scan Edge channel group`. These do
   not conflict — the clear command explicitly excludes a group the record layout allows — but
   the two are recorded here together because a reader comparing only the ranges would see a
   mismatch. No byte in this leg comes from the clear paragraph.

## Attestation

Every value recorded here was read from this single PDF's rendered page images. `pdftotext
-layout` was run on this same PDF for navigation only and was the source of no recorded value.
Nothing else was consulted: no other file, manual, transcription, source file, generated
artefact or web resource was opened, and no directory was listed.

## The vectors

Ten vectors. Three the brief names outright, plus four extra `set-record-name-with-space-<n>`
vectors because the record's tail length is conditional on the receiving mode, plus the one
worked example the guide prints.

The frame is the one drawn on PDF page 3 under `Controller to IC-R8600`:
`FE FE | 96 | E0 | Cn | Sc | Data area | FD`, with `96` labelled `Receiver's default address`
and `E0` labelled `Controller's default address`. So every record vector is
`7 framing/command bytes + <record length>`.

The record body used in all seven write vectors is identical apart from the mode byte, the
filter byte and the tail:

- ①,② group `00 00` (0000, Normal memory channel group)
- ③,④ channel `00 01` (0001)
- ⑤ `00` (SKIP OFF, select OFF)
- ⑥~⑩ `00 00 50 45 01` (145.500000 MHz in the printed digit order)
- ⑪,⑫ mode and filter — the only part that varies within the head
- ⑬ `00` (0 fixed, duplex OFF)
- ⑭~⑰ `00 00 00 00` (zero offset)
- ⑱ `01` (TS function ON), ⑲ `05` (5 kHz tuning step, from the command-10 list on PDF page 4)
- ⑳,㉑ `90 00` (programmable tuning step 9 kHz, in the drawn digit order)
- ㉒ `00` (ATT OFF), ㉓ `01` (0 fixed, preamp ON), ㉔ `00` (0 fixed, ANT1), ㉕ `00` (0 fixed, IP+ OFF)
- ㉖~㊶ `54 45 53 54 20 4E 41 4D 45 20 20 20 20 20 20 20` — the memory name
  `TEST NAME` followed by seven pad bytes. The space required by the brief is byte 36 of the
  frame, `20`, sitting between `TEST` and `NAME`. Every one of these sixteen bytes is
  `inherited_assumed`; see the register.

The nibble columns in the CSV use the drawn order: nibble 1 is the half printed on the left,
nibble 2 the half printed on the right. Where a run covers only one nibble, `bytes_hex` still
carries the whole byte the run sits in, and the nibble columns say which half of it the run
accounts for.

### `read-record` — 11 bytes

`FE FE 96 E0 1A 00 00 00 00 01 FD`

Reads memory group 0000, channel 0001. Bytes 1–4 are the frame from PDF page 3. Bytes 5–6 are
`1A 00`, printed as `Command: 1A 00` on PDF page 12. Bytes 7–10 carry ①~④ using the group and
channel encodings printed on PDF page 12, but the guide prints **no** read-request data area for
`1A 00` anywhere: that the request consists of ①~④ and then stops is a choice, so the whole run
is `inherited_assumed`. Byte 11 is the `FD` end-of-message.

### `set-record-name-with-space-48` — 48 bytes (mode class: everything other than FM and Digital)

`FE FE 96 E0 1A 00` + the 41-byte record head + `FD`, with ⑪ `02` (`02:AM`) and ⑫ `01`
(`01:FIL1`). This is the shortest derived total, because PDF page 12 states `In the modes other
than FM and Digital, ㊷ and or later is not used.` Byte-by-byte: CSV rows for this vector,
frame bytes 1–48.

### `set-record-name-with-space-50` — 50 bytes (mode class: D-STAR)

Head with ⑪ `17` (`17:D-STAR`), ⑫ `01` (assumed), then the two-byte D-STAR tail from PDF page
13: ㊷ `02` (0 fixed, `2=CSQL`) and ㊸ `12` (CSQL code, 10th position 1, once position 2, in the
drawn nibble order), then `FD`. 41 + 2 = 43 record bytes, 50 in the frame.

### `set-record-name-with-space-52` — 52 bytes (mode class: P25)

Head with ⑪ `16` (`16:P25`), ⑫ `01` (assumed), then the four-byte P25 tail from PDF page 13:
㊷ `01` (0 fixed, `1=NAC`) and ㊸㊹㊺ `02 09 03` — NAC 293h, each byte a fixed `0` nibble
followed by one hexadecimal digit, in the drawn order 100th, 10th, once. Then `FD`.
41 + 4 = 45 record bytes, 52 in the frame.

### `set-record-name-with-space-54` — 54 bytes (mode class: NXDN)

Head with ⑪ `19` (`19:NXDN-VN`), ⑫ `01` (assumed), then the six-byte NXDN tail from PDF page
14: ㊷ `01` (0 fixed, `1=RAN`), ㊸ `05` (RAN code, 10th position 0, once position 5), ㊹ `00`
(0 fixed, encryption OFF), ㊺㊻㊼ `00 00 00` (encryption key 00000, its leading nibble fixed
`0`). Then `FD`. 41 + 6 = 47 record bytes, 54 in the frame.

### `set-record-name-with-space-55-fm` — 55 bytes (mode class: FM)

Head with ⑪ `05` (`05:FM`), ⑫ `01` (`01:FIL1`), then the seven-byte FM tail from PDF page 13:
㊷ `01` (0 fixed, `1=TSQL`); ㊸㊹㊺ `00 08 85` — TSQL frequency 88.5 Hz, using the `1B 01`
encoding on PDF page 19 where byte ① is `0 (Fixed)` as a whole byte and the remaining four
nibbles are the 100 Hz, 10 Hz, 1 Hz and 0.1 Hz digits; ㊻㊼㊽ `00 00 23` — DTCS code 023,
polarity Normal, using the `1B 02` encoding on PDF page 19 (nibbles: fixed `0`, polarity, fixed
`0`, 100th, 10th, once). Then `FD`. 41 + 7 = 48 record bytes, 55 in the frame.

### `set-record-name-with-space-55-dcr` — 55 bytes (mode class: DCR)

Head with ⑪ `21` (`21:DCR`), ⑫ `01` (assumed), then the seven-byte DCR tail from PDF page 15:
㊷ `01` (0 fixed, `1=UC`), ㊸㊹ `01 23` (UC code 123, nibbles fixed `0`, 100th, 10th, once),
㊺ `00` (0 fixed, encryption OFF), ㊻㊼㊽ `00 00 00` (encryption key 00000). Then `FD`.
41 + 7 = 48 record bytes, 55 in the frame.

DCR and FM derive the same total, 55. The brief asks for one vector per derived total and names
them `set-record-name-with-space-<n>`; the Tier 4b clause asks for at least one vector per mode
class. Two mode classes land on 55, so a bare `set-record-name-with-space-55` cannot satisfy
both. The judgement taken here was to keep one vector per mode class and disambiguate the
colliding pair with a `-fm` / `-dcr` suffix, so that no derived total is missing and no mode
class is missing. Every other name is exactly `set-record-name-with-space-<n>`.

### `set-record-name-with-space-56` — 56 bytes (mode class: dPMR)

Head with ⑪ `18` (`18:dPMR`), ⑫ `01` (assumed), then the eight-byte dPMR tail from PDF page 14:
㊷ `01` (0 fixed, `1=COM ID`), ㊸㊹ `01 23` (COM ID 123), ㊺ `12` (CC 12, 10th position 1, once
position 2), ㊻ `00` (0 fixed, scrambler OFF), ㊼㊽㊾ `00 00 00` (scrambler key 00000). Then
`FD`. 41 + 8 = 49 record bytes, 56 in the frame — the longest derived total.

### `read-transceiver-id` — 7 bytes

`FE FE 96 E0 19 00 FD`

The command table on PDF page 5 prints one row `19 / 00 / <blank Data cell> / Read the receiver
ID`. The blank Data cell is why there is no data area between `00` and `FD`. Note that the
guide calls this the *receiver* ID, not the transceiver ID; the vector name is the brief's.
Every byte is either `structural` (from PDF page 3) or `manual_documented` (from PDF page 5).

### `manual-example-1` — 12 bytes

`FE FE FE FE FE FE FE 96 E0 18 01 FD`

The one worked example frame printed in this guide, on PDF page 9, headed `Example: When using
4800 bps`, illustrating footnote `*1` (`When sending the power ON command (18 01), you need to
repeatedly send "FE" before the standard format`). The table draws ten cells: a bold `F E` cell
annotated `×5` beneath it, then a cell headed `Preamble` printed `F E F E`, then `R8600's
address` `9 6`, `Controller's address` `E 0`, `Command` `1 8`, `Sub command` `0 1`, `Post amble`
`F D`. The `×5` annotation is expanded here into five `FE` bytes, giving twelve bytes in all;
that expansion is recorded in the CSV `notes` for the first run. It is neither a clear/erase
frame nor a transceive frame.

## Assumption register

Every `inherited_assumed` run, consolidated. The guide states none of these.

### A1 — the memory name characters (frame bytes 32–40 in all seven record vectors)

`54 45 53 54 20 4E 41 4D 45`, the nine characters `T E S T <space> N A M E`.

**What was assumed:** that the sixteen bytes of field ㉖~㊶ carry 7-bit ASCII, one character per
byte, so that `T` is `54`, the space is `20`, `N` is `4E`, and so on.

**Why that value and not another.** The document is silent. PDF page 11's `Character entries`
table tells you *which* characters may be entered for `MEMORY NAME` — printed there, over four
lines, as: A to Z, a to z, 0 to 9, (space), @ % & # + - = [ ] / ( ) : ; ^ ! ? < > . , " $ ' *
_ ` { | } ~ \ — and that the total
character number is 16, but it prints no code for any of them, and nowhere on PDF pages 12-15
is a code given either. ASCII was chosen because it is the only encoding this same guide names
for any text field at all — PDF page 21 annotates the D-STAR call-sign field of command `20 03`
`*9 ASCII characters (A ~ Z, 0 ~ 9, /, -, space)`, and other fields in that family
`*2 ASCII characters (00h ~ EFh)` — and because the printable set listed for `MEMORY NAME` maps
one-to-one onto printable ASCII with nothing left over. That reasoning is an inference from a
*different command's* annotation; the document is silent for `1A 00`. Any other single-byte
encoding sharing that repertoire would produce different bytes.

**The one capture that would settle it.** A **Stage R** capture on an IC-R8600: set one memory
channel's name from the front panel to `TEST NAME`, then send `FE FE 96 E0 1A 00 00 00 00 01 FD`
and record the reply. The bytes the receiver returns in positions ㉖~㊶ are the encoding. That
single capture settles the codes for the characters `T`, `E`, `S`, `A`, `M`, `N` and the space,
and nothing else — not the rest of the repertoire, not any other model.

### A2 — the memory name padding (frame bytes 41–47 in all seven record vectors)

`20 20 20 20 20 20 20`, seven bytes.

**What was assumed:** that a name shorter than sixteen characters is padded to the full sixteen
bytes with the space character, and that the pad character is the same `20` assumed in A1.

**Why that value and not another.** The document is silent on both halves of this. PDF page 11
prints `16` as the `Total character number` for `MEMORY NAME`, and PDF page 12 draws ㉖~㊶ as
sixteen cells with no shorter variant, so the field's *width* is fixed at sixteen; but nothing
says what occupies the cells a shorter name leaves over. Space was chosen over a null byte
because `(space)` is explicitly one of the selectable characters for this field, so it is
certainly a value the field can hold, whereas `00` is not in the listed repertoire at all.
The alternative — that the radio expects `00` filler, or that it truncates the field — is not
excluded by anything printed.

**The one capture that would settle it.** A **Stage R** capture on an IC-R8600: set one memory
channel's name from the front panel to a name of fewer than sixteen characters, then send
`FE FE 96 E0 1A 00 00 00 00 01 FD` and record what the receiver returns in the trailing cells of
㉖~㊶. That single capture shows the pad byte the radio itself emits, and nothing more — in
particular it does not show whether the radio would *accept* a differently padded write.

### A3 — the filter byte ⑫ for the five digital mode classes (frame byte 18)

`01`, in `set-record-name-with-space-50`, `-52`, `-54`, `-55-dcr` and `-56`.

**What was assumed:** that field ⑫ carries `01` when ⑪ names one of the digital modes.

**Why that value and not another.** The document is silent. The record layout on PDF page 12
defers ⑪,⑫ to `Receiving mode` on PDF page 10, and that table prints a long dash in the
`②Filter setting` column for every one of `16:P25`, `17:D-STAR`, `18:dPMR`, `19:NXDN-VN`,
`20:NXDN-N` and `21:DCR` — it gives no value at all for those modes, while ⑫ remains a drawn
byte of the record for every mode class. `01` was chosen because it is the lowest of the three
values the table does define (`01:FIL1`, `02:FIL2`, `03:FIL3`), so it is at least a value the
field is documented to be able to hold. A filler such as `00` is equally plausible and is not
excluded. Note this assumption does not arise in `set-record-name-with-space-48` (AM) or
`-55-fm` (FM), where the table prints `01:FIL1` and the byte is a documented choice.

**The one capture that would settle it.** A **Stage R** capture on an IC-R8600: set one memory
channel to a digital mode from the front panel, then send
`FE FE 96 E0 1A 00 00 00 00 01 FD` and read byte ⑫ of the reply. That single capture shows what
the receiver puts in ⑫ for the one digital mode chosen, and nothing beyond it — not the other
four digital mode classes, and not whether a different value would be accepted on a write.

### A4 — the data area of the read request (frame bytes 7–10 of `read-record`)

`00 00 00 01`.

**What was assumed:** that a `1A 00` read request carries fields ①~④ — the group number and the
channel number — and nothing after them.

**Why that value and not another.** The digit encoding of ①,② and ③,④ *is* printed, on PDF page
12, and the values `0000` and `0001` are within the printed ranges; what is not printed anywhere
is the shape of the read request. The command table on PDF page 5 gives `1A / 00* / See pp. 11 ~
14 / Send/read memory channel contents` and marks it with the asterisk that means `Send/read
data`, but pages 11–14 draw only the full record, never a request. The only request format the
guide prints for this command is the *clear* format on PDF page 15 (`①, ② : 0000 ~ 0101 group;
③, ④ : Memory channel number; ⑤ : "FF"; ⑥ ~ : None`), which this leg was told not to build and
which is a write, not a read. Four bytes was chosen because it is the clear format minus its
`FF` marker — that is, the smallest prefix that identifies a channel. A request carrying only
①~② , or one carrying ①~⑤ with some other marker, is not excluded by anything printed.

**The one capture that would settle it.** A **Stage R** capture on an IC-R8600: send
`FE FE 96 E0 1A 00 00 00 00 01 FD` and record whether the receiver answers with a memory record
or with the `FA` NG message shown on PDF page 3. That single capture settles whether this exact
four-byte request is accepted for this one channel, and nothing wider — it does not establish the
request shape for any other group, nor for any other model.

## Hardware status

UNVERIFIED. No IC-R8600 has ever been asked anything by this project. Every vector here is
derived from printed documentation alone.
