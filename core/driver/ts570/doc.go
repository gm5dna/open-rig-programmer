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
// # THE FOUR core/kw GAPS THIS ROW MEETS, RATHER THAN FIXES
//
// All four are Lift K's, not this package's own: RecordLen became a
// per-layout axis (core/kw/layout.go) but three consumers elsewhere in
// core/kw were not updated for a value narrower than the family's 50-byte
// grid. Fixing any of them means editing core/kw, which this milestone
// reserves to one cited exception (errors.go's Book570/Book870S
// stream-error entries) that this package does not use. Three are routed
// around entirely within this package; the fourth cannot be, and is
// recorded rather than hidden:
//
//   - core/kw/errors.go's newStreamError PANICS if ever asked for a
//     Book570 "E;"/"O;" cause, because neither S2 nor S3's evidence
//     transcribes one. This driver's Open and every session method build
//     ordinary frames through the shared framing adapter exactly as every
//     other Kenwood row does — there is no separate code path here that
//     avoids IsFatal — so the honest statement is the one lift K's own
//     report already makes: no TS-570 has ever been on a real port, this
//     row is a paper registration, and the panicking branch is therefore
//     never reached in practice. It would first be reached the day a real
//     TS-570 is connected and its own stream sends a genuine "E;"/"O;" —
//     at which point the fix is a citation into core/kw/errors.go, not a
//     change here.
//   - core/kw/kwtest's conformance suite (checkMemorySets,
//     checkGateRefusesAMutatedPrintedFixedByte) hardcodes the family's
//     full 50-byte RecordLen and a non-empty PrintedFixed set, neither of
//     which this row's genuinely 28-byte, no-tail layout has. See
//     core/kw/ts570/layout_test.go's runConformance and
//     reviews/driver-ts570.md.
//   - core/kw/record.go's isEmptyWindow tests positions 7-41 and refuses
//     to fire at all on a frame shorter than that window
//     (`len(frame) <= recEmptyHiOff`), so it can never recognise this
//     row's own genuinely 28-byte vacant-channel answer. THIS ONE IS
//     ROUTED AROUND, NOT MERELY DOCUMENTED: the MR chart's own note
//     ("For a vacant channel, the Answer command sends '0' for all
//     parameters except the memory channel number.", manual layout lines
//     ~5926-5928, the same printed page as the byte diagram, PDF p.84)
//     gives this row its OWN vacant shape — P4 through P8 all zero — and
//     read.go's isVacantAnswer tests it directly against the raw frame,
//     before ParseMRAnswer would otherwise refuse the '0' mode nibble.
//     Every real reason a never-written TS-570 channel would fail to read
//     is closed by this package alone.
//
// # THE WRITE PATH CANNOT PASS ITS OWN GATE TODAY, AND THIS ONE CANNOT BE
// ROUTED AROUND
//
// core/kw.BuildMWSet allocates `make([]byte, l.recordLen)` and, on the
// hasTail-false path this row's 28-byte RecordLen takes, never writes
// positions 23-27 (the P9 "NOT USED" span, matrix §1.2) before the
// terminator — they are left at Go's zero byte, 0x00, never the family's
// own '0' filler (the byte core/kw's P2Unused slotWire case already uses
// one position to the west, for exactly this row). A 0x00 byte fails the
// outbound envelope's printable-ASCII rule, so BuildMWSet's OWN OUTPUT
// FOR THIS ROW IS REFUSED BY ITS OWN GATE, unconditionally, for every
// record this row can build.
//
// THIS IS NOT ROUTABLE AROUND FROM A DRIVER PACKAGE, and write.go's own
// doc comment records why a hand-patched frame was tried and abandoned
// during this package's development: core/kw.Layout.AllowedCommand's
// validMWCommand re-parses whatever frame it is offered and then requires
// BYTE-FOR-BYTE EQUALITY with BuildMWSet's own rebuild of it — which
// carries the identical zero bytes. Correcting the span therefore makes
// the frame pass the envelope and fail the round-trip check instead; no
// 28-byte MW frame satisfies both checks at once as core/kw stands today.
// Bypassing AllowedCommand by calling kw.NewFraming directly, or by
// hand-building a frame outside BuildMWSet's own validated output and
// sending it past the gate, are both closed: the former is the exact
// misuse internal/guards.TestKenwoodDriversUseNewFramingFor exists to
// catch, and the latter would mean this package asserting a frame shape
// no builder in core/kw endorses, which is precisely the invention this
// whole codec is built to refuse.
//
// THE CONSEQUENCE: Session.WriteChannel reaches this ladder's final rung
// for every channel, on every profile, and is refused there by the
// transport engine rather than by anything this package decided.
// TestWriteChannel_CurrentlyRefusedByCoreKWsOwnGate pins the CURRENT,
// fully reproducible outcome precisely so a future core/kw fix (filling
// the P9 span in BuildMWSet the way P2Unused already fills its own byte)
// breaks that test loudly rather than leaving a silently-still-broken
// write path uncaught. Capabilities still grades frequency, mode,
// scan_skip, tone_mode, tone_tx and tone_rx Sup/Sup (caps.go): that
// grading describes what the DOCUMENT'S wire shape supports, which this
// gap does not change, on the same convention the erase citation below
// keeps separate from what today's codec can build.
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
