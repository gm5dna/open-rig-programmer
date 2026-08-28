// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestParseCHIRPTone_ConsultsTheSharedPredicate is E3's consumer half in
// core/csvio. The CHIRP importer used to carry its own list-only copy of
// "is this tone in caps.CTCSSTones" — a second, independently written
// twin of the one in core/codeplug — so a CI-V radio declaring a numeric
// RANGE would have had every tone in a CHIRP file refused as unusable and
// reported as a Blocking loss. Both consumers now ask
// spec.Capabilities.AdmitsTone.
//
// The EXACTNESS of the parse is untouched and is re-pinned here: "88.54"
// is refused outright rather than rounded, on a range radio just as on a
// chart radio.
func TestParseCHIRPTone_ConsultsTheSharedPredicate(t *testing.T) {
	rangeCaps := spec.Capabilities{
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1},
	}
	listCaps := spec.Capabilities{CTCSSTones: []spec.Tone{670, 885, 2541}}

	cases := []struct {
		name     string
		caps     spec.Capabilities
		cell     string
		wantTone spec.Tone
		wantOK   bool
	}{
		{"range: a tone the chart never listed", rangeCaps, "100.0", 1000, true},
		{"range: the lower bound", rangeCaps, "67.0", 670, true},
		{"range: above the range", rangeCaps, "300.0", 0, false},
		{"range: too much precision is still refused", rangeCaps, "88.54", 0, false},
		{"list: a listed tone", listCaps, "88.5", 885, true},
		{"list: an unlisted tone", listCaps, "100.0", 0, false},

		// PINNED: a radio declaring neither refuses every tone, which is
		// what an empty CTCSSTones has always done here.
		{"neither declared: fail-closed", spec.Capabilities{}, "88.5", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCHIRPTone(tc.cell, tc.caps)
			if ok != tc.wantOK {
				t.Fatalf("parseCHIRPTone(%q) ok = %v, want %v", tc.cell, ok, tc.wantOK)
			}
			if got != tc.wantTone {
				t.Errorf("parseCHIRPTone(%q) = %v, want %v", tc.cell, got, tc.wantTone)
			}
		})
	}
}
