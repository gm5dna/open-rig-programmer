// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeftdx1200 is an independent, stdlib-only FTdx1200 CAT simulator:
// the test double the transport engine, core/driver/ftdx1200, the CLI's
// --fake mode and the GUI's demo mode run against, the role internal/fakeradio
// plays for the FT-710 and internal/fakeft2000 for the FT-2000 family.
//
// ONE ROW, TWO CATIDs. The FTdx1200 is a single radio whose ID answer depends
// on whether the optional FFT-1 board is fitted: "P1 0582: FTdx1200 (optional
// FFT-1 is installed) / 0583: FTdx1200 (optional FFT-1 is not installed)"
// (docs/superpowers/ftdx1200-capability-matrix.md §2.2, manual layout:727-733).
// This is NOT a two-row situation like FT-2000/FT-2000D (two model names,
// two constructors' worth of everything else too) — every other byte,
// legend and range in this radio's manual is a single, undifferentiated
// text. New defaults to "0582" (FFT-1 fitted, the ID legend's first-listed,
// canonical value); WithFFT1NotFitted flips it to "0583". No WithModelName
// map: there is only one row here.
//
// THIS RADIO IS NoTag: whole-document grep of the layout extraction found no
// name/tag/label route at all (matrix §0), stronger than FT-2000's own
// finding (not even a stray pin-name legend). So there is no Tag field in
// MemState, no charset, and no echo.
//
// THE RECORD IS MW-Set / MR-Answer, 27 bytes, 8-digit frequency, 24-byte
// field block — byte-for-byte the same shape as internal/fakeft2000's own
// (matrix §1.1), which is why that package is this one's shape template
// rather than internal/fakeft991a. PMS is numeric, 100-117, nine pairs,
// direct decimal channel numbers, the same span and grammar as fakeft2000's.
//
// # The hard rule: NOTHING project-internal
//
// fakeftdx1200 MUST NOT import any package of this project — not core/cat,
// not core/cat/ftdx1200, not core/codeplug, not core/spec, and not
// internal/fakeft2000 or any sibling fake. Standard library only, in every
// non-test file, in this directory and every directory beneath it
// (imports_test.go). This package was built without reading
// core/driver/ftdx1200 at all — not even for a byte offset or an enum
// spelling — precisely so that a systematic error in that driver's own
// reading of the manual cannot sit on both sides of the cross-check
// `go test ./core/driver/ftdx1200/...` runs against a fake shaped like this
// one. Every field below is re-derived from
// docs/superpowers/ftdx1200-capability-matrix.md, cited "matrix §N" (the
// matrix itself cites "layout:NNN" against the gitignored manual layout
// extraction — not re-cited here, to avoid restating a citation this
// package never opened).
//
// # A SIBLING of internal/fakeft2000, not a refactor of it
//
// This package copies that sibling's SHAPE: the pipe-and-goroutine Radio,
// the bounded reassembler, the "?;" convention, the per-command handlers,
// the Option pattern, the recursive import fence, the 24-byte field block at
// the same offsets. That duplication is deliberate, for the same two
// reasons fakeft2000's own doc.go gives. Where THIS radio's own wire
// disagrees with that sibling's, it is this matrix's finding, not a
// preference:
//
//   - MODE HAS A GENUINE HOLE AT 'A', AND NO 'D' ANYWHERE. Twelve print
//     positions, eleven real values: "1: LSB 2: USB 3: CW 4: FM 5: AM 6:
//     RTTY-LSB 7: CW-R 8: DATA-LSB 9: RTTY-USB A: ---- B: FM-N C: DATA-USB"
//     (matrix §1.2) — true on MD, MR and MW alike. 'A' is refused on every
//     write, not merely absent from the default image; fakeft2000's own
//     Mode legend has no hole at all.
//   - P9 (TONE) IS PRINTED-FIXED "00" ON BOTH READ AND WRITE (matrix §1.3):
//     "P10 00: (Fixed)" on MR, "P9 00: (Fixed)" on MW. Where fakeft2000's P9
//     is a LIVE two-digit tone-table index, this radio's is not a live
//     value on either side — MemState carries no Tone field at all, and a
//     Set must supply the literal bytes "00" or be refused.
//   - TWO CATIDs, ONE ROW, ONE CONSTRUCTOR — see above; fakeft2000 has two
//     rows (WithModelName) sharing an identical record.
package fakeftdx1200

// writeTrialsComplete records this package's one honest status line: NO
// FTdx1200 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE IS
// AVAILABLE TO IT (the capability matrix's own opening sentence). Every
// place below marked ASSUMED is a place this fake had to decide something
// the matrix does not settle, and none of them has been lifted by a real
// session.
const writeTrialsComplete = false

// THE ASSUMED REGISTER
//
// Every place this fake had to decide something the capability matrix does
// not settle is listed here, cited BY NAME at the code that implements it
// (register_test.go's TestASSUMEDRegisterIsComplete holds both halves
// mechanically), never by number.
//
//  1. EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS. A slot this fake holds no state
//     for answers "?;" to an MR read, indistinguishable from a slot outside
//     the valid span altogether. The matrix states no MR/MW behaviour for
//     an unprogrammed channel; "?;" is the fleet-wide convention every
//     registered Yaesu fake plays.
//     (parser.go: handleMR)
//
//  2. CHANNEL "000" IS OUT OF SCOPE. The matrix's own §2.4 finding: MC's P1
//     legend spans "000-117" but MR's and MW's own P1 legends both print
//     "(001 ～ 117)", excluding "000" — an open erratum the matrix itself
//     declines to resolve (a QMB/VFO-adjacent pseudo-channel by analogy with
//     other Yaesu radios, though nothing in this manual names it as such).
//     This fake follows MR/MW (the verbs its own record is built from,
//     matrix §2.4's own choice) and refuses "000" as a slot on every
//     command it implements; MC itself is not modelled at all (see entry 8).
//     (parser.go: validSlot, parseSlotForm)
//
//  3. SET-DIRECTION FIELD STRICTNESS. Every wire-level validator in
//     parser.go is ASSUMED to be what the radio itself enforces on a Set.
//     The matrix prints the legends; it does not say what the radio does
//     with a Set that leaves one. Nothing here normalises: a byte outside
//     its legend is refused, never quietly corrected.
//     (parser.go: the validators, parseMemoryBlock)
//
//  4. AN ANSWER'S KIND BYTE IS ALWAYS '1' (Memory), FOR EVERY SLOT THIS FAKE
//     HOLDS, MEMORY OR PMS ALIKE. MR's read legend has exactly two members,
//     "0: VFO 1: Memory" (matrix §1.4), and the matrix does not state which
//     one a PMS band-edge answers with; '1' is the only member that is not
//     plainly false for a slot this fake actually stores. Mirrors
//     fakeft2000's own register entry for the identical question.
//     (parser.go: kindMemory)
//
//  5. P9 (TONE) IS PRINTED-FIXED "00" ON BOTH DIRECTIONS, AND NOTHING ELSE
//     IS ACCEPTED. The matrix states the fixed value on both MR and MW
//     (§1.3); it does not say what, if anything, a Set carrying a different
//     two-digit value does. Treated the same as the write-fixed Kind byte
//     (matrix §1.4's own idiom for a "(Fixed)" field): a Set must supply the
//     literal "00" or be refused, and every read answers "00" regardless of
//     what was ever sent, since no live state exists for this byte pair to
//     hold.
//     (parser.go: validToneBytes, parseMemoryBlock, appendMemBlock)
//
//  6. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than
//     maxAccumulatorBytes have accumulated without a ';', this fake replies
//     "?;" once and discards bytes up to and including the next ';' before
//     resuming normal framing. NOT a radio claim: the matrix states no
//     buffer size; this is this package's own bounded-input policy,
//     inherited in shape from fakeft2000's identical one.
//     (parser.go: reassembler)
//
//  7. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Every byte of every shipped
//     record is a printed constant or a printed legend value, but no MW or
//     MR frame is printed as a literal anywhere in the matrix or manual, so
//     the CROSS-FIELD COMBINATION has never been printed or observed. The
//     records are not factory defaults and no byte is derived from another
//     model or padded to make a test pass.
//     (image.go: DefaultImage)
//
//  8. MC (CURRENT CHANNEL SELECTION) IS NOT IMPLEMENTED. The matrix cites
//     MC's legend only to derive the Banks/slot-span facts (§2.4); nothing
//     in the brief or spec that dispatched this package names MC as a
//     behaviour to model, and modelling it would require resolving entry
//     2's channel-"000" erratum one way or the other on no evidence. Left
//     out entirely rather than guessed. AI (auto information) IS modelled
//     — not because the matrix documents it (it does not: the brief's
//     pins are ID/PMS/P9/Mode, and this axis is unread), but because
//     core/transport.Engine.Init opens every CAT session with an
//     unconditional "AI0;", the same fleet-wide reason fakeft2000's own
//     handleAI exists; this fake never pushes anything unsolicited,
//     whatever AI is set to.
//     (fakeftdx1200.go: absence of an "MC" case; parser.go: handleAI)
