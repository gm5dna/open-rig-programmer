// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

func TestAddressBoundaryFrames(t *testing.T) {
	p := Profile()
	for _, tc := range []struct {
		name string
		addr civ.ChannelAddress
		want []byte
	}{
		{
			name: "bank A first memory",
			addr: civ.ChannelAddress{Group: 1, Channel: 1},
			want: []byte{0xfe, 0xfe, 0x88, 0xe0, 0x1a, 0x00, 0x01, 0x00, 0x01, 0xfd},
		},
		{
			name: "bank E last memory",
			addr: civ.ChannelAddress{Group: 5, Channel: 99},
			want: []byte{0xfe, 0xfe, 0x88, 0xe0, 0x1a, 0x00, 0x05, 0x00, 0x99, 0xfd},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := p.BuildMemoryRead(tc.addr)
			if err != nil {
				t.Fatalf("BuildMemoryRead(%v): %v", tc.addr, err)
			}
			if got := cmd.Bytes(); !bytes.Equal(got, tc.want) {
				t.Errorf("frame = % X, want % X", got, tc.want)
			}
			if !p.AllowedCommand(cmd.Bytes()) {
				t.Errorf("profile gate refused its own valid boundary frame % X", cmd.Bytes())
			}
		})
	}
}

func TestAddressesOutsideBaseRectangleAreRefused(t *testing.T) {
	// CANNOT ESTABLISH: ic7100-special-bank-byte. The lift selects scan
	// edge 0100 and call channel 0106 at the front panel, reads each with
	// 1A 00, and records byte 1. Until then no scan/call address is guessed.
	for _, tc := range []struct {
		name string
		addr civ.ChannelAddress
	}{
		{"bank 00", civ.ChannelAddress{Group: 0, Channel: 1}},
		{"bank 06", civ.ChannelAddress{Group: 6, Channel: 1}},
		{"channel 0000", civ.ChannelAddress{Group: 1, Channel: 0}},
		{"channel 0100 scan edge", civ.ChannelAddress{Group: 1, Channel: 100}},
		{"channel 0106 call", civ.ChannelAddress{Group: 1, Channel: 106}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := Profile().BuildMemoryRead(tc.addr)
			if err == nil {
				t.Fatalf("BuildMemoryRead(%v) built out-of-scope frame % X", tc.addr, cmd.Bytes())
			}
			if !cmd.IsZero() {
				t.Errorf("BuildMemoryRead(%v) returned non-zero command with error", tc.addr)
			}
			if !strings.Contains(err.Error(), "outside base g1..5/ch1..99") {
				t.Errorf("error = %q, want declared base space g1..5/ch1..99", err)
			}
		})
	}
}
