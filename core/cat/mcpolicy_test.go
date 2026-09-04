// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"strings"
	"testing"
)

// This file is the FT-891 Stage 0 seam S0.2: cat.MCSlotPolicy, the MC
// command's SEND-side slot domain.
//
// IT IS A NEW FILE RATHER THAN AN APPENDIX TO mc_test.go, deliberately.
// mc_test.go is one of the files core/cat/testdata/evidence-literals.golden
// pins BY FILE AND ORDINAL: a literal added anywhere in it — including a new
// import — renumbers every literal after it and fails that pin. A new file
// carries no golden lines at all, so the evidence pin is left exactly as it
// stands rather than regenerated. The same reasoning puts the other three
// Stage 0 axes in files of their own.

// TestMCSelects_MemoryPMSRefusesSixtyAndEMGAtBuilderAndGate is the
// disagreeing fixture the seam exists for: a dialect whose MC legend prints
// memory and PMS only must refuse an MC Set of a 60m or EMG slot at the
// BUILDER and at the OUTBOUND GATE, while a dialect declaring MCSelectsAll
// accepts both.
//
// Every refusal is paired with a POSITIVE CONTROL ON THE SAME WIRE FORM, so
// a change that merely broke MC everywhere cannot pass for a policy: the
// FT-710 must still build and admit "MC501;" and "MCEMG;", and
// mcMemoryPMSDialect must still build and admit its own memory and PMS
// slots.
func TestMCSelects_MemoryPMSRefusesSixtyAndEMGAtBuilderAndGate(t *testing.T) {
	sixty, err := mcMemoryPMSDialect.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("fixture broken: SixtyMSlot(1): %v", err)
	}
	emg := mcMemoryPMSDialect.EMGSlot()
	if emg.Wire() == "" {
		t.Fatal("fixture broken: mcMemoryPMSDialect declares no emergency channel, so this test has nothing for the policy to refuse")
	}

	for _, s := range []Slot{sixty, emg} {
		cmd, err := mcMemoryPMSDialect.BuildMCSet(s)
		if err == nil {
			t.Errorf("mcMemoryPMSDialect.BuildMCSet(%q) succeeded, emitting %q — its MCSelects is MCSelectsMemoryPMS, so this slot is outside its MC send domain", s.Wire(), cmd.Bytes())
		} else {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("mcMemoryPMSDialect.BuildMCSet(%q): error is %T, want *ParseError", s.Wire(), err)
			}
			if !cmd.IsZero() {
				t.Errorf("mcMemoryPMSDialect.BuildMCSet(%q) returned a non-zero Command alongside its error; every fallible builder returns Command{}", s.Wire())
			}
			if !strings.Contains(err.Error(), s.Wire()) {
				t.Errorf("mcMemoryPMSDialect.BuildMCSet(%q) refused with %q, which does not name the slot", s.Wire(), err)
			}
			if !strings.Contains(err.Error(), MCSelectsMemoryPMS.String()) {
				t.Errorf("mcMemoryPMSDialect.BuildMCSet(%q) refused with %q, which does not name the policy %v — a refusal that does not say which rule fired sends whoever has to fix it to the wrong place", s.Wire(), err, MCSelectsMemoryPMS)
			}
		}

		frame := []byte("MC" + s.Wire() + ";")
		if mcMemoryPMSDialect.AllowedCommand(frame) {
			t.Errorf("mcMemoryPMSDialect's gate ADMITTED %q — the gate and the builder must share one send-side predicate, or this program can be made to send a recall its own builder refuses", frame)
		}

		// POSITIVE CONTROL, same wire form, on a dialect declaring the wide
		// domain.
		ft710Slot, err := FT710.ParseSlot(s.Wire())
		if err != nil {
			t.Fatalf("FT710.ParseSlot(%q): %v", s.Wire(), err)
		}
		if _, err := FT710.BuildMCSet(ft710Slot); err != nil {
			t.Errorf("FT710.BuildMCSet(%q) = %v — the FT-710's MC block prints the 5xx and EMG banks, so it must still build", s.Wire(), err)
		}
		if !FT710.AllowedCommand(frame) {
			t.Errorf("FT710's gate refused %q — the FT-710 declares MCSelectsAll", frame)
		}
	}

	// POSITIVE CONTROL on the narrowed dialect itself: the classes its own
	// legend does print must still build and still be admitted.
	mem, err := mcMemoryPMSDialect.MemorySlot(1)
	if err != nil {
		t.Fatalf("fixture broken: MemorySlot(1): %v", err)
	}
	pms, err := mcMemoryPMSDialect.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("fixture broken: PMSSlot(1, false): %v", err)
	}
	for _, s := range []Slot{mem, pms} {
		cmd, err := mcMemoryPMSDialect.BuildMCSet(s)
		if err != nil {
			t.Errorf("mcMemoryPMSDialect.BuildMCSet(%q) = %v — memory and PMS are inside its own MC send domain", s.Wire(), err)
			continue
		}
		if !mcMemoryPMSDialect.AllowedCommand(cmd.Bytes()) {
			t.Errorf("mcMemoryPMSDialect's gate refused its own builder's %q", cmd.Bytes())
		}
	}
}

// TestMCSelects_ParseMCAnswerKeepsTheFullReadableSpace pins the OTHER half
// of the split, and it is the half a narrowing would silently take with it.
//
// An MC Set and an MC Answer share one wire shape, so a policy applied to
// both would narrow what a radio may be HEARD to say as well as what this
// program may send — and a radio parked on a 60m channel it reached from the
// front panel answers "MC5xx;" however narrow its Set domain is. The FT-891's
// own MC block prints no Answer domain at all, so accepting the full
// readable space here is the conservative reading and is registered as such.
func TestMCSelects_ParseMCAnswerKeepsTheFullReadableSpace(t *testing.T) {
	for _, frame := range []string{"MC501;", "MCEMG;"} {
		got, err := mcMemoryPMSDialect.ParseMCAnswer([]byte(frame))
		if err != nil {
			t.Errorf("mcMemoryPMSDialect.ParseMCAnswer(%q) = %v — MCSelects is a SEND-side domain and must not reach the parser", frame, err)
			continue
		}
		if got.Wire() != frame[2:5] {
			t.Errorf("mcMemoryPMSDialect.ParseMCAnswer(%q) returned slot %q, want %q", frame, got.Wire(), frame[2:5])
		}
	}
	// And "000" is still refused on the parse side, exactly as before: the
	// none form is outside the MC space in either direction.
	if _, err := mcMemoryPMSDialect.ParseMCAnswer([]byte("MC000;")); err == nil {
		t.Error("mcMemoryPMSDialect.ParseMCAnswer(\"MC000;\") succeeded — the none form is not an MC target in either direction")
	}
}

// TestMCSelects_RegisteredDialectsSelectAll pins the declaration each
// registered dialect makes, against its own MC legend.
//
// core/cat can see only the FT-710 here; the FTdx10's and FTdx101's are
// pinned in their own packages' tests, where their manuals are cited.
func TestMCSelects_RegisteredDialectsSelectAll(t *testing.T) {
	if got := FT710.slots.mcSelects; got != MCSelectsAll {
		t.Errorf("FT710 declares MCSelects %v, want MCSelectsAll — the FT-710 CAT manual's MC block prints \"001-099 / P1L-P9U / 5xx: (5MHz BAND) / EMG: (EMERGENCY CH)\"", got)
	}
	// The literal and the constructor must agree, as they do for every other
	// field: FT710 is a package literal that bypasses NewDialect.
	for _, s := range []string{"501", "EMG"} {
		slot, err := FT710.ParseSlot(s)
		if err != nil {
			t.Fatalf("FT710.ParseSlot(%q): %v", s, err)
		}
		if _, err := FT710.BuildMCSet(slot); err != nil {
			t.Errorf("FT710.BuildMCSet(%q) = %v — MCSelectsAll admits every class outside the none form", s, err)
		}
	}
}

// TestEveryDialect_MCGateAgreesWithItsBuilder is the general property the
// split must not break: for every slot every dialect classifies, that
// dialect's gate admits the MC Set frame EXACTLY when its own builder
// produces one.
//
// It exists because dialecttest.checkSlotFrames tolerates a refused MC
// build — it holds only the frames a builder DID produce to the gate — so
// nothing there would notice a gate and a builder disagreeing under a
// narrowed policy. The refusal counter is what stops this passing on a
// builder that accepts everything.
func TestEveryDialect_MCGateAgreesWithItsBuilder(t *testing.T) {
	checked, refused := 0, 0
	for _, nd := range allTestDialects() {
		var slots []Slot
		for n := 0; n <= 999; n++ {
			if s, err := nd.dia.ParseSlot(threeDigits(n)); err == nil {
				slots = append(slots, s)
			}
		}
		for pair := 1; pair <= 9; pair++ {
			for _, upper := range []bool{false, true} {
				if s, err := nd.dia.PMSSlot(pair, upper); err == nil {
					slots = append(slots, s)
				}
			}
		}
		if s := nd.dia.EMGSlot(); s.Wire() != "" {
			slots = append(slots, s)
		}

		for _, s := range slots {
			frame := []byte("MC" + s.Wire() + ";")
			_, buildErr := nd.dia.BuildMCSet(s)
			admitted := nd.dia.AllowedCommand(frame)
			checked++
			if buildErr == nil && !admitted {
				t.Errorf("%s: BuildMCSet(%q) built a frame its own gate then refused — a builder and a gate that disagree mean this program cannot send a command it believes is valid", nd.name, s.Wire())
			}
			if buildErr != nil {
				refused++
				if admitted {
					t.Errorf("%s: its gate ADMITTED %q while its own builder refused it (%v) — the gate is not judging by the send-side predicate", nd.name, frame, buildErr)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no slots were checked — the walk is not reaching any dialect's slot space")
	}
	if refused == 0 {
		t.Fatal("no MC build refusal was observed anywhere in the walk — the agreement asserted above would hold vacuously on a builder that accepts everything")
	}
	t.Logf("MC builder/gate agreement checked over %d slots across %d dialects (%d refusals seen)", checked, len(allTestDialects()), refused)
}

// TestValidateDialectConfig_V13MCSelects is V13's own clause table: the zero
// value is refused and names its field, and both policies are accepted.
//
// It is here rather than appended to dialectvalidate_test.go's own table for
// the reason given at the top of this file — that table sits mid-file, and
// adding literals to it renumbers everything after it in the evidence pin.
func TestValidateDialectConfig_V13MCSelects(t *testing.T) {
	tests := []struct {
		name    string
		policy  MCSlotPolicy
		wantErr string // "" means the config MUST be accepted
	}{
		{"zero refused", 0, "MCSelects"},
		{"out-of-range refused", MCSlotPolicy(99), "MCSelects"},
		{"MCSelectsAll accepted", MCSelectsAll, ""},
		{"MCSelectsMemoryPMS accepted", MCSelectsMemoryPMS, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaselineConfig()
			cfg.Slots.MCSelects = tc.policy
			err := validateDialectConfig(cfg)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("validateDialectConfig() = %v, want accepted", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("validateDialectConfig() = nil, want an error mentioning %q — an omitted config semantic is refused, never defaulted", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("validateDialectConfig() = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

// TestMCSlotPolicy_String pins the names the refusals quote. A policy whose
// String() rendered a bare integer would make every refusal above unreadable
// and would still satisfy a strings.Contains on the field name alone.
func TestMCSlotPolicy_String(t *testing.T) {
	for _, tc := range []struct {
		p    MCSlotPolicy
		want string
	}{
		{MCSelectsAll, "MCSelectsAll"},
		{MCSelectsMemoryPMS, "MCSelectsMemoryPMS"},
		{0, "MCSlotPolicy(0)"},
		{MCSlotPolicy(7), "MCSlotPolicy(7)"},
	} {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("MCSlotPolicy(%d).String() = %q, want %q", int(tc.p), got, tc.want)
		}
	}
}
