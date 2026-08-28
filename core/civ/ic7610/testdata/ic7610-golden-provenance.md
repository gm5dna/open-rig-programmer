# IC-7610 golden wire frames — provenance

## Source

Document title as printed on the cover (PDF page 1): the black band carries **CI-V REFERENCE GUIDE**; below it, on the white field, **HF/50MHz TRANSCEIVER** over **IC-7610**; at the foot, **Icom Inc.**

Revision code as printed: **A7380-7EX-4**, printed at the foot of the back cover (PDF page 17), left-hand column, on the line immediately above `© 2017–2025 Icom Inc.   Sep. 2025`. No revision code is printed on the front cover.

File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7610_civ_ENG_4.pdf`

Page count: **17 PDF pages** (cover, folios 1–15, back cover).

## Extent

Rendered at 300 dpi: PDF pages 1–17 (the whole document), to locate the sections by eye.
Re-rendered at 400 dpi: PDF pages 3, 10, 11, 12, 14.
Re-rendered at 500 dpi for the second pass: PDF pages 3, 10, 11, 12, 14.

Pages actually read for values, with the printed folio beside the PDF page number:

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | (none, cover) | document title, model |
| 2 | 1 | table of contents — navigation only, no value taken |
| 3 | 2 | `◇ About the data format`: the `Controller to IC-7610` frame strip — `FE FE 98 E0 Cn Sc Data area FD`, and the labels naming 98 as the transceiver's default address and E0 as the controller's |
| 5 | 4 | `◇ Command table`: the row `19 / 00 / (blank Data) / Read the transceiver ID`, and the row `1A* / 00 / see p. 11 / Send/read memory contents` |
| 10 | 9 | `◇ Command table` right column: footnote *4 and the worked frame `Example: When using 4800 bps` |
| 11 | 10 | `◇ Command formats`: `• Operating frequency` (five-cell strip, ten digit labels) and `• Operating mode` (two-cell strip and its mode/filter table); also `• Codes for CW message contents`, row `Space \| 20` |
| 12 | 11 | `◇ Command formats`: `• Memory content` (`Command: 1A 00`) — the 18-cell data-block strip and all its field blocks; and `• Codes for character entries` (`Command: 1A 00, 1A 05 …`) with its two ASCII tables and the `* Usable characters` footnote |
| 14 | 13 | `◇ Command formats`: `• Repeater tone/tone squelch frequency settings` (`Command: 1B 00, 1B 01`); also `• Memory keyer character entries`, row `space \| 20 \| Word space` |

Pages 4, 6, 7, 8, 9, 13, 15, 16 were rendered and read at 300 dpi only, to confirm that no further worked example frame is printed anywhere in the document. Page 17 was read for the revision code.

Where the transcribed material begins and ends:

- **PDF page 12 (folio 11).** The running head `Remote control` sits at the top, then the section heading `◇ Command formats`. The first block under it is `• Memory content` / `Command: 1A 00`, immediately followed by the 18-cell data-block strip — this is where the record material begins. The record material runs down the left column (blocks `①, ② Memory channel numbers`, `③ Select memory setting`, `④ ~ ⑧ Operating frequency setting`, `⑨, ⑩ Operating mode setting`) and continues in the right column (`⑪ Data mode and tone type settings`, `⑫ ~ ⑭ Repeater tone frequency setting`, `⑮ ~ ⑰ Tone squelch frequency setting`, `⑱ ~ ㉗ Memory name settings`). It ends with the right-column paragraph `To clear the memory channel contents on 1A 00: ①,②: Memory channel (00 01~00 99) / ③: "FF" / ④: None`. The next thing printed after the record material, in the left column, is the heading `• Codes for character entries`; that block ends with the two ASCII tables and the page ends at folio `11`.
- **PDF page 3 (folio 2).** The four frame strips sit under `◇ About the data format`, which is preceded by `◇ Preparing`. Nothing is printed after them but the folio `2`.
- **PDF page 10 (folio 9).** The worked example is in the right column. Immediately above it is the bullet list ending `• 4800 bps: 7 "FE"s`; immediately below it is `*5 Read only data`.
- **PDF page 11 (folio 10).** `• Operating frequency` is the first block under `◇ Command formats`; `• Operating mode` follows it; `• Codes for CW message contents` follows that.
- **PDF page 14 (folio 13).** `• Repeater tone/tone squelch frequency settings` is in the right column, printed immediately below `• RIT frequency settings` and immediately above `• Main or Sub band's frequency settings`.

## Method

1. **Locate.** `pdftoppm -png -r 300 -f 1 -l 17 <pdf> <out>/r300/p` — all 17 pages at 300 dpi, into a freshly created directory (`rm -rf` first, so nothing pre-existing could be mistaken for evidence). Every page was read as an image to find the headings named in the task and to sweep for any further worked example frame.
2. **Read.** `pdftoppm -png -r 400` on PDF pages 3, 10, 11, 12, 14 (files under `r400/`). Every first-pass value was read from those.
3. **Crop and enlarge.** ImageMagick **was** available (`/opt/homebrew/bin/magick`, `/opt/homebrew/bin/convert`) and was used throughout. First-pass crops (under `crops/`), e.g.:
   - `magick r400/p-12.png -crop 2250x300+560+820 +repage -resize 200% crops/p12-memdiagram.png` (whole data-block strip), then split into halves at 300%: `-crop 1150x300+560+820` and `-crop 1150x300+1660+820`.
   - `magick r400/p-12.png -crop 1150x560+330+1470 +repage -resize 250% crops/p12-field3.png` (field ③).
   - `magick r400/p-12.png -crop 1350x450+1750+1130 +repage -resize 250% crops/p12-field11.png` (field ⑪).
   - `magick r400/p-12.png -crop 1350x270+290+2780 +repage -resize 300% crops/p12-chartable-letters.png`, and two 250% crops for the symbols table.
   - `magick r400/p-03.png -crop 1350x230+470+3200 +repage -resize 250% crops/p03-frame-ctrl2rig.png`.
   - `magick r400/p-10.png -crop 1350x330+1720+1770 +repage -resize 250% crops/p10-example.png`.
   - `magick r400/p-11.png -crop 1400x850+280+880 +repage -resize 200% crops/p11-opfreq.png`; `-crop 1400x300+280+1930 -resize 250%` for the operating-mode strip.
   - `magick r400/p-14.png -crop 1350x1000+1800+1500 +repage -resize 200% crops/p14-tone.png`.
   Enlargement was pushed until every numeral, rule and cell divider stood clear of its neighbours.
4. **`pdftotext -layout` was never run.** Navigation was done entirely by reading the 300 dpi page renders. No text layer of this or any PDF was extracted at any point.
5. **`tesseract` was available** (`/opt/homebrew/bin/tesseract`) but **was not used**. Every numeral, label and code recorded here was read by eye off an enlarged raster.
6. **Second independent pass — done.** After the first pass was complete, all five value-bearing pages were re-rendered at a *different* dpi — `pdftoppm -png -r 500` into `r500/` — and re-cropped with *different windows and different enlargements* into `crops2/`, deliberately not reusing the first pass's crop geometry:
   - the data-block strip was cut into **three** overlapping windows at 300% (`-crop 900x280+740+1050`, `-crop 900x280+1560+1050`, `-crop 1020x280+2380+1050`) instead of the first pass's two halves at 300%;
   - the p.3 frame strip at 200% over a wider window (`-crop 1700x200+590+4050`);
   - the p.10 example at 200% (`-crop 1700x400+2140+2240`);
   - the p.11 frequency strip split left/right at 250% (`-crop 900x1000+350+1120` and `+1150+1120`) rather than as one 200% block;
   - the p.14 tone strip at 300% (`-crop 1300x220+2320+2140`);
   - p.12 fields ③ and ⑪ at 350% and 300% (`-crop 800x220+430+1920`, `-crop 1000x420+2210+1540`);
   - the Letters and Numbers table at 400% (`-crop 800x230+380+3480`).
   **Cells where the two passes disagreed: none.** Both passes read 18 drawn cells in the data-block strip (16 byte cells plus two dashed ellipsis cells), the same bracket spans, the same ten frequency digit labels in the same printed order, the same reversed leader assignment in field ⑪, the same `0 | 0` fixed pair in the tone strip's first cell, and the same example bytes on p. 10. No third render was needed.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** In the p.12 memory-content data-block strip every index (`①, ②`, `③`, `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭`, `⑮ ~ ⑰`, `⑱ ~ ㉗`) is drawn one way: a plain numeral inside a thin outlined circle, unfilled, unbracketed, not bold. The same outlined-circle style is used for the indices in the p.3 frame strips, the p.11 frequency and mode strips and the p.14 tone strip. (Separately, a *filled/reversed* circled `29` glyph is used as a footnote marker in the command tables on PDF pages 4–9, e.g. `❷❾ Command 29 supported`; it is a marker, not an index, and it appears in none of the diagrams transcribed here. It is recorded, not normalised.)
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** The p.11 `Operating frequency` strip, the p.14 tone strip and the p.3 frame strips all carry their per-nibble labels rotated 90°, set below the cells and joined to them by long vertical leaders. Every one of them was read positionally from the render by following its leader up to the cell it lands on. The text layer was never extracted, so nothing here depends on extraction order.
- **(c) Leader-line label order may be reversed — ENCOUNTERED.** In the p.12 block `⑪ Data mode and tone type settings` the two labels sit stacked to the right of the box and their printed order runs *opposite* to the nibbles they point at: the **upper** label `0: OFF, 1: TONE, 2: TSQL` is joined by a short leader to the **right** nibble, and the **lower** label `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3` is joined by a long leader that drops below the first and runs left to the **left** nibble. Both passes traced the leaders by eye and agree. The record therefore carries data mode in the left nibble and tone type in the right.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED.** Both were recorded for every field, and for the repeated block in particular. Printed index → measured position (data-block byte, and frame byte in `set-record-name-with-space`): ① → 1/7; ② → 2/8; ③ → 3/9; ④–⑧ → 4–8/10–14 (drawn as two cells plus one dashed ellipsis cell); ⑨ → 9/15; ⑩ → 10/16; ⑪ → 11/17; ⑫ → 12/18; ⑬ → 13/19; ⑭ → 14/20; ⑮ → 15/21; ⑯ → 16/22; ⑰ → 17/23; ⑱–㉗ → 18–27/24–33 (drawn as two cells plus one dashed ellipsis cell). The block ⑮–⑰ repeats the encoding of ⑫–⑭; its printed indices and its measured positions were taken separately and they agree. No index is repeated, skipped or out of order, and the bracket spans measured on the render match the index ranges printed above them.

## STOP findings

1. **PDF page 14 (folio 13), block `• Repeater tone/tone squelch frequency settings`, `Command: 1B 00, 1B 01`: cell `①*` printed `0 | 0`, with the footnote `*Not necessary when setting a frequency.` printed directly beneath the strip — set against PDF page 12 (folio 11), the memory-content data-block strip, which prints three separate numbered cells `⑫ ~ ⑭` for `Repeater tone frequency setting` and three more `⑮ ~ ⑰` for `Tone squelch frequency setting`, and refers the reader to that very section.**
   - What is printed, statement A (p. 14): the tone-frequency encoding is three bytes, of which the first is `0 | 0`, marked `*` = "Not necessary when setting a frequency."
   - What is printed, statement B (p. 12): inside the `1A 00` record the tone frequency occupies exactly three cells, indexed `⑫`, `⑬`, `⑭`, in an index sequence that runs `①` to `㉗` without a gap; the tone squelch likewise occupies `⑮`, `⑯`, `⑰`.
   - Why it stops: a `1A 00` record write *is* a frame that sets a tone frequency, so statement A implies the leading `00` may be dropped and the record may be 26 or 25 bytes; statement B allots the byte an index of its own and would leave a hole in the printed index sequence if it were dropped. The two cannot both be taken at face value.
   - Built from the one judged clearer: **statement B**, the memory-content strip itself, because it is the diagram that actually governs `1A 00`, it is the diagram whose printed heading matches the material being transcribed, and its index run `①`–`㉗` is complete and self-consistent (2+1+5+2+1+3+3+10 = 27). The asterisk on p. 14 is attached to command `1B`'s own data area.
   - Transcribed as seen: `00` at frame byte 18 (field 12) and `00` at frame byte 21 (field 15), both `manual_documented`, both carrying `STOP 1` in `notes`. No vector was shortened and no derived total other than 34 bytes was written.

2. **PDF page 12 (folio 11), block `③ Select memory setting`: the box is printed `0 | X` with the left nibble's arrow dropping to the word `Fixed` — set against the right-column paragraph on the same page, `To clear the memory channel contents on 1A 00: ①,②: Memory channel (00 01~00 99) / ③: "FF" / ④: None`.**
   - What is printed, statement A: the high nibble of `③` is `0`, labelled `Fixed`.
   - What is printed, statement B: to clear a channel, `③` is `"FF"` — whose high nibble is `F`, not `0`.
   - Why it stops: `Fixed` admits no other value for that nibble, yet the same page prints a value for that byte whose high nibble is `F`.
   - Built from the one judged clearer: **statement A**, the `③` diagram, because this leg writes no clear/erase frame and the diagram is the printed definition of the field for an ordinary write. Transcribed as seen: nibble 1 of frame byte 9 = `0`, `manual_documented`, `STOP 2` in `notes`. Statement B is not built into any vector; no clear frame appears in the golden file.

## Observed disagreements

- The cross-reference printed on PDF page 12 reads `See "• Repeater tone/tone squelch settings."` The heading actually printed on PDF page 14 is `• Repeater tone/tone squelch frequency settings` — the word `frequency` is present in the heading and absent from the cross-reference. Recorded as printed; no other section in the document has a heading close to either form, so the target is not in doubt.
- The p.12 block `• Codes for character entries` (`Command: 1A 00, 1A 05 …`) prints two ASCII tables — `- Character codes— Letters and Numbers` and `- Character codes— Symbols` — and **neither contains a row for the space character**, although the footnote in the same block reads, verbatim across its three printed lines joined with single spaces: * Usable characters: A to Z, a to z, 0 to 9, (space), ! " # $ % & ' ( ) * + , - . / : ; < = > ? @ [ \ ] ^ \_ \` { | } ~ (that eleventh-from-last glyph is a grave accent) — and the asterisk is attached to `Memory name*` in the adjacent table. The space code used in `set-record-name-with-space` therefore comes from elsewhere in this same document (see `## The vectors`). Recorded, not resolved.
- The p.12 note `ⓘSet 0 for P1 and P2.` under field `③` speaks only of the programmed scan edges; nothing is printed about what `③` should be for an ordinary numbered memory channel beyond the `0=OFF / 1=★1 / 2=★2 / 3=★3` enumeration. Recorded.
- PDF page 12's memory-name block prints `Up to 10 characters.` while the strip above it allots exactly ten cells, `⑱ ~ ㉗`. The document does not print what a controller sends when a name is shorter than ten characters, nor whether a shorter name shortens the frame. This leg wrote a ten-character name so that every printed cell carries a printed-encoding character, and treated the field's printed width as the ten cells drawn. Recorded, not resolved.
- The `Data` cell of the `19 / 00` row on PDF page 5 is blank, whereas most neighbouring rows carry either an explicit value range or a `see p. N` reference. Taken at face value: no data bytes.

## Attestation

Nothing beyond this single PDF's rendered page images was consulted. No other file, manual, transcription, source file, generated artefact or web resource was opened, and no directory was listed.

## The vectors

Frame skeleton, from PDF page 3 (folio 2), strip `Controller to IC-7610`, read left to right: `FE FE | 98 | E0 | Cn | Sc | Data area | FD`, with the rotated leaders naming them `Preamble code (Fixed)` (index ①, two cells), `Transceiver's default address` (②), `Controller's default address` (③), `Command number` (④), `Sub command number` (⑤), `BCD code data for frequency or memory number entry` (⑥), `End of message code (Fixed)` (⑦). The same two addresses are confirmed by the worked example on PDF page 10, whose column headings read `7610's address` over `9 8` and `Controller's address` over `E 0`.

### `read-record` — 9 bytes

`FE FE 98 E0 1A 00 00 01 FD`

Reads one memory record over command `1A 00`. Nine bytes: six of frame and command, two of channel selector, one terminator.

| frame byte(s) | hex | what it is | CSV row |
|---|---|---|---|
| 1–2 | `FE FE` | preamble, p.3 | `read-record,1,-,2,-,FE FE,-,structural` |
| 3 | `98` | transceiver's default address, p.3 | `read-record,3,…,98,-,structural` |
| 4 | `E0` | controller's default address, p.3 | `read-record,4,…,E0,-,structural` |
| 5–6 | `1A 00` | command and sub command, printed as `Command: 1A 00` on p.12 and as row `1A* / 00 / see p. 11 / Send/read memory contents` on p.5 | `read-record,5,-,6,-,1A 00,-,manual_documented` |
| 7–8 | `00 01` | memory channel 01 — **assumed to be present at all**; see the assumption register | `read-record,7,-,8,-,00 01,1 \| 2,inherited_assumed` |
| 9 | `FD` | end of message, p.3 | `read-record,9,…,FD,-,structural` |

### `set-record-name-with-space` — 34 bytes

`FE FE 98 E0 1A 00 00 01 00 00 00 25 14 00 01 01 01 00 08 85 00 10 00 48 4F 4D 45 20 51 54 48 30 31 FD`

A complete `1A 00` record write for memory channel 01, whose name is `HOME QTH01` — ten characters with a space at position five, in the middle of the name.

**Why 34 and why only one such vector.** The data block measured on the p.12 strip is 27 bytes: `①②` = 2, `③` = 1, `④~⑧` = 5, `⑨⑩` = 2, `⑪` = 1, `⑫~⑭` = 3, `⑮~⑰` = 3, `⑱~㉗` = 10; 2+1+5+2+1+3+3+10 = 27, which matches the last printed index `㉗` exactly, and the strip's 16 drawn byte cells plus two dashed ellipsis cells account for it (5 drawn as 2 + `…`, 10 drawn as 2 + `…`). Frame total = 6 (preamble, two addresses, command, sub command) + 27 + 1 (`FD`) = **34**. No field of the record is printed at more than one width *in the memory-content diagram itself*: every one of the 27 positions carries its own index. The one conditional width printed anywhere that touches these fields is the `*Not necessary when setting a frequency` asterisk in the section cross-referenced for `⑫~⑭` and `⑮~⑰`; that conflict is recorded as **STOP 1**, built from the memory-content strip, and therefore yields no additional derived total. One vector, 34 bytes.

Byte-by-byte, keyed to the CSV:

| frame byte(s) | hex | field | derivation |
|---|---|---|---|
| 1–2 | `FE FE` | – | preamble, p.3 `structural` |
| 3 | `98` | – | transceiver address, p.3 `structural` |
| 4 | `E0` | – | controller address, p.3 `structural` |
| 5–6 | `1A 00` | – | `Command: 1A 00`, p.12, `manual_documented` |
| 7–8 | `00 01` | ① ② | p.12 prints `00 01 ~ 00 99: Memory channel 01 ~ 99`. Channel **01** chosen → the range's first member → `00 01`. `manual_derived` |
| 9 nibble 1 | `0` | ③ | printed `0`, leader labelled `Fixed`. `manual_documented`, **STOP 2** |
| 9 nibble 2 | `0` | ③ | printed enumeration `0=OFF / 1=★1 / 2=★2 / 3=★3`; **OFF** chosen (no select-memory star). `manual_derived` |
| 10–13 | `00 00 25 14` | ④ ⑤ ⑥ ⑦ | frequency **14.250000 MHz** chosen. Digits: 1 Hz 0, 10 Hz 0, 100 Hz 0, 1 kHz 0, 10 kHz 5, 100 kHz 2, 1 MHz 4, 10 MHz 1, 100 MHz 0, 1 GHz 0. The p.11 strip's ten rotated labels, read left to right, are 10 Hz, 1 Hz, 1 kHz, 100 Hz, 100 kHz, 10 kHz, 10 MHz, 1 MHz, 1 GHz, 100 MHz — so byte ④ = (10 Hz)(1 Hz) = `00`; byte ⑤ = (1 kHz)(100 Hz) = `00`; byte ⑥ = (100 kHz)(10 kHz) = `25`; byte ⑦ = (10 MHz)(1 MHz) = `14`. The `10 MHz digit: 0–6` range is respected (value 1). `manual_derived` |
| 14 | `00` | ⑧ | both nibbles printed `0`, labelled `1 GHz digit: 0 (Fixed)` and `100 MHz digit: 0 (Fixed)`. `manual_documented` |
| 15 | `01` | ⑨ | p.11 `①Receiving mode` column, row `01: USB`. USB chosen. `manual_derived` |
| 16 | `01` | ⑩ | p.11 `②Filter setting` column, row `01: FIL1`. FIL1 chosen. `manual_derived` |
| 17 | `01` | ⑪ | left nibble = data mode, `0: OFF` chosen; right nibble = tone type, `1: TONE` chosen. Nibble roles taken from the leaders, whose order is reversed relative to the labels (hazard c). `manual_derived` |
| 18 | `00` | ⑫ | p.14 cell `①*` printed `0 \| 0`. `manual_documented`, **STOP 1** |
| 19 | `08` | ⑬ | repeater tone **88.5 Hz** chosen: 100 Hz digit 0, 10 Hz digit 8 → `08`. 100 Hz digit range `0–2` respected. `manual_derived` |
| 20 | `85` | ⑭ | same tone: 1 Hz digit 8, 0.1 Hz digit 5 → `85`. `manual_derived` |
| 21 | `00` | ⑮ | p.14 cell `①*` printed `0 \| 0`. `manual_documented`, **STOP 1** |
| 22 | `10` | ⑯ | tone squelch **100.0 Hz** chosen (deliberately different from the repeater tone, so a field swap cannot pass): 100 Hz digit 1, 10 Hz digit 0 → `10`. `manual_derived` |
| 23 | `00` | ⑰ | same: 1 Hz digit 0, 0.1 Hz digit 0 → `00`. `manual_derived` |
| 24–27 | `48 4F 4D 45` | ⑱ ⑲ ⑳ ㉑ | `HOME`. p.12 prints `A–Z \| 41–5A`; the range spans 26 letters (`5A − 41 = 0x19 = 25`), so `A = 41` and letter *n* = `41 + (n−1)`. `H` = 41+7 = **48**; `O` = 41+14 = **4F**; `M` = 41+12 = **4D**; `E` = 41+4 = **45**. `manual_derived` |
| 28 | `20` | ㉒ | the **space**, in the middle of the name. The p.12 tables for `1A 00` print no space row, but the p.12 footnote lists `(space)` among the usable Memory name characters, and both p.12 tables are headed `ASCII code`; this document prints the ASCII code of a space twice — p.11 `Space \| 20` and p.14 `space \| 20 \| Word space`, both under an `ASCII code` heading. Hence **20**. `manual_derived` |
| 29–31 | `51 54 48` | ㉓ ㉔ ㉕ | `QTH`. `Q` = 41+16 = **51**; `T` = 41+19 = **54**; `H` = **48**. `manual_derived` |
| 32–33 | `30 31` | ㉖ ㉗ | `01`. p.12 prints `0–9 \| 30–39`, so `'0'` = **30** and `'1'` = **31** (check: `'9'` = 39, the printed upper bound). `manual_derived` |
| 34 | `FD` | – | end of message, p.3 `structural` |

### `read-transceiver-id` — 7 bytes

`FE FE 98 E0 19 00 FD`

The transceiver-identification read. Seven bytes; there is no data area.

| frame byte(s) | hex | derivation |
|---|---|---|
| 1–2 | `FE FE` | preamble, p.3 `structural` |
| 3 | `98` | transceiver address, p.3 `structural` |
| 4 | `E0` | controller address, p.3 `structural` |
| 5–6 | `19 00` | p.5 command table row `Cmd. 19 / Sub cmd. 00 / Data (blank) / Read the transceiver ID`. `manual_documented` |
| 7 | `FD` | end of message, p.3 `structural` |

The `Data` cell of that row is blank, so nothing stands between the sub command and `FD`. No byte of this vector is assumed.

### `manual-example-14` — 14 bytes

`FE FE FE FE FE FE FE FE FE 98 E0 18 01 FD`

The one worked example frame the document prints, on PDF page 10 (folio 9), headed `Example: When using 4800 bps`, illustrating footnote `*4` ("When sending the power ON command (18 01), you need to repeatedly send 'FE' before the standard format"). Transcribed byte for byte as printed: the strip is drawn as eight cells — a taller leading cell printed `F | E` in bold with `×7` set beneath it, then `Preamble` `F E F E`, `7610's address` `9 8`, `Controller's address` `E 0`, `Command` `1 8`, `Sub command` `0 1`, `Post amble` `F D`. The `×7` is the multiplier printed on the leading cell, so the leading cell denotes seven `FE` bytes — matching the same page's bullet `• 4800 bps: 7 "FE"s`. 7 + 7 = **14** bytes; hence the name.

| frame byte(s) | hex | derivation |
|---|---|---|
| 1–7 | `FE` ×7 | the bold leading cell and its `×7`, p.10 `manual_documented` |
| 8–9 | `FE FE` | cells under `Preamble`, p.10 `structural` |
| 10 | `98` | cells under `7610's address`, p.10 `structural` |
| 11 | `E0` | cells under `Controller's address`, p.10 `structural` |
| 12 | `18` | cells under `Command`, p.10 `manual_documented` |
| 13 | `01` | cells under `Sub command`, p.10 `manual_documented` |
| 14 | `FD` | cells under `Post amble`, p.10 `structural` |

No other worked example frame is printed anywhere in the document. Pages 4, 6, 7, 8, 9, 13, 15 and 16 were read and carry only command tables and per-command data-block strips; the two other strips on p.3 (`OK message to controller` `FE FE E0 98 FB FD` and `NG message to controller` `FE FE E0 98 FA FD`) are parts of the generic `About the data format` definition, drawn with the same `Preamble code (Fixed)` / `OK code (Fixed)` leaders as the request strip, not worked examples of a command; and the p.11 line `the code "07 03" is used` is a data value, not a frame. No clear/erase frame and no transceive frame of any kind was built, and none is present in the golden file.

## Assumption register

There is exactly one `inherited_assumed` run in this leg.

**`read-record`, frame bytes 7–8, `00 01` (record fields ① and ②).**

- **What was assumed.** That a `1A 00` *read* request carries the two-byte memory-channel selector in its data area at all. The document prints the `1A 00` data block only as a complete 27-byte record (the p.12 strip), and prints one truncated form of it — the clear form, `①,② / ③ = "FF" / ④: None`. It prints no read request for `1A 00`, and it nowhere states that a read carries the channel number, or that it carries nothing.
- **Why this value and not another.** Given the assumption that the two bytes are present, their value is fixed by an encoding the document does print: p.12, `00 01 ~ 00 99: Memory channel 01 ~ 99`. Channel **01** was chosen because it is the first member of that printed range, so `00 01` is the byte pair the page itself prints as the range's lower bound; any other channel would have required arithmetic on a value the page does not print. `01 00` and `01 01` were rejected because the same block reserves them for `Programmed scan edge P1` and `P2` — a scan edge is not "one memory record". Sending no data bytes at all was rejected because the radio would then have nothing to say which of the 99 channels to return; the document states nothing either way, so this remains the assumption, not a deduction.
- **The document is silent** on the form of a `1A 00` read request. Nothing here rests on any other model, any other manual, or any recollection of the protocol.
- **The one capture that would settle it — Stage R.** *Stage R capture: connect a real IC-7610 over CI-V, send exactly `FE FE 98 E0 1A 00 00 01 FD`, and record every byte the radio sends back.* That single capture can settle exactly one thing: whether this nine-byte frame, with these two channel bytes in it, is accepted as a read of memory channel 01 — the radio either returns a `1A 00` record frame (accepted) or the NG code `FA` (not accepted). It cannot settle what a `1A 00` read with *no* data bytes would do, it cannot settle the behaviour of any other channel number, and nothing is claimed about any radio other than the IC-7610 the capture is taken from.

Every other byte in every vector is `structural`, `manual_documented` or `manual_derived`, and each such run names in the CSV the PDF page and the printed anchor that justifies it.

## Hardware status

UNVERIFIED. No ic7610 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.
