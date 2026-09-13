// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft450d drives the Yaesu FT-450D over CAT: ONE registered row,
// "FT-450D", bare New (own document, no sibling row — matrix header), built
// on an INLINE cat.DialectConfig (dialect.go) rather than a
// core/cat/ft450d subpackage — the v1.7.0 Kenwood/Yaesu wave brief's own
// instruction, applied here to a sixth single-row Yaesu package. Source:
// docs/superpowers/ft450d-capability-matrix.md, built from the FT-450D CAT
// Operation Reference Book (revision 1710-B) and Operating Manual
// (revision 1901L-LS-1).
//
// NO FT-450D HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE IS
// AVAILABLE TO IT (matrix header). Every byte here came from those two
// manuals through the matrix, and from nothing else.
//
// # This radio's shape, in one paragraph
//
// 27-byte MR/MW frame, 8-digit P2 (matrix §1.1, identical shape to
// ft2000/ftdx5000/ftdx9000/ft950), NoTag (a front-panel-only 7-character
// tag with no CAT text route — matrix §0/§2.7, distinct from the ft2000
// family's total absence of a name field), no MT/combined-tag command
// anywhere in the 90-command index, a live P9 CTCSS tone-table index, and
// TWO STATIC BANKS WITH GENUINELY DIFFERENT WRITE POSTURE: MEM (001-500,
// read/write) and PMS (501-504, read-only per the roadmap's SAFE SHAPE
// ruling) — the one shape this package does NOT share with its ft2000/
// ft991a precedents, which both put MEM and PMS behind one Fields map.
// 505-510 (60 m + Alaska Emergency, real channels the OM names but the CAT
// book never addresses) is not in the dialect at all.
//
// # Register — decisions this matrix left open or this driver phase makes,
// reconciled here
//
//  1. The PMS bank's own Fields map (caps.go, pmsFields) is the shape this
//     matrix's own §3 explicitly forbids sharing with MEM: unlike
//     ft991a/ft2000 (one bankFields map for both banks, because nothing
//     distinguishes them field by field), this radio's SAFE SHAPE ruling
//     (radio-roadmap.md:80-87) makes PMS's six shared fields Write:
//     Unsupported UNCONDITIONALLY — on both CapabilitiesUnverified and
//     CapabilitiesSimulated, and immune to
//     WithConsentedUnverifiedWrites — while MEM's identical six stay the
//     ordinary profile-dependent rw. This is a codec-level policy
//     position ("this project's caution about writing into a scan-limit
//     pair blind", matrix §4), not a hardware-unverified state consent
//     could open. The matrix names the exact lift that flips it: "a
//     one-line capability flip" once an owner probe (community-hints.md
//     probe #3/#4) shows "MW501...;" succeeding on real hardware.
//
//  2. MTPolicy is DECLARED BUT NEVER EXERCISED (dialect.go). This radio's
//     90-command index has no MT row at all (matrix §0: the alphabetical
//     jump MC->MD->MG->MK->ML->MR->MS->MW), but cat.DialectConfig.MT has
//     no default and NewDialect's V9 refuses the zero value
//     unconditionally. The values declared (MTFormShort, TagMaxBytes 1,
//     ClearTagByte ' ') satisfy construction only; read.go/write.go never
//     call BuildMTRead/BuildMTSet/ParseMTAnswer. The same gap every
//     no-MT Yaesu package in this fleet hits independently
//     (ft2000/ftdx5000/ftdx9000/ft950's own doc.go entries say so).
//
//  3. MWWriteKind is cat.KindVFO, NOT cat.KindMemory (dialect.go). MW's P7
//     is printed "0: Fixed" (layout:798), and the ASCII character '0' is
//     cat.KindVFO under memdata.go's own Kind byte legend ("0 VFO,
//     1 Memory") — the same naming trap every registered Yaesu sibling's
//     matrix flags, re-confirmed independently against this radio's own
//     manual (matrix §1.4).
//
//  4. MinFreqHz/MaxFreqHz: 30,000/60,000,000 Hz (caps.go), a Phase-2
//     finding beyond the matrix, which left this pair explicitly OPEN
//     (§2.11) rather than guessed — following ft2000's own precedent for
//     the identical gap. FA's own Set legend prints "P1 30000 - 60000000
//     (Hz)" (ft450d_layout.txt:555); FB's own line prints "300000" (an
//     apparent extra digit, ft450d_layout.txt:568) — FA's range is taken,
//     matching every sibling package's own choice when the two commands'
//     printed ranges disagree by a transcription-plausible margin.
//
//  5. RequiredSlots is left EMPTY, matching the matrix's own conservative
//     call (§2.12): PMS's NoBlank: true already expresses "these four
//     channels are never blank" at the bank level, so naming individual
//     MEM slots would be an invented constraint this manual does not
//     state.
//
//  6. Mode display names keep the manual's own spelling ("DATA (RTTY-LSB)",
//     "USER-L"/"USER-U", not core/cat's canonical "RTTY-LSB"/"DATA-L"/
//     "DATA-U") — the ft950/ftdx9000 precedent in the v1.7.0 wave, not the
//     ft2000 one (which substitutes core/cat's canonical words). Eleven
//     nibbles, '1'-'9','B','C'; 'A' is a clean hole (no legend anywhere
//     prints it) and 'D'/'E'/'F' are simply absent (matrix §1.2).
//
// # Not this package's job
//
// EX/menu inventory (spec.md §2, out for all three v1.8.0 packages):
// EXItems is empty and no settings.go exists here — menu row "010 CATRATE"
// (layout:401) and EX036 (layout:427) are cited only to support
// Bauds/DefaultBaud and the NoTag finding, never transcribed as an EX
// inventory entry. internal/fakeft450d (a different phase, disjoint
// files). Registration — internal/wiring, radiotext.go, app/uispec.go,
// README.md, docs/*.md, CHANGELOG.md — is Phase 4's job. The six bench
// probes that would settle 505-510/the write-ceiling promotion
// (community-hints.md) are named, not run, here.
//
// # Field audit
//
// TestFieldAuditCoversEverySpecField (caps_test.go) pins that every one of
// spec.AllFields()'s twenty-seven members is explicitly named in memFields
// (caps.go) — mapped (six: frequency, mode, clarifier, ctcss state, ctcss
// tone, shift) or explicitly zeroed with a one-line reason at that map
// entry (tag/tag_display: NoTag, this radio's tag route is front-panel
// only; scan_skip and erase: no such byte/command exists on this radio's
// 27-byte record or in its 90-command index; the seventeen Icom-tier
// fields: manual-evidenced absences, this being a Yaesu-family record).
// pmsFields shares the identical key set (register entry 1) and needs no
// separate audit.
package ft450d
