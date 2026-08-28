// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is every spec.Field this project models, and the LENGTH
// ASSERTION below is what stops a newly added Field escaping the write
// guard: a twenty-first constant would leave this slice short, the pin
// would fail, and whoever added it has to decide what this radio does
// with it rather than inheriting a silent Unsupported.
//
// Ten from the pre-Icom model and ten the Icom tier added
// (core/spec/field.go), in that file's own declaration order.
var allFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldClarifier,
	spec.FieldCTCSSState,
	spec.FieldCTCSSTone,
	spec.FieldShift,
	spec.FieldTag,
	spec.FieldTagDisplay,
	spec.FieldScanSkip,
	spec.FieldErase,

	spec.FieldTxFrequency,
	spec.FieldDuplex,
	spec.FieldOffset,
	spec.FieldToneMode,
	spec.FieldToneTx,
	spec.FieldToneRx,
	spec.FieldDTCSCode,
	spec.FieldDTCSPolarity,
	spec.FieldFilter,
	spec.FieldDataMode,
}

func TestAllFieldsCoversEverySpecField(t *testing.T) {
	if len(allFields) != 20 {
		t.Fatalf("allFields has %d entries, want 20 — core/spec/field.go declares ten pre-tier Fields and ten the Icom tier added; a Field missing from this slice is a Field the write guard below never checks", len(allFields))
	}
	seen := map[spec.Field]bool{}
	for _, f := range allFields {
		if seen[f] {
			t.Errorf("allFields lists %s twice", f)
		}
		seen[f] = true
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true for the IC-7300: no IC-7300 has ever been asked anything by this project (matrix §3.14), so no write trial has completed. If a trial really has been performed, revert this test alongside the flip and state the evidence.")
	}
	caps := New(RealHardware).Capabilities()
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true on the RealHardware profile — the write guard is broken", b.ID, f)
			}
		}
	}
}

// REQUIRES ENABLER E5b. At 57a1188 core/spec/validate.go:388-390 appended
// "ShiftOptions must not be empty" whenever BOTH ShiftOptions and
// DuplexOptions were empty — which is this pair's honest shape, since spec
// D6 puts per-channel duplex/offset out of scope and both matrices grade
// duplex a MANUAL-EVIDENCED absence. Inventing a dummy duplex vocabulary to
// satisfy the validator would be dishonest and is refused: this test cannot
// pass before E5b lands, and that is a sequencing fact, not a defect in the
// capabilities.
func TestCapabilities_Validate(t *testing.T) {
	for _, p := range []Profile{RealHardware, Simulated} {
		if err := New(p).Capabilities().Validate(); err != nil {
			t.Errorf("profile %v: Capabilities().Validate(): %v — if this is the ShiftOptions/DuplexOptions non-empty rule, enabler E5b has not landed and Stage 2 started too early", p, err)
		}
	}
}

func TestCapabilities_TheNumbersFromTheMatrix(t *testing.T) {
	caps := New(RealHardware).Capabilities()
	if caps.Model != "IC-7300" {
		t.Errorf("Model = %q, want %q", caps.Model, "IC-7300")
	}
	if caps.CATID != "94" {
		t.Errorf("CATID = %q, want %q (spec D3.2: the address hex, plus the observed 19 00 token, which is recorded at Open and never matched)", caps.CATID, "94")
	}
	if caps.TagLen != 10 {
		t.Errorf("TagLen = %d, want 10", caps.TagLen)
	}
	if caps.DefaultBaud != 19200 {
		t.Errorf("DefaultBaud = %d, want 19200 — the factory setting is Auto, and 19200 is the highest rate present in BOTH the [USB] and [REMOTE] lists, so it survives CI-V USB Port = Link to [REMOTE] (matrix §3.16 A4)", caps.DefaultBaud)
	}
	if caps.MinFreqHz != 30_000 {
		t.Errorf("MinFreqHz = %d, want 30000", caps.MinFreqHz)
	}
	if caps.MaxFreqHz != 69_999_999 {
		t.Errorf("MaxFreqHz = %d, want 69999999 — the RECORD's frequency field caps the 10 MHz digit at 6 (PDF p.167); the 74.8 MHz figure is tuning coverage, not what a memory channel can store (plan decision D6)", caps.MaxFreqHz)
	}
	if len(caps.Banks) != 2 {
		t.Fatalf("len(Banks) = %d, want 2 (MEM and SCAN)", len(caps.Banks))
	}
	if got := caps.FieldSupport(spec.BankMemory, spec.FieldScanSkip); got.Read != spec.Unsupported || got.Write != spec.Unsupported {
		t.Errorf("MEM/scan_skip = %+v, want Unsupported both ways — the ③ SELECT nibble is group MEMBERSHIP, the inverse of a skip flag, and the tier forbids the mapping (plan decision D4)", got)
	}
	if got := caps.FieldSupport(spec.BankMemory, spec.FieldErase); got.Read != spec.Unsupported || got.Write != spec.Unsupported {
		t.Errorf("MEM/erase = %+v, want Unsupported both ways — this tier ships no erase path (spec D4, adjudication 19)", got)
	}
	if got := caps.FieldSupport(spec.BankMemory, spec.FieldTagDisplay); got.Read != spec.Unsupported || got.Write != spec.Unsupported {
		t.Errorf("MEM/tag_display = %+v, want the ZERO FieldSupport both ways — this record carries no display flag, so the field reads back Unavailable, exactly as core/driver/ftdx10/caps.go:194 and ftdx101/caps.go:225 grade it on their flagless records (R13; plan decision D5, REV 2)", got)
	}
	// D16: the tone DOMAIN is E3's numeric range, not a list. An empty
	// CTCSSTones with no range declared fails CLOSED on every Known tone
	// (core/codeplug/fieldstate.go's ToneField.Valid, via
	// spec.Capabilities.AdmitsTone), and every read produces Known tones.
	if len(caps.CTCSSTones) != 0 {
		t.Errorf("CTCSSTones has %d entries, want 0 — this model declares E3's numeric range instead, because the record stores a BCD FREQUENCY and indexes no table (matrix §1 row 8's own words); E3 forbids declaring both", len(caps.CTCSSTones))
	}
	if caps.CTCSSToneRange == nil {
		t.Fatal("CTCSSToneRange is nil — with neither a list nor a range, spec.Capabilities.AdmitsTone fails closed and every tone this driver reads back would be refused by codeplug.ToneField.Valid (D16, T1(2))")
	}
	if want := (spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1}); *caps.CTCSSToneRange != want {
		t.Errorf("CTCSSToneRange = %+v, want %+v — the printed legend gives 0..2999 deciHz on a 0.1 Hz grid (PDF p.171), intersected with the capability floor of 1 because 0 Hz is not a tone (T1(2))", *caps.CTCSSToneRange, want)
	}
	if got := caps.FieldSupport(spec.BankMemory, spec.FieldDuplex); got.Read != spec.Unsupported {
		t.Errorf("MEM/duplex = %+v, want Unsupported — MANUAL-EVIDENCED absence (matrix §1b)", got)
	}
}

func TestConsentOpensExactlyTheWritableFields(t *testing.T) {
	base := New(RealHardware).Capabilities()
	consented := spec.ConsentUnverifiedWrites(base)
	if consented.FieldSupport(spec.BankMemory, spec.FieldErase).CanWrite() {
		t.Error("erase CanWrite() after consent — spec.ConsentUnverifiedWrites must never consent erase (spec D4, adjudication 19)")
	}
	for _, f := range []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag, spec.FieldFilter, spec.FieldDataMode, spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx, spec.FieldTxFrequency} {
		if !consented.FieldSupport(spec.BankMemory, f).CanWrite() {
			t.Errorf("field %s: CanWrite() = false after consent — this field IS in the 1A 00 record and consent must open it", f)
		}
	}
}

// TestCapabilities_EveryFieldExplicit reflects over spec.Capabilities and
// requires every field either to hold a non-zero value or to appear in
// deliberatelyZero. A zero left in a capability field is not a neutral
// omission: a zero MaxFreqHz reads as "no ceiling" to every validator, and a
// zero TagLen makes CHIRP import truncate every imported name to nothing and
// report it as an approximated loss rather than refusing.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// Each entry is a field this radio genuinely has no value for, with the
	// matrix reading that says so. Adding a name here is a decision; leaving
	// a field out of the struct is not.
	deliberatelyZero := map[string]string{
		"ClarMaxHz":              "no clarifier/RIT field in the 1A 00 record (matrix §1 row 6, poor fit, graded)",
		"ClarStepHz":             "as ClarMaxHz (matrix §1 row 7)",
		"ShiftOptions":           "no shift or duplex field exists on this model (matrix §1 row 14)",
		"CTCSSStates":            "displaced by ToneModes on Icom models (spec D4; matrix §1 row 15)",
		"DuplexOptions":          "MANUAL-EVIDENCED absence (matrix §1b, duplex)",
		"DTCSCodes":              "the record carries no DTCS field at all; MANUAL-EVIDENCED absence (matrix §1b, dtcs_code)",
		"TuningSteps":            "additions design D8 — this record carries no receiver tuning-step field",
		"ProgramTuningStepRange": "additions design D8 — this record carries no programmable tuning-step field",
		"AttenuatorDB":           "additions design D8 — this record carries no attenuator field",
		"PreampOptions":          "additions design D8 — this record carries no preamp field",
		"AntennaOptions":         "additions design D8 — this record carries no antenna field",
		"DTCSPolarities":         "the record carries no DTCS field at all; MANUAL-EVIDENCED absence (matrix §1b, dtcs_polarity)",
		"RequiredSlots":          "nothing in this document is declared never-empty (matrix §1 row 13; plan decision D8)",
		"CTCSSTones":             "the tone domain is CTCSSToneRange {1, 2999, 1} deciHz, not a list; E3 forbids declaring both (D16; matrix §1 row 8, deviation + erratum 3)",
	}
	caps := New(RealHardware).Capabilities()
	v := reflect.ValueOf(caps)
	ty := v.Type()
	if ty.NumField() == 0 {
		t.Fatal("spec.Capabilities has no fields — this test would pass vacuously")
	}
	// THE COUNT ITSELF, pinned rather than left to prose. The audit below
	// only asks that each field be non-zero OR named in deliberatelyZero, so
	// a NEW capability field arriving with a plausible zero value and no
	// entry would be caught — but a field REMOVED, or the struct reshaped,
	// would not, so the exact shape remains pinned alongside the table.
	if ty.NumField() != 27 {
		t.Errorf("spec.Capabilities has %d fields, want 27 — every one of them is written down explicitly in baseCapabilities, and the count is stated in caps.go and doc.go; if the struct has genuinely changed, set the new value HERE and account for the new field in the literal and in deliberatelyZero", ty.NumField())
	}
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if v.Field(i).IsZero() {
			if _, ok := deliberatelyZero[name]; !ok {
				t.Errorf("Capabilities.%s is the zero value and is not in deliberatelyZero — set it, or record why it is zero", name)
			}
			continue
		}
		if why, ok := deliberatelyZero[name]; ok {
			t.Errorf("Capabilities.%s is NON-zero but deliberatelyZero says %q — one of the two is now wrong", name, why)
		}
	}
}

// The bank inventory D11 fixes: MEM 001..099 and SCAN P1/P2, flat, dense,
// with no CALL bank and no group addressing.
func TestBanks_TheInventoryFromD11(t *testing.T) {
	caps := New(RealHardware).Capabilities()
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	if len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
		t.Errorf("MEM slots = %d entries, first %q last %q, want 99 entries 001..099", len(mem.Slots), mem.Slots[0], mem.Slots[len(mem.Slots)-1])
	}
	scan, ok := caps.Bank(spec.BankScan)
	if !ok {
		t.Fatal("no SCAN bank")
	}
	if len(scan.Slots) != 2 || scan.Slots[0] != "P1" || scan.Slots[1] != "P2" {
		t.Errorf("SCAN slots = %v, want [P1 P2] — what both manuals print (D11)", scan.Slots)
	}
	for _, b := range caps.Banks {
		if b.Sparse || b.Groups != 0 || b.PerGroup != 0 || b.Budget != 0 {
			t.Errorf("bank %s declares a sparse space (%+v) — this model is flat-addressed and dense (D11)", b.ID, b)
		}
		if b.NoBlank {
			t.Errorf("bank %s has NoBlank = true — this document says nothing about whether P1/P2 may be blank (matrix §1b, ic7300-scan-edge-noblank); the MK2's own document does, and its NoBlank is not this model's (D8)", b.ID)
		}
	}
}

// The tag charset is TAKEN FROM the profile rather than restated, so the
// driver's advertised charset and the codec's cannot drift.
func TestTagCharsetComesFromTheProfile(t *testing.T) {
	caps := New(RealHardware).Capabilities()
	if len(caps.TagCharset) != 95 {
		t.Errorf("TagCharset is %d bytes, want 95 (space + 10 digits + 26 upper + 26 lower + 32 symbols)", len(caps.TagCharset))
	}
	for i := 0; i < len(caps.TagCharset); i++ {
		if !caps.TagByteOK(caps.TagCharset[i]) {
			t.Errorf("TagCharset byte %#02x is not accepted by TagByteOK", caps.TagCharset[i])
		}
	}
}
