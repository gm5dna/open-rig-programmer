// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// TestFT710Dialect_CarriesTheRadioSpecificData pins what the dialect is
// FOR: the things that vary across the classic NEWCAT family.
func TestFT710Dialect_CarriesTheRadioSpecificData(t *testing.T) {
	if !FT710.Configured() {
		t.Fatal("FT710.Configured() = false — the package's own dialect must be configured")
	}
	if got := FT710.CATID(); got != "0800" {
		t.Errorf("FT710.CATID() = %q, want %q", got, "0800")
	}
	if !FT710.ValidMode(ModeUSB) {
		t.Error("FT710.ValidMode(ModeUSB) = false, want true")
	}
	if got := FT710.ModeName(ModeDATAFMN); got != "DATA-FM-N" {
		t.Errorf("FT710.ModeName(ModeDATAFMN) = %q, want %q", got, "DATA-FM-N")
	}
	if n := len(FT710.EXItems()); n != 296 {
		t.Errorf("len(FT710.EXItems()) = %d, want 296", n)
	}
}

// TestZeroDialect_IsUnconfiguredAndKnowsNothing is the fail-closed
// property at the codec layer.
//
// Codex plan-review F6: an exported struct always has a constructible
// zero value, unexported fields or not. `var d cat.Dialect` compiles, and
// d.AllowedCommand is a non-nil method value that would satisfy
// transport.NewEngine's nil check. So the zero dialect must be INERT by
// construction — no slot space, no modes, no inventory — and therefore
// able neither to build nor to accept anything.
func TestZeroDialect_IsUnconfiguredAndKnowsNothing(t *testing.T) {
	var d Dialect

	if d.Configured() {
		t.Error("zero Dialect reports Configured() = true")
	}
	if d.CATID() != "" {
		t.Errorf("zero Dialect CATID() = %q, want empty", d.CATID())
	}
	if d.ValidMode(ModeUSB) {
		t.Error("zero Dialect claims to know ModeUSB")
	}
	if len(d.EXItems()) != 0 {
		t.Error("zero Dialect returned EX items")
	}
	for _, w := range []string{"001", "P1L", "501", "EMG", "000"} {
		if _, err := d.ParseSlot(w); err == nil {
			t.Errorf("zero Dialect parsed slot %q — it has no slot space", w)
		}
	}
}

// TestFT710Dialect_SlotSpaceIsDialectData pins slot classification as
// something the dialect OWNS. The FTX-1's 5-digit slots are the eventual
// forcing case; the classic family is 3-digit.
func TestFT710Dialect_SlotSpaceIsDialectData(t *testing.T) {
	cases := []struct {
		wire string
		want bool
	}{
		{"001", true},
		{"099", true},
		{"P1L", true},
		{"P9U", true},
		{"501", true},
		{"EMG", true},
		{"000", true},
		{"100", false},
		{"00001", false},
		{"abc", false},
	}
	for _, tc := range cases {
		_, err := FT710.ParseSlot(tc.wire)
		if (err == nil) != tc.want {
			t.Errorf("FT710.ParseSlot(%q): err == nil is %v, want %v", tc.wire, err == nil, tc.want)
		}
	}
}

// TestFT710Dialect_EXMembershipIsPerDialect pins that membership consults
// the dialect's own index rather than a package global. Task 57's second
// dialect proves it beyond doubt; this is the cheap first check.
func TestFT710Dialect_EXMembershipIsPerDialect(t *testing.T) {
	addrs := FT710.EXAddresses()
	if len(addrs) == 0 {
		t.Fatal("FT710.EXAddresses() is empty")
	}
	if !FT710.KnownEXAddress(addrs[0]) {
		t.Errorf("FT710.KnownEXAddress(%s) = false for its own first address", FT710.EXWire(addrs[0]))
	}

	var zero Dialect
	if zero.KnownEXAddress(addrs[0]) {
		t.Errorf("zero Dialect claims to know EX address %s — membership is reading a package global, not the receiver", FT710.EXWire(addrs[0]))
	}
}

// TestFT710Dialect_EXItemsReturnsFreshCopies mirrors the existing
// TestEXItems_ReturnsFreshCopies guarantee.
func TestFT710Dialect_EXItemsReturnsFreshCopies(t *testing.T) {
	first := FT710.EXItems()
	if len(first) == 0 {
		t.Fatal("FT710.EXItems() returned nothing")
	}
	original := first[0]
	first[0].Name = "MUTATED"

	second := FT710.EXItems()
	if second[0].Name == "MUTATED" {
		t.Error("FT710.EXItems() shares backing storage between calls")
	}
	if second[0] != original {
		t.Errorf("FT710.EXItems()[0] = %+v, want %+v", second[0], original)
	}
}
