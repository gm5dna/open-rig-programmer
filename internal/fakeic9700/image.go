// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"fmt"
)

// This file holds EVERYTHING this package knows about the IC-9700 memory
// record, and it is deliberately very little. Two facts, and no third:
//
//  1. WHERE A MEMORY REQUEST NAMES ITS CHANNEL. The first three bytes of a
//     1A 00 data block address a channel: a frequency-band byte, then a
//     two-byte memory channel number. Transcription B gives the meanings —
//     field ① "Frequency band setting", one byte, enum, "01: 144 MHz
//     frequency band | 02: 430 MHz frequency band | 03: 1.2 GHz frequency
//     band"; fields ②, ③ "Memory channel number", two bytes, packed BCD,
//     "0001 ~ 0099: Memory channel 1 to 99" through "0106, 0107: Call channel
//     C1, C2". The geometry witness gives the positions — D1 cell 1 for ①,
//     cells 2 and 3 for ②, ③, each cell split into two nibble halves. The two
//     artefacts AGREE here: B's measured byte positions 1 and 2-3 are the same
//     numbers as W's cell ordinals 1 and 2-3, because the witness records the
//     printed index and the measured position as diverging only from ⑩ onward.
//     Positions 1 to 3 are the only part of the record this package needs and
//     the only part on which the two legs are not in dispute.
//
//  2. WHAT THE CLEAR FORM LOOKS LIKE. Under the printed heading "To clear the
//     memory channel contents on 1A 00:" the page gives the same block with
//     ②, ③ as the memory channel, ④ as "FF," and ⑤ ~ as None. Both artefacts
//     put field ④ at position 4, immediately after the address; B carries the
//     ④ : "FF," entry in its ④ row and W quotes the whole line ④ : "FF,"
//     ⑤ ~ :None. So a clear is the three address bytes, one FF, and nothing
//     more. Field ① is not respecified by that heading, which respecifies only
//     the fields that change; it keeps the meaning the same block's own diagram
//     gives it, so the clear form still names its band.
//
// NOT here, and deliberately: any total record length, any offset past
// position 4, and the meaning of any other field. The two artefacts do not
// agree on how long a record is — see doc.go, THE RECORD LENGTH STOP — and
// this package is built so that it never has to know.

// Positions within the 1A 00 data block, 1-based, as both artefacts count
// them. They are stated as named constants rather than bare numbers so that
// anything that reads this file can see how few of them there are.
const (
	// bandFieldPosition is field ①, "Frequency band setting" (B), drawn as D1
	// cell 1 (W).
	bandFieldPosition = 1
	// channelFieldFirst and channelFieldLast are fields ②, ③, "Memory channel
	// number" (B), drawn as D1 cells 2 and 3 (W).
	channelFieldFirst = 2
	channelFieldLast  = 3
	// selectMemoryFieldPosition is field ④, drawn as D1 cell 4 (W), the field
	// the clearing instruction prints as "FF,".
	selectMemoryFieldPosition = 4
)

// channelAddressLen is how many bytes of a 1A 00 data block name the channel:
// the band byte and the two channel-number bytes, positions 1 to 3.
const channelAddressLen = channelFieldLast - bandFieldPosition + 1

// clearFormMarker is the byte the clearing instruction prints in field ④'s
// place, and clearFormLen is the whole length of that printed form: the
// address, that one byte, and nothing after it (⑤ ~ : None).
const (
	clearFormMarker = 0xFF
	clearFormLen    = selectMemoryFieldPosition
)

// The printed frequency band codes, transcription B's field ① value list.
const (
	bandCode144MHz = 0x01
	bandCode430MHz = 0x02
	bandCode1200   = 0x03
)

// The printed extent of the memory channel number, transcription B's ②, ③
// value list: 0001 ~ 0099 are memory channels, 0100 to 0105 are the three
// Program Scan Edge channel pairs, 0106 and 0107 are the two Call channels.
// Nothing outside 0001 ~ 0107 is printed against the field.
const (
	firstPrintedChannel = 1
	lastPrintedChannel  = 107
)

// recordPadByte is what a record is padded with when WithRecordLength asks for
// answers longer than what was seeded. It is an INVENTION of this fake and is
// not claimed to be what any slot of any IC-9700 holds; it is only a byte that
// is neither the preamble nor the end-of-message byte, so that a padded answer
// is still a legal frame.
const recordPadByte = 0x00

// channelAddress renders a band and a memory channel number as the three bytes
// that open a 1A 00 data block for that channel.
//
// The channel number is packed BCD, which is how transcription B reads the
// field: its printed values are four decimal digits, and its two byte cells are
// each split by a dotted rule into two nibble cells. So decimal 0107 is 01 07,
// not 00 6B.
func channelAddress(band, channel int) ([]byte, error) {
	switch band {
	case bandCode144MHz, bandCode430MHz, bandCode1200:
	default:
		return nil, fmt.Errorf("band %d is not one of the printed frequency band codes 1 (144 MHz), 2 (430 MHz) or 3 (1.2 GHz)", band)
	}
	if channel < firstPrintedChannel || channel > lastPrintedChannel {
		return nil, fmt.Errorf("memory channel %d is outside the printed range %04d ~ %04d", channel, firstPrintedChannel, lastPrintedChannel)
	}

	d := [4]byte{
		byte(channel / 1000 % 10),
		byte(channel / 100 % 10),
		byte(channel / 10 % 10),
		byte(channel % 10),
	}
	return []byte{byte(band), d[0]<<4 | d[1], d[2]<<4 | d[3]}, nil
}

// describeAddress renders an address for a failure message or a panic. It reads
// the channel number back as the packed BCD it is, which is the only way the
// number in a message matches the number the caller passed in.
func describeAddress(addr []byte) string {
	if len(addr) != channelAddressLen {
		return hexBytes(addr)
	}
	return fmt.Sprintf("band %02X, channel %02X%02X", addr[0], addr[1], addr[2])
}

// isClearForm reports whether a 1A 00 data block is the printed clearing form:
// the channel address, then FF where field ④ stands, and nothing after it.
func isClearForm(payload []byte) bool {
	return len(payload) == clearFormLen && payload[selectMemoryFieldPosition-1] == clearFormMarker
}

// fitRecord returns rec cut or padded to exactly n bytes. Padding is
// recordPadByte, an invention; see its comment.
func fitRecord(rec []byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = recordPadByte
	}
	copy(out, rec)
	return out
}

// slot is one memory channel of the fake's image. A slot that is not occupied
// answers the NG code even though it exists, which is what WithEmptySlot asks
// for and what a channel that was never seeded gets anyway.
type slot struct {
	record   []byte
	occupied bool
}

// image is the fake's memory: a set of channels keyed by their three address
// bytes, plus the length of record it serves.
//
// The key is the raw address bytes as they arrived on the wire, not a decoded
// (band, channel) pair. That is on purpose: a probe naming a channel this
// package would refuse to seed — a band code the page never prints, or a
// channel-number byte that is not valid BCD — simply misses the map and gets
// the NG code, rather than being decoded into something it is not.
type image struct {
	slots map[string]slot

	// served is the record length every memory answer carries, or 0 for "no
	// length is known, so none is enforced". explicit records whether it was
	// SET rather than inferred from what was seeded.
	served   int
	explicit bool
}

func newImage() *image { return &image{slots: make(map[string]slot)} }

// seed installs one slot before the radio starts answering.
func (i *image) seed(addr []byte, record []byte, occupied bool) {
	i.slots[string(addr)] = slot{record: append([]byte(nil), record...), occupied: occupied}
}

// setServedLength fixes the record length the image serves. n <= 0 means no
// explicit length was asked for, in which case the length is inferred: if every
// occupied slot holds the same number of bytes, that is the length served, and
// otherwise no length is known and none is enforced.
//
// This is the whole of this package's answer to not knowing how long a record
// is. It never needs to know: it serves what it was handed, and it can only
// judge a write's length once something has told it one.
func (i *image) setServedLength(n int) {
	if n > 0 {
		i.served, i.explicit = n, true
		return
	}
	i.explicit = false
	i.served = 0
	first := true
	for _, s := range i.slots {
		if !s.occupied {
			continue
		}
		switch {
		case first:
			i.served = len(s.record)
			first = false
		case len(s.record) != i.served:
			i.served = 0
			return
		}
	}
}

// servedLength is the record length every memory answer carries and every
// memory write must match, or 0 if no length is known.
func (i *image) servedLength() int { return i.served }

// read returns a copy of the record at addr, and whether that channel is
// occupied at all.
func (i *image) read(addr []byte) ([]byte, bool) {
	s, ok := i.slots[string(addr)]
	if !ok || !s.occupied {
		return nil, false
	}
	return append([]byte(nil), s.record...), true
}

// write stores a record at addr and marks the channel occupied.
func (i *image) write(addr []byte, record []byte) {
	i.slots[string(addr)] = slot{record: append([]byte(nil), record...), occupied: true}
}
