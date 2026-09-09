// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// settingsVersion is the exact string this package's SettingsDescriptor
// identifies itself with, and the one codeplug.MenuSnapshot.Descriptor carries
// through verbatim so a snapshot can later be checked against the descriptor
// version that produced it.
//
// ONE STRING FOR ONE PACKAGE, where pair 1's needs two: that package serves
// two registry rows whose menu tables collide at the same addresses with
// different meanings, and this one serves the TS-890S alone. The "@1" is this
// shape's own generation: a later change to how this package builds the tree
// increments it. Pinned by TestSettingsDescriptor_VersionIsThisRowsOwn.
const settingsVersion = "ts890s-ex@1"

// settingsRootID and settingsRootLabel name the descriptor's SINGLE menu and
// its single group, and they are STRUCTURAL: they carry no claim that this
// radio has a menu called "Menu" or a group inside it.
//
// driver.SettingsDescriptor is a two-level tree and its Validate requires at
// least one menu, at least one group inside it and a non-empty ID and Label at
// every level. This radio's parameter list prints an address, a Function name
// and a value legend, and NO GROUP HIERARCHY AT ALL — which is why this row's
// extable profile registers LabelsAbsent and every EXItem's P1Label and
// P2Label is "". So the two nodes exist to satisfy the neutral type, and they
// are named for the radio's own word for the surface: the chart's address
// block is headed "Menu" (890:1935).
//
// THERE IS STRUCTURE HERE THAT IS DELIBERATELY NOT USED. P1 is a menu-TYPE
// flag with two printed values, "0: Menu" and "1: Advanced Menu"
// (890:1897-1900), so a two-menu tree was available. It is not taken, for the
// reason P8 gives for the flat shape generally: the chart prints the two lists
// under ONE heading with one continuous address column, and splitting them
// would be this project's arrangement rendered as though it were the radio's.
const (
	settingsRootID    = "MENU"
	settingsRootLabel = "Menu"
)

// settingsSurface is this row's settings surface: the descriptor a caller
// walks, and the inventory rows ReadSetting looks an id up in.
//
// THE MAP IS WHY THIS IS A STRUCT AND NOT A DESCRIPTOR ALONE.
// ma.Layout.ParseEXAnswer takes the INVENTORY ROW and not just an address,
// because A19's width bound is per menu number — item.Digits, from the
// transcribed chart — so the driver must be able to recover the row an id
// names. Building the two together, from one pass over one inventory, is what
// makes it impossible for the published tree and the parse-time bound to come
// from different tables.
type settingsSurface struct {
	descriptor driver.SettingsDescriptor
	items      map[string]kw.EXItem
}

// settings is built ONCE, at package init, from THIS ROW'S OWN generated
// inventory — reached through the layout this package's sessions speak, which
// is also where BuildEXRead, ParseEXAnswer and the outbound gate ask their
// membership question. A bound is consulted from the same place as its datum.
var settings = buildSettings()

// buildSettings builds the descriptor and the address index from the layout's
// inventory, IN INVENTORY ORDER: one menu, one group, every row inside it.
//
// FLAT, AND THAT IS A RULING RATHER THAN AN OVERSIGHT (Stuart decisions row 4;
// plan P8). The alternative on the table was a DISPLAY-ONLY grouping — by menu
// type, or by category number — with the item IDs unchanged. It was refused
// because it would put structure in a book that prints none as though it were
// the radio's menu tree. The cost is named rather than hidden, and it is a cost
// in navigation only; nothing about what can be read changes with it.
//
// ITEM ID = THE FIVE-CHARACTER WIRE ADDRESS, P1 + P2P2 + P3P3 (890:1900),
// which is why Stage 0 had to widen codeplug's setting-ID rule again:
// clone.ReadSettings validates every descriptor item ID through
// codeplug.MenuSnapshot.Validate before any wire traffic, and until that
// widening the rule admitted three, four and six digits only (core/codeplug/
// menus.go's isSettingIDWidth, the only place a width is judged).
// TestCloneReadSettings_WalksTheWholeDescriptor is where that dependency is
// exercised rather than merely stated.
//
// DISPLAY IS BUILT SEPARATELY FROM ID AND ON THIS ROW THEY DIFFER. The chart
// prints the address as THREE CELLS under headings P1, P2 and P3 —
// "0   00   00" (890:1937-1939) — while the wire runs them together into five
// characters, so the printed form and the wire form are genuinely two things
// here where pair 1's coincide. TestSettingsDescriptor_DisplayIsThePrinted
// AddressAndNotTheWireOne states it, so that nobody later "de-duplicates" one
// into the other.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an address,
// the chart's own Function name and a display form per item; no value legend,
// no units, no enumerated options, no default, and ReadSetting returns P5
// verbatim. Every legend in this parameter list is therefore untouched by this
// surface, which keeps a menu read a transcription of what the radio said
// rather than an interpretation of it.
func buildSettings() settingsSurface {
	l := layout()
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
// defensive Clone() of the tree buildSettings built once at package init.
//
// EVERY GETTER RETURNS A CLONE and never the value itself — this func, the
// driver's StaticSettingsDescriptor and the session's SettingsDescriptor —
// because nothing outside this file may hold a reference to the shared
// original: a caller that mutated the tree it was handed would silently change
// what every later caller received (driver.SettingsDescriptor.Clone's own doc
// comment).
func SettingsDescriptor() driver.SettingsDescriptor { return settings.descriptor.Clone() }

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to Session.SettingsDescriptor.
// Identical to it and to the package-level func, because this driver's
// settings tree depends only on the static inventory and never on anything a
// live session discovers — and no Kenwood row discovers anything at all
// (decision 5, matrix §3.4).
func (d *ts890Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements half of the optional driver.SettingsReader
// capability on the concrete *Session, identically to the two above. It
// rebuilds nothing: the tree is built from the same inventory every session of
// this driver opens against, and a per-session rebuild would cost the whole
// chart's allocation per call to produce the identical answer.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError reports that ReadSetting's id argument does not name a
// setting THIS ROW publishes — refused BEFORE any wire traffic, exactly like
// ReadChannel's unknown-slot refusal (read.go).
//
// It covers two refusals a caller cannot tell apart from the id alone and does
// not need to: a malformed shape (anything but five ASCII digits on this row,
// so a sibling's three-, four- or six-digit ID lands here) and a well-formed
// five-digit address that is not a row of this chart. BOTH CHARTS OF THIS PAIR
// ARE SPARSE, with gaps inside every category, so the second is the ordinary
// case rather than the exotic one — and the book says what the radio does with
// such an address: "Entering a non-existing number causes an error to occur"
// (890:1904).
//
// A DISTINCT TYPE FROM THE SLOT-WORDED UnknownSlotError (read.go), whose
// fields and message are worded for memory channels. It carries no errors.Is
// sentinel, where that one does: every id that reaches ReadSetting in practice
// came out of THIS descriptor, so an unknown one is a caller's own mistake
// rather than a condition to branch on.
type UnknownSettingError = driver.UnknownSettingError

// exSpec is the transport spec for one EX read of the menu address id.
//
// THE MATCH PREFIX CARRIES THE FULL FIVE-CHARACTER ADDRESS, never the bare
// "EX" command name — the shared-prefix-family rule, which
// kw.PrefixLenMatcher's own doc comment states and which it cannot enforce,
// having no address-shaped parameter. Every one of this row's menu addresses
// answers with a frame starting "EX" (890:1909-1913), so a bare prefix would
// let transport.Engine.Do correlate a DIFFERENT address's still-in-flight
// answer as this read's own and hand back one setting's value labelled as
// another's. TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the
// negative-space proof.
//
// THE EXACT LENGTH IS LEFT 0 — VARIABLE LENGTH — AND ON THIS ROW THAT IS THE
// BOOK'S OWN SHAPE rather than a concession. The 890S's EX ruler reads "9~"
// and the terminator's header cell is the letter "x" (890:1898-1904,
// 890:1909-1913), so the answer has no printed width at all; P5's ceiling is
// per menu number (890:1916-1921), which is A19, and ma.Layout.ParseEXAnswer
// applies it afterwards from the inventory row itself.
//
// THE MATCHER ALSO ADMITS A FRAME OF EXACTLY ma.EXReadLen — the read frame's
// own eight bytes, one short of the shortest possible answer — which is a real
// property of a full-address prefix matcher and not a bug in this one. What
// keeps an echoed read from being delivered as its own answer is A12, that
// neither radio echoes the host's own frames, which is unlifted; if it ever
// turns out false, ParseEXAnswer's own minimum-length check catches the echo
// one layer down and this driver refuses loudly rather than returning a wrong
// value.
//
// NO RETRY, where ma0Spec carries one, and the two paths differ in what a
// retry would buy. A whole-radio read is a hundred MA0 exchanges that should
// not fail on one swallowed reply, and its retry is preceded by the engine's
// drain-to-quiet so it cannot correlate a stale answer; a settings read is one
// item a caller asked for and can ask for again. Nothing here claims a TS-890S
// would answer a repeat differently — none has ever been on a wire for this
// project — the driver simply does not send a second frame the caller did not
// ask for. Pinned, by frame COUNT, in
// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame.
func (s *Session) exSpec(id string) transport.CommandSpec {
	return s.newReadSpec(kw.PrefixLenMatcher("EX"+id, 0), 0)
}

// ReadSetting implements the other half of the optional driver.SettingsReader
// capability: reads one menu setting by the opaque, radio-neutral id this
// driver mints as the setting's five-character wire address.
//
// MEMBERSHIP IS CHECKED HERE AND IT IS THE SAME QUESTION core/kw/ma ASKS,
// which is the difference from pair 1 rather than a duplication of it: that
// family's builder bounds an address by a printed contiguous DOMAIN and knows
// nothing of which addresses inside it a radio has, because its inventories
// live in packages that import it. Here the inventory and the codec are in one
// package, so ma.Layout.BuildEXRead refuses a non-member itself — and this
// check is still first, so the refusal a caller meets is the driver's typed
// *UnknownSettingError naming the model, before any frame is built.
//
// THE WHOLE EXCHANGE HOLDS s.opMu, which is the Session type's own rule: ONE
// DRIVER OPERATION at a time (P13). The lock is not protecting the setting —
// one EX read is one Engine.Do, and the engine already serialises an
// individual exchange — it is protecting the OTHERS: a settings read landing
// inside Open's three-frame probe, a ReadChannel, or a WriteChannel's
// read-then-Set pair would interleave a frame of its own with theirs on a
// radio whose only acknowledgement of a Set is silence.
// TestReadSetting_IsAtomicUnderOpMu pins it.
//
// THE TWO WIRE OUTCOMES ARE READ DIFFERENTLY, AND THE DIFFERENCE IS THE SEAM'S
// RATHER THAN THIS RADIO'S:
//
//   - A "?;" maps to SettingValue{State: SettingUnavailable} with NO error,
//     which is driver.SettingsReader's own stated contract. It adds no new
//     reading of this family's unattributed NAK beyond the three already
//     registered — the probe's, ReadChannel's and WriteChannel's: the address
//     was a member of this row's own inventory before the frame went out, so a
//     "?;" here records that the radio declined to report a setting it
//     declares, and guesses nothing about why. This is the ONE path in this
//     driver where a rejection does not fail the operation whole, because a
//     channel read has no neutral state meaning "the radio declined" and the
//     settings seam has exactly one.
//   - SILENCE stays a failure, typed by wireFailure as *kw.TimeoutError, which
//     says in as many words that it is not an inference of absence. Same rule,
//     same function, as the probe's ID; and FV; and the read path's MA0.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	item, ok := settings.items[id]
	if !ok {
		return driver.SettingValue{}, &UnknownSettingError{ID: id, Model: modelName}
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	cmd, err := s.layout.BuildEXRead(item.Addr)
	if err != nil {
		// Unreachable for an id the inventory published: BuildEXRead asks the
		// same layout's own EXItem, which is where this map came from. Refuse
		// rather than assume the two rules stay the same one.
		return driver.SettingValue{}, fmt.Errorf("ts890: ReadSetting %s: %w", id, err)
	}

	frame, err := s.eng.Do(ctx, cmd, s.exSpec(id))
	switch {
	case errors.Is(err, transport.ErrRejected):
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ts890: ReadSetting %s: %w", id, wireFailure(s.layout.Book(), "EX", err))
	}

	raw, err := s.layout.ParseEXAnswer(frame, item)
	if err != nil {
		// The codec's verdict stays the codec's — the address correlation,
		// the printed P4 space, A19's per-item width ceiling and A2's charset
		// bound are all its — and the driver adds only the context the parser
		// cannot know, mirroring ReadChannel's own error-typing split.
		return driver.SettingValue{}, fmt.Errorf("ts890: ReadSetting %s: %w", id, err)
	}
	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}
