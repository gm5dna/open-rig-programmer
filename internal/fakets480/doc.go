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
// The fence is enforced mechanically and recursively (imports_test.go), from
// birth, ahead of the gen/ subdirectory a later task of this milestone's plan
// brings in.
//
// # A SIBLING of internal/fakets590, not a copy of it
//
// The two Kenwood fakes share a shape — the pipe-and-goroutine Radio, the
// bounded reassembler, the "?;" convention, the per-command handlers, the
// Image contract, the options — and share NO CODE and NO TABLE. The hard
// rule above forbids the import that would let them, and the radios do not
// in fact agree: this book gives several bytes of the same 50-byte grid
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
//     statement either (erratum E15). TY is the nearest command and reports
//     a hardware variant.
//
// # What this fake deliberately does NOT model
//
// EX (MENU), IN EITHER DIRECTION. This book prints a full EX chart
// (480:399-416) and the best-documented menu legend of any radio in this
// programme, and the fake's inventory is to come from its own copy of that
// chart's independent transcription — the two-source evidence design this
// project uses on the Yaesu side. That work is the NEXT task of this
// milestone's plan; until it lands, an EX frame draws "?;". That is a
// MODELLING GAP, KNOWN-DIVERGENT from the documented grammar, and it is not
// a claim that this radio refuses EX. TestEX_IsNotModelledYet pins the gap's
// shape so that adding EX has to change a test rather than fill a silence.
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
//  2. THE DEFAULT TY ANSWER. P1's two bytes are printed "Reserved"
//     (480:1623) and given no legend anywhere, so the shipped value takes the
//     character every OTHER hard-wired field in this book prints, "Always 0"
//     (480:953, 480:973, 480:975, 480:982). P2's shipped value is the FIRST
//     of the four printed variants, "0: TS-480HX (200 W)" (480:1626). Neither
//     is a claim about any radio: no TS-480 has answered this project, and
//     WithTYAnswer is how a test reaches the other three variants, the fifth
//     the document does not print, and the high bytes P1 may carry.
//
//  3. THE INITIAL AI VALUE IS THE POWER-OFF ONE. This book prints no
//     power-on value for AI. What it does print is "When the transceiver is
//     turned OFF, the AI parameter becomes 0." (480:196-198), and this fake
//     reads that ONE STEP ONWARD — a radio that has been turned off and on
//     again reports 0 — rather than inventing a value. The 590 pair's book
//     states its initial state outright (590:81-82) and needs no such step.
//
//  4. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
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
//  5. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound, the single
//     "?;" per overflow and the discard-to-next-';' resync are this package's
//     own bounded-input policy. No Kenwood book prints a buffer size; what
//     this one prints is that data received but not fully processed produces
//     "O;" (480:143-144), which is a different event and is modelled by
//     WithStreamError instead.
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF. On the Yaesu side this is an inherited
// convention with no line to cite. Here the book prints its own error table
// with two named causes (480:126-135), so using "?;" for every refusal is
// transcription, not assumption. The NOTE beneath it — that the message may
// not appear at all (480:136-138) — is likewise printed, and is played by
// WithTransientNAKSuppressed rather than assumed away.
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
package fakets480
