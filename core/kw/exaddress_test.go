// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"
)

// TestEXAddress_WireIsTheThreeDigitMenuNumber pins the field both books
// print: "EX" then a THREE-digit menu number (590:552, 480:410), whose
// declared domains are 000~087 on the TS-590S, 000~099 on the TS-590SG
// (590:543-544) and 000~060 on the TS-480 (480:401).
//
// Three digits for a domain that is numerically 0..99 is the document's
// choice, not this codec's, and rendering it any narrower would build a
// frame no chart describes.
func TestEXAddress_WireIsTheThreeDigitMenuNumber(t *testing.T) {
	tests := []struct {
		addr EXAddress
		want string
	}{
		{EXAddress{P1: 0}, "000"},
		{EXAddress{P1: 1}, "001"},
		{EXAddress{P1: 56}, "056"},
		{EXAddress{P1: 60}, "060"},
		{EXAddress{P1: 87}, "087"},
		{EXAddress{P1: 99}, "099"},
	}
	for _, tt := range tests {
		if got := tt.addr.Wire(); got != tt.want {
			t.Errorf("EXAddress{P1: %d}.Wire() = %q, want %q", tt.addr.P1, got, tt.want)
		}
		if len(tt.addr.Wire()) != 3 {
			t.Errorf("EXAddress{P1: %d}.Wire() is %d bytes, want 3", tt.addr.P1, len(tt.addr.Wire()))
		}
	}
}

// TestEXAddress_WireFailsClosedOnANonZeroP2OrP3 is the core/kw side of
// internal/extable's AddressSingle rule: P2 and P3 exist on this struct
// only because the renderer emits all three field names, and every Kenwood
// row's are zero. An address that carries a non-zero one is not a Kenwood
// address, and rendering P1 alone from it would silently drop information
// the caller believed it had supplied.
func TestEXAddress_WireFailsClosedOnANonZeroP2OrP3(t *testing.T) {
	for _, addr := range []EXAddress{{P1: 7, P2: 1}, {P1: 7, P3: 1}, {P1: 7, P2: 2, P3: 3}} {
		if got := addr.Wire(); got != "" {
			t.Errorf("%v.Wire() = %q, want \"\" — a non-zero P2 or P3 is not a Kenwood address", addr, got)
		}
	}
}

// TestEXAddress_StringIsNotAWireField follows core/cat's precedent: the
// debug rendering names ALL THREE components and carries bytes no address
// field may hold, so a debug print can never be read back as a wire field.
func TestEXAddress_StringIsNotAWireField(t *testing.T) {
	got := EXAddress{P1: 87}.String()
	for _, want := range []string{"P1=", "P2=", "P3="} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, want it to name %s", got, want)
		}
	}
	if !strings.ContainsAny(got, "= ") {
		t.Errorf("String() = %q, want a form that cannot be mistaken for a wire field", got)
	}
	if got == (EXAddress{P1: 87}).Wire() {
		t.Error("String() equals Wire() — the debug form must be unmistakable")
	}
}
