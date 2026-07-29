// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// baseDigestChannels returns a small, fully-populated set of channels
// used as the starting point for the digest sensitivity tests below.
func baseDigestChannels() []Channel {
	return []Channel{
		{Slot: "001", Data: &ChannelData{
			FreqHz:     14250000,
			Mode:       "USB",
			CTCSS:      "OFF",
			CTCSSTone:  ToneField{State: Unknown},
			Shift:      "SIMPLEX",
			Tag:        "ALPHA",
			TagDisplay: BoolField{State: Known, Value: false},
			ScanSkip:   BoolField{State: Known, Value: false},
		}},
		{Slot: "002", Data: &ChannelData{
			FreqHz:     43012500,
			Mode:       "FM",
			CTCSS:      "ENC-DEC",
			CTCSSTone:  ToneField{State: Known, Value: spec.Tone(885)},
			Shift:      "PLUS",
			Tag:        "BRAVO",
			TagDisplay: BoolField{State: Known, Value: false},
			ScanSkip:   BoolField{State: Known, Value: true},
		}},
		{Slot: "003"}, // empty slot
	}
}

// TestDigest_OrderIndependent checks that Digest does not depend on the
// order of the input slice.
func TestDigest_OrderIndependent(t *testing.T) {
	a := baseDigestChannels()
	b := []Channel{a[2], a[0], a[1]}

	da, db := Digest(a), Digest(b)
	if da != db {
		t.Errorf("Digest differs by input order: %q vs %q", da, db)
	}
}

// TestDigest_Deterministic checks that computing the digest twice over
// the same (unmutated) input gives the same result.
func TestDigest_Deterministic(t *testing.T) {
	a := baseDigestChannels()
	if Digest(a) != Digest(a) {
		t.Error("Digest is not deterministic across repeated calls")
	}
}

// TestDigest_SensitiveToChanges checks that changing a single field of a
// single channel changes the digest, for a representative field from
// each area of a channel's data.
func TestDigest_SensitiveToChanges(t *testing.T) {
	base := Digest(baseDigestChannels())

	cases := []struct {
		name   string
		mutate func([]Channel)
	}{
		{"freq changes", func(cs []Channel) { cs[0].Data.FreqHz++ }},
		{"tag changes", func(cs []Channel) { cs[0].Data.Tag = "CHANGED" }},
		{"tone state changes", func(cs []Channel) { cs[0].Data.CTCSSTone.State = Unavailable }},
		{"scan-skip value changes", func(cs []Channel) { cs[1].Data.ScanSkip.Value = false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := baseDigestChannels()
			tc.mutate(cs)
			if got := Digest(cs); got == base {
				t.Errorf("Digest unchanged after mutating %s: %q", tc.name, got)
			}
		})
	}
}

// TestDigest_ContentIdenticalReReadDigestsEqual pins the corrected claim
// in Digest's doc comment: Digest identifies CONTENT alone, so a
// "reconnect" or "re-read" that happens to read back identical data
// (independently constructed, not the same slice) produces the SAME
// digest, not a different one — Digest cannot and does not detect
// session/device identity by itself.
func TestDigest_ContentIdenticalReReadDigestsEqual(t *testing.T) {
	firstRead := baseDigestChannels()
	secondRead := baseDigestChannels() // simulates an independent re-read of identical data
	if Digest(firstRead) != Digest(secondRead) {
		t.Error("Digest differs for two independently-built, content-identical channel sets, want equal")
	}
}

// TestDigest_EmptyVsPopulatedDiffers checks that the same slot, empty vs.
// populated, produces different digests.
func TestDigest_EmptyVsPopulatedDiffers(t *testing.T) {
	empty := []Channel{{Slot: "001"}}
	populated := []Channel{{Slot: "001", Data: &ChannelData{FreqHz: 14250000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX", TagDisplay: BoolField{State: Known, Value: false}}}}

	de, dp := Digest(empty), Digest(populated)
	if de == dp {
		t.Errorf("Digest same for empty vs. populated slot: %q", de)
	}
}
