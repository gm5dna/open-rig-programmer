# Provenance — FT-991A frame geometry and hand-derived golden vectors

**Date of derivation:** 05/09/2026

**Manual:** *HF/VHF/UHF ALL MODE TRANSCEIVER FT-991A — CAT Operation Reference Manual*,
YAESU MUSEN CO., LTD., Copyright 2017. The title page (PDF page 1) carries no revision
string; the printed revision **1711-D** appears alone at the bottom right of the back cover
(PDF page 20). The running footer on every content page reads
"FT-991A CAT Operation Reference Book" — the cover says "Manual", the footer says "Book".

**Nothing else was consulted.** No code, no generator, no other file, no other document, no
web access, no text-layer extraction. Every frame in the five `.golden` files was derived by
reading the printed position charts visually and counting the numbered cells. No knowledge
of any sibling Yaesu radio's frames was used.

**Hardware status: UNVERIFIED for every vector in every file.** Nothing here has been sent
to or read from an FT-991A.

## Pages read

| PDF page | Printed folio | What was taken from it |
|---|---|---|
| 1 | (cover, unnumbered) | Title, manufacturer; confirmed no revision string |
| 2 | 1 | Confirmed the folio offset (PDF page = printed folio + 1) |
| 3 | 2 | Control-command structure; the general note on filling unusable parameter digits |
| 4 | 3 | Control Command List (the Set/Read/Ans./AI table) — MC, MD, MR, MT, MW, EX, ID rows |
| 5 | 4 | Chart conventions (how a Set/Read/Answer block is laid out) |
| 8 | 7 | EX (MENU) command block; menu chart rows 001–042 |
| 9 | 8 | Menu chart rows 043–124 |
| 10 | 9 | Menu chart rows 125–153 |
| 11 | 10 | ID (IDENTIFICATION) block; IF block (third copy of the mode legend) |
| 12 | 11 | MC (MEMORY CHANNEL) block; MD (OPERATING MODE) block |
| 13 | 12 | MR, MT, MW blocks — the source of every memory frame |
| 20 | (back cover, unnumbered) | Printed revision 1711-D; copyright year |

## Method

- Pages were supplied pre-rendered at **300 dpi**. Every chart used for a count was
  **re-rendered at 600 dpi** and cropped with `pdftoppm -r 600 -x -y -W -H`, because at
  300 dpi the position numbers in the narrow chart cells sit close to the cell rules and
  the legend text interleaves with the chart rows at the right-hand edge.
- Legends were cropped **separately from the charts at 600 dpi**, since they run alongside
  the chart rows and get clipped by any crop tight enough to make the cells legible.
- One glyph was re-rendered at **1800 dpi**: the clarifier-direction legend on MR/MT/MW
  (see disagreement 1 below), to settle whether it prints one dash or two.
- The menu chart's Digits column was checked by lifting the P1 column and the Digits column
  out of the 600 dpi page renders and butting them together with `magick`, so that row
  numbers and digit values could be read against each other; the columns sit at opposite
  edges of a full-width table.
- **Two independent counts** were made of every chart used: (A) left-to-right along the
  printed position numbers, noting the last filled cell; (B) by row totals — number of
  printed rows × 10 numbered cells, minus the empty trailing cells. **All counts agreed on
  the first pass; no third look was required for any chart**, and there are no unreconciled
  count discrepancies.
- Only rendering, cropping and file writes were run.

## Per vector class

### MT — `mt-vectors.golden`

- **Chart used:** MT Set chart, printed folio 12 (PDF page 13). The MT Answer chart was
  counted independently and is the same 41-position layout; the MT Read chart is a separate
  6-position layout.
- **Counted frame length:** Set **41 bytes**; Read **6 bytes**; Answer **41 bytes**.
- **Field map:** 1–2 `MT`; 3–5 P1 channel; 6–14 P2 frequency (9 digits, Hz); 15–19 P3
  clarifier (1 direction + 4 offset digits); 20 P4 RX CLAR; 21 P5 TX CLAR; 22 P6 mode;
  23 P7 (Set: fixed 0); 24 P8 tone/squelch state; 25–26 P9 fixed `00`; 27 P10 shift;
  28 P11 fixed `0`; 29–40 P12 tag (12 positions); 41 `;`.
- **No tag ON/OFF flag exists on this chart.** The position adjacent to the tag field, P11,
  is printed `0: (Fixed)`. No flag was invented and no "tag off" vector was written.
- **Manual-documented bytes:** the command letters, every field width (counted), the
  terminator, the P1 range 001–117, the P2 unit, the P3 offset range 0000–9999, every
  P4/P5/P6/P8/P10 index and label, and the three fixed values P7=`0` (Set), P9=`00`,
  P11=`0`.
- **INHERITED-ASSUMED bytes:**
  1. *Tag padding.* The tag positions not covered by a shorter tag. The manual says only
     "up to 12 characters (ASCII)"; it never states the padding byte or the justification.
     Vectors left-justify the tag and pad with ASCII SPACE (0x20), a choice supported only
     indirectly by the general note on printed folio 2 that unusable digits may be filled
     with any character except ASCII 00–1Fh and `;`.
     **Settling capture:** one MT read of a channel whose tag is shorter than 12 characters
     — the returned Answer frame shows the padding byte and the justification in one shot.
  2. *Minus clarifier-direction byte.* Vectors use a single `-` (0x2D). See disagreement 1.
     **Settling capture:** one MT read of a channel set on the front panel to a negative
     clarifier offset shows the direction byte directly.
  3. *Tag character set.* Only "(ASCII)" is printed. Vectors use A–Z and 0–9 only.
     **Settling capture:** one MT write of a tag containing a space, a lower-case letter and
     a punctuation character, read back, shows what survives.

### MW — `mw-vectors.golden`

- **Chart used:** MW Set chart, printed folio 12 (PDF page 13). The MW Read and Answer
  charts are printed as **empty grids** (verified at 600 dpi), so no Read or Answer layout
  exists for this command.
- **Counted frame length:** Set **28 bytes**.
- **Field map:** 1–2 `MW`; 3–5 P1; 6–14 P2; 15–19 P3; 20 P4; 21 P5; 22 P6; 23 P7; 24 P8;
  25–26 P9 fixed `00`; 27 P10; 28 `;`. No P11, no P12, no tag, no tag flag.
- **Manual-documented bytes:** command letters, widths, terminator, P1 range, P2 unit, P3
  offset range, every P4/P5/P6/P8/P10 index and label, P9=`00`.
- **INHERITED-ASSUMED bytes:**
  1. *P7.* Written as a single `0`. The legend prints `P7  00: (Fixed)` — two characters —
     but the chart allots P7 one position. See disagreement 2.
     **Settling capture:** one MW write of a full 28-byte frame followed by an MR read of the
     same channel — if the radio accepts the 28-byte frame and the values land correctly,
     the single-character P7 is confirmed.
  2. *Minus clarifier-direction byte.* Single `-`, as for MT.

### MR — `mr-vectors.golden`

- **Charts used:** MR Read chart (the MR Set chart is an empty grid) and MR Answer chart,
  printed folio 12 (PDF page 13).
- **Counted frame lengths:** Read request **6 bytes**; Answer **28 bytes**.
- **Field map (Answer):** 1–2 `MR`; 3–5 P1; 6–14 P2; 15–19 P3; 20 P4; 21 P5; 22 P6;
  23 P7 (`0: VFO  1: Memory`); 24 P8; 25–26 P9 fixed `00`; 27 P10; 28 `;`. No tag field.
- **Manual-documented bytes:** as for MW, plus both P7 labels.
- **INHERITED-ASSUMED bytes:**
  1. *P7 value in the Answer vectors.* Set to `1` (Memory). The manual documents both
     values but never says which a memory answer carries.
     **Settling capture:** one MR read of any programmed memory channel shows P7 directly.
  2. *The Answer frames themselves are constructed.* Nothing in the manual states what a
     radio returns for a given channel.
     **Settling capture:** one MR read of a channel programmed to known values.
  3. *Minus clarifier-direction byte* (not exercised by the two Answer vectors, which carry
     `+`, but the assumption stands for any minus-direction MR answer).

### MC — `mc-vectors.golden`

- **Chart used:** MC Set chart, printed folio 11 (PDF page 12). Answer chart counted
  independently: same 6-position layout. Read chart: 3 positions (`MC;`).
- **Counted frame lengths:** Set **6 bytes**; Read **3 bytes**; Answer **6 bytes**.
- **Field map:** 1–2 `MC`; 3–5 P1 channel; 6 `;`.
- **Manual-documented bytes:** all of them — the chart and the MC legend fix every byte.
- **INHERITED-ASSUMED bytes:** none.
- Also recorded in that file, as comments: the ID Answer frame exactly as printed
  (`ID0670;`, 7 positions counted, legend `P1   0670: FT-991A`), and the two mode legends
  verbatim (memory commands vs MD).

### EX — `ex-vectors.golden`

- **Charts used:** EX Read chart, printed folio 7 (PDF page 8); menu chart, printed folios
  7–9 (PDF pages 8–10), rows 001–153.
- **Counted frame length:** Read request **6 bytes**. The EX Set and Answer charts print an
  open-ended run (`... P2 P2 ~ P2 ;` over `1 2 3 4 5 6 7 ~ n-1 n`) and have **no countable
  length**; no Set or Answer vector was written, and the 14-byte Answer length quoted in
  that file's comments for menu item 151 is DERIVED (chart run + Digits column = 8), not
  counted.
- **Field map (Read):** 1–2 `EX`; 3–5 P1 menu number; 6 `;`.
- **Largest Digits value in the chart:** **8**, at row 151 PRESET FREQUENCY
  (`00030000 ~ 47000000`), unique in the table. Next largest is 5 (rows 027, 064, 065, 083).
- **Manual-documented bytes:** all of them.
- **INHERITED-ASSUMED bytes:** none for the Read vectors.

## Places where the manual's charts and legends disagree with each other

Recorded, not resolved.

1. **Clarifier direction: legend prints two dashes, chart allows one position.** On MR, MT
   and MW alike (printed folio 12), the P3 legend reads
   `Clarifier Direction +: Plus Shift, --: Minus Shift`. At 1800 dpi the minus token is
   unmistakably **two separate hyphen glyphs with a gap**, not one wide dash. But the chart
   gives P3 exactly **five** positions and the same legend's next line gives the offset as
   `0000 - 9999` — four digits — leaving exactly **one** position for the direction. The
   golden vectors use a single `-`. Which of the two the wire actually wants is undecided
   by the manual.
2. **MW P7: legend prints `00: (Fixed)`, chart gives P7 one position.** In the MW block the
   legend reads `P7  00: (Fixed)` while the Set chart shows P7 occupying position 23 alone,
   with `;` counted at position 28. A two-character P7 cannot fit the counted frame. The MR
   and MT blocks avoid this by giving P7 a single-value legend (`0: VFO  1: Memory`, and
   `Set: 0: (Fixed) / Read: 0: VFO  1: Memory`), so the anomaly is MW-specific.
3. **Mode legend, index 3 and index 7.** The memory commands MR, MT and MW (printed folio
   12) print `3: CW` and `7: CW-R`; the MD command (printed folio 11) prints `3: CW-U` and
   `7: CW-L` for what is otherwise the identical 14-entry list. The IF command (printed
   folio 10) prints the memory-command wording. Two of the three copies agree; the MD copy
   is the odd one.
4. **Slot ranges: MC breaks 001–117 down, the memory commands do not.** MC prints
   `001 - 099: Regular Memory Channel` and `100: P-1L  101: P-1U ~ 116: P-9L  117: P-9U`;
   MR and MT print only `P0/1  001-117 (Memory Channel)` and MW only
   `P1  001-117 (Memory Channel)`. The outer spans match; only MC names the PMS slots.
   Any PMS spelling used in an MR/MT/MW vector name here was borrowed from the MC legend
   and is labelled as such in those files.
5. **Command names differ between the command list and the detail blocks.** Command list
   (printed folio 3) vs detail block heading (printed folios 11–12):
   `MR MEMORY READ` vs `MEMORY CHANNEL READ`; `MW MEMORY WRITE` vs `MEMORY CHANNEL WRITE`;
   `MD MODE` vs `OPERATING MODE`. `MC MEMORY CHANNEL` and
   `MT MEMORY CHANNEL WRITE/TAG` match.
6. **Menu chart row 087.** `087 | RADIO ID | - - - - - - - - - - | -` — the P2 column and
   the Digits column both print dashes instead of a parameter range and a width. The row is
   inside the `001 - 153 (MENU Number)` span the EX legend permits, but no EX frame can be
   built for it from the chart.
7. **Command-list vs detail-block directions: no disagreement found.** For every command
   examined, the two readings agree —
   `MT | MEMORY CHANNEL WRITE/TAG | Set O | Read O | Ans. O | AI X` against a block with all
   three charts filled; `MW | MEMORY WRITE | Set O | Read X | Ans. X | AI X` against a block
   whose Read and Answer charts are printed empty; `MR | MEMORY READ | Set X | Read O |
   Ans. O | AI X` against a block whose Set chart is printed empty;
   `MC | MEMORY CHANNEL | Set O | Read O | Ans. O | AI X` against three filled charts;
   `EX | MENU | Set O | Read O | Ans. O | AI O` against three filled charts; and
   `ID | IDENTIFICATION | Set X | Read O | Ans. O | AI X` against a block whose Set chart is
   printed empty. This item is recorded because the brief asks the question, not because
   anything is in conflict.

## STOP findings

None. Every chart the brief asked for was found, legible at 600 dpi, and countable, and
both independent counts agreed on every one.
