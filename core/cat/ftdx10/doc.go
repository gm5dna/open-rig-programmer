// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx10 holds the Yaesu FTdx10's CAT dialect: its EX menu
// inventory, transcribed from the manual's Table 2, and (from M9c-4 task 6)
// the cat.DialectConfig literal that binds it to the FT-710 codec in
// core/cat. It is DATA ONLY — no driver, no fake, no registration, no
// session, no wire. The package cannot register itself with the application:
// SupportedModels derives solely from internal/wiring's driver table.
//
// # Provenance
//
// Everything here comes from the Yaesu FTdx10 CAT Operation Reference
// Manual, edition 2308-F (docs/fixtures-private/manuals/ftdx10_cat_2308-F.pdf
// and its layout extraction ftdx10_layout.txt — both gitignored, so the line
// references below are citations, not links). Table 2 "MENU Chart" spans
// layout lines 652-899; the EX inventory transcribed from it lives in
// table2.csv, whose own header carries the transcription conventions and the
// chart's verbatim defects.
//
// NO FTdx10 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project. Every
// statement in this package is a reading of a manual. The FT-710's inventory
// can point at two hardware sweeps; this one cannot point at a single frame.
// That is why there is no observation CSV, no corrections file, and why the
// ASSUMED register below exists at all.
//
// # The header-vs-chart anomaly
//
// The EX command's own grammar block (layout 637-641) states "P1 : 01 - 05",
// "P2 : 01 - 07" and "P3 : 01 - 23". Table 2 refutes two of the three. The
// chart populates P1 01-04 ONLY — RADIO SETTING, CW SETTING, OPERATION
// SETTING, DISPLAY SETTING — with no P1=05 group and no EXTENSION group
// anywhere in this manual; and its P3 tops out at 21, at (01,03,21) ENC/DEC.
// "P2 : 01 - 07" does hold. The inventory follows the CHART: members at P1
// in {01,02,03,04}, none at 05, no P3 above 21.
//
// This is the FTdx10's own instance of the anomaly the FT-710 carries — see
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
// Transcription B (M9c-4 task 4) was derived from the rendered PDF by an
// agent with no repository access and no sight of transcription A, the
// ledger or any row count. It found four printing defects in the chart's
// VALUE legends BLIND, and they are recorded here because an undocumented
// defect is one somebody later silently "corrects":
//
//   - (01,05,16) SHIFT FREQUENCY numbers two options 1 — "1: 170 Hz 1: 200
//     Hz 2: 425 Hz 3: 850 Hz" — and has no index 0. This is the FTdx10's
//     own analogue of the duplicate-index defect the FT-710's chart carries
//     at its own SHIFT FREQUENCY row.
//
//   - (01,05,15) MARK FREQUENCY likewise has no index 0: its legend opens
//     at "1: 1275 Hz" and runs "2: 2125 Hz".
//
//   - (01,06,02) DECODE AFC RANGE is non-monotonic — "0: 8 1: 1.5 2: 30 Hz"
//     — so the printed order of its indices is not the order of its values.
//
//   - (01,06,05) PSK TX LEVEL declares Digits 3 but prints its range as
//     bare "0 ~ 100", with no zero-padded form, where every comparable row
//     spells the padding out (e.g. "0 ~ 100 (P4 = 000 ~ 100)").
//
// NONE of the four affects M9c-4. The EX inventory models an item's NAME,
// its DIGITS and its TEXT flag only — it neither models nor validates the
// value legends all four defects live in — and transcriptions A and B and
// the ledger agree on every field it does model (crosscheck_test.go). The
// four are recorded against M9c-5 and later, where menu VALUES are
// interpreted for read-only display and each becomes a question that
// display must answer: which index SHIFT FREQUENCY's second "1" really is,
// whether an unpadded P4 is admissible for PSK TX LEVEL, and so on. None of
// them can be settled from this manual alone.
//
// # The ASSUMED register
//
// Four members of this dialect are NOT FTdx10-manual facts. They are
// inherited or structurally required, they are marked ASSUMED at the point
// of use, and the identity-pin tests that compare this dialect with the
// FT-710's therefore compare tables that embed them. Each is listed here so
// that the set has one statement of record, and each is lifted individually
// by a named Stage R capture, never wholesale.
//
// Each entry below names the field, the value it carries, the evidence gap,
// and the ONE Stage R capture that lifts it. The captures are individual on
// purpose: a single FTdx10 session does not retire this register wholesale,
// it retires the assumptions its own frames actually speak to, and an entry
// whose capture was not taken stays here afterwards.
//
//   - MTPolicy.TagFill = ' ' (dialect.go). The byte the FTdx10 pads a short
//     memory tag with, in both directions: builds pad the outbound P12 field
//     to 12 bytes with it, and parses trim it from the answer. Inherited from
//     the FT-710, whose padding is spaces; the FTdx10's has never been
//     observed. The manual's P12 legend says only "TAG Characters (up to 12
//     characters) (ASCII)" (layout 1236) and names no fill.
//     STAGE R LIFTS IT WITH: one MT Set of a tag SHORTER than 12 characters
//     to a memory channel, then an MT read of that channel — the bytes the
//     radio returns after the written characters ARE the fill. If they are
//     not spaces, this field changes and the goldens' padding bytes change
//     with it; if the field comes back short instead, the assumption that
//     failed is the answer's exact width, not this byte (see mtcombined.go).
//
//   - ClarifierPolicy.StepHz = 10 (dialect.go). The clarifier's offset
//     granularity, which core/cat enforces as a multiple-of-step rule on
//     every MW and combined-MT Set. NO step is stated anywhere in this
//     manual. The 0000-9990 Hz range the MR/MT/MW legends and the RD/RU
//     command pages (layout 1507, 1605) all agree on SUPPORTS the inherited
//     value without proving it: a 20 Hz radio could not reach its own stated
//     9990, a 10 Hz one can, and a 1 Hz one would be free to stop at 9999 and
//     does not.
//     STAGE R LIFTS IT WITH: one MW Set carrying a clarifier offset that is
//     NOT a multiple of 10 — 0005 Hz — followed by an MR read of the same
//     channel. A radio answering 0005 has a finer step than this and the
//     value drops; a radio answering 0000 or 0010 has quantised, and 10 is
//     confirmed at that resolution.
//
//   - SlotSpace.NoneWire = "000" (dialect.go). The wire form of "no slot" —
//     the value an MR answer carries when the source is not a memory. It
//     appears in NO FTdx10 slot legend: MC's gives 001-099, P1L-P9U, 5xx and
//     EMG (layout 1131-1133), MR's the same (1184-1185), MW's only 001-099
//     and P1L-P9U (1259). It is the FT-710's MR-answer fact, and cat.SlotSpace
//     structurally requires a none form, so one is supplied.
//     STAGE R LIFTS IT WITH: one MR read taken while the radio is on a VFO
//     rather than a memory — the P1 field of the answer is this radio's own
//     none form. Note the collision this field guards against: a radio
//     numbering memories from 000 would make "000" ambiguous, which is why
//     cat.NewDialect validates the two against each other rather than
//     assuming.
//
//   - the cat.ModeUnset member of the mode table (dialect.go's modeNames).
//     The '0' = "-" placeholder. EVERY FTdx10 mode legend runs 1-F with no
//     '0' member, and there are four of them, all identical: MD's at layout
//     1146-1149, MR's P6 at 1192-1194, MT's P6 at 1227-1229, MW's P6 at
//     1267-1269. It is included because parsers must accept the placeholder —
//     core/cat refuses to EMIT it in any Set frame, so its presence widens
//     only what this dialect can read — not because the manual names it.
//     STAGE R LIFTS IT WITH: one MR read of an EMPTY memory channel, one the
//     radio has never had written to. The P6 byte of that answer is what this
//     radio says for "no mode". If it is not '0', this member is wrong rather
//     than merely unattested, and the real byte replaces it.
//
// # Reused-command verification
//
// Before this dialect reuses the FT-710 codec in core/cat for MC, ID, AI,
// MR, MW and the EX read/answer grammar, each command's frame table in the
// manual was checked against the shape that codec assumes. A deviation would
// be a STOP-and-respec, because core/cat's parsers and builders are
// fixed-offset: a frame one byte different is not a dialect parameter, it is
// a different codec.
//
// The FTdx10's availability table (layout 192-271, columns Set / Read /
// Ans. / AI) was checked first, then each command's own position chart.
//
// VERDICT: ALL SIX ARE IDENTICAL to the FT-710's frame shapes. No deviation
// was found, and no STOP was raised. Command by command:
//
//   - AI (availability 198: O O O X; frames 309-320). Set "AI P1 ;" and
//     Answer "AI P1 ;" are four bytes, Read "AI;" three. core/cat's
//     aiFrameLen is 4 (ai.go:7). Identical.
//
//   - ID (availability 238: X O O X; frames 976-984). No Set; Read "ID;";
//     Answer "ID P1 P1 P1 P1 ;" is seven bytes. core/cat's idAnswerLen is 7
//     (id.go:7). Identical. The ID VALUE differs — 0761 here against the
//     FT-710's 0800 — but that is dialect data carried by CATID, not frame
//     shape, and is pinned as a difference at task 6.
//
//   - MC (availability 254: O O O X; frames 1130-1140). Set and Answer
//     "MC P1 P1 P1 ;" are six bytes, Read "MC;" three; the P1 legend reads
//     "001-099 (Memory Channel), P1L-P9U (PMS), 5xx (5MHz BAND), EMG
//     (EMERGENCY CH)". core/cat's mcSetLen is 6 and mcReadFrame is "MC;"
//     (mc.go:7, mc.go:11), over the same slot space. Identical.
//
//   - MR (availability 258: X O O X; frames 1183-1202). No Set, as core/cat
//     assumes; Read "MR P0 P0 P0 ;" is six bytes (mrReadLen, mr.go:9); the
//     Answer's position chart runs to 28 and matches memdata.go's field
//     block offset for offset — P1 slot at 3-5, P2 frequency at 6-14 (nine
//     digits), P3 clarifier sign at 15 and magnitude at 16-19, P4 at 20, P5
//     at 21, P6 mode at 22, P7 kind at 23, P8 CTCSS at 24, P9 fixed "00" at
//     25-26, P10 shift at 27, ';' at 28 (memoryFrameLen 28, memdata.go:14,
//     offsets memdata.go:27-38). Identical.
//
//   - MW (availability 263: O X X X; frames 1258-1272). Set only, no Read
//     and no Answer, exactly as core/cat's mw.go assumes; the Set frame is
//     the same 28-byte chart as MR's Answer under an "MW" prefix, which is
//     why core/cat decodes both through one parseMemoryFrame; and the P1
//     legend is restricted to "001-099 (Memory Channel), P1L-P9U (PMS)" —
//     no 5xx, no EMG — which is the FT-710's MW restriction too. Identical.
//     The one difference is again a VALUE, not a shape: this manual's P7
//     reads "0: (Fixed)" where the FT-710's write kind is '1',
//     HW-confirmed. That is what MWWriteKind carries.
//
//   - EX read/answer grammar (availability 233: O O O O; frames 636-645).
//     Read "EX P1 P1 P2 P2 P3 P3 ;" is nine bytes (exReadLen, ex.go:10);
//     Answer "EX P1 P1 P2 P2 P3 P3 P4 ~ P4 ;" is the six-digit address
//     followed by a variable P4 and the terminator, which is exactly
//     core/cat's exAnswerMinLen of 2+6+1+1 (ex.go:23) with the upper bound
//     derived per dialect from the widest Digits. Identical. The grammar
//     block's P1/P3 ranges are the anomaly recorded above; they bound
//     nothing here, because membership comes from the chart.
//
// Checked additionally, though outside the six: MT (availability 261: O O O
// X; frames 1217-1256). Read "MT P0 P0 P0 ;" is six bytes (mtReadLen,
// mt.go:12), and the Set/Answer chart runs to 41 — the 28 shared positions,
// P11 fixed "0" at 28, a 12-character P12 tag at 29-40, ';' at 41 — which is
// core/cat's mtCombinedLen() of 29 + TagMaxBytes at a 12-byte tag
// (mtcombined.go). P7 reads "Set: 0: (Fixed) / Read: 0: VFO 1: Memory",
// matching cat.CombinedMTSetKind. Identical.
package ftdx10
