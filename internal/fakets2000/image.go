// SPDX-License-Identifier: GPL-3.0-or-later

package fakets2000

// Image is a factory for a fake's whole record map, the same contract every
// sibling Kenwood fake uses (WithFactoryImage).
type Image func() map[recordKey]MemState

// emptyRecord is the shape this fake invents for an unwritten channel —
// doc.go's register entry 3. Every field is its own "nothing here" value:
// zero digits, OFF/Simplex enum members, an eight-space name. It is NOT a
// reading of a printed sentence, unlike the 590 pair's own empty-channel
// answer; the matrix found none for this row (matrix §5, §6 item 5).
func emptyRecord() MemState {
	return MemState{
		Freq:        "00000000000",
		Mode:        '0',
		Lockout:     '0',
		ToneMode:    '0',
		ToneNo:      "00",
		CTCSSNo:     "00",
		DCSCode:     "000",
		Reverse:     '0',
		Shift:       '0',
		Offset:      "000000000",
		Step:        "00",
		MemoryGroup: '0',
		Name:        "        ",
	}
}

// exampleFreq is the manual's own worked frequency example, "FA00007000000;"
// for 7 MHz (ts2000:9567, repeated at 9595/9600/9607) — an 11-digit BCD Hz
// value, the same convention MR/MW's own P4 uses. It is a printed example,
// not an observed channel.
const exampleFreq = "00007000000"

// DefaultImage returns two example channels, deliberately small: with one
// printed frequency and one printed name state (blank, per emptyRecord's own
// reasoning) available, a larger image would repeat itself, and would turn
// every "this channel is empty" assertion in a test into a fixture accident.
//
// Every byte here is one of: a printed example (the frequency), a printed
// legend value (mode nibble '1' = LSB, the first of MD's nine values,
// ts2000:10611 per the matrix), or a value emptyRecord already uses for "no
// information" (tone/CTCSS/DCS/offset/step at their zero spelling, REVERSE
// and Shift at Simplex/OFF, Memory Group 0, the eight-space name — no name
// string is printed anywhere in this document) — doc.go's register entry 15.
// PROVENANCE.md carries the same posture in full.
func DefaultImage() map[recordKey]MemState {
	base := emptyRecord()
	base.Freq = exampleFreq
	base.Mode = '1' // LSB, MD's first legend value (ts2000:10611)

	return map[recordKey]MemState{
		{channel: 0, half: HalfRXOrStart}: base,
		{channel: 1, half: HalfRXOrStart}: base,
	}
}

// defaultSatelliteChannels is the ten-channel Satellite Memory bank's
// factory image: every flag OFF, the eight-space name — MemState's own
// "no information" convention (emptyRecord's doc comment), since no name
// string is printed anywhere in this document for this bank either.
func defaultSatelliteChannels() [10]satelliteChannel {
	var chans [10]satelliteChannel
	for i := range chans {
		chans[i] = satelliteChannel{name: "        "}
	}
	return chans
}
