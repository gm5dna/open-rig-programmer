// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"strings"
	"testing"
)

// This file is the FT-891 Stage 0 seam S0.3: cat.MTReadSlotPolicy, the MT
// READ request's slot domain. A new file rather than an appendix to
// mt_test.go, for the reason mcpolicy_test.go's header gives: that file is
// pinned by file and ordinal in evidence-literals.golden.

// TestMTReadSlots_MemoryPMSRefusesSixtyAndEMGAtCodecAndGate is the
// disagreeing fixture: a dialect whose MT block prints memory and PMS only
// must refuse "MT501;" and "MTEMG;" at the CODEC and at the OUTBOUND GATE,
// while a dialect declaring MTReadsReadable builds and admits both.
//
// THE MR CONTROL IS THE OTHER HALF OF THE CLAIM, and without it this test
// would pass on a change that simply stopped reading 5xx and EMG at all: the
// same slots must still build an MR read and still be admitted, because on
// such a radio MR is the only command that reaches those banks.
func TestMTReadSlots_MemoryPMSRefusesSixtyAndEMGAtCodecAndGate(t *testing.T) {
	d := mtReadMemoryPMSDialect

	sixty, err := d.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("fixture broken: SixtyMSlot(1): %v", err)
	}
	emg := d.EMGSlot()
	if emg.Wire() == "" {
		t.Fatal("fixture broken: mtReadMemoryPMSDialect declares no emergency channel, so this test has nothing for the policy to refuse")
	}

	for _, s := range []Slot{sixty, emg} {
		cmd, err := d.BuildMTRead(s)
		if err == nil {
			t.Errorf("mtReadMemoryPMSDialect.BuildMTRead(%q) succeeded, emitting %q — its ReadSlots is MTReadsMemoryPMS, so this slot is outside its MT read domain", s.Wire(), cmd.Bytes())
		} else {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("BuildMTRead(%q): error is %T, want *ParseError", s.Wire(), err)
			}
			if !cmd.IsZero() {
				t.Errorf("BuildMTRead(%q) returned a non-zero Command alongside its error; every fallible builder returns Command{}", s.Wire())
			}
			if !strings.Contains(err.Error(), s.Wire()) {
				t.Errorf("BuildMTRead(%q) refused with %q, which does not name the slot", s.Wire(), err)
			}
			if !strings.Contains(err.Error(), MTReadsMemoryPMS.String()) {
				t.Errorf("BuildMTRead(%q) refused with %q, which does not name the policy %v", s.Wire(), err, MTReadsMemoryPMS)
			}
		}

		mtFrame := []byte("MT" + s.Wire() + ";")
		if d.AllowedCommand(mtFrame) {
			t.Errorf("mtReadMemoryPMSDialect's gate ADMITTED %q — the gate and the codec must share one read predicate", mtFrame)
		}

		// MR IS UNTOUCHED. The banks this dialect's MT legend omits are read
		// by MR, and that is the whole design of the axis: a narrowed MT read
		// must not narrow MR with it.
		mrCmd, err := d.BuildMRRead(s)
		if err != nil {
			t.Errorf("mtReadMemoryPMSDialect.BuildMRRead(%q) = %v — MTReadSlotPolicy governs MT alone; MR still reads this dialect's whole readable slot space", s.Wire(), err)
		} else if !d.AllowedCommand(mrCmd.Bytes()) {
			t.Errorf("mtReadMemoryPMSDialect's gate refused its own MR read %q", mrCmd.Bytes())
		}

		// POSITIVE CONTROL, same wire form, on a dialect declaring the wide
		// domain: the FT-710's MT column is ✓ for both classes.
		ft710Slot, err := FT710.ParseSlot(s.Wire())
		if err != nil {
			t.Fatalf("FT710.ParseSlot(%q): %v", s.Wire(), err)
		}
		if _, err := FT710.BuildMTRead(ft710Slot); err != nil {
			t.Errorf("FT710.BuildMTRead(%q) = %v — the FT-710 declares MTReadsReadable", s.Wire(), err)
		}
		if !FT710.AllowedCommand(mtFrame) {
			t.Errorf("FT710's gate refused %q — the FT-710 declares MTReadsReadable", mtFrame)
		}
	}

	// POSITIVE CONTROL on the narrowed dialect itself: the two classes its
	// own MT legend does print must still build and still be admitted.
	mem, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("fixture broken: MemorySlot(7): %v", err)
	}
	pms, err := d.PMSSlot(3, true)
	if err != nil {
		t.Fatalf("fixture broken: PMSSlot(3, true): %v", err)
	}
	for _, s := range []Slot{mem, pms} {
		cmd, err := d.BuildMTRead(s)
		if err != nil {
			t.Errorf("mtReadMemoryPMSDialect.BuildMTRead(%q) = %v — memory and PMS are inside its own MT read domain", s.Wire(), err)
			continue
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("mtReadMemoryPMSDialect's gate refused its own builder's %q", cmd.Bytes())
		}
	}

	// And the none form is still refused under the narrow policy, by the
	// FIRST of BuildMTRead's two refusals — the wording that the frame corpus
	// pins for the FT-710.
	none, err := d.ParseSlot("000")
	if err != nil {
		t.Fatalf("fixture broken: ParseSlot(\"000\"): %v", err)
	}
	if _, err := d.BuildMTRead(none); err == nil {
		t.Error("mtReadMemoryPMSDialect.BuildMTRead(\"000\") succeeded — the none form is not a read target under either policy")
	}
}

// TestMTReadSlots_RegisteredDialectsReadEverythingReadable pins the
// declaration the FT-710 makes, and pins it AGAINST readableSlot: under
// MTReadsReadable the new predicate must admit exactly what the old one did,
// which is what makes "every existing MT-read golden byte-identical" a
// property rather than a hope.
func TestMTReadSlots_RegisteredDialectsReadEverythingReadable(t *testing.T) {
	if got := FT710.mt.ReadSlots; got != MTReadsReadable {
		t.Fatalf("FT710 declares MT.ReadSlots %v, want MTReadsReadable — the FT-710 manual's slot table marks the MT column ✓ for 5xx and EMG as well as memory and PMS", got)
	}

	checked, agreed := 0, 0
	for _, nd := range allTestDialects() {
		if nd.dia.mt.ReadSlots != MTReadsReadable {
			continue
		}
		for n := 0; n <= 999; n++ {
			s, err := nd.dia.ParseSlot(threeDigits(n))
			if err != nil {
				continue
			}
			checked++
			if nd.dia.mtReadSlotValid(s) != nd.dia.readableSlot(s) {
				t.Errorf("%s: mtReadSlotValid(%q) = %v but readableSlot(%q) = %v — under MTReadsReadable the two must agree exactly, or an existing MT read has moved", nd.name, s.Wire(), nd.dia.mtReadSlotValid(s), s.Wire(), nd.dia.readableSlot(s))
				continue
			}
			agreed++
		}
	}
	if checked == 0 {
		t.Fatal("no MTReadsReadable dialect contributed a slot — this agreement would hold vacuously")
	}
	if agreed != checked {
		t.Errorf("%d of %d slots agreed", agreed, checked)
	}

	// And the narrow dialect must DISAGREE somewhere, or the check above
	// proves only that both predicates are the same function.
	disagreements := 0
	for n := 0; n <= 999; n++ {
		s, err := mtReadMemoryPMSDialect.ParseSlot(threeDigits(n))
		if err != nil {
			continue
		}
		if mtReadMemoryPMSDialect.mtReadSlotValid(s) != mtReadMemoryPMSDialect.readableSlot(s) {
			disagreements++
		}
	}
	if disagreements == 0 {
		t.Error("mtReadMemoryPMSDialect's MT read domain agrees with readableSlot at every slot — the fixture is not narrowing anything, and the agreement asserted above is vacuous")
	}
	t.Logf("MT read domain: %d slots agreed with readableSlot on the wide dialects, %d disagreements on the narrow one", agreed, disagreements)
}

// TestEveryDialect_MTReadGateAgreesWithItsBuilder is the builder/gate
// agreement over every dialect and every slot, the same property the MC
// split gets, for the same reason: dialecttest tolerates a refused MT read
// and would not see the two drifting apart.
func TestEveryDialect_MTReadGateAgreesWithItsBuilder(t *testing.T) {
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
			frame := []byte("MT" + s.Wire() + ";")
			_, buildErr := nd.dia.BuildMTRead(s)
			admitted := nd.dia.AllowedCommand(frame)
			checked++
			if buildErr == nil && !admitted {
				t.Errorf("%s: BuildMTRead(%q) built a frame its own gate then refused", nd.name, s.Wire())
			}
			if buildErr != nil {
				refused++
				if admitted {
					t.Errorf("%s: its gate ADMITTED %q while its own codec refused it (%v)", nd.name, frame, buildErr)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no slots were checked — the walk is not reaching any dialect's slot space")
	}
	if refused == 0 {
		t.Fatal("no MT read refusal was observed anywhere in the walk — the agreement would hold vacuously")
	}
	t.Logf("MT read builder/gate agreement checked over %d slots across %d dialects (%d refusals seen)", checked, len(allTestDialects()), refused)
}

// TestValidateDialectConfig_V9ReadSlots is the ReadSlots clause of V9: the
// zero value is refused UNDER EVERY FORM, and both policies are accepted
// under both. The per-form sweep is what stops the rule being written into
// one arm of the form switch by accident.
func TestValidateDialectConfig_V9ReadSlots(t *testing.T) {
	forms := []struct {
		name string
		mt   MTPolicy
	}{
		{"short", MTPolicy{Form: MTFormShort, TagMaxBytes: 8, ClearTagByte: '_'}},
		{"combined", MTPolicy{Form: MTFormCombined, TagMaxBytes: 8, TagFill: '_'}},
	}
	for _, f := range forms {
		t.Run(f.name+"/zero refused", func(t *testing.T) {
			cfg := validBaselineConfig()
			cfg.MT = f.mt
			cfg.MT.ReadSlots = 0
			err := validateDialectConfig(cfg)
			if err == nil {
				t.Fatal("validateDialectConfig() = nil for an omitted MT.ReadSlots — an omitted config semantic is refused, never defaulted")
			}
			if !strings.Contains(err.Error(), "ReadSlots") {
				t.Fatalf("validateDialectConfig() = %q, want it to name MT.ReadSlots", err)
			}
		})
		for _, p := range []MTReadSlotPolicy{MTReadsReadable, MTReadsMemoryPMS} {
			t.Run(f.name+"/"+p.String()+" accepted", func(t *testing.T) {
				cfg := validBaselineConfig()
				cfg.MT = f.mt
				cfg.MT.ReadSlots = p
				if err := validateDialectConfig(cfg); err != nil {
					t.Fatalf("validateDialectConfig() = %v, want accepted", err)
				}
			})
		}
	}
	t.Run("out-of-range refused", func(t *testing.T) {
		cfg := validBaselineConfig()
		cfg.MT.ReadSlots = MTReadSlotPolicy(42)
		err := validateDialectConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "ReadSlots") {
			t.Fatalf("validateDialectConfig() = %v, want an error naming MT.ReadSlots", err)
		}
	})
}

// TestMTReadSlotPolicy_String pins the names the refusals quote.
func TestMTReadSlotPolicy_String(t *testing.T) {
	for _, tc := range []struct {
		p    MTReadSlotPolicy
		want string
	}{
		{MTReadsReadable, "MTReadsReadable"},
		{MTReadsMemoryPMS, "MTReadsMemoryPMS"},
		{0, "MTReadSlotPolicy(0)"},
		{MTReadSlotPolicy(7), "MTReadSlotPolicy(7)"},
	} {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("MTReadSlotPolicy(%d).String() = %q, want %q", int(tc.p), got, tc.want)
		}
	}
}
