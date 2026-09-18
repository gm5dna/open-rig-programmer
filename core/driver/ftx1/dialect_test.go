// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
)

// TestDialectConformance runs core/cat/dialecttest's shared conformance
// suite against this package's inline dialect.
func TestDialectConformance(t *testing.T) { dialecttest.Run(t, dialect) }

func TestDialect_CATID(t *testing.T) {
	if got := dialect.CATID(); got != "0840" {
		t.Errorf("dialect.CATID() = %q, want \"0840\"", got)
	}
}

func TestDialect_Configured(t *testing.T) {
	if !dialect.Configured() {
		t.Error("dialect is unconfigured (zero value)")
	}
}

// TestDialect_ModeDomain pins spec.md §5: the FT-710's own sixteen
// names plus 'H'/'I', and 'G'/'J' refused (not named).
func TestDialect_ModeDomain(t *testing.T) {
	want := []struct {
		wire byte
		name string
	}{
		{'1', "LSB"}, {'2', "USB"}, {'3', "CW-U"}, {'4', "FM"}, {'5', "AM"},
		{'6', "RTTY-L"}, {'7', "CW-L"}, {'8', "DATA-L"}, {'9', "RTTY-U"},
		{'A', "DATA-FM"}, {'B', "FM-N"}, {'C', "DATA-U"}, {'D', "AM-N"},
		{'E', "PSK"}, {'F', "DATA-FM-N"},
		{'H', "C4FM-DN"}, {'I', "C4FM-VW"},
	}
	for _, w := range want {
		m, err := dialect.ParseMode(w.wire)
		if err != nil {
			t.Errorf("ParseMode(%q): %v", w.wire, err)
			continue
		}
		if got := dialect.ModeName(m); got != w.name {
			t.Errorf("ModeName(%q) = %q, want %q", w.wire, got, w.name)
		}
	}
	for _, absent := range []byte{'G', 'J'} {
		if _, err := dialect.ParseMode(absent); err == nil {
			t.Errorf("ParseMode(%q) succeeded, want refusal — the manual prints \"-\" for both, ASSUMED reserved (spec.md §5)", absent)
		}
	}
}

func TestDialect_SlotSpace(t *testing.T) {
	lo, err := dialect.MemorySlot(1)
	if err != nil || lo.Wire() != "00001" {
		t.Errorf("MemorySlot(1) = %v, %v, want \"00001\"", lo, err)
	}
	hi, err := dialect.MemorySlot(999)
	if err != nil || hi.Wire() != "00999" {
		t.Errorf("MemorySlot(999) = %v, %v, want \"00999\"", hi, err)
	}
	if _, err := dialect.MemorySlot(1000); err == nil {
		t.Error("MemorySlot(1000) succeeded, want refusal")
	}

	pLower, err := dialect.PMSSlot(1, false)
	if err != nil || pLower.Wire() != "P-01L" {
		t.Errorf("PMSSlot(1, false) = %v, %v, want \"P-01L\"", pLower, err)
	}
	pUpper, err := dialect.PMSSlot(50, true)
	if err != nil || pUpper.Wire() != "P-50U" {
		t.Errorf("PMSSlot(50, true) = %v, %v, want \"P-50U\"", pUpper, err)
	}
	if _, err := dialect.PMSSlot(51, false); err == nil {
		t.Error("PMSSlot(51, false) succeeded, want refusal — 50 pairs only")
	}

	sLo, err := dialect.SixtyMSlot(1)
	if err != nil || sLo.Wire() != "50001" {
		t.Errorf("SixtyMSlot(1) = %v, %v, want \"50001\"", sLo, err)
	}
	sHi, err := dialect.SixtyMSlot(20)
	if err != nil || sHi.Wire() != "50020" {
		t.Errorf("SixtyMSlot(20) = %v, %v, want \"50020\"", sHi, err)
	}
	if _, err := dialect.SixtyMSlot(21); err == nil {
		t.Error("SixtyMSlot(21) succeeded, want refusal — 20 channels only")
	}

	if got := dialect.EMGSlot().Wire(); got != "EMGCH" {
		t.Errorf("EMGSlot().Wire() = %q, want \"EMGCH\"", got)
	}

	// "00000" is a parse-ACCEPT-only placeholder ("VFO or MT or QMB",
	// spec.md §3.1/§4): ParseSlot must accept it and classify it IsNone,
	// but no builder may ever emit it (readableSlot/writableSlot both
	// exclude the none form).
	n, err := dialect.ParseSlot("00000")
	if err != nil || !n.IsNone() {
		t.Errorf("ParseSlot(%q) = %v, %v, want a none-form Slot with no error", "00000", n, err)
	}
}

func TestDialect_MWWriteKind(t *testing.T) {
	if got := dialect.MWWriteKind(); got != cat.KindMemory {
		t.Errorf("MWWriteKind() = %q, want cat.KindMemory ('1') — ASSUMED, doc.go register item 6", rune(got))
	}
}

func TestDialect_MCUnsupported(t *testing.T) {
	if dialect.MCSupported() {
		t.Error("MCSupported() = true, want false — dialect.go declares MCSelectsUnsupported (spec.md §3.4)")
	}
	if _, err := dialect.BuildMCSet(mustMemorySlot(t, 1)); err == nil {
		t.Error("BuildMCSet succeeded on an MCSelectsUnsupported dialect, want refusal")
	}
}

func TestDialect_MTForm(t *testing.T) {
	if got := dialect.MTForm(); got != cat.MTFormShortNoDisplay {
		t.Errorf("MTForm() = %v, want MTFormShortNoDisplay", got)
	}
	lo, hi, err := dialect.MTAnswerBounds()
	if err != nil {
		t.Fatalf("MTAnswerBounds(): %v", err)
	}
	if lo != 20 || hi != 20 {
		t.Errorf("MTAnswerBounds() = (%d, %d), want (20, 20) — 2 + slot(5) + tag(12) + ';'", lo, hi)
	}
}

// TestDialect_ToneStatesSix pins spec.md §6's six-value P8 domain,
// including the two unnamed bytes '4'/'5'.
func TestDialect_ToneStatesSix(t *testing.T) {
	for _, c := range []byte{'0', '1', '2', '3', '4', '5'} {
		if _, err := dialect.ParseCTCSSState(c); err != nil {
			t.Errorf("ParseCTCSSState(%q): %v, want accepted", c, err)
		}
	}
	if _, err := dialect.ParseCTCSSState('6'); err == nil {
		t.Error("ParseCTCSSState('6') succeeded, want refusal — six-value domain, '0'-'5' only")
	}
}

func TestDialect_MemoryFrameShape(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	cmd, err := dialect.BuildMWSet(cat.MemoryData{
		Slot: slot, FreqHz: 14250000, Mode: cat.ModeUSB,
		Kind: dialect.MWWriteKind(), CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	if got := len(cmd.Bytes()); got != 30 {
		t.Errorf("MW frame length = %d, want 30 (spec.md §3.1)", got)
	}
	if got := string(cmd.Bytes()[0:7]); got != "MW00001" {
		t.Errorf("MW frame prefix = %q, want \"MW00001\"", got)
	}
}
