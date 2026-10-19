// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	catft891 "github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	catft991a "github.com/gm5dna/open-rig-programmer/core/cat/ft991a"
	catftdx10 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	catftdx101 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestYaesuBuildDescriptor_OtherFourDialectsMintUnverified is spec A1/M8's
// named test: BuildDescriptor mints SettingItem.Write from
// cat.Dialect.CanSetEX, which is dialect DATA — a nil d.exWrite map for
// every dialect but the FT-710 (core/cat/dialect.go's buildFT710ExWrite is
// called nowhere else) — so every item of every other Yaesu dialect mints
// spec.Unverified, never spec.Supported, with no "dialect == FT710"
// literal anywhere in BuildDescriptor to make that true. Covers all four
// of this project's other Yaesu drivers (FT-891, FT-991A, FTdx10,
// FTdx101D/MP), proving the shared body is benign for them rather than
// asserting it for the FT-710 alone.
func TestYaesuBuildDescriptor_OtherFourDialectsMintUnverified(t *testing.T) {
	dialects := map[string]cat.Dialect{
		"ft891":     catft891.Dialect(),
		"ft991a":    catft991a.Dialect(),
		"ftdx10":    catftdx10.Dialect(),
		"ftdx101d":  catftdx101.DialectD(),
		"ftdx101mp": catftdx101.DialectMP(),
	}

	for name, dialect := range dialects {
		t.Run(name, func(t *testing.T) {
			items := dialect.EXItems()
			if len(items) == 0 {
				t.Fatalf("%s: EXItems() is empty — test fixture has nothing to check", name)
			}
			d := BuildDescriptor(dialect, &Params{Name: name, Model: name}, spec.Supported)
			var checked int
			for _, m := range d.Menus {
				for _, g := range m.Groups {
					for _, it := range g.Items {
						checked++
						if it.Write != spec.Unverified {
							t.Errorf("%s: item %q Write = %v, want spec.Unverified (this dialect has no write descriptor)", name, it.ID, it.Write)
						}
					}
				}
			}
			if checked != len(items) {
				t.Errorf("%s: checked %d items, want %d (== EXItems() count)", name, checked, len(items))
			}
		})
	}
}

// TestWriteSupport_Labels: writable on a hardware-width dialect is Supported,
// writable on a paper dialect is the caller's label, not writable is
// Unverified whatever the label.
func TestWriteSupport_Labels(t *testing.T) {
	addr := cat.EXAddress{P1: 1, P2: 1, P3: 1}
	hw := cat.DialectWithEXWriteForTest(cat.FT710, addr, cat.Domain{Lo: 0, Hi: 9, Step: 1}, 1)
	paper := cat.DialectWithEXWritePaperForTest(cat.FT710, addr, cat.Domain{Lo: 0, Hi: 9, Step: 1}, 1)
	other := cat.EXAddress{P1: 1, P2: 1, P3: 2}
	for _, tc := range []struct {
		name string
		d    cat.Dialect
		a    cat.EXAddress
		want spec.Support
	}{
		{"hardware width", hw, addr, spec.Supported},
		{"paper width", paper, addr, spec.ConsentedUnverified},
		{"not writable, hardware dialect", hw, other, spec.Unverified},
		{"not writable, paper dialect", paper, other, spec.Unverified},
	} {
		if got := writeSupport(tc.d, tc.a, spec.ConsentedUnverified); got != tc.want {
			t.Errorf("%s: writeSupport = %v, want %v", tc.name, got, tc.want)
		}
	}
}
