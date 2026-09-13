// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx1200 is the Yaesu FTDX1200 driver: one row, bare New (matrix
// §5's two-package verdict against ftdx3000), built on an INLINE
// cat.DialectConfig (dialect.go) rather than a core/cat/ftdx1200
// subpackage.
//
// NO FTDX1200 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Every byte
// here came from the FTDX1200 CAT Operation Manual (revision 1507-E0)
// through docs/superpowers/ftdx1200-capability-matrix.md, and from
// nothing else.
//
// # This radio's shape, in one paragraph
//
// The SAME 27-byte/8-digit MW/MR frame family as the sibling ftdx3000,
// NoTag, no MT/combined-tag command, no 60m or EMG bank, nine PMS pairs
// numbered decimally 100-117. It differs from ftdx3000 in three cells
// beyond the CAT ID (matrix §5, the wave's own one-vs-two-package
// adjudication — verdict: TWO packages): the Mode domain has a genuine
// hole at 'A' (printed "----" on every command that carries it, live or
// stored) with no 'D' anywhere at all; the mode NAMING differs at
// overlapping codes (DATA-LSB/DATA-USB/RTTY-LSB/RTTY-USB vs ftdx3000's
// PKT-L/PKT-FM/PKT-U/FSK variants — a documented Yaesu model-line
// difference, not a project error); and P9 (the CTCSS tone index) is
// printed-fixed on BOTH read and write, not asymmetric — this radio needs
// no core/cat lift at all, unlike ftdx3000.
//
// # The option-split CAT ID
//
// This radio answers with ONE of TWO CAT IDs, "0582" (optional FFT-1
// board fitted) or "0583" (not fitted) — one product, not two models
// (matrix §1.6/§2.2: the FFT-1 option is mentioned nowhere else in the
// manual). ftdx1200.go's identify() accepts either directly; dialect.go's
// single dialect value carries "0582" as its canonical CATID (a CHOICE,
// the manual's first-listed variant) for capability-reporting purposes.
// This is a shape variation on the ftdx101 D/MP dual-catID precedent
// (core/driver/ftdx101/ftdx101.go:126-140) — that package's TWO
// constructors exist because the D and the MP are genuinely separate
// products; this package registers only ONE row and ONE constructor,
// since there is only one product here.
//
// # Register — decisions this matrix left open or this driver had to make
//
//  1. FieldCTCSSTone is the zero FieldSupport, unconditionally, on BOTH
//     profiles (caps.go): P9 is printed-fixed "00" on read AND write
//     (matrix §1.3) — there is no live tone state anywhere on this
//     radio's CAT surface to grade.
//
//  2. MWWriteKind is cat.KindVFO, NOT cat.KindMemory (dialect.go): the
//     same ft2000-family trap the sibling ftdx3000 also carries.
//
//  3. The Mode hole at 'A' (dialect.go's modeNames): simply absent from
//     the map, exactly like an absent D/E/F elsewhere in this fleet —
//     ParseMode/ValidMode refuse it without any special-case code.
//
//  4. MinFreqHz/MaxFreqHz: 30,000/60,000,000 Hz — the matrix (§2.11) left
//     this OPEN pending a re-read of the FA/FB detail blocks, which this
//     driver did directly (both print the identical range to the sibling
//     ftdx3000; independently confirmed, not copied).
//
//  5. MTPolicy is DECLARED BUT NEVER EXERCISED (dialect.go), for the same
//     reason as ftdx3000: no MT/tag command anywhere in this radio's
//     command index, but cat.DialectConfig.MT has no default.
//
//  6. Channel 000's MC-vs-MR/MW discrepancy (matrix §2.4, dialect.go):
//     the identical fleet-wide pattern ftdx3000 and ft2000 already carry.
package ftdx1200
