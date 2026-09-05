// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// --- S0.3: DialectConfig.ToneStates, the P8 state domain ---
//
// The FT-991A's P8 legend prints FIVE states on all five blocks that carry
// it — "0: CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS
// ENC" — where every registered sibling prints 0/1/2 only. The DCS states
// are therefore a per-radio fact, and a per-radio fact that reaches the
// OUTBOUND WRITE GATE: a MemoryData carrying CTCSSState('3') must be
// refused by every three-state dialect's builders AND by its gate, or this
// codec sends an FTdx10 a P8 byte its manual does not print.

// TestV16_ToneStatesRefusals: the domain must be declared, never inferred.
func TestV16_ToneStatesRefusals(t *testing.T) {
	cfg := pmsFormBaseConfig()
	cfg.ToneStates = ToneStateDomain(0)
	_, err := NewDialect(cfg)
	if err == nil {
		t.Fatal("NewDialect accepted a config with no ToneStates")
	}
	if !strings.Contains(err.Error(), "ToneStates") {
		t.Fatalf("refusal %q does not name ToneStates", err)
	}

	for _, domain := range []ToneStateDomain{ToneStatesCTCSS, ToneStatesCTCSSAndDCS} {
		cfg := pmsFormBaseConfig()
		cfg.ToneStates = domain
		d, err := NewDialect(cfg)
		if err != nil {
			t.Fatalf("NewDialect refused %v: %v", domain, err)
		}
		if got := d.ToneStates(); got != domain {
			t.Errorf("ToneStates() = %v, want %v", got, domain)
		}
	}
}

// TestParseCTCSSState_PackageFunctionIsUNCHANGED holds the exported
// package-level parser to its exact legacy behaviour and its exact text.
//
// It is RETAINED rather than widened or removed: it has a non-test consumer
// outside this package (core/cat/dialecttest) and core/transport's tests
// use it too, so removing it would be a public API break inconsistent with
// a MINOR release; and its refusal text is pinned four times in
// core/cat/testdata/parser-corpus.golden, which does not move. The DOMAIN
// travels on Dialect.ParseCTCSSState instead.
func TestParseCTCSSState_PackageFunctionIsUNCHANGED(t *testing.T) {
	for _, c := range []byte{'0', '1', '2'} {
		if got, err := ParseCTCSSState(c); err != nil || got != CTCSSState(c) {
			t.Errorf("ParseCTCSSState(%q) = %v, %v — want the byte back with no error", c, got, err)
		}
	}
	for _, c := range []byte{'3', '4', '5', '9', 'A'} {
		_, err := ParseCTCSSState(c)
		if err == nil {
			t.Fatalf("ParseCTCSSState(%q) accepted a byte outside the legacy domain", c)
		}
		if want := "invalid CTCSS code: want '0'-'2'"; !strings.Contains(err.Error(), want) {
			t.Errorf("ParseCTCSSState(%q) = %q, want it to contain %q verbatim", c, err, want)
		}
	}
}

// TestDialect_ParseCTCSSState_CarriesTheDomain is the receiver-sensitive
// parser, both ways.
func TestDialect_ParseCTCSSState_CarriesTheDomain(t *testing.T) {
	three := mustFixtureDialect(pmsFormBaseConfig()) // ToneStatesCTCSS

	fiveCfg := pmsFormBaseConfig()
	fiveCfg.ToneStates = ToneStatesCTCSSAndDCS
	five := mustFixtureDialect(fiveCfg)

	for _, c := range []byte{'0', '1', '2'} {
		if _, err := three.ParseCTCSSState(c); err != nil {
			t.Errorf("three-state dialect refused %q: %v", c, err)
		}
		if _, err := five.ParseCTCSSState(c); err != nil {
			t.Errorf("five-state dialect refused %q: %v", c, err)
		}
	}
	for _, c := range []byte{'3', '4'} {
		if _, err := three.ParseCTCSSState(c); err == nil {
			t.Errorf("three-state dialect ACCEPTED %q — its manual prints P8 0/1/2 only", c)
		} else if want := "invalid CTCSS code: want '0'-'2'"; !strings.Contains(err.Error(), want) {
			t.Errorf("three-state refusal of %q is %q, want the legacy text %q byte for byte", c, err, want)
		}
		got, err := five.ParseCTCSSState(c)
		if err != nil {
			t.Errorf("five-state dialect refused %q: %v", c, err)
		} else if got != CTCSSState(c) {
			t.Errorf("five-state ParseCTCSSState(%q) = %v, want the byte back", c, got)
		}
	}
	for _, c := range []byte{'5', '9', 'A'} {
		if _, err := five.ParseCTCSSState(c); err == nil {
			t.Errorf("five-state dialect ACCEPTED %q — the legend stops at '4'", c)
		} else if want := "invalid CTCSS code: want '0'-'4'"; !strings.Contains(err.Error(), want) {
			t.Errorf("five-state refusal of %q is %q, want it to name the domain %q", c, err, want)
		}
	}

	// The zero Dialect declares no domain and must accept nothing, exactly
	// as it accepts no frame: an omitted config semantic refuses.
	var zero Dialect
	for _, c := range []byte{'0', '1', '2', '3', '4'} {
		if _, err := zero.ParseCTCSSState(c); err == nil {
			t.Errorf("the zero Dialect accepted P8 %q", c)
		}
	}
}

// TestCTCSSState_DCSMembers pins the two new constants and their names.
func TestCTCSSState_DCSMembers(t *testing.T) {
	if CTCSSDCSEncDec != '3' || CTCSSDCSEnc != '4' {
		t.Fatalf("CTCSSDCSEncDec/CTCSSDCSEnc = %q/%q, want '3'/'4'", byte(CTCSSDCSEncDec), byte(CTCSSDCSEnc))
	}
	if got := CTCSSDCSEncDec.String(); got != "DCS ENC/DEC" {
		t.Errorf("CTCSSDCSEncDec.String() = %q, want %q", got, "DCS ENC/DEC")
	}
	if got := CTCSSDCSEnc.String(); got != "DCS ENC" {
		t.Errorf("CTCSSDCSEnc.String() = %q, want %q", got, "DCS ENC")
	}
	// The three legacy constants and their names do not move.
	if CTCSSOff != '0' || CTCSSEncDec != '1' || CTCSSEnc != '2' {
		t.Error("a legacy CTCSSState constant's value moved")
	}
	if got := CTCSSEncDec.String(); got != "ENC/DEC" {
		t.Errorf("CTCSSEncDec.String() = %q, want %q", got, "ENC/DEC")
	}
}

// TestToneStates_ADCSRecordCannotBeBuilt is the THREE-WAY red proof, over
// every dialect this package can see.
//
// A record carrying a DCS state must be refused by BuildMWSet, by
// BuildMTSetCombined AND by AllowedCommand on any three-state dialect —
// it cannot be BUILT by any route, and a frame forged elsewhere cannot get
// past the gate either. Widening only the codec's parse site would have let
// a MemoryData{CTCSS: CTCSSState('3')} be built and admitted for an FTdx10,
// FTdx101D/MP, FT-891 or FT-710, whose manuals print P8 0/1/2 only.
func TestToneStates_ADCSRecordCannotBeBuilt(t *testing.T) {
	for _, nd := range allTestDialects() {
		t.Run(nd.name, func(t *testing.T) {
			d := nd.dia
			if d.ToneStates() != ToneStatesCTCSS {
				t.Skipf("%v is not a three-state dialect", d.ToneStates())
			}
			slot, err := d.MemorySlot(d.slots.memoryLo)
			if err != nil {
				t.Fatalf("MemorySlot(%d): %v", d.slots.memoryLo, err)
			}
			mode, ok := firstRealMode(d)
			if !ok {
				t.Fatal("this dialect declares no emittable mode")
			}
			for _, state := range []CTCSSState{CTCSSDCSEncDec, CTCSSDCSEnc} {
				m := MemoryData{
					Slot: slot, FreqHz: 14250000, Mode: mode,
					Kind: d.mwWriteKind, CTCSS: state, Shift: ShiftSimplex,
				}
				if _, err := d.BuildMWSet(m); err == nil {
					t.Errorf("BuildMWSet built an MW frame carrying P8 %q", state.Wire())
				}
				m.Kind = CombinedMTSetKind
				if _, err := d.BuildMTSetCombined(m, "TAG"); err == nil {
					t.Errorf("BuildMTSetCombined built a combined MT frame carrying P8 %q", state.Wire())
				}
				// The gate, against a frame forged whole. It is built by
				// splicing the state byte into a frame this same dialect
				// built and its own gate admits, so the ONLY difference the
				// gate can be reacting to is P8.
				clean := m
				clean.Kind, clean.CTCSS = d.mwWriteKind, CTCSSOff
				cmd, err := d.BuildMWSet(clean)
				if err != nil {
					t.Fatalf("BuildMWSet of the clean record: %v", err)
				}
				if !d.AllowedCommand(cmd.Bytes()) {
					t.Fatalf("its own gate refused its own clean MW frame %q", cmd.Bytes())
				}
				forged := append([]byte(nil), cmd.Bytes()...)
				forged[memCTCSSOffset] = state.Wire()
				if d.AllowedCommand(forged) {
					t.Errorf("its own gate ADMITTED the forged MW frame %q, whose P8 is %q", forged, state.Wire())
				}
			}
		})
	}
}

// TestToneStates_FiveStateDialectBuildsAndParsesADCSRecord is the other
// direction: the axis must actually let the FT-991A's states through, or
// the refusals above would be indistinguishable from a codec that simply
// cannot express DCS at all.
func TestToneStates_FiveStateDialectBuildsAndParsesADCSRecord(t *testing.T) {
	cfg := pmsFormBaseConfig()
	cfg.ToneStates = ToneStatesCTCSSAndDCS
	d := mustFixtureDialect(cfg)

	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	m := MemoryData{
		Slot: slot, FreqHz: 14250000, Mode: ModeUSB,
		Kind: d.mwWriteKind, CTCSS: CTCSSDCSEncDec, Shift: ShiftSimplex,
	}
	cmd, err := d.BuildMWSet(m)
	if err != nil {
		t.Fatalf("BuildMWSet on a five-state dialect: %v", err)
	}
	if !d.AllowedCommand(cmd.Bytes()) {
		t.Fatalf("its own gate refused its own MW frame %q", cmd.Bytes())
	}
	// And the parse direction, through an MR answer built from the same
	// bytes: a '3' must come back as the DCS state rather than an error.
	answer := append([]byte("MR"), cmd.Bytes()[2:]...)
	back, err := d.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): %v", answer, err)
	}
	if back.CTCSS != CTCSSDCSEncDec {
		t.Errorf("ParseMRAnswer round-tripped P8 as %v, want %v", back.CTCSS, CTCSSDCSEncDec)
	}
}

// firstRealMode returns some mode this dialect may EMIT (never ModeUnset),
// so the tests above can build a record without hardcoding a mode table.
func firstRealMode(d Dialect) (Mode, bool) {
	for m := Mode(0x20); m <= Mode(0x7E); m++ {
		if m == ModeUnset {
			continue
		}
		if _, err := d.ParseMode(m.Wire()); err == nil {
			return m, true
		}
	}
	return 0, false
}

// TestToneStates_RegisteredDialectDeclaresTheCTCSSDomain holds the FT-710's
// own declaration. Its siblings assert the same thing in their own
// packages, against their own manuals, because a citation belongs beside
// the dialect it describes.
func TestToneStates_RegisteredDialectDeclaresTheCTCSSDomain(t *testing.T) {
	if got := FT710.ToneStates(); got != ToneStatesCTCSS {
		t.Errorf("FT710 declares ToneStates %v, want ToneStatesCTCSS — its reference gives P8 as \"CTCSS: 0 off, 1 ENC/DEC, 2 ENC\"", got)
	}
}
