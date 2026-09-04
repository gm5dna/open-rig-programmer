// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// This file is the FT-891 Stage 0 seam S0.4: cat.MemoryP5Policy, byte 21 of
// the shared 28-position memory field block. A new file rather than an
// appendix to memdata_test.go or mw_test.go, for the reason
// mcpolicy_test.go's header gives.

// p5TestRecord is a record every dialect in this file can write: a memory
// slot inside its own space, its own emittable mode and its own declared MW
// write kind, with TxClar set as the caller asks.
func p5TestRecord(t *testing.T, d Dialect, txClar bool) MemoryData {
	t.Helper()

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("fixture broken: MemorySlot(7): %v", err)
	}
	var mode Mode
	found := false
	for i := 0; i < 256; i++ {
		m := Mode(byte(i))
		if d.ValidMode(m) && m != ModeUnset {
			mode, found = m, true
			break
		}
	}
	if !found {
		t.Fatal("fixture broken: the dialect declares no emittable mode")
	}
	return MemoryData{
		Slot:   slot,
		FreqHz: 14_250_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: txClar,
		Mode:   mode,
		Kind:   d.MWWriteKind(),
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
}

// TestMemoryP5_FixedRefusesATxClarRecordAtBuilderAndGate is the truth table
// the seam exists for, in the WRITE direction.
//
// Under P5Fixed a record asking for the TX clarifier is REFUSED rather than
// quietly encoded as '0' — the validate-don't-rewrite posture every other
// field in these validators takes — and the refusal comes from the builder
// AND from the gate, which reaches the same validator. Under P5TxClar the
// identical record builds, carries '1' at position 21, and is admitted.
func TestMemoryP5_FixedRefusesATxClarRecordAtBuilderAndGate(t *testing.T) {
	// P5TxClar: the positive control, and it is what stops this test passing
	// on a change that simply broke TxClar everywhere.
	wide := p5TestRecord(t, FT710, true)
	cmd, err := FT710.BuildMWSet(wide)
	if err != nil {
		t.Fatalf("FT710.BuildMWSet with TxClar true = %v — the FT-710's MW block prints P5 \"0: TX CLAR OFF 1: TX CLAR ON\", so both values are writable", err)
	}
	wideFrame := cmd.Bytes()
	if got := wideFrame[memTxClarOffset]; got != '1' {
		t.Errorf("FT710.BuildMWSet with TxClar true emitted %q, whose P5 byte is %q, want '1'", wideFrame, got)
	}
	if !FT710.AllowedCommand(wideFrame) {
		t.Errorf("FT710's gate refused its own builder's %q", wideFrame)
	}

	// P5Fixed: the refusal, at the builder.
	narrow := p5TestRecord(t, p5FixedDialect, true)
	cmd, err = p5FixedDialect.BuildMWSet(narrow)
	if err == nil {
		t.Errorf("p5FixedDialect.BuildMWSet with TxClar true succeeded, emitting %q — its manual prints P5 \"(Fixed)\", so the flag must be refused, not silently encoded as '0'", cmd.Bytes())
	} else {
		if !cmd.IsZero() {
			t.Error("p5FixedDialect.BuildMWSet returned a non-zero Command alongside its P5 refusal")
		}
		for _, want := range []string{"P5", P5Fixed.String()} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("p5FixedDialect.BuildMWSet refused with %q, which does not mention %q", err, want)
			}
		}
	}

	// And at the gate, on a frame whose P5 byte is '1'. It is spliced from a
	// frame this dialect really did build, so that byte is the only thing
	// that can be refusing it.
	clean, err := p5FixedDialect.BuildMWSet(p5TestRecord(t, p5FixedDialect, false))
	if err != nil {
		t.Fatalf("p5FixedDialect.BuildMWSet with TxClar false = %v — P5Fixed refuses the FLAG, not the record", err)
	}
	cleanFrame := clean.Bytes()
	if got := cleanFrame[memTxClarOffset]; got != '0' {
		t.Errorf("p5FixedDialect's own MW Set carries %q at position 21, want '0'", got)
	}
	if !p5FixedDialect.AllowedCommand(cleanFrame) {
		t.Errorf("p5FixedDialect's gate refused its own builder's %q", cleanFrame)
	}
	forged := append([]byte(nil), cleanFrame...)
	forged[memTxClarOffset] = '1'
	if p5FixedDialect.AllowedCommand(forged) {
		t.Errorf("p5FixedDialect's gate ADMITTED %q, whose P5 is '1' — a byte its manual prints \"(Fixed)\" must never reach the wire as anything else", forged)
	}
	// The SAME forged frame, offered to a P5TxClar dialect with a slot space
	// that admits it, must be ALLOWED: the refusal above is the policy's, not
	// some other check firing on the splice.
	wideForged := append([]byte(nil), wideFrame...)
	if !FT710.AllowedCommand(wideForged) {
		t.Errorf("FT710's gate refused %q, the same shape with P5 '1' — then the P5Fixed refusal above proves nothing about the policy", wideForged)
	}
}

// TestMemoryP5_FixedRequiresAZeroOnParse is the READ direction of the same
// table: under P5Fixed a '1' at position 21 is an undocumented frame and is
// refused, not decoded into a flag; under P5TxClar it decodes to TxClar
// true, exactly as it always did.
//
// The FT-891's own posture on the neighbouring P7 byte is deliberately the
// OPPOSITE — tolerant on read, because that radio prints no read-direction
// P7 vocabulary at all — and the difference is the evidence, not a
// preference: P5 IS printed, on four separate blocks, and what it is printed
// as is "(Fixed)".
func TestMemoryP5_FixedRequiresAZeroOnParse(t *testing.T) {
	clean, err := p5FixedDialect.BuildMWSet(p5TestRecord(t, p5FixedDialect, false))
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := append([]byte(nil), clean.Bytes()...)
	frame[0], frame[1] = 'M', 'R' // MR's answer and MW's Set share this block

	got, err := p5FixedDialect.ParseMRAnswer(frame)
	if err != nil {
		t.Fatalf("p5FixedDialect.ParseMRAnswer(%q) = %v — a P5 of '0' is the one value this dialect's manual prints", frame, err)
	}
	if got.TxClar {
		t.Error("p5FixedDialect.ParseMRAnswer returned TxClar true from a frame whose P5 is '0'")
	}

	frame[memTxClarOffset] = '1'
	if _, err := p5FixedDialect.ParseMRAnswer(frame); err == nil {
		t.Errorf("p5FixedDialect.ParseMRAnswer(%q) accepted a P5 of '1' — this dialect's manual prints the byte \"(Fixed)\", so a '1' is an undocumented frame and turning one into data is what this package refuses to do", frame)
	} else {
		for _, want := range []string{"P5", P5Fixed.String()} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal %q does not mention %q", err, want)
			}
		}
	}

	// POSITIVE CONTROL: the same byte, on a P5TxClar dialect, is the flag.
	wide, err := FT710.BuildMWSet(p5TestRecord(t, FT710, true))
	if err != nil {
		t.Fatalf("FT710.BuildMWSet: %v", err)
	}
	wf := append([]byte(nil), wide.Bytes()...)
	wf[0], wf[1] = 'M', 'R'
	back, err := FT710.ParseMRAnswer(wf)
	if err != nil {
		t.Fatalf("FT710.ParseMRAnswer(%q) = %v", wf, err)
	}
	if !back.TxClar {
		t.Error("FT710.ParseMRAnswer returned TxClar false from a frame whose P5 is '1' — the flag is lost on the read side")
	}
}

// TestMemoryP5_CombinedMTCarriesTheSamePolicy pins that the combined MT Set
// obeys the axis too. It shares the 28-position block byte for byte, so a
// policy honoured by MW alone would leave the FTdx10-family form free to put
// a '1' on the wire.
func TestMemoryP5_CombinedMTCarriesTheSamePolicy(t *testing.T) {
	slot, err := combinedDialect.MemorySlot(7)
	if err != nil {
		t.Fatal(err)
	}
	m := MemoryData{
		Slot: slot, FreqHz: 14_250_000, TxClar: true,
		Mode: ModeUSB, Kind: CombinedMTSetKind, CTCSS: CTCSSOff, Shift: ShiftSimplex,
	}
	// combinedDialect declares P5TxClar, so the flag is writable and reaches
	// position 21 of the combined record's own block.
	cmd, err := combinedDialect.BuildMTSetCombined(m, "AB")
	if err != nil {
		t.Fatalf("combinedDialect.BuildMTSetCombined with TxClar true = %v", err)
	}
	if got := cmd.Bytes()[memTxClarOffset]; got != '1' {
		t.Errorf("the combined MT Set carries %q at position 21, want '1' — the block is shared with MW byte for byte", got)
	}
	if !combinedDialect.AllowedCommand(cmd.Bytes()) {
		t.Errorf("combinedDialect's gate refused its own builder's %q", cmd.Bytes())
	}

	// And a P5Fixed COMBINED dialect must refuse the same record. Built here
	// rather than added to allTestDialects(), because what it proves is that
	// the P5 rule reaches validateCombinedMTFields, and one fixture is
	// enough for that.
	cfg := DialectConfig{
		CATID:     "1111",
		ModeNames: map[Mode]string{ModeUnset: "-", ModeUSB: "USB"},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99, PMSPairs: 9,
			NoneWire: "000", MCSelects: MCSelectsAll,
		},
		MT: MTPolicy{
			Form: MTFormCombined, ReadSlots: MTReadsReadable, P11: P11Fixed,
			TagMaxBytes: 6, TagFill: ' ',
		},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:    P5Fixed,
		MWWriteKind: KindMemory,
	}
	fixed, err := NewDialect(cfg)
	if err != nil {
		t.Fatalf("NewDialect: %v", err)
	}
	slot, err = fixed.MemorySlot(7)
	if err != nil {
		t.Fatal(err)
	}
	m.Slot = slot
	if cmd, err := fixed.BuildMTSetCombined(m, "AB"); err == nil {
		t.Errorf("a P5Fixed combined dialect built %q from a TxClar-true record — the axis must reach validateCombinedMTFields as well as validateMWFields", cmd.Bytes())
	}
	m.TxClar = false
	cmd, err = fixed.BuildMTSetCombined(m, "AB")
	if err != nil {
		t.Fatalf("a P5Fixed combined dialect refused a TxClar-FALSE record (%v) — P5Fixed refuses the flag, not the record", err)
	}
	if got := cmd.Bytes()[memTxClarOffset]; got != '0' {
		t.Errorf("a P5Fixed combined Set carries %q at position 21, want '0'", got)
	}
	if !fixed.AllowedCommand(cmd.Bytes()) {
		t.Errorf("its own gate refused %q", cmd.Bytes())
	}
}

// TestMemoryP5_RegisteredDialectsCarryTheTxClarifier pins the FT-710's
// declaration against its own MW/MR legend, and pins that the byte still
// moves for it — which is what makes "no existing golden moves" a property.
func TestMemoryP5_RegisteredDialectsCarryTheTxClarifier(t *testing.T) {
	if got := FT710.memoryP5; got != P5TxClar {
		t.Fatalf("FT710 declares MemoryP5 %v, want P5TxClar — its MW block prints P5 \"0: TX CLAR OFF 1: TX CLAR ON\"", got)
	}
	for _, tx := range []bool{false, true} {
		cmd, err := FT710.BuildMWSet(p5TestRecord(t, FT710, tx))
		if err != nil {
			t.Errorf("FT710.BuildMWSet with TxClar %v = %v", tx, err)
			continue
		}
		want := byte('0')
		if tx {
			want = '1'
		}
		if got := cmd.Bytes()[memTxClarOffset]; got != want {
			t.Errorf("FT710.BuildMWSet with TxClar %v put %q at position 21, want %q", tx, got, want)
		}
	}
}

// TestValidateDialectConfig_V14MemoryP5 is V14's clause table: the zero value
// is refused and names its field, and both policies are accepted.
func TestValidateDialectConfig_V14MemoryP5(t *testing.T) {
	tests := []struct {
		name    string
		policy  MemoryP5Policy
		wantErr string // "" means the config MUST be accepted
	}{
		{"zero refused", 0, "MemoryP5"},
		{"out-of-range refused", MemoryP5Policy(99), "MemoryP5"},
		{"P5TxClar accepted", P5TxClar, ""},
		{"P5Fixed accepted", P5Fixed, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaselineConfig()
			cfg.MemoryP5 = tc.policy
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

// TestMemoryP5Policy_String pins the names the refusals quote.
func TestMemoryP5Policy_String(t *testing.T) {
	for _, tc := range []struct {
		p    MemoryP5Policy
		want string
	}{
		{P5TxClar, "P5TxClar"},
		{P5Fixed, "P5Fixed"},
		{0, "MemoryP5Policy(0)"},
		{MemoryP5Policy(7), "MemoryP5Policy(7)"},
	} {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("MemoryP5Policy(%d).String() = %q, want %q", int(tc.p), got, tc.want)
		}
	}
}
