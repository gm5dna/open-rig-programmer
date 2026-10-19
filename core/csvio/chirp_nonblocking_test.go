// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/ts480"
)

// TestImportCHIRP_TS480DuplexIsPerRow pins that a radio with no shift
// vocabulary (the TS-480, built but unregistered, so no registry fixture
// reaches it) judges each CHIRP Duplex cell on its own row: blank imports as
// simplex, anything else blocks that row alone.
func TestImportCHIRP_TS480DuplexIsPerRow(t *testing.T) {
	caps := ts480.New(ts480.RealHardware).Capabilities()
	const csv = "Location,Name,Frequency,Duplex,Mode\n" +
		"1,BLANK,145.500000,,FM\n" +
		"2,PLUS,145.510000,+,FM\n" +
		"3,SPLIT,145.520000,split,FM\n" +
		"4,OFF,145.530000,off,FM\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	for line, wantBlocking := range map[int]bool{2: false, 3: true, 4: true, 5: true} {
		var blocking, duplex int
		for _, e := range entriesForLine(report, line) {
			if e.Column != "Duplex" {
				continue
			}
			duplex++
			if e.Blocking {
				blocking++
			}
		}
		if wantBlocking && (duplex != 1 || blocking != 1) {
			t.Errorf("line %d: %d Duplex entries (%d blocking), want exactly one blocking", line, duplex, blocking)
		}
		if !wantBlocking && duplex != 0 {
			t.Errorf("line %d: %d Duplex entries, want none for a blank cell", line, duplex)
		}
	}
	var blank bool
	for _, ch := range channels {
		if ch.Data != nil && ch.Data.Shift == "" && ch.Data.FreqHz == 145_500_000 {
			blank = true
		}
	}
	if !blank {
		t.Errorf("the blank-Duplex row did not import as a simplex channel: %+v", channels)
	}
}
