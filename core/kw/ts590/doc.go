// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts590 holds the TS-590S and TS-590SG halves of the Kenwood codec:
// the two layout values and the two menu inventories.
//
// TWO REGISTRY ROWS, ONE PACKAGE, AND NEVER ONE RADIO. Kenwood prints the
// S and the SG in one document, which is a property of the book and not of
// the radios: they have different firmware, different menu domains (88 rows
// against 100, A26) and a byte whose liveness differs between them (byte 28
// / P11, A14). Nothing in this package may say "a TS-590" — the ASSUMED
// register's standing rule from draft 5, and the reason both inventories
// and both layout values are per-row rather than shared.
//
// The codec itself — framing, the accumulator, the matcher, the EX types
// and the typed error family — is core/kw's. This package carries only what
// differs between the two rows. It never imports core/cat or core/civ
// (core/kw/imports_test.go's fence covers this directory too).
//
// PROVENANCE STUB. Task 8 fills this comment with the per-row LAYOUT AXES
// and their citations — byte 19 data mode per DA (590:1546-1548), byte 28
// filter A/B (590:1560-1563) and its firmware condition (590:1564), bytes
// 39-40 FM Normal/Narrow (590:1569-1571), byte 41 lockout (590:1572-1574),
// the mode legend and its spellings (590:1353-1363), the slot space and its
// space convention (590:1332-1347) — together with this package's share of
// the errata schedule (E1, E2, E4, E5, E7, E16, E19, E20 are 590-book
// entries; E6 is the SS/MC numbering trap a transcriber must not carry into
// an MC frame). The ASSUMED register lives once, in core/kw/doc.go, and is
// not restated here.
package ts590
