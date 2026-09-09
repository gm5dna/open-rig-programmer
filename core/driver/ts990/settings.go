// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// settingsVersion is minted HERE: the exact string this package's
// SettingsDescriptor identifies itself with, and the one
// codeplug.MenuSnapshot.Descriptor carries through verbatim so a snapshot can
// later be checked against the descriptor version that produced it.
//
// ONE STRING, WHERE PAIR 1's PACKAGE NEEDS TWO. That package serves two rows
// whose menu tables collide at the same addresses with different meanings;
// this package is one registry row (spec decision 2), so there is one
// inventory and one shape to version. The "@1" is this shape's own
// generation: a later change to how this package builds the tree increments
// it, and a change to the CHART itself is the inventory's own affair.
const settingsVersion = "ts990s-ex@1"

// settingsRootID and settingsRootLabel name the descriptor's SINGLE menu and
// its single group, and they are STRUCTURAL: they carry no claim that this
// radio has a menu called "EX" or a group inside it.
//
// driver.SettingsDescriptor is a two-level tree and its Validate requires at
// least one menu, at least one group inside it and a non-empty ID and Label
// at every level. This chart prints P1, P2, P3, Function and P5 columns and
// NO GROUP HIERARCHY AT ALL — which is why this row's extable profile
// registers LabelsAbsent and every EXItem's P1Label and P2Label is "" — so
// the two nodes exist to satisfy the neutral type.
//
// THEY ARE NAMED FOR THE COMMAND AND NOT FOR EITHER PRINTED LIST. This book
// prints TWO lists, "Menu" (P1 = 0) and "Advanced Menu" (P1 = 1,
// 990:1720-1723), and the flat group holds both — so borrowing pair 1's
// "Menu" here would name the whole surface after half of it. The command
// whose parameter lists these are is EX, which is the one word true of every
// row in the group.
const (
	settingsRootID    = "EX"
	settingsRootLabel = "EX"
)

// settingsSurface is this row's settings surface: the descriptor a caller
// walks, and the inventory rows ReadSetting looks an id up in.
//
// THE MAP IS WHY THIS IS A STRUCT AND NOT A DESCRIPTOR ALONE.
// ma.Layout.ParseEXAnswer takes the INVENTORY ROW and not just an address,
// because A19's width bound is per menu address — item.Digits, from the
// transcribed chart — so the driver must be able to recover the row an id
// names. Building the two together, from one pass over one inventory, is what
// makes it impossible for the published tree and the parse-time bound to come
// from different tables.
type settingsSurface struct {
	descriptor driver.SettingsDescriptor
	items      map[string]kw.EXItem
}

// settings is built ONCE, at package initialisation, from THIS ROW'S OWN
// generated inventory — the layout's copy of it, so the descriptor's
// membership set and the one BuildEXRead, ParseEXAnswer and the outbound gate
// consult are one datum rather than two copies of one.
var settings = buildSettingsSurface(layout())

// buildSettingsSurface builds the descriptor and the address index from l's
// inventory, IN INVENTORY ORDER: one menu, one group, every row inside it.
//
// FLAT, AND THAT IS A RULING RATHER THAN AN OVERSIGHT (Stuart decisions row
// 4, RULED 05/09/2026; plan P8). The alternative on the table was a
// DISPLAY-ONLY grouping — by P1's two printed lists, or by P2's category
// number — with the item IDs unchanged. It was refused because it would put
// structure in a book that prints none: the grouping would be this project's
// invention rendered as though it were the radio's own menu tree. The cost is
// named rather than hidden, and it is a cost in navigation only; nothing
// about what can be read changes with it.
//
// ITEM ID = THE FIVE-CHARACTER WIRE ADDRESS, P1 + P2P2 + P3P3, which is why
// core/codeplug's setting-ID width rule had to admit five (spec decision 13):
// clone.ReadSettings validates every descriptor item ID through
// codeplug.MenuSnapshot.Validate before any wire traffic.
// TestCloneReadSettings_WalksTheWholeDescriptor is where that dependency is
// exercised rather than merely stated.
//
// DISPLAY IS BUILT SEPARATELY AND DIFFERS FROM THE ID ON THIS ROW, which is
// the opposite of pair 1's coincidence: this chart prints the address as
// THREE columns of one, two and two digits, so the printed form is "0 00 34"
// where the wire form is "00034". Keeping them two expressions is what stops
// a later reader "de-duplicating" one into the other.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an
// address, the chart's own Function name and a display form per item; it
// carries no value legend, no units, no enumerated options and no default,
// and ReadSetting returns P5 verbatim. Every legend in this parameter list is
// therefore untouched by this surface, which is what keeps a menu read a
// transcription of what the radio said rather than an interpretation of it —
// and it is why the five FIRMWARE-CONDITIONAL rows this chart prints
// (990:1814, 1866, 1924, 2054, 2163) need no branch here: their condition is
// part of the printed Function name the inventory carries, and nothing in
// this driver consults the version the FV probe answered.
func buildSettingsSurface(l ma.Layout) settingsSurface {
	items := l.EXItems()
	s := settingsSurface{
		descriptor: driver.SettingsDescriptor{
			Version: settingsVersion,
			Menus: []driver.SettingMenu{{
				ID:     settingsRootID,
				Label:  settingsRootLabel,
				Groups: []driver.SettingGroup{{ID: settingsRootID, Label: settingsRootLabel}},
			}},
		},
		items: make(map[string]kw.EXItem, len(items)),
	}
	group := &s.descriptor.Menus[0].Groups[0]
	for _, it := range items {
		id := l.WireEXAddress(it.Addr)
		group.Items = append(group.Items, driver.SettingItem{
			ID:      id,
			Label:   it.Name,
			Display: fmt.Sprintf("%d %02d %02d", it.Addr.P1, it.Addr.P2, it.Addr.P3),
		})
		s.items[id] = it
	}
	return s
}

// SettingsDescriptor returns this row's radio-neutral settings descriptor: a
// defensive Clone() of the tree built once at package initialisation.
//
// Every getter — this func, the driver's StaticSettingsDescriptor and the
// session's SettingsDescriptor — returns a Clone and never the value itself:
// nothing outside this file may hold a reference to the shared original,
// because a caller that mutated the tree it was handed would silently change
// what every later caller received.
//
// IT TAKES NO ARGUMENT, where pair 1's takes a Row, for the same reason New
// takes no Row: one package, one registry row.
func SettingsDescriptor() driver.SettingsDescriptor {
	return settings.descriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability: the driver-level,
// no-session-required counterpart to Session.SettingsDescriptor. Identical to
// it and to the package-level func, because this driver's settings tree
// depends only on the static inventory and no Kenwood row discovers anything
// at all (decision 5, matrix §3.4).
func (d *ts990Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements half of the optional driver.SettingsReader
// capability on the concrete *Session, identically to the two above.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError reports that ReadSetting's id argument does not name a
// setting THIS ROW publishes — refused BEFORE any wire traffic, exactly like
// ReadChannel's unknown-slot refusal (read.go).
//
// It covers two refusals a caller cannot tell apart from the id alone and
// does not need to: a malformed shape (a Yaesu sibling's four- or six-digit
// ID, or pair 1's three-digit Kenwood one) and a well-formed five-digit
// address that is not a row of this chart. BOTH CHARTS IN THIS FAMILY ARE
// SPARSE, with gaps inside every category, so the second is the case that
// matters and it is why membership rather than a scalar bound is the check at
// all: this book says an address it does not print "causes an error to occur"
// (990:1727, 990:1733-1735).
//
// A DISTINCT TYPE FROM THE SLOT-WORDED UnknownSlotError (read.go), whose
// fields and message are worded for memory channels. A menu address is a
// structurally different namespace, and reusing one error shape for both
// would blur two unrelated wire-address kinds.
type UnknownSettingError = driver.UnknownSettingError

// exSpec is the transport spec for one EX read of the menu address id.
//
// THE MATCH PREFIX CARRIES THE FULL FIVE-DIGIT ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule, which kw.PrefixLenMatcher's
// own doc comment states and which it cannot enforce, having no
// address-shaped parameter. Every one of this row's menu addresses answers
// with a frame starting "EX" (990:1738-1747), so a bare prefix would let
// transport.Engine.Do correlate a DIFFERENT address's still-in-flight answer
// as this read's own and hand back one setting's value labelled as another's.
// TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the
// negative-space proof.
//
// THE EXACT LENGTH IS LEFT 0 — VARIABLE, and on this row that is doubly
// necessary. P5's printed width is per item class, "3 digits" for most rows,
// 4 for the PF keys, 8 for the frequency settings, up to 15 for a power-on
// message (990:1742-1756); and THIS BOOK'S OWN Set/Answer diagram draws P5
// filling a fifteen-wide window with ';' nailed to position 24
// (990:1738-1747), which is erratum E19. The two readings cannot both be
// literal, so core/kw/ma admits both and applies the per-item width itself —
// there is no single length for a matcher to check.
//
// THE MATCHER ALSO ADMITS A FRAME OF EXACTLY THE READ'S OWN EIGHT BYTES, and
// that is a real property of a full-address EX matcher rather than a bug in
// this one: core/kw/ma records it as A12/L-HW-9's assumption that neither
// radio echoes the host's own frames back on the line. If that ever turns
// out false, ma.ParseEXAnswer's own check catches the echo one layer down —
// it names an eight-byte frame as the READ, "which carries no P4 at all" —
// so this driver refuses loudly rather than returning a wrong value.
//
// NO RETRY, where the channel read carries one, and the two paths differ in
// what a retry would buy. A whole-radio read is a hundred MA0 exchanges that
// should not fail on one swallowed reply; a settings read is one item a
// caller asked for and can ask for again. Nothing here claims a TS-990S would
// answer a repeat differently — no radio of this family has ever been on a
// wire for this project — the driver simply does not send a second frame the
// caller did not ask for. Pinned, by frame COUNT, in
// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame.
func (s *Session) exSpec(id string) transport.CommandSpec {
	return s.newReadSpec(kw.PrefixLenMatcher("EX"+id, 0), 0)
}

// ReadSetting implements the other half of the optional driver.SettingsReader
// capability: reads one menu setting by the opaque, radio-neutral id this
// driver mints as the setting's five-digit wire address.
//
// MEMBERSHIP IS CHECKED HERE AND IT IS THE SAME MEMBERSHIP THE CODEC APPLIES.
// ma.Layout.BuildEXRead refuses an address that is not in this row's own
// generated inventory, and so does the outbound gate; this map is built from
// that same inventory, so an id that is not in it is refused with
// *UnknownSettingError before any frame is built, with a message worded for a
// caller rather than for a codec.
//
// THE WHOLE EXCHANGE HOLDS s.opMu, which is the Session type's own rule: ONE
// DRIVER OPERATION at a time (P12/P13). The lock is not protecting the
// setting — one EX read is one Engine.Do, and the engine already serialises
// an individual exchange — it is protecting the OTHERS: a settings read
// landing inside Open's three-frame probe, a ReadChannel, or a WriteChannel's
// pre-write read and Set, would interleave a frame of its own with theirs on
// a radio whose only acknowledgement of a Set is silence (A20/L-HW-3).
// TestReadSetting_IsAtomicUnderOpMu pins it.
//
// THE TWO WIRE OUTCOMES ARE READ DIFFERENTLY, AND THE DIFFERENCE IS THE
// SEAM'S RATHER THAN THIS RADIO'S:
//
//   - A "?;" maps to SettingValue{State: SettingUnavailable} with NO error,
//     which is driver.SettingsReader's own stated contract. It adds no new
//     reading of this family's unattributed NAK beyond the ones already
//     registered: the address was a member of this row's own printed
//     inventory before the frame went out, so a "?;" here records that the
//     radio declined to report a setting it declares, and guesses nothing
//     about why. This is the ONE path in this driver where a rejection does
//     not fail the operation whole — ReadChannel's and WriteChannel's do —
//     because a channel read has no neutral state meaning "the radio
//     declined" and the settings seam has exactly one.
//   - SILENCE stays a failure, typed by wireFailure as *kw.TimeoutError,
//     which says in as many words that it is not an inference of absence.
//     Same rule, same function, as the probe's ID; and FV; and the channel
//     path's MA0.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	item, ok := settings.items[id]
	if !ok {
		return driver.SettingValue{}, &UnknownSettingError{ID: id, Model: modelName}
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	cmd, err := s.layout.BuildEXRead(item.Addr)
	if err != nil {
		// Unreachable for an id this inventory published: BuildEXRead's
		// bound is membership of the very table this map was built from.
		// Refuse rather than assume the two rules stay the same one.
		return driver.SettingValue{}, fmt.Errorf("ts990: ReadSetting %s: %w", id, err)
	}

	frame, err := s.eng.Do(ctx, cmd, s.exSpec(id))
	switch {
	case errors.Is(err, transport.ErrRejected):
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ts990: ReadSetting %s: %w", id, wireFailure("EX", err))
	}

	raw, err := s.layout.ParseEXAnswer(frame, item)
	if err != nil {
		// The codec's verdict stays the codec's — the address correlation,
		// the printed P4 space, E19's two admitted answer shapes, A19's
		// per-item width ceiling and A2's charset bound are all its — and
		// the driver adds only the context the parser cannot know,
		// mirroring ReadChannel's own error-typing split.
		return driver.SettingValue{}, fmt.Errorf("ts990: ReadSetting %s: %w", id, err)
	}
	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}
