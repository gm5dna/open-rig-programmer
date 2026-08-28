// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"testing"
)

func TestEncodeFrequencyBCD_KnownVectors(t *testing.T) {
	for _, tc := range []struct {
		name string
		hz   uint64
		n    int
		want []byte
	}{
		{
			// 14.250000 MHz over the five-byte form: little-endian packed
			// BCD, least significant PAIR first — 00 the tens/units, then
			// 00 hundreds/thousands, 25 the 10 kHz pair, 14 the MHz pair,
			// 00 the 100 MHz pair.
			name: "14.250 MHz, 5 bytes",
			hz:   14_250_000,
			n:    5,
			want: []byte{0x00, 0x00, 0x25, 0x14, 0x00},
		},
		{
			name: "1 Hz, 5 bytes",
			hz:   1,
			n:    5,
			want: []byte{0x01, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "the 5-byte ceiling, 9 999 999 999 Hz",
			hz:   9_999_999_999,
			n:    5,
			want: []byte{0x99, 0x99, 0x99, 0x99, 0x99},
		},
		{
			name: "zero, 5 bytes",
			hz:   0,
			n:    5,
			want: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			// The 905's six-byte form: twelve digits, so 10 GHz is
			// representable where the five-byte form stops at 9.99 GHz.
			name: "10 GHz, 6 bytes",
			hz:   10_000_000_000,
			n:    6,
			// 010000000000 as twelve digits: five zero pairs then 01.
			want: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name: "the 6-byte ceiling, 999 999 999 999 Hz",
			hz:   999_999_999_999,
			n:    6,
			want: []byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeFrequencyBCD(tc.hz, tc.n)
			if err != nil {
				t.Fatalf("EncodeFrequencyBCD(%d, %d) returned %v", tc.hz, tc.n, err)
			}
			if string(got) != string(tc.want) {
				t.Fatalf("EncodeFrequencyBCD(%d, %d) = % 02x, want % 02x", tc.hz, tc.n, got, tc.want)
			}
			back, err := DecodeFrequencyBCD(got)
			if err != nil {
				t.Fatalf("DecodeFrequencyBCD(% 02x) returned %v", got, err)
			}
			if back != tc.hz {
				t.Fatalf("round trip: DecodeFrequencyBCD(EncodeFrequencyBCD(%d, %d)) = %d", tc.hz, tc.n, back)
			}
		})
	}
}

// TestFrequencyBCD_ExhaustiveRoundTrip walks EVERY value the five-byte
// form can express at a stride that still visits every digit position, and
// every value of a narrow window exhaustively. A BCD codec that is right
// for a handful of hand-picked frequencies and wrong on a carry is the
// defect this exists to catch.
func TestFrequencyBCD_ExhaustiveRoundTrip(t *testing.T) {
	// Every value 0..99_999 exhaustively: that covers all carries in the
	// low three bytes, including the 9->0 rollovers.
	for hz := uint64(0); hz <= 99_999; hz++ {
		b, err := EncodeFrequencyBCD(hz, 5)
		if err != nil {
			t.Fatalf("EncodeFrequencyBCD(%d, 5): %v", hz, err)
		}
		got, err := DecodeFrequencyBCD(b)
		if err != nil {
			t.Fatalf("DecodeFrequencyBCD(% 02x) for %d: %v", b, hz, err)
		}
		if got != hz {
			t.Fatalf("round trip lost %d: got %d (% 02x)", hz, got, b)
		}
	}
	// Then a stride over the whole 10-digit space that lands on every
	// decade boundary and on values with a 9 in each position.
	for _, hz := range []uint64{
		1, 12, 123, 1234, 12345, 123456, 1234567, 12345678, 123456789,
		1234567890, 9_999_999_999, 9_000_000_000, 900_000_000, 90_000_000,
		9_000_000, 900_000, 90_000, 9_000, 900, 90, 9,
	} {
		for _, n := range []int{5, 6} {
			b, err := EncodeFrequencyBCD(hz, n)
			if err != nil {
				t.Fatalf("EncodeFrequencyBCD(%d, %d): %v", hz, n, err)
			}
			got, err := DecodeFrequencyBCD(b)
			if err != nil {
				t.Fatalf("DecodeFrequencyBCD(% 02x): %v", b, err)
			}
			if got != hz {
				t.Fatalf("round trip lost %d at width %d: got %d", hz, n, got)
			}
		}
	}
}

func TestEncodeFrequencyBCD_Refusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		hz   uint64
		n    int
	}{
		{"width 0", 1, 0},
		{"negative-shaped width", 1, -1},
		{"width past the ceiling", 1, maxBCDBytes + 1},
		{"value too wide for 5 bytes", 10_000_000_000, 5},
		{"value too wide for 6 bytes", 1_000_000_000_000, 6},
		{"value too wide for 1 byte", 100, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := EncodeFrequencyBCD(tc.hz, tc.n); err == nil {
				t.Fatalf("EncodeFrequencyBCD(%d, %d) = % 02x, nil — want an error", tc.hz, tc.n, got)
			}
		})
	}
}

func TestDecodeFrequencyBCD_RefusesNonBCDNibbles(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"high nibble 0xA", []byte{0x00, 0x00, 0xA0, 0x00, 0x00}},
		{"low nibble 0xF", []byte{0x0F, 0x00, 0x00, 0x00, 0x00}},
		{"the empty slice", nil},
		{"wider than the ceiling", make([]byte, maxBCDBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := DecodeFrequencyBCD(tc.in); err == nil {
				t.Fatalf("DecodeFrequencyBCD(% 02x) = %d, nil — want an error", tc.in, got)
			} else if !errors.Is(err, ErrBCD) {
				t.Fatalf("DecodeFrequencyBCD(% 02x) error %v does not match ErrBCD", tc.in, err)
			}
		})
	}
}

// TestBCD2_ExhaustiveRoundTrip walks the whole two-digit domain, both
// directions, plus every byte that is NOT a valid two-digit BCD pair.
func TestBCD2_ExhaustiveRoundTrip(t *testing.T) {
	for v := 0; v <= 99; v++ {
		b, err := EncodeBCD2(v)
		if err != nil {
			t.Fatalf("EncodeBCD2(%d): %v", v, err)
		}
		if hi, lo := b>>4, b&0x0F; int(hi) != v/10 || int(lo) != v%10 {
			t.Fatalf("EncodeBCD2(%d) = %#02x, want nibbles %d %d", v, b, v/10, v%10)
		}
		got, err := DecodeBCD2(b)
		if err != nil {
			t.Fatalf("DecodeBCD2(%#02x): %v", b, err)
		}
		if got != v {
			t.Fatalf("round trip lost %d: got %d", v, got)
		}
	}
	for _, v := range []int{-1, 100, 1000} {
		if b, err := EncodeBCD2(v); err == nil {
			t.Fatalf("EncodeBCD2(%d) = %#02x, nil — want an error", v, b)
		}
	}
	// Every byte outside the 00..99 BCD image must be refused, and every
	// byte inside it must be accepted: a codec that silently accepts 0x1A
	// as "1A" puts a byte no Icom document defines into a decoded field.
	valid := 0
	for i := 0; i < 256; i++ {
		b := byte(i)
		_, err := DecodeBCD2(b)
		isBCD := b>>4 <= 9 && b&0x0F <= 9
		if isBCD {
			if err != nil {
				t.Fatalf("DecodeBCD2(%#02x) refused a valid BCD pair: %v", b, err)
			}
			valid++
			continue
		}
		if err == nil {
			t.Fatalf("DecodeBCD2(%#02x) accepted a non-BCD byte", b)
		}
	}
	if valid != 100 {
		t.Fatalf("accepted %d bytes as valid BCD pairs, want exactly 100", valid)
	}
}

// TestEncodeBCDNumber_BothByteOrders pins that the two wire orders this
// package supports are genuinely different and each round-trips.
func TestEncodeBCDNumber_BothByteOrders(t *testing.T) {
	const v = 885 // an Icom CTCSS tone, 88.5 Hz in tenths

	little, err := encodeBCDNumber(v, 3, OrderLittleEndian)
	if err != nil {
		t.Fatalf("little-endian encode: %v", err)
	}
	big, err := encodeBCDNumber(v, 3, OrderBigEndian)
	if err != nil {
		t.Fatalf("big-endian encode: %v", err)
	}
	if want := []byte{0x85, 0x08, 0x00}; string(little) != string(want) {
		t.Fatalf("little-endian 885 over 3 bytes = % 02x, want % 02x", little, want)
	}
	if want := []byte{0x00, 0x08, 0x85}; string(big) != string(want) {
		t.Fatalf("big-endian 885 over 3 bytes = % 02x, want % 02x", big, want)
	}
	if string(little) == string(big) {
		t.Fatal("the two byte orders produced identical bytes — one of them is not being honoured")
	}
	for _, tc := range []struct {
		order ByteOrder
		bytes []byte
	}{{OrderLittleEndian, little}, {OrderBigEndian, big}} {
		got, err := decodeBCDNumber(tc.bytes, tc.order)
		if err != nil {
			t.Fatalf("decode % 02x under %v: %v", tc.bytes, tc.order, err)
		}
		if got != v {
			t.Fatalf("decode % 02x under %v = %d, want %d", tc.bytes, tc.order, got, v)
		}
	}
}
