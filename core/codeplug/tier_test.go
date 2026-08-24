// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
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
			Sparse: true, Groups: 100, PerGroup: 100, Budget: 3,
			Fields: fields,
		}},
	}
}

// icomChannelData is a fully-specified channel for icomTestCapabilities.
func icomChannelData(freqHz uint64) *ChannelData {
	return &ChannelData{
		FreqHz: freqHz, Mode: "FM", Tag: "REPEATER",
		CTCSSTone:    ToneField{State: Unavailable},
		TagDisplay:   BoolField{State: Unavailable},
		ScanSkip:     BoolField{State: Unavailable},
		TxFreqHz:     FreqField{State: Unknown},
		Duplex:       StringField{State: Known, Value: "DUP-"},
		OffsetHz:     FreqField{State: Known, Value: 600_000},
		ToneMode:     StringField{State: Known, Value: "TSQL"},
		ToneTx:       ToneField{State: Known, Value: 885},
		ToneRx:       ToneField{State: Known, Value: 885},
		DTCSCode:     IntField{State: Unavailable},
		DTCSPolarity: StringField{State: Unavailable},
		Filter:       StringField{State: Known, Value: "FIL2"},
		DataMode:     BoolField{State: Known, Value: false},
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
