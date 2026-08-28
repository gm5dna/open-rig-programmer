// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// fullCodeplug builds a Codeplug with every field populated, for
// round-trip testing.
//
// Its channels' ten tier-added fields are set to Unavailable
// (withUnavailableTierFields) rather than left at their zero value,
// because that is what every REAL producer of a Yaesu channel yields —
// a driver read, and a load of any schema-1/2/3 file — and this fixture
// is round-tripped through Save and Load, which normalise to exactly
// that. Leaving them zero would have made the fixture the only
// ChannelData in the project that says "nothing was ever said about
// these fields" about an FT-710.
func fullCodeplug() *Codeplug {
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "open-rig-programmer v0.1.0",
		Radio: RadioInfo{
			Model:             "FT-710",
			CATID:             "0800",
			ReadAt:            time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
			Port:              "/dev/cu.usbserial-1234",
			USBSerial:         "AB12CD34",
			FirmwareConfirmed: "1.06",
			Region:            "UK",
			BaselineDigest:    "deadbeef",
		},
		Channels: []Channel{
			{Slot: "001", Data: &ChannelData{
				FreqHz:     14250000,
				Mode:       "USB",
				ClarHz:     10,
				RxClar:     true,
				CTCSS:      "ENC-DEC",
				CTCSSTone:  ToneField{State: Known, Value: spec.Tone(885)},
				Shift:      "PLUS",
				Tag:        "MB9XYZ",
				TagDisplay: BoolField{State: Known, Value: true},
				ScanSkip:   BoolField{State: Known, Value: true},
			}},
			{Slot: "002"}, // empty
		},
		Menus: &MenuSnapshot{
			Descriptor: "ft710-ex@1",
			Complete:   true,
			Entries: []MenuEntry{
				{ID: "000101", Value: "3", State: MenuKnown},
				{ID: "000205", Value: "USB", State: MenuKnown},
			},
		},
	}
	for i := range cp.Channels {
		if cp.Channels[i].Data != nil {
			withUnavailableTierFields(cp.Channels[i].Data)
		}
	}
	return cp
}

// TestSaveLoad_V2RoundTrip checks a lossless round trip of a fully
// populated schema-2 codeplug, including a typed MenuSnapshot.
func TestSaveLoad_V2RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rop.json")

	want := fullCodeplug()
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Schema != want.Schema {
		t.Errorf("Schema = %d, want %d", got.Schema, want.Schema)
	}
	if got.Generator != want.Generator {
		t.Errorf("Generator = %q, want %q", got.Generator, want.Generator)
	}
	gotRadio := got.Radio
	wantRadio := want.Radio
	if !gotRadio.ReadAt.Equal(wantRadio.ReadAt) {
		t.Errorf("Radio.ReadAt = %v, want %v", gotRadio.ReadAt, wantRadio.ReadAt)
	}
	gotRadio.ReadAt = wantRadio.ReadAt
	if gotRadio != wantRadio {
		t.Errorf("Radio = %+v, want %+v", gotRadio, wantRadio)
	}
	if len(got.Channels) != len(want.Channels) {
		t.Fatalf("len(Channels) = %d, want %d", len(got.Channels), len(want.Channels))
	}
	for i := range want.Channels {
		if got.Channels[i].Slot != want.Channels[i].Slot {
			t.Errorf("Channels[%d].Slot = %q, want %q", i, got.Channels[i].Slot, want.Channels[i].Slot)
		}
		if want.Channels[i].Data == nil {
			if got.Channels[i].Data != nil {
				t.Errorf("Channels[%d].Data = %+v, want nil", i, got.Channels[i].Data)
			}
			continue
		}
		if got.Channels[i].Data == nil || *got.Channels[i].Data != *want.Channels[i].Data {
			t.Errorf("Channels[%d].Data = %+v, want %+v", i, got.Channels[i].Data, want.Channels[i].Data)
		}
	}
	if got.Menus == nil {
		t.Fatalf("Menus = nil, want the typed snapshot preserved")
	}
	if got.Menus.Descriptor != want.Menus.Descriptor {
		t.Errorf("Menus.Descriptor = %q, want %q", got.Menus.Descriptor, want.Menus.Descriptor)
	}
	if got.Menus.Complete != want.Menus.Complete {
		t.Errorf("Menus.Complete = %v, want %v", got.Menus.Complete, want.Menus.Complete)
	}
	if !reflect.DeepEqual(got.Menus.Entries, want.Menus.Entries) {
		t.Errorf("Menus.Entries = %+v, want %+v", got.Menus.Entries, want.Menus.Entries)
	}
}

// TestSave_EndsWithNewline checks that the written file ends with a
// trailing newline.
func TestSave_EndsWithNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rop.json")

	if err := Save(path, fullCodeplug()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Errorf("saved file does not end with a newline")
	}
}

// TestSave_Permissions0600 checks that a saved file has mode 0600 — a
// deliberate, privacy-lean default, since a codeplug can carry callsigns.
func TestSave_Permissions0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rop.json")

	if err := Save(path, fullCodeplug()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := fi.Mode().Perm(); got != 0600 {
		t.Errorf("file mode = %o, want 0600", got)
	}
}

// TestSave_AtomicNoTempFileRemains checks that after Save overwrites an
// existing file, the directory contains only the final file — no
// leftover temp file from the write.
func TestSave_AtomicNoTempFileRemains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rop.json")

	if err := Save(path, fullCodeplug()); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	cp2 := fullCodeplug()
	cp2.Generator = "second generator"
	if err := Save(path, cp2); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("directory has %d entries, want 1: %v", len(entries), names)
	}
	if entries[0].Name() != filepath.Base(path) {
		t.Errorf("directory entry = %q, want %q", entries[0].Name(), filepath.Base(path))
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Generator != "second generator" {
		t.Errorf("Generator = %q, want %q (second Save should have overwritten the file)", got.Generator, "second generator")
	}
}

// TestLoad_MissingFile checks Load's error path for a nonexistent file.
func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

// TestLoad_InvalidJSON checks Load's error path for a file that is not
// valid JSON at all.
func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

// TestLoad_CorruptedTruncatedJSON checks Load's error path for JSON that
// is truncated mid-object — it must return a typed error, not panic.
func TestLoad_CorruptedTruncatedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "truncated.json")
	full := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[`
	if err := os.WriteFile(path, []byte(full), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Load() panicked on truncated JSON: %v", r)
		}
	}()
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

// minimalCodeplugBody is a well-formed, minimal codeplug JSON document
// used as the base for the strict-decoding tests below: valid schema,
// radio, and one populated channel with every field present.
const minimalCodeplugBody = `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","scan_skip":{"state":"unknown"}}}]}`

// TestLoad_TrimsTrailingSpacesFromTag pins the JSON-file mirror of
// cat.Dialect.ParseMTAnswer's trim fix: a LEGACY file (written before
// this normalisation existed, or hand-edited from one) may still carry
// a space-padded tag exactly as a pre-fix Read left it — Load must trim
// it on the way in, rather than perpetuating the false verify-mismatch
// this whole fix exists to prevent. Two channels: an ordinary padded
// tag, and an all-spaces tag (the radio's own tag-CLEAR form), which
// must trim to "" — matching "no tag", not a 12-space tag.
func TestLoad_TrimsTrailingSpacesFromTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy-padded-tag.json")
	body := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[` +
		`{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","tag":"BBC ANT 3   ","scan_skip":{"state":"unknown"}}},` +
		`{"slot":"002","data":{"freq_hz":7100000,"mode":"LSB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","tag":"            ","scan_skip":{"state":"unknown"}}}` +
		`]}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Channels) != 2 {
		t.Fatalf("len(Channels) = %d, want 2", len(got.Channels))
	}
	if tag := got.Channels[0].Data.Tag; tag != "BBC ANT 3" {
		t.Errorf("Channels[0].Data.Tag = %q, want %q (trailing padding trimmed)", tag, "BBC ANT 3")
	}
	if tag := got.Channels[1].Data.Tag; tag != "" {
		t.Errorf("Channels[1].Data.Tag = %q, want \"\" (all-spaces legacy tag trims to no-tag)", tag)
	}
}

// TestLoad_TrimmedTagPassesValidate directly proves the adjudicated
// invariant end to end: Validate's tag byte-length limit applies to the
// TRIMMED form, not the raw file bytes. This legacy tag is 12
// significant bytes PLUS 3 bytes of trailing-space padding (15 raw
// bytes — over testCapabilities().TagLen, 12): pre-fix this would fail
// Validate as "exceeds this radio's maximum of 12" even though the real,
// significant content fits exactly. Load's trim (this fix) makes it
// pass cleanly instead.
func TestLoad_TrimmedTagPassesValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "padded-full-length-tag.json")
	body := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[` +
		`{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","tag":"123456789012   ","scan_skip":{"state":"unknown"}}}` +
		`]}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tag := got.Channels[0].Data.Tag; tag != "123456789012" {
		t.Fatalf("Tag = %q, want %q (trimmed to the 12 significant bytes)", tag, "123456789012")
	}

	for _, is := range Validate(got, testCapabilities()) {
		if is.Field == spec.FieldTag {
			t.Errorf("Validate found a tag issue on a trimmed 12-byte tag: %+v", is)
		}
	}
}

// TestLoad_UnknownFieldRejected checks that a hand-edit misspelling a
// known key (e.g. "rx_clar" -> "rx_clarr") is rejected with a
// *UnknownFieldError naming the offending key, rather than silently
// defaulting the real field (RxClar) to its zero value.
func TestLoad_UnknownFieldRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	body := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[{"slot":"001","data":{"freq_hz":14250000,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","scan_skip":{"state":"unknown"},"rx_clarr":true}}]}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil for a misspelled field")
	}
	var ufe *UnknownFieldError
	if !errors.As(err, &ufe) {
		t.Fatalf("errors.As(err, *UnknownFieldError) = false, want true (err = %v)", err)
	}
	if ufe.Field != "rx_clarr" {
		t.Errorf("UnknownFieldError.Field = %q, want %q", ufe.Field, "rx_clarr")
	}
}

// TestLoad_UnknownTopLevelKeyRejected checks that an extra, unrecognised
// key at the top level of the file is rejected the same way.
func TestLoad_UnknownTopLevelKeyRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	body := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[],"bogus_extra_field":true}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil for an unknown top-level key")
	}
	var ufe *UnknownFieldError
	if !errors.As(err, &ufe) {
		t.Fatalf("errors.As(err, *UnknownFieldError) = false, want true (err = %v)", err)
	}
	if ufe.Field != "bogus_extra_field" {
		t.Errorf("UnknownFieldError.Field = %q, want %q", ufe.Field, "bogus_extra_field")
	}
}

// TestLoad_DuplicateKeyRejected checks that a duplicate "freq_hz" key
// inside one channel's data object is rejected with a
// *DuplicateKeyError naming the key, rather than silently resolved
// "last wins" by encoding/json.
func TestLoad_DuplicateKeyRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	body := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[{"slot":"001","data":{"freq_hz":14250000,"freq_hz":99999999,"mode":"USB","ctcss":"OFF","ctcss_tone":{"state":"unknown"},"shift":"SIMPLEX","scan_skip":{"state":"unknown"}}}]}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil for a duplicate key")
	}
	var dke *DuplicateKeyError
	if !errors.As(err, &dke) {
		t.Fatalf("errors.As(err, *DuplicateKeyError) = false, want true (err = %v)", err)
	}
	if dke.Key != "freq_hz" {
		t.Errorf("DuplicateKeyError.Key = %q, want %q", dke.Key, "freq_hz")
	}
}

// TestLoad_V1OpaqueMenus_PreservedAsLegacy is the successor to the old
// TestLoad_MenusArbitraryNestedKeysStillLoads: a schema-1 file's arbitrary
// (even duplicate-keyed) "menus" subtree must migrate to a v2 MenuSnapshot
// whose Legacy carries the ORIGINAL bytes verbatim. The check is
// byte-level, NOT decoded-value-equal: json.Unmarshal resolves duplicate
// keys last-wins, so we compare json.Compact of the raw legacy bytes and
// assert the duplicate key "ex01" survives with BOTH occurrences, in order.
func TestLoad_V1OpaqueMenus_PreservedAsLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	body := `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[],"menus":{"ex01":3,"ex01":5,"nested":{"a":1,"a":2},"whatever_arbitrary_key":true}}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (menus subtree is exempt)", err)
	}
	if got.Schema != CurrentSchema {
		t.Errorf("Schema = %d, want %d (migrate-on-load bumps to current)", got.Schema, CurrentSchema)
	}
	if got.Menus == nil {
		t.Fatalf("Menus = nil, want a MenuSnapshot carrying the legacy bytes")
	}
	if len(got.Menus.Legacy) == 0 {
		t.Fatalf("Menus.Legacy is empty, want the raw v1 menus JSON preserved")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, got.Menus.Legacy); err != nil {
		t.Fatalf("json.Compact(Menus.Legacy) error = %v", err)
	}
	want := `{"ex01":3,"ex01":5,"nested":{"a":1,"a":2},"whatever_arbitrary_key":true}`
	if compact.String() != want {
		t.Errorf("compacted Menus.Legacy = %s, want %s (both duplicate ex01 occurrences must survive, in order)", compact.String(), want)
	}
}

// TestLoad_TrailingDataRejected checks that extra data after the
// top-level JSON value is rejected rather than silently ignored.
func TestLoad_TrailingDataRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	body := minimalCodeplugBody + `{"extra":"object"}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil for trailing data after the top-level value")
	}
}

// TestLoad_Oversize checks that Load rejects a file larger than
// maxCodeplugFileSize with a typed *OversizeFileError, WITHOUT actually
// writing maxCodeplugFileSize bytes of real data: Truncate extends the
// file's logical length (what os.Stat reports) without writing any
// content, so this test stays fast and light regardless of the limit's
// size.
func TestLoad_Oversize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.json")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := f.Truncate(maxCodeplugFileSize + 1); err != nil {
		f.Close()
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil for an oversize file")
	}
	var oe *OversizeFileError
	if !errors.As(err, &oe) {
		t.Fatalf("errors.As(err, *OversizeFileError) = false, want true (err = %v)", err)
	}
}

// TestLoad_UnderLimitStillLoads is a boundary check: a file at (or
// under) maxCodeplugFileSize must not be rejected for size alone. Uses
// the existing minimalCodeplugBody, which is nowhere near the limit —
// this only pins that the size gate does not misfire on an ordinary
// small file.
func TestLoad_UnderLimitStillLoads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.json")
	if err := os.WriteFile(path, []byte(minimalCodeplugBody), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v, want nil for a well-under-limit file", err)
	}
}

// TestLoad_SchemaErrors is table-driven over the Schema-related failure
// modes: 0, negative, and greater than CurrentSchema.
func TestLoad_SchemaErrors(t *testing.T) {
	cases := []struct {
		name          string
		schema        int
		wantTooNew    bool
		wantSchemaErr bool
	}{
		{"schema zero", 0, false, true},
		{"schema negative", -1, false, true},
		{"schema too new", CurrentSchema + 1, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.json")
			body := `{"schema":` + strconv.Itoa(tc.schema) + `,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[]}`
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			_, err := Load(path)
			if err == nil {
				t.Fatal("Load() error = nil, want non-nil")
			}
			if tc.wantSchemaErr {
				var se *SchemaError
				if !errors.As(err, &se) {
					t.Errorf("errors.As(err, *SchemaError) = false, want true (err = %v)", err)
				}
			}
			if got := errors.Is(err, ErrSchemaTooNew); got != tc.wantTooNew {
				t.Errorf("errors.Is(err, ErrSchemaTooNew) = %v, want %v", got, tc.wantTooNew)
			}
			if tc.wantTooNew && !strings.Contains(err.Error(), "upgrade") {
				t.Errorf("error message %q does not mention upgrading the app", err.Error())
			}
		})
	}
}

// v1Body builds a schema-1 codeplug JSON document with an empty channel
// list and the given raw "menus" clause (e.g. `,"menus":null`, or "" for
// no menus key at all).
func v1Body(menusClause string) string {
	return `{"schema":1,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[]` + menusClause + `}`
}

// v2Body is v1Body's schema-2 sibling.
func v2Body(menusClause string) string {
	return `{"schema":2,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[]` + menusClause + `}`
}

// writeAndLoad writes body to a temp file and returns Load's result.
func writeAndLoad(t *testing.T, body string) (*Codeplug, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return Load(path)
}

// compactBytes returns the compact form of raw for byte-level comparison
// that is insensitive to Save's re-indentation but not to key order or
// duplicate keys.
func compactBytes(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("json.Compact error = %v", err)
	}
	return buf.String()
}

// TestLoad_V1MissingMenus_MigratesToNil: a schema-1 file with no "menus"
// key migrates to a schema-2 codeplug whose Menus is nil.
func TestLoad_V1MissingMenus_MigratesToNil(t *testing.T) {
	got, err := writeAndLoad(t, v1Body(""))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Schema != CurrentSchema {
		t.Errorf("Schema = %d, want %d", got.Schema, CurrentSchema)
	}
	if got.Menus != nil {
		t.Errorf("Menus = %+v, want nil", got.Menus)
	}
}

// TestLoad_V1NullMenus_MigratesToNil: a schema-1 file with "menus":null
// migrates to a schema-2 codeplug whose Menus is nil (a literal null is
// treated identically to an absent key).
func TestLoad_V1NullMenus_MigratesToNil(t *testing.T) {
	got, err := writeAndLoad(t, v1Body(`,"menus":null`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Schema != CurrentSchema {
		t.Errorf("Schema = %d, want %d", got.Schema, CurrentSchema)
	}
	if got.Menus != nil {
		t.Errorf("Menus = %+v, want nil", got.Menus)
	}
}

// TestLoad_V1EmptyObjectMenus_PreservedNotNil: a schema-1 file with
// "menus":{} migrates to a NON-nil MenuSnapshot whose Legacy is the empty
// object, verbatim — predictability over cleverness: {} is preserved, not
// collapsed to nil.
func TestLoad_V1EmptyObjectMenus_PreservedNotNil(t *testing.T) {
	got, err := writeAndLoad(t, v1Body(`,"menus":{}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Menus == nil {
		t.Fatalf("Menus = nil, want a non-nil MenuSnapshot preserving {}")
	}
	if c := compactBytes(t, got.Menus.Legacy); c != `{}` {
		t.Errorf("Menus.Legacy = %s, want {}", c)
	}
}

// TestLoad_V1Opaque_RoundTripsThroughSave: Load a schema-1 opaque menus
// file, Save it, Load it again — the result must be schema 2 with the
// legacy bytes json.Compact-equal to the original, duplicates intact.
func TestLoad_V1Opaque_RoundTripsThroughSave(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	body := v1Body(`,"menus":{"ex01":3,"ex01":5,"nested":{"a":1}}`)
	if err := os.WriteFile(src, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	first, err := Load(src)
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	dst := filepath.Join(dir, "dst.json")
	if err := Save(dst, first); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	second, err := Load(dst)
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}

	if second.Schema != CurrentSchema {
		t.Errorf("Schema = %d, want %d", second.Schema, CurrentSchema)
	}
	if second.Menus == nil {
		t.Fatalf("Menus = nil after round trip, want legacy preserved")
	}
	want := `{"ex01":3,"ex01":5,"nested":{"a":1}}`
	if c := compactBytes(t, second.Menus.Legacy); c != want {
		t.Errorf("round-tripped Menus.Legacy = %s, want %s (both ex01 keys survive)", c, want)
	}
}

// TestSaveLoad_V2NativeLegacy_RoundTripsBytes: a native v2 snapshot whose
// Legacy carries duplicate JSON keys round-trips byte-preserved through
// Save/Load — the narrowed exemption keeps menus.legacy an opaque bag.
func TestSaveLoad_V2NativeLegacy_RoundTripsBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "x",
		Radio:     RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []Channel{},
		Menus: &MenuSnapshot{
			Descriptor: "ft710-ex@1",
			Complete:   false,
			Entries:    []MenuEntry{{ID: "000101", Value: "3", State: MenuKnown}},
			Legacy:     json.RawMessage(`{"raw01":1,"raw01":2}`),
		},
	}
	if err := Save(path, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Menus == nil {
		t.Fatalf("Menus = nil, want the native snapshot preserved")
	}
	if c := compactBytes(t, got.Menus.Legacy); c != `{"raw01":1,"raw01":2}` {
		t.Errorf("Menus.Legacy = %s, want both raw01 keys preserved in order", c)
	}
}

// TestLoad_V2Typed: a schema-2 file with entries in all three states
// decodes to the exact MenuEntry structs.
func TestLoad_V2Typed(t *testing.T) {
	body := v2Body(`,"menus":{"descriptor":"ft710-ex@1","complete":false,"entries":[` +
		`{"id":"000101","value":"3","state":"known"},` +
		`{"id":"000202","state":"unavailable"},` +
		`{"id":"000303","value":"7","state":"unsupported"}]}`)
	got, err := writeAndLoad(t, body)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Menus == nil {
		t.Fatal("Menus = nil, want the typed snapshot")
	}
	if got.Menus.Descriptor != "ft710-ex@1" || got.Menus.Complete {
		t.Errorf("Menus header = {Descriptor:%q Complete:%v}, want {ft710-ex@1 false}", got.Menus.Descriptor, got.Menus.Complete)
	}
	want := []MenuEntry{
		{ID: "000101", Value: "3", State: MenuKnown},
		{ID: "000202", State: MenuUnavailable},
		{ID: "000303", Value: "7", State: MenuUnsupported},
	}
	if !reflect.DeepEqual(got.Menus.Entries, want) {
		t.Errorf("Menus.Entries = %+v, want %+v", got.Menus.Entries, want)
	}
}

// TestLoad_V2DuplicateMenuID_Rejected: two entries with the same ID are
// rejected with a *DuplicateMenuIDError.
func TestLoad_V2DuplicateMenuID_Rejected(t *testing.T) {
	body := v2Body(`,"menus":{"descriptor":"x","complete":false,"entries":[` +
		`{"id":"000101","value":"3","state":"known"},` +
		`{"id":"000101","value":"5","state":"known"}]}`)
	_, err := writeAndLoad(t, body)
	if err == nil {
		t.Fatal("Load() error = nil, want *DuplicateMenuIDError")
	}
	var dme *DuplicateMenuIDError
	if !errors.As(err, &dme) {
		t.Fatalf("errors.As(err, *DuplicateMenuIDError) = false (err = %v)", err)
	}
	if dme.ID != "000101" {
		t.Errorf("DuplicateMenuIDError.ID = %q, want %q", dme.ID, "000101")
	}
}

// TestLoad_V2MenusDuplicateJSONKey_Rejected: a duplicate JSON key directly
// inside the "menus" object (here "complete") is rejected — the exemption
// is narrowed to only menus.legacy, so menus itself is now policed.
func TestLoad_V2MenusDuplicateJSONKey_Rejected(t *testing.T) {
	body := v2Body(`,"menus":{"descriptor":"x","complete":false,"complete":true,"entries":[]}`)
	_, err := writeAndLoad(t, body)
	if err == nil {
		t.Fatal("Load() error = nil, want *DuplicateKeyError for a duplicate key inside menus")
	}
	var dke *DuplicateKeyError
	if !errors.As(err, &dke) {
		t.Fatalf("errors.As(err, *DuplicateKeyError) = false (err = %v)", err)
	}
	if dke.Key != "complete" {
		t.Errorf("DuplicateKeyError.Key = %q, want %q", dke.Key, "complete")
	}
}

// TestLoad_V2LegacySubtreeDuplicateKeys_StillExempt: duplicate keys INSIDE
// menus.legacy remain exempt from the duplicate-key walk.
func TestLoad_V2LegacySubtreeDuplicateKeys_StillExempt(t *testing.T) {
	body := v2Body(`,"menus":{"descriptor":"x","complete":false,"entries":[],"legacy":{"dup":1,"dup":2}}`)
	got, err := writeAndLoad(t, body)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (menus.legacy is exempt)", err)
	}
	if got.Menus == nil {
		t.Fatal("Menus = nil, want legacy preserved")
	}
	if c := compactBytes(t, got.Menus.Legacy); c != `{"dup":1,"dup":2}` {
		t.Errorf("Menus.Legacy = %s, want both dup keys preserved", c)
	}
}

// TestLoad_V2UnknownFieldInMenus_Rejected: an unrecognised key inside the
// typed menus object (not menus.legacy) is rejected with an
// *UnknownFieldError.
func TestLoad_V2UnknownFieldInMenus_Rejected(t *testing.T) {
	body := v2Body(`,"menus":{"descriptor":"x","complete":false,"entries":[],"bogus":1}`)
	_, err := writeAndLoad(t, body)
	if err == nil {
		t.Fatal("Load() error = nil, want *UnknownFieldError")
	}
	var ufe *UnknownFieldError
	if !errors.As(err, &ufe) {
		t.Fatalf("errors.As(err, *UnknownFieldError) = false (err = %v)", err)
	}
	if ufe.Field != "bogus" {
		t.Errorf("UnknownFieldError.Field = %q, want %q", ufe.Field, "bogus")
	}
}

// TestLoad_V2EntryShape_Rejected exercises each per-entry consistency rule
// in isolation; every case must surface a *MenuEntryError.
func TestLoad_V2EntryShape_Rejected(t *testing.T) {
	cases := []struct {
		name  string
		menus string
	}{
		{"5-digit id", `,"menus":{"descriptor":"x","complete":false,"entries":[{"id":"00010","value":"3","state":"known"}]}`},
		{"non-digit id", `,"menus":{"descriptor":"x","complete":false,"entries":[{"id":"0001A1","value":"3","state":"known"}]}`},
		{"unknown state", `,"menus":{"descriptor":"x","complete":false,"entries":[{"id":"000101","value":"3","state":"bogus"}]}`},
		{"unavailable with value", `,"menus":{"descriptor":"x","complete":false,"entries":[{"id":"000101","value":"3","state":"unavailable"}]}`},
		{"known with empty value", `,"menus":{"descriptor":"x","complete":false,"entries":[{"id":"000101","state":"known"}]}`},
		{"complete true with unavailable entry", `,"menus":{"descriptor":"x","complete":true,"entries":[{"id":"000101","state":"unavailable"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := writeAndLoad(t, v2Body(tc.menus))
			if err == nil {
				t.Fatal("Load() error = nil, want *MenuEntryError")
			}
			var mee *MenuEntryError
			if !errors.As(err, &mee) {
				t.Fatalf("errors.As(err, *MenuEntryError) = false (err = %v)", err)
			}
		})
	}
}

// TestSave_InvalidSnapshotRejected: Save refuses an invalid snapshot and
// writes nothing at the target path.
func TestSave_InvalidSnapshotRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "x",
		Radio:     RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []Channel{},
		Menus: &MenuSnapshot{
			Descriptor: "x",
			Entries:    []MenuEntry{{ID: "000101", State: MenuKnown}}, // known with empty value
		},
	}
	err := Save(path, cp)
	if err == nil {
		t.Fatal("Save() error = nil, want refusal of an invalid snapshot")
	}
	var mee *MenuEntryError
	if !errors.As(err, &mee) {
		t.Fatalf("errors.As(err, *MenuEntryError) = false (err = %v)", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("target file exists after a refused Save (stat err = %v), want absent", statErr)
	}
}

// TestLoad_SchemaTooNew_VersionFirst pins the pass order: a schema newer
// than CurrentSchema that ALSO carries an unknown top-level key must be
// reported as ErrSchemaTooNew (upgrade the app), NOT as an
// *UnknownFieldError from a premature strict decode.
func TestLoad_SchemaTooNew_VersionFirst(t *testing.T) {
	// CurrentSchema+1, not a literal: this test is about the PASS ORDER,
	// not about any particular version number, and a literal went stale
	// the moment the Icom tier moved CurrentSchema from 3 to 4.
	body := `{"schema":` + strconv.Itoa(CurrentSchema+1) + `,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[],"bogus_extra_field":true}`
	_, err := writeAndLoad(t, body)
	if err == nil {
		t.Fatal("Load() error = nil, want ErrSchemaTooNew")
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("errors.Is(err, ErrSchemaTooNew) = false (err = %v)", err)
	}
	var ufe *UnknownFieldError
	if errors.As(err, &ufe) {
		t.Errorf("errors.As(err, *UnknownFieldError) = true, want false (version check must precede strict decode)")
	}
}

// TestLoad_SchemaZeroOrNegative_StillInvalid re-pins, post-refactor, that
// a 0 or negative schema is a *SchemaError that is NOT ErrSchemaTooNew.
func TestLoad_SchemaZeroOrNegative_StillInvalid(t *testing.T) {
	for _, schema := range []int{0, -1} {
		t.Run(strconv.Itoa(schema), func(t *testing.T) {
			body := `{"schema":` + strconv.Itoa(schema) + `,"generator":"x","radio":{"model":"FT-710","cat_id":"0800","read_at":"2026-07-10T12:00:00Z"},"channels":[]}`
			_, err := writeAndLoad(t, body)
			if err == nil {
				t.Fatal("Load() error = nil, want *SchemaError")
			}
			var se *SchemaError
			if !errors.As(err, &se) {
				t.Fatalf("errors.As(err, *SchemaError) = false (err = %v)", err)
			}
			if errors.Is(err, ErrSchemaTooNew) {
				t.Errorf("errors.Is(err, ErrSchemaTooNew) = true, want false for schema %d", schema)
			}
		})
	}
}

// trickyLegacy is a Legacy payload exercising every byte json.MarshalIndent's
// default HTML escaping would have silently rewritten, violating the
// verbatim-Legacy contract (Codex M8b #2): literal < > &, literal
// U+2028/U+2029 (valid inside a JSON string, only awkward for JavaScript),
// a \uXXXX escape that must stay a literal escape (never be re-encoded),
// ordinary non-ASCII (é as raw UTF-8), and duplicate keys (the opaque bag
// permits them).
const trickyLegacy = "{" +
	"\"lt\":\"a<b\",\"gt\":\"a>b\",\"amp\":\"a&b\"," +
	"\"sep\":\"x\u2028y\u2029z\"," + // literal U+2028 / U+2029
	"\"esc\":\"\\u00e9\"," + // a literal \uXXXX escape (the backslash-u must survive)
	"\"uni\":\"caf\u00e9\"," + // ordinary non-ASCII (é as raw UTF-8)
	"\"dup\":1,\"dup\":2" + // duplicate keys — the legacy bag permits them
	"}"

// assertLegacyVerbatim re-loads the codeplug Save wrote to path and checks
// its Menus.Legacy is byte-identical to want under json.Compact (order and
// duplicates intact, whitespace-insensitive), AND that the raw saved file
// kept literal < > & and U+2028/U+2029 un-escaped — the exact bytes
// MarshalIndent's HTML escaping would have rewritten.
func assertLegacyVerbatim(t *testing.T, path, want string) {
	t.Helper()
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load(saved) error = %v", err)
	}
	if got.Menus == nil {
		t.Fatal("Menus = nil after Save, want the legacy payload preserved")
	}
	if gc, wc := compactBytes(t, got.Menus.Legacy), compactBytes(t, json.RawMessage(want)); gc != wc {
		t.Errorf("round-tripped Menus.Legacy = %q, want %q (bytes must survive Save verbatim)", gc, wc)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`, `\u2028`, `\u2029`} {
		if bytes.Contains(raw, []byte(escaped)) {
			t.Errorf("saved file contains HTML-escaped %s, want the literal byte (verbatim contract)", escaped)
		}
	}
	if !bytes.Contains(raw, []byte("a<b")) || !bytes.Contains(raw, []byte("a>b")) || !bytes.Contains(raw, []byte("a&b")) {
		t.Error("saved file lost a literal <, > or & from the legacy payload")
	}
}

// TestSave_LegacyBytesSurviveVerbatim_NativeV2 (Codex M8b #2): a natively
// written schema-2 snapshot whose Legacy carries HTML-escapable and
// non-ASCII bytes round-trips through Save byte-for-byte.
func TestSave_LegacyBytesSurviveVerbatim_NativeV2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "native.json")
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "x",
		Radio:     RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []Channel{},
		Menus:     &MenuSnapshot{Legacy: json.RawMessage(trickyLegacy)},
	}
	if err := Save(path, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertLegacyVerbatim(t, path, trickyLegacy)
}

// TestSave_LegacyBytesSurviveVerbatim_V1Migrated (Codex M8b #2): the same
// verbatim guarantee for a MIGRATED v1 opaque "menus" payload — Load a v1
// file carrying the tricky bytes, Save it as schema 2, and the bytes must
// still be literal (not HTML-escaped) in the output.
func TestSave_LegacyBytesSurviveVerbatim_V1Migrated(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	if err := os.WriteFile(src, []byte(v1Body(`,"menus":`+trickyLegacy)), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	migrated, err := Load(src)
	if err != nil {
		t.Fatalf("Load(v1) error = %v", err)
	}
	dst := filepath.Join(dir, "dst.json")
	if err := Save(dst, migrated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertLegacyVerbatim(t, dst, trickyLegacy)
}

// TestSave_NearBound_SavesAndReloads (Codex M8b #3): a codeplug whose
// encoded output lands just UNDER maxCodeplugFileSize saves fine and
// re-loads fine — the size guard must not reject a legitimately large file.
func TestSave_NearBound_SavesAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "near.json")
	// A single JSON string sized so the fully-encoded, indented file lands
	// just under the bound (the wrapper/indentation overhead is a few
	// hundred bytes; 4 KiB of headroom is comfortably clear of it).
	big := strings.Repeat("a", maxCodeplugFileSize-4096)
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "x",
		Radio:     RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []Channel{},
		Menus:     &MenuSnapshot{Legacy: json.RawMessage(`"` + big + `"`)},
	}
	if err := Save(path, cp); err != nil {
		t.Fatalf("Save(near-bound) error = %v, want success", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	if fi.Size() > maxCodeplugFileSize {
		t.Fatalf("saved file = %d bytes, want <= %d (a near-bound file must stay Load-readable)", fi.Size(), maxCodeplugFileSize)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load(near-bound) error = %v, want the near-bound file to re-load fine", err)
	}
}

// TestSave_OverBound_RefusedDestinationUntouched (Codex M8b #3): a codeplug
// whose encoded output exceeds maxCodeplugFileSize is refused with a typed
// *OversizeSaveError BEFORE any temp file is created — an existing
// destination survives byte-for-byte, and no stray temp file is left.
func TestSave_OverBound_RefusedDestinationUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	const sentinel = `{"schema":2,"pre-existing":true}`
	if err := os.WriteFile(path, []byte(sentinel), 0600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	big := strings.Repeat("a", maxCodeplugFileSize+4096)
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "x",
		Radio:     RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []Channel{},
		Menus:     &MenuSnapshot{Legacy: json.RawMessage(`"` + big + `"`)},
	}
	err := Save(path, cp)
	if err == nil {
		t.Fatal("Save(over-bound) error = nil, want *OversizeSaveError")
	}
	var oe *OversizeSaveError
	if !errors.As(err, &oe) {
		t.Fatalf("errors.As(err, *OversizeSaveError) = false (err = %v)", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(got) != sentinel {
		t.Errorf("destination changed after a refused Save = %q, want the pre-existing %q untouched", got, sentinel)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want 1 (no temp file left behind by a refused Save)", len(entries))
	}
}
