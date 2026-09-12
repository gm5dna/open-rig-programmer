// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

const (
	// RecordLength is record-only: it excludes the band+channel address.
	// MANUAL-EVIDENCED (matrix §3.11 term-by-term arithmetic sums to the
	// printed 60-byte data area; 60 - AddressBytes = 57).
	RecordLength = 57
	// AddressBytes is one packed-BCD band byte followed by the two-byte
	// channel number (matrix §3.11's AddressFormBankChannel resolution).
	AddressBytes = 3
	// DataAreaLength is the complete 1A 00 data area on the wire.
	DataAreaLength = RecordLength + AddressBytes

	// DigitalSquelchOffset is the D-STAR digital squelch byte (matrix §1b's
	// D-STAR bytes table, "!4 Digital squelch setting"). No spec.Field
	// exists for it anywhere in this codebase, so — following the IC-7100
	// precedent spec.md names by name — it is never read into the
	// codeplug and stays outside every mapped span.
	DigitalSquelchOffset = 10
	// DigitalCodeSquelchOffset is matrix §3.11 term 12, "@4 digital code
	// squelch setting" — a second, separate D-STAR byte the matrix's own
	// arithmetic table names but its §1b prose table does not repeat.
	// Same treatment as DigitalSquelchOffset: no neutral field, never
	// mapped.
	DigitalCodeSquelchOffset = 20

	// The three D-STAR call-sign regions (matrix §1b), 8 bytes ASCII each,
	// fixed width, unmapped for the identical IC-7100-precedent reason.
	DestCallOffset = 24
	R1CallOffset   = 32
	R2CallOffset   = 40
	callLength     = 8
)

// nameCharset is the manual's own character-code table (matrix §3.9 /
// §1 row 30): printable ASCII 0x20-0x7E, all 95 characters, space and
// semicolon both included — identical to IC-7100's table, not the family
// default (which would wrongly reject semicolon).
const nameCharset = " !\"#$%&'()*+,-./0123456789:;<=>?@" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`" +
	"abcdefghijklmnopqrstuvwxyz{|}~"
