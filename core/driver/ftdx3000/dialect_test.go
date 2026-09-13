// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
)

// TestDialectConformance runs core/cat/dialecttest's shared conformance
// suite against this package's inline dialect — the brief's proof
// obligation for a Yaesu package with no core/cat/<model> subpackage of
// its own. This is the FIRST dialect exercising cat.P9ToneIndexReadOnly.
func TestDialectConformance(t *testing.T) { dialecttest.Run(t, dialect) }

func TestDialect_CATID(t *testing.T) {
	if got := dialect.CATID(); got != "0462" {
		t.Errorf("dialect.CATID() = %q, want \"0462\"", got)
	}
}

func TestDialect_Configured(t *testing.T) {
	if !dialect.Configured() {
		t.Error("dialect is unconfigured (zero value)")
	}
}

// TestDialect_ModeDomain pins matrix §1.2: twelve names '1'-'C', D/E/F
// refused.
func TestDialect_ModeDomain(t *testing.T) {
	want := []struct {
		wire byte
		name string
	}{
		{'1', "LSB"}, {'2', "USB"}, {'3', "CW"}, {'4', "FM"}, {'5', "AM"},
		{'6', "RTTY-LSB"}, {'7', "CW-R"}, {'8', "PKT-L"}, {'9', "RTTY-USB"},
		{'A', "PKT-FM"}, {'B', "FM-N"}, {'C', "PKT-U"},
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
	for _, absent := range []byte{'D', 'E', 'F'} {
		if _, err := dialect.ParseMode(absent); err == nil {
			t.Errorf("ParseMode(%q) succeeded, want refusal — matrix §1.2's MW/MR legend prints no D/E/F row (AM-N lives on the live MD command only)", absent)
		}
	}
}

// TestDialect_SlotSpace pins matrix §2.4: 001-099 memory, 100-117 PMS
// (numeric), no 60m, no EMG, no "000" none-form.
func TestDialect_SlotSpace(t *testing.T) {
	if _, err := dialect.MemorySlot(1); err != nil {
		t.Errorf("MemorySlot(1): %v", err)
	}
	if _, err := dialect.MemorySlot(99); err != nil {
		t.Errorf("MemorySlot(99): %v", err)
	}
	if _, err := dialect.MemorySlot(100); err == nil {
		t.Error("MemorySlot(100) succeeded, want refusal — matrix §2.4 caps memory at 099")
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
		t.Error("PMSSlot(10, false) succeeded, want refusal — matrix §2.4 declares nine pairs")
	}
	if _, err := dialect.SixtyMSlot(1); err == nil {
		t.Error("SixtyMSlot(1) succeeded, want refusal — no 60m bank")
	}
	if got := dialect.EMGSlot().Wire(); got != "" {
		t.Errorf("EMGSlot().Wire() = %q, want \"\" — no EMG channel", got)
	}
	if _, err := dialect.ParseSlot("000"); err == nil {
		t.Error(`ParseSlot("000") succeeded, want refusal — this dialect declares no none-form`)
	}
}

// TestDialect_MWWriteKind pins matrix §1.4's trap: the write-fixed byte
// '0' is cat.KindVFO, not cat.KindMemory.
func TestDialect_MWWriteKind(t *testing.T) {
	if got := dialect.MWWriteKind(); got != cat.KindVFO {
		t.Errorf("MWWriteKind() = %q, want cat.KindVFO ('0') — matrix §1.4", rune(got))
	}
}

// TestDialect_MemoryFrameShape pins matrix §1.1: 27 bytes, 8-digit P2, and
// the asymmetric P9 (live index on the READ side).
func TestDialect_MemoryFrameShape(t *testing.T) {
	m, err := dialect.ParseMRAnswer([]byte("MR00100030000+000000110260;"))
	if err != nil {
		t.Fatalf("ParseMRAnswer of a hand-built 27-byte frame: %v", err)
	}
	if m.FreqHz != 30000 {
		t.Errorf("FreqHz = %d, want 30000 (8-digit P2)", m.FreqHz)
	}
	if m.ToneIndex != 26 {
		t.Errorf("ToneIndex = %d, want 26 (live P9 on read)", m.ToneIndex)
	}
}

// TestDialect_P9WriteFixed pins matrix §1.3's headline wrinkle directly:
// MW's P9 is fixed and refuses a nonzero tone, even though MR's decodes
// one live.
func TestDialect_P9WriteFixed(t *testing.T) {
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
		t.Error("BuildMWSet(ToneIndex 5) succeeded, want refusal — MW's P9 is printed-fixed \"00\" (matrix §1.3)")
	}
}
