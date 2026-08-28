// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"math"
	"testing"
)

// TestThreeByteAddressWireBytesArePinned is E4's no-change guarantee, and
// it is deliberately written as LITERAL BYTES rather than as a round-trip.
// The tier adds a second grouped address form and moves the group index
// off a hardcoded zero base; neither may alter one byte of what the
// existing forms put on a wire.
//
// A round-trip test would pass against any self-consistent encoding,
// including a changed one. These are the frames themselves.
func TestThreeByteAddressWireBytesArePinned(t *testing.T) {
	cases := []struct {
		name string
		p    Profile
		addr ChannelAddress
		want []byte
	}{
		{
			name: "flat: two packed-BCD channel bytes, most significant first",
			p:    flatProfile,
			addr: ChannelAddress{Channel: 12},
			want: []byte{0xFE, 0xFE, 0x94, 0xE0, 0x1A, 0x00, 0x00, 0x12, 0xFD},
		},
		{
			name: "group x channel: one BCD group byte before the pair",
			p:    groupProfile,
			addr: ChannelAddress{Group: 1, Channel: 12},
			want: []byte{0xFE, 0xFE, 0x70, 0xE1, 0x1A, 0x00, 0x01, 0x00, 0x12, 0xFD},
		},
		{
			name: "group x channel: the last group and the last channel",
			p:    groupProfile,
			addr: ChannelAddress{Group: 99, Channel: 99},
			want: []byte{0xFE, 0xFE, 0x70, 0xE1, 0x1A, 0x00, 0x99, 0x00, 0x99, 0xFD},
		},
		{
			name: "band x channel: same width, different meaning",
			p:    bandProfile,
			addr: ChannelAddress{Group: 2, Channel: 99},
			want: []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, 0x02, 0x00, 0x99, 0xFD},
		},
		{
			name: "group zero is still group zero",
			p:    groupProfile,
			addr: ChannelAddress{Group: 0, Channel: 0},
			want: []byte{0xFE, 0xFE, 0x70, 0xE1, 0x1A, 0x00, 0x00, 0x00, 0x00, 0xFD},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := tc.p.BuildMemoryRead(tc.addr)
			if err != nil {
				t.Fatalf("BuildMemoryRead(%v): %v", tc.addr, err)
			}
			if got := cmd.Bytes(); string(got) != string(tc.want) {
				t.Errorf("BuildMemoryRead(%v) = % x, want % x", tc.addr, got, tc.want)
			}
		})
	}
}

// TestWideGroupChannelWireBytes is the new form's own vector set: a
// TWO-byte packed-BCD group index before the two-byte channel.
//
// THE HUNDREDTH GROUP IS THE WHOLE REASON THE FORM EXISTS. The IC-705 and
// IC-905 both number a CALL group at wire index 100, which one packed-BCD
// byte cannot hold; `01 00` is what the radio prints and sends, and it is
// what this form encodes.
func TestWideGroupChannelWireBytes(t *testing.T) {
	p := wideProfile
	cases := []struct {
		name string
		addr ChannelAddress
		want []byte
	}{
		{
			name: "the base group",
			addr: ChannelAddress{Group: 1, Channel: 1},
			want: []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0x00, 0x01, 0xFD},
		},
		{
			name: "a two-digit group",
			addr: ChannelAddress{Group: 42, Channel: 12},
			want: []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x00, 0x42, 0x00, 0x12, 0xFD},
		},
		{
			name: "the hundredth group, which one BCD byte cannot hold",
			addr: ChannelAddress{Group: 100, Channel: 99},
			want: []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x99, 0xFD},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := p.BuildMemoryRead(tc.addr)
			if err != nil {
				t.Fatalf("BuildMemoryRead(%v): %v", tc.addr, err)
			}
			if got := cmd.Bytes(); string(got) != string(tc.want) {
				t.Errorf("BuildMemoryRead(%v) = % x, want % x", tc.addr, got, tc.want)
			}
		})
	}
}

// TestGroupIndexIsTheWireIndex is E4's decided semantics. Group is what
// the RADIO prints and sends, not an offset this package invented: a model
// numbering its groups from 1 has no group 0, and asking for one must be
// refused rather than silently rewritten into the model's first group.
func TestGroupIndexIsTheWireIndex(t *testing.T) {
	p := wideProfile
	base := p.GroupBase()
	if base != 1 {
		t.Fatalf("fixture error: wideProfile's GroupBase is %d, want 1 — this test cannot tell a base from a zero otherwise", base)
	}

	if _, err := p.BuildMemoryRead(ChannelAddress{Group: 0, Channel: 1}); err == nil {
		t.Error("BuildMemoryRead built a frame for group 0 on a profile whose groups start at 1")
	}
	last := base + p.Groups() - 1
	if _, err := p.BuildMemoryRead(ChannelAddress{Group: last, Channel: 1}); err != nil {
		t.Errorf("BuildMemoryRead(group %d): %v — base+Groups-1 is the last valid index", last, err)
	}
	if _, err := p.BuildMemoryRead(ChannelAddress{Group: last + 1, Channel: 1}); err == nil {
		t.Errorf("BuildMemoryRead built a frame for group %d, one past this profile's last", last+1)
	}
}

// TestGroupBaseDefaultsToZeroForTheExistingForms pins the other side of
// the same change: a profile that says nothing about a base is numbered
// from 0, exactly as every profile was before E4.
func TestGroupBaseDefaultsToZeroForTheExistingForms(t *testing.T) {
	for _, np := range []namedProfile{{"groupProfile", groupProfile}, {"bandProfile", bandProfile}} {
		t.Run(np.name, func(t *testing.T) {
			if got := np.p.GroupBase(); got != 0 {
				t.Errorf("GroupBase() = %d, want 0 for a profile that declares none", got)
			}
			if _, err := np.p.BuildMemoryRead(ChannelAddress{Group: 0, Channel: 1}); err != nil {
				t.Errorf("BuildMemoryRead(group 0): %v — a zero-based profile has a group 0", err)
			}
		})
	}
	if got := flatProfile.GroupBase(); got != 0 {
		t.Errorf("flatProfile.GroupBase() = %d, want 0 — a flat form has no group at all", got)
	}
}

// TestValidate_GroupSpaceMustFitTheFormsBCDWidth is the V-validator E4
// adds: base + count − 1 must be an index the form's own BCD width can
// encode. Without it a profile could declare a group space its own
// builders would refuse one frame at a time, at the far end of a read.
func TestValidate_GroupSpaceMustFitTheFormsBCDWidth(t *testing.T) {
	base := func() ProfileConfig {
		cfg := ProfileConfig{
			Model:         "TEST-V",
			RadioAddress:  0x88,
			MaxFrame:      64,
			AddressForm:   AddressFormGroupChannel,
			Groups:        4,
			GroupBase:     0,
			ChannelLo:     1,
			ChannelHi:     99,
			Discriminator: DiscriminatorSingleLength,
			BuildLength:   6,
			Layouts: []RecordLayout{{
				Length: 6,
				Fields: []FieldSpan{
					{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
					{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0x00: "LSB"}},
				},
			}},
		}
		return cfg
	}

	cases := []struct {
		name    string
		mutate  func(cfg *ProfileConfig)
		wantSub string
	}{
		{
			name:    "a one-byte group index cannot reach 100",
			mutate:  func(cfg *ProfileConfig) { cfg.GroupBase = 1; cfg.Groups = 100 },
			wantSub: "group index",
		},
		{
			name:    "nor can it if the base alone is past the width",
			mutate:  func(cfg *ProfileConfig) { cfg.GroupBase = 100; cfg.Groups = 1 },
			wantSub: "group index",
		},
		{
			name:    "a negative base is not an index",
			mutate:  func(cfg *ProfileConfig) { cfg.GroupBase = -1 },
			wantSub: "GroupBase",
		},
		{
			name:    "a flat form has nowhere to put a base",
			mutate:  func(cfg *ProfileConfig) { cfg.AddressForm = AddressFormFlat; cfg.Groups = 0; cfg.GroupBase = 1 },
			wantSub: "GroupBase",
		},
		{
			name: "the wide form still has a ceiling",
			mutate: func(cfg *ProfileConfig) {
				cfg.AddressForm = AddressFormWideGroupChannel
				cfg.GroupBase = 9999
				cfg.Groups = 2
			},
			wantSub: "group index",
		},
		{
			// THE SUM MUST NOT WRAP. base+count-1 computed in int
			// overflows for MaxInt-scale values and comes back NEGATIVE,
			// which sails past a "highest >= capacity" comparison — so the
			// absurd profile is ACCEPTED by the very rule written to
			// refuse it. Every downstream path refuses it anyway, so
			// nothing reaches a radio; but a validator that admits what it
			// exists to reject has said something false about the profile
			// it just blessed, and the next reader believes it.
			name: "a base that overflows the sum",
			mutate: func(cfg *ProfileConfig) {
				cfg.GroupBase = math.MaxInt
				cfg.Groups = 2
			},
			wantSub: "GroupBase",
		},
		{
			name: "a count that overflows the sum",
			mutate: func(cfg *ProfileConfig) {
				cfg.GroupBase = 1
				cfg.Groups = math.MaxInt
			},
			wantSub: "group index",
		},
		{
			name: "both at the ceiling",
			mutate: func(cfg *ProfileConfig) {
				cfg.GroupBase = math.MaxInt
				cfg.Groups = math.MaxInt
			},
			wantSub: "GroupBase",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			_, err := NewProfile(cfg)
			if err == nil {
				t.Fatalf("NewProfile accepted a profile whose group space does not fit its form")
			}
			if !errors.Is(err, ErrInvalidProfile) {
				t.Errorf("NewProfile error = %v, want one matching ErrInvalidProfile", err)
			}
			if !contains(err.Error(), tc.wantSub) {
				t.Errorf("NewProfile error = %v, want it to name %q", err, tc.wantSub)
			}
		})
	}

	t.Run("the wide form reaches the hundredth group", func(t *testing.T) {
		cfg := base()
		cfg.AddressForm = AddressFormWideGroupChannel
		cfg.GroupBase = 0
		cfg.Groups = 101 // indices 0..100, the 705/905 shape
		if _, err := NewProfile(cfg); err != nil {
			t.Fatalf("NewProfile: %v — a two-byte BCD group must hold 101 groups", err)
		}
	})
}

// contains is strings.Contains without the import, kept local so this file
// stays about addresses.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestWideFormRoundTripsThroughTheGate proves the new form all the way
// through: a set built for a wide-addressed profile is admitted by that
// profile's own gate, and its answer parses back to the address it was
// built for.
func TestWideFormRoundTripsThroughTheGate(t *testing.T) {
	p := wideProfile
	for _, addr := range []ChannelAddress{
		{Group: 1, Channel: 1},
		{Group: 100, Channel: 99},
	} {
		rec := sampleRecord(t, p, p.BuildRecordLength())
		rec.Address = addr
		cmd, err := p.BuildMemorySet(rec)
		if err != nil {
			t.Fatalf("BuildMemorySet(%v): %v", addr, err)
		}
		frame := cmd.Bytes()
		if !p.AllowedCommand(frame) {
			t.Fatalf("AllowedCommand refused this profile's own memory set for %v: % x", addr, frame)
		}
		// The ANSWER form: the same body, addresses reversed.
		answer := append([]byte{PreambleByte, PreambleByte, p.ControllerAddress(), p.RadioAddress()}, frame[4:]...)
		back, err := p.ParseMemoryAnswer(answer)
		if err != nil {
			t.Fatalf("ParseMemoryAnswer(%v): %v", addr, err)
		}
		if back.Address != addr {
			t.Errorf("round trip gave address %v, want %v", back.Address, addr)
		}
	}
}
