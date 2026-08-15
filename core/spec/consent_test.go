// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"reflect"
	"testing"
)

// consentFixture builds a Capabilities exercising every Support state on
// both sides of a field, across two banks, with every top-level slice
// populated — so the transform tests can assert both what changes (only
// write-side Unverified) and what survives a round trip untouched.
func consentFixture() Capabilities {
	tones := StandardCTCSSTones()
	return Capabilities{
		Model:         "TEST-CONSENT",
		CATID:         "0000",
		Modes:         []string{"LSB", "USB"},
		TagLen:        12,
		ClarMaxHz:     9990,
		ClarStepHz:    10,
		CTCSSTones:    tones[:],
		Bauds:         []int{9600, 38400},
		DefaultBaud:   38400,
		MinFreqHz:     30000,
		MaxFreqHz:     56000000,
		RequiredSlots: []string{"001"},
		ShiftOptions:  StandardShiftOptions(),
		CTCSSStates:   StandardCTCSSStates(),
		Banks: []Bank{
			{
				ID:    BankMemory,
				Label: "Memories",
				Slots: []string{"001", "002"},
				Fields: map[Field]FieldSupport{
					// The one field the transform may touch.
					FieldFrequency: {Read: Supported, Write: Unverified},
					// Read-side Unverified: never transformed.
					FieldMode: {Read: Unverified, Write: Supported},
					// Neither side transformable.
					FieldClarifier:  {Read: Supported, Write: Inert},
					FieldTagDisplay: {Read: Supported, Write: Unsupported},
					// The zero FieldSupport passes through as-is.
					FieldScanSkip: {},
					// Both sides Unverified: only the write side moves.
					FieldTag: {Read: Unverified, Write: Unverified},
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

// TestConsentUnverifiedWrites_WriteSideOnly is the transform's core
// contract: write-side Unverified becomes ConsentedUnverified in every
// bank, and NOTHING else moves — not Supported, Unsupported or Inert
// write labels, not the zero FieldSupport, and above all not a single
// Read label. Read labels are excluded by design: reads already flow, so
// an Unverified read is a true statement that consent does not alter (and
// Capabilities.Validate rejects a read-side ConsentedUnverified outright).
func TestConsentUnverifiedWrites_WriteSideOnly(t *testing.T) {
	in := consentFixture()
	out := ConsentUnverifiedWrites(in)

	cases := []struct {
		bank BankID
		f    Field
		want FieldSupport
	}{
		{BankMemory, FieldFrequency, FieldSupport{Read: Supported, Write: ConsentedUnverified}},
		{BankMemory, FieldMode, FieldSupport{Read: Unverified, Write: Supported}},
		{BankMemory, FieldClarifier, FieldSupport{Read: Supported, Write: Inert}},
		{BankMemory, FieldTagDisplay, FieldSupport{Read: Supported, Write: Unsupported}},
		{BankMemory, FieldScanSkip, FieldSupport{}},
		{BankMemory, FieldTag, FieldSupport{Read: Unverified, Write: ConsentedUnverified}},
		{BankPMS, FieldFrequency, FieldSupport{Read: Supported, Write: ConsentedUnverified}},
	}
	for _, tc := range cases {
		if got := out.FieldSupport(tc.bank, tc.f); got != tc.want {
			t.Errorf("FieldSupport(%v, %v) = %+v, want %+v", tc.bank, tc.f, got, tc.want)
		}
	}

	// Everything outside Banks is carried across unchanged in value.
	if out.Model != in.Model || out.CATID != in.CATID || out.TagLen != in.TagLen ||
		out.ClarMaxHz != in.ClarMaxHz || out.ClarStepHz != in.ClarStepHz ||
		out.DefaultBaud != in.DefaultBaud || out.MinFreqHz != in.MinFreqHz ||
		out.MaxFreqHz != in.MaxFreqHz {
		t.Errorf("scalar fields changed: got %+v, want the input's values", out)
	}
	if !reflect.DeepEqual(out.Modes, in.Modes) ||
		!reflect.DeepEqual(out.CTCSSTones, in.CTCSSTones) ||
		!reflect.DeepEqual(out.Bauds, in.Bauds) ||
		!reflect.DeepEqual(out.RequiredSlots, in.RequiredSlots) ||
		!reflect.DeepEqual(out.ShiftOptions, in.ShiftOptions) ||
		!reflect.DeepEqual(out.CTCSSStates, in.CTCSSStates) {
		t.Error("a non-Banks slice field's contents changed, want carried across unchanged")
	}
	// Bank metadata other than Fields survives too.
	pms, ok := out.Bank(BankPMS)
	if !ok {
		t.Fatal("Bank(BankPMS) ok = false after consent, want true")
	}
	if pms.Label != "Scan limits (PMS)" || !pms.NoBlank || len(pms.Slots) != 2 {
		t.Errorf("PMS bank metadata = %+v, want label/NoBlank/Slots unchanged", pms)
	}
}

// TestConsentUnverifiedWrites_InputUntouched pins the transform as a pure
// function over a DEEP copy. The caller's Capabilities is the driver's
// baseline — shared, long-lived, and the data every write gate consults —
// so the transform must neither rewrite it in place nor hand back a value
// that aliases it. Aliasing here would let a consented session's caps
// silently redefine what an unconsented one enforces.
func TestConsentUnverifiedWrites_InputUntouched(t *testing.T) {
	in := consentFixture()
	before := consentFixture() // an independent copy of the same shape

	out := ConsentUnverifiedWrites(in)

	if !reflect.DeepEqual(in, before) {
		t.Error("input Capabilities was modified by ConsentUnverifiedWrites, want untouched")
	}

	// Mutating the result must not reach the input, in either the Fields
	// maps or the Slots slices.
	out.Banks[0].Fields[FieldFrequency] = FieldSupport{Read: Supported, Write: Supported}
	out.Banks[0].Fields[FieldErase] = FieldSupport{Read: Supported, Write: Supported}
	out.Banks[0].Slots[0] = "TAMPERED"
	out.Banks = append(out.Banks, Bank{ID: BankEMG})
	if !reflect.DeepEqual(in, before) {
		t.Error("mutating the transformed Capabilities reached the input, want independent storage")
	}

	// Every top-level slice must be freshly allocated, enumerated from the
	// Capabilities struct itself so a slice field added later cannot be
	// silently left aliasing the input.
	inV, outV := reflect.ValueOf(in), reflect.ValueOf(ConsentUnverifiedWrites(in))
	for i := 0; i < inV.NumField(); i++ {
		if inV.Type().Field(i).Type.Kind() != reflect.Slice {
			continue
		}
		name := inV.Type().Field(i).Name
		inF, outF := inV.Field(i), outV.Field(i)
		if inF.Len() == 0 {
			continue
		}
		if inF.Pointer() == outF.Pointer() {
			t.Errorf("%s shares its backing array with the input, want a fresh slice", name)
		}
	}
}

// TestConsentUnverifiedWrites_Idempotent checks that applying the
// transform twice equals applying it once: ConsentedUnverified is not
// itself Unverified, so a second pass has nothing left to convert. This
// matters because the wiring may compose capability assembly steps, and a
// double application must never be a different outcome from a single one.
func TestConsentUnverifiedWrites_Idempotent(t *testing.T) {
	once := ConsentUnverifiedWrites(consentFixture())
	twice := ConsentUnverifiedWrites(once)
	if !reflect.DeepEqual(once, twice) {
		t.Errorf("applying the transform twice differs from once:\nonce  = %+v\ntwice = %+v", once, twice)
	}
}

// TestConsentUnverifiedWrites_NoUnverifiedNoOp pins the FT-710-shaped
// case: a capability set with no write-side Unverified anywhere comes back
// deep-equal to its input. The FT-710's RealHardware profile is exactly
// this shape (write labels Supported or Inert), so consent on that radio
// is provably a no-op rather than a promise about the transform's
// gentleness.
func TestConsentUnverifiedWrites_NoUnverifiedNoOp(t *testing.T) {
	in := Capabilities{
		Model: "FT-710",
		Modes: []string{"LSB", "USB"},
		Banks: []Bank{{
			ID:    BankMemory,
			Label: "Memories",
			Slots: []string{"001"},
			Fields: map[Field]FieldSupport{
				FieldFrequency: {Read: Supported, Write: Supported},
				FieldClarifier: {Read: Supported, Write: Inert},
				FieldErase:     {Read: Unsupported, Write: Unsupported},
				FieldTag:       {Read: Unverified, Write: Unsupported},
			},
		}},
	}
	out := ConsentUnverifiedWrites(in)
	if !reflect.DeepEqual(in, out) {
		t.Errorf("transform of a no-write-side-Unverified capability set changed it:\nin  = %+v\nout = %+v", in, out)
	}
}

// TestConsentUnverifiedWrites_OutputStillValidates ties this transform to
// the validation Task 1 added: Capabilities.Validate accepts
// ConsentedUnverified write-side but REJECTS it read-side, so a valid
// capability set must stay valid after consent. This is the cheapest
// standing guard that the transform never strays onto the read half.
func TestConsentUnverifiedWrites_OutputStillValidates(t *testing.T) {
	in := consentFixture()
	if err := in.Validate(); err != nil {
		t.Fatalf("fixture Validate() = %v, want nil (the premise of this test)", err)
	}
	if err := ConsentUnverifiedWrites(in).Validate(); err != nil {
		t.Errorf("Validate() after consent = %v, want nil", err)
	}
}

// TestConsentUnverifiedWrites_FieldEraseNeverTouched pins spec
// amendment 4: consent can never mint a writable erase, whatever a
// profile's FieldErase label says. The concrete hazard is real, not
// hypothetical: the FT-710 fail-safe profile leaves MEM FieldErase
// Write: Unverified (caps.go, CapabilitiesUnverified — the label was
// simply never tightened because Unverified and Unsupported are equally
// unwritable), and a transformed erase label would unblock
// codeplug.Diff's erase gate and reach clone/execute's documented-
// unreachable DiffErased branch.
func TestConsentUnverifiedWrites_FieldEraseNeverTouched(t *testing.T) {
	in := Capabilities{Banks: []Bank{{ID: BankMemory, Fields: map[Field]FieldSupport{
		FieldErase:     {Read: Unsupported, Write: Unverified},
		FieldFrequency: {Read: Supported, Write: Unverified},
	}}}}
	out := ConsentUnverifiedWrites(in)
	if got := out.FieldSupport(BankMemory, FieldErase); got.Write != Unverified {
		t.Fatalf("FieldErase.Write = %v after consent, want Unverified (erase is never consentable)", got.Write)
	}
	if got := out.FieldSupport(BankMemory, FieldFrequency); got.Write != ConsentedUnverified {
		t.Fatalf("FieldFrequency.Write = %v, want ConsentedUnverified", got.Write)
	}
}
