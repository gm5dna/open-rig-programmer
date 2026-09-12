// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft950 drives the Yaesu FT-950 over CAT: ONE registered row,
// "FT-950", bare New (no multi-row wrapper — spec.md §1), built on an
// INLINE cat.DialectConfig (dialect.go) rather than a core/cat/ft950
// subpackage — the v1.7.0 Kenwood/Yaesu wave brief's own instruction for
// this Yaesu family. Source:
// docs/superpowers/ft950-capability-matrix.md, built from the Yaesu FT-950
// CAT Operation Reference Book (internal doc code EC030H120).
//
// NO FT-950 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NONE IS
// AVAILABLE TO IT (matrix header). Every byte here came from that manual
// through the matrix, and from nothing else.
//
// # This radio's shape, in one paragraph
//
// 27-byte MR/MW frame (one digit narrower than every registered Yaesu
// sibling's 28: an 8-digit P2, matrix §1.1/§1.2), NoTag (no channel-name
// route over CAT at all, matrix §0), no MT/combined-tag command anywhere
// in the 90-command index, no 60m or EMG bank, nine PMS pairs numbered
// decimally 100-117 (cat.PMSFormNumeric), a live P9 CTCSS tone-table
// index, and its own delta from every sibling in this wave: MemoryLo is
// 000, not 001 (matrix §1.5) — one more regular channel than
// FT-2000/FTdx5000/FTdx9000 document.
//
// # Register — decisions this matrix left open or a Phase-3 driver must
// make, reconciled here
//
//  1. FieldCTCSSTone grading (caps.go, bankFields) is MAPPED, and OPTIONAL
//     on write — the ft2000 shape, not the FTdx9000 one. Matrix §1.4/§4
//     leaves the grading choice to the driver; the codec fully expresses
//     P9 in both directions (lift Y's MemoryP9Policy/ToneIndex axis), so
//     Unsupported would misdescribe a live byte. Unlike FTdx9000 (which
//     refuses any write whose CTCSSTone is not Known, because it treats
//     P9 as mandatory), this driver follows FT-2000's precedent: a
//     non-Known CTCSSTone defaults ToneIndex to 0 on write — a real,
//     representable chart entry (index 0, the FT-2000/ft950 shared 50-tone
//     chart's first row), not a placeholder — so an ordinary write that
//     never mentions a tone is not refused. Both readings are defensible;
//     this is this package's own choice, named so a later reader does not
//     mistake it for the matrix's.
//
//  2. MTPolicy is DECLARED BUT NEVER EXERCISED (dialect.go). This radio's
//     Control Command List (layout:118-181) has no MT row at all — a
//     whole-document grep for "MT" as a command header finds none — but
//     cat.DialectConfig.MT has no default, and NewDialect's V9 refuses the
//     zero value unconditionally, radio-has-no-MT or not. The values
//     declared (MTFormShort, TagMaxBytes 1, ClearTagByte ' ') are the
//     minimum that satisfies construction and describe nothing this
//     radio's manual prints; read.go/write.go never call
//     BuildMTRead/BuildMTSet/ParseMTAnswer. Same shape as lift K's
//     newStreamError gap for ts570/ts870s, and the identical gap the
//     ft2000/ftdx5000/ftdx9000 packages in this same wave each hit and
//     route around independently (their own doc.go entries say so) — a
//     codec-level gap shared by every no-MT Yaesu package this wave adds,
//     not fixable here since core/cat is out of this package's scope.
//
//  3. MWWriteKind is cat.KindVFO, NOT cat.KindMemory (dialect.go). MW's P7
//     is printed "0: (Fixed)" (layout:895), and the ASCII character '0' is
//     cat.KindVFO under memdata.go's own Kind byte legend ("0 VFO,
//     1 Memory") — the same naming trap the brief flags for this whole
//     Yaesu family (every registered sibling's MW write-fixes KindMemory
//     instead; this one's printed byte happens to be the OTHER constant's
//     spelling).
//
//  4. MemoryLo is 000, not 001 — THIS RADIO'S OWN DELTA FROM ITS SIBLINGS
//     (matrix §1.5, dialect.go, caps.go). The MC legend's own first line
//     reads "000 - 099: Regular Memory Channel" (layout:797), one more
//     regular channel than FT-2000/FTdx5000/FTdx9000 document.
//     dialectvalidate.go's own comment states MemoryLo == 0 is PERMITTED
//     ("a radio numbering its channels from 000 is legitimate"), so this
//     needs no new validation rule, only Slots.MemoryLo = 0 and
//     Slots.NoneWire = "" (NOT "000" — that string is a real channel on
//     this radio, and V7's shadowing rule would refuse a NoneWire that
//     collided with it). caps.go's memSlots() walks from n=0, not n=1 —
//     the one place a sibling's template cannot be copied verbatim.
//
//  5. MinFreqHz/MaxFreqHz: 30,000/56,000,000 Hz, a Phase-3 finding beyond
//     the matrix (caps.go), which is silent on this pair entirely. FA's
//     and FB's own Set legends both print "0030000 - 56000000 (Hz)"
//     (layout:606, :618) — the VFO tuning domain read as the
//     memory-storable domain, the same two-part grading (numbers
//     MANUAL-EVIDENCED, the VFO-to-memory step ASSUMED) every sibling in
//     this wave carries on identical grounds.
//
//  6. RequiredSlots is left EMPTY, not the matrix's flagged {"000"}
//     (caps.go). Matrix §4 explicitly declines to resolve whether this
//     radio needs a RequiredSlots entry at all, only what its slot string
//     would be IF one were carried; following the FT-991A's own
//     counter-argument (and FTdx9000/ft2000's identical choice in this
//     wave), a wrong guess here refuses real candidates rather than merely
//     describing one, so nothing is claimed.
//
//  7. Mode display names keep the manual's own spelling ("FSK (RTTY-LSB)",
//     "PKT-L", not core/cat's canonical "RTTY-LSB"/"DATA-L") — the
//     FTdx9000 precedent in this same wave, not the ft2000 one (which
//     substitutes core/cat's canonical words). Twelve nibbles, '1'-'C';
//     'D'/'E'/'F' are printed on no block in this manual (matrix §1.3) and
//     are simply absent from ModeNames.
//
//  8. The AM-N erratum (matrix §1.7) is NOT mapped into this dialect's mode
//     table. MD's own P2 legend prints a THIRTEENTH value, "D: AM-N"
//     (layout:812), disagreeing with the MR/MW memory-record legend's
//     twelve (§1.3) — recorded as an erratum, left standing, same trap
//     class as FTdx101 layout:931. This driver encodes the MEMORY blocks'
//     P6 (twelve values), not MD's P2, so Capabilities.Modes is not
//     widened to include AM-N from this citation alone.
//
// # Not this package's job
//
// EX/menu inventory (spec.md §3, out for all seven packages in this
// wave): EXItems is empty and no settings.go exists here — menu row "026
// CAT BAUD RATE" (layout:480) is cited only to support Bauds/DefaultBaud,
// never transcribed as an EX inventory entry. internal/fakeft950 (Phase 3b,
// disjoint files). Registration — internal/wiring, radiotext.go,
// app/uispec.go, README.md, docs/*.md, CHANGELOG.md — is Phase 4's job.
// The lift-K stream-error gap (core/kw/errors.go's newStreamError) is N/A:
// it binds Book570/Book870S only, and this is a Yaesu package with no
// core/kw involvement at all.
//
// # Field audit
//
// TestFieldAuditCoversEverySpecField (caps_test.go) pins that every one of
// spec.AllFields()'s twenty-seven members is explicitly named in
// bankFields (caps.go), whether mapped (six: frequency, mode, clarifier,
// ctcss state, ctcss tone, shift) or explicitly zeroed with a one-line
// reason at that map entry (tag/tag_display: NoTag per the 12/09/2026
// nameless-capability rule; scan_skip and erase: no such byte/command
// exists on this radio's 27-byte record or in its 90-command index; the
// seventeen Icom-tier fields: manual-evidenced absences, this being a
// Yaesu-family record).
package ft950
