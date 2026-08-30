// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

const (
	// RecordOnlyLength excludes the two address bytes; TestProfileShape pins
	// this convention against the 27-byte data-area figure.
	RecordOnlyLength = 25
	// DataAreaLength includes the flat address in the printed 1A 00 area.
	DataAreaLength = 27
	// AddressBytes is the packed-BCD flat channel selector width.
	AddressBytes = 2
	// These offsets identify E6 regions rather than neutral fields.
	SelectNibbleOffset   = 0
	DataModeNibbleOffset = 8

	// The three record bytes whose BOTH nibbles the document prints as
	// fixed zeros, and which therefore lie under no mapped span:
	//
	//   - FreqFixedOffset is printed ⑧, the fifth frequency cell, whose
	//     two rotated leaders read "1000 MHz digit: 0 (Fixed)" and
	//     "100 MHz digit: 0 (Fixed)" (matrix §3.16.3, register entry
	//     ic7851-fixed-nibble-reencode).
	//   - ToneTXFixedOffset is printed ⑫ and ToneRXFixedOffset printed
	//     ⑮, the leading cell of the repeater-tone diagram that both tone
	//     triples point at, whose two leaders read "Fixed digit: 0*"
	//     (matrix §3.16.4, register entry ic7851-tone-fixed-byte).
	//
	// They are constants rather than a derivation because a fixed byte is
	// a PRINTED fact about this record, and every leg that has to know
	// about one — the layout, the gate's re-encode, and the driver's
	// read-side refusal — must be reading the same three numbers.
	// TestFixedBytesLieUnderNoMappedSpan pins that no span covers them.
	FreqFixedOffset   = 5
	ToneTXFixedOffset = 9
	ToneRXFixedOffset = 12
)

// NameCharset is printable ASCII. Digit and space codes are ASSUMED under
// register ic7851-name-digit-space-codes; TestGoldenFramesAndGate pins them.
const NameCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!$&?'^-/,;<([{|~#%\\\"`+*.:=>)]}_@ "
