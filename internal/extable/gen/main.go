// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen regenerates a registered radio model's EX inventory from that
// model's committed CSV sources. It is a thin wrapper around
// internal/extable: resolve the profile named by -profile, read that
// profile's manual CSV and — under the observed regime — its hardware
// READ-observation CSV, ParseCSV / ParseObservedCSV, RenderGo, write the
// result. Every path comes from the named profile rather than from a flag,
// and each is relative to the profile's own package directory, which is the
// working directory of the `//go:generate` directive that invokes this
// command (for the FT-710, the directive in core/cat/exinventory.go).
//
// The two sources have different provenance and are never conflated: the
// manual CSV is a transcription of the model's menu chart, while the
// observation CSV records what ONE radio answered to EX READ requests (for
// the FT-710, during the two M8c sweeps — one firmware, one configuration,
// read direction only; see that file's own header for the full scope). A
// profile under the manual-only regime has no observation CSV at all.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("extable/gen: ")

	profileName := flag.String("profile", "", "name of the registered extable profile to generate (e.g. ft710)")
	flag.Parse()

	// gen takes no operands. Silently ignoring them would let the old
	// three-path invocation — or `gen ft710` written without the flag — read
	// as a successful run that generated something other than what was
	// asked for.
	if flag.NArg() > 0 {
		log.Fatalf("unexpected positional arguments %v — gen takes only -profile", flag.Args())
	}
	if *profileName == "" {
		log.Fatal("-profile is required; see internal/extable/profile.go for the registered names")
	}
	p, ok := extable.Lookup(*profileName)
	if !ok {
		var names []string
		for _, np := range extable.RegisteredProfiles() {
			names = append(names, np.Name)
		}
		log.Fatalf("unknown profile %q; registered: %v", *profileName, names)
	}

	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		log.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, data)
	if err != nil {
		log.Fatalf("parsing %s: %v", p.ManualCSV, err)
	}

	// A manual-only profile has no observation source; RenderGo requires the
	// map to be empty for it, and a nil map is empty.
	//
	// This branches on ObservedCSV while RenderGo ENFORCES the regime on
	// p.Observations — two proxies for one fact, safe only because Validate
	// has already run (ParseCSV above validates p, as every extable API
	// does) and forces the biconditional: ObservedCSV is non-empty iff
	// Observations is ObservationsRequired.
	var observed map[string]extable.Observed
	if p.ObservedCSV != "" {
		observedData, err := os.ReadFile(p.ObservedCSV)
		if err != nil {
			log.Fatalf("reading %s: %v", p.ObservedCSV, err)
		}
		if observed, err = extable.ParseObservedCSV(p, observedData); err != nil {
			log.Fatalf("parsing %s: %v", p.ObservedCSV, err)
		}
	}

	out, err := extable.RenderGo(p, rows, observed)
	if err != nil {
		log.Fatalf("rendering Go: %v", err)
	}
	if err := os.WriteFile(p.OutFile, out, 0o644); err != nil {
		log.Fatalf("writing %s: %v", p.OutFile, err)
	}
}
