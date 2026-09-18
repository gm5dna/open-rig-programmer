// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FTX-1's P6 mode table: the FT-710's own sixteen values
// (identical wire bytes and names — FTX-1 spec.md §5, "FTX-1's P6 legend
// ... is the FT-710's own 16-value table") plus TWO NEW members this
// radio's own legend adds, 'H' (C4FM Data-Narrow) and 'I' (C4FM
// Voice-Wide) — this codec's first C4FM digital-voice nibbles.
//
// TRANSCRIBED FRESH rather than referenced from core/cat's package-level
// table, for core/cat/ft991a/dialect.go's own reason: core/cat's mode
// table is unexported, and even if it were not, two radios agreeing on a
// mode nibble is a fact about those two radios, not a shared definition.
//
// 'G' AND 'J' ARE DELIBERATELY ABSENT. The manual prints both as "-", the
// same placeholder the table uses for '0' (cat.ModeUnset) — ASSUMED
// reserved/unused nibbles rather than a second real "unset" mode, on
// documentation-depth grounds alone (spec.md §5, open question 3): no
// statement either way. Leaving them out of this map means
// cat.Dialect.ValidMode refuses them exactly as it refuses every other
// nibble no dialect declares — the safer failure mode if a real radio ever
// emits one (a *ParseError, not a misnamed mode).
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-",

	cat.ModeLSB:   "LSB",
	cat.ModeUSB:   "USB",
	cat.ModeCWU:   "CW-U",
	cat.ModeFM:    "FM",
	cat.ModeAM:    "AM",
	cat.ModeRTTYL: "RTTY-L",
	cat.ModeCWL:   "CW-L",
	cat.ModeDATAL: "DATA-L",
	cat.ModeRTTYU: "RTTY-U",

	cat.ModeDATAFM:  "DATA-FM",
	cat.ModeFMN:     "FM-N",
	cat.ModeDATAU:   "DATA-U",
	cat.ModeAMN:     "AM-N",
	cat.ModePSK:     "PSK",
	cat.ModeDATAFMN: "DATA-FM-N",

	// 'H' and 'I': the FTX-1's own two new modes (spec.md §5,
	// ftx1_layout.txt:1546-1549). MANUAL-EVIDENCED, not assumed — unlike
	// 'G'/'J' immediately above, which this manual never names at all.
	cat.Mode('H'): "C4FM-DN",
	cat.Mode('I'): "C4FM-VW",
}

// dialect is the FTX-1, built once at init and validated by
// cat.MustNewDialect's rules. EVERY FIELD IS SET EXPLICITLY, including the
// ones this configuration requires to be zero — MT.P11 (no P11 byte exists
// under MTFormShortNoDisplay) and MT.ClearTagByte/PadByte (no distinct
// clear encoding under a fixed-width, TagFill-padded tag field) — for
// core/cat/ft991a/dialect.go's own reason: a field left out of this
// literal would be indistinguishable from a field deliberately zeroed.
//
// MustNewDialect rather than NewDialect: this is a compile-time constant
// table, and a mistake in it is a build-time defect that must stop the
// programme loudly on first use.
//
// No FTX-1 has ever been asked anything by this project (no hardware
// exists to ask, and no owner has run a write trial): every value below is
// MANUAL-EVIDENCED or ASSUMED, never HW-CONFIRMED. See doc.go's register
// for every ASSUMED value in one place, and spec.md/matrix-ftx1.md
// (.superpowers/sdd/2026-09-18-v1100-ftx1/reviews/) for the full citation
// trail.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// "ID D ;" -> "ID0840;", fixed, shared unmodified by both bodies
	// (spec.md §1, ftx1_layout.txt:1305). Body ("field" vs "optima") is
	// NOT part of this identity and is never inferred from it, from AC, or
	// from anything else on the wire — no CAT mechanism distinguishes them
	// (spec.md §1, doc.go's register).
	CATID:     "0840",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// "00001-00999: (Memory Channel)" (spec.md §3.1, §10). The
		// printed "00001-00099" on MC's/MT's own P2/P0 legends is the
		// erratum ruling already taken on the roadmap and re-confirmed
		// directly against the manual while writing spec.md: 999 ships,
		// 099 is footnoted. Not this package's decision to re-litigate.
		MemoryLo: 1, MemoryHi: 999,
		// The "5 MHz BAND", 50001-50020 (spec.md §3.1/§10) — the SAME
		// physical allocation every other registered dialect's
		// SixtyLo/SixtyHi bank names (the 60 m band IS the 5 MHz band),
		// just five digits wide and numbered from 50001 rather than 501.
		// The majority-reading erratum ruling: MC's own "50000-50020"
		// and MT's own "50001-50009" are footnoted, not shipped (spec.md
		// §10).
		SixtyLo: 50001, SixtyHi: 50020,
		// 50 pairs, "P-01L"-"P-50U" (spec.md §4, ftx1_layout.txt:1533).
		PMSPairs: 50,
		// The FTX-1's own dash-token shape: a hyphen and a TWO-digit pair
		// number, not the registered family's single-digit "P<n><L|U>".
		PMSForm: cat.PMSFormDashToken,
		// Must be exactly 0 under a token form (PMSFormDashToken included
		// — dialectvalidate.go's V15): the pair number never reaches the
		// wire as a decimal channel number under either token shape.
		PMSNumericLo: 0,
		// "EMGCH: (EMERGENCY CH)" (spec.md §3.1) — a NAMED TOKEN, not a
		// numeric slot, unlike every registered dialect's 3-character
		// "EMG".
		EmergencyWire: "EMGCH",
		// "00000: VFO or MT or QMB" (spec.md §3.1, §4) — this dialect's
		// own none form, five digits wide like everything else in its
		// slot space. Semantics UNKNOWN/ASSUMED per the reference, same
		// as every registered dialect's "000"; never emitted by a
		// builder.
		NoneWire: "00000",
		// The FTX-1's entire slot space is 5 bytes wide, not 3 (spec.md
		// §4) — the seam F1 built this milestone specifically for this
		// radio.
		SlotDigits: 5,
		// FTX-1's MC carries a LEADING per-port (MAIN/SUB) byte before
		// its slot (spec.md §3.4) — a frame shape nothing in this
		// package's "MC" + slot + ";" codec can build or parse, port
		// byte aside. MCSelectsUnsupported declares no MC support at
		// all: BuildMCSet/BuildMCRead/ParseMCAnswer and the outbound
		// gate's validMCCommand all refuse cleanly rather than
		// mis-building or mis-parsing a coincidentally-shaped frame.
		// Consistent with this milestone's own scoping (spec.md §3.4,
		// open question 2; plan.md's Lane F "MC is not read by the
		// driver this milestone") — F2 reads via MR+MT only, never MC.
		MCSelects: cat.MCSelectsUnsupported,
	},
	// EX/GT/VM are all deferred this milestone (spec.md §10, "EX/GT
	// deferred" — FTX-1's own EX chart is not evidenced in the manual
	// excerpts this spec pass read). EXItems stays nil — "a radio with no
	// modelled EX surface is representable" (dialectconfig.go).
	EXItems: nil,
	// EXAddressForm has no default and V12 refuses the zero value even
	// for an empty EXItems (the width also sizes the EX read frame the
	// outbound gate measures). ASSUMED PLACEHOLDER: this milestone has no
	// EX evidence to choose a real width from (spec.md §10), and the
	// value is inert either way — EXItems is empty, and this driver's
	// read path never issues an EX command at all (plan.md's Lane F: "the
	// read path for FTX-1 (MR+MT only) never issues EX"). Recorded in
	// doc.go's register rather than treated as a real fact about this
	// radio.
	EXAddressForm: cat.EXAddressTriple,
	MT: cat.MTPolicy{
		// "MT" + P0(5) + P1(12, tag) + ";" = 20 bytes, ALWAYS — no
		// display byte at all, unlike the FT-710's short form (spec.md
		// §3.3/§8).
		Form: cat.MTFormShortNoDisplay,
		// FTX-1's MT Read domain prints memory, PMS, 5 MHz AND EMGCH —
		// the same span Dialect.readableSlot admits (spec.md §3.3): "The
		// READ direction is MTReadsReadable territory ... memory, PMS,
		// 60m and EMG all legal MT-read targets, matching FTX-1's own
		// printed Read domain exactly." The SET direction stays
		// memory/PMS-only regardless (mtSlotValid delegates to
		// writableSlot, standing project policy, spec.md §3.3) —
		// ReadSlots governs the read request only.
		ReadSlots: cat.MTReadsReadable,
		// P11 belongs to MTFormCombined alone; this form has no P11 byte
		// at all, so it MUST be the zero policy (dialectvalidate.go's V9
		// case for MTFormShortNoDisplay).
		P11: 0,
		// "up to 12 characters" (spec.md §8, ftx1_layout.txt:1580) —
		// identical width to the FT-710's own MTPolicy.TagMaxBytes.
		TagMaxBytes: 12,
		// Must be 0 under MTFormShortNoDisplay: no distinct clear
		// encoding is documented for this form (an empty tag is simply
		// the all-TagFill field, mtnodisplay.go's own doc comment).
		ClearTagByte: 0,
		PadByte:      0,
		// ASSUMED — the FT-710's own value. The manual gives no worked
		// example of an empty tag (spec.md §8): "nothing confirms
		// agreement either" beyond there being no reason to expect a
		// difference. See doc.go's register.
		TagFill: ' ',
	},
	Clarifier: cat.ClarifierPolicy{
		// "0000-9990 (Hz) (4 Bytes)" (spec.md §7, ftx1_layout.txt:1538-1539)
		// — byte-identical bound to the FT-710's own ClarifierPolicy.
		// StepHz is ASSUMED: the manual states the range, not the step
		// granularity, exactly as the FT-710's own citation does.
		StepHz:   10,
		MaxAbsHz: 9990,
	},
	// Byte 20, "TX CLAR on/off", is a LIVE field on every MR/MW block
	// (spec.md §3.1 offset table) — not the FT-891's printed-fixed "0".
	MemoryP5: cat.P5TxClar,
	// The FTX-1's own six-value P8 domain (spec.md §6): 0 OFF, 1 CTCSS
	// ENC/DEC, 2 CTCSS ENC, 3 DCS (unsplit), 4 PR FREQ, 5 REV TONE — wider
	// than either registered ToneStateDomain.
	ToneStates: cat.ToneStatesSix,
	// Byte-for-byte the FT-710's own 28-byte shared field block with the
	// address widened from 3 to 5 bytes: 30 bytes total, 9-digit
	// frequency unchanged (spec.md §3.1).
	MemoryFrameLen:   30,
	MemoryFreqDigits: 9,
	// "P9 00: (Fixed)" (spec.md §3.1, offset 24-25) — printed-fixed,
	// same as every registered dialect.
	MemoryP9: cat.P9Fixed00,
	// ASSUMED. The FTX-1's own P7 domain (spec.md §3.1: "0 VFO 1 Memory
	// 2 Memory Tune 3 QMB 4 - 5 PMS") is byte-identical to the FT-710's,
	// whose HW-CONFIRMED finding is that the radio requires KindMemory on
	// EVERY MW write regardless of bank (core/cat/mw.go). No FTX-1 has
	// ever been asked, so this is the same value taken on analogy, not
	// evidence — matrix §2 probe 4 records it as OPEN. See doc.go's
	// register.
	MWWriteKind: cat.KindMemory,
})

// Dialect returns the FTX-1's cat.Dialect.
//
// A function over an unexported var, matching every other registered
// dialect's shape (core/cat/ft991a/dialect.go et al.): a Dialect is what
// the outbound write gate consults on every frame, and one a caller could
// reassign after init would not be a gate. cat.Dialect is a value type
// carrying only copied maps and slices, so the returned copy is inert in
// the other direction too.
func Dialect() cat.Dialect { return dialect }
