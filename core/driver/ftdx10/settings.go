// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// settingsDescriptorVersion is minted HERE — the exact string this driver's
// SettingsDescriptor identifies itself with, and the one
// codeplug.MenuSnapshot.Descriptor carries through verbatim so a snapshot
// can later be checked against the descriptor version that produced it.
//
// A DIFFERENT string from the FT-710's "ft710-ex@1", necessarily: the two
// descriptors describe different radios' menus (197 items in four menus
// here, 296 in five there), so a snapshot taken from one must never
// validate against the other. The "@1" is this shape's own generation, not
// the FT-710's — a later change to how THIS driver builds its tree
// increments it here alone.
const settingsDescriptorVersion = "ftdx10-ex@1"

// ftdx10SettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries (catDialect.EXItems, generated
// from core/cat/ftdx10/table2.csv) — see buildSettingsDescriptor.
//
// Every getter — the package-level SettingsDescriptor func, the driver's
// StaticSettingsDescriptor and the session's SettingsDescriptor — returns a
// Clone() of this and never the value itself: nothing outside this file may
// ever hold a reference to the shared original, because a caller that
// mutated the tree it was handed would silently change what every later
// caller received (driver.SettingsDescriptor.Clone's own doc comment).
var ftdx10SettingsDescriptor = buildSettingsDescriptor(catDialect)

// buildSettingsDescriptor builds the FTdx10's driver.SettingsDescriptor
// from dialect.EXItems(): one SettingMenu per distinct P1 (ID the 2-digit
// decimal P1, e.g. "01"; Label the manual's P1 column), one SettingGroup
// per distinct (P1,P2) pair nested under its menu (ID the 4-digit P1P2,
// e.g. "0101"; Label the manual's P2 column), and one SettingItem per
// inventory row nested under its group, IN INVENTORY ORDER (ID the item's
// 6-digit wire address via EXAddress.Wire, Label the manual's Function
// name, Display the human "P1-P2-P3" form, e.g. "01-01-01").
//
// DIALECT-PARAMETERISED THROUGHOUT, which is what lets this be the FT-710
// template's second instance rather than a copy of its output: the item
// count, the menu and group partition, every label and every width come
// from the dialect argument. Nothing about the FTdx10's own numbers (197
// items, P1 in {01,02,03,04}, 18 groups) is written into this function —
// they are properties of the inventory it is handed, asserted in
// settings_test.go against the dialect rather than against literals alone.
//
// Dialect.EXItems returns its rows already sorted by (P1,P2,P3) (its own
// doc comment, and core/cat/ftdx10's generated inventory is emitted in that
// order), so a single linear pass — opening a new menu or group only when
// the running P1/P2 changes from the PREVIOUS item — reproduces the
// manual's own chart grouping with no extra sorting or map bookkeeping.
// Each menu/group pointer used to append is re-derived fresh, by index, on
// every iteration and never carried across one: that sidesteps any question
// of whether an earlier append's slice growth could invalidate a held
// pointer, at the cost of one slice index per item — a non-issue at 197
// items, run once at package init.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an
// address, two labels and a human display form per item; it does not carry
// an item's value legend, its units, its enumerated options or its default,
// and ReadSetting below returns the P4 body verbatim. That is why
// core/cat/ftdx10/doc.go's four recorded CHART PRINTING DEFECTS — the
// duplicate index in SHIFT FREQUENCY, the missing index 0 in MARK
// FREQUENCY, DECODE AFC RANGE's non-monotonic legend, PSK TX LEVEL's
// unpadded range — do not bite this surface: every one of them lives in a
// value legend, and this driver interprets no legend. They become questions
// the moment a caller tries to render a menu value as a MEANING rather than
// as the bytes the radio sent, and that is deliberately not this file's
// business.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	items := dialect.EXItems()

	d := driver.SettingsDescriptor{Version: settingsDescriptorVersion}

	for _, it := range items {
		menuID := fmt.Sprintf("%02d", it.Addr.P1)
		groupID := fmt.Sprintf("%02d%02d", it.Addr.P1, it.Addr.P2)

		if len(d.Menus) == 0 || d.Menus[len(d.Menus)-1].ID != menuID {
			d.Menus = append(d.Menus, driver.SettingMenu{ID: menuID, Label: it.P1Label})
		}
		menu := &d.Menus[len(d.Menus)-1]

		if len(menu.Groups) == 0 || menu.Groups[len(menu.Groups)-1].ID != groupID {
			menu.Groups = append(menu.Groups, driver.SettingGroup{ID: groupID, Label: it.P2Label})
		}
		group := &menu.Groups[len(menu.Groups)-1]

		group.Items = append(group.Items, driver.SettingItem{
			ID:      it.Addr.Wire(),
			Label:   it.Name,
			Display: fmt.Sprintf("%02d-%02d-%02d", it.Addr.P1, it.Addr.P2, it.Addr.P3),
		})
	}

	return d
}

// SettingsDescriptor returns the FTdx10's radio-neutral settings
// descriptor: a defensive Clone() of the package-level tree
// buildSettingsDescriptor built once at package init. Every call returns an
// independent copy — see driver.SettingsDescriptor.Clone's doc comment for
// why that independence is load-bearing.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ftdx10SettingsDescriptor.Clone()
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
// ftdx10.go — where core/driver/ft710 keeps its own — because the settings
// surface is one subject and reads better whole: the method is three lines
// of delegation to the descriptor built above it, and splitting it across
// files would put the capability's driver half out of sight of its session
// half for no gain.
func (d *ftdx10Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability (see that interface's doc comment) on the concrete *Session.
// Identical to the package-level SettingsDescriptor func and to the
// driver's StaticSettingsDescriptor, for the reason stated there.
//
// It deliberately does NOT consult s.dialect, even though the session
// carries one: the package-level tree is built from catDialect, the single
// dialect every session of this driver is opened with (ftdx10.go), and a
// per-session rebuild would cost 197 items of allocation per call to
// produce the identical answer. A future FTdx10 session whose menu surface
// genuinely varied — discovered, not declared — would rebuild here, and
// driver.StaticSettingsProvider's own contract already allows the two to
// disagree in that case.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError reports that ReadSetting's id argument does not name
// a known FTdx10 EX (MENU) address — refused BEFORE any wire traffic,
// exactly like ReadChannel's malformed-slot refusal (read.go, via
// cat.Dialect.ParseSlot's error path).
//
// This driver's OWN type in this driver's own namespace, like
// AnswerMismatchError (ftdx10.go): core/driver/ft710 has a same-shaped one
// and neither package imports the other, because a caller distinguishing
// which radio's settings read went wrong needs two distinct types.
type UnknownSettingError struct {
	// ID is the caller-supplied setting ID that did not parse.
	ID string
}

// Error implements the error interface.
func (e *UnknownSettingError) Error() string {
	return fmt.Sprintf("ftdx10: ReadSetting: %q is not a known FTdx10 setting ID", e.ID)
}

// SettingAnswerMismatchError reports that an EX answer named a DIFFERENT
// wire address than the one just requested.
//
// Deliberately a distinct type from this package's slot-worded
// *AnswerMismatchError (ftdx10.go): that type's fields and message are
// worded for memory-channel slots ("requested slot ... but the answer names
// slot ..."), and EX addresses are a structurally different namespace —
// six-digit P1P2P3 triples, never a cat.Slot — so reusing it would blur two
// unrelated wire-address kinds under one error shape.
//
// It carries NO errors.Is sentinel, where the slot-worded type does. A
// caller asking "did the radio answer about the wrong CHANNEL?" has a real
// errors.Is question, put to a branch a live read reaches; this one is
// reached only through the driver's own defence in depth — the engine's
// full-address correlation makes it unreachable on the real path (see
// parseEXResponse) — so errors.As on the concrete type is the whole
// interface it needs. core/driver/ft710's own setting-mismatch type carries
// no sentinel either.
type SettingAnswerMismatchError struct {
	// Requested is the six-digit wire address the read asked for.
	Requested string
	// Answered is the six-digit wire address the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *SettingAnswerMismatchError) Error() string {
	return fmt.Sprintf("ftdx10: ReadSetting: requested EX address %q but the answer names address %q — refusing to map a reply onto the wrong setting", e.Requested, e.Answered)
}

// exSpec is the transport spec for an EX read of addr.
//
// THE MATCH PREFIX carries the FULL SIX-DIGIT ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule, and
// cat.PrefixLenMatcher's own doc comment states it: EX shares
// its two-byte command prefix across every one of this dialect's 197
// inventory addresses, so a bare "EX" would let Engine.Do correlate a
// DIFFERENT address's still-in-flight answer (or an unsolicited push) as
// this read's own, and hand back one setting's value labelled as another's.
//
// The exact length is left 0 — VARIABLE LENGTH, and the deliberate
// contrast with mtSpec (read.go), which pins an exact length derived from
// the dialect's MT geometry. There is no single EX answer length to derive:
// the P4 body's width runs 1 to 12 bytes across this inventory (96 items at
// one digit, one Text item at twelve — cat.EXItem.Digits/Text), so only the
// prefix is checked and cat.Dialect.ParseEXAnswer applies the dialect's own
// derived 1..12 bound afterwards. Deriving a per-item exact length from Digits
// would be worse than useless: the FT-710's M8c sweep found the manual's
// Digits column WRONG for one of its own addresses (core/cat/ex.go's
// ParseEXAnswer notes it), no FTdx10 address has ever answered anything at
// all, and a spec that pinned an unobserved width would turn the radio's
// honest answer into a timeout.
//
// One retry: an EX read is idempotent, exactly mtSpec's rationale.
func exSpec(addr cat.EXAddress) transport.CommandSpec {
	return transport.CATReadSpec("EX"+addr.Wire(), 0, 1)
}

// parseEXResponse interprets the outcome of one EX exchange for requested:
//   - a rejection frame (cat.IsRejection) maps to
//     SettingValue{ID: requested.Wire(), State: SettingUnavailable}, with NO
//     error — the project's established "?;" -> empty-result rule, the same
//     one ReadChannel's empty-slot mapping follows (read.go);
//   - a well-formed EX answer naming requested's own address maps to
//     SettingKnown, with Raw the P4 body VERBATIM (no trim, no typed value
//     model — cat.Dialect.ParseEXAnswer's own policy, and see
//     buildSettingsDescriptor on why no value semantics appear anywhere on
//     this surface);
//   - a well-formed EX answer naming a DIFFERENT address is refused with
//     *SettingAnswerMismatchError;
//   - anything else — a frame cat.Dialect.ParseEXAnswer rejects — is that
//     parser's typed *cat.ParseError under a wrap adding the address, so
//     errors.As finds it, mirroring ReadChannel's own error-typing split
//     (read.go: the parser's verdict stays the parser's, the driver adds the
//     context the parser cannot know).
//
// A PURE function — no ctx, no *Session, no wire I/O — deliberately
// separated from ReadSetting's exchange so it can be unit-tested directly
// with hand-built frame values. That separation matters most for the
// WRONG-ADDRESS branch: exSpec's match prefix carries the complete
// six-digit address, so transport.Engine.Do can only ever return a frame
// that ALREADY matches that address as a successful answer — a genuinely
// differently-addressed reply fails Do's own matching and is counted as an
// unexpected frame instead of being handed back here. The branch is
// therefore, BY DESIGN, unreachable through the real Session.ReadSetting
// path; calling this helper directly with a hand-built mismatched frame is
// the only way to exercise it, and to prove the defence in depth works
// should that engine guarantee ever regress.
//
// ReadSetting reaches the other branches for real: a genuine rejection is
// reconstructed as the literal "?;" bytes (see ReadSetting) before being
// handed to this same function, so every response-interpretation rule lives
// in exactly one place.
func parseEXResponse(dialect cat.Dialect, requested cat.EXAddress, frame []byte) (driver.SettingValue, error) {
	id := requested.Wire()

	if cat.IsRejection(frame) {
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	}

	addr, raw, err := dialect.ParseEXAnswer(frame)
	if err != nil {
		return driver.SettingValue{}, fmt.Errorf("ftdx10: ReadSetting %s: %w", id, err)
	}
	if addr.Wire() != id {
		return driver.SettingValue{}, &SettingAnswerMismatchError{Requested: id, Answered: addr.Wire()}
	}

	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}

// rejectionFrameBytes is the literal "?;" NAK frame, reconstructed by
// ReadSetting from Engine.Do's cat.ErrRejected sentinel — see ReadSetting's
// doc comment for why.
var rejectionFrameBytes = []byte("?;")

// ReadSetting implements the optional driver.SettingsReader capability:
// reads one FTdx10 EX (MENU) setting by its opaque, radio-neutral id, which
// this driver mints as the setting's 6-digit EX wire address (see
// buildSettingsDescriptor).
//
// id is parsed via cat.Dialect.ParseEXAddress FIRST, entirely before any
// wire traffic: a failure — a malformed shape, or a syntactically
// well-formed address that is not a member of THIS dialect's inventory,
// which includes every (05,*,*) triple the EX grammar block names and the
// chart does not enumerate (core/cat/ftdx10/doc.go's header-vs-chart
// anomaly) — returns *UnknownSettingError and nothing is ever sent. That is
// ReadChannel's malformed-slot refusal shape exactly (read.go).
//
// NO OPERATION MUTEX, and this Session has none to take: one EX read is one
// wire exchange, so there is no gap between two frames of the same
// operation for a concurrent operation to land in, and transport.Engine
// already serialises the individual exchange. This is the same property the
// MT-only read and write paths have, and it is the whole reason this
// driver's Session differs from the FT-710's on this point — that radio's
// operations are two exchanges each (MR+MT, MW+MT), and its ReadSetting
// holds the opMu those operations need. See Session's doc comment
// (ftdx10.go) and doc.go: a future FTdx10 operation needing two frames
// needs an opMu with it, and this method would then have to take it too.
//
// Rejection mechanism: Engine.Do surfaces a "?;" reply as the
// cat.ErrRejected ERROR SENTINEL — detected here via errors.Is, exactly as
// ReadChannel detects an empty slot (read.go) — never as returned frame
// bytes; Do's own answer wait checks cat.IsRejection internally and
// converts a rejection straight into that sentinel before returning to its
// caller. ReadSetting reconstructs the canonical "?;" bytes from the
// sentinel and hands them to parseEXResponse exactly like a real answer
// frame, so that function remains the single place which interprets what a
// response means, for every outcome alike.
//
// What a rejection MEANS here is NOT this driver's assumption to record: a
// menu address is either in the dialect's inventory (and was therefore
// asked) or refused above, so a "?;" says the radio declined to report a
// setting it declares — reported as SettingUnavailable, a recorded fact
// about this exchange, and never guessed into a value.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	addr, err := s.dialect.ParseEXAddress(id)
	if err != nil {
		return driver.SettingValue{}, &UnknownSettingError{ID: id}
	}

	cmd, err := s.dialect.BuildEXRead(addr)
	if err != nil {
		// Unreachable in practice: ParseEXAddress above already enforced the
		// identical inventory membership BuildEXRead checks (both via
		// cat.Dialect.KnownEXAddress). Kept as defence in depth rather than
		// as a silent assumption that the two rules stay the same one.
		return driver.SettingValue{}, fmt.Errorf("ftdx10: ReadSetting %s: %w", addr.Wire(), err)
	}

	frame, err := s.eng.Do(ctx, cmd, exSpec(addr))
	switch {
	case errors.Is(err, cat.ErrRejected):
		frame = rejectionFrameBytes
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ftdx10: ReadSetting %s: %w", addr.Wire(), err)
	}

	return parseEXResponse(s.dialect, addr, frame)
}
