// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// legacyBody builds a schema-1 or schema-2 codeplug JSON document with one
// populated channel, splicing dataClause into that channel's "data" object
// immediately after "tag". It exists so the migration matrix below can
// vary ONE key ("tag_display") across otherwise byte-identical legacy
// files, in both old schemas, without hand-writing six near-identical
// literals.
func legacyBody(schema int, dataClause string) string {
	return fmt.Sprintf(`{"schema":%d,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},`+
		`"channels":[{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF",`+
		`"ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","tag":"CALLING"%s,"scan_skip":{"state":"unknown"}}}]}`,
		schema, dataClause)
}

// TestLoad_TagDisplayMigrationMatrix is E1's migration proof: schema 1 and
// schema 2 alike, with "tag_display" PRESENT-TRUE, PRESENT-FALSE, or
// ABSENT, must load as a BoolField carrying the documented result.
//
// All six cases land on Known — see migrateLegacyTagDisplay for why absent
// migrates to Known-false (strict behaviour preservation: the field was
// already being SENT as false, so the migrated file sends the identical
// byte) rather than to the honest-provenance Unknown, which would have
// mass-blocked every legacy FT-710 file's channels at plan time.
//
// Present-FALSE is tested separately from absent on purpose: on disk they
// were indistinguishable (v1/v2 wrote the key with omitempty), and the
// frozen decoder's *bool is precisely what tells them apart, so a
// regression that collapsed the two would otherwise pass unnoticed.
func TestLoad_TagDisplayMigrationMatrix(t *testing.T) {
	cases := []struct {
		name       string
		dataClause string
		want       BoolField
	}{
		{"present true", `,"tag_display":true`, BoolField{State: Known, Value: true}},
		{"present false", `,"tag_display":false`, BoolField{State: Known, Value: false}},
		{"absent", ``, BoolField{State: Known, Value: false}},
	}
	for _, schema := range []int{1, 2} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("v%d/%s", schema, tc.name), func(t *testing.T) {
				got, err := writeAndLoad(t, legacyBody(schema, tc.dataClause))
				if err != nil {
					t.Fatalf("Load() error = %v", err)
				}
				if got.Schema != CurrentSchema {
					t.Errorf("Schema = %d, want %d (migrate-on-load bumps to current)", got.Schema, CurrentSchema)
				}
				if len(got.Channels) != 1 || got.Channels[0].Data == nil {
					t.Fatalf("Channels = %+v, want one populated channel", got.Channels)
				}
				if td := got.Channels[0].Data.TagDisplay; td != tc.want {
					t.Errorf("TagDisplay = %+v, want %+v", td, tc.want)
				}
				// The migrated value must also be internally consistent —
				// a migration that produced a shape Validate rejects would
				// have made every legacy file unsendable.
				if err := got.Channels[0].Data.TagDisplay.Valid(); err != nil {
					t.Errorf("migrated TagDisplay.Valid() = %v, want nil", err)
				}
				// Everything else must survive the frozen-shape decode
				// untouched: the migration changes exactly one field.
				d := got.Channels[0].Data
				if d.FreqHz != 14250000 || d.Mode != "USB" || d.CTCSS != "OFF" || d.Shift != "SIMPLEX" || d.Tag != "CALLING" {
					t.Errorf("non-TagDisplay fields = %+v, want them carried across unchanged", *d)
				}
				if d.CTCSSTone != (ToneField{State: Unknown}) || d.ScanSkip != (BoolField{State: Unknown}) {
					t.Errorf("CTCSSTone/ScanSkip = %+v/%+v, want both {unknown}", d.CTCSSTone, d.ScanSkip)
				}
			})
		}
	}
}

// TestLoad_LegacyEmptySlotStillEmpty: a legacy channel with no "data" key
// migrates to an EMPTY channel, not to a populated one carrying a
// zero-valued (and therefore invalid) TagDisplay. Data == nil is the sole
// empty/populated discriminator, and the migration must not blunt it.
func TestLoad_LegacyEmptySlotStillEmpty(t *testing.T) {
	for _, schema := range []int{1, 2} {
		t.Run(fmt.Sprintf("v%d", schema), func(t *testing.T) {
			body := fmt.Sprintf(`{"schema":%d,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[{"slot":"001"}]}`, schema)
			got, err := writeAndLoad(t, body)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(got.Channels) != 1 {
				t.Fatalf("len(Channels) = %d, want 1", len(got.Channels))
			}
			if !got.Channels[0].Empty() {
				t.Errorf("Channels[0] = %+v, want an empty slot (Data == nil)", got.Channels[0])
			}
		})
	}
}

// TestSaveLoad_Schema3TagDisplayFourStates round-trips every value a
// TagDisplay BoolField can hold — Known-true, Known-false, Unknown and
// Unavailable — through Save and Load at schema 3, and additionally
// asserts that "tag_display" is ALWAYS present in the encoded JSON.
//
// That last assertion is the point of dropping omitempty: with it, a
// Known-false and an Unknown channel would both encode to no key at all,
// re-creating on disk exactly the ambiguity this field was widened to
// remove.
func TestSaveLoad_Schema3TagDisplayFourStates(t *testing.T) {
	states := []BoolField{
		{State: Known, Value: true},
		{State: Known, Value: false},
		{State: Unknown},
		{State: Unavailable},
	}
	for _, want := range states {
		t.Run(fmt.Sprintf("%s/%v", want.State, want.Value), func(t *testing.T) {
			cp := &Codeplug{
				Schema:    CurrentSchema,
				Generator: "test",
				Radio:     RadioInfo{Model: "FT-710", CATID: "0800"},
				Channels: []Channel{{Slot: "001", Data: &ChannelData{
					FreqHz:     14250000,
					Mode:       "USB",
					CTCSS:      "OFF",
					CTCSSTone:  ToneField{State: Unknown},
					Shift:      "SIMPLEX",
					Tag:        "CALLING",
					TagDisplay: want,
					ScanSkip:   BoolField{State: Unknown},
				}}},
			}
			path := filepath.Join(t.TempDir(), "test.rop.json")
			if err := Save(path, cp); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading saved file: %v", err)
			}
			if !strings.Contains(string(raw), `"tag_display"`) {
				t.Errorf("saved file has no \"tag_display\" key:\n%s", raw)
			}

			got, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.Schema != CurrentSchema {
				t.Errorf("Schema = %d, want %d", got.Schema, CurrentSchema)
			}
			if len(got.Channels) != 1 || got.Channels[0].Data == nil {
				t.Fatalf("Channels = %+v, want one populated channel", got.Channels)
			}
			if td := got.Channels[0].Data.TagDisplay; td != want {
				t.Errorf("TagDisplay = %+v, want %+v (schema-3 round trip must be lossless)", td, want)
			}
		})
	}
}

// TestLoad_LegacyUnknownFieldStillRejected proves the frozen legacy shapes
// did not quietly loosen strictness: a MISSPELT key inside a legacy
// channel's data is still an *UnknownFieldError, in v1 and v2 alike, while
// the legitimate legacy "tag_display" key next to it is accepted.
//
// Freezing a decode struct is exactly the change that could have broken
// this — a frozen shape that omitted a real legacy key would reject valid
// files, and one that gained a catch-all would accept typos silently.
func TestLoad_LegacyUnknownFieldStillRejected(t *testing.T) {
	for _, schema := range []int{1, 2} {
		t.Run(fmt.Sprintf("v%d", schema), func(t *testing.T) {
			body := legacyBody(schema, `,"tag_display":true,"tag_displayy":false`)
			_, err := writeAndLoad(t, body)
			if err == nil {
				t.Fatal("Load() error = nil, want *UnknownFieldError for a misspelt legacy key")
			}
			var ufe *UnknownFieldError
			if !errors.As(err, &ufe) {
				t.Fatalf("Load() error = %v, want *UnknownFieldError", err)
			}
			if ufe.Field != "tag_displayy" {
				t.Errorf("UnknownFieldError.Field = %q, want %q", ufe.Field, "tag_displayy")
			}
		})
	}
}

// TestLoad_LegacyDuplicateKeyStillRejected proves the frozen shapes did not
// disturb the duplicate-key walk either: a legacy channel repeating
// "tag_display" is still a *DuplicateKeyError with its containing path,
// rather than silently resolving last-wins into the migration.
func TestLoad_LegacyDuplicateKeyStillRejected(t *testing.T) {
	for _, schema := range []int{1, 2} {
		t.Run(fmt.Sprintf("v%d", schema), func(t *testing.T) {
			body := legacyBody(schema, `,"tag_display":true,"tag_display":false`)
			_, err := writeAndLoad(t, body)
			if err == nil {
				t.Fatal("Load() error = nil, want *DuplicateKeyError")
			}
			var dke *DuplicateKeyError
			if !errors.As(err, &dke) {
				t.Fatalf("Load() error = %v, want *DuplicateKeyError", err)
			}
			if dke.Key != "tag_display" {
				t.Errorf("DuplicateKeyError.Key = %q, want %q", dke.Key, "tag_display")
			}
			if dke.Path != "channels[0].data" {
				t.Errorf("DuplicateKeyError.Path = %q, want %q", dke.Path, "channels[0].data")
			}
		})
	}
}

// TestMigrateLegacyChannels_NilAndEmpty pins the two list-shape edge cases
// the migration must not change: a legacy file with NO "channels" key
// loads with nil Channels (as it did before schema 3), and one with an
// empty list loads with an empty, non-nil list.
func TestMigrateLegacyChannels_NilAndEmpty(t *testing.T) {
	if got := migrateLegacyChannels(nil); got != nil {
		t.Errorf("migrateLegacyChannels(nil) = %+v, want nil", got)
	}
	got := migrateLegacyChannels([]legacyChannel{})
	if got == nil {
		t.Error("migrateLegacyChannels(empty) = nil, want an empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("migrateLegacyChannels(empty) = %+v, want length 0", got)
	}
}

// THE RULE THIS SECTION ENFORCES, stated once and deliberately:
// **the legacy grammar NEVER derives from the live ChannelData.** Schemas
// 1 and 2 are closed formats — whatever they could hold, they held it
// before schema 3 existed and will hold it forever. The live struct, by
// contrast, is free to gain and lose fields at every future schema bump.
// Deriving one from the other in EITHER direction is therefore wrong, and
// wrong in a way that hides itself:
//
//   - Live → frozen (what this file did until M9c-5's review, W3) says
//     "whatever a zero live struct now marshals is what v1/v2 could
//     contain". Both halves of that were wrong, and they failed in
//     OPPOSITE directions — both confirmed by mutation, not reasoned:
//     adding a NON-omitempty field to ChannelData made the old assertion
//     FAIL (`legacyChannelData cannot decode key "mutation_probe"`),
//     demanding the frozen decoder accept a key no schema-2 file could
//     possibly carry — the loudest signal the guard could give, for the
//     one change that must not concern it at all. Adding an OMITEMPTY one
//     was invisible to it instead, because json.Marshal of a ZERO
//     ChannelData omits every omitempty field: 4 of the 11 (clar_hz,
//     rx_clar, tx_clar, tag) never reached the probe at all, so the "every
//     current key" the old name promised was really 7 of 11, and a frozen
//     decoder that had simply LOST clar_hz would have passed.
//   - Frozen → live would say "the live struct may only ever hold what
//     v1/v2 held", which is simply false.
//
// So the grammar is written down HERE, explicitly, as the manifest below,
// and the ONLY thing checked against it is the frozen structs' own
// reflected json tags — both directions, so neither the manifest nor a
// frozen struct can move without the other. Editing either is a
// deliberate act, and legitimate only to correct a MIS-STATEMENT of what
// v1/v2 actually held (see legacyChannel's doc comment, file.go).

// frozenLegacyChannelKeys is the complete set of JSON keys a schema-1 or
// schema-2 channel WRAPPER object could carry. See the rule above.
var frozenLegacyChannelKeys = []string{"slot", "data"}

// frozenLegacyChannelDataKeys is the complete set of JSON keys a schema-1
// or schema-2 populated channel's "data" object could carry, in the order
// those schemas emitted them. See the rule above.
var frozenLegacyChannelDataKeys = []string{
	"freq_hz", "mode", "clar_hz", "rx_clar", "tx_clar", "ctcss",
	"ctcss_tone", "shift", "tag", "tag_display", "scan_skip",
}

// jsonTagNames reflects the json key name of every field of the struct
// value v, in declaration order. It fails the test on a field with no json
// tag, an empty name, or a "-" name: the frozen structs are an on-disk
// grammar, so a field whose wire name is implicit (Go's default, the Go
// field name) or absent is a mis-statement of the format, not a style
// choice.
func jsonTagNames(t *testing.T, v any) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	out := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			t.Fatalf("%s.%s has no json tag — a frozen on-disk shape must name every key explicitly", rt.Name(), f.Name)
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			t.Fatalf("%s.%s json tag %q names no key", rt.Name(), f.Name, tag)
		}
		out = append(out, name)
	}
	return out
}

// TestFrozenLegacyKeyManifest is the frozen shapes' completeness guard,
// decoupled from the live structs (M9c-5 review, W3 — see the rule stated
// above this test for what the old live-derived version got wrong).
//
// Two assertions per shape:
//
//  1. The manifest and the frozen struct's reflected json tags agree BOTH
//     WAYS. A key in the manifest that no frozen field declares means the
//     decoder would REJECT a valid legacy file carrying it
//     (DisallowUnknownFields); a frozen field the manifest does not name
//     means the format grew a key nobody wrote down.
//  2. Every manifest key actually decodes, through a
//     DisallowUnknownFields decoder — the runtime property the old test
//     checked, now driven by the manifest rather than by whatever the
//     live struct happens to marshal.
//
// Key ORDER is deliberately not asserted. These structs are decode-only
// (nothing marshals them — see loadV1/loadV2), and JSON decoding is
// order-independent, so field order carries no format meaning here; the
// manifest lists them in declaration order for readability alone.
func TestFrozenLegacyKeyManifest(t *testing.T) {
	shapes := []struct {
		name     string
		manifest []string
		zero     any
	}{
		{"legacyChannel", frozenLegacyChannelKeys, legacyChannel{}},
		{"legacyChannelData", frozenLegacyChannelDataKeys, legacyChannelData{}},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			declared := jsonTagNames(t, shape.zero)

			inManifest := make(map[string]bool, len(shape.manifest))
			for _, k := range shape.manifest {
				inManifest[k] = true
			}
			isDeclared := make(map[string]bool, len(declared))
			for _, k := range declared {
				isDeclared[k] = true
			}

			for _, k := range shape.manifest {
				if !isDeclared[k] {
					t.Errorf("frozen key %q is in the manifest but no %s field declares it — a valid legacy file carrying it would be REJECTED by DisallowUnknownFields", k, shape.name)
				}
			}
			for _, k := range declared {
				if !inManifest[k] {
					t.Errorf("%s declares json key %q, which the frozen manifest does not list — either the manifest is out of date, or the frozen shape has drifted from what schema 1/2 actually held", shape.name, k)
				}
			}

			// The runtime half: each manifest key must survive the very
			// decoder Load uses for these shapes.
			for _, k := range shape.manifest {
				probe := reflect.New(reflect.TypeOf(shape.zero)).Interface()
				dec := json.NewDecoder(strings.NewReader(fmt.Sprintf(`{%q:null}`, k)))
				dec.DisallowUnknownFields()
				if err := dec.Decode(probe); err != nil {
					t.Errorf("%s cannot decode manifest key %q (%v)", shape.name, k, err)
				}
			}
		})
	}
}
