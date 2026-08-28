# IC-905 CI-V golden vectors — provenance

## Source

- Title, as printed on the cover (PDF page 1): `ICOM` / `CI-V REFERENCE GUIDE` / `ALL MODE TRANSCEIVER` / `IC-905` / `Icom Inc.` The cover prints no revision code.
- Revision code, as printed: `A7711-9EX-2`, at the foot of the left column of PDF page 31 (the back matter), directly above `© 2023–2024  Icom Inc.      May 2024`.
- File: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic905_civ_2.pdf`
- Page count: 31 PDF pages. Printed folios run one behind the PDF page number throughout the body (PDF page *n* carries folio *n*−1); the cover and the back matter carry no folio.

## Extent

Pages rendered and read, with the folio printed on each:

| PDF page | Printed folio | What it contributed |
|---|---|---|
| 1 | (none) | Cover: document title and model, for `## Source`. No byte. |
| 3 | 2 | `◇ About the data format` — the frame diagrams. Source of every `structural` byte: `FE FE`, `AC`, `E0`, `FD`. |
| 6 | 5 | `◇ Command table` — the rows `19*1 \| 00 \| \| Read the transceiver ID` and `1A* \| 00 \| See pp. 18 and 19. \| Send/read memory contents`. Source of `19 00`, and of the fact that the `19 00` request's Data cell is empty. |
| 9 | 8 | `◇ Command table`, `1A 05` `SET > Connectors` rows (Output Select … REF OUT). This is the set-mode/menu material the task named. It prints no CI-V address and no memory-record field: **it contributed no byte**. Read and discarded. |
| 16 | 15 | `◇ Command table` footnote legend: `*(Asterisk) Send/read data`, `*1 Read only data`, `*2 Send only data`, … Read to learn what `19*1` and `1A*` mean. No byte. |
| 17 | 16 | `• Operating frequency` (Command: 00, 03, 05, 1C 03) and `• Operating mode` (Command: 01, 04, 06). Source of the frequency BCD digit order, of the conditional 10-digit/12-digit width, and of `05` (FM) and `01` (FIL1). |
| 18 | 17 | `• Duplex Offset frequency setting` (Command: 0C, 0D) — source of the offset digit order; and `• Codes for CW message contents`, read only as corroboration for the assumed space code. |
| 19 | 18 | `• Memory content`, `Command: 1A 00` — the memory-record data-block diagram and its legend. The spine of this leg. |
| 20 | 19 | `• Codes for character entries` (`Command: 1A 00, 1A 05 …`) — the Letters and Numbers table and the Symbols table. Source of the name field's letter and digit codes, and of the fact that no space is printed for this field. |
| 21 | 20 | `• Keyer memory character entries` — read only as corroboration for the assumed space code (`Space \| 20 \| Word space`). No byte of any vector. |
| 24 | 23 | `• Repeater tone/tone squelch frequency settings`, `• DTCS code and polarity setting`, `• DV Digital code squelch setting`, `• DV TX call signs setting` and `Character's code of the call sign`. Source of fields (16)–(25) and of the call-sign space code `20`. |
| 25 | 24 | `• DV RX call sign data` / `• DV RX message` — read only to check whether the document prints a worked example frame. It does not. No byte. |
| 31 | (none) | Back matter: revision code. No byte. |

Where the transcribed material begins and ends:

- **PDF page 19 (folio 18).** The material begins at the bullet heading `• Memory content`, immediately below `◇ Command formats`, which is itself immediately below the grey section band `Remote control (CI-V) information`. It ends at the block `To clear the memory channel contents on 1A 00:` … `(5): "FF," (6) ~ : None` at the foot of the right column; below that, the page prints only the folio `18`. Nothing else appears on the page.
- **PDF page 20 (folio 19).** The character material begins at `• Codes for character entries` (immediately below `◇ Command formats`) and ends at the last row of the `- Character codes— Symbols` table (`~ | 7E | @ | 40`). Immediately to its right, in the second column, begins the unrelated `• Band stacking register`, which was not used.
- **PDF page 17 (folio 16).** `• Operating frequency` begins immediately below `◇ Command formats` and ends at the note `When the 5600 MHz or lower band is selected, the number of digits is 10 …`; `• Operating mode` follows immediately below it and ends at the note about the filter setting being skippable with command 01.
- **PDF page 3 (folio 2).** The frame diagrams begin at the heading `◇ About the data format` (immediately below `◇ Preparing`) and end with the four labelled frames; below them the page prints only the folio `2`.

Nothing was transcribed from any page not listed above.

## Method

1. **Locate — 300 dpi.** `pdftoppm -png -r 300 -f <n> -l <n> <pdf> r300/p` into a fresh directory `evidence/ic905-G/r300`, created for this leg. Whole-page renders were read as images to find the printed headings named in the task and the sections they cross-refer to.
2. **Read — 400 dpi.** Pages 3, 17, 18, 19, 20 and 24 were re-rendered with `-r 400` into `r400/` and every first-pass value was read from those.
3. **Crop and enlarge.** ImageMagick **was available** (`/opt/homebrew/bin/magick`) and was used throughout, e.g.
   - `magick r400/p-19.png -crop 2800x300+200+1020 +repage -resize 200% crops/p19_band1.png` (diagram band 1)
   - `magick r400/p-19.png -crop 2100x300+200+1280 +repage -resize 200% crops/p19_band2.png` (diagram band 2)
   - `magick r400/p-19.png -crop 700x450+180+2480 +repage -resize 250% crops/p19_f5.png` (the boxed figure for field (5))
   - `magick r400/p-17.png -crop 1450x850+200+1050 +repage -resize 200% crops/p17_freq.png` (frequency diagram with its twelve vertical labels)
   Every numbered band, boxed figure and legend line recorded here was enlarged until each numeral, rule and glyph stood clear of its neighbours.
4. **`pdftotext -layout`.** It **was run once**, on this same PDF, to `nav.txt`, and used only to find which PDF page a printed heading sits on (`Memory content`, `Codes for character entries`, `Operating frequency`, `Duplex Offset`, the footnote legend, the word `Example`). It was **navigational only** and was the source of no recorded value: every byte, index, width, label and enum below was read from a rendered page image.
5. **`tesseract`.** Available (`/opt/homebrew/bin/tesseract`) but **not used**. No OCR was run on any crop; every value was read by eye off the render.
6. **`pdfinfo`** was run once on this same PDF for the page count. It was the source of no recorded value.

**Second independent pass.** After the first pass was complete, every value was re-read from a different raster: pages 3, 6, 17, 18, 19, 20 and 24 were re-rendered at **600 dpi** into `r600/` and cropped with **different windows** (each diagram band split into two overlapping halves rather than taken whole, tables cropped individually, different enlargement factors of 120–200 %), e.g. `magick r600/p-19.png -crop 1500x400+330+1520 +repage -resize 200% crops2/b1a.png` and `-crop 1500x400+1780+1520` for the right half of the same band. Re-read on the second pass: the whole memory-content diagram (every box, every shading, every index numeral and its style), the legend entries for fields (1)–(5), (13)–(15), (16)–(25) and (29)–(68), the letters/numbers and symbols tables, the call-sign character table, the repeater-tone, DTCS and DV-code-squelch diagrams, the operating-mode table, the duplex-offset diagram, the two command-table rows and the frame diagram.

**Disagreements between the passes: none.** Every cell agreed. No third render was needed to settle anything.

## Hazards encountered

- **(a) Numeral styling varying within one diagram — NOT ENCOUNTERED.** Every index in the memory-content diagram on PDF p.19 and in its legend, from (1) to (68), is drawn one way: a plain outlined circle containing plain digits, one or two digits, no fill, no reversal, no brackets, no bold. Checked at 400 dpi and again at 600 dpi across both bands and both legend columns. The same single style is used in the diagrams on pp. 3, 17, 18 and 24.
- **(b) Vector groups with rotated labels — ENCOUNTERED.** The frequency diagram (p.17), the duplex-offset diagram (p.18) and the repeater-tone, DTCS and DV-code-squelch diagrams (p.24) all carry their nibble labels as 90°-rotated vertical text under arrows. Every one of those labels was read from the render by following its arrow to the cell half it lands on; the text layer's ordering was never consulted for any of them (`pdftotext` output was used only to find page numbers).
- **(c) Leader-line label order reversed — ENCOUNTERED.** On PDF p.24, in the `DTCS code and polarity setting` diagram, cell (1)'s two leaders cross the label block: the **left** half's leader runs down past the first label to the **lower** label `Transmit polarity: 0=Normal 1=Reverse`, while the **right** half's leader stops at the **upper** label `Receive polarity: 0=Normal 1=Reverse`. Read by following each leader by eye at 400 dpi and again at 600 dpi. (Both halves are 0 in these vectors, so the reversal changes no byte here; it is recorded because it would change one.)
- **(d) Printed index differing from measured position — ENCOUNTERED.** Within the printed p.19 diagram the two agree everywhere: indices (1)…(68) run continuously, with no repeat, gap or out-of-order entry, and each index's bracket spans exactly the cells its legend width states — (29)~(36), (37)~(44), (45)~(52) each span 8, and (53)~(68) spans 16. The divergence appears only in the derived 12-digit-frequency record (`set-record-name-with-space-69`): there the frequency occupies six bytes, so every field from the operating mode onward still carries its printed index (11)…(68) while sitting one byte later than the diagram draws it. Both are recorded — the printed index in `field_index`, the measured position in `first_byte`/`last_byte` — and are not reconciled. See STOP 1.

## STOP findings

1. **The width of the operating-frequency field in a memory record is printed two ways.**
   - PDF page 19 (folio 18), memory-content diagram, band 1: the bracket labelled `(6)~(10)` spans **five** byte boxes (a shaded box, a dashed elision, a shaded box), and the legend beneath reads `(6)~(10): Operating frequency setting` / `See "Operating frequency." (p. 16)`. Five bytes, and the next index printed is (11) for the operating mode.
   - PDF page 17 (folio 16), `• Operating frequency`, the note directly under the six-cell diagram: `When the 5600 MHz or lower band is selected, the number of digits is 10 ((1) ~ (5)). When the 10 GHz band is selected, the number of digits is 12 ((1) ~ (6)) from 100 GHz to 1 Hz.` Twelve digits is **six** bytes.
   - Why it stops: the memory-content diagram allots five boxes and consecutive indices to a field whose own definition, on the page it points at, is 10 **or** 12 digits; the p.19 page also prints memory call channels for the 10 GHz band (`00 10, 00 11: 10G C1, C2`), so a 10 GHz record is a record this diagram covers. A measured extent (five boxes) therefore disagrees with a documented width (six bytes) for one documented band, and every printed index after (10) is one byte adrift in that case.
   - Both readings are built, as the task requires: `set-record-name-with-space-68` follows the five-box width drawn on p.19, `set-record-name-with-space-69` follows the 12-digit width printed on p.17. The affected runs carry `STOP 1` in `notes`. Their `status` stays `manual_derived` because what conflicts is the field's **width**, not any printed byte value — no printed value was overridden, and nothing was interpolated, averaged or carried over.

No other STOP arose. Confidence for the rest: the index sequence (1)…(68) is continuous with no repeat or gap; the three 8-character call-sign blocks and the 16-character name block each measure exactly what their legends state; the field widths sum to 68 (2+2+1+5+2+1+1+1+3+3+3+1+3+8+8+8+16), which is the last printed index; every numeral and label recorded was legible at 400 dpi enlarged and was re-read identically at 600 dpi from different crops.

## Observed disagreements

Printed exactly as seen; not resolved, and none of them stopped the work.

- PDF p.19, right legend column, two consecutive entries: `(16)~(18): Repeater tone frequency setting` and `(19)~(21): Repeater tone frequency setting` — the same label for two different three-byte blocks, followed by a single cross-reference `See "Repeater tone/tone squelch frequency setting." (p. 23)` whose own section title names **two** settings (`• Repeater tone/tone squelch frequency settings`, `Command: 1B 00, 1B 01`). The document nowhere says which of (16)~(18) and (19)~(21) is the tone and which the tone squelch. Both blocks were encoded from the one printed 1B 00/1B 01 layout.
- PDF p.19, `(37)~(44): R1 (Access repeater) call sign setting` is followed by `(8 characters, fixed.)` — with a full stop inside the bracket, where the neighbouring (29)~(36) and (45)~(52) entries print `(8 characters, fixed)`.
- The document carries **three** character-code tables plus a fourth for call signs, and they do not agree on their contents. `Codes for character entries` (PDF p.20), the table this record's name field points to, prints letters, digits, lower case and 32 symbols but **no space**. `Codes for CW message contents` (PDF p.18) prints `Space | 20`. `Keyer memory character entries` (PDF p.21) prints `Space | 20 | Word space`. `Character's code of the call sign` (PDF p.24) prints `(Space) | 20`. This is the basis of assumption A2 below.
- PDF p.17, `(1)Operating mode` column: the printed codes run `00:LSB 01:USB 02:AM 03:CW 04:RTTY 05:FM 07:CW-R 08:RTTY-R 17:DV 22:DD* 23:ATV*` — `06` is absent, with no note explaining the gap.
- **No worked example frame is printed anywhere in this document**, so no `manual-example-<n>` vector was built. What the document does print, and what was checked and rejected: PDF p.20, `Example: When reading the frequency displayed in the center of the display in the UHF band, use code "02 02."` — a two-byte code, not a frame; PDF p.21, `Example: to send BT, enter ^4254` — a character sequence, not a frame; PDF p.25, `Example: When a Gateway call is received` — a picture of two stations, no bytes. The only fully concrete byte sequences printed are the `OK message to controller (PC)` (`FE FE E0 AC FB FD`) and `NG message to controller (PC)` (`FE FE E0 AC FA FD`) diagrams on PDF p.3; these are the **format definitions** of the two response messages, labelled as such, alongside the `Controller (PC) to IC-905` diagram whose command, sub command and data cells are placeholders (`Cn`, `Sc`, `Data area`). They are not worked examples, and the task's own list already excludes the frames they belong to, so they were not transcribed as a vector.

## Attestation

Every value recorded here was read from this single PDF's rendered page images. `pdftotext -layout` was run on this same PDF for navigation only and was the source of no recorded value. Nothing else was consulted: no other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

## The vectors

Conventions used in `ic905-golden-assumptions.csv`:

- `first_byte`/`last_byte` are 1-based within the **whole frame**, so the data block starts at byte 7 in every `1A 00` vector.
- A run that carries only half a byte gives `first_nibble`/`last_nibble` (1 = the half printed first, i.e. leftmost in these left-to-right diagrams) and puts that **single hex digit** in `bytes_hex`, so that the run's extent is never mistaken for a whole byte.
- `field_index` gives the record field index as printed in the p.19 diagram, or an inclusive range of printed indices (`6-10`, `53-68`); `-` marks framing, addressing and command bytes; an **empty** cell marks a byte for which the diagram prints no index at all (the sixth frequency byte of the 69-byte record).

### `read-record` — 11 bytes

Reads one memory record: the `1A 00` command addressed to one memory group and channel.

`FE FE AC E0 1A 00 00 00 00 01 FD`

| Bytes | Value | Where it comes from |
|---|---|---|
| 1–2 | `FE FE` | `structural`. Preamble, printed in the `Controller (PC) to IC-905` frame on PDF p.3. |
| 3 | `AC` | `structural`. The transceiver's default address, printed on p.3. |
| 4 | `E0` | `structural`. The controller's default address, printed on p.3. |
| 5–6 | `1A 00` | `manual_documented`. Printed verbatim as `Command: 1A 00` on p.19 (and as the row `1A* | 00` on p.6). |
| 7–8 | `00 00` | `inherited_assumed` — **A1**. Memory group 00. |
| 9–10 | `00 01` | `inherited_assumed` — **A1**. Memory channel 01. |
| 11 | `FD` | `structural`. End of message, printed on p.3. |

The four addressing bytes are marked assumed because the document prints **no** read form for `1A 00`: it prints the full send layout (the data-block diagram) and one truncated form, the clear form, which this leg was told not to build. Their values are inside the ranges printed for fields (1),(2) and (3),(4), but their presence — and the fact that the request ends after field (4) — is mine, not the document's.

### `set-record-name-with-space-68` — record 68 bytes, frame 75 bytes

A complete `1A 00` write of one memory record whose name field contains a space in the middle of the name. `68` is the record's derived total length in bytes, i.e. the data block, which is also the last index the p.19 diagram prints; the frame that carries it is 75 bytes (6 header + 68 + `FD`).

`FE FE AC E0 1A 00 00 00 00 01 00 00 00 50 44 01 05 01 00 00 00 00 08 85 00 08 85 00 00 23 00 00 00 00 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 48 49 47 48 4C 41 4E 44 20 42 41 53 45 39 30 35 FD`

Field-by-field, keyed to the CSV rows (frame positions in brackets):

| Field | Bytes | Value | Working |
|---|---|---|---|
| — | 1–6 | `FE FE AC E0 1A 00` | as above |
| (1),(2) | 7–8 | `00 00` | `derived`: memory channel group 00, from `00 00 ~ 00 99: Memory channel group`. |
| (3),(4) | 9–10 | `00 01` | `derived`: channel 01, from `00 00 ~ 00 99: 00 ~ 99`. |
| (5) | 11 n1 | `0` | `documented`: printed 0, labelled `Fixed`. |
| (5) | 11 n2 | `0` | `derived`: `0=OFF*` chosen from `0=OFF*, 1=★1, 2=★2, 3=★3`. |
| (6)–(10) | 12–16 | `00 00 50 44 01` | `derived`, **STOP 1**. 144.500000 MHz (inside the printed 144 band, `144.000000 ~ 148.000000`). Digit order from p.17: byte1 `(10 Hz 0)(1 Hz 0)`, byte2 `(1 kHz 0)(100 Hz 0)`, byte3 `(100 kHz 5)(10 kHz 0)`, byte4 `(10 MHz 4)(1 MHz 4)`, byte5 `(1 GHz 0)(100 MHz 1)`. Five bytes = the width the p.19 diagram draws. |
| (11) | 17 | `05` | `derived`: `05:FM` from the operating-mode table. |
| (12) | 18 | `01` | `derived`: `01:FIL1` from the filter column. |
| (13) | 19 | `00` | `derived`: `00: Data mode OFF`. |
| (14) | 20 | `00` | `derived`: `0=Duplex OFF` in the first half, `0=OFF` in the second. |
| (15) | 21 n1 | `0` | `derived`: `0=Digital squelch function OFF`. |
| (15) | 21 n2 | `0` | `documented`: printed 0, labelled `Fixed`. |
| (16) | 22 | `00` | `documented`: both halves printed 0 (`Fixed digit: 0*`). |
| (17),(18) | 23–24 | `08 85` | `derived`: 088.5 Hz — `(100 Hz 0)(10 Hz 8)`, `(1 Hz 8)(0.1 Hz 5)`, each inside its printed range. |
| (19) | 25 | `00` | `documented`: as (16). |
| (20),(21) | 26–27 | `08 85` | `derived`: as (17),(18). |
| (22) | 28 | `00` | `derived`: transmit polarity 0=Normal, receive polarity 0=Normal (leaders followed by eye, see hazard (c)). |
| (23) | 29 n1 | `0` | `documented`: printed `0 (fixed)`. |
| (23) | 29 n2 | `0` | `derived`: DTCS first digit 0, from `First digit: 0 ~ 7`. |
| (24) | 30 | `23` | `derived`: DTCS second digit 2, third digit 3 → code 023. |
| (25) | 31 | `00` | `derived`: DV digital code squelch, first digit 0, second digit 0. |
| (26)–(28) | 32–34 | `00 00 00` | `derived`: all six offset digits 0, consistent with `Duplex OFF` in (14). |
| (29)–(36) | 35–42 | `20` ×8 | `derived`: UR call sign, eight spaces; `(Space) | 20` in the call-sign character table the field points to. |
| (37)–(44) | 43–50 | `20` ×8 | `derived`: R1 call sign, eight spaces. |
| (45)–(52) | 51–58 | `20` ×8 | `derived`: R2 call sign, eight spaces. |
| (53)–(60) | 59–66 | `48 49 47 48 4C 41 4E 44` | `derived`: `HIGHLAND`, from `A ~ Z = 41 ~ 5A`: A=41 so H=41+7=48, I=49, G=47, H=48, L=4C, A=41, N=4E, D=44. |
| (61) | 67 | `20` | `inherited_assumed` — **A2**. The space, ninth of the sixteen name characters. |
| (62)–(68) | 68–74 | `42 41 53 45 39 30 35` | `derived`: `BASE905` — B=42, A=41, S=53, E=45 from `A ~ Z = 41 ~ 5A`; 9=39, 0=30, 5=35 from `0 ~ 9 = 30 ~ 39`. |
| — | 75 | `FD` | `structural`. |

The name is `HIGHLAND BASE905`: sixteen characters exactly, filling the fixed 16-character field with no padding, and carrying exactly one space, in the middle. Every name byte but that one space is derived from a printed range, so the field costs the register exactly one assumption.

### `set-record-name-with-space-69` — record 69 bytes, frame 76 bytes

The same record written for a 10 GHz-band channel, where PDF p.17 prints the frequency as 12 digits (six bytes). `69` is that derived record total; the frame is 76 bytes.

`FE FE AC E0 1A 00 00 00 00 01 00 00 00 00 50 02 01 05 01 00 00 00 00 08 85 00 08 85 00 00 23 00 00 00 00 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 48 49 47 48 4C 41 4E 44 20 42 41 53 45 39 30 35 FD`

It differs from the 68-byte record in exactly one place, and everything after it shifts one byte later:

| Field | Bytes | Value | Working |
|---|---|---|---|
| (6)–(10) | 12–16 | `00 00 00 50 02` | `derived`, **STOP 1**. First five bytes of 10250.000000 MHz (inside the printed 10G band, `10000.000000 ~ 10500.000000`): `(10 Hz 0)(1 Hz 0)`, `(1 kHz 0)(100 Hz 0)`, `(100 kHz 0)(10 kHz 0)`, `(10 MHz 5)(1 MHz 0)`, `(1 GHz 0)(100 MHz 2)`. |
| (no index printed) | 17 n1 | `0` | `documented`, **STOP 1**: printed `0` and labelled `100 GHz digit: 0 (Fixed)` on p.17. The p.19 diagram prints no index for this byte. |
| (no index printed) | 17 n2 | `1` | `derived`, **STOP 1**: `10 GHz digit: 0, 1` → 1, which is what makes this a 10 GHz frequency and so what requires the sixth byte at all. |

From byte 18 (`05`, field (11)) to byte 75 (the last name byte) every run carries the same printed index and the same value as in the 68-byte record, one byte later. `FD` closes the frame at byte 76.

### `read-transceiver-id` — 7 bytes

`FE FE AC E0 19 00 FD`

| Bytes | Value | Where it comes from |
|---|---|---|
| 1–2 | `FE FE` | `structural`, p.3. |
| 3 | `AC` | `structural`, p.3. |
| 4 | `E0` | `structural`, p.3. |
| 5–6 | `19 00` | `manual_documented`. PDF p.6 command table: `19*1 | 00 | | Read the transceiver ID`. The Data cell of that row is **empty**, so the request carries no data bytes; the `*1` legend on PDF p.16 reads `Read only data`. |
| 7 | `FD` | `structural`, p.3. |

This vector contains no assumed byte.

## Assumption register

### A1 — the four addressing bytes of `read-record` (frame bytes 7–10, `00 00 00 01`)

- **What was assumed.** That a `1A 00` read request consists of `FE FE AC E0 1A 00`, then the memory group number (fields (1),(2)) and the memory channel number (fields (3),(4)), then `FD` — four data bytes and no more — and that the record so addressed is group 00, channel 01.
- **Why this and not another.** The document is silent on the composition of a `1A 00` read. It states that a read exists — PDF p.6 prints `1A*` against `Send/read memory contents`, and the footnote legend on PDF p.16 prints `*(Asterisk) Send/read data` — but it prints only two `1A 00` layouts: the full send layout (the 68-field data-block diagram) and one truncated layout, the clear form on PDF p.19 (`(1), (2): Memory channel group (00 00 ~ 00 99)` … `(3), (4): Memory channel (00 00 ~ 00 99)` … `(5): "FF," (6) ~ : None`), which this leg was told not to build. That clear form is the document's only demonstration that a `1A 00` frame may stop early, and it stops after the same four addressing fields, which is why the read was cut at field (4) rather than at field (5) or at the command. The two values are the lowest the printed ranges allow for a real record (`00 00 ~ 00 99` for the group, `00 00 ~ 00 99: 00 ~ 99` for the channel), chosen so no byte depends on an unprinted maximum. Readings this document does not exclude: a read that carries no addressing bytes at all, or one that also carries field (5).
- **The one capture that would settle it.** A **Stage R** capture on a real ic905: send `FE FE AC E0 1A 00 00 00 00 01 FD` on the CI-V port and record what comes back — a memory-content reply for group 00 channel 01, or an `FA` (NG) frame. That single capture observes only whether this ten-byte request is accepted and what it returns for that one channel; it settles nothing about the write layout and nothing about the name character codes.

### A2 — the space in the memory name (frame byte 67 of `set-record-name-with-space-68`, byte 68 of `set-record-name-with-space-69`, value `20`)

- **What was assumed.** That the code for a space character inside the memory name field, fields (53)~(68), is `20`.
- **Why this and not another.** For this field the document is silent. Field (53)~(68) points at `See "Codes for character entries." (p. 19)`, and that section (PDF page 20) prints two tables — `Letters and Numbers` (`A ~ Z | 41 ~ 5A`, `0 ~ 9 | 30 ~ 39`, `a ~ z | 61 ~ 7A`) and `Symbols` (32 rows, `!` through `@`) — neither of which contains a space. `20` was chosen because it is the only value this document ever prints for a space anywhere: `Character's code of the call sign` on PDF p.24 prints `(Space) | 20`, `Codes for CW message contents` on PDF p.18 prints `Space | 20`, and `Keyer memory character entries` on PDF p.21 prints `Space | 20 | Word space`. None of those three tables governs the memory name field, so the byte stays assumed rather than derived; no competing value for a space is printed anywhere in the document. Readings this document does not exclude: that the name field uses some other code for a space, or that it will not accept one.
- **The one capture that would settle it.** A **Stage R** capture on a real ic905: with a memory name that already contains a space stored in group 00, channel 01 from the radio's own front panel, send the `1A 00` read for that channel and record the byte returned in the ninth position of the sixteen-byte name field. That single capture observes only what this radio returns for that one stored name; it does not establish what the radio will accept on a write, and it says nothing about assumption A1's request layout beyond the fact that the request used to make the capture was answered.

No other byte in any of the four vectors is assumed. Every remaining byte is either `structural` (printed in the frame diagram on PDF p.3), `manual_documented` (the page prints that exact byte or that exact fixed value) or `manual_derived` (computed above from a printed encoding, with the working shown).

## Hardware status

UNVERIFIED. No ic905 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.
