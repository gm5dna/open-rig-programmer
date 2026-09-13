// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft2000 is an independent, stdlib-only FT-2000/FT-2000D CAT
// simulator: the test double the transport engine, core/driver/ft2000, the
// CLI's --fake mode and the GUI's demo mode run against, the role
// internal/fakeradio plays for the FT-710 and internal/fakeft991a for the
// FT-991A.
//
// TWO ROWS, ONE PACKAGE, PLAIN OPTION. The FT-2000 and FT-2000D share one
// SERIES manual and one wire record byte for byte — the only difference
// either book ever states is the ID answer's CATID ("0251: FT-2000" /
// "0252: FT-2000D", layout:725-732) — so one package serves both and
// WithModelName says which. THIS IS A DELIBERATE DEVIATION from
// internal/fakets590's REQUIRED Row argument (this milestone's plan, and the
// brief that dispatched this package): New() alone resolves to FT-2000,
// CATID "0251", because FT-2000 is the row this package is named after and
// the row the manual's own ID legend lists first — not an arbitrary
// tie-break, and nothing about a caller naming no model should panic.
//
// THIS RADIO IS NoTag: no CAT command in the full 90-command index
// (layout:113-165) reads or writes a channel name, and there is no
// name/tag route of any kind in either book — the fact
// core/spec.Capabilities.NoTag exists to state (merged d0b2498). So there is
// no Tag field in MemState, no charset, and no echo: building one for a radio
// whose matrix found none would be inventing wire behaviour, not modelling
// it.
//
// The record is WRITE/READ SEPARATE, not combined: MW (MEMORY CHANNEL WRITE,
// availability "O X X X", layout:924-936) is Set-only, and MR (MEMORY CHANNEL
// READ, "X O O X", layout:894-905) is Read/Answer-only — unlike the FT-991A's
// single combined MT, this radio has no command that carries the tag anyway,
// so there is nothing for a combined form to gain. MC (MEMORY CHANNEL,
// "O O O X", layout:844-852) selects the current channel over the same
// 001-117 span MW/MR/MC all draw on: 001-099 plain memory channels, 100-117
// nine PMS pairs as DIRECT DECIMAL CHANNEL NUMBERS (matrix §2.4) — the same
// wire numbering internal/fakeft991a's PMS slots already use, so this
// package's slot grammar is copied in SHAPE from that sibling, not
// re-invented.
//
// # The hard rule: NOTHING project-internal
//
// fakeft2000 MUST NOT import any package of this project — not core/cat, not
// core/cat/ft2000, not core/codeplug, not core/spec, and not
// internal/fakeft991a or any sibling fake. Standard library only, in every
// non-test file, in this directory and every directory beneath it
// (imports_test.go). THE QUARANTINE GOES FURTHER for this package than for
// most: it was built without reading core/kw/ft2000 or core/driver/ft2000 at
// all — not even for a byte offset or an enum spelling — precisely so that a
// systematic error in that driver's own reading of the manual (an off-by-one
// offset, a validation rule subtly wrong) cannot sit on both sides of the
// cross-check `go test ./core/driver/ft2000/...` runs against a fake shaped
// like this one. Every field below is re-derived from the FT-2000 SERIES CAT
// OPERATION REFERENCE BOOK (revision EH025H124, docs/fixtures-private/manuals/
// ft2000_layout.txt, gitignored) and from
// docs/superpowers/ft2000-capability-matrix.md, cited "layout:NNN" throughout
// — a citation names where the chart is, not a link, since the manual itself
// is never committed.
//
// # A SIBLING of internal/fakeft991a, not a refactor of it
//
// This package copies a good deal of that sibling's SHAPE: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Option pattern, the recursive import fence. That
// duplication is deliberate for the same two reasons fakeft991a's own doc.go
// gives: sharing it would need a project-internal helper package, which the
// hard rule forbids, and two radios agreeing on a shape is a fact about those
// two radios, not a definition worth factoring out. Where this radio's OWN
// wire disagrees with that sibling's, it is this manual's legend, not a
// preference:
//
//   - THE RECORD IS WRITE/READ SEPARATE, NOT COMBINED, AND THERE IS NO TAG AT
//     ALL — see above. MemState here has no Tag and no P11 schema byte.
//   - P8 IS THREE-VALUED, NOT FIVE. "0: CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS
//     ENC" (layout:934, repeated on MR/MC's own blocks) — the ordinary legacy
//     domain, no DCS member, where the FT-991A's is the fleet's first
//     five-value P8.
//   - P9 IS A LIVE 2-DIGIT TONE-TABLE INDEX WHERE EVERY OTHER REGISTERED
//     DIALECT PRINTS AND ENFORCES A FIXED "00" (matrix §1.3). This is this
//     radio's own novelty, the mirror image of the FT-991A's DCS one, and
//     nothing about either precedent is read across to the other (matrix
//     §2.9's own warning, repeated at TONE INDEX SHAPE below).
//   - THE MODE LEGEND HAS NO '0' PLACEHOLDER. Twelve values, `1`-`9` then
//     `A`-`C` (layout:934-936, identical on MR/MC), with no hole and nothing
//     printed for a thirteenth nibble. Where the FT-991A's dialect ASSUMES a
//     `cat.ModeUnset` placeholder nibble a parser must tolerate, no such
//     assumption is made here: this manual's legend is closed, so a mode byte
//     outside `1`-`9`/`A`-`C` is refused as a MANUAL FACT, not an omission.
//   - KIND (P7) IS WRITE-FIXED, READ-VARIABLE. MW's own block prints "P7 0:
//     (Fixed)" (layout:934) — the Set direction carries no channel
//     information at all — where MR's prints the ordinary "0: VFO 1: Memory"
//     pair (layout:894, repeated at MC's Answer). Structurally the FT-991A's
//     same split, by a different route: there P7 Set and Read/Answer legends
//     print side by side in ONE block; here they are two commands' own
//     blocks that happen to agree.
package fakeft2000

// writeTrialsComplete records this package's one honest status line: NO
// FT-2000 OR FT-2000D HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE
// IS AVAILABLE TO IT (the capability matrix's own opening sentence). Every
// place below marked ASSUMED is a place this fake had to decide something
// neither book settles, and none of them has been lifted by a real session.
const writeTrialsComplete = false

// THE ASSUMED REGISTER
//
// Every place this fake had to decide something neither the FT-2000 SERIES
// CAT OPERATION REFERENCE BOOK nor docs/superpowers/ft2000-capability-matrix.md
// settles is listed here, cited BY NAME at the code that implements it
// (register_test.go's TestASSUMEDRegisterIsComplete holds both halves
// mechanically), never by number.
//
//  1. EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS. A slot this fake holds no state
//     for answers "?;" to an MR read and to an MC-set, indistinguishable from
//     a slot outside 001-117 altogether. Neither book states what an MR read
//     or an MC-set of an unprogrammed channel returns; "?;" is the
//     protocol's one unattributed NAK (layout:93-108's error-table
//     convention, the same one every registered Yaesu fake plays), and this
//     is the honest default for a channel nobody has watched a radio answer.
//     (parser.go: handleMR, handleMC)
//
//  2. THE "NO SELECTION YET" CHANNEL IS "000". Neither MC's block nor any
//     other prints what "MC;" answers before any MC-set has happened — MC's
//     own legend runs 001-117 with no zero member (layout:844-846). "000" is
//     the lowest three-digit numeral outside that span, so it can never
//     collide with a real channel and an MC read against it is unambiguous.
//     It is this package's own choice, not a citation: no FT-2000 or
//     FT-2000D has been asked what it powers on to.
//     (fakeft2000.go: New; parser.go: slotNoneWire)
//
//  3. TONE INDEX (P9) IS STORED AT ITS PRINTED WIDTH, NOT RANGE-CHECKED. CN's
//     own legend prints "P2 0-49: Tone Frequency Number" (layout:322-323),
//     but neither MW's nor MR's own P9 cell restates that ceiling — both
//     print only "Tone Number (See Page 5)" (layout:936, :905). Whether the
//     0-49 ceiling that binds CN's live tone-select parameter also binds this
//     byte pair INSIDE a stored memory record is unproven either way, so only
//     the field's SHAPE (two ASCII digits) is enforced on a Set; refusing a
//     shape-valid "50"-"99" would assert the CN ceiling as a fact about this
//     other command's parameter, which is exactly the kind of borrowed
//     assumption this package exists not to make (the same reasoning
//     internal/fakets590's own register states for its TN/CN pair).
//     (parser.go: validToneDigits)
//
//  4. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go — the slot form, the 8-digit frequency, the clarifier sign
//     and 4-digit magnitude, the two clarifier flags, the 12-member mode
//     nibble, MW's fixed Kind byte, the 3-state CTCSS byte, the tone digits'
//     shape, the 3-state shift byte — is ASSUMED to be what the radio itself
//     enforces on a Set. Both books print the legends; neither says what
//     either radio does with a Set that leaves one. Nothing here NORMALISES:
//     a byte outside its legend is refused, never quietly corrected.
//     (parser.go: the validators, parseMemoryBlock)
//
//  5. AN ANSWER'S KIND BYTE IS ALWAYS '1' (Memory), FOR EVERY SLOT THIS FAKE
//     HOLDS, MEMORY OR PMS ALIKE. MR's read legend has exactly two members,
//     "0: VFO 1: Memory" (layout:894), and no book states which one a PMS
//     band-edge answers with; '1' is the only member that is not plainly
//     false for a slot this fake actually stores. Mirrors
//     internal/fakeft991a's own register entry for the identical question on
//     that radio's P7 — cited as a parallel, not re-derived, since the two
//     manuals never agree with each other about anything else on this byte.
//     (parser.go: kindMemory)
//
//  6. A SET DOES NOT MOVE THE SELECTED CHANNEL. Nothing in MW's block
//     mentions the current-channel selection (layout:924-936); only an
//     MC-set changes what "MC;" answers.
//     (parser.go: handleMW)
//
//  7. AN MW SET CREATES AN ABSENT CHANNEL AS WELL AS OVERWRITING A PRESENT
//     ONE. "MEMORY CHANNEL WRITE" names no precondition and MC's Read
//     legend's span (001-117) is drawn once, not doubled for "already
//     populated" and "not yet populated" (layout:844-846). A write command
//     that refused to create a channel it had no record for would need a
//     second, unprinted rule to do so.
//     (parser.go: handleMW)
//
//  8. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than
//     maxAccumulatorBytes have accumulated without a ';', this fake replies
//     "?;" once and discards bytes up to and including the next ';' before
//     resuming normal framing. NOT a radio claim: neither book prints a
//     buffer size, and this is this package's own bounded-input policy,
//     inherited in shape from internal/fakeft991a's identical one.
//     (parser.go: reassembler)
//
//  9. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Every byte of every shipped
//     record is a printed constant or a printed legend value, but no MW, MR
//     or MC frame is printed as a literal anywhere in either book, so the
//     CROSS-FIELD COMBINATION has never been printed or observed. The
//     records are not factory defaults and no byte is derived from another
//     model or padded to make a test pass.
//     (image.go: DefaultImage)
//
//  10. AUTOMATIC-INFORMATION SUPPRESSION. This fake never PUSHES anything
//     unsolicited, whatever AI is set to. Neither book documents what an
//     AI-ON FT-2000 or FT-2000D actually sends, when, or in what order — "AI
//     AUTO INFORMATION" appears only in the 90-command index and its own
//     four-frame block (layout:201-206), never in a worked example — and no
//     FT-2000 of either row has been observed by this project. Modelling
//     silence is the honest default, not a claim that either radio is
//     silent: core/transport.Engine's drain-to-quiet discipline is already
//     exercised against internal/fakeradio, whose own AI-flood facts are the
//     FT-710's, and no FT-2000 test in this project's plan exercises the
//     engine against a talking radio.
//     (parser.go: handleAI; fakeft2000.go: New, handleEvent)
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AND SILENCE ON AN ACCEPTED SET. Neither book
// prints an acknowledgement anywhere in its 90-command index (layout:113-165)
// or in MW's own chart (layout:924-936), and every registered Yaesu fake in
// this fleet already plays exactly this convention — inherited in shape, not
// re-assumed per package.
//
// COMMAND NAMES ARE ACCEPTED IN EITHER CASE. "A command consists of 2
// alphabetical characters. You may use either lower or upper case
// characters." (layout:106-107) — a MANUAL FACT, printed for this radio by
// name, not inherited from a sibling.
//
// THE MODE LEGEND HAS NO '0' PLACEHOLDER, AND P8 IS THREE-VALUED. Both are
// what MW's and MR's own legends print (layout:934-936, :894-905), not
// assumptions — see the sibling-comparison section above.
//
// THE SLOT SPAN IS 001-117, AND THE PMS PAIRS ARE DIRECT DECIMAL NUMBERS.
// MC's legend prints both the span and its decomposition in one place
// (layout:844-846); nothing here re-derives it.
