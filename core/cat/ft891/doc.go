// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft891 holds the Yaesu FT-891's CAT dialect: its EX menu
// inventory, transcribed from the manual's menu chart, and the
// cat.DialectConfig literal that binds it to the shared codec in core/cat.
// It is DATA ONLY — no driver, no fake, no registration, no session, no
// wire. The package cannot register itself with the application:
// SupportedModels derives solely from internal/wiring's driver table.
//
// # Provenance
//
// Everything here comes from the Yaesu FT-891 CAT Operation Reference Book,
// revision 1909-C (docs/fixtures-private/manuals/ft891_cat_1909-C.pdf,
// SHA-256 59e2295177633b970408bec10aae62f697c3bde5fe48a3be25282d992bc3bbb0,
// 20 PDF pages, and its layout extraction ft891_layout.txt — both
// gitignored, so the line references throughout this package are citations,
// not links; the file's own provenance note is
// docs/fixtures-private/manuals/ft891-manual-provenance.md). The untitled
// menu chart that follows the EX command block spans layout lines 524-697;
// the EX inventory transcribed from it lives in table2.csv, whose header
// carries the transcription conventions and the chart's verbatim defects.
// The memory-frame blocks this dialect's other axes are read from are MR at
// 959-979, MT at 996-1027 and MW at 1034-1050, with IF at 773-792 and OI at
// 1118-1138 as the two further blocks that carry the same 28-position field
// grid.
//
// NO FT-891 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project, and none
// is available to it. Every statement in this package is a reading of a
// manual. There is no observation CSV, no corrections file and not a single
// captured frame, which is why the ASSUMED register below exists at all and
// why nothing here may be quoted as verification.
//
// # Chart printing defects
//
// The chart's own defects are recorded here because an undocumented defect
// is one somebody later silently "corrects". They are RECORDED, NOT
// RESOLVED: this repository has no FT-891 to ask which reading is right.
// table2.csv's provenance header carries the same list against the rows it
// transcribes; this is the dialect's copy of record.
//
//   - 0905 RPT SHIFT 50MHz (layout 609) PRINTS Digits 1 AGAINST ITS OWN
//     "0 - 4000 kHz (P2= 0000 - 4000) (10 kHz/step)" LEGEND, which needs
//     four. Its twin 0904 RPT SHIFT 28MHz (608) prints 4 for a legend of the
//     same shape. This one is first because it is the one the cross-check
//     CANNOT CATCH: transcriptions A and B both record the printed 1, and
//     they agree, so three-way agreement is not evidence about this row. It
//     is the only row in the chart whose Digits contradicts its own printed
//     range — checked mechanically over every row carrying a "P2= x - y"
//     range — and the printed 1 is what the inventory carries.
//
//   - 1507 EQ3 FREQ (657) and 1516 P-EQ3 FREQ (668) SPLICE A RANGE INTO AN
//     OPTION LIST with no space after the hyphen: "... 06: 2000 Hz -18:
//     3200 Hz", where the intent is plainly "06: 2000 Hz - 18: 3200 Hz".
//     Transcribed as printed — not respaced, not expanded into the twelve
//     options it stands for. The same splice, spaced, is the chart's house
//     style on the ten cut-filter rows (table2.csv's quirk 6), so a
//     consumer that parses an option list must not read either as three
//     options.
//
//   - 0710 CW WAVE SHAPE (589) prints "1: 2 msec 2: 4 msec" — an option
//     list with NO "0:" ENTRY, the only such row in the chart. Whether the
//     radio's P2 domain starts at 1 or the chart lost a line is not knowable
//     from this manual, and nothing here fills the gap in.
//
//   - 1504 EQ2 FREQ (653) and 1513 P-EQ2 FREQ (664) print "04: 1000Hz" with
//     NO SPACE between the number and the unit, while every neighbouring
//     option in the same row is spaced ("03: 900 Hz", "05: 1100 Hz").
//
//   - THE CW MEMORY ROWS' "TEXT" OPTION IS NOT A TEXT ITEM. 0407-0411 CW
//     MEMORY 1-5 (543-547) print "0: TEXT 1: MESSAGE" with Digits 1: "TEXT"
//     NAMES one of two enumerated choices, it does not make the parameter
//     textual. This chart has no free-text row at all — there is no "(up to
//     N characters) (ASCII)" anywhere in it — which is why the ft891
//     extable profile declares TextRowsAbsent with TextWidth 0, and why a
//     text row in this CSV is refused by the parser rather than transcribed.
//     Recorded as a defect-shaped TRAP rather than a defect: a reader
//     skimming for the FTdx10's MY CALL. row will find these five and must
//     not promote them.
//
// # The MT contradiction: the command list against the detail block
//
// THIS MANUAL CONTRADICTS ITSELF ABOUT WHETHER MT CAN BE READ, and the
// contradiction is recorded here rather than resolved.
//
// The command availability list gives MT "MEMORY WRITE & TAG" the columns
// Set O, Read X, Answer X (layout 166) — Set only. Its own detail block
// prints a Read chart ("M T P0 P0 P0 ;", layout 1016) and a full 41-position
// Answer chart (1018-1027), in the same block, on the same page. Both
// cannot be true. Every registered sibling's list says O O O (the FTdx10's
// at ftdx10_layout.txt:261, the FTdx101's at ftdx101_layout.txt:334), so
// this radio is the only one of the four whose two records disagree.
//
// IT IS NOT THIS PACKAGE'S PROBLEM TO SETTLE, and this package deliberately
// does not try. A dialect describes the vocabulary the documents print;
// BuildMTRead exists here and is domain-limited by MTPolicy.ReadSlots to the
// slots MT's own legend names (memory and PMS). WHICH OF THE TWO RECORDS
// THE RADIO HONOURS IS THE DRIVER'S QUESTION, and the design's answer lives
// there: core/driver/ft891 reads memory and PMS by MT with a per-slot MR
// cross-check, and an MT "?;" on a slot MR shows to be occupied becomes a
// typed whole-session refusal that names this contradiction rather than
// diagnosing it (spec §Stage 2 item 2, ErrMTReadRejectedForOccupiedSlot).
// The 5xx and EMG banks are read by MR alone, which no reading of either
// record contradicts.
//
// # The ASSUMED register
//
// EIGHT members of this dialect, or of the codec it hands its records to,
// are NOT FT-891-manual facts. They are inherited, structurally required or
// deduced; each is marked ASSUMED at its point of use where that point is in
// this package; and each is listed here so that the set has one statement of
// record, with the ONE Stage R capture that would lift it.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION. Every citation of this
// register elsewhere in the repository names the entry's field. A positional
// citation ("entry 6") is correct only until somebody adds or reorders an
// entry, and it then silently points at the wrong assumption rather than
// failing. dialect_test.go's TestASSUMEDRegisterIsComplete holds the two
// halves together mechanically: every entry named below that has a point of
// use in dialect.go carries an "ASSUMED" marker there, and an entry added
// here without being tabled in that test fails it.
//
// WHAT IS DELIBERATELY NOT AN ENTRY: THE 5 MHz BANK'S NUMBERING. SixtyLo
// 501 and SixtyHi 510 sit on the FTdx10's and the FT-710's registers as
// assumptions, because those manuals print only "5xx (5MHz BAND)". This one
// PRINTS THE NUMBERS — "501 - 510 (5 MHz, U.S. and U.K. version only)", MR's
// slot legend at layout 962, repeated by IF at 776 and OI at 1122 — so the
// bounds here are transcribed and belong in dialect.go's citation, not in
// this register (spec §S0 evidence base; the region condition is a fact
// about which unit is in front of you, which discovery answers, not a
// dialect one).
//
//   - MTPolicy.TagFill = ' ' (dialect.go). The byte the FT-891 pads a short
//     memory tag with, in both directions: builds pad the outbound P12 field
//     to 12 bytes with it, and parses trim it from the answer. Inherited
//     from the FT-710, whose padding is spaces. The manual's P12 legend says
//     only "TAG Characters (up to 12 characters) (ASCII)" (layout 1017) and
//     names no fill.
//     STAGE R LIFTS IT WITH: one MT Set of a tag SHORTER than 12 characters
//     to a memory channel, then an MT read of that channel — the bytes the
//     radio returns after the written characters ARE the fill. If they are
//     not spaces, this field changes and this package's golden padding bytes
//     change with it; if the field comes back short instead, the assumption
//     that failed is the answer's exact width, the next entry.
//
//   - THE COMBINED MT ANSWER'S EXACT LENGTH, 41 (consumed here as
//     MTAnswerBounds() = (41, 41), pinned by TestIdentityPinMTAnswerBounds).
//     Not a field of this dialect but an assumption it inherits from
//     core/cat's combined form (mtcombined.go's own ASSUMED-until-Stage-R
//     note): the manual's Answer grid draws the MAXIMAL frame (layout
//     1018-1027), and the FT-710 precedent — hardware accepting short MT
//     Sets against a maximal grid — makes a variable-width ANSWER live. No
//     vector this package holds is an answer, so nothing here establishes
//     answer width.
//     STAGE R LIFTS IT WITH: one MT READ of a channel carrying a tag SHORTER
//     than 12 characters, the raw answer captured whole. A 41-byte answer
//     confirms exactness; anything shorter converts the parser and the gate
//     to the recorded 30..41 window contingency.
//
//   - SlotSpace.NoneWire = "000" (dialect.go). The wire form of "no slot" —
//     the value an MR answer carries when the source is not a memory. It
//     appears in NO FT-891 slot legend: MR's gives 001-099, P1L-P9U,
//     501-510 and EMG (layout 960-964), MT's and MW's only 001-099 and
//     P1L-P9U (998-999, 1035-1036), MC's the same (907-909). It is the
//     FT-710's MR-answer fact, and cat.SlotSpace structurally requires a
//     none form, so one is supplied.
//     STAGE R LIFTS IT WITH: one MR read taken while the radio is on a VFO
//     rather than a memory — the P1 field of that answer is this radio's own
//     none form. Note the collision the field guards against: a radio
//     numbering memories from 000 would make "000" ambiguous, which is why
//     cat.NewDialect validates the two against each other (V7) rather than
//     assuming.
//
//   - THE cat.ModeUnset MEMBER OF THE MODE TABLE (dialect.go's modeNames).
//     The '0' = "-" placeholder. All three FT-891 mode legends run 1..9 then
//     B, C, D with a printed hole at A and no '0' member: MR's P6 at layout
//     972-974, MT's at 1007-1010, MW's at 1043-1046. It is included because
//     parsers must accept the placeholder — core/cat refuses to EMIT it in
//     any Set frame, so its presence widens only what this dialect can read
//     — not because the manual names it.
//     STAGE R LIFTS IT WITH: one MR read of an EMPTY memory channel, one the
//     radio has never had written to. The P6 byte of that answer is what
//     this radio says for "no mode". If it is not '0', this member is wrong
//     rather than merely unattested, and the real byte replaces it.
//
//   - ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990
//     (dialect.go). ONE ENTRY, because ONE capture settles both and neither
//     is readable without the other. The manual prints "Clarifier Offset:
//     0000 - 9999 (Hz)" on every block carrying the field — MR 967, MT 1003,
//     MW 1040, IF 781, OI 1126 — and states NO STEP ANYWHERE. THE PRINTED
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
//     (memdata.go). THIS MANUAL DOES NOT YIELD THE BYTE. Unlike the
//     FTdx10's, it is at least self-consistent about the glyph — every block
//     carrying the legend prints "P3 Clarifier Direction +: Plus Shift, -:
//     Minus Shift" with a single hyphen (MR 966, MT 1002, MW 1039, IF 780,
//     OI 1125) — but a rendered glyph in a PDF is not a byte value, and the
//     extraction that produced ft891_layout.txt is where any en-dash would
//     have been flattened to an ASCII hyphen without trace.
//     STAGE R LIFTS IT WITH: one IF or MR Answer captured from a channel
//     carrying a NEGATIVE clarifier offset, with position 15 dumped as a RAW
//     BYTE VALUE rather than rendered as text, since the rendering is
//     precisely what is in question. If it is not 0x2D, core/cat's sign
//     handling becomes dialect data rather than a shared constant.
//
//   - THE COMBINED ANSWER'S P7 READ DOMAIN. core/cat's combined parser
//     tolerates '0' (VFO) and '1' (Memory) in the P7 position of an MT
//     answer. THE FT-891 PRINTS NO READ VOCABULARY FOR MT's P7 AT ALL: its
//     MT block gives "P7 0: (Fixed)" (layout 1011) and nothing else, where
//     the FTdx10's prints "Set: 0: (Fixed) / Read: 0: VFO 1: Memory"
//     (ftdx10_layout.txt:1230). This radio's MR block does print "P7 0: VFO
//     1: Memory" (layout 976), which is the only reason the tolerated pair
//     is the pair it is — an inference ACROSS COMMANDS, not a legend. The
//     spec ruled tolerance on READ and strictness on SET rather than adding
//     an axis (§S0.6): a strict parse against an unprinted domain would
//     refuse every MT answer if the radio answers like its sibling's Read
//     legend, and refusing a real answer is the worse failure.
//     STAGE R LIFTS IT WITH: one MT read of an occupied memory channel, P7
//     read off the raw answer. Whatever byte is there IS the read domain,
//     and if it is neither '0' nor '1' this tolerance is wrong rather than
//     merely wide.
//
//   - THE MC ANSWER DOMAIN BEYOND MEMORY AND PMS. Slots.MCSelects is
//     MCSelectsMemoryPMS, transcribed from the MC block's own legend (layout
//     907-909), and it governs the SEND direction only: cat.ParseMCAnswer
//     keeps the full readable space, so this dialect will ACCEPT an MC
//     answer naming a 501-510 or EMG channel. THE MC BLOCK DESCRIBES NO
//     ANSWER DOMAIN — it prints one legend, against Set — so that acceptance
//     is an assumption, made because a radio put on a 5 MHz channel from the
//     front panel will answer with it however narrow its Set domain is, and
//     because narrowing the parse side would turn a legitimate answer into
//     an error. Nothing here widens the SEND side: BuildMCSet of a 5xx or
//     EMG slot is refused, and so is such a frame at the gate.
//     STAGE R LIFTS IT WITH: one MC read taken while the radio is on a 5 MHz
//     channel selected from the front panel. The P1 field of that answer
//     settles whether MC reports the bank at all — and if it reports
//     something else entirely, this tolerance is replaced by what it reports
//     rather than kept.
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
// VERDICT: no STOP. The FT-891 deviates from the FT-710 family in exactly
// the five places Stage 0 turned into declared axes — the EX address's
// width, MC's send domain, MT's read domain, byte 21's meaning and byte
// 28's — and in no other position of any shared frame. Command by command:
//
//   - AI (availability 117; frames 226-235). Set "AI P1 ;" and Answer
//     "AI P1 ;" four bytes, Read "AI;" three. Identical to the FT-710's.
//
//   - ID (availability 147; frames 762-770). No Set; Read "ID;"; Answer
//     "ID P1 P1 P1 P1 ;" seven bytes. Identical. The VALUE differs — 0650
//     here (layout 763) — but that is dialect data carried by CATID, not
//     frame shape, and is pinned as a difference by
//     TestDifferencePinCATID.
//
//   - MC (availability 160; frames 906-915). Set and Answer
//     "MC P1 P1 P1 ;" six bytes, Read "MC;" three. Frame shape identical;
//     the SLOT DOMAIN is the narrower one (Slots.MCSelects).
//
//   - MR (availability 164; frames 959-979). No Set, as core/cat assumes;
//     Read "MR P0 P0 P0 ;" six bytes; the Answer chart runs to 28 and
//     matches memdata.go's field block position for position. Identical
//     save for byte 21's MEANING (MemoryP5).
//
//   - MW (availability 167; frames 1034-1050). Set only, no Read and no
//     Answer, exactly as core/cat's mw.go assumes; the Set frame is the same
//     28-position chart as MR's Answer under an "MW" prefix; the P1 legend
//     is restricted to memory and PMS, which is the FT-710's MW restriction
//     too. P7 reads "0: (Fixed)", which MWWriteKind carries.
//
//   - MT (availability 166 — the contradiction recorded above; frames
//     996-1027). The Set/Answer chart runs to 41: the 28 shared positions,
//     P11 at 28, a 12-character P12 tag at 29-40, ';' at 41, which is
//     core/cat's mtCombinedLen() at a 12-byte tag. Read "MT P0 P0 P0 ;" six
//     bytes. Identical save for byte 28's MEANING (MTPolicy.P11) and the
//     read's slot domain (MTPolicy.ReadSlots).
//
//   - EX read/answer grammar (availability 142; frames 513-522). Read
//     "EX P1 P1 P1 P1 ;" is SEVEN bytes, not nine, and the Answer is those
//     four address digits followed by a variable parameter and the
//     terminator. THIS IS THE ONE SHARED-FRAME LENGTH THAT MOVES, and it is
//     what cat.EXAddressForm exists to carry; core/cat derives both lengths
//     from Dialect.EXAddressWidth() rather than from a constant, so the
//     shape is a parameter of the codec rather than a fork of it.
package ft891
