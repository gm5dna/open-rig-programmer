// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets2000 simulates the Kenwood TS-2000/TS-2000X/TS-B2000's
// PC-control behaviour over an in-memory serial connection (Radio.Port()).
// It is the test double the ts2000 registry rows run against: the transport
// engine, core/driver/ts2000, and, once wired, the CLI's --fake mode and the
// GUI's demo mode — the role internal/fakets590 plays for the TS-590 pair.
//
// ONE PACKAGE, THREE ROWS, NO BARE New. The manual names all three models on
// its own front-matter page and prints one shared command block for them
// (ts2000:100-101, PDF p.3), so New takes zero arguments and WithModelName is
// a plain Option — the ic7851/ic7850 mechanism (internal/fakeic7851) — rather
// than internal/fakets590's required Row argument. Without an option, New
// resolves to "TS-2000": see WITH NO OPTION, THE MODEL IS "TS-2000" below.
//
// # The hard rule: NOTHING project-internal
//
// fakets2000 MUST NOT import any package of this project — not core/kw, not
// core/kw/ts2000, not core/codeplug, not core/spec, and not any sibling fake.
// Standard library only, in every non-test file, in this directory AND every
// directory beneath it. Every byte offset, field width and validation rule
// below is re-derived from the TS-2000/2000X/B2000 instruction manual's own
// PC CONTROL COMMAND TABLES appendix — cited "ts2000:NNNN" throughout, a line
// of docs/fixtures-private/manuals/ts2000_manual_50_layout.txt (the manual
// itself is gitignored) — and from docs/superpowers/ts2000-capability-matrix.md,
// never from core/kw/ts2000 or core/driver/ts2000.
//
// This is not a style preference: if this fake reused core/kw's codec, a
// systematic bug in that codec — an off-by-one in a field offset, a
// validation rule subtly wrong — would be applied identically on both sides
// of every "send a command, check the reply" test this project runs. The bug
// would never surface. The fake has to be able to DISAGREE with the
// production codec for a test against it to mean anything, and it can only
// disagree if it was built from the manual rather than from the code. The
// reasoning is internal/fakets590's own, restated for this package because
// the quarantine forbids reading core/driver/ts2000's report beyond its
// Verdict, let alone its code.
//
// The fence is enforced mechanically and recursively (imports_test.go): this
// directory and every one beneath it.
//
// THE ONE EXCEPTION IS internal/fakepipe: the net.Pipe pair, the goroutine
// bookkeeping, the interruptible latency wait and the raw write. It is
// permitted because it is PROTOCOL-FREE — it sees []byte and a duration and
// nothing else, so it carries no framing, no field layout and no reply
// building. A bug in it therefore cannot make a wrong codec look right; it
// can only stop bytes moving, which this package's own tests notice at once.
// Everything above the wire — the reassembler, the parser, the image, the
// replies — stays here, written independently of every sibling Kenwood fake.
//
// # A SIBLING of internal/fakets590 and internal/fakets480, sharing no table
//
// The three Kenwood fakes share a shape — the pipe-and-goroutine Radio, the
// bounded reassembler, the "?;" convention, the per-command handlers, the
// Image contract, the options — and share NO PACKAGE, NO TABLE AND NO LEGEND:
// every offset, width, legend and validator in this file is re-derived from
// this row's own manual, independently of the other two. What differs here,
// and is this book's own:
//
//   - ELEVEN OF SIXTEEN PARAMETER SLOTS MATCH THE 590/480 FRAME EXACTLY, but
//     FIVE DO NOT: P10 (DCS code), P11 (REVERSE), P12 (Shift status), P13
//     (Offset frequency) and P15 (Memory Group) carry LIVE data here where the
//     590/480 grid prints a constant on all five — the matrix's whole point
//     (docs/superpowers/ts2000-capability-matrix.md §2). There is therefore
//     NO printed-fixed byte anywhere in this row's own record: every MW this
//     fake accepts validates all sixteen fields as data, never as a required
//     constant.
//   - P7 (tone mode) prints four values — "0: OFF, 1: TONE, 2: CTCSS, 3: DCS"
//     (ts2000:10703-10704, 10773-10774) — where the 590 pair's fourth value is
//     Cross Tone, not DCS (matrix §2, erratum recorded there, not here).
//   - THE HUNDREDS-DIGIT BYTE (P2 on MR/MW, P1 on MC) HAS THREE BANK DIGITS,
//     NOT ONE OR TWO: "0 ~ 2: Memory bank number" (ts2000:10592), against the
//     590 pair's 0-1 and the 480's fixed zero. Channel numbers therefore run
//     000-299, not 000-119 or 00-99.
//   - THE BANK BYTE'S SPACE VALUE MEANS "NO BANK NUMBER" ON THIS ROW, and that
//     is NOT the 590 pair's "0 or a space for a channel below 100" rule — see
//     entry 4 below.
//   - P11 (REVERSE) HAS NO PRINTED LEGEND AT ALL, anywhere in this document —
//     see entry 5.
//   - THE "O;" TOKEN'S CAUSE MATCHES THE TS-480's, NOT THE TS-590's: "Receive
//     data was sent but processing was not completed." (ts2000:9617-9618),
//     word for word the TS-480's sentence (480:143-144) and NOT the TS-590's
//     "A receive buffer overrun error occurred" (590:113). No borrowing
//     occurred: both documents were read independently and happen to agree,
//     which is the point of writing each fake from its own book.
//
// # What this fake deliberately does NOT model
//
// SATELLITE MEMORY (SA/SI). Out of this wave by spec §6 open question 1 (the
// matrix's §3): a separate 10-channel record with no frequency field, not a
// row and not a spec.Field this milestone. Not simulated, not even as an
// unsupported stub — the brief's own instruction for this package.
//
// TUNING STEPS AND THE EX/MENU SURFACE. Design addition D8 does not apply to
// any of this wave's seven packages (matrix §4), and no menu chart is cited
// for this row at all — unlike the TS-480/TS-590 pair, this package carries no
// ex.go and no menu inventory.
//
// FV. No "FV" command is cited anywhere in the matrix or the transcribed
// appendix for this row; inventing one would be exactly the fabrication the
// quarantine forbids. TY IS now modelled (Follow-up 2, below) — parser.go's
// handleTY — because core/driver/ts2000's own Open sends it unconditionally
// after ID, per reviews/registration.md's finding; it was omitted from the
// first pass only because the matrix itself does not cite it.
//
// FAULT INJECTION beyond the book. No dropped-reply, garbled-byte,
// spurious-frame or chunked-write option here, for the same reason
// internal/fakets480's doc.go gives: those exercise core/transport.Engine, one
// model-independent implementation already covered elsewhere.
//
// A FRONT PANEL. The selected channel moves only by an MC Set.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something the manual and the matrix do
// not settle is listed here, and each entry appears as an inline comment
// beside the code that implements it, cited by number ("doc.go's register
// entry N") since this package carries no mechanised completeness test for
// it — a deliberate omission, not an oversight: nothing in the brief asks
// for one, and the entries below are few enough to keep in sync by eye.
//
//  1. AN ACCEPTED SET PRODUCES NO REPLY. Neither MW's chart nor MC's Set row
//     prints an acknowledgement (ts2000:10761-10774, 10589-10601), and the
//     error table lists only failures (ts2000:9600-9618), so this fake
//     answers nothing at all to an accepted Set.
//
//  2. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go is ASSUMED to be what the radio itself enforces: the manual
//     prints the legends, never what a radio does with a Set that leaves one.
//     Nothing here normalises: a byte outside its printed legend is refused,
//     never quietly corrected.
//
//  3. AN UNWRITTEN CHANNEL'S ANSWER IS CANNOT-ESTABLISH, AND THIS FAKE STILL
//     ANSWERS ONE. The matrix is explicit that no sentence equivalent to the
//     590 pair's "P4~P15 will be 0, P16 will be blank" exists anywhere in this
//     document for an empty channel (matrix §5, §6 item 5) — a WEAKER
//     position than the 480's unlifted A4, which at least has a sibling
//     sentence to read across. This fake answers the all-zero/all-blank shape
//     anyway, purely so the protocol has SOMETHING to say to an MR of a slot
//     nobody has written, and flags it here as invented rather than assumed.
//     Every byte of it is overridable per channel via WithChannel/
//     WithFactoryImage, which is the "option where the manual leaves it open"
//     the brief asks for.
//
//  4. THE BANK BYTE'S SPACE VALUE MEANS "NO BANK NUMBER", per MC's own P1
//     legend: "_ (space): No bank number, 0 ~ 2: Memory bank number"
//     (ts2000:10591-10592). THIS IS NOT THE 590 PAIR'S RULE. That book states
//     a DIRECTIONAL convention explicitly — enter 0 or a space for a channel
//     under 100, but always ANSWER with a space there (590:1334-1337) — and
//     ts2000's own chart states no such asymmetry: the legend is one list, not
//     a Set-versus-Answer pair. This fake therefore accepts a space on the SET
//     direction (folded to bank 0, "no bank number") and always ANSWERS a
//     concrete digit ('0'/'1'/'2'), because every stored record has a
//     concrete address and nothing in this row's own text asks for a space on
//     the way out.
//
//  5. REVERSE (P11) IS STORED AS A SINGLE ASCII DIGIT, NOT RANGE-CHECKED,
//     BECAUSE NO LEGEND FOR IT EXISTS ANYWHERE IN THIS DOCUMENT. MR's own
//     table prints only "REVERSE status." (ts2000:10712) with no value list at
//     all — not even a width note beyond the one-byte position the ruler
//     gives it. Refusing anything but a claimed pair of values would assert a
//     legend this book never prints; admitting any digit is the least this
//     fake can assume.
//
//  6. DCS CODE (P10) AND OFFSET FREQUENCY (P13) ARE STORED, NOT
//     RANGE-CHECKED. Both are cited to another command ("See QC command",
//     ts2000:10709-10710; "See OS command", ts2000:10716-10717) whose charts
//     this milestone's matrix does not transcribe (matrix §6 item 4); only the
//     WIDTH (3 and 9 ASCII digits) is enforced.
//
//  7. TONE NUMBER (P8) AND CTCSS TONE NUMBER (P9) ARE STORED, NOT
//     RANGE-CHECKED, matching every sibling Kenwood fake's treatment of an
//     index into a chart this milestone pins by position but not by value
//     (the 39-entry CTCSS chart is transcribed in the matrix, §4, but nothing
//     in either P-field's own citation says what a Set outside it does).
//
//  8. THE STEP INDEX (P14) IS STORED, NOT RANGE-CHECKED. "Step size. See ST
//     command." (ts2000:10720-10721) cites a command this milestone's matrix
//     does not transcribe either; only the two-digit width is enforced.
//
//  9. A SET CARRYING MODE NIBBLE 0 (undocumented) IS STORED, not refused. The
//     MD legend runs 1-9 with nibble 8 a printed hole ("8: Reserved",
//     matrix §5); nibble 0 is simply absent from the legend, and nothing says
//     what a Set carrying it does. This fake stores every digit nibble rather
//     than inventing a refusal, matching the sibling Kenwood fakes'
//     "store an unused nibble" convention.
//
//  10. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in the MW block
//     mentions the selection (ts2000:10761-10774).
//
//  11. THE SELECTED CHANNEL AT CONSTRUCTION IS THE LOWEST CHANNEL, 000. No
//     power-on value for MC is printed anywhere; this fake takes the lowest
//     number in its slot space, the same reading every sibling Kenwood fake
//     gives the same silence.
//
//  12. THE INITIAL AI VALUE IS OFF. No power-on value for AI is printed here
//     either (unlike the TS-590's explicit "the initial state is OFF",
//     590:81-82); this fake takes AI OFF as the construction default anyway,
//     the same value every sibling Kenwood fake starts at, because a
//     transport engine's session-opening AI-off Set is idempotent against it
//     either way and no other value has any citation behind it.
//
//  13. AUTOMATIC-INFORMATION SUPPRESSION. This fake never pushes anything
//     unsolicited, whatever AI is set to. "When the extended AI format is
//     selected, the transceiver automatically sends the parameters."
//     (ts2000:9670-9672) is not modelled; no TS-2000 has been observed by this
//     project, and modelling silence is the honest default.
//
//  14. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound is this
//     package's own bounded-input policy, not a manual figure, matching every
//     sibling Kenwood fake's choice.
//
//  15. THE DEFAULT IMAGE'S RECORD COMPOSITION. Every byte of every shipped
//     record is a printed constant, the manual's own worked frequency example
//     ("FA00007000000;", ts2000:9567/9595/9600/9607, read as an 11-digit BCD
//     value), a printed legend value, or the eight-space name — but no MR, MW
//     or MC frame is printed as a literal anywhere in this document, so the
//     CROSS-FIELD COMBINATION has never been printed or observed. PROVENANCE.md
//     carries this posture. No byte is invented, derived from another model, or
//     padded to make a test pass.
//
//  16. WHY THE THREE ROWS SHARE ONE WIRE IDENTITY. The matrix (§4) finds no
//     "2000X" or "B2000" qualifier anywhere beside MR/MW/MC/ID in the command
//     tables; ID answers "019" (ts2000:10429-10441) for all three, ASSUMED for
//     the X and B2000 rows since only "TS-2000" is printed against that value.
//     WithModelName therefore changes only what Model() reports for a test's
//     own bookkeeping — it produces NO wire difference, because the manual
//     records none. WITH NO OPTION, THE MODEL IS "TS-2000": it is the one row
//     with a MANUAL-EVIDENCED (not ASSUMED) CATID, and the document's own
//     title and front matter name it first of the three
//     (ts2000:100, PDF p.3) — a reasonable single default for a caller that
//     names no model, not an arbitrary pick.
//
//  17. THE DEFAULT TY ANSWER (Follow-up 2, added once reviews/registration.md
//     found core/driver/ts2000's Open sends "TY;" unconditionally after
//     "ID;" and this fake had no case for it). TY's OWN Set chart prints no
//     content (ts2000:11678-11693, ruler only under "Se t") — the TS-480's
//     own erratum E10 shape — so this fake refuses a TY Set and only
//     answers a Read. P1 (two bytes, "Reserved", no legend at all,
//     ts2000:11680) ships as "00", an invented placeholder with no hard-wired
//     convention to borrow (this row has no printed-fixed byte anywhere,
//     matrix §2). P2 ships as "0", the FIRST of the three printed variants,
//     "0: Overseas type" (ts2000:11683-11685) — not a claim that any
//     TS-2000 is the overseas type. Both bytes are the SAME across all
//     three rows — entry 16's reasoning again, and the coordinator's own
//     instruction: the manual gives no separate TY answer for TS-2000X or
//     TS-B2000, so the TS-2000 one is answered for all three, ASSUMED.
//
//  18. SA SET WRITES ITS OWN CHANNEL DIRECTLY (v1.10.0, the Satellite
//     Memory bank). The manual prints ONE SA Set form carrying P2 (the
//     channel, 0-9) alongside P1 and P3-P7 in the same frame
//     (ts2000:11296-11298), with no separate CAT-reachable "recall
//     channel N" command and no CAT-reachable equivalent of the front
//     panel's M.IN commit step (ts2000:5364-5379 describes M.IN, a
//     physical button, not a command this book prints). This fake reads
//     that the way it already reads MW — entry 1's own "a Set does not
//     move the selection" is the ONE exception, since SA's own P2 DOES
//     move it, being the channel address itself — an SA Set WRITES the
//     addressed channel's P3/P5/P6 (and the whole-radio P1/P4/P7)
//     DIRECTLY, with no recall step to model. ASSUMED, exactly like every
//     other MR/MW/MC frame this fake answers: no TS-2000/2000X/B2000 has
//     ever been asked anything (entry 1's own opening line).
package fakets2000
