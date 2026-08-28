// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"fmt"
)

// ErrBCD is the sentinel every packed-BCD failure in this package wraps:
// a value too wide for its field, a field width this package will not
// build, or a wire byte carrying a nibble above 9.
//
// One sentinel rather than three because a caller's only sensible reaction
// to any of them is the same — refuse the frame — and errors.Is on a
// single name is what the gate and the parsers actually ask.
var ErrBCD = errors.New("civ: packed BCD")

// maxBCDBytes is the widest packed-BCD field this package encodes or
// decodes.
//
// DERIVED, not chosen: decodeBCDNumber returns a uint64, so the widest
// field whose every representable value fits is nine bytes (18 digits,
// max 999 999 999 999 999 999 < 2^64). Ten bytes could overflow silently,
// which is the one failure mode a numeric codec must not have. Every
// documented Icom field is far below this — the widest is the IC-905's
// six-byte frequency — so the ceiling exists to bound the arithmetic,
// not to express a protocol fact.
const maxBCDBytes = 9

// ByteOrder names which end of a packed-BCD field carries the least
// significant digit pair.
//
// Both exist because CI-V uses both and the choice is per FIELD, not per
// radio: the frequency fields are documented least-significant-pair first,
// while the tone and DTCS fields are drawn most-significant-pair first.
// A profile's layout table declares the order for each field it maps, so
// nothing in this package has to remember which is which.
//
// The zero value is OrderUnspecified, deliberately: a layout that omits
// the order is refused by profile validation rather than silently given
// the frequency fields' convention.
type ByteOrder int

const (
	// OrderUnspecified is the zero value: never a wire order. See ByteOrder.
	OrderUnspecified ByteOrder = iota
	// OrderLittleEndian puts the least significant digit pair in byte 0 —
	// the CI-V frequency convention.
	OrderLittleEndian
	// OrderBigEndian puts the most significant digit pair in byte 0.
	OrderBigEndian
)

func (o ByteOrder) String() string {
	switch o {
	case OrderUnspecified:
		return "OrderUnspecified"
	case OrderLittleEndian:
		return "OrderLittleEndian"
	case OrderBigEndian:
		return "OrderBigEndian"
	default:
		return fmt.Sprintf("ByteOrder(%d)", int(o))
	}
}

// EncodeFrequencyBCD renders hz as an n-byte little-endian packed-BCD
// field — the CI-V frequency form: two decimal digits per byte, least
// significant pair first.
//
// n is the FIELD WIDTH in bytes, and the two widths this tier meets are 5
// (ten digits, up to 9.999 999 999 GHz) and 6 (twelve digits, which is
// what the IC-905 needs to reach 10 GHz). Any width from 1 to maxBCDBytes
// is accepted so a per-model layout can declare a narrower numeric field
// through the same helper.
//
// It REFUSES a value that does not fit rather than truncating it. A
// truncated frequency is a plausible-looking frame naming a different
// channel, and this package's whole safety story is that a frame it built
// says what the caller asked for.
func EncodeFrequencyBCD(hz uint64, n int) ([]byte, error) {
	return encodeBCDNumber(hz, n, OrderLittleEndian)
}

// DecodeFrequencyBCD reads an n-byte little-endian packed-BCD frequency
// field back to Hz. It refuses any byte carrying a nibble above 9: those
// are not BCD, and guessing at them would turn line noise into a
// frequency.
func DecodeFrequencyBCD(b []byte) (uint64, error) {
	return decodeBCDNumber(b, OrderLittleEndian)
}

// EncodeBCD2 renders v (0..99) as ONE packed-BCD byte: tens in the high
// nibble, units in the low. The two-digit field is CI-V's unit of address
// and count — a channel number's halves, a group index, a bank number.
func EncodeBCD2(v int) (byte, error) {
	if v < 0 || v > 99 {
		return 0, fmt.Errorf("%w: value %d does not fit a 2-digit field (want 0..99)", ErrBCD, v)
	}
	return byte(v/10)<<4 | byte(v%10), nil
}

// DecodeBCD2 reads one packed-BCD byte back to 0..99, refusing any byte
// with a nibble above 9.
func DecodeBCD2(b byte) (int, error) {
	hi, lo := b>>4, b&0x0F
	if hi > 9 || lo > 9 {
		return 0, fmt.Errorf("%w: byte %#02x is not a 2-digit packed-BCD pair (a nibble exceeds 9)", ErrBCD, b)
	}
	return int(hi)*10 + int(lo), nil
}

// encodeBCDNumber is the one packed-BCD encoder every exported helper and
// every layout field routes through: value, field width, byte order.
//
// It allocates a fresh slice per call, so nothing it returns aliases a
// caller's buffer or a previously returned field.
func encodeBCDNumber(v uint64, n int, order ByteOrder) ([]byte, error) {
	if n < 1 || n > maxBCDBytes {
		return nil, fmt.Errorf("%w: field width %d is outside 1..%d", ErrBCD, n, maxBCDBytes)
	}
	switch order {
	case OrderLittleEndian, OrderBigEndian:
	default:
		return nil, fmt.Errorf("%w: byte order %v is not a wire order", ErrBCD, order)
	}

	out := make([]byte, n)
	rest := v
	for i := 0; i < n; i++ {
		pair := rest % 100
		rest /= 100
		b := byte(pair/10)<<4 | byte(pair%10)
		if order == OrderLittleEndian {
			out[i] = b
		} else {
			out[n-1-i] = b
		}
	}
	if rest != 0 {
		return nil, fmt.Errorf("%w: value %d needs more than the %d bytes (%d digits) of its field", ErrBCD, v, n, 2*n)
	}
	return out, nil
}

// decodeBCDNumber is the one packed-BCD decoder, mirroring
// encodeBCDNumber. It refuses an empty field, an over-wide field and any
// non-BCD nibble.
func decodeBCDNumber(b []byte, order ByteOrder) (uint64, error) {
	if len(b) < 1 || len(b) > maxBCDBytes {
		return 0, fmt.Errorf("%w: field of %d bytes is outside 1..%d", ErrBCD, len(b), maxBCDBytes)
	}
	switch order {
	case OrderLittleEndian, OrderBigEndian:
	default:
		return 0, fmt.Errorf("%w: byte order %v is not a wire order", ErrBCD, order)
	}

	var out uint64
	// Walk most significant pair first, whichever end that is.
	for i := 0; i < len(b); i++ {
		var by byte
		if order == OrderLittleEndian {
			by = b[len(b)-1-i]
		} else {
			by = b[i]
		}
		pair, err := DecodeBCD2(by)
		if err != nil {
			return 0, fmt.Errorf("%w (at byte %d of a %d-byte field)", err, i, len(b))
		}
		out = out*100 + uint64(pair)
	}
	return out, nil
}
