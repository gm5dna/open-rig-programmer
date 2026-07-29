# Provenance — FTDX10 CAT wire-frame test vectors (m9c4-G)

**Derivation date:** 29/07/2026

**Manual:** Yaesu *FTDX10 CAT Operation Reference Manual*, **revision 2308-F**
(revision code printed on the back cover, PDF page 25; PDF title metadata
"FTDX10 CAT Operation Reference Manual", Yaesu Musen Co., Ltd., 25 pages).
File: `docs/fixtures-private/manuals/ftdx10_cat_2308-F.pdf`.

**What was consulted — complete list:**

* That one PDF, and nothing else. Pages were rendered to images and read
  visually; the position charts were additionally re-rendered at 300 dpi and
  cropped so that individual numbered byte positions could be counted rather
  than inferred.
* Pages actually read: PDF 1–6, 9–14, 17, 18, 25 (printed cover, 1–5, 8–13,
  16, 17, back cover).

**NO code, no generator, no library, no existing fixture, no other document and
no other file of any kind was consulted.** No source tree was read, no directory
was listed, no filesystem search was performed. Nothing was copied from an
existing implementation of this protocol.

**Hardware status: UNVERIFIED for every vector in all four files.** Not one
frame in `mt-vectors.golden`, `mw-vectors.golden`, `mr-vectors.golden` or
`ex-vectors.golden` has been transmitted to, or captured from, a real FTDX10.
They are paper derivations only.

---

## Charts used, per vector class

| Vector class | File | Printed page (PDF page) | Chart used |
|---|---|---|---|
| MT — MEMORY CHANNEL WRITE/TAG | `mt-vectors.golden` | 16 (PDF 17) | **ANSWER chart** for the byte layout; **P7 value from the Set line of the legend** |
| MW — MEMORY CHANNEL WRITE | `mw-vectors.golden` | 17 (PDF 18) | Set chart (the only populated direction) |
| MR — MEMORY CHANNEL READ | `mr-vectors.golden` | 16 (PDF 17) | Read chart + Answer chart |
| EX — MENU | `ex-vectors.golden` | 9 (PDF 10) for the frame; Table 2 (MENU Chart) on 10–12 (PDF 11–13) for the addresses | Read chart |

Supporting pages used for cross-checks: printed page 4 (PDF 5) "Control
Command" — terminator, parameter-width rules, the worked `FA014250000;`
example; printed page 5 (PDF 6) "Control Command List" — which directions each
of MR/MT/MW supports.

### Note on the MT chart choice

The task's expectation is confirmed: at normal rendering the MT **Set** chart's
lower rows sit against the parameter legend and are hard to read with
confidence. The **Answer** chart, lower on the same printed page, is printed
cleanly with all five numbered rows (1–10, 11–20, 21–30, 31–40, 41–50) legible,
so **the Answer chart is what the field layout was counted from**. As a check,
the Set chart was re-rendered at 300 dpi and is byte-for-byte identical in
layout to the Answer chart. The one direction-specific value, P7, was taken not
from either grid but from the legend line
`P7  Set: 0: (Fixed) / Read: 0: VFO  1: Memory` — so the MT vectors, being Set
frames, carry **P7 = `0`**.

---

## MT — `mt-vectors.golden` (41 bytes per frame)

Counted directly off the numbered positions of the MT Answer chart:

```
1-2 "MT" | 3-5 P1 | 6-14 P2 | 15-19 P3 | 20 P4 | 21 P5 | 22 P6 | 23 P7
| 24 P8 | 25-26 P9 | 27 P10 | 28 P11 | 29-40 P12 | 41 ";"
2 + 3 + 9 + 5 + 1 + 1 + 1 + 1 + 1 + 2 + 1 + 1 + 12 + 1 = 41
```

### Manual-documented bytes

| Field | Value used | Manual authority |
|---|---|---|
| positions 1–2 | `MT` | Chart cells; opcode is 2 alphabetical characters (printed p.4) |
| P1, 3 positions | `001` / `017` / `099` | Legend "P0/1 001-099 (Memory Channel), P1L-P9U (PMS), 5xx (5MHz BAND), EMG"; printed with leading zeros |
| P2, 9 positions | `007100000` | Legend "Frequency (Hz)"; width fixed by chart at 9. Left zero-fill corroborated by the manual's own worked example on printed p.4, `FA014250000;` for 14.250000 MHz |
| P3 direction, 1 position | `+` | Legend "Clarifier Direction +: Plus Shift" |
| P3 offset, 4 positions | `0000` | Legend "Clarifier Offset: 0000 - 9990 (Hz)" |
| P4 | `0` | Legend "0: RX CLAR \"OFF\"" |
| P5 | `0` | Legend "0: TX CLAR \"OFF\"" |
| P6 | `1` | Legend "MODE 1: LSB" |
| P7 | `0` | Legend "P7 Set: 0: (Fixed)" — Set direction |
| P8 | `0` | Legend "0: CTCSS \"OFF\"" |
| P9, 2 positions | `00` | Legend "P9 00: (Fixed)"; occupies chart positions 25 and 26 |
| P10 | `0` | Legend "0: Simplex" |
| P11 | `0` | Legend "P11 0: (Fixed)" |
| P12 field **width** | 12 positions | Chart positions 29–40; legend "TAG Characters (up to 12 characters) (ASCII)" |
| position 41 | `;` | Chart cell; terminator rule, printed p.4 |

### INHERITED ASSUMPTIONS

**A1 — The TAG pad byte is ASCII SPACE (0x20).** *Applies to vectors (b) and (c).*
What I assumed: that a tag shorter than 12 characters is right-padded to the
full 12 positions with 0x20, and that a cleared tag is 12 × 0x20.
Why: the chart fixes the field at 12 positions and the frame has no length
prefix, so *something* must occupy the unused positions — but the manual never
names a pad byte for P12. The nearest text (printed p.4) is about *inapplicable*
parameters — "the parameter digits should be filled using any character except
the ASCII control codes (00 to 1Fh) and the terminator (;)" — which permits
space but equally permits `0`, `*` or anything else printable. Space is
inherited convention, not manual fact.
**To lift it, a hardware capture would need to show:** an `MT` **Answer**
(or `MT`+3-digit read) from a radio holding a tag of fewer than 12 characters,
with positions 29–40 dumped as raw bytes — the trailing bytes are the radio's
own pad character. A second capture of a channel with a *cleared* tag would
confirm the all-pad case. Additionally, a write-then-read round trip (send the
vector-(b) frame, then `MT017;`) that returns the identical 12 bytes would show
the radio accepts and preserves 0x20 padding rather than normalising it.

**A2 — Padding is trailing, not leading, and not centred.**
What I assumed: `GB3TST` + 6 pad bytes, i.e. content left-justified in the field.
Why: universal convention for text fields; the manual says nothing about
justification.
**To lift it:** the same capture as A1 — whether the non-pad bytes appear at
positions 29… or at the end of the field settles it.

**A3 — Printable-ASCII tag content including `-`.**
What I assumed: `SCOTLAND-40M` is an acceptable P12 payload, i.e. the hyphen and
digits are inside whatever subset of ASCII the radio accepts in a tag.
Why: the legend says only "(ASCII)" and gives no character-set restriction.
**To lift it:** write the vector-(a) frame and read the channel back; a capture
showing the same 12 bytes returned proves the character set is accepted, whereas
substituted or dropped characters would expose a narrower alphabet.

**A4 — Upper-case opcode.**
Strictly a choice, not an assumption: printed p.4 explicitly permits "either
lower or upper case characters" for the 2-letter command. Recorded here only so
that a future byte-identity comparison against a lower-case generator is not
mistaken for a derivation error.

**A5 — Payload semantics are illustrative.**
The 7.100000 MHz / LSB / simplex / no-clarifier / CTCSS-off combination is a
plausible 40 m memory, chosen so that vectors (a), (b) and (c) differ *only* in
slot number and TAG field. The manual does not assert that any particular radio
holds these values; the vectors test encoding, not radio state.

**A6 — TAG payload strings are synthetic.**
`SCOTLAND-40M` and `GB3TST` are invented placeholders chosen purely to exercise
the 12-position field at full width and at 6 characters respectively. Neither is
derived from the manual, and neither is a real amateur callsign — committed
fixtures must not carry one. `GB3TST` replaced an earlier draft value at the
coordinator's instruction; it is the same length (6 characters), so the padding
structure derived from the chart is preserved byte-for-byte as 6 tag bytes plus
6 pad bytes, and assumptions A1 and A2 are tested exactly as before.

---

## MW — `mw-vectors.golden` (28 bytes per frame)

```
1-2 "MW" | 3-5 P1 | 6-14 P2 | 15-19 P3 | 20 P4 | 21 P5 | 22 P6 | 23 P7
| 24 P8 | 25-26 P9 | 27 P10 | 28 ";"
2 + 3 + 9 + 5 + 1 + 1 + 1 + 1 + 1 + 2 + 1 + 1 = 28
```

MW has **no** P11 and **no** P12 — the chart's third row ends `P5 P6 P7 P8 P9 P9
P10 ;` at position 28, with positions 29 and 30 empty. This is the concrete
difference from MT and was counted twice at 300 dpi.

### Manual-documented bytes

Same legend basis as MT for P1–P6 and P8–P10, plus:

| Field | Value used | Manual authority |
|---|---|---|
| P1 | `003`, `021` | MW legend "001-099 (Memory Channel), P1L-P9U (PMS)" — note MW's legend, unlike MT's and MR's, does **not** list 5xx or EMG |
| P2 | `014250000`, `029600000` | Legend "Frequency (Hz)", 9 positions |
| P6 | `2` (USB), `4` (FM) | Legend "2: USB", "4: FM" |
| P7 | `0` | MW legend "P7 0: (Fixed)" — MW has no Read vocabulary at all |
| P8 | `0`, `1` | Legend "0: CTCSS \"OFF\"", "1: CTCSS ENC/DEC" |
| P10 | `0` (Simplex), `2` (Minus Shift) | Legend "0: Simplex … 2: Minus Shift" |
| Direction of the command | Set only | MW chart's Read and Answer rows are printed empty; Control Command List (printed p.5) gives MW as Set=O, Read=X, Ans.=X |

### INHERITED ASSUMPTIONS

**B1 — The minus direction character is a single ASCII HYPHEN-MINUS (0x2D).**
*Applies to vector 2, position 15.*
What I assumed: `-` (one byte, 0x2D).
Why: in revision 2308-F the MW, MR and MT legends all render this glyph as a
**double** dash — "--: Minus Shift" — whereas the IF command legend on printed
page 14 of the same manual renders the same concept as a single "-". The field
is one position wide on the chart, so a two-character token cannot fit; a single
0x2D is the only reading consistent with the position count, but the glyph
itself is a typographic artefact and the byte is inherited, not stated.
**To lift it:** a captured `IF` or `MR` **Answer** from a radio with a *negative*
clarifier offset, with position 15 dumped as a raw byte value — it will read
0x2D, or something else (0x2013 en-dash cannot appear in an ASCII stream, so the
realistic alternatives are 0x2D or a digit-encoded sign).

**B2 — A zero clarifier offset is signed `+`.**
*Applies to `+0000` in MW vector 1, both MT vectors' `+0000`, and the MR answer.*
What I assumed: when the offset is 0000 the direction position carries `+`.
Why: the legend defines exactly two direction characters and is silent about
which accompanies zero; `+` is the conventional choice.
**To lift it:** a captured `MR` or `IF` Answer from a radio with the clarifier
off / offset zero — position 15 shows what the radio itself emits for the
zero case.

**B3 — Offset granularity of the chosen value.**
`-0500` sits inside the documented 0000–9990 range; the legend's 9990 ceiling
implies a 10 Hz step, which 0500 satisfies. If the real step is coarser the
frame would be rejected. Low risk, but unverified.
**To lift it:** send the vector-2 frame and read the channel back via `MR021;`;
the returned P3 either matches or has been quantised.

**B4 — CTCSS `1` without an accompanying tone number.**
The MW frame carries no tone-frequency field, so setting P8=`1` presumes the
tone number comes from elsewhere (the `CN` command, printed p.8). This is a
semantic assumption about radio behaviour, not about byte layout — the bytes
themselves are fully documented.
**To lift it:** set a tone with `CN`, write the vector-2 frame, then read the
channel back and confirm the CTCSS state survived.

---

## MR — `mr-vectors.golden`

**Read request, 6 bytes** — chart row reads `M R P0 P0 P0 ;`:
`1-2 "MR" | 3-5 P0 | 6 ";"`.

**Answer, 28 bytes** — identical position map to MW, with `MR` as the opcode:
`1-2 "MR" | 3-5 P1 | 6-14 P2 | 15-19 P3 | 20 P4 | 21 P5 | 22 P6 | 23 P7 | 24 P8
| 25-26 P9 | 27 P10 | 28 ";"`.

The MR chart's **Set** row is printed empty, matching the Control Command List
(printed p.5): MR is Set=X, Read=O, Ans.=O.

### Manual-documented bytes

| Field | Value used | Manual authority |
|---|---|---|
| Read `MR` + P0 + `;` | `MR001;` | MR Read chart row, positions 1–6; legend "P0 001-099 (Memory Channel), P1L-P9U (PMS), 5xx (5MHz BAND), EMG (EMERGENCY CH)" |
| Answer P2/P3/P4/P5/P6/P8/P9/P10 | as in the MT table above | MR legend, printed p.16 |
| **P7 vocabulary** | `0: VFO` / `1: Memory` | MR legend "P7 0: VFO 1: Memory" — this is the **Read-direction** vocabulary the task asked for, and it is what distinguishes an MR answer from an MT/MW Set frame, where the same position is a fixed `0` |

### INHERITED ASSUMPTIONS

**C1 — P1 at Answer positions 3–5 is the memory channel.**
What I assumed: the Answer's `P1 P1 P1` triple is the channel number and echoes
the requested slot (`001`).
Why: the MR legend defines **P0** and then jumps to **P2** — it never defines P1,
even though the Answer chart uses P1 at positions 3–5. I read P1 = memory channel
from the **MT** legend on the *same printed page*, which is written
"P0/1 001-099 (Memory Channel), …", i.e. P0 and P1 are the same parameter. That
is a within-manual cross-command inference, not a direct statement, so it is an
inherited assumption. That the answer *echoes* the requested slot rather than,
say, reporting the currently selected channel is a further inference.
**To lift it:** send `MR001;` and `MR017;` to a radio with different contents in
those two channels and capture both answers — positions 3–5 echoing `001` and
`017` respectively, with matching payloads, settles both halves of C1.

**C2 — P7 = `1` (Memory) in a memory-read answer.**
What I assumed: `1` at position 23.
Why: the legend offers both `0: VFO` and `1: Memory` but never says which value
an MR answer carries. `1` is inferred because the record being reported *is* a
memory channel; `0` would presumably appear only if the same structure were
reused to report a VFO.
**To lift it:** capture any `MR` answer and read position 23. A single capture
resolves it. Capturing the answer both when the radio is in VFO mode and when it
is in memory mode would additionally show whether the field reflects the radio's
current mode rather than the record type.

**C3 — Payload values are illustrative.**
The answer's P2–P10 (7.100000 MHz, LSB, simplex, no clarifier, CTCSS off) were
chosen to mirror the MT and MW vectors so the four files form one coherent
scenario. They are not a manual-documented claim about what any radio returns.
**To lift it:** the capture in C1 supplies real payload values.

**C4 — B1/B2 (the `-` byte and the `+0000` zero sign) apply to the MR answer too**,
since MR reuses the same P3 field. Same lifting evidence as B1/B2.

---

## EX — `ex-vectors.golden` (9 bytes per Read request)

```
1-2 "EX" | 3-4 P1 | 5-6 P2 | 7-8 P3 | 9 ";"
```

The Read row of the EX chart terminates at position 9 with `;`, leaving the
`10 / nn / **` columns empty — a Read request carries **no** P4. (The Set and
Answer rows do carry P4 from position 9 to a variable end, shown as
`P4 ~ P4 ;`.)

### Manual-documented bytes

| Field | Values used | Manual authority |
|---|---|---|
| positions 1–2 | `EX` | Chart cells |
| P1, 2 positions | `01`, `02`, `03` | EX legend "P1 : 01 - 05"; Table 2 column P1 |
| P2, 2 positions | `01`, `02`, `01` | EX legend "P2 : 01 - 07"; Table 2 column P2 |
| P3, 2 positions | `11`, `03`, `08` | EX legend "P3 : 01 - 23"; Table 2 column P3 |
| position 9 | `;` | Chart cell |

Addresses, each read directly out of Table 2 (MENU Chart) and each from a
different P1 group:

| Frame | P1 group | P2 subgroup | P3 item | Table 2 location |
|---|---|---|---|---|
| `EX010111;` | 01 RADIO SETTING | 01 MODE SSB | 11 SSB OUT LEVEL (0–100, 3 digits) | printed p.10 |
| `EX020203;` | 02 CW SETTING | 02 KEYER | 03 CW WEIGHT (2.5–4.5, P4 = 25–45, 2 digits) | printed p.11 |
| `EX030108;` | 03 OPERATION SETTING | 01 GENERAL | 08 CAT RATE (0:4800 1:9600 2:19200 3:38400 bps, 1 digit) | printed p.11 |

### INHERITED ASSUMPTIONS

**D1 — The EX frame's P1/P2/P3 are the same digits as Table 2's P1/P2/P3 columns.**
What I assumed: the 6 address digits index Table 2's three-level hierarchy in
that order.
Why: the EX legend cross-references Table 2 explicitly only for **P4**
("P4 : Parameter (See Table 2)"). It does not spell out that P1/P2/P3 are
Table 2's first three columns. The inference is strong — Table 2's columns are
literally headed P1, P2, P3, P4, and the legend's ranges are consistent with the
table — but it is an inference.
**To lift it:** send `EX030108;` to a radio, capture the answer, and confirm the
returned P4 is a single digit in 0–3 that tracks the radio's actual CAT RATE menu
setting (change the menu item on the front panel and re-read; the digit must
follow).

**D2 — Two-digit zero-padded encoding of each address level.**
What I assumed: `1` is sent as `01`, `8` as `08`, `11` as `11`.
Why: the chart allots exactly two positions per level and Table 2 prints every
value two-digit (01, 02, 03, 08, 11). Near-certain, but the chart never shows a
worked EX example, so no printed EX frame confirms the zero-fill directly.
**To lift it:** any single captured EX exchange showing the request bytes on the
wire — one worked example fixes it permanently.

**D3 — Legend ranges exceed what Table 2 tabulates.**
Observed manual inconsistency, recorded rather than assumed: the EX legend gives
P1 : 01–05 and P3 : 01–23, but Table 2 as printed on pages 10–12 of revision
2308-F tabulates only P1 groups 01–04 (01 RADIO SETTING, 02 CW SETTING,
03 OPERATION SETTING, 04 DISPLAY SETTING) and no P3 above 21. **All three
addresses used here come from tabulated rows**, so none of the vectors depends
on the untabulated region. Any future vector using P1 = 05 would be entirely
unsupported by this revision of the manual.
**To lift it:** enumerate EX reads across P1 = 05 on real hardware and record
which addresses answer versus which are rejected.

**D4 — Upper-case opcode**, as A4.

---

## Summary of the assumption ledger

| ID | Class | Assumption | Status |
|---|---|---|---|
| A1 | MT | TAG pad byte = ASCII SPACE 0x20 | **INHERITED ASSUMPTION** |
| A2 | MT | Padding is trailing (content left-justified) | **INHERITED ASSUMPTION** |
| A3 | MT | `-` and digits acceptable inside a TAG | **INHERITED ASSUMPTION** |
| A4 | all | Upper-case opcode | documented as *permitted* (printed p.4); choice |
| A5 | MT | Payload values illustrative | not a manual claim |
| B1 | MW, MR, MT | Minus direction = single 0x2D | **INHERITED ASSUMPTION** |
| B2 | MW, MR, MT | Zero offset signed `+` | **INHERITED ASSUMPTION** |
| B3 | MW | `-0500` respects the real offset step | **INHERITED ASSUMPTION** |
| B4 | MW | CTCSS state meaningful without an in-frame tone number | **INHERITED ASSUMPTION** (semantic) |
| C1 | MR | Answer P1 (pos 3–5) = memory channel, echoing the request | **INHERITED ASSUMPTION** (cross-command, from the MT legend) |
| C2 | MR | Answer P7 = `1` (Memory) | **INHERITED ASSUMPTION** |
| C3 | MR | Answer payload illustrative | not a manual claim |
| D1 | EX | Frame P1/P2/P3 = Table 2's P1/P2/P3 columns | **INHERITED ASSUMPTION** |
| D2 | EX | Two-digit zero-padded address levels | **INHERITED ASSUMPTION** |
| D3 | EX | (recorded inconsistency, not an assumption) legend ranges exceed Table 2 | manual defect noted |

Everything **not** listed above — every field width, every position boundary,
every fixed value (`P7` Set = 0, `P9` = "00", `P11` = 0), the mode nibbles, the
CTCSS and shift codes, the channel-number format, the 9-digit frequency and the
`;` terminator — was counted or read directly off the manual's position charts
and parameter legends at the pages named above.

**Every vector in all four files remains HARDWARE-UNVERIFIED.**
