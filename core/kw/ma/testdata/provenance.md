# Provenance — Brief G, frame-geometry witness, TS-890S and TS-990S

Derivation date: 08/09/2026. Quarantined evidence agent.
Everything below was read visually off 300 dpi page renders and off the two PDFs themselves.
NO text-layer extraction, no code, no generator, no other file and no other document was
consulted. Every position was COUNTED off the printed numbered ruler above each chart row.

## Source files

| File | SHA-256 |
|---|---|
| `docs/fixtures-private/manuals/ts890s_pc_rev1.pdf` | `e106050c990747234bc9bf6d5b5ca7619ebebdb0ce6f36fe3009a1304e37ea77` |
| `docs/fixtures-private/manuals/ts990s_pc_rev2.pdf` | `ca117fdde0ed993d2c66f7b9dcb57622e2c72864a5a18ac3aedadd96b678ecde` |

Printed identity of both books: title page reads "PC CONTROL COMMAND REFERENCE GUIDE" over
the model name and the date "January/30/2019". **Neither book prints a revision number
anywhere on its cover or first pages** — the "rev1"/"rev2" in the file names is not the
books' own. Printed page N = PDF page N+1 in both books (PDF p.2 carries printed p.1).

## Pages used

**TS-890S (77 PDF pages).** Witnessed: PDF 1 (cover), 4 (AI, printed 3), 17 (CN, printed
16), 25 (EX, printed 24), 35 (FV, printed 34), 36 (ID, printed 35), 42 (MA0, printed 41),
43 (MA2/MA3, printed 42), 52 (OM, printed 51), 67 (TN/TO, printed 66).
Also opened while locating commands: PDF 2, 3, 13–16, 20, 22–24, 34, 50, 51, 66, 68, 70.

**TS-990S (66 PDF pages).** Witnessed: PDF 1 (cover), 4 (AI, printed 3), 16 (CN, printed
15), 22 (EX, printed 21), 23 (EX Command Parameter Lists, printed 22), 32 (FV and ID,
printed 31), 36 (MA0, printed 35), 37 (MA1–MA6, printed 36), 40 (MI, printed 39 — carries
the MA7 reference), 41 (MN, printed 40), 45 (OM, printed 44), 61 (TN/TO, printed 60).
Also opened while locating commands: PDF 2, 14, 20, 31, 38, 39, 43, 58, 59.

Crops kept under `work/`: `ex890-chart.png`, `ex890-legend.png`, `ex990-chart.png`,
`ma0-890-set.png`, `ma0-890-ans.png`, `ma0-890-leg1.png`, `ma0-890-leg2.png`,
`ma0-890-notes.png`, `ma0-990-set.png`, `ma0-990-ans.png`, `ma0-990-legend1.png`,
`ma0-990-legend2.png`, `ma12-890.png`, `cn890-table.png`, `ai890.png`, `ai890-tail.png`.

## Golden files and vector counts

| File | Vectors | Breakdown |
|---|---|---|
| `MA0-890.golden` | 8 | 2 Read, 4 Answer, 2 Set |
| `MA0-990.golden` | 8 | 2 Read, 4 Answer, 2 Set |
| `ID-890.golden` | 2 | 1 Read, 1 Answer |
| `ID-990.golden` | 2 | 1 Read, 1 Answer |
| `FV-890.golden` | 2 | 1 Read, 1 Answer (the book's own worked example) |
| `FV-990.golden` | 2 | 1 Read, 1 Answer (the book's own worked example) |
| `AI-890.golden` | 7 | 3 Set, 1 Read, 3 Answer |
| `AI-990.golden` | 7 | 3 Set, 1 Read, 3 Answer |
| `EX-890.golden` | 4 | 1 Read, 2 Set, 1 Answer |
| `EX-990.golden` | 4 | 1 Read, 2 Set, 1 Answer |
| `MN-990.golden` | 3 | 1 Set, 1 Read, 1 Answer |
| **total** | **49** | |

## The 890S MA0 frame-length rule (derived, not inferred from a range)

The 890S MA0 Set and Answer rulers print five tiers. Tiers 1–4 number positions 1…39
individually, then the ninth column of tier 4 is headed **`40 ~`** and carries `P13`. Tier 5
prints two columns: the first has **no position header at all** and still carries `P13`; the
second is headed with the **letter `x`** and carries `;`.

    frame length = 39 fixed positions + N name characters + 1 terminator = 40 + N
    terminator's position x = 40 + N,  for 0 <= N <= 10  (frames of 40 … 50 bytes)

The name is therefore **not padded**. Counted instances in the goldens: N=10 → 50 bytes;
N=0 → 40 bytes; N=3 → 43 bytes; N=6 → 46 bytes.

The same `n ~` / `x` device is used everywhere in the 890S book for a variable-length tail:
MA2 (printed p.42, headers `1…7`, `8 ~`, `x`), CK7 (printed p.14, `1…4`, `5 ~`, `x`), CM4
and CM5 (printed pp.15–16, `1…5`, `6 ~`, `x`), EX (printed p.24, `1…8`, `9 ~`, `x`).

The 990S never does this. Its MA0 is a **fixed 57** with the terminator at the numbered
position 57, its EX is a **fixed 24** with the terminator at the numbered position 24, and
even its variable-looking CK7 numbers its columns (`5 ~ 55`, `56`, `57`, `58`, `59`).

## FINDINGS (recorded, never resolved)

**F1 — 890S: the terminator's printed position is a letter, not a number.**
Wherever a variable-length parameter precedes it, the 890S prints `x` as the terminator's
position header (MA0 p.41, MA2 p.42, EX p.24, CK7 p.14, CM4/CM5 pp.15–16). Crops
`work/ma0-890-set.png`, `work/ex890-chart.png`.

**F2 — 890S MA0: a parameter label printed under an EMPTY position header.**
In the last tier of the MA0 Set and Answer charts the first column carries `P13` with
**nothing** printed in its header cell, while the adjacent column is headed `x` and carries
`;`. Crops `work/ma0-890-set.png`, `work/ma0-890-ans.png`.

**F3 — 890S CN: a tone chart whose index column is headed `P2` while the command carries
only `P1`.** Printed p.16: the frame is `C N P1 P1 ;` (Set and Answer, `;` at position 5),
the parameter heading reads `P1 (CTCSS frequency)`, and the frequency table's index column
is headed **`P2`** in all four column pairs. The 990S CN (printed p.15) really does have
`P1` = Main/Sub Band and `P2` = frequency, and its table is correctly headed `P2` — the
890S table carries the 990S numbering into a command that has no P2. Crop
`work/cn890-table.png`. Consequence: 890S MA0 P7 says "Refer to the P1 value of the CN
command", which agrees with CN's parameter heading and disagrees with CN's own table head.

**F4 — spelling and content differ between the two tone tables of the same book, and
between books.** 890S CN (p.16) spells row 99 "to default"; 890S TN (p.66) spells it
"To default"; 990S TN (p.60) spells it "Default"; 990S CN (p.15) spells it "to default".
890S TN prints index 50 as `1750.0`; 990S TN prints index 50 as `1750`. Neither CN table
has an index 50 at all, though both TN tables do.

**F5 — 990S: a lockout encoded two ways in two places of the same book.**
MA0 P17 (printed p.35) prints `1: Scan Lockout OFF` / `2: Scan Lockout ON`.
MA3 (printed p.36) prints `0: Scan Lockout OFF` / `1: Scan Lockout ON`.
Crop `work/ma0-990-legend2.png`. The 890S is value-consistent across the same two places
(MA0 P12 `0: Lockout OFF` / `1: Lockout ON`; MA3 P2 `0: Scan Lockout OFF` /
`1: Scan Lockout ON`) but words them differently.

**F6 — 990S MA0: a mode legend cited to two different parameters of the same other command.**
P4 reads "Mode information for frequency 1 (refer to the **P2** value of the OM command)".
P10 reads "Mode information for frequency 2 (refer to the **P1** value of the OM command)".
OM (printed p.44) has `P1` = `0: Main Band / 1: Sub Band` and `P2` = the mode list, so the
P10 citation points at the band selector. Crop `work/ma0-990-legend1.png`.

**F7 — 990S: a note about a command that does not exist in that book.**
MI (printed p.39) ends: "With the section defined memory channel, the start and end
frequency are stored as the same frequency. **The end frequency is set using the MA7
command.**" The book's MA block runs MA0 … MA6 only (printed pp.35–36), and MA6 is
"Section Defined Memory Channel End Frequency". There is no MA7 anywhere in the book.

**F8 — 890S MA0: the blank-channel note stops one parameter short of the name.**
"When reading a blank channel, parameters P2 to P12 becomes blank." P13 is the channel name
and the only variable-length field, so the book does **not** determine the length of a blank
channel's Answer frame. The 990S note covers its whole frame ("parameters P2 to P18 becomes
blank") because its name field is fixed-width. No vector for a blank channel is emitted in
either golden.

**F9 — EX: the same command has a different frame LENGTH in the two books.**
Read frames agree exactly (8 positions, `;` at 8, fields `E X P1 P2 P2 P3 P3`).
Set/Answer: 890S is `E X P1 P2 P2 P3 P3 P4` then `P5` at `9 ~` and `;` at `x` (variable);
990S is the same eight fields then `P5` at positions 9–23 and `;` at the numbered position
**24** (fixed 24). The **field order is identical**; only the length and the terminator's
printed form differ. Both books print the SAME variable-length note for P5 ("A power-on
message can vary in length from 0 to 15 characters. Screen saver text can vary in length
from 0 to 10 characters."), so the 990S chart and the 990S legend disagree with each other.

**F10 — MA0: the two books' frames agree only on positions 1–6; every position from 7 to
the end differs.** Position by position:

| pos | TS-890S | TS-990S |
|---|---|---|
| 1–3 | `M` `A` `0` | `M` `A` `0` — same |
| 4–6 | P1 channel number | P1 channel number — same |
| 7 | P2 frequency digit 1 | P2 memory channel type |
| 8–17 | P2 frequency digits 2–11 | P3 frequency-1 digits 1–10 |
| 18 | P3 mode | P3 frequency-1 digit 11 |
| 19 | P4 FM normal/narrow | P4 mode for frequency 1 |
| 20 | P5 FM tone type | P5 FM wide/narrow for frequency 1 |
| 21 | P6 tone-frequency digit 1 | P6 FM tone function for frequency 1 |
| 22 | P6 tone-frequency digit 2 | P7 tone-frequency-1 digit 1 |
| 23 | P7 CTCSS digit 1 | P7 tone-frequency-1 digit 2 |
| 24 | P7 CTCSS digit 2 | P8 CTCSS-1 digit 1 |
| 25 | P8 split-TX frequency digit 1 | P8 CTCSS-1 digit 2 |
| 26–35 | P8 split-TX frequency digits 2–11 | P9 frequency-2 digits 1–10 |
| 36 | P9 split-TX mode | P9 frequency-2 digit 11 |
| 37 | P10 split-TX FM normal/narrow | P10 mode for frequency 2 |
| 38 | P11 split information | P11 FM wide/narrow for frequency 2 |
| 39 | P12 scan lockout | P12 FM tone function for frequency 2 |
| 40 | P13 name char 1, **or** `;` when the name is empty | P13 tone-frequency-2 digit 1 |
| 41 | P13 name char 2 | P13 tone-frequency-2 digit 2 |
| 42–43 | P13 name chars 3–4 | P14 CTCSS-2 digits 1–2 |
| 44 | P13 name char 5 | P15 simplex/split |
| 45 | P13 name char 6 | P16 dual reception |
| 46 | P13 name char 7 | P17 scan lockout |
| 47–49 | P13 name chars 8–10 | P18 name chars 1–3 |
| 50 | `;` at most (`x` = 50 when N = 10) | P18 name char 4 |
| 51–56 | *(does not exist)* | P18 name chars 5–10 |
| 57 | *(does not exist)* | `;` |

Lengths: 890S Set/Answer = 40 + N (40…50); 990S Set/Answer = 57 fixed. Read frames agree
(7 positions, `M A 0 P1 P1 P1 ;`).

Semantics differ as well as geometry: the 890S second frequency is a **split transmission**
frequency (P8/P9/P10 with P11 simplex/split); the 990S second frequency is **frequency 2**
of a dual memory channel, with its own tone block (P12/P13/P14) and separate P15 split and
P16 dual-reception flags. The 990S also has P2 (memory channel type) with no 890S
counterpart, and the 890S has no dual-reception concept at all.

**F11 — 990S MA0 P18 is described as "digits" and given a fixed width for an "up to"
value.** "Channel Name (Up to 10 digits.)" for a character field, printed against a
fixed 10-position field (47–56) with the terminator nailed to 57. The pad character is
never stated. The 890S says "Up to 10 characters" and lets the frame shrink. This is the
single largest INHERITED-ASSUMED item in the 990S golden.

**F12 — the ID legends are shaped differently in the two books.**
890S (printed p.35) prints `024:  TS-890S` — value and model. 990S (printed p.31) prints
only `022` with **no model name at all**. Each book prints exactly one entry.

**F13 — EX legend wording differs between the two books for the same parameters.**
890S: `P1 (Menu type number)`, `P2 (Category number)`, `P3 (Item number)`,
`P4 (Configuration classification)`, `9: Initial value setting (setting command only)`.
990S: `P1` (no name), `00 ~ 99: Category Number`, `00 ~ 99: Entry Number`,
`P4 (Configuration Classification)`, `9: Initializing`. The 990S P5 legend also carries a
line the 890S does not: "Frequency settings use 8 digits (blank digits must be entered as
0)." and "Entering a value larger than the size limit causes an error to occur."

**F14 — the channel-number wording differs within the 990S book.**
MA0 (printed p.35): "Channels P0 ~ P9 are represented as 100 ~ 109 and channels E0 ~ E9 are
represented as 110 ~ 119." MA2 (printed p.36): "Channel numbers P00 ~ P09 are represented by
100 ~ 109." (the E range is omitted entirely). MI (printed p.39) and MN (printed p.40):
"Channel numbers P00 ~ P09 are represented by 100 ~ 109. Channel numbers E00 ~ E09 are
represented by 110 ~ 119." The 890S MA0/MA2/MA3 all use the "P0 ~ P9" / "E0 ~ E9" form.

**F15 — neither book's MA0 box carries the unregistered-channel note or the AI note.**
The brief asked for them verbatim; they are not in MA0 in either book. They live on the
sibling commands. 890S MA2/MA3 (printed p.42): "Setting an unassigned channel causes an
error." and "When the AI function is ON, a response is provided by the MA0 command."
990S MA1 (printed p.36): "When the AI function is ON, a response can consist of the MA0
command."; 990S MA2/MA3/MA6: "Setting an unassigned channel causes an error." / "You cannot
set an unassigned channel." with "When the AI function is ON, a response is provided by the
MA0 command."

**F16 — a chart position whose printed VALUE is an ASCII space.**
EX P4 in both books: "Space: Normal Configuration" and "Response is always a space", at
counted position 8. The 890S MA2 P2 and CK7 P1, and the 990S MA2 P2, do the same
("Always a space"). Not a defect, but a geometry hazard: a real 0x20 byte sits inside the
frame and is easily lost.

**Explicitly NOT found** (the brief asked; the answer is negative in both books):
no chart cell prints a terminator or punctuation where a parameter is expected; no
full-width or doubled terminator glyph appears in any chart witnessed; every terminator
cell holds a single ASCII `;`.

## INHERITED-ASSUMED summary (all files)

* **890S MA0** — P6/P7 tone and CTCSS indices when P5 (tone type) is 0/OFF: assumed `00`
  (lowest legal printed index, 67.0 Hz). Channel-name character set: not printed, assumed
  uppercase ASCII and digits. "Empty name" assumed to mean zero name bytes (x = 40). All
  field VALUES are choices; only widths and value sets come from the book.
* **990S MA0** — the pad character for P18 when the name is shorter than 10: assumed ASCII
  space, name left-aligned. P7/P8 and P13/P14 when the tone function is OFF: assumed `00`.
  P2 (stated to be "ignored. Enter a dummy value") carried as a plausible Answer value.
  Channel-name character set: not printed, assumed uppercase ASCII and digits.
* **890S EX** — P2 `00`, P3 `00`, P5 `000` are arbitrary in-range choices (the 890S menu
  tables were not read for this witness); whether P5 is still sent when P4 is `9` is not
  stated and is assumed present.
* **990S EX** — the 12 pad positions of the 15-wide P5 field when the value is 3 digits:
  assumed ASCII spaces, value left-aligned. Whether the 24-position geometry survives when
  P4 is `9` is not stated and is assumed unchanged. P1/P2/P3/P5 themselves are read off the
  book's own EX Command Parameter Lists (printed p.22) and are manual-determined.
* **990S MN** — P1 `0` and P2 `000` are arbitrary in-range choices.
* **ID, FV, AI (both books)** — no assumed bytes. The FV Answer vectors are the books' own
  printed worked examples.
