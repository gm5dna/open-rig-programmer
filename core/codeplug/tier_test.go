// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// icomTestCapabilities builds a Capabilities in the shape the Icom tier
// registers: a SPARSE, group-addressed memory bank with a budget, the
// duplex/tone-mode vocabularies instead of the Yaesu shift/CTCSS-state
// pair, and the tier's fields declared writable so the gates under test
// have something to let through. It is a TEST fixture, not a claim about
// any real radio: no Icom driver exists yet, and none of these values is
// evidence.
func icomTestCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	fields := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:    {Read: spec.Supported, Write: spec.Supported},
		spec.FieldMode:         {Read: spec.Supported, Write: spec.Supported},
		spec.FieldTag:          {Read: spec.Supported, Write: spec.Supported},
		spec.FieldTxFrequency:  {Read: spec.Supported, Write: spec.Supported},
		spec.FieldDuplex:       {Read: spec.Supported, Write: spec.Supported},
		spec.FieldOffset:       {Read: spec.Supported, Write: spec.Supported},
		spec.FieldToneMode:     {Read: spec.Supported, Write: spec.Supported},
		spec.FieldToneTx:       {Read: spec.Supported, Write: spec.Supported},
		spec.FieldToneRx:       {Read: spec.Supported, Write: spec.Unverified},
		spec.FieldDTCSCode:     {Read: spec.Supported, Write: spec.Supported},
		spec.FieldDTCSPolarity: {Read: spec.Supported, Write: spec.Supported},
		spec.FieldFilter:       {Read: spec.Supported, Write: spec.Supported},
		spec.FieldDataMode:     {Read: spec.Supported, Write: spec.Supported},
	}
	return spec.Capabilities{
		Model: "TEST-ICOM", CATID: "A4",
		Modes: []string{"USB", "FM"}, TagLen: 10,
		CTCSSTones:  tones[:],
		Bauds:       []int{19200},
		DefaultBaud: 19200,
		MinFreqHz:   30_000,
		MaxFreqHz:   10_500_000_000,
		DuplexOptions: []spec.DuplexOption{
			{Value: "OFF", Direction: spec.DuplexOff},
			{Value: "DUP+", Direction: spec.DuplexUp},
			{Value: "DUP-", Direction: spec.DuplexDown},
		},
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
			{Value: "DTCS", Semantics: spec.ToneModeDTCS},
		},
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		DTCSCodes:      []int{23, 25, 754},
		Filters:        []string{"FIL1", "FIL2", "FIL3"},
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Label: "Memories",
			Slots:  []string{"G01-001", "G01-002"},
			Sparse: true, Groups: 100, GroupBase: 1, PerGroup: 100, ChannelBase: 1, Budget: 3,
			Fields: fields,
		}},
	}
}

// icomChannelData is a fully-specified channel for icomTestCapabilities.
func icomChannelData(freqHz uint64) *ChannelData {
	return &ChannelData{
		FreqHz: freqHz, Mode: "FM", Tag: "REPEATER",
		CTCSSTone:           ToneField{State: Unavailable},
		TagDisplay:          BoolField{State: Unavailable},
		ScanSkip:            BoolField{State: Unavailable},
		TxFreqHz:            FreqField{State: Unknown},
		Duplex:              StringField{State: Known, Value: "DUP-"},
		OffsetHz:            FreqField{State: Known, Value: 600_000},
		ToneMode:            StringField{State: Known, Value: "TSQL"},
		ToneTx:              ToneField{State: Known, Value: 885},
		ToneRx:              ToneField{State: Known, Value: 885},
		DTCSCode:            IntField{State: Unavailable},
		DTCSPolarity:        StringField{State: Unavailable},
		Filter:              StringField{State: Known, Value: "FIL2"},
		DataMode:            BoolField{State: Known, Value: false},
		TuningStepEnabled:   BoolField{State: Unavailable},
		TuningStep:          StringField{State: Unavailable},
		ProgramTuningStepHz: FreqField{State: Unavailable},
		AttenuatorDB:        IntField{State: Unavailable},
		Preamp:              StringField{State: Unavailable},
		Antenna:             StringField{State: Unavailable},
		IPPlus:              BoolField{State: Unavailable},
	}
}

// TestTouchedFields_YaesuUnchanged: on a radio registered before this
// tier the capability-keyed touched set is EXACTLY addedFields' answer,
// in exactly its order. This is the pin that says the tier's Diff work
// cannot have moved a Yaesu BlockReason.
//
// It runs against TWO capability fixtures, and the second is the one
// that does the work (Wave-1c review 1, finding 2):
// yaesuProfileShapedCapabilities() reproduces the registered profiles'
// ZERO FieldSupport entries for ctcss_tone, scan_skip and tag_display,
// so on it a Known value for any of the three is a request the radio
// cannot reach — and must STILL be in the touched set, so the per-field
// gate refuses the channel by name exactly as it did before the tier.
// The earlier version of this test used only testCapabilities(), which
// declares all three Supported, and therefore passed while the real
// FT-710 and FTdx refusals had been deleted.
func TestTouchedFields_YaesuUnchanged(t *testing.T) {
	for _, caps := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"a bank declaring all three", testCapabilities()},
		{"the registered profiles' zero entries", yaesuProfileShapedCapabilities()},
	} {
		t.Run(caps.name, func(t *testing.T) {
			touchedFieldsUnchanged(t, caps.caps)
		})
	}
}

func touchedFieldsUnchanged(t *testing.T, caps spec.Capabilities) {
	t.Helper()
	for _, tt := range []struct {
		name string
		data ChannelData
	}{
		{"all FieldState fields Known", ChannelData{
			CTCSSTone:  ToneField{State: Known},
			TagDisplay: BoolField{State: Known},
			ScanSkip:   BoolField{State: Known},
		}},
		{"all FieldState fields Unknown", ChannelData{
			CTCSSTone:  ToneField{State: Unknown},
			TagDisplay: BoolField{State: Unknown},
			ScanSkip:   BoolField{State: Unknown},
		}},
		{"tag display Unavailable", ChannelData{
			CTCSSTone:  ToneField{State: Known},
			TagDisplay: BoolField{State: Unavailable},
			ScanSkip:   BoolField{State: Known},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := addedFields(tt.data)
			got := touchedFields(caps, spec.BankMemory, tt.data)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("touchedFields() = %v, want addedFields()'s %v unchanged", got, want)
			}
		})
	}
}

// TestTouchedFields_UnreachableFieldIsNotTouched pins the rule design D4
// states in those words, in the scope Wave-1c review 1 (finding 1)
// settled it to: a field the capabilities mark Unreachable in that bank
// is not touched by an add — for the UNCONDITIONAL six, which are in the
// set because the frame always carries them rather than because anyone
// asked. A bank that expresses only some of the six is the Icom case the
// filter exists for; here it is exercised directly on tag.
//
// The three Known-conditional fields keep their places in the same
// answer, unfiltered, which is the other half of the rule: their
// presence IS a request, so an unreachable one must reach the gate and
// be refused, not quietly leave the set.
func TestTouchedFields_UnreachableFieldIsNotTouched(t *testing.T) {
	caps := testCapabilities()
	// Make tag Unreachable on MEM and check it drops out — and ONLY it.
	bank := caps.Banks[0]
	fields := make(map[spec.Field]spec.FieldSupport, len(bank.Fields))
	for f, fs := range bank.Fields {
		fields[f] = fs
	}
	delete(fields, spec.FieldTag)
	caps.Banks[0].Fields = fields

	data := ChannelData{
		CTCSSTone:  ToneField{State: Known},
		TagDisplay: BoolField{State: Known},
		ScanSkip:   BoolField{State: Known},
	}
	got := touchedFields(caps, spec.BankMemory, data)
	for _, f := range got {
		if f == spec.FieldTag {
			t.Fatalf("touchedFields() = %v, want no tag: the bank cannot reach it", got)
		}
	}
	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
		spec.FieldCTCSSState, spec.FieldShift,
		spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("touchedFields() = %v, want %v (order preserved around the gap)", got, want)
	}
}

// TestTouchedFields_TierFieldsAdmittedByKnownAlone: the tier's fields
// join the touched set on ONE question — does this channel carry a Known
// value for the field, i.e. did the user ask for it? — and the radio's
// capabilities are deliberately not consulted at the door. A Known value
// the bank cannot reach still enters the set, so the per-field gate can
// REFUSE the channel with the reason that names the field, which is what
// this project does with a request it cannot honour (Wave-1c review 1,
// findings 1 and 5). Whether the field is writable is the gate's
// question, asked below in TestDiff_KnownTierFieldOnUnreachableBankIsRefused.
func TestTouchedFields_TierFieldsAdmittedByKnownAlone(t *testing.T) {
	caps := icomTestCapabilities()
	data := *icomChannelData(145_500_000)

	got := touchedFields(caps, spec.BankMemory, data)
	want := []spec.Field{
		// The pre-tier six, minus clarifier/ctcss_state/shift which this
		// radio's bank does not declare at all.
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		// Then the tier's, in ChannelData order, Known ones only.
		spec.FieldDuplex, spec.FieldOffset, spec.FieldToneMode,
		spec.FieldToneTx, spec.FieldToneRx, spec.FieldFilter, spec.FieldDataMode,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("touchedFields() = %v, want %v", got, want)
	}

	// Known SURVIVES unreachability: drop duplex from the bank and the
	// Known value stays touched, so the gate refuses it by name instead
	// of the write going out with the duplex request evaporated.
	fields := make(map[spec.Field]spec.FieldSupport, len(caps.Banks[0].Fields))
	for f, fs := range caps.Banks[0].Fields {
		fields[f] = fs
	}
	delete(fields, spec.FieldDuplex)
	caps.Banks[0].Fields = fields
	var sawDuplex bool
	for _, f := range touchedFields(caps, spec.BankMemory, data) {
		if f == spec.FieldDuplex {
			sawDuplex = true
		}
	}
	if !sawDuplex {
		t.Error("touchedFields() dropped a Known duplex once the bank stopped declaring it; the request must reach the gate to be refused")
	}
}

// TestDiff_KnownTierFieldOnUnreachableBankIsRefused is finding 5's pin at
// the level a user meets it: a version-2 CSV written for one radio,
// imported into a codeplug for a radio whose bank has no room for the
// field (csvio.Import accepts both header versions and carries no radio
// identity, so nothing upstream catches it), must be REFUSED at plan
// time and told which field is the problem — not written with the
// request silently missing.
func TestDiff_KnownTierFieldOnUnreachableBankIsRefused(t *testing.T) {
	caps := icomTestCapabilities()
	caps.Banks[0].Fields[spec.FieldToneRx] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	// This radio has no filter at all — the bank stops declaring it.
	fields := make(map[spec.Field]spec.FieldSupport, len(caps.Banks[0].Fields))
	for f, fs := range caps.Banks[0].Fields {
		fields[f] = fs
	}
	delete(fields, spec.FieldFilter)
	caps.Banks[0].Fields = fields
	if fs := caps.FieldSupport(spec.BankMemory, spec.FieldFilter); !fs.Unreachable() {
		t.Fatalf("FieldFilter support = %+v, want Unreachable", fs)
	}

	baseline := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{{Slot: "G01-001"}, {Slot: "G01-002"}},
	}
	file := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)}, // Filter is Known here
			{Slot: "G01-002"},
		},
	}

	res, err := Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	e := res.Entries[0]
	if !e.Blocked {
		t.Fatal("Blocked = false, want true: the file asks for a filter this radio has no room for")
	}
	if !strings.Contains(e.BlockReason, "filter not writable on this radio") {
		t.Errorf("BlockReason = %q, want it to name filter", e.BlockReason)
	}
}

// TestDiff_TierFieldGatedThroughCanWrite: every new field goes through
// the SAME CanWrite gate as the existing ten. tone_rx is Unverified in
// the fixture, so a channel carrying it blocks — and names it.
func TestDiff_TierFieldGatedThroughCanWrite(t *testing.T) {
	caps := icomTestCapabilities()
	baseline := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{{Slot: "G01-001"}, {Slot: "G01-002"}},
	}
	file := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{{Slot: "G01-001", Data: icomChannelData(145_500_000)}, {Slot: "G01-002"}},
	}

	res, err := Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	e := res.Entries[0]
	if e.Kind != DiffAdded {
		t.Fatalf("Kind = %q, want %q", e.Kind, DiffAdded)
	}
	if !e.Blocked {
		t.Fatal("Blocked = false, want true: tone_rx is not writable on this radio")
	}
	if !strings.Contains(e.BlockReason, "tone_rx not writable on this radio") {
		t.Errorf("BlockReason = %q, want it to name tone_rx", e.BlockReason)
	}
}

// TestDiff_SparseBankAdmitsAddsOutsideTheBaseline: on a sparse bank the
// file may materialise a slot the baseline never held, anywhere inside
// the addressable space — the whole point of the sparse model.
func TestDiff_SparseBankAdmitsAddsOutsideTheBaseline(t *testing.T) {
	caps := icomTestCapabilities()
	// Everything writable, so the add is not blocked for an unrelated
	// reason.
	caps.Banks[0].Fields[spec.FieldToneRx] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}

	baseline := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{{Slot: "G01-001", Data: icomChannelData(145_500_000)}, {Slot: "G01-002"}},
	}
	file := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)},
			{Slot: "G01-002"},
			{Slot: "G77-042", Data: icomChannelData(430_100_000)},
		},
	}

	res, err := Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if len(res.Entries) != 3 {
		t.Fatalf("Entries = %d, want 3 (the two baseline slots then the add)", len(res.Entries))
	}
	// Determinism: baseline order first, then the file-only slot.
	for i, want := range []string{"G01-001", "G01-002", "G77-042"} {
		if res.Entries[i].Slot != want {
			t.Errorf("Entries[%d].Slot = %q, want %q", i, res.Entries[i].Slot, want)
		}
	}
	add := res.Entries[2]
	if add.Kind != DiffAdded {
		t.Errorf("the added slot's Kind = %q, want %q", add.Kind, DiffAdded)
	}
	if add.Bank != spec.BankMemory {
		t.Errorf("the added slot's Bank = %q, want %q (WithinSpace, not Slots membership)", add.Bank, spec.BankMemory)
	}
	if add.Blocked {
		t.Errorf("the added slot is Blocked (%q), want it to flow", add.BlockReason)
	}
	if res.Added != 1 {
		t.Errorf("Added = %d, want 1", res.Added)
	}
}

// TestDiff_SparseBankRefusesOutOfSpaceAndVanishedSlots: the two ways a
// sparse inventory can still be wrong — an address outside the space,
// and a materialised slot the file simply drops.
func TestDiff_SparseBankRefusesOutOfSpaceAndVanishedSlots(t *testing.T) {
	caps := icomTestCapabilities()
	baseline := &Codeplug{
		Schema: CurrentSchema,
		Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)},
			{Slot: "G01-002"},
		},
	}

	t.Run("an address outside the addressable space", func(t *testing.T) {
		file := &Codeplug{Schema: CurrentSchema, Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)},
			{Slot: "G01-002"},
			{Slot: "G101-001", Data: icomChannelData(430_100_000)},
		}}
		if _, err := Diff(baseline, file, caps); err == nil {
			t.Fatal("Diff() error = nil, want an inventory mismatch for a slot outside Groups x PerGroup")
		}
	})

	t.Run("a materialised slot missing from the file", func(t *testing.T) {
		file := &Codeplug{Schema: CurrentSchema, Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)},
		}}
		if _, err := Diff(baseline, file, caps); err == nil {
			t.Fatal("Diff() error = nil, want an inventory mismatch: an erase must be an empty channel, never an omission")
		}
	})
}

// TestDiff_SparseBudgetEnforcedAtPlanTime: over-budget is refused HERE,
// with a message naming the bank and the overshoot — never discovered by
// sending, since what an over-budget radio does is undocumented.
func TestDiff_SparseBudgetEnforcedAtPlanTime(t *testing.T) {
	caps := icomTestCapabilities() // Budget: 3
	baseline := &Codeplug{Schema: CurrentSchema, Channels: []Channel{
		{Slot: "G01-001"}, {Slot: "G01-002"},
	}}

	atBudget := &Codeplug{Schema: CurrentSchema, Channels: []Channel{
		{Slot: "G01-001", Data: icomChannelData(145_500_000)},
		{Slot: "G01-002", Data: icomChannelData(145_600_000)},
		{Slot: "G02-001", Data: icomChannelData(145_700_000)},
	}}
	if _, err := Diff(baseline, atBudget, caps); err != nil {
		t.Fatalf("Diff() at exactly the budget error = %v, want nil", err)
	}

	overBudget := &Codeplug{Schema: CurrentSchema, Channels: append(
		append([]Channel(nil), atBudget.Channels...),
		Channel{Slot: "G02-002", Data: icomChannelData(145_800_000)},
	)}
	_, err := Diff(baseline, overBudget, caps)
	if err == nil {
		t.Fatal("Diff() over budget error = nil, want a refusal")
	}
	for _, want := range []string{"MEM", "4 populated channels", "limit of 3", "remove 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Diff() error = %q, want it to contain %q", err, want)
		}
	}

	// Empty slots do not count against the budget: the limit is on
	// STORED channels.
	manyEmpty := &Codeplug{Schema: CurrentSchema, Channels: []Channel{
		{Slot: "G01-001"}, {Slot: "G01-002"},
		{Slot: "G02-001"}, {Slot: "G02-002"}, {Slot: "G02-003"},
	}}
	if _, err := Diff(baseline, manyEmpty, caps); err != nil {
		t.Fatalf("Diff() with five empty slots error = %v, want nil", err)
	}
}

func TestDiff_SparseBudgetUnstatedSkipsOnlyBudgetRefusal(t *testing.T) {
	caps := icomTestCapabilities()
	caps.Banks[0].Budget = 0
	caps.Banks[0].BudgetUnstated = true
	baseline := &Codeplug{Schema: CurrentSchema, Channels: []Channel{{Slot: "G01-001"}}}
	file := &Codeplug{Schema: CurrentSchema, Channels: []Channel{
		{Slot: "G01-001", Data: icomChannelData(145_500_000)},
		{Slot: "G01-002", Data: icomChannelData(145_600_000)},
		{Slot: "G02-001", Data: icomChannelData(145_700_000)},
		{Slot: "G02-002", Data: icomChannelData(145_800_000)},
	}}
	if _, err := Diff(baseline, file, caps); err != nil {
		t.Fatalf("Diff() with an unstated budget refused occupancy: %v", err)
	}
	file.Channels = append(file.Channels, Channel{Slot: "G101-001", Data: icomChannelData(430_000_000)})
	if _, err := Diff(baseline, file, caps); err == nil {
		t.Fatal("Diff() admitted an out-of-space slot merely because the occupancy budget was unstated")
	}
}

// TestDiff_DenseInventoryMessageUnchanged: with no sparse bank in caps
// the widened inventory rule reduces to the exact set equality it
// replaced, and reports the identical error a user may already have
// seen.
func TestDiff_DenseInventoryMessageUnchanged(t *testing.T) {
	caps := testCapabilities()
	baseline := testBaselineCodeplug()
	file := testBaselineCodeplug()
	file.Channels = file.Channels[:len(file.Channels)-1]

	_, err := Diff(baseline, file, caps)
	if err == nil {
		t.Fatal("Diff() error = nil, want an inventory mismatch")
	}
	const want = "codeplug: Diff: baseline and file slot inventories differ; the file must descend from a read of this radio's current layout — re-read the radio and try again"
	if err.Error() != want {
		t.Errorf("Diff() error = %q, want %q", err, want)
	}

	// And the other direction: an extra slot no dense bank's baseline
	// held.
	file2 := testBaselineCodeplug()
	file2.Channels = append(file2.Channels, Channel{Slot: "002-EXTRA"})
	if _, err := Diff(baseline, file2, caps); err == nil || err.Error() != want {
		t.Errorf("Diff() with an extra slot error = %v, want %q", err, want)
	}
}

// TestValidate_VocabularyChecksAreCapabilityKeyed: a radio that supplies
// no Yaesu shift/CTCSS-state vocabulary is not judged against one — the
// change design D4 (adjudication 10) required so that an honest Icom
// grading does not report an error on every channel.
func TestValidate_VocabularyChecksAreCapabilityKeyed(t *testing.T) {
	caps := icomTestCapabilities()
	cp := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)},
			{Slot: "G01-002", Data: icomChannelData(145_600_000)},
		},
	}
	issues := Validate(cp, caps)
	for _, i := range issues {
		if i.Field == spec.FieldShift || i.Field == spec.FieldCTCSSState || i.Field == spec.FieldCTCSSTone {
			t.Errorf("Validate reported a Yaesu-vocabulary issue on a radio with no such vocabulary: %+v", i)
		}
	}
	if HasErrors(issues) {
		t.Errorf("Validate() = %+v, want no errors", issues)
	}
}

// TestValidate_YaesuVocabularyStillChecked is the converse pin: a radio
// that DOES supply the vocabulary is judged against it exactly as
// before, on every bank — including one whose Fields map lists only
// FieldFrequency, which is why the check is keyed on the vocabulary and
// not on per-bank field support.
func TestValidate_YaesuVocabularyStillChecked(t *testing.T) {
	caps := testCapabilities()
	cp := testBaselineCodeplug()
	cp.Channels[5].Data.Shift = "SIDEWAYS" // slot "501", the 60M bank
	cp.Channels[5].Data.CTCSS = "MAYBE"

	var sawShift, sawCTCSS bool
	for _, i := range Validate(cp, caps) {
		if i.Slot != "501" {
			continue
		}
		switch i.Field {
		case spec.FieldShift:
			sawShift = true
		case spec.FieldCTCSSState:
			sawCTCSS = true
		}
	}
	if !sawShift || !sawCTCSS {
		t.Errorf("Validate() on a bank listing only FieldFrequency: shift issue = %v, ctcss issue = %v, want both", sawShift, sawCTCSS)
	}
}

// TestValidate_TierFieldsJudgedOnlyWhenReachable: the tier's fields are
// checked against the radio's own vocabularies when the bank reaches
// them, and not judged at all when it does not — which is what keeps
// every pre-tier channel (all ten Absent) unaffected.
func TestValidate_TierFieldsJudgedOnlyWhenReachable(t *testing.T) {
	t.Run("a Yaesu codeplug is untouched by the tier checks", func(t *testing.T) {
		issues := Validate(testBaselineCodeplug(), testCapabilities())
		if HasErrors(issues) {
			t.Errorf("Validate() = %+v, want no errors", issues)
		}
	})

	t.Run("a value outside the radio's vocabulary is an error", func(t *testing.T) {
		caps := icomTestCapabilities()
		cp := &Codeplug{
			Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
			Channels: []Channel{
				{Slot: "G01-001", Data: icomChannelData(145_500_000)},
				{Slot: "G01-002", Data: icomChannelData(145_600_000)},
			},
		}
		cp.Channels[0].Data.Duplex = StringField{State: Known, Value: "DUPLEX-SIDEWAYS"}
		cp.Channels[0].Data.Filter = StringField{State: Known, Value: "FIL9"}

		var sawDuplex, sawFilter bool
		for _, i := range Validate(cp, caps) {
			switch i.Field {
			case spec.FieldDuplex:
				sawDuplex = true
			case spec.FieldFilter:
				sawFilter = true
			}
		}
		if !sawDuplex || !sawFilter {
			t.Errorf("Validate(): duplex issue = %v, filter issue = %v, want both", sawDuplex, sawFilter)
		}
	})

	t.Run("a reachable field the channel says nothing about is an error", func(t *testing.T) {
		caps := icomTestCapabilities()
		cp := &Codeplug{
			Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
			Channels: []Channel{
				{Slot: "G01-001", Data: icomChannelData(145_500_000)},
				{Slot: "G01-002", Data: icomChannelData(145_600_000)},
			},
		}
		cp.Channels[0].Data.ToneMode = StringField{} // Absent
		found := false
		for _, i := range Validate(cp, caps) {
			if i.Field == spec.FieldToneMode && strings.Contains(i.Msg, "says nothing about it") {
				found = true
			}
		}
		if !found {
			t.Error("Validate() did not report an Absent tone_mode on a radio that has the field")
		}
	})
}

// TestValidate_TagCharsetIsCapabilitySupplied: the default rule is the
// one this package always applied (and its message is unchanged), while
// a radio supplying a charset is judged against exactly that — including
// a charset that omits the space, which the Icom tables do.
func TestValidate_TagCharsetIsCapabilitySupplied(t *testing.T) {
	t.Run("default rule and message", func(t *testing.T) {
		caps := testCapabilities()
		cp := testBaselineCodeplug()
		cp.Channels[0].Data.Tag = "BAD;TAG"
		found := false
		for _, i := range Validate(cp, caps) {
			if i.Field == spec.FieldTag {
				found = true
				const want = `slot "001": tag contains an invalid byte (must be printable ASCII 0x20-0x7E, excluding ';')`
				if i.Msg != want {
					t.Errorf("Msg = %q, want %q", i.Msg, want)
				}
			}
		}
		if !found {
			t.Error("Validate() did not reject a ';' in a tag")
		}
	})

	t.Run("a supplied charset that omits the space", func(t *testing.T) {
		caps := icomTestCapabilities()
		caps.TagCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		cp := &Codeplug{
			Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
			Channels: []Channel{
				{Slot: "G01-001", Data: icomChannelData(145_500_000)},
				{Slot: "G01-002", Data: icomChannelData(145_600_000)},
			},
		}
		cp.Channels[0].Data.Tag = "GB3 TEST" // legal by the default rule, not by this one
		found := false
		for _, i := range Validate(cp, caps) {
			if i.Field == spec.FieldTag {
				found = true
				if !strings.Contains(i.Msg, "ABCDEFGHIJ") {
					t.Errorf("Msg = %q, want it to name the supplied charset", i.Msg)
				}
			}
		}
		if !found {
			t.Error("Validate() accepted a space against a charset that omits it")
		}
	})
}

// TestValidate_FrequencyBoundsReachPastUint32: the widened frequency is
// checked against the widened capability bounds, so a 10 GHz channel on
// a 10.5 GHz radio is fine and a 12 GHz one is not — neither question
// was expressible before the tier.
func TestValidate_FrequencyBoundsReachPastUint32(t *testing.T) {
	caps := icomTestCapabilities()
	cp := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(10_000_000_000)},
			{Slot: "G01-002", Data: icomChannelData(145_600_000)},
		},
	}
	if HasErrors(Validate(cp, caps)) {
		t.Errorf("Validate() reported errors for a 10 GHz channel on a 10.5 GHz radio: %+v", Validate(cp, caps))
	}

	cp.Channels[0].Data.FreqHz = 12_000_000_000
	found := false
	for _, i := range Validate(cp, caps) {
		if i.Field == spec.FieldFrequency && strings.Contains(i.Msg, "above this radio's maximum") {
			found = true
		}
	}
	if !found {
		t.Error("Validate() accepted a 12 GHz channel on a 10.5 GHz radio")
	}
}

// TestUnconditionallyAdded_IsExactlyTheAlwaysTransmittedSix pins the set
// touchedFields applies the Unreachable filter to. It is derived from
// addedFields rather than restated, so this test is a statement of WHICH
// SIX, not a duplicate of the derivation: the six fields a write frame
// carries whatever the channel says, and therefore the only six a bank
// may legitimately be missing without anybody having asked for them.
//
// Membership matters in both directions. A field wrongly IN this set is
// one whose Known value can be dropped instead of refused (Wave-1c
// review 1, finding 1); a field wrongly OUT of it blocks every channel
// on a bank that cannot express it, over a value nobody chose.
func TestUnconditionallyAdded_IsExactlyTheAlwaysTransmittedSix(t *testing.T) {
	want := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
	}
	if !reflect.DeepEqual(unconditionallyAdded, want) {
		t.Errorf("unconditionallyAdded = %v, want %v", unconditionallyAdded, want)
	}
	for _, f := range []spec.Field{spec.FieldCTCSSTone, spec.FieldTagDisplay, spec.FieldScanSkip} {
		if unconditionallyAdded[f] {
			t.Errorf("%s is in unconditionallyAdded: it is Known-conditional, so its presence IS a request and must never be filtered away", f)
		}
	}
}

// TestDiff_DuplicateSlotRefused restores and pins a refusal the tier's
// rewrite of checkInventory lost (Wave-1c review 1, finding 3): the rule
// used to compare two SORTED SLOT LISTS, so a file that repeated a slot
// was longer than the baseline and was refused; the set-based rewrite
// could not see the repeat, and fileBySlot then silently kept the LAST
// occurrence and diffed against that one.
//
// It matters because `rigprog diff` calls codeplug.Diff directly, with no
// Validate pass in front of it (the send path has one, and Validate does
// report a duplicate slot as an error). Without this, a hand-edited file
// with a repeated slot quietly reported one of the two duplicates'
// changes instead of telling the user the file was malformed.
func TestDiff_DuplicateSlotRefused(t *testing.T) {
	caps := testCapabilities()
	const want = "codeplug: Diff: baseline and file slot inventories differ; the file must descend from a read of this radio's current layout — re-read the radio and try again"

	t.Run("a slot repeated in the file", func(t *testing.T) {
		baseline := testBaselineCodeplug()
		file := testBaselineCodeplug()
		dup := file.Channels[0]
		dup.Data = copyChannelData(file.Channels[0].Data)
		dup.Data.FreqHz = 51_000_000
		file.Channels = append([]Channel{file.Channels[0], dup}, file.Channels[1:]...)

		_, err := Diff(baseline, file, caps)
		if err == nil {
			t.Fatal("Diff() error = nil, want a refusal: the file names one slot twice")
		}
		if err.Error() != want {
			t.Errorf("Diff() error = %q, want %q", err, want)
		}
	})

	t.Run("a slot repeated in the baseline", func(t *testing.T) {
		baseline := testBaselineCodeplug()
		baseline.Channels = append([]Channel{baseline.Channels[0]}, baseline.Channels...)
		file := testBaselineCodeplug()

		if _, err := Diff(baseline, file, caps); err == nil {
			t.Fatal("Diff() error = nil, want a refusal: a baseline naming one slot twice is not a read of any radio")
		}
	})

	t.Run("a slot merely missing is still refused", func(t *testing.T) {
		baseline := testBaselineCodeplug()
		file := testBaselineCodeplug()
		file.Channels = file.Channels[:len(file.Channels)-1]

		_, err := Diff(baseline, file, caps)
		if err == nil {
			t.Fatal("Diff() error = nil, want an inventory mismatch")
		}
		if err.Error() != want {
			t.Errorf("Diff() error = %q, want %q", err, want)
		}
	})
}

// v4YaesuBodyNoTierKeys is a schema-4 file for a Yaesu-shaped radio whose
// ten tier-added keys are simply OMITTED from the JSON — the file shape
// deviation (c) is about. It is deliberately hand-written rather than
// produced by Save: schemaFor picks schema 3 when every one of the ten
// is Unavailable (RepresentableByOmission), and schema 4 with explicit
// "state":"" keys when any is Absent — Save never emits a v4 file with
// the keys themselves omitted, so this shape has no producer to pin
// against; the case that was never in doubt, since the schema-3 loader
// migrates all ten to Unavailable unconditionally.
const v4YaesuBodyNoTierKeys = `{"schema":4,"generator":"test","radio":{"model":"TEST-710","cat_id":"0000","read_at":"2026-08-27T09:00:00Z"},"channels":[` +
	`{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","tag":"CALLING","tag_display":{"state":"known"},"scan_skip":{"state":"known"}}},` +
	`{"slot":"003"}]}`

// tierFieldStates returns d's seventeen tier-added field states in
// ChannelData's declaration order, for asserting on the set as a whole
// rather than field by field.
func tierFieldStates(d *ChannelData) []FieldState {
	return []FieldState{
		d.TxFreqHz.State, d.Duplex.State, d.OffsetHz.State, d.ToneMode.State,
		d.ToneTx.State, d.ToneRx.State, d.DTCSCode.State, d.DTCSPolarity.State,
		d.Filter.State, d.DataMode.State,
		d.TuningStepEnabled.State, d.TuningStep.State,
		d.ProgramTuningStepHz.State, d.AttenuatorDB.State,
		d.Preamp.State, d.Antenna.State, d.IPPlus.State,
	}
}

// allTierStates returns the seventeen-state slice tierFieldStates returns
// for a channel every one of whose tier fields is in state s.
func allTierStates(s FieldState) []FieldState {
	out := make([]FieldState, len(tierFieldNormalisers))
	for i := range out {
		out[i] = s
	}
	return out
}

// TestTierFieldNormalisers_AgreeWithTheLegacyMigration pins
// NormaliseTierFields' table against withUnavailableTierFields, the
// legacy migrations' own list: apply every row's "make it Unavailable" to
// a zero ChannelData and the result must be EXACTLY what a schema-1/2/3
// channel migrates to. A row left out, or one wired to the wrong struct
// field, fails here — where the cause is obvious — rather than as a
// single silently un-normalised field in some later test.
//
// The Absent half is pinned in the same breath: on the zero ChannelData
// every row must report Absent, since the zero FieldState IS Absent.
func TestTierFieldNormalisers_AgreeWithTheLegacyMigration(t *testing.T) {
	if len(tierFieldNormalisers) != 17 {
		t.Fatalf("tierFieldNormalisers has %d rows, want 17 — D4 added ten fields and D8 added seven", len(tierFieldNormalisers))
	}

	zero := &ChannelData{}
	for _, n := range tierFieldNormalisers {
		if !n.absent(zero) {
			t.Errorf("%s: absent(zero ChannelData) = false, want true — the zero FieldState is Absent", n.field)
		}
	}

	got := &ChannelData{}
	for _, n := range tierFieldNormalisers {
		n.unavailable(got)
	}
	want := withUnavailableTierFields(&ChannelData{})
	if *got != *want {
		t.Errorf("applying every normaliser to a zero ChannelData gave\n %+v\nwant (withUnavailableTierFields)\n %+v", *got, *want)
	}

	seen := make(map[spec.Field]bool, len(tierFieldNormalisers))
	for _, n := range tierFieldNormalisers {
		if seen[n.field] {
			t.Errorf("field %s appears twice in tierFieldNormalisers", n.field)
		}
		seen[n.field] = true
	}

	// And the same seventeen fields, in the same order, as diff.go's
	// tierAddedFieldFor — the OTHER table in this package that walks the
	// two generations. Both are ChannelData's declaration order because that is
	// the order Validate documents and the grid renders; two tables that
	// silently disagreed about which field is in which position would put
	// this pass and the send plan on different footing.
	wantOrder := make([]spec.Field, 0, len(tierAddedFieldFor))
	for _, f := range tierAddedFieldFor {
		wantOrder = append(wantOrder, f.field)
	}
	gotOrder := make([]spec.Field, 0, len(tierFieldNormalisers))
	for _, n := range tierFieldNormalisers {
		gotOrder = append(gotOrder, n.field)
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Errorf("tierFieldNormalisers names\n %v\nbut diff.go's tierAddedFieldFor names\n %v\n(the two must be the same seventeen fields in the same order)", gotOrder, wantOrder)
	}
}

// tierFieldToAbsent makes exactly ONE tier field Absent, in
// tierFieldNormalisers' own order — the inverse of that table's
// "unavailable" column, written out by hand precisely so it is
// INDEPENDENT of the table under test. A per-row check built from the
// table's own accessors could not catch a row wired to the wrong struct
// field, since both halves would be wrong together.
var tierFieldToAbsent = []func(*ChannelData){
	func(d *ChannelData) { d.TxFreqHz = FreqField{} },
	func(d *ChannelData) { d.Duplex = StringField{} },
	func(d *ChannelData) { d.OffsetHz = FreqField{} },
	func(d *ChannelData) { d.ToneMode = StringField{} },
	func(d *ChannelData) { d.ToneTx = ToneField{} },
	func(d *ChannelData) { d.ToneRx = ToneField{} },
	func(d *ChannelData) { d.DTCSCode = IntField{} },
	func(d *ChannelData) { d.DTCSPolarity = StringField{} },
	func(d *ChannelData) { d.Filter = StringField{} },
	func(d *ChannelData) { d.DataMode = BoolField{} },
	func(d *ChannelData) { d.TuningStepEnabled = BoolField{} },
	func(d *ChannelData) { d.TuningStep = StringField{} },
	func(d *ChannelData) { d.ProgramTuningStepHz = FreqField{} },
	func(d *ChannelData) { d.AttenuatorDB = IntField{} },
	func(d *ChannelData) { d.Preamp = StringField{} },
	func(d *ChannelData) { d.Antenna = StringField{} },
	func(d *ChannelData) { d.IPPlus = BoolField{} },
}

// TestTierFieldNormalisers_EachRowIsWiredToItsOwnField pins every row of
// the table to its OWN struct field, both halves separately — which is
// what catches a SWAPPED PAIR, the one mistake the two checks above
// cannot see between them. A row that reads and writes some other tier
// field consistently still satisfies
// TestTierFieldNormalisers_AgreeWithTheLegacyMigration (all seventeen fields
// still end Unavailable) and still satisfies the order check (the
// spec.Field labels are untouched) — and would normalise the wrong field
// on any bank that reaches one and not the other, which is every real
// Icom bank.
//
// Row i, applied to a zero ChannelData, must make position i and no
// other Unavailable; and row i's Absent test, on a channel where only
// position i is Absent, must be the only one that fires.
func TestTierFieldNormalisers_EachRowIsWiredToItsOwnField(t *testing.T) {
	if len(tierFieldToAbsent) != len(tierFieldNormalisers) {
		t.Fatalf("tierFieldToAbsent has %d entries, tierFieldNormalisers %d — the two walk the same seventeen fields in the same order", len(tierFieldToAbsent), len(tierFieldNormalisers))
	}

	for i, n := range tierFieldNormalisers {
		t.Run(string(n.field), func(t *testing.T) {
			// The assignment half: exactly position i changes.
			d := &ChannelData{}
			n.unavailable(d)
			want := allTierStates(Absent)
			want[i] = Unavailable
			if got := tierFieldStates(d); !reflect.DeepEqual(got, want) {
				t.Errorf("row %d (%s) sets\n %v\nwant\n %v — this row is wired to the wrong struct field", i, n.field, got, want)
			}

			// The Absent half: on a channel where only position i is
			// Absent, only row i reports Absent.
			base := withUnavailableTierFields(&ChannelData{})
			tierFieldToAbsent[i](base)
			for j, other := range tierFieldNormalisers {
				if got := other.absent(base); got != (i == j) {
					t.Errorf("with only %s Absent, row %d (%s) reports absent = %v, want %v", n.field, j, other.field, got, i == j)
				}
			}
		})
	}
}

// TestNormaliseTierFields_V4YaesuFileWithNoTierKeys is deviation (c)'s
// headline case: a schema-4 file from a radio that has none of D4's ten
// fields loads those fields Absent — Load alone cannot know better — while
// D8's seven fields migrate to Unavailable because schema 4 could not
// express them. The composition roots' capability-keyed pass resolves the
// remaining ten to Unavailable, the same answer the schema-1/2/3 loaders
// reach unconditionally and the same answer a read of that radio gives.
func TestNormaliseTierFields_V4YaesuFileWithNoTierKeys(t *testing.T) {
	cp, err := writeAndLoad(t, v4YaesuBodyNoTierKeys)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cp.Schema != CurrentSchema {
		t.Fatalf("Schema = %d, want %d", cp.Schema, CurrentSchema)
	}
	if len(cp.Channels) != 2 || cp.Channels[0].Data == nil {
		t.Fatalf("channels = %+v, want a populated 001 and an empty 003", cp.Channels)
	}

	wantLoaded := allTierStates(Absent)
	for i := 10; i < len(wantLoaded); i++ {
		wantLoaded[i] = Unavailable
	}
	if got := tierFieldStates(cp.Channels[0].Data); !reflect.DeepEqual(got, wantLoaded) {
		t.Fatalf("straight off Load, tier states = %v, want D4 Absent and D8 Unavailable", got)
	}

	NormaliseTierFields(cp, testCapabilities())

	if got, want := tierFieldStates(cp.Channels[0].Data), allTierStates(Unavailable); !reflect.DeepEqual(got, want) {
		t.Errorf("after NormaliseTierFields, tier states = %v, want all Unavailable", got)
	}
	if cp.Channels[1].Data != nil {
		t.Errorf("channel 003 = %+v, want still empty — an empty slot has no fields to normalise", cp.Channels[1])
	}

	// The point of the exercise: such a channel now compares EQUAL, field
	// for field, to the same channel arriving from a legacy file, which is
	// what keeps Diff from reporting it as modified.
	legacy := withUnavailableTierFields(&ChannelData{})
	if got := tierFieldStates(cp.Channels[0].Data); !reflect.DeepEqual(got, tierFieldStates(legacy)) {
		t.Errorf("normalised v4 tier states = %v, legacy-migrated = %v — the two must agree", got, tierFieldStates(legacy))
	}
}

// TestNormaliseTierFields_ReachableAbsentIsLeftAlone: on a radio that DOES
// have a field, Absent means "nobody has said anything about this yet"
// and is NOT rewritten — not to Unavailable (a claim about the radio that
// would be false), and not to any value at all.
//
// It also PINS what the rest of the system then does with such a channel,
// which is the reason leaving it alone is safe (Wave 4 task R2's ruling):
// Validate REFUSES it with an error naming the field, and a write never
// transmits it — touchedFields counts a tier field only when it is Known,
// so there is no request to send and nothing to invent.
func TestNormaliseTierFields_ReachableAbsentIsLeftAlone(t *testing.T) {
	caps := icomTestCapabilities()
	cp := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{
			{Slot: "G01-001", Data: icomChannelData(145_500_000)},
			{Slot: "G01-002", Data: icomChannelData(145_600_000)},
		},
	}
	// Every tier field of this bank is reachable, so an Absent one is the
	// GUI's bankless-add shape on a radio that really has the field.
	cp.Channels[0].Data.ToneMode = StringField{}
	cp.Channels[0].Data.Filter = StringField{}

	NormaliseTierFields(cp, caps)

	if got := cp.Channels[0].Data.ToneMode.State; got != Absent {
		t.Errorf("tone_mode state = %q, want Absent — this radio HAS the field, so silence is not a statement that it lacks one", got)
	}
	if got := cp.Channels[0].Data.Filter.State; got != Absent {
		t.Errorf("filter state = %q, want Absent", got)
	}
	if !reflect.DeepEqual(cp.Channels[1].Data, icomChannelData(145_600_000)) {
		t.Errorf("the untouched channel changed: %+v", cp.Channels[1].Data)
	}

	t.Run("Validate refuses it", func(t *testing.T) {
		var sawToneMode, sawFilter bool
		for _, i := range Validate(cp, caps) {
			if i.Severity != SeverityError || !strings.Contains(i.Msg, "says nothing about it") {
				continue
			}
			switch i.Field {
			case spec.FieldToneMode:
				sawToneMode = true
			case spec.FieldFilter:
				sawFilter = true
			}
		}
		if !sawToneMode || !sawFilter {
			t.Errorf("Validate(): tone_mode issue = %v, filter issue = %v, want both as errors", sawToneMode, sawFilter)
		}
	})

	t.Run("a write neither transmits nor invents it", func(t *testing.T) {
		touched := touchedFields(caps, spec.BankMemory, *cp.Channels[0].Data)
		for _, f := range touched {
			if f == spec.FieldToneMode || f == spec.FieldFilter {
				t.Errorf("touchedFields includes %s for an Absent field: %v", f, touched)
			}
		}
	})
}

// TestNormaliseTierFields_MixedReachability walks the rule field by field
// on ONE bank that reaches some of the ten and not others — the shape a
// real Icom bank has (the registered IC-7610 reaches four of the ten) —
// and pins that no state other than Absent is ever touched.
func TestNormaliseTierFields_MixedReachability(t *testing.T) {
	caps := icomTestCapabilities()
	fields := make(map[spec.Field]spec.FieldSupport, len(caps.Banks[0].Fields))
	for f, fs := range caps.Banks[0].Fields {
		fields[f] = fs
	}
	// The six this bank cannot reach at all, in the shape a driver
	// declares: no entry rather than a zero one, since caps that say
	// nothing about a field are not evidence that the radio has one.
	for _, f := range []spec.Field{
		spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
		spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldDataMode,
	} {
		delete(fields, f)
	}
	caps.Banks[0].Fields = fields

	d := &ChannelData{
		FreqHz: 145_500_000, Mode: "FM",
		// Reachable, and each in a state the pass must not touch.
		ToneMode: StringField{State: Known, Value: "TSQL"},
		ToneTx:   ToneField{State: Unknown},
		ToneRx:   ToneField{State: Unavailable},
		// Reachable and Absent: stays Absent.
		Filter: StringField{},
		// The six unreachable ones are all Absent (the zero value).
	}
	cp := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{{Slot: "G01-001", Data: d}},
	}

	NormaliseTierFields(cp, caps)

	want := []FieldState{
		Unavailable, Unavailable, Unavailable, // tx_frequency, duplex, offset
		Known, Unknown, Unavailable, // tone_mode, tone_tx, tone_rx
		Unavailable, Unavailable, // dtcs_code, dtcs_polarity
		Absent,      // filter — reachable, so silence stays silence
		Unavailable, // data_mode
		Unavailable, Unavailable, Unavailable, Unavailable,
		Unavailable, Unavailable, Unavailable, // the seven D8 fields
	}
	if got := tierFieldStates(d); !reflect.DeepEqual(got, want) {
		t.Errorf("tier states = %v, want %v", got, want)
	}
	if d.ToneMode.Value != "TSQL" {
		t.Errorf("tone_mode value = %q, want TSQL untouched", d.ToneMode.Value)
	}

	t.Run("idempotent", func(t *testing.T) {
		again := *d
		NormaliseTierFields(cp, caps)
		if *d != again {
			t.Errorf("a second pass changed the channel:\n got %+v\nwant %+v", *d, again)
		}
	})

	// A KNOWN value on a field this bank cannot reach: the pass is keyed
	// on Absent, not on reachability, so it must leave such a field
	// exactly where it found it — value and all.
	//
	// Erasing it would be the Wave-1c review's finding 1 all over again,
	// one layer earlier: a Known value is the user's REQUEST, and this
	// project refuses a request it cannot honour rather than dropping it.
	// The refusal is the send gate's — touchedFields keeps an unreachable
	// Known field in the touched set precisely so the per-field gate
	// blocks the channel by name ("not writable on this radio") — and a
	// loader that had quietly rewritten the value to Unavailable would
	// have destroyed the request before the gate ever saw it, turning a
	// named refusal into a silent no-op.
	t.Run("a Known value on an unreachable field is left for the gates to refuse", func(t *testing.T) {
		unreachable := &ChannelData{
			FreqHz: 145_500_000, Mode: "FM",
			Duplex:   StringField{State: Known, Value: "DUP-"}, // unreachable on this bank
			DTCSCode: IntField{State: Known, Value: 23},        // unreachable too
			DataMode: BoolField{State: Known, Value: true},     // and this one
		}
		local := &Codeplug{
			Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
			Channels: []Channel{{Slot: "G01-001", Data: unreachable}},
		}
		NormaliseTierFields(local, caps)

		// The three Known fields specifically: the channel's OTHER seven
		// tier fields are Absent and are expected to move, so comparing
		// the whole struct would assert the opposite of this test's point.
		if got := unreachable.Duplex; got != (StringField{State: Known, Value: "DUP-"}) {
			t.Errorf("duplex = %+v, want the Known DUP- it arrived with", got)
		}
		if got := unreachable.DTCSCode; got != (IntField{State: Known, Value: 23}) {
			t.Errorf("dtcs_code = %+v, want the Known 23 it arrived with", got)
		}
		if got := unreachable.DataMode; got != (BoolField{State: Known, Value: true}) {
			t.Errorf("data_mode = %+v, want the Known true it arrived with", got)
		}

		touched := make(map[spec.Field]bool)
		for _, f := range touchedFields(caps, spec.BankMemory, *unreachable) {
			touched[f] = true
		}
		for _, f := range []spec.Field{spec.FieldDuplex, spec.FieldDTCSCode, spec.FieldDataMode} {
			if !touched[f] {
				t.Errorf("touchedFields omits the unreachable Known %s — the send gate can only refuse a request it is shown", f)
			}
		}
	})
}

// TestNormaliseTierFields_SlotInNoBankAtAll: a slot no bank claims is
// judged by the same answer Validate gives it — the zero BankID, which
// reaches nothing — so its tier fields normalise to Unavailable. One
// rule, not two: bankForSlot is asked once and its verdict is used
// exactly as validateTierFields uses it.
func TestNormaliseTierFields_SlotInNoBankAtAll(t *testing.T) {
	cp := &Codeplug{
		Schema: CurrentSchema, Radio: RadioInfo{Model: "TEST-ICOM", CATID: "A4"},
		Channels: []Channel{{Slot: "NO-SUCH-SLOT", Data: &ChannelData{FreqHz: 145_500_000, Mode: "FM"}}},
	}
	NormaliseTierFields(cp, icomTestCapabilities())
	if got, want := tierFieldStates(cp.Channels[0].Data), allTierStates(Unavailable); !reflect.DeepEqual(got, want) {
		t.Errorf("tier states = %v, want all Unavailable", got)
	}
}

// TestNormaliseTierFields_LegacyLoadIsUntouched: the pass is a NO-OP on
// anything loadV1/loadV2/loadV3 produced, because those loaders leave no
// Absent tier field for it to resolve. This is what makes it safe to run
// on every load whatever the file's schema was — and it is asserted
// against the most capable capabilities in this package's fixtures, the
// one that reaches all ten fields, since a pass that DID touch a legacy
// channel would touch it hardest there.
func TestNormaliseTierFields_LegacyLoadIsUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	legacy := testBaselineCodeplug()
	NormaliseTierFields(legacy, testCapabilities())
	if err := Save(path, legacy); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	cp, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	before := make([]ChannelData, 0, len(cp.Channels))
	for _, ch := range cp.Channels {
		if ch.Data != nil {
			before = append(before, *ch.Data)
		}
	}
	if len(before) == 0 {
		t.Fatal("the legacy fixture loaded no populated channel — nothing asserted")
	}

	NormaliseTierFields(cp, icomTestCapabilities())

	i := 0
	for _, ch := range cp.Channels {
		if ch.Data == nil {
			continue
		}
		if *ch.Data != before[i] {
			t.Errorf("slot %s changed:\n got %+v\nwant %+v", ch.Slot, *ch.Data, before[i])
		}
		i++
	}
}

// TestNormaliseTierFields_NilCodeplug: a nil *Codeplug is a no-op, not a
// panic — the pass sits on a load path whose error handling belongs to
// its callers.
func TestNormaliseTierFields_NilCodeplug(t *testing.T) {
	NormaliseTierFields(nil, icomTestCapabilities())
}
