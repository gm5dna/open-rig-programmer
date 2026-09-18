// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// ftx1LikeDialectConfig is a SYNTHETIC dialect exercising the FTX-1 seam
// this file's tests cover: 5-digit slots, dash-token PMS (50 pairs),
// MCSelectsUnsupported, MTFormShortNoDisplay and the six-state tone
// domain. It is not the real FTX-1 dialect (F2's job, a future
// milestone) — only a minimal literal proving core/cat's seam accepts
// this shape at all, per the plan's Phase F1 brief ("core/cat seam only,
// no driver package, no fake yet").
func ftx1LikeDialectConfig() DialectConfig {
	return DialectConfig{
		CATID:         "0840",
		ModeNames:     map[Mode]string{ModeLSB: "LSB", ModeUSB: "USB"},
		EXAddressForm: EXAddressTriple,
		Slots: SlotSpace{
			MemoryLo:      1,
			MemoryHi:      999,
			PMSPairs:      50,
			PMSForm:       PMSFormDashToken,
			EmergencyWire: "EMGCH",
			NoneWire:      "00000",
			SlotDigits:    5,
			MCSelects:     MCSelectsUnsupported,
		},
		MT: MTPolicy{
			Form:        MTFormShortNoDisplay,
			ReadSlots:   MTReadsReadable,
			TagMaxBytes: 12,
			TagFill:     ' ',
		},
		Clarifier:        ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:         P5TxClar,
		ToneStates:       ToneStatesSix,
		MemoryFrameLen:   30,
		MemoryFreqDigits: 9,
		MemoryP9:         P9Fixed00,
		MWWriteKind:      KindMemory,
	}
}

// TestFTX1Seam_FiveDigitSlotRoundTrip covers MemorySlot's 5-digit wire form
// and ParseSlot accepting it back, under SlotSpace.SlotDigits: 5.
func TestFTX1Seam_FiveDigitSlotRoundTrip(t *testing.T) {
	d := MustNewDialect(ftx1LikeDialectConfig())

	s, err := d.MemorySlot(42)
	if err != nil {
		t.Fatalf("MemorySlot(42): %v", err)
	}
	if got, want := s.Wire(), "00042"; got != want {
		t.Fatalf("MemorySlot(42).Wire() = %q, want %q", got, want)
	}
	if !s.IsMemory() {
		t.Fatal("MemorySlot(42) is not classified as memory")
	}

	parsed, err := d.ParseSlot("00999")
	if err != nil {
		t.Fatalf("ParseSlot(%q): %v", "00999", err)
	}
	if !parsed.IsMemory() {
		t.Fatal("ParseSlot(\"00999\") is not classified as memory")
	}

	// A 3-byte wire form — the registered dialects' own width — must be
	// refused under a 5-digit dialect: the seam must not silently accept
	// the wrong width from either direction.
	if _, err := d.ParseSlot("042"); err == nil {
		t.Fatal("ParseSlot(\"042\") (3-byte form) should be refused under a 5-digit dialect")
	}

	if _, err := d.MemorySlot(1000); err == nil {
		t.Fatal("MemorySlot(1000) should be refused: outside the declared 1..999 range")
	}
}

// TestFTX1Seam_PMSDashTokenRoundTrip covers PMSFormDashToken's "P-%02d%c"
// wire form (dialectconfig.go), including pair 50 (the declared ceiling)
// and pair 51 (rejected).
func TestFTX1Seam_PMSDashTokenRoundTrip(t *testing.T) {
	d := MustNewDialect(ftx1LikeDialectConfig())

	lo, err := d.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("PMSSlot(1, false): %v", err)
	}
	if got, want := lo.Wire(), "P-01L"; got != want {
		t.Fatalf("PMSSlot(1, false).Wire() = %q, want %q", got, want)
	}

	hi, err := d.PMSSlot(50, true)
	if err != nil {
		t.Fatalf("PMSSlot(50, true): %v", err)
	}
	if got, want := hi.Wire(), "P-50U"; got != want {
		t.Fatalf("PMSSlot(50, true).Wire() = %q, want %q", got, want)
	}
	if !hi.IsPMS() {
		t.Fatal("PMSSlot(50, true) is not classified as PMS")
	}

	parsed, err := d.ParseSlot("P-50U")
	if err != nil {
		t.Fatalf("ParseSlot(%q): %v", "P-50U", err)
	}
	if !parsed.IsPMS() {
		t.Fatal("ParseSlot(\"P-50U\") is not classified as PMS")
	}

	// Pair 51 is past the declared 50-pair ceiling: PMSSlot must refuse to
	// build it, and classifySlot (via ParseSlot) must refuse to accept the
	// wire form even if handed one directly.
	if _, err := d.PMSSlot(51, false); err == nil {
		t.Fatal("PMSSlot(51, false) should be refused: pair 51 is past the 50-pair ceiling")
	}
	if _, err := d.ParseSlot("P-51L"); err == nil {
		t.Fatal("ParseSlot(\"P-51L\") should be refused: pair 51 is past the 50-pair ceiling")
	}
}

// TestFTX1Seam_MRMWRoundTrip30Byte covers the 30-byte MR-answer/MW-set
// frame a 5-digit-slot, 9-digit-frequency dialect implies (memoryFrameLenFor
// widened for slot width, memdata.go), including that BuildMWSet's slot
// bytes are not clobbered by the frequency field that follows them
// (Codex spec review BLOCKER 1).
func TestFTX1Seam_MRMWRoundTrip30Byte(t *testing.T) {
	d := MustNewDialect(ftx1LikeDialectConfig())

	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	want := MemoryData{
		Slot:   slot,
		FreqHz: 14074000,
		ClarHz: 0,
		Mode:   ModeUSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}

	cmd, err := d.BuildMWSet(want)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	if got := len(cmd.Bytes()); got != 30 {
		t.Fatalf("BuildMWSet frame is %d bytes, want 30", got)
	}
	// The slot field (bytes 2-6) must be the 5-digit slot, byte-for-byte —
	// this is exactly the encode-side half of BLOCKER 1: a hardcoded
	// memFreqOffset of 5 would have the frequency's own encode clobber the
	// last two bytes of this field.
	if got, want := string(cmd.Bytes()[2:7]), "00001"; got != want {
		t.Fatalf("MW frame slot field = %q, want %q (frame: %q)", got, want, cmd.Bytes())
	}

	// Re-parse as an MR answer (same field-block shape, different prefix)
	// and confirm every field round-trips.
	frame := append([]byte(nil), cmd.Bytes()...)
	frame[0], frame[1] = 'M', 'R'
	got, err := d.ParseMRAnswer(frame)
	if err != nil {
		t.Fatalf("ParseMRAnswer: %v (frame %q)", err, frame)
	}
	if got.Slot.Wire() != "00001" {
		t.Errorf("Slot.Wire() = %q, want %q", got.Slot.Wire(), "00001")
	}
	if got.FreqHz != want.FreqHz {
		t.Errorf("FreqHz = %d, want %d", got.FreqHz, want.FreqHz)
	}
	if got.Mode != want.Mode {
		t.Errorf("Mode = %v, want %v", got.Mode, want.Mode)
	}
	if got.Shift != want.Shift {
		t.Errorf("Shift = %v, want %v", got.Shift, want.Shift)
	}
}

// TestFTX1Seam_MTFormShortNoDisplayRoundTrip covers the new MTFormShortNoDisplay
// form: no display byte, a fixed-width (TagFill-padded) tag field, exact
// 20-byte frame for a 5-digit slot and a 12-byte tag.
func TestFTX1Seam_MTFormShortNoDisplayRoundTrip(t *testing.T) {
	d := MustNewDialect(ftx1LikeDialectConfig())

	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}

	cmd, err := d.BuildMTSetNoDisplay(slot, "CALLING")
	if err != nil {
		t.Fatalf("BuildMTSetNoDisplay: %v", err)
	}
	if got, want := len(cmd.Bytes()), 20; got != want {
		t.Fatalf("BuildMTSetNoDisplay frame is %d bytes, want %d (frame %q)", got, want, cmd.Bytes())
	}
	if got, want := string(cmd.Bytes()), "MT00001CALLING     ;"; got != want {
		t.Fatalf("BuildMTSetNoDisplay frame = %q, want %q", got, want)
	}

	gotSlot, gotTag, err := d.ParseMTAnswerNoDisplay(cmd.Bytes())
	if err != nil {
		t.Fatalf("ParseMTAnswerNoDisplay: %v", err)
	}
	if gotSlot.Wire() != "00001" {
		t.Errorf("Slot.Wire() = %q, want %q", gotSlot.Wire(), "00001")
	}
	if gotTag != "CALLING" {
		t.Errorf("tag = %q, want %q", gotTag, "CALLING")
	}

	// An empty tag is the all-fill field (mirroring the combined form's own
	// semantics, mtnodisplay.go's own doc comment) and must round-trip to
	// "" rather than a string of spaces.
	emptyCmd, err := d.BuildMTSetNoDisplay(slot, "")
	if err != nil {
		t.Fatalf("BuildMTSetNoDisplay(\"\"): %v", err)
	}
	_, emptyTag, err := d.ParseMTAnswerNoDisplay(emptyCmd.Bytes())
	if err != nil {
		t.Fatalf("ParseMTAnswerNoDisplay(empty): %v", err)
	}
	if emptyTag != "" {
		t.Errorf("empty tag round-trips to %q, want \"\"", emptyTag)
	}

	// Wrong form: calling BuildMTSet (the display-bearing short form) on a
	// MTFormShortNoDisplay dialect must refuse cleanly, not build a
	// plausible-but-wrong frame.
	if _, err := d.BuildMTSet(slot, true, "X"); err == nil {
		t.Fatal("BuildMTSet should refuse on a MTFormShortNoDisplay dialect")
	}
}

// TestFTX1Seam_MCUnsupportedRefusesCleanly covers MCSelectsUnsupported:
// BuildMCSet, ParseMCAnswer and the outbound gate must all refuse rather
// than build or misparse a coincidentally-shaped legacy MC frame.
func TestFTX1Seam_MCUnsupportedRefusesCleanly(t *testing.T) {
	d := MustNewDialect(ftx1LikeDialectConfig())

	if d.MCSupported() {
		t.Fatal("MCSupported() should be false under MCSelectsUnsupported")
	}

	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	if _, err := d.BuildMCSet(slot); err == nil {
		t.Fatal("BuildMCSet should refuse under MCSelectsUnsupported")
	}

	// A frame that LOOKS like a legacy 6-byte MC answer ("MC" + 3-byte slot
	// + ";") must not be misparsed as one.
	if _, err := d.ParseMCAnswer([]byte("MC001;")); err == nil {
		t.Fatal("ParseMCAnswer should refuse under MCSelectsUnsupported, not misparse a coincidentally-shaped frame")
	}

	// The outbound gate: neither the fixed read request nor a Set-shaped
	// frame may be admitted.
	if d.AllowedCommand([]byte("MC;")) {
		t.Fatal("AllowedCommand should refuse \"MC;\" under MCSelectsUnsupported")
	}
	if d.AllowedCommand([]byte("MC001;")) {
		t.Fatal("AllowedCommand should refuse a Set-shaped MC frame under MCSelectsUnsupported")
	}
}

// TestFTX1Seam_SixStateToneRoundTrip covers ToneStatesSix: all six P8
// values, including '4' and '5' (PR FREQ, REV TONE), which have no named
// CTCSSState constant but must still parse and round-trip.
func TestFTX1Seam_SixStateToneRoundTrip(t *testing.T) {
	d := MustNewDialect(ftx1LikeDialectConfig())

	for _, c := range []byte{'0', '1', '2', '3', '4', '5'} {
		state, err := d.ParseCTCSSState(c)
		if err != nil {
			t.Fatalf("ParseCTCSSState(%q) under ToneStatesSix: %v", c, err)
		}
		if got := state.Wire(); got != c {
			t.Errorf("ParseCTCSSState(%q).Wire() = %q, want %q", c, got, c)
		}
	}

	if _, err := d.ParseCTCSSState('6'); err == nil {
		t.Fatal("ParseCTCSSState('6') should be refused: outside the six-value domain")
	}

	// A full MR-answer round trip carrying P8 '4' (PR FREQ) must parse and
	// encode cleanly — the spec's own requirement ("a valid MR answer with
	// 4/5 must round-trip, never fail to parse").
	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	m := MemoryData{
		Slot:   slot,
		FreqHz: 7074000,
		Mode:   ModeUSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSState('4'),
		Shift:  ShiftSimplex,
	}
	cmd, err := d.BuildMWSet(m)
	if err != nil {
		t.Fatalf("BuildMWSet with CTCSS '4': %v", err)
	}
	frame := append([]byte(nil), cmd.Bytes()...)
	frame[0], frame[1] = 'M', 'R'
	got, err := d.ParseMRAnswer(frame)
	if err != nil {
		t.Fatalf("ParseMRAnswer with CTCSS '4': %v", err)
	}
	if got.CTCSS != CTCSSState('4') {
		t.Errorf("CTCSS round-tripped as %v, want CTCSSState('4')", got.CTCSS)
	}
}

// TestCTCSSStateString_SixStateDomainNamesBytesFourAndFive pins
// Dialect.CTCSSStateString: under ToneStatesSix it must name '4'/'5' from
// the FTX-1's own P8 legend ("PR FREQ"/"REV TONE"), not fall back to the
// shared ctcssNames table's FT-991A label ("DCS ENC", CTCSSDCSEnc's byte)
// — and every OTHER domain must defer to CTCSSState.String() unchanged,
// so the FT-991A's own diagnostic label is untouched by this seam.
func TestCTCSSStateString_SixStateDomainNamesBytesFourAndFive(t *testing.T) {
	six := MustNewDialect(ftx1LikeDialectConfig())
	if got := six.CTCSSStateString(CTCSSState('4')); got != "PR FREQ" {
		t.Errorf("CTCSSStateString('4') under ToneStatesSix = %q, want \"PR FREQ\"", got)
	}
	if got := six.CTCSSStateString(CTCSSState('5')); got != "REV TONE" {
		t.Errorf("CTCSSStateString('5') under ToneStatesSix = %q, want \"REV TONE\"", got)
	}

	cfg := pmsFormBaseConfig()
	cfg.ToneStates = ToneStatesCTCSSAndDCS
	fiveState := MustNewDialect(cfg)
	if got := fiveState.CTCSSStateString(CTCSSDCSEnc); got != "DCS ENC" {
		t.Errorf("CTCSSStateString(CTCSSDCSEnc) under ToneStatesCTCSSAndDCS = %q, want \"DCS ENC\" — the FT-991A label must survive this seam", got)
	}
}
