// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets570 is an independent, stdlib-only simulator of the Kenwood
// TS-570 family (TS-570D, TS-570S, TS-570DG) over an in-memory serial
// connection (Radio.Port()). It plays the same role internal/fakets480 and
// internal/fakets590 play for their own rows: the test double a real driver
// exchanges commands against, built from the radio's own PC-command document
// rather than from any of this project's own code.
//
// # The hard rule: NOTHING project-internal
//
// fakets570 MUST NOT import any package of this project — not core/kw, not
// core/kw/ts570, not core/codeplug, not core/spec, not core/driver/ts570, and
// not any sibling fake. Standard library only, in every non-test file, in
// this directory AND every directory beneath it (imports_test.go enforces
// this recursively). Every byte offset, field width and legend below is
// re-derived from `docs/superpowers/ts570-capability-matrix.md` (itself
// derived from the TS-570 Instruction Manual's "COMPUTER CONTROL COMMAND
// TABLES" appendix, cited "PDF p.N (printed M)") and from the manual's own
// layout text directly — never from core/kw/ts570 or core/driver/ts570.
//
// THE ONE EXCEPTION IS internal/fakepipe: the net.Pipe pair, the goroutine
// bookkeeping, the interruptible latency wait and the raw write. It is
// PROTOCOL-FREE — it carries no framing, no field layout and no reply
// building — so a bug in it cannot make a wrong codec look right; it can only
// stop bytes moving, which this package's own tests notice at once.
//
// The reasoning for the fence as a whole is internal/fakets590's own: if this
// fake reused core/kw's or core/driver/ts570's understanding of the wire, a
// systematic bug in that understanding — an off-by-one offset, a validation
// rule subtly wrong — would sit on both sides of every "send a command, check
// the reply" test this project runs, and never surface. The fake disagreeing
// with the codec is what makes the cross-check mean anything.
//
// # Three rows, one Option
//
// A single *Radio serves all three registry rows. WithModelName selects
// which one: "TS-570D" (the default a bare New() resolves to), "TS-570S" or
// "TS-570DG" — anything else panics, a construction-time programming error
// rather than a silently-wrong row. This is a DELIBERATE DEVIATION from
// internal/fakets590's own precedent (a required Row enum argument, no zero
// value): this milestone's fakes use the option form instead, so a caller not
// naming a model gets a sane default row rather than a construction that
// panics — internal/fakeic7851's mechanism, not internal/fakets590's.
//
// TS-570D is the default because it is the row the manual's own cover page
// names ("HF TRANSCEIVER TS-570D"), one of the two rows with direct textual
// evidence throughout the document (matrix §0, §2), and the lower of the two
// printed CATID values (017 against TS-570S's 018) — not an arbitrary pick
// among three equally-evidenced rows. TS-570DG is never the default: every
// cell in its column is UNVERIFIED-BY-INHERITANCE (matrix §5), and a fake
// claiming no hardware access should not default to the one row with no
// direct textual evidence of its own.
//
// # The 28-byte MR/MW record
//
// Positions are 1-indexed, matching the matrix's own table (§1.2):
//
//	1-2    "MR"/"MW"
//	3      P1   half: '0' RX/Start, '1' TX/End
//	4      P2   NOT USED — a dash entry, no legend printed at all
//	5-6    P3   memory channel, "00"-"99", no bank digit
//	7-17   P4   frequency, 11 ASCII digits
//	18     P5   mode, one of '0'-'9' (core/kw.Mode's ten nibbles, matrix §1.3)
//	19     P6   channel lockout, '0' off / '1' on
//	20     P7   tone mode, '0' OFF / '1' ON — TWO values, not the 590 pair's
//	            four or the 480's three
//	21-22  P8   tone number, "01"-"39", shared by tone_tx and tone_rx (matrix §2)
//	23-27  P9   NOT USED — five bytes, another dash entry
//	28     ';'  terminator
//
// P2 and P9 are genuinely unused, not printed constants (contrast the
// TS-480's P2, "Always 0" — a legend this radio's book does not print for
// its own P2 or P9). This fake stores whatever a Set carries there, subject
// only to the manual's general Set-direction rule for an inapplicable
// parameter: "any character except the ASCII control codes (00 to 1Fh) and
// the terminator (;)" (printed folio 70). Register entry 3 below.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something neither the matrix nor the
// manual settles is listed here, cited by name in the code beside it.
//
//  1. THE DEFAULT ROW is TS-570D. See "Three rows, one Option" above.
//
//  2. AN UNWRITTEN CHANNEL — EITHER HALF — ANSWERS THE ZERO RECORD. Neither
//     the matrix nor the manual pages it cites say anything about what an MR
//     of an empty channel answers; TS-570's own book is silent where the 590
//     pair's states "If the selected channel is empty, P4 ~ P15 will be 0"
//     (matrix §1.2 cites no such sentence for this row). This fake reads the
//     sibling Kenwood fakes' own A4-style assumption across, unlifted: the
//     zero shape is at least internally consistent with this row's own
//     hard-wired legend (mode '0' is "No selection", matrix §1.3), and no
//     TS-570 has ever answered this project (matrix §0, writeTrialsComplete
//     false for all three rows).
//
//  3. P2 AND P9 ARE STORED VERBATIM, NOT GIVEN A LEGEND. The matrix's own
//     register home `ts570-p2-unused` leaves "what byte the write side
//     actually emits" to the implementer; this fake does not choose one
//     emitted value because it is not the emitter — it stores whatever a Set
//     sends, validated only by the general filler-byte rule (manual printed
//     folio seventy), and echoes it back on a Read. The zero record's own P2
//     and P9 use '0' fill, matching entry 2's zero shape.
//
//  4. THE TONE NUMBER (P8) IS STORED, NOT RANGE-CHECKED. The Parameter Table
//     prints "01~39" (matrix §1.2, format code 14) but says nothing about
//     what a Set carrying a value outside that range does inside a memory
//     frame — the same shape of gap the sibling Kenwood fakes' own tone/CTCSS
//     index entries name. Only the field's two-digit SHAPE is enforced.
//
//  5. THE ID COMMAND HAS NO SET. The matrix quotes only the Answer shape,
//     "ID P1 P1 P1 ;", six bytes (matrix §2, CATID; PDF p.83, printed 77) —
//     the same three-digit-P1 form the 590/480 matrix already records as "a
//     third form" of Kenwood identity answer, always read-only in that
//     family. No TS-570-specific text says so directly, so this is this
//     fake's own crossing, not a manual fact.
//
//  6. TS-570DG's CATID IS A PLACEHOLDER, "000". The matrix states plainly
//     that no document assigns TS-570DG a CATID at all (matrix §2, register
//     home `ts570-dg-catid-assumed`) — there is nothing to read, and "000" is
//     not a claim about any radio. WithCATID overrides it, for the day a
//     DG-specific document or a corroborating ID capture surfaces.
//
//  7. WithStreamError SCRIPTS "E;"/"O;" (lift-K follow-up, commit e7515d0;
//     superseding this entry's original stance under option 2). Both tokens
//     are printed in this radio's own manual (printed folio 70, layout lines
//     5158-5175, the same general error-message table the 480/590 pair
//     carry) — a fact this fake's first report flagged as a correction to
//     reviews/driver-ts570.md's original "no citation exists" wording. The
//     lift-K follow-up then wired a real, cited Book570 stream-error entry
//     in core/kw, and core/driver/ts570's `Open` builds ordinary framing
//     unconditionally, so a live session can now reach that path — this
//     option (StreamError, parser.go) is what lets a test script it against
//     this fake, the same shape internal/fakets590's own WithStreamError
//     takes. The basic "?;" syntax-refusal token remains separate and
//     unconditional — the ordinary reply to every malformed or unrecognised
//     frame — and was never in question.
//
//  8. AI (AUTO INFORMATION) IS IMPLEMENTED — Set/Answer "A I P1 ;", Read
//     "AI;", legend '0'-'3' (Parameter Table format 32) — because
//     core/transport.Engine.Init opens every session with a fire-and-forget
//     "AI0;" (internal/wiring.OpenFakeSessionFor's own critical path), and a
//     fake with no AI handler answers "?;" to it, failing Open for all three
//     rows. The power-on value, '0', is MANUAL-EVIDENCED ("Switching the
//     transceiver ON restores '0'.", the AI command's own page) — not this
//     entry's own assumption. What IS this fake's own choice: it never
//     pushes anything unsolicited, whatever AI is set to (see "What this
//     fake deliberately does NOT model" below).
//
// # What this fake deliberately does NOT model
//
// SATELLITE MEMORY, EX/MENU INVENTORY. Both are out of scope for this
// package regardless of row (spec.md's open questions 1 and 2; matrix §7)
// and are not simulated even as unsupported stubs.
//
// AN UNSOLICITED AI PUSH. Whatever AI is set to, this fake never sends
// anything the host did not ask for — no periodic IF/Answer push, no front
// panel driving one. Register entry 8.
//
// A FRONT PANEL. Nothing here models one.
//
// FAULT INJECTION BEYOND THE BOOK. No dropped-reply, garbled-byte or
// chunked-write option — those exercise core/transport.Engine, already
// covered against internal/fakeradio's fault suite.
package fakets570
