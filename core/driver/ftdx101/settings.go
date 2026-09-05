// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

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
// ONE STRING FOR BOTH MODELS, and that is a decision (M9d-2 plan D4) rather
// than an omission. What a descriptor version identifies is a MENU TREE, and
// the FTDX101D's and the FTDX101MP's are the same tree: they are generated
// from one Table 2, whose 193 rows the manual prints once for both radios,
// and the three places the manual does distinguish the models (matrix §4)
// touch nothing this tree carries — the ID answer is not a menu item, the PC
// command is not a menu item, and the three MAX POWER rows differ only in a
// P4 VALUE LEGEND, which this surface reads no part of (see
// buildSettingsDescriptor). Model identity lives in Capabilities.Model and
// Capabilities.CATID, where it is a per-model fact with per-model evidence;
// putting it in the descriptor version as well would mint two version
// strings for one tree and make a snapshot taken from a D fail to validate
// against an MP for no reason on the wire.
//
// A DIFFERENT string from the FTdx10's "ftdx10-ex@1" and the FT-710's
// "ft710-ex@1", necessarily: those describe different radios' menus (197
// items there, 296 in the FT-710's, 193 here), so a snapshot taken from one
// must never validate against another. The "@1" is this shape's own
// generation — a later change to how THIS driver builds its tree increments
// it here alone.
const settingsDescriptorVersion = "ftdx101-ex@1"

// ftdx101SettingsDescriptor is built ONCE, at package init, from the EX
// inventory core/cat/ftdx101 carries (generated from that package's
// table2.csv) — see buildSettingsDescriptor.
//
// BUILT FROM modelD's DIALECT, AND THE CHOICE IS IMMATERIAL BY EVIDENCE, not
// by hope: core/cat/ftdx101 builds both dialects over ONE config from ONE
// generated inventory (its newDialect), the ledger's applicability
// attestation records that no stored EXItem property is model-conditional,
// and TestSettingsDescriptor_IsModelUnconditional rebuilds the whole tree
// from modelMP's dialect and requires it EQUAL. Should the two inventories
// ever diverge, that test fails loudly — which is the point of building from
// one and asserting against the other rather than building two trees that
// would then share one version string and disagree in silence.
//
// This is the ONE place in this package where a value is taken from a
// package-level model rather than from the driver or session that a call
// arrived on, and it is deliberate. Elsewhere that rule is load-bearing
// because the value is a CODEC — a builder, a parser, a gate — and a session
// encoding with one radio's dialect while its transport gated with the
// other's would differ from a correct one only in what the ID probe had
// accepted (Session's doc comment, ftdx101.go). A settings TREE is not a
// codec: it is a static inventory shape, identical for the two models, and
// there is no per-model answer for a getter to get wrong. ReadSetting, which
// IS a codec call, goes through s.dialect like every other one.
//
// Every getter — the package-level SettingsDescriptor func, the driver's
// StaticSettingsDescriptor and the session's SettingsDescriptor — returns a
// Clone() of this and never the value itself: nothing outside this file may
// ever hold a reference to the shared original, because a caller that
// mutated the tree it was handed would silently change what every later
// caller received, INCLUDING a caller holding the other model's driver
// (driver.SettingsDescriptor.Clone's own doc comment;
// TestSettingsDescriptor_IsADefensiveCopy's sibling leg).
var ftdx101SettingsDescriptor = buildSettingsDescriptor(modelD.dialect)

// buildSettingsDescriptor builds the FTdx101's driver.SettingsDescriptor
// from dialect.EXItems(): one SettingMenu per distinct P1 (ID the 2-digit
// decimal P1, e.g. "01"; Label the manual's P1 column), one SettingGroup
// per distinct (P1,P2) pair nested under its menu (ID the 4-digit P1P2,
// e.g. "0101"; Label the manual's P2 column), and one SettingItem per
// inventory row nested under its group, IN INVENTORY ORDER (ID the item's
// 6-digit wire address via cat.Dialect.EXWire, Label the manual's Function
// name, Display the human "P1-P2-P3" form, e.g. "01-01-01").
//
// DIALECT-PARAMETERISED THROUGHOUT, which is what lets it be a shape this
// project has built before rather than a copy of that build's output: the
// item count, the menu and group partition, every label and every width come
// from the dialect argument. Nothing about the FTdx101's own numbers (193
// items, P1 in {01,02,03,04}, 18 groups — matrix §3.9) is written into this
// function; they are properties of the inventory it is handed, asserted in
// settings_test.go against the dialect rather than against literals alone.
// It is also what lets the same function be handed the OTHER model's dialect
// in a test and prove the two trees identical.
//
// Dialect.EXItems returns its rows already sorted by (P1,P2,P3) (its own doc
// comment, and core/cat/ftdx101's generated inventory is emitted in that
// order), so a single linear pass — opening a new menu or group only when
// the running P1/P2 changes from the PREVIOUS item — reproduces the manual's
// own chart grouping with no extra sorting or map bookkeeping. Each
// menu/group pointer used to append is re-derived fresh, by index, on every
// iteration and never carried across one: that sidesteps any question of
// whether an earlier append's slice growth could invalidate a held pointer,
// at the cost of one slice index per item — a non-issue at 193 items, run
// once at package init.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an
// address, two labels and a human display form per item; it does not carry
// an item's value legend, its units, its enumerated options or its default,
// and ReadSetting below returns the P4 body verbatim. That is why
// core/cat/ftdx101/doc.go's recorded CHART PRINTING DEFECTS — AM MAX POWER's
// self-inconsistent MP arm, CW BK-IN DELAY's truncated legend, SHIFT
// FREQUENCY's duplicate index 1 and MARK FREQUENCY's missing index 0, DECODE
// AFC RANGE's non-monotonic legend, KEYBOARD LANGUAGE's twelfth entry "11:
// LEVEL", QSK DELAY TIME's "mesc" — do not bite this surface: every one of
// them lives in a value legend, and this driver interprets no legend. They
// become questions the moment a caller tries to render a menu value as a
// MEANING rather than as the bytes the radio sent, and that is deliberately
// not this file's business (M9d-1's boundary; matrix §3.9).
//
// IT IS ALSO WHY THE SURFACE IS MODEL-UNCONDITIONAL. The only three Table 2
// rows this manual prints differently for the D and the MP — (03,04,01) HF
// MAX POWER, (03,04,02) 50M MAX POWER, (03,04,04) AM MAX POWER — differ in
// their P4 VALUE range and in nothing else: same address, same P1/P2 labels,
// same function name, same Digits width, same text flag (matrix §4, entry
// 2). Every field this tree carries is therefore printed once for both
// radios.
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
			ID:      dialect.EXWire(it.Addr),
			Label:   it.Name,
			Display: fmt.Sprintf("%02d-%02d-%02d", it.Addr.P1, it.Addr.P2, it.Addr.P3),
		})
	}

	return d
}

// SettingsDescriptor returns the FTdx101's radio-neutral settings
// descriptor: a defensive Clone() of the package-level tree
// buildSettingsDescriptor built once at package init. Every call returns an
// independent copy — see driver.SettingsDescriptor.Clone's doc comment for
// why that independence is load-bearing.
//
// A BARE, MODEL-FREE FUNCTION, in a package that deliberately offers no bare
// New and whose dialect package deliberately offers no bare Dialect(). That
// is not an inconsistency, it is the claim: there is no bare New because a
// driver IS one model or the other and a default would silently pick a side,
// and there is exactly one settings tree because this surface provably is
// not model-conditional (settingsDescriptorVersion; matrix §4). A
// SettingsDescriptorD/SettingsDescriptorMP pair would be two names for one
// value, and the day they were built from two dialects that had diverged
// they would still be carrying one version string.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ftdx101SettingsDescriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to
// Session.SettingsDescriptor. Identical to both it and the package-level
// func, FOR EITHER MODEL: this driver's settings tree depends only on the
// static EX inventory the two models share, never on anything a live session
// discovers (unlike Session.Capabilities, which folds in per-session 5xx/EMG
// discovery), so all three call sites — and both registrations — return
// equal trees.
//
// It lives in THIS file rather than beside the driver's other methods in
// ftdx101.go — where core/driver/ft710 keeps its own — because the settings
// surface is one subject and reads better whole: the method is three lines
// of delegation to the descriptor built above it, and splitting it across
// files would put the capability's driver half out of sight of its session
// half for no gain.
func (d *ftdx101Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability (see that interface's doc comment) on the concrete *Session.
// Identical to the package-level SettingsDescriptor func and to the driver's
// StaticSettingsDescriptor, for the reason stated there.
//
// It deliberately does NOT consult s.dialect, even though the session
// carries one and every codec call in this package does. The tree is the
// same for the D and the MP (ftdx101SettingsDescriptor's doc comment on why
// that is evidence rather than assumption), so a per-session rebuild would
// cost 193 items of allocation per call to produce the identical answer for
// either model. A future FTdx101 session whose menu surface genuinely varied
// — discovered, not declared — would rebuild here, and
// driver.StaticSettingsProvider's own contract already allows the two to
// disagree in that case.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError reports that ReadSetting's id argument does not name a
// known FTdx101 EX (MENU) address — refused BEFORE any wire traffic, exactly
// like ReadChannel's malformed-slot refusal (read.go, via
// cat.Dialect.ParseSlot's error path).
//
// This driver's OWN type in this driver's own namespace, like
// AnswerMismatchError (ftdx101.go): core/driver/ft710 and core/driver/ftdx10
// have same-shaped ones and no two of the three packages import each other,
// because a caller distinguishing WHICH radio's settings read went wrong
// needs distinct types.
//
// It names no MODEL, only the family. The refusal is a statement about the
// shared inventory — the address is not a member of the Table 2 both radios
// are printed from — so wording it "the FTDX101D does not have this setting"
// would claim a per-model fact this package has no evidence for, and would
// mislead a user of the other radio into thinking their own might.
type UnknownSettingError struct {
	// ID is the caller-supplied setting ID that did not parse.
	ID string
}

// Error implements the error interface.
func (e *UnknownSettingError) Error() string {
	return fmt.Sprintf("ftdx101: ReadSetting: %q is not a known FTdx101 setting ID", e.ID)
}

// SettingAnswerMismatchError reports that an EX answer named a DIFFERENT
// wire address than the one just requested.
//
// Deliberately a distinct type from this package's slot-worded
// *AnswerMismatchError (ftdx101.go): that type's fields and message are
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
// interface it needs.
type SettingAnswerMismatchError struct {
	// Requested is the six-digit wire address the read asked for.
	Requested string
	// Answered is the six-digit wire address the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *SettingAnswerMismatchError) Error() string {
	return fmt.Sprintf("ftdx101: ReadSetting: requested EX address %q but the answer names address %q — refusing to map a reply onto the wrong setting", e.Requested, e.Answered)
}

// exSpec is the transport spec for an EX read of addr.
//
// THE MATCH PREFIX carries the FULL SIX-DIGIT ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule, and
// cat.PrefixLenMatcher's own doc comment states it: EX shares
// its two-byte command prefix across every one of this dialect's 193
// inventory addresses, so a bare "EX" would let Engine.Do correlate a
// DIFFERENT address's still-in-flight answer (or an unsolicited push) as
// this read's own, and hand back one setting's value labelled as another's.
//
// The exact length is left 0 — VARIABLE LENGTH, and the deliberate
// contrast with mtSpec (read.go), which derives an exact length from the
// dialect's MT geometry. There is no single EX answer length to derive: the
// P4 body's width runs 1 to 12 bytes across this inventory (106 items at one
// digit, one Text item at twelve — cat.EXItem.Digits/Text), so only the
// prefix is checked and cat.Dialect.ParseEXAnswer applies the dialect's own
// derived 1..12 bound afterwards. Deriving a per-item exact length from Digits
// would be worse than useless here: NO FTdx101 OF EITHER MODEL HAS EVER
// ANSWERED ANYTHING — every inventory row carries the absence sentinels
// ObservedReadWidth 0 and ObservedReadShape "" — and the one radio in this
// project that has been swept found the manual's Digits column WRONG for one
// of its own addresses (the FT-710's M8c sweep; core/cat/ex.go's
// ParseEXAnswer notes it). A spec that pinned an unobserved width would turn
// this radio's honest answer into a timeout.
//
// One retry: an EX read is idempotent, exactly mtSpec's rationale.
func exSpec(dialect cat.Dialect, addr cat.EXAddress) transport.CommandSpec {
	return transport.CATReadSpec("EX"+dialect.EXWire(addr), 0, 1)
}

// parseEXResponse interprets the outcome of one EX exchange for requested:
//   - a rejection frame (cat.IsRejection) maps to
//     SettingValue{ID: dialect.EXWire(requested), State: SettingUnavailable}, with NO
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
// A PURE function — no ctx, no *Session, no wire I/O — and it takes the
// DIALECT as an argument rather than reaching for one, which in this
// two-model package is worth more than it was in the single-model one it is
// modelled on: a test can hand it either model's dialect and prove the
// verdicts identical, and no arm of it can quietly consult a package-level
// radio.
//
// The separation from ReadSetting's exchange matters most for the
// WRONG-ADDRESS branch: exSpec's match prefix carries the complete six-digit
// address, so transport.Engine.Do can only ever return a frame that ALREADY
// matches that address as a successful answer — a genuinely
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
		return driver.SettingValue{}, fmt.Errorf("ftdx101: ReadSetting %s: %w", id, err)
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

// ReadSetting implements the optional driver.SettingsReader capability:
// reads one FTdx101 EX (MENU) setting by its opaque, radio-neutral id, which
// this driver mints as the setting's 6-digit EX wire address (see
// buildSettingsDescriptor).
//
// id is parsed via cat.Dialect.ParseEXAddress FIRST, entirely before any
// wire traffic: a failure — a malformed shape, or a syntactically
// well-formed address that is not a member of THIS dialect's inventory,
// which includes every (05,*,*) triple the EX grammar block names at layout
// 700 and the chart does not enumerate (core/cat/ftdx101/doc.go's
// header-vs-chart anomaly, recorded UNRESOLVED because no FTdx101 has ever
// been asked) — returns *UnknownSettingError and nothing is ever sent. That
// is ReadChannel's malformed-slot refusal shape exactly (read.go), and it is
// what keeps this driver from inventing a question: membership follows the
// chart, so a P1=05 address is refused here rather than probed.
//
// NO OPERATION MUTEX, and this Session has none to take: one EX read is one
// wire exchange, so there is no gap between two frames of the same operation
// for a concurrent operation to land in, and transport.Engine already
// serialises the individual exchange. This is the same property the MT-only
// read and write paths have, and it is the whole reason this driver's
// Session differs from the FT-710's on this point — that radio's operations
// are two exchanges each (MR+MT, MW+MT), and its ReadSetting holds the opMu
// those operations need. See Session's doc comment (ftdx101.go) and doc.go:
// a future FTdx101 operation needing two frames needs an opMu with it, and
// this method would then have to take it too.
//
// Rejection mechanism: Engine.Do surfaces a "?;" reply as the
// cat.ErrRejected ERROR SENTINEL — detected here via errors.Is, exactly as
// ReadChannel detects an empty slot (read.go) — never as returned frame
// bytes; Do's own answer wait checks cat.IsRejection internally and converts
// a rejection straight into that sentinel before returning to its caller.
// ReadSetting reconstructs the canonical "?;" bytes from the sentinel and
// hands them to parseEXResponse exactly like a real answer frame, so that
// function remains the single place which interprets what a response means,
// for every outcome alike.
//
// What a rejection MEANS here is NOT this driver's assumption to record: a
// menu address is either in the dialect's inventory (and was therefore
// asked) or refused above, so a "?;" says the radio declined to report a
// setting it declares — reported as SettingUnavailable, a recorded fact
// about this exchange, and never guessed into a value. That the two models
// might decline different addresses is exactly the kind of thing a Stage R
// session would find out; it needs no code here, because the answer is
// reported per exchange rather than predicted per model.
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
		return driver.SettingValue{}, fmt.Errorf("ftdx101: ReadSetting %s: %w", s.dialect.EXWire(addr), err)
	}

	frame, err := s.eng.Do(ctx, cmd, exSpec(s.dialect, addr))
	switch {
	case errors.Is(err, cat.ErrRejected):
		frame = rejectionFrameBytes
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ftdx101: ReadSetting %s: %w", s.dialect.EXWire(addr), err)
	}

	return parseEXResponse(s.dialect, addr, frame)
}
