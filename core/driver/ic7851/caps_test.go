// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestConstructors(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func() interface {
			Model() string
			Capabilities() spec.Capabilities
		}
	}{
		{"7851", func() interface {
			Model() string
			Capabilities() spec.Capabilities
		} {
			return New7851()
		}},
		{"7850", func() interface {
			Model() string
			Capabilities() spec.Capabilities
		} {
			return New7850()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.make()
			c := d.Capabilities()
			want := "IC-" + tc.name
			if d.Model() != want {
				t.Fatalf("Model = %q", d.Model())
			}
			if c.CATID != "8e" || c.Transmit != spec.HasTransmitter || c.TagLen != 10 {
				t.Fatalf("capabilities = %+v", c)
			}
			if err := c.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestCapabilitiesShape(t *testing.T) {
	c := New7851().Capabilities()
	mem, ok := c.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
		t.Fatalf("MEM = %+v", mem)
	}
	scan, ok := c.Bank(spec.BankScan)
	if !ok || len(scan.Slots) != 2 || scan.Slots[0] != "P1" || scan.Slots[1] != "P2" {
		t.Fatalf("SCAN = %+v", scan)
	}
	if _, ok := c.Bank(spec.BankCall); ok {
		t.Fatal("CALL bank unexpectedly present")
	}
	for _, b := range c.Banks {
		if b.Fields[spec.FieldScanSkip].CanWrite() || b.Fields[spec.FieldDataMode].CanWrite() {
			t.Fatal("lossy E6 field writable")
		}
	}
}

// TestCapabilityValuesArePinnedToTheMatrix is F2.
//
// Every value here is a row of the reviewed capability matrix, and each is
// pinned EXACTLY rather than by a bound: a capability that is merely
// "wide enough" authorises, after consent, a frequency this radio cannot
// receive and a tone that is not on its chart.
func TestCapabilityValuesArePinnedToTheMatrix(t *testing.T) {
	for _, caps := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
		// §1 row 12 — receiver coverage floor 0.030000 MHz.
		if caps.MinFreqHz != 30_000 {
			t.Errorf("MinFreqHz = %d, want 30_000 (matrix §1 row 12, PDF p.267's \"Receiver 0.030000–60.000000\")", caps.MinFreqHz)
		}
		// §1 row 13 — the CHOICE is the narrower, radio-true ceiling.
		if caps.MaxFreqHz != 60_000_000 {
			t.Errorf("MaxFreqHz = %d, want 60_000_000 (matrix §1 row 13: the RADIO ceiling is the declared one, not the 69_999_999 the field could encode)", caps.MaxFreqHz)
		}
		// §1 row 9 — the 50 selectable tones run 67.0 to 254.1 Hz; the
		// step is the row's one ASSUMED half.
		want := spec.ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1}
		if caps.CTCSSToneRange == nil || *caps.CTCSSToneRange != want {
			t.Errorf("CTCSSToneRange = %+v, want %+v (matrix §1 row 9, PDF p.115's \"Selectable tone frequencies\")", caps.CTCSSToneRange, want)
		}
		// §1b.3 — PDF p.181's capability table gives the scan-edge row
		// CLEAR = "No" and the regular row CLEAR = "Yes".
		mem, _ := caps.Bank(spec.BankMemory)
		if mem.NoBlank {
			t.Error("MEM.NoBlank = true, want false (matrix §1b.3: the Regular row's CLEAR column reads \"Yes\")")
		}
		scan, _ := caps.Bank(spec.BankScan)
		if !scan.NoBlank {
			t.Error("SCAN.NoBlank = false, want true (matrix §1b.3: the Scan Edge row's CLEAR column reads \"No\")")
		}
		if err := caps.Validate(); err != nil {
			t.Errorf("Validate: %v", err)
		}
	}
}

// TestPreBuildRefusalEnforcesTheCapabilityBounds is F2's other half: a
// declared bound that nothing enforces is not a bound.
//
// The refusals are read off THIS SESSION'S capabilities rather than from
// package constants, so the domain a UI offers, the domain
// codeplug.Validate judges and the domain this driver admits are one set
// of numbers. Rung 4 precedes the preservation read, so a nil engine here
// is not reached — which is itself the proof that the refusal costs no
// wire traffic.
func TestPreBuildRefusalEnforcesTheCapabilityBounds(t *testing.T) {
	caps := spec.ConsentUnverifiedWrites(capabilitiesUnverified())
	s := &Session{caps: caps}
	base := codeplug.ChannelData{
		FreqHz:   14_250_000,
		Mode:     "USB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(1000)},
		Tag:      "ALPHA",
	}
	for _, tc := range []struct {
		name   string
		mutate func(*codeplug.ChannelData)
		field  spec.Field
	}{
		{"a frequency above the radio's 60 MHz ceiling", func(d *codeplug.ChannelData) { d.FreqHz = 60_000_001 }, spec.FieldFrequency},
		{"a frequency below the radio's 30 kHz floor", func(d *codeplug.ChannelData) { d.FreqHz = 29_999 }, spec.FieldFrequency},
		{"a repeater tone above the printed chart", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(2542)}
		}, spec.FieldToneTx},
		{"a repeater tone below the printed chart", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(669)}
		}, spec.FieldToneTx},
		{"a tone squelch above the printed chart", func(d *codeplug.ChannelData) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(2542)}
		}, spec.FieldToneRx},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			tc.mutate(&d)
			res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: &d})
			var e *OutOfDomainError
			if !errors.As(err, &e) {
				t.Fatalf("WriteChannel = %v, want *OutOfDomainError", err)
			}
			if e.Field != tc.field {
				t.Errorf("refusal named %s, want %s", e.Field, tc.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want none: the refusal precedes all wire traffic", res.Steps)
			}
		})
	}

	// AND THE BOUNDS THEMSELVES ARE ADMITTED. A refusal at exactly the
	// declared edge would be a different bug from the one F2 names, so
	// both edges are exercised for absence of *OutOfDomainError. The
	// write goes no further than rung 5 here — the nil engine panics if
	// it ever gets that far, which is the assertion.
	for _, tc := range []struct {
		name   string
		mutate func(*codeplug.ChannelData)
	}{
		{"the floor itself", func(d *codeplug.ChannelData) { d.FreqHz = 30_000 }},
		{"the ceiling itself", func(d *codeplug.ChannelData) { d.FreqHz = 60_000_000 }},
		{"the lowest chart tone", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(670)}
		}},
		{"the highest chart tone", func(d *codeplug.ChannelData) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(2541)}
		}},
	} {
		t.Run(tc.name+" is admitted", func(t *testing.T) {
			d := base
			tc.mutate(&d)
			if err := domainRefusal(d, caps); err != nil {
				t.Fatalf("the declared edge was refused: %v", err)
			}
		})
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete7851 || writeTrialsComplete7850 {
		t.Fatal("write trial guard unlocked")
	}
	// AND NEITHER ARM IS WRITABLE WITHOUT CONSENT while they are false.
	// The constants are not read by Capabilities, deliberately, so this is
	// what actually holds the fail-safe down.
	for _, caps := range []spec.Capabilities{New7851().Capabilities(), New7850().Capabilities()} {
		for _, b := range caps.Banks {
			for f, sup := range b.Fields {
				if sup.CanWrite() {
					t.Errorf("%s is writable on bank %s with both write-trial guards false", f, b.ID)
				}
			}
		}
	}
}

// TestBaseline_Validate runs the shared validator over every capability
// set this package can hand out, including the consented transforms: an
// invalid set is a set no registry would accept and no UI could render.
func TestBaseline_Validate(t *testing.T) {
	for name, caps := range map[string]spec.Capabilities{
		"unverified":            capabilitiesUnverified(),
		"simulated":             capabilitiesSimulated(),
		"unverified+consent":    spec.ConsentUnverifiedWrites(capabilitiesUnverified()),
		"simulated+consent":     spec.ConsentUnverifiedWrites(capabilitiesSimulated()),
		"IC-7851 constructor":   New7851().Capabilities(),
		"IC-7850 constructor":   New7850().Capabilities(),
		"IC-7851 simulated arm": New7851(WithSimulatedProfile()).Capabilities(),
	} {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestModes_MatchTheCodec and TestFilters_MatchTheCodec pin the
// vocabularies caps.go writes out against the codec's own enum tables.
//
// THE DUPLICATION IS DELIBERATE AND THESE TESTS ARE WHAT MAKE IT SAFE.
// core/civ/ic7851's enum maps live inside a frozen, evidence-bound
// package; the capability lists are the UI's vocabulary and its order is a
// display choice. Writing them out here and pinning them against the codec
// keeps one radio's reading in one place without the driver reaching into
// the codec's tables at run time.
func TestModes_MatchTheCodec(t *testing.T) {
	checkVocabulary(t, civ.FieldMode, New7851().Capabilities().Modes)
}
func TestFilters_MatchTheCodec(t *testing.T) {
	checkVocabulary(t, civ.FieldFilter, New7851().Capabilities().Filters)
}
func TestToneModes_MatchTheCodec(t *testing.T) {
	caps := New7851().Capabilities()
	names := make([]string, len(caps.ToneModes))
	for i, m := range caps.ToneModes {
		names[i] = m.Value
	}
	checkVocabulary(t, civ.FieldToneMode, names)
}

func checkVocabulary(t *testing.T, field civ.FieldID, declared []string) {
	t.Helper()
	var codec map[byte]string
	for _, sp := range civic7851.Profile().Layouts()[0].Fields {
		if sp.Field == field {
			codec = sp.Enum
		}
	}
	if codec == nil {
		t.Fatalf("the codec maps no %s span", field)
	}
	if len(codec) != len(declared) {
		t.Fatalf("the capability declares %d %s values and the codec maps %d: %v vs %v", len(declared), field, len(codec), declared, codec)
	}
	have := map[string]bool{}
	for _, v := range declared {
		if have[v] {
			t.Errorf("the capability declares %s %q twice", field, v)
		}
		have[v] = true
	}
	for code, name := range codec {
		if !have[name] {
			t.Errorf("the codec decodes %#02x as %s %q, which the capability does not declare — a record this driver can READ would then carry a value its own UI refuses", code, field, name)
		}
	}
}

func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, caps := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
		v := reflect.ValueOf(caps)
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			zero := v.Field(i).IsZero() || (v.Field(i).Kind() == reflect.Slice && v.Field(i).Len() == 0)
			_, listed := deliberatelyZero[name]
			if zero != listed {
				t.Errorf("%s zero=%v listed=%v", name, zero, listed)
			}
		}
	}
}
