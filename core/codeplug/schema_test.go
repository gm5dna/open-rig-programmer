// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// canonicalV3Goldens are the schema-3 files in testdata. Every one of
// them was produced by the writer AS IT STOOD AT 074ed97 — the commit
// this tier branched from, before CurrentSchema moved to 4 — by running
// that build's own Save over hand-built codeplugs. They are therefore
// evidence of what the pre-tier writer emitted, not a re-statement of
// what the current writer happens to emit.
//
// Between them they cover what the pin has to cover: every FieldState on
// both tri-state fields, an omitempty field present and absent (clar_hz,
// tag), a negative clarifier, HTML-escapable bytes in a tag and inside a
// verbatim-preserved legacy menus blob (which must NOT be escaped — see
// Save), a uint32-maximum frequency, an empty channel, a typed menu
// snapshot, and a nil channel list (which must stay JSON null, not []).
var canonicalV3Goldens = []string{
	"canonical-v3-basic.json",
	"canonical-v3-menus.json",
	"canonical-v3-empty.json",
}

var canonicalV4Goldens = []string{
	"canonical-v4-basic.json",
}

// canonicalV5Goldens are the schema-5 files in testdata, mirroring
// canonicalV4Goldens' construction: produced by this package's own Save
// over a hand-built codeplug, then frozen.
//
// Their PROVENANCE differs from the schema-3 goldens' and the difference
// is worth stating. A schema-3 golden had to come from the pre-tier
// build, because its whole job is to prove the CURRENT writer still
// emits what an OLDER one did. Schema 5 has no older writer: it is what
// this build introduced, so the pin here is against DRIFT — a changed
// key order, a lost omitempty, a marshal type quietly swapped for the
// live struct — rather than against a predecessor.
//
// The one fixture covers what schema 5 alone can carry: all SEVEN
// receiver fields (additions design D8) Recorded, each with a value
// inside the IC-R8600's own declared domain (core/driver/icr8600/caps.go
// — "12.5 kHz" from TuningSteps, 500 Hz on the 100 Hz programmable
// grid, 20 dB from AttenuatorDB, "ON" from PreampOptions, "ANT2" from
// AntennaOptions), on that receiver's zero-based sparse slot form. Its
// two transmit fields are Unavailable because the IC-R8600 is
// spec.ReceiveOnly and has no transmitter to describe — the honest
// state, not a convenient one.
var canonicalV5Goldens = []string{
	"canonical-v5-basic.json",
}

// TestSaveLoad_CanonicalV3ByteIdentical is the FIRST of design D4's two
// pinned tests (adjudication 4; round 2 F6+C7): save(load(f)) is
// byte-identical for every canonical writer-produced schema-3 file.
//
// It is what makes "every existing Yaesu codeplug and manifest artefact
// is byte-identical by construction" checkable rather than merely
// asserted. If the lowest-schema writer, the frozen schema-3 marshal
// type, its key order, or its migration ever drifts, a byte of one of
// these files changes and this fails.
//
// Scope, stated because it is deliberately narrow: the pin is on
// CANONICAL files — ones this package's own writer produced. Arbitrary
// hand-formatted JSON is normalised by Load (indentation, key order, an
// omitted optional key) and is explicitly out of scope.
func TestSaveLoad_CanonicalV3ByteIdentical(t *testing.T) {
	for _, name := range canonicalV3Goldens {
		t.Run(name, func(t *testing.T) {
			src := filepath.Join("testdata", name)
			want, err := os.ReadFile(src)
			if err != nil {
				t.Fatalf("reading golden: %v", err)
			}

			cp, err := Load(src)
			if err != nil {
				t.Fatalf("Load(%s) error = %v", src, err)
			}
			// Load migrates every file to the current schema in memory;
			// the writer's schema choice is made from the CONTENT, not
			// from this value, which is the whole point.
			if cp.Schema != CurrentSchema {
				t.Fatalf("Load(%s).Schema = %d, want %d (migrate-on-load)", src, cp.Schema, CurrentSchema)
			}

			dst := filepath.Join(t.TempDir(), name)
			if err := Save(dst, cp); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("reading re-saved file: %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("save(load(%s)) is not byte-identical.\n--- want (%d bytes) ---\n%s\n--- got (%d bytes) ---\n%s",
					name, len(want), want, len(got), got)
			}
		})
	}
}

func TestSaveLoad_CanonicalV4ByteIdentical(t *testing.T) {
	for _, name := range canonicalV4Goldens {
		t.Run(name, func(t *testing.T) {
			src := filepath.Join("testdata", name)
			want, err := os.ReadFile(src)
			if err != nil {
				t.Fatalf("reading golden: %v", err)
			}
			cp, err := Load(src)
			if err != nil {
				t.Fatalf("Load(%s) error = %v", src, err)
			}
			dst := filepath.Join(t.TempDir(), name)
			if err := Save(dst, cp); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("reading re-saved file: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("save(load(%s)) is not byte-identical.\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
			}
		})
	}
}

// TestSaveLoad_CanonicalV5ByteIdentical is the schema-5 arm of the
// byte-identity pin the schema-3 and schema-4 goldens already have.
// Until it existed, schema 5 was the ONE version this package could both
// read and write with nothing checking that the two agreed byte for
// byte: a reordered ChannelData field or a saveValue branch quietly
// rerouted would both now fail here. It does NOT cover a state key
// gaining omitempty: every field in the fixture is Known or Unavailable,
// so no field ever renders as the empty Absent state
// (`{"state": ""}`), and omitempty changes no byte when the value is
// never empty. The on-disk rendering of Absent is not pinned by any
// test in this package.
func TestSaveLoad_CanonicalV5ByteIdentical(t *testing.T) {
	for _, name := range canonicalV5Goldens {
		t.Run(name, func(t *testing.T) {
			src := filepath.Join("testdata", name)
			want, err := os.ReadFile(src)
			if err != nil {
				t.Fatalf("reading golden: %v", err)
			}
			cp, err := Load(src)
			if err != nil {
				t.Fatalf("Load(%s) error = %v", src, err)
			}
			if cp.Schema != CurrentSchema {
				t.Fatalf("Load(%s).Schema = %d, want %d (migrate-on-load)", src, cp.Schema, CurrentSchema)
			}
			dst := filepath.Join(t.TempDir(), name)
			if err := Save(dst, cp); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("reading re-saved file: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("save(load(%s)) is not byte-identical.\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
			}
		})
	}
}

// TestCanonicalV5Golden_RecordsEveryReceiverField holds the golden to
// the job it was added for: a receiver field left out of the fixture
// would leave that field's on-disk rendering unpinned while the
// byte-identity test above still passed.
func TestCanonicalV5Golden_RecordsEveryReceiverField(t *testing.T) {
	cp, err := Load(filepath.Join("testdata", "canonical-v5-basic.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	d := cp.Channels[0].Data
	for name, state := range receiverFieldStates(d) {
		if state != Known {
			t.Errorf("golden receiver field %s = %q, want Known: the fixture must exercise every schema-5 field", name, state)
		}
	}
}

// TestSchemaFor_NoV3RepresentableContentEverWritesV4 is the SECOND of
// design D4's two pinned tests: no schema-3-representable content ever
// produces a schema-4 file.
//
// The two clauses of the rule are exercised from both sides — a
// tier-added field at each state, and a frequency either side of
// schema 3's uint32 ceiling — so neither clause can be quietly dropped
// without a failure here.
func TestSchemaFor_NoV3RepresentableContentEverWritesV4(t *testing.T) {
	v3able := func(mutate func(*ChannelData)) *Codeplug {
		d := &ChannelData{
			FreqHz: 14250000, Mode: "USB", CTCSS: "OFF",
			CTCSSTone:  ToneField{State: Unknown},
			Shift:      "SIMPLEX",
			TagDisplay: BoolField{State: Known},
			ScanSkip:   BoolField{State: Unknown},
		}
		if mutate != nil {
			mutate(d)
		}
		return &Codeplug{Schema: CurrentSchema, Generator: "test", Channels: []Channel{{Slot: "001", Data: d}}}
	}

	for _, tt := range []struct {
		name   string
		cp     *Codeplug
		want   int
		reason string
	}{
		{"nothing but the pre-tier fields", v3able(nil), lowestSchema, "no tier field, freq in range"},
		{"no channels at all", &Codeplug{Schema: CurrentSchema}, lowestSchema, "nothing to represent"},
		{"an empty channel", &Codeplug{Schema: CurrentSchema, Channels: []Channel{{Slot: "003"}}}, lowestSchema, "an empty slot holds no fields"},
		{
			"the highest frequency schema 3 can hold",
			v3able(func(d *ChannelData) { d.FreqHz = math.MaxUint32 }),
			lowestSchema, "MaxUint32 still fits v3's uint32",
		},
		{
			"one hertz past schema 3's ceiling",
			v3able(func(d *ChannelData) { d.FreqHz = math.MaxUint32 + 1 }),
			4, "a value v3 cannot represent forces v4 even with no tier field",
		},
		{
			"a 10 GHz frequency",
			v3able(func(d *ChannelData) { d.FreqHz = 10_000_000_000 }),
			4, "the IC-905's reach",
		},
		{
			"a Known tier field",
			v3able(func(d *ChannelData) { d.Duplex = StringField{State: Known, Value: "DUP+"} }),
			4, "a recorded D4 field requires schema 4",
		},
		{
			"an Unknown tier field is still present",
			v3able(func(d *ChannelData) { d.ToneMode = StringField{State: Unknown} }),
			4, "Unknown is a state somebody chose; Absent is not",
		},
		{
			"a tier field left Absent records nothing",
			v3able(func(d *ChannelData) { d.Filter = StringField{} }),
			lowestSchema, "the zero FieldState says nothing",
		},
		{
			"an Unavailable tier field records nothing either",
			v3able(func(d *ChannelData) { d.Filter = StringField{State: Unavailable} }),
			lowestSchema, "schema 3 says the same thing by having no key",
		},
		{
			"every tier field Unavailable, as a Yaesu read leaves them",
			v3able(func(d *ChannelData) { withUnavailableTierFields(d) }),
			lowestSchema, "this is the case the byte-identity guarantee rests on",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := schemaFor(tt.cp); got != tt.want {
				t.Errorf("schemaFor() = %d, want %d (%s)", got, tt.want, tt.reason)
			}
		})
	}
}

func TestSchemaFor_ReceiverFieldsForceV5OnlyWhenRecorded(t *testing.T) {
	setters := map[string]func(*ChannelData, FieldState){
		"tuning_step_enabled": func(d *ChannelData, s FieldState) { d.TuningStepEnabled = BoolField{State: s} },
		"tuning_step":         func(d *ChannelData, s FieldState) { d.TuningStep = StringField{State: s} },
		"program_tuning_step": func(d *ChannelData, s FieldState) { d.ProgramTuningStepHz = FreqField{State: s} },
		"attenuator":          func(d *ChannelData, s FieldState) { d.AttenuatorDB = IntField{State: s} },
		"preamp":              func(d *ChannelData, s FieldState) { d.Preamp = StringField{State: s} },
		"antenna":             func(d *ChannelData, s FieldState) { d.Antenna = StringField{State: s} },
		"ip_plus":             func(d *ChannelData, s FieldState) { d.IPPlus = BoolField{State: s} },
	}
	for name, set := range setters {
		for _, state := range []FieldState{Known, Unknown} {
			t.Run(name+"/"+string(state), func(t *testing.T) {
				d := &ChannelData{FreqHz: 145_500_000, Duplex: StringField{State: Known, Value: "OFF"}}
				set(d, state)
				cp := &Codeplug{Channels: []Channel{{Slot: "001", Data: d}}}
				if got := schemaFor(cp); got != 5 {
					t.Errorf("schemaFor() = %d, want 5", got)
				}
			})
		}
		t.Run(name+"/unavailable", func(t *testing.T) {
			d := &ChannelData{FreqHz: 145_500_000, Duplex: StringField{State: Known, Value: "OFF"}}
			set(d, Unavailable)
			cp := &Codeplug{Channels: []Channel{{Slot: "001", Data: d}}}
			if got := schemaFor(cp); got != 4 {
				t.Errorf("schemaFor() = %d, want 4: Unavailable is not recorded", got)
			}
		})
	}

	t.Run("a later D8 channel wins over an earlier schema-4 channel", func(t *testing.T) {
		cp := &Codeplug{Channels: []Channel{
			{Slot: "001", Data: &ChannelData{FreqHz: math.MaxUint32 + 1}},
			{Slot: "002", Data: &ChannelData{
				FreqHz:  145_500_000,
				Antenna: StringField{State: Known, Value: "ANT2"},
			}},
		}}
		if got := schemaFor(cp); got != 5 {
			t.Errorf("schemaFor() = %d, want 5: a schema-4 channel must not hide later D8 content", got)
		}
	})
}

func TestLoadV4_FrozenShapeMigratesReceiverFields(t *testing.T) {
	body := `{"schema":4,"generator":"test","radio":{"model":"IC-7610","cat_id":"98","read_at":"2026-08-28T00:00:00Z"},` +
		`"channels":[{"slot":"001","data":{"freq_hz":145500000,"mode":"FM","ctcss":"","ctcss_tone":{"state":"unavailable"},` +
		`"shift":"","tag_display":{"state":"unavailable"},"scan_skip":{"state":"unavailable"},` +
		`"tx_frequency":{"state":"unavailable"},"duplex":{"state":"known","value":"OFF"},"offset":{"state":"unavailable"},` +
		`"tone_mode":{"state":"known","value":"OFF"},"tone_tx":{"state":"unavailable"},"tone_rx":{"state":"unavailable"},` +
		`"dtcs_code":{"state":"unavailable"},"dtcs_polarity":{"state":"unavailable"},"filter":{"state":"known","value":"FIL1"},` +
		`"data_mode":{"state":"unavailable"}}}]} `
	path := filepath.Join(t.TempDir(), "v4.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cp, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	d := cp.Channels[0].Data
	if d.Duplex != (StringField{State: Known, Value: "OFF"}) || d.Filter != (StringField{State: Known, Value: "FIL1"}) {
		t.Errorf("v4 fields were not preserved: %+v", *d)
	}
	for name, state := range map[string]FieldState{
		"tuning_step_enabled": d.TuningStepEnabled.State,
		"tuning_step":         d.TuningStep.State,
		"program_tuning_step": d.ProgramTuningStepHz.State,
		"attenuator":          d.AttenuatorDB.State,
		"preamp":              d.Preamp.State,
		"antenna":             d.Antenna.State,
		"ip_plus":             d.IPPlus.State,
	} {
		if state != Unavailable {
			t.Errorf("%s state = %q, want Unavailable", name, state)
		}
	}

	bad := strings.Replace(body, `"data_mode":{"state":"unavailable"}`, `"data_mode":{"state":"unavailable"},"ip_plus":{"state":"known"}`, 1)
	badPath := filepath.Join(t.TempDir(), "v4-with-v5-key.json")
	if err := os.WriteFile(badPath, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(badPath); err == nil || !strings.Contains(err.Error(), `unknown field "ip_plus"`) {
		t.Fatalf("Load(v4 with v5 key) error = %v, want unknown ip_plus", err)
	}
}

func TestSave_ReceiverContentRoundTripsThroughSchema5(t *testing.T) {
	d := withUnavailableTierFields(&ChannelData{FreqHz: 145_500_000, Mode: "FM"})
	d.TuningStepEnabled = BoolField{State: Known, Value: true}
	d.TuningStep = StringField{State: Known, Value: "5 kHz"}
	d.ProgramTuningStepHz = FreqField{State: Known, Value: 500}
	d.AttenuatorDB = IntField{State: Known, Value: 10}
	d.Preamp = StringField{State: Known, Value: "ON"}
	d.Antenna = StringField{State: Known, Value: "ANT2"}
	d.IPPlus = BoolField{State: Known, Value: true}
	cp := &Codeplug{Schema: CurrentSchema, Generator: "test", Channels: []Channel{{Slot: "001", Data: d}}}
	path := filepath.Join(t.TempDir(), "v5.json")
	if err := Save(path, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"schema": 5`) {
		t.Fatalf("saved file is not schema 5:\n%s", raw)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got.Channels, cp.Channels) {
		t.Errorf("schema 5 round trip differs:\n got %+v\nwant %+v", got.Channels, cp.Channels)
	}

	// And a second save is byte-identical to the first — the check its
	// schema-4 sibling (TestSave_TierContentRoundTripsThroughSchema4) has
	// always had, and schema 5 went without. DeepEqual on the channels
	// alone cannot see a writer that settles on different BYTES for the
	// same value: a key order taken from a map, a re-marshal that drops
	// an omitempty, a schema promoted on the second pass. This is the
	// assertion that catches those.
	again := filepath.Join(t.TempDir(), "v5.json")
	if err := Save(again, got); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	raw2, err := os.ReadFile(again)
	if err != nil {
		t.Fatalf("reading re-saved file: %v", err)
	}
	if string(raw2) != string(raw) {
		t.Errorf("save(load(save(cp))) is not byte-identical:\n--- first ---\n%s\n--- second ---\n%s", raw, raw2)
	}
}

// TestSchemaFor_EveryTierFieldForcesV4 walks all ten tier-added fields
// so that a field added to ChannelData without being accounted for in
// ChannelData.tierFieldsAbsent fails here, rather than silently being
// written into a schema-3 file that has no key for it.
func TestSchemaFor_EveryTierFieldForcesV4(t *testing.T) {
	for name, mutate := range map[string]func(*ChannelData){
		"tx_frequency":  func(d *ChannelData) { d.TxFreqHz = FreqField{State: Known, Value: 145_000_000} },
		"duplex":        func(d *ChannelData) { d.Duplex = StringField{State: Known, Value: "DUP+"} },
		"offset":        func(d *ChannelData) { d.OffsetHz = FreqField{State: Known, Value: 600_000} },
		"tone_mode":     func(d *ChannelData) { d.ToneMode = StringField{State: Known, Value: "TSQL"} },
		"tone_tx":       func(d *ChannelData) { d.ToneTx = ToneField{State: Known, Value: 885} },
		"tone_rx":       func(d *ChannelData) { d.ToneRx = ToneField{State: Known, Value: 885} },
		"dtcs_code":     func(d *ChannelData) { d.DTCSCode = IntField{State: Known, Value: 23} },
		"dtcs_polarity": func(d *ChannelData) { d.DTCSPolarity = StringField{State: Known, Value: "NN"} },
		"filter":        func(d *ChannelData) { d.Filter = StringField{State: Known, Value: "FIL1"} },
		"data_mode":     func(d *ChannelData) { d.DataMode = BoolField{State: Known, Value: true} },
	} {
		t.Run(name, func(t *testing.T) {
			d := &ChannelData{FreqHz: 14250000, Mode: "USB"}
			mutate(d)
			cp := &Codeplug{Schema: CurrentSchema, Channels: []Channel{{Slot: "001", Data: d}}}
			if got := schemaFor(cp); got != 4 {
				t.Errorf("schemaFor() = %d with %s present, want 4", got, name)
			}
		})
	}
}

// TestSave_TierContentRoundTripsThroughSchema4: a codeplug the tier's
// fields make unrepresentable in schema 3 is written as schema 4, and
// reading it back reproduces every field — including the Absent ones,
// which must come back Absent rather than as some state the file
// invented for them.
func TestSave_TierContentRoundTripsThroughSchema4(t *testing.T) {
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "test",
		Radio:     RadioInfo{Model: "IC-905", CATID: "A4"},
		Channels: []Channel{
			{Slot: "G01-001", Data: &ChannelData{
				FreqHz: 10_000_000_000, Mode: "USB",
				CTCSSTone:    ToneField{State: Unavailable},
				TagDisplay:   BoolField{State: Unavailable},
				ScanSkip:     BoolField{State: Known, Value: true},
				TxFreqHz:     FreqField{State: Known, Value: 10_000_600_000},
				Duplex:       StringField{State: Known, Value: "DUP+"},
				OffsetHz:     FreqField{State: Known, Value: 600_000},
				ToneMode:     StringField{State: Known, Value: "TSQL"},
				ToneTx:       ToneField{State: Known, Value: 885},
				ToneRx:       ToneField{State: Unknown},
				DTCSCode:     IntField{State: Unavailable},
				DTCSPolarity: StringField{State: Unavailable},
				Filter:       StringField{State: Known, Value: "FIL1"},
				DataMode:     BoolField{State: Known, Value: false},
			}},
			{Slot: "G01-002", Data: &ChannelData{
				FreqHz: 145_500_000, Mode: "FM",
				// Every tier field Absent on this one: a schema-4 file
				// must be able to carry a channel that says nothing about
				// them, and say so honestly.
			}},
		},
	}
	for i := range cp.Channels {
		if cp.Channels[i].Data != nil {
			withUnavailableReceiverFields(cp.Channels[i].Data)
		}
	}

	path := filepath.Join(t.TempDir(), "v4.json")
	if err := Save(path, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	var probe struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("probing saved file: %v", err)
	}
	if probe.Schema != 4 {
		t.Fatalf("saved schema = %d, want 4", probe.Schema)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Channels) != len(cp.Channels) {
		t.Fatalf("Load() returned %d channels, want %d", len(got.Channels), len(cp.Channels))
	}
	for i := range cp.Channels {
		if *got.Channels[i].Data != *cp.Channels[i].Data {
			t.Errorf("channel %d round-tripped differently:\n got %+v\nwant %+v", i, *got.Channels[i].Data, *cp.Channels[i].Data)
		}
	}

	// And a second save is byte-identical to the first: the schema-4
	// writer is as stable as the schema-3 one.
	again := filepath.Join(t.TempDir(), "v4.json")
	if err := Save(again, got); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	raw2, err := os.ReadFile(again)
	if err != nil {
		t.Fatalf("reading re-saved file: %v", err)
	}
	if string(raw2) != string(raw) {
		t.Errorf("save(load(save(cp))) is not byte-identical:\n--- first ---\n%s\n--- second ---\n%s", raw, raw2)
	}
}

// TestLoadV3_MigratesTierFieldsToUnavailable pins the decision
// migrateV3ChannelData records, and its consequence: every tier field
// comes back Unavailable — the same state a READ of one of those radios
// produces, so a codeplug loaded from an old file still compares equal
// to a fresh read — and none of them Records anything, so the file is
// re-saved as schema 3.
func TestLoadV3_MigratesTierFieldsToUnavailable(t *testing.T) {
	cp, err := Load(filepath.Join("testdata", "canonical-v3-basic.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, ch := range cp.Channels {
		if ch.Data == nil {
			continue
		}
		if !ch.Data.tierFieldsUnrecorded() {
			t.Errorf("slot %q: a schema-3 file produced a Recorded tier field: %+v", ch.Slot, *ch.Data)
		}
		for name, state := range map[string]FieldState{
			"tx_frequency":        ch.Data.TxFreqHz.State,
			"duplex":              ch.Data.Duplex.State,
			"offset":              ch.Data.OffsetHz.State,
			"tone_mode":           ch.Data.ToneMode.State,
			"tone_tx":             ch.Data.ToneTx.State,
			"tone_rx":             ch.Data.ToneRx.State,
			"dtcs_code":           ch.Data.DTCSCode.State,
			"dtcs_polarity":       ch.Data.DTCSPolarity.State,
			"filter":              ch.Data.Filter.State,
			"data_mode":           ch.Data.DataMode.State,
			"tuning_step_enabled": ch.Data.TuningStepEnabled.State,
			"tuning_step":         ch.Data.TuningStep.State,
			"program_tuning_step": ch.Data.ProgramTuningStepHz.State,
			"attenuator":          ch.Data.AttenuatorDB.State,
			"preamp":              ch.Data.Preamp.State,
			"antenna":             ch.Data.Antenna.State,
			"ip_plus":             ch.Data.IPPlus.State,
		} {
			if state != Unavailable {
				t.Errorf("slot %q: %s = %q, want %q", ch.Slot, name, state, Unavailable)
			}
		}
	}
}

// TestLoadV3_FrozenShapeRejectsTierKeys: the frozen schema-3 decoder
// must refuse a schema-3 file carrying a schema-4 key. Without the
// freeze this would have SUCCEEDED — loadV3 decoded straight into the
// live struct until this tier — quietly accepting a file that claims a
// version it does not conform to.
func TestLoadV3_FrozenShapeRejectsTierKeys(t *testing.T) {
	body := `{"schema":3,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},` +
		`"channels":[{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},` +
		`"shift":"SIMPLEX","scan_skip":{"state":"unknown"},"tag_display":{"state":"known"},"duplex":{"state":"known","value":"DUP+"}}}]}`
	path := filepath.Join(t.TempDir(), "v3-with-v4-key.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an unknown-field rejection for a schema-4 key in a schema-3 file")
	}
	var ufe *UnknownFieldError
	if !errors.As(err, &ufe) || ufe.Field != "duplex" {
		t.Fatalf("Load() error = %v, want *UnknownFieldError naming \"duplex\"", err)
	}
}

// v3FileWithFreqHz builds a minimal, well-formed schema-3 file whose one
// channel carries freqHz, and returns its path.
//
// Built INLINE rather than by editing one of the frozen canonical goldens
// in testdata: those files are evidence of what the pre-tier writer
// emitted (see canonicalV3Goldens), and a test that edited one would
// destroy the evidence to make its point. The shape is modelled on them —
// every key schema 3 required, and nothing else.
//
// freqHz is a string, not a uint64, because the value under test is one
// encoding/json can decode and the LOADER must refuse: a Go literal would
// only prove that a number too big for uint64 is rejected, which was never
// in doubt.
func v3FileWithFreqHz(t *testing.T, freqHz string) string {
	t.Helper()
	body := `{"schema":3,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},` +
		`"channels":[{"slot":"001","data":{"freq_hz":` + freqHz + `,"mode":"USB","ctcss":"OFF",` +
		`"ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","tag_display":{"state":"known"},` +
		`"scan_skip":{"state":"unknown"}}}]}`
	path := filepath.Join(t.TempDir(), "v3.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// TestLoadV3_FreqHzOutsideSchema3RangeIsRefused pins the bound
// channelDataV3's uint64 freq_hz does NOT enforce.
//
// The uint64 is a deliberate decision (see channelDataV3's doc comment) and
// stays: it exists so that a bypass of the schema rule can never TRUNCATE
// on encode. But it decodes WIDER than schema 3 could hold, and until this
// check a hand-edited file carrying a freq_hz above uint32's range loaded
// happily — an out-of-schema value laundered through a loader named for the
// schema. Nothing downstream misbehaved (the live model is uint64
// throughout); what was wrong is that the file was accepted as a schema-3
// file while conforming to no schema at all.
//
// Both sides of the boundary are asserted, because a refusal alone cannot
// tell a correct bound from one that is off by one — or from a check that
// refuses every v3 file.
func TestLoadV3_FreqHzOutsideSchema3RangeIsRefused(t *testing.T) {
	t.Run("above the range is refused", func(t *testing.T) {
		_, err := Load(v3FileWithFreqHz(t, "4294967296"))
		if err == nil {
			t.Fatal("Load() error = nil, want a refusal: freq_hz 4294967296 is outside schema 3's uint32 range")
		}
		for _, want := range []string{"codeplug: load ", "001", "4294967296", "4294967295"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Load() error = %q, want it to contain %q (the message must name the file, the slot and schema 3's range)", err, want)
			}
		}
	})

	t.Run("the boundary value loads", func(t *testing.T) {
		cp, err := Load(v3FileWithFreqHz(t, "4294967295"))
		if err != nil {
			t.Fatalf("Load() = %v, want success — 4294967295 is IN schema 3's range", err)
		}
		if got := cp.Channels[0].Data.FreqHz; got != math.MaxUint32 {
			t.Errorf("FreqHz = %d, want %d", got, uint64(math.MaxUint32))
		}
	})
}

// receiverFieldStates names every one of the seven receiver fields
// schema 5 added (additions design D8) alongside the state d records for
// it. One table serves the three schema-5 tests below so that an eighth
// receiver field cannot be added without a row here, exactly as
// tierFieldNormalisers works for the seventeen.
func receiverFieldStates(d *ChannelData) map[string]FieldState {
	return map[string]FieldState{
		"tuning_step_enabled": d.TuningStepEnabled.State,
		"tuning_step":         d.TuningStep.State,
		"program_tuning_step": d.ProgramTuningStepHz.State,
		"attenuator":          d.AttenuatorDB.State,
		"preamp":              d.Preamp.State,
		"antenna":             d.Antenna.State,
		"ip_plus":             d.IPPlus.State,
	}
}

// icomTierFieldStates names the ten fields the Icom tier added (design
// D4) alongside the state d records for each. Companion to
// receiverFieldStates.
func icomTierFieldStates(d *ChannelData) map[string]FieldState {
	return map[string]FieldState{
		"tx_frequency":  d.TxFreqHz.State,
		"duplex":        d.Duplex.State,
		"offset":        d.OffsetHz.State,
		"tone_mode":     d.ToneMode.State,
		"tone_tx":       d.ToneTx.State,
		"tone_rx":       d.ToneRx.State,
		"dtcs_code":     d.DTCSCode.State,
		"dtcs_polarity": d.DTCSPolarity.State,
		"filter":        d.Filter.State,
		"data_mode":     d.DataMode.State,
	}
}

// v5FileWithExtraKeys builds a minimal, well-formed schema-5 file whose
// one channel carries schema 3's own seven required keys plus whatever
// extra is (a JSON fragment, leading comma included, or empty), and
// returns its path.
//
// Built INLINE for the reason v3FileWithFreqHz is: a SPARSE file — one
// omitting keys the writer would always emit — cannot be obtained by
// editing a canonical golden without destroying the evidence that golden
// exists to be.
func v5FileWithExtraKeys(t *testing.T, extra string) string {
	t.Helper()
	body := `{"schema":5,"generator":"x","radio":{"model":"IC-R8600","cat_id":"96","read_at":"2026-08-30T00:00:00Z"},` +
		`"channels":[{"slot":"G00-000","data":{"freq_hz":145500000,"mode":"FM","ctcss":"OFF",` +
		`"ctcss_tone":{"state":"unavailable"},"shift":"SIMPLEX","tag_display":{"state":"unavailable"},` +
		`"scan_skip":{"state":"unavailable"}` + extra + `}}]}`
	path := filepath.Join(t.TempDir(), "v5.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// TestLoadV5_SparseFileGetsNoBlanketRule pins the ONE rule Load's doc
// comment states for schema 5 and that no test previously exercised:
// loadV5 applies no blanket migration, and must not.
//
// Every older loader resolves the added fields for the whole file —
// loadV1/V2/V3 set all seventeen Unavailable, loadV4 sets the seven
// receiver ones Unavailable — because a file written before those fields
// existed says, by having no key, that its radio has none. A schema-5
// file says no such thing: it can hold any of the seventeen honestly, so
// a key it does NOT hold means either "this radio has no such field" or
// "nobody has answered yet", and only the model's own capabilities can
// tell those apart. That decision is NormaliseTierFields', not Load's,
// and this test is what stops a well-meaning "loadV5 is the odd one out"
// change from pre-empting it.
//
// So the assertion is deliberately the negative one: the missing keys
// come back ABSENT — not Unavailable, which would be a claim about the
// radio this file never made.
func TestLoadV5_SparseFileGetsNoBlanketRule(t *testing.T) {
	cp, err := Load(v5FileWithExtraKeys(t, `,"preamp":{"state":"unknown"},"antenna":{"state":"known","value":"ANT1"}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Load's contract: every accepted file comes back at CurrentSchema.
	// Tautological while schema 5 IS the current one; it stops being so
	// the day CurrentSchema moves and loadV5 becomes a frozen loader.
	if cp.Schema != CurrentSchema {
		t.Errorf("Load().Schema = %d, want %d (migrate-on-load)", cp.Schema, CurrentSchema)
	}

	d := cp.Channels[0].Data
	if d.Preamp != (StringField{State: Unknown}) {
		t.Errorf("preamp = %+v, want an Unknown StringField preserved as written", d.Preamp)
	}
	if d.Antenna != (StringField{State: Known, Value: "ANT1"}) {
		t.Errorf("antenna = %+v, want the Known value preserved as written", d.Antenna)
	}
	for name, state := range receiverFieldStates(d) {
		if name == "preamp" || name == "antenna" {
			continue
		}
		if state != Absent {
			t.Errorf("receiver field %s = %q, want Absent: a schema-5 file's missing key is not a claim that the radio lacks the field", name, state)
		}
	}
	for name, state := range icomTierFieldStates(d) {
		if state != Absent {
			t.Errorf("tier field %s = %q, want Absent: loadV5 has no blanket rule for schema 4's ten either", name, state)
		}
	}
}

// TestSaveLoad_SparseV5RoundTripsAbsentDistinctFromRecorded is the
// round-trip half: a schema-5 file whose receiver content is PRESENT but
// SPARSE — one field Known, one Unknown, the other five never spoken
// about — must survive Save/Load with all three answers still distinct.
//
// It can only work because schemaFor promotes the file to 5 (a Recorded
// receiver field forces it), and because the schema-5 marshal shape
// writes every field's "state" key without omitempty, so an Absent field
// goes to disk as an explicit empty state rather than vanishing. Drop
// either and Absent would come back as something the file invented for
// it — which is precisely the distinction the send gate depends on
// (codeplug.Validate refuses an Absent field the radio HAS; it accepts
// an Unavailable one).
func TestSaveLoad_SparseV5RoundTripsAbsentDistinctFromRecorded(t *testing.T) {
	src := v5FileWithExtraKeys(t, `,"preamp":{"state":"unknown"},"antenna":{"state":"known","value":"ANT1"}`)
	cp, err := Load(src)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	dst := filepath.Join(t.TempDir(), "resaved.json")
	if err := Save(dst, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading re-saved file: %v", err)
	}
	var probe struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("probing re-saved file: %v", err)
	}
	if probe.Schema != 5 {
		t.Fatalf("re-saved schema = %d, want 5: a Recorded receiver field is exactly what schema 5 exists to hold\n%s", probe.Schema, raw)
	}

	got, err := Load(dst)
	if err != nil {
		t.Fatalf("re-Load() error = %v", err)
	}
	if !reflect.DeepEqual(got.Channels, cp.Channels) {
		t.Errorf("sparse schema-5 round trip differs:\n got %+v\nwant %+v", *got.Channels[0].Data, *cp.Channels[0].Data)
	}

	// And a second save is byte-identical to the first, so the sparse
	// file settles rather than drifting a key at a time.
	again := filepath.Join(t.TempDir(), "resaved.json")
	if err := Save(again, got); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	raw2, err := os.ReadFile(again)
	if err != nil {
		t.Fatalf("reading second re-saved file: %v", err)
	}
	if string(raw2) != string(raw) {
		t.Errorf("save(load(save(cp))) is not byte-identical:\n--- first ---\n%s\n--- second ---\n%s", raw, raw2)
	}
}

// TestSaveLoad_V5WithNothingRecordedIsRewrittenAsSchema3 pins the
// behaviour that looks like a bug and is not, so that nobody "fixes" it
// into one.
//
// A schema-5 file whose channel mentions none of the seventeen added
// fields re-saves as SCHEMA 3, and reloading it turns those seventeen
// from Absent into Unavailable. Both halves are correct by the schemaFor
// doctrine (see schemaFor and FieldState.Recorded), and the alternative
// is the damaging one:
//
//   - Absent is not Recorded, so it needs no key, so schema 3 can
//     represent this content. Promoting a file on Absent instead would
//     mean every channel built without a bank in hand — the GUI's
//     newChannelData omits every added key — dragged an ordinary Yaesu
//     codeplug up to schema 5, destroying the byte identity design D4
//     exists to guarantee.
//   - the reload's Unavailable is what a schema-3 file MEANS (see
//     migrateV3ChannelData), and it is what a read of a radio without
//     those fields reports, so the reloaded codeplug still compares
//     equal to a fresh read.
//
// What IS lost across that round trip is the Absent/Unavailable
// distinction, and it is lost deliberately: this file recorded nothing,
// so there is nothing the lower schema fails to carry. The distinction
// is preserved wherever the file actually holds receiver content — see
// TestSaveLoad_SparseV5RoundTripsAbsentDistinctFromRecorded.
func TestSaveLoad_V5WithNothingRecordedIsRewrittenAsSchema3(t *testing.T) {
	cp, err := Load(v5FileWithExtraKeys(t, ""))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for name, state := range receiverFieldStates(cp.Channels[0].Data) {
		if state != Absent {
			t.Fatalf("receiver field %s = %q on load, want Absent", name, state)
		}
	}

	dst := filepath.Join(t.TempDir(), "resaved.json")
	if err := Save(dst, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading re-saved file: %v", err)
	}
	var probe struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("probing re-saved file: %v", err)
	}
	if probe.Schema != lowestSchema {
		t.Fatalf("re-saved schema = %d, want %d: nothing here is Recorded, so nothing needs a later schema\n%s", probe.Schema, lowestSchema, raw)
	}
	for _, key := range []string{"tx_frequency", "preamp", "ip_plus"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("re-saved schema-3 file carries the key %q, which schema 3 has no place for:\n%s", key, raw)
		}
	}

	got, err := Load(dst)
	if err != nil {
		t.Fatalf("re-Load() error = %v", err)
	}
	d := got.Channels[0].Data
	for name, state := range receiverFieldStates(d) {
		if state != Unavailable {
			t.Errorf("after the schema-3 round trip, receiver field %s = %q, want Unavailable — what a schema-3 file says", name, state)
		}
	}
	for name, state := range icomTierFieldStates(d) {
		if state != Unavailable {
			t.Errorf("after the schema-3 round trip, tier field %s = %q, want Unavailable", name, state)
		}
	}
}
