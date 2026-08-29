// SPDX-License-Identifier: GPL-3.0-or-later

// This file is the package's ONE INTERNAL test file (package ic9700 rather
// than ic9700_test), and it is internal for exactly one reason: it pins
// writeTrialsComplete, an unexported CONST whose whole purpose is to be
// false, and a const cannot be reached through an export_test.go alias
// without becoming a variable. Everything else in this package's suite is
// external and goes through the public surface or through export_test.go's
// two function aliases, which is what keeps the tests honest about what a
// caller can actually reach.
package ic9700

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

var receiverCapabilitiesDeliberatelyZero = map[string]string{
	"TuningSteps":            "additions design D8 — the IC-9700 record carries no receiver tuning-step field",
	"ProgramTuningStepRange": "additions design D8 — the IC-9700 record carries no programmable tuning-step field",
	"AttenuatorDB":           "additions design D8 — the IC-9700 record carries no attenuator field",
	"PreampOptions":          "additions design D8 — the IC-9700 record carries no preamp field",
	"AntennaOptions":         "additions design D8 — the IC-9700 record carries no antenna field",
}

func TestDeliberatelyZeroAudit_ReceiverCapabilities(t *testing.T) {
	if got := reflect.TypeOf(spec.Capabilities{}).NumField(); got != 28 {
		t.Fatalf("spec.Capabilities has %d fields, this audit knows 28", got)
	}
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		value := reflect.ValueOf(caps)
		for name, reason := range receiverCapabilitiesDeliberatelyZero {
			if field := value.FieldByName(name); !field.IsValid() {
				t.Errorf("Capabilities.%s no longer exists (%s)", name, reason)
			} else if !field.IsZero() {
				t.Errorf("Capabilities.%s is non-zero but deliberatelyZero says %q", name, reason)
			}
		}
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true; NO IC-9700 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT")
	}
}

func TestCapabilitiesValidate(t *testing.T) {
	for name, caps := range map[string]spec.Capabilities{
		"unverified": CapabilitiesUnverified(),
		"simulated":  CapabilitiesSimulated(),
	} {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s: Validate: %v", name, err)
		}
	}
}

func TestUnverifiedCapabilitiesWriteNothing(t *testing.T) {
	caps := CapabilitiesUnverified()
	for _, bank := range caps.Banks {
		for f, fs := range bank.Fields {
			if fs.Write == spec.Supported || fs.Write == spec.ConsentedUnverified {
				t.Errorf("%s/%s: Write = %v on the unverified profile", bank.ID, f, fs.Write)
			}
		}
	}
}

func TestEraseIsZeroOnEveryBank(t *testing.T) {
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		for _, bank := range caps.Banks {
			if fs := caps.FieldSupport(bank.ID, spec.FieldErase); !fs.Unreachable() {
				t.Errorf("%s: erase = %+v, want zero on both sides (spec D4, adjudication 19)", bank.ID, fs)
			}
		}
	}
}

func TestScanSkipIsUnsupportedBecauseFieldFourIsSelectGroupMembership(t *testing.T) {
	// matrix erratum 1 / ADDED entry A9, and the tier rule: scan_skip on
	// Icom is SELECT group membership and is never mapped as skip. Field
	// ④ prints 0=OFF, 1=★1, 2=★2, 3=★3 — four values, and the neutral
	// field is a boolean. Under R6/E6 a star-group channel is REFUSED on
	// write (write.go), never preserved by cache and never flattened.
	// See OQ-4.
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		for _, bank := range caps.Banks {
			if fs := caps.FieldSupport(bank.ID, spec.FieldScanSkip); !fs.Unreachable() {
				t.Errorf("%s: scan_skip = %+v, want zero on both sides", bank.ID, fs)
			}
		}
	}
}

func TestBanksAreDenseAndCarryNoSparseSpace(t *testing.T) {
	// D6 gives this model (band, channel) addressing, not a sparse group
	// space; Sparse/Groups/PerGroup/Budget are legal only together and
	// must all be zero here (matrix §1b).
	for _, bank := range CapabilitiesUnverified().Banks {
		if bank.Sparse || bank.Groups != 0 || bank.PerGroup != 0 || bank.Budget != 0 {
			t.Errorf("%s: sparse space set on a band-addressed model: %+v", bank.ID, bank)
		}
	}
}

func TestBankSlotCounts(t *testing.T) {
	want := map[spec.BankID]int{spec.BankMemory: 297, spec.BankScan: 18, spec.BankCall: 6}
	caps := CapabilitiesUnverified()
	if len(caps.Banks) != len(want) {
		t.Fatalf("%d banks, want %d", len(caps.Banks), len(want))
	}
	for _, b := range caps.Banks {
		if got := len(b.Slots); got != want[b.ID] {
			t.Errorf("%s: %d slots, want %d", b.ID, got, want[b.ID])
		}
	}
	for _, id := range []spec.BankID{spec.BankScan, spec.BankCall} {
		b, _ := caps.Bank(id)
		if !b.NoBlank {
			t.Errorf("%s: NoBlank is false; the printed clear form admits only 0001~0099", id)
		}
	}
}

func TestLegitimatelyEmptyVocabulariesAreAdmitted(t *testing.T) {
	// E5b: an Icom bank carries no Yaesu shift/CTCSS-state vocabulary,
	// and fail-closed is preserved through those fields' Unsupported
	// write grades rather than through a non-empty list.
	caps := CapabilitiesUnverified()
	if len(caps.ShiftOptions) != 0 || len(caps.CTCSSStates) != 0 {
		t.Fatal("this model must declare neither vocabulary")
	}
	if err := caps.Validate(); err != nil {
		t.Fatalf("Validate refused the empty-vocabulary shape: %v", err)
	}
	for _, bank := range caps.Banks {
		for _, f := range []spec.Field{spec.FieldShift, spec.FieldCTCSSState, spec.FieldCTCSSTone} {
			if fs := caps.FieldSupport(bank.ID, f); !fs.Unreachable() {
				t.Errorf("%s/%s: %+v, want zero on both sides", bank.ID, f, fs)
			}
		}
	}
}

func TestDuplexAndToneVocabulariesHaveOneCanonicalEntryEach(t *testing.T) {
	// E5: where two entries share a DuplexDirection or ToneModeSemantics
	// exactly one must be canonical. This model has three duplex entries
	// with three distinct directions and four tone modes with four
	// distinct semantics, so the rule is satisfied without multiplicity —
	// asserted so a later vocabulary addition cannot break it silently.
	caps := CapabilitiesUnverified()

	seenDir := map[spec.DuplexDirection]int{}
	for _, d := range caps.DuplexOptions {
		seenDir[d.Direction]++
	}
	if len(caps.DuplexOptions) != 3 {
		t.Errorf("%d duplex entries, want 3 (OFF, DUP-, DUP+; RPS has no direction — OQ-6)", len(caps.DuplexOptions))
	}
	for dir, n := range seenDir {
		if n != 1 {
			t.Errorf("direction %v has %d entries; E5 requires exactly one canonical", dir, n)
		}
	}

	seenSem := map[spec.ToneModeSemantics]int{}
	for _, m := range caps.ToneModes {
		seenSem[m.Semantics]++
	}
	if len(caps.ToneModes) != 4 {
		t.Errorf("%d tone modes, want 4 (OFF, TONE, TSQL, DTCS)", len(caps.ToneModes))
	}
	for sem, n := range seenSem {
		if n != 1 {
			t.Errorf("semantics %v has %d entries; E5 requires exactly one canonical", sem, n)
		}
	}
}

func TestToneDomainIsARangeAndNotAChart(t *testing.T) {
	// OQ-5 / enabler E3 / tier ruling T1(2). The wire encodes 0.0–299.9 Hz
	// in tenths; the CAPABILITY floor is 1 deciHz because zero is not a
	// tone and the landed ToneRange refuses MinDeciHz <= 0. The one
	// encodable value the capability excludes is therefore ZERO, and T1(3)
	// is what happens to it on a read (read.go maps it Unknown).
	caps := CapabilitiesUnverified()
	if caps.CTCSSToneRange == nil {
		t.Fatal("no CTCSSToneRange; this radio's tone field is a NUMBER, not an index into a chart")
	}
	if len(caps.CTCSSTones) != 0 {
		t.Error("CTCSSTones is non-empty; Validate refuses a profile declaring both shapes, and the Yaesu 50-tone chart is not evidence about an Icom")
	}
	if got := *caps.CTCSSToneRange; got.MinDeciHz != 1 || got.MaxDeciHz != 2999 || got.StepDeciHz != 1 {
		t.Errorf("CTCSSToneRange = %+v, want {1, 2999, 1}", got)
	}
	for _, tc := range []struct {
		tone spec.Tone
		want bool
	}{{0, false}, {1, true}, {885, true}, {2999, true}, {3000, false}} {
		if got := caps.AdmitsTone(tc.tone); got != tc.want {
			t.Errorf("AdmitsTone(%v) = %v, want %v", tc.tone, got, tc.want)
		}
	}
}

func TestDTCSCodesAreTheFiveHundredAndTwelveOctalDigitCodes(t *testing.T) {
	// PDF p.21 "DTCS code and polarity setting": First/Second/Third digit
	// 0 ~ 7. That is every value 0..777 whose every decimal digit is <= 7
	// — 8x8x8 = 512 of them — and it is a TABLE rather than a range
	// because the admissible values are not contiguous (E3 says so
	// explicitly).
	caps := CapabilitiesUnverified()
	if got := len(caps.DTCSCodes); got != 512 {
		t.Fatalf("%d DTCS codes, want 512", got)
	}
	if caps.DTCSCodes[0] != 0 || caps.DTCSCodes[511] != 777 {
		t.Errorf("DTCS codes run %d..%d, want 0..777", caps.DTCSCodes[0], caps.DTCSCodes[511])
	}
	for i, c := range caps.DTCSCodes {
		if i > 0 && caps.DTCSCodes[i-1] >= c {
			t.Fatalf("DTCSCodes is not strictly ascending at index %d", i)
		}
		for n := c; n > 0; n /= 10 {
			if n%10 > 7 {
				t.Fatalf("DTCSCodes carries %d, whose digits are not all 0..7", c)
			}
		}
	}
}

func TestTagCharsetAdmitsTheSemicolonThisRadioAccepts(t *testing.T) {
	// TagCharset is supplied rather than defaulted because core/spec's
	// default rule excludes ';' — it terminates a NEWCAT frame. CI-V is
	// binary and has no such terminator, and this radio's own charset
	// table prints the character, so the default would refuse a name the
	// radio accepts.
	caps := CapabilitiesUnverified()
	if got := len(caps.TagCharset); got != 95 {
		t.Errorf("TagCharset has %d bytes, want the 95 printable ASCII 0x20-0x7E", got)
	}
	for b := 0x20; b <= 0x7E; b++ {
		if !caps.TagByteOK(byte(b)) {
			t.Errorf("TagByteOK(%#02x) is false", b)
		}
	}
	for _, b := range []byte{0x00, 0x1F, 0x7F, 0xFD, 0xFE} {
		if caps.TagByteOK(b) {
			t.Errorf("TagByteOK(%#02x) is true; only printable ASCII is in this radio's table", b)
		}
	}
}

func TestClarifierIsNotAFieldThisRecordHasAtAll(t *testing.T) {
	// matrix §1 #6/#7: the record carries no clarifier, so the two
	// clarifier capability numbers are ZERO and the field is unreachable
	// on every bank. A poor fit stated as zero rather than left to a
	// default someone would later read as a real offset.
	caps := CapabilitiesUnverified()
	if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 {
		t.Errorf("ClarMaxHz/ClarStepHz = %d/%d, want 0/0", caps.ClarMaxHz, caps.ClarStepHz)
	}
	for _, bank := range caps.Banks {
		for _, f := range []spec.Field{spec.FieldClarifier, spec.FieldTagDisplay} {
			if fs := caps.FieldSupport(bank.ID, f); !fs.Unreachable() {
				t.Errorf("%s/%s: %+v, want zero on both sides", bank.ID, f, fs)
			}
		}
	}
}

func TestTheThirteenFieldsThisRecordDoesCarryAreReadableAndUnverifiedToWrite(t *testing.T) {
	// The positive half of the grid: every field the 111-byte record maps
	// onto the neutral model is READ Unverified and WRITE Unverified on
	// every bank — documented, never proven, and therefore not writable
	// until a user consents.
	caps := CapabilitiesUnverified()
	for _, bank := range caps.Banks {
		for _, f := range []spec.Field{
			spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
			spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
			spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
			spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter,
			spec.FieldDataMode,
		} {
			fs := caps.FieldSupport(bank.ID, f)
			if fs.Read != spec.Unverified || fs.Write != spec.Unverified {
				t.Errorf("%s/%s: %+v, want Read and Write both Unverified", bank.ID, f, fs)
			}
			if fs.CanWrite() {
				t.Errorf("%s/%s: CanWrite is true without consent", bank.ID, f)
			}
		}
	}
}
