// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The compiler assertions this package's types owe the transport seam.
// Command is the one thing that crosses it, and shape alone would let a
// method be renamed without a failure.
var _ transport.Command = Command{}

// Layout is one MA-family registry row: the per-row data every operation in
// this package consults, and NOT a kw.Layout.
//
// NO ma.Layout IS A kw.Layout, AND THAT IS THE WHOLE SAFETY ARGUMENT
// (spec decision 1). kw.Layout describes the 50-byte MR/MW record — package
// constants RecordLen, recNameOff, recTermOff, a hardWiredPositions set
// requiring P10/P12/P13 at fixed offsets, an MRAnswerMatcher whose first act
// is a length equality against RecordLen, and an eight-opcode AllowedCommand
// admitting MR, MW, MC and TY. An MA0 record is 40-50 bytes on one row and 57
// on the other, has no hard-wired byte at all, and neither radio has any of
// those four commands. Making this a kw.Layout would mean loosening every one
// of those to a per-layout datum, re-opening pair 1's frozen goldens and its
// whole conformance suite for two radios that share none of the grid. The two
// types are therefore deliberately unrelated, and an ma.Layout reaching any
// core/kw API that takes a kw.Layout is a STOP rather than a judgement call.
//
// THE ZERO VALUE FAILS CLOSED: it builds nothing, parses nothing and admits
// nothing. Every axis below is explicit with a refusing default, so a
// forgotten field is a construction error rather than a quietly weaker
// layout. The values are minted ONCE at initialisation by mustNewLayout and
// handed out BY VALUE; the fields are unexported and the container accessors
// copy, so a caller holding what Layout890 or Layout990 returns cannot edit
// the radio.
//
// BOTH ROWS LIVE IN THIS PACKAGE rather than in core/kw/ma/ts890 and
// core/kw/ma/ts990, and that is the one place this design is deliberately
// smaller than pair 1's. core/kw/ts590 and core/kw/ts480 are separate
// packages because core/kw must stay radio-agnostic and its in-package test
// fixtures must not become real rows, and the cost of that separation was a
// whole extra package, kwtest, whose only reason to exist is that a model
// package importing core/kw cannot reach core/kw's in-package fixtures. With
// both layouts here, the conformance suite is an ordinary in-package walk
// over the two, no import cycle arises, and three packages are not created.
// Skipped: the kwtest shape; add it when a third MA radio needs a layout in a
// package of its own.
type Layout struct {
	book          kw.Book
	model         string
	catID         string
	slots         []kw.SlotRange
	modeNames     map[byte]string
	maxToneIndex  uint8
	maxCTCSSIndex uint8
	mainSub       bool
	exItems       []kw.EXItem
}

// layoutConfig is the axis set newLayout validates. It is unexported, as is
// the constructor, because the only two honest values are the two below: a
// caller outside this package selects a row by calling that row's own
// function, never by handing a config to one function that branches.
type layoutConfig struct {
	Book          kw.Book
	Model         string
	CATID         string
	Slots         []kw.SlotRange
	ModeNames     map[byte]string
	MaxToneIndex  uint8
	MaxCTCSSIndex uint8
	MainSub       bool
	EXItems       []kw.EXItem
}

// layout890 is the TS-890S row, minted once. See Layout890.
var layout890 = mustNewLayout(layout890Config())

// layout990 is the TS-990S row, minted once. See Layout990.
var layout990 = mustNewLayout(layout990Config())

// Layout890 returns the TS-890S row.
//
// THE NAME IS PART OF THIS PACKAGE'S CONTRACT, on core/kw/ts590's terms: a
// driver selects a row by calling the row's own function, never by passing a
// string to one function that branches. A branch would put the two rows one
// typo apart, which is the cross-model borrowing the Tier 4b sweep exists to
// forbid.
func Layout890() Layout { return layout890 }

// Layout990 returns the TS-990S row.
//
// It differs from Layout890 in the frame grid, the field count, the mode
// legend, the lockout encoding, the number of tone tuples and the Main/Sub
// band pointer — two grammars, not two rows of one grammar — and
// layout_test.go pins the axes this type carries in BOTH directions, so a
// value copied from one row to the other fails there rather than shipping.
func Layout990() Layout { return layout990 }

// layout890Config is the TS-890S's axes, each read off that book's own
// chart.
//
// IT IS A FUNCTION RATHER THAN A SHARED VARIABLE, on core/kw/ts590's
// reasoning: a package-level config copied and amended would share its
// ModeNames map and its Slots slice between the two values, and one mutation
// would then edit both radios. Every call builds fresh containers.
func layout890Config() layoutConfig {
	return layoutConfig{
		// The document this row speaks, which is what an "E;" or an "O;"
		// quotes its cause sentence from (890:118-123).
		Book:  kw.Book890,
		Model: "TS-890S",
		// "024: TS-890S" (890:2733) — printed WITH the model name beside
		// it, so this row's identity is self-describing. The probe compares
		// against this row's own value and never against a table of five.
		CATID: "024",
		// "000 ~ 119", with "Channels P0 ~ P9 are represented as 100 ~ 109
		// and channels E0 ~ E9 are represented as 110 ~ 119"
		// (890:3165-3169).
		//
		// THIS IS THE CODEC'S SLOT DOMAIN AND NOT THE DRIVER'S PUBLISHED
		// BANKS, exactly as core/kw/ts590 states the same division: the
		// domain admits what the book prints, and the driver publishes what
		// is confirmed. 100-119 are published in no bank of either row.
		//
		// kw.SlotScan NAMES THE 100-109 CLASS ON BOTH ROWS, AND THE TWO
		// BOOKS NAME IT DIFFERENTLY: "Programmable VFO" here
		// (890:3315-3319) against "Section defined Memory channel" on the
		// TS-990S (990:3051-3054), for a class with identical semantics.
		// That divergence is erratum E12 and it is RECORDED in doc.go
		// rather than encoded, because it is a naming divergence between
		// two books and not a difference between two radios.
		Slots: []kw.SlotRange{
			{Class: kw.SlotMemory, Lo: 0, Hi: 99},
			{Class: kw.SlotScan, Lo: 100, Hi: 109},
			{Class: kw.SlotExtension, Lo: 110, Hi: 119},
		},
		ModeNames: modeNames890(),
		// TN runs 00-50, index 50 being 1750.0 Hz (890:5149-5163, the 1750
		// entry at 890:5162). Index 99 is "To default" and is a SETTING
		// COMMAND ONLY (890:5163, 890:5166), so it is not a tone and is
		// never published or built.
		MaxToneIndex: 50,
		// CN runs 00-49 and prints NO index 50 at all (890:1354-1369), which
		// is why a Known tone_rx of 1750 Hz has no receive encoding and is
		// refused on write by the driver.
		MaxCTCSSIndex: 49,
		// This book gives no command a Main/Sub band pointer: MN, MV, OM, TN
		// and CN all carry their parameters without one (890:3647-3657,
		// 890:3781-3792, 890:3955-3975, 890:5143-5155, 890:1348-1360). OM
		// DOES have a P1 at those lines — but it selects a display area,
		// "0: Left-hand frequency display" / "1: Right-hand frequency
		// display" (890:3956, 890:3960-3972), not a Main/Sub band; the
		// 990S's own Main/Sub pointer is a different axis entirely, on the
		// SAME command (990:3699-3705, "0: Main Band / 1: Sub Band" —
		// already cited below for that row's OM P1).
		MainSub: false,
		EXItems: EXItems890S(),
	}
}

// layout990Config is the TS-990S's axes, each read off that book's own
// chart. A function, for layout890Config's reason.
func layout990Config() layoutConfig {
	return layoutConfig{
		// 990:118-121 is this book's own error-message table.
		Book:  kw.Book990,
		Model: "TS-990S",
		// "022" (990:2612) — printed BARE, with no model name beside it, so
		// this row's identity is a number this project binds to a name. That
		// binding is a project choice over a documentary fact, exactly as
		// core/driver/ts590 says of its own two rows.
		CATID: "022",
		// "000 ~ 119: Channel Number", with the same P0 ~ P9 / E0 ~ E9
		// mapping (990:2892-2896). See layout890Config's note on E12 for why
		// the 100-109 class carries the same kw.SlotClass on both rows while
		// the two books name it differently.
		Slots: []kw.SlotRange{
			{Class: kw.SlotMemory, Lo: 0, Hi: 99},
			{Class: kw.SlotScan, Lo: 100, Hi: 109},
			{Class: kw.SlotExtension, Lo: 110, Hi: 119},
		},
		ModeNames: modeNames990(),
		// TN runs 00-50, index 50 being 1750 (990:4960-4972, the 1750 entry
		// at 990:4971); index 99 is "Default", a setting command only
		// (990:4972, 990:4974).
		MaxToneIndex: 50,
		// CN runs 00-49 with no index 50 (990:1251-1264).
		MaxCTCSSIndex: 49,
		// THIS BOOK PUTS A MAIN/SUB POINTER ON FIVE COMMANDS — MN
		// (990:3325-3336), MV (990:3461-3474), OM P1 (990:3699-3705), TN P1
		// (990:4952-4954) and CN P1 (990:1241-1245) — and the TS-890S has one
		// nowhere. It is a COMMAND-GRAMMAR difference and not a record
		// difference: no byte of MA0 names a band on this radio either
		// (990:2891-2903 is the whole parameter list), which is why MN is off
		// the outbound roster in both directions. The axis is carried so that
		// a refusal or a driver-side note can say which grammar it is looking
		// at without re-deriving it from the model name.
		MainSub: true,
		EXItems: EXItems990S(),
	}
}

// modeNames890 is the OM P2 legend the TS-890S's MA0 P3 and P9 are read
// against (890:3976-3992), in the book's own spellings.
//
// MA0 CARRIES NO MODE LEGEND OF ITS OWN: P3 says "Refer to the P2 value of
// the OM command" (890:3174-3175) and P9 the same (890:3193-3195), so the
// memory mode vocabulary IS OM's. THERE IS NO MD COMMAND ON EITHER RADIO,
// which is the whole reason this legend is not core/kw's.
//
// VALUES 0 AND 8 ARE ABSENT, and their absence is the point rather than an
// omission. The chart prints both "Unused" (890:3977, 890:3985), so neither
// names a mode a channel can be in; newLayout refuses a legend that names
// either, and the codec refuses both bytes on read and never builds them.
func modeNames890() map[byte]string {
	return map[byte]string{
		'1': "LSB",
		'2': "USB",
		'3': "CW",
		'4': "FM",
		'5': "AM",
		'6': "FSK",
		'7': "CW-R",
		'9': "FSK-R",
		'A': "PSK",
		'B': "PSK-R",
		'C': "LSB-D",
		'D': "USB-D",
		'E': "FM-D",
		'F': "AM-D",
	}
}

// modeNames990 is the OM P2 legend the TS-990S's MA0 P4 and P10 are read
// against (990:3706-3730), in the book's own spellings.
//
// IT IS EIGHT VALUES LONGER THAN THE TS-890S'S AND ITS DATA MODES ARE
// NUMBERED. Where the 890S prints one data family — LSB-D, USB-D, FM-D, AM-D
// at C-F — this radio prints THREE, D1 at C-F, D2 at G-J and D3 at K-N
// (990:3719-3730). So the mode byte is not a hex nibble here: it is an
// alphanumeric character running '0'-'9' then 'A'-'N', and a codec that
// parsed it as hex would refuse two thirds of this radio's data modes.
//
// Values 0 and 8 are absent for modeNames890's reason, printed "Unused" at
// 990:3707 and 990:3715.
//
// KENWOOD SPELLS RTTY "FSK" ON BOTH RADIOS, which is the fact the CHIRP
// posture carries forward: a CHIRP row whose mode is RTTY names no mode
// either radio has.
func modeNames990() map[byte]string {
	return map[byte]string{
		'1': "LSB",
		'2': "USB",
		'3': "CW",
		'4': "FM",
		'5': "AM",
		'6': "FSK",
		'7': "CW-R",
		'9': "FSK-R",
		'A': "PSK",
		'B': "PSK-R",
		'C': "LSB-D1",
		'D': "USB-D1",
		'E': "FM-D1",
		'F': "AM-D1",
		'G': "LSB-D2",
		'H': "USB-D2",
		'I': "FM-D2",
		'J': "AM-D2",
		'K': "LSB-D3",
		'L': "USB-D3",
		'M': "FM-D3",
		'N': "AM-D3",
	}
}

// newLayout validates cfg and returns the Layout it describes, or an error
// wrapping kw.ErrLayoutInvalid.
//
// EVERY AXIS IS CHECKED, INCLUDING THE ONES BOTH ROWS AGREE ON. The point of
// a refusing default is that a forgotten field cannot become a quietly weaker
// layout, and the two rows are not the only configs this constructor will
// ever see: the tests spoil one axis at a time, and a third MA radio would
// arrive as a third config.
func newLayout(cfg layoutConfig) (Layout, error) {
	// THE BOOK TEST IS BY NAME, and it is the CONVERSE of core/kw's own.
	// kw.Book.valid() says "a document core/kw has READ", which is all four
	// books; core/kw's NewLayout narrows that to Book590 and Book480 by name
	// so that no 50-byte MR/MW layout can claim an MA document. This is the
	// same door in the other direction — an ma.Layout on Book590 would be an
	// MA codec claiming a book whose memory command is MR/MW — and the
	// message points the caller at where that record type does live.
	switch cfg.Book {
	case kw.Book890, kw.Book990:
	case kw.Book590, kw.Book480:
		return Layout{}, fmt.Errorf("%w: %v describes the 50-byte MR/MW memory record, which is core/kw's kw.Layout and not this package's MA0 record", kw.ErrLayoutInvalid, cfg.Book)
	default:
		return Layout{}, fmt.Errorf("%w: this layout names no PC-command document, and a layout that describes no radio may not be constructed (got %v)", kw.ErrLayoutInvalid, cfg.Book)
	}
	if cfg.Model == "" {
		return Layout{}, fmt.Errorf("%w: no model name, which every refusal message names", kw.ErrLayoutInvalid)
	}
	if err := validateCATID(cfg.CATID); err != nil {
		return Layout{}, err
	}
	if err := validateSlots(cfg.Slots); err != nil {
		return Layout{}, err
	}
	if err := validateModeNames(cfg.ModeNames); err != nil {
		return Layout{}, err
	}
	if cfg.MaxToneIndex == 0 {
		return Layout{}, fmt.Errorf("%w: no TN ceiling; a zero ceiling would refuse every tone index the chart prints", kw.ErrLayoutInvalid)
	}
	if cfg.MaxCTCSSIndex == 0 {
		return Layout{}, fmt.Errorf("%w: no CN ceiling; a zero ceiling would refuse every CTCSS index the chart prints", kw.ErrLayoutInvalid)
	}

	// AN EMPTY EX INVENTORY IS ACCEPTED, DELIBERATELY. mustNewLayout runs at
	// initialisation, so refusing one would stop this package compiling until
	// its transcription lands — and the closed direction is already the
	// empty set's: an empty membership set refuses every EX address. Each
	// row's staleness test and the cross-check are what stop a bootstrap
	// inventory shipping.
	l := Layout{
		book:          cfg.Book,
		model:         cfg.Model,
		catID:         cfg.CATID,
		slots:         append([]kw.SlotRange(nil), cfg.Slots...),
		modeNames:     make(map[byte]string, len(cfg.ModeNames)),
		maxToneIndex:  cfg.MaxToneIndex,
		maxCTCSSIndex: cfg.MaxCTCSSIndex,
		mainSub:       cfg.MainSub,
		exItems:       kw.CopyEXItems(cfg.EXItems),
	}
	for k, v := range cfg.ModeNames {
		l.modeNames[k] = v
	}
	return l, nil
}

// mustNewLayout is newLayout for a value minted at initialisation.
//
// A PANIC AT START-UP IS THE RIGHT FAILURE HERE, and it is core/kw's ruling
// restated: a zero Layout failing closed at every later site would read as
// "this radio refuses everything", which is a much harder fault to diagnose
// than a config the constructor named at the moment it was built.
func mustNewLayout(cfg layoutConfig) Layout {
	l, err := newLayout(cfg)
	if err != nil {
		panic(err)
	}
	return l
}

// validateCATID requires exactly kw.IDDigits decimal digits.
//
// THE WIDTH IS core/kw'S CONSTANT AND NOT A LITERAL: an ID answer is six
// bytes with three digits on all four Kenwood books (890:2738, 990:2617), and
// a row that transcribed the number here would hold a second copy of a datum
// core/kw owns.
func validateCATID(id string) error {
	if len(id) != kw.IDDigits {
		return fmt.Errorf("%w: the CAT ID is %q, and every Kenwood book prints P1 as exactly %d digits (890:2738, 990:2617)", kw.ErrLayoutInvalid, id, kw.IDDigits)
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return fmt.Errorf("%w: the CAT ID is %q, and byte %d is not a decimal digit", kw.ErrLayoutInvalid, id, i+1)
		}
	}
	return nil
}

// validateSlots requires at least one range, each with a real class, each
// running forwards, all ascending and non-overlapping.
//
// THE ORDERING AND OVERLAP RULES ARE WHAT MAKE THE DOMAIN A PARTITION. A slot
// number that fell in two ranges would have two classes, and which one a
// caller got would depend on the order this slice happened to be written in.
func validateSlots(slots []kw.SlotRange) error {
	if len(slots) == 0 {
		return fmt.Errorf("%w: no slot classes; a layout with no slot domain admits no channel at all", kw.ErrLayoutInvalid)
	}
	prevHi := -1
	for i, r := range slots {
		if r.Class == kw.SlotClassInvalid {
			return fmt.Errorf("%w: slot range %d (%d-%d) has no class", kw.ErrLayoutInvalid, i+1, r.Lo, r.Hi)
		}
		if r.Lo < 0 || r.Hi < r.Lo {
			return fmt.Errorf("%w: slot range %d is %d-%d, which is not an ascending range of non-negative channel numbers", kw.ErrLayoutInvalid, i+1, r.Lo, r.Hi)
		}
		if r.Lo <= prevHi {
			return fmt.Errorf("%w: slot range %d starts at %d, which is not past the previous range's %d — the ranges must ascend and must not overlap, or one channel number would have two classes", kw.ErrLayoutInvalid, i+1, r.Lo, prevHi)
		}
		prevHi = r.Hi
	}
	return nil
}

// validateModeNames requires a non-empty legend, every value named, every key
// inside the printed alphabet, and NEITHER of the two values both books print
// "Unused".
//
// THE ALPHABET IS '0'-'9' THEN 'A'-'N', which is the widest of the two
// charts (890:3977-3992 stops at 'F'; 990:3707-3730 runs to 'N'). It is a
// capacity check and not a per-row domain: what each row publishes is its own
// map, and layout_test.go pins the two legends' contents and their difference.
func validateModeNames(names map[byte]string) error {
	if len(names) == 0 {
		return fmt.Errorf("%w: no mode legend; MA0 carries none of its own and reads OM P2's, so a layout without one can name no channel's mode", kw.ErrLayoutInvalid)
	}
	for b, name := range names {
		if name == "" {
			return fmt.Errorf("%w: the mode legend names value %q with an empty string", kw.ErrLayoutInvalid, b)
		}
		if !(b >= '0' && b <= '9') && !(b >= 'A' && b <= 'N') {
			return fmt.Errorf("%w: the mode legend carries value %q, which is outside OM P2's printed alphabet '0'-'9' then 'A'-'N' (890:3976-3992, 990:3706-3730)", kw.ErrLayoutInvalid, b)
		}
		if b == '0' || b == '8' {
			return fmt.Errorf("%w: the mode legend names value %q, which BOTH books print \"Unused\" (890:3977, 890:3985; 990:3707, 990:3715) — it names no mode a channel can be in", kw.ErrLayoutInvalid, b)
		}
	}
	return nil
}

// Configured reports whether l was built by this package's constructor and
// describes one of the two MA radios. The zero Layout does not.
func (l Layout) Configured() bool {
	return (l.book == kw.Book890 || l.book == kw.Book990) && l.modeNames != nil
}

// Book is the PC-command document this row speaks — what an "E;" or an "O;"
// quotes its cause sentence from.
func (l Layout) Book() kw.Book { return l.book }

// Model is this row's registry model name, as every refusal message spells
// it.
func (l Layout) Model() string { return l.model }

// CATID is the three digits this row's ID answer carries: "024" on the
// TS-890S (890:2733), "022" on the TS-990S (990:2612).
func (l Layout) CATID() string { return l.catID }

// Slots is this row's slot domain, as a copy.
func (l Layout) Slots() []kw.SlotRange {
	return append([]kw.SlotRange(nil), l.slots...)
}

// ModeName returns the name this row publishes for OM P2 value b, and
// whether it publishes one at all. A zero Layout publishes none.
func (l Layout) ModeName(b byte) (string, bool) {
	name, ok := l.modeNames[b]
	return name, ok
}

// ModeNames is this row's whole legend, as a copy.
func (l Layout) ModeNames() map[byte]string {
	out := make(map[byte]string, len(l.modeNames))
	for k, v := range l.modeNames {
		out[k] = v
	}
	return out
}

// MaxToneIndex is the highest TN index this row's chart prints, 50 on both
// (890:5162, 990:4971). Index 99 is a setting command only and is not a tone.
func (l Layout) MaxToneIndex() uint8 { return l.maxToneIndex }

// MaxCTCSSIndex is the highest CN index this row's chart prints, 49 on both
// (890:1367, 990:1262). There is no CN index 50 on either radio, which is
// why 1750 Hz has no receive encoding at all.
func (l Layout) MaxCTCSSIndex() uint8 { return l.maxCTCSSIndex }

// HasMainSub reports whether this row's BOOK gives its commands a Main/Sub
// band pointer. See layout990Config for what the axis is and is not.
func (l Layout) HasMainSub() bool { return l.mainSub }

// EXItems is this row's whole menu inventory, as a copy.
func (l Layout) EXItems() []kw.EXItem { return kw.CopyEXItems(l.exItems) }

// EXItem returns the inventory row at addr, and whether this radio has one.
//
// MEMBERSHIP, NOT A SCALAR BOUND, AND IT IS ASKED FROM THE SAME PLACE AS ITS
// DATUM. Both charts are SPARSE, with gaps inside every category, so a
// ceiling would admit an address no chart prints — which breaches the
// standing rule that every frame this programme sends is one the book
// describes. The builder, the parser and the outbound gate all ask this one
// method, so no two of them can disagree about what this radio has.
//
// A LINEAR SCAN, AND THAT IS THE WHOLE IMPLEMENTATION. The inventories are a
// few hundred rows and the callers are a settings sweep and a gate: an index
// built at construction would be a second copy of a generated artefact to
// keep in step, for a lookup nothing measures.
func (l Layout) EXItem(addr kw.EXAddress) (kw.EXItem, bool) {
	for _, it := range l.exItems {
		if it.Addr == addr {
			return it, true
		}
	}
	return kw.EXItem{}, false
}

// NewFramingFor returns the transport.Framing for a CONFIGURED LAYOUT: the
// adapter core/kw builds for that layout's book, with THIS package's outbound
// gate in front of the envelope.
//
// IT IS THE CONSTRUCTOR EVERY MA DRIVER USES, and internal/guards'
// TestKenwoodDriversUseNewFramingFor is what says so mechanically: no
// non-test file under core/driver may call kw.NewFraming or
// kw.NewFramingWithGate, because both leave the gate weaker than the row's
// own grammars — the first at the envelope, the second at whatever predicate
// the caller passes.
//
// IT KEEPS ITS OWN Configured() REFUSAL rather than leaning on the
// constructor it delegates to, for kw.NewFramingFor's reason exactly: a zero
// Layout's AllowedCommand is a perfectly non-nil method value, and
// kw.NewFramingWithGate — which refuses a NIL predicate — cannot tell it from
// a real gate.
func NewFramingFor(l Layout) (transport.Framing, error) {
	if !l.Configured() {
		return nil, fmt.Errorf("%w: the layout is unconfigured and describes no radio, so its outbound gate would speak for none", kw.ErrLayoutInvalid)
	}
	return kw.NewFramingWithGate(l.Book(), l.AllowedCommand)
}
