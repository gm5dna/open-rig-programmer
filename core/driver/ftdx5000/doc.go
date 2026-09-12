// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx5000 is the Yaesu FTdx5000 driver: one model row (FTDX5000/
// FTDX5000D/FTDX5000MP, one CAT document), bare New.
//
// # Provenance
//
// Everything protocol-shaped here comes from the Yaesu FTDX5000 SERIES CAT
// OPERATION REFERENCE, revision 1907-D, through
// docs/superpowers/ftdx5000-capability-matrix.md (gitignored, main
// checkout) — every pinned value below cites that matrix's own section,
// never the raw manual directly. NO FTDX5000 HAS EVER BEEN ASKED ANYTHING
// BY THIS PROJECT: every value is a reading of the manual or a written-down
// assumption, and writeTrialsComplete (caps.go) is false.
//
// # The dialect is built INLINE, not in a core/cat subpackage
//
// Unlike ftdx10/ftdx101/ft891/ft991a, this package's cat.Dialect is a
// literal cat.MustNewDialect(cat.DialectConfig{...}) in dialect.go, with no
// wrapping core/cat/ftdx5000 package. This radio shares its whole shape
// with three siblings (ft2000, ftdx9000, ft950), differing only in Model
// and CATID, so a second package would carry a crosscheck/exinventory/
// golden apparatus proving nothing a shared literal does not already show.
// dialect_test.go proves it against core/cat/dialecttest.Run the same as
// the four registered dialects do.
//
// # This radio has NO "MT" command at all
//
// Every other Yaesu dialect registered before this wave (FT-710, FTdx10,
// FTdx101, FT-891, FT-991A) documents an "MT" command — either the short
// form's separate tag/display frame or the combined form's whole-record-
// plus-tag frame — and core/driver/internal/yaesu's shared WriteChannel/
// BuildWriteCommand bodies are built entirely around sending one. The
// FTdx5000's Control Command List (matrix §0, layout:118-181) has no MT
// row at all: only MR (Read/Answer only) and MW (Set only), the classic
// 27-byte field-block frame core/cat/memdata.go's parseMemoryFrame/
// BuildMWSet already speak, with no tag concept anywhere.
//
// So this package's read.go and write.go call dialect.BuildMRRead/
// ParseMRAnswer/BuildMWSet DIRECTLY and do NOT use core/driver/internal/
// yaesu's WriteChannel/BuildWriteCommand/MTSpec/MTSetSpec — those hardcode
// the wire mnemonic "MT" into driver.WriteStep and gate through
// cat.CombinedMTSetKind, both wrong for a radio that sends "MW". The
// generic pieces that make no MT assumption (yaesu.NewEngine, yaesu.
// Handshake, yaesu.CTCSSMap/CTCSSLegend/CTCSSState, yaesu.ShiftByName,
// yaesu.TierRequestedFields) are reused as usual.
//
// cat.DialectConfig.MT is nonetheless a REQUIRED field (V9 refuses the
// zero Form) even though this radio has nothing for it to describe — the
// same structural requirement EXAddressForm carries under an empty
// EXItems (V12). Both are declared with the smallest legal values (MT:
// MTFormShort/MTReadsMemoryPMS/TagMaxBytes 1; EXAddressForm:
// EXAddressSingle, which IS manual-evidenced — see dialect.go) and never
// exercised by this driver: no method here calls BuildMTRead, BuildMTSet,
// ParseMTAnswer*, or anything EX-shaped.
//
// # CTCSSTone is a LIVE, WRITABLE field — the one real divergence from
// every registered sibling
//
// Every registered Yaesu dialect's P9 is a printed-fixed "00" (matrix §1.4,
// core/cat's P9Fixed00 default) and FieldCTCSSTone is therefore
// Unsupported on all of them. This radio's P9 is a genuine two-digit
// CTCSS tone-table index, 00-49, into the SAME standard 50-entry chart
// core/spec.StandardCTCSSTones() already carries (matrix §1.4, spot-
// checked at index 26/44) — core/cat's Lift Y built exactly the
// MemoryData.ToneIndex/MemoryP9Policy axis needed to round-trip it. Since
// the array index IS the CAT tone number by the SAME convention core/spec/
// tones.go documents for the FT-710's own CN command, mapping
// spec.FieldCTCSSTone to it is a direct index lookup (read.go, write.go's
// toneIndex), not a new vocabulary.
//
// Leaving it Unmapped (Unsupported) was considered and rejected: since
// this radio's ONE write frame (MW) always carries a P9 value on every
// channel it touches, a driver that never set ToneIndex would silently
// write "00" (67.0 Hz) into the tone slot on EVERY write regardless of
// what the radio previously held — corrupting a real per-channel setting
// under the banner of "not modelled yet". Mapping it is the smaller
// hazard and the more honest one.
//
// THE FIELD REQUIRES A KNOWN VALUE ON EVERY WRITE (write.go's
// buildWriteCommand refuses otherwise), because there is no "leave it
// alone" encoding: unlike core/codeplug's FieldState contract for a field
// this protocol genuinely cannot see, P9 is always on the wire and always
// means something, so Unknown cannot be silently defaulted without
// guessing. This is the same shape core/driver/internal/yaesu.Params.
// RefuseTagDisplayUnknown gives the FT-891's live tag-display flag — a
// live, mandatory field with no null encoding — restated here as a direct
// check because this driver does not go through Params' write path at all.
//
// ReadChannel always reports it Known: the byte is read on every MR
// answer, so there is no honest reason to report anything less.
//
// # The lift-K-gap equivalent does not apply here
//
// The brief's lift-K gap (Book570/Book870S's missing stream-error
// citation) is a core/kw concern; this package touches no core/kw file.
//
// # The ASSUMED register
//
// FIVE ENTRIES. Each is registered facts this package depends on that the
// matrix itself does not pin, together with what would lift it.
//
//  1. DefaultBaud 38400 (caps.go). The matrix's own §1.6 pins the FOUR
//     RATES manual-evidenced (menu 032 CAT RATE, {4800,9600,19200,38400})
//     but the factory-DEFAULT is ASSUMED on the same grounds every
//     registered sibling gives (core/driver/ftdx10/doc.go's entry 3): this
//     20-page CAT reference carries no factory-default column at all.
//     Cited, not re-registered, per the matrix's own "Register home" note.
//     LIFTS WITH: the baud a factory-configured FTdx5000's ID; exchange
//     actually answers at.
//
//  2. ClarMaxHz 9990 / ClarStepHz 10 (dialect.go). The 0000-9999 printed
//     range is manual-evidenced (MW's own P3 legend); the 10 Hz STEP and
//     the 9990 ceiling (the largest multiple of it inside the printed
//     range) are the family's single shared ASSUMED entry
//     (core/driver/ftdx10/doc.go's entry, cited not re-derived).
//     LIFTS WITH: a clarifier write trial against a real radio.
//
//  3. MinFreqHz 30_000 / MaxFreqHz 75_000_000 (caps.go). NEITHER is
//     discussed anywhere in the matrix — this manual is a CAT command
//     reference, not a specifications sheet, and states no tuning range at
//     all. Mirrored from every registered Yaesu sibling's own same-
//     generation working value (core/driver/ftdx10/doc.go's entry 4)
//     rather than left zero, since a zero MaxFreqHz reads as "no ceiling"
//     to every validator. LIFTS WITH: the FTDX5000 OPERATING manual's
//     specifications page, or edge-frequency write probes against a real
//     radio.
//
//  4. RequiredSlots left EMPTY (caps.go). The matrix's own §4 explicitly
//     declines to resolve this ("This matrix does not resolve whether
//     ftdx5000 needs a RequiredSlots entry") — unlike FT-950, whose
//     MemoryLo 000 forced the question, this radio's regular range starts
//     at 001 and nothing in this manual states any channel must stay
//     populated. Left empty rather than guessing "001" from the FT-710/
//     FTdx10 precedent: an unpopulated RequiredSlots makes
//     codeplug.Validate refuse nothing extra, which is the conservative
//     direction when no manual statement exists either way. LIFTS WITH:
//     observation of a real FTdx5000's channel 001 (whether the radio
//     ships with it populated, and whether the front panel will erase it).
//
//  5. MCSelects MCSelectsAll, and MT.ReadSlots MTReadsReadable (dialect.go).
//     The MC command's own legend (layout:890-897) enumerates 001-117 —
//     this radio's WHOLE slot space, there being no 60 m/EMG bank to
//     enumerate alongside it (matrix: no "5xx"/"5MHz"/"EMG" hit anywhere
//     in the whole extraction). MCSelectsAll and MCSelectsMemoryPMS are
//     BEHAVIOURALLY IDENTICAL on a radio with neither bank — the same
//     degenerate case core/cat/ft991a's own dialect documents — and All
//     is the correct member of the pair: the legend prints the radio's
//     full span, and MemoryPMS would assert a narrowing this manual does
//     not state. core/cat/dialecttest's own non-vacuity check (commit
//     1ca7aac's sibling addition) refuses to let an unevidenced narrowing
//     stand for a dialect with nothing 60m/EMG-shaped to narrow away.
//     MT.ReadSlots is inert regardless (no MT command exists at all) and
//     follows the same reasoning.
//
// SlotSpace.NoneWire is left ABSENT ("") rather than the family's usual
// ASSUMED "000": no MR, MW or MC legend in this manual prints a "000"/VFO
// placeholder row at all (unlike the FT-710's, which documents one
// explicitly), and cat.SlotSpace's own doc comment treats "" as a
// legitimate declared absence — inventing one here would be a claim this
// manual gives no grounds for, however every registered sibling reads.
package ftdx5000
