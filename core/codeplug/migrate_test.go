// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// TestLegacyChannelData_CoversEveryCurrentKey is the frozen shapes' own
// completeness guard: every JSON key the schema-2 channel-data object could
// carry must be decodable by legacyChannelData, or a valid legacy file
// would be rejected outright by DisallowUnknownFields. It works from a
// marshalled ChannelData rather than a hand-kept list, so a future field
// added to the LIVE struct without a matching migration decision fails
// here loudly.
//
// It is deliberately a ONE-WAY check: the frozen shape may legitimately
// gain keys the live struct later drops, but it may never LACK one that a
// schema-2 file could contain.
func TestLegacyChannelData_CoversEveryCurrentKey(t *testing.T) {
	b, err := json.Marshal(ChannelData{})
	if err != nil {
		t.Fatalf("Marshal(ChannelData{}) error = %v", err)
	}
	var live map[string]json.RawMessage
	if err := json.Unmarshal(b, &live); err != nil {
		t.Fatalf("Unmarshal into map error = %v", err)
	}
	frozen, err := json.Marshal(legacyChannelData{})
	if err != nil {
		t.Fatalf("Marshal(legacyChannelData{}) error = %v", err)
	}
	var old map[string]json.RawMessage
	if err := json.Unmarshal(frozen, &old); err != nil {
		t.Fatalf("Unmarshal frozen into map error = %v", err)
	}
	// omitempty means neither marshal emits every key, so probe the frozen
	// DECODER directly with each live key instead of comparing key sets.
	for key := range live {
		var probe legacyChannelData
		dec := json.NewDecoder(strings.NewReader(fmt.Sprintf(`{%q:null}`, key)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&probe); err != nil {
			t.Errorf("legacyChannelData cannot decode key %q (%v) — a valid legacy file carrying it would be REJECTED", key, err)
		}
	}
	if len(old) == 0 && len(live) == 0 {
		t.Fatal("both marshals produced no keys at all; the probe above proved nothing")
	}
}
