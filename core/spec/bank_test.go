// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// TestBankIDConstants pins the four canonical BankID wire-form strings.
func TestBankIDConstants(t *testing.T) {
	cases := []struct {
		id   BankID
		want string
	}{
		{BankMemory, "MEM"},
		{BankPMS, "PMS"},
		{Bank60m, "60M"},
		{BankEMG, "EMG"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.id) != tc.want {
				t.Errorf("BankID = %q, want %q", string(tc.id), tc.want)
			}
		})
	}
}

// TestBankConstruction is a smoke test that Bank's fields hold what they
// are assigned, including the NoBlank flag (PMS pairs + M-01 must stay
// populated) and a Fields map keyed by Field.
func TestBankConstruction(t *testing.T) {
	b := Bank{
		ID:      BankPMS,
		Label:   "Scan limits (PMS)",
		Slots:   []string{"P01L", "P01U"},
		NoBlank: true,
		Fields: map[Field]FieldSupport{
			FieldFrequency: {Read: Supported, Write: Unverified},
		},
	}
	if b.ID != BankPMS {
		t.Errorf("ID = %v, want %v", b.ID, BankPMS)
	}
	if b.Label != "Scan limits (PMS)" {
		t.Errorf("Label = %q, want %q", b.Label, "Scan limits (PMS)")
	}
	if len(b.Slots) != 2 || b.Slots[0] != "P01L" || b.Slots[1] != "P01U" {
		t.Errorf("Slots = %v, want [P01L P01U]", b.Slots)
	}
	if !b.NoBlank {
		t.Error("NoBlank = false, want true")
	}
	fs, ok := b.Fields[FieldFrequency]
	if !ok {
		t.Fatal("Fields[FieldFrequency] missing")
	}
	if fs.Read != Supported || fs.Write != Unverified {
		t.Errorf("Fields[FieldFrequency] = %+v, want {Read:Supported Write:Unverified}", fs)
	}
}
