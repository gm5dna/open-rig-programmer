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
//   - the P4 VALUE ranges of three MAX POWER rows in Table 2 — (03,04,01) HF
//     MAX POWER and (03,04,02) 50M MAX POWER, each "5 ~ 100 ... FTDX101D /
//     5 ~ 200 ... FTDX101MP", and (03,04,04) AM MAX POWER (layout 927, 928,
//     931). P4 SEMANTICS are not stored: an EXItem models the address, the
//     labels, the name, the Digits width and the text flag, and all five are
//     printed identically for both models on all three rows. So there is one
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
//   - (03,04,04) AM MAX POWER is SELF-INCONSISTENT within one row: the
//     FTDX101MP arm's human-readable range reads "5 ~ 500" while its own
//     parenthesised P4 form reads "(P4 = 005 ~ 050)" (layout 931, printed
//     page 12). Five hundred watts AM on a 200 W radio is not credible and
//     050 matches the FTDX101D arm's shape, but the chart is recorded as
//     printed, not resolved.
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
//   - (02,01,16) QSK DELAY TIME misspells msec once, "2: 25 mesc" (layout
//     832); and the TX AUDIO subgroup punctuates one legend two ways, "00 :
//     OFF" at (03,03,03/09/12/18) against "00: OFF" at (03,03,06/15).
//
// NONE of these affects M9d-1. The EX inventory models an item's NAME, its
// DIGITS and its TEXT flag only — it neither models nor validates the value
// legends every one of them lives in. They are recorded against the milestone
// where menu VALUES are interpreted for read-only display, at which point each
// becomes a question that display must answer: which index SHIFT FREQUENCY's
// second "1" really is, what AM MAX POWER's MP ceiling actually is, and so on.
// None can be settled from this manual alone.
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
// THE REGISTER IS EMPTY AT THIS TASK AND IS NOT YET A COMPLETENESS CLAIM. This
// package currently declares no dialect — only the inventory and its
// generation. The entries land with dialect.go at M9d-1 task 6, which is where
// the assumed values are first written down, and the register must be complete
// before that task's dialect is reviewed.
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
