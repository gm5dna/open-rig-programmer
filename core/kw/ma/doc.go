// SPDX-License-Identifier: GPL-3.0-or-later

// Package ma is the MA-family memory codec: the TS-890S's and the TS-990S's
// MA0 memory-channel grid, their outbound gate, and both rows' EX menu
// inventories.
//
// IT IS A SIBLING OF core/kw, NOT A LAYER ON TOP OF IT. core/kw owns the
// Kenwood ENVELOPE — the framing adapter, the accumulator, the acknowledgement
// tokens, the typed error family, the EX address and item types — and it also
// owns a 50-byte MR/MW memory RECORD that neither of these radios has. This
// package imports the first and reimplements nothing of it; it declares its
// own Layout, its own Command and its own record because the MA0 grid is 40
// to 50 bytes on one row and 57 on the other, has no hard-wired byte at all,
// and belongs to two radios that have no MR, MW, MC or TY command between
// them. No ma.Layout is a kw.Layout, and none is ever passed to core/kw's
// conformance suite; imports_test.go is the structural half of that.
//
// # The registers this file is authoritative for
//
// The ASSUMED register A1-A22 below is the authoritative one for this
// milestone: what is assumed, why the documents do not settle it, and the ONE
// observation that would lift it. Lift IDs are L-HW-n (a wire observation),
// L-DOC-n (a named document read at a named place) and L-DEC-n (a decision).
// EVERY ENTRY IS PER REGISTRY ROW UNLESS IT NAMES ONE: this design has two
// rows, in two different books, and therefore needs two radios. An entry
// scoped to the manufacturer would let one radio's session retire the other
// row's assumption, and register_test.go refuses that wording outright.
//
// The errata schedule E1-E19 records what the two documents get wrong, in
// three categories kept distinct because two of the nineteen rows are not
// defects at all. The A4 capability matrix's own findings against the design,
// M-E1 to M-E9, follow as plan-level rulings, each naming where it lands.
//
// Citations are 890:NNNN for
// docs/fixtures-private/manuals/ts890s_pc_rev1_layout.txt and 990:NNNN for
// ts990s_pc_rev2_layout.txt. Every one carried into this package was
// re-derived from those texts.
//
// # ASSUMED register
//
//	A1  TS-990S ONLY: the channel-name field is padded with ASCII SPACE to
//	    its fixed ten-byte window on write, and trailing spaces are not part
//	    of the name on read. Unsettled because that book prints "Up to 10
//	    digits" against a FIXED 10-byte window (990:2955-2956) — "up to" and
//	    a fixed window cannot both be literal — and states no pad byte (E13).
//	    The strongest in-book support is KY's "Characters that are left blank
//	    will be filled with spaces", printed in both books (890:2910-2911,
//	    990:2788-2789), which is about the CW keying buffer and not the
//	    memory name, and which on the 890S sits in a block with its own
//	    defect (E6).
//	    THE 890S HALF IS NOT AN ASSUMPTION AND IS NOT HERE, AND THE CLAUSE IS
//	    REVERSED (C-MED-1, adjudication overturning the T8 MED-1 ruling, both
//	    rounds): its terminator floats after the name (890:3181-3182,
//	    confirmed on the printed p.41 render), so "...AB ;" and "...AB;" are
//	    DISTINCT, unambiguous frames — the space is real content, not a pad
//	    byte a fixed window would absorb. This row's codec therefore carries
//	    P13 VERBATIM in BOTH directions: buildMA0Set890 emits the name as-is
//	    with no pad, and parseMA0Answer890 calls checkName directly (not
//	    parseName, which stays TS-990S ONLY) and applies no trim. Parse ∘
//	    Build is then the identity on the 890S for every name this design
//	    admits, including one ending in a space, and the outbound gate admits
//	    every frame buildMA0Set890 produces with no special pleading. Verbatim
//	    is lossless whichever way the real hardware turns out to behave: if a
//	    real 890S pads the field on write, the pad round-trips unchanged; if
//	    it does not, verbatim is exact either way — so the hardware lift below
//	    is a curiosity about the radio's own behaviour, not a correctness
//	    question for this codec.
//	    Lift: L-HW-1, observing the MA0 family — write a 3-character name to
//	    a scratch channel, read it back, record the ten bytes exactly.
//	    TS-990S. (On the 890S the same write/read, with a trailing-space
//	    name, would settle only whether a real radio pads the field; this
//	    codec's own behaviour does not depend on the answer.)
//
//	A2  A memory name may contain the printable-ASCII characters this design
//	    writes: the KY charset's members plus the digits and letters, all
//	    inside 0x20-0x7E and excluding ';'. Both books give a global coding
//	    rule — ASCII, with 80h-FFh remapped by Menu 9-01 (890:31-43,
//	    990:32-39) — and neither prints a charset for the memory name. The
//	    ';' exclusion is FORCED by the envelope (890:92-96) and is not an
//	    assumption. Narrowed from draft 1, which claimed the whole 0x20-0x7E
//	    domain while its lift observes three characters: the entry claims
//	    only what a passing lift would establish, and the upper bound at 0x7E
//	    remains open.
//	    Lift: L-HW-2, observing the MA0 family — write a name containing '@',
//	    '/' and a high-ASCII character and read back what survives. A pass
//	    narrows the doubt to those three; it does not sweep the domain.
//	    (ii) THE OUTBOUND WIRE REFUSES 0x00-0x1F on every frame this codec
//	    builds for either radio, through core/kw's envelopeAllows, which
//	    NewFramingFor keeps in front of this package's own gate. The
//	    prohibition is printed by the TS-480 alone ("Do not use the control
//	    characters 00 to 1Fh", 480:127-129); NEITHER of these two books
//	    prints it in any form, so applying it here is this design's choice,
//	    not a read. It is KEPT because the failure direction is safe — an
//	    over-cautious Allow, never a malformed frame reaching hardware — and
//	    for that reason it has NO LIFT: there is no wire trial that would
//	    relax it.
//
//	A3  An MA0 Set to an unassigned channel cannot create it. MA2, MA3, MA6,
//	    MA4 and MA7 all print an unassigned-channel prohibition (890:3265,
//	    890:3282, 890:3300-3301, 890:3324, 890:3356; 990:3008, 990:3023,
//	    990:3037-3038, 990:3058); MA0 prints NOTHING either way; and MA1
//	    prints the opposite for its own case (890:3239-3241, 990:2989-2991).
//	    So a create path exists and only MA0's own participation is unknown.
//	    Lift: L-HW-3 — THE GATE for one-frame writes, hardware item 1.
//	    Observing the MA0 family: Set a channel confirmed blank from the
//	    front panel, then read it back. Per row.
//
//	A4  On the 890S, a blank channel's P13 window is blank too, so a blank
//	    answer is 40 bytes. The note covers "parameters P2 to P12"
//	    (890:3215-3216) and stops; the 990S's equivalent covers "P2 to P18"
//	    (990:2962-2963), which is what makes the omission visible rather than
//	    a reading.
//	    Lift: L-HW-4, observing the MA0 family on a TS-890S ONLY — read a
//	    channel confirmed blank and record the frame length and every byte.
//
//	A5  THE ANSWER DIRECTION ONLY, and this is a RECORDED NARROWING rather
//	    than an assumption of the whole field: the MA family spells the
//	    channel number back as three zero-padded ASCII digits, never
//	    space-padded. The SET direction is NOT assumed — both books print the
//	    Set grid with three P1 cells over a domain of "000 ~ 119"
//	    (890:3166-3182, 990:2893-2915), which DOCUMENTS what this programme
//	    emits. What no book prints is what the radio ANSWERS with, and a
//	    space convention exists elsewhere in this manufacturer's own charts.
//	    THE FAILURE DIRECTION IS SAFE: a space-padded answer MISSES the
//	    prefix matcher and times out; it cannot be mis-attributed to another
//	    channel.
//	    Lift: L-HW-5, observing the MA0 family — read channel 007 and record
//	    bytes 4-6, which is exactly what this narrowed entry claims.
//
//	A6  "Blank", where the MA0 notes use it, means ASCII space 0x20. Neither
//	    MA0 block defines it; both books define it for QR — "this setting is
//	    space" (890:4362-4363) and "this setting is blank <0x20>" (990:4082).
//	    THE PREDICATE IS ALL-OR-NOTHING OVER THE WINDOW (isBlankWindow: all
//	    space OR all '0'); a MIXED window — spaces in some fields, '0' in
//	    others — is NOT covered and routes to the field parse, where one such
//	    slot aborts the whole read. This is a residual risk, not a defect: no
//	    per-field predicate is derivable from either book, and inventing one
//	    would be the guess this design refuses.
//	    Lift: L-HW-4, the same read.
//
//	A7  On the 890S, MA0 P8 on a Programmable VFO channel does NOT carry that
//	    slot's end frequency. Wholly unprinted: MA6 sets the end frequency as
//	    a separate command (890:3315-3323) and MA0's only note about the
//	    split side is scoped to "a single memory channel" (890:3217-3218).
//	    Lift: L-HW-6, observing the MA0 family — read a registered
//	    Programmable VFO (e.g. 100) and compare P8 against the end frequency
//	    the front panel shows. TS-890S.
//
//	A8  On the 990S, MA0 P9 ("frequency 2") on a section-defined channel does
//	    NOT carry that slot's end frequency. As A7; MA6 is the end-frequency
//	    command here too (990:3051-3059).
//	    Lift: L-HW-6, the same observation. TS-990S.
//
//	A9  An MA0 read of a slot in class 100-109 answers at all. Both books say
//	    only that such a slot cannot be SET while it is being read
//	    (890:3213-3214, 990:2960-2961); neither says what a read returns.
//	    Lift: L-HW-6, the same session.
//
//	A10 Slots 110-119 (E0-E9) are memory channels of the same record shape as
//	    000-099 — ALL TEN of them. The number mapping is the only thing
//	    either book says about them, verified exhaustively (E18, M-E6). The
//	    claim is universal over the class, so THE LIFT IS WIDENED RATHER THAN
//	    THE CLAIM NARROWED: one E0 read would establish E0 and nothing else,
//	    and a class of ten slots nobody has explained is not a class to
//	    generalise from one member.
//	    Lift: L-HW-8, observing the MA0 family — read ALL TEN slots 110-119
//	    on a radio where at least E0 has been written from the front panel,
//	    and record each answer's length and shape.
//
//	A11 Default baud is 9600. Neither book prints a factory value; both print
//	    only the selectable list (890:13-14, 990:13-14). THIS CANNOT BE A
//	    WIRE OBSERVATION — reading it over EX presupposes a session at the
//	    very baud in question.
//	    Lift: L-DOC-1 (the instruction manual's baud menu page) or L-DEC-1
//	    (Stuart).
//
//	A12 Neither radio echoes the host's own frames. Wholly unprinted on both,
//	    as on pair 1's two rows; core/kw's NoteSent is a no-op and this pair
//	    inherits it.
//	    Lift: L-HW-9, observing "AI0;" — send it and record whether it comes
//	    back. Per (row, path): USB-B and COM on each radio, four legs.
//
//	A13 The FV answer is a fixed 7-byte frame carrying four characters FOR
//	    EVERY FIRMWARE VERSION EITHER RADIO HAS SHIPPED. Both books draw a
//	    four-cell P1 and print one example, "1.00" (890:2657-2659,
//	    990:2533-2536). A version needing five characters does not fit the
//	    printed grid, and nothing says what the radio would then send. THE
//	    CLAIM IS UNIVERSAL OVER VERSION STRINGS AND ITS LIFT IS NOT: one
//	    radio at one other version establishes that version alone, so the
//	    entry stays open after a pass and is narrowed to the versions
//	    observed.
//	    Lift: L-HW-10, observing "FV;" on a radio whose front panel shows a
//	    version other than 1.00. A pass records THAT version; it does not
//	    lift the universal claim.
//
//	A14 The 990S's MA0 P2 may be emitted as '0' on a Set. Printed "ignored …
//	    Enter a dummy value" (990:2901-2903), which names no value. THIS IS
//	    THE MILESTONE'S ONLY DEFAULTED BYTE.
//	    Lift: L-HW-11, observing the MA0 family — Set with P2='0' and again
//	    with P2='9', reading back each time. TS-990S.
//
//	A15 Frequency range. Not printed anywhere in either PC-command document.
//	    As pair 1's A17, and with the same consequence: MinFreqHz/MaxFreqHz
//	    of 0/0 disable codeplug.Validate's bound checks altogether, which is
//	    a live gap in the shipping programme.
//	    Lift: L-DOC-2 (either radio's instruction manual, general
//	    specifications).
//
//	A16 NOT ASSUMED — DOCUMENTED, retained for safety. Reading a single
//	    memory channel zeroes the second frequency's parameters. Printed on
//	    both: 890:3217-3218 and 990:2964-2965. Kept in the table because the
//	    whole of decision 9's narrower refusal rests on it, and an entry a
//	    reader can find is better than a fact a reader must re-derive.
//	    NO LIFT NEEDED: it is what the books print.
//
//	A17 The 890S's minimum MA0 frame is 40 bytes (a zero-character name).
//	    Derived from "Up to 10 characters" plus the floating 'x'
//	    (890:3181-3182, confirmed on the p.41 render). No worked short frame
//	    is printed anywhere in the book.
//	    Lift: L-HW-4, the same blank-channel read.
//
//	A18 An MA0 Read needs no preceding MN. Both Read forms carry their own
//	    channel number (890:3186, 990:2918), so the command is stateless on
//	    its face — but MA1, MA7 and MI all act on "the channel appointed when
//	    using this command", so the family has state and MA0 Read's
//	    independence from it is not printed. DECISION 5'S REMOVAL OF MN FROM
//	    THE ROSTER PROMOTES THIS ENTRY RATHER THAN WITHDRAWING IT: with no MN
//	    builder the driver could not select a channel even if it had to, so
//	    A18 carries every read in the milestone. If A18 is false, no read
//	    works at all.
//	    Lift: L-HW-12, hardware item 10, observing the MA0 family — from VFO
//	    mode, with no MN sent, read channel 005.
//
//	A19 The EX answer's P5 never exceeds 15 characters. Both books print
//	    widths per item class — 3 normally, 4 for PF keys, 8 for the 990S's
//	    frequency settings, 0-15 for a power-on message, 0-10 for
//	    screen-saver text (890:1917-1921, 990:1746-1752) — and neither prints
//	    a general ceiling. Independent support: the 990S's own Answer diagram
//	    is drawn to byte 24, which is 2+5+1+15+1 (990:1738-1747), and that
//	    diagram's disagreement with the note beside it is E19.
//	    Lift: L-DOC-3 (the transcription leg's own boundary ledger, derived
//	    from the rendered chart).
//
//	A20 An MA0 Set produces no answer while AI is off. INFERRED from the
//	    family's siblings and printed nowhere for MA0. MA2, MA3 and MA6 each
//	    print "when the AI function is ON" (890:3266-3267, 890:3283-3284,
//	    890:3327-3328; 990:3009-3010, 990:3024-3025, 990:3062-3063); MA0's
//	    OWN BLOCK IS SILENT (its notes are 890:3211-3221 and 990:2958-2965)
//	    and, unlike those three, MA0 HAS an Answer form. Consequence if
//	    false: an unmatched frame the engine counts — benign, which is why
//	    the entry is cheap to hold. It is also why a driver reports an MA0
//	    Set as Sent and never as Confirmed: reporting Confirmed on silence
//	    would be asserting this entry as a fact.
//	    Lift: L-HW-3's existing session (hardware item 1), which already
//	    sends an MA0 Set and reads back — record whether anything arrived
//	    unprompted.
//
//	A21 TS-890S ONLY: a blank channel carries no meaningful content in its
//	    P13 name window, so ignoring P13 in the empty predicate discards
//	    nothing. The blank-channel note stops at P12 (890:3215-3216, E4)
//	    where the 990S's covers P2-P18 (990:2962-2963). A4 assumes the window
//	    is blank; A21 IS THE SEPARATE ASSUMPTION THAT A RESIDUE, IF ONE
//	    EXISTS, IS NOT CHANNEL CONTENT — and it is the one the design acts
//	    on, because clone.ReadAll abandons the whole read on any channel
//	    error, so refusing such a frame would make the radio unreadable end
//	    to end. The recorded behaviour is already the safe one: report
//	    unassigned, carry the residue in the read's detail. THE SAME STOP AT
//	    P12 ALSO LEAVES THE BLANK FRAME'S LENGTH UNPRINTED, not only P13's
//	    content (A17 assumes the length, 40 bytes with a zero-character
//	    name); the empty predicate does not depend on it, testing the printed
//	    P2-P12 window alone.
//	    Lift: L-HW-4, observing the MA0 family on a TS-890S ONLY — the same
//	    blank-channel read that lifts A4, recording bytes 40 onwards exactly.
//
//	A22 The fake's complete MA0 record images are compositions of printed
//	    VALUES, not records any radio has produced. Every byte value in them
//	    is printed, but NO LITERAL COMPLETE MA0 ANSWER APPEARS IN EITHER
//	    BOOK, so the combination is this project's. PER ROW: the 890S's image
//	    and the 990S's are two compositions.
//	    Lift: a complete real MA0 answer from that row, which hardware item 2
//	    (L-HW-1) produces as a by-product of its read-back. Lifts for that
//	    row alone.
//
// # Errata schedule
//
// Nineteen rows in three categories. Seventeen are document defects; ONE is a
// cross-book naming divergence that is a defect of neither book; ONE is a
// transcription trap that is not a defect at all. The categories are kept
// distinct because "correcting" either of the last two into evidence is
// exactly what recording them prevents.
//
// Seven of the nineteen — E1, E2, E6, E10, E11, E14, E17 — concern commands
// this milestone does not build. They are recorded because a later milestone
// that builds them must not re-derive them, and because two of them (E6, E14)
// are load-bearing for something this milestone DOES do. Three bear on what
// this milestone builds: E16 settles the 890S's ParameterlessPolicy, E18 is
// why slot class 110-119 is unreached, and E19 is why the EX answer parser
// accepts two 990S frame shapes rather than one.
//
// SEVENTEEN DOCUMENT DEFECTS:
//
//	E1  890S. MA7's P2 is labelled "11 digit END frequency in Hz" — a paste
//	    from MA6, whose P2 really is an end frequency. MA7 is the TEMPORARY
//	    CHANGE frequency of the channel currently being displayed.
//	    890:3343 vs 890:3320, and 890:3336, 890:3347-3350.
//
//	E2  890S. MA7's SET DIAGRAM HAS NO P1: the row reads "M A 7 P2 P2 …",
//	    although P1 is declared "Target frequency for read and answer (Read /
//	    response only)" and the Read and Answer rows both carry it.
//	    890:3340 vs 890:3337, 890:3347, 890:3351.
//
//	E3  BOTH. Channel nomenclature printed two ways ON EACH RADIO: "P0 ~ P9"
//	    / "E0 ~ E9" in the MA blocks against "P00 ~ P09" / "E00 ~ E09" in MI
//	    and MN. 890S: 890:3168-3169 vs 890:3597-3598, 890:3650-3651. 990S:
//	    990:2895-2896 vs 990:3269-3271, 990:3333-3334.
//
//	E4  890S. MA0's blank-channel note covers "parameters P2 to P12", leaving
//	    P13 (the channel name) unspecified. The 990S's equivalent note covers
//	    "P2 to P18". 890:3215-3216 vs 990:2962-2963.
//
//	E5  890S. CN's frequency table heads its index column P2, although the
//	    command's only parameter is P1 and MA0 P7 refers to "the P1 value of
//	    the CN command". CN's parameter head prints "P1 (CTCSS frequency)"
//	    and all three command forms are "C N P1 P1 ;", but the chart body's
//	    four index columns are headed P2 — the 990S's shape, where P1 really
//	    is Main/Sub and P2 is the index. The 890S's own TN chart is clean,
//	    heading its columns P1, so the defect is CN-only and asymmetric: a
//	    paste from the 990S, the same class and severity as E17. THE ONE
//	    PLACE A LITERAL READING OF A CHART HEADER WOULD PRODUCE A WRONG
//	    BUILDER. CN is not on the outbound roster, so nothing this milestone
//	    builds touches it; the record matters because MA0 P7 points AT this
//	    chart, and because a later tone-command milestone must not read a
//	    Main/Sub P1 into an 890S command that has none. THIS ENTRY ABSORBS
//	    THE MATRIX'S M-E9, which is the same defect.
//	    890:1349 (parameter head), 890:1352 (Set row and the P2-headed chart
//	    body), 890:1360 (Answer row), 890:1352-1353, 890:3190 (MA0 P7's
//	    reference); 890:5147 (TN's clean P1 header); the 990S's real form at
//	    990:1241-1246.
//
//	E6  890S. KY's notes describe P1 values "spaces" and "2" against a P1
//	    legend of "0: Character buffer space / 1: No character buffer space".
//	    KY is not a command this milestone builds; recorded because the same
//	    block carries the book's only statement that a fixed-length text
//	    field is space-filled, which is A1's strongest support and must not
//	    be quoted without its defect.
//	    890:2909, 890:2914 vs 890:2885-2886; the support sentence at
//	    890:2910-2911.
//
//	E7  990S. MI's note says "The end frequency is set using the MA7
//	    command". This radio has no MA7 — its family stops at MA6 — and MA6
//	    is the command meant. An un-renumbered paste from the 890S.
//	    990:3281 vs 990:3051, 990:3060-3061.
//
//	E8  990S. Scan lockout has TWO ENCODINGS ON FACING PAGES: MA0 P17 is "1:
//	    OFF / 2: ON" and MA3 P2 is "0: OFF / 1: ON". A driver that
//	    "helpfully" accepted 0 in MA0's field would be importing MA3's
//	    convention into it. 990:2952-2954 vs 990:3020-3021.
//
//	E9  990S. MA0 P10 refers a mode legend to "the P1 value of the OM
//	    command"; OM P1 is the Main/Sub band selector and the legend is P2.
//	    P4 — the same field for frequency 1 — cites P2 correctly.
//	    990:2929-2931 vs 990:3699-3710, 990:2907-2910.
//
//	E10 990S. QA P3 and P5 refer a mode legend to "P2 of the MS command"; MS
//	    is "Transmission Audio Entry Sound Generator Selection" and its P2 is
//	    a microphone-input on/off flag. QA is not a command this milestone
//	    builds. 990:4019, 990:4029 vs 990:3389-3397.
//
//	E11 990S. MA2 names only the "P00 ~ P09" mapping and omits "E00 ~ E09",
//	    which the same radio's MA0 and MN both print.
//	    990:3001 vs 990:2895-2896, 990:3333-3334.
//
//	E13 990S. MA0 P18 is "Channel Name (Up to 10 digits.)" in a FIXED 10-byte
//	    window, while MA2 P3 for the same datum is "10 digit channel name".
//	    "Up to" and "10 digit" describe one field two ways and neither states
//	    the pad byte — which is why A1 exists.
//	    990:2955-2956 vs 990:3006.
//
//	E14 890S. MA2's P2 is labelled "Unused (1 digit)" and then "Always a
//	    space". A mandatory literal space in an outbound Set is not "unused",
//	    and the label invites a builder to emit '0'. MA2 is not a command
//	    this milestone builds; recorded because EX P4 has the same shape and
//	    IS built. 890:3260-3261; the 990S's cleaner wording at 990:3002-3003;
//	    EX P4 at 890:1912-1913.
//
//	E16 890S. Four Advanced Menu rows printed "Does not correspond to a
//	    command" carry REAL ADDRESSES — 1 00 23 Touchscreen Calibration, 1 00
//	    24 Software License Agreement, 1 00 25 Important Notices concerning
//	    Free Open Source, 1 00 26 About Various Software License Agreements —
//	    so the chart prints four addresses the radio answers with an error
//	    ("Entering a number that cannot be set also causes an error to
//	    occur"). A fifth row, Firmware Version, is printed "1 — 27" with a
//	    literal em dash for P2 under the note "P2 is any value", which no
//	    two-digit address form can express. THE 990S PRINTS THE SAME FOUR
//	    NOTICE ROWS WITH AN EM DASH IN ALL THREE ADDRESS COLUMNS and has no
//	    "1 — 27" row at all.
//	    890:2273-2280, 890:2281, 890:2283, 890:1909-1911 vs 990:2272-2279.
//
//	E17 990S. CN's READ FORM OMITS P1: the Set row is "C N P1 P2 P2 ;" and
//	    the Answer row is "C N P1 P2 P2 ;", both carrying the Main/Sub P1,
//	    but the Read row is "C N ;" — where the same book's TN Read is "T N
//	    P1 ;". CN is not a command this milestone builds; recorded so a later
//	    milestone that builds tone commands does not re-derive it.
//	    990:1249 vs 990:1245, 990:1253; TN Read at 990:4958.
//
//	E18 BOTH. E0-E9 ARE NEVER EXPLAINED. Slots 110-119 are mapped to E0-E9 in
//	    every MA parameter list and nowhere else: an exhaustive search of
//	    both documents returns EIGHT SITES IN THE 890S AND THREE SITES (five
//	    output lines) IN THE 990S, every one the same number-mapping sentence
//	    inside a parameter list. No block, note or chart says what an E
//	    channel is for, on either radio. Recorded as a documentation defect
//	    rather than only in prose, because it is the reason a whole slot
//	    class is unreached (A10). Counts corrected from an earlier nine/five
//	    on the matrix's finding, M-E6: the 990S figure was a LINE count
//	    reported as a site count.
//	    890:3169 and its seven siblings (890:3259, 890:3277, 890:3294,
//	    890:3298, 890:3311, 890:3598, 890:3651); 990:2896 and its two
//	    (990:3270-3271, 990:3333-3334).
//
//	E19 990S. The EX Set/Answer diagram is drawn FIXED to 24 bytes — the
//	    position ruler holds P5 to exactly 15 bytes (positions 9-23) and
//	    nails ';' to 24 — although the SAME chart's own P5 note is the
//	    variable-length one both books print (3 digits normally, 4 for PF
//	    keys, 8 for a frequency setting, 0-15 for a power-on message, 0-10
//	    for screen-saver text), so the diagram and the note beside it
//	    disagree. The 890S's diagram for the identical field is honest about
//	    that variability: its position ruler reads "9~" and the terminator's
//	    header cell is the letter "x", never a number. DESIGN CONSEQUENCE:
//	    this package's 990S ParseEXAnswer accepts BOTH the fixed-24 padded
//	    form the diagram draws and a shorter, unpadded form, pinned both ways
//	    — a builder reading only the diagram would pad every 990S EX Set to
//	    24 bytes, which is wrong for every item class narrower than the
//	    widest.
//	    990:1719-1732 (Set), 990:1738-1747 (Answer), 990:1742-1756 (the
//	    note); 890:1898-1904 (Set), 890:1909-1913 (Answer).
//
// ONE CROSS-BOOK NAMING DIVERGENCE THAT IS A DEFECT OF NEITHER BOOK:
//
//	E12 BOTH. The same slot class is named two different things by the two
//	    books: "Programmable VFO" (890S) and "Section defined Memory channel"
//	    (990S), for slots 100-109 with identical semantics. NOT A DEFECT OF
//	    EITHER BOOK ALONE — a naming divergence a fleet-wide slot vocabulary
//	    must not smooth over. layout.go uses one kw.SlotClass for the class
//	    on both rows and cites this entry at the site, so the divergence is
//	    recorded rather than encoded.
//	    890:3315-3319 vs 990:3051-3054.
//
// ONE TRANSCRIPTION TRAP THAT IS NOT A DEFECT:
//
//	E15 890S. NOT A DEFECT — a transcription trap. Several terminator cells
//	    render in the layout text as a full-width Japanese semicolon
//	    immediately followed by an ASCII ';'. It is ONE terminator; the
//	    doubling is a font/extraction artefact. Recorded so no transcriber
//	    reads a two-byte terminator into evidence.
//	    890:3786, 890:3789, 890:3792, 890:3804, 890:2642.
//
// # The A4 capability matrix's rulings
//
// Nine findings the matrix raised against the design, each ruled at plan
// level and each naming where it lands.
//
//	M-E1 spec.Banks is one BankMemory per row, "000"-"099", no scan bank, and
//	     100-119 published nowhere. Lands in each driver package's caps.go.
//
//	M-E2 Tone direction and semantics are ASSUMED per row: the 590 book's IF
//	     P14 sentence is absent from both of these books and neither has an
//	     IF command block at all. Register home is K-D1, one entry per driver
//	     doc.go, lifted by that radio's instruction manual or a wire trial.
//
//	M-E3 Data mode is INSIDE the mode legend — no DA command, no data byte —
//	     so FieldDataMode is Unsupported on both rows and the data-ness is
//	     carried by the mode NAMES ("LSB-D" on the 890S; "LSB-D1"/"D2"/"D3"
//	     on the 990S). Lands in each driver's caps.go and in the published
//	     model documentation.
//
//	M-E4 The full 27-Field table governs, not the design's thirteen-row
//	     neutral-model table. "erase" is Unsupported (the standing no-erase
//	     rule, over a PRINTED MA5) and "data_mode" per M-E3. Lands in each
//	     driver's bank field map, with coverage by spec.AllFields().
//
//	M-E5 The design supplies no radio-level counterpart for five absence
//	     cells. Four (clarifier, attenuator, preamp, antenna) have no row in
//	     the design's table at all — that absence is M-E4; the fifth (filter)
//	     already reads record-scoped and is correct. What was missing on all
//	     five is the POSITIVE content: the radio-level command, per row, with
//	     a cite — RT/XT, FL0-FL3, RA, PA, AN. Lands in each driver's caps.go
//	     comments and in the published model documentation.
//
//	M-E6 The E-channel census is EIGHT sites on the 890S and THREE on the
//	     990S, not nine and five. Lands in E18 above; its evidential force is
//	     unchanged.
//
//	M-E7 Slots 100-119 are ABSENT from the model, never codeplug.Unavailable
//	     — no channel is ever produced for one, so no FieldState is ever set.
//	     FieldTxFrequency is graded ONCE, in MEM, for 000-099. Lands in each
//	     driver's caps.go and in its write path's slot rung.
//
//	M-E8 The Unknown-TX-disposition refusal is FILE-SIDE ONLY: one MA0 answer
//	     carries both frequencies and the split flag, so a fresh read makes
//	     the disposition Known by construction. It can fire only on a channel
//	     from a loaded file or a CHIRP import, and its pin must construct
//	     one — a fresh-read fixture would make the rung vacuous. Lands in
//	     each driver's write ladder.
//
//	M-E9 The 890S's CN chart heads its index column P2 although the command
//	     carries only P1. NOT A NEW ERRATUM: it is E5 above, whose own
//	     citation set covers the same defect and adds the reason it matters
//	     here — MA0 P7 refers to "the P1 value of the CN command". Lands
//	     INSIDE E5, not as a row of its own.
//
// # The driver register lives elsewhere
//
// K-D1 (the tone-mode semantics, and with them the tone_tx/tone_rx
// assignment) and K-D2 (the control lines' state at open) are DRIVER-register
// entries. They land in core/driver/ts890/doc.go and core/driver/ts990/doc.go,
// WRITTEN TWICE — once per package, each with that row's own lift — because a
// single shared entry would be the sibling inheritance the per-row rule exists
// to prevent. They are not in the register above, and register_test.go refuses
// a copy of one here: correcting an A-number is a DESIGN change and correcting
// a K-number is a DRIVER-PACKAGE change, and no task may quietly move one for
// the other.
//
// # Design commitments
//
// THE "E;" AND "O;" ROUTE IS inherited from core/kw unchanged. Both are
// stream-health tokens rather than answers, both end the STREAM through
// core/kw's transport.FatalFramer hook, and each carries THAT BOOK'S own
// cause sentence — "A receive buffer overrun error occurred" for "O;" on both
// of these radios (890:121-123, 990:121). Nothing in this package touches
// that path; NewFramingFor delegates to core/kw's own constructor and
// overrides only the outbound gate.
//
// MA0 AND EX ARE THE ONLY VARIABLE-LENGTH FRAMES THIS PACKAGE PARSES, and
// MA0's length is a RANGE rather than unbounded: 40 to 50 bytes on the 890S
// (the floating terminator of 890:3181-3182, with A17's derived minimum) and
// a FIXED 57 on the 990S (990:2915). That is why the 890S's answer matcher is
// a prefix comparison with a length range rather than kw.PrefixLenMatcher's
// unbounded branch, which core/kw's own doc.go reserves and warns about: a
// bare unbounded matcher would correlate a two-hundred-byte run of noise that
// happened to start with the right six bytes.
//
// NEITHER MA0 GRID HAS A SINGLE HARD-WIRED BYTE, which is decision 7 and is
// stated because a reader coming from pair 1 will look for one. Pair 1's
// layout type governs thirteen printed-fixed bytes on one of its rows and
// sixteen on another, and its whole defaulted-byte register class exists for
// them. Re-derived here: every byte of the 890S's forty-to-fifty and every
// byte of the 990S's fifty-seven carries a live P-number with a printed
// domain. The one byte that comes close is the 990S's P2 — "the memory
// channel type is decided while setting the P9 and P10 values, so this
// parameter is ignored. Enter a dummy value" (990:2901-2903) — which is not
// fixed but IGNORED, and what to emit for it is A14. So this milestone's
// entire defaulted-byte list is one item on one row, and the refusal class
// pair 1 needed for its hard-wired bytes is empty here.
package ma
