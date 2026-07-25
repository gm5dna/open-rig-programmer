// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// twoBankCaps builds a minimal Capabilities with two banks for the lookup
// tests below.
func twoBankCaps() Capabilities {
	return Capabilities{
		Model: "TEST-1",
		CATID: "0000",
		Banks: []Bank{
			{
				ID:    BankMemory,
				Label: "Memories",
				Slots: []string{"001", "002"},
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Supported, Write: Supported},
					FieldMode:      {Read: Supported, Write: Unverified},
				},
			},
			{
				ID:      BankPMS,
				Label:   "Scan limits (PMS)",
				Slots:   []string{"P01L", "P01U"},
				NoBlank: true,
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Supported, Write: Unverified},
				},
			},
		},
	}
}

// TestCapabilitiesBank covers Bank() for a present bank and an absent one.
func TestCapabilitiesBank(t *testing.T) {
	caps := twoBankCaps()

	t.Run("present", func(t *testing.T) {
		b, ok := caps.Bank(BankPMS)
		if !ok {
			t.Fatal("Bank(BankPMS) ok = false, want true")
		}
		if b.Label != "Scan limits (PMS)" {
			t.Errorf("Bank(BankPMS).Label = %q, want %q", b.Label, "Scan limits (PMS)")
		}
	})

	t.Run("absent", func(t *testing.T) {
		b, ok := caps.Bank(BankEMG)
		if ok {
			t.Fatalf("Bank(BankEMG) ok = true, want false (not in this Capabilities)")
		}
		if b.ID != "" || b.Label != "" || b.Slots != nil || b.NoBlank || b.Fields != nil {
			t.Errorf("Bank(BankEMG) = %+v, want zero value when absent", b)
		}
	})
}

// TestCapabilitiesBank_DeepCopy checks that Bank() returns a defensive
// copy: mutating the returned Bank's Fields map or Slots slice must
// never reach back into the Capabilities value it came from. Without
// this, a caller holding a Bank returned by one lookup could corrupt
// this project's own hardware-write gate data (Fields) for every other
// caller sharing the same Capabilities value.
func TestCapabilitiesBank_DeepCopy(t *testing.T) {
	caps := twoBankCaps()

	b, ok := caps.Bank(BankMemory)
	if !ok {
		t.Fatal("Bank(BankMemory) ok = false, want true")
	}

	// Mutate the returned copy's map and slice.
	b.Fields[FieldFrequency] = FieldSupport{Read: Unsupported, Write: Unsupported}
	b.Fields[FieldErase] = FieldSupport{Read: Supported, Write: Supported}
	if len(b.Slots) > 0 {
		b.Slots[0] = "TAMPERED"
	}
	b.Slots = append(b.Slots, "999")

	// Re-fetch and confirm the original Capabilities is untouched.
	again, ok := caps.Bank(BankMemory)
	if !ok {
		t.Fatal("Bank(BankMemory) ok = false on second call, want true")
	}
	if again.Fields[FieldFrequency] != (FieldSupport{Read: Supported, Write: Supported}) {
		t.Errorf("Fields[FieldFrequency] = %+v after mutating a prior returned copy, want unaffected {Supported Supported}", again.Fields[FieldFrequency])
	}
	if _, ok := again.Fields[FieldErase]; ok {
		t.Error("Fields[FieldErase] present after being added only to a prior returned copy, want absent")
	}
	if len(again.Slots) != 2 || again.Slots[0] != "001" || again.Slots[1] != "002" {
		t.Errorf("Slots = %v after mutating a prior returned copy, want unaffected [001 002]", again.Slots)
	}

	// Also confirm caps.Banks itself (the underlying storage) is
	// unaffected, not just what a second Bank() call happens to return.
	for _, orig := range caps.Banks {
		if orig.ID != BankMemory {
			continue
		}
		if orig.Fields[FieldFrequency] != (FieldSupport{Read: Supported, Write: Supported}) {
			t.Errorf("caps.Banks[...].Fields[FieldFrequency] = %+v, want unaffected {Supported Supported}", orig.Fields[FieldFrequency])
		}
		if len(orig.Slots) != 2 || orig.Slots[0] != "001" {
			t.Errorf("caps.Banks[...].Slots = %v, want unaffected [001 002]", orig.Slots)
		}
	}
}

// TestCapabilitiesFieldSupport covers FieldSupport() for a present
// bank+field, a present bank with an absent field, and an absent bank
// entirely — all three must degrade to the zero value
// (Unsupported/Unsupported) rather than panicking, per the documented
// zero-value semantics.
func TestCapabilitiesFieldSupport(t *testing.T) {
	caps := twoBankCaps()

	t.Run("present bank, present field", func(t *testing.T) {
		fs := caps.FieldSupport(BankMemory, FieldFrequency)
		if fs.Read != Supported || fs.Write != Supported {
			t.Errorf("FieldSupport(BankMemory, FieldFrequency) = %+v, want {Supported Supported}", fs)
		}
	})

	t.Run("present bank, absent field", func(t *testing.T) {
		fs := caps.FieldSupport(BankMemory, FieldErase)
		if fs != (FieldSupport{}) {
			t.Errorf("FieldSupport(BankMemory, FieldErase) = %+v, want zero value", fs)
		}
		if fs.CanWrite() {
			t.Error("zero-value FieldSupport.CanWrite() = true, want false")
		}
	})

	t.Run("absent bank", func(t *testing.T) {
		fs := caps.FieldSupport(BankEMG, FieldFrequency)
		if fs != (FieldSupport{}) {
			t.Errorf("FieldSupport(BankEMG, FieldFrequency) = %+v, want zero value", fs)
		}
	})
}

// TestExampleFT710Shape constructs a miniature Capabilities the way the
// FT-710 driver will: a handful of banks, each with a small Fields map,
// then exercises the same lookups generic code (UI, validation, clone
// service) will perform. This documents the intended usage of this
// package: capability data drives generic code, so nothing here should
// need to know FT-710-specific wire details.
func TestExampleFT710Shape(t *testing.T) {
	tones := StandardCTCSSTones()
	caps := Capabilities{
		Model:         "FT-710",
		CATID:         "0800",
		Modes:         []string{"LSB", "USB", "CW-U", "FM", "AM"},
		TagLen:        12,
		ClarMaxHz:     9990,
		ClarStepHz:    10,
		CTCSSTones:    tones[:],
		Bauds:         []int{4800, 9600, 19200, 38400, 115200},
		DefaultBaud:   38400,
		MinFreqHz:     30000,
		MaxFreqHz:     56000000,
		RequiredSlots: []string{"001"},
		Banks: []Bank{
			{
				ID:    BankMemory,
				Label: "Memories",
				Slots: []string{"001", "002", "003"},
				Fields: map[Field]FieldSupport{
					FieldFrequency:  {Read: Supported, Write: Supported},
					FieldMode:       {Read: Supported, Write: Supported},
					FieldTag:        {Read: Supported, Write: Unverified},
					FieldTagDisplay: {Read: Supported, Write: Unverified},
					FieldScanSkip:   {Read: Supported, Write: Unverified},
					FieldErase:      {Read: Unsupported, Write: Unverified},
				},
			},
			{
				ID:      BankPMS,
				Label:   "Scan limits (PMS)",
				Slots:   []string{"P01L", "P01U"},
				NoBlank: true,
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Supported, Write: Unverified},
				},
			},
			{
				ID:    BankEMG,
				Label: "Emergency",
				Slots: []string{"E01"},
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Unverified, Write: Unverified},
				},
			},
		},
	}

	// A UI populating a tone picker would use the shared table directly.
	if len(caps.CTCSSTones) != 50 {
		t.Fatalf("CTCSSTones has %d entries, want 50", len(caps.CTCSSTones))
	}

	// A validation layer deciding whether to enable a "write frequency"
	// button for the Memories bank: this field is Supported for write.
	if fs := caps.FieldSupport(BankMemory, FieldFrequency); !fs.CanWrite() {
		t.Error("Memories/Frequency should be writable in this example shape")
	}

	// The same check for PMS: Frequency there is Unverified for write, so
	// the hardware gate must keep it disabled even though PMS is
	// NoBlank/read-capable.
	if fs := caps.FieldSupport(BankPMS, FieldFrequency); fs.CanWrite() {
		t.Error("PMS/Frequency is Unverified for write and must not be writable (hardware gate)")
	}

	// NoBlank is a per-bank property a clone/erase tool would consult
	// before allowing a slot in that bank to be cleared.
	pms, ok := caps.Bank(BankPMS)
	if !ok {
		t.Fatal("Bank(BankPMS) not found")
	}
	if !pms.NoBlank {
		t.Error("PMS bank should be NoBlank in this example shape")
	}

	// A bank this Capabilities doesn't model (e.g. Bank60m) must degrade
	// safely rather than panic.
	if _, ok := caps.Bank(Bank60m); ok {
		t.Error("Bank(Bank60m) should be absent in this example shape")
	}

	// MinFreqHz/MaxFreqHz describe the radio's tuning range floor/ceiling —
	// a validation layer uses these, together with a channel's FreqHz, to
	// reject out-of-range frequencies.
	if caps.MinFreqHz != 30000 || caps.MaxFreqHz != 56000000 {
		t.Errorf("MinFreqHz/MaxFreqHz = %d/%d, want 30000/56000000", caps.MinFreqHz, caps.MaxFreqHz)
	}

	// RequiredSlots lists canonical slot wire forms that must never be
	// empty, e.g. FT-710 M-01 ("001") — distinct from Bank.NoBlank, which
	// governs a whole bank (e.g. PMS) rather than one individual slot.
	if len(caps.RequiredSlots) != 1 || caps.RequiredSlots[0] != "001" {
		t.Errorf("RequiredSlots = %v, want [001]", caps.RequiredSlots)
	}
}
