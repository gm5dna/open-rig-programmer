// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeftdx3000 is an independent, stdlib-only FTdx3000 CAT
// simulator: the test double the transport engine, core/driver/ftdx3000,
// the CLI's --fake mode and the GUI's demo mode run against, the role
// internal/fakeradio plays for the FT-710 and internal/fakeft2000 for the
// FT-2000/FT-2000D.
//
// ONE ROW, BARE New. Unlike internal/fakeft2000's two-row WithModelName
// shape, the FTdx3000 CAT manual states exactly one ID answer, "P1 0462:
// FTdx3000" (layout:759, `ID IDENTIFICATION`) — there is no second row to
// select, so New takes no model option at all and always answers CATID
// "0462". This matches the driver verdict's own finding
// (reviews/driver-ftdx3000.md, "one row, bare New, CATID 0462").
//
// THIS RADIO IS NoTag: a whole-document grep of the layout extraction for
// `tag`/`name`/`label` returns one irrelevant hit (a connector-pin legend)
// and nothing else — no channel-name route exists anywhere in the command
// set (matrix §0). So there is no Tag field in MemState, no charset, and no
// echo.
//
// The record is WRITE/READ SEPARATE, not combined: MW (MEMORY CHANNEL
// WRITE, layout:944-957) is Set-only, and MR (MEMORY CHANNEL READ,
// layout:917-931) is Read/Answer-only. MC (MEMORY CHANNEL, layout:875-883)
// selects the current channel.
//
// # The hard rule: NOTHING project-internal
//
// fakeftdx3000 MUST NOT import any package of this project — not core/cat,
// not core/cat/ftdx3000, not core/codeplug, not core/spec, and not
// internal/fakeft2000 or any sibling fake — in any non-test file, in this
// directory and every directory beneath it (imports_test.go). THE
// QUARANTINE GOES FURTHER for this package: it was built without reading
// core/cat/ftdx3000 or core/driver/ftdx3000 at all — not even for a byte
// offset or an enum spelling — precisely so that a systematic error in
// that driver's own reading of the manual cannot sit on both sides of the
// cross-check `go test ./core/driver/ftdx3000/...` runs against a fake
// shaped like this one. Every field below is re-derived from the FTdx3000
// CAT OPERATION MANUAL (revision 2006-D,
// docs/fixtures-private/manuals/ftdx3000_layout.txt, gitignored) and from
// docs/superpowers/ftdx3000-capability-matrix.md, cited "layout:NNN"
// throughout — a citation names where the chart is, not a link, since the
// manual itself is never committed.
//
// # A SIBLING of internal/fakeft2000, not a refactor of it
//
// This package copies a good deal of that sibling's SHAPE: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention,
// the per-command handlers, the Option pattern, the recursive import
// fence, the 24-byte shared memory field block (this radio's own frame is
// 27 bytes total, byte-for-byte the same block layout as the FT-2000's:
// matrix §1.1 notes every offset beyond P2 is explained purely by P2's
// shared 8-digit width). Where this radio's OWN wire disagrees with that
// sibling's, it is this manual's legend, not a preference:
//
//   - P9 IS ASYMMETRIC: LIVE ON READ, FIXED "00" ON WRITE — the mirror
//     image of the FT-2000's own novelty (which is live on BOTH
//     directions). MR's own block prints a live two-digit tone-table index
//     (layout:930-931, "P10: Tone Number (See Table 1)"); MW's own block
//     prints "P9: 0: (Fixed)" (layout:955-957). This is the wave's headline
//     wrinkle for this package (matrix §1.3) and the reason
//     core/cat.MemoryP9Policy grew a third value on the driver side
//     (reviews/driver-ftdx3000.md Verdict) — this fake models the same
//     asymmetry from the manual directly, independently of that policy's
//     name or shape.
//   - MC'S OWN LEGEND HAS A "000" HOLE MW/MR DO NOT SHARE. MC's Set prints
//     "P1 000 - 117: Memory Channel Number / 000 - 099: Regular Memory
//     Channel" (layout:876-878) — channel 000 included — where MW's and
//     MR's own P1 cells both print "(001 117)", excluding it
//     (layout:918, :945). The capability matrix records this as an open
//     erratum and follows MR/MW's narrower span (`MemoryLo: 1`, matrix
//     §2.4). This fake follows the same ruling: 000 is refused as a Set
//     target on every command, including MC, and survives only as this
//     package's own answer-only "no selection yet" sentinel (register
//     entry below) — the ft2000-shape trick, applied to a genuinely
//     different manual gap.
package fakeftdx3000

// writeTrialsComplete records this package's one honest status line: NO
// FTdx3000 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE IS
// AVAILABLE TO IT (the capability matrix's own opening sentence). Every
// place below marked ASSUMED is a place this fake had to decide something
// neither the manual nor the matrix settles, and none of them has been
// lifted by a real session.
const writeTrialsComplete = false

// THE ASSUMED REGISTER
//
// Every place this fake had to decide something neither the FTdx3000 CAT
// OPERATION MANUAL nor docs/superpowers/ftdx3000-capability-matrix.md
// settles is listed here, cited BY NAME at the code that implements it
// (register_test.go's TestASSUMEDRegisterIsComplete holds both halves
// mechanically), never by number.
//
//  1. EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS. A slot this fake holds no state
//     for answers "?;" to an MR read and to an MC-set, indistinguishable
//     from a slot outside the valid span altogether. Neither the manual
//     nor the matrix states what an MR read or an MC-set of an
//     unprogrammed channel returns; "?;" is the protocol's one
//     unattributed NAK (layout:98-100's case-and-terminator convention,
//     the same one every registered Yaesu fake plays), and this is the
//     honest default for a channel nobody has watched a radio answer.
//     (parser.go: handleMR, handleMC)
//
//  2. THE "NO SELECTION YET" CHANNEL IS "000", AND MC's OWN "000" HOLE IS
//     NOT A VALID SET TARGET. Neither book states what "MC;" answers
//     before any MC-set has happened. "000" is chosen because it can never
//     collide with a real channel under this fake's own registered span
//     (matrix §2.4's `MemoryLo: 1` ruling, doc.go) — even though MC's own
//     legend prints 000 as a nameable "Regular Memory Channel" (the
//     MC-vs-MW/MR erratum, doc.go), this fake refuses 000 as a Set target
//     on every command, so the sentinel can never be confused with a
//     channel a caller actually selected.
//     (fakeftdx3000.go: New; parser.go: slotNoneWire)
//
//  3. TONE INDEX (P9): WRITE MUST BE LITERAL "00"; A STORED SLOT'S TONE
//     BECOMES LIVE ONLY THROUGH WithSlot/WithFactoryImage. MW's own block
//     prints "P9: 0: (Fixed)" (layout:955-957) — this fake refuses any Set
//     whose P9 bytes are not exactly "00", never silently coercing them.
//     MR's own block prints a live two-digit index (layout:930-931), so a
//     read answers whatever this fake currently holds for that slot —
//     which can only be nonzero if an Option placed it there directly (a
//     channel this fake treats as having been tone-programmed from the
//     front panel, not over CAT), never as the result of an accepted Set.
//     (parser.go: validP9Write, parseMemoryBlock, handleMW)
//
//  4. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go — the slot form, the 8-digit frequency, the clarifier sign
//     and 4-digit magnitude, the two clarifier flags, the 12-member mode
//     nibble, the 3-state CTCSS byte, the fixed P9 bytes, the 3-state
//     shift byte — is ASSUMED to be what the radio itself enforces on a
//     Set. The manual prints the legends; it never says what the radio
//     does with a Set that leaves one. Nothing here NORMALISES: a byte
//     outside its legend is refused, never quietly corrected.
//     (parser.go: the validators, parseMemoryBlock)
//
//  5. AN ANSWER'S KIND BYTE IS ALWAYS '1' (Memory), FOR EVERY SLOT THIS
//     FAKE HOLDS, MEMORY OR PMS ALIKE. MR's read legend has exactly two
//     members, "0: VFO 1: Memory" (layout:929), and neither book states
//     which one a PMS band-edge answers with; '1' is the only member that
//     is not plainly false for a slot this fake actually stores. Mirrors
//     internal/fakeft2000's own register entry for the identical question
//     on that radio's P8 — cited as a parallel, not re-derived, since the
//     two manuals never agree with each other about anything else on this
//     byte.
//     (parser.go: kindMemory)
//
//  6. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in MW's block
//     mentions the current-channel selection (layout:944-957); only an
//     MC-set changes what "MC;" answers.
//     (parser.go: handleMW)
//
//  7. AN MW SET CREATES AN ABSENT CHANNEL AS WELL AS OVERWRITING A PRESENT
//     ONE. "MEMORY CHANNEL WRITE" names no precondition, and no separate
//     legend distinguishes "already populated" from "not yet populated"
//     (layout:944-957). A write command that refused to create a channel
//     it had no record for would need a second, unprinted rule to do so.
//     (parser.go: handleMW)
//
//  8. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than
//     maxAccumulatorBytes have accumulated without a ';', this fake
//     replies "?;" once and discards bytes up to and including the next
//     ';' before resuming normal framing. NOT a radio claim: the manual
//     prints no buffer size, and this is this package's own bounded-input
//     policy, inherited in shape from internal/fakeft2000's identical one.
//     (parser.go: reassembler)
//
//  9. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Every byte of every shipped
//     record is a printed constant or a printed legend value, but no MW,
//     MR or MC frame is printed as a literal anywhere in the manual, so
//     the CROSS-FIELD COMBINATION has never been printed or observed —
//     including the one slot this image gives a nonzero tone index, which
//     exercises the read/write P9 asymmetry (register entry 3) and is not
//     a factory default of any kind.
//     (image.go: DefaultImage)
//
//  10. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
//     unsolicited, whatever AI is set to. The manual documents AI only in
//     its command index and its own four-cell block (layout:210-217),
//     never in a worked example, and no FTdx3000 has been observed by
//     this project. Modelling silence is the honest default, not a claim
//     that the radio is silent.
//     (parser.go: handleAI; fakeftdx3000.go: New, handleEvent)
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AND SILENCE ON AN ACCEPTED SET. The manual
// prints no acknowledgement anywhere in its command index or in MW's own
// chart (layout:944-957), and every registered Yaesu fake in this fleet
// already plays exactly this convention — inherited in shape, not
// re-assumed per package.
//
// COMMAND NAMES ARE ACCEPTED IN EITHER CASE. "A command consists of 2
// alphabetical characters. You may use either lower or upper case
// characters." (layout:98-99) — a MANUAL FACT, printed for this radio by
// name, not inherited from a sibling.
//
// THE MODE LEGEND HAS NO 'D' MEMBER ON MW/MR. Both are what MW's and MR's
// own legends print (layout:922-925, :950-952) — AM-N ('D') exists only on
// the separate MD (OPERATING MODE) command's own legend (layout:891), one
// command away and not part of the memory record — matrix §1.2, cited
// there, not re-derived here.
//
// THE SLOT SPAN IS 001-099 (MEMORY) / 100-117 (PMS), NOT MC's PRINTED
// 000-117. Matrix §2.4's own erratum note and register entry 2 above cover
// this; nothing here re-derives it.
