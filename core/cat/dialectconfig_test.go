// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// TestNewDialect_AcceptsAValidConfig is the plain positive case. It exists
// so the rejection tests around it cannot all pass by the constructor
// simply refusing everything.
func TestNewDialect_AcceptsAValidConfig(t *testing.T) {
	d, err := NewDialect(validBaselineConfig())
	if err != nil {
		t.Fatalf("NewDialect(valid) = %v, want a dialect", err)
	}
	if !d.Configured() {
		t.Error("NewDialect returned a dialect that reports Configured() == false")
	}
	if d.CATID() != "1234" {
		t.Errorf("CATID() = %q, want %q", d.CATID(), "1234")
	}
}

// TestNewDialect_RejectionReturnsTheZeroDialect pins that a refused config
// yields an INERT dialect, not a partly-built one.
//
// This matters more than it looks: Dialect.AllowedCommand is a method
// value, so a caller who ignores the error still holds something that
// satisfies transport's AllowFunc and would be installed as a real engine's
// gate. The zero value accepts nothing, so ignoring the error fails closed.
func TestNewDialect_RejectionReturnsTheZeroDialect(t *testing.T) {
	cfg := validBaselineConfig()
	cfg.CATID = "nope-too-long"

	d, err := NewDialect(cfg)
	if err == nil {
		t.Fatal("NewDialect(invalid) returned no error")
	}
	if d.Configured() {
		t.Error("a refused NewDialect returned a Configured() dialect — it must be inert")
	}
	// The property that actually protects a radio: its gate admits nothing.
	for _, frame := range [][]byte{
		[]byte("ID;"),
		[]byte("MR001;"),
		[]byte("AI0;"),
	} {
		if d.AllowedCommand(frame) {
			t.Errorf("the dialect returned alongside an error admitted %q at its gate", frame)
		}
	}
}

// TestMustNewDialect_ValidConfigReturnsTheSameDialect covers the success
// half. Both halves are needed: a MustNewDialect that panicked on
// everything would pass a panic-only test.
func TestMustNewDialect_ValidConfigReturnsTheSameDialect(t *testing.T) {
	cfg := validBaselineConfig()

	want, err := NewDialect(cfg)
	if err != nil {
		t.Fatalf("NewDialect(valid) = %v", err)
	}
	got := MustNewDialect(cfg)

	if got.CATID() != want.CATID() {
		t.Errorf("MustNewDialect CATID = %q, NewDialect CATID = %q", got.CATID(), want.CATID())
	}
	if !got.Configured() {
		t.Error("MustNewDialect returned an unconfigured dialect")
	}
}

// TestMustNewDialect_InvalidConfigPanics covers the failure half, and
// asserts the panic CARRIES the validation error rather than being a bare
// runtime fault. A panic that says nothing about which field was wrong
// makes a build-time table error harder to fix than the error return it
// replaced.
func TestMustNewDialect_InvalidConfigPanics(t *testing.T) {
	cfg := validBaselineConfig()
	cfg.Slots.PMSPairs = 10 // rejected, not clamped

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustNewDialect(invalid) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want a string carrying the validation error", r)
		}
		if !strings.Contains(msg, "PMSPairs") {
			t.Errorf("panic message %q does not name the offending field", msg)
		}
	}()

	MustNewDialect(cfg)
}

// TestNewDialect_CopiesItsInput proves the constructor's central safety
// promise: a caller that mutates the config it passed cannot afterwards
// change the dialect.
//
// A Dialect is consulted by the outbound gate on every write. One whose
// data can still be edited by whoever built it is not a gate — and because
// Go maps and slices are reference types, storing them directly would make
// that the DEFAULT behaviour rather than a bug someone had to introduce.
//
// Task 63 extends this across every derived structure; this is the
// constructor-level check that the copy happens at all.
func TestNewDialect_CopiesItsInput(t *testing.T) {
	cfg := validBaselineConfig()
	d, err := NewDialect(cfg)
	if err != nil {
		t.Fatalf("NewDialect: %v", err)
	}

	beforeName := d.ModeName(Mode('2'))
	beforeItems := d.EXItems()

	// Mutate the caller's own containers after construction.
	cfg.ModeNames[Mode('2')] = "MUTATED"
	cfg.EXItems[0].Name = "MUTATED"
	cfg.EXItems[0].Digits = 99

	if got := d.ModeName(Mode('2')); got != beforeName {
		t.Errorf("mutating the caller's ModeNames changed the dialect: ModeName = %q, want %q", got, beforeName)
	}
	if got := d.EXItems(); got[0].Name != beforeItems[0].Name {
		t.Errorf("mutating the caller's EXItems changed the dialect: Name = %q, want %q", got[0].Name, beforeItems[0].Name)
	}
	// The DERIVED width must not move either — it is computed from the
	// copy, so a constructor that copied the slice but derived from the
	// original would still fail here.
	if got := d.exP4MaxBytes(); got != 5 {
		t.Errorf("exP4MaxBytes() = %d after caller mutation, want 5 (the widest Digits in the copy)", got)
	}
}

// TestNewDialect_DerivesEveryIndexFromTheCopy pins that all four derived
// structures exist and describe the config, not the FT-710.
func TestNewDialect_DerivesEveryIndexFromTheCopy(t *testing.T) {
	d, err := NewDialect(validBaselineConfig())
	if err != nil {
		t.Fatalf("NewDialect: %v", err)
	}

	// exMembers: this dialect's own addresses, and none of the FT-710's.
	own := EXAddress{P1: 7, P2: 1, P3: 1}
	if !d.KnownEXAddress(own) {
		t.Errorf("KnownEXAddress(%v) = false for the dialect's own item", own)
	}
	for _, a := range FT710.EXAddresses()[:5] {
		if d.KnownEXAddress(a) {
			t.Errorf("KnownEXAddress(%v) = true — that is an FT-710 address, not this dialect's", a)
		}
	}

	// exByTriple, observable only through NewEXAddress.
	if _, err := d.NewEXAddress(7, 1, 2); err != nil {
		t.Errorf("NewEXAddress(7,1,2) = %v, want the dialect's own second item", err)
	}
	if _, err := d.NewEXAddress(1, 1, 1); err == nil {
		t.Error("NewEXAddress(1,1,1) succeeded — that triple is the FT-710's, not this dialect's")
	}

	// exP4Max: the widest Digits in THIS inventory (5), not the FT-710's 12.
	if got := d.exP4MaxBytes(); got != 5 {
		t.Errorf("exP4MaxBytes() = %d, want 5", got)
	}

	// modeByName.
	if m, ok := d.ModeByName("ALPHA"); !ok || m != Mode('2') {
		t.Errorf("ModeByName(\"ALPHA\") = (%#02x, %v), want (%#02x, true)", byte(m), ok, byte('2'))
	}
	if _, ok := d.ModeByName("LSB"); ok {
		t.Error("ModeByName(\"LSB\") found an FT-710 mode name in this dialect")
	}
}
