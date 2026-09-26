// SPDX-License-Identifier: GPL-3.0-or-later

package fakeyaesuclone

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
)

// allProfiles lists every Profile this package's tests exercise, across all
// three families (Phase 3 covers all of Phase 2a/2b/2c, per the
// orchestrator's deviation note — see the milestone's phase3.md report).
var allProfiles = append(append(append([]clonewire.Profile{}, clonewire.FT817Family...), clonewire.FT857Family...), clonewire.FT897Family...)

func TestBuildImage_Length(t *testing.T) {
	for _, p := range allProfiles {
		img := BuildImage(p)
		if len(img) != p.ImageLen {
			t.Errorf("%s: BuildImage returned %d bytes, want ImageLen %d", p.Model, len(img), p.ImageLen)
		}
	}
}

// TestBuildImage_RoundTrips proves the fabricated image actually parses:
// clonewire.ParseImage (the same code Receive calls once a length match is
// found) decodes the populated channels back out with the values BuildImage
// put in, and every other channel decodes Empty.
func TestBuildImage_RoundTrips(t *testing.T) {
	for _, p := range allProfiles {
		img := BuildImage(p)
		parsed, err := clonewire.ParseImage(img, p)
		if err != nil {
			t.Fatalf("%s: ParseImage: %v", p.Model, err)
		}
		if len(parsed.Channels) != p.ChannelCount {
			t.Fatalf("%s: got %d decoded channels, want %d", p.Model, len(parsed.Channels), p.ChannelCount)
		}
		for i, want := range populatedChannels {
			ch := parsed.Channels[i]
			if ch.Empty {
				t.Errorf("%s: channel %d: got Empty, want populated %q", p.Model, i, want.name)
				continue
			}
			if ch.Name != want.name {
				t.Errorf("%s: channel %d: name = %q, want %q", p.Model, i, ch.Name, want.name)
			}
			wantHz := uint64(want.freqHz) * 10
			if ch.FreqHz != wantHz {
				t.Errorf("%s: channel %d: FreqHz = %d, want %d", p.Model, i, ch.FreqHz, wantHz)
			}
		}
		for i := len(populatedChannels); i < len(parsed.Channels); i++ {
			if !parsed.Channels[i].Empty {
				t.Errorf("%s: channel %d: want Empty (unpopulated), got populated", p.Model, i)
			}
		}
	}
}

// TestBuildImage_UnmappedFillNeverZero proves the unmapped fill byte is
// never zero (spec.md §Fakes and byte-identity: a zero-filled unmapped
// region misreads as "confirmed empty").
func TestBuildImage_UnmappedFillNeverZero(t *testing.T) {
	if unmappedFill == 0x00 {
		t.Fatal("unmappedFill must not be zero")
	}
	p := clonewire.FT817
	img := BuildImage(p)
	for i := 0; i < p.RecordOffset; i++ {
		if img[i] == 0x00 {
			t.Fatalf("byte %d of the unmapped leading region is zero", i)
		}
	}
}
