// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen regenerates core/cat/exinventory_gen.go from the committed
// Table 2 CSV and the committed hardware READ-observation CSV. It is a thin
// wrapper around internal/extable: read both sources, ParseCSV /
// ParseObservedCSV, RenderGo, write the result. It is invoked by the
// `//go:generate` directive in core/cat/exinventory.go with the package
// directory as its working directory, so its default flag values are
// relative to core/cat.
//
// The two sources have different provenance and are never conflated: the
// Table 2 CSV is a transcription of the manual, while the observation CSV
// records what ONE radio answered to EX READ requests during the two M8c
// sweeps — one firmware, one configuration, read direction only (see that
// file's own header for the full scope).
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

	csvPath := flag.String("csv", "table2.csv", "path to the Table 2 CSV source")
	observedPath := flag.String("observed", "table2-observed.csv", "path to the hardware READ observation CSV")
	outPath := flag.String("out", "exinventory_gen.go", "path to write the generated Go file")
	flag.Parse()

	data, err := os.ReadFile(*csvPath)
	if err != nil {
		log.Fatalf("reading %s: %v", *csvPath, err)
	}

	rows, err := extable.ParseCSV(extable.FT710Profile(), data)
	if err != nil {
		log.Fatalf("parsing %s: %v", *csvPath, err)
	}

	observedData, err := os.ReadFile(*observedPath)
	if err != nil {
		log.Fatalf("reading %s: %v", *observedPath, err)
	}

	observed, err := extable.ParseObservedCSV(extable.FT710Profile(), observedData)
	if err != nil {
		log.Fatalf("parsing %s: %v", *observedPath, err)
	}

	out, err := extable.RenderGo(rows, observed)
	if err != nil {
		log.Fatalf("rendering Go: %v", err)
	}

	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		log.Fatalf("writing %s: %v", *outPath, err)
	}
}
