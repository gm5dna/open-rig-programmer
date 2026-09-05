// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"path/filepath"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// --- FT-991A Stage 0 (S0.4): a five-state CTCSS vocabulary ---
//
// spec.ToneSemantics gained two DCS members so a radio whose P8 legend
// prints five states can declare a vocabulary Validate accepts. This
// package needs no change for that — ChannelData.CTCSS is an opaque string
// and the vocabulary check reads caps' own list — and these pins are what
// says so rather than leaving it assumed.
//
// A TEST FIXTURE, not a registered model: no radio declares this vocabulary
// until Stage 2.

// fiveStateCapabilities is testCapabilities() with the FT-991A's P8 legend
// in place of the family three.
func fiveStateCapabilities() spec.Capabilities {
	caps := testCapabilities()
	caps.CTCSSStates = []spec.ToneState{
		{Value: "OFF", Semantics: spec.ToneOff},
		{Value: "ENC-DEC", Semantics: spec.ToneEncodeDecode},
		{Value: "ENC", Semantics: spec.ToneEncode},
		{Value: "DCS-ENC-DEC", Semantics: spec.ToneDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: spec.ToneDCSEncode},
	}
	return caps
}

// TestValidate_AcceptsADCSStateChannel is the point of the widening seen
// from this side: a channel in a DCS state is valid, and — because
// RequiresTone was deliberately NOT widened — it raises NO tone warning
// even though it carries no known CTCSS tone. A DCS state needs a code,
// and the code is not a field of this record.
func TestValidate_AcceptsADCSStateChannel(t *testing.T) {
	caps := fiveStateCapabilities()
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "test",
		Radio:     RadioInfo{Model: caps.Model},
		Channels: []Channel{
			{Slot: "001", Data: &ChannelData{
				FreqHz: 145500000, Mode: "FM", CTCSS: "DCS-ENC-DEC", Shift: "SIMPLEX",
			}},
		},
	}

	for _, issue := range Validate(cp, caps) {
		if issue.Field == spec.FieldCTCSSState {
			t.Errorf("Validate flagged a declared DCS state: %s", issue.Msg)
		}
		if issue.Field == spec.FieldCTCSSTone && issue.Severity == SeverityWarning {
			t.Errorf("Validate warned that a DCS state's TONE cannot be set: %s — a DCS state needs a CODE, not a tone, and RequiresTone is deliberately false for it", issue.Msg)
		}
	}

	// And the vocabulary is still a vocabulary: a state outside it fails.
	cp.Channels[0].Data.CTCSS = "DCS-TSQL"
	found := false
	for _, issue := range Validate(cp, caps) {
		if issue.Field == spec.FieldCTCSSState && issue.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Error("Validate accepted a CTCSS state outside caps' own five-member vocabulary")
	}
}

// TestSaveLoad_PreservesADCSState is the file round trip. ChannelData.CTCSS
// is an opaque string on disk, so the widening changes no schema — this
// pins that it does not.
func TestSaveLoad_PreservesADCSState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dcs.json")
	want := "DCS-ENC-DEC"
	cp := &Codeplug{
		Schema:    CurrentSchema,
		Generator: "test",
		Radio:     RadioInfo{Model: "TEST-710"},
		Channels: []Channel{
			{Slot: "001", Data: &ChannelData{
				FreqHz: 145500000, Mode: "FM", CTCSS: want, Shift: "SIMPLEX",
			}},
			{Slot: "002", Data: &ChannelData{
				FreqHz: 145600000, Mode: "FM", CTCSS: "DCS-ENC", Shift: "SIMPLEX",
			}},
		},
	}
	if err := Save(path, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(back.Channels) != 2 || back.Channels[0].Data == nil || back.Channels[1].Data == nil {
		t.Fatalf("Load returned %d channels, want 2 populated", len(back.Channels))
	}
	if got := back.Channels[0].Data.CTCSS; got != want {
		t.Errorf("channel 001 ctcss = %q, want %q", got, want)
	}
	if got := back.Channels[1].Data.CTCSS; got != "DCS-ENC" {
		t.Errorf("channel 002 ctcss = %q, want %q", got, "DCS-ENC")
	}
	if back.Schema != cp.Schema {
		t.Errorf("Save/Load moved the schema to %d, want %d — a DCS state is an ordinary string and must force no new schema", back.Schema, cp.Schema)
	}
}
