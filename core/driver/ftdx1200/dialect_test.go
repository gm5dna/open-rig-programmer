// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
)

// TestDialectConformance runs core/cat/dialecttest's shared conformance
// suite against this package's inline dialect.
func TestDialectConformance(t *testing.T) { dialecttest.Run(t, dialect) }

func TestDialect_CATID(t *testing.T) {
	if got := dialect.CATID(); got != "0582" {
		t.Errorf("dialect.CATID() = %q, want \"0582\" (the FFT-1-fitted variant, canonical for this dialect)", got)
	}
}

func TestDialect_Configured(t *testing.T) {
	if !dialect.Configured() {
		t.Error("dialect is unconfigured (zero value)")
	}
}

// TestDialect_ModeDomain pins matrix §1.2: eleven names, hole at 'A', no
// 'D' anywhere.
func TestDialect_ModeDomain(t *testing.T) {
	want := []struct {
		wire byte
		name string
	}{
		{'1', "LSB"}, {'2', "USB"}, {'3', "CW"}, {'4', "FM"}, {'5', "AM"},
		{'6', "RTTY-LSB"}, {'7', "CW-R"}, {'8', "DATA-LSB"}, {'9', "RTTY-USB"},
		{'B', "FM-N"}, {'C', "DATA-USB"},
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
	for _, absent := range []byte{'A', 'D', 'E', 'F'} {
		if _, err := dialect.ParseMode(absent); err == nil {
			t.Errorf("ParseMode(%q) succeeded, want refusal — matrix §1.2: 'A' is a genuine hole, 'D'/'E'/'F' are printed nowhere", absent)
		}
	}
}

func TestDialect_SlotSpace(t *testing.T) {
	if _, err := dialect.MemorySlot(1); err != nil {
		t.Errorf("MemorySlot(1): %v", err)
	}
	if _, err := dialect.MemorySlot(99); err != nil {
		t.Errorf("MemorySlot(99): %v", err)
	}
	if _, err := dialect.MemorySlot(100); err == nil {
		t.Error("MemorySlot(100) succeeded, want refusal")
	}
	lower, err := dialect.PMSSlot(1, false)
	if err != nil || lower.Wire() != "100" {
		t.Errorf("PMSSlot(1, false) = %v, %v, want \"100\"", lower, err)
	}
	upper, err := dialect.PMSSlot(9, true)
	if err != nil || upper.Wire() != "117" {
		t.Errorf("PMSSlot(9, true) = %v, %v, want \"117\"", upper, err)
	}
	if _, err := dialect.PMSSlot(10, false); err == nil {
		t.Error("PMSSlot(10, false) succeeded, want refusal")
	}
	if _, err := dialect.SixtyMSlot(1); err == nil {
		t.Error("SixtyMSlot(1) succeeded, want refusal — no 60m bank")
	}
	if got := dialect.EMGSlot().Wire(); got != "" {
		t.Errorf("EMGSlot().Wire() = %q, want \"\"", got)
	}
	if _, err := dialect.ParseSlot("000"); err == nil {
		t.Error(`ParseSlot("000") succeeded, want refusal`)
	}
}

func TestDialect_MWWriteKind(t *testing.T) {
	if got := dialect.MWWriteKind(); got != cat.KindVFO {
		t.Errorf("MWWriteKind() = %q, want cat.KindVFO ('0')", rune(got))
	}
}

// TestDialect_P9AlwaysFixed pins matrix §1.3: P9 is fixed on BOTH read
// and write — a nonzero tone is refused on write, and a live index is
// refused on READ too (unlike the sibling ftdx3000).
func TestDialect_P9AlwaysFixed(t *testing.T) {
	slot, err := dialect.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	base := cat.MemoryData{
		Slot: slot, FreqHz: 14250000, Mode: cat.Mode('2'),
		Kind: dialect.MWWriteKind(), CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	}

	clean, err := dialect.BuildMWSet(base)
	if err != nil {
		t.Fatalf("BuildMWSet(ToneIndex 0) = %v, want accepted", err)
	}
	if got := string(clean.Bytes()[21:23]); got != "00" {
		t.Errorf("MW P9 = %q, want \"00\"", got)
	}

	nonzero := base
	nonzero.ToneIndex = 5
	if _, err := dialect.BuildMWSet(nonzero); err == nil {
		t.Error("BuildMWSet(ToneIndex 5) succeeded, want refusal — P9 is printed-fixed \"00\" on write")
	}

	// READ: a live index in the wire frame is REFUSED, not decoded — this
	// is the fact that distinguishes P9Fixed00 from the sibling
	// ftdx3000's P9ToneIndexReadOnly.
	frame := append([]byte(nil), clean.Bytes()...)
	frame[0], frame[1] = 'M', 'R'
	frame[23], frame[24] = '2', '6'
	if _, err := dialect.ParseMRAnswer(frame); err == nil {
		t.Error("ParseMRAnswer accepted a live tone index at P9, want refusal — this radio's P9 is fixed on READ too (matrix §1.3), unlike ftdx3000's")
	}
}
