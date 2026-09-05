# Boundary ledger — TS-590SG EX Command Parameter List

**Date:** 05/09/2026

## Source

- File read: `/Users/stuart/coding/ft710-programmer/docs/fixtures-private/manuals/ts590sg_pc_rev3.pdf`
- Title **as printed on the cover (PDF page 1)**: `TS-590S` / `TS-590SG` (blue title panel), beneath it
  `PC CONTROL COMMAND` / `Reference Guide`. Publisher line: `JVCKENWOOD Corporation`.
  Printed date on the cover: `January/30/2019`.
- **Revision:** no revision number ("Rev.3" or similar) is printed anywhere I looked. The cover
  carries only the date above; the back cover (PDF page 35) carries `© 2019 JVCKENWOOD Corporation`
  and the document code `B5A-0316-20/01`. The "Rev.3" in the filename is therefore NOT corroborated
  by anything printed on the pages I read — recorded, not resolved.
- Running head on every chart page: `PC CONTROL COMMAND REFERENCE GUIDE`.

## Chart read

The list headed **`EX Command Parameter List (for TS-590SG)`**. That heading is printed on PDF
page 10, immediately below the last row (`087`) of the earlier, separate list headed
`EX Command Parameter List (for TS-590S)`. The TS-590S list was NOT read beyond confirming where
it stops, so as to locate the start of the TS-590SG list.

### Pages used

| PDF page | Printed folio | Chart rows on the page |
|---|---|---|
| 10 | – 9 – | 000 – 021 (22 rows) |
| 11 | – 10 – | 022 – 069 (48 rows) |
| 12 | – 11 – | 070 – 099 (30 rows) |

The chart begins part-way down PDF page 10 (under its heading) and ends part-way down PDF page 12,
where the table's heavy bottom rule is followed by the `FA / FB` command box. PDF page 9 (folio 8)
and PDF page 13 (folio 12) carry no rows of this chart.

### Method

- Worked only from rendered images. No text-layer extraction, no `pdftotext`, no copy/paste from
  the PDF, no other file, no search, no web access. **Nothing else was consulted** — not the
  TS-590S list in the same document, and not any recollection of other radios.
- Pass 0 (locating): the supplied 300 dpi renders `p-01.png` … `p-35.png` were downsampled to
  1000 px wide and their top 420 px strips appended into three contact sheets, to find which pages
  carry the two EX lists. Full 1000 px page views of PDF pages 8, 10, 11 and 12 then fixed the
  boundaries of the TS-590SG list.
- Pass 1 (reading): PDF pages 10–12 re-rendered at **600 dpi** with
  `pdftoppm -r 600 -f 10 -l 12 -png` (4961 × 7016 px per page). Each page was cropped into
  overlapping horizontal bands 4400 px wide × 1050 px tall and read at roughly 2000 px on screen —
  an effective enlargement of about 2.5× the 300 dpi baseline. At that size `0`/`8`/`6` and
  `l`/`1`/`I` are unambiguous (compare the round-shouldered `0` of `000` against the closed `8` of
  `018`, and the serif-less `1` of `021` against the `l` of `level`).
- Pass 2 (reconciliation, independent): the menu-number column alone (x = 350, w = 260 at 600 dpi)
  and the last grid column alone (x = 4090, w = 560) were re-cropped for each page, sliced
  vertically and tiled side by side, then re-read without reference to the pass-1 notes. Selected
  cells were additionally enlarged 150–300 % (rows 016/017 cells, row 022 Function, row 064 cells,
  rows 094–099 Function).
- **Discrepancies between the two passes: none.** Both passes returned the same 100 numbers in the
  same order and the same four-plus-three-plus-one set of rows reaching the `10 ~` column. No third
  look was needed to settle a disagreement; the extra enlargements listed above were taken to pin
  down spacing and cell wording, not to break a tie.

## Column headers, exactly as printed

Top header band, two rows deep, shaded grey:

- Column 1, spanning both header rows: `Menu` (line 1) / `(P1)` (line 2)
- Column 2, spanning both header rows: `Function`
- Columns 3–13, under a single spanning label `Command Parameter (P5)`:
  `0`, `1`, `2`, `3`, `4`, `5`, `6`, `7`, `8`, `9`, `10 ~`

The last sub-header is printed `10 ~` — the digits, a space, then a tilde. There is no "Over"
column and no eleventh numeric header.

**Group structure:** there is **no group-label column and no group heading of any kind** inside the
chart. The chart is one flat, unbroken numeric sequence. The only text outside the ruled table is
the heading line `EX Command Parameter List (for TS-590SG)` above it on PDF page 10, and the
identical header band repeated at the top of PDF pages 11 and 12 (not counted as rows).

## Totals

- **Total rows: 100** (22 + 48 + 30)
- **Pages: 3** (PDF 10–12; folios 9–11)
- **Lowest menu number: 000. Highest menu number: 099.**
- **Sequence: contiguous.** Every integer 000 – 099 appears exactly once, in ascending order,
  across the three pages and across both page seams (021 → 022, 069 → 070). **No gap and no
  duplicate was found**, so there is no STOP finding of that kind.

## TEXT rows (prose printed across the grid instead of cells)

| Menu | Function | Prose printed across the grid |
|---|---|---|
| 000 | Firmware Version | `Version information (4 ASCII characters) read only` |
| 001 | Power on message | `Power on Message (up to 8 ASCII characters)` |
| 087 | Panel PF A function | one block spanning rows 087–099 (see below) |
| 088 | Panel PF B function | " |
| 089 | RIT Key function | " |
| 090 | XIT Key function | " |
| 091 | CL Key function | " |
| 092 | Front panel MULTI/CH key assignment (exclude CW mode) | " |
| 093 | Front panel MULTI/CH key assignment (CW mode) | " |
| 094 | Mic PF 1 function | " |
| 095 | Mic  PF 2 function | " |
| 096 | Mic  PF 3 function | " |
| 097 | Mic  PF 4 function | " |
| 098 | Mic  PF (DWN) function | " |
| 099 | Mic  PF (UP) function | " |

Rows 087–099 (thirteen rows) share ONE unruled prose block that occupies the whole grid area from
the P5=0 column to the right-hand table edge and from the top of row 087 to the bottom of row 099.
Its text, as printed on two paragraphs:

```
000 ~ 255 (3-digit)
Refer to the TS-590SG instruction manual for the numbers and functions. (When the function is turned
OFF, 255 is used.)
```

That is 15 TEXT rows in all (000, 001, 087–099).

## Rows whose grid reaches a column header of two or more characters

The only such header is `10 ~`. Ten rows print something in it:

| Menu | Function | Highest header used | Cell content |
|---|---|---|---|
| 005 | Beep volume | `10 ~` | `~ 20 (steps of 1)` |
| 006 | Sidetone volume | `10 ~` | `~ 20 (steps of 1)` |
| 007 | Message playback volume | `10 ~` | `~ 20 (steps of 1)` |
| 008 | Voice guide volume | `10 ~` | `~ 20 (steps of 1)` |
| 018 | MULTI/CH control step change for AM (kHz) | `10 ~` | `P5=10: 100` |
| 019 | MULTI/CH control step change for FM (kHz) | `10 ~` | `P5=10: 100` |
| 040 | Side tone/ pitch frequency setting (Hz) | `10 ~` | `up to 1000 (steps of 50)` |
| 042 | Keying weight ratio | `10 ~` | `up to 4.0 (steps of 0.1)` |
| 063 | Voice/ message playback repeat duration (seconds) | `10 ~` | `up to 60 (steps of 1)` |
| 077 | DATA VOX delay | `10 ~` | `up to 100 (steps of 5)` |

Ten rows, confirmed identically by both passes. Every other row's last populated cell is under a
single-digit header.

Note (not a defect, recorded to avoid confusion): row `003 Back light color` prints the literal
value `10` — but it prints it in the cell under the header `9`, not under `10 ~`. Its grid does not
reach a two-character header. Similarly `018`/`019` print the value `50` under header `9` and only
then `P5=10: 100` under `10 ~`.

## Printing defects and oddities (recorded, not resolved)

1. **Duplicated option value, rows 016 and 017.** `016 MULTI/CH control step change for SSB (kHz)`
   and `017 MULTI/CH control step change for CW/ FSK (kHz)` both print `OFF, 0.5, 0.5, 1, 2.5, 5, 10`
   under headers 0–6: the value `0.5` appears twice, under P5=1 AND under P5=2, making the option
   list non-monotonic. Verified at 300 % enlargement on both rows — the two cells are typographically
   identical, neither is `0.25` or `0.05`. **STOP FINDING.** I have not chosen which of the two is
   the intended step.
2. **Row 064 `Split transfer function`** prints unusual cell strings: P5=0 `OFF`, P5=1 `A-T R`
   (with an internal space), P5=2 `A-SUB R` (wrapped over two lines as `A-SUB` / `R`), P5=3 `B`.
   Verified at 300 %. The text stays inside its cell rules — no straddling — but the spacing of
   `A-T R` and `A-SUB R` is as printed and I have not normalised it.
3. **Inconsistent inter-word spacing in Function text.** Several Function cells print a double
   space where one would be expected. Confirmed at 150–300 %:
   - `022` `Temporary variable of the standard/ Extension  memory frequency` (double space between
     `Extension` and `memory`; also a space after `standard/`)
   - `028` `Low Cut/ Low Cut and Width/ Shift  change (SSB)` (double space before `change`) whereas
     the otherwise-parallel `029` prints `Shift change (SSB-DATA)` with one space
   - `057` `TX  hold  when  AT  completes the tuning` (justified line, wide inter-word gaps)
   - `094` prints `Mic PF 1 function` with ONE space, while `095`–`099` print `Mic  PF n function`
     with TWO. The CSV records these verbatim.
4. **No printed revision number** anywhere on the pages inspected (cover, chart pages, back cover),
   although the file is named `..._rev3.pdf`. Recorded above; not resolved.
5. Row `036 Transmit equalizer` prints `C` under P5=6 where the parallel row `037 Receive equalizer`
   prints `FLAT`. Both cells are legible and complete; noted only because the two rows are otherwise
   identical in shape. Not treated as a defect.

No cell was found straddling two columns, no header row was found duplicated within a page (the
header band at the top of PDF pages 11 and 12 is the normal continuation header and was not counted
as a row), and no cell was unreadable.
