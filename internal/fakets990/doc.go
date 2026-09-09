// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets990 simulates a Kenwood TS-990S's PC-control behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double this
// registry row's own layers run against: the transport engine,
// core/driver/ts990, the CLI's --fake mode and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakets590 for the
// TS-590 pair.
//
// ONE FAKE, ONE REGISTRY ROW. Unlike internal/fakets590, which serves two
// siblings out of one book, this package serves one radio out of one book and
// therefore has no Row argument and no per-row table. Its sibling
// internal/fakets890 is a SEPARATE PACKAGE for the same reason the two driver
// packages are separate: the two books print different memory grids (a fixed
// 57 bytes here, 40 to 50 there), different mode legends, different lockout
// encodings and two disjoint menu charts, and a shared table would let one
// radio's evidence stand in for the other's.
//
// # The hard rule: NOTHING project-internal
//
// fakets990 MUST NOT import any package of this project — not core/kw, not
// core/kw/ma, not core/codeplug, not core/spec, and not internal/fakets890 or
// any sibling fake. Standard library only, in every non-test file, in this
// directory AND every directory beneath it. Every byte offset, field width and
// validation rule below is re-derived from the TS-990S PC Control Command
// Reference Guide (rev 2, January/30/2019) — cited "990:NNNN" throughout, the
// same convention core/kw/ma/doc.go uses, naming where a chart is rather than
// linking to it, because the manual itself is gitignored
// (docs/fixtures-private/manuals/).
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/kw/ma's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake has
// to be able to DISAGREE with the production codec for a test against it to
// mean anything, and it can only disagree if it was built from the manual
// rather than from the code.
//
// The fence is enforced mechanically and recursively (imports_test.go): this
// directory and every one beneath it.
//
// THE ONE EXCEPTION IS internal/fakepipe. It carries the net.Pipe pair, the
// goroutine bookkeeping, the interruptible latency wait and the raw write, and
// it is permitted because it is PROTOCOL-FREE — it sees []byte and a duration
// and nothing else, so it carries no framing, no field layout and no reply
// building. A bug in it cannot make a wrong codec look right; it can only stop
// bytes moving, which this package's own tests notice at once. Everything
// above the wire — the reassembler, the parser, the image, the replies — stays
// here, written independently.
//
// # A SIBLING of internal/fakets890, not a refactor of it
//
// This package duplicates a good deal of that one's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options, the transcription-B
// projection. The duplication is deliberate and it is not going to be factored
// into a shared "fake core" package: a shared helper would be a
// project-internal import, which the hard rule above forbids in every fake, so
// the only way to share code would be to abandon the property that makes any
// of them worth having.
//
// And the two radios do not in fact agree. Where this fake diverges from its
// nearest Kenwood sibling, the divergence is this book's:
//
//   - THE MA0 GRID IS EIGHTEEN PARAMETERS IN A FIXED 57-BYTE FRAME
//     (990:2893-2915), where the 890S's is thirteen in 40 to 50 bytes with a
//     FLOATING terminator. Nothing about the two field maps is shared, and a
//     byte read at the sibling's offset lands in the wrong parameter here.
//   - THE SECOND FREQUENCY IS NOT A SPLIT SIDE. This chart prints "frequency
//     1" and "frequency 2" (990:2906, 990:2928) with a SEPARATE P15 saying
//     whether the channel is split (990:2947-2948) and a P16 saying whether it
//     receives on both (990:2950-2951); the 890S's second side IS its split
//     transmission and it has no dual-reception field at all.
//   - SCAN LOCKOUT IS 1/2, NOT 0/1. "1: Scan Lockout OFF / 2: Scan Lockout
//     ON" (990:2952-2954) — and this book's own MA3 prints 0/1 for the same
//     function on the facing page (990:3020-3021), which is erratum E8.
//   - THE MODE LEGEND IS OM's TWENTY-FOUR VALUES, 0-9 and A-N
//     (990:3707-3730), where the 890S's prints sixteen.
//   - THE CHANNEL NAME SITS IN A FIXED TEN-BYTE WINDOW (990:2955-2956) rather
//     than after a floating terminator, so a name is never shorter than the
//     field that holds it.
//   - THE EX ANSWER IS DRAWN TO A FIXED TWENTY-FOUR BYTES (990:1738-1747),
//     where the 890S's terminator floats under a ruler head printed "x". That
//     is erratum E19 and it is what ex.go's fifteen-wide P5 window is.
//
// # WHERE THE EX INVENTORY COMES FROM
//
// The widths table ex.go answers from is NOT hand-typed. It is projected at
// init, by exinventory.go, from this package's OWN COPY of TRANSCRIPTION B —
// transcription-b-990s.csv beside this file, with PROVENANCE.md recording
// where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of this row's two-source cross-check:
//
//   - the CODEC's inventory (core/kw/ma/exinventory990s_gen.go) is generated
//     from TRANSCRIPTION A (core/kw/ma/menu990s.csv) by internal/extable;
//   - THIS inventory is projected from TRANSCRIPTION B by exinventory.go,
//     which imports nothing project-internal at all;
//   - core/transport/ex_crosscheck_ts990_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined
// agent with no repository access and no sight of A, the page ledger or any
// row count. So a mis-read row in either transcription, or a defect in either
// parser, surfaces as a cross-check MISMATCH rather than as two tables quietly
// agreeing on the same wrong number.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF.
//
// ONE CORRECTION IS APPLIED IN THE PROJECTION, and it is not an edit to the
// evidence. Transcription B is frozen and is not touched; what exinventory.go
// does is fold it under a fact printed in this book:
//
//   - RULING R-B, THE PF KEY WIDTH. B reads three digits on all EIGHTEEN PF
//     key rows of this chart and A reads four; the EX block's own P5 note
//     settles it — "PF key settings use 4 digits (refer to the PF Key
//     assignment ID lists)." (990:1747-1748) — and the list it refers to, the
//     PF Key Assignment Lists (990:2287 onwards), prints every allotment ID
//     as four digits, so the book settles the width twice. The leg is FROZEN
//     EVIDENCE, so the error is corrected in the projection and never in the
//     CSV; the projection consults neither transcription A nor
//     internal/extable, and it REFUSES to apply the correction unless every
//     one of the eighteen is present and still carries three — a leg that has
//     moved is an arbitration, not a correction to apply blind. A fake
//     answering a width the book contradicts would be modelling the radio
//     wrongly on a point the book settles, which is not what an independent
//     evidence leg is for; it is the SAME CLASS AS internal/fakets890's
//     FOUR-ADDRESS EXCLUSION: a fact this book prints that the leg does not
//     carry, applied at this side from an independently written list.
//
// THE RUN IS THIS CHART'S OWN, 0/00/15 to 0/00/32 (990:1793-1810): the 990S
// inserts Voice (Main Band) and Voice (Sub Band) at items 17 and 18, so its
// run is EIGHTEEN rows where the 890S's is seventeen ending at 0/00/31.
//
// AND THERE IS NO SECOND CORRECTION HERE. internal/fakets890's projection also
// excludes four Advanced Menu rows its chart prints with the body "Does not
// correspond to a command" (890:2273-2280, erratum E16); this book prints no
// such row, so this projection excludes nothing and the fake's inventory is
// transcription B's addresses exactly.
//
// # What this fake deliberately does NOT model
//
// THE EX (MENU) SET. The EX READ is modelled — ex.go, from this package's own
// copy of transcription B — and the Set is not. The book prints one
// (990:1721-1732), and core/kw/ma builds none either: the Set and the Answer
// share an identical wire shape, so admitting the Set would admit a captured
// answer being written back. A Set-shaped body therefore falls through
// handleEX's read check to "?;". That is a MODELLING GAP, KNOWN-DIVERGENT from
// the documented grammar, and it is not a claim that a TS-990S refuses EX Set.
// TestEXRead_ASetShapedBodyIsRefused pins the gap's shape.
//
// THE EX ANSWER'S SHORTER FORM. This chart's Answer diagram and the P5 note
// beside it disagree (erratum E19), and core/kw/ma's parser admits BOTH the
// fixed twenty-four-byte frame the diagram draws and a shorter one carrying
// the row's own printed width. THIS FAKE PRINTS THE DIAGRAM'S FORM ONLY. A
// fake models one radio's behaviour, not both readings of a chart, and the
// diagram is this radio's own picture of its own answer; the parser's other
// arm is pinned on the codec's side, where the ambiguity lives.
//
// EVERY MA COMMAND EXCEPT MA0. The family runs MA0 to MA6 (990:2891-3059) and
// the design's outbound roster admits MA0 alone, in both directions; MA1's
// direct entry, MA2's channel name, MA3's scan lockout, MA4's copy, MA5's
// deletion and MA6's section-defined end frequency are all off it, as are MI,
// MN and MV. They fall through the dispatcher to "?;", which is NOT a claim
// that the radio lacks them — several of them are how a real TS-990S creates a
// channel at all, which is why the create path is a published cost of this
// registry row rather than a gap this fake hides.
//
// THE ERASE PATH. MA5 is the deletion command and it is not built anywhere in
// this programme — a standing rule of this repository — so nothing here can
// remove a channel over the wire. WithEmptyChannel does it at construction
// instead, by deleting a map entry, which triggers the fake's existing
// documented blank-channel answer and introduces no new behaviour.
//
// FAULT INJECTION BEYOND THE BOOK. There is no dropped-reply, garbled-byte,
// spurious-frame or chunked-write option here. Those exercise
// core/transport.Engine, one model-independent implementation already covered
// against internal/fakeradio's fault suite, and nothing is learnt by running
// them past a second dialect. What IS modelled is what THIS BOOK prints: the
// two stream-error tokens (990:118-121), and the transient "?;" suppression
// its own error table flags (990:114-116).
//
// A FRONT PANEL, AND ANY NOTION OF A SELECTED CHANNEL. This radio's MA0 Read
// carries its own channel number (990:2918), so there is no selection for a
// host to move and none for a panel to move behind its back. That is the
// design's A18 — the load-bearing read assumption of the whole milestone — and
// this fake is built to it: nothing here depends on a preceding MN, because
// nothing here has any state an MN could set.
//
// FIRMWARE VERSIONS. This book dates several menu rows and one AI value to a
// firmware version — "Parameter 4 is supported from firmware version 1.20."
// (990:189), "<Firmware version 1.20 or later>" throughout the menu chart —
// and this fake models none of that. It answers the one FV example the book
// prints and serves every address transcription B carries, which is the same
// posture the codec's inventory takes.
//
// THE LAN SURFACE. The book documents a TCP/IP path with its own ##CN
// connection command (990:19-67). This programme speaks serial, and nothing
// here models a socket.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something the TS-990S PC Control Command
// Reference Guide does not settle is listed here, and each entry appears as an
// inline comment beside the code that implements it. Entries are cited BY
// NAME, never by number: renumbering is an editorial act and must not break a
// citation. TestASSUMEDRegisterIsComplete holds both halves of that promise
// mechanically — the roll and the headings must agree, and every entry must
// have a point of use outside this file.
//
// The register's SUBJECT is this fake's behaviour. The DESIGN's own register —
// A1..A22, each with a named lift — lives in the milestone's design document
// and is carried in the repository by core/kw/ma/doc.go; where an entry below
// exists because a design entry is unlifted, it says which one, but correcting
// an A-number is a design change and correcting an entry here is a change to
// this package.
//
//  1. AN ACCEPTED SET PRODUCES NO REPLY. MA0's own notes (990:2958-2965) say
//     nothing about an acknowledgement, where MA1, MA2, MA3 and MA6 each print
//     "When the AI function is ON, a response is provided by the MA0 command"
//     (990:2992, 990:3009, 990:3024, 990:3062) — so the family's siblings
//     print what MA0's block does not. That is the design's A20, and this fake
//     answers nothing at all to an accepted Set. The silence is the SAME code
//     path as "this handler has nothing to say", deliberately, so no handler
//     can acknowledge a Set by accident. What a real radio does on the wire
//     after an accepted MA0 Set has not been observed by this project.
//
//  2. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in parser.go
//     — the channel number's spelling, the two eleven-digit frequencies, the
//     two mode nibbles, the two FM width flags, the two tone types, the four
//     two-digit tone indices, the split flag, the dual-reception flag, the
//     scan lockout and the name window's charset — is ASSUMED to be what the
//     radio itself enforces. The book prints the legends; it never says what a
//     radio does with a Set that leaves one. Nothing here NORMALISES: a byte
//     outside its legend is refused, never quietly corrected, because a driver
//     that sent one would otherwise pass its own tests and fail on hardware.
//     NOTHING ACROSS FIELDS IS CHECKED EITHER: unlike the 890S's book, which
//     instructs a host to keep its two FM width flags equal on a split channel
//     (890:3219-3221), this one prints no cross-field rule at all.
//
//  3. THE ANSWER'S CHANNEL NUMBER IS THREE ZERO-PADDED DIGITS. The chart
//     prints one domain, "000 ~ 119" (990:2894), over three P1 cells in all
//     three directions and describes no space form anywhere — unlike the
//     TS-590's MC, whose own chart prints "a space is entered for a channel
//     number less than 100". So what a TS-990S ANSWERS with is unprinted, and
//     this fake spells it zero-padded. That is the design's A5, and its
//     failure direction is safe: a space-padded answer MISSES the codec's
//     prefix matcher and times out; it cannot be mis-attributed to another
//     channel.
//
//  4. THE SET'S P2 IS IGNORED AND THE ANSWER'S CLASS FOLLOWS FREQUENCY 2. Two
//     printed sentences and one assumed split (parser.go's classFor). PRINTED:
//     the memory channel type "is decided while setting the P9 and P10 values,
//     so this parameter is ignored. Enter a dummy value." (990:2901-2903, the
//     design's A14), and "when reading a single memory channel, all parameters
//     for frequency 2 become 0" (990:2964-2965, A16). So a Set's P2 is
//     discarded, and an all-zero frequency 2 is what "0: Single Memory
//     channel" answers with. ASSUMED: that the OTHER class a Set can produce
//     is "1: Dual Memory channel" rather than "2: Section defined Memory
//     channel". The book never gives the rule separating the two; what it does
//     say is that a section is registered with MA1 or MI (990:3060-3061), not
//     with MA0. A fake that stored the dummy byte instead would answer a class
//     the radio's own sentence says it discards.
//
//  5. THE NAME WINDOW IS TEN BYTES, CARRIED VERBATIM. P18 is printed "Channel
//     Name (Up to 10 digits.)" against a FIXED ten-cell window
//     (990:2955-2956) — erratum E13, since "up to" and a fixed window cannot
//     both be literal — and this fake builds to the RULER, because that is the
//     frame's geometry where the sentence is about a value. It neither pads
//     nor trims: a Set always carries ten bytes, so there is nothing to pad,
//     and the design's A1 (space padding on write, trailing spaces not part of
//     the name on read) is the CODEC's and the DRIVER's rule, on the side that
//     has to turn a window into a name. A fake that applied A1 too would apply
//     it on both sides of every round trip and could never contradict it.
//
//  6. A SET CARRYING AN "UNUSED" MODE NIBBLE IS STORED. The OM P2 legend
//     prints nibbles 0 and 8 as "Unused" (990:3707, 990:3715) and nothing says
//     what a Set carrying one does. This fake stores the nibble rather than
//     inventing a refusal, which is what lets a test drive core/kw/ma's own
//     build refusal, and the driver's blank-channel reading, against a real
//     fake.
//
//  7. TONE INDICES ARE STORED, NOT RANGE-CHECKED. The TN chart prints "00 ~
//     50" plus a set-only 99, the CN chart "00 ~ 49" plus the same, and each
//     prints "Entering a value that does not exist is invalid" for ITS OWN
//     COMMAND (990:4973, 990:1266-1267). Whether either rule holds inside an
//     MA0 frame is unprinted, so only the field's SHAPE is enforced here, on
//     all FOUR of this grid's index windows. Refusing would assert a fact
//     about the radio and would put the codec's own refusal out of reach of a
//     real fake.
//
//  8. A SET TO A BLANK CHANNEL IS STORED. Whether MA0 alone can CREATE a
//     channel is the design's A3 — every other member of the family prints an
//     unassigned-channel prohibition and MA0's block prints nothing either way
//     — and core/driver/ts990 REFUSES such a write on that ground. A fake that
//     refused it here would assert A3 as a fact about the radio AND would put
//     the driver's own refusal out of reach of a real fake.
//
//  9. SLOTS 100-119 ARE NOT SERVED. The chart prints the domain "000 ~ 119"
//     and maps 100-109 to P0-P9 and 110-119 to E0-E9 (990:2894-2896), and that
//     number mapping is the ONLY thing this book says about either class: what
//     a Programmable VFO slot answers to an MA0 read is unprinted (the design's
//     A9), and what an E channel IS is never explained anywhere in the document
//     (erratum E18, three sites here, every one the same sentence). Both
//     classes are published in NO bank of this registry row, so there is no
//     slot ID for an image to represent. This fake serves 000-099 and refuses
//     100-119 through the same out-of-domain path as any other number — one
//     refusal path, no special case. IT IS A DELIBERATE NARROWING OF THE
//     PRINTED DOMAIN, not a claim that a TS-990S refuses those channels, and
//     the day A9 and A10 lift this entry is what a reader has to change.
//
//  10. THE DEFAULT IMAGE'S RECORD COMPOSITION. Every BYTE of every shipped
//     record is a printed constant, one of the three printed example
//     frequencies (990:86-89, 990:344-345, 990:2352), a printed legend value, a
//     character from the printed KY set (990:2770-2778), or A6's blank
//     spelling — but no MA0 frame is printed as a literal anywhere in the
//     book, so the CROSS-FIELD COMBINATION has never been printed or observed.
//     That is the design's A22, PER ROW, and PROVENANCE.md carries its
//     sentence together with the family-level entries an image rides on. The
//     records are not observed contents and not factory defaults, and no byte
//     is invented, derived from another model, or padded to make a test pass.
//
//  11. THE DEFAULT FIRMWARE STRING is the book's one worked example, "for
//     firmware version 1.00, it reads 'FV1.00;'" (990:2533). It is not a claim
//     about any radio's firmware; it is the only FV answer this document
//     prints. The four-byte WIDTH is not an assumption — the answer row's
//     ruler counts it (990:2536) — and no grammar is applied to the four
//     bytes, because reading them as "M.NN" is the design's A13 and a fake
//     that validated the field would assert it.
//
//  12. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
//     unsolicited, whatever AI is set to. The book says an AI-ON radio sends
//     the respective response command when a parameter is changed
//     (990:180-182), and no TS-990S has been observed by this project;
//     modelling silence is the honest default, not a claim that the radio is
//     silent. The engine's drain-to-quiet discipline is exercised against
//     internal/fakeradio, whose own AI-flood facts are the FT-710's.
//
//  13. THE AI VALUES PRINTED "NOT USED" ARE REFUSED. This book's legend prints
//     FIVE values and gives two of them no meaning: "1: Not used" (990:174)
//     and "3: Not used" (990:177). What a radio does with one is unprinted, so
//     this fake refuses them, which is entry 2's strict direction applied to
//     the one field where the legend itself declines to say. Not a claim that
//     a TS-990S rejects "AI1;".
//
//  14. THE AI STATE AT CONSTRUCTION is OFF. Unlike the TS-590's book, which
//     prints "the initial state is OFF" as a fact, THIS book prints no
//     power-on AI state anywhere — only that the backed-up state is
//     initialised to OFF by a reset (990:185-186), which is a different
//     sentence. The value chosen is the one core/transport.Engine.Init sets on
//     every session anyway, so nothing in this programme can observe the
//     difference — but it is a choice, not a transcription.
//
//  15. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound, the single
//     "?;" per overflow and the discard-to-next-';' resync are this package's
//     own bounded-input policy. No Kenwood book prints a buffer size; what
//     this one prints is that a receive-buffer overrun produces "O;"
//     (990:121), which is a different event and is modelled by
//     WithStreamError instead.
//
//  16. THE EX MENU VALUES ARE INVENTED. Every menu's default raw P5 is its
//     printed width in '0' bytes. The parameter lists print each menu's
//     available SETTINGS and never a shipped default, so there is nothing to
//     source a real one from — and `rigprog read --settings --fake` renders
//     these bytes to a user, who must not read them as what a TS-990S ships
//     with. The placeholder is uniform on purpose: an obviously uniform value
//     is harder to mistake for evidence than a plausible-looking spread. What
//     the tables DO carry is each menu's WIDTH, which is transcribed.
//
//  17. AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". A well-formed read naming
//     an address this chart does not print draws the rejection, with the state
//     unchanged. The chart says an ERROR occurs — three times over (990:1727,
//     990:1733, 990:1734-1735) — and never which message, so choosing "?;" is
//     this fake's step: it is the error table's first cause, a syntactically
//     correct command the transceiver cannot execute (990:108-113), applied to
//     a menu the radio has none of.
//
//  18. THE FIXED FORM'S PAD BYTE IS A SPACE. The Answer diagram gives P5
//     fifteen cells and nails the terminator to position 24 (990:1738-1747,
//     erratum E19), and never says what fills the cells a narrower value
//     leaves. The byte chosen is the one this book defines "blank" as, "this
//     setting is blank <0x20>" (990:4081-4082) — the same definition A6 rests
//     on for the MA0 grid — and it is the only candidate that cannot be read
//     as a digit of the value: a '0' pad would make a three-digit menu's
//     answer indistinguishable from a fifteen-digit one. core/kw/ma returns P5
//     verbatim and leaves the pad to its caller, so no production code
//     depends on which byte this is; this package's own ex_test.go pin,
//     TestEXAnswer_ThePadIsSpacesAndTheValueLeadsIt, holds it.
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF. This book prints its own error table with two
// named causes (990:101-113), so using "?;" for every refusal is transcription,
// not assumption. The NOTE beneath it — that the message may not appear at all
// (990:114-116) — is likewise printed, and is played by
// WithTransientNAKSuppressed rather than assumed away. The two further tokens,
// "E;" and "O;" (990:118-121), are printed with their own causes and are
// played by WithStreamError.
//
// THE BLANK CHANNEL'S FRAME. internal/fakets890 needs an ASSUMED entry for
// this and a second blank fixture besides, because its note stops at P12 and
// leaves both the name window and the frame's LENGTH unprinted (890:3215-3216,
// erratum E4, that row's A4, A17 and A21). THIS note covers "P2 to P18"
// (990:2962-2963) inside a frame whose terminator is nailed to position 57, so
// the blank answer is MA0, the slot, fifty spaces and the terminator — printed,
// modulo A6's reading of "blank", which this book states outright for QR as
// "blank <0x20>" (990:4081-4082). One fixture, no assumption of its own, and
// no residue case to invent.
//
// THE COMMAND-NAME CASE FOLD. "A command consists of 2 to 5 alphanumeric
// characters. You may use either lower or upper case characters."
// (990:77-83). A manual fact, and the same sentence is why the dispatcher
// matches a three-character name before a two-character one. Admitting the
// MIXED case is a consequence of folding each name byte independently, not a
// separate leniency, and field values stay case-sensitive because the sentence
// is about the name.
//
// THE GRID'S GEOMETRY AND EVERY LEGEND. The eighteen parameters, their
// positions, the fixed terminator and each legend's values are counted and
// transcribed off the two position charts (990:2893-2915, 990:2919-2938) and
// the parameter list beside them (990:2892-2956). Where this package and
// core/kw/ma agree about a byte position, that is two independent readings of
// one chart agreeing — which is the whole point of the hard rule — and not a
// shared definition.
package fakets990
