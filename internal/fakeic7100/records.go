// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import "fmt"

// This file holds everything this package knows about the IC-7100 memory
// record. It was derived HERE, from the two quarantined artefacts
// PROVENANCE.md names — the semantic transcription of the "• Memory content
// setting / Command: 1A 00" diagram on PDF p.375 (folio 20-16), and the
// geometry witness's independent measurement of the same two bars — and from
// nothing in this repository.
//
// # THE DERIVATION, IN FULL
//
// The diagram draws the record as two bars of cells with bracketed index
// groups above them. Long groups are abbreviated: a cell, a dashed elision
// cell, a cell, three drawn boxes standing for the whole run. So a group's
// width is its PRINTED INDEX SPAN, not the number of boxes drawn for it. That
// convention is the diagram's own, used identically for every long group, and
// both artefacts state it.
//
// Taking each group's printed span in wire order:
//
//	(1)        bank number                          1 B    running total   1
//	(2),(3)    memory channel number                2 B                    3
//	(4)        split and select memory              1 B                    4
//	(5)–(9)    operating frequency                  5 B                    9
//	(10),(11)  operating mode, filter               2 B                   11
//	(12)       data mode                            1 B                   12
//	(13)       duplex and tone                      1 B                   13
//	(14)       digital squelch                      1 B                   14
//	(15)–(17)  repeater tone frequency              3 B                   17
//	(18)–(20)  tone squelch frequency               3 B                   20
//	(21)–(23)  DTCS code and polarity               3 B                   23
//	(24)       digital code squelch                 1 B                   24
//	(25)–(27)  duplex offset frequency              3 B                   27
//	(28)–(35)  UR destination call sign             8 B                   35
//	(36)–(43)  R1 access repeater call sign         8 B                   43
//	(44)–(51)  R2 gateway/link repeater call sign   8 B                   51
//	5f–51f     the transmit duplicate              47 B                   98
//	(52)–(67)  memory name                         16 B                  114
//
// Three of those lines are not simple readings, and each is recorded as the
// decision it is:
//
//  1. THE TRANSMIT DUPLICATE'S WIDTH IS ARITHMETIC, NOT A COUNT. Its group is
//     drawn as ONE dashed box with no internal divider, so nothing can be
//     counted in it; the witness says so explicitly. Its width is its printed
//     index span, 51 − 5 + 1 = 47, and 47 is also exactly the width of the
//     fields (5)–(51) it duplicates, which is what the printed NOTE — "The
//     same data as (5)–(51) are stored in 5f–51f" — requires of it. The two
//     numbers were derived separately and agree; TestRecordGeometryIsDerived
//     FromThePrintedFieldWidths re-does both.
//
//  2. THE PRINTED INDICES AND THE MEASURED POSITIONS PART COMPANY AFTER (51),
//     and this package follows the MEASURED positions. Both artefacts record
//     the divergence and neither reconciles it: the group printed 5f–51f is
//     measured at data-area bytes 52–98, and the group whose printed index
//     begins at (52) is measured beginning at data-area byte 99. Reading the
//     printed indices as positions would put the name back at byte 52, on top
//     of the duplicate, which cannot be a record. See doc.go, entry 7
//     (ic7100-wire-order).
//
//  3. THE NAME IS SIXTEEN BYTES, NOT NINE. The diagram bar labels the last
//     group (52)～(60), nine bytes; the body text on the same page prints
//     "(52)–(67) Memory name setting / 16 characters (Fixed)". Both artefacts
//     transcribe BOTH readings, in separate rows, and reconcile neither. This
//     package takes 16, because the body text is self-consistent — 52 + 16 − 1
//     = 67 — and the bar's (60) corroborates nothing on the page, its own cells
//     being an ellipsis that conveys no count. The other reading would make the
//     data area 107 bytes; that is the near miss, and it is recorded here so
//     that a future reader can see it was considered and not merely missed.
//
// The address, in this document's own wire order, is the FIRST THREE bytes of
// the data area — the bank byte and the two channel bytes — and the record
// proper is what follows it. So: 114-byte data area, 3-byte address, 111-byte
// record.
//
// NO IC-7100 HAS BEEN ASKED ANYTHING. Every number above descends from
// rasterised pages of one PDF, read by eye by agents who never opened this
// repository.

// The lengths derived above.
const (
	// addressLength is the bank byte plus the two channel bytes: fields (1) and
	// (2),(3), data-area bytes 1 to 3.
	addressLength = 3

	// dataAreaLength is everything a 1A 00 answer carries after its command and
	// sub-command bytes and before FD: the address and the record.
	dataAreaLength = 114

	// recordLength is the record proper, the data area less the address. It is
	// what a set frame's payload carries after the three address bytes, and
	// what a read answer's payload carries after them.
	recordLength = dataAreaLength - addressLength
)

// The blocks within the record, as offsets from the record's own first byte —
// which is data-area byte 4, field (4). A data-area position counted from 1
// becomes a record offset counted from 0 by dropping the address bytes and the
// 1-based origin: offset = position − 1 − addressLength.
const (
	// splitSelectOffset is field (4), data-area byte 4. This package does not
	// interpret it; it is named so that the tiling of the record is complete
	// and visible.
	splitSelectOffset = 0

	// rxBlockOffset / rxBlockLength are fields (5)–(51), data-area bytes 5–51:
	// the receive payload the printed NOTE says is duplicated.
	rxBlockOffset = 1
	rxBlockLength = 47

	// txBlockOffset / txBlockLength are the group printed 5f–51f, MEASURED at
	// data-area bytes 52–98. See the derivation above, point 2.
	txBlockOffset = rxBlockOffset + rxBlockLength
	txBlockLength = rxBlockLength

	// nameOffset / nameLength are the group whose printed index begins at (52),
	// MEASURED at data-area bytes 99–114, sixteen characters fixed. See the
	// derivation above, points 2 and 3.
	nameOffset = txBlockOffset + txBlockLength
	nameLength = 16
)

// The printed bank codes, field (1)'s whole value list: "01: A, 02: B, 03: C,
// 04: D, 05: E". Nothing else is printed against that byte anywhere.
const (
	firstBankCode = 0x01
	lastBankCode  = 0x05
)

// The printed memory-channel range this package serves. Field (2),(3)'s legend
// prints "0001–0099: Memory channel 1 to 99" and then ten further codes —
// 0100–0105 programmed scan edges, 0106–0109 call channels. Those ten are
// REFUSED rather than served, because the document never says what bank byte
// (1) carries for them and this fake will not invent one. See doc.go, entry 3.
const (
	firstChannel = 1
	lastChannel  = 99

	// firstSpecialChannel is where the printed special channels begin; it is
	// named so that a reader can see the refusal is deliberate and knows what
	// was refused.
	firstSpecialChannel = 100
	lastSpecialChannel  = 109
)

// clearFormMarker is the byte the "About clearing operation" block prints in
// field (4)'s place.
const clearFormMarker = 0xFF

// channelAddress renders a bank and a memory channel number as the three bytes
// that open a 1A 00 data block for that channel.
//
// The channel number is packed BCD: the legend prints four decimal digits and
// maps 0001–0099 onto "Memory channel 1 to 99", so channel 10 is 00 10 and not
// 00 0A. Both artefacts read the field that way and the G leg's own read
// request, FE FE 88 E0 1A 00 01 00 01 FD, is bank A channel 1 on that reading.
func channelAddress(bank, channel int) ([]byte, error) {
	if bank < firstBankCode || bank > lastBankCode {
		return nil, fmt.Errorf("bank %d is not one of the printed bank codes %02d (A) to %02d (E)", bank, firstBankCode, lastBankCode)
	}
	if channel >= firstSpecialChannel && channel <= lastSpecialChannel {
		return nil, fmt.Errorf("memory channel %04d is one of the printed special channels %04d-%04d (programmed scan edges and call channels), which this fake refuses because the document never states the bank byte they carry", channel, firstSpecialChannel, lastSpecialChannel)
	}
	if channel < firstChannel || channel > lastChannel {
		return nil, fmt.Errorf("memory channel %d is outside the printed range %04d-%04d", channel, firstChannel, lastChannel)
	}
	return []byte{byte(bank), 0x00, byte(channel/10)<<4 | byte(channel%10)}, nil
}

// addressIsInScope reports whether three wire bytes name a channel this fake
// serves: a printed bank code, and a packed-BCD channel number inside
// 0001–0099.
//
// A channel number whose nibbles are not decimal digits is not in scope at all
// rather than being decoded into something it is not — the field is printed as
// four decimal digits, so 0x9A is not a number this field can hold.
func addressIsInScope(addr []byte) bool {
	if len(addr) != addressLength {
		return false
	}
	if addr[0] < firstBankCode || addr[0] > lastBankCode {
		return false
	}
	channel, ok := decodeBCD2(addr[1], addr[2])
	if !ok {
		return false
	}
	return channel >= firstChannel && channel <= lastChannel
}

// decodeBCD2 reads two packed-BCD bytes as a four-digit decimal number,
// reporting false if any nibble is not a decimal digit.
func decodeBCD2(hi, lo byte) (int, bool) {
	n := 0
	for _, b := range [2]byte{hi, lo} {
		high, low := int(b>>4), int(b&0x0F)
		if high > 9 || low > 9 {
			return 0, false
		}
		n = n*100 + high*10 + low
	}
	return n, true
}

// describeAddress renders an address for a failure message or a panic, reading
// the channel number back as the packed BCD it is so that the number in a
// message is the number the caller passed in.
func describeAddress(addr []byte) string {
	if len(addr) != addressLength {
		return hexBytes(addr)
	}
	channel, ok := decodeBCD2(addr[1], addr[2])
	if !ok {
		return fmt.Sprintf("bank %02X, channel %02X%02X (not packed BCD)", addr[0], addr[1], addr[2])
	}
	return fmt.Sprintf("bank %02X, channel %04d", addr[0], channel)
}

// txBlockMatchesRX reports whether a record's transmit duplicate carries the
// same bytes as the receive payload it duplicates.
//
// This is the printed NOTE read as a rule — "The same data as (5)–(51) are
// stored in 5f–51f", "Even if the Split function is OFF, enter the data into
// 5f–51f to match your transceiver. We recommend that you set the same data as
// (5)–(51)." The document says the data ARE the same and RECOMMENDS entering
// them; it never says what the radio does with a set whose blocks differ. That
// is an assumption, it is registered, and WithUnequalTransmitBlockAccepted
// turns it off. See doc.go, entry 6 (ic7100-tx-block-mandatory).
func txBlockMatchesRX(record []byte) bool {
	if len(record) != recordLength {
		return false
	}
	for i := 0; i < rxBlockLength; i++ {
		if record[rxBlockOffset+i] != record[txBlockOffset+i] {
			return false
		}
	}
	return true
}

// isClearForm reports whether a 1A 00 data block is the printed clearing form.
//
// PDF p.375's "About clearing operation" block prints "(2), (3): Memory channel
// 0 to 99 / (4) : FF / (5) or later: None", and OMITS field (1) — so the
// document does not say whether a clear frame carries a bank byte. Both
// readings are recognised, and both are refused; see doc.go, entry 8.
func isClearForm(payload []byte) bool {
	switch len(payload) {
	case addressLength + 1:
		return payload[addressLength] == clearFormMarker
	case addressLength:
		// Two channel bytes then FF: the block's own field list, taken
		// literally, with no bank byte in front of it.
		return payload[addressLength-1] == clearFormMarker
	default:
		return false
	}
}
