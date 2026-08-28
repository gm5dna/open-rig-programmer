# IC-7300MK2 golden vectors — provenance

## Source

Document title as printed on the cover (PDF page 1): **CI-V REFERENCE GUIDE**, set in the
dark band immediately below the ICOM logo. Below it the cover prints
`HF/50 MHz TRANSCEIVER` and, in large type, `IC-7300MK2`; `Icom Inc.` is printed at the
foot. The cover prints no revision code.

Revision code as printed: **A7841-8EX**, printed at the foot of PDF page 27 (the back
cover), in the right-hand block, on the line immediately above `© 2025  Icom Inc.
Oct. 2025`. The left-hand block of the same footer prints `Icom Inc. / 1-1-32 Kamiminami,
Hirano-ku, Osaka 547-0003, Japan`.

File: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7300mk2_civ_ref_0.pdf`

Page count: 27 PDF pages.

## Extent

All 27 PDF pages were rendered at 300 dpi to locate material. Pages **1, 2, 3, 4, 6, 9,
16, 17, 18, 19, 23 and 27** were read. Folio numbering in this document is simple: the
cover (PDF 1) and back cover (PDF 27) carry no folio, and every other page carries a
centred folio equal to its PDF page number — verified by eye on the footers of PDF pages
2 (`2`), 3 (`3`), 4 (`4`), 6 (`6`), 9 (`9`), 16 (`16`), 17 (`17`), 18 (`18`), 19 (`19`)
and 23 (`23`).

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | none | Cover: document title, model, publisher. Nothing transcribed. |
| 2 | 2 | Contents. Navigation only; recorded under Observed disagreements. |
| 3 | 3 | `◇ About the data format` — the CI-V frame figure. Source of every `structural` byte: `FE FE`, `B6`, `E0`, `FD`. |
| 4 | 4 | The NOTE under `◇ Command table` explaining the asterisk and how a read is addressed. Justifies the shape of `read-record`. |
| 6 | 6 | `◇ Command table` rows `19 / 00` and `1A* / 00`. Source of the command and subcommand bytes. |
| 9 | 9 | `◇ Command table`, `1A* 05 SET > Connectors > CI-V` set-mode block. Read and found to print **no** CI-V address for the transceiver itself (its `00 90` item is the `USB/LAN→REMOTE Transceive Address`, `00 00 ~ 02 23` = `00h ~ DFh`), so it neither confirms nor contradicts the `B6` on page 3. Nothing transcribed. |
| 16 | 16 | `• Operating frequency`, `• Operating mode` (encodings for record fields ④~⑧ and ⑨⑩, reached by the page-17 pointers), and `• Turning the transceiver ON`, which prints the only worked example frame in the document. |
| 17 | 17 | `• Memory channel content / Command: 1A 00` — the memory-record data-block diagram and all its legends. |
| 18 | 18 | `• Codes for character entries / Command: 1A 00, …` — the letter and digit code ranges, and the `1A 00 Memory name (⑱ ~ ㉝)` usable-character note. |
| 19 | 19 | Checked for further worked example frames. Its two `Example:` lines are `use code "07 02."` and `"5E 42 54"`, neither of which is a frame. Its own character table (for command `1A 02`) was **not** used as a source. Nothing transcribed. |
| 23 | 23 | `• Repeater tone/tone squelch frequency settings / Command: 1B 00, 1B 01` — the encoding for record fields ⑫~⑭ and ⑮~⑰, reached by the page-17 pointer. |
| 27 | none | Back cover: revision code. |

Where the transcribed material begins and ends:

- **PDF page 17.** The memory-record material begins with the bullet heading
  `• Memory channel content`, immediately below `◇ Command formats`, and the line
  `Command: 1A 00`; the data-block diagram is drawn directly beneath that line. It ends
  with the grey `NOTE:` box at the foot of the right-hand column, whose last bullet ends
  `…into ❹ ~ ⓱ to match your transceiver.` Nothing else is printed on the page below that
  box except the folio `17`. Immediately before this material, on PDF page 16, the last
  printed item is the `Example:` frame under `• Turning the transceiver ON`. Immediately
  after it, on PDF page 18, the next printed item is `• Codes for character entries`.
- **PDF page 18.** The character material begins at `• Codes for character entries` with
  `Command: 1A 00, / 1A 05    01 07, 01 14, 01 28, 01 35` and runs to the end of the
  right-hand `Cmd. / Sub cmd. / Setting item` table, whose last row is
  `01 35  Time Set > Date/Time > NTP Server Address`.
- **PDF page 16.** The example frame begins at `• Turning the transceiver ON /
  Command: 18 01` in the right-hand column and ends with the bracket caption `13 "FE"s`
  beneath the frame; nothing is printed below it in that column.
- **PDF page 23.** The tone material begins at `• Repeater tone/tone squelch frequency
  settings / Command: 1B 00, 1B 01` and ends with its footnote
  `* Not necessary when setting a frequency.`; the next bullet printed below is
  `• RIT frequency settings`.

## Method

**Tools.** ImageMagick was available (`/opt/homebrew/bin/magick`, ImageMagick 7) and was
used for every crop, rotation and enlargement. `tesseract` was available
(`/opt/homebrew/bin/tesseract`) but was **not run**: every value recorded here was read by
eye from a render. `pdftotext -layout` **was run once**, on this same PDF, writing
`nav.txt` into the evidence directory, and was used **only** to find which PDF page a
heading sits on. It was the source of no byte position, nibble label, numeral, field
index, width, label or enum value. `pdfinfo` was used once for the page count and title.
The only file listings run were `ls` over the render directories this leg created inside
the evidence directory, to confirm that the renders had been written, and a `stat` of the
PDF itself by its given absolute path. No other directory was listed and no other file was
opened.

**Pass 1 (400 dpi).**

1. Locate: `pdftoppm -png -r 300 -f 1 -l 27 <pdf> <out>/r300/p`, then read the whole-page
   renders to find the headings the task names.
2. Read: `pdftoppm -png -r 400 -f <p> -l <p> <pdf> <out>/r400/p` for pages 2, 3, 4, 6, 16,
   17, 18, 23, 27.
3. Crop and enlarge, for example:
   - `magick r400/p-17.png -crop 2400x230+430+1020 +repage -resize 250% crop/p17-band-full.png`
   - `magick r400/p-17.png -crop 900x230+430+1020  +repage -resize 400% crop/p17-s1.png`
     (and `+1280`, `+2000` for the other two thirds of the same band)
   - `magick r400/p-03.png -crop 1300x180+490+3230 +repage -resize 300% crop/p3-frame-top.png`
   - `magick r400/p-16.png -crop 800x1120+150+1180 +repage -rotate 90 -resize 170% crop/p16-freqlab2.png`
     (rotation needed because the frequency-digit labels are set vertically)
   - `magick r400/p-16.png -crop 700x220+1745+3990 +repage -resize 500% crop/p16-ex2.png`

**Pass 2 (600 dpi, independent).** With pass 1 complete, every value was read again from a
different raster, without consulting the pass-1 numbers. The second raster differed in
three ways: a different resolution (600 dpi rather than 400 dpi), different crop windows
(for instance the page-17 data block was split into two windows at origins `+640` and
`+2400` instead of three windows at `+430`, `+1280`, `+2000`, so that every field boundary
that fell at a window edge in pass 1 fell in mid-window in pass 2), and different
enlargement factors (160–250% rather than 200–500%). Representative commands:

- `magick r600/p-17.png -crop 1900x360+640+1520  +repage -resize 180% crop2/p17-a.png`
- `magick r600/p-17.png -crop 1900x360+2400+1520 +repage -resize 180% crop2/p17-b.png`
- `magick r600/p-17.png -crop 1900x800+340+2380  +repage -resize 160% crop2/p17-split.png`
- `magick r600/p-16.png -crop 1900x180+2560+5990 +repage -resize 220% crop2/p16-ex.png`
- `magick r600/p-18.png -crop 2050x560+2590+2080 +repage -resize 200% crop2/p18-usable.png`
- `magick r600/p-23.png -crop 900x600+220+1900   +repage -rotate 90 -resize 200% crop2/p23-lab.png`

**Second-pass record.** Both passes were done. Values re-read in pass 2: the page-3 frame
cells `FE FE B6 E0 Cn Sc [Data area] FD` and their index numerals; the page-6 rows `19 00`
and `1A* 00 See p. 17.`; the page-16 frequency digit labels and their ranges, the operating
mode and filter enumerations, and every cell of the `18 01` example frame including the
bracket caption; the page-17 data-block cell count, cell shading, bracket span and index
style for all nine brackets, the `①, ② Memory channel number` lines, the SPLIT/SELECT box
and table, the DATA/TONE box and table, and the NOTE box; the page-18 `A ~ Z = 41 ~ 5A`
and `0 ~ 9 = 30 ~ 39` rows and the `1A 00 Memory name` usable-character note; the page-23
three-cell box and its six digit labels. **The two passes disagreed in no cell.** No third
render was needed.

**A note on the nibble columns.** Four runs in the CSV carry only half of a byte: frame
byte 9 (field ③, SPLIT then SELECT) and frame byte 17 (field ⑪, DATA then TONE) of
`set-record-name-with-space`. Each of these bytes holds two *different settings* drawn
with their own arrow and their own table column, so each half is recorded as its own run
with `first_nibble`/`last_nibble` naming the half in printed order (nibble 1 = the half
printed leftmost). For such a run `bytes_hex` gives the whole containing byte, because a
single nibble has no two-character hex form; the nibble columns say which half of it the
run is about. Every other byte in the deliverable is recorded whole, with `-` in both
nibble columns. BCD bytes whose two nibbles are two digits of one value (the frequency and
the tone frequencies) are treated as whole-byte runs, with the digit assignment spelled
out in `notes`.

## The vectors

Common framing, from the page-3 figure `Controller (PC) to IC-7300MK2`:
`FE FE` (index ①, "Preamble code (Fixed)"), `B6` (index ②, "Transceiver's default
address"), `E0` (index ③, "Controller's (PC's) default address"), `Cn` (index ④, command
number), `Sc` (index ⑤, subcommand number), `Data area` (index ⑥), `FD` (index ⑦, "End of
message code (Fixed)").

The memory record itself, measured on the page-17 data-block diagram, is **47 bytes**:

| printed index | style | bytes | measured position in the data block |
|---|---|---|---|
| ①, ② | outlined circled | 2 | 1–2 |
| ③ | outlined circled | 1 | 3 |
| ④ ~ ⑧ | outlined circled | 5 | 4–8 |
| ⑨, ⑩ | outlined circled | 2 | 9–10 |
| ⑪ | outlined circled | 1 | 11 |
| ⑫ ~ ⑭ | outlined circled | 3 | 12–14 |
| ⑮ ~ ⑰ | outlined circled | 3 | 15–17 |
| ❹ ~ ⓱ | filled / reversed circled | 14 | 18–31 |
| ⑱ ~ ㉝ | outlined circled | 16 | 32–47 |

Total 2+1+5+2+1+3+3+14+16 = **47**. The diagram draws some of these runs abbreviated: the
④~⑧ run is drawn as a solid cell, a dashed ellipsis cell and a solid cell; the ❹~⓱ run is
drawn as one dashed region containing a dotted line; the ⑱~㉝ run is drawn as a solid
cell, a dashed ellipsis cell and a solid cell. The ⑨,⑩, ⑪, ⑫~⑭ and ⑮~⑰ runs are drawn
cell-for-cell.

**No field's printed width in this record is conditional.** The memory-name field is
described as "up to 16 characters", which is a maximum on the content, not a second
documented width: the diagram enumerates sixteen index positions ⑱ through ㉝ and prints no
alternative. Consequently there is one derived total length and one write vector, named
`set-record-name-with-space` with no numeric suffix. (The one printed hint of
conditionality anywhere near these fields, the asterisk on page 23, is recorded as STOP 2
below; it belongs to command `1B`'s own frame, and adopting it would delete indices ⑫ and
⑮ from a positional, index-numbered record.)

### `read-record` — 9 bytes

Reads one memory record. The page-4 NOTE says that a command with an asterisk is read by
sending it "without any subcommand or data", but adds that "for some commands, you must
specify the channel number … in the subcommand or data section to read the corresponding
settings or content" — so the channel number is carried and nothing else.

| frame byte(s) | hex | what |
|---|---|---|
| 1–2 | `FE FE` | preamble (`structural`) |
| 3 | `B6` | transceiver address (`structural`) |
| 4 | `E0` | controller address (`structural`) |
| 5 | `1A` | command number (`manual_documented`, page 6) |
| 6 | `00` | subcommand number (`manual_documented`, page 6) |
| 7–8 | `00 01` | fields ①② — memory channel 01 (`manual_derived`, page 17) |
| 9 | `FD` | end of message (`structural`) |

Working for bytes 7–8: page 17 prints `00 01 ~ 00 99: Memory channel 01 ~ 99`. Channel 01
was chosen, which is the printed low end of the range, so the pair is `00 01`. It is
recorded as derived rather than documented because the choice of channel is mine.

### `set-record-name-with-space` — 54 bytes

Writes one complete memory record whose name field contains a space in the middle of the
name. 6 framing/command bytes + 47 record bytes + 1 end-of-message byte = 54.

| frame byte(s) | hex | index | what |
|---|---|---|---|
| 1–2 | `FE FE` | – | preamble |
| 3 | `B6` | – | transceiver address |
| 4 | `E0` | – | controller address |
| 5 | `1A` | – | command number |
| 6 | `00` | – | subcommand number |
| 7–8 | `00 01` | ①② | memory channel 01 |
| 9 | `00` | ③ | nibble 1 SPLIT = 0 (OFF); nibble 2 SELECT = 0 (OFF) |
| 10–13 | `00 00 10 14` | ④⑤⑥⑦ | operating frequency, low four BCD bytes |
| 14 | `00` | ⑧ | 1 GHz digit `0 (Fixed)`, 100 MHz digit `0 (Fixed)` |
| 15 | `01` | ⑨ | operating mode `01=USB` |
| 16 | `01` | ⑩ | filter `01=FIL1` |
| 17 | `00` | ⑪ | nibble 1 DATA = 0 (OFF); nibble 2 TONE = 0 (OFF) |
| 18 | `00` | ⑫ | repeater tone, `Fixed digit: 0*` twice |
| 19–20 | `08 85` | ⑬⑭ | repeater tone 88.5 Hz |
| 21 | `00` | ⑮ | tone squelch, `Fixed digit: 0*` twice |
| 22–23 | `10 00` | ⑯⑰ | tone squelch 100.0 Hz |
| 24–37 | `00 00 10 14 00 01 01 00 00 08 85 00 10 00` | ❹…⓱ | copy of frame bytes 10–23 |
| 38–44 | `54 45 53 54 49 4E 47` | ⑱…㉔ | `T E S T I N G` |
| 45 | `20` | ㉕ | the space — the one assumed byte |
| 46–49 | `4E 41 4D 45` | ㉖…㉙ | `N A M E` |
| 50–53 | `30 31 32 33` | ㉚…㉝ | `0 1 2 3` |
| 54 | `FD` | – | end of message |

Working, keyed to the CSV rows:

- **Bytes 10–14, frequency.** Page 16 prints ten arrows over the five-cell box, left to
  right: `10 Hz digit: 0~9`, `1 Hz digit: 0~9`, `1 kHz digit: 0~9`, `100 Hz digit: 0~9`,
  `100 kHz digit: 0~9`, `10 kHz digit: 0~9`, `10 MHz digit: 0~7`, `1 MHz digit: 0~9`,
  `1 GHz digit: 0 (Fixed)`, `100 MHz digit: 0 (Fixed)`. 14.100000 MHz was chosen, giving
  digits 1 GHz 0, 100 MHz 0, 10 MHz 1, 1 MHz 4, 100 kHz 1, 10 kHz 0, 1 kHz 0, 100 Hz 0,
  10 Hz 0, 1 Hz 0. Packed into the printed order: `00` (10 Hz 0, 1 Hz 0), `00` (1 kHz 0,
  100 Hz 0), `10` (100 kHz 1, 10 kHz 0), `14` (10 MHz 1, 1 MHz 4), `00` (both fixed
  zeros). The `10 MHz digit` value 1 is inside the printed `0~7` bound. Byte 14 is
  documented rather than derived because both its nibbles are printed as literal `0`s in
  the box and labelled `(Fixed)`.
- **Bytes 19–20 and 22–23, tone frequencies.** Page 23 prints, over its three-cell box:
  `Fixed digit: 0*`, `Fixed digit: 0*`, `100 Hz digit: 0~2`, `10 Hz digit: 0~9`,
  `1 Hz digit: 0~9`, `0.1 Hz digit: 0~9`. 88.5 Hz gives 100 Hz 0, 10 Hz 8, 1 Hz 8,
  0.1 Hz 5 → `08 85`; 100.0 Hz gives 100 Hz 1, 10 Hz 0, 1 Hz 0, 0.1 Hz 0 → `10 00`. Two
  different tones were chosen for the two fields so that the mapping is visible in the
  vector. The document prints no list of permitted tone frequencies for this radio — only
  these digit ranges — so the choice is bounded by the ranges alone.
- **Bytes 24–37, the mirrored block.** Page 17's NOTE box prints "The same data as ④ ~ ⑰
  are stored in ❹ ~ ⓱." and "Even if the Split function is OFF, we recommend that you set
  the same data as ④ ~ ⑰ into ❹ ~ ⓱ to match your transceiver." The run is therefore a
  byte-for-byte copy of frame bytes 10–23. See STOP 1.
- **Bytes 38–53, the name.** `TESTING NAME0123` was chosen: exactly sixteen characters, so
  the sixteen-cell field is filled exactly and no question of padding or termination
  arises, with a single space at character 8, in the middle of the name. Letter codes come
  from the printed range `A ~ Z = 41 ~ 5A`, so A=41 and T=41+19=54, E=45, S=53, I=49,
  N=4E, G=47, M=4D. Digit codes come from the printed range `0 ~ 9 = 30 ~ 39`, so 0=30,
  1=31, 2=32, 3=33. Byte 45, the space, is the single assumed byte: see the assumption
  register.

### `read-transceiver-id` — 7 bytes

| frame byte(s) | hex | what |
|---|---|---|
| 1–2 | `FE FE` | preamble (`structural`) |
| 3 | `B6` | transceiver address (`structural`) |
| 4 | `E0` | controller address (`structural`) |
| 5 | `19` | command number (`manual_documented`, page 6) |
| 6 | `00` | subcommand number (`manual_documented`, page 6) |
| 7 | `FD` | end of message (`structural`) |

The page-6 row reads `19 | 00 | (Data cell empty) | Reads the transceiver ID.` The empty
Data cell is why no data area is sent. `19` carries no asterisk in the Cmd. column, so the
page-4 read/write NOTE does not apply to it.

### `manual-example-1` — 20 bytes

The one worked example frame printed anywhere in this document, on PDF page 16 under
`• Turning the transceiver ON / Command: 18 01`, captioned "Example: When the baud rate is
9600 bps, enter 13 "FE"s." It is neither a clear/erase frame nor a transceive frame.

As drawn, the frame is: a solid `FE` cell, a dashed ellipsis cell, a solid `FE` cell — the
three of them spanned by a bracket captioned `13 "FE"s` — then two further solid `FE`
cells, then `B6`, `E0`, `18`, `01`, `FD`. Fifteen `FE` bytes in all: the thirteen inside
the bracket plus the ordinary two-byte preamble outside it.

| frame byte(s) | hex | status | why |
|---|---|---|---|
| 1 | `FE` | `manual_documented` | first solid cell |
| 2–12 | `FE` × 11 | `manual_derived` | 13 printed in the caption minus the 2 drawn solid |
| 13 | `FE` | `manual_documented` | second solid cell, where the bracket ends |
| 14–15 | `FE FE` | `structural` | the ordinary preamble, drawn outside the bracket |
| 16 | `B6` | `structural` | transceiver address |
| 17 | `E0` | `structural` | controller address |
| 18–19 | `18 01` | `manual_documented` | printed in the example; page 6 prints `18 / 01 / Turns the transceiver ON.` |
| 20 | `FD` | `structural` | end of message |

## Assumption register

Exactly one run in this deliverable is `inherited_assumed`.

**Frame byte 45 of `set-record-name-with-space`, value `20`, record field ㉕.**

*What was assumed.* That the byte written into a memory-name cell to produce a space
character is `20`.

*Why the document does not settle it.* Page 18 is the section the memory-record diagram
points to for name characters: `• Codes for character entries / Command: 1A 00, 1A 05
01 07, 01 14, 01 28, 01 35`. It prints two tables. The first, `- Character codes— Letters
and Numbers`, prints only `A ~ Z = 41 ~ 5A`, `a ~ z = 61 ~ 7A` and `0 ~ 9 = 30 ~ 39`. The
second, `- Character codes— Symbols`, prints thirty-two rows — `! 21`, `# 23`, `$ 24`,
`% 25`, `& 26`, `\ 5C`, `? 3F`, `" 22`, `' 27`, `` ` `` `60`, `^ 5E`, `+ 2B`, `− 2D`,
`* 2A`, `/ 2F`, `. 2E`, `, 2C`, `: 3A`, `; 3B`, `= 3D`, `< 3C`, `> 3E`, `( 28`, `) 29`,
`[ 5B`, `] 5D`, `{ 7B`, `} 7D`, `| 7C`, `_ 5F`, `~ 7E`, `@ 40` — and **no row for a
space**. Yet the box to the right of those tables, `1A | 00 | Memory name (⑱ ~ ㉝) (up to
16 characters)`, prints `ⓘ Usable characters: A to Z, a to z, 0 to 9, (space), ! " # $ %
& ' ( ) * +, - . / : ; < = > ? @ [ \ ] ^ _ ' { | } ~` — the space is explicitly usable in
a memory name, and its code is printed nowhere in the section that gives the codes. On the
code for a space in a memory name, the document is silent.

*Why `20` and not another value.* Two other tables in this same document print
`Space | 20`: the `Codes for CW message contents` table on page 16, which is headed
`Command: 17`, and the `Character codes` table on page 19, which is headed
`Command: 1A 02` (Keyer memory character entries) and glosses it `Word space`. Both belong
to other commands, and neither is the table the memory-record diagram sends the reader to,
so carrying `20` across to command `1A 00` is my inference and not the document's
statement. `20` is nonetheless the only value with any printed support: the page-18 column
that would carry the answer is headed `ASCII code`, and the two ranges it does print,
`A ~ Z = 41 ~ 5A` and `0 ~ 9 = 30 ~ 39`, are consistent with the same code set in which
those two other tables place a space at `20`. No other candidate value appears anywhere in
this document in connection with a space.

*The one capture that would settle it.* **Stage W capture W-SPACE-1**, on an IC-7300MK2:
send the 54-byte `set-record-name-with-space` frame exactly as written in the `.golden`
file, with `20` at byte 45, and record the transceiver's reply. Page 3 prints the two
replies this capture can produce — `FE FE E0 B6 FB FD` ("PASS") and `FE FE E0 B6 FA FD`
("FAIL") — so the capture observes precisely one thing: whether a memory-name write
carrying `20` in a name cell is accepted or rejected by the radio. It does not observe
what glyph the radio then displays, and nothing here should be read as claiming it does.

## Hardware status

UNVERIFIED. No ic7300mk2 has ever been asked anything by this project. Every vector here is derived from printed documentation alone.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram.** **ENCOUNTERED.** The page-17
  data-block diagram draws its indices in two styles. Eight of its nine brackets use
  outlined circled numerals — `①, ②`, `③`, `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭`, `⑮ ~ ⑰`,
  `⑱ ~ ㉝` — and the ninth uses filled/reversed circled numerals, white figures on solid
  black discs: `❹ ~ ⓱`. The same two styles recur in the page-17 NOTE box, which prints
  `④ ~ ⑰` outlined and `❹ ~ ⓱` filled in the same sentence. Both styles are recorded in
  the CSV as drawn, using outlined and filled circled characters respectively; they have
  not been normalised, and no meaning has been inferred for the styling beyond what the
  NOTE box itself prints.
- **(b) Diagrams may be vector groups with rotated labels.** **ENCOUNTERED.** The page-16
  operating-frequency diagram and the page-23 tone diagram both set every digit label
  vertically (rotated 90°), and the page-3 frame figure sets all fourteen of its leader
  labels vertically. Every one of these was read from the render, after rotating the crop
  with `-rotate 90`, and position was taken from the picture; the `pdftotext -layout`
  output, which does scramble these labels, was used for navigation only.
- **(c) Leader-line label order may be reversed.** **NOT ENCOUNTERED.** Every leader in
  the diagrams read here runs straight, without crossing: the page-16 frequency labels sit
  below the box with ten parallel vertical arrows, and following each arrow by eye from
  label to cell gives the same left-to-right order as the labels themselves (10 Hz, 1 Hz,
  1 kHz, 100 Hz, 100 kHz, 10 kHz, 10 MHz, 1 MHz, 1 GHz, 100 MHz — an order that looks
  scrambled but is simply the printed one, byte by byte); the page-17 SPLIT/SELECT and
  DATA/TONE legends each use two short parallel arrows in label order; the page-23 tone
  labels likewise. The only crossing leaders in the document are on page 3, where the
  `Transceiver's default address` and `Controller's (PC's) default address` leaders cross
  between the top and bottom frames because the two addresses swap places between the
  outbound and inbound frames — followed by eye, they confirm `B6` then `E0` outbound and
  `E0` then `B6` inbound, which is what the cells themselves print.
- **(d) A printed index may differ from a field's measured position.** **ENCOUNTERED.**
  The page-17 record contains a block that repeats another block: after ⑮~⑰ the diagram
  prints a bracket labelled `❹ ~ ⓱`, that is, printed indices 4 to 17 for a second time.
  Both values are recorded and neither has been reconciled to the other: the *printed
  index* of that block is `❹ ~ ⓱` (filled/reversed circled 4 to 17), and its *measured
  position* is data-block bytes 18 to 31, that is, frame bytes 24 to 37, arrived at by
  measuring the drawn cells of the seven brackets that precede it (2+1+5+2+1+3+3 = 17
  bytes). Separately, the *drawn extent* of that block on the page is about three
  cell-widths — measured on the 600 dpi render, the dashed region spans roughly 480 px
  against a cell width of roughly 162 px — while the block it stands for is fourteen bytes
  long; the same mismatch occurs for ④~⑧ (three cells drawn, five bytes) and ⑱~㉝ (three
  cells drawn, sixteen bytes). In all three cases the diagram declares the abbreviation
  itself, with a dashed ellipsis cell, so the drawn extent is not a width claim and no
  arithmetic is broken by it.

## STOP findings

1. **Index sequence discontinuity: indices 4 to 17 are printed twice, in two styles.**
   PDF page 17. Visual anchor: the data-block diagram under `• Memory channel content /
   Command: 1A 00`; the bracket beginning immediately to the right of the ⑮~⑰ cells and
   ending immediately to the left of the ⑱~㉝ cells. What is printed: `❹ ~ ⓱`, in
   filled/reversed circled numerals, over a dashed region — while the same numbers 4 to 17
   have already been printed earlier in the same diagram, in outlined circled numerals, as
   `④ ~ ⑧`, `⑨, ⑩`, `⑪`, `⑫ ~ ⑭`, `⑮ ~ ⑰`. Why it stops: the diagram's index sequence
   runs 1, 2, 3, 4…17, then 4…17 again, then 18…33 — a repeat of an already-used index
   range, printed a second time in a different numeral style. Transcribed as seen: the CSV
   row for frame bytes 24–37 of `set-record-name-with-space` carries the field index
   `❹|❺|❻|❼|❽|❾|❿|⓫|⓬|⓭|⓮|⓯|⓰|⓱` exactly as drawn, with `STOP 1` in `notes`, and its
   measured position (data-block bytes 18–31) is recorded in that row's notes without
   being reconciled to the printed index. The page-17 NOTE box does print a gloss — "The
   same data as ④ ~ ⑰ are stored in ❹ ~ ⓱." — and the vector was built on it; the
   discontinuity is recorded regardless, as the rules require.

2. **A conditional first cell printed for the tone fields on page 23, against three fixed
   indexed cells for the same fields on page 17.** PDF pages 23 and 17. Visual anchors:
   on page 23, the diagram under `• Repeater tone/tone squelch frequency settings /
   Command: 1B 00, 1B 01`, whose first cell is indexed `①*` and whose footnote, printed
   directly beneath the digit labels, reads `* Not necessary when setting a frequency.`;
   on page 17, the data-block diagram's brackets `⑫ ~ ⑭` and `⑮ ~ ⑰`, each spanning three
   drawn cells with no asterisk anywhere on the page. The conflict: page 17 sends the
   reader to page 23 for the format of these fields (`ⓘ See "Repeater tone/tone squelch
   frequency settings." (p. 23)`), and page 23 says its first cell is not necessary when
   setting; but page 17 prints that same cell as a separately numbered position, ⑫ and ⑮,
   inside an index-numbered record, so omitting it on a write would delete two printed
   indices from the record. Built from page 17, which I judge the clearer for the record's
   own layout: three bytes per tone field, the first printed `0` `0`. Transcribed as seen:
   the CSV rows for frame bytes 18 and 21 of `set-record-name-with-space` carry `00`,
   status `manual_documented`, with `STOP 2` in `notes`. This is also why the record has
   one derived total length and not two.

No other STOP arose. Confidence for the remainder rests on three things: every cell count
and bracket span in the page-17 diagram was counted twice, at 400 dpi and again at 600 dpi
with different window boundaries, and agreed; the field widths sum to 47 with no gap and
no overlap, each bracket's descending leg landing exactly on a drawn cell boundary; and
every hex value transcribed (`FE`, `B6`, `E0`, `FD`, `19`, `1A`, `00`, `18`, `01`, `41`,
`5A`, `30`, `39`) was legible without ambiguity at 400 dpi enlarged, and legible again at
600 dpi.

## Observed disagreements

1. **The contents page is one page short throughout the region read.** PDF page 2, the
   `Remote control (CI-V) information` contents list. It prints `• Memory channel content
   …… 16`, `• Codes for character entries …… 17`, `• Band stacking register …… 18` and
   `• Keyer memory character entries …… 18`. The pages that actually bear those headings
   carry the folios 17, 18, 19 and 19; the folio equals the PDF page number throughout
   this document. The same list prints `Remote control (CI-V) information …… 2` and
   `D Command table …… 3` for material printed on folios 3 and 4. The page-6 command table
   itself is correct: it prints `See p. 17.` for `1A 00`, and folio 17 is indeed the
   memory channel content page. Recorded, not resolved. No byte in this deliverable was
   taken from the contents page, which is why this is not a STOP.

2. **A page-16 note about a skippable filter byte sits close to, but does not govern, the
   record.** PDF page 16, beneath the operating-mode table: `ⓘ Filter setting (②) can be
   skipped with commands 01 and 06. In that case, "FIL1" is selected with command 01 and
   the default filter setting of the operating mode is automatically selected with command
   06.` The commands it names are 01 and 06, not `1A 00`, and the page-17 diagram prints ⑨
   and ⑩ as two separate cells; but the two-cell operating-mode format on page 16 is the
   format page 17 sends the reader to. Recorded, not resolved. The vector carries both
   bytes.

3. **The character table the record points to omits a character the same page declares
   usable.** PDF page 18: `(space)` appears in the `Usable characters` list for
   `1A 00 Memory name`, and no row of either character-code table on that page gives a
   code for it. Recorded, not resolved; it is the reason byte 45 is `inherited_assumed`
   rather than documented, and it is set out in full in the assumption register.

4. **A second address appears in the CI-V set-mode block and is not the transceiver's own
   address.** PDF page 9, `1A* 05 SET > Connectors > CI-V`, item `00 90`, `Sets or reads
   the USB/LAN→REMOTE Transceive Address setting. (00 00=00h ~ 02 23=DFh)`. It is an
   address setting, and it is not the `B6` printed on page 3, but the two describe
   different things — page 3's `B6` is the transceiver's own default address in a frame,
   page 9's item is a transceive-forwarding address. Recorded so that a later reader does
   not mistake one for the other. No byte was taken from page 9.

## Attestation

Every value recorded here was read from this single PDF's rendered page images.
`pdftotext -layout` was run on this same PDF for navigation only and was the source of no
recorded value. Nothing else was consulted: no other file, manual, transcription, source
file, generated artefact or web resource was opened, and no directory was listed.
