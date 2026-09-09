// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets890 simulates a Kenwood TS-890S's PC-control behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double this
// registry row's own layers run against: the transport engine,
// core/driver/ts890, the CLI's --fake mode and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakets590 for the
// TS-590 pair.
//
// ONE FAKE, ONE REGISTRY ROW. Unlike internal/fakets590, which serves two
// siblings out of one book, this package serves one radio out of one book and
// therefore has no Row argument and no per-row table. Its sibling
// internal/fakets990 is a SEPARATE PACKAGE for the same reason the two driver
// packages are separate: the two books print different memory grids (40-50
// bytes here, a fixed 57 there), different mode legends, different lockout
// encodings and two disjoint menu charts, and a shared table would let one
// radio's evidence stand in for the other's.
//
// # The hard rule: NOTHING project-internal
//
// fakets890 MUST NOT import any package of this project — not core/kw, not
// core/kw/ma, not core/codeplug, not core/spec, and not internal/fakets590 or
// any sibling fake. Standard library only, in every non-test file, in this
// directory AND every directory beneath it. Every byte offset, field width and
// validation rule below is re-derived from the TS-890S PC Control Command
// Reference Guide (rev 1, January/30/2019) — cited "890:NNNN" throughout, the
// same convention core/kw/ma/doc.go uses, naming where a chart is rather than
// linking to it, because the manual itself is gitignored
// (docs/fixtures-private/manuals/).
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/kw/ma's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake has
// to be able to DISAGREE with the production codec for a test against it to
// mean anything, and it can only disagree if it was built from the manual
// rather than from the code.
//
// The fence is enforced mechanically and recursively (imports_test.go): this
// directory and every one beneath it.
//
// THE ONE EXCEPTION IS internal/fakepipe. It carries the net.Pipe pair, the
// goroutine bookkeeping, the interruptible latency wait and the raw write, and
// it is permitted because it is PROTOCOL-FREE — it sees []byte and a duration
// and nothing else, so it carries no framing, no field layout and no reply
// building. A bug in it cannot make a wrong codec look right; it can only stop
// bytes moving, which this package's own tests notice at once. Everything
// above the wire — the reassembler, the parser, the image, the replies — stays
// here, written independently.
//
// # A SIBLING of internal/fakets590, not a refactor of it
//
// This package duplicates a good deal of that one's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options. The duplication is
// deliberate and it is not going to be factored into a shared "fake core"
// package: a shared helper would be a project-internal import, which the hard
// rule above forbids in every fake, so the only way to share code would be to
// abandon the property that makes any of them worth having.
//
// And the two radios do not in fact agree. Where this fake diverges from its
// Kenwood siblings, the divergence is this book's:
//
//   - THERE IS NO MR, MW, MC OR TY COMMAND ON THIS RADIO. The memory record is
//     MA0, a 13-parameter grid whose Read carries its own channel number
//     (890:3184-3186) and whose Answer is 40 to 50 bytes with a FLOATING
//     TERMINATOR (890:3189-3204) — where every 590-family frame is exactly 50
//     bytes with a fixed ';'.
//   - THE COMMAND NAME IS 2 TO 5 CHARACTERS, not 2 to 3: "A command consists
//     of 2 to 5 alphanumeric characters." (890:76-80). "MA0" is a
//     three-character name, so this package's dispatcher cannot split a frame
//     at a fixed offset.
//   - THE MODE LEGEND IS OM's SIXTEEN NIBBLES 0-9 and A-F (890:3976-3992),
//     where the 590 pair's MD legend prints ten.
//   - THE EX ADDRESS IS A GROUPED TRIPLE of one, two and two digits
//     (890:1897-1911), where every other Kenwood row in this repository
//     carries a single three-digit menu number.
//   - THE AI LEGEND PRINTS FIVE VALUES, two of them "Not used"
//     (890:175-181), where the 590's prints three and the TS-480's four.
//
// # WHERE THE EX INVENTORY COMES FROM
//
// The widths table ex.go answers from is NOT hand-typed. It is projected at
// init, by exinventory.go, from this package's OWN COPY of TRANSCRIPTION B —
// transcription-b-890s.csv beside this file, with PROVENANCE.md recording
// where the copy came from and why it is a copy rather than a move.
//
// That is the whole mechanism of this row's two-source cross-check:
//
//   - the CODEC's inventory (core/kw/ma/exinventory890s_gen.go) is generated
//     from TRANSCRIPTION A (core/kw/ma/menu890s.csv) by internal/extable;
//   - THIS inventory is projected from TRANSCRIPTION B by exinventory.go,
//     which imports nothing project-internal at all;
//   - core/transport/ex_crosscheck_ts890_test.go proves the two agree, address
//     for address and width for width, and drives every address over the wire.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined
// agent with no repository access and no sight of A, the page ledger or any
// row count (core/kw/ma/crosscheck_test.go records the artefacts and hashes
// them). So a mis-read row in either transcription, or a defect in either
// parser, surfaces as a cross-check MISMATCH rather than as two tables quietly
// agreeing on the same wrong number.
//
// If the cross-check ever fires: report the diff, do NOT edit either table to
// make it pass. Which side is wrong (or whether the chart itself is) is an
// arbitration against the PDF.
package fakets890
