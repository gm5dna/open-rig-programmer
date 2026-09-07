// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets480 simulates a Kenwood TS-480's PC-control behaviour over
// an in-memory serial connection (Radio.Port()). It is the test double the
// TS-480 row's own layers run against: the transport engine,
// core/driver/ts480 and, on the day that row is registered, the CLI's --fake
// mode and the GUI's demo mode — the role internal/fakeradio plays for the
// FT-710 and internal/fakets590 for the two TS-590 rows.
//
// ONE ROW, AND NO ROW ARGUMENT. The 2003 document describes one PC-control
// radio and prints one identity for it, "020: TS-480" (480:678). The HX/SAT
// split it does print is a HARDWARE VARIANT reported by TY — "0: TS-480HX
// (200 W) / 1: TS-480SAT (100 W + AT) / 2: Japanese 50 W type / 3: Japanese
// 20 W type" (480:1626-1629) — and decision 4 is explicit that the variant
// digit is reported and never used to select a registry row. So New takes no
// row, where internal/fakets590's takes a required one.
//
// THE ROW THIS FAKE SERVES IS BUILT AND NOT REGISTERED at this milestone's
// close (Stuart decision row 2). Nothing in internal/wiring reaches this
// package yet, and the release gate is A4 — see the register entry AN
// UNWRITTEN CHANNEL ANSWERS THE ZERO RECORD.
//
// # The hard rule: NOTHING project-internal
//
// fakets480 MUST NOT import any package of this project — not core/kw, not
// core/kw/ts480, not core/codeplug, not core/spec, and not internal/fakets590
// or any other sibling fake. Standard library only, in every non-test file,
// in this directory AND every directory beneath it. Every byte offset, field
// width and validation rule below is re-derived from the PC CONTROL COMMAND
// REFERENCE FOR THE TS-480HX/SAT TRANSCEIVER (2003) — cited "480:NNNN"
// throughout, the same convention core/kw/doc.go uses, naming where a chart
// is rather than linking to it, because the manual itself is gitignored
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
// The fence is enforced mechanically and recursively (imports_test.go): this
// directory and every one beneath it.
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
// # A SIBLING of internal/fakets590, a re-skin of its scaffold
//
// The two Kenwood fakes share a shape — the pipe-and-goroutine Radio, the
// bounded reassembler, the "?;" convention, the per-command handlers, the
// Image contract, the options — and share NO PACKAGE, NO TABLE AND NO
// LEGEND: every offset, width, legend and validator in this file is
// re-derived from this book, independently of internal/fakets590's. What IS
// shared, and is shared deliberately, is the pipe/reassembler/dispatch
// scaffold around those tables: the hard rule above forbids importing it, so
// the only way to reuse it at all is to copy the source, the same trade
// imports_test.go names for itself. Roughly two thirds of this package's
// top-level function bodies are byte-identical to internal/fakets590's for
// that reason — checked against this book and correct for the TS-480 — so a
// defect in the copied scaffold (not in a table or a legend) would sit in
// both Kenwood fakes at once, and the next auditor changing either file
// should check the other. The radios do not in fact agree in what the
// tables say: this book gives several bytes of the same 50-byte grid
// entirely different jobs, and gives two of its own error tokens different
// causes. Every divergence below is this book's, not a preference:
//
//   - THE CHANNEL NUMBER IS TWO DIGITS AND THERE IS NO HUNDREDS BYTE. MR's
//     P2 is a printed constant, "Always 0 for the TS-480." (480:910), and
//     P3 is the whole channel number, "00 ~ 99" (480:912). The 590 pair
//     spend byte 4 on the hundreds digit and reach 109. There is therefore
//     no space-versus-zero answer convention on this radio at all.
//   - BYTE 19 IS THE CHANNEL LOCKOUT, "Lockout status. 0: Lockout OFF,
//     1: Lockout ON." (480:920), where the 590 pair carry the DATA-mode
//     flag; and byte 41, where THEY carry the lockout, is a printed
//     constant here (480:939).
//   - BYTES 39-40 ARE THE TUNING STEP, "Step size. Refer to the ST command."
//     (480:937), where the 590 pair carry an FM bandwidth flag.
//   - BYTE 28 IS A PRINTED CONSTANT (480:931), where the 590 pair carry the
//     FILTER A/B selection.
//   - THE TONE-MODE LEGEND HAS THREE VALUES, "0: OFF, 1: TONE, 2: CTCSS"
//     (480:922). The 590 pair print four, adding a cross-tone mode.
//   - THE AI LEGEND IS 0/1/2/3, four consecutive values with their own
//     meanings (480:185-190), against the 590 pair's 0/2/4.
//   - "O;" HAS A DIFFERENT CAUSE. Here it is "Receive data was sent but
//     processing was not completed" (480:143-144); there it is a receive
//     buffer overrun (590:113). core/kw/doc.go records the disagreement as
//     erratum E13.
//   - THERE IS NO FV AND NO READABLE FIRMWARE VERSION. "FV" appears nowhere
//     in this document, which carries no revision number and no firmware
//     statement either (erratum E15) — the one place the word "firmware"
//     itself appears is the FV command's own heading, "Sets or reads the
//     microprocessor fimware type" (480:1621, erratum E10), under which the
//     chart is empty. TY is the nearest command and reports a hardware
//     variant, which is what E10 and E15 jointly record.
//
// # What this fake deliberately does NOT model
//
// THE EX (MENU) SET. The EX READ is modelled — ex.go, from this package's own
// copy of transcription B — and the Set is not. The book prints one
// (480:399-406), and core/kw builds none either: the Set and the Answer share
// an identical wire shape, so admitting the Set would admit a captured answer
// being written back. A Set-shaped body therefore falls through handleEX's
// read check to "?;", which on this radio has a sharp illustration — the book
// prints "EX00000003;" as an ANSWER (480:416), and the same ten bytes sent the
// other way are refused. That is a MODELLING GAP, KNOWN-DIVERGENT from the
// documented grammar, and it is not a claim that this radio refuses EX Set.
// TestEX_MalformedAndSetShapedBodiesAreRefused pins the gap's shape.
//
// FAULT INJECTION beyond the book. There is no dropped-reply, garbled-byte,
// spurious-frame or chunked-write option here. Those exercise
// core/transport.Engine, one model-independent implementation already covered
// against internal/fakeradio's fault suite, and nothing is learnt by running
// them past a second dialect. What IS modelled is what THIS BOOK prints: the
// two stream-error tokens, and the transient "?;" suppression its own error
// table flags (480:136-138).
//
// A FRONT PANEL. Nothing here models one, so the selected channel moves only
// by an MC Set and this fake never produces an MC answer naming a channel no
// host asked for.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something the TS-480's PC Control
// Command Reference does not settle is listed here, and each entry appears
// as an inline comment beside the code that implements it. Entries are cited
// BY NAME, never by number: renumbering is an editorial act and must not
// break a citation. TestASSUMEDRegisterIsComplete holds both halves of that
// promise mechanically — the roll and the headings must agree, and every
// entry must have a point of use outside this file.
//
// The register's SUBJECT is this fake's behaviour. The DESIGN's own register
// — A1..A27, each with a named lift — lives in the milestone's design
// document and is carried in the repository by core/kw/doc.go; where an entry
// below exists because a design entry is unlifted, it says which one, but
// correcting an A-number is a design change and correcting an entry here is a
// change to this package.
//
//  1. AN ACCEPTED SET PRODUCES NO REPLY. Neither MW's chart nor MC's Set row
//     prints an acknowledgement (480:949-987, 480:829-830), and the book's
//     error table lists only failures (480:126-144), so this fake answers
//     nothing at all to an accepted Set. That silence is the SAME code path
//     as "this handler has nothing to say", deliberately, so no handler can
//     acknowledge a Set by accident. What a real radio does on the wire after
//     an accepted MW has not been observed by this project — the design's A6.
//
//  2. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go — P1's two values, the bank byte's single printed value, the
//     two-digit channel number, the eleven-digit frequency, the mode nibble,
//     the lockout flag, the three-value tone mode, the two-digit tone
//     indices, the two-digit step index, the SIX printed-constant runs and
//     the name charset — is ASSUMED to be what the radio itself enforces. The
//     book prints the legends; it never says what a radio does with a Set
//     that leaves one. On the printed-constant runs the assumption has its
//     own design entry, A24, because this book's general note expressly
//     permits a Set to fill an inapplicable parameter with "any character
//     except the ASCII control codes (00 to 1Fh) and the terminator (;)"
//     (480:108-111) — so strictness there is this programme's rule rather
//     than a deduction. Nothing here NORMALISES: a byte outside its legend is
//     refused, never quietly corrected, because a driver that sent one would
//     otherwise pass its own tests and fail on hardware.
//
//  3. AN UNWRITTEN CHANNEL ANSWERS THE ZERO RECORD, AND THIS IS THE FAKE
//     ASSERTING A4. This document says NOTHING about an empty channel
//     anywhere — not that an MR of one answers rather than refusing, and not
//     what such an answer would hold. The 590 pair's book prints "If the
//     selected channel is empty, P4 ~ P15 will be 0 and P16 will be blank."
//     (590:1492-1493); reading that sentence across to this radio is the
//     design's A4, whose lift L-HW-3 is THE TS-480 ROW'S RELEASE GATE. NO
//     RADIO HAS CONFIRMED EITHER HALF. The zero shape is at least internally
//     consistent with this row's own hard-wired bytes, which are '0' runs
//     already (480:910-939), and its mode nibble '0' is what this book calls
//     "No mode (Not used for the TS-480)" (480:843) rather than an
//     empty-channel marker — the 590 pair's A18a reading is not available
//     here. The eight-space name is the design's A3, likewise unlifted on
//     this row.
//
//  4. THE TRANSMIT HALF OF A CHANNEL WITH NO STORED RECORD answers the same
//     zero record. What an MR with P1=1 answers where nothing has been
//     written is unprinted here — the design's A9, which gates every Kenwood
//     channel write on all three rows — and this fake gives it the shape
//     entry 3 already carries rather than inventing a rejection or echoing
//     the receive frequency back as a transmit one, which would be a silent
//     split-flattening manufactured inside the test double.
//
//  5. A SET CARRYING AN UNUSED MODE NIBBLE IS STORED. The MD legend prints
//     nibbles 0 and 8 as "No mode (Not used for the TS-480)" and "Tune (Not
//     used for the TS-480)" (480:843, 480:853) and nothing says what a Set
//     carrying one does — the design's A18b, whose lift is a hardware trial.
//     This fake stores the nibble rather than inventing a refusal, which is
//     what lets a test drive core/kw's own build refusal against a real fake.
//
//  6. TONE INDICES ARE STORED, NOT RANGE-CHECKED. TN prints "00 ~ 42"
//     (480:1557) and CN "00 ~ 41" (480:337), both referring their tables out
//     to the instruction manual (480:1559-1560, 480:339-340), and neither
//     says anything about what happens inside a memory frame — the design's
//     A21, unlifted, and on this row not even the 590 pair's "43 or higher
//     results in an error" sentence is printed. Only the field's SHAPE is
//     enforced here. Refusing would assert A21 as a fact about the radio and
//     would put the codec's own refusal out of reach of a real fake.
//
//  7. THE STEP INDEX IS STORED, NOT RANGE-CHECKED, and this is the field the
//     whole TS-480 write refusal turns on. P14 says only "Step size. Refer to
//     the ST command." (480:937, 480:979), and ST's legend is
//     MODE-CONDITIONAL over two different ranges — 00 ~ 04 for SSB/CW/FSK and
//     00 ~ 09 for AM/FM, where 00 means 0.5 kHz in the first and 5 kHz in the
//     second (480:1494-1500). A record carries no way to know which range
//     applies without reading its own mode nibble, and no printed value is a
//     "no change" value: that is the design's A22, which is why every TS-480
//     channel write in this programme is refused. A fake that range-checked
//     P14 would have to pick one of the two legends, and would be asserting
//     A22 lifted.
//
//  8. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in the MW block
//     mentions the selection (480:949-987). A fake that moved it would let a
//     driver depend on a side-effect the book does not describe.
//
//  9. THE SELECTED CHANNEL AT CONSTRUCTION is 00. A radio that has had no MC
//     Set is sitting on some channel and this book prints no power-on value
//     for the selection anywhere. This fake takes the lowest number in its
//     slot space.
//
//  10. THE DEFAULT IMAGE'S RECORD COMPOSITION. Every BYTE of every shipped
//     record is a printed constant, the printed example frequency (480:549),
//     a printed legend value, or the eight-space name — but no MR, MW or MC
//     frame is printed as a literal anywhere in either Kenwood book, so the
//     CROSS-FIELD COMBINATION has never been printed or observed. That is the
//     design's A27, and PROVENANCE.md carries its sentence verbatim together
//     with the family-level entries an image rides on. The records are not
//     observed contents and not factory defaults, and no byte is invented,
//     derived from another model, or padded to make a test pass.
//
//  11. THE DEFAULT TY ANSWER. P1's two bytes are printed "Reserved"
//     (480:1623) and given no legend anywhere, so the shipped value takes the
//     character every OTHER hard-wired field in this book prints, "Always 0"
//     (480:953, 480:973, 480:975, 480:982). P2's shipped value is the FIRST
//     of the four printed variants, "0: TS-480HX (200 W)" (480:1626). Neither
//     is a claim about any radio: no TS-480 has answered this project, and
//     WithTYAnswer is how a test reaches the other three variants, the fifth
//     the document does not print, and the high bytes P1 may carry.
//
//  12. THE INITIAL AI VALUE IS THE POWER-OFF ONE. This book prints no
//     power-on value for AI. What it does print is "When the transceiver is
//     turned OFF, the AI parameter becomes 0." (480:196-198), and this fake
//     reads that ONE STEP ONWARD — a radio that has been turned off and on
//     again reports 0 — rather than inventing a value. The 590 pair's book
//     states its initial state outright (590:81-82) and needs no such step.
//
//  13. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
//     unsolicited, whatever AI is set to, AND ON THIS RADIO THAT IS A LARGER
//     GAP THAN ON THE 590 PAIR, because this book describes the push
//     concretely: "When the extended AI format is selected, the transceiver
//     automatically sends the parameters. When the old AI is ON and the IF
//     parameters change, the transceiver sends the IF command every 1.5
//     seconds." (480:192-195) — the very sentence core/kw's DrainPolicy.Cap
//     is sized against. No TS-480 has been observed by this project;
//     modelling silence is the honest default, not a claim that the radio is
//     silent, and the engine's drain-to-quiet discipline is exercised against
//     internal/fakeradio's own AI-flood facts instead.
//
//  14. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound, the single
//     "?;" per overflow and the discard-to-next-';' resync are this package's
//     own bounded-input policy. No Kenwood book prints a buffer size; what
//     this one prints is that data received but not fully processed produces
//     "O;" (480:143-144), which is a different event and is modelled by
//     WithStreamError instead.
//
//  15. A REFUSAL TO A CONTROL BYTE IS ALWAYS ANSWERED. This book gives a
//     control character in a parameter TWO printed outcomes, not one: "Do
//     not use the control characters 00 to 1Fh since they are either
//     IGNORED or cause a '?' answer" (480:127-129). validNameField refuses
//     one on every call, so this fake always takes the second branch and
//     never the first — a real TS-480 may instead say nothing at all. No
//     design A-number covers this choice; it is this package's own, unlifted,
//     and its lift is a hardware trial: send an MW carrying a 00-1Fh byte in
//     P16 to a real TS-480 and record whether it answers "?;" or stays
//     silent for the reply-timeout window.
//
//  16. THE EX MENU VALUES ARE INVENTED. Every menu's default raw P5 is its
//     printed width in '0' bytes. The parameter list prints each menu's
//     available SETTINGS and never a shipped default (480:424-539), so there
//     is nothing to source a real one from — and `rigprog read --settings
//     --fake` renders these bytes to a user, who must not read them as what a
//     TS-480 ships with. The placeholder is uniform on purpose: an obviously
//     uniform value is harder to mistake for evidence than a plausible-looking
//     spread. ONE ADDRESS IS NOT ARBITRARY, and the coincidence is recorded
//     rather than leant on: at menu 000 the book prints a complete worked
//     answer, "EX00000000; (Display illumination OFF)" (480:415), whose P5 is
//     exactly this byte. What the table DOES carry everywhere is each menu's
//     WIDTH, which is transcribed (ex.go).
//
//  17. AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". A well-formed read naming
//     a menu number this chart does not print draws the rejection, with the
//     state unchanged. The chart prints no such row and the book says nothing
//     about what happens at one; "?;" is the error table's first cause — a
//     syntactically correct command the transceiver cannot execute
//     (480:130-138) — applied to a menu the radio has none of. The sharp case
//     is 061: one past this chart's domain (480:401) and a real menu on both
//     590 rows, which is why no Kenwood fake may borrow another's inventory.
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AS VOCABULARY. On the Yaesu side this is an
// inherited convention with no line to cite. Here the book prints its own
// error table with two named causes (480:126-135), so USING "?;" AS THE
// TOKEN for a refusal is transcription, not assumption. Which refusals
// exist, and whether every one of them answers rather than sometimes
// staying silent, are separate questions — the control-character case is
// the one place this book itself offers a second outcome, and that choice
// is entry 15 above, not this paragraph. The NOTE beneath the error table —
// that the message may not appear at all after a Set (480:136-138) — is
// likewise printed, and is played by WithTransientNAKSuppressed rather than
// assumed away.
//
// THE COMMAND-NAME CASE FOLD. "A command consists of 2 alphabetical
// characters. You may use either lower or upper case characters."
// (480:76-78). A manual fact. Admitting the MIXED case is a consequence of
// folding each name byte independently, not a separate leniency, and field
// values stay case-sensitive because the sentence is about the name.
//
// THE ABSENCE OF FV. A grep of this document for the name returns nothing,
// so refusing "FV;" is transcription rather than a modelling decision — and
// is the one refusal in this package that is a fact about the radio rather
// than about the fake.
//
// THE ABSENCE OF AN ERASE FORM. The 590 pair's book describes a short MW that
// erases a channel (590:1579-1581); this one describes none, so a 42-byte MW
// is not a documented frame on this radio at all and is refused by the same
// width rule that refuses any other malformed width. Nothing is assumed away:
// there is nothing printed to assume about. This programme builds no erase
// frame on any radio in any case — a standing rule of this repository.
//
// THE FIFTY-BYTE WIDTH, THE FIELD OFFSETS AND EVERY LEGEND. Counted and
// transcribed off the two position charts (480:923-943, 480:955-976). Where
// this package and core/kw agree about a byte position, that is two
// independent readings of one chart agreeing — which is the whole point of
// the hard rule — and not a shared definition.
package fakets480
