// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx9000 drives the Yaesu FTdx9000 over CAT: ONE registered row,
// "FTdx9000", built from
// docs/superpowers/ftdx9000-capability-matrix.md (v1.7.0 Kenwood/Yaesu
// wave, part 2, Phase 3). Source: manual EH010H121, "FT DX 9000 SERIES CAT
// OPERATION REFERENCE BOOK" — one document covering the FTDX9000D,
// FTDX9000Contest and FTDX9000MP sub-variants and, per this project's own
// spec (settled answer 5), the "FT-9000" trade name too.
//
// # Register — decisions this package makes that the matrix leaves open
//
// FT-9000 IS NOT A MANUAL STRING. The matrix's own full-document grep for
// the literal "FT-9000" returns zero hits (matrix §1.1): the document
// prints "FTDX9000D/Contest/MP" (ID legend) and "FT DX 9000" (spaced,
// running headers). "FT-9000" remains the settled radiotext-only alias
// (Phase 4's job), never cited here as a manual string.
//
// THREE CAT IDS, ONE ROW. The ID legend prints three four-digit answers —
// 0101 (D), 0102 (Contest), 0103 (MP) — for what this project registers as
// a single radio (matrix §1.2, explicitly left open: "not one of the six
// required pins"). dialect.go's CATID is "0101" (this dialect's own
// canonical identity, used by Capabilities().CATID and Identity.CATID);
// ftdx9000.go's handshake (NOT core/driver/internal/yaesu.Handshake, which
// takes exactly one ID) accepts all three at the ID probe. A driver that
// only ever accepted 0101 would refuse two of the three real radios this
// project claims to register.
//
// NO MT COMMAND IS BUILT, ANYWHERE IN THIS PACKAGE. The matrix's whole §2
// record is MR/MW; no MT (combined memory+tag) command is cited for this
// radio at all — the family's tag-carrying siblings (FT-891, FT-991A,
// FTdx10, FTdx101) all own one, this radio's manual does not. cat.Dialect
// still requires a valid MTPolicy to construct at all (DialectConfig has
// no zero-value default for it), so dialect.go configures the smallest
// legal MTFormShort and NEVER EXERCISES it: read.go and write.go call
// BuildMRRead/ParseMRAnswer/BuildMWSet directly, never a BuildMTSet*/
// ParseMTAnswer* function, and never core/driver/internal/yaesu.
// WriteChannel/ReadChannel-shaped helpers, both of which build only the
// combined MT form. This is this package's largest deviation from the
// four registered Yaesu drivers' shape.
//
// FieldCTCSSTone IS MAPPED, not left zero. Lift Y's P9ToneIndex axis
// (matrix §1.10) makes this radio's P9 a live index into the SAME standard
// 50-tone chart every registered dialect shares; spec.md's own framing
// leaves whether to grade the field explicitly to the driver. Grading it
// Unsupported would misdescribe a byte that genuinely round-trips.
// Consequence: unlike every sibling driver's optional CTCSSTone,
// FieldCTCSSTone is UNCONDITIONALLY requested on every write here (P9 is
// mandatory on the wire, like the tag-display flag on FT-710/FT-891), and
// a channel whose CTCSSTone FieldState is not Known is refused before any
// frame is built (write.go).
//
// MODE-SPELLING DIFFERENCE, NOT OVERRIDDEN. Six of the twelve mode nibbles
// print a different word here than core/cat's canonical fallback spells
// for the SAME nibble ('3' "CW" vs "CW-U", '6'/'9' "FSK.../FSK-R..." vs
// "RTTY-LSB"/"RTTY-USB", '7' "CW-R" vs "CW-L", '8'/'A'/'C' "PKT-..." vs
// "DATA-..."). Matrix §1.5 flags this as a CHOICE, not settled: dialect.go
// transcribes this manual's own words (ModeName renders them, never
// core/cat's fallback), and no per-model ModeName override table is built,
// because — unlike the FT-991A's 'E' — no nibble here disagrees about
// WHICH mode it is, only its display word.
//
// KindVFO, not KindMemory. P7's write-fixed byte is '0', which mode.go
// names KindVFO — the naming trap the brief flags for this Yaesu family;
// MWWriteKind is cat.KindVFO, not cat.KindMemory (the FT-710/FT-891/
// FT-991A value).
//
// RequiredSlots is left EMPTY (matrix §1.14 is an open note, not a pin):
// following the FT-991A's own counter-argument, a wrong guess here refuses
// real candidates rather than merely describing one.
//
// # Not this package's job
//
// EX/menu inventory (spec.md §3, out for all seven packages in this
// wave): EXItems is empty and MaxEXAddress is unset-refused — no
// settings.go exists here. internal/fakeftdx9000 (Phase 3b, disjoint
// files). Registration — internal/wiring, radiotext.go, app/uispec.go,
// README.md, docs/*.md, CHANGELOG.md — is Phase 4's job.
//
// # Field audit
//
// TestFieldAuditCoversEverySpecField (caps_test.go) pins that every one of
// spec.AllFields()'s twenty-seven members is explicitly named in
// bankFields (caps.go), whether mapped (six: frequency, mode, clarifier,
// ctcss state, ctcss tone, shift) or explicitly zeroed with a one-line
// reason at that map entry (tag/tag_display: NoTag per the 12/09/2026
// nameless-capability rule; scan_skip and erase: no such byte/command
// exists on this radio's 27-byte record or in spec.md §3's write posture;
// the seventeen Icom-tier fields: matrix §1.17's manual-evidenced
// absences).
package ftdx9000
