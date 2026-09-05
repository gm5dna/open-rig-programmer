// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// settingsDescriptorVersion is minted HERE — the exact string this driver's
// SettingsDescriptor identifies itself with, and the one
// codeplug.MenuSnapshot.Descriptor carries through verbatim so a snapshot
// can later be checked against the descriptor version that produced it.
//
// A DIFFERENT string from every sibling's, necessarily: the four
// descriptors describe four radios' menus (159 items here, 197 on the
// FTdx10, 296 on the FT-710), so a snapshot taken from one must never
// validate against another. The "@1" is this shape's own generation — a
// later change to how THIS driver builds its tree increments it here alone.
// Pinned by TestSettingsDescriptor_VersionIsThisRadiosOwn.
const settingsDescriptorVersion = "ft891-ex@1"

// ft891SettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries (catDialect.EXItems, generated
// from core/cat/ft891/table2.csv) — see buildSettingsDescriptor.
//
// Every getter — the package-level SettingsDescriptor func, the driver's
// StaticSettingsDescriptor and the session's SettingsDescriptor — returns a
// Clone() of this and never the value itself: nothing outside this file may
// ever hold a reference to the shared original, because a caller that
// mutated the tree it was handed would silently change what every later
// caller received (driver.SettingsDescriptor.Clone's own doc comment).
var ft891SettingsDescriptor = buildSettingsDescriptor(catDialect)

// buildSettingsDescriptor builds the FT-891's driver.SettingsDescriptor
// from dialect.EXItems(): one SettingMenu per distinct two-digit prefix of
// the chart's four-digit MENU Number, ONE SettingGroup inside each carrying
// that menu's own ID, and one SettingItem per inventory row nested under it,
// IN INVENTORY ORDER.
//
// THE SHAPE IS A CHOICE FORCED BY A MANUAL FACT (matrix §3.9). This radio's
// menu chart has NO GROUP LABEL COLUMNS — its columns are
// `P1 | Function | P2 | Digits` (layout 524) where the FT-710's, FTdx10's
// and FTdx101's charts carry label columns — so the registered extable
// profile declares LabelsAbsent and every EXItem's P1Label and P2Label is
// "" (core/cat/ft891/exinventory_gen.go's header says so in terms). A
// driver.SettingsDescriptor needs a non-empty Label at every level and this
// project does not invent group names, so the descriptor falls back to the
// STRUCTURE: the two-digit prefix is both ID and Label at the menu level,
// and the single group repeats it. That group exists because the neutral
// type is a two-level tree and Validate requires at least one group per
// menu, NOT because this radio has a subgroup there — the FTdx10's second
// level is a real (P1,P2) partition off a real label column, and this one is
// not. Pinned by TestSettingsDescriptor_MenuLabelsAreTheStructuralFallback,
// whose empty-label half fails if a later transcription ever gives this
// inventory real names.
//
// SettingItem.Display is the printed MENU Number, which on this radio is the
// SAME four digits as the ID because the chart prints the address as one
// number rather than as the FTdx10's "%02d-%02d-%02d" triple. THAT THE TWO
// COINCIDE IS A FACT ABOUT THIS CHART, NOT A RULE: they are built as two
// separate expressions below, and TestSettingsDescriptor_ItemIDsAreGlobally-
// UniqueFourDigitAddresses states the coincidence, so that nobody later
// "de-duplicates" one into the other and silently makes the ID the display
// form of whatever address shape comes next.
//
// DIALECT-PARAMETERISED THROUGHOUT, which is what lets this be a second
// instance of the sibling template rather than a copy of its output: the
// item count, the menu partition, every label and the address width come
// from the dialect argument. Nothing about the FT-891's own numbers (159
// items, eighteen prefixes 01..18) is written into this function — they are
// properties of the inventory it is handed, asserted in settings_test.go
// against the dialect and the registered profile rather than against
// literals here.
//
// Dialect.EXItems returns its rows already sorted by (P1,P2) (core/cat/
// ft891's generated inventory is emitted in that order and its own header
// says so), so a single linear pass — opening a new menu only when the
// running prefix changes from the PREVIOUS item — reproduces the chart's own
// ordering with no extra sorting or map bookkeeping. Each menu pointer used
// to append is re-derived fresh, by index, on every iteration and never
// carried across one: that sidesteps any question of whether an earlier
// append's slice growth could invalidate a held pointer, at the cost of one
// slice index per item — a non-issue at 159 items, run once at package init.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an
// address, a name and a display form per item; it does not carry an item's
// value legend, its units, its enumerated options or its default, and
// ReadSetting below returns the P4 body verbatim. That is why
// core/cat/ft891/doc.go's recorded CHART PRINTING DEFECTS do not bite this
// surface: every one of them lives in a value legend, and this driver
// interprets no legend (matrix §3.9). They become questions the moment a
// caller tries to render a menu value as a MEANING rather than as the bytes
// the radio sent, and that is deliberately not this file's business.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	items := dialect.EXItems()

	d := driver.SettingsDescriptor{Version: settingsDescriptorVersion}

	for _, it := range items {
		menuID := fmt.Sprintf("%02d", it.Addr.P1)

		if len(d.Menus) == 0 || d.Menus[len(d.Menus)-1].ID != menuID {
			d.Menus = append(d.Menus, driver.SettingMenu{
				ID:    menuID,
				Label: menuID,
				Groups: []driver.SettingGroup{
					{ID: menuID, Label: menuID},
				},
			})
		}
		group := &d.Menus[len(d.Menus)-1].Groups[0]

		group.Items = append(group.Items, driver.SettingItem{
			ID:      dialect.EXWire(it.Addr),
			Label:   it.Name,
			Display: dialect.EXWire(it.Addr),
		})
	}

	return d
}

// SettingsDescriptor returns the FT-891's radio-neutral settings
// descriptor: a defensive Clone() of the package-level tree
// buildSettingsDescriptor built once at package init. Every call returns an
// independent copy — see driver.SettingsDescriptor.Clone's doc comment for
// why that independence is load-bearing.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ft891SettingsDescriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to
// Session.SettingsDescriptor. Identical to both it and the package-level
// func: this driver's settings tree depends only on the static EX
// inventory, never on anything a live session discovers (unlike
// Session.Capabilities, which folds in per-session 5xx/EMG discovery), so
// all three call sites return equal trees.
//
// It lives in THIS file rather than beside the driver's other methods in
// ft891.go because the settings surface is one subject and reads better
// whole: the method is one line of delegation to the descriptor built above
// it, and splitting it across files would put the capability's driver half
// out of sight of its session half for no gain.
func (d *ft891Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements half of the optional driver.SettingsReader
// capability (see that interface's doc comment) on the concrete *Session.
// Identical to the package-level SettingsDescriptor func and to the
// driver's StaticSettingsDescriptor, for the reason stated there.
//
// It deliberately does NOT consult s.dialect, even though the session
// carries one: the package-level tree is built from catDialect, the single
// dialect every session of this driver is opened with (caps.go), and a
// per-session rebuild would cost 159 items of allocation per call to produce
// the identical answer. A future FT-891 session whose menu surface genuinely
// varied — discovered, not declared — would rebuild here, and
// driver.StaticSettingsProvider's own contract already allows the two to
// disagree in that case.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}
