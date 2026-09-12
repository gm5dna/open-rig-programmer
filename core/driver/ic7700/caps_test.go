// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7700 "github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestConstructors(t *testing.T) {
	d := New(RealHardware)
	c := d.Capabilities()
	if d.Model() != "IC-7700" {
		t.Fatalf("Model = %q", d.Model())
	}
	if c.CATID != "74" || c.Transmit != spec.HasTransmitter || c.TagLen != 10 {
		t.Fatalf("capabilities = %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestCapabilitiesShape(t *testing.T) {
	c := New(RealHardware).Capabilities()
	mem, ok := c.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
		t.Fatalf("MEM = %+v", mem)
	}
	scan, ok := c.Bank(spec.BankScan)
	if !ok || len(scan.Slots) != 2 || scan.Slots[0] != "P1" || scan.Slots[1] != "P2" {
		t.Fatalf("SCAN = %+v", scan)
	}
	if _, ok := c.Bank(spec.BankCall); ok {
		t.Fatal("CALL bank unexpectedly present (matrix §1 row 5: no CALL bank on this model)")
	}
	for _, b := range c.Banks {
		if b.Fields[spec.FieldScanSkip].CanWrite() || b.Fields[spec.FieldDataMode].CanWrite() {
			t.Fatal("lossy E6 field writable")
		}
	}
}

// TestCapabilityValuesArePinnedToTheMatrix pins every declared numeric
// bound to the matrix row it came from, exactly rather than by a loose
// range: a capability that is merely "wide enough" authorises, after
// consent, a frequency or tone this radio's record cannot even encode.
func TestCapabilityValuesArePinnedToTheMatrix(t *testing.T) {
	for _, caps := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
		if caps.MinFreqHz != 0 {
			t.Errorf("MinFreqHz = %d, want 0 (matrix §1 row 15: the encoding floor)", caps.MinFreqHz)
		}
		if caps.MaxFreqHz != 69_999_999 {
			t.Errorf("MaxFreqHz = %d, want 69_999_999 (matrix §1 row 16: the encoding ceiling)", caps.MaxFreqHz)
		}
		want := spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2549, StepDeciHz: 1}
		if caps.CTCSSToneRange == nil || *caps.CTCSSToneRange != want {
			t.Errorf("CTCSSToneRange = %+v, want %+v (matrix §1 rows 11-12, PDF p.211's per-digit leaders)", caps.CTCSSToneRange, want)
		}
		mem, _ := caps.Bank(spec.BankMemory)
		if mem.NoBlank {
			t.Error("MEM.NoBlank = true, want false (matrix §1b Bank descriptors)")
		}
		scan, _ := caps.Bank(spec.BankScan)
		if scan.NoBlank {
			t.Error("SCAN.NoBlank = true, want false (matrix §1b Bank descriptors: a CHOICE, not lifted despite the CLEAR:No data point)")
		}
		if caps.Bauds == nil || len(caps.Bauds) != 5 || caps.Bauds[len(caps.Bauds)-1] != 19200 {
			t.Errorf("Bauds = %v, want {300,1200,4800,9600,19200} (matrix §1 row 13)", caps.Bauds)
		}
		if caps.DefaultBaud != 19200 {
			t.Errorf("DefaultBaud = %d, want 19200 (CHOICE, matrix §1 row 14/§3.3 ADDED-1: the factory default is \"Auto\", which DefaultBaud int cannot represent)", caps.DefaultBaud)
		}
		if caps.SimplexTx != spec.SimplexTxEqualsRx {
			t.Errorf("SimplexTx = %v, want SimplexTxEqualsRx (this driver's own write-time mirror policy)", caps.SimplexTx)
		}
		if err := caps.Validate(); err != nil {
			t.Errorf("Validate: %v", err)
		}
	}
}

// TestPreBuildRefusalEnforcesTheCapabilityBounds: a declared bound that
// nothing enforces is not a bound. The refusals are read off THIS
// SESSION'S capabilities so the domain a UI offers, the domain
// codeplug.Validate judges and the domain this driver admits are one set
// of numbers.
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
		{"a frequency above the record's 69 999 999 Hz ceiling", func(d *codeplug.ChannelData) { d.FreqHz = 70_000_000 }, spec.FieldFrequency},
		{"a repeater tone above the record's BCD capacity", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(2550)}
		}, spec.FieldToneTx},
		{"a tone squelch above the record's BCD capacity", func(d *codeplug.ChannelData) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(2550)}
		}, spec.FieldToneRx},
		{"a repeater tone below the record's BCD floor", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(0)}
		}, spec.FieldToneTx},
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

	for _, tc := range []struct {
		name   string
		mutate func(*codeplug.ChannelData)
	}{
		{"the floor itself", func(d *codeplug.ChannelData) { d.FreqHz = 0 }},
		{"the ceiling itself", func(d *codeplug.ChannelData) { d.FreqHz = 69_999_999 }},
		{"the lowest encodable tone", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(1)}
		}},
		{"the highest encodable tone", func(d *codeplug.ChannelData) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(2549)}
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
	if writeTrialsComplete {
		t.Fatal("write trial guard unlocked")
	}
	for _, b := range New(RealHardware).Capabilities().Banks {
		for f, sup := range b.Fields {
			if sup.CanWrite() {
				t.Errorf("%s is writable on bank %s with the write-trial guard false", f, b.ID)
			}
		}
	}
}

// TestBaseline_Validate runs the shared validator over every capability
// set this package can hand out, including the consented transforms.
func TestBaseline_Validate(t *testing.T) {
	for name, caps := range map[string]spec.Capabilities{
		"unverified":          capabilitiesUnverified(),
		"simulated":           capabilitiesSimulated(),
		"unverified+consent":  spec.ConsentUnverifiedWrites(capabilitiesUnverified()),
		"simulated+consent":   spec.ConsentUnverifiedWrites(capabilitiesSimulated()),
		"IC-7700 constructor": New(RealHardware).Capabilities(),
		"IC-7700 simulated":   New(Simulated).Capabilities(),
	} {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestModes_MatchTheCodec and TestFilters_MatchTheCodec pin the
// vocabularies caps.go writes out against the codec's own enum tables.
func TestModes_MatchTheCodec(t *testing.T) {
	checkVocabulary(t, civ.FieldMode, New(RealHardware).Capabilities().Modes)
}
func TestFilters_MatchTheCodec(t *testing.T) {
	checkVocabulary(t, civ.FieldFilter, New(RealHardware).Capabilities().Filters)
}
func TestToneModes_MatchTheCodec(t *testing.T) {
	caps := New(RealHardware).Capabilities()
	names := make([]string, len(caps.ToneModes))
	for i, m := range caps.ToneModes {
		names[i] = m.Value
	}
	checkVocabulary(t, civ.FieldToneMode, names)
}

func checkVocabulary(t *testing.T, field civ.FieldID, declared []string) {
	t.Helper()
	var codec map[byte]string
	for _, sp := range civic7700.Profile().Layouts()[0].Fields {
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
