// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestChannelEmpty covers the sole empty/populated discriminator:
// Data == nil.
func TestChannelEmpty(t *testing.T) {
	cases := []struct {
		name string
		ch   Channel
		want bool
	}{
		{"nil data is empty", Channel{Slot: "001", Data: nil}, true},
		{"non-nil data is populated", Channel{Slot: "001", Data: &ChannelData{}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ch.Empty(); got != tc.want {
				t.Errorf("Empty() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestChannelJSON_EmptyOmitsData is the load-bearing JSON-shape test: an
// empty channel must marshal WITHOUT a "data" key at all (not "data":
// null), so a reader can distinguish "the encoder didn't bother" from
// "this slot is explicitly empty" — omitempty on a nil pointer gives us
// exactly that.
func TestChannelJSON_EmptyOmitsData(t *testing.T) {
	ch := Channel{Slot: "001"}
	b, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(b), `"data"`) {
		t.Errorf("Marshal() = %s, must not contain a \"data\" key for an empty channel", b)
	}

	var got Channel
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.Empty() {
		t.Errorf("round-tripped channel is not Empty(): %+v", got)
	}
}

// TestChannelJSON_RoundTrip populates every field of ChannelData with a
// non-default value and checks a lossless marshal/unmarshal round trip.
func TestChannelJSON_RoundTrip(t *testing.T) {
	want := Channel{
		Slot: "007",
		Data: &ChannelData{
			FreqHz:     14250000,
			Mode:       "USB",
			ClarHz:     -120,
			RxClar:     true,
			TxClar:     true,
			CTCSS:      "ENC-DEC",
			CTCSSTone:  ToneField{State: Known, Value: spec.Tone(885)},
			Shift:      "PLUS",
			Tag:        "MB9XYZ",
			TagDisplay: BoolField{State: Known, Value: true},
			ScanSkip:   BoolField{State: Known, Value: true},
		},
	}

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Channel
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Slot != want.Slot {
		t.Errorf("Slot = %q, want %q", got.Slot, want.Slot)
	}
	if got.Data == nil {
		t.Fatalf("Data = nil, want populated")
	}
	if *got.Data != *want.Data {
		t.Errorf("Data = %+v, want %+v", *got.Data, *want.Data)
	}
}

// TestDisplaySlot covers the full display-mapping table from the brief,
// plus unknown-form passthrough cases.
func TestDisplaySlot(t *testing.T) {
	cases := []struct {
		slot string
		want string
	}{
		{"001", "M-01"},
		{"099", "M-99"},
		{"P1L", "P1L"},
		{"501", "5-01"},
		{"515", "5-15"},
		{"EMG", "EMG"},
		// Unknown/unrecognised forms pass through unchanged.
		{"", ""},
		{"XY", "XY"},
		{"P1X", "P1X"},
		{"abc", "abc"},
		{"1234", "1234"},
	}
	for _, tc := range cases {
		t.Run(tc.slot, func(t *testing.T) {
			if got := DisplaySlot(tc.slot); got != tc.want {
				t.Errorf("DisplaySlot(%q) = %q, want %q", tc.slot, got, tc.want)
			}
		})
	}
}

// TestDisplaySlot_IdentityForNonThreeDigit pins DisplaySlot's identity
// fallback (see its doc comment's "neutral default" wording, task 38/
// M9a-2) for slot forms shaped nothing like the FT-710's 3-digit family —
// in particular 5-digit forms of the kind a future FTX-1 driver might
// use — proving DisplaySlot's len(slot) != 3 guard makes it a safe,
// unopinionated no-op for a radio-neutral wire form it was never told
// how to display, rather than a partial/incorrect transformation.
func TestDisplaySlot_IdentityForNonThreeDigit(t *testing.T) {
	cases := []string{"00001", "00999", "12345", "0"}
	for _, slot := range cases {
		t.Run(slot, func(t *testing.T) {
			if got := DisplaySlot(slot); got != slot {
				t.Errorf("DisplaySlot(%q) = %q, want %q (identity)", slot, got, slot)
			}
		})
	}
}

// TestBankForSlot covers the unexported bankForSlot helper: found in the
// bank that lists it, and not found when no bank in caps claims it.
func TestBankForSlot(t *testing.T) {
	caps := spec.Capabilities{
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Slots: []string{"001", "002", "099"}},
			{ID: spec.BankPMS, Slots: []string{"P1L", "P1U"}},
		},
	}

	cases := []struct {
		name   string
		slot   string
		wantID spec.BankID
		wantOK bool
	}{
		{"memory slot found", "002", spec.BankMemory, true},
		{"pms slot found", "P1U", spec.BankPMS, true},
		{"unknown slot not found", "EMG", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotOK := bankForSlot(caps, tc.slot)
			if gotID != tc.wantID || gotOK != tc.wantOK {
				t.Errorf("bankForSlot(caps, %q) = (%q, %v), want (%q, %v)", tc.slot, gotID, gotOK, tc.wantID, tc.wantOK)
			}
		})
	}
}
