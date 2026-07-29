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
// # The ASSUMED register
//
// Four members of this dialect are NOT FTdx10-manual facts. They are
// inherited or structurally required, they are marked ASSUMED at the point
// of use, and the identity-pin tests that compare this dialect with the
// FT-710's therefore compare tables that embed them. Each is listed here so
// that the set has one statement of record, and each is lifted individually
// by a named Stage R capture, never wholesale.
//
// The four sections below are the register's headings. Their CONTENT — the
// values, the exact wording of each caveat, and the config fields carrying
// them — arrives with dialect.go at M9c-4 task 6; this file names them now so
// that the register exists before the first assumption is written down.
//
//   - MTPolicy.TagFill — the byte the FTdx10 pads a short memory tag with.
//     Inherited from the FT-710, whose padding is spaces. The FTdx10's has
//     never been observed.
//
//   - ClarifierPolicy.StepHz — the clarifier's offset granularity. NO step
//     is stated anywhere in this manual. The 0000-9990 Hz range the MR/MT/MW
//     legends and the RD/RU command pages (layout 1507, 1605) all agree on
//     SUPPORTS the inherited value without proving it.
//
//   - SlotSpace.NoneWire — the wire form of "no slot". It appears in no
//     FTdx10 slot legend; it is the FT-710's MR-answer fact. A none-wire is
//     structurally required by the slot space, so one is supplied.
//
//   - the ModeUnset member of the mode table — the placeholder mode. Every
//     FTdx10 mode legend runs 1-F with no '0' member (e.g. MR's at layout
//     1192-1194, MD's at 1146-1149). It is included because parsers must
//     accept the placeholder, not because the manual names it.
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
