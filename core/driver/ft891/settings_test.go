// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// Compile-time proof of the two settings seams this task adds. Any
// signature drift fails the BUILD, not merely a test.
//
// BOTH halves are asserted, deliberately, because BOTH fail silently when
// absent: internal/wiring's StaticSettingsDescriptor helper asks for the
// driver-level interface by a two-result type assertion and reports "no
// settings surface" when a driver does not satisfy it, and app/settings.go
// reaches the session capability the same way
// (conn.session.(driver.SettingsReader)) and would quietly fall back to the
// static descriptor, serving every ReadSetting as unsupported. Neither
// produces an error anywhere; these assertions are the only thing that turns
// a renamed method or a changed receiver into a loud failure.
//
// They live HERE rather than in this package's optional_test.go, where the
// package's other seam assertions sit, because this task's whole surface is
// one subject in one pair of files — the same choice
// StaticSettingsDescriptor's own doc comment makes for the method.
var (
	_ driver.StaticSettingsProvider = (*ft891Driver)(nil)
	_ driver.SettingsReader         = (*Session)(nil)
)

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

// exItemByAddr returns the inventory row for a four-digit wire address, or
// fails: a fixture that names an address the inventory does not carry is a
// broken fixture, not a driver defect, and must say so where it is written.
func exItemByAddr(t *testing.T, wire string) cat.EXItem {
	t.Helper()
	for _, it := range catDialect.EXItems() {
		if catDialect.EXWire(it.Addr) == wire {
			return it
		}
	}
	t.Fatalf("no EX inventory row at address %q — this fixture names an address core/cat/ft891 does not carry", wire)
	return cat.EXItem{}
}

// exFullWidthValue builds a P4 body of exactly the width the inventory
// declares for wire, filled with fill. The width is DERIVED, so a fixture
// claiming to answer "at full width" cannot quietly answer at some other one
// — and if the transcribed Digits column ever changes, the fixture follows
// it instead of silently becoming a partial-width case.
func exFullWidthValue(t *testing.T, wire string, fill byte) string {
	t.Helper()
	it := exItemByAddr(t, wire)
	b := make([]byte, it.Digits)
	for i := range b {
		b[i] = fill
	}
	return string(b)
}

// The two addresses the scripted round-trips use, one at each end of this
// inventory's width range:
//
//	narrowSettingAddr — 0101 AGC FAST DELAY, the chart's very first row
//	                    (layout 525), 4 digits.
//	widestSettingAddr — 0803 OTHER DISP, one of the inventory's only two
//	                    5-digit items (layout 595-596) and therefore one of
//	                    the two that SET this dialect's own P4 answer bound
//	                    (cat.maxEXP4Bytes over the inventory).
//
// Written as literals rather than picked by index or by scanning for a
// width: a fixture that selected its own subject would keep passing after an
// inventory edit moved the case it was meant to cover, and these are the
// addresses the manual's chart puts at those positions.
const (
	narrowSettingAddr = "0101"
	widestSettingAddr = "0803"
)

// settingsTestImage is the scripted radio the settings session tests share:
//
//	0101 — answered at its own full declared width (4 bytes of '7')
//	0803 — the widest item, answered with the SIGNED 5-byte body its printed
//	       range implies ("-3000 Hz - 0 - +3000 Hz", layout 595-596). No
//	       FT-891 has ever answered anything, so this is the fixture asserting
//	       the driver returns whatever arrives, sign included, and never
//	       reshapes it
//	0102 — ABSENT from the map, so answered "?;": the Unavailable path
//	0103 — a frame whose prefix matches but which carries NO P4 body at all,
//	       one byte short of the narrowest EX answer this dialect defines
//	0201 — a frame whose P4 body is WIDER than this dialect's own derived
//	       bound (5, from the two OTHER DISP/SHIFT items): the other
//	       malformed edge
//
// One session serves all of them, which is safe because these are
// independent single exchanges with no state on either side — and it matters
// here, because every Open on this radio costs the AI0 window plus thirteen
// exchanges (ft891_test.go).
func settingsTestImage(t *testing.T) slotImage {
	t.Helper()
	return slotImage{exAnswers: map[string]string{
		narrowSettingAddr: "EX" + narrowSettingAddr + exFullWidthValue(t, narrowSettingAddr, '7') + ";",
		widestSettingAddr: "EX" + widestSettingAddr + "-3000" + ";",
		// 7 bytes: "EX" + the four-digit address + ';' with NO P4 at all.
		// It matches exSpec's prefix, so the engine hands it back as this
		// read's answer and the PARSER is what must refuse it.
		"0103": "EX0103;",
		// A 6-byte P4, one byte past this dialect's own derived maximum.
		"0201": "EX0201123456;",
	}}
}

// TestExSpec_FullAddressPrefixAndVariableLength pins the three decisions in
// exSpec that a plausible-looking alternative would get wrong.
//
// The PREFIX carries the whole four-digit address. EX is a shared-prefix
// family: 159 addresses answer under the same two command bytes, so a bare
// "EX" would let transport.Engine.Do correlate another address's answer as
// this read's own and report one setting's value under another's ID. The
// test proves two different addresses' specs actually discriminate, which a
// bare-"EX" spec would fail.
//
// The LENGTH is variable (the matcher pins no exact length), the deliberate
// opposite of mtSpec's exact derived length (read.go): this inventory's P4
// widths run 1 to 5 bytes, so there is no single length to pin, and pinning
// a per-item width from the transcribed Digits column would make a spec out
// of a number no FT-891 has ever confirmed — the FT-710's own M8c sweep
// found that column wrong for one of its addresses (core/cat/ex.go's
// ParseEXAnswer records it).
//
// ONE RETRY, where mtSpec has none. The zero on the MT read is plan P11's
// decision about a command this manual's own Control Command List says has
// no Read at all (doc.go, "The MT Read contradiction"), so a silent retry
// there would double the frames sent to test a registered assumption. EX has
// no such contradiction — the availability row reads `EX | MENU | O O O O`
// (layout 142) — so the ordinary "a read is idempotent" reasoning every
// sibling's read spec uses applies here unchanged.
func TestExSpec_FullAddressPrefixAndVariableLength(t *testing.T) {
	a := exItemByAddr(t, narrowSettingAddr).Addr
	b := exItemByAddr(t, widestSettingAddr).Addr

	specA := exSpec(catDialect, a)
	if specA.Class != transport.ClassRead {
		t.Errorf("exSpec(%s).Class = %v, want transport.ClassRead", catDialect.EXWire(a), specA.Class)
	}
	if specA.RetryReads != 1 {
		t.Errorf("exSpec.RetryReads = %d, want 1 (an EX read is idempotent, and this command's Read is not the contradicted one — see this test's doc comment)", specA.RetryReads)
	}
	// The full-address prefix and the variable length, asserted THROUGH THE
	// MATCHER rather than off the struct: answer matching lives in an opaque
	// transport.CommandSpec.Match built by the codec, so there is no
	// ExpectPrefix field to read. The property those fields pinned is the
	// one that matters and is stronger stated this way — a's spec accepts
	// a's own answer and REFUSES b's, which is exactly the wrong-address
	// correlation a bare "EX" prefix would permit.
	ownAnswer := "EX" + catDialect.EXWire(a) + "1;"
	othersAnswer := "EX" + catDialect.EXWire(b) + "1;"
	if !specA.Match([]byte(ownAnswer)) {
		t.Errorf("exSpec(%s).Match(%q) = false, want true — that is this address's own answer", catDialect.EXWire(a), ownAnswer)
	}
	if specA.Match([]byte(othersAnswer)) {
		t.Errorf("exSpec(%s).Match(%q) = true, want false — a shared prefix is exactly what lets one address's answer be correlated as another's", catDialect.EXWire(a), othersAnswer)
	}
	// Variable length: two answers of different widths, both this address's,
	// both matched.
	wide := "EX" + catDialect.EXWire(a) + "12345;"
	if !specA.Match([]byte(wide)) {
		t.Errorf("exSpec(%s).Match(%q) = false, want true — the length must stay variable: this inventory's P4 widths run 1..5 bytes", catDialect.EXWire(a), wide)
	}
}

// TestSession_ReadSetting_ScriptedRoundTrips drives ReadSetting through the
// real transport.Engine against the scripted radio, over the three outcomes
// a real exchange can produce.
//
// Raw values are compared VERBATIM: this surface reports what the radio sent
// and interprets nothing (see buildSettingsDescriptor on why — this
// dialect's recorded chart printing defects all live in value legends this
// driver never reads).
func TestSession_ReadSetting_ScriptedRoundTrips(t *testing.T) {
	p, sess := openSession(t, Simulated, settingsTestImage(t))

	// The live-session leg of the three-way descriptor agreement, on a
	// session that already exists: the concrete type Open returned really
	// does satisfy the optional capability, and serves the same tree.
	var opened driver.Session = sess
	reader, ok := opened.(driver.SettingsReader)
	if !ok {
		t.Fatal("the session Open returned does not implement driver.SettingsReader — app/settings.go reaches this capability by a two-result type assertion and would silently fall back to the static descriptor")
	}
	if !reflect.DeepEqual(reader.SettingsDescriptor(), SettingsDescriptor()) {
		t.Error("the live session's SettingsDescriptor() differs from the package-level one")
	}

	t.Run("an item answered at its full declared width", func(t *testing.T) {
		wantRaw := exFullWidthValue(t, narrowSettingAddr, '7')
		if len(wantRaw) != 4 {
			t.Fatalf("fixture: %s declares Digits %d, want 4 (AGC FAST DELAY) — reconcile the fixture with the inventory", narrowSettingAddr, len(wantRaw))
		}

		before := len(p.Transcript())
		got, err := sess.ReadSetting(testCtx(t), narrowSettingAddr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v", narrowSettingAddr, err)
		}
		want := driver.SettingValue{ID: narrowSettingAddr, Raw: wantRaw, State: driver.SettingKnown}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v", narrowSettingAddr, got, want)
		}

		// ONE frame, SEVEN bytes, carrying the full address: this radio's EX
		// read is "EX" + FOUR digits + ';' (layout 513-522), the one
		// shared-frame length that moves between it and its nine-byte
		// siblings.
		wantFrame := "EX" + narrowSettingAddr + ";"
		if len(wantFrame) != 7 {
			t.Fatalf("fixture: the expected read frame %q is %d bytes, want 7", wantFrame, len(wantFrame))
		}
		if sent := p.Transcript()[before:]; len(sent) != 1 || sent[0] != wantFrame {
			t.Errorf("one ReadSetting sent %v, want exactly [%q]", sent, wantFrame)
		}
	})

	t.Run("the widest item answered with a signed full-width body", func(t *testing.T) {
		got, err := sess.ReadSetting(testCtx(t), widestSettingAddr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v", widestSettingAddr, err)
		}
		const wantRaw = "-3000"
		if len(wantRaw) != exItemByAddr(t, widestSettingAddr).Digits {
			t.Fatalf("fixture: the answer is %d bytes but the inventory declares Digits %d", len(wantRaw), exItemByAddr(t, widestSettingAddr).Digits)
		}
		want := driver.SettingValue{ID: widestSettingAddr, Raw: wantRaw, State: driver.SettingKnown}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v — the P4 body is returned VERBATIM, sign included; the no-trim half of the same rule is in TestParseEXResponse_Table", widestSettingAddr, got, want)
		}
	})

	t.Run("a declared address the radio rejects is Unavailable, not an error", func(t *testing.T) {
		const addr = "0102"   // absent from the image, so answered "?;"
		exItemByAddr(t, addr) // it IS a declared inventory member

		got, err := sess.ReadSetting(testCtx(t), addr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v, want nil — a \"?;\" rejection is a recorded fact about this exchange, not an error (the rule ReadChannel's empty-slot mapping already established)", addr, err)
		}
		want := driver.SettingValue{ID: addr, State: driver.SettingUnavailable}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v (Raw must be empty when Unavailable)", addr, got, want)
		}
	})
}

// TestSession_ReadSetting_ErrorTyping covers the two refusal classes, both
// TYPED and both reachable through the real session path.
//
// An UNKNOWN ID is this driver's own *UnknownSettingError, raised before any
// wire traffic at all — the transcript assertion is what proves "before",
// and it matters because a driver that asked the radio about an address its
// own dialect does not declare would be inventing a question.
//
// A MALFORMED ANSWER stays the parser's typed *cat.ParseError under this
// driver's wrap, exactly as ReadChannel's own error typing does (read.go):
// the parser owns the verdict, the driver adds the address the bare parser
// cannot know. Neither class is a bare fmt.Errorf, so a caller can tell "you
// asked for something that does not exist" from "the radio said something
// this protocol does not define".
func TestSession_ReadSetting_ErrorTyping(t *testing.T) {
	p, sess := openSession(t, Simulated, settingsTestImage(t))

	t.Run("an unknown setting ID is refused before the wire", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			id   string
		}{
			{"malformed shape", "not-an-address"},
			{"right shape, one digit short", "010"},
			{"non-digits in the address field", "01X1"},
			// The SIBLINGS' six-digit shape. This radio's address field is
			// four digits (cat.EXAddressPair, layout 513-522), so a caller
			// that carried an FT-710 or FTdx10 setting ID across must be
			// refused rather than have two of its digits quietly dropped.
			{"a sibling's six-digit address", "010101"},
			// Well-formed, in range, and NOT a chart row: menu 01 holds
			// three items (0101-0103), so 0104 is a four-digit number the
			// grammar block's "0101 - 1803" range admits and the chart does
			// not enumerate.
			{"a well-formed address the chart does not carry", "0104"},
			// One past the chart's own last row, 1803 (layout 697).
			{"one past the chart's last row", "1804"},
			{"the zero address", "0000"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				before := len(p.Transcript())

				_, err := sess.ReadSetting(testCtx(t), tt.id)

				var unknown *UnknownSettingError
				if !errors.As(err, &unknown) {
					t.Fatalf("ReadSetting(%q) error = %v (%T), want *UnknownSettingError", tt.id, err, err)
				}
				if unknown.ID != tt.id {
					t.Errorf("UnknownSettingError.ID = %q, want %q", unknown.ID, tt.id)
				}
				if sent := p.Transcript()[before:]; len(sent) != 0 {
					t.Errorf("refused ReadSetting(%q) sent %v, want nothing — the refusal must precede ALL wire traffic", tt.id, sent)
				}
			})
		}
	})

	t.Run("a malformed answer is a wrapped cat.ParseError", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			addr string
		}{
			{"no P4 body at all: shorter than any EX answer", "0103"},
			{"a P4 body wider than this dialect's derived bound", "0201"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				_, err := sess.ReadSetting(testCtx(t), tt.addr)
				if err == nil {
					t.Fatalf("ReadSetting(%q) = nil error, want a refusal", tt.addr)
				}
				var pe *cat.ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("error %v (%T) is not a wrapped *cat.ParseError — the frame's shape is the PARSER's verdict, and its typed error must survive this driver's wrap", err, err)
				}
				if !strings.Contains(err.Error(), tt.addr) {
					t.Errorf("error text %q does not name address %q — the driver's wrap is what adds the context the parser cannot know", err.Error(), tt.addr)
				}
			})
		}
	})
}

// TestParseEXResponse_Table drives the PURE helper directly with hand-built
// frames.
//
// This is the ONLY way to reach the wrong-address branch, and that is by
// design rather than a testing convenience: exSpec's match prefix carries
// the complete four-digit address, so transport.Engine.Do can only ever hand
// back a frame that already matches the address requested — a genuinely
// differently-addressed reply fails Do's own matching and is counted as an
// unexpected frame. The branch is defence in depth against that guarantee
// regressing, and a test that could only reach it through a session could
// not test it at all.
func TestParseEXResponse_Table(t *testing.T) {
	requested := exItemByAddr(t, narrowSettingAddr).Addr
	other := exItemByAddr(t, widestSettingAddr).Addr

	for _, tt := range []struct {
		name         string
		frame        string
		want         driver.SettingValue
		wantErr      bool
		wantMismatch bool
	}{
		{
			name:  "a well-formed answer for the requested address",
			frame: "EX" + catDialect.EXWire(requested) + "123;",
			want:  driver.SettingValue{ID: catDialect.EXWire(requested), Raw: "123", State: driver.SettingKnown},
		},
		{
			// VERBATIM means no trim. A body carrying leading and trailing
			// spaces comes back byte-identical, so the "no trim, no typed
			// value model" claim is pinned and not merely asserted in prose
			// — deleting the trim would otherwise be invisible on an
			// inventory whose every item is numeric.
			//
			// This is a statement about THIS DRIVER, not about the radio: no
			// FT-891 has ever answered anything, and nothing in this chart
			// suggests a padded numeric body. The FTdx10 and FT-710 pin the
			// same rule with their own 12-byte space-padded text items,
			// which this inventory has none of (no Text row at all).
			name:  "a body with surrounding whitespace is not trimmed",
			frame: "EX" + catDialect.EXWire(requested) + " 12 ;",
			want:  driver.SettingValue{ID: catDialect.EXWire(requested), Raw: " 12 ", State: driver.SettingKnown},
		},
		{
			name:  "the rejection frame",
			frame: "?;",
			want:  driver.SettingValue{ID: catDialect.EXWire(requested), State: driver.SettingUnavailable},
		},
		{
			name:         "a well-formed answer for a DIFFERENT known address",
			frame:        "EX" + catDialect.EXWire(other) + "5;",
			wantErr:      true,
			wantMismatch: true,
		},
		{
			name:    "a frame that is not an EX answer at all",
			frame:   "garbage;",
			wantErr: true,
		},
		{
			name:    "an EX answer at an address this dialect does not declare",
			frame:   "EX01045;",
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEXResponse(catDialect, requested, []byte(tt.frame))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseEXResponse(%q) = nil error, want an error", tt.frame)
				}
				if tt.wantMismatch {
					var mismatch *SettingAnswerMismatchError
					if !errors.As(err, &mismatch) {
						t.Fatalf("error %v (%T), want *SettingAnswerMismatchError", err, err)
					}
					if mismatch.Requested != catDialect.EXWire(requested) || mismatch.Answered != catDialect.EXWire(other) {
						t.Errorf("mismatch = %+v, want Requested=%q Answered=%q", mismatch, catDialect.EXWire(requested), catDialect.EXWire(other))
					}
					// It must NOT be mistakable for the slot-worded error
					// this package already has (ft891.go): two different
					// wire-address namespaces, two types — and that type
					// carries an errors.Is sentinel this one deliberately
					// does not.
					var slotMismatch *AnswerMismatchError
					if errors.As(err, &slotMismatch) {
						t.Error("an EX address mismatch also matched *AnswerMismatchError — the slot-worded type must not stand in for the setting one")
					}
					if errors.Is(err, ErrAnswerMismatch) {
						t.Error("an EX address mismatch satisfies errors.Is(err, ErrAnswerMismatch) — that sentinel answers \"did the radio answer about the wrong CHANNEL?\", which this is not")
					}
					return
				}
				var pe *cat.ParseError
				if !errors.As(err, &pe) {
					t.Errorf("error %v (%T) is not a wrapped *cat.ParseError", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseEXResponse(%q): unexpected error: %v", tt.frame, err)
			}
			if got != tt.want {
				t.Errorf("parseEXResponse(%q) = %+v, want %+v", tt.frame, got, tt.want)
			}
		})
	}
}

// TestReadSetting_CannotInterleaveWithACrossCheck pins that ReadSetting
// takes the SAME s.opMu ReadChannel and WriteChannel hold (plan P3, spec
// S-E4, matrix M-E2).
//
// This driver's Session differs from the FTdx10's on exactly this point, and
// the difference is structural rather than stylistic: a memory or PMS read
// here is potentially TWO exchanges — the combined MT read, then the
// cross-check's MR when MT answers "?;" (read.go, matrix §3.5) — and
// transport.Engine serialises each individual exchange rather than a pair.
// The FTdx10's ReadSetting takes no lock because that driver's Session has
// none to take; copying its reasoning here would leave a gap a settings read
// could land in. What that would cost is not a corrupted SETTING (one EX
// exchange is self-contained and the engine already serialises it) but a
// corrupted CHANNEL: the cross-check would be interpreting an MT rejection
// against radio state a third frame had been sent into.
//
// Forced deterministically through readChannelGapHook rather than by
// hammering, for the reason
// TestReadChannel_CrossCheckIsAtomicUnderOpMu records: Go's sync.Mutex
// favours an immediately-re-locking goroutine so heavily that this
// interleaving is near-impossible to reproduce by scheduling luck.
func TestReadSetting_CannotInterleaveWithACrossCheck(t *testing.T) {
	img := settingsTestImage(t)
	img.mtAnswers = readTestImage().mtAnswers
	img.mrAnswers = readTestImage().mrAnswers
	p, sess := openSession(t, Simulated, img)
	before := len(p.Transcript())

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	readChannelGapHook = func() {
		once.Do(func() { close(reached) })
		<-release
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	channelDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadChannel(testCtx(t), "002") // the cross-check path
		channelDone <- err
	}()

	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("ReadChannel never reached the gap between its MT rejection and its MR read within 5s")
	}

	settingDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadSetting(testCtx(t), narrowSettingAddr)
		settingDone <- err
	}()

	// A generous, deterministic window for the settings read to reach the
	// wire if nothing is holding it back.
	time.Sleep(500 * time.Millisecond)
	if got := p.Transcript()[before:]; len(got) != 1 || got[0] != "MT002;" {
		t.Errorf("while one ReadChannel was between its MT and MR, the wire carried %v — want only [\"MT002;\"]: a settings read must not interleave with a cross-check (P3)", got)
	}

	close(release)
	for _, done := range []chan error{channelDone, settingDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("a concurrent read failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a read never completed after the gap hook was released")
		}
	}

	if got, want := p.Transcript()[before:], []string{"MT002;", "MR002;", "EX" + narrowSettingAddr + ";"}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v — the cross-check's two frames must be adjacent", got, want)
	}
}

// TestCloneReadSettings_WalksTheWholeDescriptor drives core/clone's
// ReadSettings — the layer this descriptor exists for — end to end over the
// scripted radio, and is the only test that puts every one of the
// descriptor's items on the wire.
//
// IT IS THE PREFLIGHT THAT MATTERS MOST. core/clone/settings.go refuses
// before any wire traffic if the descriptor fails driver.SettingsDescriptor.
// Validate, and again if an all-MenuUnsupported codeplug.MenuSnapshot built
// from its item IDs fails codeplug.MenuSnapshot.Validate — which requires
// every ID to be EXACTLY four or exactly six ASCII digits. This radio is the
// first registered Yaesu whose IDs are the four-digit half of that rule
// (core/codeplug/menus.go's isSettingIDWidth), so a descriptor that minted
// anything else would read the whole radio and only then fail. Neither
// package-level test can see this: the driver's own tests validate the
// descriptor but know nothing of the snapshot rule, and core/clone's tests
// use their own fixtures.
//
// The answers are built from the INVENTORY's declared width per item, not
// from one shared literal, so the walk exercises the full 1..5-byte P4 range
// this dialect derives its answer bound from rather than one width 159
// times.
//
// Complete is TRUE here because every item answers. The partial-snapshot
// behaviour — a "?;" becoming MenuUnavailable with the read continuing — is
// core/clone's own rule, pinned in its package; the driver half it depends
// on (a rejection is SettingUnavailable and NOT an error) is pinned by
// TestSession_ReadSetting_ScriptedRoundTrips.
func TestCloneReadSettings_WalksTheWholeDescriptor(t *testing.T) {
	d := SettingsDescriptor()

	// One answer per descriptor item, each at that item's own declared
	// width, filled with a digit that varies per item so a snapshot that
	// mapped one answer onto another's ID could not pass unnoticed.
	//
	// The width is taken from the inventory BY POSITION rather than by
	// looking the ID up in it, deliberately: this fixture must not depend on
	// the ID being a well-formed inventory address, or a mis-shaped ID would
	// break the FIXTURE instead of reaching the clone preflight that exists
	// to catch it. That the descriptor's items are the inventory in its own
	// order is pinned separately, by
	// TestSettingsDescriptor_ShapeFromTheInventory.
	items := catDialect.EXItems()
	answers := map[string]string{}
	wantValue := map[string]string{}
	var order []string
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				if len(order) >= len(items) {
					t.Fatalf("the descriptor holds more items than the inventory's %d", len(items))
				}
				raw := strings.Repeat(string(byte('0'+len(order)%10)), items[len(order)].Digits)
				answers[it.ID] = "EX" + it.ID + raw + ";"
				wantValue[it.ID] = raw
				order = append(order, it.ID)
			}
		}
	}

	_, sess := openSession(t, Simulated, slotImage{exAnswers: answers})
	snap, err := clone.NewService(sess, clone.SnapshotStore{}).ReadSettings(testCtx(t))
	if err != nil {
		t.Fatalf("clone.ReadSettings: unexpected error: %v", err)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("the returned snapshot fails codeplug.MenuSnapshot.Validate: %v", err)
	}
	if snap.Descriptor != d.Version {
		t.Errorf("snapshot Descriptor = %q, want this driver's own %q carried through verbatim", snap.Descriptor, d.Version)
	}
	if !snap.Complete {
		t.Error("snapshot Complete = false, want true — every item was answered")
	}
	if len(snap.Entries) != len(order) {
		t.Fatalf("snapshot holds %d entries, the descriptor declares %d items", len(snap.Entries), len(order))
	}
	for i, e := range snap.Entries {
		if e.ID != order[i] {
			t.Fatalf("Entries[%d].ID = %q, want %q — ReadSettings walks the descriptor in its own order", i, e.ID, order[i])
		}
		if e.State != codeplug.MenuKnown || e.Value != wantValue[e.ID] {
			t.Errorf("entry %q = {Value:%q State:%v}, want {Value:%q State:MenuKnown}", e.ID, e.Value, e.State, wantValue[e.ID])
		}
	}
}
