// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft991a holds the Yaesu FT-991A's CAT dialect: its EX menu
// inventory, transcribed from the manual's menu chart, and the
// cat.DialectConfig literal that binds it to the shared codec in core/cat.
// It is DATA ONLY — no driver, no fake, no registration, no session, no
// wire. The package cannot register itself with the application:
// SupportedModels derives solely from internal/wiring's driver table.
//
// # Provenance
//
// Everything here comes from the Yaesu FT-991A CAT Operation Reference
// Manual, revision 1711-D (docs/fixtures-private/manuals/ft991a_cat_1711-D.pdf,
// SHA-256 52164f737e37a3ffc8276eccedf8ecb524607198c38d386ae4ff7b2fc1d60c95,
// 20 PDF pages, and its layout extraction ft991a_layout.txt — both
// gitignored, so the line references throughout this package are citations,
// not links; the file's own provenance note is
// docs/fixtures-private/manuals/ft991a-manual-provenance.md). The untitled
// menu chart that follows the EX MENU command block spans layout lines
// 530-694; the EX inventory transcribed from it lives in table2.csv, whose
// header carries the transcription conventions and the chart's verbatim
// defects. The memory-frame blocks this dialect's other axes are read from
// are MR at 965-981, MT at 998-1033 and MW at 1036-1051, with IF at 782-799
// and OI at 1116-1132 as the two further blocks that carry the same
// 28-position field grid.
//
// NO FT-991A HARDWARE HAS EVER BEEN ASKED ANYTHING by this project, and none
// is available to it. Every statement in this package is a reading of a
// manual. There is no observation CSV, no corrections file and not a single
// captured frame, which is why the ASSUMED register below exists at all and
// why nothing here may be quoted as verification.
//
// # The evidence base, and what agreement between its legs does not prove
//
// This package's chart transcription is the product of a three-leg blind
// chain, and testdata/ holds two of the three legs beside their own
// derivation records: transcription B (transcription-b.csv, with
// transcription-b.md) and the page ledger (ledger.csv, with ledger.md),
// each derived from the rendered PDF by an agent that had not seen this
// repository, the other legs, or the plan. crosscheck_test.go binds them to
// transcription A — table2.csv, this package's only generation source — and
// to the generated inventory this dialect exposes.
//
// TWO NORMALISATIONS WERE PRE-DECLARED BEFORE THE COMPARISON RAN, and it is
// worth recording what each did, because a bare "zero mismatches" hides the
// difference between them.
//
// The FIRST is row 087's Digits cell, where A writes the chart's own hyphen
// and B writes "?" — its brief's rule for a cell that is not an integer.
// That one FIRES, on exactly the one address the ft991a extable profile
// declares parameterless, and on no other.
//
// The SECOND is the spaced colon four rows print ("00 : OFF" against their
// siblings' "00: OFF", layout 655, 666, 669 and 675). IT CANNOT FIRE AT
// COMPARISON TIME AT ALL, and this package says so rather than leaving the
// plan's wording to be read as a claim about leg B. The spaced colon is
// printed in the chart's PARAMETER column, and transcription B carries no
// parameter column: B was briefed to record the MENU number, the name and
// the Digits value only. So the cell the normalisation concerns is
// single-sourced. What the cross-check does instead is pin the printed
// four/two split inside transcription A against the literals BOTH
// quarantined records independently describe (testdata/ledger.md's defect
// list and testdata/transcription-b.md), and then prove that no COMPARED
// cell in either leg carries a spaced colon — so the normalisation cannot
// have been silently needed and skipped.
//
// AND ALL THREE LEGS READ THE SAME PRINTED CHART. A defect PRINTED in the
// chart is transcribed faithfully by every leg and is invisible to every
// comparison: three-way agreement is evidence about transcription, never
// about the manual. That is why the defects below are recorded here rather
// than discovered by a test.
//
// # Chart printing defects
//
// The chart's own defects are recorded here because an undocumented defect
// is one somebody later silently "corrects". They are RECORDED, NOT
// RESOLVED: this repository has no FT-991A to ask which reading is right.
// table2.csv's provenance header carries the same list against the rows it
// transcribes; this is the dialect's copy of record. There are SEVEN.
//
//   - 068 DATA HCUT FREQ (layout 604) and 069 DATA HCUT SLOPE (605) PRINT
//     EACH OTHER'S DIGITS. 068's legend is "00: OFF 01: 700 Hz ~ 67:
//     4000 Hz (50 Hz steps)", which needs two digits, and the chart prints
//     1; 069's is "0: 6 dB/oct 1: 18 dB/oct", which needs one, and the
//     chart prints 2. This pair is FIRST because it is the one the
//     cross-check CANNOT CATCH: a transposed pair is transcribed identically
//     by every leg, so three-way agreement is not evidence about these two
//     rows. Every other HCUT FREQ/SLOPE pair in the chart prints 2 then 1 —
//     043/044, 052/053, 094/095 and 104/105 — and crosscheck_test.go pins
//     those four alongside, so a sibling drifting the same way would make
//     the "one pair is transposed" claim false and fail a test rather than
//     pass unremarked.
//
//   - 028 GPS/232C SELECT (layout 558) prints "0: GPS1 1: GPS2 3: RS232C" —
//     KEY 2: IS SKIPPED. Whether the radio's P2 domain really has a hole at
//     2 or the chart lost an entry is not knowable from this manual, and
//     nothing here fills the gap in. IT IS RECORDED NEXT TO THE CITATION IT
//     BELONGS WITH: 028 is the menu that gates the RS-232C jack, and Stage 2
//     cites it as "the menu-028 RS-232C gate" when explaining that this
//     radio enumerates TWO serial devices — the RS-232C jack behind this
//     menu, whose rate is menu 029 (559), and the built-in USB-to-dual-UART
//     bridge (46-47) the driver actually opens, whose rate is menu 031
//     (561). A reader who takes 028's option list at face value is reading
//     a list with a hole in it while deciding which port a user should set.
//
//   - 072 DATA PORT SELECT (layout 608) and 077 FM PKT PORT SELECT (613)
//     each begin their option list at "1:", with no "0:" entry. Two rows of
//     the same shape, and the only two in the chart that begin at 1.
//
//   - 100 RTTY SHIFT FREQ (layout 636) prints "1: 170 Hz 1: 200 Hz 2:
//     425 Hz 3: 850 Hz" — A DUPLICATE KEY AND NO "0:". Read literally the
//     row names four values against three keys. Transcribed as printed.
//
//   - 101 RTTY MARK FREQ (layout 637) begins at "1:" — "1: 1275 Hz
//     2: 2125 Hz" — the third row in the chart to do so, and unlike 072 and
//     077 it is not one of a matched pair.
//
//   - 116 SCP SPAN FREQ (layout 652) begins at "03:" — "03: 50 kHz
//     04: 100 kHz ..." — so its option list starts three keys in. No other
//     row in the chart begins above 01.
//
//   - 119, 125, 128 and 134 PRMTRC EQ (layout 655, 666, 669, 675) each open
//     their option list "00 : OFF", WITH A SPACED COLON, where their two
//     siblings of the same shape — 122 and 131 — open "00: OFF". This is a
//     TRANSCRIPTION TRAP rather than a chart defect about the radio: it says
//     nothing about any parameter, and its only consequence is that two
//     independent transcriptions could normalise it differently and report a
//     mismatch that is nobody's error. It is transcribed verbatim and never
//     respaced. See the evidence-base section above for why the comparison
//     could not exercise it.
//
// # Printed WIDTH defects outside the chart, and one command-letter defect
//
// Three further printing defects sit in the command blocks rather than the
// menu chart. None changes a byte this package builds; all three are the
// kind of thing a later reader would otherwise "fix".
//
// MW's P7 LEGEND IS TWO CHARACTERS WIDE AGAINST A ONE-POSITION FIELD. The
// MW block prints "P7 00: (Fixed)" (layout 1047) while its Set chart gives
// P7 exactly one cell (1046). The GRID is what this dialect follows —
// MWWriteKind is the single byte '0' — and the parser's refusal text says
// so. The MT block's own P9 legend is "00: (Fixed)" against a genuinely
// two-position field (1012, chart 1008), which is what a correct printing of
// this shape looks like.
//
// OI's P9 LEGEND IS THE MIRROR DEFECT: "P9 0: (Fixed)" (layout 1130), ONE
// character against the two cells its chart draws (1132), where MR's and
// MW's P9 legends both print "00: (Fixed)" (979, 1050) against the same two
// cells. OI is not a command this project emits; the defect is recorded
// because the OI and IF blocks carry the same 28-position field grid as MR
// and MW and are read alongside them.
//
// THE NA BLOCK'S FRAME CHARTS DRAW THE WRONG COMMAND LETTERS. NA (NARROW,
// layout 1068) prints its Set, Read and Answer frames as "M A P1 P2 ;",
// "M A P1 ;" and "M A P1 P2 ;" (1071, 1074, 1077) — the mnemonic of MA
// (MEMORY CHANNEL TO VFO-A, 902), whose own Set chart is "M A ;" (905). The
// availability list gives NA the columns O O O O (185) and MA the columns
// O X X X (172), so the two commands are distinct there and collide only in
// the charts. Neither is a command this project emits, and neither is
// modelled by core/cat; the defect is recorded because anyone deriving frame
// shapes mechanically from this manual's charts will meet it.
//
// # MD's rival CW spelling
//
// THIS MANUAL PRINTS TWO MODE LEGENDS AND THEY DISAGREE AT TWO NIBBLES. The
// five memory-command legends — MR 973-975, MT 1006-1008, MW 1044-1046, IF
// 789-791 and OI 1124-1126 — are identical to one another and print
// "3: CW" and "7: CW-R". The MD (OPERATING MODE) command's own legend
// (927-929) prints the same fourteen nibbles with "3: CW-U" and "7: CW-L"
// instead; every other index and label matches. Evidence leg G recorded both
// verbatim and declined to resolve them (testdata/mc-vectors.golden).
//
// THIS PACKAGE TRANSCRIBES THE MEMORY LEGEND, because the field this dialect
// encodes is the memory blocks' P6 and not MD's P2. MD is not a command
// core/cat builds. Which word the radio's own display uses is not knowable
// from this manual and is not settled here; if it ever matters, the MD
// legend is a second reading of the same nibbles and this note is where a
// reader finds it.
//
// # The mode fallback is WRONG for this radio
//
// cat.Mode.String() is a diagnostic fallback that reads core/cat's
// package-level table — the FT-710's — whatever radio the Mode came from,
// because a bare cat.Mode carries no dialect and fmt.Stringer gives it
// nowhere to receive one (core/cat/mode.go:160-182). Its doc comment
// already says "ANYTHING USER-VISIBLE MUST GO THROUGH Dialect.ModeName
// INSTEAD".
//
// THE FT-991A IS THE FIRST DIALECT FOR WHICH THAT FALLBACK IS ACTIVELY
// WRONG rather than merely unauthoritative. The FT-710's table names 'E'
// "PSK"; this radio's five legends name it "C4FM" (layout 1008). Every
// earlier divergence was a spelling of the same mode or a nibble the radio
// does not have; this one is a DIFFERENT REAL MODE, so a %v of a cat.Mode
// carrying an FT-991A channel's byte prints a mode the radio does not have
// and hides the one it does.
//
// Nothing in this package renders through the fallback, and nothing may: the
// route is Dialect().ModeName, which Stage 2's driver uses for everything
// that reaches a codeplug, the CLI listing and the GUI grid.
// TestModeStringFallbackIsWrongHere pins both halves — the fallback's "PSK"
// and this dialect's "C4FM" — plus the counter-example that the fallback is
// still RIGHT for the FTdx10, so the wrongness is a fact about this radio.
// A matching clause was added to Mode.String()'s own doc comment, whose
// closing sentence ("the whole point the moment a second dialect exists")
// this radio makes an understatement.
//
// # The ASSUMED register
//
// ELEVEN members of this dialect, of the codec it hands its records to, or
// of the driver Stage 2 builds on it, are NOT FT-991A-manual facts. They are
// inherited, structurally required or deduced; each is marked ASSUMED at its
// point of use where that point is in this package; and each is listed here
// so that the set has ONE statement of record for the radio, with the ONE
// Stage R capture that would lift it. core/driver/ft991a's own doc comment
// carries the same eleven when it exists (spec §The ASSUMED register); this
// is the copy the dialect is held to.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION. Every citation of this
// register elsewhere in the repository names the entry's field. A positional
// citation ("entry 6") is correct only until somebody adds or reorders an
// entry, and it then silently points at the wrong assumption rather than
// failing. dialect_test.go's TestASSUMEDRegisterIsComplete holds the two
// halves together mechanically: every entry named below that has a point of
// use in dialect.go carries an "ASSUMED" marker there, an entry added here
// without being tabled in that test fails it, and every entry must state its
// lifting capture.
//
// WHAT IS DELIBERATELY NOT AN ENTRY, and there are two, both of which a
// reader arriving from a sibling package would expect to find here.
//
// THE PMS PAIRS' NUMBERING, 100 to 117. Every sibling dialect carries an
// assumption about a special bank's numbering, because the FTdx10's and the
// FT-710's manuals print only "5xx (5MHz BAND)". This radio prints the
// numbers: "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U" (layout 916). So
// SlotSpace.PMSNumericLo is transcribed and belongs in dialect.go's
// citation, not in this register. The same goes for the two banks this radio
// does NOT have: "5xx", "5 MHz" and "EMG" appear in no slot legend of this
// manual, checked mechanically over the whole extraction, so SixtyLo/SixtyHi
// (0, 0) and EmergencyWire "" are transcribed absences and not guesses.
//
// SLOTS.MCSelects AND MT.ReadSlots, the two WIDE policy values. MC's legend
// gives the whole "001 - 117: Memory Channel Number" span (913) and MT's
// read legend the same (999), so both fields are CITATIONS OF A LEGEND. They
// are also currently INERT: the wide and narrow values differ only over the
// 60m and EMG banks, which this radio does not have, so they give identical
// verdicts on every wire form. That is a coincidence of this radio's slot
// space rather than a licence to declare either value, and
// TestDegeneracyPinWideAndNarrowAgree converts it into a checked fact that
// fails loudly if a bank is ever added.
//
//   - MTPolicy.TagFill = ' ' (dialect.go). The byte the FT-991A pads a
//     short memory tag with, in both directions: builds pad the outbound
//     P12 field to 12 bytes with it, and parses trim it from the answer.
//     Inherited from the FT-710, whose padding is spaces. The manual's P12
//     legend says only "TAG Characters (up to 12 characters) (ASCII)"
//     (layout 1017) and names no fill.
//     STAGE R LIFTS IT WITH: one MT Set of a tag SHORTER than 12 characters
//     to a memory channel, then an MT read of that channel — the bytes the
//     radio returns after the written characters ARE the fill. If they are
//     not spaces, this field changes and this package's golden padding bytes
//     change with it; if the field comes back short instead, the assumption
//     that failed is the answer's exact width, the next entry.
//
//   - THE COMBINED MT ANSWER'S EXACT LENGTH, 41 (consumed here as
//     MTAnswerBounds() = (41, 41), pinned by TestIdentityPinFrameGeometry).
//     Not a field of this dialect but an assumption it inherits from
//     core/cat's combined form (mtcombined.go's own ASSUMED-until-Stage-R
//     note): the manual's Answer grid draws the MAXIMAL frame (layout
//     1021-1033), and the FT-710 precedent — hardware accepting short MT
//     Sets against a maximal grid — makes a variable-width ANSWER live.
//     Evidence leg G counted the grid twice and got 41 both times
//     (testdata/mt-vectors.golden); what it counted is the PRINTED chart,
//     which is exactly the thing in question.
//     STAGE R LIFTS IT WITH: one MT READ of a channel carrying a tag SHORTER
//     than 12 characters, the raw answer captured whole. A 41-byte answer
//     confirms exactness; anything shorter converts the parser and the gate
//     to a recorded variable-width contingency.
//
//   - SlotSpace.NoneWire = "000" (dialect.go). The wire form of "no slot" —
//     the value an MR answer carries when the source is not a memory. It
//     appears in NO FT-991A slot legend: MC's gives 001-117 with its PMS
//     decomposition (layout 913-916), and MR's, MT's, MW's, IF's and OI's
//     give the bare span (966, 999, 1037, 783, 1117). It is the FT-710's
//     MR-answer fact, and cat.SlotSpace structurally requires a none form,
//     so one is supplied.
//     STAGE R LIFTS IT WITH: one MR read taken while the radio is on a VFO
//     rather than a memory — the P1 field of that answer is this radio's own
//     none form. Note the collision the field guards against: a radio
//     numbering memories from 000 would make "000" ambiguous, which is why
//     cat.NewDialect validates the two against each other rather than
//     assuming.
//
//   - THE cat.ModeUnset MEMBER OF THE MODE TABLE (dialect.go's modeNames).
//     The '0' = "-" placeholder. All five FT-991A mode legends run 1..9 then
//     A..E with no hole and no '0' member: MR's P6 at layout 973-975, MT's
//     at 1006-1008, MW's at 1044-1046, IF's at 789-791 and OI's at
//     1124-1126. It is included because parsers must accept the placeholder
//     — core/cat refuses to EMIT it in any Set frame, so its presence widens
//     only what this dialect can read — not because the manual names it.
//     STAGE R LIFTS IT WITH: one MR read of an EMPTY memory channel, one the
//     radio has never had written to. The P6 byte of that answer is what
//     this radio says for "no mode". If it is not '0', this member is wrong
//     rather than merely unattested, and the real byte replaces it.
//
//   - ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990
//     (dialect.go). ONE ENTRY, because ONE capture settles both and neither
//     is readable without the other. The manual prints "Clarifier Offset:
//     0000 - 9999 (Hz)" on every block carrying the field — IF 785, MR 968,
//     MT 1001, MW 1039, OI 1119 — and states NO STEP ANYWHERE. THE PRINTED
//     CEILING IS 9999, WHICH IS NOT A MULTIPLE OF THE INHERITED 10: this
//     dialect's 9990 is the largest multiple of the assumed step inside the
//     printed range, a deduction from the assumption rather than a
//     transcription, and a radio that really does step 1 Hz would reach a
//     9999 this dialect refuses to build.
//     STAGE R LIFTS IT WITH: one MR read of a channel whose clarifier has
//     been set to its MAXIMUM from the front panel. The magnitude that comes
//     back shows the ceiling directly (9999 or 9990), and the front panel's
//     last step below it shows the granularity — both halves off one
//     capture, which is why they are one entry.
//
//   - THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D
//     ('-'). Not a field of this dialect but an assumption it inherits from
//     core/cat's memory codec, which writes '-' into the P3 sign position
//     for a negative offset and accepts only '+' or '-' when reading one
//     (memdata.go). THIS MANUAL DOES NOT YIELD THE BYTE, and it is the worst
//     of the family for it: every block carrying the legend prints
//     "P3 Clarifier Direction +: Plus Shift, --: Minus Shift" with TWO
//     hyphens (IF 784, MR 967, MT 1000, MW 1038, OI 1118), where the FT-891
//     prints one. A doubled hyphen is exactly what a flattened en- or
//     em-dash looks like after extraction, so the printed glyph is not even
//     stable evidence for its own width, let alone for a byte value.
//     STAGE R LIFTS IT WITH: one IF or MR Answer captured from a channel
//     carrying a NEGATIVE clarifier offset, with position 15 dumped as a RAW
//     BYTE VALUE rather than rendered as text, since the rendering is
//     precisely what is in question. If it is not 0x2D, core/cat's sign
//     handling becomes dialect data rather than a shared constant.
//
//   - THE DCS STATES' SET ACCEPTANCE. Not a field: ToneStates is
//     cat.ToneStatesCTCSSAndDCS, TRANSCRIBED from the P8 legend, which
//     prints "3: DCS ENC/DEC" and "4: DCS ENC" against the MT and MW SET
//     charts as well as the read-side blocks (layout 795-796, 977-978,
//     1010-1011, 1048-1049, 1128-1129). What is assumed is the radio's
//     BEHAVIOUR: whether it accepts a DCS state written into a memory
//     without a DCS code having been set first is stated nowhere in this
//     manual. CN carries the code as a separate command (364-374, 419-446),
//     so the two are not written together by any frame this codec builds.
//     STAGE R LIFTS IT WITH: one MT Set carrying P8 '3' to an empty channel,
//     then an MT read of that channel. If the state comes back, the
//     vocabulary is accepted as written; if the write is refused or the
//     state reads back as '0', the driver's write path owes a CN ordering
//     this dialect currently does not model.
//
//   - ROW 087 RADIO ID'S EXCLUSION. Not a field of this dialect: the row is
//     excluded BY ADDRESS by the ft991a extable profile's
//     ParameterlessAddresses, which is why Dialect().EXItems() holds 152
//     items for a chart of 153 rows. The chart gives that row no width and
//     no parameter — its parameter cell is exactly ten hyphens and its
//     Digits cell one (layout 623) — and ten printed hyphens could be a
//     ten-character text field or an empty cell. Excluding it is the choice
//     that sends no frame; including it would have this dialect build an
//     "EX087;" whose answer it could not size.
//     STAGE R LIFTS IT WITH: one EX087; read. An answer settles the width
//     and the row rejoins the inventory; a "?;" settles the exclusion and
//     this entry closes.
//
//   - FRAMING: 8 DATA BITS, NO PARITY, TWO STOP BITS. Not a field of this
//     package at all. This manual carries no serial-framing statement
//     anywhere — no data-bit, parity or stop-bit sentence beside the CAT
//     RATE menus (559, 561) or the USB bridge description (46-47) — so
//     Stage 2's driver declares no SerialFramingReporter and the port opens
//     at core/transport's default stop bits BY ABSENCE rather than by a
//     claim. It is registered here because an absence that reaches the wire
//     is an assumption whatever the code calls it.
//     STAGE R LIFTS IT WITH: one session that answers "ID;" at 8-N-2 and one
//     that does not at 8-N-1. Either result is decisive, and the failing
//     half is the half that matters.
//
//   - DefaultBaud 38400. Not a field of this package: it is Stage 2's driver
//     capability. Menu 031 CAT RATE (layout 561) prints four rates —
//     4800/9600/19200/38400 — and marks NONE of them as the factory
//     setting. Menu 029 (559) prints the same four for a DIFFERENT PORT, the
//     RS-232C jack menu 028 gates (558), and is not a source for this field:
//     a user sent to 029 sets the wrong port's rate. No baud override exists
//     in the CLI or the GUI, so a wrong value here leaves a real FT-991A
//     reachable only through its own menu.
//     STAGE R LIFTS IT WITH: one first real-radio session — the factory CAT
//     RATE read off menu 031, or simply the rate at which the radio answers
//     "ID;" out of the box.
//
//   - THE ACKNOWLEDGEMENT CONVENTIONS. Not a field of this package: it is
//     what Stage 2's write path assumes about what a radio says when a frame
//     is accepted or refused. This manual describes no ACK/NAK vocabulary
//     beyond the "?;" every Yaesu CAT manual in this repository shows for a
//     rejected command, and it never says whether an accepted Set answers at
//     all.
//     STAGE R LIFTS IT WITH: one write session's RAW TRANSCRIPT, every byte
//     in both directions, including the silence after an accepted Set.
//
// # Reused-command verification
//
// Before this dialect reuses core/cat's codec for AI, ID, MC, MR, MW, MT and
// the EX read/answer grammar, each command's frame chart in this manual was
// checked against the shape that codec assumes. A deviation in the SHARED
// positions would be a STOP-and-respec, because core/cat's parsers and
// builders are fixed-offset: a frame one byte different is not a dialect
// parameter, it is a different codec.
//
// VERDICT: no STOP. The FT-991A deviates from the FT-710 family in exactly
// the declared axes — the EX address's width and shape, the PMS wire form
// and the slot number line it implies, the P8 state domain, byte 21's
// meaning and byte 28's — and in no other position of any shared frame.
// Command by command:
//
//   - AI (availability 130; frames 237-246). Set "AI P1 ;" and Answer
//     "AI P1 ;" four bytes, Read "AI;" three. Identical to the FT-710's.
//
//   - ID (availability 161; frames 771-779). No Set — the Set chart is
//     printed as an empty grid; Read "ID;"; Answer "ID P1 P1 P1 P1 ;" seven
//     bytes, counted by evidence leg G. Identical. The VALUE differs — 0670
//     here (layout 772) — but that is dialect data carried by CATID, not
//     frame shape, and is pinned as a difference by TestDifferencePinCATID.
//
//   - MC (availability 174; frames 912-921). Set and Answer
//     "MC P1 P1 P1 ;" six bytes, Read "MC;" three, all three counted by
//     evidence leg G. Frame shape identical; the SLOT DOMAIN is the whole
//     001-117 span, whose upper 18 are this radio's PMS pairs rather than a
//     second bank.
//
//   - MR (availability 178; frames 965-981). No Set, as core/cat assumes;
//     Read "MR P0 P0 P0 ;" six bytes; the Answer chart runs to 28 and
//     matches memdata.go's field block position for position. Identical save
//     for byte 21's MEANING (MemoryP5) and byte 24's DOMAIN (ToneStates).
//
//   - MW (availability 183; frames 1036-1051). Set only: the Read and
//     Answer LABELS are printed (1048, 1051) but THE GRIDS BENEATH THEM ARE
//     EMPTY, which agrees with the availability row's "O X X X" and with
//     core/cat's mw.go. The Set frame is the same 28-position chart as MR's
//     Answer under an "MW" prefix. P7 reads "00: (Fixed)" against one cell,
//     which MWWriteKind carries as the single byte the grid draws.
//
//   - MT (availability 181; frames 998-1033). The Set/Answer chart runs to
//     41: the 28 shared positions, P11 at 28, a 12-character P12 tag at
//     29-40, ';' at 41, which is core/cat's mtCombinedLen() at a 12-byte
//     tag. Read "MT P0 P0 P0 ;" six bytes. Identical save for byte 28's
//     MEANING (MTPolicy.P11). THIS MANUAL DOES NOT CONTRADICT ITSELF ABOUT
//     MT, unlike the FT-891's: the availability row gives MT "O O O X"
//     (181) and its own detail block prints a Read chart and a full Answer
//     chart, which agree. There is therefore no analogue here of that
//     model's ErrMTReadRejectedForOccupiedSlot, and Stage 2 must not invent
//     one.
//
//   - EX read/answer grammar (availability 155; frames 519-528). Read
//     "EX P1 P1 P1 ;" is SIX bytes, the narrowest in the family, and the
//     Answer is those three address digits followed by a variable parameter
//     and the terminator. THIS IS THE ONE SHARED-FRAME LENGTH THAT MOVES,
//     and it is what cat.EXAddressForm exists to carry; core/cat derives
//     both lengths from Dialect.EXAddressWidth() rather than from a
//     constant, so the shape is a parameter of the codec rather than a fork
//     of it. The Set and Answer charts print an OPEN-ENDED run ("~ P2 ;")
//     and have no countable length; the answer's upper bound is DERIVED from
//     this dialect's own widest inventory item, which
//     TestEXAnswerBound proves.
package ft991a
