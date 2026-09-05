// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// Compile-time proof of the driver-level settings seam. Any signature drift
// fails the BUILD, not merely a test — and this seam's ABSENCE would fail
// silently: internal/wiring's StaticSettingsDescriptor helper asks for the
// interface by a two-result type assertion and reports "no settings surface"
// when a driver does not satisfy it, with no error anywhere.
//
// It lives HERE rather than in this package's optional_test.go, where the
// package's other seam assertions sit, because this task's whole surface is
// one subject in one pair of files — the same choice
// StaticSettingsDescriptor's own doc comment makes for the method.
var _ driver.StaticSettingsProvider = (*ft891Driver)(nil)

// registeredRowCount returns the row count the REGISTERED internal/extable
// profile for package ft891 declares.
//
// It is the staleness bound this file's shape assertions are measured
// against, and it is taken from the registry rather than written here as
// 159 on purpose: the count's datum is the dialect's generated inventory
// (core/cat/ft891/exinventory_gen.go, from table2.csv) while its bound comes
// from the group-boundary ledger a quarantined agent derived from the
// rendered PDF before either transcription existed
// (core/cat/ft891/testdata/group-ledger.md, "Total rows: 159") — the
// bound-consulted-from-one-place, datum-taken-from-another rule, which a
// literal in this file would break.
//
// The profile is selected by Package rather than by lookup name, the same
// selection core/cat/ft891's TestEXItemsCountMatchesProfile
// (dialect_test.go) makes and for the reason its own comment gives.
func registeredRowCount(t *testing.T) int {
	t.Helper()
	var matches []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == "ft891" {
			matches = append(matches, np)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("internal/extable holds %d registered profiles emitting into package ft891, want exactly 1", len(matches))
	}
	return matches[0].Profile.ExpectedRows
}

// TestSettingsDescriptor_ShapeFromTheInventory pins the descriptor against
// the inventory it is derived from, structurally and by an INDEPENDENT
// derivation.
//
// buildSettingsDescriptor walks the (already sorted) inventory once, opening
// a menu when the running two-digit prefix changes from the previous row.
// This test rebuilds the expected partition a different way — map-keyed
// accumulation, then an explicit sort of the keys — so agreement is evidence
// about the DATA rather than the same algorithm compared with itself. A
// non-contiguous run of one prefix in the inventory would split into two
// menus under the linear pass and would show up here as an extra menu with a
// duplicate ID (and would additionally fail Validate).
//
// The counts, the partition and the eighteen-menu claim are matrix §3.9's:
// 159 items in 18 menus and 18 groups, one group per menu, the four-digit
// MENU Number's first two digits taking every value 01 through 18 with none
// skipped.
func TestSettingsDescriptor_ShapeFromTheInventory(t *testing.T) {
	d := SettingsDescriptor()

	if err := d.Validate(); err != nil {
		t.Fatalf("SettingsDescriptor().Validate() = %v, want nil", err)
	}
	if d.Version != "ft891-ex@1" {
		t.Errorf("Version = %q, want \"ft891-ex@1\" (minted in this package; codeplug.MenuSnapshot.Descriptor carries it verbatim)", d.Version)
	}

	items := catDialect.EXItems()

	// The count is the DIALECT's; the registry's ExpectedRows is the
	// staleness check on this test's own assumption, not the assertion
	// itself. See registeredRowCount.
	wantItems := len(items)
	if want := registeredRowCount(t); wantItems != want {
		t.Fatalf("catDialect.EXItems() = %d items, the registered ft891 profile's ExpectedRows is %d — reconcile core/cat/ft891 with the ledger before touching this file", wantItems, want)
	}

	// Independent derivation of the expected partition.
	menuItems := map[string][]driver.SettingItem{}
	var menuIDs []string
	for _, it := range items {
		menuID := fmt.Sprintf("%02d", it.Addr.P1)
		if _, seen := menuItems[menuID]; !seen {
			menuIDs = append(menuIDs, menuID)
		}
		menuItems[menuID] = append(menuItems[menuID], driver.SettingItem{
			ID:      catDialect.EXWire(it.Addr),
			Label:   it.Name,
			Display: catDialect.EXWire(it.Addr),
		})
	}
	sort.Strings(menuIDs)

	if len(menuIDs) != 18 {
		t.Errorf("the inventory has %d distinct two-digit prefixes, want 18 (matrix §3.9: 01 through 18, none skipped)", len(menuIDs))
	}
	for i, id := range menuIDs {
		if want := fmt.Sprintf("%02d", i+1); id != want {
			t.Errorf("menu prefix %d in ascending order is %q, want %q — matrix §3.9 records the prefixes as contiguous 01..18", i, id, want)
		}
	}

	if len(d.Menus) != len(menuIDs) {
		t.Fatalf("descriptor has %d menus, the inventory has %d distinct prefixes", len(d.Menus), len(menuIDs))
	}
	var gotItems, gotGroups int
	for i, m := range d.Menus {
		if m.ID != menuIDs[i] {
			t.Errorf("Menus[%d].ID = %q, want %q (menus in ascending prefix order)", i, m.ID, menuIDs[i])
			continue
		}
		// ONE group per menu, carrying the menu's own ID: this chart has no
		// group column to partition by (matrix §3.9), so the second level
		// exists because driver.SettingsDescriptor is a two-level tree and
		// Validate requires every menu to hold at least one group — not
		// because the radio has a subgroup here.
		if len(m.Groups) != 1 {
			t.Errorf("menu %q has %d groups, want exactly 1 — this radio's chart prints no group column (matrix §3.9)", m.ID, len(m.Groups))
			continue
		}
		gotGroups++
		g := m.Groups[0]
		if g.ID != m.ID {
			t.Errorf("menu %q's single group has ID %q, want the menu's own %q", m.ID, g.ID, m.ID)
		}
		if !reflect.DeepEqual(g.Items, menuItems[m.ID]) {
			t.Errorf("menu %q Items =\n %#v\nwant (derived independently from the inventory)\n %#v", m.ID, g.Items, menuItems[m.ID])
		}
		gotItems += len(g.Items)
	}

	if gotItems != wantItems {
		t.Errorf("total descriptor items = %d, want %d (== len(catDialect.EXItems()))", gotItems, wantItems)
	}
	if gotGroups != len(menuIDs) {
		t.Errorf("total descriptor groups = %d, want %d (one per menu)", gotGroups, len(menuIDs))
	}
}

// TestSettingsDescriptor_MenuLabelsAreTheStructuralFallback pins the MANUAL
// FACT that forces the descriptor's shape, and the fallback it forces.
//
// The FT-891's menu chart has NO GROUP LABEL COLUMNS — its columns are
// `P1 | Function | P2 | Digits` (layout 524) where the three registered
// siblings' charts carry them — so the registered extable profile declares
// LabelsAbsent and every generated EXItem's P1Label and P2Label is ""
// (core/cat/ft891/exinventory_gen.go's own header says so in terms; matrix
// §3.9). driver.SettingsDescriptor.Validate refuses an empty Label at every
// level, so a name has to come from somewhere, and the project's standing
// rule is that it must not be invented (spec decision 5, "No invented group
// labels"). The two-digit prefix is therefore the label as well as the ID.
//
// THE EMPTY-LABEL HALF IS THE LOAD-BEARING ONE. If a later transcription
// ever gave this inventory real labels, the numeric fallback would be
// throwing a manual name away — so this test fails then, at the fallback,
// rather than leaving the descriptor quietly worse than its source.
func TestSettingsDescriptor_MenuLabelsAreTheStructuralFallback(t *testing.T) {
	for _, it := range catDialect.EXItems() {
		if it.P1Label != "" || it.P2Label != "" {
			t.Fatalf("inventory row %s carries P1Label %q / P2Label %q — this chart prints no label columns (matrix §3.9), and the descriptor's numeric fallback would now be discarding a manual name: decide what the labels should be before this test is changed", catDialect.EXWire(it.Addr), it.P1Label, it.P2Label)
		}
	}

	for _, m := range SettingsDescriptor().Menus {
		if m.Label != m.ID {
			t.Errorf("menu %q Label = %q, want its own ID — the structural numeric fallback, not a manual group name", m.ID, m.Label)
		}
		for _, g := range m.Groups {
			if g.Label != g.ID {
				t.Errorf("group %q Label = %q, want its own ID", g.ID, g.Label)
			}
		}
	}
}

// TestSettingsDescriptor_ItemIDsAreGloballyUniqueFourDigitAddresses: a
// caller addressing ReadSetting by ID has no group context to disambiguate a
// collision, so uniqueness must hold across the WHOLE tree and not merely
// within a group. Validate enforces it too; this asserts it independently
// and adds the ID/Display shape, which Validate has no opinion about.
//
// FOUR digits, where the FTdx10's and FTdx101's are six: this radio's EX
// address is a cat.EXAddressPair (core/cat/ft891/dialect.go, layout 513-522)
// and cat.Dialect.EXWire renders it accordingly.
//
// Display EQUALS the ID here, and that is a FACT ABOUT THIS RADIO'S CHART
// RATHER THAN A RULE: the chart prints the address as one four-digit MENU
// Number, where the FTdx10's prints a "%02d-%02d-%02d" triple whose Display
// and ID genuinely differ (matrix §3.9). The two fields are built separately
// in buildSettingsDescriptor for that reason, and this test states the
// coincidence so that nobody later "de-duplicates" one into the other.
func TestSettingsDescriptor_ItemIDsAreGloballyUniqueFourDigitAddresses(t *testing.T) {
	d := SettingsDescriptor()

	seen := map[string]string{} // item ID -> the group it was first seen in
	var count int
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				count++
				if prev, dup := seen[it.ID]; dup {
					t.Errorf("item ID %q appears in group %q and group %q — item IDs must be globally unique", it.ID, prev, g.ID)
					continue
				}
				seen[it.ID] = g.ID

				if len(it.ID) != 4 {
					t.Errorf("item ID %q has length %d, want 4 (this dialect's EX wire address; the siblings' is 6)", it.ID, len(it.ID))
				}
				// The ID must round-trip through the dialect's own parser:
				// this is the exact string ReadSetting will be handed back,
				// so an ID the dialect cannot parse would be a setting the
				// descriptor advertises and the driver refuses.
				if _, err := catDialect.ParseEXAddress(it.ID); err != nil {
					t.Errorf("item ID %q does not parse as a member EX address: %v", it.ID, err)
				}
				if it.Display != it.ID {
					t.Errorf("item %q Display = %q, want the printed MENU Number %q — see this test's doc comment", it.ID, it.Display, it.ID)
				}
				// The ID's own prefix must be the menu it hangs under: the
				// partition is the address, so a descriptor that filed an
				// item under the wrong menu would still pass every count.
				if it.ID[:2] != m.ID {
					t.Errorf("item %q sits under menu %q, want menu %q (its own two-digit prefix)", it.ID, m.ID, it.ID[:2])
				}
			}
		}
	}
	if len(seen) != count {
		t.Errorf("%d items yielded %d distinct IDs", count, len(seen))
	}
}

// TestSettingsDescriptor_MenuSeventeenHoldsExactlyOneItem pins the
// inventory's degenerate menu.
//
// Menu 17 holds exactly one item, RESET at 1701 (matrix §3.9, and the
// group-ledger's own per-menu totals). A one-row menu is the case a
// change-of-prefix loop gets wrong most easily — an off-by-one that opened a
// menu on the row AFTER the change would swallow it into menu 16 and leave
// menu 18 short — and every count in the shape test above would still add up
// to 159.
func TestSettingsDescriptor_MenuSeventeenHoldsExactlyOneItem(t *testing.T) {
	for _, m := range SettingsDescriptor().Menus {
		if m.ID != "17" {
			continue
		}
		var items []driver.SettingItem
		for _, g := range m.Groups {
			items = append(items, g.Items...)
		}
		if len(items) != 1 {
			t.Fatalf("menu 17 holds %d items, want exactly 1 (matrix §3.9)", len(items))
		}
		if want := (driver.SettingItem{ID: "1701", Label: "RESET", Display: "1701"}); items[0] != want {
			t.Errorf("menu 17's only item = %+v, want %+v", items[0], want)
		}
		return
	}
	t.Fatal("the descriptor has no menu 17")
}

// TestSettingsDescriptor_VersionIsThisRadiosOwn: two radios' menu trees must
// never share a descriptor version, or a codeplug.MenuSnapshot taken from
// one would validate against the other's descriptor.
//
// The sibling versions are written here as LITERALS rather than imported:
// this package deliberately imports no sibling driver (doc.go), and a
// version string is exactly the kind of value that must not travel between
// them.
func TestSettingsDescriptor_VersionIsThisRadiosOwn(t *testing.T) {
	got := SettingsDescriptor().Version
	for _, sibling := range []string{"ft710-ex@1", "ftdx10-ex@1", "ftdx101-ex@1"} {
		if got == sibling {
			t.Errorf("Version = %q, which is a sibling's — a snapshot from that radio would validate against this descriptor", got)
		}
	}
}

// TestSettingsDescriptor_IsADefensiveCopy: each of the three getters must
// hand out an INDEPENDENT tree. The package holds one shared original, and a
// caller that mutated the value it was given would otherwise change what
// every later caller received — including callers in other packages, since
// app/settings.go and internal/wiring both pass these trees straight to
// view-building code.
//
// Mutation is applied at EVERY level (Version, a Menu's field, a Group's
// field, an Item's field, and the slice headers themselves), because a
// shallow copy would pass a Version-only check while sharing the item
// arrays.
func TestSettingsDescriptor_IsADefensiveCopy(t *testing.T) {
	for _, tt := range []struct {
		name string
		get  func() driver.SettingsDescriptor
	}{
		{"package-level SettingsDescriptor()", SettingsDescriptor},
		{"driver StaticSettingsDescriptor()", New(Simulated).(driver.StaticSettingsProvider).StaticSettingsDescriptor},
		// A hand-built Session, deliberately: the method is documented as
		// session-INDEPENDENT (it never consults s.dialect), so exercising
		// it on a zero Session is the claim rather than a shortcut — and it
		// spares this test one Open, which on this radio costs the AI0
		// window plus thirteen exchanges (ft891_test.go). The live-session
		// leg is in TestSession_ReadSetting_ScriptedRoundTrips, on a session
		// that already exists.
		{"Session.SettingsDescriptor()", (&Session{}).SettingsDescriptor},
	} {
		t.Run(tt.name, func(t *testing.T) {
			first := tt.get()
			if len(first.Menus) == 0 || len(first.Menus[0].Groups) == 0 || len(first.Menus[0].Groups[0].Items) == 0 {
				t.Fatalf("the descriptor is not deep enough to mutate: %+v", first)
			}

			first.Version = "mutated"
			first.Menus[0].ID = "mutated"
			first.Menus[0].Label = "mutated"
			first.Menus[0].Groups[0].ID = "mutated"
			first.Menus[0].Groups[0].Label = "mutated"
			first.Menus[0].Groups[0].Items[0].ID = "mutated"
			first.Menus[0].Groups[0].Items[0].Label = "mutated"
			first.Menus[0].Groups[0].Items[0].Display = "mutated"
			first.Menus[0].Groups = nil
			first.Menus = nil

			second := tt.get()
			if err := second.Validate(); err != nil {
				t.Fatalf("after mutating a previously returned tree, the next call returned an invalid one: %v", err)
			}
			if !reflect.DeepEqual(second, SettingsDescriptor()) {
				t.Error("a mutated caller copy changed what a later call returns — every getter must return a Clone()")
			}
		})
	}
}

// TestStaticSettingsDescriptor_MatchesPackageFuncAndSession: the
// driver-level optional capability, the package-level func and the session
// method must return EQUAL trees. This driver's settings surface depends
// only on the static EX inventory, never on anything Open discovers (unlike
// Session.Capabilities, which folds in per-session 5xx/EMG discovery), so
// all three agree by construction — and internal/wiring's
// StaticSettingsDescriptor serves the offline path from the driver one while
// app/settings.go prefers the session one, so a disagreement would show a
// user two different menu trees for the same radio.
func TestStaticSettingsDescriptor_MatchesPackageFuncAndSession(t *testing.T) {
	drv := New(Simulated)
	provider, ok := drv.(driver.StaticSettingsProvider)
	if !ok {
		t.Fatal("New(Simulated) does not implement driver.StaticSettingsProvider — internal/wiring.StaticSettingsDescriptor would report the FT-891 as having no settings surface at all, with no error anywhere")
	}

	want := SettingsDescriptor()
	if err := want.Validate(); err != nil {
		t.Fatalf("SettingsDescriptor().Validate() = %v, want nil", err)
	}
	if got := provider.StaticSettingsDescriptor(); !reflect.DeepEqual(got, want) {
		t.Error("StaticSettingsDescriptor() != the package-level SettingsDescriptor()")
	}
	if got := (&Session{}).SettingsDescriptor(); !reflect.DeepEqual(got, want) {
		t.Error("Session.SettingsDescriptor() != the package-level SettingsDescriptor()")
	}
}
