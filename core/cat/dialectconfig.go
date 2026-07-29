// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// SlotSpace is the exported description of one radio family's memory slot
// numbering: which 3-byte wire forms exist and what each means.
//
// It is the public face of the unexported slotSpace that Dialect stores.
// Both exist because Dialect's fields are unexported — which is what makes
// its zero value inert — while a package outside core/cat must still be
// able to describe a radio. See NewDialect.
type SlotSpace struct {
	// MemoryLo/MemoryHi are the inclusive decimal range of ordinary memory
	// channels, e.g. 1..99. Absent is expressed as exactly (0, 0).
	//
	// MemoryLo may legitimately be 0: a radio numbering its channels from
	// 000 is representable, and one of this package's own test dialects
	// depends on it. The danger that creates — a "000" none-form shadowing
	// channel 000 — is validated separately, because it is a collision
	// between two fields rather than a bad range.
	MemoryLo, MemoryHi int

	// SixtyLo/SixtyHi are the inclusive decimal range of the 60m bank, e.g.
	// 501..599. Absent is expressed as exactly (0, 0).
	SixtyLo, SixtyHi int

	// PMSPairs is the number of programmable-memory-scan pairs, e.g. 9 for
	// P1L..P9U. 0 means the family has none.
	//
	// The wire form's pair number is a SINGLE ASCII digit, so this can
	// never validly exceed 9. NewDialect rejects a larger value rather than
	// clamping it: a dialect declaring 12 pairs is a transcription error,
	// and silently giving it 9 hides the mistake at the point it is easiest
	// to find.
	PMSPairs int

	// EmergencyWire is the emergency channel's wire form, e.g. "EMG". ""
	// means the family has none.
	EmergencyWire string

	// NoneWire is the "VFO or MT or QMB" form, e.g. "000". "" means absent.
	NoneWire string
}

// MTForm names the FRAME SHAPE a family's MT command takes.
//
// Its zero value is deliberately NOT a valid form, so a config that omits
// it is refused rather than defaulting to one — the M9c-1 ruling on
// ShiftDirection and ToneSemantics, for the same reason. Defaulting here
// would silently give a combined-form config the FT-710's layout, and the
// resulting frame would be built, gate-approved and sent.
type MTForm int

const (
	// MTFormUnspecified is the zero value: refused by V9, never a form.
	MTFormUnspecified MTForm = iota
	// MTFormShort is "MT" + slot(3) + display(1) + tag(0..TagMaxBytes) +
	// ";" — the FT-710's, and the only shape evidenced by hardware.
	MTFormShort
	// MTFormCombined is "MT" + the shared 28-position memory field block's
	// fields + P11 '0' + tag(TagMaxBytes) + ";" — the FTdx10 family's.
	//
	// This value pins EXACTLY that layout: the classic memory block at
	// memdata.go's offsets, then P11, then the tag. A future radio whose
	// combined frame differs in its FIELD BLOCK is a new form (or, if slot,
	// mode and policy sharing has broken down entirely, a sibling codec) —
	// it is NOT a parameterisation of this one.
	MTFormCombined
)

func (f MTForm) String() string {
	switch f {
	case MTFormUnspecified:
		return "MTFormUnspecified"
	case MTFormShort:
		return "MTFormShort"
	case MTFormCombined:
		return "MTFormCombined"
	default:
		return fmt.Sprintf("MTForm(%d)", int(f))
	}
}

// MTPolicy carries the MT command's dialect-varying dimensions, ACROSS BOTH
// evidenced frame forms.
//
// EVERY FIELD BELONGS TO ONE FORM, and V9 enforces that ownership in both
// directions: an inapplicable field must be explicitly zero, an applicable
// one explicitly valid. Form itself selects which set applies, and its zero
// value is refused.
//
//	Field        | MTFormShort              | MTFormCombined
//	-------------|--------------------------|---------------------------
//	TagMaxBytes  | longest accepted tag     | longest accepted tag AND
//	             |                          | the tag field's width
//	ClearTagByte | required valid wire byte | must be 0
//	PadByte      | 0, or a valid wire byte  | must be 0
//	TagFill      | must be 0                | required valid wire byte
//
// The type stays comparable — scalars only — because Dialect equivalence is
// asserted with != (dialectequiv_test.go).
type MTPolicy struct {
	// Form is this family's MT frame shape. It has no default: see MTForm.
	Form MTForm

	// TagMaxBytes is the longest tag this family accepts, measured in
	// BYTES. FT-710: 12.
	//
	// UNDER MTFormCombined IT CARRIES A SECOND MEANING: the width of the
	// frame's fixed tag FIELD, which builds pad to exactly. That is a bet,
	// recorded rather than hidden — the policy bound and the field width
	// coincide on the evidenced radio (12/12 on the FTdx10), and two
	// independently configurable facts that must always agree is the worse
	// default. A combined family whose field is WIDER than the tag length
	// it accepts is inexpressible until a TagFieldWidth split, which is
	// additive: if such a counter-radio appears, the field is added then
	// and this field keeps the policy meaning.
	TagMaxBytes int

	// ClearTagByte is the byte an EMPTY tag is filled with to produce the
	// clear form. FT-710: ' '. SHORT-FORM ONLY; must be zero under
	// MTFormCombined, where no distinct clear encoding is documented — an
	// empty tag there is simply the all-TagFill field.
	//
	// It is carried separately from TagMaxBytes rather than derived,
	// because "an empty tag becomes TagMaxBytes spaces" bundles a width
	// with a fill convention and only the FT-710's is evidenced. A
	// family clearing with some other byte is expressible; one deriving the
	// clear form some entirely different way is not, and would need this
	// type extended rather than reinterpreted.
	ClearTagByte byte

	// PadByte is the byte the RADIO pads a short tag with in its answers,
	// trimmed on decode. 0 means this family declares no padding, and its
	// answers are returned verbatim. SHORT-FORM ONLY; must be zero under
	// MTFormCombined, where answer trimming is TagFill's job.
	//
	// SEPARATE FROM ClearTagByte ON PURPOSE, and the separation is the
	// whole point. Conflating the two is the defect this field exists to
	// end: decoding first trimmed every trailing ClearTagByte, destroying
	// "CALL-" on a '-'-clearing dialect; the repair trimmed spaces
	// universally, destroying "CALL " instead; the second repair trimmed
	// spaces only for a space-clearing dialect, which merely relocated the
	// assumption — a constructed dialect declaring a space clear byte
	// silently inherited the FT-710's padding behaviour without ever
	// declaring it (milestone review finding 4, three rounds).
	//
	// ClearTagByte says how an EMPTY tag is written. PadByte says how a
	// SHORT tag comes back. They happen to coincide for the FT-710, which
	// is exactly why the conflation survived so long.
	PadByte byte

	// TagFill is the COMBINED FORM'S fill byte, and it is both halves of
	// that job: the byte a build pads the outbound tag field to
	// TagMaxBytes with, AND the byte a parse trims from the answer. The two
	// are one field here because the combined form's field is fixed-width
	// in both directions — unlike the short form, where ClearTagByte and
	// PadByte describe genuinely different events and must stay apart.
	//
	// COMBINED-FORM ONLY; must be zero under MTFormShort. Under
	// MTFormCombined it is ZERO-INVALID: an omitted fill must not silently
	// emit NUL into every outbound tag field, so V9 requires a valid wire
	// byte rather than defaulting to ' '.
	TagFill byte
}

// ClarifierPolicy bounds MemoryData.ClarHz for one family.
type ClarifierPolicy struct {
	// StepHz is the clarifier's granularity in Hz. FT-710: 10.
	//
	// This is a radio characteristic rather than a field width: a rig
	// stepping 1 Hz through the same 4-digit field would reach 9999 where
	// the FT-710 stops at 9990.
	StepHz int

	// MaxAbsHz is the largest magnitude, in Hz, in either direction.
	// FT-710: 9990.
	MaxAbsHz int
}

// DialectConfig is the input to NewDialect: everything that varies between
// radios sharing the classic NEWCAT grammar, as plain data.
//
// A flat struct rather than functional options, deliberately. Dialect
// carries DATA, not behaviour, and a flat config can be validated
// exhaustively in one place — "is every required field set and mutually
// consistent?" is a question this shape can answer and a half-applied set
// of options cannot.
type DialectConfig struct {
	// CATID is the four-character identity the radio answers "ID;" with,
	// e.g. "0800". Exactly four bytes: the answer frame is "ID" + 4 + ";".
	CATID string

	// ModeNames maps every mode nibble this family knows to its display
	// name. Both halves are load-bearing: the KEY is emitted into the P6
	// field of an MW frame, and the NAME is what reaches a codeplug, the
	// CLI and the GUI — and, through Dialect.ModeByName, what a written
	// channel's mode is resolved from. Names must therefore be unique.
	ModeNames map[Mode]string

	// Slots describes the memory slot numbering.
	Slots SlotSpace

	// EXItems is this family's menu inventory. May be empty: a radio with
	// no modelled EX surface is representable.
	EXItems []EXItem

	// MT is the MT command's frame form and tag policy.
	MT MTPolicy

	// Clarifier bounds the clarifier field.
	Clarifier ClarifierPolicy

	// MWWriteKind is the single P7 "kind" byte this family accepts on
	// EVERY memory write, e.g. KindMemory for the FT-710.
	//
	// Typed byte rather than a named type because MemoryData.Kind and every
	// Kind* constant are already plain bytes; introducing a name here that
	// the rest of the package does not use would be a new spelling for an
	// existing concept.
	MWWriteKind byte
}

// validWireByte reports whether b may appear in the INTERIOR of a CAT
// frame this package builds — that is, anywhere except the terminator.
//
// The domain is printable ASCII, 0x20..0x7E, excluding ';'.
//
// ';' is excluded because it TERMINATES a frame. A byte of dialect data
// carrying one would split a single command into two on the wire, and the
// outbound gate's whole-frame checks count semicolons rather than
// re-deriving structure from scratch — so a smuggled terminator changes
// what the radio executes without the gate seeing a second command.
//
// Non-printable bytes are excluded because no CAT field in any reference
// documents one. Admitting them let a caller-built dialect emit a frame the
// gate then approved containing a NUL: a Mode key of 0x00 goes straight
// into an MW frame's P6 field, and an EmergencyWire of "\x00AB" produces a
// side-effecting MC frame. Both passed a rule set that checked lengths and
// ranges but never bytes (Codex spec review, finding 2).
func validWireByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E && b != ';'
}

// validWireString reports whether every byte of s is in the interior
// domain. Empty is true: whether empty is ACCEPTABLE is a separate question
// each caller answers for itself, since "" is a legitimate way to say a
// family has no emergency channel but not a legitimate CAT ID.
func validWireString(s string) bool {
	for i := 0; i < len(s); i++ {
		if !validWireByte(s[i]) {
			return false
		}
	}
	return true
}

// NewDialect validates cfg and returns the Dialect it describes.
//
// This is the API M9c depends on: every Dialect field is unexported, which
// is what makes the zero value inert, so before this constructor existed no
// package outside core/cat could express a radio at all — and the roadmap
// requires exactly that of core/cat/ftdx10 and its siblings.
//
// It COPIES every slice and map it is given and derives the EX and mode
// indices from the copies. A caller that mutates its input afterwards must
// not be able to change a constructed dialect: a Dialect is consulted by
// the outbound gate on every write, and a gate whose data can be edited
// after the fact by whoever built it is not a gate.
//
// Validation is exhaustive rather than advisory. See dialectvalidate.go for
// the eleven rules and, for each, the concrete failure it prevents — three
// of them (the wire-byte domain on mode keys, on special slot forms, and on
// the MT clear byte) exist specifically because a caller-built dialect
// could otherwise put a byte no CAT reference documents inside a frame this
// program's own gate then approved.
func NewDialect(cfg DialectConfig) (Dialect, error) {
	if err := validateDialectConfig(cfg); err != nil {
		return Dialect{}, err
	}

	modes := make(map[Mode]string, len(cfg.ModeNames))
	for m, name := range cfg.ModeNames {
		modes[m] = name
	}

	items := make([]EXItem, len(cfg.EXItems))
	copy(items, cfg.EXItems)

	return Dialect{
		catID:     cfg.CATID,
		modeNames: modes,
		slots: slotSpace{
			memoryLo: cfg.Slots.MemoryLo,
			memoryHi: cfg.Slots.MemoryHi,
			sixtyLo:  cfg.Slots.SixtyLo,
			sixtyHi:  cfg.Slots.SixtyHi,
			pmsPairs: cfg.Slots.PMSPairs,
			emgWire:  cfg.Slots.EmergencyWire,
			noneWire: cfg.Slots.NoneWire,
		},
		exItems:     items,
		exMembers:   buildEXMembers(items),
		exByTriple:  buildEXByTriple(items),
		exP4Max:     maxEXP4Bytes(items),
		modeByName:  buildModeByName(modes),
		mt:          cfg.MT,
		clar:        cfg.Clarifier,
		mwWriteKind: cfg.MWWriteKind,
	}, nil
}

// MustNewDialect is NewDialect for COMPILE-TIME-CONSTANT model tables, and
// panics if cfg is invalid.
//
// It exists to satisfy the roadmap's `func Dialect() cat.Dialect` shape for
// per-model packages, which cannot propagate an error, without threading
// error returns through model registration for a failure that can only ever
// be a programming mistake in a literal.
//
// Do NOT call it on caller-supplied, file-derived, or otherwise dynamic
// data. A malformed table baked into the binary is a build-time defect that
// should stop the programme loudly on first use; a malformed table read
// from a file at runtime is an ordinary error, and NewDialect is the
// function for that.
func MustNewDialect(cfg DialectConfig) Dialect {
	d, err := NewDialect(cfg)
	if err != nil {
		panic("cat: MustNewDialect: " + err.Error())
	}
	return d
}
