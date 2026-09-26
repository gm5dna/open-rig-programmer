// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"errors"
	"testing"
)

func TestFT897Family_RoundTrip(t *testing.T) {
	for _, p := range FT897Family {
		p := p
		t.Run(p.Model, func(t *testing.T) {
			assertRoundTrip(t, p)
		})
	}
}

func TestFT897Family_Incompatible(t *testing.T) {
	candidates := make([]Profile, len(FT897Family))
	for i, p := range FT897Family {
		candidates[i] = shortDeadlines(p)
	}
	_, err := matchAndParse(make([]byte, 12345), candidates)
	if !errors.Is(err, ErrImageIncompatible) {
		t.Fatalf("err = %v, want ErrImageIncompatible", err)
	}
}

// TestFT897Family_Ambiguous mirrors TestFT857Family_Ambiguous: CHIRP's own
// single "FT-857/897" class gives no byte that tells FT-897 apart from
// FT-897D either.
func TestFT897Family_Ambiguous(t *testing.T) {
	candidates := []Profile{shortDeadlines(FT897), shortDeadlines(FT897D)}
	_, err := matchAndParse(make([]byte, FT897.ImageLen), candidates)
	if !errors.Is(err, ErrImageAmbiguous) {
		t.Fatalf("err = %v, want ErrImageAmbiguous", err)
	}
}

// TestFT897And857_Ambiguous documents the cross-family fact from the
// package comment above: CHIRP's own single-class treatment of
// "FT-857/897" means the two families' images are byte-for-byte
// indistinguishable too — offering one of each to a single Arm/Receive
// call is just as ambiguous as offering two within one family.
func TestFT897And857_Ambiguous(t *testing.T) {
	candidates := []Profile{shortDeadlines(FT857), shortDeadlines(FT897)}
	_, err := matchAndParse(make([]byte, FT857.ImageLen), candidates)
	if !errors.Is(err, ErrImageAmbiguous) {
		t.Fatalf("err = %v, want ErrImageAmbiguous", err)
	}
}
