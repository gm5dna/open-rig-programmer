// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// hasIssue reports whether issues contains at least one Issue matching the
// given severity/field/slot and whose Msg contains msgSub.
func hasIssue(issues []Issue, severity Severity, field spec.Field, slot, msgSub string) bool {
	for _, is := range issues {
		if is.Severity == severity && is.Field == field && is.Slot == slot && strings.Contains(is.Msg, msgSub) {
			return true
		}
	}
	return false
}

// TestValidate exercises every rule in one table: a valid baseline fixture
// (see fixture_test.go), mutated one way per case, checked against the
// expected Issue. A zero-value want (empty Severity) means the mutation
// should leave the codeplug fully valid — this doubles as the "clean
// baseline" case when the mutate func is a no-op.
func TestValidate(t *testing.T) {
	cases := []struct {
		name         string
		mutate       func(cp *Codeplug)
		wantSeverity Severity
		wantField    spec.Field
		wantSlot     string
		wantMsgSub   string
	}{
		{
			name:   "valid baseline has no issues",
			mutate: func(cp *Codeplug) {},
		},
		{
			name: "radio model mismatch",
			mutate: func(cp *Codeplug) {
				cp.Radio.Model = "FT-991A"
			},
			wantSeverity: SeverityError,
			wantMsgSub:   "re-read the radio",
		},
		{
			name: "radio CAT ID mismatch",
			mutate: func(cp *Codeplug) {
				cp.Radio.CATID = "9999"
			},
			wantSeverity: SeverityError,
			wantMsgSub:   "re-read the radio",
		},
		{
			name: "slot not in any bank",
			mutate: func(cp *Codeplug) {
				cp.Channels = append(cp.Channels, Channel{
					Slot: "999",
					Data: &ChannelData{FreqHz: 14000000, Mode: "USB", CTCSS: "OFF", CTCSSTone: ToneField{State: Unknown}, Shift: "SIMPLEX", TagDisplay: BoolField{State: Known}, ScanSkip: BoolField{State: Known}},
				})
			},
			wantSeverity: SeverityError,
			wantSlot:     "999",
			wantMsgSub:   "not part of any bank",
		},
		{
			name: "duplicate slot",
			mutate: func(cp *Codeplug) {
				cp.Channels = append(cp.Channels, Channel{
					Slot: "002",
					Data: &ChannelData{FreqHz: 14000000, Mode: "USB", CTCSS: "OFF", CTCSSTone: ToneField{State: Unknown}, Shift: "SIMPLEX", TagDisplay: BoolField{State: Known}, ScanSkip: BoolField{State: Known}},
				})
			},
			wantSeverity: SeverityError,
			wantSlot:     "002",
			wantMsgSub:   "more than once",
		},
		{
			name: "required slot missing entirely",
			mutate: func(cp *Codeplug) {
				out := cp.Channels[:0]
				for _, ch := range cp.Channels {
					if ch.Slot == "001" {
						continue
					}
					out = append(out, ch)
				}
				cp.Channels = out
			},
			wantSeverity: SeverityError,
			wantSlot:     "001",
			wantMsgSub:   "missing",
		},
		{
			name: "required slot present but empty",
			mutate: func(cp *Codeplug) {
				for i := range cp.Channels {
					if cp.Channels[i].Slot == "001" {
						cp.Channels[i].Data = nil
					}
				}
			},
			wantSeverity: SeverityError,
			wantSlot:     "001",
			wantMsgSub:   "must not be empty",
		},
		{
			name: "NoBlank slot present but empty",
			mutate: func(cp *Codeplug) {
				for i := range cp.Channels {
					if cp.Channels[i].Slot == "P1L" {
						cp.Channels[i].Data = nil
					}
				}
			},
			wantSeverity: SeverityError,
			wantSlot:     "P1L",
			wantMsgSub:   "must stay populated",
		},
		{
			name: "NoBlank slot missing entirely",
			mutate: func(cp *Codeplug) {
				out := cp.Channels[:0]
				for _, ch := range cp.Channels {
					if ch.Slot == "P1U" {
						continue
					}
					out = append(out, ch)
				}
				cp.Channels = out
			},
			wantSeverity: SeverityError,
			wantSlot:     "P1U",
			wantMsgSub:   "missing",
		},
		{
			// "003" is an ordinary MEM slot: not in caps.RequiredSlots, not
			// in a NoBlank bank. Before Fix 6, nothing checked that an
			// ordinary bank slot is present at all, so removing it
			// entirely produced NO issue. The completeness check (built
			// from ALL caps.Banks, not just RequiredSlots/NoBlank) closes
			// that gap.
			name: "completeness: ordinary bank slot missing entirely",
			mutate: func(cp *Codeplug) {
				out := cp.Channels[:0]
				for _, ch := range cp.Channels {
					if ch.Slot == "003" {
						continue
					}
					out = append(out, ch)
				}
				cp.Channels = out
			},
			wantSeverity: SeverityError,
			wantSlot:     "003",
			wantMsgSub:   "missing",
		},
		{
			// The duplicate-multiset nuance: cp.Channels keeps the SAME
			// total length as the valid baseline (one slot dropped, one
			// duplicated), so a naive "does the channel count match the
			// expected count" check would miss this entirely. The
			// completeness check counts presence per expected slot, not
			// totals, so it still catches "003" being absent even though
			// "002" now appears twice (which the pre-existing
			// duplicate-slot check also independently flags).
			name: "completeness: duplicate slot masks a different missing slot",
			mutate: func(cp *Codeplug) {
				out := cp.Channels[:0]
				var dup Channel
				for _, ch := range cp.Channels {
					if ch.Slot == "003" {
						continue // drop 003 entirely
					}
					if ch.Slot == "002" {
						dup = ch
					}
					out = append(out, ch)
				}
				cp.Channels = append(out, dup) // duplicate 002; length unchanged overall
			},
			wantSeverity: SeverityError,
			wantSlot:     "003",
			wantMsgSub:   "missing",
		},
		{
			name: "frequency zero",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.FreqHz = 0
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldFrequency,
			wantSlot:     "002",
			wantMsgSub:   "greater than 0",
		},
		{
			name: "frequency below MinFreqHz",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.FreqHz = 1000
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldFrequency,
			wantSlot:     "002",
			wantMsgSub:   "below",
		},
		{
			name: "frequency above MaxFreqHz",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.FreqHz = 999999999
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldFrequency,
			wantSlot:     "002",
			wantMsgSub:   "above",
		},
		{
			name: "mode not supported",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.Mode = "PSK31"
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldMode,
			wantSlot:     "002",
			wantMsgSub:   "not one of",
		},
		{
			name: "clarifier exceeds max",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.ClarHz = 10000
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldClarifier,
			wantSlot:     "002",
			wantMsgSub:   "exceeds",
		},
		{
			name: "clarifier not a step multiple",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.ClarHz = 15
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldClarifier,
			wantSlot:     "002",
			wantMsgSub:   "step",
		},
		{
			name: "CTCSS state invalid",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.CTCSS = "BOGUS"
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldCTCSSState,
			wantSlot:     "002",
			wantMsgSub:   "OFF",
		},
		{
			name: "CTCSS tone invalid (not in table)",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(671)}
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldCTCSSTone,
			wantSlot:     "002",
			wantMsgSub:   "002",
		},
		{
			name: "ScanSkip invalid (Unknown with true value)",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.ScanSkip = BoolField{State: Unknown, Value: true}
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldScanSkip,
			wantSlot:     "002",
			wantMsgSub:   "002",
		},
		{
			// E1's first refusal: a non-Known TagDisplay means "preserve
			// whatever the radio has", so a true smuggled alongside it is a
			// value that must never be treated as an intent to send.
			name: "TagDisplay invalid (Unknown with true value)",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.TagDisplay = BoolField{State: Unknown, Value: true}
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldTagDisplay,
			wantSlot:     "002",
			wantMsgSub:   "002",
		},
		{
			name: "TagDisplay invalid (Unavailable with true value)",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.TagDisplay = BoolField{State: Unavailable, Value: true}
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldTagDisplay,
			wantSlot:     "002",
			wantMsgSub:   "002",
		},
		{
			name: "TagDisplay invalid (unrecognised State)",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.TagDisplay = BoolField{State: FieldState("maybe")}
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldTagDisplay,
			wantSlot:     "002",
			wantMsgSub:   "invalid State",
		},
		{
			// The other half of the rule, stated so it cannot be lost: a
			// well-formed Unknown TagDisplay is VALID data. Validate judges
			// shape only — blocking its SEND is Diff's job (M9c-5 task 2).
			name: "TagDisplay Unknown with false value is valid",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.TagDisplay = BoolField{State: Unknown}
			},
		},
		{
			name: "CTCSS tone pairing warning",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.CTCSS = "ENC"
				cp.Channels[1].Data.CTCSSTone = ToneField{State: Unknown}
			},
			wantSeverity: SeverityWarning,
			wantField:    spec.FieldCTCSSTone,
			wantSlot:     "002",
			wantMsgSub:   "cannot be set via CAT",
		},
		{
			name: "shift invalid",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.Shift = "SIDEWAYS"
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldShift,
			wantSlot:     "002",
			wantMsgSub:   "SIMPLEX",
		},
		{
			name: "tag too long",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.Tag = "THIRTEEN-CHRS"
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldTag,
			wantSlot:     "002",
			wantMsgSub:   "exceeds",
		},
		{
			name: "tag contains semicolon",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.Tag = "BAD;TAG"
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldTag,
			wantSlot:     "002",
			wantMsgSub:   "invalid byte",
		},
		{
			name: "tag contains control byte",
			mutate: func(cp *Codeplug) {
				cp.Channels[1].Data.Tag = "BAD\x01TAG"
			},
			wantSeverity: SeverityError,
			wantField:    spec.FieldTag,
			wantSlot:     "002",
			wantMsgSub:   "invalid byte",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp := testBaselineCodeplug()
			tc.mutate(cp)
			issues := Validate(cp, testCapabilities())

			if tc.wantSeverity == "" {
				if len(issues) != 0 {
					t.Fatalf("Validate() = %+v, want no issues", issues)
				}
				return
			}
			if !hasIssue(issues, tc.wantSeverity, tc.wantField, tc.wantSlot, tc.wantMsgSub) {
				t.Errorf("Validate() = %+v, want an issue matching {Severity:%v Field:%v Slot:%v MsgContains:%q}",
					issues, tc.wantSeverity, tc.wantField, tc.wantSlot, tc.wantMsgSub)
			}
		})
	}
}

// TestValidateDeterministic runs Validate twice over the same inputs (with
// several rules broken at once, so ordering is meaningful) and requires
// byte-for-byte identical results.
func TestValidateDeterministic(t *testing.T) {
	cp := testBaselineCodeplug()
	cp.Channels[0].Data.FreqHz = 0
	cp.Channels[1].Data.Mode = "BOGUS"
	cp.Channels[1].Data.Shift = "BOGUS"
	caps := testCapabilities()

	issues1 := Validate(cp, caps)
	issues2 := Validate(cp, caps)

	if len(issues1) == 0 {
		t.Fatal("expected at least one issue to make ordering meaningful")
	}
	if !reflect.DeepEqual(issues1, issues2) {
		t.Errorf("Validate() not deterministic:\nrun1: %+v\nrun2: %+v", issues1, issues2)
	}
}

// TestValidate_TagDisplayIssueOrder pins WHERE the TagDisplay shape issue
// lands in the fixed per-channel order Validate's doc comment promises:
// directly after ScanSkip's and BEFORE the CTCSS-tone-pairing warning.
//
// Position is contractual here, not incidental — Validate's determinism
// guarantee is what lets callers (and golden output) rely on the sequence,
// so a field slotted in "somewhere sensible" would silently reorder every
// multi-issue channel's report.
func TestValidate_TagDisplayIssueOrder(t *testing.T) {
	cp := testBaselineCodeplug()
	d := cp.Channels[1].Data
	// Break tone, skip and display at once, and force the tone-pairing
	// warning too, so all four issues are present for one channel.
	d.CTCSS = "ENC"
	d.CTCSSTone = ToneField{State: Unknown, Value: spec.Tone(670)}
	d.ScanSkip = BoolField{State: Unknown, Value: true}
	d.TagDisplay = BoolField{State: Unknown, Value: true}

	var got []spec.Field
	for _, is := range Validate(cp, testCapabilities()) {
		if is.Slot == "002" {
			got = append(got, is.Field)
		}
	}
	want := []spec.Field{
		spec.FieldCTCSSTone,  // CTCSSTone.Valid()
		spec.FieldScanSkip,   // ScanSkip.Valid()
		spec.FieldTagDisplay, // TagDisplay.Valid() — directly after it
		spec.FieldCTCSSTone,  // then the tone-pairing warning
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("slot 002 issue field order = %v, want %v", got, want)
	}
}

// deviantVocabCapabilities returns a minimal, self-consistent
// Capabilities whose Shift and CTCSS vocabularies are deliberately NOT
// the FT-710's own literals — in particular, its "disabled" CTCSS state
// is named "DISABLED", not "OFF" — so that TestValidate_ShiftCTCSSVocabFromCaps
// can prove Validate's Shift/CTCSS checks are genuinely data-driven
// rather than still secretly comparing against "OFF"/"SIMPLEX"/etc: a
// hardcoded-literal implementation would either wrongly reject this
// vocabulary's own values or wrongly accept the old FT-710 ones.
func deviantVocabCapabilities() spec.Capabilities {
	return spec.Capabilities{
		Model: "DEVIANT-1",
		CATID: "0001",
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: "Memories",
				Slots: []string{"001"},
			},
		},
		Modes:       []string{"USB"},
		TagLen:      12,
		ClarMaxHz:   9990,
		ClarStepHz:  10,
		Bauds:       []int{9600},
		DefaultBaud: 9600,
		ShiftOptions: []spec.ShiftOption{
			{Value: "SPLIT-MINUS", Direction: spec.ShiftDown},
			{Value: "SPLIT-PLUS", Direction: spec.ShiftUp},
			{Value: "SPLIT-NONE", Direction: spec.ShiftNone},
		},
		CTCSSStates: []spec.ToneState{
			{Value: "DISABLED", Semantics: spec.ToneOff},
			{Value: "TONE", Semantics: spec.ToneEncode},
		},
	}
}

// deviantChannel builds a single-slot Codeplug ("001", matching
// deviantVocabCapabilities' one bank) with the given Shift/CTCSS values,
// and a known CTCSSTone iff toneKnown.
func deviantChannel(shift, ctcss string, toneKnown bool) *Codeplug {
	tone := ToneField{State: Unknown}
	if toneKnown {
		tone = ToneField{State: Known, Value: spec.Tone(670)}
	}
	return &Codeplug{
		Schema: CurrentSchema,
		Radio:  RadioInfo{Model: "DEVIANT-1", CATID: "0001"},
		Channels: []Channel{
			{
				Slot: "001",
				Data: &ChannelData{
					FreqHz:     14250000,
					Mode:       "USB",
					CTCSS:      ctcss,
					CTCSSTone:  tone,
					Shift:      shift,
					TagDisplay: BoolField{State: Known, Value: false},
					ScanSkip:   BoolField{State: Known, Value: false},
				},
			},
		},
	}
}

// TestValidate_ShiftCTCSSVocabFromCaps proves the Shift/CTCSS checks in
// validateChannelData are driven entirely by caps.ShiftOptions/
// caps.CTCSSStates, not by any hardcoded literal: a deviant vocabulary
// (see deviantVocabCapabilities) accepts its own values and rejects the
// FT-710's old literals, and RequiresTone — not the literal string
// "OFF" — decides whether the CTCSS-tone-pairing warning fires.
func TestValidate_ShiftCTCSSVocabFromCaps(t *testing.T) {
	caps := deviantVocabCapabilities()

	t.Run("deviant vocab's own values are accepted", func(t *testing.T) {
		issues := Validate(deviantChannel("SPLIT-NONE", "DISABLED", false), caps)
		if len(issues) != 0 {
			t.Errorf("Validate() = %+v, want no issues", issues)
		}
	})

	t.Run("legacy FT-710 literals are rejected against a deviant vocab", func(t *testing.T) {
		issues := Validate(deviantChannel("SIMPLEX", "OFF", false), caps)
		if !hasIssue(issues, SeverityError, spec.FieldShift, "001", "SPLIT-MINUS") {
			t.Errorf("Validate() = %+v, want a Shift error naming the deviant vocab", issues)
		}
		if !hasIssue(issues, SeverityError, spec.FieldCTCSSState, "001", "DISABLED") {
			t.Errorf("Validate() = %+v, want a CTCSSState error naming the deviant vocab", issues)
		}
	})

	t.Run("disabled state not named OFF: RequiresTone false suppresses the tone-pairing warning", func(t *testing.T) {
		issues := Validate(deviantChannel("SPLIT-NONE", "DISABLED", false), caps)
		if hasIssue(issues, SeverityWarning, spec.FieldCTCSSTone, "001", "") {
			t.Errorf("Validate() = %+v, want no CTCSSTone warning (RequiresTone false, no OFF literal involved)", issues)
		}
	})

	t.Run("tone-bearing state without a known tone: RequiresTone true fires the warning", func(t *testing.T) {
		issues := Validate(deviantChannel("SPLIT-NONE", "TONE", false), caps)
		if !hasIssue(issues, SeverityWarning, spec.FieldCTCSSTone, "001", "cannot be set via CAT") {
			t.Errorf("Validate() = %+v, want a CTCSSTone warning (RequiresTone true, no known tone)", issues)
		}
	})

	t.Run("tone-bearing state with a known tone: no warning", func(t *testing.T) {
		issues := Validate(deviantChannel("SPLIT-NONE", "TONE", true), caps)
		if hasIssue(issues, SeverityWarning, spec.FieldCTCSSTone, "001", "") {
			t.Errorf("Validate() = %+v, want no CTCSSTone warning when tone is known", issues)
		}
	})
}

// narrowToneChartCapabilities returns testCapabilities() with CTCSSTones
// replaced by a single-entry chart (the standard chart's own first tone).
// Deliberately narrower than spec.StandardCTCSSTones (which testCapabilities
// otherwise uses in full) so a test can tell a genuinely caps-driven
// CTCSSTone check apart from one still secretly comparing against
// spec.StandardCTCSSTones/spec.ValidTone: the standard chart's LAST tone
// (2541) is a value a hardcoded-standard-chart implementation would wrongly
// accept here.
func narrowToneChartCapabilities() spec.Capabilities {
	caps := testCapabilities()
	caps.CTCSSTones = []spec.Tone{670}
	return caps
}

// TestValidate_CTCSSToneChartFromCaps proves the send gate's CTCSSTone
// check (via ToneField.Valid) is driven by caps.CTCSSTones — THIS radio's
// own chart — not the package-global spec.StandardCTCSSTones: a tone
// standard-chart-valid but absent from a narrower radio's own chart is
// rejected, a tone the narrower chart DOES have is accepted, and an empty
// caps.CTCSSTones fails closed (rejects every Known tone) rather than
// silently accepting everything. See FIX C1 (m9c1 registration-gate,
// dispatch C): before this fix, ToneField.Valid consulted the global
// spec.ValidTone regardless of caps, so a radio with a narrower chart than
// the FT-710 could not be safely represented.
func TestValidate_CTCSSToneChartFromCaps(t *testing.T) {
	t.Run("tone present in this radio's own (narrower) chart is accepted", func(t *testing.T) {
		cp := testBaselineCodeplug()
		cp.Channels[0].Data.CTCSS = "ENC"
		cp.Channels[0].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(670)}
		issues := Validate(cp, narrowToneChartCapabilities())
		if hasIssue(issues, SeverityError, spec.FieldCTCSSTone, "001", "") {
			t.Errorf("Validate() = %+v, want no CTCSSTone error for a tone in this radio's own chart", issues)
		}
	})

	t.Run("tone in the standard chart but absent from this radio's narrower chart is rejected", func(t *testing.T) {
		cp := testBaselineCodeplug()
		cp.Channels[0].Data.CTCSS = "ENC"
		cp.Channels[0].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(2541)} // last standard-chart tone; not in the narrow chart
		issues := Validate(cp, narrowToneChartCapabilities())
		// The substring tracks ToneField.Valid's wording, which the Icom
		// tier (E3) generalised from "not in this radio's CTCSS chart" to
		// "not a tone this radio can express": a chart is now one of two
		// shapes a radio's tone domain can take. The BEHAVIOUR asserted
		// here — a tone outside this radio's own domain is an error — is
		// unchanged.
		if !hasIssue(issues, SeverityError, spec.FieldCTCSSTone, "001", "this radio can express") {
			t.Errorf("Validate() = %+v, want a CTCSSTone error for a tone outside this radio's own chart", issues)
		}
	})

	t.Run("empty caps.CTCSSTones fails closed rather than accepting every tone", func(t *testing.T) {
		cp := testBaselineCodeplug()
		cp.Channels[0].Data.CTCSS = "ENC"
		cp.Channels[0].Data.CTCSSTone = ToneField{State: Known, Value: spec.Tone(670)}
		caps := testCapabilities()
		caps.CTCSSTones = nil
		issues := Validate(cp, caps)
		if !hasIssue(issues, SeverityError, spec.FieldCTCSSTone, "001", "this radio can express") {
			t.Errorf("Validate() = %+v, want a CTCSSTone error when caps.CTCSSTones is empty (fail closed, not fail open)", issues)
		}
	})
}

// TestHasErrors covers the HasErrors convenience: true only when at least
// one Issue is SeverityError.
func TestHasErrors(t *testing.T) {
	cases := []struct {
		name   string
		issues []Issue
		want   bool
	}{
		{"empty", nil, false},
		{"only warnings", []Issue{{Severity: SeverityWarning}}, false},
		{"has an error", []Issue{{Severity: SeverityWarning}, {Severity: SeverityError}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasErrors(tc.issues); got != tc.want {
				t.Errorf("HasErrors(%+v) = %v, want %v", tc.issues, got, tc.want)
			}
		})
	}
}
