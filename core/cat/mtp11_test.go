// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// This file is the FT-891 Stage 0 seam S0.6: cat.MTP11Policy, byte 28 of the
// combined MT record. A new file rather than an appendix to
// mtcombined_test.go, for the reason mcpolicy_test.go's header gives — and
// mtcombined_test.go's own badP11 case (~:423) stands untouched, which is
// half the evidence that P11Fixed is behaviour-identical.

// p11TestRecord is a record combined-form fixtures in this file can write.
func p11TestRecord(t *testing.T, d Dialect) MemoryData {
	t.Helper()

	// The first slot this dialect's own MT write policy admits, found by
	// sweep rather than by a fixed number: the fixtures' memory ranges
	// deliberately disagree (100-200 on the peers), and a hardcoded channel
	// would silently skip whichever of them did not have it.
	var slot Slot
	for n := 0; n <= 999; n++ {
		s, err := d.ParseSlot(threeDigits(n))
		if err == nil && d.mtSlotValid(s) {
			slot = s
			break
		}
	}
	if slot.Wire() == "" {
		t.Fatal("fixture broken: the dialect admits no MT-writable slot")
	}
	var mode Mode
	for i := 0; i < 256; i++ {
		m := Mode(byte(i))
		if d.ValidMode(m) && m != ModeUnset {
			mode = m
			break
		}
	}
	if mode == 0 {
		t.Fatal("fixture broken: the dialect declares no emittable mode")
	}
	return MemoryData{
		Slot: slot, FreqHz: 14_250_000,
		Mode: mode, Kind: CombinedMTSetKind,
		CTCSS: CTCSSOff, Shift: ShiftSimplex,
	}
}

// TestMTP11_TheFourRefusals is the seam: neither pair of combined APIs will
// serve the other policy's dialect, in either direction.
//
// A LIVE FLAG IS NEVER DEFAULTED is the whole of the first two: on a
// P11TagDisplay dialect the display-less builder and parser refuse rather
// than writing '0' for a flag the caller never expressed an intention about,
// and reading back a record whose flag was silently dropped. The other two
// are the mirror: on a P11Fixed dialect a caller asking to set a TAG flag
// has misunderstood the radio, whose manual prints byte 28 "(Fixed)".
//
// Every refusal is paired with the RIGHT pair succeeding on the same
// dialect and the same record, so a change that simply broke one API cannot
// pass for a seam.
func TestMTP11_TheFourRefusals(t *testing.T) {
	tagDisplay := combinedTagDisplayDialect
	fixed := combinedDialect

	tdRec := p11TestRecord(t, tagDisplay)
	fxRec := p11TestRecord(t, fixed)

	// 1. Display-less BUILD under P11TagDisplay: refused.
	cmd, err := tagDisplay.BuildMTSetCombined(tdRec, "AB")
	if err == nil {
		t.Errorf("combinedTagDisplayDialect.BuildMTSetCombined succeeded, emitting %q — its P11 is a live TAG flag and this builder has no flag to offer, so writing '0' would be exactly the silent defaulting the M9c-1 ruling forbids", cmd.Bytes())
	} else {
		if !cmd.IsZero() {
			t.Error("BuildMTSetCombined returned a non-zero Command alongside its P11 refusal")
		}
		if !strings.Contains(err.Error(), "live TAG flag") {
			t.Errorf("the refusal %q does not say why: it must name the live flag and point at the display-bearing API", err)
		}
	}
	// ... and the display-BEARING build succeeds on the same record.
	on, err := tagDisplay.BuildMTSetCombinedDisplay(tdRec, "AB", true)
	if err != nil {
		t.Fatalf("combinedTagDisplayDialect.BuildMTSetCombinedDisplay = %v — it is that policy's own builder", err)
	}

	// 2. Display-less PARSE under P11TagDisplay: refused, on a frame that
	//    dialect really did build.
	if _, _, err := tagDisplay.ParseMTAnswerCombined(on.Bytes()); err == nil {
		t.Errorf("combinedTagDisplayDialect.ParseMTAnswerCombined accepted %q — it would hand back a record with the radio's TAG flag silently dropped", on.Bytes())
	}
	gotM, gotTag, gotDisplay, err := tagDisplay.ParseMTAnswerCombinedDisplay(on.Bytes())
	if err != nil {
		t.Fatalf("ParseMTAnswerCombinedDisplay(%q) = %v — a frame its own builder produced must parse", on.Bytes(), err)
	}
	if gotM != tdRec {
		t.Errorf("the record round-tripped to %+v, want %+v", gotM, tdRec)
	}
	if gotTag != "AB" {
		t.Errorf("the tag round-tripped to %q, want %q", gotTag, "AB")
	}
	if !gotDisplay {
		t.Error("a frame built with the TAG flag ON came back with it OFF — the flag is not surviving its own round trip")
	}

	// 3. Display-BEARING build under P11Fixed: refused.
	cmd, err = fixed.BuildMTSetCombinedDisplay(fxRec, "AB", true)
	if err == nil {
		t.Errorf("combinedDialect.BuildMTSetCombinedDisplay succeeded, emitting %q — its manual prints byte 28 \"(Fixed)\", so there is no TAG flag for a caller to set", cmd.Bytes())
	} else {
		if !cmd.IsZero() {
			t.Error("BuildMTSetCombinedDisplay returned a non-zero Command alongside its P11 refusal")
		}
		if !strings.Contains(err.Error(), P11Fixed.String()) {
			t.Errorf("the refusal %q does not name this dialect's policy %v", err, P11Fixed)
		}
	}
	fx, err := fixed.BuildMTSetCombined(fxRec, "AB")
	if err != nil {
		t.Fatalf("combinedDialect.BuildMTSetCombined = %v — it is that policy's own builder", err)
	}

	// 4. Display-BEARING parse under P11Fixed: refused.
	if _, _, _, err := fixed.ParseMTAnswerCombinedDisplay(fx.Bytes()); err == nil {
		t.Errorf("combinedDialect.ParseMTAnswerCombinedDisplay accepted %q — reporting a printed-fixed byte as a flag reads schema as state", fx.Bytes())
	}
	if _, _, err := fixed.ParseMTAnswerCombined(fx.Bytes()); err != nil {
		t.Errorf("combinedDialect.ParseMTAnswerCombined(%q) = %v — it is that policy's own parser", fx.Bytes(), err)
	}
}

// TestMTP11_FixedIsBehaviourIdentical is the byte-identity half: under
// P11Fixed the display-less pair does exactly what it did before the policy
// existed — byte 28 is combinedMTP11, and a frame carrying anything else is
// refused with the same words.
//
// It is asserted over EVERY P11Fixed combined fixture rather than one,
// because a policy read off the wrong receiver would show up as one dialect
// behaving and another not.
func TestMTP11_FixedIsBehaviourIdentical(t *testing.T) {
	checked := 0
	for _, nd := range allTestDialects() {
		d := nd.dia
		if d.MTForm() != MTFormCombined || d.MTP11() != P11Fixed {
			continue
		}
		m := p11TestRecord(t, d)
		cmd, err := d.BuildMTSetCombined(m, "A")
		if err != nil {
			t.Errorf("%s: BuildMTSetCombined = %v", nd.name, err)
			continue
		}
		frame := cmd.Bytes()
		if got := frame[mtCombinedP11Offset]; got != combinedMTP11 {
			t.Errorf("%s: byte 28 of its combined Set is %q, want the printed-fixed %q", nd.name, got, combinedMTP11)
		}
		if !d.AllowedCommand(frame) {
			t.Errorf("%s: its own gate refused %q", nd.name, frame)
		}
		forged := append([]byte(nil), frame...)
		forged[mtCombinedP11Offset] = '1'
		if _, _, err := d.ParseMTAnswerCombined(forged); err == nil {
			t.Errorf("%s: its parser accepted %q, whose P11 is '1' under P11Fixed", nd.name, forged)
		} else if !strings.Contains(err.Error(), "P11 must be fixed") {
			t.Errorf("%s: the P11 refusal reads %q — the wording is pinned by mtcombined_test.go's badP11 case and must not move", nd.name, err)
		}
		if d.AllowedCommand(forged) {
			t.Errorf("%s: its gate ADMITTED %q, whose P11 is '1' under P11Fixed", nd.name, forged)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no P11Fixed combined dialect was checked — this property ran vacuously")
	}
}

// TestMTP11_TagDisplayGateAdmitsBothValuesAndNothingElse is the gate's own
// arm of the axis. It cannot ask which builder made a frame, so it asks the
// policy — and under P11TagDisplay it must admit BOTH documented values and
// refuse an undocumented third, which is what stops the widened branch
// becoming "any byte at position 28".
func TestMTP11_TagDisplayGateAdmitsBothValuesAndNothingElse(t *testing.T) {
	d := combinedTagDisplayDialect
	m := p11TestRecord(t, d)

	for _, display := range []bool{false, true} {
		cmd, err := d.BuildMTSetCombinedDisplay(m, "A", display)
		if err != nil {
			t.Fatalf("BuildMTSetCombinedDisplay(display=%v) = %v", display, err)
		}
		want := byte('0')
		if display {
			want = '1'
		}
		if got := cmd.Bytes()[mtCombinedP11Offset]; got != want {
			t.Errorf("BuildMTSetCombinedDisplay(display=%v) put %q at byte 28, want %q", display, got, want)
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("its own gate refused %q", cmd.Bytes())
		}
	}

	base, err := d.BuildMTSetCombinedDisplay(m, "A", false)
	if err != nil {
		t.Fatal(err)
	}
	forged := append([]byte(nil), base.Bytes()...)
	forged[mtCombinedP11Offset] = '2'
	if d.AllowedCommand(forged) {
		t.Errorf("its gate ADMITTED %q, whose byte 28 is '2' — the TAG flag has two documented values and a third is an undocumented frame", forged)
	}
	if _, _, _, err := d.ParseMTAnswerCombinedDisplay(forged); err == nil {
		t.Errorf("its parser accepted %q, whose byte 28 is '2'", forged)
	} else if !strings.Contains(err.Error(), P11TagDisplay.String()) {
		t.Errorf("the refusal %q does not name this dialect's policy %v", err, P11TagDisplay)
	}
}

// TestValidateDialectConfig_V9P11 is the P11 clause of V9, in both
// directions of the per-form ownership rule the MTPolicy table states:
// required under MTFormCombined, and required to be ZERO under MTFormShort.
//
// The short-form half is the one the FT-710 needs: it declares no P11 at
// all, because the short form's display flag is already a parameter of
// BuildMTSet, and a config that set one would be describing a byte its frame
// does not have.
func TestValidateDialectConfig_V9P11(t *testing.T) {
	combined := func(p MTP11Policy) DialectConfig {
		cfg := validBaselineConfig()
		cfg.MT = MTPolicy{
			Form: MTFormCombined, ReadSlots: MTReadsReadable,
			TagMaxBytes: 8, TagFill: '_', P11: p,
		}
		return cfg
	}
	short := func(p MTP11Policy) DialectConfig {
		cfg := validBaselineConfig()
		cfg.MT = MTPolicy{
			Form: MTFormShort, ReadSlots: MTReadsReadable,
			TagMaxBytes: 8, ClearTagByte: '_', P11: p,
		}
		return cfg
	}

	tests := []struct {
		name    string
		cfg     DialectConfig
		wantErr string // "" means the config MUST be accepted
	}{
		{"combined/zero refused", combined(0), "P11"},
		{"combined/out-of-range refused", combined(MTP11Policy(9)), "P11"},
		{"combined/P11Fixed accepted", combined(P11Fixed), ""},
		{"combined/P11TagDisplay accepted", combined(P11TagDisplay), ""},
		{"short/zero accepted", short(0), ""},
		{"short/P11Fixed refused", short(P11Fixed), "P11"},
		{"short/P11TagDisplay refused", short(P11TagDisplay), "P11"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDialectConfig(tc.cfg)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("validateDialectConfig() = %v, want accepted", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("validateDialectConfig() = nil, want an error mentioning %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("validateDialectConfig() = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}

	// The FT-710 is the live instance of the short-form half.
	if got := FT710.MTP11(); got != 0 {
		t.Errorf("FT710 declares MT.P11 %v — it is a short-form dialect, whose display flag is already a parameter of BuildMTSet", got)
	}
}

// TestMTP11_ZeroPolicyRefusesRatherThanDefaultingWide is a
// defense-in-depth arm the S0-MEM lane review's LOW-3 finding asked for: the
// config validator (V9) already refuses a zero MTP11Policy on a combined-form
// dialect, so this test proves the SAME refusal holds one layer further in,
// at the site that actually reads the field, in case a caller ever reaches
// it without going through NewDialect. A zero MTP11Policy on a combined-form
// dialect is impossible past NewDialect's V9 clause (dialectvalidate.go) —
// which is why this test builds its dialect by copying a valid one and
// zeroing the field directly, rather than through NewDialect, to reach code
// the config validator cannot.
//
// buildMTSetCombined's byte-28 write must REFUSE on this value, not
// silently take the P11Fixed (wide) reading — the same posture p11Valid
// (mtcombined.go) enforces on the read side via the shared predicate.
//
// EXTENDED at the S0-close review's HIGH-1 finding to reach p11Valid
// itself, not just the builder that consults it: p11Valid's pre-fix
// if/else took the P11Fixed reading for "anything that is not
// P11TagDisplay", zero included, so both combined parsers and the gate
// admitted a frame whose byte 28 happened to be combinedMTP11 ('0') even
// under a zero policy. See TestUnsetPolicy_MTP11_RefusesAtBuilderParserAndGate
// (unsetpolicy_test.go) for the fuller walk over all four Stage 0 axes;
// this is the one Codex's review named directly, so its assertions live
// here too.
func TestMTP11_ZeroPolicyRefusesRatherThanDefaultingWide(t *testing.T) {
	d := combinedDialect
	d.mt.P11 = 0

	rec := p11TestRecord(t, d)
	if cmd, err := d.BuildMTSetCombined(rec, "AB"); err == nil {
		t.Errorf("BuildMTSetCombined with a zero MT.P11 succeeded, emitting %q — an unset policy must refuse, not default to the wide (P11Fixed) reading", cmd.Bytes())
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("the refusal %q does not mention P11", err)
	}
	if cmd, err := d.BuildMTSetCombinedDisplay(rec, "AB", true); err == nil {
		t.Errorf("BuildMTSetCombinedDisplay with a zero MT.P11 succeeded, emitting %q — an unset policy must refuse this API too", cmd.Bytes())
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("the refusal %q does not mention P11", err)
	}

	// A frame combinedDialect (P11Fixed, unzeroed) really did build, so
	// byte 28 is combinedMTP11 ('0') — exactly the byte the pre-fix
	// p11Valid's else-arm admitted for any non-TagDisplay policy.
	cmd, err := combinedDialect.BuildMTSetCombined(rec, "AB")
	if err != nil {
		t.Fatalf("fixture broken: combinedDialect.BuildMTSetCombined: %v", err)
	}
	frame := cmd.Bytes()

	if m, tag, err := d.ParseMTAnswerCombined(frame); err == nil {
		t.Errorf("ParseMTAnswerCombined(%q) with a zero MT.P11 accepted it, returning (%+v, %q) — p11Valid must refuse rather than read byte 28 as the printed-fixed schema byte", frame, m, tag)
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("the refusal %q does not mention P11", err)
	}
	if m, tag, disp, err := d.ParseMTAnswerCombinedDisplay(frame); err == nil {
		t.Errorf("ParseMTAnswerCombinedDisplay(%q) with a zero MT.P11 accepted it, returning (%+v, %q, %v) — p11Valid must refuse rather than read byte 28 as a live TAG flag", frame, m, tag, disp)
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("the refusal %q does not mention P11", err)
	}
	if d.AllowedCommand(frame) {
		t.Errorf("the gate ADMITTED %q under a zero MT.P11 — p11Valid gates AllowedCommand too, and must refuse there as well as in the parsers", frame)
	}
}

// TestMTP11Policy_String pins the names the refusals quote.
func TestMTP11Policy_String(t *testing.T) {
	for _, tc := range []struct {
		p    MTP11Policy
		want string
	}{
		{P11Fixed, "P11Fixed"},
		{P11TagDisplay, "P11TagDisplay"},
		{0, "MTP11Policy(0)"},
		{MTP11Policy(7), "MTP11Policy(7)"},
	} {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("MTP11Policy(%d).String() = %q, want %q", int(tc.p), got, tc.want)
		}
	}
}
