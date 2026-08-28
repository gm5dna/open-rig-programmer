// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300

import "fmt"

// The channel address space, and the WHOLE of it.
//
// The band's first field, `①, ②` "Memory channel numbers", is the only place
// the page enumerates channel addresses, and it prints three forms and no
// fourth:
//
//	00 01–00 99: Memory channel 01 to 99
//	01 00: Programmed scan edge P1
//	01 01: Programmed scan edge P2
//
// The clear-command list in the right-hand column reprints the first of them as
// `(00 01~00 99)` and names no other. THREE FORMS, AND NO FOURTH: `00 00` is
// not one of them (the legend's range opens at 01), and neither is any address
// whose second byte is not two BCD digits. A read or a set naming anything else
// answers NG.
//
// The canonical slot string this package keys its map by is "001".."099" for
// the first form and "P1"/"P2" for the other two — the shortest spelling that
// is one-to-one with the printed codes.
const (
	firstMemoryChannel = 1
	lastMemoryChannel  = 99

	memoryPageByte = 0x00 // the high byte of `00 NN`
	edgePageByte   = 0x01 // the high byte of `01 00` and `01 01`
	edgeP1Byte     = 0x00
	edgeP2Byte     = 0x01

	slotP1 = "P1"
	slotP2 = "P2"
)

// bcdPair decodes one packed-BCD byte, reporting false if either nibble is not
// a decimal digit. The channel range is printed as `00 01–00 99` — two decimal
// digits per byte — so `00 1A` is not an address in it.
func bcdPair(b byte) (int, bool) {
	hi, lo := int(b>>nibbleWidth), int(b&nibbleMask)
	if hi > 9 || lo > 9 {
		return 0, false
	}
	return hi*10 + lo, true
}

// canonicalSlot turns the two channel-address bytes a `1A 00` frame carries
// into this package's canonical slot string, reporting false for anything the
// legend does not print.
func canonicalSlot(hi, lo byte) (string, bool) {
	switch hi {
	case memoryPageByte:
		n, ok := bcdPair(lo)
		if !ok || n < firstMemoryChannel || n > lastMemoryChannel {
			return "", false
		}
		return fmt.Sprintf("%03d", n), true
	case edgePageByte:
		switch lo {
		case edgeP1Byte:
			return slotP1, true
		case edgeP2Byte:
			return slotP2, true
		}
	}
	return "", false
}

// slotAddressBytes is canonicalSlot's inverse: the two wire bytes for a
// canonical slot string, or false if slot is not one of the three printed
// forms. It is what puts the address back into a read's answer, and what the
// seeding options validate against.
func slotAddressBytes(slot string) (byte, byte, bool) {
	switch slot {
	case slotP1:
		return edgePageByte, edgeP1Byte, true
	case slotP2:
		return edgePageByte, edgeP2Byte, true
	}
	if len(slot) != 3 {
		return 0, 0, false
	}
	n := 0
	for i := 0; i < 3; i++ {
		c := slot[i]
		if c < '0' || c > '9' {
			return 0, 0, false
		}
		n = n*10 + int(c-'0')
	}
	if n < firstMemoryChannel || n > lastMemoryChannel {
		return 0, 0, false
	}
	return memoryPageByte, byte(n/10)<<nibbleWidth | byte(n%10), true
}
