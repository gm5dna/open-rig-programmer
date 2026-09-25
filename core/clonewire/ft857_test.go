// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"errors"
	"testing"
)

func TestFT857Family_RoundTrip(t *testing.T) {
	for _, p := range FT857Family {
		p := p
		t.Run(p.Model, func(t *testing.T) {
			assertRoundTrip(t, p)
		})
	}
}

func TestFT857Family_Incompatible(t *testing.T) {
	candidates := make([]Profile, len(FT857Family))
	for i, p := range FT857Family {
		candidates[i] = shortDeadlines(p)
	}
	_, err := matchAndParse(make([]byte, 12345), candidates)
	if !errors.Is(err, ErrImageIncompatible) {
		t.Fatalf("err = %v, want ErrImageIncompatible", err)
	}
}

// TestFT857Family_Ambiguous proves the spec-required outcome for
// "nothing distinguishes them" (spec.md §Identity probe): CHIRP itself
// gives no byte that tells FT-857 apart from FT-857D (one MODEL, one
// _memsize, one block schedule), so offering both to one Arm/Receive call
// must return ErrImageAmbiguous, not silently pick one.
func TestFT857Family_Ambiguous(t *testing.T) {
	candidates := []Profile{shortDeadlines(FT857), shortDeadlines(FT857D)}
	_, err := matchAndParse(make([]byte, FT857.ImageLen), candidates)
	if !errors.Is(err, ErrImageAmbiguous) {
		t.Fatalf("err = %v, want ErrImageAmbiguous", err)
	}
}
