// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// GroupNested is Params.Group's nil default: one menu per P1 carrying the
// manual's P1 column as its label, one group per (P1,P2) carrying its P2
// column. IDs are the decimal digits, "01" and "0101".
func GroupNested(it cat.EXItem) (menuID, menuLabel, groupID, groupLabel string) {
	return fmt.Sprintf("%02d", it.Addr.P1), it.P1Label,
		fmt.Sprintf("%02d%02d", it.Addr.P1, it.Addr.P2), it.P2Label
}

// GroupByP1 is the grouping for a radio whose manual charts its menu by P1
// alone: one menu per P1 holding exactly one group, both identified and
// labelled by the P1 digits. The single group falls out of the shared
// build loop rather than needing its own branch, because the group ID
// changes exactly when the menu ID does.
func GroupByP1(it cat.EXItem) (menuID, menuLabel, groupID, groupLabel string) {
	id := fmt.Sprintf("%02d", it.Addr.P1)
	return id, id, id, id
}

// FlatMenuID is the single menu and group identifier GroupFlat mints.
const FlatMenuID = "MENU"

// GroupFlat is the grouping for a radio whose menu has no chart hierarchy
// at all: every item lands in one menu holding one group.
func GroupFlat(cat.EXItem) (menuID, menuLabel, groupID, groupLabel string) {
	return FlatMenuID, FlatMenuID, FlatMenuID, FlatMenuID
}

// DisplayP1P2P3 is the human "P1-P2-P3" rendering, e.g. "01-01-01".
func DisplayP1P2P3(a cat.EXAddress) string {
	return fmt.Sprintf("%02d-%02d-%02d", a.P1, a.P2, a.P3)
}

// BuildDescriptor builds a radio-neutral driver.SettingsDescriptor from
// dialect's EX inventory: one SettingItem per inventory row, IN INVENTORY
// ORDER, its ID the item's EX wire address, its Label the manual's
// Function name, its Display p.Display's rendering; nested under the menus
// and groups p.Group partitions the inventory into.
//
// DIALECT-PARAMETERISED THROUGHOUT — the item count, the partition, every
// label and every width come from the dialect argument, never from a
// radio's numbers written out here — which is what lets one body serve
// five inventories. dialect is an argument rather than p.Dialect so a test
// can build a descriptor from a synthetic inventory.
//
// Dialect.EXItems returns its rows already sorted by (P1,P2,P3), so a
// single linear pass — opening a new menu or group only when the running
// ID changes from the PREVIOUS item — reproduces the manual's own chart
// grouping with no sorting or map bookkeeping. Each menu/group pointer is
// re-derived by index on every iteration and never carried across one, so
// no append's slice growth can invalidate a held pointer.
//
// RAW ADDRESSES AND LABELS ONLY, NO VALUE SEMANTICS. The tree carries an
// address, two labels and a display form per item; it carries no value
// legend, no units, no enumerated options and no default, and ReadSetting
// returns the P4 body verbatim. That is why the printing defects the
// per-radio doc.go files record in their manuals' value legends do not
// bite this surface: nothing here interprets a legend.
func BuildDescriptor(dialect cat.Dialect, p *Params) driver.SettingsDescriptor {
	group := p.Group
	if group == nil {
		group = GroupNested
	}
	display := p.Display
	if display == nil {
		display = dialect.EXWire
	}

	d := driver.SettingsDescriptor{Version: p.DescriptorVersion}

	for _, it := range dialect.EXItems() {
		menuID, menuLabel, groupID, groupLabel := group(it)

		if len(d.Menus) == 0 || d.Menus[len(d.Menus)-1].ID != menuID {
			d.Menus = append(d.Menus, driver.SettingMenu{ID: menuID, Label: menuLabel})
		}
		menu := &d.Menus[len(d.Menus)-1]

		if len(menu.Groups) == 0 || menu.Groups[len(menu.Groups)-1].ID != groupID {
			menu.Groups = append(menu.Groups, driver.SettingGroup{ID: groupID, Label: groupLabel})
		}
		g := &menu.Groups[len(menu.Groups)-1]

		g.Items = append(g.Items, driver.SettingItem{
			ID:      dialect.EXWire(it.Addr),
			Label:   it.Name,
			Display: display(it.Addr),
			Write:   writeSupport(dialect, it.Addr),
		})
	}

	return d
}

// writeSupport mints a SettingItem's Write field from dialect's own write
// descriptor (M8: minted HERE, once at init, shared by all five Yaesu
// drivers — not a per-driver wrapper's job). cat.Dialect.CanSetEX is
// dialect data, nil-map false for every dialect but the FT-710 today
// (core/cat/dialect.go's buildFT710ExWrite), so this is benign for the
// other four: every one of their items mints Unverified, exercised by
// TestYaesuBuildDescriptor_OtherFourDialectsMintUnverified. It mints
// nothing above Unverified until a dialect's own write descriptor has a
// non-zero ObservedSetWidth row for the address — never guessed, never a
// dialect == FT710 literal.
func writeSupport(dialect cat.Dialect, addr cat.EXAddress) spec.Support {
	if dialect.CanSetEX(addr) {
		return spec.Supported
	}
	return spec.Unverified
}

// EXSpec is the transport spec for an EX read of addr.
//
// THE MATCH PREFIX CARRIES THE FULL WIRE ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule: EX shares its two-byte
// command prefix across every address in a dialect's inventory, so a bare
// "EX" would let Engine.Do correlate a DIFFERENT address's still-in-flight
// answer as this read's own and hand back one setting's value labelled as
// another's.
//
// The exact length is left 0 — VARIABLE LENGTH. There is no single EX
// answer length to derive: the P4 body's width runs 1 to 12 bytes across
// these inventories, so only the prefix is checked and
// cat.Dialect.ParseEXAnswer applies the dialect's own bound afterwards.
// Deriving a per-item exact length from the manual's Digits column would
// be worse than useless — the FT-710's M8c sweep found that column WRONG
// for one of its own addresses — and a spec pinning an unobserved width
// would turn the radio's honest answer into a timeout.
//
// One retry: an EX read is idempotent.
func EXSpec(dialect cat.Dialect, addr cat.EXAddress) transport.CommandSpec {
	return transport.CATReadSpec("EX"+dialect.EXWire(addr), 0, 1)
}

// ParseEXResponse interprets the outcome of one EX exchange for requested:
//
//   - a rejection frame (cat.IsRejection) maps to SettingUnavailable with
//     NO error — the project's established "?;" -> empty-result rule;
//   - a well-formed answer naming requested's own address maps to
//     SettingKnown, Raw the P4 body VERBATIM;
//   - a well-formed answer naming a DIFFERENT address is refused with
//     *driver.SettingAnswerMismatchError;
//   - anything else is the parser's typed *cat.ParseError under a wrap
//     adding the address, so errors.As still finds it.
//
// A PURE function — no ctx, no session, no wire I/O — separated from
// ReadSetting's exchange so it can be unit-tested with hand-built frames.
// That matters most for the WRONG-ADDRESS branch: EXSpec's match prefix
// carries the complete wire address, so Engine.Do can only ever return a
// frame that ALREADY matches it, and a genuinely differently-addressed
// reply is counted as an unexpected frame instead of reaching here. The
// branch is therefore unreachable through a real ReadSetting, and calling
// this directly is the only way to prove that defence in depth still
// works. ReadSetting reaches every other branch for real, a rejection
// included: it reconstructs the literal "?;" bytes from Engine.Do's
// sentinel and hands them here, so response interpretation lives in
// exactly one place.
func ParseEXResponse(dialect cat.Dialect, p *Params, requested cat.EXAddress, frame []byte) (driver.SettingValue, error) {
	id := dialect.EXWire(requested)

	if cat.IsRejection(frame) {
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	}

	addr, raw, err := dialect.ParseEXAnswer(frame)
	if err != nil {
		return driver.SettingValue{}, fmt.Errorf("%s: ReadSetting %s: %w", p.Name, id, err)
	}
	if answered := dialect.EXWire(addr); answered != id {
		return driver.SettingValue{}, &driver.SettingAnswerMismatchError{Model: p.Model, Requested: id, Answered: answered}
	}

	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}

// rejectionFrameBytes is the literal "?;" NAK frame, reconstructed by
// ReadSetting from Engine.Do's cat.ErrRejected sentinel.
var rejectionFrameBytes = []byte("?;")

// ReadSetting reads one EX (MENU) setting by the opaque, radio-neutral id
// these drivers mint as the setting's EX wire address.
//
// id is parsed FIRST, entirely before any wire traffic: a failure — a
// malformed shape, or a well-formed address that is not a member of THIS
// dialect's inventory — returns *driver.UnknownSettingError and nothing is
// ever sent.
//
// Rejection mechanism: Engine.Do surfaces a "?;" reply as the
// cat.ErrRejected ERROR SENTINEL, never as returned frame bytes, so it is
// detected here with errors.Is and turned back into the canonical bytes
// for ParseEXResponse. What a rejection MEANS is not guessed: the address
// was in the inventory or it was refused above, so a "?;" says the radio
// declined to report a setting it declares — recorded as
// SettingUnavailable, a fact about this exchange and never a value.
//
// The caller holds its own operation mutex around this call where it has
// one; nothing here takes a lock.
func ReadSetting(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, p *Params, id string) (driver.SettingValue, error) {
	addr, err := dialect.ParseEXAddress(id)
	if err != nil {
		return driver.SettingValue{}, &driver.UnknownSettingError{Model: p.Model, ID: id}
	}

	cmd, err := dialect.BuildEXRead(addr)
	if err != nil {
		// Unreachable in practice: ParseEXAddress above already enforced
		// the identical inventory membership BuildEXRead checks. Kept as
		// defence in depth, not as an assumption that the two rules stay
		// the same one.
		return driver.SettingValue{}, fmt.Errorf("%s: ReadSetting %s: %w", p.Name, dialect.EXWire(addr), err)
	}

	frame, err := eng.Do(ctx, cmd, EXSpec(dialect, addr))
	if p.ReadGap != nil {
		p.ReadGap()
	}
	switch {
	case errors.Is(err, cat.ErrRejected):
		frame = rejectionFrameBytes
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("%s: ReadSetting %s: %w", p.Name, dialect.EXWire(addr), err)
	}

	return ParseEXResponse(dialect, p, addr, frame)
}
