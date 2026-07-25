// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// FuzzImport asserts Import never panics on arbitrary input and only
// ever returns a typed *ParseError on failure.
func FuzzImport(f *testing.F) {
	seeds := []string{
		"",
		testHeader,
		testHeader + "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,,\n",
		testHeader + "001,,14250000,USB,-120,yes,yes,ENC-DEC,88.5,PLUS,MB9XYZ,yes,yes\n",
		testHeader + "042,M-42,,,,,,,,,,,\n",
		testHeader + ",,14250000,USB,,,,OFF,,SIMPLEX,TAG,,\n",
		testHeader + "001,,notanumber,USB,,,,OFF,,SIMPLEX,TAG,,\n",
		testHeader + "001,,14250000,USB,,,,OFF,,SIMPLEX,'=SUM(A1),,\n",
		testHeader + "001,,14250000,USB,,,,OFF,,SIMPLEX,\"unterminated,,,\n",
		"bogus,header\nrow,data\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, err := Import(strings.NewReader(s))
		if err == nil {
			return
		}
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("Import(%q) returned non-ParseError: %T (%v)", s, err, err)
		}
	})
}

// FuzzImportCHIRP asserts ImportCHIRP never panics on arbitrary input,
// only ever returns a typed *ParseError on failure, and — on success —
// never returns a malformed slot or an Action outside the three the
// package defines. Seeded with the committed CHIRP fixture (whole file,
// and split into individual lines) plus a handful of hand-picked edge
// cases.
func FuzzImportCHIRP(f *testing.F) {
	if fixture, err := os.ReadFile("testdata/chirp_sample.csv"); err == nil {
		f.Add(string(fixture))
		lines := strings.Split(string(fixture), "\n")
		header := lines[0]
		for _, line := range lines[1:] {
			if line == "" {
				continue
			}
			f.Add(header + "\n" + line + "\n")
		}
	}
	seeds := []string{
		"",
		"Location,Frequency,Mode\n",
		"Location,Frequency,Mode\n1,145.500000,FM\n",
		"Name,Frequency,Mode\n1,145.500000,FM\n", // missing core column
		"Location,Frequency,Mode\n0,145.500000,FM\n",
		"Location,Frequency,Mode\nnotanumber,145.500000,FM\n",
		"Location,Frequency,Mode\n1,notafrequency,FM\n",
		"Location,Frequency,Mode\n1,145.500000,BOGUSMODE\n",
		"Location,Frequency,Mode\n1,\"unterminated,FM\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		channels, report, err := ImportCHIRP(strings.NewReader(s))
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ImportCHIRP(%q) returned non-ParseError: %T (%v)", s, err, err)
			}
		}
		for _, ch := range channels {
			if len(ch.Slot) != 3 {
				t.Fatalf("ImportCHIRP(%q) produced malformed slot %q", s, ch.Slot)
			}
		}
		for _, e := range report.Entries {
			switch e.Action {
			case ActionDropped, ActionApproximated, ActionUnsupported:
			default:
				t.Fatalf("ImportCHIRP(%q) produced LossEntry with unknown Action %q", s, e.Action)
			}
		}
	})
}
