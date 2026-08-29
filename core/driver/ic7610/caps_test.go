// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// mappedFields is the seven spec.Fields the 1A 00 record actually MAPS —
// the same seven core/civ/ic7610's layout() carries spans for, in that
// layout's own order. Every other spec.Field is a written-down zero, and
// TestBaseline_WrittenDownZeroes names each one.
var mappedFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldFilter,
	spec.FieldToneMode,
	spec.FieldToneTx,
	spec.FieldToneRx,
	spec.FieldTag,
}

// bothBanks is every BankID this model declares, so a per-cell assertion
// can be made on each without either being forgotten.
var bothBanks = []spec.BankID{spec.BankMemory, spec.BankScan}

func bothProfiles() map[string]spec.Capabilities {
	return map[string]spec.Capabilities{
		"Unverified": capabilitiesUnverified(),
		"Simulated":  capabilitiesSimulated(),
	}
}

// TestWriteTrialsComplete_PinnedFalse is this model's own pin, on this
// model's own evidence and its own registry row.
//
// FALSE because no IC-7610 has ever been asked anything by this project
// (matrix S0, S3.14): no write trial has occurred, so no write result has
// been observed, so the flag cannot be true. Flipping it is a TWO-PART
// change - this constant AND a CapabilitiesRealHardware profile built
// field class by field class from an IC-7610's OWN trial evidence, AND the
// Capabilities switch rewritten to select it - with this test's
// expectation rewritten so the flip is a visible, reviewable test change.
//
// THE IC-7610 HAS NO REGISTERED SIBLING (matrix S4), so there is one
// constant and one row, not a pair.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true - no IC-7610 has ever answered a frame; see doc.go")
	}
}

// TestBaseline_Validate is the test enabler E5b unblocks.
//
// This model's banks carry NEITHER a shift vocabulary NOR a duplex one:
// the 1A 00 record has no repeater field at all, so ShiftOptions and
// DuplexOptions are both empty and FieldShift and FieldDuplex both carry
// the zero FieldSupport. Before Wave 2.5, spec.Validate demanded
// ShiftOptions unconditionally and would have refused this shape; E5b
// makes the requirement conditional on a bank actually REACHING
// FieldShift or FieldDuplex (core/spec/validate.go, anyBankReaches), and
// the same change was made for the CTCSSStates/ToneModes pair. This model
// satisfies the tone half by declaring ToneModes and the shift half by
// reaching neither field.
func TestBaseline_Validate(t *testing.T) {
	for name, caps := range bothProfiles() {
		t.Run(name, func(t *testing.T) {
			if err := caps.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
	t.Run("Consented", func(t *testing.T) {
		if err := spec.ConsentUnverifiedWrites(capabilitiesUnverified()).Validate(); err != nil {
			t.Fatalf("Validate after consent: %v", err)
		}
	})
}

// TestBaseline_Shape pins every capability value this model declares
// against the matrix row it came from. A shape test is the only place a
// transcription slip shows up as a failure rather than as a wrong byte on
// a wire nobody watched.
func TestBaseline_Shape(t *testing.T) {
	caps := capabilitiesUnverified()

	// S1 row 1 - the model name, and the registry key it must equal.
	if caps.Model != "IC-7610" {
		t.Errorf("Model = %q, want %q", caps.Model, "IC-7610")
	}
	// S3.4 / spec D3.2 - the STATIC, PRE-PROBE value is the CI-V address
	// ALONE. The 19 00 token is unknown until a session opens, and this
	// driver records it without ever matching it (D5 entry 7, lift R7),
	// so it cannot appear in a value the composition root reads before any
	// port exists.
	if caps.CATID != "98" {
		t.Errorf("CATID = %q, want %q - the address alone; the 19 00 token is a per-session observation", caps.CATID, "98")
	}
	// S1 row 7 - the name span (18)~(27) is ten bytes wide, so a tag is
	// ten characters. Pinned against the codec's own NameLength so the
	// number cannot drift between the two packages.
	if caps.TagLen != 10 {
		t.Errorf("TagLen = %d, want 10", caps.TagLen)
	}
	if caps.TagLen != civic7610.Profile().NameLength() {
		t.Errorf("TagLen %d and the codec's NameLength %d disagree", caps.TagLen, civic7610.Profile().NameLength())
	}
	if caps.TagCharset != civic7610.NameCharset {
		t.Error("TagCharset is not the codec's own NameCharset - one transcription, one source")
	}

	// Two banks, and the wire form of every slot in each.
	if len(caps.Banks) != 2 {
		t.Fatalf("declared %d banks, want 2 (MEM and SCAN)", len(caps.Banks))
	}
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	if len(mem.Slots) != 99 {
		t.Errorf("MEM has %d slots, want 99", len(mem.Slots))
	} else if mem.Slots[0] != "001" || mem.Slots[98] != "099" {
		t.Errorf("MEM slots run %q..%q, want \"001\"..\"099\"", mem.Slots[0], mem.Slots[98])
	}
	scan, ok := caps.Bank(spec.BankScan)
	if !ok {
		t.Fatal("no SCAN bank")
	}
	if got := scan.Slots; len(got) != 2 || got[0] != "P1" || got[1] != "P2" {
		t.Errorf("SCAN slots = %v, want [P1 P2]", got)
	}
	for _, b := range caps.Banks {
		if b.NoBlank {
			t.Errorf("bank %s has NoBlank true - nothing in this document says an IC-7610 memory or scan edge must stay populated", b.ID)
		}
		if b.Sparse || b.Groups != 0 || b.PerGroup != 0 || b.Budget != 0 {
			t.Errorf("bank %s declares a sparse space - this model's channel selector is a flat two-byte number, not a group/channel pair (matrix S1b)", b.ID)
		}
	}
	if len(caps.RequiredSlots) != 0 {
		t.Errorf("RequiredSlots = %v, want empty - no IC-7610 slot is documented as un-erasable", caps.RequiredSlots)
	}

	// S1 row 9 - the six rates printed for the CI-V link.
	wantBauds := []int{4800, 9600, 19200, 38400, 57600, 115200}
	if !reflect.DeepEqual(caps.Bauds, wantBauds) {
		t.Errorf("Bauds = %v, want %v", caps.Bauds, wantBauds)
	}
	// OQ2 - ASSUMED, register home ic7610-default-baud, matrix lift R11.
	// The document names six rates and marks NO default; the choice within
	// that set is arbitrary and recorded as such. What makes it safe is
	// the grading plus the failure mode: a wrong guess costs a clean
	// timeout at Open, never a wrong byte.
	if caps.DefaultBaud != 19200 {
		t.Errorf("DefaultBaud = %d, want 19200 (ASSUMED, lift R11)", caps.DefaultBaud)
	}
	found := false
	for _, b := range caps.Bauds {
		if b == caps.DefaultBaud {
			found = true
		}
	}
	if !found {
		t.Error("DefaultBaud is not a member of Bauds")
	}

	// S1 row 12 - the record's 10 MHz digit is labelled 0-6 and the top
	// byte is a fixed "0 : 0", so 69 999 999 Hz is the largest ENCODABLE
	// value. The STORABLE ceiling is not established (matrix lift R17a);
	// see deliberatelyZero's MaxFreqHz note in caps.go.
	if caps.MinFreqHz != 0 {
		t.Errorf("MinFreqHz = %d, want 0", caps.MinFreqHz)
	}
	if caps.MaxFreqHz != MaxEncodableFreqHz {
		t.Errorf("MaxFreqHz = %d, want %d", caps.MaxFreqHz, uint64(MaxEncodableFreqHz))
	}

	// S1 row 4 - the ten printed mode codes, in the printed order.
	wantModes := []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "PSK", "PSK-R"}
	if !reflect.DeepEqual(caps.Modes, wantModes) {
		t.Errorf("Modes = %v, want %v", caps.Modes, wantModes)
	}
	// S1 row 5 - the three printed filter values.
	if !reflect.DeepEqual(caps.Filters, []string{"FIL1", "FIL2", "FIL3"}) {
		t.Errorf("Filters = %v, want [FIL1 FIL2 FIL3]", caps.Filters)
	}
	// S1 row 8 / erratum 5 - the three values of (11)'s low nibble, with
	// their semantics.
	wantToneModes := []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
	}
	if !reflect.DeepEqual(caps.ToneModes, wantToneModes) {
		t.Errorf("ToneModes = %+v, want %+v", caps.ToneModes, wantToneModes)
	}

	// The Yaesu-family vocabularies, and the Icom ones this radio does not
	// express. Each is a positive statement, not an omission.
	if len(caps.ShiftOptions) != 0 {
		t.Error("ShiftOptions is non-empty - the 1A 00 record has no repeater shift field")
	}
	if len(caps.CTCSSStates) != 0 {
		t.Error("CTCSSStates is non-empty - this radio expresses tone through ToneModes, the Icom vocabulary")
	}
	if len(caps.DuplexOptions) != 0 {
		t.Error("DuplexOptions is non-empty - the 1A 00 record has no duplex field")
	}
	if len(caps.DTCSCodes) != 0 || len(caps.DTCSPolarities) != 0 {
		t.Error("a DTCS vocabulary is declared - DTCS is printed nowhere in this document's 17 pages")
	}
	if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 {
		t.Errorf("clarifier bounds are %d/%d, want 0/0 - the record has no clarifier field", caps.ClarMaxHz, caps.ClarStepHz)
	}
}

// codecEnum returns the enum map core/civ/ic7610's layout carries for f.
//
// It reaches through civ.Profile.Layouts rather than into the codec
// package's unexported tables, which is the only route a consumer has and
// therefore the right one for a driver to be pinned by.
func codecEnum(t *testing.T, f civ.FieldID) map[byte]string {
	t.Helper()
	layouts := civic7610.Profile().Layouts()
	if len(layouts) != 1 {
		t.Fatalf("the codec declares %d layouts, want 1 (DiscriminatorSingleLength)", len(layouts))
	}
	for _, span := range layouts[0].Fields {
		if span.Field == f {
			if len(span.Enum) == 0 {
				t.Fatalf("the codec's %v span carries no enum", f)
			}
			return span.Enum
		}
	}
	t.Fatalf("the codec's layout has no %v span", f)
	return nil
}

// TestModes_MatchTheCodec pins the driver's advertised mode vocabulary
// against the codec's own decoder table, value for value.
//
// caps.go writes the ten names out because core/civ/ic7610's modeEnum is
// unexported and that package is frozen evidence-bound code Stage 2 may
// not extend. THIS TEST IS WHAT MAKES THAT DUPLICATION SAFE: a name the
// driver offers that the codec cannot decode would be a mode the UI
// invites a user to pick and the encoder then refuses, and a code the
// codec decodes that the driver never offers would be a channel the user
// can read and not re-select.
func TestModes_MatchTheCodec(t *testing.T) {
	enum := codecEnum(t, civ.FieldMode)
	fromCodec := make(map[string]bool, len(enum))
	for _, name := range enum {
		fromCodec[name] = true
	}
	advertised := make(map[string]bool, len(capabilitiesUnverified().Modes))
	for _, name := range capabilitiesUnverified().Modes {
		if advertised[name] {
			t.Errorf("Modes lists %q twice", name)
		}
		advertised[name] = true
		if !fromCodec[name] {
			t.Errorf("Modes offers %q, which the codec's mode enum cannot decode", name)
		}
	}
	for name := range fromCodec {
		if !advertised[name] {
			t.Errorf("the codec decodes mode %q, which this driver never offers", name)
		}
	}
	// RULING OQ1, pinned at the wire level: the printed 12 and 13 are the
	// wire bytes 0x12 and 0x13, not the decimal values 12 and 13.
	if enum[0x12] != "PSK" || enum[0x13] != "PSK-R" {
		t.Errorf("the codec maps 0x12->%q and 0x13->%q; RULING OQ1 (24/08/2026) is HEXADECIMAL and requires PSK and PSK-R", enum[0x12], enum[0x13])
	}
	if _, decimal := enum[0x0C]; decimal {
		t.Error("the codec maps 0x0c to a mode name; that is the DECIMAL reading of the printed \"12\", which RULING OQ1 rejected")
	}
}

// TestFilters_MatchTheCodec is TestModes_MatchTheCodec's twin for byte
// (10), and additionally pins the 0x00 decision: the page prints three
// values and no default, so a record whose (10) is 0x00 must fail to
// decode rather than be read as "no filter".
func TestFilters_MatchTheCodec(t *testing.T) {
	enum := codecEnum(t, civ.FieldFilter)
	fromCodec := make(map[string]bool, len(enum))
	for _, name := range enum {
		fromCodec[name] = true
	}
	for _, name := range capabilitiesUnverified().Filters {
		if !fromCodec[name] {
			t.Errorf("Filters offers %q, which the codec's filter enum cannot decode", name)
		}
		delete(fromCodec, name)
	}
	for name := range fromCodec {
		t.Errorf("the codec decodes filter %q, which this driver never offers", name)
	}
	if _, ok := enum[0x00]; ok {
		t.Error("the codec maps 0x00 to a filter name - the page prints three values and no default, and inventing a fourth would be a radio claim")
	}
}

// TestToneDomainIsThePrintedRangeIntersectedWithE3sFloor pins the declared
// domain, both bounds, and the floor.
//
// The printed digit range and the declared domain are DIFFERENT, and the
// difference is the point (tier ruling T1(2)).
//
// PDF p.14's six rotated nibble labels (matrix S1 row 8) print
// 100Hz: 0-2, 10 Hz: 0-9, 1 Hz: 0-9, 0.1 Hz: 0-9, so the WIRE encodes
// 0..2999 deciHz. The CAPABILITY floor is max(printed minimum, 1),
// because 0 Hz is not a tone: spec.ToneRange requires MinDeciHz > 0 and
// Capabilities.Validate refuses a zero minimum.
//
// The wire's 0 is not thereby lost - it is handled one layer up, at
// ReadChannel, which maps an out-of-domain tone number (0 included) to
// UNKNOWN rather than to a Known value Validate would refuse (T1(3),
// Task 11).
//
// E3's RECORDED COST, which belongs in this driver's honesty rows at
// Wave 4: the tone PICKER stays list-driven, so on this model the grid
// shows and round-trips tones but the picker cannot offer them.
func TestToneDomainIsThePrintedRangeIntersectedWithE3sFloor(t *testing.T) {
	caps := capabilitiesUnverified()
	r := caps.CTCSSToneRange
	if r == nil {
		t.Fatal("CTCSSToneRange is nil - this radio carries tone as a BCD frequency and declares a range, not a chart")
	}
	if r.MinDeciHz != 1 || r.MaxDeciHz != 2999 || r.StepDeciHz != 1 {
		t.Errorf("CTCSSToneRange = {%d, %d, %d}, want {1, 2999, 1}", r.MinDeciHz, r.MaxDeciHz, r.StepDeciHz)
	}
	if len(caps.CTCSSTones) != 0 {
		t.Error("CTCSSTones is non-empty - a radio declares a chart OR a range, never both, and this one has no tone number to index")
	}
	// THE FLOOR, pinned in both directions.
	if caps.AdmitsTone(0) {
		t.Error("AdmitsTone(0) is true - 0 Hz is not a tone (T1(2)); the wire's 0 is ReadChannel's business, not the capability's")
	}
	for _, admit := range []spec.Tone{1, 885, 1000, 2999} {
		if !caps.AdmitsTone(admit) {
			t.Errorf("AdmitsTone(%d) is false, want true", admit)
		}
	}
	for _, refuse := range []spec.Tone{3000, 30000} {
		if caps.AdmitsTone(refuse) {
			t.Errorf("AdmitsTone(%d) is true, want false", refuse)
		}
	}
	// The declared ceiling and the driver's own pre-build refusal bound
	// are ONE number, so Task 12's refusal cannot drift from the domain
	// the capability advertises.
	if int(r.MaxDeciHz) != MaxToneDeciHz {
		t.Errorf("CTCSSToneRange.MaxDeciHz %d and MaxToneDeciHz %d disagree", r.MaxDeciHz, MaxToneDeciHz)
	}
}

// TestBaseline_WrittenDownZeroes names every zeroed cell of matrix S2 so
// each reads as a decision rather than an oversight.
//
// scan_skip and data_mode are REV 2's change FROM matrix S2, which has
// scan_skip Sup/Sup on MEM and data_mode Sup/Sup on both banks. Ruling E6
// unmaps both nibbles, and an unmapped region is not a field this driver
// can honestly claim. Two matrix errata are proposed under R16.
func TestBaseline_WrittenDownZeroes(t *testing.T) {
	zeroes := []struct {
		field spec.Field
		why   string
	}{
		{spec.FieldClarifier, "the 1A 00 record has no clarifier field"},
		{spec.FieldCTCSSState, "this radio expresses tone through tone_mode, the Icom vocabulary; ctcss_state is the Yaesu one"},
		{spec.FieldCTCSSTone, "the record's tone spans are tone_tx and tone_rx; there is no third, mode-agnostic tone field"},
		{spec.FieldShift, "the record has no repeater shift field"},
		{spec.FieldTagDisplay, "the record has no name-display flag"},
		{spec.FieldErase, "spec D4 Erase: the wire form EXISTS on this radio, in two shapes, and NO IC-7610 has ever been asked to use one; spec.ConsentUnverifiedWrites structurally never consents this field"},
		{spec.FieldTxFrequency, "the record carries ONE frequency span; there is no split-TX field"},
		{spec.FieldDuplex, "the record has no duplex field"},
		{spec.FieldOffset, "the record has no repeater offset field"},
		{spec.FieldDTCSCode, "DTCS is printed nowhere in this document's 17 pages"},
		{spec.FieldDTCSPolarity, "DTCS is printed nowhere in this document's 17 pages"},
		{spec.FieldScanSkip, "RULING E6: byte (3)'s LOW nibble is a four-valued SELECT-group marker (0=OFF/1=*1/2=*2/3=*3) and codeplug.ChannelData.ScanSkip is a BoolField; a 4->2 collapse would rewrite a user's SELECT group, so the nibble is UNMAPPED and the field is Unsupported both ways. On Icom, scan_skip is SELECT-group membership, never a skip"},
		{spec.FieldDataMode, "RULING E6: byte (11)'s HIGH nibble is a four-valued data mode (0=OFF/1=DATA 1/2=DATA 2/3=DATA 3) and codeplug.ChannelData.DataMode is a BoolField; same collapse, same ruling, same UNMAPPED outcome"},
	}
	for name, caps := range bothProfiles() {
		for _, bank := range bothBanks {
			for _, z := range zeroes {
				got := caps.FieldSupport(bank, z.field)
				if got != (spec.FieldSupport{}) {
					t.Errorf("%s/%s/%s = %+v, want the zero FieldSupport: %s", name, bank, z.field, got, z.why)
				}
			}
		}
	}
}

// TestE6UnmappedFieldsAreUnsupportedBothWays pins the two E6 unmaps
// against consent as well as against both profiles.
//
// THE CONSEQUENCE, stated plainly: a Known ScanSkip or DataMode is
// REFUSED BY THE CAPABILITY GATE BEFORE ANY WIRE TRAFFIC - never dropped,
// and never collapsed 4->2 (adjudication R6). Task 12's requestedFields
// appends both fields when they are Known precisely so the gate can see
// them and refuse, rather than filtering them out and writing a record
// that quietly disagrees with what the user asked for.
func TestE6UnmappedFieldsAreUnsupportedBothWays(t *testing.T) {
	profiles := bothProfiles()
	profiles["Consented"] = spec.ConsentUnverifiedWrites(capabilitiesUnverified())
	for name, caps := range profiles {
		for _, bank := range bothBanks {
			for _, f := range []spec.Field{spec.FieldScanSkip, spec.FieldDataMode} {
				fs := caps.FieldSupport(bank, f)
				if fs != (spec.FieldSupport{}) {
					t.Errorf("%s/%s/%s = %+v, want the zero FieldSupport (E6: the nibble is unmapped)", name, bank, f, fs)
				}
				if fs.CanWrite() {
					t.Errorf("%s/%s/%s reports CanWrite - consent must not reach an unmapped region", name, bank, f)
				}
			}
		}
	}
}

// TestScanBank_MatchesTheMatrixPerCell asserts the SCAN bank's remaining
// per-cell facts: every field the 1A 00 record EXPRESSES is readable on
// P1 and P2 exactly as on a memory channel, because the record is the
// SAME record - matrix S3.15(d): P1 and P2 are not a separate bank in the
// wire protocol at all, just two more values of the same two-byte
// selector. Whether every field is HONOURED on a scan edge rides matrix
// lift R18.
//
// REV 1 pinned scan_skip write-Unsupported here on the strength of PDF
// p.12's "(i) Set 0 for P1 and P2." Under E6 the field is Unsupported on
// BOTH banks, so that note now CORROBORATES the grade rather than
// carrying it.
func TestScanBank_MatchesTheMatrixPerCell(t *testing.T) {
	for name, caps := range bothProfiles() {
		mem, ok := caps.Bank(spec.BankMemory)
		if !ok {
			t.Fatalf("%s: no MEM bank", name)
		}
		scan, ok := caps.Bank(spec.BankScan)
		if !ok {
			t.Fatalf("%s: no SCAN bank", name)
		}
		if !reflect.DeepEqual(mem.Fields, scan.Fields) {
			t.Errorf("%s: the SCAN bank's field map differs from MEM's - the two banks read the SAME 1A 00 record (matrix S3.15(d)); a per-cell difference here would be a claim this document does not make", name)
		}
		for _, f := range mappedFields {
			if caps.FieldSupport(spec.BankScan, f).Read == spec.Unsupported {
				t.Errorf("%s: SCAN/%s is not readable - the record expresses it on every channel address", name, f)
			}
		}
		if caps.FieldSupport(spec.BankScan, spec.FieldScanSkip) != (spec.FieldSupport{}) {
			t.Errorf("%s: SCAN/scan_skip is not the zero FieldSupport (E6 unmaps it on both banks; PDF p.12's \"Set 0 for P1 and P2\" corroborates)", name)
		}
	}
}

// TestUnverified_NothingIsWritable: under capabilitiesUnverified, NOTHING
// this radio has is writable. No IC-7610 has ever answered a frame, so a
// write is a claim this project cannot make without recorded consent.
func TestUnverified_NothingIsWritable(t *testing.T) {
	caps := capabilitiesUnverified()
	for _, bank := range bothBanks {
		b, ok := caps.Bank(bank)
		if !ok {
			t.Fatalf("no %s bank", bank)
		}
		for f := range b.Fields {
			if caps.FieldSupport(bank, f).CanWrite() {
				t.Errorf("%s/%s is writable under the Unverified profile", bank, f)
			}
		}
	}
}

// TestConsent_LiftsEveryWriteButErase: spec.ConsentUnverifiedWrites turns
// every Unverified write into ConsentedUnverified, EXCEPT FieldErase,
// which carries the zero FieldSupport in both profiles and which consent
// structurally never touches (spec D4 "Erase", core/spec/consent.go's
// `f != FieldErase` guard).
func TestConsent_LiftsEveryWriteButErase(t *testing.T) {
	base := capabilitiesUnverified()
	consented := spec.ConsentUnverifiedWrites(base)
	for _, bank := range bothBanks {
		for _, f := range mappedFields {
			if got := consented.FieldSupport(bank, f).Write; got != spec.ConsentedUnverified {
				t.Errorf("%s/%s write = %v after consent, want ConsentedUnverified", bank, f, got)
			}
		}
		if got := consented.FieldSupport(bank, spec.FieldErase); got != (spec.FieldSupport{}) {
			t.Errorf("%s/erase = %+v after consent, want the zero FieldSupport - consent structurally never reaches erase", bank, got)
		}
	}
	// The transform must not have mutated the value it was handed.
	for _, bank := range bothBanks {
		if base.FieldSupport(bank, spec.FieldFrequency).Write != spec.Unverified {
			t.Errorf("%s: the base profile was mutated by ConsentUnverifiedWrites", bank)
		}
	}
}

// TestSimulated_IsAClaimAboutTheFakeOnly: under capabilitiesSimulated,
// Read and Write are Supported for exactly the fields the 1A 00 record
// MAPS, and every written-down zero stays zero - INCLUDING scan_skip and
// data_mode, because E6's unmapped regions are unmapped against the fake
// too. No amount of cooperative fake on the other end of the wire changes
// what the neutral model has room for.
func TestSimulated_IsAClaimAboutTheFakeOnly(t *testing.T) {
	caps := capabilitiesSimulated()
	want := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	for _, bank := range bothBanks {
		b, ok := caps.Bank(bank)
		if !ok {
			t.Fatalf("no %s bank", bank)
		}
		mapped := make(map[spec.Field]bool, len(mappedFields))
		for _, f := range mappedFields {
			mapped[f] = true
			if got := caps.FieldSupport(bank, f); got != want {
				t.Errorf("%s/%s = %+v, want %+v", bank, f, got, want)
			}
		}
		for f := range b.Fields {
			if mapped[f] {
				continue
			}
			if got := caps.FieldSupport(bank, f); got != (spec.FieldSupport{}) {
				t.Errorf("%s/%s = %+v under Simulated, want the zero FieldSupport - the record has no such field, or E6 unmaps it", bank, f, got)
			}
		}
	}
}

// TestDeliberatelyZeroAudit is adjudication R11's audit: every capability
// bound this model declares is either POPULATED from the matrix (with its
// section cited in caps.go's own comment against the field) or explicitly
// listed in caps.go's deliberatelyZero table with the reason it is zero.
//
// A zero left in a capability field is never a neutral omission - a zero
// MaxFreqHz reads as "no ceiling" to every validator, a zero TagLen makes
// core/csvio's CHIRP import truncate every imported name to nothing - so
// this test refuses to let a new spec.Capabilities field arrive and
// inherit a zero without this driver deciding about it.
func TestDeliberatelyZeroAudit(t *testing.T) {
	// 28 since additions design D4.2 added the transmit declaration.
	const wantFieldCount = 28

	for name, caps := range bothProfiles() {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(caps)
			typ := v.Type()
			if typ.NumField() != wantFieldCount {
				t.Fatalf("spec.Capabilities has %d fields, this audit knows %d - a field was added or removed and this driver must decide about it explicitly, not inherit a zero", typ.NumField(), wantFieldCount)
			}
			for i := 0; i < typ.NumField(); i++ {
				fname := typ.Field(i).Name
				f := v.Field(i)
				isZero := f.IsZero() || (f.Kind() == reflect.Slice && f.Len() == 0)
				why, listed := deliberatelyZero[fname]
				switch {
				case isZero && !listed:
					t.Errorf("field %s is zero but is not in caps.go's deliberatelyZero table - state the reason there or populate it from the matrix", fname)
				case !isZero && listed:
					t.Errorf("field %s is populated but is listed in deliberatelyZero as %q - the table has gone stale", fname, why)
				case isZero && listed && why == "":
					t.Errorf("field %s is in deliberatelyZero with no reason", fname)
				}
			}
		})
	}
}
