// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft450d is an independent, stdlib-only FT-450D CAT simulator:
// the test double the transport engine, core/driver/ft450d, the CLI's
// --fake mode and the GUI's demo mode run against, the role
// internal/fakeradio plays for the FT-710 and internal/fakeft950 plays for
// the FT-950.
//
// ONE ROW, BARE New. The FT-450D is its own document (CAT book 1710-B), the
// same "own document, bare New" shape as fakeft950 and fakeft2000's FT-2000
// row — CATID "0244" (layout:605, "P1 0244 (Fixed value)"), the ID answer's
// own printed constant.
//
// THIS RADIO IS NoTag: no `MT` command anywhere in the 90-command index
// (layout:127-196), and no other command's legend carries a name/label byte
// — the fact core/spec.Capabilities.NoTag exists to state (merged
// d0b2498). So there is no Tag field in MemState, no charset, and no echo.
//
// The record is WRITE/READ SEPARATE: MW (MEMORY WRITE, "O X X X",
// layout:131) is Set-only, MR (MEMORY READ, "X O O X", layout:171) is
// Read/Answer-only, and MC (MEMORY CHANNEL, "O O O X", layout:166) selects
// the current channel — the same three-command shape fakeft950 and
// fakeft2000 both play, re-derived here independently against this radio's
// own manual (docs/superpowers/ft450d-capability-matrix.md), not read
// across from either sibling's code.
//
// THE RECORD ITSELF IS BYTE-FOR-BYTE THE SAME SHAPE AS fakeft950's: 27-byte
// MW Set / MR Answer frame, 8-digit P2 frequency, a live 2-digit P9 CTCSS
// tone-table index, an eleven-member P6 mode legend ('1'-'9' then 'B','C',
// a CLEAN HOLE at 'A' and no 'D'/'E'/'F' member at all — unlike fakeft950's
// own closed twelve-member legend, matrix §1.2), P7 write-fixed to Set's
// own "0: Fixed" and read/answered from the ordinary "0: VFO 1: Memory"
// pair, and a three-valued P8 CTCSS byte with no DCS member.
//
// THIS PACKAGE'S OWN DELTA, THE ONE THE WHOLE WAVE WAS BUILT TO EXERCISE:
// the PMS bank (channels 501-504, two pairs) answers MR but REFUSES every
// MW Set with "?;" — the roadmap's own 13/09/2026 SAFE SHAPE ruling
// (matrix §3/§4), not a manual-stated restriction: the CAT book's own MW
// legend prints no ceiling narrower than 504, so a real FT-450D may accept
// a PMS write today; this fake plays the CAUTIOUS project posture, not the
// manual's silence, and register entry 6 below says so at the point that
// matters, not merely in this file.
//
// # The hard rule: NOTHING project-internal
//
// fakeft450d MUST NOT import any package of this project — not core/cat,
// not core/cat/ft450d, not core/driver/ft450d, not core/codeplug, not
// core/spec, and not internal/fakeft950 or any sibling fake. Standard
// library only, in every non-test file, in this directory and every
// directory beneath it (imports_test.go). This package was built without
// reading core/driver/ft450d's caps.go, its tests, or reviews/driver-
// ft450d.md's detail section — only the matrix, the manual layout, and the
// driver review's own `## Verdict` (a fact about the wire, never an
// implementation) — precisely so that a systematic error in that driver's
// own reading of the manual cannot sit on both sides of the cross-check
// `go test ./core/driver/ft450d/...` runs against a fake shaped like this
// one. Every field below is re-derived from the Yaesu FT-450D CAT
// Operation Reference Book (internal doc code 1710-B,
// docs/fixtures-private/manuals/ft450d_cat_1710-B... layout extraction,
// gitignored) and from docs/superpowers/ft450d-capability-matrix.md, cited
// "layout:NNN" throughout — a citation names where the chart is, not a
// link, since the manual itself is never committed.
//
// # A SIBLING of fakeft950, not a refactor of it
//
// This package copies a good deal of that sibling's SHAPE: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention,
// the per-command handlers, the Option pattern, the recursive import
// fence. That duplication is deliberate, for the same reasons fakeft950's
// own doc.go gives: sharing it would need a project-internal helper
// package, which the hard rule forbids, and two radios agreeing on a shape
// is a fact about those two radios, not a definition worth factoring out.
// Where this radio's OWN wire disagrees with that sibling's, it is this
// manual (or the roadmap ruling), not a preference:
//
//   - MEMORYLO IS 1, NOT 0. MC's own legend: "001 - 500: Regular Memory
//     Channel" (layout:714-715) — the answer-only "no selection yet" form
//     is therefore "000", sitting BELOW the span the way fakeft991a's and
//     fakeft2000's own none-forms do, not above it the way fakeft950's
//     "118" must (that radio's span starts at 000 and leaves no room
//     below).
//   - THE MODE LEGEND HAS A HOLE AT 'A' AND STOPS AT 'C'. Eleven members,
//     not fakeft950's closed twelve (matrix §1.2, dialect.go's own
//     modeNames comment).
//   - PMS (501-504) IS READ-ONLY. fakeft950 and fakeft2000 both treat
//     every slot their MC span covers as read/write alike; this package's
//     MW refuses any PMS slot outright (register entry 6) — the one
//     genuinely new shape this wave's plan flagged for this package.
package fakeft450d

// writeTrialsComplete records this package's one honest status line: NO
// FT-450D HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE IS
// AVAILABLE TO IT (the capability matrix's own opening sentence). Every
// place below marked ASSUMED is a place this fake had to decide something
// the manual does not settle, and none of them has been lifted by a real
// session.
const writeTrialsComplete = false

// THE ASSUMED REGISTER
//
// Every place this fake had to decide something neither the FT-450D CAT
// Operation Reference Book, the OM, nor
// docs/superpowers/ft450d-capability-matrix.md settles is listed here,
// cited BY NAME at the code that implements it (register_test.go's
// TestASSUMEDRegisterIsComplete holds both halves mechanically), never by
// number.
//
//  1. EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS. A slot this fake holds no state
//     for answers "?;" to an MR read and to an MC-set, indistinguishable
//     from a slot outside the 001-504 span altogether — 505-510 included:
//     the matrix's own SixtyLo/Hi=0 finding (§2.5.1) leaves this fake
//     nothing to model there, so those wire numbers fall out of the same
//     "outside the span" bucket as 999, not a bucket of their own. "?;" is
//     the protocol's one unattributed NAK, the same convention every
//     registered Yaesu fake plays, and this is the honest default for a
//     channel nobody has watched a real radio answer.
//     (parser.go: handleMR, handleMC)
//
//  2. THE "NO SELECTION YET" CHANNEL IS "000". Nothing prints what "MC;"
//     answers before any MC-set has happened. Unlike fakeft950 (whose own
//     span starts at 000, leaving no room below it), this radio's MemoryLo
//     is 1 (layout:714-715), so "000" sits immediately below the whole
//     001-504 span and can never collide with a real channel — the same
//     choice fakeft991a and fakeft2000 make on the identical shape. It is
//     this package's own choice, not a citation: no FT-450D has been asked
//     what it powers on to.
//     (fakeft450d.go: New; parser.go: slotNoneWire)
//
//  3. TONE INDEX (P9) IS STORED AT ITS PRINTED WIDTH, NOT RANGE-CHECKED.
//     Both MR's and MW's own P9 cell print only "Tone Number (See Table 1)"
//     (layout:800, :776), never restating the CN command's own separate
//     "00 - 49" ceiling (matrix §1.3). Whether that ceiling also binds this
//     byte pair INSIDE a stored memory record is unproven either way, so
//     only the field's SHAPE (two ASCII digits) is enforced on a Set;
//     refusing a shape-valid "50"-"99" would assert CN's ceiling as a fact
//     about this other command's parameter, the same reasoning fakeft950's
//     and fakeft2000's own registers state for their own P9 pair.
//     (parser.go: validToneDigits)
//
//  4. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go — the slot form, the 8-digit frequency, the clarifier sign
//     and 4-digit magnitude, the two clarifier flags, the eleven-member
//     mode nibble, MW's fixed Kind byte, the 3-state CTCSS byte, the tone
//     digits' shape, the 3-state shift byte — is ASSUMED to be what the
//     radio itself enforces on a Set. The manual prints the legends; it
//     does not say what the radio does with a Set that leaves one. Nothing
//     here NORMALISES: a byte outside its legend is refused, never quietly
//     corrected.
//     (parser.go: the validators, parseMemoryBlock)
//
//  5. AN ANSWER'S KIND BYTE IS ALWAYS '1' (Memory), FOR EVERY SLOT THIS
//     FAKE HOLDS, MEMORY OR PMS ALIKE. MR's read legend has exactly two
//     members, "0: VFO 1: Memory" (layout:796), and nothing states which
//     one a PMS band-edge answers with; '1' is the only member that is not
//     plainly false for a slot this fake actually stores. Mirrors
//     fakeft950's own register entry for the identical question.
//     (parser.go: kindMemory)
//
//  6. PMS SLOTS (501-504) REFUSE EVERY MW SET WITH "?;", EVEN A
//     WELL-FORMED ONE. This is the SAFE SHAPE ruling
//     (docs/superpowers/ft450d-capability-matrix.md §3/§4, the roadmap's
//     13/09/2026 paragraph beginning "FT-450D registered in the SAFE
//     SHAPE"), NOT a manual-stated restriction: the CAT book's own MW
//     legend prints 501-504 as ordinary P1 values with no asterisk or
//     footnote excluding them (layout:713-719). The ruling's own caution —
//     "PMS 501-504 READ-ONLY until an owner probe shows MW 501 succeeding"
//     — is what this fake plays; a real radio may accept the write today,
//     and that gap is the whole reason this package exists independently
//     of the driver rather than copying its caps.go.
//     (parser.go: handleMW)
//
//  7. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in MW's block
//     mentions the current-channel selection (layout:789-805); only an
//     MC-set changes what "MC;" answers.
//     (parser.go: handleMW)
//
//  8. AN MW SET CREATES AN ABSENT MEMORY CHANNEL AS WELL AS OVERWRITING A
//     PRESENT ONE. "MEMORY WRITE" names no precondition, and the writable
//     001-500 span is drawn once, not doubled for "already populated" and
//     "not yet populated" (layout:713-715). A write command that refused
//     to create a channel it had no record for would need a second,
//     unprinted rule to do so. PMS slots never reach this path at all
//     (register entry 6).
//     (parser.go: handleMW)
//
//  9. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than
//     maxAccumulatorBytes have accumulated without a ';', this fake
//     replies "?;" once and discards bytes up to and including the next
//     ';' before resuming normal framing. NOT a radio claim: the manual
//     prints no buffer size, and this is this package's own bounded-input
//     policy, inherited in shape from fakeft950's identical one.
//     (parser.go: reassembler)
//
//  10. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Every byte of every
//     shipped record is a printed constant or a printed legend value, but
//     no MW, MR or MC frame is printed as a literal anywhere in the
//     manual, so the CROSS-FIELD COMBINATION has never been printed or
//     observed. The records are not factory defaults and no byte is
//     derived from another model or padded to make a test pass.
//     (image.go: DefaultImage)
//
//  11. AI-SET IS FIRE-AND-FORGET, AND THIS FAKE NEVER PUSHES AN
//     UNSOLICITED AI BROADCAST. No FT-450D has had its AI-flood behaviour
//     observed by this project; core/transport.Engine.Init opens every
//     session with an AI-off Set regardless, so this fake's own choice —
//     silence, always — is on the critical path of every fake session
//     without asserting a broadcast fact either way, the same posture
//     every registered Yaesu fake in this fleet takes.
//     (parser.go: handleAI)
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AND SILENCE ON AN ACCEPTED SET. The manual
// prints no acknowledgement anywhere in its command index, and every
// registered Yaesu fake in this fleet already plays exactly this
// convention — inherited in shape, not re-assumed per package.
//
// COMMAND NAMES ARE ACCEPTED IN EITHER CASE, the same MANUAL FACT every
// Yaesu CAT book in this fleet states for its own command index.
//
// THE MODE LEGEND'S HOLE AT 'A' AND CEILING AT 'C'. What MD/MR/MW/IF's own
// legends print (matrix §1.2), not an assumption.
//
// THE SLOT SPAN IS 001-504, AND THE PMS PAIRS ARE DIRECT DECIMAL NUMBERS.
// MC's legend prints both the span and its decomposition in one place
// (layout:713-719); nothing here re-derives it.
