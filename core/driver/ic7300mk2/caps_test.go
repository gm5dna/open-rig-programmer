// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is the ten spec.Fields of the pre-Icom model plus the ten the
// Icom tier added, in core/spec/field.go's own declaration order.
//
// IT IS NOT EVERY spec.Field, and the comment here claimed it was: the
// additions design's D8 minted seven receiver fields on 28/08/2026, taking
// core/spec/field.go to twenty-seven, and the LENGTH ASSERTION below did not
// notice — it compares against a written-down twenty rather than against
// field.go. So the drift alarm this comment used to promise ("a twenty-first
// constant would leave this slice short") no longer holds, and the seven D8
// receiver fields are outside the write guard below. They are graded absent
// on this transceiver, so the guard's CONCLUSION is not thought to be wrong;
// it is simply not checked here. Closing that needs a spec-side enumeration
// this package cannot mint on its own.
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
		t.Fatalf("allFields has %d entries, want 20 — the ten pre-tier Fields plus the ten the Icom tier added; a Field missing from this slice is a Field the write guard below never checks. This is NOT core/spec/field.go's whole count: D8 took that to twenty-seven on 28/08/2026 and this pin does not track it", len(allFields))
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
		t.Fatal("writeTrialsComplete = true for the IC-7300MK2: no IC-7300MK2 has ever been asked anything by this project. Matrix §3.14 states this model's FALSE in its own terms — \"The registered sibling's FALSE is not stated here\" — so the IC-7300's pin lifts nothing for this radio and vice versa. If a trial really has been performed ON AN IC-7300MK2, revert this test alongside the flip and state the evidence.")
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
	if caps.Model != "IC-7300MK2" {
		t.Errorf("Model = %q, want %q", caps.Model, "IC-7300MK2")
	}
	if caps.CATID != "b6" {
		t.Errorf("CATID = %q, want %q (spec D3.2: the address hex, plus the observed 19 00 token, which is recorded at Open and never matched; lowercase to match the runtime Identity.CATID form Go's \"%%02x\" verb always produces, and what core/clone persists into a codeplug file)", caps.CATID, "b6")
	}
	if caps.TagLen != 16 {
		t.Errorf("TagLen = %d, want 16 — ⑱ ~ ㉝ is sixteen bytes on this model, against the IC-7300's ten", caps.TagLen)
	}
	if caps.DefaultBaud != 19200 {
		t.Errorf("DefaultBaud = %d, want 19200 — this document prints NO rate list and NO default (matrix §1 #9, #10, §3.3). The only rates it names anywhere are the three rows of the 18 01 FE-count table, which is a WAKE-UP-COMMAND table and not a supported-rate list; publishing those three and opening at the highest is the conservative derivation (D7)", caps.DefaultBaud)
	}
	if got, want := caps.Bauds, []int{4800, 9600, 19200}; !slices.Equal(got, want) {
		t.Errorf("Bauds = %v, want %v — the three rows of the 18 01 FE-count table, and nothing borrowed from the sibling's [USB] list", got, want)
	}
	if caps.MinFreqHz != 0 {
		t.Errorf("MinFreqHz = %d, want 0 — this document prints NO tuning floor anywhere (matrix §1 #11), and taking the IC-7300's 30 000 would be exactly the cross-model contamination both matrices' §4 forbid. A zero DISABLES the lower-bound check (core/spec/capabilities.go); it does not assert a known 0 Hz floor", caps.MinFreqHz)
	}
	if caps.MaxFreqHz != 79_999_999 {
		t.Errorf("MaxFreqHz = %d, want 79999999 — the ENCODING ceiling, MANUAL-EVIDENCED at PDF p.16: 10 MHz digit 0 ~ 7, with the 1 GHz and 100 MHz digits printed fixed 0 (matrix §1 #12)", caps.MaxFreqHz)
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
		t.Errorf("MEM/tag_display = %+v, want the ZERO FieldSupport both ways — this record carries no display flag either, so the field reads back Unavailable and the grading says the same thing. This model's §2 row 8 grades it exactly so and is NOT widened to agree with a sibling (R13; plan decision D5, REV 2)", got)
	}
	// D16: the tone DOMAIN is E3's numeric range, not a list. An empty
	// CTCSSTones with no range declared fails CLOSED on every Known tone
	// (core/codeplug/fieldstate.go's ToneField.Valid, via
	// spec.Capabilities.AdmitsTone), and every read produces Known tones.
	if len(caps.CTCSSTones) != 0 {
		t.Errorf("CTCSSTones has %d entries, want 0 — matrix §1 #8 grades it empty in terms (\"No list of permitted tone frequencies is printed anywhere in this document\"), and copying the IC-7300's fifty-tone list would be the cross-model borrowing both §4s forbid. The domain is the numeric range instead; E3 forbids declaring both", len(caps.CTCSSTones))
	}
	if caps.CTCSSToneRange == nil {
		t.Fatal("CTCSSToneRange is nil — with neither a list nor a range, spec.Capabilities.AdmitsTone fails closed and every tone this driver reads back would be refused by codeplug.ToneField.Valid (D16, T1(2))")
	}
	if want := (spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1}); *caps.CTCSSToneRange != want {
		t.Errorf("CTCSSToneRange = %+v, want %+v — from THIS model's own evidence: PDF p.23's per-digit legend, 100 Hz digit 0 ~ 2 and 10/1/0.1 Hz digits 0 ~ 9, intersected with the capability floor of 1 because 0 Hz is not a tone (T1(2)). Cite ic7300mk2-tone-tx-encoding beside it: the ⑫ ~ ⑭ heading prints no encoding at all (§3.16 A6)", *caps.CTCSSToneRange, want)
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
		"ClarMaxHz":              "no clarifier/RIT field in the 1A 00 record (matrix §1 #6, poor fit, graded)",
		"ClarStepHz":             "as ClarMaxHz (matrix §1 #7)",
		"ShiftOptions":           "no shift or duplex field exists on this model (matrix §1 #14)",
		"CTCSSStates":            "displaced by ToneModes on Icom models (spec D4; matrix §1 #15)",
		"DuplexOptions":          "MANUAL-EVIDENCED absence (matrix §1b, duplex)",
		"DTCSCodes":              "the record carries no DTCS field at all; MANUAL-EVIDENCED absence (matrix §1b, dtcs_code)",
		"TuningSteps":            "additions design D8 — this record carries no receiver tuning-step field",
		"ProgramTuningStepRange": "additions design D8 — this record carries no programmable tuning-step field",
		"AttenuatorDB":           "additions design D8 — this record carries no attenuator field",
		"PreampOptions":          "additions design D8 — this record carries no preamp field",
		"AntennaOptions":         "additions design D8 — this record carries no antenna field",
		"DTCSPolarities":         "the record carries no DTCS field at all; MANUAL-EVIDENCED absence (matrix §1b, dtcs_polarity)",
		"RequiredSlots":          "NoBlank is the whole-bank form and the SCAN bank is exactly P1 and P2, so saying it twice would create two places to keep in step (plan decision D8)",
		"CTCSSTones":             "the tone domain is CTCSSToneRange {1, 2999, 1} deciHz, not a list — matrix §1 #8 grades this model's list empty in terms, so no deviation arises here (D16)",
		"MinFreqHz":              "no tuning floor is printed in this document; a zero DISABLES the lower-bound check rather than asserting a 0 Hz floor (core/spec/capabilities.go) — entry ic7300mk2-min-frequency, lift MK2-R15",
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
	if ty.NumField() != 28 {
		t.Errorf("spec.Capabilities has %d fields, want 28 — every one of them is written down explicitly in baseCapabilities, and the count is stated in caps.go and doc.go; if the struct has genuinely changed, set the new value HERE and account for the new field in the literal and in deliberatelyZero", ty.NumField())
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
	}
}

// A test the IC-7300 does not have, because the IC-7300's document does not
// say this and this one does.
func TestScanBankIsNoBlank(t *testing.T) {
	caps := New(RealHardware).Capabilities()
	for _, b := range caps.Banks {
		switch b.ID {
		case spec.BankScan:
			if !b.NoBlank {
				t.Error("SCAN.NoBlank = false — P1 and P2 cannot be cleared (PDF p.4, the 0B row: \"ⓘ P1 and P2 cannot be cleared.\"; PDF p.17: \"* Except for \\\"01 00\\\" and \\\"01 01\\\" (P1/P2).\")")
			}
		case spec.BankMemory:
			if b.NoBlank {
				t.Error("MEM.NoBlank = true — nothing in this document says a memory channel must stay populated")
			}
		}
	}
	if len(caps.RequiredSlots) != 0 {
		t.Errorf("RequiredSlots = %v, want empty — NoBlank is the whole-bank form and the SCAN bank is exactly those two slots, so saying it twice creates two places to keep in step (plan decision D8)", caps.RequiredSlots)
	}
}

// The tag charset is TAKEN FROM the profile rather than restated, so the
// driver's advertised charset and the codec's cannot drift.
func TestTagCharsetComesFromTheProfile(t *testing.T) {
	caps := New(RealHardware).Capabilities()
	if len(caps.TagCharset) != 95 {
		t.Errorf("TagCharset is %d bytes, want 95 (space + 10 digits + 26 upper + 26 lower + 32 symbols)", len(caps.TagCharset))
	}
	// 0x60 is a LEGAL NAME BYTE whose GLYPH this document does not establish
	// (§3.16 A2: the p.18 Symbols table draws the same glyph against 27 and
	// 60). It is admitted as a byte and never resolved to 0x27.
	if !caps.TagByteOK(0x60) {
		t.Error("TagByteOK(0x60) = false — NameCharset is a BYTE SET, not a glyph map, and both 27 and 60 are legal name bytes (D13)")
	}
	for i := 0; i < len(caps.TagCharset); i++ {
		if !caps.TagByteOK(caps.TagCharset[i]) {
			t.Errorf("TagCharset byte %#02x is not accepted by TagByteOK", caps.TagCharset[i])
		}
	}
}
