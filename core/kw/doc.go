// SPDX-License-Identifier: GPL-3.0-or-later

// Package kw is the Kenwood PC-command codec: the ';'-terminated ASCII
// dialect the TS-590S, the TS-590SG and the TS-480 speak, and the
// transport.Framing adapter that presents it to core/transport.Engine.
//
// It never imports core/cat or core/civ. The ';' splitter, the prefix
// matcher and the outbound gate's discipline are COPIED from core/cat's,
// with this family's own maximum-frame constant and its own evidence trail;
// core/kw/imports_test.go is the fence, and it covers every directory
// beneath this one and every test file in them. The reason is spec
// §"CANNOT reuse": core/cat's constants are Yaesu facts — a 7-byte ID
// answer against this family's 6, a 28-byte memory frame against 50, a
// 9-digit frequency field against 11, a hex mode nibble against a decimal
// one, and seven dialect axes named for commands this family does not have.
//
// # What this package's own packages hold
//
// core/kw/ts590 carries the TS-590S and TS-590SG layout values and their
// TWO menu inventories; core/kw/ts480 the TS-480's. Nothing per-row lives
// here.
//
// # The digit ceiling, by name
//
// MaxEXDigits (exdigits.go) is this family's own EX width ceiling, derived
// from DefaultMaxFrame less the EX answer's ten fixed bytes. It is NOT
// internal/extable.MaxDigitsCeiling, which is core/cat's 247, derived from
// a NINE-byte Yaesu overhead; the two differ by exactly one byte, which is
// what would make the copy-paste invisible.
//
// ITS VALUE IS 246, and it is written out here because the three Kenwood
// profile stanzas must carry it as a LITERAL in their DigitsCeiling field:
// internal/extable is build-time tooling that renders this package's source
// text and may not import the package it generates into, so the stanzas
// cannot reference the constant by name. core/kw/exdigits_ceiling_test.go
// pins every registered profile whose ImportPath is this package's to the
// constant, and core/kw/register_test.go pins this sentence to the
// constant's current value, so neither the number above nor the number in a
// stanza can drift away from exdigits.go.
//
// # The one variable-length frame — a negative fact, stated because it is
// # load-bearing
//
// THE EX ANSWER IS THE ONLY VARIABLE-LENGTH FRAME THIS MILESTONE PARSES.
// Its P5 is declared "String of alphanumeric characters for the Menu
// setting (variable length)" (590:555-556) and "A string of characters
// (Variable length). Normally 1-digit for the TS-480. Menu No. 32, 35 and
// 48 ~ 52 use 2-digit parameters" (480:409-411), with no printed ceiling
// anywhere (A19). Every other frame either radio sends is fixed: ID 6, FV
// 7, TY 6, MC 6, MR 50. The TS-890S's floating MA0 terminator is not in
// this pair.
//
// So PrefixLenMatcher's exactLen <= 0 branch has exactly ONE user, and a
// second one appearing is a sign that somebody has mis-read a chart rather
// than found a new frame. It is recorded here because a reader meeting that
// branch would otherwise take it for spare generality.
//
// # E; and O; — the design commitment, and the guarantee at its true
// # strength
//
// Both books print an explicit error-message table, which is more than any
// registered Yaesu manual does, and it says three things:
//
//   - "?;" means EITHER "Command syntax was incorrect" OR "Command was not
//     executed due to the current status of the transceiver (even though
//     the command syntax was correct)" (590:100-105, 480:130-135).
//     Indistinguishable. Treated as a definitive rejection, never retried.
//   - "Occasionally, this message may not appear due to microprocessor
//     transients in the transceiver" (590:106-108, 480:136-138). THE
//     DOCUMENT STATES THAT THE NAK IS UNRELIABLE, so a read that times out
//     carries no information at all: it is neither "absent" nor "rejected".
//     TimeoutError says so in as many words.
//   - "E;" and "O;" are stream-health tokens, not answers, and may arrive
//     unsolicited (A7).
//
// THE ROUTE TAKEN IS Q13's OPTION A: framing implements the OPTIONAL
// transport.FatalFramer hook, so a fatal token is consulted over the WHOLE
// result of one Accumulator.Push before any frame from that chunk is
// delivered, and its typed cause closes the port from the reader goroutine
// itself.
//
// THE THEOREM. A fatal frame is RECEIVED when its publication has taken and
// released the engine's fatal gate. The guarantee is then exactly: NO FRAME
// LEAVES THE HOST AFTER A FATAL FRAME THE ENGINE HAS RECEIVED. It holds
// because the gate totally orders the publication's critical section
// against the final write's — either the publication went first (the
// write's closed recheck sees the closure and writes nothing) or the write
// went first (the frame left BEFORE the fatal frame was received). There is
// no third interleaving.
//
// WHAT THAT COST, AND WHAT IT BOUGHT. The alternative — the accumulator
// returning a typed error and nothing else — needs no core/transport change
// at all, and this milestone's headline claim would then have been "no
// core/cat and no core/transport file changes". It was not taken because
// its honest guarantee is weaker than it reads: fatal at the NEXT
// interaction, with THREE residual windows, all three in core/transport and
// none closable from here — (i) the answer-before-error ordering, in which
// a chunk of "«matching answer»;E;" yields a SUCCESSFUL read and leaves the
// failure for the next call, so a per-channel write verification can report
// a match on the very chunk that carried the failure; (ii) the entry
// purge's own bound under a flood; and (iii) the post-purge race, which
// needs no flood at all. Option A closes all three. The cost is an
// engine.go and framing.go edit, so the claim narrows to "no core/cat file
// changes" and the byte-identity delta count is four classes.
//
// TWO LIVENESS FACTS, because the theorem is narrower than "the port closes
// immediately in every state" and must not be read as it:
//
//  1. Port.Write runs INSIDE the gate, with no write timeout. A write that
//     blocks — a wedged line, a stalled USB endpoint — holds the gate until
//     the driver returns, so a fatal frame arriving during that write is
//     received only once the write returns. It is not a deadlock: the write
//     side never waits on the reader goroutine.
//  2. The typed cause is universal ONLY when the fatal publication wins the
//     FIRST close. closePort keeps the first cause any caller supplies, and
//     Engine.Close, a terminal read error, a consumed reader error and the
//     gated write's own error branch each reach it without queueing for the
//     gate. When one of those wins, the port is (correctly) already closed
//     but the fatal frame's own reason is lost.
//
// A token is NEVER matched as an answer, NEVER collapsed into ErrRejected,
// and NEVER retried. core/kw/fatalframer_test.go carries the four-state
// injection matrix — read wait, write error-window, drain, no command
// outstanding, times two tokens, times two books — and re-runs
// core/transport's two adversarial pins through this accumulator and this
// typed cause.
//
// # No Set is acknowledged
//
// Neither book prints an acknowledgement for any Set. "Silence means it
// worked" is unprinted, as it is across the whole Yaesu fleet, and stays
// A6 — but here it is worse, because the manual has told us the NEGATIVE
// signal is unreliable too. Every write is followed by a read-back; a write
// whose read-back does not match fails, and a write followed by silence is
// reported as INCONCLUSIVE, not as success.
//
// # Errata schedule — recorded, not resolved
//
// Twenty-one rows in THREE categories, and the categories matter: two of
// the twenty-one are not defects at all.
//
// NINETEEN DOCUMENT DEFECTS:
//
//	E1  590SG  The MR Read chart's terminator cell prints ':' where every
//	           other chart prints ';' (590:1442).
//	E2  590SG  "TS-590SG extension channel numbers E00 ~ P09" for E00 ~ E09
//	           (590:1346).
//	E3  590SG  The front matter says a command "is composed of a 2 letter
//	           alphabetical command name"; §Command says "A command consists
//	           of 2 or 3 characters"; VS0 exists (590:12-13 vs 590:62,
//	           590:2496).
//	E4  590SG  SC Set's parameter is P1 but the Answer's first parameter is
//	           P2, with near-identical legends (590:1974 vs 590:1981).
//	E5  590SG  VR renumbers P1 to P2 between Set and Answer AND the Answer
//	           reports a different datum — whether the VGS-1 board is
//	           installed — so it never reports the setting (590:2476,
//	           590:2483).
//	E7  590SG  MR P11's and MW P11's firmware notes say the same thing in
//	           different words, and MW's prints a mismatched quotation mark
//	           (590:1478 vs 590:1564).
//	E8  480    MR P8 cross-refers to "page 35" in a 24-page document; the
//	           parallel MW P8 refers to TN instead and spells it "Refero"
//	           (480:924, 480:966).
//	E9  480    The block headed XI prints its Read and Answer charts as
//	           "X T", colliding with the real XT two pages later, so an
//	           XT-prefixed answer is ambiguous between 4 and 17 bytes
//	           (480:1745, 480:1753, 480:1755-1761 vs 480:1790-1803).
//	E10 480    TY is headed "Sets or reads the microprocessor fimware type"
//	           and has an empty Set chart, so it is read-only despite the
//	           heading (480:1621, 480:1625).
//	E11 480    XO P1 legend reads "0: Plus diretion" (480:1771).
//	E12 480    MD names nibble 9 "FSR (FSK Reverse)" and nibble 7 "CWR"
//	           where the 590SG names the same nibbles FSK-R and CW-R
//	           (480:852, 480:854 vs 590:1361, 590:1363).
//	E13 both   "O;" is given DIFFERENT causes in the two books — "a receive
//	           buffer overrun error" versus "Receive data was sent but
//	           processing was not completed" (590:113 vs 480:143-144). This
//	           is why StreamError carries a Book.
//	E14 480    Nineteen rows read exactly "Always 0 for the TS-480"; one
//	           more drops the article (480:292); with the "Always 00",
//	           "Always 000" and "Always 000000000" variants the hard-wired
//	           family is 26 printed rows.
//	E15 480    No revision number, no B5A-/B62- part code, no firmware
//	           statement anywhere, an explicit no-support disclaimer, and TY
//	           P1 "Reserved" — the radio has no CAT-readable firmware
//	           version (480:8-9, 480:12-15, 480:1623).
//	E16 590SG  The EX Set/Answer charts print a ';' at a nominal position
//	           although P5 is declared "(variable length)" — the chart is
//	           illustrative, not a width (590:547, 590:556-560).
//	E17 both   The template prints Set / Read / Answer labels whether or not
//	           the direction exists; an absent direction is an EMPTY CHART
//	           UNDER A PRINTED LABEL (MR Set, MW Read and Answer, ID Set, TY
//	           Set). A transcriber who reads the label as evidence invents
//	           three commands per block (480:911, 480:980, 480:985, 480:679,
//	           480:1625).
//	E19 590SG  The MW erase note reads "If you do not specify one digit in
//	           P16", whose intended sense is almost certainly "no digits" —
//	           the whole of the 42-byte question (590:1579-1581).
//	E20 590SG  RD and RU are opposites and BOTH are described as "to
//	           increase the scan speed"; the book does not say which is
//	           wrong. Neither command is built here (590:1846, 590:1848).
//	E21 480    IF's P14 prints one tone-number field of range "00 ~ 42" and
//	           refers it to "the TN and CN command", but the book's own CN
//	           prints 00 ~ 41, so the field admits an index CN does not have
//	           (480:725 vs 480:337). IF is not built here; it is recorded
//	           because a later milestone that builds IF must not read P14
//	           against CN.
//
// ONE ANTI-DEFECT:
//
//	E18 480    "Se t" for "Set" throughout is PageMaker letter-spacing, NOT
//	           a document defect — recorded so no transcriber "corrects" it
//	           into evidence (480:155, 480:338, 480:679, 480:829, 480:1143,
//	           480:1515). It is also why SplitFrames' tolerance for
//	           consecutive terminators is justified on robustness and NOT on
//	           the 480's "; ; ; ; PS1;" wake-up string, whose printed
//	           spacing may be the same artefact.
//
// ONE TRANSCRIPTION TRAP THAT IS NOT A DEFECT:
//
//	E6  590SG  SS numbers the section channels 0-9 where MC numbers them
//	           100-109. 590:2181 is SS's own P1 domain, so this is a
//	           cross-command numbering convention the book is entitled to
//	           have; it is listed because a transcriber who carries SS's
//	           numbers into an MC frame writes the wrong channel
//	           (590:2181 vs 590:1345).
//
// # The ASSUMED register — the authoritative copy
//
// TWENTY-EIGHT A ROWS, and this is where they live. Nothing below is
// evidence. Each row states what is assumed, its LIFT ID and its EXACT
// SCOPE — which registry rows it claims, and where the scope is per mode or
// per path.
//
// THE SCOPING RULE, WHICH IS THE WHOLE POINT OF THE REGISTER. A lift is
// stated PER REGISTRY ROW, and a lifted result applies to the rows actually
// observed and to no others. This design registers THREE rows — TS-590S,
// TS-590SG, TS-480 — so no entry may name the 590 pair with a bare
// singular, because that names two of them. The two radios have different
// firmware, different menu domains (88 rows against 100, A26) and a byte
// whose liveness differs between them (A14); that Kenwood prints them in
// one book is a property of the book, not of the radios. Where an entry can
// only ever be observed on one row it is written as a one-row entry with
// the other row's status stated explicitly (A11, A12, A13, A14 are the
// model). Lift IDs are L-HW-n (a wire observation, tied to a numbered item
// in the hardware-confirmation list), L-DOC-n (a named document read at a
// named place) and L-DEC-n (a decision). Three lifts are NOT wire
// observations and could not be: A15, A17 and A26.
//
// K-D1 and K-D2 ARE NOT HERE. They are DRIVER-register entries — the
// TS-480's P7 direction semantics, and the control-line policy at open —
// and they land in core/driver/ts480/doc.go and core/driver/ts590/doc.go.
// The prefix records where a correction lands: correcting an A-number is a
// design change, correcting a K-number is a driver-package change, and no
// task may quietly move one for the other. Twenty-eight A rows here plus
// those two K rows are the complete thirty-row register.
//
//	A1   The memory name is padded to 8 bytes with SPACES on write and
//	     right-trimmed on read. Neither book states the rule for P16; the
//	     480's KY gives a same-document precedent for a different command
//	     (480:785-787).
//	     LIFT L-HW-2, observing MW then MR, ONCE PER REGISTRY ROW — on a
//	     TS-590S, on a TS-590SG and again on a TS-480: write a 3-character
//	     name, read it back, inspect bytes 45-49 for spaces versus any other
//	     filler. EACH TRIAL LIFTS ONLY THE ROW IT WAS RUN ON. On the 480
//	     this is a WRITE, so it is unavailable until item 3 has registered
//	     that row.
//
//	A2   The name charset is printable ASCII 0x20-0x7E excluding ';', and
//	     THIS ENTRY'S CLAIM IS BOUNDED AT 0x7F. 0x80-0xFF is unevidenced AND
//	     unclaimed: the programme refuses those bytes by its own charset
//	     rule and this design says nothing about what either radio would do
//	     with one. The 590SG prints only "';' cannot be used" (590:1577);
//	     the 480 prints nothing on P16 but forbids 00-1Fh and ';' generally
//	     (480:108-110, 480:127-129), which does not exclude 0x7F or above.
//	     LIFT L-HW-7, observing MW then MR, ONCE PER REGISTRY ROW — on a
//	     TS-590S, on a TS-590SG and again on a TS-480: P16 carrying 0x20,
//	     0x7E and 0x7F in turn, read back, recording which are stored,
//	     refused or normalised. EACH LIFTS ITS OWN ROW ONLY — a charset is a
//	     per-firmware normalisation rule and the siblings do not share
//	     firmware. Lifting it does NOT license 0x80-0xFF.
//
//	A3   An empty channel's P16 comes back as 8 spaces. The 590SG says P16
//	     "will be blank" (590:1492-1493) without defining blank; the 480
//	     says nothing about an empty channel at all.
//	     LIFT L-HW-2b, observing MR: read an unwritten channel and inspect
//	     bytes 42-49 — ONCE PER REGISTRY ROW, on a TS-590S, on a TS-590SG
//	     AND on a TS-480, each lifting its own row. The 480 half costs no
//	     extra session: it is item 3's own frame, read for its name bytes
//	     instead of its zero bytes.
//
//	A4   An MR of an empty channel ANSWERS (with P4-P15 zero) rather than
//	     rejecting, ON THE TS-480 TOO. Documented for the 590SG
//	     (590:1492-1493); entirely unprinted for the 480.
//	     LIFT L-HW-3 — THE RELEASE GATE, observing MR: read an unwritten
//	     TS-480 channel. Hardware item 3 gives the five possible wire
//	     outcomes and the repeat count, because a single trial is
//	     inconclusive. TS-480 ROW ONLY.
//
//	A5   The short MW erase form is 42 bytes, P16 wholly omitted.
//	     590:1579-1581's sentence admits at least two readings (E19).
//	     LIFT L-HW-11, observing the MW family: send a 42-byte MW to a
//	     channel of known content, read it back, then send a 49-byte one and
//	     observe whether it is refused. NEVER BUILT by this milestone, so
//	     the lift is not on any shipping path; recorded one-per-row for
//	     consistency, which costs nothing because nothing consumes it.
//
//	A6   Silence after an MW means the write was accepted. No Set
//	     acknowledgement is printed in either book, and both say "?;" may
//	     not appear (590:106-108, 480:136-138).
//	     LIFT L-HW-4, observing the MW FAMILY specifically — not some other
//	     Set: write, then read back, on a channel whose prior content is
//	     known, ONCE PER REGISTRY ROW: on a TS-590S, on a TS-590SG and again
//	     on a TS-480. A6 is what makes every MW on a row believable, so an
//	     unobserved sibling must not inherit it. EACH LIFTS ITS OWN ROW.
//
//	A7   E; and O; are stream-health tokens that may arrive unsolicited and
//	     are never an answer. Both books list them beside "?;" in one
//	     error-message table without saying whether they are correlated to a
//	     command (590:97-113, 480:126-144).
//	     LIFT L-HW-13, observing the tokens themselves, in TWO halves: (a)
//	     hold a port open with no command outstanding and log every byte for
//	     a full session, to see whether either arrives unsolicited; and (b)
//	     INDUCE one — a burst at the wrong baud — while an MR is
//	     outstanding, recording whether the token arrives before, with, or
//	     instead of the answer. Nothing weaker than (b) distinguishes
//	     "unsolicited" from "correlated". BOTH HALVES ON EACH OF THE THREE
//	     REGISTRY ROWS, each row's pair lifting that row only: E13 records
//	     that the two BOOKS disagree about O;, so token behaviour is the one
//	     thing this pair is documented to differ on, and a buffer-overrun
//	     threshold is a firmware property rather than a family one.
//
//	A8   An MR with P1=0 on a SPLIT channel returns the RX side and never
//	     fails. 590:1444-1447 describes the intent, not the failure mode.
//	     LIFT L-HW-1a, observing the MR family: set a channel split from the
//	     front panel with a known TX/RX pair, read it with P1=0, and compare
//	     the answered P4 against the panel's RX frequency — the answer must
//	     be the RX side and must not be "?;". ONCE PER REGISTRY ROW: on a
//	     TS-590S, on a TS-590SG and on a TS-480, each trial lifting one.
//
//	A9   An MR with P1=1 on a SIMPLEX channel is not safe to send blind.
//	     Wholly unprinted on both radios.
//	     LIFT L-HW-1, observing the MR family: send it to a known-simplex
//	     channel and observe answer versus "?;" versus silence. ONCE PER
//	     REGISTRY ROW: on a TS-590S, on a TS-590SG and on a TS-480. This is
//	     the sharpest instance of the scoping defect: A9 gates EVERY Kenwood
//	     channel write on all three rows, so a TS-590S observation unblocks
//	     the S row AND THE S ROW ONLY — the SG's writes stay gated until an
//	     SG has been observed, and vice versa. Three trials, three
//	     independent unblockings; no partial credit and no sibling
//	     inheritance.
//
//	A10  MR/MW P2 accepts '0' for a channel below 100 on the two 590 rows.
//	     Documented for MC Set (590:1334-1335); MR/MW only say "refer to the
//	     MC command" (590:1453, 590:1539-1540). This programme always emits
//	     '0' and always accepts either on parse.
//	     LIFT L-HW-6a, observing BOTH the MR and the MW family — not MC —
//	     and on EACH of the two 590 rows separately: (a) read channel 007
//	     with P2 '0' and again with P2 ' ', confirming both answer the same
//	     record; (b) WRITE channel 007 with P2 '0' and read it back,
//	     confirming the write landed on 007 and not elsewhere. The MW half
//	     is the dangerous one: a P2 the radio reads differently on a Set
//	     writes a channel the operator did not name. FOUR TRIALS (two
//	     families x two rows); each row's pair lifts that row. THE TS-480 IS
//	     UNCLAIMED by this entry and stays so.
//
//	A11  Extension channels 110-119 hold an ordinary 50-byte record.
//	     "TS-590SG extension channel numbers" is the whole of the
//	     explanation (590:1346-1347).
//	     LIFT L-HW-6, observing MR at the extension boundary on a TS-590SG:
//	     read 110 and 119, confirm each answers a 50-byte record parsing
//	     under the same grid, then write 110 and read it back. L-DOC-2, the
//	     TS-590SG instruction manual, would corroborate what an extension
//	     channel IS but cannot settle the record's shape. TS-590SG ROW ONLY.
//	     RULED 05/09/2026, Stuart decision row 6 — OMIT: until this lifts,
//	     the ten slots are omitted from the SG row rather than published on
//	     this assumption alone; publication once A11 lifts is an additive
//	     later change, not a correction.
//
//	A12  The TS-590S's slot space stops at 109. The book says 110-119 is the
//	     SG's; it never states the S's ceiling (590:1345-1347).
//	     LIFT L-HW-6b, observing MR at the same boundary on the OTHER
//	     sibling: read 109 on a TS-590S (which must answer) and 110 (answer
//	     versus "?;" versus silence). Two frames, one radio. TS-590S ROW
//	     ONLY.
//
//	A13  ON THE TS-590S, FV's answer is always the four-character form
//	     M.NN. One worked example, "FV1.00;" (590:1035); the chart pins the
//	     WIDTH but not the grammar.
//	     LIFT L-HW-5, observing FV itself: "FV;" repeated on a TS-590S at
//	     two firmware levels, recording the exact answer bytes each time.
//	     The grammar lifts only if every answer is four characters of the
//	     form digit-dot-digit-digit, and only for the S row. The width
//	     corroborator at 590:749 is SG-ONLY — it is a row of the TS-590SG
//	     parameter list, and the S's menu 000 is Display brightness
//	     (590:569) — so on the S the width rests on the answer chart alone.
//	     THE TS-590SG IS EXPLICITLY UNCLAIMED: no session has read an SG's
//	     FV answer, and nothing downstream reads FV to decide anything on
//	     that row, because A14 fixes the SG's byte 28 as live by
//	     construction.
//
//	A14  On a TS-590S reporting FV < 2.00, byte 28 is a printed-fixed '0';
//	     on every TS-590SG and on a TS-590S at >= 2.00 it is live.
//	     590:1478 and 590:1564 say "always 0 in firmware version 1.xx of
//	     TS-590S" and say nothing about 2.xx.
//	     LIFT L-HW-5a, observing FV and then byte 28 through MR/MW: on a
//	     TS-590S at 1.xx, read FV, then read a channel and check byte 28 is
//	     '0', then write that channel with byte 28 '1' and read it back to
//	     see whether it is stored, normalised or refused; then the whole
//	     sequence again on a TS-590S at >= 2.00. An FV read alone settles
//	     nothing about the byte.
//
//	A15  Default baud 9600 on both radios. Neither book prints a factory
//	     value (590:52-53, 480:533).
//	     LIFT L-DOC-3 or L-DEC-1, AND IT CANNOT BE A WIRE OBSERVATION:
//	     reading menu 056 over EX presupposes a working session at the very
//	     baud in question, so the only lifts are the instruction manual's
//	     menu-056 page, the radio's own front-panel menu read by an owner,
//	     or Stuart stating the value.
//
//	A16  MC SET is narrowed to ordinary memory. A decision, not a document
//	     fact — 590:1345-1347 says the section and extension numbers are
//	     selectable.
//	     LIFT L-DEC-2. None needed on evidence: it is a deliberate
//	     narrowing, recorded so a later widening is a reviewed decision
//	     rather than a drift.
//
//	A17  Frequency bounds. Neither book prints a range for FA/FB or for
//	     MR/MW P4 — only "11 digits in Hz" (590:961-966, 480:548-550).
//	     LIFT L-DOC-1, the instruction manuals' General Specifications
//	     tables. NO WIRE OBSERVATION LIFTS IT: the PC-command documents
//	     print a field WIDTH, and a width is not a tuning range.
//
//	A18a NOT ASSUMED — DOCUMENTED. A mode nibble 0 in an MR ANSWER is the
//	     empty-channel marker on the two 590 rows; the parser accepts it and
//	     maps the slot to an empty channel, testing "P4-P15 all zero" rather
//	     than P5 alone. 590:1492-1493: "If the selected channel is empty, P4
//	     ~ P15 will be 0 and P16 will be blank." P5 is byte 18, inside
//	     P4-P15.
//	     NO LIFT NEEDED — it is documentary fact. Kept in the register so
//	     that no later reader restores draft 1's refusal. The TS-480 half is
//	     A4, and unlifted.
//
//	A18b A mode nibble of 0 or 8 is never legal on an MW BUILD, and such a
//	     record is refused. Both books call the two nibbles "None (setting
//	     failure)" / "Not used" (590:1353, 590:1362; 480:843, 480:853)
//	     without saying what a Set carrying one does.
//	     LIFT L-HW-12, observing the MW family: write a channel with P5=0,
//	     then read it; THEN THE SAME WITH P5=8; ON EACH OF THE THREE
//	     REGISTRY ROWS — a TS-590S, a TS-590SG and a TS-480 — seeing in each
//	     case whether the radio refused, erased the channel, or stored a
//	     nibble it cannot display. SIX TRIALS, and each row's pair lifts
//	     that row alone. The two nibbles are not interchangeable: 0 is
//	     "None (setting failure)" in both books while 8 is "Tune (Not used
//	     for the TS-480)" on the 480 and another setting-failure on the 590
//	     pair, so a 0 trial says nothing about 8.
//
//	A19  The EX answer's P5 never exceeds the width the parameter list
//	     prints for that menu number. "Variable length" with no ceiling
//	     (590:555-556, 480:409-411); the 590SG's chart prints an
//	     illustrative ';' position (E16).
//	     LIFT L-HW-14, observing the EX family EXHAUSTIVELY: read EVERY
//	     address in the model's own domain — 88 on the TS-590S, 100 on the
//	     TS-590SG, 61 on the TS-480 — and record each answer's P5 width
//	     against the transcribed width. A CEILING IS NOT LIFTED BY A SAMPLE.
//
//	A20  The TS-480's XT-prefixed answers are the 4-byte XT form, and the
//	     17-byte form printed under the XI heading is a paste error. E9: the
//	     XI block's own charts spell "X T" (480:1745, 480:1753, 480:1757).
//	     LIFT L-HW-15, observing XI and XT SEPARATELY: send "XI;" and record
//	     the answer's total byte count, then "XT;" and record its; the
//	     paste-error reading holds only if the two differ (17 versus 4).
//	     NEITHER FRAME IS BUILT OR PARSED by this milestone and the outbound
//	     gate refuses both, so the lift needs a one-off diagnostic OUTSIDE
//	     the shipping programme. TS-480 ROW ONLY.
//
//	A21  A tone index outside the printed range is refused rather than
//	     clamped. The 590SG says "An entered value of 43 or higher results
//	     in an error" for TN (590:2309) and nothing for CN; the 480 says
//	     nothing for either.
//	     LIFT L-HW-8, observing the TN and CN families themselves and then
//	     the MW family: send "TN43;" and "CN42;" and observe "?;" versus a
//	     clamped read-back; then write a record carrying P8=43 and observe
//	     whether the same rule holds inside a memory frame. ONCE PER
//	     REGISTRY ROW: on a TS-590S, on a TS-590SG and on a TS-480. The TN
//	     sentence sits under a block headed "[TS-590S / TS-590SG common]"
//	     (590:2287), so THAT half is documented for both 590 rows — but CN
//	     is unprinted on both, what the rule does INSIDE an MW record is
//	     unprinted on all three, and clamp-versus-refuse is firmware
//	     behaviour rather than a family property. EACH TRIAL LIFTS ITS OWN
//	     ROW.
//
//	A22  NO TS-480 P14 VALUE IS KNOWN TO BE A "NO CHANGE" VALUE, so this
//	     milestone cannot write P14 at all — which is why EVERY TS-480
//	     channel write is refused. ST's legend is mode-conditional over two
//	     different ranges (480:1494-1500), so no flat TuningSteps list is
//	     truthful; the document does not say what P14 value is "no change";
//	     and with FieldTuningStep published Unsupported the source channel
//	     does not retain the raw index.
//	     LIFT L-HW-16, observing the MW family, ONE TRIAL PER ST MODE CLASS:
//	     ST has two distinct legends — SSB/CW/FSK, indices 00-04
//	     (480:1494-1495), and AM/FM, indices 00-09 (480:1497-1500) — and
//	     index 00 means 0.5 kHz in the first and 5 kHz in the second. So: on
//	     a channel in an SSB/CW/FSK mode, set a non-00 step from the front
//	     panel, write that channel with P14=00, and watch the panel; THEN
//	     THE WHOLE TRIAL AGAIN on a channel in an AM or FM mode. Both
//	     classes inert implies 00 is a safe no-change value and TS-480
//	     channel writes become possible; either class not inert confirms the
//	     refusal as the only honest option, AND A PARTIAL RESULT LIFTS
//	     NOTHING, because the write path has no way to branch on mode class
//	     for a field the model does not carry. TS-480 ROW ONLY — it is the
//	     one entry that never had a 590 half.
//
//	A23  NO TS-590 P14 VALUE IS KNOWN TO BE MEANINGFUL OUTSIDE FM, so a
//	     channel write whose mode is not FM is refused. 590:1569-1571 gives
//	     P14 only two values, "00: FM Normal" and "01: FM Narrow", and says
//	     nothing about what it means in SSB, CW, AM or FSK.
//	     LIFT L-HW-17, observing the MW family, ONE TRIAL PER NON-FM MODE:
//	     write a channel with P14=00 and again with P14=01, read both back,
//	     and see whether the byte is stored, normalised or refused —
//	     REPEATED IN EACH NON-FM MODE THE ROW PUBLISHES (LSB, USB, CW, CW-R,
//	     AM, FSK, FSK-R), not in SSB alone, AND REPEATED ON EACH 590
//	     REGISTRY ROW, a TS-590S and a TS-590SG. THE LIFT IS A GRID, MODE x
//	     ROW, AND THE ENABLING IS PER CELL: a (row, mode) pair whose trial
//	     shows P14 inert becomes writable on that row in that mode, and
//	     every untrialled cell stays refused. FOURTEEN CELLS, and a full
//	     sweep of one row enables that row only.
//
//	A24  On the TS-480, a printed-fixed byte is REQUIRED ON PARSE, not
//	     merely emitted on build. 480:108-110 says digits for a parameter
//	     "not applicable to this transceiver" may be filled with any
//	     character except the control codes and ';' — that governs the SET
//	     side, but it means a radio answering a hard-wired byte with
//	     something else is not necessarily faulty. Strictness is this
//	     programme's choice.
//	     LIFT L-HW-18, observing the MR family: read a dozen channels off a
//	     real TS-480 and check whether all sixteen hard-wired bytes come
//	     back as the printed constants. A single counter-example converts
//	     the rule from "required" to "accepted and normalised". TS-480 ROW
//	     ONLY.
//
//	A25  Kenwood radios do not echo the host's own frames back on the line.
//	     Unprinted in both books. It is what makes framing.NoteSent a no-op.
//	     LIFT L-HW-19, observing the AI SET — the only Set this milestone's
//	     InitSequence transmits: send "AI0;" with nothing else outstanding
//	     and read the port for one full drain period; anything returned is
//	     an echo. Repeat on EVERY (row, path) PAIR THIS DESIGN REGISTERS —
//	     FIVE LEGS: the TS-590S's USB-B virtual COM port and its RS-232C
//	     connector; the TS-590SG's USB-B port and its RS-232C connector; and
//	     the TS-480's 9-pin D-sub RS-232 port, which is the only path that
//	     radio has (480:19-25, 480:38-68). Echo is produced by a radio's own
//	     serial stack and, on a USB-B leg, by the Kenwood-supplied virtual
//	     COM driver (590:35-41) — neither is shared between two radios by
//	     virtue of a shared connector type. EACH LEG LIFTS ITS OWN (row,
//	     path); A ROW IS LIFTED WHEN ALL OF ITS PATHS ARE.
//
//	A26  The printed EX parameter lists cover their declared domains with no
//	     gaps — 88 rows on the TS-590S, 100 on the TS-590SG, 61 on the
//	     TS-480 — so those are the three profiles' ExpectedRows. The DOMAINS
//	     are printed (590:543, 590:544, 480:401); the counts are arithmetic
//	     over those ranges, not a count of printed rows.
//	     LIFT L-DOC-4 — NOT A WIRE OBSERVATION AND NOT HARDWARE: the
//	     transcription leg's own boundary ledger, derived from a render of
//	     the parameter-list sections BEFORE any transcription exists. IF THE
//	     LEDGER AND THE ARITHMETIC DISAGREE, THE LEDGER WINS, and this row
//	     is what says so.
//
//	A27  The 50-byte record a fake image serves is a record the radio would
//	     actually answer with — i.e. the cross-field COMBINATION of
//	     individually printed constants, examples and legend values is
//	     itself legal. NO MR, MW OR MC FRAME IS PRINTED AS A LITERAL
//	     ANYWHERE IN EITHER BOOK. Each byte's value is evidenced; their
//	     co-occurrence in one record is not, and no amount of re-reading the
//	     charts can evidence it.
//	     LIFT L-HW-2 / L-HW-3, observing MR: the first real MR answer is the
//	     first observed record this project will hold, and lifts this FOR
//	     THE REGISTRY ROW IT CAME FROM AND NO OTHER. THREE ROWS, THREE FIRST
//	     ANSWERS. Until a row has one, the fake proves the codec
//	     self-consistent for that row and proves nothing about that radio.
package kw
