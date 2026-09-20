// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// SettingsDescriptor describes a radio's menu/settings surface, radio-
// NEUTRAL: it names menus, subgroups, and items by opaque string IDs and
// human labels only, never a wire protocol's field shape. TWO-LEVEL
// hierarchy mirroring the radio's own menu structure: a Descriptor holds
// Menus, each Menu holds subgroup Groups, each Group holds leaf Items.
//
// Generic layers (task 33's MenuSnapshot, and everything above it) walk
// this tree without ever importing a driver's protocol package: the
// FT-710's cat.EXAddress never appears here, only the ID string its
// dialect-rendered wire form (cat.Dialect.EXWire) happens to equal
// (core/driver/ft710/settings.go mints item IDs as the 6-digit wire
// address, but nothing in this package or its callers may assume that
// shape belongs to every future driver — a Pair-form dialect renders four
// digits, not six).
type SettingsDescriptor struct {
	// Version identifies the SHAPE of this descriptor, e.g.
	// "ft710-ex@1" — minted by the driver that built it, carried
	// verbatim into task 33's MenuSnapshot.Descriptor so a snapshot can
	// be checked against the descriptor that produced it.
	Version string
	// Menus is the top-level list, in the driver's own display order.
	Menus []SettingMenu
}

// SettingMenu is one top-level menu, e.g. FT-710 EX menu P1=01 "RADIO
// SETTING".
type SettingMenu struct {
	// ID is the opaque, radio-neutral menu identifier, e.g. "01".
	ID string
	// Label is the human-readable menu name, e.g. "RADIO SETTING".
	Label string
	// Groups is this menu's subgroups, in the driver's own display
	// order.
	Groups []SettingGroup
}

// SettingGroup is one subgroup within a SettingMenu, e.g. FT-710 EX
// subgroup P1=01/P2=01 "MODE SSB".
type SettingGroup struct {
	// ID is the opaque, radio-neutral subgroup identifier, e.g. "0101".
	ID string
	// Label is the human-readable subgroup name, e.g. "MODE SSB".
	Label string
	// Items is this group's leaf settings, in the driver's own display
	// order.
	Items []SettingItem
}

// SettingItem is one leaf setting: the unit ReadSetting addresses.
type SettingItem struct {
	// ID is the opaque, radio-neutral setting identifier. A driver mints
	// this however suits its own protocol (the FT-710 uses its 6-digit
	// EX wire address); callers must treat it as an opaque token, never
	// parse it.
	ID string
	// Label is the human-readable setting name, e.g. "AF TREBLE GAIN".
	Label string
	// Display is a driver-computed, human-facing rendering of the
	// setting's position, e.g. the FT-710's "01-01-01" — for UI display
	// only, never re-parsed as an ID.
	Display string
	// Write reports this item's write-gate state: spec.Supported once a
	// Session W row has characterised it (this radio's write
	// descriptor's Width != 0 — see cat.Dialect.CanSetEX), spec.
	// Unverified otherwise (including every setting on a radio with no
	// write descriptor at all). No other spec.Support value is ever
	// minted here: a menu setting is not Session-consented
	// (SettingsWriter has no consent parameter — R1) and never Inert or
	// Unsupported by construction, since only a known inventory member
	// reaches BuildDescriptor's loop.
	Write spec.Support
}

// Validate checks d's structural invariants: a non-empty Version; at
// least one Menu, each Menu at least one Group, each Group at least one
// Item (no empty slice at any level); a non-empty, non-empty-labelled ID
// at every level; and ID uniqueness — Menu IDs unique among Menus, Group
// IDs unique among Groups, and Item IDs unique GLOBALLY across the whole
// descriptor (not merely within their own Group), since a caller
// addressing ReadSetting by ID has no group context to disambiguate a
// collision.
func (d SettingsDescriptor) Validate() error {
	if d.Version == "" {
		return fmt.Errorf("driver: SettingsDescriptor: Version must not be empty")
	}
	if len(d.Menus) == 0 {
		return fmt.Errorf("driver: SettingsDescriptor: Menus must not be empty")
	}

	menuIDs := make(map[string]bool, len(d.Menus))
	groupIDs := make(map[string]bool)
	itemIDs := make(map[string]bool)

	for _, m := range d.Menus {
		if m.ID == "" {
			return fmt.Errorf("driver: SettingsDescriptor: a menu has an empty ID")
		}
		if m.Label == "" {
			return fmt.Errorf("driver: SettingsDescriptor: menu %q has an empty Label", m.ID)
		}
		if menuIDs[m.ID] {
			return fmt.Errorf("driver: SettingsDescriptor: duplicate menu ID %q", m.ID)
		}
		menuIDs[m.ID] = true

		if len(m.Groups) == 0 {
			return fmt.Errorf("driver: SettingsDescriptor: menu %q has no Groups", m.ID)
		}
		for _, g := range m.Groups {
			if g.ID == "" {
				return fmt.Errorf("driver: SettingsDescriptor: menu %q has a group with an empty ID", m.ID)
			}
			if g.Label == "" {
				return fmt.Errorf("driver: SettingsDescriptor: group %q has an empty Label", g.ID)
			}
			if groupIDs[g.ID] {
				return fmt.Errorf("driver: SettingsDescriptor: duplicate group ID %q", g.ID)
			}
			groupIDs[g.ID] = true

			if len(g.Items) == 0 {
				return fmt.Errorf("driver: SettingsDescriptor: group %q has no Items", g.ID)
			}
			for _, it := range g.Items {
				if it.ID == "" {
					return fmt.Errorf("driver: SettingsDescriptor: group %q has an item with an empty ID", g.ID)
				}
				if it.Label == "" {
					return fmt.Errorf("driver: SettingsDescriptor: item %q has an empty Label", it.ID)
				}
				if itemIDs[it.ID] {
					return fmt.Errorf("driver: SettingsDescriptor: duplicate item ID %q (item IDs must be globally unique, not merely unique within their own group)", it.ID)
				}
				itemIDs[it.ID] = true
			}
		}
	}
	return nil
}

// Clone returns a defensive deep copy of d: mutating the returned value's
// Menus, any Menu's Groups, or any Group's Items can never reach d itself.
// This is load-bearing for SettingsReader.SettingsDescriptor, whose
// contract promises every caller an independent copy.
func (d SettingsDescriptor) Clone() SettingsDescriptor {
	out := SettingsDescriptor{Version: d.Version}
	if d.Menus == nil {
		return out
	}
	out.Menus = make([]SettingMenu, len(d.Menus))
	for i, m := range d.Menus {
		cm := SettingMenu{ID: m.ID, Label: m.Label}
		if m.Groups != nil {
			cm.Groups = make([]SettingGroup, len(m.Groups))
			for j, g := range m.Groups {
				cg := SettingGroup{ID: g.ID, Label: g.Label}
				if g.Items != nil {
					// SettingItem holds only strings — no nested
					// reference types — so a plain element copy is
					// already a full deep copy at this level.
					cg.Items = make([]SettingItem, len(g.Items))
					copy(cg.Items, g.Items)
				}
				cm.Groups[j] = cg
			}
		}
		out.Menus[i] = cm
	}
	return out
}

// SettingReadState reports what a SettingValue's Raw field means.
type SettingReadState int

const (
	// SettingKnown means Raw holds the setting's current raw value, read
	// successfully.
	SettingKnown SettingReadState = iota
	// SettingUnavailable means the radio rejected the read (its single,
	// unattributed "?;" NAK) — a recorded FACT about this exchange, not
	// an error: mirrors the project's established "?;" -> empty-result
	// rule (see core/driver/ft710/read.go's ReadChannel empty-slot
	// mapping). Raw is "" in this state.
	SettingUnavailable
)

// SettingValue is the outcome of one ReadSetting call.
type SettingValue struct {
	// ID is the SettingItem.ID this value answers for.
	ID string
	// Raw is the setting's raw canonical value, verbatim as the driver's
	// protocol carries it — no typed value model, no unit conversion.
	// "" when State is SettingUnavailable.
	Raw string
	// State reports whether Raw is meaningful.
	State SettingReadState
}

// SettingsReader is an OPTIONAL capability a driver.Session's CONCRETE
// type may implement: read individual radio settings against the tree
// SettingsDescriptor describes. Deliberately NOT added to driver.Session
// itself — precisely the design core/clone/memory_selector.go's
// MemorySelector already established (see its doc comment): adding this
// as a mandatory Session method would force every future driver to
// implement a settings surface even for a radio whose protocol has no
// such concept at all (or one this project has not yet characterised).
// Instead a caller that wants this capability performs a plain type
// assertion, sess.(driver.SettingsReader), against the concrete session a
// driver's Open returned — exactly MemorySelector's
// s.sess.(MemorySelector) pattern in core/clone/execute.go. Tasks 33-36
// consume this interface; core/driver/ft710.Session is its first (and, to
// date, only) implementation.
type SettingsReader interface {
	// SettingsDescriptor returns this session's settings tree — a
	// defensive copy (see SettingsDescriptor.Clone): mutating the
	// result can never alter what the session itself holds or what a
	// later call returns.
	SettingsDescriptor() SettingsDescriptor
	// ReadSetting reads one setting by its opaque SettingItem.ID. An id
	// that does not name a known item is refused BEFORE any wire
	// traffic, with a driver-specific typed error (e.g.
	// core/driver/ft710's *UnknownSettingError) — never a guess. A
	// known id that the radio currently rejects on the wire ("?;")
	// returns SettingValue{State: SettingUnavailable}, NOT an error —
	// mirroring the empty-slot rule ReadChannel already established.
	ReadSetting(ctx context.Context, id string) (SettingValue, error)
}

// SettingWriteOutcome reports how a WriteSetting call resolved, for
// display (CLI and GUI show all four — spec §4) and for a caller
// deciding whether a value landed.
type SettingWriteOutcome int

const (
	// SettingWriteAccepted means the Set frame drew no rejection within
	// its error window AND the paired read-back matched Wanted
	// byte-for-byte: the value is proven to have landed.
	SettingWriteAccepted SettingWriteOutcome = iota
	// SettingWriteRefused means the radio rejected the Set with "?;"
	// within its error window — an ERROR, unlike ReadSetting's read-only
	// SettingUnavailable state, because a write that is refused is a
	// failure a caller must act on, not a fact to display quietly.
	SettingWriteRefused
	// SettingWriteVerifyMismatch means the Set drew no rejection but the
	// paired read-back returned a value other than Wanted.
	SettingWriteVerifyMismatch
	// SettingWriteOutcomeUnknown means the Set's own outcome, or the
	// paired read-back's, could not be attributed — a transport failure
	// or lost link mid-pair, never a positive claim either way.
	SettingWriteOutcomeUnknown
)

// String returns the constant's identifier.
func (o SettingWriteOutcome) String() string {
	switch o {
	case SettingWriteAccepted:
		return "accepted-and-verified"
	case SettingWriteRefused:
		return "refused"
	case SettingWriteVerifyMismatch:
		return "verify-mismatch"
	case SettingWriteOutcomeUnknown:
		return "outcome-unknown"
	default:
		return "SettingWriteOutcome(invalid)"
	}
}

// SettingWriteResult is the outcome of one WriteSetting call.
type SettingWriteResult struct {
	// ID is the SettingItem.ID this result answers for.
	ID string
	// Wanted is the raw value WriteSetting was asked to set.
	Wanted string
	// Observed is the paired read-back's raw value. "" when no read-back
	// was attributable (SettingWriteRefused or SettingWriteOutcomeUnknown
	// from a Set-side failure).
	Observed string
	// Step is the frame-level record of the Set itself — reused from
	// WriteChannel's choreography type, but not sufficient alone: it
	// says whether the Set frame went and drew no rejection, never
	// whether the value it carried actually landed. Only Outcome (backed
	// by the paired read-back) answers that.
	Step WriteStep
	// Outcome is this call's overall resolution.
	Outcome SettingWriteOutcome
}

// SettingVerifyMismatchError reports that a WriteSetting call's paired
// read-back disagreed with the value just written.
//
// A DISTINCT type from *clone.VerifyMismatchError (core/clone/errors.go),
// not a reuse of it: that type is codeplug.Channel-shaped (a Slot and a
// []spec.Field), and a setting write has neither — one opaque ID and two
// raw strings is the smallest shape this comparison needs.
type SettingVerifyMismatchError struct {
	// ID is the SettingItem.ID that failed verification.
	ID string
	// Wanted is the raw value that was written.
	Wanted string
	// Observed is the raw value the paired read-back returned.
	Observed string
}

// Error implements the error interface.
func (e *SettingVerifyMismatchError) Error() string {
	return fmt.Sprintf("driver: WriteSetting %s: wrote %q but read back %q", e.ID, e.Wanted, e.Observed)
}

// SettingsWriter is an OPTIONAL capability a driver.Session's CONCRETE
// type may implement: write individual radio settings, mirroring
// SettingsReader's shape and its reason for staying OUT of driver.Session
// (see that interface's doc comment) — a plain type assertion,
// sess.(driver.SettingsWriter), against the concrete session a driver's
// Open returned. core/driver/ft710.Session is its first implementation.
//
// NO CONSENT PARAMETER (ruling R1, spec §4): unlike a channel write,
// there is no settingsCanWrite predicate and no consented argument here.
// A radio's write descriptor admits an address the moment its own
// hardware characterisation (Session W) exists, and FieldSupport.
// CanWrite's usual second key — ConsentedUnverified, recorded user
// consent standing in for missing hardware evidence — has nothing to
// stand on for a menu setting: the only paper source is the manual's
// Table 2 chart, already proven untrustworthy. WriteSetting therefore
// gates on one fact alone, a driver-specific equivalent of
// SettingItem.Write == spec.Supported.
type SettingsWriter interface {
	// WriteSetting writes value to the setting named by id, then
	// verifies it: id that does not name a known item, or a known item
	// this session cannot currently write (no hardware characterisation
	// yet, or value outside its domain/width), is refused BEFORE any
	// wire traffic, with a driver-specific typed error — never a guess.
	// A known, writable id is set fire-and-forget and read back; the
	// returned SettingWriteResult.Outcome and error together report
	// which of the four outcomes resulted (see SettingWriteOutcome). A
	// verify-mismatch's error is *SettingVerifyMismatchError.
	WriteSetting(ctx context.Context, id, value string) (SettingWriteResult, error)
}
