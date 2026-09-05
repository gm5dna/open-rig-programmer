// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// The optional settings capability is claimed at BOTH levels, and the two
// assertions are compile-time on purpose: app/settings.go reaches each by a
// two-result type assertion and would silently fall back rather than fail if
// either were dropped.
var (
	_ driver.StaticSettingsProvider = (*ft991aDriver)(nil)
	_ driver.SettingsReader         = (*Session)(nil)
)

// registeredInventoryCount returns the item count the REGISTERED
// internal/extable profile for package ft991a implies: its chart row count
// LESS the addresses it declares parameterless.
//
// It is the staleness bound this file's shape assertions are measured
// against, and it is taken from the registry rather than written here as 152
// on purpose: the count's datum is the dialect's generated inventory
// (core/cat/ft991a/exinventory_gen.go, from table2.csv) while its bound comes
// from the per-page ledger a quarantined agent derived from the rendered PDF
// before either transcription existed (core/cat/ft991a/testdata/ledger.csv,
// three rows summing to 153) — the bound-consulted-from-one-place,
// datum-taken-from-another rule, which a literal in this file would break.
//
// THE ARITHMETIC IS THE FT-891'S PLUS ONE TERM, and the extra term is this
// radio's own: ExpectedRows COUNTS row 087 because the chart prints it, and
// the inventory excludes it because it names no field an EX frame could read
// or write (the dialect register's entry ROW 087 RADIO ID'S EXCLUSION). This
// is the same arithmetic core/cat/ft991a's TestEXItemsCountMatchesProfile
// makes, consulted here for the DESCRIPTOR rather than for the inventory.
//
// The profile is selected by Package rather than by lookup name, the same
// selection that test makes and for the reason its own comment gives.
func registeredInventoryCount(t *testing.T) int {
	t.Helper()
	var matches []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == "ft991a" {
			matches = append(matches, np)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("internal/extable holds %d registered profiles emitting into package ft991a, want exactly 1", len(matches))
	}
	p := matches[0].Profile
	return p.ExpectedRows - len(p.ParameterlessAddresses)
}

// flatShapeErr is THE SHAPE ASSERTION: it states, as a predicate over a
// descriptor rather than as a run of t.Errorf calls, what "the FT-991A's
// settings tree is FLAT" means — exactly one menu and exactly one group
// inside it, both carrying flatMenuID as ID and as Label.
//
// IT IS A FUNCTION SO THAT IT CAN BE FAILED ON PURPOSE. Task 12's brief asks
// for a red proof that the one menu is a DECLARED FALLBACK and not an
// accident of this inventory's shape, and a shape stated only as assertions
// against the real descriptor cannot be shown to bite: it would pass equally
// if it asserted nothing. TestSettingsDescriptor_TwoMenusFailTheShape-
// Assertion hands this function a hand-built TWO-menu descriptor for this
// model and requires an error, so a later "improvement" that invents group
// structure cannot land silently.
//
// driver.SettingsDescriptor.Validate deliberately says NOTHING about any of
// this — a two-menu tree with distinct IDs is perfectly valid there, and
// that test asserts it — so this predicate is the only thing standing
// between the radio's documented chart and an invented hierarchy.
func flatShapeErr(d driver.SettingsDescriptor) error {
	if len(d.Menus) != 1 {
		return fmt.Errorf("descriptor holds %d menus, want exactly 1: the FT-991A menu chart has no group label columns and no substructure at all, so the tree is FLAT (matrix §3.9, spec decision 6, plan P9)", len(d.Menus))
	}
	m := d.Menus[0]
	if m.ID != flatMenuID || m.Label != flatMenuID {
		return fmt.Errorf("the single menu is {ID:%q Label:%q}, want both %q — the manual's own heading for the command (layout 519, the line reads `EX          MENU`), which is the only name this project has for it", m.ID, m.Label, flatMenuID)
	}
	if len(m.Groups) != 1 {
		return fmt.Errorf("the single menu holds %d groups, want exactly 1: the group exists because driver.SettingsDescriptor is a two-level tree, NOT because this radio has a subgroup", len(m.Groups))
	}
	g := m.Groups[0]
	if g.ID != flatMenuID || g.Label != flatMenuID {
		return fmt.Errorf("the single group is {ID:%q Label:%q}, want both %q", g.ID, g.Label, flatMenuID)
	}
	return nil
}

// TestSettingsDescriptor_IsTheDeclaredFlatFallback pins the shape, the
// version, and the EVIDENCE the shape rests on.
//
// THE EMPTY-LABEL HALF IS THE LOAD-BEARING ONE, exactly as on the FT-891.
// The descriptor falls back to the manual's own command heading because this
// chart prints no label columns at all (the registered profile declares
// LabelsAbsent, so every EXItem's P1Label and P2Label is ""). If a later
// transcription ever gave this inventory real names, the fallback would be
// throwing a manual name away — so this test fails then, at the fallback,
// rather than leaving the descriptor quietly worse than its source.
//
// This radio has LESS to fall back on than any sibling: the FT-891's
// four-digit MENU Number at least decomposes into a two-digit prefix its
// descriptor partitions on, and this one's three digits have no substructure
// whatever.
func TestSettingsDescriptor_IsTheDeclaredFlatFallback(t *testing.T) {
	d := SettingsDescriptor()

	if err := d.Validate(); err != nil {
		t.Fatalf("SettingsDescriptor().Validate() = %v, want nil", err)
	}
	if err := flatShapeErr(d); err != nil {
		t.Fatalf("SettingsDescriptor() is not the declared flat shape: %v", err)
	}
	if d.Version != "ft991a-ex@1" {
		t.Errorf("Version = %q, want \"ft991a-ex@1\" (minted in this package; codeplug.MenuSnapshot.Descriptor carries it verbatim)", d.Version)
	}

	// The evidence the fallback rests on: no label column anywhere in the
	// inventory the descriptor is built from.
	for _, it := range catDialect.EXItems() {
		if it.P1Label != "" || it.P2Label != "" {
			t.Fatalf("inventory row %s carries P1Label %q / P2Label %q — this chart prints no label columns (matrix §3.9), and the descriptor's %q fallback would now be discarding a manual name: decide what the labels should be before this test is changed", catDialect.EXWire(it.Addr), it.P1Label, it.P2Label, flatMenuID)
		}
	}

	// A radio's descriptor version must never be a sibling's, or a
	// codeplug.MenuSnapshot taken from one would validate against the
	// other's descriptor. The sibling versions are LITERALS rather than
	// imports: this package imports no sibling driver (doc.go), and a
	// version string is exactly the kind of value that must not travel
	// between them.
	for _, sibling := range []string{"ft710-ex@1", "ftdx10-ex@1", "ftdx101-ex@1", "ft891-ex@1"} {
		if d.Version == sibling {
			t.Errorf("Version = %q, which is a sibling's — a snapshot from that radio would validate against this descriptor", d.Version)
		}
	}
}

// TestSettingsDescriptor_TwoMenusFailTheShapeAssertion is the RED PROOF task
// 12's brief asks for: that the ONE menu is a DECLARED FALLBACK and not an
// accident.
//
// A hand-built descriptor for THIS MODEL — this package's own version
// string, this inventory's own items, in order — split across two menus is
// handed to both gates. driver.SettingsDescriptor.Validate ACCEPTS it: two
// menus with distinct IDs, each with a group and items, is a perfectly legal
// neutral tree, and that acceptance is asserted here rather than assumed,
// because it is the whole reason a second gate has to exist. flatShapeErr
// REFUSES it.
//
// So an "improvement" that invented groups for this radio — a synthetic
// 001-099 / 100-153 split, or names from the FT-991A operating manual, which
// this project does not hold (spec §Non-goals) — cannot land silently: it
// fails here, at the shape, with the manual fact quoted in the message.
func TestSettingsDescriptor_TwoMenusFailTheShapeAssertion(t *testing.T) {
	items := catDialect.EXItems()
	if len(items) < 2 {
		t.Fatalf("the inventory holds %d items — this proof needs at least two to split", len(items))
	}
	split := len(items) / 2

	twoMenus := driver.SettingsDescriptor{Version: settingsDescriptorVersion}
	for i, part := range [][]cat.EXItem{items[:split], items[split:]} {
		menuID := []string{"001-076", "077-153"}[i]
		g := driver.SettingGroup{ID: menuID, Label: menuID}
		for _, it := range part {
			g.Items = append(g.Items, driver.SettingItem{
				ID:      catDialect.EXWire(it.Addr),
				Label:   it.Name,
				Display: catDialect.EXWire(it.Addr),
			})
		}
		twoMenus.Menus = append(twoMenus.Menus, driver.SettingMenu{ID: menuID, Label: menuID, Groups: []driver.SettingGroup{g}})
	}

	if err := twoMenus.Validate(); err != nil {
		t.Fatalf("the two-menu descriptor fails driver.SettingsDescriptor.Validate (%v) — then this proof is vacuous: the point is that the NEUTRAL type accepts invented structure and only this package's own shape assertion refuses it", err)
	}
	if err := flatShapeErr(twoMenus); err == nil {
		t.Fatal("flatShapeErr accepted a TWO-menu descriptor for this model — the one menu would then be an accident of this inventory rather than a declared fallback, and a later change that invented groups could land with every other test still green")
	}

	// The same proof one level down: one menu, TWO groups. The FTdx10's
	// second level is a real (P1,P2) partition off a real label column and
	// this radio has neither, so an invented subgroup must fail too.
	twoGroups := SettingsDescriptor()
	twoGroups.Menus[0].Groups = append(twoGroups.Menus[0].Groups, driver.SettingGroup{
		ID: "EXTRA", Label: "EXTRA", Items: []driver.SettingItem{{ID: "999", Label: "INVENTED", Display: "999"}},
	})
	if err := twoGroups.Validate(); err != nil {
		t.Fatalf("the two-group descriptor fails Validate (%v) — see above: this leg needs the neutral type to accept it", err)
	}
	if err := flatShapeErr(twoGroups); err == nil {
		t.Fatal("flatShapeErr accepted a descriptor with a SECOND group — the single group exists because the neutral type is a two-level tree, not because this radio has a subgroup")
	}
}

// TestSettingsDescriptor_ItemsAreTheInventoryInOrder pins the descriptor's
// contents against the inventory it is derived from: the COUNT (through the
// registry's own arithmetic, not a literal), the ORDER, and the three fields
// per item.
//
// 152 ITEMS FOR A CHART PRINTING 153 ROWS, and the missing one is named
// here: 087 RADIO ID, which prints ten hyphens for its parameter and a single
// hyphen for its Digits (layout 623), so the chart gives it no width and no
// parameter. This is the driver-side site of the dialect register's entry
// ROW 087 RADIO ID'S EXCLUSION — the descriptor is what makes the exclusion
// USER-VISIBLE, because it is the count a viewer shows.
//
// Display EQUALS the ID here, and that is A FACT ABOUT THIS RADIO'S CHART
// RATHER THAN A RULE: the chart prints the address as one three-digit MENU
// Number, where the FTdx10's prints a "%02d-%02d-%02d" triple whose Display
// and ID genuinely differ (matrix §3.9). The two fields are built as separate
// expressions in buildSettingsDescriptor for that reason, and this test
// states the coincidence so that nobody later "de-duplicates" one into the
// other and silently makes the ID the display form of whatever address shape
// comes next.
func TestSettingsDescriptor_ItemsAreTheInventoryInOrder(t *testing.T) {
	items := catDialect.EXItems()
	if want := registeredInventoryCount(t); len(items) != want {
		t.Fatalf("catDialect.EXItems() = %d items; the registered ft991a profile's rows less its declared parameterless addresses is %d — reconcile core/cat/ft991a with the ledger before touching this file", len(items), want)
	}

	var got []driver.SettingItem
	for _, m := range SettingsDescriptor().Menus {
		for _, g := range m.Groups {
			got = append(got, g.Items...)
		}
	}

	want := make([]driver.SettingItem, 0, len(items))
	for _, it := range items {
		wire := catDialect.EXWire(it.Addr)
		want = append(want, driver.SettingItem{ID: wire, Label: it.Name, Display: wire})
	}
	if !reflect.DeepEqual(got, want) {
		if len(got) != len(want) {
			t.Fatalf("the descriptor holds %d items, the inventory %d", len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("item %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	}

	// The excluded row is absent, stated positively: a descriptor that
	// carried 087 would advertise a setting this dialect refuses to build a
	// frame for, and every count above would still add up if the exclusion
	// moved to some other address.
	const excluded = "087"
	for _, it := range got {
		if it.ID == excluded {
			t.Errorf("the descriptor carries item %q — the chart prints that row with no parameter at all (layout 623) and the inventory excludes it BY ADDRESS (the register's ROW 087 RADIO ID'S EXCLUSION); advertising it would promise a setting ReadSetting must refuse", excluded)
		}
	}
	if _, err := catDialect.ParseEXAddress(excluded); err == nil {
		t.Errorf("catDialect.ParseEXAddress(%q) = nil error — the exclusion is the DIALECT's, and this test's premise is that a descriptor built from EXItems() cannot reach it", excluded)
	}
}

// TestSettingsDescriptor_IDsAreGloballyUniqueThreeDigitAddresses asserts
// uniqueness AT ALL THREE LEVELS — menu, group and item — independently of
// driver.SettingsDescriptor.Validate, which enforces the same three, and
// adds the ID shape, which Validate has no opinion about.
//
// A caller addressing ReadSetting by ID has no group context to disambiguate
// a collision, so item uniqueness must hold across the WHOLE tree and not
// merely within a group. On a one-menu, one-group tree the first two levels
// are trivially satisfied today; they are asserted anyway, because the flat
// shape is a documented FALLBACK rather than a permanent fact and the
// uniqueness rule must outlive it.
//
// THREE digits, where the FT-891's is four and the FTdx10's six: this
// radio's EX address is a cat.EXAddressSingle (core/cat/ft991a/dialect.go,
// layout 520) and cat.Dialect.EXWire renders it accordingly. A three-digit
// ID is STORABLE only because core/codeplug's isSettingIDWidth admits
// exactly three, four or six ASCII digits — the Kenwood base dependency —
// and padding a menu number to four digits, printing an ID no FT-991A
// document contains, is forbidden (plan P9). The clone preflight in
// TestCloneReadSettings_WalksTheWholeDescriptor is where that rule bites.
func TestSettingsDescriptor_IDsAreGloballyUniqueThreeDigitAddresses(t *testing.T) {
	d := SettingsDescriptor()

	menuIDs := map[string]bool{}
	groupIDs := map[string]bool{}
	itemIDs := map[string]string{} // item ID -> the group it was first seen in
	var count int

	for _, m := range d.Menus {
		if menuIDs[m.ID] {
			t.Errorf("duplicate menu ID %q", m.ID)
		}
		menuIDs[m.ID] = true
		for _, g := range m.Groups {
			if groupIDs[g.ID] {
				t.Errorf("duplicate group ID %q (group IDs are unique across the whole tree, not within their menu)", g.ID)
			}
			groupIDs[g.ID] = true
			for _, it := range g.Items {
				count++
				if prev, dup := itemIDs[it.ID]; dup {
					t.Errorf("item ID %q appears in group %q and group %q — item IDs must be globally unique", it.ID, prev, g.ID)
					continue
				}
				itemIDs[it.ID] = g.ID

				if len(it.ID) != 3 {
					t.Errorf("item ID %q has length %d, want 3 (this dialect's EX wire address; the FT-891's is 4 and the FTdx10's 6)", it.ID, len(it.ID))
				}
				// The ID must round-trip through the dialect's own parser:
				// this is the exact string ReadSetting will be handed back,
				// so an ID the dialect cannot parse would be a setting the
				// descriptor advertises and the driver refuses.
				if _, err := catDialect.ParseEXAddress(it.ID); err != nil {
					t.Errorf("item ID %q does not parse as a member EX address: %v", it.ID, err)
				}
				if it.Display != it.ID {
					t.Errorf("item %q Display = %q, want the printed MENU Number %q — see TestSettingsDescriptor_ItemsAreTheInventoryInOrder on why the coincidence is stated rather than collapsed", it.ID, it.Display, it.ID)
				}
			}
		}
	}
	if len(itemIDs) != count {
		t.Errorf("%d items yielded %d distinct IDs", count, len(itemIDs))
	}
}

// TestSettingsDescriptor_IsADefensiveCopy: each of the three getters must
// hand out an INDEPENDENT tree, and all three must be EQUAL.
//
// The package holds one shared original, and a caller that mutated the value
// it was given would otherwise change what every later caller received —
// including callers in other packages, since app/settings.go and
// internal/wiring both pass these trees straight to view-building code.
//
// Mutation is applied at EVERY level (Version, the Menu's fields, the
// Group's fields, an Item's fields, and the slice headers themselves),
// because a shallow copy would pass a Version-only check while sharing the
// item arrays.
func TestSettingsDescriptor_IsADefensiveCopy(t *testing.T) {
	// A hand-built Session, deliberately: the method is documented as
	// session-INDEPENDENT (it never consults s.dialect), so exercising it on
	// a zero Session is the claim rather than a shortcut — and it spares
	// this test an Open. The live-session leg is in
	// TestSession_ReadSetting_ScriptedRoundTrips, on a session that already
	// exists.
	getters := []struct {
		name string
		get  func() driver.SettingsDescriptor
	}{
		{"package-level SettingsDescriptor()", SettingsDescriptor},
		{"driver StaticSettingsDescriptor()", New(Simulated).(driver.StaticSettingsProvider).StaticSettingsDescriptor},
		{"Session.SettingsDescriptor()", (&Session{}).SettingsDescriptor},
	}

	for _, tt := range getters {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.get(), SettingsDescriptor()) {
				t.Fatalf("%s differs from the package-level tree — this driver's settings surface depends only on the static EX inventory, never on anything a live session discovers", tt.name)
			}

			mine := tt.get()
			mine.Version = "MUTATED"
			mine.Menus[0].ID = "MUTATED"
			mine.Menus[0].Label = "MUTATED"
			mine.Menus[0].Groups[0].ID = "MUTATED"
			mine.Menus[0].Groups[0].Label = "MUTATED"
			mine.Menus[0].Groups[0].Items[0] = driver.SettingItem{ID: "MUTATED", Label: "MUTATED", Display: "MUTATED"}
			mine.Menus = append(mine.Menus, driver.SettingMenu{ID: "EXTRA"})

			if !reflect.DeepEqual(tt.get(), SettingsDescriptor()) {
				t.Errorf("after a caller mutated its own copy, %s no longer matches the package-level tree — the getters must hand out independent trees", tt.name)
			}
		})
	}
}

// exItemByAddr returns the inventory row for a three-digit wire address, or
// fails: a fixture that names an address the inventory does not carry is a
// broken fixture, not a driver defect, and must say so where it is written.
func exItemByAddr(t *testing.T, wire string) cat.EXItem {
	t.Helper()
	for _, it := range catDialect.EXItems() {
		if catDialect.EXWire(it.Addr) == wire {
			return it
		}
	}
	t.Fatalf("no EX inventory row at address %q — this fixture names an address core/cat/ft991a does not carry", wire)
	return cat.EXItem{}
}

// exFullWidthValue builds a P4 body of exactly the width the inventory
// declares for wire, filled with fill. The width is DERIVED, so a fixture
// claiming to answer "at full width" cannot quietly answer at some other
// one — and if the transcribed Digits column ever changes, the fixture
// follows it instead of silently becoming a partial-width case.
func exFullWidthValue(t *testing.T, wire string, fill byte) string {
	t.Helper()
	return strings.Repeat(string(fill), exItemByAddr(t, wire).Digits)
}

// The addresses the scripted round-trips use, one at each end of this
// inventory's width range:
//
//	firstSettingAddr  — 001 AGC FAST DELAY, the chart's very first row
//	                    (layout 531), Digits 4.
//	widestSettingAddr — 151 PRESET FREQUENCY (layout 692), the inventory's
//	                    ONLY Digits-8 row and therefore the single item that
//	                    SETS this dialect's own P4 answer bound
//	                    (cat.maxEXP4Bytes over the inventory, and the reason
//	                    the ft991a extable profile declares MaxDigits 8 where
//	                    the other four declare 4, 4, 4 and 5).
//
// Written as literals rather than picked by index or by scanning for a
// width: a fixture that selected its own subject would keep passing after an
// inventory edit moved the case it was meant to cover, and these are the
// addresses the manual's chart puts at those positions.
const (
	firstSettingAddr  = "001"
	widestSettingAddr = "151"
)

// settingsTestImage is the scripted radio the settings session tests share:
//
//	001 — answered at its own full declared width (4 bytes of '7')
//	151 — the widest item, answered with the 8-byte body its printed range
//	      "00030000 ~ 47000000" implies. No FT-991A has ever answered
//	      anything, so this is the fixture asserting the driver returns
//	      whatever arrives and never reshapes it
//	004 — ABSENT from the map, so answered "?;": the Unavailable path
//	002 — a frame whose prefix matches but which carries NO P4 body at all,
//	      one byte short of the narrowest EX answer this dialect defines
//	003 — a frame whose P4 body is NINE bytes, one past this dialect's own
//	      derived bound of 8: the other malformed edge
//
// One session serves all of them, which is safe because these are
// independent single exchanges with no state on either side.
func settingsTestImage(t *testing.T) slotImage {
	t.Helper()
	return slotImage{exAnswers: map[string]string{
		firstSettingAddr:  "EX" + firstSettingAddr + exFullWidthValue(t, firstSettingAddr, '7') + ";",
		widestSettingAddr: "EX" + widestSettingAddr + "00030000" + ";",
		// 6 bytes: "EX" + the three-digit address + ';' with NO P4 at all.
		// It matches exSpec's prefix, so the engine hands it back as this
		// read's answer and the PARSER is what must refuse it.
		"002": "EX002;",
		// A 9-byte P4, one byte past this dialect's own derived maximum.
		"003": "EX003123456789;",
	}}
}

// TestExSpec_FullAddressPrefixAndVariableLength pins the three decisions in
// exSpec that a plausible-looking alternative would get wrong.
//
// The PREFIX carries the whole three-digit address. EX is a shared-prefix
// family: all 152 addresses answer under the same two command bytes, so a
// bare "EX" would let transport.Engine.Do correlate another address's answer
// as this read's own and report one setting's value under another's ID. The
// test proves two different addresses' specs actually discriminate, which a
// bare-"EX" spec would fail.
//
// The LENGTH is variable (the matcher pins no exact length), the deliberate
// opposite of mtSpec's exact derived length (read.go): this inventory's P4
// widths run 1 to 8 bytes, so there is no single length to pin, and pinning
// a per-item width from the transcribed Digits column would make a spec out
// of a number no FT-991A has ever confirmed — the FT-710's own M8c sweep
// found that column wrong for one of its addresses (core/cat/ex.go's
// ParseEXAnswer records it).
//
// ONE RETRY, where mtSpec has none. The zero on the MT read is plan P11's
// decision about a command whose fire-and-forget siblings this radio shares;
// an EX read is idempotent and its availability row reads `EX | MENU |
// O O O O` (layout 155), so the ordinary "a read is idempotent" reasoning
// every sibling's read spec uses applies here unchanged.
func TestExSpec_FullAddressPrefixAndVariableLength(t *testing.T) {
	a := exItemByAddr(t, firstSettingAddr).Addr
	b := exItemByAddr(t, widestSettingAddr).Addr

	specA := exSpec(catDialect, a)
	if specA.Class != transport.ClassRead {
		t.Errorf("exSpec(%s).Class = %v, want transport.ClassRead", catDialect.EXWire(a), specA.Class)
	}
	if specA.RetryReads != 1 {
		t.Errorf("exSpec.RetryReads = %d, want 1 — an EX read is idempotent and this command's Read carries no contradiction (see this test's doc comment)", specA.RetryReads)
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
	wide := "EX" + catDialect.EXWire(a) + "12345678;"
	if !specA.Match([]byte(wide)) {
		t.Errorf("exSpec(%s).Match(%q) = false, want true — the length must stay variable: this inventory's P4 widths run 1..8 bytes", catDialect.EXWire(a), wide)
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
		wantRaw := exFullWidthValue(t, firstSettingAddr, '7')
		if len(wantRaw) != 4 {
			t.Fatalf("fixture: %s declares Digits %d, want 4 (AGC FAST DELAY) — reconcile the fixture with the inventory", firstSettingAddr, len(wantRaw))
		}

		before := len(p.Transcript())
		got, err := sess.ReadSetting(testCtx(t), firstSettingAddr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v", firstSettingAddr, err)
		}
		want := driver.SettingValue{ID: firstSettingAddr, Raw: wantRaw, State: driver.SettingKnown}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v", firstSettingAddr, got, want)
		}

		// ONE frame, SIX bytes, carrying the full address: this radio's EX
		// read is "EX" + THREE digits + ';' (layout 525), the narrowest in
		// the fleet and the one shared-frame length that moves between this
		// radio and its seven- and nine-byte siblings.
		wantFrame := "EX" + firstSettingAddr + ";"
		if len(wantFrame) != exReadFrameLen {
			t.Fatalf("fixture: the expected read frame %q is %d bytes, want %d", wantFrame, len(wantFrame), exReadFrameLen)
		}
		if sent := p.Transcript()[before:]; len(sent) != 1 || sent[0] != wantFrame {
			t.Errorf("one ReadSetting sent %v, want exactly [%q]", sent, wantFrame)
		}
	})

	t.Run("the widest item answered at its own eight-byte width", func(t *testing.T) {
		got, err := sess.ReadSetting(testCtx(t), widestSettingAddr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v", widestSettingAddr, err)
		}
		const wantRaw = "00030000"
		if len(wantRaw) != exItemByAddr(t, widestSettingAddr).Digits {
			t.Fatalf("fixture: the answer is %d bytes but the inventory declares Digits %d", len(wantRaw), exItemByAddr(t, widestSettingAddr).Digits)
		}
		want := driver.SettingValue{ID: widestSettingAddr, Raw: wantRaw, State: driver.SettingKnown}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v — the P4 body is returned VERBATIM, leading zeros included; the no-trim half of the same rule is in TestParseEXResponse_Table", widestSettingAddr, got, want)
		}
	})

	t.Run("a declared address the radio rejects is Unavailable, not an error", func(t *testing.T) {
		const addr = "004"    // absent from the image, so answered "?;"
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
// 087 IS IN THAT LIST AND IS THE INTERESTING MEMBER. It is a row the chart
// PRINTS, so a user who counts the printed chart will ask for it; it names
// no field, so this dialect excludes it by address (the register's ROW 087
// RADIO ID'S EXCLUSION) and this driver must refuse it with zero frames sent
// rather than build an EX087; whose answer it could not size.
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
			{"right shape, one digit short", "01"},
			{"non-digits in the address field", "0X1"},
			// The SIBLINGS' shapes. This radio's address field is three
			// digits (cat.EXAddressSingle, layout 520), so a caller that
			// carried an FT-891 or FTdx10 setting ID across must be refused
			// rather than have its digits quietly dropped or padded.
			{"the FT-891's four-digit address", "0101"},
			{"the FTdx10's six-digit address", "010101"},
			// The chart's own excluded row: printed, counted, and carrying
			// no parameter at all (layout 623).
			{"row 087, printed by the chart and excluded by address", "087"},
			// One past the chart's own last row, 153 (layout 694).
			{"one past the chart's last row", "154"},
			{"the zero address", "000"},
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
			{"no P4 body at all: shorter than any EX answer", "002"},
			{"a P4 body wider than this dialect's derived bound of 8", "003"},
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
// the complete three-digit address, so transport.Engine.Do can only ever
// hand back a frame that already matches the address requested — a genuinely
// differently-addressed reply fails Do's own matching and is counted as an
// unexpected frame. The branch is defence in depth against that guarantee
// regressing, and a test that could only reach it through a session could
// not test it at all.
func TestParseEXResponse_Table(t *testing.T) {
	requested := exItemByAddr(t, firstSettingAddr).Addr
	other := exItemByAddr(t, widestSettingAddr).Addr

	for _, tt := range []struct {
		name         string
		frame        string
		want         driver.SettingValue
		wantMismatch bool
		wantParseErr bool
	}{
		{
			name:  "a well-formed answer at this item's own width",
			frame: "EX" + firstSettingAddr + "0012" + ";",
			want:  driver.SettingValue{ID: firstSettingAddr, Raw: "0012", State: driver.SettingKnown},
		},
		{
			// NO TRIM, in either direction: a body of spaces is what the
			// radio sent, and this surface reports bytes rather than
			// meanings.
			name:  "a body of spaces is returned verbatim, untrimmed",
			frame: "EX" + firstSettingAddr + "  1 " + ";",
			want:  driver.SettingValue{ID: firstSettingAddr, Raw: "  1 ", State: driver.SettingKnown},
		},
		{
			name:  "the canonical rejection maps to Unavailable with no error",
			frame: "?;",
			want:  driver.SettingValue{ID: firstSettingAddr, State: driver.SettingUnavailable},
		},
		{
			name:         "an answer naming a DIFFERENT address is refused",
			frame:        "EX" + catDialect.EXWire(other) + "1;",
			wantMismatch: true,
		},
		{
			name:         "a frame the parser rejects stays the parser's verdict",
			frame:        "EX" + firstSettingAddr + ";",
			wantParseErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEXResponse(catDialect, requested, []byte(tt.frame))

			if tt.wantMismatch {
				var mm *SettingAnswerMismatchError
				if !errors.As(err, &mm) {
					t.Fatalf("parseEXResponse(%q) error = %v (%T), want *SettingAnswerMismatchError", tt.frame, err, err)
				}
				if mm.Requested != firstSettingAddr || mm.Answered != widestSettingAddr {
					t.Errorf("mismatch = {Requested:%q Answered:%q}, want {%q %q}", mm.Requested, mm.Answered, firstSettingAddr, widestSettingAddr)
				}
				return
			}
			if tt.wantParseErr {
				var pe *cat.ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("parseEXResponse(%q) error = %v (%T), want a wrapped *cat.ParseError", tt.frame, err, err)
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

// TestSession_ReadSetting_TimeoutIsTypedAndRetriedOnce pins the surface's
// third outcome: a radio that answers NOTHING.
//
// The transport's own error survives ReadSetting's wrap — errors.Is finds
// transport.ErrTimeout with no driver type standing between them, the same
// stance read.go takes for an MT timeout (doc.go's ruling).
//
// TWO frames, not one: exSpec's RetryReads is 1, so a timeout retransmits
// exactly once before it is final. Pinning the count is what makes that 1
// non-vacuous; a spec that silently regressed to mtSpec's 0 would still
// satisfy every other assertion here.
func TestSession_ReadSetting_TimeoutIsTypedAndRetriedOnce(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{exSilent: map[string]bool{firstSettingAddr: true}})

	before := len(p.Transcript())
	_, err := sess.ReadSetting(testCtx(t), firstSettingAddr)
	if err == nil {
		t.Fatal("ReadSetting = nil error, want a timeout refusal: the scripted radio answered nothing at all")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("errors.Is(err, transport.ErrTimeout) = false — the transport's own error must survive ReadSetting's wrap: got %v", err)
	}

	wantFrame := "EX" + firstSettingAddr + ";"
	if got, want := p.Transcript()[before:], []string{wantFrame, wantFrame}; !reflect.DeepEqual(got, want) {
		t.Errorf("the timeout path sent %v, want exactly %v — TWO EX frames: the original plus exSpec's one retry (RetryReads: 1)", got, want)
	}
}

// parkFirstSettingsRead arms readSettingGapHook so that the FIRST settings
// read to reach the gap parks there until release is closed, and every later
// one passes straight through. It returns the channel closed when that first
// read has parked.
//
// ONE-SHOT ON PURPOSE. The hook is a package var reached by EVERY
// ReadSetting, so a hook that blocked unconditionally would park the very
// concurrent read the atomicity test is trying to observe — after that read
// had already put its own frame on the wire, which is precisely what the
// test must prove cannot happen.
func parkFirstSettingsRead(t *testing.T, release <-chan struct{}) <-chan struct{} {
	t.Helper()
	reached := make(chan struct{})
	first := make(chan struct{}, 1)
	first <- struct{}{}
	readSettingGapHook = func() {
		select {
		case <-first:
		default:
			return
		}
		close(reached)
		<-release
	}
	t.Cleanup(func() { readSettingGapHook = nil })
	return reached
}

// awaitPark blocks until a settings read has parked in the gap, or fails.
func awaitPark(t *testing.T, reached <-chan struct{}) {
	t.Helper()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("no ReadSetting reached the gap hook within 5s")
	}
}

// TestReadSetting_HoldsOpMuAgainstAConcurrentWrite is the pin write.go's
// operation mutex was waiting for (matrix erratum M-E2, spec erratum S-E4,
// and the task 11 review's MEDIUM-2, which deleted that lock and found the
// whole package still green under -race).
//
// WHY NOTHING COULD PIN IT BEFORE. transport.Engine.Do holds the engine's
// own mutex for its whole body, so every operation this session performs —
// a read, a write, a settings read, retries included — is already atomic AS
// AN EXCHANGE. opMu's claim is a different and larger one: that a whole
// DRIVER OPERATION excludes another, including any work the operation does
// outside its Do call. Nothing in this driver had such work, so the lock was
// asserted in prose and pinned by nothing.
//
// readSettingGapHook creates exactly that window deterministically: a
// settings read that has sent its EX frame and has not yet finished the
// operation. THE WINDOW IS SYNTHETIC AND THE EXCLUSION IS NOT — the hook
// runs under opMu because ReadSetting holds it, so what the concurrent
// WriteChannel is blocked by is the lock and nothing else. Forced through a
// hook rather than by hammering for the reason
// TestReadChannel_ConcurrentReadsDoNotCrossAnswers records: Go's sync.Mutex
// favours an immediately-re-locking goroutine so heavily that the
// interleaving is near-impossible to reproduce by scheduling luck.
//
// RED BY DELETING EITHER LOCK — WriteChannel's or ReadSetting's — which is
// what makes this test the pin for both.
func TestReadSetting_HoldsOpMuAgainstAConcurrentWrite(t *testing.T) {
	p, sess := openSession(t, Simulated, settingsTestImage(t))
	before := len(p.Transcript())

	release := make(chan struct{})
	awaitParkOn := parkFirstSettingsRead(t, release)

	settingDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadSetting(testCtx(t), firstSettingAddr)
		settingDone <- err
	}()
	awaitPark(t, awaitParkOn)

	writeDone := make(chan error, 1)
	go func() {
		_, err := sess.WriteChannel(testCtx(t), writableChannel())
		writeDone <- err
	}()

	// A generous, deterministic window for the write to reach the wire if
	// nothing is holding it back.
	time.Sleep(500 * time.Millisecond)
	wantEX := "EX" + firstSettingAddr + ";"
	if got := p.Transcript()[before:]; len(got) != 1 || got[0] != wantEX {
		t.Errorf("while a settings read was parked inside opMu, the wire carried %v — want only [%q]: a write and a settings read are different DRIVER OPERATIONS and must not interleave their frames", got, wantEX)
	}

	close(release)
	for _, done := range []chan error{settingDone, writeDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("a concurrent operation failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("an operation never completed after the gap hook was released")
		}
	}

	if got, want := p.Transcript()[before:], []string{wantEX, writableChannelFrame}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v — the Set must land only AFTER the settings operation completes", got, want)
	}
}

// TestReadSetting_IsAtomicUnderOpMu is the same pin turned on the settings
// surface itself: a SECOND settings read must not put its EX frame on the
// wire while the first is still inside its operation.
//
// It is the other half the task 11 review asked for, and it fails RED on the
// deletion of ReadSetting's own lock alone — where the write test above
// fails on the deletion of either. Two settings reads are two independent
// exchanges and the engine would happily serialise them one at a time; what
// opMu adds is that the FIRST operation completes before the second begins.
func TestReadSetting_IsAtomicUnderOpMu(t *testing.T) {
	p, sess := openSession(t, Simulated, settingsTestImage(t))
	before := len(p.Transcript())

	release := make(chan struct{})
	awaitParkOn := parkFirstSettingsRead(t, release)

	firstDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadSetting(testCtx(t), firstSettingAddr)
		firstDone <- err
	}()
	awaitPark(t, awaitParkOn)

	secondDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadSetting(testCtx(t), widestSettingAddr)
		secondDone <- err
	}()

	time.Sleep(500 * time.Millisecond)
	wantFirst := "EX" + firstSettingAddr + ";"
	if got := p.Transcript()[before:]; len(got) != 1 || got[0] != wantFirst {
		t.Errorf("while one settings read was parked inside opMu, the wire carried %v — want only [%q]", got, wantFirst)
	}

	close(release)
	for _, done := range []chan error{firstDone, secondDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("a concurrent settings read failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a settings read never completed after the gap hook was released")
		}
	}

	if got, want := p.Transcript()[before:], []string{wantFirst, "EX" + widestSettingAddr + ";"}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v", got, want)
	}
}

// TestCloneReadSettings_WalksTheWholeDescriptor drives core/clone's
// ReadSettings — the layer this descriptor exists for — end to end over the
// scripted radio, and is the only test that puts every one of the
// descriptor's items on the wire.
//
// IT IS THE PREFLIGHT THAT MATTERS MOST, and on this radio more than on any
// sibling. core/clone/settings.go probes an all-MenuUnsupported
// codeplug.MenuSnapshot built from the descriptor's item IDs BEFORE any wire
// exchange, and codeplug.MenuSnapshot.Validate requires every ID to be
// EXACTLY 3, 4 or 6 ASCII digits (core/codeplug/menus.go's
// isSettingIDWidth). THIS IS THE FIRST RADIO IN THE FLEET TO USE THE
// THREE-DIGIT ARM, which exists only because of the Kenwood base dependency
// this milestone consumes — so a descriptor that padded its menu numbers to
// four digits, printing an ID no FT-991A document contains, would still
// pass every assertion in this file and fail HERE, with zero frames sent
// (plan P9). Neither package-level test can see this: the driver's own tests
// validate the descriptor but know nothing of the snapshot rule, and
// core/clone's tests use their own fixtures.
//
// The answers are built from the INVENTORY's declared width per item, not
// from one shared literal, so the walk exercises the full 1..8-byte P4 range
// this dialect derives its answer bound from rather than one width 152
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
	// TestSettingsDescriptor_ItemsAreTheInventoryInOrder.
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

// TestCloneReadSettings_ADriverTimeoutAbortsWithNilSnapshot pins the
// snapshot-level outcome of a genuine ReadSetting failure, read out of
// core/clone/settings.go rather than guessed: on ANY error other than a "?;"
// rejection, ReadSettings aborts the WHOLE call (there is no partial
// snapshot on error — see its own doc comment), wraps the failure as
// "clone: ReadSettings: setting %q: %w" naming the item ID, and returns a
// nil *codeplug.MenuSnapshot.
//
// The silent address is firstSettingAddr, 001 — this descriptor's very first
// item (the chart's own first row) — so the walk aborts on its first
// iteration and this test costs exactly one timed-out exchange, not a walk
// over all 152.
func TestCloneReadSettings_ADriverTimeoutAbortsWithNilSnapshot(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{exSilent: map[string]bool{firstSettingAddr: true}})

	snap, err := clone.NewService(sess, clone.SnapshotStore{}).ReadSettings(testCtx(t))
	if err == nil {
		t.Fatal("clone.ReadSettings = nil error, want the driver's timeout to abort the whole call")
	}
	if snap != nil {
		t.Errorf("clone.ReadSettings snapshot = %+v, want nil — core/clone's own rule is that a partial snapshot is only ever returned on success", snap)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("errors.Is(err, transport.ErrTimeout) = false — core/clone's wrap must not swallow the transport's own sentinel: got %v", err)
	}
	if !strings.Contains(err.Error(), firstSettingAddr) {
		t.Errorf("error text %q does not name the failing item %q — core/clone/settings.go's wrap is \"clone: ReadSettings: setting %%q: %%w\"", err.Error(), firstSettingAddr)
	}
}
