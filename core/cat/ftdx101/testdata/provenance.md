# Provenance — hand-derived golden CAT frames (FTDX101MP / FTDX101D)

## 1. Source document

| Item | Value |
| --- | --- |
| Title as printed on cover | **FTDX101MP / FTDX101D — CAT Operation Reference Manual** (cover sets "FTDX101MP" and "FTDX101D" on two lines above the title line) |
| Publisher as printed | YAESU MUSEN CO., LTD. |
| Revision code as printed | **2308-L** (bottom right of the last printed page, PDF page 26, beside the "Copyright 2023" notice) |
| File | `docs/fixtures-private/manuals/ftdx101_cat_2308-L.pdf` |
| Extent | 26 PDF pages; A4; encrypted with `copy:no` |

This single PDF was the **only** thing consulted. No source code, no
generator, no other file in this or any repository, no other manual or
transcription, and no web resource was opened. No directory was listed to
discover what else exists.

## 2. Method

1. **Rasterise.** `pdftoppm -r 100 -png` over the whole document to locate
   the four subject sections and Table 2 by reading the rendered pages as
   images; then `pdftoppm -r 400 -png` for the pages actually used.
2. **Crop and enlarge.** ImageMagick `magick … -crop … +repage -resize
   130-600%` to isolate each position grid, each parameter legend and each
   Table 2 row band, then read the crops as images.
3. **Two passes per chart.** A first pass at 100 dpi to confirm the
   two-letter section heading and its neighbours (adjacent sections'
   titles resemble each other — MT "MEMORY CHANNEL WRITE/TAG" versus MW
   "MEMORY CHANNEL WRITE" in particular), a second pass at 400 dpi to read
   positions and tokens. Header rows were treated as numbering the token
   row **below** them, per the charts' printed layout.
4. **No text extraction.** `pdftotext` and every other text-layer tool
   were avoided entirely; the printed chart is the evidence. (The PDF is
   copy-protected in any case.)

A scratch render directory that already contained files from earlier,
unrelated work was **not** used: a fresh `_render/` directory beneath this
output directory was created so that no pre-existing artefact could be
mistaken for evidence.

Derivation date: **06/08/2026**.

## 3. Pages used

| Subject | Section heading confirmed in render | Printed page | PDF page |
| --- | --- | --- | --- |
| Command summary (orientation only) | (Command / Function / Set / Read / Ans. / AI table) | 5 | 6 |
| **EX** | `EX  MENU` | 9 | 10 |
| **Table 2 (MENU Chart)** | `Table 2 (MENU Chart)` | 10, 11, 12 | 11, 12, 13 |
| **MR** | `MR  MEMORY CHANNEL READ` | 16 | 17 |
| **MT** | `MT  MEMORY CHANNEL WRITE/TAG` | 16 | 17 |
| **MW** | `MW  MEMORY CHANNEL WRITE` | 17 | 18 |

## 4. The ten vectors

### `mt-vectors.golden` — 3 Set-direction MT frames (41 bytes each)

Layout read from the **Answer** grid; the Set direction's `P7` value
(`0`, "(Fixed)") taken from the legend line
`P7 Set: 0: (Fixed) / Read: 0: VFO 1: Memory`. The Set grid was also
legible at 400 dpi and **agrees with the Answer grid position-for-position
across all 41 positions** — that cross-check is recorded in the file's
header.

| Name | Frame |
| --- | --- |
| `tag_full_width_12` | `MT007014250000+0000002000000SYNTHTAG0001;` |
| `tag_short_space_padded` | `MT012007100000+0000001000000TESTTAG␣␣␣␣␣;` |
| `tag_cleared_all_pad` | `MT023021300000+0000003000000␣␣␣␣␣␣␣␣␣␣␣␣;` |

(`␣` marks a literal space **in this table only**; the `.golden` file
carries real spaces. Those trailing spaces are frame content and must not
be stripped.)

Tag payloads `SYNTHTAG0001` and `TESTTAG` are **synthetic strings invented
for this fixture**. They are not real amateur callsigns and are not
intended to resemble any issued callsign.

Field map (positions 1-based): `M`(1) `T`(2) `P1`(3-5) `P2`(6-14)
`P3`(15-19) `P4`(20) `P5`(21) `P6`(22) `P7`(23) `P8`(24) `P9`(25-26)
`P10`(27) `P11`(28) `P12`(29-40) `;`(41).

### `mr-vectors.golden` — 1 Read + 1 Answer

| Name | Frame | Length |
| --- | --- | --- |
| `read_request_ch007` | `MR007;` | 6 |
| `answer_ch007_usb_14m250` | `MR007014250000+000000210000;` | 28 |

Read map: `M`(1) `R`(2) `P0`(3-5) `;`(6).
Answer map: `M`(1) `R`(2) `P1`(3-5) `P2`(6-14) `P3`(15-19) `P4`(20)
`P5`(21) `P6`(22) `P7`(23) `P8`(24) `P9`(25-26) `P10`(27) `;`(28).

### `mw-vectors.golden` — 2 Set-direction MW frames (28 bytes each)

| Name | Frame |
| --- | --- |
| `write_ch003_datau_7m074_simplex` | `MW003007074000+000000C00000;` |
| `write_ch045_fm_51m000_minus_shift` | `MW045051000000+010011401002;` |

The pair differ in channel (003 / 045), frequency (7.074000 /
51.000000 MHz), mode (`C` DATA-U / `4` FM), RX and TX clarifier (off,off /
on,on), clarifier offset (0000 / 0100), CTCSS (`0` OFF / `1` ENC/DEC) and
repeater shift (`0` Simplex / `2` Minus Shift).

Set map: `M`(1) `W`(2) `P1`(3-5) `P2`(6-14) `P3`(15-19) `P4`(20) `P5`(21)
`P6`(22) `P7`(23) `P8`(24) `P9`(25-26) `P10`(27) `;`(28).

### `ex-vectors.golden` — 3 Read-direction EX frames (9 bytes each)

| Name | Frame | Table 2 row (group / subgroup / item / function / printed page) |
| --- | --- | --- |
| `read_radio_ssb_tx_bpf_sel` | `EX010110;` | 01 (RADIO SETTING) / 01 (MODE SSB) / 10 / TX BPF SEL / printed p.10 = PDF 11 |
| `read_cw_keyer_cw_weight` | `EX020205;` | 02 (CW SETTING) / 02 (KEYER) / 05 / CW WEIGHT / printed p.11 = PDF 12 |
| `read_display_scope_ctr` | `EX040202;` | 04 (DISPLAY SETTING) / 02 (SCOPE) / 02 / SCOPE CTR / printed p.12 = PDF 13 |

Three different P1 menu groups, as required. Read map: `E`(1) `X`(2)
`P1`(3-4) `P2`(5-6) `P3`(7-8) `;`(9). Every byte of all three EX frames is
manual-documented; the only free choice was which rows to address.

## 5. Consolidated assumption register

Everything below is an **INHERITED ASSUMPTION** — a choice of mine, or an
inference the manual does not state. Everything not listed here is
manual-documented and cited byte-by-byte in the relevant `.golden` header.

| ID | Files | Assumption | Lifting capture on a real radio |
| --- | --- | --- | --- |
| A1 | mt | **Pad byte for a short TAG is 0x20 (space), tag left-justified in P12.** The manual states only "TAG Characters (up to 12 characters) (ASCII)" and never names a pad byte or a justification. | Capture an MT or MR **Answer** for a channel whose stored tag is **shorter than 12 characters**; positions 29-40 of that Answer reveal both the pad byte and the justification. |
| A2 | mt, mr, mw | **Frequency is zero-padded on the left to fill all 9 positions.** Inferred from the fixed field width; the legend says only "Frequency (Hz)". | Capture an MR Answer for any HF channel and inspect positions 6-14. |
| A3 | mt, mr, mw | **`+` is the direction character paired with a zero clarifier offset.** The legend documents `+` and minus as directions but not which accompanies `0000`. | Capture an MR Answer for a channel whose clarifier is off / at zero offset. |
| A4 | mt, mr, mw | **Choice of channel numbers, frequencies and modes** (007/012/023/003/045; 14.250000, 7.100000, 21.300000, 7.074000, 51.000000 MHz; USB, LSB, CW-U, DATA-U, FM). Free choices inside documented ranges. | Not applicable — vector choices, not claims about the radio. |
| A5 | mt | **TAG character set.** P12 is documented as "(ASCII)" but the accepted subset is not enumerated; use of A-Z, 0-9 and an embedded space is assumed legal. | MT Set a tag containing the questioned character, then read it back with MT or MR and compare. |
| A6 | mt | **A cleared tag is expressed as 12 pad bytes.** The manual describes no "clear the tag" encoding. | Capture an Answer for a channel that has no tag set. |
| A7 | mr | **MR Answer's `P1` uses the same channel vocabulary as MR Read's `P0`.** The MR legend prints no P1 line (anomaly N1); the identity is inherited from MT's `P0/1` line. | Capture an MR Answer for a known channel and read positions 3-5. |
| A8 | mr | **An Answer echoes the channel number that was requested.** Not stated. | An MR Read/Answer round trip. |
| A9 | mw | **Clarifier offset accepts 10 Hz granularity (0100 is honoured, not rounded).** The range `0000 - 9990 (Hz)` implies a 10 Hz step but no step is printed. | MW Set 0100, then MR Read the channel and compare positions 16-19. |
| A10 | mw | **Repeater shift `2` (Minus Shift) is accepted without a shift-magnitude field.** The MW frame carries no offset-size parameter and the manual does not say where the magnitude comes from. | MW-write an FM repeater channel, MR read it back, and inspect the radio's displayed transmit frequency. |
| A11 | mw | **CTCSS ENC/DEC (`P8 = 1`) can be written by MW without a tone frequency**, for which MW has no field. | MW write with `P8 = 1`, then read the channel's tone on the radio or via the CN command. |
| A12 | ex | **P1/P2/P3 are written zero-padded to two positions** (`05`, not `5`). Implied by the two-position fields and the legend's two-digit ranges, but not stated in words. | An EX Read whose Answer echoes P1/P2/P3 in positions 3-8. |
| A13 | ex | **A Read request carries no P4 and ends at position 9.** Read off the Read grid (positions 10+ blank), not stated in prose. | Send `EX010110;` to a real radio and confirm a well-formed Answer rather than a `?;` error reply. |
| A14 | ex | **Every Table 2 row is addressable on both FTDX101D and FTDX101MP.** Table 2 marks some P4 *ranges* as model-specific but does not say whether any *row* is absent on a model. | Issue each Read on each model and compare the replies. |

## 6. Hardware status

**UNVERIFIED — for all ten vectors in all four files.** No frame has been
sent to, or captured from, a real FTDX101MP or FTDX101D. Every byte is
paper-derived from the position charts and legends of the manual named
above.

## 7. Anomalies observed in the printed charts

Recorded, not resolved.

| ID | Where | Observation |
| --- | --- | --- |
| N1 | MR legend, printed p.16 | The legend has **no `P1` line**. Its first line is labelled `P0` and covers `001-099 (Memory Channel), P1L -P9U (PMS), 5xx (5MHz BAND), EMG (EMERGENCY CH)`, yet the Answer grid labels positions 3-5 `P1`. The sibling MT command's equivalent line is labelled `P0/1`, suggesting MR's should read `P0/1` too. |
| N2 | MR, MT and MW legends | The minus clarifier direction is printed as **two hyphen glyphs**: `--: Minus Shift`. Because `P3` is only 5 positions wide and the offset alone is documented as 4 digits, the direction can occupy just one position, so the printed `--` is taken to be a typographic rendering rather than two bytes. **No vector in this set uses a minus clarifier direction**, so no delivered byte depends on the reading. This is the one place where I would otherwise have had to guess a width, so it is flagged rather than resolved. |
| N3 | MW section, printed p.17 | MW's **Read and Answer grids are printed with numbered headers but entirely empty token rows**. This is consistent with the command summary on printed p.5, which marks MW as Set `O` and Read / Ans. / AI `X`. MW therefore documents no Read or Answer frame at all — which is why `mw-vectors.golden` contains only Set-direction vectors. |
| N4 | MW legend, printed p.17 | MW's `P1` line lists only `001-099 (Memory Channel), P1L -P9U (PMS)`, omitting the `5xx (5MHz BAND)` and `EMG (EMERGENCY CH)` classes that the MR and MT lines carry. Whether MW genuinely cannot address those classes, or the line is merely abbreviated, is not stated. |
| N5 | EX legend vs Table 2 | The EX legend gives `P1 : 01 - 05`, but Table 2 as printed contains only **four** P1 groups — 01 (RADIO SETTING), 02 (CW SETTING), 03 (OPERATION SETTING), 04 (DISPLAY SETTING). No `P1 = 05` group appears anywhere across printed pages 10-12. None of the EX vectors uses 05. |
| N6 | EX Set and Answer grids, printed p.9 | The **tenth header cell is printed blank** in the Set and Answer rows (headers read `1 … 9, [blank], nn, **`), whereas the Read row's tenth header cell reads `10`. The Set and Answer token rows place their `~` continuation under that blank cell, so it appears deliberate, but it is inconsistent with the Read row. It affects no byte of the Read-direction vectors delivered here. |
| N7 | MT section, printed p.16 | Contrary to expectation, MT's **Set grid did not overprint its legend** at 400 dpi — it was fully legible and agrees with the Answer grid position-for-position across all 41 positions. The interleaving is a low-resolution rendering artefact, not a defect in the printed page. Recorded because the Set-from-Answer derivation was performed as instructed and this cross-check is what confirms it. |

## 8. Files in this directory

- `mt-vectors.golden` (3 vectors)
- `mr-vectors.golden` (2 vectors)
- `mw-vectors.golden` (2 vectors)
- `ex-vectors.golden` (3 vectors)
- `provenance.md` (this file)
- `_render/` — the intermediate PNG renders and crops the derivation was
  read from, retained so the reading can be re-checked without
  re-rasterising.
