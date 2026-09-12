// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts570 holds the TS-570D, TS-570S and TS-570DG halves of the
// Kenwood codec: three layout values sharing one 28-byte prefix of the
// family's 50-byte MR/MW grid.
//
// THREE ROWS, ONE DOCUMENT, ONE PACKAGE. The TS-570 Instruction Manual
// (document B62-1542-00, ts570-capability-matrix.md §0) names TS-570D and
// TS-570S on its own cover and body text; TS-570DG appears nowhere in the
// document at all and is carried here only because third-party catalogues
// and Kenwood's own current product page group "TS-570D/S(G)" under the
// same document code (matrix §5). NOTHING IN THIS PACKAGE MAY SAY "a
// TS-570" — the same standing rule core/kw/ts590's doc comment states for
// its own two rows.
//
// TS-570DG IS UNVERIFIED-BY-INHERITANCE, and that label is carried
// verbatim rather than softened wherever this row's own facts are named:
// its Model string, its CATID (which is additionally ASSUMED — no document
// prints one) and every other value it shares with its D/S siblings. There
// is no independent reading behind any DG cell; LayoutDG is D's and S's
// config with only the Model changed.
//
// The codec itself — framing, the accumulator, the matcher, the EX types
// and the typed error family — is core/kw's. This package carries only
// what is this row's. It never imports core/cat or core/civ
// (core/kw/imports_test.go's fence covers this directory too).
//
// # A true 28-byte PREFIX, not a second grid
//
// Positions 1-22 of this row's MR/MW record sit at the SAME offsets as the
// family's 50-byte grid for the same seven parameters (prefix, P1, P3, P4,
// P5, byte 19, P8) — matrix §1.4, settled by citation rather than by the
// spec's default. The record simply STOPS after position 27 (P9, unused)
// with the terminator at 28: there is no byte 28 (P11), no bytes 39-40
// (P14), no byte 41 (P15) and no P10/P12/P13 tail at all. Lift K's
// RecordLen axis (core/kw/layout.go) exists for exactly this shape:
// RecordLen: 28 here, and NewLayout refuses every tail axis (Byte28,
// Byte3940, Byte41, P10, P12, P13) rather than admitting a partial one.
//
// # The two axes this row is why they exist
//
//	byte 4 (P2)  UNUSED. Neither a hundreds digit (the 590 pair's
//	             P2HundredsDigit) nor a printed "0" (the TS-480's
//	             P2FixedZero): the Parameter Table prints a dash, no digit
//	             count at all (matrix §1.2, Format entry for byte 4). The
//	             P2Unused axis value exists for this row alone. The slot
//	             space stops at 99 in consequence (P3's two digits are the
//	             whole channel number) — the same ceiling P2FixedZero
//	             carries, for the same reason.
//	byte 20 (P7) TWO tone values, "0: OFF / 1: ON" (matrix §1.2, Format 1)
//	             — narrower than the 590 pair's four and the TS-480's
//	             three. The ToneModesTwo axis value exists for this row
//	             alone.
//
// Every other axis reuses existing vocabulary unchanged: Byte19Lockout
// (byte 19 is the channel lockout here, as on the TS-480 — matrix §1.2
// Format 10), and core/kw.Mode's ten nibbles in full — the Parameter
// Table's Format 2 legend is byte-for-byte the family's own eight named
// modes plus the two that name none (matrix §1.3), so this is the one
// field in the whole record needing zero new core/kw vocabulary.
//
// # One flat bank, three rows deep
//
// "MEM", 000-099, no bank digit and no group byte (matrix §2 Banks): P2
// carries nothing (above), so there is no hundreds digit and the slot
// space stops at 99, exactly as maxFixedZeroSlot bounds a P2FixedZero row.
// Channels 90-99 answer the same Start/End (P1=0/1) scan-edge overload the
// TS-480 documents (matrix §2, citing decision 15) — ORDINARY memories
// that also answer a second frame, not a second SlotScan bank. All three
// rows declare the identical space.
//
// # The two named gaps this package inherits and routes around
//
// Both are Lift K's, not this package's, and neither is fixed here — see
// core/driver/ts570's own doc comment and reviews/driver-ts570.md for the
// routing:
//
//   - core/kw/errors.go's newStreamError has no transcribed "E;"/"O;"
//     citation for Book570 (S2/S3's evidence stops at the command-table
//     pages) and panics if ever asked for one; this row's driver never
//     reaches a live session that could ask it.
//   - core/kw/kwtest's conformance suite hardcodes the family's full
//     50-byte RecordLen in two checks (checkMemorySets' width assertion
//     and checkGateRefusesAMutatedPrintedFixedByte's non-empty
//     PrintedFixed requirement) and was not updated for Lift K's RecordLen
//     axis in either place; both fail unconditionally against this row's
//     genuinely 28-byte, no-tail layout regardless of this package's own
//     correctness. Documented and skipped in layout_test.go, not
//     silenced.
//
// # EX/menu inventory is out of scope
//
// This wave reads memory channels only (spec.md, Phase 3 brief). This
// package sets MaxEXAddress (51, the Parameter Table's own Format 35 "MENU
// NUMBER … Represented using 000~051", matrix's own re-read of PDF p.79
// printed 73) because kw.NewLayout refuses an unset one unconditionally —
// it is the printed DOMAIN, a bound every row must carry regardless of
// scope, not the per-address INVENTORY (names, widths) core/kw/ts590's
// exinventory files build. No EXItem list is built for this row; no
// core/driver/ts570 settings surface reads or writes a menu.
//
// # The CTCSS table's 39th entry
//
// Recorded, not resolved (matrix §2, register ts570-tone39-burst-not-ctcss):
// this row's subtone table's index 39 is a 1750 Hz European burst tone, not
// a CTCSS sub-audible tone, per the manual's own note. core/driver/ts570
// carries it as an ordinary 39th chart entry, the technically-accurate-to-
// the-wire reading, and says so at the constant.
package ts570
