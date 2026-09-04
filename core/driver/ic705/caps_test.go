// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is matrix §2's row set, written out by hand: the ten spec.Fields
// of the Yaesu era plus the ten the Icom tier added. It is NOT every
// spec.Field this project models — the additions design's D8 minted seven
// receiver fields on 28/08/2026, which this radio's matrix §2 does not carry
// a row for. Hand-written so that the "every cell is explicit" test
// asserts a set rather than whatever the code under test happened to put in
// its map.
var allFields = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier, spec.FieldCTCSSState,
	spec.FieldCTCSSTone, spec.FieldShift, spec.FieldTag, spec.FieldTagDisplay,
	spec.FieldScanSkip, spec.FieldErase,
	spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset, spec.FieldToneMode,
	spec.FieldToneTx, spec.FieldToneRx, spec.FieldDTCSCode, spec.FieldDTCSPolarity,
	spec.FieldFilter, spec.FieldDataMode,
}

var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldTuningStepEnabled: "additions design D8 — the IC-705 memory frame carries no tuning-step-enabled field",
	spec.FieldTuningStep:        "additions design D8 — the IC-705 memory frame carries no tuning-step field",
	spec.FieldProgramTuningStep: "additions design D8 — the IC-705 memory frame carries no programmable-tuning-step field",
	spec.FieldAttenuator:        "additions design D8 — the IC-705 memory frame carries no attenuator field",
	spec.FieldPreamp:            "additions design D8 — the IC-705 memory frame carries no preamp field",
	spec.FieldAntenna:           "additions design D8 — the IC-705 memory frame carries no antenna-selection field",
	spec.FieldIPPlus:            "additions design D8 — the IC-705 memory frame carries no IP+ field",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// bothProfiles is every capability set this driver can hand out, for the
// checks that must hold of all of them.
func bothProfiles() map[string]spec.Capabilities {
	return map[string]spec.Capabilities{
		"unverified": capabilitiesUnverified(),
		"simulated":  capabilitiesSimulated(),
	}
}

// fullRecord is a complete, valid IC-705 memory record: every mapped field
// Known, the unmapped areas left to the profile's own template. It is the
// fixture the encoder-level tests build from, and it deliberately carries
// no Select value — record offset 0 is unmapped (O-6), so a record that
// named one would be describing a layout this profile does not have.
func fullRecord(addr civ.ChannelAddress) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      addr,
		RXFreqHz:     civ.Available[uint64](145500000),
		TXFreqHz:     civ.Available[uint64](145500000),
		OffsetHz:     civ.Available[uint64](600000),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](885),
		DTCSCode:     civ.Available[uint64](23),
		Duplex:       civ.Available("DUP-"),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("TONE"),
		DTCSPolarity: civ.Available("NN"),
		Name:         civ.Available("MY REPEATER CH01"),
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true for a radio no byte has ever been sent to")
	}
	caps := capabilitiesUnverified()
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if fs.CanWrite() {
				t.Errorf("bank %s field %s is writable on the unverified profile (Write %v)", b.ID, f, fs.Write)
			}
		}
	}
}

func TestProfilesValidate(t *testing.T) {
	// Both profiles pass spec.Capabilities.Validate: the sparse-descriptor
	// rules, DefaultBaud in Bauds, TagLen > 0, E3's "not both list and
	// range" rule, and E5b's admission of a legitimately empty
	// ShiftOptions/CTCSSStates on a model whose banks reach neither field.
	for name, caps := range bothProfiles() {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s profile: Validate: %v", name, err)
		}
	}
}

func TestMemBankIsSparseWithTheRecordedSpace(t *testing.T) {
	for name, caps := range bothProfiles() {
		b, ok := caps.Bank(spec.BankMemory)
		if !ok {
			t.Fatalf("%s profile: no MEM bank", name)
		}
		if !b.Sparse {
			t.Errorf("%s profile: MEM bank is not Sparse", name)
		}
		if b.Groups != 100 || b.PerGroup != 100 {
			t.Errorf("%s profile: MEM space is %dx%d, want 100x100 (matrix §1b)", name, b.Groups, b.PerGroup)
		}
		if b.GroupBase != 1 || b.ChannelBase != 1 {
			t.Errorf("%s profile: MEM bases = %d/%d, want 1/1 so G01-001 remains the first radio slot", name, b.GroupBase, b.ChannelBase)
		}
		if b.Budget != 500 {
			t.Errorf("%s profile: MEM Budget = %d, want 500 (spec D4/D6; ASSUMED — ic705-group-budget, lift L-BUDGET-CEILING)", name, b.Budget)
		}
		if len(b.Slots) != 0 {
			t.Errorf("%s profile: MEM bank lists %d slots — a sparse bank's Slots is what a READ materialised, and no session has read anything yet (Task 9a fills it)", name, len(b.Slots))
		}
		if b.NoBlank {
			t.Errorf("%s profile: MEM bank claims NoBlank — an ordinary memory may be empty", name)
		}
	}
}

func TestCallBankIsDenseWithFourSlots(t *testing.T) {
	want := []string{"G101-001", "G101-002", "G101-003", "G101-004"}
	for name, caps := range bothProfiles() {
		b, ok := caps.Bank(spec.BankCall)
		if !ok {
			t.Fatalf("%s profile: no CALL bank", name)
		}
		if len(b.Slots) != len(want) {
			t.Fatalf("%s profile: CALL holds %d slots, want %d", name, len(b.Slots), len(want))
		}
		for i, s := range want {
			if b.Slots[i] != s {
				t.Errorf("%s profile: CALL slot %d = %q, want %q", name, i, b.Slots[i], s)
			}
		}
		if !b.NoBlank {
			t.Errorf("%s profile: CALL bank does not set NoBlank — a call channel can never be empty (ASSUMED, ic705-call-channel-emptiness, lift L-CALL-EMPTY)", name)
		}
		// Sparse false and every sparse descriptor zero, or
		// spec.Validate refuses the bank outright.
		if b.Sparse || b.Groups != 0 || b.GroupBase != 0 || b.PerGroup != 0 || b.ChannelBase != 0 || b.Budget != 0 || b.BudgetUnstated {
			t.Errorf("%s profile: CALL bank carries a sparse descriptor (Sparse:%v Groups:%d GroupBase:%d PerGroup:%d ChannelBase:%d Budget:%d BudgetUnstated:%v) — it is dense",
				name, b.Sparse, b.Groups, b.GroupBase, b.PerGroup, b.ChannelBase, b.Budget, b.BudgetUnstated)
		}
	}
}

func TestToneDomainIsARangeNotAList(t *testing.T) {
	// T1(2). The printed digit ranges (PDF p.23, folio 22: 100 Hz digit
	// 0~2, then 0~9 three times) make 0..2999 ENCODABLE; T1 raises the
	// DECLARED floor to 1 because 0 Hz is not a tone, and
	// spec.Validate refuses MinDeciHz <= 0 outright.
	for name, caps := range bothProfiles() {
		if len(caps.CTCSSTones) != 0 {
			t.Errorf("%s profile: CTCSSTones has %d entries — this radio's tone field is a NUMBER, so the domain is a range and the list must be empty (E3)", name, len(caps.CTCSSTones))
		}
		r := caps.CTCSSToneRange
		if r == nil {
			t.Fatalf("%s profile: no CTCSSToneRange declared", name)
		}
		if r.MinDeciHz != 1 || r.MaxDeciHz != 2999 || r.StepDeciHz != 1 {
			t.Errorf("%s profile: CTCSSToneRange = %+v, want {1, 2999, 1} (T1(2))", name, *r)
		}
		if err := caps.Validate(); err != nil {
			t.Errorf("%s profile: the declared tone shape does not validate: %v", name, err)
		}
		for _, tc := range []struct {
			tone spec.Tone
			want bool
		}{
			{0, false}, // the wire zero is NOT a tone — T1(3)/(4) route it through Unknown/preserve
			{1, true},
			{885, true},
			{2999, true},
			{3000, false},
		} {
			if got := caps.AdmitsTone(tc.tone); got != tc.want {
				t.Errorf("%s profile: AdmitsTone(%v) = %v, want %v", name, tc.tone, got, tc.want)
			}
		}
		// O-12's negative, asserted: the declared maximum EQUALS the
		// encodable maximum, so the declared/encodable gap is exactly the
		// single value zero and nothing else has drifted.
		const encodableMax = 2999
		if int(r.MaxDeciHz) != encodableMax {
			t.Errorf("%s profile: MaxDeciHz %d != the encodable maximum %d — the O-12 gap must be the single value zero", name, r.MaxDeciHz, encodableMax)
		}
	}
}

// deliberatelyZero is the audit R11 requires: every capability value this
// driver leaves at zero ON PURPOSE, with the reason and the register entry
// that owes the real number. A zero that is NOT in this table is an author
// who forgot one; a table entry whose value is no longer zero is a claim
// somebody filled in without recording where it came from.
var deliberatelyZero = []struct {
	name   string
	value  func(spec.Capabilities) int
	reason string
}{
	{"MinFreqHz", func(c spec.Capabilities) int { return int(c.MinFreqHz) },
		"the matrix leaves this radio's lowest STORABLE frequency ASSUMED and unfilled (§1 row 11); the record field's encoding range is not the radio's tuning floor — ic705-min-storable-frequency, lift L-FREQ-FLOOR"},
	{"MaxFreqHz", func(c spec.Capabilities) int { return int(c.MaxFreqHz) },
		"likewise the highest storable frequency (§1 row 12) — ic705-max-storable-frequency, lift L-FREQ-CEIL"},
	{"ClarMaxHz", func(c spec.Capabilities) int { return c.ClarMaxHz },
		"this radio has no per-channel clarifier at all (matrix §2: clarifier Unsupported on both banks)"},
	{"ClarStepHz", func(c spec.Capabilities) int { return c.ClarStepHz },
		"as ClarMaxHz"},
	{"len(CTCSSTones)", func(c spec.Capabilities) int { return len(c.CTCSSTones) },
		"E3: this radio's tone field is a number, so the domain is CTCSSToneRange and the list is empty by declaration"},
	{"len(ShiftOptions)", func(c spec.Capabilities) int { return len(c.ShiftOptions) },
		"the Yaesu shift vocabulary; this radio expresses duplex instead, and E5b admits the empty half because no bank reaches FieldShift"},
	{"len(CTCSSStates)", func(c spec.Capabilities) int { return len(c.CTCSSStates) },
		"as ShiftOptions: this radio expresses tone_mode instead"},
	{"len(RequiredSlots)", func(c spec.Capabilities) int { return len(c.RequiredSlots) },
		"no individual slot on this radio is documented as never-empty; the CALL BANK's NoBlank carries what is claimed, and claims nothing per-slot"},
	{"len(TuningSteps)", func(c spec.Capabilities) int { return len(c.TuningSteps) },
		"additions design D8 — this record carries no receiver tuning-step field"},
	{"ProgramTuningStepRange", func(c spec.Capabilities) int {
		if c.ProgramTuningStepRange == nil {
			return 0
		}
		return 1
	},
		"additions design D8 — this record carries no programmable tuning-step field"},
	{"len(AttenuatorDB)", func(c spec.Capabilities) int { return len(c.AttenuatorDB) },
		"additions design D8 — this record carries no attenuator field"},
	{"len(PreampOptions)", func(c spec.Capabilities) int { return len(c.PreampOptions) },
		"additions design D8 — this record carries no preamp field"},
	{"len(AntennaOptions)", func(c spec.Capabilities) int { return len(c.AntennaOptions) },
		"additions design D8 — this record carries no antenna field"},
}

func TestDeliberateZerosAreAudited(t *testing.T) {
	// The audit covers all 28 top-level capabilities plus the two sparse
	// numbering bases pinned by TestMemBankIsSparseWithTheRecordedSpace.
	if got := reflect.TypeOf(spec.Capabilities{}).NumField() + 2; got != 30 {
		t.Fatalf("capability/base audit has %d fields, this audit knows 30", got)
	}
	for name, caps := range bothProfiles() {
		for _, z := range deliberatelyZero {
			if got := z.value(caps); got != 0 {
				t.Errorf("%s profile: %s = %d, but it is listed as a deliberate zero (%s) — fill the register entry or drop the audit row", name, z.name, got, z.reason)
			}
		}
	}
	if len(deliberatelyZero) == 0 {
		t.Fatal("the audit table is empty — this test would pass vacuously")
	}
}

func TestFrequencyBoundsAreZeroAndAudited(t *testing.T) {
	// The bounds are ZERO and audited above; what remains to pin here is
	// the WITHDRAWN claim. REV 2 said civ refuses a 500 MHz frequency; it
	// does not — the landed BCD validator checks scale and byte width
	// only, and 500,000,000 fits five packed-BCD bytes perfectly well. The
	// manual's ceiling comes from the printed "1 GHz digit: (fixed)" and
	// "100 MHz digit: 0 ~ 4" leaders, which no byte-width check can see,
	// so the <500 MHz rule is a DRIVER refusal (Task 11's ladder).
	p := civic705.Profile()
	if _, err := p.BuildMemorySet(fullRecord(civ.ChannelAddress{Group: 0, Channel: 0})); err != nil {
		t.Fatalf("the fixture record does not encode at all: %v", err)
	}
	tooWide := fullRecord(civ.ChannelAddress{Group: 0, Channel: 0})
	tooWide.RXFreqHz = civ.Available[uint64](10000000000) // eleven digits; five BCD bytes hold ten
	if _, err := p.BuildMemorySet(tooWide); err == nil {
		t.Error("civ encoded a frequency too wide for five packed-BCD bytes")
	}
	atFiveHundred := fullRecord(civ.ChannelAddress{Group: 0, Channel: 0})
	atFiveHundred.RXFreqHz = civ.Available[uint64](500000000)
	if _, err := p.BuildMemorySet(atFiveHundred); err != nil {
		t.Errorf("civ refused 500 MHz (%v) — if it now refuses it, Task 11's own ceiling rung is not the only thing standing between a consented write and a digit the manual bounds at 4, and this comment must be rewritten rather than left as a stale claim", err)
	}
}

func TestDTCSTableIsTheFieldsPrintedDomain(t *testing.T) {
	// 512 values, every decimal digit 0-7, strictly ascending, first 0,
	// last 777 — the field's PRINTED domain (PDF p.23, folio 22, three
	// digit leaders each "0 ~ 7").
	//
	// GOVERNED BY ENABLERS E3, and the ruling is named at the declaration
	// in caps.go: "DTCS stays an explicit table: the 512 codes 000..777
	// (every digit <= 7) are NOT a contiguous range". The review finding
	// that this should instead be empty-or-narrowed was DISPUTED and the
	// dispute SUSTAINED on 24/08/2026 (O-10): an empty table fails closed
	// on every Known code, which would make this radio's DTCS channels
	// unreadable rather than merely unverified. The narrowing to the
	// radio's actual selectable subset is carried as ic705-dtcs-selectable-set,
	// lift L-DTCS-SET.
	for name, caps := range bothProfiles() {
		codes := caps.DTCSCodes
		if len(codes) != 512 {
			t.Fatalf("%s profile: %d DTCS codes, want 512", name, len(codes))
		}
		if codes[0] != 0 || codes[511] != 777 {
			t.Errorf("%s profile: DTCS table runs %d..%d, want 0..777", name, codes[0], codes[511])
		}
		for i, c := range codes {
			if i > 0 && c <= codes[i-1] {
				t.Fatalf("%s profile: DTCS table is not strictly ascending at index %d (%d after %d)", name, i, c, codes[i-1])
			}
			for n := c; n > 0; n /= 10 {
				if n%10 > 7 {
					t.Fatalf("%s profile: DTCS code %d carries a digit above 7", name, c)
				}
			}
		}
	}
}

func TestScanSkipIsUnsupportedOnBothBanks(t *testing.T) {
	// O-6. The star nibble marks a channel INTO a select-scan group;
	// scan_skip means "skip this one". The nibble is UNMAPPED in the
	// record (R6), so a marked channel is REFUSED on write, never demoted.
	// This DIVERGES from matrix §2, which grades the cell Supported; §2's
	// own A2 calls the mapping a live question for the plan, and the plan
	// answers it (matrix erratum proposed).
	for name, caps := range bothProfiles() {
		for _, id := range []spec.BankID{spec.BankMemory, spec.BankCall} {
			if fs := caps.FieldSupport(id, spec.FieldScanSkip); !fs.Unreachable() {
				t.Errorf("%s profile: bank %s reaches scan_skip (%+v) — the ★n nibble is select-group membership, not skip", name, id, fs)
			}
		}
	}
}

func TestEraseIsZeroOnEveryBank(t *testing.T) {
	// Spec D4 adjudication 19: no erase path at all this tier, though TWO
	// clear wire forms exist on this radio (1A 00 with FF at the fifth
	// position, and command 0B) — recorded in doc.go, shipped nowhere.
	for name, caps := range bothProfiles() {
		for _, id := range []spec.BankID{spec.BankMemory, spec.BankCall} {
			if fs := caps.FieldSupport(id, spec.FieldErase); !fs.Unreachable() {
				t.Errorf("%s profile: bank %s reaches erase (%+v)", name, id, fs)
			}
		}
	}
}

func TestConsentNeverReachesErase(t *testing.T) {
	base := capabilitiesUnverified()
	c := spec.ConsentUnverifiedWrites(base)
	moved := 0
	for _, b := range c.Banks {
		for f, fs := range b.Fields {
			was := base.FieldSupport(b.ID, f)
			switch {
			case f == spec.FieldErase:
				if fs.Write != spec.Unsupported {
					t.Errorf("bank %s: consent moved erase to %v", b.ID, fs.Write)
				}
			case was.Write == spec.Unverified:
				if fs.Write != spec.ConsentedUnverified {
					t.Errorf("bank %s field %s: consent left Write at %v, want ConsentedUnverified", b.ID, f, fs.Write)
				}
				moved++
			default:
				if fs.Write != was.Write {
					t.Errorf("bank %s field %s: consent changed Write %v -> %v", b.ID, f, was.Write, fs.Write)
				}
			}
		}
	}
	if moved == 0 {
		t.Fatal("consent moved nothing — this test would pass vacuously")
	}
}

func TestEveryFieldOfEveryBankIsExplicit(t *testing.T) {
	// Matrix §2's FORTY CELLS, as code. A field absent from the map reads
	// as Unsupported BY ACCIDENT, which is indistinguishable from
	// forgetting it.
	cells := 0
	for name, caps := range bothProfiles() {
		for _, b := range caps.Banks {
			if len(b.Fields) != len(allFields) {
				t.Errorf("%s profile: bank %s lists %d fields, want %d", name, b.ID, len(b.Fields), len(allFields))
			}
			for _, f := range allFields {
				if _, ok := b.Fields[f]; !ok {
					t.Errorf("%s profile: bank %s does not mention field %s", name, b.ID, f)
					continue
				}
				cells++
			}
		}
	}
	if cells != 2*len(bothProfiles())*len(allFields) {
		t.Fatalf("counted %d written-down cells, want %d — the sweep is incomplete", cells, 2*len(bothProfiles())*len(allFields))
	}
}

func TestSimulatedProfileIsNotTheUnverifiedOne(t *testing.T) {
	unv := capabilitiesUnverified()
	sim := capabilitiesSimulated()
	writable := 0
	for _, b := range sim.Banks {
		for f, fs := range b.Fields {
			if fs.CanWrite() {
				writable++
				if unv.FieldSupport(b.ID, f).CanWrite() {
					t.Errorf("bank %s field %s is writable on the UNVERIFIED profile too", b.ID, f)
				}
			}
		}
	}
	if writable == 0 {
		t.Fatal("the simulated profile has no writable field — the write choreography could not be exercised against the fake at all")
	}
}

func TestVocabulariesAreCanonicalAndDistinct(t *testing.T) {
	// E5's canonical marking is TRIVIALLY satisfied here — three distinct
	// duplex directions and four distinct tone-mode semantics, one entry
	// each — and the plan requires that be ASSERTED rather than assumed.
	caps := capabilitiesUnverified()
	dirs := map[spec.DuplexDirection]int{}
	for _, d := range caps.DuplexOptions {
		if d.Direction == spec.DuplexUnspecified {
			t.Errorf("duplex option %q carries the zero direction", d.Value)
		}
		if d.Canonical {
			t.Errorf("duplex option %q is marked Canonical, but no direction is expressed twice — the marking would be ceremony, and Validate requires it only where there is a choice to make", d.Value)
		}
		dirs[d.Direction]++
	}
	if len(dirs) != len(caps.DuplexOptions) {
		t.Errorf("the %d duplex options express only %d distinct directions", len(caps.DuplexOptions), len(dirs))
	}
	sems := map[spec.ToneModeSemantics]int{}
	for _, m := range caps.ToneModes {
		if m.Semantics == spec.ToneModeUnspecified {
			t.Errorf("tone mode %q carries the zero semantics", m.Value)
		}
		if m.Canonical {
			t.Errorf("tone mode %q is marked Canonical, but no semantic is expressed twice", m.Value)
		}
		sems[m.Semantics]++
	}
	if len(sems) != len(caps.ToneModes) {
		t.Errorf("the %d tone modes express only %d distinct semantics", len(caps.ToneModes), len(sems))
	}
}

func TestTagCharsetIsTheProfilesOwnCharset(t *testing.T) {
	// TagByteOK (the codeplug/CSV side) and civ's name validator (the wire
	// side) must not drift: both come from ONE string.
	caps := capabilitiesUnverified()
	profileCharset := string(civic705.Profile().NameCharset())
	if caps.TagCharset != profileCharset {
		t.Errorf("TagCharset (%d bytes) differs from the civ profile's name charset (%d bytes) — the model layer and the wire layer would disagree about a legal name", len(caps.TagCharset), len(profileCharset))
	}
	if caps.TagLen != civic705.Profile().NameLength() {
		t.Errorf("TagLen = %d, want the profile's own name length %d", caps.TagLen, civic705.Profile().NameLength())
	}
	if !caps.TagByteOK(' ') {
		t.Error("the space is not a legal tag byte — the golden vector's name carries two of them (ASSUMED, lift L-NAME-SPACE)")
	}
	if caps.TagByteOK(0x7F) {
		t.Error("0x7F is a legal tag byte — the charset is 0x20..0x7E and no wider")
	}
}

func TestModesAreThePrintedTen(t *testing.T) {
	want := []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "WFM", "CW-R", "RTTY-R", "DV"}
	caps := capabilitiesUnverified()
	if len(caps.Modes) != len(want) {
		t.Fatalf("Modes has %d entries, want %d", len(caps.Modes), len(want))
	}
	for i, m := range want {
		if caps.Modes[i] != m {
			t.Errorf("Modes[%d] = %q, want %q (PDF p.18, folio 17, printed order)", i, caps.Modes[i], m)
		}
	}
}

func TestCloneCapabilitiesSharesNoStorage(t *testing.T) {
	// The session hands its capabilities out on every call and must not be
	// mutable through what it handed out — this is the project's
	// hardware-write gate data.
	src := capabilitiesUnverified()
	cp := cloneCapabilities(src)
	cp.Banks[0].Slots = append(cp.Banks[0].Slots, "G01-001")
	cp.Banks[0].Fields[spec.FieldFrequency] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	cp.Modes[0] = "TAMPERED"
	if len(src.Banks[0].Slots) != 0 {
		t.Error("appending to the copy's Slots reached the original")
	}
	if src.FieldSupport(src.Banks[0].ID, spec.FieldFrequency).CanWrite() {
		t.Error("rewriting the copy's write gate reached the original")
	}
	if src.Modes[0] == "TAMPERED" {
		t.Error("mutating the copy's Modes reached the original")
	}
}

func TestCATIDIsTheRadiosAddress(t *testing.T) {
	caps := capabilitiesUnverified()
	if caps.CATID != "A4" {
		t.Errorf("CATID = %q, want \"A4\" (matrix §1 row 2; the same byte the civ profile carries)", caps.CATID)
	}
	if got := civic705.Profile().RadioAddress(); got != 0xA4 {
		t.Errorf("the civ profile's radio address is %#02x — the two must not drift", got)
	}
	if caps.Model != "IC-705" {
		t.Errorf("Model = %q, want \"IC-705\"", caps.Model)
	}
}

func TestBaudsAreAssumedPlaceholdersWithTheDefaultAmongThem(t *testing.T) {
	// R14's admission of the Basic Manual settled TRANSCEIVE and ECHO, not
	// this. What the page images DO settle is a negative: the microUSB
	// CI-V port is baud-agnostic ("You can communicate regardless of the
	// PC software's baud rate setting" — PDF p.92, printed folio 13-2,
	// §13 CONNECTOR INFORMATION, [microUSB] › USB Serial Port), so no
	// default rate is printed anywhere and these stay ASSUMED with lifts
	// L-BAUD and L-BAUDLIST.
	caps := capabilitiesUnverified()
	if caps.DefaultBaud <= 0 {
		t.Fatalf("DefaultBaud = %d", caps.DefaultBaud)
	}
	found := false
	for _, b := range caps.Bauds {
		if b == caps.DefaultBaud {
			found = true
		}
	}
	if !found {
		t.Errorf("DefaultBaud %d is not in Bauds %v — spec.Validate refuses that, and OpenSerial would silently substitute its own", caps.DefaultBaud, caps.Bauds)
	}
}
