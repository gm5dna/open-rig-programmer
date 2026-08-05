// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx101 holds the Yaesu FTdx101D and FTdx101MP's CAT dialect: its
// EX menu inventory, transcribed from the manual's Table 2, and (from M9d-1
// task 6) the cat.DialectConfig literals that bind it to the FT-710 codec in
// core/cat. It is DATA ONLY — no driver, no fake, no registration, no
// session, no wire. The package cannot register itself with the application:
// SupportedModels derives solely from internal/wiring's driver table.
//
// # Provenance
//
// Everything here comes from the Yaesu FTDX101MP/FTDX101D CAT Operation
// Reference Manual, edition 2308-L
// (docs/fixtures-private/manuals/ftdx101_cat_2308-L.pdf and its layout
// extraction ftdx101_layout.txt — both gitignored, so the line references
// below are citations, not links). Table 2 "MENU Chart" spans layout lines
// 715-962 (printed pages 10-12); the EX inventory transcribed from it lives
// in table2.csv, whose own header carries the transcription conventions and
// the chart's verbatim defects.
//
// NO FTdx101 HARDWARE OF EITHER MODEL HAS EVER BEEN ASKED ANYTHING by this
// project. Every statement in this package is a reading of a manual. The
// FT-710's inventory can point at two hardware sweeps; this one cannot point
// at a single frame. That is why there is no observation CSV, no corrections
// file, and why the ASSUMED register below exists at all.
//
// # One manual, two radios
//
// Yaesu prints ONE CAT manual for the FTDX101MP and the FTDX101D, and the two
// models are distinguished in exactly two places in it:
//
//   - the ID answer's value — "0681: FTDX101D", "0682: FTDX101MP" (layout
//     1070-1072). This is a dialect datum carried by CATID, and it is why
//     M9d-1 task 6 builds TWO Dialect instances over ONE inventory rather
//     than one dialect for both.
//
//   - the P4 VALUE ranges of three MAX POWER rows in Table 2, printed IN FULL
//     here because a model-conditional figure abbreviated to one arm is how a
//     model-conditional figure stops being read as one:
//
//     (03,04,01) HF MAX POWER (layout 927) —
//     "5 ~ 100 (P4 = 005 ~ 100) FTDX101D / 5 ~ 200 (P4 = 005 ~ 200) FTDX101MP".
//
//     (03,04,02) 50M MAX POWER (layout 928) — identical wording,
//     "5 ~ 100 (P4 = 005 ~ 100) FTDX101D / 5 ~ 200 (P4 = 005 ~ 200) FTDX101MP".
//
//     (03,04,04) AM MAX POWER (layout 931) —
//     "5 ~ 25 (P4 = 005 ~ 025) FTDX101D / 5 ~ 500 (P4 = 005 ~ 050) FTDX101MP".
//     BOTH arms are given: the D arm is internally consistent at 25 W, and it
//     is what makes the MP arm's "5 ~ 500" against "(P4 = 005 ~ 050)" visibly
//     a defect rather than merely a large number. See the chart printing
//     defects below.
//
//     (03,04,03) 70M MAX POWER (layout 929) is NOT model-conditional —
//     "5 ~ 50 (P4 = 005 ~ 050)" for both — which is why the count is three and
//     not four.
//
//     P4 SEMANTICS are not stored: an EXItem models the address, the labels,
//     the name, the Digits width and the text flag, and all five are printed
//     identically for both models on all three rows. So there is one
//     transcription and one generated inventory.
//
// Nothing else in the chart or in the frame tables distinguishes them.
//
// # The header-vs-chart anomaly
//
// The EX command's own grammar block (layout 700-704) states "P1 : 01 - 05",
// "P2 : 01 - 07" and "P3 : 01 - 23". Table 2 refutes ONE of the three. The
// chart populates P1 01-04 ONLY — RADIO SETTING, CW SETTING, OPERATION
// SETTING, DISPLAY SETTING — with no P1=05 group and no EXTENSION group
// anywhere in this manual; Table 2 ends at (04,03,02) PIXEL, layout 962. The
// other two bounds hold exactly, and both are REACHED: the widest menu, P1=01,
// runs P2 01-07, and (03,01,23) KEYBOARD LANGUAGE is the chart's highest P3
// at 23. The inventory follows the CHART: members at P1 in {01,02,03,04},
// none at 05.
//
// This is the FTdx101's own instance of the anomaly the FT-710 carries — see
// the P1 ANOMALY note on KnownEXAddress in core/cat/dialect.go, where that
// radio's grammar block said "P1: 01 - 04, 05", its Table 2 named
// {01,02,03,04,06}, and a real FT-710 at M8c rejected both probed P1=05
// addresses. The FT-710's anomaly could be put to hardware; this one cannot
// be, and is recorded UNRESOLVED rather than reasoned away. It is
// deliberately not written as a corrections file: that artefact format
// records corrections hardware evidence establishes.
//
// # Chart printing defects
//
// The chart's VALUE legends carry defects, found during transcription A and
// verified against the rendered PDF. They are recorded here because an
// undocumented defect is one somebody later silently "corrects":
//
//   - (03,04,04) AM MAX POWER is SELF-INCONSISTENT within one row. The row is
//     printed "5 ~ 25 (P4 = 005 ~ 025) FTDX101D / 5 ~ 500 (P4 = 005 ~ 050)
//     FTDX101MP" (layout 931, printed page 12): the FTDX101D arm agrees with
//     itself, and the FTDX101MP arm's human-readable range reads "5 ~ 500"
//     while its own parenthesised P4 form reads "(P4 = 005 ~ 050)". Five
//     hundred watts AM on a 200 W radio is not credible, and 050 matches both
//     the shape of the D arm and the two-to-one D:MP ratio the other two MAX
//     POWER rows carry — but the chart is recorded as printed, not resolved.
//
//   - (02,01,12) CW BK-IN DELAY prints a TRUNCATED legend whose ninth entry
//     repeats its sixth: "00:30 01:50 02:100 03:150 04:200 05:250 06:300
//     07:400 05:250 .... 31:2800 32:2900 33:3000msec" (layout 825-827). The
//     elision is the chart's own; the second "05:250", where an 08: entry
//     belongs, is a printing defect.
//
//   - (01,05,14) SHIFT FREQUENCY numbers two options 1 — "1: 170 Hz 1: 200
//     Hz 2: 425 Hz 3: 850 Hz" — and has no index 0 (layout 793). This is the
//     FTdx101's own analogue of the duplicate-index defect the FT-710's chart
//     carries at its own SHIFT FREQUENCY row.
//
//   - (01,05,13) MARK FREQUENCY likewise has no index 0: its legend opens at
//     "1: 1275 Hz" and runs "2: 2125 Hz" (layout 792).
//
//   - (01,06,02) DECODE AFC RANGE is non-monotonic — "0: 8 1: 1.5 2: 30 Hz"
//     (layout 795) — so the printed order of its indices is not the order of
//     its values.
//
//   - (03,01,23) KEYBOARD LANGUAGE ends its list of languages with something
//     that is not one. The legend runs "00: JAPANESE 01: ENGLISH(US) 02:
//     ENGLISH(UK) 03: FRENCH 04: FRENCH(CA) 05: GERMAN 06: PORTUGUESE 07:
//     PORTUGUESE(BR) 08: SPANISH 09: SPANISH(LATAM) 10: ITALIAN 11: LEVEL"
//     (layout 879-883, printed page 12 / folio 11). "11: LEVEL" is the last
//     entry of the row IMMEDIATELY ABOVE it — (03,01,22) CS DIAL, whose own
//     legend ends "08: MEM CH 09: GROUP 10: R.FIL 11: LEVEL" (layout 876-878)
//     — and the two rows are typeset interleaved on the page, CS DIAL's third
//     legend line sitting directly above KEYBOARD LANGUAGE's first. A twelfth
//     keyboard language called LEVEL does not exist; the entry has migrated
//     from the row above. Recorded as printed, not repaired: whether KEYBOARD
//     LANGUAGE really has eleven values (00-10) or twelve with a different
//     twelfth cannot be settled from this manual.
//
//   - (02,01,16) QSK DELAY TIME misspells msec once, "2: 25 mesc" (layout
//     832); and the TX AUDIO subgroup punctuates one legend two ways, "00 :
//     OFF" at (03,03,03/09/12/18) against "00: OFF" at (03,03,06/15).
//
// ONE DEFECT LIES OUTSIDE TABLE 2, in a frame table's own legend, and is
// recorded here with the rest because it is the same kind of fault and
// because dialect.go's mode transcription had to decide what to do about it:
//
//   - OI's P6 MODE legend MISNUMBERS its last two members. It prints "D: AM-N
//     E: PSK E: DATA-FM-N" (layout 1443-1446, PDF page 19) — a duplicated
//     "E:" prefix where "F:" belongs. The FIVE other mode legends in this
//     manual are identical to each other and clean, all running 1 to F: MD's
//     P2 (layout 1240-1243), IF's P6 (1089-1091), MR's P6 (1286-1288), MT's
//     P6 (1321-1323) and MW's P6 (1361-1363). The mode table in dialect.go is
//     sourced from those five and OI's is EXCLUDED — not reconciled against
//     them, and not "corrected" to F. Reading the defective legend as
//     evidence would either lose DATA-FM-N or make 'E' ambiguous, and the
//     five clean legends are sufficient without it.
//
// NONE of these affects M9d-1. The EX inventory models an item's NAME, its
// DIGITS and its TEXT flag only — it neither models nor validates the value
// legends every one of them lives in — and transcriptions A and B and the
// ledger agree on every field it does model (crosscheck_test.go, which binds
// all three artefacts to one another and is the evidence that they agree).
// They are recorded against the milestone where menu VALUES are interpreted
// for read-only display, at which point each becomes a question that display
// must answer: which index SHIFT FREQUENCY's second "1" really is, what AM MAX
// POWER's MP ceiling actually is, whether KEYBOARD LANGUAGE has eleven values
// or twelve, and so on. None can be settled from this manual alone.
//
// One further chart property is a layout fact rather than a defect, and is
// recorded in table2.csv's header: (04,01,07)'s Function name wraps across two
// printed lines as "MOUSE POINTER" / "SPEED" (layout 952-954) and is
// transcribed as the single name "MOUSE POINTER SPEED". Seven rows wrap their
// P4 legend the same way.
//
// # The ASSUMED register
//
// Members of this dialect that are NOT FTdx101-manual facts — inherited or
// structurally required values, marked ASSUMED at the point of use — belong in
// a register here, each with the field, the value it carries, the evidence gap,
// and the ONE Stage R capture that lifts it, exactly as core/cat/ftdx10/doc.go
// keeps its six. The captures are individual on purpose: a single FTdx101
// session does not retire the register wholesale, it retires the assumptions
// its own frames actually speak to.
//
// SIX MEMBERS OF THIS DIALECT ARE NOT FTdx101-MANUAL FACTS. They are inherited
// or structurally required, they are marked ASSUMED at the point of use, and
// the identity-pin tests that compare these dialects with the FT-710's
// therefore compare tables that embed them. Each is listed here so that the set
// has one statement of record.
//
// EVERY CAPTURE BELOW IS PER MODEL. There are two radios and two dialect
// instances, and a capture taken from an FTDX101D lifts the D's entry only:
// the two models share a manual, not a serial port, and "the D answered 0005"
// is not evidence about the MP. An entry stays here for whichever model has
// not been asked. This is the register's one difference from the FTdx10's,
// where there is a single radio and a single lifting.
//
//   - MTPolicy.TagFill = ' ' (dialect.go). The byte the FTdx101 pads a short
//     memory tag with, in both directions: builds pad the outbound P12 field
//     to 12 bytes with it, and parses trim it from the answer. Inherited from
//     the FT-710, whose padding is spaces; neither FTdx101's has ever been
//     observed. The manual's P12 legend says only "TAG Characters (up to 12
//     characters) (ASCII)" (layout 1330) and names no fill.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MT Set of a tag SHORTER than 12
//     characters to a memory channel, then an MT read of that channel — the
//     bytes the radio returns after the written characters ARE the fill. If
//     they are not spaces, this field changes for that model and the goldens'
//     padding bytes change with it; if the field comes back short instead, the
//     assumption that failed is the answer's exact width, not this byte (see
//     the entry below and mtcombined.go).
//
//   - ClarifierPolicy.StepHz = 10 (dialect.go). The clarifier's offset
//     granularity, which core/cat enforces as a multiple-of-step rule on every
//     MW and combined-MT Set. NO step is stated anywhere in this manual. The
//     0000-9990 Hz range that the IF, MR, MT and MW legends and the RD and RU
//     command pages (layout 1602, 1700) all agree on SUPPORTS the inherited
//     value without proving it: a 20 Hz radio could not reach its own stated
//     9990, a 10 Hz one can, and a 1 Hz one would be free to stop at 9999 and
//     does not.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MW Set carrying a clarifier
//     offset that is NOT a multiple of 10 — 0005 Hz — followed by an MR read
//     of the same channel. A radio answering 0005 has a finer step than this
//     and the value drops for that model; a radio answering 0000 or 0010 has
//     quantised, and 10 is confirmed for it at that resolution.
//
//   - SlotSpace.NoneWire = "000" (dialect.go). The wire form of "no slot" —
//     the value an MR answer carries when the source is not a memory. It
//     appears in NO FTdx101 slot legend: MC's gives 001-099, P1L-P9U, 5xx and
//     EMG (layout 1225-1227), IF's the same (1082-1083), MR's the same
//     (1278-1279), MT's the same (1312-1313), and MW's only 001-099 and
//     P1L-P9U (1353). It is the FT-710's MR-answer fact, and cat.SlotSpace
//     structurally requires a none form, so one is supplied.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MR read taken while the radio is
//     on a VFO rather than a memory — the P1 field of the answer is that
//     radio's own none form. Note the collision this field guards against: a
//     radio numbering memories from 000 would make "000" ambiguous, which is
//     why cat.NewDialect validates the two against each other rather than
//     assuming.
//
//   - the cat.ModeUnset member of the mode table (dialect.go's modeNames).
//     The '0' = "-" placeholder. EVERY FTdx101 mode legend runs 1-F with no
//     '0' member: the five clean ones — MD's P2 at layout 1240-1243, IF's P6
//     at 1089-1091, MR's P6 at 1286-1288, MT's P6 at 1321-1323, MW's P6 at
//     1361-1363 — and the defective OI one at 1443-1446, whose fault is in its
//     last two indices and not at its head. It is included because parsers must
//     accept the placeholder — core/cat refuses to EMIT it in any Set frame, so
//     its presence widens only what this dialect can read — not because the
//     manual names it.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MR read of an EMPTY memory
//     channel, one that radio has never had written to. The P6 byte of that
//     answer is what it says for "no mode". If it is not '0', this member is
//     wrong rather than merely unattested, and the real byte replaces it.
//
//   - SlotSpace.SixtyLo/SixtyHi = 501/599 (dialect.go). The 60 m bank's
//     NUMBERING. Every FTdx101 slot legend that mentions the bank at all says
//     only "5xx (5MHz BAND)" (layout 1082-1083, 1225-1227, 1278-1279,
//     1312-1313): the start at 501 rather than 500, the ceiling at 599 and
//     therefore the channel count are interpretation inherited from the FT-710
//     by way of the FTdx10 — and the FT-710's own reference marks exactly this
//     numbering unverified (core/cat/slot.go's 60 m note), so the FTdx10's
//     agreement adds a second unverified user rather than evidence.
//     STAGE R LIFTS IT, PER MODEL, WITH: an MR enumeration of the 5xx range
//     INCLUDING 500 — which wire numbers answer as populated-or-empty channels
//     and which answer "?;" fixes the real bounds. A radio accepting 500 moves
//     SixtyLo; one refusing 599 moves SixtyHi.
//
//   - The combined MT answer's EXACT length (consumed here as
//     MTAnswerBounds() = (41, 41)). Not a field of this dialect but an
//     assumption it inherits from core/cat's combined form (mtcombined.go's own
//     ASSUMED-until-Stage-R note): the manual's grid draws the MAXIMAL frame,
//     and the FT-710 precedent — hardware accepting short MT Sets against a
//     maximal grid — makes a variable-width ANSWER live. The grid itself is
//     independently attested for this radio: testdata/geometry-witness.csv
//     records the MT Set and Answer charts running to 41 as counted off 300 dpi
//     raster renders, and geometry_test.go binds that count to this dialect.
//     What neither establishes is that the RADIO answers at full width; a
//     printed grid and a returned frame are different claims.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MT READ of a channel carrying a
//     tag SHORTER than 12 characters, the raw answer captured whole. A 41-byte
//     answer confirms exactness for that model; anything shorter converts the
//     parser and the gate to the recorded 30..41 window contingency.
//
// THE REGISTER IS COMPLETE AS AT M9d-1 TASK 6 and is a completeness claim: no
// other value in dialect.go is assumed. The four things this package states
// that ARE manual facts, and so are deliberately absent from the register, are
// the CAT IDs (layout 1070-1072), the 1-F mode table (five identical legends),
// the memory/PMS slot bounds and the EMG wire (the slot legends above), and
// MWWriteKind (MW's "P7 0: (Fixed)", layout 1364).
//
// # Reused-command verification
//
// Before this dialect reuses the FT-710 codec in core/cat for MC, ID, AI, MR,
// MW and the EX read/answer grammar, each command's frame table in THIS manual
// was checked against the shape that codec assumes. A deviation would be a
// STOP-and-respec, because core/cat's parsers and builders are fixed-offset: a
// frame one byte different is not a dialect parameter, it is a different codec.
//
// The FTdx101's availability table (layout 236-337, columns Set / Read / Ans. /
// AI) was checked first, then each command's own position chart.
//
// VERDICT: ALL SIX ARE IDENTICAL to the FT-710's frame shapes. No deviation
// was found, and no STOP was raised. Command by command:
//
//   - AI (availability 244: O O O X; frames 376-385). Set "AI P1 ;" and
//     Answer "AI P1 ;" are four bytes, Read "AI;" three. core/cat's
//     aiFrameLen is 4 (ai.go:7). Identical. This manual adds a note the
//     FT-710's does not — AI is available only over the USB cable, and the
//     radio forces the parameter to 0 at power-off (layout 381, 384) — but
//     that is operating context, not frame shape.
//
//   - ID (availability 304: X O O X; frames 1069-1078). No Set; Read "ID;";
//     Answer "ID P1 P1 P1 P1 ;" is seven bytes. core/cat's idAnswerLen is 7
//     (id.go:7). Identical. The ID VALUE differs, and differs BETWEEN THE TWO
//     MODELS — 0681 for the FTDX101D and 0682 for the FTDX101MP against the
//     FT-710's 0800 — but that is dialect data carried by CATID, not frame
//     shape. It is pinned as a difference, and as the reason for two dialect
//     instances, at task 6.
//
//   - MC (availability 327: O O O X; frames 1224-1233). Set and Answer
//     "MC P1 P1 P1 ;" are six bytes, Read "MC;" three; the P1 legend reads
//     "001-099 (Memory Channel), P1L -P9U (PMS), 5xx (5MHz BAND), EMG
//     (EMERGENCY CH)" (layout 1225-1227; the chart sets that legend with
//     letter tracking, which the extraction renders as spaced-out characters,
//     so it was read from the rendered page instead). core/cat's mcSetLen is 6
//     and mcReadFrame is "MC;" (mc.go:7, mc.go:11), over the same slot space.
//     Identical.
//
//   - MR (availability 331: X O O X; frames 1277-1294). No Set, as core/cat
//     assumes; Read "MR P0 P0 P0 ;" is six bytes (mrReadLen, mr.go:9); the
//     Answer's position chart runs to 28 and matches memdata.go's field block
//     offset for offset — P1 slot at 3-5, P2 frequency at 6-14 (nine digits),
//     P3 clarifier sign at 15 and magnitude at 16-19, P4 at 20, P5 at 21, P6
//     mode at 22, P7 kind at 23, P8 CTCSS at 24, P9 fixed "00" at 25-26, P10
//     shift at 27, ';' at 28 (memoryFrameLen 28, memdata.go:14, offsets
//     memdata.go:27-38). Identical.
//
//   - MW (availability 336: O X X X; frames 1352-1367). Set only, no Read and
//     no Answer, exactly as core/cat's mw.go assumes; the Set frame is the
//     same 28-byte chart as MR's Answer under an "MW" prefix, which is why
//     core/cat decodes both through one parseMemoryFrame; and the P1 legend is
//     restricted to "001-099 (Memory Channel), P1L -P9U (PMS)" — no 5xx, no
//     EMG — which is the FT-710's MW restriction too. Identical. The one
//     difference is again a VALUE, not a shape: this manual's P7 reads
//     "0: (Fixed)" (layout 1364) where the FT-710's write kind is '1',
//     HW-confirmed. That is what MWWriteKind carries.
//
//   - EX read/answer grammar (availability 286: O O O O; frames 699-708).
//     Read "EX P1 P1 P2 P2 P3 P3 ;" is nine bytes (exReadLen, ex.go:10);
//     Answer "EX P1 P1 P2 P2 P3 P3 P4 ~ P4 ;" is the six-digit address
//     followed by a variable P4 and the terminator, which is exactly
//     core/cat's exAnswerMinLen of 2+6+1+1 (ex.go:23) with the upper bound
//     derived per dialect from the widest Digits. Identical. The grammar
//     block's P1 range is the anomaly recorded above; it bounds nothing here,
//     because membership comes from the chart.
//
// Checked additionally, though outside the six: MT (availability 334: O O O X;
// frames 1311-1345). Read "MT P0 P0 P0 ;" is six bytes (mtReadLen, mt.go:12),
// and the Set/Answer chart runs to 41 — the 28 shared positions, P11 fixed "0"
// at 28, a 12-character P12 tag at 29-40, ';' at 41 — which is core/cat's
// mtCombinedLen() of 29 + TagMaxBytes at a 12-byte tag (mtcombined.go). P7
// reads "Set: 0: (Fixed) / Read: 0: VFO 1: Memory" (layout 1324), matching
// cat.CombinedMTSetKind, and P12 reads "TAG Characters (up to 12 characters)
// (ASCII)" (layout 1330) — which, as on the FTdx10, names no fill byte.
package ftdx101
