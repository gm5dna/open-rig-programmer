// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// --- S0.2: the MW/MT slot-domain refusal sentences, rendered per dialect ---
//
// Until now all three sentences named "PMS (P1L-P9U)" and the "5xx/EMG"
// banks as literals. Both halves are false on the FT-991A: its pairs are
// 100-117, and it has neither a 5 MHz bank nor an emergency channel, so a
// support diagnostic quoting the old text would tell its owner their radio
// has banks awaiting verification that it does not have at all.
//
// THE CONSTRAINT IS THAT THE FOUR TOKEN DIALECTS DO NOT MOVE ONE BYTE.
// core/cat/testdata/frame-corpus.golden carries the FT-710's own renders of
// both sentences twelve times each, and mw.go's own comment forbids
// rewording them outside a change allowed to regenerate that corpus. This
// change rewords nothing: it composes the same bytes from the dialect's own
// data. The golden is the proof, and these tests say the same thing where a
// reader will look for it.

// TestSlotDomainText_FT710SentencesAreByteIdentical holds the two shipped
// sentences against their pre-seam spelling, verbatim.
func TestSlotDomainText_FT710SentencesAreByteIdentical(t *testing.T) {
	const (
		wantMW = `MW: slot must be Writable() (memory 001-099 or PMS P1L-P9U; 5xx/EMG/"000" rejected)`
		wantMT = `MT: slot must be memory (001-099) or PMS (P1L-P9U); 5xx/EMG rejected by project policy pending M5a, "000"/invalid rejected per reference`
	)
	if got := FT710.mwSlotDomainRefusal(); got != wantMW {
		t.Errorf("mwSlotDomainRefusal() =\n  %q\nwant\n  %q", got, wantMW)
	}
	if got := FT710.mtSlotDomainRefusal(); got != wantMT {
		t.Errorf("mtSlotDomainRefusal() =\n  %q\nwant\n  %q", got, wantMT)
	}
}

// TestSlotDomainText_ReachesTheShippedRefusals proves the renderers are
// what the builders actually emit, on both commands and through the gate's
// own validator, rather than a pair of functions nothing calls.
func TestSlotDomainText_ReachesTheShippedRefusals(t *testing.T) {
	sixty, err := FT710.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("SixtyMSlot(1): %v", err)
	}
	m := MemoryData{Slot: sixty, FreqHz: 14250000, Mode: ModeUSB, Kind: KindMemory, CTCSS: CTCSSOff, Shift: ShiftSimplex}

	if _, err := FT710.BuildMWSet(m); err == nil {
		t.Fatal("BuildMWSet accepted a 60m slot")
	} else if !strings.Contains(err.Error(), FT710.mwSlotDomainRefusal()) {
		t.Errorf("BuildMWSet refusal %q does not carry mwSlotDomainRefusal()", err)
	}
	if _, err := FT710.BuildMTSet(sixty, false, "X"); err == nil {
		t.Fatal("BuildMTSet accepted a 60m slot")
	} else if !strings.Contains(err.Error(), FT710.mtSlotDomainRefusal()) {
		t.Errorf("BuildMTSet refusal %q does not carry mtSlotDomainRefusal()", err)
	}
}

// TestSlotDomainText_NumericPMSDialect is the FT-991A's shape: the PMS
// domain is the decimal range its MC legend prints, and NEITHER special
// bank is named, because it has neither.
func TestSlotDomainText_NumericPMSDialect(t *testing.T) {
	const (
		wantMW = `MW: slot must be Writable() (memory 001-099 or PMS 100-117; "000" rejected)`
		wantMT = `MT: slot must be memory (001-099) or PMS (100-117); "000"/invalid rejected per reference`
	)
	if got := numericPMSDialect.mwSlotDomainRefusal(); got != wantMW {
		t.Errorf("mwSlotDomainRefusal() =\n  %q\nwant\n  %q", got, wantMW)
	}
	if got := numericPMSDialect.mtSlotDomainRefusal(); got != wantMT {
		t.Errorf("mtSlotDomainRefusal() =\n  %q\nwant\n  %q", got, wantMT)
	}
	// The M5a policy citation belongs to the banks it governs, and this
	// radio has none of them: naming 5xx or EMG here would tell its owner
	// their radio has banks awaiting hardware verification.
	for _, sentence := range []string{numericPMSDialect.mwSlotDomainRefusal(), numericPMSDialect.mtSlotDomainRefusal()} {
		for _, forbidden := range []string{"5xx", "EMG", "M5a"} {
			if strings.Contains(sentence, forbidden) {
				t.Errorf("%q names %q on a dialect that has no such bank", sentence, forbidden)
			}
		}
	}
}

// TestSpecialBankText_DerivesFromTheDECLAREDBanks pins the choice the spec
// makes over the alternative: the clause is keyed on whether this dialect
// HAS the banks, not on its PMS form. A future token-PMS radio with no 5
// MHz bank must not inherit the FT-710's sentence, and the two questions
// are only accidentally the same on the six dialects registered today.
func TestSpecialBankText_DerivesFromTheDECLAREDBanks(t *testing.T) {
	base := pmsFormBaseConfig() // memory 001-099, 9 token pairs, no 60m, no EMG

	both := base
	both.Slots.SixtyLo, both.Slots.SixtyHi = 501, 599
	both.Slots.EmergencyWire = "EMG"

	sixtyOnly := base
	sixtyOnly.Slots.SixtyLo, sixtyOnly.Slots.SixtyHi = 501, 510 // the FT-891's transcribed bounds

	emgOnly := base
	emgOnly.Slots.EmergencyWire = "EMG"

	for _, tc := range []struct {
		name string
		cfg  DialectConfig
		want string
	}{
		{"both banks", both, "5xx/EMG"},
		{"60m only", sixtyOnly, "5xx"},
		{"emergency only", emgOnly, "EMG"},
		{"neither", base, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := mustFixtureDialect(tc.cfg)
			if got := d.specialBankText(); got != tc.want {
				t.Errorf("specialBankText() = %q, want %q", got, tc.want)
			}
		})
	}

	// The "5xx" shorthand is the reference's own spelling and is derived
	// from this dialect's declared bounds, not assumed: a bank straddling
	// two hundred-blocks could not honestly be called "5xx", so it is
	// spelled out instead.
	straddling := base
	straddling.Slots.SixtyLo, straddling.Slots.SixtyHi = 490, 510
	if got := mustFixtureDialect(straddling).specialBankText(); got != "490-510" {
		t.Errorf("specialBankText() = %q for a 490..510 bank, want %q", got, "490-510")
	}
}

// TestSlotDomainText_DegenerateSlotSpaces holds the two halves that can be
// absent. Neither is a radio this project has registered; both are
// representable, and a sentence naming a bank its dialect does not have is
// exactly what S0.2 exists to end.
func TestSlotDomainText_DegenerateSlotSpaces(t *testing.T) {
	noPMS := pmsFormBaseConfig()
	noPMS.Slots.PMSPairs, noPMS.Slots.PMSForm = 0, PMSSlotForm(0)

	noMemory := pmsFormBaseConfig()
	noMemory.Slots.MemoryLo, noMemory.Slots.MemoryHi = 0, 0

	for _, tc := range []struct {
		name   string
		cfg    DialectConfig
		wantMW string
		wantMT string
	}{
		{
			"no PMS pairs", noPMS,
			`MW: slot must be Writable() (memory 001-099; "000" rejected)`,
			`MT: slot must be memory (001-099); "000"/invalid rejected per reference`,
		},
		{
			"no memory bank", noMemory,
			`MW: slot must be Writable() (PMS P1L-P9U; "000" rejected)`,
			`MT: slot must be PMS (P1L-P9U); "000"/invalid rejected per reference`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := mustFixtureDialect(tc.cfg)
			if got := d.mwSlotDomainRefusal(); got != tc.wantMW {
				t.Errorf("mwSlotDomainRefusal() =\n  %q\nwant\n  %q", got, tc.wantMW)
			}
			if got := d.mtSlotDomainRefusal(); got != tc.wantMT {
				t.Errorf("mtSlotDomainRefusal() =\n  %q\nwant\n  %q", got, tc.wantMT)
			}
		})
	}
}

// TestSlotDomainText_NamesNoBankItsDialectLacks is the property stated over
// every dialect this package can see, so a fixture added later cannot
// quietly acquire a sentence about somebody else's radio.
func TestSlotDomainText_NamesNoBankItsDialectLacks(t *testing.T) {
	for _, nd := range allTestDialects() {
		t.Run(nd.name, func(t *testing.T) {
			d := nd.dia
			for _, sentence := range []string{d.mwSlotDomainRefusal(), d.mtSlotDomainRefusal()} {
				if d.slots.sixtyHi == 0 && strings.Contains(sentence, "5xx") {
					t.Errorf("%q names the 5 MHz bank on a dialect with none", sentence)
				}
				if d.slots.emgWire == "" && strings.Contains(sentence, "EMG") {
					t.Errorf("%q names an emergency channel on a dialect with none", sentence)
				}
				if d.PMSForm() == PMSFormNumeric && strings.Contains(sentence, "P1L") {
					t.Errorf("%q names the token PMS form on a numeric-PMS dialect", sentence)
				}
				// The none form is this dialect's datum in exactly the way
				// the memory range, the PMS domain and the special banks
				// are, and it was the last literal left in the two
				// renderers. It was false on noneWireDialect, whose none
				// form is "900" and whose "000" is an ordinary writable
				// memory channel: the sentence offered "memory 000-005" and
				// rejected "000" in the same breath (Stage 0 close review,
				// seat 2 MEDIUM-3).
				if nw := d.slots.noneWire; nw != "" {
					if !strings.Contains(sentence, strconv.Quote(nw)) {
						t.Errorf("%q does not name this dialect's own none form %q", sentence, nw)
					}
				} else if strings.Contains(sentence, `"000"`) {
					t.Errorf("%q names the FT-710's none form on a dialect that has no none form at all", sentence)
				}
			}
		})
	}
}

// TestMemorySlot_RangeTextNamesItsOwnDialect closes the asymmetry the
// adversarial review recorded as finding L3.
//
// PMSSlot's out-of-range sentence names the RECEIVER's cap (slot.go, "PMS
// pair out of range 1-%d"), which is the principle S0.2 exists to establish:
// a bound is consulted from the same place as its datum. Four lines above
// it, MemorySlot's said "memory channel out of range 1-99" as a literal —
// already false for peerDialect, whose memories are 100-200, and false for
// every future radio that does not happen to share the FT-710's bank.
//
// The FT-710's own sentence does not move a byte, which is the constraint on
// every render this lane touched.
func TestMemorySlot_RangeTextNamesItsOwnDialect(t *testing.T) {
	if _, err := FT710.MemorySlot(0); err == nil {
		t.Fatal("MemorySlot(0) was accepted")
	} else if want := "memory channel out of range 1-99"; !strings.Contains(err.Error(), want) {
		t.Errorf("the FT-710's refusal is %q, want it to contain %q byte for byte", err, want)
	}

	// peerDialect's memories are 100-200, so the literal was simply wrong
	// there. Its own numbers, and a channel inside the FT-710's range that
	// this radio does not have, are the two halves of the same statement.
	if _, err := peerDialect.MemorySlot(50); err == nil {
		t.Fatal("peerDialect.MemorySlot(50) was accepted — its memories start at 100")
	} else if want := "memory channel out of range 100-200"; !strings.Contains(err.Error(), want) {
		t.Errorf("peerDialect's refusal is %q, want it to contain %q", err, want)
	}

	// Stated over every dialect this package can see, so a fixture added
	// later cannot acquire a sentence about somebody else's memory bank.
	for _, nd := range allTestDialects() {
		d := nd.dia
		_, err := d.MemorySlot(d.slots.memoryHi + 1)
		if err == nil {
			t.Errorf("%s: MemorySlot(%d) was accepted, one past its own memoryHi", nd.name, d.slots.memoryHi+1)
			continue
		}
		want := fmt.Sprintf("memory channel out of range %d-%d", d.slots.memoryLo, d.slots.memoryHi)
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: refusal %q does not contain %q", nd.name, err, want)
		}
	}
}
