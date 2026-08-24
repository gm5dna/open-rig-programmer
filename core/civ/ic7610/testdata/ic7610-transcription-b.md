# IC-7610 memory-record transcription — leg B

## Source

Document title as printed on the cover (PDF page 1): the black cover panel prints the ICOM
logo and, in the band beneath it, **CI-V REFERENCE GUIDE**; lower on the same page,
**HF/50MHz TRANSCEIVER** above the model name **IC-7610**, and **Icom Inc.** at the foot.

Revision code as printed: **A7380-7EX-4**, printed at the bottom left of PDF page 17 (the
unnumbered back cover), directly above the line `© 2017–2025 Icom Inc.    Sep. 2025`.

File path: `/Users/stuart/Documents/working/coding/ft710-programmer.nosync/docs/fixtures-private/manuals/ic7610_civ_ENG_4.pdf`

Page count: 17 PDF pages.

## Extent

Rendered: PDF pages 1, 6–14 and 17. Read in detail: PDF pages 7, 10, 11, 12, 14; PDF pages
1 and 17 were read only for the Source section above.

| PDF page | printed folio | what it contributed |
|---|---|---|
| 1 | (no folio) | cover title |
| 7 | 6 | see below — set-mode/menu material |
| 10 | 9 | see below — set-mode/menu material |
| 11 | 10 | the referenced sections `• Operating frequency` (values for ④ ~ ⑧) and `• Operating mode` (values for ⑨, ⑩) |
| 12 | 11 | **all three transcribed diagrams** and the character tables |
| 14 | 13 | the referenced section `• Repeater tone/tone squelch frequency settings` (values for ⑫ ~ ⑭ and ⑮ ~ ⑰) |
| 17 | (no folio) | revision code |

PDF page 13 (folio 12) was rendered and read in full, far enough to confirm that it carries
no part of the memory record — it holds `• Data mode with filter width settings`, `• IF
filter width settings`, `• AGC time constant settings`, `• SSB transmission passband width
settings`, `• SSB-DATA transmission passband width setting`, `• RX HPF/LPF setting for each
operating mode`, `• Bandscope edge frequency settings`, `• Color settings` and `• Offset
frequency settings` — and nothing was taken from it. PDF pages 6, 8 and 9 were rendered as
part of the locating sweep but were never opened.

**Where the transcribed material begins and ends.** On PDF page 12 (folio 11) the running
head `Remote control` is printed at the top, then `◇ Command formats`. The memory record
begins with the bold heading `• Memory content` and the line `Command: 1A 00`, immediately
followed by the byte band (D1). It continues through the numbered paragraphs `①, ② Memory
channel numbers` … `⑱ ~ ㉗ Memory name settings` (with the D2 and D3 sub-diagrams) and ends
with the right-column block `To clear the memory channel contents on 1A 00: ①, ②: … ③: “FF”
④: None`. What is printed immediately before it is the end of PDF page 11 (`• Band stacking
register`, its two code tables and the sentence beginning `For example, when sending/reading
the oldest contents…`). What is printed immediately after it, in the left column of the same
page 12, is the bold heading `• Codes for character entries`.

**Character table — was it printed at all, and what it contributed.** Yes. PDF page 12
(folio 11) prints, under `• Codes for character entries` / `Command: 1A 00, 1A 05 01 33,
01 40, 01 55, 01 61, 01 65`, two tables: `- Character codes— Letters and Numbers` (three
populated rows: `A–Z 41–5A`, `0–9 30–39`, `a-z 61–7A`, with two cells struck through by a
diagonal rule) and `- Character codes— Symbols` (16 rows × two Character/ASCII-code pairs).
These supplied the whole `values_verbatim` cell for ⑱ ~ ㉗. The same page's right column
prints a `Cmd. | Sub cmd. | Set item/selectable characters` table whose first row is
`1A | 00 | Memory name*`, and the footnote `* Usable characters: …`, which is recorded in
that row's `notes`.

**Set-mode/menu pages — were they printed at all, and what they contributed.** Both were
printed and both were read.
- PDF page 7 (folio 6) prints the running head `Remote control` and the heading
  `◇ Command table`, and carries two `Cmd. | Sub cmd. | Data | Description` tables of
  `1A* 05` set-mode items (`Connectors > …`, `Network > …`). **It contributed nothing** to
  this transcription: none of the memory-record fields ① ~ ㉗ refers to a set-mode item, and
  no value in the CSV came from this page.
- PDF page 10 (folio 9) prints `◇ Command table` and carries the `1C`/`1E`/`21*`/`25*`/`26*`/
  `27*`/`28`/`29` command table plus footnotes `*1` ~ `*7` and a `MENU » SET > Connectors >
  CI-V` menu-path box. **It contributed nothing** to this transcription, for the same reason;
  no value in the CSV came from this page.

An absent statement is recorded as absent: on PDF page 12 the fields ④ ~ ⑧, ⑨, ⑩, ⑫ ~ ⑭,
⑮ ~ ⑰ and ⑱ ~ ㉗ have **no values printed against them at all** — only a `See “…”`
cross-reference. Each of those rows says so in `notes`, and the `values_verbatim` cell was
filled from the cross-referenced section on the page named in `pdf_page`.

## Method

1. **Locate, 300 dpi.** `pdftoppm -png -r 300 -f 6 -l 13` and `-f 14 -l 14` of the target
   PDF into a fresh directory, read as images to find the sections named in the task.
   (A first directory I created was removed from under me by something else writing into
   `evidence/ic7610`; I therefore re-made a uniquely named directory,
   `evidence/ic7610/legb_221221/`, and re-rendered everything into it. Diagnosing that
   required listing `evidence/ic7610/` itself, which showed a sibling directory
   `evidence/ic7610/r300/` containing seventeen `p-NN.png` files; those filenames are all I
   saw of it — no file in it was opened, and it was left untouched. Apart from that one
   listing, and listings of my own `legb_221221/` output directory, no directory was listed
   and no file outside the target PDF and my own renders and outputs was opened. The
   attestation below should be read against this disclosure.)
2. **Read, 400 dpi.** `pdftoppm -png -r 400 -f 7 -l 14 … legb_221221/q` (page raster
   3308 × 4678). Every pass-1 value was read from these.
3. **Crop and enlarge.** ImageMagick **was** available (`/opt/homebrew/bin/magick`) and was
   used throughout, e.g.
   `magick q-12.png -crop 2400x400+450+760 +repage -resize 200% c_band_full.png`
   (whole band with brackets), `-crop 2000x100+630+930 +repage -resize 200%` (the cell row
   alone, for counting), `-crop 700x180+600+830 … -resize 400%` and the same at +1250,
   +1900, +2450 (four overlapping bracket segments), `-crop 1300x520+250+1450 … -resize 250%`
   (the ③ box and its legend), `-crop 1700x330+1250+1290 … -resize 250%` (the ⑪ box and its
   two leader lines), `-crop 1350x280+310+2800`, `+310+3110`, `+310+3760` at 250% (the two
   character tables), `-crop 1400x620+1690+2790` at 250% (the Cmd./Sub cmd. table),
   `-crop 1500x200+1670+3930` at 350% (the `* Usable characters` footnote), plus
   `-crop 1400x1200+250+840` and `-crop 1450x800+250+1930` on `q-11.png` and
   `-crop 1500x900+1700+1780` on `q-14.png` for the cross-referenced sections.
4. **`pdftotext -layout`** *was* run once, on this same PDF, to find which page carries the
   heading `Repeater tone/tone squelch`. It was **navigational only** and is the source of
   no recorded value.
5. **tesseract** was available but was **not used**. Every glyph was legible by eye on the
   400 dpi and 500 dpi enlargements, so no OCR aid was needed and no OCR value was recorded.
6. **Second independent pass.** Done, from a different raster. The second raster differs in
   three ways: a different dpi (**500 dpi**, page raster 4134 × 5847, rendered by a separate
   `pdftoppm` run into `legb_221221/pass2/`), **different crop windows** (the band cut into
   *thirds* — `-crop 900x290+780+1055`, `+1650+1055`, `+2520+1055` — where pass 1 had cut it
   into halves and into four bracket segments), and **different enlargements** (400 % / 350 %
   / 300 % / 220 % / 200 % rather than pass 1's 200 % / 250 % / 350 %). Re-read in pass 2:
   the whole cell row and every bracket span and shading state; the ①, ② value list; the ③
   box, its `Fixed` leader and its four-value legend; the ⑪ box and both leader lines; the
   `• Operating frequency` nibble labels; the `①Receiving mode` / `②Filter setting` table;
   the `• Repeater tone/tone squelch frequency settings` nibble labels and its footnote; and
   both character tables in full.

   **Disagreements between the two passes: none.** Every cell agreed, including the count of
   18 drawn cells, the position of all eight bracket end-ticks, the shading of every cell,
   the crossing of the ⑪ leader lines, and every code and glyph in the character tables. No
   third render was needed.

**One formatting decision, declared.** Where the page sets a value list in aligned columns
(e.g. `00 01 ~ 00 99:` and its meaning are separated by a tab-width gap), the run of spaces
is column alignment rather than content and has been reduced to a single space in the CSV.
In the ⑱ ~ ㉗ `values_verbatim` cell the ` = ` between a character and its code is a join
separator of mine: the tables print the character and the code in adjacent cells with no
punctuation between them. Both facts are also stated in that row's `notes`.

## Hazards encountered

- **(a) Numeral styling may vary within one diagram — NOT ENCOUNTERED.** Every index numeral
  on all three diagrams (D1's eight bracket labels ①, ②, ③, ④ ~ ⑧, ⑨, ⑩, ⑪, ⑫ ~ ⑭, ⑮ ~ ⑰,
  ⑱ ~ ㉗, and the standalone ③ over D2 and ⑪ over D3) is drawn in one and the same style: a
  plain black numeral inside a thin unfilled circular outline, on white. No circled/filled,
  outlined, bracketed or bold variant appears anywhere in this material, at 400 dpi or at
  500 dpi.
- **(b) Diagrams may be vector groups with rotated labels — ENCOUNTERED.** Not on D1/D2/D3
  themselves, whose labels are all horizontal, but on the two cross-referenced diagrams whose
  values I recorded: `• Operating frequency` (PDF page 11) prints its ten nibble labels
  rotated 90° anticlockwise, and `• Repeater tone/tone squelch frequency settings` (PDF
  page 14) prints its six the same way. Both were read from the render by following each
  arrow from the label up to the nibble it points at, left to right, not from any text order.
- **(c) Leader-line label order may be reversed — ENCOUNTERED, in D3.** In the ⑪ sub-diagram
  the two leader lines cross. The upper, first-printed legend line `0: OFF, 1: TONE, 2: TSQL`
  is reached by the arrow rising from the **second (right)** nibble; the lower, second-printed
  line `0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3` is reached by the arrow rising from the
  **first (left)** nibble, which runs down and then right beneath the other leader. Confirmed
  by eye at 250 % (400 dpi) and again at 350 % (500 dpi). In D2 the order is *not* reversed:
  the left nibble's arrow goes to `Fixed`, the right nibble's to the four-value list. The CSV
  lists the values in printed order and states the positional mapping in `notes`.
- **(d) A printed index may differ from a field's measured position — NOT ENCOUNTERED.** D1
  contains two structurally identical three-byte blocks (⑫ ~ ⑭ and ⑮ ~ ⑰), so both printed
  index and measured byte position are recorded for every field in the CSV `notes`. Measured
  on the render, expanding each dashed `…` cell to the indices its bracket spans, the block
  runs: ①=byte 1, ②=2, ③=3, ④ ~ ⑧=bytes 4–8, ⑨=9, ⑩=10, ⑪=11, ⑫ ~ ⑭=bytes 12–14,
  ⑮ ~ ⑰=bytes 15–17, ⑱ ~ ㉗=bytes 18–27. Printed index and measured position coincide
  everywhere; neither was reinterpreted in the light of the other.

## STOP findings

1. **PDF page 12 (folio 11), right column, the two lines
   `⑫ ~ ⑭ Repeater tone frequency setting` / `⑮ ~ ⑰ Tone squelch frequency setting` and the
   line beneath them, `See “• Repeater tone/tone squelch settings.”`** — the cross-reference
   names a section title that the document does not print. The section it points at, on PDF
   page 14 (folio 13), right column, is headed `• Repeater tone/tone squelch frequency
   settings`; the word `frequency` is present in the heading and absent from the
   cross-reference. Two printed things about the same object disagree, which is why it stops.
   Transcribed as seen: the cross-reference text is quoted verbatim in the `notes` of both
   affected rows, the heading is quoted verbatim in their `visual_anchor`, and both rows
   carry `STOP 1` in `notes`. Nothing was repaired: I did not silently treat the two titles
   as the same string, and the values in those rows come from the page-14 diagram identified
   by position, not by title match.

No other STOP arose. Reasons for confidence on the rest: the band's drawn-cell count (18) was
counted three times, from three different rasters, and agreed; expanding the two dashed `…`
cells against their brackets gives 2+1+5+2+1+3+3+10 = 27 bytes, which matches the highest
printed index ㉗ exactly and leaves no gap or overlap; each cross-referenced diagram's own
byte count matches the index span that points at it (5 bytes for ④ ~ ⑧, 2 for ⑨, ⑩, 3 for
each tone block, and `Up to 10 characters.` for the ten indices ⑱ ~ ㉗); the index sequence
1…27 is continuous, in order, with no index printed twice and no styling difference; and no
value anywhere was unreadable at 400 dpi enlarged.

## Observed disagreements

Recorded as printed; not resolved.

1. **Shading breaks its own alternation at the ⑮ ~ ⑰ / ⑱ ~ ㉗ boundary.** In D1 the cell
   groups alternate grey / white / grey / white / grey / white as the brackets change
   (①, ② grey; ③ white; ④ ~ ⑧ grey; ⑨, ⑩ white; ⑪ grey; ⑫ ~ ⑭ white), but then both
   ⑮ ~ ⑰ and ⑱ ~ ㉗ are drawn grey, so six consecutive cells at the right-hand end share one
   fill. The group boundary is still marked, by the ⑮ ~ ⑰ bracket's down-tick and a solid
   cell rule. Confirmed identically at 400 dpi and 500 dpi.
2. **`a-z` is set with a hyphen where every neighbouring range uses an en dash.** In
   `- Character codes— Letters and Numbers` the cells read `A–Z`, `41–5A`, `0–9`, `30–39`,
   `61–7A` — all en dash — but the third character cell reads `a-z` with a plain hyphen.
3. **The same range is printed two ways on one page.** The paragraph `①, ② Memory channel
   numbers` prints `00 01 ~ 00 99:` with spaces around the tilde; the `To clear the memory
   channel contents on 1A 00:` block, in the other column of the same page, prints
   `Memory channel (00 01~00 99)` with none.
4. **The clear-list addresses ④ on its own.** The band prints ④ only as the left end of the
   bracket `④ ~ ⑧`, yet the clear-list prints `④: None` as a single index. The two are not
   in the same sequence — the clear-list is its own four-line list, ①, ② / ③ / ④ — so this is
   not an index discontinuity in D1, and it is recorded here rather than as a STOP.
5. **No code is printed for the space character.** Neither `- Character codes— Letters and
   Numbers` nor `- Character codes— Symbols` has a row for space, yet the footnote on the
   same page reads `* Usable characters: A to Z, a to z, 0 to 9, (space), …` and applies to
   the row `1A | 00 | Memory name*`. An omission from a table is not the same kind of thing
   as a contradiction between two statements, so it is recorded here and not as a STOP.
6. **The `①Receiving mode` list on PDF page 11 has gaps.** It prints 00, 01, 02, 03, 04, 05,
   07, 08, 12, 13; codes 06, 09, 10 and 11 are not printed at all. Transcribed as printed,
   unexpanded.
7. **The `②Filter setting` column prints two em-dash cells.** Its fourth and fifth rows read
   `—`, opposite `12:PSK` and `13:PSK-R`. These are carried into `values_verbatim` as the two
   trailing `—` entries rather than dropped.
8. **The referenced tone diagram prints an asterisked index and a footnote that the memory
   record does not repeat.** PDF page 14 labels its first byte `①*` and prints
   `*Not necessary when setting a frequency.`; PDF page 12 says nothing about whether that
   applies to ⑫ ~ ⑭ or ⑮ ~ ⑰ inside a 1A 00 memory record. Recorded, not interpreted.

## Attestation

Every value recorded here was read from this single PDF's rendered page images.
`pdftotext -layout` was run on this same PDF for navigation only and was the source of no
recorded value. Nothing else was consulted: no other file, manual, transcription, source
file, generated artefact or web resource was opened, and no directory was listed.
