// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

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
// A DIFFERENT string from every sibling's, necessarily: the five descriptors
// describe five radios' menus, so a snapshot taken from one must never
// validate against another. The "@1" is this shape's own generation — a
// later change to how THIS driver builds its tree increments it here alone.
// Pinned by TestSettingsDescriptor_IsTheDeclaredFlatFallback, which also
// checks it against the four sibling strings.
const settingsDescriptorVersion = "ft991a-ex@1"

// flatMenuID is the ID and the Label of the ONE menu and the ONE group this
// descriptor carries: the manual's own heading for the command, layout 519,
// where the line reads `EX          MENU`.
//
// ONE CONSTANT FOR FOUR FIELDS, and that is the claim rather than a saving:
// the four are the same name because this radio's chart gives the project
// exactly one name for its menu surface, and writing it four times would
// invite three of them to drift into invented structure. flatShapeErr
// (settings_test.go) is the assertion that holds all four to it.
const flatMenuID = "MENU"

// ft991aSettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries (catDialect.EXItems, generated
// from core/cat/ft991a/table2.csv) — see buildSettingsDescriptor.
//
// Every getter — the package-level SettingsDescriptor func, the driver's
// StaticSettingsDescriptor and the session's SettingsDescriptor — returns a
// Clone() of this and never the value itself: nothing outside this file may
// ever hold a reference to the shared original, because a caller that
// mutated the tree it was handed would silently change what every later
// caller received (driver.SettingsDescriptor.Clone's own doc comment).
var ft991aSettingsDescriptor = buildSettingsDescriptor(catDialect)

// buildSettingsDescriptor builds the FT-991A's driver.SettingsDescriptor
// from dialect.EXItems(): ONE SettingMenu, ONE SettingGroup inside it, both
// carrying flatMenuID as ID and as Label, and one SettingItem per inventory
// row IN INVENTORY ORDER.
//
// THE FLAT SHAPE IS A CHOICE FORCED BY A MANUAL FACT (matrix §3.9, spec
// decision 6, plan P9), AND THIS RADIO HAS LESS TO FALL BACK ON THAN ANY
// SIBLING. Its menu chart's columns are `P1 | Function | P2 | Digits`
// (layout 530) with NO GROUP LABEL COLUMNS — so the registered extable
// profile declares LabelsAbsent and every EXItem's P1Label and P2Label is ""
// (core/cat/ft991a/exinventory_gen.go's header says so in terms) — and its
// MENU Number is one flat three-digit run with NO SUBSTRUCTURE AT ALL, where
// the FT-891's four digits at least decompose into a two-digit prefix that
// driver partitions on. A driver.SettingsDescriptor needs a non-empty Label
// at every level and this project does not invent group names, so the
// descriptor falls back to the manual's own command heading, `EX MENU`
// (519). The group exists because the neutral type is a two-level tree and
// Validate requires at least one group per menu, NOT because this radio has
// a subgroup there.
//
// THE STRONGEST EVIDENCE FOR FLAT IS NOT THAT HEADING BUT THE PAGE LEDGER,
// and it is evidence rather than documentation (plan P9, rev 3). That leg's
// record is per PDF page, not per group — three rows, 42 + 82 + 29 = 153 —
// and each row's visual anchor records, independently of the plan and of the
// spec, that the chart is FLAT, that the 042→043 seam is carried by the page
// break alone and by no ruled boundary, that every rule measures 5-6 px at
// 600 dpi on a uniform pitch with none thicker, and that no heading row or
// prefix change interrupts them. A leg that could not see the question
// answered it.
//
// Pinned by TestSettingsDescriptor_IsTheDeclaredFlatFallback, whose
// empty-label half fails if a later transcription ever gives this inventory
// real names, and by TestSettingsDescriptor_TwoMenusFailTheShapeAssertion,
// which is the red proof that ONE menu is a DECLARED FALLBACK rather than an
// accident: a hand-built two-menu descriptor for this model passes
// driver.SettingsDescriptor.Validate and fails the shape assertion, so an
// "improvement" that invented groups cannot land silently.
//
// SettingItem.Display is the printed MENU Number, which on this radio is the
// SAME three digits as the ID. THAT THE TWO COINCIDE IS A FACT ABOUT THIS
// CHART, NOT A RULE — the FTdx10's prints a "%02d-%02d-%02d" triple whose
// Display and ID genuinely differ — so they are built as two separate
// expressions below and TestSettingsDescriptor_ItemsAreTheInventoryInOrder
// states the coincidence, so that nobody later "de-duplicates" one into the
// other and silently makes the ID the display form of whatever address shape
// comes next.
//
// 152 ITEMS FOR A CHART PRINTING 153 ROWS. The count is the dialect's, never
// a literal: row 087 RADIO ID prints ten hyphens for its parameter and a
// single hyphen for its Digits (layout 623), so the chart gives it no width
// and no parameter and the inventory excludes it BY ADDRESS. This is the
// driver-side site of the dialect register's entry ROW 087 RADIO ID'S
// EXCLUSION, and it is the site that makes the exclusion USER-VISIBLE,
// because this is the count a viewer shows.
//
// A THREE-DIGIT ID IS STORABLE ONLY BECAUSE OF THE KENWOOD BASE DEPENDENCY:
// core/codeplug/menus.go's isSettingIDWidth admits exactly three, four or
// six ASCII digits, and this is the first radio in the fleet to use the
// three-digit arm. PADDING A MENU NUMBER TO FOUR DIGITS — printing an ID no
// FT-991A document contains — IS FORBIDDEN (plan P9).
// TestCloneReadSettings_WalksTheWholeDescriptor is where that rule bites, but
// NOT through the gate the plan and matrix §3.9 name: core/clone/settings.go
// probes an all-MenuUnsupported snapshot BEFORE any wire exchange, and that
// preflight refuses every width core/codeplug's isSettingIDWidth rejects
// (five, seven, …) — but FOUR IS A LEGAL SNAPSHOT WIDTH, so a four-digit pad
// sails through the preflight and is refused one layer down, by THIS
// DRIVER'S OWN ParseEXAddress. Zero frames either way, so the safety claim
// holds; the "zero frames" guarantee for a four-digit pad is the driver's,
// not the preflight's (P9 erratum, orchestrator's to record).
//
// DIALECT-PARAMETERISED THROUGHOUT, which is what lets this be another
// instance of the sibling template rather than a copy of its output: the
// item count, every label and the address width come from the dialect
// argument. Nothing about the FT-991A's own numbers is written into this
// function — they are properties of the inventory it is handed, asserted in
// settings_test.go against the dialect and the registered profile rather
// than against literals here.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an
// address, a name and a display form per item; it does not carry an item's
// value legend, its units, its enumerated options or its default, and
// ReadSetting below returns the P4 body verbatim — this chart prints that
// field as P2 (layout 520); core/cat names it P4 fleet-wide, after the
// FT-710's grammar (core/cat/ft991a/table2.csv:41 records the terms clash).
// Said once, here; every other occurrence below keeps the fleet name. That is
// why core/cat/ft991a/doc.go's recorded CHART PRINTING DEFECTS do not bite this
// surface: every one of them lives in a value legend, and this driver
// interprets no legend (matrix §3.9). They become questions the moment a
// caller tries to render a menu value as a MEANING rather than as the bytes
// the radio sent, and that is deliberately not this file's business.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	group := driver.SettingGroup{ID: flatMenuID, Label: flatMenuID}
	for _, it := range dialect.EXItems() {
		group.Items = append(group.Items, driver.SettingItem{
			ID:      dialect.EXWire(it.Addr),
			Label:   it.Name,
			Display: dialect.EXWire(it.Addr),
		})
	}

	return driver.SettingsDescriptor{
		Version: settingsDescriptorVersion,
		Menus: []driver.SettingMenu{
			{ID: flatMenuID, Label: flatMenuID, Groups: []driver.SettingGroup{group}},
		},
	}
}

// SettingsDescriptor returns the FT-991A's radio-neutral settings
// descriptor: a defensive Clone() of the package-level tree
// buildSettingsDescriptor built once at package init. Every call returns an
// independent copy — see driver.SettingsDescriptor.Clone's doc comment for
// why that independence is load-bearing.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ft991aSettingsDescriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to
// Session.SettingsDescriptor. Identical to both it and the package-level
// func: this driver's settings tree depends only on the static EX
// inventory, never on anything a live session discovers — and on this radio
// that is stronger than on any sibling, because Open discovers NOTHING at
// all (matrix §3.4), so even Session.Capabilities folds in no per-session
// finding.
//
// It lives in THIS file rather than beside the driver's other methods in
// ft991a.go because the settings surface is one subject and reads better
// whole: the method is one line of delegation to the descriptor built above
// it, and splitting it across files would put the capability's driver half
// out of sight of its session half for no gain.
func (d *ft991aDriver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements half of the optional driver.SettingsReader
// capability (see that interface's doc comment) on the concrete *Session.
// Identical to the package-level SettingsDescriptor func and to the driver's
// StaticSettingsDescriptor, for the reason stated there.
//
// It deliberately does NOT consult s.dialect, even though the session
// carries one: the package-level tree is built from catDialect, the single
// dialect every session of this driver is opened with (caps.go), and a
// per-session rebuild would cost 152 items of allocation per call to produce
// the identical answer. A future FT-991A session whose menu surface
// genuinely varied — discovered, not declared — would rebuild here, and
// driver.StaticSettingsProvider's own contract already allows the two to
// disagree in that case.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError reports that ReadSetting's id argument does not name a
// known FT-991A EX (MENU) address — refused BEFORE any wire traffic, exactly
// like ReadChannel's malformed-slot refusal (read.go, via
// cat.Dialect.ParseSlot's error path).
//
// This driver's OWN type in this driver's own namespace, like
// AnswerMismatchError (ft991a.go): each registered driver has a same-shaped
// one and none imports another, because a caller distinguishing which
// radio's settings read went wrong needs distinct types.
//
// It covers TWO refusals the caller cannot tell apart from the id alone and
// does not need to: a malformed shape (anything but three ASCII digits on
// this radio, so a sibling's four- or six-digit ID lands here) and a
// well-formed three-digit address that is not a member of THIS dialect's
// inventory. THE SECOND CLASS HAS A MEMBER NO SIBLING HAS: 087, a row the
// chart PRINTS and the inventory excludes by address (the dialect register's
// ROW 087 RADIO ID'S EXCLUSION). A user who counts the printed chart will
// ask for it, and the honest answer is that this radio has no setting this
// programme can read there — never an EX087; whose answer this codec could
// not size. Pinned by TestSession_ReadSetting_ErrorTyping.
type UnknownSettingError struct {
	// ID is the caller-supplied setting ID that did not parse.
	ID string
}

// Error implements the error interface.
func (e *UnknownSettingError) Error() string {
	return fmt.Sprintf("ft991a: ReadSetting: %q is not a known FT-991A setting ID", e.ID)
}

// SettingAnswerMismatchError reports that an EX answer named a DIFFERENT
// wire address than the one just requested.
//
// Deliberately a distinct type from this package's slot-worded
// *AnswerMismatchError (ft991a.go): that type's fields and message are
// worded for memory-channel slots ("requested slot ... but the answer names
// slot ..."), and EX addresses are a structurally different namespace —
// three digits here, never a cat.Slot — so reusing it would blur two
// unrelated wire-address kinds under one error shape.
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
	// Requested is the three-digit wire address the read asked for.
	Requested string
	// Answered is the three-digit wire address the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *SettingAnswerMismatchError) Error() string {
	return fmt.Sprintf("ft991a: ReadSetting: requested EX address %q but the answer names address %q — refusing to map a reply onto the wrong setting", e.Requested, e.Answered)
}

// exSpec is the transport spec for an EX read of addr.
//
// THE MATCH PREFIX CARRIES THE FULL THREE-DIGIT ADDRESS, never the bare "EX"
// command name — the shared-prefix-family rule, which cat.PrefixLenMatcher's
// own doc comment states: EX shares its two-byte command prefix across every
// one of this dialect's 152 inventory addresses, so a bare "EX" would let
// transport.Engine.Do correlate a DIFFERENT address's still-in-flight answer
// (or an unsolicited push) as this read's own, and hand back one setting's
// value labelled as another's.
//
// THREE digits, the narrowest EX read in this fleet — the whole frame is six
// bytes against the FT-891's seven and the FTdx10's nine. The width comes
// from the dialect (cat.Dialect.EXWire over this radio's cat.EXAddressSingle
// form, the grammar block's "P1 : 001 - 153 (MENU Number)" at layout 520)
// and is never written here.
//
// The exact length is left 0 — VARIABLE LENGTH, and the deliberate contrast
// with mtSpec (read.go), which pins an exact length derived from the
// dialect's MT geometry. There is no single EX answer length to derive: the
// P4 body's width runs 1 to 8 bytes across this inventory (cat.EXItem.Digits;
// the 8 is ONE row, 151 PRESET FREQUENCY at layout 692, and it is why this
// radio's extable profile declares MaxDigits 8 where the other four declare
// 4, 4, 4 and 5), so only the prefix is checked and cat.Dialect.ParseEXAnswer
// applies the dialect's own derived bound afterwards. Deriving a per-item
// exact length from Digits would be worse than useless: the FT-710's M8c
// sweep found the manual's Digits column WRONG for one of its own addresses
// (core/cat/ex.go's ParseEXAnswer notes it), no FT-991A has ever answered
// anything at all, and a spec that pinned an unobserved width would turn the
// radio's honest answer into a timeout.
//
// ONE RETRY, where mtSpec has none. An EX read is idempotent, and this
// command carries none of the doubt the MT read does: its availability row
// is `EX | MENU | O O O O` (layout 155) and its detail block prints all
// three charts filled (519-528), where MT's Read is a chart this project
// reads against a Control Command List (doc.go). Pinned by
// TestExSpec_FullAddressPrefixAndVariableLength.
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
// THE REJECTION MAPPING ADDS NO SECOND READING OF THIS RADIO'S "?;". The
// driver register's MT "?;" ON A MEMORY OR PMS SLOT MEANS THE SLOT IS EMPTY
// is an INTERPRETATION — it says what the radio MEANT by declining — and is
// registered as an assumption for that reason. This is not an interpretation
// at all: a menu address is either a member of the dialect's inventory (and
// was therefore asked) or refused above, so a "?;" here says only that the
// radio declined to report a setting it declares. SettingUnavailable records
// that exchange and guesses nothing, which is why this file adds no register
// entry and doc.go's own roll stays at ELEVEN.
//
// A PURE function — no ctx, no *Session, no wire I/O — deliberately
// separated from ReadSetting's exchange so it can be unit-tested directly
// with hand-built frame values. That separation matters most for the
// WRONG-ADDRESS branch: exSpec's match prefix carries the complete
// three-digit address, so transport.Engine.Do can only ever return a frame
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
		return driver.SettingValue{}, fmt.Errorf("ft991a: ReadSetting %s: %w", id, err)
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

// readSettingGapHook, when non-nil, is called by ReadSetting after its EX
// exchange has completed and BEFORE the operation returns — that is, with
// s.opMu still held.
//
// It exists for exactly one reason: to make the operation mutex's claim
// CONSTRUCTIBLE. transport.Engine.Do holds the engine's own mutex for its
// whole body, retries included, so every operation this driver performs is
// already atomic AS AN EXCHANGE and no test could tell opMu's presence from
// its absence — which is what the task 11 review found when it deleted
// WriteChannel's lock and the whole package stayed green under -race. What
// opMu claims is larger: that a whole DRIVER OPERATION excludes another,
// including any work the operation does outside its Do call. This hook is
// how a test creates such work deterministically, rather than by hammering,
// for the reason TestReadChannel_ConcurrentReadsDoNotCrossAnswers records —
// Go's sync.Mutex favours an immediately-re-locking goroutine so heavily
// that the interleaving is near-impossible to reproduce by scheduling luck.
//
// THE WINDOW IS SYNTHETIC AND THE EXCLUSION IS NOT: the hook runs under
// opMu because this method holds it, so what a concurrent operation is
// blocked by is the lock and nothing else. It is a plain package var, unread
// in production (nil there), and it is the same shape
// core/driver/ft891/read.go's readChannelGapHook and core/driver/ft710's
// take. Pinned by TestReadSetting_HoldsOpMuAgainstAConcurrentWrite and
// TestReadSetting_IsAtomicUnderOpMu.
var readSettingGapHook func()

// ReadSetting implements the other half of the optional
// driver.SettingsReader capability: reads one FT-991A EX (MENU) setting by
// its opaque, radio-neutral id, which this driver mints as the setting's
// three-digit EX wire address (see buildSettingsDescriptor).
//
// id is parsed via cat.Dialect.ParseEXAddress FIRST, entirely before any
// wire traffic: a failure — a malformed shape, or a well-formed three-digit
// address that is not a member of THIS dialect's inventory, 087 included —
// returns *UnknownSettingError and nothing is ever sent. That is
// ReadChannel's malformed-slot refusal shape exactly (read.go).
//
// THE WHOLE EXCHANGE HOLDS s.opMu (spec erratum S-E4, matrix M-E2). One EX
// read is ONE transport.Engine.Do call and the engine already serialises an
// individual exchange, so the lock is not protecting the SETTING; it is what
// makes a whole DRIVER OPERATION exclude another, which is a claim about
// this session and not about the engine. On this radio the read is one
// exchange too, so there is no cross-check gap for a settings read to land
// inside — the FT-891's reason, and not available here — which is precisely
// why the property had to be pinned through readSettingGapHook rather than
// through a naturally multi-frame operation. Pinned by
// TestReadSetting_HoldsOpMuAgainstAConcurrentWrite (against a concurrent
// WriteChannel; red on the deletion of EITHER lock) and
// TestReadSetting_IsAtomicUnderOpMu (against a second settings read).
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
//
// A TIMEOUT IS THE TRANSPORT'S OWN ERROR under this path's ordinary
// address-naming wrap and is NOT re-typed, the same ruling read.go's MT
// timeout follows (doc.go): errors.Is finds transport.ErrTimeout and
// errors.As finds no driver type standing between them. exSpec retries once
// before that point, where mtSpec does not retry at all.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	addr, err := s.dialect.ParseEXAddress(id)
	if err != nil {
		return driver.SettingValue{}, &UnknownSettingError{ID: id}
	}

	// Held for the WHOLE operation — see the doc comment and the Session
	// type's.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	cmd, err := s.dialect.BuildEXRead(addr)
	if err != nil {
		// Unreachable in practice: ParseEXAddress above already enforced the
		// identical inventory membership BuildEXRead checks (both via
		// cat.Dialect.KnownEXAddress). Kept as defence in depth rather than
		// as a silent assumption that the two rules stay the same one.
		return driver.SettingValue{}, fmt.Errorf("ft991a: ReadSetting %s: %w", s.dialect.EXWire(addr), err)
	}

	frame, err := s.eng.Do(ctx, cmd, exSpec(s.dialect, addr))

	if readSettingGapHook != nil {
		readSettingGapHook()
	}

	switch {
	case errors.Is(err, cat.ErrRejected):
		frame = rejectionFrameBytes
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ft991a: ReadSetting %s: %w", s.dialect.EXWire(addr), err)
	}

	return parseEXResponse(s.dialect, addr, frame)
}
