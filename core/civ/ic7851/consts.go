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
)

// NameCharset is printable ASCII. Digit and space codes are ASSUMED under
// register ic7851-name-digit-space-codes; TestGoldenFramesAndGate pins them.
const NameCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!$&?'^-/,;<([{|~#%\\\"`+*.:=>)]}_@ "
