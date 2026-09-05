// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts480 holds the TS-480 half of the Kenwood codec: its layout
// value and its menu inventory.
//
// THE ROW IS BUILT AND NOT REGISTERED at this milestone's close. A4 — that
// an MR of an empty channel ANSWERS rather than rejecting on this radio —
// is documented for the 590 pair (590:1492-1493) and entirely unprinted for
// the 480, and if it is false a fresh TS-480 cannot be read at all. Its
// lift, L-HW-3, is hardware confirmation item 3, and it is a GATE rather
// than a priority: everything else on the list buys capability, item 3 buys
// shippability. Until it lifts, this package exists, is tested, and is
// absent from internal/wiring's tables and from SupportedModels().
//
// The codec itself — framing, the accumulator, the matcher, the EX types
// and the typed error family — is core/kw's. This package carries only what
// is this radio's. It never imports core/cat or core/civ
// (core/kw/imports_test.go's fence covers this directory too).
//
// PROVENANCE STUB. Task 8 fills this comment with the LAYOUT AXES and their
// citations — byte 19 lockout (480:962), byte 28 always 0 (480:973), bytes
// 39-40 step size per ST (480:979), byte 41 always 0 (480:982), the
// three-value tone-mode legend with no cross tone (480:964), the mode
// legend and its CWR/FSR spellings (480:852-854), the sixteen hard-wired
// bytes and A24's parse-side strictness (480:108-110) — together with this
// package's share of the errata schedule (E8, E9, E10, E11, E12, E14, E15,
// E17, E21 are 480-book entries, and E18 is the ANTI-defect: "Se t" is
// PageMaker letter-spacing, not a document error, recorded so no
// transcriber "corrects" it into evidence). The ASSUMED register lives
// once, in core/kw/doc.go, and is not restated here.
package ts480
