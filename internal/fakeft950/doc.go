// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft950 is an independent, stdlib-only FT-950 CAT simulator: the
// test double the transport engine, core/driver/ft950, the CLI's --fake mode
// and the GUI's demo mode run against, the role internal/fakeradio plays for
// the FT-710 and internal/fakeft2000 plays for the FT-2000/FT-2000D.
//
// ONE ROW, BARE New. The FT-950 (EC030H120) is its own document, unlike the
// FT-2000 SERIES manual that names two rows — spec.md §1: "FTdx5000 (1907-D)
// and FT-950 (EC030H120) are their own documents → own packages, bare New."
// THIS IS A DELIBERATE DEVIATION from internal/fakets590's REQUIRED Row
// argument (this milestone's plan, and the brief that dispatched this
// package): New() takes no model option at all, because there is only ever
// one row to resolve to — "FT-950", CATID "0310" (layout:676, "P1 0310
// (Fixed value)"), the ID answer's own printed constant.
//
// THIS RADIO IS NoTag: no CAT command in the full 90-command index
// (layout:129-181) reads or writes a channel name, and there is no name/tag
// route of any kind in the manual — the fact core/spec.Capabilities.NoTag
// exists to state (merged d0b2498). So there is no Tag field in MemState, no
// charset, and no echo: building one for a radio whose matrix found none
// would be inventing wire behaviour, not modelling it.
//
// The record is WRITE/READ SEPARATE: MW (MEMORY CHANNEL WRITE, availability
// "O X X X", layout:130) is Set-only, and MR (MEMORY CHANNEL READ, "X O O X",
// layout:177) is Read/Answer-only — this radio has no MT command anywhere in
// its command index, so there is nothing for a combined form to gain. MC
// (MEMORY CHANNEL, "O O O X", layout:172) selects the current channel over
// the same 000-117 span MW/MR/MC all draw on: 000-099 plain memory channels
// (this radio's own delta — MemoryLo is 000, not every sibling's 001, MC's
// own legend "000 - 099: Regular Memory Channel", layout:795-802), 100-117
// nine PMS pairs as DIRECT DECIMAL CHANNEL NUMBERS — the same wire numbering
// internal/fakeft2000's PMS slots already use, so this package's slot grammar
// is copied in SHAPE from that sibling, not re-invented.
//
// THE RECORD ITSELF IS BYTE-FOR-BYTE THE SAME SHAPE AS internal/fakeft2000's:
// 27-byte MW Set / MR Answer frame, 8-digit P2 frequency (one digit narrower
// than every OTHER registered Yaesu dialect's 9), a live 2-digit P9 CTCSS
// tone-table index where every registered dialect prints and enforces a fixed
// "00", the identical twelve-member P6 mode legend (`1`-`9`,`A`-`C`, no `0`
// placeholder, no hole), P7 write-fixed to Set's own "0: (Fixed)" and
// read/answered from the ordinary "0: VFO 1: Memory" pair, and a three-valued
// P8 CTCSS byte with no DCS member. All of this is spec.md §1's own "HIGH
// proximity, one new dialect variant" finding for the whole
// ft2000/ftdx5000/ftdx9000/ft950 quartet, independently re-derived here
// against this radio's own manual (docs/superpowers/ft950-capability-matrix.md,
// EC030H120), not read across from fakeft2000's code.
//
// # The hard rule: NOTHING project-internal
//
// fakeft950 MUST NOT import any package of this project — not core/cat, not
// core/cat/ft950, not core/codeplug, not core/spec, and not
// internal/fakeft2000 or any sibling fake. Standard library only, in every
// non-test file, in this directory and every directory beneath it
// (imports_test.go). This package was built without reading core/kw/ft950 or
// core/driver/ft950 at all — not even for a byte offset or an enum spelling —
// precisely so that a systematic error in that driver's own reading of the
// manual (an off-by-one offset, a validation rule subtly wrong) cannot sit on
// both sides of the cross-check `go test ./core/driver/ft950/...` runs
// against a fake shaped like this one. Every field below is re-derived from
// the Yaesu FT-950 CAT Operation Reference Book (internal doc code
// EC030H120, docs/fixtures-private/manuals/ft950_cat_ec030h120_layout.txt,
// gitignored) and from docs/superpowers/ft950-capability-matrix.md, cited
// "layout:NNN" throughout — a citation names where the chart is, not a link,
// since the manual itself is never committed.
//
// # A SIBLING of internal/fakeft2000, not a refactor of it
//
// This package copies a good deal of that sibling's SHAPE: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Option pattern, the recursive import fence. That
// duplication is deliberate for the same two reasons fakeft2000's own doc.go
// gives: sharing it would need a project-internal helper package, which the
// hard rule forbids, and two radios agreeing on a shape is a fact about those
// two radios, not a definition worth factoring out. Where this radio's OWN
// wire disagrees with that sibling's, it is this manual's legend, not a
// preference:
//
//   - ONE ROW, BARE New — no WithModelName, no per-row CATID map (above).
//   - MEMORYLO IS 000, NOT 001. MC's own legend's first line reads "000 -
//     099: Regular Memory Channel" (layout:795-802) — one more regular
//     channel than the FT-2000/FTdx5000/FTdx9000 trio document (spec.md §0's
//     own note). The answer-only "no selection yet" channel therefore CANNOT
//     be "000" the way fakeft2000's is (there it is the lowest three-digit
//     numeral outside 001-117; here 000 is a real, addressable channel) — see
//     THE ASSUMED REGISTER entry 2 below for this package's own choice.
//   - MD'S OWN P2 HAS A THIRTEENTH VALUE ("D: AM-N", layout:805-812) THAT THE
//     MEMORY-RECORD P6 LEGEND (MR/MW, layout:844-862/876-899) DOES NOT PRINT.
//     Recorded as an erratum, not resolved (matrix §1.7, same trap class as
//     FTdx101 layout:931): this fake's Mode validator enforces the
//     MEMORY-RECORD legend's twelve values only, never widened to `D` from
//     MD's own P2 — MD is live VFO-mode state, not a memory field, and this
//     package holds no MD state at all (it is not part of the 90-command
//     surface this radio's memory-channel commands need).
package fakeft950

// writeTrialsComplete records this package's one honest status line: NO
// FT-950 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE IS AVAILABLE
// TO IT (the capability matrix's own opening sentence). Every place below
// marked ASSUMED is a place this fake had to decide something the manual
// does not settle, and none of them has been lifted by a real session.
const writeTrialsComplete = false

// THE ASSUMED REGISTER
//
// Every place this fake had to decide something neither the FT-950 CAT
// Operation Reference Book nor docs/superpowers/ft950-capability-matrix.md
// settles is listed here, cited BY NAME at the code that implements it
// (register_test.go's TestASSUMEDRegisterIsComplete holds both halves
// mechanically), never by number.
//
//  1. EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS. A slot this fake holds no state
//     for answers "?;" to an MR read and to an MC-set, indistinguishable from
//     a slot outside 000-117 altogether. Nothing in the manual states what an
//     MR read or an MC-set of an unprogrammed channel returns; "?;" is the
//     protocol's one unattributed NAK (layout:106-109's command-framing
//     text, the same convention every registered Yaesu fake plays), and this
//     is the honest default for a channel nobody has watched a real radio
//     answer.
//     (parser.go: handleMR, handleMC)
//
//  2. THE "NO SELECTION YET" CHANNEL IS "118". Nothing prints what "MC;"
//     answers before any MC-set has happened, and unlike fakeft2000 (where
//     001-117 leaves "000" free below the span) this radio's own span is
//     000-117 with no room below it (layout:795-802). "118" is the lowest
//     three-digit numeral immediately ABOVE the span, so it can never
//     collide with a real channel and an MC read against it is unambiguous.
//     It is this package's own choice, not a citation: no FT-950 has been
//     asked what it powers on to.
//     (fakeft950.go: New; parser.go: slotNoneWire)
//
//  3. TONE INDEX (P9) IS STORED AT ITS PRINTED WIDTH, NOT RANGE-CHECKED. CN's
//     own legend prints "P2 00 - 49: Tone Frequency Number" (layout:322-323),
//     but neither MW's nor MR's own P9 cell restates that ceiling — both
//     print only "Tone Number (See Table 1)" / "(See Table on page 5)"
//     (layout:703/857 MR, layout:899 MW). Whether the 00-49 ceiling that
//     binds CN's live tone-select parameter also binds this byte pair INSIDE
//     a stored memory record is unproven either way, so only the field's
//     SHAPE (two ASCII digits) is enforced on a Set; refusing a shape-valid
//     "50"-"99" would assert the CN ceiling as a fact about this other
//     command's parameter, which is exactly the kind of borrowed assumption
//     this package exists not to make (fakeft2000's own register states the
//     identical reasoning for its own CN/P9 pair).
//     (parser.go: validToneDigits)
//
//  4. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go — the slot form, the 8-digit frequency, the clarifier sign
//     and 4-digit magnitude, the two clarifier flags, the 12-member mode
//     nibble, MW's fixed Kind byte, the 3-state CTCSS byte, the tone digits'
//     shape, the 3-state shift byte — is ASSUMED to be what the radio itself
//     enforces on a Set. The manual prints the legends; it does not say what
//     the radio does with a Set that leaves one. Nothing here NORMALISES: a
//     byte outside its legend is refused, never quietly corrected.
//     (parser.go: the validators, parseMemoryBlock)
//
//  5. AN ANSWER'S KIND BYTE IS ALWAYS '1' (Memory), FOR EVERY SLOT THIS FAKE
//     HOLDS, MEMORY OR PMS ALIKE. MR's read legend has exactly two members,
//     "0: VFO 1: Memory" (layout:857), and nothing states which one a PMS
//     band-edge answers with; '1' is the only member that is not plainly
//     false for a slot this fake actually stores. Mirrors fakeft2000's own
//     register entry for the identical question on that radio's P7 — cited
//     as a parallel, not re-derived, since the two manuals never agree with
//     each other about anything else on this byte.
//     (parser.go: kindMemory)
//
//  6. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in MW's block
//     mentions the current-channel selection (layout:876-899); only an
//     MC-set changes what "MC;" answers.
//     (parser.go: handleMW)
//
//  7. AN MW SET CREATES AN ABSENT CHANNEL AS WELL AS OVERWRITING A PRESENT
//     ONE. "MEMORY CHANNEL WRITE" names no precondition and MC's own span
//     (000-117) is drawn once, not doubled for "already populated" and "not
//     yet populated" (layout:795-802). A write command that refused to
//     create a channel it had no record for would need a second, unprinted
//     rule to do so.
//     (parser.go: handleMW)
//
//  8. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than
//     maxAccumulatorBytes have accumulated without a ';', this fake replies
//     "?;" once and discards bytes up to and including the next ';' before
//     resuming normal framing. NOT a radio claim: the manual prints no
//     buffer size, and this is this package's own bounded-input policy,
//     inherited in shape from fakeft2000's identical one.
//     (parser.go: reassembler)
//
//  9. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Every byte of every shipped
//     record is a printed constant or a printed legend value, but no MW, MR
//     or MC frame is printed as a literal anywhere in the manual, so the
//     CROSS-FIELD COMBINATION has never been printed or observed. The
//     records are not factory defaults and no byte is derived from another
//     model or padded to make a test pass.
//     (image.go: DefaultImage)
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AND SILENCE ON AN ACCEPTED SET. The manual
// prints no acknowledgement anywhere in its 90-command index (layout:129-181)
// or in MW's own chart (layout:876-899), and every registered Yaesu fake in
// this fleet already plays exactly this convention — inherited in shape, not
// re-assumed per package.
//
// COMMAND NAMES ARE ACCEPTED IN EITHER CASE. "A command consists of 2
// alphabetical characters. You may use either lower or upper case
// characters." (layout:106-107) — a MANUAL FACT, printed for this radio by
// name, not inherited from a sibling.
//
// THE MODE LEGEND HAS NO '0' PLACEHOLDER. What MR's and MW's own legends
// print (layout:844-862, :876-899), not an assumption — see the
// sibling-comparison section above for the MD erratum this deliberately does
// NOT widen the legend against.
//
// THE SLOT SPAN IS 000-117, AND THE PMS PAIRS ARE DIRECT DECIMAL NUMBERS.
// MC's legend prints both the span and its decomposition in one place
// (layout:795-802); nothing here re-derives it.
