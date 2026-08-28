// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"bytes"
	"testing"
)

// TestChannelAddress walks the whole printed value list of the memory channel
// number field, transcription B's ②, ③ row, one case per printed line — and
// the band codes are transcription B's ① row, likewise all three of them.
//
// The expected bytes are written out by hand from the printed decimal values
// (0001 ~ 0107) under the packed-BCD reading B records for the field: four
// decimal digits across two byte cells, each cell split by a dotted rule into
// two nibble cells.
func TestChannelAddress(t *testing.T) {
	tests := []struct {
		name    string
		band    int
		channel int
		want    []byte
	}{
		{"144 MHz, memory channel 1 — first of 0001 ~ 0099", 1, 1, []byte{0x01, 0x00, 0x01}},
		{"144 MHz, memory channel 9 — last single BCD digit", 1, 9, []byte{0x01, 0x00, 0x09}},
		{"144 MHz, memory channel 10 — the first carry into the upper nibble", 1, 10, []byte{0x01, 0x00, 0x10}},
		{"430 MHz, memory channel 42", 2, 42, []byte{0x02, 0x00, 0x42}},
		{"1.2 GHz, memory channel 99 — last of 0001 ~ 0099", 3, 99, []byte{0x03, 0x00, 0x99}},
		{"144 MHz, 0100 — Program Scan Edge channel 1A", 1, 100, []byte{0x01, 0x01, 0x00}},
		{"144 MHz, 0101 — Program Scan Edge channel 1B", 1, 101, []byte{0x01, 0x01, 0x01}},
		{"430 MHz, 0102 — Program Scan Edge channel 2A", 2, 102, []byte{0x02, 0x01, 0x02}},
		{"430 MHz, 0103 — Program Scan Edge channel 2B", 2, 103, []byte{0x02, 0x01, 0x03}},
		{"1.2 GHz, 0104 — Program Scan Edge channel 3A", 3, 104, []byte{0x03, 0x01, 0x04}},
		{"1.2 GHz, 0105 — Program Scan Edge channel 3B", 3, 105, []byte{0x03, 0x01, 0x05}},
		{"144 MHz, 0106 — Call channel C1", 1, 106, []byte{0x01, 0x01, 0x06}},
		{"144 MHz, 0107 — Call channel C2", 1, 107, []byte{0x01, 0x01, 0x07}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := channelAddress(tt.band, tt.channel)
			if err != nil {
				t.Fatalf("channelAddress(%d, %d): %v", tt.band, tt.channel, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("channelAddress(%d, %d) = % 02X, want % 02X", tt.band, tt.channel, got, tt.want)
			}
			if len(got) != channelAddressLen {
				t.Errorf("address is %d bytes, want %d — the band byte at position 1 and the two channel-number bytes at positions 2 and 3", len(got), channelAddressLen)
			}
		})
	}
}

// TestChannelAddress_RefusesWhatThePageDoesNotPrint. The band field is an
// enum_byte with a closed printed list (01, 02, 03) and the channel number's
// printed values stop at 0107. Seeding outside either is a caller's mistake,
// and the fake says so rather than inventing a slot the page does not describe.
func TestChannelAddress_RefusesWhatThePageDoesNotPrint(t *testing.T) {
	tests := []struct {
		name    string
		band    int
		channel int
	}{
		{"band 0", 0, 1},
		{"band 4 — one past the printed 03", 4, 1},
		{"band 0x03 read as something else entirely", 300, 1},
		{"channel 0 — below the printed 0001", 1, 0},
		{"channel 108 — one past the printed 0107", 1, 108},
		{"channel 9999", 1, 9999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := channelAddress(tt.band, tt.channel); err == nil {
				t.Errorf("channelAddress(%d, %d) succeeded, want a refusal", tt.band, tt.channel)
			}
		})
	}
}

// TestDescribeAddress is only for failure messages, but a wrong one makes every
// other failure harder to read, so it is pinned.
func TestDescribeAddress(t *testing.T) {
	if got, want := describeAddress([]byte{0x02, 0x01, 0x07}), "band 02, channel 0107"; got != want {
		t.Errorf("describeAddress = %q, want %q", got, want)
	}
	if got, want := describeAddress([]byte{0x01}), "01"; got != want {
		t.Errorf("describeAddress = %q, want %q", got, want)
	}
}

// TestIsClearForm pins the one memory-write shape this fake refuses. The page
// prints it under the heading "To clear the memory channel contents on 1A 00:"
// as ②, ③ : Memory channel (0001~0099), ④ : "FF," and ⑤ ~ : None — that is,
// the channel address, then FF in field ④'s place, and nothing after it. Both
// artefacts put field ④ at position 4, immediately after the three address
// bytes.
func TestIsClearForm(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"address then FF and nothing after — the printed clear form", []byte{0x01, 0x00, 0x07, 0xFF}, true},
		{"the clear form on another band", []byte{0x03, 0x01, 0x07, 0xFF}, true},
		{"address alone — a read, not a clear", []byte{0x01, 0x00, 0x07}, false},
		{"FF in field 4 but more data after it — a write, not a clear", []byte{0x01, 0x00, 0x07, 0xFF, 0x00}, false},
		{"a one-byte record that is not FF", []byte{0x01, 0x00, 0x07, 0x00}, false},
		{"too short to carry an address", []byte{0x01, 0x00}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClearForm(tt.payload); got != tt.want {
				t.Errorf("isClearForm(% 02X) = %v, want %v", tt.payload, got, tt.want)
			}
		})
	}
}

func TestImage_ReadWriteAndEmptySlots(t *testing.T) {
	addrA := mustAddress(t, 1, 7)
	addrB := mustAddress(t, 2, 7)

	img := newImage()
	img.seed(addrA, []byte{0x11, 0x22}, true)
	img.seed(addrB, nil, false)

	if rec, ok := img.read(addrA); !ok || !bytes.Equal(rec, []byte{0x11, 0x22}) {
		t.Errorf("read(seeded) = % 02X, %v; want 11 22, true", rec, ok)
	}
	if _, ok := img.read(addrB); ok {
		t.Error("read(explicitly empty) reported occupied")
	}
	if _, ok := img.read(mustAddress(t, 3, 7)); ok {
		t.Error("read(never seeded) reported occupied")
	}

	img.write(addrB, []byte{0x33})
	if rec, ok := img.read(addrB); !ok || !bytes.Equal(rec, []byte{0x33}) {
		t.Errorf("after write, read = % 02X, %v; want 33, true", rec, ok)
	}
}

// TestImage_ReadDoesNotAliasStoredBytes: an answer handed to the wire must not
// be a window onto the image, or a later write would rewrite an answer already
// sent.
func TestImage_ReadDoesNotAliasStoredBytes(t *testing.T) {
	addr := mustAddress(t, 1, 1)
	img := newImage()
	img.seed(addr, []byte{0x11, 0x22}, true)

	rec, _ := img.read(addr)
	rec[0] = 0x99
	again, _ := img.read(addr)
	if again[0] != 0x11 {
		t.Errorf("read handed out the stored slice: second read = % 02X", again)
	}
}

// TestImage_ServedLength covers the one length rule this fake can have without
// knowing a record length: it serves the length it was given, and only then can
// it judge a write's length.
func TestImage_ServedLength(t *testing.T) {
	t.Run("nothing seeded, no explicit length — unconstrained", func(t *testing.T) {
		img := newImage()
		img.setServedLength(0)
		if got := img.servedLength(); got != 0 {
			t.Errorf("servedLength = %d, want 0 (unconstrained)", got)
		}
	})
	t.Run("one seeded slot sets the served length", func(t *testing.T) {
		img := newImage()
		img.seed(mustAddress(t, 1, 1), make([]byte, 5), true)
		img.setServedLength(0)
		if got := img.servedLength(); got != 5 {
			t.Errorf("servedLength = %d, want 5", got)
		}
	})
	t.Run("seeded slots of differing lengths leave it unconstrained", func(t *testing.T) {
		img := newImage()
		img.seed(mustAddress(t, 1, 1), make([]byte, 5), true)
		img.seed(mustAddress(t, 1, 2), make([]byte, 6), true)
		img.setServedLength(0)
		if got := img.servedLength(); got != 0 {
			t.Errorf("servedLength = %d, want 0 — two seeded lengths agree on nothing", got)
		}
	})
	t.Run("an explicit length overrides what was seeded", func(t *testing.T) {
		img := newImage()
		img.seed(mustAddress(t, 1, 1), make([]byte, 5), true)
		img.setServedLength(9)
		if got := img.servedLength(); got != 9 {
			t.Errorf("servedLength = %d, want 9", got)
		}
	})
	t.Run("an empty slot contributes no length", func(t *testing.T) {
		img := newImage()
		img.seed(mustAddress(t, 1, 1), nil, false)
		img.setServedLength(0)
		if got := img.servedLength(); got != 0 {
			t.Errorf("servedLength = %d, want 0", got)
		}
	})
}

func TestFitRecord(t *testing.T) {
	tests := []struct {
		name string
		rec  []byte
		n    int
		want []byte
	}{
		{"already the served length", []byte{0x11, 0x22}, 2, []byte{0x11, 0x22}},
		{"short — padded", []byte{0x11}, 3, []byte{0x11, 0x00, 0x00}},
		{"long — truncated", []byte{0x11, 0x22, 0x33}, 2, []byte{0x11, 0x22}},
		{"zero length", []byte{0x11}, 0, []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fitRecord(tt.rec, tt.n); !bytes.Equal(got, tt.want) {
				t.Errorf("fitRecord(% 02X, %d) = % 02X, want % 02X", tt.rec, tt.n, got, tt.want)
			}
		})
	}
}

func mustAddress(t *testing.T, band, channel int) []byte {
	t.Helper()
	addr, err := channelAddress(band, channel)
	if err != nil {
		t.Fatalf("channelAddress(%d, %d): %v", band, channel, err)
	}
	return addr
}
