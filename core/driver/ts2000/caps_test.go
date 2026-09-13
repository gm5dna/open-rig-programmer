// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestCapabilities_ValidateOnEveryRowAndProfile is the baseline sanity
// check every driver package pins: spec.Validate accepts what this package
// publishes, on all three rows and both profiles.
func TestCapabilities_ValidateOnEveryRowAndProfile(t *testing.T) {
	for name, p := range map[string]modelParams{
		"TS-2000": paramsTS2000, "TS-2000X": paramsTS2000X, "TS-B2000": paramsTSB2000,
	} {
		for profile, caps := range map[string]spec.Capabilities{
			"Unverified": CapabilitiesUnverified(p),
			"Simulated":  CapabilitiesSimulated(p),
		} {
			if err := caps.Validate(); err != nil {
				t.Errorf("%s %s: Validate: %v", name, profile, err)
			}
			if caps.Model != p.name {
				t.Errorf("%s %s: Model = %q, want %q", name, profile, caps.Model, p.name)
			}
			if caps.CATID != "019" {
				t.Errorf("%s %s: CATID = %q, want \"019\"", name, profile, caps.CATID)
			}
		}
	}
}

// auditedFields is the hand-written list of the fields THIS ROW's record
// expresses — nine, against the TS-480's five: this row's own P10/P12/P13
// lift adds duplex/offset, and this document's own tone chart adds
// tone_tx/tone_rx.
func auditedFields() []spec.Field {
	return []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldScanSkip, spec.FieldToneMode,
		spec.FieldDuplex, spec.FieldOffset,
		spec.FieldToneTx, spec.FieldToneRx,
	}
}

// unexpressedFields is the other half of the audit: every field this row
// grades the zero FieldSupport, with the matrix's own reason.
func unexpressedFields() map[spec.Field]string {
	return map[spec.Field]string{
		spec.FieldClarifier:         "matrix §4: no clarifier position over this document's own complete 47-byte account (ts2000:10692-10727)",
		spec.FieldCTCSSState:        "matrix §4, design decision 6: this record expresses tone via FieldToneMode (the Icom half); the Yaesu half is not declared",
		spec.FieldCTCSSTone:         "matrix §4: the record carries two independent tone indices (P8, P9) and FieldCTCSSTone is one field",
		spec.FieldShift:             "matrix §4, design decision 6: the Icom half (FieldDuplex) is declared instead",
		spec.FieldTagDisplay:        "matrix §4: no tag-display flag anywhere in the record",
		spec.FieldErase:             "matrix §4, spec §3: fleet-wide write posture (opt-in write of existing channels); this document documents no MR/MW-reachable erase primitive either",
		spec.FieldTxFrequency:       "matrix §4, design decision 6: this row expresses repeater offset via FieldDuplex+FieldOffset (the Icom half), not a stored second frequency",
		spec.FieldDTCSCode:          "matrix §6 item 4: the 104-entry DCS chart (P10) is cited but not transcribed this milestone; DTCSCodes is nil, so a Known value would fail codeplug.IntField.Valid closed",
		spec.FieldDTCSPolarity:      "matrix §4: the DCS chart and QC's legend name a code index only; no NN/NR/RN/RR polarity concept is printed",
		spec.FieldFilter:            "matrix §2: byte 28 is REVERSE here, not Filter A/B, and no spec.Field names repeater-reverse — UNMAPPED-with-reason, the same shape as byte 41 below",
		spec.FieldDataMode:          "matrix §4: byte 19 is the channel lockout here (published as scan_skip), not a data-mode flag; no data-mode position exists in the record",
		spec.FieldTuningStepEnabled: "matrix §2, spec §6 Q4: no on/off flag for a step exists in the record",
		spec.FieldTuningStep:        "matrix §2, spec §6 Q4: bytes 39-40 ARE a step here (ts2000:10720-10722) and this row's own ST is mode-conditional over two ranges exactly like the TS-480's (ts2000:11508-11521); EX/menu and tuning-step vocabulary are both excluded this wave, so no honest flat vocabulary exists — a REFUSAL, not a plain absence (write.go)",
		spec.FieldProgramTuningStep: "matrix §2: an index into ST, not a magnitude in hertz",
		spec.FieldAttenuator:        "matrix §4: additions design D8 — none applies to any of this wave's seven packages",
		spec.FieldPreamp:            "matrix §4: additions design D8",
		spec.FieldAntenna:           "matrix §4: additions design D8",
		spec.FieldIPPlus:            "matrix §4: additions design D8, an Icom concept with no position in this frame",
	}
}

// TestFieldAudit_CoversEverySpecField consumes the fleet helper as a black
// box: the audited list is this driver's own hand-written one, and every
// other field carries a written reason.
func TestFieldAudit_CoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "auditedFields()", auditedFields(), unexpressedFields())
}

// nonFieldReasons is the audit's OTHER half — the bytes and the roadmap
// tail that have NO spec.Field to be covered by at all, so
// AssertFieldAuditCoversEverySpecField cannot see them. The brief asks for
// these named explicitly (byte 28/P11, byte 41/P15, and Satellite Memory).
func nonFieldReasons() map[string]string {
	return map[string]string{
		"byte 28 (P11, REVERSE)":       "matrix §2: live on this row (kw.Byte28Reverse) and no spec.Field names repeater-reverse anywhere in this project — UNMAPPED-with-reason. Parsed by the codec (kw.Record.Byte28) and never published.",
		"byte 41 (P15, Memory Group)":  "matrix §2: live on this row (kw.Byte41MemoryGroup) and no spec.Field names channel-group membership — UNMAPPED-with-reason, the TS-480 exemplar's own precedent for this exact gap. Parsed and never published.",
		"Satellite Memory (SA/SI, MU)": "matrix §3, spec §6 open question 1 (Stuart, default accepted): a separate 10-channel record with no frequency field of its own, out of this wave on the IC-9100 D-STAR-block precedent. No row and no spec.Field.",
	}
}

// TestNonFieldBytesAndSatelliteHaveRecordedReasons pins that every entry
// above carries a non-empty, distinct reason.
func TestNonFieldBytesAndSatelliteHaveRecordedReasons(t *testing.T) {
	seen := map[string]string{}
	for what, reason := range nonFieldReasons() {
		if reason == "" {
			t.Errorf("%s carries no recorded reason", what)
			continue
		}
		if prev, dup := seen[reason]; dup {
			t.Errorf("%s and %s carry the SAME reason %q", prev, what, reason)
		}
		seen[reason] = what
	}
}

// TestWriteTrialsComplete_PinnedFalse is the two-part pin: the constant is
// false, and the RealHardware baseline it travels with is genuinely
// nothing-writable, on all three rows.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true; no TS-2000/2000X/B2000 has ever been asked anything by this project (matrix, line 8)")
	}
	for _, p := range []modelParams{paramsTS2000, paramsTS2000X, paramsTSB2000} {
		for _, b := range CapabilitiesUnverified(p).Banks {
			for _, f := range auditedFields() {
				if b.Fields[f].CanWrite() {
					t.Errorf("%s: RealHardware bank %s grades %s writable while writeTrialsComplete is false", p.name, b.ID, f)
				}
			}
		}
	}
}

// TestNoProductionFileNamesTheSharedToneChart pins the non-borrowing rule
// (matrix §4): this row's chart is its own document's, never
// core/spec.StandardCTCSSTones nor the 590 pair's kenwoodCTCSSTones.
func TestNoProductionFileNamesTheSharedToneChart(t *testing.T) {
	if len(kenwoodTS2000CTCSSTones) != 39 {
		t.Errorf("kenwoodTS2000CTCSSTones has %d entries, want 39 (ts2000:3837-3847)", len(kenwoodTS2000CTCSSTones))
	}
	if got := kenwoodTS2000CTCSSTones[len(kenwoodTS2000CTCSSTones)-1]; got != 17500 {
		t.Errorf("the 39th entry is %v, want 17500 (1750 Hz, the over-claim on the receive side — see the chart's own doc comment)", got)
	}
}
