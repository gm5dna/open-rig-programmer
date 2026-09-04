// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FTdx101D/MP's mode table, TRANSCRIBED FRESH from THIS
// radio's own manual rather than copied from the FT-710's or the FTdx10's.
//
// It has to be typed out again: core/cat's table is unexported, so there is
// nothing to reference even if referencing it were right — and it would not
// be. Three radios agreeing on a mode nibble is a fact about those three
// radios, not a shared definition, and the moment a fourth disagrees a shared
// table would have to be unpicked from every dialect that had quietly adopted
// it. The cost of retyping is a copy error, and dialect_test.go's mode
// identity pin is what catches one: it compares this table with cat.FT710's
// over all 256 wire bytes.
//
// SOURCE: the mode legend printed beside FIVE different commands in manual
// rev 2308-L, all five identical and all five running 1 to F with no other
// member — MD's P2 (layout 1240-1243, PDF page 16), IF's P6 (1089-1091, PDF
// page 15), MR's P6 (1286-1288, PDF page 17), MT's P6 (1321-1323, PDF page 17)
// and MW's P6 (1361-1363, PDF page 18). FIVE, not four: unlike the FTdx10's
// manual, this one prints the full legend beside IF as well, so a "four
// legends" claim carried over from the sibling package would be false here.
// The keys below are the legend's own wire bytes, written as byte literals
// rather than through core/cat's Mode constants, so that what is transcribed
// here is the manual's "1: LSB" and not core/cat's spelling of it.
//
// A SIXTH LEGEND EXISTS AND IS DELIBERATELY NOT SOURCED FROM. OI's P6 (layout
// 1443-1446, PDF page 19) prints the same fifteen members but MISNUMBERS the
// last two, "D: AM-N E: PSK E: DATA-FM-N" — a duplicated "E:" prefix where
// "F:" belongs. It is a printing defect, recorded as one in doc.go's chart
// defect record, and excluded from sourcing rather than silently reconciled
// against the other five. Reading it as evidence would either lose F:
// DATA-FM-N or make E: ambiguous; the five clean legends decide the table and
// the sixth is recorded as broken.
//
// The '0' entry is the exception, and it is deliberately spelt with the
// core/cat constant, because that is exactly what it is: ASSUMED, inherited,
// and named by the codec rather than by this manual. See doc.go's register.
var modeNames = map[cat.Mode]string{
	// ASSUMED — cat.ModeUnset ('0', "-") appears in NO FTdx101 mode legend;
	// all five clean ones run 1-F, and so does the defective OI one. It is
	// here because parsers must accept the placeholder: core/cat refuses to
	// EMIT it in any Set frame, so its presence widens what this dialect can
	// read and nothing else.
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW-U",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-L",
	cat.Mode('7'): "CW-L",
	cat.Mode('8'): "DATA-L",
	cat.Mode('9'): "RTTY-U",
	cat.Mode('A'): "DATA-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-U",
	cat.Mode('D'): "AM-N",
	cat.Mode('E'): "PSK",
	cat.Mode('F'): "DATA-FM-N",
}

// newDialect builds the FTdx101 family dialect for one CAT ID. D and MP
// share EVERY protocol byte but the ID answer (the ledger's
// applicability attestation is the evidence); the two instances exist so
// that probe/identity can be honest per model.
//
// ONE constructor and not two literals: two literals differing in one string
// would be two things to keep in step, and the sibling pins in dialect_test.go
// assert accessor by accessor that they do not differ anywhere else. With one
// constructor that claim is true by construction AND asserted; with two
// literals it would be asserted only.
//
// EVERY FIELD IS SET EXPLICITLY, including the two the combined MT form
// requires to be zero: a field left out of this literal would be
// indistinguishable from a field deliberately zeroed, and V9's "an
// inapplicable field must be explicitly zero" rule is only readable as a
// decision if the zero is written down.
//
// MustNewDialect rather than NewDialect because this is a compile-time
// constant table: a mistake in it is a build-time defect that must stop the
// programme loudly on first use, not an error threaded through model
// registration.
func newDialect(catID string) cat.Dialect {
	return cat.MustNewDialect(cat.DialectConfig{
		CATID:     catID,
		ModeNames: modeNames, // fresh transcription, 1..F per this manual;
		// '0' (cat.ModeUnset, "-") included as an ASSUMED member — absent
		// from every FTdx101 legend; parsers must accept the placeholder.
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			// ASSUMED bounds — the FTdx101 legends say only "5xx (5MHz
			// BAND)"; 501..599 is interpretation inherited from the
			// FT-710/FTdx10, both unverified. Register entry.
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000", // ASSUMED — in no FTdx101 slot legend
		},
		EXItems: exItems,
		// The FTdx101's EX grammar block prints "E X P1 P1 P2 P2 P3 P3"
		// (ftdx101_layout.txt:699-708, Set at 702, Read at 705, Answer at
		// 708): six digits, three components, as on both siblings.
		EXAddressForm: cat.EXAddressTriple,
		MT: cat.MTPolicy{
			Form:         cat.MTFormCombined,
			TagMaxBytes:  12,
			ClearTagByte: 0,   // must be 0 under MTFormCombined (V9)
			PadByte:      0,   // must be 0 under MTFormCombined (V9)
			TagFill:      ' ', // ASSUMED — never observed on this radio
		},
		Clarifier: cat.ClarifierPolicy{
			StepHz:   10, // ASSUMED — no step stated in the manual
			MaxAbsHz: 9990,
		},
		MWWriteKind: cat.CombinedMTSetKind, // MW P7 "(Fixed)" — a fact
		// of this radio, not a rule; see the difference pins.
	})
}

var (
	dialectD  = newDialect("0681")
	dialectMP = newDialect("0682")
)

// DialectD is the FTDX101D (CAT ID 0681).
//
// A function over an exported var so that the package-held value cannot be
// reassigned by a consumer: a Dialect is what the outbound write gate
// consults on every frame, and one a caller can swap after init is not a
// gate. cat.Dialect is a value type carrying only copied maps and slices, so
// the returned copy is inert in the other direction too.
func DialectD() cat.Dialect { return dialectD }

// DialectMP is the FTDX101MP (CAT ID 0682). Same reasoning as DialectD.
func DialectMP() cat.Dialect { return dialectMP }
