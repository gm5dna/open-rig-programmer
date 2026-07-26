// SPDX-License-Identifier: GPL-3.0-or-later

package cat

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

// MTPolicy carries the MT short form's dialect-varying dimensions.
//
// Per-command frame-shape variants — the combined record frame the
// FTdx10/101 manuals document against the FT-710's short form — are a later
// milestone's. This type describes the SHORT form only.
type MTPolicy struct {
	// TagMaxBytes is the longest tag this family accepts, measured in
	// BYTES. FT-710: 12.
	TagMaxBytes int

	// ClearTagByte is the byte an EMPTY tag is padded with to produce the
	// clear form. FT-710: ' '.
	//
	// It is carried separately from TagMaxBytes rather than derived,
	// because "an empty tag becomes TagMaxBytes spaces" bundles a width
	// with a padding convention and only the FT-710's is evidenced. A
	// family clearing with some other byte is expressible; one deriving the
	// clear form some entirely different way is not, and would need this
	// type extended rather than reinterpreted.
	ClearTagByte byte
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

	// MT is the MT short form's tag policy.
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
