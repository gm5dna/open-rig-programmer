// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

// defaultImageGroup is which memory group a fresh Radio's channels sit in:
// group 0000, the first of the printed "0000 ~ 0099: Normal memory channel
// group", 0-based.
const defaultImageGroup = 0

// defaultIdentityToken is what a Radio answers "Read the receiver ID" with when
// no WithIDToken option pins something else.
//
// IT IS ARBITRARY, AND CONSPICUOUSLY SO. The command table on PDF page 5
// (printed folio 4) prints the row "19 / 00 / <blank Data cell> / Read the
// receiver ID" and cross-references no command-format page: the request's
// emptiness is documented and the ANSWER'S VALUE IS NOT, anywhere in the guide.
// A fake that answered 96, or 0A, or anything else with the shape of a fact
// would be asserting one nobody has, and every consumer that then matched on it
// would be testing this package's guess rather than its own driver.
//
// DE AD was chosen precisely because no reader could mistake it for a fact
// about an IC-R8600 — the same choice, for the same reason, that
// internal/fakeic905 records. doc.go register entry 1, and see WithIDToken,
// which exists so a consumer can prove its driver RECORDS whatever it gets
// rather than matching a value.
var defaultIdentityToken = []byte{0xDE, 0xAD}

// defaultChannel is one channel of the factory image: which wire mode code its
// record carries, the name it holds, and the tail bytes that follow the head.
type defaultChannel struct {
	channel int
	mode    byte
	name    string
	tail    []byte
}

// defaultChannels is the image a Radio starts with when no WithRecord or
// WithEmpty option replaces part of it: one occupied channel per declared mode
// class, in group 0, and nothing anywhere else.
//
// WHICH CHANNELS ARE OCCUPIED IS INVENTED — doc.go register entry 10. Nothing
// in the guide says how many channels an IC-R8600 ships occupied, or which, or
// what they hold; the record pages print each field's permitted values and
// never a factory value. Eight channels, one per layout (with both NXDN wire
// codes present), is enough for a consumer to exercise every declared tail
// without seeding anything, small enough to read in a test failure, and stated
// here so no reader mistakes it for a fact.
//
// THE FIELD VALUES ARE NOT INVENTED. Each is a value the field's own printed
// domain admits, and the head below is the one the golden-vector leg derived
// from the same pages: 145.500000 MHz, TS on at step 05, programmable step
// 9 kHz in the drawn digit order, attenuator off, preamp on, ANT1, IP+ off.
// The tails likewise carry values from the printed domains — TSQL 88.5 Hz and
// DTCS 023 for FM, NAC 293h for P25, CSQL code 12 for D-STAR, COM ID 123 and
// CC 12 for dPMR, RAN 05 for NXDN, UC code 123 for DCR, encryption and
// scrambler off with zero keys.
var defaultChannels = []defaultChannel{
	// (11) 02 = AM, one of the eleven printed codes that take no tail.
	{channel: 0, mode: 0x02, name: "AM"},
	// (11) 05 = FM. Tail D2: TSQL, 88.5 Hz, DTCS 023 Normal.
	{channel: 1, mode: 0x05, name: "FM", tail: []byte{0x01, 0x00, 0x08, 0x85, 0x00, 0x00, 0x23}},
	// (11) 16 = P25. Tail D3: NAC, 293h.
	{channel: 2, mode: 0x16, name: "P25", tail: []byte{0x01, 0x02, 0x09, 0x03}},
	// (11) 17 = D-STAR. Tail D4: CSQL (this block prints no code 1), code 12.
	{channel: 3, mode: 0x17, name: "D-STAR", tail: []byte{0x02, 0x12}},
	// (11) 18 = dPMR. Tail D5: COM ID 123, CC 12, scrambler off, key 00000.
	{channel: 4, mode: 0x18, name: "dPMR", tail: []byte{0x01, 0x01, 0x23, 0x12, 0x00, 0x00, 0x00, 0x00}},
	// (11) 19 = NXDN-VN and (11) 20 = NXDN-N: two wire codes, one drawn tail.
	{channel: 5, mode: 0x19, name: "NXDN-VN", tail: []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00}},
	{channel: 6, mode: 0x20, name: "NXDN-N", tail: []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00}},
	// (11) 21 = DCR. Tail D7: UC code 123, encryption off, key 00000.
	{channel: 7, mode: 0x21, name: "DCR", tail: []byte{0x01, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00}},
}

// defaultImage builds a fresh channel map. A FRESH MAP PER RADIO, so an Option
// applied to one fake cannot reach another's channels.
func defaultImage() map[chanAddr]MemState {
	img := make(map[chanAddr]MemState, len(defaultChannels))
	for _, c := range defaultChannels {
		rec := append(defaultHead(c.mode, c.name), c.tail...)
		img[addrOf(defaultImageGroup, c.channel)] = MemState{Record: rec}
	}
	return img
}

// defaultHead builds the 37-byte common head, field by field in the printed
// index order of PDF page 12 (printed folio 11). See headFields for the widths
// and the labels; the values are the printed domains' own.
func defaultHead(mode byte, name string) []byte {
	head := []byte{
		0x00,                         // (5)  Skip/Select: SKIP OFF, select OFF
		0x00, 0x00, 0x50, 0x45, 0x01, // (6)-(10) receiving frequency, 145.500000 MHz
		mode,                   // (11) receiving mode
		0x01,                   // (12) filter setting, 01 = FIL1
		0x00,                   // (13) duplex, high nibble 0 fixed, low nibble 0 = OFF
		0x00, 0x00, 0x00, 0x00, // (14)-(17) offset frequency, zero
		0x01,       // (18) TS function, 01 = ON
		0x05,       // (19) tuning step setting
		0x90, 0x00, // (20),(21) programmable tuning step, in the drawn digit order
		0x00, // (22) attenuator, 00 = OFF
		0x01, // (23) preamplifier, 0 fixed / 1 = ON
		0x00, // (24) antenna, 0 fixed / 0 = ANT1
		0x00, // (25) IP+, 0 fixed / 0 = OFF
	}
	return append(head, encodeName(name)...)
}

// encodeName spells a memory name as the 16 bytes of printed indices
// (26)-(41): one byte per character, padded to sixteen.
//
// BOTH HALVES ARE ASSUMED, and separately — doc.go register entries 8 and 9.
// PDF page 11's "● Character entries" table gives the MEMORY NAME row's
// selectable characters and a total character number of 16, and prints NO CODE
// for any of them; nothing anywhere says what fills the cells a shorter name
// leaves over. One ASCII byte per character, padded with the space character,
// is this package's reading, and it is a reading, not a finding.
//
// A character outside the printed repertoire is encoded all the same: this fake
// stores what it is given and holds no opinion about a consumer's fixture. A
// name longer than sixteen characters is TRUNCATED to sixteen, because the
// field is sixteen drawn cells and there is nowhere else for the rest to go.
func encodeName(s string) []byte {
	out := make([]byte, 16)
	for i := range out {
		out[i] = namePadByte
	}
	copy(out, s)
	return out
}

// namePadByte is the assumed pad, 0x20. Space is EXPLICITLY one of the
// selectable characters the table lists — which is why it was chosen over a
// null byte, which is not in the listed repertoire at all. doc.go entry 9.
const namePadByte = 0x20
