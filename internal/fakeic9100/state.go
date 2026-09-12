package fakeic9100

import "strconv"

// This file holds the AddressFormBankChannel selector this package serves —
// one band byte, then a two-byte packed-BCD channel number — and the
// caller-facing channel-name spelling built on it. Nothing here interprets
// the 57-byte record that follows a selector: doc.go's "record is OPAQUE"
// applies to every caller of this file too.

// MemState is the record the fake holds for one channel.
type MemState struct{ Raw []byte }

// The record's own printed band values (matrix §1b, field `q`): "00: HF/50
// MHz frequency band / 01: 144 MHz frequency band / 02: 430 MHz frequency
// band / 03: 1200 MHz frequency band". All four are accepted regardless of
// whether the optional UX-9100 1200 MHz unit is fitted — the wire byte
// carries the same value either way, and this fake has no notion of which
// bands a particular radio is equipped with.
const (
	bandHF   = 0x00
	band144  = 0x01
	band430  = 0x02
	band1200 = 0x03
)

// bandCode maps the matrix's proposed slot-string band name (§1b, "Slot
// string") to its wire value.
func bandCode(name string) (byte, bool) {
	switch name {
	case "HF":
		return bandHF, true
	case "144":
		return band144, true
	case "430":
		return band430, true
	case "1200":
		return band1200, true
	}
	return 0, false
}

// parseChannel maps a caller's "<band>-<channel>" spelling (e.g. "HF-001",
// "1200-099") to the same slot key the wire selector decodes to, so
// SetSlot("144-045", …) and a 1A 00 read of band 01 channel 0045 name one
// record. The channel half is three decimal digits, 001-099, mirroring the
// record's own packed-BCD range.
func parseChannel(addr string) (int, bool) {
	if len(addr) < 5 || addr[len(addr)-4] != '-' {
		return 0, false
	}
	band, chStr := addr[:len(addr)-4], addr[len(addr)-3:]
	b, ok := bandCode(band)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(chStr)
	if err != nil || len(chStr) != 3 || n < 1 || n > 99 {
		return 0, false
	}
	return slotKey(b, n), true
}

// slotKey is the flat map key for one band+channel pair.
func slotKey(band byte, channel int) int { return int(band)*1000 + channel }

// selector decodes the wire's 3-byte AddressFormBankChannel selector —
// band byte, then packed-BCD channel — into the same slot key parseChannel
// produces. The scan-edge and call-channel codes (0100-0106 per band) are
// refused: the matrix shows they share this same address form (§1b,
// "Banks"), but they are out of scope this cycle (wave spec §1), and this
// fake will not invent record content for a bank it was never asked to
// serve.
func selector(b []byte) (int, bool) {
	if len(b) != 3 || b[0] > band1200 {
		return 0, false
	}
	channel, ok := decodeBCD2(b[1], b[2])
	if !ok || channel < 1 || channel > 99 {
		return 0, false
	}
	return slotKey(b[0], channel), true
}

// decodeBCD2 reads two packed-BCD bytes as a four-digit decimal number,
// reporting false if any nibble is not a decimal digit — the channel field
// is printed as four decimal digits, so a non-BCD byte pair names no
// channel this field can hold.
func decodeBCD2(hi, lo byte) (int, bool) {
	n := 0
	for _, d := range [4]byte{hi >> 4, hi & 0x0F, lo >> 4, lo & 0x0F} {
		if d > 9 {
			return 0, false
		}
		n = n*10 + int(d)
	}
	return n, true
}
