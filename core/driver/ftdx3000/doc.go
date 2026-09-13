// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx3000 is the Yaesu FTDX3000 driver: one row, bare New (matrix
// §5's two-package verdict against ftdx1200), built on an INLINE
// cat.DialectConfig (dialect.go) rather than a core/cat/ftdx3000
// subpackage — the v1.7.0 Kenwood/Yaesu wave brief's own instruction for
// this family.
//
// NO FTDX3000 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Every byte
// here came from the FTDX3000 CAT Operation Manual (revision 2006-D)
// through docs/superpowers/ftdx3000-capability-matrix.md, and from
// nothing else.
//
// # This radio's shape, in one paragraph
//
// 27-byte MR/MW frame (8-digit P2, matrix §1.1), NoTag, no MT/combined-tag
// command, no 60m or EMG bank, nine PMS pairs numbered decimally 100-117,
// Mode ceiling 'C' with AM-N (the manual's own 'D') ASSUMED-excluded from
// the write-capable enum because it is printed only on the live MD
// command, never on MW/MR. Its one genuinely novel feature in this fleet:
// P9 (the CTCSS tone-table index) is LIVE on read but printed-fixed on
// write — an asymmetry neither of core/cat's two existing MemoryP9Policy
// values could express, resolved by a small lift landed in the commit
// immediately before this driver's own (core/cat/dialectconfig.go's new
// P9ToneIndexReadOnly).
//
// # Register — decisions this matrix left open or this driver had to make
//
//  1. FieldCTCSSTone grading (caps.go, bankFields): `{Read: rw.Read,
//     Write: spec.Unsupported}`, UNCONDITIONALLY — the write side is a
//     structural ceiling (P9 is printed-fixed "00" on every MW Set this
//     radio's manual describes), not a profile-dependent Unverified/
//     Supported split like the other five mapped fields. Mirrors
//     core/driver/ic7410/caps.go's `scanFields` shape for the same kind
//     of asymmetric field.
//
//  2. MWWriteKind is cat.KindVFO, NOT cat.KindMemory (dialect.go): MW's P7
//     write-fixed byte is printed "0: (Fixed)" (layout:955), and the
//     ASCII character '0' is cat.KindVFO under memdata.go's own Kind byte
//     legend, not cat.KindMemory — the same trap ft2000/ftdx5000/
//     ftdx9000/ft950 all flag for this family.
//
//  3. MTPolicy is DECLARED BUT NEVER EXERCISED (dialect.go): this family
//     has no MT/tag command anywhere in its command index, but
//     cat.DialectConfig.MT has no default and NewDialect's V9 refuses the
//     zero value unconditionally. The values declared are the minimum
//     that satisfies construction; read.go/write.go never call
//     BuildMTRead/BuildMTSet/ParseMTAnswer.
//
//  4. AM-N excluded from the write-capable Mode enum (dialect.go's
//     modeNames): ASSUMED, per the addendum correcting the plan's
//     original 'D'-ceiling assumption — the memory record's own legend
//     (MW/MR) stops at 'C'; 'D' exists only on the live MD command.
//
//  5. Channel 000's MC-vs-MR/MW discrepancy (matrix §2.4, dialect.go):
//     MC's own P1 legend spans "000-117" as one line, but MR/MW's own P1
//     legends print "(001 117)", excluding 000. This driver follows
//     MR/MW (the verbs its banks are built from) and registers
//     MemoryLo: 1; channel 000's status is left an open erratum, the same
//     fleet-wide pattern ft2000 already carries.
package ftdx3000
