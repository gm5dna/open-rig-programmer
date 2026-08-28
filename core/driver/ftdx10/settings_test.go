// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Compile-time proof of the two settings seams this task adds. Any
// signature drift fails the BUILD, not merely a test.
//
// BOTH halves are asserted, deliberately. core/driver/ft710 asserts only the
// driver-level StaticSettingsProvider (its optional_test.go) and leaves
// SettingsReader to a doc comment — an omission, not a pattern: app/
// settings.go reaches the session capability by a two-result type assertion
// (conn.session.(driver.SettingsReader)), which returns false rather than
// failing when a method's shape drifts, so the app would silently fall back
// to the static descriptor and serve every ReadSetting as unsupported. The
// assertion is the only thing that turns that into a loud failure.
//
// They live HERE rather than in this package's optional_test.go, where the
// package's other seam assertions sit, because this task's whole surface is
// one subject in one pair of files (see StaticSettingsDescriptor's own doc
// comment on the same choice for the method).
var (
	_ driver.StaticSettingsProvider = (*ftdx10Driver)(nil)
	_ driver.SettingsReader         = (*Session)(nil)
)

// exItemByAddr returns the inventory row for a six-digit wire address, or
// fails: a fixture that names an address the inventory does not carry is a
// broken fixture, not a driver defect, and must say so where it is written.
func exItemByAddr(t *testing.T, wire string) cat.EXItem {
	t.Helper()
	for _, it := range catDialect.EXItems() {
		if it.Addr.Wire() == wire {
			return it
		}
	}
	t.Fatalf("no EX inventory row at address %q — this fixture names an address core/cat/ftdx10 does not carry", wire)
	return cat.EXItem{}
}

// exFullWidthValue builds a P4 body of exactly the width the inventory
// declares for wire, filled with fill. The width is DERIVED, so a fixture
// claiming to answer "at full width" cannot quietly answer at some other
// one — and if the transcribed Digits column ever changes, the fixture
// follows it instead of silently becoming a partial-width case.
func exFullWidthValue(t *testing.T, wire string, fill byte) string {
	t.Helper()
	it := exItemByAddr(t, wire)
	b := make([]byte, it.Digits)
	for i := range b {
		b[i] = fill
	}
	return string(b)
}

// The two addresses the scripted round-trips use, one of each shape this
// inventory has:
//
//	numericSettingAddr — (01,01,01) AF TREBLE GAIN, a 3-digit numeric item
//	                     and the inventory's very first row.
//	textSettingAddr    — (04,01,01) MY CALL., the inventory's ONLY Text
//	                     item, 12 bytes (pinned by
//	                     TestSettingsDescriptor_TheOneTextItem).
//
// Written as literals rather than picked by index or by scanning for a Text
// flag: a fixture that selected its own subject would keep passing after an
// inventory edit moved the case it was meant to cover, and these two
// addresses are the ones the manual's chart puts at those positions.
const (
	numericSettingAddr = "010101"
	textSettingAddr    = "040101"
)

// settingsTestImage is the scripted radio the settings session tests share:
//
//	010101 — answered at its own full numeric width (3 bytes of '7')
//	040101 — the Text item, answered with a 12-byte SPACE-PADDED value, the
//	         shape the FT-710's own text items were observed to answer with
//	         (core/cat/ex.go's ParseEXAnswer notes the hardware evidence);
//	         no FTdx10 has ever answered anything, so this is the fixture
//	         asserting the driver returns whatever arrives, padding included
//	010102 — ABSENT from the map, so answered "?;": the Unavailable path
//	010103 — a frame whose prefix matches but whose body is TOO SHORT for
//	         any EX answer: the malformed-frame path
//	010104 — a frame whose P4 body is WIDER than this dialect's own derived
//	         bound (12, from its single Text item): the other malformed edge
//
// One session serves all of them — each Open costs ~2.8 s (ftdx10_test.go)
// — which is safe because these are independent single exchanges with no
// state on either side.
func settingsTestImage(t *testing.T) slotImage {
	t.Helper()
	return slotImage{exAnswers: map[string]string{
		numericSettingAddr: "EX" + numericSettingAddr + exFullWidthValue(t, numericSettingAddr, '7') + ";",
		textSettingAddr:    "EX" + textSettingAddr + "GM5DNA      " + ";",
		// 9 bytes: "EX" + address + ';' with NO P4 at all, one byte short of
		// the narrowest EX answer the grammar block defines. It matches
		// exSpec's ExpectPrefix, so the engine hands it back as this read's
		// answer and the PARSER is what must refuse it.
		"010103": "EX010103;",
		// A 13-byte P4, one byte past this dialect's own derived maximum.
		"010104": "EX0101040123456789012;",
	}}
}

// TestSettingsDescriptor_ShapeFromTheInventory pins the descriptor against
// the inventory it is derived from, structurally and by an INDEPENDENT
// derivation.
//
// buildSettingsDescriptor walks the (already sorted) inventory once,
// opening a menu or group when the running P1/P2 changes from the previous
// row. This test rebuilds the expected partition a different way — map-keyed
// accumulation, then an explicit sort of the keys — so agreement is evidence
// about the data rather than the same algorithm compared with itself. A
// non-contiguous P1 or P2 run in the inventory would split into two menus
// or groups under the linear pass and would show up here as an extra menu or
// group with a duplicate ID (and would additionally fail Validate).
func TestSettingsDescriptor_ShapeFromTheInventory(t *testing.T) {
	d := SettingsDescriptor()

	if err := d.Validate(); err != nil {
		t.Fatalf("SettingsDescriptor().Validate() = %v, want nil", err)
	}
	if d.Version != "ftdx10-ex@1" {
		t.Errorf("Version = %q, want \"ftdx10-ex@1\" (minted in this package; codeplug.MenuSnapshot.Descriptor carries it verbatim)", d.Version)
	}
	if d.Version == "ft710-ex@1" {
		t.Error("Version equals the FT-710's — two radios' menu trees must never share a descriptor version, or a snapshot from one would validate against the other")
	}

	items := catDialect.EXItems()

	// The count comes from the dialect; the literal is a STALENESS check on
	// this test's own assumption, not the assertion itself.
	wantItems := len(items)
	if wantItems != 197 {
		t.Fatalf("catDialect.EXItems() = %d items, want 197 (this test's own assumption is stale — reconcile with core/cat/ftdx10 before editing the assertion below)", wantItems)
	}

	// Independent derivation of the expected partition.
	menuLabel := map[string]string{}
	groupLabel := map[string]string{}
	menuGroupIDs := map[string][]string{}
	groupItems := map[string][]driver.SettingItem{}
	var menuIDs []string
	for _, it := range items {
		menuID := fmt.Sprintf("%02d", it.Addr.P1)
		groupID := fmt.Sprintf("%02d%02d", it.Addr.P1, it.Addr.P2)

		if _, seen := menuLabel[menuID]; !seen {
			menuLabel[menuID] = it.P1Label
			menuIDs = append(menuIDs, menuID)
		} else if menuLabel[menuID] != it.P1Label {
			// The linear pass takes the FIRST row's label for a menu. That is
			// only lossless while every row of a P1 agrees about it.
			t.Errorf("inventory row %s carries P1Label %q but menu %q was opened with %q — the descriptor's menu label would depend on row order", it.Addr.Wire(), it.P1Label, menuID, menuLabel[menuID])
		}
		if _, seen := groupLabel[groupID]; !seen {
			groupLabel[groupID] = it.P2Label
			menuGroupIDs[menuID] = append(menuGroupIDs[menuID], groupID)
		} else if groupLabel[groupID] != it.P2Label {
			t.Errorf("inventory row %s carries P2Label %q but group %q was opened with %q — the descriptor's group label would depend on row order", it.Addr.Wire(), it.P2Label, groupID, groupLabel[groupID])
		}
		groupItems[groupID] = append(groupItems[groupID], driver.SettingItem{
			ID:      it.Addr.Wire(),
			Label:   it.Name,
			Display: fmt.Sprintf("%02d-%02d-%02d", it.Addr.P1, it.Addr.P2, it.Addr.P3),
		})
	}
	sort.Strings(menuIDs)
	for id := range menuGroupIDs {
		sort.Strings(menuGroupIDs[id])
	}

	if len(menuIDs) != 4 {
		t.Errorf("the inventory has %d distinct P1 values, want 4 (01 RADIO SETTING, 02 CW SETTING, 03 OPERATION SETTING, 04 DISPLAY SETTING — no P1=05 group; core/cat/ftdx10/doc.go's header-vs-chart anomaly)", len(menuIDs))
	}
	if len(groupLabel) != 18 {
		t.Errorf("the inventory has %d distinct (P1,P2) pairs, want 18", len(groupLabel))
	}

	if len(d.Menus) != len(menuIDs) {
		t.Fatalf("descriptor has %d menus, the inventory has %d distinct P1 values", len(d.Menus), len(menuIDs))
	}
	var gotItems, gotGroups int
	for i, m := range d.Menus {
		if m.ID != menuIDs[i] {
			t.Errorf("Menus[%d].ID = %q, want %q (menus in ascending P1 order)", i, m.ID, menuIDs[i])
			continue
		}
		if m.Label != menuLabel[m.ID] {
			t.Errorf("menu %q Label = %q, want the manual's P1 column %q", m.ID, m.Label, menuLabel[m.ID])
		}
		wantGroups := menuGroupIDs[m.ID]
		if len(m.Groups) != len(wantGroups) {
			t.Errorf("menu %q has %d groups, want %d", m.ID, len(m.Groups), len(wantGroups))
			continue
		}
		gotGroups += len(m.Groups)
		for j, g := range m.Groups {
			if g.ID != wantGroups[j] {
				t.Errorf("menu %q Groups[%d].ID = %q, want %q (groups in ascending P2 order)", m.ID, j, g.ID, wantGroups[j])
				continue
			}
			if g.Label != groupLabel[g.ID] {
				t.Errorf("group %q Label = %q, want the manual's P2 column %q", g.ID, g.Label, groupLabel[g.ID])
			}
			if !reflect.DeepEqual(g.Items, groupItems[g.ID]) {
				t.Errorf("group %q Items =\n %#v\nwant (derived independently from the inventory)\n %#v", g.ID, g.Items, groupItems[g.ID])
			}
			gotItems += len(g.Items)
		}
	}

	if gotItems != wantItems {
		t.Errorf("total descriptor items = %d, want %d (== len(catDialect.EXItems()))", gotItems, wantItems)
	}
	if gotGroups != len(groupLabel) {
		t.Errorf("total descriptor groups = %d, want %d", gotGroups, len(groupLabel))
	}
}

// TestSettingsDescriptor_ItemIDsAreGloballyUniqueSixDigitAddresses: a caller
// addressing ReadSetting by ID has no group context to disambiguate a
// collision, so uniqueness must hold across the WHOLE tree and not merely
// within a group. Validate enforces it too; this asserts it independently
// and adds the ID/Display shape, which Validate has no opinion about.
func TestSettingsDescriptor_ItemIDsAreGloballyUniqueSixDigitAddresses(t *testing.T) {
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

				if len(it.ID) != 6 {
					t.Errorf("item ID %q has length %d, want 6 (the EX wire address)", it.ID, len(it.ID))
				}
				// The ID must round-trip through the dialect's own parser:
				// this is the exact string ReadSetting will be handed back,
				// so an ID the dialect cannot parse would be a setting the
				// descriptor advertises and the driver refuses.
				if _, err := catDialect.ParseEXAddress(it.ID); err != nil {
					t.Errorf("item ID %q does not parse as a member EX address: %v", it.ID, err)
				}
				if want := it.ID[0:2] + "-" + it.ID[2:4] + "-" + it.ID[4:6]; it.Display != want {
					t.Errorf("item %q Display = %q, want %q", it.ID, it.Display, want)
				}
			}
		}
	}
	if len(seen) != count {
		t.Errorf("%d items yielded %d distinct IDs", count, len(seen))
	}
}

// TestSettingsDescriptor_TheOneTextItem pins the inventory's single Text
// item and the reason it poses no shape problem for this surface.
//
// The FTdx10's inventory has exactly ONE 12-byte free-text item, MY CALL. at
// (04,01,01), against the FT-710's six. The descriptor carries no width, no
// shape and no text flag for ANY item — driver.SettingItem is three strings
// — so the Text item's entry is structurally identical to a one-digit
// numeric item's, and there is no per-shape branch anywhere for a single
// outlier to fall through. That structural claim is what the reflect check
// below asserts; it is the reason this task needed no Text special case at
// all, and it would stop being true the moment a width or shape field was
// added to the neutral type without a decision about text items.
func TestSettingsDescriptor_TheOneTextItem(t *testing.T) {
	var text []cat.EXItem
	for _, it := range catDialect.EXItems() {
		if it.Text {
			text = append(text, it)
		}
	}
	if len(text) != 1 {
		t.Fatalf("the inventory carries %d Text items, want exactly 1 (%v)", len(text), text)
	}
	got := text[0]
	if got.Addr.Wire() != textSettingAddr {
		t.Errorf("the Text item is at %q, want %q", got.Addr.Wire(), textSettingAddr)
	}
	if got.Digits != 12 {
		t.Errorf("the Text item declares Digits %d, want 12 (\"Up to 12 characters\")", got.Digits)
	}

	// Its descriptor entry: present exactly once, with the same three
	// populated fields every other item has.
	var found *driver.SettingItem
	var inGroup string
	d := SettingsDescriptor()
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for i := range g.Items {
				if g.Items[i].ID == textSettingAddr {
					if found != nil {
						t.Fatalf("the Text item's ID appears twice (groups %q and %q)", inGroup, g.ID)
					}
					found = &g.Items[i]
					inGroup = g.ID
				}
			}
		}
	}
	if found == nil {
		t.Fatalf("no descriptor item has ID %q — the Text item is missing from the tree", textSettingAddr)
	}
	want := driver.SettingItem{ID: textSettingAddr, Label: got.Name, Display: "04-01-01"}
	if *found != want {
		t.Errorf("the Text item's descriptor entry = %+v, want %+v", *found, want)
	}
	if inGroup != textSettingAddr[0:4] {
		t.Errorf("the Text item sits in group %q, want %q", inGroup, textSettingAddr[0:4])
	}

	// The shape-invisibility claim, structurally.
	st := reflect.TypeOf(driver.SettingItem{})
	if st.NumField() != 3 {
		t.Errorf("driver.SettingItem has %d fields, want 3 (ID, Label, Display) — a width/shape/text field would need a decision about this inventory's one 12-byte Text item, and this driver's descriptor makes none", st.NumField())
	}
	for i := 0; i < st.NumField(); i++ {
		if st.Field(i).Type.Kind() != reflect.String {
			t.Errorf("driver.SettingItem.%s is a %s, want a string — see this test's doc comment", st.Field(i).Name, st.Field(i).Type.Kind())
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
		// session-INDEPENDENT (it never consults s.dialect), so exercising it
		// on a zero Session is the claim rather than a shortcut — and it
		// spares this test one ~2.8 s Open. The live-session leg is in
		// TestSession_ReadSetting_ScriptedRoundTrips, on a session that
		// already exists.
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

// TestStaticSettingsDescriptor_MatchesPackageFuncAndSession: the driver-level
// optional capability, the package-level func and the session method must
// return EQUAL trees. This driver's settings surface depends only on the
// static EX inventory, never on anything Open discovers (unlike
// Session.Capabilities, which folds in per-session 5xx/EMG discovery), so all
// three agree by construction — and internal/wiring's
// StaticSettingsDescriptor serves the offline path from the driver one while
// app/settings.go prefers the session one, so a disagreement would show a
// user two different menu trees for the same radio.
func TestStaticSettingsDescriptor_MatchesPackageFuncAndSession(t *testing.T) {
	drv := New(Simulated)
	provider, ok := drv.(driver.StaticSettingsProvider)
	if !ok {
		t.Fatal("New(Simulated) does not implement driver.StaticSettingsProvider — internal/wiring.StaticSettingsDescriptor would report the FTdx10 as having no settings surface at all, with no error anywhere")
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

// TestExSpec_FullAddressPrefixAndVariableLength pins the two decisions in
// exSpec that a plausible-looking alternative would get wrong.
//
// The PREFIX carries the whole six-digit address. EX is a shared-prefix
// family: 197 addresses answer under the same two command bytes, so a bare
// "EX" would let transport.Engine.Do correlate another address's answer as
// this read's own and report one setting's value under another's ID. The
// test proves the prefixes of two different addresses actually differ, which
// a bare-"EX" spec would fail.
//
// The LENGTH is variable (the matcher pins no exact length), the
// deliberate opposite of mtSpec's exact derived length: this inventory's P4 widths run 1 to 12
// bytes, so there is no single length to pin, and pinning a per-item width
// from the transcribed Digits column would make a spec out of a number no
// FTdx10 has ever confirmed.
func TestExSpec_FullAddressPrefixAndVariableLength(t *testing.T) {
	a := exItemByAddr(t, numericSettingAddr).Addr
	b := exItemByAddr(t, textSettingAddr).Addr

	specA := exSpec(a)
	if specA.Class != transport.ClassRead {
		t.Errorf("exSpec(%s).Class = %v, want transport.ClassRead", a.Wire(), specA.Class)
	}
	if specA.RetryReads != 1 {
		t.Errorf("exSpec.RetryReads = %d, want 1 (an EX read is idempotent)", specA.RetryReads)
	}
	// The full-address prefix and the variable length, asserted THROUGH
	// THE MATCHER rather than off the struct: D2 moved answer matching
	// into an opaque transport.CommandSpec.Match built by the codec, so
	// ExpectPrefix and ExpectLen no longer exist as fields. The property
	// they pinned is the one that matters and is stronger stated this way
	// — a's spec accepts a's own answer and REFUSES b's, which is exactly
	// the wrong-address correlation a bare "EX" prefix would permit.
	ownAnswer := "EX" + a.Wire() + "1;"
	othersAnswer := "EX" + b.Wire() + "1;"
	if !specA.Match([]byte(ownAnswer)) {
		t.Errorf("exSpec(%s).Match(%q) = false, want true — that is this address's own answer", a.Wire(), ownAnswer)
	}
	if specA.Match([]byte(othersAnswer)) {
		t.Errorf("exSpec(%s).Match(%q) = true, want false — a shared prefix is exactly what lets one address's answer be correlated as another's", a.Wire(), othersAnswer)
	}
	// Variable length: two answers of different widths, both this
	// address's, both matched.
	wide := "EX" + a.Wire() + "123456789012;"
	if !specA.Match([]byte(wide)) {
		t.Errorf("exSpec(%s).Match(%q) = false, want true — the length must stay variable: this inventory's P4 widths run 1..12 bytes", a.Wire(), wide)
	}
}

// TestSession_ReadSetting_ScriptedRoundTrips drives ReadSetting through the
// real transport.Engine against the scripted radio, over the three outcomes
// a real exchange can produce.
//
// Raw values are compared VERBATIM, padding and all: this surface reports
// what the radio sent and interprets nothing (see buildSettingsDescriptor on
// why — the dialect's four recorded chart printing defects all live in value
// legends this driver never reads).
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

	t.Run("a numeric item answered at its full declared width", func(t *testing.T) {
		wantRaw := exFullWidthValue(t, numericSettingAddr, '7')
		if len(wantRaw) != 3 {
			t.Fatalf("fixture: %s declares Digits %d, want 3 (AF TREBLE GAIN) — reconcile the fixture with the inventory", numericSettingAddr, len(wantRaw))
		}

		before := len(p.Transcript())
		got, err := sess.ReadSetting(testCtx(t), numericSettingAddr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v", numericSettingAddr, err)
		}
		want := driver.SettingValue{ID: numericSettingAddr, Raw: wantRaw, State: driver.SettingKnown}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v", numericSettingAddr, got, want)
		}

		// ONE frame, carrying the full address: the read is a single
		// exchange, which is why this Session needs no operation mutex.
		if sent := p.Transcript()[before:]; len(sent) != 1 || sent[0] != "EX"+numericSettingAddr+";" {
			t.Errorf("one ReadSetting sent %v, want exactly [\"EX%s;\"]", sent, numericSettingAddr)
		}
	})

	t.Run("the Text item answered with a 12-byte space-padded value", func(t *testing.T) {
		got, err := sess.ReadSetting(testCtx(t), textSettingAddr)
		if err != nil {
			t.Fatalf("ReadSetting(%q): unexpected error: %v", textSettingAddr, err)
		}
		const wantRaw = "GM5DNA      "
		if len(wantRaw) != exItemByAddr(t, textSettingAddr).Digits {
			t.Fatalf("fixture: the answer is %d bytes but the inventory declares Digits %d", len(wantRaw), exItemByAddr(t, textSettingAddr).Digits)
		}
		want := driver.SettingValue{ID: textSettingAddr, Raw: wantRaw, State: driver.SettingKnown}
		if got != want {
			t.Errorf("ReadSetting(%q) = %+v, want %+v — the P4 body is returned VERBATIM, trailing padding included (no trim, no typed value model)", textSettingAddr, got, want)
		}
	})

	t.Run("a declared address the radio rejects is Unavailable, not an error", func(t *testing.T) {
		const addr = "010102" // absent from the image, so answered "?;"
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
			{"right shape, wrong width", "01010"},
			{"non-digits in the address field", "0101X1"},
			// The EX grammar block names "P1 : 01 - 05" and Table 2 populates
			// P1 01-04 only — core/cat/ftdx10/doc.go's header-vs-chart
			// anomaly, recorded UNRESOLVED because this radio cannot be asked.
			// Membership follows the chart, so every (05,*,*) triple is
			// refused here rather than probed.
			{"the phantom P1=05 group the grammar block names", "050101"},
			// P3 tops out at 21 across this whole chart.
			{"a P3 beyond the chart's own ceiling", "010199"},
			{"the zero address", "000000"},
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
			{"no P4 body at all: shorter than any EX answer", "010103"},
			{"a P4 body wider than this dialect's derived bound", "010104"},
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
// design rather than a testing convenience: exSpec's ExpectPrefix carries
// the complete six-digit address, so transport.Engine.Do can only ever hand
// back a frame that already matches the address requested — a genuinely
// differently-addressed reply fails Do's own matching and is counted as an
// unexpected frame. The branch is defence in depth against that guarantee
// regressing, and a test that could only reach it through a session could
// not test it at all.
func TestParseEXResponse_Table(t *testing.T) {
	requested := exItemByAddr(t, numericSettingAddr).Addr
	other := exItemByAddr(t, textSettingAddr).Addr

	for _, tt := range []struct {
		name         string
		frame        string
		want         driver.SettingValue
		wantErr      bool
		wantMismatch bool
	}{
		{
			name:  "a well-formed answer for the requested address",
			frame: "EX" + requested.Wire() + "123;",
			want:  driver.SettingValue{ID: requested.Wire(), Raw: "123", State: driver.SettingKnown},
		},
		{
			name:  "the rejection frame",
			frame: "?;",
			want:  driver.SettingValue{ID: requested.Wire(), State: driver.SettingUnavailable},
		},
		{
			name:         "a well-formed answer for a DIFFERENT known address",
			frame:        "EX" + other.Wire() + "5;",
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
			frame:   "EX0501015;",
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
					if mismatch.Requested != requested.Wire() || mismatch.Answered != other.Wire() {
						t.Errorf("mismatch = %+v, want Requested=%q Answered=%q", mismatch, requested.Wire(), other.Wire())
					}
					// It must NOT be mistakable for the slot-worded error
					// this package already has: two different wire-address
					// namespaces, two types.
					var slotMismatch *AnswerMismatchError
					if errors.As(err, &slotMismatch) {
						t.Error("an EX address mismatch also matched *AnswerMismatchError — the slot-worded type must not stand in for the setting one")
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
