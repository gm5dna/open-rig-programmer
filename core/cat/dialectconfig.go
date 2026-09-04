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

	// MCSelects is the SEND-side domain of this family's MC (memory channel
	// recall) command: which of the slots above an MC Set may name.
	//
	// It has no default — the zero value is refused by V13 — because the
	// two readings differ by four whole slot banks and the wrong one is
	// SIDE-EFFECTING: an MC Set recalls the channel and changes the radio's
	// operating state, so defaulting a family whose legend prints memory and
	// PMS only to "all" would send a frame its own manual never describes.
	// The M9c-1 ruling, applied to a slot domain.
	MCSelects MCSlotPolicy
}

// MCSlotPolicy names the SEND-side slot domain of the MC command.
//
// It exists because the MC legend is NOT the MR legend on every radio. Each
// of the three registered dialects prints all four classes — memory, PMS,
// 5xx and EMG — against MC; a family whose MC block prints only memory and
// PMS must not have an MC Set built for a bank its manual never lists there,
// and must not have one admitted by its own outbound gate either.
//
// IT GOVERNS THE SEND DIRECTION ONLY. ParseMCAnswer keeps the full readable
// space whatever this says: an MC Set and an MC Answer share one wire shape,
// and a radio sitting on a 60m channel it reached from the front panel will
// answer with it however narrow its Set domain is. Narrowing the parse side
// from this field would turn a legitimate answer into an error.
//
// Its zero value is deliberately NOT a policy, so a config omitting it is
// refused (V13) rather than defaulted.
type MCSlotPolicy int

const (
	// MCSelectsAll is the three registered dialects' domain: memory, PMS,
	// 60m and EMG — every slot class outside the "000" none form.
	MCSelectsAll MCSlotPolicy = iota + 1
	// MCSelectsMemoryPMS is the narrower domain: memory and PMS only.
	MCSelectsMemoryPMS
)

// String names the policy, so a refusal can quote it.
func (p MCSlotPolicy) String() string {
	switch p {
	case MCSelectsAll:
		return "MCSelectsAll"
	case MCSelectsMemoryPMS:
		return "MCSelectsMemoryPMS"
	default:
		return fmt.Sprintf("MCSlotPolicy(%d)", int(p))
	}
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

// EXAddressForm names the WIRE WIDTH a family's EX address field takes.
//
// Its zero value is deliberately NOT a valid form, so a config that omits
// it is refused rather than defaulting to one — the M9c-1 ruling, and
// MTForm's own reason above. Defaulting to the six-digit form would give a
// four-digit radio an address field two bytes too long in every EX read
// this codec builds, gate-approves and sends, and would make that radio's
// own honest answers unparseable.
//
// The form is DATA on the DIALECT rather than on the ADDRESS, because an
// EXAddress carries no family: the same (P1,P2,P3) triple is six digits for
// one radio and four for another. That is why EXAddress.Wire() was deleted
// at this seam — see wireEXAddress, the one place an address becomes wire
// digits.
type EXAddressForm int

const (
	// EXAddressTriple is the six-digit "P1 P1 P2 P2 P3 P3" field, the form
	// every registered dialect's EX grammar block prints.
	EXAddressTriple EXAddressForm = iota + 1
	// EXAddressPair is the four-digit field EXAddress's own (P1,P2)
	// components render as, with P3 dropped. Under it every EXItems member
	// must have P3 == 0 (V12): the render drops P3, and a component
	// silently dropped from every frame is exactly the failure this
	// validator exists to make impossible.
	//
	// NAMING TRAP for a reader with a manual open: "(P1,P2)" is THIS
	// PACKAGE'S naming for the field's two two-digit components, not
	// necessarily the radio's own. The FT-891 — this form's first member —
	// spells the whole four-digit field a single P1 in its own EX grammar
	// block ("E X P1 P1 P1 P1", ft891_layout.txt:513-522) and calls the
	// parameter body P2; it has no P3 or P4 at all. The BYTES agree
	// exactly (wireEXAddress under this form emits the same four digits
	// the FT-891's P1 does) — only the component names differ. Any comment
	// elsewhere that mentions this form's naming should point here rather
	// than restate it.
	EXAddressPair
)

func (f EXAddressForm) String() string {
	switch f {
	case EXAddressTriple:
		return "EXAddressTriple"
	case EXAddressPair:
		return "EXAddressPair"
	default:
		return fmt.Sprintf("EXAddressForm(%d)", int(f))
	}
}

// MTReadSlotPolicy names the slot domain of the MT READ request.
//
// It exists because the FT-891's MT block is the first whose own slot legend
// prints memory and PMS ONLY — `001 - 099 / P1L - P9U` — where its MR block
// prints the 5xx and EMG banks as well, and where every registered sibling's
// MT legend prints all four classes. Until Stage 0 this codec had one rule
// for both commands ("reads are not restricted", mt.go): a read has no side
// effect, so the write-direction hardware concern that confines MT SET to
// memory and PMS does not apply to it. That reasoning is still right and is
// not what this field changes. What it changes is the assumption underneath
// it — that MT's readable domain is MR's — which the FT-891 refutes in
// print.
//
// It governs BuildMTRead and the outbound gate's MT read branch, which is
// the whole of the MT read's surface. MR's own read domain is untouched:
// Dialect.readableSlot still answers for BuildMRRead and the gate's MR
// branch, and the discovered 5xx/EMG banks are read by MR alone on a
// MTReadsMemoryPMS radio.
//
// Its zero value is deliberately NOT a policy, so a config omitting it is
// refused (inside V9) rather than defaulted.
type MTReadSlotPolicy int

const (
	// MTReadsReadable is the three registered dialects' domain: every slot
	// this dialect's ParseSlot accepts except the "000" none form —
	// Dialect.readableSlot, the rule MR reads by.
	MTReadsReadable MTReadSlotPolicy = iota + 1
	// MTReadsMemoryPMS is the narrower domain: memory and PMS only, the
	// slots an MT legend printing just those two names.
	MTReadsMemoryPMS
)

// String names the policy, so a refusal can quote it.
func (p MTReadSlotPolicy) String() string {
	switch p {
	case MTReadsReadable:
		return "MTReadsReadable"
	case MTReadsMemoryPMS:
		return "MTReadsMemoryPMS"
	default:
		return fmt.Sprintf("MTReadSlotPolicy(%d)", int(p))
	}
}

// MTP11Policy names what byte 28 of the COMBINED MT record — P11, the byte
// immediately after the shared memory field block — means on one family.
//
// Every registered combined-form sibling prints it "P11 0: (Fixed)", which
// is why core/cat carried it as the form constant combinedMTP11. The FT-891
// prints `P11 0: TAG "OFF" 1: TAG "ON"`: on that radio the byte is a live
// flag the caller supplies and the radio reports, exactly as the FT-710's
// SHORT form already carries a display flag beside its tag.
//
// A LIVE FLAG IS NEVER DEFAULTED (the M9c-1 ruling, and both spec
// reviewers). Under P11TagDisplay the display-LESS builder and parser —
// BuildMTSetCombined and ParseMTAnswerCombined — REFUSE, rather than
// silently writing '0' for a flag the caller never expressed an intention
// about; and under P11Fixed the display-BEARING pair refuses in turn, so a
// caller cannot express a flag on a radio that has none.
//
// It belongs to MTFormCombined alone. Under MTFormShort the display flag is
// already a parameter of BuildMTSet, so V9 requires this to be explicitly
// zero there — the same ownership rule TagFill, ClearTagByte and PadByte
// keep.
type MTP11Policy int

const (
	// P11Fixed is the FTdx10 family's: byte 28 is the printed "0: (Fixed)",
	// emitted by the builder and required by the parser.
	P11Fixed MTP11Policy = iota + 1
	// P11TagDisplay is the FT-891's: byte 28 is the TAG ON/OFF flag, built
	// from a caller-supplied value and parsed into one, '0' or '1' only.
	P11TagDisplay
)

// String names the policy, so a refusal can quote it.
func (p MTP11Policy) String() string {
	switch p {
	case P11Fixed:
		return "P11Fixed"
	case P11TagDisplay:
		return "P11TagDisplay"
	default:
		return fmt.Sprintf("MTP11Policy(%d)", int(p))
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
//	P11          | must be 0                | required valid policy
//
// The type stays comparable — scalars only — because Dialect equivalence is
// asserted with != (dialectequiv_test.go).
type MTPolicy struct {
	// Form is this family's MT frame shape. It has no default: see MTForm.
	Form MTForm

	// ReadSlots is the MT READ request's slot domain. It belongs to BOTH
	// forms — the read request carries neither a record nor a tag, so its
	// shape and its domain are the same question under either — which is
	// why it sits outside the per-form ownership table above and V9
	// requires it whatever Form says.
	//
	// It has no default: see MTReadSlotPolicy.
	ReadSlots MTReadSlotPolicy

	// P11 says what byte 28 of the COMBINED record means: the printed
	// "(Fixed)" '0', or a live TAG ON/OFF flag. COMBINED-FORM ONLY; must be
	// zero under MTFormShort, whose display flag is already a parameter of
	// BuildMTSet. It has no default under the combined form: see
	// MTP11Policy.
	P11 MTP11Policy

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

// MemoryP5Policy names what byte 21 of the shared 28-position memory field
// block — P5, memdata.go's memTxClarOffset — MEANS on one family.
//
// The three registered dialects print `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"`
// against MR, MT and MW alike, so the byte carries MemoryData.TxClar. The
// FT-891 prints `0: (Fixed)` on every one of those blocks and on IF: the
// byte is schema there, not state, and this codec must neither emit a '1'
// into it nor read one back as a flag.
//
// IT GOVERNS BOTH DIRECTIONS, unlike MCSelects. Under P5Fixed the encoder
// writes '0', every builder REFUSES a record carrying TxClar true rather
// than silently correcting it — a caller who believed it was writing the TX
// clarifier finds out — and the parser REQUIRES '0', which is the same
// treatment this package already gives P9 and the combined form's P11 under
// P11Fixed. A printed-fixed byte that comes back as something else is an
// undocumented frame, and turning one into data is what this package
// refuses to do.
//
// Its zero value is deliberately NOT a policy, so a config omitting it is
// refused (V14) rather than defaulted.
type MemoryP5Policy int

const (
	// P5TxClar is the three registered dialects' reading: byte 21 is the
	// TX clarifier flag, '0' off and '1' on, in both directions.
	P5TxClar MemoryP5Policy = iota + 1
	// P5Fixed is the FT-891's: byte 21 is printed "0: (Fixed)" on every
	// memory-bearing block, so '0' is the only value either direction may
	// carry and TxClar is never true.
	P5Fixed
)

// String names the policy, so a refusal can quote it.
func (p MemoryP5Policy) String() string {
	switch p {
	case P5TxClar:
		return "P5TxClar"
	case P5Fixed:
		return "P5Fixed"
	default:
		return fmt.Sprintf("MemoryP5Policy(%d)", int(p))
	}
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

	// EXAddressForm is the wire width of this family's EX address field.
	// It has no default — see EXAddressForm — and V12 refuses the zero
	// value even for a dialect whose EXItems is empty, because the width
	// also sizes the EX read frame the outbound gate measures.
	EXAddressForm EXAddressForm

	// MT is the MT command's frame form and tag policy.
	MT MTPolicy

	// Clarifier bounds the clarifier field.
	Clarifier ClarifierPolicy

	// MemoryP5 says what byte 21 of the shared memory field block means on
	// this family: the TX clarifier flag, or a printed-fixed '0'. It has no
	// default: see MemoryP5Policy.
	MemoryP5 MemoryP5Policy

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

			mcSelects: cfg.Slots.MCSelects,
		},
		exItems:     items,
		exAddrForm:  cfg.EXAddressForm,
		exMembers:   buildEXMembers(items),
		exByTriple:  buildEXByTriple(items),
		exP4Max:     maxEXP4Bytes(items),
		modeByName:  buildModeByName(modes),
		mt:          cfg.MT,
		clar:        cfg.Clarifier,
		memoryP5:    cfg.MemoryP5,
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
