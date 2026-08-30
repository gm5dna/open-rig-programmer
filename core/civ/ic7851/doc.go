// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7851 contains the independently evidenced IC-7851/IC-7850 CI-V
// dialect: one validated flat-address profile and the frozen evidence it
// was derived from. The driver next door owns probing, capabilities and
// write policy.
//
// # The document
//
// There is NO standalone CI-V reference guide for this model. The
// authority is the IC-7850/IC-7851 Instruction Manual, Revision 3,
// document code A7205H-1EX-3, 283 PDF pages; its CI-V material is Section
// 18, "CONTROL COMMAND", PDF pages 250-265 (printed folios 18-1 to 18-16),
// and the memory record is PDF p.263 (folio 18-14), "• Memory content
// setting / Command: 1A 00". The radio's printed default CI-V address is
// 8E and the controller's is E0, so every frame this package builds is
// FE FE 8E E0 ... FD.
//
// The reading is the IC-7851 capability matrix trio under
// docs/superpowers/icom-matrices/, and the three independently transcribed
// evidence legs frozen in testdata: a field ledger (printed index and
// label), a geometry witness (measured drawn-cell and nibble bounds) and a
// transcription (printed width, encoding and values), plus the golden
// vectors and their per-byte provenance table.
//
// THE ARTEFACTS ARE FROZEN. freeze_test.go carries a SHA-256 for each and
// refuses an unmanifested file; when a test disagrees with an artefact the
// fix is arbitration against the PDF, never an edit to the artefact.
//
// # The IC-7851 and the IC-7850 share this profile
//
// The two models share one manual, one address and one frame shape, and
// the 19 00 reply value is undocumented for both — so they are
// INDISTINGUISHABLE by this programme's admitted evidence and probe. There
// is one profile here and two constructors in the driver; the user picks
// the row. See the driver's doc.go §1.
//
// # The civ-homed register entries
//
// Three of the matrix's nineteen assumption entries are homed in this
// package, each with exactly ONE lift on an IC-7851 itself:
//
//   - ic7851-record-length — that the printed widths derive to 25
//     record-only and 27 data-area bytes. LIFT: read M-CH01 and count the
//     bytes between the sub-command and FD.
//   - ic7851-fixed-nibble-reencode — that frequency byte ⑧'s two nibbles
//     are the printed fixed zeros they are drawn as. LIFT: read any
//     programmed channel and record byte ⑧ to confirm it is 00.
//   - ic7851-tone-fixed-byte — the same for the leading byte of each tone
//     triple. LIFT: set M-CH04's repeater tone to 88.5 Hz, read back, and
//     record bytes ⑫⑬⑭ (expected 00 08 85).
//
// Two more assumptions are exercised here and homed in the driver's
// register, because their lifts are captures against a radio rather than
// readings of the record: ic7851-name-pad-byte (the 0x20 pad) and
// ic7851-name-digit-space-codes (the digit and space code values, which
// PDF p.261's two code tables do not print). Both are pinned as bytes by
// TestNameDigitAndSpaceCodesRoundTrip so the lifts have something exact to
// contradict.
//
// The remaining fourteen are the driver's; see its doc.go for the full
// nineteen in one table.
package ic7851
