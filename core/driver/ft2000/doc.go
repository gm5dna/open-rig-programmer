// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft2000 is the Yaesu FT-2000/FT-2000D driver: two rows over one
// package, NewFT2000/NewFT2000D (no bare New — matrix §4), built on an
// INLINE cat.DialectConfig (dialect.go) rather than a core/cat/ft2000
// subpackage — the v1.7.0 Kenwood/Yaesu wave brief's own instruction for
// this family, since the two rows share one dialect variant differing
// only in Model name and CATID.
//
// NO FT-2000 OR FT-2000D HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT.
// Every byte here came from the FT-2000 SERIES CAT OPERATION REFERENCE
// BOOK (revision EH025H124) through
// docs/superpowers/ft2000-capability-matrix.md and this driver's own
// phase-3 findings, cited below, and from nothing else.
//
// # This radio's shape, in one paragraph
//
// 27-byte MR/MW frame (one digit narrower than every registered
// sibling's 28: an 8-digit P2, matrix §1.1), NoTag (no channel-name route
// over CAT at all, matrix §0), no MT/combined-tag command of any kind, no
// 60m or EMG bank, nine PMS pairs numbered decimally 100-117
// (cat.PMSFormNumeric), and a live P9 CTCSS tone-table index — the first
// memory record in this fleet to carry one (matrix §1.3).
//
// # Register — decisions this matrix left open or a Phase-3 driver must
// make, reconciled here
//
//  1. FieldCTCSSTone grading (caps.go, bankFields). Matrix §3's single
//     largest departure from a normal "every field graded" close: P9 is a
//     live two-digit index into the standard 50-entry CTCSS tone chart,
//     round-tripped by lift Y's MemoryP9Policy/ToneIndex axis, but no
//     registered dialect had ever populated it before this package. This
//     driver grades it Supported-shape (rw) alongside the other five
//     MW/MR-mapped fields: the codec fully expresses it in both
//     directions (read.go, write.go), and nothing about the FT-991A's own
//     zero grading of FieldCTCSSTone applies here (matrix §2.9's own
//     warning against copying that precedent without re-deriving it —
//     that radio's P9 does not exist at all; this radio's P8 is the
//     ordinary three-state domain, not that radio's DCS-bearing one).
//
//  2. MTPolicy is DECLARED BUT NEVER EXERCISED (dialect.go). This family
//     has no MT/tag command anywhere in its 90-command index (a
//     whole-document grep of the layout extraction finds none) — but
//     cat.DialectConfig.MT has no default, and NewDialect's V9 refuses
//     the zero value unconditionally, radio-has-no-MT or not. The values
//     declared (MTFormShort, TagMaxBytes 1, ClearTagByte ' ') are the
//     minimum that satisfies construction and describe nothing this
//     radio's manual prints; this driver's read.go/write.go never call
//     BuildMTRead/BuildMTSet/ParseMTAnswer. Same shape as lift K's
//     newStreamError gap for ts570/ts870s: a declared-but-unreachable
//     path, not a fabricated citation.
//
//  3. MWWriteKind is cat.KindVFO, NOT cat.KindMemory (dialect.go). MW's P7
//     write-fixed byte is printed "0: (Fixed)" (layout:934), and matrix
//     §1.4 flags the trap directly: the ASCII character '0' is
//     cat.KindVFO under memdata.go's own Kind byte legend ("0 VFO, 1
//     Memory"), not cat.KindMemory. A driver built from the matrix
//     without re-checking this constant would silently write KindVFO —
//     nonetheless the byte the manual actually prints — into every
//     memory-channel record, which is in fact correct; the trap is in
//     assuming the OTHER constant's name matches the OTHER radios'
//     convention (every registered sibling's MW write-fixes KindMemory
//     instead) and reading '0' as though it must mean the same thing here.
//
//  4. MinFreqHz/MaxFreqHz: 30,000/60,000,000 Hz, MANUAL-EVIDENCED (caps.go),
//     a Phase-3 finding beyond the matrix. Matrix §2.11 left this pair
//     explicitly UNSET pending a re-read of the FA/FB detail blocks
//     (PDF pp.9-10), which this driver did:
//     "P1 00030000 - 60000000 (Hz)", printed identically on FA's and FB's
//     own Set legends (ft2000_layout.txt:654, :666). The range also
//     closes a latent gap in core/cat's own write validator: mw.go's
//     validateSetFields bounds FreqHz against the REGISTERED (9-digit)
//     family's memFreqMax, one digit wider than this family's 8-digit P2
//     field, so a value between this radio's own ceiling and that
//     9-digit one would pass core/cat's check and then overflow
//     encodeMemoryFields' fixed-width rendering. write.go's
//     buildMWCommand checks this radio's own caps.MinFreqHz/MaxFreqHz
//     first, which is what keeps that gap unreachable from this driver.
//
//  5. ClarMaxHz is 9990, not the matrix's literal 9999 (dialect.go,
//     caps.go). Matrix §2.7 states 9999 needs "no step assumption ...
//     for the CEILING" — true of the printed range alone, but
//     cat.NewDialect's V10 rule requires ClarifierPolicy.MaxAbsHz to be
//     an exact multiple of StepHz, and 9999 is not a multiple of the
//     ASSUMED 10 Hz step. 9990 (the FT-991A's own precedent for exactly
//     this shape) is the largest multiple of 10 inside the printed
//     0000-9999 range.
//
//  6. D/E/F absent from the mode table (dialect.go's modeNames, caps.go's
//     modeNames()). Matrix §1.2: this radio's mode legend covers '1'-'C'
//     (12 of core/cat's 15 named values) and prints no 'D', 'E' or 'F'
//     row at all — cat.ModeAMN, cat.ModePSK and cat.ModeDATAFMN are
//     simply not members of this dialect's ModeNames map, so
//     ValidMode/ParseMode refuse them and modeNames() never advertises
//     them. No spec.Field is affected: FieldMode itself is graded
//     Supported (bankFields), and this is a MODE-DOMAIN narrowness within
//     that field, not an unmapped field the audit test needs a separate
//     reason for.
//
//  7. The lift-K stream-error gap (core/kw/errors.go's newStreamError) is
//     N/A to this package: it binds Book570/Book870S only, and this is a
//     Yaesu package with no core/kw involvement at all.
//
//  8. core/cat/dialecttest.Run's checkToneStateDomain sub-check does NOT
//     conform for this family (dialect_test.go's
//     TestDialectConformance_KnownGap_CTCSSOffsetNotFrameWidthAware
//     documents and demonstrates this in full) — a SHARED-INFRASTRUCTURE
//     bug this driver found, not a defect in this dialect or its gate.
//     dialecttest.go:751's ctcssOffsetInMemoryFrame is hard-coded 23,
//     P8's offset in the REGISTERED family's 28-byte/9-digit-P2 frame;
//     this family's 27-byte/8-digit-P2 frame puts P8 at offset 22
//     instead, so the suite's forged byte lands on the FIRST DIGIT OF
//     P9 (this family's own live tone index) rather than on P8 at all,
//     and the gate correctly admits the resulting well-formed frame.
//     core/cat/dialecttest is out of this brief's scope to fix (another
//     package, concurrently touched by six other in-flight packages);
//     the driver's own supplementary test proves the gate DOES refuse a
//     DCS byte at this dialect's real P8 offset, which is the fact the
//     suite's own broken forgery fails to establish. dialecttest.Run
//     itself therefore reports FAIL for both dialects on this one
//     sub-check alone — every other category of the suite (frame
//     shape, mode space, slot space, MC/MT domains, memory writes, the
//     five-state/three-state split's OTHER assertions, EX, non-vacuity)
//     passes clean.
package ft2000
