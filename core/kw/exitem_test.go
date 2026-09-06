// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "testing"

// TestEXItem_CarriesTheFieldNamesTheRendererEmits is the contract with
// internal/extable, asserted by the COMPILER through a composite literal
// written exactly as RenderGo writes one (extable.go's Fprintf for the item
// line). If a field is renamed here, every generated Kenwood inventory
// stops compiling — which is the point: the renderer hard-codes these names
// and cannot be told otherwise.
func TestEXItem_CarriesTheFieldNamesTheRendererEmits(t *testing.T) {
	item := EXItem{Addr: EXAddress{P1: 0, P2: 0, P3: 0}, P1Label: "", P2Label: "", Name: "Display brightness", Digits: 1, Text: false, ObservedReadWidth: 0, ObservedReadShape: ""}
	if item.Name != "Display brightness" || item.Digits != 1 {
		t.Errorf("EXItem literal did not round-trip: %+v", item)
	}
	if item.Addr.Wire() != "000" {
		t.Errorf("item.Addr.Wire() = %q, want \"000\"", item.Addr.Wire())
	}
}

// TestCopyEXItems_ReturnsAnIndependentSlice pins the defensive copy the
// three package accessors are built on.
//
// IT IS PINNED HERE, WITH REAL DATA, AND NOT IN THE PACKAGES THAT USE IT.
// core/kw/ts590 and core/kw/ts480 hold BOOTSTRAP inventories that are empty
// until lane P generates them, so a copy test written there would assert
// nothing at all today and would look like it did. Pinning the helper once,
// where a non-empty slice can be constructed, is what makes those
// accessors' behaviour actually covered.
func TestCopyEXItems_ReturnsAnIndependentSlice(t *testing.T) {
	src := []EXItem{
		{Addr: EXAddress{P1: 0}, Name: "one", Digits: 1},
		{Addr: EXAddress{P1: 1}, Name: "two", Digits: 2},
	}
	got := CopyEXItems(src)
	if len(got) != len(src) {
		t.Fatalf("CopyEXItems returned %d items, want %d", len(got), len(src))
	}
	got[0].Name = "mutated"
	if src[0].Name != "one" {
		t.Errorf("mutating the copy changed the source: %q", src[0].Name)
	}
	second := CopyEXItems(src)
	if second[0].Name != "one" {
		t.Errorf("a later call saw an earlier caller's mutation: %q", second[0].Name)
	}
}

// TestCopyEXItems_EmptyAndNil pins the bootstrap case explicitly, because
// it is the one every Kenwood accessor is in until lane P lands: an empty
// inventory copies to an empty inventory, and never to nil-versus-empty
// confusion at a call site.
func TestCopyEXItems_EmptyAndNil(t *testing.T) {
	if got := CopyEXItems([]EXItem{}); len(got) != 0 {
		t.Errorf("CopyEXItems(empty) returned %d items, want 0", len(got))
	}
	if got := CopyEXItems(nil); len(got) != 0 {
		t.Errorf("CopyEXItems(nil) returned %d items, want 0", len(got))
	}
}
