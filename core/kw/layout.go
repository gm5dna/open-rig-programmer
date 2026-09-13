// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"fmt"
	"sort"
)

// ErrLayoutInvalid is the sentinel every NewLayout refusal wraps.
var ErrLayoutInvalid = errors.New("kw: invalid record layout")

// enumName renders one of this package's small named-constant enums: the
// case name if v is a key of names, or unset (the type's own zero-value
// name) for anything else — including a value NewLayout has already
// refused, so a config that reaches String() before validation still
// gets a real word rather than a number. Shared by every enum String()
// below, so a constant added to one of them is one map entry away from a
// self-naming diagnostic rather than a rewritten switch.
func enumName[T ~int](v T, unset string, names map[T]string) string {
	if n, ok := names[v]; ok {
		return n
	}
	return unset
}

// P2Policy says what byte 4 of a memory record means on one radio.
//
// The zero value is P2Unset and NewLayout refuses it, which is this type's
// whole point: byte 4 is the channel's hundreds digit on the 590 pair
// (590:1539-1540, referred to MC, whose own chart prints the space
// convention at 590:1332-1337) and "Always 0 for the TS-480" (480:953).
// Those are not two readings of one rule; they are two rules, and a site
// that defaulted would put a TS-480 record's bank digit where the 480 has
// no bank field at all.
type P2Policy int

// The two byte-4 policies, plus the refusing default.
const (
	P2Unset P2Policy = iota
	// P2HundredsDigit is the 590 pair's: the channel number's hundreds
	// digit, '0' or a space below 100 on a Set and always a space on an
	// Answer below 100 (590:1332-1337).
	P2HundredsDigit
	// P2FixedZero is the TS-480's: "Always 0 for the TS-480." (480:953).
	P2FixedZero
	// P2Unused is the TS-570's: byte 4 exists at the family's offset but
	// the Parameter Table prints a dash, not a digit count — a materially
	// different "carries nothing at all" convention from P2FixedZero's
	// printed "0" (ts570-capability-matrix.md §1.4, cited by the Phase 2
	// brief). It is its own member rather than an alias for P2FixedZero
	// because the two rows disagree on what byte 4 CONTAINS, not merely on
	// whether it varies.
	P2Unused
)

// String renders p for refusals and logs.
func (p P2Policy) String() string {
	return enumName(p, "P2Unset", map[P2Policy]string{
		P2HundredsDigit: "P2HundredsDigit",
		P2FixedZero:     "P2FixedZero",
		P2Unused:        "P2Unused",
	})
}

// Byte19Meaning says what byte 19 (P6) of a memory record means.
//
// The two books spend this byte on entirely different things — the data
// mode on the 590 pair (590:1546-1548, referred to DA) and the channel
// lockout on the 480 (480:962, "Lockout status. 0: Lockout OFF, 1: Lockout
// ON.") — and the 590 pair carry the lockout at byte 41 instead. A layout
// that defaulted here would read one radio's lockout as another's data
// mode, which is why the zero value refuses.
type Byte19Meaning int

// The two byte-19 meanings, plus the refusing default.
const (
	Byte19Unset Byte19Meaning = iota
	Byte19DataMode
	Byte19Lockout
)

// String renders b for refusals and logs.
func (b Byte19Meaning) String() string {
	return enumName(b, "Byte19Unset", map[Byte19Meaning]string{
		Byte19DataMode: "Byte19DataMode",
		Byte19Lockout:  "Byte19Lockout",
	})
}

// P10Policy says what bytes 25-27 (P10) of a memory record carry on one
// row: a printed constant on both registered books ("000: Always 000",
// 590:1558-1559, 480:971) and a live 3-digit DCS code on the TS-2000
// (ts2000-capability-matrix.md §2, P10). Same shape as P2Policy: the zero
// value refuses, because a layout that defaulted would read a live DCS
// code as the registered rows' "always 000", or vice versa.
type P10Policy int

// The two P10 policies, plus the refusing default.
const (
	P10Unset P10Policy = iota
	// P10FixedZero is the 590 pair's and the TS-480's: "Always 000"
	// (590:1558-1559, 480:971).
	P10FixedZero
	// P10DCSCode is the TS-2000's: a live 3-digit DCS code.
	P10DCSCode
)

// String renders p for refusals and logs.
func (p P10Policy) String() string {
	return enumName(p, "P10Unset", map[P10Policy]string{
		P10FixedZero: "P10FixedZero",
		P10DCSCode:   "P10DCSCode",
	})
}

// P12Policy says what byte 29 (P12) of a memory record carries on one row:
// a printed constant on both registered books ("0: Always 0",
// 590:1565-1566, 480:975) and a live shift-status enum on the TS-2000 — '0'
// Simplex, '1' +, '2' -, '3' "= All E-types" (ts2000-capability-matrix.md
// §2, P12).
type P12Policy int

// The two P12 policies, plus the refusing default.
const (
	P12Unset P12Policy = iota
	// P12FixedZero is the 590 pair's and the TS-480's: "Always 0"
	// (590:1565-1566, 480:975).
	P12FixedZero
	// P12ShiftLive is the TS-2000's: a live shift-status enum.
	P12ShiftLive
)

// String renders p for refusals and logs.
func (p P12Policy) String() string {
	return enumName(p, "P12Unset", map[P12Policy]string{
		P12FixedZero: "P12FixedZero",
		P12ShiftLive: "P12ShiftLive",
	})
}

// P13Policy says what bytes 30-38 (P13) of a memory record carry on one
// row: a printed constant on both registered books ("000000000: Always
// 000000000", 590:1567-1568, 480:977) and a live 9-digit offset frequency
// on the TS-2000 (ts2000-capability-matrix.md §2, P13).
type P13Policy int

// The two P13 policies, plus the refusing default.
const (
	P13Unset P13Policy = iota
	// P13FixedZero is the 590 pair's and the TS-480's: "Always 000000000"
	// (590:1567-1568, 480:977).
	P13FixedZero
	// P13OffsetLive is the TS-2000's: a live 9-digit offset frequency.
	P13OffsetLive
)

// String renders p for refusals and logs.
func (p P13Policy) String() string {
	return enumName(p, "P13Unset", map[P13Policy]string{
		P13FixedZero:  "P13FixedZero",
		P13OffsetLive: "P13OffsetLive",
	})
}

// Byte28Policy says what byte 28 (P11) of a memory record is on one row.
//
// THIS IS THE ONE AXIS ON WHICH THE TWO 590 ROWS DIFFER, which is why it is
// a policy rather than a meaning. The book prints one P11 legend for both —
// "0: FILTER A / 1: FILTER B" (590:1560-1563) — and then a sentence that
// applies to one of them: "* In firmware version 1.xx of TS-590S, always
// \"0\"." (590:1478). The 480 has no filter field here at all: "Always 0 for
// the TS-480." (480:973).
//
// THE QUOTED WORDING IS MR'S, AT 590:1478. MW prints the same condition in
// different words and with a mismatched quotation mark — "This is always set
// to “0" in the firmware version 1.xx of TS-590S." (590:1564) — which is
// erratum E7 in doc.go. The two are one datum and either may be cited; what
// must not happen is one line's text under the other line's number, because
// this axis rests on that sentence and E7 is the record that they differ.
type Byte28Policy int

// The three byte-28 policies, plus the refusing default.
const (
	Byte28Unset Byte28Policy = iota
	// Byte28FilterLive is the TS-590SG's: the byte selects FILTER A or B
	// and both values are meaningful.
	Byte28FilterLive
	// Byte28FilterEither is the TS-590S's: the field exists in the grid and
	// the parser must accept either printed value, because the "always 0"
	// sentence is scoped to firmware 1.xx and a later firmware answers '1'.
	// What a WRITE may carry is the driver's question, not the codec's.
	Byte28FilterEither
	// Byte28FixedZero is the TS-480's: a printed constant, and so a member
	// of that layout's printed-fixed set as well.
	Byte28FixedZero
	// Byte28Reverse is the TS-2000's: P11 is a live REVERSE status ('0'/'1'),
	// the same two-value domain as Byte28FilterLive but a different meaning
	// (ts2000-capability-matrix.md §2, P11) — a fourth member rather than a
	// fourth type, since the wire domain the parser and builder admit is
	// identical to the filter axis's live case.
	Byte28Reverse
)

// String renders b for refusals and logs.
func (b Byte28Policy) String() string {
	return enumName(b, "Byte28Unset", map[Byte28Policy]string{
		Byte28FilterLive:   "Byte28FilterLive",
		Byte28FilterEither: "Byte28FilterEither",
		Byte28FixedZero:    "Byte28FixedZero",
		Byte28Reverse:      "Byte28Reverse",
	})
}

// Byte3940Meaning says what bytes 39-40 (P14) of a memory record mean.
//
// An FM bandwidth flag on the 590 pair — "00: FM Normal / 01: FM Narrow"
// (590:1569-1571) — and the tuning step index on the 480, "Step size. Refer
// to the ST command." (480:979). ST's own legend is mode-conditional over
// two different ranges (480:1494-1500), which is why A22 refuses every
// TS-480 channel write; that refusal is the driver's, and this axis is what
// tells it which byte it is looking at.
type Byte3940Meaning int

// The two bytes-39-40 meanings, plus the refusing default.
const (
	Byte3940Unset Byte3940Meaning = iota
	Byte3940FMNarrowFlag
	Byte3940StepIndex
)

// String renders b for refusals and logs.
func (b Byte3940Meaning) String() string {
	return enumName(b, "Byte3940Unset", map[Byte3940Meaning]string{
		Byte3940FMNarrowFlag: "Byte3940FMNarrowFlag",
		Byte3940StepIndex:    "Byte3940StepIndex",
	})
}

// Byte41Meaning says what byte 41 (P15) of a memory record means: the
// channel lockout on the 590 pair (590:1572-1574) and a printed constant on
// the 480 (480:982, "Always 0 for the TS-480."), which spends byte 19 on its
// lockout instead.
type Byte41Meaning int

// The two byte-41 meanings, plus the refusing default.
const (
	Byte41Unset Byte41Meaning = iota
	Byte41Lockout
	Byte41FixedZero
	// Byte41MemoryGroup is the TS-2000's: P15 is a live Memory Group
	// (0-9), one printed digit, where the 590 pair carry the channel
	// lockout and the TS-480 prints a constant (ts2000-capability-matrix.md
	// §2, P15).
	Byte41MemoryGroup
)

// String renders b for refusals and logs.
func (b Byte41Meaning) String() string {
	return enumName(b, "Byte41Unset", map[Byte41Meaning]string{
		Byte41Lockout:     "Byte41Lockout",
		Byte41FixedZero:   "Byte41FixedZero",
		Byte41MemoryGroup: "Byte41MemoryGroup",
	})
}

// ToneModeSet says how many values byte 20 (P7) admits on one radio: four
// on the 590 pair, whose fourth is "3: Cross Tone ON" (590:1549-1553), and
// three on the 480, which prints "0: OFF, 1: TONE, 2: CTCSS" and no cross
// tone at all (480:964).
type ToneModeSet int

// The two tone-mode value sets, plus the refusing default.
const (
	ToneModesUnset ToneModeSet = iota
	ToneModesFour
	ToneModesThree
	// ToneModesTwo is the TS-570's: byte 20 (P7) is "0: OFF / 1: ON" only,
	// narrower than both the 590 pair's four values and the TS-480's three
	// (ts570-capability-matrix.md §1.4, cited by the Phase 2 brief).
	ToneModesTwo
)

// String renders s for refusals and logs.
func (s ToneModeSet) String() string {
	return enumName(s, "ToneModesUnset", map[ToneModeSet]string{
		ToneModesFour:  "ToneModesFour",
		ToneModesThree: "ToneModesThree",
		ToneModesTwo:   "ToneModesTwo",
	})
}

// SlotClass is what one memory slot NUMBER means in a layout's slot space.
//
// It is the datum M9 turns on: a record's P1 byte is derived from the class
// of the slot being written, never from the channel's split state. See
// Slot.P1 (record.go).
type SlotClass int

// The three slot classes, plus the refusing default.
const (
	SlotClassInvalid SlotClass = iota
	// SlotMemory is an ordinary memory channel: 000-099 on the 590 pair,
	// 00-99 on the 480 (480:955).
	SlotMemory
	// SlotScan is a section-defined channel, which holds TWO frequencies
	// reached by the same slot number with different P1 bytes: "When
	// registering a section defined channel, set parameter P1 to 0 to enter
	// the Start frequency, then set P1 to 1 to set the End frequency."
	// (590:1529-1531). On the 590 pair these are 100-109, printed as P00 ~
	// P09 (590:1345).
	SlotScan
	// SlotExtension is the TS-590SG's 110-119, printed as E00 ~ E09 with a
	// typo in the second name (590:1346-1347, erratum E2). What an extension
	// channel IS is never explained anywhere in the book (A11), so no
	// behaviour is attached to this class beyond membership; Stuart ruled on
	// 05/09/2026 that the ten slots are OMITTED from the driver's published
	// banks until A11 lifts, and this codec's slot domain is still what the
	// book prints (A12).
	SlotExtension
)

// String renders c for refusals and logs.
func (c SlotClass) String() string {
	return enumName(c, "SlotClassInvalid", map[SlotClass]string{
		SlotMemory:    "SlotMemory",
		SlotScan:      "SlotScan",
		SlotExtension: "SlotExtension",
	})
}

// SlotRange is one inclusive band of slot numbers of a single class.
type SlotRange struct {
	Class  SlotClass
	Lo, Hi int
}

// FixedField is one run of bytes a book prints as a constant.
//
// Pos is the book's OWN 1-indexed position of the run's first byte, so a
// reader can put this literal beside the chart it came from without
// arithmetic; the width is len(Printed). The 480's P13 is therefore
// {Pos: 30, Printed: "000000000"}, exactly as 480:977 prints it.
type FixedField struct {
	Pos     int
	Printed string
}

// end returns the 1-indexed position of f's last byte.
func (f FixedField) end() int { return f.Pos + len(f.Printed) - 1 }

// LayoutConfig is the per-radio description a model package hands NewLayout.
//
// EVERY FIELD IS REQUIRED. There is no partial layout: see Layout.
type LayoutConfig struct {
	// Book is the PC-command document this row speaks, which is what an
	// error quoting a cause sentence needs (errata E13; errors.go).
	Book Book
	// Model is the row's own name, used only in refusals.
	Model string

	// RecordLen is the MR answer's and the MW Set frame's width in bytes:
	// 50 on the 590 pair and the TS-480 (unchanged), 28 on the TS-570
	// (positions 1-22 a true prefix of the same grid, ts570-capability-
	// matrix.md §1.4) and, in principle, any other true prefix of it. ZERO
	// IS REFUSED like every other axis. It is a byte count and not a
	// design choice a row can leave to a package default: RecordLen was a
	// package constant before this lift, and the TWO REGISTERED ROWS PIN
	// IT EXPLICITLY (ts590/layout.go, ts480/layout.go) so their frames stay
	// byte-identical by construction rather than by an unstated default
	// surviving the lift.
	RecordLen uint8

	P2        P2Policy
	Byte19    Byte19Meaning
	Byte28    Byte28Policy
	Byte3940  Byte3940Meaning
	Byte41    Byte41Meaning
	ToneModes ToneModeSet
	// P10, P12 and P13 are the TS-2000 lift's three axes: a printed
	// constant on both registered books and a live field on the TS-2000
	// (ts2000-capability-matrix.md §2). Required like every other axis.
	P10 P10Policy
	P12 P12Policy
	P13 P13Policy

	// MaxEXAddress is the highest menu number THIS ROW'S BOOK PRINTS for
	// the EX command, inclusive: 87 on the TS-590S (590:543), 99 on the
	// TS-590SG (590:544) and 60 on the TS-480 (480:401).
	//
	// IT IS THE PRINTED DOMAIN AND NOT AN INVENTORY. Which addresses a row
	// HAS, what each is called and how wide its answer is are the generated
	// inventory's facts, and those live in core/kw/ts590 and core/kw/ts480,
	// which import this package (BuildEXRead's own doc comment states the
	// division). What this axis carries is one number a reader can put
	// beside the chart it came from, which is the same shape as SlotRange.
	//
	// ZERO IS REFUSED, like every other axis. Address 000 is a real menu on
	// all three rows, so a zero here cannot be read as "no EX surface"; it
	// is indistinguishable from an unset field, and defaulting it would put
	// the widest domain of the three on a row whose book prints the
	// narrowest.
	MaxEXAddress uint8

	// ModeNames is this row's MD legend in the programme's own spellings.
	// It may not name ModeNone or ModeTune, neither of which names a mode a
	// channel can be in.
	ModeNames map[Mode]string

	// Slots is this row's slot space: non-overlapping ranges, each of one
	// class.
	Slots []SlotRange

	// PrintedFixed is this row's complete hard-wired byte set. It must
	// contain the three runs BOTH books print — P10, P12 and P13 — and it
	// must agree with the P2, Byte28 and Byte41 axes, which are the three
	// positions that carry a meaning on one radio and a constant on
	// another.
	PrintedFixed []FixedField
}

// Layout is one radio row's reading of the shared 50-byte memory grid: the
// nine axes on which the two books differ, and nothing else.
//
// ONE GRID, TWO LAYOUTS. Both radios use the same 50 bytes in the same
// order (590:1440-1461 and 590:1518-1536; 480:923-943 and 480:955-976), so
// the offsets are the family's and are package constants (record.go). What
// differs is what several of those bytes MEAN, and every one of those
// differences is an axis here rather than a branch in a parser.
//
// A ZERO LAYOUT FAILS CLOSED. It describes no radio: it is not Configured,
// every axis reads its unset value, its legend and slot space are empty, and
// every builder and parser method on it refuses. That is the FT-891 Stage 0
// lesson — a policy-reading site must not default on a zero axis — applied
// from birth rather than retrofitted. The pin is
// TestZeroLayout_FailsClosedOnEveryAxis, named on one line so a reader can
// grep for it.
//
// The fields are unexported and the accessors copy, so a layout a model
// package minted at initialisation cannot be edited by a caller holding it.
type Layout struct {
	book  Book
	model string

	recordLen uint8

	p2           P2Policy
	byte19       Byte19Meaning
	byte28       Byte28Policy
	byte3940     Byte3940Meaning
	byte41       Byte41Meaning
	toneModes    ToneModeSet
	p10          P10Policy
	p12          P12Policy
	p13          P13Policy
	maxEXAddress uint8

	modeNames    map[Mode]string
	slots        []SlotRange
	printedFixed []FixedField
}

// hardWiredPositions is the set of 1-indexed positions a Kenwood memory
// record may declare hard-wired, derived from the two charts.
//
// It is a WHITELIST rather than a blacklist because the failure it guards
// against is a layout claiming a constant where a book prints a field: this
// codec would then refuse a legitimate answer, and no test outside the
// layout could tell that from a genuinely malformed frame. The permitted
// positions are exactly byte 4 (P2, fixed on the 480 only), bytes 25-27
// (P10, both), byte 28 (P11, fixed on the 480 only), byte 29 (P12, both),
// bytes 30-38 (P13, both) and byte 41 (P15, fixed on the 480 only).
var hardWiredPositions = map[int]bool{
	4:  true,
	25: true, 26: true, 27: true,
	28: true,
	29: true,
	30: true, 31: true, 32: true, 33: true, 34: true,
	35: true, 36: true, 37: true, 38: true,
	41: true,
}

// commonHardWiring is the run both registered books print as constants:
// P10 "000: Always 000" (590:1558-1559, 480:971), P12 "0: Always 0"
// (590:1565-1566, 480:975) and P13 "000000000: Always 000000000"
// (590:1567-1568, 480:977).
//
// IT IS NO LONGER A UNIVERSAL REQUIREMENT, and that is the TS-2000 lift's
// own change to this file. Before it, every Kenwood row hard-wired these
// three positions, so validatePrintedFixed demanded them unconditionally;
// the TS-2000 carries all three LIVE (P10Policy/P12Policy/P13Policy), so
// whether they are fixed is now axis-driven, the same crossChecks shape
// byte 4, byte 28 and byte 41 already had. This slice remains for the two
// registered rows' own construction (ts590/layout.go, ts480/layout.go,
// testlayouts_test.go) to build their PrintedFixed set from, unchanged.
var commonHardWiring = []FixedField{
	{Pos: 25, Printed: "000"},
	{Pos: 29, Printed: "0"},
	{Pos: 30, Printed: "000000000"},
}

// NewLayout validates cfg and returns the layout it describes, or the zero
// Layout and an error wrapping ErrLayoutInvalid.
//
// It refuses an unset axis rather than choosing for the caller, and it
// cross-checks the three positions that appear BOTH as an axis and as a
// possible member of the printed-fixed set. A bound consulted from
// somewhere other than its own datum is this repository's standing hazard;
// here the two would be one edit apart and, without this check, nothing
// would fail.
//
// THE BOOK IS TESTED TWICE, AND THE SECOND TEST IS NOT REDUNDANT.
// Book.valid() asks "has this package read that document" — four books —
// and the second asks "does that document print the record this type IS",
// which is two. The refusal is BY NAME rather than by a widened valid()
// because valid() is also what framing, IsFatal and the typed errors
// consult, and they legitimately speak for all four.
//
// IT IS ALSO WHAT KEEPS THE TREE'S OTHER Book SWITCHES CORRECT WITHOUT ARMS
// OF THEIR OWN: kwtest's layout self-consistency switch and
// golden_test.go's replayLayout both list Book590 and Book480 and treat
// anything else as a failure. Both stay right precisely because Book890 and
// Book990 are unreachable from any kw.Layout. Without this refusal a later
// reader "finishes the job" by giving them arms — which would need fake
// 50-byte layouts for two radios whose books print no such record.
// TestNewLayout_RefusesTheTwoBooksThisRecordDoesNotDescribe is the pin.
func NewLayout(cfg LayoutConfig) (Layout, error) {
	if !cfg.Book.valid() {
		return Layout{}, fmt.Errorf("%w: Book is unset — a layout that names no document cannot quote a cause sentence (got %v)", ErrLayoutInvalid, cfg.Book)
	}
	if cfg.Book != Book590 && cfg.Book != Book480 && cfg.Book != Book570 {
		return Layout{}, fmt.Errorf("%w: %v is a document this package READS but whose memory channel it does not DESCRIBE — this Layout is the shared MR/MW record family (the 590 pair's and the TS-480's 50-byte grid, and the TS-570's 28-byte PREFIX of the same grid, RecordLen), and the TS-890S/TS-990S channel is core/kw/ma's own layout type while the TS-870S's is core/kw's own Layout870 — a second grid, not a prefix, because its offsets shift rather than merely stopping early", ErrLayoutInvalid, cfg.Book)
	}
	if cfg.Model == "" {
		return Layout{}, fmt.Errorf("%w: Model is empty — every refusal this layout produces names the row it speaks for", ErrLayoutInvalid)
	}
	// RecordLen MUST BE ONE OF THE FAMILY'S TWO KNOWN WIDTHS, not merely
	// nonzero. Both parseRecordFrame and BuildMWSet index this frame at
	// FIXED offsets up to and including the terminator at recordLen-1
	// (the prefix, P1, P2/P3, P4-P8), so any RecordLen shorter than that
	// fixed-field span — RecordLenPrefix's own 28, the narrowest shape
	// this codec knows — indexes past a frame this short and PANICS
	// rather than refusing (Codex close review, P2): RecordLen: 1 would
	// reach recFreqOff+recFreqDigits (byte 17) on a one-byte slice before
	// any length check could catch it. A zero-refusal alone caught the
	// FT-891 Stage 0 shape (an omitted axis silently defaulting); it does
	// not catch a PRESENT but too-small one, which is the same failure
	// mode one step removed. Restricting to the two discrete widths
	// hasTail's own reasoning already treats as the only shapes (parse.go)
	// is the smaller fix over computing a minimum from the fixed-field
	// offsets, and it is what "the known widths 28/50" names.
	if cfg.RecordLen != RecordLen && cfg.RecordLen != RecordLenPrefix {
		return Layout{}, fmt.Errorf("%w (%s): RecordLen is %d — this codec knows two widths, %d (the 590 pair and the TS-480) and %d (the TS-570's prefix of the same grid), and any other value either cannot hold this family's fixed fields at all or is a shape no row has needed yet", ErrLayoutInvalid, cfg.Model, cfg.RecordLen, RecordLen, RecordLenPrefix)
	}
	if cfg.P2 == P2Unset {
		return Layout{}, fmt.Errorf("%w (%s): P2 policy is unset — byte 4 is the channel's hundreds digit on the 590 pair and a printed constant on the 480, and this codec will not guess which", ErrLayoutInvalid, cfg.Model)
	}
	if cfg.Byte19 == Byte19Unset {
		return Layout{}, fmt.Errorf("%w (%s): byte 19's meaning is unset — the data mode on the 590 pair, the channel lockout on the 480", ErrLayoutInvalid, cfg.Model)
	}
	// hasTail is whether this row's record reaches past byte 22 (P8) at
	// all. RecordLen == RecordLen (the package constant, the family's own
	// full 50-byte grid) is the only width this lift gives a tail to: the
	// TS-570's 28 stops at the terminator with nothing between P8 and it
	// (evidence/ts570d-transcription.csv: "23-27 (P9)... NOT USED", "28
	// terminator... No filter/reverse, shift, offset-frequency, step-size
	// or name field exists past this point"). So byte 28 (P11), bytes
	// 39-40 (P14), byte 41 (P15), P10, P12 and P13 have NO BYTE TO
	// DESCRIBE on a row whose RecordLen is anything else, and CTCSS (P9)
	// — which has never had a policy axis of its own, because both
	// registered books always carry it live — is gated the same way in
	// parse.go/builders.go rather than growing one now.
	hasTail := cfg.RecordLen == RecordLen
	if hasTail {
		if cfg.Byte28 == Byte28Unset {
			return Layout{}, fmt.Errorf("%w (%s): byte 28's policy is unset — a live FILTER A/B selection, a field whose printed value is accepted either way, a printed constant, or a live REVERSE status", ErrLayoutInvalid, cfg.Model)
		}
		if cfg.Byte3940 == Byte3940Unset {
			return Layout{}, fmt.Errorf("%w (%s): bytes 39-40's meaning is unset — the FM Normal/Narrow flag on the 590 pair, the tuning step index on the 480", ErrLayoutInvalid, cfg.Model)
		}
		if cfg.Byte41 == Byte41Unset {
			return Layout{}, fmt.Errorf("%w (%s): byte 41's meaning is unset — the channel lockout on the 590 pair, a printed constant on the 480, or a live Memory Group", ErrLayoutInvalid, cfg.Model)
		}
		if cfg.P10 == P10Unset {
			return Layout{}, fmt.Errorf("%w (%s): P10's policy is unset — a printed constant on the 590 pair and the TS-480, a live DCS code on the TS-2000", ErrLayoutInvalid, cfg.Model)
		}
		if cfg.P12 == P12Unset {
			return Layout{}, fmt.Errorf("%w (%s): P12's policy is unset — a printed constant on the 590 pair and the TS-480, a live shift-status enum on the TS-2000", ErrLayoutInvalid, cfg.Model)
		}
		if cfg.P13 == P13Unset {
			return Layout{}, fmt.Errorf("%w (%s): P13's policy is unset — a printed constant on the 590 pair and the TS-480, a live offset frequency on the TS-2000", ErrLayoutInvalid, cfg.Model)
		}
	} else {
		if cfg.Byte28 != Byte28Unset {
			return Layout{}, fmt.Errorf("%w (%s): byte 28's policy is %s, but this row's %d-byte record has no byte 28 at all", ErrLayoutInvalid, cfg.Model, cfg.Byte28, cfg.RecordLen)
		}
		if cfg.Byte3940 != Byte3940Unset {
			return Layout{}, fmt.Errorf("%w (%s): bytes 39-40's meaning is %s, but this row's %d-byte record has no bytes 39-40 at all", ErrLayoutInvalid, cfg.Model, cfg.Byte3940, cfg.RecordLen)
		}
		if cfg.Byte41 != Byte41Unset {
			return Layout{}, fmt.Errorf("%w (%s): byte 41's meaning is %s, but this row's %d-byte record has no byte 41 at all", ErrLayoutInvalid, cfg.Model, cfg.Byte41, cfg.RecordLen)
		}
		if cfg.P10 != P10Unset {
			return Layout{}, fmt.Errorf("%w (%s): P10's policy is %s, but this row's %d-byte record has no byte 25-27 run at all", ErrLayoutInvalid, cfg.Model, cfg.P10, cfg.RecordLen)
		}
		if cfg.P12 != P12Unset {
			return Layout{}, fmt.Errorf("%w (%s): P12's policy is %s, but this row's %d-byte record has no byte 29 at all", ErrLayoutInvalid, cfg.Model, cfg.P12, cfg.RecordLen)
		}
		if cfg.P13 != P13Unset {
			return Layout{}, fmt.Errorf("%w (%s): P13's policy is %s, but this row's %d-byte record has no bytes 30-38 run at all", ErrLayoutInvalid, cfg.Model, cfg.P13, cfg.RecordLen)
		}
	}
	if cfg.ToneModes == ToneModesUnset {
		return Layout{}, fmt.Errorf("%w (%s): the tone-mode value set is unset — four values on the 590 pair, three on the 480, two on the TS-570", ErrLayoutInvalid, cfg.Model)
	}
	if cfg.MaxEXAddress == 0 {
		return Layout{}, fmt.Errorf("%w (%s): the highest printed EX menu number is unset — each book prints its own domain per row, \"000 ~ 087\" for the TS-590S (590:543), \"000 ~ 099\" for the TS-590SG (590:544) and \"000 ~ 060\" for the TS-480 (480:401), and a zero here is not \"no menu surface\": address 000 is a real menu on all three", ErrLayoutInvalid, cfg.Model)
	}
	if len(cfg.ModeNames) == 0 {
		return Layout{}, fmt.Errorf("%w (%s): the mode legend is empty — MR/MW P5 carries no legend of its own on either radio, so a layout with no MD legend can name no channel's mode", ErrLayoutInvalid, cfg.Model)
	}
	if err := validateModeNames(cfg); err != nil {
		return Layout{}, err
	}
	if len(cfg.Slots) == 0 {
		return Layout{}, fmt.Errorf("%w (%s): the slot classes are empty — a layout with no slot space admits no channel, and P1 is derived from a slot's class", ErrLayoutInvalid, cfg.Model)
	}
	if err := validateSlots(cfg); err != nil {
		return Layout{}, err
	}
	// AN EMPTY SET IS NO LONGER REFUSED OUTRIGHT, since the TS-2000 lift
	// gives P10, P12 and P13 a live reading too — a row with P2
	// HundredsDigit-like live axes on all six cross-checked positions
	// would have nothing to hard-wire at all. validatePrintedFixed's
	// crossChecks below is what still catches a set that omits a position
	// its OWN axes say is fixed.
	if err := validatePrintedFixed(cfg); err != nil {
		return Layout{}, err
	}

	names := make(map[Mode]string, len(cfg.ModeNames))
	for m, n := range cfg.ModeNames {
		names[m] = n
	}
	slots := make([]SlotRange, len(cfg.Slots))
	copy(slots, cfg.Slots)
	fixed := make([]FixedField, len(cfg.PrintedFixed))
	copy(fixed, cfg.PrintedFixed)
	sort.Slice(fixed, func(i, j int) bool { return fixed[i].Pos < fixed[j].Pos })

	return Layout{
		book:         cfg.Book,
		model:        cfg.Model,
		recordLen:    cfg.RecordLen,
		p2:           cfg.P2,
		byte19:       cfg.Byte19,
		byte28:       cfg.Byte28,
		byte3940:     cfg.Byte3940,
		byte41:       cfg.Byte41,
		toneModes:    cfg.ToneModes,
		p10:          cfg.P10,
		p12:          cfg.P12,
		p13:          cfg.P13,
		maxEXAddress: cfg.MaxEXAddress,
		modeNames:    names,
		slots:        slots,
		printedFixed: fixed,
	}, nil
}

// MustNewLayout is NewLayout for a package-level literal, panicking on a
// config it would refuse.
//
// The model packages mint their layouts at initialisation, where an error
// return has nowhere to go; a layout that cannot be built must stop the
// programme rather than leave a zero Layout to fail closed silently at every
// later site, which would read as "this radio refuses everything".
func MustNewLayout(cfg LayoutConfig) Layout {
	l, err := NewLayout(cfg)
	if err != nil {
		panic(err)
	}
	return l
}

// validateModeNames refuses a legend that names a nibble which is not a
// mode, an undocumented nibble, an empty name, or one name on two nibbles.
func validateModeNames(cfg LayoutConfig) error {
	seen := make(map[string]Mode, len(cfg.ModeNames))
	for m, name := range cfg.ModeNames {
		if !m.documentedNibble() {
			return fmt.Errorf("%w (%s): the mode legend names nibble %q, which neither book prints", ErrLayoutInvalid, cfg.Model, byte(m))
		}
		if !m.namesAMode() {
			return fmt.Errorf("%w (%s): the mode legend names nibble %q, which both books print as a setting failure or as unused rather than as a mode a channel can be in (590:1353, 590:1362; 480:843, 480:853)", ErrLayoutInvalid, cfg.Model, byte(m))
		}
		if name == "" {
			return fmt.Errorf("%w (%s): the mode legend gives nibble %q an empty name", ErrLayoutInvalid, cfg.Model, byte(m))
		}
		if prev, dup := seen[name]; dup {
			return fmt.Errorf("%w (%s): the mode legend gives the name %q to both nibble %q and nibble %q", ErrLayoutInvalid, cfg.Model, name, byte(prev), byte(m))
		}
		seen[name] = m
	}
	return nil
}

// validateSlots refuses a malformed or ambiguous slot space. Overlap is the
// one that matters: a number resolving to two classes would make the
// P1-from-slot-class rule (M9) ambiguous, and the ambiguity would be
// resolved by map iteration order.
//
// IT ALSO CROSS-CHECKS THE SLOT SPACE AGAINST THE P2 AXIS, which is the
// third of the byte-4 cross-checks and the only one the printed-fixed set
// cannot see. A row whose byte 4 is a printed constant has no hundreds digit
// to carry and its channel number is P3's two digits alone; declaring slots
// above 99 on such a row would ask slotWire to put three digits in a
// two-digit field, and the frame that came out would name a DIFFERENT
// channel and be reported as Sent. That is the failure class decision 11
// exists to prevent, and NewLayout is where it is caught rather than at the
// wire-rendering site, which has no error return to a user.
func validateSlots(cfg LayoutConfig) error {
	for _, r := range cfg.Slots {
		switch r.Class {
		case SlotMemory, SlotScan, SlotExtension:
		default:
			return fmt.Errorf("%w (%s): slot range %d-%d has no class", ErrLayoutInvalid, cfg.Model, r.Lo, r.Hi)
		}
		if r.Lo < 0 || r.Hi < r.Lo || r.Hi > maxSlotNumber {
			return fmt.Errorf("%w (%s): slot range %d-%d is not inside 0-%d, the three digits P2 and P3 carry", ErrLayoutInvalid, cfg.Model, r.Lo, r.Hi, maxSlotNumber)
		}
		if cfg.P2 == P2FixedZero && r.Hi > maxFixedZeroSlot {
			return fmt.Errorf("%w (%s): slot range %d-%d reaches %d, but byte 4's policy is %s — a row whose P2 prints \"Always 0\" (480:953) carries its channel number in P3's two digits alone, \"00 ~ 99\" (480:955), so it has no hundreds digit for a slot above %d", ErrLayoutInvalid, cfg.Model, r.Lo, r.Hi, r.Hi, cfg.P2, maxFixedZeroSlot)
		}
		if cfg.P2 == P2Unused && r.Hi > maxFixedZeroSlot {
			return fmt.Errorf("%w (%s): slot range %d-%d reaches %d, but byte 4's policy is %s — the TS-570's Parameter Table prints no hundreds digit at byte 4 at all (ts570-capability-matrix.md §1.4), so this row's channel number is P3's two digits alone, and it has no hundreds digit for a slot above %d", ErrLayoutInvalid, cfg.Model, r.Lo, r.Hi, r.Hi, cfg.P2, maxFixedZeroSlot)
		}
	}
	for i, a := range cfg.Slots {
		for _, b := range cfg.Slots[i+1:] {
			if a.Lo <= b.Hi && b.Lo <= a.Hi {
				return fmt.Errorf("%w (%s): slot ranges %d-%d (%v) and %d-%d (%v) overlap, so a slot number would resolve to two classes and P1 could not be derived from one", ErrLayoutInvalid, cfg.Model, a.Lo, a.Hi, a.Class, b.Lo, b.Hi, b.Class)
			}
		}
	}
	return nil
}

// validatePrintedFixed refuses a hard-wired set that reaches a position no
// book prints as a constant, that overlaps itself, that omits the hard-wiring
// both books print, or that contradicts one of the three axes which cover
// the same positions.
func validatePrintedFixed(cfg LayoutConfig) error {
	covered := make(map[int]bool, len(cfg.PrintedFixed))
	starts := make(map[int]string, len(cfg.PrintedFixed))
	for _, f := range cfg.PrintedFixed {
		if f.Printed == "" {
			return fmt.Errorf("%w (%s): the printed-fixed field at position %d prints nothing", ErrLayoutInvalid, cfg.Model, f.Pos)
		}
		for i := f.Pos; i <= f.end(); i++ {
			if !hardWiredPositions[i] {
				return fmt.Errorf("%w (%s): the printed-fixed field at positions %d-%d reaches position %d, which neither book prints as a constant — a layout claiming one there would make this codec refuse a legitimate answer", ErrLayoutInvalid, cfg.Model, f.Pos, f.end(), i)
			}
			if covered[i] {
				return fmt.Errorf("%w (%s): two printed-fixed fields both cover position %d", ErrLayoutInvalid, cfg.Model, i)
			}
			covered[i] = true
		}
		for _, b := range []byte(f.Printed) {
			if b < 0x20 || b > 0x7e || b == ';' {
				return fmt.Errorf("%w (%s): the printed-fixed field at position %d contains byte %#02x, which no frame this codec builds may carry", ErrLayoutInvalid, cfg.Model, f.Pos, b)
			}
		}
		starts[f.Pos] = f.Printed
	}

	// The six positions that carry a meaning on one radio and a constant on
	// another. Each axis and the set must say the same thing — P2, byte 28
	// and byte 41 by the file's original design; P10, P12 and P13 by the
	// TS-2000 lift's, which is what retired the unconditional "every row
	// hard-wires these three" check this loop used to open with (see
	// commonHardWiring's own comment).
	crossChecks := []struct {
		pos     int
		fixed   bool
		axis    string
		meaning string
		want    string
	}{
		{4, cfg.P2 == P2FixedZero, "P2 policy", cfg.P2.String(), "0"},
		{25, cfg.P10 == P10FixedZero, "P10 policy", cfg.P10.String(), "000"},
		{28, cfg.Byte28 == Byte28FixedZero, "byte 28's policy", cfg.Byte28.String(), "0"},
		{29, cfg.P12 == P12FixedZero, "P12 policy", cfg.P12.String(), "0"},
		{30, cfg.P13 == P13FixedZero, "P13 policy", cfg.P13.String(), "000000000"},
		{41, cfg.Byte41 == Byte41FixedZero, "byte 41's meaning", cfg.Byte41.String(), "0"},
	}
	for _, c := range crossChecks {
		_, inSet := starts[c.pos]
		switch {
		case c.fixed && !inSet:
			return fmt.Errorf("%w (%s): %s is %s but position %d is not in the printed-fixed set", ErrLayoutInvalid, cfg.Model, c.axis, c.meaning, c.pos)
		case !c.fixed && inSet:
			return fmt.Errorf("%w (%s): position %d is in the printed-fixed set but %s is %s, which gives that byte a meaning", ErrLayoutInvalid, cfg.Model, c.pos, c.axis, c.meaning)
		case c.fixed && starts[c.pos] != c.want:
			return fmt.Errorf("%w (%s): position %d is printed %q, but the book that hard-wires it prints %q", ErrLayoutInvalid, cfg.Model, c.pos, starts[c.pos], c.want)
		}
	}
	return nil
}

// Configured reports whether l carries data. False for the zero Layout.
func (l Layout) Configured() bool { return l.book.valid() && l.modeNames != nil }

// Book is the PC-command document this row speaks.
func (l Layout) Book() Book { return l.book }

// Model is this row's own name, as its refusals quote it.
func (l Layout) Model() string { return l.model }

// P2Policy is byte 4's policy on this row.
func (l Layout) P2Policy() P2Policy { return l.p2 }

// Byte19 is byte 19's meaning on this row.
func (l Layout) Byte19() Byte19Meaning { return l.byte19 }

// Byte28 is byte 28's policy on this row.
func (l Layout) Byte28() Byte28Policy { return l.byte28 }

// Byte3940 is bytes 39-40's meaning on this row.
func (l Layout) Byte3940() Byte3940Meaning { return l.byte3940 }

// Byte41 is byte 41's meaning on this row.
func (l Layout) Byte41() Byte41Meaning { return l.byte41 }

// ToneModes is byte 20's value set on this row.
func (l Layout) ToneModes() ToneModeSet { return l.toneModes }

// RecordLen is this row's MR answer / MW Set frame width in bytes.
func (l Layout) RecordLen() uint8 { return l.recordLen }

// P10Policy is bytes 25-27's policy on this row.
func (l Layout) P10Policy() P10Policy { return l.p10 }

// P12Policy is byte 29's policy on this row.
func (l Layout) P12Policy() P12Policy { return l.p12 }

// P13Policy is bytes 30-38's policy on this row.
func (l Layout) P13Policy() P13Policy { return l.p13 }

// MaxEXAddress is the highest EX menu number this row's book prints,
// inclusive (590:543, 590:544, 480:401). It is 0 on the zero Layout, which
// builds and admits no EX read at all.
func (l Layout) MaxEXAddress() uint8 { return l.maxEXAddress }

// ModeNames returns an independent copy of this row's mode legend.
func (l Layout) ModeNames() map[Mode]string {
	out := make(map[Mode]string, len(l.modeNames))
	for m, n := range l.modeNames {
		out[m] = n
	}
	return out
}

// Slots returns an independent copy of this row's slot space.
func (l Layout) Slots() []SlotRange {
	out := make([]SlotRange, len(l.slots))
	copy(out, l.slots)
	return out
}

// PrintedFixed returns an independent copy of this row's hard-wired byte
// set, ordered by position.
func (l Layout) PrintedFixed() []FixedField {
	out := make([]FixedField, len(l.printedFixed))
	copy(out, l.printedFixed)
	return out
}

// classOf reports which class number falls in under this layout's slot
// space, or SlotClassInvalid. NewLayout has already refused an overlapping
// space, so at most one range can match.
func (l Layout) classOf(number int) SlotClass {
	for _, r := range l.slots {
		if number >= r.Lo && number <= r.Hi {
			return r.Class
		}
	}
	return SlotClassInvalid
}
