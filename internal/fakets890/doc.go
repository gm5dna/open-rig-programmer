// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets890 simulates a Kenwood TS-890S's PC-control behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double this
// registry row's own layers run against: the transport engine,
// core/driver/ts890, the CLI's --fake mode and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakets590 for the
// TS-590 pair.
//
// ONE FAKE, ONE REGISTRY ROW. Unlike internal/fakets590, which serves two
// siblings out of one book, this package serves one radio out of one book and
// therefore has no Row argument and no per-row table. Its sibling
// internal/fakets990 is a SEPARATE PACKAGE for the same reason the two driver
// packages are separate: the two books print different memory grids (40-50
// bytes here, a fixed 57 there), different mode legends, different lockout
// encodings and two disjoint menu charts, and a shared table would let one
// radio's evidence stand in for the other's.
//
// # The hard rule: NOTHING project-internal
//
// fakets890 MUST NOT import any package of this project — not core/kw, not
// core/kw/ma, not core/codeplug, not core/spec, and not internal/fakets590 or
// any sibling fake. Standard library only, in every non-test file, in this
// directory AND every directory beneath it. Every byte offset, field width and
// validation rule below is re-derived from the TS-890S PC Control Command
// Reference Guide (rev 1, January/30/2019) — cited "890:NNNN" throughout, the
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
// # A SIBLING of internal/fakets590, not a refactor of it
//
// This package duplicates a good deal of that one's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options. The duplication is
// deliberate and it is not going to be factored into a shared "fake core"
// package: a shared helper would be a project-internal import, which the hard
// rule above forbids in every fake, so the only way to share code would be to
// abandon the property that makes any of them worth having.
//
// And the two radios do not in fact agree. Where this fake diverges from its
// Kenwood siblings, the divergence is this book's:
//
//   - THERE IS NO MR, MW, MC OR TY COMMAND ON THIS RADIO. The memory record is
//     MA0, a 13-parameter grid whose Read carries its own channel number
//     (890:3184-3186) and whose Answer is 40 to 50 bytes with a FLOATING
//     TERMINATOR (890:3189-3204) — where every 590-family frame is exactly 50
//     bytes with a fixed ';'.
//   - THE COMMAND NAME IS 2 TO 5 CHARACTERS, not 2 to 3: "A command consists
//     of 2 to 5 alphanumeric characters." (890:76-80). "MA0" is a
//     three-character name, so this package's dispatcher cannot split a frame
//     at a fixed offset.
//   - THE MODE LEGEND IS OM's SIXTEEN NIBBLES 0-9 and A-F (890:3976-3992),
//     where the 590 pair's MD legend prints ten.
//   - THE EX ADDRESS IS A GROUPED TRIPLE of one, two and two digits
//     (890:1897-1911), where every other Kenwood row in this repository
//     carries a single three-digit menu number.
//   - THE AI LEGEND PRINTS FIVE VALUES, two of them "Not used"
//     (890:175-181), where the 590's prints three and the TS-480's four.
//
// # WHERE THE EX INVENTORY COMES FROM
//
// The widths table ex.go answers from is NOT hand-typed. It is projected at
// init, by exinventory.go, from this package's OWN COPY of TRANSCRIPTION B —
// transcription-b-890s.csv beside this file, with PROVENANCE.md recording
// where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of this row's two-source cross-check:
//
//   - the CODEC's inventory (core/kw/ma/exinventory890s_gen.go) is generated
//     from TRANSCRIPTION A (core/kw/ma/menu890s.csv) by internal/extable;
//   - THIS inventory is projected from TRANSCRIPTION B by exinventory.go,
//     which imports nothing project-internal at all;
//   - core/transport/ex_crosscheck_ts890_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined
// agent with no repository access and no sight of A, the page ledger or any
// row count (core/kw/ma/crosscheck_test.go records the artefacts and hashes
// them). So a mis-read row in either transcription, or a defect in either
// parser, surfaces as a cross-check MISMATCH rather than as two tables quietly
// agreeing on the same wrong number.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF.
//
// TWO CORRECTIONS ARE APPLIED IN THE PROJECTION, and neither is an edit to the
// evidence. Transcription B is frozen and is not touched; what exinventory.go
// does is fold it under two facts printed in this book:
//
//   - THE FOUR EXCLUDED ADDRESSES. The chart prints four Advanced Menu rows
//     with real addresses and the body "Does not correspond to a command"
//     (890:2273-2280, erratum E16). B carries them and the production
//     inventory does not, so this side applies its OWN exclusion of the same
//     four, spelt out by address.
//   - RULING R-B, THE PF KEY WIDTH. B reads three digits on all seventeen PF
//     key rows and A reads four; the EX block's own P5 note settles it — "PF
//     key settings use 4 digits (refer to the PF Key assignment ID lists)."
//     (890:1918-1919). The arbitration is already made and recorded, in
//     core/kw/ma/crosscheck_test.go's ruling R-B, which states THE LEG IS
//     WRONG, NOT A and checks the divergence in both directions. So the
//     projection corrects those seventeen widths from the printed sentence,
//     read here at this side; it consults neither transcription A nor
//     internal/extable, and it REFUSES to apply the correction unless every
//     one of the seventeen is present and still carries the three the ruling
//     records — a leg that has moved is an arbitration, not a correction to
//     apply blind. A fake answering a width the book contradicts would be
//     modelling the radio wrongly on a point the book settles, which is not
//     what an independent evidence leg is for; it is the same move
//     internal/fakets590 makes when it declines to project B's text flag under
//     the repository's own ruling.
//
// # What this fake deliberately does NOT model
//
// THE EX (MENU) SET. The EX READ is modelled — ex.go, from this package's own
// copy of transcription B — and the Set is not. The book prints one
// (890:1900-1904), and core/kw/ma builds none either: the Set and the Answer
// share an identical wire shape, so admitting the Set would admit a captured
// answer being written back. A Set-shaped body therefore falls through
// handleEX's read check to "?;". That is a MODELLING GAP, KNOWN-DIVERGENT from
// the documented grammar, and it is not a claim that a TS-890S refuses EX Set.
// TestEXRead_ASetShapedBodyIsRefused pins the gap's shape.
//
// EVERY MA COMMAND EXCEPT MA0. The family runs MA0 to MA7 (890:3164-3356) and
// the design's outbound roster admits MA0 alone, in both directions; MA1's
// direct write, MA2's channel name, MA3's scan lockout, MA4's copy, MA5's
// deletion, MA6's programmable-VFO end frequency and MA7's temporary change
// are all off it, as are MI, MN and MV. They fall through the dispatcher to
// "?;", which is NOT a claim that the radio lacks them — several of them are
// how a real TS-890S creates a channel at all, which is why the create path is
// a published cost of this registry row rather than a gap this fake hides.
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
// two stream-error tokens (890:118-123), and the transient "?;" suppression
// its own error table flags (890:114-116).
//
// A FRONT PANEL, AND ANY NOTION OF A SELECTED CHANNEL. This radio's MA0 Read
// carries its own channel number (890:3186), so there is no selection for a
// host to move and none for a panel to move behind its back. That is the
// design's A18 — the load-bearing read assumption of the whole milestone — and
// this fake is built to it: nothing here depends on a preceding MN, because
// nothing here has any state an MN could set.
//
// THE LAN SURFACE. The book documents a TCP/IP path with its own ##CN
// connection command (890:58, 890:5457 onwards). This programme speaks
// serial, and nothing here models a socket.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something the TS-890S PC Control Command
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
//  1. AN ACCEPTED SET PRODUCES NO REPLY. MA0's own notes (890:3211-3221) say
//     nothing about an acknowledgement, where MA2, MA3 and MA6 each print
//     "When the AI function is ON, a response is provided by the MA0 command"
//     (890:3266-3267, 890:3283-3284, 890:3327-3328) — so the family's siblings
//     print what MA0's block does not. That is the design's A20, and this fake
//     answers nothing at all to an accepted Set. The silence is the SAME code
//     path as "this handler has nothing to say", deliberately, so no handler
//     can acknowledge a Set by accident. What a real radio does on the wire
//     after an accepted MA0 Set has not been observed by this project.
//
//  2. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in parser.go
//     — the channel number's spelling, the two eleven-digit frequencies, the
//     two mode nibbles, the two FM narrow flags, the tone type, the two
//     two-digit tone indices, the split flag, the lockout flag and the name's
//     length and charset — is ASSUMED to be what the radio itself enforces.
//     The book prints the legends; it never says what a radio does with a Set
//     that leaves one. Nothing here NORMALISES: a byte outside its legend is
//     refused, never quietly corrected, because a driver that sent one would
//     otherwise pass its own tests and fail on hardware.
//
//  3. THE ANSWER'S CHANNEL NUMBER IS THREE ZERO-PADDED DIGITS. The chart
//     prints one domain, "000 ~ 119" (890:3167), over three P1 cells in all
//     three directions and describes no space form anywhere — unlike the
//     TS-590's MC, whose own chart prints "a space is entered for a channel
//     number less than 100". So what a TS-890S ANSWERS with is unprinted, and
//     this fake spells it zero-padded. That is the design's A5, and its
//     failure direction is safe: a space-padded answer MISSES the codec's
//     prefix matcher and times out; it cannot be mis-attributed to another
//     channel.
//
//  4. A BLANK CHANNEL ANSWERS THE 40-BYTE FRAME. Two assumptions meet in one
//     byte string. That a blank channel ANSWERS is printed — "When reading a
//     blank channel, parameters P2 to P12 becomes blank." (890:3215-3216) —
//     but neither MA0 block defines "blank" (the design's A6, taken as ASCII
//     space on the strength of QR's own definition at 890:4362-4363), and the
//     note STOPS AT P12, saying nothing about the P13 name window where the
//     990S's equivalent covers P2 to P18 (erratum E4). This fake takes A4's
//     reading — the window is blank too, so the frame carries no name at all
//     and is 40 bytes, which is the design's A17. The image is a CONCRETE BYTE
//     STRING and not "an unspecified name window", and PROVENANCE.md names A4
//     and A21 by number.
//
//  5. A SET CARRYING AN "UNUSED" MODE NIBBLE IS STORED. The OM P2 legend
//     prints nibbles 0 and 8 as "Unused" (890:3977, 890:3985) and nothing says
//     what a Set carrying one does. This fake stores the nibble rather than
//     inventing a refusal, which is what lets a test drive core/kw/ma's own
//     build refusal, and the driver's blank-channel reading, against a real
//     fake.
//
//  6. TONE INDICES ARE STORED, NOT RANGE-CHECKED. The TN chart prints "00 ~
//     50" plus a set-only 99, the CN chart "00 ~ 49" plus the same, and each
//     prints "Entering a value that does not exist is invalid" for ITS OWN
//     COMMAND (890:5165, 890:1371). Whether either rule holds inside an MA0
//     frame is unprinted, so only the field's SHAPE is enforced here. Refusing
//     would assert a fact about the radio and would put the codec's own
//     refusal out of reach of a real fake.
//
//  7. THE P4/P10 AGREEMENT IS NOT ENFORCED. The book instructs the HOST:
//     "When setting the split memory channel, set the same setting on the
//     transmission side and the reception side for FM normal / narrow
//     information (P4, P10)." (890:3219-3221). What the radio does with a
//     disagreeing Set is unprinted. core/driver/ts890 carries a refusal rung
//     for exactly this pair, and a fake that pre-empted it would put that rung
//     out of reach of a real fake — so each byte is checked against its own
//     legend and the two are never compared.
//
//  8. A SET TO A BLANK CHANNEL IS STORED. Whether MA0 alone can CREATE a
//     channel is the design's A3 — every other member of the family prints an
//     unassigned-channel prohibition and MA0's block prints nothing either way
//     — and core/driver/ts890 REFUSES such a write on that ground. A fake that
//     refused it here would assert A3 as a fact about the radio AND would put
//     the driver's own refusal out of reach of a real fake.
//
//  9. SLOTS 100-119 ARE NOT SERVED. The chart prints the domain "000 ~ 119"
//     and maps 100-109 to P0-P9 and 110-119 to E0-E9 (890:3167-3169), and that
//     number mapping is the ONLY thing this book says about either class: what
//     a Programmable VFO slot answers to an MA0 read is unprinted (the design's
//     A9), and what an E channel IS is never explained anywhere in the document
//     (erratum E18, eight sites, every one the same sentence). Both classes are
//     published in NO bank of this registry row, so there is no slot ID for an
//     image to represent. This fake serves 000-099 and refuses 100-119 through
//     the same out-of-domain path as any other number — one refusal path, no
//     special case. IT IS A DELIBERATE NARROWING OF THE PRINTED DOMAIN, not a
//     claim that a TS-890S refuses those channels, and the day A9 and A10 lift
//     this entry is what a reader has to change.
//
//  10. THE DEFAULT IMAGE'S RECORD COMPOSITION. Every BYTE of every shipped
//     record is a printed constant, one of the two printed example
//     frequencies (890:86, 890:342), a printed legend value, a character from
//     the printed KY set (890:2891-2900), or A6's blank spelling — but no MA0
//     frame is printed as a literal anywhere in the book, so the CROSS-FIELD
//     COMBINATION has never been printed or observed. That is the design's
//     A22, PER ROW, and PROVENANCE.md carries its sentence together with the
//     family-level entries an image rides on. The records are not observed
//     contents and not factory defaults, and no byte is invented, derived from
//     another model, or padded to make a test pass.
//
//  11. THE DEFAULT FIRMWARE STRING is the book's one worked example, "For
//     example: 'FV1.00;' (firmware version 1.00)" (890:2657). It is not a
//     claim about any radio's firmware; it is the only FV answer this document
//     prints. The four-byte WIDTH is not an assumption — the answer row's
//     ruler counts it (890:2659) — and no grammar is applied to the four
//     bytes, because reading them as "M.NN" is the design's A13 and a fake that
//     validated the field would assert it.
//
//  12. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
//     unsolicited, whatever AI is set to. The book says an AI-ON radio sends
//     the respective response command whenever a parameter changes
//     (890:183-185), and no TS-890S has been observed by this project;
//     modelling silence is the honest default, not a claim that the radio is
//     silent. The engine's drain-to-quiet discipline is exercised against
//     internal/fakeradio, whose own AI-flood facts are the FT-710's.
//
//  13. THE AI VALUES PRINTED "NOT USED" ARE REFUSED. This book's legend prints
//     FIVE values and gives two of them no meaning: "1: Not used" (890:177)
//     and "3: Not used" (890:180). What a radio does with one is unprinted, so
//     this fake refuses them, which is entry 2's strict direction applied to
//     the one field where the legend itself declines to say. Not a claim that
//     a TS-890S rejects "AI1;".
//
//  14. THE AI STATE AT CONSTRUCTION is OFF. Unlike the TS-590's book, which
//     prints "the initial state is OFF" as a fact, THIS book prints no
//     power-on AI state anywhere. The value chosen is the one
//     core/transport.Engine.Init sets on every session anyway, so nothing in
//     this programme can observe the difference — but it is a choice, not a
//     transcription.
//
//  15. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound, the single
//     "?;" per overflow and the discard-to-next-';' resync are this package's
//     own bounded-input policy. No Kenwood book prints a buffer size; what
//     this one prints is that a receive-buffer overrun produces "O;"
//     (890:121-123), which is a different event and is modelled by
//     WithStreamError instead.
//
//  16. THE EX MENU VALUES ARE INVENTED. Every menu's default raw P5 is its
//     printed width in '0' bytes. The parameter lists print each menu's
//     available SETTINGS and never a shipped default, so there is nothing to
//     source a real one from — and `rigprog read --settings --fake` renders
//     these bytes to a user, who must not read them as what a TS-890S ships
//     with. The placeholder is uniform on purpose: an obviously uniform value
//     is harder to mistake for evidence than a plausible-looking spread. What
//     the tables DO carry is each menu's WIDTH, which is transcribed.
//
//  17. AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". A well-formed read naming
//     an address this chart does not print draws the rejection, with the state
//     unchanged. The chart says an ERROR occurs — three times over
//     (890:1904, 890:1909, 890:1910-1911) — and never which message, so
//     choosing "?;" is this fake's step: it is the error table's first cause,
//     a syntactically correct command the transceiver cannot execute
//     (890:106-112), applied to a menu the radio has none of. THE FOUR
//     EXCLUDED ADDRESSES ANSWER THROUGH THE SAME PATH, which is the chart's
//     own "Entering a number that cannot be set" case (890:2273-2280).
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF. This book prints its own error table with two
// named causes (890:98-112), so using "?;" for every refusal is transcription,
// not assumption. The NOTE beneath it — that the message may not appear at all
// (890:114-116) — is likewise printed, and is played by
// WithTransientNAKSuppressed rather than assumed away. The two further tokens,
// "E;" and "O;" (890:118-123), are printed with their own causes and are
// played by WithStreamError.
//
// THE COMMAND-NAME CASE FOLD. "A command consists of 2 to 5 alphanumeric
// characters. You may use either lower or upper case characters."
// (890:76-80). A manual fact, and the same sentence is why the dispatcher
// matches a three-character name before a two-character one. Admitting the
// MIXED case is a consequence of folding each name byte independently, not a
// separate leniency, and field values stay case-sensitive because the sentence
// is about the name.
//
// THAT A BLANK CHANNEL ANSWERS AT ALL. "When reading a blank channel,
// parameters P2 to P12 becomes blank." (890:3215-3216) says the read is
// answered rather than refused. Documentary fact. What "blank" MEANS and what
// the name window holds are the two assumptions, and they are entry 4.
//
// THE GRID'S GEOMETRY AND EVERY LEGEND. The thirteen parameters, their
// positions, the floating terminator and each legend's values are counted and
// transcribed off the two position charts (890:3166-3182, 890:3189-3204) and
// the parameter list beside them (890:3165-3209). Where this package and
// core/kw/ma agree about a byte position, that is two independent readings of
// one chart agreeing — which is the whole point of the hard rule — and not a
// shared definition.
package fakets890
