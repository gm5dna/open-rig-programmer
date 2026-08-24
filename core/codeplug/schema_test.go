// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
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
			CurrentSchema, "a value v3 cannot represent forces v4 even with no tier field",
		},
		{
			"a 10 GHz frequency",
			v3able(func(d *ChannelData) { d.FreqHz = 10_000_000_000 }),
			CurrentSchema, "the IC-905's reach",
		},
		{
			"a Known tier field",
			v3able(func(d *ChannelData) { d.Duplex = StringField{State: Known, Value: "DUP+"} }),
			CurrentSchema, "present means the state differs from absent",
		},
		{
			"an Unknown tier field is still present",
			v3able(func(d *ChannelData) { d.ToneMode = StringField{State: Unknown} }),
			CurrentSchema, "Unknown is a state somebody chose; Absent is not",
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
			if got := schemaFor(cp); got != CurrentSchema {
				t.Errorf("schemaFor() = %d with %s present, want %d", got, name, CurrentSchema)
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
	if probe.Schema != CurrentSchema {
		t.Fatalf("saved schema = %d, want %d", probe.Schema, CurrentSchema)
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
			"tx_frequency":  ch.Data.TxFreqHz.State,
			"duplex":        ch.Data.Duplex.State,
			"offset":        ch.Data.OffsetHz.State,
			"tone_mode":     ch.Data.ToneMode.State,
			"tone_tx":       ch.Data.ToneTx.State,
			"tone_rx":       ch.Data.ToneRx.State,
			"dtcs_code":     ch.Data.DTCSCode.State,
			"dtcs_polarity": ch.Data.DTCSPolarity.State,
			"filter":        ch.Data.Filter.State,
			"data_mode":     ch.Data.DataMode.State,
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
