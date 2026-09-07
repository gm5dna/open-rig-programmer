// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts590 "github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The two descriptor versions, minted HERE: the exact strings this package's
// SettingsDescriptor identifies itself with, and the ones
// codeplug.MenuSnapshot.Descriptor carries through verbatim so a snapshot can
// later be checked against the descriptor version that produced it.
//
// TWO STRINGS FOR ONE PACKAGE, WHICH NO OTHER DRIVER IN THIS PROJECT NEEDS,
// and the reason is the reason there are two inventories. The TS-590S and the
// TS-590SG do not share a menu table: the book prints two separate lists
// (590:564 and 590:744) over COLLIDING addresses with different meanings —
// menu 000 is Display brightness on the S (590:569) and read-only Firmware
// Version on the SG (590:749) — so a snapshot taken from one row must never
// validate against the other's descriptor, even though both come out of this
// same package. The "@1" is each shape's own generation: a later change to how
// THIS package builds THAT row's tree increments the one string.
// Pinned by TestSettingsDescriptor_VersionIsThisRowsOwn.
const (
	settingsVersionS  = "ts590s-ex@1"
	settingsVersionSG = "ts590sg-ex@1"
)

// settingsRootID and settingsRootLabel name the descriptor's SINGLE menu and
// its single group, and they are STRUCTURAL: they carry no claim that this
// radio has a menu called "Menu" or a group inside it.
//
// driver.SettingsDescriptor is a two-level tree and its Validate requires at
// least one menu, at least one group inside it and a non-empty ID and Label at
// every level. This radio's parameter lists print a menu number, a function
// name and a parameter legend, and NO GROUP HIERARCHY AT ALL — which is why
// all three Kenwood profiles register LabelsAbsent and every EXItem's P1Label
// and P2Label is "" (core/kw's EXItem states it). So the two nodes exist to
// satisfy the neutral type, and they are named for the radio's own word for
// the surface: both books head the address column of every EX parameter list
// "Menu" (590:566 the TS-590S list, 590:746 the TS-590SG list, 480:424 and
// 480:498 the 480's two lists). 590:543-544 and 480:401 are cited elsewhere
// in this file for the printed menu DOMAIN, not for this string.
//
// The FT-891 met the same shortage and fell back to STRUCTURE — the two-digit
// prefix of its four-digit address, as both ID and Label
// (core/driver/ft891/settings.go). There is no structure here to fall back on:
// a Kenwood menu number is one flat three-digit field.
const (
	settingsRootID    = "MENU"
	settingsRootLabel = "Menu"
)

// settingsSurface is one row's settings surface: the descriptor a caller
// walks, and the inventory rows ReadSetting looks an id up in.
//
// THE MAP IS WHY THIS IS A STRUCT AND NOT A DESCRIPTOR ALONE. core/kw's
// ParseEXAnswer takes the INVENTORY ROW and not just an address, because A19's
// width bound is per menu number — item.Digits, from the transcribed chart —
// so the driver must be able to recover the row an id names. Building the two
// together, from one pass over one inventory, is what makes it impossible for
// the published tree and the parse-time bound to come from different tables.
type settingsSurface struct {
	descriptor driver.SettingsDescriptor
	items      map[string]kw.EXItem
}

// settingsSurfaces is built ONCE, at package init, one entry per row, each
// from ITS OWN generated inventory and never from the other's (plan P8):
// kwts590.EXItemsS() has 88 rows and kwts590.EXItemsSG() 100, and the two
// tables' addresses collide with different meanings.
//
// An unset row has NO ENTRY, so every getter below hands out the zero
// descriptor for it — which driver.SettingsDescriptor.Validate refuses. That
// is this package's standing fail-closed direction (Capabilities and Open take
// the same one on the same value), and it needs no branch of its own.
var settingsSurfaces = map[Row]settingsSurface{
	RowS:  buildSettingsSurface(settingsVersionS, kwts590.EXItemsS()),
	RowSG: buildSettingsSurface(settingsVersionSG, kwts590.EXItemsSG()),
}

// buildSettingsSurface builds one row's descriptor and address index from its
// inventory, IN INVENTORY ORDER: one menu, one group, all of the row's items
// inside it.
//
// FLAT, AND THAT IS A RULING RATHER THAN AN OVERSIGHT (Stuart decisions row 4,
// RULED 05/09/2026; plan P8). A hundred items in one list is not a comfortable
// GUI surface, and the alternative on the table was a DISPLAY-ONLY decade
// grouping — 000-009, 010-019, … — with the item IDs unchanged. It was
// refused because it would put structure in a book that prints none: the
// grouping would be this project's invention rendered as though it were the
// radio's menu tree. The cost is named rather than hidden, and it is a cost in
// navigation only; nothing about what can be read changes with it.
//
// ITEM ID = THE THREE-DIGIT MENU NUMBER, which is why Stage 0 had to widen
// codeplug's setting-ID rule a third time: clone.ReadSettings validates every
// descriptor item ID through codeplug.MenuSnapshot.Validate before any wire
// traffic, and until that widening the rule admitted four and six digits only
// (core/codeplug/menus.go's isSettingIDWidth, the only place a width is
// judged). TestCloneReadSettings_WalksTheWholeDescriptor is where that
// dependency is exercised rather than merely stated.
//
// DISPLAY IS BUILT SEPARATELY FROM ID AND HAPPENS TO EQUAL IT. Kenwood prints
// the address as ONE three-digit Menu No., so the printed form and the wire
// form coincide on this family — a FACT ABOUT THIS CHART, not a rule: the
// FT-710 prints "01-01-01" against a six-digit ID. They are two expressions
// here, and TestSettingsDescriptor_ItemIDsAreThreeASCIIDigits states the
// coincidence, so that nobody later "de-duplicates" one into the other and
// silently makes the ID the display form of whatever address shape comes next.
// The FT-891's settings.go carries the same warning for the same reason.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an address,
// the chart's own Function name and a display form per item; it carries no
// value legend, no units, no enumerated options and no default, and ReadSetting
// returns P5 verbatim. Every legend in these two parameter lists is therefore
// untouched by this surface, which is what keeps a menu read a transcription of
// what the radio said rather than an interpretation of it.
func buildSettingsSurface(version string, items []kw.EXItem) settingsSurface {
	s := settingsSurface{
		descriptor: driver.SettingsDescriptor{
			Version: version,
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
		id := it.Addr.Wire()
		group.Items = append(group.Items, driver.SettingItem{
			ID:      id,
			Label:   it.Name,
			Display: fmt.Sprintf("%03d", it.Addr.P1),
		})
		s.items[id] = it
	}
	return s
}

// SettingsDescriptor returns row's radio-neutral settings descriptor: a
// defensive Clone() of the tree buildSettingsSurface built once at package
// init, and the ZERO descriptor for an unset row.
//
// Every getter — this func, the driver's StaticSettingsDescriptor and the
// session's SettingsDescriptor — returns a Clone and never the value itself:
// nothing outside this file may hold a reference to the shared original,
// because a caller that mutated the tree it was handed would silently change
// what every later caller received (driver.SettingsDescriptor.Clone's own doc
// comment).
//
// IT TAKES A ROW WHERE EVERY SIBLING DRIVER'S TAKES NOTHING, for the same
// reason New does: one package serves two radios and neither may be reached by
// omission.
func SettingsDescriptor(row Row) driver.SettingsDescriptor {
	return settingsSurfaces[row].descriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to Session.SettingsDescriptor.
// Identical to it and to the package-level func, because this driver's
// settings tree depends only on the static inventory for its row and never on
// anything a live session discovers — and no Kenwood row discovers anything at
// all (decision 5, matrix §3.4).
func (d *ts590Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor(d.row)
}

// SettingsDescriptor implements half of the optional driver.SettingsReader
// capability on the concrete *Session, identically to the two above.
//
// It reads s.row rather than rebuilding anything: the package-level trees are
// built from the same two inventories every session of this driver opens
// against, and a per-session rebuild would cost a hundred items of allocation
// per call to produce the identical answer.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor(s.row)
}

// UnknownSettingError reports that ReadSetting's id argument does not name a
// setting THIS ROW publishes — refused BEFORE any wire traffic, exactly like
// ReadChannel's unknown-slot refusal (read.go).
//
// It covers two refusals a caller cannot tell apart from the id alone and does
// not need to: a malformed shape (anything but three ASCII digits on this
// family, so a Yaesu sibling's four- or six-digit ID lands here) and a
// well-formed three-digit address that is not a row of THIS row's inventory.
// "088" is the case that matters: a real setting on the TS-590SG, and outside
// the TS-590S's printed menu domain of 000 ~ 087 altogether (590:543). Both
// mean the same thing to a caller: this radio has no such setting.
//
// A DISTINCT TYPE FROM THE SLOT-WORDED UnknownSlotError (read.go), whose
// fields and message are worded for memory channels and whose Reason names
// what the row's BANKS publish. A menu address is a structurally different
// namespace, and reusing one error shape for both would blur two unrelated
// wire-address kinds.
//
// IT CARRIES NO errors.Is SENTINEL, where UnknownSlotError carries
// ErrUnknownSlot. A caller walking banks asks "is this slot one this radio
// has?" as a real branch — core/clone reads a bank inventory it did not
// itself publish — whereas every id that reaches ReadSetting in practice came
// out of THIS descriptor, so an unknown one is a caller's own mistake rather
// than a condition to branch on. errors.As on the concrete type is the whole
// interface it needs, which is the FT-891's judgement on its own namesake too.
type UnknownSettingError = driver.UnknownSettingError

// exSpec is the transport spec for one EX read of the menu address id.
//
// THE MATCH PREFIX CARRIES THE FULL THREE-DIGIT ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule, which kw.PrefixLenMatcher's
// own doc comment states and which it cannot enforce, having no
// address-shaped parameter. Every one of this row's menu addresses answers
// with a frame starting "EX" (590:543-544), so a bare prefix would let
// transport.Engine.Do correlate a DIFFERENT address's still-in-flight answer
// as this read's own and hand back one setting's value labelled as another's.
// TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the
// negative-space proof.
//
// THE MATCHER ALSO ADMITS A FRAME OF EXACTLY kw.EXReadLen — the read frame's
// own ten-byte shape, one byte short of kw.EXMinAnswerLen — which is a real
// property of a full-address EX matcher and not a bug in this one:
// core/kw/golden_test.go's a_commands_own_read_frame_is_not_its_answer
// subtest excludes EX from its general "a command's own read is never
// correlated as its answer" check for exactly this reason, and records that
// it is A25's assumption that Kenwood radios do not echo the host's own
// frames back on the line that keeps an echoed read from being delivered as
// this read's answer. A25 is unlifted (core/kw/doc.go). If it ever turns out
// false, ParseEXAnswer's own ten-byte-is-the-read check catches the echo one
// layer down, so this driver refuses loudly rather than returning a wrong
// value.
//
// THE EXACT LENGTH IS LEFT 0 — VARIABLE LENGTH, and this is the ONE frame in
// this family that is. P5 is "a string of alphanumeric characters for the Menu
// setting (variable length)" with no printed ceiling anywhere (590:555-556),
// which is A19; every other frame either radio sends is fixed (ID 6, FV 7,
// TY 6, MC 6, MR 50). So only the prefix is checked here and
// Layout.ParseEXAnswer applies the per-item printed width afterwards, from the
// inventory row itself.
//
// NO RETRY, where mrSpec carries one, and it is Task 13's own clause: "a
// timeout typed and never retried into a second frame". The two paths differ
// in what a retry would buy. A whole-radio read is 120 MR exchanges that
// should not fail on one swallowed reply, and its retry is preceded by the
// engine's drain-to-quiet so it cannot correlate a stale answer; a settings
// read is one item a caller asked for and can ask for again. Nothing here
// claims a TS-590 would answer a repeat differently — no Kenwood radio has
// been on a wire for this project — the driver simply does not send a second
// frame the caller did not ask for. Pinned, by frame COUNT, in
// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame.
func (s *Session) exSpec(id string) transport.CommandSpec {
	return s.newReadSpec(kw.PrefixLenMatcher("EX"+id, 0), 0)
}

// ReadSetting implements the other half of the optional driver.SettingsReader
// capability: reads one menu setting by the opaque, radio-neutral id this
// driver mints as the setting's three-digit menu number.
//
// MEMBERSHIP IS CHECKED HERE AND NOT IN core/kw, which is a division that
// package states in terms: Layout.BuildEXRead bounds an address by the row's
// printed DOMAIN (000 ~ 087 on the S, 000 ~ 099 on the SG, 590:543-544) and
// knows nothing of which addresses inside it a radio actually has, because the
// three generated inventories live in packages that import core/kw. So the
// caller reads its own inventory and asks for what is in it — and an id that
// is not in it is refused with *UnknownSettingError before any frame is built.
//
// THE WHOLE EXCHANGE HOLDS s.opMu, which is the Session type's own rule: ONE
// DRIVER OPERATION at a time (P13/P14, matrix M-E2). The lock is not
// protecting the setting — one EX read is one Engine.Do, and the engine
// already serialises an individual exchange — it is protecting the OTHERS: a
// settings read landing inside Open's three-frame probe, a ReadChannel or a
// WriteChannel would interleave a frame of its own with theirs on a radio
// whose only acknowledgement of a Set is silence (A6).
// TestReadSetting_IsAtomicUnderOpMu pins it.
//
// THE TWO WIRE OUTCOMES ARE READ DIFFERENTLY, AND THE DIFFERENCE IS THE SEAM'S
// RATHER THAN THIS RADIO'S:
//
//   - A "?;" maps to SettingValue{State: SettingUnavailable} with NO error,
//     which is driver.SettingsReader's own stated contract. It adds no new
//     reading of this family's unattributed NAK beyond the three already
//     registered — the probe's (ts590.go), ReadChannel's (read.go) and
//     WriteChannel's (write.go), plus the no-discovery ground (doc.go): the
//     address was a member of this row's own printed inventory before the
//     frame went out, so a "?;" here records that the radio declined to
//     report a setting it declares, and guesses nothing about why. This is
//     the ONE path in this driver where a rejection does not fail the
//     operation whole — ReadChannel's does (decision 5) — because
//     a channel read has no neutral state meaning "the radio declined" and the
//     settings seam has exactly one.
//   - SILENCE stays a failure, typed by wireFailure as *kw.TimeoutError, which
//     says in as many words that it is not an inference of absence. Same rule,
//     same function, as the probe's ID; and FV; and the read path's MR.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	item, ok := settingsSurfaces[s.row].items[id]
	if !ok {
		return driver.SettingValue{}, &UnknownSettingError{ID: id, Model: modelNameFor(s.row)}
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	cmd, err := s.layout.BuildEXRead(item.Addr)
	if err != nil {
		// Unreachable for an id the inventory published: every one of this
		// row's rows is inside the row's own printed domain, which is the
		// only bound BuildEXRead applies. Refuse rather than assume the two
		// rules stay the same one.
		return driver.SettingValue{}, fmt.Errorf("ts590: ReadSetting %s: %w", id, err)
	}

	frame, err := s.eng.Do(ctx, cmd, s.exSpec(id))
	switch {
	case errors.Is(err, transport.ErrRejected):
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ts590: ReadSetting %s: %w", id, wireFailure(s.layout, "EX", err))
	}

	raw, err := s.layout.ParseEXAnswer(frame, item)
	if err != nil {
		// The codec's verdict stays the codec's — the address correlation,
		// the printed P2/P3/P4 bytes, A19's per-item width ceiling and A2's
		// charset bound are all its — and the driver adds only the context
		// the parser cannot know, mirroring ReadChannel's own error-typing
		// split.
		return driver.SettingValue{}, fmt.Errorf("ts590: ReadSetting %s: %w", id, err)
	}
	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}
