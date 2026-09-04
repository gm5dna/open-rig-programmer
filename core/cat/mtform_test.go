// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
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
	cfg.MT = MTPolicy{Form: MTFormShort, ReadSlots: MTReadsReadable, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '}
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
	cfg.MT = MTPolicy{Form: MTFormCombined, P11: P11Fixed, ReadSlots: MTReadsReadable, TagMaxBytes: 6, TagFill: ' '}
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

// mustShortPeerDialect is a SHORT-form peer whose tag is SIX bytes wide,
// modelled on mtpolicy_test.go's peer policy (TagMaxBytes 6, ClearTagByte
// '-') but built fresh here so this file's window assertions stand on their
// own fixture rather than on another file's helper.
//
// Six is the load-bearing number: its derived answer window is 7+6 = 13
// bytes, strictly narrower than the FT-710's 19, so a frame between the two
// separates a receiver-derived window from a package-wide one.
func mustShortPeerDialect(t *testing.T) Dialect {
	t.Helper()
	cfg := validBaselineConfig()
	cfg.MT = MTPolicy{Form: MTFormShort, ReadSlots: MTReadsReadable, TagMaxBytes: 6, ClearTagByte: '-'}
	return MustNewDialect(cfg)
}

// mustCombinedFormDialect is a valid COMBINED-form dialect: the file's own
// combined base, constructed. It exists to be offered to the SHORT-form API,
// which must refuse it by form rather than fail somewhere downstream on a
// frame shape it cannot describe.
func mustCombinedFormDialect(t *testing.T) Dialect {
	t.Helper()
	return MustNewDialect(combinedFormBaseConfig())
}

// TestParseMTAnswer_FT710WindowTextIsUnchanged pins the FT-710's
// out-of-window error text EXACTLY, character for character.
//
// This is the milestone's byte-identity bar reaching into an error string:
// the parser corpus records this Reason verbatim, so a derived window that
// rendered "7-18", "7-20" or a differently-worded message would move a
// recorded byte even though every accepted frame still parsed. The
// comparison is against the *ParseError's Reason rather than Error(), so a
// change to the wrapper's formatting cannot silently satisfy it.
func TestParseMTAnswer_FT710WindowTextIsUnchanged(t *testing.T) {
	frame := []byte("MT0011ABCDEFGHIJKLM;") // 20 bytes: one past the FT-710's 19
	if len(frame) != 20 {
		t.Fatalf("fixture is %d bytes, want 20 — the case is only out-of-window at 20", len(frame))
	}

	_, _, _, err := FT710.ParseMTAnswer(frame)
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("FT710.ParseMTAnswer(%q): error is %T, want *ParseError", frame, err)
	}
	const want = "MT answer must be 7-19 bytes"
	if pe.Reason != want {
		t.Errorf("FT710.ParseMTAnswer(%q) Reason = %q, want exactly %q — this string is recorded in the parser corpus", frame, pe.Reason, want)
	}
}

// TestParseMTAnswer_ShortWindowDerivesFromTheReceiver is the
// derived-window disagreement: ONE frame, accepted by the FT-710 and
// refused by the six-byte peer, which is impossible while the window is a
// package constant sized to the FT-710's own tag.
//
// Fourteen bytes is "MT" + slot(3) + display(1) + a 7-byte tag + ";". The
// peer's own maximum is 7+6 = 13, so the frame is one past its ceiling and
// five inside the FT-710's — the same disagreement mtpolicy_test.go's
// TestMTPolicy_GateHonoursPeerTagLength proves at the gate, here on the
// PARSE side, where the tag body is deliberately not charset-checked and
// the window is therefore the only thing that can reject it.
//
// Both boundary neighbours are asserted too: the peer must still accept its
// own 13-byte maximum (otherwise a parser that refused every MT Set frame
// would satisfy the refusal above), and its error text must render its OWN
// bounds, not the FT-710's.
func TestParseMTAnswer_ShortWindowDerivesFromTheReceiver(t *testing.T) {
	peer := mustShortPeerDialect(t)

	overPeer := []byte("MT0101ABCDEFG;") // 14 bytes: 7-byte tag, slot 010 (the baseline's 10-40 range)
	if len(overPeer) != 14 {
		t.Fatalf("fixture is %d bytes, want 14", len(overPeer))
	}

	_, _, _, err := peer.ParseMTAnswer(overPeer)
	if err == nil {
		t.Fatalf("peer.ParseMTAnswer(%q) succeeded, though its TagMaxBytes is 6 — the window is still the FT-710's 19", overPeer)
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("peer.ParseMTAnswer(%q): error is %T, want *ParseError", overPeer, err)
	}
	const wantPeer = "MT answer must be 7-13 bytes"
	if pe.Reason != wantPeer {
		t.Errorf("peer.ParseMTAnswer(%q) Reason = %q, want %q — the message must report the RECEIVER's bounds", overPeer, pe.Reason, wantPeer)
	}

	// The peer's own maximum, 13 bytes, still parses.
	atPeerMax := []byte("MT0101ABCDEF;")
	if _, _, tag, err := peer.ParseMTAnswer(atPeerMax); err != nil {
		t.Errorf("peer.ParseMTAnswer(%q) = %v, want accepted — 13 bytes is its own maximum", atPeerMax, err)
	} else if tag != "ABCDEF" {
		t.Errorf("peer.ParseMTAnswer(%q) tag = %q, want %q", atPeerMax, tag, "ABCDEF")
	}

	// The FT-710 positive control: the SAME 14-byte frame, well inside its
	// own 19-byte window, must still parse. Without this the refusal above
	// could be satisfied by narrowing the window for everybody.
	if _, _, tag, err := FT710.ParseMTAnswer(overPeer); err != nil {
		t.Errorf("FT710.ParseMTAnswer(%q) = %v, want accepted — 14 bytes is well inside its own 19", overPeer, err)
	} else if tag != "ABCDEFG" {
		t.Errorf("FT710.ParseMTAnswer(%q) tag = %q, want %q", overPeer, tag, "ABCDEFG")
	}
}

// TestMTShortAPIs_RefuseACombinedDialect is the form seam on the SHORT
// API: BuildMTSet and ParseMTAnswer describe one frame shape, and offered a
// dialect declaring the other they must refuse BY FORM — naming it, so the
// caller is sent to the combined API rather than left to read a length or
// charset complaint about a frame whose layout was never theirs.
//
// A silent success is the failure this prevents: the combined form's frame
// begins "MT" and ends ';' too, so a short-form parser handed one would find
// a plausible slot field and return data from the wrong offsets.
func TestMTShortAPIs_RefuseACombinedDialect(t *testing.T) {
	combined := mustCombinedFormDialect(t)

	slot, err := combined.MemorySlot(10)
	if err != nil {
		t.Fatalf("MemorySlot(10) on the baseline's own 10-40 range: %v", err)
	}

	cmd, err := combined.BuildMTSet(slot, true, "CQ")
	if err == nil {
		t.Fatalf("combined.BuildMTSet() succeeded, want a form refusal — it would emit a short-form frame to a combined-form radio")
	}
	if !strings.Contains(err.Error(), "MTFormCombined") {
		t.Errorf("combined.BuildMTSet() = %q, want the message to name the dialect's form (MTFormCombined)", err)
	}
	if !cmd.IsZero() {
		t.Errorf("combined.BuildMTSet() returned a non-zero Command alongside its error; every fallible builder returns Command{} (command.go)")
	}

	answer := []byte("MT0101CQ;")
	_, _, _, err = combined.ParseMTAnswer(answer)
	if err == nil {
		t.Fatalf("combined.ParseMTAnswer(%q) succeeded, want a form refusal", answer)
	}
	if !strings.Contains(err.Error(), "MTFormCombined") {
		t.Errorf("combined.ParseMTAnswer(%q) = %q, want the message to name the dialect's form (MTFormCombined)", answer, err)
	}

	// BuildMTRead is form-INDEPENDENT: the 6-byte read request is "MT" +
	// slot + ";" in both forms, so refusing it here would be over-reach.
	if _, err := combined.BuildMTRead(slot); err != nil {
		t.Errorf("combined.BuildMTRead() = %v, want accepted — the read request's shape is the same under both forms", err)
	}
}

// TestMTForm_Accessor pins the exported accessor across all three states a
// caller can meet: each configured form, and the zero Dialect.
//
// The zero case is the one worth writing down. MTFormUnspecified is what an
// unconfigured dialect reports, so a consumer switching on the form (the
// dialecttest suite, and M9c-4's real FTdx10 dialect) meets an explicit
// "no form" rather than a plausible-looking short form.
func TestMTForm_Accessor(t *testing.T) {
	if got := FT710.MTForm(); got != MTFormShort {
		t.Errorf("FT710.MTForm() = %v, want MTFormShort", got)
	}
	if got := mustCombinedFormDialect(t).MTForm(); got != MTFormCombined {
		t.Errorf("combined.MTForm() = %v, want MTFormCombined", got)
	}
	var zero Dialect
	if got := zero.MTForm(); got != MTFormUnspecified {
		t.Errorf("zero Dialect MTForm() = %v, want MTFormUnspecified", got)
	}
}

// TestFT710_WriteKindAndClarifierAccessors pins the other two accessors
// against the FT-710's own literal (dialect.go).
//
// They exist for the ft710 driver's write path — which formerly hardcoded
// both values, and since M9c-3 task 9 consults these accessors — and for
// the exported conformance suite. Pinning them here means a later edit to
// the FT-710 literal cannot change what the driver writes without a test
// saying so.
func TestFT710_WriteKindAndClarifierAccessors(t *testing.T) {
	if got := FT710.MWWriteKind(); got != KindMemory {
		t.Errorf("FT710.MWWriteKind() = %q, want KindMemory (%q)", got, KindMemory)
	}
	want := ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990}
	if got := FT710.Clarifier(); got != want {
		t.Errorf("FT710.Clarifier() = %+v, want %+v", got, want)
	}
}
