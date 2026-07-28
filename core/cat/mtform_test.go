// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// shortFormBaseConfig is a KNOWN-GOOD short-form config: the baseline of
// dialectvalidate_test.go with the FT-710's own MT policy plus its form.
//
// The MT policy is the FT-710's deliberately — the short form is the one
// whose bytes this milestone must not move, so the ownership table's short
// base is the shape the radio actually has. Everything else stays the
// non-FT-710 baseline, so a rule accidentally written against FT-710 data
// still fails here rather than passing by coincidence.
func shortFormBaseConfig() DialectConfig {
	cfg := validBaselineConfig()
	cfg.MT = MTPolicy{Form: MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '}
	return cfg
}

// combinedFormBaseConfig is a KNOWN-GOOD combined-form config: the same
// baseline with the combined form's own field set — TagFill required,
// ClearTagByte and PadByte both absent.
//
// TagMaxBytes is 6 rather than the FTdx10's 12 so that a rule quietly
// assuming the evidenced width fails here.
func combinedFormBaseConfig() DialectConfig {
	cfg := validBaselineConfig()
	cfg.MT = MTPolicy{Form: MTFormCombined, TagMaxBytes: 6, TagFill: ' '}
	return cfg
}

// TestValidateMTPolicy_FormOwnership is V9's ownership table, in BOTH
// directions for every field: an inapplicable field must be explicitly
// zero, an applicable one explicitly valid (the M9c-2 ObservedCSV-under-
// ObservationsAbsent pattern).
//
// Driven through NewDialect rather than validateMTPolicy directly: the
// refusal a caller experiences is the constructor's, and a rule that is
// written but never reached from there would pass a direct-call test.
//
// Each entry perturbs exactly ONE field of a valid base, so a failure
// identifies the rule rather than the fixture.
func TestValidateMTPolicy_FormOwnership(t *testing.T) {
	tests := []struct {
		name    string
		base    func() DialectConfig
		mutate  func(*MTPolicy)
		wantErr string // "" means the config MUST be accepted
	}{
		{"short base accepted", shortFormBaseConfig, func(*MTPolicy) {}, ""},
		{"combined base accepted", combinedFormBaseConfig, func(*MTPolicy) {}, ""},

		// The zero value is not a form. An omitted Form must refuse rather
		// than default to the short one: defaulting would silently give an
		// FTdx10-shaped config the FT-710's frame layout.
		{"short base with Form omitted", shortFormBaseConfig, func(p *MTPolicy) { p.Form = MTFormUnspecified }, "MT.Form MTFormUnspecified must be set explicitly"},
		{"combined base with Form omitted", combinedFormBaseConfig, func(p *MTPolicy) { p.Form = MTFormUnspecified }, "MT.Form MTFormUnspecified must be set explicitly"},
		{"form outside the enum", shortFormBaseConfig, func(p *MTPolicy) { p.Form = MTForm(9) }, "MT.Form MTForm(9) must be set explicitly"},

		// Short form: TagFill is combined-form data.
		{"short form with TagFill set", shortFormBaseConfig, func(p *MTPolicy) { p.TagFill = ' ' }, "TagFill"},
		{"short form with an otherwise-valid TagFill", shortFormBaseConfig, func(p *MTPolicy) { p.TagFill = '_' }, "TagFill"},

		// Combined form: ClearTagByte and PadByte are short-form data.
		{"combined form with ClearTagByte set", combinedFormBaseConfig, func(p *MTPolicy) { p.ClearTagByte = ' ' }, "ClearTagByte"},
		{"combined form with PadByte set", combinedFormBaseConfig, func(p *MTPolicy) { p.PadByte = ' ' }, "PadByte"},

		// Combined form: TagFill is required and must be a wire byte. Zero
		// is refused rather than defaulted — an omitted fill would silently
		// emit NUL into every outbound tag field.
		{"combined form with TagFill omitted", combinedFormBaseConfig, func(p *MTPolicy) { p.TagFill = 0 }, "TagFill"},
		{"combined form with a terminator TagFill", combinedFormBaseConfig, func(p *MTPolicy) { p.TagFill = ';' }, "TagFill"},
		{"combined form with a control-byte TagFill", combinedFormBaseConfig, func(p *MTPolicy) { p.TagFill = 0x1F }, "TagFill"},
		{"combined form with a printable TagFill accepted", combinedFormBaseConfig, func(p *MTPolicy) { p.TagFill = '_' }, ""},

		// The pre-existing short-form rules survive the move into the
		// switch: without these, the MTFormShort case could lose its body
		// and this table stayed green.
		{"short form with a terminator ClearTagByte", shortFormBaseConfig, func(p *MTPolicy) { p.ClearTagByte = ';' }, "ClearTagByte"},
		{"short form with ClearTagByte omitted", shortFormBaseConfig, func(p *MTPolicy) { p.ClearTagByte = 0 }, "ClearTagByte"},
		{"short form with a terminator PadByte", shortFormBaseConfig, func(p *MTPolicy) { p.PadByte = ';' }, "PadByte"},
		{"short form with PadByte omitted accepted", shortFormBaseConfig, func(p *MTPolicy) { p.PadByte = 0 }, ""},

		// The TagMaxBytes cap stays ahead of the switch, so it applies to
		// both forms.
		{"short form tag width over the ceiling", shortFormBaseConfig, func(p *MTPolicy) { p.TagMaxBytes = maxMTTagBytes + 1 }, "TagMaxBytes"},
		{"combined form tag width over the ceiling", combinedFormBaseConfig, func(p *MTPolicy) { p.TagMaxBytes = maxMTTagBytes + 1 }, "TagMaxBytes"},
		{"combined form zero tag width", combinedFormBaseConfig, func(p *MTPolicy) { p.TagMaxBytes = 0 }, "TagMaxBytes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.base()
			tc.mutate(&cfg.MT)
			_, err := NewDialect(cfg)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("NewDialect() = %v, want accepted", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("NewDialect() = nil, want an error mentioning %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("NewDialect() = %q, want it to mention %q — a rule reporting the wrong field sends whoever has to fix it to the wrong place", err, tc.wantErr)
			}
		})
	}
}

// TestMTFrameCeilings_FitTransportFrame is the spec's "V9 proves each
// form's derived maximum fits DefaultMaxFrame", in the only honest
// shape: maxMTTagBytes caps TagMaxBytes at 64, so the maxima are
// compile-time facts, proven here rather than by an unreachable
// runtime branch.
func TestMTFrameCeilings_FitTransportFrame(t *testing.T) {
	if short := mtAnswerMinLen + maxMTTagBytes; short > DefaultMaxFrame {
		t.Errorf("short-form ceiling %d exceeds DefaultMaxFrame %d", short, DefaultMaxFrame)
	}
	if combined := 29 + maxMTTagBytes; combined > DefaultMaxFrame {
		t.Errorf("combined-form ceiling %d exceeds DefaultMaxFrame %d", combined, DefaultMaxFrame)
	}
}
