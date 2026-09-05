// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

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

// UnknownSettingError reports that ReadSetting's id argument does not name a
// known FT-891 EX (MENU) address — refused BEFORE any wire traffic, exactly
// like ReadChannel's malformed-slot refusal (read.go, via
// cat.Dialect.ParseSlot's error path).
//
// This driver's OWN type in this driver's own namespace, like
// AnswerMismatchError (ft891.go): each registered driver has a same-shaped
// one and none imports another, because a caller distinguishing which
// radio's settings read went wrong needs distinct types.
//
// It covers TWO refusals the caller cannot tell apart from the id alone and
// does not need to: a malformed shape (anything but four ASCII digits on
// this radio, so a sibling's six-digit ID lands here) and a well-formed
// four-digit address that is not a member of THIS dialect's inventory —
// 0104, say, which the grammar block's "0101 - 1803" range admits and the
// chart does not enumerate. Both mean the same thing to a caller: this radio
// has no such setting. Pinned by TestSession_ReadSetting_ErrorTyping.
type UnknownSettingError struct {
	// ID is the caller-supplied setting ID that did not parse.
	ID string
}

// Error implements the error interface.
func (e *UnknownSettingError) Error() string {
	return fmt.Sprintf("ft891: ReadSetting: %q is not a known FT-891 setting ID", e.ID)
}

// SettingAnswerMismatchError reports that an EX answer named a DIFFERENT
// wire address than the one just requested.
//
// Deliberately a distinct type from this package's slot-worded
// *AnswerMismatchError (ft891.go): that type's fields and message are worded
// for memory-channel slots ("requested slot ... but the answer names slot
// ..."), and EX addresses are a structurally different namespace — four
// digits here, never a cat.Slot — so reusing it would blur two unrelated
// wire-address kinds under one error shape.
//
// It carries NO errors.Is sentinel, where the slot-worded type carries
// ErrAnswerMismatch. A caller asking "did the radio answer about the wrong
// CHANNEL?" has a real errors.Is question, put to a branch a live read
// reaches; this one is reached only through the driver's own defence in
// depth — the engine's full-address correlation makes it unreachable on the
// real path (see parseEXResponse) — so errors.As on the concrete type is the
// whole interface it needs. Both halves are pinned by
// TestParseEXResponse_Table.
type SettingAnswerMismatchError struct {
	// Requested is the four-digit wire address the read asked for.
	Requested string
	// Answered is the four-digit wire address the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *SettingAnswerMismatchError) Error() string {
	return fmt.Sprintf("ft891: ReadSetting: requested EX address %q but the answer names address %q — refusing to map a reply onto the wrong setting", e.Requested, e.Answered)
}

// exSpec is the transport spec for an EX read of addr.
//
// THE MATCH PREFIX CARRIES THE FULL FOUR-DIGIT ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule, which cat.PrefixLenMatcher's
// own doc comment states: EX shares its two-byte command prefix across every
// one of this dialect's 159 inventory addresses, so a bare "EX" would let
// transport.Engine.Do correlate a DIFFERENT address's still-in-flight answer
// (or an unsolicited push) as this read's own, and hand back one setting's
// value labelled as another's.
//
// FOUR digits, where the FTdx10's and FTdx101's specs carry six: the width
// comes from the dialect (cat.Dialect.EXWire over this radio's
// cat.EXAddressPair form, layout 513-522) and is never written here.
//
// The exact length is left 0 — VARIABLE LENGTH, and the deliberate contrast
// with mtSpec (read.go), which pins an exact length derived from the
// dialect's MT geometry. There is no single EX answer length to derive: the
// P4 body's width runs 1 to 5 bytes across this inventory (cat.EXItem.Digits;
// the 5 is OTHER DISP and OTHER SHIFT, layout 595-596), so only the prefix is
// checked and cat.Dialect.ParseEXAnswer applies the dialect's own derived
// bound afterwards. Deriving a per-item exact length from Digits would be
// worse than useless: the FT-710's M8c sweep found the manual's Digits column
// WRONG for one of its own addresses (core/cat/ex.go's ParseEXAnswer notes
// it), no FT-891 has ever answered anything at all, and a spec that pinned an
// unobserved width would turn the radio's honest answer into a timeout.
//
// ONE RETRY, where mtSpec has none. An EX read is idempotent, which is
// mtSpec's rationale too — but mtSpec's RetryReads 0 is plan P11's decision
// about a command whose Read this manual's own Control Command List denies
// exists (doc.go, "The MT Read contradiction"), where a silent retry would
// double the frames sent to test a registered assumption. EX carries no such
// contradiction: its availability row reads `EX | MENU | O O O O` (layout
// 142). Pinned by TestExSpec_FullAddressPrefixAndVariableLength.
func exSpec(dialect cat.Dialect, addr cat.EXAddress) transport.CommandSpec {
	return transport.CATReadSpec("EX"+dialect.EXWire(addr), 0, 1)
}

// parseEXResponse interprets the outcome of one EX exchange for requested:
//   - a rejection frame (cat.IsRejection) maps to
//     SettingValue{ID: dialect.EXWire(requested), State: SettingUnavailable},
//     with NO error — the project's established "?;" -> empty-result rule,
//     the same one ReadChannel's empty-slot mapping follows (read.go);
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
// THE REJECTION MAPPING ADDS NO FIFTH READING OF THIS RADIO'S "?;", and
// that is worth saying on a radio where the unattributed NAK already carries
// four distinct meanings (matrix §3.8.1-§3.8.4, doc.go's register). Those
// four are INTERPRETATIONS — each says what the radio MEANT by declining —
// and each is registered as an assumption or refused as one. This is not an
// interpretation at all: a menu address is either a member of the dialect's
// inventory (and was therefore asked) or refused above, so a "?;" here says
// only that the radio declined to report a setting it declares.
// SettingUnavailable records that exchange and guesses nothing, which is why
// this file adds no register entry and doc.go's roll stays at SIXTEEN.
//
// A PURE function — no ctx, no *Session, no wire I/O — deliberately
// separated from ReadSetting's exchange so it can be unit-tested directly
// with hand-built frame values. That separation matters most for the
// WRONG-ADDRESS branch: exSpec's match prefix carries the complete
// four-digit address, so transport.Engine.Do can only ever return a frame
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
	id := dialect.EXWire(requested)

	if cat.IsRejection(frame) {
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	}

	addr, raw, err := dialect.ParseEXAnswer(frame)
	if err != nil {
		return driver.SettingValue{}, fmt.Errorf("ft891: ReadSetting %s: %w", id, err)
	}
	if answered := dialect.EXWire(addr); answered != id {
		return driver.SettingValue{}, &SettingAnswerMismatchError{Requested: id, Answered: answered}
	}

	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}

// rejectionFrameBytes is the literal "?;" NAK frame, reconstructed by
// ReadSetting from Engine.Do's cat.ErrRejected sentinel — see ReadSetting's
// doc comment for why.
var rejectionFrameBytes = []byte("?;")

// ReadSetting implements the other half of the optional
// driver.SettingsReader capability: reads one FT-891 EX (MENU) setting by
// its opaque, radio-neutral id, which this driver mints as the setting's
// four-digit EX wire address (see buildSettingsDescriptor).
//
// id is parsed via cat.Dialect.ParseEXAddress FIRST, entirely before any
// wire traffic: a failure — a malformed shape, or a well-formed four-digit
// address that is not a member of THIS dialect's inventory — returns
// *UnknownSettingError and nothing is ever sent. That is ReadChannel's
// malformed-slot refusal shape exactly (read.go).
//
// THE WHOLE EXCHANGE HOLDS s.opMu, and this Session differs from the
// FTdx10's on exactly that point. One EX read is ONE transport.Engine.Do
// call, and the engine already serialises an individual exchange, so the
// lock is not protecting the SETTING. It is protecting the CHANNEL: a memory
// or PMS ReadChannel on this radio is potentially TWO exchanges — the
// combined MT read, then the cross-check's MR (read.go, matrix §3.5, plan
// P3) — and a settings read that could land in that gap would leave the
// cross-check interpreting an MT rejection against radio state a third frame
// had been sent into. The FTdx10's ReadSetting takes no lock because that
// driver's Session has none to take; core/driver/ft710's does, for this same
// reason, and its comment says so. Pinned by
// TestReadSetting_CannotInterleaveWithACrossCheck.
//
// Rejection mechanism: Engine.Do surfaces a "?;" reply as the
// cat.ErrRejected ERROR SENTINEL — detected here via errors.Is, exactly as
// ReadChannel detects an empty slot (read.go) — never as returned frame
// bytes; Do's own answer wait checks cat.IsRejection internally and converts
// a rejection straight into that sentinel before returning to its caller.
// ReadSetting reconstructs the canonical "?;" bytes from the sentinel and
// hands them to parseEXResponse exactly like a real answer frame, so that
// function remains the single place which interprets what a response means,
// for every outcome alike — including what this radio's "?;" does NOT mean
// here, which parseEXResponse states.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	addr, err := s.dialect.ParseEXAddress(id)
	if err != nil {
		return driver.SettingValue{}, &UnknownSettingError{ID: id}
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	cmd, err := s.dialect.BuildEXRead(addr)
	if err != nil {
		// Unreachable in practice: ParseEXAddress above already enforced the
		// identical inventory membership BuildEXRead checks (both via
		// cat.Dialect.KnownEXAddress). Kept as defence in depth rather than
		// as a silent assumption that the two rules stay the same one.
		return driver.SettingValue{}, fmt.Errorf("ft891: ReadSetting %s: %w", s.dialect.EXWire(addr), err)
	}

	frame, err := s.eng.Do(ctx, cmd, exSpec(s.dialect, addr))
	switch {
	case errors.Is(err, cat.ErrRejected):
		frame = rejectionFrameBytes
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ft891: ReadSetting %s: %w", s.dialect.EXWire(addr), err)
	}

	return parseEXResponse(s.dialect, addr, frame)
}
