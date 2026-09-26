// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestMapChannels_EmptyAndPopulated(t *testing.T) {
	img := Image{
		Channels: []DecodedChannel{
			{Empty: true},
			{FreqHz: 14_250_000, Mode: "USB", Name: "HOME"},
		},
	}
	got := MapChannels(img)
	if len(got) != 2 {
		t.Fatalf("MapChannels: got %d channels, want 2", len(got))
	}

	if got[0].Slot != "001" || !got[0].Empty() {
		t.Errorf("MapChannels[0] = %+v, want empty slot 001", got[0])
	}

	c := got[1]
	if c.Slot != "002" || c.Empty() {
		t.Fatalf("MapChannels[1] = %+v, want populated slot 002", c)
	}
	if c.Data.FreqHz != 14_250_000 || c.Data.Mode != "USB" || c.Data.Tag != "HOME" {
		t.Errorf("MapChannels[1].Data = %+v, want FreqHz/Mode/Tag carried across", c.Data)
	}
	if c.Data.CTCSS != "OFF" || c.Data.Shift != "SIMPLEX" {
		t.Errorf("MapChannels[1].Data CTCSS/Shift = %q/%q, want the documented OFF/SIMPLEX defaults", c.Data.CTCSS, c.Data.Shift)
	}
	if c.Data.TagDisplay.State != codeplug.Unavailable || c.Data.ScanSkip.State != codeplug.Unavailable ||
		c.Data.CTCSSTone.State != codeplug.Unavailable || c.Data.DataMode.State != codeplug.Unavailable {
		t.Errorf("MapChannels[1].Data pre-tier tri-state fields = %+v, want all Unavailable", c.Data)
	}
	for _, tf := range codeplug.TierFields {
		if got := tf.State(c.Data); *got != codeplug.Unavailable {
			t.Errorf("MapChannels[1].Data tier field %s = %q, want Unavailable", tf.Name, *got)
		}
	}
}

func TestMapChannels_Empty(t *testing.T) {
	if got := MapChannels(Image{}); len(got) != 0 {
		t.Errorf("MapChannels(Image{}) = %v, want empty slice", got)
	}
}
