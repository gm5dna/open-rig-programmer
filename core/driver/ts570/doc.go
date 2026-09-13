// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts570 drives the Kenwood TS-570D, TS-570S and TS-570DG: one
// package, three registry rows, over core/kw/ts570's three layout values.
//
// A PAPER REGISTRATION, exactly as core/driver/ts480's and core/driver/ts590's
// are: no TS-570 of any suffix has ever been asked anything by this project
// (ts570-capability-matrix.md §0). writeTrialsComplete is false for every
// row and every field this row's 28-byte record expresses is labelled Read
// Unverified / Write Unverified.
//
// # ID-ONLY IDENTITY, WHERE THE 590 PAIR AND THE TS-480 EACH HAVE A SECOND
// PROBE FRAME
//
// This document's own Alphabetical Command Table prints ID, MC, MD, MR, MW
// and EX among its 62 mnemonics and neither FV (the 590 pair's firmware-
// version read) nor TY (the TS-480's hardware-variant read) at all — a
// case-insensitive grep of the manual for either name returns nothing.
// core/kw.Layout.BuildFVRead and BuildTYRead both refuse a Book570 layout by
// name (they requireBook Book590 and Book480 respectively), so Open sends
// exactly one identity frame, "ID;", and nothing past it.
//
// # THREE ROWS, ONE CATID COLLISION, NAMED RATHER THAN HIDDEN
//
// TS-570D answers "017" and TS-570S answers "018" (matrix §2, Parameter
// Table's own MODEL NUMBER format). TS-570DG answers NOTHING — no document
// prints an ID for it at all — so NewDG's driver ASSUMES the D's own value,
// 017, on the reasoning doc.go's core/kw/ts570 package states for every
// other DG cell: LayoutDG is LayoutD's config with only Model changed, and
// there is nothing more independent to assume here either (register
// ts570-dg-catid-assumed). THE CONSEQUENCE IS A REAL AMBIGUITY, RECORDED
// RATHER THAN PAPERED OVER: a driver built with NewDG accepts a radio
// reporting "017" without being able to tell an actual TS-570D from an
// actual TS-570DG, because this document gives it nothing to tell them
// apart with. That is a fact about the evidence, not a bug in Open.
//
// # THE core/kw GAPS THIS ROW MET — CLOSED BY LIFT K's FOLLOW-UP
//
// core/kw's Lift K follow-up (commit e7515d0, 13/09/2026, in this same
// worktree) closed every gap this section used to describe as unroutable
// or hand-worked-around:
//
//   - Book570 now carries real "E;"/"O;" stream-error citations
//     (bookCitations, newStreamError — cited as ts570:LINE, read from
//     this document's own full text rather than the command-table-only
//     evidence this milestone's brief originally had). A live session for
//     any TS-570 row is therefore ordinary now, on exactly the same
//     footing as ts590's: this driver's Open already built framing the
//     normal way (kw.NewFramingFor(l)) and needed no change.
//   - core/kw.BuildMWSet now fills this row's P9 "NOT USED" span
//     (positions 23-27) with the family's own '0' filler on the
//     hasTail=false path, so its output passes its own AllowedCommand —
//     write.go's WriteChannel sends the built frame unchanged and
//     TestWriteChannel_RoundTrips proves the round trip.
//   - core/kw's isEmptyWindow is now width-aware — P4 through P8 on a
//     no-tail row, citing this document's own vacant-channel note
//     directly — so core/kw.Layout.ParseMRAnswer sets Record.Empty for
//     this row's vacant shape itself; read.go no longer restates it.
//
// core/kw/kwtest's conformance suite still has three further gaps that
// commit did not touch (its own scope was the TS-2000's all-live-axis
// shape and the fixes above): checkLayoutSelfConsistency and checkIdentity
// both switch on l.Book() over exactly Book590/Book480 and fault on any
// other book, checkLayoutSelfConsistency also demands
// Byte28/Byte3940/Byte41 be set unconditionally (all three legitimately
// Unset on a no-tail layout), and checkEmptyChannel indexes past a
// 28-byte frame's last valid position — a panic, not a failed assertion.
// None is fixable from this package; core/kw/ts570/layout_test.go's
// runConformance documents and skips rather than crashing the build gate
// — see reviews/driver-ts570.md's "## Follow-up" section for the full
// account of what changed and what remains.
//
// # ERASE IS DOCUMENTED, AND STILL NOT BUILT
//
// The MW chart's own note — "the memory channel becomes a vacant channel if
// all frequency digits are '0'" (matrix §3, "the ONE field where TS-570 is
// evidenced MORE strongly than either registered sibling") — is a real,
// stronger erase citation than either registered row has. It changes
// nothing here: core/kw.BuildMWSet refuses ANY record whose FreqHz is zero,
// unconditionally, as the empty channel's own value (decision 8, the same
// refusal the 590 pair's and the TS-480's rows meet). spec.FieldErase is
// therefore the zero FieldSupport on every bank of every row, the same as
// every other Kenwood package, and the stronger citation is recorded here
// rather than acted on: building the short erase form is a core/kw change,
// not a driver one, and this milestone does not make it.
//
// # tone_tx AND tone_rx SHARE ONE WIRE BYTE
//
// P8 is this row's only tone index (matrix §2): there is no separate
// receive-tone byte at all. Both neutral fields are graded Sup/Sup and both
// read the SAME parsed index; a write that names two DIFFERENT tones for
// tone_tx and tone_rx is refused (write.go) rather than silently choosing
// one, because there is only one byte to carry either.
//
// # THE CTCSS TABLE IS THIS ROW'S OWN, NOT core/driver/ts590's
//
// The subtone frequency table (manual layout lines 2179-2190, printed page
// 25) lists 38 conventional tones, 67.0 Hz through 250.3 Hz at indices
// 01-38, then a 39th entry, 1750 Hz, which the same page calls a EUROPEAN
// FM-repeater BURST tone rather than a CTCSS sub-audible tone (matrix §2,
// register ts570-tone39-burst-not-ctcss). It is NEITHER core/spec's shared
// 50-tone Yaesu chart NOR core/driver/ts590's own 43-entry table — three of
// this book's 38 conventional entries (69.3, 206.5 and 229.1 Hz) are absent
// from it, which a reader moving from ts590 would otherwise expect. This
// package carries its own 39-entry ts570CTCSSTones (caps.go), transcribed
// directly from the manual rather than borrowed from a sibling package.
package ts570
