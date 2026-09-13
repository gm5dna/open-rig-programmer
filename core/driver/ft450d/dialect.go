// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FT-450D's P6 mode table, TRANSCRIBED from this radio's
// own manual (matrix §1.2), identical on MD (layout:711-712), MR
// (layout:794-797) and MW (layout:798). Eleven names, '1'-'9' then 'B','C':
// a CLEAN HOLE at 'A' (no `A:` row on any of the four P6-carrying blocks)
// and 'D'/'E'/'F' entirely absent — unlike ftdx3000/ftdx1200's own mode
// gaps, this radio's manual shows no MD-vs-MR/MW disagreement to record
// (matrix §1.2, "no storability ambiguity to flag here").
//
// THE MANUAL'S OWN SPELLING IS KEPT ("DATA (RTTY-LSB)"/"USER-L"/"USER-U",
// not core/cat's canonical "RTTY-LSB"/"DATA-L"/"DATA-U") — the ft950/
// ftdx9000 precedent in this wave, not ft2000's substitution.
//
// cat.ModeUnset ('0', "-") is included as an ASSUMED member, matching
// every registered sibling: it appears in no FT-450D legend, but parsers
// must accept it and core/cat refuses to ever EMIT it in a Set frame.
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "DATA (RTTY-LSB)",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "USER-L",
	cat.Mode('9'): "DATA (RTTY-USB)",
	// 'A' is a CLEAN HOLE (matrix §1.2): no `A:` row on any of MD/MR/MW/IF.
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "USER-U",
	// 'D', 'E', 'F' are printed on no P6-carrying block in this manual at
	// all (matrix §1.2) — the roadmap's own evidence-table cell for this
	// radio ("old, USER-L/U, to C") independently confirmed here.
}

// dialect is the FT-450D, built once at init and validated by
// cat.MustNewDialect. EVERY FIELD IS SET EXPLICITLY, including the ones
// this configuration requires to be zero (SixtyLo/Hi, EmergencyWire,
// NoneWire, the short-form MT fields this radio never actually sends).
//
// Built INLINE over cat.NewDialect/MustNewDialect rather than a
// core/cat/ft450d subpackage (v1.7.0 Kenwood/Yaesu wave brief, applied to
// this fifth single-row Yaesu package): a literal here is the whole of it.
//
// MustNewDialect rather than NewDialect: a mistake in this literal is a
// build-time defect that must stop the programme loudly on first use.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// matrix §1.6: the ID command's own P1 legend, "P1 0244 (Fixed value)"
	// (layout:605).
	CATID:     "0244",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// MemoryLo/Hi 1/500 (matrix §2.5, OM folio 58: "500 regular
		// memories, labeled 'MEM-001' through 'MEM-500'"; CAT book's own
		// MC legend, "001 - 500: Regular Memory Channel", layout:714-715).
		// WRITE-CAPABLE per the roadmap's SAFE SHAPE ruling
		// (radio-roadmap.md:80-87): "write 001-500 ONLY" — the codec-level
		// slot range does not itself distinguish read from write; caps.go's
		// two separate bank Fields maps carry that split, not this axis.
		MemoryLo: 1, MemoryHi: 500,
		// SixtyLo/Hi = 0, 0 — the CHOICE side of the SAFE SHAPE ruling, NOT
		// a manual-evidenced absence (matrix §2.5.1). The 505-510 channels
		// (US 5 / UK 7 sixty-metre + one Alaska Emergency) are REAL — OM
		// folio 58 names them — but the CAT book's own MC/IF legends never
		// assign them a CAT address (MC's P1 domain stops at 504,
		// layout:713-719; IF's own P1 domain prints wider, "000-510", but
		// no legend anywhere maps 505-510 to these channels by name). The
		// ruling: "505-510 NOT in the dialect at all (SixtyLo/Hi = 0;
		// semantics unproven, UK 7-channel count does not fit 6 slots)" —
		// left out entirely rather than guessed, per the six bench probes
		// in community-hints.md that would settle it.
		SixtyLo: 0, SixtyHi: 0,
		// Two PMS pairs, numbered decimally continuing the memory range:
		// the MC legend's own "501: P1L Channel 502: P1U Channel
		// 503: P2L Channel 504: P2U Channel" (layout:716-719, matrix §2.5)
		// — wire numbers ARE the PMS channel numbers, the same numeric
		// form ft2000/ft950/ft991a all declare.
		PMSPairs:     2,
		PMSForm:      cat.PMSFormNumeric,
		PMSNumericLo: 501,
		// NO EMERGENCY CHANNEL as a memory slot: the Alaska Emergency
		// frequency is one of the excluded 505-510 span above, not a
		// separately-addressed "EMG" wire form — no such legend token
		// exists anywhere in this manual (matrix §2.5/§2.5.1).
		EmergencyWire: "",
		// NO "000" none/VFO placeholder either: the MC legend's whole
		// domain is "001 - 504" (layout:708-712) with no zero form printed
		// anywhere in this manual's memory-channel legends.
		NoneWire: "",
		// The MC legend prints the WHOLE "001 - 504" span as one P1 range
		// before decomposing it into regular/PMS (layout:708-712) — the
		// same shape every registered Yaesu sibling's own MC legend takes.
		// This radio has no addressable 5xx/EMG bank (SixtyLo/Hi above), so
		// MCSelectsAll and MCSelectsMemoryPMS give IDENTICAL verdicts on
		// every wire form there is; the wide value is declared because it
		// is what the legend's own top line says.
		MCSelects: cat.MCSelectsAll,
	},
	// EX/menu inventory is OUT OF SCOPE for this wave (spec.md §2, "Out for
	// all three"): EXItems stays empty.
	EXItems: nil,
	// EX legend's own P1 component is a SINGLE three-digit MENU number (the
	// ft2000/ft950/ft991a shape, not a wider multi-digit form). Required
	// even with EXItems empty: it also sizes the EX read frame the
	// outbound gate measures (cat.DialectConfig.EXAddressForm's own doc
	// comment).
	EXAddressForm: cat.EXAddressSingle,
	// THIS FAMILY HAS NO MT (TAG) COMMAND AT ALL — the CAT book's 90-command
	// index jumps MC -> MD -> MG -> MK -> ML -> MR -> MS -> MW, no MT
	// (matrix §0). But cat.DialectConfig.MT has no default
	// (validateMTPolicy/V9 refuses the zero value unconditionally, whether
	// or not a radio's own MT command exists), so this is INFRASTRUCTURE
	// THIS DRIVER NEVER EXERCISES: read.go/write.go build MR/MW frames
	// directly and never call BuildMTRead/BuildMTSet/ParseMTAnswer — the
	// same "declared but unreachable" shape every no-MT Yaesu package in
	// this fleet hits (ft2000/ftdx5000/ftdx9000/ft950's own doc.go entries
	// say so). TagMaxBytes is pinned at its allowed minimum (1) and
	// ClearTagByte at an arbitrary valid wire byte purely to satisfy V9's
	// construction-time validation; neither value describes anything this
	// radio's manual prints. This radio DOES have a front-panel-only
	// 7-character tag (OM folio 62) — but the CAT book's only tag-adjacent
	// surface, EX036, is a display-mode toggle with no parameter that
	// carries tag TEXT (matrix §2.7) — hence NoTag, not a tag route this
	// dialect could express.
	MT: cat.MTPolicy{
		Form:         cat.MTFormShort,
		ReadSlots:    cat.MTReadsReadable,
		TagMaxBytes:  1,
		ClearTagByte: ' ',
		PadByte:      0,
		TagFill:      0,
	},
	// "Clarifier Offset: 0000 - 9999 (Hz)" on every P3-carrying block
	// (layout:791-792, :626) — matrix §2.8. StepHz is ASSUMED at 10 Hz, the
	// Yaesu-family default every registered sibling carries (no step is
	// printed anywhere in this manual). MaxAbsHz is 9990, not the printed
	// range's own 9999: cat.NewDialect's V10 rule requires MaxAbsHz to be
	// an exact multiple of StepHz, and 9999 is not a multiple of 10 — 9990
	// is the largest multiple of the assumed step inside the printed
	// range, the FT-991A/ft2000/ft950 precedent for exactly this shape.
	Clarifier: cat.ClarifierPolicy{
		StepHz:   10,
		MaxAbsHz: 9990,
	},
	// P5 (byte 20) is a LIVE flag: "P5 0: TX CLAR OFF 1: TX CLAR ON"
	// (layout:794, :793) — matrix §1.1/§1.5.
	MemoryP5: cat.P5TxClar,
	// P8 is the ordinary three-state legacy domain: "0: CTCSS OFF
	// 1: CTCSS ENC/DEC 2: CTCSS ENC" (layout:799, :626) — matrix §2.14, no
	// DCS member (NOT ft991a's five-state domain).
	ToneStates: cat.ToneStatesCTCSS,
	// MW's P7 write-fixed byte is '0' (layout:798, "P7 0: Fixed"), and
	// matrix §1.4 flags the SAME trap every sibling in this wave carries:
	// '0' is cat.KindVFO, NOT cat.KindMemory (memdata.go's own Kind byte
	// legend, "0 VFO, 1 Memory").
	MWWriteKind: cat.KindVFO,
	// 27-byte frame, 8-digit P2 (matrix §1.1: one digit narrower than the
	// registered 28/9-digit shape, a pure position shift for P3 onward) —
	// identical to ft2000/ftdx5000/ftdx9000/ft950.
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
	// P9 is a LIVE two-digit CTCSS tone-table index (matrix §1.3), into the
	// SAME standard 50-tone chart every registered dialect shares — the
	// Lift-Y P9ToneIndex axis, confirmed here rather than a new policy.
	MemoryP9: cat.P9ToneIndex,
})

// Dialect returns the FT-450D's cat.Dialect.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer — see the sibling dialects' identical reasoning.
func Dialect() cat.Dialect { return dialect }
