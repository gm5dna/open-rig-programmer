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
		t.Errorf("StopBits() = %d, want 1 (8-N-1, ASSUMED: D5 entry 8, lift R8)", got)
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
				"R8",
				// The capture, and the two things it cannot settle.
				"ic7760-framing-8n1",
				"[REMOTE]",
				"not any other model",
			},
		},
		{
			file: "doc.go",
			wants: []string{
				// The absence sweep, stated as an absence.
				"appear NOWHERE in the document",
				// The mandatory hazard sentence.
				"SUCH A LINE IS NOT EVIDENCE",
				"DATA/RTTY",
				// The trap row that must never be read as CI-V evidence.
				"1A 05 01 21",
				"Decode Baud Rate",
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
