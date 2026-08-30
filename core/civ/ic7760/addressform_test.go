// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"bytes"
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"testing"
)

func TestAddressFormAndCommandGate(t *testing.T) {
	p := ic7760.Profile()
	for _, tc := range []struct {
		ch   int
		want []byte
	}{
		{1, []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFD}},
		{99, []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x00, 0x99, 0xFD}},
		{100, []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0xFD}},
		{101, []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x01, 0x01, 0xFD}},
	} {
		cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: tc.ch})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(cmd.Bytes(), tc.want) {
			t.Errorf("channel %d = % X, want % X", tc.ch, cmd.Bytes(), tc.want)
		}
	}
	for _, ch := range []int{0, 102, -1} {
		if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: ch}); err == nil {
			t.Errorf("channel %d accepted", ch)
		}
	}
	if _, err := p.BuildMemoryRead(civ.ChannelAddress{Group: 1, Channel: 1}); err == nil {
		t.Error("group address accepted by flat profile")
	}
	for _, frame := range [][]byte{{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x05, 0xFD}, {0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0x00, 0xFD}} {
		if p.AllowedCommand(frame) {
			t.Errorf("forbidden frame admitted: % X", frame)
		}
	}
}
