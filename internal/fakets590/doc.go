// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets590 simulates a Kenwood TS-590S or TS-590SG's PC-control
// behaviour over an in-memory serial connection (Radio.Port()). It is the
// test double the 590 pair's own layers run against: the transport engine,
// core/driver/ts590, the CLI's --fake mode and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakeft891 for the
// FT-891.
//
// ONE FAKE, TWO REGISTRY ROWS. The TS-590S and the TS-590SG share one
// PC-command document, one 50-byte memory grid, one MD legend and one
// hard-wired byte set, so one package serves both and New's Row argument says
// which. The row is REQUIRED and has no zero value: the two differ at ID
// ("021: TS-590S", "023: TS-590SG", 590:1114-1116) and at byte 28, whose
// FILTER A/B selection the book calls "always 0" only "in firmware version
// 1.xx of TS-590S" (590:1478). A default row would make every test of the
// other sibling a fixture accident.
//
// # The hard rule: NOTHING project-internal
//
// fakets590 MUST NOT import any package of this project — not core/kw, not
// core/kw/ts590, not core/codeplug, not core/spec, and not internal/fakeft891
// or any sibling fake. Standard library only, in every non-test file, in this
// directory AND every directory beneath it. Every byte offset, field width
// and validation rule below is re-derived from the TS-590S/TS-590SG PC
// Control Command Reference Guide (rev 3) — cited "590:NNNN" throughout, the
// same convention core/kw/doc.go uses, naming where a chart is rather than
// linking to it, because the manual itself is gitignored
// (docs/fixtures-private/manuals/).
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/kw's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake
// has to be able to DISAGREE with the production codec for a test against it
// to mean anything, and it can only disagree if it was built from the manual
// rather than from the code.
//
// The fence is enforced mechanically and recursively (imports_test.go), from
// birth, ahead of the gen/ subdirectory a later task of this milestone's plan
// brings in.
//
// # A SIBLING of internal/fakeft891, not a refactor of it
//
// This package duplicates a good deal of internal/fakeft891's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options. That duplication is
// deliberate and it is not going to be factored into a shared "fake core"
// package. A shared helper would be a project-internal import, which the hard
// rule above forbids in every fake, so the only way to share code would be to
// abandon the property that makes any of them worth having.
//
// And the radios do not in fact agree. Where this fake diverges from the
// Yaesu ones, the divergence is this book's, not a preference:
//
//   - THE "?;" CONVENTION IS PRINTED HERE. The Yaesu fakes inherit it from a
//     sibling's reference and register that they do; this book prints its own
//     error table, with two named causes for "?;" and a note that the message
//     may not appear at all (590:93-113).
//   - THERE ARE TWO MORE ERROR TOKENS. "E;" is a serial-line communication
//     error and "O;" a receive-buffer overrun (590:110-113) — neither is a
//     command outcome, and no Yaesu fake has anything like them.
//   - THE AI LEGEND IS 0/2/4. This book prints "0: AI OFF / 2: AI ON (without
//     backup) / 4: AI ON (with backup)" (590:159-162) with no 1 and no 3;
//     the TS-480's own book prints 0, 1, 2 and 3 with different meanings
//     (480:185-190). Two Kenwood radios do not share this table either, which
//     is why internal/fakets480 will transcribe its own.
//   - THE MEMORY RECORD IS 50 BYTES AND ITS REQUEST CARRIES A P1 SELECTOR.
//     "MR P1 P2 P3 P3 ;" reads either half of a channel (590:1440-1442) where
//     every Yaesu read frame names a slot and nothing else.
//   - THE CHANNEL NUMBER'S HUNDREDS DIGIT MAY BE A SPACE (590:1332-1337).
//
// THE ONE EXCEPTION IS internal/fakepipe (added 06/09/2026): the net.Pipe pair,
// the goroutine bookkeeping, the interruptible latency wait and the raw write.
// It is permitted because it is PROTOCOL-FREE — it sees []byte and a duration
// and nothing else, so it carries no framing, no field layout and no reply
// building. A bug in it therefore cannot make a wrong codec look right; it can
// only stop bytes moving, which this package's own tests notice at once.
// Everything above the wire — the reassembler, the parser, the image, the
// replies — stays here, written independently.
//
// # What this fake deliberately does NOT model
//
// THE EX (MENU) SET. The EX READ is modelled — ex.go, from this package's own
// copies of each sibling's transcription B — and the Set is not. The book
// prints one (590:542-547), and core/kw builds none either: the Set and the
// Answer share an identical wire shape, so admitting the Set would admit a
// captured answer being written back. A Set-shaped body therefore falls
// through handleEX's read check to "?;". That is a MODELLING GAP,
// KNOWN-DIVERGENT from the documented grammar, and it is not a claim that
// either radio refuses EX Set.
// TestEX_MalformedAndSetShapedBodiesAreRefused pins the gap's shape.
//
// THE ERASE FORM OF MW. The book describes one: "If you do not specify one
// digit in P16 and execute all the parameters from P4 to P15 set to 0, the
// channels specified by P2 and P3 will be erased." (590:1579-1581). This
// programme builds no erase frame on any radio — a standing rule of this
// repository — so the fake accepts MW at exactly the 50-byte width its chart
// counts (590:1518-1536) and refuses every other width, the erase form
// included. Not a claim that the radio lacks the command.
//
// FAULT INJECTION beyond the book. There is no dropped-reply, garbled-byte,
// spurious-frame or chunked-write option here. Those exercise
// core/transport.Engine, one model-independent implementation already covered
// against internal/fakeradio's fault suite, and nothing is learnt by running
// them past a second dialect. What IS modelled is what THIS BOOK prints: the
// two stream-error tokens, and the transient "?;" suppression its own error
// table flags (590:106-108).
//
// A FRONT PANEL. Nothing here models one, so the selected channel moves only
// by an MC Set and this fake never produces an MC answer naming a channel no
// host asked for.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something the TS-590S/TS-590SG PC
// Control Command Reference Guide does not settle is listed here, and each
// entry appears as an inline comment beside the code that implements it.
// Entries are cited BY NAME, never by number: renumbering is an editorial act
// and must not break a citation. TestASSUMEDRegisterIsComplete holds both
// halves of that promise mechanically — the roll and the headings must agree,
// and every entry must have a point of use outside this file.
//
// The register's SUBJECT is this fake's behaviour. The DESIGN's own register —
// A1..A27, each with a named lift — lives in the milestone's design document
// and is carried in the repository by core/kw/doc.go; where an entry below
// exists because a design entry is unlifted, it says which one, but correcting
// an A-number is a design change and correcting an entry here is a change to
// this package.
//
//  1. AN ACCEPTED SET PRODUCES NO REPLY. Neither MW's chart nor MC's Set row
//     prints an acknowledgement (590:1516-1536, 590:1333), and the book's
//     error table lists only failures (590:93-113), so this fake answers
//     nothing at all to an accepted Set. That silence is the SAME code path
//     as "this handler has nothing to say", deliberately, so no handler can
//     acknowledge a Set by accident. What a real radio does on the wire after
//     an accepted MW has not been observed by this project.
//
//  2. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go — P1's two values, the channel number's spelling, the
//     eleven-digit frequency, the mode nibble, the DATA flag, the tone mode,
//     the two-digit tone indices, the three printed-constant runs, byte 28,
//     P14, P15 and the name charset — is ASSUMED to be what the radio itself
//     enforces. The book prints the legends; it never says what a radio does
//     with a Set that leaves one. Nothing here NORMALISES: a byte outside its
//     legend is refused, never quietly corrected, because a driver that sent
//     one would otherwise pass its own tests and fail on hardware.
//
//  3. AN ANSWER'S HUNDREDS DIGIT IS A SPACE BELOW CHANNEL 100. MC's chart
//     prints the rule for its own answer — "For a response command, a space
//     is entered for a channel number less than 100" (590:1336-1337) — and
//     both memory charts refer their P2/P3 cells to MC (590:1452-1453,
//     590:1539-1540). Reading that cross-reference as carrying the SPACE
//     CONVENTION and not merely the numbering is this fake's step. It is
//     deliberately the opposite spelling to the one core/kw emits, which
//     always writes '0' and accepts either: a fake that echoed the request's
//     byte back would never exercise the codec's tolerance.
//
//  4. THE SECOND HALF OF A CHANNEL WITH NO STORED TRANSMIT RECORD answers the
//     EMPTY record. What an MR with P1=1 answers on a simplex channel is
//     unprinted on both radios — the design's A9, and the 590SG's own note
//     assumes the caller already knows which kind of channel it is reading
//     (590:1444-1447). Of the shapes available, this fake gives the one the
//     book DOES describe for a channel holding nothing (590:1492-1493),
//     rather than inventing a rejection or echoing the receive frequency back
//     as a transmit one — which would be the silent-split-flattening the
//     driver's A9 refusal exists to prevent, manufactured inside the test
//     double.
//
//  5. A SET CARRYING A "NONE" MODE NIBBLE IS STORED. The MD legend prints
//     nibbles 0 and 8 as "None (setting failure)" (590:1353, 590:1362) and
//     nothing says what a Set carrying one does — the design's A18b, whose
//     lift is a hardware trial. This fake stores the nibble rather than
//     inventing a refusal, which is what lets a test drive core/kw's own
//     build refusal, and its empty-channel reading, against a real fake.
//
//  6. TONE INDICES ARE STORED, NOT RANGE-CHECKED. TN's own chart prints "An
//     entered value of 43 or higher results in an error" (590:2309) for the
//     TN COMMAND, and the CN chart prints no such sentence at all. Whether
//     either rule holds inside a memory frame is the design's A21, unlifted,
//     so only the field's SHAPE is enforced here. Refusing would assert A21
//     as a fact about the radio and would put the codec's own refusal out of
//     reach of a real fake.
//
//  7. BYTE 28 IS ACCEPTED EITHER WAY ON BOTH ROWS. P11's legend is printed
//     once for both siblings (590:1476-1477) and the "always 0" sentence is
//     scoped to the TS-590S's firmware 1.xx (590:1478). Tying this fake's
//     acceptance to its firmware string would be the fake asserting the
//     design's A14; that decision belongs to the driver's write path, where
//     the design puts it. So both values are accepted and answered on both
//     rows, and WithFirmwareVersion changes only what "FV;" says.
//
//  8. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in the MW block
//     mentions the selection (590:1516-1581). A fake that moved it would let
//     a driver depend on a side-effect the book does not describe.
//
//  9. THE SELECTED CHANNEL AT CONSTRUCTION is 000. A radio that has had no MC
//     Set is sitting on some channel and this book prints no power-on value
//     anywhere — unlike the AI function, whose initial state IS printed
//     (590:81-82). This fake takes the lowest number in its slot space.
//
//  10. THE SG'S EXTENSION CHANNELS ARE NOT SERVED. "TS-590SG extension
//     channel numbers E00 ~ P09 are represented by 110 ~ 119."
//     (590:1346-1347, the second name a printed typo for E09) — so those ten
//     numbers ARE printed for one of the two rows. What an extension channel
//     IS is never explained, and Stuart ruled on 05/09/2026 that the ten
//     slots are omitted from the driver's published banks until that is
//     lifted (the design's A11), which leaves no slot ID for an image to
//     represent. This fake therefore serves 000-109 on BOTH rows and refuses
//     110-119 through the same out-of-domain path as any other number. IT IS
//     A DELIBERATE NARROWING OF THE PRINTED DOMAIN, not a claim that either
//     radio refuses those channels, and the day A11 lifts this entry is what
//     a reader has to change.
//
//  11. THE DEFAULT IMAGE'S RECORD COMPOSITION. Every BYTE of every shipped
//     record is a printed constant, the printed example frequency
//     (590:965), a printed legend value, or the eight-space name — but no MR,
//     MW or MC frame is printed as a literal anywhere in either Kenwood book,
//     so the CROSS-FIELD COMBINATION has never been printed or observed.
//     That is the design's A27, and PROVENANCE.md carries its sentence
//     verbatim together with the family-level entries an image rides on. The
//     records are not observed contents and not factory defaults, and no byte
//     is invented, derived from another model, or padded to make a test pass.
//
//  12. THE DEFAULT FIRMWARE STRING is the book's one worked example, "for
//     firmware version 1.00, it reads 'FV1.00;'" (590:1035). It is not a
//     claim about any radio's firmware; it is the only FV answer this
//     document prints. The four-byte WIDTH is not an assumption — the chart
//     counts it (590:1037) and menu 000 corroborates it (590:749) — and no
//     grammar is applied to the four bytes, because reading them as "M.NN" is
//     the design's A13 and a fake that validated the field would assert it.
//
//  13. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
//     unsolicited, whatever AI is set to. The book says an AI-ON radio
//     outputs a response whenever a parameter changes (590:165-167), and no
//     TS-590 of either row has been observed by this project; modelling
//     silence is the honest default, not a claim that the radio is silent.
//     The engine's drain-to-quiet discipline is exercised against
//     internal/fakeradio, whose own AI-flood facts are the FT-710's.
//
//  14. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound, the single
//     "?;" per overflow and the discard-to-next-';' resync are this package's
//     own bounded-input policy. No Kenwood book prints a buffer size; what it
//     prints is that a receive-buffer overrun produces "O;" (590:113), which
//     is a different event and is modelled by WithStreamError instead.
//
//  15. THE S ROW'S CEILING IS UNSTATED. highestChannel = 109 serves both
//     rows, but the two rows reach it on different grounds. On the SG, 110
//     onwards is the printed extension range and stopping short of it is
//     entry 10's deliberate narrowing under A11. On the S, the book never
//     prints a ceiling at all — nothing says a TS-590S refuses "MR0110;" —
//     so 109 there is the design's own A12, unlifted, and
//     core/kw/ts590/layout.go names A12 for this same row. This fake stops
//     at 109 on the S because that is as far as the book's section-defined
//     channels go, not because a ceiling has been observed.
//
//  16. THE EX MENU VALUES ARE INVENTED. Every menu's default raw P5 is its
//     printed width in '0' bytes. The two parameter lists print each menu's
//     available SETTINGS and never a shipped default (590:564, 590:744), so
//     there is nothing to source a real one from — and `rigprog read
//     --settings --fake` renders these bytes to a user, who must not read
//     them as what a TS-590 ships with. The placeholder is uniform on
//     purpose: an obviously uniform value is harder to mistake for evidence
//     than a plausible-looking spread. What the tables DO carry is each
//     menu's WIDTH, which is transcribed (ex.go).
//
//  17. AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". A well-formed read
//     naming a menu number this ROW's chart does not print draws the
//     rejection, with the state unchanged. Neither list prints what happens
//     at an address it does not carry; "?;" is the error table's first cause
//     — a syntactically correct command the transceiver cannot execute
//     (590:96-105) — applied to a menu the radio has none of. The sharp case
//     is the S's 088: a REAL menu on the SG and past the end of the S's own
//     domain (590:543-544), which is why the inventory is per row.
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF. On the Yaesu side this is an inherited
// convention with no line to cite. Here the book prints its own error table
// with two named causes (590:93-108), so using "?;" for every refusal is
// transcription, not assumption. The NOTE beneath it — that the message may
// not appear at all (590:106-108) — is likewise printed, and is played by
// WithTransientNAKSuppressed rather than assumed away.
//
// THE COMMAND-NAME CASE FOLD. "A command consists of 2 or 3 characters. You
// may use either lower or upper case characters." (590:62-63). A manual fact.
// Admitting the MIXED case is a consequence of folding each name byte
// independently, not a separate leniency, and field values stay
// case-sensitive because the sentence is about the name.
//
// AI'S INITIAL STATE. "Turn this function on using the AI command (the
// initial state is OFF)" (590:81-82). A manual fact, and the reason New
// starts at '0' rather than at a chosen default.
//
// THE EMPTY-CHANNEL ANSWER. "If the selected channel is empty, P4 ~ P15 will
// be 0 and P16 will be blank." (590:1492-1493). Documentary fact on this
// pair, and the design keeps it in its own register (A18a) only so that no
// later reader restores an earlier draft's refusal. The TS-480 half of that
// question is a genuine assumption (A4) and belongs to the other fake.
//
// THE FIFTY-BYTE WIDTH, THE FIELD OFFSETS AND EVERY LEGEND. Counted and
// transcribed off the two position charts (590:1440-1461, 590:1518-1536).
// Where this package and core/kw agree about a byte position, that is two
// independent readings of one chart agreeing — which is the whole point of
// the hard rule — and not a shared definition.
package fakets590
