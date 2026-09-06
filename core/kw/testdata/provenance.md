# Provenance — Brief G, frame-geometry witness and hand-derived golden vectors
## Kenwood TS-590S/TS-590SG and TS-480HX/TS-480SAT

**Derivation date: 05/09/2026**

---

## 1. The two documents

| | TS-590 book | TS-480 book |
|---|---|---|
| File | `docs/fixtures-private/manuals/ts590sg_pc_rev3.pdf` | `docs/fixtures-private/manuals/ts480_pc_2003.pdf` |
| Title as printed on the cover, verbatim | "TS-590S / TS-590SG" (blue panel) then "PC CONTROL COMMAND / Reference Guide" | "PC CONTROL COMMAND / REFERENCE FOR THE / TS-480HX/ SAT TRANSCEIVER" |
| Running head on every content page, verbatim | "PC CONTROL COMMAND REFERENCE GUIDE" | "PC CONTROL COMMAND" |
| Publisher line, verbatim | "JVCKENWOOD Corporation" | "KENWOOD CORPORATION" |
| Date / revision as printed | "January/30/2019". **No revision number is printed anywhere on the cover or on any page read.** The string "rev3" occurs only in the supplied filename. | "© 2003/11/28" followed by the printing-code line "09 08 07 06 05 04 03 02 01 00". **No revision number printed.** |
| Extent | 35 PDF pages; cover unnumbered, folios "– 1 –" … "– 33 –" seen | 26 PDF pages; cover unnumbered, folios "1" … "25" seen |

Where a revision was expected and none was found, the fact is recorded rather than
inferred. See FINDING F14.

---

## 2. Pages read (PDF page → printed folio)

Nothing outside this list was opened. Only the two PDFs and the pre-rendered page
images named in the brief were read; no directory of the repository was listed.

**TS-590 book (17 of 35 pages):**
p.1 → cover · p.2 → folio 1 · p.3 → folio 2 · p.4 → folio 3 · p.6 → folio 5 ·
p.7 → folio 6 · p.8 → folio 7 · p.10 → folio 9 · p.12 → folio 11 · p.13 → folio 12 ·
p.14 → folio 13 · p.17 → folio 16 · p.18 → folio 17 · p.19 → folio 18 ·
p.20 → folio 19 · p.28 → folio 27 · p.29 → folio 28

**TS-480 book (14 of 26 pages):**
p.1 → cover · p.2 → folio 1 · p.3 → folio 2 · p.4 → folio 3 · p.6 → folio 5 ·
p.7 → folio 6 · p.8 → folio 7 · p.11 → folio 10 · p.13 → folio 12 · p.14 → folio 13 ·
p.15 → folio 14 · p.21 → folio 20 · p.22 → folio 21 · p.23 → folio 22

Statements below that say "nowhere in the book" or "no such thing appears" are
therefore bounded by these page lists, and are worded that way in the findings.

---

## 3. Method

* **Base render.** The pre-rendered 300 dpi page images supplied with the brief were
  read visually, page by page. No text layer was extracted; no `pdftotext`, no OCR,
  no search. Every position number quoted below was read off the printed numbered
  ruler that the Kenwood books draw above each chart row; no length was inferred
  from a stated range, from a parameter's apparent width, or from anything
  remembered about Kenwood or Yaesu framing.
* **Two independent counts per chart.** Each chart's ruler was counted once
  left-to-right, then a second time by locating the terminator cell and counting
  backwards to cell 1. Multi-row charts (MR/MW Answer, EX, SS, SU) were counted row
  by row and then summed, and the sum re-checked against the printed ruler value of
  the last filled cell.
* **The third look, at 600 dpi.** Where the two counts disagreed, where a glyph was
  ambiguous at 300 dpi, or where the ruler interleaved with the legend text, the
  region was re-rendered at **600 dpi** with `/opt/homebrew/bin/pdftoppm -r 600 -png
  -x … -y … -W … -H …` and read again. Every crop was written only under
  `…/scratchpad/quarantine/G/work/`. The regions that needed the third look, and
  what it settled:

  | crop | region | what it settled |
  |---|---|---|
  | `mr590-read2-19.png` | TS-590 folio 18, MR Read row | position 7 is a **colon**, two round dots of equal size — F1 |
  | `mr590-ans50-19.png` | TS-590 folio 18, MR Answer position 50 | the same-size glyph there is a **semicolon** (dot + comma tail); the two are visibly different in the same typeface at the same size, which is what makes F1 safe |
  | `mr480-read-14.png` | TS-480 folio 13, MR Read row | position 7 is a **semicolon** — the same cell of the same command in the other book |
  | `mr480-p8-14.png` | TS-480 folio 13, MR legend | "Tone Number.  Refer to page 35." — F4 |
  | `mw480-p8-15.png` | TS-480 folio 14, MW legend | "Tone Number.  Refero to the TN command." — F6 |
  | `id480-set-11.png` | TS-480 folio 10, ID Set row | ruler 1..10 printed, all ten cells genuinely blank, not faint — F3 |
  | `cd0-590-06.png` | TS-590 folio 5, CD0 Answer row | cells 1–3 read "C G 0" — F11 |
  | `eq590-setruler-08.png`, `es590-setruler-08.png`, `da590-setruler-07.png` | TS-590 folios 7 and 6 | the position-1 header cell of those Set rulers is **blank** — F12 |
  | `ex590-chart-08.png` | TS-590 folio 7, whole EX block | all three EX rulers and every filled cell, confirming Read 10 / Set 18 / Answer 18 |

* **Nothing else was run.** Rendering and cropping (`pdftoppm`) and writes of this
  agent's own output files. No code was written, no generator was used, no frame was
  produced by a program: every vector below was composed by hand from the counted
  field map, and then checked field-by-field against that map with a substring dump
  purely as a typing check on the agent's own files.
* **Nothing else was consulted.** No source file, no other manual, no web access, no
  prior knowledge of Kenwood or Yaesu wire formats. Where the books disagree with
  each other the disagreement is recorded, never reconciled.

---

## 4. The nothing-else-consulted statement

> For every vector in every `.golden` file in this directory: **NO code, no
> generator, no other file and no other document was consulted.** The only inputs
> were the two PDFs named in §1, read visually as page images at 300 dpi and, where
> §3 says so, at 600 dpi.

## 5. Hardware status

> **UNVERIFIED for every vector in every file.** No radio was connected, no frame was
> transmitted, and no capture of any kind was taken. Every vector is a paper
> derivation from a printed chart.

---

## 6. Per command, per radio

Legend for the columns: *counted* is the number of printed ruler positions actually
filled by the chart; *documented* means the byte's value follows from the printed
chart or the printed legend; *assumed* means it does not and is marked
INHERITED-ASSUMED in the `.golden` header.

### 6.1 MR

| | TS-590 (`MR-590.golden`) | TS-480 (`MR-480.golden`) |
|---|---|---|
| Source | folio 18 / PDF p.19 | folio 13 / PDF p.14 |
| Set chart | not printed at all | printed, ruler 1..10, **all cells empty** (F3) |
| Read counted | **7** | **7** |
| Answer counted | **50** | **50** |
| Read position 7 | **`:`** (colon) — F1 | `;` (semicolon) |

Field map, identical in both books:
`1-2` command · `3` P1 · `4` P2 · `5-6` P3 · `7-17` P4 (11) · `18` P5 · `19` P6 ·
`20` P7 · `21-22` P8 · `23-24` P9 · `25-27` P10 · `28` P11 · `29` P12 · `30-38` P13 (9) ·
`39-40` P14 · `41` P15 · `42-49` P16 (8) · `50` terminator.

Documented vs assumed, TS-590 MR:
* documented — every field width; the terminator glyphs as printed; P1 0/1 and its
  four notes; P2/P3 via MC (including that a **space** is what a response carries in
  P2 for a channel below 100 — so the space in the Answer vectors is documented, not
  assumed); P5/P6 via MD and DA; P7 tone state; P8/P9 via the TN and CN index→Hz
  tables, which this book **does** print; P10, P12, P13 fixed values; P11, P14, P15
  legends.
* assumed — the Read P2 digit for a channel below 100 (the MC wording permits "0"
  **or** a space for input commands and does not say which MR expects; "0" chosen);
  the 11-digit frequency values; **the padding of a name shorter than 8** (the book
  states no justification and no pad character; right-padding with ASCII space
  assumed); P8/P9 when the tone system is OFF ("00" assumed); the illustrative
  channel numbers, modes and names.

Documented vs assumed, TS-480 MR: as above except that P2, P10, P11, P12, P13 and
P15 are **fixed by legend** in this book, P6 is Lockout status and P14 is a step size
(refer to ST); the tone index→Hz tables are **absent** from this book altogether
(F15); and the same padding assumption applies to P16.

**The one hardware capture that would settle each assumption**

| assumption | the single capture that settles it |
|---|---|
| MR Read P2 for a channel < 100: `0` vs space | send `MR0003:` / `MR0003;` / `MR0 03;` in turn to a radio holding a known channel 3 and record which is answered and which returns `?;` |
| P16 padding for a short name | store a 3-character name in one channel from the front panel, then `MR` that channel and record positions 42–49 verbatim, spaces included |
| the 11-digit frequency encoding | set one channel to a known frequency from the front panel and read positions 7–17 back |
| P8/P9 when tone is OFF | store a channel with tone OFF and read positions 21–24 back |
| the TS-590 `:` vs `;` at Read position 7 | the same first capture: whichever terminator the radio answers is the true one — but **do not** pre-resolve it in the fixture |

### 6.2 MW

| | TS-590 (`MW-590.golden`) | TS-480 (`MW-480.golden`) |
|---|---|---|
| Source | folio 19 / PDF p.20 | folio 14 / PDF p.15 |
| Set counted | **50** | **50** |
| Read / Answer charts | not printed at all | printed, rulers 1..10, **all cells empty** (F3) |
| Note about omitting the name | printed: *"If you do not specify one digit in P16 and execute all the parameters from P4 to P15 set to 0, the channels specified by P2 and P3 will be erased."* plus *"* ";" (semicolon) cannot be used for the parameter P16."* | **none printed at all** |

Field map identical to MR with `MW` in positions 1–2.

Assumed, TS-590 MW: the Set P2 digit (legend permits "0" or a space; "0" chosen);
frequency values; P8/P9 when tone is OFF; **and the reading of "do not specify one
digit in P16" as meaning P16 omitted entirely, 0 characters, terminator at position
42** (F7). Assumed, TS-480 MW: frequency values, P8/P9 when tone is OFF, P14 value.

The one capture that settles the TS-590 erase shape: send the 42-character
`mw590_set_ch03_erase_no_P16` vector to a radio with channel 3 occupied, then `MR`
channel 3 and see whether it reads back empty; if it errors, send the 49-character
variant (P16 one character short) and repeat.
The one capture that settles TS-480 MW padding: store an 8-character name via `MW`,
then a 3-character one, and read both back with `MR`.

### 6.3 MC

| | TS-590 (`MC-590.golden`) | TS-480 (`MC-480.golden`) |
|---|---|---|
| Source | folio 16 / PDF p.17 | folio 12 / PDF p.13 |
| Set / Read / Answer counted | **6 / 3 / 6** | **6 / 3 / 6** |
| Field map | `1-2` MC · `3` P1 · `4-5` P2 · `6` `;` | identical |
| Position 3 | **variable** (100's digit; "0" or space on input, space on a response, real digit ≥ 1 for 100+) | **fixed**: "0: Always 0 for the TS-480 (Memory bank number)." |
| P-channel / extension wording | printed: "Channel numbers P00 ~ P09 are represented by 100 ~ 109." and "TS-590SG extension channel numbers E00 ~ P09 are represented by 110 ~ 119." (F5) | **none** — no P-channel and no extension wording at all |

Assumed in both: only the illustrative channel numbers. On the TS-590, which of
"0" and space a Set command should carry is left undecided and both forms are
emitted. The one capture that settles it: send `MC003;` and `MC 03;` in turn and
record which is accepted.

### 6.4 ID

| | TS-590 (`ID-590.golden`) | TS-480 (`ID-480.golden`) |
|---|---|---|
| Source | folio 13 / PDF p.14 | folio 10 / PDF p.11 |
| Set chart | not printed at all | printed, ruler 1..10, **all cells empty** (F3) |
| Read / Answer counted | **3 / 6** | **3 / 6** |
| Field map | `1-2` ID · `3-5` P1 · `6` `;` | identical |
| Legend, every pair printed | "021: TS-590S" and "023: TS-590SG" — two pairs, no 022 | "020: TS-480" — one pair; HX and SAT are **not** distinguished here |

Assumed bytes: **none**. Every byte of every ID vector is documented.
The one capture worth taking anyway: read `ID;` from a TS-480SAT and from a
TS-480HX and confirm both really answer `ID020;`, since the paper says they must.

### 6.5 AI

| | TS-590 (`AI-590.golden`) | TS-480 (`AI-480.golden`) |
|---|---|---|
| Source | folio 3 / PDF p.4 | folio 3 / PDF p.4 |
| Set / Read / Answer counted | **4 / 3 / 4** | **4 / 3 / 4** |
| Field map | `1-2` AI · `3` P1 · `4` `;` | identical |
| Legend values printed | 0, 2, 4 only | 0, 1, 2, 3 |
| Meaning of value 2 | "AI ON (without backup)" | "Only extended AI format is ON" |

Assumed bytes: **none** in either file. The one capture: send `AI1;` and `AI3;` to a
TS-590 and record whether they error, since the book lists neither (F9).

### 6.6 EX

| | TS-590 (`EX-590.golden`) | TS-480 (`EX-480.golden`) |
|---|---|---|
| Source | folio 7 / PDF p.8 | folio 6 / PDF p.7 |
| Read counted | **10** | **10** |
| Set counted, as illustrated | **18** (P5 drawn as 8 characters) | **12** (P5 drawn as 2 characters) |
| Answer counted, as illustrated | **18** | **12** |
| Book's own worked example | none in the EX block | `EX00000000;` and `EX00000003;` — **11** characters each |
| P1 legend | "000 ~ 087: Menu number (TS-590S)" / "000 ~ 099: Menu number (TS-590SG)" | "000 ~ 060: Menu No." |
| P2 "Always" legend, verbatim | "00:  Always 00" | "00: Always 00 for the TS-480" |
| P3 "Always" legend, verbatim | "0:  Always 0" | "0: Always 0 for the TS-480" |
| P4 "Always" legend, verbatim | "0:  Always 0" | "0: Always 0 for the TS-480" |
| P5 variable-length note, verbatim | "String of alphanumeric characters for the Menu setting (variable length)" | "A string of characters (Variable length) / Normally 1-digit for the TS-480. / Menu No. 32, 35 and 48 ~ 52 use 2-digit parameters." |

Field map, both books: `1-2` EX · `3-5` P1 · `6-7` P2 · `8` P3 · `9` P4 · `10…` P5 ·
terminator last. Fixed positions are 6, 7 (P2), 8 (P3), 9 (P4) in both.

Assumed: the **length** of P5 for any menu whose width the legend does not name, and
therefore the terminator position; the padding of a short P5 (nothing at all is
printed about it in either book); the illustrative menu numbers and P5 values.
The one capture that settles each: `EX` the menu in question with `AI` off, and read
the answer's byte count — one read per menu number settles that menu's P5 width and,
for the 8-character TS-590 power-on-message menu, its padding as well.

### 6.7 FV — TS-590 only (`FV-590.golden`)

Source folio 12 / PDF p.13. Read counted **3**; Answer counted **7**.
Field map: `1-2` FV · `3-6` P1 (four characters) · `7` `;`.
Legend verbatim: "P1 / Reads out the character string of the firmware version."
Worked example verbatim: *"For example, for firmware version 1.00, it reads
"FV1.00;"."* — 7 characters, in exact agreement with the counted chart, and it shows
that the four characters of P1 need not be digits (the full stop occupies one).
Assumed: only the bytes of the two extra version numbers offered beyond the book's
own example. The one capture: `FV;` against one radio of known firmware.

### 6.8 TY — TS-480 only (`TY-480.golden`)

Source folio 22 / PDF p.23. Set chart printed with ruler 1..10 and **all cells
empty** (F3). Read counted **3**; Answer counted **6**.
Field map: `1-2` TY · `3-4` P1 (**two** characters) · `5` P2 (one) · `6` `;`.
Legend verbatim: "P1 / Reserved" then "P2 / 0: TS-480HX (200 W) / 1: TS-480SAT
(100 W + AT) / 2: Japanese 50 W type / 3: Japanese 20 W type".
Assumed: **positions 3 and 4 in their entirety.** The legend for P1 is the single
word "Reserved" — no value, no range, no fill character. `00` was assumed. The book's
only general fill guidance is the folio-2 note, which would permit almost any two
printable characters there. This is the largest single assumption in this whole
directory. The one capture that settles it: read `TY;` once from any TS-480 and copy
positions 3 and 4 verbatim.

---

## 7. FINDINGS — recorded, not resolved

**F1 — a punctuation mark where a terminator is expected.**
TS-590 book, MR **Read** chart, folio 18 / PDF p.19, position 7: the cell prints a
**colon `:`**, not a semicolon. Verified at 600 dpi and contrasted, at the same size
in the same typeface on the same page, with the unmistakable semicolon in position
50 of the MR Answer chart. The TS-480 book prints a semicolon in the identical cell
of the identical command. Left unresolved: `MR-590.golden` carries the colon
verbatim and deliberately emits **no** semicolon-terminated Read vector.

**F2 — the same command drawn at two different lengths in the two books.**
EX. TS-590 draws Set and Answer over **18** positions with an 8-character P5;
TS-480 draws them over **12** positions with a 2-character P5. EX Read is 10 in both.
The three fixed positions are the same (6, 7, 8, 9) but the wording differs. Every
other command in this brief that appears in both books has **identical** counted
lengths: MR Read 7, MR Answer 50, MW Set 50, MC 6/3/6, ID 3/6, AI 4/3/4.

**F2b — within the TS-480 book, chart, legend and worked example disagree on EX.**
The chart draws a 2-character P5 (terminator at position 12); the legend says
"Normally 1-digit for the TS-480."; the book's own two worked examples on the same
page are 11 characters long (1-character P5). All three are quoted verbatim in
`EX-480.golden` and none is preferred over the others.

**F3 — parameter/row labels printed above EMPTY charts.**
The TS-480 book repeatedly prints a row label and its numbered ruler 1..10 with
every one of the ten cells blank. Every instance seen on the pages read:
* folio 5 / p.6 — **CH** Read and Answer; **DN** Read and Answer
* folio 10 / p.11 — **ID** Set
* folio 13 / p.14 — **MR** Set
* folio 14 / p.15 — **MW** Read and Answer
* folio 20 / p.21 — **SR** Read and Answer
* folio 21 / p.22 — **SV** Read and Answer; **TX** Read
* folio 22 / p.23 — **TY** Set; **UL** Set and Read; **UP** Read and Answer

No empty chart was seen anywhere in the TS-590 book on the pages read: that book
omits an absent row entirely (ID, FV, MW, MK, CH, EM, SR, SV, CD2 all print only the
rows that exist). The two books therefore differ in house style, not only in content.

**F4 — a cross-reference to a page that does not exist in the document.**
TS-480 book, MR legend, folio 13 / p.14: "P8 / Tone Number.  **Refer to page 35.**"
The document has 26 PDF pages and its highest folio seen is 25. The line names no
other document — unlike the TN and CN legends in the same book, which say "Refer to
page 32 **of the TS-480 instruction manual**" and "page 33 of the TS-480 instruction
manual" respectively.

**F5 — a range that mixes two prefixes.**
TS-590 book, MC legend, folio 16 / p.17: "TS-590SG extension channel numbers
**E00 ~ P09** are represented by 110 ~ 119." Transcribed verbatim into
`MC-590.golden`; the mapping 110–119 is used as printed and the prefix is not
"corrected".

**F6 — one parameter, two spellings, in the same book.**
TS-480 book, parameter P8 of the memory-channel pair:
* MR (folio 13): "Tone Number.  Refer to page 35."
* MW (folio 14): "Tone Number.  **Refero** to the TN command."

Same parameter, same book, facing pages: different wording, a typo in one of them,
and two different referents (a page number vs a command).

**F7 — an ambiguous instruction about a shorter frame.**
TS-590 book, MW: "If you do not specify **one digit** in P16 and execute all the
parameters from P4 to P15 set to 0, the channels specified by P2 and P3 will be
erased." Whether "do not specify one digit" means P16 is omitted entirely or is one
character short is not stated. `MW-590.golden` emits the omitted-entirely shape and
marks the reading INHERITED-ASSUMED.

**F8 — every position where the two books' frames of the same command differ in
which positions are fixed.** Lengths are identical (MR Answer and MW Set are 50 in
both); the fixedness is not.

| position | parameter | TS-590 | TS-480 |
|---|---|---|---|
| 4 | P2 | variable — channel-number 100's digit | **fixed** "Always 0 for the TS-480." |
| 19 | P6 | variable — Data mode (refer to DA) | variable — **Lockout status** (different meaning) |
| 25–27 | P10 | fixed "000: Always 000" | fixed "Always 000 for the TS-480." |
| 28 | P11 | variable — FILTER A/B | **fixed** "Always 0 for the TS-480." |
| 29 | P12 | fixed "0: Always 0" | fixed "Always 0 for the TS-480." |
| 30–38 | P13 | fixed "000000000: Always 000000000" | fixed "Always 000000000 for the TS-480." |
| 39–40 | P14 | variable — FM Normal/Narrow | variable — **Step size** (refer to ST) (different meaning) |
| 41 | P15 | variable — Channel Lockout OFF/ON | **fixed** "Always 0 for the TS-480." |
| 7 (Read) | terminator | prints `:` (F1) | prints `;` |

Also, outside MR/MW: MC position 3 is variable on the TS-590 and **fixed** on the
TS-480. And the TS-590 MR P1 is "0: Simplex / 1: Split" whilst the TS-480 MR P1 is
"0: RX frequency, 1: TX frequency" — the same position, the same width, an
overlapping but not identical meaning.

**F9 — a legend whose value set differs between the books, with a collision.**
AI. TS-590 prints only 0, 2, 4 (1 and 3 are not listed at all); TS-480 prints 0, 1,
2, 3. Value **2** means "AI ON (without backup)" on one radio and "Only extended AI
format is ON" on the other.

**F10 — typographic errors in printed legends and headings.** Recorded because they
are what the transcriptions above reproduce verbatim:
* TS-480 folio 22: "Sets or reads the microprocessor **fimware** type." (TY)
* TS-480 folio 14: "**Refero** to the TN command." (MW P8, also F6)
* TS-480 folio 5: "Move the current VFO **frenquency** 1 step up…" (CH)

**F11 — a chart cell printing the wrong command letter.**
TS-590 book, folio 5 / PDF p.6, the **CD0** block: the block label reads CD0, the Set
row reads "C D 0 P1 ;" and the Read row reads "C D 0 ;", but the **Answer** row's
first three cells read "**C G 0** P1 ;". Verified at 600 dpi. Not in scope for any
vector in this directory, but it is a chart cell printing a letter where the command
name is expected, so it is on the record.

**F12 — a numbered ruler whose position-1 header is blank.**
TS-590 book, three Set charts print the ruler as "␣ 2 3 4 5 6 7 8 9 10" — the header
cell for position 1 is empty although the cell beneath it holds the first command
letter: **EQ** and **ES** (folio 7 / PDF p.8) and **DA** (folio 6 / PDF p.7). All
three verified at 600 dpi. None of the commands in this brief is affected, and no
such blank was seen anywhere in the TS-480 book on the pages read.

**F13 — every "Always" / "not used" note and which radio it names.**
* **TS-590 book: no "Always" note names a radio.** The wording is model-neutral
  throughout: "00: Always 00", "0: Always 0", "000: Always 000", "000000000: Always
  000000000" — seen at EX P2/P3/P4, MR and MW P10/P12/P13, AG P1, SQ P1, BY P2, and
  SU P12/P13 ("Always 0" in the Program-Scan column of the SU table).
* **TS-480 book: every "Always" note names the radio.** "Always 0 for the TS-480.",
  "Always 000 for the TS-480.", "Always 000000000 for the TS-480.", "00: Always 00
  for the TS-480", "0: Always 0 for the TS-480" — seen at MR and MW P2/P10/P11/P12/
  P13/P15, MC P1, EX P2/P3/P4, AG P1, AS P1, and TX P2.
* **"not used" wording.** The TS-480 MD legend names the radio: "0: No mode (**Not
  used for the TS-480**)" and "8: Tune (**Not used for the TS-480**)". The TS-590 MD
  legend uses different words for the same two slots: "0: None (setting failure)"
  and "8: None (setting failure)". In the TS-590 book the only "not used"-shaped
  notes seen are AN P2 "0: RX ANT is not used" and the MK footnote "These keys do not
  exist on the operation panel of the transceiver."
* One further asymmetry of the same kind: TS-590 SU prints "P13 is only required for
  TS-590SG. P13 does not exist in TS-590S, and the next parameter of P12 is the
  terminator." — a frame whose length depends on the model, named explicitly.

**F14 — neither book prints a revision.** See §1. The "rev3" in the 590 filename is
not corroborated by anything printed in the document.

**F15 — the tone index→Hz tables are printed in one book and absent from the other.**
* TS-590 **prints** them: TN (folio 28 / p.29) gives 00→67.0 … 41→254.1 **and
  42→1750**, with the bullet "An entered value of 43 or higher results in an error.";
  CN (folio 6 / p.7) gives 00→67.0 … 41→254.1 and has **no** index 42.
* TS-480 **prints neither**: TN (folio 21 / p.22) gives only "00 ~ 42" and defers to
  "page 32 of the TS-480 instruction manual"; CN (folio 5 / p.6) gives only "00 ~ 41"
  and defers to "page 33 of the TS-480 instruction manual". Neither of those pages is
  in this document. Any TS-480 tone index → frequency mapping is therefore **not
  derivable from this book at all**, and none is asserted in `MR-480.golden` or
  `MW-480.golden`.

---

## 8. SHA-256 of every output file

Computed over the final contents of this directory. `provenance.md` is necessarily
excluded from its own list.

```
16ebc96588108947a596a284575d339d60b51502d0b0a6c2fc404c9c159b6979  AI-480.golden
93f8913b8efe780d7f9e12fabf74b46218669b873d6d1bfe452d649537453ab2  AI-590.golden
5e21df001d9a9f86c83c39edf4681d6166c6dc48335b2da1065a94d152c7d3e1  EX-480.golden
470eea93dc1f4a041e1c825c19062b0c46e9095e62b392862b244808baf98149  EX-590.golden
d71f7f2075bfd1e8e6436793fe61040efb024dc4a45ff73573b8138c081f0d00  FV-590.golden
78faf6d69ffa427c7047a9cfa66823e10bb9f9f554d5c5376f99abba2501ef17  ID-480.golden
643d710d3159280c50b15f3d219dffa93689561fe59b232629cacba5cb99012e  ID-590.golden
64aa541e995295ed043fd9bfae161e7d3915dc15ecb54b0b91ee5afdb294253e  MC-480.golden
6dd87ca8012dadb9c234f9c37037ec999ffd6149753816e4b9b55861bd8ef860  MC-590.golden
c8ae5cf0c41ead28b64b37bce4329c6b6f27789eadc8c951bb97779402890d41  MR-480.golden
fc00c69cde8a810bd3dd89cb7afe2e93b7008f83c445b42ec38768cd5fa7e6cf  MR-590.golden
79a4a2aa88fd6f5007732a10ebc29a7d0ba9bdfea9cc7b67326f3c74d0bee9be  MW-480.golden
743ecd830415ab8c46bcc0e360a4ebaa8e164a31bfc217f6136b777cb2108277  MW-590.golden
3f4aef149ebcc9b4c6c21b258d2e666e43380cc397c47ebcd444ba24e84f7b34  TY-480.golden
```

And of this file at the moment the table above was written (self-hash of `provenance.md` is not meaningful and is therefore not given).
