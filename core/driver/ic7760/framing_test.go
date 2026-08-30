// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// TestDriverImplementsSerialFramingReporter is a compile-time assertion
// plus a value pin.
//
// THE INTERFACE IS ON THE CONCRETE DRIVER, NOT THE SESSION, and that is
// forced (enabler E2): the stop bits are a property of the PORT, chosen
// when the port is opened, and the session is what Open returns once the
// port already exists. internal/wiring holds the driver value before it
// opens anything, and that is the one moment this question can be asked
// at.
//
// MATERIALITY, verified: transport.DefaultStopBits is 2, so without this
// reporter an IC-7760 would open at 8-N-2 against the tier's assumed
// 8-N-1 — the silent divergence spec D3.1 exists to prevent.
func TestDriverImplementsSerialFramingReporter(t *testing.T) {
	var d driver.Driver = New(RealHardware)
	r, ok := d.(driver.SerialFramingReporter)
	if !ok {
		t.Fatal("the IC-7760 driver does not implement driver.SerialFramingReporter - spec D3.1 requires every Icom driver to report its framing")
	}
	if got := r.StopBits(); got != 1 {
		t.Errorf("StopBits() = %d, want 1 (8-N-1, ASSUMED: additions design D5 entry 8, register entry ic7760-serial-framing)", got)
	}
	// Every profile reports the same framing: the stop bits are a fact
	// about the LINK, not about which capability set a caller chose.
	for _, p := range []Profile{RealHardware, Simulated, Profile(42)} {
		rp, ok := New(p).(driver.SerialFramingReporter)
		if !ok {
			t.Fatalf("Profile(%d) does not implement driver.SerialFramingReporter", p)
		}
		if got := rp.StopBits(); got != 1 {
			t.Errorf("Profile(%d).StopBits() = %d, want 1", p, got)
		}
	}
}

// TestStopBitsIsAssumedNotEvidenced is a DOCUMENTATION pin: it fails if
// this package stops carrying the evidence-absence record.
//
// The IC-7760's CI-V Reference Guide says NOTHING about serial framing —
// the words "stop bit", "data bit", "parity" and "8 bit" appear nowhere,
// about any port (matrix §3.1) — so the 1 is an ASSUMED tier convention
// with a named per-model lift, not a reading of this document. A future
// editor who deletes the record while leaving the value would leave a
// number wearing an authority it does not have; this test is what stops
// that.
//
// THE PINNED STRINGS ARE THE IC-7760's OWN. Stage-review finding F1 found
// this list demanding a sibling model's material: a lift identifier and a
// register alias that belong to the IC-7610, and a decoder-baud command
// row that this radio's guide does not print. None of it exists for the
// IC-7760, whose register entry is ic7760-serial-framing (matrix §5 entry
// 1). The pins below therefore name the tier home (additions design D5
// entry 8), that register entry, and the matrix §3.1 absence sweep — whose
// DATA/RTTY hazard sentence is recorded precisely because it is VACUOUS
// here: this document has no framing line to be misread in the first
// place. TestProvenanceUsesOnlyTheIC7760Authority sweeps the package for
// the removed material.
func TestStopBitsIsAssumedNotEvidenced(t *testing.T) {
	for _, tt := range []struct {
		file  string
		wants []string
	}{
		{
			file: "framing.go",
			wants: []string{
				"ASSUMED",
				"D5 entry 8",
				// The register entry, and the two things its capture
				// cannot settle.
				"ic7760-serial-framing",
				"[REMOTE]",
				"not any other model",
			},
		},
		{
			file: "doc.go",
			wants: []string{
				// The absence sweep, stated as an absence.
				"appear NOWHERE in the document",
				// The mandatory hazard sentence...
				"SUCH A LINE IS NOT EVIDENCE",
				"DATA/RTTY",
				// ...and this radio's finding about it.
				"not even a misleading line to be misread",
				// Materiality.
				"transport.DefaultStopBits is 2",
			},
		},
	} {
		data, err := os.ReadFile(tt.file)
		if err != nil {
			t.Fatalf("reading %s: %v", tt.file, err)
		}
		text := string(data)
		for _, want := range tt.wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s no longer records %q - the stop-bit value must never outlive the record of what it rests on", tt.file, want)
			}
		}
	}
}
